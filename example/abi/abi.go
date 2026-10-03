package abi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"os"
	"strings"
	"time"
)

// ABIVersion defines the frozen ABI version.
const ABIVersion = "1.0"

// HandlerID is the globally unique identifier for a route handler.
type HandlerID string

// Transport identifies the network protocol or messaging system used by a route.
type Transport string

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

// DatagramResponse specifies the response mode for datagram-based transports.
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
type Shape interface {
	ShapeKind() ShapeKind
}

type Schema struct {
	Type        string             `json:"type,omitempty"`
	Format      string             `json:"format,omitempty"`
	Description string             `json:"description,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Enum        []any              `json:"enum,omitempty"`
	Minimum     *float64           `json:"minimum,omitempty"`
	Maximum     *float64           `json:"maximum,omitempty"`
	MinLength   *int               `json:"minLength,omitempty"`
	MaxLength   *int               `json:"maxLength,omitempty"`
	Pattern     string             `json:"pattern,omitempty"`
	Raw         map[string]any     `json:"raw,omitempty"`
}

func MustCompile(raw []byte) *Schema {
	var s Schema
	_ = json.Unmarshal(raw, &s)
	return &s
}

type RequestResponseShape struct {
	Request   *Schema         `json:"request,omitempty"`
	Responses map[int]*Schema `json:"responses,omitempty"`
}

func (RequestResponseShape) ShapeKind() ShapeKind { return ShapeKindRequestResponse }

type FramesShape struct {
	In  *Schema `json:"in,omitempty"`
	Out *Schema `json:"out,omitempty"`
}

func (FramesShape) ShapeKind() ShapeKind { return ShapeKindFrames }

type DatagramShape struct {
	Body     *Schema          `json:"body,omitempty"`
	MaxSize  int              `json:"max_size,omitempty"`
	Response DatagramResponse `json:"response,omitempty"`
}

func (DatagramShape) ShapeKind() ShapeKind { return ShapeKindDatagram }

type PubSubShape struct {
	Topics map[string]string `json:"topics,omitempty"`
	QoS    int               `json:"qos,omitempty"`
	Retain bool              `json:"retain,omitempty"`
	In     *Schema           `json:"in,omitempty"`
	Out    *Schema           `json:"out,omitempty"`
}

func (PubSubShape) ShapeKind() ShapeKind { return ShapeKindPubSub }

type StreamShape struct {
	Partitions    int     `json:"partitions,omitempty"`
	ConsumerGroup string  `json:"consumer_group,omitempty"`
	In            *Schema `json:"in,omitempty"`
	Out           *Schema `json:"out,omitempty"`
}

func (StreamShape) ShapeKind() ShapeKind { return ShapeKindStream }

type RateLimitConfig struct {
	RPS   int `json:"rps"`
	Burst int `json:"burst"`
}

type Route struct {
	ID        HandlerID        `json:"id"`
	Transport Transport        `json:"transport"`
	Method    string           `json:"method"`
	Path      string           `json:"path"`
	Auth      string           `json:"auth,omitempty"`
	Scopes    []string         `json:"scopes,omitempty"`
	RateLimit *RateLimitConfig `json:"ratelimit,omitempty"`
	Shape     Shape            `json:"shape,omitempty"`
}

type Contract struct {
	ABIVersion string  `json:"abi_version"`
	BuildID    string  `json:"build_id"`
	Routes     []Route `json:"routes"`
}

type Request struct {
	ID        string              `json:"id"`
	Transport Transport           `json:"transport"`
	Method    string              `json:"method"`
	Path      string              `json:"path"`
	RoutePath string              `json:"route_path,omitempty"`
	Params    map[string]string   `json:"params,omitempty"`
	Query     map[string][]string `json:"query,omitempty"`
	Headers   map[string][]string `json:"headers,omitempty"`
	Body      []byte              `json:"body,omitempty"`
	Identity  *Identity           `json:"identity,omitempty"`
	Topic     string              `json:"topic,omitempty"`
	Partition int                 `json:"partition,omitempty"`
	Offset    int64               `json:"offset,omitempty"`
	Timestamp int64               `json:"timestamp,omitempty"`
	Metadata  map[string]string   `json:"metadata,omitempty"`
}

func NewRequest(transport Transport, method, path string) *Request {
	return &Request{
		Transport: transport,
		Method:    method,
		Path:      path,
		Params:    make(map[string]string),
		Query:     make(map[string][]string),
		Headers:   make(map[string][]string),
		Metadata:  make(map[string]string),
		Timestamp: time.Now().UnixNano(),
	}
}

func (r *Request) Header(key string) string {
	if r.Headers == nil {
		return ""
	}
	canonical := textproto.CanonicalMIMEHeaderKey(key)
	if vals, ok := r.Headers[canonical]; ok && len(vals) > 0 {
		return vals[0]
	}
	for k, vals := range r.Headers {
		if strings.EqualFold(k, key) && len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

func (r *Request) SetHeader(key, value string) {
	if r.Headers == nil {
		r.Headers = make(map[string][]string)
	}
	r.Headers[textproto.CanonicalMIMEHeaderKey(key)] = []string{value}
}

func (r *Request) Param(name string) string {
	if r.Params == nil {
		return ""
	}
	return r.Params[name]
}

func (r *Request) QueryParam(key string) string {
	if r.Query == nil {
		return ""
	}
	if vals, ok := r.Query[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func (r *Request) QueryParams(key string) []string {
	if r.Query == nil {
		return nil
	}
	return r.Query[key]
}

func (r *Request) BindJSON(target any) error {
	if len(r.Body) == 0 {
		return nil
	}
	return json.Unmarshal(r.Body, target)
}

type Response struct {
	Status    int                 `json:"status"`
	Headers   map[string][]string `json:"headers,omitempty"`
	Body      []byte              `json:"body,omitempty"`
	Metadata  map[string]string   `json:"metadata,omitempty"`
	Timestamp int64               `json:"timestamp,omitempty"`
	Error     error               `json:"-"`
}

func NewResponse(status int, body []byte) *Response {
	return &Response{
		Status:    status,
		Headers:   make(map[string][]string),
		Body:      body,
		Metadata:  make(map[string]string),
		Timestamp: time.Now().UnixNano(),
	}
}

type DownloadResult struct {
	Source    string `json:"source"`
	Name      string `json:"name,omitempty"`
	Mime      string `json:"mime,omitempty"`
	Presigned bool   `json:"presigned,omitempty"`
}

type RawResponse struct {
	ContentType string `json:"content_type"`
	Data        []byte `json:"data"`
}

func NewAutoResponse(proto *Response, defaultStatus int, data any) (*Response, error) {
	if proto == nil {
		proto = NewResponse(defaultStatus, nil)
	}
	if proto.Status == 0 {
		proto.Status = defaultStatus
	}
	if proto.Metadata == nil {
		proto.Metadata = make(map[string]string)
	}

	if data == nil {
		return proto, nil
	}

	switch v := data.(type) {
	case DownloadResult:
		proto.Metadata["file_source"] = v.Source
		proto.Metadata["file_name"] = v.Name
		proto.Metadata["file_mime"] = v.Mime
		if v.Presigned {
			proto.Metadata["presigned"] = "true"
		}
		if v.Mime != "" && proto.Header("Content-Type") == "" {
			proto.SetHeader("Content-Type", v.Mime)
		}
		return proto, nil
	case *DownloadResult:
		if v != nil {
			proto.Metadata["file_source"] = v.Source
			proto.Metadata["file_name"] = v.Name
			proto.Metadata["file_mime"] = v.Mime
			if v.Presigned {
				proto.Metadata["presigned"] = "true"
			}
			if v.Mime != "" && proto.Header("Content-Type") == "" {
				proto.SetHeader("Content-Type", v.Mime)
			}
		}
		return proto, nil
	case RawResponse:
		proto.Body = v.Data
		if v.ContentType != "" {
			proto.SetHeader("Content-Type", v.ContentType)
		}
		return proto, nil
	case *RawResponse:
		if v != nil {
			proto.Body = v.Data
			if v.ContentType != "" {
				proto.SetHeader("Content-Type", v.ContentType)
			}
		}
		return proto, nil
	case []byte:
		proto.Body = v
		if proto.Header("Content-Type") == "" {
			proto.SetHeader("Content-Type", "application/octet-stream")
		}
		return proto, nil
	case string:
		proto.Body = []byte(v)
		if proto.Header("Content-Type") == "" {
			trimmed := strings.TrimSpace(v)
			if strings.HasPrefix(trimmed, "<") {
				proto.SetHeader("Content-Type", "text/html; charset=utf-8")
			} else {
				proto.SetHeader("Content-Type", "text/plain; charset=utf-8")
			}
		}
		return proto, nil
	default:
		b, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("marshal json response: %w", err)
		}
		proto.Body = b
		if proto.Header("Content-Type") == "" {
			proto.SetHeader("Content-Type", "application/json; charset=utf-8")
		}
		return proto, nil
	}
}

func NewJSONResponse(status int, data any) (*Response, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal json response: %w", err)
	}
	resp := NewResponse(status, b)
	resp.SetHeader("Content-Type", "application/json; charset=utf-8")
	return resp, nil
}

func NewErrorResponse(err error) *Response {
	if err == nil {
		return NewResponse(200, nil)
	}
	status := 500
	if httpErr, ok := err.(interface{ HTTPStatusCode() int }); ok {
		status = httpErr.HTTPStatusCode()
	}
	body, _ := json.Marshal(map[string]any{
		"error": err.Error(),
		"code":  "error",
	})
	resp := NewResponse(status, body)
	resp.Error = err
	resp.SetHeader("Content-Type", "application/json; charset=utf-8")
	return resp
}

func (r *Response) Header(key string) string {
	if r.Headers == nil {
		return ""
	}
	canonical := textproto.CanonicalMIMEHeaderKey(key)
	if vals, ok := r.Headers[canonical]; ok && len(vals) > 0 {
		return vals[0]
	}
	for k, vals := range r.Headers {
		if strings.EqualFold(k, key) && len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

func (r *Response) SetHeader(key, value string) {
	if r.Headers == nil {
		r.Headers = make(map[string][]string)
	}
	r.Headers[textproto.CanonicalMIMEHeaderKey(key)] = []string{value}
}

func (r *Response) AddHeader(key, value string) {
	if r.Headers == nil {
		r.Headers = make(map[string][]string)
	}
	canonical := textproto.CanonicalMIMEHeaderKey(key)
	r.Headers[canonical] = append(r.Headers[canonical], value)
}

type Hello struct {
	ABIVersion string  `json:"abi_version"`
	WorkerName string  `json:"worker_name"`
	BuildID    string  `json:"build_id"`
	PID        int     `json:"pid"`
	Routes     []Route `json:"routes"`
}

type Ready struct {
	ABIVersion string `json:"abi_version,omitempty"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
}

