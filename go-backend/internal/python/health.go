package python

import (
	"context"
	"time"
)

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

func CheckServiceHealth(ctx context.Context) map[string]any {
	start := time.Now()
	client := NewClient("")

	health, err := client.Health(ctx)
	if err != nil {
		return map[string]any{
			"status":    "warn",
			"message":   "python service not reachable",
			"error":     err.Error(),
			"latencyMs": time.Since(start).Milliseconds(),
		}
	}

	return map[string]any{
		"status":    "ok",
		"message":   "python service available",
		"url":       client.BaseURL,
		"service":   health.Service,
		"version":   health.Version,
		"latencyMs": time.Since(start).Milliseconds(),
	}
}
