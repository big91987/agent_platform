package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisteredToolServer stores a connection to external tools, never their implementation.
type RegisteredToolServer struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Enabled    bool          `json:"enabled"`
	Connection MCPConnection `json:"connection"`
	Tools      []*mcp.Tool   `json:"tools,omitempty"`
	CheckedAt  string        `json:"checked_at,omitempty"`
}
type MCPConnection struct {
	URL               string   `json:"url,omitempty"`
	Command           string   `json:"command,omitempty"`
	Args              []string `json:"args,omitempty"`
	CWD               string   `json:"cwd,omitempty"`
	EnvVars           []string `json:"env_vars,omitempty"`
	BearerTokenEnvVar string   `json:"bearer_token_env_var,omitempty"`
}
type ToolBinding struct {
	ServerID  string            `json:"server_id"`
	Tools     []string          `json:"tools"`
	Approvals map[string]string `json:"approvals,omitempty"`
}
type ResolvedToolServer struct {
	Connection MCPConnection     `json:"connection"`
	Tools      []string          `json:"tools"`
	Approvals  map[string]string `json:"approvals,omitempty"`
}

var toolServerName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateMCPConnection(c MCPConnection) error {
	if c.CWD != "" && !filepath.IsAbs(c.CWD) {
		return errors.New("MCP cwd must be absolute")
	}
	if (c.URL == "") == (c.Command == "") {
		return errors.New("choose either an HTTP URL or a stdio command")
	}
	if c.URL != "" {
		u, err := url.Parse(c.URL)
		if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
			return errors.New("invalid MCP HTTP URL")
		}
		if len(c.Args) > 0 || c.CWD != "" || len(c.EnvVars) > 0 {
			return errors.New("command arguments, cwd and env_vars apply only to stdio")
		}
	} else if c.BearerTokenEnvVar != "" {
		return errors.New("bearer authentication applies only to HTTP MCP")
	}
	for _, name := range append(append([]string{}, c.EnvVars...), c.BearerTokenEnvVar) {
		if name != "" && (!envName.MatchString(name) || name == "HOME" || strings.HasPrefix(name, "CODEX_") || strings.HasPrefix(name, "AGENT_PLATFORM_")) {
			return errors.New("invalid credential/environment variable name")
		}
	}
	return nil
}
func (s *Store) initToolServers() error {
	_, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS tool_servers(id TEXT PRIMARY KEY,config TEXT NOT NULL);`)
	return err
}
func (s *Store) ToolServers() ([]RegisteredToolServer, error) {
	rows, err := s.DB.Query(`SELECT config FROM tool_servers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RegisteredToolServer{}
	for rows.Next() {
		var raw string
		var item RegisteredToolServer
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) ToolServer(id string) (RegisteredToolServer, error) {
	var v RegisteredToolServer
	var raw string
	err := s.DB.QueryRow(`SELECT config FROM tool_servers WHERE id=?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, err
	}
	err = json.Unmarshal([]byte(raw), &v)
	return v, err
}
func (s *Store) SaveToolServer(v RegisteredToolServer) (RegisteredToolServer, error) {
	if !toolServerName.MatchString(v.ID) || strings.TrimSpace(v.Name) == "" {
		return v, errors.New("name and a lowercase server ID are required")
	}
	if err := validateMCPConnection(v.Connection); err != nil {
		return v, err
	}
	// Inventory must come from discovery, not an API client's declaration.
	v.Tools = nil
	v.CheckedAt = ""
	if old, err := s.ToolServer(v.ID); err == nil {
		a, _ := json.Marshal(old.Connection)
		b, _ := json.Marshal(v.Connection)
		if string(a) == string(b) {
			v.Tools = old.Tools
			v.CheckedAt = old.CheckedAt
		}
	} else if !errors.Is(err, ErrNotFound) {
		return v, err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return v, err
	}
	_, err = s.DB.Exec(`INSERT INTO tool_servers VALUES(?,?) ON CONFLICT(id) DO UPDATE SET config=excluded.config`, v.ID, string(raw))
	return v, err
}

type rowQuerier interface{ QueryRow(string, ...any) *sql.Row }

func resolveToolServers(q rowQuerier, a Agent) (Agent, error) {
	a.ResolvedTools = nil
	for _, binding := range a.ToolServers {
		if len(binding.Tools) == 0 {
			return a, errors.New("select at least one tool from each registered server")
		}
		if _, exists := a.ResolvedTools[binding.ServerID]; exists {
			return a, errors.New("duplicate tool server binding")
		}
		var raw string
		if err := q.QueryRow(`SELECT config FROM tool_servers WHERE id=?`, binding.ServerID).Scan(&raw); err != nil {
			return a, fmt.Errorf("tool server %s is unavailable", binding.ServerID)
		}
		var server RegisteredToolServer
		if err := json.Unmarshal([]byte(raw), &server); err != nil {
			return a, err
		}
		if !server.Enabled {
			return a, fmt.Errorf("tool server %s is disabled", binding.ServerID)
		}
		available := map[string]bool{}
		for _, t := range server.Tools {
			if t == nil || t.Name == "" {
				return a, errors.New("invalid discovered tool inventory; rediscover this service")
			}
			available[t.Name] = true
		}
		seen := map[string]bool{}
		for _, name := range binding.Tools {
			if !available[name] || seen[name] {
				return a, fmt.Errorf("tool %s is not in the discovered inventory or is duplicated", name)
			}
			seen[name] = true
		}
		for name, mode := range binding.Approvals {
			if !seen[name] || (mode != "auto" && mode != "confirm") {
				return a, errors.New("approval mode must be auto or confirm for a selected tool")
			}
		}
		if a.ResolvedTools == nil {
			a.ResolvedTools = map[string]ResolvedToolServer{}
		}
		a.ResolvedTools[binding.ServerID] = ResolvedToolServer{Connection: server.Connection, Tools: binding.Tools, Approvals: binding.Approvals}
	}
	return a, nil
}
func (r ResolvedToolServer) native() map[string]any {
	c := r.Connection
	permissions := map[string]any{}
	for _, name := range r.Tools {
		mode := "approve"
		if r.Approvals[name] == "confirm" {
			mode = "prompt"
		}
		permissions[name] = map[string]any{"approval_mode": mode}
	}
	out := map[string]any{"enabled_tools": r.Tools, "required": true, "tools": permissions}
	if c.URL != "" {
		out["url"] = c.URL
		if c.BearerTokenEnvVar != "" {
			out["bearer_token_env_var"] = c.BearerTokenEnvVar
		}
	} else {
		out["command"] = c.Command
		if len(c.Args) > 0 {
			out["args"] = c.Args
		}
		if c.CWD != "" {
			out["cwd"] = c.CWD
		}
		if len(c.EnvVars) > 0 {
			out["env_vars"] = c.EnvVars
		}
	}
	return out
}

type bearerTransport struct {
	base  http.RoundTripper
	token string
}

func (t bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	copy.Header = r.Header.Clone()
	copy.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(copy)
}
func discoverTools(ctx context.Context, c MCPConnection) ([]*mcp.Tool, error) {
	if err := validateMCPConnection(c); err != nil {
		return nil, err
	}
	var transport mcp.Transport
	if c.URL != "" {
		client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		if c.BearerTokenEnvVar != "" {
			token := os.Getenv(c.BearerTokenEnvVar)
			if token == "" {
				return nil, errors.New("configured MCP bearer credential is missing from backend environment")
			}
			client.Transport = bearerTransport{http.DefaultTransport, token}
		}
		transport = &mcp.StreamableClientTransport{Endpoint: c.URL, HTTPClient: client}
	} else {
		command := exec.CommandContext(ctx, c.Command, c.Args...)
		command.Dir = c.CWD
		command.Env = []string{}
		for _, name := range append([]string{"PATH", "HOME", "USER", "TMPDIR", "LANG"}, c.EnvVars...) {
			if value, ok := os.LookupEnv(name); ok {
				command.Env = append(command.Env, name+"="+value)
			}
		}
		transport = &mcp.CommandTransport{Command: command}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "agent-platform-discovery", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, errors.New("MCP connection failed; check address, command and credentials")
	}
	defer session.Close()
	tools := []*mcp.Tool{}
	cursor := ""
	seenCursors := map[string]bool{}
	seenTools := map[string]bool{}
	for {
		page, err := session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, errors.New("MCP tools/list failed")
		}
		for _, tool := range page.Tools {
			if tool == nil || tool.Name == "" || seenTools[tool.Name] {
				return nil, errors.New("MCP inventory contains an empty or duplicate tool")
			}
			seenTools[tool.Name] = true
		}
		tools = append(tools, page.Tools...)
		if len(tools) > 2000 {
			return nil, errors.New("MCP inventory exceeds 2000 tools")
		}
		if page.NextCursor == "" {
			return tools, nil
		}
		if page.NextCursor == cursor || seenCursors[page.NextCursor] {
			return nil, errors.New("MCP pagination did not advance")
		}
		seenCursors[page.NextCursor] = true
		cursor = page.NextCursor
	}
}
func (h *Server) toolServers(w http.ResponseWriter, r *http.Request, c Caller) {
	if r.Method == "GET" {
		v, err := h.store.ToolServers()
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, v)
		return
	}
	var input RegisteredToolServer
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	if id := r.PathValue("id"); id != "" {
		input.ID = id
	}
	result, err := h.store.SaveToolServer(input)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, result)
}
func (h *Server) discoverToolServer(w http.ResponseWriter, r *http.Request, c Caller) {
	v, err := h.store.ToolServer(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	tools, err := discoverTools(ctx, v.Connection)
	if err != nil {
		fail(w, err)
		return
	}
	// Do not overwrite configuration changed while discovery was running.
	old, _ := json.Marshal(v)
	v.Tools = tools
	v.CheckedAt = now()
	raw, _ := json.Marshal(v)
	changed, err := h.store.DB.Exec(`UPDATE tool_servers SET config=? WHERE id=? AND config=?`, string(raw), v.ID, string(old))
	if err != nil {
		fail(w, err)
		return
	}
	n, _ := changed.RowsAffected()
	if n != 1 {
		fail(w, ErrConflict)
		return
	}
	respond(w, 200, v)
}

func needsToolConfirmation(a Agent) bool {
	for _, server := range a.ResolvedTools {
		for _, mode := range server.Approvals {
			if mode == "confirm" {
				return true
			}
		}
	}
	return false
}
