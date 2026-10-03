package errors_test

import (
	"errors"
	"testing"

	nxperrs "github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
)

func TestStandardErrors(t *testing.T) {
	tests := []struct {
		err        *nxperrs.Error
		wantCode   string
		wantStatus int
		wantGRPC   int
	}{
		{nxperrs.ErrForbidden, nxperrs.CodeForbidden, 403, 7},
		{nxperrs.ErrNotFound, nxperrs.CodeNotFound, 404, 5},
		{nxperrs.ErrConflict, nxperrs.CodeConflict, 409, 6},
		{nxperrs.ErrTooLarge, nxperrs.CodeTooLarge, 413, 11},
		{nxperrs.ErrUnavailable, nxperrs.CodeUnavailable, 503, 14},
		{nxperrs.ErrRetryable, nxperrs.CodeRetryable, 503, 14},
		{nxperrs.ErrUnauthorized, nxperrs.CodeUnauthorized, 401, 16},
		{nxperrs.ErrBadRequest, nxperrs.CodeBadRequest, 400, 3},
		{nxperrs.ErrRateLimited, nxperrs.CodeRateLimited, 429, 8},
		{nxperrs.ErrInternal, nxperrs.CodeInternal, 500, 13},
	}

	for _, tt := range tests {
		t.Run(tt.wantCode, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", tt.err.Code, tt.wantCode)
			}
			if tt.err.HTTPStatusCode() != tt.wantStatus {
				t.Errorf("HTTPStatusCode = %d, want %d", tt.err.HTTPStatusCode(), tt.wantStatus)
			}
			if tt.err.GRPCStatusCode() != tt.wantGRPC {
				t.Errorf("GRPCStatusCode = %d, want %d", tt.err.GRPCStatusCode(), tt.wantGRPC)
			}
		})
	}
}

func TestViolationAndWrap(t *testing.T) {
	orig := errors.New("underlying error")
	err := nxperrs.New(nxperrs.CodeContractRequestInvalid, "validation failed", 400).
		WithCause(orig).
		WithViolation(nxperrs.Violation{
			Path:     "/user/age",
			Expected: "integer >= 0",
			Got:      "-5",
			Message:  "must be non-negative",
			Code:     nxperrs.CodeContractRequestInvalid,
		}).
		WithDetail("field", "age")

	if !errors.Is(err, orig) {
		t.Errorf("expected Unwrap to match orig")
	}

	if len(err.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(err.Violations))
	}

	str := err.Error()
	if str == "" {
		t.Errorf("expected non-empty error string")
	}

	if err.Details["field"] != "age" {
		t.Errorf("expected detail 'age', got %v", err.Details["field"])
	}
}
