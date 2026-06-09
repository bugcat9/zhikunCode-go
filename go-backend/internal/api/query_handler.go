package api

import (
	"encoding/json"
	"net/http"

	"go-backend/internal/llm"
)

type queryRequest struct {
	SessionID string            `json:"session_id,omitempty"`
	Model     string            `json:"model,omitempty"`
	Prompt    string            `json:"prompt,omitempty"`
	Messages  []llm.ChatMessage `json:"messages,omitempty"`
}

type queryResponse struct {
	SessionID string    `json:"session_id,omitempty"`
	Text      string    `json:"text"`
	Model     string    `json:"model,omitempty"`
	Usage     llm.Usage `json:"usage,omitempty"`
}

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

	messages := req.Messages
	if len(messages) == 0 && req.Prompt != "" {
		messages = []llm.ChatMessage{
			{Role: llm.RoleUser, Content: req.Prompt},
		}
	}
	if len(messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "prompt or messages is required")
		return
	}

	cfg, err := llm.LoadConfig()
	if err != nil {
		writeLLMError(w, err, "Failed to load LLM config")
		return
	}

	client := llm.NewOpenAICompatibleClient(cfg)
	resp, err := client.Chat(r.Context(), llm.ChatRequest{
		Model:    req.Model,
		Messages: messages,
	})
	if err != nil {
		writeLLMError(w, err, "Failed to call LLM")
		return
	}

	writeJSON(w, http.StatusOK, queryResponse{
		SessionID: req.SessionID,
		Text:      resp.Message.Content,
		Model:     resp.Model,
		Usage:     resp.Usage,
	})
}
