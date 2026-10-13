package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func sandboxWorkflowFixture(t *testing.T, s *Store, lost, lostRelease bool) (Caller, Workflow, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	if err := s.initAuth("password"); err != nil {
		t.Fatal(err)
	}
	caller, err := s.userCaller("admin")
	if err != nil {
		t.Fatal(err)
	}
	var posts, deletes atomic.Int32
	var mu sync.Mutex
	environments := map[string]opensandbox.SandboxInfo{}
	var base string
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == "POST" && r.URL.Path == "/sandboxes":
			posts.Add(1)
			var in opensandbox.CreateSandboxRequest
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			run := in.Metadata["agent-platform-run"]
			info := opensandbox.SandboxInfo{ID: run, Metadata: in.Metadata, Status: opensandbox.SandboxStatus{State: opensandbox.StateRunning}}
			environments[run] = info
			if lost {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				conn.Close()
				return
			}
			json.NewEncoder(w).Encode(info)
		case r.Method == "GET" && r.URL.Path == "/sandboxes":
			var list []opensandbox.SandboxInfo
			for run, env := range environments {
				if r.URL.Query().Get("metadata") == "agent-platform-run="+run {
					list = append(list, env)
				}
			}
			json.NewEncoder(w).Encode(opensandbox.ListSandboxesResponse{Items: list})
		case r.URL.Path == "/ping":
			w.Write([]byte("pong"))
		default:
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			if len(parts) < 2 || parts[0] != "sandboxes" {
				w.WriteHeader(404)
				return
			}
			env, ok := environments[parts[1]]
			if !ok {
				w.WriteHeader(404)
				return
			}
			if len(parts) > 2 {
				json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
				return
			}
			if r.Method == "DELETE" {
				delete(environments, parts[1])
				deletes.Add(1)
				if lostRelease {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					conn.Close()
					return
				}
				w.WriteHeader(204)
				return
			}
			json.NewEncoder(w).Encode(env)
		}
	}))
	t.Cleanup(remote.Close)
	base = remote.URL
	client, err := newOpenSandbox(base, "private-test-secret", "python:3.12-slim", 600)
	if err != nil {
		t.Fatal(err)
	}
	s.sandbox = client
	graph, err := s.SaveWorkflow(caller, Workflow{Name: "Isolated environment", Environment: "opensandbox", Enabled: true, Entry: "review", MaxSteps: 10, ContextVersion: 2, Nodes: []WorkflowNode{{ID: "review", Name: "Review", Kind: "approval"}, {ID: "done", Name: "Done", Kind: "end"}}, Edges: []WorkflowEdge{{Source: "review", Route: "next", Target: "done"}}})
	if err != nil {
		t.Fatal(err)
	}
	return caller, graph, &posts, &deletes
}

func waitSandboxRun(t *testing.T, e *WorkflowEngine, c Caller, id, status string) WorkflowRun {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		e.Tick(context.Background())
		run, err := e.store.WorkflowRun(c, id)
		if err != nil {
			t.Fatal(err)
		}
		if run.Environment != nil && run.Environment.Status == status {
			return run
		}
		time.Sleep(10 * time.Millisecond)
	}
	run, _ := e.store.WorkflowRun(c, id)
	t.Fatalf("environment did not reach %s: %+v", status, run.Environment)
	return run
}

func TestWorkflowSandboxDeduplicatesAndSeparatesRunEnvironments(t *testing.T) {
	s := testStore(t)
	caller, graph, posts, _ := sandboxWorkflowFixture(t, s, false, false)
	input := WorkflowStart{WorkflowID: graph.ID, Input: "independent task", RequestID: "one"}
	first, err := s.StartWorkflow(caller, input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.StartWorkflow(caller, input)
	if err != nil || again.ID != first.ID {
		t.Fatal("duplicate Run", err)
	}
	input.RequestID = "two"
	second, err := s.StartWorkflow(caller, input)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "")
	first = waitSandboxRun(t, engine, caller, first.ID, "ready")
	second = waitSandboxRun(t, engine, caller, second.ID, "ready")
	if posts.Load() != 2 || first.Environment.ID == second.Environment.ID || first.WorkspacePath != "/workspace" || second.WorkspacePath != "/workspace" {
		t.Fatal("environment isolation lost")
	}
	if err := engine.ReleaseSandbox(context.Background(), Caller{UserID: "outsider"}, first.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign release accepted", err)
	}
	if err := engine.ReleaseSandbox(context.Background(), caller, first.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("active release accepted", err)
	}
	if err := engine.Stop(caller, first.ID, 1); err != nil {
		t.Fatal(err)
	}
	waitConnectorRun(t, engine, first.ID, "stopped")
	if err := engine.ReleaseSandbox(context.Background(), caller, first.ID); err != nil {
		t.Fatal(err)
	}
	first, _ = s.WorkflowRun(caller, first.ID)
	if first.Environment.Status != "released" {
		t.Fatal(first.Environment)
	}
	if err := engine.Resume(caller, first.ID, 1, "continue"); err == nil {
		t.Fatal("released environment resumed")
	}
	if err := engine.Return(caller, first.ID, 1, "review", "inspect then retry"); err == nil || !strings.Contains(err.Error(), "environment is not ready") {
		t.Fatal("released environment accepted a return")
	}
	second = waitSandboxRun(t, engine, caller, second.ID, "ready")
	if posts.Load() != 2 {
		t.Fatal("creation replayed")
	}
}

