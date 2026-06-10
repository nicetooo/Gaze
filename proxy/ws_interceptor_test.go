package proxy

import (
	"bytes"
	"encoding/binary"
	"io"
	"sync"
	"testing"
)

// ==================== wsFrameParser ====================

// buildWSFrame constructs a raw WebSocket frame for testing.
// opcode: text=1, binary=2, close=8, ping=9, pong=10
func buildWSFrame(fin bool, opcode byte, masked bool, payload []byte) []byte {
	var buf bytes.Buffer

	b0 := opcode
	if fin {
		b0 |= 0x80
	}
	buf.WriteByte(b0)

	// Payload length
	maskBit := byte(0)
	if masked {
		maskBit = 0x80
	}

	pLen := len(payload)
	if pLen < 126 {
		buf.WriteByte(maskBit | byte(pLen))
	} else if pLen < 65536 {
		buf.WriteByte(maskBit | 126)
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(pLen))
		buf.Write(b)
	} else {
		buf.WriteByte(maskBit | 127)
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(pLen))
		buf.Write(b)
	}

	if masked {
		maskKey := []byte{0x12, 0x34, 0x56, 0x78}
		buf.Write(maskKey)
		maskedPayload := make([]byte, len(payload))
		for i := range payload {
			maskedPayload[i] = payload[i] ^ maskKey[i%4]
		}
		buf.Write(maskedPayload)
	} else {
		buf.Write(payload)
	}

	return buf.Bytes()
}

func TestWSFrameParser_TextFrame(t *testing.T) {
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	frame := buildWSFrame(true, 1, false, []byte("hello world"))
	parser.feed(frame)

	if len(received) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(received))
	}
	if received[0].TypeName != "text" {
		t.Errorf("Expected type 'text', got %q", received[0].TypeName)
	}
	if received[0].Payload != "hello world" {
		t.Errorf("Expected payload 'hello world', got %q", received[0].Payload)
	}
	if received[0].Direction != "receive" {
		t.Errorf("Expected direction 'receive', got %q", received[0].Direction)
	}
	if received[0].ConnectionID != "conn1" {
		t.Errorf("Expected connectionID 'conn1', got %q", received[0].ConnectionID)
	}
	if received[0].IsBinary {
		t.Error("Text frame should not be binary")
	}
}

func TestWSFrameParser_BinaryFrame(t *testing.T) {
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	frame := buildWSFrame(true, 2, false, []byte{0x00, 0x01, 0x02})
	parser.feed(frame)

	if len(received) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(received))
	}
	if received[0].TypeName != "binary" {
		t.Errorf("Expected type 'binary', got %q", received[0].TypeName)
	}
	if !received[0].IsBinary {
		t.Error("Binary frame should be binary")
	}
	if received[0].PayloadSize != 3 {
		t.Errorf("Expected payloadSize 3, got %d", received[0].PayloadSize)
	}
}

func TestWSFrameParser_MaskedFrame(t *testing.T) {
	var received []WSMessage
	parser := newWSFrameParser("conn1", "send", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	// Client-to-server frames are always masked
	frame := buildWSFrame(true, 1, true, []byte("masked text"))
	parser.feed(frame)

	if len(received) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(received))
	}
	if received[0].Payload != "masked text" {
		t.Errorf("Expected 'masked text' after unmasking, got %q", received[0].Payload)
	}
}

func TestWSFrameParser_ControlFrames(t *testing.T) {
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	// Ping
	parser.feed(buildWSFrame(true, 9, false, []byte("ping")))
	// Pong
	parser.feed(buildWSFrame(true, 10, false, []byte("pong")))
	// Close
	parser.feed(buildWSFrame(true, 8, false, []byte{0x03, 0xe8})) // close code 1000

	if len(received) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(received))
	}
	if received[0].TypeName != "ping" {
		t.Errorf("Expected 'ping', got %q", received[0].TypeName)
	}
	if received[1].TypeName != "pong" {
		t.Errorf("Expected 'pong', got %q", received[1].TypeName)
	}
	if received[2].TypeName != "close" {
		t.Errorf("Expected 'close', got %q", received[2].TypeName)
	}
}

