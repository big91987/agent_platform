package platform

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type User struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	Source   string `json:"source"`
	UserID   string `json:"user_id"`
	Enabled  bool   `json:"enabled"`
	Password string `json:"password,omitempty"`
}

func passwordHash(password, salt string) string {
	b, _ := pbkdf2.Key(sha256.New, password, []byte(salt), 100000, 32)
	return hex.EncodeToString(b)
}
func (s *Store) initAuth(password string) error {
	_, e := s.DB.Exec(`CREATE TABLE IF NOT EXISTS users(username TEXT PRIMARY KEY,role TEXT NOT NULL,source TEXT NOT NULL,user_id TEXT NOT NULL,enabled INTEGER NOT NULL,salt TEXT NOT NULL,password TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS browser_sessions(hash TEXT PRIMARY KEY,username TEXT NOT NULL DEFAULT '',source TEXT NOT NULL DEFAULT '',user_id TEXT NOT NULL DEFAULT '',conversation_id TEXT NOT NULL DEFAULT '',expires INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS web_access(hash TEXT PRIMARY KEY,source TEXT NOT NULL,user_id TEXT NOT NULL,conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,expires INTEGER NOT NULL);`)
	if e != nil {
		return e
	}
	salt := newID()
	_, e = s.DB.Exec(`INSERT OR IGNORE INTO users VALUES('admin','admin','console','admin',1,?,?)`, salt, passwordHash(password, salt))
	return e
}
func (s *Store) Users() ([]User, error) {
	rows, e := s.DB.Query(`SELECT username,role,source,user_id,enabled FROM users ORDER BY username`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if e = rows.Scan(&u.Username, &u.Role, &u.Source, &u.UserID, &u.Enabled); e != nil {
			return nil, e
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
func (s *Store) SaveUser(u User) (User, error) {
	u.Username = strings.TrimSpace(u.Username)
	if u.Username == "" || len(u.Username) > 128 || (u.Role != "admin" && u.Role != "caller") {
		return u, errors.New("username and role (admin/caller) are required")
	}
	if u.Username == "admin" && (u.Role != "admin" || !u.Enabled) {
		return u, errors.New("built-in admin must remain enabled as administrator")
	}
	if u.Role == "caller" {
		if strings.TrimSpace(u.UserID) == "" {
			return u, errors.New("caller user_id is required")
		}
		if _, e := s.credentialCaller(u.Source); e != nil {
			return u, errors.New("select an enabled caller credential")
		}
	} else {
		u.Source = "console"
		u.UserID = u.Username
	}
	var salt, hash string
	e := s.DB.QueryRow(`SELECT salt,password FROM users WHERE username=?`, u.Username).Scan(&salt, &hash)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return u, e
	}
	if u.Password != "" {
		if len(u.Password) < 12 || len(u.Password) > 256 {
			return u, errors.New("password must contain 12 to 256 characters")
		}
		salt = newID()
		hash = passwordHash(u.Password, salt)
	} else if errors.Is(e, sql.ErrNoRows) {
		return u, errors.New("new user password is required")
	}
	_, e = s.DB.Exec(`INSERT INTO users VALUES(?,?,?,?,?,?,?) ON CONFLICT(username) DO UPDATE SET role=excluded.role,source=excluded.source,user_id=excluded.user_id,enabled=excluded.enabled,salt=excluded.salt,password=excluded.password`, u.Username, u.Role, u.Source, u.UserID, u.Enabled, salt, hash)
	if e == nil {
		_, e = s.DB.Exec(`DELETE FROM browser_sessions WHERE username=?`, u.Username)
	}
	u.Password = ""
	return u, e
}
func (s *Store) credentialCaller(source string) (Caller, error) {
	var scope string
	var enabled bool
	e := s.DB.QueryRow(`SELECT agents,enabled FROM credentials WHERE id=?`, source).Scan(&scope, &enabled)
	if e != nil || !enabled {
		return Caller{}, ErrForbidden
	}
	c := Caller{Source: source}
	e = json.Unmarshal([]byte(scope), &c.Agents)
	return c, e
}
func (s *Store) userCaller(username string) (Caller, error) {
	var u User
	e := s.DB.QueryRow(`SELECT username,role,source,user_id,enabled FROM users WHERE username=?`, username).Scan(&u.Username, &u.Role, &u.Source, &u.UserID, &u.Enabled)
	if e != nil || !u.Enabled {
		return Caller{}, ErrForbidden
	}
	if u.Role == "admin" {
		return Caller{Source: "console", Admin: true, Username: u.Username, UserID: u.UserID}, nil
	}
	c, e := s.credentialCaller(u.Source)
	c.Username = u.Username
	c.UserID = u.UserID
	return c, e
}
func (s *Store) Login(username, password string) (string, error) {
	var salt, hash string
	if e := s.DB.QueryRow(`SELECT salt,password FROM users WHERE username=? AND enabled=1`, username).Scan(&salt, &hash); e != nil {
		return "", ErrForbidden
	}
	if subtle.ConstantTimeCompare([]byte(passwordHash(password, salt)), []byte(hash)) != 1 {
		return "", ErrForbidden
	}
	if _, e := s.userCaller(username); e != nil {
		return "", e
	}
	token := newID() + newID()
	_, e := s.DB.Exec(`INSERT INTO browser_sessions(hash,username,expires) VALUES(?,?,?)`, hashText(token), username, time.Now().Add(24*time.Hour).Unix())
	return token, e
}
func (s *Store) BrowserCaller(token string) (Caller, error) {
	var username, source, user, id string
	var expiry int64
	e := s.DB.QueryRow(`SELECT username,source,user_id,conversation_id,expires FROM browser_sessions WHERE hash=?`, hashText(token)).Scan(&username, &source, &user, &id, &expiry)
	if e != nil || time.Now().Unix() >= expiry {
		return Caller{}, ErrForbidden
	}
	if username != "" {
		return s.userCaller(username)
	}
	c, e := s.credentialCaller(source)
	if e != nil {
		return c, e
	}
	c.UserID = user
	c.ConversationID = id
	_, e = s.Authorize(c, id, user)
	return c, e
}
func (s *Store) WebAccess(c Caller, id, user string, seconds int) (string, int64, error) {
	v, e := s.Authorize(c, id, user)
	if e != nil {
		return "", 0, e
	}
	if v.Source == "console" {
		return "", 0, errors.New("temporary caller links require an external conversation")
	}
	if _, e = s.credentialCaller(v.Source); e != nil {
		return "", 0, e
	}
	if seconds == 0 {
		seconds = 3600
	}
	if seconds < 60 || seconds > 86400 {
		return "", 0, errors.New("expires_in must be between 60 and 86400 seconds")
	}
	token := newID() + newID()
	expires := time.Now().Add(time.Duration(seconds) * time.Second).Unix()
	_, e = s.DB.Exec(`INSERT INTO web_access VALUES(?,?,?,?,?)`, hashText(token), v.Source, v.UserID, id, expires)
	return token, expires, e
}
func (s *Store) ExchangeAccess(token string) (string, string, error) {
	var source, user, id string
	var expires int64
	e := s.DB.QueryRow(`SELECT source,user_id,conversation_id,expires FROM web_access WHERE hash=?`, hashText(token)).Scan(&source, &user, &id, &expires)
	if e != nil || time.Now().Unix() >= expires {
		return "", "", ErrForbidden
	}
	c, e := s.credentialCaller(source)
	if e != nil {
		return "", "", e
	}
	if _, e = s.Authorize(c, id, user); e != nil {
		return "", "", e
	}
	session := newID() + newID()
	_, e = s.DB.Exec(`INSERT INTO browser_sessions(hash,source,user_id,conversation_id,expires) VALUES(?,?,?,?,?)`, hashText(session), source, user, id, expires)
	return session, id, e
}
func (h *Server) cookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: "platform_session", Value: token, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(h.base, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: 86400})
}
func (h *Server) users(w http.ResponseWriter, r *http.Request, c Caller) {
	if r.Method == "GET" {
		v, e := h.store.Users()
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, v)
		return
	}
	var u User
	if e := decode(w, r, &u); e != nil {
		fail(w, e)
		return
	}
	v, e := h.store.SaveUser(u)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (h *Server) webAccess(w http.ResponseWriter, r *http.Request, c Caller) {
	if c.ConversationID != "" {
		fail(w, ErrForbidden)
		return
	}
	var in struct {
		UserID  string `json:"user_id"`
		Expires int    `json:"expires_in"`
	}
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	if c.UserID != "" && !c.Admin {
		if in.UserID != "" && in.UserID != c.UserID {
			fail(w, ErrForbidden)
			return
		}
		in.UserID = c.UserID
	}
	token, expires, e := h.store.WebAccess(c, r.PathValue("id"), in.UserID, in.Expires)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 201, map[string]any{"conversation_id": r.PathValue("id"), "url": h.base + "/conversations/" + r.PathValue("id") + "#access=" + token, "expires_at": time.Unix(expires, 0).UTC().Format(time.RFC3339)})
}
func (h *Server) exchangeAccess(w http.ResponseWriter, r *http.Request) {
	if !h.sameOrigin(r) {
		fail(w, ErrForbidden)
		return
	}
	var in struct {
		Token string `json:"token"`
	}
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	token, id, e := h.store.ExchangeAccess(in.Token)
	if e != nil {
		respond(w, 401, map[string]string{"error": "链接无效、已过期或调用方已停用，请联系发起系统重新获取链接。"})
		return
	}
	h.cookie(w, token)
	respond(w, 200, map[string]string{"conversation_id": id})
}
