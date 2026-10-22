package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func executionTestRules(mode, target string) *WorkflowRules {
	p := &WorkflowRules{SchemaScope: "value", Rules: []WorkflowRule{{Title: "完成", When: "$ == 0", Target: target}}, OnInvalid: WorkflowRuleFailure{Action: "stop"}, OnExpression: WorkflowRuleFailure{Action: "stop"}}
	if mode == "mcp" {
		p.SchemaScope = "input"
		p.Schema = json.RawMessage(`{"type":"object","properties":{"output":{"type":"object","properties":{"is_error":{"type":"boolean"},"result":{}},"required":["is_error"]}},"required":["output"]}`)
		p.Rules[0].When = "output.is_error == false"
	}
	return p
}

func executionFixture(t *testing.T, s *Store, handler func(context.Context, string) (*mcp.CallToolResult, any, error), wrappers ...func(http.Handler) http.Handler) (Caller, Workflow, string, RegisteredToolServer) {
	t.Helper()
	if err := s.initAuth("password"); err != nil {
		t.Fatal(err)
	}
	c, err := s.userCaller("admin")
	if err != nil {
		t.Fatal(err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "workflow-execution", Version: "1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "deliver"}, func(ctx context.Context, r *mcp.CallToolRequest, in struct {
		Text string `json:"text"`
	}) (*mcp.CallToolResult, any, error) {
		return handler(ctx, in.Text)
	})
	var transport http.Handler = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	for _, wrap := range wrappers {
		transport = wrap(transport)
	}
	remote := httptest.NewServer(transport)
	t.Cleanup(remote.Close)
	config, err := s.SaveToolServer(RegisteredToolServer{ID: "execution-test", Name: "Execution test", Enabled: true, Connection: MCPConnection{URL: remote.URL}})
	if err != nil {
		t.Fatal(err)
	}
	api := NewServer(s, nil, nil, "password", "http://localhost")
	req := httptest.NewRequest("POST", "/api/tool-servers/execution-test/discover", nil)
	req.SetPathValue("id", config.ID)
	rec := httptest.NewRecorder()
	api.discoverToolServer(rec, req, c)
	if rec.Code != 200 {
		t.Fatalf("discover: %s", rec.Body.String())
	}
	config, err = s.ToolServer(config.ID)
	if err != nil {
		t.Fatal(err)
	}
	w, err := s.SaveWorkflow(c, Workflow{Name: "MCP execution", Enabled: true, Entry: "call", MaxSteps: 10, ContextVersion: 2,
		Nodes: []WorkflowNode{{ID: "call", Name: "Deliver", Kind: "execution", RuleHandoff: executionTestRules("mcp", "done"), X: 40, Y: 40, Execution: &WorkflowExecution{Mode: "mcp", ServerID: config.ID, Tool: "deliver", Arguments: json.RawMessage(`{"text":"literal $(no shell)"}`)}}, {ID: "done", Name: "Done", Kind: "end", X: 340, Y: 40}},
		Edges: []WorkflowEdge{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c, w, t.TempDir(), config
}

func TestWorkflowExecutionMCPPersistsRawResultAndDeduplicates(t *testing.T) {
	s := testStore(t)
	var calls atomic.Int32
	c, w, dir, _ := executionFixture(t, s, func(ctx context.Context, text string) (*mcp.CallToolResult, any, error) {
		calls.Add(1)
		return nil, map[string]any{"text": text, "number": 17}, nil
	})
	reloaded, err := s.Workflow(c, w.ID)
	if err != nil || string(reloaded.Nodes[0].Execution.Arguments) != string(w.Nodes[0].Execution.Arguments) {
		t.Fatalf("configuration lost: %+v %v", reloaded, err)
	}
	in := WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "test", RequestID: "execution-once"}
	r, err := s.StartWorkflow(c, in)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := s.StartWorkflow(c, in)
	if err != nil || duplicate.ID != r.ID {
		t.Fatalf("dedupe: %v", err)
	}
	e := NewWorkflowEngine(s, nil, "")
	r = waitConnectorRun(t, e, r.ID, "completed")
	receipt := r.Steps[0].ExecutionReceipt
	if receipt == nil || receipt.MCP == nil || receipt.MCP.IsError {
		t.Fatalf("missing result: %+v", receipt)
	}
	raw, _ := json.Marshal(receipt.MCP.StructuredContent)
	if !strings.Contains(string(raw), `"number":17`) || !strings.Contains(string(raw), "literal $(no shell)") {
		t.Fatalf("raw result changed: %s", raw)
	}
	NewWorkflowEngine(s, nil, "").Tick(context.Background())
	if calls.Load() != 1 {
		t.Fatalf("replayed %d times", calls.Load())
	}
}

func TestWorkflowExecutionMCPReferencesTaskAndLatestNodeResult(t *testing.T) {
	s := testStore(t)
	var calls atomic.Int32
	wanted := "quoted \"input\"\n$(never evaluated)"
	c, w, dir, config := executionFixture(t, s, func(_ context.Context, text string) (*mcp.CallToolResult, any, error) {
		calls.Add(1)
		return nil, map[string]any{"text": text}, nil
	})
	first, _ := json.Marshal(map[string]any{"mode": "mcp", "server_id": config.ID, "tool": "deliver", "arguments": map[string]any{"text": "old literal"}, "bindings": map[string]any{"text": map[string]any{"path": "/parameters/message"}}})
	if err := json.Unmarshal(first, w.Nodes[0].Execution); err != nil {
		t.Fatal(err)
	}
	second := WorkflowNode{ID: "consume", Name: "Consume result", Kind: "execution", RuleHandoff: executionTestRules("mcp", "done"), Execution: &WorkflowExecution{}}
	raw, _ := json.Marshal(map[string]any{"mode": "mcp", "server_id": config.ID, "tool": "deliver", "arguments": map[string]any{}, "bindings": map[string]any{"text": map[string]any{"node": "call", "path": "/execution_receipt/mcp/structuredContent/text"}}})
	if err := json.Unmarshal(raw, second.Execution); err != nil {
		t.Fatal(err)
	}
	w.Nodes = append(w.Nodes, second)
	w.Nodes[0].RuleHandoff.Rules[0].Target = "consume"
	w.Edges = nil
	w, err := s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := s.Workflow(c, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := json.Marshal(reloaded.Nodes[0].Execution)
	if !strings.Contains(string(saved), `"bindings"`) {
		t.Fatal("task references were dropped on save/reload")
	}
	in := WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "task", RequestID: "bound-once", Parameters: map[string]string{"message": wanted}}
	r, err := s.StartWorkflow(c, in)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := s.StartWorkflow(c, in)
	if err != nil || duplicate.ID != r.ID {
		t.Fatal("bound task was not deduplicated", err)
	}
	r = waitConnectorRun(t, NewWorkflowEngine(s, nil, ""), r.ID, "completed")
	for _, step := range r.Steps[:2] {
		raw, _ := json.Marshal(step.ExecutionReceipt.MCP.StructuredContent)
		var value map[string]any
		json.Unmarshal(raw, &value)
		if value["text"] != wanted {
			t.Fatal("reference changed, interpolated or fell back to a literal", string(raw))
		}
	}
	if calls.Load() != 2 {
		t.Fatal("reference calls replayed", calls.Load())
	}
}

func TestWorkflowExecutionMCPReferenceFailureSendsNothing(t *testing.T) {
	for _, scenario := range []struct {
		parameters map[string]string
		path       string
	}{{map[string]string{}, "/parameters/message"}, {map[string]string{"message": "private value"}, "/parameters"}} {
		s := testStore(t)
		var calls atomic.Int32
		c, w, dir, config := executionFixture(t, s, func(_ context.Context, text string) (*mcp.CallToolResult, any, error) {
			calls.Add(1)
			return nil, map[string]any{"text": text}, nil
		})
		raw, _ := json.Marshal(map[string]any{"mode": "mcp", "server_id": config.ID, "tool": "deliver", "arguments": map[string]any{"text": "do not use this fallback"}, "bindings": map[string]any{"text": map[string]any{"path": scenario.path}}})
		json.Unmarshal(raw, w.Nodes[0].Execution)
		w, err := s.SaveWorkflow(c, w)
		if err != nil {
			t.Fatal(err)
		}
		r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "task", Parameters: scenario.parameters})
		if err != nil {
			t.Fatal(err)
		}
		r = waitConnectorRun(t, NewWorkflowEngine(s, nil, ""), r.ID, "failed")
		if calls.Load() != 0 || r.Error == "" {
			t.Fatal("invalid reference was sent to the external tool", calls.Load(), r.Error)
		}
		var actions int
		s.DB.QueryRow(`SELECT count(*) FROM workflow_connector_actions WHERE run_id=?`, r.ID).Scan(&actions)
		if actions != 0 {
			t.Fatal("invalid reference recorded a dispatch intent", actions)
		}
	}
}

