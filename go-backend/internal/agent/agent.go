package agent

import (
	"context"
	"fmt"
	"strings"

	"go-backend/internal/engine"
)

type QueryRunner interface {
	Query(ctx context.Context, req engine.QueryRequest) (engine.QueryResult, error)
}

type Agent struct {
	ID     string
	Engine QueryRunner
}

func NewAgent(id string, runner QueryRunner) (*Agent, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id is required", ErrInvalidAgent)
	}
	if runner == nil {
		return nil, ErrNilEngine
	}

	return &Agent{
		ID:     id,
		Engine: runner,
	}, nil
}
