package platform

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func ruleFixture() WorkflowNode {
	return WorkflowNode{ID: "call", Name: "Evaluate result", Kind: "execution", Execution: &WorkflowExecution{Mode: "mcp"}, RuleHandoff: &WorkflowRules{Schema: json.RawMessage(`{"type":"object","properties":{"passed":{"type":"boolean"},"score":{"type":"number"},"status":{"type":"integer","enum":[0,1]}},"required":["passed","score"]}`), Rules: []WorkflowRule{{Title: "继续", When: "output.result.passed == true && output.result.score >= 80", Target: "done"}, {Title: "其他", Else: true, Target: "review"}}, OnInvalid: WorkflowRuleFailure{Action: "route", Target: "review", Title: "格式错误"}, OnExpression: WorkflowRuleFailure{Action: "stop"}}}
}
func TestRuleHandoffConditionsAndErrors(t *testing.T) {
	n := ruleFixture()
	for _, c := range []struct{ data, route string }{{`{"passed":true,"score":80}`, "rule_1"}, {`{"passed":true,"score":79}`, "rule_2"}, {`{"passed":false,"score":90}`, "rule_2"}, {`{"passed":1,"score":90}`, "rule_invalid"}, {`{"score":90}`, "rule_invalid"}, {`{"passed":true,"score":90,"status":2}`, "rule_invalid"}} {
		r, _, err := routeExecutionRules(n, ConnectorReceipt{MCP: &mcp.CallToolResult{}, Result: json.RawMessage(c.data)})
		if err != nil || r != c.route {
			t.Fatalf("%s: %s %v", c.data, r, err)
		}
	}
	n.RuleHandoff.Rules = n.RuleHandoff.Rules[:1]
	n.RuleHandoff.Rules[0].When = "output.result.status == 1"
	if _, _, err := routeExecutionRules(n, ConnectorReceipt{MCP: &mcp.CallToolResult{}, Result: json.RawMessage(`{"passed":true,"score":90}`)}); err == nil {
		t.Fatal("missing optional field did not fail")
	}
	n.RuleHandoff.OnExpression = WorkflowRuleFailure{Action: "route", Target: "review", Title: "条件错误"}
	r, _, err := routeExecutionRules(n, ConnectorReceipt{MCP: &mcp.CallToolResult{}, Result: json.RawMessage(`{"passed":true,"score":90}`)})
	if err != nil || r != "rule_expression" {
		t.Fatal(r, err)
	}
	n.RuleHandoff.Rules[0].When = "output.result.score > 100"
	if _, _, err = routeExecutionRules(n, ConnectorReceipt{MCP: &mcp.CallToolResult{}, Result: json.RawMessage(`{"passed":true,"score":90}`)}); err == nil {
		t.Fatal("missing else silently passed")
	}
}
func TestRuleHandoffRejectsUnsafeAndWrongTypes(t *testing.T) {
	n := ruleFixture()
	types, _ := ruleTypes(n.RuleHandoff, "mcp")
	for _, expr := range []string{"output.result.passed == 1", "output.result.missing == true", "system(\"id\")", "output.result.score + 1 > 80", "output.result", "output.result.passed > false"} {
		if _, err := parseRule(expr, types); err == nil {
			t.Fatal("accepted", expr)
		}
	}
	for _, expr := range []string{"output.result.status == 0", "output.result.status == 1", "!output.result.passed", "output.is_error == false"} {
		if _, err := parseRule(expr, types); err != nil {
			t.Fatal(expr, err)
		}
	}
}
func TestRuleHandoffMCPRealRunFrozenAndDeduplicated(t *testing.T) {
	s := testStore(t)
	var calls atomic.Int32
	c, w, dir, _ := executionFixture(t, s, func(context.Context, string) (*mcp.CallToolResult, any, error) {
		calls.Add(1)
		return nil, map[string]any{"passed": true, "score": 90}, nil
	})
	n := ruleFixture()
	w.Nodes[0].RuleHandoff = n.RuleHandoff
	w.Nodes[0].Execution.Output = &ExecutionOutput{Source: "tool"}
	w.Nodes = append(w.Nodes, WorkflowNode{ID: "review", Name: "Review", Kind: "end"})
	var err error
	w, err = s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	in := WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "rule check", RequestID: "rules-once"}
	r, err := s.StartWorkflow(c, in)
	if err != nil {
		t.Fatal(err)
	}
	w.Nodes[0].RuleHandoff.Rules[0].When = "output.result.score > 100"
	if _, err = s.SaveWorkflow(c, w); err != nil {
		t.Fatal(err)
	}
	r = waitConnectorRun(t, NewWorkflowEngine(s, nil, ""), r.ID, "completed")
	if r.Steps[0].Result.Route != "rule_1" || r.Steps[1].NodeID != "done" || r.Steps[0].Result.Inputs["output"] == nil || r.Steps[0].Result.Inputs["tool_result"] == nil {
		t.Fatalf("not frozen: %+v", r.Steps)
	}
	duplicate, err := s.StartWorkflow(c, in)
	if err != nil || duplicate.ID != r.ID || calls.Load() != 1 {
		t.Fatal("replayed", err, calls.Load())
	}
}
func TestExecutionStructuredResultDoesNotPersistSecrets(t *testing.T) {
	var r ConnectorReceipt
	setExecutionResult(&r, []byte(`{"token":"secret-value","score":1}`), map[string]string{"KEY": "secret-value"})
	if string(r.Result) != `{"score":1,"token":"[redacted]"}` {
		t.Fatal(string(r.Result))
	}
	setExecutionResult(&r, []byte(`{"n":9007199254740993}`), nil)
	if r.ResultError == "" {
		t.Fatal("lossy number accepted")
	}
}