func TestWSFrameParser_FragmentedMessage(t *testing.T) {
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	// First fragment (text, not fin)
	parser.feed(buildWSFrame(false, 1, false, []byte("hello ")))
	if len(received) != 0 {
		t.Error("Should not emit message before final fragment")
	}

	// Continuation (fin)
	parser.feed(buildWSFrame(true, 0, false, []byte("world")))
	if len(received) != 1 {
		t.Fatalf("Expected 1 complete message, got %d", len(received))
	}
	if received[0].Payload != "hello world" {
		t.Errorf("Expected 'hello world', got %q", received[0].Payload)
	}
	if received[0].TypeName != "text" {
		t.Errorf("Fragmented text should have type 'text', got %q", received[0].TypeName)
	}
}

func TestWSFrameParser_ContinuationWithoutInitialFragment(t *testing.T) {
	// Edge case: continuation frame arrives when fragmentType was never set (zero value = WSContinuation).
	// This can happen if the parser starts mid-stream and misses the initial fragment.
	// The fix should auto-detect type from payload instead of returning "unknown".
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	// Simulate: directly feed a continuation frame with fin=true (no preceding initial fragment)
	// The fragmentType is 0 (WSContinuation) because it was never set.
	parser.fragments = []byte("partial ")
	// fragmentType stays at zero value (WSContinuation = 0)
	parser.feed(buildWSFrame(true, 0, false, []byte("data")))

	if len(received) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(received))
	}
	// With the fix, valid UTF-8 payload should be detected as "text", not "unknown"
	if received[0].TypeName == "unknown" {
		t.Errorf("Type should not be 'unknown' for valid UTF-8 continuation payload, got %q", received[0].TypeName)
	}
	if received[0].TypeName != "text" {
		t.Errorf("Expected type 'text' for UTF-8 continuation payload, got %q", received[0].TypeName)
	}
	if received[0].Payload != "partial data" {
		t.Errorf("Expected 'partial data', got %q", received[0].Payload)
	}
}

func TestWSFrameParser_ContinuationBinaryAutoDetect(t *testing.T) {
	// When fragmentType is 0 and payload is not valid UTF-8, it should be detected as "binary"
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	// Feed binary data through continuation path
	parser.fragments = []byte{0x80, 0xFF, 0xFE}
	parser.feed(buildWSFrame(true, 0, false, []byte{0x00, 0xC0, 0xAF}))

	if len(received) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(received))
	}
	if received[0].TypeName != "binary" {
		t.Errorf("Expected type 'binary' for invalid UTF-8 continuation payload, got %q", received[0].TypeName)
	}
	if !received[0].IsBinary {
		t.Error("Should be marked as binary")
	}
}

func TestWSFrameParser_MultipleFrames(t *testing.T) {
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	// Feed multiple frames at once
	var data []byte
	data = append(data, buildWSFrame(true, 1, false, []byte("msg1"))...)
	data = append(data, buildWSFrame(true, 1, false, []byte("msg2"))...)
	data = append(data, buildWSFrame(true, 1, false, []byte("msg3"))...)

	parser.feed(data)

	if len(received) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(received))
	}
	if received[0].Payload != "msg1" || received[1].Payload != "msg2" || received[2].Payload != "msg3" {
		t.Error("Messages should be msg1, msg2, msg3")
	}
}

func TestWSFrameParser_PartialFrame(t *testing.T) {
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	frame := buildWSFrame(true, 1, false, []byte("complete message"))

	// Feed first half
	mid := len(frame) / 2
	parser.feed(frame[:mid])
	if len(received) != 0 {
		t.Error("Should not emit message from partial frame")
	}

	// Feed second half
	parser.feed(frame[mid:])
	if len(received) != 1 {
		t.Fatalf("Expected 1 message after feeding complete frame, got %d", len(received))
	}
	if received[0].Payload != "complete message" {
		t.Errorf("Expected 'complete message', got %q", received[0].Payload)
	}
}

