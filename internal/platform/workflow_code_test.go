package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestWorkflowSandboxCodeAndCommandShareEnvironmentAndRetainFailure(t *testing.T) {
	s := testStore(t)
	t.Setenv("PROJECT_GITHUB_TOKEN", "selected-\"private-fixture")
	t.Setenv("UNRELATED_SECRET", "must-not-inherit")
	if err := s.initAuth("password"); err != nil {
		t.Fatal(err)
	}
	caller, _ := s.userCaller("admin")
	var base, owner string
	var calls atomic.Int32
	var unavailable atomic.Bool
	var uploadUnavailable atomic.Bool
	var endpointUnavailable atomic.Bool
	var failEndpointAfterUpload atomic.Bool
	var uploadCount atomic.Int32
	var mu sync.Mutex
	exits := map[string]int32{}
	directory := t.TempDir()
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/sandboxes":
			var in opensandbox.CreateSandboxRequest
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				t.Error(err)
				return
			}
			owner = in.Metadata["agent-platform-run"]
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "same-environment", Metadata: in.Metadata, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case r.URL.Path == "/sandboxes/same-environment":
			if endpointUnavailable.Load() {
				w.WriteHeader(503)
				return
			}
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "same-environment", Metadata: map[string]string{"agent-platform-run": owner}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			if endpointUnavailable.Load() {
				w.WriteHeader(503)
				return
			}
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.URL.Path == "/ping":
			w.Write([]byte("pong"))
		case r.Method == "POST" && r.URL.Path == "/files/upload":
			if uploadUnavailable.Load() {
				w.WriteHeader(503)
				return
			}
			if err := r.ParseMultipartForm(2 << 20); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			metadata, _, err := r.FormFile("metadata")
			if err != nil {
				t.Error(err)
				return
			}
			defer metadata.Close()
			var value opensandbox.FileMetadata
			if err = json.NewDecoder(metadata).Decode(&value); err != nil {
				t.Error(err)
				return
			}
			source, _, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			defer source.Close()
			raw, err := io.ReadAll(source)
			if err != nil {
				t.Error(err)
				return
			}
			permissions, err := strconv.ParseUint(strconv.Itoa(value.Mode), 8, 32)
			if err != nil {
				w.WriteHeader(500)
				return
			}
			if err = os.WriteFile(filepath.Join(directory, filepath.Base(value.Path)), raw, os.FileMode(permissions)); err != nil {
				t.Error(err)
				return
			}
			if failEndpointAfterUpload.Load() && uploadCount.Add(1) == 2 {
				endpointUnavailable.Store(true)
			}
		case r.Method == "POST" && r.URL.Path == "/command":
			if r.Header.Get("OPEN-SANDBOX-API-KEY") != "" {
				t.Error("control credential leaked")
			}
			var request opensandbox.RunCommandRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
				return
			}
			if request.Cwd != "/workspace" {
				t.Error("not using shared workspace")
			}
			id := fmt.Sprintf("execution-%d", calls.Add(1))
			// The fixture maps container /tmp paths into its isolated filesystem.
			shell := strings.ReplaceAll(request.Command, "/workspace", directory)
			for _, prefix := range []string{"/tmp/agent-platform-code-"} {
				shell = strings.ReplaceAll(shell, prefix, filepath.Join(directory, "agent-platform-code-"))
			}
			command := exec.CommandContext(r.Context(), "/bin/sh", "-c", shell)
			command.Dir = directory
			command.Env = []string{"PATH=" + os.Getenv("PATH")}
			for key, value := range request.Envs {
				if key == "WORKFLOW_INPUT_FILE" {
					value = filepath.Join(directory, filepath.Base(value))
				}
				command.Env = append(command.Env, key+"="+value)
			}
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			err := command.Run()
			output := stdout.Bytes()
			exit := int32(0)
			if err != nil {
				var failure *exec.ExitError
				if !errors.As(err, &failure) {
					t.Error(err)
					return
				}
				exit = int32(failure.ExitCode())
			}
			mu.Lock()
			exits[id] = exit
			mu.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			for _, event := range []any{map[string]any{"type": "init", "text": id}, map[string]any{"type": "stdout", "text": string(output[:min(3, len(output))])}, map[string]any{"type": "stdout", "text": string(output[min(3, len(output)):])}, map[string]any{"type": "stderr", "text": stderr.String()}, map[string]any{"type": "execution_complete"}} {
				raw, _ := json.Marshal(event)
				fmt.Fprintf(w, "%s\n\n", raw)
			}
		case strings.HasPrefix(r.URL.Path, "/command/status/"):
			if unavailable.Load() {
				w.WriteHeader(503)
				return
			}
			id := strings.TrimPrefix(r.URL.Path, "/command/status/")
			mu.Lock()
			exit, ok := exits[id]
			mu.Unlock()
			if !ok {
				w.WriteHeader(404)
				return
			}
			json.NewEncoder(w).Encode(opensandbox.CommandStatusResponse{ID: id, ExitCode: &exit})
		default:
			t.Error("unexpected endpoint", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer remote.Close()
	base = remote.URL
	s.sandbox, _ = newOpenSandbox(base, "private-control-key", "python:3.12-slim", 600)
	graph, err := s.SaveWorkflow(caller, Workflow{Name: "Run code then project command", Enabled: true, Environment: "opensandbox", ContextVersion: 2, MaxSteps: 10, Entry: "code", Nodes: []WorkflowNode{
		{ID: "code", Name: "Write artifact", Kind: "execution", RuleHandoff: executionTestRules("code", "command"), Execution: &WorkflowExecution{Mode: "code", Output: &ExecutionOutput{Source: "exit_code"}, Language: "python", Env: map[string]string{"MODE": "ordinary-value"}, EnvRefs: map[string]string{"GITHUB_TOKEN": "PROJECT_GITHUB_TOKEN"}, Code: `import json,os; assert os.environ["MODE"]=="ordinary-value"; print(os.environ["MODE"]); assert len(os.environ["GITHUB_TOKEN"])==25; assert "UNRELATED_SECRET" not in os.environ; print(os.environ["GITHUB_TOKEN"]); inputs=json.load(open(os.environ['WORKFLOW_INPUT_FILE'])); assert len(inputs['input'])==60000; assert len(inputs['parameters']['parameter_30'])==4096; print(inputs['parameters']['commit']); open('proof.txt','w').write('shared proof')`}},
		{ID: "command", Name: "Read same artifact", Kind: "execution", RuleHandoff: &WorkflowRules{SchemaScope: "value", Rules: []WorkflowRule{{Title: "完成", When: "$ == 0", Target: "done"}, {Title: "失败", When: "$ != 0", Target: "review"}}, OnInvalid: WorkflowRuleFailure{Action: "stop"}, OnExpression: WorkflowRuleFailure{Action: "stop"}}, Execution: &WorkflowExecution{Mode: "command", Output: &ExecutionOutput{Source: "exit_code"}, Command: `python3 -c "import os,json; assert 'MODE' not in os.environ; v=json.load(open(os.environ['WORKFLOW_INPUT_FILE'])); assert v['previous_results'][0]['execution_receipt']['sandbox']['id']=='execution-1'" && cat proof.txt; exit 7`}},
		{ID: "review", Name: "Inspect nonzero result", Kind: "approval"}, {ID: "done", Name: "Done", Kind: "end"}}, Edges: []WorkflowEdge{{Source: "review", Route: "next", Target: "done"}}})
	if err != nil {
		t.Fatal(err)
	}
	input := WorkflowStart{WorkflowID: graph.ID, Input: "use existing artifacts", Parameters: map[string]string{"commit": "literal $(not executed) 'value'"}, RequestID: "code-run-once"}
	input.Input = strings.Repeat("x", 60000)
	for i := 0; i < 31; i++ {
		input.Parameters[fmt.Sprintf("parameter_%d", i)] = strings.Repeat("x", 4096)
	}
	run, err := s.StartWorkflow(caller, input)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "")
	run = waitConnectorRun(t, engine, run.ID, "waiting")
	if run.Seq != 3 || calls.Load() != 2 || run.Environment.ID != "same-environment" {
		t.Fatal("flow or shared environment lost", run, calls.Load())
	}
	if run.Steps[0].ExecutionReceipt.Sandbox.ID != "execution-1" || !strings.Contains(run.Steps[0].ExecutionReceipt.Output, "literal $(not executed)") {
		t.Fatal("code input/receipt lost")
	}
	if run.Steps[0].ExecutionReceipt.Cwd != "/workspace" || len(run.Steps[0].ExecutionReceipt.EnvNames) != 3 {
		t.Fatal("execution location metadata missing", run.Steps[0].ExecutionReceipt)
	}
	second := run.Steps[1].ExecutionReceipt
	if second.ExitCode == nil || *second.ExitCode != 7 || !strings.Contains(second.Output, "shared proof") || second.Log == nil || second.Sandbox.ID != "execution-2" {
		t.Fatal("original failure not retained", second)
	}
	firstLog, err := s.connectorOutput(run, 1, 0)
	if err != nil || strings.Contains(firstLog.Output, "selected-\"private-fixture") || !strings.Contains(firstLog.Output, "[redacted]") || !strings.Contains(firstLog.Output, "ordinary-value") {
		t.Fatal("selected secret escaped retained log", err)
	}
	frozen, _ := json.Marshal(run)
	if strings.Contains(string(frozen), "selected-") {
		t.Fatal("secret escaped persisted run")
	}
	page, err := s.connectorOutput(run, 2, 0)
	if err != nil || !strings.Contains(page.Output, "shared proof") || !page.EOF {
		t.Fatal("retained log unavailable", page, err)
	}
	again, err := s.StartWorkflow(caller, input)
	if err != nil || again.ID != run.ID {
		t.Fatal("duplicate started", err)
	}
	t.Setenv("MISSING_EXECUTION_CREDENTIAL", "")
	missing := graph.Nodes[0]
	copied := *missing.Execution
	missing.Execution = &copied
	missing.Execution.EnvRefs = map[string]string{"GITHUB_TOKEN": "MISSING_EXECUTION_CREDENTIAL"}
	if err := engine.startExecution(context.Background(), run, WorkflowStep{Seq: 99}, missing); err == nil {
		t.Fatal("missing reference dispatched")
	}
	var intents int
	if err := s.DB.QueryRow(`SELECT count(*) FROM workflow_connector_actions WHERE run_id=? AND seq=99`, run.ID).Scan(&intents); err != nil || intents != 0 {
		t.Fatal("missing reference left an action intent", intents, err)
	}
	engine.Tick(context.Background())
	if calls.Load() != 2 {
		t.Fatal("command replayed")
	}
	// A completed local goroutine does not prove the remote command stopped.
	blocked, err := s.SaveWorkflow(caller, Workflow{Name: "Unknown command completion", Enabled: true, Environment: "opensandbox", ContextVersion: 2, MaxSteps: 10, Entry: "call", Nodes: []WorkflowNode{{ID: "call", Name: "Original command", Kind: "execution", RuleHandoff: executionTestRules("command", "done"), Execution: &WorkflowExecution{Mode: "command", Output: &ExecutionOutput{Source: "exit_code"}, Command: "printf retained"}}, {ID: "done", Name: "Done", Kind: "end"}}, Edges: []WorkflowEdge{}})
	if err != nil {
		t.Fatal(err)
	}
	unavailable.Store(true)
	unknown, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: blocked.ID, RequestID: "unknown-command", Input: "check failure"})
	if err != nil {
		t.Fatal(err)
	}
	unknown = waitConnectorRun(t, engine, unknown.ID, "failed")
	if err := engine.Stop(caller, unknown.ID, unknown.Seq); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	unknown, _ = s.WorkflowRun(caller, unknown.ID)
	if unknown.Status == "stopped" {
		t.Fatal("unconfirmed remote command was declared stopped")
	}
	if err := engine.Return(caller, unknown.ID, unknown.Seq, "call", "inspect original"); err == nil {
		t.Fatal("overlapping work accepted")
	}
	before := calls.Load()
	unavailable.Store(false)
	if err := engine.ReconcileSandbox(context.Background(), caller, unknown.ID); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != before {
		t.Fatal("reconciliation replayed command")
	}
	if err := engine.Stop(caller, unknown.ID, unknown.Seq); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	unknown, _ = s.WorkflowRun(caller, unknown.ID)
	if unknown.Status != "stopped" {
		t.Fatal("independently stopped command cannot recover", unknown.Status, unknown.Error)
	}
	uploadUnavailable.Store(true)
	prepareFailure, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: blocked.ID, RequestID: "prepare-failure", Input: "preserve prior artifacts"})
	if err != nil {
		t.Fatal(err)
	}
	prepareFailure = waitConnectorRun(t, engine, prepareFailure.ID, "failed")
	if calls.Load() != before {
		t.Fatal("preparation failure dispatched a command")
	}
	if err := engine.Stop(caller, prepareFailure.ID, prepareFailure.Seq); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	prepareFailure, _ = s.WorkflowRun(caller, prepareFailure.ID)
	if prepareFailure.Status != "stopped" {
		t.Fatal("known undispatched preparation cannot recover", prepareFailure.Status, prepareFailure.Error)
	}
	if err := engine.Return(caller, prepareFailure.ID, prepareFailure.Seq, "call", "inspect upload failure, preserve files"); err != nil {
		t.Fatal(err)
	}
	uploadUnavailable.Store(false)
	prepareFailure, _ = s.WorkflowRun(caller, prepareFailure.ID)
	if err := engine.Stop(caller, prepareFailure.ID, prepareFailure.Seq); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	failEndpointAfterUpload.Store(true)
	uploadCount.Store(0)
	endpointFailure, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: blocked.ID, RequestID: "endpoint-failure", Input: "retain prepared source"})
	if err != nil {
		t.Fatal(err)
	}
	endpointFailure = waitConnectorRun(t, engine, endpointFailure.ID, "failed")
	if calls.Load() != before {
		t.Fatal("read-only environment lookup failure sent command")
	}
	if err := engine.Stop(caller, endpointFailure.ID, endpointFailure.Seq); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	endpointFailure, _ = s.WorkflowRun(caller, endpointFailure.ID)
	if endpointFailure.Status != "stopped" {
		t.Fatal("pre-dispatch read-only lookup failure cannot recover", endpointFailure.Status, endpointFailure.Error)
	}
	endpointUnavailable.Store(false)
	failEndpointAfterUpload.Store(false)

	for _, source := range []string{"stdout", "file"} {
		t.Run("structured-"+source, func(t *testing.T) {
			n := ruleFixture()
			n.Execution = &WorkflowExecution{Mode: "command", Command: `printf '{"passed":true,"score":86}'`, Output: &ExecutionOutput{Source: source}}
			if source == "file" {
				n.Execution.Command += " > result.json"
				n.Execution.Output.Path = "result.json"
			}
			g, err := s.SaveWorkflow(caller, Workflow{Name: "Structured execution", Enabled: true, Environment: "opensandbox", ContextVersion: 2, MaxSteps: 10, Entry: "call", Nodes: []WorkflowNode{n, {ID: "done", Name: "Done", Kind: "end"}, {ID: "review", Name: "Review", Kind: "end"}}})
			if err != nil {
				t.Fatal(err)
			}
			r, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: g.ID, Input: "check structured output", RequestID: "structured-" + source})
			if err != nil {
				t.Fatal(err)
			}
			r = waitConnectorRun(t, engine, r.ID, "completed")
			if r.Steps[0].Result.Route != "rule_1" || r.Steps[1].NodeID != "done" || string(r.Steps[0].ExecutionReceipt.Result) != `{"passed":true,"score":86}` {
				t.Fatalf("structured routing failed: %+v; receipt=%+v", r.Steps, r.Steps[0].ExecutionReceipt)
			}
		})
	}
	for _, c := range []struct{ name, source, command, schema, condition, want string }{
		{"make-pass", "exit_code", "printf 'verify:\n\t@printf pass\n' > Makefile; make verify", "", "$ == 0", "done"},
		{"make-fail", "exit_code", "printf 'verify:\n\t@exit 1\n' > Makefile; make verify", "", "$ == 0", "other"},
		{"stdout", "stdout", `printf 'PASS'; printf 'ERROR' >&2`, "", `$ == "PASS"`, "done"},
		{"stderr", "stderr", `printf 'IGNORE'; printf 'DIAGNOSTIC' >&2`, "", `$ == "DIAGNOSTIC"`, "done"},
		{"file-text", "file", `printf 'PASS' > selected.txt`, "", `$ == "PASS"`, "done"},
		{"json", "stdout", `printf '{"passed":true}'`, `{"type":"object","properties":{"passed":{"type":"boolean"}},"required":["passed"]}`, "$.passed == true", "done"},
		{"bad-json", "stderr", `printf 'not-json' >&2`, `{"type":"integer"}`, "$ == 0", "parse"},
		{"bad-schema", "file", `printf '"wrong"' > selected.txt`, `{"type":"integer"}`, "$ == 0", "invalid"},
		{"missing-file", "file", `rm -f selected.txt`, "", `$ == "PASS"`, "read"},
	} {
		t.Run("selected-"+c.name, func(t *testing.T) {
			n := valueRuleNode(t, c.source, c.schema, c.condition)
			n.Execution.Command = c.command
			if c.source == "file" {
				n.Execution.Output.Path = "selected.txt"
			}
			g := valueRuleWorkflow(n)
			g.Name = "Selected data"
			g.Enabled = true
			g.Environment = "opensandbox"
			g.ContextVersion = 2
			g.Entry = "call"
			g.MaxSteps = 10
			g, err := s.SaveWorkflow(caller, g)
			if err != nil {
				t.Fatal(err)
			}
			input := WorkflowStart{WorkflowID: g.ID, Input: "selected data", RequestID: "selected-" + c.name}
			r, err := s.StartWorkflow(caller, input)
			if err != nil {
				t.Fatal(err)
			}
			// Persisted source selection and rules belong to this run even after edits.
			g.Nodes[0].RuleHandoff.Rules[0].When = "!(" + c.condition + ")"
			if _, err = s.SaveWorkflow(caller, g); err != nil {
				t.Fatal(err)
			}
			r = waitConnectorRun(t, engine, r.ID, "completed")
			if len(r.Steps) != 2 || r.Steps[1].NodeID != c.want {
				t.Fatalf("route wrong: %+v", r.Steps)
			}
			if c.name == "make-fail" && *r.Steps[0].ExecutionReceipt.ExitCode == 0 {
				t.Fatal("make failure not captured")
			}
			count := calls.Load()
			again, err := s.StartWorkflow(caller, input)
			if err != nil || again.ID != r.ID || calls.Load() != count {
				t.Fatal("duplicate replay", err)
			}
		})
	}

	if err := engine.Return(caller, unknown.ID, unknown.Seq, "call", "confirmed original stopped"); err != nil {
		t.Fatal(err)
	}

}

