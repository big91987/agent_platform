package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// This is opt-in: it creates one real Issue, then closes that exact Issue.
// Only response delivery is fault-injected. GitHub requests and resources are real.
func TestWorkflowGitHubLiveLostResponse(t *testing.T) {
	repo := os.Getenv("WORKFLOW_LIVE_TEST_REPOSITORY")
	if repo == "" {
		t.Skip("set WORKFLOW_LIVE_TEST_REPOSITORY to an isolated test repository")
	}
	if os.Getenv("WORKFLOW_LIVE_TEST_TOKEN") == "" {
		t.Fatal("WORKFLOW_LIVE_TEST_TOKEN is required")
	}
	original := githubClient
	var posts atomic.Int32
	var created githubObject
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	githubClient = &http.Client{Timeout: 30 * time.Second, CheckRedirect: original.CheckRedirect, Transport: connectorTransport(func(r *http.Request) (*http.Response, error) {
		response, err := transport.RoundTrip(r)
		if err != nil {
			return nil, err
		}
		if r.Method == "POST" && r.URL.Path == "/repos/"+repo+"/issues" {
			posts.Add(1)
			if response.StatusCode == http.StatusCreated {
				defer response.Body.Close()
				if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
					return nil, err
				}
				return nil, errors.New("live acceptance: response deliberately dropped after real GitHub creation")
			}
		}
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/issues") && posts.Load() > 0 {
			body, readErr := io.ReadAll(response.Body)
			response.Body.Close()
			if readErr != nil {
				return nil, readErr
			}
			response.Body = io.NopCloser(bytes.NewReader(body))
			var got []githubObject
			_ = json.Unmarshal(body, &got)
			nums := []int{}
			matched := false
			for _, obj := range got {
				nums = append(nums, obj.Number)
				if obj.Number == created.Number {
					matched = true
				}
			}
			t.Logf("recovery lookup: status=%d issue_ids=%v created_visible=%t cache_control=%q age=%q", response.StatusCode, nums, matched, response.Header.Get("Cache-Control"), response.Header.Get("Age"))
		}
		return response, nil
	})}
	defer func() { githubClient = original }()
	v := Connector{Kind: "github.issue_create", Repository: repo, TokenEnv: "WORKFLOW_LIVE_TEST_TOKEN"}
	defer func() {
		if created.Number > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := githubRequest(ctx, v, "PATCH", "/repos/"+repo+"/issues/"+strconv.Itoa(created.Number), map[string]any{"state": "closed"}, nil); err != nil {
				t.Errorf("close test Issue %d: %v", created.Number, err)
			}
		}
	}()
	store := testStore(t)
	handler := NewServer(store, nil, nil, "live-test-password", "http://localhost")
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { defer close(stopped); handler.RunWorkflows(ctx) }()
	defer func() { cancel(); <-stopped }()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	call := func(method, path string, body, out any) {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Platform-Request", "1")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			b, _ := io.ReadAll(res.Body)
			t.Fatalf("%s %s: %d %s", method, path, res.StatusCode, b)
		}
		if out != nil {
			if err = json.NewDecoder(res.Body).Decode(out); err != nil {
				t.Fatal(err)
			}
		}
	}
	call("POST", "/api/login", map[string]string{"username": "admin", "password": "live-test-password"}, nil)
	workspace := t.TempDir()
	var connector Connector
	call("POST", "/api/connectors", Connector{Name: "Live GitHub response recovery", Kind: v.Kind, Repository: repo, TokenEnv: v.TokenEnv, WorkspaceRoot: workspace, Enabled: true, TimeoutSeconds: 60}, &connector)
	var graph Workflow
	call("POST", "/api/workflows", map[string]any{
		"name": "Live GitHub response recovery", "enabled": true, "entry": "issue",
		"nodes": []any{map[string]any{"id": "issue", "name": "Real test Issue", "kind": "connector", "connector_id": connector.ID}, map[string]any{"id": "done", "name": "Done", "kind": "end"}},
		"edges": []any{map[string]any{"source": "issue", "route": "next", "target": "done"}},
	}, &graph)
	var run WorkflowRun
	call("POST", "/api/workflow-runs", WorkflowStart{WorkflowID: graph.ID, Input: "[Harness acceptance] Real GitHub response-loss recovery; no product work. Test Issue will be closed after verification.", WorkspacePath: workspace}, &run)
	path := "/api/workflow-runs/" + run.ID
	wait := func(status string) WorkflowRun {
		t.Helper()
		deadline := time.Now().Add(60 * time.Second)
		var last WorkflowRun
		for time.Now().Before(deadline) {
			var r WorkflowRun
			call("GET", path, nil, &r)
			last = r
			if r.Status == status {
				return r
			}
			time.Sleep(200 * time.Millisecond)
		}
		b, _ := json.Marshal(last)
		t.Fatalf("Run did not reach %s: %s", status, b)
		return WorkflowRun{}
	}
	failed := wait("failed")
	if posts.Load() != 1 || !strings.Contains(failed.Error, "response deliberately dropped") {
		t.Fatalf("wrong initial outcome: posts=%d error=%s", posts.Load(), failed.Error)
	}
	call("POST", path+"/resume", map[string]any{"seq": 1, "message": "Reconcile real GitHub resource; do not repeat the POST"}, nil)
	complete := wait("completed")
	receipt := complete.Steps[0].Receipt
	if receipt == nil || !receipt.Recovered || receipt.Number != created.Number || posts.Load() != 1 {
		t.Fatalf("wrong recovered receipt: %#v posts=%d", receipt, posts.Load())
	}
	marker := "<!-- agent-platform:" + run.ID + ":1 -->"
	var objects []githubObject
	if err := githubRequest(context.Background(), v, "GET", "/repos/"+repo+"/issues?state=all&sort=created&direction=desc&per_page=100&_platform_reconcile="+newID(), nil, &objects); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, obj := range objects {
		if strings.Contains(obj.Body, marker) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected one actual GitHub resource, found %d", count)
	}
	evidence := map[string]any{"passed": true, "repository": repo, "issue_url": created.URL, "posts": posts.Load(), "matching_resources": count, "failed": failed, "recovered": complete, "fault_boundary": "client transport drops the actual GitHub POST response; no fake server or resource"}
	if output := os.Getenv("WORKFLOW_LIVE_TEST_EVIDENCE"); output != "" {
		b, err := json.MarshalIndent(evidence, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.MkdirAll(filepath.Dir(output), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(output, append(b, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("real Issue recovered once: %s; run=%s; POST count=%d", created.URL, run.ID, posts.Load())
}

// The first comment lookup fails before transport; retry sends one real comment.
// This verifies the public notification retry endpoint against actual GitHub.
func TestWorkflowGitHubLiveHookPreflightRetry(t *testing.T) {
	repo := os.Getenv("WORKFLOW_LIVE_TEST_REPOSITORY")
	if repo == "" {
		t.Skip("set WORKFLOW_LIVE_TEST_REPOSITORY to an isolated test repository")
	}
	if os.Getenv("WORKFLOW_LIVE_TEST_TOKEN") == "" {
		t.Fatal("WORKFLOW_LIVE_TEST_TOKEN is required")
	}
	v := Connector{Kind: "github.issue_comment", Repository: repo, TokenEnv: "WORKFLOW_LIVE_TEST_TOKEN"}
	var issue githubObject
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := githubRequest(ctx, v, "POST", "/repos/"+repo+"/issues", map[string]any{"title": "[Harness acceptance] Notification preflight retry", "body": "Isolated notification recovery verification; no product changes. Closed after verification."}, &issue); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		if err := githubRequest(cleanup, v, "PATCH", "/repos/"+repo+"/issues/"+strconv.Itoa(issue.Number), map[string]any{"state": "closed"}, nil); err != nil {
			t.Error(err)
		}
	}()
	original := githubClient
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	var reads, posts atomic.Int32
	endpoint := "/repos/" + repo + "/issues/" + strconv.Itoa(issue.Number) + "/comments"
	githubClient = &http.Client{Timeout: 30 * time.Second, CheckRedirect: original.CheckRedirect, Transport: connectorTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == endpoint {
			if r.Method == "GET" && reads.Add(1) == 1 {
				return nil, errors.New("live acceptance: lookup fails before any write")
			}
			if r.Method == "POST" {
				posts.Add(1)
			}
		}
		return transport.RoundTrip(r)
	})}
	defer func() { githubClient = original }()
	s := testStore(t)
	c, w, _ := runFixture(t, s)
	a := testAgent(t, s)
	w.Nodes[0].Kind, w.Nodes[0].AgentID = "agent", a.ID
	v.Name, v.WorkspaceRoot, v.Enabled = "Live notification retry", t.TempDir(), true
	v, err := s.SaveConnector(c, v)
	if err != nil {
		t.Fatal(err)
	}
	w.Hooks = []WorkflowHook{{Event: "node.started", ConnectorID: v.ID, Input: ConnectorInput{IssueNumber: issue.Number}, Body: "[Harness acceptance] notification retry {{run_id}}"}}
	w, err = s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	h := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, h)
	start := workflowRequest(t, h, cookie, "POST", "/api/workflow-runs", WorkflowStart{WorkflowID: w.ID, Input: "Notification recovery only", WorkspacePath: v.WorkspaceRoot})
	if start.Code != 201 {
		t.Fatal(start.Code, start.Body.String())
	}
	var r WorkflowRun
	if err = json.Unmarshal(start.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	h.workflows.Tick(ctx)
	if err = h.workflows.collectWorkflowHooks(); err != nil {
		t.Fatal(err)
	}
	if err = h.workflows.dispatchWorkflowHooks(ctx); err != nil {
		t.Fatal(err)
	}
	h.workflows.connectorWG.Wait()
	before, err := s.WorkflowHookDeliveries(c, r.ID)
	if err != nil || len(before) != 1 || before[0].Status != "failed" || posts.Load() != 0 {
		t.Fatalf("before retry: %#v posts=%d err=%v", before, posts.Load(), err)
	}
	retry := workflowRequest(t, h, cookie, "POST", "/api/workflow-runs/"+r.ID+"/notifications/"+before[0].ID+"/retry", nil)
	if retry.Code != 202 {
		t.Fatal(retry.Code, retry.Body.String())
	}
	if err = h.workflows.dispatchWorkflowHooks(ctx); err != nil {
		t.Fatal(err)
	}
	h.workflows.connectorWG.Wait()
	after, err := s.WorkflowHookDeliveries(c, r.ID)
	if err != nil || after[0].Status != "succeeded" || posts.Load() != 1 {
		t.Fatalf("after retry: %#v posts=%d err=%v", after, posts.Load(), err)
	}
	var comments []githubObject
	if err = githubRequest(ctx, v, "GET", endpoint+"?per_page=100&_platform_reconcile="+newID(), nil, &comments); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, comment := range comments {
		if strings.Contains(comment.Body, "<!-- agent-platform-hook:"+before[0].ID+" -->") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one real comment, got %d", count)
	}
	evidence := map[string]any{"passed": true, "issue_url": issue.URL, "posts": posts.Load(), "matching_comments": count, "before": before, "after": after, "fault_boundary": "first read fails before transport; retry endpoint sends one actual GitHub comment"}
	if output := os.Getenv("WORKFLOW_LIVE_TEST_EVIDENCE"); output != "" {
		raw, _ := json.MarshalIndent(evidence, "", "  ")
		if err = os.WriteFile(output, append(raw, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("real notification retry verified: %s; POST count=%d", issue.URL, posts.Load())
}

// Opt-in integration: posts two labelled Harness comments to an existing Issue.
// Approval originates from a controlled AwaitToolApproval callback, not a product
// Agent or real provider call. The owner decision uses the public HTTP endpoint.
func TestWorkflowGitHubLiveApprovalLifecycle(t *testing.T) {
	repo, ref := os.Getenv("WORKFLOW_APPROVAL_LIVE_REPOSITORY"), os.Getenv("WORKFLOW_APPROVAL_LIVE_CREDENTIAL")
	if repo == "" {
		t.Skip("set WORKFLOW_APPROVAL_LIVE_REPOSITORY, ISSUE and CREDENTIAL for isolated notification verification")
	}
	issue, err := strconv.Atoi(os.Getenv("WORKFLOW_APPROVAL_LIVE_ISSUE"))
	if err != nil || issue < 1 || ref == "" {
		t.Fatal("existing Issue and credential reference required")
	}
	s := testStore(t)
	caller, w, _ := runFixture(t, s)
	a := testAgent(t, s)
	w.Nodes[0].Kind, w.Nodes[0].AgentID = "agent", a.ID
	w.Nodes[0].Name = "【Harness验证】隔离审批通知（非产品运行）"
	v, err := s.SaveConnector(caller, Connector{Name: "Live approval notification", Kind: "github.issue_comment", Repository: repo, TokenEnv: ref, WorkspaceRoot: t.TempDir(), Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	w.Hooks = []WorkflowHook{{Event: "agent.reply.completed", ConnectorID: v.ID, Input: ConnectorInput{IssueNumber: issue}, Body: "{{text}}"}}
	w, err = s.SaveWorkflow(caller, w)
	if err != nil {
		t.Fatal(err)
	}
	h := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, h)
	start := workflowRequest(t, h, cookie, "POST", "/api/workflow-runs", WorkflowStart{WorkflowID: w.ID, Input: "Isolated Harness approval lifecycle; no product action", WorkspacePath: v.WorkspaceRoot})
	if start.Code != 201 {
		t.Fatal(start.Code, start.Body.String())
	}
	var r WorkflowRun
	if err = json.Unmarshal(start.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	h.workflows.Tick(ctx)
	c, m, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.AwaitToolApproval(ctx, c, m, json.RawMessage(`{"platform_permission_request":true,"message":"isolated Harness approval; no command executed"}`))
	}()
	defer func() { cancel(); <-done }()
	approval := waitApproval(t, s, c.ID)
	dispatch := func() {
		t.Helper()
		if err := h.workflows.collectWorkflowHooks(); err != nil {
			t.Fatal(err)
		}
		if err := h.workflows.dispatchWorkflowHooks(ctx); err != nil {
			t.Fatal(err)
		}
		h.workflows.connectorWG.Wait()
	}
	dispatch()
	before, err := s.WorkflowHookDeliveries(caller, r.ID)
	if err != nil || len(before) != 1 || before[0].Status != "succeeded" {
		t.Fatalf("pending approval not published: %+v err=%v", before, err)
	}
	decision := workflowRequest(t, h, cookie, "POST", "/api/conversations/"+c.ID+"/approvals/"+approval.ID, map[string]string{"decision": "accept"})
	if decision.Code != 200 {
		t.Fatal(decision.Code, decision.Body.String())
	}
	<-done
	dispatch()
	dispatch()
	after, err := s.WorkflowHookDeliveries(caller, r.ID)
	if err != nil || len(after) != 2 || after[1].Status != "succeeded" {
		t.Fatalf("resolution missing/duplicated: %+v err=%v", after, err)
	}
	var comments []githubObject
	if err = githubRequest(ctx, v, "GET", "/repos/"+repo+"/issues/"+strconv.Itoa(issue)+"/comments?per_page=100&_platform_reconcile="+newID(), nil, &comments); err != nil {
		t.Fatal(err)
	}
	for _, d := range after {
		count := 0
		for _, obj := range comments {
			if strings.Contains(obj.Body, "<!-- agent-platform-hook:"+d.ID+" -->") {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("notification %s has %d actual comments", d.Event, count)
		}
	}
	evidence := map[string]any{"run_id": r.ID, "notifications": after, "matching_comments": 2, "owner_decision_http": decision.Code, "scope": "controlled platform approval callback + public owner decision + actual GitHub Hook; no product Agent, no production service upgrade"}
	if output := os.Getenv("WORKFLOW_APPROVAL_LIVE_EVIDENCE"); output != "" {
		raw, _ := json.MarshalIndent(evidence, "", "  ")
		if err = os.WriteFile(output, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("actual approval lifecycle comments: %s / %s", after[0].Receipt.URL, after[1].Receipt.URL)
}
