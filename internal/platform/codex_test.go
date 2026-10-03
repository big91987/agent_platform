package platform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvironmentSupportsInheritanceOverrideAndClear(t *testing.T) {
	t.Setenv("AP_TEST_KEEP", "old")
	t.Setenv("AP_TEST_REMOVE", "secret")
	t.Setenv("AGENT_PLATFORM_PASSWORD", "hidden")
	value := "new"
	a := Agent{InheritEnv: true, Env: map[string]*string{"AP_TEST_KEEP": &value, "AP_TEST_REMOVE": nil}}
	env, e := executorEnv(a, "/native")
	if e != nil {
		t.Fatal(e)
	}
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "AP_TEST_KEEP=new") || strings.Contains(joined, "AP_TEST_REMOVE=") || strings.Contains(joined, "AGENT_PLATFORM_PASSWORD=") {
		t.Fatalf("incorrect environment: %v", env)
	}
	a.Env["CODEX_HOME"] = &value
	if _, e = executorEnv(a, "/native"); e == nil {
		t.Fatal("managed home overridden")
	}
}
func TestNativeConfigurationRejectsManagedStateAndUntrustedHooks(t *testing.T) {
	for _, raw := range []string{"sandbox_mode='danger-full-access'", "sqlite_home='/tmp/shared'", "[[hooks.Stop]]\n[[hooks.Stop.hooks]]\ntype='command'\ncommand='echo unsafe'"} {
		if _, e := nativeConfig(Agent{NativeConfig: raw}); e == nil {
			t.Fatalf("unsafe config accepted: %s", raw)
		}
	}
	a := Agent{TrustHooks: true, NativeConfig: "[[hooks.Stop]]\n[[hooks.Stop.hooks]]\ntype='command'\ncommand='echo {}'", Sandbox: "workspace-write"}
	cfg, e := nativeConfig(a)
	if e != nil || !strings.Contains(string(cfg), "hooks") {
		t.Fatalf("trusted native hooks lost: %s %v", cfg, e)
	}
}
func TestMissingNativeRecordCannotBecomeFreshSession(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "codex")
	os.WriteFile(exe, []byte("#!/bin/sh\nprintf invoked > marker\n"), 0700)
	x := Codex{Root: dir, Binary: exe}
	c := Conversation{ID: "abc", ThreadID: "missing-thread", Snapshot: Agent{Executor: "codex"}}
	if e := x.Execute(context.Background(), c, Message{Content: "continue"}, func([]byte) error { return nil }); e == nil || !strings.Contains(e.Error(), "native session record") {
		t.Fatalf("missing session was not identified: %v", e)
	}
	if _, e := os.Stat(filepath.Join(dir, "marker")); !os.IsNotExist(e) {
		t.Fatal("fresh executor was started")
	}
}

func TestFailedPreparationLeavesNoPartialWorkspace(t *testing.T) {
	root := t.TempDir()
	seed := filepath.Join(root, "seed")
	os.MkdirAll(seed, 0700)
	os.WriteFile(filepath.Join(seed, "a.txt"), []byte("seed"), 0600)
	os.Symlink("a.txt", filepath.Join(seed, "z-link"))
	x := Codex{Root: filepath.Join(root, "runtime")}
	c := Conversation{ID: "retry", Snapshot: Agent{Executor: "codex", SeedDir: seed}}
	if _, _, err := x.prepare(context.Background(), c); err == nil {
		t.Fatal("invalid template was accepted")
	}
	workspace, _ := x.paths(c.ID)
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatal("failed preparation left a workspace that poisons the next attempt")
	}
}

