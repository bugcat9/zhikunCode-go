package permission

import (
	"context"
	"sync"
	"time"
)

const DefaultRequestTimeout = 2 * time.Minute

type MemoryBroker struct {
	Policy  PermissionPolicy
	Timeout time.Duration

	mu        sync.Mutex
	pending   map[string]pendingRequest
	early     map[string]PermissionDecision
	requested []PermissionRequest
}

type pendingRequest struct {
	request  PermissionRequest
	response chan PermissionDecision
}

func NewMemoryBroker(policy PermissionPolicy, timeout time.Duration) *MemoryBroker {
	if timeout <= 0 {
		timeout = DefaultRequestTimeout
	}
	return &MemoryBroker{
		Policy:  policy,
		Timeout: timeout,
		pending: make(map[string]pendingRequest),
		early:   make(map[string]PermissionDecision),
	}
}

func (b *MemoryBroker) Hint(req PermissionRequest) DecisionHint {
	return hintForPolicy(b.policy(), req)
}

func (b *MemoryBroker) Request(ctx context.Context, req PermissionRequest) (PermissionDecision, error) {
	if b == nil {
		return NewPolicyBroker(nil).Request(ctx, req)
	}

	hint := b.Hint(req)
	req = enrichRequest(req, hint)
	switch hint.Action {
	case HintAllow:
		return decisionFromHint(req, hint, true), nil
	case HintDeny:
		return decisionFromHint(req, hint, true), nil
	}

	key := requestKey(req)
	if key == "" {
		key = req.ToolUseID
	}

	response := make(chan PermissionDecision, 1)
	b.mu.Lock()
	if decision, ok := b.early[key]; ok {
		delete(b.early, key)
		b.mu.Unlock()
		return normalizeDecisionForRequest(req, decision)
	}
	b.pending[key] = pendingRequest{request: req, response: response}
	b.requested = append(b.requested, req)
	b.mu.Unlock()

	timeout := time.NewTimer(b.Timeout)
	defer timeout.Stop()
	defer b.removePending(key)

	select {
	case <-ctx.Done():
		return PermissionDecision{}, ctx.Err()
	case decision := <-response:
		return normalizeDecisionForRequest(req, decision)
	case <-timeout.C:
		return PermissionDecision{
			RequestID: req.ID,
			ToolUseID: req.ToolUseID,
			Decision:  DecisionDeny,
			Reason:    ErrRequestTimeout.Error(),
			DecidedAt: time.Now().UTC(),
		}, nil
	}
}

func (b *MemoryBroker) Respond(ctx context.Context, decision PermissionDecision) error {
	if b == nil {
		return ErrUnknownRequest
	}
	if _, err := NormalizeDecision(decision.Decision); err != nil {
		return err
	}
	key := decisionKey(decision)
	if key == "" {
		return ErrUnknownRequest
	}
	if decision.DecidedAt.IsZero() {
		decision.DecidedAt = time.Now().UTC()
	}

	b.mu.Lock()
	pending, ok := b.pending[key]
	if !ok {
		b.early[key] = decision
		b.mu.Unlock()
		return nil
	}
	delete(b.pending, key)
	b.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case pending.response <- decision:
		return nil
	default:
		return nil
	}
}

func (b *MemoryBroker) Pending() []PermissionRequest {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	requests := make([]PermissionRequest, 0, len(b.pending))
	for _, pending := range b.pending {
		requests = append(requests, pending.request)
	}
	return requests
}

func (b *MemoryBroker) Requested() []PermissionRequest {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	requests := make([]PermissionRequest, len(b.requested))
	copy(requests, b.requested)
	return requests
}

func (b *MemoryBroker) policy() PermissionPolicy {
	if b.Policy != nil {
		return b.Policy
	}
	return NewPolicy(ModeAsk)
}

func (b *MemoryBroker) removePending(key string) {
	b.mu.Lock()
	delete(b.pending, key)
	b.mu.Unlock()
}

func enrichRequest(req PermissionRequest, hint DecisionHint) PermissionRequest {
	if req.RiskLevel == "" {
		req.RiskLevel = hint.RiskLevel
	}
	if req.Reason == "" {
		req.Reason = hint.Reason
	}
	if req.RequestedAt.IsZero() {
		req.RequestedAt = time.Now().UTC()
	}
	return req
}

func normalizeDecisionForRequest(req PermissionRequest, decision PermissionDecision) (PermissionDecision, error) {
	normalized, err := NormalizeDecision(decision.Decision)
	if err != nil {
		return PermissionDecision{}, err
	}
	decision.Decision = normalized
	if decision.RequestID == "" {
		decision.RequestID = req.ID
	}
	if decision.ToolUseID == "" {
		decision.ToolUseID = req.ToolUseID
	}
	if decision.DecidedAt.IsZero() {
		decision.DecidedAt = time.Now().UTC()
	}
	return decision, nil
}

func requestKey(req PermissionRequest) string {
	if req.ID != "" {
		return req.ID
	}
	return req.ToolUseID
}

func decisionKey(decision PermissionDecision) string {
	if decision.RequestID != "" {
		return decision.RequestID
	}
	return decision.ToolUseID
}
