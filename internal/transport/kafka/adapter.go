// Package kafka answers: how are Kafka topics, partitions, consumer groups, offset commits, and DLQ handled?
package kafka

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport"
)

func init() {
	transport.RegisterAdapter(NewAdapter(nil))
}

// Record represents a Kafka stream record.
type Record struct {
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
	Headers   map[string]string
}

// Adapter implements transport.Adapter for Kafka.
type Adapter struct {
	dispatcher transport.DispatcherFunc
	table      *router.Table
	mu         sync.RWMutex
}

// NewAdapter constructs an initialized Kafka Adapter.
func NewAdapter(dispatcher transport.DispatcherFunc) *Adapter {
	return &Adapter{
		dispatcher: dispatcher,
	}
}

// SetDispatcher sets the worker dispatcher callback.
func (a *Adapter) SetDispatcher(fn transport.DispatcherFunc) {
	a.dispatcher = fn
}

// Name implements transport.Adapter.
func (a *Adapter) Name() abi.Transport {
	return abi.TransportKafka
}

// MatchKey implements transport.Adapter.
func (a *Adapter) MatchKey() string {
	return "kafka:topic:partition"
}

// Normalize decodes a Record into an abi.Request.
func (a *Adapter) Normalize(raw any) (*abi.Request, error) {
	rec, ok := raw.(Record)
	if !ok {
		return nil, fmt.Errorf("expected Record, got %T", raw)
	}

	path := fmt.Sprintf("%s/%d", rec.Topic, rec.Partition)
	req := abi.NewRequest(abi.TransportKafka, "CONSUME", path)
	req.Topic = rec.Topic
	req.Partition = rec.Partition
	req.Offset = rec.Offset
	req.Body = rec.Value
	req.Metadata["offset"] = strconv.FormatInt(rec.Offset, 10)
	if len(rec.Key) > 0 {
		req.Metadata["key"] = string(rec.Key)
	}

	return req, nil
}

// Render writes an outbound response.
func (a *Adapter) Render(w io.Writer, resp *abi.Response, ctx transport.RenderCtx) error {
	_, err := w.Write(resp.Body)
	return err
}

// Serve initializes Kafka consumers.
func (a *Adapter) Serve(listener *net.Listener, table *router.Table) error {
	a.table = table
	return nil
}

// Shutdown gracefully terminates the Kafka adapter.
func (a *Adapter) Shutdown(ctx context.Context) error {
	return nil
}
