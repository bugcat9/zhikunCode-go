package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-backend/internal/llm"
	"go-backend/internal/permission"
	"go-backend/internal/tools"
)

func (e *QueryEngine) authorizeToolCall(ctx context.Context, sessionID string, call llm.ToolCall) (bool, permission.PermissionDecision, error) {
	if e.Permissions == nil {
		return true, permission.PermissionDecision{
			ToolUseID: call.ID,
			Decision:  permission.DecisionAllow,
			DecidedAt: time.Now().UTC(),
		}, nil
	}

	req := e.buildPermissionRequest(sessionID, call)
	decision, err := e.Permissions.Request(ctx, req)
	if err != nil {
		return false, decision, err
	}
	if decision.Decision != permission.DecisionAllow {
		return false, decision, nil
	}
	return true, decision, nil
}

func (e *QueryEngine) permissionRequestEvent(sessionID string, call llm.ToolCall) (permission.PermissionRequest, bool) {
	if e.Permissions == nil {
		return permission.PermissionRequest{}, false
	}

	req := e.buildPermissionRequest(sessionID, call)
	hinted, ok := e.Permissions.(permission.HintedBroker)
	if !ok {
		return req, false
	}

	hint := hinted.Hint(req)
	req.RiskLevel = hint.RiskLevel
	req.Reason = hint.Reason
	return req, hint.Action == permission.HintAsk
}

func (e *QueryEngine) buildPermissionRequest(sessionID string, call llm.ToolCall) permission.PermissionRequest {
	input := call.Arguments
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}

	req := permission.PermissionRequest{
		ID:          permissionRequestID(call),
		SessionID:   sessionID,
		ToolUseID:   call.ID,
		ToolName:    call.Name,
		Input:       input,
		RiskLevel:   permission.RiskMedium,
		RequestedAt: time.Now().UTC(),
	}

	if hinted, ok := e.Permissions.(permission.HintedBroker); ok {
		hint := hinted.Hint(req)
		if hint.RiskLevel != "" {
			req.RiskLevel = hint.RiskLevel
		}
		req.Reason = hint.Reason
	}
	return req
}

func permissionRequestID(call llm.ToolCall) string {
	if call.ID != "" {
		return call.ID
	}
	return call.Name
}

func permissionDeniedResult(decision permission.PermissionDecision) tools.ToolResult {
	reason := decision.Reason
	if reason == "" {
		reason = permission.ErrPermissionDenied.Error()
	}
	message := fmt.Sprintf("permission denied: %s", reason)
	return tools.ToolResult{
		Error:   message,
		Content: message,
		Data: map[string]any{
			"permission": "denied",
			"requestId":  decision.RequestID,
			"toolUseId":  decision.ToolUseID,
			"reason":     reason,
		},
	}
}
