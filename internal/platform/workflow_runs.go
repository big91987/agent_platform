package platform

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

type WorkflowStart struct {
	WorkflowID    string `json:"workflow_id"`
	StartNode     string `json:"start_node,omitempty"`
	Input         string `json:"input"`
	WorkspacePath string `json:"workspace_path"`
	RequestID     string `json:"request_id,omitempty"`
}
type NodeResult struct {
	Route     string   `json:"route"`
	Summary   string   `json:"summary"`
	Artifacts []string `json:"artifacts,omitempty"`
}
type WorkflowStep struct {
	ConnectorDispatched bool              `json:"connector_dispatched,omitempty"`
	Receipt             *ConnectorReceipt `json:"connector_receipt,omitempty"`
	Seq                 int               `json:"seq"`
	NodeID              string            `json:"node_id"`
	Status              string            `json:"status"`
	ConversationID      string            `json:"conversation_id,omitempty"`
	Result              *NodeResult       `json:"result,omitempty"`
	Error               string            `json:"error,omitempty"`
	Created             string            `json:"created_at"`
	Updated             string            `json:"updated_at"`
}
type WorkflowRun struct {
	Connectors    map[string]Connector `json:"connectors,omitempty"`
	ID            string               `json:"id"`
	WorkflowID    string               `json:"workflow_id"`
	Owner         string               `json:"owner"`
	Username      string               `json:"-"`
	Definition    Workflow             `json:"definition"`
	Input         string               `json:"input"`
	WorkspacePath string               `json:"workspace_path"`
	Status        string               `json:"status"`
	Seq           int                  `json:"seq"`
	Error         string               `json:"error,omitempty"`
	Created       string               `json:"created_at"`
	Updated       string               `json:"updated_at"`
	Steps         []WorkflowStep       `json:"steps"`
}

