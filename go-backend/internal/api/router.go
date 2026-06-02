package api

import "net/http"

// TODO: Build the HTTP router and register stage-one endpoints.
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", healthHandler)
	mux.HandleFunc("GET /api/doctor", doctorHandler)
	mux.HandleFunc("GET /api/config", configHandler)

	return mux
}
