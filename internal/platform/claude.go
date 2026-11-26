package platform

// Claude Code uses its own stream-json control protocol. No Codex wire messages
// are sent to Claude; only presentation events share the platform's item shape.
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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Claude struct {
	Root, Binary, ConfigHome string
	Store                    *Store
	Approve                  func(context.Context, Conversation, Message, json.RawMessage) (string, error)
}

func validateClaude(a Agent) error {
	if a.NativeConfig != "" || a.TrustHooks {
		return errors.New("Claude does not accept native TOML or unscoped Hooks")
	}
	if nativeApprovalReviewer(a) != "user" {
		return errors.New("Claude requires user approval; Codex auto_review is unsupported")
	}
	if a.Sandbox != "" && a.Sandbox != "workspace-write" && a.Sandbox != "read-only" {
		return errors.New("unsupported Claude file access setting")
	}
	for k := range a.Env {
		if strings.HasPrefix(k, "CLAUDE_") {
			return errors.New("CLAUDE_ environment is managed by the platform")
		}
	}
	return validateAgentExecutionLimits(a)
}
func validateExecutor(a Agent) error {
	switch a.Executor {
	case "claude":
		return validateClaude(a)
	case "codex":
		_, err := nativeConfig(a)
		return err
	default:
		return errors.New("unsupported executor")
	}
}
func (x *Claude) binary() string {
	if x.Binary != "" {
		return x.Binary
	}
	return "claude"
}
func (x *Claude) configHome() string {
	if x.ConfigHome != "" {
		return x.ConfigHome
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".claude")
}
func (x *Claude) paths(c Conversation) (string, string) {
	w := c.WorkspacePath
	if w == "" {
		w = filepath.Join(x.Root, "conversations", c.ID, "workspace")
	}
	return w, filepath.Join(x.Root, "conversations", c.ID, "native")
}
func claudeSession(home, id string) bool {
	if id == "" || strings.ContainsAny(id, "/\\") {
		return false
	}
	matches, _ := filepath.Glob(filepath.Join(home, "projects", "*", id+".jsonl"))
	return len(matches) == 1
}

