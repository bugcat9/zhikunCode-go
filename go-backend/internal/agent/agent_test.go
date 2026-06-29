package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-backend/internal/engine"
	"go-backend/internal/llm"
	"go-backend/internal/tools"
)

type fakeQueryRunner struct {
	req    engine.QueryRequest
	result engine.QueryResult
	err    error
}

func (r *fakeQueryRunner) Query(ctx context.Context, req engine.QueryRequest) (engine.QueryResult, error) {
	r.req = req
	if r.err != nil {
		return engine.QueryResult{}, r.err
	}
	return r.result, nil
}

type blockingQueryRunner struct {
	started  chan struct{}
	canceled chan struct{}
}

func (r *blockingQueryRunner) Query(ctx context.Context, req engine.QueryRequest) (engine.QueryResult, error) {
	close(r.started)
	<-ctx.Done()
	close(r.canceled)
	return engine.QueryResult{}, ctx.Err()
}

func TestAgentRunCallsQueryEngine(t *testing.T) {
	runner := &fakeQueryRunner{
		result: engine.QueryResult{
			SessionID: "child-session",
			Text:      "child answer",
			Model:     "test-model",
			Usage:     llm.Usage{TotalTokens: 7},
		},
	}
	agent, err := NewAgent("agent-1", runner)
	if err != nil {
		t.Fatal(err)
	}

	result, err := agent.Run(context.Background(), Task{
		ID:           "task-1",
		ParentID:     "main",
		Model:        "test-model",
		Instruction:  "summarize this package",
		SystemPrompt: "be brief",
	})
	if err != nil {
		t.Fatal(err)
	}

	if runner.req.Prompt != "summarize this package" || runner.req.Model != "test-model" || runner.req.SystemPrompt != "be brief" {
		t.Fatalf("unexpected query request: %#v", runner.req)
	}
	if result.Status != TaskStatusCompleted || result.Text != "child answer" || result.SessionID != "child-session" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Usage.TotalTokens != 7 {
		t.Fatalf("usage was not copied: %#v", result.Usage)
	}
}

func TestManagerCreateTaskReturnsSubAgentResult(t *testing.T) {
	runner := &fakeQueryRunner{
		result: engine.QueryResult{
			SessionID: "child-session",
			Text:      "done",
			Model:     "test-model",
		},
	}
	ids := []string{"task-1", "agent-1"}
	manager, err := NewManager(ManagerConfig{
		ParentID: "main-agent",
		Engine:   runner,
		NewID: func(prefix string) string {
			if len(ids) == 0 {
				return prefix + "-extra"
			}
			id := ids[0]
			ids = ids[1:]
			return id
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := manager.CreateTask(context.Background(), taskCreateRequest("inspect the repository"))
	if err != nil {
		t.Fatal(err)
	}

	if result.TaskID != "task-1" || result.AgentID != "agent-1" || result.ParentID != "main-agent" {
		t.Fatalf("unexpected task result ids: %#v", result)
	}
	if result.Status != string(TaskStatusCompleted) || result.Text != "done" {
		t.Fatalf("unexpected task result: %#v", result)
	}

	stored, ok := manager.Task("task-1")
	if !ok {
		t.Fatal("expected task to be stored")
	}
	if stored.Status != TaskStatusCompleted {
		t.Fatalf("stored task status = %q", stored.Status)
	}
}

func TestManagerCreateTaskCancelsChildWithParentContext(t *testing.T) {
	runner := &blockingQueryRunner{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
	}
	manager, err := NewManager(ManagerConfig{
		ParentID: "main-agent",
		Engine:   runner,
		NewID: func(prefix string) string {
			return prefix + "-1"
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := manager.CreateTask(ctx, taskCreateRequest("wait until canceled"))
		done <- err
	}()

	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("subagent did not start")
	}

	cancel()

	select {
	case <-runner.canceled:
	case <-time.After(time.Second):
		t.Fatal("subagent did not observe parent cancellation")
	}

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("manager did not return after cancellation")
	}
}

func taskCreateRequest(instruction string) tools.TaskCreateRequest {
	return tools.TaskCreateRequest{Instruction: instruction}
}
