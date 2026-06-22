package session

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

const (
	defaultListLimit = 50
	maxListLimit     = 100
)

type cursorPayload struct {
	UpdatedAt string `json:"updatedAt"`
	ID        string `json:"id"`
}

func normalizeListLimit(limit int) int {
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}

func encodeCursor(updatedAt time.Time, id string) string {
	payload := cursorPayload{
		UpdatedAt: formatTime(updatedAt),
		ID:        id,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor(cursor string) (time.Time, string, error) {
	if cursor == "" {
		return time.Time{}, "", nil
	}

	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", ErrInvalidCursor
	}

	var payload cursorPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return time.Time{}, "", ErrInvalidCursor
	}
	if payload.UpdatedAt == "" || payload.ID == "" {
		return time.Time{}, "", ErrInvalidCursor
	}

	updatedAt, err := parseTime(payload.UpdatedAt)
	if err != nil {
		return time.Time{}, "", ErrInvalidCursor
	}
	return updatedAt, payload.ID, nil
}
