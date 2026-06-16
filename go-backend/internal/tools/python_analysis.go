package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go-backend/internal/python"
	"go-backend/internal/workspace"
)

const (
	PythonAnalysisGenerateDiagram = "generate_diagram"
	PythonAnalysisAPIEndpoints    = "api_endpoints"
	PythonAnalysisCodePath        = "code_path"
)

type PythonAnalysisTool struct {
	Client   *python.Client
	Boundary workspace.Boundary
}

type pythonAnalysisInput struct {
	Operation     string   `json:"operation"`
	ProjectRoot   string   `json:"project_root,omitempty"`
	DiagramType   string   `json:"diagram_type,omitempty"`
	Target        string   `json:"target,omitempty"`
	Depth         int      `json:"depth,omitempty"`
	Languages     []string `json:"languages,omitempty"`
	EntryFile     string   `json:"entry_file,omitempty"`
	EntryFunction string   `json:"entry_function,omitempty"`
	MaxDepth      int      `json:"max_depth,omitempty"`
}

func NewPythonAnalysisTool(workspacePath string, client *python.Client) (*PythonAnalysisTool, error) {
	boundary, err := workspace.NewBoundary(workspacePath)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = python.NewClient("")
	}
	return &PythonAnalysisTool{
		Client:   client,
		Boundary: boundary,
	}, nil
}

func (t *PythonAnalysisTool) Name() string {
	return "python_analysis"
}

func (t *PythonAnalysisTool) Description() string {
	return "Call the Python analysis service for diagrams, API endpoint scanning, or code path tracing."
}

func (t *PythonAnalysisTool) Schema() any {
	return objectSchema(map[string]any{
		"operation": map[string]any{
			"type":        "string",
			"description": "Analysis operation to run.",
			"enum":        []string{PythonAnalysisGenerateDiagram, PythonAnalysisAPIEndpoints, PythonAnalysisCodePath},
		},
		"project_root": map[string]any{
			"type":        "string",
			"description": "Workspace-relative project root. Defaults to the workspace root.",
		},
		"diagram_type": map[string]any{
			"type":        "string",
			"description": "Diagram type for generate_diagram.",
		},
		"target": map[string]any{
			"type":        "string",
			"description": "Analysis target for generate_diagram.",
		},
		"depth": map[string]any{
			"type":        "integer",
			"description": "Diagram traversal depth.",
		},
		"languages": map[string]any{
			"type":        "array",
			"description": "Languages to include when scanning API endpoints.",
			"items":       map[string]any{"type": "string"},
		},
		"entry_file": map[string]any{
			"type":        "string",
			"description": "Workspace-relative entry file for code_path.",
		},
		"entry_function": map[string]any{
			"type":        "string",
			"description": "Entry function for code_path.",
		},
		"max_depth": map[string]any{
			"type":        "integer",
			"description": "Maximum traversal depth for code_path.",
		},
	}, "operation")
}

func (t *PythonAnalysisTool) Run(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	req := pythonAnalysisInput{}
	if err := decodeInput(input, &req); err != nil {
		return ToolResult{}, err
	}

	projectRoot, err := t.resolveOptionalPath(req.ProjectRoot, ".")
	if err != nil {
		return ToolResult{}, err
	}

	switch strings.TrimSpace(req.Operation) {
	case PythonAnalysisGenerateDiagram:
		return t.generateDiagram(ctx, req, projectRoot)
	case PythonAnalysisAPIEndpoints:
		return t.extractAPIEndpoints(ctx, req, projectRoot)
	case PythonAnalysisCodePath:
		return t.analyzeCodePath(ctx, req, projectRoot)
	default:
		return ToolResult{}, fmt.Errorf("%w: unsupported python_analysis operation %q", ErrInvalidInput, req.Operation)
	}
}

func (t *PythonAnalysisTool) generateDiagram(ctx context.Context, req pythonAnalysisInput, projectRoot string) (ToolResult, error) {
	diagramType := strings.TrimSpace(req.DiagramType)
	if diagramType == "" {
		diagramType = "class"
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		target = projectRoot
	}

	resp, err := t.Client.GenerateDiagram(ctx, python.DiagramRequest{
		DiagramType: diagramType,
		Target:      target,
		ProjectRoot: projectRoot,
		Depth:       req.Depth,
	})
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		Content: "python_analysis generate_diagram completed",
		Data:    resp,
	}, nil
}

func (t *PythonAnalysisTool) extractAPIEndpoints(ctx context.Context, req pythonAnalysisInput, projectRoot string) (ToolResult, error) {
	resp, err := t.Client.ExtractAPIEndpoints(ctx, python.APIEndpointsRequest{
		ProjectRoot: projectRoot,
		Languages:   req.Languages,
	})
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		Content: "python_analysis api_endpoints completed",
		Data:    resp,
	}, nil
}

func (t *PythonAnalysisTool) analyzeCodePath(ctx context.Context, req pythonAnalysisInput, projectRoot string) (ToolResult, error) {
	entryFile := strings.TrimSpace(req.EntryFile)
	if entryFile != "" {
		var err error
		entryFile, err = t.Boundary.Resolve(entryFile)
		if err != nil {
			return ToolResult{}, err
		}
	}

	resp, err := t.Client.AnalyzeCodePaths(ctx, python.CodePathRequest{
		ProjectRoot:   projectRoot,
		EntryFile:     entryFile,
		EntryFunction: req.EntryFunction,
		MaxDepth:      req.MaxDepth,
	})
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		Content: "python_analysis code_path completed",
		Data:    resp,
	}, nil
}

func (t *PythonAnalysisTool) resolveOptionalPath(value string, fallback string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	return t.Boundary.Resolve(value)
}
