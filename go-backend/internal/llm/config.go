package llm

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	EnvBaseURL      = "LLM_BASE_URL"
	EnvAPIKey       = "LLM_API_KEY"
	EnvDefaultModel = "LLM_DEFAULT_MODEL"
)

type Config struct {
	BaseURL      string
	APIKey       string
	DefaultModel string
	Timeout      time.Duration
}

func LoadConfig() (Config, error) {
	if err := LoadDotEnv(); err != nil {
		return Config{}, err
	}

	cfg := LoadConfigFromEnv()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func LoadConfigFromEnv() Config {
	cfg := Config{
		BaseURL:      strings.TrimSpace(os.Getenv(EnvBaseURL)),
		APIKey:       strings.TrimSpace(os.Getenv(EnvAPIKey)),
		DefaultModel: strings.TrimSpace(os.Getenv(EnvDefaultModel)),
		Timeout:      60 * time.Second,
	}
	return cfg.WithDefaults()
}

func (c Config) WithDefaults() Config {
	if c.Timeout == 0 {
		c.Timeout = 60 * time.Second
	}
	return c
}

func (c Config) Validate() error {
	if c.BaseURL == "" {
		return &Error{
			Kind:    ErrorKindConfig,
			Message: EnvBaseURL + " is required",
		}
	}

	if _, err := url.ParseRequestURI(c.BaseURL); err != nil {
		return &Error{
			Kind:    ErrorKindConfig,
			Message: EnvBaseURL + " must be a valid URL",
			Cause:   err,
		}
	}

	if c.APIKey == "" {
		return &Error{
			Kind:    ErrorKindConfig,
			Message: EnvAPIKey + " is required",
		}
	}

	if c.DefaultModel == "" {
		return &Error{
			Kind:    ErrorKindConfig,
			Message: EnvDefaultModel + " is required",
		}
	}

	return nil
}

func LoadDotEnv() error {
	path, ok, err := findDotEnv()
	if err != nil {
		return &Error{
			Kind:    ErrorKindConfig,
			Message: "failed to find .env",
			Cause:   err,
		}
	}
	if !ok {
		return nil
	}

	if err := godotenv.Load(path); err != nil {
		return &Error{
			Kind:    ErrorKindConfig,
			Message: "failed to load .env",
			Cause:   err,
		}
	}
	return nil
}

func findDotEnv() (string, bool, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false, err
	}

	for {
		path := filepath.Join(dir, ".env")
		info, err := os.Stat(path)
		if err == nil {
			if info.IsDir() {
				return "", false, nil
			}
			return path, true, nil
		}
		if !os.IsNotExist(err) {
			return "", false, err
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false, nil
		}
		dir = parent
	}
}
