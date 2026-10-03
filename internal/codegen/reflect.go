package codegen

import (
	"go/ast"
	"go/parser"
	"go/token"

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
}

// InspectHandlerSource parses a Go source file and extracts the exported Handler declaration.
func InspectHandlerSource(filePath, src string) (*HandlerInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, err
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
				return info, nil
			}
		}
	}

	return nil, nil
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
		return abi.RequestResponseShape{
			Request: &abi.Schema{Type: "object"},
			Responses: map[int]*abi.Schema{
				200: {Type: "object"},
			},
		}
	}
}