func TestNativeStreamProtectsCredentialsAndReportsMalformedOutput(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		t.Run(fmt.Sprint(malformed), func(t *testing.T) {
			root := t.TempDir()
			auth := filepath.Join(root, "auth")
			os.MkdirAll(auth, 0700)
			const secret = "native-token-that-must-not-be-exported"
			os.WriteFile(filepath.Join(auth, "auth.json"), []byte(`{"tokens":{"access_token":"`+secret+`"}}`), 0600)
			x := Codex{Root: root, AuthHome: auth, Binary: filepath.Join(root, "executor")}
			workspace, home := x.paths("stream")
			os.MkdirAll(workspace, 0700)
			os.MkdirAll(home, 0700)
			os.WriteFile(filepath.Join(home, "prepared"), []byte("ready"), 0600)
			script := fmt.Sprintf(`#!/usr/bin/env python3
import json, sys
secret = %q
for line in sys.stdin:
 p=json.loads(line)
 if p.get('method')=='initialize': print(json.dumps({'id':1,'result':{}}),flush=True)
 if p.get('method')=='thread/start': print(json.dumps({'id':2,'result':{'thread':{'id':'native-test'}}}),flush=True)
 if p.get('method')=='turn/start':
  print(json.dumps({'method':'item/agentMessage/delta','params':{'itemId':'m','delta':secret}}),flush=True)
  if %s:
   print('not-json',flush=True)
  else:
   print(json.dumps({'method':'item/completed','params':{'item':{'id':'m','type':'agentMessage','text':secret}}}),flush=True)
   print(json.dumps({'method':'turn/completed','params':{'turn':{'status':'completed'}}}),flush=True)
  break
`, secret, map[bool]string{true: "True", false: "False"}[malformed])
			os.WriteFile(x.Binary, []byte(script), 0700)
			var output strings.Builder
			err := x.Execute(context.Background(), Conversation{ID: "stream", Snapshot: Agent{Executor: "codex"}}, Message{Content: "run"}, func(b []byte) error { output.Write(b); return nil })
			if strings.Contains(output.String(), secret) {
				t.Fatal("native credential leaked into public events")
			}
			if malformed && (err == nil || !strings.Contains(err.Error(), "invalid app-server frame")) {
				t.Fatalf("malformed stream: %v", err)
			}
			if !malformed && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExternalWorkspaceUsesCWDWithoutRewritingProject(t *testing.T) {
	root := t.TempDir()
	workspace, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projectRules := []byte("# Project rules\nKeep the existing contract.\n")
	if err = os.WriteFile(filepath.Join(workspace, "AGENTS.md"), projectRules, 0600); err != nil {
		t.Fatal(err)
	}
	auth := filepath.Join(root, "auth")
	os.MkdirAll(auth, 0700)
	os.WriteFile(filepath.Join(auth, "auth.json"), []byte(`{}`), 0600)
	os.WriteFile(filepath.Join(auth, "config.toml"), []byte(`model="test-model"`), 0600)
	x := Codex{Root: root, AuthHome: auth, Binary: filepath.Join(root, "executor")}
	script := `#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
for line in sys.stdin:
 p=json.loads(line); method=p.get('method')
 if method=='initialize': print(json.dumps({'id':1,'result':{}}),flush=True)
 if method=='skills/list': print(json.dumps({'id':2,'result':{'data':[{'skills':[],'errors':[]}]}}),flush=True)
 if method in ('thread/start','thread/resume'):
  Path(os.environ['CODEX_HOME'],'cwd.txt').write_text(os.getcwd())
  print(json.dumps({'id':2,'result':{'thread':{'id':'workspace-proof'}}}),flush=True)
 if method=='turn/start':
  print(json.dumps({'method':'turn/completed','params':{'turn':{'status':'completed'}}}),flush=True)
  break
`
	os.WriteFile(x.Binary, []byte(script), 0700)
	c := Conversation{ID: "external", WorkspacePath: workspace, Snapshot: Agent{Executor: "codex", Instructions: "Use project documents before asking questions.", NativeConfig: "developer_instructions='Existing native guidance'", SeedDir: "/missing-template"}}
	if err = x.Execute(context.Background(), c, Message{Content: "Inspect this project"}, func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	_, home := x.paths(c.ID)
	cwd, _ := os.ReadFile(filepath.Join(home, "cwd.txt"))
	if string(cwd) != workspace {
		t.Fatalf("wrong native CWD: %s", cwd)
	}
	config, _ := os.ReadFile(filepath.Join(home, "config.toml"))
	for _, text := range []string{"Existing native guidance", c.Snapshot.Instructions, "test-model"} {
		if !strings.Contains(string(config), text) {
			t.Fatalf("native configuration lost guidance/model: %s", config)
		}
	}
	rules, _ := os.ReadFile(filepath.Join(workspace, "AGENTS.md"))
	if string(rules) != string(projectRules) {
		t.Fatal("project instructions were overwritten")
	}
	os.MkdirAll(filepath.Join(home, "sessions"), 0700)
	os.WriteFile(filepath.Join(home, "sessions", "workspace-proof.jsonl"), []byte("mock native record"), 0600)
	c.ThreadID = "workspace-proof"
	if err = x.Execute(context.Background(), c, Message{Content: "Continue"}, func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	cwd, _ = os.ReadFile(filepath.Join(home, "cwd.txt"))
	if string(cwd) != workspace {
		t.Fatal("resume changed workspace")
	}
	if err = os.Rename(workspace, workspace+"-moved"); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workspace + "-moved")
	if err = x.Execute(context.Background(), c, Message{Content: "Continue"}, func([]byte) error { return nil }); err == nil {
		t.Fatal("missing external workspace silently became empty")
	}
}
