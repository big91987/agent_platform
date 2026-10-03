package platform

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"
)

type Executor interface {
	Execute(context.Context, Conversation, Message, func([]byte) error) error
}
type Scheduler struct {
	store    *Store
	executor Executor
	limit    int
	mu       sync.Mutex
	active   map[string]context.CancelFunc
	steering map[string]chan steeringInput
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewScheduler(s *Store, x Executor, n int) *Scheduler {
	if n < 1 {
		n = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{store: s, executor: x, limit: n, active: map[string]context.CancelFunc{}, steering: map[string]chan steeringInput{}, ctx: ctx, cancel: cancel}
}
func (s *Scheduler) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.dispatch()
			}
		}
	}()
}
func (s *Scheduler) dispatch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return
	}
	for len(s.active) < s.limit {
		busy := []string{}
		for id := range s.active {
			active, err := s.store.Conversation(id)
			if err != nil {
				log.Printf("scheduler workspace: %v", err)
				return
			}
			if !active.ReadOnly {
				busy = append(busy, active.WorkspacePath)
			}
		}
		c, m, e := s.store.Claim(busy...)
		if errors.Is(e, ErrNotFound) {
			return
		}
		if e != nil {
			log.Printf("scheduler claim: %v", e)
			return
		}
		ctx, cancel := context.WithCancel(s.ctx)
		s.active[c.ID] = cancel
		if x, ok := s.executor.(interface{ SupportsSteering() bool }); ok && x.SupportsSteering() {
			ch := make(chan steeringInput, 16)
			s.steering[c.ID] = ch
			ctx = context.WithValue(ctx, steeringKey{}, ch)
		}
		s.wg.Add(1)
		go s.execute(ctx, c, m)
	}
}
func (s *Scheduler) execute(ctx context.Context, c Conversation, m Message) {
	defer s.wg.Done()
	started, _ := json.Marshal(map[string]any{"type": "platform.execution.started", "message_id": m.ID})
	s.store.RecordEvent(c.ID, m.ID, started)
	e := s.executor.Execute(ctx, c, m, func(raw []byte) error {
		var event struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
		}
		if e := json.Unmarshal(raw, &event); e != nil {
			return e
		}
		if event.Type == "thread.started" {
			if event.ThreadID == "" {
				return errors.New("native thread ID missing")
			}
			if c.ThreadID != "" && c.ThreadID != event.ThreadID {
				return errors.New("native thread identity changed")
			}
			if e := s.store.SetThread(c.ID, event.ThreadID); e != nil {
				return e
			}
		}
		return s.store.RecordEvent(c.ID, m.ID, raw)
	})
	status, reason := "completed", ""
	if e != nil {
		status = "failed"
		reason = e.Error()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch := s.steering[c.ID]; ch != nil {
		delete(s.steering, c.ID)
		for len(ch) > 0 {
			req := <-ch
			req.Finish(errors.New("当前轮已结束，消息仍在队列中"), true)
		}
	}
	if cancel, ok := s.active[c.ID]; ok {
		cancel()
		delete(s.active, c.ID)
	}
	if current, err := s.store.Conversation(c.ID); err == nil && current.Status == "stopping" {
		s.store.Halt(c.ID, "stopped", "")
		status = "stopped"
		reason = ""
	}
	if e := s.store.Complete(c.ID, m.ID, status, reason); e != nil {
		log.Printf("save execution result: %v", e)
	}
	ended, _ := json.Marshal(map[string]any{"type": "platform.execution." + status, "message_id": m.ID, "error": reason})
	s.store.RecordEvent(c.ID, m.ID, ended)
}
func (s *Scheduler) Stop(id string, close bool) error {
	return s.StopExpected(id, close, 0, false)
}
func (s *Scheduler) StopExpected(id string, close bool, expected int64, discard bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := "stopped"
	cancel, active := s.active[id]
	if active {
		state = "stopping"
	}
	if close {
		state = "closed"
	}
	if e := s.store.HaltExpected(id, state, "", expected, discard); e != nil {
		return e
	}
	if active {
		cancel()
	}
	return nil
}
func (s *Scheduler) Continue(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.active[id]; ok {
		return ErrConflict
	}
	return s.store.Continue(id)
}
func (s *Scheduler) Close() { s.cancel(); s.wg.Wait() }

// Access changes apply only between turns; a running process keeps its sandbox.
func (s *Scheduler) SetWorkspaceAccess(id string, readOnly bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, active := s.active[id]; active {
		return ErrConflict
	}
	changed, err := s.store.DB.Exec(`UPDATE conversations SET read_only=?,updated=? WHERE id=? AND status NOT IN ('running','stopping','closed')`, readOnly, now(), id)
	if err != nil {
		return err
	}
	n, err := changed.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
