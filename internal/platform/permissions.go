package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

func nativeApprovalReviewer(a Agent) string {
	if a.ApprovalsReviewer == "" {
		return "user"
	}
	return a.ApprovalsReviewer
}

func validateApprovalReviewer(a Agent) error {
	if reviewer := nativeApprovalReviewer(a); reviewer != "user" && reviewer != "auto_review" {
		return errors.New("approvals_reviewer must be user or auto_review")
	}
	if nativeApprovalReviewer(a) != "auto_review" {
		return nil
	}
	for _, b := range a.ToolServers {
		for _, name := range b.Tools {
			if b.Approvals[name] == "confirm" {
				return errors.New("automatic review conflicts with a tool requiring manual confirmation; select user review or explicitly change that tool's approval setting")
			}
		}
	}
	if needsToolConfirmation(a) {
		return errors.New("automatic review conflicts with a tool requiring manual confirmation; select user review or explicitly change that tool's approval setting")
	}
	return nil
}

func validateNativeReviewConfig(a Agent, config map[string]any) error {
	if nativeApprovalReviewer(a) != "auto_review" {
		return nil
	}
	if _, ok := config["auto_review"]; ok {
		return errors.New("saved native auto_review policy conflicts with managed automatic review; inspect the existing configuration before resuming")
	}
	servers, _ := config["mcp_servers"].(map[string]any)
	for _, value := range servers {
		server, _ := value.(map[string]any)
		tools, _ := server["tools"].(map[string]any)
		for _, value := range tools {
			tool, _ := value.(map[string]any)
			if tool["approval_mode"] == "prompt" {
				return errors.New("native manual tool confirmation conflicts with automatic review; inspect the existing configuration before resuming")
			}
		}
	}
	return nil
}

func nativeApprovalPolicy(a Agent) any {
	// Automatic native review happens before platform permission callbacks. A
	// read-only workspace must not regain writes through that path. Networking
	// for such turns must be pre-authorized through NetworkAccess.
	if a.Sandbox == "read-only" && nativeApprovalReviewer(a) == "auto_review" {
		a.AllowElevation = false
	}
	if !a.AllowElevation && !needsToolConfirmation(a) {
		return "never"
	}
	return map[string]any{"granular": map[string]bool{
		"sandbox_approval": a.AllowElevation, "request_permissions": a.AllowElevation,
		"rules": false, "skill_approval": false, "mcp_elicitations": needsToolConfirmation(a),
	}}
}
func nativeConversationApprovalPolicy(c Conversation) any {
	a := c.Snapshot
	a.Sandbox = nativeSandboxMode(c)
	return nativeApprovalPolicy(a)
}
func nativeSandboxMode(c Conversation) string {
	if c.ReadOnly || c.Snapshot.Sandbox == "read-only" {
		return "read-only"
	}
	return "workspace-write"
}
func turnSandbox(c Conversation) map[string]any {
	kind := "workspaceWrite"
	if nativeSandboxMode(c) == "read-only" {
		kind = "readOnly"
	}
	return map[string]any{"type": kind, "networkAccess": c.Snapshot.NetworkAccess}
}
func isPermissionRequest(method string) bool {
	return method == "item/commandExecution/requestApproval" || method == "item/fileChange/requestApproval" || method == "item/permissions/requestApproval"
}
func (x *Codex) nativePermissionApproval(ctx context.Context, c Conversation, m Message, method string, raw json.RawMessage, thread, turn string, redact func(string) string) (map[string]any, error) {
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	if thread == "" || turn == "" || p["threadId"] != thread || p["turnId"] != turn {
		return nil, errors.New("native permission request belongs to a different turn")
	}
	permissions, _ := p["permissions"].(map[string]any)
	decision := "decline"
	// A read-only handoff workspace must never gain writes through an approval.
	readOnly := nativeSandboxMode(c) == "read-only"
	canAsk := c.Snapshot.AllowElevation && (!readOnly || (method == "item/permissions/requestApproval" && permissions["fileSystem"] == nil))
	if canAsk && x.Approve != nil {
		p["platform_permission_request"] = true
		p["method"] = method
		safe, _ := json.Marshal(p)
		var err error
		decision, err = x.Approve(ctx, c, m, redactJSON(safe, redact))
		if err != nil {
			return nil, err
		}
		if decision != "accept" && decision != "decline" && decision != "cancel" {
			return nil, errors.New("invalid permission decision")
		}
	}
	if method == "item/permissions/requestApproval" {
		grant := map[string]any{}
		if decision == "accept" {
			for k, v := range permissions {
				grant[k] = v
			}
		}
		return map[string]any{"permissions": grant, "scope": "turn"}, nil
	}
	return map[string]any{"decision": decision}, nil
}