func TestWorkflowSandboxLostCreateSurvivesRestartWithoutRepost(t *testing.T) {
	s := testStore(t)
	caller, graph, posts, _ := sandboxWorkflowFixture(t, s, true, false)
	run, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: graph.ID, Input: "create response lost"})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "")
	run = waitSandboxRun(t, engine, caller, run.ID, "unconfirmed")
	if posts.Load() != 1 {
		t.Fatal("create retried")
	}
	directory := s.Dir
	client := s.sandbox
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.sandbox = client
	engine = NewWorkflowEngine(reopened, nil, "")
	engine.Tick(context.Background())
	if posts.Load() != 1 {
		t.Fatal("restart reposted create")
	}
	if err = engine.ReconcileSandbox(context.Background(), caller, run.ID); err != nil {
		t.Fatal(err)
	}
	run, _ = reopened.WorkflowRun(caller, run.ID)
	if run.Environment.ID != run.ID || run.Environment.Status != "ready" || posts.Load() != 1 {
		t.Fatal("read-only reconciliation failed", run.Environment)
	}
}

func TestWorkflowSandboxUnknownReleaseReconcilesWithoutAnotherDelete(t *testing.T) {
	s := testStore(t)
	caller, graph, posts, deletes := sandboxWorkflowFixture(t, s, false, true)
	run, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: graph.ID, Input: "release response lost"})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWorkflowEngine(s, nil, "")
	run = waitSandboxRun(t, engine, caller, run.ID, "ready")
	if err = engine.Stop(caller, run.ID, 1); err != nil {
		t.Fatal(err)
	}
	waitConnectorRun(t, engine, run.ID, "stopped")
	if err = engine.ReleaseSandbox(context.Background(), caller, run.ID); err == nil {
		t.Fatal("lost release receipt was accepted")
	}
	run, _ = s.WorkflowRun(caller, run.ID)
	if run.Environment.Status != "release_unconfirmed" {
		t.Fatal(run.Environment)
	}
	if err = engine.ReleaseSandbox(context.Background(), caller, run.ID); err == nil {
		t.Fatal("unknown delete replayed")
	}
	if err = engine.ReconcileSandbox(context.Background(), caller, run.ID); err != nil {
		t.Fatal(err)
	}
	run, _ = s.WorkflowRun(caller, run.ID)
	found := false
	for _, event := range run.Environment.History {
		if event.Status == "release_unconfirmed" && event.Error != "" {
			found = true
		}
	}
	if run.Environment.Status != "released" || !found || posts.Load() != 1 || deletes.Load() != 1 {
		t.Fatal("history or exactly-once deletion lost", run.Environment, posts.Load(), deletes.Load())
	}
	raw, _ := json.Marshal(run)
	if strings.Contains(string(raw), "connection_hash") || strings.Contains(string(raw), "private-test-secret") {
		t.Fatal("private service identity leaked")
	}
}

func TestWorkflowSandboxRestartBetweenIntentAndDispatchDoesNotCreate(t *testing.T) {
	s := testStore(t)
	caller, graph, posts, _ := sandboxWorkflowFixture(t, s, false, false)
	run, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: graph.ID, Input: "crash after durable intent"})
	if err != nil {
		t.Fatal(err)
	}
	// Crash boundary: use the production transition, before its external POST.
	if err = s.updateRunEnvironment(run.ID, "pending", "", "creating", ""); err != nil {
		t.Fatal(err)
	}
	directory, client := s.Dir, s.sandbox
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.sandbox = client
	engine := NewWorkflowEngine(reopened, nil, "")
	engine.Tick(context.Background())
	current, err := reopened.WorkflowRun(caller, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 0 || current.Status != "failed" || current.Environment.Status != "unconfirmed" {
		t.Fatal("restart created an environment", current.Environment, posts.Load())
	}
	if err = engine.ReconcileSandbox(context.Background(), caller, run.ID); err == nil {
		t.Fatal("missing uncertain environment accepted")
	}
	engine.Tick(context.Background())
	if posts.Load() != 0 {
		t.Fatal("reconciliation reposted create")
	}
}

