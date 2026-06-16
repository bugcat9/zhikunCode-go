package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"go-backend/internal/workspace"
)

var ErrWritesDisabled = errors.New("write_file is disabled until permission handling is configured")

type WriteFileTool struct {
	Boundary    workspace.Boundary
	AllowWrites bool
}

type writeFileInput struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	Append     bool   `json:"append,omitempty"`
	CreateDirs bool   `json:"create_dirs,omitempty"`
}

func NewWriteFileTool(workspacePath string, allowWrites bool) (*WriteFileTool, error) {
	boundary, err := workspace.NewBoundary(workspacePath)
	if err != nil {
		return nil, err
	}
	return &WriteFileTool{
		Boundary:    boundary,
		AllowWrites: allowWrites,
	}, nil
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Write a text file inside the workspace. Disabled by default until permission handling is configured."
}

func (t *WriteFileTool) Schema() any {
	return objectSchema(map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "Workspace-relative file path to write.",
		},
		"content": map[string]any{
			"type":        "string",
			"description": "Text content to write.",
		},
		"append": map[string]any{
			"type":        "boolean",
			"description": "Append instead of replacing the file.",
		},
		"create_dirs": map[string]any{
			"type":        "boolean",
			"description": "Create parent directories when needed.",
		},
	}, "path", "content")
}

func (t *WriteFileTool) Run(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	select {
	case <-ctx.Done():
		return ToolResult{}, ctx.Err()
	default:
	}

	req := writeFileInput{}
	if err := decodeInput(input, &req); err != nil {
		return ToolResult{}, err
	}

	if !t.AllowWrites {
		return ToolResult{Error: ErrWritesDisabled.Error()}, ErrWritesDisabled
	}

	path, err := t.Boundary.Resolve(req.Path)
	if err != nil {
		return ToolResult{}, err
	}

	if req.CreateDirs {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return ToolResult{}, err
		}
	}

	flag := os.O_CREATE | os.O_WRONLY
	if req.Append {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	file, err := os.OpenFile(path, flag, 0644)
	if err != nil {
		return ToolResult{}, err
	}
	defer file.Close()

	written, err := file.WriteString(req.Content)
	if err != nil {
		return ToolResult{}, err
	}

	relPath, err := t.Boundary.Relative(path)
	if err != nil {
		return ToolResult{}, err
	}

	return ToolResult{
		Content: "file written",
		Data: map[string]any{
			"path":    relPath,
			"bytes":   written,
			"append":  req.Append,
			"created": req.CreateDirs,
		},
	}, nil
}
