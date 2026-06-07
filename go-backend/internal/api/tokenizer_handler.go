package api

import (
	"encoding/json"
	"go-backend/internal/python"
	"net/http"
)

func tokenizerCountHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeNotImplemented(w, "POST /api/tokenizer/count")
		return
	}

	req := python.TokenCountRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body: "+err.Error())
		return
	}
	tokens, err := python.CountTokens(r.Context(), req)
	if err != nil {
		writePythonError(w, err, "Failed to count tokens")
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}
