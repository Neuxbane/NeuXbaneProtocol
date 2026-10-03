// Package telemetry answers: how are metrics, traces, and structured logs emitted across mother and worker processes?
package telemetry

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

type contextKey string

const (
	TraceIDKey contextKey = "trace_id"
	WorkerKey  contextKey = "worker"
	RouteKey   contextKey = "route"
	BuildIDKey contextKey = "build_id"
)

var defaultLogger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// SetLogger overrides the default slog logger.
func SetLogger(l *slog.Logger) {
	defaultLogger = l
}

// Logger returns the global telemetry logger.
func Logger() *slog.Logger {
	return defaultLogger
}

// WithContext returns a logger enriched with telemetry keys found in ctx.
func WithContext(ctx context.Context) *slog.Logger {
	l := defaultLogger
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		l = l.With("traceId", traceID)
	}
	if worker, ok := ctx.Value(WorkerKey).(string); ok && worker != "" {
		l = l.With("worker", worker)
	}
	if route, ok := ctx.Value(RouteKey).(string); ok && route != "" {
		l = l.With("route", route)
	}
	if buildID, ok := ctx.Value(BuildIDKey).(string); ok && buildID != "" {
		l = l.With("buildId", buildID)
	}
	return l
}

// Metrics tracks runtime counters.
type Metrics struct {
	TotalRequests      atomic.Uint64
	ContractViolations atomic.Uint64
	WorkerReloads      atomic.Uint64
	ActiveWorkers      atomic.Int64
}

// GlobalMetrics provides a process-wide metric collection instance.
var GlobalMetrics = &Metrics{}

// MeasureDuration logs or records the elapsed duration of an operation.
func MeasureDuration(ctx context.Context, op string, start time.Time) {
	elapsed := time.Since(start)
	WithContext(ctx).Debug("operation completed", "op", op, "duration_ms", elapsed.Milliseconds())
}
