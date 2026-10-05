package platform

import (
	"context"
	"encoding/json"
	"errors"
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
