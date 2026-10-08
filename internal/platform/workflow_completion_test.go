package platform

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func configuredCompletion(t *testing.T, w Workflow, mode, schema string) Workflow {
	t.Helper()
	raw, _ := json.Marshal(w)
	var v map[string]any
	json.Unmarshal(raw, &v)
	n := v["nodes"].([]any)[0].(map[string]any)
	n["exit_mode"] = mode
	n["completion_instructions"] = "从真实验证结果生成 passed、count 和 files，不推测。"
	if schema != "" {
		var x any
		if err := json.Unmarshal([]byte(schema), &x); err != nil {
			t.Fatal(err)
		}
		n["completion_schema"] = x
	}
	raw, _ = json.Marshal(v)
	if err := json.Unmarshal(raw, &w); err != nil {
		t.Fatal(err)
	}
	return w
}

const completionTestSchema = `{"type":"object","properties":{"passed":{"type":"boolean"},"count":{"type":"integer","minimum":1},"files":{"type":"array","items":{"type":"string"}}},"required":["passed","count","files"],"additionalProperties":false}`

func TestConfiguredCompletionRejectsInvalidResultAndPreservesTypes(t *testing.T) {
	s := testStore(t)
	c, r := inputProtocolRun(t, s, 2)
	w := r.Definition
	e := NewWorkflowEngine(s, nil, "http://localhost")
	if err := e.Stop(c, r.ID, r.Seq); err != nil {
		t.Fatal(err)
	}
	e.Tick(context.Background())
	w.Edges = []WorkflowEdge{{Source: w.Entry, Target: "done", Route: "next", Mode: "automatic"}}
	w.Nodes = []WorkflowNode{w.Nodes[0], w.node("done")}
	w = configuredCompletion(t, w, "complete", completionTestSchema)
	var err error
	w, err = s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "validate", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	e.Tick(context.Background())
	conv, _, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	token := *conv.Snapshot.Env[workflowTokenEnv]
	if err = s.completeWorkflowNode(Caller{}, r.ID, 1, token, NodeResult{Summary: "missing required output"}); err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("missing inputs should fail schema validation: %v", err)
	}
	fresh, _ := s.WorkflowRun(c, r.ID)
	if fresh.Steps[0].Result != nil {
		t.Fatal("invalid result was persisted")
	}
	var in NodeResult
	if err = json.Unmarshal([]byte(`{"summary":"verified","inputs":{"passed":true,"count":2,"files":["a.txt","b.txt"]}}`), &in); err != nil {
		t.Fatal(err)
	}
	if err = s.completeWorkflowNode(Caller{}, r.ID, 1, token, in); err != nil {
		t.Fatal(err)
	}
	fresh, _ = s.WorkflowRun(c, r.ID)
	b, _ := json.Marshal(fresh.Steps[0].Result)
	if !strings.Contains(string(b), `"passed":true`) || !strings.Contains(string(b), `"count":2`) {
		t.Fatalf("typed inputs lost: %s", b)
	}
}

func TestCompletionConfigurationRejectsModeAndSchemaErrors(t *testing.T) {
	s := testStore(t)
	_, r := inputProtocolRun(t, s, 2)
	w := r.Definition
	for _, test := range []struct{ mode, schema string }{
		{"complete", completionTestSchema}, // existing edges are handoff
		{"invalid", ""},
		{"handoff", `{"type":"object","properties":{"n":{"type":"imaginary"}}}`},
		{"handoff", `{"type":"object","$ref":"https://example.test/schema"}`},
		{"handoff", `{"type":"object","propertiez":{}}`},
	} {
		if err := validateWorkflow(configuredCompletion(t, w, test.mode, test.schema)); err == nil {
			t.Errorf("accepted invalid config: %s %s", test.mode, test.schema)
		}
	}
}

