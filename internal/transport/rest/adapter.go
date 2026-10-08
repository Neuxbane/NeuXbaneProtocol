// Package rest answers: how are HTTP/1.1 and HTTP/2 requests normalized, served, and rendered?
package rest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/contract"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/telemetry"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport"
)

func init() {
	transport.RegisterAdapter(NewAdapter(nil))
}

// Adapter implements transport.Adapter for HTTP/1.1 and HTTP/2 REST APIs.
type Adapter struct {
	dispatcher   transport.DispatcherFunc
	server       *http.Server
	table        *router.Table
	introspectFn func(w http.ResponseWriter, r *http.Request, table *router.Table) bool
	wsHandler    http.Handler
	mu           sync.RWMutex
}

// NewAdapter creates an initialized REST transport adapter.
func NewAdapter(dispatcher transport.DispatcherFunc) *Adapter {
	return &Adapter{
		dispatcher: dispatcher,
	}
}

// SetDispatcher configures the worker dispatch callback.
func (a *Adapter) SetDispatcher(fn transport.DispatcherFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dispatcher = fn
}

// SetIntrospector configures the ?nxp inspection handler.
func (a *Adapter) SetIntrospector(fn func(w http.ResponseWriter, r *http.Request, table *router.Table) bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.introspectFn = fn
}

// SetWebSocketHandler registers an http.Handler that serves WebSocket upgrade
// requests. When set, the REST adapter delegates any request carrying an
// "Upgrade: websocket" header to this handler, allowing both transports to
// share a single listener/port.
func (a *Adapter) SetWebSocketHandler(h http.Handler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.wsHandler = h
}

// Name implements transport.Adapter.
func (a *Adapter) Name() abi.Transport {
	return abi.TransportREST
}

// MatchKey implements transport.Adapter.
func (a *Adapter) MatchKey() string {
	return "rest:GET:/path"
}

// Normalize transforms a raw *http.Request into a normalized *abi.Request.
func (a *Adapter) Normalize(raw any) (*abi.Request, error) {
	httpReq, ok := raw.(*http.Request)
	if !ok {
		return nil, fmt.Errorf("expected *http.Request, got %T", raw)
	}

	req := abi.NewRequest(abi.TransportREST, httpReq.Method, httpReq.URL.Path)

	// Copy headers
	for k, v := range httpReq.Header {
		req.Headers[k] = v
	}

	// Copy query parameters
	for k, v := range httpReq.URL.Query() {
		req.Query[k] = v
	}

	// Read body
	if httpReq.Body != nil {
		bodyBytes, err := io.ReadAll(httpReq.Body)
		if err != nil {
			return nil, fmt.Errorf("read http body: %w", err)
		}
		_ = httpReq.Body.Close()
		req.Body = bodyBytes
	}

	// Extract identity from the Authorization Bearer header or the well-known
	// auth cookies (shared with the WebSocket transport).
	req.Identity = transport.ExtractIdentity(httpReq)

	return req, nil
}