// Ctx is the core REST handler execution context.
type Ctx struct {
	context.Context
	req  *Request
	resp *Response
}

func NewCtx(parent context.Context, req *Request) *Ctx {
	if parent == nil {
		parent = context.Background()
	}
	return &Ctx{
		Context: parent,
		req:     req,
		resp: &Response{
			Status:  200,
			Headers: make(map[string][]string),
		},
	}
}

func (c *Ctx) Request() *Request   { return c.req }
func (c *Ctx) Response() *Response { return c.resp }
func (c *Ctx) Param(key string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Param(key)
}
func (c *Ctx) Query(key string) string {
	if c.req == nil {
		return ""
	}
	return c.req.QueryParam(key)
}
func (c *Ctx) QueryAll(key string) []string {
	if c.req == nil {
		return nil
	}
	return c.req.QueryParams(key)
}
func (c *Ctx) Header(key string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Header(key)
}
func (c *Ctx) RawBody() []byte {
	if c.req == nil {
		return nil
	}
	return c.req.Body
}
func (c *Ctx) Bind(target any) error {
	if c.req == nil {
		return nil
	}
	return c.req.BindJSON(target)
}
func (c *Ctx) Identity() *Identity {
	if c.req == nil {
		return nil
	}
	return c.req.Identity
}
func (c *Ctx) Status(code int) {
	c.resp.Status = code
}
func (c *Ctx) SetHeader(key, val string) {
	c.resp.SetHeader(key, val)
}
func (c *Ctx) HTML(html string) (string, error) {
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	return html, nil
}
func (c *Ctx) Text(text string) (string, error) {
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	return text, nil
}
func (c *Ctx) CSS(css string) (string, error) {
	c.SetHeader("Content-Type", "text/css; charset=utf-8")
	return css, nil
}
func (c *Ctx) Blob(mime string, data []byte) (any, error) {
	c.SetHeader("Content-Type", mime)
	return data, nil
}
func (c *Ctx) File(filePath string) (DownloadResult, error) {
	return DownloadResult{Source: filePath}, nil
}
func (c *Ctx) Download(filePath, filename string) (DownloadResult, error) {
	return DownloadResult{Source: filePath, Name: filename}, nil
}
func (c *Ctx) Redirect(targetURL string, statusCode int) (any, error) {
	c.Status(statusCode)
	c.SetHeader("Location", targetURL)
	return "", nil
}

