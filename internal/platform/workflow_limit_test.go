package platform

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func limitedWorkflowRun(t *testing.T, s *Store) (Caller, WorkflowRun) {
	t.Helper()
	c, w, _ := runFixture(t, s)
	w.MaxSteps = 2
	w, err := s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "bounded repair", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []struct {
		seq   int
		route string
	}{{1, "changes"}, {2, "ready"}} {
		if err := s.CompleteWorkflowNode(c, r.ID, x.seq, NodeResult{Route: x.route, Summary: "inspected evidence"}); err != nil {
			t.Fatal(err)
		}
		if err := s.advanceWorkflow(r.ID, x.seq); err != nil {
			t.Fatal(err)
		}
	}
	r, err = s.WorkflowRun(c, r.ID)
	if err != nil || r.Status != "failed" {
		t.Fatal(r, err)
	}
	return c, r
}

func TestWorkflowExecutionLimitExplicitRecovery(t *testing.T) {
	s := testStore(t)
	c, r := limitedWorkflowRun(t, s)
	h := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, h)
	path := "/api/workflow-runs/" + r.ID + "/return"
	body := map[string]any{"seq": r.Seq, "target": "revise", "summary": "Inspected completed effects; fix the new failed assertion", "max_steps": 4}
	// A caller cannot turn a terminal cap into an automatic replay or unbounded loop.
	for _, limit := range []int{0, 2, 1001} {
		bad := map[string]any{"seq": r.Seq, "target": "revise", "summary": "inspect", "max_steps": limit}
		if q := workflowRequest(t, h, cookie, "POST", path, bad); q.Code == 202 {
			t.Fatal("invalid limit accepted", limit)
		}
	}
	if q := workflowRequest(t, h, cookie, "POST", path, map[string]any{"seq": r.Seq, "target": "missing", "summary": "inspect", "max_steps": 4}); q.Code == 202 {
		t.Fatal("invalid target accepted")
	}
	unchanged, err := s.WorkflowRun(c, r.ID)
	if err != nil || unchanged.MaxSteps != 2 || !reflect.DeepEqual(unchanged.Steps, r.Steps) {
		t.Fatal("rejected recovery changed limit/history", unchanged, err)
	}
	q := workflowRequest(t, h, cookie, "POST", path, body)
	if q.Code != 202 {
		t.Fatal("inspected bounded return unavailable", q.Code, q.Body.String())
	}
	a := testAgent(t, s)
	_, token := testUser(t, s, &a, "unrelated-budget")
	if q := requestJSON(t, h, "POST", path, token, body); q.Code != 403 {
		t.Fatal(q.Code, q.Body.String())
	}
	after, err := s.WorkflowRun(c, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Seq != 3 || after.Steps[2].NodeID != "revise" || !reflect.DeepEqual(r.Steps, after.Steps[:2]) || !reflect.DeepEqual(r.Definition, after.Definition) || r.WorkspacePath != after.WorkspacePath {
		t.Fatal("history, graph or workspace changed", after)
	}
	if !strings.Contains(after.Steps[2].Error, "2 → 4") || !strings.Contains(after.Steps[2].Error, "Inspected completed effects") {
		t.Fatal("budget review was not recorded", after.Steps[2])
	}
	if q := workflowRequest(t, h, cookie, "POST", path, body); q.Code != 409 {
		t.Fatal("stale return accepted", q.Code)
	}
	// The effective limit persists across a real Store reopen; no frozen graph edit.
	dir := s.Dir
	s.Close()
	s, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	after, err = s.WorkflowRun(c, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(after)
	var decoded map[string]any
	json.Unmarshal(raw, &decoded)
	if decoded["max_steps"] != float64(4) || after.Definition.MaxSteps != 2 {
		t.Fatal("effective limit lost or snapshot rewritten", string(raw))
	}
	for _, x := range []struct {
		seq   int
		route string
	}{{3, "ready"}, {4, "changes"}} {
		if err := s.CompleteWorkflowNode(c, r.ID, x.seq, NodeResult{Route: x.route, Summary: "new evidence"}); err != nil {
			t.Fatal(err)
		}
		if err := s.advanceWorkflow(r.ID, x.seq); err != nil {
			t.Fatal(err)
		}
	}
	after, err = s.WorkflowRun(c, r.ID)
	if err != nil || after.Status != "failed" || len(after.Steps) != 4 {
		t.Fatal("extended budget failed to stop", after, err)
	}
}
