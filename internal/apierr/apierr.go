package apierr

import (
	"errors"
	"fmt"
)

const (
	CodeUsage     = "usage"
	CodeNotFound  = "not_found"
	CodeAuth      = "auth"
	CodeForbidden = "forbidden"
	CodeRateLimit = "rate_limit"
	CodeNetwork   = "network"
	CodeGraph     = "graph"
	CodeUnknown   = "unknown"
)

type Error struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Hint       string         `json:"hint,omitempty"`
	HTTPStatus int            `json:"http_status,omitempty"`
	Cause      error          `json:"-"`
	Meta       map[string]any `json:"meta,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func Usage(message string) *Error {
	return &Error{Code: CodeUsage, Message: message}
}

func UsageHint(message, hint string) *Error {
	return &Error{Code: CodeUsage, Message: message, Hint: hint}
}

func Auth(account string) *Error {
	return &Error{
		Code:    CodeAuth,
		Message: fmt.Sprintf("no token found for account '%s'", account),
		Hint:    fmt.Sprintf("Run: md365 auth login --account %s", account),
	}
}

func Graph(status int, message string) *Error {
	return &Error{Code: CodeGraph, Message: message, HTTPStatus: status}
}

func WrapGraph(status int, message string, cause error) *Error {
	return &Error{Code: CodeGraph, Message: message, HTTPStatus: status, Cause: cause}
}

func As(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Code: CodeUnknown, Message: err.Error(), Cause: err}
}
