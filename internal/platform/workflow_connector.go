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
	if err = connectorWorkspace(current, r.WorkspacePath); err != nil {
		return frozen, err
	}
	if err = connectorWorkspace(frozen, r.WorkspacePath); err != nil {
		return frozen, err
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
		if _, err = tx.Exec(`INSERT INTO workflow_connector_actions(run_id,seq,request) VALUES(?,?,?)`, r.ID, step.Seq, raw); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`UPDATE workflow_steps SET status='running',updated=? WHERE run_id=? AND seq=?`, now(), r.ID, step.Seq); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
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
		e.mu.Lock()
		defer e.mu.Unlock()
		delete(e.connectorJobs, r.ID)
		if err := e.finishConnector(r, step, receipt, actionErr); err != nil {
			log.Printf("save connector receipt %s/%d: %v", r.ID, step.Seq, err)
		}
	}()
	return nil
}
func (e *WorkflowEngine) finishConnector(r WorkflowRun, step WorkflowStep, receipt ConnectorReceipt, actionErr error) error {
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
	route, summary := "next", "GitHub 资源已确认："+receipt.URL
	if actionErr == nil && receipt.Kind == "command" {
		if receipt.ExitCode == nil {
			actionErr = errors.New("command did not return an exit code")
		} else {
			summary = fmt.Sprintf("命令已结束 · 退出码 %d", *receipt.ExitCode)
			if *receipt.ExitCode != 0 {
				route = "failed"
				if r.Definition.target(step.NodeID, route) == "" {
					actionErr = fmt.Errorf("command exited %d; inspect receipt and return explicitly to retry", *receipt.ExitCode)
				}
			}
		}
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
		if _, err = tx.Exec(`UPDATE workflow_steps SET error=?,updated=? WHERE run_id=? AND seq=?`, message, now(), r.ID, step.Seq); err != nil {
			return err
		}
		if current.Status == "running" || current.Status == "waiting" {
			if _, err = tx.Exec(`UPDATE workflow_runs SET status='failed',error=?,updated=? WHERE id=?;UPDATE workflow_steps SET status='failed' WHERE run_id=? AND seq=?`, message, now(), r.ID, r.ID, step.Seq); err != nil {
				return err
			}
		}
	} else {
		result := NodeResult{Route: route, Summary: summary}
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
	if n > 0 && r.Connectors[r.Definition.node(step.NodeID).ConnectorID].Kind == "command" {
		return errors.New("this command was dispatched; inspect its receipt, stop and return explicitly to run it again")
	}
	return nil
}
