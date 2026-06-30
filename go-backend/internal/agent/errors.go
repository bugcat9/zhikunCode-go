package agent

import "errors"

var (
	ErrInvalidAgent       = errors.New("invalid agent")
	ErrInvalidTask        = errors.New("invalid task")
	ErrNilEngine          = errors.New("agent query engine is nil")
	ErrInvalidCoordinator = errors.New("invalid coordinator")
	ErrNoTasks            = errors.New("no coordinator tasks")
	ErrNoWorkerResults    = errors.New("no worker results")
	ErrNoSuccessfulWorker = errors.New("no successful worker")
)
