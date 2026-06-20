package permission

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type Mode string

const (
	ModeReadOnly  Mode = "read_only"
	ModeReadWrite Mode = "read_write"
	ModeAsk       Mode = "ask"
	ModeAuto      Mode = "auto"
	ModeDenyAll   Mode = "deny_all"
)

type HintAction string

const (
	HintAllow HintAction = "allow"
	HintAsk   HintAction = "ask"
	HintDeny  HintAction = "deny"
)

type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

var (
	ErrPermissionDenied = errors.New("permission denied")
	ErrRequestTimeout   = errors.New("permission request timed out")
	ErrUnknownRequest   = errors.New("permission request not found")
	ErrInvalidDecision  = errors.New("invalid permission decision")
)

type PermissionRequest struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"sessionId,omitempty"`
	ToolUseID   string          `json:"toolUseId"`
	ToolName    string          `json:"toolName"`
	Input       json.RawMessage `json:"input,omitempty"`
	RiskLevel   RiskLevel       `json:"riskLevel"`
	Reason      string          `json:"reason,omitempty"`
	RequestedAt time.Time       `json:"requestedAt,omitempty"`
}

type PermissionDecision struct {
	RequestID string    `json:"requestId,omitempty"`
	ToolUseID string    `json:"toolUseId,omitempty"`
	Decision  Decision  `json:"decision"`
	Reason    string    `json:"reason,omitempty"`
	Remember  bool      `json:"remember,omitempty"`
	Scope     string    `json:"scope,omitempty"`
	DecidedAt time.Time `json:"decidedAt,omitempty"`
}

type DecisionHint struct {
	Action    HintAction `json:"action"`
	RiskLevel RiskLevel  `json:"riskLevel"`
	Reason    string     `json:"reason,omitempty"`
}

type PermissionBroker interface {
	Request(ctx context.Context, req PermissionRequest) (PermissionDecision, error)
}

type PermissionPolicy interface {
	Decide(req PermissionRequest) DecisionHint
}

type HintedBroker interface {
	Hint(req PermissionRequest) DecisionHint
}

func NormalizeMode(mode Mode) Mode {
	switch mode {
	case ModeReadOnly, ModeReadWrite, ModeAsk, ModeAuto, ModeDenyAll:
		return mode
	default:
		return ModeAsk
	}
}

func NormalizeDecision(decision Decision) (Decision, error) {
	switch decision {
	case DecisionAllow, DecisionDeny:
		return decision, nil
	default:
		return "", ErrInvalidDecision
	}
}
