package permission

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go-backend/internal/workspace"
)

type DefaultPolicy struct {
	Mode        Mode
	Boundary    workspace.Boundary
	HasBoundary bool
}

func NewDefaultPolicy(mode Mode, workspacePath string) (*DefaultPolicy, error) {
	boundary, err := workspace.NewBoundary(workspacePath)
	if err != nil {
		return nil, err
	}
	return &DefaultPolicy{
		Mode:        NormalizeMode(mode),
		Boundary:    boundary,
		HasBoundary: true,
	}, nil
}

func NewPolicy(mode Mode) *DefaultPolicy {
	return &DefaultPolicy{Mode: NormalizeMode(mode)}
}

func (p *DefaultPolicy) Decide(req PermissionRequest) DecisionHint {
	if p == nil {
		return defaultToolHint(req.ToolName)
	}

	if hint, ok := p.boundaryHint(req); ok {
		return hint
	}

	switch NormalizeMode(p.Mode) {
	case ModeDenyAll:
		return DecisionHint{
			Action:    HintDeny,
			RiskLevel: RiskHigh,
			Reason:    "permission mode deny_all rejects every tool call",
		}
	case ModeReadOnly:
		if isLowRiskTool(req.ToolName) {
			return allow(RiskLow, "non-mutating tool is allowed in read_only mode")
		}
		return DecisionHint{
			Action:    HintDeny,
			RiskLevel: RiskHigh,
			Reason:    "permission mode read_only rejects tools that can modify state",
		}
	case ModeReadWrite:
		if isLowRiskTool(req.ToolName) || isWriteTool(req.ToolName) {
			return allow(toolRisk(req.ToolName), "read and write file tools are allowed in read_write mode")
		}
		return defaultToolHint(req.ToolName)
	case ModeAsk, ModeAuto:
		return defaultToolHint(req.ToolName)
	default:
		return defaultToolHint(req.ToolName)
	}
}

func (p *DefaultPolicy) boundaryHint(req PermissionRequest) (DecisionHint, bool) {
	if p == nil || !p.HasBoundary {
		return DecisionHint{}, false
	}

	for _, path := range extractPathCandidates(req.Input) {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if _, err := p.Boundary.Resolve(path); err != nil {
			if errors.Is(err, workspace.ErrOutsideWorkspace) {
				return DecisionHint{
					Action:    HintDeny,
					RiskLevel: RiskHigh,
					Reason:    fmt.Sprintf("path %q is outside the workspace", path),
				}, true
			}
			return DecisionHint{
				Action:    HintDeny,
				RiskLevel: RiskHigh,
				Reason:    fmt.Sprintf("path %q could not be verified: %v", path, err),
			}, true
		}
	}
	return DecisionHint{}, false
}

func defaultToolHint(toolName string) DecisionHint {
	switch {
	case isLowRiskTool(toolName):
		return allow(RiskLow, "non-mutating tool is allowed by default")
	case isWriteTool(toolName):
		return DecisionHint{
			Action:    HintAsk,
			RiskLevel: RiskHigh,
			Reason:    "write tools require user confirmation",
		}
	case isShellTool(toolName):
		return DecisionHint{
			Action:    HintAsk,
			RiskLevel: RiskHigh,
			Reason:    "shell tools require user confirmation",
		}
	default:
		return DecisionHint{
			Action:    HintAsk,
			RiskLevel: RiskMedium,
			Reason:    "unclassified tool requires user confirmation",
		}
	}
}

func allow(risk RiskLevel, reason string) DecisionHint {
	return DecisionHint{Action: HintAllow, RiskLevel: risk, Reason: reason}
}

func toolRisk(toolName string) RiskLevel {
	if isWriteTool(toolName) || isShellTool(toolName) {
		return RiskHigh
	}
	return RiskLow
}

func isLowRiskTool(toolName string) bool {
	return isReadOnlyTool(toolName) || isDelegationTool(toolName)
}

func isReadOnlyTool(toolName string) bool {
	switch normalizeToolName(toolName) {
	case "read_file", "list_files":
		return true
	default:
		return false
	}
}

func isDelegationTool(toolName string) bool {
	switch normalizeToolName(toolName) {
	case "task_create":
		return true
	default:
		return false
	}
}

func isWriteTool(toolName string) bool {
	switch normalizeToolName(toolName) {
	case "write_file", "edit_file", "apply_patch", "delete_file", "move_file":
		return true
	default:
		return false
	}
}

func isShellTool(toolName string) bool {
	name := normalizeToolName(toolName)
	return name == "shell" ||
		name == "bash" ||
		name == "terminal" ||
		name == "run_command" ||
		name == "exec_command" ||
		strings.Contains(name, "shell") ||
		strings.Contains(name, "bash")
}

func normalizeToolName(toolName string) string {
	return strings.ToLower(strings.TrimSpace(toolName))
}

func extractPathCandidates(input json.RawMessage) []string {
	if len(input) == 0 {
		return nil
	}

	var data map[string]any
	if err := json.Unmarshal(input, &data); err != nil {
		return nil
	}

	keys := []string{
		"path",
		"file_path",
		"filePath",
		"target_path",
		"targetPath",
		"directory",
		"dir",
	}

	paths := make([]string, 0, len(keys))
	for _, key := range keys {
		value, ok := data[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			paths = append(paths, typed)
		case []any:
			for _, item := range typed {
				if path, ok := item.(string); ok {
					paths = append(paths, path)
				}
			}
		}
	}
	return paths
}
