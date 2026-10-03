package runtime

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// WaitForShutdown blocks until SIGINT, SIGTERM, or context cancellation is received.
func WaitForShutdown(ctx context.Context) os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		return nil
	case sig := <-sigChan:
		return sig
	}
}
