package ws

import "abi"

type (
	Frame = abi.WsFrame
	Ctx   = abi.WsCtx
)

var (
	NewCtx       = abi.NewWsCtx
	NewStreamCtx = abi.NewWsStreamCtx
)
