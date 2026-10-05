package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHookProjectionUsesFinalReplyOnceWithoutAdvancingNode(t *testing.T) {
	s := testStore(t)
	c, w, _ := runFixture(t, s)
	a := testAgent(t, s)
	w.Nodes[0].Kind = "agent"
	w.Nodes[0].AgentID = a.ID
	dir := t.TempDir()
	connector, err := s.SaveConnector(c, Connector{Name: "notify", Kind: "github.issue_comment", Repository: "demo/repo", TokenEnv: "HOOK_TEST_TOKEN", WorkspaceRoot: dir, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	w.Hooks = []WorkflowHook{{Event: "node.started", ConnectorID: connector.ID, Input: ConnectorInput{IssueNumber: 1}, Body: "{{node_name}} {{conversation_url}}"}, {Event: "agent.reply.completed", ConnectorID: connector.ID, Input: ConnectorInput{IssueNumber: 1}, Body: "{{text}}\n{{conversation_url}}"}}
	w.Hooks[0], w.Hooks[1] = w.Hooks[1], w.Hooks[0] // Configuration order must not decide event chronology.
	w, err = s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "request", WorkspacePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "https://platform.example")
	engine.Tick(context.Background())
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"type": "item.completed", "item": map[string]any{"id": "reply", "type": "agent_message", "text": "请选择方向 {{run_id}}"}})
	if err = s.RecordEvent(conv.ID, msg.ID, raw); err != nil {
		t.Fatal(err)
	}
	if err = engine.collectWorkflowHooks(); err != nil {
		t.Fatal(err)
	}
	before, _ := s.WorkflowHookDeliveries(c, r.ID)
	if len(before) != 1 {
		t.Fatalf("unfinished reply was emitted: %+v", before)
	}
	if err = s.Complete(conv.ID, msg.ID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	// Rebuild projection from completed facts, as when collection was delayed.
	if _, err = s.DB.Exec(`DELETE FROM workflow_hook_deliveries WHERE run_id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err = engine.collectWorkflowHooks(); err != nil {
			t.Fatal(err)
		}
	}
	deliveries, err := s.WorkflowHookDeliveries(c, r.ID)
	if err != nil || len(deliveries) != 2 {
		t.Fatal(deliveries, err)
	}
	if deliveries[0].Event != "node.started" || deliveries[1].Event != "agent.reply.completed" {
		t.Fatal("events reordered by hook configuration", deliveries)
	}
	var fieldsRaw string
	if err = s.DB.QueryRow(`SELECT fields FROM workflow_hook_deliveries WHERE run_id=? AND event='agent.reply.completed'`, r.ID).Scan(&fieldsRaw); err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{}
	json.Unmarshal([]byte(fieldsRaw), &fields)
	if got := renderHookBody("{{text}}", fields); got != "请选择方向 {{run_id}}" {
		t.Fatal("message interpreted as a template", got)
	}
	after, _ := s.WorkflowRun(c, r.ID)
	if after.Seq != 1 || after.Steps[0].Result != nil {
		t.Fatal("notification advanced run", after)
	}
	if _, err = s.WorkflowHookDeliveries(Caller{UserID: "other"}, r.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	// A missing credential fails the notification only, not the Agent execution.
	if err = engine.dispatchWorkflowHooks(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, _ = s.WorkflowRun(c, r.ID)
	if after.Status == "failed" {
		t.Fatal("notification failure stopped Agent")
	}
	deliveries, _ = s.WorkflowHookDeliveries(c, r.ID)
	if deliveries[0].Status != "failed" {
		t.Fatal(deliveries)
	}
}

func TestHookRejectsUnknownFieldsAndSnapshotsActionPermission(t *testing.T) {
	raw, _ := json.Marshal(workflowFixture())
	var w Workflow
	json.Unmarshal(raw, &w)
	w.Hooks = []WorkflowHook{{Event: "agent.reply.completed", ConnectorID: "action", Body: "{{unknown}}"}}
	if err := validateWorkflow(w); err == nil {
		t.Fatal("unknown field accepted")
	}
	w.Hooks[0].Body = "{{text}}"
	w.Hooks[0].Event = "handoff.before"
	if err := validateWorkflow(w); err == nil {
		t.Fatal("unimplemented blocking hook accepted")
	}
}

// A failed read before POST is safe to retry; a lost POST response is not.
func TestHookRetryDistinguishesPreflightFromWriteFailure(t *testing.T) {
	for _, phase := range []string{"lookup", "write"} {
		t.Run(phase, func(t *testing.T) {
			s := testStore(t)
			c, w, _ := runFixture(t, s)
			a := testAgent(t, s)
			w.Nodes[0].Kind, w.Nodes[0].AgentID = "agent", a.ID
			t.Setenv("HOOK_TEST_TOKEN", "test-token")
			dir := t.TempDir()
			v, err := s.SaveConnector(c, Connector{Name: "notify", Kind: "github.issue_comment", Repository: "demo/repo", TokenEnv: "HOOK_TEST_TOKEN", WorkspaceRoot: dir, Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			w.Hooks = []WorkflowHook{{Event: "node.started", ConnectorID: v.ID, Input: ConnectorInput{IssueNumber: 1}, Body: "{{node_name}}"}}
			w, err = s.SaveWorkflow(c, w)
			if err != nil {
				t.Fatal(err)
			}
			r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "request", WorkspacePath: dir})
			if err != nil {
				t.Fatal(err)
			}
			original := githubClient
			defer func() { githubClient = original }()
			reads, posts := 0, 0
			var saved map[string]any
			githubClient = &http.Client{Transport: connectorTransport(func(req *http.Request) (*http.Response, error) {
				body := `[]`
				if req.Method == "GET" {
					reads++
					if phase == "lookup" && reads == 1 {
						return nil, errors.New("lookup timeout before POST")
					}
					if saved != nil {
						raw, _ := json.Marshal([]map[string]any{saved})
						body = string(raw)
					}
				} else {
					posts++
					var payload map[string]any
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						return nil, err
					}
					saved = map[string]any{"html_url": "https://github.com/demo/repo/issues/1#issuecomment-2", "body": payload["body"]}
					if phase == "write" {
						return nil, errors.New("POST response lost")
					}
					raw, _ := json.Marshal(saved)
					body = string(raw)
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			h := NewServer(s, nil, nil, "password", "http://localhost")
			h.workflows.Tick(context.Background())
			if err = h.workflows.collectWorkflowHooks(); err != nil {
				t.Fatal(err)
			}
			if err = h.workflows.dispatchWorkflowHooks(context.Background()); err != nil {
				t.Fatal(err)
			}
			h.workflows.connectorWG.Wait()
			deliveries, err := s.WorkflowHookDeliveries(c, r.ID)
			if err != nil || len(deliveries) != 1 {
				t.Fatal(deliveries, err)
			}
			wantStatus := "failed"
			if phase == "write" {
				wantStatus = "unknown"
			}
			if deliveries[0].Status != wantStatus {
				t.Fatalf("%s failure status=%s, want %s", phase, deliveries[0].Status, wantStatus)
			}
			cookie := workflowAdmin(t, h)
			resp := workflowRequest(t, h, cookie, "POST", "/api/workflow-runs/"+r.ID+"/notifications/"+deliveries[0].ID+"/retry", nil)
			if resp.Code != http.StatusAccepted {
				t.Fatal(resp.Code, resp.Body.String())
			}
			if err = h.workflows.dispatchWorkflowHooks(context.Background()); err != nil {
				t.Fatal(err)
			}
			h.workflows.connectorWG.Wait()
			deliveries, err = s.WorkflowHookDeliveries(c, r.ID)
			if err != nil || deliveries[0].Status != "succeeded" || posts != 1 {
				t.Fatalf("retry duplicated or lost notification: %#v posts=%d err=%v", deliveries, posts, err)
			}
			after, _ := s.WorkflowRun(c, r.ID)
			if after.Seq != 1 {
				t.Fatal("notification retry advanced workflow", after.Seq)
			}
		})
	}
}
