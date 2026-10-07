package platform

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
)

func (h *Server) workflowContextRoutes() {
	h.mux.HandleFunc("POST /api/workflows/preview", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		var in struct {
			Workflow Workflow `json:"workflow"`
			NodeID   string   `json:"node_id"`
		}
		if err := decode(w, r, &in); err != nil {
			fail(w, err)
			return
		}
		if !c.Admin {
			if in.Workflow.ID == "" {
				fail(w, ErrForbidden)
				return
			}
			if _, err := h.store.Workflow(c, in.Workflow.ID); err != nil {
				fail(w, err)
				return
			}
		}
		if err := validateWorkflow(in.Workflow); err != nil {
			fail(w, err)
			return
		}
		n := in.Workflow.node(in.NodeID)
		if n.Kind != "agent" {
			fail(w, errors.New("preview requires an Agent node"))
			return
		}
		prompt, err := workflowSessionPrompt(in.Workflow, n)
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, map[string]any{"session_prompt": prompt, "input": prompt + "\n\n## 当前任务\n运行时追加用户任务、修正及必要上游回执。", "context_version": in.Workflow.ContextVersion})
	}))
	h.mux.HandleFunc("GET /api/workflow-runs/{id}/steps/{seq}/context", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		run, err := h.store.WorkflowRun(c, r.PathValue("id"))
		if err != nil {
			fail(w, err)
			return
		}
		seq, err := strconv.Atoi(r.PathValue("seq"))
		if err != nil {
			fail(w, ErrNotFound)
			return
		}
		var step WorkflowStep
		for _, v := range run.Steps {
			if v.Seq == seq {
				step = v
				break
			}
		}
		if step.ConversationID == "" {
			fail(w, ErrNotFound)
			return
		}
		conv, err := h.store.Conversation(step.ConversationID)
		if err != nil {
			fail(w, err)
			return
		}
		msgs, err := h.store.Messages(conv.ID)
		if err != nil {
			fail(w, err)
			return
		}
		input := ""
		if len(msgs) > 0 {
			input = msgs[0].Content
		}
		tools := []string{}
		for server, binding := range conv.Snapshot.ResolvedTools {
			for _, name := range binding.Tools {
				tools = append(tools, server+"."+name)
			}
		}
		sort.Strings(tools)
		project := []string{fmt.Sprintf("%s 原生项目指令发现，工作目录：%s；平台不复制或修改 AGENTS.md", conv.Snapshot.Executor, conv.workspace(h.store.Dir))}
		if conv.Snapshot.Executor != "codex" {
			project = []string{"当前执行器的项目指令发现能力未在本次验收验证"}
		}
		// Private native options/env/auth never enter this endpoint. Reuse the executor
		// credential redactor even for administrator inspection.
		redact := func(text string) string { return text }
		if h.codex != nil {
			redact = h.codex.redactor(conv.Snapshot)
		}
		respond(w, 200, map[string]any{"role_instructions": redact(conv.Snapshot.Instructions), "project_instructions": project, "tools": tools, "input": redact(input), "context_version": run.Definition.ContextVersion, "workflow_revision": run.Definition.Revision, "executor": conv.Snapshot.Executor, "model": conv.Snapshot.Model, "native_record": filepath.Base(conv.ThreadID)})
	}))
}
