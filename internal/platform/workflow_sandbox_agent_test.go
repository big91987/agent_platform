package platform

import (
	"database/sql"
	"testing"
)

// This focused engine regression keeps the public sandbox-Agent gate in place
// until native process lifecycle and artifacts are connected.
func TestSandboxRoleConversationUsesExistingRunAssociationAndSchedulesIndependentEnvironments(t *testing.T) {
	s := testStore(t)
	caller, graph, creates, _ := sandboxWorkflowFixture(t, s, false, false)
	engine := NewWorkflowEngine(s, nil, "http://platform.local")
	ids := []string{}
	for _, task := range []string{"one", "two"} {
		run, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: graph.ID, Input: task, RequestID: task})
		if err != nil {
			t.Fatal(err)
		}
		run = waitSandboxRun(t, engine, caller, run.ID, "ready")
		// Unit fixture binds a trusted conversation in the same transaction as
		// submission; public Agent orchestration remains separately gated.
		config := Agent{Name: "Independent role", Executor: "codex", Model: "gpt-6.1-sol", Sandbox: "workspace-write", Enabled: true}
		_, err = s.submitConfigured(caller, Input{environmentRun: run.ID, UserID: run.Owner, Message: task, WorkspacePath: run.WorkspacePath, RequestID: "native-" + task}, func(tx *sql.Tx, _ *Agent, conversationID string) error {
			_, e := tx.Exec(`UPDATE workflow_steps SET conversation_id=? WHERE run_id=? AND seq=1`, conversationID, run.ID)
			return e
		}, &config)
		if err != nil {
			t.Fatal("sandbox path was interpreted as host path", err)
		}
		run, err = s.WorkflowRun(caller, run.ID)
		if err != nil {
			t.Fatal(err)
		}
		id := run.Steps[0].ConversationID
		ids = append(ids, id)
		attached, err := s.sandboxRunForConversation(id)
		if err != nil || attached == nil || attached.ID != run.ID || attached.Environment.ID != run.Environment.ID {
			t.Fatal("Run environment binding lost", attached, err)
		}
		conversation, err := s.Conversation(id)
		if err != nil || conversation.WorkspacePath != "/workspace" || conversation.Snapshot.Model != "gpt-6.1-sol" {
			t.Fatal("role settings changed", conversation, err)
		}
	}
	if ids[0] == ids[1] || creates.Load() != 2 {
		t.Fatal("native sessions or Run environments conflated", ids, creates.Load())
	}
	first, _, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	key, err := s.conversationWorkspaceKey(first)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := s.Claim(key)
	if err != nil || second.ID == first.ID {
		t.Fatal("different containers serialized by identical /workspace path", second, err)
	}
	if _, err = s.Submit(caller, Input{ConversationID: first.ID, UserID: caller.UserID, WorkspacePath: "/workspace", Message: "continue", RequestID: "follow-up"}); err != nil {
		t.Fatal("follow-up lost remote workspace", err)
	}
}
