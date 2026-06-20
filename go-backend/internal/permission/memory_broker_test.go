package permission

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestMemoryBrokerReturnsImmediateAllows(t *testing.T) {
	broker := NewMemoryBroker(NewPolicy(ModeAsk), time.Second)

	decision, err := broker.Request(context.Background(), PermissionRequest{
		ID:        "call_1",
		ToolUseID: "call_1",
		ToolName:  "read_file",
		Input:     json.RawMessage(`{"path":"README.md"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Decision != DecisionAllow {
		t.Fatalf("expected allow decision, got %#v", decision)
	}
}

func TestMemoryBrokerWaitsForUserDecision(t *testing.T) {
	broker := NewMemoryBroker(NewPolicy(ModeAsk), time.Second)
	req := PermissionRequest{
		ID:        "call_1",
		ToolUseID: "call_1",
		ToolName:  "write_file",
		Input:     json.RawMessage(`{"path":"notes.txt","content":"hello"}`),
	}

	decisions := make(chan PermissionDecision, 1)
	errors := make(chan error, 1)
	go func() {
		decision, err := broker.Request(context.Background(), req)
		if err != nil {
			errors <- err
			return
		}
		decisions <- decision
	}()

	waitForPending(t, broker)
	if err := broker.Respond(context.Background(), PermissionDecision{
		RequestID: "call_1",
		Decision:  DecisionAllow,
		Reason:    "approved in test",
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-errors:
		t.Fatal(err)
	case decision := <-decisions:
		if decision.Decision != DecisionAllow {
			t.Fatalf("expected allow decision, got %#v", decision)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for permission decision")
	}
}

func waitForPending(t *testing.T, broker *MemoryBroker) {
	t.Helper()

	deadline := time.After(time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if len(broker.Pending()) > 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatal("permission request was not marked pending")
		case <-ticker.C:
		}
	}
}