func TestCompletionMCPFrozenContractAndCorrection(t *testing.T) {
	s := testStore(t)
	c, r := inputProtocolRun(t, s, 2)
	e := NewWorkflowEngine(s, nil, "http://localhost")
	if err := e.Stop(c, r.ID, r.Seq); err != nil {
		t.Fatal(err)
	}
	e.Tick(context.Background())
	w := r.Definition
	w.Nodes = []WorkflowNode{w.Nodes[0], w.node("done")}
	w.Edges = []WorkflowEdge{{Source: w.Entry, Target: "done", Route: "next", Mode: "automatic"}}
	w = configuredCompletion(t, w, "complete", completionTestSchema)
	w, err := s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "verify", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	h := NewServer(s, nil, nil, "password", "http://localhost")
	remote := httptest.NewServer(h)
	defer remote.Close()
	h.workflows.base = remote.URL
	h.workflows.Tick(context.Background())
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg.Content, "从真实验证结果") || !strings.Contains(msg.Content, `"passed"`) {
		t.Fatal("input did not include configured guidance")
	}
	// Editing the saved workflow must never change a running Agent's contract.
	w.Nodes[0].CompletionSchema = json.RawMessage(`{"type":"object","required":["new_field"]}`)
	if _, err = s.SaveWorkflow(c, w); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "contract-proof", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: remote.URL + "/api/workflow-node/mcp", HTTPClient: &http.Client{Transport: bearerTransport{http.DefaultTransport, *conv.Snapshot.Env[workflowTokenEnv]}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range list.Tools {
		if tool.Name == "handoff" {
			t.Fatal("fixed node exposed handoff")
		}
		if tool.Name == "complete_node" {
			found = true
			b, _ := json.Marshal(tool.InputSchema)
			if !strings.Contains(string(b), `"passed"`) || strings.Contains(string(b), `"route"`) {
				t.Fatalf("wrong tool schema: %s", b)
			}
		}
	}
	if !found {
		t.Fatal("missing completion tool")
	}
	for _, args := range []string{`{"summary":"bad","inputs":{"passed":true,"count":9007199254740993,"files":[]}}`, `{"summary":"bad"}`, `{"summary":"bad","inputs":{"passed":"true","count":2,"files":[]}}`, `{"summary":"bad","inputs":{"passed":true,"count":2,"files":[],"extra":1}}`} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "complete_node", Arguments: json.RawMessage(args)})
		if err == nil && !result.IsError {
			t.Fatal("accepted invalid arguments", args)
		}
		fresh, _ := s.WorkflowRun(c, r.ID)
		if fresh.Seq != 1 || fresh.Steps[0].Result != nil {
			t.Fatal("invalid completion changed state")
		}
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "complete_node", Arguments: json.RawMessage(`{"summary":"verified","inputs":{"passed":true,"count":2,"files":["a.txt"]}}`)})
	if err != nil || result.IsError {
		t.Fatal(result, err)
	}
	h.workflows.Tick(ctx)
	fresh, _ := s.WorkflowRun(c, r.ID)
	if fresh.Seq != 1 {
		t.Fatal("advanced before the native turn ended")
	}
	// The same persisted result is delivered to command stdin without stringifying values.
	req, err := prepareConnectorRequest(Connector{Kind: "command", WorkspaceRoot: r.WorkspacePath}, fresh, WorkflowStep{Seq: 2, NodeID: "done"})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Previous []WorkflowStep `json:"previous_results"`
	}
	if err = json.Unmarshal([]byte(req.Stdin), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Previous) != 1 || payload.Previous[0].Result.Inputs["passed"] != true || payload.Previous[0].Result.Inputs["count"] != float64(2) {
		t.Fatal(req.Stdin)
	}
	if err = s.Complete(conv.ID, msg.ID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	h.workflows.Tick(ctx)
	fresh, _ = s.WorkflowRun(c, r.ID)
	if fresh.Status != "completed" {
		t.Fatal(fresh.Status)
	}
}

func TestCompletionEmptyInputsAndUnrepresentableNumbers(t *testing.T) {
	raw := `{"summary":"ok","inputs":{}}`
	var result NodeResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal(encoded, &got)
	if _, ok := got["inputs"].(map[string]any); !ok {
		t.Fatalf("empty object lost: %s", encoded)
	}
	for _, raw := range []string{`{"summary":"bad","inputs":{"id":9007199254740993}}`, `{"summary":"bad","inputs":{"nested":[{"value":1.0000000000000001}]}}`} {
		if err := json.Unmarshal([]byte(raw), &result); err == nil {
			t.Fatalf("accepted lossy number: %s", raw)
		}
	}
	n := WorkflowNode{CompletionSchema: json.RawMessage(`{"type":"object","properties":{"id":{"const":9007199254740993}}}`)}
	if _, err := completionInputsSchema(n); err == nil {
		t.Fatal("accepted lossy schema constraint")
	}
}
