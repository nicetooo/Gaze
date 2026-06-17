package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"Gaze/mcp"
)

// Integration tests for MCP Bridge
// These tests use real database connections to verify end-to-end data flow

func setupTestApp(t *testing.T) (*App, string, func()) {
	// Create temp directory for test database
	tempDir, err := os.MkdirTemp("", "gaze_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create App with test data directory
	app := &App{
		dataDir: tempDir,
		mcpMode: true,
	}

	// Initialize event store
	app.eventStore, err = NewEventStore(tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create event store: %v", err)
	}

	// Initialize event pipeline
	app.eventPipeline = NewEventPipeline(nil, nil, app.eventStore, true)
	app.eventPipeline.Start()

	cleanup := func() {
		app.eventPipeline.Stop()
		app.eventStore.Close()
		os.RemoveAll(tempDir)
	}

	return app, tempDir, cleanup
}

func TestMCPBridge_QuerySessionEvents_IncludesData(t *testing.T) {
	app, _, cleanup := setupTestApp(t)
	defer cleanup()

	// Create a test session
	sessionID := app.eventPipeline.StartSession("test-device-001", "manual", "Test Session", nil)
	if sessionID == "" {
		t.Fatal("Failed to create session")
	}

	// Wait for session to be created
	time.Sleep(100 * time.Millisecond)

	// Emit touch events with coordinates
	touchData := map[string]interface{}{
		"action":      "tap",
		"x":           540,
		"y":           960,
		"gestureType": "tap",
		"duration":    150,
	}
	dataBytes, _ := json.Marshal(touchData)

	app.eventPipeline.Emit(UnifiedEvent{
		ID:        "test-event-001",
		SessionID: sessionID,
		DeviceID:  "test-device-001",
		Timestamp: time.Now().UnixMilli(),
		Source:    SourceTouch,
		Category:  CategoryInteraction,
		Type:      "touch",
		Level:     LevelInfo,
		Title:     "Touch: tap at (540, 960)",
		Data:      dataBytes,
	})

	// Emit swipe event
	swipeData := map[string]interface{}{
		"action":      "swipe",
		"x":           100,
		"y":           200,
		"x2":          100,
		"y2":          800,
		"gestureType": "swipe",
		"duration":    300,
	}
	swipeBytes, _ := json.Marshal(swipeData)

	app.eventPipeline.Emit(UnifiedEvent{
		ID:        "test-event-002",
		SessionID: sessionID,
		DeviceID:  "test-device-001",
		Timestamp: time.Now().UnixMilli(),
		Source:    SourceTouch,
		Category:  CategoryInteraction,
		Type:      "touch",
		Level:     LevelInfo,
		Title:     "Touch: swipe at (100, 200)",
		Data:      swipeBytes,
	})

	// Wait for events to be processed and stored
	time.Sleep(600 * time.Millisecond)

	// Create MCP Bridge and query events
	bridge := NewMCPBridge(app)

	query := mcp.EventQuery{
		SessionID: sessionID,
		Sources:   []string{"touch"},
		Limit:     10,
	}

	result, err := bridge.QuerySessionEvents(query)
	if err != nil {
		t.Fatalf("QuerySessionEvents failed: %v", err)
	}

	if len(result.Events) == 0 {
		t.Fatal("Expected events but got none")
	}

	// Verify first event has data with coordinates
	eventMap, ok := result.Events[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Event is not a map, got %T", result.Events[0])
	}

	// Check that data field exists and contains coordinates
	data, ok := eventMap["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Event data is not a map, got %T (eventMap: %+v)", eventMap["data"], eventMap)
	}

	// Verify x coordinate
	x, ok := data["x"]
	if !ok {
		t.Error("Event data missing 'x' coordinate")
	} else {
		t.Logf("x coordinate: %v", x)
	}

	// Verify y coordinate
	y, ok := data["y"]
	if !ok {
		t.Error("Event data missing 'y' coordinate")
	} else {
		t.Logf("y coordinate: %v", y)
	}

	// Verify action
	action, ok := data["action"]
	if !ok {
		t.Error("Event data missing 'action'")
	} else {
		t.Logf("action: %v", action)
	}

	// Verify gestureType
	gesture, ok := data["gestureType"]
	if !ok {
		t.Error("Event data missing 'gestureType'")
	} else {
		t.Logf("gestureType: %v", gesture)
	}

	t.Logf("Total events returned: %d", len(result.Events))
	t.Logf("First event: %+v", eventMap)
}

