// Package telemetry answers: how are metrics, traces, and structured logs emitted across mother and worker processes?
package telemetry

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
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

const (
	colorReset       = "\033[0m"
	colorDim         = "\033[90m"
	colorBold        = "\033[1m"
	colorRed         = "\033[31m"
	colorGreen       = "\033[32m"
	colorYellow      = "\033[33m"
	colorBlue        = "\033[34m"
	colorMagenta     = "\033[35m"
	colorCyan        = "\033[36m"
	colorWhite       = "\033[37m"
	colorBoldGreen   = "\033[1;32m"
	colorBoldBlue    = "\033[1;34m"
	colorBoldYellow  = "\033[1;33m"
	colorBoldRed     = "\033[1;31m"
	colorBoldMagenta = "\033[1;35m"
	colorBoldCyan    = "\033[1;36m"
)

func isColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return true
}

// ColorHandler implements slog.Handler with clean colored format:
// time [service-path] ip log
// or
// time [service-path] log
type ColorHandler struct {
	w       io.Writer
	opts    slog.HandlerOptions
	mu      sync.Mutex
	attrs   []slog.Attr
	colored bool
}

// NewColorHandler creates an initialized ColorHandler.
func NewColorHandler(w io.Writer, opts *slog.HandlerOptions) *ColorHandler {
	if w == nil {
		w = os.Stdout
	}
	opt := slog.HandlerOptions{}
	if opts != nil {
		opt = *opts
	}
	return &ColorHandler{
		w:       w,
		opts:    opt,
		colored: isColorEnabled(),
	}
}

func (h *ColorHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *ColorHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	timeStr := r.Time.Format("15:04:05")

	servicePath := "nxp-server"
	ip := ""
	var extraAttrs []string

	for _, a := range h.attrs {
		switch a.Key {
		case "service":
			servicePath = a.Value.String()
		case "ip":
			ip = a.Value.String()
		case "path":
			servicePath = a.Value.String()
		default:
			extraAttrs = append(extraAttrs, fmt.Sprintf("%s=%s", a.Key, a.Value.String()))
		}
	}

	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "service":
			servicePath = a.Value.String()
		case "ip":
			ip = a.Value.String()
		case "path":
			servicePath = a.Value.String()
		case "route":
			servicePath = a.Value.String()
		case "worker":
			if servicePath == "nxp-server" {
				servicePath = a.Value.String()
			}
		default:
			extraAttrs = append(extraAttrs, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
		}
		return true
	})

	msg := r.Message
	if len(extraAttrs) > 0 {
		msg = fmt.Sprintf("%s (%s)", msg, strings.Join(extraAttrs, ", "))
	}

	// Format level indicator if warning or error
	levelPrefix := ""
	if r.Level >= slog.LevelError {
		if h.colored {
			levelPrefix = colorBoldRed + "ERROR " + colorReset
		} else {
			levelPrefix = "ERROR "
		}
	} else if r.Level >= slog.LevelWarn {
		if h.colored {
			levelPrefix = colorBoldYellow + "WARN " + colorReset
		} else {
			levelPrefix = "WARN "
		}
	}

	var sb strings.Builder
	if h.colored {
		sb.WriteString(colorDim + timeStr + colorReset + " ")
		sb.WriteString(formatServicePath(servicePath, true) + " ")
		if ip != "" {
			sb.WriteString(colorYellow + ip + colorReset + " ")
		}
		sb.WriteString(levelPrefix + msg + "\n")
	} else {
		sb.WriteString(timeStr + " [" + servicePath + "] ")
		if ip != "" {
			sb.WriteString(ip + " ")
		}
		sb.WriteString(levelPrefix + msg + "\n")
	}

	_, err := io.WriteString(h.w, sb.String())
	return err
}

func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &ColorHandler{
		w:       h.w,
		opts:    h.opts,
		attrs:   newAttrs,
		colored: h.colored,
	}
}

func (h *ColorHandler) WithGroup(name string) slog.Handler {
	return h
}

func formatServicePath(sp string, colored bool) string {
	if !colored {
		return "[" + sp + "]"
	}

	color := colorBoldCyan
	upper := strings.ToUpper(sp)
	switch {
	case strings.HasPrefix(upper, "GET"):
		color = colorBoldGreen
	case strings.HasPrefix(upper, "POST"):
		color = colorBoldBlue
	case strings.HasPrefix(upper, "PUT"):
		color = colorBoldYellow
	case strings.HasPrefix(upper, "DELETE") || strings.HasPrefix(upper, "DEL"):
		color = colorBoldRed
	case strings.HasPrefix(upper, "PATCH"):
		color = colorBoldMagenta
	case strings.Contains(sp, "mother"):
		color = colorBoldMagenta
	case strings.Contains(sp, "worker"):
		color = colorBoldGreen
	case strings.Contains(sp, "watcher"):
		color = colorBoldYellow
	case strings.Contains(sp, "nxp-server"):
		color = colorBoldCyan
	}

	return color + "[" + sp + "]" + colorReset
}

var defaultLogger = slog.New(NewColorHandler(os.Stdout, &slog.HandlerOptions{
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

// LogService logs a service-level message in the format: time [service-path] log
func LogService(service, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	defaultLogger.With("service", service).Info(msg)
}

// LogRequest logs an incoming transport or HTTP request in the format:
// time [service-path] ip log
func LogRequest(method, path, ip string, status int, duration time.Duration, extra string) {
	colored := isColorEnabled()
	timeStr := time.Now().Format("15:04:05")
	service := method + " " + path

	statusText := http.StatusText(status)
	if statusText == "" {
		statusText = fmt.Sprintf("HTTP %d", status)
	} else {
		statusText = fmt.Sprintf("%d %s", status, statusText)
	}

	// Format duration
	var durStr string
	if duration < time.Millisecond {
		durStr = fmt.Sprintf("%.2fms", float64(duration.Microseconds())/1000.0)
	} else if duration < time.Second {
		durStr = fmt.Sprintf("%.1fms", float64(duration.Microseconds())/1000.0)
	} else {
		durStr = fmt.Sprintf("%.2fs", duration.Seconds())
	}

	var sb strings.Builder
	if colored {
		sb.WriteString(colorDim + timeStr + colorReset + " ")
		sb.WriteString(formatServicePath(service, true) + " ")
		if ip != "" {
			sb.WriteString(colorYellow + ip + colorReset + " ")
		}

		// Color status
		statusColor := colorGreen
		switch {
		case status >= 500:
			statusColor = colorRed
		case status >= 400:
			statusColor = colorYellow
		case status >= 300:
			statusColor = colorCyan
		}
		sb.WriteString(statusColor + statusText + colorReset)
		sb.WriteString(" " + colorDim + "(" + durStr + ")" + colorReset)
		if extra != "" {
			sb.WriteString(" " + extra)
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString(timeStr + " [" + service + "] ")
		if ip != "" {
			sb.WriteString(ip + " ")
		}
		sb.WriteString(statusText + " (" + durStr + ")")
		if extra != "" {
			sb.WriteString(" " + extra)
		}
		sb.WriteString("\n")
	}

	_, _ = io.WriteString(os.Stdout, sb.String())
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
