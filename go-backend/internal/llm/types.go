package llm

import "encoding/json"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type ChatMessage struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ChatRequest struct {
	Model       string           `json:"model,omitempty"`
	Messages    []ChatMessage    `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
	MaxTokens   *int             `json:"max_tokens,omitempty"`
}

type ChatResponse struct {
	Model   string      `json:"model,omitempty"`
	Message ChatMessage `json:"message"`
	Usage   Usage       `json:"usage,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Schema      any    `json:"schema,omitempty"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type LLMEventType string

const (
	LLMEventDelta    LLMEventType = "delta"
	LLMEventToolCall LLMEventType = "tool_call"
	LLMEventDone     LLMEventType = "done"
	LLMEventError    LLMEventType = "error"
)

type LLMEvent struct {
	Type     LLMEventType `json:"type"`
	Text     string       `json:"text,omitempty"`
	Model    string       `json:"model,omitempty"`
	ToolCall *ToolCall    `json:"tool_call,omitempty"`
	Usage    Usage        `json:"usage,omitempty"`
	Error    string       `json:"error,omitempty"`
}
