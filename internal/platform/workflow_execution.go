package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// WorkflowExecution invokes an existing capability without a reasoning turn.
// The graph carries arguments, while credentials remain in the tool registry.
type WorkflowExecution struct {
	Language  string          `json:"language,omitempty"`
	Code      string          `json:"code,omitempty"`
	Command   string          `json:"command,omitempty"`
	Mode      string          `json:"mode"`
	ServerID  string          `json:"server_id,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

func validateWorkflowExecution(v *WorkflowExecution) error {
	if v == nil {
		return errors.New("select a supported execution mode")
	}
	switch v.Mode {
	case "code", "command":
		if v.ServerID != "" || v.Tool != "" || len(v.Arguments) > 0 {
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
		if v.Code != "" || v.Command != "" || v.Language != "" {
			return errors.New("MCP execution cannot contain code or commands")
		}
		if !toolServerName.MatchString(v.ServerID) || strings.TrimSpace(v.Tool) == "" {
			return errors.New("select an MCP server and tool")
		}
		var args map[string]json.RawMessage
		if len(v.Arguments) > 64000 || json.Unmarshal(v.Arguments, &args) != nil || args == nil {
			return errors.New("tool arguments must be a JSON object up to 64000 bytes")
		}
		return nil
	default:
		return errors.New("select a supported execution mode")
	}
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
	if !e.connectorQuiet(r.ID) || len(e.connectorJobs) >= 8 {
		return nil
	}
	server, err := e.executionAccess(r, node)
	if err != nil {
		return err
	}
	var count int
	if err = e.store.DB.QueryRow(`SELECT count(*) FROM workflow_connector_actions WHERE run_id=? AND seq=?`, r.ID, step.Seq).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return errors.New("execution was already dispatched; inspect its effects, then stop and return explicitly; replay is refused")
	}
	raw, err := json.Marshal(node.Execution)
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
			result, err := callMCPTool(callCtx, server.Connection, node.Execution.Tool, node.Execution.Arguments)
			receipt = ConnectorReceipt{Kind: "mcp", MCP: result}
			callErr = err
		} else {
			receipt, callErr = e.executeSandboxNode(callCtx, r, step, node)
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
		if _, err = tx.Exec(`INSERT INTO workflow_connector_actions(run_id,seq,request) VALUES(?,?,?)`, r.ID, step.Seq, request); err != nil {
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
func (e *WorkflowEngine) executeSandboxNode(ctx context.Context, r WorkflowRun, step WorkflowStep, node WorkflowNode) (out ConnectorReceipt, callErr error) {
	out = ConnectorReceipt{Kind: node.Execution.Mode}
	service, err := e.store.sandboxFor(r)
	if err != nil {
		return out, err
	}
	values := map[string]any{"input": r.Input, "parameters": r.Parameters, "previous_results": []any{}}
	previous := []any{}
	for _, old := range r.Steps {
		if old.Seq < step.Seq {
			previous = append(previous, map[string]any{"node": old.NodeID, "result": old.Result, "execution_receipt": old.actionReceipt(), "error": old.Error})
		}
	}
	values["previous_results"] = previous
	raw, err := json.Marshal(values)
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
	log := newBoundedConnectorLog(Connector{})
	log.window.archive = archive
	defer func() {
		out.Log = log.finishArchive()
		out.Output = log.String()
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
	request := opensandbox.RunCommandRequest{Cwd: r.WorkspacePath, Envs: map[string]string{"WORKFLOW_INPUT_FILE": inputPath}, Command: command, Timeout: int64(seconds) * 1000}
	result, commandErr := service.command(ctx, r.Environment.ID, r.ID, request, func(event opensandbox.StreamEvent) error { return json.NewEncoder(log).Encode(event) })
	out.Sandbox = &result
	if result.ExitCode != nil {
		exit := int(*result.ExitCode)
		out.ExitCode = &exit
	}
	return out, commandErr
}
