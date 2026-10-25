package platform

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"time"
)

func (h *Server) workflowArchiveRoutes() {
	h.mux.HandleFunc("POST /api/workflow-runs/{id}/artifacts/import", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		var in artifactImport
		if err := decode(w, r, &in); err != nil {
			fail(w, err)
			return
		}
		result, err := h.workflows.importArtifacts(r.Context(), c, r.PathValue("id"), in)
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, result)
	}))

	h.mux.HandleFunc("GET /api/workflow-runs/{id}/artifacts", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		run, err := h.store.WorkflowRun(c, r.PathValue("id"))
		if err != nil {
			fail(w, err)
			return
		}
		value, err := h.store.workflowArchives(run)
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, value)
	}))
	h.mux.HandleFunc("GET /api/workflow-runs/{id}/steps/{seq}/artifacts/file", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
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
		p := r.URL.Query().Get("path")
		b, err := h.store.archivedContent(run, seq, p)
		if err != nil {
			fail(w, err)
			return
		}
		serveArtifactContent(w, r, p, time.Time{}, bytes.NewReader(b))
	}))
	h.mux.HandleFunc("POST /api/workflow-runs/{id}/steps/{seq}/artifacts/retain", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		seq, err := strconv.Atoi(r.PathValue("seq"))
		if err != nil {
			fail(w, ErrNotFound)
			return
		}
		// Capture is owned by the engine; an HTTP disconnect must not cancel it.
		if err = h.workflows.RetainArtifacts(context.Background(), c, r.PathValue("id"), seq); err != nil {
			fail(w, err)
			return
		}
		respond(w, 202, map[string]string{"status": "accepted"})
	}))
}
