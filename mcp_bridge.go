package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"Gaze/mcp"
)

// MCPBridge bridges the main App to the MCP server
type MCPBridge struct {
	app *App
}

// NewMCPBridge creates a new MCP bridge
func NewMCPBridge(app *App) *MCPBridge {
	return &MCPBridge{app: app}
}

// Implement mcp.GazeApp interface

func (b *MCPBridge) GetDevices(forceLog bool) ([]mcp.Device, error) {
	devices, err := b.app.GetDevices(forceLog)
	if err != nil {
		return nil, err
	}
	result := make([]mcp.Device, len(devices))
	for i, d := range devices {
		result[i] = mcp.Device{
			ID:         d.ID,
			Serial:     d.Serial,
			State:      d.State,
			Model:      d.Model,
			Brand:      d.Brand,
			Type:       d.Type,
			IDs:        d.IDs,
			WifiAddr:   d.WifiAddr,
			LastActive: d.LastActive,
			IsPinned:   d.IsPinned,
		}
	}
	return result, nil
}

func (b *MCPBridge) GetDeviceInfo(deviceId string) (mcp.DeviceInfo, error) {
	info, err := b.app.GetDeviceInfo(deviceId)
	if err != nil {
		return mcp.DeviceInfo{}, err
	}
	return mcp.DeviceInfo{
		Model:        info.Model,
		Brand:        info.Brand,
		Manufacturer: info.Manufacturer,
		AndroidVer:   info.AndroidVer,
		SDK:          info.SDK,
		ABI:          info.ABI,
		Serial:       info.Serial,
		Resolution:   info.Resolution,
		Density:      info.Density,
		CPU:          info.CPU,
		Memory:       info.Memory,
		Props:        info.Props,
	}, nil
}

func (b *MCPBridge) AdbConnect(address string) (string, error) {
	return b.app.AdbConnect(address)
}

func (b *MCPBridge) AdbDisconnect(address string) (string, error) {
	return b.app.AdbDisconnect(address)
}

func (b *MCPBridge) AdbPair(address string, code string) (string, error) {
	return b.app.AdbPair(address, code)
}

func (b *MCPBridge) SwitchToWireless(deviceId string) (string, error) {
	return b.app.SwitchToWireless(deviceId)
}

func (b *MCPBridge) GetDeviceIP(deviceId string) (string, error) {
	return b.app.GetDeviceIP(deviceId)
}

func (b *MCPBridge) ListPackages(deviceId string, packageType string) ([]mcp.AppPackage, error) {
	packages, err := b.app.ListPackages(deviceId, packageType)
	if err != nil {
		return nil, err
	}
	result := make([]mcp.AppPackage, len(packages))
	for i, p := range packages {
		result[i] = mcp.AppPackage{
			Name:                 p.Name,
			Label:                p.Label,
			Icon:                 p.Icon,
			Type:                 p.Type,
			State:                p.State,
			VersionName:          p.VersionName,
			VersionCode:          p.VersionCode,
			MinSdkVersion:        p.MinSdkVersion,
			TargetSdkVersion:     p.TargetSdkVersion,
			Permissions:          p.Permissions,
			Activities:           p.Activities,
			LaunchableActivities: p.LaunchableActivities,
		}
	}
	return result, nil
}

func (b *MCPBridge) GetAppInfo(deviceId, packageName string, force bool) (mcp.AppPackage, error) {
	info, err := b.app.GetAppInfo(deviceId, packageName, force)
	if err != nil {
		return mcp.AppPackage{}, err
	}
	return mcp.AppPackage{
		Name:                 info.Name,
		Label:                info.Label,
		Icon:                 info.Icon,
		Type:                 info.Type,
		State:                info.State,
		VersionName:          info.VersionName,
		VersionCode:          info.VersionCode,
		MinSdkVersion:        info.MinSdkVersion,
		TargetSdkVersion:     info.TargetSdkVersion,
		Permissions:          info.Permissions,
		Activities:           info.Activities,
		LaunchableActivities: info.LaunchableActivities,
	}, nil
}

func (b *MCPBridge) StartApp(deviceId, packageName string) (string, error) {
	return b.app.StartApp(deviceId, packageName)
}

func (b *MCPBridge) ForceStopApp(deviceId, packageName string) (string, error) {
	return b.app.ForceStopApp(deviceId, packageName)
}

func (b *MCPBridge) InstallAPK(deviceId string, path string) (string, error) {
	return b.app.InstallAPK(deviceId, path)
}

func (b *MCPBridge) UninstallApp(deviceId, packageName string) (string, error) {
	return b.app.UninstallApp(deviceId, packageName)
}

func (b *MCPBridge) ClearAppData(deviceId, packageName string) (string, error) {
	return b.app.ClearAppData(deviceId, packageName)
}

func (b *MCPBridge) IsAppRunning(deviceId, packageName string) (bool, error) {
	return b.app.IsAppRunning(deviceId, packageName)
}

func (b *MCPBridge) TakeScreenshot(deviceId, savePath string) (string, error) {
	return b.app.TakeScreenshot(deviceId, savePath)
}

func (b *MCPBridge) StartRecording(deviceId string, config mcp.ScrcpyConfig) error {
	return b.app.StartRecording(deviceId, ScrcpyConfig{
		MaxSize:     config.MaxSize,
		BitRate:     config.BitRate,
		MaxFps:      config.MaxFps,
		ShowTouches: config.ShowTouches,
	})
}

func (b *MCPBridge) StopRecording(deviceId string) error {
	return b.app.StopRecording(deviceId)
}

func (b *MCPBridge) IsRecording(deviceId string) bool {
	return b.app.IsRecording(deviceId)
}

func (b *MCPBridge) GetUIHierarchy(deviceId string) (*mcp.UIHierarchyResult, error) {
	result, err := b.app.GetUIHierarchy(deviceId)
	if err != nil {
		return nil, err
	}
	return &mcp.UIHierarchyResult{
		Root:   result.Root,
		RawXML: result.RawXML,
	}, nil
}

func (b *MCPBridge) SearchUIElements(deviceId string, query string) ([]map[string]interface{}, error) {
	return b.app.SearchUIElements(deviceId, query)
}

func (b *MCPBridge) PerformNodeAction(deviceId string, bounds string, actionType string) error {
	return b.app.PerformNodeAction(deviceId, bounds, actionType)
}

// normalizeSelectorType maps MCP-facing selector type names to the names
// understood by FindElementBySelector (selector.go)
func normalizeSelectorType(selectorType string) string {
	switch selectorType {
	case "id", "resourceId", "resource-id":
		return "id"
	case "contentDesc", "desc", "description", "content-desc":
		return "desc"
	case "className", "class":
		return "class"
	default:
		// "text", "contains", "xpath", "advanced" pass through unchanged
		return selectorType
	}
}

// WaitForUIElement waits for a UI element to appear or disappear (MCP ui_wait_for tool).
// Returns a map with: satisfied (bool), elapsedMs (int64), element (map, only when
// condition=appear succeeds) and message (string, only on timeout).
// A timeout is reported via satisfied=false, not as an error.
func (b *MCPBridge) WaitForUIElement(deviceId, selectorType, selectorValue, condition string, timeoutMs, intervalMs int) (map[string]interface{}, error) {
	if timeoutMs <= 0 {
		timeoutMs = 10000
	}
	if intervalMs <= 0 {
		intervalMs = 1000
	}
	selector := &ElementSelector{Type: normalizeSelectorType(selectorType), Value: selectorValue}
	start := time.Now()

	if condition == "gone" {
		// Same polling logic as WaitElementGone (element_service.go), but honoring intervalMs
		deadline := start.Add(time.Duration(timeoutMs) * time.Millisecond)
		for {
			hierarchy, err := b.app.GetUIHierarchy(deviceId)
			if err == nil && b.app.FindElementBySelector(hierarchy.Root, selector) == nil {
				return map[string]interface{}{
					"satisfied": true,
					"elapsedMs": time.Since(start).Milliseconds(),
				}, nil
			}
			if time.Now().After(deadline) {
				return map[string]interface{}{
					"satisfied": false,
					"elapsedMs": time.Since(start).Milliseconds(),
					"message":   fmt.Sprintf("element still present after %dms", timeoutMs),
				}, nil
			}
			time.Sleep(time.Duration(intervalMs) * time.Millisecond)
		}
	}

	// condition == "appear" (default)
	node, err := b.app.waitForElement(context.Background(), deviceId, selector, timeoutMs, intervalMs)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return map[string]interface{}{
			"satisfied": false,
			"elapsedMs": elapsed,
			"message":   err.Error(),
		}, nil
	}

	element := map[string]interface{}{
		"bounds":      node.Bounds,
		"text":        node.Text,
		"resourceId":  node.ResourceID,
		"class":       node.Class,
		"contentDesc": node.ContentDesc,
		"clickable":   node.Clickable,
	}
	if bounds, berr := ParseBounds(node.Bounds); berr == nil {
		x, y := bounds.Center()
		element["centerX"] = x
		element["centerY"] = y
	}
	return map[string]interface{}{
		"satisfied": true,
		"elapsedMs": elapsed,
		"element":   element,
	}, nil
}

