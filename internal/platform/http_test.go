package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
func TestAPIRequiresCredentialAndDoesNotLeakCrossSourceConversation(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	_, t1, _ := s.CreateCredential("a", []string{a.ID})
	_, t2, _ := s.CreateCredential("b", []string{a.ID})
	server := NewServer(s, nil, nil, "password", "http://localhost")
	input := Input{AgentID: a.ID, UserID: "alice", Message: "private", RequestID: "one"}
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
	for _, path := range []string{"../private.txt", "leak.txt", "/etc/passwd", ".env"} {
		if _, e := openArtifact(root, "abc", path); e == nil {
			t.Fatalf("unsafe artifact opened: %s", path)
		}
	}
	file, e := openArtifact(root, "abc", "result.txt")
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
