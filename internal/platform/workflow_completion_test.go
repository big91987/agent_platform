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
	if mode != "" {
		n["exit_mode"] = mode
	}
	if schema != "" {
		var x any
		if err := json.Unmarshal([]byte(schema), &x); err != nil {
			t.Fatal(err)
		}
		targets := map[string]any{}
		for _, edge := range w.Edges {
			if edge.Source == w.Nodes[0].ID {
				targets[edge.Target] = map[string]any{"enabled": true, "schema": x, "instructions": "从真实验证结果生成 passed、count 和 files，不推测。"}
			}
		}
		n["handoff_parameters"] = targets
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
	w.Edges = []WorkflowEdge{{Source: w.Entry, Target: "done", Route: "next", Mode: "handoff"}}
	w.Nodes = []WorkflowNode{w.Nodes[0], w.node("done")}
	w = configuredCompletion(t, w, "", completionTestSchema)
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
	if err = s.testHandoffResult(r.ID, 1, token, NodeResult{Summary: "missing required output"}); err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("missing inputs should fail schema validation: %v", err)
	}
	fresh, _ := s.WorkflowRun(c, r.ID)
	if fresh.Steps[0].Result != nil {
		t.Fatal("invalid result was persisted")
	}
	var in NodeResult
	if err = json.Unmarshal([]byte(`{"summary":"verified","params":{"passed":true,"count":2,"files":["a.txt","b.txt"]}}`), &in); err != nil {
		t.Fatal(err)
	}
	if err = s.testHandoffResult(r.ID, 1, token, in); err != nil {
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
		{"", `{"type":"object","properties":{"n":{"type":"imaginary"}}}`},
		{"", `{"type":"object","$ref":"https://example.test/schema"}`},
		{"", `{"type":"object","propertiez":{}}`},
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
	w.Edges = []WorkflowEdge{{Source: w.Entry, Target: "done", Route: "next", Mode: "handoff"}}
	w = configuredCompletion(t, w, "", completionTestSchema)
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
	p := w.Nodes[0].HandoffParameters["done"]
	p.Schema = json.RawMessage(`{"type":"object","required":["new_field"]}`)
	w.Nodes[0].HandoffParameters["done"] = p
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
		if tool.Name == "complete_node" {
			t.Fatal("obsolete tool exposed")
		}
		if tool.Name == "handoff" {
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
	for _, args := range []string{`{"target":"done","message":"bad","params":{"passed":true,"count":9007199254740993,"files":[]}}`, `{"target":"done","message":"bad"}`, `{"target":"done","message":"bad","params":{"passed":"true","count":2,"files":[]}}`, `{"target":"done","message":"bad","params":{"passed":true,"count":2,"files":[],"extra":1}}`} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "handoff", Arguments: json.RawMessage(args)})
		if err == nil && !result.IsError {
			t.Fatal("accepted invalid arguments", args)
		}
		fresh, _ := s.WorkflowRun(c, r.ID)
		if fresh.Seq != 1 || fresh.Steps[0].Result != nil {
			t.Fatal("invalid completion changed state")
		}
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "handoff", Arguments: json.RawMessage(`{"target":"done","message":"verified","params":{"passed":true,"count":2,"files":["a.txt"]}}`)})
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
	if len(payload.Previous) != 1 || payload.Previous[0].Result.Params["passed"] != true || payload.Previous[0].Result.Params["count"] != float64(2) {
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
	raw := `{"summary":"ok","params":{}}`
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
	if _, ok := got["params"].(map[string]any); !ok {
		t.Fatalf("empty object lost: %s", encoded)
	}
	for _, raw := range []string{`{"summary":"bad","params":{"id":9007199254740993}}`, `{"summary":"bad","params":{"nested":[{"value":1.0000000000000001}]}}`} {
		if err := json.Unmarshal([]byte(raw), &result); err == nil {
			t.Fatalf("accepted lossy number: %s", raw)
		}
	}
	n := HandoffParameters{Schema: json.RawMessage(`{"type":"object","properties":{"id":{"const":9007199254740993}}}`)}
	if _, err := completionInputsSchema(n); err == nil {
		t.Fatal("accepted lossy schema constraint")
	}
}