func (b *MCPBridge) GetDeviceResolution(deviceId string) (string, error) {
	return b.app.GetDeviceResolution(deviceId)
}

func (b *MCPBridge) InputText(deviceId string, text string) error {
	return b.app.InputText(deviceId, text)
}

func (b *MCPBridge) EnsureADBKeyboard(deviceId string) (bool, bool, error) {
	return b.app.EnsureADBKeyboard(deviceId)
}

func (b *MCPBridge) IsADBKeyboardInstalled(deviceId string) bool {
	return b.app.IsADBKeyboardInstalled(deviceId)
}

func (b *MCPBridge) CreateSession(deviceId, sessionType, name string) string {
	// Use StartNewSession which persists to EventStore (database)
	return b.app.StartNewSession(deviceId, sessionType, name)
}

func (b *MCPBridge) StartSessionWithConfig(deviceId, name string, config mcp.MCPSessionConfig) string {
	// Convert MCP config to internal SessionConfig
	sessionConfig := SessionConfig{
		Logcat: LogcatConfig{
			Enabled:       config.LogcatEnabled,
			PackageName:   config.LogcatPackageName,
			PreFilter:     config.LogcatPreFilter,
			ExcludeFilter: config.LogcatExcludeFilter,
		},
		Recording: RecordingConfig{
			Enabled: config.RecordingEnabled,
			Quality: config.RecordingQuality,
		},
		Proxy: ProxyConfig{
			Enabled:     config.ProxyEnabled,
			Port:        config.ProxyPort,
			MitmEnabled: config.ProxyMitmEnabled,
		},
		Monitor: MonitorConfig{
			Enabled: config.MonitorEnabled,
		},
	}
	return b.app.StartSessionWithConfig(deviceId, name, sessionConfig)
}

func (b *MCPBridge) EndSession(sessionId string, status string) error {
	// Use EndActiveSession which properly closes via EventPipeline
	b.app.EndActiveSession(sessionId, status)
	return nil
}

func (b *MCPBridge) GetActiveSession(deviceId string) string {
	// Use GetDeviceActiveSession which reads from EventPipeline
	session := b.app.GetDeviceActiveSession(deviceId)
	if session != nil {
		return session.ID
	}
	return ""
}

func (b *MCPBridge) ListStoredSessions(deviceID string, limit int) ([]mcp.DeviceSession, error) {
	sessions, err := b.app.ListStoredSessions(deviceID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]mcp.DeviceSession, len(sessions))
	for i, s := range sessions {
		result[i] = mcp.DeviceSession{
			ID:            s.ID,
			DeviceID:      s.DeviceID,
			Name:          s.Name,
			Type:          s.Type,
			Status:        s.Status,
			StartTime:     s.StartTime,
			EndTime:       s.EndTime,
			EventCount:    s.EventCount,
			VideoPath:     s.VideoPath,
			VideoDuration: s.VideoDuration,
		}
	}
	return result, nil
}

func (b *MCPBridge) QuerySessionEvents(query mcp.EventQuery) (*mcp.EventQueryResult, error) {
	// Convert string arrays to typed arrays
	sources := make([]EventSource, len(query.Sources))
	for i, s := range query.Sources {
		sources[i] = EventSource(s)
	}
	levels := make([]EventLevel, len(query.Levels))
	for i, l := range query.Levels {
		levels[i] = EventLevel(l)
	}

	result, err := b.app.QuerySessionEvents(EventQuery{
		SessionID:   query.SessionID,
		DeviceID:    query.DeviceID,
		Types:       query.Types,
		Sources:     sources,
		Levels:      levels,
		StartTime:   query.StartTime,
		EndTime:     query.EndTime,
		SearchText:  query.SearchText,
		Limit:       query.Limit,
		Offset:      query.Offset,
		IncludeData: true, // MCP needs full event data including coordinates
	})
	if err != nil {
		return nil, err
	}
	events := make([]interface{}, len(result.Events))
	for i, e := range result.Events {
		// Convert UnifiedEvent to map for MCP serialization
		eventMap := map[string]interface{}{
			"id":           e.ID,
			"sessionId":    e.SessionID,
			"deviceId":     e.DeviceID,
			"timestamp":    e.Timestamp,
			"relativeTime": e.RelativeTime,
			"source":       string(e.Source),
			"category":     string(e.Category),
			"type":         e.Type,
			"level":        string(e.Level),
			"title":        e.Title,
			"summary":      e.Summary,
		}
		// Parse data JSON if present
		if len(e.Data) > 0 {
			var data map[string]interface{}
			if err := json.Unmarshal(e.Data, &data); err == nil {
				eventMap["data"] = data
			}
		}
		if e.Duration > 0 {
			eventMap["duration"] = e.Duration
		}
		events[i] = eventMap
	}
	return &mcp.EventQueryResult{
		Events:  events,
		Total:   result.Total,
		HasMore: result.HasMore,
	}, nil
}

func (b *MCPBridge) GetSessionStats(sessionID string) (map[string]interface{}, error) {
	return b.app.GetSessionStats(sessionID)
}

func (b *MCPBridge) LoadWorkflows() ([]mcp.Workflow, error) {
	workflows, err := b.app.LoadWorkflows()
	if err != nil {
		return nil, err
	}
	result := make([]mcp.Workflow, len(workflows))
	for i, wf := range workflows {
		converted := b.convertWorkflowToMCP(&wf)
		result[i] = *converted
	}
	return result, nil
}

func (b *MCPBridge) RunWorkflow(device mcp.Device, workflow mcp.Workflow) error {
	// Convert back to main types
	mainDevice := Device{
		ID:         device.ID,
		Serial:     device.Serial,
		State:      device.State,
		Model:      device.Model,
		Brand:      device.Brand,
		Type:       device.Type,
		IDs:        device.IDs,
		WifiAddr:   device.WifiAddr,
		LastActive: device.LastActive,
		IsPinned:   device.IsPinned,
	}
	mainWorkflow := b.convertWorkflowFromMCP(workflow)
	return b.app.RunWorkflow(mainDevice, mainWorkflow)
}

func (b *MCPBridge) StopWorkflow(device mcp.Device) {
	mainDevice := Device{
		ID:         device.ID,
		Serial:     device.Serial,
		State:      device.State,
		Model:      device.Model,
		Brand:      device.Brand,
		Type:       device.Type,
		IDs:        device.IDs,
		WifiAddr:   device.WifiAddr,
		LastActive: device.LastActive,
		IsPinned:   device.IsPinned,
	}
	b.app.StopWorkflow(mainDevice)
}

func (b *MCPBridge) PauseTask(deviceId string) {
	b.app.PauseTask(deviceId)
}

func (b *MCPBridge) ResumeTask(deviceId string) {
	b.app.ResumeTask(deviceId)
}

func (b *MCPBridge) GetWorkflow(workflowID string) (*mcp.Workflow, error) {
	wf, err := b.app.GetWorkflow(workflowID)
	if err != nil {
		return nil, err
	}
	return b.convertWorkflowToMCP(wf), nil
}

func (b *MCPBridge) SaveWorkflow(workflow mcp.Workflow) error {
	mainWorkflow := b.convertWorkflowFromMCP(workflow)
	return b.app.SaveWorkflow(mainWorkflow)
}

func (b *MCPBridge) DeleteWorkflow(id string) error {
	return b.app.DeleteWorkflow(id)
}

