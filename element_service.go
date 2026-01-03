package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ========================================
// Unified Element Service
// Consolidates all element operations
// Used by: Recording playback, Workflow execution, UI Inspector actions
// ========================================

// ElementActionConfig contains configuration for element operations
type ElementActionConfig struct {
	Timeout       int    // Max time to wait for element in ms (default: 10000)
	RetryInterval int    // Retry interval in ms (default: 1000)
	PreWait       int    // Wait before action in ms (default: 0)
	PostDelay     int    // Wait after action in ms (default: 0)
	OnError       string // "stop" or "continue" (default: "stop")
}

// DefaultElementActionConfig returns default configuration
func DefaultElementActionConfig() ElementActionConfig {
	return ElementActionConfig{
		Timeout:       10000,
		RetryInterval: 1000,
		PreWait:       0,
		PostDelay:     0,
		OnError:       "stop",
	}
}

// ========================================
// Core Element Operations
// ========================================

// ClickElement finds and clicks an element
func (a *App) ClickElement(ctx context.Context, deviceId string, selector *ElementSelector, config *ElementActionConfig) error {
	if config == nil {
		cfg := DefaultElementActionConfig()
		config = &cfg
	}

	node, err := a.waitForElement(ctx, deviceId, selector, config.Timeout, config.RetryInterval)
	if err != nil {
		return err
	}

	bounds, err := ParseBounds(node.Bounds)
	if err != nil {
		return fmt.Errorf("invalid bounds: %s", node.Bounds)
	}

	x, y := bounds.Center()
	_, err = a.RunAdbCommand(deviceId, fmt.Sprintf("shell input tap %d %d", x, y))
	return err
}

// LongClickElement finds and long-clicks an element
func (a *App) LongClickElement(ctx context.Context, deviceId string, selector *ElementSelector, duration int, config *ElementActionConfig) error {
	if config == nil {
		cfg := DefaultElementActionConfig()
		config = &cfg
	}
	if duration <= 0 {
		duration = 1000
	}

	node, err := a.waitForElement(ctx, deviceId, selector, config.Timeout, config.RetryInterval)
	if err != nil {
		return err
	}

	bounds, err := ParseBounds(node.Bounds)
	if err != nil {
		return fmt.Errorf("invalid bounds: %s", node.Bounds)
	}

	x, y := bounds.Center()
	_, err = a.RunAdbCommand(deviceId, fmt.Sprintf("shell input swipe %d %d %d %d %d", x, y, x, y, duration))
	return err
}

// InputTextToElement finds an element, taps it, and inputs text
func (a *App) InputTextToElement(ctx context.Context, deviceId string, selector *ElementSelector, text string, clearFirst bool, config *ElementActionConfig) error {
	if config == nil {
		cfg := DefaultElementActionConfig()
		config = &cfg
	}

	node, err := a.waitForElement(ctx, deviceId, selector, config.Timeout, config.RetryInterval)
	if err != nil {
		return err
	}

	bounds, err := ParseBounds(node.Bounds)
	if err != nil {
		return fmt.Errorf("invalid bounds: %s", node.Bounds)
	}

	x, y := bounds.Center()

	// Tap to focus
	_, err = a.RunAdbCommand(deviceId, fmt.Sprintf("shell input tap %d %d", x, y))
	if err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)

	// Clear existing text if requested
	if clearFirst {
		// Select all and delete
		a.RunAdbCommand(deviceId, "shell input keyevent 67") // Delete key, multiple times
	}

	// Escape special characters for shell
	escapedText := strings.ReplaceAll(text, " ", "%s")
	escapedText = strings.ReplaceAll(escapedText, "'", "\\'")

	_, err = a.RunAdbCommand(deviceId, fmt.Sprintf("shell input text '%s'", escapedText))
	return err
}

