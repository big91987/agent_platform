package platform

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Connector is an administrator-owned capability. Runs freeze its configuration;
// current enablement and user access are checked again before each external call.
type Connector struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Revision        int64             `json:"revision"`
	Enabled         bool              `json:"enabled"`
	AuthorizedUsers []string          `json:"authorized_users"`
	Kind            string            `json:"kind"`
	WorkspaceRoot   string            `json:"workspace_root"`
	Repository      string            `json:"repository,omitempty"`
	TokenEnv        string            `json:"token_env,omitempty"`
	Executable      string            `json:"executable,omitempty"`
	Args            []string          `json:"args,omitempty"`
	EnvRefs         map[string]string `json:"env_refs,omitempty"`
	TimeoutSeconds  int               `json:"timeout_seconds"`
	Updated         string            `json:"updated_at"`
}
type ConnectorInput struct {
	Title       string `json:"title,omitempty"`
	BodyFile    string `json:"body_file,omitempty"`
	Head        string `json:"head,omitempty"`
	Base        string `json:"base,omitempty"`
	IssueNode   string `json:"issue_node,omitempty"`
	IssueNumber int    `json:"issue_number,omitempty"`
}
type ConnectorReceipt struct {
	Kind      string `json:"kind"`
	URL       string `json:"url,omitempty"`
	Number    int    `json:"number,omitempty"`
	HeadSHA   string `json:"head_sha,omitempty"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	Output    string `json:"output,omitempty"`
	Recovered bool   `json:"recovered,omitempty"`
}

func (s *Store) initConnectors() error {
	_, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS connectors(id TEXT PRIMARY KEY,revision INTEGER NOT NULL,definition TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS workflow_run_connectors(run_id TEXT NOT NULL REFERENCES workflow_runs(id),connector_id TEXT NOT NULL,definition TEXT NOT NULL,PRIMARY KEY(run_id,connector_id));
 CREATE TABLE IF NOT EXISTS workflow_connector_actions(run_id TEXT NOT NULL REFERENCES workflow_runs(id),seq INTEGER NOT NULL,request TEXT NOT NULL,receipt TEXT NOT NULL DEFAULT '',error TEXT NOT NULL DEFAULT '',PRIMARY KEY(run_id,seq));`)
	return err
}
func connectorAllowed(c Caller, v Connector) bool {
	return c.Admin || c.UserID != "" && slices.Contains(v.AuthorizedUsers, c.UserID)
}
func loadConnector(q interface{ QueryRow(string, ...any) *sql.Row }, id string) (Connector, error) {
	var v Connector
	var raw string
	err := q.QueryRow(`SELECT definition FROM connectors WHERE id=?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal([]byte(raw), &v)
	}
	return v, err
}
func (s *Store) Connector(c Caller, id string) (Connector, error) {
	v, err := loadConnector(s.DB, id)
	if err == nil && !connectorAllowed(c, v) {
		return Connector{}, ErrForbidden
	}
	return v, err
}
func (s *Store) Connectors(c Caller) ([]Connector, error) {
	rows, err := s.DB.Query(`SELECT definition FROM connectors ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Connector{}
	for rows.Next() {
		var raw string
		var v Connector
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, err
		}
		if connectorAllowed(c, v) {
			out = append(out, v)
		}
	}
	return out, rows.Err()
}

