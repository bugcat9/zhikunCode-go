package api

import (
	"encoding/json"
	"net/http"
)

// TODO: Implement GET /api/doctor.
func doctorHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"checks": map[string]any{
			"goBackend": map[string]string{
				"status": "ok",
			},
			"pythonService": map[string]string{
				"status": "not_checked",
			},
			"sqlite": map[string]string{
				"status": "not_checked",
			},
			"workspace": map[string]string{
				"status": "not_checked",
			},
		},
	})
}
