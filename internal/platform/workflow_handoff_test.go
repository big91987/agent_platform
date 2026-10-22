package platform

import (
	"context"
	"testing"
)

func TestHandoffRejectsFixedAgentEdgeConfiguration(t *testing.T) {
	s := testStore(t)
	_, r := agentRun(t, s)
	w := r.Definition
	w.Edges[0].Mode = "automatic"
	if err := validateWorkflow(w); err == nil {
		t.Fatal("obsolete fixed Agent edge accepted")
	}
	w.Edges[0].Mode = "handoff"
	w.Nodes[0].ExitMode = "complete"
	if err := validateWorkflow(w); err == nil {
		t.Fatal("obsolete exit mode accepted")
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
	if e = s.handoffWorkflowNode(r.ID, 1, token, HandoffInput{Target: "missing", Message: "test"}); e == nil {
		t.Fatal("invalid target accepted")
	}
	in := HandoffInput{Target: "revise", Message: "revise evidence", Context: map[string]any{"problem": "missing evidence"}}
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
