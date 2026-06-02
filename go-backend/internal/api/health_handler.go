package api

import (
	"encoding/json"
	"net/http"
	"time"
)

var startedAt = time.Now().UTC()

// TODO: Implement GET /api/health.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"service":   "zhikuncode-go-backend",
		"version":   "0.1.0",
		"startedAt": startedAt.Format(time.RFC3339),
		"status":    "ok",
	})
}
