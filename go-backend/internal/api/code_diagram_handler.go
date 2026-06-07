package api

import (
	"encoding/json"
	"go-backend/internal/python"
	"net/http"
)

func codeDiagramGenerateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeNotImplemented(w, "POST /api/code-diagrams/generate")
		return
	}
	req := python.DiagramRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body: "+err.Error())
		return
	}
	resp, err := python.GenerateDiagram(r.Context(), req)
	if err != nil {
		writePythonError(w, err, "Failed to generate diagram")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
