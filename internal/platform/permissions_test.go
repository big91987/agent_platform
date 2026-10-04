package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNativePermissionDecisions(t *testing.T) {
	for _, method := range []string{"item/commandExecution/requestApproval", "item/fileChange/requestApproval", "item/permissions/requestApproval"} {
		for _, decision := range []string{"accept", "decline"} {
			calls := 0
			x := &Codex{Approve: func(_ context.Context, _ Conversation, _ Message, raw json.RawMessage) (string, error) {
				calls++
				var p map[string]any
				json.Unmarshal(raw, &p)
				if p["platform_permission_request"] != true {
					t.Fatal("not marked as privileged")
				}
				return decision, nil
			}}
			raw := json.RawMessage(`{"threadId":"thread","turnId":"turn","permissions":{"network":{"enabled":true}},"reason":"Read public source"}`)
			c := Conversation{Snapshot: Agent{AllowElevation: true}}
			result, err := x.nativePermissionApproval(context.Background(), c, Message{}, method, raw, "thread", "turn", func(v string) string { return v })
			if err != nil || calls != 1 {
				t.Fatalf("%s %s: %v calls=%d", method, decision, err, calls)
			}
			if method == "item/permissions/requestApproval" {
				if result["scope"] != "turn" {
					t.Fatal(result)
				}
				p := result["permissions"].(map[string]any)
				if (len(p) > 0) != (decision == "accept") {
					t.Fatal(result)
				}
			} else if result["decision"] != decision {
				t.Fatal(result)
			}
			c.Snapshot.AllowElevation = false
			calls = 0
			if _, err = x.nativePermissionApproval(context.Background(), c, Message{}, method, raw, "thread", "turn", func(v string) string { return v }); err != nil || calls != 0 {
				t.Fatal("disabled policy prompted", err)
			}
			c.Snapshot.AllowElevation = true
			if _, err = x.nativePermissionApproval(context.Background(), c, Message{}, method, raw, "different", "turn", func(v string) string { return v }); err == nil {
				t.Fatal("cross-thread request accepted")
			}
		}
	}
}

func TestNetworkPolicyKeepsWorkspaceSandbox(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		c := Conversation{Snapshot: Agent{NetworkAccess: enabled, AllowElevation: true}}
		p := turnSandbox(c)
		if p["type"] != "workspaceWrite" || p["networkAccess"] != enabled {
			t.Fatal(p)
		}
		c.ReadOnly = true
		if p = turnSandbox(c); p["type"] != "readOnly" || p["networkAccess"] != enabled {
			t.Fatal(p)
		}
	}
}

func TestApplyPermissionsPreservesSessionAndOtherSettings(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	_, token := testUser(t, s, &a, "owner")
	caller, _ := s.Authenticate(token)
	receipt, err := s.Submit(caller, Input{AgentID: a.ID, Message: "test"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.Exec(`UPDATE conversations SET status='idle',thread_id='native-session' WHERE id=?`, receipt.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := s.Conversation(receipt.ConversationID)
	a.NetworkAccess = true
	a.AllowElevation = true
	a.Instructions = "must not replace frozen instructions"
	if _, err = s.SaveAgent(a); err != nil {
		t.Fatal(err)
	}
	scheduler := &Scheduler{store: s, active: map[string]context.CancelFunc{}}
	if err = scheduler.ApplyAgentPermissions(before.ID); err != nil {
		t.Fatal(err)
	}
	after, _ := s.Conversation(before.ID)
	if after.ThreadID != before.ThreadID || after.Snapshot.Instructions != before.Snapshot.Instructions || !after.Snapshot.NetworkAccess || !after.Snapshot.AllowElevation {
		t.Fatal("session replaced or permissions missing")
	}
	scheduler.active[before.ID] = func() {}
	if err = scheduler.ApplyAgentPermissions(before.ID); err != ErrConflict {
		t.Fatal("running policy changed", err)
	}
}

func TestElevationApprovalRequiresAdministrator(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	_, token := testUser(t, s, &a, "owner")
	caller, _ := s.Authenticate(token)
	s.Submit(caller, Input{AgentID: a.ID, Message: "test"})
	c, m, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	scheduler := &Scheduler{store: s, active: map[string]context.CancelFunc{}}
	h := NewServer(s, scheduler, nil, "password", "http://localhost")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan string, 1)
	go func() {
		d, _ := s.AwaitToolApproval(ctx, c, m, json.RawMessage(`{"platform_permission_request":true,"permissions":{"network":{"enabled":true}}}`))
		done <- d
	}()
	pending := waitApproval(t, s, c.ID)
	url := "/api/conversations/" + c.ID + "/approvals/" + pending.ID
	if w := requestJSON(t, h, "POST", url, token, map[string]string{"decision": "accept"}); w.Code != 403 {
		t.Fatalf("owner granted elevation: %d", w.Code)
	}
	if w := requestJSON(t, h, "POST", "/api/conversations/"+c.ID+"/apply-agent-permissions", token, map[string]any{}); w.Code != 403 {
		t.Fatal("owner changed policy", w.Code)
	}
	login := requestJSON(t, h, "POST", "/api/login", "", map[string]string{"password": "password"})
	req := httptest.NewRequest("POST", url, strings.NewReader(`{"decision":"accept"}`))
	req.AddCookie(login.Result().Cookies()[0])
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Platform-Request", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	select {
	case d := <-done:
		if d != "accept" {
			t.Fatal(d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("approval did not resume")
	}
}

func TestLiveNetworkPermissionsAndResume(t *testing.T) {
	if os.Getenv("AGENT_PLATFORM_LIVE") != "1" {
		t.Skip("set AGENT_PLATFORM_LIVE=1")
	}
	root, err := os.MkdirTemp("", "platform-network-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence", root)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); fmt.Fprint(w, "NATIVE_NETWORK_OK") }))
	defer srv.Close()
	c := Conversation{ID: "network", Snapshot: Agent{Executor: "codex", Sandbox: "workspace-write", InheritEnv: true, NetworkAccess: true}}
	x := &Codex{Root: root}
	thread := ""
	var events []json.RawMessage
	callback := func(raw []byte) error {
		events = append(events, append(json.RawMessage(nil), raw...))
		var e struct {
			Type   string `json:"type"`
			Thread string `json:"thread_id"`
		}
		json.Unmarshal(raw, &e)
		if e.Type == "thread.started" {
			if thread != "" && thread != e.Thread {
				return errors.New("session replaced")
			}
			thread = e.Thread
		}
		return nil
	}
	defer func() {
		b, _ := json.MarshalIndent(events, "", "  ")
		os.WriteFile(filepath.Join(root, "events.json"), b, 0600)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	command := "curl --noproxy '*' --max-time 10 -fsS " + srv.URL
	if err = x.Execute(ctx, c, Message{Content: "Run exactly this shell command once and report its output: " + command + " . Do not use web tools, skills or change files."}, callback); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatal("preauthorized network unavailable", hits.Load())
	}
	t.Log("preauthorized network succeeded")
	c.ThreadID = thread
	c.Snapshot.NetworkAccess = false
	c.Snapshot.AllowElevation = true
	calls := 0
	decision := "accept"
	x.Approve = func(_ context.Context, _ Conversation, _ Message, raw json.RawMessage) (string, error) {
		calls++
		events = append(events, raw)
		t.Log("native approval received", decision)
		return decision, nil
	}
	prompt := "Run exactly this shell command once with sandbox_permissions=require_escalated and a justification requesting permission to contact the local test server: " + command + " . This is an explicit approval-path test: do not attempt it without escalation. If declined, stop immediately, do not retry or use other tools."
	if err = x.Execute(ctx, c, Message{Content: prompt}, callback); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || hits.Load() != 2 {
		t.Fatalf("approve requests=%d hits=%d", calls, hits.Load())
	}
	t.Log("approved command resumed same native session")
	decision = "decline"
	if err = x.Execute(ctx, c, Message{Content: prompt}, callback); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || hits.Load() != 2 {
		t.Fatalf("deny requests=%d hits=%d", calls, hits.Load())
	}
	t.Log("declined command never reached server")
}

