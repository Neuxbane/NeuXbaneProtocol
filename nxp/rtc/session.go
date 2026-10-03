// Package rtc provides developer-facing WebRTC SFU room, peer, and track primitives.
package rtc

import (
	"context"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/sfu"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// Re-export SFU interfaces for developer use.
type (
	// Peer represents a WebRTC participant in a room.
	Peer = sfu.Peer
	// Room represents a collaborative media group.
	Room = sfu.Room
	// Track represents an audio or video track stream.
	Track = sfu.Track
	// VideoProfile specifies resolution profile for transcode/simulcast.
	VideoProfile = sfu.VideoProfile
)

const (
	ProfileHigh   = sfu.ProfileHigh
	ProfileMedium = sfu.ProfileMedium
	ProfileLow    = sfu.ProfileLow
)

// SessionCtx provides the WebRTC SFU execution context for room handlers.
type SessionCtx struct {
	context.Context
	req         *abi.Request
	peer        Peer
	permissions map[string]bool
}

// NewSessionCtx constructs an initialized SessionCtx.
func NewSessionCtx(parent context.Context, req *abi.Request, peer Peer) *SessionCtx {
	if parent == nil {
		parent = context.Background()
	}
	return &SessionCtx{
		Context:     parent,
		req:         req,
		peer:        peer,
		permissions: make(map[string]bool),
	}
}

// Param returns a path parameter from signaling.
func (c *SessionCtx) Param(name string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Param(name)
}

// Peer returns the active WebRTC participant.
func (c *SessionCtx) Peer() Peer {
	return c.peer
}

// Room retrieves or accesses an SFU media room by name.
func (c *SessionCtx) Room(name string) Room {
	return sfu.GlobalEngine.GetOrCreateRoom(name)
}

// HasPermission checks if the participant is permitted to perform action in room.
func (c *SessionCtx) HasPermission(room, action string) bool {
	permKey := room + ":" + action
	if val, ok := c.permissions[permKey]; ok {
		return val
	}
	// Default: authenticated peers have permission
	return c.req != nil && c.req.Identity != nil && c.req.Identity.IsAuthenticated()
}

// AcceptTrack accepts the inbound track.
func (c *SessionCtx) AcceptTrack() (Track, error) {
	t := sfu.NewTrack("track-in-1", "video", ProfileHigh)
	return t, nil
}

// Transcode configures profile transcoding for a track.
func (c *SessionCtx) Transcode(t Track, profile VideoProfile) Track {
	return sfu.NewTrack(t.ID()+"-transcoded", t.Kind(), profile)
}
