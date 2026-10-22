package platform

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// WorkflowExecution invokes an existing capability without a reasoning turn.
// The graph carries arguments, while credentials remain in the tool registry.
type ExecutionOutput struct {
	Source string `json:"source"`
	Path   string `json:"path,omitempty"`
}
type ExecutionRetry struct {
	MaxRetries      int `json:"max_retries"`
	IntervalSeconds int `json:"interval_seconds"`
}
type WorkflowExecution struct {
	Retry     *ExecutionRetry             `json:"retry,omitempty"`
	OnError   *WorkflowRuleFailure        `json:"on_error,omitempty"`
	Artifacts []string                    `json:"artifacts,omitempty"`
	Output    *ExecutionOutput            `json:"output,omitempty"`
	Language  string                      `json:"language,omitempty"`
	Code      string                      `json:"code,omitempty"`
	Command   string                      `json:"command,omitempty"`
	Mode      string                      `json:"mode"`
	ServerID  string                      `json:"server_id,omitempty"`
	Tool      string                      `json:"tool,omitempty"`
	Arguments json.RawMessage             `json:"arguments,omitempty"`
	Bindings  map[string]ExecutionBinding `json:"bindings,omitempty"`
	EnvRefs   map[string]string           `json:"env_refs,omitempty"`
}

// A binding uses existing task data or a node's latest prior step. It carries
// no independent identity, expression evaluator or ambient environment access.
type ExecutionBinding struct {
	Node string `json:"node,omitempty"`
	Path string `json:"path"`
}

func validateWorkflowExecution(v *WorkflowExecution) error {
	if v == nil {
		return errors.New("select a supported execution mode")
	}
	if v.Retry != nil && (v.Retry.MaxRetries < 0 || v.Retry.MaxRetries > 10 || v.Retry.IntervalSeconds < 1 || v.Retry.IntervalSeconds > 3600) {
		return errors.New("execution retries must be 0–10 with a 1–3600 second interval")
	}
	if len(v.Artifacts) > 0 {
		if v.Mode == "mcp" {
			return errors.New("MCP nodes do not retain workspace files")
		}
		if _, err := archivePaths(WorkflowStep{Result: &NodeResult{Artifacts: v.Artifacts}}); err != nil {
			return err
		}
		for _, p := range v.Artifacts {
			if !validArchivePath(p) {
				return errors.New("retained artifacts must be workspace-relative files")
			}
		}
	}
	if v.Output != nil {
		o := v.Output
		switch o.Source {
		case "none":
			if o.Path != "" {
				return errors.New("no output cannot specify a path")
			}
		case "file":
			if v.Mode == "mcp" || o.Path == "" || path.IsAbs(o.Path) || path.Clean(o.Path) != o.Path || o.Path == ".." || strings.HasPrefix(o.Path, "../") || hiddenArtifact(o.Path) || strings.ContainsRune(o.Path, 0) {
				return errors.New("result file must be a visible workspace-relative file")
			}
		case "stdout", "stderr", "exit_code":
			if v.Mode == "mcp" || o.Path != "" {
				return errors.New("stdout result requires code or command")
			}
		case "tool":
			if v.Mode != "mcp" || o.Path != "" {
				return errors.New("tool result requires MCP")
			}
		default:
			return errors.New("unsupported execution result source")
		}
	}
	if err := validateExecutionEnvRefs(v.EnvRefs); err != nil {
		return err
	}
	switch v.Mode {
	case "code", "command":
		if v.ServerID != "" || v.Tool != "" || len(v.Arguments) > 0 || len(v.Bindings) > 0 {
			return errors.New("code and commands cannot contain MCP settings")
		}
		text := v.Command
		if v.Mode == "code" {
			if (v.Language != "python" && v.Language != "typescript") || v.Command != "" {
				return errors.New("select Python or TypeScript code; use a project command for other languages")
			}
			text = v.Code
		} else if v.Language != "" || v.Code != "" {
			return errors.New("project commands cannot contain inline code settings")
		}
		if strings.TrimSpace(text) == "" || len(text) > 64000 || strings.ContainsRune(text, 0) {
			return errors.New("execution content is required (up to 64000 bytes, without NUL)")
		}
		return nil
	case "mcp":
		if v.Code != "" || v.Command != "" || v.Language != "" || len(v.EnvRefs) != 0 {
			return errors.New("MCP execution cannot contain code or commands")
		}
		if !toolServerName.MatchString(v.ServerID) || strings.TrimSpace(v.Tool) == "" {
			return errors.New("select an MCP server and tool")
		}
		var args map[string]json.RawMessage
		if len(v.Arguments) > 64000 || json.Unmarshal(v.Arguments, &args) != nil || args == nil {
			return errors.New("tool arguments must be a JSON object up to 64000 bytes")
		}
		return validateExecutionBindings(v.Bindings)
	default:
		return errors.New("select a supported execution mode")
	}
}

