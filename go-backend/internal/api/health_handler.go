package api

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

var startedAt = time.Now().UTC()

// TODO: Implement GET /api/health.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	subsystems := map[string]map[string]any{
		"goRuntime": checkGoRuntime(),
		// 第一阶段 sqlite 可以先不放，或者只做轻量检查
	}

	allHealthy := true
	for _, item := range subsystems {
		if item["status"] != "UP" {
			allHealthy = false
			break
		}
	}

	status := "UP"
	code := http.StatusOK
	if !allHealthy {
		status = "DEGRADED"
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":        status,
		"service":       "zhikuncode-go-backend",
		"version":       "0.1.0",
		"startedAt":     startedAt.Format(time.RFC3339),
		"uptimeSeconds": int64(time.Since(startedAt).Seconds()),
		"goVersion":     runtime.Version(),
		"subsystems":    subsystems,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	})
}

func checkGoRuntime() map[string]any {
	return map[string]any{
		"status":  "UP",
		"message": "Go runtime available",
		"goos":    runtime.GOOS,
		"goarch":  runtime.GOARCH,
	}
}
