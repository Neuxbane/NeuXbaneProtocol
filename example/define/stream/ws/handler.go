// @guard
// @transport websocket
// @shape stream
package ws

import (
	"time"

	"abi/ws"
)

// Handler handles live streaming over WebSocket, pushing time updates continuously.
func Handler(ctx *ws.Ctx) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case t := <-ticker.C:
			if err := ctx.Send(map[string]any{"time": t.Format(time.RFC3339)}); err != nil {
				return err
			}
		}
	}
}

