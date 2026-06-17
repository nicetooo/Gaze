package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"Gaze/pkg/fileutil"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// DefaultMaxWorkflowSteps is the maximum number of steps a workflow can execute
// This prevents infinite loops in workflows with circular connections
const DefaultMaxWorkflowSteps = 2000

// StepResult represents the result of executing a step
type StepResult struct {
	Success        bool  // Whether the step executed successfully
	IsBranchResult bool  // For branch nodes: true if this is a condition result (not execution error)
	Error          error // Error if execution failed
}

// WorkflowExecutionResult is defined in pkg/types/workflow.go
// Using the types package version for consistency

// workflowResults stores the last execution result for each device
var (
	workflowResults   = make(map[string]*WorkflowExecutionResult)
	workflowResultsMu = &activeTaskMu // Reuse the existing mutex
)

// stepModeEnabled stores step-by-step execution mode flag for each device
var (
	stepModeEnabled = make(map[string]bool)
	stepModeMu      = &activeTaskMu // Reuse the existing mutex
)

// getWorkflowsPath returns the path to the workflows directory
func (a *App) getWorkflowsPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	workflowsPath := filepath.Join(configDir, "Gaze", "workflows")
	_ = os.MkdirAll(workflowsPath, 0755)
	return workflowsPath
}

// SaveWorkflow saves a workflow to file
func (a *App) SaveWorkflow(workflow Workflow) error {
	workflowsPath := a.getWorkflowsPath()

	safeName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(workflow.ID, "_")
	if safeName == "" {
		safeName = fmt.Sprintf("wf_%d", time.Now().Unix())
	}

	filePath := filepath.Join(workflowsPath, safeName+".json")

	data, err := SerializeWorkflow(&workflow)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow: %w", err)
	}

	if a.workflowWatcher != nil {
		a.workflowWatcher.MarkSelfWrite(safeName)
	}

	// Atomic write: the watcher only sees the final rename (a Create event for
	// the .json name, suppressed by MarkSelfWrite); temp names never end in .json
	if err := fileutil.WriteFileAtomic(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write workflow file: %w", err)
	}

	// Emit event to notify frontend of workflow change
	if !a.mcpMode {
		wailsRuntime.EventsEmit(a.ctx, "workflow-list-changed", map[string]interface{}{
			"action":     "save",
			"workflowId": workflow.ID,
		})
	}

	return nil
}

// LoadWorkflows loads all saved workflows
func (a *App) LoadWorkflows() ([]Workflow, error) {
	workflowsPath := a.getWorkflowsPath()

	entries, err := os.ReadDir(workflowsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Workflow{}, nil
		}
		return nil, fmt.Errorf("failed to read workflows directory: %w", err)
	}

	workflows := make([]Workflow, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(workflowsPath, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		workflow, err := ParseWorkflow(data)
		if err != nil {
			LogWarn("workflow").Err(err).Str("file", entry.Name()).Msg("Failed to parse workflow")
			continue
		}

		workflows = append(workflows, *workflow)
	}

	// Sort by CreatedAt descending (newest first)
	sort.Slice(workflows, func(i, j int) bool {
		// Parse RFC3339 timestamps, fallback to string comparison if parsing fails
		ti, erri := time.Parse(time.RFC3339, workflows[i].CreatedAt)
		tj, errj := time.Parse(time.RFC3339, workflows[j].CreatedAt)
		if erri != nil || errj != nil {
			return workflows[i].CreatedAt > workflows[j].CreatedAt
		}
		return ti.After(tj)
	})

	return workflows, nil
}

// LoadWorkflowByID loads a single workflow by ID
func (a *App) LoadWorkflowByID(id string) (*Workflow, error) {
	workflowsPath := a.getWorkflowsPath()
	safeName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(id, "_")
	filePath := filepath.Join(workflowsPath, safeName+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("workflow not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read workflow: %w", err)
	}

	return ParseWorkflow(data)
}

// GetWorkflow returns a workflow by ID
func (a *App) GetWorkflow(workflowID string) (*Workflow, error) {
	return a.LoadWorkflowByID(workflowID)
}

// DeleteWorkflow deletes a saved workflow
func (a *App) DeleteWorkflow(id string) error {
	workflowsPath := a.getWorkflowsPath()

	safeName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(id, "_")
	filePath := filepath.Join(workflowsPath, safeName+".json")

	if a.workflowWatcher != nil {
		a.workflowWatcher.MarkSelfWrite(safeName)
	}

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("workflow not found")
		}
		return fmt.Errorf("failed to delete workflow: %w", err)
	}

	// Emit event to notify frontend of workflow change
	if !a.mcpMode {
		wailsRuntime.EventsEmit(a.ctx, "workflow-list-changed", map[string]interface{}{
			"action":     "delete",
			"workflowId": id,
		})
	}

	return nil
}

