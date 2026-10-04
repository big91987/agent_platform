package platform

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func agentRun(t *testing.T, s *Store) (Caller, WorkflowRun) {
	t.Helper()
	c, w, _ := runFixture(t, s)
	a := testAgent(t, s)
	w.Nodes[0].Kind = "agent"
	w.Nodes[0].AgentID = a.ID
	w, e := s.SaveWorkflow(c, w)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "写一份草稿", WorkspacePath: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	return c, r
}
func TestWorkflowEngineSealsHandoffAndWaitsForNativeTurn(t *testing.T) {
	s := testStore(t)
	c, r := agentRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Steps[0].ConversationID == "" {
		t.Fatal("no conversation")
	}
	conv, msg, e := s.Claim()
	if e != nil {
		t.Fatal(e)
	}
	token := *conv.Snapshot.Env[workflowTokenEnv]
	if e = s.completeWorkflowNode(Caller{}, r.ID, 1, token, NodeResult{Route: "approved", Summary: "草稿完成"}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Submit(c, Input{UserID: c.UserID, ConversationID: conv.ID, Message: "late input"}); !errors.Is(e, ErrConflict) {
		t.Fatal("sealed input accepted", e)
	}
	engine.Tick(context.Background())
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Seq != 1 {
		t.Fatal("advanced while native process was running")
	}
	if e = s.Complete(conv.ID, msg.ID, "completed", ""); e != nil {
		t.Fatal(e)
	}
	engine.Tick(context.Background())
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "completed" {
		t.Fatal(r)
	}
	if e = s.completeWorkflowNode(Caller{}, r.ID, 1, token, NodeResult{Route: "approved"}); !errors.Is(e, ErrConflict) {
		t.Fatal("stale scoped tool accepted", e)
	}
}
func TestWorkflowIdleDoesNotMeanCompletedAndPendingInputBlocksHandoff(t *testing.T) {
	s := testStore(t)
	c, r := agentRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	conv, msg, e := s.Claim()
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Submit(c, Input{UserID: c.UserID, ConversationID: conv.ID, Message: "请加一个标题"})
	if e != nil {
		t.Fatal(e)
	}
	token := *conv.Snapshot.Env[workflowTokenEnv]
	if e = s.completeWorkflowNode(Caller{}, r.ID, 1, token, NodeResult{Route: "approved"}); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if e = s.Complete(conv.ID, msg.ID, "completed", ""); e != nil {
		t.Fatal(e)
	}
	conv, msg, e = s.Claim()
	if e != nil {
		t.Fatal(e)
	}
	s.Complete(conv.ID, msg.ID, "completed", "")
	engine.Tick(context.Background())
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Seq != 1 || r.Status != "waiting" {
		t.Fatalf("idle advanced workflow: %+v", r)
	}
}
func TestWorkflowStopAndReturnPreserveOldExecution(t *testing.T) {
	s := testStore(t)
	c, _, r := runFixture(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	if e := engine.Stop(c, r.ID, 1); e != nil {
		t.Fatal(e)
	}
	engine.Tick(context.Background())
	stopped, _ := s.WorkflowRun(c, r.ID)
	if stopped.Status != "stopped" {
		t.Fatal(stopped)
	}
	if e := engine.Return(c, r.ID, 1, "revise", "用户要求重新修订"); e != nil {
		t.Fatal(e)
	}
	returned, _ := s.WorkflowRun(c, r.ID)
	if returned.Seq != 2 || returned.Steps[1].NodeID != "revise" || returned.Steps[0].Status != "cancelled" {
		t.Fatal(returned)
	}
	if e := s.CompleteWorkflowNode(c, r.ID, 1, NodeResult{Route: "approved"}); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
}

func TestWorkflowRevokedAgentCannotRunPendingNode(t *testing.T) {
	s := testStore(t)
	c, r := agentRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	a, e := s.Agent(r.Definition.Nodes[0].AgentID)
	if e != nil {
		t.Fatal(e)
	}
	a.Enabled = false
	if _, e = s.SaveAgent(a); e != nil {
		t.Fatal(e)
	}
	engine.Tick(context.Background())
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "failed" || r.Steps[0].ConversationID != "" {
		t.Fatal(r)
	}
}

func TestWorkflowRestartDoesNotReplayUncertainAgentTurn(t *testing.T) {
	s := testStore(t)
	c, r := agentRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	conv, _, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	dir := s.Dir
	s.Close()
	reopened, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	engine = NewWorkflowEngine(reopened, nil, "http://localhost")
	engine.Tick(context.Background())
	after, err := reopened.WorkflowRun(c, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != "failed" || after.Steps[0].ConversationID != conv.ID {
		t.Fatal(after)
	}
	if _, _, err = reopened.Claim(); !errors.Is(err, ErrNotFound) {
		t.Fatal("uncertain turn replayed", err)
	}
	if err = engine.Resume(c, r.ID, 1, ""); err == nil {
		t.Fatal("resumed without explicit continuation")
	}
	if err = engine.Resume(c, r.ID, 1, "先检查已有文件，然后接着写"); err != nil {
		t.Fatal(err)
	}
	resumed, msg, err := reopened.Claim()
	if err != nil || resumed.ID != conv.ID || msg.Content != "先检查已有文件，然后接着写" {
		t.Fatal(resumed, msg, err)
	}
}
func TestWorkflowAgentContextIsNotPresentedAsUserInput(t *testing.T) {
	s := testStore(t)
	_, r := agentRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != r.Input {
		t.Fatal("internal workflow instructions in user message")
	}
	if !strings.Contains(conv.Snapshot.Instructions, "complete_node") || conv.Snapshot.ResolvedTools[workflowToolServer].Connection.URL == "" {
		t.Fatal("scoped context/tool missing")
	}
}
