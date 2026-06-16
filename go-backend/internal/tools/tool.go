package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go-backend/internal/llm"
)

var ErrInvalidInput = errors.New("invalid tool input")

type Tool interface {
	Name() string
	Description() string
	Schema() any
	Run(ctx context.Context, input json.RawMessage) (ToolResult, error)
}

type ToolRegistry interface {
	Register(tool Tool)
	Get(name string) (Tool, bool)
	Definitions() []ToolDefinition
}

type ToolDefinition = llm.ToolDefinition

type ToolResult struct {
	Content string `json:"content,omitempty"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func decodeInput(input json.RawMessage, target any) error {
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(input, target); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return nil
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
