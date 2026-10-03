package define

import (
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/rest"
)

// RootResult is returned by the root API handler.
type RootResult struct {
	Message string `json:"message" validate:"required"`
	Status  string `json:"status" validate:"required"`
}

// Handler handles GET /
func Handler(ctx *rest.Ctx) (RootResult, error) {
	return RootResult{
		Message: "Welcome to nxp framework",
		Status:  "operational",
	}, nil
}
