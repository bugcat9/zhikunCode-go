package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"go-backend/internal/engine"
	"go-backend/internal/llm"
	"go-backend/internal/permission"
	"go-backend/internal/session"
)

var (
	wsUpgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	wsMessageSeq uint64
)

type stompFrame struct {
	Command string
	Headers map[string]string
	Body    string
}

type stompSession struct {
	conn   *websocket.Conn
	sockJS bool

	writeMu sync.Mutex
	stateMu sync.Mutex

	subscriptionID string
	sessionID      string
	model          string
	currentRunID   uint64
	currentCancel  context.CancelFunc
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/info"):
		writeSockJSInfo(w, r)
	case strings.HasSuffix(r.URL.Path, "/websocket"):
		serveStompWebSocket(w, r, true)
	case websocket.IsWebSocketUpgrade(r):
		serveStompWebSocket(w, r, false)
	default:
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"transport": "websocket",
			"sockjs":    true,
		})
	}
}

func writeSockJSInfo(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Content-Type", "application/json; charset=utf-8")
	header.Set("Cache-Control", "no-store, no-cache, no-transform, must-revalidate, max-age=0")
	writeJSON(w, http.StatusOK, map[string]any{
		"websocket":     true,
		"origins":       []string{"*:*"},
		"cookie_needed": false,
		"entropy":       time.Now().UnixNano(),
	})
}

func serveStompWebSocket(w http.ResponseWriter, r *http.Request, sockJS bool) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	session := &stompSession{
		conn:      conn,
		sockJS:    sockJS,
		sessionID: r.Header.Get("X-Session-Id"),
		model:     effectiveDefaultModel(),
	}
	defer session.close()

	if sockJS {
		if err := session.writeRaw("o"); err != nil {
			return
		}
		go session.sockJSHeartbeat()
	}

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if err := session.handleIncoming(string(data)); err != nil {
			_ = session.sendServerMessage(map[string]any{
				"type":      "error",
				"code":      "stomp_error",
				"message":   err.Error(),
				"retryable": true,
			})
		}
	}
}

func (s *stompSession) close() {
	s.cancelCurrent()
	_ = s.conn.Close()
}

func (s *stompSession) sockJSHeartbeat() {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if err := s.writeRaw("h"); err != nil {
			return
		}
	}
}

func (s *stompSession) handleIncoming(raw string) error {
	messages, err := decodeIncomingWebSocketMessages(raw, s.sockJS)
	if err != nil {
		return err
	}

	for _, message := range messages {
		if message == "\n" || strings.TrimSpace(message) == "" {
			continue
		}
		frames := parseSTOMPFrames(message)
		for _, frame := range frames {
			if err := s.handleFrame(frame); err != nil {
				return err
			}
		}
	}
	return nil
}

func decodeIncomingWebSocketMessages(raw string, sockJS bool) ([]string, error) {
	if !sockJS {
		return []string{raw}, nil
	}
	if raw == "" {
		return nil, nil
	}

	var messages []string
	if err := json.Unmarshal([]byte(raw), &messages); err != nil {
		return nil, errors.New("invalid SockJS websocket frame")
	}
	return messages, nil
}

func parseSTOMPFrames(message string) []stompFrame {
	parts := strings.Split(message, "\x00")
	frames := make([]stompFrame, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimLeft(part, "\n\r")
		if strings.TrimSpace(part) == "" {
			continue
		}
		frames = append(frames, parseSTOMPFrame(part))
	}
	return frames
}

func parseSTOMPFrame(message string) stompFrame {
	headerPart := message
	body := ""
	if idx := strings.Index(message, "\n\n"); idx >= 0 {
		headerPart = message[:idx]
		body = message[idx+2:]
	}

	lines := strings.Split(headerPart, "\n")
	frame := stompFrame{
		Command: strings.TrimSpace(lines[0]),
		Headers: map[string]string{},
		Body:    body,
	}
	for _, line := range lines[1:] {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		frame.Headers[line[:idx]] = line[idx+1:]
	}
	return frame
}