// SwipeOnElement finds an element and performs a swipe from its center
func (a *App) SwipeOnElement(ctx context.Context, deviceId string, selector *ElementSelector, direction string, distance int, duration int, config *ElementActionConfig) error {
	if config == nil {
		cfg := DefaultElementActionConfig()
		config = &cfg
	}
	if distance <= 0 {
		distance = 500
	}
	if duration <= 0 {
		duration = 500
	}

	node, err := a.waitForElement(ctx, deviceId, selector, config.Timeout, config.RetryInterval)
	if err != nil {
		return err
	}

	bounds, err := ParseBounds(node.Bounds)
	if err != nil {
		return fmt.Errorf("invalid bounds: %s", node.Bounds)
	}

	x, y := bounds.Center()
	x2, y2 := x, y

	switch strings.ToLower(direction) {
	case "up":
		y2 = y - distance
	case "down":
		y2 = y + distance
	case "left":
		x2 = x - distance
	case "right":
		x2 = x + distance
	default:
		return fmt.Errorf("invalid swipe direction: %s", direction)
	}

	_, err = a.RunAdbCommand(deviceId, fmt.Sprintf("shell input swipe %d %d %d %d %d", x, y, x2, y2, duration))
	return err
}

// ========================================
// Wait Operations
// ========================================

// WaitForElement waits for an element to appear
func (a *App) WaitForElement(ctx context.Context, deviceId string, selector *ElementSelector, timeout int) error {
	if timeout <= 0 {
		timeout = 10000
	}
	_, err := a.waitForElement(ctx, deviceId, selector, timeout, 1000)
	return err
}

// WaitElementGone waits for an element to disappear
func (a *App) WaitElementGone(ctx context.Context, deviceId string, selector *ElementSelector, timeout int) error {
	if timeout <= 0 {
		timeout = 10000
	}

	startTime := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if time.Since(startTime) > time.Duration(timeout)*time.Millisecond {
			return fmt.Errorf("timeout waiting for element to disappear")
		}

		hierarchy, err := a.GetUIHierarchy(deviceId)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		node := a.FindElementBySelector(hierarchy.Root, selector)
		if node == nil {
			return nil // Element is gone
		}

		time.Sleep(1 * time.Second)
	}
}

// ========================================
// Assert Operations
// ========================================

// AssertElementExists checks if an element exists
func (a *App) AssertElementExists(deviceId string, selector *ElementSelector) (bool, error) {
	hierarchy, err := a.GetUIHierarchy(deviceId)
	if err != nil {
		return false, err
	}

	node := a.FindElementBySelector(hierarchy.Root, selector)
	return node != nil, nil
}

// AssertElementText checks if an element's text matches
func (a *App) AssertElementText(deviceId string, selector *ElementSelector, expectedText string, contains bool) (bool, error) {
	hierarchy, err := a.GetUIHierarchy(deviceId)
	if err != nil {
		return false, err
	}

	node := a.FindElementBySelector(hierarchy.Root, selector)
	if node == nil {
		return false, nil
	}

	if contains {
		return strings.Contains(node.Text, expectedText) || strings.Contains(node.ContentDesc, expectedText), nil
	}
	return node.Text == expectedText, nil
}

// ========================================
// Element Properties
// ========================================

// GetElementProperties returns all properties of an element
func (a *App) GetElementProperties(deviceId string, selector *ElementSelector) (map[string]interface{}, error) {
	hierarchy, err := a.GetUIHierarchy(deviceId)
	if err != nil {
		return nil, err
	}

	node := a.FindElementBySelector(hierarchy.Root, selector)
	if node == nil {
		return nil, fmt.Errorf("element not found")
	}

	props := map[string]interface{}{
		"class":       node.Class,
		"text":        node.Text,
		"resourceId":  node.ResourceID,
		"contentDesc": node.ContentDesc,
		"bounds":      node.Bounds,
		"clickable":   node.Clickable,
		"checkable":   node.Checkable,
		"checked":     node.Checked,
		"enabled":     node.Enabled,
		"focusable":   node.Focusable,
		"focused":     node.Focused,
		"scrollable":  node.Scrollable,
		"selected":    node.Selected,
	}

	// Add center coordinates
	if bounds, err := ParseBounds(node.Bounds); err == nil {
		x, y := bounds.Center()
		props["centerX"] = x
		props["centerY"] = y
	}

	return props, nil
}

// ========================================
// Scroll Operations
// ========================================

