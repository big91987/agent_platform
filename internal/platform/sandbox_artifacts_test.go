package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

func TestSandboxArtifactsUseConfinedRemoteFiles(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(dir, "report.html"), []byte("<h1>remote report</h1>"), 0600)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("private"), 0600)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("rules"), 0600)
	os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("outside"), 0600)
	os.Symlink(outside, filepath.Join(dir, "escape"))
	os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "leak.txt"))
	f, err := os.Create(filepath.Join(dir, "large.bin"))
	if err != nil {
		t.Fatal(err)
	}
	f.Truncate(8<<20 + 1)
	f.Close()
	var base string
	var exit int32
	calls := 0
	run := WorkflowRun{ID: newID(), WorkspacePath: "/workspace", Definition: Workflow{Environment: "opensandbox"}, Environment: &RunEnvironment{ID: "owned", Status: "ready"}}
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes/owned":
			json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": run.ID}, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}})
		case strings.Contains(r.URL.Path, "/endpoints/"):
			json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
		case r.Method == "POST" && r.URL.Path == "/command":
			calls++
			var in opensandbox.RunCommandRequest
			json.NewDecoder(r.Body).Decode(&in)
			// Execute the real descriptor-based reader in an isolated fixture root.
			command := strings.ReplaceAll(in.Command, "/workspace", dir)
			out, e := exec.CommandContext(r.Context(), "/bin/sh", "-c", command).CombinedOutput()
			exit = 0
			if e != nil {
				exit = 1
			}
			w.Header().Set("Content-Type", "text/event-stream")
			for _, event := range []any{map[string]any{"type": "init", "text": "reader"}, map[string]any{"type": "stdout", "text": string(out)}, map[string]any{"type": "execution_complete"}} {
				b, _ := json.Marshal(event)
				fmt.Fprintf(w, "%s\n\n", b)
			}
		case r.URL.Path == "/command/status/reader":
			json.NewEncoder(w).Encode(opensandbox.CommandStatusResponse{ID: "reader", ExitCode: &exit})
		default:
			w.WriteHeader(404)
		}
	}))
	defer remote.Close()
	base = remote.URL
	s := testStore(t)
	s.sandbox, _ = newOpenSandbox(base, "fixture-key", "fixture", 600)
	run.Environment.ConnectionHash = s.sandbox.connectionHash
	files, _, err := s.sandboxArtifacts(context.Background(), run, "")
	if err != nil || len(files) != 2 || files[1].Path != "report.html" {
		t.Fatal("confined listing lost", files, err)
	}
	_, content, err := s.sandboxArtifacts(context.Background(), run, "report.html")
	if err != nil || string(content) != "<h1>remote report</h1>" {
		t.Fatal("remote file unavailable", string(content), err)
	}
	for _, path := range []string{"../secret.txt", "/etc/passwd", ".env", "AGENTS.md", "escape/secret.txt", "leak.txt", "large.bin", "missing.txt"} {
		if _, _, err := s.sandboxArtifacts(context.Background(), run, path); err == nil {
			t.Fatalf("unsafe file accepted: %s", path)
		}
	}
	before := calls
	run.Environment.Status = "released"
	if _, _, err := s.sandboxArtifacts(context.Background(), run, "report.html"); err == nil || calls != before {
		t.Fatal("released environment used", err, calls)
	}

	// Use an existing Run/step foreign key, as the engine does. Only this unit
	// fixture substitutes the remote endpoint; it never changes a live Run.
	client := s.sandbox
	caller, graph, _, _ := sandboxWorkflowFixture(t, s, false, false)
	run, err = s.StartWorkflow(caller, WorkflowStart{WorkflowID: graph.ID, Input: "read report"})
	if err != nil {
		t.Fatal(err)
	}
	run = waitSandboxRun(t, NewWorkflowEngine(s, nil, ""), caller, run.ID, "ready")
	s.sandbox = client
	_, err = s.DB.Exec(`UPDATE workflow_run_environments SET sandbox_id='owned',connection_hash=? WHERE run_id=?`, client.connectionHash, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	config := Agent{Name: "Report reviewer", Executor: "codex", Model: "gpt-6.1-sol", Enabled: true}
	receipt, err := s.submitConfigured(caller, Input{environmentRun: run.ID, UserID: caller.UserID, Message: "review report", WorkspacePath: "/workspace"}, func(tx *sql.Tx, _ *Agent, id string) error {
		_, e := tx.Exec(`UPDATE workflow_steps SET conversation_id=? WHERE run_id=? AND seq=1`, id, run.ID)
		return e
	}, &config)
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(s, nil, nil, "password", "http://localhost")
	token, err := s.UserToken(caller.UserID, false)
	if err != nil {
		t.Fatal(err)
	}
	url := "/api/conversations/" + receipt.ConversationID
	w := requestJSON(t, server, "GET", url+"/file?path=report.html&preview=1", token, nil)
	if w.Code != 200 || w.Body.String() != "<h1>remote report</h1>" || strings.Contains(w.Header().Get("Content-Security-Policy"), "allow-same-origin") {
		t.Fatal("remote preview unavailable/unsafe", w.Code, w.Body.String())
	}
	w = requestJSON(t, server, "GET", url+"/artifacts", token, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "report.html") {
		t.Fatal("remote listing bypassed", w.Code, w.Body.String())
	}
	before = calls
	a := testAgent(t, s)
	_, other := testUser(t, s, &a, "other")
	if w = requestJSON(t, server, "GET", url+"/file?path=report.html", other, nil); w.Code != 403 || calls != before {
		t.Fatal("unauthorized remote read dispatched", w.Code, calls, before)
	}
	if w = requestJSON(t, server, "GET", url+"/file?path=../secret.txt", token, nil); w.Code != 403 || calls != before {
		t.Fatal("traversal dispatched", w.Code, calls, before)
	}
	_, err = s.DB.Exec(`UPDATE workflow_run_environments SET status='released' WHERE run_id=?`, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	w = requestJSON(t, server, "GET", url, token, nil)
	var view struct {
		Artifacts     []Artifact
		ArtifactError string `json:"artifact_error"`
		Messages      []Message
	}
	json.Unmarshal(w.Body.Bytes(), &view)
	if w.Code != 200 || view.ArtifactError == "" || len(view.Messages) == 0 || len(view.Artifacts) != 0 || calls != before {
		t.Fatal("released files hid history or read host fallback", w.Code, w.Body.String())
	}
}
