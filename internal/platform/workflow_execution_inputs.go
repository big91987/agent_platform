package platform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func validateExecutionBindings(bindings map[string]ExecutionBinding) error {
	if len(bindings) > 64 {
		return errors.New("tool arguments support at most 64 input references")
	}
	for name, binding := range bindings {
		if strings.TrimSpace(name) == "" || len(name) > 256 || strings.ContainsRune(name, 0) {
			return errors.New("an input reference needs a tool parameter name up to 256 bytes")
		}
		if binding.Node != "" && !workflowKey.MatchString(binding.Node) {
			return errors.New("an input reference needs an existing node key")
		}
		if _, err := executionPointerParts(binding.Path); err != nil {
			return err
		}
	}
	return nil
}

func executionPointerParts(path string) ([]string, error) {
	if !strings.HasPrefix(path, "/") || len(path) > 2048 || strings.ContainsRune(path, 0) {
		return nil, errors.New("an input reference path must start with / and fit within 2048 bytes")
	}
	parts := strings.Split(path[1:], "/")
	if len(parts) > 64 {
		return nil, errors.New("an input reference path exceeds 64 fields")
	}
	for i, part := range parts {
		for j := 0; j < len(part); j++ {
			if part[j] == '~' {
				if j+1 == len(part) || (part[j+1] != '0' && part[j+1] != '1') {
					return nil, errors.New("an input reference path uses an invalid JSON Pointer escape")
				}
				j++
			}
		}
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts, nil
}

func executionStepValues(step WorkflowStep) map[string]any {
	return map[string]any{"node": step.NodeID, "seq": step.Seq, "status": step.Status, "result": step.Result, "execution_receipt": step.actionReceipt(), "error": step.Error}
}

func executionInputValues(run WorkflowRun, step WorkflowStep) map[string]any {
	previous := []any{}
	for _, old := range run.Steps {
		if old.Seq < step.Seq {
			previous = append(previous, executionStepValues(old))
		}
	}
	return map[string]any{"run_id": run.ID, "workspace_path": run.WorkspacePath, "input": run.Input, "parameters": run.Parameters, "previous_results": previous}
}

func resolveExecutionArguments(run WorkflowRun, step WorkflowStep, execution WorkflowExecution) (json.RawMessage, error) {
	if err := validateExecutionBindings(execution.Bindings); err != nil {
		return nil, err
	}
	if len(execution.Bindings) == 0 {
		return append(json.RawMessage(nil), execution.Arguments...), nil
	}
	var arguments map[string]json.RawMessage
	if err := json.Unmarshal(execution.Arguments, &arguments); err != nil || arguments == nil {
		return nil, errors.New("tool arguments must be a JSON object")
	}
	// Decode the existing public execution data once per source. This excludes
	// server connections, credentials, Agent homes and other Run identities.
	sources := map[string]any{}
	for name, binding := range execution.Bindings {
		source, ok := sources[binding.Node]
		if !ok {
			var value any = executionInputValues(run, step)
			if binding.Node != "" {
				var latest *WorkflowStep
				for i := range run.Steps {
					old := &run.Steps[i]
					if old.NodeID == binding.Node && old.Seq < step.Seq && (latest == nil || old.Seq > latest.Seq) {
						latest = old
					}
				}
				if latest == nil || (latest.Status != "completed" && latest.Status != "failed" && latest.Status != "stopped") {
					return nil, fmt.Errorf("input reference for tool parameter %q has no finished prior node step", name)
				}
				value = executionStepValues(*latest)
			}
			raw, err := json.Marshal(value)
			if err != nil || len(raw) > connectorArchiveLimit {
				return nil, errors.New("input reference source cannot be encoded within 8 MiB; reference large artifacts instead")
			}
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			if err := decoder.Decode(&source); err != nil {
				return nil, errors.New("input reference source is not JSON data")
			}
			sources[binding.Node] = source
		}
		parts, _ := executionPointerParts(binding.Path)
		value := source
		for _, part := range parts {
			found := false
			switch current := value.(type) {
			case map[string]any:
				value, found = current[part]
			case []any:
				index, err := strconv.Atoi(part)
				if err == nil && index >= 0 && index < len(current) && strconv.Itoa(index) == part {
					value, found = current[index], true
				}
			}
			if !found {
				return nil, fmt.Errorf("input reference for tool parameter %q is unavailable", name)
			}
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, errors.New("input reference value cannot be encoded")
		}
		arguments[name] = raw
	}
	raw, err := json.Marshal(arguments)
	if err != nil || len(raw) > 64000 {
		return nil, errors.New("resolved tool arguments exceed 64000 bytes; reference large artifacts instead")
	}
	return raw, nil
}