var githubRepo = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func validateConnector(v Connector) error {
	if strings.TrimSpace(v.Name) == "" || len(v.Name) > 200 {
		return errors.New("connector name required (up to 200 bytes)")
	}
	if v.TimeoutSeconds < 1 || v.TimeoutSeconds > 1800 {
		return errors.New("timeout_seconds must be 1–1800")
	}
	if !filepath.IsAbs(v.WorkspaceRoot) {
		return errors.New("workspace_root must be absolute")
	}
	switch v.Kind {
	case "command":
		if !filepath.IsAbs(v.Executable) || len(v.Args) > 128 || len(v.EnvRefs) > 32 {
			return errors.New("command needs an absolute executable and bounded arguments/environment")
		}
		for _, a := range v.Args {
			if len(a) > 16000 || strings.ContainsRune(a, 0) {
				return errors.New("invalid command argument")
			}
		}
		for k, ref := range v.EnvRefs {
			if !envName.MatchString(k) || !envName.MatchString(ref) {
				return errors.New("environment values must be variable names, not secrets")
			}
		}
		if v.Repository != "" || v.TokenEnv != "" {
			return errors.New("command cannot include GitHub configuration")
		}
	case "github.issue_create", "github.issue_comment", "github.pull_request":
		if !githubRepo.MatchString(v.Repository) || !envName.MatchString(v.TokenEnv) {
			return errors.New("GitHub needs owner/repository and token environment-variable name")
		}
		if v.Executable != "" || len(v.Args) > 0 || len(v.EnvRefs) > 0 {
			return errors.New("GitHub connector cannot include a command")
		}
	default:
		return errors.New("unsupported connector kind")
	}
	return nil
}
func (s *Store) SaveConnector(c Caller, v Connector) (Connector, error) {
	if !c.Admin {
		return v, ErrForbidden
	}
	v.Name = strings.TrimSpace(v.Name)
	if v.TimeoutSeconds == 0 {
		v.TimeoutSeconds = 60
	}
	if err := validateConnector(v); err != nil {
		return v, err
	}
	if v.ID == "" {
		if v.Revision != 0 {
			return v, ErrConflict
		}
		v.ID = newID()
	} else if !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(v.ID) || v.Revision < 1 {
		return v, ErrConflict
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return v, err
	}
	defer tx.Rollback()
	for _, id := range v.AuthorizedUsers {
		var n int
		if err = tx.QueryRow(`SELECT count(*) FROM users WHERE user_id=?`, id).Scan(&n); err != nil {
			return v, err
		}
		if n != 1 {
			return v, errors.New("unknown authorized user")
		}
	}
	old := v.Revision
	v.Revision++
	v.Updated = now()
	b, _ := json.Marshal(v)
	if old == 0 {
		_, err = tx.Exec(`INSERT INTO connectors(id,revision,definition) VALUES(?,?,?)`, v.ID, v.Revision, string(b))
	} else {
		var result sql.Result
		result, err = tx.Exec(`UPDATE connectors SET revision=?,definition=? WHERE id=? AND revision=?`, v.Revision, string(b), v.ID, old)
		if err == nil {
			n, _ := result.RowsAffected()
			if n != 1 {
				return v, ErrConflict
			}
		}
	}
	if err != nil {
		return v, err
	}
	return v, tx.Commit()
}
func connectorWorkspace(v Connector, workspace string) error {
	root, err := normalizeWorkspace(v.WorkspaceRoot)
	if err != nil {
		return fmt.Errorf("connector workspace root: %w", err)
	}
	actual, err := normalizeWorkspace(workspace)
	if err != nil {
		return err
	}
	if !containsPath(root, actual) {
		return errors.New("workspace is outside connector's allowed root")
	}
	return nil
}
func connectorBody(workspace, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", errors.New("body_file must be a relative file inside workspace")
	}
	root, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		return "", err
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, name))
	if err != nil {
		return "", err
	}
	if !containsPath(root, path) {
		return "", errors.New("body_file escapes workspace")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("body_file must be regular")
	}
	b, err := io.ReadAll(io.LimitReader(f, 64001))
	if err != nil {
		return "", err
	}
	if len(b) > 64000 {
		return "", errors.New("body_file exceeds 64000 bytes")
	}
	return string(b), nil
}
func validateConnectorNode(v Connector, n WorkflowNode, w Workflow) error {
	if w.target(n.ID, "next") == "" {
		return errors.New("connector requires a next route")
	}
	for _, route := range w.routes(n.ID) {
		if route != "next" && !(v.Kind == "command" && route == "failed") {
			return errors.New("connector routes: next; command also supports failed")
		}
	}
	return validateConnectorInput(v, n, w)
}
func validateConnectorInput(v Connector, n WorkflowNode, w Workflow) error {
	p := n.ConnectorInput
	if len(p.Title) > 256 || len(p.BodyFile) > 4096 || len(p.Head) > 256 || len(p.Base) > 256 || p.IssueNumber < 0 {
		return errors.New("invalid connector parameters")
	}
	if v.Kind == "command" {
		if p != (ConnectorInput{}) {
			return errors.New("command receives run context on stdin; node arguments are not accepted")
		}
		return nil
	}
	if p.IssueNode != "" && p.IssueNumber != 0 {
		return errors.New("choose issue_node or issue_number")
	}
	if p.IssueNode != "" {
		node := w.node(p.IssueNode)
		if node.Kind != "connector" || node.ID == n.ID {
			return errors.New("issue_node must reference another connector node")
		}
	}
	if v.Kind == "github.issue_comment" && p.IssueNode == "" && p.IssueNumber == 0 {
		return errors.New("comment needs issue_node or issue_number")
	}
	if v.Kind == "github.pull_request" && (p.Head == "" || p.Base == "") {
		return errors.New("PR needs head and base branches")
	}
	return nil
}
func (h *Server) connectorRoutes() {
	h.mux.HandleFunc("GET /api/connectors", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		v, e := h.store.Connectors(c)
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, v)
	}))
	h.mux.HandleFunc("GET /api/connectors/{id}", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		v, e := h.store.Connector(c, r.PathValue("id"))
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, v)
	}))
	save := h.protect(true, func(w http.ResponseWriter, r *http.Request, c Caller) {
		var v Connector
		if e := decode(w, r, &v); e != nil {
			fail(w, e)
			return
		}
		if r.Method == "PUT" {
			if v.ID != r.PathValue("id") {
				fail(w, ErrConflict)
				return
			}
		} else if v.ID != "" {
			fail(w, ErrConflict)
			return
		}
		v, e := h.store.SaveConnector(c, v)
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, v)
	})
	h.mux.HandleFunc("POST /api/connectors", save)
	h.mux.HandleFunc("PUT /api/connectors/{id}", save)
	h.mux.HandleFunc("POST /api/connectors/{id}/check", h.protect(true, func(w http.ResponseWriter, r *http.Request, c Caller) {
		v, e := h.store.Connector(c, r.PathValue("id"))
		if e == nil {
			e = checkConnector(r.Context(), v)
		}
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, map[string]any{"ok": true, "message": "Configuration and read access checked; no write or command executed."})
	}))
}