func TestWSFrameParser_ExtendedPayloadLength16(t *testing.T) {
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	// 126+ bytes payload uses 16-bit extended length
	payload := make([]byte, 200)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	frame := buildWSFrame(true, 2, false, payload)
	parser.feed(frame)

	if len(received) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(received))
	}
	if received[0].PayloadSize != 200 {
		t.Errorf("Expected payload size 200, got %d", received[0].PayloadSize)
	}
}

func TestWSFrameParser_NilCallback(t *testing.T) {
	// Should not panic with nil callback
	parser := newWSFrameParser("conn1", "receive", "", nil)
	frame := buildWSFrame(true, 1, false, []byte("test"))
	parser.feed(frame) // should not panic
}

func TestWSFrameParser_OversizedFrame(t *testing.T) {
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	// Build a frame header claiming > 16MB payload (will be rejected)
	header := []byte{0x81, 127} // fin + text, 8-byte length
	lenBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(lenBytes, 20*1024*1024) // 20MB
	header = append(header, lenBytes...)
	parser.feed(header)

	if len(received) != 0 {
		t.Error("Oversized frame should be rejected")
	}
}

func TestWSFrameParser_MaxUint64LengthFrame(t *testing.T) {
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	// RFC 6455 violation: 64-bit length with the MSB set. The naive
	// total = offset + payloadLen computation would overflow uint64 and
	// consume almost nothing — must be rejected outright instead.
	header := []byte{0x81, 127}
	lenBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(lenBytes, ^uint64(0)) // 2^64-1
	header = append(header, lenBytes...)
	parser.feed(append(header, []byte("garbage")...))

	if len(received) != 0 {
		t.Fatalf("Protocol-violating frame must not emit messages, got %d", len(received))
	}

	// Parser must still handle a clean frame afterwards (buffer was dropped)
	parser.feed(buildWSFrame(true, 1, false, []byte("recovered")))
	if len(received) != 1 || received[0].Payload != "recovered" {
		t.Fatalf("Expected recovery after protocol violation, got %v", received)
	}
}

func TestWSFrameParser_OversizedFrameThenNormalFrame(t *testing.T) {
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	// Build a complete 20MB frame and feed it in chunks, followed by a normal
	// frame. The parser must skip exactly the oversized frame and stay aligned.
	bigPayload := make([]byte, 20*1024*1024)
	bigFrame := buildWSFrame(true, 1, false, bigPayload)
	const chunkSize = 32 * 1024
	for i := 0; i < len(bigFrame); i += chunkSize {
		end := i + chunkSize
		if end > len(bigFrame) {
			end = len(bigFrame)
		}
		parser.feed(bigFrame[i:end])
	}

	if len(received) != 0 {
		t.Fatalf("Oversized frame should be skipped, got %d messages", len(received))
	}

	normalFrame := buildWSFrame(true, 1, false, []byte("hello after big frame"))
	parser.feed(normalFrame)

	if len(received) != 1 {
		t.Fatalf("Expected 1 message after oversized frame, got %d", len(received))
	}
	if received[0].Payload != "hello after big frame" {
		t.Errorf("Expected payload 'hello after big frame', got %q", received[0].Payload)
	}
}

// ==================== New Protobuf-related tests ====================

func TestWSFrameParser_BinaryFramePreservesRawPayload(t *testing.T) {
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	payload := []byte{0x08, 0x96, 0x01, 0x12, 0x05, 0x68, 0x65, 0x6c, 0x6c, 0x6f}
	frame := buildWSFrame(true, 2, false, payload)
	parser.feed(frame)

	if len(received) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(received))
	}
	if received[0].RawPayload == nil {
		t.Fatal("Binary frame should have RawPayload set")
	}
	if !bytes.Equal(received[0].RawPayload, payload) {
		t.Errorf("RawPayload mismatch: got %v, want %v", received[0].RawPayload, payload)
	}
}