func (b *MCPBridge) ExecuteSingleWorkflowStep(deviceId string, step mcp.WorkflowStep) error {
	mainStep := b.convertStepFromMCP(step)
	return b.app.ExecuteSingleWorkflowStep(deviceId, mainStep)
}

func (b *MCPBridge) IsWorkflowRunning(deviceId string) bool {
	return b.app.IsPlayingTouch(deviceId)
}

func (b *MCPBridge) GetWorkflowExecutionResult(deviceId string) *mcp.WorkflowExecutionResult {
	result := b.app.GetWorkflowExecutionResult(deviceId)
	if result == nil {
		return nil
	}
	return &mcp.WorkflowExecutionResult{
		WorkflowID:      result.WorkflowID,
		WorkflowName:    result.WorkflowName,
		Status:          result.Status,
		Error:           result.Error,
		StartTime:       result.StartTime,
		EndTime:         result.EndTime,
		Duration:        result.Duration,
		StepsTotal:      result.StepsTotal,
		CurrentStepID:   result.CurrentStepID,
		CurrentStepName: result.CurrentStepName,
		CurrentStepType: result.CurrentStepType,
		Variables:       result.Variables,
		StepsExecuted:   result.StepsExecuted,
		IsPaused:        result.IsPaused,
	}
}

func (b *MCPBridge) StepNextWorkflow(deviceId string) (*mcp.WorkflowExecutionResult, error) {
	result, err := b.app.StepNextWorkflow(deviceId)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &mcp.WorkflowExecutionResult{
		WorkflowID:      result.WorkflowID,
		WorkflowName:    result.WorkflowName,
		Status:          result.Status,
		Error:           result.Error,
		StartTime:       result.StartTime,
		EndTime:         result.EndTime,
		Duration:        result.Duration,
		StepsTotal:      result.StepsTotal,
		CurrentStepID:   result.CurrentStepID,
		CurrentStepName: result.CurrentStepName,
		CurrentStepType: result.CurrentStepType,
		Variables:       result.Variables,
		StepsExecuted:   result.StepsExecuted,
		IsPaused:        result.IsPaused,
	}, nil
}

// Helper functions for workflow type conversion
// Both packages now use the same nested structure format

func (b *MCPBridge) convertWorkflowToMCP(wf *Workflow) *mcp.Workflow {
	if wf == nil {
		return nil
	}
	steps := make([]mcp.WorkflowStep, len(wf.Steps))
	for i, s := range wf.Steps {
		steps[i] = b.convertStepToMCP(s)
	}
	return &mcp.Workflow{
		ID:          wf.ID,
		Name:        wf.Name,
		Description: wf.Description,
		Version:     2,
		Steps:       steps,
		Variables:   wf.Variables,
		CreatedAt:   wf.CreatedAt,
		UpdatedAt:   wf.UpdatedAt,
	}
}

func (b *MCPBridge) convertStepToMCP(s WorkflowStep) mcp.WorkflowStep {
	result := mcp.WorkflowStep{
		ID:   s.ID,
		Type: s.Type,
		Name: s.Name,
	}

	// Convert Common (value to value)
	result.Common = mcp.StepCommon{
		Timeout:   s.Common.Timeout,
		OnError:   s.Common.OnError,
		Loop:      s.Common.Loop,
		PostDelay: s.Common.PostDelay,
		PreWait:   s.Common.PreWait,
	}

	// Convert Connections (value to value)
	result.Connections = mcp.StepConnections{
		SuccessStepId: s.Connections.SuccessStepId,
		ErrorStepId:   s.Connections.ErrorStepId,
		TrueStepId:    s.Connections.TrueStepId,
		FalseStepId:   s.Connections.FalseStepId,
	}

	// Convert Layout (value to value)
	result.Layout = mcp.StepLayout{
		PosX: s.Layout.PosX,
		PosY: s.Layout.PosY,
	}
	if s.Layout.Handles != nil {
		result.Layout.Handles = make(map[string]mcp.HandleInfo)
		for k, v := range s.Layout.Handles {
			result.Layout.Handles[k] = mcp.HandleInfo{
				SourceHandle: v.SourceHandle,
				TargetHandle: v.TargetHandle,
			}
		}
	}

	// Copy type-specific params (pointer to pointer)
	if s.Tap != nil {
		result.Tap = &mcp.TapParams{X: s.Tap.X, Y: s.Tap.Y}
	}
	if s.Swipe != nil {
		result.Swipe = &mcp.SwipeParams{
			X: s.Swipe.X, Y: s.Swipe.Y, X2: s.Swipe.X2, Y2: s.Swipe.Y2,
			Direction: s.Swipe.Direction, Distance: s.Swipe.Distance, Duration: s.Swipe.Duration,
		}
	}
	if s.Element != nil {
		result.Element = &mcp.ElementParams{
			Selector:      mcp.ElementSelector{Type: s.Element.Selector.Type, Value: s.Element.Selector.Value, Index: s.Element.Selector.Index},
			Action:        s.Element.Action,
			InputText:     s.Element.InputText,
			SwipeDir:      s.Element.SwipeDir,
			SwipeDistance: s.Element.SwipeDistance,
			SwipeDuration: s.Element.SwipeDuration,
		}
	}
	if s.App != nil {
		result.App = &mcp.AppParams{PackageName: s.App.PackageName, Action: s.App.Action}
	}
	if s.Branch != nil {
		result.Branch = &mcp.BranchParams{
			Condition:     s.Branch.Condition,
			ExpectedValue: s.Branch.ExpectedValue,
			VariableName:  s.Branch.VariableName,
		}
		if s.Branch.Selector != nil {
			result.Branch.Selector = &mcp.ElementSelector{
				Type: s.Branch.Selector.Type, Value: s.Branch.Selector.Value, Index: s.Branch.Selector.Index,
			}
		}
	}
	if s.Wait != nil {
		result.Wait = &mcp.WaitParams{DurationMs: s.Wait.DurationMs}
	}
	if s.Script != nil {
		result.Script = &mcp.ScriptParams{ScriptName: s.Script.ScriptName}
	}
	if s.Variable != nil {
		result.Variable = &mcp.VariableParams{Name: s.Variable.Name, Value: s.Variable.Value}
	}
	if s.ADB != nil {
		result.ADB = &mcp.ADBParams{Command: s.ADB.Command}
	}
	if s.Workflow != nil {
		result.Workflow = &mcp.SubWorkflowParams{WorkflowId: s.Workflow.WorkflowId}
	}
	if s.ReadToVariable != nil {
		result.ReadToVariable = &mcp.ReadToVariableParams{
			Selector:     mcp.ElementSelector{Type: s.ReadToVariable.Selector.Type, Value: s.ReadToVariable.Selector.Value, Index: s.ReadToVariable.Selector.Index},
			VariableName: s.ReadToVariable.VariableName,
			Attribute:    s.ReadToVariable.Attribute,
			Regex:        s.ReadToVariable.Regex,
			DefaultValue: s.ReadToVariable.DefaultValue,
		}
	}

	return result
}

func (b *MCPBridge) convertWorkflowFromMCP(wf mcp.Workflow) Workflow {
	steps := make([]WorkflowStep, len(wf.Steps))
	for i, s := range wf.Steps {
		steps[i] = b.convertStepFromMCP(s)
	}
	return Workflow{
		ID:          wf.ID,
		Name:        wf.Name,
		Description: wf.Description,
		Steps:       steps,
		Variables:   wf.Variables,
		CreatedAt:   wf.CreatedAt,
		UpdatedAt:   wf.UpdatedAt,
	}
}

