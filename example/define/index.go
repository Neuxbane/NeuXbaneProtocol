package define

import (
	"abi/rest"
)

// WelcomeResult is returned by the root API handler.
type WelcomeResult struct {
	Message string `json:"message" validate:"required"`
	Status  string `json:"status"  validate:"required"`
}

// Handler handles GET /
func Handler(ctx *rest.Ctx) (WelcomeResult, error) {
	return WelcomeResult{
		Message: "Welcome to nxp framework!",
		Status:  "operational",
	}, nil
}
