package session

import "errors"

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrInvalidSession  = errors.New("invalid session")
)
