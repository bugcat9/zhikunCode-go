package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"go-backend/internal/llm"
	"go-backend/internal/session"
)

func TestModelsHandlerReturnsCompatibleCatalog(t *testing.T) {
	t.Setenv("LLM_DEFAULT_MODEL", "test-model")
	t.Setenv("LLM_MODELS", "test-model,other-model")

	router := NewRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		DefaultModel string `json:"defaultModel"`
		Models       []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.DefaultModel != "test-model" {
		t.Fatalf("defaultModel = %q", body.DefaultModel)
	}
	if len(body.Models) == 0 || body.Models[0].ID == "" || body.Models[0].DisplayName == "" {
		t.Fatalf("unexpected models: %#v", body.Models)
	}
}

func TestSessionCompatibilityEndpoints(t *testing.T) {
	workspacePath := t.TempDir()
	resetQuerySessionsForTest(t)
	t.Setenv("WORKSPACE_PATH", workspacePath)
	t.Setenv("LLM_DEFAULT_MODEL", "test-model")

	router := NewRouter()
	createReq := httptest.NewRequest(http.MethodPost, "/api/sessions", strings.NewReader(`{"dir":".","model":"test-model"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, createReq)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(createRecorder.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.SessionID == "" {
		t.Fatal("expected sessionId")
	}

	sessions, err := getQuerySessions()
	if err != nil {
		t.Fatal(err)
	}
	if err := sessions.AppendMessage(context.Background(), created.SessionID, session.Message{
		Role:    llm.RoleUser,
		Content: "hello",
	}); err != nil {
		t.Fatal(err)
	}

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/api/sessions?limit=10", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listBody struct {
		Sessions []struct {
			ID           string `json:"id"`
			SessionID    string `json:"sessionId"`
			MessageCount int    `json:"messageCount"`
		} `json:"sessions"`
		HasMore bool `json:"hasMore"`
	}
	if err := json.NewDecoder(listRecorder.Body).Decode(&listBody); err != nil {
		t.Fatal(err)
	}
	if len(listBody.Sessions) != 1 || listBody.Sessions[0].ID != created.SessionID || listBody.Sessions[0].MessageCount != 1 {
		t.Fatalf("unexpected session list: %#v", listBody)
	}

	messagesRecorder := httptest.NewRecorder()
	router.ServeHTTP(messagesRecorder, httptest.NewRequest(http.MethodGet, "/api/sessions/"+created.SessionID+"/messages", nil))
	if messagesRecorder.Code != http.StatusOK {
		t.Fatalf("messages status = %d, body = %s", messagesRecorder.Code, messagesRecorder.Body.String())
	}
	var messagesBody struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.NewDecoder(messagesRecorder.Body).Decode(&messagesBody); err != nil {
		t.Fatal(err)
	}
	if len(messagesBody.Messages) != 1 || messagesBody.Messages[0]["type"] != "user" {
		t.Fatalf("unexpected messages: %#v", messagesBody.Messages)
	}

	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, httptest.NewRequest(http.MethodDelete, "/api/sessions/"+created.SessionID, nil))
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
}

func TestMCPDisabledCompatibilityEndpoints(t *testing.T) {
	router := NewRouter()

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/mcp/resources"},
		{method: http.MethodGet, path: "/api/mcp/resources/read?uri=file://demo&server=local"},
		{method: http.MethodGet, path: "/api/mcp/prompts"},
		{method: http.MethodPost, path: "/api/mcp/prompts/execute", body: `{"promptName":"demo","server":"local","arguments":{}}`},
		{method: http.MethodGet, path: "/api/mcp/capabilities"},
		{method: http.MethodGet, path: "/api/mcp/capabilities/domains"},
		{method: http.MethodPost, path: "/api/mcp/reconnect?server=local"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d, body = %s", tc.method, tc.path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestSockJSStompPing(t *testing.T) {
	router := NewRouter()
	server := httptest.NewServer(router)
	defer server.Close()

	infoRecorder := httptest.NewRecorder()
	router.ServeHTTP(infoRecorder, httptest.NewRequest(http.MethodGet, "/ws/info", nil))
	if infoRecorder.Code != http.StatusOK {
		t.Fatalf("sockjs info status = %d", infoRecorder.Code)
	}

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/000/test/websocket"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	_, openFrame, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(openFrame) != "o" {
		t.Fatalf("expected SockJS open frame, got %q", string(openFrame))
	}

	writeSockJSFrame(t, conn, buildSTOMPFrame("CONNECT", map[string]string{
		"accept-version": "1.2",
		"X-Session-Id":   "default",
	}, ""))
	connected := readSockJSSTOMPFrame(t, conn)
	if connected.Command != "CONNECTED" {
		t.Fatalf("expected CONNECTED, got %#v", connected)
	}

	writeSockJSFrame(t, conn, buildSTOMPFrame("SUBSCRIBE", map[string]string{
		"id":          "sub-test",
		"destination": "/user/queue/messages",
	}, ""))
	writeSockJSFrame(t, conn, buildSTOMPFrame("SEND", map[string]string{
		"destination": "/app/ping",
	}, `{}`))

	message := readSockJSSTOMPFrame(t, conn)
	if message.Command != "MESSAGE" {
		t.Fatalf("expected MESSAGE, got %#v", message)
	}
	var payload struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(message.Body), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Type != "pong" {
		t.Fatalf("expected pong payload, got %q body=%s", payload.Type, message.Body)
	}
}

func writeSockJSFrame(t *testing.T, conn *websocket.Conn, frame string) {
	t.Helper()
	data, err := json.Marshal([]string{frame})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatal(err)
	}
}

func readSockJSSTOMPFrame(t *testing.T, conn *websocket.Conn) stompFrame {
	t.Helper()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		raw := string(data)
		if raw == "h" {
			continue
		}
		if !strings.HasPrefix(raw, "a") {
			t.Fatalf("expected SockJS message frame, got %q", raw)
		}
		var messages []string
		if err := json.Unmarshal([]byte(raw[1:]), &messages); err != nil {
			t.Fatal(err)
		}
		if len(messages) == 0 {
			continue
		}
		frames := parseSTOMPFrames(messages[0])
		if len(frames) == 0 {
			continue
		}
		return frames[0]
	}
}

func resetQuerySessionsForTest(t *testing.T) {
	t.Helper()

	querySessionsMu.Lock()
	previous := querySessions
	querySessions = nil
	querySessionsMu.Unlock()

	t.Cleanup(func() {
		querySessionsMu.Lock()
		current := querySessions
		querySessions = previous
		querySessionsMu.Unlock()

		if current != previous {
			if closer, ok := current.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
		}
	})
}
