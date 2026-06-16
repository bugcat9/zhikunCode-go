package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go-backend/internal/python"
)

type TokenCountTool struct {
	Client *python.Client
}

type tokenCountInput struct {
	Text  string `json:"text"`
	Model string `json:"model,omitempty"`
}

func NewTokenCountTool(client *python.Client) *TokenCountTool {
	if client == nil {
		client = python.NewClient("")
	}
	return &TokenCountTool{Client: client}
}

func (t *TokenCountTool) Name() string {
	return "token_count"
}

func (t *TokenCountTool) Description() string {
	return "Count tokens for a text string using the Python tokenizer service."
}

func (t *TokenCountTool) Schema() any {
	return objectSchema(map[string]any{
		"text": map[string]any{
			"type":        "string",
			"description": "Text to count.",
		},
		"model": map[string]any{
			"type":        "string",
			"description": "Optional model name for tokenizer-specific counting.",
		},
	}, "text")
}

func (t *TokenCountTool) Run(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	req := tokenCountInput{}
	if err := decodeInput(input, &req); err != nil {
		return ToolResult{}, err
	}
	if strings.TrimSpace(req.Text) == "" {
		return ToolResult{}, fmt.Errorf("%w: text is required", ErrInvalidInput)
	}

	resp, err := t.Client.CountTokens(ctx, python.TokenCountRequest{
		Text:  req.Text,
		Model: req.Model,
	})
	if err != nil {
		return ToolResult{}, err
	}

	return ToolResult{
		Content: fmt.Sprintf("%d tokens", resp.TokenCount),
		Data:    resp,
	}, nil
}
