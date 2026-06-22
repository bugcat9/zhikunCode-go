package api

import (
	"encoding/json"
	"net/http"
	"sync"
)

var (
	configMu        sync.Mutex
	configOverrides = map[string]any{}
)

func configHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, currentConfigResponse())
	case http.MethodPut:
		updates := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body: "+err.Error())
			return
		}
		configMu.Lock()
		for key, value := range updates {
			configOverrides[key] = value
		}
		configMu.Unlock()
		writeJSON(w, http.StatusOK, currentConfigResponse())
	default:
		writeNotImplemented(w, "GET|PUT /api/config")
	}
}

func currentConfigResponse() map[string]any {
	cfg := map[string]any{
		"service":          "zhikuncode-go-backend",
		"httpPort":         8081,
		"pythonServiceUrl": "http://localhost:8000",
		"locale":           "zh-CN",
		"autoCompact": map[string]any{
			"enabled":   true,
			"threshold": 80,
		},
		"verbose":      false,
		"expandedView": false,
		"defaultModel": effectiveDefaultModel(),
		"outputStyle": map[string]any{
			"availableStyles": []any{},
			"activeStyleName": nil,
		},
	}

	configMu.Lock()
	defer configMu.Unlock()
	for key, value := range configOverrides {
		cfg[key] = value
	}
	return cfg
}
