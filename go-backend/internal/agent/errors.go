package agent

import "errors"

var (
	ErrInvalidAgent = errors.New("invalid agent")
	ErrInvalidTask  = errors.New("invalid task")
	ErrNilEngine    = errors.New("agent query engine is nil")
)
