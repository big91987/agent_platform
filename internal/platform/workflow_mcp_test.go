package platform

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkflowMCPAuthenticatesAndCompletesOnlyItsNode(t *testing.T) {
	s := testStore(t)
	c, r := agentRun(t, s)
	h := NewServer(s, nil, nil, "password", "http://localhost")
	remote := httptest.NewServer(h)
	defer remote.Close()
	h.workflows.base = remote.URL
	h.workflows.Tick(context.Background())
	conv, msg, e := s.Claim()
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connect := func(token string) (*mcp.ClientSession, error) {
		return mcp.NewClient(&mcp.Implementation{Name: "workflow-proof", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: remote.URL + "/api/workflow-node/mcp", HTTPClient: &http.Client{Transport: bearerTransport{http.DefaultTransport, token}}}, nil)
	}
	if session, e := connect("wrong"); e == nil {
		session.Close()
		t.Fatal("unscoped client authorized")
	}
	session, e := connect(*conv.Snapshot.Env[workflowTokenEnv])
	if e != nil {
		t.Fatal(e)
	}
	defer session.Close()
	list, e := session.ListTools(ctx, nil)
	if e != nil || len(list.Tools) != 3 || list.Tools[0].Name != "handoff" {
		t.Fatal(list, e)
	}
	result, e := session.CallTool(ctx, &mcp.CallToolParams{Name: "handoff", Arguments: map[string]any{"target": "done", "message": "完成"}})
	if e != nil || result.IsError {
		t.Fatal(result, e)
	}
	s.Complete(conv.ID, msg.ID, "completed", "")
	h.workflows.Tick(ctx)
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "completed" {
		t.Fatal(r)
	}
	result, e = session.CallTool(ctx, &mcp.CallToolParams{Name: "handoff", Arguments: map[string]any{"target": "done", "message": "完成"}})
	if e == nil && !result.IsError {
		t.Fatal("old MCP execution remained authorized")
	}
}

func TestWorkflowMCPReadsOnlyPriorLogsWhileCurrent(t *testing.T) {
	s := testStore(t)
	c, r := agentRun(t, s)
	// Fixture history: one completed command, then the current Agent execution.
	if _, err := s.DB.Exec(`UPDATE workflow_steps SET seq=2 WHERE run_id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`UPDATE workflow_runs SET seq=2 WHERE id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`INSERT INTO workflow_steps(run_id,seq,node_id,status,created,updated) VALUES(?,1,'prior','completed',?,?)`, r.ID, now(), now()); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(s.Dir, "workflow-processes", r.ID+"-1")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	output := "actual middle failure\n" + strings.Repeat("中x", 20000)
	if err := os.WriteFile(filepath.Join(home, "output.log"), []byte(output), 0600); err != nil {
		t.Fatal(err)
	}
	receipt, _ := json.Marshal(ConnectorReceipt{Kind: "command", Log: &ConnectorLogInfo{Bytes: int64(len(output))}})
	if _, err := s.DB.Exec(`INSERT INTO workflow_connector_actions(run_id,seq,request,receipt) VALUES(?,?,?,?)`, r.ID, 1, `{}`, string(receipt)); err != nil {
		t.Fatal(err)
	}
	h := NewServer(s, nil, nil, "password", "http://localhost")
	remote := httptest.NewServer(h)
	defer remote.Close()
	h.workflows.base = remote.URL
	h.workflows.Tick(context.Background())
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "log-proof", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: remote.URL + "/api/workflow-node/mcp", HTTPClient: &http.Client{Transport: bearerTransport{http.DefaultTransport, *conv.Snapshot.Env[workflowTokenEnv]}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "read_command_output", Arguments: map[string]any{"seq": 1}})
	if err != nil || result.IsError {
		t.Fatal(result, err)
	}
	raw, _ := json.Marshal(result)
	if !strings.Contains(string(raw), "actual middle failure") {
		t.Fatal("missing prior log")
	}
	// Export is a write capability even when the source is a read-only log.
	for _, column := range []string{"read_only", "snapshot"} {
		var original string
		if err := s.DB.QueryRow("SELECT "+column+" FROM conversations WHERE id=?", conv.ID).Scan(&original); err != nil {
			t.Fatal(err)
		}
		value := "1"
		if column == "snapshot" {
			frozen := conv.Snapshot
			frozen.Sandbox = "read-only"
			b, _ := json.Marshal(frozen)
			value = string(b)
		}
		if _, err := s.DB.Exec("UPDATE conversations SET "+column+"=? WHERE id=?", value, conv.ID); err != nil {
			t.Fatal(err)
		}
		denied, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "read_command_output", Arguments: map[string]any{"seq": 1, "save_to_workspace": true}})
		if err == nil && !denied.IsError {
			t.Fatal("read-only conversation exported workspace file", column)
		}
		read, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "read_command_output", Arguments: map[string]any{"seq": 1}})
		if err != nil || read.IsError {
			t.Fatal("read-only conversation lost paging", err)
		}
		s.DB.Exec("UPDATE conversations SET "+column+"=? WHERE id=?", original, conv.ID)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "read_command_output", Arguments: map[string]any{"seq": 1, "save_to_workspace": true}})
	if err != nil || result.IsError {
		t.Fatal("complete retained log could not be exported", result, err)
	}
	invalid, invalidErr := session.CallTool(ctx, &mcp.CallToolParams{Name: "read_command_output", Arguments: map[string]any{"seq": 1, "offset": 1, "save_to_workspace": true}})
	if invalidErr == nil && !invalid.IsError {
		t.Fatal("partial offset accepted for complete export")
	}
	var exported struct {
		Path      string `json:"path"`
		Bytes     int64  `json:"bytes"`
		SHA256    string `json:"sha256"`
		Truncated bool   `json:"truncated"`
	}
	b, _ := json.Marshal(result.StructuredContent)
	if err = json.Unmarshal(b, &exported); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(conv.WorkspacePath, exported.Path))
	if err != nil || string(got) != output || exported.Bytes != int64(len(output)) || exported.Truncated || exported.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(output))) {
		t.Fatal("export lost retained bytes or provenance", exported, err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "read_command_output", Arguments: map[string]any{"seq": 1, "save_to_workspace": true}})
	if err != nil || result.IsError {
		t.Fatal("identical export could not be reconciled", result, err)
	}
	if err = os.WriteFile(filepath.Join(conv.WorkspacePath, exported.Path), []byte("conflict"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "read_command_output", Arguments: map[string]any{"seq": 1, "save_to_workspace": true}})
	if err == nil && !result.IsError {
		t.Fatal("changed export silently overwritten")
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "read_command_output", Arguments: map[string]any{"seq": 2, "save_to_workspace": true}})
	if err == nil && !result.IsError {
		t.Fatal("read current/future execution")
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "handoff", Arguments: map[string]any{"target": "done", "message": "done"}})
	if err != nil || result.IsError {
		t.Fatal(result, err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "read_command_output", Arguments: map[string]any{"seq": 1}})
	if err == nil && !result.IsError {
		t.Fatal("sealed node retained log capability")
	}
	if err := s.Complete(conv.ID, msg.ID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	h.workflows.Tick(ctx)
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "completed" {
		t.Fatal(r.Status)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "read_command_output", Arguments: map[string]any{"seq": 1}})
	if err == nil && !result.IsError {
		t.Fatal("stale execution retained log capability")
	}
}