// Render writes the *abi.Response to an io.Writer with HTTP caching, ETag, and Range support.
func (a *Adapter) Render(w io.Writer, resp *abi.Response, ctx transport.RenderCtx) error {
	rw, isHTTPWriter := w.(http.ResponseWriter)
	if !isHTTPWriter {
		_, err := w.Write(resp.Body)
		return err
	}

	// Handle file asset streaming or presigned URL redirection
	if redir := resp.Metadata["redirect"]; redir != "" {
		for k, vals := range resp.Headers {
			for _, val := range vals {
				rw.Header().Add(k, val)
			}
		}
		rw.Header().Set("Location", redir)
		status := resp.Status
		if status < 300 || status >= 400 {
			status = http.StatusTemporaryRedirect
		}
		rw.WriteHeader(status)
		return nil
	}

	fileSource := resp.Metadata["file_source"]
	if fileSource == "" {
		fileSource = resp.Metadata["file_path"]
	}

	if fileSource != "" {
		if resp.Metadata["presigned"] == "true" || strings.HasPrefix(fileSource, "http://") || strings.HasPrefix(fileSource, "https://") {
			for k, vals := range resp.Headers {
				for _, val := range vals {
					rw.Header().Add(k, val)
				}
			}
			rw.Header().Set("Location", fileSource)
			status := resp.Status
			if status < 300 || status >= 400 {
				status = http.StatusTemporaryRedirect
			}
			rw.WriteHeader(status)
			return nil
		}

		if fileInfo, err := os.Stat(fileSource); err == nil && !fileInfo.IsDir() {
			for k, vals := range resp.Headers {
				for _, val := range vals {
					rw.Header().Add(k, val)
				}
			}
			if resp.Metadata["file_name"] != "" && rw.Header().Get("Content-Disposition") == "" {
				rw.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, resp.Metadata["file_name"]))
			}
			if resp.Metadata["file_mime"] != "" && rw.Header().Get("Content-Type") == "" {
				rw.Header().Set("Content-Type", resp.Metadata["file_mime"])
			}
			if req, ok := ctx.Raw.(*http.Request); ok && req != nil {
				http.ServeFile(rw, req, fileSource)
				return nil
			}
		}
	}

	// Compute ETag
	bodyHash := sha256.Sum256(resp.Body)
	etag := fmt.Sprintf(`W/"%s"`, hex.EncodeToString(bodyHash[:8]))

	// Copy response headers
	for k, vals := range resp.Headers {
		for _, val := range vals {
			rw.Header().Add(k, val)
		}
	}
	rw.Header().Set("ETag", etag)

	// Handle conditional GET (If-None-Match)
	if req, ok := ctx.Raw.(*http.Request); ok && req != nil {
		if ifNoneMatch := req.Header.Get("If-None-Match"); ifNoneMatch != "" {
			if ifNoneMatch == etag || ifNoneMatch == "*" {
				rw.WriteHeader(http.StatusNotModified)
				return nil
			}
		}

		// Handle Range requests
		rangeHeader := req.Header.Get("Range")
		if strings.HasPrefix(rangeHeader, "bytes=") && len(resp.Body) > 0 {
			rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
			parts := strings.Split(rangeSpec, "-")
			total := len(resp.Body)
			start, _ := strconv.Atoi(parts[0])
			end := total - 1
			if len(parts) > 1 && parts[1] != "" {
				if endVal, err := strconv.Atoi(parts[1]); err == nil && endVal < total {
					end = endVal
				}
			}

			if start >= 0 && start <= end && end < total {
				partialBody := resp.Body[start : end+1]
				rw.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
				rw.Header().Set("Content-Length", strconv.Itoa(len(partialBody)))
				rw.WriteHeader(http.StatusPartialContent)
				_, err := rw.Write(partialBody)
				return err
			}
		}
	}

	if rw.Header().Get("Content-Type") == "" {
		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	}

	// If response is successful and contains a "token" field, ensure auth cookies are set
	if resp.Status >= 200 && resp.Status < 300 && len(resp.Body) > 0 && rw.Header().Get("Set-Cookie") == "" {
		var tokenDetector struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(resp.Body, &tokenDetector); err == nil && tokenDetector.Token != "" {
			cookie := &http.Cookie{
				Name:     "xm_session",
				Value:    tokenDetector.Token,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			}
			http.SetCookie(rw, cookie)
		}
	}

	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}

	rw.WriteHeader(status)
	if len(resp.Body) > 0 {
		_, err := rw.Write(resp.Body)
		return err
	}
	return nil
}

// Serve starts listening for HTTP traffic on listener and routes via table.
func (a *Adapter) Serve(listener *net.Listener, table *router.Table) error {
	a.table = table

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		a.handleHTTP(w, r)
	})

	a.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return a.server.Serve(*listener)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

