package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

// Rules belong to a node. Edges are a deterministic projection for the canvas
// and the existing scheduler; a line never owns an independent condition.
type WorkflowRules struct {
	// Empty scope preserves persisted result-only schemas and frozen runs.
	SchemaScope  string              `json:"schema_scope,omitempty"`
	Schema       json.RawMessage     `json:"schema"`
	Rules        []WorkflowRule      `json:"rules"`
	OnInvalid    WorkflowRuleFailure `json:"on_invalid"`
	OnExpression WorkflowRuleFailure `json:"on_expression"`
}
type WorkflowRule struct {
	When    string `json:"when,omitempty"`
	Else    bool   `json:"else,omitempty"`
	Title   string `json:"title"`
	Content string `json:"content,omitempty"`
	Target  string `json:"target"`
}
type WorkflowRuleFailure struct {
	Action  string `json:"action"`
	Target  string `json:"target,omitempty"`
	Title   string `json:"title,omitempty"`
	Content string `json:"content,omitempty"`
}

func ruleRoute(i int) string { return fmt.Sprintf("rule_%d", i+1) }
func ruleEdges(n WorkflowNode) []WorkflowEdge {
	if n.RuleHandoff == nil {
		return nil
	}
	var edges []WorkflowEdge
	for i, r := range n.RuleHandoff.Rules {
		edges = append(edges, WorkflowEdge{Source: n.ID, Route: ruleRoute(i), Target: r.Target, Mode: "automatic", Description: r.Content, Title: r.Title})
	}
	for _, v := range []struct {
		key string
		f   WorkflowRuleFailure
	}{{"invalid", n.RuleHandoff.OnInvalid}, {"expression", n.RuleHandoff.OnExpression}} {
		if v.f.Action == "route" {
			edges = append(edges, WorkflowEdge{Source: n.ID, Route: "rule_" + v.key, Target: v.f.Target, Mode: "automatic", Title: v.f.Title, Description: v.f.Content})
		}
	}
	return edges
}
func (w *Workflow) projectRuleEdges() {
	edges := make([]WorkflowEdge, 0, len(w.Edges))
	for _, e := range w.Edges {
		n := w.node(e.Source)
		if n.RuleHandoff == nil {
			edges = append(edges, e)
		}
	}
	for _, n := range w.Nodes {
		edges = append(edges, ruleEdges(n)...)
	}
	w.Edges = edges
}
func ruleSchema(raw json.RawMessage) (*jsonschema.Resolved, error) {
	if len(raw) == 0 || len(raw) > 32000 {
		return nil, errors.New("JSON Schema is required (up to 32000 bytes)")
	}
	value, err := completionJSONValue(raw)
	if err != nil {
		return nil, err
	}
	if err = checkCompletionSchema(value, 0); err != nil {
		return nil, errors.New(strings.ReplaceAll(err.Error(), "completion_schema", "JSON Schema"))
	}
	var schema jsonschema.Schema
	if err = json.Unmarshal(raw, &schema); err != nil {
		return nil, err
	}
	return schema.Resolve(nil)
}
func ruleTypes(p *WorkflowRules, mode string) (map[string]string, error) {
	out := map[string]string{}
	if p.SchemaScope != "input" && mode == "mcp" {
		out["output.is_error"] = "boolean"
	} else if p.SchemaScope != "input" {
		out["output.exit_code"] = "number"
	}
	var schema map[string]any
	if err := json.Unmarshal(p.Schema, &schema); err != nil {
		return nil, err
	}
	var walk func(map[string]any, string)
	walk = func(s map[string]any, path string) {
		t, _ := s["type"].(string)
		if t == "integer" {
			t = "number"
		}
		if t != "" && path != "" {
			out[path] = t
		}
		props, _ := s["properties"].(map[string]any)
		for k, v := range props {
			if child, ok := v.(map[string]any); ok && token.IsIdentifier(k) {
				next := k
				if path != "" {
					next = path + "." + k
				}
				walk(child, next)
			}
		}
	}
	if p.SchemaScope == "input" {
		walk(schema, "")
	} else {
		walk(schema, "output.result")
	}
	return out, nil
}
func validateRuleHandoff(w Workflow, n WorkflowNode) error {
	p := n.RuleHandoff
	if p == nil {
		return nil
	}
	if n.Kind != "execution" || n.Execution == nil {
		return errors.New("rule handoff requires an execution node")
	}
	if p.SchemaScope != "" && p.SchemaScope != "input" {
		return errors.New("unsupported handoff schema scope")
	}
	if _, err := ruleSchema(p.Schema); err != nil {
		return fmt.Errorf("handoff schema: %w", err)
	}
	if len(p.Rules) > 64 {
		return errors.New("at most 64 handoff rules")
	}
	types, err := ruleTypes(p, n.Execution.Mode)
	if err != nil {
		return err
	}
	target := func(id string) bool { return id != n.ID && w.node(id).ID != "" }
	for i, r := range p.Rules {
		if !target(r.Target) || strings.TrimSpace(r.Title) == "" || len(r.Title) > 200 || len(r.Content) > 4000 {
			return fmt.Errorf("rule %d needs a title and valid target", i+1)
		}
		if r.Else {
			if i != len(p.Rules)-1 || r.When != "" {
				return errors.New("else must be last and have no expression")
			}
		} else if _, err := parseRule(r.When, types); err != nil {
			return fmt.Errorf("rule %d: %w", i+1, err)
		}
	}
	for _, f := range []WorkflowRuleFailure{p.OnInvalid, p.OnExpression} {
		if f.Action != "stop" && f.Action != "route" {
			return errors.New("choose stop or route for rule errors")
		}
		if f.Action == "stop" && f.Target != "" {
			return errors.New("stop cannot have a target")
		}
		if f.Action == "route" && (!target(f.Target) || strings.TrimSpace(f.Title) == "") {
			return errors.New("error route needs a title and valid target")
		}
		if len(f.Title) > 200 || len(f.Content) > 4000 {
			return errors.New("error route text exceeds limit")
		}
	}
	return nil
}

