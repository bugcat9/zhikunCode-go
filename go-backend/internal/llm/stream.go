package llm

import (
	"context"
	"encoding/json"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
)

func (c *OpenAICompatibleClient) stream(ctx context.Context, req ChatRequest) (<-chan LLMEvent, error) {
	if len(req.Messages) == 0 {
		return nil, &Error{
			Kind:    ErrorKindConfig,
			Message: "messages are required",
		}
	}

	model := req.Model
	if model == "" {
		model = c.Config.DefaultModel
	}
	if model == "" {
		return nil, &Error{
			Kind:    ErrorKindConfig,
			Message: EnvDefaultModel + " is required",
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: toOpenAIMessages(req.Messages),
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: param.NewOpt(true),
		},
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

	stream := c.client.Chat.Completions.NewStreaming(ctx, params)
	events := make(chan LLMEvent)

	go func() {
		defer close(events)

		acc := openai.ChatCompletionAccumulator{}
		var usage Usage
		lastModel := model

		for stream.Next() {
			chunk := stream.Current()
			if chunk.Model != "" {
				lastModel = chunk.Model
			}
			if chunk.Usage.TotalTokens > 0 || chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
				usage = Usage{
					PromptTokens:     int(chunk.Usage.PromptTokens),
					CompletionTokens: int(chunk.Usage.CompletionTokens),
					TotalTokens:      int(chunk.Usage.TotalTokens),
				}
			}

			if !acc.AddChunk(chunk) {
				sendLLMEvent(ctx, events, LLMEvent{
					Type:  LLMEventError,
					Error: "failed to accumulate streaming chat completion chunk",
				})
				return
			}

			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					if !sendLLMEvent(ctx, events, LLMEvent{
						Type:  LLMEventDelta,
						Text:  choice.Delta.Content,
						Model: lastModel,
					}) {
						return
					}
				}
			}

			if toolCall, ok := acc.JustFinishedToolCall(); ok {
				arguments := toolCall.Arguments
				if arguments == "" {
					arguments = "{}"
				}
				if !sendLLMEvent(ctx, events, LLMEvent{
					Type:  LLMEventToolCall,
					Model: lastModel,
					ToolCall: &ToolCall{
						ID:        toolCall.ID,
						Name:      toolCall.Name,
						Arguments: json.RawMessage(arguments),
					},
				}) {
					return
				}
			}
		}

		if err := stream.Err(); err != nil {
			_ = sendLLMEvent(ctx, events, LLMEvent{
				Type:  LLMEventError,
				Error: mapOpenAIError(err).Error(),
			})
			return
		}

		_ = sendLLMEvent(ctx, events, LLMEvent{
			Type:  LLMEventDone,
			Model: lastModel,
			Usage: usage,
		})
	}()

	return events, nil
}

func sendLLMEvent(ctx context.Context, events chan<- LLMEvent, event LLMEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- event:
		return true
	}
}
