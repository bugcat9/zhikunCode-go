package api

import "net/http"

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", healthHandler)
	mux.HandleFunc("GET /api/doctor", doctorHandler)
	mux.HandleFunc("GET /api/config", configHandler)
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
