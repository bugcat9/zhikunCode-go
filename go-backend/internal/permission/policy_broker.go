package permission

import (
	"context"
	"time"
)

type PolicyBroker struct {
	Policy PermissionPolicy
}

func NewPolicyBroker(policy PermissionPolicy) *PolicyBroker {
	return &PolicyBroker{Policy: policy}
}

func (b *PolicyBroker) Hint(req PermissionRequest) DecisionHint {
	return hintForPolicy(b.Policy, req)
}

func (b *PolicyBroker) Request(ctx context.Context, req PermissionRequest) (PermissionDecision, error) {
	select {
	case <-ctx.Done():
		return PermissionDecision{}, ctx.Err()
	default:
	}

	hint := b.Hint(req)
	return decisionFromHint(req, hint, true), nil
}

func hintForPolicy(policy PermissionPolicy, req PermissionRequest) DecisionHint {
	if policy == nil {
		policy = NewPolicy(ModeAsk)
	}
	hint := policy.Decide(req)
	if hint.Action == "" {
		hint.Action = HintAsk
	}
	if hint.RiskLevel == "" {
		hint.RiskLevel = RiskMedium
	}
	return hint
}

func decisionFromHint(req PermissionRequest, hint DecisionHint, denyAsk bool) PermissionDecision {
	decision := DecisionDeny
	reason := hint.Reason
	if hint.Action == HintAllow {
		decision = DecisionAllow
	} else if hint.Action == HintAsk && !denyAsk {
		reason = "permission requested"
	}

	return PermissionDecision{
		RequestID: req.ID,
		ToolUseID: req.ToolUseID,
		Decision:  decision,
		Reason:    reason,
		DecidedAt: time.Now().UTC(),
	}
}
