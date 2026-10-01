package platform

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
func testUser(t *testing.T, s *Store, a *Agent, name string) (User, string) {
	t.Helper()
	u, err := s.SaveUser(User{Username: name, UserID: name, Role: "caller", Enabled: true, Password: name + "-password-123"})
	if err != nil {
		t.Fatal(err)
	}
	a.AuthorizedUsers = append(a.AuthorizedUsers, u.UserID)
	*a, err = s.SaveAgent(*a)
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.UserToken(u.UserID, false)
	if err != nil {
		t.Fatal(err)
	}
	return u, token
}
func TestUserTokenAndAccountResumeSameConversation(t *testing.T) {
	s := testStore(t)
	h := NewServer(s, nil, nil, "administrator-password", "http://localhost")
	a := testAgent(t, s)
	a.Instructions = "private configuration"
	u, token := testUser(t, s, &a, "alice")
	_, otherToken := testUser(t, s, &a, "bob")
	caller, err := s.Authenticate(token)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Submit(caller, Input{AgentID: a.ID, Message: "Clarify export", RequestID: "original"})
	if err != nil {
		t.Fatal(err)
	}
	otherCaller, _ := s.Authenticate(otherToken)
	other, _ := s.Submit(otherCaller, Input{AgentID: a.ID, Message: "Other person"})
	login := cookieRequest(t, h, nil, "POST", "/api/login", map[string]string{"username": "alice", "password": "alice-password-123"})
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	for _, path := range []string{"/api/conversations/" + first.ConversationID, "/api/me"} {
		if w := cookieRequest(t, h, cookie, "GET", path, nil); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := cookieRequest(t, h, cookie, "POST", "/api/conversations/"+first.ConversationID+"/messages", map[string]string{"message": "Use JSON"})
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/conversations/" + other.ConversationID, "/api/users"} {
		if w := cookieRequest(t, h, cookie, "GET", path, nil); w.Code != 403 {
			t.Fatal("account isolation", path, w.Code)
		}
	}
	if w := requestJSON(t, h, "POST", "/api/invoke", token, Input{AgentID: a.ID, UserID: "bob", Message: "impersonate"}); w.Code != 403 {
		t.Fatal("token can change user", w.Code)
	}
	agents := cookieRequest(t, h, cookie, "GET", "/api/agents", nil)
	if bytes.Contains(agents.Body.Bytes(), []byte("private configuration")) {
		t.Fatal("configuration exposed")
	}
	// Token reset/revocation affects API credentials, not account login or saved history.
	replacement, err := s.UserToken(u.UserID, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(token); err == nil {
		t.Fatal("old token still works")
	}
	if _, err = s.Authenticate(replacement); err != nil {
		t.Fatal(err)
	}
	s.UserToken(u.UserID, true)
	if w := requestJSON(t, h, "GET", "/api/me", replacement, nil); w.Code != 401 {
		t.Fatal("revoked token accepted")
	}
	h = NewServer(s, nil, nil, "ignored-password", "http://localhost")
	if w := cookieRequest(t, h, cookie, "GET", "/api/conversations/"+first.ConversationID, nil); w.Code != 200 {
		t.Fatal("revocation or restart lost account access", w.Code)
	}
	// Removing an Agent grant immediately affects both auth paths.
	a.AuthorizedUsers = []string{"bob"}
	s.SaveAgent(a)
	if w := cookieRequest(t, h, cookie, "GET", "/api/conversations/"+first.ConversationID, nil); w.Code != 403 {
		t.Fatal("revoked Agent grant still works", w.Code)
	}
	fresh, _ := s.UserToken(u.UserID, false)
	if w := requestJSON(t, h, "POST", "/api/invoke", fresh, Input{AgentID: a.ID, Message: "new"}); w.Code != 403 {
		t.Fatal("ungranted Agent accepted", w.Code)
	}
	if w := requestJSON(t, h, "POST", "/api/invoke", fresh, Input{AgentID: a.ID, Message: "Clarify export", RequestID: "original"}); w.Code != 403 {
		t.Fatal("cached receipt bypassed revoked grant", w.Code)
	}
	if w := cookieRequest(t, h, nil, "POST", "/api/access", map[string]string{"token": "legacy"}); w.Code != 405 {
		t.Fatal("legacy grant route remains", w.Code)
	}
	if w := requestJSON(t, h, "POST", "/api/conversations/"+first.ConversationID+"/web-access", fresh, map[string]any{}); w.Code != 405 {
		t.Fatal("temporary link route remains", w.Code)
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

func TestLegacyAccessMigrationPreservesConversationAndAccount(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := testAgent(t, s)
	receipt, err := s.Submit(Caller{Source: "legacy-source", Admin: true}, Input{AgentID: a.ID, UserID: "external-alice", Message: "Saved input", RequestID: "legacy-request"})
	if err != nil {
		t.Fatal(err)
	}
	s.SetThread(receipt.ConversationID, "native-original")
	s.Close()
	db, err := sql.Open("sqlite3", filepath.Join(dir, "platform.db"))
	if err != nil {
		t.Fatal(err)
	}
	scope, _ := json.Marshal([]string{a.ID})
	_, err = db.Exec(`DROP INDEX user_api_token; DROP TABLE users;
 CREATE TABLE users(username TEXT PRIMARY KEY,role TEXT NOT NULL,source TEXT NOT NULL,user_id TEXT NOT NULL,enabled INTEGER NOT NULL,salt TEXT NOT NULL,password TEXT NOT NULL);
 CREATE TABLE credentials(id TEXT PRIMARY KEY,hash TEXT,name TEXT,agents TEXT,enabled INTEGER);
 CREATE TABLE web_access(hash TEXT PRIMARY KEY);`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO users VALUES('alice','caller','legacy-source','external-alice',1,'salt',?);
 INSERT INTO credentials VALUES('legacy-source',?,'legacy',?,1);
 INSERT INTO browser_sessions VALUES('account-cookie','alice',9999999999);
 INSERT INTO browser_sessions VALUES('anonymous-cookie','',9999999999)`, passwordHash("alice-password-123", "salt"), hashText("legacy-token"), string(scope))
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	caller, err := s.BrowserCaller("unused")
	if err == nil {
		t.Fatal("unknown login accepted")
	}
	// Existing username/password still works; ownership and native history did not change.
	cookie, err := s.Login("alice", "alice-password-123")
	if err != nil {
		t.Fatal(err)
	}
	caller, err = s.BrowserCaller(cookie)
	if err != nil {
		t.Fatal(err)
	}
	conv, err := s.Authorize(caller, receipt.ConversationID, "")
	if err != nil || conv.ThreadID != "native-original" || conv.UserID != "external-alice" {
		t.Fatal("migration lost continuity", conv, err)
	}
	messages, _ := s.Messages(conv.ID)
	if len(messages) != 1 || messages[0].Content != "Saved input" {
		t.Fatal("migration changed input")
	}
	duplicate, err := s.Submit(caller, Input{AgentID: a.ID, UserID: "external-alice", Message: "Saved input", RequestID: "legacy-request"})
	if err != nil || !duplicate.Duplicate || duplicate.ConversationID != conv.ID {
		t.Fatal("migration lost request deduplication", duplicate, err)
	}
	if _, err = s.Authenticate("legacy-token"); err == nil {
		t.Fatal("source credential became user token")
	}
	var grants int
	if err = s.DB.QueryRow(`SELECT count(*) FROM browser_sessions WHERE username=''`).Scan(&grants); err != nil || grants != 0 {
		t.Fatal("anonymous access retained", err)
	}
	// Legacy users table had no default for source: new users must still be creatable.
	if _, err = s.SaveUser(User{Username: "new-user", Role: "caller", Enabled: true, Password: "new-password-123"}); err != nil {
		t.Fatal(err)
	}
}
