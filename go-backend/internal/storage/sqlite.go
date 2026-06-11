package storage

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const DefaultDataDir = ".ai-code-assistant"

type SQLiteConfig struct {
	Path string
}

func DefaultSQLitePath(workspacePath string) string {
	return filepath.Join(workspacePath, DefaultDataDir, "data.db")
}

func OpenSQLite(cfg SQLiteConfig) (*sql.DB, error) {
	if cfg.Path == "" {
		workspacePath, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		cfg.Path = DefaultSQLitePath(workspacePath)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(SchemaV1); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}
