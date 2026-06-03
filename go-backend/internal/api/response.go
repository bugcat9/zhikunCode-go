package api

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}

func writeNotImplemented(w http.ResponseWriter, endpoint string) {
	writeJSON(w, http.StatusNotImplemented, map[string]any{
		"error": map[string]any{
			"code":     "not_implemented",
			"message":  endpoint + " is not implemented yet",
			"endpoint": endpoint,
		},
	})
}
