package agent

import "context"

type PermissionBridge interface {
	PrepareTask(ctx context.Context, task Task) (Task, error)
}

type NoopPermissionBridge struct{}

func (NoopPermissionBridge) PrepareTask(ctx context.Context, task Task) (Task, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return Task{}, ctx.Err()
	default:
		return task, nil
	}
}
