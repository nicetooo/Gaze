package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ========================================
// Test Helpers
// ========================================

// createTestSession creates a session in the store and returns it
func createTestSession(t *testing.T, store *EventStore, name string) *DeviceSession {
	t.Helper()
	session := &DeviceSession{
		ID:        uuid.New().String()[:8],
		DeviceID:  "test-device-001",
		Type:      "manual",
		Name:      name,
		StartTime: time.Now().UnixMilli(),
		EndTime:   time.Now().UnixMilli() + 60000,
		Status:    "completed",
		Metadata:  map[string]any{"env": "test"},
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	return session
}

// createTestEvents writes events to the store and returns them
func createTestEvents(t *testing.T, store *EventStore, sessionID, deviceID string, count int) []UnifiedEvent {
	t.Helper()
	var events []UnifiedEvent
	for i := 0; i < count; i++ {
		event := UnifiedEvent{
			ID:           uuid.New().String(),
			SessionID:    sessionID,
			DeviceID:     deviceID,
			Timestamp:    time.Now().UnixMilli() + int64(i*100),
			RelativeTime: int64(i * 100),
			Source:       SourceLogcat,
			Category:     CategoryLog,
			Type:         "logcat",
			Level:        LevelInfo,
			Title:        "Test log " + uuid.New().String()[:4],
			Summary:      "Summary text",
			Data:         json.RawMessage(`{"tag":"TestTag","message":"msg ` + uuid.New().String()[:4] + `"}`),
		}
		if err := store.WriteEventDirect(event); err != nil {
			t.Fatalf("Failed to write event %d: %v", i, err)
		}
		events = append(events, event)
	}
	return events
}

// createTestBookmarks creates bookmarks in the store and returns them
func createTestBookmarks(t *testing.T, store *EventStore, sessionID string, count int) []Bookmark {
	t.Helper()
	var bookmarks []Bookmark
	for i := 0; i < count; i++ {
		b := Bookmark{
			ID:           uuid.New().String(),
			SessionID:    sessionID,
			RelativeTime: int64(i * 1000),
			Label:        "Bookmark " + uuid.New().String()[:4],
			Color:        "#ff0000",
			Type:         "user",
			CreatedAt:    time.Now().UnixMilli(),
		}
		if err := store.CreateBookmark(&b); err != nil {
			t.Fatalf("Failed to create bookmark %d: %v", i, err)
		}
		bookmarks = append(bookmarks, b)
	}
	return bookmarks
}

// readZipEntry reads a specific file from a ZIP archive
func readZipEntry(t *testing.T, zipPath, entryName string) []byte {
	t.Helper()
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip: %v", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == entryName {
			data, err := readZipFile(f)
			if err != nil {
				t.Fatalf("Failed to read zip entry %s: %v", entryName, err)
			}
			return data
		}
	}
	return nil
}

// listZipEntries lists all file names in a ZIP archive
func listZipEntries(t *testing.T, zipPath string) []string {
	t.Helper()
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip: %v", err)
	}
	defer r.Close()

	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	return names
}

// setupTestAppForExport creates an App with EventStore for export/import testing
func setupTestAppForExport(t *testing.T) (*App, string, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "session_export_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	store, err := NewEventStore(tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create EventStore: %v", err)
	}

	app := &App{
		dataDir:    tempDir,
		mcpMode:    true,
		eventStore: store,
		version:    "1.0.0-test",
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir)
	}

	return app, tempDir, cleanup
}

// ========================================
// EventStore.ExportSessionEvents Tests
// ========================================

func TestExportSessionEvents_WithData(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := createTestSession(t, store, "Export Test")
	events := createTestEvents(t, store, session.ID, session.DeviceID, 5)

	// Export
	exported, err := store.ExportSessionEvents(session.ID)
	if err != nil {
		t.Fatalf("ExportSessionEvents failed: %v", err)
	}

	if len(exported) != 5 {
		t.Fatalf("Expected 5 exported events, got %d", len(exported))
	}

	// Verify events have data
	for i, e := range exported {
		if e.ID != events[i].ID {
			t.Errorf("Event %d: ID mismatch: got %s, want %s", i, e.ID, events[i].ID)
		}
		if e.SessionID != session.ID {
			t.Errorf("Event %d: SessionID mismatch: got %s, want %s", i, e.SessionID, session.ID)
		}
		if len(e.Data) == 0 {
			t.Errorf("Event %d: Data should not be empty", i)
		}
		if e.Title != events[i].Title {
			t.Errorf("Event %d: Title mismatch: got %s, want %s", i, e.Title, events[i].Title)
		}
	}

	// Verify ordering (by relative_time ASC)
	for i := 1; i < len(exported); i++ {
		if exported[i].RelativeTime < exported[i-1].RelativeTime {
			t.Errorf("Events not ordered by RelativeTime: event %d (%d) < event %d (%d)",
				i, exported[i].RelativeTime, i-1, exported[i-1].RelativeTime)
		}
	}
}

func TestExportSessionEvents_EmptySession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := createTestSession(t, store, "Empty Session")

	exported, err := store.ExportSessionEvents(session.ID)
	if err != nil {
		t.Fatalf("ExportSessionEvents failed: %v", err)
	}

	if len(exported) != 0 {
		t.Errorf("Expected 0 events for empty session, got %d", len(exported))
	}
}

func TestExportSessionEvents_NonExistentSession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	exported, err := store.ExportSessionEvents("nonexistent-id")
	if err != nil {
		t.Fatalf("ExportSessionEvents should not error for nonexistent session: %v", err)
	}

	if len(exported) != 0 {
		t.Errorf("Expected 0 events for nonexistent session, got %d", len(exported))
	}
}

