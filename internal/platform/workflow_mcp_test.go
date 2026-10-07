package platform

import (
	"context"
	"encoding/json"
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
	if e != nil || len(list.Tools) != 3 || list.Tools[0].Name != "complete_node" {
		t.Fatal(list, e)
	}
	result, e := session.CallTool(ctx, &mcp.CallToolParams{Name: "complete_node", Arguments: map[string]any{"route": "approved", "summary": "完成"}})
	if e != nil || result.IsError {
		t.Fatal(result, e)
	}
	s.Complete(conv.ID, msg.ID, "completed", "")
	h.workflows.Tick(ctx)
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "completed" {
		t.Fatal(r)
	}
	result, e = session.CallTool(ctx, &mcp.CallToolParams{Name: "complete_node", Arguments: map[string]any{"route": "approved", "summary": "完成"}})
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
	output := "actual middle failure\n"
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
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "read_command_output", Arguments: map[string]any{"seq": 2}})
	if err == nil && !result.IsError {
		t.Fatal("read current/future execution")
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "complete_node", Arguments: map[string]any{"route": "approved", "summary": "done"}})
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
