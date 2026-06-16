package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go-backend/internal/workspace"
)

const DefaultListMaxEntries = 200

type ListFilesTool struct {
	Boundary   workspace.Boundary
	MaxEntries int
}

type listFilesInput struct {
	Path       string `json:"path,omitempty"`
	Recursive  bool   `json:"recursive,omitempty"`
	MaxEntries int    `json:"max_entries,omitempty"`
}

type FileEntry struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size,omitempty"`
}

func NewListFilesTool(workspacePath string) (*ListFilesTool, error) {
	boundary, err := workspace.NewBoundary(workspacePath)
	if err != nil {
		return nil, err
	}
	return &ListFilesTool{
		Boundary:   boundary,
		MaxEntries: DefaultListMaxEntries,
	}, nil
}

func (t *ListFilesTool) Name() string {
	return "list_files"
}

func (t *ListFilesTool) Description() string {
	return "List files and directories inside the workspace."
}

func (t *ListFilesTool) Schema() any {
	return objectSchema(map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "Workspace-relative directory path to list.",
		},
		"recursive": map[string]any{
			"type":        "boolean",
			"description": "Whether to recursively walk subdirectories.",
		},
		"max_entries": map[string]any{
			"type":        "integer",
			"description": "Optional maximum number of entries to return.",
		},
	})
}

func (t *ListFilesTool) Run(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	req := listFilesInput{}
	if err := decodeInput(input, &req); err != nil {
		return ToolResult{}, err
	}
	if strings.TrimSpace(req.Path) == "" {
		req.Path = "."
	}

	path, err := t.Boundary.Resolve(req.Path)
	if err != nil {
		return ToolResult{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return ToolResult{}, err
	}
	if !info.IsDir() {
		return ToolResult{}, fmt.Errorf("%w: %s is not a directory", ErrInvalidInput, req.Path)
	}

	maxEntries := t.MaxEntries
	if req.MaxEntries > 0 {
		maxEntries = req.MaxEntries
	}
	if maxEntries <= 0 {
		maxEntries = DefaultListMaxEntries
	}

	entries := make([]FileEntry, 0)
	truncated := false
	addEntry := func(absPath string, info fs.FileInfo) error {
		if len(entries) >= maxEntries {
			truncated = true
			return fs.SkipAll
		}

		relPath, err := t.Boundary.Relative(absPath)
		if err != nil {
			return err
		}
		entryType := "file"
		if info.IsDir() {
			entryType = "directory"
		}
		entries = append(entries, FileEntry{
			Path: relPath,
			Name: info.Name(),
			Type: entryType,
			Size: info.Size(),
		})
		return nil
	}

	if req.Recursive {
		err = filepath.WalkDir(path, func(absPath string, dirEntry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if absPath == path {
				return nil
			}
			info, err := dirEntry.Info()
			if err != nil {
				return err
			}
			return addEntry(absPath, info)
		})
		if err != nil && err != fs.SkipAll {
			return ToolResult{}, err
		}
	} else {
		dirEntries, err := os.ReadDir(path)
		if err != nil {
			return ToolResult{}, err
		}
		for _, dirEntry := range dirEntries {
			select {
			case <-ctx.Done():
				return ToolResult{}, ctx.Err()
			default:
			}
			info, err := dirEntry.Info()
			if err != nil {
				return ToolResult{}, err
			}
			if err := addEntry(filepath.Join(path, dirEntry.Name()), info); err != nil {
				if err == fs.SkipAll {
					break
				}
				return ToolResult{}, err
			}
		}
	}

	return ToolResult{
		Content: formatFileEntries(entries),
		Data: map[string]any{
			"path":      req.Path,
			"entries":   entries,
			"truncated": truncated,
		},
	}, nil
}

func formatFileEntries(entries []FileEntry) string {
	if len(entries) == 0 {
		return "(empty)"
	}

	var builder strings.Builder
	for _, entry := range entries {
		builder.WriteString(entry.Type)
		builder.WriteByte('\t')
		builder.WriteString(entry.Path)
		builder.WriteByte('\n')
	}
	return strings.TrimRight(builder.String(), "\n")
}
