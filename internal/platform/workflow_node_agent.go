package platform

import (
	"encoding/json"
	"errors"
)

// Node configuration reuses native execution settings, never a shared Agent's
// identity or user grants. Workflow authorization owns this execution boundary.
func validateWorkflowAgent(q rowQuerier, a Agent) error {
	if a.ID != "" || a.Name != "" || a.Enabled || len(a.AuthorizedUsers) > 0 || len(a.ResolvedTools) > 0 {
		return errors.New("node agent accepts execution settings only; identity and authorization belong to the workflow")
	}
	if a.Executor != "codex" {
		return errors.New("only Codex is supported")
	}
	if _, err := nativeConfig(a); err != nil {
		return err
	}
	if _, err := executorEnv(a, "managed"); err != nil {
		return err
	}
	for _, p := range a.Skills {
		if _, err := normalizeSkill(p); err != nil {
			return err
		}
	}
	_, err := resolveToolServers(q, a)
	return err
}

func nodeAgentAllowed(c Caller, n WorkflowNode) bool { return n.Agent != nil || allowed(c, n.AgentID) }

// Called with the caller's owner check in addition to the current workflow grant.
// A blank agent_id alone never grants access: there must be a persisted Run step.
func conversationAgentAllowed(q rowQuerier, c Caller, id, agentID string) bool {
	if c.Admin {
		return true
	}
	if agentID != "" {
		return allowed(c, agentID)
	}
	var raw, owner string
	if q.QueryRow(`SELECT w.definition,r.owner FROM workflow_steps s JOIN workflow_runs r ON r.id=s.run_id JOIN workflows w ON w.id=r.workflow_id WHERE s.conversation_id=?`, id).Scan(&raw, &owner) != nil {
		return false
	}
	var w Workflow
	return json.Unmarshal([]byte(raw), &w) == nil && owner == c.UserID && workflowAllowed(c, w)
}

// Permission reset/subset controls use the Run's frozen node grant, not another
// workflow's shared role or a later revision of this workflow.
func (s *Store) conversationAgent(c Conversation) (Agent, error) {
	if c.AgentID != "" {
		return s.Agent(c.AgentID)
	}
	var raw, nodeID string
	if err := s.DB.QueryRow(`SELECT r.definition,ws.node_id FROM workflow_steps ws JOIN workflow_runs r ON r.id=ws.run_id WHERE ws.conversation_id=?`, c.ID).Scan(&raw, &nodeID); err != nil {
		return Agent{}, err
	}
	var w Workflow
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		return Agent{}, err
	}
	n := w.node(nodeID)
	if n.Agent == nil {
		return Agent{}, ErrNotFound
	}
	a := *n.Agent
	a.Enabled = true
	return a, nil
}

// HTTP copies omit administrator-only execution configuration for ordinary
// callers, while persistence and runtime continue to hold the complete graph.
func workflowHTTPValue(c Caller, value any) any {
	if c.Admin {
		return value
	}
	clean := func(w Workflow) Workflow {
		w.Nodes = append([]WorkflowNode(nil), w.Nodes...)
		for i, n := range w.Nodes {
			if n.Agent != nil {
				w.Nodes[i].Agent = &Agent{Executor: n.Agent.Executor, Model: n.Agent.Model, Sandbox: n.Agent.Sandbox, NetworkAccess: n.Agent.NetworkAccess, AllowElevation: n.Agent.AllowElevation, ApprovalsReviewer: n.Agent.ApprovalsReviewer}
			}
		}
		return w
	}
	switch v := value.(type) {
	case Workflow:
		return clean(v)
	case []Workflow:
		out := make([]Workflow, len(v))
		for i, w := range v {
			out[i] = clean(w)
		}
		return out
	case WorkflowRun:
		v.Definition = clean(v.Definition)
		return v
	case []WorkflowRun:
		out := append([]WorkflowRun(nil), v...)
		for i := range out {
			out[i].Definition = clean(out[i].Definition)
		}
		return out
	default:
		return value
	}
}
