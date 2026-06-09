package llm

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type ChatMessage struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model,omitempty"`
	Messages    []ChatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
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

type LLMEventType string

const (
	LLMEventDelta LLMEventType = "delta"
	LLMEventDone  LLMEventType = "done"
	LLMEventError LLMEventType = "error"
)

type LLMEvent struct {
	Type  LLMEventType `json:"type"`
	Text  string       `json:"text,omitempty"`
	Usage Usage        `json:"usage,omitempty"`
	Error string       `json:"error,omitempty"`
}
