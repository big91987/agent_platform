package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	if err := service.nativeHome(ctx, id, run, "/tmp/probe-native-home"); err != nil {
		return err
	}
	if err := record("native_home_prepared", map[string]any{"mode": "0700"}); err != nil {
		return err
	}
	selectedSkill := "/tmp/probe-native-home/skills/probe-selected/SKILL.md"
	for _, fixture := range []struct{ path, name string }{{selectedSkill, "probe-selected"}, {"/workspace/.agents/skills/probe-unselected/SKILL.md", "probe-unselected"}} {
		if err := service.nativeHome(ctx, id, run, strings.TrimSuffix(fixture.path, "/SKILL.md")); err != nil {
			return err
		}
		raw := "---\nname: " + fixture.name + "\ndescription: Credential-free native Skill scope probe.\n---\nThis probe sends no model task.\n"
		if err := service.upload(ctx, id, run, fixture.path, []byte(raw), 0600); err != nil {
			return err
		}
	}
	token := newID() + newID()
	if err = service.upload(ctx, id, run, "/tmp/probe-native-token", []byte(token), 0600); err != nil {
		return err
	}
	if err = record("native_launch_intent", map[string]any{"port": 8080, "model_turn": false}); err != nil {
		return err
	}
	process, launchErr := service.startNative(ctx, id, run, opensandbox.RunCommandRequest{Command: "exec codex app-server --strict-config --listen ws://0.0.0.0:8080 --ws-auth capability-token --ws-token-file /tmp/probe-native-token", Cwd: "/workspace", Envs: map[string]string{"CODEX_HOME": "/tmp/probe-native-home"}, Timeout: 120000}, events)
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
	return probeSandboxNativeSkillScope(stream, selectedSkill, record)
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
