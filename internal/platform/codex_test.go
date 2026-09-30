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
			script := "#!/bin/sh\nprintf '%s\\n' '{\"type\":\"thread.started\",\"thread_id\":\"native-test\"}'\nprintf '%s\\n' '{\"type\":\"item.completed\",\"item\":{\"id\":\"message\",\"type\":\"agent_message\",\"text\":\"" + secret + "\"}}'\n"
			if malformed {
				script += "(i=0; while [ $i -lt 300 ]; do echo diagnostic >&2; i=$((i+1)); done) &\nprintf '%s\\n' not-json\nwait\n"
			} else {
				script += "echo '" + secret + "' >&2\nprintf '%s\\n' '{\"type\":\"turn.completed\"}'\n"
			}
			os.WriteFile(x.Binary, []byte(script), 0700)
			var output strings.Builder
			err := x.Execute(context.Background(), Conversation{ID: "stream", Snapshot: Agent{Executor: "codex"}}, Message{Content: "run"}, func(b []byte) error { output.Write(b); return nil })
			if strings.Contains(output.String(), secret) {
				t.Fatal("native credential leaked into public events")
			}
			if malformed && (err == nil || !strings.Contains(err.Error(), "invalid native event")) {
				t.Fatalf("malformed stream: %v", err)
			}
			if !malformed && err != nil {
				t.Fatal(err)
			}
		})
	}
}
