package platform

import (
	"database/sql"
	"errors"
)

const sandboxConversationRunSQL = `SELECT CASE WHEN count(DISTINCT ws.run_id)=0 THEN '' WHEN count(DISTINCT ws.run_id)=1 THEN max(ws.run_id) ELSE 'ambiguous' END FROM workflow_steps ws JOIN workflow_run_environments re ON re.run_id=ws.run_id WHERE ws.conversation_id=?`

func sandboxConversationRunID(row *sql.Row) (string, error) {
	var id string
	if err := row.Scan(&id); err != nil {
		return "", err
	}
	if id != "" && !validSandboxRun(id) {
		return "", errors.New("conversation has conflicting Run environment associations")
	}
	return id, nil
}

// The existing workflow-step foreign key identifies the environment. No second
// task identity or independent conversation/environment registry is maintained.
func (s *Store) sandboxRunForConversation(id string) (*WorkflowRun, error) {
	runID, err := sandboxConversationRunID(s.DB.QueryRow(sandboxConversationRunSQL, id))
	if err != nil || runID == "" {
		return nil, err
	}
	run, err := s.WorkflowRun(Caller{Admin: true}, runID)
	if err != nil {
		return nil, err
	}
	if run.Environment == nil || run.Definition.Environment != "opensandbox" || run.WorkspacePath != "/workspace" {
		return nil, errors.New("conversation Run environment is inconsistent")
	}
	return &run, nil
}

func (s *Store) conversationWorkspaceKey(c Conversation) (string, error) {
	id, err := sandboxConversationRunID(s.DB.QueryRow(sandboxConversationRunSQL, c.ID))
	if err != nil {
		return "", err
	}
	if id != "" {
		return "sandbox:" + id, nil
	}
	return c.WorkspacePath, nil
}

// Aliases are fixed internal SQL identifiers. A path inside one Run's container
// cannot reserve the same spelling in another container or on the host.
func workspaceReservationSQL(alias string) string {
	return `COALESCE((SELECT 'sandbox:'||max(ws.run_id) FROM workflow_steps ws JOIN workflow_run_environments re ON re.run_id=ws.run_id WHERE ws.conversation_id=` + alias + `.id GROUP BY ws.conversation_id),'local:'||` + alias + `.workspace_path)`
}
