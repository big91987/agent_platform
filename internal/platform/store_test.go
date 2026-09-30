package platform

import (
	"errors"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, e := OpenStore(filepath.Join(t.TempDir(), "data"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func testAgent(t *testing.T, s *Store) Agent {
	t.Helper()
	a := Agent{Name: "test", Executor: "codex", Enabled: true, InheritEnv: true, Sandbox: "workspace-write"}
	a, e := s.SaveAgent(a)
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func TestInputDeduplicatesAndRejectsChangedPayload(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	c := Caller{Source: "system-a", Agents: []string{a.ID}}
	in := Input{AgentID: a.ID, UserID: "alice", Message: "hello", RequestID: "event-1"}
	r, e := s.Submit(c, in)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.Submit(c, in)
	if e != nil || again.ConversationID != r.ConversationID || again.MessageID != r.MessageID || !again.Duplicate {
		t.Fatalf("duplicate: %+v %v", again, e)
	}
	in.Message = "different"
	if _, e = s.Submit(c, in); !errors.Is(e, ErrConflict) {
		t.Fatalf("changed payload accepted: %v", e)
	}
	msgs, _ := s.Messages(r.ConversationID)
	if len(msgs) != 1 {
		t.Fatalf("replayed input: %d", len(msgs))
	}
}
func TestConversationCannotCrossSourceOrUser(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	c := Caller{Source: "a", Agents: []string{a.ID}}
	r, e := s.Submit(c, Input{AgentID: a.ID, UserID: "alice", Message: "private"})
	if e != nil {
		t.Fatal(e)
	}
	for _, attempt := range []struct {
		caller Caller
		user   string
	}{{Caller{Source: "b", Agents: []string{a.ID}}, "alice"}, {c, "bob"}} {
		if _, e := s.Authorize(attempt.caller, r.ConversationID, attempt.user); !errors.Is(e, ErrForbidden) {
			t.Fatalf("leaked conversation: %v", e)
		}
	}
}
func TestClaimSerializesConversationAndPreservesQueueAfterStop(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	c := Caller{Source: "console", Admin: true}
	r, _ := s.Submit(c, Input{AgentID: a.ID, UserID: "operator", Message: "first"})
	second, _ := s.Submit(c, Input{ConversationID: r.ConversationID, UserID: "operator", Message: "second"})
	conv, msg, e := s.Claim()
	if e != nil || msg.ID != r.MessageID {
		t.Fatalf("claim first: %+v %v", msg, e)
	}
	if _, _, e = s.Claim(); !errors.Is(e, ErrNotFound) {
		t.Fatalf("parallel execution: %v", e)
	}
	if e = s.Halt(conv.ID, "stopped", "user stop"); e != nil {
		t.Fatal(e)
	}
	s.Complete(conv.ID, msg.ID, "completed", "")
	if _, _, e = s.Claim(); !errors.Is(e, ErrNotFound) {
		t.Fatalf("queue started after stop: %v", e)
	}
	if e = s.Continue(conv.ID); e != nil {
		t.Fatal(e)
	}
	_, msg, e = s.Claim()
	if e != nil || msg.ID != second.MessageID {
		t.Fatalf("lost queued input: %+v %v", msg, e)
	}
}
func TestRestartRetainsNativeThreadAndDoesNotReplayRunningInput(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	s, e := OpenStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	a := testAgent(t, s)
	r, _ := s.Submit(Caller{Source: "console", Admin: true}, Input{AgentID: a.ID, UserID: "operator", Message: "write something"})
	conv, msg, _ := s.Claim()
	s.SetThread(conv.ID, "native-thread")
	s.Close()
	s, e = OpenStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	got, e := s.Conversation(r.ConversationID)
	if e != nil || got.ThreadID != "native-thread" || got.Status != "failed" {
		t.Fatalf("lost restart state: %+v %v", got, e)
	}
	if _, _, e = s.Claim(); !errors.Is(e, ErrNotFound) {
		t.Fatalf("uncertain action replayed: %v", e)
	}
	messages, _ := s.Messages(conv.ID)
	if messages[0].ID != msg.ID || messages[0].Status != "failed" {
		t.Fatalf("missing input: %+v", messages)
	}
}
func TestDisabledAgentBlocksNewConversationButExistingSnapshotPersists(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	c := Caller{Source: "console", Admin: true}
	r, _ := s.Submit(c, Input{AgentID: a.ID, UserID: "operator", Message: "hello"})
	a.Name = "changed"
	a.Enabled = false
	s.SaveAgent(a)
	if _, e := s.Submit(c, Input{AgentID: a.ID, UserID: "operator", Message: "new"}); e == nil {
		t.Fatal("disabled Agent accepted new conversation")
	}
	conv, _ := s.Conversation(r.ConversationID)
	if conv.Snapshot.Name != "test" {
		t.Fatal("existing configuration changed")
	}
}