func (b *MCPBridge) convertStepFromMCP(s mcp.WorkflowStep) WorkflowStep {
	result := WorkflowStep{
		ID:   s.ID,
		Type: s.Type,
		Name: s.Name,
	}

	// Convert Common (value to value)
	result.Common = StepCommon{
		Timeout:   s.Common.Timeout,
		OnError:   s.Common.OnError,
		Loop:      s.Common.Loop,
		PostDelay: s.Common.PostDelay,
		PreWait:   s.Common.PreWait,
	}

	// Convert Connections (value to value)
	result.Connections = StepConnections{
		SuccessStepId: s.Connections.SuccessStepId,
		ErrorStepId:   s.Connections.ErrorStepId,
		TrueStepId:    s.Connections.TrueStepId,
		FalseStepId:   s.Connections.FalseStepId,
	}

	// Convert Layout (value to value)
	result.Layout = StepLayout{
		PosX: s.Layout.PosX,
		PosY: s.Layout.PosY,
	}
	if s.Layout.Handles != nil {
		result.Layout.Handles = make(map[string]HandleInfo)
		for k, v := range s.Layout.Handles {
			result.Layout.Handles[k] = HandleInfo{
				SourceHandle: v.SourceHandle,
				TargetHandle: v.TargetHandle,
			}
		}
	}

	// Copy type-specific params (pointer to pointer)
	if s.Tap != nil {
		result.Tap = &TapParams{X: s.Tap.X, Y: s.Tap.Y}
	}
	if s.Swipe != nil {
		result.Swipe = &SwipeParams{
			X: s.Swipe.X, Y: s.Swipe.Y, X2: s.Swipe.X2, Y2: s.Swipe.Y2,
			Direction: s.Swipe.Direction, Distance: s.Swipe.Distance, Duration: s.Swipe.Duration,
		}
	}
	if s.Element != nil {
		result.Element = &ElementParams{
			Selector:      ElementSelector{Type: s.Element.Selector.Type, Value: s.Element.Selector.Value, Index: s.Element.Selector.Index},
			Action:        s.Element.Action,
			InputText:     s.Element.InputText,
			SwipeDir:      s.Element.SwipeDir,
			SwipeDistance: s.Element.SwipeDistance,
			SwipeDuration: s.Element.SwipeDuration,
		}
	}
	if s.App != nil {
		result.App = &AppParams{PackageName: s.App.PackageName, Action: s.App.Action}
	}
	if s.Branch != nil {
		result.Branch = &BranchParams{
			Condition:     s.Branch.Condition,
			ExpectedValue: s.Branch.ExpectedValue,
			VariableName:  s.Branch.VariableName,
		}
		if s.Branch.Selector != nil {
			result.Branch.Selector = &ElementSelector{
				Type: s.Branch.Selector.Type, Value: s.Branch.Selector.Value, Index: s.Branch.Selector.Index,
			}
		}
	}
	if s.Wait != nil {
		result.Wait = &WaitParams{DurationMs: s.Wait.DurationMs}
	}
	if s.Script != nil {
		result.Script = &ScriptParams{ScriptName: s.Script.ScriptName}
	}
	if s.Variable != nil {
		result.Variable = &VariableParams{Name: s.Variable.Name, Value: s.Variable.Value}
	}
	if s.ADB != nil {
		result.ADB = &ADBParams{Command: s.ADB.Command}
	}
	if s.Workflow != nil {
		result.Workflow = &SubWorkflowParams{WorkflowId: s.Workflow.WorkflowId}
	}
	if s.ReadToVariable != nil {
		result.ReadToVariable = &ReadToVariableParams{
			Selector:     ElementSelector{Type: s.ReadToVariable.Selector.Type, Value: s.ReadToVariable.Selector.Value, Index: s.ReadToVariable.Selector.Index},
			VariableName: s.ReadToVariable.VariableName,
			Attribute:    s.ReadToVariable.Attribute,
			Regex:        s.ReadToVariable.Regex,
			DefaultValue: s.ReadToVariable.DefaultValue,
		}
	}

	return result
}

func (b *MCPBridge) StartProxy(port int) (string, error) {
	return b.app.StartProxy(port)
}

func (b *MCPBridge) StopProxy() (string, error) {
	return b.app.StopProxy()
}

func (b *MCPBridge) GetProxyStatus() bool {
	return b.app.GetProxyStatus()
}

func (b *MCPBridge) SetProxyDevice(deviceId string) {
	b.app.SetProxyDevice(deviceId)
}

func (b *MCPBridge) GetProxyDevice() string {
	return b.app.GetProxyDevice()
}

func (b *MCPBridge) SetupProxyForDevice(deviceId string, port int) error {
	return b.app.SetupProxyForDevice(deviceId, port)
}

func (b *MCPBridge) CleanupProxyForDevice(deviceId string, port int) error {
	return b.app.CleanupProxyForDevice(deviceId, port)
}

func (b *MCPBridge) SetProxyMITM(enabled bool) {
	b.app.SetProxyMITM(enabled)
}

func (b *MCPBridge) SetProxyWSEnabled(enabled bool) {
	b.app.SetProxyWSEnabled(enabled)
}

func (b *MCPBridge) SetProxyLimit(uploadSpeed, downloadSpeed int) {
	b.app.SetProxyLimit(uploadSpeed, downloadSpeed)
}

func (b *MCPBridge) SetProxyLatency(latencyMs int) {
	b.app.SetProxyLatency(latencyMs)
}

func (b *MCPBridge) SetMITMBypassPatterns(patterns []string) {
	b.app.SetMITMBypassPatterns(patterns)
}

func (b *MCPBridge) GetMITMBypassPatterns() []string {
	return b.app.GetMITMBypassPatterns()
}

func (b *MCPBridge) GetProxySettings() map[string]interface{} {
	return b.app.GetProxySettings()
}

func (b *MCPBridge) InstallProxyCert(deviceId string) (string, error) {
	return b.app.InstallProxyCert(deviceId)
}

func (b *MCPBridge) CheckCertTrust(deviceId string) string {
	return b.app.CheckCertTrust(deviceId)
}

func (b *MCPBridge) GetAppVersion() string {
	return b.app.GetAppVersion()
}

func (b *MCPBridge) GetVideoFrame(videoPath string, timeMs int64, width int) (string, error) {
	return b.app.GetVideoFrame(videoPath, timeMs, width)
}

func (b *MCPBridge) GetVideoMetadata(videoPath string) (*mcp.VideoMetadata, error) {
	meta, err := b.app.GetVideoMetadata(videoPath)
	if err != nil {
		return nil, err
	}
	return &mcp.VideoMetadata{
		Path:        meta.Path,
		Duration:    meta.Duration,
		DurationMs:  meta.DurationMs,
		Width:       meta.Width,
		Height:      meta.Height,
		FrameRate:   meta.FrameRate,
		Codec:       meta.Codec,
		BitRate:     meta.BitRate,
		TotalFrames: meta.TotalFrames,
	}, nil
}

func (b *MCPBridge) GetSessionVideoInfo(sessionID string) (map[string]interface{}, error) {
	return b.app.GetSessionVideoInfo(sessionID)
}

// RunAdbCommand executes an arbitrary ADB command on a device
func (b *MCPBridge) RunAdbCommand(deviceId string, command string) (string, error) {
	return b.app.RunAdbCommand(deviceId, command)
}

// RunAdbCommandWithTimeout executes an arbitrary ADB command with a caller-specified timeout
func (b *MCPBridge) RunAdbCommandWithTimeout(deviceId string, command string, timeoutSec int) (string, error) {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()
	return b.app.RunAdbCommandWithContext(ctx, deviceId, command)
}

// RunAaptCommand executes an aapt command
func (b *MCPBridge) RunAaptCommand(command string, timeoutSec int) (string, error) {
	return b.app.RunAaptCommand(command, timeoutSec)
}

// RunFfmpegCommand executes an ffmpeg command
func (b *MCPBridge) RunFfmpegCommand(command string, timeoutSec int) (string, error) {
	return b.app.RunFfmpegCommand(command, timeoutSec)
}

// RunFfprobeCommand executes an ffprobe command
func (b *MCPBridge) RunFfprobeCommand(command string, timeoutSec int) (string, error) {
	return b.app.RunFfprobeCommand(command, timeoutSec)
}

// UploadFile uploads a file from host to device
func (b *MCPBridge) UploadFile(deviceId, localPath, remotePath string) error {
	return b.app.UploadFile(deviceId, localPath, remotePath)
}

// ListFiles lists files in a directory on the device
func (b *MCPBridge) ListFiles(deviceId, pathStr string) ([]map[string]interface{}, error) {
	files, err := b.app.ListFiles(deviceId, pathStr)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, len(files))
	for i, f := range files {
		result[i] = map[string]interface{}{
			"name":    f.Name,
			"path":    f.Path,
			"size":    f.Size,
			"isDir":   f.IsDir,
			"modTime": f.ModTime,
		}
	}
	return result, nil
}

