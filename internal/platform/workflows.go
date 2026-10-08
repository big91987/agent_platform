package platform

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strings"
)

// Workflow is a saved graph. A running instance will own its definition snapshot.
// Node names and routes have no built-in business-stage semantics.
type Workflow struct {
	ContextVersion  int            `json:"context_version,omitempty"`
	Hooks           []WorkflowHook `json:"hooks,omitempty"`
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Revision        int64          `json:"revision"`
	Enabled         bool           `json:"enabled"`
	AuthorizedUsers []string       `json:"authorized_users"`
	Entry           string         `json:"entry"`
	StartNodes      []string       `json:"start_nodes"`
	MaxSteps        int            `json:"max_steps"`
	Nodes           []WorkflowNode `json:"nodes"`
	Edges           []WorkflowEdge `json:"edges"`
	Updated         string         `json:"updated_at"`
}
type WorkflowNode struct {
	ExitMode                string          `json:"exit_mode,omitempty"`
	CompletionSchema        json.RawMessage `json:"completion_schema,omitempty"`
	CompletionInstructions  string          `json:"completion_instructions,omitempty"`
	AllowUserInput          *bool           `json:"allow_user_input,omitempty"`
	ContinuationLimit       int             `json:"continuation_limit,omitempty"`
	ExecutionTimeoutSeconds int             `json:"execution_timeout_seconds,omitempty"`
	ID                      string          `json:"id"`
	Name                    string          `json:"name"`
	Kind                    string          `json:"kind"`
	AgentID                 string          `json:"agent_id,omitempty"`
	Agent                   *Agent          `json:"agent,omitempty"`
	ConnectorID             string          `json:"connector_id,omitempty"`
	ConnectorInput          ConnectorInput  `json:"connector_input,omitempty"`
	Prompt                  string          `json:"prompt,omitempty"`
	X                       float64         `json:"x"`
	Y                       float64         `json:"y"`
}
type WorkflowEdge struct {
	Mode        string `json:"mode,omitempty"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source"`
	Route       string `json:"route"`
	Target      string `json:"target"`
}

var workflowKey = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

