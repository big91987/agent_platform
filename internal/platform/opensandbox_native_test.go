package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/coder/websocket"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestOpenSandboxNativeProcessHasOneLaunchAndExplicitOwnedStop(t *testing.T) {
	var base string
	var launches, interrupts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.Method == "POST" && r.URL.Path == "/command":
			var in opensandbox.RunCommandRequest
			json.NewDecoder(r.Body).Decode(&in)
			if !in.Background || in.Command != "exec codex app-server" || in.Cwd != "/workspace" {
				t.Error("native launch contract changed", in)
			}
			launches.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte("data: {\"type\":\"init\",\"text\":\"native-process\"}\n\ndata: {\"type\":\"execution_complete\"}\n\n"))
		case r.Method == "DELETE" && r.URL.Path == "/command":
			if r.URL.Query().Get("id") != "native-process" {
				t.Error("different process interrupted")
			}
			interrupts.Add(1)
		case r.URL.Path == "/command/status/native-process":
			running := interrupts.Load() == 0
			value := opensandbox.CommandStatusResponse{ID: "native-process", Running: running}
			if !running {
				exit := int32(-1)
				value.ExitCode = &exit
			}
			json.NewEncoder(w).Encode(value)
		default:
			t.Error("unexpected native request", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base = server.URL
	service, _ := newOpenSandbox(base, "private-control-key", "fixed-runtime", 600)
	process, err := service.startNative(context.Background(), "owned", sandboxTestRun, opensandbox.RunCommandRequest{Command: "exec codex app-server", Cwd: "/workspace", Timeout: 600000}, nil)
	if err != nil || process.ID != "native-process" || process.Stopped || launches.Load() != 1 {
		t.Fatal("native process not running", process, err)
	}
	stopped, err := service.stopNative(context.Background(), "owned", sandboxTestRun, process.ID)
	if err != nil || !stopped.Stopped || stopped.ExitCode == nil || *stopped.ExitCode != -1 || interrupts.Load() != 1 {
		t.Fatal("native stop unconfirmed", stopped, err)
	}
	if _, err = service.stopNative(context.Background(), "owned", sandboxTestRun, process.ID); err != nil || interrupts.Load() != 1 {
		t.Fatal("completed process interruption was replayed", err, interrupts.Load())
	}
}

func TestOpenSandboxUnconfirmedNativeLaunchNeverReposts(t *testing.T) {
	var base string
	var launches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.Method == "POST" && r.URL.Path == "/command":
			launches.Add(1)
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base = server.URL
	service, _ := newOpenSandbox(base, "private", "fixed-runtime", 600)
	result, err := service.startNative(context.Background(), "owned", sandboxTestRun, opensandbox.RunCommandRequest{Command: "exec codex app-server", Cwd: "/workspace", Timeout: 600000}, nil)
	if err == nil || result.ID != "" || result.Stopped || launches.Load() != 1 {
		t.Fatal("uncertain native launch hidden or replayed", result, err, launches.Load())
	}
}

