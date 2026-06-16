package workspace

import (
	"os"
	"path/filepath"
)

const EnvPath = "WORKSPACE_PATH"

func DefaultPath() (string, error) {
	if value := os.Getenv(EnvPath); value != "" {
		return filepath.Abs(value)
	}
	return os.Getwd()
}
