package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

func TestSandboxReadinessWaitsForSameOwnedExecdWithoutRecreating(t *testing.T) {
	for _, mode := range []string{"starts late", "authentication rejected", "cancel waiting", "environment ended"} {
		t.Run(mode, func(t *testing.T) {
			var base string
			var pings, writes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					writes.Add(1)
					w.WriteHeader(500)
					return
				}
				switch {
				case r.URL.Path == "/sandboxes/owned":
					state := opensandbox.StateRunning
					if mode == "environment ended" && pings.Load() > 0 {
						state = opensandbox.StateTerminated
					}
					json.NewEncoder(w).Encode(opensandbox.SandboxInfo{ID: "owned", Metadata: map[string]string{"agent-platform-run": sandboxTestRun}, Status: opensandbox.SandboxStatus{State: state}})
				case strings.Contains(r.URL.Path, "/endpoints/"):
					json.NewEncoder(w).Encode(opensandbox.Endpoint{Endpoint: base})
				case r.URL.Path == "/ping":
					n := pings.Add(1)
					if mode == "authentication rejected" {
						w.WriteHeader(403)
						return
					}
					// The fixed SDK can retry one reused GET connection; both initial
					// attempts fail before a response, unlike an actual HTTP refusal.
					if n <= 2 || mode == "cancel waiting" {
						conn, _, _ := w.(http.Hijacker).Hijack()
						conn.Close()
						return
					}
					w.Write([]byte("pong"))
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			base = server.URL
			service, _ := newOpenSandbox(base, "private-control", "runtime", 600)
			deadline := time.Second
			if mode == "cancel waiting" {
				deadline = 80 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), deadline)
			defer cancel()
			err := service.waitReady(ctx, "owned", sandboxTestRun)
			if writes.Load() != 0 {
				t.Fatal("readiness recreated or modified environment")
			}
			switch mode {
			case "starts late":
				if err != nil || pings.Load() != 3 {
					t.Fatal("running container mistaken for ready service", err, pings.Load())
				}
			case "authentication rejected":
				if err == nil || pings.Load() != 1 {
					t.Fatal("HTTP rejection retried", err, pings.Load())
				}
			case "cancel waiting":
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("waiting did not honor cancellation", err)
				}
			case "environment ended":
				if err == nil || !strings.Contains(err.Error(), "failed or expired") || pings.Load() > 2 {
					t.Fatal("expired environment was reused", err, pings.Load())
				}
			}
		})
	}
}
