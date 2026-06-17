package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// setupTestStore creates a temporary EventStore for testing
func setupTestStore(t *testing.T) (*EventStore, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "event_store_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	store, err := NewEventStore(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create EventStore: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

// TestEventStoreCreation tests that EventStore can be created and closed
func TestEventStoreCreation(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	if store == nil {
		t.Fatal("Store should not be nil")
	}

	if store.db == nil {
		t.Fatal("Database connection should not be nil")
	}

	// Verify database file exists
	if _, err := os.Stat(store.dbPath); os.IsNotExist(err) {
		t.Fatalf("Database file should exist at %s", store.dbPath)
	}
}

// TestSessionCRUD tests Create, Read, Update operations for sessions
func TestSessionCRUD(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create a session
	session := &DeviceSession{
		ID:        uuid.New().String(),
		DeviceID:  "test-device-001",
		Type:      "manual",
		Name:      "Test Session",
		StartTime: time.Now().UnixMilli(),
		Status:    "active",
		Metadata:  map[string]any{"key": "value"},
	}

	err := store.CreateSession(session)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Read the session
	retrieved, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved session should not be nil")
	}

	if retrieved.ID != session.ID {
		t.Errorf("Session ID mismatch: got %s, want %s", retrieved.ID, session.ID)
	}

	if retrieved.Name != session.Name {
		t.Errorf("Session Name mismatch: got %s, want %s", retrieved.Name, session.Name)
	}

	if retrieved.Status != session.Status {
		t.Errorf("Session Status mismatch: got %s, want %s", retrieved.Status, session.Status)
	}

	// Update the session
	session.Status = "completed"
	session.EndTime = time.Now().UnixMilli()
	session.EventCount = 100

	err = store.UpdateSession(session)
	if err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	// Verify update
	updated, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to get updated session: %v", err)
	}

	if updated.Status != "completed" {
		t.Errorf("Updated status mismatch: got %s, want completed", updated.Status)
	}

	if updated.EventCount != 100 {
		t.Errorf("Updated event count mismatch: got %d, want 100", updated.EventCount)
	}
}

// TestSessionRename tests renaming a session
func TestSessionRename(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := &DeviceSession{
		ID:        uuid.New().String(),
		DeviceID:  "test-device-001",
		Type:      "manual",
		Name:      "Original Name",
		StartTime: time.Now().UnixMilli(),
		Status:    "active",
	}

	err := store.CreateSession(session)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Rename
	newName := "Renamed Session"
	err = store.RenameSession(session.ID, newName)
	if err != nil {
		t.Fatalf("Failed to rename session: %v", err)
	}

	// Verify
	renamed, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to get renamed session: %v", err)
	}

	if renamed.Name != newName {
		t.Errorf("Name mismatch: got %s, want %s", renamed.Name, newName)
	}
}

// TestListSessions tests listing sessions
func TestListSessions(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	deviceID := "test-device-001"

	// Create multiple sessions
	for i := 0; i < 5; i++ {
		session := &DeviceSession{
			ID:        uuid.New().String(),
			DeviceID:  deviceID,
			Type:      "manual",
			Name:      "Test Session",
			StartTime: time.Now().UnixMilli() + int64(i*1000),
			Status:    "active",
		}
		if err := store.CreateSession(session); err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
	}

	// List all sessions for device
	sessions, err := store.ListSessions(deviceID, 10)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(sessions) != 5 {
		t.Errorf("Expected 5 sessions, got %d", len(sessions))
	}

	// List with limit
	limited, err := store.ListSessions(deviceID, 3)
	if err != nil {
		t.Fatalf("Failed to list sessions with limit: %v", err)
	}

	if len(limited) != 3 {
		t.Errorf("Expected 3 sessions with limit, got %d", len(limited))
	}
}

