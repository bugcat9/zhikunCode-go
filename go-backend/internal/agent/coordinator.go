package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-backend/internal/llm"
)

const DefaultMaxConcurrency = 3

type CoordinatorConfig struct {
	ID               string
	Engine           QueryRunner
	Planner          TaskPlanner
	Aggregator       Aggregator
	PermissionBridge PermissionBridge
	MaxConcurrency   int
	Timeout          time.Duration
	NewID            func(prefix string) string
	Now              func() time.Time
}

type Coordinator struct {
	id               string
	engine           QueryRunner
	planner          TaskPlanner
	aggregator       Aggregator
	permissionBridge PermissionBridge
	maxConcurrency   int
	timeout          time.Duration
	newID            func(prefix string) string
	now              func() time.Time

	sequence uint64
}

type CoordinatorResult struct {
	ID             string        `json:"id"`
	Status         TaskStatus    `json:"status"`
	Text           string        `json:"text,omitempty"`
	Results        []AgentResult `json:"results,omitempty"`
	Usage          llm.Usage     `json:"usage,omitempty"`
	MaxConcurrency int           `json:"max_concurrency"`
	StartedAt      time.Time     `json:"started_at,omitempty"`
	CompletedAt    time.Time     `json:"completed_at,omitempty"`
	Error          string        `json:"error,omitempty"`
}

func NewCoordinator(cfg CoordinatorConfig) (*Coordinator, error) {
	if cfg.Engine == nil {
		return nil, ErrNilEngine
	}

	id := strings.TrimSpace(cfg.ID)
	if id == "" {
		id = "coordinator"
	}

	planner := cfg.Planner
	if planner == nil {
		planner = NewStaticTaskPlanner()
	}

	aggregator := cfg.Aggregator
	if aggregator == nil {
		aggregator = NewSummaryAggregator()
	}

	permissionBridge := cfg.PermissionBridge
	if permissionBridge == nil {
		permissionBridge = NoopPermissionBridge{}
	}

	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	return &Coordinator{
		id:               id,
		engine:           cfg.Engine,
		planner:          planner,
		aggregator:       aggregator,
		permissionBridge: permissionBridge,
		maxConcurrency:   normalizeMaxConcurrency(cfg.MaxConcurrency),
		timeout:          cfg.Timeout,
		newID:            cfg.NewID,
		now:              now,
	}, nil
}

func (c *Coordinator) Run(ctx context.Context, req CoordinatorRequest) (CoordinatorResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil {
		return CoordinatorResult{}, ErrInvalidCoordinator
	}
	if c.engine == nil {
		return CoordinatorResult{}, ErrNilEngine
	}
	if c.planner == nil {
		c.planner = NewStaticTaskPlanner()
	}
	if c.aggregator == nil {
		c.aggregator = NewSummaryAggregator()
	}
	if c.permissionBridge == nil {
		c.permissionBridge = NoopPermissionBridge{}
	}

	startedAt := c.now()
	result := CoordinatorResult{
		ID:             c.coordinatorID(req.ID),
		Status:         TaskStatusRunning,
		MaxConcurrency: c.effectiveMaxConcurrency(req.MaxConcurrency),
		StartedAt:      startedAt,
	}

	runCtx := ctx
	cancel := func() {}
	timeout := c.effectiveTimeout(req.Timeout)
	if timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	tasks, err := c.planner.Plan(runCtx, req)
	if err != nil {
		return c.failResult(result, err), err
	}

	tasks, err = c.prepareTasks(runCtx, result.ID, req, tasks)
	if err != nil {
		return c.failResult(result, err), err
	}
	if len(tasks) == 0 {
		return c.failResult(result, ErrNoTasks), ErrNoTasks
	}

	workerResults := c.runWorkers(runCtx, tasks, result.MaxConcurrency)
	agentResults := agentResultsFromWorkerResults(workerResults)
	result.Results = agentResults
	result.Usage = sumUsage(agentResults)
	result.CompletedAt = c.now()

	text, aggregateErr := c.aggregator.Aggregate(runCtx, req, agentResults)
	result.Text = text

	if runCtx.Err() != nil {
		result.Status = TaskStatusCanceled
		result.Error = runCtx.Err().Error()
		return result, runCtx.Err()
	}
	if aggregateErr != nil {
		result.Status = TaskStatusFailed
		result.Error = aggregateErr.Error()
		return result, aggregateErr
	}

	if hasSuccessfulWorker(agentResults) {
		result.Status = TaskStatusCompleted
		return result, nil
	}

	result.Status = TaskStatusFailed
	result.Error = ErrNoSuccessfulWorker.Error()
	return result, ErrNoSuccessfulWorker
}

