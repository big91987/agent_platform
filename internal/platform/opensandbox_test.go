package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

const sandboxTestRun = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestOpenSandboxUncertainCreateIsNotReplayedAndCanBeReconciled(t *testing.T) {
	var posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("OPEN-SANDBOX-API-KEY") != "private-test-key" {
			t.Error("missing control-plane authentication")
		}
		if r.Method == "POST" {
			posts.Add(1)
			var in opensandbox.CreateSandboxRequest
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				t.Error(err)
			}
			if in.Metadata["agent-platform-run"] != sandboxTestRun || len(in.Volumes) != 0 || in.Timeout == nil || *in.Timeout != 600 {
				t.Errorf("unexpected creation boundary: %+v", in)
			}
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			conn.Close()
			return
		}
		if r.URL.Query().Get("metadata") != "agent-platform-run="+sandboxTestRun {
			t.Error("unscoped reconciliation")
		}
		json.NewEncoder(w).Encode(opensandbox.ListSandboxesResponse{Items: []opensandbox.SandboxInfo{{ID: "actual-sandbox", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}}}})
	}))
	defer server.Close()
	client, err := newOpenSandbox(server.URL, "private-test-key", "python:3.12-slim", 600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.create(context.Background(), sandboxTestRun); err == nil || strings.Contains(err.Error(), "private-test-key") {
		t.Fatal("uncertain create incorrectly accepted", err)
	}
	existing, err := client.find(context.Background(), sandboxTestRun)
	if err != nil || existing == nil || existing.ID != "actual-sandbox" {
		t.Fatal(existing, err)
	}
	if posts.Load() != 1 {
		t.Fatal("creation replayed", posts.Load())
	}
}

func TestOpenSandboxOwnershipRejectsForeignEnvironmentBeforeCommand(t *testing.T) {
	var commands atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandboxes/foreign" {
			commands.Add(1)
		}
		json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "foreign", Metadata: map[string]string{"agent-platform-run": "another-run"}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
	}))
	defer server.Close()
	client, _ := newOpenSandbox(server.URL, "test", "python:3.12-slim", 600)
	if _, err := client.command(context.Background(), "foreign", sandboxTestRun, opensandbox.RunCommandRequest{Argv: []string{"python", "-c", "print('one')"}}, nil); err == nil {
		t.Fatal("foreign environment executed")
	}
	if commands.Load() != 0 {
		t.Fatal("command reached foreign environment")
	}
}

func TestOpenSandboxCommandKeepsEventsAndActualNonzeroStatus(t *testing.T) {
	var base string
	var posts atomic.Int32
	var actualExit atomic.Int32
	directory := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base, Headers: map[string]string{"X-Access-Token": "endpoint-only"}})
		case r.URL.Path == "/command":
			posts.Add(1)
			if r.Header.Get("OPEN-SANDBOX-API-KEY") != "" || r.Header.Get("X-Access-Token") != "endpoint-only" {
				t.Error("control credential crossed into execd")
			}
			var in opensandbox.RunCommandRequest
			json.NewDecoder(r.Body).Decode(&in)
			if in.Command == "" || len(in.Argv) != 0 {
				w.WriteHeader(400)
				w.Write([]byte(`{"code":"INVALID_REQUEST_BODY","message":"Command required"}`))
				return
			}
			cmd := exec.CommandContext(r.Context(), "/bin/sh", "-c", in.Command)
			cmd.Dir = directory
			output, processErr := cmd.CombinedOutput()
			exit := 0
			if processErr != nil {
				var failure *exec.ExitError
				if !errors.As(processErr, &failure) {
					t.Error(processErr)
					return
				}
				exit = failure.ExitCode()
			}
			actualExit.Store(int32(exit))
			if string(output) != "literal $(printf unintended) 'quote'\nnext" {
				t.Errorf("literal arguments changed: %q", output)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte("data: {\"type\":\"init\",\"text\":\"command-one\"}\n\ndata: {\"type\":\"stderr\",\"text\":\"original failure\"}\n\ndata: {\"type\":\"execution_complete\"}\n\n"))
		case r.URL.Path == "/command/status/command-one":
			exit := actualExit.Load()
			json.NewEncoder(w).Encode(opensandbox.CommandStatusResponse{ID: "command-one", ExitCode: &exit})
		default:
			t.Error("unexpected request", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base = server.URL
	client, _ := newOpenSandbox(server.URL, "control-secret", "python:3.12-slim", 600)
	var events []string
	result, err := client.command(context.Background(), "owned", sandboxTestRun, opensandbox.RunCommandRequest{Argv: []string{"/bin/sh", "-c", `printf '%s' "$1"; exit 13`, "argument", "literal $(printf unintended) 'quote'\nnext"}}, func(e opensandbox.StreamEvent) error { events = append(events, e.Data); return nil })
	if err != nil || result.ID != "command-one" || result.ExitCode == nil || *result.ExitCode != 13 || posts.Load() != 1 {
		t.Fatal(result, err)
	}
	if len(events) != 3 || !strings.Contains(events[1], "original failure") {
		t.Fatal("events rewritten", events)
	}
}

