package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func independentWorkflow(t *testing.T, config map[string]any) Workflow {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"name": "独立节点", "enabled": true, "context_version": 1, "entry": "worker", "max_steps": 10, "nodes": []any{map[string]any{"id": "worker", "name": "研发", "kind": "agent", "agent": config, "prompt": "完成节点工作", "continuation_limit": 1}, map[string]any{"id": "done", "name": "结束", "kind": "end"}}, "edges": []any{map[string]any{"source": "worker", "target": "done", "route": "next", "mode": "automatic"}}})
	if err != nil {
		t.Fatal(err)
	}
	var w Workflow
	if err = json.Unmarshal(raw, &w); err != nil {
		t.Fatal(err)
	}
	return w
}

func TestIndependentNodeExecutionFreezesConfigAndUsesWorkflowPermission(t *testing.T) {
	s := testStore(t)
	s.initAuth("password")
	admin, _ := s.userCaller("admin")
	u, err := s.SaveUser(User{Username: "node-user", UserID: "node-user", Role: "caller", Enabled: true, Password: "test-password-123"})
	if err != nil {
		t.Fatal(err)
	}
	caller, _ := s.userCaller(u.Username)
	w := independentWorkflow(t, map[string]any{"executor": "codex", "model": "original-model", "instructions": "original role", "sandbox": "read-only"})
	w.AuthorizedUsers = []string{u.UserID}
	w, err = s.SaveWorkflow(admin, w)
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: w.ID, Input: "task", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	changed := independentWorkflow(t, map[string]any{"executor": "codex", "model": "changed-model", "instructions": "changed role"})
	changed.ID = w.ID
	changed.Revision = w.Revision
	changed.AuthorizedUsers = w.AuthorizedUsers
	changed, err = s.SaveWorkflow(admin, changed)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	run, _ = s.WorkflowRun(caller, run.ID)
	if run.Steps[0].ConversationID == "" {
		t.Fatalf("no independent node conversation: %+v", run)
	}
	conv, err := s.Authorize(caller, run.Steps[0].ConversationID, "")
	if err != nil {
		t.Fatal(err)
	}
	if conv.AgentID != "" || conv.Snapshot.Model != "original-model" || conv.Snapshot.Instructions != "original role" {
		t.Fatalf("did not freeze node settings: %+v", conv)
	}
	agents, _ := s.Agents()
	if len(agents) != 0 {
		t.Fatal("created hidden shared Agents")
	}
	conversations, err := s.Conversations(caller, "")
	if err != nil || len(conversations) != 1 {
		t.Fatal(conversations, err)
	}
	if _, err = s.Submit(caller, Input{ConversationID: conv.ID, Message: "clarification", RequestID: "reply"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authorize(Caller{UserID: "other"}, conv.ID, ""); !errors.Is(err, ErrForbidden) {
		t.Fatal("cross-user access", err)
	}
	changed.AuthorizedUsers = nil
	if _, err = s.SaveWorkflow(admin, changed); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authorize(caller, conv.ID, ""); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked read allowed", err)
	}
	if _, err = s.Submit(caller, Input{ConversationID: conv.ID, Message: "clarification", RequestID: "reply"}); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked duplicate accepted", err)
	}
	if _, _, err = s.Claim(); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked queued work started", err)
	}
}

