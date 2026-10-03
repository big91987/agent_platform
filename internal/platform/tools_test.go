package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExternalMCPDiscoveryBindingAndSnapshot(t *testing.T) {
	service := mcp.NewServer(&mcp.Implementation{Name: "external-test", Version: "1"}, nil)
	type args struct {
		Message string `json:"message"`
	}
	mcp.AddTool(service, &mcp.Tool{Name: "deliver", Description: "An external operation"}, func(ctx context.Context, r *mcp.CallToolRequest, a args) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: a.Message}}}, nil, nil
	})
	remote := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return service }, nil))
	defer remote.Close()
	root := t.TempDir()
	s, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	a := testAgent(t, s)
	_, token := testUser(t, s, &a, "tool-user")
	api := NewServer(s, nil, nil, "password", "http://localhost")
	if w := requestJSON(t, api, "GET", "/api/tool-servers", token, nil); w.Code != 403 {
		t.Fatalf("nonadmin inventory: %d", w.Code)
	}
	v, err := s.SaveToolServer(RegisteredToolServer{ID: "external", Name: "External", Enabled: true, Connection: MCPConnection{URL: remote.URL}, Tools: []*mcp.Tool{{Name: "forged"}}})
	if err != nil || len(v.Tools) != 0 {
		t.Fatalf("untrusted inventory: %+v %v", v, err)
	}
	req := httptest.NewRequest("POST", "/api/tool-servers/external/discover", nil)
	req.SetPathValue("id", "external")
	w := httptest.NewRecorder()
	api.discoverToolServer(w, req, Caller{Admin: true, Source: "console"})
	if w.Code != 200 {
		t.Fatalf("discover: %s", w.Body.String())
	}
	v, err = s.ToolServer("external")
	if err != nil || len(v.Tools) != 1 || v.Tools[0].Name != "deliver" || v.CheckedAt == "" {
		t.Fatalf("inventory: %+v %v", v, err)
	}
	a.ToolServers = []ToolBinding{{ServerID: v.ID, Tools: []string{"forged"}}}
	if _, err = s.SaveAgent(a); err == nil {
		t.Fatal("accepted unknown tool")
	}
	a.ToolServers[0].Tools = []string{"deliver"}
	a.ToolServers[0].Approvals = map[string]string{"unknown": "confirm"}
	if _, err = s.SaveAgent(a); err == nil {
		t.Fatal("accepted approval for unselected tool")
	}
	a.ToolServers[0].Approvals = map[string]string{"deliver": "invalid"}
	if _, err = s.SaveAgent(a); err == nil {
		t.Fatal("accepted invalid approval mode")
	}
	a.ToolServers[0].Approvals = nil

	a, err = s.SaveAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := s.Submit(Caller{Admin: true, Source: "console"}, Input{AgentID: a.ID, UserID: "operator", Message: "use external tool"})
	if err != nil {
		t.Fatal(err)
	}
	conv, err := s.Conversation(receipt.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	config, err := nativeConfig(conv.Snapshot)
	if err != nil || !strings.Contains(string(config), "registered_external") || !strings.Contains(string(config), "enabled_tools = ['deliver']") {
		t.Fatalf("native binding: %s %v", config, err)
	}
	if !strings.Contains(string(config), "approval_mode = 'approve'") {
		t.Fatal("automatic mode missing from native config")
	}
	confirmed := conv.Snapshot.ResolvedTools[v.ID]
	confirmed.Approvals = map[string]string{"deliver": "confirm"}
	conv.Snapshot.ResolvedTools[v.ID] = confirmed
	config, err = nativeConfig(conv.Snapshot)
	if err != nil || !strings.Contains(string(config), "approval_mode = 'prompt'") || !needsToolConfirmation(conv.Snapshot) {
		t.Fatalf("confirmation policy: %s %v", config, err)
	}
	// Reopen storage to prove recovery uses persisted bindings rather than memory.
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	restoredAgent, err := s.Agent(a.ID)
	if err != nil || len(restoredAgent.ToolServers) != 1 {
		t.Fatalf("agent binding lost on restart: %+v %v", restoredAgent, err)
	}
	restoredServer, err := s.ToolServer(v.ID)
	if err != nil || len(restoredServer.Tools) != 1 {
		t.Fatalf("inventory lost on restart: %+v %v", restoredServer, err)
	}
	v.Enabled = false
	_, err = s.SaveToolServer(v)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Submit(Caller{Admin: true, Source: "console"}, Input{AgentID: a.ID, UserID: "operator", Message: "new"}); err == nil {
		t.Fatal("disabled server allowed new conversation")
	}
	existing, _ := s.Conversation(receipt.ConversationID)
	if existing.Snapshot.ResolvedTools["external"].Connection.URL != remote.URL {
		t.Fatal("existing snapshot changed")
	}
	// Registry is persisted independently of a connection's lifetime.
	var stored string
	if err = s.DB.QueryRow(`SELECT config FROM tool_servers WHERE id='external'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var restored RegisteredToolServer
	json.Unmarshal([]byte(stored), &restored)
	if len(restored.Tools) != 1 {
		t.Fatal("discovered inventory lost on save")
	}
	v.Connection.URL = remote.URL + "/new"
	v, _ = s.SaveToolServer(v)
	if len(v.Tools) != 0 {
		t.Fatal("stale inventory retained for changed endpoint")
	}
}
func TestRegisteredEnvironmentHonorsExplicitOverrides(t *testing.T) {
	t.Setenv("EXTERNAL_TOOL_TOKEN", "inherited")
	value := "overridden"
	a := Agent{Env: map[string]*string{"EXTERNAL_TOOL_TOKEN": &value}, ResolvedTools: map[string]ResolvedToolServer{"test": {Connection: MCPConnection{URL: "http://example.test", BearerTokenEnvVar: "EXTERNAL_TOOL_TOKEN"}}}}
	env, err := executorEnv(a, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(env, "\n"), "EXTERNAL_TOOL_TOKEN=overridden") {
		t.Fatal("registry overrode Agent environment")
	}
	a.Env["EXTERNAL_TOOL_TOKEN"] = nil
	env, err = executorEnv(a, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(env, "\n"), "EXTERNAL_TOOL_TOKEN=") {
		t.Fatal("clear ignored")
	}
}
