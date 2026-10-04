package platform

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runFixture(t *testing.T, s *Store) (Caller, Workflow, WorkflowRun) {
	t.Helper()
	if e := s.initAuth("password"); e != nil {
		t.Fatal(e)
	}
	c, e := s.userCaller("admin")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(workflowFixture())
	var w Workflow
	json.Unmarshal(b, &w)
	w, e = s.SaveWorkflow(c, w)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "检查并修订文稿", WorkspacePath: t.TempDir(), RequestID: "start-1"})
	if e != nil {
		t.Fatal(e)
	}
	return c, w, r
}
func TestWorkflowRunCyclesKeepHistoryAndFrozenDefinition(t *testing.T) {
	s := testStore(t)
	c, w, r := runFixture(t, s)
	for _, x := range []struct {
		seq   int
		route string
	}{{1, "changes"}, {2, "ready"}, {3, "approved"}} {
		if e := s.CompleteWorkflowNode(c, r.ID, x.seq, NodeResult{Route: x.route, Summary: "检查结论"}); e != nil {
			t.Fatal(e)
		}
		// Accepting a result never implicitly starts a second execution.
		before, _ := s.WorkflowRun(c, r.ID)
		if before.Seq != x.seq {
			t.Fatal(before)
		}
		if e := s.advanceWorkflow(r.ID, x.seq); e != nil {
			t.Fatal(e)
		}
		if x.seq == 1 {
			w.Edges[0].Target = "revise"
			w.Edges[1].Target = "done"
			if _, e := s.SaveWorkflow(c, w); e != nil {
				t.Fatal(e)
			}
		}
	}
	r, e := s.WorkflowRun(c, r.ID)
	if e != nil {
		t.Fatal(e)
	}
	if r.Status != "completed" || r.Seq != 4 || len(r.Steps) != 4 || r.Definition.Revision != 1 {
		t.Fatalf("not completed on original graph: %+v", r)
	}
	if r.Steps[0].NodeID != r.Steps[2].NodeID || r.Steps[0].Seq == r.Steps[2].Seq {
		t.Fatal(r.Steps)
	}
}
func TestWorkflowRunDedupAndStaleCompletion(t *testing.T) {
	s := testStore(t)
	c, w, r := runFixture(t, s)
	same, e := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: r.Input, WorkspacePath: r.WorkspacePath, RequestID: "start-1"})
	if e != nil || same.ID != r.ID {
		t.Fatal(same, e)
	}
	_, e = s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "different", WorkspacePath: r.WorkspacePath, RequestID: "start-1"})
	if !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	result := NodeResult{Route: "changes", Summary: "needs revision"}
	for i := 0; i < 2; i++ {
		if e = s.CompleteWorkflowNode(c, r.ID, 1, result); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.CompleteWorkflowNode(c, r.ID, 1, NodeResult{Route: "approved"}); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if e = s.advanceWorkflow(r.ID, 1); e != nil {
		t.Fatal(e)
	}
	if e = s.CompleteWorkflowNode(c, r.ID, 1, result); !errors.Is(e, ErrConflict) {
		t.Fatalf("old completion accepted: %v", e)
	}
	if e = s.CompleteWorkflowNode(c, r.ID, 2, NodeResult{Route: "missing"}); e == nil {
		t.Fatal("unknown route accepted")
	}
}
func TestWorkflowRunOwnershipAndStepBound(t *testing.T) {
	s := testStore(t)
	c, w, r := runFixture(t, s)
	intruder := Caller{UserID: "someone-else"}
	if _, e := s.WorkflowRun(intruder, r.ID); !errors.Is(e, ErrForbidden) {
		t.Fatal(e)
	}
	if e := s.CompleteWorkflowNode(intruder, r.ID, 1, NodeResult{Route: "approved"}); !errors.Is(e, ErrForbidden) {
		t.Fatal(e)
	}
	w.MaxSteps = 2
	w, e := s.SaveWorkflow(c, w)
	if e != nil {
		t.Fatal(e)
	}
	r, e = s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "loop", WorkspacePath: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range []struct {
		seq   int
		route string
	}{{1, "changes"}, {2, "ready"}} {
		if e = s.CompleteWorkflowNode(c, r.ID, x.seq, NodeResult{Route: x.route}); e != nil {
			t.Fatal(e)
		}
		if e = s.advanceWorkflow(r.ID, x.seq); e != nil {
			t.Fatal(e)
		}
	}
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "failed" || len(r.Steps) != 2 || r.Error == "" {
		t.Fatal(r)
	}
}
func TestWorkflowRunRejectsAgentHandoffWithoutScopedTool(t *testing.T) {
	s := testStore(t)
	c, w, _ := runFixture(t, s)
	a := testAgent(t, s)
	w.Nodes[0].Kind = "agent"
	w.Nodes[0].AgentID = a.ID
	w, e := s.SaveWorkflow(c, w)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "work", WorkspacePath: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.CompleteWorkflowNode(c, r.ID, 1, NodeResult{Route: "approved"}); e == nil {
		t.Fatal("human manufactured Agent result")
	}
}

func TestWorkflowRunHTTPRequiresOwnerAndLatestStep(t *testing.T) {
	s := testStore(t)
	c, w, r := runFixture(t, s)
	a := testAgent(t, s)
	_, token := testUser(t, s, &a, "unrelated")
	h := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, h)
	for _, path := range []string{"/api/workflow-runs/" + r.ID, "/api/workflow-runs/" + r.ID + "/decision"} {
		method := "GET"
		var body any
		if strings.HasSuffix(path, "decision") {
			method = "POST"
			body = map[string]any{"seq": 1, "route": "approved"}
		}
		response := requestJSON(t, h, method, path, token, body)
		if response.Code != 403 {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	response := workflowRequest(t, h, cookie, "POST", "/api/workflow-runs/"+r.ID+"/decision", map[string]any{"seq": 1, "route": "changes", "summary": "modify"})
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	h.workflows.Tick(context.Background())
	response = workflowRequest(t, h, cookie, "POST", "/api/workflow-runs/"+r.ID+"/decision", map[string]any{"seq": 1, "route": "approved"})
	if response.Code != 409 {
		t.Fatal(response.Code, response.Body.String())
	}
	fresh, _ := s.WorkflowRun(c, r.ID)
	if fresh.Seq != 2 || fresh.Definition.ID != w.ID {
		t.Fatal(fresh)
	}
}

func TestWorkflowWorkspaceIsReservedUntilRunCompletes(t *testing.T) {
	s := testStore(t)
	c, w, r := runFixture(t, s)
	child := filepath.Join(r.WorkspacePath, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(r.WorkspacePath, link); err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		for _, path := range []string{r.WorkspacePath, child, link} {
			_, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "another task", WorkspacePath: path})
			if !errors.Is(err, ErrConflict) {
				t.Fatalf("overlapping workspace accepted %s: %v", path, err)
			}
		}
	}
	check()
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	if err := engine.Stop(c, r.ID, r.Seq); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	check() // Stopped runs remain resumable and still own their files.
	if err := engine.Return(c, r.ID, r.Seq, "done", "用户结束此任务"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "next task", WorkspacePath: r.WorkspacePath}); err != nil {
		t.Fatal(err)
	}
}
