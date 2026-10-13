package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

// A role home/config must have been prepared and verified before this method.
// The shared workspace and process association come from the owned Run, never
// an external conversation path. Public sandbox Agent entry stays gated until
// role preparation, credentials, Skills and native execution are verified.
func (x *Codex) executeSandboxAppServer(ctx context.Context, store *Store, client *OpenSandbox, run WorkflowRun, c Conversation, m Message, home string, configRaw []byte, env map[string]string, onEvent func([]byte) error) (executeErr error) {
	if store == nil || client == nil || run.Environment == nil || run.Environment.Status != "ready" || run.WorkspacePath != "/workspace" || !validSandboxRun(c.ID) || home != "/var/lib/agent-platform/roles/"+c.ID {
		return errors.New("native role context does not match its owned Run")
	}
	if client.connectionHash != run.Environment.ConnectionHash {
		return errors.New("OpenSandbox service configuration changed; no native launch was sent")
	}
	if err := store.beginSandboxNative(run, c, m); err != nil {
		return err
	}
	state := "launching"
	process := SandboxCommandResult{}
	token := newID() + newID()
	tokenPath := fmt.Sprintf("%s/transport-%d", home, m.ID)
	redact := x.redactor(c.Snapshot)
	emit := func(value any) error {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return onEvent(redactJSON(raw, func(text string) string { return redact(strings.ReplaceAll(text, token, "[private capability]")) }))
	}
	update := func(next, reason string) error {
		if err := store.updateSandboxNative(m.ID, state, process.ID, next, reason); err != nil {
			return err
		}
		state = next
		return nil
	}
	if err := client.upload(ctx, run.Environment.ID, run.ID, tokenPath, []byte(token), 0600); err != nil {
		// No process launch was sent, even if this private file upload is uncertain.
		persistErr := update("stopped", "Native launch was not sent; capability preparation failed.")
		return errors.Join(err, persistErr)
	}
	launchSent := false
	defer func() {
		if !launchSent {
			return
		}
		if process.ID == "" {
			persistErr := update("unconfirmed", "Native launch response was lost; no replacement was sent.")
			executeErr = errors.Join(executeErr, persistErr, &nativeExecutionUnconfirmed{"Native launch identity is unconfirmed. Reconcile or release the owned environment before continuing; no launch was replayed."})
			return
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if !process.Stopped && !process.interruptSent {
			if state == "running" {
				if err := update("stopping", ""); err != nil {
					executeErr = errors.Join(executeErr, err)
				}
			}
			stopped, err := client.stopNative(cleanup, run.Environment.ID, run.ID, process.ID)
			process = stopped
			executeErr = errors.Join(executeErr, err)
		}
		if process.Stopped {
			if err := update("stopped", "Independent status confirmed the owned native process stopped."); err != nil {
				executeErr = errors.Join(executeErr, err, &nativeExecutionUnconfirmed{"Native stop was observed but its receipt could not be saved. Inspect the original process before continuing."})
			}
		} else {
			executeErr = errors.Join(executeErr, update("unconfirmed", "Native stop could not be confirmed; no interrupt was repeated."), &nativeExecutionUnconfirmed{"Native stop is unconfirmed. Reconcile or release the owned environment before continuing; no interrupt was replayed."})
		}
		executeErr = errors.Join(executeErr, emit(map[string]any{"type": "platform.sandbox.native.receipt", "receipt": process}))
		logs, err := client.nativeLogs(cleanup, run.Environment.ID, run.ID, process.ID)
		executeErr = errors.Join(executeErr, err)
		if logs != nil {
			executeErr = errors.Join(executeErr, emit(map[string]any{"type": "platform.sandbox.native.output", "output": logs.Output, "cursor": logs.Cursor}))
		}
	}()
	nativeEnv := map[string]string{"HOME": home, "CODEX_HOME": home, "NO_COLOR": "1"}
	for name, value := range env {
		if name == "HOME" || strings.HasPrefix(name, "CODEX_") || strings.HasPrefix(name, "AGENT_PLATFORM_") || !envName.MatchString(name) || strings.ContainsRune(value, 0) {
			// Preparation has no process side effect; close the reserved launch intent.
			return errors.Join(errors.New("native role environment contains a reserved or invalid key"), update("stopped", "Native launch was not sent; environment validation failed."))
		}
		nativeEnv[name] = value
	}
	launchSent = true
	launched, err := client.startNative(ctx, run.Environment.ID, run.ID, opensandbox.RunCommandRequest{Command: "exec codex app-server --strict-config --listen ws://0.0.0.0:8080 --ws-auth capability-token --ws-token-file " + tokenPath, Cwd: "/workspace", Envs: nativeEnv, Timeout: int64(run.Environment.TTL) * 1000}, func(event opensandbox.StreamEvent) error {
		previous := process.ID
		if err := acceptSandboxCommandID(event.Data, &process); err != nil {
			return err
		}
		if previous == "" && process.ID != "" {
			// Save the actual ID as soon as init arrives, before other event writes.
			if err := update("running", ""); err != nil {
				return err
			}
		}
		return emit(map[string]any{"type": "platform.sandbox.native.launch_output", "event": event})
	})
	// The helper retains this ID even when its callback/status/cleanup fails.
	process = launched
	if err != nil {
		return err
	}
	stream, err := client.connectNative(ctx, run.Environment.ID, run.ID, process.ID, token, nil)
	if err != nil {
		return err
	}
	defer stream.Close()
	return x.executeAppServerTransport(ctx, c, m, "/workspace", configRaw, stream, stream, func() string { return "Native diagnostics are retained with the owned execution receipt." }, onEvent)
}
