package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

	// Use ID as filename for uniqueness
	safeName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(workflow.ID, "_")
	if safeName == "" {
		safeName = fmt.Sprintf("wf_%d", time.Now().Unix())
	}

	filePath := filepath.Join(workflowsPath, safeName+".json")

	data, err := json.MarshalIndent(workflow, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workflow: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write workflow file: %w", err)
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

		var workflow Workflow
		if err := json.Unmarshal(data, &workflow); err != nil {
			continue
		}

		workflows = append(workflows, workflow)
	}

	return workflows, nil
}

// DeleteWorkflow deletes a saved workflow
func (a *App) DeleteWorkflow(id string) error {
	workflowsPath := a.getWorkflowsPath()

	safeName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(id, "_")
	filePath := filepath.Join(workflowsPath, safeName+".json")

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("workflow not found")
		}
		return fmt.Errorf("failed to delete workflow: %w", err)
	}

	return nil
}

// StopWorkflow stops the currently running workflow on the device
func (a *App) StopWorkflow(device Device) {
	// Re-use logic from automation.go
	a.StopTask(device.ID)
}

// ExecuteSingleWorkflowStep executes a single workflow step on the device
func (a *App) ExecuteSingleWorkflowStep(deviceId string, step WorkflowStep) error {
	// Skip start node - it's just a visual marker
	if step.Type == "start" {
		return nil
	}

	ctx := context.Background()
	vars := make(map[string]string)

	// Handle pre-wait
	if step.PreWait > 0 {
		time.Sleep(time.Duration(step.PreWait) * time.Millisecond)
	}

	// Execute the step
	_, err := a.runWorkflowStep(ctx, deviceId, step, 1, 1, 0, vars)
	if err != nil {
		return err
	}

	// Handle post-delay
	if step.PostDelay > 0 {
		time.Sleep(time.Duration(step.PostDelay) * time.Millisecond)
	}

	return nil
}

// RunWorkflow executes a workflow on the specified device
func (a *App) RunWorkflow(device Device, workflow Workflow) error {
	deviceId := device.ID

	// Check if already running
	touchPlaybackMu.Lock()
	if _, exists := touchPlaybackCancel[deviceId]; exists {
		touchPlaybackMu.Unlock()
		return fmt.Errorf("workflow execution already in progress")
	}

	ctx, cancel := context.WithCancel(context.Background())
	touchPlaybackCancel[deviceId] = cancel
	touchPlaybackMu.Unlock()

	go func() {
		defer func() {
			touchPlaybackMu.Lock()
			delete(touchPlaybackCancel, deviceId)
			touchPlaybackMu.Unlock()
		}()

		wailsRuntime.EventsEmit(a.ctx, "workflow-started", map[string]interface{}{
			"deviceId":     deviceId,
			"workflowName": workflow.Name,
			"workflowId":   workflow.ID,
			"steps":        len(workflow.Steps),
		})

		// Initialize variable context
		vars := make(map[string]string)
		if workflow.Variables != nil {
			for k, v := range workflow.Variables {
				vars[k] = v
			}
		}

		err := a.runWorkflowInternal(ctx, deviceId, workflow, 0, vars)
		if err != nil && err != context.Canceled {
			wailsRuntime.EventsEmit(a.ctx, "workflow-error", map[string]interface{}{
				"deviceId":   deviceId,
				"workflowId": workflow.ID,
				"error":      err.Error(),
			})
		}

		wailsRuntime.EventsEmit(a.ctx, "workflow-completed", map[string]interface{}{
			"deviceId":     deviceId,
			"workflowName": workflow.Name,
			"workflowId":   workflow.ID,
		})
	}()

	return nil
}

