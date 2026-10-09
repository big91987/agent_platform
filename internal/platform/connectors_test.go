package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func connectorFixture(t *testing.T, s *Store, c Connector) (Caller, Connector, Workflow, string) {
	t.Helper()
	if err := s.initAuth("password"); err != nil {
		t.Fatal(err)
	}
	owner, err := s.userCaller("admin")
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	c.WorkspaceRoot = workspace
	saved, err := s.SaveConnector(owner, c)
	if err != nil {
		t.Fatal(err)
	}
	w, err := s.SaveWorkflow(owner, Workflow{Name: "external", Enabled: true, Entry: "call", MaxSteps: 10,
		Nodes: []WorkflowNode{{ID: "call", Name: "Call", Kind: "connector", ConnectorID: saved.ID}, {ID: "done", Name: "Done", Kind: "end"}},
		Edges: []WorkflowEdge{{Source: "call", Route: "next", Target: "done"}}})
	if err != nil {
		t.Fatal(err)
	}
	return owner, saved, w, workspace
}
func waitConnectorRun(t *testing.T, e *WorkflowEngine, id, state string) WorkflowRun {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		e.Tick(context.Background())
		r, err := e.store.WorkflowRun(Caller{Admin: true}, id)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status == state {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	r, _ := e.store.WorkflowRun(Caller{Admin: true}, id)
	t.Fatalf("want %s, got %+v", state, r)
	return r
}
func TestConnectorCommandUsesFrozenConfigAndPersistsReceipt(t *testing.T) {
	s := testStore(t)
	c, config, w, dir := connectorFixture(t, s, Connector{Name: "Read input", Kind: "command", Enabled: true, Executable: "/bin/cat", TimeoutSeconds: 3})
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "literal $(do not execute)", WorkspacePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	config.Executable = "/usr/bin/false"
	if _, err = s.SaveConnector(c, config); err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "")
	r = waitConnectorRun(t, engine, r.ID, "completed")
	var input map[string]any
	receipt := r.Steps[0].Receipt
	if receipt == nil || receipt.ExitCode == nil || *receipt.ExitCode != 0 || json.Unmarshal([]byte(receipt.Output), &input) != nil || input["input"] != r.Input {
		t.Fatalf("missing real command receipt: %+v", receipt)
	}
	if r.Connectors[config.ID].Revision != 1 {
		t.Fatal("not frozen")
	}
}
func TestConnectorAuthorityAndWorkspaceAreNotInheritedFromGraph(t *testing.T) {
	s := testStore(t)
	c, config, w, dir := connectorFixture(t, s, Connector{Name: "command", Kind: "command", Enabled: true, Executable: "/bin/true", TimeoutSeconds: 3})
	if _, err := s.SaveConnector(Caller{UserID: "unauthorized"}, config); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if _, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "bad workspace", WorkspacePath: t.TempDir()}); err == nil {
		t.Fatal("root escaped")
	}
	a := testAgent(t, s)
	_, _ = testUser(t, s, &a, "reader")
	reader, err := s.userCaller("reader")
	if err != nil {
		t.Fatal(err)
	}
	w.AuthorizedUsers = []string{reader.UserID}
	w, err = s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.StartWorkflow(reader, WorkflowStart{WorkflowID: w.ID, Input: "work", WorkspacePath: dir}); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "work", WorkspacePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	config.Enabled = false
	if _, err = s.SaveConnector(c, config); err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "")
	waitConnectorRun(t, e, r.ID, "failed")
}
func TestConnectorUnknownCommandIsNeverAutomaticallyReplayed(t *testing.T) {
	s := testStore(t)
	c, _, w, dir := connectorFixture(t, s, Connector{Name: "command", Kind: "command", Enabled: true, Executable: "/bin/cat", TimeoutSeconds: 3})
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "work", WorkspacePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate durable dispatch before a process crash. This is a unit failure
	// boundary, not a real end-to-end recovery claim.
	_, err = s.DB.Exec(`INSERT INTO workflow_connector_actions(run_id,seq,request) VALUES(?,1,'{}'); UPDATE workflow_steps SET status='running' WHERE run_id=? AND seq=1`, r.ID, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "")
	r = waitConnectorRun(t, e, r.ID, "failed")
	if err = e.Resume(c, r.ID, 1, "retry"); err == nil {
		t.Fatal("uncertain command replayed")
	}
	if err = e.Stop(c, r.ID, 1); err != nil {
		t.Fatal(err)
	}
	waitConnectorRun(t, e, r.ID, "stopped")
	if err = e.Return(c, r.ID, 1, "call", "已检查效果，重新执行"); err != nil {
		t.Fatal(err)
	}
	r = waitConnectorRun(t, e, r.ID, "completed")
	if r.Seq != 3 {
		t.Fatal(r)
	}
}
func TestConnectorBodyRejectsTraversalAndSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(outside, []byte("private"), 0600)
	os.Symlink(outside, filepath.Join(dir, "escape"))
	for _, p := range []string{"../secret", outside, "escape"} {
		if _, err := connectorBody(dir, p); err == nil {
			t.Fatalf("read %s", p)
		}
	}
	os.WriteFile(filepath.Join(dir, "body.md"), []byte("public"), 0600)
	b, err := connectorBody(dir, "body.md")
	if err != nil || b != "public" {
		t.Fatal(b, err)
	}
}

