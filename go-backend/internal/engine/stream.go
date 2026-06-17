package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"go-backend/internal/llm"
	"go-backend/internal/session"
)

func (e *QueryEngine) Stream(ctx context.Context, req QueryRequest) (<-chan Event, error) {
	if e == nil {
		return nil, errors.New("query engine is nil")
	}
	if e.LLM == nil {
		return nil, errors.New("query engine LLM client is nil")
	}
	if e.Sessions == nil {
		return nil, errors.New("query engine session service is nil")
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}

	sess, err := e.Sessions.GetOrCreate(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}

	history, err := e.Sessions.ListMessages(ctx, sess.ID, e.Config.MaxHistoryMessages)
	if err != nil {
		return nil, err
	}

	systemPrompt := strings.TrimSpace(req.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = e.Config.DefaultSystemPrompt
	}

	messages := buildMessages(systemPrompt, history, prompt)
	if err := e.Sessions.AppendMessage(ctx, sess.ID, session.Message{
		Role:    llm.RoleUser,
		Content: prompt,
	}); err != nil {
		return nil, err
	}

	events := make(chan Event)
	go func() {
		defer close(events)

		if !sendEvent(ctx, events, Event{
			Type:      EventMessageStart,
			SessionID: sess.ID,
			Payload: map[string]any{
				"model": req.Model,
			},
		}) {
			return
		}

		if _, err := e.runStreamLoop(ctx, events, sess.ID, req.Model, messages); err != nil {
			_ = sendEvent(ctx, events, Event{
				Type:      EventError,
				SessionID: sess.ID,
				Payload: map[string]any{
					"message": err.Error(),
				},
			})
		}
	}()

	return events, nil
}

func (e *QueryEngine) runStreamLoop(ctx context.Context, events chan<- Event, sessionID string, model string, messages []llm.ChatMessage) (QueryResult, error) {
	var totalUsage llm.Usage
	var lastModel string
	toolDefinitions := e.toolDefinitions()

	for round := 0; ; round++ {
		llmEvents, err := e.LLM.Stream(ctx, llm.ChatRequest{
			Model:    model,
			Messages: messages,
			Tools:    toolDefinitions,
		})
		if err != nil {
			return QueryResult{}, err
		}

		assistantText := strings.Builder{}
		toolCalls := []llm.ToolCall{}
		toolMessages := []llm.ChatMessage{}

		for event := range llmEvents {
			switch event.Type {
			case llm.LLMEventDelta:
				if event.Model != "" {
					lastModel = event.Model
				}
				assistantText.WriteString(event.Text)
				if event.Text != "" && !sendEvent(ctx, events, Event{
					Type:      EventStreamDelta,
					SessionID: sessionID,
					Delta:     event.Text,
				}) {
					return QueryResult{}, ctx.Err()
				}

			case llm.LLMEventToolCall:
				if event.Model != "" {
					lastModel = event.Model
				}
				if event.ToolCall == nil {
					continue
				}
				call := *event.ToolCall
				if len(call.Arguments) == 0 {
					call.Arguments = json.RawMessage(`{}`)
				}
				toolCalls = append(toolCalls, call)

				if !sendEvent(ctx, events, Event{
					Type:      EventToolUseStart,
					SessionID: sessionID,
					ToolName:  call.Name,
					ToolUseID: call.ID,
					Payload: map[string]any{
						"arguments": call.Arguments,
					},
				}) {
					return QueryResult{}, ctx.Err()
				}

				toolMessage := e.runToolCall(ctx, call)
				toolMessages = append(toolMessages, toolMessage)
				if !sendEvent(ctx, events, Event{
					Type:      EventToolResult,
					SessionID: sessionID,
					ToolName:  call.Name,
					ToolUseID: call.ID,
					Payload:   decodeToolResultPayload(toolMessage.Content),
				}) {
					return QueryResult{}, ctx.Err()
				}

			case llm.LLMEventDone:
				addUsage(&totalUsage, event.Usage)
				if event.Model != "" {
					lastModel = event.Model
				}

			case llm.LLMEventError:
				if event.Error == "" {
					return QueryResult{}, errors.New("LLM stream failed")
				}
				return QueryResult{}, errors.New(event.Error)
			}
		}

		if len(toolCalls) == 0 {
			text := assistantText.String()
			if err := e.Sessions.AppendMessage(ctx, sessionID, session.Message{
				Role:    llm.RoleAssistant,
				Content: text,
			}); err != nil {
				return QueryResult{}, err
			}

			result := QueryResult{
				SessionID: sessionID,
				Text:      text,
				Model:     lastModel,
				Usage:     totalUsage,
			}
			if !sendEvent(ctx, events, Event{
				Type:      EventMessageComplete,
				SessionID: sessionID,
				Payload: map[string]any{
					"text":  result.Text,
					"model": result.Model,
					"usage": result.Usage,
				},
			}) {
				return QueryResult{}, ctx.Err()
			}
			return result, nil
		}

		if round >= e.Config.MaxToolRounds {
			return QueryResult{}, ErrMaxToolRoundsExceeded
		}

		messages = append(messages, llm.ChatMessage{
			Role:      llm.RoleAssistant,
			Content:   assistantText.String(),
			ToolCalls: toolCalls,
		})
		messages = append(messages, toolMessages...)
	}
}

func decodeToolResultPayload(content string) any {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	var payload any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return map[string]any{"content": content}
	}
	return payload
}

func sendEvent(ctx context.Context, events chan<- Event, event Event) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- event:
		return true
	}
}
