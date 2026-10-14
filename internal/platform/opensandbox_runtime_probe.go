package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

// The runtime probe supplies no native login and sends no model turn. It checks
// actual TypeScript and native service transport/lifecycle in the owned sandbox.
func probeSandboxRuntime(ctx context.Context, service *OpenSandbox, id, run string, record func(string, any) error) (probeErr error) {
	events := func(e opensandbox.StreamEvent) error { return record("runtime_command_event", e) }
	result, err := service.command(ctx, id, run, opensandbox.RunCommandRequest{Command: `tsx -e 'import fs from "node:fs"; const value: string = fs.readFileSync("/workspace/run-proof.txt","utf8"); if(value!=="literal $(no shell)")throw Error("shared file lost"); console.log("SANDBOX_TYPESCRIPT_PASS")'`, Cwd: "/workspace", Timeout: 30000}, events)
	if e := record("typescript_result", map[string]any{"receipt": result, "error": errorText(err)}); e != nil {
		return e
	}
	if err != nil || result.ExitCode == nil || *result.ExitCode != 0 || !result.Stopped {
		return errors.Join(errors.New("sandbox TypeScript completion was not proved"), err)
	}
	home, selectedSkill, err := probeSandboxRoleInputs(ctx, service, id, run, record)
	if err != nil {
		return err
	}
	// This unselected project Skill checks scope exclusion in the same native
	// process that consumes the prepared role home.
	unselected := "/workspace/.agents/skills/probe-unselected/SKILL.md"
	if err = service.nativeHome(ctx, id, run, strings.TrimSuffix(unselected, "/SKILL.md")); err != nil {
		return err
	}
	if err = service.upload(ctx, id, run, unselected, []byte("---\nname: probe-unselected\ndescription: Credential-free scope fixture.\n---\nNo model task is sent.\n"), 0600); err != nil {
		return err
	}
	token := newID() + newID()
	if err = service.upload(ctx, id, run, "/tmp/probe-native-token", []byte(token), 0600); err != nil {
		return err
	}
	if err = record("native_launch_intent", map[string]any{"port": 8080, "model_turn": false}); err != nil {
		return err
	}
	process, launchErr := service.startNative(ctx, id, run, opensandbox.RunCommandRequest{Command: "exec codex app-server --strict-config --listen ws://0.0.0.0:8080 --ws-auth capability-token --ws-token-file /tmp/probe-native-token", Cwd: "/workspace", Envs: map[string]string{"HOME": home, "CODEX_HOME": home}, Timeout: 120000}, events)
	if process.ID != "" {
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := record("native_stop_intent", process.ID); err != nil {
				probeErr = errors.Join(probeErr, err)
				return
			}
			stopped := process
			var err error
			if !process.Stopped && !process.interruptSent {
				stopped, err = service.stopNative(cleanup, id, run, process.ID)
			}
			logErr := record("native_stop_result", map[string]any{"receipt": stopped, "error": errorText(err)})
			if err != nil || !stopped.Stopped {
				probeErr = errors.Join(probeErr, errors.New("native process stop was not proved"), err)
			}
			probeErr = errors.Join(probeErr, logErr)
			logs, logsErr := service.nativeLogs(cleanup, id, run, process.ID)
			var output string
			var cursor int64
			if logs != nil {
				output = strings.ReplaceAll(logs.Output, token, "[private capability]")
				cursor = logs.Cursor
			}
			probeErr = errors.Join(probeErr, logsErr, record("native_process_output", map[string]any{"output": output, "cursor": cursor, "error": errorText(logsErr)}))
		}()
	}
	if err = record("native_launch_result", map[string]any{"receipt": process, "error": errorText(launchErr)}); err != nil {
		return err
	}
	if launchErr != nil {
		return launchErr
	}
	stream, err := service.connectNative(ctx, id, run, process.ID, token, func(count int, err error) error {
		return record("native_connect_attempt", map[string]any{"attempt": count, "error": errorText(err)})
	})
	if err != nil {
		return err
	}
	defer stream.Close()
	if err = json.NewEncoder(stream).Encode(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]string{"name": "agent_platform_probe", "version": "0.1.0"}, "capabilities": map[string]any{"experimentalApi": true}}}); err != nil {
		return err
	}
	var initialized struct {
		ID     int             `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err = json.NewDecoder(io.LimitReader(stream, 4*1024*1024)).Decode(&initialized); err != nil {
		return err
	}
	if initialized.ID != 1 || len(initialized.Error) > 0 || len(initialized.Result) == 0 {
		return errors.New("native initialization was not proved")
	}
	if err = record("native_initialized", map[string]any{"result": initialized.Result, "transport": "authenticated WebSocket", "native_login": "not_run", "model_turn": "not_run"}); err != nil {
		return err
	}
	if err = json.NewEncoder(stream).Encode(map[string]any{"method": "initialized"}); err != nil {
		return err
	}
	if err = probeSandboxNativeSkillScope(stream, selectedSkill, record); err != nil {
		return err
	}
	if err = probeSandboxExternalCommand(stream, record); err != nil {
		return err
	}
	return record("native_role_process_confirmed", map[string]any{"home": home, "selected": selectedSkill, "same_process": process.ID, "credentials": "empty_fixture_only", "platform_conversation": "not_run", "model_turn": "not_run"})
}

// The real SDK exercises role files using an empty authentication object. No
// platform conversation, credential validity or model result is asserted here.
func probeSandboxRoleInputs(ctx context.Context, service *OpenSandbox, id, run string, record func(string, any) error) (string, string, error) {
	root, err := os.MkdirTemp("", "agent-platform-role-probe-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(root)
	auth, skill := filepath.Join(root, "auth-fixture"), filepath.Join(root, "selected-skill")
	for _, directory := range []string{auth, skill} {
		if err = os.Mkdir(directory, 0700); err != nil {
			return "", "", err
		}
	}
	if err = os.WriteFile(filepath.Join(auth, "auth.json"), []byte(`{}`), 0600); err != nil {
		return "", "", err
	}
	if err = os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: role-input-probe\ndescription: Credential-free role transfer fixture.\n---\nNo model task is sent.\n"), 0600); err != nil {
		return "", "", err
	}
	c := Conversation{ID: newID(), Snapshot: Agent{Executor: "codex", Model: "gpt-6.1-sol", Skills: []string{skill}, Instructions: "Credential-free SDK role preparation fixture."}}
	r := WorkflowRun{ID: run, WorkspacePath: "/workspace", Definition: Workflow{Environment: "opensandbox"}, Environment: &RunEnvironment{ID: id, Status: "ready", ConnectionHash: service.connectionHash}}
	x := Codex{Root: root, AuthHome: auth}
	home, cfg, selected, _, err := x.prepareSandboxRole(ctx, service, r, c)
	if err != nil {
		return "", "", err
	}
	if len(selected) != 1 || len(cfg) == 0 {
		return "", "", errors.New("SDK role inputs were not prepared")
	}
	if err = os.Remove(filepath.Join(auth, "auth.json")); err != nil {
		return "", "", err
	}
	// Resume uses the recorded assets, not mutable host sources or refreshed
	// credentials. The caller then starts one native process in this role home.
	if err = os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("changed host fixture"), 0600); err != nil {
		return "", "", err
	}
	home2, cfg2, selected2, _, err := x.prepareSandboxRole(ctx, service, r, c)
	if err != nil {
		return "", "", err
	}
	if home2 != home || !bytes.Equal(cfg, cfg2) || len(selected2) != 1 || selected2[0] != selected[0] {
		return "", "", errors.New("SDK role resume replaced its frozen inputs")
	}
	err = record("native_role_inputs_confirmed", map[string]any{"home": home, "selected_count": 1, "same_preparation": true, "host_auth_removed": true, "host_skill_changed": true, "credentials": "empty_fixture_only", "platform_conversation": "not_run", "model_turn": "not_run"})
	if err != nil {
		return "", "", err
	}
	return home, selected[0], nil
}

// A diagnostic uses the same schema and scope comparison as native execution,
// while deliberately sending neither a thread nor a model turn.
func probeSandboxNativeSkillScope(stream io.ReadWriter, selected string, record func(string, any) error) error {
	decoder, encoder := json.NewDecoder(stream), json.NewEncoder(stream)
	request := func(id int, method string, params any) (json.RawMessage, error) {
		if err := encoder.Encode(map[string]any{"id": id, "method": method, "params": params}); err != nil {
			return nil, err
		}
		for count := 0; count < 32; count++ {
			var reply struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := decoder.Decode(&reply); err != nil {
				return nil, err
			}
			if reply.ID != id {
				continue
			}
			if err := record("native_skill_reply", map[string]any{"method": method, "reply": reply}); err != nil {
				return nil, err
			}
			if len(reply.Error) > 0 {
				return nil, errors.New("native Skill request rejected")
			}
			return reply.Result, nil
		}
		return nil, errors.New("native Skill response was not received within the diagnostic bound")
	}
	params := map[string]any{"cwds": []string{"/workspace"}, "forceReload": true}
	raw, err := request(10, "skills/list", params)
	if err != nil {
		return err
	}
	actual, err := sandboxNativeSkills(raw, "/workspace")
	if err != nil {
		return err
	}
	choices, err := sandboxSkillChoices(actual, []string{selected}, false)
	if err != nil {
		return err
	}
	for _, choice := range choices {
		raw, err = request(11, "skills/config/write", map[string]any{"path": choice.Path, "enabled": choice.Enabled})
		if err != nil {
			return err
		}
		var reply struct {
			Enabled *bool `json:"effectiveEnabled"`
		}
		if json.Unmarshal(raw, &reply) != nil || reply.Enabled == nil || *reply.Enabled != choice.Enabled {
			return errors.New("native Skill setting was not confirmed")
		}
	}
	raw, err = request(12, "skills/list", params)
	if err != nil {
		return err
	}
	actual, err = sandboxNativeSkills(raw, "/workspace")
	if err != nil {
		return err
	}
	if _, err = sandboxSkillChoices(actual, []string{selected}, true); err != nil {
		return err
	}
	return record("native_skill_scope_confirmed", map[string]any{"selected": selected, "enabled_count": 1, "native_login": "not_run", "model_turn": "not_run"})
}

// The actual native command runs in the already-owned container with no model
// authentication and no nested namespace. This is a capability check, not a
// platform conversation or product acceptance.
func probeSandboxExternalCommand(stream io.ReadWriter, record func(string, any) error) error {
	params := map[string]any{
		"command": []string{"python3", "-c", `from pathlib import Path; p=Path("/workspace/external-command-proof.json"); p.write_text('"literal $(no shell)"'); assert p.read_text()=='"literal $(no shell)"'; assert not Path("/var/run/docker.sock").exists(); print("EXTERNAL_SANDBOX_COMMAND_PASS")`},
		"cwd":     "/workspace", "timeoutMs": 10000,
		"sandboxPolicy": map[string]any{"type": "externalSandbox", "networkAccess": "enabled"},
	}
	if err := json.NewEncoder(stream).Encode(map[string]any{"id": 13, "method": "command/exec", "params": params}); err != nil {
		return err
	}
	decoder := json.NewDecoder(stream)
	for count := 0; count < 32; count++ {
		var reply struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if err := decoder.Decode(&reply); err != nil {
			return err
		}
		if reply.ID != 13 {
			continue
		}
		if err := record("native_external_command_reply", reply); err != nil {
			return err
		}
		if len(reply.Error) > 0 {
			return errors.New("native external-sandbox command was rejected; original reply retained")
		}
		var result struct {
			Exit   *int   `json:"exitCode"`
			Stdout string `json:"stdout"`
			Stderr string `json:"stderr"`
		}
		if json.Unmarshal(reply.Result, &result) != nil || result.Exit == nil || *result.Exit != 0 || !strings.Contains(result.Stdout, "EXTERNAL_SANDBOX_COMMAND_PASS") || result.Stderr != "" {
			return errors.New("native external-sandbox command did not prove successful execution")
		}
		return record("native_external_command_confirmed", map[string]any{"policy": "externalSandbox", "native_exit": *result.Exit, "workspace_write_read": true, "docker_socket_absent": true, "credentials": "empty_fixture_only", "model_turn": "not_run", "platform_conversation": "not_run"})
	}
	return errors.New("native external command response exceeded the diagnostic bound")
}
