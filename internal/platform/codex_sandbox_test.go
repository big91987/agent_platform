package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/coder/websocket"
)

// The native service is isolated here. This verifies platform ownership and
// protocol integration, not authentication/model behavior on the actual host.
func TestSandboxNativeTransportOwnsLaunchTurnAndStop(t *testing.T) {
	for _, mode := range []string{"complete", "execute", "cancel", "lost launch", "lost stop", "event fail lost stop"} {
		t.Run(mode, func(t *testing.T) {
			store := testStore(t)
			caller, run, c, m := nativeProcessFixture(t, store)
			c.Snapshot.NetworkAccess = true
			var base, capability string
			var filesMu sync.Mutex
			files := map[string][]byte{}
			var launches, turns, interrupts atomic.Int32
			turnStarted := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/sandboxes/"+run.Environment.ID:
					json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: run.Environment.ID, Metadata: map[string]string{"agent-platform-run": run.ID}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
				case strings.Contains(r.URL.Path, "/endpoints/"):
					json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
				case r.URL.Path == "/files/info":
					w.WriteHeader(404)
				case r.URL.Path == "/directories":
					w.WriteHeader(200)
				case r.URL.Path == "/files/download":
					filesMu.Lock()
					raw, ok := files[r.URL.Query().Get("path")]
					filesMu.Unlock()
					if !ok {
						w.WriteHeader(404)
						return
					}
					w.Write(raw)
				case r.URL.Path == "/files/upload":
					if err := r.ParseMultipartForm(1 << 20); err != nil {
						t.Error(err)
						return
					}
					defer r.MultipartForm.RemoveAll()
					metadata, _ := r.MultipartForm.File["metadata"][0].Open()
					var info opensandbox.FileMetadata
					json.NewDecoder(metadata).Decode(&info)
					metadata.Close()
					for _, file := range r.MultipartForm.File["file"] {
						stream, err := file.Open()
						if err != nil {
							t.Error(err)
							continue
						}
						raw, _ := io.ReadAll(stream)
						stream.Close()
						filesMu.Lock()
						files[info.Path] = raw
						filesMu.Unlock()
						capability = string(raw)
					}
					w.WriteHeader(200)
				case r.Method == "POST" && r.URL.Path == "/command":
					launches.Add(1)
					var in opensandbox.RunCommandRequest
					json.NewDecoder(r.Body).Decode(&in)
					if !in.Background || in.Cwd != "/workspace" || in.Envs["CODEX_HOME"] != "/var/lib/agent-platform/roles/"+c.ID {
						t.Error("role context changed", in.Cwd, in.Envs["CODEX_HOME"])
					}
					if mode == "lost launch" {
						conn, _, _ := w.(http.Hijacker).Hijack()
						conn.Close()
						return
					}
					w.Header().Set("Content-Type", "text/event-stream")
					io.WriteString(w, "data: {\"type\":\"init\",\"text\":\"native-owned\"}\n\ndata: {\"type\":\"execution_complete\"}\n\n")
				case r.URL.Path == "/command/status/native-owned":
					value := opensandbox.CommandStatusResponse{ID: "native-owned", Running: mode == "event fail lost stop" || interrupts.Load() == 0}
					if !value.Running {
						exit := int32(-1)
						value.ExitCode = &exit
					}
					json.NewEncoder(w).Encode(value)
				case r.Method == "DELETE" && r.URL.Path == "/command":
					interrupts.Add(1)
					if r.URL.Query().Get("id") != "native-owned" {
						t.Error("wrong interrupt identity")
					}
					if mode == "lost stop" || mode == "event fail lost stop" {
						conn, _, _ := w.(http.Hijacker).Hijack()
						conn.Close()
					}
				case r.URL.Path == "/command/native-owned/logs":
					io.WriteString(w, "native diagnostics\n")
				case r.URL.Path == "/":
					if r.Header.Get("Authorization") != "Bearer "+capability || len(capability) < 32 {
						t.Error("private native capability lost")
						w.WriteHeader(401)
						return
					}
					ws, err := websocket.Accept(w, r, nil)
					if err != nil {
						t.Error(err)
						return
					}
					defer ws.CloseNow()
					for {
						_, raw, err := ws.Read(r.Context())
						if err != nil {
							return
						}
						var packet struct {
							ID     json.RawMessage `json:"id"`
							Method string          `json:"method"`
							Params map[string]any  `json:"params"`
						}
						if err = json.Unmarshal(raw, &packet); err != nil {
							t.Error(err)
							return
						}
						send := func(value any) {
							b, _ := json.Marshal(value)
							if err := ws.Write(r.Context(), websocket.MessageText, b); err != nil {
								t.Error(err)
							}
						}
						switch packet.Method {
						case "initialize":
							send(map[string]any{"id": packet.ID, "result": map[string]any{}})
						case "skills/list":
							send(map[string]any{"id": packet.ID, "result": map[string]any{"data": []any{map[string]any{"cwd": "/workspace", "skills": []any{}, "errors": []any{}}}}})
						case "thread/start":
							if packet.Params["cwd"] != "/workspace" || packet.Params["sandbox"] != "workspace-write" || packet.Params["model"] != "gpt-6.1-sol" || packet.Params["allowProviderModelFallback"] != false {
								t.Error("native task configuration changed", packet.Params)
							}
							send(map[string]any{"id": packet.ID, "result": map[string]any{"model": packet.Params["model"], "thread": map[string]any{"id": "stable-role-thread"}}})
						case "turn/start":
							policy, _ := packet.Params["sandboxPolicy"].(map[string]any)
							if policy["type"] != "externalSandbox" || policy["networkAccess"] != "enabled" {
								t.Error("owned container turn still requests inner isolation", packet.Params["sandboxPolicy"])
							}
							if packet.Params["approvalPolicy"] == nil || packet.Params["approvalsReviewer"] != "user" {
								t.Error("external isolation discarded approval settings")
							}
							turns.Add(1)
							if mode == "cancel" {
								close(turnStarted)
								continue
							}
							send(map[string]any{"method": "item/agentMessage/delta", "params": map[string]any{"itemId": "answer", "delta": "Native role result"}})
							send(map[string]any{"method": "turn/completed", "params": map[string]any{"turn": map[string]any{"status": "completed"}}})
						}
					}
				default:
					t.Error("unexpected native request", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			base = server.URL
			client, err := newOpenSandbox(base, "private-control", "runtime", 600)
			if err != nil {
				t.Fatal(err)
			}
			// This test's owned endpoint is the native service above; real calls
			// obtain this same frozen connection through Store.sandboxFor.
			run.Environment.ConnectionHash = client.connectionHash
			executor := Codex{Root: store.Dir, AuthHome: t.TempDir()}
			if mode == "execute" {
				executor.Store = store
				store.sandbox = client
				// Fixture-only connection binding, not a production recovery path.
				if _, err := store.DB.Exec(`UPDATE workflow_run_environments SET connection_hash=? WHERE run_id=?`, client.connectionHash, run.ID); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(executor.AuthHome, "auth.json"), []byte(`{"tokens":{"access_token":"protocol-fixture-only"}}`), 0600); err != nil {
					t.Fatal(err)
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if mode == "cancel" {
				go func() {
					select {
					case <-turnStarted:
						cancel()
					case <-ctx.Done():
					}
				}()
			}
			var events strings.Builder
			onEvent := func(raw []byte) error {
				if mode == "event fail lost stop" && strings.Contains(string(raw), "native.launch_output") {
					return errors.New("event persistence unavailable")
				}
				events.Write(raw)
				return store.RecordEvent(c.ID, m.ID, raw)
			}
			if mode == "execute" {
				err = executor.Execute(ctx, c, m, onEvent)
			} else {
				err = executor.executeSandboxAppServer(ctx, store, client, run, c, m, "/var/lib/agent-platform/roles/"+c.ID, []byte("model='gpt-6.1-sol'\n"), map[string]string{}, []string{}, onEvent)
			}
			actual, readErr := store.WorkflowRun(caller, run.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if len(actual.NativeProcesses) != 1 || launches.Load() != 1 {
				t.Fatal("native intent missing or launch replayed", actual.NativeProcesses, launches.Load())
			}
			process := actual.NativeProcesses[0]
			if mode == "cancel" {
				if !errors.Is(err, context.Canceled) || process.Status != "stopped" || interrupts.Load() != 1 || turns.Load() != 1 {
					t.Fatal("cancellation lost bounded owned cleanup", process, err)
				}
			} else if mode == "complete" || mode == "execute" {
				if err != nil || process.Status != "stopped" || process.ID != "native-owned" || turns.Load() != 1 || interrupts.Load() != 1 || !strings.Contains(events.String(), "Native role result") {
					t.Fatal("owned native transport did not complete", process, err, turns.Load(), interrupts.Load(), events.String())
				}
			} else {
				if err == nil || process.Status != "unconfirmed" {
					t.Fatal("uncertain side effect hidden", process, err)
				}
				if mode == "lost launch" && (turns.Load() != 0 || interrupts.Load() != 0 || process.ID != "") {
					t.Fatal("unknown launch was replayed or guessed", process)
				}
				if mode == "lost stop" && (turns.Load() != 1 || interrupts.Load() != 1 || process.ID != "native-owned") {
					t.Fatal("unknown stop retried or identity lost", process)
				}
				if mode == "event fail lost stop" && (turns.Load() != 0 || interrupts.Load() != 1 || process.ID != "native-owned") {
					t.Fatal("failed launch cleanup interrupt was repeated", process, interrupts.Load())
				}
				if err := store.nativeRunReady(run.ID); err == nil {
					t.Fatal("unconfirmed native execution allowed continuation")
				}
			}
		})
	}
}

func TestExternalSandboxRefusesUnenforcedNodeRestrictionsBeforePreparation(t *testing.T) {
	for _, readonly := range []bool{false, true} {
		store := testStore(t)
		_, run, c, m := nativeProcessFixture(t, store)
		c.Snapshot.NetworkAccess = true
		if readonly {
			c.Snapshot.Sandbox = "read-only"
		} else {
			c.Snapshot.NetworkAccess = false
		}
		var contacted atomic.Int32
		remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { contacted.Add(1); w.WriteHeader(500) }))
		service, _ := newOpenSandbox(remote.URL, "private", "runtime", 600)
		run.Environment.ConnectionHash = service.connectionHash
		x := Codex{Root: store.Dir, AuthHome: t.TempDir()}
		err := x.executeSandboxAppServer(context.Background(), store, service, run, c, m, "/var/lib/agent-platform/roles/"+c.ID, []byte("model='gpt-6.1-sol'\n"), map[string]string{}, []string{}, func([]byte) error { return nil })
		remote.Close()
		if err == nil || contacted.Load() != 0 {
			t.Fatalf("unenforced restriction reached external environment: %v, contacts=%d", err, contacted.Load())
		}
		var count int
		if err = store.DB.QueryRow(`SELECT count(*) FROM workflow_native_processes WHERE message_id=?`, m.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal("unsupported policy recorded a native launch intent")
		}
	}
}
