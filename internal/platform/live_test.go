package platform

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
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
	var calls atomic.Int32
	service := mcp.NewServer(&mcp.Implementation{Name: "external-proof", Version: "1"}, nil)
	closedWorld := false
	mcp.AddTool(service, &mcp.Tool{Name: "proof_marker", Description: "Return the external verification marker", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld}}, func(ctx context.Context, r *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		calls.Add(1)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "EXTERNAL_TOOL_B63F_OK"}}}, nil, nil
	})
	remote := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return service }, nil))
	defer remote.Close()
	registry := testStore(t)
	v, _ := registry.SaveToolServer(RegisteredToolServer{ID: "proof", Name: "external proof", Enabled: true, Connection: MCPConnection{URL: remote.URL}})
	inventory, err := discoverTools(context.Background(), v.Connection)
	if err != nil {
		t.Fatal(err)
	}
	v.Tools = inventory
	data, _ := json.Marshal(v)
	registry.DB.Exec(`UPDATE tool_servers SET config=? WHERE id=?`, string(data), v.ID)
	a.ToolServers = []ToolBinding{{ServerID: "proof", Tools: []string{"proof_marker"}}}
	a, err = resolveToolServers(registry.DB, a)
	if err != nil {
		t.Fatal(err)
	}
	c := Conversation{ID: "proof", Snapshot: a}
	x := &Codex{Root: root}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	check, e := x.Check(ctx, a)
	if e != nil || !check.Ready {
		t.Fatalf("native check: %+v %v", check, e)
	}
	var events [][]byte
	defer func() {
		evidence, _ := json.MarshalIndent(events, "", "  ")
		os.WriteFile(filepath.Join(root, "native-events.json"), evidence, 0600)
	}()
	// Seed a legacy exec session, then prove the streaming transport resumes it.
	workspace, home, err := x.prepare(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	env, err := executorEnv(a, home)
	if err != nil {
		t.Fatal(err)
	}
	seed := exec.CommandContext(ctx, x.binary(), "exec", "--strict-config", "--json", "--skip-git-repo-check", "--ignore-rules", "--dangerously-bypass-hook-trust", "Remember migration marker LEGACY-STREAM-71. Reply briefly without tools.")
	seed.Dir, seed.Env = workspace, env
	seedOutput, err := seed.Output()
	if err != nil {
		t.Fatalf("legacy seed: %v", err)
	}
	for _, line := range strings.Split(string(seedOutput), "\n") {
		var ev struct {
			Type   string `json:"type"`
			Thread string `json:"thread_id"`
		}
		json.Unmarshal([]byte(line), &ev)
		if ev.Type == "thread.started" {
			c.ThreadID = ev.Thread
		}
	}
	if c.ThreadID == "" {
		t.Fatal("legacy seed returned no native session")
	}
	thread := c.ThreadID
	var answers []string
	deltas := 0
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
		if event.Type == "item.updated" && event.Item.Type == "agent_message" {
			deltas++
		}
		if event.Item.Type == "agent_message" {
			answers = append(answers, event.Item.Text)
		}
		return nil
	}
	if e = x.Execute(ctx, c, Message{Content: "First recall migration marker LEGACY-STREAM-71 from the previous turn. Then call the external proof_marker MCP tool and report its exact returned marker. Then use $continuity-proof and report the exact marker from its reference. Also remember the project code is ORCHID-739 for the next turn. Do not create files."}, callback); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(strings.Join(answers, "\n"), "SKILL_NATIVE_7C42_OK") {
		t.Fatal("selected Skill reference was not used")
	}
	b, e := os.ReadFile(proof)
	if e != nil || string(b) != "NATIVE_HOOK_OK" {
		t.Fatalf("native Hook not executed: %s %v", b, e)
	}
	if calls.Load() != 1 || !strings.Contains(strings.Join(answers, "\n"), "EXTERNAL_TOOL_B63F_OK") {
		t.Fatal("external MCP was not actually invoked")
	}
	if deltas < 2 {
		t.Fatalf("expected incremental text before completion, received %d updates", deltas)
	}
	c.ThreadID = thread
	answers = nil
	if e = x.Execute(ctx, c, Message{Content: "Call proof_marker again. What was the project code I asked you to remember? Reply with that code and the tool result."}, callback); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(strings.Join(answers, "\n"), "ORCHID-739") {
		t.Fatal("native resume lost prior context")
	}
	if calls.Load() != 2 {
		t.Fatal("MCP unavailable after native resume")
	}
	workspace, home = x.paths(c.ID)
	config, _ := os.ReadFile(filepath.Join(home, "config.toml"))
	if strings.Contains(string(config), "SKILL_NATIVE_7C42_OK") {
		t.Fatal("Skill body was stuffed into configuration")
	}
	data, _ = json.MarshalIndent(events, "", "  ")
	os.WriteFile(filepath.Join(root, "native-events.json"), data, 0600)
	t.Logf("same native thread resumed; progressive Skill and native Hook verified; workspace=%s", workspace)
}