func (s *stompSession) handleFrame(frame stompFrame) error {
	switch frame.Command {
	case "CONNECT", "STOMP":
		s.applyConnectHeaders(frame.Headers)
		return s.writeSTOMPFrame("CONNECTED", map[string]string{
			"version":    "1.2",
			"heart-beat": "0,0",
		}, "")
	case "SUBSCRIBE":
		destination := frame.Headers["destination"]
		if destination == "/user/queue/messages" {
			subscriptionID := frame.Headers["id"]
			if subscriptionID == "" {
				subscriptionID = "sub-0"
			}
			s.stateMu.Lock()
			s.subscriptionID = subscriptionID
			s.stateMu.Unlock()
		}
		return nil
	case "SEND":
		return s.handleSend(frame.Headers["destination"], frame.Body)
	case "DISCONNECT":
		if receipt := frame.Headers["receipt"]; receipt != "" {
			_ = s.writeSTOMPFrame("RECEIPT", map[string]string{"receipt-id": receipt}, "")
		}
		return s.conn.Close()
	default:
		return nil
	}
}

func (s *stompSession) applyConnectHeaders(headers map[string]string) {
	sessionID := strings.TrimSpace(headers["X-Session-Id"])
	if sessionID == "" {
		sessionID = strings.TrimSpace(headers["x-session-id"])
	}
	if sessionID == "" {
		return
	}

	s.stateMu.Lock()
	s.sessionID = sessionID
	s.stateMu.Unlock()
}

func (s *stompSession) handleSend(destination string, body string) error {
	switch destination {
	case "/app/bind-session":
		payload := struct {
			SessionID string `json:"sessionId"`
		}{}
		_ = json.Unmarshal([]byte(body), &payload)
		return s.bindSession(payload.SessionID)
	case "/app/chat":
		payload := struct {
			Text string `json:"text"`
		}{}
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			return err
		}
		s.runChat(payload.Text)
	case "/app/permission":
		return s.handlePermission(body)
	case "/app/interrupt":
		s.cancelCurrent()
		return s.sendServerMessage(map[string]any{
			"type":   "interrupt_ack",
			"reason": "USER_INTERRUPT",
		})
	case "/app/model":
		payload := struct {
			Model string `json:"model"`
		}{}
		_ = json.Unmarshal([]byte(body), &payload)
		model := strings.TrimSpace(payload.Model)
		if model == "" {
			model = effectiveDefaultModel()
		}
		s.stateMu.Lock()
		s.model = model
		s.stateMu.Unlock()
		return s.sendServerMessage(map[string]any{
			"type":  "model_changed",
			"model": model,
		})
	case "/app/permission-mode":
		payload := struct {
			Mode string `json:"mode"`
		}{}
		_ = json.Unmarshal([]byte(body), &payload)
		mode := permission.NormalizeMode(permission.Mode(payload.Mode))
		if err := setPermissionMode(mode); err != nil {
			return err
		}
		return s.sendServerMessage(map[string]any{
			"type": "permission_mode_changed",
			"mode": string(mode),
		})
	case "/app/command":
		payload := struct {
			Command string `json:"command"`
			Args    string `json:"args"`
		}{}
		_ = json.Unmarshal([]byte(body), &payload)
		return s.sendServerMessage(map[string]any{
			"type":       "command_result",
			"command":    payload.Command,
			"resultType": "text",
			"output":     "Command handling is not implemented in the Go compatibility backend yet.",
		})
	case "/app/mcp":
		return s.sendServerMessage(map[string]any{
			"type":       "command_result",
			"command":    "mcp",
			"resultType": "text",
			"output":     "MCP is disabled in the Go compatibility backend.",
		})
	case "/app/rewind":
		payload := struct {
			MessageID string   `json:"messageId"`
			FilePaths []string `json:"filePaths"`
		}{}
		_ = json.Unmarshal([]byte(body), &payload)
		return s.sendServerMessage(map[string]any{
			"type":      "rewind_complete",
			"messageId": payload.MessageID,
			"files":     payload.FilePaths,
		})
	case "/app/elicitation":
		return nil
	case "/app/ping":
		return s.sendServerMessage(map[string]any{
			"type":      "pong",
			"timestamp": time.Now().UnixMilli(),
		})
	default:
		return nil
	}
	return nil
}