func validateWorkflow(w Workflow) error {
	if strings.TrimSpace(w.Name) == "" || len(w.Name) > 200 {
		return errors.New("workflow name is required (up to 200 bytes)")
	}
	if len(w.Nodes) == 0 || len(w.Nodes) > 128 || len(w.Edges) > 512 {
		return errors.New("workflow needs 1–128 nodes and at most 512 edges")
	}
	if w.MaxSteps < 1 || w.MaxSteps > 1000 {
		return errors.New("max_steps must be between 1 and 1000")
	}
	if w.ContextVersion < 0 || w.ContextVersion > 2 {
		return errors.New("unsupported context_version")
	}
	nodes := map[string]WorkflowNode{}
	for _, n := range w.Nodes {
		if !workflowKey.MatchString(n.ID) {
			return fmt.Errorf("invalid node key: %s", n.ID)
		}
		if _, ok := nodes[n.ID]; ok {
			return fmt.Errorf("duplicate node: %s", n.ID)
		}
		if strings.TrimSpace(n.Name) == "" || len(n.Name) > 200 || len(n.Prompt) > 32000 {
			return fmt.Errorf("invalid name or prompt for node %s", n.ID)
		}
		for _, v := range []float64{n.X, n.Y} {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 10000 {
				return fmt.Errorf("invalid position for node %s", n.ID)
			}
		}
		switch n.Kind {
		case "agent":
			if (n.AgentID == "") == (n.Agent == nil) || n.ConnectorID != "" {
				return fmt.Errorf("node %s requires one agent", n.ID)
			}
			if w.ContextVersion == 2 && n.Agent != nil && n.Agent.SeedDir != "" {
				return fmt.Errorf("node %s: set the shared workspace when starting the Run, not a node seed_dir", n.ID)
			}
		case "connector":
			if n.ConnectorID == "" || n.AgentID != "" || n.Agent != nil {
				return fmt.Errorf("node %s requires one connector", n.ID)
			}
		case "approval", "end":
			if n.AgentID != "" || n.ConnectorID != "" || n.Agent != nil {
				return fmt.Errorf("node %s cannot reference an executor", n.ID)
			}
		default:
			return fmt.Errorf("unsupported node kind: %s", n.Kind)
		}
		if n.ContinuationLimit < 0 || n.ContinuationLimit > 10 || n.ExecutionTimeoutSeconds < 0 || n.ExecutionTimeoutSeconds > 86400 {
			return fmt.Errorf("invalid continuation limits for node %s", n.ID)
		}
		if err := validateCompletionConfig(w, n); err != nil {
			return fmt.Errorf("node %s: %w", n.ID, err)
		}
		nodes[n.ID] = n
	}
	if _, ok := nodes[w.Entry]; !ok {
		return errors.New("entry node does not exist")
	}
	for _, id := range w.StartNodes {
		if _, ok := nodes[id]; !ok {
			return fmt.Errorf("start node does not exist: %s", id)
		}
	}
	forward, reverse := map[string][]string{}, map[string][]string{}
	routes := map[[2]string]bool{}
	automatic := map[string]int{}
	for _, e := range w.Edges {
		source, ok := nodes[e.Source]
		if !ok {
			return fmt.Errorf("edge source missing: %s", e.Source)
		}
		if _, ok := nodes[e.Target]; !ok {
			return fmt.Errorf("edge target missing: %s", e.Target)
		}
		if source.Kind == "end" {
			return fmt.Errorf("end node %s cannot have outgoing edges", source.ID)
		}
		if !workflowKey.MatchString(e.Route) {
			return fmt.Errorf("invalid route: %s", e.Route)
		}
		if e.Mode != "" && e.Mode != "handoff" && e.Mode != "automatic" {
			return fmt.Errorf("invalid edge mode: %s", e.Mode)
		}
		if len(e.Description) > 4000 {
			return errors.New("edge description exceeds 4000 bytes")
		}
		if e.Mode == "handoff" && source.Kind != "agent" {
			return errors.New("only Agent nodes can choose handoff edges")
		}
		if e.Mode == "automatic" && source.Kind == "agent" {
			automatic[e.Source]++
			if automatic[e.Source] > 1 {
				return errors.New("Agent completion requires at most one fixed edge")
			}
		}
		key := [2]string{e.Source, e.Route}
		if routes[key] {
			return fmt.Errorf("ambiguous route %s on %s", e.Route, e.Source)
		}
		routes[key] = true
		forward[e.Source] = append(forward[e.Source], e.Target)
		reverse[e.Target] = append(reverse[e.Target], e.Source)
	}
	walk := func(starts []string, graph map[string][]string) map[string]bool {
		seen := map[string]bool{}
		queue := append([]string(nil), starts...)
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if seen[id] {
				continue
			}
			seen[id] = true
			queue = append(queue, graph[id]...)
		}
		return seen
	}
	reachable := walk([]string{w.Entry}, forward)
	ends := []string{}
	for _, n := range w.Nodes {
		if n.Kind == "end" {
			ends = append(ends, n.ID)
		}
	}
	canFinish := walk(ends, reverse)
	for _, n := range w.Nodes {
		if !reachable[n.ID] {
			return fmt.Errorf("unreachable node: %s", n.ID)
		}
		if !canFinish[n.ID] {
			return fmt.Errorf("node %s has no path to an end", n.ID)
		}
	}
	if w.ContextVersion >= 1 {
		for _, n := range w.Nodes {
			if n.Kind == "agent" {
				if _, err := workflowNodeInstructions(w, n); err != nil {
					return err
				}
			}
		}
	}
	return validateWorkflowHooks(w)
}

