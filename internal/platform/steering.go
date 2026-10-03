package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type steeringKey struct{}
type steeringInput struct {
	Message Message
	Finish  func(error, bool) error // rejected=true means native input was definitely not accepted
}

func (x *Codex) SupportsSteering() bool { return true }

// Steer reserves an existing queued input. Uncertain delivery is never replayed.
func (s *Scheduler) Steer(id string, mid int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	input := s.steering[id]
	if input == nil {
		return ErrConflict
	}
	if len(input) == cap(input) {
		return ErrConflict
	}
	m, err := s.store.reserveSteering(id, mid)
	if err != nil {
		return err
	}
	input <- steeringInput{Message: m, Finish: func(err error, rejected bool) error {
		return s.store.finishSteering(id, mid, err, rejected)
	}}
	return nil
}

func (s *Store) reserveSteering(id string, mid int64) (Message, error) {
	var m Message
	tx, err := s.DB.Begin()
	if err != nil {
		return m, err
	}
	defer tx.Rollback()
	err = tx.QueryRow(`SELECT m.id,m.content,active.id FROM messages m
 JOIN conversations c ON c.id=m.conversation_id
 JOIN messages active ON active.conversation_id=c.id AND active.role='user' AND active.status='running'
 WHERE m.id=? AND m.conversation_id=? AND m.role='user' AND m.status='queued' AND c.status='running'`, mid, id).Scan(&m.ID, &m.Content, &m.ParentID)
	if errors.Is(err, sql.ErrNoRows) {
		return m, ErrConflict
	}
	if err != nil {
		return m, err
	}
	_, err = tx.Exec(`UPDATE messages SET status='steering',parent_id=? WHERE id=?`, m.ParentID, mid)
	if err != nil {
		return m, err
	}
	m.ConversationID = id
	return m, tx.Commit()
}

func (s *Store) finishSteering(id string, mid int64, nativeErr error, rejected bool) error {
	status, detail := "steered", ""
	if nativeErr != nil {
		status, detail = "failed", nativeErr.Error()
		if rejected {
			status = "queued"
		}
	}
	_, err := s.DB.Exec(`UPDATE messages SET status=?,parent_id=CASE WHEN ?='queued' THEN 0 ELSE parent_id END WHERE id=? AND conversation_id=? AND status='steering'`, status, status, mid, id)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(map[string]any{"type": "platform.input." + status, "message_id": mid, "error": detail})
	return s.RecordEvent(id, mid, raw)
}

func steeringChannel(ctx context.Context) chan steeringInput {
	c, _ := ctx.Value(steeringKey{}).(chan steeringInput)
	return c
}
