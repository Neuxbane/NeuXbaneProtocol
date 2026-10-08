// Package ws provides developer-facing WebSocket context and duplex framing helpers.
package ws

import (
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

type (
	// Frame represents a raw WebSocket frame.
	Frame = abi.WsFrame

	// Ctx is the execution context for WebSocket handlers, supporting both
	// synchronous frame-by-frame RPC and long-lived streaming / server-push.
	Ctx = abi.WsCtx
)

var (
	// NewCtx constructs an initialized WebSocket Ctx.
	NewCtx = abi.NewWsCtx

	// NewStreamCtx constructs a WebSocket Ctx backed by a StreamSink and inbound channel.
	NewStreamCtx = abi.NewWsStreamCtx
)