func TestWorkflowSandboxRejectsHostPathAndChangedService(t *testing.T) {
	s := testStore(t)
	caller, graph, posts, _ := sandboxWorkflowFixture(t, s, false, false)
	if _, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: graph.ID, Input: "host path", WorkspacePath: t.TempDir()}); err == nil {
		t.Fatal("host path accepted")
	}
	run, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: graph.ID, Input: "freeze connection"})
	if err != nil {
		t.Fatal(err)
	}
	original := s.sandbox
	changed, err := newOpenSandbox("http://127.0.0.1:1", "different-identity", "python:3.12-slim", 600)
	if err != nil {
		t.Fatal(err)
	}
	s.sandbox = changed
	engine := NewWorkflowEngine(s, nil, "")
	engine.Tick(context.Background())
	current, _ := s.WorkflowRun(caller, run.ID)
	if current.Status != "failed" || !strings.Contains(current.Error, "configuration changed") || posts.Load() != 0 {
		t.Fatal("connection identity was replaced", current.Error, posts.Load())
	}
	s.sandbox = original
}

func TestWorkflowSandboxEnvironmentHTTPPreservesOwnerAuthorization(t *testing.T) {
	s := testStore(t)
	caller, graph, _, _ := sandboxWorkflowFixture(t, s, false, false)
	server := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, server)
	response := workflowRequest(t, server, cookie, "POST", "/api/workflow-runs", WorkflowStart{WorkflowID: graph.ID, Input: "supported HTTP lifecycle", RequestID: "http-environment"})
	if response.Code != 201 {
		t.Fatal(response.Code, response.Body.String())
	}
	var run WorkflowRun
	if err := json.Unmarshal(response.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	run = waitSandboxRun(t, server.workflows, caller, run.ID, "ready")
	response = workflowRequest(t, server, cookie, "POST", "/api/workflow-runs/"+run.ID+"/environment/release", nil)
	if response.Code != 409 {
		t.Fatal("HTTP released active environment", response.Code)
	}
	response = workflowRequest(t, server, cookie, "POST", "/api/workflow-runs/"+run.ID+"/stop", map[string]any{"seq": 1})
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	waitConnectorRun(t, server.workflows, run.ID, "stopped")
	response = workflowRequest(t, server, cookie, "POST", "/api/workflow-runs/"+run.ID+"/environment/reconcile", nil)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	response = workflowRequest(t, server, cookie, "POST", "/api/workflow-runs/"+run.ID+"/environment/release", nil)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if run.Environment.Status != "released" || len(run.Environment.History) < 5 {
		t.Fatal("HTTP lost environment evidence", run.Environment)
	}
	response = requestJSON(t, server, "POST", "/api/workflow-runs/"+run.ID+"/environment/release", "", nil)
	if response.Code != 401 {
		t.Fatal("anonymous release", response.Code)
	}
}

func TestWorkflowSandboxStoppedBeforeCreationCanResumeOrReleaseWithoutSideEffects(t *testing.T) {
	for _, resume := range []bool{true, false} {
		t.Run(fmt.Sprint(resume), func(t *testing.T) {
			s := testStore(t)
			caller, graph, posts, _ := sandboxWorkflowFixture(t, s, false, false)
			run, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: graph.ID, Input: "stop before environment dispatch"})
			if err != nil {
				t.Fatal(err)
			}
			engine := NewWorkflowEngine(s, nil, "")
			if err = engine.Stop(caller, run.ID, 1); err != nil {
				t.Fatal(err)
			}
			waitConnectorRun(t, engine, run.ID, "stopped")
			if posts.Load() != 0 {
				t.Fatal("stop dispatched creation")
			}
			if resume {
				if err = engine.Resume(caller, run.ID, 1, "begin preparation"); err != nil {
					t.Fatal(err)
				}
				waitSandboxRun(t, engine, caller, run.ID, "ready")
				if posts.Load() != 1 {
					t.Fatal("resume duplicated creation")
				}
			}
			if !resume {
				if err = engine.ReleaseSandbox(context.Background(), caller, run.ID); err != nil {
					t.Fatal(err)
				}
				current, _ := s.WorkflowRun(caller, run.ID)
				if current.Environment.Status != "released" || current.Environment.ID != "" || posts.Load() != 0 {
					t.Fatal("release created a sandbox")
				}
			}
		})
	}
}

func TestWorkflowSandboxUpgradeUsesRunFrozenImageAndTTL(t *testing.T) {
	s := testStore(t)
	caller, graph, _, _ := sandboxWorkflowFixture(t, s, false, false)
	run, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: graph.ID, Input: "keep pending environment configuration"})
	if err != nil {
		t.Fatal(err)
	}
	next := *s.sandbox
	next.image = "different-runtime"
	next.ttl = 3600
	s.sandbox = &next
	client, err := s.sandboxFor(run)
	if err != nil {
		t.Fatal(err)
	}
	if client.image != run.Environment.Image || client.ttl != run.Environment.TTL {
		t.Fatal("frozen environment options changed")
	}
	waitSandboxRun(t, NewWorkflowEngine(s, nil, ""), caller, run.ID, "ready")
}
