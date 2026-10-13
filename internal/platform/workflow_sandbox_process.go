package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// The input message already identifies one native execution. No additional
// task ID is issued, and native credentials never enter this lifecycle record.
type SandboxNativeProcess struct {
	MessageID      int64  `json:"message_id"`
	ConversationID string `json:"conversation_id"`
	ID             string `json:"id,omitempty"`
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
	Updated        string `json:"updated_at"`
}

type nativeExecutionUnconfirmed struct{ reason string }

func (e *nativeExecutionUnconfirmed) Error() string { return e.reason }

func (s *Store) initSandboxNativeProcesses() error {
	_, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS workflow_native_processes(
 message_id INTEGER PRIMARY KEY REFERENCES messages(id),conversation_id TEXT NOT NULL REFERENCES conversations(id),
 run_id TEXT NOT NULL REFERENCES workflow_runs(id),sandbox_id TEXT NOT NULL,command_id TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL,error TEXT NOT NULL DEFAULT '',updated TEXT NOT NULL);
 UPDATE workflow_native_processes SET status='unconfirmed',error='Platform restarted while native execution was active. Reconcile its exact process before continuing.' WHERE status IN ('launching','running','stopping');`)
	return err
}

func loadSandboxNativeProcesses(tx *sql.Tx, run string) ([]SandboxNativeProcess, error) {
	rows, err := tx.Query(`SELECT message_id,conversation_id,command_id,status,error,updated FROM workflow_native_processes WHERE run_id=? ORDER BY message_id`, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SandboxNativeProcess
	for rows.Next() {
		var process SandboxNativeProcess
		if err := rows.Scan(&process.MessageID, &process.ConversationID, &process.ID, &process.Status, &process.Error, &process.Updated); err != nil {
			return nil, err
		}
		out = append(out, process)
	}
	return out, rows.Err()
}

func (s *Store) nativeRunReady(run string) error {
	var unresolved int
	if err := s.DB.QueryRow(`SELECT count(*) FROM workflow_native_processes WHERE run_id=? AND status='unconfirmed'`, run).Scan(&unresolved); err != nil {
		return err
	}
	if unresolved != 0 {
		return &nativeExecutionUnconfirmed{"Native process outcome is unconfirmed. Reconcile the owned environment before continuing; no execution was replayed."}
	}
	return nil
}

func (s *Store) beginSandboxNative(run WorkflowRun, c Conversation, m Message) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if run.Environment == nil || run.Environment.Status != "ready" || m.ConversationID != c.ID {
		return ErrConflict
	}
	var valid int
	err = tx.QueryRow(`SELECT count(*) FROM messages m JOIN conversations c ON c.id=m.conversation_id JOIN workflow_steps ws ON ws.conversation_id=c.id JOIN workflow_run_environments re ON re.run_id=ws.run_id WHERE m.id=? AND c.id=? AND ws.run_id=? AND re.sandbox_id=? AND re.status='ready' AND m.role='user' AND m.status='running' AND c.status='running'`, m.ID, c.ID, run.ID, run.Environment.ID).Scan(&valid)
	if err != nil {
		return err
	}
	if valid != 1 {
		return ErrConflict
	}
	var active int
	if err = tx.QueryRow(`SELECT count(*) FROM workflow_native_processes WHERE run_id=? AND status!='stopped'`, run.ID).Scan(&active); err != nil {
		return err
	}
	if active != 0 {
		return &nativeExecutionUnconfirmed{"Another native process still owns this Run; no additional launch was sent."}
	}
	stamp := now()
	if _, err = tx.Exec(`INSERT INTO workflow_native_processes(message_id,conversation_id,run_id,sandbox_id,status,updated) VALUES(?,?,?,?,'launching',?)`, m.ID, c.ID, run.ID, run.Environment.ID, stamp); err != nil {
		return ErrConflict
	}
	if err = recordSandboxNativeEvent(tx, c.ID, m.ID, "launching", "", "", stamp); err != nil {
		return err
	}
	return tx.Commit()
}

func recordSandboxNativeEvent(tx *sql.Tx, c string, m int64, status, id, reason, stamp string) error {
	kind := "platform.sandbox.native." + status
	raw, _ := json.Marshal(map[string]any{"type": kind, "message_id": m, "command_id": id, "error": reason})
	_, err := tx.Exec(`INSERT INTO events(conversation_id,message_id,type,raw,created) VALUES(?,?,?,?,?)`, c, m, kind, string(raw), stamp)
	return err
}

func (s *Store) updateSandboxNative(m int64, old, id, status, reason string) error {
	if id != "" && !validSandboxCommandID(id) {
		return errors.New("invalid native execution identity")
	}
	allowed := (old == "launching" && (status == "running" || status == "stopped" || status == "unconfirmed")) || (old == "running" && (status == "stopping" || status == "stopped" || status == "unconfirmed")) || (old == "stopping" && (status == "stopped" || status == "unconfirmed")) || (old == "unconfirmed" && status == "stopped")
	if !allowed {
		return ErrConflict
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var c, actualID, actualState string
	if err = tx.QueryRow(`SELECT conversation_id,command_id,status FROM workflow_native_processes WHERE message_id=?`, m).Scan(&c, &actualID, &actualState); err != nil {
		return err
	}
	if actualState != old || (actualID != "" && actualID != id) {
		return ErrConflict
	}
	if status == "running" && id == "" {
		return ErrConflict
	}
	stamp := now()
	if _, err = tx.Exec(`UPDATE workflow_native_processes SET command_id=?,status=?,error=?,updated=? WHERE message_id=?`, id, status, reason, stamp, m); err != nil {
		return err
	}
	if err = recordSandboxNativeEvent(tx, c, m, status, id, reason, stamp); err != nil {
		return err
	}
	return tx.Commit()
}

// A read-only native reconciliation never interrupts or restarts a process.
// A missing execution history does not prove that a process is absent.
func (s *Store) reconcileSandboxNativeProcesses(ctx context.Context, client *OpenSandbox, run WorkflowRun) error {
	for _, process := range run.NativeProcesses {
		if process.Status != "unconfirmed" {
			continue
		}
		if process.ID == "" {
			return &nativeExecutionUnconfirmed{"Native launch response was lost. Its process identity cannot be recovered; inspect the preserved environment or explicitly release it."}
		}
		execd, err := client.execd(ctx, run.Environment.ID, run.ID)
		if err != nil {
			return err
		}
		status, err := execd.GetCommandStatus(ctx, process.ID)
		if err != nil {
			return sandboxError("native process reconciliation", err)
		}
		if status.ID != process.ID {
			return errors.New("native reconciliation returned another execution identity")
		}
		if status.Running {
			return &nativeExecutionUnconfirmed{"The original native process is still running. No interrupt or replacement was sent; preserve its environment for inspection or explicitly release it."}
		}
		if err = s.updateSandboxNative(process.MessageID, "unconfirmed", process.ID, "stopped", "Independent execution status confirmed the original process stopped."); err != nil {
			return err
		}
	}
	return nil
}

// Called in the same transaction as confirmed environment absence. This proves
// termination by destruction, not a successful individual native exit status.
func confirmSandboxNativeAbsence(tx *sql.Tx, run, stamp string) error {
	processes, err := loadSandboxNativeProcesses(tx, run)
	if err != nil {
		return err
	}
	for _, p := range processes {
		if p.Status == "stopped" {
			continue
		}
		reason := "Owned environment absence confirmed; individual native exit status is unavailable."
		if _, err = tx.Exec(`UPDATE workflow_native_processes SET status='stopped',error=?,updated=? WHERE message_id=?`, reason, stamp, p.MessageID); err != nil {
			return err
		}
		if err = recordSandboxNativeEvent(tx, p.ConversationID, p.MessageID, "stopped", p.ID, reason, stamp); err != nil {
			return err
		}
	}
	return nil
}