// StopWorkflow stops the currently running workflow on the device
func (a *App) StopWorkflow(device Device) {
	log.Printf("[Workflow] StopWorkflow called for device: %s", device.ID)
	a.StopTask(device.ID)
	log.Printf("[Workflow] StopWorkflow completed for device: %s", device.ID)
}

// ExecuteSingleWorkflowStep executes a single workflow step on the device
func (a *App) ExecuteSingleWorkflowStep(deviceId string, step WorkflowStep) error {
	if step.Type == "start" {
		return nil
	}

	ctx := a.ctx
	vars := make(map[string]string)

	if step.Common.PreWait > 0 {
		time.Sleep(time.Duration(step.Common.PreWait) * time.Millisecond)
	}

	result := a.executeStep(ctx, deviceId, &step, vars, "")

	if step.Common.PostDelay > 0 {
		time.Sleep(time.Duration(step.Common.PostDelay) * time.Millisecond)
	}

	return result.Error
}

// RunWorkflow executes a workflow on the specified device
func (a *App) RunWorkflow(device Device, workflow Workflow) error {
	deviceId := device.ID

	activeTaskMu.Lock()
	if _, exists := activeTaskCancel[deviceId]; exists {
		activeTaskMu.Unlock()
		return fmt.Errorf("workflow execution already in progress")
	}

	ctx, cancel := context.WithCancel(a.ctx)
	activeTaskCancel[deviceId] = cancel
	activeTaskMu.Unlock()

	sessionId := a.eventPipeline.EnsureActiveSession(deviceId)

	a.eventPipeline.EmitRaw(deviceId, SourceWorkflow, "workflow_start", LevelInfo,
		fmt.Sprintf("Workflow started: %s", workflow.Name),
		map[string]interface{}{
			"workflowId": workflow.ID,
			"totalSteps": len(workflow.Steps),
			"sessionId":  sessionId,
		})

	if !a.mcpMode {
		wailsRuntime.EventsEmit(a.ctx, "workflow-started", map[string]interface{}{
			"workflowId":   workflow.ID,
			"workflowName": workflow.Name,
			"deviceId":     deviceId,
		})
	}

	go func() {
		defer cleanupTaskPause(deviceId)
		defer a.clearStepMode(deviceId)

		startTime := time.Now()
		sessionStatus := "completed"

		// Initialize runtime result early so it can be observed during execution
		result := &WorkflowExecutionResult{
			WorkflowID:   workflow.ID,
			WorkflowName: workflow.Name,
			Status:       "running",
			StartTime:    startTime.UnixMilli(),
			StepsTotal:   len(workflow.Steps),
			Variables:    make(map[string]string),
		}

		activeTaskMu.Lock()
		workflowResults[deviceId] = result
		activeTaskMu.Unlock()

		err := a.runWorkflowInternal(ctx, deviceId, workflow, nil, result)

		endTime := time.Now()
		duration := endTime.Sub(startTime).Milliseconds()

		activeTaskMu.Lock()
		delete(activeTaskCancel, deviceId)
		result.EndTime = endTime.UnixMilli()
		result.Duration = duration
		result.Status = "completed"
		if err != nil {
			sessionStatus = "error"
			result.Status = "error"
			result.Error = err.Error()
			if err == context.Canceled {
				sessionStatus = "cancelled"
				result.Status = "cancelled"
				result.Error = "workflow was cancelled"
			}
		}
		activeTaskMu.Unlock()

		if err != nil {
			a.eventPipeline.EmitRaw(deviceId, SourceWorkflow, "workflow_error", LevelError,
				fmt.Sprintf("Workflow error: %s", workflow.Name),
				map[string]interface{}{
					"workflowId": workflow.ID,
					"error":      err.Error(),
					"status":     sessionStatus,
					"duration":   duration,
				})

			if !a.mcpMode {
				wailsRuntime.EventsEmit(a.ctx, "workflow-error", map[string]interface{}{
					"workflowId":   workflow.ID,
					"workflowName": workflow.Name,
					"deviceId":     deviceId,
					"error":        err.Error(),
				})
			}
		} else {
			a.eventPipeline.EmitRaw(deviceId, SourceWorkflow, "workflow_complete", LevelInfo,
				fmt.Sprintf("Workflow completed: %s", workflow.Name),
				map[string]interface{}{
					"workflowId": workflow.ID,
					"status":     sessionStatus,
					"duration":   duration,
				})

			if !a.mcpMode {
				wailsRuntime.EventsEmit(a.ctx, "workflow-completed", map[string]interface{}{
					"workflowId":   workflow.ID,
					"workflowName": workflow.Name,
					"deviceId":     deviceId,
					"duration":     duration,
				})
			}
		}
	}()

	return nil
}

// GetWorkflowExecutionResult returns the last execution result for a device
func (a *App) GetWorkflowExecutionResult(deviceId string) *WorkflowExecutionResult {
	activeTaskMu.Lock()
	defer activeTaskMu.Unlock()
	return workflowResults[deviceId]
}

