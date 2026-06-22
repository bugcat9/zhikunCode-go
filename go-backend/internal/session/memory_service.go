package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
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

func (s *MemoryService) List(ctx context.Context, opts ListOptions) (ListResult, error) {
	limit := normalizeListLimit(opts.Limit)
	cursorUpdatedAt, cursorID, err := decodeCursor(opts.Cursor)
	if err != nil {
		return ListResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	summaries := make([]Summary, 0, len(s.sessions))
	for _, sess := range s.sessions {
		summaries = append(summaries, Summary{
			ID:           sess.ID,
			Title:        sess.Title,
			CreatedAt:    sess.CreatedAt,
			UpdatedAt:    sess.UpdatedAt,
			MessageCount: len(s.messages[sess.ID]),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].UpdatedAt.Equal(summaries[j].UpdatedAt) {
			return summaries[i].ID > summaries[j].ID
		}
		return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt)
	})

	if !cursorUpdatedAt.IsZero() {
		filtered := summaries[:0]
		for _, item := range summaries {
			if item.UpdatedAt.Before(cursorUpdatedAt) ||
				(item.UpdatedAt.Equal(cursorUpdatedAt) && item.ID < cursorID) {
				filtered = append(filtered, item)
			}
		}
		summaries = filtered
	}

	result := ListResult{}
	if len(summaries) > limit {
		result.HasMore = true
		summaries = summaries[:limit]
	}
	result.Sessions = append(result.Sessions, summaries...)
	if result.HasMore && len(result.Sessions) > 0 {
		last := result.Sessions[len(result.Sessions)-1]
		result.NextCursor = encodeCursor(last.UpdatedAt, last.ID)
	}
	return result, nil
}

func (s *MemoryService) Delete(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return ErrInvalidSession
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[sessionID]; !ok {
		return ErrSessionNotFound
	}
	delete(s.sessions, sessionID)
	delete(s.messages, sessionID)
	return nil
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
