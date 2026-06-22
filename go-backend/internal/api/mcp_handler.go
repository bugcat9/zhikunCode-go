package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type mcpCapability struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	ToolName         string         `json:"toolName"`
	SSEURL           string         `json:"sseUrl"`
	APIKeyConfig     string         `json:"apiKeyConfig"`
	APIKeyDefault    string         `json:"apiKeyDefault,omitempty"`
	Domain           string         `json:"domain"`
	Category         string         `json:"category"`
	BriefDescription string         `json:"briefDescription"`
	VideoCallSummary string         `json:"videoCallSummary,omitempty"`
	Description      string         `json:"description"`
	Input            map[string]any `json:"input"`
	Output           map[string]any `json:"output"`
	TimeoutMs        int            `json:"timeoutMs"`
	Enabled          bool           `json:"enabled"`
	VideoCallEnabled bool           `json:"videoCallEnabled"`
}

var (
	mcpCapabilitiesMu sync.Mutex
	mcpCapabilities   = map[string]mcpCapability{}
)

func mcpResourcesHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"resources": map[string][]any{},
		"servers":   []any{},
		"status":    "disabled",
		"message":   "MCP is not configured in the Go compatibility backend",
	})
}

func mcpResourceReadHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"uri":        r.URL.Query().Get("uri"),
		"serverName": r.URL.Query().Get("server"),
		"content":    "",
		"status":     "unavailable",
		"message":    "MCP resource reading is unavailable because MCP is disabled",
	})
}

func mcpPromptsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"prompts": map[string][]any{},
		"servers": []any{},
		"status":  "disabled",
	})
}

func mcpPromptExecuteHandler(w http.ResponseWriter, r *http.Request) {
	req := struct {
		PromptName string            `json:"promptName"`
		Server     string            `json:"server"`
		Arguments  map[string]string `json:"arguments"`
	}{}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":    false,
		"serverName": req.Server,
		"promptName": req.PromptName,
		"messages":   []any{},
		"error":      "MCP prompts are unavailable because MCP is disabled",
		"details":    []string{"Go phase-8 compatibility layer returns a stable disabled state."},
	})
}

func mcpCapabilitiesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listMcpCapabilitiesHandler(w, r)
	case http.MethodPost:
		createMcpCapabilityHandler(w, r)
	default:
		writeNotImplemented(w, "GET|POST /api/mcp/capabilities")
	}
}

func mcpCapabilityDomainsHandler(w http.ResponseWriter, r *http.Request) {
	mcpCapabilitiesMu.Lock()
	defer mcpCapabilitiesMu.Unlock()

	seen := map[string]bool{}
	for _, capability := range mcpCapabilities {
		if capability.Domain != "" {
			seen[capability.Domain] = true
		}
	}

	domains := make([]string, 0, len(seen))
	for domain := range seen {
		domains = append(domains, domain)
	}
	sort.Strings(domains)

	writeJSON(w, http.StatusOK, map[string]any{
		"domains": domains,
		"status":  "disabled",
	})
}

func mcpCapabilityHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	switch r.Method {
	case http.MethodGet:
		getMcpCapabilityHandler(w, r, id)
	case http.MethodPut:
		updateMcpCapabilityHandler(w, r, id)
	case http.MethodDelete:
		deleteMcpCapabilityHandler(w, r, id)
	default:
		writeNotImplemented(w, "GET|PUT|DELETE /api/mcp/capabilities/{id}")
	}
}

func mcpCapabilityToggleHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	enabled, _ := strconv.ParseBool(r.URL.Query().Get("enabled"))

	mcpCapabilitiesMu.Lock()
	capability, ok := mcpCapabilities[id]
	if ok {
		capability.Enabled = enabled
		mcpCapabilities[id] = capability
	}
	mcpCapabilitiesMu.Unlock()

	if !ok {
		writeError(w, http.StatusNotFound, "mcp_capability_not_found", "MCP capability not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"id":         id,
		"enabled":    enabled,
		"mcpStatus":  "disabled",
		"updatedCap": capability,
	})
}

func mcpCapabilityTestHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	mcpCapabilitiesMu.Lock()
	_, ok := mcpCapabilities[id]
	mcpCapabilitiesMu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "mcp_capability_not_found", "MCP capability not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "unavailable",
		"error":     "MCP is disabled in the Go compatibility backend",
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func mcpReconnectHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"success":     false,
		"server":      r.URL.Query().Get("server"),
		"status":      "disabled",
		"message":     "MCP reconnect is unavailable because MCP is disabled",
		"reconnected": false,
	})
}

func listMcpCapabilitiesHandler(w http.ResponseWriter, r *http.Request) {
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))

	mcpCapabilitiesMu.Lock()
	defer mcpCapabilitiesMu.Unlock()

	capabilities := make([]mcpCapability, 0, len(mcpCapabilities))
	for _, capability := range mcpCapabilities {
		if domain != "" && capability.Domain != domain {
			continue
		}
		capabilities = append(capabilities, capability)
	}
	sort.Slice(capabilities, func(i, j int) bool {
		return capabilities[i].ID < capabilities[j].ID
	})

	enabledCount := 0
	for _, capability := range capabilities {
		if capability.Enabled {
			enabledCount++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"capabilities": capabilities,
		"total":        len(capabilities),
		"enabledCount": enabledCount,
		"status":       "disabled",
	})
}

func createMcpCapabilityHandler(w http.ResponseWriter, r *http.Request) {
	capability, ok := decodeMcpCapability(w, r)
	if !ok {
		return
	}
	if capability.ID == "" {
		capability.ID = "cap_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	fillMcpCapabilityDefaults(&capability)

	mcpCapabilitiesMu.Lock()
	mcpCapabilities[capability.ID] = capability
	mcpCapabilitiesMu.Unlock()

	writeJSON(w, http.StatusCreated, capability)
}

func getMcpCapabilityHandler(w http.ResponseWriter, r *http.Request, id string) {
	mcpCapabilitiesMu.Lock()
	capability, ok := mcpCapabilities[id]
	mcpCapabilitiesMu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "mcp_capability_not_found", "MCP capability not found")
		return
	}
	writeJSON(w, http.StatusOK, capability)
}

func updateMcpCapabilityHandler(w http.ResponseWriter, r *http.Request, id string) {
	capability, ok := decodeMcpCapability(w, r)
	if !ok {
		return
	}
	capability.ID = id
	fillMcpCapabilityDefaults(&capability)

	mcpCapabilitiesMu.Lock()
	mcpCapabilities[id] = capability
	mcpCapabilitiesMu.Unlock()

	writeJSON(w, http.StatusOK, capability)
}

func deleteMcpCapabilityHandler(w http.ResponseWriter, r *http.Request, id string) {
	mcpCapabilitiesMu.Lock()
	_, ok := mcpCapabilities[id]
	if ok {
		delete(mcpCapabilities, id)
	}
	mcpCapabilitiesMu.Unlock()

	if !ok {
		writeError(w, http.StatusNotFound, "mcp_capability_not_found", "MCP capability not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func decodeMcpCapability(w http.ResponseWriter, r *http.Request) (mcpCapability, bool) {
	capability := mcpCapability{}
	if err := json.NewDecoder(r.Body).Decode(&capability); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body: "+err.Error())
		return mcpCapability{}, false
	}
	return capability, true
}

func fillMcpCapabilityDefaults(capability *mcpCapability) {
	if capability.Name == "" {
		capability.Name = capability.ID
	}
	if capability.ToolName == "" {
		capability.ToolName = capability.Name
	}
	if capability.Domain == "" {
		capability.Domain = "general"
	}
	if capability.Category == "" {
		capability.Category = "custom"
	}
	if capability.Input == nil {
		capability.Input = map[string]any{}
	}
	if capability.Output == nil {
		capability.Output = map[string]any{}
	}
	if capability.TimeoutMs <= 0 {
		capability.TimeoutMs = 30000
	}
}