// runWorkflowInternal contains the core graph traversal logic
func (a *App) runWorkflowInternal(ctx context.Context, deviceId string, workflow Workflow, depth int, vars map[string]string) error {
	// Build ID->Step map for graph navigation
	stepMap := make(map[string]*WorkflowStep)
	var startStep *WorkflowStep
	for i := range workflow.Steps {
		s := &workflow.Steps[i]
		stepMap[s.ID] = s
		if s.Type == "start" {
			startStep = s
		}
	}

	// Find start node
	if startStep == nil {
		return fmt.Errorf("workflow must have a 'start' node")
	}

	// Get the first actual step (what start node points to)
	currentStepID := startStep.NextStepId
	if currentStepID == "" {
		return nil
	}

	// Emit event for Start node to show it's running
	wailsRuntime.EventsEmit(a.ctx, "workflow-step-running", map[string]interface{}{
		"deviceId": deviceId,
		"stepId":   startStep.ID,
		"stepName": startStep.Name,
		"stepType": "start",
	})
	time.Sleep(200 * time.Millisecond)

	// Graph-based execution loop
	executedCount := 0
	maxSteps := 2000 // Safety limit

	for currentStepID != "" && executedCount < maxSteps {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		// Check pause
		a.checkPause(deviceId)

		step, exists := stepMap[currentStepID]
		if !exists {
			break
		}

		executedCount++

		wailsRuntime.EventsEmit(a.ctx, "workflow-step-running", map[string]interface{}{
			"deviceId":   deviceId,
			"workflowId": workflow.ID,
			"stepIndex":  executedCount,
			"stepId":     step.ID,
			"stepName":   step.Name,
			"stepType":   step.Type,
		})

		loopCount := step.Loop
		if loopCount < 1 {
			loopCount = 1
		}

		var stepResult bool = true

		for l := 0; l < loopCount; l++ {
			// Pre-Wait
			if step.PreWait > 0 {
				wailsRuntime.EventsEmit(a.ctx, "workflow-step-waiting", map[string]interface{}{
					"deviceId":   deviceId,
					"workflowId": workflow.ID,
					"stepId":     step.ID,
					"duration":   step.PreWait,
					"phase":      "pre",
				})
				time.Sleep(time.Duration(step.PreWait) * time.Millisecond)
			}

			var err error
			stepResult, err = a.runWorkflowStep(ctx, deviceId, *step, l+1, loopCount, depth, vars)
			if err != nil {
				if err == context.Canceled {
					return context.Canceled
				}
				if step.OnError == "continue" {
					goto check_cancel
				}
				return err
			}

			// Post Delay (Wait After)
			if step.PostDelay > 0 {
				wailsRuntime.EventsEmit(a.ctx, "workflow-step-waiting", map[string]interface{}{
					"deviceId":   deviceId,
					"workflowId": workflow.ID,
					"stepId":     step.ID,
					"duration":   step.PostDelay,
					"phase":      "post",
				})
				time.Sleep(time.Duration(step.PostDelay) * time.Millisecond)
			}

		check_cancel:
			select {
			case <-ctx.Done():
				return context.Canceled
			default:
			}
		}

		// Determine Next Step
		nextStepID := ""
		if step.Type == "branch" {
			if stepResult {
				nextStepID = step.TrueStepId
			} else {
				nextStepID = step.FalseStepId
			}
		} else {
			nextStepID = step.NextStepId
		}

		currentStepID = nextStepID
	}

	if executedCount >= maxSteps {
		return fmt.Errorf("workflow exceeded maximum step limit (possible infinite loop)")
	}

	return nil
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

// Updated signature to return (result, error)
func (a *App) runWorkflowStep(ctx context.Context, deviceId string, step WorkflowStep, _, _, depth int, vars map[string]string) (bool, error) {
	// Recursion guard
	if depth > 10 {
		return false, fmt.Errorf("maximum workflow nesting depth exceeded")
	}

	// Process variables in step value and selector before execution
	processedValue := a.processWorkflowVariables(step.Value, vars)
	var processedSelectorValue string
	if step.Selector != nil {
		processedSelectorValue = a.processWorkflowVariables(step.Selector.Value, vars)
	}

	switch step.Type {
	case "set_variable":
		if step.Name != "" {
			vars[step.Name] = processedValue
			fmt.Printf("[Workflow] Set variable: %s = %s\n", step.Name, processedValue)
		}
		return true, nil

	case "wait":
		duration := 1000
		if val, err := strconv.Atoi(processedValue); err == nil {
			duration = val
		}
		time.Sleep(time.Duration(duration) * time.Millisecond)

	case "adb":
		// Raw ADB Command
		if _, err := a.RunAdbCommand(deviceId, processedValue); err != nil {
			return false, fmt.Errorf("adb command failed: %w", err)
		}

	case "run_workflow":
		return true, a.loadAndRunSubWorkflow(ctx, deviceId, processedValue, depth, vars)

	case "branch":
		// 1. Evaluate condition based on type
		timeout := 2000 // Default 2s check
		if step.Timeout > 0 {
			timeout = step.Timeout
		}

		conditionType := step.ConditionType
		if conditionType == "" {
			conditionType = "exists" // Default to existence check
		}

		result := a.evaluateBranchCondition(deviceId, step, conditionType, processedSelectorValue, processedValue, timeout, vars)

		// 2. Determine functionality based on configuration
		isGraphMode := step.TrueStepId != "" || step.FalseStepId != ""

		if isGraphMode {
			// Graph Mode: Just return the result, navigation layout handled by caller
			return result, nil
		}

		// Legacy Mode: Nested Sub-Workflow Call
		// Parse Value to get branch targets
		var targets map[string]string
		if step.Value != "" {
			_ = json.Unmarshal([]byte(step.Value), &targets)
		}

		targetID := ""
		if result {
			targetID = targets["true"]
			fmt.Printf("[Workflow] Legacy Branch TRUE -> %s\n", targetID)
		} else {
			targetID = targets["false"]
			fmt.Printf("[Workflow] Legacy Branch FALSE -> %s\n", targetID)
		}

		if targetID == "" {
			return true, nil // No workflow configured, but step itself was successful
		}

		// Run the selected workflow recursively
		return true, a.loadAndRunSubWorkflow(ctx, deviceId, targetID, depth, vars)

	case "script":
		// Run recorded script
		scriptsPath := a.getScriptsPath()
		scriptName := step.Value
		safeName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(scriptName, "_")
		filePath := filepath.Join(scriptsPath, safeName+".json")

		data, err := os.ReadFile(filePath)
		if err != nil {
			filePath = filepath.Join(scriptsPath, scriptName+".json")
			data, err = os.ReadFile(filePath)
			if err != nil {
				return false, fmt.Errorf("script not found: %s", scriptName)
			}
		}

		var script TouchScript
		if err := json.Unmarshal(data, &script); err != nil {
			return false, fmt.Errorf("failed to parse script: %w", err)
		}

		return true, a.playTouchScriptSync(ctx, deviceId, script, nil)

	case "launch_app":
		_, err := a.StartApp(deviceId, step.Value)
		return true, err

	case "stop_app":
		_, err := a.ForceStopApp(deviceId, step.Value)
		return true, err

	case "clear_app":
		_, err := a.ClearAppData(deviceId, step.Value)
		return true, err

	case "open_settings":
		_, err := a.OpenSettings(deviceId, "android.settings.APPLICATION_DETAILS_SETTINGS", "package:"+step.Value)
		return true, err

	case "click_element", "long_click_element", "input_text", "assert_element", "wait_element", "wait_gone", "swipe_element":
		// Create a copy of the step with processed values for the handler
		processedStep := step
		processedStep.Value = processedValue
		if processedStep.Selector != nil {
			processedStep.Selector.Value = processedSelectorValue
		}
		return true, a.handleElementAction(ctx, deviceId, processedStep)

	// System key events
	case "key_back":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 4")
		return true, err
	case "key_home":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 3")
		return true, err
	case "key_recent":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 187")
		return true, err
	case "key_power":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 26")
		return true, err
	case "key_volume_up":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 24")
		return true, err
	case "key_volume_down":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 25")
		return true, err
	case "screen_on":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 224")
		return true, err
	case "screen_off":
		_, err := a.RunAdbCommand(deviceId, "shell input keyevent 223")
		return true, err

	case "start":
		return true, nil

	default:
		return false, fmt.Errorf("unknown step type: %s", step.Type)
	}

	return true, nil
}

func (a *App) handleElementAction(ctx context.Context, deviceId string, step WorkflowStep) error {
	// Use unified element service for all element operations
	config := &ElementActionConfig{
		Timeout:       step.Timeout,
		RetryInterval: 1000,
		OnError:       step.OnError,
	}
	if config.Timeout <= 0 {
		config.Timeout = 10000
	}

	switch step.Type {
	case "click_element":
		return a.ClickElement(ctx, deviceId, step.Selector, config)

	case "long_click_element":
		return a.LongClickElement(ctx, deviceId, step.Selector, 1000, config)

	case "input_text":
		return a.InputTextToElement(ctx, deviceId, step.Selector, step.Value, false, config)

	case "swipe_element":
		return a.SwipeOnElement(ctx, deviceId, step.Selector, step.Value, step.SwipeDistance, step.SwipeDuration, config)

	case "wait_element", "assert_element":
		return a.WaitForElement(ctx, deviceId, step.Selector, config.Timeout)

	case "wait_gone":
		return a.WaitElementGone(ctx, deviceId, step.Selector, config.Timeout)

	default:
		return fmt.Errorf("unknown element action: %s", step.Type)
	}
}

// findElementNode is a legacy wrapper that uses the unified selector service
// Deprecated: Use FindElementBySelector with ElementSelector instead
func (a *App) findElementNode(node *UINode, checkType, checkValue string) *UINode {
	selector := &ElementSelector{Type: checkType, Value: checkValue, Index: 0}
	return a.FindElementBySelector(node, selector)
}

// findAllElementNodes is a legacy wrapper that uses the unified selector service
// Deprecated: Use FindAllElementsBySelector with ElementSelector instead
func (a *App) findAllElementNodes(node *UINode, checkType, checkValue string) []*UINode {
	selector := &ElementSelector{Type: checkType, Value: checkValue}
	return a.FindAllElementsBySelector(node, selector)
}

// evaluateBranchCondition evaluates different types of branch conditions
func (a *App) evaluateBranchCondition(deviceId string, step WorkflowStep, conditionType, selectorValue, compareValue string, timeout int, vars map[string]string) bool {
	startTime := time.Now()

	for {
		// Check timeout
		if time.Since(startTime) > time.Duration(timeout)*time.Millisecond {
			break
		}

		// Get UI hierarchy
		hierarchy, err := a.GetUIHierarchy(deviceId)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		switch conditionType {
		case "exists":
			// Check if element exists
			if step.Selector == nil {
				return false
			}
			if step.Selector.Type == "xpath" {
				results := a.SearchElementsXPath(hierarchy.Root, selectorValue)
				if len(results) > 0 {
					return true
				}
			} else {
				if node := a.findElementNode(hierarchy.Root, step.Selector.Type, selectorValue); node != nil {
					return true
				}
			}

		case "not_exists":
			// Check if element does NOT exist
			if step.Selector == nil {
				return true
			}
			if step.Selector.Type == "xpath" {
				results := a.SearchElementsXPath(hierarchy.Root, selectorValue)
				if len(results) == 0 {
					return true
				}
			} else {
				if node := a.findElementNode(hierarchy.Root, step.Selector.Type, selectorValue); node == nil {
					return true
				}
			}

		case "text_equals":
			// Check if element's text equals the value
			if step.Selector == nil {
				return false
			}
			var node *UINode
			if step.Selector.Type == "xpath" {
				results := a.SearchElementsXPath(hierarchy.Root, selectorValue)
				if len(results) > 0 {
					node = results[0].Node
				}
			} else {
				node = a.findElementNode(hierarchy.Root, step.Selector.Type, selectorValue)
			}
			if node != nil && node.Text == compareValue {
				return true
			}

		case "text_contains":
			// Check if element's text contains the value
			if step.Selector == nil {
				return false
			}
			var node *UINode
			if step.Selector.Type == "xpath" {
				results := a.SearchElementsXPath(hierarchy.Root, selectorValue)
				if len(results) > 0 {
					node = results[0].Node
				}
			} else {
				node = a.findElementNode(hierarchy.Root, step.Selector.Type, selectorValue)
			}
			if node != nil && strings.Contains(node.Text, compareValue) {
				return true
			}

		case "variable_equals":
			// Compare two variables or a variable with a literal value
			// selectorValue is the first variable name (without {{}}), compareValue is the expected value
			varName := strings.TrimSpace(selectorValue)
			varValue, exists := vars[varName]
			if exists && varValue == compareValue {
				return true
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Timeout reached, condition not met
	return false
}

// parseBoundsCenter is a legacy wrapper that uses the unified ParseBounds
// Deprecated: Use ParseBounds().Center() instead
func parseBoundsCenter(bounds string) (int, int, error) {
	rect, err := ParseBounds(bounds)
	if err != nil {
		return 0, 0, err
	}
	x, y := rect.Center()
	return x, y, nil
}

func (a *App) loadAndRunSubWorkflow(ctx context.Context, deviceId, workflowID string, depth int, vars map[string]string) error {
	workflowsPath := a.getWorkflowsPath()

	safeName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(workflowID, "_")
	filePath := filepath.Join(workflowsPath, safeName+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	var subWorkflow Workflow
	if err := json.Unmarshal(data, &subWorkflow); err != nil {
		return fmt.Errorf("failed to parse sub-workflow: %w", err)
	}

	wailsRuntime.EventsEmit(a.ctx, "workflow-started", map[string]interface{}{
		"deviceId":     deviceId,
		"workflowName": subWorkflow.Name,
		"workflowId":   subWorkflow.ID,
		"steps":        len(subWorkflow.Steps),
	})

	// Inherit variables from parent but also load sub-workflow defaults
	subVars := make(map[string]string)
	for k, v := range vars {
		subVars[k] = v
	}
	if subWorkflow.Variables != nil {
		for k, v := range subWorkflow.Variables {
			subVars[k] = v
		}
	}

	err = a.runWorkflowInternal(ctx, deviceId, subWorkflow, depth+1, subVars)

	if err != nil && err != context.Canceled {
		wailsRuntime.EventsEmit(a.ctx, "workflow-error", map[string]interface{}{
			"deviceId":   deviceId,
			"workflowId": subWorkflow.ID,
			"error":      err.Error(),
		})
	}

	wailsRuntime.EventsEmit(a.ctx, "workflow-completed", map[string]interface{}{
		"deviceId":     deviceId,
		"workflowName": subWorkflow.Name,
		"workflowId":   subWorkflow.ID,
	})

	return err
}
