package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-backend/internal/llm"
	"go-backend/internal/session"
)

type createSessionRequest struct {
	Dir   string `json:"dir,omitempty"`
	Model string `json:"model,omitempty"`
	Title string `json:"title,omitempty"`
}

type sessionSummaryResponse struct {
	ID               string  `json:"id"`
	SessionID        string  `json:"sessionId"`
	Title            *string `json:"title"`
	Model            string  `json:"model"`
	WorkingDirectory string  `json:"workingDirectory"`
	MessageCount     int     `json:"messageCount"`
	CostUSD          float64 `json:"costUsd"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
}

func sessionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listSessionsHandler(w, r)
	case http.MethodPost:
		createSessionHandler(w, r)
	default:
		writeNotImplemented(w, "GET|POST /api/sessions")
	}
}

func sessionHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.PathValue("sessionId"))
	switch r.Method {
	case http.MethodGet:
		getSessionHandler(w, r, sessionID)
	case http.MethodDelete:
		deleteSessionHandler(w, r, sessionID)
	default:
		writeNotImplemented(w, "GET|DELETE /api/sessions/{sessionId}")
	}
}

func listSessionsHandler(w http.ResponseWriter, r *http.Request) {
	sessions, err := getQuerySessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", "Failed to open SQLite: "+err.Error())
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := sessions.List(r.Context(), session.ListOptions{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeSessionError(w, err, "Failed to list sessions")
		return
	}

	workspacePath := effectiveWorkspacePath()
	model := effectiveDefaultModel()
	items := make([]sessionSummaryResponse, 0, len(result.Sessions))
	for _, item := range result.Sessions {
		items = append(items, sessionSummaryFromSession(item, model, workspacePath))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sessions":   items,
		"hasMore":    result.HasMore,
		"nextCursor": nullableString(result.NextCursor),
	})
}

func createSessionHandler(w http.ResponseWriter, r *http.Request) {
	req := createSessionRequest{}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body: "+err.Error())
			return
		}
	}

	sessions, err := getQuerySessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", "Failed to open SQLite: "+err.Error())
		return
	}

	sess, err := sessions.Create(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", "Failed to create session: "+err.Error())
		return
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = effectiveDefaultModel()
	}
	workspacePath := strings.TrimSpace(req.Dir)
	if workspacePath == "" || workspacePath == "." {
		workspacePath = effectiveWorkspacePath()
	}

	writeJSON(w, http.StatusCreated, sessionSummaryResponse{
		ID:               sess.ID,
		SessionID:        sess.ID,
		Title:            stringPtrOrNil(req.Title),
		Model:            model,
		WorkingDirectory: workspacePath,
		MessageCount:     0,
		CostUSD:          0,
		CreatedAt:        formatAPITime(sess.CreatedAt),
		UpdatedAt:        formatAPITime(sess.UpdatedAt),
	})
}

func getSessionHandler(w http.ResponseWriter, r *http.Request, sessionID string) {
	sessions, err := getQuerySessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", "Failed to open SQLite: "+err.Error())
		return
	}

	sess, err := sessions.Get(r.Context(), sessionID)
	if err != nil {
		writeSessionError(w, err, "Failed to get session")
		return
	}

	messages, err := sessions.ListMessages(r.Context(), sessionID, 0)
	if err != nil {
		writeSessionError(w, err, "Failed to list session messages")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session": sessionSummaryFromSession(session.Summary{
			ID:           sess.ID,
			Title:        sess.Title,
			CreatedAt:    sess.CreatedAt,
			UpdatedAt:    sess.UpdatedAt,
			MessageCount: len(messages),
		}, effectiveDefaultModel(), effectiveWorkspacePath()),
		"messages": frontendMessagesFromSession(messages),
	})
}

func deleteSessionHandler(w http.ResponseWriter, r *http.Request, sessionID string) {
	sessions, err := getQuerySessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", "Failed to open SQLite: "+err.Error())
		return
	}

	if err := sessions.Delete(r.Context(), sessionID); err != nil {
		writeSessionError(w, err, "Failed to delete session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func sessionMessagesHandler(w http.ResponseWriter, r *http.Request) {
	sessions, err := getQuerySessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", "Failed to open SQLite: "+err.Error())
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	messages, err := sessions.ListMessages(r.Context(), r.PathValue("sessionId"), limit)
	if err != nil {
		writeSessionError(w, err, "Failed to list session messages")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"messages": frontendMessagesFromSession(messages),
		"total":    len(messages),
	})
}

func sessionActivitiesHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"activities": []any{},
		"hasMore":    false,
		"total":      0,
	})
}

func sessionHistorySnapshotsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{})
}

func sessionHistoryDiffHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"filesAdded":    0,
		"filesModified": 0,
		"filesDeleted":  0,
		"changedFiles":  []string{},
		"fromMessageId": r.URL.Query().Get("fromMessageId"),
		"toMessageId":   r.URL.Query().Get("toMessageId"),
		"sessionId":     r.PathValue("sessionId"),
		"compatibility": "empty-history",
	})
}

func sessionSummaryFromSession(item session.Summary, model string, workspacePath string) sessionSummaryResponse {
	return sessionSummaryResponse{
		ID:               item.ID,
		SessionID:        item.ID,
		Title:            stringPtrOrNil(item.Title),
		Model:            model,
		WorkingDirectory: workspacePath,
		MessageCount:     item.MessageCount,
		CostUSD:          0,
		CreatedAt:        formatAPITime(item.CreatedAt),
		UpdatedAt:        formatAPITime(item.UpdatedAt),
	}
}

func frontendMessagesFromSession(messages []session.Message) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		timestamp := message.CreatedAt.UnixMilli()
		if message.CreatedAt.IsZero() {
			timestamp = time.Now().UnixMilli()
		}

		switch message.Role {
		case llm.RoleAssistant:
			result = append(result, map[string]any{
				"type":       "assistant",
				"uuid":       message.ID,
				"timestamp":  timestamp,
				"content":    []map[string]any{{"type": "text", "text": message.Content}},
				"stopReason": "end_turn",
				"usage":      zeroFrontendUsage(),
			})
		case llm.RoleSystem:
			result = append(result, map[string]any{
				"type":      "system",
				"uuid":      message.ID,
				"timestamp": timestamp,
				"content":   message.Content,
			})
		default:
			result = append(result, map[string]any{
				"type":      "user",
				"uuid":      message.ID,
				"timestamp": timestamp,
				"content":   []map[string]any{{"type": "text", "text": message.Content}},
			})
		}
	}
	return result
}

func zeroFrontendUsage() map[string]int {
	return map[string]int{
		"inputTokens":              0,
		"outputTokens":             0,
		"cacheReadInputTokens":     0,
		"cacheCreationInputTokens": 0,
	}
}

func writeSessionError(w http.ResponseWriter, err error, message string) {
	switch {
	case errors.Is(err, session.ErrInvalidSession):
		writeError(w, http.StatusBadRequest, "invalid_session", message+": "+err.Error())
	case errors.Is(err, session.ErrInvalidCursor):
		writeError(w, http.StatusBadRequest, "invalid_cursor", message+": "+err.Error())
	case errors.Is(err, session.ErrSessionNotFound):
		writeError(w, http.StatusNotFound, "session_not_found", message+": "+err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "session_error", message+": "+err.Error())
	}
}

func effectiveDefaultModel() string {
	catalog := loadModelCatalog()
	if catalog.DefaultModel != "" {
		return catalog.DefaultModel
	}
	return fallbackDefaultModel
}

func effectiveWorkspacePath() string {
	path, err := loadWorkspacePath()
	if err != nil {
		return "."
	}
	return path
}

func formatAPITime(t time.Time) string {
	if t.IsZero() {
		t = time.Now().UTC()
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func stringPtrOrNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