func TestWorkflowExecutionRejectsMixedModesAndHostCode(t *testing.T) {
	for _, value := range []*WorkflowExecution{{Mode: "code", Output: &ExecutionOutput{Source: "exit_code"}, Language: "go", Code: "main()"}, {Mode: "command", Output: &ExecutionOutput{Source: "exit_code"}, Command: "go test ./...", Code: "other instructions"}, {Mode: "code", Output: &ExecutionOutput{Source: "exit_code"}, Language: "python", Code: "print(1)", ServerID: "other-server"}} {
		if err := validateWorkflowExecution(value); err == nil {
			t.Fatal("conflicting execution configuration accepted", value)
		}
	}
	s := testStore(t)
	s.initAuth("password")
	caller, _ := s.userCaller("admin")
	_, err := s.SaveWorkflow(caller, Workflow{Name: "No host fallback", Enabled: true, Entry: "code", MaxSteps: 10, Nodes: []WorkflowNode{{ID: "code", Name: "Code", Kind: "execution", RuleHandoff: executionTestRules("code", "done"), Execution: &WorkflowExecution{Mode: "code", Output: &ExecutionOutput{Source: "exit_code"}, Language: "python", Code: "print(1)"}}, {ID: "done", Name: "Done", Kind: "end"}}, Edges: []WorkflowEdge{}})
	if err == nil {
		t.Fatal("code without isolated environment saved")
	}
	if err = validateWorkflowExecution(&WorkflowExecution{Mode: "command", Output: &ExecutionOutput{Source: "exit_code"}, Command: "go test ./..."}); err != nil {
		t.Fatal("compiled repository language was restricted", err)
	}
}