func (s *Store) initWorkflows() error {
	_, e := s.DB.Exec(`CREATE TABLE IF NOT EXISTS workflows(id TEXT PRIMARY KEY,revision INTEGER NOT NULL,definition TEXT NOT NULL)`)
	if e != nil {
		return e
	}
	if e = s.initWorkflowRuns(); e != nil {
		return e
	}
	if e = s.initConnectors(); e != nil {
		return e
	}
	return s.initWorkflowHooks()
}
func workflowAllowed(c Caller, w Workflow) bool {
	return c.Admin || c.UserID != "" && slices.Contains(w.AuthorizedUsers, c.UserID)
}
func (s *Store) Workflow(c Caller, id string) (Workflow, error) {
	var w Workflow
	var raw string
	e := s.DB.QueryRow(`SELECT definition FROM workflows WHERE id=?`, id).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return w, ErrNotFound
	}
	if e != nil {
		return w, e
	}
	if e = json.Unmarshal([]byte(raw), &w); e != nil {
		return w, e
	}
	if !workflowAllowed(c, w) {
		return Workflow{}, ErrForbidden
	}
	return w, nil
}
func (s *Store) Workflows(c Caller) ([]Workflow, error) {
	rows, e := s.DB.Query(`SELECT definition FROM workflows ORDER BY id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []Workflow{}
	for rows.Next() {
		var raw string
		var w Workflow
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(raw), &w); e != nil {
			return nil, e
		}
		if workflowAllowed(c, w) {
			result = append(result, w)
		}
	}
	return result, rows.Err()
}
func (s *Store) SaveWorkflow(c Caller, w Workflow) (Workflow, error) {
	if !c.Admin {
		return w, ErrForbidden
	}
	w.Name = strings.TrimSpace(w.Name)
	if w.MaxSteps == 0 {
		w.MaxSteps = 100
	}
	if e := validateWorkflow(w); e != nil {
		return w, e
	}
	if w.ID == "" {
		if w.Revision != 0 {
			return w, ErrConflict
		}
		w.ID = newID()
	} else if !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(w.ID) || w.Revision < 1 {
		return w, ErrConflict
	}
	tx, e := s.DB.Begin()
	if e != nil {
		return w, e
	}
	defer tx.Rollback()
	for _, id := range w.AuthorizedUsers {
		var n int
		if e = tx.QueryRow(`SELECT count(*) FROM users WHERE user_id=?`, id).Scan(&n); e != nil {
			return w, e
		}
		if n != 1 {
			return w, fmt.Errorf("authorized user does not exist: %s", id)
		}
	}
	for _, n := range w.Nodes {
		if n.Kind == "connector" {
			v, err := loadConnector(tx, n.ConnectorID)
			if err != nil {
				return w, err
			}
			if err = validateConnectorNode(v, n, w); err != nil {
				return w, fmt.Errorf("node %s: %w", n.ID, err)
			}
		}
		if n.Kind == "agent" {
			if n.Agent != nil {
				if err := validateWorkflowAgent(tx, *n.Agent); err != nil {
					return w, fmt.Errorf("node %s: %w", n.ID, err)
				}
				continue
			}
			var count int
			if e = tx.QueryRow(`SELECT count(*) FROM agents WHERE id=?`, n.AgentID).Scan(&count); e != nil {
				return w, e
			}
			if count != 1 {
				return w, fmt.Errorf("agent does not exist for node %s", n.ID)
			}
		}
	}
	for _, hook := range w.Hooks {
		v, err := loadConnector(tx, hook.ConnectorID)
		if err != nil {
			return w, err
		}
		if err = validateHookAction(v, hook, w); err != nil {
			return w, err
		}
	}
	previous := w.Revision
	w.Revision++
	w.Updated = now()
	raw, e := json.Marshal(w)
	if e != nil {
		return w, e
	}
	if previous == 0 {
		_, e = tx.Exec(`INSERT INTO workflows(id,revision,definition) VALUES(?,?,?)`, w.ID, w.Revision, string(raw))
	} else {
		var result sql.Result
		result, e = tx.Exec(`UPDATE workflows SET revision=?,definition=? WHERE id=? AND revision=?`, w.Revision, string(raw), w.ID, previous)
		if e == nil {
			var count int64
			count, e = result.RowsAffected()
			if e == nil && count != 1 {
				return w, fmt.Errorf("%w: workflow changed; reload before saving", ErrConflict)
			}
		}
	}
	if e != nil {
		return w, e
	}
	return w, tx.Commit()
}
func (h *Server) workflowRoutes() {
	h.mux.HandleFunc("GET /api/workflows", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		v, e := h.store.Workflows(c)
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, workflowHTTPValue(c, v))
	}))
	h.mux.HandleFunc("GET /api/workflows/{id}", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		v, e := h.store.Workflow(c, r.PathValue("id"))
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, workflowHTTPValue(c, v))
	}))
	save := h.protect(true, func(w http.ResponseWriter, r *http.Request, c Caller) {
		var v Workflow
		if e := decode(w, r, &v); e != nil {
			fail(w, e)
			return
		}
		if r.Method == "POST" {
			if v.ID != "" || v.Revision != 0 {
				fail(w, errors.New("new workflow must not include id or revision"))
				return
			}
		} else {
			if v.ID != "" && v.ID != r.PathValue("id") {
				fail(w, errors.New("workflow id differs from URL"))
				return
			}
			v.ID = r.PathValue("id")
		}
		v, e := h.store.SaveWorkflow(c, v)
		if e != nil {
			fail(w, e)
			return
		}
		code := 200
		if r.Method == "POST" {
			code = 201
		}
		respond(w, code, workflowHTTPValue(c, v))
	})
	h.mux.HandleFunc("POST /api/workflows", save)
	h.mux.HandleFunc("PUT /api/workflows/{id}", save)
}