func TestWorkflowExecutionMCPReferenceTypesAndLatestAttempt(t *testing.T) {
	values := json.RawMessage(`{"zero":0,"off":false,"nil":null,"items":["first",{"a/b~c":9007199254740993}]}`)
	prior := WorkflowStep{Seq: 1, NodeID: "report", Status: "completed", ExecutionReceipt: &ConnectorReceipt{MCP: &mcp.CallToolResult{StructuredContent: values}}}
	run := WorkflowRun{ID: "this-run", Steps: []WorkflowStep{prior}}
	current := WorkflowStep{Seq: 4, NodeID: "consume"}
	args, err := resolveExecutionArguments(run, current, WorkflowExecution{Arguments: json.RawMessage(`{"literal":9007199254740995}`), Bindings: map[string]ExecutionBinding{
		"zero":  {Node: "report", Path: "/execution_receipt/mcp/structuredContent/zero"},
		"off":   {Node: "report", Path: "/execution_receipt/mcp/structuredContent/off"},
		"nil":   {Node: "report", Path: "/execution_receipt/mcp/structuredContent/nil"},
		"large": {Node: "report", Path: "/execution_receipt/mcp/structuredContent/items/1/a~1b~0c"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var actual map[string]json.RawMessage
	if err = json.Unmarshal(args, &actual); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"zero": "0", "off": "false", "nil": "null", "large": "9007199254740993", "literal": "9007199254740995"} {
		if string(actual[name]) != want {
			t.Fatalf("%s changed type or precision: %s", name, actual[name])
		}
	}
	execution := WorkflowExecution{Arguments: json.RawMessage(`{"value":"never fall back"}`), Bindings: map[string]ExecutionBinding{"value": {Node: "report", Path: "/execution_receipt/mcp/structuredContent/off"}}}
	for _, latest := range []WorkflowStep{{Seq: 3, NodeID: "report", Status: "completed"}, {Seq: 3, NodeID: "report", Status: "running"}} {
		run.Steps = []WorkflowStep{prior, latest}
		if _, err = resolveExecutionArguments(run, current, execution); err == nil {
			t.Fatal("used an older attempt after the latest lacked a result")
		}
	}
	run.Steps = []WorkflowStep{{Seq: 5, NodeID: "report", Status: "completed", ExecutionReceipt: prior.ExecutionReceipt}}
	if _, err = resolveExecutionArguments(run, current, execution); err == nil {
		t.Fatal("used a future step")
	}
	for _, path := range []string{"/credentials", "/execution_tools", "/owner"} {
		execution.Bindings["value"] = ExecutionBinding{Path: path}
		if _, err = resolveExecutionArguments(run, current, execution); err == nil {
			t.Fatal("exposed private Run field", path)
		}
	}
}

func TestWorkflowExecutionMCPRejectsSchemaAndChangedConnection(t *testing.T) {
	s := testStore(t)
	var calls atomic.Int32
	c, w, dir, config := executionFixture(t, s, func(context.Context, string) (*mcp.CallToolResult, any, error) {
		calls.Add(1)
		return nil, map[string]any{}, nil
	})
	w.Nodes[0].Execution.Arguments = json.RawMessage(`{"text":17}`)
	if _, err := s.SaveWorkflow(c, w); err == nil {
		t.Fatal("schema-invalid input saved")
	}
	w.Nodes[0].Execution.Arguments = json.RawMessage(`{"text":"original"}`)
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "test"})
	if err != nil {
		t.Fatal(err)
	}
	config.Connection.URL += "/different-identity"
	if _, err = s.SaveToolServer(config); err != nil {
		t.Fatal(err)
	}
	r = waitConnectorRun(t, NewWorkflowEngine(s, nil, ""), r.ID, "failed")
	if calls.Load() != 0 || !strings.Contains(r.Error, "connection changed") {
		t.Fatalf("changed identity used: calls=%d error=%s", calls.Load(), r.Error)
	}
}

func TestWorkflowExecutionMCPBusinessFailureRoutesAndKeepsContent(t *testing.T) {
	s := testStore(t)
	c, w, dir, _ := executionFixture(t, s, func(context.Context, string) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "quota denied"}}}, nil, nil
	})
	w.Nodes[0].Execution.Retry = &ExecutionRetry{MaxRetries: 2, IntervalSeconds: 1}
	w.Nodes = append(w.Nodes, WorkflowNode{ID: "review", Name: "Review", Kind: "approval"})
	w.Nodes[0].RuleHandoff.Rules = append(w.Nodes[0].RuleHandoff.Rules, WorkflowRule{Title: "失败", When: "output.is_error == true", Target: "review"})
	w.Edges = append(w.Edges, WorkflowEdge{Source: "review", Route: "next", Target: "done"})
	w, err := s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "test"})
	if err != nil {
		t.Fatal(err)
	}
	r = waitConnectorRun(t, NewWorkflowEngine(s, nil, ""), r.ID, "waiting")
	if r.Seq != 2 || r.Steps[0].Result.Route != "rule_2" || r.Steps[0].ExecutionReceipt.MCP.Content[0].(*mcp.TextContent).Text != "quota denied" {
		t.Fatalf("lost failure routing: %+v", r)
	}
}

