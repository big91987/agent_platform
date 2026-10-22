package platform

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// Exercise the shipped template through the real handoff consumer, rather than
// checking for a route string in its JSON.
func TestProductTestBlockedPlanCanDeliverReport(t *testing.T) {
	data, err := os.ReadFile("../../examples/platform-workflows/product-e2e.json")
	if err != nil {
		t.Fatal(err)
	}
	var template Workflow
	if err = json.Unmarshal(data, &template); err != nil {
		t.Fatal(err)
	}
	s := testStore(t)
	if err = s.initAuth("password"); err != nil {
		t.Fatal(err)
	}
	c, err := s.userCaller("admin")
	if err != nil {
		t.Fatal(err)
	}
	a := testAgent(t, s)
	w := Workflow{Name: "blocked plan report", Enabled: true, ContextVersion: 2, Entry: "e2e_plan_review", MaxSteps: 10}
	for _, id := range []string{"e2e_plan_review", "e2e_plan", "e2e_execute", "e2e_report"} {
		w.Nodes = append(w.Nodes, WorkflowNode{ID: id, Name: id, Kind: "agent", AgentID: a.ID})
	}
	w.Nodes = append(w.Nodes, WorkflowNode{ID: "done", Name: "done", Kind: "end"})
	w.Edges = append(w.Edges,
		WorkflowEdge{Source: "e2e_plan", Target: "e2e_plan_review", Route: "next", Mode: "handoff"},
		WorkflowEdge{Source: "e2e_execute", Target: "e2e_report", Route: "next", Mode: "handoff"},
		WorkflowEdge{Source: "e2e_report", Target: "done", Route: "next", Mode: "handoff"},
	)
	for _, edge := range template.Edges {
		if edge.Source == w.Entry {
			w.Edges = append(w.Edges, edge)
		}
	}
	w.Nodes[0].HandoffParams = true
	w.Nodes[0].HandoffSchema = json.RawMessage(`{"type":"object"}`)
	w, err = s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "deliver the blocked plan with unexecuted cases", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	in := HandoffInput{Target: "e2e_report", Message: "Blocked: required authorization is unavailable", Params: map[string]any{"product_result": "Not Run", "plan_verdict": "Blocked"}}
	if err = s.handoffWorkflowNode(r.ID, 1, *conv.Snapshot.Env[workflowTokenEnv], in); err != nil {
		t.Fatalf("blocked plan cannot reach the existing report agent: %v", err)
	}
	engine.Tick(context.Background())
	before, _ := s.WorkflowRun(c, r.ID)
	if before.Seq != 1 {
		t.Fatal("report started before the native review turn completed")
	}
	if err = s.Complete(conv.ID, msg.ID, "completed", "Blocked"); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	after, err := s.WorkflowRun(c, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Seq != 2 || after.Steps[1].NodeID != "e2e_report" || after.Steps[0].Result.Params["product_result"] != "Not Run" {
		t.Fatalf("blocked result did not reach reporting intact: %+v", after)
	}
}