func TestConnectorFailureRouteAndCancellationKeepActualOutcome(t *testing.T) {
	s := testStore(t)
	c, config, w, dir := connectorFixture(t, s, Connector{Name: "Failure", Kind: "command", Enabled: true, Executable: "/usr/bin/false", TimeoutSeconds: 3})
	w.Nodes = append(w.Nodes, WorkflowNode{ID: "repair", Name: "Review failure", Kind: "approval"})
	w.Edges = append(w.Edges, WorkflowEdge{Source: "call", Route: "failed", Target: "repair"}, WorkflowEdge{Source: "repair", Route: "next", Target: "done"})
	var err error
	w, err = s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "expect nonzero", WorkspacePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "")
	r = waitConnectorRun(t, e, r.ID, "waiting")
	if r.Seq != 2 || r.Steps[0].Result.Route != "failed" || *r.Steps[0].Receipt.ExitCode == 0 {
		t.Fatal(r)
	}
	if err = s.CompleteWorkflowNode(c, r.ID, r.Seq, NodeResult{Route: "next", Summary: "Failure reviewed"}); err != nil {
		t.Fatal(err)
	}
	waitConnectorRun(t, e, r.ID, "completed")
	config.Executable = "/bin/sleep"
	config.Args = []string{"30"}
	config.TimeoutSeconds = 60
	_, err = s.SaveConnector(c, config)
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "stop real command", WorkspacePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	e.Tick(context.Background())
	time.Sleep(70 * time.Millisecond)
	if err = e.Stop(c, r.ID, 1); err != nil {
		t.Fatal(err)
	}
	r = waitConnectorRun(t, e, r.ID, "stopped")
	if r.Seq != 1 || r.Steps[0].Error == "" {
		t.Fatal(r)
	}
	if err = e.Resume(c, r.ID, 1, "do not replay"); err == nil {
		t.Fatal("interrupted command replayed")
	}
}

type connectorTransport func(*http.Request) (*http.Response, error)