func TestWorkflowExecutionMCPStopPreventsReplay(t *testing.T) {
	s := testStore(t)
	started := make(chan struct{})
	var calls atomic.Int32
	c, w, dir, _ := executionFixture(t, s, func(ctx context.Context, _ string) (*mcp.CallToolResult, any, error) {
		calls.Add(1)
		close(started)
		<-ctx.Done()
		return nil, nil, ctx.Err()
	})
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "test"})
	if err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "")
	e.Tick(context.Background())
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("tool never started")
	}
	if err = e.Stop(c, r.ID, 1); err != nil {
		t.Fatal(err)
	}
	r = waitConnectorRun(t, e, r.ID, "stopped")
	if r.Steps[0].ExecutionReceipt == nil {
		t.Fatal("lost cancellation receipt")
	}
	e = NewWorkflowEngine(s, nil, "")
	if err = e.Resume(c, r.ID, 1, "inspect and continue"); err == nil {
		t.Fatal("resumed a dispatched external write")
	}
	e.Tick(context.Background())
	if calls.Load() != 1 {
		t.Fatal("stop/restart repeated the tool")
	}
}

func TestWorkflowExecutionMCPResultReachesNativeInputAfterStoreRestart(t *testing.T) {
	s := testStore(t)
	c, w, dir, _ := executionFixture(t, s, func(context.Context, string) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "external literal response"}}, StructuredContent: map[string]any{"ready": true}}, nil, nil
	})
	a := testAgent(t, s)
	w.Nodes = append(w.Nodes, WorkflowNode{ID: "reader", Name: "Review", Kind: "agent", AgentID: a.ID})
	w.Nodes[0].RuleHandoff.Rules[0].Target = "reader"
	w.Edges = append(w.Edges, WorkflowEdge{Source: "reader", Route: "next", Target: "done", Mode: "handoff"})
	w, err := s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "read tool result"})
	if err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		e.Tick(context.Background())
		r, _ = s.WorkflowRun(c, r.ID)
		if r.Seq == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if r.Seq != 2 {
		t.Fatal("did not reach native reader", r.Status, r.Error)
	}
	s.Close()
	reopened, err := OpenStore(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	NewWorkflowEngine(reopened, nil, "http://localhost").Tick(context.Background())
	_, message, err := reopened.Claim()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Content, "external literal response") || !strings.Contains(message.Content, `"ready":true`) {
		t.Fatalf("lost typed tool receipt in native input: %s", message.Content)
	}
	r, err = reopened.WorkflowRun(c, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(r)
	if strings.Contains(string(raw), "connection") || strings.Contains(string(raw), "bearer_token_env_var") {
		t.Fatal("private MCP connection snapshot exposed")
	}
}

