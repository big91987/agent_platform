package platform

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (h *Server) workflowRunRoutes() {
	h.workflowContextRoutes()
	h.connectorRoutes()
	h.workflowHookRoutes()
	h.mux.HandleFunc("GET /api/workflow-runs", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		v, e := h.store.WorkflowRunsPage(c, r.URL.Query().Get("workflow_id"), r.URL.Query().Get("before"))
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, workflowHTTPValue(c, v))
	}))
	h.mux.HandleFunc("GET /api/workflow-runs/by-request", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		v, err := h.store.WorkflowRunByRequest(c, r.URL.Query().Get("request_id"))
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, workflowHTTPValue(c, v))
	}))
	h.mux.HandleFunc("POST /api/workflow-runs/{id}/messages", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		var in struct {
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
			Seq       int    `json:"seq,omitempty"`
		}
		if err := decode(w, r, &in); err != nil {
			fail(w, err)
			return
		}
		v, err := h.store.Submit(c, Input{UserID: c.UserID, WorkflowRunID: r.PathValue("id"), WorkflowSeq: in.Seq, Message: in.Message, RequestID: in.RequestID})
		if err != nil {
			fail(w, err)
			return
		}
		v.ConversationURL = h.base + "/conversations/" + v.ConversationID
		respond(w, 202, workflowHTTPValue(c, v))
	}))
	h.mux.HandleFunc("GET /api/workflow-run-groups", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		groups, err := h.store.WorkflowRunGroups(c)
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, groups)
	}))
	h.mux.HandleFunc("POST /api/workflow-runs", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		var in WorkflowStart
		if e := decode(w, r, &in); e != nil {
			fail(w, e)
			return
		}
		v, e := h.store.StartWorkflow(c, in)
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 201, workflowHTTPValue(c, v))
	}))
	h.mux.HandleFunc("GET /api/workflow-runs/{id}", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		v, e := h.store.WorkflowRun(c, r.PathValue("id"))
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, workflowHTTPValue(c, v))
	}))
	h.mux.HandleFunc("GET /api/workflow-runs/{id}/steps/{seq}/output", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
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
		offset := int64(0)
		if value := r.URL.Query().Get("offset"); value != "" {
			offset, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				fail(w, ErrNotFound)
				return
			}
		}
		page, err := h.store.connectorOutput(run, seq, offset)
		if err != nil {
			fail(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Query().Get("format") != "text" {
			respond(w, 200, page)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		for {
			if _, err := w.Write([]byte(page.Output)); err != nil {
				return
			}
			if page.EOF {
				break
			}
			page, err = h.store.connectorOutput(run, seq, page.NextOffset)
			if err != nil {
				w.Write([]byte("\n[command log read interrupted; output is incomplete]\n"))
				return
			}
		}
		if page.Truncated {
			w.Write([]byte("\n[log capped at 8 MiB; remaining output not retained]\n"))
		}
	}))
	h.mux.HandleFunc("GET /api/workflow-runs/{id}/delivery", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		v, err := h.workflowDelivery(r.Context(), c, r.PathValue("id"))
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, workflowHTTPValue(c, v))
	}))
	for _, action := range []string{"decision", "stop", "return", "resume"} {
		h.mux.HandleFunc("POST /api/workflow-runs/{id}/"+action, h.protect(false, h.workflowRunCommand))
	}
	for _, method := range []string{"GET", "POST", "DELETE"} {
		h.mux.HandleFunc(method+" /api/workflow-node/mcp", h.workflowNodeMCP)
	}
}
func (h *Server) workflowRunCommand(w http.ResponseWriter, r *http.Request, c Caller) {
	var in struct {
		Seq     int    `json:"seq"`
		Route   string `json:"route"`
		Summary string `json:"summary"`
		Target  string `json:"target"`
		Message string `json:"message"`
	}
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	id := r.PathValue("id")
	var err error
	switch {
	case strings.HasSuffix(r.URL.Path, "/decision"):
		err = h.store.CompleteWorkflowNode(c, id, in.Seq, NodeResult{Route: in.Route, Summary: in.Summary})
	case strings.HasSuffix(r.URL.Path, "/stop"):
		err = h.workflows.Stop(c, id, in.Seq)
	case strings.HasSuffix(r.URL.Path, "/return"):
		err = h.workflows.Return(c, id, in.Seq, in.Target, in.Summary)
	case strings.HasSuffix(r.URL.Path, "/resume"):
		err = h.workflows.Resume(c, id, in.Seq, in.Message)
	default:
		err = ErrNotFound
	}
	if err != nil {
		fail(w, err)
		return
	}
	v, err := h.store.WorkflowRun(c, id)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 202, workflowHTTPValue(c, v))
}
func (h *Server) workflowNodeMCP(w http.ResponseWriter, r *http.Request) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || len(token) != 64 {
		http.Error(w, "node credential required", 401)
		return
	}
	var id string
	var seq int
	err := h.store.DB.QueryRow(`SELECT s.run_id,s.seq FROM workflow_steps s JOIN workflow_runs r ON r.id=s.run_id WHERE s.token_hash=?`, hashText(token)).Scan(&id, &seq)
	if err != nil {
		http.Error(w, "invalid node credential", 401)
		return
	}
	// The bearer token identifies only one execution. The tool transaction checks
	// that it is still current; user cookies or global admin tokens do not work.
	current, loadErr := h.store.WorkflowRun(Caller{Admin: true}, id)
	if loadErr != nil {
		fail(w, loadErr)
		return
	}
	toolNames := workflowNodeTools(current.Definition, current.Definition.node(current.Steps[len(current.Steps)-1].NodeID))
	allows := func(name string) bool {
		for _, tool := range toolNames {
			if name == tool {
				return true
			}
		}
		return false
	}
	service := mcp.NewServer(&mcp.Implementation{Name: "workflow-node", Version: "1.0.0"}, nil)
	if allows("complete_node") {
		mcp.AddTool(service, &mcp.Tool{Name: "complete_node", Description: "Declare this node complete using a factual summary and artifact paths. Omit route to follow its fixed edge. Legacy graphs also accept their named routes; use handoff for autonomous choices. The platform waits for this turn to finish before advancing. After acceptance, end this turn and stop changing files."}, func(ctx context.Context, req *mcp.CallToolRequest, in NodeResult) (*mcp.CallToolResult, any, error) {
			current, e := h.store.WorkflowRun(Caller{Admin: true}, id)
			if e != nil {
				return nil, nil, e
			}
			owner, e := h.store.workflowCaller(current)
			if e != nil {
				return nil, nil, e
			}
			if _, e = h.store.Workflow(owner, current.WorkflowID); e != nil {
				return nil, nil, e
			}
			node := current.Definition.node(current.Steps[len(current.Steps)-1].NodeID)
			if current.Seq == seq && !nodeAgentAllowed(owner, node) {
				return nil, nil, ErrForbidden
			}
			if e = h.store.completeWorkflowNode(Caller{}, id, seq, token, in); e != nil {
				return nil, nil, e
			}
			return nil, map[string]any{"accepted": true, "run_id": id, "seq": seq, "route": in.Route, "message": "Result saved. End this turn; the platform advances after the Agent is quiet."}, nil
		})
	}
	if allows("handoff") {
		mcp.AddTool(service, &mcp.Tool{Name: "handoff", Description: "Choose an allowed Agent handoff target. Include reason, structured inputs and artifact paths. After acceptance end this turn; the platform waits for it to finish."}, func(ctx context.Context, req *mcp.CallToolRequest, in HandoffInput) (*mcp.CallToolResult, any, error) {
			current, e := h.store.WorkflowRun(Caller{Admin: true}, id)
			if e != nil {
				return nil, nil, e
			}
			owner, e := h.store.workflowCaller(current)
			if e != nil {
				return nil, nil, e
			}
			if _, e = h.store.Workflow(owner, current.WorkflowID); e != nil {
				return nil, nil, e
			}
			node := current.Definition.node(current.Steps[len(current.Steps)-1].NodeID)
			if current.Seq != seq {
				return nil, nil, ErrConflict
			}
			if !nodeAgentAllowed(owner, node) {
				return nil, nil, ErrForbidden
			}
			if e = h.store.handoffWorkflowNode(id, seq, token, in); e != nil {
				return nil, nil, e
			}
			return nil, map[string]any{"accepted": true, "run_id": id, "seq": seq, "target": in.Target, "message": "Handoff saved. End this turn; dispatch follows after the Agent is quiet."}, nil
		})
	}
	if allows("wait_for_input") {
		mcp.AddTool(service, &mcp.Tool{Name: "wait_for_input", Description: "Explicitly wait for a user clarification or a real external blocker. Supply kind clarification/blocked and a concrete reason. This does not complete the node. A new user message resumes this same conversation."}, func(ctx context.Context, req *mcp.CallToolRequest, in WorkflowWait) (*mcp.CallToolResult, any, error) {
			run, err := h.store.WorkflowRun(Caller{Admin: true}, id)
			if err != nil {
				return nil, nil, err
			}
			owner, err := h.store.workflowCaller(run)
			if err != nil {
				return nil, nil, err
			}
			if _, err = h.store.Workflow(owner, run.WorkflowID); err != nil {
				return nil, nil, err
			}
			if !nodeAgentAllowed(owner, run.Definition.node(run.Steps[len(run.Steps)-1].NodeID)) {
				return nil, nil, ErrForbidden
			}
			if err = h.store.waitWorkflowNode(id, seq, token, in); err != nil {
				return nil, nil, err
			}
			return nil, map[string]any{"accepted": true, "message": "Wait recorded. Explain the question/blocker and end this turn. New user input resumes the same conversation."}, nil
		})
	}
	mcp.AddTool(service, &mcp.Tool{Name: "read_command_output", Description: "Read a retained redacted command log from an earlier step of this Run. Use next_offset until eof. Truncated means output exceeded the archive cap. Old executions without a retained log return not found; never infer success from missing output."}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Seq    int   `json:"seq"`
		Offset int64 `json:"offset,omitempty"`
	}) (*mcp.CallToolResult, any, error) {
		current, e := h.store.WorkflowRun(Caller{Admin: true}, id)
		if e != nil {
			return nil, nil, e
		}
		owner, e := h.store.workflowCaller(current)
		if e != nil {
			return nil, nil, e
		}
		if _, e = h.store.Workflow(owner, current.WorkflowID); e != nil {
			return nil, nil, e
		}
		if current.Seq != seq || current.Status != "running" || len(current.Steps) == 0 {
			return nil, nil, ErrConflict
		}
		step := current.Steps[len(current.Steps)-1]
		node := current.Definition.node(step.NodeID)
		if step.Status != "running" || step.Result != nil || in.Seq >= seq {
			return nil, nil, ErrConflict
		}
		if !nodeAgentAllowed(owner, node) {
			return nil, nil, ErrForbidden
		}
		page, e := h.store.connectorOutput(current, in.Seq, in.Offset)
		return nil, page, e
	})
	r.Body = http.MaxBytesReader(w, r.Body, 512*1024)
	mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return service }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}).ServeHTTP(w, r)
}

// RunWorkflows is explicitly tied to the HTTP server lifecycle. Tests may tick
// the engine without creating a background scheduler.
func (h *Server) RunWorkflows(ctx context.Context) { h.workflows.Run(ctx) }