func (w Workflow) node(id string) WorkflowNode {
	for _, n := range w.Nodes {
		if n.ID == id {
			return n
		}
	}
	return WorkflowNode{}
}
func (w Workflow) target(id, route string) string {
	for _, e := range w.Edges {
		if e.Source == id && e.Route == route {
			return e.Target
		}
	}
	return ""
}
func (w Workflow) routes(id string) []string {
	out := []string{}
	for _, e := range w.Edges {
		if e.Source == id {
			out = append(out, e.Route)
		}
	}
	return out
}
func (s *Store) initWorkflowRuns() error {
	_, e := s.DB.Exec(`CREATE TABLE IF NOT EXISTS workflow_runs (
 id TEXT PRIMARY KEY, workflow_id TEXT NOT NULL, owner TEXT NOT NULL, username TEXT NOT NULL,
 definition TEXT NOT NULL,input TEXT NOT NULL,workspace TEXT NOT NULL,status TEXT NOT NULL,seq INTEGER NOT NULL,
 error TEXT NOT NULL DEFAULT '',created TEXT NOT NULL,updated TEXT NOT NULL,request_key TEXT,request_hash TEXT NOT NULL,
 UNIQUE(owner,request_key));
 CREATE TABLE IF NOT EXISTS workflow_steps (
 run_id TEXT NOT NULL REFERENCES workflow_runs(id),seq INTEGER NOT NULL,node_id TEXT NOT NULL,status TEXT NOT NULL,
 conversation_id TEXT NOT NULL DEFAULT '',token_hash TEXT NOT NULL DEFAULT '',result TEXT NOT NULL DEFAULT '',
 error TEXT NOT NULL DEFAULT '',created TEXT NOT NULL,updated TEXT NOT NULL,PRIMARY KEY(run_id,seq));
 CREATE UNIQUE INDEX IF NOT EXISTS workflow_conversation ON workflow_steps(conversation_id) WHERE conversation_id!='';`)
	return e
}
func loadWorkflowRun(tx *sql.Tx, id string) (WorkflowRun, error) {
	var r WorkflowRun
	var raw string
	e := tx.QueryRow(`SELECT id,workflow_id,owner,username,definition,input,workspace,status,seq,error,created,updated FROM workflow_runs WHERE id=?`, id).Scan(&r.ID, &r.WorkflowID, &r.Owner, &r.Username, &raw, &r.Input, &r.WorkspacePath, &r.Status, &r.Seq, &r.Error, &r.Created, &r.Updated)
	if errors.Is(e, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal([]byte(raw), &r.Definition); e != nil {
		return r, e
	}
	r.Connectors = map[string]Connector{}
	configs, e := tx.Query(`SELECT connector_id,definition FROM workflow_run_connectors WHERE run_id=?`, id)
	if e != nil {
		return r, e
	}
	for configs.Next() {
		var key, raw string
		var v Connector
		if e = configs.Scan(&key, &raw); e != nil {
			configs.Close()
			return r, e
		}
		if e = json.Unmarshal([]byte(raw), &v); e != nil {
			configs.Close()
			return r, e
		}
		r.Connectors[key] = v
	}
	e = configs.Err()
	configs.Close()
	if e != nil {
		return r, e
	}
	rows, e := tx.Query(`SELECT s.seq,s.node_id,s.status,s.conversation_id,s.result,s.error,s.created,s.updated,COALESCE(a.receipt,''),a.request IS NOT NULL FROM workflow_steps s LEFT JOIN workflow_connector_actions a ON a.run_id=s.run_id AND a.seq=s.seq WHERE s.run_id=? ORDER BY s.seq`, id)
	if e != nil {
		return r, e
	}
	defer rows.Close()
	r.Steps = []WorkflowStep{}
	for rows.Next() {
		var step WorkflowStep
		var result, receipt string
		if e = rows.Scan(&step.Seq, &step.NodeID, &step.Status, &step.ConversationID, &result, &step.Error, &step.Created, &step.Updated, &receipt, &step.ConnectorDispatched); e != nil {
			return r, e
		}
		if receipt != "" {
			if e = json.Unmarshal([]byte(receipt), &step.Receipt); e != nil {
				return r, e
			}
		}
		if result != "" {
			if e = json.Unmarshal([]byte(result), &step.Result); e != nil {
				return r, e
			}
		}
		r.Steps = append(r.Steps, step)
	}
	return r, rows.Err()
}
func runAllowed(c Caller, r WorkflowRun) bool {
	return c.Admin || c.UserID != "" && c.UserID == r.Owner
}
func (s *Store) WorkflowRun(c Caller, id string) (WorkflowRun, error) {
	tx, e := s.DB.Begin()
	if e != nil {
		return WorkflowRun{}, e
	}
	defer tx.Rollback()
	r, e := loadWorkflowRun(tx, id)
	if e == nil && !runAllowed(c, r) {
		return WorkflowRun{}, ErrForbidden
	}
	return r, e
}
func (s *Store) WorkflowRuns(c Caller) ([]WorkflowRun, error) {
	rows, e := s.DB.Query(`SELECT id FROM workflow_runs WHERE owner=? OR ? ORDER BY created DESC LIMIT 200`, c.UserID, c.Admin)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []WorkflowRun{}
	for _, id := range ids {
		r, e := s.WorkflowRun(c, id)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, nil
}

// Every dispatch checks current account and Agent authorization, even for frozen graphs.
func (s *Store) workflowCaller(r WorkflowRun) (Caller, error) { return s.userCaller(r.Username) }
func (s *Store) StartWorkflow(c Caller, in WorkflowStart) (WorkflowRun, error) {
	var zero WorkflowRun
	if c.UserID == "" || c.Username == "" {
		return zero, ErrForbidden
	}
	if strings.TrimSpace(in.Input) == "" || len(in.Input) > 64000 || len(in.RequestID) > 256 {
		return zero, errors.New("input is required (up to 64000 bytes)")
	}
	if in.WorkspacePath == "" {
		return zero, errors.New("choose an existing workspace directory")
	}
	path, e := normalizeWorkspace(in.WorkspacePath)
	if e != nil {
		return zero, e
	}
	in.WorkspacePath = path
	data, e := normalizeWorkspace(s.Dir)
	if e != nil {
		return zero, e
	}
	if containsPath(data, path) || containsPath(path, data) {
		return zero, errors.New("workspace must be outside platform data")
	}
	fingerprint, _ := json.Marshal(in)
	tx, e := s.DB.Begin()
	if e != nil {
		return zero, e
	}
	defer tx.Rollback()
	if in.RequestID != "" {
		var id, hash string
		e = tx.QueryRow(`SELECT id,request_hash FROM workflow_runs WHERE owner=? AND request_key=?`, c.UserID, in.RequestID).Scan(&id, &hash)
		if e == nil {
			if hash != hashText(string(fingerprint)) {
				return zero, ErrConflict
			}
			return loadWorkflowRun(tx, id)
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return zero, e
		}
	}
	var raw string
	var w Workflow
	if e = tx.QueryRow(`SELECT definition FROM workflows WHERE id=?`, in.WorkflowID).Scan(&raw); errors.Is(e, sql.ErrNoRows) {
		return zero, ErrNotFound
	} else if e != nil {
		return zero, e
	}
	if e = json.Unmarshal([]byte(raw), &w); e != nil {
		return zero, e
	}
	if !workflowAllowed(c, w) {
		return zero, ErrForbidden
	}
	if !w.Enabled {
		return zero, errors.New("workflow is disabled")
	}
	connectors := map[string]Connector{}
	for _, n := range w.Nodes {
		if n.Kind == "connector" {
			v, err := loadConnector(tx, n.ConnectorID)
			if err != nil {
				return zero, err
			}
			if !connectorAllowed(c, v) {
				return zero, ErrForbidden
			}
			if !v.Enabled {
				return zero, errors.New("connector is disabled")
			}
			if err = connectorWorkspace(v, path); err != nil {
				return zero, err
			}
			if err = validateConnectorNode(v, n, w); err != nil {
				return zero, err
			}
			connectors[v.ID] = v
		}
		if n.Kind == "agent" {
			var cfg string
			var a Agent
			if e = tx.QueryRow(`SELECT config FROM agents WHERE id=?`, n.AgentID).Scan(&cfg); e != nil {
				return zero, e
			}
			if e = json.Unmarshal([]byte(cfg), &a); e != nil {
				return zero, e
			}
			if !allowed(c, a.ID) {
				return zero, ErrForbidden
			}
			if !a.Enabled {
				return zero, fmt.Errorf("Agent at node %s is disabled", n.ID)
			}
		}
	}
	entry := w.Entry
	if in.StartNode != "" && in.StartNode != entry {
		if !slices.Contains(w.StartNodes, in.StartNode) {
			return zero, ErrForbidden
		}
		entry = in.StartNode
	}
	// A stopped/failed Run is still resumable; its workspace must remain exclusive.
	rows, e := tx.Query(`SELECT workspace FROM workflow_runs WHERE status != 'completed'`)
	if e != nil {
		return zero, e
	}
	for rows.Next() {
		var occupied string
		if e = rows.Scan(&occupied); e != nil {
			rows.Close()
			return zero, e
		}
		if containsPath(occupied, path) || containsPath(path, occupied) {
			rows.Close()
			return zero, fmt.Errorf("%w: workspace overlaps an unfinished workflow; resume that run or use a separate checkout", ErrConflict)
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return zero, e
	}
	id, stamp := newID(), now()
	var key any
	if in.RequestID != "" {
		key = in.RequestID
	}
	_, e = tx.Exec(`INSERT INTO workflow_runs(id,workflow_id,owner,username,definition,input,workspace,status,seq,created,updated,request_key,request_hash) VALUES(?,?,?,?,?,?,?,'running',1,?,?,?,?)`, id, w.ID, c.UserID, c.Username, raw, in.Input, path, stamp, stamp, key, hashText(string(fingerprint)))
	if e != nil {
		return zero, e
	}
	for key, v := range connectors {
		raw, _ := json.Marshal(v)
		if _, e = tx.Exec(`INSERT INTO workflow_run_connectors(run_id,connector_id,definition) VALUES(?,?,?)`, id, key, string(raw)); e != nil {
			return zero, e
		}
	}
	r := WorkflowRun{ID: id, Definition: w, Seq: 1}
	if e = insertWorkflowStep(tx, r, entry); e != nil {
		return zero, e
	}
	out, e := loadWorkflowRun(tx, id)
	if e != nil {
		return zero, e
	}
	return out, tx.Commit()
}
func insertWorkflowStep(tx *sql.Tx, r WorkflowRun, nodeID string) error {
	status, runStatus := "pending", "running"
	switch r.Definition.node(nodeID).Kind {
	case "approval":
		status, runStatus = "waiting", "waiting"
	case "end":
		status, runStatus = "completed", "completed"
	}
	stamp := now()
	_, e := tx.Exec(`INSERT INTO workflow_steps(run_id,seq,node_id,status,created,updated) VALUES(?,?,?,?,?,?)`, r.ID, r.Seq, nodeID, status, stamp, stamp)
	if e != nil {
		return e
	}
	_, e = tx.Exec(`UPDATE workflow_runs SET status=?,seq=?,error='',updated=? WHERE id=?`, runStatus, r.Seq, stamp, r.ID)
	return e
}
func (s *Store) CompleteWorkflowNode(c Caller, id string, seq int, result NodeResult) error {
	return s.completeWorkflowNode(c, id, seq, "", result)
}
func (s *Store) completeWorkflowNode(c Caller, id string, seq int, token string, result NodeResult) error {

	raw, e := json.Marshal(result)
	if e != nil {
		return e
	}
	if len(raw) > 64000 {
		return errors.New("node result exceeds 64000 bytes")
	}
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	r, e := loadWorkflowRun(tx, id)
	if e != nil {
		return e
	}
	if token == "" && !runAllowed(c, r) {
		return ErrForbidden
	}
	if seq != r.Seq || (r.Status != "running" && r.Status != "waiting") {
		return ErrConflict
	}
	step := r.Steps[len(r.Steps)-1]
	node := r.Definition.node(step.NodeID)
	if token != "" {
		var hash string
		if e = tx.QueryRow(`SELECT token_hash FROM workflow_steps WHERE run_id=? AND seq=?`, id, seq).Scan(&hash); e != nil {
			return e
		}
		if hash == "" || hash != hashText(token) || node.Kind != "agent" {
			return ErrForbidden
		}
	} else if node.Kind != "approval" {
		return errors.New("Agent nodes require their scoped completion tool")
	}
	if r.Definition.target(step.NodeID, result.Route) == "" {
		return errors.New("route is not an outgoing edge of this node")
	}
	if step.Result != nil {
		previous, _ := json.Marshal(step.Result)
		if string(previous) == string(raw) {
			return nil
		}
		return ErrConflict
	}
	if step.Status != "running" && step.Status != "waiting" {
		return ErrConflict
	}
	if step.ConversationID != "" {
		var n int
		e = tx.QueryRow(`SELECT count(*) FROM messages WHERE conversation_id=? AND role='user' AND status IN ('queued','steering')`, step.ConversationID).Scan(&n)
		if e != nil {
			return e
		}
		if n > 0 {
			return fmt.Errorf("pending user input must be handled before handoff: %w", ErrConflict)
		}
	}
	_, e = tx.Exec(`UPDATE workflow_steps SET result=?,updated=? WHERE run_id=? AND seq=?`, string(raw), now(), id, seq)
	if e != nil {
		return e
	}
	return tx.Commit()
}

// Advances only after the old native execution is quiet. Closing the conversation
// in the same transaction prevents late input from racing the next node.
func (s *Store) advanceWorkflow(id string, seq int) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	r, e := loadWorkflowRun(tx, id)
	if e != nil {
		return e
	}
	if r.Seq != seq || (r.Status != "running" && r.Status != "waiting") {
		return ErrConflict
	}
	step := r.Steps[len(r.Steps)-1]
	if step.Result == nil {
		return ErrConflict
	}
	if step.ConversationID != "" {
		var status string
		if e = tx.QueryRow(`SELECT status FROM conversations WHERE id=?`, step.ConversationID).Scan(&status); e != nil {
			return e
		}
		if status != "idle" {
			return ErrConflict
		}
		var n int
		if e = tx.QueryRow(`SELECT count(*) FROM messages WHERE conversation_id=? AND role='user' AND status IN ('queued','running','steering')`, step.ConversationID).Scan(&n); e != nil {
			return e
		}
		if n != 0 {
			return ErrConflict
		}
		if _, e = tx.Exec(`UPDATE conversations SET status='closed',updated=? WHERE id=?`, now(), step.ConversationID); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(`UPDATE workflow_steps SET status='completed',updated=? WHERE run_id=? AND seq=?`, now(), id, seq); e != nil {
		return e
	}
	if seq >= r.Definition.MaxSteps {
		_, e = tx.Exec(`UPDATE workflow_runs SET status='failed',error='maximum node executions reached; inspect loop before starting another run',updated=? WHERE id=?`, now(), id)
	} else {
		r.Seq++
		e = insertWorkflowStep(tx, r, r.Definition.target(step.NodeID, step.Result.Route))
	}
	if e != nil {
		return e
	}
	return tx.Commit()
}

// Used by all conversation input paths. The accepted handoff seals user input
// before the execution is quiet; it does not terminate the native Agent.
func workflowInputAllowed(tx *sql.Tx, conversationID string) error {
	var n int
	e := tx.QueryRow(`SELECT count(*) FROM workflow_steps s JOIN workflow_runs r ON r.id=s.run_id WHERE s.conversation_id=? AND (s.seq!=r.seq OR s.result!='' OR r.status NOT IN ('running','waiting'))`, conversationID).Scan(&n)
	if e != nil {
		return e
	}
	if n != 0 {
		return fmt.Errorf("workflow node is sealed; continue from its run page: %w", ErrConflict)
	}
	return nil
}
