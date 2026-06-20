package api

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"

	"go-backend/internal/permission"
)

const envPermissionMode = "PERMISSION_MODE"

var (
	permissionBrokerMu sync.Mutex
	permissionBroker   *permission.MemoryBroker
	permissionMode     permission.Mode
)

func pendingPermissionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeNotImplemented(w, "GET /api/permissions/pending")
		return
	}

	broker, err := getPermissionBroker()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "permission_error", "Failed to initialize permissions: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"requests": broker.Pending(),
	})
}

func permissionDecisionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeNotImplemented(w, "POST /api/permissions/decision")
		return
	}

	decision := permission.PermissionDecision{}
	if err := json.NewDecoder(r.Body).Decode(&decision); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body: "+err.Error())
		return
	}

	broker, err := getPermissionBroker()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "permission_error", "Failed to initialize permissions: "+err.Error())
		return
	}
	if err := broker.Respond(r.Context(), decision); err != nil {
		status := http.StatusBadRequest
		if err == permission.ErrUnknownRequest {
			status = http.StatusNotFound
		}
		writeError(w, status, "permission_decision_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
	})
}

type permissionModeRequest struct {
	Mode permission.Mode `json:"mode"`
}

func permissionModeHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		broker, err := getPermissionBroker()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "permission_error", "Failed to initialize permissions: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"mode": currentPermissionMode(broker),
		})

	case http.MethodPut:
		req := permissionModeRequest{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body: "+err.Error())
			return
		}
		mode := permission.NormalizeMode(req.Mode)
		if mode != req.Mode {
			writeError(w, http.StatusBadRequest, "invalid_permission_mode", "unsupported permission mode")
			return
		}
		if err := setPermissionMode(mode); err != nil {
			writeError(w, http.StatusInternalServerError, "permission_error", "Failed to set permission mode: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"mode": mode,
		})

	default:
		writeNotImplemented(w, "GET|PUT /api/permissions/mode")
	}
}

func getPermissionBroker() (*permission.MemoryBroker, error) {
	permissionBrokerMu.Lock()
	defer permissionBrokerMu.Unlock()

	if permissionBroker != nil {
		return permissionBroker, nil
	}

	workspacePath, err := loadWorkspacePath()
	if err != nil {
		return nil, err
	}

	mode := loadPermissionMode()
	policy, err := permission.NewDefaultPolicy(mode, workspacePath)
	if err != nil {
		return nil, err
	}

	permissionMode = mode
	permissionBroker = permission.NewMemoryBroker(policy, 0)
	return permissionBroker, nil
}

func setPermissionMode(mode permission.Mode) error {
	permissionBrokerMu.Lock()
	defer permissionBrokerMu.Unlock()

	workspacePath, err := loadWorkspacePath()
	if err != nil {
		return err
	}

	policy, err := permission.NewDefaultPolicy(mode, workspacePath)
	if err != nil {
		return err
	}

	if permissionBroker == nil {
		permissionBroker = permission.NewMemoryBroker(policy, 0)
	} else {
		permissionBroker.Policy = policy
	}
	permissionMode = mode
	return nil
}

func currentPermissionMode(broker *permission.MemoryBroker) permission.Mode {
	if permissionMode != "" {
		return permissionMode
	}
	if broker != nil {
		if policy, ok := broker.Policy.(*permission.DefaultPolicy); ok {
			return policy.Mode
		}
	}
	return loadPermissionMode()
}

func loadPermissionMode() permission.Mode {
	if value := os.Getenv(envPermissionMode); value != "" {
		return permission.NormalizeMode(permission.Mode(value))
	}
	return permission.ModeAsk
}
