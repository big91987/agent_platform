package platform

import (
	"context"
	"errors"
	"time"
)

// Retain process observations under the existing action key, without rewriting
// the original command receipt or assigning another task identity.
func (s *Store) initExecutionProcesses() error {
	rows, err := s.DB.Query(`PRAGMA table_info(workflow_connector_actions)`)
	if err != nil {
		return err
	}
	fields := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, kind string
		var def any
		if err = rows.Scan(&cid, &name, &kind, &notnull, &def, &pk); err != nil {
			rows.Close()
			return err
		}
		fields[name] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, field := range []struct{ name, definition string }{{"command_id", "TEXT NOT NULL DEFAULT ''"}, {"command_quiet", "INTEGER NOT NULL DEFAULT 0"}} {
		if !fields[field.name] {
			if _, err = s.DB.Exec(`ALTER TABLE workflow_connector_actions ADD COLUMN ` + field.name + ` ` + field.definition); err != nil {
				return err
			}
		}
	}
	// Only original completed receipts prove termination during upgrade.
	_, err = s.DB.Exec(`UPDATE workflow_connector_actions SET command_id=json_extract(receipt,'$.sandbox.id'),command_quiet=1 WHERE command_id='' AND CASE WHEN json_valid(receipt) THEN json_extract(receipt,'$.sandbox.stopped')=1 AND json_extract(receipt,'$.sandbox.id') IS NOT NULL ELSE 0 END`)
	return err
}

// Preparation has no command process. Persist the dispatch boundary before
// calling the SDK, so a restart never reclassifies an uncertain POST as unsent.
func (s *Store) recordExecutionDispatch(run string, seq int) error {
	result, err := s.DB.Exec(`UPDATE workflow_connector_actions SET command_quiet=0 WHERE run_id=? AND seq=? AND command_id='' AND command_quiet=1`, run, seq)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
func (s *Store) recordExecutionProcess(run string, seq int, id string, stopped bool) error {
	if !validSandboxCommandID(id) {
		return errors.New("invalid execution identity")
	}
	value := 0
	if stopped {
		value = 1
	}
	result, err := s.DB.Exec(`UPDATE workflow_connector_actions SET command_id=?,command_quiet=max(command_quiet,?) WHERE run_id=? AND seq=? AND (command_id='' OR command_id=?)`, id, value, run, seq, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
func (s *Store) executionRunReady(run string) error {
	if err := s.logExportReady(run); err != nil {
		return err
	}
	var count int
	if err := s.DB.QueryRow(`SELECT count(*) FROM workflow_connector_actions WHERE run_id=? AND command_quiet=0 AND json_extract(request,'$.mode') IN ('code','command')`, run).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return errors.New("Execution process outcome is unconfirmed. Reconcile its exact command or explicitly release its environment before continuing; no work was replayed.")
	}
	return nil
}
func (s *Store) reconcileExecutionProcesses(ctx context.Context, client *OpenSandbox, run WorkflowRun) error {
	rows, err := s.DB.Query(`SELECT seq,command_id FROM workflow_connector_actions WHERE run_id=? AND command_quiet=0 AND json_extract(request,'$.mode') IN ('code','command')`, run.ID)
	if err != nil {
		return err
	}
	type command struct {
		seq int
		id  string
	}
	pending := []command{}
	for rows.Next() {
		var cmd command
		if err = rows.Scan(&cmd.seq, &cmd.id); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, cmd)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, cmd := range pending {
		if cmd.id == "" {
			return errors.New("Command launch identity is unknown; preserve and inspect the environment or explicitly release it. No replacement was sent.")
		}
		execd, err := client.execd(ctx, run.Environment.ID, run.ID)
		if err != nil {
			return err
		}
		status, err := execd.GetCommandStatus(ctx, cmd.id)
		if err != nil {
			return sandboxError("execution reconciliation", err)
		}
		if status.ID != cmd.id || status.Running {
			return errors.New("Original execution is not independently confirmed stopped; no interrupt or replacement was sent.")
		}
		if err = s.recordExecutionProcess(run.ID, cmd.seq, cmd.id, true); err != nil {
			return err
		}
	}
	return nil
}