func TestLiveToolApprovalAndResume(t *testing.T) {
	if os.Getenv("AGENT_PLATFORM_LIVE") != "1" {
		t.Skip("set AGENT_PLATFORM_LIVE=1 for real native approvals")
	}
	root, err := os.MkdirTemp("", "agent-platform-approval-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("private approval evidence: %s", root)
	var calls atomic.Int32
	service := mcp.NewServer(&mcp.Implementation{Name: "approval-proof", Version: "1"}, nil)
	type args struct {
		Value string `json:"value"`
	}
	mcp.AddTool(service, &mcp.Tool{Name: "save_marker", Description: "Save a verification marker to a test-only local record; this writes data."}, func(ctx context.Context, r *mcp.CallToolRequest, a args) (*mcp.CallToolResult, any, error) {
		calls.Add(1)
		err := os.WriteFile(filepath.Join(root, "marker.txt"), []byte(a.Value), 0600)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "saved " + a.Value}}}, nil, err
	})
	remote := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return service }, nil))
	defer remote.Close()
	a := Agent{Executor: "codex", Name: "approval proof", Sandbox: "read-only", InheritEnv: true, ResolvedTools: map[string]ResolvedToolServer{"proof": {Connection: MCPConnection{URL: remote.URL}, Tools: []string{"save_marker"}}}}
	x := &Codex{Root: root}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	var events []json.RawMessage
	defer func() {
		raw, _ := json.MarshalIndent(events, "", "  ")
		os.WriteFile(filepath.Join(root, "events.json"), raw, 0600)
	}()
	thread := ""
	callback := func(raw []byte) error {
		events = append(events, append(json.RawMessage(nil), raw...))
		var e struct {
			Type   string `json:"type"`
			Thread string `json:"thread_id"`
		}
		json.Unmarshal(raw, &e)
		if e.Type == "thread.started" {
			if thread != "" && thread != e.Thread {
				return errors.New("changed thread")
			}
			thread = e.Thread
		}
		return nil
	}
	c := Conversation{ID: "auto", Snapshot: a}
	if err = x.Execute(ctx, c, Message{Content: "Call save_marker exactly once with value AUTO_OK. It is a test-only local record. Do not write through shell or any other tool. Report its result."}, callback); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("automatic tool did not run once: %d", calls.Load())
	}
	t.Log("automatic approval invoked real write tool")
	hookProof := filepath.Join(root, "confirmed-hook.txt")
	a.TrustHooks = true
	a.NativeConfig = "[[hooks.Stop]]\n[[hooks.Stop.hooks]]\ntype = \"command\"\ncommand = " + quote("printf hook-ok > "+hookProof) + "\ntimeout = 10\n"
	server := a.ResolvedTools["proof"]
	server.Approvals = map[string]string{"save_marker": "confirm"}
	a.ResolvedTools["proof"] = server
	c = Conversation{ID: "manual", Snapshot: a}
	thread = ""
	decision := "accept"
	requests := 0
	x.Approve = func(ctx context.Context, c Conversation, m Message, raw json.RawMessage) (string, error) {
		requests++
		events = append(events, append(json.RawMessage(nil), raw...))
		if calls.Load() != 1 && decision == "accept" {
			return "", errors.New("tool ran before confirmation")
		}
		return decision, nil
	}
	if err = x.Execute(ctx, c, Message{Content: "Call save_marker exactly once with value CONFIRMED_OK. Do not use shell or alternative tools. Wait for the tool approval response. Also remember project code CEDAR-94 for the next turn."}, callback); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || requests != 1 {
		t.Fatalf("confirmed call count=%d requests=%d", calls.Load(), requests)
	}
	hookData, hookErr := os.ReadFile(hookProof)
	if hookErr != nil || string(hookData) != "hook-ok" {
		t.Fatalf("configured Hook lost in approval transport: %s %v", hookData, hookErr)
	}
	t.Log("explicit confirmation released real write tool")
	c.ThreadID = thread
	decision = "decline"
	if err = x.Execute(ctx, c, Message{Content: "Recall the project code, then call save_marker exactly once with value SHOULD_NOT_WRITE. If the user declines the tool request, acknowledge and stop; do not retry or use other tools."}, callback); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || requests != 2 {
		t.Fatalf("declined call executed or no request: calls=%d requests=%d", calls.Load(), requests)
	}
	saved, _ := os.ReadFile(filepath.Join(root, "marker.txt"))
	if string(saved) != "CONFIRMED_OK" {
		t.Fatalf("wrong saved value: %s", saved)
	}
	all, _ := json.Marshal(events)
	if !strings.Contains(string(all), "CEDAR-94") {
		t.Fatal("native continuation lost context")
	}
	t.Log("same session resumed; declined tool had no effect")
}
