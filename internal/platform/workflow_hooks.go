package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Hooks map durable execution facts to an existing administrator-owned action.
// Notifications do not gate or replay node execution. Blocking gates are separate.
type WorkflowHook struct {
	Event       string         `json:"event"`
	Node        string         `json:"node,omitempty"`
	ConnectorID string         `json:"connector_id"`
	Input       ConnectorInput `json:"input"`
	Body        string         `json:"body"`
}
type WorkflowHookDelivery struct {
	ID      string            `json:"id"`
	Seq     int               `json:"seq"`
	Event   string            `json:"event"`
	Node    string            `json:"node"`
	Status  string            `json:"status"`
	Error   string            `json:"error,omitempty"`
	Receipt *ConnectorReceipt `json:"receipt,omitempty"`
}

var errApprovalResolved = errors.New("approval already resolved before notification write")

var hookField = regexp.MustCompile(`\{\{\s*([a-z_]+)\s*\}\}`)
var hookFields = map[string]bool{"event": true, "run_id": true, "node": true, "node_name": true, "seq": true, "text": true, "summary": true, "target": true, "artifacts": true, "conversation_url": true, "run_url": true, "approval_status": true}

// A configured Agent reply channel also carries execution approval lifecycle.
// Explicit rules replace these defaults. The frozen graph is never rewritten.
func workflowNotificationHooks(w Workflow) []WorkflowHook {
	hooks := append([]WorkflowHook{}, w.Hooks...)
	type destination struct {
		event, node, connector string
		input                  ConnectorInput
	}
	seen := map[destination]bool{}
	for _, h := range hooks {
		seen[destination{h.Event, h.Node, h.ConnectorID, h.Input}] = true
	}
	for _, h := range w.Hooks {
		if h.Event != "agent.reply.completed" {
			continue
		}
		for _, event := range []string{"approval.requested", "approval.resolved"} {
			key := destination{event, h.Node, h.ConnectorID, h.Input}
			if seen[key] {
				continue
			}
			copy := h
			copy.Event = event
			if event == "approval.requested" {
				copy.Body = "**{{node_name}} 等待执行审批**\n\n请在会话中查看并处理当前申请。\n\n[打开待审批会话]({{conversation_url}})"
			} else {
				copy.Body = "**{{node_name}} 的执行审批已处理**\n\n结果：{{approval_status}}。\n\n[查看会话]({{conversation_url}})"
			}
			hooks = append(hooks, copy)
			seen[key] = true
		}
	}
	return hooks
}