// setStepMode enables step-by-step execution mode for a device
func (a *App) setStepMode(deviceId string, enabled bool) {
	activeTaskMu.Lock()
	defer activeTaskMu.Unlock()
	stepModeEnabled[deviceId] = enabled
}

// isStepMode checks if step-by-step execution mode is enabled
func (a *App) isStepMode(deviceId string) bool {
	activeTaskMu.Lock()
	defer activeTaskMu.Unlock()
	return stepModeEnabled[deviceId]
}

// clearStepMode clears the step-by-step execution mode flag
func (a *App) clearStepMode(deviceId string) {
	activeTaskMu.Lock()
	defer activeTaskMu.Unlock()
	delete(stepModeEnabled, deviceId)
}

// updateWorkflowRuntimeState updates the runtime state during workflow execution
func (a *App) updateWorkflowRuntimeState(deviceId string, result *WorkflowExecutionResult, stepId, stepName, stepType string, vars map[string]string, stepsExecuted int) {
	if result == nil {
		return
	}
	activeTaskMu.Lock()
	defer activeTaskMu.Unlock()

	result.CurrentStepID = stepId
	result.CurrentStepName = stepName
	result.CurrentStepType = stepType
	result.StepsExecuted = stepsExecuted
	result.IsPaused = a.isTaskPausedLocked(deviceId)

	// Copy variables state
	if vars != nil {
		result.Variables = make(map[string]string)
		for k, v := range vars {
			result.Variables[k] = v
		}
	}

	// Emit runtime update event to frontend
	if !a.mcpMode {
		wailsRuntime.EventsEmit(a.ctx, "workflow-runtime-update", map[string]interface{}{
			"deviceId":        deviceId,
			"workflowId":      result.WorkflowID,
			"workflowName":    result.WorkflowName,
			"status":          result.Status,
			"currentStepId":   result.CurrentStepID,
			"currentStepName": result.CurrentStepName,
			"currentStepType": result.CurrentStepType,
			"stepsExecuted":   result.StepsExecuted,
			"stepsTotal":      result.StepsTotal,
			"isPaused":        result.IsPaused,
			"variables":       result.Variables,
		})
	}
}

// isTaskPausedLocked checks if the task is paused (must be called with lock held)
func (a *App) isTaskPausedLocked(deviceId string) bool {
	taskPauseMu.Lock()
	defer taskPauseMu.Unlock()
	return taskIsPaused[deviceId]
}

// emitWorkflowRuntimeUpdate sends the current workflow runtime state to frontend
func (a *App) emitWorkflowRuntimeUpdate(deviceId string) {
	if a.mcpMode {
		return
	}
	activeTaskMu.Lock()
	result := workflowResults[deviceId]
	activeTaskMu.Unlock()

	if result == nil {
		return
	}

	// Update isPaused state
	taskPauseMu.Lock()
	isPaused := taskIsPaused[deviceId]
	taskPauseMu.Unlock()

	wailsRuntime.EventsEmit(a.ctx, "workflow-runtime-update", map[string]interface{}{
		"deviceId":        deviceId,
		"workflowId":      result.WorkflowID,
		"workflowName":    result.WorkflowName,
		"status":          result.Status,
		"currentStepId":   result.CurrentStepID,
		"currentStepName": result.CurrentStepName,
		"currentStepType": result.CurrentStepType,
		"stepsExecuted":   result.StepsExecuted,
		"stepsTotal":      result.StepsTotal,
		"isPaused":        isPaused,
		"variables":       result.Variables,
	})
}

// IsWorkflowRunning checks if a workflow is running on the device
func (a *App) IsWorkflowRunning(deviceId string) bool {
	return a.IsPlayingTouch(deviceId)
}

// StepNextWorkflow executes the next step and then pauses automatically
func (a *App) StepNextWorkflow(deviceId string) (*WorkflowExecutionResult, error) {
	if !a.IsWorkflowRunning(deviceId) {
		return nil, fmt.Errorf("no workflow running on device %s", deviceId)
	}

	// Check if already paused
	if !a.isTaskPausedLocked(deviceId) {
		return nil, fmt.Errorf("workflow is not paused, use workflow_pause first or wait for it to pause")
	}

	a.setStepMode(deviceId, true) // Mark step mode: will pause after next step
	a.ResumeTask(deviceId)        // Resume execution

	// Wait for the step to complete (workflow will pause again after one step)
	for i := 0; i < 600; i++ { // Max wait 60 seconds
		time.Sleep(100 * time.Millisecond)
		if a.isTaskPausedLocked(deviceId) || !a.IsWorkflowRunning(deviceId) {
			break
		}
	}

	return a.GetWorkflowExecutionResult(deviceId), nil
}

