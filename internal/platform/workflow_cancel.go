package platform

// Cancel seals a stopped/failed run only after its processes have exited. Stop
// remains the interrupt-and-resume operation; cancellation never rolls back files
// or external effects and does not manufacture a successful node result.
func (e *WorkflowEngine) Cancel(c Caller, id string, seq int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, err := e.store.WorkflowRun(c, id)
	if err != nil {
		return err
	}
	if r.Seq != seq {
		return ErrConflict
	}
	if r.Status == "cancelled" {
		return nil
	}
	if r.Status != "stopped" && r.Status != "failed" {
		return ErrConflict
	}
	step := r.Steps[len(r.Steps)-1]
	if !e.connectorQuiet(id) || !e.quiet(step.ConversationID) {
		return ErrConflict
	}
	tx, err := e.store.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, err := loadWorkflowRun(tx, id)
	if err != nil {
		return err
	}
	if !runAllowed(c, current) {
		return ErrForbidden
	}
	if current.Seq != seq || current.Status != r.Status {
		return ErrConflict
	}
	var active int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM conversations WHERE id IN (SELECT conversation_id FROM workflow_steps WHERE run_id=?) AND status IN ('running','stopping')`, id).Scan(&active); err != nil {
		return err
	}
	if active != 0 {
		return ErrConflict
	}
	stamp := now()
	if step.ConversationID != "" {
		if _, err = tx.Exec(`UPDATE conversations SET status='closed',updated=? WHERE id=?; UPDATE messages SET status='stopped' WHERE conversation_id=? AND role='user' AND status='queued'`, stamp, step.ConversationID, step.ConversationID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`UPDATE workflow_runs SET status='cancelled',updated=? WHERE id=?; UPDATE workflow_steps SET status='cancelled',token_hash='',updated=? WHERE run_id=? AND seq=?`, stamp, id, stamp, id, seq); err != nil {
		return err
	}
	return tx.Commit()
}
