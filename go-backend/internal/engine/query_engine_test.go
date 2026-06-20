package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go-backend/internal/llm"
	"go-backend/internal/permission"
	"go-backend/internal/session"
	"go-backend/internal/tools"
)

type fakeLLM struct {
	requests        []llm.ChatRequest
	responses       []llm.ChatResponse
	streamResponses [][]llm.LLMEvent
}

func (f *fakeLLM) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	f.requests = append(f.requests, req)
	if len(f.responses) == 0 {
		return llm.ChatResponse{}, errors.New("missing fake response")
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func (f *fakeLLM) Stream(ctx context.Context, req llm.ChatRequest) (<-chan llm.LLMEvent, error) {
	f.requests = append(f.requests, req)
	if len(f.streamResponses) == 0 {
		return nil, errors.New("missing fake stream response")
	}
	events := f.streamResponses[0]
	f.streamResponses = f.streamResponses[1:]

	ch := make(chan llm.LLMEvent, len(events))
	for _, event := range events {
		ch <- event
	}
	close(ch)
	return ch, nil
}

type fakeTool struct {
	input json.RawMessage
}

func (t *fakeTool) Name() string {
	return "fake_lookup"
}

func (t *fakeTool) Description() string {
	return "Fake lookup tool for QueryEngine tests."
}

func (t *fakeTool) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
		},
	}
}

func (t *fakeTool) Run(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	t.input = append(t.input[:0], input...)
	return tools.ToolResult{
		Content: "tool says hi",
		Data: map[string]any{
			"seen_input": string(input),
		},
	}, nil
}

type fakePermissionBroker struct {
	hint     permission.DecisionHint
	decision permission.PermissionDecision
	requests []permission.PermissionRequest
}

func (b *fakePermissionBroker) Hint(req permission.PermissionRequest) permission.DecisionHint {
	if b.hint.Action != "" {
		return b.hint
	}
	return permission.DecisionHint{Action: permission.HintAllow, RiskLevel: permission.RiskLow}
}

func (b *fakePermissionBroker) Request(ctx context.Context, req permission.PermissionRequest) (permission.PermissionDecision, error) {
	b.requests = append(b.requests, req)
	if b.decision.Decision != "" {
		return b.decision, nil
	}
	return permission.PermissionDecision{
		RequestID: req.ID,
		ToolUseID: req.ToolUseID,
		Decision:  permission.DecisionAllow,
	}, nil
}

