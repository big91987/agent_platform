package platform

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerTurnTimeoutStopsOnlyItsConversationAndPreservesQueuedInput(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	// Exercise the persisted public configuration, including its frozen snapshot.
	raw, _ := json.Marshal(a)
	var config map[string]any
	json.Unmarshal(raw, &config)
	config["turn_timeout_seconds"] = 1
	raw, _ = json.Marshal(config)
	json.Unmarshal(raw, &a)
	a, err := s.SaveAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	x := &controlledExecutor{started: make(chan string, 5), release: make(chan struct{}, 5)}
	sched := NewScheduler(s, x, 2)
	sched.Start()
	defer sched.Close()
	caller := Caller{Source: "console", Admin: true}
	r, err := s.Submit(caller, Input{AgentID: a.ID, UserID: "operator", Message: "blocked turn"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-x.started:
	case <-time.After(2 * time.Second):
		t.Fatal("executor not started")
	}
	_, err = s.Submit(caller, Input{ConversationID: r.ConversationID, UserID: "operator", Message: "keep queued"})
	if err != nil {
		t.Fatal(err)
	}
	// A new saved configuration must not extend an already frozen conversation.
	config["turn_timeout_seconds"] = 0
	raw, _ = json.Marshal(config)
	json.Unmarshal(raw, &a)
	if _, err = s.SaveAgent(a); err != nil {
		t.Fatal(err)
	}
	other, err := s.Submit(caller, Input{AgentID: a.ID, UserID: "operator", Message: "independent turn"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-x.started:
	case <-time.After(2 * time.Second):
		t.Fatal("independent executor not started")
	}
	waitStatus(t, s, r.ConversationID, "failed")
	c, _ := s.Conversation(r.ConversationID)
	if !strings.Contains(c.Error, "timeout") {
		t.Fatalf("timeout cause missing: %s", c.Error)
	}
	messages, _ := s.Messages(r.ConversationID)
	if len(messages) != 2 || messages[0].Status != "failed" || messages[1].Status != "queued" {
		t.Fatalf("lost queued input: %+v", messages)
	}
	c, _ = s.Conversation(other.ConversationID)
	if c.Status != "running" {
		t.Fatalf("unrelated execution interrupted: %s", c.Status)
	}
	x.release <- struct{}{}
	waitStatus(t, s, other.ConversationID, "idle")
}

func TestAgentTurnTimeoutRejectsInvalidLimitsInStandaloneAndWorkflow(t *testing.T) {
	s := testStore(t)
	base := testAgent(t, s)
	for _, limit := range []int{-1, 86401} {
		a := base
		raw, _ := json.Marshal(a)
		var config map[string]any
		json.Unmarshal(raw, &config)
		config["turn_timeout_seconds"] = limit
		raw, _ = json.Marshal(config)
		json.Unmarshal(raw, &a)
		if _, err := s.SaveAgent(a); err == nil {
			t.Fatalf("standalone accepted timeout %d", limit)
		}
		a.ID = ""
		a.Name = ""
		a.Enabled = false
		a.AuthorizedUsers = nil
		if err := validateWorkflowAgent(s.DB, a); err == nil {
			t.Fatalf("workflow accepted timeout %d", limit)
		}
	}
}

type controlledExecutor struct {
	started chan string
	release chan struct{}
	active  atomic.Int32
	peak    atomic.Int32
}

func (x *controlledExecutor) Execute(ctx context.Context, c Conversation, m Message, event func([]byte) error) error {
	n := x.active.Add(1)
	defer x.active.Add(-1)
	for {
		p := x.peak.Load()
		if n <= p || x.peak.CompareAndSwap(p, n) {
			break
		}
	}
	x.started <- c.ID
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-x.release:
	}
	raw, _ := json.Marshal(map[string]any{"type": "item.completed", "item": map[string]string{"id": "reply", "type": "agent_message", "text": "actual reply"}})
	return event(raw)
}
func waitStatus(t *testing.T, s *Store, id, status string) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		c, e := s.Conversation(id)
		if e == nil && c.Status == status {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	c, _ := s.Conversation(id)
	t.Fatalf("wanted %s, got %s", status, c.Status)
}
func TestSchedulerStopPreservesQueuedInputUntilExplicitContinue(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	x := &controlledExecutor{started: make(chan string, 5), release: make(chan struct{}, 5)}
	sched := NewScheduler(s, x, 1)
	sched.Start()
	defer sched.Close()
	c := Caller{Source: "console", Admin: true}
	r, _ := s.Submit(c, Input{AgentID: a.ID, UserID: "operator", Message: "first"})
	select {
	case <-x.started:
	case <-time.After(2 * time.Second):
		t.Fatal("executor not started")
	}
	second, _ := s.Submit(c, Input{ConversationID: r.ConversationID, UserID: "operator", Message: "second"})
	if e := sched.Stop(r.ConversationID, false); e != nil {
		t.Fatal(e)
	}
	waitStatus(t, s, r.ConversationID, "stopped")
	select {
	case <-x.started:
		t.Fatal("queued input started without consent")
	case <-time.After(300 * time.Millisecond):
	}
	if e := sched.Continue(r.ConversationID); e != nil {
		t.Fatal(e)
	}
	select {
	case <-x.started:
	case <-time.After(3 * time.Second):
		t.Fatal("queue did not resume")
	}
	x.release <- struct{}{}
	waitStatus(t, s, r.ConversationID, "idle")
	msgs, _ := s.Messages(r.ConversationID)
	if len(msgs) != 3 || msgs[2].Content != "actual reply" || msgs[2].Kind != "reply" {
		t.Fatalf("native reply missing: %+v", msgs)
	}
	var reply map[string]any
	encoded, _ := json.Marshal(msgs[2])
	json.Unmarshal(encoded, &reply)
	if reply["parent_id"] != float64(second.MessageID) {
		t.Fatalf("reply cannot be associated with its submitted input: %s", encoded)
	}
}
func TestSchedulerLimitsConcurrencyAndRunsIndependentConversations(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	x := &controlledExecutor{started: make(chan string, 5), release: make(chan struct{}, 5)}
	sched := NewScheduler(s, x, 1)
	sched.Start()
	defer sched.Close()
	c := Caller{Source: "console", Admin: true}
	r1, _ := s.Submit(c, Input{AgentID: a.ID, UserID: "operator", Message: "one"})
	r2, _ := s.Submit(c, Input{AgentID: a.ID, UserID: "operator", Message: "two"})
	select {
	case <-x.started:
	case <-time.After(2 * time.Second):
		t.Fatal("executor not started")
	}
	select {
	case <-x.started:
		t.Fatal("capacity exceeded")
	case <-time.After(300 * time.Millisecond):
	}
	x.release <- struct{}{}
	select {
	case <-x.started:
	case <-time.After(3 * time.Second):
		t.Fatal("second conversation starved")
	}
	x.release <- struct{}{}
	waitStatus(t, s, r1.ConversationID, "idle")
	waitStatus(t, s, r2.ConversationID, "idle")
	if x.peak.Load() != 1 {
		t.Fatal("capacity not enforced")
	}
}

type uncertainNativeExecutor struct{ started chan struct{} }

func (x uncertainNativeExecutor) Execute(ctx context.Context, _ Conversation, _ Message, _ func([]byte) error) error {
	close(x.started)
	<-ctx.Done()
	return &nativeExecutionUnconfirmed{"Native stop could not be confirmed"}
}
func TestSchedulerStopRetainsUnconfirmedNativeOutcome(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	started := make(chan struct{})
	scheduler := NewScheduler(s, uncertainNativeExecutor{started}, 1)
	scheduler.Start()
	defer scheduler.Close()
	receipt, err := s.Submit(Caller{Source: "console", Admin: true}, Input{AgentID: a.ID, UserID: "operator", Message: "stop evidence"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("executor did not start")
	}
	if err = scheduler.Stop(receipt.ConversationID, false); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, s, receipt.ConversationID, "failed")
	conversation, _ := s.Conversation(receipt.ConversationID)
	if conversation.Error != "Native stop could not be confirmed" {
		t.Fatal("stop uncertainty erased", conversation)
	}
	messages, _ := s.Messages(receipt.ConversationID)
	if messages[0].Status != "failed" {
		t.Fatal("uncertain stop presented as confirmed", messages)
	}
}

func TestSchedulerTimeoutRetainsUnconfirmedNativeOutcome(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	a.TurnTimeoutSeconds = 1
	if _, err := s.SaveAgent(a); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	scheduler := NewScheduler(s, uncertainNativeExecutor{started}, 1)
	scheduler.Start()
	defer scheduler.Close()
	receipt, err := s.Submit(Caller{Source: "console", Admin: true}, Input{AgentID: a.ID, UserID: "operator", Message: "timeout evidence"})
	if err != nil {
		t.Fatal(err)
	}
	waitStatus(t, s, receipt.ConversationID, "failed")
	conversation, _ := s.Conversation(receipt.ConversationID)
	if !strings.Contains(conversation.Error, "timeout") || !strings.Contains(conversation.Error, "stop could not be confirmed") {
		t.Fatalf("timeout or native uncertainty erased: %s", conversation.Error)
	}
}
