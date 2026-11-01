package httpapi_test

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"example.com/service/internal/gateway"
	"example.com/service/internal/httpapi"
	"example.com/service/internal/infrastructure/upstream"
	"example.com/service/internal/routing"
)

func TestStreamsBeforeCompletionAndPropagatesCancellation(t *testing.T) {
	canceled := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("data: first\n\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(canceled)
	}))
	defer provider.Close()
	pool := routing.New([]routing.Target{{ID: "local", Model: "chat", URL: provider.URL, Version: 1}})
	service := gateway.New(pool, upstream.New(provider.Client()))
	server := httptest.NewServer(httpapi.NewRelay(service))
	defer server.Close()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Post(server.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model":"chat","stream":true}`))
	if err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(response.Body).ReadString('\n')
	if err != nil || line != "data: first\n" {
		t.Fatalf("first frame %q %v", line, err)
	}
	_ = response.Body.Close()
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("client cancellation did not reach upstream")
	}
}

func TestMalformedRequestDoesNotDispatch(t *testing.T) {
	pool := routing.New(nil)
	service := gateway.New(pool, upstream.New(http.DefaultClient))
	recorder := httptest.NewRecorder()
	httpapi.NewRelay(service).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":`)))
	if recorder.Code != 400 {
		t.Fatalf("status %d", recorder.Code)
	}
}

func TestTruncatedUpstreamDoesNotBecomeSuccessfulEndOfStream(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("partial"))
	}))
	defer provider.Close()
	pool := routing.New([]routing.Target{{ID: "local", Model: "chat", URL: provider.URL, Version: 1}})
	server := httptest.NewServer(httpapi.NewRelay(gateway.New(pool, upstream.New(provider.Client()))))
	defer server.Close()
	response, err := server.Client().Post(server.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model":"chat"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, err = io.ReadAll(response.Body)
	if err == nil {
		t.Fatal("truncated upstream was presented as a completed response")
	}
}

func TestRedirectDoesNotReplayModelRequest(t *testing.T) {
	for _, status := range []int{307, 308} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var count atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { count.Add(1); _, _ = w.Write([]byte("replayed")) }))
			defer target.Close()
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count.Add(1)
				w.Header().Set("Location", target.URL)
				w.WriteHeader(status)
			}))
			defer provider.Close()
			pool := routing.New([]routing.Target{{ID: "local", Model: "chat", URL: provider.URL, Version: 1}})
			server := httptest.NewServer(httpapi.NewRelay(gateway.New(pool, upstream.New(provider.Client()))))
			defer server.Close()
			response, err := server.Client().Post(server.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model":"chat"}`))
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if count.Load() != 1 || response.StatusCode != status {
				t.Fatalf("requests=%d status=%d", count.Load(), response.StatusCode)
			}
		})
	}
}
