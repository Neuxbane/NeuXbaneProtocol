// @guard
// @transport websocket
// @shape stream
package ws

import (
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/ws"
)

// Handler streams messages down the WebSocket connection spontaneously.
func Handler(ctx *ws.Ctx) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for i := 0; i < 3; i++ {
		select {
		case <-ctx.Done():
			return nil
		case t := <-ticker.C:
			if err := ctx.Send(map[string]any{"count": i, "time": t.UnixNano()}); err != nil {
				return err
			}
		}
	}
	return nil
}