// Domain contexts and results
type UploadCtx struct {
	sha256       string
	size         int64
	mime         string
	originalName string
	fields       map[string]string
	data         []byte
}

func NewUploadCtx(r io.Reader, params map[string]string, meta map[string]string) (*UploadCtx, error) {
	var data []byte
	if r != nil {
		data, _ = io.ReadAll(r)
	}
	h := sha256.Sum256(data)
	return &UploadCtx{
		sha256:       hex.EncodeToString(h[:]),
		size:         int64(len(data)),
		mime:         meta["mime"],
		originalName: meta["name"],
		fields:       params,
		data:         data,
	}, nil
}

func (c *UploadCtx) SHA256() string       { return c.sha256 }
func (c *UploadCtx) Size() int64          { return c.size }
func (c *UploadCtx) Mime() string         { return c.mime }
func (c *UploadCtx) OriginalName() string { return c.originalName }
func (c *UploadCtx) SafeName() string     { return c.originalName }
func (c *UploadCtx) Quarantine() bool     { return false }
func (c *UploadCtx) Field(name string) string {
	if c.fields == nil {
		return ""
	}
	return c.fields[name]
}
func (c *UploadCtx) Reader() io.Reader { return bytes.NewReader(c.data) }
func (c *UploadCtx) MoveTo(dest string) error {
	return os.WriteFile(dest, c.data, 0644)
}
func (c *UploadCtx) CopyTo(dest string) error {
	return os.WriteFile(dest, c.data, 0644)
}
func (c *UploadCtx) Discard() error { return nil }
func (c *UploadCtx) Dedupe() (string, bool, error) { return "", false, nil }

