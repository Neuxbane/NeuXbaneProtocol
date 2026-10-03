// Package rest provides the developer-facing HTTP/REST context and helper methods.
package rest

import (
	"context"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// Ctx is the handler execution context provided to REST route handlers.
type Ctx struct {
	context.Context
	req  *abi.Request
	resp *abi.Response
}

// NewCtx creates an initialized REST Ctx.
func NewCtx(parent context.Context, req *abi.Request) *Ctx {
	if parent == nil {
		parent = context.Background()
	}
	return &Ctx{
		Context: parent,
		req:     req,
		resp:    abi.NewResponse(200, nil),
	}
}

// Request returns the underlying normalized abi.Request.
func (c *Ctx) Request() *abi.Request {
	return c.req
}

// Param returns a path parameter by name.
func (c *Ctx) Param(name string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Param(name)
}

// Query returns the query parameter value for key.
func (c *Ctx) Query(key string) string {
	if c.req == nil {
		return ""
	}
	return c.req.QueryParam(key)
}

// Header returns the request header value for key.
func (c *Ctx) Header(key string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Header(key)
}

// Identity returns the authenticated caller context if present.
func (c *Ctx) Identity() *abi.Identity {
	if c.req == nil {
		return nil
	}
	return c.req.Identity
}

// Bind unmarshals the request payload into target.
func (c *Ctx) Bind(target any) error {
	if c.req == nil {
		return nil
	}
	return c.req.BindJSON(target)
}

// SetHeader sets a response header.
func (c *Ctx) SetHeader(key, val string) {
	if c.resp == nil {
		c.resp = abi.NewResponse(200, nil)
	}
	c.resp.SetHeader(key, val)
}

// Status sets the HTTP status code.
func (c *Ctx) Status(code int) {
	if c.resp == nil {
		c.resp = abi.NewResponse(code, nil)
	}
	c.resp.Status = code
}

// Response returns the underlying response proto.
func (c *Ctx) Response() *abi.Response {
	if c.resp == nil {
		c.resp = abi.NewResponse(200, nil)
	}
	return c.resp
}

// HTML sets Content-Type to text/html and returns the string.
func (c *Ctx) HTML(html string) (string, error) {
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	return html, nil
}

// Text sets Content-Type to text/plain and returns the string.
func (c *Ctx) Text(text string) (string, error) {
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	return text, nil
}

// CSS sets Content-Type to text/css and returns the string.
func (c *Ctx) CSS(css string) (string, error) {
	c.SetHeader("Content-Type", "text/css; charset=utf-8")
	return css, nil
}

// Blob returns arbitrary binary data with the given MIME type.
func (c *Ctx) Blob(mime string, data []byte) (any, error) {
	c.SetHeader("Content-Type", mime)
	return data, nil
}

// File tells the runtime to serve/stream a file from disk or storage with Range request support.
func (c *Ctx) File(filePath string) (abi.DownloadResult, error) {
	return abi.DownloadResult{
		Source: filePath,
	}, nil
}

// Download tells the runtime to serve a file with attachment Content-Disposition.
func (c *Ctx) Download(filePath, filename string) (abi.DownloadResult, error) {
	return abi.DownloadResult{
		Source: filePath,
		Name:   filename,
	}, nil
}

// Redirect initiates an HTTP redirect.
func (c *Ctx) Redirect(targetURL string, statusCode int) (any, error) {
	c.Status(statusCode)
	c.SetHeader("Location", targetURL)
	return "", nil
}
