// @method POST
package order

import "abi/rest"

type CustomerInfo struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type OrderItem struct {
	SKU      string  `json:"sku" validate:"required"`
	Quantity int     `json:"quantity" validate:"min=1"`
	Price    float64 `json:"price" validate:"min=0"`
}

type CreateOrderPayload struct {
	Title    string       `json:"title" validate:"required"`
	Priority string       `json:"priority" enum:"low|medium|high" validate:"required"`
	Customer CustomerInfo `json:"customer"`
	Items    []OrderItem  `json:"items"`
	Active   bool         `json:"active"`
}

type OrderResponse struct {
	OrderID string  `json:"order_id"`
	Status  string  `json:"status"`
	Total   float64 `json:"total"`
}

func Handler(ctx *rest.Ctx) (OrderResponse, error) {
	var payload CreateOrderPayload
	if err := ctx.Bind(&payload); err != nil {
		return OrderResponse{}, err
	}
	var total float64
	for _, it := range payload.Items {
		total += float64(it.Quantity) * it.Price
	}
	return OrderResponse{
		OrderID: "ORD-999",
		Status:  "created",
		Total:   total,
	}, nil
}
