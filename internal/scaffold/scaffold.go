package scaffold

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnsureAll scaffolds the entire developer workspace: define/, abi/ packages, go.mod, and README.md.
func EnsureAll(workDir, defineDir string) error {
	if workDir == "" {
		workDir = "."
	}
	if defineDir == "" {
		defineDir = "define"
	}

	if err := EnsureABI(workDir); err != nil {
		return err
	}
	if err := EnsureGoMod(workDir); err != nil {
		return err
	}
	if err := EnsureDefine(defineDir); err != nil {
		return err
	}
	if err := EnsureREADME(workDir); err != nil {
		return err
	}

	// Run go mod tidy to ensure local modules and dependencies resolve cleanly
	abiDir := filepath.Join(workDir, "abi")
	if _, err := os.Stat(filepath.Join(abiDir, "go.mod")); err == nil {
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = abiDir
		_ = cmd.Run()
	}
	if _, err := os.Stat(filepath.Join(workDir, "go.mod")); err == nil {
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = workDir
		_ = cmd.Run()
	}

	return nil
}

// EnsureDefine ensures the define/ directory exists along with developer instructions and starter index.go.
func EnsureDefine(defineDir string) error {
	if defineDir == "" {
		defineDir = "define"
	}

	info, err := os.Stat(defineDir)
	if os.IsNotExist(err) || (err == nil && !info.IsDir()) {
		if err := os.MkdirAll(defineDir, 0755); err != nil {
			return fmt.Errorf("create define directory %q: %w", defineDir, err)
		}
	}

	// 1. Ensure HOW_TO_DEFINE.md
	howToPath := filepath.Join(defineDir, "HOW_TO_DEFINE.md")
	if _, err := os.Stat(howToPath); os.IsNotExist(err) {
		_ = os.WriteFile(howToPath, []byte(howToDefineDoc), 0644)
	}

	// 2. Ensure ABI.md
	abiPath := filepath.Join(defineDir, "ABI.md")
	if _, err := os.Stat(abiPath); os.IsNotExist(err) {
		_ = os.WriteFile(abiPath, []byte(abiDoc), 0644)
	}

	// 3. Ensure starter index.go if define/ is empty
	entries, _ := os.ReadDir(defineDir)
	hasGoFiles := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".go") {
			hasGoFiles = true
			break
		}
	}

	if !hasGoFiles {
		indexPath := filepath.Join(defineDir, "index.go")
		_ = os.WriteFile(indexPath, []byte(starterIndexGo), 0644)
	}

	return nil
}

// EnsureABI creates the 100% self-contained local developer-facing abi/ packages.
// Everything is generated locally from stdlib without requiring external downloads or repositories.
func EnsureABI(workDir string) error {
	abiDir := filepath.Join(workDir, "abi")

	// 1. abi/go.mod - Zero external dependencies!
	abiModPath := filepath.Join(abiDir, "go.mod")
	if _, err := os.Stat(abiModPath); os.IsNotExist(err) {
		_ = os.MkdirAll(abiDir, 0755)
		modContent := "module abi\n\ngo 1.22\n"
		_ = os.WriteFile(abiModPath, []byte(modContent), 0644)
	}

	// 2. abi/abi.go (core unified types and contexts)
	_ = os.WriteFile(filepath.Join(abiDir, "abi.go"), []byte(abiCoreGo), 0644)

	// 3. abi/rest/ctx.go
	restDir := filepath.Join(abiDir, "rest")
	_ = os.MkdirAll(restDir, 0755)
	_ = os.WriteFile(filepath.Join(restDir, "ctx.go"), []byte(abiRestGo), 0644)

	// 4. abi/files/files.go
	filesDir := filepath.Join(abiDir, "files")
	_ = os.MkdirAll(filesDir, 0755)
	_ = os.WriteFile(filepath.Join(filesDir, "files.go"), []byte(abiFilesGo), 0644)

	// 5. abi/ws/ctx.go
	wsDir := filepath.Join(abiDir, "ws")
	_ = os.MkdirAll(wsDir, 0755)
	_ = os.WriteFile(filepath.Join(wsDir, "ctx.go"), []byte(abiWsGo), 0644)

	// 6. abi/rtc/session.go
	rtcDir := filepath.Join(abiDir, "rtc")
	_ = os.MkdirAll(rtcDir, 0755)
	_ = os.WriteFile(filepath.Join(rtcDir, "session.go"), []byte(abiRtcGo), 0644)

	// 7. abi/mqtt/ctx.go
	mqttDir := filepath.Join(abiDir, "mqtt")
	_ = os.MkdirAll(mqttDir, 0755)
	_ = os.WriteFile(filepath.Join(mqttDir, "ctx.go"), []byte(abiMqttGo), 0644)

	// 8. abi/errors/errors.go
	errDir := filepath.Join(abiDir, "errors")
	_ = os.MkdirAll(errDir, 0755)
	_ = os.WriteFile(filepath.Join(errDir, "errors.go"), []byte(abiErrorsGo), 0644)

	// 9. abi/worker/worker.go (self-contained worker IPC runtime)
	workerDir := filepath.Join(abiDir, "worker")
	_ = os.MkdirAll(workerDir, 0755)
	_ = os.WriteFile(filepath.Join(workerDir, "worker.go"), []byte(abiWorkerGo), 0644)

	return nil
}

