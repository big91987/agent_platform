package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWorkflowDeletePreservesHistoryAndRejectsActiveOrStale(t *testing.T) {
	s := testStore(t)
	c, w, r := runFixture(t, s)
	h := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, h)
	remove := func(revision int64) int {
		return workflowRequest(t, h, cookie, "DELETE", "/api/workflows/"+w.ID, map[string]int64{"revision": revision}).Code
	}
	if got := remove(w.Revision); got != 409 {
		t.Fatalf("active run deletion: %d", got)
	}
	if err := s.CompleteWorkflowNode(c, r.ID, 1, NodeResult{Route: "approved", Summary: "retain this evidence"}); err != nil {
		t.Fatal(err)
	}
	if err := s.advanceWorkflow(r.ID, 1); err != nil {
		t.Fatal(err)
	}
	before, err := s.WorkflowRun(c, r.ID)
	if err != nil || before.Status != "completed" {
		t.Fatal(before, err)
	}
	w.Name = "updated"
	w, err = s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	if got := remove(w.Revision - 1); got != 409 {
		t.Fatalf("stale deletion: %d", got)
	}
	a := testAgent(t, s)
	_, token := testUser(t, s, &a, "non-admin")
	if got := requestJSON(t, h, "DELETE", "/api/workflows/"+w.ID, token, map[string]int64{"revision": w.Revision}).Code; got != 403 {
		t.Fatalf("non-admin deletion: %d", got)
	}
	if got := remove(w.Revision); got != 200 {
		t.Fatalf("delete: %d", got)
	}
	if got := remove(w.Revision); got != 404 {
		t.Fatalf("repeat delete: %d", got)
	}
	if got := workflowRequest(t, h, cookie, "GET", "/api/workflows/"+w.ID, nil).Code; got != 404 {
		t.Fatalf("still exists: %d", got)
	}
	after, err := s.WorkflowRun(c, r.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("history changed", err)
	}
	groups, err := s.WorkflowRunGroups(c)
	if err != nil || len(groups) != 1 || groups[0].ID != w.ID {
		t.Fatal(groups, err)
	}
	if _, err = s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "new", WorkspacePath: t.TempDir()}); err != ErrNotFound {
		t.Fatal("new run accepted", err)
	}
	if _, err = s.SaveWorkflow(c, w); err == nil {
		t.Fatal("stale save resurrected deleted workflow")
	}
	response := workflowRequest(t, h, cookie, "GET", "/api/workflows", nil)
	var items []Workflow
	if err = json.Unmarshal(response.Body.Bytes(), &items); err != nil || len(items) != 0 {
		t.Fatal(response.Body.String(), err)
	}
}

func TestWorkflowDeleteRunStates(t *testing.T) {
	for _, status := range []string{"running", "waiting", "stopping", "queued", "stopped", "failed", "cancelled", "completed"} {
		t.Run(status, func(t *testing.T) {
			s := testStore(t)
			c, w, r := runFixture(t, s)
			// Isolate lifecycle eligibility without starting native executors.
			if _, err := s.DB.Exec(`UPDATE workflow_runs SET status=? WHERE id=?`, status, r.ID); err != nil {
				t.Fatal(err)
			}
			err := s.DeleteWorkflow(c, w.ID, w.Revision)
			terminal := status == "stopped" || status == "failed" || status == "cancelled" || status == "completed"
			if (err == nil) != terminal {
				t.Fatalf("status %s: %v", status, err)
			}
			preserved, loadErr := s.WorkflowRun(c, r.ID)
			if loadErr != nil || preserved.Status != status {
				t.Fatal("run changed", loadErr)
			}
		})
	}
}

