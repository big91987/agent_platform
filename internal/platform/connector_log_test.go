package platform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestConnectorLogPreservesStartAndFailureTail(t *testing.T) {
	input := "source-sha: checked-build\n" + strings.Repeat("旧版预期失败，不是本次失败\n", 3000) + "\nactual failure: browser assertion failed\n"
	for _, size := range []int{1, 37, len(input)} {
		t.Run(fmt.Sprintf("chunk-%d", size), func(t *testing.T) {
			log := &boundedConnectorLog{}
			for start := 0; start < len(input); start += size {
				end := min(start+size, len(input))
				if n, err := log.Write([]byte(input[start:end])); err != nil || n != end-start {
					t.Fatalf("write = %d, %v", n, err)
				}
			}
			out := log.String()
			if !strings.HasPrefix(out, "source-sha: checked-build\n") || !strings.HasSuffix(out, "actual failure: browser assertion failed\n") {
				t.Fatal("receipt lost source identity or actual final failure")
			}
			if len(out) > 25000 || !utf8.ValidString(out) {
				t.Fatal("receipt is unbounded or contains a split UTF-8 character")
			}
			if !strings.Contains(out, "omitted") {
				t.Fatal("omission is not disclosed")
			}
		})
	}
}

func TestConnectorLogSmallOutputIsUnchanged(t *testing.T) {
	log := &boundedConnectorLog{}
	for _, part := range []string{"开始\n", "done\n"} {
		_, _ = log.Write([]byte(part))
	}
	if got := log.String(); got != "开始\ndone\n" {
		t.Fatalf("small output changed: %q", got)
	}
}

func TestConnectorCommandLargeReceiptRedactsBeforeTruncation(t *testing.T) {
	secret := "credential-prefix-SENSITIVE-credential-suffix"
	t.Setenv("CONNECTOR_LOG_TEST_SECRET", secret)
	workspace := t.TempDir()
	// Secrets straddle both retained-window boundaries. The end of the command,
	// not the first noisy test, matters. Cross-write matches are tested below.
	payload := "source-sha: checked-build\n" + strings.Repeat("x", 11960) + secret + strings.Repeat("y", 70000) + secret + strings.Repeat("z", 11940) + "\nactual failure: browser assertion failed\n"
	file := filepath.Join(workspace, "output.txt")
	if err := os.WriteFile(file, []byte(payload), 0600); err != nil {
		t.Fatal(err)
	}
	v := Connector{Name: "large output", Kind: "command", Enabled: true, WorkspaceRoot: workspace, Executable: "/bin/sh", Args: []string{"-c", `cat "$1"; printf 'stderr terminal error\n' >&2; exit 23`, "test", file}, EnvRefs: map[string]string{"TEST_SECRET": "CONNECTOR_LOG_TEST_SECRET"}, TimeoutSeconds: 5}
	receipt, err := executeConnectorCommand(context.Background(), v, WorkflowRun{ID: "log-test", WorkspacePath: workspace}, WorkflowStep{Seq: 1}, connectorRequest{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ExitCode == nil || *receipt.ExitCode != 23 {
		t.Fatalf("lost actual exit code: %+v", receipt.ExitCode)
	}
	if !strings.Contains(receipt.Output, "source-sha: checked-build") || !strings.Contains(receipt.Output, "actual failure: browser assertion failed") || !strings.HasSuffix(receipt.Output, "stderr terminal error\n") {
		t.Fatal("real command receipt lost source or failure tail")
	}
	for _, part := range []string{"credential-prefix", "SENSITIVE", "credential-suffix"} {
		if strings.Contains(receipt.Output, part) {
			t.Fatalf("secret fragment leaked: %s", part)
		}
	}
	if !strings.Contains(receipt.Output, "[redacted]") {
		t.Fatal("missing redaction")
	}
}

func TestConnectorLogRedactsSecretsAcrossWritesAndFinalFlush(t *testing.T) {
	t.Setenv("CONNECTOR_LOG_SHORT", "shared-prefix")
	t.Setenv("CONNECTOR_LOG_LONG", "shared-prefix-private-suffix")
	v := Connector{TokenEnv: "CONNECTOR_LOG_SHORT", EnvRefs: map[string]string{"LONG": "CONNECTOR_LOG_LONG"}}
	input := "start shared-prefix-private-suffix\nend shared-prefix"
	for _, size := range []int{1, 5, len(input)} {
		log := newBoundedConnectorLog(v)
		for start := 0; start < len(input); start += size {
			_, _ = log.Write([]byte(input[start:min(start+size, len(input))]))
		}
		for range 2 {
			if got := log.String(); got != "start [redacted]\nend [redacted]" {
				t.Fatalf("chunk size %d: unsafe output %q", size, got)
			}
		}
	}
}

func TestConnectorCommandMayIgnoreLargeWorkflowInput(t *testing.T) {
	workspace := t.TempDir()
	v := Connector{Name: "fixed test entry", Kind: "command", Enabled: true, WorkspaceRoot: workspace, Executable: "/bin/sh", Args: []string{"-c", "printf 'actual failure: assertion failed\\n'; exit 23"}, TimeoutSeconds: 5}
	receipt, err := executeConnectorCommand(context.Background(), v, WorkflowRun{ID: "large-input", WorkspacePath: workspace}, WorkflowStep{Seq: 1}, connectorRequest{Stdin: strings.Repeat("previous workflow results\n", 10000)}, t.TempDir())
	if err != nil {
		t.Fatalf("command did not need stdin, but its result was discarded: %v", err)
	}
	if receipt.ExitCode == nil || *receipt.ExitCode != 23 || receipt.Output != "actual failure: assertion failed\n" {
		t.Fatalf("lost actual command result: %+v", receipt)
	}
}

func TestConnectorCommandConsumesLargeWorkflowInput(t *testing.T) {
	workspace := t.TempDir()
	payload := "input-start\n" + strings.Repeat("input-body\n", 10000) + "input-end\n"
	v := Connector{Name: "read workflow input", Kind: "command", Enabled: true, WorkspaceRoot: workspace, Executable: "/bin/cat", TimeoutSeconds: 5}
	receipt, err := executeConnectorCommand(context.Background(), v, WorkflowRun{ID: "read-large-input", WorkspacePath: workspace}, WorkflowStep{Seq: 1}, connectorRequest{Stdin: payload}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ExitCode == nil || *receipt.ExitCode != 0 || !strings.HasPrefix(receipt.Output, "input-start\n") || !strings.HasSuffix(receipt.Output, "input-end\n") {
		t.Fatalf("input-consuming command lost data: %+v", receipt)
	}
}

func TestConnectorCancelWithUnreadWorkflowInputKeepsReceipt(t *testing.T) {
	workspace := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	v := Connector{Name: "cancel unread input", Kind: "command", Enabled: true, WorkspaceRoot: workspace, Executable: "/bin/sh", Args: []string{"-c", "printf 'started\\n'; sleep 20"}, TimeoutSeconds: 30}
	receipt, err := executeConnectorCommand(ctx, v, WorkflowRun{ID: "cancel-large-input", WorkspacePath: workspace}, WorkflowStep{Seq: 1}, connectorRequest{Stdin: strings.Repeat("previous results\n", 10000)}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "command interrupted") {
		t.Fatalf("interruption replaced by a pipe error: %v", err)
	}
	if receipt.ExitCode == nil || *receipt.ExitCode != -1 || receipt.Output != "started\n" {
		t.Fatalf("lost interrupted command receipt: %+v", receipt)
	}
}
