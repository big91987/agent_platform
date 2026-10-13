package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// WorkflowExecution invokes an existing capability without a reasoning turn.
// The graph carries arguments, while credentials remain in the tool registry.
type WorkflowExecution struct {
	Mode      string          `json:"mode"`
	ServerID  string          `json:"server_id,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

func validateWorkflowExecution(v *WorkflowExecution) error {
	if v == nil || v.Mode != "mcp" {
		return errors.New("select a supported execution mode")
	}
	if !toolServerName.MatchString(v.ServerID) || strings.TrimSpace(v.Tool) == "" {
		return errors.New("select an MCP server and tool")
	}
	var args map[string]json.RawMessage
	if len(v.Arguments) > 64000 || json.Unmarshal(v.Arguments, &args) != nil || args == nil {
		return errors.New("tool arguments must be a JSON object up to 64000 bytes")
	}
	return nil
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
		result, callErr := callMCPTool(callCtx, server.Connection, node.Execution.Tool, node.Execution.Arguments)
		receipt := ConnectorReceipt{Kind: "mcp", MCP: result, TimeoutSeconds: seconds}
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
