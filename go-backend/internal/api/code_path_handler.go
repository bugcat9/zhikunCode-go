package api

import (
	"encoding/json"
	"go-backend/internal/python"
	"net/http"
)

func codePathEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeNotImplemented(w, "POST /api/code-path/endpoints")
		return
	}
	req := python.APIEndpointsRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body: "+err.Error())
		return
	}
	resp, err := python.ExtractAPIEndpoints(r.Context(), req)
	if err != nil {
		writePythonError(w, err, "Failed to extract API endpoints")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func codePathTraceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeNotImplemented(w, "POST /api/code-path/trace")
		return
	}
	req := python.CodePathRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body: "+err.Error())
		return
	}
	resp, err := python.AnalyzeCodePaths(r.Context(), req)
	if err != nil {
		writePythonError(w, err, "Failed to generate trace")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
