package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/pelletier/go-toml/v2"
)

// This loader accepts only a private, same-user service configuration. Task
// input cannot choose a credential file or change the service connection.
func loadOpenSandboxConfig(path, image string, ttl int) (*OpenSandbox, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, errors.New("OpenSandbox service configuration is unavailable")
	}
	if !before.Mode().IsRegular() || before.Mode().Perm()&0077 != 0 {
		return nil, errors.New("OpenSandbox configuration must be a private regular file")
	}
	if owner, ok := before.Sys().(*syscall.Stat_t); !ok || int(owner.Uid) != os.Getuid() {
		return nil, errors.New("OpenSandbox configuration must belong to the platform user")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("OpenSandbox service configuration is unavailable")
	}
	defer f.Close()
	actual, err := f.Stat()
	if err != nil || !os.SameFile(before, actual) {
		return nil, errors.New("OpenSandbox configuration changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(raw) > 65536 {
		return nil, errors.New("OpenSandbox configuration cannot be read safely")
	}
	var config struct {
		Server struct {
			Host string `toml:"host"`
			Port int    `toml:"port"`
			Key  string `toml:"api_key"`
		} `toml:"server"`
	}
	if toml.Unmarshal(raw, &config) != nil || net.ParseIP(config.Server.Host) == nil || !net.ParseIP(config.Server.Host).IsLoopback() || config.Server.Port < 1024 || config.Server.Port > 65535 {
		return nil, errors.New("OpenSandbox configuration requires the private loopback service")
	}
	return newOpenSandbox("http://"+net.JoinHostPort(config.Server.Host, fmt.Sprint(config.Server.Port)), config.Server.Key, image, ttl)
}

func (s *OpenSandbox) remove(ctx context.Context, id, run string) error {
	if _, err := s.owned(ctx, id, run); err != nil {
		return err
	}
	if err := s.lifecycle.DeleteSandbox(ctx, id); err != nil {
		return sandboxError("environment release unconfirmed", err)
	}
	_, err := s.lifecycle.GetSandbox(ctx, id)
	var api *opensandbox.APIError
	if errors.As(err, &api) && api.StatusCode == 404 {
		return nil
	}
	if err != nil {
		return sandboxError("environment absence unconfirmed", err)
	}
	return errors.New("sandbox remains present after release")
}

// ProbeOpenSandbox is a bounded infrastructure check using the same client as
// Run execution. It never performs product acceptance or supplies credentials to
// the sandbox. The report is exclusive, private and retains uncertain outcomes.
func ProbeOpenSandbox(ctx context.Context, config, image, reportPath string) error {
	return probeOpenSandbox(ctx, config, image, reportPath, false)
}

func ProbeOpenSandboxRuntime(ctx context.Context, config, image, reportPath string) error {
	return probeOpenSandbox(ctx, config, image, reportPath, true)
}