// TestEventWriteAndQuery tests writing and querying events
func TestEventWriteAndQuery(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create a session first
	session := &DeviceSession{
		ID:        uuid.New().String(),
		DeviceID:  "test-device-001",
		Type:      "manual",
		Name:      "Test Session",
		StartTime: time.Now().UnixMilli(),
		Status:    "active",
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Write events
	for i := 0; i < 10; i++ {
		event := UnifiedEvent{
			ID:           uuid.New().String(),
			SessionID:    session.ID,
			DeviceID:     session.DeviceID,
			Timestamp:    time.Now().UnixMilli() + int64(i*100),
			RelativeTime: int64(i * 100),
			Source:       SourceLogcat,
			Category:     CategoryLog,
			Type:         "logcat",
			Level:        LevelInfo,
			Title:        "Test log message",
			Summary:      "Test summary",
			Data:         json.RawMessage(`{"key": "value"}`),
		}
		store.WriteEvent(event)
	}

	// Flush and wait
	store.Flush()
	time.Sleep(100 * time.Millisecond)

	// Query events
	query := EventQuery{
		SessionID: session.ID,
		Limit:     100,
	}

	result, err := store.QueryEvents(query)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if result.Total != 10 {
		t.Errorf("Expected 10 events, got %d", result.Total)
	}

	if len(result.Events) != 10 {
		t.Errorf("Expected 10 events in result, got %d", len(result.Events))
	}
}

// TestEventQueryWithFilters tests event queries with various filters
func TestEventQueryWithFilters(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := &DeviceSession{
		ID:        uuid.New().String(),
		DeviceID:  "test-device-001",
		Type:      "manual",
		Name:      "Test Session",
		StartTime: time.Now().UnixMilli(),
		Status:    "active",
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Write events with different levels
	levels := []EventLevel{LevelInfo, LevelInfo, LevelWarn, LevelError, LevelError}
	sources := []EventSource{SourceLogcat, SourceLogcat, SourceNetwork, SourceNetwork, SourceLogcat}

	for i, level := range levels {
		event := UnifiedEvent{
			ID:           uuid.New().String(),
			SessionID:    session.ID,
			DeviceID:     session.DeviceID,
			Timestamp:    time.Now().UnixMilli() + int64(i*100),
			RelativeTime: int64(i * 100),
			Source:       sources[i],
			Category:     CategoryLog,
			Type:         "test",
			Level:        level,
			Title:        "Test message",
		}
		if err := store.WriteEventDirect(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	// Query by level
	query := EventQuery{
		SessionID: session.ID,
		Levels:    []EventLevel{LevelError},
		Limit:     100,
	}

	result, err := store.QueryEvents(query)
	if err != nil {
		t.Fatalf("Failed to query by level: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("Expected 2 error events, got %d", result.Total)
	}

	// Query by source
	query = EventQuery{
		SessionID: session.ID,
		Sources:   []EventSource{SourceNetwork},
		Limit:     100,
	}

	result, err = store.QueryEvents(query)
	if err != nil {
		t.Fatalf("Failed to query by source: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("Expected 2 network events, got %d", result.Total)
	}
}

// TestDeleteSession tests deleting a session and its events
func TestDeleteSession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := &DeviceSession{
		ID:        uuid.New().String(),
		DeviceID:  "test-device-001",
		Type:      "manual",
		Name:      "Test Session",
		StartTime: time.Now().UnixMilli(),
		Status:    "active",
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Add some events
	for i := 0; i < 5; i++ {
		event := UnifiedEvent{
			ID:           uuid.New().String(),
			SessionID:    session.ID,
			DeviceID:     session.DeviceID,
			Timestamp:    time.Now().UnixMilli(),
			RelativeTime: int64(i * 100),
			Source:       SourceLogcat,
			Category:     CategoryLog,
			Type:         "logcat",
			Level:        LevelInfo,
			Title:        "Test",
		}
		if err := store.WriteEventDirect(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	// Delete session
	err := store.DeleteSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify session is gone
	deleted, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("Error getting deleted session: %v", err)
	}
	if deleted != nil {
		t.Error("Session should be nil after deletion")
	}

	// Verify events are gone (cascade delete)
	query := EventQuery{SessionID: session.ID, Limit: 100}
	result, err := store.QueryEvents(query)
	if err != nil {
		t.Fatalf("Error querying deleted session events: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("Expected 0 events after session deletion, got %d", result.Total)
	}
}

// TestTimeIndex tests time index operations
// Note: GetTimeIndex generates the time index by aggregating the events table on the fly
func TestTimeIndex(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create a session first (required for foreign key)
	session := &DeviceSession{
		ID:        uuid.New().String(),
		DeviceID:  "test-device-001",
		Type:      "manual",
		Name:      "Test Session",
		StartTime: time.Now().UnixMilli(),
		Status:    "active",
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	sessionID := session.ID

	// Write events at different seconds to generate time index
	// Second 0: 5 events (1 error)
	for i := 0; i < 5; i++ {
		level := LevelInfo
		if i == 0 {
			level = LevelError
		}
		event := UnifiedEvent{
			ID:           uuid.New().String(),
			SessionID:    sessionID,
			DeviceID:     session.DeviceID,
			Timestamp:    time.Now().UnixMilli(),
			RelativeTime: int64(i * 100), // 0-400ms -> second 0
			Source:       SourceLogcat,
			Category:     CategoryLog,
			Type:         "logcat",
			Level:        level,
			Title:        "Test",
		}
		if err := store.WriteEventDirect(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	// Second 1: 3 events (no error)
	for i := 0; i < 3; i++ {
		event := UnifiedEvent{
			ID:           uuid.New().String(),
			SessionID:    sessionID,
			DeviceID:     session.DeviceID,
			Timestamp:    time.Now().UnixMilli(),
			RelativeTime: int64(1000 + i*100), // 1000-1200ms -> second 1
			Source:       SourceLogcat,
			Category:     CategoryLog,
			Type:         "logcat",
			Level:        LevelInfo,
			Title:        "Test",
		}
		if err := store.WriteEventDirect(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	// Second 2: 2 events (1 error)
	for i := 0; i < 2; i++ {
		level := LevelInfo
		if i == 1 {
			level = LevelError
		}
		event := UnifiedEvent{
			ID:           uuid.New().String(),
			SessionID:    sessionID,
			DeviceID:     session.DeviceID,
			Timestamp:    time.Now().UnixMilli(),
			RelativeTime: int64(2000 + i*100), // 2000-2100ms -> second 2
			Source:       SourceLogcat,
			Category:     CategoryLog,
			Type:         "logcat",
			Level:        level,
			Title:        "Test",
		}
		if err := store.WriteEventDirect(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	// Get time index (generated from events table)
	retrieved, err := store.GetTimeIndex(sessionID)
	if err != nil {
		t.Fatalf("Failed to get time index: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 time index entries, got %d", len(retrieved))
	}

	// Verify content
	expectedCounts := map[int]int{0: 5, 1: 3, 2: 2}
	expectedErrors := map[int]bool{0: true, 1: false, 2: true}

	for _, e := range retrieved {
		if expected, ok := expectedCounts[e.Second]; ok {
			if e.EventCount != expected {
				t.Errorf("Second %d: expected event count %d, got %d", e.Second, expected, e.EventCount)
			}
		}
		if expected, ok := expectedErrors[e.Second]; ok {
			if e.HasError != expected {
				t.Errorf("Second %d: expected hasError=%v, got %v", e.Second, expected, e.HasError)
			}
		}
	}
}

// TestGetSessionStats tests session statistics
func TestGetSessionStats(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := &DeviceSession{
		ID:        uuid.New().String(),
		DeviceID:  "test-device-001",
		Type:      "manual",
		Name:      "Test Session",
		StartTime: time.Now().UnixMilli(),
		Status:    "active",
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Add events with different levels
	levels := []EventLevel{LevelInfo, LevelInfo, LevelWarn, LevelError}
	for i, level := range levels {
		event := UnifiedEvent{
			ID:           uuid.New().String(),
			SessionID:    session.ID,
			DeviceID:     session.DeviceID,
			Timestamp:    time.Now().UnixMilli(),
			RelativeTime: int64(i * 100),
			Source:       SourceLogcat,
			Category:     CategoryLog,
			Type:         "logcat",
			Level:        level,
			Title:        "Test",
		}
		if err := store.WriteEventDirect(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	stats, err := store.GetSessionStats(session.ID)
	if err != nil {
		t.Fatalf("Failed to get session stats: %v", err)
	}

	if stats == nil {
		t.Fatal("Stats should not be nil")
	}

	// GetSessionStats returns "totalEvents" (camelCase), not "total_events"
	totalEvents, ok := stats["totalEvents"].(int)
	if !ok {
		t.Fatalf("totalEvents should be int, got %T", stats["totalEvents"])
	}
	if totalEvents != 4 {
		t.Errorf("Expected 4 total events, got %d", totalEvents)
	}

	// Check error count
	errorCount, ok := stats["errorCount"].(int)
	if !ok {
		t.Fatalf("errorCount should be int, got %T", stats["errorCount"])
	}
	if errorCount != 1 {
		t.Errorf("Expected 1 error event, got %d", errorCount)
	}
}

// TestConcurrentWrites tests concurrent event writes
func TestConcurrentWrites(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := &DeviceSession{
		ID:        uuid.New().String(),
		DeviceID:  "test-device-001",
		Type:      "manual",
		Name:      "Test Session",
		StartTime: time.Now().UnixMilli(),
		Status:    "active",
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			for j := 0; j < 100; j++ {
				event := UnifiedEvent{
					ID:           uuid.New().String(),
					SessionID:    session.ID,
					DeviceID:     session.DeviceID,
					Timestamp:    time.Now().UnixMilli(),
					RelativeTime: int64(idx*100 + j),
					Source:       SourceLogcat,
					Category:     CategoryLog,
					Type:         "logcat",
					Level:        LevelInfo,
					Title:        "Concurrent test",
				}
				store.WriteEvent(event)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Flush and verify
	store.Flush()
	time.Sleep(200 * time.Millisecond)

	query := EventQuery{SessionID: session.ID, Limit: 2000}
	result, err := store.QueryEvents(query)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if result.Total != 1000 {
		t.Errorf("Expected 1000 events from concurrent writes, got %d", result.Total)
	}
}

// TestDataDirectoryCreation tests that data directory is created if it doesn't exist
func TestDataDirectoryCreation(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "event_store_test_nonexistent_"+uuid.New().String())
	defer os.RemoveAll(tmpDir)

	store, err := NewEventStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create EventStore with new directory: %v", err)
	}
	defer store.Close()

	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Fatal("Data directory should be created")
	}
}

// TestDeepSearchCompressedData verifies that SearchText matches inside
// event_data that was gzip-compressed at write time (>=1KB payloads).
func TestDeepSearchCompressedData(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := &DeviceSession{
		ID:        uuid.New().String(),
		DeviceID:  "test-device-search",
		Type:      "manual",
		Name:      "Search Session",
		StartTime: time.Now().UnixMilli(),
		Status:    "active",
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Large payload (>1KB) triggers gzip compression in writeBatch
	filler := make([]byte, 2048)
	for i := range filler {
		filler[i] = 'a' + byte(i%26)
	}
	largeData, _ := json.Marshal(map[string]string{
		"marker": "unique-needle-in-compressed-haystack",
		"filler": string(filler),
	})
	// Small payload (<1KB) stays uncompressed
	smallData, _ := json.Marshal(map[string]string{"marker": "small-payload-needle"})

	events := []UnifiedEvent{
		{
			ID: uuid.New().String(), SessionID: session.ID, DeviceID: session.DeviceID,
			Timestamp: time.Now().UnixMilli(), Source: SourceNetwork, Category: CategoryNetwork,
			Type: "network_request", Level: LevelInfo, Title: "POST /api/big",
			Data: json.RawMessage(largeData),
		},
		{
			ID: uuid.New().String(), SessionID: session.ID, DeviceID: session.DeviceID,
			Timestamp: time.Now().UnixMilli(), Source: SourceNetwork, Category: CategoryNetwork,
			Type: "network_request", Level: LevelInfo, Title: "GET /api/small",
			Data: json.RawMessage(smallData),
		},
	}
	for _, e := range events {
		store.WriteEvent(e)
	}
	store.Flush()
	time.Sleep(200 * time.Millisecond)

	// Search inside the compressed body
	result, err := store.QueryEvents(EventQuery{SessionID: session.ID, SearchText: "unique-needle-in-compressed-haystack", Limit: 10})
	if err != nil {
		t.Fatalf("Search in compressed data failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Expected 1 match in compressed data, got %d", result.Total)
	}

	// Search inside the uncompressed body still works
	result, err = store.QueryEvents(EventQuery{SessionID: session.ID, SearchText: "small-payload-needle", Limit: 10})
	if err != nil {
		t.Fatalf("Search in uncompressed data failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Expected 1 match in uncompressed data, got %d", result.Total)
	}
}

// TestSearchSpecialCharacters verifies FTS5 syntax characters in user input
// do not break the search query (they are escaped to literal phrases).
func TestSearchSpecialCharacters(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := &DeviceSession{
		ID:        uuid.New().String(),
		DeviceID:  "test-device-fts",
		Type:      "manual",
		Name:      "FTS Session",
		StartTime: time.Now().UnixMilli(),
		Status:    "active",
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	store.WriteEvent(UnifiedEvent{
		ID: uuid.New().String(), SessionID: session.ID, DeviceID: session.DeviceID,
		Timestamp: time.Now().UnixMilli(), Source: SourceNetwork, Category: CategoryNetwork,
		Type: "network_request", Level: LevelInfo,
		Title: "GET http://example.com/api/users",
	})
	store.Flush()
	time.Sleep(200 * time.Millisecond)

	// All of these previously raised FTS5 syntax errors
	for _, input := range []string{
		"http://example.com",
		"tag: message",
		`"unbalanced`,
		"AND",
		"NEAR(",
		"*",
		"   ",
	} {
		if _, err := store.QueryEvents(EventQuery{SessionID: session.ID, SearchText: input, Limit: 10}); err != nil {
			t.Errorf("Search with input %q failed: %v", input, err)
		}
	}

	// URL search should actually match the stored title
	result, err := store.QueryEvents(EventQuery{SessionID: session.ID, SearchText: "http://example.com", Limit: 10})
	if err != nil {
		t.Fatalf("URL search failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Expected URL search to match 1 event, got %d", result.Total)
	}
}

// TestQueryEventsFullLoadTotal verifies that limit==0 full loads skip the
// standalone COUNT query and report Total as the number of returned rows.
func TestQueryEventsFullLoadTotal(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := &DeviceSession{
		ID:        uuid.New().String(),
		DeviceID:  "test-device-fullload",
		Type:      "manual",
		Name:      "Full Load Session",
		StartTime: time.Now().UnixMilli(),
		Status:    "active",
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	for i := 0; i < 25; i++ {
		title := "regular event"
		if i%5 == 0 {
			title = "needle event"
		}
		store.WriteEvent(UnifiedEvent{
			ID: uuid.New().String(), SessionID: session.ID, DeviceID: session.DeviceID,
			Timestamp: time.Now().UnixMilli(), RelativeTime: int64(i * 100),
			Source: SourceLogcat, Category: CategoryLog, Type: "logcat",
			Level: LevelInfo, Title: title,
		})
	}
	store.Flush()
	time.Sleep(200 * time.Millisecond)

	// limit==0 without search: Total equals returned rows, no pagination
	result, err := store.QueryEvents(EventQuery{SessionID: session.ID})
	if err != nil {
		t.Fatalf("Full load query failed: %v", err)
	}
	if len(result.Events) != 25 {
		t.Fatalf("Expected 25 events, got %d", len(result.Events))
	}
	if result.Total != 25 {
		t.Errorf("Expected Total 25, got %d", result.Total)
	}
	if result.HasMore {
		t.Error("Expected HasMore=false for full load")
	}

	// limit==0 with search: Total equals matched rows
	result, err = store.QueryEvents(EventQuery{SessionID: session.ID, SearchText: "needle"})
	if err != nil {
		t.Fatalf("Full load search query failed: %v", err)
	}
	if result.Total != len(result.Events) {
		t.Errorf("Expected Total == len(Events), got Total=%d len=%d", result.Total, len(result.Events))
	}
	if result.Total != 5 {
		t.Errorf("Expected 5 matches, got %d", result.Total)
	}
	if result.HasMore {
		t.Error("Expected HasMore=false for full load search")
	}

	// Paged query keeps the real COUNT-based Total semantics
	result, err = store.QueryEvents(EventQuery{SessionID: session.ID, Limit: 10})
	if err != nil {
		t.Fatalf("Paged query failed: %v", err)
	}
	if result.Total != 25 {
		t.Errorf("Expected paged Total 25, got %d", result.Total)
	}
	if !result.HasMore {
		t.Error("Expected HasMore=true for paged query")
	}
}

// TestCompressDataRoundTrip verifies gzip compression survives pooled-writer
// reuse, including concurrent callers sharing the sync.Pool.
func TestCompressDataRoundTrip(t *testing.T) {
	// Small payloads (<1KB) bypass compression
	small := []byte("tiny payload")
	got, err := compressData(small)
	if err != nil {
		t.Fatalf("compressData failed on small payload: %v", err)
	}
	if !bytes.Equal(got, small) {
		t.Error("Expected small payload to pass through uncompressed")
	}

	// Large compressible payload round-trips
	large := bytes.Repeat([]byte("abcdefgh"), 1024) // 8KB
	compressed, err := compressData(large)
	if err != nil {
		t.Fatalf("compressData failed: %v", err)
	}
	if len(compressed) >= len(large) {
		t.Error("Expected compressible payload to shrink")
	}
	restored, err := decompressData(compressed)
	if err != nil {
		t.Fatalf("decompressData failed: %v", err)
	}
	if !bytes.Equal(restored, large) {
		t.Error("Round-trip mismatch for compressed payload")
	}

	// Concurrent compression exercises gzip writer pool reuse
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(seed byte) {
			defer wg.Done()
			payload := bytes.Repeat([]byte{seed, seed + 1, seed + 2, seed + 3}, 1024)
			for j := 0; j < 50; j++ {
				c, err := compressData(payload)
				if err != nil {
					t.Errorf("Concurrent compressData failed: %v", err)
					return
				}
				d, err := decompressData(c)
				if err != nil {
					t.Errorf("Concurrent decompressData failed: %v", err)
					return
				}
				if !bytes.Equal(d, payload) {
					t.Error("Concurrent round-trip mismatch")
					return
				}
			}
		}(byte('a' + i))
	}
	wg.Wait()
}
