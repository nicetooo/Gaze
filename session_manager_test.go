package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// ========================================
// Phase 1b: Tests for unified session management (EventPipeline only)
//
// After removing the old dual-system (session_manager.go), all session
// management now goes through EventPipeline. These tests verify:
// 1. EnsureActiveSession creates sessions in EventPipeline + SQLite
// 2. EmitRaw stores events when pipeline has an active session
// 3. Emit with full UnifiedEvent preserves Duration and all fields
// 4. ParseEventLevel maps string levels to EventLevel correctly
// 5. Session lifecycle (Start/End/GetActive) works correctly
// 6. Metadata management works via EventPipeline
// 7. End-to-end flows for each migrated caller
// ========================================

// setupTestAppForSession creates a test App with real EventStore + EventPipeline.
func setupTestAppForSession(t *testing.T) (*App, string, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "gaze_session_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	app := &App{
		dataDir: tempDir,
		mcpMode: true, // skip Wails EventsEmit calls
	}

	app.eventStore, err = NewEventStore(tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create event store: %v", err)
	}

	app.eventPipeline = NewEventPipeline(nil, nil, app.eventStore, true)
	app.eventPipeline.Start()

	cleanup := func() {
		app.eventPipeline.Stop()
		app.eventStore.Close()
		os.RemoveAll(tempDir)
	}

	return app, tempDir, cleanup
}

// waitForPipeline gives the EventPipeline time to process async events.
func waitForPipeline() {
	time.Sleep(700 * time.Millisecond)
}

// ========================================
// Test: EnsureActiveSession (EventPipeline)
// ========================================

func TestEnsureActiveSession_CreatesInPipeline(t *testing.T) {
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-ensure-001"

	// Initially no active session
	if app.eventPipeline.GetActiveSessionID(deviceID) != "" {
		t.Fatal("Expected no active session initially")
	}

	// EnsureActiveSession creates session in pipeline + SQLite
	sessionID := app.eventPipeline.EnsureActiveSession(deviceID)
	if sessionID == "" {
		t.Fatal("EnsureActiveSession returned empty session ID")
	}

	// Verify pipeline has the session
	pipelineSession := app.eventPipeline.GetActiveSession(deviceID)
	if pipelineSession == nil {
		t.Fatal("Pipeline should have an active session after EnsureActiveSession")
	}
	if pipelineSession.ID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, pipelineSession.ID)
	}
	if pipelineSession.Type != "auto" {
		t.Errorf("Expected session type 'auto', got %s", pipelineSession.Type)
	}
	if pipelineSession.Status != "active" {
		t.Errorf("Expected session status 'active', got %s", pipelineSession.Status)
	}
	if pipelineSession.DeviceID != deviceID {
		t.Errorf("Expected deviceID %s, got %s", deviceID, pipelineSession.DeviceID)
	}

	// Verify stored in SQLite
	waitForPipeline()
	stored, err := app.eventStore.GetSession(sessionID)
	if err != nil {
		t.Fatalf("GetSession from store failed: %v", err)
	}
	if stored == nil {
		t.Fatal("Session not found in SQLite")
	}
	if stored.Type != "auto" {
		t.Errorf("Expected stored type 'auto', got %s", stored.Type)
	}
}

func TestEnsureActiveSession_ReturnsExistingSession(t *testing.T) {
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-ensure-002"

	sessionID1 := app.eventPipeline.EnsureActiveSession(deviceID)
	sessionID2 := app.eventPipeline.EnsureActiveSession(deviceID)

	if sessionID1 != sessionID2 {
		t.Errorf("Expected same session ID on repeat call, got %s and %s", sessionID1, sessionID2)
	}
}

func TestEnsureActiveSession_DifferentDevicesGetDifferentSessions(t *testing.T) {
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	sid1 := app.eventPipeline.EnsureActiveSession("device-a")
	sid2 := app.eventPipeline.EnsureActiveSession("device-b")

	if sid1 == sid2 {
		t.Error("Different devices should get different sessions")
	}
}

// ========================================
// Test: EmitRaw stores events in SQLite
// ========================================