func (c *Coordinator) prepareTasks(ctx context.Context, coordinatorID string, req CoordinatorRequest, tasks []Task) ([]Task, error) {
	prepared := make([]Task, 0, len(tasks))
	for i, task := range tasks {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		task = c.applyTaskDefaults(coordinatorID, req, task, i)
		next, err := c.permissionBridge.PrepareTask(ctx, task)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, next)
	}
	return prepared, nil
}

func (c *Coordinator) runWorkers(ctx context.Context, tasks []Task, maxConcurrency int) []WorkerResult {
	mailbox := NewMailbox(len(tasks))
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

launch:
	for _, task := range tasks {
		select {
		case <-ctx.Done():
			break launch
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(task Task) {
			defer wg.Done()
			defer func() { <-sem }()

			worker, err := NewWorkerAgent(task.WorkerID(), c.engine)
			if err != nil {
				mailbox.Send(ctx, WorkerResult{
					Task: task,
					Result: AgentResult{
						TaskID:      task.ID,
						ParentID:    task.ParentID,
						SessionID:   task.SessionID,
						Status:      TaskStatusFailed,
						Error:       err.Error(),
						CompletedAt: c.now(),
					},
					Err: err,
				})
				return
			}

			result, err := worker.Run(ctx, task)
			mailbox.Send(ctx, WorkerResult{
				Task:   task,
				Result: result,
				Err:    err,
			})
		}(task)
	}

	go func() {
		wg.Wait()
		mailbox.Close()
	}()

	return mailbox.Collect()
}

func (c *Coordinator) applyTaskDefaults(coordinatorID string, req CoordinatorRequest, task Task, index int) Task {
	if strings.TrimSpace(task.ID) == "" {
		task.ID = c.nextID("task")
	}
	if strings.TrimSpace(task.ParentID) == "" {
		task.ParentID = coordinatorID
	}
	if strings.TrimSpace(task.SessionID) == "" {
		task.SessionID = strings.TrimSpace(req.SessionID)
	}
	if strings.TrimSpace(task.Model) == "" {
		task.Model = strings.TrimSpace(req.Model)
	}
	if strings.TrimSpace(task.SystemPrompt) == "" {
		task.SystemPrompt = strings.TrimSpace(req.SystemPrompt)
	}
	task.Instruction = strings.TrimSpace(task.Instruction)
	if task.Instruction == "" {
		task.Instruction = strings.TrimSpace(req.Instruction)
	}
	if task.Instruction == "" {
		task.Instruction = fmt.Sprintf("Worker task %d for coordinator %s", index+1, coordinatorID)
	}
	if task.Status == "" {
		task.Status = TaskStatusPending
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = c.now()
	}
	return task
}

func (c *Coordinator) coordinatorID(id string) string {
	id = strings.TrimSpace(id)
	if id != "" {
		return id
	}
	return c.nextID("coord")
}

func (c *Coordinator) nextID(prefix string) string {
	if c.newID != nil {
		if id := strings.TrimSpace(c.newID(prefix)); id != "" {
			return id
		}
	}

	seq := atomic.AddUint64(&c.sequence, 1)
	return prefix + "-" + strconv.FormatInt(c.now().UnixNano(), 36) + "-" + strconv.FormatUint(seq, 36)
}

func (c *Coordinator) effectiveMaxConcurrency(value int) int {
	if value > 0 {
		return value
	}
	return normalizeMaxConcurrency(c.maxConcurrency)
}

func (c *Coordinator) effectiveTimeout(value time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return c.timeout
}

func (c *Coordinator) failResult(result CoordinatorResult, err error) CoordinatorResult {
	result.Status = TaskStatusFailed
	result.CompletedAt = c.now()
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func normalizeMaxConcurrency(value int) int {
	if value <= 0 {
		return DefaultMaxConcurrency
	}
	return value
}

func agentResultsFromWorkerResults(workerResults []WorkerResult) []AgentResult {
	results := make([]AgentResult, 0, len(workerResults))
	for _, workerResult := range workerResults {
		result := workerResult.Result
		if workerResult.Err != nil && result.Error == "" {
			result.Error = workerResult.Err.Error()
		}
		if result.Status == "" {
			if workerResult.Err != nil {
				result.Status = TaskStatusFailed
			} else {
				result.Status = TaskStatusCompleted
			}
		}
		results = append(results, result)
	}
	return results
}

func sumUsage(results []AgentResult) llm.Usage {
	var usage llm.Usage
	for _, result := range results {
		usage.PromptTokens += result.Usage.PromptTokens
		usage.CompletionTokens += result.Usage.CompletionTokens
		usage.TotalTokens += result.Usage.TotalTokens
	}
	return usage
}

func hasSuccessfulWorker(results []AgentResult) bool {
	for _, result := range results {
		if result.Status == TaskStatusCompleted {
			return true
		}
	}
	return false
}

func (t Task) WorkerID() string {
	if strings.TrimSpace(t.ID) == "" {
		return "worker"
	}
	return "worker-" + t.ID
}