func (s *stompSession) bindSession(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || sessionID == "default" {
		return s.sendServerMessage(map[string]any{
			"type":      "error",
			"code":      "invalid_session",
			"message":   "sessionId is required",
			"retryable": true,
		})
	}

	sessions, err := getQuerySessions()
	if err != nil {
		return err
	}
	sess, err := sessions.Get(context.Background(), sessionID)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return s.sendServerMessage(map[string]any{
				"type":      "error",
				"code":      "session_not_found",
				"message":   "Session not found: " + sessionID,
				"retryable": true,
			})
		}
		return err
	}

	messages, err := sessions.ListMessages(context.Background(), sessionID, 0)
	if err != nil {
		return err
	}

	s.stateMu.Lock()
	s.sessionID = sessionID
	model := s.model
	if model == "" {
		model = effectiveDefaultModel()
		s.model = model
	}
	s.stateMu.Unlock()

	return s.sendServerMessage(map[string]any{
		"type":       "session_restored",
		"messages":   frontendMessagesFromSession(messages),
		"activities": []any{},
		"metadata": map[string]any{
			"sessionId": sess.ID,
			"model":     model,
			"status":    "idle",
		},
		"totalCount":       len(messages),
		"hasMore":          false,
		"compactSummary":   nil,
		"oldestLoadedUuid": oldestMessageID(messages),
	})
}

func (s *stompSession) handlePermission(body string) error {
	payload := struct {
		ToolUseID string `json:"toolUseId"`
		Decision  string `json:"decision"`
		Remember  bool   `json:"remember"`
		Scope     string `json:"scope"`
	}{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return err
	}

	decision := permission.Decision(payload.Decision)
	if decision == "allow_always" {
		decision = permission.DecisionAllow
	}
	if decision == "" {
		decision = permission.DecisionDeny
	}

	broker, err := getPermissionBroker()
	if err != nil {
		return err
	}
	return broker.Respond(context.Background(), permission.PermissionDecision{
		ToolUseID: payload.ToolUseID,
		Decision:  decision,
		Remember:  payload.Remember,
		Scope:     payload.Scope,
	})
}

