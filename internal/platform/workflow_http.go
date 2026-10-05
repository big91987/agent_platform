package platform

import (
	"context"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (h *Server) workflowRunRoutes() {
	h.connectorRoutes()
	h.workflowHookRoutes()
	h.mux.HandleFunc("GET /api/workflow-runs", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		v, e := h.store.WorkflowRunsFor(c, r.URL.Query().Get("workflow_id"))
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, v)
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
		respond(w, 201, v)
	}))
	h.mux.HandleFunc("GET /api/workflow-runs/{id}", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		v, e := h.store.WorkflowRun(c, r.PathValue("id"))
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, v)
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
	respond(w, 202, v)
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
	service := mcp.NewServer(&mcp.Implementation{Name: "workflow-node", Version: "1.0.0"}, nil)
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
		if current.Seq == seq && !allowed(owner, node.AgentID) {
			return nil, nil, ErrForbidden
		}
		if e = h.store.completeWorkflowNode(Caller{}, id, seq, token, in); e != nil {
			return nil, nil, e
		}
		return nil, map[string]any{"accepted": true, "run_id": id, "seq": seq, "route": in.Route, "message": "Result saved. End this turn; the platform advances after the Agent is quiet."}, nil
	})
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
		if !allowed(owner, node.AgentID) {
			return nil, nil, ErrForbidden
		}
		if e = h.store.handoffWorkflowNode(id, seq, token, in); e != nil {
			return nil, nil, e
		}
		return nil, map[string]any{"accepted": true, "run_id": id, "seq": seq, "target": in.Target, "message": "Handoff saved. End this turn; dispatch follows after the Agent is quiet."}, nil
	})
	r.Body = http.MaxBytesReader(w, r.Body, 512*1024)
	mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return service }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}).ServeHTTP(w, r)
}

// RunWorkflows is explicitly tied to the HTTP server lifecycle. Tests may tick
// the engine without creating a background scheduler.
func (h *Server) RunWorkflows(ctx context.Context) { h.workflows.Run(ctx) }
