package platform

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUnifiedHandoffMCPRequiresTargetAndTypedContext(t *testing.T) {
	s := testStore(t)
	c, r := inputProtocolRun(t, s, 2)
	h := NewServer(s, nil, nil, "password", "http://localhost")
	remote := httptest.NewServer(h)
	defer remote.Close()
	h.workflows.base = remote.URL
	h.workflows.Tick(context.Background())
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "unified", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: remote.URL + "/api/workflow-node/mcp", HTTPClient: &http.Client{Transport: bearerTransport{http.DefaultTransport, *conv.Snapshot.Env[workflowTokenEnv]}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range list.Tools {
		if tool.Name == "complete_node" {
			t.Fatal("obsolete completion tool exposed")
		}
		if tool.Name == "handoff" {
			found = true
			b, _ := json.Marshal(tool.InputSchema)
			if !strings.Contains(string(b), `"message"`) || !strings.Contains(string(b), `"context"`) {
				t.Fatalf("missing unified fields: %s", b)
			}
		}
	}
	if !found {
		t.Fatal("handoff missing")
	}
	for _, args := range []string{`{"message":"done"}`, `{"target":"missing","message":"done"}`, `{"target":"done","summary":"old"}`, `{"target":"done","message":"done","route":"next"}`, `{"target":"done","message":" "}`} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "handoff", Arguments: json.RawMessage(args)})
		if err == nil && !result.IsError {
			t.Fatalf("accepted invalid arguments %s", args)
		}
		fresh, _ := s.WorkflowRun(c, r.ID)
		if fresh.Steps[0].Result != nil {
			t.Fatal("invalid call persisted")
		}
	}
	args := json.RawMessage(`{"target":"done","message":"verified","context":{"ready":true,"count":2,"files":["a.txt"]}}`)
	for i := 0; i < 2; i++ {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "handoff", Arguments: args})
		if err != nil || result.IsError {
			t.Fatalf("valid handoff %v %v", result, err)
		}
	}
	h.workflows.Tick(ctx)
	fresh, _ := s.WorkflowRun(c, r.ID)
	if fresh.Seq != 1 {
		t.Fatal("advanced while native running")
	}
	if fresh.Steps[0].Result.Inputs["ready"] != true {
		t.Fatal("lost typed context")
	}
	if err = s.Complete(conv.ID, msg.ID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	h.workflows.Tick(ctx)
	fresh, _ = s.WorkflowRun(c, r.ID)
	if fresh.Status != "completed" {
		t.Fatal(fresh.Status)
	}
}

// Existing lifecycle regressions express expected persisted results. Convert
// their fixture route into an explicit target before exercising handoff.
func (s *Store) testHandoffResult(id string, seq int, token string, result NodeResult) error {
	r, err := s.WorkflowRun(Caller{Admin: true}, id)
	if err != nil {
		return err
	}
	target := "done"
	for _, step := range r.Steps {
		if step.Seq == seq && result.Route != "" {
			target = r.Definition.target(step.NodeID, result.Route)
		}
	}
	message := result.Summary
	if message == "" {
		message = "fixture handoff"
	}
	return s.handoffWorkflowNode(id, seq, token, HandoffInput{Target: target, Message: message, Context: result.Inputs, Artifacts: result.Artifacts})
}

func TestRetiredHandoffConfigurationRemainsReadableButCannotExecute(t *testing.T) {
	raw := []byte(`{"id":"worker","kind":"agent","exit_mode":"complete","completion_schema":{"type":"object","properties":{"ready":{"type":"boolean"}}},"completion_instructions":"Keep evidence"}`)
	var node WorkflowNode
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatal(err)
	}
	stored, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}
	var before, after map[string]any
	json.Unmarshal(raw, &before)
	json.Unmarshal(stored, &after)
	for key, value := range before {
		a, _ := json.Marshal(value)
		b, _ := json.Marshal(after[key])
		if string(a) != string(b) {
			t.Fatalf("historical field lost: %s", key)
		}
	}
	graph := Workflow{Nodes: []WorkflowNode{node, {ID: "done", Kind: "end"}}, Edges: []WorkflowEdge{{Source: "worker", Route: "next", Target: "done", Mode: "handoff"}}}
	if err := validateCompletionConfig(graph, node); err == nil {
		t.Fatal("retired configuration accepted")
	}
	if _, err := handoffToolSchema(graph, node); err == nil {
		t.Fatal("retired instructions silently replaced by new tool")
	}
}

