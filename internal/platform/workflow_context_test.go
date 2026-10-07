package platform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func markdownRun(t *testing.T, s *Store) (Caller, WorkflowRun) {
	t.Helper()
	c, r := agentRun(t, s)
	w := r.Definition
	// Original legacy fixture run has no conversation and is stopped through the engine.
	e := NewWorkflowEngine(s, nil, "http://localhost")
	if err := e.Stop(c, r.ID, r.Seq); err != nil {
		t.Fatal(err)
	}
	e.Tick(context.Background())
	raw, _ := json.Marshal(w)
	var v map[string]any
	json.Unmarshal(raw, &v)
	v["context_version"] = 1
	n := v["nodes"].([]any)[0].(map[string]any)
	n["prompt"] = "完成本节点工作。\n{{handoff}}"
	n["continuation_limit"] = 2
	n["execution_timeout_seconds"] = 14400
	raw, _ = json.Marshal(v)
	json.Unmarshal(raw, &w)
	w, err := s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "UNIQUE TASK marker", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	return c, r
}
func TestWorkflowMarkdownSeparatesRoleAndTask(t *testing.T) {
	s := testStore(t)
	c, r := markdownRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	r, _ = s.WorkflowRun(c, r.ID)
	conv, err := s.Conversation(r.Steps[0].ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(conv.Snapshot.Instructions, "UNIQUE TASK") || strings.Contains(conv.Snapshot.Instructions, "previous_results") {
		t.Fatal("task and history leaked into role instructions")
	}
	msgs, _ := s.Messages(conv.ID)
	if len(msgs) != 1 || !strings.Contains(msgs[0].Content, "## 当前任务") || !strings.Contains(msgs[0].Content, "UNIQUE TASK") {
		t.Fatalf("missing actual Markdown task input: %v", msgs)
	}
}
func TestWorkflowMarkdownContinuesUnfinishedTurnOnce(t *testing.T) {
	s := testStore(t)
	c, r := markdownRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Complete(conv.ID, msg.ID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	engine.Tick(context.Background())
	msgs, _ := s.Messages(conv.ID)
	if len(msgs) != 2 {
		t.Fatalf("unfinished turn should queue exactly one continuation, got %d", len(msgs))
	}
	if msgs[1].Kind != "workflow_continue" {
		t.Fatalf("continuation source missing: %v", msgs[1])
	}
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Seq != 1 || r.Steps[0].Result != nil {
		t.Fatal("ordinary turn was marked node complete")
	}
}

func TestWorkflowMarkdownPromptValidationAndModes(t *testing.T) {
	s := testStore(t)
	_, r := markdownRun(t, s)
	w := r.Definition
	n := w.Nodes[0]
	for _, prompt := range []string{"missing", "{{handoff}} {{handoff}}", "{{handoff}} {{unknown}}"} {
		n.Prompt = prompt
		if _, err := workflowSessionPrompt(w, n); err == nil {
			t.Fatalf("accepted invalid placeholder: %s", prompt)
		}
	}
	n.Prompt = "{{handoff}}"
	w.Edges[0].Mode = "automatic"
	// Fixture has one completion edge and no remaining handoff edge.
	w.Edges = w.Edges[:1]
	text, err := workflowSessionPrompt(w, n)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "## 自主交接") || strings.Contains(text, "{{handoff}}") {
		t.Fatal("fixed mode leaked handoff SOP")
	}
	for _, name := range workflowNodeTools(w, n) {
		if name == "handoff" {
			t.Fatal("fixed node received handoff tool")
		}
	}
}
func TestWorkflowMarkdownWaitAndUserReplyStayInSession(t *testing.T) {
	s := testStore(t)
	c, r := markdownRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	token := *conv.Snapshot.Env[workflowTokenEnv]
	if err = s.waitWorkflowNode(r.ID, 1, token, WorkflowWait{Kind: "clarification", Reason: "请选择账单周期"}); err != nil {
		t.Fatal(err)
	}
	if err = s.Complete(conv.ID, msg.ID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	engine.Tick(context.Background())
	msgs, _ := s.Messages(conv.ID)
	if len(msgs) != 1 {
		t.Fatal("real clarification automatically continued")
	}
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "waiting" || r.Steps[0].WaitKind != "clarification" {
		t.Fatal("explicit wait not visible")
	}
	receipt, err := s.Submit(c, Input{UserID: r.Owner, ConversationID: conv.ID, Message: "使用九月，修正之前的范围", RequestID: "clarification-reply"})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ConversationID != conv.ID {
		t.Fatal("new conversation created")
	}
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Steps[0].WaitKind != "" {
		t.Fatal("accepted user reply left stale clarification state")
	}
	resumed, next, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	if next.Content != "使用九月，修正之前的范围" {
		t.Fatal("repeated whole context")
	}
	s.Complete(resumed.ID, next.ID, "completed", "")
	engine.Tick(context.Background())
	msgs, _ = s.Messages(conv.ID)
	if len(msgs) != 3 {
		t.Fatal("new user message did not clear explicit wait")
	}
}
func TestWorkflowMarkdownContinuationBudgetAndStop(t *testing.T) {
	s := testStore(t)
	c, r := markdownRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	for i := 0; i < 3; i++ {
		conv, msg, err := s.Claim()
		if err != nil {
			t.Fatal(err)
		}
		s.Complete(conv.ID, msg.ID, "completed", "")
		engine.Tick(context.Background())
	}
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "waiting" || r.Steps[0].Continuations != 2 || r.Steps[0].WaitKind != "limit" {
		t.Fatalf("budget failed: %+v", r.Steps[0])
	}
	fresh := NewWorkflowEngine(s, nil, "http://localhost")
	fresh.Tick(context.Background())
	msgs, _ := s.Messages(r.Steps[0].ConversationID)
	if len(msgs) != 3 {
		t.Fatal("restart duplicated continuation")
	}
	if err := engine.Stop(c, r.ID, r.Seq); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	fresh.Tick(context.Background())
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "stopped" {
		t.Fatal("user stop ignored")
	}
	msgs, _ = s.Messages(r.Steps[0].ConversationID)
	if len(msgs) != 3 {
		t.Fatal("continued after user stop")
	}
}
func TestWorkflowMarkdownCorrectionHandoffAndReturnInput(t *testing.T) {
	s := testStore(t)
	c, r := markdownRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	r, _ = s.WorkflowRun(c, r.ID)
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	s.Complete(conv.ID, msg.ID, "completed", "")
	_, err = s.Submit(c, Input{UserID: r.Owner, ConversationID: conv.ID, Message: "有效修正：不是美元，是人民币", RequestID: "correction"})
	if err != nil {
		t.Fatal(err)
	}
	conv, msg, err = s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	s.Complete(conv.ID, msg.ID, "completed", "")
	r.Steps[0].Result = &NodeResult{Summary: "真实上游结论，代码版本abc", Artifacts: []string{"docs/requirements.md"}, Inputs: map[string]string{"unresolved": "分页失败，证据test.log"}}
	r.Steps = append(r.Steps, WorkflowStep{Seq: 2, NodeID: r.Steps[0].NodeID, Error: "人工回退：重新核验"})
	text, err := s.workflowAgentInput(r, r.Steps[1], r.Definition.Nodes[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"有效修正：不是美元，是人民币", "真实上游结论", "docs/requirements.md", "分页失败", "人工回退"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(text, "previous_results") {
		t.Fatal("cumulative JSON copied")
	}
}

func TestWorkflowMarkdownRejectsAmbiguousTargetStrategy(t *testing.T) {
	s := testStore(t)
	_, r := markdownRun(t, s)
	w := r.Definition
	w.Edges = append(w.Edges, WorkflowEdge{Source: w.Nodes[0].ID, Route: "other_choice", Target: w.Edges[0].Target, Mode: "handoff", Description: "另一策略"})
	if err := validateWorkflow(w); err == nil {
		t.Fatal("new target-only contract accepted ambiguous duplicate handoff target")
	}
	w.ContextVersion = 0
	if err := validateWorkflow(w); err != nil {
		t.Fatalf("legacy graph was silently invalidated: %v", err)
	}
}
func TestWorkflowMarkdownRealReturnCarriesCancelledFeedback(t *testing.T) {
	s := testStore(t)
	c, r := markdownRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	engine.Tick(context.Background())
	// Supported stop and return before this Agent produced a result.
	if err := engine.Stop(c, r.ID, r.Seq); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	if err := engine.Return(c, r.ID, r.Seq, r.Definition.Nodes[0].ID, "界面范围错误，请按原型重新核对，版本abc"); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	r, _ = s.WorkflowRun(c, r.ID)
	msgs, err := s.Messages(r.Steps[len(r.Steps)-1].ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || !strings.Contains(msgs[0].Content, "界面范围错误，请按原型重新核对，版本abc") {
		t.Fatal("actual stopped/cancelled Return lost user correction")
	}
}

func TestWorkflowMarkdownContinuationTimeLimit(t *testing.T) {
	s := testStore(t)
	c, r := markdownRun(t, s)
	engine := NewWorkflowEngine(s, nil, "http://localhost")
	if err := engine.Stop(c, r.ID, r.Seq); err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	w := r.Definition
	w.Nodes[0].ExecutionTimeoutSeconds = 1
	w, err := s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "deadline proof", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	engine.Tick(context.Background())
	conv, msg, err := s.Claim()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	s.Complete(conv.ID, msg.ID, "completed", "")
	engine.Tick(context.Background())
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "waiting" || r.Steps[0].WaitKind != "limit" || r.Steps[0].Continuations != 0 || !strings.Contains(r.Steps[0].WaitReason, "时间") {
		t.Fatal("elapsed continuation deadline did not retain unfinished waiting state")
	}
}
