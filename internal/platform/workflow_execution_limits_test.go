package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExecutionFailureRetriesRetainAttempts(t *testing.T) {
	s := testStore(t)
	c, w, dir, _ := executionFixture(t, s, func(context.Context, string) (*mcp.CallToolResult, any, error) {
		return nil, map[string]any{"ok": true}, nil
	})
	raw, _ := json.Marshal(w.Nodes[0].Execution)
	var config map[string]any
	_ = json.Unmarshal(raw, &config)
	config["retry"] = map[string]any{"max_retries": 1, "interval_seconds": 1}
	config["on_error"] = map[string]any{"action": "route", "target": "done", "title": "执行异常"}
	raw, _ = json.Marshal(config)
	_ = json.Unmarshal(raw, w.Nodes[0].Execution)
	w, err := s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "retry"})
	if err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "")
	receipt := ConnectorReceipt{Kind: "mcp"}
	// This transport failure happened before tools/call, so no external write occurred.
	_ = json.Unmarshal([]byte(`{"kind":"mcp","retry_safe":true}`), &receipt)
	step := r.Steps[0]
	raw, _ = json.Marshal(w.Nodes[0].Execution)
	if err = e.recordWorkflowAction(r, step, string(raw), false); err != nil {
		t.Fatal(err)
	}
	if err = e.finishWorkflowAction(r, step, receipt, errors.New("connect failed")); err != nil {
		t.Fatal(err)
	}
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "running" || len(r.Steps) != 2 || r.Steps[0].Error != "connect failed" || r.Steps[1].NodeID != "call" {
		t.Fatalf("retry not scheduled with preserved evidence: %+v", r)
	}
	// A fresh engine must respect the persisted delay instead of dispatching immediately.
	e = NewWorkflowEngine(s, nil, "")
	e.Tick(context.Background())
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Steps[1].ExecutionDispatched {
		t.Fatal("retry interval ignored")
	}
	step = r.Steps[1]
	if err = e.recordWorkflowAction(r, step, string(raw), false); err != nil {
		t.Fatal(err)
	}
	if err = e.finishWorkflowAction(r, step, receipt, errors.New("connect failed again")); err != nil {
		t.Fatal(err)
	}
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Seq != 2 || r.Steps[1].Result == nil || r.Steps[1].Result.Route != "rule_execution" {
		t.Fatalf("exhausted retries did not route: %+v", r)
	}
	if err = s.advanceWorkflow(r.ID, 2); err != nil {
		t.Fatal(err)
	}
	r, _ = s.WorkflowRun(c, r.ID)
	if r.Status != "completed" || r.Steps[0].ExecutionReceipt == nil || r.Steps[1].ExecutionReceipt == nil {
		t.Fatalf("lost attempts: %+v", r)
	}
}

func TestExecutionUnknownOutcomeAndCancellationNeverRetryOrRoute(t *testing.T) {
	for _, tc := range []struct {
		name string
		safe bool
		err  error
	}{{"unknown", false, context.DeadlineExceeded}, {"cancelled", true, context.Canceled}} {
		t.Run(tc.name, func(t *testing.T) {
			s := testStore(t)
			c, w, dir, _ := executionFixture(t, s, func(context.Context, string) (*mcp.CallToolResult, any, error) { return nil, map[string]any{}, nil })
			w.Nodes[0].Execution.Retry = &ExecutionRetry{MaxRetries: 3, IntervalSeconds: 1}
			w.Nodes[0].Execution.OnError = &WorkflowRuleFailure{Action: "route", Target: "done", Title: "失败"}
			w, err := s.SaveWorkflow(c, w)
			if err != nil {
				t.Fatal(err)
			}
			r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "unknown"})
			if err != nil {
				t.Fatal(err)
			}
			e := NewWorkflowEngine(s, nil, "")
			raw, _ := json.Marshal(w.Nodes[0].Execution)
			if err = e.recordWorkflowAction(r, r.Steps[0], string(raw), false); err != nil {
				t.Fatal(err)
			}
			if err = e.finishWorkflowAction(r, r.Steps[0], ConnectorReceipt{Kind: "mcp", RetrySafe: tc.safe}, tc.err); err != nil {
				t.Fatal(err)
			}
			r, _ = s.WorkflowRun(c, r.ID)
			if r.Status != "failed" || r.Seq != 1 || r.Steps[0].Result != nil {
				t.Fatalf("replayed or routed unconfirmed result: %+v", r)
			}
		})
	}
}