func TestExportSessionEvents_PreservesAllFields(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := createTestSession(t, store, "Fields Test")

	// Create event with all fields populated
	event := UnifiedEvent{
		ID:             uuid.New().String(),
		SessionID:      session.ID,
		DeviceID:       session.DeviceID,
		Timestamp:      time.Now().UnixMilli(),
		RelativeTime:   500,
		Duration:       150,
		Source:         SourceNetwork,
		Category:       CategoryNetwork,
		Type:           "http_request",
		Level:          LevelWarn,
		Title:          "GET /api/test",
		Summary:        "HTTP request summary",
		ParentID:       "parent-123",
		StepID:         "step-456",
		TraceID:        "trace-789",
		AggregateCount: 3,
		AggregateFirst: 100,
		AggregateLast:  200,
		Data:           json.RawMessage(`{"method":"GET","url":"/api/test","status":200}`),
	}
	if err := store.WriteEventDirect(event); err != nil {
		t.Fatalf("Failed to write event: %v", err)
	}

	exported, err := store.ExportSessionEvents(session.ID)
	if err != nil {
		t.Fatalf("ExportSessionEvents failed: %v", err)
	}

	if len(exported) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(exported))
	}

	e := exported[0]
	if e.ID != event.ID {
		t.Errorf("ID mismatch")
	}
	if e.Duration != 150 {
		t.Errorf("Duration mismatch: got %d, want 150", e.Duration)
	}
	if e.Source != SourceNetwork {
		t.Errorf("Source mismatch: got %s, want %s", e.Source, SourceNetwork)
	}
	if e.Category != CategoryNetwork {
		t.Errorf("Category mismatch: got %s, want %s", e.Category, CategoryNetwork)
	}
	if e.Level != LevelWarn {
		t.Errorf("Level mismatch: got %s, want %s", e.Level, LevelWarn)
	}
	if e.Summary != "HTTP request summary" {
		t.Errorf("Summary mismatch: got %s", e.Summary)
	}
	if e.ParentID != "parent-123" {
		t.Errorf("ParentID mismatch: got %s", e.ParentID)
	}
	if e.StepID != "step-456" {
		t.Errorf("StepID mismatch: got %s", e.StepID)
	}
	if e.TraceID != "trace-789" {
		t.Errorf("TraceID mismatch: got %s", e.TraceID)
	}
	if e.AggregateCount != 3 {
		t.Errorf("AggregateCount mismatch: got %d", e.AggregateCount)
	}

	// Verify Data JSON
	var data map[string]interface{}
	if err := json.Unmarshal(e.Data, &data); err != nil {
		t.Fatalf("Failed to unmarshal event data: %v", err)
	}
	if data["method"] != "GET" {
		t.Errorf("Data.method mismatch: got %v", data["method"])
	}
}

func TestExportSessionEvents_LargeEventCount(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := createTestSession(t, store, "Large Export")
	createTestEvents(t, store, session.ID, session.DeviceID, 500)

	exported, err := store.ExportSessionEvents(session.ID)
	if err != nil {
		t.Fatalf("ExportSessionEvents failed: %v", err)
	}

	if len(exported) != 500 {
		t.Errorf("Expected 500 events, got %d", len(exported))
	}
}

// ========================================
// EventStore.ImportSession Tests
// ========================================

func TestImportSession_WithEventsAndBookmarks(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := &DeviceSession{
		ID:            "imported-001",
		DeviceID:      "device-xyz",
		Type:          "manual",
		Name:          "Imported Session",
		StartTime:     time.Now().UnixMilli() - 60000,
		EndTime:       time.Now().UnixMilli(),
		Status:        "completed",
		VideoPath:     "/path/to/video.mp4",
		VideoDuration: 30000,
		Metadata:      map[string]any{"source": "export"},
	}

	events := []UnifiedEvent{
		{
			ID:           "evt-001",
			SessionID:    session.ID,
			DeviceID:     session.DeviceID,
			Timestamp:    time.Now().UnixMilli(),
			RelativeTime: 100,
			Source:       SourceLogcat,
			Category:     CategoryLog,
			Type:         "logcat",
			Level:        LevelInfo,
			Title:        "Test event 1",
			Data:         json.RawMessage(`{"tag":"Test"}`),
		},
		{
			ID:           "evt-002",
			SessionID:    session.ID,
			DeviceID:     session.DeviceID,
			Timestamp:    time.Now().UnixMilli() + 100,
			RelativeTime: 200,
			Source:       SourceNetwork,
			Category:     CategoryNetwork,
			Type:         "http_request",
			Level:        LevelWarn,
			Title:        "GET /api",
			Summary:      "HTTP request",
		},
	}

	bookmarks := []Bookmark{
		{
			ID:           "bm-001",
			SessionID:    session.ID,
			RelativeTime: 100,
			Label:        "Important point",
			Color:        "#ff0000",
			Type:         "user",
			CreatedAt:    time.Now().UnixMilli(),
		},
	}

	err := store.ImportSession(session, events, bookmarks)
	if err != nil {
		t.Fatalf("ImportSession failed: %v", err)
	}

	// Verify session was persisted
	got, err := store.GetSession("imported-001")
	if err != nil {
		t.Fatalf("Failed to get imported session: %v", err)
	}
	if got == nil {
		t.Fatal("Imported session not found")
	}
	if got.Name != "Imported Session" {
		t.Errorf("Session name mismatch: got %s", got.Name)
	}
	if got.Status != "completed" {
		t.Errorf("Session status mismatch: got %s", got.Status)
	}
	if got.VideoPath != "/path/to/video.mp4" {
		t.Errorf("Session video path mismatch: got %s", got.VideoPath)
	}
	if got.EventCount != 2 {
		t.Errorf("Session event count should be set to actual event count: got %d, want 2", got.EventCount)
	}

	// Verify events were persisted
	result, err := store.QueryEvents(EventQuery{SessionID: session.ID, Limit: 100, IncludeData: true})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("Expected 2 events, got %d", result.Total)
	}

	// Verify event data was persisted
	foundData := false
	for _, e := range result.Events {
		if e.ID == "evt-001" && len(e.Data) > 0 {
			foundData = true
			var data map[string]interface{}
			json.Unmarshal(e.Data, &data)
			if data["tag"] != "Test" {
				t.Errorf("Event data tag mismatch: got %v", data["tag"])
			}
		}
	}
	if !foundData {
		t.Error("Event data was not persisted")
	}

	// Verify bookmarks were persisted
	bms, err := store.GetBookmarks(session.ID)
	if err != nil {
		t.Fatalf("Failed to get bookmarks: %v", err)
	}
	if len(bms) != 1 {
		t.Errorf("Expected 1 bookmark, got %d", len(bms))
	}
	if len(bms) > 0 && bms[0].Label != "Important point" {
		t.Errorf("Bookmark label mismatch: got %s", bms[0].Label)
	}
}

