package abi

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
)

// JSONSchema202012URI is the dialect identifier for JSON Schema draft 2020-12.
const JSONSchema202012URI = "https://json-schema.org/draft/2020-12/schema"

var (
	uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// Schema wraps a JSON Schema 2020-12 definition and provides compilation and validation.
type Schema struct {
	SchemaURI            string             `json:"$schema,omitempty"`
	ID                   string             `json:"$id,omitempty"`
	Title                string             `json:"title,omitempty"`
	Description          string             `json:"description,omitempty"`
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Enum                 []any              `json:"enum,omitempty"`
	Minimum              *float64           `json:"minimum,omitempty"`
	Maximum              *float64           `json:"maximum,omitempty"`
	MinLength            *int               `json:"minLength,omitempty"`
	MaxLength            *int               `json:"maxLength,omitempty"`
	Pattern              string             `json:"pattern,omitempty"`
	Ref                  string             `json:"$ref,omitempty"`
	Defs                 map[string]*Schema `json:"$defs,omitempty"`
	AdditionalProperties *bool              `json:"additionalProperties,omitempty"`

	compiledPattern *regexp.Regexp
	raw             []byte
}

// Raw returns the underlying raw JSON schema bytes if available.
func (s *Schema) Raw() []byte {
	if len(s.raw) > 0 {
		return s.raw
	}
	b, _ := json.Marshal(s)
	return b
}

// Compile compiles a JSON Schema from raw JSON bytes.
func Compile(raw []byte) (*Schema, error) {
	var s Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("compile schema: invalid json: %w", err)
	}
	s.raw = raw
	if err := s.compile(); err != nil {
		return nil, err
	}
	return &s, nil
}

// MustCompile compiles raw JSON schema bytes, panicking on syntax or pattern error.
func MustCompile(raw []byte) *Schema {
	s, err := Compile(raw)
	if err != nil {
		panic(err)
	}
	return s
}

