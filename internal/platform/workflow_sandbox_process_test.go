package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func nativeProcessFixture(t *testing.T, s *Store) (Caller, WorkflowRun, Conversation, Message) {
	t.Helper()
	caller, graph, _, _ := sandboxWorkflowFixture(t, s, false, false)
	run, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: graph.ID, Input: "native ownership", RequestID: newID()})
	if err != nil {
		t.Fatal(err)
	}
	run = waitSandboxRun(t, NewWorkflowEngine(s, nil, "http://platform.local"), caller, run.ID, "ready")
	config := Agent{Name: "Role", Executor: "codex", Model: "gpt-6.1-sol", Sandbox: "workspace-write", Enabled: true}
	_, err = s.submitConfigured(caller, Input{environmentRun: run.ID, UserID: run.Owner, Message: "native ownership", WorkspacePath: run.WorkspacePath, RequestID: newID()}, func(tx *sql.Tx, _ *Agent, id string) error {
		_, e := tx.Exec(`UPDATE workflow_steps SET conversation_id=? WHERE run_id=? AND seq=1`, id, run.ID)
		return e
	}, &config)
	if err != nil {
		t.Fatal(err)
	}
	c, m, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	return caller, run, c, m
}

func TestSandboxNativeLaunchIntentSurvivesRestartAndRefusesReplay(t *testing.T) {
	s := testStore(t)
	caller, run, c, m := nativeProcessFixture(t, s)
	if err := s.beginSandboxNative(run, c, m); err != nil {
		t.Fatal(err)
	}
	if err := s.beginSandboxNative(run, c, m); err == nil {
		t.Fatal("native launch intent replayed")
	}
	if err := s.updateSandboxNative(m.ID, "launching", "native-owned", "running", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.updateSandboxNative(m.ID, "running", "different-id", "stopped", ""); err == nil {
		t.Fatal("native process identity replaced")
	}
	root := s.Dir
	s.Close()
	reopened, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	actual, err := reopened.WorkflowRun(caller, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(actual.NativeProcesses) != 1 || actual.NativeProcesses[0].Status != "unconfirmed" || actual.NativeProcesses[0].ID != "native-owned" {
		t.Fatal("restart lost uncertain process", actual.NativeProcesses)
	}
	if err = reopened.Continue(c.ID); err == nil {
		t.Fatal("native input resumed before process reconciliation")
	}
	if err = reopened.nativeRunReady(run.ID); err == nil {
		t.Fatal("Run silently advanced with unresolved native process")
	}
	if err = reopened.updateSandboxNative(m.ID, "unconfirmed", "native-owned", "stopped", "Read-only status confirmed stopped"); err != nil {
		t.Fatal(err)
	}
	if err = reopened.nativeRunReady(run.ID); err != nil {
		t.Fatal("confirmed stop still blocks Run", err)
	}
	if err = reopened.updateSandboxNative(m.ID, "stopped", "native-owned", "running", ""); !errors.Is(err, ErrConflict) {
		t.Fatal("finished execution became running", err)
	}
}

func TestSandboxNativeIntentRequiresOwnedRunningMessage(t *testing.T) {
	s := testStore(t)
	_, run, c, m := nativeProcessFixture(t, s)
	changed := m
	changed.ID++
	if err := s.beginSandboxNative(run, c, changed); err == nil {
		t.Fatal("another message acquired native launch")
	}
	other := c
	other.ID = newID()
	if err := s.beginSandboxNative(run, other, m); err == nil {
		t.Fatal("another conversation acquired native launch")
	}
	if err := s.beginSandboxNative(run, c, m); err != nil {
		t.Fatal("valid native intent refused", err)
	}
	if err := s.updateSandboxNative(m.ID, "launching", "", "unconfirmed", "Launch response was lost"); err != nil {
		t.Fatal(err)
	}
	if err := s.nativeRunReady(run.ID); err == nil {
		t.Fatal("unknown-ID launch considered stopped")
	}
}

func TestSandboxNativeReconciliationIsReadOnlyAndKeepsUnknownResults(t *testing.T) {
	s := testStore(t)
	caller, run, c, m := nativeProcessFixture(t, s)
	if err := s.beginSandboxNative(run, c, m); err != nil {
		t.Fatal(err)
	}
	if err := s.updateSandboxNative(m.ID, "launching", "native-owned", "unconfirmed", "Interrupted during launch"); err != nil {
		t.Fatal(err)
	}
	run, _ = s.WorkflowRun(caller, run.ID)
	running := true
	missing := false
	var base string
	var writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes.Add(1)
			w.WriteHeader(500)
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/sandboxes/") && strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.URL.Path == "/sandboxes/"+run.Environment.ID:
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: run.Environment.ID, Metadata: map[string]string{"agent-platform-run": run.ID}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case r.URL.Path == "/command/status/native-owned":
			if missing {
				w.WriteHeader(404)
				return
			}
			value := opensandbox.CommandStatusResponse{ID: "native-owned", Running: running}
			if !running {
				exit := int32(-1)
				value.ExitCode = &exit
			}
			json.NewEncoder(w).Encode(value)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base = server.URL
	client, _ := newOpenSandbox(base, "private", "runtime", 600)
	if err := s.reconcileSandboxNativeProcesses(context.Background(), client, run); err == nil {
		t.Fatal("live process considered stopped")
	}
	running = false
	missing = true
	if err := s.reconcileSandboxNativeProcesses(context.Background(), client, run); err == nil {
		t.Fatal("missing command history considered absent process")
	}
	missing = false
	if err := s.reconcileSandboxNativeProcesses(context.Background(), client, run); err != nil {
		t.Fatal(err)
	}
	actual, _ := s.WorkflowRun(caller, run.ID)
	if writes.Load() != 0 || actual.NativeProcesses[0].Status != "stopped" {
		t.Fatal("reconciliation wrote native service or did not preserve identity", writes.Load(), actual.NativeProcesses)
	}
}

func TestSandboxEnvironmentReleaseClosesUnknownNativeWithoutInventingExit(t *testing.T) {
	s := testStore(t)
	caller, run, c, m := nativeProcessFixture(t, s)
	if err := s.beginSandboxNative(run, c, m); err != nil {
		t.Fatal(err)
	}
	if err := s.updateSandboxNative(m.ID, "launching", "", "unconfirmed", "Launch response lost"); err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, NewScheduler(s, nil, 1), "http://platform.local")
	if err := engine.Stop(caller, run.ID, run.Seq); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	actual, err := s.WorkflowRun(caller, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Status != "failed" {
		t.Fatal("unconfirmed native stop shown as completed stop", actual.Status)
	}
	if err = engine.ReleaseSandbox(context.Background(), caller, run.ID); err != nil {
		t.Fatal(err)
	}
	actual, _ = s.WorkflowRun(caller, run.ID)
	if actual.Environment.Status != "released" || actual.NativeProcesses[0].Status != "stopped" || !strings.Contains(actual.NativeProcesses[0].Error, "exit status is unavailable") {
		t.Fatal("environment absence was misrepresented", actual.NativeProcesses, actual.Environment)
	}
	if err = engine.Resume(caller, run.ID, actual.Seq, "resume"); err == nil {
		t.Fatal("released native environment resumed")
	}
}