func (f connectorTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestConnectorGitHubReconcilesLostResponseWithoutAnotherWrite(t *testing.T) {
	t.Setenv("CONNECTOR_TEST_TOKEN", "unit-secret")
	original := githubClient
	t.Cleanup(func() { githubClient = original })
	marker := "<!-- agent-platform:run:1 -->"
	created := false
	posts := 0
	githubClient = &http.Client{Transport: connectorTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.github.com" || r.Header.Get("Authorization") != "Bearer unit-secret" {
			t.Errorf("wrong credential destination")
		}
		body := "[]"
		if r.Method == "POST" {
			posts++
			created = true
			return nil, errors.New("response lost after server applied mutation")
		}
		if created {
			body = `[{"number":7,"html_url":"https://github.com/demo/repo/issues/7","body":"` + marker + `"}]`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	v := Connector{Kind: "github.issue_create", TokenEnv: "CONNECTOR_TEST_TOKEN"}
	request := connectorRequest{Endpoint: "/repos/demo/repo/issues", Lookup: "/repos/demo/repo/issues?state=all", Marker: marker, Payload: map[string]any{"title": "test"}}
	if _, err := executeGitHub(context.Background(), v, request, false); err == nil {
		t.Fatal("lost response treated as success")
	}
	receipt, err := executeGitHub(context.Background(), v, request, true)
	if err != nil || receipt.Number != 7 || !receipt.Recovered || posts != 1 {
		t.Fatal(receipt, err, posts)
	}
	created = false
	if _, err = executeGitHub(context.Background(), v, request, true); err == nil || posts != 1 {
		t.Fatal("missing receipt was replayed", err, posts)
	}
}
func TestConnectorSecretsAreNeitherInputNorOutput(t *testing.T) {
	t.Setenv("CONNECTOR_SECRET", "secret-test-value")
	s := testStore(t)
	c, _, w, dir := connectorFixture(t, s, Connector{Name: "Explicit environment", Kind: "command", Enabled: true, Executable: "/usr/bin/printenv", Args: []string{"TOKEN"}, EnvRefs: map[string]string{"TOKEN": "CONNECTOR_SECRET"}, TimeoutSeconds: 3})
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "test", WorkspacePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "")
	r = waitConnectorRun(t, e, r.ID, "completed")
	raw, _ := json.Marshal(r)
	if strings.Contains(string(raw), "secret-test-value") || !strings.Contains(r.Steps[0].Receipt.Output, "[redacted]") {
		t.Fatal("credential leaked")
	}
}

func TestConnectorBindsExistingIssueWithoutCreatingOne(t *testing.T) {
	s := testStore(t)
	c, v, w, dir := connectorFixture(t, s, Connector{Name: "Issue", Kind: "github.issue", Enabled: true, Repository: "owner/repo", TokenEnv: "TEST_GITHUB_TOKEN", TimeoutSeconds: 3})
	t.Setenv("TEST_GITHUB_TOKEN", "secret")
	w.Nodes[0].ConnectorInput = ConnectorInput{IssueParameter: "issue_number"}
	w, err := s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	original := githubClient
	t.Cleanup(func() { githubClient = original })
	calls := 0
	githubClient = &http.Client{Transport: connectorTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != "GET" || req.URL.Path != "/repos/owner/repo/issues/42" {
			t.Fatalf("unexpected write or wrong Issue: %s %s", req.Method, req.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"number":42,"html_url":"https://github.com/owner/repo/issues/42","body":"original"}`)), Header: http.Header{}}, nil
	})}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "original", WorkspacePath: dir, Parameters: map[string]string{"issue_number": "42"}})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "")
	r = waitConnectorRun(t, engine, r.ID, "completed")
	receipt := r.Steps[0].Receipt
	if calls != 1 || receipt.Number != 42 || receipt.Kind != "github.issue" {
		t.Fatal(calls, receipt, v)
	}
}

func TestIssueBindingRejectsPullRequestAndInvalidParameter(t *testing.T) {
	s := testStore(t)
	c, _, w, dir := connectorFixture(t, s, Connector{Name: "Issue", Kind: "github.issue", Enabled: true, Repository: "owner/repo", TokenEnv: "TEST_GITHUB_TOKEN", TimeoutSeconds: 3})
	t.Setenv("TEST_GITHUB_TOKEN", "secret")
	w.Nodes[0].ConnectorInput = ConnectorInput{IssueParameter: "issue_number"}
	w, err := s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	original := githubClient
	t.Cleanup(func() { githubClient = original })
	writes := 0
	githubClient = &http.Client{Transport: connectorTransport(func(req *http.Request) (*http.Response, error) {
		if req.Method != "GET" {
			writes++
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"number":42,"html_url":"https://github.com/owner/repo/pull/42","pull_request":{}}`)), Header: http.Header{}}, nil
	})}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "wrong resource", WorkspacePath: dir, Parameters: map[string]string{"issue_number": "42"}})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "")
	r = waitConnectorRun(t, engine, r.ID, "failed")
	if writes != 0 || !strings.Contains(r.Error, "not the requested Issue") {
		t.Fatal(writes, r.Error)
	}
	r.Parameters["issue_number"] = "42; echo unsafe"
	_, err = prepareConnectorRequest(r.Connectors[w.Nodes[0].ConnectorID], r, r.Steps[0])
	if err == nil {
		t.Fatal("unsafe parameter accepted")
	}
	delete(r.Parameters, "issue_number")
	req, err := prepareConnectorRequest(r.Connectors[w.Nodes[0].ConnectorID], r, r.Steps[0])
	if err != nil || req.ReadIssue || req.Endpoint != "/repos/owner/repo/issues" || req.Payload["title"] != "wrong resource" {
		t.Fatal(req, err)
	}
}

