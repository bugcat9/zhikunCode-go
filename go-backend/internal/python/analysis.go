package python

import "context"

type DiagramRequest struct {
	DiagramType string `json:"diagramType,omitempty"`
	Target      string `json:"target,omitempty"`
	ProjectRoot string `json:"projectRoot,omitempty"`
	Depth       int    `json:"depth,omitempty"`
}

type DiagramResponse struct {
	Success         bool             `json:"success"`
	DiagramType     string           `json:"diagramType,omitempty"`
	MermaidSyntax   string           `json:"mermaidSyntax,omitempty"`
	ConfidenceScore float64          `json:"confidenceScore,omitempty"`
	Metadata        *DiagramMetadata `json:"metadata,omitempty"`
	Warnings        []string         `json:"warnings"`
	Error           string           `json:"error,omitempty"`
}

type DiagramMetadata struct {
	NodesCount        int      `json:"nodesCount"`
	EdgesCount        int      `json:"edgesCount"`
	LanguagesAnalyzed []string `json:"languagesAnalyzed"`
	AnalysisTimeMS    float64  `json:"analysisTimeMs"`
}

type APIEndpointsRequest struct {
	ProjectRoot string   `json:"projectRoot,omitempty"`
	Languages   []string `json:"languages,omitempty"`
}

type APIEndpointsResponse struct {
	Success   bool          `json:"success"`
	Endpoints []APIEndpoint `json:"endpoints"`
	Total     int           `json:"total"`
	ElapsedMS float64       `json:"elapsedMs,omitempty"`
	Error     string        `json:"error,omitempty"`
}

type APIEndpoint struct {
	HTTPMethod      string              `json:"httpMethod"`
	Path            string              `json:"path"`
	HandlerFunction string              `json:"handlerFunction"`
	HandlerClass    string              `json:"handlerClass"`
	FilePath        string              `json:"filePath"`
	LineNumber      int                 `json:"lineNumber"`
	Language        string              `json:"language"`
	Parameters      []map[string]string `json:"parameters"`
}

type CodePathRequest struct {
	ProjectRoot   string `json:"projectRoot,omitempty"`
	EntryFile     string `json:"entryFile,omitempty"`
	EntryFunction string `json:"entryFunction,omitempty"`
	MaxDepth      int    `json:"maxDepth,omitempty"`
}

type CodePathResponse struct {
	Success   bool        `json:"success"`
	Nodes     []PathNode  `json:"nodes"`
	Edges     []PathEdge  `json:"edges"`
	Layers    []LayerInfo `json:"layers"`
	ElapsedMS float64     `json:"elapsedMs,omitempty"`
	Warnings  []string    `json:"warnings,omitempty"`
	Error     string      `json:"error,omitempty"`
}

type PathNode struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	ClassName   string              `json:"className"`
	FilePath    string              `json:"filePath"`
	LineRange   []int               `json:"lineRange"`
	Layer       string              `json:"layer"`
	NodeType    string              `json:"nodeType"`
	Annotations []string            `json:"annotations"`
	Parameters  []map[string]string `json:"parameters"`
	ReturnType  string              `json:"returnType"`
}

type PathEdge struct {
	Source           string            `json:"source"`
	Target           string            `json:"target"`
	CallType         string            `json:"callType"`
	ParameterMapping map[string]string `json:"parameterMapping,omitempty"`
}

type LayerInfo struct {
	Layer       string `json:"layer"`
	NodeCount   int    `json:"nodeCount"`
	Description string `json:"description"`
}

func GenerateDiagram(ctx context.Context, req DiagramRequest) (DiagramResponse, error) {
	client := NewClient("")
	return client.GenerateDiagram(ctx, req)
}

func ExtractAPIEndpoints(ctx context.Context, req APIEndpointsRequest) (APIEndpointsResponse, error) {
	client := NewClient("")
	return client.ExtractAPIEndpoints(ctx, req)
}

func AnalyzeCodePaths(ctx context.Context, req CodePathRequest) (CodePathResponse, error) {
	client := NewClient("")
	return client.AnalyzeCodePaths(ctx, req)
}

type pythonDiagramRequest struct {
	DiagramType string               `json:"diagram_type"`
	Target      string               `json:"target"`
	ProjectRoot string               `json:"project_root"`
	Options     pythonDiagramOptions `json:"options"`
}

