package main

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ========================================
// RingBuffer Tests
// ========================================

func TestRingBufferCreation(t *testing.T) {
	rb := NewRingBuffer(10)

	if rb == nil {
		t.Fatal("RingBuffer should not be nil")
	}

	if rb.size != 10 {
		t.Errorf("Expected size 10, got %d", rb.size)
	}

	if rb.Size() != 0 {
		t.Errorf("Expected initial count 0, got %d", rb.Size())
	}
}

func TestRingBufferPush(t *testing.T) {
	rb := NewRingBuffer(5)

	// Push some events
	for i := 0; i < 3; i++ {
		event := UnifiedEvent{
			ID:    uuid.New().String(),
			Title: "Event",
		}
		rb.Push(event)
	}

	if rb.Size() != 3 {
		t.Errorf("Expected count 3, got %d", rb.Size())
	}
}

func TestRingBufferOverflow(t *testing.T) {
	rb := NewRingBuffer(3)

	// Push more than capacity
	ids := make([]string, 5)
	for i := 0; i < 5; i++ {
		ids[i] = uuid.New().String()
		rb.Push(UnifiedEvent{ID: ids[i], Title: "Event"})
	}

	// Size should be at capacity
	if rb.Size() != 3 {
		t.Errorf("Expected count 3 (capacity), got %d", rb.Size())
	}

	// Get recent should return last 3
	recent := rb.GetRecent(3)
	if len(recent) != 3 {
		t.Errorf("Expected 3 recent events, got %d", len(recent))
	}

	// Verify oldest events were overwritten (should have ids[2], ids[3], ids[4])
	expectedIDs := []string{ids[2], ids[3], ids[4]}
	for i, event := range recent {
		if event.ID != expectedIDs[i] {
			t.Errorf("Event %d: expected ID %s, got %s", i, expectedIDs[i], event.ID)
		}
	}
}

func TestRingBufferGetRecent(t *testing.T) {
	rb := NewRingBuffer(10)

	// Push 5 events
	for i := 0; i < 5; i++ {
		rb.Push(UnifiedEvent{ID: uuid.New().String()})
	}

	// Get more than available
	recent := rb.GetRecent(10)
	if len(recent) != 5 {
		t.Errorf("Expected 5 events when requesting more than available, got %d", len(recent))
	}

	// Get less than available
	recent = rb.GetRecent(3)
	if len(recent) != 3 {
		t.Errorf("Expected 3 events, got %d", len(recent))
	}

	// Get 0
	recent = rb.GetRecent(0)
	if len(recent) != 0 {
		t.Errorf("Expected 0 events, got %d", len(recent))
	}
}

func TestRingBufferClear(t *testing.T) {
	rb := NewRingBuffer(10)

	for i := 0; i < 5; i++ {
		rb.Push(UnifiedEvent{ID: uuid.New().String()})
	}

	rb.Clear()

	if rb.Size() != 0 {
		t.Errorf("Expected count 0 after clear, got %d", rb.Size())
	}

	recent := rb.GetRecent(10)
	if len(recent) != 0 {
		t.Errorf("Expected 0 events after clear, got %d", len(recent))
	}
}

func TestRingBufferConcurrency(t *testing.T) {
	rb := NewRingBuffer(100)
	var wg sync.WaitGroup

	// Concurrent pushes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				rb.Push(UnifiedEvent{ID: uuid.New().String()})
			}
		}()
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rb.GetRecent(10)
				rb.Size()
			}
		}()
	}

	wg.Wait()

	// Should be at capacity
	if rb.Size() != 100 {
		t.Errorf("Expected size 100 after concurrent ops, got %d", rb.Size())
	}
}

// ========================================
// BackpressureController Tests
// ========================================

func TestBackpressureControllerCreation(t *testing.T) {
	bp := NewBackpressureController(1000)

	if bp == nil {
		t.Fatal("BackpressureController should not be nil")
	}

	if bp.maxEventsPerSecond != 1000 {
		t.Errorf("Expected max 1000, got %d", bp.maxEventsPerSecond)
	}
}

