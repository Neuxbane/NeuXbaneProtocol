// Package contract answers: does an inbound request or outbound worker response satisfy the route's shape and schema contract?
package contract

import (
	"fmt"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
)

// Re-exported error codes for contract enforcement.
const (
	CodeRequestInvalid          = errors.CodeContractRequestInvalid
	CodeResponseInvalid         = errors.CodeContractResponseInvalid
	CodeResponseStatusNotAllowed = errors.CodeContractResponseStatusNotAllowed
	CodeWorkerUnavailable       = errors.CodeContractWorkerUnavailable
	CodeTransportMismatch       = errors.CodeContractTransportMismatch
	CodeRequestUnauthorized     = errors.CodeContractRequestUnauthorized
	CodeRequestForbidden        = errors.CodeContractRequestForbidden
	CodeRequestRateLimited      = errors.CodeContractRequestRateLimited
)

// Violation represents an identified mismatch against a contract specification.
type Violation = errors.Violation

// EgressPolicy defines the action taken upon detecting a response contract violation.
type EgressPolicy string

const (
	EgressPolicyWarn       EgressPolicy = "warn"
	EgressPolicyQuarantine EgressPolicy = "quarantine"
	EgressPolicyKill       EgressPolicy = "kill"
)

// HandleEgressViolation executes the configured policy when worker response fails contract validation.
func HandleEgressViolation(policy EgressPolicy, workerName string, violationErr *errors.Error) error {
	switch policy {
	case EgressPolicyWarn:
		// Log warning, return 502 or pass-through
		return violationErr
	case EgressPolicyKill:
		// In production, supervisor can terminate worker PID
		return violationErr
	case EgressPolicyQuarantine:
		fallthrough
	default:
		// Quarantine worker from serving further traffic
		return violationErr
	}
}

// NewValidationError constructs an *errors.Error with contract violations.
func NewValidationError(code, message string, status int, violations ...Violation) *errors.Error {
	err := errors.New(code, message, status)
	for _, v := range violations {
		err = err.WithViolation(v)
	}
	return err
}

// FormatViolation formats a single violation for error output.
func FormatViolation(path, expected, got, message, code string) Violation {
	if message == "" {
		message = fmt.Sprintf("expected %s, got %s", expected, got)
	}
	return Violation{
		Path:     path,
		Expected: expected,
		Got:      got,
		Message:  message,
		Code:     code,
	}
}