func (s *stompSession) runChat(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		_ = s.sendServerMessage(map[string]any{
			"type":      "error",
			"code":      "invalid_request",
			"message":   "message text is required",
			"retryable": true,
		})
		return
	}

	sessionID := s.currentSessionID()
	model := s.currentModel()
	if sessionID == "" || sessionID == "default" {
		sessions, err := getQuerySessions()
		if err != nil {
			_ = s.sendServerError("storage_error", err.Error(), true)
			return
		}
		sess, err := sessions.Create(context.Background())
		if err != nil {
			_ = s.sendServerError("session_error", err.Error(), true)
			return
		}
		sessionID = sess.ID
		s.stateMu.Lock()
		s.sessionID = sessionID
		s.stateMu.Unlock()
		_ = s.bindSession(sessionID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runID := s.setCurrentCancel(cancel)

	go func() {
		defer s.clearCurrentCancel(runID)
		defer cancel()

		queryEngine, err := newWebSocketQueryEngine()
		if err != nil {
			_ = s.sendServerError("query_engine_error", err.Error(), true)
			return
		}

		events, err := queryEngine.Stream(ctx, engine.QueryRequest{
			SessionID: sessionID,
			Model:     model,
			Prompt:    text,
		})
		if err != nil {
			_ = s.sendServerError("query_error", err.Error(), true)
			return
		}

		for event := range events {
			for _, message := range serverMessagesFromEngineEvent(event) {
				if err := s.sendServerMessage(message); err != nil {
					return
				}
			}
		}
		_ = s.sendServerMessage(map[string]any{"type": "session_list_updated"})
	}()
}

func newWebSocketQueryEngine() (*engine.QueryEngine, error) {
	cfg, err := llm.LoadConfig()
	if err != nil {
		return nil, err
	}

	sessions, err := getQuerySessions()
	if err != nil {
		return nil, err
	}

	workspacePath, err := loadWorkspacePath()
	if err != nil {
		return nil, err
	}

	broker, err := getPermissionBroker()
	if err != nil {
		return nil, err
	}

	return newRuntimeQueryEngine(
		llm.NewOpenAICompatibleClient(cfg),
		sessions,
		workspacePath,
		broker,
	)
}

func serverMessagesFromEngineEvent(event engine.Event) []map[string]any {
	switch event.Type {
	case engine.EventStreamDelta:
		return []map[string]any{{
			"type":      "stream_delta",
			"delta":     event.Delta,
			"messageId": event.SessionID,
		}}
	case engine.EventToolUseStart:
		return []map[string]any{{
			"type":      "tool_use_start",
			"toolUseId": event.ToolUseID,
			"toolName":  event.ToolName,
			"input":     normalizeToolInput(event.Payload),
		}}
	case engine.EventToolUseProgress:
		return []map[string]any{{
			"type":      "tool_use_progress",
			"toolUseId": event.ToolUseID,
			"progress":  stringifyPayload(event.Payload),
		}}
	case engine.EventToolResult:
		content, isError, metadata := normalizeToolResult(event.Payload)
		return []map[string]any{{
			"type":      "tool_result",
			"toolUseId": event.ToolUseID,
			"content":   content,
			"isError":   isError,
			"result": map[string]any{
				"content":  content,
				"isError":  isError,
				"metadata": metadata,
			},
		}}
	case engine.EventPermissionRequest:
		payload := mapPayload(event.Payload)
		payload["type"] = "permission_request"
		payload["toolUseId"] = event.ToolUseID
		payload["toolName"] = event.ToolName
		return []map[string]any{payload}
	case engine.EventMessageComplete:
		payload := mapPayload(event.Payload)
		usage := frontendUsageFromAny(payload["usage"])
		return []map[string]any{
			{
				"type":       "message_complete",
				"messageId":  event.SessionID,
				"usage":      usage,
				"stopReason": "end_turn",
			},
			{
				"type":        "cost_update",
				"sessionCost": 0,
				"totalCost":   0,
				"usage":       usage,
			},
		}
	case engine.EventError:
		payload := mapPayload(event.Payload)
		message, _ := payload["message"].(string)
		if message == "" {
			message = "stream query failed"
		}
		return []map[string]any{{
			"type":      "error",
			"code":      "query_error",
			"message":   message,
			"retryable": true,
		}}
	default:
		return nil
	}
}

func normalizeToolInput(payload any) any {
	payloadMap := mapPayload(payload)
	arguments, ok := payloadMap["arguments"]
	if !ok {
		return payloadMap
	}

	switch value := arguments.(type) {
	case json.RawMessage:
		var decoded any
		if err := json.Unmarshal(value, &decoded); err == nil {
			return decoded
		}
	case []byte:
		var decoded any
		if err := json.Unmarshal(value, &decoded); err == nil {
			return decoded
		}
	case string:
		var decoded any
		if err := json.Unmarshal([]byte(value), &decoded); err == nil {
			return decoded
		}
	}
	return arguments
}

func normalizeToolResult(payload any) (string, bool, map[string]any) {
	payloadMap := mapPayload(payload)
	content, _ := payloadMap["content"].(string)
	errorText, _ := payloadMap["error"].(string)
	isError := errorText != ""
	if content == "" {
		content = errorText
	}
	if content == "" {
		content = stringifyPayload(payload)
	}
	return content, isError, payloadMap
}

func mapPayload(payload any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	if payloadMap, ok := payload.(map[string]any); ok {
		return payloadMap
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return map[string]any{"value": stringifyPayload(payload)}
	}
	result := map[string]any{}
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]any{"value": string(data)}
	}
	return result
}

