package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

// A completion contract is part of the frozen node, not a mutable tool catalog entry.
func completionInputsSchema(n HandoffParameters) (*jsonschema.Schema, error) {
	if len(n.Schema) == 0 {
		return &jsonschema.Schema{Type: "object", AdditionalProperties: &jsonschema.Schema{}}, nil
	}
	if len(n.Schema) > 32000 {
		return nil, errors.New("handoff_schema exceeds 32000 bytes")
	}
	value, err := completionJSONValue(n.Schema)
	if err != nil {
		return nil, fmt.Errorf("handoff_schema: %w", err)
	}
	root, ok := value.(map[string]any)
	if !ok || root["type"] != "object" {
		return nil, errors.New("handoff_schema must describe an object with type=object")
	}
	if err := checkHandoffSchema(value, 0); err != nil {
		return nil, err
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(n.Schema, &schema); err != nil {
		return nil, fmt.Errorf("handoff_schema: %w", err)
	}
	if _, err := schema.Resolve(nil); err != nil {
		return nil, fmt.Errorf("handoff_schema: %w", err)
	}
	return &schema, nil
}

// Keep the accepted dialect explicit: unknown keywords must not silently weaken
// a contract. No references, remote loaders, formats or default-value injection.
func checkHandoffSchema(value any, depth int) error {
	if depth > 32 {
		return errors.New("handoff_schema nesting exceeds 32")
	}
	if _, ok := value.(bool); ok {
		return nil
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return errors.New("handoff_schema contains a non-schema value")
	}
	for key, v := range obj {
		switch key {
		case "type":
			types, ok := v.([]any)
			if !ok {
				types = []any{v}
			}
			seen := map[string]bool{}
			if len(types) == 0 {
				return errors.New("handoff_schema type must not be empty")
			}
			for _, t := range types {
				name, ok := t.(string)
				if !ok || !slices.Contains([]string{"object", "array", "string", "number", "integer", "boolean", "null"}, name) || seen[name] {
					return errors.New("handoff_schema contains an invalid or duplicate type")
				}
				seen[name] = true
			}
		case "minLength", "maxLength", "minItems", "maxItems", "minProperties", "maxProperties":
			num, ok := v.(float64)
			if !ok || num < 0 || num != math.Trunc(num) {
				return fmt.Errorf("handoff_schema %s must be a nonnegative integer", key)
			}
		case "multipleOf":
			num, ok := v.(float64)
			if !ok || num <= 0 {
				return errors.New("handoff_schema multipleOf must be positive")
			}
		case "required":
			list, ok := v.([]any)
			if !ok {
				return errors.New("handoff_schema required must be an array")
			}
			seen := map[string]bool{}
			for _, item := range list {
				name, ok := item.(string)
				if !ok || seen[name] {
					return errors.New("handoff_schema required must contain unique strings")
				}
				seen[name] = true
			}
		case "enum":
			list, ok := v.([]any)
			if !ok || len(list) == 0 {
				return errors.New("handoff_schema enum must be a nonempty array")
			}
			seen := map[string]bool{}
			for _, item := range list {
				b, _ := json.Marshal(item)
				if seen[string(b)] {
					return errors.New("handoff_schema enum must contain unique values")
				}
				seen[string(b)] = true
			}
		case "$schema", "title", "description", "pattern":
			if _, ok := v.(string); !ok {
				return fmt.Errorf("handoff_schema %s must be a string", key)
			}
		case "examples":
			if _, ok := v.([]any); !ok {
				return errors.New("handoff_schema examples must be an array")
			}
		case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum":
			if _, ok := v.(float64); !ok {
				return fmt.Errorf("handoff_schema %s must be a number", key)
			}
		case "uniqueItems":
			if _, ok := v.(bool); !ok {
				return errors.New("handoff_schema uniqueItems must be a boolean")
			}
		case "const":
		case "properties":
			props, ok := v.(map[string]any)
			if !ok {
				return errors.New("handoff_schema properties must be an object")
			}
			for _, child := range props {
				if err := checkHandoffSchema(child, depth+1); err != nil {
					return err
				}
			}
		case "items", "additionalProperties", "not":
			if err := checkHandoffSchema(v, depth+1); err != nil {
				return err
			}
		case "allOf", "anyOf", "oneOf":
			list, ok := v.([]any)
			if !ok || len(list) == 0 {
				return fmt.Errorf("handoff_schema %s must be a nonempty array", key)
			}
			for _, child := range list {
				if err := checkHandoffSchema(child, depth+1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("handoff_schema keyword %q is not supported", key)
		}
	}
	return nil
}

func validateCompletionConfig(w Workflow, n WorkflowNode) error {
	if err := validateExecutionDrafts(n); err != nil {
		return err
	}
	if n.ExitMode != "" || len(n.RetiredCompletionSchema) > 0 || n.RetiredCompletionInstructions != "" {
		return errors.New("retired Agent handoff configuration; upgrade the workflow before starting a new Run")
	}
	if n.RetiredHandoffParams != nil || len(n.RetiredHandoffSchema) > 0 || n.RetiredHandoffInstructions != "" {
		return errors.New("node-wide handoff parameters are retired; configure parameters for each target")
	}
	if n.Kind != "agent" {
		if len(n.HandoffParameters) > 0 {
			return errors.New("handoff settings require an Agent node")
		}
		return nil
	}
	for target, p := range n.HandoffParameters {
		found := false
		for _, edge := range w.Edges {
			if edge.Source == n.ID && edge.Target == target && w.edgeMode(edge) == "handoff" {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("handoff parameters target %q is not an outgoing destination", target)
		}
		if len(p.Instructions) > 16000 {
			return fmt.Errorf("target %s: parameter instructions exceed 16000 bytes", target)
		}
		if p.Enabled && len(p.Schema) == 0 {
			return fmt.Errorf("target %s: enabled params require a JSON Schema", target)
		}
		if len(p.Schema) > 0 {
			if _, err := completionInputsSchema(p); err != nil {
				return fmt.Errorf("target %s: %w", target, err)
			}
		}
	}
	seen := map[string]bool{}
	for _, edge := range w.Edges {
		if edge.Source != n.ID {
			continue
		}
		if w.edgeMode(edge) != "handoff" {
			return errors.New("Agent edges must use handoff")
		}
		if seen[edge.Target] {
			return errors.New("duplicate handoff target; combine its conditions")
		}
		seen[edge.Target] = true
	}
	return nil
}

// Historical definitions remain readable, but cannot dispatch under a changed
// Agent tool contract. Stop/cancel do not call this execution guard.
var errRetiredAgentHandoff = errors.New("this Run uses a retired Agent handoff contract; upgrade the workflow and start a new Run")

func validateWorkflowHandoffs(w Workflow) error {
	for _, n := range w.Nodes {
		if err := validateCompletionConfig(w, n); err != nil {
			return fmt.Errorf("%w: %v", errRetiredAgentHandoff, err)
		}
	}
	return nil
}

func handoffToolSchema(w Workflow, n WorkflowNode) (*jsonschema.Schema, error) {
	if err := validateWorkflowHandoffs(w); err != nil {
		return nil, err
	}
	schema, err := jsonschema.For[HandoffInput](nil)
	if err != nil {
		return nil, err
	}
	schema.Required = []string{"target", "message"}
	schema.Properties["message"].Description = "给下一节点的交接说明：完成事项、未决问题及后续工作。"
	schema.Properties["artifacts"].Description = "需留存的实际产物，使用工作区相对文件路径。"
	delete(schema.Properties, "params")
	targets := []any{}
	for _, edge := range w.Edges {
		if edge.Source != n.ID {
			continue
		}
		targets = append(targets, edge.Target)
		branch := &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"target": {Enum: []any{edge.Target}}}}
		p := n.HandoffParameters[edge.Target]
		if p.Enabled {
			params, err := completionInputsSchema(p)
			if err != nil {
				return nil, err
			}
			params.Description = p.Instructions
			branch.Properties["params"] = params
			branch.Required = []string{"params"}
			schema.Properties["params"] = &jsonschema.Schema{Type: "object", Description: "交给所选target的参数；仅对该目标启用时填写，并符合该目标的Schema和说明。"}
		} else {
			branch.Properties["params"] = &jsonschema.Schema{Not: &jsonschema.Schema{}}
		}
		schema.OneOf = append(schema.OneOf, branch)
	}
	schema.Properties["target"].Enum = targets
	schema.Properties["target"].Description = "必须填写配置允许的目标节点ID，包括唯一目标或结束节点。参数要求按所选target区分。"
	return schema, nil
}

func validateCompletionResult(n WorkflowNode, target string, result NodeResult) error {
	p := n.HandoffParameters[target]
	if !p.Enabled {
		if result.Params != nil {
			return fmt.Errorf("target %s: handoff params are disabled", target)
		}
		return nil
	}
	if result.Params == nil {
		return fmt.Errorf("target %s: params are required by the configured schema", target)
	}
	schema, err := completionInputsSchema(p)
	if err != nil {
		return err
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return err
	}
	if err = resolved.Validate(result.Params); err != nil {
		return fmt.Errorf("target %s: handoff params do not match the configured schema: %w", target, err)
	}
	return nil
}

func completionGuidance(w Workflow, n WorkflowNode) string {
	var b strings.Builder
	for _, edge := range w.Edges {
		if edge.Source != n.ID {
			continue
		}
		p := n.HandoffParameters[edge.Target]
		fmt.Fprintf(&b, "\n\n### target=%s 的参数要求\n", edge.Target)
		if !p.Enabled {
			b.WriteString("此目标不接收params。填写message，并按需列出artifacts。\n")
			continue
		}
		b.WriteString("此目标必须填写params，符合下列JSON Schema；校验不通过时修正工具参数，不能声称已完成交接。\n")
		if p.Instructions != "" {
			b.WriteString("填写说明：" + p.Instructions + "\n")
		}
		b.WriteString("```json\n" + string(p.Schema) + "\n```\n")
	}
	return b.String()
}

// Validate before any SDK map[string]any decoding/re-encoding. Decimal values
// that cannot round-trip must be expressed as strings, not silently rounded.
func completionJSONValue(data []byte) (any, error) {
	if !json.Valid(data) {
		return nil, errors.New("invalid JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var convert func(any) (any, error)
	convert = func(v any) (any, error) {
		switch x := v.(type) {
		case json.Number:
			f, err := strconv.ParseFloat(string(x), 64)
			if err != nil {
				return nil, errors.New("number cannot be represented without loss; use a string")
			}
			// Compare decimal JSON values, not the binary floating-point expansion.
			encoded, _ := json.Marshal(f)
			original, ok := new(big.Rat).SetString(string(x))
			roundtrip, ok2 := new(big.Rat).SetString(string(encoded))
			if !ok || !ok2 || original.Cmp(roundtrip) != 0 {
				return nil, errors.New("number cannot be represented without loss; use a string")
			}
			return f, nil
		case map[string]any:
			for key, item := range x {
				converted, err := convert(item)
				if err != nil {
					return nil, err
				}
				x[key] = converted
			}
		case []any:
			for i, item := range x {
				converted, err := convert(item)
				if err != nil {
					return nil, err
				}
				x[i] = converted
			}
		}
		return v, nil
	}
	return convert(value)
}

func addHandoffTool(server *mcp.Server, tool *mcp.Tool, schema *jsonschema.Schema, handler mcp.ToolHandlerFor[HandoffInput, any]) error {
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return err
	}
	server.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		failed := func(err error) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil
		}
		value, err := completionJSONValue(req.Params.Arguments)
		if err != nil {
			return failed(err)
		}
		if err = resolved.Validate(value); err != nil {
			return failed(err)
		}
		var input HandoffInput
		if err = json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return failed(err)
		}
		result, output, err := handler(ctx, req, input)
		if err != nil {
			return failed(err)
		}
		if result == nil {
			result = &mcp.CallToolResult{}
		}
		if output != nil {
			encoded, err := json.Marshal(output)
			if err != nil {
				return failed(err)
			}
			result.StructuredContent = json.RawMessage(encoded)
			if len(result.Content) == 0 {
				result.Content = []mcp.Content{&mcp.TextContent{Text: string(encoded)}}
			}
		}
		return result, nil
	})
	return nil
}
