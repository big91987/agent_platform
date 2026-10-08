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
	if len(n.CompletionSchema) == 0 {
		return &jsonschema.Schema{Type: "object", AdditionalProperties: &jsonschema.Schema{Type: "string"}}, nil
	}
	if len(n.CompletionSchema) > 32000 {
		return nil, errors.New("completion_schema exceeds 32000 bytes")
	}
	value, err := completionJSONValue(n.CompletionSchema)
	if err != nil {
		return nil, fmt.Errorf("completion_schema: %w", err)
	}
	root, ok := value.(map[string]any)
	if !ok || root["type"] != "object" {
		return nil, errors.New("completion_schema must describe an object with type=object")
	}
	if err := checkCompletionSchema(value, 0); err != nil {
		return nil, err
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(n.CompletionSchema, &schema); err != nil {
		return nil, fmt.Errorf("completion_schema: %w", err)
	}
	if _, err := schema.Resolve(nil); err != nil {
		return nil, fmt.Errorf("completion_schema: %w", err)
	}
	return &schema, nil
}

// Keep the accepted dialect explicit: unknown keywords must not silently weaken
// a contract. No references, remote loaders, formats or default-value injection.
func checkCompletionSchema(value any, depth int) error {
	if depth > 32 {
		return errors.New("completion_schema nesting exceeds 32")
	}
	if _, ok := value.(bool); ok {
		return nil
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return errors.New("completion_schema contains a non-schema value")
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
				return errors.New("completion_schema type must not be empty")
			}
			for _, t := range types {
				name, ok := t.(string)
				if !ok || !slices.Contains([]string{"object", "array", "string", "number", "integer", "boolean", "null"}, name) || seen[name] {
					return errors.New("completion_schema contains an invalid or duplicate type")
				}
				seen[name] = true
			}
		case "minLength", "maxLength", "minItems", "maxItems", "minProperties", "maxProperties":
			num, ok := v.(float64)
			if !ok || num < 0 || num != math.Trunc(num) {
				return fmt.Errorf("completion_schema %s must be a nonnegative integer", key)
			}
		case "multipleOf":
			num, ok := v.(float64)
			if !ok || num <= 0 {
				return errors.New("completion_schema multipleOf must be positive")
			}
		case "required":
			list, ok := v.([]any)
			if !ok {
				return errors.New("completion_schema required must be an array")
			}
			seen := map[string]bool{}
			for _, item := range list {
				name, ok := item.(string)
				if !ok || seen[name] {
					return errors.New("completion_schema required must contain unique strings")
				}
				seen[name] = true
			}
		case "enum":
			list, ok := v.([]any)
			if !ok || len(list) == 0 {
				return errors.New("completion_schema enum must be a nonempty array")
			}
			seen := map[string]bool{}
			for _, item := range list {
				b, _ := json.Marshal(item)
				if seen[string(b)] {
					return errors.New("completion_schema enum must contain unique values")
				}
				seen[string(b)] = true
			}
		case "$schema", "title", "description", "pattern":
			if _, ok := v.(string); !ok {
				return fmt.Errorf("completion_schema %s must be a string", key)
			}
		case "examples":
			if _, ok := v.([]any); !ok {
				return errors.New("completion_schema examples must be an array")
			}
		case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum":
			if _, ok := v.(float64); !ok {
				return fmt.Errorf("completion_schema %s must be a number", key)
			}
		case "uniqueItems":
			if _, ok := v.(bool); !ok {
				return errors.New("completion_schema uniqueItems must be a boolean")
			}
		case "const":
		case "properties":
			props, ok := v.(map[string]any)
			if !ok {
				return errors.New("completion_schema properties must be an object")
			}
			for _, child := range props {
				if err := checkCompletionSchema(child, depth+1); err != nil {
					return err
				}
			}
		case "items", "additionalProperties", "not":
			if err := checkCompletionSchema(v, depth+1); err != nil {
				return err
			}
		case "allOf", "anyOf", "oneOf":
			list, ok := v.([]any)
			if !ok || len(list) == 0 {
				return fmt.Errorf("completion_schema %s must be a nonempty array", key)
			}
			for _, child := range list {
				if err := checkCompletionSchema(child, depth+1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("completion_schema keyword %q is not supported", key)
		}
	}
	return nil
}