// A deliberately bounded expression language: scalar literals, field paths,
// comparisons, boolean operations and parentheses. No calls or code execution.
func parseRule(text string, types map[string]string) (ast.Expr, error) {
	if strings.TrimSpace(text) == "" || len(text) > 4000 {
		return nil, errors.New("condition is required (up to 4000 bytes)")
	}
	x, err := parser.ParseExpr(text)
	if err != nil {
		return nil, errors.New("invalid condition syntax")
	}
	t, err := ruleType(x, types, 0)
	if err != nil {
		return nil, err
	}
	if t != "boolean" {
		return nil, errors.New("condition must return boolean")
	}
	return x, nil
}
func rulePath(x ast.Expr) string {
	switch v := x.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		base := rulePath(v.X)
		if base != "" {
			return base + "." + v.Sel.Name
		}
	}
	return ""
}
func ruleType(x ast.Expr, types map[string]string, depth int) (string, error) {
	if depth > 64 {
		return "", errors.New("condition nesting exceeds 64")
	}
	bad := func() (string, error) { return "", errors.New("unsupported operation or incompatible types") }
	switch v := x.(type) {
	case *ast.ParenExpr:
		return ruleType(v.X, types, depth+1)
	case *ast.BasicLit:
		if v.Kind == token.STRING && strings.HasPrefix(v.Value, "\"") {
			if _, err := strconv.Unquote(v.Value); err == nil {
				return "string", nil
			}
		}
		if v.Kind == token.INT || v.Kind == token.FLOAT {
			if _, err := strconv.ParseFloat(v.Value, 64); err == nil {
				return "number", nil
			}
		}
		return bad()
	case *ast.Ident:
		if v.Name == "true" || v.Name == "false" {
			return "boolean", nil
		}
		if v.Name == "null" {
			return "null", nil
		}
	case *ast.UnaryExpr:
		t, err := ruleType(v.X, types, depth+1)
		if err != nil {
			return "", err
		}
		if v.Op == token.NOT && t == "boolean" || v.Op == token.SUB && t == "number" {
			return t, nil
		}
		return bad()
	case *ast.BinaryExpr:
		a, err := ruleType(v.X, types, depth+1)
		if err != nil {
			return "", err
		}
		b, err := ruleType(v.Y, types, depth+1)
		if err != nil {
			return "", err
		}
		if a != b {
			return bad()
		}
		switch v.Op {
		case token.LAND, token.LOR:
			if a == "boolean" {
				return "boolean", nil
			}
		case token.EQL, token.NEQ:
			if a == "number" || a == "boolean" || a == "string" || a == "null" {
				return "boolean", nil
			}
		case token.GTR, token.GEQ, token.LSS, token.LEQ:
			if a == "number" {
				return "boolean", nil
			}
		}
		return bad()
	}
	if p := rulePath(x); p != "" {
		if t, ok := types[p]; ok {
			return t, nil
		}
		return "", fmt.Errorf("undeclared field: %s", p)
	}
	return bad()
}
func ruleScalar(value any) bool {
	switch value.(type) {
	case nil, bool, float64, string:
		return true
	}
	return false
}
func evalRule(x ast.Expr, values map[string]any) (any, error) {
	switch v := x.(type) {
	case *ast.ParenExpr:
		return evalRule(v.X, values)
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			return strconv.Unquote(v.Value)
		}
		return strconv.ParseFloat(v.Value, 64)
	case *ast.Ident:
		if v.Name == "true" {
			return true, nil
		}
		if v.Name == "false" {
			return false, nil
		}
		if v.Name == "null" {
			return nil, nil
		}
	case *ast.UnaryExpr:
		a, err := evalRule(v.X, values)
		if err != nil {
			return nil, err
		}
		if v.Op == token.NOT {
			if b, ok := a.(bool); ok {
				return !b, nil
			}
			return nil, errors.New("condition expected boolean")
		}
		if b, ok := a.(float64); ok {
			return -b, nil
		}
		return nil, errors.New("condition expected number")
	case *ast.BinaryExpr:
		a, err := evalRule(v.X, values)
		if err != nil {
			return nil, err
		}
		if v.Op == token.LAND || v.Op == token.LOR {
			boolean, ok := a.(bool)
			if !ok {
				return nil, errors.New("condition expected boolean")
			}
			if v.Op == token.LAND && !boolean {
				return false, nil
			}
			if v.Op == token.LOR && boolean {
				return true, nil
			}
		}
		b, err := evalRule(v.Y, values)
		if err != nil {
			return nil, err
		}
		if (v.Op == token.EQL || v.Op == token.NEQ) && (!ruleScalar(a) || !ruleScalar(b)) {
			return nil, errors.New("condition expected scalar values")
		}
		if v.Op == token.GTR || v.Op == token.GEQ || v.Op == token.LSS || v.Op == token.LEQ {
			if _, ok := a.(float64); !ok {
				return nil, errors.New("condition expected number")
			}
			if _, ok := b.(float64); !ok {
				return nil, errors.New("condition expected number")
			}
		}
		switch v.Op {
		case token.LAND, token.LOR:
			if _, ok := b.(bool); !ok {
				return nil, errors.New("condition expected boolean")
			}
			return b, nil
		case token.EQL:
			return a == b, nil
		case token.NEQ:
			return a != b, nil
		case token.GTR:
			return a.(float64) > b.(float64), nil
		case token.GEQ:
			return a.(float64) >= b.(float64), nil
		case token.LSS:
			return a.(float64) < b.(float64), nil
		case token.LEQ:
			return a.(float64) <= b.(float64), nil
		}
	}
	path := rulePath(x)
	var value any = values
	for _, part := range strings.Split(path, ".") {
		m, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("field unavailable: %s", path)
		}
		value, ok = m[part]
		if !ok {
			return nil, fmt.Errorf("field unavailable: %s", path)
		}
	}
	return value, nil
}