// EnsureGoMod creates go.mod in workDir if not present, configured with local abi replacement.
func EnsureGoMod(workDir string) error {
	modFile := filepath.Join(workDir, "go.mod")
	if _, err := os.Stat(modFile); err == nil {
		return nil // already exists
	}

	modName := filepath.Base(workDir)
	if modName == "." || modName == "/" || modName == "" {
		modName = "app"
	}

	content := fmt.Sprintf(`module %s

go 1.22

require abi v0.0.0

replace abi => ./abi
`, modName)

	return os.WriteFile(modFile, []byte(content), 0644)
}

// EnsureREADME generates README.md in workDir if not present.
func EnsureREADME(workDir string) error {
	readmePath := filepath.Join(workDir, "README.md")
	if _, err := os.Stat(readmePath); err == nil {
		return nil // already exists
	}
	return os.WriteFile(readmePath, []byte(readmeDoc), 0644)
}

// DetectModulePath discovers the Go module path for importing packages under defineDir.
func DetectModulePath(defineDir string) string {
	curr, err := os.Getwd()
	if err != nil {
		curr = "."
	}

	if modName := parseModuleFromDir(curr); modName != "" {
		return modName
	}

	dir, err := filepath.Abs(defineDir)
	if err != nil {
		dir = defineDir
	}

	for i := 0; i < 5; i++ {
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		if modName := parseModuleFromDir(parent); modName != "" {
			rel, err := filepath.Rel(parent, curr)
			if err == nil && rel != "." && rel != "" {
				return modName + "/" + filepath.ToSlash(rel)
			}
			return modName
		}
		dir = parent
	}

	return "app"
}