func TestIndependentNodeRejectsInvalidExecutionConfiguration(t *testing.T) {
	for _, field := range []string{"id", "authorized_users", "resolved_tools", "executor", "native_config", "skills"} {
		t.Run(field, func(t *testing.T) {
			s := testStore(t)
			config := map[string]any{"executor": "codex"}
			switch field {
			case "id":
				config[field] = "shared-agent"
			case "authorized_users":
				config[field] = []string{"anyone"}
			case "resolved_tools":
				config[field] = map[string]any{"injected": map[string]any{}}
			case "executor":
				config[field] = "unsupported"
			case "native_config":
				config[field] = "sandbox_mode='danger-full-access'"
			case "skills":
				config[field] = []string{"/does-not-exist/skill"}
			}
			if _, err := s.SaveWorkflow(Caller{Admin: true}, independentWorkflow(t, config)); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestIndependentNodeWaitResumeAndPermissionReset(t *testing.T) {
	s := testStore(t)
	s.initAuth("password")
	admin, _ := s.userCaller("admin")
	w, err := s.SaveWorkflow(admin, independentWorkflow(t, map[string]any{"executor": "codex", "network_access": true}))
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(admin, WorkflowStart{WorkflowID: w.ID, Input: "task", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.waitWorkflowNode(r.ID, 1, *conv.Snapshot.Env[workflowTokenEnv], WorkflowWait{Kind: "clarification", Reason: "Confirm scope"}); err != nil {
		t.Fatal(err)
	}
	if err = s.Complete(conv.ID, msg.ID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	r, _ = s.WorkflowRun(admin, r.ID)
	if r.Status != "waiting" {
		t.Fatal(r.Status)
	}
	w.Nodes[0].Agent.NetworkAccess = false
	if _, err = s.SaveWorkflow(admin, w); err != nil {
		t.Fatal(err)
	}
	scheduler := NewScheduler(s, nil, 1)
	if err = scheduler.SetConversationPermissions(conv.ID, false, false, nil); err != nil {
		t.Fatal(err)
	}
	if err = scheduler.ApplyAgentPermissions(conv.ID); err != nil {
		t.Fatal(err)
	}
	restored, _ := s.Conversation(conv.ID)
	if !restored.Snapshot.NetworkAccess {
		t.Fatal("permission reset used edited workflow instead of frozen Run")
	}
	if err = engine.Stop(admin, r.ID, r.Seq); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	if err = engine.Resume(admin, r.ID, r.Seq, "Confirmed scope; continue"); err != nil {
		t.Fatal(err)
	}
	next, input, err := s.Claim()
	if err != nil || next.ID != conv.ID || input.Content != "Confirmed scope; continue" {
		t.Fatal(next.ID, input.Content, err)
	}
	if err = s.completeWorkflowNode(Caller{}, r.ID, r.Seq, *next.Snapshot.Env[workflowTokenEnv], NodeResult{Route: "next", Summary: "Verified"}); err != nil {
		t.Fatal(err)
	}
	if err = s.Complete(next.ID, input.ID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	r, _ = s.WorkflowRun(admin, r.ID)
	if r.Status != "completed" {
		t.Fatal(r.Status, r.Error)
	}
}

func TestNodeConversationAuthorizationNeverGrantsAnUnlinkedEmptyAgent(t *testing.T) {
	s := testStore(t)
	for _, caller := range []Caller{{UserID: "owner"}, {UserID: "owner", Agents: []string{""}}, {Source: "workflow", UserID: "owner"}} {
		if conversationAgentAllowed(s.DB, caller, "not-a-workflow-conversation", "") {
			t.Fatal("empty agent or fabricated source granted access")
		}
	}
	if conversationAgentAllowed(s.DB, Caller{UserID: "owner"}, "unknown", "shared-agent") {
		t.Fatal("legacy shared Agent grant bypassed")
	}
	if !conversationAgentAllowed(s.DB, Caller{UserID: "owner", Agents: []string{"shared-agent"}}, "unknown", "shared-agent") {
		t.Fatal("legacy grant changed")
	}
}

func TestIndependentNodeHTTPDoesNotExposePrivateConfiguration(t *testing.T) {
	s := testStore(t)
	h := NewServer(s, nil, nil, "administrator-password", "http://localhost")
	admin, _ := s.userCaller("admin")
	u, err := s.SaveUser(User{Username: "reader", UserID: "reader", Role: "caller", Enabled: true, Password: "reader-password-123"})
	if err != nil {
		t.Fatal(err)
	}
	w := independentWorkflow(t, map[string]any{"executor": "codex", "instructions": "private-role-marker", "env": map[string]string{"PRIVATE_VALUE": "private-env-marker"}, "native_config": "model_reasoning_effort='high'"})
	w.AuthorizedUsers = []string{u.UserID}
	w, err = s.SaveWorkflow(admin, w)
	if err != nil {
		t.Fatal(err)
	}
	login := cookieRequest(t, h, nil, "POST", "/api/login", map[string]string{"username": "reader", "password": "reader-password-123"})
	cookie := login.Result().Cookies()[0]
	runResponse := cookieRequest(t, h, cookie, "POST", "/api/workflow-runs", WorkflowStart{WorkflowID: w.ID, Input: "task", WorkspacePath: t.TempDir()})
	if runResponse.Code != 201 {
		t.Fatal(runResponse.Code, runResponse.Body.String())
	}
	var run WorkflowRun
	json.Unmarshal(runResponse.Body.Bytes(), &run)
	paths := []string{"/api/workflows", "/api/workflows/" + w.ID, "/api/workflow-runs", "/api/workflow-runs/" + run.ID}
	responses := []string{runResponse.Body.String()}
	for _, path := range paths {
		r := cookieRequest(t, h, cookie, http.MethodGet, path, nil)
		if r.Code != 200 {
			t.Fatal(path, r.Code)
		}
		responses = append(responses, r.Body.String())
	}
	for _, body := range responses {
		if strings.Contains(body, "private-env-marker") || strings.Contains(body, "private-role-marker") || strings.Contains(body, "model_reasoning_effort") {
			t.Fatal("configuration exposed to workflow caller")
		}
	}
	original, _ := s.Workflow(admin, w.ID)
	raw, _ := json.Marshal(original)
	if !strings.Contains(string(raw), "private-env-marker") {
		t.Fatal("response redaction mutated saved configuration")
	}
}
