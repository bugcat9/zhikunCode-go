package api

import (
	"net/http"
	"os"
	"strings"

	"go-backend/internal/llm"
)

const fallbackDefaultModel = "qwen3.7-max"

type modelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Provider    string `json:"provider,omitempty"`
}

type providerInfo struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	Configured  bool     `json:"configured"`
	BaseURL     string   `json:"baseUrl,omitempty"`
	Models      []string `json:"models"`
}

type modelCatalog struct {
	Models       []modelInfo    `json:"models"`
	DefaultModel string         `json:"defaultModel"`
	Providers    []providerInfo `json:"providers"`
	Metadata     map[string]any `json:"metadata"`
}

func modelsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeNotImplemented(w, "GET /api/models")
		return
	}
	writeJSON(w, http.StatusOK, loadModelCatalog())
}

func loadModelCatalog() modelCatalog {
	configError := ""
	if err := llm.LoadDotEnv(); err != nil {
		configError = err.Error()
	}

	defaultModel := strings.TrimSpace(os.Getenv(llm.EnvDefaultModel))
	if defaultModel == "" {
		defaultModel = fallbackDefaultModel
	}

	providers := loadProviderInfo()
	models := modelsFromProviders(providers)
	if len(models) == 0 {
		for _, id := range fallbackModelIDs(defaultModel) {
			models = append(models, modelInfo{
				ID:          id,
				DisplayName: displayModelName(id),
				Provider:    inferProvider(id),
			})
		}
	}

	knownDefault := false
	for _, model := range models {
		if model.ID == defaultModel {
			knownDefault = true
			break
		}
	}
	if !knownDefault && defaultModel != "" {
		models = append([]modelInfo{{
			ID:          defaultModel,
			DisplayName: displayModelName(defaultModel),
			Provider:    inferProvider(defaultModel),
		}}, models...)
	}

	return modelCatalog{
		Models:       models,
		DefaultModel: defaultModel,
		Providers:    providers,
		Metadata: map[string]any{
			"configured":   anyProviderConfigured(providers),
			"configError":  configError,
			"compatLayer":  "go-phase-8",
			"fallbackUsed": len(providers) == 0,
		},
	}
}

func loadProviderInfo() []providerInfo {
	providers := []providerInfo{
		{
			ID:          "dashscope",
			DisplayName: "DashScope",
			Configured:  configuredAPIKey(os.Getenv("LLM_PROVIDER_DASHSCOPE_API_KEY")),
			BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
			Models:      splitCSV(os.Getenv("LLM_PROVIDER_DASHSCOPE_MODELS")),
		},
		{
			ID:          "deepseek",
			DisplayName: "DeepSeek",
			Configured:  configuredAPIKey(os.Getenv("LLM_PROVIDER_DEEPSEEK_API_KEY")),
			BaseURL:     "https://api.deepseek.com",
			Models:      splitCSV(os.Getenv("LLM_PROVIDER_DEEPSEEK_MODELS")),
		},
		{
			ID:          "moonshot",
			DisplayName: "Moonshot",
			Configured:  configuredAPIKey(os.Getenv("LLM_PROVIDER_MOONSHOT_API_KEY")),
			BaseURL:     "https://api.moonshot.cn/v1",
			Models:      splitCSV(os.Getenv("LLM_PROVIDER_MOONSHOT_MODELS")),
		},
	}

	legacyModels := splitCSV(os.Getenv("LLM_MODELS"))
	if len(legacyModels) > 0 || os.Getenv(llm.EnvBaseURL) != "" || os.Getenv(llm.EnvAPIKey) != "" {
		providers = append(providers, providerInfo{
			ID:          "legacy",
			DisplayName: "OpenAI Compatible",
			Configured:  configuredAPIKey(os.Getenv(llm.EnvAPIKey)) && strings.TrimSpace(os.Getenv(llm.EnvBaseURL)) != "",
			BaseURL:     strings.TrimSpace(os.Getenv(llm.EnvBaseURL)),
			Models:      legacyModels,
		})
	}

	result := providers[:0]
	for _, provider := range providers {
		if len(provider.Models) == 0 && !provider.Configured && provider.BaseURL == "" {
			continue
		}
		result = append(result, provider)
	}
	return result
}

func modelsFromProviders(providers []providerInfo) []modelInfo {
	seen := map[string]bool{}
	var models []modelInfo
	for _, provider := range providers {
		for _, id := range provider.Models {
			if seen[id] {
				continue
			}
			seen[id] = true
			models = append(models, modelInfo{
				ID:          id,
				DisplayName: displayModelName(id),
				Provider:    provider.ID,
			})
		}
	}
	return models
}

func fallbackModelIDs(defaultModel string) []string {
	ids := []string{defaultModel, "qwen3.7-max", "deepseek-v4-pro", "kimi-k2.6"}
	seen := map[string]bool{}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func configuredAPIKey(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	return !strings.Contains(lower, "your-") && !strings.Contains(lower, "placeholder")
}

func displayModelName(id string) string {
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '-' || r == '_'
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	if len(parts) == 0 {
		return id
	}
	return strings.Join(parts, " ")
}

func inferProvider(modelID string) string {
	lower := strings.ToLower(modelID)
	switch {
	case strings.Contains(lower, "qwen"):
		return "dashscope"
	case strings.Contains(lower, "deepseek"):
		return "deepseek"
	case strings.Contains(lower, "kimi"), strings.Contains(lower, "moonshot"):
		return "moonshot"
	default:
		return "legacy"
	}
}

func anyProviderConfigured(providers []providerInfo) bool {
	for _, provider := range providers {
		if provider.Configured {
			return true
		}
	}
	return false
}
