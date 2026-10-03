package ipc

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Client manages a duplex IPC connection to a remote mother or worker process.
type Client struct {
	conn      net.Conn
	mu        sync.Mutex
	writeMu   sync.Mutex
	pending   map[string]chan *Frame
	pendingMu sync.RWMutex
	closed    atomic.Bool
	closeChan chan struct{}
}

// Dial connects to a Unix or network address and returns an initialized Client.
func Dial(network, address string) (*Client, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, fmt.Errorf("ipc dial %s %s: %w", network, address, err)
	}
	return NewClient(conn), nil
}

// DialTimeout connects to a Unix or network address with timeout.
func DialTimeout(network, address string, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, fmt.Errorf("ipc dial timeout %s %s: %w", network, address, err)
	}
	return NewClient(conn), nil
}

// NewClient wraps an existing net.Conn in an IPC Client.
func NewClient(conn net.Conn) *Client {
	c := &Client{
		conn:      conn,
		pending:   make(map[string]chan *Frame),
		closeChan: make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// Send transmits a single frame over the connection.
func (c *Client) Send(frame *Frame) error {
	if c.closed.Load() {
		return fmt.Errorf("ipc client closed")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return WriteFrame(c.conn, frame)
}

// RoundTrip sends a request frame and awaits the matching response frame correlated by ID.
func (c *Client) RoundTrip(ctx context.Context, req *Frame) (*Frame, error) {
	if c.closed.Load() {
		return nil, fmt.Errorf("ipc client closed")
	}

	replyChan := make(chan *Frame, 1)

	c.pendingMu.Lock()
	c.pending[req.Header.ID] = replyChan
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, req.Header.ID)
		c.pendingMu.Unlock()
	}()

	if err := c.Send(req); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closeChan:
		return nil, fmt.Errorf("ipc connection closed while awaiting response")
	case resp := <-replyChan:
		return resp, nil
	}
}

func (c *Client) readLoop() {
	defer func() {
		c.Close()
	}()

	for {
		frame, err := ReadFrame(c.conn)
		if err != nil {
			return
		}

		// Correlate with pending request if any
		c.pendingMu.RLock()
		ch, exists := c.pending[frame.Header.ID]
		c.pendingMu.RUnlock()

		if exists && ch != nil {
			select {
			case ch <- frame:
			default:
			}
		}
	}
}

// Close closes the connection and cancels pending roundtrips.
func (c *Client) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		close(c.closeChan)
		return c.conn.Close()
	}
	return nil
}
