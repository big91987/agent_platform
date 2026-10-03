package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func requestJSON(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestAPIRequiresUserTokenAndDoesNotLeakAnotherUsersConversation(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	_, t1 := testUser(t, s, &a, "alice")
	_, t2 := testUser(t, s, &a, "bob")
	server := NewServer(s, nil, nil, "password", "http://localhost")
	workspace := t.TempDir()
	os.WriteFile(filepath.Join(workspace, "handoff.md"), []byte("previous stage output"), 0600)
	input := Input{AgentID: a.ID, UserID: "alice", Message: "private", RequestID: "one", WorkspacePath: workspace}
	if w := requestJSON(t, server, "POST", "/api/invoke", "", input); w.Code != 401 {
		t.Fatalf("unauthorized status %d", w.Code)
	}
	w := requestJSON(t, server, "POST", "/api/invoke", t1, input)
	if w.Code != 202 {
		t.Fatalf("invoke: %d %s", w.Code, w.Body.String())
	}
	var r Receipt
	json.Unmarshal(w.Body.Bytes(), &r)
	if r.ConversationURL != "http://localhost/conversations/"+r.ConversationID {
		t.Fatal("missing conversation link")
	}
	w = requestJSON(t, server, "GET", "/api/conversations/"+r.ConversationID+"?user_id=alice", t2, nil)
	if w.Code != 403 {
		t.Fatalf("cross-source read: %d %s", w.Code, w.Body.String())
	}
	w = requestJSON(t, server, "GET", "/api/conversations/"+r.ConversationID+"/file?path=handoff.md", t1, nil)
	if w.Code != 200 || w.Body.String() != "previous stage output" {
		t.Fatalf("workspace handoff unavailable: %d %s", w.Code, w.Body.String())
	}
	os.WriteFile(filepath.Join(workspace, "preview.html"), []byte("<style>body{color:blue}</style><script>document.body.textContent='prototype'</script>"), 0600)
	previewURL := "/api/conversations/" + r.ConversationID + "/file?path=preview.html&preview=1"
	w = requestJSON(t, server, "GET", previewURL, t1, nil)
	policy := w.Header().Get("Content-Security-Policy")
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") || !strings.Contains(policy, "sandbox allow-scripts;") || strings.Contains(policy, "allow-same-origin") || !strings.Contains(policy, "default-src 'none'") {
		t.Fatalf("unsafe or unavailable HTML preview: %d %s", w.Code, policy)
	}
	for _, token := range []string{"", t2} {
		w = requestJSON(t, server, "GET", previewURL, token, nil)
		if w.Code != 401 && w.Code != 403 {
			t.Fatalf("preview bypassed authorization: %d", w.Code)
		}
	}
	w = requestJSON(t, server, "GET", previewURL+"&download=1", t1, nil)
	if !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") || strings.Contains(w.Header().Get("Content-Type"), "text/html") {
		t.Fatal("preview flag overrode explicit download")
	}
	w = requestJSON(t, server, "POST", "/api/webhooks/"+a.ID, t1, input)
	if w.Code != 202 {
		t.Fatalf("webhook: %d %s", w.Code, w.Body.String())
	}
	var again Receipt
	json.Unmarshal(w.Body.Bytes(), &again)
	if !again.Duplicate || again.ConversationID != r.ConversationID {
		t.Fatal("webhook delivery duplicated work")
	}
}
func TestCookieMutationRequiresSameOrigin(t *testing.T) {
	s := testStore(t)
	server := NewServer(s, nil, nil, "password", "http://localhost")
	w := requestJSON(t, server, "POST", "/api/login", "", map[string]string{"password": "password"})
	if w.Code != 200 {
		t.Fatalf("login: %d", w.Code)
	}
	cookie := w.Result().Cookies()[0]
	r := httptest.NewRequest("POST", "http://localhost/api/agents", bytes.NewBufferString(`{"name":"test","executor":"codex","enabled":true}`))
	r.AddCookie(cookie)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://other.example")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("cross-origin write accepted: %d", w.Code)
	}
}
func TestArtifactsRejectTraversalAndSymlinks(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "conversations", "abc", "workspace")
	os.MkdirAll(workspace, 0700)
	os.WriteFile(filepath.Join(workspace, "result.txt"), []byte("actual result"), 0600)
	outside := filepath.Join(root, "private.txt")
	os.WriteFile(outside, []byte("private"), 0600)
	os.Symlink(outside, filepath.Join(workspace, "leak.txt"))
	for _, path := range []string{"../private.txt", "leak.txt", "/etc/passwd", ".env", "AGENTS.md"} {
		if _, e := openArtifact(workspace, path); e == nil {
			t.Fatalf("unsafe artifact opened: %s", path)
		}
	}
	file, e := openArtifact(workspace, "result.txt")
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	b := make([]byte, 12)
	file.Read(b)
	if string(b) != "actual resul" {
		t.Fatalf("wrong artifact: %q", b)
	}
}

