package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"go-backend/internal/engine"
	"go-backend/internal/llm"
)

func streamQueryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeNotImplemented(w, "POST /api/query/stream")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported", "Response writer does not support streaming")
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

	broker, err := getPermissionBroker()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "permission_error", "Failed to initialize permissions: "+err.Error())
		return
	}

	queryEngine, err := newRuntimeQueryEngine(llm.NewOpenAICompatibleClient(cfg), sessions, workspacePath, broker)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query_engine_error", "Failed to initialize QueryEngine: "+err.Error())
		return
	}

	events, err := queryEngine.Stream(r.Context(), engine.QueryRequest{
		SessionID:    req.SessionID,
		Model:        req.Model,
		Prompt:       prompt,
		SystemPrompt: req.System,
	})
	if err != nil {
		writeQueryError(w, err, "Failed to start stream query")
		return
	}

	writeSSEHeaders(w)
	for event := range events {
		if err := writeSSEEvent(w, event); err != nil {
			return
		}
		flusher.Flush()
	}
}

func writeSSEHeaders(w http.ResponseWriter) {
	header := w.Header()
	header.Set("Content-Type", "text/event-stream; charset=utf-8")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
}

func writeSSEEvent(w http.ResponseWriter, event engine.Event) error {
	eventType := eventTypeForSSE(event)
	data, err := json.Marshal(event)
	if err != nil {
		eventType = engine.EventError
		data, _ = json.Marshal(engine.Event{
			Type: eventType,
			Payload: map[string]any{
				"message": "failed to encode stream event: " + err.Error(),
			},
		})
	}

	if _, err := w.Write([]byte("event: " + string(eventType) + "\n")); err != nil {
		return err
	}
	if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
		return err
	}
	return nil
}

func eventTypeForSSE(event engine.Event) engine.EventType {
	if event.Type != "" {
		return event.Type
	}
	return engine.EventError
}
