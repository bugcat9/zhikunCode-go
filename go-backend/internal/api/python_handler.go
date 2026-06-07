package api

import (
	"go-backend/internal/python"
	"net/http"
)

func pythonCapabilitiesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeNotImplemented(w, "GET /api/python/capabilities")
		return
	}

	capabilities, err := python.GetCapabilities(r.Context())
	if err != nil {
		writePythonError(w, err, "Failed to get Python capabilities")
		return
	}
	writeJSON(w, http.StatusOK, capabilities)
}