// runWorkflowInternal is the core execution loop
func (a *App) runWorkflowInternal(ctx context.Context, deviceId string, workflow Workflow, parentVars map[string]string, runtimeResult *WorkflowExecutionResult) error {
	stepMap := make(map[string]*WorkflowStep)
	for i := range workflow.Steps {
		stepMap[workflow.Steps[i].ID] = &workflow.Steps[i]
	}

	vars := make(map[string]string)
	for k, v := range workflow.Variables {
		vars[k] = v
	}
	for k, v := range parentVars {
		vars[k] = v
	}
	// Store workflow metadata for use by session nodes
	vars["_workflowId"] = workflow.ID
	vars["_workflowName"] = workflow.Name

	var startStep *WorkflowStep
	for i := range workflow.Steps {
		if workflow.Steps[i].Type == "start" {
			startStep = &workflow.Steps[i]
			break
		}
	}

	if startStep == nil {
		return fmt.Errorf("workflow has no start node")
	}

	currentStepId := startStep.Connections.SuccessStepId
	if currentStepId == "" {
		return nil
	}

	maxSteps := DefaultMaxWorkflowSteps
	stepCount := 0

	for currentStepId != "" && stepCount < maxSteps {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		// Check for pause (blocks if paused, but also responds to context cancellation)
		if a.checkPauseWithContext(ctx, deviceId) {
			return context.Canceled
		}

		step, exists := stepMap[currentStepId]
		if !exists {
			return fmt.Errorf("step not found: %s", currentStepId)
		}

		stepCount++

		// Update runtime state before executing step
		a.updateWorkflowRuntimeState(deviceId, runtimeResult, step.ID, step.Name, step.Type, vars, stepCount-1)

		if !a.mcpMode {
			wailsRuntime.EventsEmit(a.ctx, "workflow-step-running", map[string]interface{}{
				"workflowId": workflow.ID,
				"stepId":     step.ID,
				"stepType":   step.Type,
				"stepName":   step.Name,
				"deviceId":   deviceId,
			})
		}

		a.eventPipeline.EmitRaw(deviceId, SourceWorkflow, "workflow_step_start", LevelInfo,
			fmt.Sprintf("Step: %s (%s)", step.Name, step.Type),
			map[string]interface{}{
				"workflowId": workflow.ID,
				"stepId":     step.ID,
				"stepType":   step.Type,
			})

		loopCount := step.Common.Loop
		if loopCount <= 0 {
			loopCount = 1
		}

		var result StepResult
		for i := 0; i < loopCount; i++ {
			if step.Common.PreWait > 0 {
				time.Sleep(time.Duration(step.Common.PreWait) * time.Millisecond)
			}

			result = a.executeStep(ctx, deviceId, step, vars, workflow.ID)

			if step.Common.PostDelay > 0 {
				time.Sleep(time.Duration(step.Common.PostDelay) * time.Millisecond)
			}

			if !result.Success && step.ShouldStopOnError() {
				break
			}
		}

		nextStepId := a.determineNextStep(step, result)

		logLevel := LevelInfo
		if !result.Success {
			logLevel = LevelWarn
		}
		a.eventPipeline.EmitRaw(deviceId, SourceWorkflow, "workflow_step_end", logLevel,
			fmt.Sprintf("Step completed: %s", step.Name),
			map[string]interface{}{
				"workflowId": workflow.ID,
				"stepId":     step.ID,
				"success":    result.Success,
				"nextStepId": nextStepId,
			})

		// Update runtime state after executing step
		a.updateWorkflowRuntimeState(deviceId, runtimeResult, nextStepId, "", "", vars, stepCount)

		// Check step mode: pause after each step if enabled
		if a.isStepMode(deviceId) {
			a.clearStepMode(deviceId)
			a.PauseTask(deviceId)
		}

		if result.Error != nil && nextStepId == "" && step.ShouldStopOnError() {
			return fmt.Errorf("step %s failed: %w", step.ID, result.Error)
		}

		currentStepId = nextStepId
	}

	if stepCount >= maxSteps {
		return fmt.Errorf("maximum step count exceeded (%d)", maxSteps)
	}

	return nil
}

// determineNextStep determines the next step based on execution result
func (a *App) determineNextStep(step *WorkflowStep, result StepResult) string {
	if step.Type == "branch" && result.IsBranchResult {
		if result.Success {
			return step.Connections.TrueStepId
		}
		return step.Connections.FalseStepId
	}

	if result.Success {
		return step.Connections.SuccessStepId
	}

	if step.Connections.ErrorStepId != "" {
		return step.Connections.ErrorStepId
	}

	if step.Common.OnError == "continue" {
		return step.Connections.SuccessStepId
	}

	return ""
}

