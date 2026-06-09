package llm

import "fmt"

type ErrorKind string

const (
	ErrorKindConfig       ErrorKind = "config_error"
	ErrorKindUnauthorized ErrorKind = "unauthorized"
	ErrorKindRateLimited  ErrorKind = "rate_limited"
	ErrorKindTimeout      ErrorKind = "timeout"
	ErrorKindProvider     ErrorKind = "provider_error"
	ErrorKindUnexpected   ErrorKind = "unexpected_error"
)

type Error struct {
	Kind       ErrorKind
	StatusCode int
	Message    string
	Cause      error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
