package platform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkflowArchivesDeclaredFilesBeforeAdvancing(t *testing.T) {
	s := testStore(t)
	c, _, r := runFixture(t, s)
	if err := os.WriteFile(filepath.Join(r.WorkspacePath, "report.txt"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteWorkflowNode(c, r.ID, 1, NodeResult{Route: "approved", Summary: "report", Artifacts: []string{"report.txt"}}); err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		engine.Tick(context.Background())
		r, _ = s.WorkflowRun(c, r.ID)
		if r.Status == "completed" || r.Status == "failed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if r.Status != "completed" {
		t.Fatal(r.Status, r.Error)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "runs", r.ID, "artifacts", "1", "manifest.json")); err != nil {
		t.Fatal("handoff advanced without durable artifact", err)
	}
}

func TestWorkflowMissingArtifactStopsBeforeNextNode(t *testing.T) {
	s := testStore(t)
	c, _, r := runFixture(t, s)
	if err := s.CompleteWorkflowNode(c, r.ID, 1, NodeResult{Route: "approved", Summary: "missing", Artifacts: []string{"missing.txt"}}); err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		engine.Tick(context.Background())
		r, _ = s.WorkflowRun(c, r.ID)
		if r.Status == "completed" || r.Status == "failed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if r.Status != "failed" || r.Seq != 1 {
		t.Fatal("missing artifact did not block handoff", r.Status, r.Seq)
	}
}