func validateCompletionConfig(w Workflow, n WorkflowNode) error {
	if n.Kind != "agent" {
		if n.ExitMode != "" || len(n.CompletionSchema) > 0 || n.CompletionInstructions != "" {
			return errors.New("completion settings require an Agent node")
		}
		return nil
	}
	if len(n.CompletionInstructions) > 16000 {
		return errors.New("completion_instructions exceeds 16000 bytes")
	}
	if n.ExitMode != "" && n.ExitMode != "handoff" && n.ExitMode != "complete" {
		return errors.New("exit_mode must be handoff or complete")
	}
	if (n.ExitMode != "" || len(n.CompletionSchema) > 0 || n.CompletionInstructions != "") && w.ContextVersion != 2 {
		return errors.New("Agent exit settings require context_version=2")
	}
	if len(n.CompletionSchema) > 0 {
		if _, err := completionInputsSchema(n); err != nil {
			return err
		}
	}
	if n.ExitMode == "" {
		if len(n.CompletionSchema) > 0 || n.CompletionInstructions != "" {
			return errors.New("select exit_mode before configuring completion output")
		}
		return nil // frozen legacy and earlier v2 graphs retain their original edges
	}
	count := 0
	for _, edge := range w.Edges {
		if edge.Source != n.ID {
			continue
		}
		count++
		expected := "handoff"
		if n.ExitMode == "complete" {
			expected = "automatic"
		}
		if w.edgeMode(edge) != expected {
			return fmt.Errorf("node %s: outgoing edge conflicts with exit_mode", n.ID)
		}
	}
	if n.ExitMode == "complete" && count != 1 {
		return errors.New("fixed completion requires exactly one target")
	}
	return nil
}

func completionToolSchema(n WorkflowNode) (*jsonschema.Schema, error) {
	inputs, err := completionInputsSchema(n)
	if err != nil {
		return nil, err
	}
	schema, err := jsonschema.For[NodeResult](nil)
	if err != nil {
		return nil, err
	}
	schema.Required = []string{"summary"}
	schema.Properties["summary"].Description = "真实完成结论、未决问题及版本。"
	schema.Properties["artifacts"].Description = "实际产物的工作区相对路径。"
	schema.Properties["inputs"] = inputs
	if len(n.CompletionSchema) > 0 {
		schema.Required = append(schema.Required, "inputs")
	}
	if n.ExitMode == "complete" {
		delete(schema.Properties, "route")
	}
	return schema, nil
}

func validateCompletionResult(n WorkflowNode, result NodeResult) error {
	schema, err := completionInputsSchema(n)
	if err != nil {
		return err
	}
	if result.Inputs == nil && len(n.CompletionSchema) == 0 {
		return nil
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return err
	}
	if err = resolved.Validate(result.Inputs); err != nil {
		return fmt.Errorf("complete_node inputs do not match the configured schema: %w", err)
	}
	return nil
}

func completionGuidance(n WorkflowNode) string {
	var b strings.Builder
	if n.CompletionInstructions != "" {
		b.WriteString("\n\n## 固定流转输出说明\n" + n.CompletionInstructions)
	}
	if len(n.CompletionSchema) > 0 {
		b.WriteString("\n\n## 固定流转输出格式\ncomplete_node 的 inputs 必须符合以下 JSON Schema；summary 提交真实结论，artifacts 列出实际产物路径。校验不通过时修正工具参数后重试，不得声称完成。\n```json\n" + string(n.CompletionSchema) + "\n```")
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

func addCompletionTool(server *mcp.Server, tool *mcp.Tool, schema *jsonschema.Schema, handler mcp.ToolHandlerFor[NodeResult, any]) error {
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
		var input NodeResult
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
