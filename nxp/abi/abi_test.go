package abi_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
)

func TestABIVersion(t *testing.T) {
	if abi.ABIVersion != "1.0" {
		t.Fatalf("expected ABIVersion 1.0, got %s", abi.ABIVersion)
	}
}

func TestTransportEnum(t *testing.T) {
	transports := abi.AllTransports()
	if len(transports) != 13 {
		t.Errorf("expected 13 transports, got %d", len(transports))
	}

	expected := []abi.Transport{
		abi.TransportREST, abi.TransportGRPC, abi.TransportGraphQL,
		abi.TransportWebSocket, abi.TransportSSE, abi.TransportWebTransport,
		abi.TransportWebRTC, abi.TransportUDP, abi.TransportMQTT,
		abi.TransportNATS, abi.TransportAMQP, abi.TransportRedis, abi.TransportKafka,
	}

	for _, tr := range expected {
		if !tr.IsValid() {
			t.Errorf("expected %s to be valid", tr)
		}
	}

	if abi.Transport("unknown").IsValid() {
		t.Errorf("expected 'unknown' transport to be invalid")
	}
}

func TestIdentity(t *testing.T) {
	var nilIdent *abi.Identity
	if nilIdent.IsAuthenticated() {
		t.Errorf("nil identity should not be authenticated")
	}
	if nilIdent.HasScope("admin") {
		t.Errorf("nil identity should not have scope")
	}

	ident := &abi.Identity{
		Subject: "user_123",
		Issuer:  "https://auth.example.com",
		Scopes:  []string{"read", "write"},
		Roles:   []string{"admin"},
	}

	if !ident.IsAuthenticated() {
		t.Errorf("expected identity to be authenticated")
	}
	if !ident.HasScope("write") || ident.HasScope("delete") {
		t.Errorf("scope check failed")
	}
	if !ident.HasRole("admin") || ident.HasRole("guest") {
		t.Errorf("role check failed")
	}
}

func TestShapes(t *testing.T) {
	schema := &abi.Schema{Type: "string"}

	rr := abi.RequestResponseShape{
		Request:   schema,
		Responses: map[int]*abi.Schema{200: schema},
	}
	if rr.ShapeKind() != abi.ShapeKindRequestResponse {
		t.Errorf("wrong shape kind %s", rr.ShapeKind())
	}

	frames := abi.FramesShape{In: schema, Out: schema}
	if frames.ShapeKind() != abi.ShapeKindFrames {
		t.Errorf("wrong shape kind %s", frames.ShapeKind())
	}

	dg := abi.DatagramShape{
		Body:     schema,
		MaxSize:  1024,
		Response: abi.DatagramResponseEcho,
	}
	if dg.ShapeKind() != abi.ShapeKindDatagram {
		t.Errorf("wrong shape kind %s", dg.ShapeKind())
	}

	ps := abi.PubSubShape{
		Topics: map[string]string{"orders": "orders/+"},
		QoS:    1,
		Retain: true,
		In:     schema,
		Out:    schema,
	}
	if ps.ShapeKind() != abi.ShapeKindPubSub {
		t.Errorf("wrong shape kind %s", ps.ShapeKind())
	}

	stream := abi.StreamShape{
		Partitions:    4,
		ConsumerGroup: "order-processors",
		In:            schema,
		Out:           schema,
	}
	if stream.ShapeKind() != abi.ShapeKindStream {
		t.Errorf("wrong shape kind %s", stream.ShapeKind())
	}
}

