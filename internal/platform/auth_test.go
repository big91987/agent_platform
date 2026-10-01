package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func cookieRequest(t *testing.T, h http.Handler, cookie *http.Cookie, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "http://localhost"+path, bytes.NewReader(b))
	if cookie != nil {
		r.AddCookie(cookie)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Platform-Request", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestUserRolesAndScopedBrowserLink(t *testing.T) {
	s := testStore(t)
	h := NewServer(s, nil, nil, "administrator-password", "http://localhost")
	a, _ := s.SaveAgent(Agent{Name: "requirements", Executor: "codex", Enabled: true, Instructions: "private configuration"})
	cred, token, _ := s.CreateCredential("GitHub Runner", []string{a.ID})
	caller, _ := s.Authenticate(token)
	first, _ := s.Submit(caller, Input{AgentID: a.ID, UserID: "alice", Message: "Clarify export"})
	other, _ := s.Submit(caller, Input{AgentID: a.ID, UserID: "bob", Message: "Other person"})
	_, e := s.SaveUser(User{Username: "alice", Role: "caller", Source: cred.ID, UserID: "alice", Enabled: true, Password: "alice-password-123"})
	if e != nil {
		t.Fatal(e)
	}
	login := cookieRequest(t, h, nil, "POST", "/api/login", map[string]string{"username": "alice", "password": "alice-password-123"})
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	if w := cookieRequest(t, h, cookie, "GET", "/api/conversations/"+first.ConversationID, nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/conversations/" + other.ConversationID, "/api/users", "/api/credentials"} {
		if w := cookieRequest(t, h, cookie, "GET", path, nil); w.Code != 403 {
			t.Fatal("role boundary", path, w.Code)
		}
	}
	agents := cookieRequest(t, h, cookie, "GET", "/api/agents", nil)
	if bytes.Contains(agents.Body.Bytes(), []byte("private configuration")) {
		t.Fatal("configuration exposed")
	}
	link, _, e := s.WebAccess(caller, first.ConversationID, "alice", 3600)
	if e != nil {
		t.Fatal(e)
	}
	exchange := cookieRequest(t, h, nil, "POST", "/api/access", map[string]string{"token": link})
	if exchange.Code != 200 {
		t.Fatal(exchange.Code, exchange.Body.String())
	}
	guest := exchange.Result().Cookies()[0]
	if w := cookieRequest(t, h, guest, "POST", "/api/conversations/"+first.ConversationID+"/messages", map[string]string{"message": "Use JSON"}); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, input := range []Input{{ConversationID: other.ConversationID, Message: "wrong"}, {ConversationID: first.ConversationID, UserID: "bob", Message: "wrong"}, {AgentID: a.ID, Message: "new"}} {
		if w := cookieRequest(t, h, guest, "POST", "/api/invoke", input); w.Code != 403 {
			t.Fatal("grant escaped scope", w.Code, w.Body.String())
		}
	}
	list := cookieRequest(t, h, guest, "GET", "/api/conversations", nil)
	var conversations []Conversation
	json.Unmarshal(list.Body.Bytes(), &conversations)
	if len(conversations) != 1 || conversations[0].ID != first.ConversationID {
		t.Fatal("scope list", list.Body.String())
	}
	if w := cookieRequest(t, h, guest, "POST", "/api/conversations/"+first.ConversationID+"/web-access", map[string]int{"expires_in": 3600}); w.Code != 403 {
		t.Fatal("temporary access delegated")
	}
	// Persistent browser credentials survive constructing the server again.
	h = NewServer(s, nil, nil, "ignored-password", "http://localhost")
	if w := cookieRequest(t, h, guest, "GET", "/api/conversations/"+first.ConversationID, nil); w.Code != 200 {
		t.Fatal("restart lost auth", w.Code)
	}
	s.DB.Exec(`UPDATE web_access SET expires=0 WHERE hash=?`, hashText(link))
	if w := cookieRequest(t, h, nil, "POST", "/api/access", map[string]string{"token": link}); w.Code != 401 {
		t.Fatal("expired link accepted")
	}
	s.DB.Exec(`UPDATE browser_sessions SET expires=0 WHERE hash=?`, hashText(guest.Value))
	if w := cookieRequest(t, h, guest, "GET", "/api/me", nil); w.Code != 401 {
		t.Fatal("expired browser access accepted")
	}
	s.RevokeCredential(cred.ID)
	if w := cookieRequest(t, h, cookie, "GET", "/api/me", nil); w.Code != 401 {
		t.Fatal("disabled caller accepted")
	}
}
func TestAccountDisableAndAdminConfiguration(t *testing.T) {
	s := testStore(t)
	h := NewServer(s, nil, nil, "administrator-password", "http://localhost")
	w := cookieRequest(t, h, nil, "POST", "/api/login", map[string]string{"username": "admin", "password": "administrator-password"})
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	admin := w.Result().Cookies()[0]
	w = cookieRequest(t, h, admin, "POST", "/api/users", User{Username: "operator2", Role: "admin", Enabled: true, Password: "operator-password"})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = cookieRequest(t, h, nil, "POST", "/api/login", map[string]string{"username": "operator2", "password": "operator-password"})
	second := w.Result().Cookies()[0]
	w = cookieRequest(t, h, admin, "POST", "/api/users", User{Username: "operator2", Role: "admin", Enabled: false})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = cookieRequest(t, h, second, "GET", "/api/users", nil); w.Code != 401 {
		t.Fatal("disabled account still authorized")
	}
	if w = cookieRequest(t, h, admin, "POST", "/api/users", User{Username: "admin", Role: "caller", Enabled: false}); w.Code != 400 {
		t.Fatal("built-in admin lockout")
	}
	if _, err := s.SaveUser(User{Username: "admin", Role: "admin", Enabled: true, Password: "admin"}); err != nil {
		t.Fatal(err)
	}
	if w = cookieRequest(t, h, admin, "GET", "/api/users", nil); w.Code != 401 {
		t.Fatal("password change did not revoke old login")
	}
	h = NewServer(s, nil, nil, "ignored-password", "http://localhost")
	if w = cookieRequest(t, h, nil, "POST", "/api/login", map[string]string{"username": "admin", "password": "admin"}); w.Code != 200 {
		t.Fatal("admin password not preserved", w.Code)
	}

}