func validateWorkflowHooks(w Workflow) error {
	if len(w.Hooks) > 32 {
		return errors.New("at most 32 hooks are allowed")
	}
	for _, h := range w.Hooks {
		switch h.Event {
		case "node.started", "agent.reply.completed", "handoff.after", "run.completed", "approval.requested", "approval.resolved":
		default:
			return errors.New("unsupported Hook event")
		}
		if h.ConnectorID == "" || strings.TrimSpace(h.Body) == "" || len(h.Body) > 16000 {
			return errors.New("Hook action and body template required (up to 16000 bytes)")
		}
		if h.Node != "" && w.node(h.Node).ID == "" {
			return errors.New("Hook node does not exist")
		}
		for _, m := range hookField.FindAllStringSubmatch(h.Body, -1) {
			if !hookFields[m[1]] {
				return fmt.Errorf("unknown Hook field: %s", m[1])
			}
		}
		remaining := hookField.ReplaceAllString(h.Body, "")
		if strings.Contains(remaining, "{{") || strings.Contains(remaining, "}}") {
			return errors.New("Hook templates support only named fields, not expressions")
		}
	}
	return nil
}
func validateHookAction(v Connector, h WorkflowHook, w Workflow) error {
	// The first supported notification adapter reuses the Issue comment Connector.
	// Other adapters must define their own validation and reconciliation semantics.
	if v.Kind != "github.issue_comment" {
		return errors.New("this version supports github.issue_comment notification actions")
	}
	if h.Input.BodyFile != "" {
		return errors.New("Hook body comes from its template, not a workspace file")
	}
	return validateConnectorInput(v, WorkflowNode{ConnectorInput: h.Input}, w)
}
func (s *Store) initWorkflowHooks() error {
	_, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS workflow_hook_deliveries(
 id TEXT PRIMARY KEY,run_id TEXT NOT NULL REFERENCES workflow_runs(id),seq INTEGER NOT NULL,hook_index INTEGER NOT NULL,
 event TEXT NOT NULL,node TEXT NOT NULL,fields TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'pending',
 request TEXT NOT NULL DEFAULT '',receipt TEXT NOT NULL DEFAULT '',error TEXT NOT NULL DEFAULT '',updated TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS workflow_hook_pending ON workflow_hook_deliveries(status);
 UPDATE workflow_hook_deliveries SET status='unknown',error='Notification interrupted; reconcile external result before retrying' WHERE status='sending';`)
	return err
}
func renderHookBody(body string, fields map[string]string) string {
	return hookField.ReplaceAllStringFunc(body, func(m string) string { return fields[hookField.FindStringSubmatch(m)[1]] })
}

// Events are a projection of durable facts, not text classification. Rebuilding
// this projection after restart cannot duplicate deliveries due to stable keys.
func (e *WorkflowEngine) collectWorkflowHooks() error {
	rows, err := e.store.DB.Query(`SELECT id FROM workflow_runs WHERE json_array_length(definition,'$.hooks')>0`)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		r, err := e.store.WorkflowRun(Caller{Admin: true}, id)
		if err != nil {
			return err
		}
		hooks := workflowNotificationHooks(r.Definition)
		order := make([]int, len(hooks))
		for i := range order {
			order[i] = i
		}
		rank := map[string]int{"node.started": 0, "approval.requested": 1, "approval.resolved": 2, "agent.reply.completed": 3, "handoff.after": 4, "run.completed": 5}
		sort.SliceStable(order, func(i, j int) bool {
			return rank[hooks[order[i]].Event] < rank[hooks[order[j]].Event]
		})
		for _, step := range r.Steps {
			for _, index := range order {
				h := hooks[index]
				if h.Node != "" && h.Node != step.NodeID {
					continue
				}
				// Defaults follow the existing channel, but a configured rule for
				// this destination/node takes precedence. Earlier defaults also
				// cover overlapping all-node and node-specific reply channels.
				if index >= len(r.Definition.Hooks) {
					overridden := false
					for _, other := range hooks[:index] {
						if other.Event == h.Event && other.ConnectorID == h.ConnectorID && other.Input == h.Input && (other.Node == "" || other.Node == step.NodeID) {
							overridden = true
							break
						}
					}
					if overridden {
						continue
					}
				}
				fields := map[string]string{"event": h.Event, "run_id": r.ID, "node": step.NodeID, "node_name": r.Definition.node(step.NodeID).Name, "seq": strconv.Itoa(step.Seq), "run_url": e.base + "/workflow-runs/" + r.ID}
				if step.ConversationID != "" {
					fields["conversation_url"] = e.base + "/conversations/" + step.ConversationID
				}
				if step.Result != nil {
					fields["summary"] = step.Result.Summary
					fields["target"] = r.Definition.target(step.NodeID, step.Result.Route)
					fields["artifacts"] = strings.Join(step.Result.Artifacts, "\n")
				}
				emit := func(suffix string) error {
					raw, _ := json.Marshal(fields)
					key := hashText(fmt.Sprintf("%s:%d:%d:%s", id, step.Seq, index, suffix))
					_, err := e.store.DB.Exec(`INSERT OR IGNORE INTO workflow_hook_deliveries(id,run_id,seq,hook_index,event,node,fields,updated) VALUES(?,?,?,?,?,?,?,?)`, key, id, step.Seq, index, h.Event, step.NodeID, string(raw), now())
					return err
				}
				switch h.Event {
				case "approval.requested", "approval.resolved":
					if step.ConversationID == "" {
						continue
					}
					// Do not load or publish native commands, reasons or credential data.
					rows, qerr := e.store.DB.Query(`SELECT id,decision FROM tool_approvals WHERE conversation_id=? ORDER BY created,id`, step.ConversationID)
					if qerr != nil {
						return qerr
					}
					type decision struct{ id, value string }
					decisions := []decision{}
					for rows.Next() {
						var d decision
						if qerr = rows.Scan(&d.id, &d.value); qerr != nil {
							break
						}
						decisions = append(decisions, d)
					}
					if qerr == nil {
						qerr = rows.Err()
					}
					rows.Close()
					if qerr != nil {
						return qerr
					}
					for _, d := range decisions {
						if h.Event == "approval.requested" && d.value != "" || h.Event == "approval.resolved" && d.value == "" {
							continue
						}
						if h.Event == "approval.resolved" && index >= len(r.Definition.Hooks) {
							// No historical approval backlog on upgrade: resolve only an
							// approval whose waiting notification was actually collected.
							var exists bool
							if qerr = e.store.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM workflow_hook_deliveries WHERE run_id=? AND seq=? AND event='approval.requested' AND json_extract(fields,'$._approval_id')=?)`, r.ID, step.Seq, d.id).Scan(&exists); qerr != nil {
								return qerr
							}
							if !exists {
								continue
							}
						}
						fields["_approval_id"] = d.id
						fields["approval_status"] = map[string]string{"": "待处理", "accept": "已批准", "decline": "已拒绝", "cancel": "已失效"}[d.value]
						if err = emit(d.id); err != nil {
							break
						}
					}
				case "node.started":
					if step.ConversationID != "" {
						err = emit("")
					}
				case "handoff.after":
					if step.Result != nil && step.Status == "completed" && step.Seq < r.Seq {
						err = emit("")
					}
				case "run.completed":
					if r.Status == "completed" && step.Seq == r.Seq {
						err = emit("")
					}
				case "agent.reply.completed":
					if step.ConversationID == "" {
						continue
					}
					replies, qerr := e.store.DB.Query(`SELECT m.id,m.content FROM messages m JOIN messages parent ON parent.id=m.parent_id WHERE m.conversation_id=? AND m.role='agent' AND m.kind='reply' AND parent.status='completed' ORDER BY m.id`, step.ConversationID)
					if qerr != nil {
						return qerr
					}
					type reply struct {
						id   int64
						text string
					}
					all := []reply{}
					for replies.Next() {
						var v reply
						if qerr = replies.Scan(&v.id, &v.text); qerr != nil {
							break
						}
						all = append(all, v)
					}
					if qerr == nil {
						qerr = replies.Err()
					}
					replies.Close()
					if qerr != nil {
						return qerr
					}
					for _, v := range all {
						fields["text"] = v.text
						if err = emit(strconv.FormatInt(v.id, 10)); err != nil {
							break
						}
					}
				}
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func (e *WorkflowEngine) dispatchWorkflowHooks(ctx context.Context) error {
	if len(e.hookJobs) >= 4 {
		return nil
	}
	rows, err := e.store.DB.Query(`SELECT id,run_id,seq,hook_index,fields,request FROM workflow_hook_deliveries WHERE status='pending' ORDER BY rowid LIMIT 8`)
	if err != nil {
		return err
	}
	type pending struct {
		id, run, fields, request string
		seq, index               int
	}
	list := []pending{}
	for rows.Next() {
		var p pending
		if err = rows.Scan(&p.id, &p.run, &p.seq, &p.index, &p.fields, &p.request); err != nil {
			rows.Close()
			return err
		}
		list = append(list, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, p := range list {
		if e.hookJobs[p.run+":"+p.id] || len(e.hookJobs) >= 4 {
			continue
		}
		// Keep per-run delivery order: a later reply cannot overtake its start notice.
		busy := false
		for key := range e.hookJobs {
			if strings.HasPrefix(key, p.run+":") {
				busy = true
			}
		}
		if busy {
			continue
		}
		r, err := e.store.WorkflowRun(Caller{Admin: true}, p.run)
		if err != nil {
			return err
		}
		hooks := workflowNotificationHooks(r.Definition)
		if p.index < 0 || p.index >= len(hooks) {
			return errors.New("notification rule is unavailable")
		}
		h := hooks[p.index]
		if h.Event == "approval.requested" && p.request == "" {
			fields := map[string]string{}
			if err = json.Unmarshal([]byte(p.fields), &fields); err != nil {
				return err
			}
			var decision string
			if err = e.store.DB.QueryRow(`SELECT decision FROM tool_approvals WHERE id=?`, fields["_approval_id"]).Scan(&decision); err != nil {
				return err
			}
			if decision != "" {
				if _, err = e.store.DB.Exec(`UPDATE workflow_hook_deliveries SET status='skipped',error='',updated=? WHERE id=? AND status='pending' AND request=''`, now(), p.id); err != nil {
					return err
				}
				continue
			}
		}
		owner, err := e.store.workflowCaller(r)
		if err == nil {
			_, err = e.store.Workflow(owner, r.WorkflowID)
		}
		var v Connector
		if err == nil {
			v, err = e.connectorAccess(owner, r, WorkflowNode{ConnectorID: h.ConnectorID})
		}
		var request connectorRequest
		recoverOnly := p.request != ""
		if err == nil && recoverOnly {
			err = json.Unmarshal([]byte(p.request), &request)
		}
		if err == nil && !recoverOnly {
			fields := map[string]string{}
			err = json.Unmarshal([]byte(p.fields), &fields)
			if err == nil {
				// Adapter parameter preparation reuses the exact same validation/credential path.
				nodeID := "hook_action"
				for r.Definition.node(nodeID).ID != "" {
					nodeID += "_"
				}
				copyRun := r
				copyRun.Definition.Nodes = append(append([]WorkflowNode{}, r.Definition.Nodes...), WorkflowNode{ID: nodeID, ConnectorInput: h.Input})
				request, err = prepareConnectorRequest(v, copyRun, WorkflowStep{NodeID: nodeID, Seq: p.seq})
				if err == nil {
					request.Marker = "<!-- agent-platform-hook:" + p.id + " -->"
					request.Payload["body"] = renderHookBody(h.Body, fields) + "\n\n" + request.Marker
				}
			}
		}
		if err != nil {
			if _, saveErr := e.store.DB.Exec(`UPDATE workflow_hook_deliveries SET status='failed',error=?,updated=? WHERE id=?`, err.Error(), now(), p.id); saveErr != nil {
				return saveErr
			}
			continue
		}
		raw, _ := json.Marshal(request)
		if _, err = e.store.DB.Exec(`UPDATE workflow_hook_deliveries SET status='sending',request=?,error='',updated=? WHERE id=? AND status='pending'`, string(raw), now(), p.id); err != nil {
			return err
		}
		jobKey := p.run + ":" + p.id
		e.hookJobs[jobKey] = true
		e.connectorWG.Add(1)
		go func(id, key string, v Connector, request connectorRequest, recoverOnly, approvalWait bool, fieldsRaw string) {
			defer e.connectorWG.Done()
			callCtx, cancel := context.WithTimeout(ctx, time.Duration(v.TimeoutSeconds)*time.Second)
			defer cancel()
			var beforeWrite func() error
			if approvalWait {
				beforeWrite = func() error {
					fields := map[string]string{}
					if err := json.Unmarshal([]byte(fieldsRaw), &fields); err != nil {
						return err
					}
					var decision string
					if err := e.store.DB.QueryRow(`SELECT decision FROM tool_approvals WHERE id=?`, fields["_approval_id"]).Scan(&decision); err != nil {
						return err
					}
					if decision != "" {
						return errApprovalResolved
					}
					return nil
				}
			}
			receipt, err := executeGitHubBeforeWrite(callCtx, v, request, recoverOnly, beforeWrite)
			status, message := "succeeded", ""
			clearRequest := false
			if err != nil {
				status, message = "unknown", err.Error()
				var notSent *githubNotSentError
				if errors.As(err, &notSent) {
					status, clearRequest = "failed", true
					if errors.Is(err, errApprovalResolved) {
						status, message = "skipped", ""
					}
				}
			}
			raw, _ := json.Marshal(receipt)
			e.mu.Lock()
			defer e.mu.Unlock()
			delete(e.hookJobs, key)
			if _, saveErr := e.store.DB.Exec(`UPDATE workflow_hook_deliveries SET status=?,receipt=?,error=?,request=CASE WHEN ? THEN '' ELSE request END,updated=? WHERE id=?`, status, string(raw), message, clearRequest, now(), id); saveErr != nil {
				log.Printf("save hook delivery: %v", saveErr)
			}
		}(p.id, jobKey, v, request, recoverOnly, h.Event == "approval.requested", p.fields)
	}
	return nil
}
func (s *Store) WorkflowHookDeliveries(c Caller, id string) ([]WorkflowHookDelivery, error) {
	if _, err := s.WorkflowRun(c, id); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`SELECT id,seq,event,node,status,error,receipt FROM workflow_hook_deliveries WHERE run_id=? ORDER BY rowid`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorkflowHookDelivery{}
	for rows.Next() {
		var v WorkflowHookDelivery
		var raw string
		if err = rows.Scan(&v.ID, &v.Seq, &v.Event, &v.Node, &v.Status, &v.Error, &raw); err != nil {
			return nil, err
		}
		if raw != "" {
			if err = json.Unmarshal([]byte(raw), &v.Receipt); err != nil {
				return nil, err
			}
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (h *Server) workflowHookRoutes() {
	h.mux.HandleFunc("GET /api/workflow-runs/{id}/notifications", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		v, err := h.store.WorkflowHookDeliveries(c, r.PathValue("id"))
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, v)
	}))
	h.mux.HandleFunc("POST /api/workflow-runs/{id}/notifications/{notification}/retry", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		if _, err := h.store.WorkflowRun(c, r.PathValue("id")); err != nil {
			fail(w, err)
			return
		}
		changed, err := h.store.DB.Exec(`UPDATE workflow_hook_deliveries SET status='pending',updated=? WHERE id=? AND run_id=? AND status IN ('failed','unknown')`, now(), r.PathValue("notification"), r.PathValue("id"))
		if err != nil {
			fail(w, err)
			return
		}
		n, _ := changed.RowsAffected()
		if n != 1 {
			fail(w, ErrConflict)
			return
		}
		respond(w, 202, map[string]bool{"accepted": true})
	}))
}