func TestBackpressureCriticalEventsAlwaysPass(t *testing.T) {
	bp := NewBackpressureController(1) // Very low limit

	// Send many events to trigger backpressure
	for i := 0; i < 100; i++ {
		bp.ShouldProcess(UnifiedEvent{
			Level:  LevelVerbose,
			Source: SourceLogcat,
		})
	}

	// Critical events should still pass
	criticalEvents := []UnifiedEvent{
		{Level: LevelError, Source: SourceLogcat},
		{Level: LevelFatal, Source: SourceLogcat},
		{Source: SourceNetwork, Level: LevelInfo},
		{Type: "app_crash", Level: LevelInfo},
		{Type: "app_anr", Level: LevelInfo},
		{Source: SourceWorkflow, Level: LevelInfo},
		{Source: SourceAssertion, Level: LevelInfo},
		{Type: "session_start", Level: LevelInfo},
		{Type: "session_end", Level: LevelInfo},
	}

	for i, event := range criticalEvents {
		if !bp.ShouldProcess(event) {
			t.Errorf("Critical event %d should always pass, but was dropped", i)
		}
	}
}

func TestBackpressureNormalConditions(t *testing.T) {
	bp := NewBackpressureController(1000)

	// Under limit, all events should pass
	for i := 0; i < 100; i++ {
		event := UnifiedEvent{
			Level:  LevelVerbose,
			Source: SourceLogcat,
		}
		if !bp.ShouldProcess(event) {
			t.Errorf("Event %d should pass under normal conditions", i)
		}
	}
}

func TestBackpressureVerboseDropping(t *testing.T) {
	bp := NewBackpressureController(10)

	// Exceed limit significantly (>5x)
	for i := 0; i < 60; i++ {
		bp.ShouldProcess(UnifiedEvent{
			Level:  LevelInfo,
			Source: SourceLogcat,
		})
	}

	// Verbose events should be dropped
	droppedCount := 0
	for i := 0; i < 10; i++ {
		if !bp.ShouldProcess(UnifiedEvent{
			Level:  LevelVerbose,
			Source: SourceLogcat,
		}) {
			droppedCount++
		}
	}

	if droppedCount == 0 {
		t.Error("Some verbose events should be dropped under heavy backpressure")
	}
}

func TestBackpressureWindowReset(t *testing.T) {
	bp := NewBackpressureController(10)

	// Fill up the window
	for i := 0; i < 100; i++ {
		bp.ShouldProcess(UnifiedEvent{Level: LevelVerbose})
	}

	// Wait for window to reset
	time.Sleep(1100 * time.Millisecond)

	// Events should pass again
	passCount := 0
	for i := 0; i < 5; i++ {
		if bp.ShouldProcess(UnifiedEvent{Level: LevelVerbose}) {
			passCount++
		}
	}

	if passCount != 5 {
		t.Errorf("Expected all 5 events to pass after window reset, got %d", passCount)
	}
}

func TestBackpressureStats(t *testing.T) {
	bp := NewBackpressureController(5)

	// Generate some drops
	for i := 0; i < 50; i++ {
		bp.ShouldProcess(UnifiedEvent{Level: LevelVerbose})
	}

	stats := bp.GetStats()

	if stats == nil {
		t.Fatal("Stats should not be nil")
	}

	// Check that stats keys exist
	if _, ok := stats["dropped"]; !ok {
		t.Error("Stats should contain 'dropped' key")
	}
	if _, ok := stats["sampled"]; !ok {
		t.Error("Stats should contain 'sampled' key")
	}
	if _, ok := stats["aggregated"]; !ok {
		t.Error("Stats should contain 'aggregated' key")
	}
}

// ========================================
// EventSampler Tests
// ========================================

func TestEventSampler(t *testing.T) {
	sampler := &EventSampler{rate: 5}

	keptCount := 0
	for i := 0; i < 20; i++ {
		if sampler.ShouldKeep() {
			keptCount++
		}
	}

	// With rate 5, every 5th event is kept: events 5, 10, 15, 20 = 4 events
	if keptCount != 4 {
		t.Errorf("Expected 4 events kept with rate 5, got %d", keptCount)
	}
}

func TestEventSamplerRateOne(t *testing.T) {
	sampler := &EventSampler{rate: 1}

	keptCount := 0
	for i := 0; i < 10; i++ {
		if sampler.ShouldKeep() {
			keptCount++
		}
	}

	// Rate 1 means keep all
	if keptCount != 10 {
		t.Errorf("Expected all 10 events kept with rate 1, got %d", keptCount)
	}
}

