package platform

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"agentplatform/web"
)

type Server struct {
	store          *Store
	scheduler      *Scheduler
	codex          *Codex
	password, base string
	mu             sync.Mutex
	sessions       map[string]time.Time
	loginFailures  int
	loginWindow    time.Time
	mux            *http.ServeMux
}

func NewServer(s *Store, sched *Scheduler, x *Codex, password, base string) *Server {
	h := &Server{store: s, scheduler: sched, codex: x, password: password, base: strings.TrimRight(base, "/"), sessions: map[string]time.Time{}, mux: http.NewServeMux()}
	h.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if e := s.DB.PingContext(r.Context()); e != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		respond(w, 200, map[string]any{"ok": true, "version": "0.1.0"})
	})
	h.mux.HandleFunc("POST /api/login", h.login)
	h.mux.HandleFunc("GET /api/me", h.protect(false, func(w http.ResponseWriter, r *http.Request, c Caller) {
		respond(w, 200, map[string]any{"admin": c.Admin, "source": c.Source})
	}))
	h.mux.HandleFunc("POST /api/logout", h.protect(true, func(w http.ResponseWriter, r *http.Request, c Caller) {
		cookie, _ := r.Cookie("platform_session")
		if cookie != nil {
			h.mu.Lock()
			delete(h.sessions, cookie.Value)
			h.mu.Unlock()
		}
		http.SetCookie(w, &http.Cookie{Name: "platform_session", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		respond(w, 200, map[string]bool{"ok": true})
	}))
	h.mux.HandleFunc("GET /api/agents", h.protect(true, h.agents))
	h.mux.HandleFunc("POST /api/agents", h.protect(true, h.saveAgent))
	h.mux.HandleFunc("PUT /api/agents/{id}", h.protect(true, h.saveAgent))
	h.mux.HandleFunc("POST /api/agents/{id}/check", h.protect(true, h.checkAgent))
	h.mux.HandleFunc("GET /api/credentials", h.protect(true, h.credentials))
	h.mux.HandleFunc("POST /api/credentials", h.protect(true, h.credentials))
	h.mux.HandleFunc("DELETE /api/credentials/{id}", h.protect(true, h.revoke))
	h.mux.HandleFunc("POST /api/invoke", h.protect(false, h.invoke))
	h.mux.HandleFunc("POST /api/webhooks/{agent}", h.protect(false, h.invoke))
	h.mux.HandleFunc("POST /api/conversations/{id}/messages", h.protect(false, h.invoke))
	h.mux.HandleFunc("GET /api/conversations", h.protect(false, h.conversations))
	h.mux.HandleFunc("GET /api/conversations/{id}", h.protect(false, h.conversation))
	h.mux.HandleFunc("POST /api/conversations/{id}/stop", h.protect(false, h.action))
	h.mux.HandleFunc("POST /api/conversations/{id}/continue", h.protect(false, h.action))
	h.mux.HandleFunc("POST /api/conversations/{id}/close", h.protect(false, h.action))
	h.mux.HandleFunc("DELETE /api/conversations/{id}", h.protect(true, h.remove))
	h.mux.HandleFunc("GET /api/conversations/{id}/events", h.protect(false, h.events))
	h.mux.HandleFunc("GET /api/conversations/{id}/artifacts", h.protect(false, h.artifacts))
	h.mux.HandleFunc("GET /api/conversations/{id}/file", h.protect(false, h.file))
	h.mux.HandleFunc("GET /", h.static)
	return h
}
func (h *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; frame-ancestors 'none'")
	h.mux.ServeHTTP(w, r)
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, e error) {
	status := 400
	switch {
	case errors.Is(e, ErrForbidden):
		status = 403
	case errors.Is(e, ErrNotFound) || errors.Is(e, os.ErrNotExist):
		status = 404
	case errors.Is(e, ErrConflict):
		status = 409
	}
	respond(w, status, map[string]string{"error": e.Error()})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return errors.New("Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if e := dec.Decode(v); e != nil {
		return fmt.Errorf("invalid JSON input: %w", e)
	}
	var trailing any
	if e := dec.Decode(&trailing); e != io.EOF {
		return errors.New("exactly one JSON object is required")
	}
	return nil
}
func (h *Server) sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == h.base || r.Header.Get("X-Platform-Request") == "1" && origin == ""
}
func (h *Server) caller(r *http.Request) (Caller, error) {
	if auth := r.Header.Get("Authorization"); auth != "" {
		if !strings.HasPrefix(auth, "Bearer ") {
			return Caller{}, ErrForbidden
		}
		return h.store.Authenticate(strings.TrimPrefix(auth, "Bearer "))
	}
	cookie, e := r.Cookie("platform_session")
	if e != nil {
		return Caller{}, ErrForbidden
	}
	h.mu.Lock()
	expiry, ok := h.sessions[cookie.Value]
	if ok && time.Now().After(expiry) {
		delete(h.sessions, cookie.Value)
		ok = false
	}
	h.mu.Unlock()
	if !ok {
		return Caller{}, ErrForbidden
	}
	return Caller{Source: "console", Admin: true}, nil
}
func (h *Server) protect(admin bool, next func(http.ResponseWriter, *http.Request, Caller)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, e := h.caller(r)
		if e != nil {
			respond(w, 401, map[string]string{"error": "login or valid API credential required"})
			return
		}
		if admin && !c.Admin {
			fail(w, ErrForbidden)
			return
		}
		if c.Admin && r.Method != "GET" && r.Method != "HEAD" && !h.sameOrigin(r) {
			fail(w, ErrForbidden)
			return
		}
		next(w, r, c)
	}
}
func (h *Server) login(w http.ResponseWriter, r *http.Request) {
	if o := r.Header.Get("Origin"); o != "" && o != h.base {
		fail(w, ErrForbidden)
		return
	}
	h.mu.Lock()
	if time.Since(h.loginWindow) > time.Minute {
		h.loginWindow = time.Now()
		h.loginFailures = 0
	}
	limited := h.loginFailures >= 8
	h.mu.Unlock()
	if limited {
		respond(w, 429, map[string]string{"error": "too many login attempts; retry in a minute"})
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	if subtle.ConstantTimeCompare([]byte(hashText(in.Password)), []byte(hashText(h.password))) != 1 {
		h.mu.Lock()
		h.loginFailures++
		h.mu.Unlock()
		respond(w, 401, map[string]string{"error": "incorrect password"})
		return
	}
	token := newID() + newID()
	h.mu.Lock()
	for k, v := range h.sessions {
		if time.Now().After(v) {
			delete(h.sessions, k)
		}
	}
	h.sessions[token] = time.Now().Add(24 * time.Hour)
	h.loginFailures = 0
	h.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "platform_session", Value: token, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(h.base, "https://"), SameSite: http.SameSiteStrictMode, MaxAge: 86400})
	respond(w, 200, map[string]bool{"ok": true})
}
func (h *Server) agents(w http.ResponseWriter, r *http.Request, c Caller) {
	v, e := h.store.Agents()
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (h *Server) saveAgent(w http.ResponseWriter, r *http.Request, c Caller) {
	var a Agent
	if e := decode(w, r, &a); e != nil {
		fail(w, e)
		return
	}
	if id := r.PathValue("id"); id != "" {
		a.ID = id
		if _, e := h.store.Agent(id); e != nil {
			fail(w, e)
			return
		}
	} else {
		a.ID = ""
	}
	if _, e := nativeConfig(a); e != nil {
		fail(w, e)
		return
	}
	if _, e := executorEnv(a, "managed"); e != nil {
		fail(w, e)
		return
	}
	for _, p := range a.Skills {
		if _, e := normalizeSkill(p); e != nil {
			fail(w, e)
			return
		}
	}
	v, e := h.store.SaveAgent(a)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (h *Server) checkAgent(w http.ResponseWriter, r *http.Request, c Caller) {
	a, e := h.store.Agent(r.PathValue("id"))
	if e != nil {
		fail(w, e)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 70*time.Second)
	defer cancel()
	v, e := h.codex.Check(ctx, a)
	if e != nil {
		respond(w, 200, CheckResult{Error: e.Error()})
		return
	}
	respond(w, 200, v)
}
func (h *Server) credentials(w http.ResponseWriter, r *http.Request, c Caller) {
	if r.Method == "GET" {
		v, e := h.store.Credentials()
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, v)
		return
	}
	var in struct {
		Name   string   `json:"name"`
		Agents []string `json:"agents"`
	}
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	v, token, e := h.store.CreateCredential(in.Name, in.Agents)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 201, map[string]any{"credential": v, "token": token})
}
func (h *Server) revoke(w http.ResponseWriter, r *http.Request, c Caller) {
	if e := h.store.RevokeCredential(r.PathValue("id")); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
func (h *Server) invoke(w http.ResponseWriter, r *http.Request, c Caller) {
	var in Input
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	if c.Admin && in.UserID == "" {
		in.UserID = "operator"
	}
	if id := r.PathValue("id"); id != "" {
		if in.ConversationID != "" && in.ConversationID != id {
			fail(w, ErrConflict)
			return
		}
		in.ConversationID = id
	}
	if agent := r.PathValue("agent"); agent != "" {
		if in.AgentID != "" && in.AgentID != agent {
			fail(w, ErrConflict)
			return
		}
		in.AgentID = agent
	}
	v, e := h.store.Submit(c, in)
	if e != nil {
		fail(w, e)
		return
	}
	v.ConversationURL = h.base + "/conversations/" + v.ConversationID
	respond(w, 202, v)
}
func (h *Server) conversations(w http.ResponseWriter, r *http.Request, c Caller) {
	if !c.Admin && r.URL.Query().Get("user_id") == "" {
		fail(w, errors.New("user_id is required"))
		return
	}
	v, e := h.store.Conversations(c, r.URL.Query().Get("user_id"))
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (h *Server) authorize(w http.ResponseWriter, r *http.Request, c Caller) (Conversation, bool) {
	v, e := h.store.Authorize(c, r.PathValue("id"), r.URL.Query().Get("user_id"))
	if e != nil {
		fail(w, e)
		return v, false
	}
	return v, true
}
func (h *Server) conversation(w http.ResponseWriter, r *http.Request, c Caller) {
	v, ok := h.authorize(w, r, c)
	if !ok {
		return
	}
	messages, e := h.store.Messages(v.ID)
	if e != nil {
		fail(w, e)
		return
	}
	artifacts, e := listArtifacts(h.store.Dir, v.ID)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]any{"conversation": v, "messages": messages, "artifacts": artifacts})
}
func (h *Server) action(w http.ResponseWriter, r *http.Request, c Caller) {
	v, ok := h.authorize(w, r, c)
	if !ok {
		return
	}
	var e error
	switch {
	case strings.HasSuffix(r.URL.Path, "/continue"):
		e = h.scheduler.Continue(v.ID)
	case strings.HasSuffix(r.URL.Path, "/close"):
		e = h.scheduler.Stop(v.ID, true)
	default:
		e = h.scheduler.Stop(v.ID, false)
	}
	if e != nil {
		fail(w, e)
		return
	}
	updated, _ := h.store.Conversation(v.ID)
	respond(w, 202, updated)
}
func (h *Server) remove(w http.ResponseWriter, r *http.Request, c Caller) {
	v, ok := h.authorize(w, r, c)
	if !ok {
		return
	}
	if v.Status != "closed" {
		fail(w, errors.New("close this conversation before deleting its history and files"))
		return
	}
	h.scheduler.mu.Lock()
	defer h.scheduler.mu.Unlock()
	if _, active := h.scheduler.active[v.ID]; active {
		fail(w, ErrConflict)
		return
	}
	if e := os.RemoveAll(filepath.Join(h.store.Dir, "conversations", v.ID)); e != nil {
		fail(w, e)
		return
	}
	tx, e := h.store.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	_, e = tx.Exec(`DELETE FROM requests WHERE json_extract(receipt,'$.conversation_id')=?; DELETE FROM conversations WHERE id=?`, v.ID, v.ID)
	if e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"deleted": true})
}
func (h *Server) events(w http.ResponseWriter, r *http.Request, c Caller) {
	v, ok := h.authorize(w, r, c)
	if !ok {
		return
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if last, e := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64); e == nil && last > after {
		after = last
	}
	if r.URL.Query().Get("format") == "json" {
		events, e := h.store.Events(v.ID, after)
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, events)
		return
	}
	flush, ok := w.(http.Flusher)
	if !ok {
		fail(w, errors.New("streaming unavailable"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	fmt.Fprint(w, ": connected\n\n")
	flush.Flush()
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	for {
		caller, e := h.caller(r)
		if e != nil {
			return
		}
		if _, e = h.store.Authorize(caller, v.ID, r.URL.Query().Get("user_id")); e != nil {
			return
		}
		events, e := h.store.Events(v.ID, after)
		if e != nil {
			return
		}
		for _, ev := range events {
			b, _ := json.Marshal(ev)
			if _, e = fmt.Fprintf(w, "id: %d\ndata: %s\n\n", ev.ID, b); e != nil {
				return
			}
			after = ev.ID
		}
		if len(events) == 0 {
			fmt.Fprint(w, ": heartbeat\n\n")
		}
		flush.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
func (h *Server) artifacts(w http.ResponseWriter, r *http.Request, c Caller) {
	v, ok := h.authorize(w, r, c)
	if !ok {
		return
	}
	files, e := listArtifacts(h.store.Dir, v.ID)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, files)
}
func (h *Server) file(w http.ResponseWriter, r *http.Request, c Caller) {
	v, ok := h.authorize(w, r, c)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	f, e := openArtifact(h.store.Dir, v.ID, path)
	if e != nil {
		fail(w, e)
		return
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		fail(w, e)
		return
	}
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	if imageArtifact(path) && r.URL.Query().Get("download") != "1" {
		w.Header().Set("Content-Type", mime.TypeByExtension(strings.ToLower(filepath.Ext(path))))
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(path)}))
	}
	http.ServeContent(w, r, filepath.Base(path), st.ModTime(), f)
}
func (h *Server) static(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" || strings.HasPrefix(name, "conversations/") || name == "agents" || name == "integrations" {
		name = "index.html"
	}
	if name != "index.html" && name != "app.js" && name != "request.js" && name != "style.css" {
		http.NotFound(w, r)
		return
	}
	b, e := web.Files.ReadFile(name)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(name)))
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(b)
}