// ExportSessionToPath exports a session to a file path
func (b *MCPBridge) ExportSessionToPath(sessionID, outputPath string) (string, error) {
	return b.app.ExportSessionToPath(sessionID, outputPath)
}

// ImportSessionFromPath imports a session from a file path
func (b *MCPBridge) ImportSessionFromPath(inputPath string) (string, error) {
	return b.app.ImportSessionFromPath(inputPath)
}

// Performance Monitoring bridge methods

func (b *MCPBridge) StartPerfMonitor(deviceId string, config mcp.PerfMonitorConfig) string {
	return b.app.StartPerfMonitor(deviceId, PerfMonitorConfig{
		PackageName:   config.PackageName,
		IntervalMs:    config.IntervalMs,
		EnableCPU:     config.EnableCPU,
		EnableMemory:  config.EnableMemory,
		EnableFPS:     config.EnableFPS,
		EnableNetwork: config.EnableNetwork,
		EnableBattery: config.EnableBattery,
	})
}

func (b *MCPBridge) StopPerfMonitor(deviceId string) string {
	return b.app.StopPerfMonitor(deviceId)
}

func (b *MCPBridge) IsPerfMonitorRunning(deviceId string) bool {
	return b.app.IsPerfMonitorRunning(deviceId)
}

func (b *MCPBridge) GetPerfSnapshot(deviceId string, packageName string) (*mcp.PerfSampleData, error) {
	sample, err := b.app.GetPerfSnapshot(deviceId, packageName)
	if err != nil {
		return nil, err
	}
	result := &mcp.PerfSampleData{
		CPUUsage:     sample.CPUUsage,
		CPUApp:       sample.CPUApp,
		CPUCores:     sample.CPUCores,
		CPUFreqMHz:   sample.CPUFreqMHz,
		CPUTempC:     sample.CPUTempC,
		MemTotalMB:   sample.MemTotalMB,
		MemUsedMB:    sample.MemUsedMB,
		MemFreeMB:    sample.MemFreeMB,
		MemUsage:     sample.MemUsage,
		MemAppMB:     sample.MemAppMB,
		FPS:          sample.FPS,
		JankCount:    sample.JankCount,
		NetRxKBps:    sample.NetRxKBps,
		NetTxKBps:    sample.NetTxKBps,
		NetRxTotalMB: sample.NetRxTotalMB,
		NetTxTotalMB: sample.NetTxTotalMB,
		BatteryLevel: sample.BatteryLevel,
		BatteryTemp:  sample.BatteryTemp,
		PackageName:  sample.PackageName,
	}
	// Convert process list
	for _, p := range sample.Processes {
		result.Processes = append(result.Processes, mcp.ProcessPerfData{
			PID:       p.PID,
			Name:      p.Name,
			CPU:       p.CPU,
			MemoryKB:  p.MemoryKB,
			User:      p.User,
			Kernel:    p.Kernel,
			LinuxUser: p.LinuxUser,
			PPID:      p.PPID,
			VSZKB:     p.VSZKB,
			State:     p.State,
		})
	}
	return result, nil
}

func (b *MCPBridge) GetProcessDetail(deviceId string, pid int) (*mcp.ProcessDetail, error) {
	detail, err := b.app.GetProcessDetail(deviceId, pid)
	if err != nil {
		return nil, err
	}
	result := &mcp.ProcessDetail{
		PID:               detail.PID,
		PackageName:       detail.PackageName,
		TotalPSSKB:        detail.TotalPSSKB,
		TotalRSSKB:        detail.TotalRSSKB,
		SwapPSSKB:         detail.SwapPSSKB,
		JavaHeapSizeKB:    detail.JavaHeapSizeKB,
		JavaHeapAllocKB:   detail.JavaHeapAllocKB,
		JavaHeapFreeKB:    detail.JavaHeapFreeKB,
		NativeHeapSizeKB:  detail.NativeHeapSizeKB,
		NativeHeapAllocKB: detail.NativeHeapAllocKB,
		NativeHeapFreeKB:  detail.NativeHeapFreeKB,
		Threads:           detail.Threads,
		FDSize:            detail.FDSize,
		VmSwapKB:          detail.VmSwapKB,
		OomScoreAdj:       detail.OomScoreAdj,
		UID:               detail.UID,
		Objects: mcp.ProcessObjects{
			Views:           detail.Objects.Views,
			ViewRootImpl:    detail.Objects.ViewRootImpl,
			AppContexts:     detail.Objects.AppContexts,
			Activities:      detail.Objects.Activities,
			Assets:          detail.Objects.Assets,
			AssetManagers:   detail.Objects.AssetManagers,
			LocalBinders:    detail.Objects.LocalBinders,
			ProxyBinders:    detail.Objects.ProxyBinders,
			DeathRecipients: detail.Objects.DeathRecipients,
			WebViews:        detail.Objects.WebViews,
		},
	}
	for _, m := range detail.Memory {
		result.Memory = append(result.Memory, mcp.ProcessMemoryCategory{
			Name:  m.Name,
			PssKB: m.PssKB,
			RssKB: m.RssKB,
		})
	}
	return result, nil
}

// === Protobuf Management ===

func (b *MCPBridge) AddProtoFile(name, content string) (string, error) {
	return b.app.AddProtoFile(name, content)
}

func (b *MCPBridge) UpdateProtoFile(id, name, content string) error {
	return b.app.UpdateProtoFile(id, name, content)
}

func (b *MCPBridge) RemoveProtoFile(id string) error {
	return b.app.RemoveProtoFile(id)
}

func (b *MCPBridge) GetProtoFiles() []mcp.MCPProtoFile {
	files := b.app.GetProtoFiles()
	result := make([]mcp.MCPProtoFile, len(files))
	for i, f := range files {
		result[i] = mcp.MCPProtoFile{
			ID:       f.ID,
			Name:     f.Name,
			Content:  f.Content,
			LoadedAt: f.LoadedAt,
		}
	}
	return result
}

func (b *MCPBridge) AddProtoMapping(urlPattern, messageType, direction, description string) (string, error) {
	return b.app.AddProtoMapping(urlPattern, messageType, direction, description)
}

func (b *MCPBridge) UpdateProtoMapping(id, urlPattern, messageType, direction, description string) error {
	return b.app.UpdateProtoMapping(id, urlPattern, messageType, direction, description)
}

func (b *MCPBridge) RemoveProtoMapping(id string) error {
	return b.app.RemoveProtoMapping(id)
}

func (b *MCPBridge) GetProtoMappings() []mcp.MCPProtoMapping {
	mappings := b.app.GetProtoMappings()
	result := make([]mcp.MCPProtoMapping, len(mappings))
	for i, m := range mappings {
		result[i] = mcp.MCPProtoMapping{
			ID:          m.ID,
			URLPattern:  m.URLPattern,
			MessageType: m.MessageType,
			Direction:   m.Direction,
			Description: m.Description,
		}
	}
	return result
}

func (b *MCPBridge) GetProtoMessageTypes() []string {
	return b.app.GetProtoMessageTypes()
}

func (b *MCPBridge) LoadProtoFromURL(rawURL string) ([]string, error) {
	return b.app.LoadProtoFromURL(rawURL)
}

// === Mock Rules ===

func (b *MCPBridge) AddMockRule(urlPattern, method string, statusCode int, headers map[string]string, body, bodyFile string, delay int, description string, conditions []mcp.MCPMockCondition) string {
	rule := MockRule{
		URLPattern:  urlPattern,
		Method:      method,
		StatusCode:  statusCode,
		Headers:     headers,
		Body:        body,
		BodyFile:    bodyFile,
		Delay:       delay,
		Description: description,
		Conditions:  fromMCPConditions(conditions),
	}
	return b.app.AddMockRule(rule)
}

func (b *MCPBridge) UpdateMockRule(id, urlPattern, method string, statusCode int, headers map[string]string, body, bodyFile string, delay int, enabled bool, description string, conditions []mcp.MCPMockCondition) error {
	rule := MockRule{
		ID:          id,
		URLPattern:  urlPattern,
		Method:      method,
		StatusCode:  statusCode,
		Headers:     headers,
		Body:        body,
		BodyFile:    bodyFile,
		Delay:       delay,
		Enabled:     enabled,
		Description: description,
		Conditions:  fromMCPConditions(conditions),
	}
	return b.app.UpdateMockRule(rule)
}

