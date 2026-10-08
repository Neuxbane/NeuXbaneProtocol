// @guard
// @desc Create a new item
package post

import (
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/rest"
)

// CreateItemPayload defines the creation request payload.
type CreateItemPayload struct {
	Name  string `json:"name" validate:"required"`
	Price int    `json:"price" validate:"min=1"`
}

// CreateItemResult defines the creation response.
type CreateItemResult struct {
	ID    string `json:"id" validate:"required"`
	Name  string `json:"name" validate:"required"`
	Price int    `json:"price" validate:"min=1"`
}

// Handler handles POST /items/create
func Handler(ctx *rest.Ctx) (CreateItemResult, error) {
	var payload CreateItemPayload
	if err := ctx.Bind(&payload); err != nil {
		return CreateItemResult{}, errors.New(errors.CodeBadRequest, "invalid payload: "+err.Error(), 400)
	}
	if payload.Name == "" {
		return CreateItemResult{}, errors.New(errors.CodeBadRequest, "name is required", 400)
	}

	return CreateItemResult{
		ID:    "item-123",
		Name:  payload.Name,
		Price: payload.Price,
	}, nil
}
