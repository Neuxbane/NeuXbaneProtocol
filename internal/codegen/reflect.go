package codegen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"strings"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// HandlerInfo contains metadata about a discovered Go handler function.
type HandlerInfo struct {
	FunctionName string
	Domain       string
	TypeName     string
	InputType    string
	ResultType   string
	TypeArgs     []string
	IsGeneric    bool
	InputSchema  *abi.Schema
	ResultSchema *abi.Schema
}

// InspectHandlerSource parses a Go source file and extracts the exported Handler declaration.
func InspectHandlerSource(filePath, src string) (*HandlerInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// 1. Discover all struct types in the file
	structSchemas := make(map[string]*abi.Schema)
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			structSchemas[typeSpec.Name.Name] = parseStructAST(structType)
		}
	}

	// 1b. Resolve named struct references inside field schemas (e.g. []ModelInput)
	// so nested objects carry their real properties instead of a bare object.
	for _, s := range structSchemas {
		resolveNamedRefs(s, structSchemas)
	}

	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}

		name := fn.Name.Name
		// Look for "Handler" or capitalized exported functions
		if name == "Handler" || (len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z') {
			info := parseFuncSignature(fn)
			if info != nil {
				info.FunctionName = name

				// Resolve input schema from input type or bound struct
				if info.InputType != "" {
					info.InputSchema = structSchemas[info.InputType]
				}
				if info.InputSchema == nil {
					info.InputSchema = pickInputSchema(structSchemas)
				}

				// Resolve result schema
				cleanRet := strings.TrimPrefix(info.ResultType, "*")
				if resSchema, ok := structSchemas[cleanRet]; ok {
					info.ResultSchema = resSchema
				}

				return info, nil
			}
		}
	}

	return nil, nil
}

func parseStructAST(st *ast.StructType) *abi.Schema {
	schema := &abi.Schema{
		Type:       "object",
		Properties: make(map[string]*abi.Schema),
		Required:   make([]string, 0),
	}
	if st.Fields == nil {
		return schema
	}
	for _, field := range st.Fields.List {
		var fieldName string
		if len(field.Names) > 0 {
			fieldName = field.Names[0].Name
		}
		if fieldName != "" && !token.IsExported(fieldName) {
			continue
		}

		jsonName := fieldName
		var tagVal string
		if field.Tag != nil {
			tagVal = strings.Trim(field.Tag.Value, "`")
		}

		fieldSchema := parseFieldAST(field.Type)

		if tagVal != "" {
			tag := reflect.StructTag(tagVal)
			if jTag := tag.Get("json"); jTag != "" {
				parts := strings.Split(jTag, ",")
				if parts[0] == "-" {
					continue
				}
				if parts[0] != "" {
					jsonName = parts[0]
				}
			}
			if docTag := tag.Get("doc"); docTag != "" {
				fieldSchema.Description = docTag
			}
			if fmtTag := tag.Get("format"); fmtTag != "" {
				fieldSchema.Format = fmtTag
			}
			if enumTag := tag.Get("enum"); enumTag != "" {
				sep := ","
				if strings.Contains(enumTag, "|") {
					sep = "|"
				}
				for _, item := range strings.Split(enumTag, sep) {
					item = strings.TrimSpace(item)
					if item != "" {
						fieldSchema.Enum = append(fieldSchema.Enum, item)
					}
				}
			}
			if valTag := tag.Get("validate"); valTag != "" {
				rules := strings.Split(valTag, ",")
				for _, r := range rules {
					r = strings.TrimSpace(r)
					if r == "required" {
						schema.Required = append(schema.Required, jsonName)
					} else if strings.HasPrefix(r, "min=") {
						if v, err := strconv.ParseFloat(strings.TrimPrefix(r, "min="), 64); err == nil {
							fieldSchema.Minimum = &v
						}
					} else if strings.HasPrefix(r, "max=") {
						if v, err := strconv.ParseFloat(strings.TrimPrefix(r, "max="), 64); err == nil {
							fieldSchema.Maximum = &v
						}
					} else if strings.HasPrefix(r, "minlen=") {
						if v, err := strconv.Atoi(strings.TrimPrefix(r, "minlen=")); err == nil {
							fieldSchema.MinLength = &v
						}
					} else if strings.HasPrefix(r, "maxlen=") {
						if v, err := strconv.Atoi(strings.TrimPrefix(r, "maxlen=")); err == nil {
							fieldSchema.MaxLength = &v
						}
					} else if strings.HasPrefix(r, "pattern=") {
						fieldSchema.Pattern = strings.TrimPrefix(r, "pattern=")
					} else if strings.HasPrefix(r, "enum=") || strings.HasPrefix(r, "options=") {
						valStr := strings.TrimPrefix(strings.TrimPrefix(r, "enum="), "options=")
						for _, item := range strings.Split(valStr, "|") {
							item = strings.TrimSpace(item)
							if item != "" {
								fieldSchema.Enum = append(fieldSchema.Enum, item)
							}
						}
					} else if r == "email" || r == "uuid" || r == "uri" || r == "url" || r == "date-time" {
						fieldSchema.Format = r
					}
				}
			}
		}

		schema.Properties[jsonName] = fieldSchema
	}
	return schema
}

