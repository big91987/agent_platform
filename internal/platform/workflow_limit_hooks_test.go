package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func limitHookRun(t *testing.T, explicit ...bool) (*Store, Caller, WorkflowRun, *WorkflowEngine) {
	t.Helper()
	s := testStore(t)
	c, w, _ := runFixture(t, s)
	dir := t.TempDir()
	v, err := s.SaveConnector(c, Connector{Name: "replies", Kind: "github.issue_comment", Repository: "demo/repo", TokenEnv: "LIMIT_HOOK_TEST", WorkspaceRoot: dir, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	// A node-specific Agent reply channel must still carry a run-level stop
	// after another node, which may have no conversation to write a reply.
	w.Hooks = []WorkflowHook{{Event: "agent.reply.completed", Node: w.Nodes[0].ID, ConnectorID: v.ID, Input: ConnectorInput{IssueNumber: 1}, Body: "{{text}}"}}
	w.Hooks = append(w.Hooks, WorkflowHook{Event: "agent.reply.completed", Node: w.Nodes[1].ID, ConnectorID: v.ID, Input: ConnectorInput{IssueNumber: 1}, Body: "{{text}}"})
	if len(explicit) > 0 && explicit[0] {
		w.Hooks = append(w.Hooks, WorkflowHook{Event: "run.execution_limit_reached", ConnectorID: v.ID, Input: ConnectorInput{IssueNumber: 1}, Body: "custom {{execution_limit}} {{run_url}}"})
	}
	w.MaxSteps = 2
	w, err = s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "bounded", WorkspacePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []struct {
		seq   int
		route string
	}{{1, "changes"}, {2, "ready"}} {
		if err = s.CompleteWorkflowNode(c, r.ID, x.seq, NodeResult{Route: x.route, Summary: "private-path-and-secret-not-for-stop-notice"}); err != nil {
			t.Fatal(err)
		}
		if err = s.advanceWorkflow(r.ID, x.seq); err != nil {
			t.Fatal(err)
		}
	}
	r, err = s.WorkflowRun(c, r.ID)
	if err != nil || r.Status != "failed" {
		t.Fatal(r, err)
	}
	return s, c, r, NewWorkflowEngine(s, nil, "https://platform.example")
}

func TestWorkflowLimitNoticeExplicitRuleOverridesReplyDefaults(t *testing.T) {
	s, c, r, e := limitHookRun(t, true)
	if err := e.collectWorkflowHooks(); err != nil {
		t.Fatal(err)
	}
	list, _ := s.WorkflowHookDeliveries(c, r.ID)
	if len(list) != 1 {
		t.Fatal("explicit rule duplicated", list)
	}
	var index int
	s.DB.QueryRow(`SELECT hook_index FROM workflow_hook_deliveries WHERE id=?`, list[0].ID).Scan(&index)
	if workflowNotificationHooks(r.Definition)[index].Body != "custom {{execution_limit}} {{run_url}}" {
		t.Fatal("default replaced explicit rule")
	}
}

func TestWorkflowLimitNoticeRecoveredDuringLookupIsNotPosted(t *testing.T) {
	s, c, r, e := limitHookRun(t)
	t.Setenv("LIMIT_HOOK_TEST", "test-token")
	original := githubClient
	defer func() { githubClient = original }()
	posts := 0
	githubClient = &http.Client{Transport: connectorTransport(func(req *http.Request) (*http.Response, error) {
		if req.Method == "GET" {
			if err := e.Return(c, r.ID, 2, "revise", "inspected while lookup pending", 4); err != nil {
				return nil, err
			}
		} else {
			posts++
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`[]`))}, nil
	})}
	if err := e.collectWorkflowHooks(); err != nil {
		t.Fatal(err)
	}
	if err := e.dispatchWorkflowHooks(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.connectorWG.Wait()
	list, _ := s.WorkflowHookDeliveries(c, r.ID)
	if posts != 0 || len(list) != 1 || list[0].Status != "skipped" {
		t.Fatal("recovery posted stale stop", posts, list)
	}
}

func TestWorkflowLimitNoticeSurvivesRestartWithoutDuplicatingOrAdvancing(t *testing.T) {
	s, c, r, e := limitHookRun(t)
	for range 3 {
		if err := e.collectWorkflowHooks(); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.WorkflowHookDeliveries(c, r.ID)
	if err != nil || len(list) != 1 || list[0].Event != "run.execution_limit_reached" {
		t.Fatalf("limit stop invisible: %+v %v", list, err)
	}
	var raw string
	if err = s.DB.QueryRow(`SELECT fields FROM workflow_hook_deliveries WHERE id=?`, list[0].ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{}
	json.Unmarshal([]byte(raw), &fields)
	if fields["execution_limit"] != "2" || fields["seq"] != "2" || fields["run_url"] != "https://platform.example/workflow-runs/"+r.ID || strings.Contains(raw, "private-path") {
		t.Fatal("missing safe stop facts or leaked node summary", raw)
	}
	dir := s.Dir
	s.Close()
	s, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e = NewWorkflowEngine(s, nil, "https://platform.example")
	if err = e.collectWorkflowHooks(); err != nil {
		t.Fatal(err)
	}
	afterList, _ := s.WorkflowHookDeliveries(c, r.ID)
	after, _ := s.WorkflowRun(c, r.ID)
	if len(afterList) != 1 || afterList[0].ID != list[0].ID || !reflect.DeepEqual(r, after) {
		t.Fatal("restart duplicated notice or changed run", afterList, after)
	}
}

func TestWorkflowLimitNoticeRecoveredBeforeSendIsSkipped(t *testing.T) {
	s, c, r, e := limitHookRun(t)
	if err := e.collectWorkflowHooks(); err != nil {
		t.Fatal(err)
	}
	if err := e.Return(c, r.ID, 2, "revise", "inspected effects", 4); err != nil {
		t.Fatal(err)
	}
	if err := e.dispatchWorkflowHooks(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.connectorWG.Wait()
	list, _ := s.WorkflowHookDeliveries(c, r.ID)
	if len(list) != 1 || list[0].Status != "skipped" {
		t.Fatal("stale stop notice was not skipped", list)
	}
	if err := e.collectWorkflowHooks(); err != nil {
		t.Fatal(err)
	}
	list, _ = s.WorkflowHookDeliveries(c, r.ID)
	if len(list) != 1 {
		t.Fatal("recovery created historical stop", list)
	}
}

func TestWorkflowLimitNoticeLostWriteIsReconciledAfterRecovery(t *testing.T) {
	s, c, r, e := limitHookRun(t)
	t.Setenv("LIMIT_HOOK_TEST", "test-token")
	original := githubClient
	defer func() { githubClient = original }()
	posts := 0
	var saved map[string]any
	githubClient = &http.Client{Transport: connectorTransport(func(req *http.Request) (*http.Response, error) {
		body := `[]`
		if req.Method == "POST" {
			posts++
			var payload map[string]any
			json.NewDecoder(req.Body).Decode(&payload)
			saved = map[string]any{"html_url": "https://github.com/demo/repo/issues/1#issuecomment-7", "body": payload["body"]}
			if strings.Contains(payload["body"].(string), "private-path") {
				t.Error("leaked business summary")
			}
			return nil, errors.New("lost write response")
		}
		if saved != nil {
			raw, _ := json.Marshal([]map[string]any{saved})
			body = string(raw)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	if err := e.collectWorkflowHooks(); err != nil {
		t.Fatal(err)
	}
	if err := e.dispatchWorkflowHooks(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.connectorWG.Wait()
	list, _ := s.WorkflowHookDeliveries(c, r.ID)
	if len(list) != 1 || list[0].Status != "unknown" {
		t.Fatal(list)
	}
	if err := e.Return(c, r.ID, 2, "revise", "inspected effects", 4); err != nil {
		t.Fatal(err)
	}
	h := NewServer(s, nil, nil, "password", "https://platform.example")
	cookie := workflowAdmin(t, h)
	resp := workflowRequest(t, h, cookie, "POST", "/api/workflow-runs/"+r.ID+"/notifications/"+list[0].ID+"/retry", nil)
	if resp.Code != 202 {
		t.Fatal(resp.Code, resp.Body.String())
	}
	if err := h.workflows.dispatchWorkflowHooks(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.workflows.connectorWG.Wait()
	list, _ = s.WorkflowHookDeliveries(c, r.ID)
	if posts != 1 || list[0].Status != "succeeded" {
		t.Fatal("uncertain write replayed or lost", posts, list)
	}
}