func TestOpenSandboxCancellationInterruptsKnownCommandAndConfirmsStopped(t *testing.T) {
	var base string
	started := make(chan struct{})
	var interrupted atomic.Bool
	var posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.URL.Path == "/command" && r.Method == "POST":
			posts.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte("data: {\"type\":\"init\",\"text\":\"blocked\"}\n\n"))
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-time.After(2 * time.Second):
			}
		case r.URL.Path == "/command" && r.Method == "DELETE":
			if r.URL.Query().Get("id") != "blocked" {
				t.Error("wrong command cancelled")
			}
			interrupted.Store(true)
		case r.URL.Path == "/command/status/blocked":
			exit := int32(143)
			json.NewEncoder(w).Encode(opensandbox.CommandStatusResponse{ID: "blocked", Running: !interrupted.Load(), ExitCode: &exit})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base = server.URL
	client, _ := newOpenSandbox(base, "test", "python:3.12-slim", 600)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		result, err := client.command(ctx, "owned", sandboxTestRun, opensandbox.RunCommandRequest{Command: "sleep 60"}, func(event opensandbox.StreamEvent) error {
			if strings.Contains(event.Data, `"type":"init"`) {
				close(started)
			}
			return nil
		})
		t.Logf("cancellation observation: received_id=%q stopped=%t error=%v", result.ID, result.Stopped, err)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("not started")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancel reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup not bounded")
	}
	if !interrupted.Load() || posts.Load() != 1 {
		t.Fatal("cancel did not target original execution")
	}
}

