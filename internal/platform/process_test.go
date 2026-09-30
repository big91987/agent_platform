package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCancelKillsTermIgnoringDescendants(t *testing.T) {
	root := t.TempDir()
	x := Codex{Root: root, Binary: filepath.Join(root, "executor")}
	workspace, home := x.paths("cancel")
	os.MkdirAll(workspace, 0700)
	os.MkdirAll(home, 0700)
	os.WriteFile(filepath.Join(home, "prepared"), []byte("ready"), 0600)
	pidfile := filepath.Join(root, "pid")
	os.WriteFile(x.Binary, []byte("#!/bin/sh\ntrap '' TERM\necho $$ > "+pidfile+"\nsleep 120 &\nprintf '%s\\n' '{\"type\":\"thread.started\",\"thread_id\":\"proof\"}'\nwait\n"), 0700)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- x.Execute(ctx, Conversation{ID: "cancel", Snapshot: Agent{Executor: "codex"}}, Message{Content: "go"}, func(b []byte) error {
			select {
			case started <- struct{}{}:
			default:
			}
			return nil
		})
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("not started")
	}
	raw, _ := os.ReadFile(pidfile)
	pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	defer syscall.Kill(-pid, syscall.SIGKILL)
	cancel()
	select {
	case <-done:
	case <-time.After(6 * time.Second):
		syscall.Kill(-pid, syscall.SIGKILL)
		<-done
		t.Fatal("Execute stayed blocked >6s after cancellation: TERM-ignoring descendant kept stdout open past 3s WaitDelay")
	}
}

func TestCrashHelper(t *testing.T) {
	root := os.Getenv("PLATFORM_TEST_CRASH_ROOT")
	if root == "" {
		return
	}
	s, e := OpenStore(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a := testAgent(t, s)
	r, e := s.Submit(Caller{Admin: true, Source: "console"}, Input{AgentID: a.ID, UserID: "u", Message: "run"})
	if e != nil {
		t.Fatal(e)
	}
	c, m, e := s.Claim()
	if e != nil {
		t.Fatal(e)
	}
	x := Codex{Root: root, Binary: filepath.Join(root, "executor")}
	w, h := x.paths(c.ID)
	os.MkdirAll(w, 0700)
	os.MkdirAll(h, 0700)
	os.WriteFile(filepath.Join(h, "prepared"), []byte("ready"), 0600)
	os.MkdirAll(filepath.Join(h, "sessions"), 0700)
	os.WriteFile(filepath.Join(h, "sessions", "crash-proof.jsonl"), []byte("native record"), 0600)
	os.WriteFile(filepath.Join(root, "conversation"), []byte(r.ConversationID), 0600)
	script := "#!/bin/sh\necho $$ > " + filepath.Join(root, "native-pid") + "\nprintf '%s\\n' '{\"type\":\"thread.started\",\"thread_id\":\"crash-proof\"}'\nsleep 120 &\nwait\n"
	os.WriteFile(x.Binary, []byte(script), 0700)
	x.Execute(context.Background(), c, m, func(b []byte) error {
		s.SetThread(c.ID, "crash-proof")
		return os.WriteFile(filepath.Join(root, "ready"), []byte("yes"), 0600)
	})
}
func TestCrashReconcilesNativeExecutionBeforeContinuing(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashHelper$")
	cmd.Env = append(os.Environ(), "PLATFORM_TEST_CRASH_ROOT="+root)
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer cmd.Process.Kill()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, e := os.Stat(filepath.Join(root, "ready")); e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper not ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "native-pid"))
	pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	defer syscall.Kill(-pid, syscall.SIGKILL)
	cmd.Process.Kill()
	cmd.Wait()
	s, e := OpenStore(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	b, _ := os.ReadFile(filepath.Join(root, "conversation"))
	id := string(b)
	c, e := s.Conversation(id)
	if e != nil || c.Status != "failed" {
		t.Fatalf("restart state: %+v %v", c, e)
	}
	s.Submit(Caller{Admin: true, Source: "console"}, Input{ConversationID: id, UserID: "u", Message: "continue work"})
	s.Continue(id)
	if _, _, e = s.Claim(); e != nil {
		t.Fatal(e)
	}
	if e = syscall.Kill(pid, 0); e == nil {
		t.Fatal("restart admitted next turn while previous native process survived platform crash")
	}
}
