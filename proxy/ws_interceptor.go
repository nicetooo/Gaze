package proxy

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"
	"unicode/utf8"
)

// WSMessageType represents WebSocket frame opcodes
type WSMessageType int

const (
	WSContinuation WSMessageType = 0
	WSText         WSMessageType = 1
	WSBinary       WSMessageType = 2
	WSClose        WSMessageType = 8
	WSPing         WSMessageType = 9
	WSPong         WSMessageType = 10
)

// WSMessage represents a captured WebSocket message
type WSMessage struct {
	ConnectionID string `json:"connectionId"`
	Direction    string `json:"direction"` // "send" (client→server) or "receive" (server→client)
	Type         int    `json:"type"`
	TypeName     string `json:"typeName"`
	Payload      string `json:"payload"`
	PayloadSize  int    `json:"payloadSize"`
	IsBinary     bool   `json:"isBinary"`
	Timestamp    int64  `json:"timestamp"`
	URL          string `json:"url,omitempty"` // WebSocket connection URL (from upgrade request)
	RawPayload   []byte `json:"-"`             // Raw binary payload for protobuf decoding (not serialized)
}

// wsFrameParser parses WebSocket frames from a byte stream
type wsFrameParser struct {
	buf          []byte
	connectionID string
	direction    string
	url          string // WebSocket connection URL
	onMessage    func(msg WSMessage)

	// For fragmented messages
	fragments    []byte
	fragmentType WSMessageType

	// Bytes remaining to discard for an oversized frame being skipped
	skipRemaining uint64
}

func newWSFrameParser(connID, direction, url string, onMessage func(WSMessage)) *wsFrameParser {
	return &wsFrameParser{
		connectionID: connID,
		direction:    direction,
		url:          url,
		onMessage:    onMessage,
	}
}

func (p *wsFrameParser) feed(data []byte) {
	// Discard bytes belonging to an oversized frame being skipped, so the
	// parser stays aligned on the next frame boundary
	if p.skipRemaining > 0 {
		if uint64(len(data)) < p.skipRemaining {
			p.skipRemaining -= uint64(len(data))
			return
		}
		data = data[p.skipRemaining:]
		p.skipRemaining = 0
	}
	p.buf = append(p.buf, data...)
	for p.tryParseFrame() {
	}
}

