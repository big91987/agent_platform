package platform

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Codex struct{ Root, Binary, AuthHome string }
type CheckResult struct {
	Ready   bool     `json:"ready"`
	Version string   `json:"version"`
	Skills  []string `json:"skills"`
	Error   string   `json:"error,omitempty"`
}

func executorEnv(a Agent, home string) ([]string, error) {
	values := map[string]string{}
	if a.InheritEnv {
		for _, entry := range os.Environ() {
			k, v, _ := strings.Cut(entry, "=")
			if !strings.HasPrefix(k, "CODEX_") && !strings.HasPrefix(k, "AGENT_PLATFORM_") {
				values[k] = v
			}
		}
	} else {
		for _, k := range []string{"PATH", "HOME", "USER", "TMPDIR", "LANG"} {
			if v := os.Getenv(k); v != "" {
				values[k] = v
			}
		}
	}
	for k, v := range a.Env {
		if k == "HOME" || strings.HasPrefix(k, "CODEX_") || strings.HasPrefix(k, "AGENT_PLATFORM_") || strings.ContainsAny(k, "=\x00") || k == "" {
			return nil, fmt.Errorf("environment key is reserved or invalid: %s", k)
		}
		if v == nil {
			delete(values, k)
		} else {
			if strings.ContainsRune(*v, 0) {
				return nil, errors.New("environment value contains NUL")
			}
			values[k] = *v
		}
	}
	values["CODEX_HOME"] = home
	values["NO_COLOR"] = "1"
	out := make([]string, 0, len(values))
	for k, v := range values {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out, nil
}
func nativeConfig(a Agent) ([]byte, error) {
	cfg := map[string]any{}
	if a.NativeConfig != "" {
		if e := toml.Unmarshal([]byte(a.NativeConfig), &cfg); e != nil {
			return nil, fmt.Errorf("invalid native TOML: %w", e)
		}
	}
	for _, k := range []string{"approval_policy", "sandbox_mode", "sqlite_home", "skills", "profiles", "active_profile"} {
		if _, ok := cfg[k]; ok {
			return nil, fmt.Errorf("native option %s is managed by the platform", k)
		}
	}
	if _, ok := cfg["hooks"]; ok && !a.TrustHooks {
		return nil, errors.New("native Hooks require explicit operator trust")
	}
	if a.Sandbox != "" && a.Sandbox != "workspace-write" && a.Sandbox != "read-only" {
		return nil, errors.New("sandbox must be workspace-write or read-only")
	}
	cfg["approval_policy"] = "never"
	cfg["sandbox_mode"] = "workspace-write"
	if a.Sandbox != "" {
		cfg["sandbox_mode"] = a.Sandbox
	}
	if a.Model != "" {
		cfg["model"] = a.Model
	}
	// Native authentication remains in auth.json. HTTPS avoids inheriting app-specific transport.
	if _, ok := cfg["model_provider"]; !ok {
		cfg["model_provider"] = "platform_http"
		providers, _ := cfg["model_providers"].(map[string]any)
		if providers == nil {
			providers = map[string]any{}
		}
		providers["platform_http"] = map[string]any{"name": "OpenAI HTTPS", "wire_api": "responses", "requires_openai_auth": true, "supports_websockets": false}
		cfg["model_providers"] = providers
	}
	cfg["skills"] = map[string]any{"bundled": map[string]any{"enabled": false}}
	if _, ok := cfg["hooks"]; ok {
		features, _ := cfg["features"].(map[string]any)
		if features == nil {
			features = map[string]any{}
		}
		features["hooks"] = true
		cfg["features"] = features
	}
	return toml.Marshal(cfg)
}
func (x *Codex) binary() string {
	if x.Binary != "" {
		return x.Binary
	}
	return "codex"
}
func (x *Codex) authHome() string {
	if x.AuthHome != "" {
		return x.AuthHome
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".codex")
}
func (x *Codex) paths(id string) (string, string) {
	root := filepath.Join(x.Root, "conversations", id)
	return filepath.Join(root, "workspace"), filepath.Join(root, "native")
}
func normalizeSkill(path string) (string, error) {
	if filepath.Base(path) == "SKILL.md" {
		path = filepath.Dir(path)
	}
	p, e := filepath.EvalSymlinks(path)
	if e != nil {
		return "", e
	}
	p, e = filepath.Abs(p)
	if e != nil {
		return "", e
	}
	st, e := os.Stat(filepath.Join(p, "SKILL.md"))
	if e != nil || !st.Mode().IsRegular() {
		return "", fmt.Errorf("Skill directory has no regular SKILL.md: %s", path)
	}
	return p, nil
}
func copySeed(src, dst string) error {
	if src == "" {
		return nil
	}
	src, e := filepath.Abs(src)
	if e != nil {
		return e
	}
	st, e := os.Stat(src)
	if e != nil || !st.IsDir() {
		return errors.New("workspace template is not a directory")
	}
	if _, e = os.Stat(filepath.Join(src, ".codex")); e == nil {
		return errors.New("template .codex configuration must be moved to the Agent native configuration before managed execution")
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(src, path)
		if e != nil {
			return e
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == ".data" || d.Name() == "node_modules" || d.Name() == "bin") {
			return filepath.SkipDir
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("template symlink is unsupported: %s", rel)
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("template contains non-regular file: %s", rel)
		}
		if info.Size() > 32*1024*1024 {
			return fmt.Errorf("template file exceeds 32 MB: %s", rel)
		}
		in, e := os.Open(path)
		if e != nil {
			return e
		}
		defer in.Close()
		out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if e != nil {
			return e
		}
		_, e = io.Copy(out, in)
		ce := out.Close()
		if e != nil {
			return e
		}
		return ce
	})
}
func (x *Codex) prepare(ctx context.Context, c Conversation) (string, string, error) {
	workspace, home := x.paths(c.ID)
	finalRoot := filepath.Dir(workspace)
	marker := filepath.Join(home, "prepared")
	if _, e := os.Stat(marker); e == nil {
		return workspace, home, nil
	}
	if _, e := os.Stat(finalRoot); e == nil {
		return "", "", errors.New("incomplete native preparation exists; inspect it before retrying")
	} else if !os.IsNotExist(e) {
		return "", "", e
	}
	parent := filepath.Dir(finalRoot)
	if e := os.MkdirAll(parent, 0700); e != nil {
		return "", "", e
	}
	staging, e := os.MkdirTemp(parent, ".prepare-"+c.ID+"-")
	if e != nil {
		return "", "", e
	}
	defer os.RemoveAll(staging)
	workspace, home = filepath.Join(staging, "workspace"), filepath.Join(staging, "native")
	marker = filepath.Join(home, "prepared")
	a := c.Snapshot
	cfg, e := nativeConfig(a)
	if e != nil {
		return "", "", e
	}
	if e = os.MkdirAll(home, 0700); e != nil {
		return "", "", e
	}
	if e = os.MkdirAll(workspace, 0700); e != nil {
		return "", "", e
	}
	if e = copySeed(a.SeedDir, workspace); e != nil {
		return "", "", e
	}
	if a.Instructions != "" {
		p := filepath.Join(workspace, "AGENTS.md")
		existing, _ := os.ReadFile(p)
		body := string(existing) + "\n\n# Agent configuration\n\n" + a.Instructions + "\n"
		if e = os.WriteFile(p, []byte(body), 0600); e != nil {
			return "", "", e
		}
	}
	auth := filepath.Join(x.authHome(), "auth.json")
	if _, e = os.Stat(auth); e != nil {
		return "", "", errors.New("native Codex login missing; run codex login on this machine")
	}
	target := filepath.Join(home, "auth.json")
	if _, e = os.Lstat(target); os.IsNotExist(e) {
		if e = os.Symlink(auth, target); e != nil {
			return "", "", e
		}
	}
	if a.Model == "" {
		var machine map[string]any
		if raw, e := os.ReadFile(filepath.Join(x.authHome(), "config.toml")); e == nil && toml.Unmarshal(raw, &machine) == nil {
			if model, ok := machine["model"].(string); ok {
				a.Model = model
				cfg, e = nativeConfig(a)
				if e != nil {
					return "", "", e
				}
			}
		}
	}
	if e = os.MkdirAll(filepath.Join(home, "skills"), 0700); e != nil {
		return "", "", e
	}
	expected := map[string]bool{}
	for i, p := range a.Skills {
		skill, e := normalizeSkill(p)
		if e != nil {
			return "", "", e
		}
		if expected[skill] {
			return "", "", errors.New("duplicate Skill directory")
		}
		expected[skill] = true
		if e = os.Symlink(skill, filepath.Join(home, "skills", fmt.Sprintf("selected-%d", i))); e != nil && !os.IsExist(e) {
			return "", "", e
		}
	}
	confPath := filepath.Join(home, "config.toml")
	if e = os.WriteFile(confPath, cfg, 0600); e != nil {
		return "", "", e
	}
	env, e := executorEnv(a, home)
	if e != nil {
		return "", "", e
	}
	skills, e := x.catalog(ctx, workspace, env)
	if e != nil {
		return "", "", e
	}
	seen := map[string]bool{}
	for _, s := range skills {
		p, e := normalizeSkill(s.Path)
		if e != nil {
			return "", "", e
		}
		seen[p] = true
		cfg = append(cfg, []byte(fmt.Sprintf("\n[[skills.config]]\npath = %s\nenabled = %t\n", quote(p), expected[p]))...)
	}
	for p := range expected {
		if !seen[p] {
			return "", "", fmt.Errorf("configured Skill was not discovered: %s", p)
		}
	}
	if e = os.WriteFile(confPath, cfg, 0600); e != nil {
		return "", "", e
	}
	actual, e := x.catalog(ctx, workspace, env)
	if e != nil {
		return "", "", e
	}
	enabled := map[string]bool{}
	for _, s := range actual {
		if s.Enabled {
			p, e := normalizeSkill(s.Path)
			if e != nil {
				return "", "", e
			}
			enabled[p] = true
		}
	}
	if len(enabled) != len(expected) {
		return "", "", errors.New("native Skill scope did not match configuration")
	}
	for p := range expected {
		if !enabled[p] {
			return "", "", fmt.Errorf("native Skill disabled unexpectedly: %s", p)
		}
	}
	metadata, _ := json.MarshalIndent(actual, "", "  ")
	if e = os.WriteFile(filepath.Join(home, "skills.json"), metadata, 0600); e != nil {
		return "", "", e
	}
	// Discovery used staging paths. Preserve disabled project Skills after publication.
	stagedPrefix, finalPrefix := quote(staging), quote(finalRoot)
	cfg = []byte(strings.ReplaceAll(string(cfg), stagedPrefix[1:len(stagedPrefix)-1], finalPrefix[1:len(finalPrefix)-1]))
	if e = os.WriteFile(confPath, cfg, 0600); e != nil {
		return "", "", e
	}
	if e = os.WriteFile(marker, []byte(now()), 0600); e != nil {
		return "", "", e
	}
	if e = os.Rename(staging, finalRoot); e != nil {
		return "", "", e
	}
	workspace, home = x.paths(c.ID)
	return workspace, home, nil
}
func quote(s string) string { b, _ := json.Marshal(s); return string(b) }

