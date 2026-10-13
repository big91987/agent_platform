package platform

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSandboxRoleAssetsContainOnlySelectedPrivateInputs(t *testing.T) {
	auth := t.TempDir()
	skill := t.TempDir()
	if err := os.WriteFile(filepath.Join(auth, "auth.json"), []byte(`{"tokens":{"access_token":"private-fixture-token"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(auth, "unselected-secret"), []byte("must not transfer"), 0600); err != nil {
		t.Fatal(err)
	}
	os.Mkdir(filepath.Join(skill, "scripts"), 0700)
	os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: selected-role\ndescription: role fixture\n---\nKeep project evidence.\n"), 0600)
	os.WriteFile(filepath.Join(skill, "scripts", "run.py"), []byte("print('selected asset')\n"), 0700)
	executor := Codex{AuthHome: auth}
	c := Conversation{ID: newID(), Snapshot: Agent{Executor: "codex", Model: "gpt-6.1-sol", Skills: []string{skill}, Instructions: "Selected role", Sandbox: "workspace-write"}}
	files, selected, env, err := executor.sandboxRoleInputs(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || !strings.HasSuffix(selected[0], "/skills/selected-0/SKILL.md") || len(files) != 4 || len(env) != 0 {
		t.Fatal("role assets missing or host environment leaked", len(files), selected, env)
	}
	for _, file := range files {
		if !strings.HasPrefix(file.Path, "/var/lib/agent-platform/roles/"+c.ID+"/") || bytes.Contains(file.Content, []byte("must not transfer")) {
			t.Fatal("role package escaped selected inputs", file.Path)
		}
		if file.Path == selected[0] && file.Mode != 0600 {
			t.Fatal("Skill source is not private")
		}
		if strings.HasSuffix(file.Path, "scripts/run.py") && file.Mode != 0700 {
			t.Fatal("selected executable lost its mode")
		}
	}
	other := c
	other.ID = newID()
	otherFiles, _, _, err := executor.sandboxRoleInputs(other)
	if err != nil {
		t.Fatal(err)
	}
	if otherFiles[0].Path == files[0].Path {
		t.Fatal("independent role reused private home")
	}
}

func TestSandboxRoleAssetsRejectUnsafeCredentialsAndSkillLinks(t *testing.T) {
	auth := t.TempDir()
	skill := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(auth, "auth.json"), []byte(`{"tokens":{"access_token":"fixture-secret"}}`), 0600)
	os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("Role\n"), 0600)
	os.WriteFile(filepath.Join(outside, "secret"), []byte("outside secret"), 0600)
	os.Symlink(filepath.Join(outside, "secret"), filepath.Join(skill, "outside-link"))
	executor := Codex{AuthHome: auth}
	c := Conversation{ID: newID(), Snapshot: Agent{Executor: "codex", Model: "gpt-6.1-sol", Skills: []string{skill}}}
	if _, _, _, err := executor.sandboxRoleInputs(c); err == nil {
		t.Fatal("selected Skill link escaped its source")
	}
	c.Snapshot.Skills = nil
	os.Chmod(filepath.Join(auth, "auth.json"), 0644)
	if _, _, _, err := executor.sandboxRoleInputs(c); err == nil {
		t.Fatal("shared-readable native credential transferred")
	}
	os.Chmod(filepath.Join(auth, "auth.json"), 0600)
	c.Snapshot.InheritEnv = true
	if _, _, _, err := executor.sandboxRoleInputs(c); err == nil {
		t.Fatal("whole host environment copied into role")
	}
	c.Snapshot.InheritEnv = false
	c.Snapshot.ResolvedTools = map[string]ResolvedToolServer{"host-program": {Connection: MCPConnection{Command: "/host/private/program"}}}
	if _, _, _, err := executor.sandboxRoleInputs(c); err == nil {
		t.Fatal("host stdio program silently treated as a sandbox tool")
	}
}