func TestOpenSandboxCancellationBeforeInitRetainsUnknownWithoutReplay(t *testing.T) {
	var base string
	accepted := make(chan struct{})
	var posts, interrupts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.Method == "POST" && r.URL.Path == "/command":
			posts.Add(1)
			var in opensandbox.RunCommandRequest
			json.NewDecoder(r.Body).Decode(&in)
			close(accepted)
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
		case r.Method == "DELETE":
			interrupts.Add(1)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base = server.URL
	client, _ := newOpenSandbox(base, "test", "python:3.12-slim", 600)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type observation struct {
		result SandboxCommandResult
		err    error
	}
	done := make(chan observation, 1)
	go func() {
		result, err := client.command(ctx, "owned", sandboxTestRun, opensandbox.RunCommandRequest{Argv: []string{"touch", "evidence"}}, nil)
		done <- observation{result, err}
	}()
	select {
	case <-accepted:
	case <-time.After(3 * time.Second):
		t.Fatal("command not accepted")
	}
	cancel()
	select {
	case value := <-done:
		if value.err == nil || value.result.Stopped || !strings.Contains(value.err.Error(), "unconfirmed") {
			t.Fatal("unknown result was invented", value)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("unconfirmed result hung")
	}
	if posts.Load() != 1 || interrupts.Load() != 0 {
		t.Fatal("unknown execution was replayed or a different process was interrupted")
	}
}

func TestOpenSandboxPrivateConfigRejectsSharedOrLinkedCredentials(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "sandbox.toml")
	content := []byte("[server]\nhost='127.0.0.1'\nport=8095\napi_key='private-fixture-secret'\n")
	if err := os.WriteFile(config, content, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOpenSandboxConfig(config, "python:3.12-slim", 600); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(dir, "linked.toml")
	if err := os.Symlink(config, linked); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOpenSandboxConfig(linked, "python:3.12-slim", 600); err == nil {
		t.Fatal("symlink config accepted")
	}
	os.Chmod(config, 0644)
	_, err := loadOpenSandboxConfig(config, "python:3.12-slim", 600)
	if err == nil || strings.Contains(err.Error(), "private-fixture-secret") {
		t.Fatal("unsafe config or secret-bearing error", err)
	}
}

func TestOpenSandboxRejectedCommandKeepsSafeCauseBeforeInit(t *testing.T) {
	var base string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		default:
			w.WriteHeader(400)
			w.Write([]byte(`{"code":"BAD_ARGUMENT","message":"private-fixture-secret"}`))
		}
	}))
	defer server.Close()
	base = server.URL
	client, _ := newOpenSandbox(base, "private-fixture-secret", "python:3.12-slim", 600)
	result, err := client.command(context.Background(), "owned", sandboxTestRun, opensandbox.RunCommandRequest{Argv: []string{"python", "-c", "print(1)"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") || strings.Contains(err.Error(), "private-fixture-secret") || result.Stopped {
		t.Fatal("lost safe initial failure cause", result, err)
	}
}

func TestOpenSandboxFileRoundTripPreservesBinaryAndPrivateMode(t *testing.T) {
	var base string
	var uploads atomic.Int32
	file := filepath.Join(t.TempDir(), "round-trip")
	payload := []byte{0, 1, '\n', 255}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.URL.Path == "/files/upload":
			uploads.Add(1)
			if err := r.ParseMultipartForm(4096); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			metadata, _, err := r.FormFile("metadata")
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			defer metadata.Close()
			var meta opensandbox.FileMetadata
			if err = json.NewDecoder(metadata).Decode(&meta); err != nil {
				t.Error(err)
				return
			}
			// Pinned execd parses the JSON integer as octal digits, not decimal bits.
			mode, err := strconv.ParseUint(strconv.Itoa(meta.Mode), 8, 32)
			if err != nil {
				w.WriteHeader(500)
				return
			}
			content, _, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			defer content.Close()
			raw, err := io.ReadAll(content)
			if err != nil {
				t.Error(err)
				return
			}
			if err = os.WriteFile(file, raw, os.FileMode(mode)); err != nil {
				t.Error(err)
				return
			}
		case r.URL.Path == "/files/download":
			if r.URL.Query().Get("path") != "/workspace/binary" {
				t.Error("path changed")
			}
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Error(err)
				w.WriteHeader(404)
				return
			}
			w.Write(raw)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base = server.URL
	client, _ := newOpenSandbox(base, "test", "python:3.12-slim", 600)
	if err := client.upload(context.Background(), "owned", sandboxTestRun, "/workspace/binary", payload, 0600); err != nil {
		t.Fatal(err)
	}
	execd, err := client.execd(context.Background(), "owned", sandboxTestRun)
	if err != nil {
		t.Fatal(err)
	}
	body, err := execd.DownloadFile(context.Background(), "/workspace/binary", "")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	actual, err := io.ReadAll(body)
	info, statErr := os.Stat(file)
	if err != nil || statErr != nil || !bytes.Equal(actual, payload) || info.Mode().Perm() != 0600 || uploads.Load() != 1 {
		t.Fatalf("round trip failed: %v %v", err, statErr)
	}
}