type nativeSkill struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

func (x *Codex) catalog(ctx context.Context, dir string, env []string) ([]nativeSkill, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, x.binary(), "app-server", "--strict-config", "--stdio")
	cmd.Dir = dir
	cmd.Env = env
	in, e := cmd.StdinPipe()
	if e != nil {
		return nil, e
	}
	out, e := cmd.StdoutPipe()
	if e != nil {
		return nil, e
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if e = cmd.Start(); e != nil {
		return nil, e
	}
	defer func() { in.Close(); cmd.Process.Kill(); cmd.Wait() }()
	reader := bufio.NewScanner(out)
	reader.Buffer(make([]byte, 4096), 2*1024*1024)
	request := func(id int, method string, params any) (json.RawMessage, error) {
		raw, _ := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
		if _, e := in.Write(append(raw, '\n')); e != nil {
			return nil, e
		}
		for reader.Scan() {
			var reply struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if e = json.Unmarshal(reader.Bytes(), &reply); e != nil {
				return nil, e
			}
			if reply.ID != id {
				continue
			}
			if len(reply.Error) > 0 {
				return nil, fmt.Errorf("native configuration rejected: %s", reply.Error)
			}
			return reply.Result, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("native discovery ended: %s", stderr.String())
	}
	if _, e = request(1, "initialize", map[string]any{"clientInfo": map[string]string{"name": "agent_platform", "version": "0.1.0"}}); e != nil {
		return nil, e
	}
	raw, e := request(2, "skills/list", map[string]any{"cwds": []string{dir}, "forceReload": true})
	if e != nil {
		return nil, e
	}
	var result struct {
		Data []struct {
			Skills []nativeSkill     `json:"skills"`
			Errors []json.RawMessage `json:"errors"`
		} `json:"data"`
	}
	if e = json.Unmarshal(raw, &result); e != nil {
		return nil, e
	}
	if len(result.Data) != 1 || len(result.Data[0].Errors) > 0 {
		return nil, errors.New("native Skill discovery returned errors")
	}
	return result.Data[0].Skills, nil
}
func (x *Codex) Check(ctx context.Context, a Agent) (CheckResult, error) {
	if a.Executor != "codex" {
		return CheckResult{}, errors.New("only Codex is implemented")
	}
	dir, e := os.MkdirTemp(x.Root, "check-")
	if e != nil {
		return CheckResult{}, e
	}
	defer os.RemoveAll(dir)
	probe := *x
	probe.Root = dir
	c := Conversation{ID: "probe", Snapshot: a}
	workspace, home, e := probe.prepare(ctx, c)
	if e != nil {
		return CheckResult{}, e
	}
	env, e := executorEnv(a, home)
	if e != nil {
		return CheckResult{}, e
	}
	cmd := exec.CommandContext(ctx, x.binary(), "--version")
	cmd.Dir = workspace
	cmd.Env = env
	v, e := cmd.Output()
	if e != nil {
		return CheckResult{}, e
	}
	cmd = exec.CommandContext(ctx, x.binary(), "login", "status")
	cmd.Env = env
	if b, e := cmd.CombinedOutput(); e != nil {
		return CheckResult{}, fmt.Errorf("native login unavailable: %s", b)
	}
	return CheckResult{Ready: true, Version: strings.TrimSpace(string(v)), Skills: a.Skills}, nil
}
func nativeRecord(home, id string) bool {
	found := false
	filepath.WalkDir(filepath.Join(home, "sessions"), func(p string, d fs.DirEntry, e error) error {
		if e == nil && !d.IsDir() && strings.Contains(d.Name(), id) && strings.HasSuffix(p, ".jsonl") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}
func (x *Codex) Execute(ctx context.Context, c Conversation, m Message, onEvent func([]byte) error) error {
	workspace, home := x.paths(c.ID)
	if c.ThreadID != "" && !nativeRecord(home, c.ThreadID) {
		return errors.New("native session record is missing; no fresh session was created")
	}
	var e error
	workspace, home, e = x.prepare(ctx, c)
	if e != nil {
		return e
	}
	env, e := executorEnv(c.Snapshot, home)
	if e != nil {
		return e
	}
	args := []string{"exec"}
	if c.ThreadID != "" {
		args = append(args, "resume", c.ThreadID)
	}
	args = append(args, "--strict-config", "--json", "--skip-git-repo-check", "--ignore-rules")
	if c.Snapshot.TrustHooks {
		args = append(args, "--dangerously-bypass-hook-trust")
	}
	args = append(args, "-")
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	nonce := "agent-platform-run-" + newID()
	// Hold the native prompt until its supervised group is durably registered.
	supervisor := `IFS= read -r start || exit 1; [ "$start" = "$0" ] || exit 1; "$@" <&0 & child=$!; wait "$child"`
	supervisorArgs := append([]string{"-c", supervisor, nonce, x.binary()}, args...)
	cmd := exec.CommandContext(runCtx, "/bin/sh", supervisorArgs...)
	cmd.Dir = workspace
	cmd.Env = env
	input, e := cmd.StdinPipe()
	if e != nil {
		return e
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 3 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		}
		return nil
	}
	out, e := cmd.StdoutPipe()
	if e != nil {
		return e
	}
	errpipe, e := cmd.StderrPipe()
	if e != nil {
		return e
	}
	if e = cmd.Start(); e != nil {
		return e
	}
	executionDone := make(chan struct{})
	defer close(executionDone)
	go func() {
		select {
		case <-executionDone:
			return
		case <-runCtx.Done():
		}
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		select {
		case <-executionDone:
			return
		case <-timer.C:
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	if e = registerProcess(home, cmd.Process.Pid, nonce); e != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		input.Close()
		cmd.Wait()
		return e
	}
	defer os.Remove(filepath.Join(home, "process.json"))
	go func() { defer input.Close(); io.WriteString(input, nonce+"\n"+m.Content) }()
	redact := x.redactor(c.Snapshot)
	var mu sync.Mutex
	var callbackErr error
	var lastError string
	var stderrText strings.Builder
	var seenThread, completed bool
	emit := func(raw []byte) {
		mu.Lock()
		defer mu.Unlock()
		if callbackErr == nil {
			callbackErr = onEvent(redactJSON(raw, redact))
		}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		scan := bufio.NewScanner(errpipe)
		scan.Buffer(make([]byte, 4096), 1024*1024)
		for scan.Scan() {
			text := redact(scan.Text())
			mu.Lock()
			if stderrText.Len() < 16384 {
				stderrText.WriteString(text + "\n")
			}
			mu.Unlock()
			b, _ := json.Marshal(map[string]any{"type": "executor.stderr", "text": text})
			emit(b)
		}
	}()
	scan := bufio.NewScanner(out)
	scan.Buffer(make([]byte, 4096), 4*1024*1024)
	for scan.Scan() {
		raw := append([]byte(nil), scan.Bytes()...)
		var event struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
			Message  string `json:"message"`
			Error    struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if e = json.Unmarshal(raw, &event); e != nil {
			mu.Lock()
			callbackErr = fmt.Errorf("invalid native event: %w", e)
			mu.Unlock()
			break
		}
		if event.Type == "thread.started" {
			if c.ThreadID != "" && event.ThreadID != c.ThreadID {
				mu.Lock()
				callbackErr = errors.New("native session identity changed during resume")
				mu.Unlock()
				break
			}
			seenThread = true
		}
		if event.Type == "turn.completed" {
			completed = true
		}
		if event.Type == "turn.failed" {
			lastError = event.Error.Message
		}
		if event.Type == "error" {
			lastError = event.Message
		}
		emit(raw)
		mu.Lock()
		bad := callbackErr != nil
		mu.Unlock()
		if bad {
			break
		}
	}
	if scan.Err() != nil {
		mu.Lock()
		callbackErr = scan.Err()
		mu.Unlock()
	}
	mu.Lock()
	bad := callbackErr != nil
	mu.Unlock()
	if bad {
		cancelRun()
	}
	waitErr := cmd.Wait()
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	<-done
	if ctx.Err() != nil {
		return ctx.Err()
	}
	mu.Lock()
	defer mu.Unlock()
	if callbackErr != nil {
		return callbackErr
	}
	if waitErr != nil || !seenThread || !completed {
		if lastError == "" {
			lastError = strings.TrimSpace(stderrText.String())
		}
		if len(lastError) > 4000 {
			lastError = lastError[:4000]
		}
		return fmt.Errorf("Codex execution did not complete: %v %s", waitErr, redact(lastError))
	}
	return nil
}