func TestWorkflowExecutionMCPDisabledDuringCallCancelsWithoutAdvance(t *testing.T) {
	s := testStore(t)
	started := make(chan struct{})
	cancelled := make(chan struct{})
	c, w, dir, config := executionFixture(t, s, func(ctx context.Context, _ string) (*mcp.CallToolResult, any, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return nil, nil, ctx.Err()
	})
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "test"})
	if err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "")
	e.Tick(context.Background())
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("tool not started")
	}
	config.Enabled = false
	if _, err = s.SaveToolServer(config); err != nil {
		t.Fatal(err)
	}
	e.Tick(context.Background())
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("disabled tool kept running")
	}
	e.connectorWG.Wait()
	r, err = s.WorkflowRun(c, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "failed" || r.Seq != 1 || r.Steps[0].ExecutionReceipt == nil || !strings.Contains(r.Error, "disabled") {
		t.Fatalf("revocation lost: %+v", r)
	}
}

// The standard verifier opts in after installing the documented browser tooling.
func TestWorkflowExecutionBrowserRoundTrip(t *testing.T) {
	if os.Getenv("PLATFORM_BROWSER_TEST") != "1" {
		t.Skip("run through scripts/verify.sh for browser dependencies")
	}
	s := testStore(t)
	var calls atomic.Int32
	_, w, workspace, _ := executionFixture(t, s, func(_ context.Context, text string) (*mcp.CallToolResult, any, error) {
		calls.Add(1)
		return nil, map[string]any{"native_response": text}, nil
	})
	_, _, creates, _ := sandboxWorkflowFixture(t, s, false, false)
	server := httptest.NewUnstartedServer(nil)
	handler := NewServer(s, nil, nil, "password", "http://"+server.Listener.Addr().String())
	server.Config.Handler = handler
	server.Start()
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	stopped := make(chan struct{})
	go func() { defer close(stopped); handler.RunWorkflows(ctx) }()
	defer func() { cancel(); <-stopped }()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "output", "playwright")
	if err = os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, "node", "web/workflow_execution_browser.cjs")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "NODE_PATH="+filepath.Join(root, "examples/github/tooling/full_harness/browser/node_modules"))
	input, _ := json.Marshal(map[string]string{"url": server.URL, "workflow": w.ID, "workspace": workspace, "screenshot": filepath.Join(output, "workflow-execution.png")})
	cmd.Stdin = strings.NewReader(string(input))
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("browser failed: %v\n%s", err, result)
	}
	if calls.Load() != 1 {
		t.Fatal("browser dispatch repeated", calls.Load())
	}
	if creates.Load() != 1 {
		t.Fatal("browser environment creation repeated", creates.Load())
	}
	t.Log(string(result))
}

