package errors

import "abi"

type (
	Error          = abi.Error
	ViolationError = abi.ViolationError
)

var (
	New           = abi.NewError
	Violation     = abi.Violation
	ErrNotFound   = abi.ErrNotFound
	ErrForbidden  = abi.ErrForbidden
	ErrConflict   = abi.ErrConflict
	ErrBadRequest = abi.ErrBadRequest
)
