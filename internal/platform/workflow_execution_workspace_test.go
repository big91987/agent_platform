package platform

import (
	"encoding/json"
	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutionRejectsInvalidDirectoryAndEnvironmentConfiguration(t *testing.T) {
	for _, fields := range []string{
		`"cwd":"../other"`, `"cwd":"/tmp"`, `"cwd":"service/../other"`,
		`"env":{"WORKFLOW_INPUT_FILE":"other"}`, `"env":{"PATH":"other"}`,
		`"env":{"VALUE":"literal"},"env_refs":{"VALUE":"PROJECT_VALUE"}`,
	} {
		var v WorkflowExecution
		if err := json.Unmarshal([]byte(`{"mode":"command","command":"pwd",`+fields+`}`), &v); err != nil {
			t.Fatal(err)
		}
		if validateWorkflowExecution(&v) == nil {
			t.Errorf("accepted unsafe execution settings: %s", fields)
		}
	}
}

func TestExecutionDirectoryAndVariablesReachOnlySelectedProcess(t *testing.T) {
	root := t.TempDir()
	sub := "services/api"
	if err := os.MkdirAll(filepath.Join(root, sub), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SELECTED_EXECUTION_TOKEN", "private-selected-value")
	values, err := executionEnvironment(map[string]string{"MODE": "literal $(touch PWN)"}, map[string]string{"SELECTED_TOKEN": "SELECTED_EXECUTION_TOKEN"})
	if err != nil {
		t.Fatal(err)
	}
	request := opensandbox.RunCommandRequest{Cwd: root, Command: `python3 -c 'import os,json; print(json.dumps({"cwd":os.getcwd(),"mode":os.environ["MODE"],"selected":os.environ["SELECTED_TOKEN"]=="private-selected-value","ambient": "SELECTED_EXECUTION_TOKEN" in os.environ}))'`}
	configureExecutionDirectory(&request, sub)
	cmd := exec.Command(request.Argv[0], request.Argv[1:]...)
	cmd.Dir = root
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	for name, value := range values {
		cmd.Env = append(cmd.Env, name+"="+value)
	}
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(raw))
	}
	var output struct {
		Cwd, Mode         string
		Selected, Ambient bool
	}
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatal(err, string(raw))
	}
	actual, _ := filepath.EvalSymlinks(filepath.Join(root, sub))
	if output.Cwd != actual || output.Mode != values["MODE"] || !output.Selected || output.Ambient {
		t.Fatal("process environment or directory differs", output)
	}
	fresh, err := executionEnvironment(nil, nil)
	if err != nil || len(fresh) != 0 {
		t.Fatal("next node inherited variables", fresh, err)
	}
	if _, err := os.Stat(filepath.Join(root, "PWN")); !os.IsNotExist(err) {
		t.Fatal("literal environment was executed", err)
	}
}

func TestExecutionDirectoryRejectsMissingAndSymlinkBeforeCommand(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"missing", "linked"} {
		request := opensandbox.RunCommandRequest{Cwd: root, Command: "touch MUST_NOT_RUN"}
		configureExecutionDirectory(&request, relative)
		cmd := exec.Command(request.Argv[0], request.Argv[1:]...)
		cmd.Dir = root
		raw, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(raw), "Execution working directory") {
			t.Fatal(relative, err, string(raw))
		}
		for _, dir := range []string{root, outside} {
			if _, err := os.Stat(filepath.Join(dir, "MUST_NOT_RUN")); !os.IsNotExist(err) {
				t.Fatal("project command executed in rejected directory", relative, err)
			}
		}
	}
}

func TestExecutionSettingsPersistAndStayFrozenPerRun(t *testing.T) {
	s := testStore(t)
	caller, w, _, _ := sandboxWorkflowFixture(t, s, false, false)
	w.Entry = "execute"
	w.Nodes = append(w.Nodes, WorkflowNode{ID: "execute", Name: "Execute", Kind: "execution", RuleHandoff: executionTestRules("command", "review"), Execution: &WorkflowExecution{Mode: "command", Command: "pwd", Cwd: "services/api", Env: map[string]string{"MODE": "original"}, Output: &ExecutionOutput{Source: "exit_code"}}})
	saved, err := s.SaveWorkflow(caller, w)
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.StartWorkflow(caller, WorkflowStart{WorkflowID: saved.ID, Input: "check location"})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := s.Workflow(caller, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	n := stored.node("execute")
	if n.Execution.Cwd != "services/api" || n.Execution.Env["MODE"] != "original" {
		t.Fatal("configuration did not persist", n)
	}
	for i := range stored.Nodes {
		if stored.Nodes[i].ID == "execute" {
			stored.Nodes[i].Execution.Cwd = "other"
			stored.Nodes[i].Execution.Env["MODE"] = "new"
		}
	}
	if _, err := s.SaveWorkflow(caller, stored); err != nil {
		t.Fatal(err)
	}
	frozen, err := s.WorkflowRun(caller, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v := frozen.Definition.node("execute").Execution; v.Cwd != "services/api" || v.Env["MODE"] != "original" {
		t.Fatal("new edits changed frozen run", v)
	}
}

func TestMCPRejectsProcessDirectoryAndEnvironment(t *testing.T) {
	for _, v := range []*WorkflowExecution{{Mode: "mcp", Cwd: "services"}, {Mode: "mcp", Env: map[string]string{"MODE": "test"}}} {
		if err := validateWorkflowExecution(v); err == nil {
			t.Fatal("MCP accepted process configuration", v)
		}
	}
}
