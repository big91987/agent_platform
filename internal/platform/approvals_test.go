package platform

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func waitApproval(t *testing.T, s *Store, id string) ToolApproval {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		list, e := s.Approvals(id)
		if e != nil {
			t.Fatal(e)
		}
		for _, a := range list {
			if a.Decision == "" {
				return a
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("approval did not appear")
	return ToolApproval{}
}
func TestToolApprovalOwnershipDecisionAndCancellation(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	_, owner := testUser(t, s, &a, "owner")
	_, other := testUser(t, s, &a, "other")
	caller, _ := s.Authenticate(owner)
	_, err := s.Submit(caller, Input{AgentID: a.ID, Message: "call tool"})
	if err != nil {
		t.Fatal(err)
	}
	c, m, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	h := NewServer(s, nil, nil, "password", "http://localhost")
	for _, decision := range []string{"accept", "decline", "cancel"} {
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan string, 1)
		go func() {
			value, e := s.AwaitToolApproval(ctx, c, m, json.RawMessage(`{"serverName":"external","message":"Write test record?"}`))
			if e != nil {
				value = "error"
			}
			result <- value
		}()
		pending := waitApproval(t, s, c.ID)
		path := "/api/conversations/" + c.ID + "/approvals/" + pending.ID
		if w := requestJSON(t, h, "POST", path, other, map[string]string{"decision": "accept"}); w.Code != 403 {
			cancel()
			t.Fatalf("cross-user approval: %d", w.Code)
		}
		snapshot := requestJSON(t, h, "GET", "/api/conversations/"+c.ID, owner, nil)
		var data struct {
			Approvals []ToolApproval `json:"approvals"`
		}
		json.Unmarshal(snapshot.Body.Bytes(), &data)
		if len(data.Approvals) == 0 {
			cancel()
			t.Fatal("pending confirmation absent after refresh")
		}
		if decision == "cancel" {
			cancel()
		} else {
			if w := requestJSON(t, h, "POST", path, owner, map[string]string{"decision": decision}); w.Code != 200 {
				cancel()
				t.Fatal(w.Body.String())
			}
			if w := requestJSON(t, h, "POST", path, owner, map[string]string{"decision": decision}); w.Code != 409 {
				cancel()
				t.Fatalf("replayed decision: %d", w.Code)
			}
		}
		select {
		case got := <-result:
			if decision != "cancel" && got != decision {
				t.Fatalf("wrong decision %s", got)
			}
		case <-time.After(2 * time.Second):
			cancel()
			t.Fatal("native waiter not released")
		}
		cancel()
		if err = s.DecideToolApproval(c.ID, pending.ID, "accept"); !errors.Is(err, ErrConflict) {
			t.Fatalf("late approval accepted: %v", err)
		}
	}
	// A platform restart invalidates stale pending requests instead of executing them.
	_, err = s.DB.Exec(`INSERT INTO tool_approvals VALUES('orphan',?,?,?,'',?)`, c.ID, m.ID, `{}`, now())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.initApprovals(); err != nil {
		t.Fatal(err)
	}
	list, _ := s.Approvals(c.ID)
	for _, a := range list {
		if a.ID == "orphan" && a.Decision != "cancel" {
			t.Fatal("orphan approval survived startup")
		}
	}
}
