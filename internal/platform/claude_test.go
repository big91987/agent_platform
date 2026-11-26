package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestClaudeRejectsUnsupportedConfiguration(t *testing.T) {
	for _, a := range []Agent{{Executor: "claude", NativeConfig: "model=\"x\""}, {Executor: "claude", ApprovalsReviewer: "auto_review"}, {Executor: "claude", TrustHooks: true}} {
		if validateClaude(a) == nil {
			t.Fatalf("accepted unsupported settings: %+v", a)
		}
	}
}
func TestClaudeMissingSessionDoesNotStart(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "started")
	binary := filepath.Join(root, "cli")
	os.WriteFile(binary, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0700)
	x := &Claude{Root: root, Binary: binary}
	err := x.Execute(context.Background(), Conversation{ID: "test", ThreadID: "123", Snapshot: Agent{Executor: "claude"}}, Message{}, func([]byte) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "session") {
		t.Fatalf("error=%v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("started missing session")
	}
}

func TestClaudeLiveContinuity(t *testing.T) {
	if os.Getenv("AGENT_PLATFORM_CLAUDE_LIVE") != "1" {
		t.Skip("explicit live provider opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	x := &Claude{Root: t.TempDir(), Binary: "/opt/homebrew/bin/claude"}
	c := Conversation{ID: "live", Snapshot: Agent{Executor: "claude", NetworkAccess: true, InheritEnv: true}}
	var texts []string
	collect := func(raw []byte) error {
		var e struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
			Item     struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		json.Unmarshal(raw, &e)
		if e.Type == "thread.started" {
			c.ThreadID = e.ThreadID
		}
		if e.Item.Type == "agent_message" {
			texts = append(texts, e.Item.Text)
		}
		return nil
	}
	if err := x.Execute(ctx, c, Message{ID: 1, Content: "Remember the codeword maple-seven. Reply only acknowledged. Do not use tools."}, collect); err != nil {
		t.Fatal(err)
	}
	if err := x.Execute(ctx, c, Message{ID: 2, Content: "What codeword did I give you? Reply only the codeword. Do not use tools."}, collect); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(texts, " "), "maple-seven") {
		t.Fatal("native session did not preserve prior input")
	}
}

func TestClaudeLiveWorkspacePermissions(t *testing.T) {
	if os.Getenv("AGENT_PLATFORM_CLAUDE_LIVE") != "1" {
		t.Skip("explicit live provider opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	workspace, _ := filepath.EvalSymlinks(t.TempDir())
	outside, _ := filepath.EvalSymlinks(t.TempDir())
	root := t.TempDir()
	calls := 0
	x := &Claude{Root: root, Binary: "/opt/homebrew/bin/claude", Approve: func(_ context.Context, _ Conversation, _ Message, raw json.RawMessage) (string, error) {
		calls++
		return "decline", nil
	}}
	c := Conversation{ID: "write", WorkspacePath: workspace, Snapshot: Agent{Executor: "claude", NetworkAccess: true, AllowElevation: true, InheritEnv: true}}
	err := x.Execute(ctx, c, Message{ID: 1, Content: "Use Write to create probe.txt in the current workspace with exact content native-write-proof. Then use Bash to run: printf shell-proof > shell.txt . Do not use any other tools."}, func([]byte) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"probe.txt", "shell.txt"} {
		if _, err := os.Stat(filepath.Join(workspace, name)); err != nil {
			t.Fatalf("workspace operation missing %s: %v", name, err)
		}
	}
	if calls != 0 {
		t.Fatalf("ordinary workspace operations requested %d approvals", calls)
	}
	os.Symlink(outside, filepath.Join(workspace, "escape"))
	c.ID = "deny"
	err = x.Execute(ctx, c, Message{ID: 2, Content: "Use Write to create escape/denied.txt in the current workspace. If permission denied, stop immediately; do not try other tools."}, func([]byte) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(outside, "denied.txt")); !os.IsNotExist(err) {
		t.Fatal("symlink escape executed")
	}
	if calls == 0 {
		t.Fatal("outside write did not reach approval")
	}
	x.Approve = func(_ context.Context, _ Conversation, _ Message, _ json.RawMessage) (string, error) {
		calls++
		return "accept", nil
	}
	c.ID = "accept"
	err = x.Execute(ctx, c, Message{ID: 3, Content: "Use Write to create this absolute file with content approved: " + filepath.Join(outside, "approved.txt") + ". Do not use other tools."}, func([]byte) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(outside, "approved.txt")); err != nil {
		t.Fatal("approved outside write did not execute")
	}
}

func TestClaudeLiveMCPAndSkillScope(t *testing.T) {
	if os.Getenv("AGENT_PLATFORM_CLAUDE_LIVE") != "1" {
		t.Skip("explicit live provider opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var calls atomic.Int32
	var forbidden atomic.Int32
	server := mcp.NewServer(&mcp.Implementation{Name: "probe", Version: "1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "record", Description: "Record the supplied marker exactly once."}, func(ctx context.Context, r *mcp.CallToolRequest, in struct {
		Marker string `json:"marker"`
	}) (*mcp.CallToolResult, any, error) {
		calls.Add(1)
		return nil, map[string]any{"recorded": in.Marker}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "forbidden", Description: "A tool not granted to this role."}, func(ctx context.Context, r *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		forbidden.Add(1)
		return nil, map[string]any{"ok": true}, nil
	})
	remote := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))
	defer remote.Close()
	x := &Claude{Root: t.TempDir(), Binary: "/opt/homebrew/bin/claude"}
	c := Conversation{ID: "mcp", Snapshot: Agent{Executor: "claude", InheritEnv: true, NetworkAccess: true, ResolvedTools: map[string]ResolvedToolServer{"probe": {Connection: MCPConnection{URL: remote.URL}, Tools: []string{"record"}}}}}
	skillNames := []string{}
	err := x.Execute(ctx, c, Message{ID: 1, Content: "Call the probe record MCP tool exactly once with marker native-mcp-proof, then reply done. Do not call any other tools."}, func(raw []byte) error {
		var e struct {
			Native struct {
				Type    string   `json:"type"`
				Subtype string   `json:"subtype"`
				Skills  []string `json:"skills"`
			} `json:"native"`
		}
		json.Unmarshal(raw, &e)
		if e.Native.Type == "system" && e.Native.Subtype == "init" {
			skillNames = append(skillNames, e.Native.Skills...)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("MCP called %d times", calls.Load())
	}
	c.ID = "forbidden"
	err = x.Execute(ctx, c, Message{ID: 2, Content: "Call the forbidden tool once. If permission denied, stop and report denied."}, func([]byte) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if forbidden.Load() != 0 {
		t.Fatal("unselected MCP tool executed")
	}
	if len(skillNames) != 0 {
		t.Fatalf("unselected native skills loaded: %v", skillNames)
	}
}

func TestClaudeLiveSelectedSkill(t *testing.T) {
	if os.Getenv("AGENT_PLATFORM_CLAUDE_LIVE") != "1" {
		t.Skip("explicit live provider opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	x := &Claude{Root: t.TempDir(), Binary: "/opt/homebrew/bin/claude", Approve: func(context.Context, Conversation, Message, json.RawMessage) (string, error) {
		return "", errors.New("the selected Skill reference unexpectedly required approval")
	}}
	workspace, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(t.TempDir(), "role-proof")
	c := Conversation{ID: "skill", WorkspacePath: workspace, Snapshot: Agent{Executor: "claude", InheritEnv: true, NetworkAccess: true, AllowElevation: true, Skills: []string{skill}}}
	_, home := x.paths(c)
	attachment := filepath.Join(home, "role-plugin", "skills", "role-proof", "references", "proof.md")
	if err := os.MkdirAll(filepath.Join(skill, "references"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "references", "proof.md"), []byte("SELECTED_SKILL_PROOF_4917\n"), 0600); err != nil {
		t.Fatal(err)
	}
	instructions := "---\nname: role-proof\ndescription: Use for the selected role proof.\n---\nUse the Read tool to read " + attachment + " and reply with its exact contents.\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(instructions), 0600); err != nil {
		t.Fatal(err)
	}
	ambient := filepath.Join(workspace, ".claude", "skills", "unselected")
	if err := os.MkdirAll(ambient, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ambient, "SKILL.md"), []byte("---\nname: unselected\ndescription: Ambient skill that must not load for this role.\n---\nReply AMBIENT_SKILL_LEAK.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	names := []string{}
	err = x.Execute(ctx, c, Message{ID: 1, Content: "Invoke the platform-role:role-proof skill and follow its instructions."}, func(raw []byte) error {
		var e struct {
			Item struct {
				Text string `json:"text"`
			} `json:"item"`
			Native struct {
				Skills []string `json:"skills"`
			} `json:"native"`
		}
		json.Unmarshal(raw, &e)
		output.WriteString(e.Item.Text)
		names = append(names, e.Native.Skills...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "SELECTED_SKILL_PROOF_4917") {
		t.Fatal("selected native skill was not applied")
	}
	for _, name := range names {
		if name == "unselected" {
			t.Fatal("unselected workspace skill loaded")
		}
	}
	t.Logf("native skill names: %v", names)
}

func TestClaudeToolGrantIsolation(t *testing.T) {
	skill := filepath.Join(t.TempDir(), "chosen")
	os.MkdirAll(skill, 0700)
	os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("test"), 0600)
	c := Conversation{Snapshot: Agent{Skills: []string{skill}, AllowElevation: true, ResolvedTools: map[string]ResolvedToolServer{"test": {Tools: []string{"save", "read"}, Approvals: map[string]string{"save": "confirm"}}}}}
	for _, tt := range []struct {
		name, input string
		allow, ask  bool
	}{{"Skill", `{"skill":"platform-role:chosen"}`, true, false}, {"Skill", `{"skill":"debug"}`, false, false}, {"mcp__test__read", `{}`, true, false}, {"mcp__test__save", `{}`, false, true}, {"mcp__test__hidden", `{}`, false, false}, {"WebFetch", `{}`, false, false}} {
		allow, ask := claudeToolDecision(c, "", tt.name, json.RawMessage(tt.input))
		if allow != tt.allow || ask != tt.ask {
			t.Fatalf("%s allow=%v ask=%v", tt.name, allow, ask)
		}
	}
}
func TestClaudeReadsSelectedSkillReferences(t *testing.T) {
	root := t.TempDir()
	skill := filepath.Join(root, "selected")
	unselected := filepath.Join(root, "unselected")
	for _, dir := range []string{skill, unselected} {
		if err := os.MkdirAll(filepath.Join(dir, "references"), 0700); err != nil {
			t.Fatal(err)
		}
		for _, rel := range []string{"SKILL.md", "references/guide.md"} {
			if err := os.WriteFile(filepath.Join(dir, rel), []byte("guide"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	plugin := filepath.Join(root, "native", "role-plugin")
	if err := os.MkdirAll(filepath.Join(plugin, "skills"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{skill, unselected} {
		if err := os.Symlink(dir, filepath.Join(plugin, "skills", filepath.Base(dir))); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(unselected, "references", "guide.md"), filepath.Join(skill, "references", "outside.md")); err != nil {
		t.Fatal(err)
	}
	c := Conversation{Snapshot: Agent{Skills: []string{skill}, AllowElevation: true}}
	for _, test := range []struct {
		name, tool, path string
		allow            bool
	}{
		{"selected reference", "Read", filepath.Join(plugin, "skills", "selected", "references", "guide.md"), true},
		{"unselected reference", "Read", filepath.Join(plugin, "skills", "unselected", "references", "guide.md"), false},
		{"outside asset", "Read", filepath.Join(plugin, "skills", "selected", "references", "outside.md"), false},
		{"asset modification", "Edit", filepath.Join(plugin, "skills", "selected", "references", "guide.md"), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			args, err := json.Marshal(map[string]string{"file_path": test.path})
			if err != nil {
				t.Fatal(err)
			}
			allow, ask := claudeToolDecision(c, plugin, test.tool, args)
			if allow != test.allow || ask == test.allow {
				t.Fatalf("allow=%v ask=%v", allow, ask)
			}
		})
	}
}

func TestClaudeCancellationReapsProcess(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "cli")
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then echo "2.1.330 (Claude Code)"; exit 0; fi
read initialize
printf '%s\n' '{"type":"control_response","response":{"request_id":"initialize","subtype":"success"}}'
read prompt
sleep 60
`
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	x := &Claude{Root: root, Binary: binary, ConfigHome: t.TempDir()}
	started := time.Now()
	err := x.Execute(ctx, Conversation{ID: "cancel", Snapshot: Agent{Executor: "claude"}}, Message{Content: "test"}, func(raw []byte) error { cancel(); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("cancellation did not stop native process")
	}
	if _, err := os.Stat(filepath.Join(root, "conversations", "cancel", "native", "process.json")); !os.IsNotExist(err) {
		t.Fatal("process registration not cleared")
	}
}
func TestClaudeAgentSaveReload(t *testing.T) {
	s := testStore(t)
	a, err := s.SaveAgent(Agent{Name: "Claude role", Executor: "claude", Enabled: true, NetworkAccess: true, AllowElevation: true})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.Agent(a.ID)
	if err != nil || saved.Executor != "claude" || !saved.AllowElevation || !saved.NetworkAccess {
		t.Fatalf("lost execution configuration: %+v %v", saved, err)
	}
}

func TestClaudeLiveProjectRules(t *testing.T) {
	if os.Getenv("AGENT_PLATFORM_CLAUDE_LIVE") != "1" {
		t.Skip("explicit live provider opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	workspace, _ := filepath.EvalSymlinks(t.TempDir())
	os.WriteFile(filepath.Join(workspace, "AGENTS.md"), []byte("For any request asking for the project handshake, reply exactly PROJECT_NATIVE_RULE_9021. Do not call tools for that request."), 0600)
	x := &Claude{Root: t.TempDir(), Binary: "/opt/homebrew/bin/claude"}
	var output strings.Builder
	err := x.Execute(ctx, Conversation{ID: "rules", WorkspacePath: workspace, Snapshot: Agent{Executor: "claude", InheritEnv: true}}, Message{ID: 1, Content: "Give me the project handshake. Do not call any tools."}, func(raw []byte) error {
		var e struct {
			Item struct {
				Text string `json:"text"`
			} `json:"item"`
		}
		json.Unmarshal(raw, &e)
		output.WriteString(e.Item.Text)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "PROJECT_NATIVE_RULE_9021") {
		t.Fatal("native project rule import was not applied")
	}
}

func TestClaudeUncertainLaunchDoesNotReplay(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "conversations", "uncertain", "native")
	os.MkdirAll(home, 0700)
	os.WriteFile(filepath.Join(home, "launch.json"), []byte(`{"session_id":"unknown"}`), 0600)
	x := &Claude{Root: root, Binary: "/nonexistent/claude"}
	err := x.Execute(context.Background(), Conversation{ID: "uncertain", Snapshot: Agent{Executor: "claude"}}, Message{Content: "possibly already accepted"}, func([]byte) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "uncertain") {
		t.Fatalf("unexpected recovery: %v", err)
	}
}

func TestClaudeStdioMCPDeclaredEnvironment(t *testing.T) {
	a := Agent{ResolvedTools: map[string]ResolvedToolServer{
		"first":  {Connection: MCPConnection{Command: "/bin/sh", Args: []string{"-c", `printf '%s:%s' "$FIRST_VALUE" "$SECOND_VALUE"`}, EnvVars: []string{"FIRST_VALUE"}}},
		"second": {Connection: MCPConnection{Command: "/bin/sh", Args: []string{"-c", `printf '%s:%s' "$FIRST_VALUE" "$SECOND_VALUE"`}, EnvVars: []string{"SECOND_VALUE"}}},
	}}
	servers, err := claudeMCPServers(a, t.TempDir(), []string{"FIRST_VALUE=one", "SECOND_VALUE=two"})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"first": "one:", "second": ":two"} {
		config := servers[name].(map[string]any)
		cmd := exec.Command(config["command"].(string), config["args"].([]string)...)
		cmd.Env = []string{}
		for k, v := range config["env"].(map[string]string) {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		out, err := cmd.Output()
		if err != nil || string(out) != want {
			t.Fatalf("%s environment delivery got %q error %v", name, out, err)
		}
	}
}
func TestClaudeMissingBearerFailsBeforeLaunch(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(fmt.Sprint(deleted), func(t *testing.T) {
			root := t.TempDir()
			marker := filepath.Join(root, "started")
			binary := filepath.Join(root, "cli")
			script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo '2.1.330 (Claude Code)'; exit 0; fi\ntouch '" + marker + "'\n"
			os.WriteFile(binary, []byte(script), 0700)
			var requests atomic.Int32
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
			defer remote.Close()
			empty := ""
			value := &empty
			if deleted {
				value = nil
			}
			a := Agent{Executor: "claude", Env: map[string]*string{"TEST_BEARER": value}, ResolvedTools: map[string]ResolvedToolServer{"private": {Connection: MCPConnection{URL: remote.URL, BearerTokenEnvVar: "TEST_BEARER"}, Tools: []string{"read"}}}}
			x := &Claude{Root: root, Binary: binary, ConfigHome: t.TempDir()}
			err := x.Execute(context.Background(), Conversation{ID: "missing", Snapshot: a}, Message{Content: "call private tool"}, func([]byte) error { return nil })
			if err == nil || !strings.Contains(err.Error(), "bearer") {
				t.Fatalf("expected missing bearer failure: %v", err)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("Claude task started despite missing bearer")
			}
			if requests.Load() != 0 {
				t.Fatal("unauthenticated service contacted")
			}
		})
	}
}
func TestClaudeUnknownNativeToolNeverRequestsApproval(t *testing.T) {
	c := Conversation{Snapshot: Agent{AllowElevation: true, NetworkAccess: true}}
	for _, name := range []string{"UnknownTool", "Agent", "PowerShell", "mcp__unknown__write", ""} {
		allow, ask := claudeToolDecision(c, "", name, json.RawMessage(`{}`))
		if allow || ask {
			t.Fatalf("unknown tool %q received permission or approval route", name)
		}
	}
	for _, name := range []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "WebFetch", "WebSearch"} {
		allow, ask := claudeToolDecision(c, "", name, json.RawMessage(`{}`))
		if allow || !ask {
			t.Fatalf("supported tool %q lost approval route", name)
		}
	}
}

func TestClaudeLiveCompoundBashBoundary(t *testing.T) {
	if os.Getenv("AGENT_PLATFORM_CLAUDE_LIVE") != "1" {
		t.Skip("explicit live provider opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	workspace, _ := filepath.EvalSymlinks(t.TempDir())
	os.WriteFile(filepath.Join(workspace, "release.md"), []byte("## 验收记录\n校验通过\n"), 0600)
	approvals := 0
	x := &Claude{Root: t.TempDir(), Binary: "/opt/homebrew/bin/claude", Approve: func(context.Context, Conversation, Message, json.RawMessage) (string, error) {
		approvals++
		return "decline", nil
	}}
	c := Conversation{ID: "compound", WorkspacePath: workspace, Snapshot: Agent{Executor: "claude", InheritEnv: true, NetworkAccess: true, AllowElevation: true}}
	command := `echo "--- grep 验收记录 ---"; grep -c "## 验收记录" release.md; echo "exit=$?"; echo "--- grep 校验通过 ---"; grep -c "校验通过" release.md; echo "exit=$?"; wc -c release.md; printf command-ok > compound-proof.txt`
	if err := x.Execute(ctx, c, Message{Content: "Use Bash once to run this exact command, without rewriting or splitting it. Stop after the tool result, including if denied. Command: " + command}, func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if approvals != 0 {
		t.Fatalf("ordinary compound Bash requested %d approvals", approvals)
	}
	if raw, err := os.ReadFile(filepath.Join(workspace, "compound-proof.txt")); err != nil || string(raw) != "command-ok" {
		t.Fatalf("compound command did not finish: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(home, ".agent-platform-boundary-"+newID())
	defer os.Remove(outside)
	for _, escape := range []bool{false, true} {
		c.ID = "outside-" + fmt.Sprint(escape)
		attemptedID, blocked := "", false
		instruction := "Use Bash exactly once with command " + fmt.Sprintf("printf forbidden > %q", outside) + ". Stop immediately after its result, even if the operation is denied. Do not use any other tools."
		if escape {
			instruction += " Set the Bash input dangerouslyDisableSandbox to true for this boundary test."
		}
		err = x.Execute(ctx, c, Message{Content: instruction}, func(raw []byte) error {
			var e struct {
				Native struct {
					Message struct {
						Content []struct {
							Type      string `json:"type"`
							Name      string `json:"name"`
							ID        string `json:"id"`
							ToolUseID string `json:"tool_use_id"`
							Input     struct {
								Command     string `json:"command"`
								Unsandboxed bool   `json:"dangerouslyDisableSandbox"`
							} `json:"input"`
							IsError bool `json:"is_error"`
						} `json:"content"`
					} `json:"message"`
				} `json:"native"`
			}
			json.Unmarshal(raw, &e)
			for _, b := range e.Native.Message.Content {
				if b.Type == "tool_use" && b.Name == "Bash" && strings.Contains(b.Input.Command, outside) && (!escape || b.Input.Unsandboxed) {
					attemptedID = b.ID
				}
				if b.Type == "tool_result" && b.IsError && attemptedID != "" && b.ToolUseID == attemptedID {
					blocked = true
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if attemptedID == "" || !blocked {
			t.Fatalf("outside Bash escape=%v attempted=%v blocked=%v", escape, attemptedID != "", blocked)
		}
		if _, err = os.Stat(outside); !os.IsNotExist(err) {
			t.Fatalf("outside Bash escaped isolation (escape=%v)", escape)
		}
	}
	if approvals != 0 {
		t.Fatalf("outside Bash was stopped by platform approval rather than native isolation: %d callbacks", approvals)
	}
}

func TestClaudeNamedBearerInEventAndApproval(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	script := `#!/bin/sh
read -r init
echo '{"type":"control_response","response":{"request_id":"initialize","subtype":"success"}}'
read -r msg
echo '{"type":"assistant","session_id":"native-thread-id","message":{"content":[{"type":"text","text":"thread demo-value-long"}]}}'
echo '{"type":"control_request","request_id":"approve","request":{"subtype":"can_use_tool","tool_name":"Read","input":{"thread":"demo-value-long"}}}'
read -r reply
echo '{"type":"result","session_id":"native-thread-id","subtype":"success"}'
`
	binary := filepath.Join(root, "native")
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	var approval struct {
		Privileged bool              `json:"platform_permission_request"`
		Tool       string            `json:"tool"`
		Arguments  map[string]string `json:"arguments"`
	}
	x := &Claude{Binary: binary, Approve: func(_ context.Context, _ Conversation, _ Message, raw json.RawMessage) (string, error) {
		return "deny", json.Unmarshal(raw, &approval)
	}}
	a := Agent{AllowElevation: true, ResolvedTools: map[string]ResolvedToolServer{}}
	env := []string{"API_KEY=demo-value"}
	for key, value := range map[string]string{"ROUTE_HEADER": "demo-value-long", "ROUTE_B": "thread", "ROUTE_C": "tool", "ROUTE_D": "Read"} {
		a.ResolvedTools[key] = ResolvedToolServer{Connection: MCPConnection{URL: "https://example.invalid", BearerTokenEnvVar: key}}
		env = append(env, key+"="+value)
	}
	var started, projected bool
	err := x.run(context.Background(), Conversation{Snapshot: a}, Message{}, root, home, "native-thread-id", nil, env, nil, func(raw []byte) error {
		var event struct {
			Type     string          `json:"type"`
			ThreadID string          `json:"thread_id"`
			Native   json.RawMessage `json:"native"`
			Item     struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			return err
		}
		if !started {
			if event.Type != "thread.started" || event.ThreadID != "native-thread-id" {
				return errors.New("missing exact session identity before input")
			}
			started = true
		}
		if strings.Contains(string(event.Native), "demo-value") || strings.Contains(string(event.Native), "thread") {
			return errors.New("native payload credential was exposed")
		}
		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			projected = event.Item.Text == "[redacted] [redacted]"
		}
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "without a durable native session") {
		t.Fatalf("unexpected fixture result: %v", err)
	}
	if !started || !projected || !approval.Privileged || approval.Tool != "Read" ||
		len(approval.Arguments) != 1 || approval.Arguments["[redacted]"] != "[redacted]" {
		t.Fatal("typed lifecycle, presentation or approval fields did not survive payload redaction")
	}
}