// executeStep executes a single step and returns the result
func (a *App) executeStep(ctx context.Context, deviceId string, step *WorkflowStep, vars map[string]string, workflowId string) StepResult {
	switch step.Type {
	case "start":
		return StepResult{Success: true}

	case "tap":
		if step.Tap == nil {
			return StepResult{Success: false, Error: fmt.Errorf("tap params missing")}
		}
		_, err := a.RunAdbCommand(deviceId, fmt.Sprintf("shell input tap %d %d", step.Tap.X, step.Tap.Y))
		return StepResult{Success: err == nil, Error: err}

	case "swipe":
		if step.Swipe == nil {
			return StepResult{Success: false, Error: fmt.Errorf("swipe params missing")}
		}
		x1, y1 := step.Swipe.X, step.Swipe.Y
		x2, y2 := step.Swipe.X2, step.Swipe.Y2
		duration := step.Swipe.Duration
		if duration <= 0 {
			duration = 300
		}

		if step.Swipe.Direction != "" && step.Swipe.Distance > 0 {
			distance := step.Swipe.Distance
			switch step.Swipe.Direction {
			case "up":
				y2 = y1 - distance
				x2 = x1
			case "down":
				y2 = y1 + distance
				x2 = x1
			case "left":
				x2 = x1 - distance
				y2 = y1
			case "right":
				x2 = x1 + distance
				y2 = y1
			}
		}

		_, err := a.RunAdbCommand(deviceId, fmt.Sprintf("shell input swipe %d %d %d %d %d", x1, y1, x2, y2, duration))
		return StepResult{Success: err == nil, Error: err}

	case "click_element", "long_click_element", "input_text", "swipe_element", "wait_element", "wait_gone", "assert_element":
		return a.executeElementStep(ctx, deviceId, step, vars)

	case "launch_app", "stop_app", "clear_app", "open_settings":
		return a.executeAppStep(deviceId, step)

	case "branch":
		return a.executeBranchStep(deviceId, step, vars)

	case "wait":
		if step.Wait == nil {
			return StepResult{Success: false, Error: fmt.Errorf("wait params missing")}
		}
		time.Sleep(time.Duration(step.Wait.DurationMs) * time.Millisecond)
		return StepResult{Success: true}

	case "script":
		if step.Script == nil {
			return StepResult{Success: false, Error: fmt.Errorf("script params missing")}
		}
		err := a.executeScriptStep(ctx, deviceId, step.Script.ScriptName)
		return StepResult{Success: err == nil, Error: err}

	case "set_variable":
		if step.Variable == nil {
			return StepResult{Success: false, Error: fmt.Errorf("variable params missing")}
		}
		// Use expression-aware processing for set_variable to support arithmetic
		processedValue := a.processWorkflowVariablesWithExpr(step.Variable.Value, vars)
		vars[step.Variable.Name] = processedValue
		return StepResult{Success: true}

	case "read_to_variable":
		return a.executeReadToVariableStep(ctx, deviceId, step, vars)

	case "adb":
		if step.ADB == nil {
			return StepResult{Success: false, Error: fmt.Errorf("adb params missing")}
		}
		processedCmd := a.processWorkflowVariables(step.ADB.Command, vars)
		_, err := a.RunAdbCommand(deviceId, processedCmd)
		return StepResult{Success: err == nil, Error: err}

	case "run_workflow":
		if step.Workflow == nil {
			return StepResult{Success: false, Error: fmt.Errorf("workflow params missing")}
		}
		err := a.executeSubWorkflow(ctx, deviceId, step.Workflow.WorkflowId, vars, 0)
		return StepResult{Success: err == nil, Error: err}

	case "key_back":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 4")
		return StepResult{Success: err == nil, Error: err}

	case "key_home":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 3")
		return StepResult{Success: err == nil, Error: err}

	case "key_recent":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 187")
		return StepResult{Success: err == nil, Error: err}

	case "key_power":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 26")
		return StepResult{Success: err == nil, Error: err}

	case "key_volume_up":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 24")
		return StepResult{Success: err == nil, Error: err}

	case "key_volume_down":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 25")
		return StepResult{Success: err == nil, Error: err}

	case "screen_on":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 224")
		return StepResult{Success: err == nil, Error: err}

	case "screen_off":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 223")
		return StepResult{Success: err == nil, Error: err}

	case "start_session":
		sessionName := ""
		if step.Session != nil && step.Session.SessionName != "" {
			sessionName = a.processWorkflowVariables(step.Session.SessionName, vars)
		}
		// Generate default session name if not provided
		if sessionName == "" {
			workflowName := vars["_workflowName"]
			if workflowName != "" {
				sessionName = fmt.Sprintf("Workflow: %s", workflowName)
			} else {
				sessionName = fmt.Sprintf("Workflow Session %s", time.Now().Format("15:04:05"))
			}
		}
		config := a.sessionParamsToConfig(step.Session)
		sessionID := a.StartSessionWithConfig(deviceId, sessionName, config)
		if sessionID == "" {
			return StepResult{Success: false, Error: fmt.Errorf("failed to create session")}
		}
		vars["_sessionId"] = sessionID
		return StepResult{Success: true}

	case "end_session":
		status := "completed"
		if step.Session != nil && step.Session.Status != "" {
			status = step.Session.Status
		}
		sessionID := vars["_sessionId"]
		if sessionID == "" {
			// Try to get active session for device
			if session := a.GetDeviceActiveSession(deviceId); session != nil {
				sessionID = session.ID
			}
		}
		if sessionID != "" {
			a.EndActiveSession(sessionID, status)
		}
		delete(vars, "_sessionId")
		return StepResult{Success: true}

	default:
		return StepResult{Success: false, Error: fmt.Errorf("unknown step type: %s", step.Type)}
	}
}

