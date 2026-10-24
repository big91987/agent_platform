package platform

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestWorkflowPreviewShowsCompleteSystemInstructionsAndSeparateInput(t *testing.T) {
	s := testStore(t)
	if err := s.initAuth("password"); err != nil {
		t.Fatal(err)
	}
	token, err := s.UserToken("admin", false)
	if err != nil {
		t.Fatal(err)
	}
	w := independentWorkflow(t, map[string]any{"executor": "codex", "instructions": "ROLE_MARKER", "native_config": "developer_instructions='NATIVE_MARKER'"})
	w.ContextVersion = 2
	w.Nodes[0].Prompt = ""
	h := NewServer(s, nil, nil, "password", "http://localhost")
	out := requestJSON(t, h, "POST", "/api/workflows/preview", token, map[string]any{"workflow": w, "node_id": "worker"})
	if out.Code != http.StatusOK {
		t.Fatal(out.Code, out.Body.String())
	}
	var result map[string]any
	if err = json.Unmarshal(out.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	system, _ := result["system_prompt"].(string)
	for _, value := range []string{"NATIVE_MARKER", "ROLE_MARKER", "This turn is a conversation with the user in Agent Platform."} {
		if !strings.Contains(system, value) {
			t.Fatalf("system preview omitted %q: %s", value, system)
		}
	}
	input, _ := result["input"].(string)
	if !strings.Contains(input, "handoff") || strings.Contains(system, "## 当前任务") || strings.Contains(input, "ROLE_MARKER") {
		t.Fatal("system and user input are mixed", result)
	}
}

func TestWorkflowPreviewProtectsCredentialsAndSharedAgentAccess(t *testing.T) {
	s := testStore(t)
	s.initAuth("password")
	admin, _ := s.userCaller("admin")
	token, _ := s.UserToken("admin", false)
	secret := "private-preview-token-value"
	config := map[string]any{"executor": "codex", "instructions": "Use " + secret, "native_config": "model_provider='private-provider'", "env": map[string]string{"API_KEY": secret}}
	w := independentWorkflow(t, config)
	w.ContextVersion = 2
	w.Nodes[0].Prompt = ""
	h := NewServer(s, nil, &Codex{AuthHome: t.TempDir()}, "password", "http://localhost")
	out := requestJSON(t, h, "POST", "/api/workflows/preview", token, map[string]any{"workflow": w, "node_id": "worker"})
	if out.Code != 200 || strings.Contains(out.Body.String(), secret) || !strings.Contains(out.Body.String(), "[redacted]") || strings.Contains(out.Body.String(), "private-provider") {
		t.Fatal(out.Code, out.Body.String())
	}
	a := testAgent(t, s)
	_, callerToken := testUser(t, s, &a, "preview-caller")
	w.AuthorizedUsers = []string{"preview-caller"}
	saved, err := s.SaveWorkflow(admin, w)
	if err != nil {
		t.Fatal(err)
	}
	private := testAgent(t, s)
	private.Instructions = "UNGRANTED_ROLE"
	private, err = s.SaveAgent(private)
	if err != nil {
		t.Fatal(err)
	}
	sanitized := workflowHTTPValue(Caller{}, saved).(Workflow)
	out = requestJSON(t, h, "POST", "/api/workflows/preview", callerToken, map[string]any{"workflow": sanitized, "node_id": "worker"})
	if out.Code != 200 || !strings.Contains(out.Body.String(), "Use [redacted]") {
		t.Fatal("authorized caller saw an incomplete system preview", out.Code, out.Body.String())
	}
	saved.Nodes[0].Agent = nil
	saved.Nodes[0].AgentID = private.ID
	saved, err = s.SaveWorkflow(admin, saved)
	if err != nil {
		t.Fatal(err)
	}
	out = requestJSON(t, h, "POST", "/api/workflows/preview", callerToken, map[string]any{"workflow": saved, "node_id": "worker"})
	if out.Code != http.StatusForbidden || strings.Contains(out.Body.String(), "UNGRANTED_ROLE") {
		t.Fatal(out.Code, out.Body.String())
	}
}
