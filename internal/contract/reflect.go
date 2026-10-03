package contract

import (
	"reflect"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// SchemaOf reflects a Go type T into a JSON Schema 2020-12 definition.
func SchemaOf[T any]() *abi.Schema {
	return abi.SchemaOf[T]()
}

// SchemaOfType reflects a reflect.Type into a JSON Schema definition.
func SchemaOfType(t reflect.Type) *abi.Schema {
	return abi.SchemaOfType(t)
}
