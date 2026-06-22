package session

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"go-backend/internal/llm"
)

type SQLiteService struct {
	db *sql.DB
}

func NewSQLiteService(db *sql.DB) *SQLiteService {
	return &SQLiteService{db: db}
}

func (s *SQLiteService) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteService) Create(ctx context.Context) (Session, error) {
	now := time.Now().UTC()
	sess := Session{
		ID:        newID(),
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO sessions (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		sess.ID,
		sess.Title,
		formatTime(sess.CreatedAt),
		formatTime(sess.UpdatedAt),
	)
	if err != nil {
		return Session{}, err
	}
	return sess, nil
}

func (s *SQLiteService) Get(ctx context.Context, sessionID string) (Session, error) {
	if sessionID == "" {
		return Session{}, ErrInvalidSession
	}

	var sess Session
	var createdAt string
	var updatedAt string
	var title sql.NullString
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, title, created_at, updated_at FROM sessions WHERE id = ?`,
		sessionID,
	).Scan(&sess.ID, &title, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, err
	}

	if title.Valid {
		sess.Title = title.String
	}
	if sess.CreatedAt, err = parseTime(createdAt); err != nil {
		return Session{}, err
	}
	if sess.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return Session{}, err
	}
	return sess, nil
}

func (s *SQLiteService) GetOrCreate(ctx context.Context, sessionID string) (Session, error) {
	if sessionID == "" {
		return s.Create(ctx)
	}

	sess, err := s.Get(ctx, sessionID)
	if err == nil {
		return sess, nil
	}
	if !errors.Is(err, ErrSessionNotFound) {
		return Session{}, err
	}
	return s.Create(ctx)
}

func (s *SQLiteService) List(ctx context.Context, opts ListOptions) (ListResult, error) {
	limit := normalizeListLimit(opts.Limit)
	cursorUpdatedAt, cursorID, err := decodeCursor(opts.Cursor)
	if err != nil {
		return ListResult{}, err
	}

	query := `
SELECT s.id, s.title, s.created_at, s.updated_at, COUNT(m.id) AS message_count
FROM sessions s
LEFT JOIN messages m ON m.session_id = s.id`
	args := []any{}
	if !cursorUpdatedAt.IsZero() {
		query += `
WHERE s.updated_at < ? OR (s.updated_at = ? AND s.id < ?)`
		formatted := formatTime(cursorUpdatedAt)
		args = append(args, formatted, formatted, cursorID)
	}
	query += `
GROUP BY s.id, s.title, s.created_at, s.updated_at
ORDER BY s.updated_at DESC, s.id DESC
LIMIT ?`
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()

	summaries := make([]Summary, 0, limit+1)
	for rows.Next() {
		var summary Summary
		var title sql.NullString
		var createdAt string
		var updatedAt string
		if err := rows.Scan(&summary.ID, &title, &createdAt, &updatedAt, &summary.MessageCount); err != nil {
			return ListResult{}, err
		}
		if title.Valid {
			summary.Title = title.String
		}
		if summary.CreatedAt, err = parseTime(createdAt); err != nil {
			return ListResult{}, err
		}
		if summary.UpdatedAt, err = parseTime(updatedAt); err != nil {
			return ListResult{}, err
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}

	result := ListResult{}
	if len(summaries) > limit {
		result.HasMore = true
		summaries = summaries[:limit]
	}
	result.Sessions = summaries
	if result.HasMore && len(result.Sessions) > 0 {
		last := result.Sessions[len(result.Sessions)-1]
		result.NextCursor = encodeCursor(last.UpdatedAt, last.ID)
	}
	return result, nil
}

func (s *SQLiteService) Delete(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return ErrInvalidSession
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE session_id = ?`, sessionID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrSessionNotFound
	}
	return tx.Commit()
}

func (s *SQLiteService) AppendMessage(ctx context.Context, sessionID string, message Message) error {
	if sessionID == "" {
		return ErrInvalidSession
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	result, err := tx.ExecContext(
		ctx,
		`UPDATE sessions SET updated_at = ? WHERE id = ?`,
		formatTime(now),
		sessionID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrSessionNotFound
	}

	if message.ID == "" {
		message.ID = newID()
	}
	message.SessionID = sessionID
	message.CreatedAt = now

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)`,
		message.ID,
		message.SessionID,
		string(message.Role),
		message.Content,
		formatTime(message.CreatedAt),
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteService) ListMessages(ctx context.Context, sessionID string, limit int) ([]Message, error) {
	if sessionID == "" {
		return nil, ErrInvalidSession
	}

	if _, err := s.Get(ctx, sessionID); err != nil {
		return nil, err
	}

	query := `
SELECT id, session_id, role, content, created_at
FROM messages
WHERE session_id = ?
ORDER BY created_at DESC`
	args := []any{sessionID}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reversed []Message
	for rows.Next() {
		var message Message
		var role string
		var createdAt string
		if err := rows.Scan(&message.ID, &message.SessionID, &role, &message.Content, &createdAt); err != nil {
			return nil, err
		}
		message.Role = llm.Role(role)
		message.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		reversed = append(reversed, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	messages := make([]Message, len(reversed))
	for i := range reversed {
		messages[len(reversed)-1-i] = reversed[i]
	}
	return messages, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}
