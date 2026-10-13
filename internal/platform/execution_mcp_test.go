package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExecutionMCPStructuredResultAndBusinessFailure(t *testing.T) {
	var calls atomic.Int32
	service := mcp.NewServer(&mcp.Implementation{Name: "execution-test", Version: "1"}, nil)
	type args struct {
		Title string `json:"title"`
		Fail  bool   `json:"fail"`
	}
	mcp.AddTool(service, &mcp.Tool{Name: "create"}, func(ctx context.Context, r *mcp.CallToolRequest, a args) (*mcp.CallToolResult, map[string]any, error) {
		calls.Add(1)
		if a.Fail {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "quota exhausted"}}}, nil, nil
		}
		return nil, map[string]any{"number": 42, "title": a.Title}, nil
	})
	remote := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return service }, nil))
	defer remote.Close()
	c := MCPConnection{URL: remote.URL}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := callMCPTool(ctx, c, "create", json.RawMessage(`{"title":"quote \" and newline\n","fail":false}`))
	if err != nil || result == nil || result.IsError {
		t.Fatalf("call: %+v %v", result, err)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(raw), `"number":42`) || !strings.Contains(string(raw), `quote \" and newline\n`) {
		t.Fatalf("structured data lost: %s %v", raw, err)
	}
	result, err = callMCPTool(ctx, c, "create", json.RawMessage(`{"title":"second","fail":true}`))
	if err != nil || result == nil || !result.IsError || result.Content[0].(*mcp.TextContent).Text != "quota exhausted" {
		t.Fatalf("business failure must remain a tool result: %+v %v", result, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("unexpected replay: %d calls", calls.Load())
	}
}

func TestExecutionMCPRejectsInvalidArgumentsBeforeConnecting(t *testing.T) {
	var requests atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(500)
	}))
	defer remote.Close()
	for _, raw := range []string{`null`, `[]`, `"text"`, `{broken`} {
		if _, err := callMCPTool(context.Background(), MCPConnection{URL: remote.URL}, "create", json.RawMessage(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if requests.Load() != 0 {
		t.Fatal("invalid input contacted the server")
	}
}

func TestExecutionMCPCancellationReachesTool(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	service := mcp.NewServer(&mcp.Implementation{Name: "execution-cancel", Version: "1"}, nil)
	mcp.AddTool(service, &mcp.Tool{Name: "wait"}, func(ctx context.Context, r *mcp.CallToolRequest, a struct{}) (*mcp.CallToolResult, any, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return nil, nil, ctx.Err()
	})
	remote := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return service }, nil))
	defer remote.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := callMCPTool(ctx, MCPConnection{URL: remote.URL}, "wait", json.RawMessage(`{}`))
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("tool never started")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancellation reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("caller did not stop")
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("server tool did not observe cancellation")
	}
}

func TestExecutionMCPUnresponsiveSessionDeletionIsBounded(t *testing.T) {
	service := mcp.NewServer(&mcp.Implementation{Name: "cleanup-test", Version: "1"}, nil)
	mcp.AddTool(service, &mcp.Tool{Name: "read"}, func(ctx context.Context, r *mcp.CallToolRequest, a struct{}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "done"}}}, nil, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return service }, nil)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			select {
			case <-r.Context().Done():
			case <-time.After(6 * time.Second):
			}
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer remote.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	result, err := callMCPTool(ctx, MCPConnection{URL: remote.URL}, "read", json.RawMessage(`{}`))
	if err != nil || result == nil {
		t.Fatalf("completed tool result was lost: %v", err)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("completed action hung behind remote session cleanup")
	}
}

func TestExecutionMCPDroppedWriteResponseDoesNotReplay(t *testing.T) {
	var writes atomic.Int32
	service := mcp.NewServer(&mcp.Implementation{Name: "lost-response", Version: "1"}, nil)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return service }, nil)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			r.Body = io.NopCloser(strings.NewReader(string(raw)))
			var packet struct{ Method string }
			if json.Unmarshal(raw, &packet) == nil && packet.Method == "tools/call" {
				writes.Add(1) // external side effect committed before the response is lost
				connection, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				connection.Close()
				return
			}
		}
		handler.ServeHTTP(w, r)
	}))
	defer remote.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := callMCPTool(ctx, MCPConnection{URL: remote.URL}, "create", json.RawMessage(`{}`))
	if err == nil || result != nil || writes.Load() != 1 {
		t.Fatalf("uncertain write replayed or reported success: result=%v error=%v writes=%d", result, err, writes.Load())
	}
}
