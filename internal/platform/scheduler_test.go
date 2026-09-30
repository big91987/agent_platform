package platform

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"
)

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
	s.Submit(c, Input{ConversationID: r.ConversationID, UserID: "operator", Message: "second"})
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