func (a *Adapter) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// 0. Delegate WebSocket upgrade requests to the WebSocket adapter so both
	// transports can share a single listener/port.
	if isWebSocketUpgrade(r) {
		a.mu.RLock()
		wsHandler := a.wsHandler
		a.mu.RUnlock()
		if wsHandler != nil {
			wsHandler.ServeHTTP(w, r)
			return
		}
	}

	start := time.Now()
	sw := &statusWriter{ResponseWriter: w, status: 200}

	defer func() {
		duration := time.Since(start)
		clientIP := r.RemoteAddr
		if host, _, err := net.SplitHostPort(clientIP); err == nil {
			clientIP = host
		}
		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		telemetry.LogRequest(r.Method, path, clientIP, sw.status, duration, "")
	}()

	// 1. Intercept ?nxp before route dispatch
	a.mu.RLock()
	introspectFn := a.introspectFn
	dispatcher := a.dispatcher
	table := a.table
	a.mu.RUnlock()

	if r.URL.Query().Has("nxp") && introspectFn != nil {
		if handled := introspectFn(sw, r, table); handled {
			return
		}
	}

	// 2. Normalize HTTP request
	abiReq, err := a.Normalize(r)
	if err != nil {
		sw.WriteHeader(http.StatusBadRequest)
		_, _ = sw.Write([]byte(err.Error()))
		return
	}

	// 3. Look up route in table
	entry, params, found := table.Load(abi.TransportREST, r.Method, r.URL.Path)
	if !found {
		sw.status = http.StatusNotFound
		errResp := abi.NewErrorResponse(errors.ErrNotFound)
		_ = a.Render(sw, errResp, transport.RenderCtx{Raw: r})
		return
	}

	abiReq.RoutePath = entry.Route.Path
	if len(params) > 0 {
		abiReq.Params = params
	}

	// 4. Validate Ingress contract in mother process (Principle 3: The mother enforces, the worker trusts)
	if valErr := contract.ValidateRequest(&entry.Route, abiReq); valErr != nil {
		telemetry.GlobalMetrics.ContractViolations.Add(1)
		sw.status = http.StatusBadRequest
		errResp := abi.NewErrorResponse(valErr)
		_ = a.Render(sw, errResp, transport.RenderCtx{Raw: r})
		return
	}

	// 5. Dispatch request to worker
	if dispatcher == nil {
		sw.WriteHeader(http.StatusServiceUnavailable)
		_, _ = sw.Write([]byte("dispatcher not configured"))
		return
	}

	abiResp, err := dispatcher(r.Context(), abiReq)
	if err != nil {
		sw.status = http.StatusServiceUnavailable
		errResp := abi.NewErrorResponse(errors.New(errors.CodeContractWorkerUnavailable, err.Error(), 503))
		_ = a.Render(sw, errResp, transport.RenderCtx{Raw: r})
		return
	}

	// 6. Validate Egress contract
	if egressErr := contract.ValidateResponse(&entry.Route, abiResp); egressErr != nil {
		telemetry.GlobalMetrics.ContractViolations.Add(1)
		_ = contract.HandleEgressViolation(contract.EgressPolicyQuarantine, entry.WorkerName, egressErr)
		sw.status = http.StatusInternalServerError
		errResp := abi.NewErrorResponse(egressErr)
		_ = a.Render(sw, errResp, transport.RenderCtx{Raw: r})
		return
	}

	// 7. Render response to client
	if abiResp != nil && abiResp.Status != 0 {
		sw.status = abiResp.Status
	}
	_ = a.Render(sw, abiResp, transport.RenderCtx{Raw: r})
}

// Shutdown gracefully terminates the HTTP server.
func (a *Adapter) Shutdown(ctx context.Context) error {
	if a.server != nil {
		return a.server.Shutdown(ctx)
	}
	return nil
}

// isWebSocketUpgrade reports whether r is a WebSocket upgrade handshake.
func isWebSocketUpgrade(r *http.Request) bool {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	for _, token := range strings.Split(r.Header.Get("Connection"), ",") {
		if strings.EqualFold(strings.TrimSpace(token), "upgrade") {
			return true
		}
	}
	return false
}

// JSONResponse helper parses data into an *abi.Response.
func JSONResponse(status int, v any) (*abi.Response, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	resp := abi.NewResponse(status, b)
	resp.SetHeader("Content-Type", "application/json; charset=utf-8")
	return resp, nil
}
