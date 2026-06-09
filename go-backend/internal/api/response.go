package api

import (
	"encoding/json"
	"errors"
	"go-backend/internal/llm"
	"go-backend/internal/python"
	"net/http"
)

func writeJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, statusCode int, code string, message string) {
	writeJSON(w, statusCode, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func writePythonError(w http.ResponseWriter, err error, message string) {
	statusCode := http.StatusInternalServerError
	detail := err.Error()

	var pythonErr *python.Error
	if errors.As(err, &pythonErr) {
		statusCode = pythonErr.StatusCode
		if pythonErr.Body != "" {
			detail = pythonErr.Body
		}
	}

	writeError(w, statusCode, "python_service_error", message+": "+detail)
}

func writeLLMError(w http.ResponseWriter, err error, message string) {
	statusCode := http.StatusInternalServerError
	code := "llm_error"
	detail := err.Error()

	var llmErr *llm.Error
	if errors.As(err, &llmErr) {
		code = string(llmErr.Kind)
		detail = llmErr.Message

		switch llmErr.Kind {
		case llm.ErrorKindConfig:
			statusCode = http.StatusBadRequest
		case llm.ErrorKindUnauthorized:
			statusCode = http.StatusUnauthorized
		case llm.ErrorKindRateLimited:
			statusCode = http.StatusTooManyRequests
		case llm.ErrorKindTimeout:
			statusCode = http.StatusGatewayTimeout
		case llm.ErrorKindProvider:
			if llmErr.StatusCode > 0 {
				statusCode = llmErr.StatusCode
			}
		}
	}

	writeError(w, statusCode, code, message+": "+detail)
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
