package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

func completedRunForReturn(t *testing.T, s *Store) (Caller, WorkflowRun) {
	t.Helper()
	c, _, r := runFixture(t, s)
	if err := s.CompleteWorkflowNode(c, r.ID, 1, NodeResult{Route: "approved", Summary: "Original acceptance"}); err != nil {
		t.Fatal(err)
	}
	if err := s.advanceWorkflow(r.ID, 1); err != nil {
		t.Fatal(err)
	}
	r, err := s.WorkflowRun(c, r.ID)
	if err != nil || r.Status != "completed" {
		t.Fatal(r, err)
	}
	return c, r
}

func TestWorkflowCompletedReturnPreservesHistoryAndRejectsReplay(t *testing.T) {
	s := testStore(t)
	c, r := completedRunForReturn(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	if err := engine.Return(c, r.ID, r.Seq, "revise", "Review found a defect"); err != nil {
		t.Fatal(err)
	}
	after, err := s.WorkflowRun(c, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Seq != r.Seq+1 || after.Status == "completed" || after.Steps[len(after.Steps)-1].NodeID != "revise" {
		t.Fatal(after)
	}
	if !reflect.DeepEqual(r.Steps, after.Steps[:len(r.Steps)]) || !reflect.DeepEqual(r.Definition, after.Definition) || after.WorkspacePath != r.WorkspacePath {
		t.Fatal("completed history/snapshot/workspace changed")
	}
	if !strings.Contains(after.Steps[len(after.Steps)-1].Error, "Review found a defect") {
		t.Fatal("redirection reason not recorded")
	}
	if err := engine.Return(c, r.ID, r.Seq, "revise", "duplicate"); !errors.Is(err, ErrConflict) {
		t.Fatal("stale return accepted", err)
	}
	if err := s.CompleteWorkflowNode(c, r.ID, 1, NodeResult{Route: "approved"}); !errors.Is(err, ErrConflict) {
		t.Fatal("late result accepted", err)
	}
	if _, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: r.WorkflowID, Input: "competing", WorkspacePath: r.WorkspacePath}); !errors.Is(err, ErrConflict) {
		t.Fatal("workspace was not reacquired", err)
	}
}

func TestWorkflowCompletedReturnRejectsOccupiedWorkspace(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprint(nested), func(t *testing.T) {
			s := testStore(t)
			c, r := completedRunForReturn(t, s)
			path := r.WorkspacePath
			if nested {
				path = filepath.Join(path, "child")
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			other, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: r.WorkflowID, Input: "other task", WorkspacePath: path})
			if err != nil {
				t.Fatal(err)
			}
			engine := NewWorkflowEngine(s, nil, "http://localhost")
			if err = engine.Return(c, r.ID, r.Seq, "revise", "review feedback"); !errors.Is(err, ErrConflict) {
				t.Fatal("occupied workspace accepted", err)
			}
			after, _ := s.WorkflowRun(c, r.ID)
			otherAfter, _ := s.WorkflowRun(c, other.ID)
			if !reflect.DeepEqual(after, r) || !reflect.DeepEqual(otherAfter, other) {
				t.Fatal("failed return mutated history")
			}
		})
	}
}

func TestWorkflowCompletedReturnValidatesAuthorityReasonAndTarget(t *testing.T) {
	s := testStore(t)
	c, r := completedRunForReturn(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	for _, x := range []struct {
		caller         Caller
		target, reason string
	}{
		{Caller{}, "revise", "reason"}, {c, "revise", " "}, {c, "missing", "reason"},
	} {
		if err := engine.Return(x.caller, r.ID, r.Seq, x.target, x.reason); err == nil {
			t.Fatal("invalid return accepted", x)
		}
	}
	after, _ := s.WorkflowRun(c, r.ID)
	if !reflect.DeepEqual(r, after) {
		t.Fatal("invalid return changed run")
	}
}

func TestWorkflowCompletedReturnFeedbackSurvivesFailureAndResume(t *testing.T) {
	for _, connectorFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(connectorFailure), func(t *testing.T) {
			s := testStore(t)
			c, r := completedRunForReturn(t, s)
			engine := NewWorkflowEngine(s, nil, "http://localhost")
			if err := engine.Return(c, r.ID, r.Seq, "revise", "required review correction"); err != nil {
				t.Fatal(err)
			}
			returned, _ := s.WorkflowRun(c, r.ID)
			if connectorFailure {
				if err := engine.finishConnector(returned, returned.Steps[len(returned.Steps)-1], ConnectorReceipt{}, errors.New("temporary connector failure")); err != nil {
					t.Fatal(err)
				}
			} else {
				engine.fail(returned, errors.New("temporary startup failure"))
			}
			if err := engine.Resume(c, r.ID, returned.Seq, "retry startup"); err != nil {
				t.Fatal(err)
			}
			resumed, _ := s.WorkflowRun(c, r.ID)
			feedback := resumed.Steps[len(resumed.Steps)-1].Error
			if !strings.Contains(feedback, "required review correction") || !strings.Contains(feedback, "temporary") {
				t.Fatal("feedback lost", feedback)
			}
		})
	}
}

func TestWorkflowCompletedReturnRejectsRevokedWorkflowAccess(t *testing.T) {
	s := testStore(t)
	admin, w, _ := runFixture(t, s)
	a := testAgent(t, s)
	aliceUser, _ := testUser(t, s, &a, "return-owner")
	alice, err := s.userCaller(aliceUser.Username)
	if err != nil {
		t.Fatal(err)
	}
	w.AuthorizedUsers = []string{alice.UserID}
	w, err = s.SaveWorkflow(admin, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(alice, WorkflowStart{WorkflowID: w.ID, Input: "owned", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteWorkflowNode(alice, r.ID, 1, NodeResult{Route: "approved"}); err != nil {
		t.Fatal(err)
	}
	if err = s.advanceWorkflow(r.ID, 1); err != nil {
		t.Fatal(err)
	}
	before, _ := s.WorkflowRun(alice, r.ID)
	w.AuthorizedUsers = nil
	if w, err = s.SaveWorkflow(admin, w); err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	if err = engine.Return(alice, r.ID, before.Seq, "revise", "feedback"); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked owner reactivated run", err)
	}
	after, _ := s.WorkflowRun(alice, r.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("revoked return changed completed run")
	}
	// Historical stopped runs must remain administratively closable after revocation.
	w.AuthorizedUsers = []string{alice.UserID}
	w, err = s.SaveWorkflow(admin, w)
	if err != nil {
		t.Fatal(err)
	}
	active, err := s.StartWorkflow(alice, WorkflowStart{WorkflowID: w.ID, Input: "cleanup", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.Stop(admin, active.ID, active.Seq); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	w.AuthorizedUsers = nil
	w, err = s.SaveWorkflow(admin, w)
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.Return(admin, active.ID, active.Seq, "done", "administrative close"); err != nil {
		t.Fatal("revoked stopped run cannot be closed", err)
	}
	if _, err = s.StartWorkflow(admin, WorkflowStart{WorkflowID: w.ID, Input: "after cleanup", WorkspacePath: active.WorkspacePath}); err != nil {
		t.Fatal("closed workspace not released", err)
	}
	if _, err = s.StartWorkflow(admin, WorkflowStart{WorkflowID: w.ID, Input: "authorized task", WorkspacePath: r.WorkspacePath}); err != nil {
		t.Fatal("workspace was reclaimed", err)
	}
}