func TestEmitRaw_WithActiveSession_Stored(t *testing.T) {
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-emitraw-001"

	// Create pipeline session
	sessionID := app.eventPipeline.StartSession(deviceID, "manual", "Test", nil)
	waitForPipeline()

	// Emit logcat event (simulates migrated logcat.go:281)
	app.eventPipeline.EmitRaw(deviceID, SourceLogcat, "logcat", LevelInfo,
		"[ActivityManager] Starting activity",
		[]map[string]interface{}{
			{"tag": "ActivityManager", "message": "Starting activity", "level": "I"},
		})
	waitForPipeline()

	// Verify stored in SQLite
	result, err := app.eventStore.QueryEvents(EventQuery{
		SessionID: sessionID,
		Types:     []string{"logcat"},
		Limit:     10,
		OrderDesc: false,
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(result.Events) == 0 {
		t.Fatal("Expected logcat events in SQLite")
	}

	evt := result.Events[0]
	if evt.Source != SourceLogcat {
		t.Errorf("Expected source 'logcat', got '%s'", evt.Source)
	}
	if evt.Category != CategoryLog {
		t.Errorf("Expected category 'log', got '%s'", evt.Category)
	}
	if evt.Level != LevelInfo {
		t.Errorf("Expected level 'info', got '%s'", evt.Level)
	}

	// Verify data serialized
	if len(evt.Data) > 0 {
		var data []map[string]interface{}
		if err := json.Unmarshal(evt.Data, &data); err == nil {
			if len(data) > 0 && data[0]["tag"] != "ActivityManager" {
				t.Errorf("Expected tag 'ActivityManager', got %v", data[0]["tag"])
			}
		}
	}
}

func TestEmitRaw_WithNoSession_NotStored(t *testing.T) {
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-emitraw-nosession"

	// Emit without creating a session
	app.eventPipeline.EmitRaw(deviceID, SourceLogcat, "logcat", LevelInfo, "test", nil)
	waitForPipeline()

	// Should NOT be stored
	result, err := app.eventStore.QueryEvents(EventQuery{
		DeviceID: deviceID,
		Types:    []string{"logcat"},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(result.Events) > 0 {
		t.Errorf("Expected 0 events without active session, got %d", len(result.Events))
	}
}

// ========================================
// Test: Emit with full UnifiedEvent (proxy migration)
// ========================================

func TestEmit_FullUnifiedEvent_WithDuration(t *testing.T) {
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-emit-full"

	sessionID := app.eventPipeline.StartSession(deviceID, "manual", "Test", nil)
	waitForPipeline()

	// Simulate migrated proxy_bridge.go emission
	detail := map[string]interface{}{
		"method":     "GET",
		"url":        "https://api.example.com/users",
		"statusCode": 200,
		"duration":   150,
	}
	dataBytes, _ := json.Marshal(detail)

	app.eventPipeline.Emit(UnifiedEvent{
		DeviceID:  deviceID,
		Timestamp: time.Now().UnixMilli(),
		Source:    SourceNetwork,
		Category:  CategoryNetwork,
		Type:      "network_request",
		Level:     LevelInfo,
		Title:     "GET https://api.example.com/users → 200",
		Data:      dataBytes,
		Duration:  150,
	})
	waitForPipeline()

	result, err := app.eventStore.QueryEvents(EventQuery{
		SessionID: sessionID,
		Types:     []string{"network_request"},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(result.Events) == 0 {
		t.Fatal("network_request event not found in EventStore")
	}

	evt := result.Events[0]
	if evt.Source != SourceNetwork {
		t.Errorf("Expected source 'network', got '%s'", evt.Source)
	}
	if evt.Category != CategoryNetwork {
		t.Errorf("Expected category 'network', got '%s'", evt.Category)
	}
	if evt.Duration != 150 {
		t.Errorf("Expected duration 150, got %d", evt.Duration)
	}
	if evt.ID == "" {
		t.Error("Expected ID to be auto-generated")
	}
}

func TestEmit_AutoFillsFields(t *testing.T) {
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-emit-autofill"
	sessionID := app.eventPipeline.StartSession(deviceID, "manual", "Test", nil)
	waitForPipeline()

	// Emit event with minimal fields — ID and Timestamp should be auto-filled
	app.eventPipeline.Emit(UnifiedEvent{
		DeviceID: deviceID,
		Source:   SourceSystem,
		Type:     "test_event",
		Level:    LevelInfo,
		Title:    "Auto-fill test",
	})
	waitForPipeline()

	result, err := app.eventStore.QueryEvents(EventQuery{
		SessionID: sessionID,
		Types:     []string{"test_event"},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(result.Events) == 0 {
		t.Fatal("test_event not found in EventStore")
	}

	evt := result.Events[0]
	if evt.ID == "" {
		t.Error("Expected ID to be auto-generated")
	}
	if evt.Timestamp == 0 {
		t.Error("Expected Timestamp to be auto-filled")
	}
}

// ========================================
// Test: ParseEventLevel (pure function)
// ========================================

func TestParseEventLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected EventLevel
	}{
		{"fatal", LevelFatal},
		{"error", LevelError},
		{"warn", LevelWarn},
		{"info", LevelInfo},
		{"debug", LevelDebug},
		{"verbose", LevelVerbose},
		{"", LevelInfo},        // default
		{"unknown", LevelInfo}, // default
		{"ERROR", LevelInfo},   // case-sensitive, defaults to info
	}

	for _, tt := range tests {
		got := ParseEventLevel(tt.input)
		if got != tt.expected {
			t.Errorf("ParseEventLevel(%q) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

// ========================================
// Test: Session Lifecycle via EventPipeline
// ========================================

func TestStartSession_CreatesAndStores(t *testing.T) {
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-start-001"
	sessionID := app.eventPipeline.StartSession(deviceID, "workflow", "Test Workflow", nil)

	if sessionID == "" {
		t.Fatal("StartSession returned empty session ID")
	}

	session := app.eventPipeline.GetActiveSession(deviceID)
	if session == nil {
		t.Fatal("No active session after StartSession")
	}
	if session.Type != "workflow" {
		t.Errorf("Expected type 'workflow', got '%s'", session.Type)
	}
	if session.Name != "Test Workflow" {
		t.Errorf("Expected name 'Test Workflow', got '%s'", session.Name)
	}
	if session.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", session.Status)
	}

	// Verify stored in SQLite
	waitForPipeline()
	stored, err := app.eventStore.GetSession(sessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if stored == nil {
		t.Fatal("Session not found in SQLite")
	}
}

func TestEndSession_CleansUpState(t *testing.T) {
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-end-001"
	sessionID := app.eventPipeline.StartSession(deviceID, "debug", "Test Debug", nil)

	app.eventPipeline.EndSession(sessionID, "completed")

	// Session should no longer be active for this device
	activeID := app.eventPipeline.GetActiveSessionID(deviceID)
	if activeID != "" {
		t.Errorf("Expected no active session after EndSession, got %s", activeID)
	}

	// In-memory session state should be released (RingBuffer freed)
	if s := app.eventPipeline.GetSession(sessionID); s != nil {
		t.Errorf("Expected in-memory session state to be released after EndSession, got %+v", s)
	}

	// Session data should be updated in store
	waitForPipeline()
	stored, err := app.eventStore.GetSession(sessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if stored != nil && stored.Status != "completed" {
		t.Errorf("Expected stored status 'completed', got '%s'", stored.Status)
	}
}

func TestStartSession_ReplacesOldSession(t *testing.T) {
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-replace-001"

	sid1 := app.eventPipeline.StartSession(deviceID, "manual", "First", nil)
	sid2 := app.eventPipeline.StartSession(deviceID, "manual", "Second", nil)

	if sid1 == sid2 {
		t.Error("Expected different session IDs")
	}

	// Only sid2 should be active
	activeID := app.eventPipeline.GetActiveSessionID(deviceID)
	if activeID != sid2 {
		t.Errorf("Expected active session %s, got %s", sid2, activeID)
	}

	// Old session should be marked completed
	waitForPipeline()
	old, err := app.eventStore.GetSession(sid1)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if old != nil && old.Status != "completed" {
		t.Errorf("Expected old session status 'completed', got '%s'", old.Status)
	}
}

// ========================================
// Test: Metadata via EventPipeline
// ========================================

func TestPipelineSessionMetadata(t *testing.T) {
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	sessionID := app.eventPipeline.StartSession("test-device-meta", "debug", "Meta Test", nil)

	app.eventPipeline.SetSessionMetadata(sessionID, "key1", "value1")
	app.eventPipeline.SetSessionMetadata(sessionID, "count", 42)

	val1 := app.eventPipeline.GetSessionMetadata(sessionID, "key1")
	if val1 != "value1" {
		t.Errorf("Expected 'value1', got %v", val1)
	}

	val2 := app.eventPipeline.GetSessionMetadata(sessionID, "count")
	if val2 != 42 {
		t.Errorf("Expected 42, got %v", val2)
	}

	nilVal := app.eventPipeline.GetSessionMetadata(sessionID, "nonexistent")
	if nilVal != nil {
		t.Errorf("Expected nil for nonexistent key, got %v", nilVal)
	}

	// Non-existent session returns nil
	nilVal2 := app.eventPipeline.GetSessionMetadata("fake-session", "key1")
	if nilVal2 != nil {
		t.Errorf("Expected nil for fake session, got %v", nilVal2)
	}
}

// ========================================
// Test: EventRegistry has network_request registered
// ========================================

func TestEventRegistry_NetworkRequestRegistered(t *testing.T) {
	info, ok := EventRegistry["network_request"]
	if !ok {
		t.Fatal("network_request not found in EventRegistry")
	}
	if info.Source != SourceNetwork {
		t.Errorf("Expected source 'network', got '%s'", info.Source)
	}
	if info.Category != CategoryNetwork {
		t.Errorf("Expected category 'network', got '%s'", info.Category)
	}
}

// ========================================
// Test: End-to-end flow simulating migrated callers
// ========================================

func TestDeviceConnect_NoAutoSession(t *testing.T) {
	// After fix: GetDevices() no longer calls EnsureActiveSession.
	// Events without a session are pushed to frontend but NOT stored in SQLite.
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-no-auto"

	// Device connects — no auto session creation (was removed from GetDevices)
	activeID := app.eventPipeline.GetActiveSessionID(deviceID)
	if activeID != "" {
		t.Fatal("Newly connected device should NOT have an auto session")
	}

	// Logcat emits directly to pipeline without a session
	app.eventPipeline.EmitRaw(deviceID, SourceLogcat, "logcat",
		ParseEventLevel("info"), "[ActivityManager] Starting activity",
		[]map[string]interface{}{
			{"tag": "ActivityManager", "message": "Starting activity", "level": "I"},
		})
	waitForPipeline()

	// Events should NOT be stored (no active session)
	result, err := app.eventStore.QueryEvents(EventQuery{
		DeviceID: deviceID,
		Types:    []string{"logcat"},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(result.Events) != 0 {
		t.Fatalf("Expected 0 stored events without session, got %d", len(result.Events))
	}
}

func TestDeviceConnect_ManualSessionStoresEvents(t *testing.T) {
	// After fix: user must explicitly start a session for events to be stored.
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-manual-session"

	// User explicitly starts a session
	sessionID := app.eventPipeline.StartSession(deviceID, "manual", "My Debug Session", nil)
	waitForPipeline()

	// Logcat emits to pipeline — now events should be stored
	app.eventPipeline.EmitRaw(deviceID, SourceLogcat, "logcat",
		ParseEventLevel("info"), "[ActivityManager] Starting activity",
		[]map[string]interface{}{
			{"tag": "ActivityManager", "message": "Starting activity", "level": "I"},
		})
	waitForPipeline()

	result, err := app.eventStore.QueryEvents(EventQuery{
		SessionID: sessionID,
		Types:     []string{"logcat"},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(result.Events) == 0 {
		t.Fatal("Events should be stored when user has an active session")
	}
}

func TestEndSession_NoAutoRecreate(t *testing.T) {
	// Core regression test: ending a session must NOT trigger auto-recreation.
	// Previously GetDevices() polling would call EnsureActiveSession every 2 seconds,
	// recreating the session immediately after the user ended it.
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-no-recreate"

	// User starts and ends a session
	sessionID := app.eventPipeline.StartSession(deviceID, "manual", "Temporary", nil)
	waitForPipeline()
	app.eventPipeline.EndSession(sessionID, "completed")

	// After ending, no active session should exist
	activeID := app.eventPipeline.GetActiveSessionID(deviceID)
	if activeID != "" {
		t.Fatalf("Expected no active session after EndSession, got %s", activeID)
	}

	// Simulate what GetDevices() used to do (now removed) — verify it stays empty
	// (In the old code, GetDevices would call EnsureActiveSession here, recreating the session)
	// Now nothing calls EnsureActiveSession, so the device remains without a session.
	activeID = app.eventPipeline.GetActiveSessionID(deviceID)
	if activeID != "" {
		t.Fatal("Session should NOT be auto-recreated after user explicitly ended it")
	}
}

func TestEndToEnd_ProxyEmit(t *testing.T) {
	// Simulates migrated proxy_bridge.go: proxy event → pipeline.Emit
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-e2e-proxy"

	sessionID := app.eventPipeline.StartSession(deviceID, "manual", "Proxy Test", nil)
	waitForPipeline()

	// Simulate migrated proxy_bridge.go:178
	detail := map[string]interface{}{
		"method":     "GET",
		"url":        "https://api.example.com/data",
		"statusCode": float64(200),
	}
	dataBytes, _ := json.Marshal(detail)

	app.eventPipeline.Emit(UnifiedEvent{
		DeviceID:  deviceID,
		Timestamp: time.Now().UnixMilli(),
		Source:    SourceNetwork,
		Category:  CategoryNetwork,
		Type:      "network_request",
		Level:     ParseEventLevel("info"),
		Title:     "GET https://api.example.com/data → 200",
		Data:      dataBytes,
		Duration:  120,
	})
	waitForPipeline()

	result, err := app.eventStore.QueryEvents(EventQuery{
		SessionID: sessionID,
		Types:     []string{"network_request"},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(result.Events) == 0 {
		t.Fatal("No network events found")
	}

	evt := result.Events[0]
	if evt.Source != SourceNetwork {
		t.Errorf("Expected source network, got %s", evt.Source)
	}
	if evt.Duration != 120 {
		t.Errorf("Expected duration 120, got %d", evt.Duration)
	}
}

func TestEndToEnd_WorkflowWithEnsureSession(t *testing.T) {
	// Simulates migrated workflow.go:225 — pipeline.EnsureActiveSession + EmitRaw
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-e2e-workflow"

	// Step 1: workflow.go calls EnsureActiveSession on pipeline
	sessionID := app.eventPipeline.EnsureActiveSession(deviceID)
	waitForPipeline()

	// Step 2: workflow.go calls EmitRaw directly on pipeline
	app.eventPipeline.EmitRaw(deviceID, SourceWorkflow, "workflow_start", LevelInfo,
		"Workflow started: Test Flow",
		map[string]interface{}{
			"workflowId": "wf_001",
			"totalSteps": 5,
			"sessionId":  sessionID,
		})
	waitForPipeline()

	result, err := app.eventStore.QueryEvents(EventQuery{
		SessionID: sessionID,
		Types:     []string{"workflow_start"},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(result.Events) == 0 {
		t.Fatal("workflow_start event not found")
	}
	if result.Events[0].Source != SourceWorkflow {
		t.Errorf("Expected source workflow, got %s", result.Events[0].Source)
	}

	// Verify the session used is from EnsureActiveSession
	if result.Events[0].SessionID != sessionID {
		t.Errorf("Expected sessionID %s, got %s", sessionID, result.Events[0].SessionID)
	}
}

func TestEndToEnd_LogcatLevelMapping(t *testing.T) {
	// Simulates migrated logcat.go: logcatLevelToSessionLevel → ParseEventLevel
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-e2e-logcat-levels"
	sessionID := app.eventPipeline.StartSession(deviceID, "manual", "Levels Test", nil)
	waitForPipeline()

	// Simulate logcat events at different levels
	levels := []struct {
		androidLevel  string // E, W, I, D, V
		sessionLevel  string // from logcatLevelToSessionLevel
		expectedLevel EventLevel
	}{
		{"E", "error", LevelError},
		{"W", "warn", LevelWarn},
		{"I", "info", LevelInfo},
		{"D", "debug", LevelDebug},
		{"V", "verbose", LevelVerbose},
	}

	for _, l := range levels {
		app.eventPipeline.EmitRaw(deviceID, SourceLogcat, "logcat",
			ParseEventLevel(l.sessionLevel),
			"[Test] "+l.androidLevel+" level",
			map[string]interface{}{"level": l.androidLevel})
	}
	waitForPipeline()

	result, err := app.eventStore.QueryEvents(EventQuery{
		SessionID: sessionID,
		Types:     []string{"logcat"},
		Limit:     50,
		OrderDesc: false,
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	for _, l := range levels {
		title := "[Test] " + l.androidLevel + " level"
		found := false
		for _, evt := range result.Events {
			if evt.Title == title {
				found = true
				if evt.Level != l.expectedLevel {
					t.Errorf("[%s] Expected level '%s', got '%s'", l.androidLevel, l.expectedLevel, evt.Level)
				}
				break
			}
		}
		if !found {
			t.Errorf("[%s] Event not found in store", l.androidLevel)
		}
	}
}

func TestEndToEnd_ProxyStatusCodeLevels(t *testing.T) {
	// Simulates proxy_bridge.go level determination based on status codes
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-e2e-proxy-levels"
	sessionID := app.eventPipeline.StartSession(deviceID, "manual", "Proxy Levels", nil)
	waitForPipeline()

	cases := []struct {
		statusCode    int
		stringLevel   string // as determined by proxy_bridge.go
		expectedLevel EventLevel
	}{
		{200, "info", LevelInfo},
		{404, "warn", LevelWarn},
		{500, "error", LevelError},
	}

	for _, c := range cases {
		detail := map[string]interface{}{"statusCode": c.statusCode}
		dataBytes, _ := json.Marshal(detail)
		app.eventPipeline.Emit(UnifiedEvent{
			DeviceID:  deviceID,
			Timestamp: time.Now().UnixMilli(),
			Source:    SourceNetwork,
			Category:  CategoryNetwork,
			Type:      "network_request",
			Level:     ParseEventLevel(c.stringLevel),
			Title:     "test",
			Data:      dataBytes,
		})
	}
	waitForPipeline()

	result, err := app.eventStore.QueryEvents(EventQuery{
		SessionID: sessionID,
		Types:     []string{"network_request"},
		Limit:     50,
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	if len(result.Events) < 3 {
		t.Fatalf("Expected at least 3 network events, got %d", len(result.Events))
	}

	// Check we have events at each expected level
	levelSet := map[EventLevel]bool{}
	for _, evt := range result.Events {
		levelSet[evt.Level] = true
	}
	for _, c := range cases {
		if !levelSet[c.expectedLevel] {
			t.Errorf("Expected at least one event with level %s", c.expectedLevel)
		}
	}
}

// ========================================
// Test: No dual-system leakage (regression)
// ========================================

func TestNoDualSystem_PipelineSessionStoredInSQLite(t *testing.T) {
	// Verifies that EnsureActiveSession (now on pipeline) creates sessions
	// that are stored in SQLite — the key fix from removing the dual system.
	app, _, cleanup := setupTestAppForSession(t)
	defer cleanup()

	deviceID := "test-device-no-dual"

	// EnsureActiveSession now creates in pipeline → stored in SQLite
	sessionID := app.eventPipeline.EnsureActiveSession(deviceID)
	waitForPipeline()

	// Emit event
	app.eventPipeline.EmitRaw(deviceID, SourceLogcat, "logcat", LevelInfo, "test", nil)
	waitForPipeline()

	// Events MUST be stored (the old dual-system bug was that EnsureActiveSession
	// only created in the old in-memory map, not in pipeline, so events weren't stored)
	result, err := app.eventStore.QueryEvents(EventQuery{
		SessionID: sessionID,
		Types:     []string{"logcat"},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(result.Events) == 0 {
		t.Fatal("REGRESSION: Events not stored after EnsureActiveSession — dual-system bug returned")
	}
}
