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
func completionInputsSchema(n WorkflowNode) (*jsonschema.Schema, error) {
	if len(n.HandoffSchema) == 0 {
		return &jsonschema.Schema{Type: "object", AdditionalProperties: &jsonschema.Schema{}}, nil
	}
	if len(n.HandoffSchema) > 32000 {
		return nil, errors.New("handoff_schema exceeds 32000 bytes")
	}
	value, err := completionJSONValue(n.HandoffSchema)
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
	if err := json.Unmarshal(n.HandoffSchema, &schema); err != nil {
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
	if n.ExitMode != "" || len(n.RetiredCompletionSchema) > 0 || n.RetiredCompletionInstructions != "" {
		return errors.New("retired Agent handoff configuration; upgrade the workflow before starting a new Run")
	}
	if n.Kind != "agent" {
		if len(n.HandoffSchema) > 0 || n.HandoffInstructions != "" {
			return errors.New("handoff settings require an Agent node")
		}
		return nil
	}
	if len(n.HandoffInstructions) > 16000 {
		return errors.New("handoff_instructions exceeds 16000 bytes")
	}
	if len(n.HandoffSchema) > 0 {
		if _, err := completionInputsSchema(n); err != nil {
			return err
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
	contextSchema, err := completionInputsSchema(n)
	if err != nil {
		return nil, err
	}
	schema, err := jsonschema.For[HandoffInput](nil)
	if err != nil {
		return nil, err
	}
	schema.Required = []string{"target", "message"}
	schema.Properties["message"].Description = "给下一节点的交接说明：完成事项、未决问题及后续工作。"

	schema.Properties["context"] = contextSchema
	schema.Properties["artifacts"].Description = "需留存的实际产物，使用工作区相对文件路径。"
	targets := []any{}
	for _, edge := range w.Edges {
		if edge.Source == n.ID {
			targets = append(targets, edge.Target)
		}
	}
	schema.Properties["target"].Enum = targets
	schema.Properties["target"].Description = "必须填写配置允许的目标节点ID，包括唯一目标或结束节点。"
	if len(n.HandoffSchema) > 0 {
		schema.Required = append(schema.Required, "context")
	}
	return schema, nil
}

func validateCompletionResult(n WorkflowNode, result NodeResult) error {
	schema, err := completionInputsSchema(n)
	if err != nil {
		return err
	}
	if result.Inputs == nil && len(n.HandoffSchema) == 0 {
		return nil
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return err
	}
	if err = resolved.Validate(result.Inputs); err != nil {
		return fmt.Errorf("handoff context do not match the configured schema: %w", err)
	}
	return nil
}

func completionGuidance(n WorkflowNode) string {
	var b strings.Builder
	if n.HandoffInstructions != "" {
		b.WriteString("\n\n## 交接信息填写说明\n" + n.HandoffInstructions)
	}
	if len(n.HandoffSchema) > 0 {
		b.WriteString("\n\n## 交接信息格式\nhandoff 的 context 必须符合以下 JSON Schema；message 提交交接说明，artifacts 列出实际产物路径。校验不通过时修正工具参数后重试，不得声称完成。\n```json\n" + string(n.HandoffSchema) + "\n```")
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
