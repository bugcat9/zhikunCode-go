package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-backend/internal/tools"
)

type ManagerConfig struct {
	ParentID     string
	Engine       QueryRunner
	Model        string
	SystemPrompt string
	NewID        func(prefix string) string
	Now          func() time.Time
}

type Manager struct {
	parentID     string
	engine       QueryRunner
	model        string
	systemPrompt string
	newID        func(prefix string) string
	now          func() time.Time

	sequence uint64
	mu       sync.Mutex
	tasks    map[string]Task
	results  map[string]AgentResult
}

type runOutcome struct {
	result AgentResult
	err    error
}

func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.Engine == nil {
		return nil, ErrNilEngine
	}

	parentID := strings.TrimSpace(cfg.ParentID)
	if parentID == "" {
		parentID = "main"
	}

	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	return &Manager{
		parentID:     parentID,
		engine:       cfg.Engine,
		model:        strings.TrimSpace(cfg.Model),
		systemPrompt: strings.TrimSpace(cfg.SystemPrompt),
		newID:        cfg.NewID,
		now:          now,
		tasks:        make(map[string]Task),
		results:      make(map[string]AgentResult),
	}, nil
}

func (m *Manager) CreateTask(ctx context.Context, req tools.TaskCreateRequest) (tools.TaskCreateResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if m == nil {
		return tools.TaskCreateResult{}, ErrInvalidAgent
	}

	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" {
		return tools.TaskCreateResult{}, fmt.Errorf("%w: instruction is required", ErrInvalidTask)
	}

	task := Task{
		ID:           m.taskID(req.TaskID),
		ParentID:     m.parentTaskID(req.ParentID),
		SessionID:    strings.TrimSpace(req.SessionID),
		Model:        m.valueOrDefault(req.Model, m.model),
		Instruction:  instruction,
		SystemPrompt: m.valueOrDefault(req.SystemPrompt, m.systemPrompt),
		Status:       TaskStatusPending,
		CreatedAt:    m.now(),
	}
	m.storeTask(task)

	agentID := m.nextID("agent")
	subAgent, err := NewAgent(agentID, m.engine)
	if err != nil {
		return tools.TaskCreateResult{}, err
	}

	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	task.Status = TaskStatusRunning
	task.StartedAt = m.now()
	m.storeTask(task)

	resultCh := make(chan runOutcome, 1)
	go func() {
		result, err := subAgent.Run(childCtx, task)
		resultCh <- runOutcome{result: result, err: err}
	}()

	select {
	case outcome := <-resultCh:
		m.storeResult(outcome.result)
		return taskCreateResultFromAgent(outcome.result), outcome.err
	case <-ctx.Done():
		cancel()
		result := AgentResult{
			TaskID:      task.ID,
			AgentID:     agentID,
			ParentID:    task.ParentID,
			SessionID:   task.SessionID,
			Status:      TaskStatusCanceled,
			Error:       ctx.Err().Error(),
			StartedAt:   task.StartedAt,
			CompletedAt: m.now(),
		}
		m.storeResult(result)
		return taskCreateResultFromAgent(result), ctx.Err()
	}
}

func (m *Manager) Task(id string) (Task, bool) {
	if m == nil {
		return Task{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	return task, ok
}

func (m *Manager) Result(id string) (AgentResult, bool) {
	if m == nil {
		return AgentResult{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	result, ok := m.results[id]
	return result, ok
}

func (m *Manager) taskID(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID != "" {
		return taskID
	}
	return m.nextID("task")
}

func (m *Manager) parentTaskID(parentID string) string {
	parentID = strings.TrimSpace(parentID)
	if parentID != "" {
		return parentID
	}
	return m.parentID
}

func (m *Manager) valueOrDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}

func (m *Manager) nextID(prefix string) string {
	if m.newID != nil {
		if id := strings.TrimSpace(m.newID(prefix)); id != "" {
			return id
		}
	}

	seq := atomic.AddUint64(&m.sequence, 1)
	return prefix + "-" + strconv.FormatInt(m.now().UnixNano(), 36) + "-" + strconv.FormatUint(seq, 36)
}

func (m *Manager) storeTask(task Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks[task.ID] = task
}

func (m *Manager) storeResult(result AgentResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task, ok := m.tasks[result.TaskID]; ok {
		task.Status = result.Status
		task.CompletedAt = result.CompletedAt
		m.tasks[result.TaskID] = task
	}
	m.results[result.TaskID] = result
}

func taskCreateResultFromAgent(result AgentResult) tools.TaskCreateResult {
	return tools.TaskCreateResult{
		TaskID:      result.TaskID,
		AgentID:     result.AgentID,
		ParentID:    result.ParentID,
		SessionID:   result.SessionID,
		Status:      string(result.Status),
		Text:        result.Text,
		Model:       result.Model,
		Usage:       result.Usage,
		Error:       result.Error,
		StartedAt:   result.StartedAt,
		CompletedAt: result.CompletedAt,
	}
}