func TestWSFrameParser_TextFrameNoRawPayload(t *testing.T) {
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", "", func(msg WSMessage) {
		received = append(received, msg)
	})

	frame := buildWSFrame(true, 1, false, []byte("text message"))
	parser.feed(frame)

	if len(received) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(received))
	}
	if received[0].RawPayload != nil {
		t.Errorf("Text frame should not have RawPayload, got %v", received[0].RawPayload)
	}
}

func TestWSFrameParser_URLPropagation(t *testing.T) {
	testURL := "wss://example.com/ws/chat"
	var received []WSMessage
	parser := newWSFrameParser("conn1", "receive", testURL, func(msg WSMessage) {
		received = append(received, msg)
	})

	frame := buildWSFrame(true, 1, false, []byte("hello"))
	parser.feed(frame)

	if len(received) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(received))
	}
	if received[0].URL != testURL {
		t.Errorf("Expected URL %q, got %q", testURL, received[0].URL)
	}
}

// ==================== WSInterceptor ====================

type mockReadWriteCloser struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	closed   bool
}

func newMockRWC(readData []byte) *mockReadWriteCloser {
	return &mockReadWriteCloser{
		readBuf:  bytes.NewBuffer(readData),
		writeBuf: &bytes.Buffer{},
	}
}

func (m *mockReadWriteCloser) Read(p []byte) (n int, err error) {
	return m.readBuf.Read(p)
}

func (m *mockReadWriteCloser) Write(p []byte) (n int, err error) {
	return m.writeBuf.Write(p)
}

func (m *mockReadWriteCloser) Close() error {
	m.closed = true
	return nil
}

func TestWSInterceptor_ReadCapture(t *testing.T) {
	frame := buildWSFrame(true, 1, false, []byte("server says hi"))
	inner := newMockRWC(frame)

	var mu sync.Mutex
	var received []WSMessage
	interceptor := NewWSInterceptor(inner, "conn1", "", func(msg WSMessage) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})

	// Read all data through interceptor
	buf := make([]byte, 1024)
	for {
		_, err := interceptor.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("Expected 1 captured message, got %d", len(received))
	}
	if received[0].Payload != "server says hi" {
		t.Errorf("Expected 'server says hi', got %q", received[0].Payload)
	}
	if received[0].Direction != "receive" {
		t.Errorf("Read should be 'receive' direction, got %q", received[0].Direction)
	}
}

func TestWSInterceptor_WriteCapture(t *testing.T) {
	inner := newMockRWC(nil)

	var mu sync.Mutex
	var received []WSMessage
	interceptor := NewWSInterceptor(inner, "conn2", "", func(msg WSMessage) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})

	// Write a masked frame (client → server)
	frame := buildWSFrame(true, 1, true, []byte("client says hi"))
	_, err := interceptor.Write(frame)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("Expected 1 captured message, got %d", len(received))
	}
	if received[0].Payload != "client says hi" {
		t.Errorf("Expected 'client says hi', got %q", received[0].Payload)
	}
	if received[0].Direction != "send" {
		t.Errorf("Write should be 'send' direction, got %q", received[0].Direction)
	}

	// Verify data was also written to inner
	if inner.writeBuf.Len() != len(frame) {
		t.Errorf("Inner should receive %d bytes, got %d", len(frame), inner.writeBuf.Len())
	}
}

func TestWSInterceptor_Close(t *testing.T) {
	inner := newMockRWC(nil)
	interceptor := NewWSInterceptor(inner, "conn1", "", nil)
	interceptor.Close()
	if !inner.closed {
		t.Error("Close should propagate to inner")
	}
}
