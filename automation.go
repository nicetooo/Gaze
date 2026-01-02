package main

import (
	"bufio"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Touch recording state management
var (
	touchRecordCmd    = make(map[string]*exec.Cmd)
	touchRecordCancel = make(map[string]context.CancelFunc)
	touchRecordData   = make(map[string]*TouchRecordingSession)
	touchRecordMu     sync.Mutex

	touchPlaybackCancel = make(map[string]context.CancelFunc)
	touchPlaybackMu     sync.Mutex
	// Pause control
	taskPauseSignal = make(map[string]chan struct{})
	taskIsPaused    = make(map[string]bool)
	taskPauseMu     sync.Mutex
)

// GetTouchInputDevice finds the touch input device path on the Android device
func (a *App) GetTouchInputDevice(deviceId string) (string, error) {
	// 1. Get all input devices and their properties in one go
	output, err := a.RunAdbCommand(deviceId, "shell getevent -p")
	if err != nil {
		return "", fmt.Errorf("failed to get input devices: %w", err)
	}

	// Clean up output
	output = strings.ReplaceAll(output, "\r\n", "\n")

	// Split by "add device" to handle multiple devices
	devices := strings.Split(output, "add device")

	touchKeywords := []string{
		"touch", "ts", "ft5", "goodix", "synaptics", "atmel",
		"elan", "himax", "focaltech", "mxt", "nvt", "ilitek",
		"sec_touchscreen", "input_mt", "mtk-tpd",
	}

	type Candidate struct {
		Path  string
		Score int
	}
	var candidates []Candidate

	for _, deviceBlock := range devices {
		if strings.TrimSpace(deviceBlock) == "" {
			continue
		}

		// Extract device path (e.g., "1: /dev/input/event4")
		firstLineEnd := strings.Index(deviceBlock, "\n")
		if firstLineEnd == -1 {
			continue
		}
		firstLine := deviceBlock[:firstLineEnd]

		pathIdx := strings.Index(firstLine, "/dev/input/")
		if pathIdx == -1 {
			continue
		}
		path := strings.TrimSpace(firstLine[pathIdx:])

		// Check for multi-touch properties (ABS_MT_POSITION_X / 0035)
		isMultitouch := strings.Contains(deviceBlock, "ABS_MT_POSITION_X") ||
			strings.Contains(deviceBlock, "0035")

		if !isMultitouch {
			continue
		}

		score := 1 // Base score for having multitouch

		// Check name for keywords
		nameMatch := false
		lines := strings.Split(deviceBlock, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if strings.Contains(line, "name:") {
				nameLower := strings.ToLower(line)
				for _, keyword := range touchKeywords {
					if strings.Contains(nameLower, keyword) {
						nameMatch = true
						break
					}
				}
				break // Found name line
			}
		}

		if nameMatch {
			score += 10
		}

		fmt.Printf("[Automation] Found candidate: %s (score=%d)\n", path, score)
		candidates = append(candidates, Candidate{Path: path, Score: score})
	}

	// Find best candidate
	var bestPath string
	var bestScore int = 0

	for _, c := range candidates {
		if c.Score > bestScore {
			bestScore = c.Score
			bestPath = c.Path
		}
	}

	if bestPath != "" {
		fmt.Printf("[Automation] Selected touch device: %s\n", bestPath)
		return bestPath, nil
	}

	return "", fmt.Errorf("no touch input device found")
}

// GetDeviceResolution gets the screen resolution of the device
func (a *App) GetDeviceResolution(deviceId string) (string, error) {
	output, err := a.RunAdbCommand(deviceId, "shell wm size")
	if err != nil {
		return "", err
	}

	// Parse "Physical size: 1080x2400" or "Override size: 1080x2400"
	re := regexp.MustCompile(`(\d+)x(\d+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 3 {
		return matches[1] + "x" + matches[2], nil
	}

	return "1080x1920", nil // Default fallback
}

// StartTouchRecording starts recording touch events from the device
func (a *App) StartTouchRecording(deviceId string) error {
	touchRecordMu.Lock()
	defer touchRecordMu.Unlock()

	// Check if already recording
	if _, exists := touchRecordCmd[deviceId]; exists {
		return fmt.Errorf("already recording on this device")
	}

	// Get touch input device
	inputDevice, err := a.GetTouchInputDevice(deviceId)
	if err != nil {
		return fmt.Errorf("failed to find touch input device: %w", err)
	}
	fmt.Printf("[Automation] Starting recording on device %s, touch input: %s\n", deviceId, inputDevice)

	// Get resolution for coordinate scaling later
	resolution, _ := a.GetDeviceResolution(deviceId)
	fmt.Printf("[Automation] Device resolution: %s\n", resolution)

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Start getevent command for specific device
	// Run getevent -lt /dev/input/eventX
	cmd := exec.CommandContext(ctx, a.adbPath, "-s", deviceId, "shell", "getevent", "-lt", inputDevice)

	// Create a pipe to read output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Also capture stderr
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start getevent: %w", err)
	}

	fmt.Printf("[Automation] getevent process started, PID: %d, listening on %s\n", cmd.Process.Pid, inputDevice)

	// Log stderr in background
	go func() {
		stderrScanner := bufio.NewScanner(stderr)
		for stderrScanner.Scan() {
			fmt.Printf("[Automation] stderr: %s\n", stderrScanner.Text())
		}
	}()

	// Get device min/max coordinates
	maxX, maxY := 0, 0
	minX, minY := 0, 0

	propsCmd := fmt.Sprintf("shell getevent -p %s", inputDevice)
	propsOutput, err := a.RunAdbCommand(deviceId, propsCmd)
	if err == nil {
		lines := strings.Split(propsOutput, "\n")
		// Regex to match "min 0, max 1079"
		re := regexp.MustCompile(`min\s+(-?\d+),\s+max\s+(-?\d+)`)

		for _, line := range lines {
			if strings.Contains(line, "ABS_MT_POSITION_X") || strings.Contains(line, "0035") {
				if matches := re.FindStringSubmatch(line); len(matches) >= 3 {
					minX, _ = strconv.Atoi(matches[1])
					maxX, _ = strconv.Atoi(matches[2])
				}
			}
			if strings.Contains(line, "ABS_MT_POSITION_Y") || strings.Contains(line, "0036") {
				if matches := re.FindStringSubmatch(line); len(matches) >= 3 {
					minY, _ = strconv.Atoi(matches[1])
					maxY, _ = strconv.Atoi(matches[2])
				}
			}
		}
	}
	fmt.Printf("[Automation] Touch device coords detected: X[%d, %d], Y[%d, %d]\n", minX, maxX, minY, maxY)

	// Store recording state
	touchRecordCmd[deviceId] = cmd
	touchRecordCancel[deviceId] = cancel
	touchRecordData[deviceId] = &TouchRecordingSession{
		DeviceID:    deviceId,
		StartTime:   time.Now(),
		RawEvents:   make([]string, 0),
		Resolution:  resolution,
		InputDevice: inputDevice,
		MaxX:        maxX,
		MaxY:        maxY,
		MinX:        minX,
		MinY:        minY,
	}

	// Start goroutine to read events
	go func() {
		scanner := bufio.NewScanner(stdout)
		lineCount := 0
		capturedCount := 0

		fmt.Printf("[Automation] Listening for events from: %s\n", inputDevice)

		for scanner.Scan() {
			line := scanner.Text()
			lineCount++

			// Debug: print first few lines to see what we're getting
			// With specific device, output usually looks like:
			// [ 1234.567890] EV_ABS       ABS_MT_POSITION_X    00000123
			if lineCount <= 10 {
				fmt.Printf("[Automation] Line %d: %s\n", lineCount, line)
			}

			// Filter: ensure it contains EV_
			if strings.Contains(line, "EV_") {
				touchRecordMu.Lock()
				if session, ok := touchRecordData[deviceId]; ok {
					session.RawEvents = append(session.RawEvents, line)
					capturedCount++
				}
				touchRecordMu.Unlock()

				// Check for touch up / release action to notify frontend (real-time feedback)
				// Use an else-if to prioritize BTN_TOUCH and avoid double-counting if both signals are in the same loop (though they are usually separate lines)
				// The main fix is ensuring we don't trigger for BOTH signals if they are emitted for the same lift event.
				if strings.Contains(line, "BTN_TOUCH") && (strings.Contains(line, "UP") || strings.HasSuffix(strings.TrimSpace(line), "00000000")) {
					wailsRuntime.EventsEmit(a.ctx, "touch-action-recorded", map[string]interface{}{
						"deviceId": deviceId,
					})
				} else if strings.Contains(line, "ABS_MT_TRACKING_ID") && strings.Contains(strings.ToLower(line), "ffffffff") {
					// Only use Tracking ID if this line isn't a BTN_TOUCH line (prevents double fire on line-by-line basis)
					wailsRuntime.EventsEmit(a.ctx, "touch-action-recorded", map[string]interface{}{
						"deviceId": deviceId,
					})
				}
			}
		}
		fmt.Printf("[Automation] Scanner finished: %d lines read, %d events captured\n", lineCount, capturedCount)
		if err := scanner.Err(); err != nil {
			fmt.Printf("[Automation] Scanner error: %v\n", err)
		}
	}()

	// Emit event
	wailsRuntime.EventsEmit(a.ctx, "touch-record-started", map[string]interface{}{
		"deviceId":    deviceId,
		"startTime":   time.Now().Unix(),
		"inputDevice": inputDevice,
	})

	return nil
}

// StopTouchRecording stops recording and returns the parsed touch script
func (a *App) StopTouchRecording(deviceId string) (*TouchScript, error) {
	// First, get the cancel function and command without holding the lock
	touchRecordMu.Lock()
	cancel, exists := touchRecordCancel[deviceId]
	cmd := touchRecordCmd[deviceId]
	touchRecordMu.Unlock()

	if !exists {
		return nil, fmt.Errorf("no active recording for this device")
	}

	// Cancel the recording - this stops the getevent process
	cancel()

	// Wait for process to finish - don't hold the lock here!
	// This allows the reading goroutine to finish processing remaining events
	if cmd != nil {
		_ = cmd.Wait()
	}

	// Give the reading goroutine a moment to finish processing
	time.Sleep(100 * time.Millisecond)

	// Now acquire the lock to get the recorded data
	touchRecordMu.Lock()
	defer touchRecordMu.Unlock()

	// Get recorded data
	session, ok := touchRecordData[deviceId]
	if !ok {
		return nil, fmt.Errorf("no recording data found")
	}

	fmt.Printf("[Automation] StopRecording: got %d raw events\n", len(session.RawEvents))

	// Parse raw events into TouchScript
	script := a.parseRawEvents(session)

	// Enrich with device model info
	info, err := a.GetDeviceInfo(deviceId)
	if err == nil {
		script.DeviceModel = info.Model
	}

	// Cleanup
	delete(touchRecordCmd, deviceId)
	delete(touchRecordCancel, deviceId)
	delete(touchRecordData, deviceId)

	// Emit event
	wailsRuntime.EventsEmit(a.ctx, "touch-record-stopped", map[string]interface{}{
		"deviceId":   deviceId,
		"eventCount": len(script.Events),
	})

	return script, nil
}

// IsRecordingTouch returns whether touch recording is active for a device
func (a *App) IsRecordingTouch(deviceId string) bool {
	touchRecordMu.Lock()
	defer touchRecordMu.Unlock()
	_, exists := touchRecordCmd[deviceId]
	return exists
}

// PickPointOnScreen waits for a single tap on the device screen and returns the coordinates
// Returns a map with x, y coordinates and a bounds string
func (a *App) PickPointOnScreen(deviceId string, timeoutSeconds int) (map[string]interface{}, error) {
	// Get touch input device
	inputDevice, err := a.GetTouchInputDevice(deviceId)
	if err != nil {
		return nil, fmt.Errorf("failed to find touch input device: %w", err)
	}

	// Get device resolution for coordinate scaling
	resolution, _ := a.GetDeviceResolution(deviceId)
	parts := strings.Split(resolution, "x")
	screenWidth, screenHeight := 1080, 1920
	if len(parts) == 2 {
		screenWidth, _ = strconv.Atoi(parts[0])
		screenHeight, _ = strconv.Atoi(parts[1])
	}

	// Get min/max coordinates for the touch device
	maxX, maxY := 0, 0
	minX, minY := 0, 0

	propsCmd := fmt.Sprintf("shell getevent -p %s", inputDevice)
	propsOutput, err := a.RunAdbCommand(deviceId, propsCmd)
	if err == nil {
		lines := strings.Split(propsOutput, "\n")
		re := regexp.MustCompile(`min\s+(-?\d+),\s+max\s+(-?\d+)`)

		for _, line := range lines {
			if strings.Contains(line, "ABS_MT_POSITION_X") || strings.Contains(line, "0035") {
				if matches := re.FindStringSubmatch(line); len(matches) >= 3 {
					minX, _ = strconv.Atoi(matches[1])
					maxX, _ = strconv.Atoi(matches[2])
				}
			}
			if strings.Contains(line, "ABS_MT_POSITION_Y") || strings.Contains(line, "0036") {
				if matches := re.FindStringSubmatch(line); len(matches) >= 3 {
					minY, _ = strconv.Atoi(matches[1])
					maxY, _ = strconv.Atoi(matches[2])
				}
			}
		}
	}

	// Default timeout 30 seconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Start getevent
	cmd := exec.CommandContext(ctx, a.adbPath, "-s", deviceId, "shell", "getevent", "-lt", inputDevice)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start getevent: %w", err)
	}

	// Emit event to notify frontend that we're waiting
	wailsRuntime.EventsEmit(a.ctx, "point-picker-started", map[string]interface{}{
		"deviceId": deviceId,
	})

	// Channel to receive result
	resultChan := make(chan map[string]interface{}, 1)
	errChan := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(stdout)
		var currentX, currentY int
		var hasX, hasY bool
		touchDown := false

		for scanner.Scan() {
			line := scanner.Text()

			// Parse X coordinate
			if strings.Contains(line, "ABS_MT_POSITION_X") {
				re := regexp.MustCompile(`([0-9a-fA-F]{8})\s*$`)
				if matches := re.FindStringSubmatch(line); len(matches) >= 2 {
					val, _ := strconv.ParseInt(matches[1], 16, 32)
					currentX = int(val)
					hasX = true
				}
			}

			// Parse Y coordinate
			if strings.Contains(line, "ABS_MT_POSITION_Y") {
				re := regexp.MustCompile(`([0-9a-fA-F]{8})\s*$`)
				if matches := re.FindStringSubmatch(line); len(matches) >= 2 {
					val, _ := strconv.ParseInt(matches[1], 16, 32)
					currentY = int(val)
					hasY = true
				}
			}

			// Detect touch down
			if strings.Contains(line, "BTN_TOUCH") && strings.Contains(line, "DOWN") {
				touchDown = true
			}
			if strings.Contains(line, "BTN_TOUCH") && strings.HasSuffix(strings.TrimSpace(line), "00000001") {
				touchDown = true
			}

			// Detect touch up - this is when we capture the point
			isTouchUp := false
			if strings.Contains(line, "BTN_TOUCH") && (strings.Contains(line, "UP") || strings.HasSuffix(strings.TrimSpace(line), "00000000")) {
				isTouchUp = true
			}
			if strings.Contains(line, "ABS_MT_TRACKING_ID") && strings.Contains(strings.ToLower(line), "ffffffff") {
				isTouchUp = true
			}

			if isTouchUp && touchDown && hasX && hasY {
				// Scale coordinates to screen resolution
				scaledX := currentX
				scaledY := currentY

				if maxX > 0 && maxY > 0 {
					scaledX = (currentX - minX) * screenWidth / (maxX - minX + 1)
					scaledY = (currentY - minY) * screenHeight / (maxY - minY + 1)
				}

				// Create bounds string (a small area around the tap point)
				tapSize := 10 // 10 pixel tap area
				x1 := scaledX - tapSize
				y1 := scaledY - tapSize
				x2 := scaledX + tapSize
				y2 := scaledY + tapSize

				// Clamp to screen bounds
				if x1 < 0 {
					x1 = 0
				}
				if y1 < 0 {
					y1 = 0
				}
				if x2 > screenWidth {
					x2 = screenWidth
				}
				if y2 > screenHeight {
					y2 = screenHeight
				}

				bounds := fmt.Sprintf("[%d,%d][%d,%d]", x1, y1, x2, y2)

				resultChan <- map[string]interface{}{
					"x":      scaledX,
					"y":      scaledY,
					"bounds": bounds,
					"rawX":   currentX,
					"rawY":   currentY,
				}
				return
			}
		}

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			errChan <- err
		}
	}()

	// Wait for result or timeout
	select {
	case result := <-resultChan:
		cmd.Process.Kill()
		wailsRuntime.EventsEmit(a.ctx, "point-picker-completed", result)
		return result, nil
	case err := <-errChan:
		cmd.Process.Kill()
		return nil, err
	case <-ctx.Done():
		cmd.Process.Kill()
		wailsRuntime.EventsEmit(a.ctx, "point-picker-timeout", map[string]interface{}{
			"deviceId": deviceId,
		})
		return nil, fmt.Errorf("timeout waiting for tap")
	}
}

// CancelPointPicker can be used to cancel an ongoing point picker (not currently tracked per-device, relies on timeout)
// For now, the timeout mechanism handles cancellation

// GetRecordingEventCount returns the current number of recorded events
func (a *App) GetRecordingEventCount(deviceId string) int {
	touchRecordMu.Lock()
	defer touchRecordMu.Unlock()
	if session, ok := touchRecordData[deviceId]; ok {
		return len(session.RawEvents)
	}
	return 0
}

// parseRawEvents converts raw getevent output into TouchScript
func (a *App) parseRawEvents(session *TouchRecordingSession) *TouchScript {
	script := &TouchScript{
		DeviceID:   session.DeviceID,
		Resolution: session.Resolution,
		CreatedAt:  session.StartTime.Format(time.RFC3339),
		Events:     make([]TouchEvent, 0),
	}

	fmt.Printf("[Automation] Parsing %d raw events\n", len(session.RawEvents))

	if len(session.RawEvents) == 0 {
		return script
	}

	// Parse resolution for coordinate scaling
	var screenW, screenH int = 1080, 1920
	if parts := strings.Split(session.Resolution, "x"); len(parts) == 2 {
		screenW, _ = strconv.Atoi(parts[0])
		screenH, _ = strconv.Atoi(parts[1])
	}

	// Regular expression to parse getevent lines
	// Format 1 (all devices): [ 1234.567890] /dev/input/event2: EV_ABS ABS_MT_POSITION_X 00000500
	// Format 2 (specific device): [ 1234.567890] EV_ABS       ABS_MT_POSITION_X    00000500
	// Make the device path optional
	// Regular expression to parse getevent lines
	// Format: [ 1234.567890] EV_ABS       ABS_MT_POSITION_X    00000500
	// We need to be flexible with whitespace
	re := regexp.MustCompile(`\[\s*([\d.]+)\].*?(EV_\w+)\s+(\w+)\s+([0-9a-fA-F]+|DOWN|UP)`)

	// Use stored max coordinates, default to screen parsing if missing (though they shouldn't be)
	var maxX, maxY int = session.MaxX, session.MaxY
	var minX, minY int = session.MinX, session.MinY

	// Validation: if we didn't get valid range from startRecording, fall back to simple scaling
	// This avoids divide by zero
	if maxX == minX {
		maxX = screenW
		minX = 0
	}
	if maxY == minY {
		maxY = screenH
		minY = 0
	}

	fmt.Printf("[Automation] Screen: %dx%d, Coord Range: X[%d-%d] Y[%d-%d]\n", screenW, screenH, minX, maxX, minY, maxY)

	var firstTimestamp float64 = -1
	var currentX, currentY int = -1, -1
	var touchStartTime float64 = -1
	var touchStartX, touchStartY int = -1, -1
	var tracking bool = false

	for _, line := range session.RawEvents {
		matches := re.FindStringSubmatch(line)
		if len(matches) < 5 {
			continue
		}

		timestamp, _ := strconv.ParseFloat(matches[1], 64)
		evType := matches[2]
		evCode := matches[3]
		evValue := matches[4]

		if firstTimestamp < 0 {
			firstTimestamp = timestamp
		}

		relativeMs := int64((timestamp - firstTimestamp) * 1000)

		// Handle special value cases like UP/DOWN for BTN_TOUCH
		if evValue == "DOWN" {
			evValue = "00000001"
		} else if evValue == "UP" {
			evValue = "00000000"
		}

		if evType == "EV_ABS" {
			// Parse as unsigned 32-bit int first, then convert to signed int32
			// This handles -1 (0xffffffff) correctly -> -1
			uValue, err := strconv.ParseUint(evValue, 16, 32)
			if err != nil {
				continue
			}
			value := int32(uValue)

			switch evCode {
			case "ABS_MT_TRACKING_ID":
				// Tracking ID -1 (0xffffffff) means finger up
				if value != -1 && !tracking {
					// Finger down - Start of new stroke
					tracking = true
					touchStartTime = timestamp
					// Reset start coords to detect if they change in this stroke
					touchStartX = -1
					touchStartY = -1
				} else if value == -1 && tracking {
					// Finger up - End of stroke
					tracking = false
					duration := int((timestamp - touchStartTime) * 1000)

					// If start coords were never updated in this stroke, it means
					// they didn't change from the previous state (Input Protocol Type B)
					// So use the current state as the start.
					if touchStartX == -1 {
						touchStartX = currentX
					}
					if touchStartY == -1 {
						touchStartY = currentY
					}

					// Ensure we have valid coordinates before emitting
					if touchStartX == -1 || touchStartY == -1 || currentX == -1 || currentY == -1 {
						fmt.Printf("[Automation] Warning: Skipping event with invalid coords: Start(%d,%d) End(%d,%d)\n",
							touchStartX, touchStartY, currentX, currentY)
						continue
					}

					// Scale coordinates using floating point arithmetic to avoid precision loss
					// Formula: screen_x = (raw_x - min_raw_x) * screen_width / (max_raw_x - min_raw_x)
					var scaledStartX, scaledStartY, scaledEndX, scaledEndY int

					// Helper for proper rounding: int(val + 0.5)
					round := func(val float64) int {
						return int(val + 0.5)
					}

					if maxX > minX {
						width := float64(maxX - minX)
						scaledStartX = round(float64(touchStartX-minX) * float64(screenW) / width)
						scaledEndX = round(float64(currentX-minX) * float64(screenW) / width)
					} else {
						scaledStartX = touchStartX
						scaledEndX = currentX
					}

					if maxY > minY {
						height := float64(maxY - minY)
						scaledStartY = round(float64(touchStartY-minY) * float64(screenH) / height)
						scaledEndY = round(float64(currentY-minY) * float64(screenH) / height)
					} else {
						scaledStartY = touchStartY
						scaledEndY = currentY
					}

					// Debug log for coordinate mapping verification
					// fmt.Printf("[Automation] Coord mapping: Raw(%d,%d) -> Screen(%d,%d) [Max: %dx%d -> %dx%d]\n",
					// 	touchStartX, touchStartY, scaledStartX, scaledStartY, maxX, maxY, screenW, screenH)

					// Calculate distance
					dx := scaledEndX - scaledStartX
					dy := scaledEndY - scaledStartY
					distance := dx*dx + dy*dy

					event := TouchEvent{
						Timestamp: relativeMs,
					}

					if distance < 2500 && duration < 300 {
						// Tap: small movement and quick release
						event.Type = "tap"
						event.X = scaledStartX
						event.Y = scaledStartY
					} else {
						// Swipe: significant movement
						event.Type = "swipe"
						event.X = scaledStartX
						event.Y = scaledStartY
						event.X2 = scaledEndX
						event.Y2 = scaledEndY
						event.Duration = duration
					}

					script.Events = append(script.Events, event)
				}

			case "BTN_TOUCH":
				// Support for older devices or single-touch screens (Protocol A)
				// Value 1 = Down, 0 = Up
				if value == 1 && !tracking {
					// Finger down
					tracking = true
					touchStartTime = timestamp
					touchStartX = -1
					touchStartY = -1
				} else if value == 0 && tracking {
					// Finger up
					tracking = false
					duration := int((timestamp - touchStartTime) * 1000)

					// Fallback for coordinates if not updated
					if touchStartX == -1 {
						touchStartX = currentX
					}
					if touchStartY == -1 {
						touchStartY = currentY
					}

					if touchStartX == -1 || touchStartY == -1 || currentX == -1 || currentY == -1 {
						continue
					}

					// Shared logic for event generation...
					// To avoid code duplication, we could refactor, but for this specific tool usage
					// we will duplicate the scaling and event creation logic for stability.

					var scaledStartX, scaledStartY, scaledEndX, scaledEndY int

					// Helper for proper rounding
					round := func(val float64) int { return int(val + 0.5) }

					if maxX > minX {
						width := float64(maxX - minX)
						scaledStartX = round(float64(touchStartX-minX) * float64(screenW) / width)
						scaledEndX = round(float64(currentX-minX) * float64(screenW) / width)
					} else {
						scaledStartX = touchStartX
						scaledEndX = currentX
					}

					if maxY > minY {
						height := float64(maxY - minY)
						scaledStartY = round(float64(touchStartY-minY) * float64(screenH) / height)
						scaledEndY = round(float64(currentY-minY) * float64(screenH) / height)
					} else {
						scaledStartY = touchStartY
						scaledEndY = currentY
					}

					dx := scaledEndX - scaledStartX
					dy := scaledEndY - scaledStartY
					distance := dx*dx + dy*dy

					event := TouchEvent{
						Timestamp: relativeMs,
					}

					if distance < 2500 && duration < 300 {
						event.Type = "tap"
						event.X = scaledStartX
						event.Y = scaledStartY
					} else {
						event.Type = "swipe"
						event.X = scaledStartX
						event.Y = scaledStartY
						event.X2 = scaledEndX
						event.Y2 = scaledEndY
						event.Duration = duration
					}
					script.Events = append(script.Events, event)
				}

			case "ABS_MT_POSITION_X":
				// Some devices only report changes.
				currentX = int(value)
				if tracking {
					if touchStartX == -1 {
						touchStartX = currentX
					}
				}

			case "ABS_MT_POSITION_Y":
				currentY = int(value)
				if tracking {
					if touchStartY == -1 {
						touchStartY = currentY
					}
				}
			}
		}
	}

	return script
}

// PlayTouchScript plays back a recorded touch script
func (a *App) PlayTouchScript(deviceId string, script TouchScript) error {
	touchPlaybackMu.Lock()
	if _, exists := touchPlaybackCancel[deviceId]; exists {
		touchPlaybackMu.Unlock()
		return fmt.Errorf("playback already in progress")
	}

	ctx, cancel := context.WithCancel(context.Background())
	touchPlaybackCancel[deviceId] = cancel
	touchPlaybackMu.Unlock()

	go func() {
		defer func() {
			touchPlaybackMu.Lock()
			delete(touchPlaybackCancel, deviceId)
			touchPlaybackMu.Unlock()

			wailsRuntime.EventsEmit(a.ctx, "touch-playback-completed", map[string]interface{}{
				"deviceId": deviceId,
			})
		}()

		// Use the synchronous helper
		_ = a.playTouchScriptSync(ctx, deviceId, script, func(current, total int) {
			wailsRuntime.EventsEmit(a.ctx, "touch-playback-progress", map[string]interface{}{
				"deviceId": deviceId,
				"current":  current,
				"total":    total,
			})
		})
	}()

	wailsRuntime.EventsEmit(a.ctx, "touch-playback-started", map[string]interface{}{
		"deviceId": deviceId,
		"total":    len(script.Events),
	})

	return nil
}

// playTouchScriptSync is the synchronous core logic for playing a script
func (a *App) playTouchScriptSync(ctx context.Context, deviceId string, script TouchScript, progressCb func(int, int)) error {
	startTime := time.Now()
	total := len(script.Events)

	// 1. Get target device resolution
	targetResStr, err := a.GetDeviceResolution(deviceId)
	var scaleX, scaleY float64 = 1.0, 1.0

	if err == nil && script.Resolution != "" {
		// Parse target resolution
		targetW, targetH, ok1 := parseResolution(targetResStr)
		// Parse source resolution
		sourceW, sourceH, ok2 := parseResolution(script.Resolution)

		if ok1 && ok2 && sourceW > 0 && sourceH > 0 {
			scaleX = float64(targetW) / float64(sourceW)
			scaleY = float64(targetH) / float64(sourceH)
			fmt.Printf("Auto-scaling enabled: Source=%dx%d, Target=%dx%d, ScaleX=%.2f, ScaleY=%.2f\n",
				sourceW, sourceH, targetW, targetH, scaleX, scaleY)
		}
	}

	for i, event := range script.Events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Wait until it's time to execute this event
		elapsed := time.Since(startTime).Milliseconds()
		if event.Timestamp > elapsed {
			sleepDuration := time.Duration(event.Timestamp-elapsed) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(sleepDuration):
			}
		}

		// Check pause
		a.checkPause(deviceId)

		// Apply scaling
		finalX := int(float64(event.X) * scaleX)
		finalY := int(float64(event.Y) * scaleY)

		// Execute the touch event
		var cmd string
		switch event.Type {
		case "tap":
			cmd = fmt.Sprintf("shell input tap %d %d", finalX, finalY)
		case "swipe":
			finalX2 := int(float64(event.X2) * scaleX)
			finalY2 := int(float64(event.Y2) * scaleY)
			cmd = fmt.Sprintf("shell input swipe %d %d %d %d %d",
				finalX, finalY, finalX2, finalY2, event.Duration)
		case "wait":
			time.Sleep(time.Duration(event.Duration) * time.Millisecond)
			continue
		default:
			continue
		}

		_, _ = a.RunAdbCommand(deviceId, cmd)

		if progressCb != nil {
			progressCb(i+1, total)
		}
	}
	return nil
}

// Helper to parse "WxH" string
func parseResolution(res string) (int, int, bool) {
	parts := strings.Split(res, "x")
	if len(parts) != 2 {
		// Try to handle "Physical size: WxH" format just in case, though GetDeviceResolution usually cleans it
		// But let's stick to simple split as GetDeviceResolution seems to return "WxH" or raw output
		// Let's rely on standard format
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return w, h, true
}

// StopTouchPlayback stops an ongoing touch playback
func (a *App) StopTouchPlayback(deviceId string) {
	touchPlaybackMu.Lock()
	defer touchPlaybackMu.Unlock()

	if cancel, exists := touchPlaybackCancel[deviceId]; exists {
		cancel()
		delete(touchPlaybackCancel, deviceId)
	}
}

// IsPlayingTouch returns whether touch playback is active for a device
func (a *App) IsPlayingTouch(deviceId string) bool {
	touchPlaybackMu.Lock()
	defer touchPlaybackMu.Unlock()
	_, exists := touchPlaybackCancel[deviceId]
	return exists
}

// PauseTask pauses the running task (or script)
func (a *App) PauseTask(deviceId string) {
	taskPauseMu.Lock()
	defer taskPauseMu.Unlock()

	if _, paused := taskIsPaused[deviceId]; !paused {
		// Create a blocking channel
		taskPauseSignal[deviceId] = make(chan struct{})
		taskIsPaused[deviceId] = true
		wailsRuntime.EventsEmit(a.ctx, "task-paused", map[string]interface{}{"deviceId": deviceId})
	}
}

// ResumeTask resumes the paused task
func (a *App) ResumeTask(deviceId string) {
	taskPauseMu.Lock()
	defer taskPauseMu.Unlock()

	if ch, paused := taskPauseSignal[deviceId]; paused {
		close(ch) // Unblock waiting goroutines
		delete(taskPauseSignal, deviceId)
		delete(taskIsPaused, deviceId)
		wailsRuntime.EventsEmit(a.ctx, "task-resumed", map[string]interface{}{"deviceId": deviceId})
	}
}

// StopTask stops the task (alias for StopTouchPlayback for now, but explicit for API)
func (a *App) StopTask(deviceId string) {
	// Resume first if paused to allow exit
	a.ResumeTask(deviceId)
	a.StopTouchPlayback(deviceId)
}

// checkPause blocks if the device is paused
func (a *App) checkPause(deviceId string) {
	taskPauseMu.Lock()
	ch, paused := taskPauseSignal[deviceId]
	taskPauseMu.Unlock()

	if paused && ch != nil {
		<-ch // Wait until channel is closed (resumed)
	}
}

// getScriptsPath returns the path to the scripts directory
func (a *App) getScriptsPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	scriptsPath := filepath.Join(configDir, "Gaze", "scripts")
	_ = os.MkdirAll(scriptsPath, 0755)
	return scriptsPath
}

// SaveTouchScript saves a touch script to file
func (a *App) SaveTouchScript(script TouchScript) error {
	scriptsPath := a.getScriptsPath()

	// Sanitize filename
	safeName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(script.Name, "_")
	if safeName == "" {
		safeName = fmt.Sprintf("script_%d", time.Now().Unix())
	}

	filePath := filepath.Join(scriptsPath, safeName+".json")

	data, err := json.MarshalIndent(script, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal script: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write script file: %w", err)
	}

	return nil
}

// LoadTouchScripts loads all saved touch scripts
func (a *App) LoadTouchScripts() ([]TouchScript, error) {
	scriptsPath := a.getScriptsPath()

	entries, err := os.ReadDir(scriptsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []TouchScript{}, nil
		}
		return nil, fmt.Errorf("failed to read scripts directory: %w", err)
	}

	scripts := make([]TouchScript, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(scriptsPath, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var script TouchScript
		if err := json.Unmarshal(data, &script); err != nil {
			continue
		}

		scripts = append(scripts, script)
	}

	return scripts, nil
}

// DeleteTouchScript deletes a saved touch script
func (a *App) DeleteTouchScript(name string) error {
	scriptsPath := a.getScriptsPath()

	safeName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(name, "_")
	filePath := filepath.Join(scriptsPath, safeName+".json")

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("script not found")
		}
		return fmt.Errorf("failed to delete script: %w", err)
	}

	return nil
}

// RenameTouchScript renames a script
func (a *App) RenameTouchScript(oldName, newName string) error {
	scriptsPath := a.getScriptsPath()

	// 1. Read old file
	safeOldName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(oldName, "_")
	oldFilePath := filepath.Join(scriptsPath, safeOldName+".json")

	data, err := os.ReadFile(oldFilePath)
	if err != nil {
		return fmt.Errorf("script not found: %w", err)
	}

	var script TouchScript
	if err := json.Unmarshal(data, &script); err != nil {
		return fmt.Errorf("failed to parse script: %w", err)
	}

	// 2. Update name
	script.Name = newName

	// 3. Save new file
	if err := a.SaveTouchScript(script); err != nil {
		return err
	}

	// 4. Delete old file if name changed (and safe names are different)
	safeNewName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(newName, "_")
	if safeOldName != safeNewName {
		_ = os.Remove(oldFilePath)
	}

	return nil
}

// ---------------- Task Orchestration ----------------

// getTasksPath returns the path to the tasks directory
func (a *App) getTasksPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	tasksPath := filepath.Join(configDir, "Gaze", "tasks")
	_ = os.MkdirAll(tasksPath, 0755)
	return tasksPath
}

// SaveScriptTask saves a task compilation
func (a *App) SaveScriptTask(task ScriptTask) error {
	tasksPath := a.getTasksPath()

	safeName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(task.Name, "_")
	if safeName == "" {
		safeName = fmt.Sprintf("task_%d", time.Now().Unix())
	}

	filePath := filepath.Join(tasksPath, safeName+".json")

	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write task file: %w", err)
	}

	return nil
}

// LoadScriptTasks loads all saved tasks
func (a *App) LoadScriptTasks() ([]ScriptTask, error) {
	tasksPath := a.getTasksPath()

	entries, err := os.ReadDir(tasksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ScriptTask{}, nil
		}
		return nil, fmt.Errorf("failed to read tasks directory: %w", err)
	}

	tasks := make([]ScriptTask, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(tasksPath, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var task ScriptTask
		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// DeleteScriptTask deletes a saved task
func (a *App) DeleteScriptTask(name string) error {
	tasksPath := a.getTasksPath()

	safeName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(name, "_")
	filePath := filepath.Join(tasksPath, safeName+".json")

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("task not found")
		}
		return fmt.Errorf("failed to delete task: %w", err)
	}

	return nil
}

// RunScriptTask executes a composite task
func (a *App) RunScriptTask(deviceId string, task ScriptTask) error {
	touchPlaybackMu.Lock()
	if _, exists := touchPlaybackCancel[deviceId]; exists {
		touchPlaybackMu.Unlock()
		return fmt.Errorf("playback already in progress")
	}

	ctx, cancel := context.WithCancel(context.Background())
	touchPlaybackCancel[deviceId] = cancel
	touchPlaybackMu.Unlock()

	go func() {
		defer func() {
			touchPlaybackMu.Lock()
			delete(touchPlaybackCancel, deviceId)
			touchPlaybackMu.Unlock()

			wailsRuntime.EventsEmit(a.ctx, "task-completed", map[string]interface{}{
				"deviceId": deviceId,
				"taskName": task.Name,
			})
		}()

		wailsRuntime.EventsEmit(a.ctx, "task-started", map[string]interface{}{
			"deviceId": deviceId,
			"taskName": task.Name,
			"steps":    len(task.Steps),
		})

		// Load all available scripts first to quickly look them up
		scripts, _ := a.LoadTouchScripts()
		scriptMap := make(map[string]TouchScript)
		for _, s := range scripts {
			scriptMap[s.Name] = s
		}

		for i, step := range task.Steps {
			// Check cancel
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Check pause
			a.checkPause(deviceId)

			wailsRuntime.EventsEmit(a.ctx, "task-step-started", map[string]interface{}{
				"deviceId":  deviceId,
				"stepIndex": i,
				"type":      step.Type,
				"value":     step.Value,
			})

			loopCount := step.Loop
			if loopCount < 1 {
				loopCount = 1
			}

			for l := 0; l < loopCount; l++ {
				// Check cancel inside loop
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Check pause inside loop
				a.checkPause(deviceId)

				// Emit step progress including loop info
				wailsRuntime.EventsEmit(a.ctx, "task-step-running", map[string]interface{}{
					"deviceId":    deviceId,
					"taskName":    task.Name,
					"stepIndex":   i,
					"totalSteps":  len(task.Steps),
					"currentLoop": l + 1,
					"totalLoops":  loopCount,
					"type":        step.Type,
					"value":       step.Value,
				})

				if step.Type == "wait" {
					duration, _ := strconv.Atoi(step.Value)
					if duration > 0 {
						time.Sleep(time.Duration(duration) * time.Millisecond)
					}
				} else if step.Type == "script" {
					script, ok := scriptMap[step.Value]
					if !ok {
						fmt.Printf("[Automation] Script not found: %s\n", step.Value)
						continue
					}

					// Run the script synchronously using our helper
					err := a.playTouchScriptSync(ctx, deviceId, script, func(current, total int) {
						// Optional: emit more granular progress if needed,
						// but task-step-running might be enough for general status
					})
					if err != nil {
						// Context cancelled or error
						return
					}
				} else if step.Type == "adb" {
					// Execute ADB command
					// step.Value contains the command arguments (e.g. "shell input keyevent 3")
					// Users might provide "shell input ..." or just "input ..."
					// RunAdbCommand expects the full arguments string.
					cmd := step.Value
					_, err := a.RunAdbCommand(deviceId, cmd)
					if err != nil {
						fmt.Printf("[Automation] ADB command failed: %s, error: %v\n", cmd, err)
						// Decide if we should stop the task. For now, continue but log error.
					}
				} else if step.Type == "check" {
					// Content-aware check: wait for element to appear
					timeout := step.WaitTimeout
					if timeout <= 0 {
						timeout = 5000 // Default 5s
					}

					checkType := step.CheckType
					if checkType == "" {
						checkType = "text"
					}

					fmt.Printf("[Automation] Checking for element: %s=%s (timeout: %dms)\n", checkType, step.CheckValue, timeout)

					startCheck := time.Now()
					found := false
					for {
						// Check cancel/pause
						select {
						case <-ctx.Done():
							return
						default:
						}
						a.checkPause(deviceId)

						wailsRuntime.EventsEmit(a.ctx, "task-step-running", map[string]interface{}{
							"deviceId":      deviceId,
							"taskName":      task.Name,
							"stepIndex":     i,
							"currentAction": fmt.Sprintf("Checking UI: %s=%s", checkType, step.CheckValue),
						})

						result, err := a.GetUIHierarchy(deviceId)
						if err == nil && a.FindElement(result.Root, checkType, step.CheckValue) {
							found = true
							break
						}

						if time.Since(startCheck) >= time.Duration(timeout)*time.Millisecond {
							break
						}
						time.Sleep(1 * time.Second)
					}

					if !found {
						fmt.Printf("[Automation] Element not found: %s=%s\n", checkType, step.CheckValue)
						if step.OnFailure == "stop" {
							wailsRuntime.EventsEmit(a.ctx, "task-error", map[string]interface{}{
								"deviceId": deviceId,
								"error":    fmt.Sprintf("Element not found: %s=%s", checkType, step.CheckValue),
							})
							return
						}
					} else {
						fmt.Printf("[Automation] Element found: %s=%s\n", checkType, step.CheckValue)
					}
				}
			}

			// Apply PostDelay after the step (all loops) is completed
			if step.PostDelay > 0 {
				wailsRuntime.EventsEmit(a.ctx, "task-step-running", map[string]interface{}{
					"deviceId":      deviceId,
					"taskName":      task.Name,
					"stepIndex":     i,
					"currentAction": fmt.Sprintf("Post-Wait: %dms", step.PostDelay),
				})

				// Check cancel before waiting
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Check pause
				a.checkPause(deviceId)

				time.Sleep(time.Duration(step.PostDelay) * time.Millisecond)
			}
		}
	}()

	return nil
}

// UI Hierarchy structures for parsing uiautomator dump
type UINode struct {
	XMLName       xml.Name `xml:"node" json:"-"`
	Text          string   `xml:"text,attr" json:"text"`
	ResourceID    string   `xml:"resource-id,attr" json:"resourceId"`
	Class         string   `xml:"class,attr" json:"class"`
	Package       string   `xml:"package,attr" json:"package"`
	ContentDesc   string   `xml:"content-desc,attr" json:"contentDesc"`
	Checkable     string   `xml:"checkable,attr" json:"checkable"`
	Checked       string   `xml:"checked,attr" json:"checked"`
	Clickable     string   `xml:"clickable,attr" json:"clickable"`
	Enabled       string   `xml:"enabled,attr" json:"enabled"`
	Focusable     string   `xml:"focusable,attr" json:"focusable"`
	Focused       string   `xml:"focused,attr" json:"focused"`
	Scrollable    string   `xml:"scrollable,attr" json:"scrollable"`
	LongClickable string   `xml:"long-clickable,attr" json:"longClickable"`
	Password      string   `xml:"password,attr" json:"password"`
	Selected      string   `xml:"selected,attr" json:"selected"`
	Bounds        string   `xml:"bounds,attr" json:"bounds"`
	Nodes         []UINode `xml:"node" json:"nodes"`
}

type UIHierarchy struct {
	XMLName xml.Name `xml:"hierarchy"`
	Nodes   []UINode `xml:"node"`
}

type UIHierarchyResult struct {
	Root   *UINode `json:"root"`
	RawXML string  `json:"rawXml"`
}

// GetUIHierarchy dumps the UI hierarchy and parses it
func (a *App) GetUIHierarchy(deviceId string) (*UIHierarchyResult, error) {
	// Try dumping several times as it can be flaky
	var xmlContent string
	var err error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		// Dump to a temporary file on device
		// We use /data/local/tmp as it's usually more reliable than /sdcard
		dumpFile := "/data/local/tmp/view.xml"
		dumpCmd := fmt.Sprintf("shell uiautomator dump %s", dumpFile)
		_, err = a.RunAdbCommand(deviceId, dumpCmd)
		if err == nil {
			// Read the file content
			catCmd := fmt.Sprintf("shell cat %s", dumpFile)
			xmlContent, err = a.RunAdbCommand(deviceId, catCmd)
			if err == nil && strings.Contains(xmlContent, "<?xml") {
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil || xmlContent == "" {
		return nil, fmt.Errorf("failed to dump UI after %d attempts: %v", maxRetries, err)
	}

	// Basic cleanup if output has extra stuff (sometimes ADB adds headers or footers)
	startIdx := strings.Index(xmlContent, "<?xml")
	if startIdx != -1 {
		xmlContent = xmlContent[startIdx:]
	}
	endIdx := strings.LastIndex(xmlContent, ">")
	if endIdx != -1 && endIdx < len(xmlContent)-1 {
		xmlContent = xmlContent[:endIdx+1]
	}

	rawXml := xmlContent // Save cleaned XML

	// Fix common XML escaping issues if any
	// Go's regexp doesn't support lookaheads, so we use a safe replacement chain
	xmlContent = strings.ReplaceAll(xmlContent, "&", "&amp;")
	xmlContent = strings.ReplaceAll(xmlContent, "&amp;amp;", "&amp;")
	xmlContent = strings.ReplaceAll(xmlContent, "&amp;lt;", "&lt;")
	xmlContent = strings.ReplaceAll(xmlContent, "&amp;gt;", "&gt;")
	xmlContent = strings.ReplaceAll(xmlContent, "&amp;quot;", "&quot;")
	xmlContent = strings.ReplaceAll(xmlContent, "&amp;apos;", "&apos;")
	xmlContent = strings.ReplaceAll(xmlContent, "&amp;#", "&#") // Fix numeric entities

	var root UIHierarchy
	err = xml.Unmarshal([]byte(xmlContent), &root)
	if err != nil {
		return nil, fmt.Errorf("failed to parse UI XML (length: %d): %w", len(xmlContent), err)
	}

	var finalRoot *UINode
	if len(root.Nodes) == 1 {
		finalRoot = &root.Nodes[0]
	} else {
		finalRoot = &UINode{
			Class:   "android.view.View",
			Text:    "Root Container",
			Package: root.Nodes[0].Package,
			Bounds:  "[0,0][0,0]",
			Nodes:   root.Nodes,
		}
	}

	return &UIHierarchyResult{
		Root:   finalRoot,
		RawXML: rawXml,
	}, nil
}

// FindElement recursively searches for an element matching the criteria
func (a *App) FindElement(node *UINode, checkType, checkValue string) bool {
	match := false
	switch checkType {
	case "text":
		match = node.Text == checkValue
	case "id":
		match = node.ResourceID == checkValue || strings.HasSuffix(node.ResourceID, ":id/"+checkValue)
	case "class":
		match = node.Class == checkValue
	case "contains":
		match = strings.Contains(node.Text, checkValue) || strings.Contains(node.ContentDesc, checkValue)
	case "description":
		match = node.ContentDesc == checkValue
	case "bounds":
		match = node.Bounds == checkValue
	}

	if match {
		return true
	}

	for i := range node.Nodes {
		if a.FindElement(&node.Nodes[i], checkType, checkValue) {
			return true
		}
	}

	return false
}

// GetElementsWithText returns all elements containing the given text (useful for debugging/frontend)
func (a *App) GetElementsWithText(deviceId string, text string) ([]map[string]interface{}, error) {
	result, err := a.GetUIHierarchy(deviceId)
	if err != nil {
		return nil, err
	}
	root := result.Root

	var results []map[string]interface{}
	var find func(*UINode)
	find = func(node *UINode) {
		if strings.Contains(node.Text, text) || strings.Contains(node.ContentDesc, text) {
			results = append(results, map[string]interface{}{
				"text":       node.Text,
				"resourceId": node.ResourceID,
				"bounds":     node.Bounds,
				"class":      node.Class,
			})
		}
		for i := range node.Nodes {
			find(&node.Nodes[i])
		}
	}

	find(root)
	return results, nil
}

// SearchResult represents a search result with path information
type SearchResult struct {
	Node   *UINode  `json:"node"`
	Path   string   `json:"path"`
	Depth  int      `json:"depth"`
	Index  int      `json:"index"`
}

// SearchElementsXPath searches elements using XPath-like syntax
// Supports: //node[@attr='value'], //node[@attr], //ClassName, //node[contains(@attr,'value')]
func (a *App) SearchElementsXPath(root *UINode, xpath string) []SearchResult {
	var results []SearchResult
	xpath = strings.TrimSpace(xpath)

	if !strings.HasPrefix(xpath, "//") {
		return results
	}

	query := strings.TrimPrefix(xpath, "//")

	// Parse the XPath expression
	var className string
	var conditions []struct {
		attr     string
		op       string // "=" or "contains"
		value    string
		hasValue bool
	}

	// Check for predicate brackets
	bracketIdx := strings.Index(query, "[")
	if bracketIdx != -1 {
		className = query[:bracketIdx]
		predicate := query[bracketIdx+1 : len(query)-1] // Remove [ and ]

		// Split by " and " (case insensitive)
		parts := regexp.MustCompile(`(?i)\s+and\s+`).Split(predicate, -1)

		for _, part := range parts {
			part = strings.TrimSpace(part)

			// contains(@attr, 'value')
			if strings.HasPrefix(part, "contains(") {
				re := regexp.MustCompile(`contains\(@(\w+),\s*['"]([^'"]*)['"]\)`)
				if matches := re.FindStringSubmatch(part); len(matches) == 3 {
					conditions = append(conditions, struct {
						attr     string
						op       string
						value    string
						hasValue bool
					}{matches[1], "contains", matches[2], true})
				}
			} else if strings.HasPrefix(part, "@") {
				// @attr='value' or @attr
				part = strings.TrimPrefix(part, "@")
				if strings.Contains(part, "=") {
					eqParts := strings.SplitN(part, "=", 2)
					attr := strings.TrimSpace(eqParts[0])
					value := strings.Trim(strings.TrimSpace(eqParts[1]), "'\"")
					conditions = append(conditions, struct {
						attr     string
						op       string
						value    string
						hasValue bool
					}{attr, "=", value, true})
				} else {
					// Just @attr (check if attribute exists and is non-empty)
					conditions = append(conditions, struct {
						attr     string
						op       string
						value    string
						hasValue bool
					}{part, "exists", "", false})
				}
			}
		}
	} else {
		className = query
	}

	// Recursive search
	var search func(node *UINode, path string, depth int, index int)
	search = func(node *UINode, path string, depth int, index int) {
		if node == nil {
			return
		}

		// Check class name match
		classMatch := className == "" || className == "node" || className == "*"
		if !classMatch {
			// Check full class name or short name
			shortName := node.Class
			if idx := strings.LastIndex(node.Class, "."); idx != -1 {
				shortName = node.Class[idx+1:]
			}
			classMatch = node.Class == className || shortName == className
		}

		// Check all conditions
		conditionsMatch := true
		for _, cond := range conditions {
			attrValue := a.getNodeAttribute(node, cond.attr)

			switch cond.op {
			case "=":
				if attrValue != cond.value {
					conditionsMatch = false
				}
			case "contains":
				if !strings.Contains(strings.ToLower(attrValue), strings.ToLower(cond.value)) {
					conditionsMatch = false
				}
			case "exists":
				if attrValue == "" {
					conditionsMatch = false
				}
			}

			if !conditionsMatch {
				break
			}
		}

		if classMatch && conditionsMatch {
			results = append(results, SearchResult{
				Node:  node,
				Path:  path,
				Depth: depth,
				Index: index,
			})
		}

		// Search children
		for i := range node.Nodes {
			childPath := fmt.Sprintf("%s/%s[%d]", path, node.Nodes[i].Class, i)
			search(&node.Nodes[i], childPath, depth+1, i)
		}
	}

	search(root, "/"+root.Class, 0, 0)
	return results
}

// getNodeAttribute returns the value of a node attribute by name
func (a *App) getNodeAttribute(node *UINode, attr string) string {
	switch strings.ToLower(attr) {
	case "text":
		return node.Text
	case "resource-id", "resourceid", "id":
		return node.ResourceID
	case "class":
		return node.Class
	case "package":
		return node.Package
	case "content-desc", "contentdesc", "description", "desc":
		return node.ContentDesc
	case "bounds":
		return node.Bounds
	case "clickable":
		return node.Clickable
	case "enabled":
		return node.Enabled
	case "focused":
		return node.Focused
	case "scrollable":
		return node.Scrollable
	case "checkable":
		return node.Checkable
	case "checked":
		return node.Checked
	case "focusable":
		return node.Focusable
	case "long-clickable", "longclickable":
		return node.LongClickable
	case "password":
		return node.Password
	case "selected":
		return node.Selected
	}
	return ""
}

// SearchElementsAdvanced searches elements using combined conditions
// Syntax: "attr:value AND attr:value OR attr:value"
// Operators: = (exact), ~ (contains), ^ (starts with), $ (ends with)
// Example: "clickable:true AND text~确定" or "class:Button OR class:ImageButton"
func (a *App) SearchElementsAdvanced(root *UINode, query string) []SearchResult {
	var results []SearchResult
	query = strings.TrimSpace(query)

	if query == "" {
		return results
	}

	// Parse OR groups first (lower precedence)
	orGroups := regexp.MustCompile(`(?i)\s+OR\s+`).Split(query, -1)

	var search func(node *UINode, path string, depth int, index int)
	search = func(node *UINode, path string, depth int, index int) {
		if node == nil {
			return
		}

		// Check if any OR group matches
		anyGroupMatch := false
		for _, orGroup := range orGroups {
			// Parse AND conditions within each OR group
			andParts := regexp.MustCompile(`(?i)\s+AND\s+`).Split(strings.TrimSpace(orGroup), -1)

			allAndMatch := true
			for _, part := range andParts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}

				if !a.evaluateCondition(node, part) {
					allAndMatch = false
					break
				}
			}

			if allAndMatch {
				anyGroupMatch = true
				break
			}
		}

		if anyGroupMatch {
			results = append(results, SearchResult{
				Node:  node,
				Path:  path,
				Depth: depth,
				Index: index,
			})
		}

		// Search children
		for i := range node.Nodes {
			childPath := fmt.Sprintf("%s/%s[%d]", path, node.Nodes[i].Class, i)
			search(&node.Nodes[i], childPath, depth+1, i)
		}
	}

	search(root, "/"+root.Class, 0, 0)
	return results
}

// evaluateCondition evaluates a single condition like "text:value" or "clickable=true"
func (a *App) evaluateCondition(node *UINode, condition string) bool {
	// Supported operators: : (contains), = (exact), ~ (contains), ^ (starts with), $ (ends with)
	var attr, op, value string

	// Try different operators in order of specificity
	for _, operator := range []string{"~", "^", "$", "=", ":"} {
		if idx := strings.Index(condition, operator); idx != -1 {
			attr = strings.TrimSpace(condition[:idx])
			op = operator
			value = strings.TrimSpace(condition[idx+len(operator):])
			break
		}
	}

	if attr == "" {
		// No operator found, treat as text contains search
		lowerCond := strings.ToLower(condition)
		return strings.Contains(strings.ToLower(node.Text), lowerCond) ||
			strings.Contains(strings.ToLower(node.ContentDesc), lowerCond) ||
			strings.Contains(strings.ToLower(node.ResourceID), lowerCond)
	}

	attrValue := a.getNodeAttribute(node, attr)
	lowerAttrValue := strings.ToLower(attrValue)
	lowerValue := strings.ToLower(value)

	switch op {
	case "=":
		return lowerAttrValue == lowerValue
	case ":", "~":
		return strings.Contains(lowerAttrValue, lowerValue)
	case "^":
		return strings.HasPrefix(lowerAttrValue, lowerValue)
	case "$":
		return strings.HasSuffix(lowerAttrValue, lowerValue)
	}

	return false
}

// SearchUIElements is the unified search API exposed to frontend
// Automatically detects query type: XPath (starts with //), Advanced (has :), or simple text
func (a *App) SearchUIElements(deviceId string, query string) ([]map[string]interface{}, error) {
	result, err := a.GetUIHierarchy(deviceId)
	if err != nil {
		return nil, err
	}

	var searchResults []SearchResult
	query = strings.TrimSpace(query)

	if strings.HasPrefix(query, "//") {
		// XPath mode
		searchResults = a.SearchElementsXPath(result.Root, query)
	} else if strings.Contains(query, ":") || strings.Contains(query, "=") ||
		regexp.MustCompile(`(?i)\s+(AND|OR)\s+`).MatchString(query) {
		// Advanced mode
		searchResults = a.SearchElementsAdvanced(result.Root, query)
	} else {
		// Simple text search (default)
		searchResults = a.SearchElementsAdvanced(result.Root, query)
	}

	// Convert to frontend-friendly format
	var output []map[string]interface{}
	for _, sr := range searchResults {
		output = append(output, map[string]interface{}{
			"text":        sr.Node.Text,
			"resourceId":  sr.Node.ResourceID,
			"class":       sr.Node.Class,
			"contentDesc": sr.Node.ContentDesc,
			"bounds":      sr.Node.Bounds,
			"clickable":   sr.Node.Clickable,
			"path":        sr.Path,
			"depth":       sr.Depth,
		})
	}

	return output, nil
}

// PerformNodeAction executes a node-based action (click, long click, swipe, keys)
func (a *App) PerformNodeAction(deviceId string, bounds string, actionType string) error {
	// Bounds format: "[x1,y1][x2,y2]"
	re := regexp.MustCompile(`\[(\d+),(\d+)\]\[(\d+),(\d+)\]`)
	matches := re.FindStringSubmatch(bounds)
	if len(matches) < 5 {
		return fmt.Errorf("invalid bounds format: %s", bounds)
	}

	x1, _ := strconv.Atoi(matches[1])
	y1, _ := strconv.Atoi(matches[2])
	x2, _ := strconv.Atoi(matches[3])
	y2, _ := strconv.Atoi(matches[4])

	centerX := (x1 + x2) / 2
	centerY := (y1 + y2) / 2
	width := x2 - x1
	height := y2 - y1

	var cmd string
	switch actionType {
	case "long_click":
		cmd = fmt.Sprintf("shell input swipe %d %d %d %d 1000", centerX, centerY, centerX, centerY)
	case "swipe_up":
		// Swipe from bottom of node to top
		cmd = fmt.Sprintf("shell input swipe %d %d %d %d 300", centerX, y2-height/10, centerX, y1+height/10)
	case "swipe_down":
		// Swipe from top of node to bottom
		cmd = fmt.Sprintf("shell input swipe %d %d %d %d 300", centerX, y1+height/10, centerX, y2-height/10)
	case "swipe_left":
		// Swipe from right of node to left
		cmd = fmt.Sprintf("shell input swipe %d %d %d %d 300", x2-width/10, centerY, x1+width/10, centerY)
	case "swipe_right":
		// Swipe from left of node to right
		cmd = fmt.Sprintf("shell input swipe %d %d %d %d 300", x1+width/10, centerY, x2-width/10, centerY)
	case "back":
		cmd = "shell input keyevent 4"
	case "home":
		cmd = "shell input keyevent 3"
	case "recent":
		cmd = "shell input keyevent 187"
	default:
		cmd = fmt.Sprintf("shell input tap %d %d", centerX, centerY)
	}

	_, err := a.RunAdbCommand(deviceId, cmd)
	return err
}

// InputNodeText taps a node to focus it and then sends text input
func (a *App) InputNodeText(deviceId string, bounds string, text string) error {
	// First click to focus
	err := a.PerformNodeAction(deviceId, bounds, "click")
	if err != nil {
		return err
	}

	// Small delay to ensure focus
	time.Sleep(200 * time.Millisecond)

	// ADB input text doesn't like spaces directly, replace with %s
	processedText := strings.ReplaceAll(text, " ", "%s")
	cmd := fmt.Sprintf("shell input text \"%s\"", processedText)
	_, err = a.RunAdbCommand(deviceId, cmd)
	return err
}
