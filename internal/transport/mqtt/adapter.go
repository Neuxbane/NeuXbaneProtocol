// Package mqtt answers: how are MQTT topic patterns, QoS levels, retain flags, and message broker connections handled?
package mqtt

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport"
)

func init() {
	transport.RegisterAdapter(NewAdapter(nil))
}

// Packet represents an MQTT PUBLISH packet payload and metadata.
type Packet struct {
	Topic   string
	Payload []byte
	QoS     int
	Retain  bool
}

// Adapter implements transport.Adapter for MQTT.
type Adapter struct {
	dispatcher transport.DispatcherFunc
	table      *router.Table
	subscribers map[string]struct{}
	mu         sync.RWMutex
}

// NewAdapter constructs an initialized MQTT Adapter.
func NewAdapter(dispatcher transport.DispatcherFunc) *Adapter {
	return &Adapter{
		dispatcher:  dispatcher,
		subscribers: make(map[string]struct{}),
	}
}

// SetDispatcher sets the worker dispatcher callback.
func (a *Adapter) SetDispatcher(fn transport.DispatcherFunc) {
	a.dispatcher = fn
}

// Name implements transport.Adapter.
func (a *Adapter) Name() abi.Transport {
	return abi.TransportMQTT
}

// MatchKey implements transport.Adapter.
func (a *Adapter) MatchKey() string {
	return "mqtt:topic/+/data"
}

// Normalize decodes an MQTT Packet into an abi.Request.
func (a *Adapter) Normalize(raw any) (*abi.Request, error) {
	pkt, ok := raw.(Packet)
	if !ok {
		return nil, fmt.Errorf("expected Packet, got %T", raw)
	}

	req := abi.NewRequest(abi.TransportMQTT, "SUB", pkt.Topic)
	req.Topic = pkt.Topic
	req.Body = pkt.Payload
	req.Metadata["qos"] = strconv.Itoa(pkt.QoS)
	req.Metadata["retain"] = strconv.FormatBool(pkt.Retain)

	return req, nil
}

// Render writes an outbound MQTT payload to an io.Writer.
func (a *Adapter) Render(w io.Writer, resp *abi.Response, ctx transport.RenderCtx) error {
	_, err := w.Write(resp.Body)
	return err
}

// MatchTopic checks if an MQTT topic matches a subscription pattern with + or #.
func MatchTopic(pattern, topic string) bool {
	if pattern == topic || pattern == "#" {
		return true
	}

	pParts := strings.Split(pattern, "/")
	tParts := strings.Split(topic, "/")

	for i, p := range pParts {
		if p == "#" {
			return true
		}
		if i >= len(tParts) {
			return false
		}
		if p != "+" && p != tParts[i] {
			return false
		}
	}

	return len(pParts) == len(tParts)
}

// Serve initializes topic listeners.
func (a *Adapter) Serve(listener *net.Listener, table *router.Table) error {
	a.table = table
	return nil
}

// DispatchMessage routes an incoming MQTT message through the dispatcher.
func (a *Adapter) DispatchMessage(ctx context.Context, pkt Packet) (*abi.Response, error) {
	req, err := a.Normalize(pkt)
	if err != nil {
		return nil, err
	}
	if a.dispatcher != nil {
		return a.dispatcher(ctx, req)
	}
	return nil, nil
}

// Shutdown gracefully shuts down the adapter.
func (a *Adapter) Shutdown(ctx context.Context) error {
	return nil
}
