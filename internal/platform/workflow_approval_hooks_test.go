package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWorkflowApprovalUsesExistingIssueChannelOnce(t *testing.T) {
	s := testStore(t)
	caller, w, _ := runFixture(t, s)
	a := testAgent(t, s)
	w.Nodes[0].Kind, w.Nodes[0].AgentID = "agent", a.ID
	dir := t.TempDir()
	v, err := s.SaveConnector(caller, Connector{Name: "Issue replies", Kind: "github.issue_comment", Repository: "demo/repo", TokenEnv: "APPROVAL_HOOK_TEST", WorkspaceRoot: dir, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	w.Hooks = []WorkflowHook{{Event: "agent.reply.completed", ConnectorID: v.ID, Input: ConnectorInput{IssueNumber: 1}, Body: "{{text}}"}}
	w, err = s.SaveWorkflow(caller, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: w.ID, Input: "Use the browser", WorkspacePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "http://platform.example")
	engine.Tick(context.Background())
	c, m, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.AwaitToolApproval(ctx, c, m, json.RawMessage(`{"command":"secret-command","reason":"secret-reason","platform_permission_request":true}`))
	}()
	approval := waitApproval(t, s, c.ID)
	for range 3 {
		if err = engine.collectWorkflowHooks(); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.WorkflowHookDeliveries(caller, r.ID)
	if err != nil || len(list) != 1 || list[0].Event != "approval.requested" {
		t.Fatalf("pending approval is invisible in the Issue channel: %+v, %v", list, err)
	}
	var fields string
	if err = s.DB.QueryRow(`SELECT fields FROM workflow_hook_deliveries WHERE id=?`, list[0].ID).Scan(&fields); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fields, "secret-") || !strings.Contains(fields, "/conversations/"+c.ID) {
		t.Fatal("approval notification leaks request or has no action link")
	}
	if err = s.DecideToolApproval(c.ID, approval.ID, "accept"); err != nil {
		t.Fatal(err)
	}
	<-done
	for range 3 {
		if err = engine.collectWorkflowHooks(); err != nil {
			t.Fatal(err)
		}
	}
	list, _ = s.WorkflowHookDeliveries(caller, r.ID)
	if len(list) != 2 || list[1].Event != "approval.resolved" {
		t.Fatalf("approval resolution missing or duplicated: %+v", list)
	}
	// An approval handled before notification dispatch must not post a stale wait.
	if err = engine.dispatchWorkflowHooks(context.Background()); err != nil {
		t.Fatal(err)
	}
	engine.connectorWG.Wait()
	list, _ = s.WorkflowHookDeliveries(caller, r.ID)
	if list[0].Status != "skipped" {
		t.Fatalf("obsolete approval wait was sent: %+v", list[0])
	}
	after, _ := s.WorkflowRun(caller, r.ID)
	beforeRaw, _ := json.Marshal(r.Definition)
	afterRaw, _ := json.Marshal(after.Definition)
	if after.Seq != r.Seq || string(beforeRaw) != string(afterRaw) {
		t.Fatal("approval notification rewrote the frozen graph or advanced execution")
	}
}

func TestWorkflowApprovalDoesNotBackfillHistoryOrDuplicateExplicitRule(t *testing.T) {
	for _, handledBeforeCollection := range []bool{true, false} {
		t.Run(map[bool]string{true: "already handled", false: "explicit rule"}[handledBeforeCollection], func(t *testing.T) {
			s := testStore(t)
			caller, w, _ := runFixture(t, s)
			a := testAgent(t, s)
			w.Nodes[0].Kind, w.Nodes[0].AgentID = "agent", a.ID
			dir := t.TempDir()
			v, err := s.SaveConnector(caller, Connector{Name: "Issue replies", Kind: "github.issue_comment", Repository: "demo/repo", TokenEnv: "APPROVAL_HOOK_TEST", WorkspaceRoot: dir, Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			input := ConnectorInput{IssueNumber: 1}
			w.Hooks = []WorkflowHook{{Event: "agent.reply.completed", ConnectorID: v.ID, Input: input, Body: "{{text}}"}}
			if !handledBeforeCollection {
				w.Hooks = append(w.Hooks, WorkflowHook{Event: "approval.requested", ConnectorID: v.ID, Input: input, Body: "custom pending {{conversation_url}}"})
			}
			w, err = s.SaveWorkflow(caller, w)
			if err != nil {
				t.Fatal(err)
			}
			r, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: w.ID, Input: "Use the browser", WorkspacePath: dir})
			if err != nil {
				t.Fatal(err)
			}
			engine := NewWorkflowEngine(s, nil, "http://platform.example")
			engine.Tick(context.Background())
			c, m, err := s.Claim()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				defer close(done)
				s.AwaitToolApproval(ctx, c, m, json.RawMessage(`{"message":"approval"}`))
			}()
			approval := waitApproval(t, s, c.ID)
			if handledBeforeCollection {
				if err = s.DecideToolApproval(c.ID, approval.ID, "accept"); err != nil {
					t.Fatal(err)
				}
				<-done
			}
			if err = engine.collectWorkflowHooks(); err != nil {
				t.Fatal(err)
			}
			list, err := s.WorkflowHookDeliveries(caller, r.ID)
			if err != nil || handledBeforeCollection && len(list) != 0 || !handledBeforeCollection && (len(list) != 1 || list[0].Event != "approval.requested") {
				t.Fatalf("unexpected historical or duplicate notification: %+v, %v", list, err)
			}
			if !handledBeforeCollection {
				cancel()
				<-done
				if err = engine.collectWorkflowHooks(); err != nil {
					t.Fatal(err)
				}
				list, _ = s.WorkflowHookDeliveries(caller, r.ID)
				var fields string
				if len(list) != 2 || list[1].Event != "approval.resolved" {
					t.Fatalf("cancelled approval was not resolved: %+v", list)
				}
				if err = s.DB.QueryRow(`SELECT fields FROM workflow_hook_deliveries WHERE id=?`, list[1].ID).Scan(&fields); err != nil || !strings.Contains(fields, "已失效") {
					t.Fatal("cancelled request presented as accepted", err)
				}
			}
		})
	}
}

