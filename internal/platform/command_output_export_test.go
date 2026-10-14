package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

func TestCommandLogWritersResumeWithoutOverwritingOrFollowingLinks(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(fmt.Sprint("sandbox=", remote), func(t *testing.T) {
			workspace := t.TempDir()
			os.WriteFile(filepath.Join(workspace, "json.py"), []byte("print('{\"complete\":true}');raise SystemExit(0)"), 0600)
			run := strings.Repeat("a", 32)
			raw := append([]byte(strings.Repeat("中 retained\n", 20000)), 0xff, 0x00)
			digest := fmt.Sprintf("%x", sha256.Sum256(raw))
			write := func(offset int, b []byte) error {
				if !remote {
					return writeCommandLogChunk(workspace, run, 3, offset, b, len(raw), digest)
				}
				meta, _ := json.Marshal(map[string]any{"run": run, "seq": 3, "offset": offset, "total": len(raw), "sha256": digest})
				script := strings.Replace(sandboxCommandLogWriter, "'/workspace'", fmt.Sprintf("%q", workspace), 1)
				cmd := exec.Command("python3", "-I", "-S", "-c", script, string(meta), base64.StdEncoding.EncodeToString(b))
				cmd.Dir = workspace
				cmd.Env = append(os.Environ(), "PYTHONPATH="+workspace, "PYTHONOPTIMIZE=1")
				out, err := cmd.CombinedOutput()
				if err != nil {
					return fmt.Errorf("%w: %s", err, out)
				}
				return nil
			}
			if err := write(0, raw[:65536]); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(workspace, "workflow-output", run, "command-3.log")
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatal("partial export presented as complete", err)
			}
			// Retry after an unknown response checks the already-written prefix.
			if err := write(0, raw[:65536]); err != nil {
				t.Fatal(err)
			}
			for offset := 65536; offset < len(raw); offset += 65536 {
				if err := write(offset, raw[offset:min(offset+65536, len(raw))]); err != nil {
					t.Fatal(err)
				}
			}
			got, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(got, raw) {
				t.Fatal("export changed raw bytes", err)
			}
			st, _ := os.Stat(target)
			if st.Mode().Perm() != 0600 {
				t.Fatal("export permissions", st.Mode())
			}
			if err := write(0, raw[:65536]); err != nil {
				t.Fatal("repeat complete export", err)
			}
			os.WriteFile(target, []byte("other evidence"), 0600)
			if err := write(0, raw[:65536]); err == nil {
				t.Fatal("conflicting file overwritten")
			}
			got, _ = os.ReadFile(target)
			if string(got) != "other evidence" {
				t.Fatal("changed conflicting evidence")
			}
			os.Remove(target)
			outside := filepath.Join(t.TempDir(), "outside")
			os.WriteFile(outside, []byte("keep"), 0600)
			os.Symlink(outside, target)
			if err := write(0, raw[:65536]); err == nil {
				t.Fatal("final symlink followed")
			}
			os.Remove(target)
			os.Symlink(outside, target+".partial")
			if err := write(0, raw[:65536]); err == nil {
				t.Fatal("partial symlink followed")
			}
			os.Remove(target + ".partial")
			os.Link(outside, target+".partial")
			if err := write(0, raw[:65536]); err == nil {
				t.Fatal("partial hardlink written")
			}
			os.Remove(target + ".partial")
			os.Remove(filepath.Dir(target))
			os.Symlink(filepath.Dir(outside), filepath.Dir(target))
			if err := write(0, raw[:65536]); err == nil {
				t.Fatal("parent symlink followed")
			}
			got, _ = os.ReadFile(outside)
			if string(got) != "keep" {
				t.Fatal("outside evidence changed")
			}
		})
	}
}

