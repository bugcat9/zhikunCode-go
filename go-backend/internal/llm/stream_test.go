package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAICompatibleClientStreamParsesDeltasAndUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"created\":0,\"model\":\"fake-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hel\"},\"finish_reason\":\"\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"created\":0,\"model\":\"fake-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"created\":0,\"model\":\"fake-model\",\"choices\":[],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":1,\"total_tokens\":3}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(Config{
		BaseURL:      server.URL,
		APIKey:       "test-key",
		DefaultModel: "fake-model",
		Timeout:      5 * time.Second,
	})

	stream, err := client.Stream(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var events []LLMEvent
	for event := range stream {
		events = append(events, event)
	}

	if len(events) != 3 {
		t.Fatalf("event count = %d, want 3: %#v", len(events), events)
	}
	if events[0].Type != LLMEventDelta || events[0].Text != "hel" {
		t.Fatalf("first event = %#v", events[0])
	}
	if events[1].Type != LLMEventDelta || events[1].Text != "lo" {
		t.Fatalf("second event = %#v", events[1])
	}
	if events[2].Type != LLMEventDone || events[2].Usage.TotalTokens != 3 {
		t.Fatalf("done event = %#v", events[2])
	}
}
