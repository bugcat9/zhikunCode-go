package agent

import "time"

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCanceled  TaskStatus = "canceled"
)

type Task struct {
	ID           string     `json:"id"`
	ParentID     string     `json:"parent_id,omitempty"`
	SessionID    string     `json:"session_id,omitempty"`
	Model        string     `json:"model,omitempty"`
	Instruction  string     `json:"instruction"`
	SystemPrompt string     `json:"system_prompt,omitempty"`
	Status       TaskStatus `json:"status"`
	CreatedAt    time.Time  `json:"created_at,omitempty"`
	StartedAt    time.Time  `json:"started_at,omitempty"`
	CompletedAt  time.Time  `json:"completed_at,omitempty"`
}