func TestImportSession_EmptyEvents(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := &DeviceSession{
		ID:        "imported-empty",
		DeviceID:  "device-xyz",
		Type:      "manual",
		Name:      "Empty Import",
		StartTime: time.Now().UnixMilli(),
		Status:    "completed",
	}

	err := store.ImportSession(session, nil, nil)
	if err != nil {
		t.Fatalf("ImportSession with empty events failed: %v", err)
	}

	got, err := store.GetSession("imported-empty")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if got == nil {
		t.Fatal("Session not found")
	}
	if got.EventCount != 0 {
		t.Errorf("Expected 0 event count, got %d", got.EventCount)
	}
}

func TestImportSession_EmptyBookmarks(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := &DeviceSession{
		ID:        "imported-nobm",
		DeviceID:  "device-xyz",
		Type:      "manual",
		Name:      "No Bookmarks",
		StartTime: time.Now().UnixMilli(),
		Status:    "completed",
	}
	events := []UnifiedEvent{
		{
			ID:           "evt-x",
			SessionID:    session.ID,
			DeviceID:     session.DeviceID,
			Timestamp:    time.Now().UnixMilli(),
			RelativeTime: 0,
			Source:       SourceLogcat,
			Category:     CategoryLog,
			Type:         "logcat",
			Level:        LevelInfo,
			Title:        "Test",
		},
	}

	err := store.ImportSession(session, events, nil)
	if err != nil {
		t.Fatalf("ImportSession failed: %v", err)
	}

	bms, err := store.GetBookmarks(session.ID)
	if err != nil {
		t.Fatalf("Failed to get bookmarks: %v", err)
	}
	if len(bms) != 0 {
		t.Errorf("Expected 0 bookmarks, got %d", len(bms))
	}
}

func TestImportSession_DuplicateIDFails(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := createTestSession(t, store, "Original")

	// Try to import with the same ID
	dup := &DeviceSession{
		ID:        session.ID,
		DeviceID:  "device-xyz",
		Type:      "manual",
		Name:      "Duplicate",
		StartTime: time.Now().UnixMilli(),
		Status:    "completed",
	}

	err := store.ImportSession(dup, nil, nil)
	if err == nil {
		t.Error("ImportSession should fail for duplicate session ID")
	}
}

// ========================================
// App.ExportSessionToPath Tests
// ========================================

func TestExportSessionToPath_Success(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	// Create session with events and bookmarks
	session := createTestSession(t, app.eventStore, "Export Full Test")
	createTestEvents(t, app.eventStore, session.ID, session.DeviceID, 10)
	createTestBookmarks(t, app.eventStore, session.ID, 3)

	outputPath := filepath.Join(tempDir, "test_export.gaze")

	result, err := app.ExportSessionToPath(session.ID, outputPath)
	if err != nil {
		t.Fatalf("ExportSessionToPath failed: %v", err)
	}

	if result != outputPath {
		t.Errorf("Result path mismatch: got %s, want %s", result, outputPath)
	}

	// Verify file exists
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Output file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Output file is empty")
	}

	// Verify ZIP contents
	entries := listZipEntries(t, outputPath)
	expectedEntries := map[string]bool{
		"manifest.json":  false,
		"session.json":   false,
		"events.jsonl":   false,
		"bookmarks.json": false,
	}
	for _, name := range entries {
		if _, ok := expectedEntries[name]; ok {
			expectedEntries[name] = true
		}
	}
	for name, found := range expectedEntries {
		if !found {
			t.Errorf("Expected zip entry %s not found", name)
		}
	}

	// Verify manifest
	manifestData := readZipEntry(t, outputPath, "manifest.json")
	if manifestData == nil {
		t.Fatal("manifest.json not found in archive")
	}
	var manifest GazeExportManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("Failed to parse manifest: %v", err)
	}
	if manifest.FormatVersion != 1 {
		t.Errorf("Manifest FormatVersion: got %d, want 1", manifest.FormatVersion)
	}
	if manifest.AppVersion != "1.0.0-test" {
		t.Errorf("Manifest AppVersion: got %s, want 1.0.0-test", manifest.AppVersion)
	}
	if manifest.EventCount != 10 {
		t.Errorf("Manifest EventCount: got %d, want 10", manifest.EventCount)
	}
	if manifest.SessionID != session.ID {
		t.Errorf("Manifest SessionID: got %s, want %s", manifest.SessionID, session.ID)
	}
	if manifest.HasBookmarks != true {
		t.Error("Manifest HasBookmarks should be true")
	}
	if manifest.HasVideo != false {
		t.Error("Manifest HasVideo should be false (no video)")
	}

	// Verify session.json
	sessionData := readZipEntry(t, outputPath, "session.json")
	if sessionData == nil {
		t.Fatal("session.json not found in archive")
	}
	var exportedSession DeviceSession
	if err := json.Unmarshal(sessionData, &exportedSession); err != nil {
		t.Fatalf("Failed to parse session.json: %v", err)
	}
	if exportedSession.ID != session.ID {
		t.Errorf("Session ID mismatch: got %s", exportedSession.ID)
	}
	if exportedSession.Name != "Export Full Test" {
		t.Errorf("Session Name mismatch: got %s", exportedSession.Name)
	}

	// Verify events.jsonl
	eventsData := readZipEntry(t, outputPath, "events.jsonl")
	if eventsData == nil {
		t.Fatal("events.jsonl not found in archive")
	}
	lines := strings.Split(strings.TrimSpace(string(eventsData)), "\n")
	if len(lines) != 10 {
		t.Errorf("Expected 10 event lines, got %d", len(lines))
	}
	// Verify each line is valid JSON
	for i, line := range lines {
		var event UnifiedEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Errorf("Event line %d is not valid JSON: %v", i, err)
		}
	}

	// Verify bookmarks.json
	bookmarkRaw := readZipEntry(t, outputPath, "bookmarks.json")
	if bookmarkRaw == nil {
		t.Fatal("bookmarks.json not found in archive")
	}
	var bookmarks []Bookmark
	if err := json.Unmarshal(bookmarkRaw, &bookmarks); err != nil {
		t.Fatalf("Failed to parse bookmarks.json: %v", err)
	}
	if len(bookmarks) != 3 {
		t.Errorf("Expected 3 bookmarks, got %d", len(bookmarks))
	}
}

