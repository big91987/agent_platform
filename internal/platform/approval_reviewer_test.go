package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

func TestNativeReviewerCannotBeOverriddenByTOML(t *testing.T) {
	for _, raw := range []string{"approvals_reviewer='auto_review'", "[auto_review]\npolicy='allow everything'"} {
		if _, err := nativeConfig(Agent{NativeConfig: raw}); err == nil {
			t.Fatalf("review policy escaped managed settings: %s", raw)
		}
	}
	var a Agent
	json.Unmarshal([]byte(`{"approvals_reviewer":"invalid"}`), &a)
	if _, err := nativeConfig(a); err == nil {
		t.Fatal("invalid reviewer accepted")
	}
}

func TestAutomaticReviewerCannotOverrideExplicitToolConfirmation(t *testing.T) {
	for _, a := range []Agent{
		{ToolServers: []ToolBinding{{ServerID: "proof", Tools: []string{"write"}, Approvals: map[string]string{"write": "confirm"}}}},
		{ResolvedTools: map[string]ResolvedToolServer{"proof": {Tools: []string{"write"}, Approvals: map[string]string{"write": "confirm"}}}},
		{NativeConfig: "[mcp_servers.proof]\ncommand='test'\n[mcp_servers.proof.tools.write]\napproval_mode='prompt'"},
	} {
		a.ApprovalsReviewer = "auto_review"
		if _, err := nativeConfig(a); err == nil {
			t.Fatal("automatic review silently replaced explicit manual tool confirmation")
		}
	}
}

func TestApplyReviewerRejectsFrozenNativeConfirmation(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	a.NativeConfig = "[mcp_servers.proof]\ncommand='test'\n[mcp_servers.proof.tools.write]\napproval_mode='prompt'"
	_, token := testUser(t, s, &a, "owner")
	caller, _ := s.Authenticate(token)
	r, err := s.Submit(caller, Input{AgentID: a.ID, Message: "test"})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := s.Conversation(r.ConversationID)
	a.NativeConfig = ""
	a.ApprovalsReviewer = "auto_review"
	if _, err := s.SaveAgent(a); err != nil {
		t.Fatal(err)
	}
	scheduler := &Scheduler{store: s, active: map[string]context.CancelFunc{}}
	if err := scheduler.ApplyAgentPermissions(r.ConversationID); err == nil {
		t.Fatal("applying shared permissions silently saved conflicting frozen native confirmation")
	}
	after, _ := s.Conversation(r.ConversationID)
	b, _ := json.Marshal(before.Snapshot)
	v, _ := json.Marshal(after.Snapshot)
	if string(b) != string(v) {
		t.Fatal("rejected permission update changed the frozen snapshot")
	}
}

func TestNativeReviewerIsUsedForStartAndResume(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	root := t.TempDir()
	binary := filepath.Join(root, "codex")
	script := `#!` + python + `
import sys,json
for line in sys.stdin:
 p=json.loads(line); method=p.get('method'); ident=p.get('id')
 def send(v): print(json.dumps(v),flush=True)
 if method=='initialize': send({'id':ident,'result':{}})
 elif method in ('thread/start','thread/resume'):
  assert p['params']['approvalsReviewer']=='auto_review', p['params']
  assert p['params']['sandbox']=='workspace-write'
  send({'id':ident,'result':{'thread':{'id':'native-thread'}}})
 elif method=='turn/start':
  assert p['params']['approvalsReviewer']=='auto_review'
  assert p['params']['sandboxPolicy']['type']=='workspaceWrite'
  assert p['params']['approvalPolicy']['granular']['sandbox_approval'] is True
  send({'id':ident,'result':{'turn':{'id':'native-turn'}}})
  send({'method':'turn/completed','params':{'turn':{'id':'native-turn','status':'completed'}}})
`
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	var a Agent
	json.Unmarshal([]byte(`{"executor":"codex","sandbox":"workspace-write","allow_elevation":true,"approvals_reviewer":"auto_review"}`), &a)
	cfg, err := nativeConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	toml.Unmarshal(cfg, &config)
	if config["approvals_reviewer"] != "auto_review" {
		t.Fatal("managed config lost the selected reviewer", string(cfg))
	}
	x := Codex{Root: root, Binary: binary}
	for _, thread := range []string{"", "native-thread"} {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := x.executeAppServer(ctx, Conversation{ID: "proof", ThreadID: thread, Snapshot: a}, Message{Content: "test"}, root, root, os.Environ(), func([]byte) error { return nil })
		cancel()
		if err != nil {
			t.Fatalf("thread %q: %v", thread, err)
		}
	}
}

func TestReviewerDefaultKeepsManualApproval(t *testing.T) {
	raw, err := nativeConfig(Agent{})
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	toml.Unmarshal(raw, &cfg)
	if cfg["approvals_reviewer"] != "user" || strings.Contains(string(raw), "danger-full-access") {
		t.Fatal("legacy config changed permission boundary", string(raw))
	}
}