func TestConditionalStopRejectsNewerInputAndDiscardsOnlyAuthorizedQueue(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	_, token := testUser(t, s, &a, "stop-user")
	sched := NewScheduler(s, nil, 1)
	server := NewServer(s, sched, nil, "password", "http://localhost")
	caller := Caller{Admin: true, Source: "console"}
	first, _ := s.Submit(caller, Input{AgentID: a.ID, UserID: "stop-user", Message: "old"})
	second, _ := s.Submit(caller, Input{ConversationID: first.ConversationID, UserID: "stop-user", Message: "new"})
	path := "/api/conversations/" + first.ConversationID + "/stop"
	w := requestJSON(t, server, "POST", path, token, map[string]any{"expected_message_id": first.MessageID, "discard_queued": true})
	if w.Code != 409 {
		t.Fatalf("stale cancellation affected new work: %d %s", w.Code, w.Body.String())
	}
	w = requestJSON(t, server, "POST", path, token, map[string]any{"expected_message_id": second.MessageID, "discard_queued": true})
	if w.Code != 202 {
		t.Fatalf("stop failed: %d %s", w.Code, w.Body.String())
	}
	sched.Continue(first.ConversationID)
	if _, _, err := s.Claim(); err == nil {
		t.Fatal("withdrawn queued work restarted")
	}
	messages, _ := s.Messages(first.ConversationID)
	if len(messages) != 2 || messages[0].Status != "stopped" || messages[1].Status != "stopped" {
		t.Fatal("history missing or queued work not stopped", messages)
	}
}

func TestReadOnlyConversationCanManageWorkWhileWriterKeepsWorkspace(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	_, token := testUser(t, s, &a, "owner")
	sched := NewScheduler(s, nil, 3)
	server := NewServer(s, sched, nil, "password", "http://localhost")
	caller := Caller{Admin: true, Source: "console"}
	workspace := t.TempDir()
	writer, _ := s.Submit(caller, Input{AgentID: a.ID, UserID: "owner", Message: "write", WorkspacePath: workspace})
	reader, _ := s.Submit(caller, Input{AgentID: a.ID, UserID: "owner", Message: "manage", WorkspacePath: workspace})
	other, _ := s.Submit(caller, Input{AgentID: a.ID, UserID: "owner", Message: "another writer", WorkspacePath: workspace})
	w := requestJSON(t, server, "PATCH", "/api/conversations/"+reader.ConversationID+"/workspace-access", token, map[string]any{"read_only": true})
	if w.Code != 200 {
		t.Fatalf("set access: %d %s", w.Code, w.Body.String())
	}
	_, m, err := s.Claim()
	if err != nil || m.ID != writer.MessageID {
		t.Fatal(m, err)
	}
	c, m, err := s.Claim(workspace)
	if err != nil || m.ID != reader.MessageID {
		t.Fatalf("management was blocked by writer: %v %v", m, err)
	}
	raw, _ := json.Marshal(c)
	if !bytes.Contains(raw, []byte(`"read_only":true`)) {
		t.Fatal("access not persisted", string(raw))
	}
	if _, m, err = s.Claim(workspace); err == nil {
		t.Fatal("second writer ran", other, m)
	}
	w = requestJSON(t, server, "PATCH", "/api/conversations/"+reader.ConversationID+"/workspace-access", token, map[string]any{"read_only": false})
	if w.Code != 409 {
		t.Fatal("changed write rights during a running turn", w.Code)
	}
}