// ========================================
// TimeIndexEntry Tests
// ========================================

func TestTimeIndexEntry(t *testing.T) {
	entry := &TimeIndexEntry{
		Second:       5,
		EventCount:   10,
		FirstEventID: "evt-123",
		HasError:     false,
	}

	if entry.Second != 5 {
		t.Errorf("Expected second 5, got %d", entry.Second)
	}

	if entry.EventCount != 10 {
		t.Errorf("Expected event count 10, got %d", entry.EventCount)
	}

	if entry.FirstEventID != "evt-123" {
		t.Errorf("Expected first event ID evt-123, got %s", entry.FirstEventID)
	}

	if entry.HasError {
		t.Error("Expected no error")
	}
}

// ========================================
// SessionState Tests
// ========================================

func TestSessionState(t *testing.T) {
	session := &DeviceSession{
		ID:        uuid.New().String(),
		DeviceID:  "test-device",
		Name:      "Test Session",
		StartTime: time.Now().UnixMilli(),
		Status:    "active",
	}

	state := &SessionState{
		Session:      session,
		StartTime:    session.StartTime,
		EventCount:   0,
		RecentEvents: NewRingBuffer(100),
	}

	if state.Session.ID != session.ID {
		t.Errorf("Session ID mismatch")
	}

	// Add events
	for i := 0; i < 10; i++ {
		event := UnifiedEvent{ID: uuid.New().String()}
		state.RecentEvents.Push(event)
		state.EventCount++
	}

	if state.EventCount != 10 {
		t.Errorf("Expected event count 10, got %d", state.EventCount)
	}

	if state.RecentEvents.Size() != 10 {
		t.Errorf("Expected 10 recent events, got %d", state.RecentEvents.Size())
	}
}

// ========================================
// GetCategoryForType Tests (helper function)
// ========================================

func TestGetCategoryForType(t *testing.T) {
	// Test known types from EventRegistry
	testCases := []struct {
		eventType string
		expected  EventCategory
	}{
		{"logcat", CategoryLog},
		{"http_request", CategoryNetwork},
		{"activity_start", CategoryState},      // App lifecycle events
		{"workflow_start", CategoryAutomation}, // Workflow events
		{"unknown_type", CategoryDiagnostic},   // Default for unknown types
	}

	for _, tc := range testCases {
		got := GetCategoryForType(tc.eventType)
		if got != tc.expected {
			t.Errorf("GetCategoryForType(%s): expected %s, got %s", tc.eventType, tc.expected, got)
		}
	}
}

// ========================================
// isCriticalEvent Tests
// ========================================

func TestIsCriticalEvent(t *testing.T) {
	bp := NewBackpressureController(1000)

	criticalCases := []struct {
		name   string
		event  UnifiedEvent
		expect bool
	}{
		{"Error level", UnifiedEvent{Level: LevelError}, true},
		{"Fatal level", UnifiedEvent{Level: LevelFatal}, true},
		{"Network source", UnifiedEvent{Source: SourceNetwork, Level: LevelInfo}, true},
		{"App crash", UnifiedEvent{Type: "app_crash", Level: LevelInfo}, true},
		{"App ANR", UnifiedEvent{Type: "app_anr", Level: LevelInfo}, true},
		{"Workflow source", UnifiedEvent{Source: SourceWorkflow, Level: LevelInfo}, true},
		{"Assertion source", UnifiedEvent{Source: SourceAssertion, Level: LevelInfo}, true},
		{"Session start", UnifiedEvent{Type: "session_start", Level: LevelInfo}, true},
		{"Session end", UnifiedEvent{Type: "session_end", Level: LevelInfo}, true},
		{"Regular info", UnifiedEvent{Source: SourceLogcat, Level: LevelInfo, Type: "logcat"}, false},
		{"Regular debug", UnifiedEvent{Source: SourceLogcat, Level: LevelDebug, Type: "logcat"}, false},
		{"Regular verbose", UnifiedEvent{Source: SourceLogcat, Level: LevelVerbose, Type: "logcat"}, false},
	}

	for _, tc := range criticalCases {
		got := bp.isCriticalEvent(tc.event)
		if got != tc.expect {
			t.Errorf("%s: expected critical=%v, got %v", tc.name, tc.expect, got)
		}
	}
}