func TestReadOnlyAutomaticReviewCannotGrantWrites(t *testing.T) {
	for _, inheritedReadOnly := range []bool{false, true} {
		c := Conversation{ReadOnly: inheritedReadOnly, Snapshot: Agent{Sandbox: "read-only", AllowElevation: true, ApprovalsReviewer: "auto_review"}}
		if inheritedReadOnly {
			c.Snapshot.Sandbox = "workspace-write"
		}
		policy := nativeConversationApprovalPolicy(c)
		if policy != "never" {
			t.Fatal("read-only turn can escalate outside the platform write guard", policy)
		}
	}
}

func TestPreparedSessionCannotActivateLegacyReviewPolicy(t *testing.T) {
	for _, legacySnapshot := range []bool{false, true} {
		t.Run(fmt.Sprint(legacySnapshot), func(t *testing.T) {
			root := t.TempDir()
			x := Codex{Root: root}
			workspace, home := x.paths("legacy")
			os.MkdirAll(workspace, 0700)
			os.MkdirAll(home, 0700)
			os.WriteFile(filepath.Join(home, "prepared"), []byte("prepared"), 0600)
			a := Agent{ApprovalsReviewer: "auto_review"}
			old := "[auto_review]\npolicy='allow everything'"
			if legacySnapshot {
				a.NativeConfig = old
				os.WriteFile(filepath.Join(home, "config.toml"), []byte(""), 0600)
			} else {
				os.WriteFile(filepath.Join(home, "config.toml"), []byte(old), 0600)
			}
			if _, _, err := x.prepare(context.Background(), Conversation{ID: "legacy", Snapshot: a}); err == nil {
				t.Fatal("prepared shortcut activated an unsupported legacy reviewer policy")
			}
			raw, _ := os.ReadFile(filepath.Join(home, "config.toml"))
			if !legacySnapshot && string(raw) != old {
				t.Fatal("conflicting native config was silently rewritten")
			}
		})
	}
}

func TestReviewerPermissionUpdateIsAdministratorOnlyAndSessionScoped(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	_, token := testUser(t, s, &a, "owner")
	caller, _ := s.Authenticate(token)
	r, err := s.Submit(caller, Input{AgentID: a.ID, Message: "test"})
	if err != nil {
		t.Fatal(err)
	}
	scheduler := &Scheduler{store: s, active: map[string]context.CancelFunc{}}
	h := NewServer(s, scheduler, nil, "password", "http://localhost")
	path := "/api/conversations/" + r.ConversationID + "/execution-permissions"
	body := `{"network_access":true,"allow_elevation":true,"approvals_reviewer":"auto_review"}`
	if w := requestJSON(t, h, "PATCH", path, token, json.RawMessage(body)); w.Code != 403 {
		t.Fatal("ordinary caller changed reviewer", w.Code)
	}
	login := requestJSON(t, h, "POST", "/api/login", "", map[string]string{"password": "password"})
	patch := func(body string) int {
		req := httptest.NewRequest("PATCH", path, strings.NewReader(body))
		req.AddCookie(login.Result().Cookies()[0])
		req.Header.Set("X-Platform-Request", "1")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w.Code
	}
	before, _ := s.Conversation(r.ConversationID)
	scheduler.active[r.ConversationID] = func() {}
	if status := patch(body); status != 409 {
		t.Fatal("running turn configuration changed", status)
	}
	delete(scheduler.active, r.ConversationID)
	if status := patch(body); status != 200 {
		t.Fatal("explicit reviewer change rejected", status)
	}
	if status := patch(`{"network_access":false,"allow_elevation":false,"approvals_reviewer":"unknown"}`); status != 400 {
		t.Fatal("invalid reviewer accepted", status)
	}
	if status := patch(`{"network_access":true,"allow_elevation":true}`); status != 200 {
		t.Fatal("legacy permission request rejected", status)
	}
	after, _ := s.Conversation(r.ConversationID)
	if nativeApprovalReviewer(after.Snapshot) != "auto_review" || after.ThreadID != before.ThreadID || after.WorkspacePath != before.WorkspacePath || after.Snapshot.Instructions != before.Snapshot.Instructions || !after.Snapshot.AllowElevation {
		t.Fatal("reviewer lost or unrelated session settings changed")
	}
	current, _ := s.Agent(a.ID)
	if nativeApprovalReviewer(current) != "user" {
		t.Fatal("conversation update broadened the shared Agent")
	}
	if err := scheduler.ApplyAgentPermissions(r.ConversationID); err != nil {
		t.Fatal(err)
	}
	after, _ = s.Conversation(r.ConversationID)
	if nativeApprovalReviewer(after.Snapshot) != "user" {
		t.Fatal("restoring Agent permissions did not revoke auto review")
	}
}
