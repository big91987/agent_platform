package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestCodexWebSocketPreservesFramesAndRejectsOversizedOrBinaryInput(t *testing.T) {
	for _, mode := range []string{"text", "binary", "oversized"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer native-capability" || r.Header.Get("OPEN-SANDBOX-API-KEY") != "" {
					t.Error("native credentials mixed with control credentials")
				}
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.CloseNow()
				ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
				defer cancel()
				_, input, err := conn.Read(ctx)
				if err != nil {
					return
				}
				var value map[string]any
				if json.Unmarshal(input, &value) != nil || value["method"] != "initialize" {
					t.Error("native request corrupted")
				}
				kind := websocket.MessageText
				output := `{"id":1,"result":{"literal":"$(not a shell)"}}`
				if mode == "binary" {
					kind = websocket.MessageBinary
				}
				if mode == "oversized" {
					output = strings.Repeat("x", 5*1024*1024)
				}
				conn.Write(ctx, kind, []byte(output))
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			stream, err := dialCodexWebSocket(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), http.Header{"Authorization": []string{"Bearer native-capability"}})
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			if err = json.NewEncoder(stream).Encode(map[string]any{"id": 1, "method": "initialize"}); err != nil {
				t.Fatal(err)
			}
			var value map[string]any
			err = json.NewDecoder(stream).Decode(&value)
			if mode != "text" {
				if err == nil {
					t.Fatal("unsafe native frame accepted")
				}
				return
			}
			if err != nil || value["id"] != float64(1) || value["result"].(map[string]any)["literal"] != "$(not a shell)" {
				t.Fatal(value, err)
			}
			if _, err = stream.Write([]byte("partial JSON")); err == nil {
				t.Fatal("invalid frame sent")
			}
			var zero [0]byte
			if n, e := stream.Read(zero[:]); n != 0 || e != nil {
				t.Fatal("empty Read did not respect io.Reader contract", n, e)
			}
		})
	}
}
