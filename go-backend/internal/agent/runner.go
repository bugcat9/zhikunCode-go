package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-backend/internal/engine"
)

func (a *Agent) Run(ctx context.Context, task Task) (AgentResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if a == nil {
		return AgentResult{}, ErrInvalidAgent
	}
	if a.Engine == nil {
		return AgentResult{}, ErrNilEngine
	}

	instruction := strings.TrimSpace(task.Instruction)
	if instruction == "" {
		return AgentResult{}, fmt.Errorf("%w: instruction is required", ErrInvalidTask)
	}

	startedAt := time.Now().UTC()
	result := AgentResult{
		TaskID:    task.ID,
		AgentID:   a.ID,
		ParentID:  task.ParentID,
		Status:    TaskStatusRunning,
		StartedAt: startedAt,
	}

	queryResult, err := a.Engine.Query(ctx, engine.QueryRequest{
		SessionID:    task.SessionID,
		Model:        task.Model,
		Prompt:       instruction,
		SystemPrompt: task.SystemPrompt,
	})

	result.CompletedAt = time.Now().UTC()
	if err != nil {
		result.Status = statusForError(ctx, err)
		result.Error = err.Error()
		return result, err
	}

	result.Status = TaskStatusCompleted
	result.SessionID = queryResult.SessionID
	result.Text = queryResult.Text
	result.Model = queryResult.Model
	result.Usage = queryResult.Usage
	return result, nil
}

func statusForError(ctx context.Context, err error) TaskStatus {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return TaskStatusCanceled
	}
	if ctx != nil && ctx.Err() != nil {
		return TaskStatusCanceled
	}
	return TaskStatusFailed
}
