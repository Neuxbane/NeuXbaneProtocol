package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport/sse"
	wsTransport "github.com/Neuxbane/NeuXbaneProtocol/internal/transport/websocket"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/ws"
)

// TestWebSocketStreamingServerPush verifies that ShapeKindStream routes trigger immediately
// on connection, pushing frames down the socket spontaneously without waiting for client requests.
func TestWebSocketStreamingServerPush(t *testing.T) {
	table := router.NewTable()
	table.Store(&router.Entry{
		Route: abi.Route{
			ID:        "connect.stream.test",
			Transport: abi.TransportWebSocket,
			Method:    "CONNECT",
			Path:      "/stream/test",
			Shape: abi.StreamShape{
				Partitions: 0,
			},
		},
		WorkerName: "worker-test",
	})

	wsAdapter := wsTransport.NewAdapter(nil)
	wsAdapter.SetStreamDispatcher(func(ctx context.Context, req *abi.Request, sink abi.StreamSink, in <-chan []byte) error {
		// Verify server can push multiple messages spontaneously
		for i := 1; i <= 3; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if err := sink.Send(map[string]any{"seq": i, "msg": "server-push"}); err != nil {
					return err
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
		return nil
	})

	server := httptest.NewServer(wsAdapter.Handler(table))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/stream/test"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v (resp: %+v)", err, resp)
	}
	defer conn.Close()

	// 1. First message must be the connected handshake frame
	var firstFrame map[string]any
	if err := conn.ReadJSON(&firstFrame); err != nil {
		t.Fatalf("failed to read handshake frame: %v", err)
	}
	if firstFrame["type"] != "connected" {
		t.Fatalf("expected connected handshake, got: %+v", firstFrame)
	}

	// 2. Read the pushed messages WITHOUT client ever sending anything
	for i := 1; i <= 3; i++ {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var pushed map[string]any
		if err := conn.ReadJSON(&pushed); err != nil {
			t.Fatalf("failed to read pushed frame #%d: %v", i, err)
		}
		if int(pushed["seq"].(float64)) != i {
			t.Errorf("expected seq %d, got %v", i, pushed["seq"])
		}
		if pushed["msg"] != "server-push" {
			t.Errorf("expected 'server-push', got %v", pushed["msg"])
		}
	}
}

// TestWebSocketBidirectionalStreaming verifies that both client-to-server and
// server-to-client duplex streaming operate concurrently without deadlock or write races.
func TestWebSocketBidirectionalStreaming(t *testing.T) {
	table := router.NewTable()
	table.Store(&router.Entry{
		Route: abi.Route{
			ID:        "connect.duplex.test",
			Transport: abi.TransportWebSocket,
			Method:    "CONNECT",
			Path:      "/duplex/test",
			Shape:     abi.StreamShape{},
		},
		WorkerName: "worker-duplex",
	})

	wsAdapter := wsTransport.NewAdapter(nil)
	wsAdapter.SetStreamDispatcher(func(ctx context.Context, req *abi.Request, sink abi.StreamSink, in <-chan []byte) error {
		wsCtx := ws.NewStreamCtx(ctx, req, sink, in)

		// Concurrent echo loop reading from client
		for {
			msg, err := wsCtx.Next()
			if err != nil {
				return nil
			}
			var received map[string]any
			if err := json.Unmarshal(msg, &received); err == nil {
				received["echo"] = true
				if err := wsCtx.Send(received); err != nil {
					return err
				}
			}
		}
	})

	server := httptest.NewServer(wsAdapter.Handler(table))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/duplex/test"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// Discard handshake frame
	var handshake map[string]any
	if err := conn.ReadJSON(&handshake); err != nil {
		t.Fatalf("read handshake failed: %v", err)
	}

	// Send 5 messages and verify echoes
	for i := 1; i <= 5; i++ {
		payload := map[string]any{"data": i}
		if err := conn.WriteJSON(payload); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var echo map[string]any
		if err := conn.ReadJSON(&echo); err != nil {
			t.Fatalf("read echo failed: %v", err)
		}
		if echo["echo"] != true || int(echo["data"].(float64)) != i {
			t.Fatalf("unexpected echo: %+v", echo)
		}
	}
}

