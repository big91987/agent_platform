package platform

import (
	"encoding/json"
	"testing"
)

func TestHandoffSchemaSelectsParametersByTarget(t *testing.T) {
	var w Workflow
	if err := json.Unmarshal([]byte(`{"nodes":[{"id":"author","kind":"agent","handoff_parameters":{"execute":{"enabled":true,"schema":{"type":"object","properties":{"commit":{"type":"string"}},"required":["commit"],"additionalProperties":false},"instructions":"commit填写实际版本"},"review":{"enabled":true,"schema":{"type":"object","properties":{"ready":{"type":"boolean"}},"required":["ready"],"additionalProperties":false}}}},{"id":"execute","kind":"execution"},{"id":"review","kind":"agent"},{"id":"qa","kind":"agent"}],"edges":[{"source":"author","target":"execute","route":"execute","mode":"handoff"},{"source":"author","target":"review","route":"review","mode":"handoff"},{"source":"author","target":"qa","route":"qa","mode":"handoff"}]}`), &w); err != nil {
		t.Fatal(err)
	}
	schema, err := handoffToolSchema(w, w.Nodes[0])
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		input string
		valid bool
	}{
		{`{"target":"execute","message":"检查版本","params":{"commit":"abc"}}`, true},
		{`{"target":"review","message":"独立复核","params":{"ready":true}}`, true},
		{`{"target":"qa","message":"读取报告","artifacts":["report.md"]}`, true},
		{`{"target":"execute","message":"缺参数"}`, false},
		{`{"target":"execute","message":"错分支格式","params":{"ready":true}}`, false},
		{`{"target":"review","message":"错分支格式","params":{"commit":"abc"}}`, false},
		{`{"target":"qa","message":"未启用","params":{}}`, false},
		{`{"target":"unknown","message":"非法目标"}`, false},
	} {
		var input any
		json.Unmarshal([]byte(tc.input), &input)
		err := resolved.Validate(input)
		if (err == nil) != tc.valid {
			t.Fatalf("%s: valid=%v error=%v", tc.input, tc.valid, err)
		}
	}
}

func TestRetiredNodeParametersRemainReadableButCannotExecute(t *testing.T) {
	var w Workflow
	if err := json.Unmarshal([]byte(`{"nodes":[{"id":"author","kind":"agent","handoff_params":false}],"edges":[]}`), &w); err != nil {
		t.Fatal(err)
	}
	if err := validateWorkflowHandoffs(w); err == nil {
		t.Fatal("retired node parameters silently accepted")
	}
	w.Nodes[0].RetiredHandoffParams = nil
	w.Nodes[0].HandoffParameters = map[string]HandoffParameters{"missing": {Enabled: false}}
	if err := validateWorkflowHandoffs(w); err == nil {
		t.Fatal("orphan target configuration accepted")
	}
}

func TestEnabledTargetRequiresPresentParamsEvenForEmptyObjectSchema(t *testing.T) {
	n := WorkflowNode{HandoffParameters: map[string]HandoffParameters{"next": {Enabled: true, Schema: json.RawMessage(`{"type":"object"}`)}}}
	if err := validateCompletionResult(n, "next", NodeResult{}); err == nil {
		t.Fatal("missing params accepted")
	}
	if err := validateCompletionResult(n, "next", NodeResult{Params: map[string]any{}}); err != nil {
		t.Fatal("explicit empty object rejected", err)
	}
}