func TestReadOnlyConversationCannotElevateWrites(t *testing.T) {
	x := &Codex{Approve: func(context.Context, Conversation, Message, json.RawMessage) (string, error) {
		t.Fatal("read-only write permission prompted")
		return "accept", nil
	}}
	c := Conversation{ReadOnly: true, Snapshot: Agent{AllowElevation: true}}
	for _, method := range []string{"item/commandExecution/requestApproval", "item/fileChange/requestApproval", "item/permissions/requestApproval"} {
		r, e := x.nativePermissionApproval(context.Background(), c, Message{}, method, json.RawMessage(`{"threadId":"t","turnId":"u","permissions":{"fileSystem":{"write":["/outside"]}}}`), "t", "u", func(v string) string { return v })
		if e != nil {
			t.Fatal(e)
		}
		if method == "item/permissions/requestApproval" {
			if len(r["permissions"].(map[string]any)) != 0 {
				t.Fatal(r)
			}
		} else if r["decision"] != "decline" {
			t.Fatal(r)
		}
	}
}

func TestConversationPermissionChangeDoesNotBroadenAgentDefaults(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	_, token := testUser(t, s, &a, "owner")
	caller, _ := s.Authenticate(token)
	r, err := s.Submit(caller, Input{AgentID: a.ID, Message: "test"})
	if err != nil {
		t.Fatal(err)
	}
	h := NewServer(s, &Scheduler{store: s, active: map[string]context.CancelFunc{}}, nil, "password", "http://localhost")
	path := "/api/conversations/" + r.ConversationID + "/execution-permissions"
	if w := requestJSON(t, h, "PATCH", path, token, map[string]bool{"network_access": true, "allow_elevation": true}); w.Code != 403 {
		t.Fatal(w.Code)
	}
	login := requestJSON(t, h, "POST", "/api/login", "", map[string]string{"password": "password"})
	req := httptest.NewRequest("PATCH", path, strings.NewReader(`{"network_access":true,"allow_elevation":true}`))
	req.AddCookie(login.Result().Cookies()[0])
	req.Header.Set("X-Platform-Request", "1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	c, _ := s.Conversation(r.ConversationID)
	current, _ := s.Agent(a.ID)
	if !c.Snapshot.NetworkAccess || !c.Snapshot.AllowElevation || current.NetworkAccess || current.AllowElevation {
		t.Fatal("permission scope leaked")
	}
	r2, err := s.Submit(caller, Input{AgentID: a.ID, Message: "unrelated"})
	if err != nil {
		t.Fatal(err)
	}
	other, _ := s.Conversation(r2.ConversationID)
	if other.Snapshot.NetworkAccess || other.Snapshot.AllowElevation {
		t.Fatal("unrelated new session inherited permission")
	}
}
