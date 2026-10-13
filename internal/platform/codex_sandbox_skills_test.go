package platform

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSandboxNativeChecksSkillScopeBeforeThread(t *testing.T) {
	for _, mode := range []string{"selected", "none", "missing", "discovery error", "write rejected", "write unknown", "extra after reload", "wrong cwd"} {
		t.Run(mode, func(t *testing.T) {
			in, toNative := io.Pipe()
			fromNative, out := io.Pipe()
			defer in.Close()
			defer toNative.Close()
			defer fromNative.Close()
			defer out.Close()
			selectedPath := "/var/lib/agent-platform/roles/abc/skills/selected-0/SKILL.md"
			selected := []string{selectedPath}
			if mode == "none" {
				selected = []string{}
			}
			var threads, lists, writes atomic.Int32
			go func() {
				defer out.Close()
				defer in.Close()
				decoder, encoder := json.NewDecoder(in), json.NewEncoder(out)
				for {
					var p struct {
						ID     json.RawMessage `json:"id"`
						Method string          `json:"method"`
						Params map[string]any  `json:"params"`
					}
					if decoder.Decode(&p) != nil {
						return
					}
					reply := func(value any) { encoder.Encode(map[string]any{"id": p.ID, "result": value}) }
					switch p.Method {
					case "initialize":
						reply(map[string]any{})
					case "skills/list":
						n := lists.Add(1)
						cwds, _ := p.Params["cwds"].([]any)
						if p.Params["forceReload"] != true || len(cwds) != 1 || cwds[0] != "/workspace" {
							t.Error("Skill scope not read from shared workspace", p.Params)
						}
						skills := []nativeSkill{{Path: selectedPath, Enabled: n > 1}, {Path: "/workspace/.agents/skills/unselected/SKILL.md", Enabled: n == 1 || mode == "extra after reload"}}
						if mode == "missing" || mode == "none" {
							skills = skills[1:]
						}
						errors := []string{}
						if mode == "discovery error" {
							errors = append(errors, "invalid skill")
						}
						cwd := "/workspace"
						if mode == "wrong cwd" {
							cwd = "/other-workspace"
						}
						reply(map[string]any{"data": []any{map[string]any{"cwd": cwd, "skills": skills, "errors": errors}}})
					case "skills/config/write":
						writes.Add(1)
						value, ok := p.Params["enabled"].(bool)
						if !ok || (p.Params["path"] == selectedPath) != value {
							t.Error("incorrect Skill enable choice", p.Params)
						}
						if mode == "write unknown" {
							reply(map[string]any{})
						} else {
							if mode == "write rejected" {
								value = !value
							}
							reply(map[string]any{"effectiveEnabled": value})
						}
					case "thread/start":
						threads.Add(1)
						if lists.Load() != 2 || writes.Load() == 0 {
							t.Error("task thread opened before actual Skill scope check")
						}
						reply(map[string]any{"thread": map[string]any{"id": "skill-thread"}})
					case "turn/start":
						encoder.Encode(map[string]any{"method": "turn/completed", "params": map[string]any{"turn": map[string]any{"status": "completed"}}})
						return
					}
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := (&Codex{AuthHome: t.TempDir()}).executeAppServerTransport(ctx, Conversation{Snapshot: Agent{Executor: "codex"}}, Message{Content: "private task"}, "/workspace", nil, toNative, fromNative, func() string { return "" }, func([]byte) error { return nil }, selected)
			if mode == "selected" || mode == "none" {
				if err != nil || threads.Load() != 1 {
					t.Fatal("valid Skill scope rejected", err, threads.Load())
				}
			} else if err == nil || threads.Load() != 0 || !strings.Contains(err.Error(), "Skill") {
				t.Fatal("unverified Skill scope received a task", mode, err, threads.Load())
			}
		})
	}
}

func TestSandboxSkillScopeRequiresExplicitNativeEnabledState(t *testing.T) {
	raw := json.RawMessage(`{"data":[{"cwd":"/workspace","errors":[],"skills":[{"name":"unselected","path":"/workspace/.agents/skills/unselected/SKILL.md"}]}]}`)
	if _, err := sandboxNativeSkills(raw, "/workspace"); err == nil {
		t.Fatal("missing native enabled state was silently accepted as disabled")
	}
}
