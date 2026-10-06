package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

const workflowTokenEnv = "WORKFLOW_NODE_TOKEN"
const workflowToolServer = "workflow_node"

type WorkflowEngine struct {
	store         *Store
	scheduler     *Scheduler
	base          string
	mu            sync.Mutex
	connectorJobs map[string]workflowConnectorJob
	connectorWG   sync.WaitGroup
	hookJobs      map[string]bool
	hookLast      time.Time
}

func NewWorkflowEngine(s *Store, scheduler *Scheduler, base string) *WorkflowEngine {
	return &WorkflowEngine{store: s, scheduler: scheduler, base: strings.TrimRight(base, "/"), connectorJobs: map[string]workflowConnectorJob{}, hookJobs: map[string]bool{}}
}
func (e *WorkflowEngine) Run(ctx context.Context) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		e.mu.Lock()
		for _, job := range e.connectorJobs {
			job.cancel()
		}
		e.mu.Unlock()
		e.connectorWG.Wait()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.Tick(ctx)
		}
	}
}
func (e *WorkflowEngine) Tick(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if time.Since(e.hookLast) > time.Second {
		e.hookLast = time.Now()
		if err := e.collectWorkflowHooks(); err != nil {
			log.Printf("collect workflow hooks: %v", err)
		}
		if err := e.dispatchWorkflowHooks(ctx); err != nil {
			log.Printf("dispatch workflow hooks: %v", err)
		}
	}
	runs, err := e.activeRuns()
	if err != nil {
		log.Printf("workflow poll: %v", err)
		return
	}
	for _, r := range runs {
		if ctx.Err() != nil {
			return
		}
		if r.Status != "running" && r.Status != "waiting" && r.Status != "stopping" {
			continue
		}
		if err = e.tickRun(ctx, r); err != nil && !errors.Is(err, ErrConflict) {
			log.Printf("workflow %s: %v", r.ID, err)
			e.fail(r, err)
		}
	}
}
func (e *WorkflowEngine) activeRuns() ([]WorkflowRun, error) {
	rows, err := e.store.DB.Query(`SELECT id FROM workflow_runs WHERE status IN ('running','waiting','stopping') ORDER BY created`)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	runs := []WorkflowRun{}
	for _, id := range ids {
		r, err := e.store.WorkflowRun(Caller{Admin: true}, id)
		if err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, nil
}
func (e *WorkflowEngine) fail(r WorkflowRun, err error) {
	e.stopConnector(r.ID)
	if len(r.Steps) > 0 && e.scheduler != nil {
		id := r.Steps[len(r.Steps)-1].ConversationID
		if id != "" && !e.quiet(id) {
			if stopErr := e.scheduler.Stop(id, false); stopErr != nil {
				log.Printf("stop failed workflow Agent: %v", stopErr)
			}
		}
	}
	// Record the failure on this exact execution, retaining its accepted result.
	tx, dbErr := e.store.DB.Begin()
	if dbErr != nil {
		log.Printf("save workflow failure: %v", dbErr)
		return
	}
	defer tx.Rollback()
	changed, dbErr := tx.Exec(`UPDATE workflow_runs SET status='failed',error=?,updated=? WHERE id=? AND seq=? AND status IN ('running','waiting','stopping')`, err.Error(), now(), r.ID, r.Seq)
	if dbErr == nil {
		var n int64
		n, dbErr = changed.RowsAffected()
		if n == 1 && dbErr == nil {
			_, dbErr = tx.Exec(`UPDATE workflow_steps SET status='failed',error=?,updated=? WHERE run_id=? AND seq=?`, err.Error(), now(), r.ID, r.Seq)
		}
	}
	if dbErr == nil {
		dbErr = tx.Commit()
	}
	if dbErr != nil {
		log.Printf("save workflow failure: %v", dbErr)
	}
}
func (e *WorkflowEngine) quiet(id string) bool {
	if id == "" {
		return true
	}
	if e.scheduler != nil {
		e.scheduler.mu.Lock()
		defer e.scheduler.mu.Unlock()
		if _, ok := e.scheduler.active[id]; ok {
			return false
		}
	}
	c, err := e.store.Conversation(id)
	return err == nil && c.Status != "running" && c.Status != "stopping"
}
func (e *WorkflowEngine) tickRun(ctx context.Context, r WorkflowRun) error {
	step := r.Steps[len(r.Steps)-1]
	if r.Status == "stopping" {
		if !e.stopConnector(r.ID) {
			return nil
		}
		if step.ConversationID != "" {
			if e.scheduler != nil {
				conv, err := e.store.Conversation(step.ConversationID)
				if err != nil {
					return err
				}
				if err := func() error {
					if conv.Status == "closed" {
						return nil
					}
					return e.scheduler.Stop(step.ConversationID, false)
				}(); err != nil {
					return err
				}
			} else if !e.quiet(step.ConversationID) {
				return ErrConflict
			}
		}
		if !e.quiet(step.ConversationID) {
			return nil
		}
		tx, err := e.store.DB.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		_, err = tx.Exec(`UPDATE workflow_runs SET status='stopped',updated=? WHERE id=? AND status='stopping' AND seq=?; UPDATE workflow_steps SET status='stopped',updated=? WHERE run_id=? AND seq=?`, now(), r.ID, r.Seq, now(), r.ID, r.Seq)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	caller, err := e.store.workflowCaller(r)
	if err != nil {
		return fmt.Errorf("run owner account is unavailable: %w", err)
	}
	if _, err = e.store.Workflow(caller, r.WorkflowID); err != nil {
		return fmt.Errorf("workflow authorization changed: %w", err)
	}
	node := r.Definition.node(step.NodeID)
	if node.Kind == "connector" {
		if _, err = e.connectorAccess(caller, r, node); err != nil {
			return err
		}
		if step.Status == "pending" {
			return e.startConnector(ctx, caller, r, step, node)
		}
		if step.Result == nil && e.connectorQuiet(r.ID) {
			return errors.New("connector stopped without a confirmed result; inspect its receipt and use explicit recovery")
		}
	}
	if step.Status == "pending" {
		node := r.Definition.node(step.NodeID)
		if node.Kind != "agent" {
			return errors.New("unsupported executable node")
		}
		return e.startAgent(caller, r, step, node)
	}
	if step.ConversationID != "" {
		conv, err := e.store.Conversation(step.ConversationID)
		if err != nil {
			return err
		}
		if !allowed(caller, conv.AgentID) {
			return ErrForbidden
		}
		if conv.Status == "failed" || conv.Status == "stopped" || conv.Status == "closed" {
			return fmt.Errorf("node conversation %s; inspect and resume or return from the run page: %s", conv.Status, conv.Error)
		}
		if step.Result == nil {
			state := "running"
			if conv.Status == "idle" {
				state = "waiting"
			}
			if state != r.Status {
				tx, beginErr := e.store.DB.Begin()
				if beginErr != nil {
					return beginErr
				}
				defer tx.Rollback()
				_, err = tx.Exec(`UPDATE workflow_runs SET status=?,updated=? WHERE id=? AND seq=? AND status IN ('running','waiting'); UPDATE workflow_steps SET status=?,updated=? WHERE run_id=? AND seq=?`, state, now(), r.ID, r.Seq, state, now(), r.ID, r.Seq)
				if err == nil {
					err = tx.Commit()
				}
			}
			return err
		}
	}
	if step.ConversationID == "" && step.Result == nil && r.Status == "running" && r.Definition.node(step.NodeID).Kind == "approval" {
		_, err = e.store.DB.Exec(`UPDATE workflow_runs SET status='waiting',updated=? WHERE id=? AND seq=? AND status='running'`, now(), r.ID, r.Seq)
		return err
	}
	if step.Result != nil {
		return e.store.advanceWorkflow(r.ID, r.Seq)
	}
	return nil
}
func (e *WorkflowEngine) startAgent(c Caller, r WorkflowRun, step WorkflowStep, node WorkflowNode) error {
	// Input is generated entirely from the frozen graph and stored results. The
	// capability is installed privately in the Agent snapshot, never in text.
	previous := []map[string]any{}
	for _, old := range r.Steps {
		if old.Result != nil || old.Error != "" {
			previous = append(previous, map[string]any{"node": old.NodeID, "seq": old.Seq, "result": old.Result, "feedback": old.Error, "connector_receipt": old.Receipt})
		}
	}
	contextData, _ := json.MarshalIndent(map[string]any{"run_id": r.ID, "task": r.Input, "node": node.Name, "instructions": node.Prompt, "routes": r.Definition.routes(node.ID), "outgoing_edges": workflowOutgoing(r.Definition, node.ID), "previous_results": previous}, "", "  ")
	message := "你正在执行智能体编排中的一个节点。根据任务上下文与用户继续工作，意图和分支由你根据节点 SOP 判断，不使用额外意图模型。需要澄清时正常提问，等待用户回复，不交接。outgoing_edges 中 mode=handoff 表示你可以自主选择的目标；使用 registered_workflow_node 的 handoff 工具提交 target、真实 summary、可选 inputs 和 artifacts。mode=automatic 表示固定完成线：确认整个节点任务完成后调用 complete_node，省略 route，由平台沿固定线推进。一次执行只能交接或完成一次。工具接受后结束本轮，不再修改工作区。前序命令回执有 log 字段时，可用 read_command_output 按 seq 和 offset 分页读取脱敏日志；eof 表示已读完保存部分，truncated 表示仍有未保存输出。没有 log 的旧执行不能恢复缺失日志。自然语言回复不会推进流程。不要自行创建下一节点或重启整个流程。\n\n" + string(contextData)
	token := newID() + newID()
	_, err := e.store.submit(c, Input{AgentID: node.AgentID, UserID: r.Owner, Message: r.Input, WorkspacePath: r.WorkspacePath, RequestID: "workflow:" + r.ID + ":" + fmt.Sprint(step.Seq)}, func(tx *sql.Tx, a *Agent, conversationID string) error {
		var state, nodeState string
		var seq int
		if err := tx.QueryRow(`SELECT r.status,r.seq,s.status FROM workflow_runs r JOIN workflow_steps s ON s.run_id=r.id AND s.seq=r.seq WHERE r.id=?`, r.ID).Scan(&state, &seq, &nodeState); err != nil {
			return err
		}
		if seq != step.Seq || (state != "running" && state != "waiting") || nodeState != "pending" {
			return ErrConflict
		}
		if _, exists := a.ResolvedTools[workflowToolServer]; exists {
			return errors.New("workflow_node is a reserved tool server name")
		}
		if a.ResolvedTools == nil {
			a.ResolvedTools = map[string]ResolvedToolServer{}
		}
		a.ResolvedTools[workflowToolServer] = ResolvedToolServer{Connection: MCPConnection{URL: e.base + "/api/workflow-node/mcp", BearerTokenEnvVar: workflowTokenEnv}, Tools: []string{"complete_node", "handoff", "read_command_output"}, Approvals: map[string]string{"complete_node": "auto", "handoff": "auto", "read_command_output": "auto"}}
		if a.Env == nil {
			a.Env = map[string]*string{}
		}
		a.Env[workflowTokenEnv] = &token
		a.Instructions += "\n\n" + message
		_, err := tx.Exec(`UPDATE workflow_steps SET status='running',conversation_id=?,token_hash=?,updated=? WHERE run_id=? AND seq=?`, conversationID, hashText(token), now(), r.ID, step.Seq)
		return err
	})
	return err
}
func (e *WorkflowEngine) Stop(c Caller, id string, seq int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	tx, err := e.store.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := loadWorkflowRun(tx, id)
	if err != nil {
		return err
	}
	if !runAllowed(c, r) {
		return ErrForbidden
	}
	if r.Seq != seq || r.Status == "completed" {
		return ErrConflict
	}
	if r.Status == "stopped" || r.Status == "stopping" {
		return nil
	}
	_, err = tx.Exec(`UPDATE workflow_runs SET status='stopping',updated=? WHERE id=?`, now(), id)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Return deliberately requires the current process to be stopped first. The UI
// exposes this as two explicit operations, never a optimistic cancellation.
func (e *WorkflowEngine) Return(c Caller, id string, seq int, target, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, err := e.store.WorkflowRun(c, id)
	if err != nil {
		return err
	}
	if r.Seq != seq || r.Status != "stopped" || strings.TrimSpace(reason) == "" {
		return ErrConflict
	}
	if !e.connectorQuiet(r.ID) || !e.quiet(r.Steps[len(r.Steps)-1].ConversationID) {
		return ErrConflict
	}
	if r.Definition.node(target).ID == "" || r.Seq >= r.Definition.MaxSteps {
		return errors.New("invalid return target or execution limit reached")
	}
	tx, err := e.store.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, err := loadWorkflowRun(tx, id)
	if err != nil {
		return err
	}
	if current.Seq != seq || current.Status != "stopped" {
		return ErrConflict
	}
	old := current.Steps[len(current.Steps)-1]
	if old.ConversationID != "" {
		if _, err = tx.Exec(`UPDATE conversations SET status='closed',updated=? WHERE id=?`, now(), old.ConversationID); err != nil {
			return err
		}
	}
	// A human redirection is recorded as such, never as a successful node result.
	if _, err = tx.Exec(`UPDATE workflow_steps SET status='cancelled',error=CASE WHEN error='' THEN ? ELSE error || char(10) || ? END,updated=? WHERE run_id=? AND seq=?`, "用户回退到 "+target+": "+reason, "用户回退到 "+target+": "+reason, now(), id, seq); err != nil {
		return err
	}
	current.Seq++
	if err = insertWorkflowStep(tx, current, target); err != nil {
		return err
	}
	return tx.Commit()
}
func (e *WorkflowEngine) Resume(c Caller, id string, seq int, message string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, err := e.store.WorkflowRun(c, id)
	if err != nil {
		return err
	}
	if r.Seq != seq || (r.Status != "stopped" && r.Status != "failed") {
		return ErrConflict
	}
	owner, err := e.store.workflowCaller(r)
	if err != nil {
		return err
	}
	step := r.Steps[len(r.Steps)-1]
	if r.Definition.node(step.NodeID).Kind == "connector" {
		if err = e.connectorResumeAllowed(r, step, message); err != nil {
			return err
		}
	}
	if !e.quiet(step.ConversationID) {
		return ErrConflict
	}
	if step.ConversationID != "" {
		if step.Result != nil {
			return errors.New("handoff was accepted before interruption; stop and return to a chosen node after inspecting its effects")
		}
		if strings.TrimSpace(message) == "" {
			return errors.New("describe how the existing Agent should continue; uncertain input is not replayed")
		}
		conv, err := e.store.Conversation(step.ConversationID)
		if err != nil {
			return err
		}
		if conv.Status == "closed" {
			return errors.New("conversation is closed; stop and return to a node")
		}
	}
	if _, err = e.store.Workflow(owner, r.WorkflowID); err != nil {
		return err
	}
	nodeStatus := "pending"
	runStatus := "running"
	if step.ConversationID != "" {
		nodeStatus = "running"
	} else if r.Definition.node(step.NodeID).Kind == "approval" {
		nodeStatus, runStatus = "waiting", "waiting"
	}
	// Change run state first so Submit can accept a new user input. Failure leaves
	// an inspectable failed run rather than silently rerunning an old message.
	tx, err := e.store.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`UPDATE workflow_runs SET status=?,error='',updated=? WHERE id=? AND seq=?; UPDATE workflow_steps SET status=?,updated=? WHERE run_id=? AND seq=?`, runStatus, now(), id, seq, nodeStatus, now(), id, seq)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if step.ConversationID != "" {
		if _, err = e.store.Submit(owner, Input{UserID: r.Owner, ConversationID: step.ConversationID, Message: message}); err != nil {
			e.fail(r, err)
			return err
		}
		if e.scheduler != nil {
			err = e.scheduler.Continue(step.ConversationID)
		} else {
			err = e.store.Continue(step.ConversationID)
		}
		if err != nil {
			e.fail(r, err)
			return err
		}
	}
	return nil
}

func workflowOutgoing(w Workflow, id string) []WorkflowEdge {
	out := []WorkflowEdge{}
	for _, edge := range w.Edges {
		if edge.Source == id {
			edge.Mode = w.edgeMode(edge)
			out = append(out, edge)
		}
	}
	return out
}
