package platform

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestWorkflowEntryParametersAndRequestLookup(t *testing.T) {
	s := testStore(t)
	c, w, _ := runFixture(t, s)
	// Decode the public wire contract so missing fields fail behaviorally.
	var in WorkflowStart
	raw, _ := json.Marshal(map[string]any{"workflow_id": w.ID, "input": "fix issue", "workspace_path": t.TempDir(), "request_id": "github:issue:123", "parameters": map[string]string{"issue_number": "42"}})
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(c, in)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(r)
	var stored map[string]any
	json.Unmarshal(raw, &stored)
	if stored["parameters"] == nil {
		t.Fatal("Run lost its external input parameters")
	}
	found, err := s.WorkflowRunByRequest(c, "github:issue:123")
	if err != nil || found.ID != r.ID {
		t.Fatal(found, err)
	}
	_, err = s.WorkflowRunByRequest(Caller{UserID: "someone-else", Admin: true}, "github:issue:123")
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("request keys must be caller-scoped", err)
	}

}

func TestWorkflowMessageAtomicRoutingAndReplay(t *testing.T) {
	s := testStore(t)
	c, r := agentRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	h := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, h)
	body := map[string]any{"message": "补充要求", "request_id": "github:comment:5"}
	path := "/api/workflow-runs/" + r.ID + "/messages"
	resp := workflowRequest(t, h, cookie, "POST", path, body)
	if resp.Code != 202 {
		t.Fatal(resp.Code, resp.Body.String())
	}
	var receipt Receipt
	json.Unmarshal(resp.Body.Bytes(), &receipt)
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	s.Complete(conv.ID, msg.ID, "completed", "")
	conv, msg, err = s.Claim()
	if err != nil || msg.Content != "补充要求" {
		t.Fatal(msg, err)
	}
	token := *conv.Snapshot.Env[workflowTokenEnv]
	if err = s.completeWorkflowNode(Caller{}, r.ID, 1, token, NodeResult{Route: "approved", Summary: "done"}); err != nil {
		t.Fatal(err)
	}
	s.Complete(conv.ID, msg.ID, "completed", "")
	engine.Tick(context.Background())
	fresh, _ := s.WorkflowRun(c, r.ID)
	if fresh.Status != "completed" {
		t.Fatal(fresh.Status)
	}
	resp = workflowRequest(t, h, cookie, "POST", path, body)
	var again Receipt
	json.Unmarshal(resp.Body.Bytes(), &again)
	if resp.Code != 202 || !again.Duplicate || again.MessageID != receipt.MessageID {
		t.Fatal(resp.Code, resp.Body.String())
	}
	body["request_id"] = "github:comment:6"
	resp = workflowRequest(t, h, cookie, "POST", path, body)
	if resp.Code != 409 {
		t.Fatal("completed run accepted new input", resp.Code)
	}
}

func TestWorkflowMessageRejectsOtherOwnerAndStaleSequence(t *testing.T) {
	s := testStore(t)
	_, r := agentRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	a := testAgent(t, s)
	_, token := testUser(t, s, &a, "outsider")
	h := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, h)
	path := "/api/workflow-runs/" + r.ID + "/messages"
	resp := requestJSON(t, h, "POST", path, token, map[string]any{"message": "inject", "request_id": "event1"})
	if resp.Code != 403 {
		t.Fatal(resp.Code, resp.Body.String())
	}
	resp = workflowRequest(t, h, cookie, "POST", path, map[string]any{"message": "stale", "request_id": "event2", "seq": 99})
	if resp.Code != 409 {
		t.Fatal(resp.Code, resp.Body.String())
	}
}
