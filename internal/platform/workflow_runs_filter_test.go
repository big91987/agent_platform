package platform

import (
	"encoding/json"
	"testing"
)

func TestWorkflowRunsFilterByWorkflowAndOwner(t *testing.T) {
	s := testStore(t)
	c, w, r := runFixture(t, s)
	other := w
	other.ID = ""
	other.Revision = 0
	other.Name = "Another workflow"
	other, err := s.SaveWorkflow(c, other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.StartWorkflow(c, WorkflowStart{WorkflowID: other.ID, Input: "other", WorkspacePath: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	runs, err := s.WorkflowRunsFor(c, w.ID)
	if err != nil || len(runs) != 1 || runs[0].ID != r.ID {
		t.Fatalf("wrong workflow runs: %+v %v", runs, err)
	}
	runs, err = s.WorkflowRunsFor(Caller{UserID: "unrelated"}, w.ID)
	if err != nil || len(runs) != 0 {
		t.Fatalf("owner boundary lost: %+v %v", runs, err)
	}
}

func TestWorkflowRunPagesPreserveOlderRunsWhenNewRunsArrive(t *testing.T) {
	s := testStore(t)
	c, w, oldest := runFixture(t, s)
	for i := 0; i < 200; i++ {
		if _, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "page fixture", WorkspacePath: t.TempDir()}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.WorkflowRunsPage(c, w.ID, "")
	if err != nil || len(first) != 200 {
		t.Fatalf("first page: %d %v", len(first), err)
	}
	newest, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "arrived during pagination", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.WorkflowRunsPage(c, w.ID, first[len(first)-1].ID)
	if err != nil || len(second) != 1 || second[0].ID != oldest.ID {
		t.Fatalf("older run lost: %+v %v", second, err)
	}
	for _, run := range second {
		if run.ID == newest.ID {
			t.Fatal("new run crossed cursor")
		}
	}
	h := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, h)
	response := workflowRequest(t, h, cookie, "GET", "/api/workflow-runs?workflow_id="+w.ID+"&before="+first[len(first)-1].ID, nil)
	var page []WorkflowRun
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != 200 || len(page) != 1 || page[0].ID != oldest.ID {
		t.Fatalf("HTTP cursor ignored: %d %s %v", response.Code, response.Body.String(), err)
	}
	other, err := s.WorkflowRunsPage(Caller{UserID: "unrelated"}, w.ID, first[len(first)-1].ID)
	if err != nil || len(other) != 0 {
		t.Fatalf("cross-user page: %+v %v", other, err)
	}
}

func TestRevokedWorkflowStillListsOwnedRunHistory(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	alice, aliceToken := testUser(t, s, &a, "alice")
	_, bobToken := testUser(t, s, &a, "bob")
	h := NewServer(s, nil, nil, "password", "http://localhost")
	adminCookie := workflowAdmin(t, h)
	spec := workflowFixture()
	spec["authorized_users"] = []string{alice.UserID}
	response := workflowRequest(t, h, adminCookie, "POST", "/api/workflows", spec)
	if response.Code != 201 {
		t.Fatal(response.Code, response.Body.String())
	}
	graph := workflowJSON(t, response)
	id := graph["id"].(string)
	response = requestJSON(t, h, "POST", "/api/workflow-runs", aliceToken, WorkflowStart{WorkflowID: id, Input: "保留历史", WorkspacePath: t.TempDir()})
	if response.Code != 201 {
		t.Fatal(response.Code, response.Body.String())
	}
	runID := workflowJSON(t, response)["id"].(string)
	graph["authorized_users"] = []string{}
	response = workflowRequest(t, h, adminCookie, "PUT", "/api/workflows/"+id, graph)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	response = requestJSON(t, h, "GET", "/api/workflows/"+id, aliceToken, nil)
	if response.Code != 403 {
		t.Fatalf("revoked workflow access: %d", response.Code)
	}
	for _, path := range []string{"/api/workflow-runs/" + runID, "/api/workflow-runs?workflow_id=" + id} {
		response = requestJSON(t, h, "GET", path, aliceToken, nil)
		if response.Code != 200 {
			t.Fatalf("owned history %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	response = requestJSON(t, h, "GET", "/api/workflow-run-groups", aliceToken, nil)
	var groups []WorkflowRunGroup
	if err := json.Unmarshal(response.Body.Bytes(), &groups); err != nil || response.Code != 200 || len(groups) != 1 || groups[0].ID != id || groups[0].Name != "文稿审核" {
		t.Fatalf("frozen group: %d %s %v", response.Code, response.Body.String(), err)
	}
	response = requestJSON(t, h, "GET", "/api/workflow-run-groups", bobToken, nil)
	if response.Code != 200 || response.Body.String() != "[]\n" {
		t.Fatalf("other user's history leaked: %d %s", response.Code, response.Body.String())
	}
}
