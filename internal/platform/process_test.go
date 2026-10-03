package platform

import (
	"context"
	"encoding/json"
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
	os.WriteFile(x.Binary, []byte("#!/bin/sh\ntrap '' TERM\necho $$ > "+pidfile+"\nsleep 120 &\nprintf '%s\\n' '{\"method\":\"turn/started\",\"params\":{}}'\nwait\n"), 0700)
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
	script := "#!/bin/sh\necho $$ > " + filepath.Join(root, "native-pid") + "\nprintf '%s\\n' '{\"method\":\"turn/started\",\"params\":{}}'\nsleep 120 &\nwait\n"
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

func TestStopDuringNativeToolConfirmation(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	home := filepath.Join(root, "native")
	os.MkdirAll(workspace, 0700)
	os.MkdirAll(home, 0700)
	binary := filepath.Join(root, "interactive")
	// A protocol peer that leaves a TERM-ignoring child behind until the group is killed.
	script := `#!/usr/bin/env python3
import json, signal, subprocess, sys
signal.signal(signal.SIGTERM, signal.SIG_IGN)
subprocess.Popen(['sleep','120'])
for line in sys.stdin:
 p=json.loads(line)
 if p.get('method')=='initialize': print(json.dumps({'id':1,'result':{}}),flush=True)
 if p.get('method')=='thread/start': print(json.dumps({'id':2,'result':{'thread':{'id':'test-thread'}}}),flush=True)
 if p.get('method')=='turn/start':
  print(json.dumps({'id':3,'result':{}}),flush=True)
  print(json.dumps({'id':99,'method':'mcpServer/elicitation/request','params':{'threadId':'test-thread','mode':'form','_meta':{'codex_approval_kind':'mcp_tool_call'}}}),flush=True)
`
	os.WriteFile(binary, []byte(script), 0700)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pending := make(chan struct{})
	done := make(chan error, 1)
	x := Codex{Root: root, Binary: binary, Approve: func(ctx context.Context, c Conversation, m Message, raw json.RawMessage) (string, error) {
		close(pending)
		<-ctx.Done()
		return "", ctx.Err()
	}}
	go func() {
		done <- x.executeAppServer(ctx, Conversation{Snapshot: Agent{Executor: "codex"}}, Message{Content: "test"}, workspace, home, os.Environ(), func([]byte) error { return nil })
	}()
	select {
	case <-pending:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("no confirmation request")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("stopped turn marked successful")
		}
	case <-time.After(7 * time.Second):
		t.Fatal("native confirmation did not cancel")
	}
	if _, err := os.Stat(filepath.Join(home, "process.json")); !os.IsNotExist(err) {
		t.Fatalf("process guard not removed after stop: %v", err)
	}
}
