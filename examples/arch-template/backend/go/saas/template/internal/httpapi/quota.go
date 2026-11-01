package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	accounting "example.com/service/internal/accounting/application"
)

func NewQuota(service *accounting.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/quotas/{tenant}", func(w http.ResponseWriter, r *http.Request) {
		q, err := service.Usage(r.Context(), r.PathValue("tenant"))
		if err != nil {
			status := 500
			if errors.Is(err, accounting.ErrNotFound) {
				status = 404
			}
			http.Error(w, http.StatusText(status), status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(q)
	})
	return mux
}
