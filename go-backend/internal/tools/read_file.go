package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"go-backend/internal/workspace"
)

const DefaultReadMaxBytes int64 = 256 * 1024

type ReadFileTool struct {
	Boundary workspace.Boundary
	MaxBytes int64
}

type readFileInput struct {
	Path     string `json:"path"`
	MaxBytes int64  `json:"max_bytes,omitempty"`
}

func NewReadFileTool(workspacePath string) (*ReadFileTool, error) {
	boundary, err := workspace.NewBoundary(workspacePath)
	if err != nil {
		return nil, err
	}
	return &ReadFileTool{
		Boundary: boundary,
		MaxBytes: DefaultReadMaxBytes,
	}, nil
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read a UTF-8 text file inside the workspace."
}

func (t *ReadFileTool) Schema() any {
	return objectSchema(map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "Workspace-relative file path to read.",
		},
		"max_bytes": map[string]any{
			"type":        "integer",
			"description": "Optional maximum number of bytes to read.",
		},
	}, "path")
}

func (t *ReadFileTool) Run(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	select {
	case <-ctx.Done():
		return ToolResult{}, ctx.Err()
	default:
	}

	req := readFileInput{}
	if err := decodeInput(input, &req); err != nil {
		return ToolResult{}, err
	}

	path, err := t.Boundary.Resolve(req.Path)
	if err != nil {
		return ToolResult{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return ToolResult{}, err
	}
	if info.IsDir() {
		return ToolResult{}, fmt.Errorf("%w: %s is a directory", ErrInvalidInput, req.Path)
	}

	maxBytes := t.MaxBytes
	if req.MaxBytes > 0 {
		maxBytes = req.MaxBytes
	}
	if maxBytes <= 0 {
		maxBytes = DefaultReadMaxBytes
	}

	file, err := os.Open(path)
	if err != nil {
		return ToolResult{}, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return ToolResult{}, err
	}

	truncated := int64(len(data)) > maxBytes
	if truncated {
		data = data[:maxBytes]
	}

	relPath, err := t.Boundary.Relative(path)
	if err != nil {
		return ToolResult{}, err
	}

	return ToolResult{
		Content: string(data),
		Data: map[string]any{
			"path":      relPath,
			"bytes":     len(data),
			"truncated": truncated,
		},
	}, nil
}