func TestMCPBridge_QuerySessionEvents_SwipeCoordinates(t *testing.T) {
	app, _, cleanup := setupTestApp(t)
	defer cleanup()

	// Create session
	sessionID := app.eventPipeline.StartSession("test-device-002", "manual", "Swipe Test", nil)
	time.Sleep(100 * time.Millisecond)

	// Emit swipe event with start and end coordinates
	swipeData := map[string]interface{}{
		"action":      "swipe",
		"x":           100,
		"y":           500,
		"x2":          100,
		"y2":          1500,
		"gestureType": "swipe",
		"swipeDir":    "down",
		"duration":    400,
	}
	dataBytes, _ := json.Marshal(swipeData)

	app.eventPipeline.Emit(UnifiedEvent{
		ID:        "swipe-event-001",
		SessionID: sessionID,
		DeviceID:  "test-device-002",
		Timestamp: time.Now().UnixMilli(),
		Source:    SourceTouch,
		Category:  CategoryInteraction,
		Type:      "touch",
		Level:     LevelInfo,
		Title:     "Touch: swipe down",
		Data:      dataBytes,
	})

	time.Sleep(600 * time.Millisecond)

	bridge := NewMCPBridge(app)
	result, err := bridge.QuerySessionEvents(mcp.EventQuery{
		SessionID: sessionID,
		Sources:   []string{"touch"}, // Filter to touch events only
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result.Events) == 0 {
		t.Fatal("No touch events returned")
	}

	eventMap := result.Events[0].(map[string]interface{})
	data, ok := eventMap["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Event data is not a map: %+v", eventMap)
	}

	// Verify start coordinates
	if x, ok := data["x"]; !ok || x != float64(100) {
		t.Errorf("Expected x=100, got %v", data["x"])
	}
	if y, ok := data["y"]; !ok || y != float64(500) {
		t.Errorf("Expected y=500, got %v", data["y"])
	}

	// Verify end coordinates
	if x2, ok := data["x2"]; !ok || x2 != float64(100) {
		t.Errorf("Expected x2=100, got %v", data["x2"])
	}
	if y2, ok := data["y2"]; !ok || y2 != float64(1500) {
		t.Errorf("Expected y2=1500, got %v", data["y2"])
	}

	// Verify swipe direction
	if dir, ok := data["swipeDir"]; !ok || dir != "down" {
		t.Errorf("Expected swipeDir='down', got %v", data["swipeDir"])
	}

	t.Logf("Swipe event data: %+v", data)
}

