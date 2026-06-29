package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeTaskSpawner struct {
	req    TaskCreateRequest
	result TaskCreateResult
	err    error
}

func (s *fakeTaskSpawner) CreateTask(ctx context.Context, req TaskCreateRequest) (TaskCreateResult, error) {
	s.req = req
	if s.err != nil {
		return s.result, s.err
	}
	return s.result, nil
}

func TestTaskCreateToolRunsSpawner(t *testing.T) {
	spawner := &fakeTaskSpawner{
		result: TaskCreateResult{
			TaskID:    "task-1",
			AgentID:   "agent-1",
			Status:    "completed",
			Text:      "done",
			SessionID: "child-session",
		},
	}
	tool := NewTaskCreateTool(spawner)

	result, err := tool.Run(context.Background(), json.RawMessage(`{"instruction":"  inspect code  ","model":"test-model"}`))
	if err != nil {
		t.Fatal(err)
	}

	if spawner.req.Instruction != "inspect code" || spawner.req.Model != "test-model" {
		t.Fatalf("unexpected spawner request: %#v", spawner.req)
	}
	if !strings.Contains(result.Content, "done") {
		t.Fatalf("expected content to include subagent text, got %q", result.Content)
	}

	data, ok := result.Data.(TaskCreateResult)
	if !ok {
		t.Fatalf("expected TaskCreateResult data, got %#v", result.Data)
	}
	if data.SessionID != "child-session" {
		t.Fatalf("unexpected data: %#v", data)
	}
}

func TestTaskCreateToolRequiresInstruction(t *testing.T) {
	tool := NewTaskCreateTool(&fakeTaskSpawner{})

	_, err := tool.Run(context.Background(), json.RawMessage(`{"instruction":" "}`))
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTaskCreateToolRequiresSpawner(t *testing.T) {
	tool := NewTaskCreateTool(nil)

	_, err := tool.Run(context.Background(), json.RawMessage(`{"instruction":"inspect code"}`))
	if !errors.Is(err, ErrTaskSpawnerUnavailable) {
		t.Fatalf("expected ErrTaskSpawnerUnavailable, got %v", err)
	}
}