// TestWebSocketRPCBackwardCompatibility verifies that legacy frame-by-frame RPC
// routes (ShapeKindFrames) continue to work seamlessly.
func TestWebSocketRPCBackwardCompatibility(t *testing.T) {
	table := router.NewTable()
	table.Store(&router.Entry{
		Route: abi.Route{
			ID:        "connect.rpc.test",
			Transport: abi.TransportWebSocket,
			Method:    "CONNECT",
			Path:      "/rpc/test",
			Shape: abi.FramesShape{
				In:  &abi.Schema{Type: "string"},
				Out: &abi.Schema{Type: "string"},
			},
		},
		WorkerName: "worker-rpc",
	})

	wsAdapter := wsTransport.NewAdapter(func(ctx context.Context, req *abi.Request) (*abi.Response, error) {
		return abi.NewJSONResponse(200, map[string]string{
			"status": "rpc-response",
			"body":   string(req.Body),
		})
	})

	server := httptest.NewServer(wsAdapter.Handler(table))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/rpc/test"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Discard handshake
	var handshake map[string]any
	if err := conn.ReadJSON(&handshake); err != nil {
		t.Fatalf("read handshake: %v", err)
	}

	// Client sends RPC message
	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatalf("write rpc frame: %v", err)
	}

	var rpcResp map[string]string
	if err := conn.ReadJSON(&rpcResp); err != nil {
		t.Fatalf("read rpc response: %v", err)
	}
	if rpcResp["status"] != "rpc-response" || rpcResp["body"] != "ping" {
		t.Fatalf("unexpected rpc response: %+v", rpcResp)
	}
}

// TestSSEStreamingServerPush verifies Server-Sent Events push stream.
func TestSSEStreamingServerPush(t *testing.T) {
	table := router.NewTable()
	table.Store(&router.Entry{
		Route: abi.Route{
			ID:        "get.sse.test",
			Transport: abi.TransportSSE,
			Method:    "GET",
			Path:      "/sse/events",
			Shape:     abi.StreamShape{},
		},
		WorkerName: "worker-sse",
	})

	sseAdapter := sse.NewAdapter(nil)
	sseAdapter.SetStreamDispatcher(func(ctx context.Context, req *abi.Request, sink abi.StreamSink, in <-chan []byte) error {
		for i := 1; i <= 3; i++ {
			if err := sink.Send(map[string]any{"event_num": i}); err != nil {
				return err
			}
		}
		return nil
	})

	server := httptest.NewServer(sseAdapter.Handler(table))
	defer server.Close()

	resp, err := http.Get(server.URL + "/sse/events")
	if err != nil {
		t.Fatalf("http get failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected Content-Type text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	buf := make([]byte, 1024)
	n, err := resp.Body.Read(buf)
	if err != nil && n == 0 {
		t.Fatalf("failed to read sse body: %v", err)
	}
	bodyStr := string(buf[:n])
	if !strings.Contains(bodyStr, `"event_num":1`) {
		t.Errorf("missing event 1 in body: %s", bodyStr)
	}
}

// TestConcurrentPushThreadSafety verifies that concurrent calls to ctx.Send
// do not cause race conditions or socket panics.
func TestConcurrentPushThreadSafety(t *testing.T) {
	table := router.NewTable()
	table.Store(&router.Entry{
		Route: abi.Route{
			ID:        "connect.concurrent.test",
			Transport: abi.TransportWebSocket,
			Method:    "CONNECT",
			Path:      "/concurrent/test",
			Shape:     abi.StreamShape{},
		},
		WorkerName: "worker-concurrent",
	})

	wsAdapter := wsTransport.NewAdapter(nil)
	wsAdapter.SetStreamDispatcher(func(ctx context.Context, req *abi.Request, sink abi.StreamSink, in <-chan []byte) error {
		wsCtx := ws.NewStreamCtx(ctx, req, sink, in)

		var wg sync.WaitGroup
		const numSenders = 10
		const msgsPerSender = 20

		for s := 0; s < numSenders; s++ {
			wg.Add(1)
			go func(senderID int) {
				defer wg.Done()
				for m := 0; m < msgsPerSender; m++ {
					_ = wsCtx.Send(map[string]int{"sender": senderID, "msg": m})
				}
			}(s)
		}

		wg.Wait()
		return nil
	})

	server := httptest.NewServer(wsAdapter.Handler(table))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/concurrent/test"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Discard handshake
	var handshake map[string]any
	_ = conn.ReadJSON(&handshake)

	// Read 200 messages safely
	receivedCount := 0
	for receivedCount < 200 {
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		var msg map[string]int
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		receivedCount++
	}

	if receivedCount != 200 {
		t.Errorf("expected 200 messages, received %d", receivedCount)
	}
}

