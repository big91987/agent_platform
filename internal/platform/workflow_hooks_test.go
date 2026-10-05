package platform

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestHookProjectionUsesFinalReplyOnceWithoutAdvancingNode(t *testing.T) {
	s := testStore(t)
	c, w, _ := runFixture(t, s)
	a := testAgent(t, s)
	w.Nodes[0].Kind = "agent"
	w.Nodes[0].AgentID = a.ID
	dir := t.TempDir()
	connector, err := s.SaveConnector(c, Connector{Name: "notify", Kind: "github.issue_comment", Repository: "demo/repo", TokenEnv: "HOOK_TEST_TOKEN", WorkspaceRoot: dir, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	w.Hooks = []WorkflowHook{{Event: "node.started", ConnectorID: connector.ID, Input: ConnectorInput{IssueNumber: 1}, Body: "{{node_name}} {{conversation_url}}"}, {Event: "agent.reply.completed", ConnectorID: connector.ID, Input: ConnectorInput{IssueNumber: 1}, Body: "{{text}}\n{{conversation_url}}"}}
	w.Hooks[0], w.Hooks[1] = w.Hooks[1], w.Hooks[0] // Configuration order must not decide event chronology.
	w, err = s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "request", WorkspacePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "https://platform.example")
	engine.Tick(context.Background())
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"type": "item.completed", "item": map[string]any{"id": "reply", "type": "agent_message", "text": "请选择方向 {{run_id}}"}})
	if err = s.RecordEvent(conv.ID, msg.ID, raw); err != nil {
		t.Fatal(err)
	}
	if err = engine.collectWorkflowHooks(); err != nil {
		t.Fatal(err)
	}
	before, _ := s.WorkflowHookDeliveries(c, r.ID)
	if len(before) != 1 {
		t.Fatalf("unfinished reply was emitted: %+v", before)
	}
	if err = s.Complete(conv.ID, msg.ID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	// Rebuild projection from completed facts, as when collection was delayed.
	if _, err = s.DB.Exec(`DELETE FROM workflow_hook_deliveries WHERE run_id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err = engine.collectWorkflowHooks(); err != nil {
			t.Fatal(err)
		}
	}
	deliveries, err := s.WorkflowHookDeliveries(c, r.ID)
	if err != nil || len(deliveries) != 2 {
		t.Fatal(deliveries, err)
	}
	if deliveries[0].Event != "node.started" || deliveries[1].Event != "agent.reply.completed" {
		t.Fatal("events reordered by hook configuration", deliveries)
	}
	var fieldsRaw string
	if err = s.DB.QueryRow(`SELECT fields FROM workflow_hook_deliveries WHERE run_id=? AND event='agent.reply.completed'`, r.ID).Scan(&fieldsRaw); err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{}
	json.Unmarshal([]byte(fieldsRaw), &fields)
	if got := renderHookBody("{{text}}", fields); got != "请选择方向 {{run_id}}" {
		t.Fatal("message interpreted as a template", got)
	}
	after, _ := s.WorkflowRun(c, r.ID)
	if after.Seq != 1 || after.Steps[0].Result != nil {
		t.Fatal("notification advanced run", after)
	}
	if _, err = s.WorkflowHookDeliveries(Caller{UserID: "other"}, r.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	// A missing credential fails the notification only, not the Agent execution.
	if err = engine.dispatchWorkflowHooks(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, _ = s.WorkflowRun(c, r.ID)
	if after.Status == "failed" {
		t.Fatal("notification failure stopped Agent")
	}
	deliveries, _ = s.WorkflowHookDeliveries(c, r.ID)
	if deliveries[0].Status != "failed" {
		t.Fatal(deliveries)
	}
}

func TestHookRejectsUnknownFieldsAndSnapshotsActionPermission(t *testing.T) {
	raw, _ := json.Marshal(workflowFixture())
	var w Workflow
	json.Unmarshal(raw, &w)
	w.Hooks = []WorkflowHook{{Event: "agent.reply.completed", ConnectorID: "action", Body: "{{unknown}}"}}
	if err := validateWorkflow(w); err == nil {
		t.Fatal("unknown field accepted")
	}
	w.Hooks[0].Body = "{{text}}"
	w.Hooks[0].Event = "handoff.before"
	if err := validateWorkflow(w); err == nil {
		t.Fatal("unimplemented blocking hook accepted")
	}
}
