package python

type DiagramRequest struct {
	WorkingDirectory string   `json:"workingDirectory,omitempty"`
	FilePaths        []string `json:"filePaths,omitempty"`
	DiagramType      string   `json:"diagramType,omitempty"`
}

type DiagramResponse struct {
	Mermaid string `json:"mermaid"`
}

type APIEndpointsRequest struct {
	WorkingDirectory string   `json:"workingDirectory,omitempty"`
	FilePaths        []string `json:"filePaths,omitempty"`
}

type APIEndpointsResponse struct {
	Endpoints []APIEndpoint `json:"endpoints"`
}

type APIEndpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
}

type CodePathRequest struct {
	WorkingDirectory string `json:"workingDirectory,omitempty"`
	Symbol           string `json:"symbol,omitempty"`
	FilePath         string `json:"filePath,omitempty"`
}

type CodePathResponse struct {
	Paths []CodePathStep `json:"paths"`
}

type CodePathStep struct {
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Symbol      string `json:"symbol,omitempty"`
	Description string `json:"description,omitempty"`
}
