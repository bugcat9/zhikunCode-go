package engine

import (
	"context"
	"errors"
	"strings"

	"go-backend/internal/llm"
	"go-backend/internal/session"
)

type QueryEngine struct {
	LLM      llm.LLMClient
	Sessions session.Service
	Config   Config
}

func NewQueryEngine(llmClient llm.LLMClient, sessions session.Service, cfg Config) *QueryEngine {
	return &QueryEngine{
		LLM:      llmClient,
		Sessions: sessions,
		Config:   cfg.WithDefaults(),
	}
}

// Query 执行第四阶段的纯文本对话流程：
// session -> history -> LLM -> persisted assistant message.
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

	// 第四阶段只做非流式纯文本对话，工具调用和流式输出后面阶段再接。
	resp, err := e.LLM.Chat(ctx, llm.ChatRequest{
		Model:    req.Model,
		Messages: messages,
	})
	if err != nil {
		return QueryResult{}, err
	}

	// 保存助手回复，下一轮对话才能把它作为历史上下文传给 LLM。
	if err := e.Sessions.AppendMessage(ctx, sess.ID, session.Message{
		Role:    resp.Message.Role,
		Content: resp.Message.Content,
	}); err != nil {
		return QueryResult{}, err
	}

	return QueryResult{
		SessionID: sess.ID,
		Text:      resp.Message.Content,
		Model:     resp.Model,
		Usage:     resp.Usage,
	}, nil
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
