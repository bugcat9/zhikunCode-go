package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type MemoryService struct {
	mu       sync.Mutex
	sessions map[string]Session
	messages map[string][]Message
}

func NewMemoryService() *MemoryService {
	return &MemoryService{
		sessions: make(map[string]Session),
		messages: make(map[string][]Message),
	}
}

func (s *MemoryService) Create(ctx context.Context) (Session, error) {
	sessionID := newID()
	now := time.Now().UTC()
	sess := Session{
		ID:        sessionID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = sess
	return sess, nil
}

func (s *MemoryService) Get(ctx context.Context, sessionID string) (Session, error) {
	if sessionID == "" {
		return Session{}, ErrInvalidSession
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	return sess, nil
}

func (s *MemoryService) GetOrCreate(ctx context.Context, sessionID string) (Session, error) {
	if sessionID == "" {
		return s.Create(ctx)
	}

	sess, err := s.Get(ctx, sessionID)
	if err == nil {
		return sess, nil
	}
	if err != ErrSessionNotFound {
		return Session{}, err
	}
	return s.Create(ctx)
}

func (s *MemoryService) AppendMessage(ctx context.Context, sessionID string, message Message) error {
	if sessionID == "" {
		return ErrInvalidSession
	}

	now := time.Now().UTC()
	message.ID = newID()
	message.SessionID = sessionID
	message.CreatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	sess.UpdatedAt = now
	s.sessions[sessionID] = sess
	s.messages[sessionID] = append(s.messages[sessionID], message)
	return nil
}

func (s *MemoryService) ListMessages(ctx context.Context, sessionID string, limit int) ([]Message, error) {
	if sessionID == "" {
		return nil, ErrInvalidSession
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[sessionID]; !ok {
		return nil, ErrSessionNotFound
	}

	messages := s.messages[sessionID]
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}

	result := make([]Message, len(messages))
	copy(result, messages)
	return result, nil
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
