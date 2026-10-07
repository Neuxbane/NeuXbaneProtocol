package contract

import (
	"fmt"
	"strings"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
)

// ValidateRequest enforces ingress contract rules against a route's Shape.
func ValidateRequest(route *abi.Route, req *abi.Request) *errors.Error {
	if route == nil || req == nil {
		return nil
	}

	// 1. Verify transport match
	if route.Transport != "" && req.Transport != "" && route.Transport != req.Transport {
		return errors.New(CodeTransportMismatch,
			fmt.Sprintf("transport mismatch: route requires %s, request used %s", route.Transport, req.Transport), 400).
			WithViolation(FormatViolation("", string(route.Transport), string(req.Transport), "transport mismatch", CodeTransportMismatch))
	}

	// 2. Auth checks
	if route.Auth == "required" {
		if req.Identity == nil || !req.Identity.IsAuthenticated() {
			return errors.New(CodeRequestUnauthorized, "authentication required", 401).
				WithViolation(FormatViolation("", "authenticated identity", "unauthenticated", "missing credentials", CodeRequestUnauthorized))
		}
	}

	// 3. Scope checks
	if len(route.Scopes) > 0 {
		for _, requiredScope := range route.Scopes {
			if req.Identity == nil || !req.Identity.HasScope(requiredScope) {
				return errors.New(CodeRequestForbidden, fmt.Sprintf("missing required scope: %s", requiredScope), 403).
					WithViolation(FormatViolation("", requiredScope, "missing scope", "forbidden", CodeRequestForbidden))
			}
		}
	}

	// 4. Guard dependencies check
	if len(route.Guards) > 0 {
		if req.Identity == nil || !req.Identity.IsAuthenticated() {
			return errors.New(CodeRequestUnauthorized, fmt.Sprintf("guard check failed: requires %v", route.Guards), 401).
				WithViolation(FormatViolation("", strings.Join(route.Guards, ", "), "unauthenticated", "guard check failed", CodeRequestUnauthorized))
		}
		// If specific guard scopes/roles are tagged on identity, ensure compliance
		for _, guardName := range route.Guards {
			if guardName != "" && !req.Identity.HasScope(guardName) && !req.Identity.HasRole(guardName) && !req.Identity.IsAuthenticated() {
				return errors.New(CodeRequestUnauthorized, fmt.Sprintf("guard check failed: missing guard %s", guardName), 401).
					WithViolation(FormatViolation("", guardName, "missing guard", "guard check failed", CodeRequestUnauthorized))
			}
		}
	}

	// 5. Body schema validation against route Shape
	if route.Shape != nil {
		switch s := route.Shape.(type) {
		case abi.RequestResponseShape:
			if s.Request != nil && len(req.Body) > 0 {
				validator, err := CompileSchema(s.Request)
				if err == nil && validator != nil {
					violations := validator.ValidateBytes(req.Body)
					if len(violations) > 0 {
						return NewValidationError(CodeRequestInvalid, "request body failed contract validation", 400, violations...)
					}
				} else {
					// Fallback to abi.Schema validator
					if err := s.Request.Validate(req.Body); err != nil {
						if nxpErr, ok := err.(*errors.Error); ok {
							return nxpErr
						}
						return errors.New(CodeRequestInvalid, err.Error(), 400)
					}
				}
			}
		case abi.FramesShape:
			if s.In != nil && len(req.Body) > 0 {
				if err := s.In.Validate(req.Body); err != nil {
					return errors.New(CodeRequestInvalid, err.Error(), 400)
				}
			}
		case abi.DatagramShape:
			if s.MaxSize > 0 && len(req.Body) > s.MaxSize {
				return errors.New(CodeRequestInvalid, fmt.Sprintf("datagram size %d exceeds max %d", len(req.Body), s.MaxSize), 413)
			}
			if s.Body != nil && len(req.Body) > 0 {
				if err := s.Body.Validate(req.Body); err != nil {
					return errors.New(CodeRequestInvalid, err.Error(), 400)
				}
			}
		case abi.PubSubShape:
			if s.In != nil && len(req.Body) > 0 {
				if err := s.In.Validate(req.Body); err != nil {
					return errors.New(CodeRequestInvalid, err.Error(), 400)
				}
			}
		case abi.StreamShape:
			if s.In != nil && len(req.Body) > 0 {
				if err := s.In.Validate(req.Body); err != nil {
					return errors.New(CodeRequestInvalid, err.Error(), 400)
				}
			}
		}
	}

	return nil
}

// ValidateResponse enforces egress contract rules against a route's Shape.
func ValidateResponse(route *abi.Route, resp *abi.Response) *errors.Error {
	if route == nil || resp == nil {
		return nil
	}

	// If worker returned an error response, egress status checking applies
	if resp.Error != nil {
		return nil
	}

	// Skip schema validation for file streaming, downloads, redirects, or non-JSON payloads
	if resp.Metadata != nil && (resp.Metadata["file_source"] != "" || resp.Metadata["file_path"] != "" || resp.Metadata["redirect"] != "") {
		return nil
	}
	if resp.Status >= 300 && resp.Status < 400 {
		return nil
	}
	if ct := resp.Header("Content-Type"); ct != "" && !strings.Contains(ct, "json") {
		return nil
	}

	if route.Shape != nil {
		switch s := route.Shape.(type) {
		case abi.RequestResponseShape:
			if len(s.Responses) > 0 {
				expectedSchema, allowed := s.Responses[resp.Status]
				if !allowed {
					return errors.New(CodeResponseStatusNotAllowed,
						fmt.Sprintf("response status %d is not allowed by contract", resp.Status), 502).
						WithViolation(FormatViolation("", "declared response status", fmt.Sprintf("%d", resp.Status), "status not allowed", CodeResponseStatusNotAllowed))
				}

				if expectedSchema != nil && len(resp.Body) > 0 {
					validator, err := CompileSchema(expectedSchema)
					if err == nil && validator != nil {
						violations := validator.ValidateBytes(resp.Body)
						if len(violations) > 0 {
							return NewValidationError(CodeResponseInvalid, "response body violates contract schema", 502, violations...)
						}
					} else {
						if err := expectedSchema.Validate(resp.Body); err != nil {
							return errors.New(CodeResponseInvalid, "response body violates contract: "+err.Error(), 502)
						}
					}
				}
			}
		case abi.FramesShape:
			if s.Out != nil && len(resp.Body) > 0 {
				if err := s.Out.Validate(resp.Body); err != nil {
					return errors.New(CodeResponseInvalid, "outbound frame violates contract: "+err.Error(), 502)
				}
			}
		case abi.PubSubShape:
			if s.Out != nil && len(resp.Body) > 0 {
				if err := s.Out.Validate(resp.Body); err != nil {
					return errors.New(CodeResponseInvalid, "outbound message violates contract: "+err.Error(), 502)
				}
			}
		case abi.StreamShape:
			if s.Out != nil && len(resp.Body) > 0 {
				if err := s.Out.Validate(resp.Body); err != nil {
					return errors.New(CodeResponseInvalid, "outbound stream record violates contract: "+err.Error(), 502)
				}
			}
		}
	}

	return nil
}