type pythonDiagramOptions struct {
	Depth        int    `json:"depth"`
	IncludeTests bool   `json:"include_tests"`
	Format       string `json:"format"`
}

type pythonDiagramResponse struct {
	DiagramType     string                `json:"diagram_type"`
	MermaidSyntax   string                `json:"mermaid_syntax"`
	ConfidenceScore float64               `json:"confidence_score"`
	Metadata        pythonDiagramMetadata `json:"metadata"`
	Warnings        []string              `json:"warnings"`
	Error           string                `json:"error,omitempty"`
}

type pythonDiagramMetadata struct {
	NodesCount        int      `json:"nodes_count"`
	EdgesCount        int      `json:"edges_count"`
	LanguagesAnalyzed []string `json:"languages_analyzed"`
	AnalysisTimeMS    float64  `json:"analysis_time_ms"`
}

func (r DiagramRequest) toPythonRequest() pythonDiagramRequest {
	depth := r.Depth
	if depth == 0 {
		depth = 3
	}
	return pythonDiagramRequest{
		DiagramType: r.DiagramType,
		Target:      r.Target,
		ProjectRoot: r.ProjectRoot,
		Options: pythonDiagramOptions{
			Depth:        depth,
			IncludeTests: false,
			Format:       "mermaid",
		},
	}
}

func (r pythonDiagramResponse) toPublic() DiagramResponse {
	if r.Error != "" {
		return DiagramResponse{
			Success:  false,
			Warnings: emptyStringsIfNil(r.Warnings),
			Error:    r.Error,
		}
	}
	return DiagramResponse{
		Success:         true,
		DiagramType:     r.DiagramType,
		MermaidSyntax:   r.MermaidSyntax,
		ConfidenceScore: r.ConfidenceScore,
		Metadata: &DiagramMetadata{
			NodesCount:        r.Metadata.NodesCount,
			EdgesCount:        r.Metadata.EdgesCount,
			LanguagesAnalyzed: emptyStringsIfNil(r.Metadata.LanguagesAnalyzed),
			AnalysisTimeMS:    r.Metadata.AnalysisTimeMS,
		},
		Warnings: emptyStringsIfNil(r.Warnings),
	}
}

type pythonAPIEndpointsRequest struct {
	ProjectRoot string   `json:"project_root"`
	Languages   []string `json:"languages,omitempty"`
}

type pythonAPIEndpointsResponse struct {
	Success   bool                `json:"success"`
	Endpoints []pythonAPIEndpoint `json:"endpoints"`
	Total     int                 `json:"total"`
	ElapsedMS float64             `json:"elapsed_ms,omitempty"`
	Error     string              `json:"error,omitempty"`
}

type pythonAPIEndpoint struct {
	HTTPMethod      string              `json:"http_method"`
	Path            string              `json:"path"`
	HandlerFunction string              `json:"handler_function"`
	HandlerClass    string              `json:"handler_class"`
	FilePath        string              `json:"file_path"`
	LineNumber      int                 `json:"line_number"`
	Language        string              `json:"language"`
	Parameters      []map[string]string `json:"parameters"`
}

func (r APIEndpointsRequest) toPythonRequest() pythonAPIEndpointsRequest {
	return pythonAPIEndpointsRequest{
		ProjectRoot: r.ProjectRoot,
		Languages:   r.Languages,
	}
}

func (r pythonAPIEndpointsResponse) toPublic() APIEndpointsResponse {
	endpoints := make([]APIEndpoint, 0, len(r.Endpoints))
	for _, endpoint := range r.Endpoints {
		endpoints = append(endpoints, APIEndpoint{
			HTTPMethod:      endpoint.HTTPMethod,
			Path:            endpoint.Path,
			HandlerFunction: endpoint.HandlerFunction,
			HandlerClass:    endpoint.HandlerClass,
			FilePath:        endpoint.FilePath,
			LineNumber:      endpoint.LineNumber,
			Language:        endpoint.Language,
			Parameters:      emptyMapListIfNil(endpoint.Parameters),
		})
	}
	total := r.Total
	if total == 0 {
		total = len(endpoints)
	}
	return APIEndpointsResponse{
		Success:   r.Error == "" && (r.Success || len(endpoints) > 0 || total == 0),
		Endpoints: endpoints,
		Total:     total,
		ElapsedMS: r.ElapsedMS,
		Error:     r.Error,
	}
}