func TestRuleHandoffPreviewMatchesRuntimeAndRequiresAdmin(t *testing.T) {
	s := testStore(t)
	s.initAuth("password")
	h := NewServer(s, nil, nil, "password", "http://localhost")
	login := cookieRequest(t, h, nil, "POST", "/api/login", map[string]string{"username": "admin", "password": "password"})
	if login.Code != 200 {
		t.Fatal(login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	n := ruleFixture()
	w := Workflow{Nodes: []WorkflowNode{n, {ID: "done", Kind: "end"}, {ID: "review", Kind: "end"}}}
	body := map[string]any{"workflow": w, "node_id": "call", "output": map[string]any{"is_error": false, "result": map[string]any{"passed": true, "score": 86}}}
	unauth := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/workflows/rules/preview", nil)
	h.ServeHTTP(unauth, req)
	if unauth.Code == 200 {
		t.Fatal("unauthenticated preview")
	}
	check := func(want string) {
		t.Helper()
		r := cookieRequest(t, h, cookie, "POST", "/api/workflows/rules/preview", body)
		var out struct{ Route string }
		json.Unmarshal(r.Body.Bytes(), &out)
		if r.Code != 200 || out.Route != want {
			t.Fatal(r.Code, r.Body.String())
		}
	}
	check("rule_1")
	body["output"] = map[string]any{"is_error": false, "result": map[string]any{"passed": 1, "score": 86}}
	check("rule_invalid")
	n.RuleHandoff.Rules[0].When = "output.result.passed == 1"
	r := cookieRequest(t, h, cookie, "POST", "/api/workflows/rules/preview", body)
	if r.Code == 200 {
		t.Fatal("wrong type accepted")
	}
}

func TestRuleHandoffDoesNotTreatDottedPropertyAsNestedField(t *testing.T) {
	n := ruleFixture()
	n.RuleHandoff.Schema = json.RawMessage(`{"type":"object","properties":{"passed.value":{"type":"boolean"}}}`)
	types, err := ruleTypes(n.RuleHandoff, "mcp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = parseRule("output.result.passed.value == true", types); err == nil {
		t.Fatal("literal dotted property granted access to undeclared nested field")
	}
}
