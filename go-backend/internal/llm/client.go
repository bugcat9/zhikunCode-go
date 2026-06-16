package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type LLMClient interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	Stream(ctx context.Context, req ChatRequest) (<-chan LLMEvent, error)
}

type OpenAICompatibleClient struct {
	Config Config
	client openai.Client
}

func NewOpenAICompatibleClient(cfg Config) *OpenAICompatibleClient {
	cfg = cfg.WithDefaults()

	return &OpenAICompatibleClient{
		Config: cfg,
		client: openai.NewClient(
			option.WithAPIKey(cfg.APIKey),
			option.WithBaseURL(cfg.BaseURL),
			option.WithRequestTimeout(cfg.Timeout),
		),
	}
}

func (c *OpenAICompatibleClient) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	if len(req.Messages) == 0 {
		return ChatResponse{}, &Error{
			Kind:    ErrorKindConfig,
			Message: "messages are required",
		}
	}

	model := req.Model
	if model == "" {
		model = c.Config.DefaultModel
	}
	if model == "" {
		return ChatResponse{}, &Error{
			Kind:    ErrorKindConfig,
			Message: EnvDefaultModel + " is required",
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: toOpenAIMessages(req.Messages),
	}
	if len(req.Tools) > 0 {
		params.Tools = toOpenAITools(req.Tools)
	}
	if req.Temperature != nil {
		params.Temperature = openai.Float(*req.Temperature)
	}
	if req.MaxTokens != nil {
		params.MaxTokens = openai.Int(int64(*req.MaxTokens))
	}

	resp, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return ChatResponse{}, mapOpenAIError(err)
	}
	if len(resp.Choices) == 0 {
		return ChatResponse{}, &Error{
			Kind:    ErrorKindProvider,
			Message: "LLM provider returned no choices",
		}
	}

	message := resp.Choices[0].Message
	return ChatResponse{
		Model: resp.Model,
		Message: ChatMessage{
			Role:      RoleAssistant,
			Content:   message.Content,
			ToolCalls: fromOpenAIToolCalls(message.ToolCalls),
		},
		Usage: Usage{
			PromptTokens:     int(resp.Usage.PromptTokens),
			CompletionTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
		},
	}, nil
}

func (c *OpenAICompatibleClient) Stream(ctx context.Context, req ChatRequest) (<-chan LLMEvent, error) {
	return nil, &Error{
		Kind:    ErrorKindUnexpected,
		Message: "llm stream is not implemented yet",
	}
}

func toOpenAIMessages(messages []ChatMessage) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case RoleSystem:
			result = append(result, openai.SystemMessage(message.Content))
		case RoleAssistant:
			result = append(result, toOpenAIAssistantMessage(message))
		case RoleTool:
			result = append(result, openai.ToolMessage(message.Content, message.ToolCallID))
		case RoleUser:
			result = append(result, openai.UserMessage(message.Content))
		default:
			result = append(result, openai.UserMessage(message.Content))
		}
	}
	return result
}

func toOpenAIAssistantMessage(message ChatMessage) openai.ChatCompletionMessageParamUnion {
	openaiMessage := openai.AssistantMessage(message.Content)
	if len(message.ToolCalls) == 0 {
		return openaiMessage
	}

	// Assistant tool-call messages must be replayed verbatim before tool results.
	// Without this, the next LLM request cannot connect a tool result to its request.
	for _, call := range message.ToolCalls {
		arguments := string(call.Arguments)
		if arguments == "" {
			arguments = "{}"
		}
		openaiMessage.OfAssistant.ToolCalls = append(openaiMessage.OfAssistant.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
			OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
				ID: call.ID,
				Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
					Name:      call.Name,
					Arguments: arguments,
				},
			},
		})
	}
	return openaiMessage
}

func toOpenAITools(definitions []ToolDefinition) []openai.ChatCompletionToolUnionParam {
	tools := make([]openai.ChatCompletionToolUnionParam, 0, len(definitions))
	for _, definition := range definitions {
		if definition.Name == "" {
			continue
		}
		function := openai.FunctionDefinitionParam{
			Name:        definition.Name,
			Description: openai.String(definition.Description),
			Parameters:  toOpenAIParameters(definition.Schema),
		}
		tools = append(tools, openai.ChatCompletionFunctionTool(function))
	}
	return tools
}

func toOpenAIParameters(schema any) openai.FunctionParameters {
	if schema == nil {
		return nil
	}
	if parameters, ok := schema.(map[string]any); ok {
		return openai.FunctionParameters(parameters)
	}

	// Tool schemas are usually map[string]any already. This fallback lets callers
	// pass a typed schema struct without making the LLM package depend on that type.
	data, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	var parameters map[string]any
	if err := json.Unmarshal(data, &parameters); err != nil {
		return nil
	}
	return openai.FunctionParameters(parameters)
}

func fromOpenAIToolCalls(calls []openai.ChatCompletionMessageToolCallUnion) []ToolCall {
	if len(calls) == 0 {
		return nil
	}

	result := make([]ToolCall, 0, len(calls))
	for _, call := range calls {
		// 第五阶段只接 function tool。custom tool 可以等后面需要时再加。
		if call.Type != "" && call.Type != "function" {
			continue
		}
		if call.Function.Name == "" {
			continue
		}
		arguments := call.Function.Arguments
		if arguments == "" {
			arguments = "{}"
		}
		result = append(result, ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: json.RawMessage(arguments),
		})
	}
	return result
}

func mapOpenAIError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &Error{
			Kind:    ErrorKindTimeout,
			Message: "LLM request timed out",
			Cause:   err,
		}
	}

	var apiErr *openai.Error
	if !errors.As(err, &apiErr) {
		return &Error{
			Kind:    ErrorKindUnexpected,
			Message: "LLM request failed",
			Cause:   err,
		}
	}

	kind := ErrorKindProvider
	switch apiErr.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		kind = ErrorKindUnauthorized
	case http.StatusTooManyRequests:
		kind = ErrorKindRateLimited
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		kind = ErrorKindTimeout
	}

	message := apiErr.Message
	if message == "" {
		message = "LLM provider returned an error"
	}

	return &Error{
		Kind:       kind,
		StatusCode: apiErr.StatusCode,
		Message:    message,
		Cause:      err,
	}
}