func parseFieldAST(expr ast.Expr) *abi.Schema {
	if expr == nil {
		return &abi.Schema{Type: "string"}
	}
	switch t := expr.(type) {
	case *ast.StarExpr:
		return parseFieldAST(t.X)
	case *ast.Ident:
		switch t.Name {
		case "string":
			return &abi.Schema{Type: "string"}
		case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
			return &abi.Schema{Type: "integer"}
		case "float32", "float64":
			return &abi.Schema{Type: "number"}
		case "bool":
			return &abi.Schema{Type: "boolean"}
		case "any", "interface{}":
			return &abi.Schema{}
		default:
			// Named type reference; resolved in a second pass.
			return &abi.Schema{Type: "object", Ref: "#/" + t.Name}
		}
	case *ast.ArrayType:
		itemSchema := parseFieldAST(t.Elt)
		return &abi.Schema{
			Type:  "array",
			Items: itemSchema,
		}
	case *ast.MapType:
		return &abi.Schema{Type: "object"}
	default:
		return &abi.Schema{Type: "object"}
	}
}

// resolveNamedRefs walks a schema tree and replaces named type references
// (recorded as $ref "#/TypeName") with the actual struct schema. Cycles are
// broken by leaving the reference in place.
func resolveNamedRefs(s *abi.Schema, defs map[string]*abi.Schema) {
	resolveNamedRefsSeen(s, defs, make(map[string]bool))
}

func resolveNamedRefsSeen(s *abi.Schema, defs map[string]*abi.Schema, seen map[string]bool) {
	if s == nil {
		return
	}
	if s.Items != nil {
		if name, ok := refName(s.Items.Ref); ok {
			if target, found := defs[name]; found && !seen[name] {
				seen[name] = true
				s.Items = target
				resolveNamedRefsSeen(target, defs, seen)
				delete(seen, name)
			}
		} else {
			resolveNamedRefsSeen(s.Items, defs, seen)
		}
	}
	for _, prop := range s.Properties {
		if name, ok := refName(prop.Ref); ok {
			if target, found := defs[name]; found && !seen[name] {
				seen[name] = true
				*prop = *target
				resolveNamedRefsSeen(prop, defs, seen)
				delete(seen, name)
			}
			continue
		}
		resolveNamedRefsSeen(prop, defs, seen)
	}
}

// refName extracts the type name from a "#/TypeName" reference.
func refName(ref string) (string, bool) {
	if strings.HasPrefix(ref, "#/") {
		return strings.TrimPrefix(ref, "#/"), true
	}
	return "", false
}

// pickInputSchema deterministically selects the request payload struct from a
// file's struct declarations. It prefers names containing "request" or
// "payload" over "input", and breaks ties by choosing the shortest name so
// the result is stable across map iteration order.
func pickInputSchema(defs map[string]*abi.Schema) *abi.Schema {
	best := ""
	bestRank := 99
	for name := range defs {
		lower := strings.ToLower(name)
		rank := 99
		switch {
		case strings.Contains(lower, "request"):
			rank = 0
		case strings.Contains(lower, "payload"):
			rank = 1
		case strings.Contains(lower, "input"):
			rank = 2
		}
		if rank == 99 {
			continue
		}
		if rank < bestRank || (rank == bestRank && (best == "" || len(name) < len(best))) {
			best, bestRank = name, rank
		}
	}
	if best == "" {
		return nil
	}
	return defs[best]
}

