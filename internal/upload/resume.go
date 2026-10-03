package upload

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// ChunkSession tracks state for a chunked upload.
type ChunkSession struct {
	Token        string
	OriginalName string
	TotalSize    int64
	ReceivedSize int64
	TempFile     *os.File
	CreatedAt    time.Time
	LastActive   time.Time
}

// SessionManager manages active chunked upload sessions.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*ChunkSession
}

var GlobalSessions = &SessionManager{
	sessions: make(map[string]*ChunkSession),
}

// CreateSession initializes a new chunked upload session and returns a unique resume token.
func (sm *SessionManager) CreateSession(originalName string, totalSize int64) (*ChunkSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var tokenBytes [16]byte
	_, _ = rand.Read(tokenBytes[:])
	token := hex.EncodeToString(tokenBytes[:])

	tmpFile, err := os.CreateTemp("", "nxp-chunk-*")
	if err != nil {
		return nil, fmt.Errorf("create chunk temp: %w", err)
	}

	session := &ChunkSession{
		Token:        token,
		OriginalName: originalName,
		TotalSize:    totalSize,
		TempFile:     tmpFile,
		CreatedAt:    time.Now(),
		LastActive:   time.Now(),
	}

	sm.sessions[token] = session
	return session, nil
}

// AppendChunk appends chunk bytes to the session temp file.
func (sm *SessionManager) AppendChunk(token string, r io.Reader) (int64, error) {
	sm.mu.RLock()
	session, ok := sm.sessions[token]
	sm.mu.RUnlock()

	if !ok {
		return 0, fmt.Errorf("chunk session %s not found or expired", token)
	}

	written, err := io.Copy(session.TempFile, r)
	if err != nil {
		return session.ReceivedSize, err
	}

	session.ReceivedSize += written
	session.LastActive = time.Now()
	return session.ReceivedSize, nil
}

// Complete closes and removes the session, returning the completed temp file path.
func (sm *SessionManager) Complete(token string) (string, error) {
	sm.mu.Lock()
	session, ok := sm.sessions[token]
	delete(sm.sessions, token)
	sm.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("session not found")
	}

	_ = session.TempFile.Close()
	return session.TempFile.Name(), nil
}
