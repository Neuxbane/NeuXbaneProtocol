package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/config"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/debug"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/introspect"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/runtime"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/telemetry"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport/rest"
)

func main() {
	addr := flag.String("addr", ":8080", "Server listen address")
	env := flag.String("env", "dev", "Environment (dev, staging, prod)")
	defineDir := flag.String("define", "define", "Path to define/ directory")
	flag.Parse()

	cfg := config.DefaultConfig()
	cfg.Addr = *addr
	cfg.Env = config.Environment(*env)
	cfg.DefineDir = *defineDir

	mother, err := runtime.NewMother(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize mother: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interceptor := introspect.NewInterceptor(cfg.Introspect, "build-init")
	mother.OnWorkerReload(func(buildID string) {
		interceptor.SetBuildID(buildID)
	})

	if err := mother.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start mother: %v\n", err)
		os.Exit(1)
	}

	// Mount REST adapter
	restAdapter := rest.NewAdapter(mother.Dispatch)
	restAdapter.SetIntrospector(interceptor.Intercept)

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen on %s: %v\n", cfg.Addr, err)
		os.Exit(1)
	}

	// Mount Debug server
	_ = debug.NewServer(mother)

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	portStr := cfg.Addr
	if ok {
		portStr = fmt.Sprintf("%d", tcpAddr.Port)
	}
	telemetry.LogService("nxp-server", "listening on http://localhost:%s (port %s, env: %s)", portStr, portStr, cfg.Env)

	go func() {
		if err := restAdapter.Serve(&listener, mother.Table()); err != nil {
			telemetry.LogService("nxp-server", "rest server terminated: %v", err)
		}
	}()

	sig := runtime.WaitForShutdown(ctx)
	telemetry.Logger().Info("caught signal, shutting down", "sig", sig)

	_ = restAdapter.Shutdown(context.Background())
	_ = mother.Shutdown(context.Background())
}
