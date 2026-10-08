package platform

import "errors"

// Explicit modes apply to new definitions. Old frozen definitions retain their
// original route tool and source-based semantics without a data migration.
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
	Target    string            `json:"target"`
	Route     string            `json:"route,omitempty"`
	Summary   string            `json:"summary"`
	Inputs    map[string]string `json:"inputs,omitempty"`
	Artifacts []string          `json:"artifacts,omitempty"`
}

func (s *Store) handoffWorkflowNode(id string, seq int, token string, in HandoffInput) error {
	r, err := s.WorkflowRun(Caller{Admin: true}, id)
	if err != nil {
		return err
	}
	if r.Seq != seq {
		return ErrConflict
	}
	step := r.Steps[len(r.Steps)-1]
	if r.Definition.node(step.NodeID).ExitMode == "complete" {
		return errors.New("this node uses fixed completion")
	}
	routes := []string{}
	for _, edge := range r.Definition.Edges {
		if edge.Source == step.NodeID && edge.Target == in.Target && r.Definition.edgeMode(edge) == "handoff" && (in.Route == "" || in.Route == edge.Route) {
			routes = append(routes, edge.Route)
		}
	}
	if len(routes) != 1 {
		return errors.New("select one allowed handoff target; specify route when several edges share the same target")
	}
	inputs := map[string]any{}
	for k, v := range in.Inputs {
		inputs[k] = v
	}
	return s.submitWorkflowResult(Caller{}, id, seq, token, NodeResult{Route: routes[0], Summary: in.Summary, Inputs: inputs, Artifacts: in.Artifacts}, true)
}