func (p *wsFrameParser) tryParseFrame() bool {
	if len(p.buf) < 2 {
		return false
	}

	fin := p.buf[0]&0x80 != 0
	opcode := WSMessageType(p.buf[0] & 0x0F)
	masked := p.buf[1]&0x80 != 0
	payloadLen := uint64(p.buf[1] & 0x7F)

	offset := 2

	if payloadLen == 126 {
		if len(p.buf) < 4 {
			return false
		}
		payloadLen = uint64(binary.BigEndian.Uint16(p.buf[2:4]))
		offset = 4
	} else if payloadLen == 127 {
		if len(p.buf) < 10 {
			return false
		}
		payloadLen = binary.BigEndian.Uint64(p.buf[2:10])
		offset = 10
	}

	// RFC 6455: the most significant bit of a 64-bit length must be 0.
	// A violating length means the stream is corrupt or malicious — the
	// total computation below would overflow, so drop the buffer instead.
	if payloadLen&(1<<63) != 0 {
		p.buf = nil
		return false
	}

	// Sanity check: skip absurdly large frames to avoid OOM, consuming
	// exactly this frame's bytes so parsing stays aligned afterwards
	if payloadLen > 16*1024*1024 {
		total := uint64(offset) + payloadLen
		if masked {
			total += 4
		}
		if uint64(len(p.buf)) >= total {
			// Entire oversized frame already buffered: drop just this frame
			p.buf = p.buf[total:]
			return true
		}
		p.skipRemaining = total - uint64(len(p.buf))
		p.buf = nil
		return false
	}

	if masked {
		if len(p.buf) < offset+4 {
			return false
		}
		offset += 4 // masking key
	}

	totalLen := offset + int(payloadLen)
	if len(p.buf) < totalLen {
		return false
	}

	// Extract payload
	payload := make([]byte, payloadLen)
	copy(payload, p.buf[offset:totalLen])

	// Unmask if needed (client→server frames are always masked)
	if masked {
		maskKey := p.buf[offset-4 : offset]
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	// Consume buffer
	p.buf = p.buf[totalLen:]

	// Handle frame
	switch {
	case opcode == WSContinuation:
		p.fragments = append(p.fragments, payload...)
		if fin {
			p.emitMessage(p.fragmentType, p.fragments)
			p.fragments = nil
		}
	case opcode == WSText || opcode == WSBinary:
		if fin {
			p.emitMessage(opcode, payload)
		} else {
			p.fragmentType = opcode
			p.fragments = payload
		}
	default:
		// Control frames: close, ping, pong
		p.emitMessage(opcode, payload)
	}

	return true
}

func (p *wsFrameParser) emitMessage(msgType WSMessageType, payload []byte) {
	if p.onMessage == nil {
		return
	}

	typeName := "unknown"
	isBinary := false
	switch msgType {
	case WSContinuation:
		// Continuation frames shouldn't normally reach emitMessage directly,
		// but handle as safety net (e.g., fragmentType was never set due to
		// missed initial fragment). Auto-detect type from payload content.
		if utf8.Valid(payload) {
			typeName = "text"
		} else {
			typeName = "binary"
			isBinary = true
		}
	case WSText:
		typeName = "text"
	case WSBinary:
		typeName = "binary"
		isBinary = true
	case WSClose:
		typeName = "close"
	case WSPing:
		typeName = "ping"
	case WSPong:
		typeName = "pong"
	}

	// Convert payload to string representation
	payloadStr := ""
	if isBinary {
		if len(payload) > 200 {
			payloadStr = fmt.Sprintf("[binary %d bytes]", len(payload))
		} else {
			payloadStr = fmt.Sprintf("[binary %d bytes] %x", len(payload), payload)
		}
	} else {
		if len(payload) > 8192 {
			payloadStr = string(payload[:8192]) + "..."
		} else {
			payloadStr = string(payload)
		}
	}

	msg := WSMessage{
		ConnectionID: p.connectionID,
		Direction:    p.direction,
		Type:         int(msgType),
		TypeName:     typeName,
		Payload:      payloadStr,
		PayloadSize:  len(payload),
		IsBinary:     isBinary,
		Timestamp:    time.Now().UnixMilli(),
		URL:          p.url,
	}
	// Preserve raw bytes for binary frames (needed for protobuf decoding)
	if isBinary && len(payload) > 0 {
		raw := make([]byte, len(payload))
		copy(raw, payload)
		msg.RawPayload = raw
	}
	p.onMessage(msg)
}

// WSInterceptor wraps a ReadWriteCloser to capture WebSocket frames.
// It implements io.ReadWriteCloser so goproxy can use it for bidirectional WS proxying.
type WSInterceptor struct {
	inner       io.ReadWriteCloser
	readParser  *wsFrameParser // server → client
	writeParser *wsFrameParser // client → server
	mu          sync.Mutex
}

// NewWSInterceptor wraps a server connection to capture WS frames in both directions.
// url is the WebSocket connection URL (from the HTTP upgrade request).
func NewWSInterceptor(inner io.ReadWriteCloser, connID, url string, onMessage func(WSMessage)) *WSInterceptor {
	return &WSInterceptor{
		inner:       inner,
		readParser:  newWSFrameParser(connID, "receive", url, onMessage),
		writeParser: newWSFrameParser(connID, "send", url, onMessage),
	}
}

func (w *WSInterceptor) Read(p []byte) (n int, err error) {
	n, err = w.inner.Read(p)
	if n > 0 {
		w.mu.Lock()
		w.readParser.feed(p[:n])
		w.mu.Unlock()
	}
	return
}

func (w *WSInterceptor) Write(p []byte) (n int, err error) {
	n, err = w.inner.Write(p)
	if n > 0 {
		w.mu.Lock()
		w.writeParser.feed(p[:n])
		w.mu.Unlock()
	}
	return
}

func (w *WSInterceptor) Close() error {
	return w.inner.Close()
}
