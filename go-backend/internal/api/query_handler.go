package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	"go-backend/internal/engine"
	"go-backend/internal/llm"
	"go-backend/internal/session"
	"go-backend/internal/storage"
	"go-backend/internal/tools"
)

type queryRequest struct {
	SessionID string            `json:"session_id,omitempty"`
	Model     string            `json:"model,omitempty"`
	Prompt    string            `json:"prompt,omitempty"`
	System    string            `json:"system_prompt,omitempty"`
	Messages  []llm.ChatMessage `json:"messages,omitempty"`
}

type queryResponse struct {
	SessionID string    `json:"session_id,omitempty"`
	Text      string    `json:"text"`
	Model     string    `json:"model,omitempty"`
	Usage     llm.Usage `json:"usage,omitempty"`
}

var (
	querySessionsMu sync.Mutex
	querySessions   session.Service
)

func queryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeNotImplemented(w, "POST /api/query")
		return
	}

	req := queryRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body: "+err.Error())
		return
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = latestUserPrompt(req.Messages)
	}
	if prompt == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "prompt is required")
		return
	}

	cfg, err := llm.LoadConfig()
	if err != nil {
		writeLLMError(w, err, "Failed to load LLM config")
		return
	}

	client := llm.NewOpenAICompatibleClient(cfg)

	sessions, err := getQuerySessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", "Failed to open SQLite: "+err.Error())
		return
	}

	workspacePath, err := loadWorkspacePath()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "workspace_error", "Failed to resolve workspace: "+err.Error())
		return
	}

	toolRegistry, err := tools.NewDefaultRegistry(workspacePath, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "tool_registry_error", "Failed to initialize tools: "+err.Error())
		return
	}

	queryEngine := engine.NewQueryEngine(client, sessions, toolRegistry, engine.Config{})
	result, err := queryEngine.Query(r.Context(), engine.QueryRequest{
		SessionID:    req.SessionID,
		Model:        req.Model,
		Prompt:       prompt,
		SystemPrompt: req.System,
	})

	if err != nil {
		writeQueryError(w, err, "Failed to run query")
		return
	}

	writeJSON(w, http.StatusOK, queryResponse{
		SessionID: result.SessionID,
		Text:      result.Text,
		Model:     result.Model,
		Usage:     result.Usage,
	})
}

func latestUserPrompt(messages []llm.ChatMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != llm.RoleUser {
			continue
		}
		if content := strings.TrimSpace(messages[i].Content); content != "" {
			return content
		}
	}
	return ""
}

func writeQueryError(w http.ResponseWriter, err error, message string) {
	var llmErr *llm.Error
	if errors.As(err, &llmErr) {
		writeLLMError(w, err, "Failed to call LLM")
		return
	}

	if errors.Is(err, session.ErrInvalidSession) {
		writeError(w, http.StatusBadRequest, "invalid_session", message+": "+err.Error())
		return
	}
	if errors.Is(err, session.ErrSessionNotFound) {
		writeError(w, http.StatusNotFound, "session_not_found", message+": "+err.Error())
		return
	}
	if errors.Is(err, engine.ErrMaxToolRoundsExceeded) {
		writeError(w, http.StatusInternalServerError, "max_tool_rounds_exceeded", message+": "+err.Error())
		return
	}

	writeError(w, http.StatusInternalServerError, "query_error", message+": "+err.Error())
}

func getQuerySessions() (session.Service, error) {
	querySessionsMu.Lock()
	defer querySessionsMu.Unlock()

	if querySessions != nil {
		return querySessions, nil
	}

	workspacePath, err := loadWorkspacePath()
	if err != nil {
		return nil, err
	}

	db, err := storage.OpenSQLite(storage.SQLiteConfig{
		Path: storage.DefaultSQLitePath(workspacePath),
	})
	if err != nil {
		return nil, err
	}

	querySessions = session.NewSQLiteService(db)
	return querySessions, nil
}