func TestSandboxCommandLogExportUsesOwnedEnvironmentAndCompleteReceipt(t *testing.T) {
	workspace := t.TempDir()
	s := testStore(t)
	raw := []byte(strings.Repeat("retained redacted 中\n", 17000))
	run := WorkflowRun{ID: newID(), WorkspacePath: "/workspace", Definition: Workflow{Environment: "opensandbox"}, Environment: &RunEnvironment{ID: "owned", Status: "ready"}}
	run.Steps = []WorkflowStep{{Seq: 1, Status: "completed", ExecutionReceipt: &ConnectorReceipt{Kind: "command", Log: &ConnectorLogInfo{Bytes: int64(len(raw)), Truncated: true}}}}
	home := filepath.Join(s.Dir, "workflow-processes", run.ID+"-1")
	os.MkdirAll(home, 0700)
	os.WriteFile(filepath.Join(home, "output.log"), raw, 0600)
	var base string
	var exit int32
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": run.ID}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.Method == "POST" && r.URL.Path == "/command":
			calls++
			var in opensandbox.RunCommandRequest
			json.NewDecoder(r.Body).Decode(&in)
			out, err := exec.CommandContext(r.Context(), "/bin/sh", "-c", strings.ReplaceAll(in.Command, "/workspace", workspace)).CombinedOutput()
			exit = 0
			if err != nil {
				exit = 1
			}
			w.Header().Set("Content-Type", "text/event-stream")
			for _, event := range []any{map[string]any{"type": "init", "text": "writer"}, map[string]any{"type": "stdout", "text": string(out)}, map[string]any{"type": "execution_complete"}} {
				b, _ := json.Marshal(event)
				fmt.Fprintf(w, "%s\n\n", b)
			}
		case r.URL.Path == "/command/status/writer":
			json.NewEncoder(w).Encode(opensandbox.CommandStatusResponse{ID: "writer", ExitCode: &exit})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base = server.URL
	s.sandbox, _ = newOpenSandbox(base, "fixture-key", "fixture", 600)
	run.Environment.ConnectionHash = s.sandbox.connectionHash
	dispatch := func(ctx context.Context, in opensandbox.RunCommandRequest, event opensandbox.EventHandler) (SandboxCommandResult, error) {
		return s.sandbox.command(ctx, run.Environment.ID, run.ID, in, event)
	}
	out, err := s.exportCommandOutput(context.Background(), run, 1, dispatch)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(workspace, out.Path))
	if err != nil || !bytes.Equal(raw, got) || !out.Truncated || out.Bytes != int64(len(raw)) || out.SHA256 != fmt.Sprintf("%x", sha256.Sum256(raw)) || calls < 2 {
		t.Fatal("invalid export receipt", out, calls, err)
	}
	callsBefore := calls
	run.Environment.Status = "released"
	if _, err = s.exportCommandOutput(context.Background(), run, 1, dispatch); err == nil || calls != callsBefore {
		t.Fatal("released environment received export")
	}
}