func parseFuncSignature(fn *ast.FuncDecl) *HandlerInfo {
	if fn.Type == nil || fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		return nil
	}

	// 1. Inspect first parameter: ctx *DOMAIN.Ctx
	firstParam := fn.Type.Params.List[0]
	starExpr, ok := firstParam.Type.(*ast.StarExpr)
	if !ok {
		return nil
	}

	domain := "rest"
	typeName := "Ctx"
	inputType := ""
	var typeArgs []string
	isGeneric := false

	switch expr := starExpr.X.(type) {
	case *ast.SelectorExpr:
		if ident, ok := expr.X.(*ast.Ident); ok {
			domain = ident.Name
		}
		typeName = expr.Sel.Name
	case *ast.IndexExpr: // Generic single param e.g. *mqtt.Ctx[T]
		isGeneric = true
		if sel, ok := expr.X.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				domain = ident.Name
			}
			typeName = sel.Sel.Name
		}
		if typeIdent, ok := expr.Index.(*ast.Ident); ok {
			inputType = typeIdent.Name
			typeArgs = append(typeArgs, typeIdent.Name)
		}
	case *ast.IndexListExpr: // Generic multi param e.g. *mqtt.Ctx[In, Out]
		isGeneric = true
		if sel, ok := expr.X.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				domain = ident.Name
			}
			typeName = sel.Sel.Name
		}
		for _, idx := range expr.Indices {
			if typeIdent, ok := idx.(*ast.Ident); ok {
				if inputType == "" {
					inputType = typeIdent.Name
				}
				typeArgs = append(typeArgs, typeIdent.Name)
			}
		}
	}

	if domain == "abi" {
		switch typeName {
		case "UploadCtx":
			domain = "files"
		case "DownloadCtx":
			domain = "files"
		case "WsCtx":
			domain = "ws"
		case "SessionCtx", "RtcCtx":
			domain = "rtc"
		case "MqttCtx":
			domain = "mqtt"
		default:
			domain = "rest"
		}
	}

	// 2. Inspect return types: (Result, error)
	resultType := ""
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		firstRet := fn.Type.Results.List[0]
		switch ret := firstRet.Type.(type) {
		case *ast.Ident:
			resultType = ret.Name
		case *ast.SelectorExpr:
			resultType = ret.Sel.Name
		case *ast.StarExpr:
			if ident, ok := ret.X.(*ast.Ident); ok {
				resultType = "*" + ident.Name
			}
		}
	}

	return &HandlerInfo{
		FunctionName: fn.Name.Name,
		Domain:       domain,
		TypeName:     typeName,
		InputType:    inputType,
		ResultType:   resultType,
		TypeArgs:     typeArgs,
		IsGeneric:    isGeneric,
	}
}

// buildRouteMetadata assembles the route metadata map from directives. It is
// the single place where declarative, transport-agnostic route hints (such as
// the live-stream poll contract) are lifted into abi.Route.Metadata so that
// generic consumers can read them from the schema bundle.
func buildRouteMetadata(d *Directives) map[string]string {
	if d == nil {
		return nil
	}
	md := make(map[string]string)
	for k, v := range d.StreamContract {
		md["stream."+k] = v
	}
	if len(md) == 0 {
		return nil
	}
	return md
}

// BuildShape constructs an abi.Shape based on transport, directives, and handler types.
func BuildShape(transport abi.Transport, d *Directives, info *HandlerInfo) abi.Shape {
	switch transport {
	case abi.TransportWebSocket, abi.TransportWebRTC:
		return abi.FramesShape{
			In:  &abi.Schema{Type: "string"},
			Out: &abi.Schema{Type: "string"},
		}
	case abi.TransportUDP:
		return abi.DatagramShape{
			MaxSize:  1024,
			Response: abi.DatagramResponseEcho,
		}
	case abi.TransportMQTT:
		topics := make(map[string]string)
		if d.Topic != "" {
			topics["default"] = d.Topic
		}
		return abi.PubSubShape{
			Topics: topics,
			QoS:    d.QoS,
			Retain: d.Retain,
		}
	case abi.TransportKafka, abi.TransportNATS:
		if d.Stream {
			return abi.StreamShape{
				Partitions:    d.Partitions,
				ConsumerGroup: d.Group,
			}
		}
		return abi.PubSubShape{
			Topics: map[string]string{"default": d.Topic},
			QoS:    d.QoS,
		}
	default: // REST, gRPC
		reqSchema := &abi.Schema{Type: "object"}
		if info != nil && info.InputSchema != nil {
			reqSchema = info.InputSchema
		}
		resSchema := &abi.Schema{Type: "object"}
		if info != nil && info.ResultSchema != nil {
			resSchema = info.ResultSchema
		}
		return abi.RequestResponseShape{
			Request: reqSchema,
			Responses: map[int]*abi.Schema{
				200: resSchema,
			},
		}
	}
}