func TestExecutionRetryDelayStopAndRunLimit(t *testing.T) {
	for _, limit := range []int{1, 10} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			s := testStore(t)
			c, w, dir, _ := executionFixture(t, s, func(context.Context, string) (*mcp.CallToolResult, any, error) {
				t.Error("unexpected tool call")
				return nil, map[string]any{}, nil
			})
			w.MaxSteps = limit
			w.Nodes[0].Execution.Retry = &ExecutionRetry{MaxRetries: 3, IntervalSeconds: 60}
			w, err := s.SaveWorkflow(c, w)
			if err != nil {
				t.Fatal(err)
			}
			r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "bounded"})
			if err != nil {
				t.Fatal(err)
			}
			e := NewWorkflowEngine(s, nil, "")
			raw, _ := json.Marshal(w.Nodes[0].Execution)
			if err = e.recordWorkflowAction(r, r.Steps[0], string(raw), false); err != nil {
				t.Fatal(err)
			}
			if err = e.finishWorkflowAction(r, r.Steps[0], ConnectorReceipt{Kind: "mcp", RetrySafe: true}, errors.New("connect failed")); err != nil {
				t.Fatal(err)
			}
			r, _ = s.WorkflowRun(c, r.ID)
			if limit == 1 {
				if r.Status != "failed" || r.Seq != 1 || !strings.Contains(r.Error, workflowExecutionLimitError) {
					t.Fatalf("limit escaped: %+v", r)
				}
				return
			}
			if err = e.Stop(c, r.ID, r.Seq); err != nil {
				t.Fatal(err)
			}
			e = NewWorkflowEngine(s, nil, "")
			e.Tick(context.Background())
			r, _ = s.WorkflowRun(c, r.ID)
			if r.Status != "stopped" || r.Steps[1].ExecutionDispatched {
				t.Fatalf("stopped retry dispatched: %+v", r)
			}
		})
	}
}

func TestExecutionMCPTimeoutAfterCallIsNotReplayed(t *testing.T) {
	s := testStore(t)
	var calls atomic.Int32
	c, w, dir, _ := executionFixture(t, s, func(ctx context.Context, _ string) (*mcp.CallToolResult, any, error) {
		calls.Add(1)
		<-ctx.Done()
		return nil, nil, ctx.Err()
	})
	w.Nodes[0].ExecutionTimeoutSeconds = 1
	w.Nodes[0].Execution.Retry = &ExecutionRetry{MaxRetries: 2, IntervalSeconds: 1}
	w.Nodes[0].Execution.OnError = &WorkflowRuleFailure{Action: "route", Target: "done", Title: "异常"}
	w, err := s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "")
	r = waitConnectorRun(t, e, r.ID, "failed")
	if calls.Load() != 1 || r.Seq != 1 || r.Steps[0].ExecutionReceipt.RetrySafe || r.Steps[0].ExecutionReceipt.TimeoutSeconds != 1 {
		t.Fatalf("unsafe timeout recovery: %+v calls=%d", r, calls.Load())
	}
	NewWorkflowEngine(s, nil, "").Tick(context.Background())
	if calls.Load() != 1 {
		t.Fatal("restart repeated a timed out write")
	}
}

func TestExecutionMCPConnectFailureRecoversWithoutRepeatingTool(t *testing.T) {
	s := testStore(t)
	var blocked atomic.Bool
	var calls atomic.Int32
	c, w, dir, _ := executionFixture(t, s, func(context.Context, string) (*mcp.CallToolResult, any, error) {
		calls.Add(1)
		return nil, map[string]any{"ok": true}, nil
	}, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if blocked.Load() {
				http.Error(w, "unavailable", 503)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	w.Nodes[0].Execution.Retry = &ExecutionRetry{MaxRetries: 1, IntervalSeconds: 1}
	w, err := s.SaveWorkflow(c, w)
	if err != nil {
		t.Fatal(err)
	}
	blocked.Store(true)
	r, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, WorkspacePath: dir, Input: "connect retry"})
	if err != nil {
		t.Fatal(err)
	}
	e := NewWorkflowEngine(s, nil, "")
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		e.Tick(context.Background())
		r, _ = s.WorkflowRun(c, r.ID)
		if r.Seq == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if r.Seq != 2 || r.Steps[0].ExecutionReceipt == nil || !r.Steps[0].ExecutionReceipt.RetrySafe || calls.Load() != 0 {
		t.Fatalf("connect failure not safely scheduled: %+v", r)
	}
	blocked.Store(false)
	r = waitConnectorRun(t, NewWorkflowEngine(s, nil, ""), r.ID, "completed")
	if calls.Load() != 1 || len(r.Steps) != 3 || r.Steps[1].RetryAttempt != 1 || r.Steps[0].Error == "" {
		t.Fatalf("retry lost evidence or repeated tool: %+v", r)
	}
}

func TestExecutionMCPConfigurationFailureAndCancelledConnectNotRetryable(t *testing.T) {
	t.Setenv("TEST_MCP_MISSING", "")
	_, err := callMCPTool(context.Background(), MCPConnection{URL: "http://127.0.0.1:1", BearerTokenEnvVar: "TEST_MCP_MISSING"}, "test", json.RawMessage(`{}`))
	if err == nil || mcpNotCalled(err) {
		t.Fatalf("configuration failure allowed retry: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = callMCPTool(ctx, MCPConnection{URL: "http://127.0.0.1:1"}, "test", json.RawMessage(`{}`))
	if err == nil || mcpNotCalled(err) {
		t.Fatalf("cancelled connect allowed retry: %v", err)
	}
}
