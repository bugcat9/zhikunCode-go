package llm

import (
	"context"
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
			Role:    RoleAssistant,
			Content: message.Content,
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
			result = append(result, openai.AssistantMessage(message.Content))
		case RoleUser:
			result = append(result, openai.UserMessage(message.Content))
		default:
			result = append(result, openai.UserMessage(message.Content))
		}
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
