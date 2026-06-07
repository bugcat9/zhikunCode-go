package python

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCountTokensMapsPythonResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/tokenizer/count" {
			t.Fatalf("path = %s, want /api/tokenizer/count", r.URL.Path)
		}

		var req TokenCountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Text != "hello world" || req.Model != "default" {
			t.Fatalf("request = %+v", req)
		}

		writeTestJSON(t, w, map[string]any{
			"token_count": 7,
			"model":       "default",
			"elapsed_ms":  1.5,
		})
	}))
	defer server.Close()

	resp, err := NewClient(server.URL).CountTokens(context.Background(), TokenCountRequest{
		Text:  "hello world",
		Model: "default",
	})
	if err != nil {
		t.Fatalf("CountTokens returned error: %v", err)
	}
	if resp.TokenCount != 7 || resp.Count != 7 {
		t.Fatalf("token counts = token_count:%d count:%d, want 7/7", resp.TokenCount, resp.Count)
	}
	if resp.Model != "default" || resp.ElapsedMS != 1.5 {
		t.Fatalf("response metadata = model:%q elapsed:%v", resp.Model, resp.ElapsedMS)
	}
}

func TestClientGenerateDiagramMapsRequestAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/analysis/generate-diagram" {
			t.Fatalf("path = %s, want /api/analysis/generate-diagram", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["diagram_type"] != "sequence" || body["target"] != "main" || body["project_root"] != "/repo" {
			t.Fatalf("request body = %#v", body)
		}
		options, ok := body["options"].(map[string]any)
		if !ok {
			t.Fatalf("options missing or wrong type: %#v", body["options"])
		}
		if int(options["depth"].(float64)) != 4 || options["format"] != "mermaid" || options["include_tests"] != false {
			t.Fatalf("options = %#v", options)
		}

		writeTestJSON(t, w, map[string]any{
			"diagram_type":     "sequence",
			"mermaid_syntax":   "sequenceDiagram\nA->>B: call",
			"confidence_score": 0.9,
			"metadata": map[string]any{
				"nodes_count":        2,
				"edges_count":        1,
				"languages_analyzed": []string{"go"},
				"analysis_time_ms":   12.3,
			},
			"warnings": []string{"partial"},
		})
	}))
	defer server.Close()

	resp, err := NewClient(server.URL).GenerateDiagram(context.Background(), DiagramRequest{
		DiagramType: "sequence",
		Target:      "main",
		ProjectRoot: "/repo",
		Depth:       4,
	})
	if err != nil {
		t.Fatalf("GenerateDiagram returned error: %v", err)
	}
	if !resp.Success || resp.DiagramType != "sequence" || resp.MermaidSyntax == "" {
		t.Fatalf("diagram response = %+v", resp)
	}
	if resp.Metadata == nil || resp.Metadata.NodesCount != 2 || resp.Metadata.AnalysisTimeMS != 12.3 {
		t.Fatalf("metadata = %+v", resp.Metadata)
	}
	if len(resp.Warnings) != 1 || resp.Warnings[0] != "partial" {
		t.Fatalf("warnings = %#v", resp.Warnings)
	}
}

func TestClientExtractAPIEndpointsMapsRequestAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/analysis/api-endpoints" {
			t.Fatalf("path = %s, want /api/analysis/api-endpoints", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["project_root"] != "/repo" {
			t.Fatalf("project_root = %#v", body["project_root"])
		}
		languages := body["languages"].([]any)
		if len(languages) != 1 || languages[0] != "go" {
			t.Fatalf("languages = %#v", languages)
		}

		writeTestJSON(t, w, map[string]any{
			"success": true,
			"endpoints": []map[string]any{
				{
					"http_method":      "GET",
					"path":             "/api/health",
					"handler_function": "healthHandler",
					"handler_class":    "",
					"file_path":        "internal/api/health_handler.go",
					"line_number":      12,
					"language":         "go",
					"parameters":       []map[string]string{{"name": "id", "type": "string"}},
				},
			},
			"total":      1,
			"elapsed_ms": 3.5,
		})
	}))
	defer server.Close()

	resp, err := NewClient(server.URL).ExtractAPIEndpoints(context.Background(), APIEndpointsRequest{
		ProjectRoot: "/repo",
		Languages:   []string{"go"},
	})
	if err != nil {
		t.Fatalf("ExtractAPIEndpoints returned error: %v", err)
	}
	if !resp.Success || resp.Total != 1 || resp.ElapsedMS != 3.5 || len(resp.Endpoints) != 1 {
		t.Fatalf("endpoints response = %+v", resp)
	}
	endpoint := resp.Endpoints[0]
	if endpoint.HTTPMethod != "GET" || endpoint.FilePath != "internal/api/health_handler.go" || endpoint.LineNumber != 12 {
		t.Fatalf("endpoint = %+v", endpoint)
	}
}

func TestClientAnalyzeCodePathsMapsRequestAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/analysis/code-path" {
			t.Fatalf("path = %s, want /api/analysis/code-path", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["project_root"] != "/repo" || body["entry_file"] != "main.go" || body["entry_function"] != "main" {
			t.Fatalf("request body = %#v", body)
		}
		if int(body["max_depth"].(float64)) != 5 {
			t.Fatalf("max_depth = %#v", body["max_depth"])
		}

		writeTestJSON(t, w, map[string]any{
			"success": true,
			"data": map[string]any{
				"nodes": []map[string]any{
					{
						"id":          "n1",
						"name":        "main",
						"class_name":  "Main",
						"file_path":   "main.go",
						"line_range":  []int{10, 20},
						"layer":       "controller",
						"node_type":   "api",
						"annotations": []string{"GetMapping"},
						"parameters":  []map[string]string{{"name": "ctx", "type": "context.Context"}},
						"return_type": "error",
					},
				},
				"edges": []map[string]any{
					{
						"source":            "n1",
						"target":            "n2",
						"call_type":         "method_call",
						"parameter_mapping": map[string]string{"ctx": "ctx"},
					},
				},
				"layers": []map[string]any{
					{
						"layer":       "controller",
						"node_count":  1,
						"description": "controller layer",
					},
				},
				"analysis_time_ms": 9.5,
				"warnings":         []string{"truncated"},
			},
		})
	}))
	defer server.Close()

	resp, err := NewClient(server.URL).AnalyzeCodePaths(context.Background(), CodePathRequest{
		ProjectRoot:   "/repo",
		EntryFile:     "main.go",
		EntryFunction: "main",
		MaxDepth:      5,
	})
	if err != nil {
		t.Fatalf("AnalyzeCodePaths returned error: %v", err)
	}
	if !resp.Success || resp.ElapsedMS != 9.5 || len(resp.Nodes) != 1 || len(resp.Edges) != 1 || len(resp.Layers) != 1 {
		t.Fatalf("code path response = %+v", resp)
	}
	if resp.Nodes[0].ClassName != "Main" || resp.Nodes[0].LineRange[1] != 20 {
		t.Fatalf("node = %+v", resp.Nodes[0])
	}
	if resp.Edges[0].CallType != "method_call" || resp.Edges[0].ParameterMapping["ctx"] != "ctx" {
		t.Fatalf("edge = %+v", resp.Edges[0])
	}
	if len(resp.Warnings) != 1 || resp.Warnings[0] != "truncated" {
		t.Fatalf("warnings = %#v", resp.Warnings)
	}
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
