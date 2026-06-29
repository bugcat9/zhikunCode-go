package agent

import (
	"time"

	"go-backend/internal/llm"
)

type AgentResult struct {
	TaskID      string     `json:"task_id"`
	AgentID     string     `json:"agent_id"`
	ParentID    string     `json:"parent_id,omitempty"`
	SessionID   string     `json:"session_id,omitempty"`
	Status      TaskStatus `json:"status"`
	Text        string     `json:"text,omitempty"`
	Model       string     `json:"model,omitempty"`
	Usage       llm.Usage  `json:"usage,omitempty"`
	Error       string     `json:"error,omitempty"`
	StartedAt   time.Time  `json:"started_at,omitempty"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
}