type pythonCodePathRequest struct {
	ProjectRoot   string `json:"project_root"`
	EntryFile     string `json:"entry_file"`
	EntryFunction string `json:"entry_function"`
	MaxDepth      int    `json:"max_depth"`
}

type pythonCodePathResponse struct {
	Success   bool               `json:"success"`
	Data      pythonCodePathData `json:"data"`
	ElapsedMS float64            `json:"elapsed_ms,omitempty"`
	Error     string             `json:"error,omitempty"`
}

type pythonCodePathData struct {
	Nodes          []pythonPathNode  `json:"nodes"`
	Edges          []pythonPathEdge  `json:"edges"`
	Layers         []pythonLayerInfo `json:"layers"`
	AnalysisTimeMS float64           `json:"analysis_time_ms"`
	Warnings       []string          `json:"warnings"`
}

type pythonPathNode struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	ClassName   string              `json:"class_name"`
	FilePath    string              `json:"file_path"`
	LineRange   []int               `json:"line_range"`
	Layer       string              `json:"layer"`
	NodeType    string              `json:"node_type"`
	Annotations []string            `json:"annotations"`
	Parameters  []map[string]string `json:"parameters"`
	ReturnType  string              `json:"return_type"`
}

type pythonPathEdge struct {
	Source           string            `json:"source"`
	Target           string            `json:"target"`
	CallType         string            `json:"call_type"`
	ParameterMapping map[string]string `json:"parameter_mapping"`
}

type pythonLayerInfo struct {
	Layer       string `json:"layer"`
	NodeCount   int    `json:"node_count"`
	Description string `json:"description"`
}

func (r CodePathRequest) toPythonRequest() pythonCodePathRequest {
	maxDepth := r.MaxDepth
	if maxDepth == 0 {
		maxDepth = 10
	}
	return pythonCodePathRequest{
		ProjectRoot:   r.ProjectRoot,
		EntryFile:     r.EntryFile,
		EntryFunction: r.EntryFunction,
		MaxDepth:      maxDepth,
	}
}

func (r pythonCodePathResponse) toPublic() CodePathResponse {
	nodes := make([]PathNode, 0, len(r.Data.Nodes))
	for _, node := range r.Data.Nodes {
		nodes = append(nodes, PathNode{
			ID:          node.ID,
			Name:        node.Name,
			ClassName:   node.ClassName,
			FilePath:    node.FilePath,
			LineRange:   emptyIntsIfNil(node.LineRange),
			Layer:       node.Layer,
			NodeType:    node.NodeType,
			Annotations: emptyStringsIfNil(node.Annotations),
			Parameters:  emptyMapListIfNil(node.Parameters),
			ReturnType:  node.ReturnType,
		})
	}

	edges := make([]PathEdge, 0, len(r.Data.Edges))
	for _, edge := range r.Data.Edges {
		edges = append(edges, PathEdge{
			Source:           edge.Source,
			Target:           edge.Target,
			CallType:         edge.CallType,
			ParameterMapping: edge.ParameterMapping,
		})
	}

	layers := make([]LayerInfo, 0, len(r.Data.Layers))
	for _, layer := range r.Data.Layers {
		layers = append(layers, LayerInfo{
			Layer:       layer.Layer,
			NodeCount:   layer.NodeCount,
			Description: layer.Description,
		})
	}

	elapsedMS := r.Data.AnalysisTimeMS
	if elapsedMS == 0 {
		elapsedMS = r.ElapsedMS
	}

	return CodePathResponse{
		Success:   r.Error == "" && (r.Success || len(nodes) > 0 || len(edges) > 0 || len(layers) > 0),
		Nodes:     nodes,
		Edges:     edges,
		Layers:    layers,
		ElapsedMS: elapsedMS,
		Warnings:  emptyStringsIfNil(r.Data.Warnings),
		Error:     r.Error,
	}
}

func emptyStringsIfNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func emptyIntsIfNil(values []int) []int {
	if values == nil {
		return []int{}
	}
	return values
}

func emptyMapListIfNil(values []map[string]string) []map[string]string {
	if values == nil {
		return []map[string]string{}
	}
	return values
}