// Only administrator-configured references select backend values. Task input
// cannot introduce environment keys or override platform execution controls.
func executionEnvName(name string) bool {
	if !envName.MatchString(name) || len(name) > 128 {
		return false
	}
	for _, prefix := range []string{"CODEX_", "AGENT_PLATFORM_", "WORKFLOW_"} {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	switch name {
	case "HOME", "PATH", "LD_PRELOAD", "LD_LIBRARY_PATH", "NODE_OPTIONS", "PYTHONPATH", "PYTHONHOME", "BASH_ENV", "ENV":
		return false
	}
	return true
}
func validateExecutionEnvRefs(refs map[string]string) error {
	if len(refs) > 32 {
		return errors.New("execution environment supports up to 32 references")
	}
	for name, ref := range refs {
		if !executionEnvName(name) || !executionEnvName(ref) {
			return errors.New("execution environment references must use valid, non-reserved names")
		}
	}
	return nil
}
func executionEnvironment(refs map[string]string) (map[string]string, error) {
	if err := validateExecutionEnvRefs(refs); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(refs)+1)
	for name, ref := range refs {
		value := os.Getenv(ref)
		if value == "" || len(value) > 64000 || strings.ContainsRune(value, 0) {
			return nil, fmt.Errorf("execution environment reference %s is unavailable or invalid", ref)
		}
		out[name] = value
	}
	return out, nil
}

func executionTool(server RegisteredToolServer, name string) (*mcp.Tool, error) {
	for _, tool := range server.Tools {
		if tool != nil && tool.Name == name {
			return tool, nil
		}
	}
	return nil, errors.New("MCP tool is absent from the discovered inventory")
}

func validateExecutionTool(server RegisteredToolServer, v *WorkflowExecution) error {
	if err := validateWorkflowExecution(v); err != nil {
		return err
	}
	if !server.Enabled {
		return errors.New("MCP server is disabled")
	}
	tool, err := executionTool(server, v.Tool)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return errors.New("MCP tool input schema is invalid")
	}
	var schema jsonschema.Schema
	if err = json.Unmarshal(raw, &schema); err != nil {
		return errors.New("MCP tool input schema is invalid")
	}
	resolved, err := schema.Resolve(nil) // no remote schema loader or network access
	if err != nil {
		return errors.New("MCP tool input schema cannot be resolved locally")
	}
	if len(v.Bindings) > 0 {
		// Task values and earlier results do not exist when the graph is saved.
		// The complete, unchanged schema is enforced after resolution, before
		// recording an action intent or calling the external tool.
		return nil
	}
	var input any
	if err = json.Unmarshal(v.Arguments, &input); err != nil {
		return err
	}
	if err = resolved.Validate(input); err != nil {
		return errors.New("tool arguments do not match the discovered input schema")
	}
	return nil
}

func (e *WorkflowEngine) executionAccess(r WorkflowRun, node WorkflowNode) (RegisteredToolServer, error) {
	if err := validateWorkflowExecution(node.Execution); err != nil {
		return RegisteredToolServer{}, err
	}
	if node.Execution.Mode != "mcp" {
		_, err := e.store.sandboxFor(r)
		if err != nil {
			return RegisteredToolServer{}, err
		}
		if r.Environment.Status != "ready" {
			return RegisteredToolServer{}, errors.New("Run environment is not ready")
		}
		return RegisteredToolServer{}, nil
	}
	frozen, ok := r.ExecutionTools[node.ID]
	if !ok || node.Execution == nil {
		return frozen, errors.New("run has no execution tool snapshot")
	}
	current, err := e.store.ToolServer(node.Execution.ServerID)
	if err != nil {
		return frozen, err
	}
	if !current.Enabled {
		return frozen, errors.New("MCP server is disabled")
	}
	if !reflect.DeepEqual(current.Connection, frozen.Connection) {
		return frozen, errors.New("MCP connection changed since this Run started; review before starting a new Run")
	}
	oldTool, err := executionTool(frozen, node.Execution.Tool)
	if err != nil {
		return frozen, err
	}
	newTool, err := executionTool(current, node.Execution.Tool)
	if err != nil {
		return frozen, err
	}
	if !reflect.DeepEqual(oldTool.InputSchema, newTool.InputSchema) || !reflect.DeepEqual(oldTool.OutputSchema, newTool.OutputSchema) {
		return frozen, errors.New("MCP tool schema changed since this Run started")
	}
	return frozen, validateExecutionTool(frozen, node.Execution)
}

