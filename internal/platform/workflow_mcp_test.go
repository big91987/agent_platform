package platform

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
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
	if e != nil || len(list.Tools) != 2 || list.Tools[0].Name != "complete_node" {
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
