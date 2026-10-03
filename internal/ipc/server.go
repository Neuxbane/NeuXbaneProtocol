package ipc

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

// Handler handles an incoming Frame and optionally returns a response Frame.
type Handler func(frame *Frame) (*Frame, error)

// Server accepts IPC connections and dispatches incoming frames to registered handlers.
type Server struct {
	listener net.Listener
	handlers map[FrameType]Handler
	mu       sync.RWMutex
	conns    map[net.Conn]struct{}
	connsMu  sync.Mutex
	closed   atomic.Bool
	doneChan chan struct{}
}

// NewServer initializes an IPC Server.
func NewServer() *Server {
	return &Server{
		handlers: make(map[FrameType]Handler),
		conns:    make(map[net.Conn]struct{}),
		doneChan: make(chan struct{}),
	}
}

// Handle registers a frame handler for a specific FrameType.
func (s *Server) Handle(ft FrameType, h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[ft] = h
}

// Serve starts accepting connections on the provided net.Listener.
func (s *Server) Serve(listener net.Listener) error {
	s.listener = listener
	for {
		conn, err := listener.Accept()
		if err != nil {
			if s.closed.Load() {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}

		s.trackConn(conn, true)
		go s.handleConn(conn)
	}
}

func (s *Server) trackConn(c net.Conn, add bool) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	if add {
		s.conns[c] = struct{}{}
	} else {
		delete(s.conns, c)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer func() {
		conn.Close()
		s.trackConn(conn, false)
	}()

	var writeMu sync.Mutex

	for {
		frame, err := ReadFrame(conn)
		if err != nil {
			return
		}

		s.mu.RLock()
		handler, exists := s.handlers[frame.Header.Type]
		s.mu.RUnlock()

		if exists && handler != nil {
			go func(f *Frame) {
				resp, err := handler(f)
				if err != nil {
					errResp := NewFrame(FrameTypeResponse, f.Header.ID, []byte(err.Error()))
					errResp.Header.Metadata["error"] = "true"
					writeMu.Lock()
					_ = WriteFrame(conn, errResp)
					writeMu.Unlock()
					return
				}
				if resp != nil {
					if resp.Header.ID == "" {
						resp.Header.ID = f.Header.ID
					}
					writeMu.Lock()
					_ = WriteFrame(conn, resp)
					writeMu.Unlock()
				}
			}(frame)
		}
	}
}

// Shutdown gracefully stops the listener and closes all active client connections.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.closed.CompareAndSwap(false, true) {
		close(s.doneChan)
		if s.listener != nil {
			_ = s.listener.Close()
		}
		s.connsMu.Lock()
		for c := range s.conns {
			_ = c.Close()
		}
		s.connsMu.Unlock()
	}
	return nil
}