func TestConnectorTitlesUseTaskFirstLineAndPreserveBody(t *testing.T) {
	t.Setenv("TITLE_TEST_TOKEN", "test-only")
	dir := t.TempDir()
	for _, kind := range []string{"github.issue", "github.issue_create", "github.pull_request"} {
		t.Run(kind, func(t *testing.T) {
			input := "修复健康检查\r\n\r\n完整要求必须保留在正文。"
			run := WorkflowRun{ID: "run", Input: input, WorkspacePath: dir, Definition: Workflow{Nodes: []WorkflowNode{{ID: "call", ConnectorInput: ConnectorInput{Title: "{{input}}", Head: "branch", Base: "main"}}}}}
			request, err := prepareConnectorRequest(Connector{Kind: kind, WorkspaceRoot: dir, Repository: "owner/repo", TokenEnv: "TITLE_TEST_TOKEN"}, run, WorkflowStep{NodeID: "call", Seq: 1})
			if err != nil {
				t.Fatal(err)
			}
			if request.Payload["title"] != "修复健康检查" {
				t.Fatalf("unexpected title: %q", request.Payload["title"])
			}
			if !strings.HasPrefix(request.Payload["body"].(string), input) {
				t.Fatal("task body was lost")
			}
		})
	}
}

func TestPRTitleFollowsAuthoredDeliveryHeadingWithoutChangingBody(t *testing.T) {
	t.Setenv("DELIVERY_TITLE_TOKEN", "test-only")
	dir := t.TempDir()
	body := "# 【产品测试】验证计量边界\n\n实际交付及未验边界。\n"
	if err := os.WriteFile(filepath.Join(dir, "pr.md"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	run := WorkflowRun{ID: "run", Input: "【产品修复】原任务目标", WorkspacePath: dir, Definition: Workflow{Nodes: []WorkflowNode{{ID: "pr", ConnectorInput: ConnectorInput{Title: "{{input}}", BodyFile: "pr.md", Head: "branch", Base: "main"}}}}}
	connector := Connector{Kind: "github.pull_request", WorkspaceRoot: dir, Repository: "owner/repo", TokenEnv: "DELIVERY_TITLE_TOKEN"}
	for _, title := range []string{"{{input}}", ""} {
		run.Definition.Nodes[0].ConnectorInput.Title = title
		request, err := prepareConnectorRequest(connector, run, WorkflowStep{NodeID: "pr", Seq: 2})
		if err != nil {
			t.Fatal(err)
		}
		if request.Payload["title"] != "【产品测试】验证计量边界" {
			t.Fatal(request.Payload["title"])
		}
		if request.Payload["body"] != body+"\n\n"+request.Marker {
			t.Fatal("authored body changed")
		}
	}
	run.Definition.Nodes[0].ConnectorInput.Title = "明确指定的标题"
	request, err := prepareConnectorRequest(connector, run, WorkflowStep{NodeID: "pr", Seq: 2})
	if err != nil || request.Payload["title"] != "明确指定的标题" {
		t.Fatal(request, err)
	}
	run.Definition.Nodes[0].ConnectorInput.Title = "{{input}}"
	if err := os.WriteFile(filepath.Join(dir, "pr.md"), []byte("没有标题的旧正文"), 0600); err != nil {
		t.Fatal(err)
	}
	request, err = prepareConnectorRequest(connector, run, WorkflowStep{NodeID: "pr", Seq: 2})
	if err != nil || request.Payload["title"] != run.Input {
		t.Fatal(request, err)
	}
	connector.Kind = "github.issue_create"
	if err := os.WriteFile(filepath.Join(dir, "pr.md"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	request, err = prepareConnectorRequest(connector, run, WorkflowStep{NodeID: "pr", Seq: 2})
	if err != nil || request.Payload["title"] != run.Input {
		t.Fatal(request, err)
	}
}

func TestConnectorBodyFileUsesRunDirectoryAndRejectsEscapingSymlink(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TEST_DOC_TOKEN", "test-token")
	c := Connector{Kind: "github.pull_request", WorkspaceRoot: dir, Repository: "owner/repo", TokenEnv: "TEST_DOC_TOKEN"}
	r := WorkflowRun{ID: "run-a", WorkspacePath: dir, Input: "Task", Definition: Workflow{Nodes: []WorkflowNode{{ID: "pr", ConnectorInput: ConnectorInput{Head: "feature", Base: "main", BodyFile: "docs/{{run_id}}/pr.md"}}}}}
	docs := filepath.Join(dir, "docs", r.ID)
	if err := os.MkdirAll(docs, 0700); err != nil {
		t.Fatal(err)
	}
	body := filepath.Join(docs, "pr.md")
	if err := os.WriteFile(body, []byte("Skill-authored delivery"), 0600); err != nil {
		t.Fatal(err)
	}
	request, err := prepareConnectorRequest(c, r, WorkflowStep{NodeID: "pr", Seq: 1})
	if err != nil || !strings.Contains(fmt.Sprint(request.Payload["body"]), "Skill-authored delivery") {
		t.Fatalf("%+v %v", request, err)
	}
	if err := os.Remove(body); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "private.md")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, body); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareConnectorRequest(c, r, WorkflowStep{NodeID: "pr", Seq: 1}); err == nil {
		t.Fatal("escaped body accepted")
	}
}

func TestPullRequestIssueAssociationDoesNotDeclareCompletion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TEST_PR_TOKEN", "test-only")
	c := Connector{Kind: "github.pull_request", WorkspaceRoot: dir, Repository: "owner/repo", TokenEnv: "TEST_PR_TOKEN"}
	for _, body := range []string{"Partial delivery; supplier validation remains blocked.", "Full delivery verified.\n\nCloses #42"} {
		r := WorkflowRun{ID: "run", WorkspacePath: dir, Input: body, Definition: Workflow{Nodes: []WorkflowNode{{ID: "pr", ConnectorInput: ConnectorInput{Head: "feature", Base: "main", IssueNumber: 42}}}}}
		request, err := prepareConnectorRequest(c, r, WorkflowStep{NodeID: "pr", Seq: 1})
		if err != nil {
			t.Fatal(err)
		}
		want := body + "\n\n<!-- agent-platform:run:1 -->\n\nRefs #42"
		if request.Payload["body"] != want {
			t.Fatalf("connector changed completion intent: %q", request.Payload["body"])
		}
	}
}

func TestConnectorTimeoutRecoveryKeepsIdentityAndRequiresExplicitReturn(t *testing.T) {
	s := testStore(t)
	c, config, w, dir := connectorFixture(t, s, Connector{Name: "bounded test", Kind: "command", Enabled: true, Executable: "/bin/sh", Args: []string{"-c", "printf started; sleep 1.2; printf finished"}, TimeoutSeconds: 1})
	run, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "test", WorkspacePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "")
	engine.Tick(context.Background())
	config.TimeoutSeconds = 3
	config.Executable = "/usr/bin/false"
	config.Args = nil
	if _, err := s.SaveConnector(c, config); err != nil {
		t.Fatal(err)
	}
	failed := waitConnectorRun(t, engine, run.ID, "failed")
	if failed.Steps[0].Receipt.TimeoutSeconds != 1 {
		t.Fatal("active execution deadline changed with admin policy")
	}
	if err := engine.Resume(c, run.ID, 1, "retry"); err == nil {
		t.Fatal("budget change replayed uncertain command")
	}
	if err := engine.Stop(c, run.ID, 1); err != nil {
		t.Fatal(err)
	}
	waitConnectorRun(t, engine, run.ID, "stopped")
	if err := engine.Return(c, run.ID, 1, "call", "inspected effects; retry original command with revised admin time budget"); err != nil {
		t.Fatal(err)
	}
	completed := waitConnectorRun(t, engine, run.ID, "completed")
	if completed.Connectors[config.ID].TimeoutSeconds != 1 || completed.Steps[0].Receipt.TimeoutSeconds != 1 || completed.Steps[1].Receipt.TimeoutSeconds != 3 {
		t.Fatal("history/frozen identity or actual execution budget changed incorrectly")
	}
	if completed.Steps[1].Receipt.Output != "startedfinished" || *completed.Steps[1].Receipt.ExitCode != 0 {
		t.Fatal("live executable replaced frozen command or old budget repeated")
	}
}

