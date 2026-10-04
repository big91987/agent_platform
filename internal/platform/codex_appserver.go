package platform

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// One native transport supplies incremental text and interactive tool approvals.
// CODEX_HOME and thread IDs also resume sessions created by codex exec.
func (x *Codex) executeAppServer(ctx context.Context, c Conversation, m Message, workspace, home string, env []string, onEvent func([]byte) error) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	nonce := "agent-platform-run-" + newID()
	supervisor := `IFS= read -r start || exit 1; [ "$start" = "$0" ] || exit 1; "$@" <&0 & child=$!; wait "$child"`
	cmd := exec.CommandContext(runCtx, "/bin/sh", "-c", supervisor, nonce, x.binary(), "app-server", "--strict-config", "--stdio")
	cmd.Dir, cmd.Env = workspace, env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 3 * time.Second
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	// Always reap the process group, including while waiting for a human decision.
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
			return
		case <-runCtx.Done():
		}
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	registered := false
	defer func() {
		cancel()
		input.Close()
		syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		cmd.Wait()
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		close(done)
		if registered {
			os.Remove(filepath.Join(home, "process.json"))
		}
	}()
	if err = registerProcess(home, cmd.Process.Pid, nonce); err != nil {
		return err
	}

	registered = true
	if _, err = io.WriteString(input, nonce+"\n"); err != nil {
		return err
	}
	send := func(v any) error { return json.NewEncoder(input).Encode(v) }
	request := func(id int, method string, params any) error {
		return send(map[string]any{"id": id, "method": method, "params": params})
	}
	redact := x.redactor(c.Snapshot)
	emit := func(v any) error {
		raw, e := json.Marshal(v)
		if e != nil {
			return e
		}
		return onEvent(redactJSON(raw, redact))
	}
	var stderrMu sync.Mutex
	var stderrText strings.Builder
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scan := bufio.NewScanner(stderr)
		for scan.Scan() {
			stderrMu.Lock()
			if stderrText.Len() < 16000 {
				stderrText.WriteString(redact(scan.Text()) + "\n")
			}
			stderrMu.Unlock()
		}
	}()
	type packet struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	frames := make(chan packet, 32)
	readErrors := make(chan error, 1)
	approvalCtx, abortApproval := context.WithCancel(runCtx)
	defer abortApproval()
	go func() {
		defer abortApproval()
		defer close(frames)
		scan := bufio.NewScanner(output)
		scan.Buffer(make([]byte, 4096), 4*1024*1024)
		for scan.Scan() {
			var p packet
			if e := json.Unmarshal(scan.Bytes(), &p); e != nil {
				readErrors <- fmt.Errorf("invalid app-server frame: %w", e)
				return
			}
			select {
			case frames <- p:
			case <-runCtx.Done():
				return
			}
		}
		e := scan.Err()
		if e == nil {
			e = io.ErrUnexpectedEOF
		}
		readErrors <- e
	}()
	if err = request(1, "initialize", map[string]any{"capabilities": map[string]any{"experimentalApi": true}, "clientInfo": map[string]string{"name": "agent_platform", "title": "Agent Platform", "version": "0.1.0"}}); err != nil {
		return err
	}
	thread := ""
	turnID := ""
	steerID := 100
	pendingSteers := map[string]steeringInput{}
	defer func() {
		for _, req := range pendingSteers {
			req.Finish(errors.New("引导接收结果未知，请检查会话；未自动重发"), false)
		}
	}()
	textByItem := map[string]string{}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case req := <-steeringChannel(ctx):
			if turnID == "" {
				if err := req.Finish(errors.New("当前轮尚未开始或已结束，消息仍在队列中"), true); err != nil {
					return err
				}
				continue
			}
			steerID++
			pendingSteers[fmt.Sprint(steerID)] = req
			if err = request(steerID, "turn/steer", map[string]any{"threadId": thread, "expectedTurnId": turnID, "input": []any{map[string]any{"type": "text", "text": req.Message.Content}}}); err != nil {
				return err
			}
		case p, ok := <-frames:
			if !ok {
				err = <-readErrors
				stderrMu.Lock()
				detail := stderrText.String()
				stderrMu.Unlock()
				return fmt.Errorf("Codex app-server ended before turn completion: %w %s", err, detail)
			}
			if req, ok := pendingSteers[string(p.ID)]; ok && p.Method == "" {
				var rejected error
				if len(p.Error) > 0 {
					rejected = fmt.Errorf("Codex 拒绝引导：%s", redact(string(p.Error)))
				}
				if err = req.Finish(rejected, rejected != nil); err != nil {
					return err
				}
				delete(pendingSteers, string(p.ID))
				continue
			}
			if len(p.Error) > 0 {
				return fmt.Errorf("Codex app-server: %s", redact(string(p.Error)))
			}
			if len(p.ID) > 0 && p.Method != "" {
				if isPermissionRequest(p.Method) {
					result, e := x.nativePermissionApproval(approvalCtx, c, m, p.Method, p.Params, thread, turnID, redact)
					if e != nil {
						return e
					}
					if e = send(map[string]any{"id": p.ID, "result": result}); e != nil {
						return e
					}
					if e = emit(map[string]any{"type": "platform.approval.resolved", "method": p.Method}); e != nil {
						return e
					}
					continue
				}
				if p.Method != "mcpServer/elicitation/request" {
					return fmt.Errorf("unsupported native request: %s", p.Method)
				}
				var params struct {
					ThreadID string         `json:"threadId"`
					Mode     string         `json:"mode"`
					Meta     map[string]any `json:"_meta"`
				}
				if err = json.Unmarshal(p.Params, &params); err != nil {
					return err
				}
				// Only the native tool-call approval form is supported, not arbitrary forms
				// requiring passwords, URLs or other user-provided values.
				if params.ThreadID != thread || params.Mode != "form" || params.Meta["codex_approval_kind"] != "mcp_tool_call" {
					return errors.New("unsupported MCP input request; tool approval requires a native tool-call confirmation form")
				}
				if x.Approve == nil {
					return errors.New("tool confirmation handler is unavailable")
				}
				decision, e := x.Approve(approvalCtx, c, m, redactJSON(p.Params, redact))
				if e != nil {
					return e
				}
				if decision != "accept" && decision != "decline" && decision != "cancel" {
					return errors.New("invalid approval decision")
				}
				var content any
				if decision == "accept" {
					content = map[string]any{}
				}
				if err = send(map[string]any{"id": p.ID, "result": map[string]any{"action": decision, "content": content, "_meta": nil}}); err != nil {
					return err
				}
				if err = emit(map[string]any{"type": "platform.approval.resolved", "decision": decision}); err != nil {
					return err
				}
				continue
			}
			if len(p.ID) > 0 {
				switch string(p.ID) {
				case "1":
					if err = send(map[string]any{"method": "initialized"}); err != nil {
						return err
					}
					raw, e := os.ReadFile(filepath.Join(home, "config.toml"))
					if os.IsNotExist(e) {
						raw, e = nativeConfig(c.Snapshot)
					}
					if e != nil {
						return e
					}
					var config map[string]any
					if e = toml.Unmarshal(raw, &config); e != nil {
						return e
					}
					options := map[string]any{"cwd": workspace, "approvalPolicy": nativeApprovalPolicy(c.Snapshot), "sandbox": nativeSandboxMode(c), "approvalsReviewer": "user"}
					guidance, _ := config["developer_instructions"].(string)
					if c.ReadOnly {
						options["sandbox"] = "read-only"
						guidance += "\nCurrent workspace access is read-only. Do not modify workspace files. Use registered management tools to pass corrections or stop/redirect delegated work."
					}
					options["developerInstructions"] = strings.TrimSpace(guidance + "\n\nThis turn is a conversation with the user in Agent Platform. Reply in natural language/Markdown unless the current user input explicitly requests a structured format. Do not apply an output envelope from a different integration context to this conversation.")
					if c.Snapshot.TrustHooks {
						options["config"] = map[string]any{"bypass_hook_trust": true}
					}
					method := "thread/start"
					if c.ThreadID != "" {
						method = "thread/resume"
						options["threadId"] = c.ThreadID
					}
					if err = request(2, method, options); err != nil {
						return err
					}
				case "2":
					var result struct {
						Thread struct {
							ID string `json:"id"`
						} `json:"thread"`
					}
					if err = json.Unmarshal(p.Result, &result); err != nil {
						return err
					}
					thread = result.Thread.ID
					if thread == "" || (c.ThreadID != "" && c.ThreadID != thread) {
						return errors.New("native session identity changed during resume")
					}
					if err = emit(map[string]any{"type": "thread.started", "thread_id": thread}); err != nil {
						return err
					}
					if err = request(3, "turn/start", map[string]any{"threadId": thread, "approvalPolicy": nativeApprovalPolicy(c.Snapshot), "approvalsReviewer": "user", "sandboxPolicy": turnSandbox(c), "input": []any{map[string]any{"type": "text", "text": m.Content}}}); err != nil {
						return err
					}
				}
				continue
			}
			var params map[string]any
			if len(p.Params) > 0 {
				if err = json.Unmarshal(p.Params, &params); err != nil {
					return err
				}
			}
			native := map[string]any{"method": p.Method, "params": params}
			event := map[string]any{"type": "native." + p.Method, "native": native}
			switch p.Method {
			case "turn/started":
				turn, _ := params["turn"].(map[string]any)
				turnID, _ = turn["id"].(string)
				event["type"] = "turn.started"
			case "item/agentMessage/delta":
				id, _ := params["itemId"].(string)
				delta, _ := params["delta"].(string)
				textByItem[id] += delta
				event["type"] = "item.updated"
				event["item"] = map[string]any{"id": id, "type": "agent_message", "text": textByItem[id]}
			case "item/started", "item/completed":
				item, _ := params["item"].(map[string]any)
				if item != nil {
					itemType, _ := item["type"].(string)
					normalized := map[string]any{}
					for k, v := range item {
						normalized[k] = v
					}
					switch itemType {
					case "agentMessage":
						normalized["type"] = "agent_message"
					case "commandExecution":
						normalized["type"] = "command_execution"
					case "mcpToolCall":
						normalized["type"] = "mcp_tool_call"
					case "fileChange":
						normalized["type"] = "file_change"
					}
					event["item"] = normalized
					event["type"] = strings.Replace(p.Method, "/", ".", 1)
				}
			case "turn/completed":
				turn, _ := params["turn"].(map[string]any)
				if turn["status"] != "completed" {
					event["type"] = "turn.failed"
					if err = emit(event); err != nil {
						return err
					}
					return fmt.Errorf("native turn %v: %s", turn["status"], redact(fmt.Sprint(turn["error"])))
				}
				event["type"] = "turn.completed"
				return emit(event)
			}
			if err = emit(event); err != nil {
				return err
			}
		}
	}
}