func waitArchiveRun(t *testing.T, e *WorkflowEngine, c Caller, id, status string) WorkflowRun {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	var r WorkflowRun
	for time.Now().Before(deadline) {
		e.Tick(context.Background())
		r, _ = e.store.WorkflowRun(c, id)
		if r.Status == status {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("wanted %s got %s: %s", status, r.Status, r.Error)
	return r
}
func TestWorkflowArchiveRetryRetainsAcceptedHandoffAndSnapshot(t *testing.T) {
	s := testStore(t)
	c, _, r := runFixture(t, s)
	e := NewWorkflowEngine(s, nil, "")
	if err := s.CompleteWorkflowNode(c, r.ID, 1, NodeResult{Route: "approved", Summary: "same accepted result", Artifacts: []string{"report.txt"}}); err != nil {
		t.Fatal(err)
	}
	failed := waitArchiveRun(t, e, c, r.ID, "failed")
	if err := s.requireWorkflowArchives(failed); err == nil {
		t.Fatal("release guard accepted missing file")
	}
	if err := os.WriteFile(filepath.Join(r.WorkspacePath, "report.txt"), []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.RetainArtifacts(context.Background(), c, r.ID, 1); err != nil {
		t.Fatal(err)
	}
	done := waitArchiveRun(t, e, c, r.ID, "completed")
	if done.Steps[0].Result.Summary != failed.Steps[0].Result.Summary || len(done.Steps) != 2 {
		t.Fatal("replayed or replaced handoff")
	}
	if err := s.requireWorkflowArchives(done); err != nil {
		t.Fatal(err)
	}
	// Remove the entire workspace. The retained copy must remain readable.
	if err := os.RemoveAll(r.WorkspacePath); err != nil {
		t.Fatal(err)
	}
	data, err := s.archivedContent(done, 1, "report.txt")
	if err != nil || string(data) != "retained" {
		t.Fatal(string(data), err)
	}
	if err = e.RetainArtifacts(context.Background(), c, r.ID, 1); err != nil {
		t.Fatal(err)
	}
	e.connectorWG.Wait()
	data, err = s.archivedContent(done, 1, "report.txt")
	if err != nil || string(data) != "retained" {
		t.Fatal("idempotent capture lost snapshot", err)
	}
	if err := os.WriteFile(filepath.Join(s.archiveDir(r.ID, 1), "files", "report.txt"), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.archivedContent(done, 1, "report.txt"); err == nil {
		t.Fatal("served corrupted artifact")
	}
	if err = s.requireWorkflowArchives(done); err == nil {
		t.Fatal("release allowed corrupt artifact")
	}
}
func TestWorkflowArchivesIsolateRunsAndRejectEscape(t *testing.T) {
	s := testStore(t)
	c, w, a := runFixture(t, s)
	b, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "other", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "")
	for _, r := range []WorkflowRun{a, b} {
		if err = os.WriteFile(filepath.Join(r.WorkspacePath, "same.json"), []byte(r.ID), 0600); err != nil {
			t.Fatal(err)
		}
		if err = s.CompleteWorkflowNode(c, r.ID, 1, NodeResult{Route: "approved", Summary: "done", Artifacts: []string{"same.json"}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []WorkflowRun{a, b} {
		done := waitArchiveRun(t, e, c, r.ID, "completed")
		data, err := s.archivedContent(done, 1, "same.json")
		if err != nil || string(data) != r.ID {
			t.Fatal("cross-Run content", err)
		}
		for _, p := range []string{"../same.json", "/etc/passwd", ".env", "sub/../../secret", "other.json"} {
			if _, err = s.archivedContent(done, 1, p); err == nil {
				t.Fatal("read unexpected path", p)
			}
		}
	}
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret"), []byte("private"), 0600)
	os.Symlink(outside, filepath.Join(a.WorkspacePath, "link"))
	if _, err = readLocalArchive(a.WorkspacePath, "link/secret"); err == nil {
		t.Fatal("followed symlink")
	}
	os.WriteFile(filepath.Join(a.WorkspacePath, "large"), make([]byte, archiveFileLimit+1), 0600)
	if _, err = readLocalArchive(a.WorkspacePath, "large"); err == nil {
		t.Fatal("unbounded archive")
	}
}
func TestWorkflowArchiveWaitsForNativeTurn(t *testing.T) {
	s := testStore(t)
	c, r := agentRun(t, s)
	e := NewWorkflowEngine(s, nil, "")
	e.Tick(context.Background())
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(r.WorkspacePath, "report.txt"), []byte("unfinished"), 0600)
	if err = s.testHandoffResult(r.ID, 1, *conv.Snapshot.Env[workflowTokenEnv], NodeResult{Route: "approved", Summary: "done", Artifacts: []string{"report.txt"}}); err != nil {
		t.Fatal(err)
	}
	e.Tick(context.Background())
	if _, err = os.Stat(s.archiveDir(r.ID, 1)); !os.IsNotExist(err) {
		t.Fatal("captured while native still running")
	}
	os.WriteFile(filepath.Join(r.WorkspacePath, "report.txt"), []byte("finished"), 0600)
	s.Complete(conv.ID, msg.ID, "completed", "")
	done := waitArchiveRun(t, e, c, r.ID, "completed")
	data, err := s.archivedContent(done, 1, "report.txt")
	if err != nil || string(data) != "finished" {
		t.Fatal(string(data), err)
	}
}

func TestWorkflowArchivedHTTPAuthorizationAndPreviewAfterWorkspaceRemoval(t *testing.T) {
	s := testStore(t)
	c, _, r := runFixture(t, s)
	e := NewWorkflowEngine(s, nil, "")
	os.WriteFile(filepath.Join(r.WorkspacePath, "report.html"), []byte("<h1>retained</h1>"), 0600)
	if err := s.CompleteWorkflowNode(c, r.ID, 1, NodeResult{Route: "approved", Summary: "done", Artifacts: []string{"report.html"}}); err != nil {
		t.Fatal(err)
	}
	waitArchiveRun(t, e, c, r.ID, "completed")
	os.RemoveAll(r.WorkspacePath)
	token, err := s.UserToken(c.UserID, false)
	if err != nil {
		t.Fatal(err)
	}
	agent := testAgent(t, s)
	_, other := testUser(t, s, &agent, "outsider")
	server := NewServer(s, nil, nil, "password", "http://localhost")
	base := "/api/workflow-runs/" + r.ID
	for _, endpoint := range []string{base + "/artifacts", base + "/steps/1/artifacts/file?path=report.html&preview=1"} {
		for _, auth := range []string{"", other} {
			w := requestJSON(t, server, "GET", endpoint, auth, nil)
			if w.Code != 401 && w.Code != 403 {
				t.Fatal("cross-user artifact access", w.Code)
			}
		}
	}
	w := requestJSON(t, server, "GET", base+"/steps/1/artifacts/file?path=report.html&preview=1", token, nil)
	if w.Code != 200 || w.Body.String() != "<h1>retained</h1>" {
		t.Fatal(w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox allow-scripts;") || strings.Contains(w.Header().Get("Content-Security-Policy"), "allow-same-origin") {
		t.Fatal("unsafe preview")
	}
	w = requestJSON(t, server, "GET", base+"/steps/1/artifacts/file?path=report.html&download=1", token, nil)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("download missing")
	}
	w = requestJSON(t, server, "POST", base+"/steps/1/artifacts/retain", other, map[string]any{})
	if w.Code != 403 {
		t.Fatal("cross-user capture allowed", w.Code)
	}
}

func TestWorkflowArchiveReleaseGuardBeforeRemoteDelete(t *testing.T) {
	s := testStore(t)
	c, w, _, deletes := sandboxWorkflowFixture(t, s, false, false)
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "archive guard"})
	if err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "")
	e.Tick(context.Background())
	if err = e.Stop(c, r.ID, 1); err != nil {
		t.Fatal(err)
	}
	e.Tick(context.Background())
	// A declared result on a historical stopped step models upgrade retention:
	// no remote command should be sent just to check whether release is safe.
	_, err = s.DB.Exec(`UPDATE workflow_steps SET result=? WHERE run_id=? AND seq=1`, `{"route":"approved","summary":"historical","artifacts":["report.txt"]}`, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.ReleaseSandbox(context.Background(), c, r.ID); err == nil {
		t.Fatal("released without retained copy")
	}
	if deletes.Load() != 0 {
		t.Fatal("contacted remote delete before archive check")
	}
}
func TestWorkflowArchiveVersionsSurviveEngineRecreation(t *testing.T) {
	s := testStore(t)
	c, _, r := runFixture(t, s)
	e := NewWorkflowEngine(s, nil, "")
	for i, route := range []string{"changes", "ready", "approved"} {
		text := fmt.Sprintf("version-%d", i+1)
		os.WriteFile(filepath.Join(r.WorkspacePath, "report.txt"), []byte(text), 0600)
		if err := s.CompleteWorkflowNode(c, r.ID, i+1, NodeResult{Route: route, Summary: text, Artifacts: []string{"report.txt"}}); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			e.Tick(context.Background())
			r, _ = s.WorkflowRun(c, r.ID)
			if r.Seq == i+2 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if r.Seq != i+2 {
			t.Fatal("archive did not advance", r.Status, r.Error)
		}
	}
	if err := s.requireWorkflowArchives(r); err != nil {
		t.Fatal(err)
	}
	// A new engine must consume immutable snapshots without the source files.
	os.RemoveAll(r.WorkspacePath)
	e = NewWorkflowEngine(s, nil, "")
	for seq := 1; seq <= 3; seq++ {
		b, err := e.store.archivedContent(r, seq, "report.txt")
		if err != nil || string(b) != fmt.Sprintf("version-%d", seq) {
			t.Fatal("overwrote prior version", seq, string(b), err)
		}
	}
}
