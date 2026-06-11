package session

import "context"

type Service interface {
	Create(ctx context.Context) (Session, error)
	Get(ctx context.Context, sessionID string) (Session, error)
	GetOrCreate(ctx context.Context, sessionID string) (Session, error)
	AppendMessage(ctx context.Context, sessionID string, message Message) error
	ListMessages(ctx context.Context, sessionID string, limit int) ([]Message, error)
}
