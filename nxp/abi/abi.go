// Package abi defines the frozen Application Binary Interface and shared vocabulary for nxp.
// Every package in the framework depends on nxp/abi, while nxp/abi depends only on stdlib and nxp/errors.
package abi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
)

// ABIVersion defines the frozen ABI version.
const ABIVersion = "1.0"

// HandlerID is the globally unique identifier for a route handler.
type HandlerID string

// Transport identifies the network protocol or messaging system used by a route.
type Transport string

// Supported Transport constants.
const (
	TransportREST         Transport = "rest"
	TransportGRPC         Transport = "grpc"
	TransportGraphQL      Transport = "graphql"
	TransportWebSocket    Transport = "websocket"
	TransportSSE          Transport = "sse"
	TransportWebTransport Transport = "webtransport"
	TransportWebRTC       Transport = "webrtc"
	TransportUDP          Transport = "udp"
	TransportMQTT         Transport = "mqtt"
	TransportNATS         Transport = "nats"
	TransportAMQP         Transport = "amqp"
	TransportRedis        Transport = "redis"
	TransportKafka        Transport = "kafka"
)

// IsValid checks if the transport is one of the recognized transports.
func (t Transport) IsValid() bool {
	switch t {
	case TransportREST, TransportGRPC, TransportGraphQL, TransportWebSocket,
		TransportSSE, TransportWebTransport, TransportWebRTC, TransportUDP,
		TransportMQTT, TransportNATS, TransportAMQP, TransportRedis, TransportKafka:
		return true
	default:
		return false
	}
}

// AllTransports returns a slice of all supported Transport values.
func AllTransports() []Transport {
	return []Transport{
		TransportREST,
		TransportGRPC,
		TransportGraphQL,
		TransportWebSocket,
		TransportSSE,
		TransportWebTransport,
		TransportWebRTC,
		TransportUDP,
		TransportMQTT,
		TransportNATS,
		TransportAMQP,
		TransportRedis,
		TransportKafka,
	}
}

// DatagramResponse specifies the response mode for datagram-based transports (e.g. UDP).
type DatagramResponse string

const (
	DatagramResponseNone    DatagramResponse = "none"
	DatagramResponseEcho    DatagramResponse = "echo"
	DatagramResponseUnicast DatagramResponse = "unicast"
)

// Identity represents the authenticated caller context associated with a request.
type Identity struct {
	Subject string         `json:"sub,omitempty"`
	Issuer  string         `json:"iss,omitempty"`
	Scopes  []string       `json:"scopes,omitempty"`
	Roles   []string       `json:"roles,omitempty"`
	Claims  map[string]any `json:"claims,omitempty"`
	Raw     string         `json:"raw,omitempty"`
}

