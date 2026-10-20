package platform

import (
	"encoding/json"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
)

func (h *Server) workflowRuleRoutes() {
	h.mux.HandleFunc("POST /api/workflows/rules/preview", h.protect(true, func(w http.ResponseWriter, r *http.Request, c Caller) {
		var in struct {
			Workflow Workflow        `json:"workflow"`
			NodeID   string          `json:"node_id"`
			Output   json.RawMessage `json:"output,omitempty"`
		}
		if err := decode(w, r, &in); err != nil {
			fail(w, err)
			return
		}
		n := in.Workflow.node(in.NodeID)
		if n.RuleHandoff == nil {
			fail(w, errors.New("select a rule-based execution node"))
			return
		}
		if err := validateRuleHandoff(in.Workflow, n); err != nil {
			fail(w, err)
			return
		}
		if len(in.Output) == 0 {
			respond(w, 200, map[string]any{"valid": true})
			return
		}
		value, err := completionJSONValue(in.Output)
		if err != nil {
			fail(w, err)
			return
		}
		output, ok := value.(map[string]any)
		if !ok {
			fail(w, errors.New("output must be a JSON object"))
			return
		}
		receipt := ConnectorReceipt{}
		if v, exists := output["result"]; exists {
			receipt.Result, _ = json.Marshal(v)
		} else {
			receipt.ResultError = "output.result is missing"
		}
		if n.Execution.Mode == "mcp" {
			v, ok := output["is_error"].(bool)
			if !ok {
				fail(w, errors.New("output.is_error must be boolean"))
				return
			}
			receipt.MCP = &mcp.CallToolResult{IsError: v}
		} else {
			v, ok := output["exit_code"].(float64)
			if !ok || v != float64(int(v)) {
				fail(w, errors.New("output.exit_code must be integer"))
				return
			}
			i := int(v)
			receipt.ExitCode = &i
		}
		route, summary, err := routeExecutionRules(n, receipt)
		if err != nil {
			respond(w, 200, map[string]any{"valid": true, "error": err.Error(), "action": "stop"})
			return
		}
		in.Workflow.projectRuleEdges()
		respond(w, 200, map[string]any{"valid": true, "route": route, "summary": summary, "target": in.Workflow.target(n.ID, route)})
	}))
}
