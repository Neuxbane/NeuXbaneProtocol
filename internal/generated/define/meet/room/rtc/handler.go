// @desc Meeting room WebRTC session
// @transport webrtc
package rtc

import (
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/rtc"
)

// MeetResult represents the WebRTC session result.
type MeetResult struct {
	SessionID string `json:"session_id" validate:"required"`
	Room      string `json:"room" validate:"required"`
	Status    string `json:"status" validate:"required"`
}

// Handler handles WebRTC room sessions with transcode profiles.
func Handler(ctx *rtc.SessionCtx) (MeetResult, error) {
	roomName := ctx.Param("room")
	if roomName == "" {
		roomName = "lobby"
	}

	room := ctx.Room(roomName)
	track, _ := ctx.AcceptTrack()
	if track != nil {
		transcoded := ctx.Transcode(track, rtc.ProfileMedium)
		room.BroadcastTrack(transcoded, "peer-1")
	}

	return MeetResult{
		SessionID: "sfu-session-1",
		Room:      roomName,
		Status:    "active",
	}, nil
}