func TestRoutePolymorphicSerialization(t *testing.T) {
	routes := []abi.Route{
		{
			ID:        "auth.login",
			Transport: abi.TransportREST,
			Method:    "POST",
			Path:      "/auth/login",
			Auth:      "public",
			Shape: abi.RequestResponseShape{
				Request:   &abi.Schema{Type: "object"},
				Responses: map[int]*abi.Schema{200: {Type: "object"}},
			},
		},
		{
			ID:        "events.stream",
			Transport: abi.TransportKafka,
			Method:    "CONSUME",
			Path:      "events.v1",
			Shape: abi.StreamShape{
				Partitions:    8,
				ConsumerGroup: "event-group",
			},
		},
		{
			ID:        "chat.ws",
			Transport: abi.TransportWebSocket,
			Method:    "CONNECT",
			Path:      "/ws/chat",
			Shape: abi.FramesShape{
				In:  &abi.Schema{Type: "string"},
				Out: &abi.Schema{Type: "string"},
			},
		},
		{
			ID:        "telemetry.udp",
			Transport: abi.TransportUDP,
			Method:    "SEND",
			Path:      "0.0.0.0:9000",
			Shape: abi.DatagramShape{
				MaxSize:  512,
				Response: abi.DatagramResponseNone,
			},
		},
		{
			ID:        "mqtt.sensors",
			Transport: abi.TransportMQTT,
			Method:    "SUB",
			Path:      "sensors/+/temperature",
			Shape: abi.PubSubShape{
				Topics: map[string]string{"temp": "sensors/+/temperature"},
				QoS:    1,
			},
		},
	}

	for _, original := range routes {
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal route %s failed: %v", original.ID, err)
		}

		var restored abi.Route
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("unmarshal route %s failed: %v", original.ID, err)
		}

		if restored.ID != original.ID {
			t.Errorf("route ID mismatch: got %s, want %s", restored.ID, original.ID)
		}
		if restored.Shape.ShapeKind() != original.Shape.ShapeKind() {
			t.Errorf("route %s shape kind mismatch: got %s, want %s",
				original.ID, restored.Shape.ShapeKind(), original.Shape.ShapeKind())
		}
	}
}

func TestHelloFrame(t *testing.T) {
	hello := &abi.Hello{
		ABIVersion: abi.ABIVersion,
		WorkerName: "worker-auth",
		BuildID:    "bld_12345",
		PID:        4242,
		Routes: []abi.Route{
			{
				ID:        "auth.index",
				Transport: abi.TransportREST,
				Method:    "GET",
				Path:      "/auth",
				Shape:     abi.RequestResponseShape{},
			},
		},
	}

	data, err := hello.Marshal()
	if err != nil {
		t.Fatalf("marshal hello failed: %v", err)
	}

	unmarshaled, err := abi.UnmarshalHello(data)
	if err != nil {
		t.Fatalf("unmarshal hello failed: %v", err)
	}

	if unmarshaled.WorkerName != "worker-auth" {
		t.Errorf("expected worker-auth, got %s", unmarshaled.WorkerName)
	}
	if len(unmarshaled.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(unmarshaled.Routes))
	}

	// Test ABI version mismatch
	badHello := `{"abi_version":"99.0","worker_name":"bad"}`
	_, err = abi.UnmarshalHello([]byte(badHello))
	if err == nil {
		t.Fatal("expected error on ABI version mismatch, got nil")
	}
}

func TestRequestResponseHelpers(t *testing.T) {
	req := abi.NewRequest(abi.TransportREST, "POST", "/users")
	req.SetHeader("Content-Type", "application/json")
	req.SetHeader("x-api-key", "secret123")
	req.Params["id"] = "42"
	req.Query["filter"] = []string{"active", "verified"}
	req.Body = []byte(`{"name":"Alice","email":"alice@example.com"}`)

	if req.Header("content-type") != "application/json" {
		t.Errorf("header lookup failed: %s", req.Header("content-type"))
	}
	if req.Header("X-API-KEY") != "secret123" {
		t.Errorf("case-insensitive header lookup failed: %s", req.Header("X-API-KEY"))
	}
	if req.Param("id") != "42" {
		t.Errorf("param lookup failed: %s", req.Param("id"))
	}
	if req.QueryParam("filter") != "active" {
		t.Errorf("query param lookup failed: %s", req.QueryParam("filter"))
	}
	if len(req.QueryParams("filter")) != 2 {
		t.Errorf("query params lookup failed: %d", len(req.QueryParams("filter")))
	}

	var payload struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := req.BindJSON(&payload); err != nil {
		t.Fatalf("bind json failed: %v", err)
	}
	if payload.Name != "Alice" || payload.Email != "alice@example.com" {
		t.Errorf("payload fields mismatch: %+v", payload)
	}

	// Response helpers
	resp, err := abi.NewJSONResponse(201, payload)
	if err != nil {
		t.Fatalf("new json response failed: %v", err)
	}
	if resp.Status != 201 {
		t.Errorf("expected status 201, got %d", resp.Status)
	}
	if resp.Header("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("expected json content type, got %s", resp.Header("Content-Type"))
	}

	// Error response helper
	errResp := abi.NewErrorResponse(errors.ErrNotFound)
	if errResp.Status != 404 {
		t.Errorf("expected status 404, got %d", errResp.Status)
	}
	if errResp.Error == nil || errResp.Error.Code != errors.CodeNotFound {
		t.Errorf("error response mapping failed")
	}
}

