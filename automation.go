package main

import (
	"bufio"
	"context"
	"encoding/json"
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
					if capturedCount <= 5 {
						fmt.Printf("[Automation] Captured #%d: %s\n", capturedCount, line)
					}
				}
				touchRecordMu.Unlock()
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
				currentX = int(value)
				if tracking && touchStartX == -1 {
					touchStartX = currentX
				}

			case "ABS_MT_POSITION_Y":
				currentY = int(value)
				if tracking && touchStartY == -1 {
					touchStartY = currentY
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

		startTime := time.Now()
		total := len(script.Events)

		for i, event := range script.Events {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Wait until it's time to execute this event
			elapsed := time.Since(startTime).Milliseconds()
			if event.Timestamp > elapsed {
				sleepDuration := time.Duration(event.Timestamp-elapsed) * time.Millisecond
				select {
				case <-ctx.Done():
					return
				case <-time.After(sleepDuration):
				}
			}

			// Execute the touch event
			var cmd string
			switch event.Type {
			case "tap":
				cmd = fmt.Sprintf("shell input tap %d %d", event.X, event.Y)
			case "swipe":
				cmd = fmt.Sprintf("shell input swipe %d %d %d %d %d",
					event.X, event.Y, event.X2, event.Y2, event.Duration)
			case "wait":
				time.Sleep(time.Duration(event.Duration) * time.Millisecond)
				continue
			default:
				continue
			}

			_, _ = a.RunAdbCommand(deviceId, cmd)

			// Emit progress
			wailsRuntime.EventsEmit(a.ctx, "touch-playback-progress", map[string]interface{}{
				"deviceId": deviceId,
				"current":  i + 1,
				"total":    total,
			})
		}
	}()

	wailsRuntime.EventsEmit(a.ctx, "touch-playback-started", map[string]interface{}{
		"deviceId": deviceId,
		"total":    len(script.Events),
	})

	return nil
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
