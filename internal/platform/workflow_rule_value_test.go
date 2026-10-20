package platform

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func valueRuleNode(t *testing.T, source, schema, condition string) WorkflowNode {
	t.Helper()
	raw := `{"schema_scope":"value","rules":[{"when":` + strconvJSON(condition) + `,"title":"匹配","target":"done"},{"else":true,"title":"其他","target":"other"}],"on_read":{"action":"route","target":"read","title":"读取失败"},"on_parse":{"action":"route","target":"parse","title":"解析失败"},"on_invalid":{"action":"route","target":"invalid","title":"校验失败"},"on_expression":{"action":"route","target":"expression","title":"计算失败"}`
	if schema != "" {
		raw += `,"schema":` + schema
	}
	raw += `}`
	var p WorkflowRules
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	return WorkflowNode{ID: "call", Name: "Execute", Kind: "execution", Execution: &WorkflowExecution{Mode: "command", Command: "make verify", Output: &ExecutionOutput{Source: source}}, RuleHandoff: &p}
}
func strconvJSON(s string) string { b, _ := json.Marshal(s); return string(b) }
func valueRuleWorkflow(n WorkflowNode) Workflow {
	w := Workflow{Nodes: []WorkflowNode{n}}
	for _, id := range []string{"done", "other", "read", "parse", "invalid", "expression"} {
		w.Nodes = append(w.Nodes, WorkflowNode{ID: id, Name: id, Kind: "end"})
	}
	return w
}
func TestRuleValueSourcesAndFailureDestinations(t *testing.T) {
	for _, c := range []struct {
		name, source, schema, condition, result, readError, want string
		code                                                     int
	}{
		{name: "exit success", source: "exit_code", condition: "$ == 0", code: 0, want: "rule_1"},
		{name: "make failure is not limited to one", source: "exit_code", condition: "$ != 0", code: 2, want: "rule_1"},
		{name: "stdout stays text", source: "stdout", condition: `$ == "0"`, result: `"0"`, want: "rule_1"},
		{name: "stderr separate text", source: "stderr", condition: `$ == "diagnostic\n"`, result: `"diagnostic\n"`, want: "rule_1"},
		{name: "file stays text", source: "file", condition: `$ == "PASS\n"`, result: `"PASS\n"`, want: "rule_1"},
		{name: "JSON scalar", source: "stdout", schema: `{"type":"integer","enum":[0,1]}`, condition: "$ == 1", result: `"1"`, want: "rule_1"},
		{name: "JSON object", source: "file", schema: `{"type":"object","properties":{"passed":{"type":"boolean"}},"required":["passed"]}`, condition: "$.passed == true", result: `"{\"passed\":true}"`, want: "rule_1"},
		{name: "JSON syntax failure", source: "stdout", schema: `{"type":"integer"}`, condition: "$ == 0", result: `"not JSON"`, want: "rule_parse"},
		{name: "schema failure", source: "stderr", schema: `{"type":"boolean"}`, condition: "$ == true", result: `"1"`, want: "rule_invalid"},
		{name: "missing file never goes else", source: "file", condition: `$ == "ok"`, readError: "file unavailable", want: "rule_read"},
		{name: "optional field unavailable", source: "file", schema: `{"type":"object","properties":{"passed":{"type":"boolean"}}}`, condition: "$.passed == true", result: `"{}"`, want: "rule_expression"},
		{name: "plain text dollar is literal", source: "stdout", condition: `$ == "$"`, result: `"$"`, want: "rule_1"},
		{name: "unmatched text", source: "stdout", condition: `$ == "PASS"`, result: `"OTHER"`, want: "rule_2"},
	} {
		t.Run(c.name, func(t *testing.T) {
			n := valueRuleNode(t, c.source, c.schema, c.condition)
			if c.source == "file" {
				n.Execution.Output.Path = "result.txt"
			}
			if err := validateWorkflowExecution(n.Execution); err != nil {
				t.Fatal(err)
			}
			if err := validateRuleHandoff(valueRuleWorkflow(n), n); err != nil {
				t.Fatal(err)
			}
			route, _, err := routeExecutionRules(n, ConnectorReceipt{ExitCode: &c.code, Result: json.RawMessage(c.result), ResultError: c.readError})
			if err != nil || route != c.want {
				t.Fatalf("got %q %v, want %s", route, err, c.want)
			}
		})
	}
}

