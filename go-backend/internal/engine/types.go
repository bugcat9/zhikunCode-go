package engine

import "go-backend/internal/llm"

type QueryRequest struct {
	SessionID    string `json:"session_id,omitempty"`
	Model        string `json:"model,omitempty"`
	Prompt       string `json:"prompt"`
	SystemPrompt string `json:"system_prompt,omitempty"`
}

type QueryResult struct {
	SessionID string    `json:"session_id"`
	Text      string    `json:"text"`
	Model     string    `json:"model,omitempty"`
	Usage     llm.Usage `json:"usage,omitempty"`
}

type Config struct {
	DefaultSystemPrompt string
	MaxHistoryMessages  int
}

func (c Config) WithDefaults() Config {
	if c.DefaultSystemPrompt == "" {
		c.DefaultSystemPrompt = "You are a helpful coding assistant."
	}
	if c.MaxHistoryMessages <= 0 {
		c.MaxHistoryMessages = 20
	}
	return c
}
