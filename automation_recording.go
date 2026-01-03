package main

import (
	"fmt"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// SubmitSelectorChoice handles user's selector choice and resumes recording
func (a *App) SubmitSelectorChoice(deviceId string, selectorType string, selectorValue string) error {
	touchRecordMu.Lock()
	defer touchRecordMu.Unlock()

	sess, exists := touchRecordData[deviceId]
	if !exists {
		return fmt.Errorf("no active recording session")
	}

	if !sess.IsPaused || sess.PendingSelectorReq == nil {
		return fmt.Errorf("no pending selector choice")
	}

	// Create/Update element info with user's specific choice
	elemInfo := sess.PendingSelectorReq.ElementInfo
	if elemInfo == nil {
		elemInfo = &ElementInfo{
			X: sess.PendingSelectorReq.X,
			Y: sess.PendingSelectorReq.Y,
		}
	}

	// Override with user's selected selector
	// This ensures that during playback, 'resolveSmartTapCoords' uses Exactly what the user picked
	if elemInfo.Selector == nil {
		elemInfo.Selector = &ElementSelector{Index: 0}
	}

	switch selectorType {
	case "text":
		elemInfo.Selector.Type = "text"
		elemInfo.Selector.Value = selectorValue
	case "id":
		elemInfo.Selector.Type = "id"
		elemInfo.Selector.Value = selectorValue
	case "desc":
		elemInfo.Selector.Type = "desc"
		elemInfo.Selector.Value = selectorValue
	case "xpath":
		elemInfo.Selector.Type = "xpath"
		elemInfo.Selector.Value = selectorValue
	case "class":
		elemInfo.Selector.Type = "class"
		elemInfo.Selector.Value = selectorValue
	case "coordinates":
		elemInfo.Selector.Type = "coordinates"
		elemInfo.Selector.Value = fmt.Sprintf("%d,%d", elemInfo.X, elemInfo.Y)
	}

	sess.ElementInfos = append(sess.ElementInfos, *elemInfo)
	fmt.Printf("[Automation] User selected selector: %+v\n", elemInfo.Selector)

	// Clear pending request and resume
	sess.IsPaused = false
	sess.PendingSelectorReq = nil

	// Clear cache before pre-capturing for the next screen
	uiHierarchyCacheMu.Lock()
	delete(uiHierarchyCache, deviceId)
	uiHierarchyCacheMu.Unlock()

	// Emit event to frontend
	wailsRuntime.EventsEmit(a.ctx, "recording-resumed", map[string]interface{}{
		"deviceId": deviceId,
	})

	// Pre-capture NEXT screen for precise mode
	if sess.RecordingMode == "precise" {
		go func() {
			// Delay to allow transitions to finish
			time.Sleep(1500 * time.Millisecond)

			wailsRuntime.EventsEmit(a.ctx, "recording-pre-capture-started", map[string]interface{}{
				"deviceId": deviceId,
			})

			fmt.Printf("[Automation] Pre-capturing UI for NEXT action (Cache cleared)\n")
			a.captureElementInfoAtPoint(deviceId, -1, -1)

			wailsRuntime.EventsEmit(a.ctx, "recording-pre-capture-finished", map[string]interface{}{
				"deviceId": deviceId,
			})
		}()
	}

	fmt.Printf("[Automation] Recording resumed\n")
	return nil
}

// GetRecordingStatus returns the current recording status including pause state
func (a *App) GetRecordingStatus(deviceId string) map[string]interface{} {
	touchRecordMu.Lock()
	defer touchRecordMu.Unlock()

	sess, exists := touchRecordData[deviceId]
	if !exists {
		return map[string]interface{}{
			"isRecording": false,
		}
	}

	result := map[string]interface{}{
		"isRecording":   true,
		"recordingMode": sess.RecordingMode,
		"isPaused":      sess.IsPaused,
		"eventCount":    len(sess.RawEvents),
	}

	if sess.PendingSelectorReq != nil {
		result["pendingSelector"] = sess.PendingSelectorReq
	}

	return result
}