func approvalHookFixture(t *testing.T, hooks func(string, ConnectorInput, string) []WorkflowHook) (*Store, Caller, WorkflowRun, *WorkflowEngine, Conversation, ToolApproval, func()) {
	t.Helper()
	s := testStore(t)
	caller, w, _ := runFixture(t, s)
	a := testAgent(t, s)
	w.Nodes[0].Kind, w.Nodes[0].AgentID = "agent", a.ID
	dir := t.TempDir()
	v, err := s.SaveConnector(caller, Connector{Name: "notify", Kind: "github.issue_comment", Repository: "demo/repo", TokenEnv: "HOOK_TEST_TOKEN", WorkspaceRoot: dir, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	w.Hooks = hooks(v.ID, ConnectorInput{IssueNumber: 1}, w.Nodes[0].ID)
	w, err = s.SaveWorkflow(caller, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: w.ID, Input: "browser", WorkspacePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "http://platform.example")
	e.Tick(context.Background())
	c, m, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); s.AwaitToolApproval(ctx, c, m, json.RawMessage(`{"message":"approval"}`)) }()
	approval := waitApproval(t, s, c.ID)
	return s, caller, r, e, c, approval, func() { cancel(); <-done }
}

func TestWorkflowApprovalNodeOverrideAndResolvedOnly(t *testing.T) {
	for _, resolvedOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "node override", true: "resolved only"}[resolvedOnly], func(t *testing.T) {
			s, caller, r, e, c, approval, cleanup := approvalHookFixture(t, func(id string, input ConnectorInput, node string) []WorkflowHook {
				if resolvedOnly {
					return []WorkflowHook{{Event: "approval.resolved", ConnectorID: id, Input: input, Body: "{{approval_status}}"}}
				}
				return []WorkflowHook{{Event: "agent.reply.completed", ConnectorID: id, Input: input, Body: "{{text}}"}, {Event: "approval.requested", Node: node, ConnectorID: id, Input: input, Body: "custom {{conversation_url}}"}}
			})
			defer cleanup()
			if err := e.collectWorkflowHooks(); err != nil {
				t.Fatal(err)
			}
			list, _ := s.WorkflowHookDeliveries(caller, r.ID)
			if !resolvedOnly && len(list) != 1 {
				t.Fatalf("node override duplicated notification: %+v", list)
			}
			if err := s.DecideToolApproval(c.ID, approval.ID, "decline"); err != nil {
				t.Fatal(err)
			}
			if err := e.collectWorkflowHooks(); err != nil {
				t.Fatal(err)
			}
			list, _ = s.WorkflowHookDeliveries(caller, r.ID)
			want := 2
			if resolvedOnly {
				want = 1
			}
			if len(list) != want || list[len(list)-1].Event != "approval.resolved" {
				t.Fatalf("resolution missing: %+v", list)
			}
		})
	}
}