// executeElementStep handles element-based actions
func (a *App) executeElementStep(ctx context.Context, deviceId string, step *WorkflowStep, vars map[string]string) StepResult {
	if step.Element == nil {
		return StepResult{Success: false, Error: fmt.Errorf("element params missing")}
	}

	selector := &ElementSelector{
		Type:  step.Element.Selector.Type,
		Value: a.processWorkflowVariables(step.Element.Selector.Value, vars),
		Index: step.Element.Selector.Index,
	}

	timeout := step.Common.Timeout
	if timeout <= 0 {
		timeout = 5000
	}

	config := &ElementActionConfig{
		Timeout: timeout,
	}

	var err error
	switch step.Element.Action {
	case "click":
		err = a.ClickElement(ctx, deviceId, selector, config)

	case "long_click":
		err = a.LongClickElement(ctx, deviceId, selector, 1000, config)

	case "input":
		inputText := a.processWorkflowVariables(step.Element.InputText, vars)
		err = a.InputTextToElement(ctx, deviceId, selector, inputText, false, config)

	case "swipe":
		distance := step.Element.SwipeDistance
		if distance <= 0 {
			distance = 300
		}
		duration := step.Element.SwipeDuration
		if duration <= 0 {
			duration = 300
		}
		err = a.SwipeOnElement(ctx, deviceId, selector, step.Element.SwipeDir, distance, duration, config)

	case "wait":
		err = a.WaitForElement(ctx, deviceId, selector, timeout)

	case "wait_gone":
		err = a.WaitElementGone(ctx, deviceId, selector, timeout)

	case "assert":
		err = a.WaitForElement(ctx, deviceId, selector, timeout)

	default:
		err = fmt.Errorf("unknown element action: %s", step.Element.Action)
	}

	return StepResult{Success: err == nil, Error: err}
}

// executeAppStep handles app operations
func (a *App) executeAppStep(deviceId string, step *WorkflowStep) StepResult {
	if step.App == nil {
		return StepResult{Success: false, Error: fmt.Errorf("app params missing")}
	}

	var err error
	switch step.App.Action {
	case "launch":
		_, err = a.StartApp(deviceId, step.App.PackageName)
	case "stop":
		_, err = a.ForceStopApp(deviceId, step.App.PackageName)
	case "clear":
		_, err = a.ClearAppData(deviceId, step.App.PackageName)
	case "settings":
		_, err = a.OpenSettings(deviceId, "android.settings.APPLICATION_DETAILS_SETTINGS", "package:"+step.App.PackageName)
	default:
		err = fmt.Errorf("unknown app action: %s", step.App.Action)
	}

	return StepResult{Success: err == nil, Error: err}
}

// executeBranchStep handles branch condition evaluation
func (a *App) executeBranchStep(deviceId string, step *WorkflowStep, vars map[string]string) StepResult {
	if step.Branch == nil {
		return StepResult{Success: false, Error: fmt.Errorf("branch params missing")}
	}

	condition := step.Branch.Condition
	if condition == "" {
		condition = "exists"
	}

	if condition == "variable_equals" {
		varName := step.Branch.VariableName
		expected := step.Branch.ExpectedValue
		actual := vars[varName]
		result := actual == expected
		return StepResult{Success: result, IsBranchResult: true}
	}

	if step.Branch.Selector == nil {
		return StepResult{Success: false, Error: fmt.Errorf("branch selector required for condition: %s", condition)}
	}

	hierarchy, err := a.GetUIHierarchy(deviceId)
	if err != nil {
		return StepResult{Success: false, Error: err}
	}

	selectorValue := a.processWorkflowVariables(step.Branch.Selector.Value, vars)

	var conditionResult bool
	switch condition {
	case "exists":
		node := a.findElementInHierarchy(hierarchy, step.Branch.Selector.Type, selectorValue)
		conditionResult = node != nil

	case "not_exists":
		node := a.findElementInHierarchy(hierarchy, step.Branch.Selector.Type, selectorValue)
		conditionResult = node == nil

	case "text_equals":
		node := a.findElementInHierarchy(hierarchy, step.Branch.Selector.Type, selectorValue)
		if node != nil {
			expected := step.Branch.ExpectedValue
			conditionResult = node.Text == expected
		}

	case "text_contains":
		node := a.findElementInHierarchy(hierarchy, step.Branch.Selector.Type, selectorValue)
		if node != nil {
			expected := step.Branch.ExpectedValue
			conditionResult = strings.Contains(node.Text, expected)
		}

	default:
		return StepResult{Success: false, Error: fmt.Errorf("unknown branch condition: %s", condition)}
	}

	return StepResult{Success: conditionResult, IsBranchResult: true}
}

