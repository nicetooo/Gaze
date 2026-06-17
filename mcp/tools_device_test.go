package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// Helper to create a CallToolRequest with arguments
func makeToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// Helper to get text content from result
func getTextContent(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// ==================== device_list ====================

func TestHandleDeviceList_Success(t *testing.T) {
	mock := NewMockGazeApp()
	mock.SetupWithDevices(
		SampleDevice("device1"),
		SampleDevice("device2"),
	)
	server := NewMCPServer(mock)

	result, err := server.handleDeviceList(context.Background(), makeToolRequest(nil))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if !strings.Contains(text, "device1") {
		t.Error("Result should contain device1")
	}
	if !strings.Contains(text, "device2") {
		t.Error("Result should contain device2")
	}
	if !strings.Contains(text, "2 device") {
		t.Error("Result should mention 2 devices")
	}
}

func TestHandleDeviceList_NoDevices(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	result, err := server.handleDeviceList(context.Background(), makeToolRequest(nil))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if !strings.Contains(strings.ToLower(text), "no device") {
		t.Errorf("Result should indicate no devices, got: %s", text)
	}
}

func TestHandleDeviceList_Error(t *testing.T) {
	mock := NewMockGazeApp()
	mock.SetupWithError("GetDevices", ErrDeviceNotFound)
	server := NewMCPServer(mock)

	_, err := server.handleDeviceList(context.Background(), makeToolRequest(nil))
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// ==================== device_info ====================

func TestHandleDeviceInfo_Success(t *testing.T) {
	mock := NewMockGazeApp()
	mock.GetDeviceInfoResult = SampleDeviceInfo()
	server := NewMCPServer(mock)

	result, err := server.handleDeviceInfo(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if !strings.Contains(text, "Pixel 6") {
		t.Error("Result should contain model name")
	}
	if !strings.Contains(text, "1080x2400") {
		t.Error("Result should contain resolution")
	}

	// Verify correct device ID was passed
	if !mock.WasMethodCalled("GetDeviceInfo") {
		t.Error("GetDeviceInfo should have been called")
	}
	lastCall := mock.GetLastCall()
	if lastCall.Args[0] != "device1" {
		t.Errorf("Expected device_id 'device1', got %v", lastCall.Args[0])
	}
}

func TestHandleDeviceInfo_MissingDeviceId(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleDeviceInfo(context.Background(), makeToolRequest(nil))
	if err == nil {
		t.Error("Expected error for missing device_id")
	}
	if !strings.Contains(err.Error(), "device_id") {
		t.Errorf("Error should mention device_id, got: %v", err)
	}
}

func TestHandleDeviceInfo_EmptyDeviceId(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleDeviceInfo(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "",
	}))
	if err == nil {
		t.Error("Expected error for empty device_id")
	}
}

func TestHandleDeviceInfo_Error(t *testing.T) {
	mock := NewMockGazeApp()
	mock.SetupWithError("GetDeviceInfo", ErrDeviceNotFound)
	server := NewMCPServer(mock)

	_, err := server.handleDeviceInfo(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "nonexistent",
	}))
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// ==================== device_connect ====================

