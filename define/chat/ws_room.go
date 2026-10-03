package chat

import (
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/ws"
)

// RoomResult is the response after room participation.
type RoomResult struct {
	Status string `json:"status" validate:"required"`
	Room   string `json:"room" validate:"required"`
}

// Handler handles WebSocket connection at /chat/room
func Handler(ctx *ws.Ctx) (RoomResult, error) {
	room := ctx.Param("room")
	if room == "" {
		room = "general"
	}
	_ = ctx.WriteJSON(map[string]string{
		"event": "joined",
		"room":  room,
	})
	return RoomResult{
		Status: "connected",
		Room:   room,
	}, nil
}
