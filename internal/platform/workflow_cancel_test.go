package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkflowCancelPreservesEvidenceAndReleasesWorkspace(t *testing.T) {
	s := testStore(t)
	c, r := agentRun(t, s)
	e := NewWorkflowEngine(s, nil, "http://localhost")
	e.Tick(context.Background())
	r, _ = s.WorkflowRun(c, r.ID)
	conversation := r.Steps[0].ConversationID
	if err := e.Cancel(c, r.ID, r.Seq); !errors.Is(err, ErrConflict) {
		t.Fatalf("cancelled active run: %v", err)
	}
	if err := e.Stop(c, r.ID, r.Seq); err != nil {
		t.Fatal(err)
	}
	e.Tick(context.Background())
	path := filepath.Join(r.WorkspacePath, "evidence.txt")
	if err := os.WriteFile(path, []byte("real work"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.Cancel(Caller{UserID: "stranger"}, r.ID, r.Seq); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unauthorized cancellation: %v", err)
	}
	if err := e.Cancel(c, r.ID, r.Seq+1); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale cancellation: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := e.Cancel(c, r.ID, r.Seq); err != nil {
			t.Fatal(err)
		}
	}
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "cancelled" || r.Steps[0].Status != "cancelled" || r.Steps[0].ConversationID != conversation {
		t.Fatalf("lost history: %+v", r)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "real work" {
		t.Fatal("lost files", err)
	}
	if msgs, err := s.Messages(conversation); err != nil || len(msgs) != 1 {
		t.Fatal("lost messages", err)
	}
	if _, err := s.Submit(c, Input{UserID: r.Owner, ConversationID: conversation, Message: "resume cancelled", RequestID: "after-cancel"}); err == nil {
		t.Fatal("cancelled conversation accepted input")
	}
	e = NewWorkflowEngine(s, nil, "http://localhost")
	e.Tick(context.Background())
	for _, err := range []error{e.Resume(c, r.ID, r.Seq, "continue"), e.Return(c, r.ID, r.Seq, r.Definition.Entry, "retry"), e.Stop(c, r.ID, r.Seq)} {
		if !errors.Is(err, ErrConflict) {
			t.Fatal("cancelled run was reactivated", err)
		}
	}
	if _, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: r.WorkflowID, Input: "new task", WorkspacePath: r.WorkspacePath}); err != nil {
		t.Fatal("workspace still reserved", err)
	}
}

func TestWorkflowCancelHTTP(t *testing.T) {
	s := testStore(t)
	_, _, r := runFixture(t, s)
	h := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, h)
	for _, action := range []string{"stop", "cancel"} {
		response := workflowRequest(t, h, cookie, "POST", "/api/workflow-runs/"+r.ID+"/"+action, map[string]any{"seq": r.Seq})
		if response.Code != 202 {
			t.Fatal(action, response.Code, response.Body.String())
		}
		h.workflows.Tick(context.Background())
	}
	r, err := s.WorkflowRun(Caller{Admin: true}, r.ID)
	if err != nil || r.Status != "cancelled" {
		t.Fatal(r.Status, err)
	}
}
