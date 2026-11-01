package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"example.com/service/internal/gateway"
)

type flushWriter struct{ http.ResponseWriter }

func (w flushWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
	return n, err
}

func NewRelay(service *gateway.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			http.Error(w, "request too large", 413)
			return
		}
		var input struct {
			Model string `json:"model"`
		}
		if json.Unmarshal(data, &input) != nil || input.Model == "" {
			http.Error(w, "model is required", 400)
			return
		}
		committed := false
		err = service.Forward(r.Context(), input.Model, data, func(status int, contentType string) io.Writer {
			committed = true
			w.Header().Set("Content-Type", contentType)
			w.WriteHeader(status)
			return flushWriter{w}
		})
		if err != nil && !committed {
			http.Error(w, "upstream unavailable", 502)
		}
		if err != nil && committed {
			// Abort the HTTP stream so a truncated upstream cannot look like a normal EOF.
			panic(http.ErrAbortHandler)
		}
	})
	return mux
}