func (s *Schema) compile() error {
	if s.Pattern != "" {
		re, err := regexp.Compile(s.Pattern)
		if err != nil {
			return fmt.Errorf("compile schema pattern %q: %w", s.Pattern, err)
		}
		s.compiledPattern = re
	}
	for _, prop := range s.Properties {
		if prop != nil {
			if err := prop.compile(); err != nil {
				return err
			}
		}
	}
	if s.Items != nil {
		if err := s.Items.compile(); err != nil {
			return err
		}
	}
	for _, def := range s.Defs {
		if def != nil {
			if err := def.compile(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Validate validates the given JSON payload against this schema.
func (s *Schema) Validate(data []byte) error {
	if s == nil {
		return nil
	}
	if len(data) == 0 {
		if s.Type == "" || s.Type == "null" {
			return nil
		}
		return errors.New(errors.CodeContractRequestInvalid, "empty payload", 400).
			WithViolation(errors.Violation{
				Path:     "",
				Expected: s.Type,
				Got:      "empty",
				Message:  "payload is empty",
				Code:     errors.CodeContractRequestInvalid,
			})
	}

	var val any
	if err := json.Unmarshal(data, &val); err != nil {
		return errors.New(errors.CodeContractRequestInvalid, "malformed JSON: "+err.Error(), 400).
			WithViolation(errors.Violation{
				Path:     "",
				Expected: "valid json",
				Got:      "malformed json",
				Message:  err.Error(),
				Code:     errors.CodeContractRequestInvalid,
			})
	}

	return s.ValidateValue(val)
}

// ValidateValue validates a decoded Go value (maps, slices, primitives) against the schema.
func (s *Schema) ValidateValue(val any) error {
	if s == nil {
		return nil
	}
	if s.compiledPattern == nil && s.Pattern != "" {
		_ = s.compile()
	}

	violations := s.validateNode(val, "")
	if len(violations) == 0 {
		return nil
	}

	err := errors.New(errors.CodeContractRequestInvalid, "contract validation failed", 400)
	for _, v := range violations {
		err = err.WithViolation(v)
	}
	return err
}

func (s *Schema) validateNode(val any, path string) []errors.Violation {
	var violations []errors.Violation
	if s == nil {
		return nil
	}

	curPath := path
	if curPath == "" {
		curPath = "/"
	}

	// 1. Type validation
	if s.Type != "" {
		ok := false
		gotType := "unknown"
		switch s.Type {
		case "object":
			_, ok = val.(map[string]any)
			gotType = fmt.Sprintf("%T", val)
		case "array":
			_, ok = val.([]any)
			gotType = fmt.Sprintf("%T", val)
		case "string":
			_, ok = val.(string)
			gotType = fmt.Sprintf("%T", val)
		case "number":
			switch val.(type) {
			case float64, float32, int, int64, int32, uint, uint64, uint32:
				ok = true
			}
			gotType = fmt.Sprintf("%T", val)
		case "integer":
			switch v := val.(type) {
			case int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
				ok = true
			case float64:
				if math.Floor(v) == v && !math.IsNaN(v) && !math.IsInf(v, 0) {
					ok = true
				}
			}
			gotType = fmt.Sprintf("%T", val)
		case "boolean":
			_, ok = val.(bool)
			gotType = fmt.Sprintf("%T", val)
		case "null":
			ok = (val == nil)
			gotType = fmt.Sprintf("%T", val)
		default:
			ok = true
		}

		if !ok {
			violations = append(violations, errors.Violation{
				Path:     curPath,
				Expected: "type " + s.Type,
				Got:      gotType,
				Message:  fmt.Sprintf("expected type %s, got %s", s.Type, gotType),
				Code:     errors.CodeContractRequestInvalid,
			})
			return violations
		}
	}

	// 2. Object validation
	if obj, isObj := val.(map[string]any); isObj {
		for _, req := range s.Required {
			if _, exists := obj[req]; !exists {
				p := path + "/" + req
				violations = append(violations, errors.Violation{
					Path:     p,
					Expected: "required field present",
					Got:      "missing",
					Message:  fmt.Sprintf("field %q is required", req),
					Code:     errors.CodeContractRequestInvalid,
				})
			}
		}

		if s.AdditionalProperties != nil && !*s.AdditionalProperties {
			for k := range obj {
				if _, allowed := s.Properties[k]; !allowed {
					p := path + "/" + k
					violations = append(violations, errors.Violation{
						Path:     p,
						Expected: "no additional properties",
						Got:      k,
						Message:  fmt.Sprintf("unknown property %q", k),
						Code:     errors.CodeContractRequestInvalid,
					})
				}
			}
		}

		for propName, propSchema := range s.Properties {
			if propVal, exists := obj[propName]; exists {
				p := path + "/" + propName
				violations = append(violations, propSchema.validateNode(propVal, p)...)
			}
		}
	}

	// 3. Array validation
	if arr, isArr := val.([]any); isArr && s.Items != nil {
		for i, item := range arr {
			p := fmt.Sprintf("%s/%d", path, i)
			violations = append(violations, s.Items.validateNode(item, p)...)
		}
	}

	// 4. String validations
	if str, isStr := val.(string); isStr {
		if s.MinLength != nil && len(str) < *s.MinLength {
			violations = append(violations, errors.Violation{
				Path:     curPath,
				Expected: fmt.Sprintf("minLength >= %d", *s.MinLength),
				Got:      fmt.Sprintf("length %d", len(str)),
				Message:  fmt.Sprintf("string length %d is less than minLength %d", len(str), *s.MinLength),
				Code:     errors.CodeContractRequestInvalid,
			})
		}
		if s.MaxLength != nil && len(str) > *s.MaxLength {
			violations = append(violations, errors.Violation{
				Path:     curPath,
				Expected: fmt.Sprintf("maxLength <= %d", *s.MaxLength),
				Got:      fmt.Sprintf("length %d", len(str)),
				Message:  fmt.Sprintf("string length %d exceeds maxLength %d", len(str), *s.MaxLength),
				Code:     errors.CodeContractRequestInvalid,
			})
		}
		if s.compiledPattern != nil && !s.compiledPattern.MatchString(str) {
			violations = append(violations, errors.Violation{
				Path:     curPath,
				Expected: fmt.Sprintf("pattern %s", s.Pattern),
				Got:      str,
				Message:  fmt.Sprintf("string does not match pattern %s", s.Pattern),
				Code:     errors.CodeContractRequestInvalid,
			})
		}
		if s.Format != "" {
			if err := validateFormat(s.Format, str); err != nil {
				violations = append(violations, errors.Violation{
					Path:     curPath,
					Expected: "format " + s.Format,
					Got:      str,
					Message:  err.Error(),
					Code:     errors.CodeContractRequestInvalid,
				})
			}
		}
	}

	// 5. Number validations
	var num float64
	var isNum bool
	switch v := val.(type) {
	case float64:
		num = v
		isNum = true
	case int:
		num = float64(v)
		isNum = true
	case int64:
		num = float64(v)
		isNum = true
	}
	if isNum {
		if s.Minimum != nil && num < *s.Minimum {
			violations = append(violations, errors.Violation{
				Path:     curPath,
				Expected: fmt.Sprintf("minimum >= %v", *s.Minimum),
				Got:      fmt.Sprintf("%v", num),
				Message:  fmt.Sprintf("value %v is less than minimum %v", num, *s.Minimum),
				Code:     errors.CodeContractRequestInvalid,
			})
		}
		if s.Maximum != nil && num > *s.Maximum {
			violations = append(violations, errors.Violation{
				Path:     curPath,
				Expected: fmt.Sprintf("maximum <= %v", *s.Maximum),
				Got:      fmt.Sprintf("%v", num),
				Message:  fmt.Sprintf("value %v exceeds maximum %v", num, *s.Maximum),
				Code:     errors.CodeContractRequestInvalid,
			})
		}
	}

	// 6. Enum validation
	if len(s.Enum) > 0 {
		matched := false
		for _, e := range s.Enum {
			if fmt.Sprintf("%v", e) == fmt.Sprintf("%v", val) {
				matched = true
				break
			}
		}
		if !matched {
			violations = append(violations, errors.Violation{
				Path:     curPath,
				Expected: fmt.Sprintf("enum %v", s.Enum),
				Got:      fmt.Sprintf("%v", val),
				Message:  fmt.Sprintf("value %v not in enum set", val),
				Code:     errors.CodeContractRequestInvalid,
			})
		}
	}

	return violations
}

func validateFormat(format, val string) error {
	switch format {
	case "date-time":
		if _, err := time.Parse(time.RFC3339, val); err != nil {
			return fmt.Errorf("invalid RFC3339 date-time: %w", err)
		}
	case "email":
		if _, err := mail.ParseAddress(val); err != nil || !strings.Contains(val, "@") {
			return fmt.Errorf("invalid email address")
		}
	case "uuid":
		if !uuidRegex.MatchString(val) {
			return fmt.Errorf("invalid UUID string")
		}
	case "uri", "url":
		u, err := url.ParseRequestURI(val)
		if err != nil || u.Scheme == "" {
			return fmt.Errorf("invalid URI/URL string")
		}
	case "ipv4":
		ip := net.ParseIP(val)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("invalid IPv4 address")
		}
	case "ipv6":
		ip := net.ParseIP(val)
		if ip == nil || ip.To4() != nil {
			return fmt.Errorf("invalid IPv6 address")
		}
	}
	return nil
}
