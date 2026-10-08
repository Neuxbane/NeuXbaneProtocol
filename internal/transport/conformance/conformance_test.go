package conformance_test

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport/amqp"
	_ "github.com/Neuxbane/NeuXbaneProtocol/internal/transport/grpc"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport/kafka"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport/mqtt"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport/nats"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport/redis"
	_ "github.com/Neuxbane/NeuXbaneProtocol/internal/transport/rest"
	_ "github.com/Neuxbane/NeuXbaneProtocol/internal/transport/sse"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport/udp"
	_ "github.com/Neuxbane/NeuXbaneProtocol/internal/transport/webrtc"
	_ "github.com/Neuxbane/NeuXbaneProtocol/internal/transport/websocket"
)

func TestTransportConformance(t *testing.T) {
	adapters := transport.AllAdapters()
	if len(adapters) == 0 {
		t.Fatal("no transport adapters registered")
	}

	for _, a := range adapters {
		t.Run(string(a.Name()), func(t *testing.T) {
			// Invariant 1: Name must be a valid Transport enum
			if !a.Name().IsValid() {
				t.Errorf("adapter name %s is not a valid abi.Transport", a.Name())
			}

			// Invariant 2: MatchKey must be non-empty
			if a.MatchKey() == "" {
				t.Errorf("adapter %s returned empty MatchKey", a.Name())
			}

			// Invariant 3: Normalization produces valid abi.Request
			switch a.Name() {
			case abi.TransportREST:
				raw := httptest.NewRequest("GET", "/test?q=1", bytes.NewBufferString("hello"))
				raw.Header.Set("x-custom", "value")

				req, err := a.Normalize(raw)
				if err != nil {
					t.Fatalf("normalize failed: %v", err)
				}
				if req.Transport != abi.TransportREST {
					t.Errorf("expected transport rest, got %s", req.Transport)
				}
				if req.Method != "GET" || req.Path != "/test" {
					t.Errorf("expected GET /test, got %s %s", req.Method, req.Path)
				}
				if req.Header("x-custom") != "value" {
					t.Errorf("header x-custom mismatch")
				}
				if req.QueryParam("q") != "1" {
					t.Errorf("query param q mismatch")
				}

			case abi.TransportWebSocket:
				raw := httptest.NewRequest("GET", "/ws/chat?room=main", nil)
				req, err := a.Normalize(raw)
				if err != nil {
					t.Fatalf("normalize failed: %v", err)
				}
				if req.Transport != abi.TransportWebSocket {
					t.Errorf("expected transport websocket, got %s", req.Transport)
				}
				if req.Path != "/ws/chat" {
					t.Errorf("expected /ws/chat, got %s", req.Path)
				}

			case abi.TransportSSE:
				raw := httptest.NewRequest("GET", "/events?channel=news", nil)
				req, err := a.Normalize(raw)
				if err != nil {
					t.Fatalf("normalize sse failed: %v", err)
				}
				if req.Transport != abi.TransportSSE {
					t.Errorf("expected transport sse, got %s", req.Transport)
				}
				if req.Path != "/events" {
					t.Errorf("expected /events, got %s", req.Path)
				}

			case abi.TransportGRPC:
				raw := httptest.NewRequest("POST", "/service.Service/Call", bytes.NewBuffer([]byte("\x00\x00\x00\x00\x04test")))
				req, err := a.Normalize(raw)
				if err != nil {
					t.Fatalf("normalize grpc failed: %v", err)
				}
				if req.Transport != abi.TransportGRPC {
					t.Errorf("expected transport grpc, got %s", req.Transport)
				}

			case abi.TransportUDP:
				raw := udp.RawDatagram{
					Data: []byte("ping"),
				}
				req, err := a.Normalize(raw)
				if err != nil {
					t.Fatalf("normalize udp failed: %v", err)
				}
				if req.Transport != abi.TransportUDP {
					t.Errorf("expected transport udp, got %s", req.Transport)
				}

			case abi.TransportWebRTC:
				raw := httptest.NewRequest("POST", "/room/signal", bytes.NewBuffer([]byte(`{"type":"offer"}`)))
				req, err := a.Normalize(raw)
				if err != nil {
					t.Fatalf("normalize webrtc failed: %v", err)
				}
				if req.Transport != abi.TransportWebRTC {
					t.Errorf("expected transport webrtc, got %s", req.Transport)
				}

			case abi.TransportMQTT:
				raw := mqtt.Packet{
					Topic:   "sensors/temp",
					Payload: []byte(`{"val":22.5}`),
					QoS:     1,
				}
				req, err := a.Normalize(raw)
				if err != nil {
					t.Fatalf("normalize mqtt failed: %v", err)
				}
				if req.Transport != abi.TransportMQTT || req.Topic != "sensors/temp" {
					t.Errorf("mqtt normalize mismatch: %+v", req)
				}

			case abi.TransportNATS:
				raw := nats.Msg{Subject: "orders.new", Data: []byte("payload")}
				req, err := a.Normalize(raw)
				if err != nil || req.Transport != abi.TransportNATS {
					t.Fatalf("nats normalize failed")
				}

			case abi.TransportAMQP:
				raw := amqp.Delivery{Exchange: "ex", RoutingKey: "key", Body: []byte("payload")}
				req, err := a.Normalize(raw)
				if err != nil || req.Transport != abi.TransportAMQP {
					t.Fatalf("amqp normalize failed")
				}

			case abi.TransportRedis:
				raw := redis.Message{Channel: "chat", Payload: []byte("payload")}
				req, err := a.Normalize(raw)
				if err != nil || req.Transport != abi.TransportRedis {
					t.Fatalf("redis normalize failed")
				}

			case abi.TransportKafka:
				raw := kafka.Record{Topic: "events", Partition: 0, Value: []byte("payload")}
				req, err := a.Normalize(raw)
				if err != nil || req.Transport != abi.TransportKafka {
					t.Fatalf("kafka normalize failed")
				}
			}

			// Invariant 4: Rendering produces non-empty output
			var buf bytes.Buffer
			resp := abi.NewResponse(200, []byte("conformance response"))
			if err := a.Render(&buf, resp, transport.RenderCtx{}); err != nil {
				t.Fatalf("render failed: %v", err)
			}
			if buf.Len() == 0 {
				t.Errorf("rendered output was empty")
			}
		})
	}
}
