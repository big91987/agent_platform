package platform

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This is an opt-in real CLI test; unit-suite substitutes do not prove native behavior.
func TestLiveNativeSkillHookAndResume(t *testing.T) {
	if os.Getenv("AGENT_PLATFORM_LIVE") != "1" {
		t.Skip("set AGENT_PLATFORM_LIVE=1 to run the real local Codex test")
	}
	root, e := os.MkdirTemp("", "agent-platform-live-")
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("private native evidence: %s", root)
	skill := filepath.Join(root, "skill")
	os.MkdirAll(skill, 0700)
	os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: continuity-proof\ndescription: Use when the user asks for the continuity proof marker.\n---\n\nRead references/marker.txt and include its exact marker in your final response. Do not infer the marker from the name.\n"), 0600)
	os.MkdirAll(filepath.Join(skill, "references"), 0700)
	os.WriteFile(filepath.Join(skill, "references", "marker.txt"), []byte("SKILL_NATIVE_7C42_OK"), 0600)
	proof := filepath.Join(root, "hook-proof.txt")
	command := "python3 -c 'from pathlib import Path; Path(" + quote(proof) + ").write_text(\"NATIVE_HOOK_OK\"); print(\"{}\")'"
	a := Agent{Executor: "codex", Name: "native proof", Sandbox: "workspace-write", InheritEnv: true, Skills: []string{skill}, TrustHooks: true, NativeConfig: "[[hooks.Stop]]\n[[hooks.Stop.hooks]]\ntype = \"command\"\ncommand = " + quote(command) + "\ntimeout = 10\n"}
	c := Conversation{ID: "proof", Snapshot: a}
	x := &Codex{Root: root}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	check, e := x.Check(ctx, a)
	if e != nil || !check.Ready {
		t.Fatalf("native check: %+v %v", check, e)
	}
	var events [][]byte
	var thread string
	var answers []string
	callback := func(raw []byte) error {
		events = append(events, append([]byte(nil), raw...))
		var event struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
			Item     struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		json.Unmarshal(raw, &event)
		if event.Type == "thread.started" {
			if thread != "" && thread != event.ThreadID {
				t.Fatalf("thread changed: %s / %s", thread, event.ThreadID)
			}
			thread = event.ThreadID
		}
		if event.Item.Type == "agent_message" {
			answers = append(answers, event.Item.Text)
		}
		return nil
	}
	if e = x.Execute(ctx, c, Message{Content: "Use $continuity-proof and report the exact marker from its reference. Also remember the project code is ORCHID-739 for the next turn. Do not create files."}, callback); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(strings.Join(answers, "\n"), "SKILL_NATIVE_7C42_OK") {
		t.Fatal("selected Skill reference was not used")
	}
	b, e := os.ReadFile(proof)
	if e != nil || string(b) != "NATIVE_HOOK_OK" {
		t.Fatalf("native Hook not executed: %s %v", b, e)
	}
	c.ThreadID = thread
	answers = nil
	if e = x.Execute(ctx, c, Message{Content: "What was the project code I asked you to remember? Reply with only that code."}, callback); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(strings.Join(answers, "\n"), "ORCHID-739") {
		t.Fatal("native resume lost prior context")
	}
	workspace, home := x.paths(c.ID)
	config, _ := os.ReadFile(filepath.Join(home, "config.toml"))
	if strings.Contains(string(config), "SKILL_NATIVE_7C42_OK") {
		t.Fatal("Skill body was stuffed into configuration")
	}
	data, _ := json.MarshalIndent(events, "", "  ")
	os.WriteFile(filepath.Join(root, "native-events.json"), data, 0600)
	t.Logf("same native thread resumed; progressive Skill and native Hook verified; workspace=%s", workspace)
}
