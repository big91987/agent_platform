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
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Codex struct {
	Root, Binary, AuthHome string
	Approve                func(context.Context, Conversation, Message, json.RawMessage) (string, error)
}
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
	for _, server := range a.ResolvedTools {
		names := append([]string{}, server.Connection.EnvVars...)
		if server.Connection.BearerTokenEnvVar != "" {
			names = append(names, server.Connection.BearerTokenEnvVar)
		}
		for _, name := range names {
			if _, overridden := a.Env[name]; overridden {
				continue
			}
			if name == "HOME" || strings.HasPrefix(name, "CODEX_") || strings.HasPrefix(name, "AGENT_PLATFORM_") {
				return nil, fmt.Errorf("MCP environment key is reserved: %s", name)
			}
			if value, ok := os.LookupEnv(name); ok {
				values[name] = value
			} else {
				return nil, fmt.Errorf("MCP environment variable is missing: %s", name)
			}
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
	for _, k := range []string{"approval_policy", "sandbox_mode", "sqlite_home", "skills", "profiles", "active_profile", "sandbox_workspace_write", "permissions"} {
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
	cfg["approval_policy"] = nativeApprovalPolicy(a)
	cfg["sandbox_workspace_write"] = map[string]any{"network_access": a.NetworkAccess}
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
	if len(a.ResolvedTools) > 0 {
		servers, _ := cfg["mcp_servers"].(map[string]any)
		if servers == nil {
			servers = map[string]any{}
		}
		for name, server := range a.ResolvedTools {
			key := "registered_" + name
			if _, exists := servers[key]; exists {
				return nil, fmt.Errorf("MCP server %s is configured twice", key)
			}
			servers[key] = server.native()
		}
		cfg["mcp_servers"] = servers
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
	if c.WorkspacePath != "" {
		resolved, err := normalizeWorkspace(c.WorkspacePath)
		if err != nil {
			return "", "", err
		}
		if resolved != c.WorkspacePath {
			return "", "", errors.New("workspace path changed; refusing to resume in another directory")
		}
		workspace = resolved
	}
	marker := filepath.Join(home, "prepared")
	if _, e := os.Stat(marker); e == nil {
		return workspace, home, nil
	}
	if _, e := os.Stat(finalRoot); e == nil {
		return "", "", errors.New("incomplete native preparation exists; inspect it before retrying")
	} else if !os.IsNotExist(e) {
		return "", "", e
	}
	toolWorkspace := workspace
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
	if c.WorkspacePath != "" {
		workspace = c.WorkspacePath
	}
	marker = filepath.Join(home, "prepared")
	a := bindToolWorkspace(c.Snapshot, toolWorkspace)
	cfg, e := nativeConfig(a)
	if e != nil {
		return "", "", e
	}
	if c.WorkspacePath != "" && a.Instructions != "" {
		// Keep repository instructions intact; Agent configuration stays in the native home.
		var options map[string]any
		if e = toml.Unmarshal(cfg, &options); e != nil {
			return "", "", e
		}
		previous, _ := options["developer_instructions"].(string)
		options["developer_instructions"] = strings.TrimSpace(previous + "\n\n" + a.Instructions)
		if cfg, e = toml.Marshal(options); e != nil {
			return "", "", e
		}
	}
	if e = os.MkdirAll(home, 0700); e != nil {
		return "", "", e
	}
	if c.WorkspacePath == "" {
		if e = os.MkdirAll(workspace, 0700); e != nil {
			return "", "", e
		}
		if e = copySeed(a.SeedDir, workspace); e != nil {
			return "", "", e
		}
	}
	if c.WorkspacePath == "" && a.Instructions != "" {
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
				var options map[string]any
				e = toml.Unmarshal(cfg, &options)
				if e == nil {
					options["model"] = model
					cfg, e = toml.Marshal(options)
				}
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
		// Native config matches the discovered SKILL.md filename. Normalization
		// is only for comparing administrator-selected identities; resolving the
		// path here would leave project/symlink Skills enabled unexpectedly.
		configPath := s.Path
		if filepath.Base(configPath) != "SKILL.md" {
			configPath = filepath.Join(configPath, "SKILL.md")
		}
		cfg = append(cfg, []byte(fmt.Sprintf("\n[[skills.config]]\npath = %s\nenabled = %t\n", quote(configPath), expected[p]))...)
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
	_, home = x.paths(c.ID)
	workspace = c.workspace(x.Root)
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
	return x.executeAppServer(ctx, c, m, workspace, home, env, onEvent)
}