func TestRuntimeProbeStopsNativeProcessWhenReceiptPersistenceFails(t *testing.T) {
	var base string
	var stopped atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.Method == "POST" && (r.URL.Path == "/files/upload" || r.URL.Path == "/directories"):
			w.WriteHeader(200)
		case r.Method == "POST" && r.URL.Path == "/command":
			var in opensandbox.RunCommandRequest
			json.NewDecoder(r.Body).Decode(&in)
			id := "typescript"
			if in.Background {
				id = "native"
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: {\"type\":\"init\",\"text\":%q}\n\ndata: {\"type\":\"execution_complete\"}\n\n", id)
		case strings.HasSuffix(r.URL.Path, "/logs"):
			io.WriteString(w, "native log\n")
		case r.Method == "DELETE" && r.URL.Path == "/command":
			stopped.Store(true)
		case strings.HasPrefix(r.URL.Path, "/command/status/"):
			id := strings.TrimPrefix(r.URL.Path, "/command/status/")
			value := opensandbox.CommandStatusResponse{ID: id, Running: id == "native" && !stopped.Load()}
			if !value.Running {
				exit := int32(0)
				value.ExitCode = &exit
			}
			json.NewEncoder(w).Encode(value)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base = server.URL
	service, _ := newOpenSandbox(base, "private", "fixed-runtime", 600)
	err := probeSandboxRuntime(context.Background(), service, "owned", sandboxTestRun, func(event string, _ any) error {
		if event == "native_launch_result" {
			return errors.New("receipt unavailable")
		}
		return nil
	})
	if err == nil || !stopped.Load() {
		t.Fatal("launched process leaked after persistence failure", err, stopped.Load())
	}
}

func TestRuntimeProbeRejectedNativeAuthenticationIsNotRetried(t *testing.T) {
	var base string
	var handshakes atomic.Int32
	var stopped atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.Method == "POST" && (r.URL.Path == "/files/upload" || r.URL.Path == "/directories"):
			w.WriteHeader(200)
		case r.Method == "POST" && r.URL.Path == "/command":
			var in opensandbox.RunCommandRequest
			json.NewDecoder(r.Body).Decode(&in)
			id := "typescript"
			if in.Background {
				id = "native"
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: {\"type\":\"init\",\"text\":%q}\n\ndata: {\"type\":\"execution_complete\"}\n\n", id)
		case strings.HasSuffix(r.URL.Path, "/logs"):
			io.WriteString(w, "native log\n")
		case r.Method == "DELETE" && r.URL.Path == "/command":
			stopped.Store(true)
		case strings.HasPrefix(r.URL.Path, "/command/status/"):
			id := strings.TrimPrefix(r.URL.Path, "/command/status/")
			value := opensandbox.CommandStatusResponse{ID: id, Running: id == "native" && !stopped.Load()}
			if !value.Running {
				exit := int32(0)
				value.ExitCode = &exit
			}
			json.NewEncoder(w).Encode(value)
		case r.URL.Path == "/":
			handshakes.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base = server.URL
	service, _ := newOpenSandbox(base, "private", "fixed-runtime", 600)
	err := probeSandboxRuntime(context.Background(), service, "owned", sandboxTestRun, func(string, any) error { return nil })
	if err == nil || handshakes.Load() != 1 || !stopped.Load() {
		t.Fatal("rejected native connection retried or process left running", err, handshakes.Load(), stopped.Load())
	}
}

func TestRuntimeProbeWaitsForNativeListenerWithoutRelaunch(t *testing.T) {
	var base string
	var handshakes, launches atomic.Int32
	var stopped atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.Method == "POST" && (r.URL.Path == "/files/upload" || r.URL.Path == "/directories"):
			w.WriteHeader(200)
		case r.Method == "POST" && r.URL.Path == "/command":
			var in opensandbox.RunCommandRequest
			json.NewDecoder(r.Body).Decode(&in)
			id := "typescript"
			if in.Background {
				id = "native"
				launches.Add(1)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: {\"type\":\"init\",\"text\":%q}\n\ndata: {\"type\":\"execution_complete\"}\n\n", id)
		case strings.HasSuffix(r.URL.Path, "/logs"):
			io.WriteString(w, "native log\n")
		case r.Method == "DELETE" && r.URL.Path == "/command":
			stopped.Store(true)
		case strings.HasPrefix(r.URL.Path, "/command/status/"):
			id := strings.TrimPrefix(r.URL.Path, "/command/status/")
			value := opensandbox.CommandStatusResponse{ID: id, Running: id == "native" && !stopped.Load()}
			if !value.Running {
				exit := int32(0)
				value.ExitCode = &exit
			}
			json.NewEncoder(w).Encode(value)
		case r.URL.Path == "/":
			if handshakes.Add(1) == 1 {
				conn, _, _ := w.(http.Hijacker).Hijack()
				conn.Close()
				return
			}
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.CloseNow()
			_, _, err = conn.Read(r.Context())
			if err != nil {
				return
			}
			conn.Write(r.Context(), websocket.MessageText, []byte(`{"id":1,"result":{}}`))
			unselected := true
			for {
				_, raw, err := conn.Read(r.Context())
				if err != nil {
					return
				}
				var req struct {
					ID     int            `json:"id"`
					Method string         `json:"method"`
					Params map[string]any `json:"params"`
				}
				if json.Unmarshal(raw, &req) != nil {
					return
				}
				if req.Method == "initialized" {
					continue
				}
				var result any
				switch req.Method {
				case "skills/list":
					result = map[string]any{"data": []any{map[string]any{"cwd": "/workspace", "errors": []any{}, "skills": []nativeSkill{{Path: "/tmp/probe-native-home/skills/probe-selected/SKILL.md", Enabled: true}, {Path: "/workspace/.agents/skills/probe-unselected/SKILL.md", Enabled: unselected}}}}}
				case "skills/config/write":
					unselected = false
					result = map[string]any{"effectiveEnabled": false}
				default:
					t.Error("unexpected probe request", req.Method)
					return
				}
				reply, _ := json.Marshal(map[string]any{"id": req.ID, "result": result})
				if err := conn.Write(r.Context(), websocket.MessageText, reply); err != nil {
					return
				}
			}

		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base = server.URL
	service, _ := newOpenSandbox(base, "private", "fixed-runtime", 600)
	initialized := false
	skillChecked := false
	err := probeSandboxRuntime(context.Background(), service, "owned", sandboxTestRun, func(event string, _ any) error {
		if event == "native_initialized" {
			initialized = true
		}
		if event == "native_skill_scope_confirmed" {
			skillChecked = true
		}
		return nil
	})
	if err != nil || !initialized || !skillChecked || handshakes.Load() != 2 || launches.Load() != 1 || !stopped.Load() {
		t.Fatal("native readiness failed or relaunched", err, initialized, handshakes.Load(), launches.Load(), stopped.Load())
	}
}

func TestOpenSandboxNativeHomePreparationAndBoundedLogs(t *testing.T) {
	var base string
	var made atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.URL.Path == "/directories" && r.Method == "POST":
			var in map[string]opensandbox.FileMetadata
			json.NewDecoder(r.Body).Decode(&in)
			if entry, ok := in["/tmp/native-home"]; !ok || entry.Mode != 700 {
				t.Error("private native home was not created", in)
			}
			made.Store(true)
		case r.URL.Path == "/command/native/logs":
			w.Header().Set("EXECD-COMMANDS-TAIL-CURSOR", "23")
			io.WriteString(w, "original startup error\n")
		case r.URL.Path == "/command/huge/logs":
			io.CopyN(w, strings.NewReader(strings.Repeat("x", 16*1024*1024+1)), 16*1024*1024+1)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base = server.URL
	service, _ := newOpenSandbox(base, "private", "fixed-runtime", 600)
	if err := service.nativeHome(context.Background(), "owned", sandboxTestRun, "/tmp/native-home"); err != nil || !made.Load() {
		t.Fatal("native home missing", err)
	}
	logs, err := service.nativeLogs(context.Background(), "owned", sandboxTestRun, "native")
	if err != nil || logs.Output != "original startup error\n" || logs.Cursor != 23 {
		t.Fatal("original background output lost", logs, err)
	}
	if _, err = service.nativeLogs(context.Background(), "owned", sandboxTestRun, "huge"); !errors.Is(err, errSandboxOutputLimit) {
		t.Fatal("unbounded native log response", err)
	}
	if _, err = service.nativeLogs(context.Background(), "owned", sandboxTestRun, "../other"); err == nil {
		t.Fatal("untrusted execution log path accepted")
	}
}

func TestSandboxCommandsRetainOwnedIDWhenEventPersistenceFails(t *testing.T) {
	for _, native := range []bool{true, false} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			var base string
			var launches, interrupts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/sandboxes/owned":
					json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
				case strings.Contains(r.URL.Path, "/endpoints/"):
					json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
				case r.Method == "POST" && r.URL.Path == "/command":
					launches.Add(1)
					w.Header().Set("Content-Type", "text/event-stream")
					io.WriteString(w, "data: {\"type\":\"init\",\"text\":\"owned-launch\"}\n\n")
				case r.Method == "DELETE" && r.URL.Path == "/command":
					if r.URL.Query().Get("id") != "owned-launch" {
						t.Error("interrupt targeted a different process")
					}
					interrupts.Add(1)
				case r.URL.Path == "/command/status/owned-launch":
					value := opensandbox.CommandStatusResponse{ID: "owned-launch", Running: interrupts.Load() == 0}
					if !value.Running {
						exit := int32(-1)
						value.ExitCode = &exit
					}
					json.NewEncoder(w).Encode(value)
				default:
					t.Error("unexpected request", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			base = server.URL
			service, _ := newOpenSandbox(base, "private", "fixed-runtime", 600)
			call := service.command
			if native {
				call = service.startNative
			}
			result, err := call(context.Background(), "owned", sandboxTestRun, opensandbox.RunCommandRequest{Command: "exec codex app-server", Cwd: "/workspace", Timeout: 600000}, func(opensandbox.StreamEvent) error {
				return errors.New("event persistence unavailable")
			})
			if err == nil || result.ID != "owned-launch" || !result.Stopped || result.ExitCode == nil || *result.ExitCode != -1 || launches.Load() != 1 || interrupts.Load() != 1 {
				t.Fatal("received launch identity was lost or owned cleanup skipped", result, err, launches.Load(), interrupts.Load())
			}
		})
	}
}
