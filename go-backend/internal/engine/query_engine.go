package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go-backend/internal/llm"
	"go-backend/internal/session"
	"go-backend/internal/tools"
)

var ErrMaxToolRoundsExceeded = errors.New("maximum tool rounds exceeded")

type QueryEngine struct {
	LLM      llm.LLMClient
	Sessions session.Service
	Tools    tools.ToolRegistry
	Config   Config
}

func NewQueryEngine(llmClient llm.LLMClient, sessions session.Service, toolRegistry tools.ToolRegistry, cfg Config) *QueryEngine {
	return &QueryEngine{
		LLM:      llmClient,
		Sessions: sessions,
		Tools:    toolRegistry,
		Config:   cfg.WithDefaults(),
	}
}

// Query 执行第五阶段的最小 agent 循环：
// session -> history -> LLM -> optional tools -> LLM -> final assistant message.
func (e *QueryEngine) Query(ctx context.Context, req QueryRequest) (QueryResult, error) {
	// 先检查 QueryEngine 是否正确组装，避免后面出现不清楚的空指针错误。
	if e == nil {
		return QueryResult{}, errors.New("query engine is nil")
	}
	if e.LLM == nil {
		return QueryResult{}, errors.New("query engine LLM client is nil")
	}
	if e.Sessions == nil {
		return QueryResult{}, errors.New("query engine session service is nil")
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return QueryResult{}, errors.New("prompt is required")
	}

	// 有 session_id 就复用已有会话；没有就创建一个新会话。
	sess, err := e.Sessions.GetOrCreate(ctx, req.SessionID)
	if err != nil {
		return QueryResult{}, err
	}

	// 先读取历史消息，再追加本轮用户消息，避免当前 prompt 被重复放进 LLM 请求。
	history, err := e.Sessions.ListMessages(ctx, sess.ID, e.Config.MaxHistoryMessages)
	if err != nil {
		return QueryResult{}, err
	}

	// 本次请求可以临时覆盖默认 system prompt。
	systemPrompt := strings.TrimSpace(req.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = e.Config.DefaultSystemPrompt
	}

	// 把 system prompt、历史消息和本轮输入拼成完整的 LLM messages。
	messages := buildMessages(systemPrompt, history, prompt)

	// 先保存用户消息。即使后续 LLM 调用失败，也能知道用户当时问了什么。
	if err := e.Sessions.AppendMessage(ctx, sess.ID, session.Message{
		Role:    llm.RoleUser,
		Content: prompt,
	}); err != nil {
		return QueryResult{}, err
	}

	return e.runLoop(ctx, sess.ID, req.Model, messages)
}

func (e *QueryEngine) runLoop(ctx context.Context, sessionID string, model string, messages []llm.ChatMessage) (QueryResult, error) {
	var totalUsage llm.Usage
	var lastModel string
	toolDefinitions := e.toolDefinitions()

	for round := 0; ; round++ {
		resp, err := e.LLM.Chat(ctx, llm.ChatRequest{
			Model:    model,
			Messages: messages,
			Tools:    toolDefinitions,
		})
		if err != nil {
			return QueryResult{}, err
		}
		addUsage(&totalUsage, resp.Usage)
		if resp.Model != "" {
			lastModel = resp.Model
		}

		// 没有 tool call 就说明模型已经给出最终文本，可以持久化 assistant 回复。
		if len(resp.Message.ToolCalls) == 0 {
			if err := e.Sessions.AppendMessage(ctx, sessionID, session.Message{
				Role:    resp.Message.Role,
				Content: resp.Message.Content,
			}); err != nil {
				return QueryResult{}, err
			}

			return QueryResult{
				SessionID: sessionID,
				Text:      resp.Message.Content,
				Model:     lastModel,
				Usage:     totalUsage,
			}, nil
		}

		if round >= e.Config.MaxToolRounds {
			return QueryResult{}, ErrMaxToolRoundsExceeded
		}

		// tool call 本身必须放回 messages；紧跟着再放 tool result。
		// 这两类消息只参与本轮内存上下文，当前 SQLite schema 先只持久化用户消息和最终回答。
		messages = append(messages, resp.Message)
		for _, call := range resp.Message.ToolCalls {
			messages = append(messages, e.runToolCall(ctx, call))
		}
	}
}

func (e *QueryEngine) toolDefinitions() []llm.ToolDefinition {
	if e.Tools == nil {
		return nil
	}
	return e.Tools.Definitions()
}

func (e *QueryEngine) runToolCall(ctx context.Context, call llm.ToolCall) llm.ChatMessage {
	result := tools.ToolResult{}
	runErr := error(nil)

	if e.Tools == nil {
		runErr = errors.New("tool registry is not configured")
		return llm.ChatMessage{
			Role:       llm.RoleTool,
			ToolCallID: call.ID,
			Content:    encodeToolResult(result, runErr),
		}
	}

	tool, ok := e.Tools.Get(call.Name)
	if !ok {
		runErr = fmt.Errorf("tool %q is not registered", call.Name)
	} else {
		result, runErr = tool.Run(ctx, call.Arguments)
	}

	return llm.ChatMessage{
		Role:       llm.RoleTool,
		ToolCallID: call.ID,
		Content:    encodeToolResult(result, runErr),
	}
}

func encodeToolResult(result tools.ToolResult, runErr error) string {
	if runErr != nil && result.Error == "" {
		result.Error = runErr.Error()
	}
	if result.Content == "" && result.Data == nil && result.Error == "" {
		result.Content = "ok"
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}

func addUsage(total *llm.Usage, usage llm.Usage) {
	total.PromptTokens += usage.PromptTokens
	total.CompletionTokens += usage.CompletionTokens
	total.TotalTokens += usage.TotalTokens
}

// buildMessages 把会话里的历史消息转换成 LLM client 需要的 chat messages。
func buildMessages(systemPrompt string, history []session.Message, prompt string) []llm.ChatMessage {
	messages := make([]llm.ChatMessage, 0, len(history)+2)
	if systemPrompt != "" {
		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleSystem,
			Content: systemPrompt,
		})
	}

	for _, message := range history {
		if message.Content == "" {
			continue
		}
		messages = append(messages, llm.ChatMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}

	messages = append(messages, llm.ChatMessage{
		Role:    llm.RoleUser,
		Content: prompt,
	})
	return messages
}
