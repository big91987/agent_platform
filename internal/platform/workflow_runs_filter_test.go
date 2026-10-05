package platform

import "testing"

func TestWorkflowRunsFilterByWorkflowAndOwner(t *testing.T) {
	s := testStore(t)
	c, w, r := runFixture(t, s)
	other := w
	other.ID = ""
	other.Revision = 0
	other.Name = "Another workflow"
	other, err := s.SaveWorkflow(c, other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.StartWorkflow(c, WorkflowStart{WorkflowID: other.ID, Input: "other", WorkspacePath: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	runs, err := s.WorkflowRunsFor(c, w.ID)
	if err != nil || len(runs) != 1 || runs[0].ID != r.ID {
		t.Fatalf("wrong workflow runs: %+v %v", runs, err)
	}
	runs, err = s.WorkflowRunsFor(Caller{UserID: "unrelated"}, w.ID)
	if err != nil || len(runs) != 0 {
		t.Fatalf("owner boundary lost: %+v %v", runs, err)
	}
}
