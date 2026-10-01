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
	Username     string `json:"username"`
	Role         string `json:"role"`
	UserID       string `json:"user_id"`
	Enabled      bool   `json:"enabled"`
	TokenEnabled bool   `json:"token_enabled"`
	Password     string `json:"password,omitempty"`
}

func passwordHash(password, salt string) string {
	b, _ := pbkdf2.Key(sha256.New, password, []byte(salt), 100000, 32)
	return hex.EncodeToString(b)
}

// Upgrade account ownership once; never recreate conversations or native histories.
func (s *Store) initUsers() error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS users(username TEXT PRIMARY KEY,role TEXT NOT NULL,source TEXT NOT NULL DEFAULT '',user_id TEXT NOT NULL,enabled INTEGER NOT NULL,salt TEXT NOT NULL,password TEXT NOT NULL,token_hash TEXT NOT NULL DEFAULT '');`)
	if err != nil {
		return err
	}
	rows, err := tx.Query(`PRAGMA table_info(users)`)
	if err != nil {
		return err
	}
	hasToken := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var def any
		if err = rows.Scan(&cid, &name, &typ, &notnull, &def, &pk); err != nil {
			rows.Close()
			return err
		}
		hasToken = hasToken || name == "token_hash"
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !hasToken {
		// Legacy keys only proved a source, not a user. They cannot be reused as user tokens.
		if _, err = tx.Exec(`ALTER TABLE users ADD COLUMN token_hash TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
		type legacyUser struct{ name, source, id, role string }
		rows, err = tx.Query(`SELECT username,source,user_id,role FROM users ORDER BY username`)
		if err != nil {
			return err
		}
		var users []legacyUser
		pairs := map[string]bool{}
		for rows.Next() {
			var u legacyUser
			if err = rows.Scan(&u.name, &u.source, &u.id, &u.role); err != nil {
				rows.Close()
				return err
			}
			pair := u.source + "\x00" + u.id
			if pairs[pair] {
				rows.Close()
				return errors.New("ambiguous legacy account ownership; migration requires explicit mapping")
			}
			pairs[pair] = true
			users = append(users, u)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		var credentials int
		if err = tx.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='credentials'`).Scan(&credentials); err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, u := range users {
			id := u.id
			if id == "" || seen[id] {
				id = newID()
			}
			seen[id] = true
			if _, err = tx.Exec(`UPDATE conversations SET user_id=? WHERE source=? AND user_id=?`, id, u.source, u.id); err != nil {
				return err
			}
			if _, err = tx.Exec(`UPDATE users SET user_id=? WHERE username=?`, id, u.name); err != nil {
				return err
			}
			if credentials == 0 || u.role == "admin" {
				continue
			}
			var scope string
			err = tx.QueryRow(`SELECT agents FROM credentials WHERE id=? AND enabled=1`, u.source).Scan(&scope)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			var agents []string
			if err = json.Unmarshal([]byte(scope), &agents); err != nil {
				return err
			}
			for _, aid := range agents {
				var raw string
				if err = tx.QueryRow(`SELECT config FROM agents WHERE id=?`, aid).Scan(&raw); err != nil {
					return err
				}
				var a Agent
				if err = json.Unmarshal([]byte(raw), &a); err != nil {
					return err
				}
				found := false
				for _, v := range a.AuthorizedUsers {
					found = found || v == id
				}
				if !found {
					a.AuthorizedUsers = append(a.AuthorizedUsers, id)
				}
				b, _ := json.Marshal(a)
				if _, err = tx.Exec(`UPDATE agents SET config=? WHERE id=?`, string(b), aid); err != nil {
					return err
				}
			}
		}
		// Existing account logins remain valid. Anonymous temporary access is withdrawn.
		if _, err = tx.Exec(`DELETE FROM browser_sessions WHERE username=''`); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS user_identity ON users(user_id);
 CREATE UNIQUE INDEX IF NOT EXISTS user_api_token ON users(token_hash) WHERE token_hash!='';
 CREATE TABLE IF NOT EXISTS browser_sessions(hash TEXT PRIMARY KEY,username TEXT NOT NULL,expires INTEGER NOT NULL);
 DROP TABLE IF EXISTS web_access; DROP TABLE IF EXISTS credentials;`)
	if err != nil {
		return err
	}
	// Carry existing deduplication receipts into the same stable user namespace.
	// A conflicting key fails the transaction rather than replaying an uncertain task.
	_, err = tx.Exec(`UPDATE requests SET source=(
 SELECT 'user:'||c.user_id FROM conversations c JOIN users u ON u.user_id=c.user_id
 WHERE c.id=json_extract(requests.receipt,'$.conversation_id'))
 WHERE source NOT LIKE 'user:%' AND EXISTS(
 SELECT 1 FROM conversations c JOIN users u ON u.user_id=c.user_id
 WHERE c.id=json_extract(requests.receipt,'$.conversation_id'))`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) initAuth(password string) error {
	salt := newID()
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO users(username,role,source,user_id,enabled,salt,password) VALUES('admin','admin','','admin',1,?,?)`, salt, passwordHash(password, salt))
	return err
}
func (s *Store) Users() ([]User, error) {
	rows, err := s.DB.Query(`SELECT username,role,user_id,enabled,token_hash!='' FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err = rows.Scan(&u.Username, &u.Role, &u.UserID, &u.Enabled, &u.TokenEnabled); err != nil {
			return nil, err
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
	var salt, hash, id string
	err := s.DB.QueryRow(`SELECT salt,password,user_id FROM users WHERE username=?`, u.Username).Scan(&salt, &hash, &id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return u, err
	}
	if err == nil {
		if u.UserID != "" && u.UserID != id {
			return u, errors.New("user_id is immutable")
		}
		u.UserID = id
	} else if u.UserID == "" {
		u.UserID = newID()
	}
	if len(u.UserID) > 256 || strings.TrimSpace(u.UserID) == "" {
		return u, errors.New("invalid user_id")
	}
	var duplicates int
	if e := s.DB.QueryRow(`SELECT count(*) FROM users WHERE user_id=? AND username!=?`, u.UserID, u.Username).Scan(&duplicates); e != nil {
		return u, e
	}
	if duplicates > 0 {
		return u, errors.New("user_id already belongs to another account")
	}
	if u.Password != "" {
		if (len(u.Password) < 12 && !(u.Username == "admin" && u.Password == "admin")) || len(u.Password) > 256 {
			return u, errors.New("password must contain 12 to 256 characters")
		}
		salt = newID()
		hash = passwordHash(u.Password, salt)
	} else if errors.Is(err, sql.ErrNoRows) {
		return u, errors.New("new user password is required")
	}
	tx, e := s.DB.Begin()
	if e != nil {
		return u, e
	}
	defer tx.Rollback()
	_, e = tx.Exec(`INSERT INTO users(username,role,source,user_id,enabled,salt,password) VALUES(?,?,'',?,?,?,?) ON CONFLICT(username) DO UPDATE SET role=excluded.role,enabled=excluded.enabled,salt=excluded.salt,password=excluded.password`, u.Username, u.Role, u.UserID, u.Enabled, salt, hash)
	if e != nil {
		return u, e
	}
	if _, e = tx.Exec(`DELETE FROM browser_sessions WHERE username=?`, u.Username); e != nil {
		return u, e
	}
	u.Password = ""
	return u, tx.Commit()
}
func (s *Store) userCaller(username string) (Caller, error) {
	var u User
	err := s.DB.QueryRow(`SELECT username,role,user_id,enabled FROM users WHERE username=?`, username).Scan(&u.Username, &u.Role, &u.UserID, &u.Enabled)
	if err != nil || !u.Enabled {
		return Caller{}, ErrForbidden
	}
	c := Caller{Source: "user:" + u.UserID, Username: u.Username, UserID: u.UserID, Admin: u.Role == "admin"}
	agents, err := s.Agents()
	if err != nil {
		return Caller{}, err
	}
	for _, a := range agents {
		for _, id := range a.AuthorizedUsers {
			if id == u.UserID {
				c.Agents = append(c.Agents, a.ID)
				break
			}
		}
	}
	return c, nil
}
func (s *Store) Login(username, password string) (string, error) {
	var salt, hash string
	if err := s.DB.QueryRow(`SELECT salt,password FROM users WHERE username=? AND enabled=1`, username).Scan(&salt, &hash); err != nil {
		return "", ErrForbidden
	}
	if subtle.ConstantTimeCompare([]byte(passwordHash(password, salt)), []byte(hash)) != 1 {
		return "", ErrForbidden
	}
	if _, err := s.userCaller(username); err != nil {
		return "", err
	}
	token := newID() + newID()
	_, err := s.DB.Exec(`INSERT INTO browser_sessions(hash,username,expires) VALUES(?,?,?)`, hashText(token), username, time.Now().Add(24*time.Hour).Unix())
	return token, err
}
func (s *Store) BrowserCaller(token string) (Caller, error) {
	var username string
	var expiry int64
	err := s.DB.QueryRow(`SELECT username,expires FROM browser_sessions WHERE hash=?`, hashText(token)).Scan(&username, &expiry)
	if err != nil || username == "" || time.Now().Unix() >= expiry {
		return Caller{}, ErrForbidden
	}
	return s.userCaller(username)
}
func (s *Store) UserToken(id string, revoke bool) (string, error) {
	var enabled bool
	if err := s.DB.QueryRow(`SELECT enabled FROM users WHERE user_id=?`, id).Scan(&enabled); err != nil {
		return "", ErrNotFound
	}
	if !enabled && !revoke {
		return "", ErrForbidden
	}
	token, hash := "", ""
	if !revoke {
		token = "ap_" + newID() + newID()
		hash = hashText(token)
	}
	_, err := s.DB.Exec(`UPDATE users SET token_hash=? WHERE user_id=?`, hash, id)
	return token, err
}
func (s *Store) Authenticate(token string) (Caller, error) {
	if token == "" {
		return Caller{}, ErrForbidden
	}
	var username string
	if err := s.DB.QueryRow(`SELECT username FROM users WHERE token_hash=? AND enabled=1`, hashText(token)).Scan(&username); err != nil {
		return Caller{}, ErrForbidden
	}
	return s.userCaller(username)
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
func (h *Server) userToken(w http.ResponseWriter, r *http.Request, c Caller) {
	token, e := h.store.UserToken(r.PathValue("id"), r.Method == "DELETE")
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]any{"user_id": r.PathValue("id"), "token": token, "revoked": r.Method == "DELETE"})
}
