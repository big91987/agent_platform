package platform

import (
	"context"
	"errors"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

// One export at a time belongs to the existing Run. Retain the last helper
// identity across restarts: an uncertain command must never disappear with the
// HTTP request or be mistaken for a completed write.
func (s *Store) initCommandOutputExports() error {
	_, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS workflow_log_exports(
 run_id TEXT PRIMARY KEY REFERENCES workflow_runs(id),command_id TEXT NOT NULL DEFAULT '',quiet INTEGER NOT NULL DEFAULT 1)`)
	return err
}

func (s *Store) logExportReady(run string) error {
	var pending int
	if err := s.DB.QueryRow(`SELECT count(*) FROM workflow_log_exports WHERE run_id=? AND quiet=0`, run).Scan(&pending); err != nil {
		return err
	}
	if pending != 0 {
		return errors.New("Log export process is unconfirmed. Reconcile the owned environment before continuing; no export was replayed.")
	}
	return nil
}

func (s *Store) workspaceExportAccess(id string, seq int) (WorkflowRun, error) {
	r, err := s.WorkflowRun(Caller{Admin: true}, id)
	if err != nil {
		return r, err
	}
	if (r.Status != "running" && r.Status != "waiting") || r.Seq != seq || len(r.Steps) == 0 {
		return r, ErrConflict
	}
	step := r.Steps[len(r.Steps)-1]
	if (step.Status != "running" && step.Status != "waiting") || step.Result != nil || r.Definition.node(step.NodeID).Kind != "agent" {
		return r, ErrConflict
	}
	owner, err := s.workflowCaller(r)
	if err != nil {
		return r, err
	}
	if _, err = s.Workflow(owner, r.WorkflowID); err != nil {
		return r, err
	}
	if !nodeAgentAllowed(owner, r.Definition.node(step.NodeID)) {
		return r, ErrForbidden
	}
	if r.Definition.Environment == "opensandbox" && (r.Environment == nil || r.Environment.Status != "ready") {
		return r, ErrConflict
	}
	c, err := s.Conversation(step.ConversationID)
	if err != nil {
		return r, err
	}
	if nativeSandboxMode(c) == "read-only" {
		return r, ErrForbidden
	}
	return r, nil
}

func (e *WorkflowEngine) exportCommandOutput(ctx context.Context, id string, seq, logSeq int) (commandOutputExport, error) {
	var out commandOutputExport
	if logSeq >= seq {
		return out, ErrConflict
	}
	err := e.withWorkspaceExport(ctx, id, seq, func(ctx context.Context, r WorkflowRun, dispatch logExportCommand) error {
		var err error
		out, err = e.store.exportCommandOutput(ctx, r, logSeq, dispatch)
		return err
	})
	return out, err
}

func (e *WorkflowEngine) withWorkspaceExport(ctx context.Context, id string, seq int, perform func(context.Context, WorkflowRun, logExportCommand) error) error {
	e.mu.Lock()
	r, err := e.store.workspaceExportAccess(id, seq)
	if err == nil && !e.connectorQuiet(id) {
		err = ErrConflict
	}
	if err == nil {
		err = e.store.executionRunReady(id)
	}
	if err != nil {
		e.mu.Unlock()
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	e.connectorJobs[id] = workflowConnectorJob{seq: seq, cancel: cancel}
	e.mu.Unlock()
	defer func() { cancel(); e.mu.Lock(); delete(e.connectorJobs, id); e.mu.Unlock() }()
	var dispatch logExportCommand
	if r.Definition.Environment == "opensandbox" {
		client, err := e.store.sandboxFor(r)
		if err != nil {
			return err
		}
		dispatch = func(ctx context.Context, in opensandbox.RunCommandRequest, onEvent opensandbox.EventHandler) (SandboxCommandResult, error) {
			var observed SandboxCommandResult
			result, commandErr := client.commandWithIntent(ctx, r.Environment.ID, id, in, func(event opensandbox.StreamEvent) error {
				if err := acceptSandboxCommandID(event.Data, &observed); err != nil {
					return err
				}
				if observed.ID != "" {
					if err := e.store.recordLogExportProcess(id, observed.ID, false); err != nil {
						return err
					}
				}
				return onEvent(event)
			}, func() error {
				// Serialize the dispatch boundary with Stop/Return/release. Once
				// committed, cancellation still reconciles this exact command.
				e.mu.Lock()
				defer e.mu.Unlock()
				if err := ctx.Err(); err != nil {
					return err
				}
				if _, err := e.store.workspaceExportAccess(id, seq); err != nil {
					return err
				}
				if err := e.store.logExportReady(id); err != nil {
					return err
				}
				_, err := e.store.DB.Exec(`INSERT INTO workflow_log_exports(run_id,quiet) VALUES(?,0) ON CONFLICT(run_id) DO UPDATE SET command_id='',quiet=0 WHERE quiet=1`, id)
				return err
			})
			if result.ID != "" {
				commandErr = errors.Join(commandErr, e.store.recordLogExportProcess(id, result.ID, result.Stopped))
			}
			return result, commandErr
		}
	}
	return perform(ctx, r, dispatch)
}

func (s *Store) recordLogExportProcess(run, id string, quiet bool) error {
	if !validSandboxCommandID(id) {
		return errors.New("invalid log export execution identity")
	}
	value := 0
	if quiet {
		value = 1
	}
	result, err := s.DB.Exec(`UPDATE workflow_log_exports SET command_id=?,quiet=max(quiet,?) WHERE run_id=? AND (command_id='' OR command_id=?)`, id, value, run, id)
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

func (s *Store) reconcileLogExport(ctx context.Context, client *OpenSandbox, run WorkflowRun) error {
	var pending int
	if err := s.DB.QueryRow(`SELECT count(*) FROM workflow_log_exports WHERE run_id=? AND quiet=0`, run.ID).Scan(&pending); err != nil {
		return err
	}
	if pending == 0 {
		return nil
	}
	var id string
	if err := s.DB.QueryRow(`SELECT command_id FROM workflow_log_exports WHERE run_id=?`, run.ID).Scan(&id); err != nil {
		return err
	}
	if id == "" {
		return errors.New("Log export launch identity is unknown; preserve the environment or explicitly release it. No replacement was sent.")
	}
	clientExec, err := client.execd(ctx, run.Environment.ID, run.ID)
	if err != nil {
		return err
	}
	status, err := clientExec.GetCommandStatus(ctx, id)
	if err != nil {
		return sandboxError("log export reconciliation", err)
	}
	if status.ID != id || status.Running {
		return errors.New("Log export is not independently confirmed stopped")
	}
	// A late reconciliation must not mark a newer dispatch (whose identity is
	// still empty) quiet. Match exactly the execution observed above.
	result, err := s.DB.Exec(`UPDATE workflow_log_exports SET quiet=1 WHERE run_id=? AND command_id=? AND quiet=0`, run.ID, id)
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