// ScrollToElement scrolls until the element is found
func (a *App) ScrollToElement(ctx context.Context, deviceId string, selector *ElementSelector, direction string, maxScrolls int) error {
	if maxScrolls <= 0 {
		maxScrolls = 10
	}

	for i := 0; i < maxScrolls; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if element exists
		hierarchy, err := a.GetUIHierarchy(deviceId)
		if err != nil {
			continue
		}

		node := a.FindElementBySelector(hierarchy.Root, selector)
		if node != nil {
			return nil // Found
		}

		// Perform scroll
		err = a.performScroll(deviceId, direction)
		if err != nil {
			return err
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("element not found after %d scrolls", maxScrolls)
}

func (a *App) performScroll(deviceId string, direction string) error {
	// Get screen size for scroll coordinates
	resolution, _ := a.GetDeviceResolution(deviceId)
	width, height := 1080, 1920
	if parts := strings.Split(resolution, "x"); len(parts) == 2 {
		fmt.Sscanf(parts[0], "%d", &width)
		fmt.Sscanf(parts[1], "%d", &height)
	}

	centerX := width / 2
	centerY := height / 2
	scrollDist := height / 3

	var x1, y1, x2, y2 int
	switch strings.ToLower(direction) {
	case "up":
		x1, y1 = centerX, centerY+scrollDist/2
		x2, y2 = centerX, centerY-scrollDist/2
	case "down":
		x1, y1 = centerX, centerY-scrollDist/2
		x2, y2 = centerX, centerY+scrollDist/2
	case "left":
		x1, y1 = centerX+scrollDist/2, centerY
		x2, y2 = centerX-scrollDist/2, centerY
	case "right":
		x1, y1 = centerX-scrollDist/2, centerY
		x2, y2 = centerX+scrollDist/2, centerY
	default:
		return fmt.Errorf("invalid scroll direction: %s", direction)
	}

	_, err := a.RunAdbCommand(deviceId, fmt.Sprintf("shell input swipe %d %d %d %d 300", x1, y1, x2, y2))
	return err
}

// ========================================
// Internal Helpers
// ========================================

// waitForElement waits for an element to appear and returns it
func (a *App) waitForElement(ctx context.Context, deviceId string, selector *ElementSelector, timeout int, retryInterval int) (*UINode, error) {
	if selector == nil {
		return nil, fmt.Errorf("selector is nil")
	}

	// Handle bounds selector directly (no need to wait/search)
	if selector.Type == "bounds" {
		return &UINode{Bounds: selector.Value}, nil
	}

	startTime := time.Now()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if time.Since(startTime) > time.Duration(timeout)*time.Millisecond {
			return nil, fmt.Errorf("element not found within timeout (selector: %s=%s)", selector.Type, selector.Value)
		}

		hierarchy, err := a.GetUIHierarchy(deviceId)
		if err != nil {
			time.Sleep(time.Duration(retryInterval) * time.Millisecond)
			continue
		}

		node := a.FindElementBySelector(hierarchy.Root, selector)
		if node != nil {
			return node, nil
		}

		time.Sleep(time.Duration(retryInterval) * time.Millisecond)
	}
}

// ========================================
// Coordinate-based Operations (for recording playback)
// ========================================

// TapAtCoordinates performs a tap at specific coordinates
func (a *App) TapAtCoordinates(deviceId string, x, y int) error {
	_, err := a.RunAdbCommand(deviceId, fmt.Sprintf("shell input tap %d %d", x, y))
	return err
}

// LongPressAtCoordinates performs a long press at specific coordinates
func (a *App) LongPressAtCoordinates(deviceId string, x, y int, duration int) error {
	if duration <= 0 {
		duration = 1000
	}
	_, err := a.RunAdbCommand(deviceId, fmt.Sprintf("shell input swipe %d %d %d %d %d", x, y, x, y, duration))
	return err
}

// SwipeCoordinates performs a swipe between two points
func (a *App) SwipeCoordinates(deviceId string, x1, y1, x2, y2 int, duration int) error {
	if duration <= 0 {
		duration = 300
	}
	_, err := a.RunAdbCommand(deviceId, fmt.Sprintf("shell input swipe %d %d %d %d %d", x1, y1, x2, y2, duration))
	return err
}