func TestQueryEngineRunsToolLoop(t *testing.T) {
	llmClient := &fakeLLM{
		responses: []llm.ChatResponse{
			{
				Model: "fake-model",
				Message: llm.ChatMessage{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{
							ID:        "call_1",
							Name:      "fake_lookup",
							Arguments: json.RawMessage(`{"query":"repo"}`),
						},
					},
				},
				Usage: llm.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12},
			},
			{
				Model:   "fake-model",
				Message: llm.ChatMessage{Role: llm.RoleAssistant, Content: "final answer"},
				Usage:   llm.Usage{PromptTokens: 20, CompletionTokens: 5, TotalTokens: 25},
			},
		},
	}
	tool := &fakeTool{}
	sessions := session.NewMemoryService()
	engine := NewQueryEngine(llmClient, sessions, tools.NewRegistry(tool), Config{MaxToolRounds: 3})

	result, err := engine.Query(context.Background(), QueryRequest{Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	if result.Text != "final answer" {
		t.Fatalf("expected final answer, got %q", result.Text)
	}
	if result.Usage.TotalTokens != 37 {
		t.Fatalf("expected accumulated usage, got %#v", result.Usage)
	}
	if string(tool.input) != `{"query":"repo"}` {
		t.Fatalf("tool did not receive expected input: %s", tool.input)
	}
	if len(llmClient.requests) != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", len(llmClient.requests))
	}
	if len(llmClient.requests[0].Tools) != 1 || llmClient.requests[0].Tools[0].Name != "fake_lookup" {
		t.Fatalf("tool definitions were not sent to LLM: %#v", llmClient.requests[0].Tools)
	}

	secondMessages := llmClient.requests[1].Messages
	if len(secondMessages) < 2 {
		t.Fatalf("expected tool loop messages, got %#v", secondMessages)
	}
	assistantToolCall := secondMessages[len(secondMessages)-2]
	toolResult := secondMessages[len(secondMessages)-1]
	if len(assistantToolCall.ToolCalls) != 1 || assistantToolCall.ToolCalls[0].ID != "call_1" {
		t.Fatalf("assistant tool call was not replayed: %#v", assistantToolCall)
	}
	if toolResult.Role != llm.RoleTool || toolResult.ToolCallID != "call_1" {
		t.Fatalf("tool result message is malformed: %#v", toolResult)
	}
	if !strings.Contains(toolResult.Content, "tool says hi") {
		t.Fatalf("tool result content was not returned to LLM: %s", toolResult.Content)
	}

	persisted, err := sessions.ListMessages(context.Background(), result.SessionID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 2 {
		t.Fatalf("expected user and final assistant messages only, got %#v", persisted)
	}
}

func TestQueryEngineStopsAfterMaxToolRounds(t *testing.T) {
	llmClient := &fakeLLM{
		responses: []llm.ChatResponse{
			{
				Message: llm.ChatMessage{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "fake_lookup", Arguments: json.RawMessage(`{}`)},
					},
				},
			},
			{
				Message: llm.ChatMessage{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_2", Name: "fake_lookup", Arguments: json.RawMessage(`{}`)},
					},
				},
			},
		},
	}
	engine := NewQueryEngine(llmClient, session.NewMemoryService(), tools.NewRegistry(&fakeTool{}), Config{MaxToolRounds: 1})

	_, err := engine.Query(context.Background(), QueryRequest{Prompt: "loop forever"})
	if !errors.Is(err, ErrMaxToolRoundsExceeded) {
		t.Fatalf("expected ErrMaxToolRoundsExceeded, got %v", err)
	}
}

func TestQueryEngineDoesNotRunToolWhenPermissionDenied(t *testing.T) {
	llmClient := &fakeLLM{
		responses: []llm.ChatResponse{
			{
				Message: llm.ChatMessage{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "fake_lookup", Arguments: json.RawMessage(`{"query":"repo"}`)},
					},
				},
			},
			{
				Message: llm.ChatMessage{Role: llm.RoleAssistant, Content: "permission noted"},
			},
		},
	}
	tool := &fakeTool{}
	broker := &fakePermissionBroker{
		hint: permission.DecisionHint{
			Action:    permission.HintAsk,
			RiskLevel: permission.RiskMedium,
			Reason:    "test confirmation required",
		},
		decision: permission.PermissionDecision{
			RequestID: "call_1",
			ToolUseID: "call_1",
			Decision:  permission.DecisionDeny,
			Reason:    "denied in test",
		},
	}
	engine := NewQueryEngine(llmClient, session.NewMemoryService(), tools.NewRegistry(tool), Config{MaxToolRounds: 3}).
		SetPermissionBroker(broker)

	result, err := engine.Query(context.Background(), QueryRequest{Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "permission noted" {
		t.Fatalf("expected final answer after denied tool result, got %q", result.Text)
	}
	if len(tool.input) != 0 {
		t.Fatalf("tool should not have been executed, got input %s", tool.input)
	}
	if len(broker.requests) != 1 || broker.requests[0].ToolName != "fake_lookup" {
		t.Fatalf("permission broker did not receive tool request: %#v", broker.requests)
	}

	secondMessages := llmClient.requests[1].Messages
	toolResult := secondMessages[len(secondMessages)-1]
	if !strings.Contains(toolResult.Content, "permission denied: denied in test") {
		t.Fatalf("permission denial was not returned to LLM: %s", toolResult.Content)
	}
}