func stringifyPayload(payload any) string {
	switch value := payload.(type) {
	case nil:
		return ""
	case string:
		return value
	case []byte:
		return string(value)
	case json.RawMessage:
		return string(value)
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(data)
	}
}

func frontendUsageFromAny(value any) map[string]int {
	usage := zeroFrontendUsage()
	switch typed := value.(type) {
	case llm.Usage:
		usage["inputTokens"] = typed.PromptTokens
		usage["outputTokens"] = typed.CompletionTokens
	case map[string]any:
		usage["inputTokens"] = intFromAny(firstNonNil(typed["prompt_tokens"], typed["promptTokens"], typed["inputTokens"]))
		usage["outputTokens"] = intFromAny(firstNonNil(typed["completion_tokens"], typed["completionTokens"], typed["outputTokens"]))
	case map[string]int:
		usage["inputTokens"] = firstNonZero(typed["prompt_tokens"], typed["promptTokens"], typed["inputTokens"])
		usage["outputTokens"] = firstNonZero(typed["completion_tokens"], typed["completionTokens"], typed["outputTokens"])
	}
	return usage
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		result, _ := typed.Int64()
		return int(result)
	case string:
		result, _ := strconv.Atoi(typed)
		return result
	default:
		return 0
	}
}

func (s *stompSession) sendServerError(code string, message string, retryable bool) error {
	return s.sendServerMessage(map[string]any{
		"type":      "error",
		"code":      code,
		"message":   message,
		"retryable": retryable,
	})
}

func (s *stompSession) sendServerMessage(message map[string]any) error {
	if _, ok := message["ts"]; !ok {
		message["ts"] = time.Now().UnixMilli()
	}
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}

	headers := map[string]string{
		"destination":  "/user/queue/messages",
		"subscription": s.currentSubscriptionID(),
		"message-id":   nextWSMessageID(),
		"content-type": "application/json;charset=utf-8",
	}
	return s.writeSTOMPFrame("MESSAGE", headers, string(body))
}

func (s *stompSession) writeSTOMPFrame(command string, headers map[string]string, body string) error {
	frame := buildSTOMPFrame(command, headers, body)
	if s.sockJS {
		data, err := json.Marshal([]string{frame})
		if err != nil {
			return err
		}
		return s.writeRaw("a" + string(data))
	}
	return s.writeRaw(frame)
}

func buildSTOMPFrame(command string, headers map[string]string, body string) string {
	var builder strings.Builder
	builder.WriteString(command)
	builder.WriteByte('\n')

	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte(':')
		builder.WriteString(headers[key])
		builder.WriteByte('\n')
	}

	builder.WriteByte('\n')
	builder.WriteString(body)
	builder.WriteByte(0)
	return builder.String()
}

func (s *stompSession) writeRaw(message string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, []byte(message))
}

func (s *stompSession) currentSubscriptionID() string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.subscriptionID != "" {
		return s.subscriptionID
	}
	return "sub-0"
}

func (s *stompSession) currentSessionID() string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.sessionID
}

func (s *stompSession) currentModel() string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.model != "" {
		return s.model
	}
	return effectiveDefaultModel()
}

func (s *stompSession) setCurrentCancel(cancel context.CancelFunc) uint64 {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.currentCancel != nil {
		s.currentCancel()
	}
	s.currentRunID++
	s.currentCancel = cancel
	return s.currentRunID
}

func (s *stompSession) clearCurrentCancel(runID uint64) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.currentRunID == runID {
		s.currentCancel = nil
	}
}

func (s *stompSession) cancelCurrent() {
	s.stateMu.Lock()
	cancel := s.currentCancel
	s.currentCancel = nil
	s.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func nextWSMessageID() string {
	return "msg-" + strconv.FormatUint(atomic.AddUint64(&wsMessageSeq, 1), 10)
}

func oldestMessageID(messages []session.Message) string {
	if len(messages) == 0 {
		return ""
	}
	return messages[0].ID
}
