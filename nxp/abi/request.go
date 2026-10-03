package abi

import (
	"encoding/json"
	"net/textproto"
	"strings"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
)

// Request is the normalized, transport-agnostic representation of an inbound request or event.
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

// NewRequest creates an initialized Request with current timestamp.
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

// Header returns the first value associated with the canonicalized key.
func (r *Request) Header(key string) string {
	if r.Headers == nil {
		return ""
	}
	canonical := textproto.CanonicalMIMEHeaderKey(key)
	if vals, ok := r.Headers[canonical]; ok && len(vals) > 0 {
		return vals[0]
	}
	// Fallback case-insensitive search
	for k, vals := range r.Headers {
		if strings.EqualFold(k, key) && len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

// SetHeader sets the header entries associated with key to the single element value.
func (r *Request) SetHeader(key, value string) {
	if r.Headers == nil {
		r.Headers = make(map[string][]string)
	}
	r.Headers[textproto.CanonicalMIMEHeaderKey(key)] = []string{value}
}

// Param returns a path parameter by name.
func (r *Request) Param(name string) string {
	if r.Params == nil {
		return ""
	}
	return r.Params[name]
}

// QueryParam returns the first value associated with the query parameter key.
func (r *Request) QueryParam(key string) string {
	if r.Query == nil {
		return ""
	}
	if vals, ok := r.Query[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// QueryParams returns all values associated with the query parameter key.
func (r *Request) QueryParams(key string) []string {
	if r.Query == nil {
		return nil
	}
	return r.Query[key]
}

// BindJSON unmarshals the request body into the target value.
func (r *Request) BindJSON(target any) error {
	if len(r.Body) == 0 {
		return errors.New(errors.CodeBadRequest, "empty request body", 400)
	}
	if err := json.Unmarshal(r.Body, target); err != nil {
		return errors.New(errors.CodeBadRequest, "malformed JSON payload: "+err.Error(), 400).WithCause(err)
	}
	return nil
}

// Response is the normalized, transport-agnostic representation of an outbound response or event.
type Response struct {
	Status   int                 `json:"status"`
	Headers  map[string][]string `json:"headers,omitempty"`
	Body     []byte              `json:"body,omitempty"`
	Error    *errors.Error       `json:"error,omitempty"`
	Ack      bool                `json:"ack,omitempty"`
	Topic    string              `json:"topic,omitempty"`
	Metadata map[string]string   `json:"metadata,omitempty"`
}

// NewResponse creates an initialized Response.
func NewResponse(status int, body []byte) *Response {
	return &Response{
		Status:   status,
		Headers:  make(map[string][]string),
		Body:     body,
		Metadata: make(map[string]string),
	}
}

// DownloadResult represents a file asset to download or stream.
type DownloadResult struct {
	Source    string `json:"source"`
	Name      string `json:"name,omitempty"`
	Mime      string `json:"mime,omitempty"`
	Presigned bool   `json:"presigned,omitempty"`
}

// RawResponse represents a response with custom Content-Type and raw byte body.
type RawResponse struct {
	ContentType string `json:"content_type"`
	Data        []byte `json:"data"`
}

// NewAutoResponse inspects the return value from a handler and creates an appropriate Response.
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
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, errors.New(errors.CodeInternal, "failed to marshal response JSON: "+err.Error(), 500).WithCause(err)
		}
		proto.Body = bytes
		if proto.Header("Content-Type") == "" {
			proto.SetHeader("Content-Type", "application/json; charset=utf-8")
		}
		return proto, nil
	}
}

// NewJSONResponse serializes data as JSON and creates a 200/status Response.
func NewJSONResponse(status int, data any) (*Response, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, errors.New(errors.CodeInternal, "failed to marshal response JSON: "+err.Error(), 500).WithCause(err)
	}
	resp := NewResponse(status, bytes)
	resp.SetHeader("Content-Type", "application/json; charset=utf-8")
	return resp, nil
}

// NewErrorResponse builds a Response from an error, mapping nxp/errors appropriately.
func NewErrorResponse(err error) *Response {
	if err == nil {
		return NewResponse(200, nil)
	}
	var nxpErr *errors.Error
	switch e := err.(type) {
	case *errors.Error:
		nxpErr = e
	default:
		nxpErr = errors.New(errors.CodeInternal, err.Error(), 500).WithCause(err)
	}

	body, _ := json.Marshal(nxpErr)
	resp := NewResponse(nxpErr.HTTPStatusCode(), body)
	resp.Error = nxpErr
	resp.SetHeader("Content-Type", "application/json; charset=utf-8")
	return resp
}

// Header returns the first value associated with key.
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

// SetHeader sets the header entries associated with key to the single element value.
func (r *Response) SetHeader(key, value string) {
	if r.Headers == nil {
		r.Headers = make(map[string][]string)
	}
	r.Headers[textproto.CanonicalMIMEHeaderKey(key)] = []string{value}
}

// AddHeader adds the key, value pair to the response headers.
func (r *Response) AddHeader(key, value string) {
	if r.Headers == nil {
		r.Headers = make(map[string][]string)
	}
	k := textproto.CanonicalMIMEHeaderKey(key)
	r.Headers[k] = append(r.Headers[k], value)
}
