package platform

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSteeringReservationOwnershipAndQueueRecovery(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	caller := Caller{Admin: true, Source: "console"}
	r, _ := s.Submit(caller, Input{AgentID: a.ID, UserID: "operator", Message: "first"})
	c, m, _ := s.Claim()
	second, _ := s.Submit(caller, Input{ConversationID: c.ID, UserID: "operator", Message: "second"})
	third, _ := s.Submit(caller, Input{ConversationID: c.ID, UserID: "operator", Message: "third"})
	if _, err := s.reserveSteering("other", second.MessageID); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	req, err := s.reserveSteering(c.ID, second.MessageID)
	if err != nil || req.ParentID != m.ID {
		t.Fatal(req, err)
	}
	if _, err = s.reserveSteering(c.ID, second.MessageID); !errors.Is(err, ErrConflict) {
		t.Fatal("duplicate steering", err)
	}
	if err := s.finishSteering(c.ID, second.MessageID, errors.New("native rejected"), true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.reserveSteering(c.ID, second.MessageID); err != nil {
		t.Fatal(err)
	}
	if err := s.finishSteering(c.ID, second.MessageID, nil, false); err != nil {
		t.Fatal(err)
	}
	s.Complete(c.ID, r.MessageID, "completed", "")
	_, next, err := s.Claim()
	if err != nil || next.ID != third.MessageID {
		t.Fatal("steered input replayed or queue reordered", next, err)
	}
	msgs, _ := s.Messages(c.ID)
	if msgs[1].Status != "steered" {
		t.Fatal(msgs)
	}
}

func TestNativeSteeringAcknowledgementRejectionAndUncertainDelivery(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	for _, mode := range []string{"accept", "reject", "disconnect"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			binary := filepath.Join(root, "codex")
			script := `#!` + python + `
import sys,json
for line in sys.stdin:
 p=json.loads(line); method=p.get('method'); ident=p.get('id')
 def send(v): print(json.dumps(v),flush=True)
 if method=='initialize': send({'id':ident,'result':{}})
 elif method=='thread/start':
  if '` + mode + `'=='accept':
   assert p['params']['sandbox']=='read-only'
   assert p['params']['approvalPolicy']['granular']['mcp_elicitations'] is True
   assert p['params']['approvalPolicy']['granular']['sandbox_approval'] is False
  else: assert p['params']['sandbox']=='workspace-write'
  send({'id':ident,'result':{'thread':{'id':'native-thread'}}})
 elif method=='turn/start':
  send({'id':ident,'result':{'turn':{'id':'native-turn'}}})
  send({'method':'turn/started','params':{'turn':{'id':'native-turn'}}})
 elif method=='turn/steer':
  assert p['params']['expectedTurnId']=='native-turn'
  assert p['params']['threadId']=='native-thread'
  assert p['params']['input'][0]['text']=='new direction'
  if '` + mode + `'=='disconnect': sys.exit(0)
  if '` + mode + `'=='reject': send({'id':ident,'error':{'message':'turn already ended'}})
  else: send({'id':ident,'result':{'turnId':'native-turn'}})
  send({'method':'turn/completed','params':{'turn':{'id':'native-turn','status':'completed'}}})
`
			os.WriteFile(binary, []byte(script), 0700)
			x := Codex{Root: root, Binary: binary}
			ch := make(chan steeringInput, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = context.WithValue(ctx, steeringKey{}, ch)
			var finished, rejected, failed bool
			var events []string
			err := x.executeAppServer(ctx, Conversation{ID: "test", ReadOnly: mode == "accept", Snapshot: Agent{Sandbox: "workspace-write", ResolvedTools: map[string]ResolvedToolServer{"test": {Approvals: map[string]string{"replace": "confirm"}}}}}, Message{Content: "original"}, root, root, os.Environ(), func(raw []byte) error {
				var e struct {
					Type string `json:"type"`
				}
				json.Unmarshal(raw, &e)
				events = append(events, e.Type)
				if e.Type == "turn.started" {
					ch <- steeringInput{Message: Message{Content: "new direction"}, Finish: func(e error, r bool) error { finished = true; failed = e != nil; rejected = r; return nil }}
				}
				return nil
			})
			if !finished || rejected != (mode == "reject") || failed != (mode != "accept") {
				t.Fatalf("mode=%s finished=%v rejected=%v failed=%v err=%v", mode, finished, rejected, failed, err)
			}
			if mode == "disconnect" && err == nil {
				t.Fatal("lost delivery hidden")
			}
			if mode != "disconnect" && err != nil {
				t.Fatal(err)
			}
			if strings.Count(strings.Join(events, ","), "turn.started") != 1 {
				t.Fatal("steer restarted turn", events)
			}
		})
	}
}

func TestSteeringAPIRequiresConversationOwnerAndQueuedInput(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	_, alice := testUser(t, s, &a, "alice")
	_, bob := testUser(t, s, &a, "bob")
	r, err := s.Submit(Caller{Source: "console", UserID: "alice", Agents: []string{a.ID}}, Input{AgentID: a.ID, UserID: "alice", Message: "first"})
	if err != nil {
		t.Fatal(err)
	}
	s.Claim()
	next, err := s.Submit(Caller{Source: "console", UserID: "alice", Agents: []string{a.ID}}, Input{ConversationID: r.ConversationID, UserID: "alice", Message: "guide"})
	if err != nil {
		t.Fatal(err)
	}
	sched := NewScheduler(s, nil, 1)
	sched.steering[r.ConversationID] = make(chan steeringInput, 1)
	server := NewServer(s, sched, nil, "password", "http://localhost")
	url := "/api/conversations/" + r.ConversationID + "/steer"
	for _, test := range []struct {
		token string
		mid   int64
		want  int
	}{{bob, next.MessageID, 403}, {alice, r.MessageID, 409}, {alice, next.MessageID, 202}, {alice, next.MessageID, 409}} {
		w := requestJSON(t, server, "POST", url, test.token, map[string]any{"message_id": test.mid})
		if w.Code != test.want {
			t.Fatalf("want %d got %d %s", test.want, w.Code, w.Body.String())
		}
	}
	req := <-sched.steering[r.ConversationID]
	req.Finish(nil, false)
}