func TestWorkflowDeletedNodeHistoryRetainsCurrentGrant(t *testing.T) {
	for _, revoked := range []bool{false, true} {
		t.Run(map[bool]string{false: "owner", true: "revoked"}[revoked], func(t *testing.T) {
			s := testStore(t)
			s.initAuth("password")
			admin, _ := s.userCaller("admin")
			u, err := s.SaveUser(User{Username: "reader", UserID: "reader", Role: "caller", Enabled: true, Password: "test-password-123"})
			if err != nil {
				t.Fatal(err)
			}
			caller, _ := s.userCaller(u.Username)
			w := independentWorkflow(t, map[string]any{"executor": "codex"})
			w.AuthorizedUsers = []string{u.UserID}
			w, err = s.SaveWorkflow(admin, w)
			if err != nil {
				t.Fatal(err)
			}
			r, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: w.ID, Input: "retain history", WorkspacePath: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			engine := NewWorkflowEngine(s, nil, "http://localhost")
			engine.Tick(context.Background())
			conv, msg, err := s.Claim()
			if err != nil {
				t.Fatal(err)
			}
			if err = s.testHandoffResult(r.ID, 1, *conv.Snapshot.Env[workflowTokenEnv], NodeResult{Summary: "verified"}); err != nil {
				t.Fatal(err)
			}
			if err = s.Complete(conv.ID, msg.ID, "completed", ""); err != nil {
				t.Fatal(err)
			}
			engine.Tick(context.Background())
			before, _ := s.WorkflowRun(admin, r.ID)
			if before.Status != "completed" {
				t.Fatal(before.Status)
			}
			if revoked {
				w.AuthorizedUsers = nil
				w, err = s.SaveWorkflow(admin, w)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err = s.DeleteWorkflow(admin, w.ID, w.Revision); err != nil {
				t.Fatal(err)
			}
			_, err = s.Authorize(caller, conv.ID, "")
			if (err == nil) == revoked {
				t.Fatalf("history grant revoked=%v: %v", revoked, err)
			}
			if _, err = s.Authorize(Caller{UserID: "unrelated"}, conv.ID, ""); !errors.Is(err, ErrForbidden) {
				t.Fatal(err)
			}
			if err = engine.Return(admin, r.ID, before.Seq, "worker", "check old evidence"); err == nil {
				t.Fatal("deleted workflow resumed")
			}
			if _, err = s.Submit(caller, Input{ConversationID: conv.ID, Message: "resume"}); err == nil {
				t.Fatal("history write accepted")
			}
			after, _ := s.WorkflowRun(admin, r.ID)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("history changed")
			}
		})
	}
}
func TestDeletedWorkflowRejectsStoppedReturnAndResume(t *testing.T) {
	s := testStore(t)
	c, w, r := runFixture(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	if err := engine.Stop(c, r.ID, r.Seq); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	before, _ := s.WorkflowRun(c, r.ID)
	if before.Status != "stopped" {
		t.Fatal(before.Status)
	}
	if err := s.DeleteWorkflow(c, w.ID, w.Revision); err != nil {
		t.Fatal(err)
	}
	if err := engine.Return(c, r.ID, r.Seq, "review", "explicit return"); err == nil {
		t.Fatal("return accepted")
	}
	if err := engine.Resume(c, r.ID, r.Seq, "explicit resume"); err == nil {
		t.Fatal("resume accepted")
	}
	after, _ := s.WorkflowRun(c, r.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("history changed")
	}
}

func TestWorkflowDeleteMigratesExistingDefinitionsAndSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(dir, "platform.db"))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(workflowFixture())
	var w Workflow
	json.Unmarshal(raw, &w)
	w.ID = newID()
	w.Revision = 1
	raw, _ = json.Marshal(w)
	if _, err = db.Exec(`CREATE TABLE workflows(id TEXT PRIMARY KEY,revision INTEGER NOT NULL,definition TEXT NOT NULL); INSERT INTO workflows VALUES(?,?,?)`, w.ID, w.Revision, string(raw)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	admin := Caller{Admin: true}
	before, err := s.Workflow(admin, w.ID)
	if err != nil || !reflect.DeepEqual(before, w) {
		t.Fatal("migration changed definition", err)
	}
	if err = s.DeleteWorkflow(admin, w.ID, w.Revision); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Workflow(admin, w.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted definition returned after restart", err)
	}
}
