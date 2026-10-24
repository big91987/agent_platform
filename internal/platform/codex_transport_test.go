package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
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
			err := executor.executeAppServerTransport(ctx, Conversation{ID: "role-session", ThreadID: "original-thread", Snapshot: Agent{Executor: "codex"}}, Message{Content: "task input"}, "/workspace", []byte(""), platformInput, platformOutput, func() string { return "" }, func(raw []byte) error { events.Write(raw); return nil }, nil, nil)
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

func TestNativeTransportUsesRequestedModelAndRefusesFallbackBeforeTurn(t *testing.T) {
	s := testStore(t)
	s.initAuth("password")
	token, err := s.UserToken("admin", false)
	if err != nil {
		t.Fatal(err)
	}
	agent := Agent{Executor: "codex", Model: "gpt-6.1-sol", Instructions: "Role from visible draft", NativeConfig: "developer_instructions='Native guidance'"}
	w := independentWorkflow(t, map[string]any{"executor": agent.Executor, "instructions": agent.Instructions, "native_config": agent.NativeConfig})
	w.ContextVersion = 2
	w.Nodes[0].Prompt = ""
	response := requestJSON(t, NewServer(s, nil, nil, "password", "http://localhost"), "POST", "/api/workflows/preview", token, map[string]any{"workflow": w, "node_id": "worker"})
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var preview struct {
		System string `json:"system_prompt"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil || preview.System == "" {
		t.Fatal(err, response.Body.String())
	}
	for _, resumed := range []bool{false, true} {
		for _, actualModel := range []string{"gpt-6.1-sol", "different-model"} {
			t.Run(fmt.Sprintf("%s/resume=%t", actualModel, resumed), func(t *testing.T) {
				inbound, toNative := io.Pipe()
				fromNative, outbound := io.Pipe()
				defer inbound.Close()
				defer toNative.Close()
				defer outbound.Close()
				defer fromNative.Close()
				var turns atomic.Int32
				go func() {
					defer inbound.Close()
					defer outbound.Close()
					decoder, encoder := json.NewDecoder(inbound), json.NewEncoder(outbound)
					for {
						var packet struct {
							ID     json.RawMessage `json:"id"`
							Method string          `json:"method"`
							Params map[string]any  `json:"params"`
						}
						if decoder.Decode(&packet) != nil {
							return
						}
						switch packet.Method {
						case "initialize":
							encoder.Encode(map[string]any{"id": packet.ID, "result": map[string]any{}})
						case "thread/start", "thread/resume":
							if packet.Params["developerInstructions"] != preview.System {
								t.Error("native instructions differ from complete preview", packet.Params["developerInstructions"], preview.System)
							}
							if packet.Params["model"] != "gpt-6.1-sol" || (!resumed && packet.Params["allowProviderModelFallback"] != false) || (resumed && packet.Params["threadId"] != "new-stable-thread") {
								t.Error("requested model could be replaced by project/default configuration", packet.Params)
							}
							encoder.Encode(map[string]any{"id": packet.ID, "result": map[string]any{"model": actualModel, "thread": map[string]any{"id": "new-stable-thread"}}})
						case "turn/start":
							turns.Add(1)
							encoder.Encode(map[string]any{"method": "turn/completed", "params": map[string]any{"turn": map[string]any{"status": "completed"}}})
							return
						}
					}
				}()
				executor := Codex{AuthHome: t.TempDir()}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				persistedThread := ""
				conversation := Conversation{ID: "model-binding", Snapshot: agent}
				if resumed {
					conversation.ThreadID = "new-stable-thread"
				}
				err := executor.executeAppServerTransport(ctx, conversation, Message{Content: "original task"}, "/workspace", nil, toNative, fromNative, func() string { return "" }, func(raw []byte) error {
					var event struct {
						Type     string `json:"type"`
						ThreadID string `json:"thread_id"`
					}
					json.Unmarshal(raw, &event)
					if event.Type == "thread.started" {
						persistedThread = event.ThreadID
					}
					return nil
				}, nil, nil)
				if persistedThread != "new-stable-thread" {
					t.Fatal("created native identity not retained", persistedThread, err)
				}
				if actualModel == "gpt-6.1-sol" {
					if err != nil || turns.Load() != 1 {
						t.Fatal("requested model turn refused", err, turns.Load())
					}
				} else if err == nil || turns.Load() != 0 {
					t.Fatal("fallback model received a task", err, turns.Load())
				}
			})
		}
	}
}
