package ipc

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// SendHello serializes and transmits an abi.Hello frame.
func SendHello(w io.Writer, hello *abi.Hello) error {
	body, err := hello.Marshal()
	if err != nil {
		return fmt.Errorf("marshal hello: %w", err)
	}

	frame := NewFrame(FrameTypeHello, hello.WorkerName, body)
	frame.Header.Metadata["abi_version"] = hello.ABIVersion
	frame.Header.Metadata["worker_name"] = hello.WorkerName
	frame.Header.Metadata["build_id"] = hello.BuildID

	return WriteFrame(w, frame)
}

// ReadHello reads the next frame and unmarshals it as an abi.Hello frame.
func ReadHello(r io.Reader) (*abi.Hello, error) {
	frame, err := ReadFrame(r)
	if err != nil {
		return nil, fmt.Errorf("read hello frame: %w", err)
	}

	if frame.Header.Type != FrameTypeHello {
		return nil, fmt.Errorf("expected frame type hello, got %s", frame.Header.Type)
	}

	return abi.UnmarshalHello(frame.Body)
}

// SendReady serializes and transmits an abi.Ready frame.
func SendReady(w io.Writer, ready *abi.Ready) error {
	if ready.ABIVersion == "" {
		ready.ABIVersion = abi.ABIVersion
	}
	if ready.Status == "" {
		ready.Status = "ready"
	}

	body, err := json.Marshal(ready)
	if err != nil {
		return fmt.Errorf("marshal ready: %w", err)
	}

	frame := NewFrame(FrameTypeReady, "ready", body)
	return WriteFrame(w, frame)
}

// ReadReady reads the next frame and unmarshals it as an abi.Ready frame.
func ReadReady(r io.Reader) (*abi.Ready, error) {
	frame, err := ReadFrame(r)
	if err != nil {
		return nil, fmt.Errorf("read ready frame: %w", err)
	}

	if frame.Header.Type != FrameTypeReady {
		return nil, fmt.Errorf("expected frame type ready, got %s", frame.Header.Type)
	}

	var ready abi.Ready
	if err := json.Unmarshal(frame.Body, &ready); err != nil {
		return nil, fmt.Errorf("unmarshal ready: %w", err)
	}

	return &ready, nil
}