func (b *MCPBridge) RemoveMockRule(ruleID string) {
	b.app.RemoveMockRule(ruleID)
}

func (b *MCPBridge) GetMockRules() []mcp.MCPMockRule {
	rules := b.app.GetMockRules()
	result := make([]mcp.MCPMockRule, len(rules))
	for i, r := range rules {
		result[i] = mcp.MCPMockRule{
			ID:          r.ID,
			URLPattern:  r.URLPattern,
			Method:      r.Method,
			StatusCode:  r.StatusCode,
			Headers:     r.Headers,
			Body:        r.Body,
			BodyFile:    r.BodyFile,
			Delay:       r.Delay,
			Enabled:     r.Enabled,
			Description: r.Description,
			Conditions:  toMCPConditions(r.Conditions),
		}
	}
	return result
}

// toMCPConditions converts app-level conditions to MCP-level conditions.
func toMCPConditions(conditions []MockCondition) []mcp.MCPMockCondition {
	if len(conditions) == 0 {
		return nil
	}
	result := make([]mcp.MCPMockCondition, len(conditions))
	for i, c := range conditions {
		result[i] = mcp.MCPMockCondition{
			Type:     c.Type,
			Key:      c.Key,
			Operator: c.Operator,
			Value:    c.Value,
		}
	}
	return result
}

// fromMCPConditions converts MCP-level conditions to app-level conditions.
func fromMCPConditions(conditions []mcp.MCPMockCondition) []MockCondition {
	if len(conditions) == 0 {
		return nil
	}
	result := make([]MockCondition, len(conditions))
	for i, c := range conditions {
		result[i] = MockCondition{
			Type:     c.Type,
			Key:      c.Key,
			Operator: c.Operator,
			Value:    c.Value,
		}
	}
	return result
}

func (b *MCPBridge) ToggleMockRule(ruleID string, enabled bool) error {
	return b.app.ToggleMockRule(ruleID, enabled)
}

func (b *MCPBridge) ExportMockRules() (string, error) {
	return b.app.ExportMockRules()
}

func (b *MCPBridge) ImportMockRules(jsonStr string) (int, error) {
	return b.app.ImportMockRules(jsonStr)
}

func (b *MCPBridge) ResendRequest(method, url string, headers map[string]string, body string) (map[string]interface{}, error) {
	return b.app.ResendRequest(method, url, headers, body)
}

// === Breakpoint Rules ===

func (b *MCPBridge) AddBreakpointRule(urlPattern, method, phase, description string) string {
	rule := BreakpointRule{
		URLPattern:  urlPattern,
		Method:      method,
		Phase:       phase,
		Description: description,
	}
	return b.app.AddBreakpointRule(rule)
}

func (b *MCPBridge) UpdateBreakpointRule(id, urlPattern, method, phase string, enabled bool, description string) error {
	// We need to preserve CreatedAt from the existing rule
	existing := b.app.GetBreakpointRules()
	var createdAt int64
	for _, r := range existing {
		if r.ID == id {
			createdAt = r.CreatedAt
			break
		}
	}
	rule := BreakpointRule{
		ID:          id,
		URLPattern:  urlPattern,
		Method:      method,
		Phase:       phase,
		Enabled:     enabled,
		Description: description,
		CreatedAt:   createdAt,
	}
	return b.app.UpdateBreakpointRule(rule)
}

func (b *MCPBridge) RemoveBreakpointRule(ruleID string) {
	b.app.RemoveBreakpointRule(ruleID)
}

func (b *MCPBridge) GetBreakpointRules() []mcp.MCPBreakpointRule {
	rules := b.app.GetBreakpointRules()
	result := make([]mcp.MCPBreakpointRule, len(rules))
	for i, r := range rules {
		result[i] = mcp.MCPBreakpointRule{
			ID:          r.ID,
			URLPattern:  r.URLPattern,
			Method:      r.Method,
			Phase:       r.Phase,
			Enabled:     r.Enabled,
			Description: r.Description,
			CreatedAt:   r.CreatedAt,
		}
	}
	return result
}

func (b *MCPBridge) ToggleBreakpointRule(ruleID string, enabled bool) error {
	return b.app.ToggleBreakpointRule(ruleID, enabled)
}

func (b *MCPBridge) ResolveBreakpoint(breakpointID string, action string, modifications map[string]interface{}) error {
	return b.app.ResolveBreakpoint(breakpointID, action, modifications)
}

func (b *MCPBridge) GetPendingBreakpoints() []mcp.MCPPendingBreakpointInfo {
	pending := b.app.GetPendingBreakpoints()
	result := make([]mcp.MCPPendingBreakpointInfo, len(pending))
	for i, p := range pending {
		result[i] = mcp.MCPPendingBreakpointInfo{
			ID:          p.ID,
			RuleID:      p.RuleID,
			Phase:       p.Phase,
			Method:      p.Method,
			URL:         p.URL,
			Headers:     p.Headers,
			Body:        p.Body,
			StatusCode:  p.StatusCode,
			RespHeaders: p.RespHeaders,
			RespBody:    p.RespBody,
			CreatedAt:   p.CreatedAt,
		}
	}
	return result
}

func (b *MCPBridge) ForwardAllBreakpoints() {
	b.app.ForwardAllBreakpoints()
}

// --- Map Remote Rules Bridge ---

func (b *MCPBridge) AddMapRemoteRule(sourcePattern, targetURL, method, description string) string {
	return b.app.AddMapRemoteRule(sourcePattern, targetURL, method, description)
}

func (b *MCPBridge) UpdateMapRemoteRule(id, sourcePattern, targetURL, method string, enabled bool, description string) error {
	return b.app.UpdateMapRemoteRule(id, sourcePattern, targetURL, method, enabled, description)
}

func (b *MCPBridge) RemoveMapRemoteRule(ruleID string) {
	b.app.RemoveMapRemoteRule(ruleID)
}

func (b *MCPBridge) GetMapRemoteRules() []mcp.MCPMapRemoteRule {
	rules := b.app.GetMapRemoteRules()
	result := make([]mcp.MCPMapRemoteRule, len(rules))
	for i, r := range rules {
		result[i] = mcp.MCPMapRemoteRule{
			ID:            r.ID,
			SourcePattern: r.SourcePattern,
			TargetURL:     r.TargetURL,
			Method:        r.Method,
			Enabled:       r.Enabled,
			Description:   r.Description,
			CreatedAt:     r.CreatedAt,
		}
	}
	return result
}

func (b *MCPBridge) ToggleMapRemoteRule(ruleID string, enabled bool) error {
	return b.app.ToggleMapRemoteRule(ruleID, enabled)
}

// === Rewrite Rules ===

func (b *MCPBridge) AddRewriteRule(urlPattern, method, phase, target, headerName, match, replace, description string) string {
	return b.app.AddRewriteRule(urlPattern, method, phase, target, headerName, match, replace, description)
}

func (b *MCPBridge) UpdateRewriteRule(id, urlPattern, method, phase, target, headerName, match, replace string, enabled bool, description string) error {
	return b.app.UpdateRewriteRule(id, urlPattern, method, phase, target, headerName, match, replace, enabled, description)
}

func (b *MCPBridge) RemoveRewriteRule(ruleID string) {
	b.app.RemoveRewriteRule(ruleID)
}

func (b *MCPBridge) GetRewriteRules() []mcp.MCPRewriteRule {
	rules := b.app.GetRewriteRules()
	result := make([]mcp.MCPRewriteRule, len(rules))
	for i, r := range rules {
		result[i] = mcp.MCPRewriteRule{
			ID:          r.ID,
			URLPattern:  r.URLPattern,
			Method:      r.Method,
			Phase:       r.Phase,
			Target:      r.Target,
			HeaderName:  r.HeaderName,
			Match:       r.Match,
			Replace:     r.Replace,
			Enabled:     r.Enabled,
			Description: r.Description,
			CreatedAt:   r.CreatedAt,
		}
	}
	return result
}

func (b *MCPBridge) ToggleRewriteRule(ruleID string, enabled bool) error {
	return b.app.ToggleRewriteRule(ruleID, enabled)
}

// Touch Recording & Script Management

func (b *MCPBridge) StartTouchRecording(deviceId string, mode string) error {
	return b.app.StartTouchRecording(deviceId, mode)
}

