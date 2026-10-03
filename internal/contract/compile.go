package contract

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// Validator encapsulates a compiled fast JSON Schema validator.
type Validator struct {
	schema *jsonschema.Schema
	raw    []byte
}

var (
	compilerMu sync.Mutex
	cacheMu    sync.RWMutex
	cache      = make(map[string]*Validator)
)

// CompileSchema compiles an *abi.Schema into a fast Validator using santhosh-tekuri/jsonschema/v6.
func CompileSchema(s *abi.Schema) (*Validator, error) {
	if s == nil {
		return nil, nil
	}

	raw := s.Raw()
	if len(raw) == 0 {
		var err error
		raw, err = json.Marshal(s)
		if err != nil {
			return nil, fmt.Errorf("marshal schema: %w", err)
		}
	}

	key := string(raw)
	cacheMu.RLock()
	v, exists := cache[key]
	cacheMu.RUnlock()
	if exists {
		return v, nil
	}

	compilerMu.Lock()
	defer compilerMu.Unlock()

	// Double-check after lock
	cacheMu.RLock()
	v, exists = cache[key]
	cacheMu.RUnlock()
	if exists {
		return v, nil
	}

	c := jsonschema.NewCompiler()
	url := fmt.Sprintf("urn:nxp:schema:%d.json", len(cache)+1)
	if err := c.AddResource(url, strings.NewReader(string(raw))); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}

	sch, err := c.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("compile fast schema: %w", err)
	}

	v = &Validator{
		schema: sch,
		raw:    raw,
	}

	cacheMu.Lock()
	cache[key] = v
	cacheMu.Unlock()

	return v, nil
}

// ValidateBytes validates raw JSON bytes against the compiled schema.
func (v *Validator) ValidateBytes(data []byte) []Violation {
	if v == nil || v.schema == nil || len(data) == 0 {
		return nil
	}

	var val any
	if err := json.Unmarshal(data, &val); err != nil {
		return []Violation{
			FormatViolation("", "valid json", "malformed json", err.Error(), CodeRequestInvalid),
		}
	}

	return v.ValidateValue(val)
}

// ValidateValue validates an unmarshaled JSON value against the compiled schema.
func (v *Validator) ValidateValue(val any) []Violation {
	if v == nil || v.schema == nil {
		return nil
	}

	err := v.schema.Validate(val)
	if err == nil {
		return nil
	}

	var violations []Violation
	if valErr, ok := err.(*jsonschema.ValidationError); ok {
		violations = extractViolations(valErr)
	} else {
		violations = append(violations, FormatViolation("", "schema compliance", "validation failed", err.Error(), CodeRequestInvalid))
	}

	return violations
}

func extractViolations(ve *jsonschema.ValidationError) []Violation {
	var list []Violation

	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if e == nil {
			return
		}
		if len(e.Causes) == 0 {
			loc := "/" + strings.Join(e.InstanceLocation, "/")
			list = append(list, FormatViolation(
				loc,
				fmt.Sprintf("%v", e.ErrorKind),
				"invalid",
				e.Error(),
				CodeRequestInvalid,
			))
		}
		for _, child := range e.Causes {
			walk(child)
		}
	}

	walk(ve)
	return list
}
