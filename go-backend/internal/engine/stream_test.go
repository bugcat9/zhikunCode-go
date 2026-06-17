package engine

import (
	"context"
	"encoding/json"
	"testing"

	"go-backend/internal/llm"
	"go-backend/internal/session"
	"go-backend/internal/tools"
)

func TestQueryEngineStreamRunsToolLoop(t *testing.T) {
	llmClient := &fakeLLM{
		streamResponses: [][]llm.LLMEvent{
			{
				{Type: llm.LLMEventDelta, Text: "checking", Model: "fake-model"},
				{
					Type:  llm.LLMEventToolCall,
					Model: "fake-model",
					ToolCall: &llm.ToolCall{
						ID:        "call_1",
						Name:      "fake_lookup",
						Arguments: json.RawMessage(`{"query":"repo"}`),
					},
				},
				{Type: llm.LLMEventDone, Model: "fake-model", Usage: llm.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12}},
			},
			{
				{Type: llm.LLMEventDelta, Text: "final ", Model: "fake-model"},
				{Type: llm.LLMEventDelta, Text: "answer", Model: "fake-model"},
				{Type: llm.LLMEventDone, Model: "fake-model", Usage: llm.Usage{PromptTokens: 20, CompletionTokens: 5, TotalTokens: 25}},
			},
		},
	}
	tool := &fakeTool{}
	sessions := session.NewMemoryService()
	engine := NewQueryEngine(llmClient, sessions, tools.NewRegistry(tool), Config{MaxToolRounds: 3})

	stream, err := engine.Stream(context.Background(), QueryRequest{Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	var events []Event
	for event := range stream {
		events = append(events, event)
	}

	wantTypes := []EventType{
		EventMessageStart,
		EventStreamDelta,
		EventToolUseStart,
		EventToolResult,
		EventStreamDelta,
		EventStreamDelta,
		EventMessageComplete,
	}
	if len(events) != len(wantTypes) {
		t.Fatalf("event count = %d, want %d: %#v", len(events), len(wantTypes), events)
	}
	for i, want := range wantTypes {
		if events[i].Type != want {
			t.Fatalf("event[%d].Type = %q, want %q; events=%#v", i, events[i].Type, want, events)
		}
	}

	if events[1].Delta != "checking" {
		t.Fatalf("first delta = %q", events[1].Delta)
	}
	if events[2].ToolName != "fake_lookup" || events[2].ToolUseID != "call_1" {
		t.Fatalf("tool start event = %#v", events[2])
	}
	if string(tool.input) != `{"query":"repo"}` {
		t.Fatalf("tool input = %s", tool.input)
	}
	if events[4].Delta+events[5].Delta != "final answer" {
		t.Fatalf("final deltas = %q + %q", events[4].Delta, events[5].Delta)
	}

	complete, ok := events[6].Payload.(map[string]any)
	if !ok {
		t.Fatalf("complete payload = %#v", events[6].Payload)
	}
	if complete["text"] != "final answer" || complete["model"] != "fake-model" {
		t.Fatalf("complete payload = %#v", complete)
	}

	persisted, err := sessions.ListMessages(context.Background(), events[0].SessionID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 2 {
		t.Fatalf("expected user and final assistant messages only, got %#v", persisted)
	}
	if persisted[1].Content != "final answer" {
		t.Fatalf("persisted assistant = %#v", persisted[1])
	}
}