func TestHandleDeviceConnect_Success(t *testing.T) {
	mock := NewMockGazeApp()
	mock.AdbConnectResult = "connected to 192.168.1.100:5555"
	server := NewMCPServer(mock)

	result, err := server.handleDeviceConnect(context.Background(), makeToolRequest(map[string]interface{}{
		"address": "192.168.1.100:5555",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if !strings.Contains(text, "connected") {
		t.Error("Result should indicate connection success")
	}

	// Verify correct address was passed
	lastCall := mock.GetLastCall()
	if lastCall.Method != "AdbConnect" {
		t.Errorf("Expected AdbConnect call, got %s", lastCall.Method)
	}
	if lastCall.Args[0] != "192.168.1.100:5555" {
		t.Errorf("Expected address '192.168.1.100:5555', got %v", lastCall.Args[0])
	}
}

func TestHandleDeviceConnect_MissingAddress(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleDeviceConnect(context.Background(), makeToolRequest(nil))
	if err == nil {
		t.Error("Expected error for missing address")
	}
}

func TestHandleDeviceConnect_Error(t *testing.T) {
	mock := NewMockGazeApp()
	mock.SetupWithError("AdbConnect", ErrTimeout)
	server := NewMCPServer(mock)

	_, err := server.handleDeviceConnect(context.Background(), makeToolRequest(map[string]interface{}{
		"address": "192.168.1.100:5555",
	}))
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// ==================== device_disconnect ====================

func TestHandleDeviceDisconnect_Success(t *testing.T) {
	mock := NewMockGazeApp()
	mock.AdbDisconnectResult = "disconnected 192.168.1.100:5555"
	server := NewMCPServer(mock)

	result, err := server.handleDeviceDisconnect(context.Background(), makeToolRequest(map[string]interface{}{
		"address": "192.168.1.100:5555",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if !strings.Contains(text, "disconnected") {
		t.Error("Result should indicate disconnection")
	}
}

func TestHandleDeviceDisconnect_MissingAddress(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleDeviceDisconnect(context.Background(), makeToolRequest(nil))
	if err == nil {
		t.Error("Expected error for missing address")
	}
}

func TestHandleDeviceDisconnect_Error(t *testing.T) {
	mock := NewMockGazeApp()
	mock.SetupWithError("AdbDisconnect", ErrDeviceNotFound)
	server := NewMCPServer(mock)

	_, err := server.handleDeviceDisconnect(context.Background(), makeToolRequest(map[string]interface{}{
		"address": "192.168.1.100:5555",
	}))
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// ==================== device_pair ====================

func TestHandleDevicePair_Success(t *testing.T) {
	mock := NewMockGazeApp()
	mock.AdbPairResult = "Successfully paired"
	server := NewMCPServer(mock)

	result, err := server.handleDevicePair(context.Background(), makeToolRequest(map[string]interface{}{
		"address": "192.168.1.100:37123",
		"code":    "123456",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if !strings.Contains(strings.ToLower(text), "pair") {
		t.Error("Result should mention pairing")
	}

	// Verify both arguments were passed
	lastCall := mock.GetLastCall()
	if lastCall.Args[0] != "192.168.1.100:37123" {
		t.Errorf("Expected address '192.168.1.100:37123', got %v", lastCall.Args[0])
	}
	if lastCall.Args[1] != "123456" {
		t.Errorf("Expected code '123456', got %v", lastCall.Args[1])
	}
}

func TestHandleDevicePair_MissingAddress(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleDevicePair(context.Background(), makeToolRequest(map[string]interface{}{
		"code": "123456",
	}))
	if err == nil {
		t.Error("Expected error for missing address")
	}
}

func TestHandleDevicePair_MissingCode(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleDevicePair(context.Background(), makeToolRequest(map[string]interface{}{
		"address": "192.168.1.100:37123",
	}))
	if err == nil {
		t.Error("Expected error for missing code")
	}
}

func TestHandleDevicePair_Error(t *testing.T) {
	mock := NewMockGazeApp()
	mock.SetupWithError("AdbPair", ErrTimeout)
	server := NewMCPServer(mock)

	_, err := server.handleDevicePair(context.Background(), makeToolRequest(map[string]interface{}{
		"address": "192.168.1.100:37123",
		"code":    "123456",
	}))
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// ==================== device_wireless ====================

func TestHandleDeviceWireless_Success(t *testing.T) {
	mock := NewMockGazeApp()
	mock.SwitchToWirelessResult = "192.168.1.100:5555"
	server := NewMCPServer(mock)

	result, err := server.handleDeviceWireless(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if !strings.Contains(text, "192.168.1.100") {
		t.Error("Result should contain the wireless address")
	}
}

func TestHandleDeviceWireless_MissingDeviceId(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleDeviceWireless(context.Background(), makeToolRequest(nil))
	if err == nil {
		t.Error("Expected error for missing device_id")
	}
}

func TestHandleDeviceWireless_Error(t *testing.T) {
	mock := NewMockGazeApp()
	mock.SetupWithError("SwitchToWireless", ErrDeviceOffline)
	server := NewMCPServer(mock)

	_, err := server.handleDeviceWireless(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// ==================== device_ip ====================

func TestHandleDeviceIP_Success(t *testing.T) {
	mock := NewMockGazeApp()
	mock.GetDeviceIPResult = "192.168.1.100"
	server := NewMCPServer(mock)

	result, err := server.handleDeviceIP(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if !strings.Contains(text, "192.168.1.100") {
		t.Error("Result should contain the IP address")
	}
}

func TestHandleDeviceIP_MissingDeviceId(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleDeviceIP(context.Background(), makeToolRequest(nil))
	if err == nil {
		t.Error("Expected error for missing device_id")
	}
}

func TestHandleDeviceIP_Error(t *testing.T) {
	mock := NewMockGazeApp()
	mock.SetupWithError("GetDeviceIP", ErrDeviceOffline)
	server := NewMCPServer(mock)

	_, err := server.handleDeviceIP(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestHandleDeviceIP_EmptyResult(t *testing.T) {
	mock := NewMockGazeApp()
	mock.GetDeviceIPResult = ""
	server := NewMCPServer(mock)

	result, err := server.handleDeviceIP(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should still return a result, possibly indicating no IP found
	if result == nil {
		t.Error("Result should not be nil")
	}
}

// ==================== adb_execute timeout ====================

func TestHandleAdbExecute_DefaultTimeout(t *testing.T) {
	mock := NewMockGazeApp()
	mock.RunAdbCommandResult = "ok"
	server := NewMCPServer(mock)

	_, err := server.handleAdbExecute(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
		"command":   "shell ls",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	lastCall := mock.GetLastCallByMethod("RunAdbCommandWithTimeout")
	if lastCall == nil {
		t.Fatal("RunAdbCommandWithTimeout should have been called")
	}
	if lastCall.Args[2] != 30 {
		t.Errorf("Expected default timeout 30, got %v", lastCall.Args[2])
	}
}

func TestHandleAdbExecute_CustomTimeoutPassed(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleAdbExecute(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
		"command":   "shell logcat -d",
		"timeout":   float64(120),
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	lastCall := mock.GetLastCallByMethod("RunAdbCommandWithTimeout")
	if lastCall == nil {
		t.Fatal("RunAdbCommandWithTimeout should have been called")
	}
	if lastCall.Args[2] != 120 {
		t.Errorf("Expected timeout 120, got %v", lastCall.Args[2])
	}
}

func TestHandleAdbExecute_TimeoutClampedToMax(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleAdbExecute(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
		"command":   "pull /sdcard/big.mp4 /tmp/",
		"timeout":   float64(9999),
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	lastCall := mock.GetLastCallByMethod("RunAdbCommandWithTimeout")
	if lastCall == nil {
		t.Fatal("RunAdbCommandWithTimeout should have been called")
	}
	if lastCall.Args[2] != 300 {
		t.Errorf("Expected timeout clamped to 300, got %v", lastCall.Args[2])
	}
}

// ==================== output truncation ====================

func TestTruncateOutput_ShortUnchanged(t *testing.T) {
	out := truncateOutput("hello", defaultMaxOutputBytes)
	if out != "hello" {
		t.Errorf("Short output should be unchanged, got %q", out)
	}
}

func TestTruncateOutput_LongTruncated(t *testing.T) {
	long := strings.Repeat("a", 1000) + strings.Repeat("z", 1000)
	out := truncateOutput(long, 1000)
	if !strings.Contains(out, "truncated") {
		t.Error("Truncated output should contain a truncation marker")
	}
	if !strings.HasPrefix(out, "aaaa") {
		t.Error("Truncated output should keep the head")
	}
	if !strings.HasSuffix(out, "zzzz") {
		t.Error("Truncated output should keep the tail")
	}
	// head (80%) + tail (20%) + marker should stay close to the limit
	if len(out) > 1000+200 {
		t.Errorf("Truncated output too large: %d bytes", len(out))
	}
}

func TestTruncateOutput_ZeroUsesDefault(t *testing.T) {
	long := strings.Repeat("x", defaultMaxOutputBytes+1000)
	out := truncateOutput(long, 0)
	if !strings.Contains(out, "truncated") {
		t.Error("Output above the default limit should be truncated")
	}
}

func TestHandleAdbExecute_OutputTruncated(t *testing.T) {
	mock := NewMockGazeApp()
	mock.RunAdbCommandResult = strings.Repeat("L", 200*1024)
	server := NewMCPServer(mock)

	result, err := server.handleAdbExecute(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
		"command":   "shell logcat -d",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if !strings.Contains(text, "truncated") {
		t.Error("Oversized output should be truncated with a marker")
	}
	if len(text) > 60*1024 {
		t.Errorf("Result should be near the 50KB default limit, got %d bytes", len(text))
	}
}

func TestHandleAdbExecute_MaxOutputBytesRespected(t *testing.T) {
	mock := NewMockGazeApp()
	mock.RunAdbCommandResult = strings.Repeat("L", 200*1024)
	server := NewMCPServer(mock)

	result, err := server.handleAdbExecute(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id":        "device1",
		"command":          "shell logcat -d",
		"max_output_bytes": float64(300 * 1024),
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if strings.Contains(text, "truncated") {
		t.Error("Output below max_output_bytes should not be truncated")
	}
}
