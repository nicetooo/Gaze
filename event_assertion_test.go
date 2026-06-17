package main

import (
	"testing"
)

// ========================================
// evaluateSequence Tests (AW-7)
// Ordered=true 要求严格顺序; Ordered=false 只要求全部出现
// ========================================

func makeSequenceAssertion(ordered bool) *Assertion {
	return &Assertion{
		ID:   "seq-test",
		Name: "sequence test",
		Type: AssertSequence,
		Expected: AssertionExpected{
			Ordered: ordered,
			Sequence: []EventCriteria{
				{Types: []string{"app_start"}},
				{Types: []string{"network_request"}},
				{Types: []string{"app_stop"}},
			},
		},
	}
}

// 乱序事件集: network_request 出现在 app_start 之前
func makeOutOfOrderEvents() []UnifiedEvent {
	return []UnifiedEvent{
		{ID: "e3", Type: "app_stop", Timestamp: 300},
		{ID: "e1", Type: "network_request", Timestamp: 100},
		{ID: "e2", Type: "app_start", Timestamp: 200},
	}
}

func TestEvaluateSequenceUnorderedPassesOutOfOrder(t *testing.T) {
	e := &AssertionEngine{}
	assertion := makeSequenceAssertion(false)
	result := &AssertionResult{}

	e.evaluateSequence(assertion, makeOutOfOrderEvents(), result)

	if !result.Passed {
		t.Fatalf("Ordered=false should pass with out-of-order events, got: %s", result.Message)
	}
	if len(result.MatchedEvents) != 3 {
		t.Fatalf("Expected 3 matched events, got %d", len(result.MatchedEvents))
	}
}

func TestEvaluateSequenceOrderedFailsOutOfOrder(t *testing.T) {
	e := &AssertionEngine{}
	assertion := makeSequenceAssertion(true)
	result := &AssertionResult{}

	e.evaluateSequence(assertion, makeOutOfOrderEvents(), result)

	if result.Passed {
		t.Fatal("Ordered=true should fail when events arrive out of sequence order")
	}
}

func TestEvaluateSequenceOrderedPassesInOrder(t *testing.T) {
	e := &AssertionEngine{}
	assertion := makeSequenceAssertion(true)
	result := &AssertionResult{}

	events := []UnifiedEvent{
		{ID: "e1", Type: "app_start", Timestamp: 100},
		{ID: "e2", Type: "network_request", Timestamp: 200},
		{ID: "e3", Type: "app_stop", Timestamp: 300},
	}
	e.evaluateSequence(assertion, events, result)

	if !result.Passed {
		t.Fatalf("Ordered=true should pass with in-order events, got: %s", result.Message)
	}
}

func TestEvaluateSequenceUnorderedDeduplicatesEvents(t *testing.T) {
	// 同一事件不能同时满足多个 criteria
	e := &AssertionEngine{}
	assertion := &Assertion{
		ID:   "seq-dedup",
		Type: AssertSequence,
		Expected: AssertionExpected{
			Ordered: false,
			Sequence: []EventCriteria{
				{Types: []string{"app_start"}},
				{Types: []string{"app_start"}},
			},
		},
	}
	result := &AssertionResult{}

	// 只有一个 app_start 事件, 两个 criteria 不应共享它
	events := []UnifiedEvent{
		{ID: "e1", Type: "app_start", Timestamp: 100},
	}
	e.evaluateSequence(assertion, events, result)

	if result.Passed {
		t.Fatal("Unordered match must not reuse the same event for multiple criteria")
	}

	// 两个 app_start 事件则应通过
	result = &AssertionResult{}
	events = append(events, UnifiedEvent{ID: "e2", Type: "app_start", Timestamp: 200})
	e.evaluateSequence(assertion, events, result)

	if !result.Passed {
		t.Fatalf("Expected pass with two matching events, got: %s", result.Message)
	}
}