// Only copy explicit execution permissions. Model, instructions, tools and native
// thread identity remain the conversation's original snapshot.
func (s *Scheduler) ApplyAgentPermissions(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, active := s.active[id]; active {
		return ErrConflict
	}
	c, err := s.store.Conversation(id)
	if err != nil {
		return err
	}
	a, err := s.store.conversationAgent(c)
	if err != nil {
		return err
	}
	c.Snapshot.NetworkAccess = a.NetworkAccess
	c.Snapshot.AllowElevation = a.AllowElevation
	c.Snapshot.ApprovalsReviewer = a.ApprovalsReviewer
	if _, err := nativeConfig(c.Snapshot); err != nil {
		return err
	}
	return s.saveConversationPermissions(c)
}

func (s *Scheduler) SetConversationPermissions(id string, network, elevation bool, reviewer *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, active := s.active[id]; active {
		return ErrConflict
	}
	c, err := s.store.Conversation(id)
	if err != nil {
		return err
	}
	c.Snapshot.NetworkAccess = network
	c.Snapshot.AllowElevation = elevation
	if reviewer != nil {
		if *reviewer != "user" && *reviewer != "auto_review" {
			return errors.New("approvals_reviewer must be user or auto_review")
		}
		c.Snapshot.ApprovalsReviewer = *reviewer
	}
	if _, err := nativeConfig(c.Snapshot); err != nil {
		return err
	}
	return s.saveConversationPermissions(c)
}

func (s *Scheduler) saveConversationPermissions(c Conversation) error {
	raw, err := json.Marshal(c.Snapshot)
	if err != nil {
		return err
	}
	result, err := s.store.DB.Exec(`UPDATE conversations SET snapshot=?,updated=? WHERE id=? AND status NOT IN ('running','stopping','closed')`, string(raw), now(), c.ID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
func (h *Server) applyAgentPermissions(w http.ResponseWriter, r *http.Request, caller Caller) {
	c, ok := h.authorize(w, r, caller)
	if !ok {
		return
	}
	if err := h.scheduler.ApplyAgentPermissions(c.ID); err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}

func (h *Server) conversationPermissions(w http.ResponseWriter, r *http.Request, caller Caller) {
	c, ok := h.authorize(w, r, caller)
	if !ok {
		return
	}
	var input struct {
		Network   *bool   `json:"network_access"`
		Elevation *bool   `json:"allow_elevation"`
		Reviewer  *string `json:"approvals_reviewer"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	if input.Network == nil || input.Elevation == nil {
		fail(w, errors.New("network_access and allow_elevation are required"))
		return
	}
	if err := h.scheduler.SetConversationPermissions(c.ID, *input.Network, *input.Elevation, input.Reviewer); err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}

// A caller may select a subset of the administrator's current Agent grant.
// Never change a running native turn's permissions underneath its execution.
func (s *Scheduler) SetConversationNetwork(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, active := s.active[id]; active {
		return ErrConflict
	}
	c, err := s.store.Conversation(id)
	if err != nil {
		return err
	}
	a, err := s.store.conversationAgent(c)
	if err != nil {
		return err
	}
	if enabled && (!a.Enabled || !a.NetworkAccess) {
		return ErrForbidden
	}
	c.Snapshot.NetworkAccess = enabled
	c.Snapshot.AllowElevation = enabled && a.AllowElevation
	return s.saveConversationPermissions(c)
}
func (h *Server) conversationNetwork(w http.ResponseWriter, r *http.Request, caller Caller) {
	c, ok := h.authorize(w, r, caller)
	if !ok {
		return
	}
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	if input.Enabled == nil {
		fail(w, errors.New("enabled is required"))
		return
	}
	if err := h.scheduler.SetConversationNetwork(c.ID, *input.Enabled); err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