func TestConnectorPullRequestReworkUpdatesOwnedPRAndRecoversLostResponse(t *testing.T) {
	t.Setenv("PR_REWORK_TOKEN", "test-only-token")
	original := githubClient
	t.Cleanup(func() { githubClient = original })
	dir := t.TempDir()
	v := Connector{ID: "pr", Kind: "github.pull_request", Repository: "demo/repo", TokenEnv: "PR_REWORK_TOKEN", WorkspaceRoot: dir}
	r := WorkflowRun{ID: "run", Input: "updated delivery", WorkspacePath: dir, Definition: Workflow{Nodes: []WorkflowNode{{ID: "pr", Kind: "connector", ConnectorID: "pr", ConnectorInput: ConnectorInput{Head: "workflow/run", Base: "main"}}}}}
	r.Steps = []WorkflowStep{{Seq: 3, NodeID: "pr", Status: "completed", Receipt: &ConnectorReceipt{Kind: "github.pull_request", Number: 7, URL: "https://github.com/demo/repo/pull/7"}}}
	request, err := prepareConnectorRequest(v, r, WorkflowStep{Seq: 9, NodeID: "pr"})
	if err != nil {
		t.Fatal(err)
	}
	// A confirmed external PR still exists if the execution was then cancelled.
	r.Steps[0].Status = "cancelled"
	cancelledRequest, err := prepareConnectorRequest(v, r, WorkflowStep{Seq: 9, NodeID: "pr"})
	if err != nil || cancelledRequest.UpdatePR != request.UpdatePR || cancelledRequest.Endpoint != request.Endpoint {
		t.Fatal("cancelled receipt lost PR identity", cancelledRequest, err)
	}
	body := "original delivery"
	patches := 0
	githubClient = &http.Client{Transport: connectorTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/repos/demo/repo/pulls/7" {
			t.Errorf("unexpected endpoint %s", req.URL.Path)
			return nil, errors.New("unexpected endpoint")
		}
		if req.Method == "PATCH" {
			patches++
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if len(payload) != 2 || payload["title"] != "updated delivery" {
				t.Fatal("unexpected PR mutation", payload)
			}
			body = payload["body"].(string)
			return nil, errors.New("response lost after PATCH")
		}
		if req.Method != "GET" {
			t.Fatal("unexpected write", req.Method)
		}
		raw, _ := json.Marshal(map[string]any{"number": 7, "html_url": "https://github.com/demo/repo/pull/7", "state": "open", "body": body, "head": map[string]any{"sha": "new-head", "ref": "workflow/run", "repo": map[string]any{"full_name": "demo/repo"}}, "base": map[string]any{"ref": "main"}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(raw)), Header: http.Header{}}, nil
	})}
	if _, err = executeGitHub(context.Background(), v, request, false); err == nil || patches != 1 {
		t.Fatal("first PATCH not uncertain", err, patches)
	}
	receipt, err := executeGitHub(context.Background(), v, request, true)
	if err != nil || !receipt.Recovered || receipt.Number != 7 || receipt.HeadSHA != "new-head" || patches != 1 {
		t.Fatal(receipt, err, patches)
	}
}

