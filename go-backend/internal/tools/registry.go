package tools

import (
	"sort"

	"go-backend/internal/python"
)

type Registry struct {
	tools map[string]Tool
}

func NewRegistry(toolList ...Tool) *Registry {
	registry := &Registry{
		tools: make(map[string]Tool),
	}
	for _, tool := range toolList {
		registry.Register(tool)
	}
	return registry
}

func NewDefaultRegistry(workspacePath string, pythonClient *python.Client) (*Registry, error) {
	listFiles, err := NewListFilesTool(workspacePath)
	if err != nil {
		return nil, err
	}
	readFile, err := NewReadFileTool(workspacePath)
	if err != nil {
		return nil, err
	}
	writeFile, err := NewWriteFileTool(workspacePath, false)
	if err != nil {
		return nil, err
	}
	pythonAnalysis, err := NewPythonAnalysisTool(workspacePath, pythonClient)
	if err != nil {
		return nil, err
	}

	return NewRegistry(
		listFiles,
		readFile,
		writeFile,
		pythonAnalysis,
		NewTokenCountTool(pythonClient),
	), nil
}

func (r *Registry) Register(tool Tool) {
	if tool == nil || tool.Name() == "" {
		return
	}
	if r.tools == nil {
		r.tools = make(map[string]Tool)
	}
	r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) (Tool, bool) {
	if r == nil || r.tools == nil {
		return nil, false
	}
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *Registry) Definitions() []ToolDefinition {
	if r == nil || len(r.tools) == 0 {
		return []ToolDefinition{}
	}

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	definitions := make([]ToolDefinition, 0, len(names))
	for _, name := range names {
		tool := r.tools[name]
		definitions = append(definitions, ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Schema:      tool.Schema(),
		})
	}
	return definitions
}