func (b *MCPBridge) StopTouchRecording(deviceId string) (*mcp.TouchScript, error) {
	script, err := b.app.StopTouchRecording(deviceId)
	if err != nil {
		return nil, err
	}
	// Convert main.TouchScript to mcp.TouchScript
	result := &mcp.TouchScript{
		Name:              script.Name,
		DeviceID:          script.DeviceID,
		DeviceModel:       script.DeviceModel,
		Resolution:        script.Resolution,
		CreatedAt:         script.CreatedAt,
		Events:            make([]mcp.TouchEvent, len(script.Events)),
		SmartTapTimeoutMs: script.SmartTapTimeoutMs,
		PlaybackSpeed:     script.PlaybackSpeed,
	}
	for i, e := range script.Events {
		result.Events[i] = mcp.TouchEvent{
			Timestamp: e.Timestamp,
			Type:      e.Type,
			X:         e.X,
			Y:         e.Y,
			X2:        e.X2,
			Y2:        e.Y2,
			Duration:  e.Duration,
		}
		if e.Selector != nil {
			result.Events[i].Selector = &mcp.ElementSelector{
				Type:  e.Selector.Type,
				Value: e.Selector.Value,
				Index: e.Selector.Index,
			}
		}
	}
	return result, nil
}

func (b *MCPBridge) IsRecordingTouch(deviceId string) bool {
	return b.app.IsRecordingTouch(deviceId)
}

func (b *MCPBridge) PlayTouchScript(deviceId string, script mcp.TouchScript) error {
	// Convert mcp.TouchScript to main.TouchScript
	mainScript := TouchScript{
		Name:              script.Name,
		DeviceID:          script.DeviceID,
		DeviceModel:       script.DeviceModel,
		Resolution:        script.Resolution,
		CreatedAt:         script.CreatedAt,
		Events:            make([]TouchEvent, len(script.Events)),
		SmartTapTimeoutMs: script.SmartTapTimeoutMs,
		PlaybackSpeed:     script.PlaybackSpeed,
	}
	for i, e := range script.Events {
		mainScript.Events[i] = TouchEvent{
			Timestamp: e.Timestamp,
			Type:      e.Type,
			X:         e.X,
			Y:         e.Y,
			X2:        e.X2,
			Y2:        e.Y2,
			Duration:  e.Duration,
		}
		if e.Selector != nil {
			mainScript.Events[i].Selector = &ElementSelector{
				Type:  e.Selector.Type,
				Value: e.Selector.Value,
				Index: e.Selector.Index,
			}
		}
	}
	return b.app.PlayTouchScript(deviceId, mainScript)
}

func (b *MCPBridge) StopTouchPlayback(deviceId string) {
	b.app.StopTouchPlayback(deviceId)
}

func (b *MCPBridge) LoadTouchScripts() ([]mcp.TouchScript, error) {
	scripts, err := b.app.LoadTouchScripts()
	if err != nil {
		return nil, err
	}
	result := make([]mcp.TouchScript, len(scripts))
	for i, s := range scripts {
		result[i] = mcp.TouchScript{
			Name:              s.Name,
			DeviceID:          s.DeviceID,
			DeviceModel:       s.DeviceModel,
			Resolution:        s.Resolution,
			CreatedAt:         s.CreatedAt,
			Events:            make([]mcp.TouchEvent, len(s.Events)),
			SmartTapTimeoutMs: s.SmartTapTimeoutMs,
			PlaybackSpeed:     s.PlaybackSpeed,
		}
		for j, e := range s.Events {
			result[i].Events[j] = mcp.TouchEvent{
				Timestamp: e.Timestamp,
				Type:      e.Type,
				X:         e.X,
				Y:         e.Y,
				X2:        e.X2,
				Y2:        e.Y2,
				Duration:  e.Duration,
			}
		}
	}
	return result, nil
}

func (b *MCPBridge) SaveTouchScript(script mcp.TouchScript) error {
	mainScript := TouchScript{
		Name:              script.Name,
		DeviceID:          script.DeviceID,
		DeviceModel:       script.DeviceModel,
		Resolution:        script.Resolution,
		CreatedAt:         script.CreatedAt,
		Events:            make([]TouchEvent, len(script.Events)),
		SmartTapTimeoutMs: script.SmartTapTimeoutMs,
		PlaybackSpeed:     script.PlaybackSpeed,
	}
	for i, e := range script.Events {
		mainScript.Events[i] = TouchEvent{
			Timestamp: e.Timestamp,
			Type:      e.Type,
			X:         e.X,
			Y:         e.Y,
			X2:        e.X2,
			Y2:        e.Y2,
			Duration:  e.Duration,
		}
	}
	return b.app.SaveTouchScript(mainScript)
}

func (b *MCPBridge) DeleteTouchScript(name string) error {
	return b.app.DeleteTouchScript(name)
}

func (b *MCPBridge) ExecuteSingleTouchEvent(deviceId string, event mcp.TouchEvent, resolution string) error {
	mainEvent := TouchEvent{
		Timestamp: event.Timestamp,
		Type:      event.Type,
		X:         event.X,
		Y:         event.Y,
		X2:        event.X2,
		Y2:        event.Y2,
		Duration:  event.Duration,
	}
	if event.Selector != nil {
		mainEvent.Selector = &ElementSelector{
			Type:  event.Selector.Type,
			Value: event.Selector.Value,
			Index: event.Selector.Index,
		}
	}
	return b.app.ExecuteSingleTouchEvent(deviceId, mainEvent, resolution)
}

// === Individual Assertions ===

func (b *MCPBridge) ListStoredAssertions(sessionID, deviceID string, templatesOnly bool, limit int) ([]mcp.MCPStoredAssertion, error) {
	assertions, err := b.app.ListStoredAssertions(sessionID, deviceID, templatesOnly, limit)
	if err != nil {
		return nil, err
	}
	result := make([]mcp.MCPStoredAssertion, len(assertions))
	for i, a := range assertions {
		result[i] = mcp.MCPStoredAssertion{
			ID:          a.ID,
			Name:        a.Name,
			Description: a.Description,
			Type:        a.Type,
			SessionID:   a.SessionID,
			DeviceID:    a.DeviceID,
			Criteria:    a.Criteria,
			Expected:    a.Expected,
			IsTemplate:  a.IsTemplate,
			CreatedAt:   a.CreatedAt,
			UpdatedAt:   a.UpdatedAt,
		}
	}
	return result, nil
}

func (b *MCPBridge) CreateStoredAssertionJSON(assertionJSON string, saveAsTemplate bool) error {
	return b.app.CreateStoredAssertionJSON(assertionJSON, saveAsTemplate)
}

func (b *MCPBridge) GetStoredAssertion(assertionID string) (*mcp.MCPStoredAssertion, error) {
	stored, err := b.app.GetStoredAssertion(assertionID)
	if err != nil {
		return nil, err
	}
	return &mcp.MCPStoredAssertion{
		ID:          stored.ID,
		Name:        stored.Name,
		Description: stored.Description,
		Type:        stored.Type,
		SessionID:   stored.SessionID,
		DeviceID:    stored.DeviceID,
		Criteria:    stored.Criteria,
		Expected:    stored.Expected,
		IsTemplate:  stored.IsTemplate,
		CreatedAt:   stored.CreatedAt,
		UpdatedAt:   stored.UpdatedAt,
	}, nil
}

func (b *MCPBridge) UpdateStoredAssertionJSON(assertionID string, assertionJSON string) error {
	return b.app.UpdateStoredAssertionJSON(assertionID, assertionJSON)
}

func (b *MCPBridge) DeleteStoredAssertion(assertionID string) error {
	return b.app.DeleteStoredAssertion(assertionID)
}

func (b *MCPBridge) ExecuteStoredAssertionInSession(assertionID, sessionID, deviceID string) (*mcp.MCPAssertionResult, error) {
	result, err := b.app.ExecuteStoredAssertionInSession(assertionID, sessionID, deviceID)
	if err != nil {
		return nil, err
	}
	return &mcp.MCPAssertionResult{
		ID:            result.ID,
		AssertionID:   result.AssertionID,
		AssertionName: result.AssertionName,
		SessionID:     result.SessionID,
		Passed:        result.Passed,
		Message:       result.Message,
		ActualValue:   result.ActualValue,
		ExpectedValue: result.ExpectedValue,
		ExecutedAt:    result.ExecutedAt,
		Duration:      result.Duration,
	}, nil
}