// executeReadToVariableStep reads element text and stores it in a variable
func (a *App) executeReadToVariableStep(ctx context.Context, deviceId string, step *WorkflowStep, vars map[string]string) StepResult {
	if step.ReadToVariable == nil {
		return StepResult{Success: false, Error: fmt.Errorf("readToVariable params missing")}
	}

	params := step.ReadToVariable
	timeout := step.Common.Timeout
	if timeout <= 0 {
		timeout = 5000
	}

	// Process selector value with variables
	selector := &ElementSelector{
		Type:  params.Selector.Type,
		Value: a.processWorkflowVariables(params.Selector.Value, vars),
		Index: params.Selector.Index,
	}

	// Wait for element
	node, err := a.waitForElement(ctx, deviceId, selector, timeout, 1000)
	if err != nil {
		// Element not found, use default value if provided
		if params.DefaultValue != "" {
			vars[params.VariableName] = a.processWorkflowVariables(params.DefaultValue, vars)
			return StepResult{Success: true}
		}
		return StepResult{Success: false, Error: fmt.Errorf("element not found: %w", err)}
	}

	// Get the attribute value
	var value string
	attribute := params.Attribute
	if attribute == "" {
		attribute = "text"
	}

	switch attribute {
	case "text":
		value = node.Text
	case "contentDesc":
		value = node.ContentDesc
	case "resourceId":
		value = node.ResourceID
	case "className":
		value = node.Class
	case "bounds":
		value = node.Bounds
	default:
		// Default to text
		value = node.Text
	}

	// Apply regex if provided
	if params.Regex != "" {
		re, err := regexp.Compile(params.Regex)
		if err != nil {
			return StepResult{Success: false, Error: fmt.Errorf("invalid regex: %w", err)}
		}
		matches := re.FindStringSubmatch(value)
		if len(matches) > 1 {
			// Use first capture group
			value = matches[1]
		} else if len(matches) == 1 {
			// Use full match
			value = matches[0]
		}
	}

	// If value is empty, use default value
	if value == "" && params.DefaultValue != "" {
		value = a.processWorkflowVariables(params.DefaultValue, vars)
	}

	// Store the value in variables
	vars[params.VariableName] = value

	return StepResult{Success: true}
}

// executeSubWorkflow runs a sub-workflow
func (a *App) executeSubWorkflow(ctx context.Context, deviceId string, workflowId string, vars map[string]string, depth int) error {
	if depth > 10 {
		return fmt.Errorf("maximum workflow nesting depth exceeded")
	}

	workflow, err := a.LoadWorkflowByID(workflowId)
	if err != nil {
		return fmt.Errorf("failed to load sub-workflow: %w", err)
	}

	// Sub-workflows don't track their own runtime state (share parent's)
	return a.runWorkflowInternal(ctx, deviceId, *workflow, vars, nil)
}

// executeScriptStep runs a recorded touch script
func (a *App) executeScriptStep(ctx context.Context, deviceId string, scriptName string) error {
	scriptsPath := a.getScriptsPath()
	safeName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(scriptName, "_")
	filePath := filepath.Join(scriptsPath, safeName+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		filePath = filepath.Join(scriptsPath, scriptName+".json")
		data, err = os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("script not found: %s", scriptName)
		}
	}

	var script TouchScript
	if err := json.Unmarshal(data, &script); err != nil {
		return fmt.Errorf("failed to parse script: %w", err)
	}

	return a.playTouchScriptSync(ctx, deviceId, script, nil)
}

// processWorkflowVariables replaces placeholders like {{var}} with actual values
func (a *App) processWorkflowVariables(text string, vars map[string]string) string {
	if text == "" {
		return ""
	}
	for k, v := range vars {
		placeholder := "{{" + k + "}}"
		text = strings.ReplaceAll(text, placeholder, v)
	}
	return text
}

// processWorkflowVariablesWithExpr replaces variables and evaluates arithmetic expressions
// Supports: +, -, *, /, % and parentheses
// Example: "{{count}} + 1" with count=5 returns "6"
func (a *App) processWorkflowVariablesWithExpr(text string, vars map[string]string) string {
	// First, replace variables
	result := a.processWorkflowVariables(text, vars)

	// Try to evaluate as arithmetic expression
	if evaluated, ok := evaluateArithmeticExpression(result); ok {
		return evaluated
	}

	return result
}

