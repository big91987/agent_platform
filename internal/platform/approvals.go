package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type ToolApproval struct {
	ID        string          `json:"id"`
	MessageID int64           `json:"message_id"`
	Request   json.RawMessage `json:"request"`
	Decision  string          `json:"decision"`
	Created   string          `json:"created_at"`
}

func (s *Store) initApprovals() error {
	_, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS tool_approvals(id TEXT PRIMARY KEY,conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,request TEXT NOT NULL,decision TEXT NOT NULL,created TEXT NOT NULL); UPDATE tool_approvals SET decision='cancel' WHERE decision='';`)
	return err
}
func (s *Store) Approvals(id string) ([]ToolApproval, error) {
	rows, err := s.DB.Query(`SELECT id,message_id,request,decision,created FROM tool_approvals WHERE conversation_id=? ORDER BY created,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ToolApproval{}
	for rows.Next() {
		var a ToolApproval
		var raw string
		if err = rows.Scan(&a.ID, &a.MessageID, &raw, &a.Decision, &a.Created); err != nil {
			return nil, err
		}
		a.Request = json.RawMessage(raw)
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) AwaitToolApproval(ctx context.Context, c Conversation, m Message, raw json.RawMessage) (string, error) {
	id := newID()
	_, err := s.DB.Exec(`INSERT INTO tool_approvals VALUES(?,?,?,?,'',?)`, id, c.ID, m.ID, string(raw), now())
	if err != nil {
		return "", err
	}
	defer s.DB.Exec(`UPDATE tool_approvals SET decision='cancel' WHERE id=? AND decision=''`, id)
	event, _ := json.Marshal(map[string]any{"type": "platform.approval.requested", "approval_id": id})
	if err = s.RecordEvent(c.ID, m.ID, event); err != nil {
		return "", err
	}
	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-tick.C:
			var decision string
			if err = s.DB.QueryRow(`SELECT decision FROM tool_approvals WHERE id=?`, id).Scan(&decision); err != nil {
				return "", err
			}
			if decision != "" {
				return decision, nil
			}
		}
	}
}
func (s *Store) DecideToolApproval(conversation, id, decision string) error {
	if decision != "accept" && decision != "decline" {
		return errors.New("decision must be accept or decline")
	}
	result, err := s.DB.Exec(`UPDATE tool_approvals SET decision=? WHERE id=? AND conversation_id=? AND decision='' AND EXISTS(SELECT 1 FROM conversations c JOIN messages m ON m.conversation_id=c.id WHERE c.id=tool_approvals.conversation_id AND c.status='running' AND m.id=tool_approvals.message_id AND m.status='running')`, decision, id, conversation)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return ErrConflict
	}
	return nil
}
func (h *Server) approveTool(w http.ResponseWriter, r *http.Request, c Caller) {
	conv, ok := h.authorize(w, r, c)
	if !ok {
		return
	}
	var input struct {
		Decision string `json:"decision"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	if err := h.store.DecideToolApproval(conv.ID, r.PathValue("approval"), input.Decision); err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
