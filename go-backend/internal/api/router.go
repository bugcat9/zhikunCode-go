package api

import "net/http"

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", healthHandler)
	mux.HandleFunc("GET /api/doctor", doctorHandler)
	mux.HandleFunc("GET /api/config", configHandler)
	mux.HandleFunc("PUT /api/config", configHandler)
	mux.HandleFunc("GET /api/models", modelsHandler)
	mux.HandleFunc("GET /api/sessions", sessionsHandler)
	mux.HandleFunc("POST /api/sessions", sessionsHandler)
	mux.HandleFunc("GET /api/sessions/{sessionId}", sessionHandler)
	mux.HandleFunc("DELETE /api/sessions/{sessionId}", sessionHandler)
	mux.HandleFunc("GET /api/sessions/{sessionId}/messages", sessionMessagesHandler)
	mux.HandleFunc("GET /api/sessions/{sessionId}/activities", sessionActivitiesHandler)
	mux.HandleFunc("GET /api/sessions/{sessionId}/history/snapshots", sessionHistorySnapshotsHandler)
	mux.HandleFunc("GET /api/sessions/{sessionId}/history/diff", sessionHistoryDiffHandler)
	mux.HandleFunc("GET /api/mcp/resources", mcpResourcesHandler)
	mux.HandleFunc("GET /api/mcp/resources/read", mcpResourceReadHandler)
	mux.HandleFunc("GET /api/mcp/prompts", mcpPromptsHandler)
	mux.HandleFunc("POST /api/mcp/prompts/execute", mcpPromptExecuteHandler)
	mux.HandleFunc("GET /api/mcp/capabilities", mcpCapabilitiesHandler)
	mux.HandleFunc("POST /api/mcp/capabilities", mcpCapabilitiesHandler)
	mux.HandleFunc("GET /api/mcp/capabilities/domains", mcpCapabilityDomainsHandler)
	mux.HandleFunc("GET /api/mcp/capabilities/{id}", mcpCapabilityHandler)
	mux.HandleFunc("PUT /api/mcp/capabilities/{id}", mcpCapabilityHandler)
	mux.HandleFunc("DELETE /api/mcp/capabilities/{id}", mcpCapabilityHandler)
	mux.HandleFunc("PATCH /api/mcp/capabilities/{id}/toggle", mcpCapabilityToggleHandler)
	mux.HandleFunc("POST /api/mcp/capabilities/{id}/test", mcpCapabilityTestHandler)
	mux.HandleFunc("POST /api/mcp/reconnect", mcpReconnectHandler)
	mux.HandleFunc("GET /ws", wsHandler)
	mux.HandleFunc("GET /ws/", wsHandler)
	mux.HandleFunc("POST /api/query", queryHandler)
	mux.HandleFunc("POST /api/query/stream", streamQueryHandler)
	mux.HandleFunc("GET /api/permissions/pending", pendingPermissionsHandler)
	mux.HandleFunc("POST /api/permissions/decision", permissionDecisionHandler)
	mux.HandleFunc("GET /api/permissions/mode", permissionModeHandler)
	mux.HandleFunc("PUT /api/permissions/mode", permissionModeHandler)
	mux.HandleFunc("GET /api/python/capabilities", pythonCapabilitiesHandler)
	mux.HandleFunc("POST /api/tokenizer/count", tokenizerCountHandler)
	mux.HandleFunc("POST /api/code-diagrams/generate", codeDiagramGenerateHandler)
	mux.HandleFunc("POST /api/code-path/endpoints", codePathEndpointsHandler)
	mux.HandleFunc("POST /api/code-path/trace", codePathTraceHandler)

	return mux
}