// evaluateArithmeticExpression evaluates a simple arithmetic expression
// Returns the result as string and true if successful, or original and false if not an expression
func evaluateArithmeticExpression(expr string) (string, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return expr, false
	}

	// Check if it looks like an arithmetic expression (contains operators)
	if !strings.ContainsAny(expr, "+-*/%") {
		return expr, false
	}

	// Tokenize and evaluate
	result, err := evalExpr(expr)
	if err != nil {
		return expr, false
	}

	// Format result: use integer format if it's a whole number
	if result == float64(int64(result)) {
		return strconv.FormatInt(int64(result), 10), true
	}
	return strconv.FormatFloat(result, 'f', -1, 64), true
}

// evalExpr evaluates an arithmetic expression with +, -, *, /, %, and parentheses
func evalExpr(expr string) (float64, error) {
	p := &exprParser{input: expr, pos: 0}
	result, err := p.parseExpression()
	if err != nil {
		return 0, err
	}
	p.skipWhitespace()
	if p.pos < len(p.input) {
		return 0, fmt.Errorf("unexpected character at position %d", p.pos)
	}
	return result, nil
}

type exprParser struct {
	input string
	pos   int
}

func (p *exprParser) skipWhitespace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

func (p *exprParser) parseExpression() (float64, error) {
	return p.parseAddSub()
}

func (p *exprParser) parseAddSub() (float64, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return 0, err
	}

	for {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			break
		}

		op := p.input[p.pos]
		if op != '+' && op != '-' {
			break
		}
		p.pos++

		right, err := p.parseMulDiv()
		if err != nil {
			return 0, err
		}

		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}

	return left, nil
}

func (p *exprParser) parseMulDiv() (float64, error) {
	left, err := p.parseUnary()
	if err != nil {
		return 0, err
	}

	for {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			break
		}

		op := p.input[p.pos]
		if op != '*' && op != '/' && op != '%' {
			break
		}
		p.pos++

		right, err := p.parseUnary()
		if err != nil {
			return 0, err
		}

		switch op {
		case '*':
			left *= right
		case '/':
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		case '%':
			if right == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			left = float64(int64(left) % int64(right))
		}
	}

	return left, nil
}

func (p *exprParser) parseUnary() (float64, error) {
	p.skipWhitespace()

	if p.pos < len(p.input) && p.input[p.pos] == '-' {
		p.pos++
		val, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		return -val, nil
	}

	if p.pos < len(p.input) && p.input[p.pos] == '+' {
		p.pos++
		return p.parseUnary()
	}

	return p.parsePrimary()
}

func (p *exprParser) parsePrimary() (float64, error) {
	p.skipWhitespace()

	if p.pos >= len(p.input) {
		return 0, fmt.Errorf("unexpected end of expression")
	}

	// Parentheses
	if p.input[p.pos] == '(' {
		p.pos++
		result, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.skipWhitespace()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		p.pos++
		return result, nil
	}

	// Number
	return p.parseNumber()
}

func (p *exprParser) parseNumber() (float64, error) {
	p.skipWhitespace()

	start := p.pos
	hasDecimal := false

	// Handle negative sign (should be handled by parseUnary, but just in case)
	if p.pos < len(p.input) && p.input[p.pos] == '-' {
		p.pos++
	}

	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch >= '0' && ch <= '9' {
			p.pos++
		} else if ch == '.' && !hasDecimal {
			hasDecimal = true
			p.pos++
		} else {
			break
		}
	}

	if p.pos == start {
		return 0, fmt.Errorf("expected number at position %d", p.pos)
	}

	numStr := p.input[start:p.pos]
	return strconv.ParseFloat(numStr, 64)
}

// findElementInHierarchy finds an element in the UI hierarchy
func (a *App) findElementInHierarchy(hierarchy *UIHierarchyResult, selectorType, selectorValue string) *UINode {
	if hierarchy == nil || hierarchy.Root == nil {
		return nil
	}

	if selectorType == "xpath" {
		results := a.SearchElementsXPath(hierarchy.Root, selectorValue)
		if len(results) > 0 {
			return results[0].Node
		}
		return nil
	}

	selector := &ElementSelector{Type: selectorType, Value: selectorValue, Index: 0}
	return a.FindElementBySelector(hierarchy.Root, selector)
}

// sessionParamsToConfig converts workflow SessionParams to SessionConfig
func (a *App) sessionParamsToConfig(params *SessionParams) SessionConfig {
	if params == nil {
		return SessionConfig{}
	}
	return SessionConfig{
		Logcat: LogcatConfig{
			Enabled:       params.LogcatEnabled,
			PackageName:   params.LogcatPackageName,
			PreFilter:     params.LogcatPreFilter,
			ExcludeFilter: params.LogcatExcludeFilter,
		},
		Recording: RecordingConfig{
			Enabled: params.RecordingEnabled,
			Quality: params.RecordingQuality,
		},
		Proxy: ProxyConfig{
			Enabled:     params.ProxyEnabled,
			Port:        params.ProxyPort,
			MitmEnabled: params.ProxyMitmEnabled,
		},
		Monitor: MonitorConfig{
			Enabled: params.MonitorEnabled,
		},
	}
}