func TestSandboxLogExportStopAndUnknownOutcomes(t *testing.T) {
	for _, mode := range []string{"stop", "lost-launch", "lost-status"} {
		t.Run(mode, func(t *testing.T) {
			s := testStore(t)
			caller, run := agentRun(t, s)
			s.DB.Exec(`UPDATE workflow_steps SET seq=2 WHERE run_id=?`, run.ID)
			s.DB.Exec(`UPDATE workflow_runs SET seq=2 WHERE id=?`, run.ID)
			s.DB.Exec(`INSERT INTO workflow_steps(run_id,seq,node_id,status,created,updated) VALUES(?,1,'prior','completed',?,?)`, run.ID, now(), now())
			raw := bytes.Repeat([]byte("retained\n"), 20000)
			home := filepath.Join(s.Dir, "workflow-processes", run.ID+"-1")
			os.MkdirAll(home, 0700)
			os.WriteFile(filepath.Join(home, "output.log"), raw, 0600)
			receipt, _ := json.Marshal(ConnectorReceipt{Kind: "command", Log: &ConnectorLogInfo{Bytes: int64(len(raw))}})
			s.DB.Exec(`INSERT INTO workflow_connector_actions(run_id,seq,request,receipt) VALUES(?,1,'{}',?)`, run.ID, string(receipt))
			engine := NewWorkflowEngine(s, nil, "http://localhost")
			engine.Tick(context.Background())
			conv, msg, err := s.Claim()
			if err != nil {
				t.Fatal(err)
			}
			run, _ = s.WorkflowRun(caller, run.ID)
			run.Definition.Environment = "opensandbox"
			definition, _ := json.Marshal(run.Definition)
			s.DB.Exec(`UPDATE workflow_runs SET definition=? WHERE id=?`, string(definition), run.ID)
			entered := make(chan struct{})
			var posts, interrupts atomic.Int32
			var statusOK atomic.Bool
			var base string
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/sandboxes/owned":
					json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": run.ID}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
				case strings.Contains(r.URL.Path, "/endpoints/"):
					json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
				case r.Method == "POST" && r.URL.Path == "/command":
					posts.Add(1)
					if mode == "lost-launch" {
						w.WriteHeader(503)
						return
					}
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "{\"type\":\"init\",\"text\":\"writer\"}\n\n")
					w.(http.Flusher).Flush()
					if mode == "stop" {
						close(entered)
						<-r.Context().Done()
						return
					}
					fmt.Fprint(w, "{\"type\":\"stdout\",\"text\":\"{\\\"complete\\\":false}\"}\n\n")
				case r.URL.Path == "/command/status/writer":
					if mode == "lost-status" && !statusOK.Load() {
						w.WriteHeader(503)
						return
					}
					exit := int32(0)
					json.NewEncoder(w).Encode(opensandbox.CommandStatusResponse{ID: "writer", ExitCode: &exit})
				case r.Method == "DELETE" && r.URL.Path == "/command":
					interrupts.Add(1)
					w.WriteHeader(204)
				default:
					w.WriteHeader(404)
				}
			}))
			defer remote.Close()
			base = remote.URL
			s.sandbox, _ = newOpenSandbox(base, "fixture", "fixture", 600)
			s.DB.Exec(`INSERT INTO workflow_run_environments(run_id,sandbox_id,status,image,ttl,connection_hash,updated) VALUES(?,'owned','ready','fixture',600,?,?)`, run.ID, s.sandbox.connectionHash, now())
			done := make(chan error, 1)
			go func() { _, err := engine.exportCommandOutput(context.Background(), run.ID, 2, 1); done <- err }()
			if mode == "stop" {
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					t.Fatal("export did not dispatch")
				}
				if err := engine.Stop(caller, run.ID, 2); err != nil {
					t.Fatal(err)
				}
				if err := engine.ReleaseSandbox(context.Background(), caller, run.ID); err == nil {
					t.Fatal("active export released")
				}
				engine.Tick(context.Background())
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("unconfirmed/interrupted export passed")
				}
			case <-time.After(8 * time.Second):
				t.Fatal("export did not cancel")
			}
			if posts.Load() != 1 {
				t.Fatal("export dispatched another chunk", posts.Load())
			}
			s.Complete(conv.ID, msg.ID, "completed", "")
			if mode == "stop" {
				if interrupts.Load() != 1 || s.logExportReady(run.ID) != nil {
					t.Fatal("owned command not stopped", interrupts.Load())
				}
				engine.Tick(context.Background())
				after, _ := s.WorkflowRun(caller, run.ID)
				if after.Status != "stopped" {
					t.Fatal("stop not settled", after.Status, after.Error)
				}
				if _, err := engine.exportCommandOutput(context.Background(), run.ID, 2, 1); err == nil {
					t.Fatal("stopped Run exported")
				}
			} else {
				if err := s.initCommandOutputExports(); err != nil {
					t.Fatal(err)
				} // startup does not erase uncertain dispatch
				if s.executionRunReady(run.ID) == nil {
					t.Fatal("unknown export vanished after restart")
				}
				engine.Tick(context.Background())
				after, _ := s.WorkflowRun(caller, run.ID)
				if after.Status != "failed" {
					t.Fatal("unknown export did not block handoff", after.Status)
				}
				if err := engine.Resume(caller, run.ID, 2, "inspect export"); err == nil {
					t.Fatal("unknown export allowed resume")
				}
				if err := s.reconcileLogExport(context.Background(), s.sandbox, after); err == nil {
					t.Fatal("unknown status accepted")
				}
				if mode == "lost-status" {
					statusOK.Store(true)
					if err := s.reconcileLogExport(context.Background(), s.sandbox, after); err != nil {
						t.Fatal(err)
					}
					if s.executionRunReady(run.ID) != nil {
						t.Fatal("independently stopped helper remained blocked")
					}
				}
				if posts.Load() != 1 {
					t.Fatal("reconciliation replayed export")
				}
			}
		})
	}
}

func TestLogExportLateReconciliationCannotConfirmNewDispatch(t *testing.T) {
	s := testStore(t)
	_, run := agentRun(t, s)
	if _, err := s.DB.Exec(`INSERT INTO workflow_log_exports(run_id,command_id,quiet) VALUES(?,'previous',0)`, run.ID); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var base string
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": run.ID}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.URL.Path == "/command/status/previous":
			close(entered)
			<-release
			exit := int32(0)
			json.NewEncoder(w).Encode(opensandbox.CommandStatusResponse{ID: "previous", ExitCode: &exit})
		default:
			w.WriteHeader(404)
		}
	}))
	defer remote.Close()
	base = remote.URL
	client, _ := newOpenSandbox(base, "fixture", "fixture", 600)
	run.Environment = &RunEnvironment{ID: "owned", Status: "ready"}
	done := make(chan error, 1)
	go func() { done <- s.reconcileLogExport(context.Background(), client, run) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("status not requested")
	}
	// Another reconciliation and subsequent dispatch won the race. This isolated
	// fixture models its persisted boundary before the new command ID arrives.
	s.DB.Exec(`UPDATE workflow_log_exports SET command_id='',quiet=0 WHERE run_id=?`, run.ID)
	close(release)
	if err := <-done; err == nil {
		t.Fatal("late response confirmed another dispatch")
	}
	var id string
	var quiet int
	s.DB.QueryRow(`SELECT command_id,quiet FROM workflow_log_exports WHERE run_id=?`, run.ID).Scan(&id, &quiet)
	if id != "" || quiet != 0 {
		t.Fatal("new intent overwritten", id, quiet)
	}
}
