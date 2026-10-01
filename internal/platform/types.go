package platform

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

var ErrNotFound = errors.New("record not found")
var ErrForbidden = errors.New("access denied")
var ErrConflict = errors.New("request conflicts with existing state")

type Agent struct {
	ID              string             `json:"id"`
	AuthorizedUsers []string           `json:"authorized_users"`
	Name            string             `json:"name"`
	Executor        string             `json:"executor"`
	Model           string             `json:"model"`
	Instructions    string             `json:"instructions"`
	SeedDir         string             `json:"seed_dir"`
	Skills          []string           `json:"skills"`
	NativeConfig    string             `json:"native_config"`
	InheritEnv      bool               `json:"inherit_env"`
	Env             map[string]*string `json:"env"`
	Sandbox         string             `json:"sandbox"`
	TrustHooks      bool               `json:"trust_hooks"`
	Enabled         bool               `json:"enabled"`
}
type Caller struct {
	Username string
	UserID   string
	Source   string
	Admin    bool
	Agents   []string
}
type Input struct {
	AgentID        string `json:"agent_id"`
	UserID         string `json:"user_id"`
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
	RequestID      string `json:"request_id"`
	WorkspacePath  string `json:"workspace_path,omitempty"`
}
type Conversation struct {
	ID            string `json:"id"`
	AgentID       string `json:"agent_id"`
	AgentName     string `json:"agent_name"`
	UserID        string `json:"user_id"`
	Source        string `json:"source"`
	Title         string `json:"title"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
	ThreadID      string `json:"-"`
	WorkspacePath string `json:"workspace_path"`
	Snapshot      Agent  `json:"-"`
	Created       string `json:"created_at"`
	Updated       string `json:"updated_at"`
}
type Message struct {
	ID             int64  `json:"id"`
	ParentID       int64  `json:"parent_id,omitempty"`
	ConversationID string `json:"conversation_id"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	Kind           string `json:"kind"`
	Status         string `json:"status"`
	Created        string `json:"created_at"`
}
type Event struct {
	ID        int64           `json:"id"`
	MessageID int64           `json:"message_id"`
	Type      string          `json:"type"`
	Raw       json.RawMessage `json:"raw"`
	Created   string          `json:"created_at"`
}
type Receipt struct {
	ConversationID  string `json:"conversation_id"`
	MessageID       int64  `json:"message_id"`
	Status          string `json:"status"`
	ConversationURL string `json:"conversation_url"`
	Duplicate       bool   `json:"duplicate"`
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func allowed(c Caller, a string) bool {
	if c.Admin {
		return true
	}
	for _, v := range c.Agents {
		if v == a {
			return true
		}
	}
	return false
}
