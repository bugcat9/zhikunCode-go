package api

import "net/http"

func codePathEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, "POST /api/code-path/endpoints")
}

func codePathTraceHandler(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, "POST /api/code-path/trace")
}
