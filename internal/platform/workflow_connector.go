package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

type workflowConnectorJob struct {
	seq    int
	cancel context.CancelFunc
}

func (e *WorkflowEngine) connectorQuiet(id string) bool {
	_, running := e.connectorJobs[id]
	return !running
}
func (e *WorkflowEngine) stopConnector(id string) bool {
	if job, ok := e.connectorJobs[id]; ok {
		job.cancel()
		return false
	}
	return true
}
func (e *WorkflowEngine) connectorAccess(c Caller, r WorkflowRun, node WorkflowNode) (Connector, error) {
	frozen, ok := r.Connectors[node.ConnectorID]
	if !ok {
		return frozen, errors.New("run has no connector snapshot")
	}
	current, err := e.store.Connector(c, node.ConnectorID)
	if err != nil {
		return frozen, err
	}
	if !current.Enabled {
		return frozen, errors.New("connector is disabled")
	}
	if r.Definition.Environment == "opensandbox" {
		if frozen.Kind != "github.issue_comment" || current.Kind != frozen.Kind || current.Repository != frozen.Repository || current.TokenEnv != frozen.TokenEnv {
			return frozen, errors.New("notification destination changed; review before starting a new Run")
		}
		return frozen, nil
	}
	if err = connectorWorkspace(current, r.WorkspacePath); err != nil {
		return frozen, err
	}
	if err = connectorWorkspace(frozen, r.WorkspacePath); err != nil {
		return frozen, err
	}
	// The command identity remains frozen. Time budget is an administrator's
	// operational policy, sampled once at dispatch (never extending an active
	// context). A timed-out command still requires explicit stop/return.
	if frozen.Kind == "command" {
		frozen.TimeoutSeconds = current.TimeoutSeconds
	}
	return frozen, nil
}
func (e *WorkflowEngine) startConnector(ctx context.Context, c Caller, r WorkflowRun, step WorkflowStep, node WorkflowNode) error {
	if !e.connectorQuiet(r.ID) || len(e.connectorJobs) >= 8 {
		return nil
	}
	v, err := e.connectorAccess(c, r, node)
	if err != nil {
		return err
	}
	var raw string
	err = e.store.DB.QueryRow(`SELECT request FROM workflow_connector_actions WHERE run_id=? AND seq=?`, r.ID, step.Seq).Scan(&raw)
	recoverOnly := err == nil
	var request connectorRequest
	if recoverOnly {
		if v.Kind == "command" {
			return errors.New("previous command may have run; inspect its effects, then stop and return to a new execution; replay is refused")
		}
		if err = json.Unmarshal([]byte(raw), &request); err != nil {
			return err
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		if v.Kind == "command" {
			if err = checkConnector(ctx, v); err != nil {
				return err
			}
		}
		request, err = prepareConnectorRequest(v, r, step)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(request)
		raw = string(b)
	} else {
		return err
	}
	if err = e.recordWorkflowAction(r, step, raw, recoverOnly); err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(v.TimeoutSeconds)*time.Second)
	e.connectorJobs[r.ID] = workflowConnectorJob{seq: step.Seq, cancel: cancel}
	e.connectorWG.Add(1)
	go func() {
		defer e.connectorWG.Done()
		defer cancel()
		var receipt ConnectorReceipt
		var actionErr error
		if v.Kind == "command" {
			receipt, actionErr = executeConnectorCommand(callCtx, v, r, step, request, e.store.Dir)
		} else {
			receipt, actionErr = executeGitHub(callCtx, v, request, recoverOnly)
		}
		receipt.TimeoutSeconds = v.TimeoutSeconds
		e.mu.Lock()
		defer e.mu.Unlock()
		delete(e.connectorJobs, r.ID)
		if err := e.finishWorkflowAction(r, step, receipt, actionErr); err != nil {
			log.Printf("save connector receipt %s/%d: %v", r.ID, step.Seq, err)
		}
	}()
	return nil
}
func (e *WorkflowEngine) finishWorkflowAction(r WorkflowRun, step WorkflowStep, receipt ConnectorReceipt, actionErr error) error {
	tx, err := e.store.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, err := loadWorkflowRun(tx, r.ID)
	if err != nil {
		return err
	}
	if current.Seq != step.Seq {
		return ErrConflict
	}
	node := current.Definition.node(step.NodeID)
	ruleBased := node.RuleHandoff != nil
	route, summary := "next", "GitHub 资源已确认："+receipt.URL
	if actionErr == nil && (receipt.Kind == "command" || receipt.Kind == "code") {
		if receipt.ExitCode == nil {
			actionErr = errors.New("command did not return an exit code")
		} else {
			summary = fmt.Sprintf("命令已结束 · 退出码 %d", *receipt.ExitCode)
			if *receipt.ExitCode != 0 {
				route = "failed"
				if !ruleBased && r.Definition.target(step.NodeID, route) == "" {
					actionErr = fmt.Errorf("command exited %d; inspect receipt and return explicitly to retry", *receipt.ExitCode)
				}
			}
		}
	}
	if actionErr == nil && receipt.Kind == "mcp" {
		summary = "工具调用已完成"
		if receipt.MCP == nil {
			actionErr = errors.New("MCP call returned no result")
		} else if receipt.MCP.IsError {
			route, summary = "failed", "工具返回失败"
			if !ruleBased && r.Definition.target(step.NodeID, route) == "" {
				actionErr = errors.New("MCP tool returned an error; inspect its receipt and return explicitly to retry")
			}
		}
	}
	if actionErr == nil && ruleBased {
		if len(receipt.Result) == 0 && receipt.ResultError == "" && (node.Execution.Output == nil || node.Execution.Output.Source == "none") {
			receipt.Result = json.RawMessage(`{}`)
		}
		route, summary, actionErr = routeExecutionRules(node, receipt)
	}
	message := ""
	if actionErr != nil {
		message = actionErr.Error()
	}
	raw, _ := json.Marshal(receipt)
	if _, err = tx.Exec(`UPDATE workflow_connector_actions SET receipt=?,error=? WHERE run_id=? AND seq=?`, string(raw), message, r.ID, step.Seq); err != nil {
		return err
	}
	// Receipt, result and failure are committed together. Stop/revocation retains
	// the actual outcome but cannot turn into permission to advance.
	if actionErr != nil {
		if _, err = tx.Exec(`UPDATE workflow_steps SET error=CASE WHEN error='' THEN ? ELSE error || char(10) || ? END,updated=? WHERE run_id=? AND seq=?`, message, message, now(), r.ID, step.Seq); err != nil {
			return err
		}
		if current.Status == "running" || current.Status == "waiting" {
			if _, err = tx.Exec(`UPDATE workflow_runs SET status='failed',error=?,updated=? WHERE id=?;UPDATE workflow_steps SET status='failed' WHERE run_id=? AND seq=?`, message, now(), r.ID, r.ID, step.Seq); err != nil {
				return err
			}
		}
	} else {
		result := NodeResult{Route: route, Summary: summary}
		if ruleBased {
			result.Inputs = map[string]any{"output": map[string]any{"result": receipt.Result, "exit_code": receipt.ExitCode}, "handoff_route": route}
		}
		if receipt.MCP != nil {
			if result.Inputs == nil {
				result.Inputs = map[string]any{}
			}
			result.Inputs["tool_result"] = receipt.MCP
			if ruleBased {
				result.Inputs["output"].(map[string]any)["is_error"] = receipt.MCP.IsError
			}
		}
		if receipt.URL != "" {
			result.Artifacts = []string{receipt.URL}
		}
		b, _ := json.Marshal(result)
		if _, err = tx.Exec(`UPDATE workflow_steps SET result=?,updated=? WHERE run_id=? AND seq=?`, string(b), now(), r.ID, step.Seq); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (e *WorkflowEngine) connectorResumeAllowed(r WorkflowRun, step WorkflowStep, message string) error {
	if !e.connectorQuiet(r.ID) {
		return ErrConflict
	}
	if strings.TrimSpace(message) == "" {
		return errors.New("describe the recovery; GitHub reconciliation is read-only and commands are not replayed")
	}
	var n int
	if err := e.store.DB.QueryRow(`SELECT count(*) FROM workflow_connector_actions WHERE run_id=? AND seq=?`, r.ID, step.Seq).Scan(&n); err != nil {
		return err
	}
	if n > 0 && (r.Definition.node(step.NodeID).Kind == "execution" || r.Connectors[r.Definition.node(step.NodeID).ConnectorID].Kind == "command") {
		return errors.New("this action was dispatched; inspect its receipt, stop and return explicitly to run it again")
	}
	return nil
}