func TestHandoffParamsToggleAndDescriptions(t *testing.T) {
	p := HandoffParameters{Schema: json.RawMessage(completionTestSchema), Instructions: "根据实际执行结果填写；count为通过用例数。"}
	n := WorkflowNode{ID: "a", Kind: "agent", HandoffParameters: map[string]HandoffParameters{"done": p}}
	w := Workflow{Nodes: []WorkflowNode{n, {ID: "done", Kind: "end"}}, Edges: []WorkflowEdge{{Source: "a", Target: "done", Route: "next", Mode: "handoff"}}}
	schema, err := handoffToolSchema(w, n)
	if err != nil {
		t.Fatal(err)
	}
	if schema.Properties["params"] != nil || schema.Properties["context"] != nil {
		t.Fatal("disabled params exposed")
	}
	p.Enabled = true
	n.HandoffParameters["done"] = p
	schema, err = handoffToolSchema(w, n)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"target", "message", "params", "artifacts"} {
		if schema.Properties[key] == nil || schema.Properties[key].Description == "" {
			t.Fatal("missing description", key)
		}
	}
	if !strings.Contains(completionGuidance(w, n), "count为通过用例数") {
		t.Fatal("missing target guidance")
	}
	if err := validateCompletionResult(n, "done", NodeResult{Params: map[string]any{"passed": true, "count": 2, "files": []any{"proof.txt"}}}); err != nil {
		t.Fatal(err)
	}
	if err := validateCompletionResult(n, "done", NodeResult{Params: map[string]any{"passed": "true"}}); err == nil {
		t.Fatal("invalid params accepted")
	}
	p.Enabled = false
	n.HandoffParameters["done"] = p
	if strings.Contains(completionGuidance(w, n), "count为通过用例数") {
		t.Fatal("disabled instructions leaked")
	}
	if err := validateCompletionResult(n, "done", NodeResult{Params: map[string]any{}}); err == nil {
		t.Fatal("disabled params accepted")
	}
	p.Enabled = true
	p.Schema = nil
	n.HandoffParameters["done"] = p
	if err := validateCompletionConfig(w, n); err == nil {
		t.Fatal("enabled params need schema")
	}
}

// Real platform/MCP transports prove both parameter consumption and the next
// Agent input. Native reasoning is controlled by Claim/Complete in this test.
func TestAgentParamsReachMCPAndNextAgent(t *testing.T) {
	s := testStore(t)
	var received string
	c, w, dir, _ := executionFixture(t, s, func(_ context.Context, text string) (*mcp.CallToolResult, any, error) {
		received = text
		return nil, map[string]any{"checked_commit": text}, nil
	})
	agent := testAgent(t, s)
	w.Entry = "author"
	w.Nodes = append(w.Nodes, WorkflowNode{ID: "author", Name: "Author", Kind: "agent", AgentID: agent.ID, HandoffParameters: map[string]HandoffParameters{"call": {Enabled: true, Schema: json.RawMessage(`{"type":"object","properties":{"commit":{"type":"string"}},"required":["commit"],"additionalProperties":false}`), Instructions: "commit填写实际提交，不填分支名。"}}}, WorkflowNode{ID: "review", Name: "Review", Kind: "agent", AgentID: agent.ID})
	w.Nodes[len(w.Nodes)-2].HandoffParameters["review"] = HandoffParameters{Enabled: true, Schema: json.RawMessage(`{"type":"object","properties":{"ready":{"type":"boolean"}},"required":["ready"],"additionalProperties":false}`)}
	w.Nodes[0].RuleHandoff = executionTestRules("mcp", "review")
	w.Nodes[0].Execution.Bindings = map[string]ExecutionBinding{"text": {Node: "author", Path: "/result/params/commit"}}
	w.Edges = append(w.Edges, WorkflowEdge{Source: "author", Target: "review", Route: "direct", Mode: "handoff"}, WorkflowEdge{Source: "author", Target: "done", Route: "finish", Mode: "handoff"}, WorkflowEdge{Source: "author", Target: "call", Route: "next", Mode: "handoff"}, WorkflowEdge{Source: "review", Target: "done", Route: "next", Mode: "handoff"})
	var err error
	w, err = s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "verify params"})
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := mcp.NewClient(&mcp.Implementation{Name: "params-flow", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: remote.URL + "/api/workflow-node/mcp", HTTPClient: &http.Client{Transport: bearerTransport{http.DefaultTransport, *conv.Snapshot.Env[workflowTokenEnv]}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for _, args := range []string{`{"target":"review","message":"wrong branch schema","params":{"commit":"abc"}}`, `{"target":"done","message":"disabled branch","params":{}}`, `{"target":"call","message":"bad","params":{"commit":1}}`, `{"target":"call","message":"old","context":{"commit":"old"}}`} {
		out, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "handoff", Arguments: json.RawMessage(args)})
		if err == nil && !out.IsError {
			t.Fatal("bad params accepted")
		}
	}
	out, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "handoff", Arguments: json.RawMessage(`{"target":"call","message":"检查已提交版本","params":{"commit":"abc123 $(not a command)"}}`)})
	if err != nil || out.IsError {
		t.Fatal(out, err)
	}
	s.Complete(conv.ID, msg.ID, "completed", "")
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		h.workflows.Tick(ctx)
		r, _ = s.WorkflowRun(c, r.ID)
		if r.Seq == 3 && r.Steps[2].ConversationID != "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if received != "abc123 $(not a command)" {
		t.Fatal("params not passed literally", received)
	}
	review, input, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(input.Content, "checked_commit") || !strings.Contains(input.Content, "abc123 $(not a command)") {
		t.Fatal("execution result missing from Agent input", input.Content)
	}
	if err = s.handoffWorkflowNode(r.ID, 3, *review.Snapshot.Env[workflowTokenEnv], HandoffInput{Target: "done", Message: "检查完成"}); err != nil {
		t.Fatal(err)
	}
	s.Complete(review.ID, input.ID, "completed", "")
	h.workflows.Tick(ctx)
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "completed" {
		t.Fatal(r.Status)
	}
}
