package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport/rest"
)

type UserReq struct {
	Name  string `json:"name" validate:"required,minlen=2"`
	Email string `json:"email" validate:"required,email"`
}

type UserResp struct {
	ID   string `json:"id" validate:"required,uuid"`
	Name string `json:"name" validate:"required"`
}

func TestRESTNormalize(t *testing.T) {
	adapter := rest.NewAdapter(nil)

	httpReq := httptest.NewRequest("POST", "/users?sort=asc", bytes.NewBufferString(`{"name":"Alice"}`))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer my-secret-jwt")

	abiReq, err := adapter.Normalize(httpReq)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if abiReq.Method != "POST" || abiReq.Path != "/users" {
		t.Errorf("method/path mismatch: %s %s", abiReq.Method, abiReq.Path)
	}
	if abiReq.QueryParam("sort") != "asc" {
		t.Errorf("query param sort mismatch: %s", abiReq.QueryParam("sort"))
	}
	if abiReq.Header("Content-Type") != "application/json" {
		t.Errorf("header Content-Type mismatch: %s", abiReq.Header("Content-Type"))
	}
	if abiReq.Identity == nil || abiReq.Identity.Subject != "my-secret-jwt" {
		t.Errorf("identity extraction failed: %+v", abiReq.Identity)
	}
	if string(abiReq.Body) != `{"name":"Alice"}` {
		t.Errorf("body mismatch: %s", string(abiReq.Body))
	}
}

func TestRESTRenderCachingAndRange(t *testing.T) {
	adapter := rest.NewAdapter(nil)

	body := []byte("Hello, world! 0123456789")
	resp := abi.NewResponse(200, body)

	// 1. Initial Render (verify ETag generated)
	rec := httptest.NewRecorder()
	httpReq := httptest.NewRequest("GET", "/file", nil)
	if err := adapter.Render(rec, resp, transport.RenderCtx{Raw: httpReq}); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header to be set")
	}

	// 2. Conditional GET with matching If-None-Match (expect 304)
	condReq := httptest.NewRequest("GET", "/file", nil)
	condReq.Header.Set("If-None-Match", etag)
	condRec := httptest.NewRecorder()

	if err := adapter.Render(condRec, resp, transport.RenderCtx{Raw: condReq}); err != nil {
		t.Fatalf("Render conditional failed: %v", err)
	}
	if condRec.Code != http.StatusNotModified {
		t.Errorf("expected 304 Not Modified, got %d", condRec.Code)
	}

	// 3. Range request (bytes=0-4 -> "Hello")
	rangeReq := httptest.NewRequest("GET", "/file", nil)
	rangeReq.Header.Set("Range", "bytes=0-4")
	rangeRec := httptest.NewRecorder()

	if err := adapter.Render(rangeRec, resp, transport.RenderCtx{Raw: rangeReq}); err != nil {
		t.Fatalf("Render range failed: %v", err)
	}
	if rangeRec.Code != http.StatusPartialContent {
		t.Errorf("expected 206 Partial Content, got %d", rangeRec.Code)
	}
	if rangeRec.Body.String() != "Hello" {
		t.Errorf("range body mismatch: got %q, want 'Hello'", rangeRec.Body.String())
	}
}

func TestRESTEndToEndContractEnforcement(t *testing.T) {
	reqSchema := abi.SchemaOf[UserReq]()
	respSchema := abi.SchemaOf[UserResp]()

	tbl := router.NewTable()
	route := abi.Route{
		ID:        "users.create",
		Transport: abi.TransportREST,
		Method:    "POST",
		Path:      "/users",
		Shape: abi.RequestResponseShape{
			Request: reqSchema,
			Responses: map[int]*abi.Schema{
				200: respSchema,
			},
		},
	}
	tbl.Store(&router.Entry{
		Route:      route,
		WorkerName: "worker-users",
	})

	workerReached := false
	var mockWorkerResponse *abi.Response

	adapter := rest.NewAdapter(func(ctx context.Context, req *abi.Request) (*abi.Response, error) {
		workerReached = true
		return mockWorkerResponse, nil
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	go func() {
		_ = adapter.Serve(&listener, tbl)
	}()
	defer adapter.Shutdown(context.Background())

	baseURL := "http://" + listener.Addr().String()

	// 1. Invalid Request Body -> returns contract.request.invalid (400) and NEVER reaches worker!
	workerReached = false
	badReqBody := `{"name":"a","email":"not-an-email"}`
	resp, err := http.Post(baseURL+"/users", "application/json", bytes.NewBufferString(badReqBody))
	if err != nil {
		t.Fatalf("post bad request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
	if workerReached {
		t.Errorf("principle violated: worker was reached despite invalid inbound contract!")
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(bodyBytes, []byte("contract.request.invalid")) {
		t.Errorf("expected contract.request.invalid in response: %s", string(bodyBytes))
	}

	// 2. Worker returns invalid response shape -> client gets 502 Bad Gateway!
	workerReached = false
	// Invalid token (not a uuid)
	mockWorkerResponse = abi.NewResponse(200, []byte(`{"id":"invalid-uuid","name":"Alice"}`))

	validReqBody := `{"name":"Alice","email":"alice@example.com"}`
	resp2, err := http.Post(baseURL+"/users", "application/json", bytes.NewBufferString(validReqBody))
	if err != nil {
		t.Fatalf("post request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 502 {
		t.Errorf("expected status 502 Bad Gateway for egress violation, got %d", resp2.StatusCode)
	}

	bodyBytes2, _ := io.ReadAll(resp2.Body)
	if !bytes.Contains(bodyBytes2, []byte("contract.response.invalid")) {
		t.Errorf("expected contract.response.invalid in error: %s", string(bodyBytes2))
	}

	// 3. Valid Request & Response -> 200 OK
	workerReached = false
	validUUID := "123e4567-e89b-12d3-a456-426614174000"
	mockWorkerResponse = abi.NewResponse(200, []byte(`{"id":"`+validUUID+`","name":"Alice"}`))

	resp3, err := http.Post(baseURL+"/users", "application/json", bytes.NewBufferString(validReqBody))
	if err != nil {
		t.Fatalf("post request failed: %v", err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != 200 {
		t.Errorf("expected status 200 OK, got %d", resp3.StatusCode)
	}
	if !workerReached {
		t.Errorf("expected worker to be reached for valid request")
	}

	var user UserResp
	if err := json.NewDecoder(resp3.Body).Decode(&user); err != nil {
		t.Fatalf("decode user failed: %v", err)
	}
	if user.ID != validUUID || user.Name != "Alice" {
		t.Errorf("unexpected user response: %+v", user)
	}
}