func TestConnectorPullRequestReworkRefusesClosedChangedAndUnknown(t *testing.T) {
	t.Setenv("PR_REWORK_TOKEN", "test-only-token")
	original := githubClient
	t.Cleanup(func() { githubClient = original })
	for _, mode := range []string{"closed", "head", "base", "repo", "number", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			v := Connector{Kind: "github.pull_request", Repository: "demo/repo", TokenEnv: "PR_REWORK_TOKEN"}
			request := connectorRequest{UpdatePR: 7, Endpoint: "/repos/demo/repo/pulls/7", Marker: "new-marker", Payload: map[string]any{"head": "workflow/run", "base": "main", "title": "updated", "body": "new-marker"}}
			githubClient = &http.Client{Transport: connectorTransport(func(req *http.Request) (*http.Response, error) {
				if req.Method != "GET" {
					t.Fatal("unverified update attempted", req.Method)
				}
				obj := map[string]any{"number": 7, "html_url": "https://github.com/demo/repo/pull/7", "state": "open", "body": "old-marker", "head": map[string]any{"sha": "sha", "ref": "workflow/run", "repo": map[string]any{"full_name": "demo/repo"}}, "base": map[string]any{"ref": "main"}}
				switch mode {
				case "closed":
					obj["state"] = "closed"
				case "head":
					obj["head"].(map[string]any)["ref"] = "other"
				case "base":
					obj["base"].(map[string]any)["ref"] = "other"
				case "repo":
					obj["head"].(map[string]any)["repo"] = map[string]any{"full_name": "other/repo"}
				case "number":
					obj["number"] = 8
				}
				raw, _ := json.Marshal(obj)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(raw)), Header: http.Header{}}, nil
			})}
			_, err := executeGitHub(context.Background(), v, request, mode == "unknown")
			if err == nil {
				t.Fatal("unconfirmed PR accepted")
			}
			var notSent *githubNotSentError
			if errors.As(err, &notSent) == (mode == "unknown") {
				t.Fatal("write certainty incorrect", err)
			}
		})
	}
}
