package platform

import (
	"context"
	"testing"
)

func TestHandoffDistinguishesAgentChoiceFromFixedCompletion(t *testing.T) {
	s := testStore(t)
	c, w, _ := runFixture(t, s)
	a := testAgent(t, s)
	w.Nodes[0].Kind = "agent"
	w.Nodes[0].AgentID = a.ID
	for i := range w.Edges {
		if w.Edges[i].Source == w.Entry {
			w.Edges[i].Mode = "handoff"
		}
	}
	w.Edges = append(w.Edges, WorkflowEdge{Source: w.Entry, Route: "finished", Target: "done", Mode: "automatic"})
	w, e := s.SaveWorkflow(c, w)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "inspect", WorkspacePath: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	conv, msg, e := s.Claim()
	if e != nil {
		t.Fatal(e)
	}
	token := *conv.Snapshot.Env[workflowTokenEnv]
	if e = s.handoffWorkflowNode(r.ID, 1, token, HandoffInput{Target: "done", Route: "finished", Summary: "wrong tool"}); e == nil {
		t.Fatal("Agent selected a fixed edge")
	}
	if e = s.completeWorkflowNode(Caller{}, r.ID, 1, token, NodeResult{Route: "approved"}); e == nil {
		t.Fatal("new autonomous edge accepted through fixed completion")
	}
	if e = s.completeWorkflowNode(Caller{}, r.ID, 1, token, NodeResult{Summary: "complete"}); e != nil {
		t.Fatal(e)
	}
	engine.Tick(context.Background())
	before, _ := s.WorkflowRun(c, r.ID)
	if before.Seq != 1 {
		t.Fatal("advanced before turn ended")
	}
	s.Complete(conv.ID, msg.ID, "completed", "")
	engine.Tick(context.Background())
	after, _ := s.WorkflowRun(c, r.ID)
	if after.Status != "completed" {
		t.Fatal(after)
	}
}

func TestHandoffRejectsInvalidTargetsAndRetainsStructuredInputs(t *testing.T) {
	s := testStore(t)
	c, r := agentRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	conv, _, e := s.Claim()
	if e != nil {
		t.Fatal(e)
	}
	token := *conv.Snapshot.Env[workflowTokenEnv]
	if e = s.handoffWorkflowNode(r.ID, 1, token, HandoffInput{Target: "missing", Summary: "test"}); e == nil {
		t.Fatal("invalid target accepted")
	}
	in := HandoffInput{Target: "revise", Summary: "revise evidence", Inputs: map[string]string{"problem": "missing evidence"}}
	if e = s.handoffWorkflowNode(r.ID, 1, token, in); e != nil {
		t.Fatal(e)
	}
	if e = s.handoffWorkflowNode(r.ID, 1, token, in); e != nil {
		t.Fatal("duplicate", e)
	}
	after, _ := s.WorkflowRun(c, r.ID)
	if after.Steps[0].Result.Inputs["problem"] != "missing evidence" {
		t.Fatal(after)
	}
	in.Target = "done"
	if e = s.handoffWorkflowNode(r.ID, 1, token, in); e == nil {
		t.Fatal("conflicting handoff accepted")
	}
}