func (b *MCPBridge) QuickAssertNoErrors(sessionID, deviceID string) (*mcp.MCPAssertionResult, error) {
	result, err := b.app.QuickAssertNoErrors(sessionID, deviceID)
	if err != nil {
		return nil, err
	}
	return &mcp.MCPAssertionResult{
		ID:            result.ID,
		AssertionID:   result.AssertionID,
		AssertionName: result.AssertionName,
		SessionID:     result.SessionID,
		Passed:        result.Passed,
		Message:       result.Message,
		ActualValue:   result.ActualValue,
		ExpectedValue: result.ExpectedValue,
		ExecutedAt:    result.ExecutedAt,
		Duration:      result.Duration,
	}, nil
}

func (b *MCPBridge) QuickAssertNoCrashes(sessionID, deviceID string) (*mcp.MCPAssertionResult, error) {
	result, err := b.app.QuickAssertNoCrashes(sessionID, deviceID)
	if err != nil {
		return nil, err
	}
	return &mcp.MCPAssertionResult{
		ID:            result.ID,
		AssertionID:   result.AssertionID,
		AssertionName: result.AssertionName,
		SessionID:     result.SessionID,
		Passed:        result.Passed,
		Message:       result.Message,
		ActualValue:   result.ActualValue,
		ExpectedValue: result.ExpectedValue,
		ExecutedAt:    result.ExecutedAt,
		Duration:      result.Duration,
	}, nil
}

// === Assertion Sets ===

func (b *MCPBridge) CreateAssertionSet(name, description string, assertionIDs []string) (string, error) {
	return b.app.CreateAssertionSet(name, description, assertionIDs)
}

func (b *MCPBridge) UpdateAssertionSet(id, name, description string, assertionIDs []string) error {
	return b.app.UpdateAssertionSet(id, name, description, assertionIDs)
}

func (b *MCPBridge) DeleteAssertionSet(id string) error {
	return b.app.DeleteAssertionSet(id)
}

func (b *MCPBridge) GetAssertionSet(id string) (*mcp.MCPAssertionSet, error) {
	set, err := b.app.GetAssertionSet(id)
	if err != nil {
		return nil, err
	}
	return &mcp.MCPAssertionSet{
		ID:          set.ID,
		Name:        set.Name,
		Description: set.Description,
		Assertions:  set.Assertions,
		CreatedAt:   set.CreatedAt,
		UpdatedAt:   set.UpdatedAt,
	}, nil
}

func (b *MCPBridge) ListAssertionSets() ([]mcp.MCPAssertionSet, error) {
	sets, err := b.app.ListAssertionSets()
	if err != nil {
		return nil, err
	}
	result := make([]mcp.MCPAssertionSet, len(sets))
	for i, s := range sets {
		result[i] = mcp.MCPAssertionSet{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Assertions:  s.Assertions,
			CreatedAt:   s.CreatedAt,
			UpdatedAt:   s.UpdatedAt,
		}
	}
	return result, nil
}

func (b *MCPBridge) ExecuteAssertionSet(setID, sessionID, deviceID string) (*mcp.MCPAssertionSetResult, error) {
	result, err := b.app.ExecuteAssertionSet(setID, sessionID, deviceID)
	if err != nil {
		return nil, err
	}
	return b.convertAssertionSetResult(result), nil
}

func (b *MCPBridge) GetAssertionSetResults(setID string, limit int) ([]mcp.MCPAssertionSetResult, error) {
	results, err := b.app.GetAssertionSetResults(setID, limit)
	if err != nil {
		return nil, err
	}
	mcpResults := make([]mcp.MCPAssertionSetResult, len(results))
	for i, r := range results {
		mcpResults[i] = *b.convertAssertionSetResult(&r)
	}
	return mcpResults, nil
}

func (b *MCPBridge) GetAssertionSetResultByExecution(executionID string) (*mcp.MCPAssertionSetResult, error) {
	result, err := b.app.GetAssertionSetResultByExecution(executionID)
	if err != nil {
		return nil, err
	}
	return b.convertAssertionSetResult(result), nil
}

// convertAssertionSetResult converts an App-level AssertionSetResult to MCP-level
func (b *MCPBridge) convertAssertionSetResult(r *AssertionSetResult) *mcp.MCPAssertionSetResult {
	mcpResults := make([]mcp.MCPAssertionResult, len(r.Results))
	for i, ar := range r.Results {
		mcpResults[i] = mcp.MCPAssertionResult{
			ID:            ar.ID,
			AssertionID:   ar.AssertionID,
			AssertionName: ar.AssertionName,
			SessionID:     ar.SessionID,
			Passed:        ar.Passed,
			Message:       ar.Message,
			ActualValue:   ar.ActualValue,
			ExpectedValue: ar.ExpectedValue,
			ExecutedAt:    ar.ExecutedAt,
			Duration:      ar.Duration,
		}
	}
	return &mcp.MCPAssertionSetResult{
		ID:          r.ID,
		SetID:       r.SetID,
		SetName:     r.SetName,
		SessionID:   r.SessionID,
		DeviceID:    r.DeviceID,
		ExecutionID: r.ExecutionID,
		StartTime:   r.StartTime,
		EndTime:     r.EndTime,
		Duration:    r.Duration,
		Status:      r.Status,
		Summary: mcp.MCPAssertionSetSummary{
			Total:    r.Summary.Total,
			Passed:   r.Summary.Passed,
			Failed:   r.Summary.Failed,
			Error:    r.Summary.Error,
			PassRate: r.Summary.PassRate,
		},
		Results:    mcpResults,
		ExecutedAt: r.ExecutedAt,
	}
}

// ========================================
// Plugin System Bridge
// ========================================

func (b *MCPBridge) ListPlugins() ([]interface{}, error) {
	plugins, err := b.app.ListPlugins()
	if err != nil {
		return nil, err
	}
	result := make([]interface{}, len(plugins))
	for i, p := range plugins {
		result[i] = p
	}
	return result, nil
}

func (b *MCPBridge) GetPlugin(id string) (interface{}, error) {
	return b.app.GetPlugin(id)
}

func (b *MCPBridge) SavePlugin(req interface{}) error {
	// Convert interface{} back to PluginSaveRequest
	jsonData, err := json.Marshal(req)
	if err != nil {
		return err
	}
	var saveReq PluginSaveRequest
	if err := json.Unmarshal(jsonData, &saveReq); err != nil {
		return err
	}
	return b.app.SavePlugin(saveReq)
}

func (b *MCPBridge) DeletePlugin(id string) error {
	return b.app.DeletePlugin(id)
}

func (b *MCPBridge) TogglePlugin(id string, enabled bool) error {
	return b.app.TogglePlugin(id, enabled)
}

func (b *MCPBridge) TestPlugin(script string, eventID string) (interface{}, error) {
	events, err := b.app.TestPlugin(script, eventID)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (b *MCPBridge) TestPluginDetailed(script string, eventID string) (interface{}, error) {
	result, err := b.app.TestPluginDetailed(script, eventID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (b *MCPBridge) TestPluginWithEventData(script string, eventDataJSON string) (interface{}, error) {
	result, err := b.app.TestPluginWithEventData(script, eventDataJSON)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (b *MCPBridge) TestPluginBatch(script string, eventIDs []string) (interface{}, error) {
	results, err := b.app.TestPluginBatch(script, eventIDs)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (b *MCPBridge) GetSampleEvents(sessionID string, sources []string, types []string, limit int) ([]interface{}, error) {
	events, err := b.app.GetSampleEvents(sessionID, sources, types, limit)
	if err != nil {
		return nil, err
	}
	// 转换为 []interface{}
	result := make([]interface{}, len(events))
	for i, e := range events {
		result[i] = e
	}
	return result, nil
}

// StartMCPServer starts the MCP server with the given app
func StartMCPServer(app *App) {
	bridge := NewMCPBridge(app)
	mcpServer := mcp.NewMCPServer(bridge)
	if err := mcpServer.Start(); err != nil {
		LogError("mcp").Err(err).Msg("Failed to start MCP server")
	}
}
