package platform

import (
	"errors"
	"strings"
)

// Agent edges represent tool-selected targets; other nodes follow their rules.
func (w Workflow) edgeMode(edge WorkflowEdge) string {
	if edge.Mode != "" {
		return edge.Mode
	}
	if w.node(edge.Source).Kind == "agent" {
		return "handoff"
	}
	return "automatic"
}

type HandoffInput struct {
	Target    string         `json:"target"`
	Message   string         `json:"message"`
	Params    map[string]any `json:"params,omitempty"`
	Artifacts []string       `json:"artifacts,omitempty"`
}

func (s *Store) handoffWorkflowNode(id string, seq int, token string, in HandoffInput) error {
	if strings.TrimSpace(in.Target) == "" || strings.TrimSpace(in.Message) == "" {
		return errors.New("target and message are required")
	}
	r, err := s.WorkflowRun(Caller{Admin: true}, id)
	if err != nil {
		return err
	}
	if r.Seq != seq {
		return ErrConflict
	}
	step := r.Steps[len(r.Steps)-1]
	node := r.Definition.node(step.NodeID)
	routes := []string{}
	for _, edge := range r.Definition.Edges {
		if edge.Source == step.NodeID && edge.Target == in.Target && r.Definition.edgeMode(edge) == "handoff" {
			routes = append(routes, edge.Route)
		}
	}
	if len(routes) != 1 {
		return errors.New("target must identify exactly one configured handoff destination")
	}
	result := NodeResult{Route: routes[0], Summary: in.Message, Params: in.Params, Artifacts: in.Artifacts}
	if err := validateCompletionResult(node, in.Target, result); err != nil {
		return err
	}
	return s.submitWorkflowResult(Caller{}, id, seq, token, result, true)
}
