package permission

import (
	"encoding/json"
	"testing"
)

func TestDefaultPolicyAllowsReadToolsByDefault(t *testing.T) {
	policy := NewPolicy(ModeAsk)

	hint := policy.Decide(PermissionRequest{
		ToolName: "read_file",
		Input:    json.RawMessage(`{"path":"README.md"}`),
	})

	if hint.Action != HintAllow {
		t.Fatalf("expected read_file to be allowed, got %#v", hint)
	}
	if hint.RiskLevel != RiskLow {
		t.Fatalf("expected low risk, got %#v", hint)
	}
}

func TestDefaultPolicyAsksForWritesByDefault(t *testing.T) {
	policy := NewPolicy(ModeAsk)

	hint := policy.Decide(PermissionRequest{
		ToolName: "write_file",
		Input:    json.RawMessage(`{"path":"notes.txt","content":"hello"}`),
	})

	if hint.Action != HintAsk {
		t.Fatalf("expected write_file to ask, got %#v", hint)
	}
	if hint.RiskLevel != RiskHigh {
		t.Fatalf("expected high risk, got %#v", hint)
	}
}

func TestReadOnlyModeDeniesWriteTools(t *testing.T) {
	policy := NewPolicy(ModeReadOnly)

	hint := policy.Decide(PermissionRequest{
		ToolName: "write_file",
		Input:    json.RawMessage(`{"path":"notes.txt","content":"hello"}`),
	})

	if hint.Action != HintDeny {
		t.Fatalf("expected write_file to be denied in read_only mode, got %#v", hint)
	}
}

func TestDefaultPolicyRejectsPathsOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	policy, err := NewDefaultPolicy(ModeReadWrite, root)
	if err != nil {
		t.Fatal(err)
	}

	hint := policy.Decide(PermissionRequest{
		ToolName: "read_file",
		Input:    json.RawMessage(`{"path":"../outside.txt"}`),
	})

	if hint.Action != HintDeny {
		t.Fatalf("expected outside path to be denied, got %#v", hint)
	}
}
