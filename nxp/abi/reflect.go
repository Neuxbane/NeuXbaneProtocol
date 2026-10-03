package abi

import (
	"reflect"
	"strconv"
	"strings"
	"time"
)

// SchemaOf reflects a Go type T into a JSON Schema 2020-12 definition using struct tags.
// It inspects struct tags: `json`, `validate`, `format`, `doc`.
func SchemaOf[T any]() *Schema {
	var zero T
	s := SchemaOfType(reflect.TypeOf(zero))
	if s != nil {
		s.SchemaURI = JSONSchema202012URI
	}
	return s
}

// SchemaOfType reflects a reflect.Type into a JSON Schema definition.
func SchemaOfType(t reflect.Type) *Schema {
	if t == nil {
		return &Schema{Type: "null"}
	}

	// Unwrap pointers
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	// Special stdlib types
	if t == reflect.TypeOf(time.Time{}) {
		return &Schema{
			Type:   "string",
			Format: "date-time",
		}
	}

	switch t.Kind() {
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 { // []byte
			return &Schema{Type: "string", Format: "byte"}
		}
		itemSchema := SchemaOfType(t.Elem())
		return &Schema{
			Type:  "array",
			Items: itemSchema,
		}
	case reflect.Map:
		return &Schema{
			Type: "object",
		}
	case reflect.Struct:
		s := &Schema{
			Type:       "object",
			Properties: make(map[string]*Schema),
			Required:   make([]string, 0),
		}

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}

			// Handle embedded anonymous struct
			if field.Anonymous {
				embedded := SchemaOfType(field.Type)
				if embedded != nil && embedded.Type == "object" {
					for k, prop := range embedded.Properties {
						s.Properties[k] = prop
					}
					s.Required = append(s.Required, embedded.Required...)
				}
				continue
			}

			jsonTag := field.Tag.Get("json")
			if jsonTag == "-" {
				continue
			}

			fieldName := field.Name
			var omitempty bool
			if jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if parts[0] != "" {
					fieldName = parts[0]
				}
				for _, p := range parts[1:] {
					if p == "omitempty" {
						omitempty = true
					}
				}
			}

			fieldSchema := SchemaOfType(field.Type)
			if fieldSchema == nil {
				fieldSchema = &Schema{}
			}

			// doc tag
			if docTag := field.Tag.Get("doc"); docTag != "" {
				fieldSchema.Description = docTag
			}

			// format tag
			if fmtTag := field.Tag.Get("format"); fmtTag != "" {
				fieldSchema.Format = fmtTag
			}

			// validate tag
			if valTag := field.Tag.Get("validate"); valTag != "" {
				rules := strings.Split(valTag, ",")
				for _, r := range rules {
					r = strings.TrimSpace(r)
					if r == "required" {
						s.Required = append(s.Required, fieldName)
					} else if strings.HasPrefix(r, "min=") {
						valStr := strings.TrimPrefix(r, "min=")
						if v, err := strconv.ParseFloat(valStr, 64); err == nil {
							fieldSchema.Minimum = &v
						}
					} else if strings.HasPrefix(r, "max=") {
						valStr := strings.TrimPrefix(r, "max=")
						if v, err := strconv.ParseFloat(valStr, 64); err == nil {
							fieldSchema.Maximum = &v
						}
					} else if strings.HasPrefix(r, "minlen=") {
						valStr := strings.TrimPrefix(r, "minlen=")
						if v, err := strconv.Atoi(valStr); err == nil {
							fieldSchema.MinLength = &v
						}
					} else if strings.HasPrefix(r, "maxlen=") {
						valStr := strings.TrimPrefix(r, "maxlen=")
						if v, err := strconv.Atoi(valStr); err == nil {
							fieldSchema.MaxLength = &v
						}
					} else if strings.HasPrefix(r, "pattern=") {
						valStr := strings.TrimPrefix(r, "pattern=")
						fieldSchema.Pattern = valStr
					} else if r == "email" || r == "uuid" || r == "uri" || r == "url" || r == "ipv4" || r == "ipv6" || r == "date-time" {
						fieldSchema.Format = r
					}
				}
			} else if !omitempty && field.Type.Kind() != reflect.Pointer {
				// Default convention: non-pointer fields without omitempty can be required if tagged
			}

			s.Properties[fieldName] = fieldSchema
		}

		return s
	default:
		return &Schema{}
	}
}