func TestWorkflowApprovalResolvedDuringLookupIsNotPosted(t *testing.T) {
	s, caller, r, e, c, approval, cleanup := approvalHookFixture(t, func(id string, input ConnectorInput, node string) []WorkflowHook {
		return []WorkflowHook{{Event: "agent.reply.completed", ConnectorID: id, Input: input, Body: "{{text}}"}}
	})
	defer cleanup()
	t.Setenv("HOOK_TEST_TOKEN", "test-token")
	original := githubClient
	defer func() { githubClient = original }()
	posts := 0
	githubClient = &http.Client{Transport: connectorTransport(func(req *http.Request) (*http.Response, error) {
		body := `[]`
		if req.Method == "GET" {
			if err := s.DecideToolApproval(c.ID, approval.ID, "accept"); err != nil {
				return nil, err
			}
		} else {
			posts++
			body = `{"html_url":"https://github.com/demo/repo/issues/1#issuecomment-2"}`
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
	list, _ := s.WorkflowHookDeliveries(caller, r.ID)
	if posts != 0 || len(list) != 1 || list[0].Status != "skipped" {
		t.Fatalf("approved during lookup posted stale wait: posts=%d deliveries=%+v", posts, list)
	}
}

func TestWorkflowApprovalResolvedDuringAppRefreshIsNotPosted(t *testing.T) {
	v, _ := githubAppFixture(t)
	s, _, _, _, c, approval, cleanup := approvalHookFixture(t, func(id string, input ConnectorInput, node string) []WorkflowHook {
		return []WorkflowHook{{Event: "agent.reply.completed", ConnectorID: id, Input: input, Body: "{{text}}"}}
	})
	defer cleanup()
	minted, posts := 0, 0
	githubClient = &http.Client{Transport: connectorTransport(func(req *http.Request) (*http.Response, error) {
		body := `[]`
		if strings.HasSuffix(req.URL.Path, "/access_tokens") {
			minted++
			if minted == 2 {
				if err := s.DecideToolApproval(c.ID, approval.ID, "accept"); err != nil {
					return nil, err
				}
			}
			body = fmt.Sprintf(`{"token":"isolated-installation-token","expires_at":%q}`, time.Now().Add(45*time.Second).UTC().Format(time.RFC3339))
		} else if req.Method == "POST" {
			posts++
			body = `{"html_url":"https://github.com/demo/repo/issues/1#issuecomment-2"}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	_, err := executeGitHubBeforeWrite(context.Background(), Connector{Kind: "github.issue_comment", Repository: "demo/repo", TokenEnv: v.TokenEnv}, connectorRequest{Lookup: "/repos/demo/repo/issues/1/comments?per_page=100", Endpoint: "/repos/demo/repo/issues/1/comments", Marker: "approval-test", Payload: map[string]any{"body": "waiting"}}, false, func() error {
		var decision string
		if err := s.DB.QueryRow(`SELECT decision FROM tool_approvals WHERE id=?`, approval.ID).Scan(&decision); err != nil {
			return err
		}
		if decision != "" {
			return errApprovalResolved
		}
		return nil
	})
	if !errors.Is(err, errApprovalResolved) || minted != 2 || posts != 0 {
		t.Fatalf("refresh sent stale wait: minted=%d posts=%d err=%v", minted, posts, err)
	}
}