func TestExecutionEnvironmentReferencesSurviveAndRejectOverrides(t *testing.T) {
	var v WorkflowExecution
	if err := json.Unmarshal([]byte(`{"mode":"command","command":"git fetch","env_refs":{"GITHUB_TOKEN":"PROJECT_GITHUB_TOKEN"}}`), &v); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v)
	if !strings.Contains(string(raw), `"env_refs"`) {
		t.Fatal("trusted references lost on round trip")
	}
	for _, name := range []string{"HOME", "PATH", "WORKFLOW_INPUT_FILE", "CODEX_HOME", "AGENT_PLATFORM_PASSWORD", "LD_PRELOAD", "bad-name"} {
		raw := []byte(`{"mode":"command","command":"git fetch","env_refs":{"` + name + `":"PROJECT_GITHUB_TOKEN"}}`)
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		if validateWorkflowExecution(&v) == nil {
			t.Errorf("accepted protected key %s", name)
		}
	}
}

func TestExecutionEnvironmentCapturesSelectedValuesAndRedactsSplitWrites(t *testing.T) {
	t.Setenv("PROJECT_MISSING_TOKEN", "")
	if _, err := executionEnvironment(map[string]string{"GITHUB_TOKEN": "PROJECT_MISSING_TOKEN"}); err == nil {
		t.Fatal("missing credential accepted")
	}
	// A captured value is immutable for dispatch/redaction even after rotation.
	t.Setenv("PROJECT_TOKEN", "captured-private")
	values, err := executionEnvironment(map[string]string{"GITHUB_TOKEN": "PROJECT_TOKEN"})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROJECT_TOKEN", "rotated-private")
	log := newBoundedConnectorLogValues(values)
	log.Write([]byte("cap"))
	log.Write([]byte("tured-private"))
	if strings.Contains(log.String(), "captured-private") || !strings.Contains(log.String(), "[redacted]") {
		t.Fatal("split secret was not redacted")
	}
}

func TestExecutionRejectsMissingRulesBeforeDispatch(t *testing.T) {
	n := WorkflowNode{ID: "call", Kind: "execution", Execution: &WorkflowExecution{Mode: "command", Command: "touch should-not-exist"}}
	if err := validateRuleHandoff(Workflow{}, n); err == nil {
		t.Fatal("missing rule policy accepted")
	}
	// No store or sandbox is needed: reject before any environment or action access.
	if err := (&WorkflowEngine{}).startExecution(context.Background(), WorkflowRun{}, WorkflowStep{}, n); err == nil || !strings.Contains(err.Error(), "rule_handoff") {
		t.Fatalf("old dispatch path remains: %v", err)
	}
}