// Provider settings are selected explicitly. Never inherit permission rules,
// plugins, project settings, hooks, or skill catalogs from the operator account.
func (x *Claude) provider(a Agent, home string) ([]string, map[string]any, error) {
	base, err := executorEnv(a, home)
	if err != nil {
		return nil, nil, err
	}
	values := map[string]string{}
	for _, e := range base {
		k, v, _ := strings.Cut(e, "=")
		if !strings.HasPrefix(k, "CLAUDE_") && !strings.HasPrefix(k, "CODEX_") {
			values[k] = v
		}
	}
	settings := map[string]any{}
	raw, err := os.ReadFile(filepath.Join(x.configHome(), "settings.json"))
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	if len(raw) > 0 {
		var source map[string]json.RawMessage
		if err = json.Unmarshal(raw, &source); err != nil {
			return nil, nil, errors.New("invalid Claude provider settings")
		}
		var env map[string]string
		json.Unmarshal(source["env"], &env)
		for k, v := range env {
			if strings.HasPrefix(k, "ANTHROPIC_") {
				if _, override := a.Env[k]; !override {
					values[k] = v
				}
			}
		}
		for _, k := range []string{"model"} {
			if v, ok := source[k]; ok {
				settings[k] = v
			}
		}
	}
	values["CLAUDE_CONFIG_DIR"] = home
	values["CLAUDE_CODE_DISABLE_AUTO_MEMORY"] = "1"
	values["CLAUDE_CODE_DISABLE_BACKGROUND_TASKS"] = "1"
	values["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] = "1"
	out := []string{}
	for k, v := range values {
		out = append(out, k+"="+v)
	}
	return out, settings, nil
}
func (x *Claude) version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, x.binary(), "--version").Output()
	if err != nil {
		return "", errors.New("Claude CLI unavailable")
	}
	version := strings.TrimSpace(string(out))
	parts := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`).FindStringSubmatch(version)
	if len(parts) != 4 {
		return "", errors.New("cannot verify Claude CLI version")
	}
	major, _ := strconv.Atoi(parts[1])
	minor, _ := strconv.Atoi(parts[2])
	patch, _ := strconv.Atoi(parts[3])
	if major < 2 || (major == 2 && (minor < 1 || (minor == 1 && patch < 330))) {
		return "", errors.New("Claude Code 2.1.330 or newer is required for the managed permission boundary")
	}
	return version, nil
}
func (x *Claude) Check(ctx context.Context, a Agent) (CheckResult, error) {
	if err := validateClaude(a); err != nil {
		return CheckResult{}, err
	}
	version, err := x.version(ctx)
	if err != nil {
		return CheckResult{}, err
	}
	for _, p := range a.Skills {
		if _, err := normalizeSkill(p); err != nil {
			return CheckResult{}, err
		}
	}
	env, _, err := x.provider(a, x.configHome())
	if err != nil {
		return CheckResult{}, err
	}
	configured := false
	for _, entry := range env {
		k, v, _ := strings.Cut(entry, "=")
		if (k == "ANTHROPIC_API_KEY" || k == "ANTHROPIC_AUTH_TOKEN") && v != "" {
			configured = true
		}
	}
	if _, err := os.Stat(filepath.Join(x.configHome(), ".credentials.json")); err == nil {
		configured = true
	}
	if !configured {
		return CheckResult{}, errors.New("Claude provider credentials unavailable; configure local provider environment or native credential file")
	}
	return CheckResult{Ready: true, Version: version, Skills: a.Skills}, nil
}
func (x *Claude) Execute(ctx context.Context, c Conversation, m Message, onEvent func([]byte) error) error {
	if err := validateClaude(c.Snapshot); err != nil {
		return err
	}
	if x.Store != nil {
		run, err := x.Store.sandboxRunForConversation(c.ID)
		if err != nil {
			return err
		}
		if run != nil {
			return errors.New("Claude execution supports local workspaces only; OpenSandbox is unsupported")
		}
	}
	workspace, home := x.paths(c)
	if c.ThreadID != "" && !claudeSession(home, c.ThreadID) {
		return errors.New("Claude native session is missing; no replacement session was created")
	}
	if c.ThreadID == "" {
		if _, err := os.Stat(filepath.Join(home, "launch.json")); err == nil {
			return errors.New("previous Claude launch outcome is uncertain; inspect before retrying")
		}
	}
	if _, err := x.version(ctx); err != nil {
		return err
	}
	if c.WorkspacePath != "" {
		p, err := normalizeWorkspace(workspace)
		if err != nil {
			return err
		}
		if p != workspace {
			return errors.New("workspace path changed")
		}
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(workspace, 0700); err != nil {
		return err
	}
	if c.ThreadID == "" && c.WorkspacePath == "" {
		if err := copySeed(c.Snapshot.SeedDir, workspace); err != nil {
			return err
		}
	}
	env, settings, err := x.provider(c.Snapshot, home)
	if err != nil {
		return err
	}
	// Credential references remain on this host, never copied into managed data.
	credential := filepath.Join(x.configHome(), ".credentials.json")
	if _, err := os.Stat(credential); err == nil {
		target := filepath.Join(home, ".credentials.json")
		if _, err = os.Lstat(target); os.IsNotExist(err) {
			if err = os.Symlink(credential, target); err != nil {
				return err
			}
		}
	}
	settings["disableAllHooks"] = true
	mode := "acceptEdits"
	if nativeSandboxMode(c) == "read-only" {
		mode = "default"
	}
	permissions := map[string]any{"defaultMode": mode, "ask": []string{"Skill", "mcp__*"}}
	// Native auto-allow still prompts for some compound commands. The explicit
	// Bash rule removes that prompt; mandatory OS sandboxing below remains the
	// filesystem/network boundary, with no excluded commands or escape hatch.
	if nativeSandboxMode(c) != "read-only" {
		permissions["allow"] = []string{"Bash"}
	}
	settings["permissions"] = permissions
	domains := []string{}
	if c.Snapshot.NetworkAccess {
		domains = []string{"*"}
	}
	settings["sandbox"] = map[string]any{"enabled": true, "failIfUnavailable": true, "autoAllowBashIfSandboxed": true, "allowUnsandboxedCommands": false, "network": map[string]any{"allowedDomains": domains}, "filesystem": map[string]any{"allowWrite": []string{workspace}}}
	settingsRaw, _ := json.Marshal(settings)
	settingsPath := filepath.Join(home, "settings.json")
	if err = os.WriteFile(settingsPath, settingsRaw, 0600); err != nil {
		return err
	}
	// Scoped skills are delivered as a private native plugin. The initialize
	// allowlist also prevents discovery of ambient skills in a shared workspace.
	plugin := filepath.Join(home, "role-plugin")
	os.RemoveAll(plugin)
	if err = os.MkdirAll(filepath.Join(plugin, ".claude-plugin"), 0700); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(plugin, ".claude-plugin", "plugin.json"), []byte(`{"name":"platform-role","version":"1.0.0"}`), 0600); err != nil {
		return err
	}
	skills := []string{}
	for _, p := range c.Snapshot.Skills {
		p, err = normalizeSkill(p)
		if err != nil {
			return err
		}
		name := filepath.Base(p)
		target := filepath.Join(plugin, "skills", name)
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		if err = os.Symlink(p, target); err != nil {
			return err
		}
		skills = append(skills, "platform-role:"+name)
	}
	a := bindToolWorkspace(c.Snapshot, workspace)
	servers, err := claudeMCPServers(a, workspace, env)
	if err != nil {
		return err
	}
	mcpRaw, _ := json.Marshal(map[string]any{"mcpServers": servers})
	mcpPath := filepath.Join(home, "mcp.json")
	if err = os.WriteFile(mcpPath, mcpRaw, 0600); err != nil {
		return err
	}
	args := []string{"--print", "--verbose", "--input-format", "stream-json", "--output-format", "stream-json", "--permission-prompt-tool", "stdio", "--permission-mode", mode, "--setting-sources", "user", "--settings", settingsPath, "--strict-mcp-config", "--mcp-config", mcpPath, "--plugin-dir", plugin, "--tools", "Bash,Read,Write,Edit,Glob,Grep,WebFetch,WebSearch,Skill"}
	if len(skills) == 0 {
		args = append(args, "--disable-slash-commands")
	}
	if nativeSandboxMode(c) == "read-only" {
		args = append(args, "--disallowedTools", "Bash,Write,Edit")
	}
	if !c.Snapshot.NetworkAccess {
		args = append(args, "--disallowedTools", "WebFetch,WebSearch")
	}
	if a.Model != "" {
		args = append(args, "--model", a.Model)
	}
	// Claude's native project-rule loader follows this local import reference.
	if _, e := os.Stat(filepath.Join(workspace, "AGENTS.md")); e == nil {
		if err = os.WriteFile(filepath.Join(home, "CLAUDE.md"), []byte("@"+filepath.Join(workspace, "AGENTS.md")+"\n"), 0600); err != nil {
			return err
		}
	}
	if a.Instructions != "" {
		args = append(args, "--append-system-prompt", a.Instructions)
	}
	id := c.ThreadID
	if id == "" {
		v := newID()
		id = v[:8] + "-" + v[8:12] + "-" + v[12:16] + "-" + v[16:20] + "-" + v[20:]
		args = append(args, "--session-id", id)
	} else {
		args = append(args, "--resume", id)
	}
	return x.run(ctx, c, m, workspace, home, id, args, env, skills, onEvent)
}
func (x *Claude) run(ctx context.Context, c Conversation, m Message, workspace, home, id string, args, env, skills []string, onEvent func([]byte) error) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	nonce := "agent-platform-run-" + newID()
	supervisor := `IFS= read -r start || exit 1; [ "$start" = "$0" ] || exit 1; "$@" <&0 & child=$!; wait "$child"`
	command := append([]string{"-c", supervisor, nonce, x.binary()}, args...)
	cmd := exec.CommandContext(runCtx, "/bin/sh", command...)
	cmd.Dir = workspace
	cmd.Env = env
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
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		return errors.New("cannot start Claude CLI")
	}
	registered := false
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
	launch, _ := json.Marshal(map[string]any{"session_id": id, "message_id": m.ID})
	if err = os.WriteFile(filepath.Join(home, "launch.json"), launch, 0600); err != nil {
		return err
	}
	if _, err = io.WriteString(input, nonce+"\n"); err != nil {
		return err
	}
	enc := json.NewEncoder(input)
	if err = enc.Encode(map[string]any{"type": "control_request", "request_id": "initialize", "request": map[string]any{"subtype": "initialize", "skills": skills}}); err != nil {
		return err
	}
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 65536), 16*1024*1024)
	initialized, started, result := false, false, false
	redact := claudeRedactor(c.Snapshot, env, home)
	// Only redact exported content. The scheduler and approval router consume
	// the exact envelope fields, tool names and native identifiers.
	payload := func(raw json.RawMessage) json.RawMessage {
		return redactJSONContent(raw, redact, true)
	}
	emit := func(v any) error {
		raw, e := json.Marshal(v)
		if e != nil {
			return e
		}
		return onEvent(raw)
	}
	for scanner.Scan() {
		raw := append([]byte(nil), scanner.Bytes()...)
		var event map[string]json.RawMessage
		if err = json.Unmarshal(raw, &event); err != nil {
			return errors.New("invalid Claude stream event")
		}
		var kind string
		json.Unmarshal(event["type"], &kind)
		if kind == "control_response" && !initialized {
			var response struct {
				RequestID string `json:"request_id"`
				Subtype   string `json:"subtype"`
			}
			json.Unmarshal(event["response"], &response)
			if response.RequestID != "initialize" || response.Subtype != "success" {
				return errors.New("Claude initialization rejected")
			}
			initialized = true
			if err = emit(map[string]any{"type": "thread.started", "thread_id": id}); err != nil {
				return err
			}
			started = true
			if err = enc.Encode(map[string]any{"type": "user", "client_composed": true, "session_id": id, "message": map[string]any{"role": "user", "content": m.Content}}); err != nil {
				return err
			}
			continue
		}
		if kind == "control_request" {
			var req struct {
				RequestID string `json:"request_id"`
				Request   struct {
					Subtype  string          `json:"subtype"`
					ToolName string          `json:"tool_name"`
					Input    json.RawMessage `json:"input"`
				} `json:"request"`
			}
			json.Unmarshal(raw, &req)
			if req.Request.Subtype != "can_use_tool" {
				return errors.New("unsupported Claude control request")
			}
			answer := map[string]any{"behavior": "deny", "message": "Tool is not authorized"}
			if req.Request.Subtype == "can_use_tool" {
				allow, ask := claudeToolDecision(c, req.Request.ToolName, req.Request.Input)
				if ask && x.Approve != nil {
					safe, _ := json.Marshal(map[string]any{"platform_permission_request": !strings.HasPrefix(req.Request.ToolName, "mcp__"), "tool": req.Request.ToolName, "arguments": payload(req.Request.Input)})
					decision, e := x.Approve(ctx, c, m, safe)
					if e != nil {
						return e
					}
					allow = decision == "accept"
				}
				if allow {
					answer = map[string]any{"behavior": "allow", "updatedInput": req.Request.Input}
				}
			}
			if err = enc.Encode(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": req.RequestID, "response": answer}}); err != nil {
				return err
			}
			continue
		}
		if !started {
			return errors.New("Claude emitted output before initialization")
		}
		var session string
		json.Unmarshal(event["session_id"], &session)
		if (session != "" && session != id) || ((kind == "assistant" || kind == "result") && session == "") {
			return errors.New("Claude native session identity changed")
		}
		native := payload(raw)
		if err = emit(map[string]any{"type": "claude." + kind, "native": native}); err != nil {
			return err
		}
		if kind == "assistant" || kind == "user" {
			var msg struct {
				ID      string `json:"id"`
				Content []struct {
					Type      string          `json:"type"`
					ID        string          `json:"id"`
					Text      string          `json:"text"`
					Name      string          `json:"name"`
					Input     json.RawMessage `json:"input"`
					ToolUseID string          `json:"tool_use_id"`
					Content   json.RawMessage `json:"content"`
				} `json:"content"`
			}
			if json.Unmarshal(event["message"], &msg) == nil {
				for i, b := range msg.Content {
					item := map[string]any{}
					typ := "item.completed"
					switch b.Type {
					case "text":
						if kind != "assistant" {
							continue
						}
						item = map[string]any{"id": fmt.Sprintf("%s-%d", msg.ID, i), "type": "agent_message", "text": redact(b.Text)}
					case "tool_use":
						typ = "item.started"
						item = map[string]any{"id": b.ID, "type": "mcp_tool_call", "tool": b.Name, "arguments": payload(b.Input)}
					case "tool_result":
						item = map[string]any{"id": b.ToolUseID, "type": "mcp_tool_call", "result": payload(b.Content)}
					default:
						continue
					}
					if err = emit(map[string]any{"type": typ, "item": item, "native": native}); err != nil {
						return err
					}
				}
			}
		}
		if kind == "result" {
			result = true
			var isError bool
			json.Unmarshal(event["is_error"], &isError)
			if isError {
				return errors.New("Claude reported a failed turn; inspect native result")
			}
			input.Close()
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err = scanner.Err(); err != nil {
		return err
	}
	if !result {
		return errors.New("Claude ended without a result; inspect native session before retrying")
	}
	if err := cmd.Wait(); err != nil {
		return errors.New("Claude process failed after its result; inspect native session before retrying")
	}
	if !claudeSession(home, id) {
		return errors.New("Claude completed without a durable native session; inspect before retrying")
	}
	return nil
}
func claudeToolDecision(c Conversation, name string, input json.RawMessage) (bool, bool) {
	if name == "Skill" {
		var request struct {
			Skill string `json:"skill"`
		}
		if json.Unmarshal(input, &request) != nil {
			return false, false
		}
		for _, path := range c.Snapshot.Skills {
			normalized, err := normalizeSkill(path)
			if err == nil && request.Skill == "platform-role:"+filepath.Base(normalized) {
				return true, false
			}
		}
		return false, false
	}

	if strings.HasPrefix(name, "mcp__") {
		for server, binding := range c.Snapshot.ResolvedTools {
			for _, tool := range binding.Tools {
				if name == "mcp__"+server+"__"+tool {
					return binding.Approvals[tool] != "confirm", binding.Approvals[tool] == "confirm"
				}
			}
		}
		return false, false
	}
	switch name {
	case "Bash", "Read", "Write", "Edit", "Glob", "Grep", "WebFetch", "WebSearch":
	default:
		return false, false
	}
	if nativeSandboxMode(c) == "read-only" && (name == "Bash" || name == "Write" || name == "Edit") {
		return false, false
	}
	if !c.Snapshot.NetworkAccess && (name == "WebFetch" || name == "WebSearch") {
		return false, false
	}
	// Native sandbox is the automatic write boundary. Requests reaching this
	// callback require explicit administrator permission, never blanket bypass.
	return false, c.Snapshot.AllowElevation
}

// Each MCP subprocess receives its declared environment explicitly; Claude's
// own environment is not a substitute for a server's connection contract.
func claudeMCPServers(a Agent, workspace string, env []string) (map[string]any, error) {
	values := map[string]string{}
	for _, entry := range env {
		k, v, _ := strings.Cut(entry, "=")
		values[k] = v
	}
	servers := map[string]any{}
	for name, server := range a.ResolvedTools {
		conn := server.Connection
		v := map[string]any{}
		if conn.URL != "" {
			v["type"] = "http"
			v["url"] = conn.URL
			if conn.BearerTokenEnvVar != "" {
				value, ok := values[conn.BearerTokenEnvVar]
				if !ok || strings.TrimSpace(value) == "" {
					return nil, fmt.Errorf("MCP server %s bearer credential is missing or empty", name)
				}
				v["headers"] = map[string]string{"Authorization": "Bearer " + value}
			}
		} else {
			if conn.CWD != "" && conn.CWD != workspace {
				return nil, errors.New("Claude stdio MCP cwd must match the task workspace")
			}
			v["type"] = "stdio"
			v["command"] = conn.Command
			v["args"] = conn.Args
			serverEnv := map[string]string{}
			for _, key := range conn.EnvVars {
				value, ok := values[key]
				if !ok {
					return nil, fmt.Errorf("MCP server %s environment variable %s is missing", name, key)
				}
				serverEnv[key] = value
			}
			v["env"] = serverEnv
		}
		servers[name] = v
	}
	return servers, nil
}

func claudeRedactor(agent Agent, env []string, home string) func(string) string {
	secrets := []string{}
	bearerNames := map[string]bool{}
	for _, binding := range agent.ResolvedTools {
		if conn := binding.Connection; conn.URL != "" && conn.BearerTokenEnvVar != "" {
			bearerNames[conn.BearerTokenEnvVar] = true
		}
	}
	for _, entry := range env {
		k, v, _ := strings.Cut(entry, "=")
		if (v != "" && bearerNames[k]) || len(v) >= 8 && (strings.Contains(k, "TOKEN") || strings.Contains(k, "KEY") || strings.Contains(k, "SECRET") || strings.Contains(k, "PASSWORD")) {
			secrets = append(secrets, v)
		}
	}
	// Native credential files can be used by this host through a reference.
	// Their token values receive the same export redaction as provider env.
	if raw, e := os.ReadFile(filepath.Join(home, ".credentials.json")); e == nil {
		var value any
		var collect func(any)
		collect = func(v any) {
			switch item := v.(type) {
			case string:
				if len(item) >= 12 {
					secrets = append(secrets, item)
				}
			case map[string]any:
				for _, child := range item {
					collect(child)
				}
			case []any:
				for _, child := range item {
					collect(child)
				}
			}
		}
		if json.Unmarshal(raw, &value) == nil {
			collect(value)
		}
	}
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	return func(text string) string {
		for _, secret := range secrets {
			text = strings.ReplaceAll(text, secret, "[redacted]")
		}
		return text
	}
}
