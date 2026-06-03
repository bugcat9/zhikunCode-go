package api

import "net/http"

func pythonCapabilitiesHandler(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, "GET /api/python/capabilities")
}