func TestExportSessionToPath_SessionNotFound(t *testing.T) {
	app, _, cleanup := setupTestAppForExport(t)
	defer cleanup()

	_, err := app.ExportSessionToPath("nonexistent-id", "/tmp/test.gaze")
	if err == nil {
		t.Error("ExportSessionToPath should fail for nonexistent session")
	}
	if !strings.Contains(err.Error(), "session not found") {
		t.Errorf("Error should mention 'session not found', got: %v", err)
	}
}

func TestExportSessionToPath_AutoAppendExtension(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	session := createTestSession(t, app.eventStore, "Extension Test")

	// Path without .gaze extension
	outputPath := filepath.Join(tempDir, "test_export")

	result, err := app.ExportSessionToPath(session.ID, outputPath)
	if err != nil {
		t.Fatalf("ExportSessionToPath failed: %v", err)
	}

	if !strings.HasSuffix(result, ".gaze") {
		t.Errorf("Result should have .gaze extension, got: %s", result)
	}

	// Verify file was created at path with extension
	if _, err := os.Stat(result); err != nil {
		t.Errorf("File not created at expected path: %v", err)
	}
}

func TestExportSessionToPath_NoBookmarks(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	session := createTestSession(t, app.eventStore, "No Bookmarks")
	createTestEvents(t, app.eventStore, session.ID, session.DeviceID, 3)

	outputPath := filepath.Join(tempDir, "no_bookmarks.gaze")
	_, err := app.ExportSessionToPath(session.ID, outputPath)
	if err != nil {
		t.Fatalf("ExportSessionToPath failed: %v", err)
	}

	// bookmarks.json should NOT be present since there are no bookmarks
	bookmarkData := readZipEntry(t, outputPath, "bookmarks.json")
	if bookmarkData != nil {
		t.Error("bookmarks.json should not be present when there are no bookmarks")
	}

	// Manifest should reflect no bookmarks
	manifestData := readZipEntry(t, outputPath, "manifest.json")
	var manifest GazeExportManifest
	json.Unmarshal(manifestData, &manifest)
	if manifest.HasBookmarks {
		t.Error("Manifest HasBookmarks should be false")
	}
}

func TestExportSessionToPath_EmptySession(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	session := createTestSession(t, app.eventStore, "Empty")

	outputPath := filepath.Join(tempDir, "empty.gaze")
	_, err := app.ExportSessionToPath(session.ID, outputPath)
	if err != nil {
		t.Fatalf("ExportSessionToPath failed: %v", err)
	}

	// Still should have manifest and session
	entries := listZipEntries(t, outputPath)
	hasManifest := false
	hasSession := false
	for _, name := range entries {
		if name == "manifest.json" {
			hasManifest = true
		}
		if name == "session.json" {
			hasSession = true
		}
	}
	if !hasManifest {
		t.Error("Empty export should still have manifest.json")
	}
	if !hasSession {
		t.Error("Empty export should still have session.json")
	}

	manifestData := readZipEntry(t, outputPath, "manifest.json")
	var manifest GazeExportManifest
	json.Unmarshal(manifestData, &manifest)
	if manifest.EventCount != 0 {
		t.Errorf("Manifest EventCount should be 0 for empty session, got %d", manifest.EventCount)
	}
}

func TestExportSessionToPath_CreatesDirectory(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	session := createTestSession(t, app.eventStore, "Dir Test")

	// Use a nested path that doesn't exist
	outputPath := filepath.Join(tempDir, "nested", "dir", "test.gaze")

	_, err := app.ExportSessionToPath(session.ID, outputPath)
	if err != nil {
		t.Fatalf("ExportSessionToPath should create directories: %v", err)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("File should exist at nested path: %v", err)
	}
}

// ========================================
// App.ImportSessionFromPath Tests
// ========================================

