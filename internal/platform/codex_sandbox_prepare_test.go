package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

func TestSandboxRolePreparationFreezesAssetsAndNeverReplaysUnknownUpload(t *testing.T) {
	for _, mode := range []string{"normal", "lost asset", "lost marker", "missing marker", "different role", "rotated workflow capability", "changed user variable", "missing resumed preparation"} {
		t.Run(mode, func(t *testing.T) {
			auth, root := t.TempDir(), t.TempDir()
			os.WriteFile(filepath.Join(auth, "auth.json"), []byte(`{"tokens":{"access_token":"fixture-only"}}`), 0600)
			c := Conversation{ID: newID(), Snapshot: Agent{Executor: "codex", Model: "gpt-6.1-sol", Instructions: "role", Sandbox: "workspace-write"}}
			if mode == "rotated workflow capability" {
				token := "initial-private-fixture-capability"
				c.Snapshot.Env = map[string]*string{workflowTokenEnv: &token}
				c.Snapshot.ResolvedTools = map[string]ResolvedToolServer{workflowToolServer: {Connection: MCPConnection{URL: "http://platform.invalid/api/workflow-node/mcp", BearerTokenEnvVar: workflowTokenEnv}}}
			}
			if mode == "changed user variable" {
				value := "initial setting"
				c.Snapshot.Env = map[string]*string{"USER_SETTING": &value}
			}
			run := WorkflowRun{ID: newID(), WorkspacePath: "/workspace", Environment: &RunEnvironment{ID: "owned", Status: "ready"}}
			var base string
			var writes atomic.Int32
			var mu sync.Mutex
			files := map[string][]byte{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				if r.Method != "GET" {
					writes.Add(1)
				}
				switch {
				case r.URL.Path == "/sandboxes/owned":
					json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": run.ID}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
				case strings.Contains(r.URL.Path, "/endpoints/"):
					json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
				case r.URL.Path == "/files/info":
					w.WriteHeader(404)
				case r.URL.Path == "/directories":
					w.WriteHeader(200)
				case r.URL.Path == "/files/upload":
					if err := r.ParseMultipartForm(1 << 20); err != nil {
						t.Error(err)
						w.WriteHeader(400)
						return
					}
					defer r.MultipartForm.RemoveAll()
					metadata, err := r.MultipartForm.File["metadata"][0].Open()
					if err != nil {
						t.Error(err)
						return
					}
					var info opensandbox.FileMetadata
					json.NewDecoder(metadata).Decode(&info)
					metadata.Close()
					file, err := r.MultipartForm.File["file"][0].Open()
					if err != nil {
						t.Error(err)
						return
					}
					value, _ := io.ReadAll(file)
					file.Close()
					if !strings.HasPrefix(info.Path, "/var/lib/agent-platform/roles/"+c.ID+"/") || info.Mode != 600 {
						t.Error("role upload escaped private home", info.Path, info.Mode)
					}
					if mode == "lost asset" && strings.HasSuffix(info.Path, "config.toml") {
						conn, _, _ := w.(http.Hijacker).Hijack()
						conn.Close()
						return
					}
					files[info.Path] = value
					if mode == "lost marker" && strings.HasSuffix(info.Path, "prepared.json") {
						conn, _, _ := w.(http.Hijacker).Hijack()
						conn.Close()
						return
					}
				case r.URL.Path == "/files/download":
					value, ok := files[r.URL.Query().Get("path")]
					if !ok {
						w.WriteHeader(404)
						return
					}
					w.Write(value)
				default:
					t.Error("unexpected preparation request", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			base = server.URL
			client, _ := newOpenSandbox(base, "private-control", "runtime", 600)
			run.Environment.ConnectionHash = client.connectionHash
			x := Codex{Root: root, AuthHome: auth}
			if mode == "missing resumed preparation" {
				c.ThreadID = "original-thread"
			}
			home, config, skills, _, err := x.prepareSandboxRole(context.Background(), client, run, c)
			if mode == "missing resumed preparation" {
				if err == nil || writes.Load() != 0 {
					t.Fatal("missing preparation replaced native session", err, writes.Load())
				}
				return
			}
			if mode == "lost asset" || mode == "lost marker" {
				if err == nil {
					t.Fatal("lost upload reported prepared")
				}
			} else if err != nil || home != "/var/lib/agent-platform/roles/"+c.ID || len(config) == 0 || skills == nil {
				t.Fatal("role preparation failed", err, home)
			}
			before := writes.Load()
			// Existing role credentials are native-owned; resuming must not copy a
			// different/expired host auth file or replace the native home.
			os.Remove(filepath.Join(auth, "auth.json"))
			if mode == "missing marker" {
				mu.Lock()
				delete(files, home+"/prepared.json")
				mu.Unlock()
			}
			if mode == "different role" {
				c.Snapshot.Instructions = "replaced role"
			}
			if mode == "rotated workflow capability" {
				token := "new-turn-private-fixture-capability"
				c.Snapshot.Env[workflowTokenEnv] = &token
			}
			if mode == "changed user variable" {
				value := "replaced setting"
				c.Snapshot.Env["USER_SETTING"] = &value
			}
			c.Snapshot.NetworkAccess = true
			c.Snapshot.AllowElevation = true
			c.Snapshot.ApprovalsReviewer = "auto_review"
			home2, config2, _, resumedEnv, err := (&Codex{Root: root, AuthHome: auth}).prepareSandboxRole(context.Background(), client, run, c)
			if writes.Load() != before {
				t.Fatal("resume replayed an upload", writes.Load(), before)
			}
			if mode == "rotated workflow capability" && (resumedEnv[workflowTokenEnv] != "new-turn-private-fixture-capability" || *c.Snapshot.Env[workflowTokenEnv] != "new-turn-private-fixture-capability") {
				t.Fatal("fresh capability missing or conversation snapshot mutated")
			}
			if mode == "normal" || mode == "lost marker" || mode == "rotated workflow capability" {
				if err != nil || home2 == "" || (mode == "normal" && string(config2) != string(config)) {
					t.Fatal("same role or read-only marker reconciliation failed", err)
				}
			} else if err == nil {
				t.Fatal("incomplete or changed role was accepted", mode)
			}
		})
	}
}