func TestSandboxDeliveryTemplateUsesSelectedExitCode(t *testing.T) {
	raw, err := exec.Command("python3", "../../examples/platform-workflows/sandbox_delivery.py", "--repository", "example/project", "--credential-ref", "PROJECT_GITHUB_TOKEN").Output()
	if err != nil {
		t.Fatal(err)
	}
	var w Workflow
	if err = json.Unmarshal(raw, &w); err != nil {
		t.Fatal(err)
	}
	n := w.node("tests")
	if n.RuleHandoff == nil || n.Execution.Output == nil || n.Execution.Output.Source != "exit_code" {
		t.Fatal("template still uses implicit success/failure paths")
	}
	if err = validateRuleHandoff(w, n); err != nil {
		t.Fatal(err)
	}
	w.projectRuleEdges()
	for _, c := range []struct {
		code   int
		target string
	}{{0, "qa"}, {1, "development"}, {2, "development"}, {137, "development"}} {
		route, _, err := routeExecutionRules(n, ConnectorReceipt{ExitCode: &c.code})
		if err != nil || w.target(n.ID, route) != c.target {
			t.Fatalf("exit %d: %s %v", c.code, route, err)
		}
	}
}

func TestSelectedTextPreservesBytesAndRedactsInjectedSecrets(t *testing.T) {
	n := valueRuleNode(t, "stderr", "", `$ == "text"`)
	var receipt ConnectorReceipt
	setSelectedExecutionResult(n, &receipt, []byte("{broken JSON}\nprivate-token"), map[string]string{"TOKEN": "private-token"})
	if receipt.ResultError != "" || string(receipt.Result) != `"{broken JSON}\n[redacted]"` {
		t.Fatal(string(receipt.Result), receipt.ResultError)
	}
	for _, raw := range [][]byte{{0xff}, []byte(strings.Repeat("x", (1<<20)+1))} {
		receipt = ConnectorReceipt{}
		setSelectedExecutionResult(n, &receipt, raw, nil)
		route, _, err := routeExecutionRules(n, receipt)
		if err != nil || route != "rule_read" {
			t.Fatal("invalid text did not use read failure", route, err)
		}
	}
}

func TestSelectedValuePreviewUsesSameRawInputAndErrors(t *testing.T) {
	s := testStore(t)
	s.initAuth("password")
	h := NewServer(s, nil, nil, "password", "http://localhost")
	login := cookieRequest(t, h, nil, "POST", "/api/login", map[string]string{"username": "admin", "password": "password"})
	cookie := login.Result().Cookies()[0]
	n := valueRuleNode(t, "stderr", `{"type":"boolean"}`, "$ == true")
	for _, c := range []struct {
		input any
		route string
	}{{"true", "rule_1"}, {"false", "rule_2"}, {"bad json", "rule_parse"}, {"1", "rule_invalid"}, {true, "rule_read"}} {
		result := cookieRequest(t, h, cookie, "POST", "/api/workflows/rules/preview", map[string]any{"workflow": valueRuleWorkflow(n), "node_id": n.ID, "input": c.input})
		var out struct{ Route string }
		json.Unmarshal(result.Body.Bytes(), &out)
		if result.Code != 200 || out.Route != c.route {
			t.Fatalf("%v: %d %s", c.input, result.Code, result.Body.String())
		}
	}
	n.RuleHandoff.OnParse = nil
	if _, _, err := routeSelectedValue(n, "bad json"); err == nil {
		t.Fatal("default parse error silently advanced")
	}
	n.Execution.Output.Source = "exit_code"
	n.RuleHandoff.Schema = nil
	n.RuleHandoff.Rules[0].When = "$ == 0"
	for _, expr := range []string{"$.exit_code == 0", "output.exit_code == 0", "_handoff_value == 0", "$ == \"0\""} {
		n.RuleHandoff.Rules[0].When = expr
		if err := validateRuleHandoff(valueRuleWorkflow(n), n); err == nil {
			t.Fatal("accepted hidden wrapper or type coercion", expr)
		}
	}
}
