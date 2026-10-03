package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/generated"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/worker"
)

func main() {
	workerName := os.Getenv("WORKER_NAME")
	if workerName == "" {
		workerName = "nxp-worker"
	}
	buildID := os.Getenv("BUILD_ID")
	if buildID == "" {
		buildID = generated.BuildID
	}
	sockPath := os.Getenv("WORKER_SOCK")

	w := worker.NewRuntime(workerName, buildID, sockPath)

	// Register compiled routes and adapters from generated contract
	for _, route := range generated.GeneratedRoutes {
		if handler, ok := generated.GeneratedRegistry[route.ID]; ok {
			w.RegisterHandler(route, handler)
		}
	}

	if err := w.Start(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "worker error: %v\n", err)
		os.Exit(1)
	}
}
