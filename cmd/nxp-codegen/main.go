package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/codegen"
)

func main() {
	defineDir := flag.String("define", "define", "Path to define/ directory tree")
	modulePath := flag.String("module", "github.com/Neuxbane/NeuXbaneProtocol", "Go module path")
	outPath := flag.String("out", "internal/generated/routes_gen.go", "Path to write generated routes Go file")
	buildIDPath := flag.String("build-id", ".nxp-build-id", "Path to write build ID file")
	flag.Parse()

	if _, err := os.Stat(*defineDir); os.IsNotExist(err) {
		fmt.Printf("define directory %q not found, creating placeholder\n", *defineDir)
		_ = os.MkdirAll(*defineDir, 0755)
	}

	buildID, err := codegen.GenerateProject(*defineDir, *modulePath, *outPath, *buildIDPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codegen error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("nxp-codegen completed: build ID %s, emitted %s\n", buildID, *outPath)
}
