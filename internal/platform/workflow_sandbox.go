package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"time"
)

// RunEnvironment owns the one persistent environment association for a Run.
// Credentials stay in the trusted platform service configuration.
type EnvironmentEvent struct {
	Status  string `json:"status"`
	ID      string `json:"id,omitempty"`
	Error   string `json:"error,omitempty"`
	Created string `json:"created_at"`
}

type RunEnvironment struct {
	History        []EnvironmentEvent `json:"history"`
	ID             string             `json:"id,omitempty"`
	Status         string             `json:"status"`
	Image          string             `json:"image"`
	TTL            int                `json:"ttl_seconds"`
	Error          string             `json:"error,omitempty"`
	Updated        string             `json:"updated_at"`
	ConnectionHash string             `json:"-"`
}

// ConfigureSandbox is a startup setting, not task input or an Agent permission.
func (s *Store) ConfigureSandbox(config, image string, ttl int) error {
	if s.sandbox != nil {
		return errors.New("OpenSandbox is already configured")
	}
	client, err := loadOpenSandboxConfig(config, image, ttl)
	if err != nil {
		return err
	}
	s.sandbox = client
	return nil
}

func (s *Store) initRunEnvironments() error {
	_, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS workflow_run_environments(
 run_id TEXT PRIMARY KEY REFERENCES workflow_runs(id),sandbox_id TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL,image TEXT NOT NULL,ttl INTEGER NOT NULL,connection_hash TEXT NOT NULL,
 error TEXT NOT NULL DEFAULT '',updated TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS workflow_environment_events(run_id TEXT NOT NULL REFERENCES workflow_runs(id),status TEXT NOT NULL,sandbox_id TEXT NOT NULL,error TEXT NOT NULL,created TEXT NOT NULL);
 INSERT INTO workflow_environment_events(run_id,status,sandbox_id,error,created) SELECT run_id,'unconfirmed',sandbox_id,'Platform restarted during environment creation; reconcile before continuing.',updated FROM workflow_run_environments WHERE status='creating';
 INSERT INTO workflow_environment_events(run_id,status,sandbox_id,error,created) SELECT run_id,'release_unconfirmed',sandbox_id,'Platform restarted during environment release; reconcile before continuing.',updated FROM workflow_run_environments WHERE status='releasing';
 UPDATE workflow_run_environments SET status='unconfirmed',error='Platform restarted during environment creation; reconcile before continuing.' WHERE status='creating';
 UPDATE workflow_run_environments SET status='release_unconfirmed',error='Platform restarted during environment release; reconcile before continuing.' WHERE status='releasing';`)
	if err != nil {
		return err
	}
	return s.initSandboxNativeProcesses()
}
func loadRunEnvironment(tx *sql.Tx, id string) (*RunEnvironment, error) {
	var env RunEnvironment
	err := tx.QueryRow(`SELECT sandbox_id,status,image,ttl,connection_hash,error,updated FROM workflow_run_environments WHERE run_id=?`, id).Scan(&env.ID, &env.Status, &env.Image, &env.TTL, &env.ConnectionHash, &env.Error, &env.Updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(`SELECT status,sandbox_id,error,created FROM workflow_environment_events WHERE run_id=? ORDER BY rowid`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	env.History = []EnvironmentEvent{}
	for rows.Next() {
		var event EnvironmentEvent
		if err = rows.Scan(&event.Status, &event.ID, &event.Error, &event.Created); err != nil {
			return nil, err
		}
		env.History = append(env.History, event)
	}
	return &env, rows.Err()
}
func (s *Store) updateRunEnvironment(run, oldState, id, state, reason string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stamp := now()
	result, err := tx.Exec(`UPDATE workflow_run_environments SET sandbox_id=?,status=?,error=?,updated=? WHERE run_id=? AND status=?`, id, state, reason, stamp, run, oldState)
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
	if oldState != state || reason != "" {
		if _, err = tx.Exec(`INSERT INTO workflow_environment_events(run_id,status,sandbox_id,error,created) VALUES(?,?,?,?,?)`, run, state, id, reason, stamp); err != nil {
			return err
		}
	}
	if state == "released" {
		if err = confirmSandboxNativeAbsence(tx, run, stamp); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) sandboxFor(r WorkflowRun) (*OpenSandbox, error) {
	if r.Definition.Environment != "opensandbox" || r.Environment == nil {
		return nil, errors.New("Run has no OpenSandbox environment")
	}
	if s.sandbox == nil {
		return nil, errors.New("OpenSandbox service is not configured")
	}
	if s.sandbox.connectionHash != r.Environment.ConnectionHash {
		return nil, errors.New("OpenSandbox service configuration changed; restore the original connection before continuing")
	}
	// New runtime defaults do not change environments promised to existing Runs.
	client := *s.sandbox
	client.image, client.ttl = r.Environment.Image, r.Environment.TTL
	return &client, nil
}
func (s *OpenSandbox) waitReady(ctx context.Context, id, run string) error {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	for {
		info, err := s.owned(ctx, id, run)
		if err != nil {
			return err
		}
		switch info.Status.State {
		case opensandbox.StateRunning:
			execd, err := s.execd(ctx, id, run)
			if err != nil {
				return err
			}
			return sandboxError("execution readiness", execd.Ping(ctx))
		case opensandbox.StateFailed, opensandbox.StateTerminated:
			return errors.New("sandbox failed or expired; its environment was not recreated")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}
func (e *WorkflowEngine) ensureSandbox(ctx context.Context, r WorkflowRun) (bool, error) {
	client, err := e.store.sandboxFor(r)
	if err != nil {
		return false, err
	}
	switch r.Environment.Status {
	case "ready":
		if err := e.store.nativeRunReady(r.ID); err != nil {
			return false, err
		}
		return true, nil
	case "pending":
		if !e.connectorQuiet(r.ID) || len(e.connectorJobs) >= 8 {
			return false, nil
		}
		if err := e.store.updateRunEnvironment(r.ID, "pending", "", "creating", ""); err != nil {
			return false, err
		}
		task, cancel := context.WithTimeout(ctx, 120*time.Second)
		e.connectorJobs[r.ID] = workflowConnectorJob{seq: r.Seq, cancel: cancel}
		e.connectorWG.Add(1)
		go func() {
			defer e.connectorWG.Done()
			defer cancel()
			info, createErr := client.create(task, r.ID)
			id := ""
			if info != nil {
				id = info.ID
			}
			if createErr == nil {
				createErr = client.waitReady(task, id, r.ID)
			}
			state, reason := "ready", ""
			if createErr != nil {
				state, reason = "unconfirmed", createErr.Error()
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			delete(e.connectorJobs, r.ID)
			saveErr := e.store.updateRunEnvironment(r.ID, "creating", id, state, reason)
			if saveErr != nil {
				e.fail(r, saveErr)
				return
			}
			if createErr != nil {
				current, readErr := e.store.WorkflowRun(Caller{Admin: true}, r.ID)
				if readErr == nil && current.Status != "stopping" {
					e.fail(current, createErr)
				}
			}
		}()
		return false, nil
	case "creating":
		if e.connectorQuiet(r.ID) {
			return false, errors.New("sandbox creation result is unconfirmed; reconcile before continuing")
		}
		return false, nil
	default:
		return false, fmt.Errorf("sandbox is %s; inspect and reconcile before continuing", r.Environment.Status)
	}
}
func (e *WorkflowEngine) ReconcileSandbox(ctx context.Context, c Caller, id string) error {
	r, err := e.store.WorkflowRun(c, id)
	if err != nil {
		return err
	}
	if r.Owner != c.UserID && !c.Admin {
		return ErrForbidden
	}
	e.mu.Lock()
	quiet := e.connectorQuiet(id) && e.quiet(r.Steps[len(r.Steps)-1].ConversationID)
	e.mu.Unlock()
	if !quiet || (r.Status != "stopped" && r.Status != "failed" && r.Status != "completed" && r.Status != "cancelled") {
		return ErrConflict
	}
	client, err := e.store.sandboxFor(r)
	if err != nil {
		return err
	}
	if r.Environment.Status == "released" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var info *opensandbox.SandboxInfo
	if r.Environment.ID != "" {
		info, err = client.lifecycle.GetSandbox(ctx, r.Environment.ID)
		var api *opensandbox.APIError
		if errors.As(err, &api) && api.StatusCode == 404 && r.Environment.Status == "release_unconfirmed" {
			return e.store.updateRunEnvironment(r.ID, r.Environment.Status, r.Environment.ID, "released", "")
		}
		if err != nil {
			return sandboxError("environment reconciliation", err)
		}
		if info.ID != r.Environment.ID || info.Metadata["agent-platform-run"] != r.ID {
			return errors.New("sandbox does not belong to this Run")
		}

	} else {
		info, err = client.find(ctx, r.ID)
	}
	if err != nil {
		return err
	}
	if info == nil {
		return errors.New("no environment was found; creation remains unconfirmed and was not replayed")
	}
	if err = client.waitReady(ctx, info.ID, r.ID); err != nil {
		return err
	}
	if err = e.store.reconcileSandboxNativeProcesses(ctx, client, r); err != nil {
		return err
	}
	return e.store.updateRunEnvironment(r.ID, r.Environment.Status, info.ID, "ready", "")
}
func (e *WorkflowEngine) ReleaseSandbox(ctx context.Context, c Caller, id string) error {
	e.mu.Lock()
	r, err := e.store.WorkflowRun(c, id)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	if r.Owner != c.UserID && !c.Admin {
		e.mu.Unlock()
		return ErrForbidden
	}
	if !e.connectorQuiet(id) || !e.quiet(r.Steps[len(r.Steps)-1].ConversationID) || (r.Status != "stopped" && r.Status != "failed" && r.Status != "completed" && r.Status != "cancelled") {
		e.mu.Unlock()
		return ErrConflict
	}
	client, err := e.store.sandboxFor(r)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	if r.Environment.Status == "released" {
		e.mu.Unlock()
		return nil
	}
	if r.Environment.Status == "pending" {
		// No create intent was committed, so there is no external effect to replay.
		err := e.store.updateRunEnvironment(id, "pending", "", "released", "")
		e.mu.Unlock()
		return err
	}
	if r.Environment.Status != "ready" {
		e.mu.Unlock()
		return errors.New("reconcile the environment before releasing; uncertain release was not replayed")
	}
	if err := e.store.updateRunEnvironment(r.ID, "ready", r.Environment.ID, "releasing", ""); err != nil {
		e.mu.Unlock()
		return err
	}
	e.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	releaseErr := client.remove(ctx, r.Environment.ID, r.ID)
	state, reason := "released", ""
	if releaseErr != nil {
		state, reason = "release_unconfirmed", releaseErr.Error()
	}
	err = e.store.updateRunEnvironment(id, "releasing", r.Environment.ID, state, reason)
	return errors.Join(releaseErr, err)
}
