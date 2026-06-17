package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"go-backend/internal/engine"
)

func TestWriteSSEEvent(t *testing.T) {
	recorder := httptest.NewRecorder()

	err := writeSSEEvent(recorder, engine.Event{
		Type:      engine.EventStreamDelta,
		SessionID: "session_1",
		Delta:     "hello",
	})
	if err != nil {
		t.Fatal(err)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "event: stream_delta\n") {
		t.Fatalf("missing event line: %q", body)
	}
	if !strings.Contains(body, `"type":"stream_delta"`) ||
		!strings.Contains(body, `"sessionId":"session_1"`) ||
		!strings.Contains(body, `"delta":"hello"`) {
		t.Fatalf("missing JSON data fields: %q", body)
	}
	if !strings.HasSuffix(body, "\n\n") {
		t.Fatalf("SSE event should end with a blank line: %q", body)
	}
}