func TestImportSessionFromPath_Success(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	// Create and export a session
	session := createTestSession(t, app.eventStore, "Original Session")
	createTestEvents(t, app.eventStore, session.ID, session.DeviceID, 5)
	createTestBookmarks(t, app.eventStore, session.ID, 2)

	exportPath := filepath.Join(tempDir, "for_import.gaze")
	_, err := app.ExportSessionToPath(session.ID, exportPath)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Import the exported session
	newID, err := app.ImportSessionFromPath(exportPath)
	if err != nil {
		t.Fatalf("ImportSessionFromPath failed: %v", err)
	}

	if newID == "" {
		t.Fatal("New session ID should not be empty")
	}

	// New ID should be different from original
	if newID == session.ID {
		t.Error("Imported session should have a new ID")
	}

	// Verify imported session
	imported, err := app.eventStore.GetSession(newID)
	if err != nil {
		t.Fatalf("Failed to get imported session: %v", err)
	}
	if imported == nil {
		t.Fatal("Imported session not found")
	}

	// Verify name has "(imported)" suffix
	if !strings.Contains(imported.Name, "(imported)") {
		t.Errorf("Imported session name should contain '(imported)': got %s", imported.Name)
	}

	// Verify status is "completed"
	if imported.Status != "completed" {
		t.Errorf("Imported session status should be 'completed': got %s", imported.Status)
	}

	// Verify events were imported with correct sessionID
	result, err := app.eventStore.QueryEvents(EventQuery{SessionID: newID, Limit: 100})
	if err != nil {
		t.Fatalf("Failed to query imported events: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("Expected 5 imported events, got %d", result.Total)
	}
	for _, e := range result.Events {
		if e.SessionID != newID {
			t.Errorf("Imported event sessionID should be %s, got %s", newID, e.SessionID)
		}
	}

	// Verify bookmarks were imported
	bms, err := app.eventStore.GetBookmarks(newID)
	if err != nil {
		t.Fatalf("Failed to get imported bookmarks: %v", err)
	}
	if len(bms) != 2 {
		t.Errorf("Expected 2 imported bookmarks, got %d", len(bms))
	}
	for _, bm := range bms {
		if bm.SessionID != newID {
			t.Errorf("Imported bookmark sessionID should be %s, got %s", newID, bm.SessionID)
		}
	}
}

func TestImportSessionFromPath_FileNotFound(t *testing.T) {
	app, _, cleanup := setupTestAppForExport(t)
	defer cleanup()

	_, err := app.ImportSessionFromPath("/nonexistent/path/session.gaze")
	if err == nil {
		t.Error("ImportSessionFromPath should fail for nonexistent file")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("Error should mention 'file not found', got: %v", err)
	}
}

func TestImportSessionFromPath_InvalidArchive(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	// Create a non-ZIP file
	badPath := filepath.Join(tempDir, "bad.gaze")
	os.WriteFile(badPath, []byte("this is not a zip file"), 0644)

	_, err := app.ImportSessionFromPath(badPath)
	if err == nil {
		t.Error("ImportSessionFromPath should fail for non-ZIP file")
	}
	if !strings.Contains(err.Error(), "failed to open archive") {
		t.Errorf("Error should mention 'failed to open archive', got: %v", err)
	}
}

func TestImportSessionFromPath_MissingSessionJson(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	// Create a valid ZIP but without session.json
	badArchivePath := filepath.Join(tempDir, "no_session.gaze")
	zipFile, err := os.Create(badArchivePath)
	if err != nil {
		t.Fatalf("Failed to create zip: %v", err)
	}
	w := zip.NewWriter(zipFile)
	writer, _ := w.Create("manifest.json")
	writer.Write([]byte(`{"formatVersion":1}`))
	w.Close()
	zipFile.Close()

	_, err = app.ImportSessionFromPath(badArchivePath)
	if err == nil {
		t.Error("ImportSessionFromPath should fail when session.json is missing")
	}
	if !strings.Contains(err.Error(), "missing session.json") {
		t.Errorf("Error should mention 'missing session.json', got: %v", err)
	}
}

func TestImportSessionFromPath_EmptyEventsAndBookmarks(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	// Export a session with no events
	session := createTestSession(t, app.eventStore, "Empty Export")
	exportPath := filepath.Join(tempDir, "empty_export.gaze")
	_, err := app.ExportSessionToPath(session.ID, exportPath)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	newID, err := app.ImportSessionFromPath(exportPath)
	if err != nil {
		t.Fatalf("ImportSessionFromPath failed: %v", err)
	}

	imported, _ := app.eventStore.GetSession(newID)
	if imported == nil {
		t.Fatal("Imported session not found")
	}
	if imported.EventCount != 0 {
		t.Errorf("Expected 0 events, got %d", imported.EventCount)
	}
}

// ========================================
// App.ExportSessionToPath with Video Tests
// ========================================

func TestExportSessionToPath_WithVideo(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	// Create a fake video file
	videoPath := filepath.Join(tempDir, "test_recording.mp4")
	fakeVideoData := []byte("fake mp4 video data for testing purposes")
	if err := os.WriteFile(videoPath, fakeVideoData, 0644); err != nil {
		t.Fatalf("Failed to create fake video: %v", err)
	}

	// Create session with video
	session := &DeviceSession{
		ID:            uuid.New().String()[:8],
		DeviceID:      "test-device-001",
		Type:          "recording",
		Name:          "Video Session",
		StartTime:     time.Now().UnixMilli(),
		EndTime:       time.Now().UnixMilli() + 30000,
		Status:        "completed",
		VideoPath:     videoPath,
		VideoDuration: 30000,
	}
	if err := app.eventStore.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	createTestEvents(t, app.eventStore, session.ID, session.DeviceID, 3)

	outputPath := filepath.Join(tempDir, "with_video.gaze")
	_, err := app.ExportSessionToPath(session.ID, outputPath)
	if err != nil {
		t.Fatalf("ExportSessionToPath failed: %v", err)
	}

	// Verify recording.mp4 is in the archive
	entries := listZipEntries(t, outputPath)
	hasRecording := false
	for _, name := range entries {
		if strings.HasPrefix(name, "recording") {
			hasRecording = true
			break
		}
	}
	if !hasRecording {
		t.Error("Archive should contain recording file")
	}

	// Verify recording data
	recordingData := readZipEntry(t, outputPath, "recording.mp4")
	if recordingData == nil {
		t.Fatal("recording.mp4 not found in archive")
	}
	if string(recordingData) != string(fakeVideoData) {
		t.Error("Recording data in archive doesn't match original")
	}

	// Verify manifest reflects video
	manifestData := readZipEntry(t, outputPath, "manifest.json")
	var manifest GazeExportManifest
	json.Unmarshal(manifestData, &manifest)
	if !manifest.HasVideo {
		t.Error("Manifest HasVideo should be true")
	}
}

func TestImportSessionFromPath_WithVideo(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	// Create a fake video file
	videoPath := filepath.Join(tempDir, "test_video.mp4")
	fakeVideoData := []byte("fake mp4 video data for import testing")
	os.WriteFile(videoPath, fakeVideoData, 0644)

	// Create session with video
	session := &DeviceSession{
		ID:            uuid.New().String()[:8],
		DeviceID:      "test-device-001",
		Type:          "recording",
		Name:          "Video Import Test",
		StartTime:     time.Now().UnixMilli(),
		EndTime:       time.Now().UnixMilli() + 30000,
		Status:        "completed",
		VideoPath:     videoPath,
		VideoDuration: 30000,
	}
	app.eventStore.CreateSession(session)

	// Export
	exportPath := filepath.Join(tempDir, "video_export.gaze")
	_, err := app.ExportSessionToPath(session.ID, exportPath)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Import
	newID, err := app.ImportSessionFromPath(exportPath)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify imported session has a video path
	imported, _ := app.eventStore.GetSession(newID)
	if imported == nil {
		t.Fatal("Imported session not found")
	}
	if imported.VideoPath == "" {
		t.Error("Imported session should have a video path")
	}

	// Verify video file was extracted
	if _, err := os.Stat(imported.VideoPath); err != nil {
		t.Errorf("Imported video file should exist at %s: %v", imported.VideoPath, err)
	}

	// Verify video content
	extractedData, err := os.ReadFile(imported.VideoPath)
	if err != nil {
		t.Fatalf("Failed to read extracted video: %v", err)
	}
	if string(extractedData) != string(fakeVideoData) {
		t.Error("Extracted video data doesn't match original")
	}

	// Cleanup extracted video
	os.Remove(imported.VideoPath)
}

// ========================================
// Round-Trip Tests
// ========================================

func TestExportImport_RoundTrip(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	// Create rich session data
	session := createTestSession(t, app.eventStore, "Round Trip Test")
	originalEvents := createTestEvents(t, app.eventStore, session.ID, session.DeviceID, 20)
	originalBookmarks := createTestBookmarks(t, app.eventStore, session.ID, 5)

	// Export
	exportPath := filepath.Join(tempDir, "roundtrip.gaze")
	_, err := app.ExportSessionToPath(session.ID, exportPath)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Import
	newID, err := app.ImportSessionFromPath(exportPath)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify session metadata
	imported, _ := app.eventStore.GetSession(newID)
	if imported == nil {
		t.Fatal("Imported session not found")
	}
	if imported.DeviceID != session.DeviceID {
		t.Errorf("DeviceID mismatch: got %s, want %s", imported.DeviceID, session.DeviceID)
	}
	if imported.Type != session.Type {
		t.Errorf("Type mismatch: got %s, want %s", imported.Type, session.Type)
	}

	// Verify all events round-tripped
	result, err := app.eventStore.QueryEvents(EventQuery{
		SessionID:   newID,
		Limit:       100,
		IncludeData: true,
	})
	if err != nil {
		t.Fatalf("Failed to query imported events: %v", err)
	}
	if result.Total != 20 {
		t.Errorf("Expected 20 events, got %d", result.Total)
	}

	// Verify event data round-tripped
	for _, e := range result.Events {
		if len(e.Data) == 0 {
			t.Errorf("Event %s: Data should not be empty after round-trip", e.ID)
		}
	}

	// Verify imported events have new IDs (not same as originals)
	importedIDs := make(map[string]bool)
	for _, e := range result.Events {
		importedIDs[e.ID] = true
	}
	for _, orig := range originalEvents {
		if importedIDs[orig.ID] {
			t.Errorf("Imported event should have new ID, but found original ID %s", orig.ID)
		}
	}

	// Verify event content is preserved (titles match)
	originalTitles := make(map[string]bool)
	for _, orig := range originalEvents {
		originalTitles[orig.Title] = true
	}
	for _, e := range result.Events {
		if !originalTitles[e.Title] {
			t.Errorf("Imported event title %q not found in originals", e.Title)
		}
	}

	// Verify bookmarks round-tripped
	bms, err := app.eventStore.GetBookmarks(newID)
	if err != nil {
		t.Fatalf("Failed to get imported bookmarks: %v", err)
	}
	if len(bms) != 5 {
		t.Errorf("Expected 5 bookmarks, got %d", len(bms))
	}

	// Verify bookmarks have new IDs (not same as originals)
	importedBmIDs := make(map[string]bool)
	for _, bm := range bms {
		importedBmIDs[bm.ID] = true
	}
	for _, orig := range originalBookmarks {
		if importedBmIDs[orig.ID] {
			t.Errorf("Imported bookmark should have new ID, but found original ID %s", orig.ID)
		}
	}

	// Verify bookmark content is preserved (labels match)
	originalLabels := make(map[string]bool)
	for _, orig := range originalBookmarks {
		originalLabels[orig.Label] = true
	}
	for _, bm := range bms {
		if !originalLabels[bm.Label] {
			t.Errorf("Imported bookmark label %q not found in originals", bm.Label)
		}
	}
}

func TestExportImport_RoundTrip_EventFields(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	session := createTestSession(t, app.eventStore, "Field Round Trip")

	// Write event with all possible fields
	original := UnifiedEvent{
		ID:                uuid.New().String(),
		SessionID:         session.ID,
		DeviceID:          session.DeviceID,
		Timestamp:         time.Now().UnixMilli(),
		RelativeTime:      1234,
		Duration:          567,
		Source:            SourceNetwork,
		Category:          CategoryNetwork,
		Type:              "http_request",
		Level:             LevelError,
		Title:             "POST /api/data",
		Summary:           "Failed request",
		ParentID:          "parent-abc",
		StepID:            "step-def",
		TraceID:           "trace-ghi",
		AggregateCount:    7,
		AggregateFirst:    1000,
		AggregateLast:     2000,
		Tags:              []string{"plugin-tag", "validated"},
		Metadata:          map[string]interface{}{"generatedBy": "test-plugin"},
		ParentEventID:     "parent-event-xyz",
		GeneratedByPlugin: "test-plugin",
		Data:              json.RawMessage(`{"method":"POST","url":"/api/data","status":500,"body":"error"}`),
	}
	if err := app.eventStore.WriteEventDirect(original); err != nil {
		t.Fatalf("Failed to write event: %v", err)
	}

	// Export and import
	exportPath := filepath.Join(tempDir, "field_roundtrip.gaze")
	app.ExportSessionToPath(session.ID, exportPath)
	newID, err := app.ImportSessionFromPath(exportPath)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Retrieve imported event
	result, _ := app.eventStore.QueryEvents(EventQuery{
		SessionID:   newID,
		Limit:       10,
		IncludeData: true,
	})
	if result.Total != 1 {
		t.Fatalf("Expected 1 event, got %d", result.Total)
	}

	e := result.Events[0]

	// Verify ID is new (import generates new IDs to avoid conflicts)
	if e.ID == original.ID {
		t.Errorf("Imported event should have new ID, but got original ID %s", e.ID)
	}
	// Verify all other fields survived the round-trip
	if e.RelativeTime != original.RelativeTime {
		t.Errorf("RelativeTime mismatch: got %d, want %d", e.RelativeTime, original.RelativeTime)
	}
	if e.Duration != original.Duration {
		t.Errorf("Duration mismatch: got %d, want %d", e.Duration, original.Duration)
	}
	if e.Source != original.Source {
		t.Errorf("Source mismatch: got %s, want %s", e.Source, original.Source)
	}
	if e.Category != original.Category {
		t.Errorf("Category mismatch: got %s, want %s", e.Category, original.Category)
	}
	if e.Type != original.Type {
		t.Errorf("Type mismatch: got %s, want %s", e.Type, original.Type)
	}
	if e.Level != original.Level {
		t.Errorf("Level mismatch: got %s, want %s", e.Level, original.Level)
	}
	if e.Title != original.Title {
		t.Errorf("Title mismatch: got %s, want %s", e.Title, original.Title)
	}
	if e.Summary != original.Summary {
		t.Errorf("Summary mismatch: got %s, want %s", e.Summary, original.Summary)
	}
	if e.ParentID != original.ParentID {
		t.Errorf("ParentID mismatch: got %s, want %s", e.ParentID, original.ParentID)
	}
	if e.StepID != original.StepID {
		t.Errorf("StepID mismatch: got %s, want %s", e.StepID, original.StepID)
	}
	if e.TraceID != original.TraceID {
		t.Errorf("TraceID mismatch: got %s, want %s", e.TraceID, original.TraceID)
	}
	if e.AggregateCount != original.AggregateCount {
		t.Errorf("AggregateCount mismatch: got %d, want %d", e.AggregateCount, original.AggregateCount)
	}
	if e.AggregateFirst != original.AggregateFirst {
		t.Errorf("AggregateFirst mismatch: got %d, want %d", e.AggregateFirst, original.AggregateFirst)
	}
	if e.AggregateLast != original.AggregateLast {
		t.Errorf("AggregateLast mismatch: got %d, want %d", e.AggregateLast, original.AggregateLast)
	}
	if len(e.Tags) != 2 || e.Tags[0] != "plugin-tag" || e.Tags[1] != "validated" {
		t.Errorf("Tags mismatch: got %v, want %v", e.Tags, original.Tags)
	}
	if e.Metadata == nil || e.Metadata["generatedBy"] != "test-plugin" {
		t.Errorf("Metadata mismatch: got %v, want %v", e.Metadata, original.Metadata)
	}
	if e.ParentEventID != original.ParentEventID {
		t.Errorf("ParentEventID mismatch: got %s, want %s", e.ParentEventID, original.ParentEventID)
	}
	if e.GeneratedByPlugin != original.GeneratedByPlugin {
		t.Errorf("GeneratedByPlugin mismatch: got %s, want %s", e.GeneratedByPlugin, original.GeneratedByPlugin)
	}

	// Verify Data JSON round-trip
	var originalData, importedData map[string]interface{}
	json.Unmarshal(original.Data, &originalData)
	json.Unmarshal(e.Data, &importedData)
	if importedData["method"] != originalData["method"] {
		t.Errorf("Data.method mismatch: got %v, want %v", importedData["method"], originalData["method"])
	}
	if importedData["status"] != originalData["status"] {
		t.Errorf("Data.status mismatch: got %v, want %v", importedData["status"], originalData["status"])
	}
}

func TestExportImport_MultipleImports(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	session := createTestSession(t, app.eventStore, "Multi Import")
	createTestEvents(t, app.eventStore, session.ID, session.DeviceID, 3)

	exportPath := filepath.Join(tempDir, "multi.gaze")
	app.ExportSessionToPath(session.ID, exportPath)

	// Import the same file multiple times
	var importedIDs []string
	for i := 0; i < 3; i++ {
		newID, err := app.ImportSessionFromPath(exportPath)
		if err != nil {
			t.Fatalf("Import %d failed: %v", i, err)
		}
		importedIDs = append(importedIDs, newID)
	}

	// All imported IDs should be unique
	idSet := make(map[string]bool)
	for _, id := range importedIDs {
		if idSet[id] {
			t.Errorf("Duplicate imported session ID: %s", id)
		}
		idSet[id] = true
	}

	// Original session should still exist
	original, _ := app.eventStore.GetSession(session.ID)
	if original == nil {
		t.Error("Original session should still exist after imports")
	}

	// Each import should have its own events
	for _, id := range importedIDs {
		result, _ := app.eventStore.QueryEvents(EventQuery{SessionID: id, Limit: 100})
		if result.Total != 3 {
			t.Errorf("Import %s should have 3 events, got %d", id, result.Total)
		}
	}
}

// ========================================
// Edge Cases and Error Handling
// ========================================

func TestExportSessionToPath_SpecialCharactersInName(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	// Session with special characters in name
	session := &DeviceSession{
		ID:        uuid.New().String()[:8],
		DeviceID:  "test-device-001",
		Type:      "manual",
		Name:      "Debug Session / Test & More <Special>",
		StartTime: time.Now().UnixMilli(),
		Status:    "completed",
	}
	app.eventStore.CreateSession(session)

	outputPath := filepath.Join(tempDir, "special.gaze")
	result, err := app.ExportSessionToPath(session.ID, outputPath)
	if err != nil {
		t.Fatalf("ExportSessionToPath should handle special chars: %v", err)
	}
	if result == "" {
		t.Error("Should return non-empty path")
	}
}

func TestImportSessionFromPath_MalformedEventsJsonl(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	// Create a .gaze archive with malformed events
	archivePath := filepath.Join(tempDir, "malformed_events.gaze")
	zipFile, _ := os.Create(archivePath)
	w := zip.NewWriter(zipFile)

	// Valid session.json
	sw, _ := w.Create("session.json")
	sw.Write([]byte(`{"id":"test-123","deviceId":"dev-1","type":"manual","name":"Test","startTime":1000,"status":"completed"}`))

	// Malformed events - mix of valid and invalid lines
	ew, _ := w.Create("events.jsonl")
	ew.Write([]byte(`{"id":"e1","sessionId":"test-123","deviceId":"dev-1","timestamp":1000,"relativeTime":0,"source":"logcat","category":"log","type":"logcat","level":"info","title":"Valid event"}
this is not json
{"id":"e2","sessionId":"test-123","deviceId":"dev-1","timestamp":1100,"relativeTime":100,"source":"logcat","category":"log","type":"logcat","level":"info","title":"Another valid event"}
`))

	w.Close()
	zipFile.Close()

	// Import should succeed, skipping malformed lines
	newID, err := app.ImportSessionFromPath(archivePath)
	if err != nil {
		t.Fatalf("ImportSessionFromPath should skip malformed lines: %v", err)
	}

	// Should have imported the valid events
	result, _ := app.eventStore.QueryEvents(EventQuery{SessionID: newID, Limit: 100})
	if result.Total != 2 {
		t.Errorf("Expected 2 valid events (skipping malformed), got %d", result.Total)
	}
}

func TestImportSessionFromPath_MalformedBookmarks(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	archivePath := filepath.Join(tempDir, "bad_bookmarks.gaze")
	zipFile, _ := os.Create(archivePath)
	w := zip.NewWriter(zipFile)

	sw, _ := w.Create("session.json")
	sw.Write([]byte(`{"id":"test-bm","deviceId":"dev-1","type":"manual","name":"Test","startTime":1000,"status":"completed"}`))

	// Malformed bookmarks
	bw, _ := w.Create("bookmarks.json")
	bw.Write([]byte(`not a valid json array`))

	w.Close()
	zipFile.Close()

	// Import should succeed, skipping malformed bookmarks
	newID, err := app.ImportSessionFromPath(archivePath)
	if err != nil {
		t.Fatalf("ImportSessionFromPath should handle malformed bookmarks: %v", err)
	}

	// Session should still be imported
	imported, _ := app.eventStore.GetSession(newID)
	if imported == nil {
		t.Fatal("Session should be imported despite malformed bookmarks")
	}
}

func TestExportSessionToPath_WithVideoButMissingFile(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	// Session with non-existent video path
	session := &DeviceSession{
		ID:        uuid.New().String()[:8],
		DeviceID:  "test-device-001",
		Type:      "recording",
		Name:      "Missing Video",
		StartTime: time.Now().UnixMilli(),
		Status:    "completed",
		VideoPath: "/nonexistent/video.mp4",
	}
	app.eventStore.CreateSession(session)

	outputPath := filepath.Join(tempDir, "missing_video.gaze")
	_, err := app.ExportSessionToPath(session.ID, outputPath)
	if err != nil {
		t.Fatalf("ExportSessionToPath should succeed even when video is missing: %v", err)
	}

	// Archive should not contain recording
	entries := listZipEntries(t, outputPath)
	for _, name := range entries {
		if strings.HasPrefix(name, "recording") {
			t.Error("Archive should not contain recording when video file is missing")
		}
	}

	// Manifest should reflect no video
	manifestData := readZipEntry(t, outputPath, "manifest.json")
	var manifest GazeExportManifest
	json.Unmarshal(manifestData, &manifest)
	if manifest.HasVideo {
		t.Error("Manifest HasVideo should be false when video is missing")
	}
}

func TestManifest_ExportTime(t *testing.T) {
	app, tempDir, cleanup := setupTestAppForExport(t)
	defer cleanup()

	session := createTestSession(t, app.eventStore, "Manifest Time")

	before := time.Now().UnixMilli()

	outputPath := filepath.Join(tempDir, "manifest_time.gaze")
	app.ExportSessionToPath(session.ID, outputPath)

	after := time.Now().UnixMilli()

	manifestData := readZipEntry(t, outputPath, "manifest.json")
	var manifest GazeExportManifest
	json.Unmarshal(manifestData, &manifest)

	if manifest.ExportTime < before || manifest.ExportTime > after {
		t.Errorf("ExportTime %d should be between %d and %d", manifest.ExportTime, before, after)
	}
}
