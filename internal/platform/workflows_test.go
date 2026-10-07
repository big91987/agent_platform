package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func workflowFixture() map[string]any {
	return map[string]any{"name": "文稿审核", "enabled": true, "entry": "review", "nodes": []any{
		map[string]any{"id": "review", "name": "检查", "kind": "approval", "x": 40, "y": 80},
		map[string]any{"id": "revise", "name": "修订", "kind": "approval", "x": 320, "y": 80},
		map[string]any{"id": "done", "name": "结束", "kind": "end", "x": 600, "y": 80},
	}, "edges": []any{
		map[string]any{"source": "review", "route": "approved", "target": "done"},
		map[string]any{"source": "review", "route": "changes", "target": "revise"},
		map[string]any{"source": "revise", "route": "ready", "target": "review"},
	}}
}

func workflowAdmin(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	w := requestJSON(t, h, "POST", "/api/login", "", map[string]string{"password": "password"})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	return w.Result().Cookies()[0]
}
func workflowRequest(t *testing.T, h http.Handler, c *http.Cookie, method, path string, v any) *httptest.ResponseRecorder {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.AddCookie(c)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Platform-Request", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func workflowJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var v map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
		t.Fatal(e, w.Body.String())
	}
	return v
}

func TestWorkflowSaveAllowsFeedbackLoopAndRejectsStaleEditor(t *testing.T) {
	s := testStore(t)
	h := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, h)
	w := workflowRequest(t, h, cookie, "POST", "/api/workflows", workflowFixture())
	if w.Code != 201 {
		t.Fatalf("save graph with exiting loop: %d %s", w.Code, w.Body.String())
	}
	v := workflowJSON(t, w)
	id := v["id"].(string)
	if v["revision"] != float64(1) {
		t.Fatal(v)
	}
	v["name"] = "新标题"
	w = workflowRequest(t, h, cookie, "PUT", "/api/workflows/"+id, v)
	if w.Code != 200 || workflowJSON(t, w)["revision"] != float64(2) {
		t.Fatal(w.Code, w.Body.String())
	}
	v["name"] = "过期页面的标题"
	w = workflowRequest(t, h, cookie, "PUT", "/api/workflows/"+id, v)
	if w.Code != 409 {
		t.Fatalf("lost update accepted: %d %s", w.Code, w.Body.String())
	}
	w = workflowRequest(t, h, cookie, "GET", "/api/workflows/"+id, nil)
	saved := workflowJSON(t, w)
	if saved["name"] != "新标题" || len(saved["edges"].([]any)) != 3 {
		t.Fatal(saved)
	}
}

func TestWorkflowRejectsAmbiguousRoutesAndTrappedLoops(t *testing.T) {
	s := testStore(t)
	h := NewServer(s, nil, nil, "password", "http://localhost")
	c := workflowAdmin(t, h)
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"ambiguous", func(v map[string]any) { v["edges"].([]any)[1].(map[string]any)["route"] = "approved" }},
		{"trapped loop", func(v map[string]any) { v["edges"] = v["edges"].([]any)[1:]; v["nodes"] = v["nodes"].([]any)[:2] }},
		{"missing target", func(v map[string]any) { v["edges"].([]any)[0].(map[string]any)["target"] = "missing" }},
		{"duplicate node", func(v map[string]any) { v["nodes"].([]any)[2].(map[string]any)["id"] = "review" }},
		{"unknown entry", func(v map[string]any) { v["entry"] = "missing" }},
		{"unsupported node", func(v map[string]any) { v["nodes"].([]any)[0].(map[string]any)["kind"] = "eval" }},
		{"unbounded steps", func(v map[string]any) { v["max_steps"] = 100000 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := workflowFixture()
			tc.change(v)
			w := workflowRequest(t, h, c, "POST", "/api/workflows", v)
			if w.Code != 400 {
				t.Fatalf("invalid graph accepted: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestWorkflowAccessRequiresExplicitAuthorization(t *testing.T) {
	s := testStore(t)
	a := testAgent(t, s)
	alice, at := testUser(t, s, &a, "alice")
	_, bt := testUser(t, s, &a, "bob")
	h := NewServer(s, nil, nil, "password", "http://localhost")
	c := workflowAdmin(t, h)
	v := workflowFixture()
	v["authorized_users"] = []string{alice.UserID}
	w := workflowRequest(t, h, c, "POST", "/api/workflows", v)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	id := workflowJSON(t, w)["id"].(string)
	for _, tc := range []struct {
		token string
		want  int
	}{{"", 401}, {at, 200}, {bt, 403}} {
		w = requestJSON(t, h, "GET", "/api/workflows/"+id, tc.token, nil)
		if w.Code != tc.want {
			t.Fatalf("read: got %d want %d", w.Code, tc.want)
		}
	}
	w = requestJSON(t, h, "GET", "/api/workflows", bt, nil)
	if w.Code != 200 || w.Body.String() != "[]\n" {
		t.Fatal("private workflow leaked", w.Body.String())
	}
	w = requestJSON(t, h, "POST", "/api/workflows", at, workflowFixture())
	if w.Code != 403 {
		t.Fatal("caller created workflow", w.Code)
	}
}