// HasScope checks whether the identity has the requested scope.
func (i *Identity) HasScope(scope string) bool {
	if i == nil {
		return false
	}
	for _, s := range i.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasRole checks whether the identity has the requested role.
func (i *Identity) HasRole(role string) bool {
	if i == nil {
		return false
	}
	for _, r := range i.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsAuthenticated reports whether this identity has a subject.
func (i *Identity) IsAuthenticated() bool {
	return i != nil && i.Subject != ""
}

// ShapeKind identifies the shape classification of a route.
type ShapeKind string

const (
	ShapeKindRequestResponse ShapeKind = "request_response"
	ShapeKindFrames          ShapeKind = "frames"
	ShapeKindDatagram        ShapeKind = "datagram"
	ShapeKindPubSub          ShapeKind = "pubsub"
	ShapeKindStream          ShapeKind = "stream"
)

// Shape describes the payload and flow topology of a route.
// Exactly five implementations exist.
type Shape interface {
	ShapeKind() ShapeKind
}

// RequestResponseShape models standard request-response RPC/REST interactions.
type RequestResponseShape struct {
	Request   *Schema         `json:"request,omitempty"`
	Responses map[int]*Schema `json:"responses,omitempty"`
}

// ShapeKind implements Shape.
func (RequestResponseShape) ShapeKind() ShapeKind { return ShapeKindRequestResponse }

// FramesShape models duplex framing interactions such as WebSockets or WebRTC data channels.
type FramesShape struct {
	In  *Schema `json:"in,omitempty"`
	Out *Schema `json:"out,omitempty"`
}

// ShapeKind implements Shape.
func (FramesShape) ShapeKind() ShapeKind { return ShapeKindFrames }

// DatagramShape models unreliable or connectionless packet exchanges (e.g. UDP).
type DatagramShape struct {
	Body     *Schema          `json:"body,omitempty"`
	MaxSize  int              `json:"max_size,omitempty"`
	Response DatagramResponse `json:"response,omitempty"`
}

// ShapeKind implements Shape.
func (DatagramShape) ShapeKind() ShapeKind { return ShapeKindDatagram }

// PubSubShape models topic-based publish/subscribe interactions (e.g. MQTT, Redis PubSub, NATS).
type PubSubShape struct {
	Topics map[string]string `json:"topics,omitempty"`
	QoS    int               `json:"qos,omitempty"`
	Retain bool              `json:"retain,omitempty"`
	In     *Schema           `json:"in,omitempty"`
	Out    *Schema           `json:"out,omitempty"`
}

// ShapeKind implements Shape.
func (PubSubShape) ShapeKind() ShapeKind { return ShapeKindPubSub }

// StreamShape models partitioned, consumer-group streaming (e.g. Kafka, Redis Streams, JetStream).
type StreamShape struct {
	Partitions    int     `json:"partitions,omitempty"`
	ConsumerGroup string  `json:"consumer_group,omitempty"`
	In            *Schema `json:"in,omitempty"`
	Out           *Schema `json:"out,omitempty"`
}

// ShapeKind implements Shape.
func (StreamShape) ShapeKind() ShapeKind { return ShapeKindStream }

// RateLimitConfig configures rate limiting parameters for a route.
type RateLimitConfig struct {
	RPS   int `json:"rps"`
	Burst int `json:"burst"`
}

// Route represents a registered handler route within the system.
type Route struct {
	ID          HandlerID         `json:"id"`
	Transport   Transport         `json:"transport"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Description string            `json:"description,omitempty"`
	Auth        string            `json:"auth,omitempty"` // "required", "optional", "public"
	Scopes      []string          `json:"scopes,omitempty"`
	Guards      []string          `json:"guards,omitempty"`
	RateLimit   *RateLimitConfig  `json:"ratelimit,omitempty"`
	Shape       Shape             `json:"shape"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type routeWire struct {
	ID          HandlerID         `json:"id"`
	Transport   Transport         `json:"transport"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Description string            `json:"description,omitempty"`
	Auth        string            `json:"auth,omitempty"`
	Scopes      []string          `json:"scopes,omitempty"`
	Guards      []string          `json:"guards,omitempty"`
	RateLimit   *RateLimitConfig  `json:"ratelimit,omitempty"`
	ShapeKind   ShapeKind         `json:"shape_kind"`
	Shape       json.RawMessage   `json:"shape"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// MarshalJSON provides stable polymorphic serialization for Route and its Shape.
func (r Route) MarshalJSON() ([]byte, error) {
	var shapeRaw []byte
	var shapeKind ShapeKind
	if r.Shape != nil {
		shapeKind = r.Shape.ShapeKind()
		var err error
		shapeRaw, err = json.Marshal(r.Shape)
		if err != nil {
			return nil, fmt.Errorf("marshal shape: %w", err)
		}
	}

	w := routeWire{
		ID:          r.ID,
		Transport:   r.Transport,
		Method:      r.Method,
		Path:        r.Path,
		Description: r.Description,
		Auth:        r.Auth,
		Scopes:      r.Scopes,
		Guards:      r.Guards,
		RateLimit:   r.RateLimit,
		ShapeKind:   shapeKind,
		Shape:       shapeRaw,
		Metadata:    r.Metadata,
	}
	return json.Marshal(w)
}

// UnmarshalJSON unpacks Route and restores the appropriate concrete Shape implementation.
func (r *Route) UnmarshalJSON(data []byte) error {
	var w routeWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}

	r.ID = w.ID
	r.Transport = w.Transport
	r.Method = w.Method
	r.Path = w.Path
	r.Description = w.Description
	r.Auth = w.Auth
	r.Scopes = w.Scopes
	r.Guards = w.Guards
	r.RateLimit = w.RateLimit
	r.Metadata = w.Metadata

	if len(w.Shape) > 0 {
		var shape Shape
		switch w.ShapeKind {
		case ShapeKindRequestResponse:
			var s RequestResponseShape
			if err := json.Unmarshal(w.Shape, &s); err != nil {
				return err
			}
			shape = s
		case ShapeKindFrames:
			var s FramesShape
			if err := json.Unmarshal(w.Shape, &s); err != nil {
				return err
			}
			shape = s
		case ShapeKindDatagram:
			var s DatagramShape
			if err := json.Unmarshal(w.Shape, &s); err != nil {
				return err
			}
			shape = s
		case ShapeKindPubSub:
			var s PubSubShape
			if err := json.Unmarshal(w.Shape, &s); err != nil {
				return err
			}
			shape = s
		case ShapeKindStream:
			var s StreamShape
			if err := json.Unmarshal(w.Shape, &s); err != nil {
				return err
			}
			shape = s
		default:
			// Fallback: try inspecting structure if ShapeKind is empty
			var probe map[string]any
			if err := json.Unmarshal(w.Shape, &probe); err == nil {
				if _, ok := probe["request"]; ok {
					var s RequestResponseShape
					_ = json.Unmarshal(w.Shape, &s)
					shape = s
				} else if _, ok := probe["response"]; ok {
					var s DatagramShape
					_ = json.Unmarshal(w.Shape, &s)
					shape = s
				} else if _, ok := probe["topics"]; ok {
					var s PubSubShape
					_ = json.Unmarshal(w.Shape, &s)
					shape = s
				} else if _, ok := probe["consumer_group"]; ok {
					var s StreamShape
					_ = json.Unmarshal(w.Shape, &s)
					shape = s
				} else {
					var s FramesShape
					_ = json.Unmarshal(w.Shape, &s)
					shape = s
				}
			}
		}
		r.Shape = shape
	}

	return nil
}

// Contract encapsulates the complete set of routes and schema definitions exposed by a worker.
type Contract struct {
	ABIVersion string             `json:"abi_version"`
	BuildID    string             `json:"build_id,omitempty"`
	Routes     []Route            `json:"routes"`
	Schemas    map[string]*Schema `json:"schemas,omitempty"`
}

// Hello is sent by a worker to the mother supervisor upon startup.
type Hello struct {
	ABIVersion string            `json:"abi_version"`
	WorkerName string            `json:"worker_name"`
	BuildID    string            `json:"build_id"`
	PID        int               `json:"pid"`
	Routes     []Route           `json:"routes"`
	Contract   *Contract         `json:"contract,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Marshal provides stable, versioned JSON marshaling for the Hello frame.
func (h *Hello) Marshal() ([]byte, error) {
	if h.ABIVersion == "" {
		h.ABIVersion = ABIVersion
	}
	return json.Marshal(h)
}

// UnmarshalHello deserializes a Hello frame and verifies ABI compatibility.
func UnmarshalHello(data []byte) (*Hello, error) {
	var h Hello
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("unmarshal hello: %w", err)
	}
	if h.ABIVersion != ABIVersion {
		return nil, errors.New(
			errors.CodeContractWorkerUnavailable,
			fmt.Sprintf("ABI version mismatch: expected %s, got %s", ABIVersion, h.ABIVersion),
			503,
		)
	}
	return &h, nil
}

// Ready is sent by the mother supervisor to a worker to signal that route swapping is complete.
type Ready struct {
	ABIVersion string `json:"abi_version"`
	Status     string `json:"status"` // "ready"
	Message    string `json:"message,omitempty"`
}

// StreamSink is an interface for pushing events or frames down an active connection.
type StreamSink interface {
	Send(data any) error
	Close() error
}

// WsFrame represents a raw WebSocket frame.
type WsFrame struct {
	Type int
	Data []byte
}

// WsCtx is the execution context for WebSocket handlers, supporting both
// synchronous frame-by-frame RPC and long-lived streaming / server-push.
type WsCtx struct {
	context.Context
	req     *Request
	inChan  <-chan []byte
	outChan chan<- []byte
	sink    StreamSink
	sendMu  sync.Mutex
}

// NewWsCtx constructs an initialized WebSocket Ctx.
func NewWsCtx(parent context.Context, req *Request, in, out any) *WsCtx {
	if parent == nil {
		parent = context.Background()
	}
	c := &WsCtx{
		Context: parent,
		req:     req,
	}
	if inCh, ok := in.(<-chan []byte); ok {
		c.inChan = inCh
	}
	if outCh, ok := out.(chan<- []byte); ok {
		c.outChan = outCh
	}
	if sink, ok := out.(StreamSink); ok {
		c.sink = sink
	}
	return c
}

// NewWsStreamCtx constructs a WebSocket Ctx backed by a StreamSink and inbound channel.
func NewWsStreamCtx(parent context.Context, req *Request, sink StreamSink, in <-chan []byte) *WsCtx {
	if parent == nil {
		parent = context.Background()
	}
	return &WsCtx{
		Context: parent,
		req:     req,
		sink:    sink,
		inChan:  in,
	}
}

// Request returns the underlying request.
func (c *WsCtx) Request() *Request { return c.req }

// Param returns a path parameter by name.
func (c *WsCtx) Param(name string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Param(name)
}

// Query returns a query parameter value.
func (c *WsCtx) Query(key string) string {
	if c.req == nil {
		return ""
	}
	return c.req.QueryParam(key)
}

// Header returns a header value.
func (c *WsCtx) Header(key string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Header(key)
}

// Identity returns the authenticated identity if present.
func (c *WsCtx) Identity() *Identity {
	if c.req == nil {
		return nil
	}
	return c.req.Identity
}

// Bind unmarshals the initial request payload into target.
func (c *WsCtx) Bind(target any) error {
	if c.req == nil {
		return nil
	}
	return c.req.BindJSON(target)
}

// Next awaits the next inbound message frame from the client.
func (c *WsCtx) Next() ([]byte, error) {
	select {
	case <-c.Done():
		return nil, c.Err()
	case msg, ok := <-c.inChan:
		if !ok {
			return nil, io.EOF
		}
		return msg, nil
	}
}

// Send serializes and pushes data to the client in a thread-safe manner.
func (c *WsCtx) Send(data any) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	select {
	case <-c.Done():
		return c.Err()
	default:
	}

	if c.sink != nil {
		return c.sink.Send(data)
	}

	var b []byte
	switch v := data.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		var err error
		b, err = json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal send data: %w", err)
		}
	}

	if c.outChan != nil {
		select {
		case <-c.Done():
			return c.Err()
		case c.outChan <- b:
			return nil
		}
	}

	return nil
}

// Write sends an outbound frame to the connected client.
func (c *WsCtx) Write(data []byte) error {
	return c.Send(data)
}

// WriteJSON serializes v as JSON and transmits it to the client.
func (c *WsCtx) WriteJSON(v any) error {
	return c.Send(v)
}