// The action intent is committed before starting external work. Both legacy
// connectors and execution nodes use the same per-step receipt and stop path.
func (e *WorkflowEngine) startExecution(ctx context.Context, r WorkflowRun, step WorkflowStep, node WorkflowNode) error {
	if step.RetryAt != "" {
		at, err := time.Parse(time.RFC3339Nano, step.RetryAt)
		if err != nil {
			return err
		}
		if time.Now().Before(at) {
			return nil
		}
	}
	if node.RuleHandoff == nil {
		return errors.New("execution node requires rule_handoff; update the workflow before starting a new run")
	}
	if !e.connectorQuiet(r.ID) || len(e.connectorJobs) >= 8 {
		return nil
	}
	if err := e.store.executionRunReady(r.ID); err != nil {
		return err
	}
	server, err := e.executionAccess(r, node)
	if err != nil {
		return err
	}
	runtimeEnv, err := executionEnvironment(node.Execution.EnvRefs)
	if err != nil {
		return err
	}
	request := *node.Execution
	if request.Mode == "mcp" {
		request.Arguments, err = resolveExecutionArguments(r, step, request)
		if err != nil {
			return err
		}
		request.Bindings = nil
		if err = validateExecutionTool(server, &request); err != nil {
			return err
		}
	}
	var count int
	if err = e.store.DB.QueryRow(`SELECT count(*) FROM workflow_connector_actions WHERE run_id=? AND seq=?`, r.ID, step.Seq).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return errors.New("execution was already dispatched; inspect its effects, then stop and return explicitly; replay is refused")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	if err = e.recordWorkflowAction(r, step, string(raw), false); err != nil {
		return err
	}
	seconds := node.ExecutionTimeoutSeconds
	if seconds == 0 {
		seconds = 60
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	e.connectorJobs[r.ID] = workflowConnectorJob{seq: step.Seq, cancel: cancel}
	e.connectorWG.Add(1)
	go func() {
		defer e.connectorWG.Done()
		defer cancel()
		var receipt ConnectorReceipt
		var callErr error
		if node.Execution.Mode == "mcp" {
			result, err := callMCPTool(callCtx, server.Connection, request.Tool, request.Arguments)
			receipt = ConnectorReceipt{Kind: "mcp", MCP: result, RetrySafe: mcpNotCalled(err)}
			callErr = err
			if err == nil && node.Execution.Output != nil && node.Execution.Output.Source == "tool" {
				if result == nil || result.StructuredContent == nil {
					receipt.ResultError = "MCP tool returned no structured result"
				} else {
					raw, err := json.Marshal(result.StructuredContent)
					if err != nil {
						receipt.ResultError = "MCP structured result cannot be encoded"
					} else {
						setExecutionResult(&receipt, raw, runtimeEnv)
					}
				}
			}
		} else {
			receipt, callErr = e.executeSandboxNode(callCtx, r, step, node, runtimeEnv)
		}
		receipt.TimeoutSeconds = seconds
		e.mu.Lock()
		defer e.mu.Unlock()
		delete(e.connectorJobs, r.ID)
		if err := e.finishWorkflowAction(r, step, receipt, callErr); err != nil {
			log.Printf("save execution receipt %s/%d: %v", r.ID, step.Seq, err)
		}
	}()
	return nil
}

func (e *WorkflowEngine) recordWorkflowAction(r WorkflowRun, step WorkflowStep, request string, recoverOnly bool) error {
	tx, err := e.store.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var seq int
	var status string
	if err = tx.QueryRow(`SELECT seq,status FROM workflow_runs WHERE id=?`, r.ID).Scan(&seq, &status); err != nil {
		return err
	}
	if seq != step.Seq || (status != "running" && status != "waiting") {
		return ErrConflict
	}
	if !recoverOnly {
		var execution WorkflowExecution
		_ = json.Unmarshal([]byte(request), &execution)
		quiet := 0
		if execution.Mode == "code" || execution.Mode == "command" {
			quiet = 1
		}
		if _, err = tx.Exec(`INSERT INTO workflow_connector_actions(run_id,seq,request,command_quiet) VALUES(?,?,?,?)`, r.ID, step.Seq, request, quiet); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`UPDATE workflow_steps SET status='running',updated=? WHERE run_id=? AND seq=?`, now(), r.ID, step.Seq); err != nil {
		return err
	}
	return tx.Commit()
}

func freezeExecutionTools(tx *sql.Tx, w Workflow) (map[string]RegisteredToolServer, error) {
	out := map[string]RegisteredToolServer{}
	for _, node := range w.Nodes {
		if node.Kind != "execution" {
			continue
		}
		if err := validateWorkflowExecution(node.Execution); err != nil {
			return nil, err
		}
		if node.Execution.Mode != "mcp" {
			continue
		}
		server, err := loadToolServer(tx, node.Execution.ServerID)
		if err != nil {
			return nil, err
		}
		if err = validateExecutionTool(server, node.Execution); err != nil {
			return nil, fmt.Errorf("node %s: %w", node.ID, err)
		}
		out[node.ID] = server
	}
	return out, nil
}

// The same Run environment executes scripts and project commands. No host
// fallback is available; the SDK's actual command ID/status remain the receipt.
func (e *WorkflowEngine) executeSandboxNode(ctx context.Context, r WorkflowRun, step WorkflowStep, node WorkflowNode, runtimeEnv map[string]string) (out ConnectorReceipt, callErr error) {
	out = ConnectorReceipt{Kind: node.Execution.Mode}
	service, err := e.store.sandboxFor(r)
	if err != nil {
		return out, err
	}
	raw, err := json.Marshal(executionInputValues(r, step))
	if err != nil {
		return out, err
	}
	if len(raw) > connectorArchiveLimit {
		return out, errors.New("execution input exceeds 8 MiB; reference large artifacts instead of embedding their contents")
	}
	seconds := node.ExecutionTimeoutSeconds
	if seconds == 0 {
		seconds = 60
	}
	home := filepath.Join(e.store.Dir, "workflow-processes", fmt.Sprintf("%s-%d", r.ID, step.Seq))
	if err = os.MkdirAll(home, 0700); err != nil {
		return out, err
	}
	archive, err := newConnectorArchive(home)
	if err != nil {
		return out, errors.New("cannot retain sandbox execution log before dispatch")
	}
	log := newBoundedConnectorLogValues(runtimeEnv)
	log.window.archive = archive
	defer func() {
		out.Log = log.finishArchive()
		out.Output = log.String()
		if callErr != nil && out.Log != nil && !errors.Is(callErr, context.Canceled) {
			var quiet bool
			if e.store.DB.QueryRow(`SELECT command_quiet FROM workflow_connector_actions WHERE run_id=? AND seq=?`, r.ID, step.Seq).Scan(&quiet) == nil {
				out.RetrySafe = quiet
			}
		}
		if out.Log == nil {
			callErr = errors.Join(callErr, errors.New("sandbox execution log could not be retained; inspect actual effects before explicitly returning"))
		}
	}()
	// Full input and source are private files, not large environment variables or
	// shell-quoted code. Even legal task inputs can exceed Linux's per-argument
	// limit. Run/sequence are platform-generated keys, never user paths.
	inputPath := fmt.Sprintf("/tmp/agent-platform-input-%s-%d.json", r.ID, step.Seq)
	if err = service.upload(ctx, r.Environment.ID, r.ID, inputPath, raw, 0600); err != nil {
		return out, err
	}
	source, extension, command := node.Execution.Command, "sh", "/bin/sh"
	if node.Execution.Mode == "code" {
		source = node.Execution.Code
		if node.Execution.Language == "python" {
			extension, command = "py", "python3 -c"
		} else {
			extension, command = "ts", "tsx -e"
		}
	}
	sourcePath := fmt.Sprintf("/tmp/agent-platform-code-%s-%d.%s", r.ID, step.Seq, extension)
	if err = service.upload(ctx, r.Environment.ID, r.ID, sourcePath, []byte(source), 0600); err != nil {
		return out, err
	}
	if node.Execution.Mode == "code" {
		command += " \"$(cat '" + sourcePath + "')\""
	} else {
		command += " '" + sourcePath + "'"
	}
	runtimeEnv["WORKFLOW_INPUT_FILE"] = inputPath
	request := opensandbox.RunCommandRequest{Cwd: r.WorkspacePath, Envs: runtimeEnv, Command: command, Timeout: int64(seconds) * 1000}
	var stdout bytes.Buffer
	stdoutOverflow := false
	result, commandErr := service.commandWithIntent(ctx, r.Environment.ID, r.ID, request, func(event opensandbox.StreamEvent) error {
		var output struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal([]byte(event.Data), &output) == nil && output.Type == "init" {
			if err := e.store.recordExecutionProcess(r.ID, step.Seq, output.Text, false); err != nil {
				return err
			}
		}
		if json.Unmarshal([]byte(event.Data), &output) == nil && node.Execution.Output != nil && (node.Execution.Output.Source == "stdout" || node.Execution.Output.Source == "stderr") && output.Type == node.Execution.Output.Source {
			if stdout.Len()+len(output.Text) > 1<<20 {
				stdoutOverflow = true
			} else if !stdoutOverflow {
				stdout.WriteString(output.Text)
			}
		}
		if json.Unmarshal([]byte(event.Data), &output) == nil && (output.Type == "stdout" || output.Type == "stderr") {
			_, err := log.Write([]byte(output.Text))
			return err
		}
		return json.NewEncoder(log).Encode(event)
	}, func() error { return e.store.recordExecutionDispatch(r.ID, step.Seq) })
	if result.Stopped && result.ID != "" {
		commandErr = errors.Join(commandErr, e.store.recordExecutionProcess(r.ID, step.Seq, result.ID, true))
	}
	out.Sandbox = &result
	if result.ExitCode != nil {
		exit := int(*result.ExitCode)
		out.ExitCode = &exit
	}
	if commandErr == nil && result.Stopped && out.ExitCode != nil && node.Execution.Output != nil {
		switch node.Execution.Output.Source {
		case "stdout", "stderr":
			if stdoutOverflow {
				out.ResultError = "selected stream exceeds 1 MiB"
			} else {
				setSelectedExecutionResult(node, &out, stdout.Bytes(), runtimeEnv)
			}
		case "file":
			_, raw, err := e.store.sandboxArtifacts(ctx, r, node.Execution.Output.Path)
			if err != nil {
				out.ResultError = "cannot read result file: " + err.Error()
			} else {
				setSelectedExecutionResult(node, &out, raw, runtimeEnv)
			}
		}
	}
	return out, commandErr
}

func setExecutionResult(out *ConnectorReceipt, raw []byte, env map[string]string) {
	out.Result, out.ResultError = nil, ""
	if len(raw) > 1<<20 {
		out.ResultError = "structured result exceeds 1 MiB"
		return
	}
	value, err := completionJSONValue(raw)
	if err != nil {
		out.ResultError = "structured result must be valid JSON with lossless numbers"
		return
	}
	// Never persist an explicitly injected secret in a structured result.
	var redact func(any) any
	redact = func(v any) any {
		switch x := v.(type) {
		case string:
			for _, secret := range env {
				if secret != "" {
					x = strings.ReplaceAll(x, secret, "[redacted]")
				}
			}
			return x
		case map[string]any:
			m := map[string]any{}
			for k, item := range x {
				m[redact(k).(string)] = redact(item)
			}
			return m
		case []any:
			for i, item := range x {
				x[i] = redact(item)
			}
			return x
		}
		return v
	}
	out.Result, _ = json.Marshal(redact(value))
}

func setSelectedExecutionResult(n WorkflowNode, out *ConnectorReceipt, raw []byte, env map[string]string) {
	if n.RuleHandoff != nil && n.RuleHandoff.SchemaScope == "value" {
		if len(raw) > 1<<20 || !utf8.Valid(raw) {
			out.ResultError = "selected data must be UTF-8 text up to 1 MiB"
			return
		}
		// JSON encoding here persists the original string, without interpreting its contents.
		encoded, _ := json.Marshal(string(raw))
		setExecutionResult(out, encoded, env)
		return
	}
	setExecutionResult(out, raw, env)
}

// Inactive editor forms are retained with the workflow, never executed. The
// selected execution still passes the full runtime configuration validation.
func validateExecutionDrafts(node WorkflowNode) error {
	if len(node.ExecutionDrafts) == 0 {
		return nil
	}
	if node.Kind != "execution" || len(node.ExecutionDrafts) > 256000 {
		return errors.New("execution drafts require an execution node and fit within 256000 bytes")
	}
	var drafts map[string]json.RawMessage
	if json.Unmarshal(node.ExecutionDrafts, &drafts) != nil || drafts == nil || len(drafts) > 3 {
		return errors.New("execution drafts must be an object of execution modes")
	}
	for mode, raw := range drafts {
		if mode != "code" && mode != "command" && mode != "mcp" {
			return errors.New("unsupported execution draft mode")
		}
		var value struct {
			Execution *WorkflowExecution `json:"execution"`
			Rules     *WorkflowRules     `json:"rules"`
		}
		if json.Unmarshal(raw, &value) != nil || value.Execution == nil || value.Execution.Mode != mode || value.Rules == nil {
			return errors.New("execution draft must retain its mode and rules")
		}
	}
	return nil
}