type SampleUser struct {
	ID        string    `json:"id" validate:"required,uuid" doc:"Unique user UUID"`
	Name      string    `json:"name" validate:"required,minlen=2,maxlen=50" doc:"User full name"`
	Email     string    `json:"email" validate:"required,email" format:"email"`
	Age       int       `json:"age" validate:"min=0,max=120"`
	CreatedAt time.Time `json:"created_at"`
	Ignored   string    `json:"-"`
}

func TestSchemaOfAndValidate(t *testing.T) {
	schema := abi.SchemaOf[SampleUser]()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if schema.SchemaURI != abi.JSONSchema202012URI {
		t.Errorf("expected schema URI %s, got %s", abi.JSONSchema202012URI, schema.SchemaURI)
	}
	if schema.Type != "object" {
		t.Errorf("expected object type, got %s", schema.Type)
	}

	// Required fields check
	reqMap := make(map[string]bool)
	for _, r := range schema.Required {
		reqMap[r] = true
	}
	if !reqMap["id"] || !reqMap["name"] || !reqMap["email"] {
		t.Errorf("required fields missing: %v", schema.Required)
	}

	// Field properties check
	idProp := schema.Properties["id"]
	if idProp == nil || idProp.Format != "uuid" || idProp.Description != "Unique user UUID" {
		t.Errorf("id property invalid: %+v", idProp)
	}

	nameProp := schema.Properties["name"]
	if nameProp == nil || *nameProp.MinLength != 2 || *nameProp.MaxLength != 50 {
		t.Errorf("name property invalid: %+v", nameProp)
	}

	ageProp := schema.Properties["age"]
	if ageProp == nil || *ageProp.Minimum != 0 || *ageProp.Maximum != 120 {
		t.Errorf("age property invalid: %+v", ageProp)
	}

	if _, exists := schema.Properties["Ignored"]; exists {
		t.Errorf("ignored field should not be in properties")
	}

	// Valid payload
	validPayload := []byte(`{
		"id": "123e4567-e89b-12d3-a456-426614174000",
		"name": "Bob",
		"email": "bob@example.com",
		"age": 30,
		"created_at": "2026-10-03T12:00:00Z"
	}`)

	if err := schema.Validate(validPayload); err != nil {
		t.Fatalf("expected valid payload to pass, got: %v", err)
	}

	// Invalid payload: missing required field 'name', invalid uuid, invalid email, age too high
	invalidPayload := []byte(`{
		"id": "not-a-uuid",
		"email": "invalid-email",
		"age": 150
	}`)

	err := schema.Validate(invalidPayload)
	if err == nil {
		t.Fatal("expected invalid payload to fail validation, got nil")
	}

	nxpErr, ok := err.(*errors.Error)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T", err)
	}

	if len(nxpErr.Violations) < 3 {
		t.Errorf("expected at least 3 violations, got %d: %v", len(nxpErr.Violations), nxpErr.Violations)
	}
}

func TestMustCompile(t *testing.T) {
	raw := []byte(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"code": {
				"type": "string",
				"pattern": "^[A-Z]{3}$"
			}
		},
		"required": ["code"]
	}`)

	compiled := abi.MustCompile(raw)
	if compiled == nil {
		t.Fatal("MustCompile returned nil")
	}

	if err := compiled.Validate([]byte(`{"code":"ABC"}`)); err != nil {
		t.Errorf("expected ABC to pass validation, got: %v", err)
	}

	if err := compiled.Validate([]byte(`{"code":"abcd"}`)); err == nil {
		t.Errorf("expected abcd to fail pattern validation")
	}
}