func probeOpenSandbox(ctx context.Context, config, image, reportPath string, runtime bool) (probeErr error) {
	service, err := loadOpenSandboxConfig(config, image, 600)
	if err != nil {
		return err
	}
	report, err := os.OpenFile(reportPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("choose a new private probe report path")
	}
	defer report.Close()
	record := func(event string, values any) error {
		err := json.NewEncoder(report).Encode(map[string]any{"event": event, "time": time.Now().UTC(), "values": values})
		if err != nil {
			return err
		}
		return report.Sync()
	}
	run := newID()
	var sandboxID string
	defer func() {
		if sandboxID != "" {
			cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := record("release_intent", sandboxID); err != nil {
				probeErr = errors.Join(probeErr, err)
				return
			}
			err := service.remove(cleanup, sandboxID, run)
			if err != nil {
				record("release_unconfirmed", err.Error())
				probeErr = errors.Join(probeErr, err)
			} else if err = record("absence_confirmed", sandboxID); err != nil {
				probeErr = errors.Join(probeErr, err)
			}
		}
	}()
	if err = record("create_intent", map[string]string{"run": run, "image": image, "sdk": "v1.1.1"}); err != nil {
		return err
	}
	info, err := service.create(ctx, run)
	if err != nil {
		record("create_unconfirmed", err.Error())
		// Read-only reconciliation may recover the ID for cleanup, never repost.
		existing, lookupErr := service.find(ctx, run)
		if lookupErr == nil && existing != nil {
			sandboxID = existing.ID
			record("reconciled", existing)
		}
		return errors.Join(err, lookupErr)
	}
	sandboxID = info.ID
	if err = record("created", info); err != nil {
		return err
	}
	if err = service.waitReadyObserved(ctx, sandboxID, run, func(attempt int, result string) error {
		return record("execution_readiness", map[string]any{"attempt": attempt, "result": result})
	}); err != nil {
		return err
	}
	info, err = service.owned(ctx, sandboxID, run)
	if err != nil {
		return err
	}
	execd, err := service.execd(ctx, sandboxID, run)
	if err != nil {
		return err
	}
	if err = record("running", info); err != nil {
		return err
	}
	events := func(e opensandbox.StreamEvent) error { return record("command_event", e) }
	program := `from pathlib import Path; Path('/workspace').mkdir(exist_ok=True); Path('/workspace/run-proof.txt').write_text('literal $(no shell)'); print('SANDBOX_PYTHON_PASS')`
	result, commandErr := service.command(ctx, sandboxID, run, opensandbox.RunCommandRequest{Argv: []string{"python3", "-c", program}, Cwd: "/", Timeout: 30000}, events)
	if err = record("python_result", map[string]any{"receipt": result, "error": errorText(commandErr)}); err != nil {
		return err
	}
	if commandErr != nil {
		return commandErr
	}
	if result.ExitCode == nil || *result.ExitCode != 0 || !result.Stopped {
		return errors.New("sandbox Python completion was not proved")
	}
	content := []byte(`{"run":"` + run + `"}`)
	target := "/workspace/sdk-proof.json"
	if err = service.upload(ctx, sandboxID, run, target, content, 0600); err != nil {
		return err
	}
	source, err := execd.DownloadFile(ctx, target, "")
	if err != nil {
		return sandboxError("probe download", err)
	}
	found, readErr := io.ReadAll(io.LimitReader(source, 4096))
	source.Close()
	if readErr != nil || !bytes.Equal(found, content) {
		return errors.New("sandbox file round-trip did not preserve content")
	}
	result, commandErr = service.command(ctx, sandboxID, run, opensandbox.RunCommandRequest{Argv: []string{"python3", "-c", "import os,stat; assert stat.S_IMODE(os.stat('/workspace/sdk-proof.json').st_mode)==0o600; print('SANDBOX_PRIVATE_FILE_PASS')"}, Timeout: 30000}, events)
	if commandErr != nil || result.ExitCode == nil || *result.ExitCode != 0 || !result.Stopped {
		return errors.Join(errors.New("sandbox file permissions were not proved"), commandErr)
	}
	if err = record("file_round_trip", map[string]any{"bytes": len(found), "same": true, "mode": "0600", "path": filepath.ToSlash(target)}); err != nil {
		return err
	}
	// Use the conversation API's confined reader, rather than equating a raw
	// SDK download with safe access to workspace files.
	store := Store{sandbox: service}
	artifactRun := WorkflowRun{ID: run, WorkspacePath: "/workspace", Definition: Workflow{Environment: "opensandbox"}, Environment: &RunEnvironment{ID: sandboxID, Status: "ready", ConnectionHash: service.connectionHash}}
	files, _, artifactErr := store.sandboxArtifacts(ctx, artifactRun, "")
	if artifactErr != nil {
		return artifactErr
	}
	if len(files) != 2 {
		return errors.New("confined workspace inventory did not contain the two probe files")
	}
	_, found, artifactErr = store.sandboxArtifacts(ctx, artifactRun, "sdk-proof.json")
	if artifactErr != nil || !bytes.Equal(found, content) {
		return errors.Join(errors.New("confined workspace file read did not preserve probe content"), artifactErr)
	}
	if err = record("artifact_access_confirmed", map[string]any{"file_count": len(files), "bytes": len(found), "exact_content": true, "platform_conversation": "not_run"}); err != nil {
		return err
	}
	bounded, stop := context.WithTimeout(ctx, 2*time.Second)
	result, commandErr = service.command(bounded, sandboxID, run, opensandbox.RunCommandRequest{Argv: []string{"python3", "-c", "import time; time.sleep(30)"}, Timeout: 30000}, events)
	stop()
	if err = record("cancel_result", map[string]any{"receipt": result, "error": errorText(commandErr)}); err != nil {
		return err
	}
	if !errors.Is(commandErr, context.DeadlineExceeded) || result.ID == "" || !result.Stopped {
		return errors.New("sandbox cancellation was not independently confirmed")
	}
	if runtime {
		if err = probeSandboxRuntime(ctx, service, sandboxID, run, record); err != nil {
			return err
		}
	}
	return record("probe_pass", map[string]bool{"lifecycle": true, "python": true, "files": true, "cancel": true, "product_acceptance": false})
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
