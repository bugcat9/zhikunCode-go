package engine

type EventType string

const (
	EventMessageStart      EventType = "message_start"
	EventStreamDelta       EventType = "stream_delta"
	EventToolUseStart      EventType = "tool_use_start"
	EventToolUseProgress   EventType = "tool_use_progress"
	EventToolResult        EventType = "tool_result"
	EventPermissionRequest EventType = "permission_request"
	EventMessageComplete   EventType = "message_complete"
	EventError             EventType = "error"
)

type Event struct {
	Type      EventType `json:"type"`
	SessionID string    `json:"sessionId,omitempty"`
	Delta     string    `json:"delta,omitempty"`
	ToolName  string    `json:"toolName,omitempty"`
	ToolUseID string    `json:"toolUseId,omitempty"`
	Payload   any       `json:"payload,omitempty"`
}
