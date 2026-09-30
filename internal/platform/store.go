package platform

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type Store struct {
	DB   *sql.DB
	Dir  string
	lock *os.File
}

func OpenStore(dir string) (*Store, error) {
	dir, e := filepath.Abs(dir)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	lock, e := os.OpenFile(filepath.Join(dir, "platform.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		lock.Close()
		return nil, fmt.Errorf("data directory already in use: %w", e)
	}
	if e = reconcileNativeProcesses(dir); e != nil {
		lock.Close()
		return nil, e
	}
	db, e := sql.Open("sqlite3", filepath.Join(dir, "platform.db")+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if e != nil {
		lock.Close()
		return nil, e
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db, Dir: dir, lock: lock}
	_, e = db.Exec(`
 CREATE TABLE IF NOT EXISTS agents(id TEXT PRIMARY KEY,config TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS credentials(id TEXT PRIMARY KEY, hash TEXT UNIQUE NOT NULL, name TEXT NOT NULL, agents TEXT NOT NULL, enabled INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS conversations(id TEXT PRIMARY KEY,agent_id TEXT NOT NULL,source TEXT NOT NULL,user_id TEXT NOT NULL,title TEXT NOT NULL,status TEXT NOT NULL,error TEXT NOT NULL DEFAULT '',thread_id TEXT NOT NULL DEFAULT '',snapshot TEXT NOT NULL,created TEXT NOT NULL,updated TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS messages(id INTEGER PRIMARY KEY AUTOINCREMENT,conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,role TEXT NOT NULL,content TEXT NOT NULL,kind TEXT NOT NULL,status TEXT NOT NULL,created TEXT NOT NULL,parent_id INTEGER NOT NULL DEFAULT 0,native_item TEXT NOT NULL DEFAULT '');
 CREATE UNIQUE INDEX IF NOT EXISTS native_message ON messages(conversation_id,parent_id,native_item) WHERE native_item!='';
 CREATE TABLE IF NOT EXISTS events(id INTEGER PRIMARY KEY AUTOINCREMENT,conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,message_id INTEGER NOT NULL,type TEXT NOT NULL,raw TEXT NOT NULL,created TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS requests(source TEXT NOT NULL,key TEXT NOT NULL,hash TEXT NOT NULL,receipt TEXT NOT NULL,PRIMARY KEY(source,key));
 CREATE INDEX IF NOT EXISTS input_queue ON messages(role,status,id);
 CREATE INDEX IF NOT EXISTS conversation_events ON events(conversation_id,id);`)
	if e != nil {
		s.Close()
		return nil, e
	}
	_, e = db.Exec(`UPDATE messages SET status='failed' WHERE role='user' AND status='running'; UPDATE conversations SET status='failed',error='Platform restarted during execution. Previous effects may have completed; review the log before continuing.',updated=? WHERE status='running'; UPDATE conversations SET status='stopped',error='' WHERE status='stopping'`, now())
	if e != nil {
		s.Close()
		return nil, e
	}
	return s, nil
}
func (s *Store) Close() error {
	e := s.DB.Close()
	syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
	s.lock.Close()
	return e
}
func (s *Store) SaveAgent(a Agent) (Agent, error) {
	if strings.TrimSpace(a.Name) == "" {
		return a, errors.New("Agent name is required")
	}
	if a.Executor != "codex" {
		return a, errors.New("only Codex is implemented in this experiment")
	}
	if a.ID == "" {
		a.ID = newID()
	}
	if a.Sandbox == "" {
		a.Sandbox = "workspace-write"
	}
	b, e := json.Marshal(a)
	if e != nil {
		return a, e
	}
	_, e = s.DB.Exec(`INSERT INTO agents VALUES(?,?) ON CONFLICT(id) DO UPDATE SET config=excluded.config`, a.ID, string(b))
	return a, e
}
func (s *Store) Agent(id string) (Agent, error) {
	var raw string
	var a Agent
	e := s.DB.QueryRow(`SELECT config FROM agents WHERE id=?`, id).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	if e != nil {
		return a, e
	}
	e = json.Unmarshal([]byte(raw), &a)
	return a, e
}
func (s *Store) Agents() ([]Agent, error) {
	rows, e := s.DB.Query(`SELECT config FROM agents ORDER BY id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []Agent{}
	for rows.Next() {
		var raw string
		var a Agent
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(raw), &a); e != nil {
			return nil, e
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

type scanner interface{ Scan(...any) error }

const convColumns = "id,agent_id,source,user_id,title,status,error,thread_id,snapshot,created,updated"

func scanConv(row scanner) (Conversation, error) {
	var c Conversation
	var snapshot string
	e := row.Scan(&c.ID, &c.AgentID, &c.Source, &c.UserID, &c.Title, &c.Status, &c.Error, &c.ThreadID, &snapshot, &c.Created, &c.Updated)
	if errors.Is(e, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	if e != nil {
		return c, e
	}
	e = json.Unmarshal([]byte(snapshot), &c.Snapshot)
	c.AgentName = c.Snapshot.Name
	return c, e
}
func (s *Store) Conversation(id string) (Conversation, error) {
	return scanConv(s.DB.QueryRow(`SELECT `+convColumns+` FROM conversations WHERE id=?`, id))
}
func (s *Store) Authorize(c Caller, id, user string) (Conversation, error) {
	v, e := s.Conversation(id)
	if e != nil {
		return v, e
	}
	if !c.Admin && (v.Source != c.Source || v.UserID != user || !allowed(c, v.AgentID)) {
		return v, ErrForbidden
	}
	return v, nil
}
func (s *Store) Conversations(c Caller, user string) ([]Conversation, error) {
	q := `SELECT ` + convColumns + ` FROM conversations`
	args := []any{}
	if !c.Admin {
		q += ` WHERE source=? AND user_id=?`
		args = append(args, c.Source, user)
	}
	q += ` ORDER BY updated DESC LIMIT 200`
	rows, e := s.DB.Query(q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Conversation{}
	for rows.Next() {
		v, e := scanConv(rows)
		if e != nil {
			return nil, e
		}
		if allowed(c, v.AgentID) {
			out = append(out, v)
		}
	}
	return out, rows.Err()
}
func hashText(text string) string { b := sha256.Sum256([]byte(text)); return hex.EncodeToString(b[:]) }
func (s *Store) Submit(c Caller, in Input) (Receipt, error) {
	var out Receipt
	if c.Source == "" || strings.TrimSpace(in.UserID) == "" || strings.TrimSpace(in.Message) == "" {
		return out, errors.New("user_id and message are required")
	}
	if len(in.Message) > 256*1024 || len(in.UserID) > 256 || len(in.RequestID) > 256 {
		return out, errors.New("input exceeds size limit")
	}
	raw, _ := json.Marshal(in)
	fingerprint := hashText(string(raw))
	tx, e := s.DB.Begin()
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if in.RequestID != "" {
		var h, r string
		e = tx.QueryRow(`SELECT hash,receipt FROM requests WHERE source=? AND key=?`, c.Source, in.RequestID).Scan(&h, &r)
		if e == nil {
			if h != fingerprint {
				return out, ErrConflict
			}
			if e = json.Unmarshal([]byte(r), &out); e != nil {
				return out, e
			}
			out.Duplicate = true
			return out, nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return out, e
		}
	}
	var conv Conversation
	if in.ConversationID != "" {
		conv, e = scanConv(tx.QueryRow(`SELECT `+convColumns+` FROM conversations WHERE id=?`, in.ConversationID))
		if e != nil {
			return out, e
		}
		if !c.Admin && (conv.Source != c.Source || conv.UserID != in.UserID || !allowed(c, conv.AgentID)) {
			return out, ErrForbidden
		}
		if in.AgentID != "" && in.AgentID != conv.AgentID {
			return out, ErrConflict
		}
		if conv.Status == "closed" {
			return out, ErrConflict
		}
	} else {
		var a Agent
		var cfg string
		e = tx.QueryRow(`SELECT config FROM agents WHERE id=?`, in.AgentID).Scan(&cfg)
		if errors.Is(e, sql.ErrNoRows) {
			return out, ErrNotFound
		}
		if e != nil {
			return out, e
		}
		if e = json.Unmarshal([]byte(cfg), &a); e != nil {
			return out, e
		}
		if !allowed(c, a.ID) {
			return out, ErrForbidden
		}
		if !a.Enabled {
			return out, errors.New("Agent is disabled")
		}
		title := []rune(strings.TrimSpace(in.Message))
		if len(title) > 60 {
			title = title[:60]
		}
		stamp := now()
		conv = Conversation{ID: newID(), AgentID: a.ID, Source: c.Source, UserID: in.UserID, Title: string(title), Status: "queued", Snapshot: a, Created: stamp, Updated: stamp}
		_, e = tx.Exec(`INSERT INTO conversations(id,agent_id,source,user_id,title,status,snapshot,created,updated)VALUES(?,?,?,?,?,?,?,?,?)`, conv.ID, a.ID, c.Source, in.UserID, conv.Title, conv.Status, cfg, stamp, stamp)
		if e != nil {
			return out, e
		}
	}
	result, e := tx.Exec(`INSERT INTO messages(conversation_id,role,content,kind,status,created)VALUES(?,'user',?,'input','queued',?)`, conv.ID, in.Message, now())
	if e != nil {
		return out, e
	}
	mid, e := result.LastInsertId()
	if e != nil {
		return out, e
	}
	status := conv.Status
	if status == "idle" {
		status = "queued"
	}
	_, e = tx.Exec(`UPDATE conversations SET status=?,updated=? WHERE id=?`, status, now(), conv.ID)
	if e != nil {
		return out, e
	}
	out = Receipt{ConversationID: conv.ID, MessageID: mid, Status: status}
	if in.RequestID != "" {
		b, _ := json.Marshal(out)
		_, e = tx.Exec(`INSERT INTO requests VALUES(?,?,?,?)`, c.Source, in.RequestID, fingerprint, string(b))
		if e != nil {
			return out, e
		}
	}
	return out, tx.Commit()
}
func (s *Store) Messages(id string) ([]Message, error) {
	rows, e := s.DB.Query(`SELECT id,conversation_id,role,content,kind,status,created FROM messages WHERE conversation_id=? ORDER BY id`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if e = rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.Kind, &m.Status, &m.Created); e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) Claim() (Conversation, Message, error) {
	var conv Conversation
	var m Message
	tx, e := s.DB.Begin()
	if e != nil {
		return conv, m, e
	}
	defer tx.Rollback()
	e = tx.QueryRow(`SELECT m.id,m.conversation_id,m.content,m.created FROM messages m JOIN conversations c ON c.id=m.conversation_id WHERE m.role='user' AND m.status='queued' AND c.status IN('queued','idle') ORDER BY m.id LIMIT 1`).Scan(&m.ID, &m.ConversationID, &m.Content, &m.Created)
	if errors.Is(e, sql.ErrNoRows) {
		return conv, m, ErrNotFound
	}
	if e != nil {
		return conv, m, e
	}
	conv, e = scanConv(tx.QueryRow(`SELECT `+convColumns+` FROM conversations WHERE id=?`, m.ConversationID))
	if e != nil {
		return conv, m, e
	}
	_, e = tx.Exec(`UPDATE messages SET status='running' WHERE id=?; UPDATE conversations SET status='running',error='',updated=? WHERE id=?`, m.ID, now(), conv.ID)
	if e != nil {
		return conv, m, e
	}
	conv.Status = "running"
	m.Role = "user"
	m.Kind = "input"
	m.Status = "running"
	return conv, m, tx.Commit()
}
func (s *Store) Halt(id, status, reason string) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	c, e := scanConv(tx.QueryRow(`SELECT `+convColumns+` FROM conversations WHERE id=?`, id))
	if e != nil {
		return e
	}
	if c.Status == "closed" && status != "closed" {
		return ErrConflict
	}
	_, e = tx.Exec(`UPDATE conversations SET status=?,error=?,updated=? WHERE id=?; UPDATE messages SET status='stopped' WHERE conversation_id=? AND role='user' AND status='running'`, status, reason, now(), id, id)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Continue(id string) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	c, e := scanConv(tx.QueryRow(`SELECT `+convColumns+` FROM conversations WHERE id=?`, id))
	if e != nil {
		return e
	}
	if c.Status == "closed" || c.Status == "running" {
		return ErrConflict
	}
	var count int
	e = tx.QueryRow(`SELECT count(*) FROM messages WHERE conversation_id=? AND role='user' AND status='queued'`, id).Scan(&count)
	if e != nil {
		return e
	}
	state := "idle"
	if count > 0 {
		state = "queued"
	}
	_, e = tx.Exec(`UPDATE conversations SET status=?,error='',updated=? WHERE id=?`, state, now(), id)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Complete(id string, mid int64, status, reason string) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	c, e := scanConv(tx.QueryRow(`SELECT `+convColumns+` FROM conversations WHERE id=?`, id))
	if e != nil {
		return e
	}
	if c.Status == "stopped" || c.Status == "closed" {
		return nil
	}
	state := "failed"
	if status == "completed" {
		state = "idle"
		var n int
		e = tx.QueryRow(`SELECT count(*) FROM messages WHERE conversation_id=? AND status='queued' AND role='user'`, id).Scan(&n)
		if e != nil {
			return e
		}
		if n > 0 {
			state = "queued"
		}
	}
	_, e = tx.Exec(`UPDATE messages SET status=? WHERE id=?; UPDATE conversations SET status=?,error=?,updated=? WHERE id=?`, status, mid, state, reason, now(), id)
	if e != nil {
		return e
	}
	if status == "completed" {
		_, e = tx.Exec(`UPDATE messages SET kind='reply' WHERE id=(SELECT max(id) FROM messages WHERE conversation_id=? AND parent_id=? AND role='agent')`, id, mid)
		if e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) SetThread(id, thread string) error {
	_, e := s.DB.Exec(`UPDATE conversations SET thread_id=?,updated=? WHERE id=? AND (thread_id='' OR thread_id=?)`, thread, now(), id, thread)
	return e
}
func (s *Store) RecordEvent(id string, mid int64, raw []byte) error {
	var event struct {
		Type string `json:"type"`
		Item struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"item"`
	}
	if e := json.Unmarshal(raw, &event); e != nil {
		return e
	}
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	stamp := now()
	_, e = tx.Exec(`INSERT INTO events(conversation_id,message_id,type,raw,created)VALUES(?,?,?,?,?)`, id, mid, event.Type, string(raw), stamp)
	if e != nil {
		return e
	}
	if event.Item.Type == "agent_message" && event.Item.Text != "" && (event.Type == "item.completed" || event.Type == "item.updated") {
		_, e = tx.Exec(`INSERT INTO messages(conversation_id,role,content,kind,status,created,parent_id,native_item)VALUES(?,'agent',?,'progress','completed',?,?,?) ON CONFLICT(conversation_id,parent_id,native_item) WHERE native_item!='' DO UPDATE SET content=excluded.content`, id, event.Item.Text, stamp, mid, event.Item.ID)
		if e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) Events(id string, after int64) ([]Event, error) {
	rows, e := s.DB.Query(`SELECT id,message_id,type,raw,created FROM events WHERE conversation_id=? AND id>? ORDER BY id LIMIT 300`, id, after)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var v Event
		var raw string
		if e = rows.Scan(&v.ID, &v.MessageID, &v.Type, &raw, &v.Created); e != nil {
			return nil, e
		}
		v.Raw = json.RawMessage(raw)
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) CreateCredential(name string, agents []string) (Credential, string, error) {
	if strings.TrimSpace(name) == "" || len(agents) == 0 {
		return Credential{}, "", errors.New("name and allowed Agents are required")
	}
	for _, id := range agents {
		if _, e := s.Agent(id); e != nil {
			return Credential{}, "", e
		}
	}
	c := Credential{ID: newID(), Name: name, Agents: agents, Enabled: true}
	token := "ap_" + newID() + newID()
	b, _ := json.Marshal(agents)
	_, e := s.DB.Exec(`INSERT INTO credentials VALUES(?,?,?,?,1)`, c.ID, hashText(token), name, string(b))
	return c, token, e
}
func (s *Store) Authenticate(token string) (Caller, error) {
	var id, scope string
	var enabled bool
	e := s.DB.QueryRow(`SELECT id,agents,enabled FROM credentials WHERE hash=?`, hashText(token)).Scan(&id, &scope, &enabled)
	if e != nil || !enabled {
		return Caller{}, ErrForbidden
	}
	var agents []string
	if e = json.Unmarshal([]byte(scope), &agents); e != nil {
		return Caller{}, e
	}
	return Caller{Source: id, Agents: agents}, nil
}
func (s *Store) Credentials() ([]Credential, error) {
	rows, e := s.DB.Query(`SELECT id,name,agents,enabled FROM credentials`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Credential{}
	for rows.Next() {
		var c Credential
		var raw string
		if e = rows.Scan(&c.ID, &c.Name, &raw, &c.Enabled); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(raw), &c.Agents); e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) RevokeCredential(id string) error {
	_, e := s.DB.Exec(`UPDATE credentials SET enabled=0 WHERE id=?`, id)
	return e
}