func parseModuleFromDir(dir string) string {
	modFile := filepath.Join(dir, "go.mod")
	f, err := os.Open(modFile)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

const starterIndexGo = `package define

import (
	"abi/rest"
)

// WelcomeResult is returned by the root API handler.
type WelcomeResult struct {
	Message string ` + "`json:\"message\" validate:\"required\"`" + `
	Status  string ` + "`json:\"status\"  validate:\"required\"`" + `
}

// Handler handles GET /
func Handler(ctx *rest.Ctx) (WelcomeResult, error) {
	return WelcomeResult{
		Message: "Welcome to nxp framework!",
		Status:  "operational",
	}, nil
}
`

const abiCoreGo = `package abi

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
	Subject string         ` + "`json:\"sub,omitempty\"`" + `
	Issuer  string         ` + "`json:\"iss,omitempty\"`" + `
	Scopes  []string       ` + "`json:\"scopes,omitempty\"`" + `
	Roles   []string       ` + "`json:\"roles,omitempty\"`" + `
	Claims  map[string]any ` + "`json:\"claims,omitempty\"`" + `
	Raw     string         ` + "`json:\"raw,omitempty\"`" + `
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
	Type        string             ` + "`json:\"type,omitempty\"`" + `
	Format      string             ` + "`json:\"format,omitempty\"`" + `
	Description string             ` + "`json:\"description,omitempty\"`" + `
	Properties  map[string]*Schema ` + "`json:\"properties,omitempty\"`" + `
	Required    []string           ` + "`json:\"required,omitempty\"`" + `
	Items       *Schema            ` + "`json:\"items,omitempty\"`" + `
	Enum        []any              ` + "`json:\"enum,omitempty\"`" + `
	Minimum     *float64           ` + "`json:\"minimum,omitempty\"`" + `
	Maximum     *float64           ` + "`json:\"maximum,omitempty\"`" + `
	MinLength   *int               ` + "`json:\"minLength,omitempty\"`" + `
	MaxLength   *int               ` + "`json:\"maxLength,omitempty\"`" + `
	Pattern     string             ` + "`json:\"pattern,omitempty\"`" + `
	Raw         map[string]any     ` + "`json:\"raw,omitempty\"`" + `
}

func MustCompile(raw []byte) *Schema {
	var s Schema
	_ = json.Unmarshal(raw, &s)
	return &s
}

type RequestResponseShape struct {
	Request   *Schema         ` + "`json:\"request,omitempty\"`" + `
	Responses map[int]*Schema ` + "`json:\"responses,omitempty\"`" + `
}

func (RequestResponseShape) ShapeKind() ShapeKind { return ShapeKindRequestResponse }

type FramesShape struct {
	In  *Schema ` + "`json:\"in,omitempty\"`" + `
	Out *Schema ` + "`json:\"out,omitempty\"`" + `
}

func (FramesShape) ShapeKind() ShapeKind { return ShapeKindFrames }

type DatagramShape struct {
	Body     *Schema          ` + "`json:\"body,omitempty\"`" + `
	MaxSize  int              ` + "`json:\"max_size,omitempty\"`" + `
	Response DatagramResponse ` + "`json:\"response,omitempty\"`" + `
}

func (DatagramShape) ShapeKind() ShapeKind { return ShapeKindDatagram }

type PubSubShape struct {
	Topics map[string]string ` + "`json:\"topics,omitempty\"`" + `
	QoS    int               ` + "`json:\"qos,omitempty\"`" + `
	Retain bool              ` + "`json:\"retain,omitempty\"`" + `
	In     *Schema           ` + "`json:\"in,omitempty\"`" + `
	Out    *Schema           ` + "`json:\"out,omitempty\"`" + `
}

func (PubSubShape) ShapeKind() ShapeKind { return ShapeKindPubSub }

type StreamShape struct {
	Partitions    int     ` + "`json:\"partitions,omitempty\"`" + `
	ConsumerGroup string  ` + "`json:\"consumer_group,omitempty\"`" + `
	In            *Schema ` + "`json:\"in,omitempty\"`" + `
	Out           *Schema ` + "`json:\"out,omitempty\"`" + `
}

func (StreamShape) ShapeKind() ShapeKind { return ShapeKindStream }

type RateLimitConfig struct {
	RPS   int ` + "`json:\"rps\"`" + `
	Burst int ` + "`json:\"burst\"`" + `
}

type Route struct {
	ID        HandlerID        ` + "`json:\"id\"`" + `
	Transport Transport        ` + "`json:\"transport\"`" + `
	Method    string           ` + "`json:\"method\"`" + `
	Path      string           ` + "`json:\"path\"`" + `
	Auth      string           ` + "`json:\"auth,omitempty\"`" + `
	Scopes    []string         ` + "`json:\"scopes,omitempty\"`" + `
	RateLimit *RateLimitConfig ` + "`json:\"ratelimit,omitempty\"`" + `
	Shape     Shape            ` + "`json:\"shape,omitempty\"`" + `
}

type Contract struct {
	ABIVersion string  ` + "`json:\"abi_version\"`" + `
	BuildID    string  ` + "`json:\"build_id\"`" + `
	Routes     []Route ` + "`json:\"routes\"`" + `
}

type Request struct {
	ID        string              ` + "`json:\"id\"`" + `
	Transport Transport           ` + "`json:\"transport\"`" + `
	Method    string              ` + "`json:\"method\"`" + `
	Path      string              ` + "`json:\"path\"`" + `
	RoutePath string              ` + "`json:\"route_path,omitempty\"`" + `
	Params    map[string]string   ` + "`json:\"params,omitempty\"`" + `
	Query     map[string][]string ` + "`json:\"query,omitempty\"`" + `
	Headers   map[string][]string ` + "`json:\"headers,omitempty\"`" + `
	Body      []byte              ` + "`json:\"body,omitempty\"`" + `
	Identity  *Identity           ` + "`json:\"identity,omitempty\"`" + `
	Topic     string              ` + "`json:\"topic,omitempty\"`" + `
	Partition int                 ` + "`json:\"partition,omitempty\"`" + `
	Offset    int64               ` + "`json:\"offset,omitempty\"`" + `
	Timestamp int64               ` + "`json:\"timestamp,omitempty\"`" + `
	Metadata  map[string]string   ` + "`json:\"metadata,omitempty\"`" + `
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
	Status    int                 ` + "`json:\"status\"`" + `
	Headers   map[string][]string ` + "`json:\"headers,omitempty\"`" + `
	Body      []byte              ` + "`json:\"body,omitempty\"`" + `
	Metadata  map[string]string   ` + "`json:\"metadata,omitempty\"`" + `
	Timestamp int64               ` + "`json:\"timestamp,omitempty\"`" + `
	Error     error               ` + "`json:\"-\"`" + `
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
	Source    string ` + "`json:\"source\"`" + `
	Name      string ` + "`json:\"name,omitempty\"`" + `
	Mime      string ` + "`json:\"mime,omitempty\"`" + `
	Presigned bool   ` + "`json:\"presigned,omitempty\"`" + `
}

type RawResponse struct {
	ContentType string ` + "`json:\"content_type\"`" + `
	Data        []byte ` + "`json:\"data\"`" + `
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
	ABIVersion string  ` + "`json:\"abi_version\"`" + `
	WorkerName string  ` + "`json:\"worker_name\"`" + `
	BuildID    string  ` + "`json:\"build_id\"`" + `
	PID        int     ` + "`json:\"pid\"`" + `
	Routes     []Route ` + "`json:\"routes\"`" + `
}

type Ready struct {
	ABIVersion string ` + "`json:\"abi_version,omitempty\"`" + `
	Status     string ` + "`json:\"status\"`" + `
	Message    string ` + "`json:\"message,omitempty\"`" + `
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
	ID   string ` + "`json:\"id\"   validate:\"required\"`" + `
	Size int64  ` + "`json:\"size\" validate:\"min=0\"`" + `
	Name string ` + "`json:\"name\" validate:\"required\"`" + `
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
	Code       string ` + "`json:\"code\"`" + `
	Message    string ` + "`json:\"message\"`" + `
	StatusCode int    ` + "`json:\"status_code\"`" + `
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
	Code     string ` + "`json:\"code\"`" + `
	Path     string ` + "`json:\"path\"`" + `
	Expected string ` + "`json:\"expected\"`" + `
	Got      string ` + "`json:\"got\"`" + `
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
`

const abiRestGo = `package rest

import "abi"

// Ctx is the execution context for REST handlers.
type Ctx = abi.Ctx

var NewCtx = abi.NewCtx
`

const abiFilesGo = `package files

import "abi"

type (
	UploadCtx      = abi.UploadCtx
	DownloadCtx    = abi.DownloadCtx
	UploadResult   = abi.UploadResult
	DownloadResult = abi.DownloadResult
)

var (
	NewUploadCtx   = abi.NewUploadCtx
	NewDownloadCtx = abi.NewDownloadCtx
)
`

const abiWsGo = `package ws

import "abi"

type (
	Frame = abi.WsFrame
	Ctx   = abi.WsCtx
)

var NewCtx = abi.NewWsCtx
`

const abiRtcGo = `package rtc

import "abi"

type (
	VideoProfile = abi.VideoProfile
	Peer         = abi.Peer
	Room         = abi.Room
	Track        = abi.Track
	SessionCtx   = abi.SessionCtx
)

const (
	ProfileHigh   = abi.ProfileHigh
	ProfileMedium = abi.ProfileMedium
	ProfileLow    = abi.ProfileLow
)

var NewSessionCtx = abi.NewSessionCtx
`

const abiMqttGo = `package mqtt

import (
	"context"

	"abi"
)

type Ctx[In, Out any] struct {
	context.Context
	req     *abi.Request
	Payload In
}

func NewCtx[In, Out any](parent context.Context, req *abi.Request, client any) *Ctx[In, Out] {
	if parent == nil {
		parent = context.Background()
	}
	var in In
	if req != nil && len(req.Body) > 0 {
		_ = req.BindJSON(&in)
	}
	return &Ctx[In, Out]{Context: parent, req: req, Payload: in}
}

func (c *Ctx[In, Out]) Request() *abi.Request { return c.req }
func (c *Ctx[In, Out]) Topic() string {
	if c.req == nil {
		return ""
	}
	return c.req.Topic
}
`

const abiErrorsGo = `package errors

import "abi"

type (
	Error          = abi.Error
	ViolationError = abi.ViolationError
)

var (
	New           = abi.NewError
	Violation     = abi.Violation
	ErrNotFound   = abi.ErrNotFound
	ErrForbidden  = abi.ErrForbidden
	ErrConflict   = abi.ErrConflict
	ErrBadRequest = abi.ErrBadRequest
)
`

const abiWorkerGo = `package worker

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"abi"
)

// HandlerFunc is the internal dispatch adapter for a business logic route handler.
type HandlerFunc func(*abi.Request) (*abi.Response, error)

type frameHeader struct {
	Type       string            ` + "`json:\"type\"`" + `
	ID         string            ` + "`json:\"id\"`" + `
	BodyLength int               ` + "`json:\"body_length\"`" + `
	Metadata   map[string]string ` + "`json:\"metadata,omitempty\"`" + `
}

type frame struct {
	Header frameHeader
	Body   []byte
}

func readFrame(r io.Reader) (*frame, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	headerLen := binary.BigEndian.Uint32(lenBuf[:])
	if headerLen == 0 || headerLen > 16*1024*1024 {
		return nil, fmt.Errorf("invalid frame header length: %d", headerLen)
	}
	headerBytes := make([]byte, headerLen)
	if _, err := io.ReadFull(r, headerBytes); err != nil {
		return nil, fmt.Errorf("read frame header: %w", err)
	}
	var header frameHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("unmarshal frame header: %w", err)
	}
	var body []byte
	if header.BodyLength > 0 {
		body = make([]byte, header.BodyLength)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, fmt.Errorf("read frame body: %w", err)
		}
	}
	return &frame{Header: header, Body: body}, nil
}

func writeFrame(w io.Writer, f *frame) error {
	f.Header.BodyLength = len(f.Body)
	headerBytes, err := json.Marshal(f.Header)
	if err != nil {
		return fmt.Errorf("marshal header: %w", err)
	}
	headerLen := uint32(len(headerBytes))
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], headerLen)
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := w.Write(headerBytes); err != nil {
		return err
	}
	if len(f.Body) > 0 {
		if _, err := w.Write(f.Body); err != nil {
			return err
		}
	}
	return nil
}

// Runtime orchestrates the worker-side process lifecycle.
type Runtime struct {
	workerName string
	buildID    string
	sockPath   string
	routes     []abi.Route
	handlers   map[abi.HandlerID]HandlerFunc
	handlersMu sync.RWMutex

	conn     net.Conn
	inFlight atomic.Int64
	draining atomic.Bool
	stopChan chan struct{}
}

// NewRuntime initializes a worker Runtime instance.
func NewRuntime(workerName, buildID, sockPath string) *Runtime {
	return &Runtime{
		workerName: workerName,
		buildID:    buildID,
		sockPath:   sockPath,
		handlers:   make(map[abi.HandlerID]HandlerFunc),
		stopChan:   make(chan struct{}),
	}
}

// RegisterHandler registers a route and its handler adapter in the worker runtime.
func (r *Runtime) RegisterHandler(route abi.Route, fn HandlerFunc) {
	r.handlersMu.Lock()
	defer r.handlersMu.Unlock()
	r.routes = append(r.routes, route)
	r.handlers[route.ID] = fn
}

// Start connects to WORKER_SOCK, transmits Hello, awaits Ready, and enters request serving.
func (r *Runtime) Start(ctx context.Context) error {
	sock := r.sockPath
	if sock == "" {
		sock = os.Getenv("WORKER_SOCK")
	}
	if sock == "" {
		return fmt.Errorf("WORKER_SOCK environment variable not specified")
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		return fmt.Errorf("connect to mother socket %s: %w", sock, err)
	}
	r.conn = conn

	// 1. Send Hello frame
	hello := &abi.Hello{
		ABIVersion: abi.ABIVersion,
		WorkerName: r.workerName,
		BuildID:    r.buildID,
		PID:        os.Getpid(),
		Routes:     r.routes,
	}
	helloBytes, err := json.Marshal(hello)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("marshal hello: %w", err)
	}

	hFrame := &frame{
		Header: frameHeader{
			Type: "hello",
			ID:   r.workerName,
			Metadata: map[string]string{
				"abi_version": abi.ABIVersion,
				"worker_name": r.workerName,
				"build_id":    r.buildID,
			},
		},
		Body: helloBytes,
	}
	if err := writeFrame(conn, hFrame); err != nil {
		_ = conn.Close()
		return fmt.Errorf("send hello frame: %w", err)
	}

	// 2. Wait for Ready frame
	readyFrame, err := readFrame(conn)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("read ready frame: %w", err)
	}
	var ready abi.Ready
	if err := json.Unmarshal(readyFrame.Body, &ready); err != nil {
		_ = conn.Close()
		return fmt.Errorf("unmarshal ready frame: %w", err)
	}
	if ready.Status != "ready" {
		_ = conn.Close()
		return fmt.Errorf("mother rejected worker readiness: %s", ready.Message)
	}

	// 3. Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		select {
		case <-sigChan:
			_ = r.Drain(context.Background(), 5*time.Second)
		case <-r.stopChan:
		}
	}()

	return r.serveLoop()
}

func (r *Runtime) serveLoop() error {
	defer func() {
		if r.conn != nil {
			_ = r.conn.Close()
		}
	}()

	var writeMu sync.Mutex

	for {
		f, err := readFrame(r.conn)
		if err != nil {
			if r.draining.Load() || err == io.EOF {
				return nil
			}
			return fmt.Errorf("worker read frame error: %w", err)
		}

		switch f.Header.Type {
		case "drain":
			return r.Drain(context.Background(), 5*time.Second)
		case "ping":
			pong := &frame{Header: frameHeader{Type: "pong", ID: f.Header.ID}}
			writeMu.Lock()
			_ = writeFrame(r.conn, pong)
			writeMu.Unlock()
		case "request":
			r.inFlight.Add(1)
			go func(reqFrame *frame) {
				defer r.inFlight.Add(-1)
				respFrame := r.dispatch(reqFrame)
				writeMu.Lock()
				_ = writeFrame(r.conn, respFrame)
				writeMu.Unlock()
			}(f)
		}
	}
}

func (r *Runtime) dispatch(f *frame) *frame {
	var req abi.Request
	if err := json.Unmarshal(f.Body, &req); err != nil {
		errResp := abi.NewErrorResponse(fmt.Errorf("malformed request frame: %w", err))
		body, _ := json.Marshal(errResp)
		return &frame{Header: frameHeader{Type: "response", ID: f.Header.ID}, Body: body}
	}

	handlerID := abi.HandlerID(f.Header.Metadata["handler_id"])
	if handlerID == "" {
		handlerID = abi.HandlerID(req.RoutePath)
	}

	r.handlersMu.RLock()
	handler, ok := r.handlers[handlerID]
	r.handlersMu.RUnlock()

	if !ok {
		errResp := abi.NewErrorResponse(fmt.Errorf("handler %q not registered in worker", handlerID))
		errResp.Status = 404
		body, _ := json.Marshal(errResp)
		return &frame{Header: frameHeader{Type: "response", ID: f.Header.ID}, Body: body}
	}

	resp, err := handler(&req)
	if err != nil {
		resp = abi.NewErrorResponse(err)
	} else if resp == nil {
		resp = abi.NewResponse(204, nil)
	}

	respBytes, _ := json.Marshal(resp)
	return &frame{Header: frameHeader{Type: "response", ID: f.Header.ID}, Body: respBytes}
}

// Drain gracefully drains in-flight requests and exits.
func (r *Runtime) Drain(ctx context.Context, timeout time.Duration) error {
	if !r.draining.CompareAndSwap(false, true) {
		return nil
	}

	deadline := time.Now().Add(timeout)
	for r.inFlight.Load() > 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	close(r.stopChan)
	if r.conn != nil {
		_ = r.conn.Close()
	}
	return nil
}
`

const readmeDoc = `# nxp Standalone Server

This project runs on the standalone **nxp** backend framework.

## 100% Self-Contained Binary
The ` + "`nxp-server`" + ` executable embeds everything it needs:
- **Zero external Git dependencies**: Developers do not need to clone or download ` + "`github.com/Neuxbane/NeuXbaneProtocol`" + `.
- **Offline compilation**: Handlers and workers compile completely locally using the local ` + "`abi/`" + ` layer.
- **Single-binary portability**: Share only the ` + "`nxp-server`" + ` binary. On first run, it scaffolds all required files and documentation.

## Developer Quickstart

All business logic lives inside ` + "`define/`" + `.
You do NOT need to write HTTP routers, chunk logic, or schema manifests.

### 1. Handler Signature

Every route handler implements:
` + "```go" + `
func Handler(ctx *rest.Ctx) (Result, error)
` + "```" + `

### 2. Clean Local ABI Imports

Handlers import the server-generated ` + "`abi`" + ` packages directly:
` + "```go" + `
package define

import (
    "abi/rest"
)

type WelcomeResult struct {
    Message string ` + "`json:\"message\" validate:\"required\"`" + `
    Status  string ` + "`json:\"status\"  validate:\"required\"`" + `
}

func Handler(ctx *rest.Ctx) (WelcomeResult, error) {
    return WelcomeResult{
        Message: "Welcome to nxp framework!",
        Status:  "operational",
    }, nil
}
` + "```" + `

Alternatively, you can use the unified import:
` + "```go" + `
import "abi"

func Handler(ctx *abi.Ctx) (WelcomeResult, error)
` + "```" + `

### 3. Running the Server

Start the server:
` + "```bash" + `
./nxp-server -addr :8080 -env dev
` + "```" + `

- Test endpoint: ` + "`curl http://localhost:8080/`" + `
- Test introspection: ` + "`curl http://localhost:8080/?nxp`" + `
- Hot reload: edit any file in ` + "`define/`" + ` and save. Changes take effect in <500ms without restarting the server!

See ` + "`define/HOW_TO_DEFINE.md`" + ` and ` + "`define/ABI.md`" + ` for comprehensive guides.
`

const howToDefineDoc = `# How to Write Handlers in nxp

Welcome to **nxp**! In this framework, you write **only business logic**.
Routing, schema reflection, validation, transports, WebRTC SFU, hot reload, and introspection are handled automatically.

---

## 1. The Core Rule

Every handler must implement the exact signature:

` + "```go" + `
func Handler(ctx *DOMAIN.Ctx) (Result, error)
` + "```" + `

You never see ` + "`net/http`" + `, socket file descriptors, low-level chunk buffers, or raw schemas.

---

## 2. Directory Tree Routing

The directory structure under ` + "`define/`" + ` directly defines your routes:

- ` + "`define/index.go`" + ` maps to ` + "`GET /`" + `
- ` + "`define/auth/index.go`" + ` maps to ` + "`GET /auth`" + ` (never ` + "`/auth/index`" + `)
- ` + "`define/items/item/index.go`" + ` with ` + "`// @route /items/{id}`" + ` maps to ` + "`GET /items/{id}`" + `
- ` + "`define/items/create/index.go`" + ` with ` + "`// @method POST`" + ` maps to ` + "`POST /items/create`" + `

Filename prefixes can also specify method and transport:
- ` + "`get_`, `post_`, `put_`, `del_`, `patch_`" + ` -> REST HTTP verbs
- ` + "`ws_`" + ` -> WebSocket
- ` + "`grpc_`" + ` -> gRPC HTTP/2
- ` + "`udp_`" + ` -> UDP datagram
- ` + "`mqtt_`" + ` -> MQTT subscriber
- ` + "`nats_`" + ` -> NATS subscriber
- ` + "`kafka_`" + ` -> Kafka consumer

---

## 3. Types are the Contract

Struct tags are reflected into JSON Schema Draft 2020-12 at build time:
- ` + "`json:\"field_name\"`" + `
- ` + "`validate:\"required,min=1,max=100\"`" + `
- ` + "`format:\"email\"`" + ` or ` + "`format:\"date-time\"`" + `
- ` + "`doc:\"Human-readable description\"`" + `

Ingress validation runs in the Mother process before your handler is ever called.
If a request fails validation, a 400 error is returned immediately and your worker is not invoked.

---

## 4. Available Domains (via server-generated abi/)

Import from ` + "`abi/*`" + ` or unified ` + "`abi`" + `:

1. **REST APIs (abi/rest):**
   ` + "```go" + `
   import "abi/rest"

   func Handler(ctx *rest.Ctx) (Result, error)
   ` + "```" + `
   Available methods on ` + "`ctx`" + `:
   - ` + "`ctx.Param(\"id\")`" + ` - path parameter
   - ` + "`ctx.Query(\"sort\")`" + ` - query parameter
   - ` + "`ctx.Header(\"Authorization\")`" + ` - header
   - ` + "`ctx.Bind(&data)`" + ` - binds JSON body
   - ` + "`ctx.Identity()`" + ` - authenticated user (subject, scopes, roles)

2. **File Uploads / Downloads (abi/files):**
   ` + "```go" + `
   import "abi/files"

   func Handler(ctx *files.UploadCtx) (files.UploadResult, error)
   ` + "```" + `
   Methods:
   - ` + "`ctx.SHA256()`" + `, ` + "`ctx.Size()`" + `, ` + "`ctx.Mime()`" + `, ` + "`ctx.Reader()`" + `, ` + "`ctx.MoveTo(dest)`" + `

3. **WebSockets (abi/ws):**
   ` + "```go" + `
   import "abi/ws"

   func Handler(ctx *ws.Ctx) error
   ` + "```" + `

4. **WebRTC SFU (abi/rtc):**
   ` + "```go" + `
   import "abi/rtc"

   func Handler(ctx *rtc.SessionCtx) error
   ` + "```" + `

5. **MQTT Pub/Sub (abi/mqtt):**
   ` + "```go" + `
   import "abi/mqtt"

   func Handler(ctx *mqtt.Ctx[InputEvent, OutputEvent]) (OutputEvent, error)
   ` + "```" + `

---

## 5. Serving Files, HTML, CSS, Video, and S3 Assets

Handlers are not limited to JSON. You can return HTML, CSS, images, videos with byte-range seeking, database blobs, or S3 presigned downloads:

### A. Returning HTML or CSS
` + "```go" + `
func Handler(ctx *rest.Ctx) (string, error) {
    return ctx.HTML("<h1>Welcome to nxp!</h1>")
}

func Handler(ctx *rest.Ctx) (string, error) {
    return ctx.CSS("body { background: #111; color: #fff; }")
}
` + "```" + `

### B. Streaming Video & Audio (with Range & Seeking Support)
The framework automatically handles HTTP 206 Partial Content byte ranges, ETags, and streaming without loading the entire video into memory:
` + "```go" + `
func Handler(ctx *rest.Ctx) (any, error) {
    return ctx.File("./assets/video.mp4")
}
` + "```" + `

### C. Serving In-Memory Images or Database Blobs
` + "```go" + `
func Handler(ctx *rest.Ctx) (any, error) {
    pngBytes := fetchImageFromDatabase() // []byte
    return ctx.Blob("image/png", pngBytes)
}
` + "```" + `

### D. Download as Attachment
` + "```go" + `
func Handler(ctx *rest.Ctx) (any, error) {
    return ctx.Download("./reports/annual.pdf", "Annual_Report_2026.pdf")
}
` + "```" + `

### E. S3 Presigned URLs & Redirects
` + "```go" + `
import "abi/files"

func Handler(ctx *rest.Ctx) (files.DownloadResult, error) {
    return files.DownloadResult{
        Source:    "https://my-bucket.s3.amazonaws.com/large-asset.zip?...",
        Presigned: true,
    }, nil
}
` + "```" + `
`

const abiDoc = `# Application Binary Interface (abi) Reference

The ` + "`abi`" + ` package provides the frozen contract between your handlers and the **nxp** runtime engine.

## Unified Imports

You can import domain-specific packages:
` + "```go" + `
import "abi/rest"
import "abi/files"
import "abi/ws"
import "abi/rtc"
import "abi/mqtt"
import "abi/errors"
` + "```" + `

Or import the unified ` + "`abi`" + ` package directly:
` + "```go" + `
import "abi"

func Handler(ctx *abi.Ctx) (MyResult, error)
` + "```" + `

All types in ` + "`abi`" + ` are standard Go structs that require no external dependencies.
`
