package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// TODO: Implement GET /api/doctor.
func doctorHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"checks": map[string]any{
			"goBackend": map[string]string{
				"status": "ok",
			},
			"pythonService": checkPythonService(r.Context()),
			"sqlite":        checkSQLite(),
			"workspace":     checkWorkspace(),
		},
	},
	)
}

func checkPythonService(ctx context.Context) map[string]any {
	start := time.Now()

	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8000/api/health", nil)
	if err != nil {
		return map[string]any{
			"status":  "warn",
			"message": err.Error(),
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{
			"status":    "warn",
			"message":   "python service not reachable",
			"error":     err.Error(),
			"latencyMs": time.Since(start).Milliseconds(),
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return map[string]any{
			"status":    "warn",
			"message":   fmt.Sprintf("python service returned HTTP %d", resp.StatusCode),
			"latencyMs": time.Since(start).Milliseconds(),
		}
	}

	return map[string]any{
		"status":    "ok",
		"message":   "python service available",
		"url":       "http://localhost:8000",
		"latencyMs": time.Since(start).Milliseconds(),
	}
}

func loadWorkspacePath() (string, error) {
	if v := os.Getenv("WORKSPACE_PATH"); v != "" {
		return filepath.Abs(v)
	}
	return os.Getwd()
}

func checkWorkspace() map[string]any {
	path, err := loadWorkspacePath()
	if err != nil {
		return map[string]any{
			"status":  "error",
			"message": "failed to resolve workspace path",
			"error":   err.Error(),
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return map[string]any{
			"status":  "error",
			"path":    path,
			"message": "workspace not accessible",
			"error":   err.Error(),
		}
	}

	if !info.IsDir() {
		return map[string]any{
			"status":  "error",
			"path":    path,
			"message": "workspace path is not a directory",
		}
	}

	if _, err := os.ReadDir(path); err != nil {
		return map[string]any{
			"status":  "error",
			"path":    path,
			"message": "workspace is not readable",
			"error":   err.Error(),
		}
	}

	return map[string]any{
		"status":  "ok",
		"path":    path,
		"message": "workspace available",
	}
}

func checkSQLite() map[string]any {
	workspacePath, err := loadWorkspacePath()
	if err != nil {
		return map[string]any{
			"status":  "warn",
			"message": "cannot resolve workspace, sqlite path not checked",
			"error":   err.Error(),
		}
	}

	dbDir := filepath.Join(workspacePath, ".ai-code-assistant")
	dbPath := filepath.Join(dbDir, "data.db")

	info, err := os.Stat(dbDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{
				"status":  "warn",
				"path":    dbPath,
				"message": "sqlite database directory not initialized yet",
			}
		}

		return map[string]any{
			"status":  "warn",
			"path":    dbPath,
			"message": "sqlite database directory not accessible",
			"error":   err.Error(),
		}
	}

	if !info.IsDir() {
		return map[string]any{
			"status":  "error",
			"path":    dbDir,
			"message": "sqlite database directory path is not a directory",
		}
	}

	if dbInfo, err := os.Stat(dbPath); err == nil && !dbInfo.IsDir() {
		return map[string]any{
			"status":  "ok",
			"path":    dbPath,
			"message": "sqlite database file exists",
		}
	}

	return map[string]any{
		"status":  "warn",
		"path":    dbPath,
		"message": "sqlite database file not created yet",
	}
}
