package platform

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type WorkflowWait struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

func (s *Store) waitWorkflowNode(id string, seq int, token string, in WorkflowWait) error {
	if (in.Kind != "clarification" && in.Kind != "blocked") || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 8000 {
		return errors.New("wait needs kind clarification/blocked and a concrete reason (up to 8000 bytes)")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := loadWorkflowRun(tx, id)
	if err != nil {
		return err
	}
	if r.Definition.ContextVersion < 1 || r.Seq != seq || (r.Status != "running" && r.Status != "waiting") {
		return ErrConflict
	}
	step := r.Steps[len(r.Steps)-1]
	node := r.Definition.node(step.NodeID)
	if node.AllowUserInput != nil && !*node.AllowUserInput {
		return ErrForbidden
	}
	var hash, status string
	var mid int64
	if err = tx.QueryRow(`SELECT s.token_hash,c.status FROM workflow_steps s JOIN conversations c ON c.id=s.conversation_id WHERE s.run_id=? AND s.seq=?`, id, seq).Scan(&hash, &status); err != nil {
		return err
	}
	if hash != hashText(token) || status != "running" || step.Result != nil {
		return ErrConflict
	}
	if err = tx.QueryRow(`SELECT COALESCE(MAX(id),0) FROM messages WHERE conversation_id=? AND role='user' AND status='running'`, step.ConversationID).Scan(&mid); err != nil {
		return err
	}
	var pending int
	if mid == 0 {
		return ErrConflict
	}
	if err = tx.QueryRow(`SELECT COUNT(*) FROM messages WHERE conversation_id=? AND role='user' AND status IN ('queued','steering','steered') AND id>?`, step.ConversationID, mid).Scan(&pending); err != nil {
		return err
	}
	if pending > 0 {
		return ErrConflict
	}
	_, err = tx.Exec(`INSERT INTO workflow_node_controls(run_id,seq,wait_kind,wait_reason,wait_message_id) VALUES(?,?,?,?,?) ON CONFLICT(run_id,seq) DO UPDATE SET wait_kind=excluded.wait_kind,wait_reason=excluded.wait_reason,wait_message_id=excluded.wait_message_id`, id, seq, in.Kind, strings.TrimSpace(in.Reason), mid)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Called only after a normal native turn ends. Both queue insertion and its
// persistent budget/checkpoint are committed together; no text classification.
func (s *Store) continueWorkflowNode(id string, seq int) error {
	before, err := s.WorkflowRun(Caller{Admin: true}, id)
	if err != nil {
		return err
	}
	owner, err := s.workflowCaller(before)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := loadWorkflowRun(tx, id)
	if err != nil {
		return err
	}
	if r.Seq != seq || (r.Status != "running" && r.Status != "waiting") {
		return ErrConflict
	}
	step := r.Steps[len(r.Steps)-1]
	if step.Result != nil {
		return ErrConflict
	}
	c, err := scanConv(tx.QueryRow(`SELECT `+convColumns+` FROM conversations WHERE id=?`, step.ConversationID))
	if err != nil {
		return err
	}
	if c.Status != "idle" {
		return ErrConflict
	}
	if !conversationAgentAllowed(tx, owner, c.ID, c.AgentID) {
		return ErrForbidden
	}
	var pending int
	var mid, waitID, last int64
	var count int
	var kind, reason string
	if err = tx.QueryRow(`SELECT COUNT(*) FROM messages WHERE conversation_id=? AND role='user' AND status IN ('queued','steering')`, c.ID).Scan(&pending); err != nil {
		return err
	}
	if pending > 0 {
		return ErrConflict
	}
	if err = tx.QueryRow(`SELECT COALESCE(MAX(id),0) FROM messages WHERE conversation_id=? AND role='user' AND status IN ('completed','steered')`, c.ID).Scan(&mid); err != nil {
		return err
	}
	if mid == 0 {
		return ErrConflict
	}
	err = tx.QueryRow(`SELECT wait_kind,wait_reason,wait_message_id,continuations,last_message_id FROM workflow_node_controls WHERE run_id=? AND seq=?`, id, seq).Scan(&kind, &reason, &waitID, &count, &last)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if kind != "" && mid > waitID {
		kind = ""
		reason = ""
	}
	n := r.Definition.node(step.NodeID)
	if kind == "" {
		switch {
		case n.ContinuationLimit == 0:
			kind = "disabled"
			reason = "持续推进未启用；节点未完成，等待显式输入。"
		case count >= n.ContinuationLimit:
			kind = "limit"
			reason = fmt.Sprintf("达到本节点 %d 次持续推进上限；保留现场，节点未完成。", n.ContinuationLimit)
		case n.ExecutionTimeoutSeconds > 0:
			started, parseErr := time.Parse(time.RFC3339Nano, step.Created)
			if parseErr != nil {
				return parseErr
			}
			if time.Since(started) >= time.Duration(n.ExecutionTimeoutSeconds)*time.Second {
				kind = "limit"
				reason = "达到本节点持续推进时间上限；保留现场，节点未完成。"
			}
		}
	}
	if kind != "" {
		if kind == step.WaitKind && reason == step.WaitReason && mid == waitID && r.Status == "waiting" && step.Status == "waiting" {
			return nil
		}
		_, err = tx.Exec(`INSERT INTO workflow_node_controls(run_id,seq,wait_kind,wait_reason,wait_message_id) VALUES(?,?,?,?,?) ON CONFLICT(run_id,seq) DO UPDATE SET wait_kind=excluded.wait_kind,wait_reason=excluded.wait_reason,wait_message_id=excluded.wait_message_id`, id, seq, kind, reason, mid)
		if err == nil {
			_, err = tx.Exec(`UPDATE workflow_runs SET status='waiting',updated=? WHERE id=? AND seq=?;UPDATE workflow_steps SET status='waiting',updated=? WHERE run_id=? AND seq=?`, now(), id, seq, now(), id, seq)
		}
	} else {
		if last >= mid {
			return ErrConflict
		}
		text := "平台执行检查：本轮已经结束，但本节点尚无完成或交接回执，也没有明确等待信号。请沿用当前会话和工作区，继续完成已授权的节点责任；不用重读整份首轮输入。全部完成后调用已配置完成/交接工具。"
		if n.AllowUserInput == nil || *n.AllowUserInput {
			text += "确需用户回答或真实外部阻塞，调用 wait_for_input 说明具体问题/原因后等待。"
		} else {
			text += "本节点未开放请求用户输入；需要澄清沿合法交接出口处理。无可执行工作且无合法出口，明确报告阻塞，保留现场，不编造输入；达到上限时平台暂停。"
		}
		text += "不要重复创建业务对象，不将局部完成当作完整交付。"
		_, err = tx.Exec(`INSERT INTO messages(conversation_id,role,content,kind,status,created) VALUES(?,'user',?,'workflow_continue','queued',?)`, c.ID, text, now())
		if err == nil {
			_, err = tx.Exec(`INSERT INTO workflow_node_controls(run_id,seq,continuations,last_message_id) VALUES(?,?,1,?) ON CONFLICT(run_id,seq) DO UPDATE SET continuations=continuations+1,last_message_id=excluded.last_message_id,wait_kind='',wait_reason='',wait_message_id=0`, id, seq, mid)
		}
		if err == nil {
			_, err = tx.Exec(`UPDATE conversations SET status='queued',updated=? WHERE id=?;UPDATE workflow_runs SET status='running',updated=? WHERE id=? AND seq=?;UPDATE workflow_steps SET status='running',updated=? WHERE run_id=? AND seq=?`, now(), c.ID, now(), id, seq, now(), id, seq)
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
