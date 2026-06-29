package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-backend/internal/llm"
)

var ErrTaskSpawnerUnavailable = errors.New("task spawner is not configured")

type TaskCreateRequest struct {
	TaskID       string `json:"task_id,omitempty"`
	ParentID     string `json:"parent_id,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
	Model        string `json:"model,omitempty"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	Instruction  string `json:"instruction"`
}

type TaskCreateResult struct {
	TaskID      string    `json:"task_id"`
	AgentID     string    `json:"agent_id"`
	ParentID    string    `json:"parent_id,omitempty"`
	SessionID   string    `json:"session_id,omitempty"`
	Status      string    `json:"status"`
	Text        string    `json:"text,omitempty"`
	Model       string    `json:"model,omitempty"`
	Usage       llm.Usage `json:"usage,omitempty"`
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

type TaskSpawner interface {
	CreateTask(ctx context.Context, req TaskCreateRequest) (TaskCreateResult, error)
}

type TaskCreateTool struct {
	Spawner TaskSpawner
}

func NewTaskCreateTool(spawner TaskSpawner) *TaskCreateTool {
	return &TaskCreateTool{Spawner: spawner}
}

func (t *TaskCreateTool) Name() string {
	return "task_create"
}

func (t *TaskCreateTool) Description() string {
	return "Create a SubAgent to run an isolated instruction and return the result."
}

func (t *TaskCreateTool) Schema() any {
	return objectSchema(map[string]any{
		"instruction": map[string]any{
			"type":        "string",
			"description": "The standalone instruction the SubAgent should execute.",
		},
		"model": map[string]any{
			"type":        "string",
			"description": "Optional model override for the SubAgent.",
		},
		"system_prompt": map[string]any{
			"type":        "string",
			"description": "Optional system prompt override for the SubAgent.",
		},
		"session_id": map[string]any{
			"type":        "string",
			"description": "Optional existing session id. Leave empty to create an independent SubAgent session.",
		},
		"parent_id": map[string]any{
			"type":        "string",
			"description": "Optional parent task or agent id for tracing.",
		},
	}, "instruction")
}

func (t *TaskCreateTool) Run(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	select {
	case <-ctx.Done():
		return ToolResult{}, ctx.Err()
	default:
	}

	if t == nil || t.Spawner == nil {
		return ToolResult{}, ErrTaskSpawnerUnavailable
	}

	req := TaskCreateRequest{}
	if err := decodeInput(input, &req); err != nil {
		return ToolResult{}, err
	}
	req.Instruction = strings.TrimSpace(req.Instruction)
	if req.Instruction == "" {
		return ToolResult{}, fmt.Errorf("%w: instruction is required", ErrInvalidInput)
	}

	result, err := t.Spawner.CreateTask(ctx, req)
	toolResult := ToolResult{
		Content: formatTaskCreateResult(result),
		Data:    result,
	}
	if err != nil {
		if result.Error == "" {
			result.Error = err.Error()
		}
		toolResult.Content = formatTaskCreateResult(result)
		toolResult.Data = result
		toolResult.Error = err.Error()
		return toolResult, err
	}
	return toolResult, nil
}

func formatTaskCreateResult(result TaskCreateResult) string {
	status := strings.TrimSpace(result.Status)
	if status == "" {
		status = "unknown"
	}

	if result.Error != "" {
		return fmt.Sprintf("SubAgent task %s ended with status %s: %s", result.TaskID, status, result.Error)
	}
	if strings.TrimSpace(result.Text) == "" {
		return fmt.Sprintf("SubAgent task %s ended with status %s.", result.TaskID, status)
	}
	return fmt.Sprintf("SubAgent task %s ended with status %s:\n%s", result.TaskID, status, result.Text)
}