type UploadResult struct {
	ID   string `json:"id"   validate:"required"`
	Size int64  `json:"size" validate:"min=0"`
	Name string `json:"name" validate:"required"`
}

type DownloadCtx struct {
	id     string
	params map[string]string
}

func NewDownloadCtx(id string, params map[string]string) *DownloadCtx {
	return &DownloadCtx{id: id, params: params}
}

func (c *DownloadCtx) ID() string { return c.id }

type WsFrame struct {
	Type int
	Data []byte
}

type WsCtx struct {
	context.Context
	req *Request
}

func NewWsCtx(parent context.Context, req *Request, in, out any) *WsCtx {
	if parent == nil {
		parent = context.Background()
	}
	return &WsCtx{Context: parent, req: req}
}

func (c *WsCtx) Request() *Request { return c.req }

type VideoProfile string

const (
	ProfileHigh   VideoProfile = "high"
	ProfileMedium VideoProfile = "medium"
	ProfileLow    VideoProfile = "low"
)

type Peer struct{ ID string }
type Room struct{ ID string }
type Track struct {
	ID   string
	Kind string
}

type SessionCtx struct {
	context.Context
	req *Request
}

func NewSessionCtx(parent context.Context, req *Request, sfu any) *SessionCtx {
	if parent == nil {
		parent = context.Background()
	}
	return &SessionCtx{Context: parent, req: req}
}

func (c *SessionCtx) Request() *Request { return c.req }

type MqttCtx[In, Out any] struct {
	context.Context
	req     *Request
	Payload In
}

func NewMqttCtx[In, Out any](parent context.Context, req *Request, client any) *MqttCtx[In, Out] {
	if parent == nil {
		parent = context.Background()
	}
	var in In
	if req != nil && len(req.Body) > 0 {
		_ = req.BindJSON(&in)
	}
	return &MqttCtx[In, Out]{Context: parent, req: req, Payload: in}
}

func (c *MqttCtx[In, Out]) Request() *Request { return c.req }
func (c *MqttCtx[In, Out]) Topic() string {
	if c.req == nil {
		return ""
	}
	return c.req.Topic
}

type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("[%s] %s (HTTP %d)", e.Code, e.Message, e.StatusCode)
}

func (e *Error) HTTPStatusCode() int {
	if e.StatusCode == 0 {
		return 500
	}
	return e.StatusCode
}

func NewError(code, message string, statusCode int) *Error {
	return &Error{Code: code, Message: message, StatusCode: statusCode}
}

type ViolationError struct {
	Code     string `json:"code"`
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Got      string `json:"got"`
}

func (v *ViolationError) Error() string {
	return fmt.Sprintf("[%s] validation failure at %s: expected %s, got %s", v.Code, v.Path, v.Expected, v.Got)
}

func (v *ViolationError) HTTPStatusCode() int { return 400 }

func Violation(code, path, expected, got string) error {
	return &ViolationError{Code: code, Path: path, Expected: expected, Got: got}
}

var (
	ErrNotFound   = NewError("not_found", "resource not found", 404)
	ErrForbidden  = NewError("forbidden", "access forbidden", 403)
	ErrConflict   = NewError("conflict", "resource conflict", 409)
	ErrBadRequest = NewError("bad_request", "bad request", 400)
)
