package platform

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestCodexTransportResumesIdentityAndKeepsInteractiveApproval(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "same thread", true: "changed thread"}[changed], func(t *testing.T) {
			input, platformInput := io.Pipe()
			platformOutput, output := io.Pipe()
			defer input.Close()
			defer platformInput.Close()
			defer output.Close()
			defer platformOutput.Close()
			go func() {
				defer input.Close()
				defer output.Close()
				decoder, encoder := json.NewDecoder(input), json.NewEncoder(output)
				for {
					var packet struct {
						ID     json.RawMessage `json:"id"`
						Method string          `json:"method"`
						Params map[string]any  `json:"params"`
						Result map[string]any  `json:"result"`
					}
					if decoder.Decode(&packet) != nil {
						return
					}
					switch packet.Method {
					case "initialize":
						encoder.Encode(map[string]any{"id": packet.ID, "result": map[string]any{}})
					case "thread/resume":
						if packet.Params["threadId"] != "original-thread" || packet.Params["cwd"] != "/workspace" {
							t.Error("resume lost task identity", packet.Params)
						}
						id := "original-thread"
						if changed {
							id = "different-thread"
						}
						encoder.Encode(map[string]any{"id": packet.ID, "result": map[string]any{"thread": map[string]any{"id": id}}})
					case "turn/start":
						encoder.Encode(map[string]any{"method": "turn/started", "params": map[string]any{"turn": map[string]any{"id": "turn-one"}}})
						encoder.Encode(map[string]any{"id": 99, "method": "mcpServer/elicitation/request", "params": map[string]any{"threadId": "original-thread", "mode": "form", "_meta": map[string]any{"codex_approval_kind": "mcp_tool_call"}}})
					case "":
						if string(packet.ID) == "99" {
							if packet.Result["action"] != "accept" {
								t.Error("approval response lost", packet.Result)
							}
							encoder.Encode(map[string]any{"method": "item/agentMessage/delta", "params": map[string]any{"itemId": "answer", "delta": "Native answer"}})
							encoder.Encode(map[string]any{"method": "turn/completed", "params": map[string]any{"turn": map[string]any{"status": "completed"}}})
							return
						}
					}
				}
			}()
			approved := false
			executor := Codex{Approve: func(_ context.Context, c Conversation, m Message, _ json.RawMessage) (string, error) {
				approved = c.ID == "role-session" && m.Content == "task input"
				return "accept", nil
			}}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			var events strings.Builder
			err := executor.executeAppServerTransport(ctx, Conversation{ID: "role-session", ThreadID: "original-thread", Snapshot: Agent{Executor: "codex"}}, Message{Content: "task input"}, "/workspace", []byte(""), platformInput, platformOutput, func() string { return "" }, func(raw []byte) error { events.Write(raw); return nil })
			if changed {
				if err == nil || !strings.Contains(err.Error(), "identity changed") || approved {
					t.Fatal("changed native identity accepted", err)
				}
				return
			}
			if err != nil || !approved || !strings.Contains(events.String(), "Native answer") || !strings.Contains(events.String(), "turn.completed") {
				t.Fatal("native transport continuity failed", err, approved, events.String())
			}
		})
	}
}