// Evaluate only known completed actions. Infrastructure errors never become a
// business else. Missing/invalid data and condition errors have separate exits.
func routeExecutionRules(n WorkflowNode, receipt ConnectorReceipt) (string, string, error) {
	p := n.RuleHandoff
	fail := func(key string, f WorkflowRuleFailure, err error) (string, string, error) {
		if f.Action == "route" {
			return "rule_" + key, err.Error(), nil
		}
		return "", "", err
	}
	var result any
	if receipt.ResultError != "" {
		return fail("invalid", p.OnInvalid, errors.New(receipt.ResultError))
	}
	if len(receipt.Result) > 0 {
		var err error
		result, err = completionJSONValue(receipt.Result)
		if err != nil {
			return fail("invalid", p.OnInvalid, err)
		}
	}
	output := map[string]any{"result": result}
	if n.Execution.Mode == "mcp" {
		if receipt.MCP == nil {
			return "", "", errors.New("MCP result unavailable")
		}
		output["is_error"] = receipt.MCP.IsError
	} else {
		if receipt.ExitCode == nil {
			return "", "", errors.New("exit code unavailable")
		}
		output["exit_code"] = float64(*receipt.ExitCode)
	}
	return routeRuleInput(n, map[string]any{"output": output})
}

// Runtime receipts and preview samples use the same schema and expression input.
func routeRuleInput(n WorkflowNode, input any) (string, string, error) {
	p := n.RuleHandoff
	fail := func(key string, f WorkflowRuleFailure, err error) (string, string, error) {
		if f.Action == "route" {
			return "rule_" + key, err.Error(), nil
		}
		return "", "", err
	}
	schema, err := ruleSchema(p.Schema)
	if err != nil {
		return "", "", err
	}
	values, _ := input.(map[string]any)
	var value any = input
	name := "handoff input"
	if p.SchemaScope != "input" {
		output, _ := values["output"].(map[string]any)
		value = output["result"]
		name = "output.result"
	}
	if err = schema.Validate(value); err != nil {
		return fail("invalid", p.OnInvalid, fmt.Errorf("%s does not match JSON Schema: %w", name, err))
	}
	types, err := ruleTypes(p, n.Execution.Mode)
	if err != nil {
		return "", "", err
	}
	for i, r := range p.Rules {
		if r.Else {
			return ruleRoute(i), r.Title, nil
		}
		x, err := parseRule(r.When, types)
		if err != nil {
			return fail("expression", p.OnExpression, err)
		}
		hit, err := evalRule(x, values)
		if err != nil {
			return fail("expression", p.OnExpression, err)
		}
		if hit == true {
			return ruleRoute(i), r.Title, nil
		}
	}
	return "", "", errors.New("no handoff condition matched and no else is configured; inspect the receipt before continuing")
}
