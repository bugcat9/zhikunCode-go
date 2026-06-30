package agent

import "context"

type WorkerAgent struct {
	Agent *Agent
}

func NewWorkerAgent(id string, runner QueryRunner) (*WorkerAgent, error) {
	agent, err := NewAgent(id, runner)
	if err != nil {
		return nil, err
	}

	return &WorkerAgent{Agent: agent}, nil
}

func (w *WorkerAgent) Run(ctx context.Context, task Task) (AgentResult, error) {
	if w == nil || w.Agent == nil {
		return AgentResult{}, ErrInvalidAgent
	}
	return w.Agent.Run(ctx, task)
}
