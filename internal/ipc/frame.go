// Package ipc answers: how do the mother process and worker processes exchange framed requests, responses, and lifecycle signals?
package ipc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// FrameType identifies the IPC message classification.
type FrameType string

const (
	FrameTypeHello    FrameType = "hello"
	FrameTypeReady    FrameType = "ready"
	FrameTypeRequest  FrameType = "request"
	FrameTypeResponse FrameType = "response"
	FrameTypePing         FrameType = "ping"
	FrameTypePong         FrameType = "pong"
	FrameTypeDrain        FrameType = "drain"
	FrameTypeStreamStart  FrameType = "stream_start"
	FrameTypeStreamData   FrameType = "stream_data"
	FrameTypeStreamEnd    FrameType = "stream_end"
	FrameTypeStreamCancel FrameType = "stream_cancel"
)

// FrameHeader carries routing, timing, and length metadata for an IPC frame.
type FrameHeader struct {
	Type       FrameType         `json:"type"`
	ID         string            `json:"id"`
	BodyLength int               `json:"body_length"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Frame represents a complete framed IPC message.
type Frame struct {
	Header FrameHeader
	Body   []byte
}

// NewFrame constructs a Frame with computed BodyLength in its header.
func NewFrame(ft FrameType, id string, body []byte) *Frame {
	return &Frame{
		Header: FrameHeader{
			Type:       ft,
			ID:         id,
			BodyLength: len(body),
			Metadata:   make(map[string]string),
		},
		Body: body,
	}
}

// WriteFrame serializes a Frame into w using the 4-byte BE length + JSON header + raw body protocol.
func WriteFrame(w io.Writer, frame *Frame) error {
	if frame == nil {
		return fmt.Errorf("cannot write nil frame")
	}

	frame.Header.BodyLength = len(frame.Body)
	headerBytes, err := json.Marshal(frame.Header)
	if err != nil {
		return fmt.Errorf("marshal frame header: %w", err)
	}

	headerLen := uint32(len(headerBytes))
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], headerLen)

	// 1. Write 4-byte BE header length
	if _, err := w.Write(lenBuf[:]); err != nil {
		return fmt.Errorf("write header length: %w", err)
	}

	// 2. Write JSON header bytes
	if _, err := w.Write(headerBytes); err != nil {
		return fmt.Errorf("write header bytes: %w", err)
	}

	// 3. Write raw body bytes
	if len(frame.Body) > 0 {
		if _, err := w.Write(frame.Body); err != nil {
			return fmt.Errorf("write frame body: %w", err)
		}
	}

	return nil
}

// ReadFrame decodes the next Frame from r.
func ReadFrame(r io.Reader) (*Frame, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}

	headerLen := binary.BigEndian.Uint32(lenBuf[:])
	if headerLen == 0 || headerLen > 16*1024*1024 { // 16MB header sanity limit
		return nil, fmt.Errorf("invalid frame header length: %d", headerLen)
	}

	headerBytes := make([]byte, headerLen)
	if _, err := io.ReadFull(r, headerBytes); err != nil {
		return nil, fmt.Errorf("read frame header: %w", err)
	}

	var header FrameHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("unmarshal frame header: %w", err)
	}

	var body []byte
	if header.BodyLength > 0 {
		if header.BodyLength > 256*1024*1024 { // 256MB body limit
			return nil, fmt.Errorf("frame body length exceeds limit: %d", header.BodyLength)
		}
		body = make([]byte, header.BodyLength)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, fmt.Errorf("read frame body: %w", err)
		}
	}

	return &Frame{
		Header: header,
		Body:   body,
	}, nil
}