// Seed an on-disk historical contract as it would exist before an upgrade.
func retireHandoffRunFixture(t *testing.T, s *Store, r WorkflowRun) {
	t.Helper()
	r.Definition.Nodes[0].ExitMode = "complete"
	raw, err := json.Marshal(r.Definition)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(`UPDATE workflow_runs SET definition=? WHERE id=?`, string(raw), r.ID); err != nil {
		t.Fatal(err)
	}
}

func TestRetiredRunCannotContinueSubmitAdvanceOrReturn(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(map[bool]string{false: "waiting", true: "accepted result"}[accepted], func(t *testing.T) {
			s := testStore(t)
			caller, r := inputProtocolRun(t, s, 2)
			e := NewWorkflowEngine(s, nil, "http://localhost")
			e.Tick(context.Background())
			conv, msg, err := s.Claim()
			if err != nil {
				t.Fatal(err)
			}
			token := *conv.Snapshot.Env[workflowTokenEnv]
			if accepted {
				err = s.handoffWorkflowNode(r.ID, 1, token, HandoffInput{Target: "done", Message: "finished"})
			} else {
				err = s.waitWorkflowNode(r.ID, 1, token, WorkflowWait{Kind: "clarification", Reason: "confirm task"})
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Complete(conv.ID, msg.ID, "completed", ""); err != nil {
				t.Fatal(err)
			}
			if !accepted {
				e.Tick(context.Background())
			}
			retireHandoffRunFixture(t, s, r)
			before, _ := s.WorkflowRun(caller, r.ID)
			messagesBefore, _ := s.Messages(conv.ID)
			e.Tick(context.Background())
			after, _ := s.WorkflowRun(caller, r.ID)
			messagesAfter, _ := s.Messages(conv.ID)
			a, _ := json.Marshal(before)
			b, _ := json.Marshal(after)
			if string(a) != string(b) || len(messagesBefore) != len(messagesAfter) {
				t.Fatal("background scheduling changed historical Run")
			}
			if !accepted {
				if _, err = s.Submit(caller, Input{ConversationID: conv.ID, UserID: caller.UserID, Message: "continue"}); !errors.Is(err, errRetiredAgentHandoff) {
					t.Fatalf("input allowed: %v", err)
				}
				if err = s.continueWorkflowNode(r.ID, 1); !errors.Is(err, errRetiredAgentHandoff) {
					t.Fatalf("continuation allowed: %v", err)
				}
			}
			if err = s.advanceWorkflow(r.ID, 1); !errors.Is(err, errRetiredAgentHandoff) {
				t.Fatalf("advance allowed: %v", err)
			}
			if err = s.CompleteWorkflowNode(caller, r.ID, 1, NodeResult{Route: "done"}); !errors.Is(err, errRetiredAgentHandoff) {
				t.Fatalf("decision allowed: %v", err)
			}
			if err = e.Stop(caller, r.ID, 1); err != nil {
				t.Fatal(err)
			}
			e.Tick(context.Background())
			if err = e.Return(caller, r.ID, 1, "done", "review"); !errors.Is(err, errRetiredAgentHandoff) {
				t.Fatalf("return allowed: %v", err)
			}
		})
	}
}

func TestClaimSkipsRetiredRunWithoutStarvingCurrentWork(t *testing.T) {
	s := testStore(t)
	caller, old := inputProtocolRun(t, s, 2)
	e := NewWorkflowEngine(s, nil, "http://localhost")
	e.Tick(context.Background())
	old, _ = s.WorkflowRun(caller, old.ID)
	oldID := old.Steps[0].ConversationID
	retireHandoffRunFixture(t, s, old)
	next, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: old.WorkflowID, Input: "new current task", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	e.Tick(context.Background())
	next, _ = s.WorkflowRun(caller, next.ID)
	conv, _, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	if conv.ID != next.Steps[0].ConversationID {
		t.Fatal("claimed historical queued execution")
	}
	oldConversation, err := s.Conversation(oldID)
	if err != nil || oldConversation.Status != "queued" {
		t.Fatal("modified historical queued conversation", err)
	}
}
