// @desc Get a single item by ID
package get

import (
	"fmt"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/rest"
)

// ItemResult describes a single inventory item.
type ItemResult struct {
	ID    string `json:"id" validate:"required"`
	Name  string `json:"name" validate:"required"`
	Price int    `json:"price" validate:"min=1"`
}

// Handler handles GET /items/{id}
func Handler(ctx *rest.Ctx) (ItemResult, error) {
	id := ctx.Param("id")
	if id == "" {
		return ItemResult{}, errors.New(errors.CodeBadRequest, "missing id parameter", 400)
	}
	if id == "notfound" {
		return ItemResult{}, errors.New(errors.CodeNotFound, fmt.Sprintf("item %s not found", id), 404)
	}
	return ItemResult{
		ID:    id,
		Name:  "Test Item " + id,
		Price: 100,
	}, nil
}
