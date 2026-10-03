package rtc

import "abi"

type (
	VideoProfile = abi.VideoProfile
	Peer         = abi.Peer
	Room         = abi.Room
	Track        = abi.Track
	SessionCtx   = abi.SessionCtx
)

const (
	ProfileHigh   = abi.ProfileHigh
	ProfileMedium = abi.ProfileMedium
	ProfileLow    = abi.ProfileLow
)

var NewSessionCtx = abi.NewSessionCtx
