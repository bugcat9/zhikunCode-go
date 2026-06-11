package session

import (
	"time"

	"go-backend/internal/llm"
)

type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID        string    `json:"id,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	Role      llm.Role  `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}
