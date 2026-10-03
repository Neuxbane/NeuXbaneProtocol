package contract_test

import (
	"testing"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/contract"
)

type LoginRequest struct {
	Username string `json:"username" validate:"required,minlen=3"`
	Password string `json:"password" validate:"required,minlen=8"`
}

type LoginResponse struct {
	Token string `json:"token" validate:"required,uuid"`
}

func TestValidateRequest(t *testing.T) {
	reqSchema := abi.SchemaOf[LoginRequest]()
	respSchema := abi.SchemaOf[LoginResponse]()

	route := &abi.Route{
		ID:        "auth.login",
		Transport: abi.TransportREST,
		Method:    "POST",
		Path:      "/auth/login",
		Auth:      "required",
		Scopes:    []string{"login:access"},
		Shape: abi.RequestResponseShape{
			Request: reqSchema,
			Responses: map[int]*abi.Schema{
				200: respSchema,
			},
		},
	}

	// 1. Transport mismatch
	badTransportReq := abi.NewRequest(abi.TransportWebSocket, "POST", "/auth/login")
	err := contract.ValidateRequest(route, badTransportReq)
	if err == nil || err.Code != contract.CodeTransportMismatch {
		t.Fatalf("expected transport mismatch, got: %v", err)
	}

	// 2. Unauthenticated request
	unauthReq := abi.NewRequest(abi.TransportREST, "POST", "/auth/login")
	err = contract.ValidateRequest(route, unauthReq)
	if err == nil || err.Code != contract.CodeRequestUnauthorized {
		t.Fatalf("expected unauthorized, got: %v", err)
	}

	// 3. Forbidden (missing scope)
	authReqNoScope := abi.NewRequest(abi.TransportREST, "POST", "/auth/login")
	authReqNoScope.Identity = &abi.Identity{
		Subject: "user_1",
		Scopes:  []string{"other:scope"},
	}
	err = contract.ValidateRequest(route, authReqNoScope)
	if err == nil || err.Code != contract.CodeRequestForbidden {
		t.Fatalf("expected forbidden, got: %v", err)
	}

	// 4. Invalid body
	authReqBadBody := abi.NewRequest(abi.TransportREST, "POST", "/auth/login")
	authReqBadBody.Identity = &abi.Identity{
		Subject: "user_1",
		Scopes:  []string{"login:access"},
	}
	authReqBadBody.Body = []byte(`{"username":"ab","password":"short"}`)
	err = contract.ValidateRequest(route, authReqBadBody)
	if err == nil || err.Code != contract.CodeRequestInvalid {
		t.Fatalf("expected request invalid, got: %v", err)
	}

	// 5. Valid request
	validReq := abi.NewRequest(abi.TransportREST, "POST", "/auth/login")
	validReq.Identity = &abi.Identity{
		Subject: "user_1",
		Scopes:  []string{"login:access"},
	}
	validReq.Body = []byte(`{"username":"alice","password":"supersecret123"}`)
	err = contract.ValidateRequest(route, validReq)
	if err != nil {
		t.Fatalf("expected valid request to pass, got: %v", err)
	}
}

func TestValidateResponse(t *testing.T) {
	respSchema := abi.SchemaOf[LoginResponse]()

	route := &abi.Route{
		ID:        "auth.login",
		Transport: abi.TransportREST,
		Method:    "POST",
		Path:      "/auth/login",
		Shape: abi.RequestResponseShape{
			Responses: map[int]*abi.Schema{
				200: respSchema,
			},
		},
	}

	// 1. Status not allowed (e.g. 201 when only 200 declared)
	respStatusNotAllowed := abi.NewResponse(201, []byte(`{}`))
	err := contract.ValidateResponse(route, respStatusNotAllowed)
	if err == nil || err.Code != contract.CodeResponseStatusNotAllowed {
		t.Fatalf("expected status not allowed, got: %v", err)
	}
	if err.HTTPStatusCode() != 502 {
		t.Errorf("expected 502 Bad Gateway for egress status violation, got %d", err.HTTPStatusCode())
	}

	// 2. Response body invalid (token is not a UUID)
	respBadBody := abi.NewResponse(200, []byte(`{"token":"not-a-uuid"}`))
	err = contract.ValidateResponse(route, respBadBody)
	if err == nil || err.Code != contract.CodeResponseInvalid {
		t.Fatalf("expected response invalid, got: %v", err)
	}
	if err.HTTPStatusCode() != 502 {
		t.Errorf("expected 502 Bad Gateway for egress body violation, got %d", err.HTTPStatusCode())
	}

	// 3. Valid response
	validResp := abi.NewResponse(200, []byte(`{"token":"123e4567-e89b-12d3-a456-426614174000"}`))
	err = contract.ValidateResponse(route, validResp)
	if err != nil {
		t.Fatalf("expected valid response to pass, got: %v", err)
	}
}

func TestEgressPolicy(t *testing.T) {
	violationErr := errors.New(contract.CodeResponseInvalid, "egress violation", 502)

	// Test quarantine policy
	res := contract.HandleEgressViolation(contract.EgressPolicyQuarantine, "worker-1", violationErr)
	if res == nil || res.Error() != violationErr.Error() {
		t.Errorf("HandleEgressViolation failed: %v", res)
	}
}
