package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go-backend/internal/engine"
	"go-backend/internal/llm"
)

type coordinatorRunner struct {
	delay    time.Duration
	failures map[string]error

	mu        sync.Mutex
	active    int
	maxActive int
	requests  []engine.QueryRequest
}

func (r *coordinatorRunner) Query(ctx context.Context, req engine.QueryRequest) (engine.QueryResult, error) {
	r.mu.Lock()
	r.active++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	r.requests = append(r.requests, req)
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.active--
		r.mu.Unlock()
	}()

	if r.delay > 0 {
		select {
		case <-ctx.Done():
			return engine.QueryResult{}, ctx.Err()
		case <-time.After(r.delay):
		}
	}

	if err := r.failures[req.Prompt]; err != nil {
		return engine.QueryResult{}, err
	}

	return engine.QueryResult{
		SessionID: "session-" + strings.ReplaceAll(req.Prompt, " ", "-"),
		Text:      "answer: " + req.Prompt,
		Model:     req.Model,
		Usage:     llm.Usage{TotalTokens: 1},
	}, nil
}

func (r *coordinatorRunner) MaxActive() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maxActive
}

type blockingCoordinatorRunner struct {
	started  chan struct{}
	canceled chan struct{}
	once     sync.Once
}

func (r *blockingCoordinatorRunner) Query(ctx context.Context, req engine.QueryRequest) (engine.QueryResult, error) {
	r.once.Do(func() {
		close(r.started)
	})

	<-ctx.Done()
	close(r.canceled)
	return engine.QueryResult{}, ctx.Err()
}

type recordingAggregator struct {
	called  bool
	results []AgentResult
}

func (a *recordingAggregator) Aggregate(ctx context.Context, req CoordinatorRequest, results []AgentResult) (string, error) {
	a.called = true
	a.results = append([]AgentResult(nil), results...)
	return "custom aggregate", nil
}

func TestCoordinatorRespectsMaxConcurrency(t *testing.T) {
	runner := &coordinatorRunner{delay: 20 * time.Millisecond}
	coordinator, err := NewCoordinator(CoordinatorConfig{
		Engine:         runner,
		MaxConcurrency: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := coordinator.Run(context.Background(), CoordinatorRequest{
		Instruction: "inspect several areas",
		Tasks: []Task{
			{Instruction: "task one"},
			{Instruction: "task two"},
			{Instruction: "task three"},
			{Instruction: "task four"},
			{Instruction: "task five"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != TaskStatusCompleted {
		t.Fatalf("unexpected coordinator status: %#v", result)
	}
	if runner.MaxActive() > 2 {
		t.Fatalf("expected max concurrency <= 2, got %d", runner.MaxActive())
	}
	if len(result.Results) != 5 {
		t.Fatalf("expected five worker results, got %d", len(result.Results))
	}
	if result.Usage.TotalTokens != 5 {
		t.Fatalf("expected usage to be aggregated, got %#v", result.Usage)
	}
}

func TestCoordinatorKeepsRunningWhenWorkerFails(t *testing.T) {
	workerErr := errors.New("worker boom")
	runner := &coordinatorRunner{
		failures: map[string]error{
			"fail task": workerErr,
		},
	}
	coordinator, err := NewCoordinator(CoordinatorConfig{Engine: runner})
	if err != nil {
		t.Fatal(err)
	}

	result, err := coordinator.Run(context.Background(), CoordinatorRequest{
		Instruction: "mix success and failure",
		Tasks: []Task{
			{ID: "ok", Instruction: "ok task"},
			{ID: "fail", Instruction: "fail task"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != TaskStatusCompleted {
		t.Fatalf("expected completed when at least one worker succeeds, got %#v", result)
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected two worker results, got %d", len(result.Results))
	}

	var sawFailure bool
	for _, workerResult := range result.Results {
		if workerResult.TaskID == "fail" {
			sawFailure = true
			if workerResult.Status != TaskStatusFailed || !strings.Contains(workerResult.Error, workerErr.Error()) {
				t.Fatalf("failure result was not preserved: %#v", workerResult)
			}
		}
	}
	if !sawFailure {
		t.Fatalf("expected failed worker result in %#v", result.Results)
	}
	if !strings.Contains(result.Text, "1 succeeded, 1 failed") {
		t.Fatalf("expected summary to include success/failure counts, got %q", result.Text)
	}
}

func TestCoordinatorReturnsErrorWhenAllWorkersFail(t *testing.T) {
	runner := &coordinatorRunner{
		failures: map[string]error{
			"fail one": errors.New("boom one"),
			"fail two": errors.New("boom two"),
		},
	}
	coordinator, err := NewCoordinator(CoordinatorConfig{Engine: runner})
	if err != nil {
		t.Fatal(err)
	}

	result, err := coordinator.Run(context.Background(), CoordinatorRequest{
		Instruction: "all fail",
		Tasks: []Task{
			{Instruction: "fail one"},
			{Instruction: "fail two"},
		},
	})
	if !errors.Is(err, ErrNoSuccessfulWorker) {
		t.Fatalf("expected ErrNoSuccessfulWorker, got %v", err)
	}
	if result.Status != TaskStatusFailed {
		t.Fatalf("expected failed coordinator status, got %#v", result)
	}
}

func TestCoordinatorCancelsWorkersWithParentContext(t *testing.T) {
	runner := &blockingCoordinatorRunner{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
	}
	coordinator, err := NewCoordinator(CoordinatorConfig{Engine: runner})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := coordinator.Run(ctx, CoordinatorRequest{
			Instruction: "cancel work",
			Tasks:       []Task{{Instruction: "wait"}},
		})
		done <- err
	}()

	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}

	cancel()

	select {
	case <-runner.canceled:
	case <-time.After(time.Second):
		t.Fatal("worker did not observe cancellation")
	}

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("coordinator did not return after cancellation")
	}
}

func TestCoordinatorTimeoutCancelsWorkers(t *testing.T) {
	runner := &coordinatorRunner{delay: 100 * time.Millisecond}
	coordinator, err := NewCoordinator(CoordinatorConfig{
		Engine:  runner,
		Timeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := coordinator.Run(context.Background(), CoordinatorRequest{
		Instruction: "timeout work",
		Tasks:       []Task{{Instruction: "slow"}},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if result.Status != TaskStatusCanceled {
		t.Fatalf("expected canceled result, got %#v", result)
	}
}

func TestCoordinatorUsesAggregator(t *testing.T) {
	runner := &coordinatorRunner{}
	aggregator := &recordingAggregator{}
	coordinator, err := NewCoordinator(CoordinatorConfig{
		Engine:     runner,
		Aggregator: aggregator,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := coordinator.Run(context.Background(), CoordinatorRequest{
		Instruction: "aggregate work",
		Tasks:       []Task{{Instruction: "one"}, {Instruction: "two"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !aggregator.called || len(aggregator.results) != 2 {
		t.Fatalf("aggregator was not called with worker results: %#v", aggregator)
	}
	if result.Text != "custom aggregate" {
		t.Fatalf("expected custom aggregate text, got %q", result.Text)
	}
}
