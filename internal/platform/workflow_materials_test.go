package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func materialRuns(t *testing.T, s *Store) (Caller, WorkflowRun, WorkflowRun) {
	t.Helper()
	c, w, source := runFixture(t, s)
	raw := append([]byte(strings.Repeat("原始失败\n", 12000)), 0, 255)
	if err := os.WriteFile(filepath.Join(source.WorkspacePath, "failure.log"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteWorkflowNode(c, source.ID, 1, NodeResult{Route: "approved", Summary: "original failure", Artifacts: []string{"failure.log"}}); err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "http://localhost")
	source = waitArchiveRun(t, e, c, source.ID, "completed")
	// The live source is no longer available. Only the retained bytes are valid.
	if err := os.RemoveAll(source.WorkspacePath); err != nil {
		t.Fatal(err)
	}
	a := testAgent(t, s)
	w.Nodes[0].Kind, w.Nodes[0].AgentID = "agent", a.ID
	w, err := s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	target, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "review original failure", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	e.Tick(context.Background())
	target, err = s.WorkflowRun(c, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	return c, source, target
}

func TestImportRetainedArtifactUsesOriginalBytesAndDoesNotAdvance(t *testing.T) {
	s := testStore(t)
	_, source, target := materialRuns(t, s)
	h := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, h)
	endpoint := "/api/workflow-runs/" + target.ID + "/artifacts/import"
	body := map[string]any{"seq": target.Seq, "source_run": source.ID, "source_seq": 1, "paths": []string{"failure.log"}}
	for i := 0; i < 2; i++ {
		response := workflowRequest(t, h, cookie, "POST", endpoint, body)
		if response.Code != http.StatusOK {
			t.Fatalf("import: %d %s", response.Code, response.Body.String())
		}
		var out struct {
			Files []struct {
				Path       string `json:"path"`
				SourcePath string `json:"source_path"`
				Bytes      int64  `json:"bytes"`
				SHA256     string `json:"sha256"`
			} `json:"files"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if len(out.Files) != 1 || out.Files[0].SourcePath != "failure.log" {
			t.Fatal(out)
		}
		want, err := s.archivedContent(source, 1, "failure.log")
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(target.WorkspacePath, out.Files[0].Path))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("changed retained input", err)
		}
		fresh, _ := s.WorkflowRun(Caller{Admin: true}, target.ID)
		if fresh.Seq != target.Seq || fresh.Status != target.Status || fresh.Steps[0].Result != nil {
			t.Fatal("import advanced target")
		}
	}
}

func TestImportArtifactsRejectsUnretainedOrStaleWithoutWriting(t *testing.T) {
	s := testStore(t)
	_, source, target := materialRuns(t, s)
	h := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, h)
	for _, body := range []map[string]any{
		{"seq": target.Seq + 1, "source_run": source.ID, "source_seq": 1, "paths": []string{"failure.log"}},
		{"seq": target.Seq, "source_run": source.ID, "source_seq": 1, "paths": []string{"../failure.log"}},
		{"seq": target.Seq, "source_run": source.ID, "source_seq": 1, "paths": []string{"failure.log", "missing.log"}},
	} {
		response := workflowRequest(t, h, cookie, "POST", "/api/workflow-runs/"+target.ID+"/artifacts/import", body)
		if response.Code < 400 {
			t.Fatal("invalid import accepted", response.Body.String())
		}
		if _, err := os.Stat(filepath.Join(target.WorkspacePath, "harness-output/imports")); !os.IsNotExist(err) {
			t.Fatal("wrote before validating all inputs", err)
		}
	}
}

func TestImportArtifactsEnforcesOwnershipIntegrityAndWritableStep(t *testing.T) {
	for _, scenario := range []string{"owner", "tampered", "readonly", "waiting", "stopped", "committed"} {
		t.Run(scenario, func(t *testing.T) {
			s := testStore(t)
			_, source, target := materialRuns(t, s)
			switch scenario {
			case "owner":
				if _, err := s.DB.Exec(`UPDATE workflow_runs SET owner='another-owner' WHERE id=?`, source.ID); err != nil {
					t.Fatal(err)
				}
			case "tampered":
				if err := os.WriteFile(filepath.Join(s.archiveDir(source.ID, 1), "files/failure.log"), []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			case "readonly":
				// Apply the ordinary persisted conversation permission used by execution.
				if _, err := s.DB.Exec(`UPDATE conversations SET snapshot=json_set(snapshot,'$.sandbox','read-only') WHERE id=?`, target.Steps[0].ConversationID); err != nil {
					t.Fatal(err)
				}
			case "waiting":
				if _, err := s.DB.Exec(`UPDATE workflow_runs SET status='waiting' WHERE id=?`, target.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := s.DB.Exec(`UPDATE workflow_steps SET status='waiting' WHERE run_id=?`, target.ID); err != nil {
					t.Fatal(err)
				}
			case "stopped":
				if _, err := s.DB.Exec(`UPDATE workflow_runs SET status='stopped' WHERE id=?`, target.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := s.DB.Exec(`UPDATE workflow_steps SET status='stopped' WHERE run_id=?`, target.ID); err != nil {
					t.Fatal(err)
				}
			case "committed":
				if _, err := s.DB.Exec(`UPDATE workflow_steps SET result='{"route":"next","summary":"already done"}' WHERE run_id=?`, target.ID); err != nil {
					t.Fatal(err)
				}
			}
			h := NewServer(s, nil, nil, "password", "http://localhost")
			response := workflowRequest(t, h, workflowAdmin(t, h), "POST", "/api/workflow-runs/"+target.ID+"/artifacts/import", map[string]any{"seq": target.Seq, "source_run": source.ID, "source_seq": 1, "paths": []string{"failure.log"}})
			if scenario == "waiting" {
				if response.Code != 200 {
					t.Fatal(response.Code, response.Body.String())
				}
				return
			}
			if response.Code < 400 {
				t.Fatal("unsafe import accepted", scenario, response.Body.String())
			}
			if _, err := os.Stat(filepath.Join(target.WorkspacePath, "harness-output/imports")); !os.IsNotExist(err) {
				t.Fatal("unsafe import wrote files", err)
			}
		})
	}
}
