// Package errors provides the stable error taxonomy and contract violation types for nxp.
// It maps errors uniformly across transports (REST, gRPC, WebSocket, messaging).
package errors

import (
	"fmt"
	"strings"
)

// Standard error codes used across the nxp framework.
const (
	CodeForbidden   = "forbidden"
	CodeNotFound    = "not_found"
	CodeConflict    = "conflict"
	CodeTooLarge    = "payload_too_large"
	CodeUnavailable = "unavailable"
	CodeRetryable   = "retryable"
	CodeUnauthorized = "unauthorized"
	CodeBadRequest  = "bad_request"
	CodeRateLimited = "rate_limited"
	CodeInternal    = "internal"

	// Contract violation codes
	CodeContractRequestInvalid          = "contract.request.invalid"
	CodeContractResponseInvalid         = "contract.response.invalid"
	CodeContractResponseStatusNotAllowed = "contract.response.statusnotallowed"
	CodeContractWorkerUnavailable       = "contract.worker.unavailable"
	CodeContractTransportMismatch       = "contract.transport.mismatch"
	CodeContractRequestUnauthorized     = "contract.request.unauthorized"
	CodeContractRequestForbidden        = "contract.request.forbidden"
	CodeContractRequestRateLimited      = "contract.request.ratelimited"
)

// Violation represents a specific contract mismatch at a given JSON Pointer path.
type Violation struct {
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Got      string `json:"got"`
	Message  string `json:"message"`
	Code     string `json:"code"`
}

func (v Violation) String() string {
	if v.Message != "" {
		return fmt.Sprintf("%s: %s (path: %s, expected: %s, got: %s)", v.Code, v.Message, v.Path, v.Expected, v.Got)
	}
	return fmt.Sprintf("%s: at %s expected %s but got %s", v.Code, v.Path, v.Expected, v.Got)
}

// Error is the canonical structured error type for nxp.
type Error struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	StatusCode int            `json:"status_code,omitempty"`
	GRPCStatus int            `json:"grpc_status,omitempty"`
	Violations []Violation    `json:"violations,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
	cause      error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if len(e.Violations) > 0 {
		var msgs []string
		for _, v := range e.Violations {
			msgs = append(msgs, v.String())
		}
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, strings.Join(msgs, "; "))
	}
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause if present.
func (e *Error) Unwrap() error {
	return e.cause
}

// HTTPStatusCode returns the HTTP status code mapped to this error.
func (e *Error) HTTPStatusCode() int {
	if e.StatusCode > 0 {
		return e.StatusCode
	}
	return HTTPStatusFromCode(e.Code)
}

// GRPCStatusCode returns the gRPC canonical status code mapped to this error.
func (e *Error) GRPCStatusCode() int {
	if e.GRPCStatus > 0 {
		return e.GRPCStatus
	}
	return GRPCStatusFromCode(e.Code)
}

// WithCause wraps an existing error as the cause.
func (e *Error) WithCause(err error) *Error {
	cp := *e
	cp.cause = err
	return &cp
}

// WithViolation appends a contract violation.
func (e *Error) WithViolation(v Violation) *Error {
	cp := *e
	cp.Violations = append(append([]Violation(nil), e.Violations...), v)
	return &cp
}

// WithDetail adds a key-value pair to details.
func (e *Error) WithDetail(key string, val any) *Error {
	cp := *e
	cp.Details = make(map[string]any, len(e.Details)+1)
	for k, v := range e.Details {
		cp.Details[k] = v
	}
	cp.Details[key] = val
	return &cp
}

// New creates a new Error with the given code, message, and status code.
func New(code, message string, statusCode int) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		GRPCStatus: GRPCStatusFromCode(code),
	}
}

// Sentinel errors representing standard HTTP/transport error states.
var (
	ErrForbidden   = New(CodeForbidden, "forbidden", 403)
	ErrNotFound    = New(CodeNotFound, "not found", 404)
	ErrConflict    = New(CodeConflict, "conflict", 409)
	ErrTooLarge    = New(CodeTooLarge, "payload too large", 413)
	ErrUnavailable = New(CodeUnavailable, "service unavailable", 503)
	ErrRetryable   = New(CodeRetryable, "temporarily unavailable, please retry", 503)
	ErrUnauthorized = New(CodeUnauthorized, "unauthorized", 401)
	ErrBadRequest  = New(CodeBadRequest, "bad request", 400)
	ErrRateLimited = New(CodeRateLimited, "rate limit exceeded", 429)
	ErrInternal    = New(CodeInternal, "internal server error", 500)
)

// HTTPStatusFromCode returns an HTTP status code for a given error code.
func HTTPStatusFromCode(code string) int {
	switch code {
	case CodeForbidden, CodeContractRequestForbidden:
		return 403
	case CodeNotFound:
		return 404
	case CodeConflict:
		return 409
	case CodeTooLarge:
		return 413
	case CodeUnavailable, CodeContractWorkerUnavailable:
		return 503
	case CodeRetryable:
		return 503
	case CodeUnauthorized, CodeContractRequestUnauthorized:
		return 401
	case CodeBadRequest, CodeContractRequestInvalid, CodeContractTransportMismatch:
		return 400
	case CodeRateLimited, CodeContractRequestRateLimited:
		return 429
	case CodeContractResponseInvalid, CodeContractResponseStatusNotAllowed:
		return 502 // Bad Gateway when worker response violates contract
	case CodeInternal:
		return 500
	default:
		return 500
	}
}

// GRPCStatusFromCode returns canonical gRPC status codes (matching codes.Code numbers).
func GRPCStatusFromCode(code string) int {
	switch code {
	case CodeForbidden, CodeContractRequestForbidden:
		return 7 // PermissionDenied
	case CodeNotFound:
		return 5 // NotFound
	case CodeConflict:
		return 6 // AlreadyExists
	case CodeTooLarge:
		return 11 // OutOfRange / ResourceExhausted
	case CodeUnavailable, CodeContractWorkerUnavailable, CodeRetryable:
		return 14 // Unavailable
	case CodeUnauthorized, CodeContractRequestUnauthorized:
		return 16 // Unauthenticated
	case CodeBadRequest, CodeContractRequestInvalid, CodeContractTransportMismatch:
		return 3 // InvalidArgument
	case CodeRateLimited, CodeContractRequestRateLimited:
		return 8 // ResourceExhausted
	case CodeContractResponseInvalid, CodeContractResponseStatusNotAllowed:
		return 13 // Internal
	case CodeInternal:
		return 13 // Internal
	default:
		return 2 // Unknown
	}
}
