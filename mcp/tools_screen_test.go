package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// Helper to create a test PNG file (minimal valid PNG)
func createTestPNG(path string) error {
	// Minimal valid 1x1 PNG
	png := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xFF, 0xFF, 0x3F,
		0x00, 0x05, 0xFE, 0x02, 0xFE, 0xDC, 0xCC, 0x59,
		0xE7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82, // IEND chunk
	}
	return os.WriteFile(path, png, 0644)
}

// Helper to check if result contains image content
func hasImageContent(result *mcp.CallToolResult) bool {
	for _, content := range result.Content {
		if _, ok := content.(mcp.ImageContent); ok {
			return true
		}
	}
	return false
}

// ==================== screen_screenshot ====================

func TestHandleScreenshot_Success(t *testing.T) {
	// Create a temp PNG file
	tempFile := filepath.Join(os.TempDir(), "test_screenshot.png")
	if err := createTestPNG(tempFile); err != nil {
		t.Fatalf("Failed to create test PNG: %v", err)
	}
	defer os.Remove(tempFile)

	mock := NewMockGazeApp()
	mock.TakeScreenshotResult = tempFile
	server := NewMCPServer(mock)

	result, err := server.handleScreenshot(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should contain image content (base64)
	if !hasImageContent(result) {
		t.Error("Result should contain image content")
	}

	// Should contain text content
	text := getTextContent(result)
	if !strings.Contains(text, "device1") {
		t.Error("Result should mention device ID")
	}
}

func TestHandleScreenshot_WithUIHierarchy(t *testing.T) {
	// Create a temp PNG file
	tempFile := filepath.Join(os.TempDir(), "test_screenshot_ui.png")
	if err := createTestPNG(tempFile); err != nil {
		t.Fatalf("Failed to create test PNG: %v", err)
	}
	defer os.Remove(tempFile)

	mock := NewMockGazeApp()
	mock.TakeScreenshotResult = tempFile
	mock.GetUIHierarchyResult = &UIHierarchyResult{
		RawXML: `<?xml version="1.0"?><hierarchy><node text="Hello" bounds="[0,0][100,100]"/></hierarchy>`,
	}
	server := NewMCPServer(mock)

	result, err := server.handleScreenshot(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id":  "device1",
		"include_ui": true,
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should contain image content
	if !hasImageContent(result) {
		t.Error("Result should contain image content")
	}

	// Should contain UI hierarchy in text
	text := getTextContent(result)
	if !strings.Contains(text, "UI Hierarchy") {
		t.Error("Result should contain UI hierarchy")
	}
	if !strings.Contains(text, "Hello") {
		t.Error("Result should contain element text from hierarchy")
	}
}

func TestHandleScreenshot_AutoGeneratePath(t *testing.T) {
	// Create a temp PNG file
	tempFile := filepath.Join(os.TempDir(), "test_screenshot_auto.png")
	if err := createTestPNG(tempFile); err != nil {
		t.Fatalf("Failed to create test PNG: %v", err)
	}
	defer os.Remove(tempFile)

	mock := NewMockGazeApp()
	mock.TakeScreenshotResult = tempFile
	server := NewMCPServer(mock)

	result, err := server.handleScreenshot(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
		// No save_path - should auto-generate
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should still return a result with image
	if result == nil {
		t.Error("Result should not be nil")
	}
	if !hasImageContent(result) {
		t.Error("Result should contain image content")
	}

	// Verify TakeScreenshot was called
	if !mock.WasMethodCalled("TakeScreenshot") {
		t.Error("TakeScreenshot should have been called")
	}
}

func TestHandleScreenshot_MissingDeviceId(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleScreenshot(context.Background(), makeToolRequest(nil))
	if err == nil {
		t.Error("Expected error for missing device_id")
	}
}

func TestHandleScreenshot_Error(t *testing.T) {
	mock := NewMockGazeApp()
	mock.SetupWithError("TakeScreenshot", ErrDeviceOffline)
	server := NewMCPServer(mock)

	_, err := server.handleScreenshot(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// ==================== screen_record_start ====================

func TestHandleRecordStart_Success(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	result, err := server.handleRecordStart(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if !strings.Contains(strings.ToLower(text), "record") {
		t.Error("Result should mention recording")
	}

	// Verify StartRecording was called
	if !mock.WasMethodCalled("StartRecording") {
		t.Error("StartRecording should have been called")
	}
}

func TestHandleRecordStart_WithOptions(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleRecordStart(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
		"max_size":  float64(720),
		"bit_rate":  float64(4),
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify StartRecording was called with config
	lastCall := mock.GetLastCall()
	if lastCall.Method != "StartRecording" {
		t.Errorf("Expected StartRecording call, got %s", lastCall.Method)
	}
	config, ok := lastCall.Args[1].(ScrcpyConfig)
	if !ok {
		t.Fatal("Second argument should be ScrcpyConfig")
	}
	if config.MaxSize != 720 {
		t.Errorf("Expected MaxSize 720, got %d", config.MaxSize)
	}
}

func TestHandleRecordStart_MissingDeviceId(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleRecordStart(context.Background(), makeToolRequest(nil))
	if err == nil {
		t.Error("Expected error for missing device_id")
	}
}

func TestHandleRecordStart_Error(t *testing.T) {
	mock := NewMockGazeApp()
	mock.SetupWithError("StartRecording", ErrDeviceOffline)
	server := NewMCPServer(mock)

	_, err := server.handleRecordStart(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// ==================== screen_record_stop ====================

func TestHandleRecordStop_Success(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	result, err := server.handleRecordStop(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if !strings.Contains(strings.ToLower(text), "stop") {
		t.Error("Result should mention stopping")
	}

	// Verify StopRecording was called
	if !mock.WasMethodCalled("StopRecording") {
		t.Error("StopRecording should have been called")
	}
}

func TestHandleRecordStop_MissingDeviceId(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleRecordStop(context.Background(), makeToolRequest(nil))
	if err == nil {
		t.Error("Expected error for missing device_id")
	}
}

func TestHandleRecordStop_Error(t *testing.T) {
	mock := NewMockGazeApp()
	mock.SetupWithError("StopRecording", ErrDeviceOffline)
	server := NewMCPServer(mock)

	_, err := server.handleRecordStop(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// ==================== screen_recording_status ====================

func TestHandleRecordingStatus_Recording(t *testing.T) {
	mock := NewMockGazeApp()
	mock.IsRecordingResult = true
	server := NewMCPServer(mock)

	result, err := server.handleRecordingStatus(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if !strings.Contains(strings.ToLower(text), "recording") {
		t.Error("Result should indicate recording status")
	}
}

func TestHandleRecordingStatus_NotRecording(t *testing.T) {
	mock := NewMockGazeApp()
	mock.IsRecordingResult = false
	server := NewMCPServer(mock)

	result, err := server.handleRecordingStatus(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	text := getTextContent(result)
	if !strings.Contains(strings.ToLower(text), "not") {
		t.Error("Result should indicate not recording")
	}
}

func TestHandleRecordingStatus_MissingDeviceId(t *testing.T) {
	mock := NewMockGazeApp()
	server := NewMCPServer(mock)

	_, err := server.handleRecordingStatus(context.Background(), makeToolRequest(nil))
	if err == nil {
		t.Error("Expected error for missing device_id")
	}
}

// ==================== screenshot format / resize (MC-6) ====================

// Helper to create a solid-color PNG of the given size
func createTestPNGSized(path string, w, h int) error {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0x80
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// Helper to get image content from result
func getImageContent(result *mcp.CallToolResult) *mcp.ImageContent {
	for _, content := range result.Content {
		if ic, ok := content.(mcp.ImageContent); ok {
			return &ic
		}
	}
	return nil
}

func TestHandleScreenshot_DefaultFormatJPEG(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "shot.png")
	if err := createTestPNGSized(tempFile, 10, 10); err != nil {
		t.Fatalf("Failed to create test PNG: %v", err)
	}

	mock := NewMockGazeApp()
	mock.TakeScreenshotResult = tempFile
	server := NewMCPServer(mock)

	result, err := server.handleScreenshot(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ic := getImageContent(result)
	if ic == nil {
		t.Fatal("Result should contain image content")
	}
	if ic.MIMEType != "image/jpeg" {
		t.Errorf("Default format should be image/jpeg, got %s", ic.MIMEType)
	}
}

func TestHandleScreenshot_PNGFormat(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "shot.png")
	if err := createTestPNGSized(tempFile, 10, 10); err != nil {
		t.Fatalf("Failed to create test PNG: %v", err)
	}

	mock := NewMockGazeApp()
	mock.TakeScreenshotResult = tempFile
	server := NewMCPServer(mock)

	result, err := server.handleScreenshot(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
		"format":    "png",
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ic := getImageContent(result)
	if ic == nil {
		t.Fatal("Result should contain image content")
	}
	if ic.MIMEType != "image/png" {
		t.Errorf("Expected image/png, got %s", ic.MIMEType)
	}
}

func TestHandleScreenshot_InvalidFormat(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "shot.png")
	if err := createTestPNGSized(tempFile, 10, 10); err != nil {
		t.Fatalf("Failed to create test PNG: %v", err)
	}

	mock := NewMockGazeApp()
	mock.TakeScreenshotResult = tempFile
	server := NewMCPServer(mock)

	_, err := server.handleScreenshot(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id": "device1",
		"format":    "webp",
	}))
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

func TestHandleScreenshot_DownscaleToMaxDimension(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "shot.png")
	if err := createTestPNGSized(tempFile, 100, 50); err != nil {
		t.Fatalf("Failed to create test PNG: %v", err)
	}

	mock := NewMockGazeApp()
	mock.TakeScreenshotResult = tempFile
	server := NewMCPServer(mock)

	result, err := server.handleScreenshot(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id":     "device1",
		"max_dimension": float64(40),
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ic := getImageContent(result)
	if ic == nil {
		t.Fatal("Result should contain image content")
	}
	data, err := base64.StdEncoding.DecodeString(ic.Data)
	if err != nil {
		t.Fatalf("Image data should be valid base64: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Image data should be decodable: %v", err)
	}
	if img.Bounds().Dx() != 40 || img.Bounds().Dy() != 20 {
		t.Errorf("Expected 40x20 after downscale, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestHandleScreenshot_SavePathKeepsOriginalPNG(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "shot.png")
	if err := createTestPNGSized(tempFile, 100, 50); err != nil {
		t.Fatalf("Failed to create test PNG: %v", err)
	}
	original, _ := os.ReadFile(tempFile)

	savePath := filepath.Join(t.TempDir(), "saved.png")
	mock := NewMockGazeApp()
	mock.TakeScreenshotResult = tempFile
	server := NewMCPServer(mock)

	_, err := server.handleScreenshot(context.Background(), makeToolRequest(map[string]interface{}{
		"device_id":     "device1",
		"save_path":     savePath,
		"max_dimension": float64(40), // downscale the returned image, but not the saved one
	}))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	saved, err := os.ReadFile(savePath)
	if err != nil {
		t.Fatalf("save_path file should exist: %v", err)
	}
	if !bytes.Equal(saved, original) {
		t.Error("save_path should receive the original full-resolution PNG bytes")
	}
}

func TestResizeBilinear_SolidColorPreserved(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range src.Pix {
		src.Pix[i] = 200
	}
	dst := resizeBilinear(src, 4, 4)
	if dst.Bounds().Dx() != 4 || dst.Bounds().Dy() != 4 {
		t.Fatalf("Expected 4x4 output, got %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}
	for i, v := range dst.Pix {
		if v != 200 {
			t.Fatalf("Solid color should be preserved at pix[%d]: got %d", i, v)
		}
	}
}

func TestResizeBilinear_TinySource(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1, 1))
	// Must not panic on degenerate input
	dst := resizeBilinear(src, 2, 2)
	if dst.Bounds().Dx() != 2 || dst.Bounds().Dy() != 2 {
		t.Errorf("Expected 2x2 output, got %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}
}