func TestMCPBridge_QuerySessionEvents_FilterBySource(t *testing.T) {
	app, _, cleanup := setupTestApp(t)
	defer cleanup()

	sessionID := app.eventPipeline.StartSession("test-device-003", "manual", "Filter Test", nil)
	time.Sleep(100 * time.Millisecond)

	// Emit touch event
	touchData, _ := json.Marshal(map[string]interface{}{"x": 100, "y": 200})
	app.eventPipeline.Emit(UnifiedEvent{
		ID:        "touch-001",
		SessionID: sessionID,
		DeviceID:  "test-device-003",
		Timestamp: time.Now().UnixMilli(),
		Source:    SourceTouch,
		Type:      "touch",
		Level:     LevelInfo,
		Title:     "Touch event",
		Data:      touchData,
	})

	// Emit logcat event
	logData, _ := json.Marshal(map[string]interface{}{"tag": "Test", "message": "Log message"})
	app.eventPipeline.Emit(UnifiedEvent{
		ID:        "log-001",
		SessionID: sessionID,
		DeviceID:  "test-device-003",
		Timestamp: time.Now().UnixMilli(),
		Source:    SourceLogcat,
		Type:      "logcat",
		Level:     LevelInfo,
		Title:     "Log event",
		Data:      logData,
	})

	// Emit device event
	deviceData, _ := json.Marshal(map[string]interface{}{"level": 85})
	app.eventPipeline.Emit(UnifiedEvent{
		ID:        "device-001",
		SessionID: sessionID,
		DeviceID:  "test-device-003",
		Timestamp: time.Now().UnixMilli(),
		Source:    SourceDevice,
		Type:      "battery_change",
		Level:     LevelInfo,
		Title:     "Battery 85%",
		Data:      deviceData,
	})

	time.Sleep(600 * time.Millisecond)

	bridge := NewMCPBridge(app)

	// Query only touch events
	result, err := bridge.QuerySessionEvents(mcp.EventQuery{
		SessionID: sessionID,
		Sources:   []string{"touch"},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result.Events) != 1 {
		t.Errorf("Expected 1 touch event, got %d", len(result.Events))
	}

	eventMap := result.Events[0].(map[string]interface{})
	if eventMap["source"] != "touch" {
		t.Errorf("Expected source='touch', got %v", eventMap["source"])
	}

	// Query all events
	allResult, err := bridge.QuerySessionEvents(mcp.EventQuery{
		SessionID: sessionID,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Query all failed: %v", err)
	}

	// Should have 4 events: session_start + touch + logcat + device
	if len(allResult.Events) < 3 {
		t.Errorf("Expected at least 3 events, got %d", len(allResult.Events))
	}

	t.Logf("Touch only: %d events, All: %d events", len(result.Events), len(allResult.Events))
}

func TestMCPBridge_SessionList_IncludesVideoPath(t *testing.T) {
	app, tempDir, cleanup := setupTestApp(t)
	defer cleanup()

	// Create session with video path
	sessionID := app.eventPipeline.StartSession("test-device-004", "recording", "Video Session", nil)
	time.Sleep(100 * time.Millisecond)

	// Update video path
	videoPath := filepath.Join(tempDir, "test_recording.mp4")
	app.eventPipeline.UpdateSessionVideoPath(sessionID, videoPath)

	time.Sleep(100 * time.Millisecond)

	bridge := NewMCPBridge(app)
	sessions, err := bridge.ListStoredSessions("", 10)
	if err != nil {
		t.Fatalf("ListStoredSessions failed: %v", err)
	}

	if len(sessions) == 0 {
		t.Fatal("No sessions returned")
	}

	// Find our session
	var found bool
	for _, s := range sessions {
		if s.ID == sessionID {
			found = true
			if s.VideoPath != videoPath {
				t.Errorf("Expected VideoPath=%s, got %s", videoPath, s.VideoPath)
			}
			t.Logf("Session found with VideoPath: %s", s.VideoPath)
			break
		}
	}

	if !found {
		t.Error("Created session not found in list")
	}
}

func TestMCPBridge_CreateSession_PersistsToDatabase(t *testing.T) {
	app, _, cleanup := setupTestApp(t)
	defer cleanup()

	bridge := NewMCPBridge(app)

	// Create session via bridge
	sessionID := bridge.CreateSession("test-device-005", "manual", "Bridge Test Session")
	if sessionID == "" {
		t.Fatal("CreateSession returned empty ID")
	}

	time.Sleep(200 * time.Millisecond)

	// Verify session is in database via ListStoredSessions
	sessions, err := bridge.ListStoredSessions("test-device-005", 10)
	if err != nil {
		t.Fatalf("ListStoredSessions failed: %v", err)
	}

	var found bool
	for _, s := range sessions {
		if s.ID == sessionID {
			found = true
			if s.Name != "Bridge Test Session" {
				t.Errorf("Expected Name='Bridge Test Session', got %s", s.Name)
			}
			if s.Type != "manual" {
				t.Errorf("Expected Type='manual', got %s", s.Type)
			}
			if s.Status != "active" {
				t.Errorf("Expected Status='active', got %s", s.Status)
			}
			break
		}
	}

	if !found {
		t.Errorf("Session %s not found in database", sessionID)
	}

	t.Logf("Session created and persisted: %s", sessionID)
}

func TestMCPBridge_EndSession_UpdatesDatabase(t *testing.T) {
	app, _, cleanup := setupTestApp(t)
	defer cleanup()

	bridge := NewMCPBridge(app)

	// Create session
	sessionID := bridge.CreateSession("test-device-006", "manual", "End Test Session")
	time.Sleep(200 * time.Millisecond)

	// End session
	err := bridge.EndSession(sessionID, "completed")
	if err != nil {
		t.Fatalf("EndSession failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify status updated
	sessions, _ := bridge.ListStoredSessions("test-device-006", 10)
	for _, s := range sessions {
		if s.ID == sessionID {
			if s.Status != "completed" {
				t.Errorf("Expected Status='completed', got %s", s.Status)
			}
			t.Logf("Session ended with status: %s", s.Status)
			return
		}
	}

	t.Error("Session not found after ending")
}

func TestMCPBridge_GetActiveSession_ReturnsCorrectID(t *testing.T) {
	app, _, cleanup := setupTestApp(t)
	defer cleanup()

	bridge := NewMCPBridge(app)

	// Initially no active session
	activeID := bridge.GetActiveSession("test-device-007")
	if activeID != "" {
		t.Errorf("Expected no active session, got %s", activeID)
	}

	// Create session
	sessionID := bridge.CreateSession("test-device-007", "manual", "Active Test")
	time.Sleep(100 * time.Millisecond)

	// Now should have active session
	activeID = bridge.GetActiveSession("test-device-007")
	if activeID != sessionID {
		t.Errorf("Expected active session %s, got %s", sessionID, activeID)
	}

	// End session
	bridge.EndSession(sessionID, "completed")
	time.Sleep(100 * time.Millisecond)

	// No longer active
	activeID = bridge.GetActiveSession("test-device-007")
	if activeID != "" {
		t.Errorf("Expected no active session after end, got %s", activeID)
	}

	t.Log("Active session tracking works correctly")
}

func TestNormalizeSelectorType(t *testing.T) {
	cases := map[string]string{
		"id":           "id",
		"resourceId":   "id",
		"resource-id":  "id",
		"contentDesc":  "desc",
		"desc":         "desc",
		"description":  "desc",
		"content-desc": "desc",
		"className":    "class",
		"class":        "class",
		"text":         "text",
		"contains":     "contains",
		"xpath":        "xpath",
		"advanced":     "advanced",
	}
	for input, expected := range cases {
		if got := normalizeSelectorType(input); got != expected {
			t.Errorf("normalizeSelectorType(%q) = %q, want %q", input, got, expected)
		}
	}
}
