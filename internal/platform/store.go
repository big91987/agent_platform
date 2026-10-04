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
	var workspaceColumn int
	if e = db.QueryRow(`SELECT count(*) FROM pragma_table_info('conversations') WHERE name='workspace_path'`).Scan(&workspaceColumn); e != nil {
		s.Close()
		return nil, e
	}
	if workspaceColumn == 0 {
		if _, e = db.Exec(`ALTER TABLE conversations ADD COLUMN workspace_path TEXT NOT NULL DEFAULT ''`); e != nil {
			s.Close()
			return nil, e
		}
	}
	var accessColumn int
	if e = db.QueryRow(`SELECT count(*) FROM pragma_table_info('conversations') WHERE name='read_only'`).Scan(&accessColumn); e != nil {
		s.Close()
		return nil, e
	}
	if accessColumn == 0 {
		if _, e = db.Exec(`ALTER TABLE conversations ADD COLUMN read_only INTEGER NOT NULL DEFAULT 0`); e != nil {
			s.Close()
			return nil, e
		}
	}
	if e = s.initWorkflows(); e != nil {
		s.Close()
		return nil, e
	}
	if e = s.initApprovals(); e != nil {
		s.Close()
		return nil, e
	}
	if e = s.initToolServers(); e != nil {
		s.Close()
		return nil, e
	}
	if e = s.initUsers(); e != nil {
		s.Close()
		return nil, e
	}
	_, e = db.Exec(`UPDATE messages SET status='failed' WHERE role='user' AND status IN('running','steering'); UPDATE conversations SET status='failed',error='Platform restarted during execution. Previous effects may have completed; review the log before continuing.',updated=? WHERE status='running'; UPDATE conversations SET status='stopped',error='' WHERE status='stopping'`, now())
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
	if len(a.ResolvedTools) > 0 {
		return a, errors.New("resolved_tools is managed by the platform")
	}
	if _, e := resolveToolServers(s.DB, a); e != nil {
		return a, e
	}
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
	for _, id := range a.AuthorizedUsers {
		var n int
		if e := s.DB.QueryRow(`SELECT count(*) FROM users WHERE user_id=?`, id).Scan(&n); e != nil {
			return a, e
		}
		if n != 1 {
			return a, errors.New("authorized user does not exist")
		}
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

const convColumns = "id,agent_id,source,user_id,title,status,error,thread_id,workspace_path,read_only,snapshot,created,updated"

func scanConv(row scanner) (Conversation, error) {
	var c Conversation
	var snapshot string
	e := row.Scan(&c.ID, &c.AgentID, &c.Source, &c.UserID, &c.Title, &c.Status, &c.Error, &c.ThreadID, &c.WorkspacePath, &c.ReadOnly, &snapshot, &c.Created, &c.Updated)
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
	if c.UserID != "" && !c.Admin {
		if user != "" && user != c.UserID {
			return Conversation{}, ErrForbidden
		}
		user = c.UserID
	}
	v, e := s.Conversation(id)
	if e != nil {
		return v, e
	}
	if !c.Admin && (c.UserID == "" || v.UserID != c.UserID || !allowed(c, v.AgentID)) {
		return v, ErrForbidden
	}
	return v, nil
}
func (s *Store) Conversations(c Caller, user string) ([]Conversation, error) {
	if c.UserID != "" && !c.Admin {
		if user != "" && user != c.UserID {
			return nil, ErrForbidden
		}
		user = c.UserID
	}
	q := `SELECT ` + convColumns + ` FROM conversations`
	args := []any{}
	if !c.Admin {
		q += ` WHERE user_id=?`
		args = append(args, c.UserID)
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
	if !c.Admin && c.UserID != "" {
		if in.UserID != "" && in.UserID != c.UserID {
			return out, ErrForbidden
		}
		in.UserID = c.UserID
	}
	if !c.Admin && c.UserID == "" {
		return out, ErrForbidden
	}
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
			if !c.Admin {
				var owner, agent string
				if e = tx.QueryRow(`SELECT user_id,agent_id FROM conversations WHERE id=?`, out.ConversationID).Scan(&owner, &agent); e != nil {
					return out, ErrNotFound
				}
				if owner != c.UserID || !allowed(c, agent) {
					return out, ErrForbidden
				}
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
		if in.NetworkAccess != nil {
			return out, errors.New("change existing conversation network through network-access endpoint")
		}
		conv, e = scanConv(tx.QueryRow(`SELECT `+convColumns+` FROM conversations WHERE id=?`, in.ConversationID))
		if e != nil {
			return out, e
		}
		if !c.Admin && (conv.UserID != c.UserID || !allowed(c, conv.AgentID)) {
			return out, ErrForbidden
		}
		if in.AgentID != "" && in.AgentID != conv.AgentID {
			return out, ErrConflict
		}
		if in.WorkspacePath != "" {
			workspace, err := normalizeWorkspace(in.WorkspacePath)
			if err != nil {
				return out, err
			}
			if workspace != conv.WorkspacePath {
				return out, fmt.Errorf("workspace is fixed for this conversation: %w", ErrConflict)
			}
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
		workspace := ""
		if in.WorkspacePath != "" {
			workspace, e = normalizeWorkspace(in.WorkspacePath)
			if e != nil {
				return out, e
			}
			// Native records and credentials are not project workspaces.
			dataDir, err := normalizeWorkspace(s.Dir)
			if err != nil {
				return out, err
			}
			if containsPath(dataDir, workspace) || containsPath(workspace, dataDir) {
				return out, errors.New("workspace must be outside the platform data directory")
			}
		}
		title := []rune(strings.TrimSpace(in.Message))
		if len(title) > 60 {
			title = title[:60]
		}
		a, e = resolveToolServers(tx, a)
		if e != nil {
			return out, e
		}
		if in.NetworkAccess != nil {
			if *in.NetworkAccess && !a.NetworkAccess {
				return out, ErrForbidden
			}
			a.NetworkAccess = *in.NetworkAccess
			if !a.NetworkAccess {
				a.AllowElevation = false
			}
		}
		snapshot, e := json.Marshal(a)
		if e != nil {
			return out, e
		}
		cfg = string(snapshot)
		stamp := now()
		conv = Conversation{ID: newID(), AgentID: a.ID, Source: c.Source, UserID: in.UserID, Title: string(title), Status: "queued", WorkspacePath: workspace, Snapshot: a, Created: stamp, Updated: stamp}
		_, e = tx.Exec(`INSERT INTO conversations(id,agent_id,source,user_id,title,status,workspace_path,snapshot,created,updated)VALUES(?,?,?,?,?,?,?,?,?,?)`, conv.ID, a.ID, c.Source, in.UserID, conv.Title, conv.Status, workspace, cfg, stamp, stamp)
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
	rows, e := s.DB.Query(`SELECT id,parent_id,conversation_id,role,content,kind,status,created,native_item FROM messages WHERE conversation_id=? ORDER BY id`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if e = rows.Scan(&m.ID, &m.ParentID, &m.ConversationID, &m.Role, &m.Content, &m.Kind, &m.Status, &m.Created, &m.NativeItem); e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) Claim(busyWorkspaces ...string) (Conversation, Message, error) {
	var conv Conversation
	var m Message
	tx, e := s.DB.Begin()
	if e != nil {
		return conv, m, e
	}
	defer tx.Rollback()
	query := `SELECT m.id,m.conversation_id,m.content,m.created FROM messages m JOIN conversations c ON c.id=m.conversation_id WHERE m.role='user' AND m.status='queued' AND c.status IN('queued','idle') AND (c.read_only=1 OR c.workspace_path='' OR NOT EXISTS(SELECT 1 FROM conversations active WHERE active.workspace_path=c.workspace_path AND active.read_only=0 AND active.status IN('running','stopping')))`
	args := []any{}
	for _, path := range busyWorkspaces {
		if path != "" {
			query += ` AND (c.read_only=1 OR c.workspace_path!=?)`
			args = append(args, path)
		}
	}
	e = tx.QueryRow(query+` ORDER BY m.id LIMIT 1`, args...).Scan(&m.ID, &m.ConversationID, &m.Content, &m.Created)
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
	return s.HaltExpected(id, status, reason, 0, false)
}
func (s *Store) HaltExpected(id, status, reason string, expected int64, discard bool) error {
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
	if expected > 0 {
		var latest int64
		if e = tx.QueryRow(`SELECT COALESCE(MAX(id),0) FROM messages WHERE conversation_id=? AND role='user'`, id).Scan(&latest); e != nil {
			return e
		}
		if latest != expected {
			return ErrConflict
		}
	}
	if discard {
		if expected <= 0 {
			return ErrConflict
		}
		if _, e = tx.Exec(`UPDATE messages SET status='stopped' WHERE conversation_id=? AND role='user' AND status IN ('queued','steering')`, id); e != nil {
			return e
		}
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
