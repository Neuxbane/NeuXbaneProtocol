// Package sfu answers: how are WebRTC peer connections, rooms, media tracks, and SFU forwarding managed?
package sfu

import (
	"sync"
)

// VideoProfile defines video resolution and bitrate for transcoding and simulcast.
type VideoProfile string

const (
	ProfileHigh   VideoProfile = "high"   // 1080p
	ProfileMedium VideoProfile = "medium" // 720p
	ProfileLow    VideoProfile = "low"    // 360p
)

// Track represents a routed media track (audio or video).
type Track interface {
	ID() string
	Kind() string // "audio" or "video"
	Profile() VideoProfile
}

type localTrack struct {
	id      string
	kind    string
	profile VideoProfile
}

func (t *localTrack) ID() string             { return t.id }
func (t *localTrack) Kind() string           { return t.kind }
func (t *localTrack) Profile() VideoProfile { return t.profile }

// NewTrack creates a Track implementation.
func NewTrack(id, kind string, profile VideoProfile) Track {
	return &localTrack{
		id:      id,
		kind:    kind,
		profile: profile,
	}
}

// Peer represents a WebRTC participant in a Room.
type Peer interface {
	ID() string
	Tracks() []Track
	AddTrack(t Track) error
	Close() error
}

type localPeer struct {
	id     string
	tracks []Track
	mu     sync.RWMutex
}

func (p *localPeer) ID() string { return p.id }

func (p *localPeer) Tracks() []Track {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cp := make([]Track, len(p.tracks))
	copy(cp, p.tracks)
	return cp
}

func (p *localPeer) AddTrack(t Track) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tracks = append(p.tracks, t)
	return nil
}

func (p *localPeer) Close() error {
	return nil
}

// Room represents a collaborative media space with multiple peers.
type Room interface {
	Name() string
	Peers() []Peer
	AddPeer(p Peer)
	RemovePeer(peerID string)
	BroadcastTrack(t Track, senderPeerID string)
}

type localRoom struct {
	name  string
	peers map[string]Peer
	mu    sync.RWMutex
}

func (r *localRoom) Name() string { return r.name }

func (r *localRoom) Peers() []Peer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]Peer, 0, len(r.peers))
	for _, p := range r.peers {
		list = append(list, p)
	}
	return list
}

func (r *localRoom) AddPeer(p Peer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peers[p.ID()] = p
}

func (r *localRoom) RemovePeer(peerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.peers, peerID)
}

func (r *localRoom) BroadcastTrack(t Track, senderPeerID string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for id, p := range r.peers {
		if id != senderPeerID {
			_ = p.AddTrack(t)
		}
	}
}

// Engine coordinates WebRTC SFU rooms, signaling, and peer lifecycle.
type Engine struct {
	rooms map[string]Room
	mu    sync.RWMutex
}

// GlobalEngine is the singleton WebRTC SFU engine instance.
var GlobalEngine = NewEngine()

// NewEngine constructs an initialized SFU Engine.
func NewEngine() *Engine {
	return &Engine{
		rooms: make(map[string]Room),
	}
}

// GetOrCreateRoom returns or creates an SFU Room.
func (e *Engine) GetOrCreateRoom(name string) Room {
	e.mu.Lock()
	defer e.mu.Unlock()
	if r, ok := e.rooms[name]; ok {
		return r
	}
	r := &localRoom{
		name:  name,
		peers: make(map[string]Peer),
	}
	e.rooms[name] = r
	return r
}

// NewPeer constructs a Peer instance.
func (e *Engine) NewPeer(id string) Peer {
	return &localPeer{id: id}
}
