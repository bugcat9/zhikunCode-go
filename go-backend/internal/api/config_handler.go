package api

import (
	"encoding/json"
	"net/http"
)

// TODO: Implement GET /api/config.
func configHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"service":          "zhikuncode-go-backend",
		"httpPort":         8081,
		"pythonServiceUrl": "http://localhost:8000",
	})
}
