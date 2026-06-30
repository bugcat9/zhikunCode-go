package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type CoordinatorRequest struct {
	ID             string        `json:"id,omitempty"`
	SessionID      string        `json:"session_id,omitempty"`
	Model          string        `json:"model,omitempty"`
	Instruction    string        `json:"instruction,omitempty"`
	SystemPrompt   string        `json:"system_prompt,omitempty"`
	Tasks          []Task        `json:"tasks,omitempty"`
	MaxConcurrency int           `json:"max_concurrency,omitempty"`
	Timeout        time.Duration `json:"timeout,omitempty"`
}

type TaskPlanner interface {
	Plan(ctx context.Context, req CoordinatorRequest) ([]Task, error)
}

type StaticTaskPlanner struct{}

func NewStaticTaskPlanner() StaticTaskPlanner {
	return StaticTaskPlanner{}
}

func (StaticTaskPlanner) Plan(ctx context.Context, req CoordinatorRequest) ([]Task, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if len(req.Tasks) > 0 {
		tasks := make([]Task, len(req.Tasks))
		copy(tasks, req.Tasks)
		return tasks, nil
	}

	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" {
		return nil, fmt.Errorf("%w: instruction is required", ErrInvalidTask)
	}

	return []Task{
		{
			SessionID:    strings.TrimSpace(req.SessionID),
			Model:        strings.TrimSpace(req.Model),
			Instruction:  instruction,
			SystemPrompt: strings.TrimSpace(req.SystemPrompt),
		},
	}, nil
}
