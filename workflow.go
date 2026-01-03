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

			wailsRuntime.EventsEmit(a.ctx, "workflow-completed", map[string]interface{}{
				"deviceId":     deviceId,
				"workflowName": workflow.Name,
			})
		}()

		wailsRuntime.EventsEmit(a.ctx, "workflow-started", map[string]interface{}{
			"deviceId":     deviceId,
			"workflowName": workflow.Name,
			"steps":        len(workflow.Steps),
		})

		// Build ID->Step map for graph navigation
		fmt.Printf("--- WORKFLOW EXECUTION: %s ---\n", workflow.Name)
		stepMap := make(map[string]*WorkflowStep)
		var startStep *WorkflowStep
		for i := range workflow.Steps {
			s := &workflow.Steps[i]
			fmt.Printf("  [%d] ID=%s Type=%s Next=%s TrueNext=%s FalseNext=%s\n",
				i, s.ID, s.Type, s.NextStepId, s.TrueStepId, s.FalseStepId)
			stepMap[s.ID] = s
			if s.Type == "start" {
				startStep = s
			}
		}

		// Find start node
		if startStep == nil {
			wailsRuntime.EventsEmit(a.ctx, "workflow-error", map[string]interface{}{
				"deviceId": deviceId,
				"error":    "Workflow must have a 'start' node",
			})
			return
		}

		// Get the first actual step (what start node points to)
		currentStepID := startStep.NextStepId
		if currentStepID == "" {
			// No steps after start - workflow is empty, just complete successfully
			fmt.Printf(">>> Workflow has no steps after Start, completing immediately\n")
			return
		}

		fmt.Printf(">>> Starting from: %s -> %s\n", startStep.ID, currentStepID)

		// Emit event for Start node to show it's running
		wailsRuntime.EventsEmit(a.ctx, "workflow-step-running", map[string]interface{}{
			"deviceId":  deviceId,
			"stepIndex": 0,
			"stepId":    startStep.ID,
			"stepName":  startStep.Name,
			"stepType":  "start",
		})
		time.Sleep(300 * time.Millisecond) // Brief highlight for Start node

		// Graph-based execution loop
		executedCount := 0
		maxSteps := 1000 // Safety limit to prevent infinite loops

		for currentStepID != "" && executedCount < maxSteps {
			step, exists := stepMap[currentStepID]
			if !exists {
				fmt.Printf("[Workflow] Step not found: %s\n", currentStepID)
				break
			}

			executedCount++
			fmt.Printf(">>> Executing Step [%d] %s (%s)\n", executedCount, step.ID, step.Type)

			select {
			case <-ctx.Done():
				return
			default:
			}

			// Check pause (reusing automation.go logic)
			a.checkPause(deviceId)

			wailsRuntime.EventsEmit(a.ctx, "workflow-step-running", map[string]interface{}{
				"deviceId":  deviceId,
				"stepIndex": executedCount - 1,
				"stepId":    step.ID,
				"stepName":  step.Name,
				"stepType":  step.Type,
			})

			loopCount := step.Loop
			if loopCount < 1 {
				loopCount = 1
			}

			var stepResult bool = true

			for l := 0; l < loopCount; l++ {
				var err error
				stepResult, err = a.runWorkflowStep(ctx, deviceId, *step, l+1, loopCount, 0)
				if err != nil {
					// Handle error
					if step.OnError == "continue" {
						fmt.Printf("[Workflow] Step failed but continuing: %v\n", err)
						continue
					}

					wailsRuntime.EventsEmit(a.ctx, "workflow-error", map[string]interface{}{
						"deviceId": deviceId,
						"error":    err.Error(),
					})
					return // Stop execution
				}
				// Check cancel between loops
				select {
				case <-ctx.Done():
					return
				default:
				}
			}

			// Post Delay
			if step.PostDelay > 0 {
				time.Sleep(time.Duration(step.PostDelay) * time.Millisecond)
			}

			// Determine Next Step based on graph edges
			nextStepID := ""

			if step.Type == "branch" {
				// Conditional branch: use result to determine path
				if stepResult {
					nextStepID = step.TrueStepId
					fmt.Printf(">>> Branch result: TRUE -> %s\n", nextStepID)
				} else {
					nextStepID = step.FalseStepId
					fmt.Printf(">>> Branch result: FALSE -> %s\n", nextStepID)
				}
			} else {
				// Normal step: follow nextStepId
				nextStepID = step.NextStepId
			}

			if nextStepID == "" {
				fmt.Printf(">>> End of workflow (no next step)\n")
			}

			currentStepID = nextStepID
		}

		if executedCount >= maxSteps {
			wailsRuntime.EventsEmit(a.ctx, "workflow-error", map[string]interface{}{
				"deviceId": deviceId,
				"error":    "Workflow exceeded maximum step limit (possible infinite loop)",
			})
		}
	}()

	return nil
}

// Updated signature to return (result, error)
func (a *App) runWorkflowStep(ctx context.Context, deviceId string, step WorkflowStep, _currentLoop, _totalLoops, depth int) (bool, error) {
	// Recursion guard
	if depth > 10 {
		return false, fmt.Errorf("maximum workflow nesting depth exceeded")
	}

	switch step.Type {
	case "wait":
		duration := 1000
		if val, err := strconv.Atoi(step.Value); err == nil {
			duration = val
		}
		time.Sleep(time.Duration(duration) * time.Millisecond)

	case "adb":
		// Raw ADB Command
		if _, err := a.RunAdbCommand(deviceId, step.Value); err != nil {
			return false, fmt.Errorf("adb command failed: %w", err)
		}

	case "run_workflow":
		return true, a.loadAndRunSubWorkflow(ctx, deviceId, step.Value, depth)

	case "branch":
		// 1. Check condition (Selector)
		timeout := 2000 // Default 2s check
		if step.Timeout > 0 {
			timeout = step.Timeout
		}

		found := false
		startTime := time.Now()

		// Polling for element presence
		for {
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			default:
			}

			// Capture Hierarchy takes time, so we just check once per loop
			hierarchy, err := a.GetUIHierarchy(deviceId)
			if err == nil && step.Selector != nil {
				if step.Selector.Type == "xpath" {
					results := a.SearchElementsXPath(hierarchy.Root, step.Selector.Value)
					if len(results) > 0 {
						found = true
						break
					}
				} else {
					if node := a.findElementNode(hierarchy.Root, step.Selector.Type, step.Selector.Value); node != nil {
						found = true
						break
					}
				}
			}

			if time.Since(startTime) > time.Duration(timeout)*time.Millisecond {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

		// 2. Determine functionality based on configuration
		isGraphMode := step.TrueStepId != "" || step.FalseStepId != ""

		if isGraphMode {
			// Graph Mode: Just return the result, navigation layout handled by caller
			return found, nil
		}

		// Legacy Mode: Nested Sub-Workflow Call
		// Parse Value to get branch targets
		var targets map[string]string
		if step.Value != "" {
			_ = json.Unmarshal([]byte(step.Value), &targets)
		}

		targetID := ""
		if found {
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
		return true, a.loadAndRunSubWorkflow(ctx, deviceId, targetID, depth)

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

	case "click_element", "long_click_element", "input_text", "assert_element", "wait_element", "wait_gone":
		return true, a.handleElementAction(ctx, deviceId, step)

	case "swipe_element":
		return false, fmt.Errorf("swipe_element not fully implemented yet")

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
	// ... (No change needed here as it returns error)
	// Wait, I am not rewriting this function, so I just need to make sure I don't lose it if I overwrite.
	// I AM overwriting the file. I MUST include all content.
	// I'll reuse the existing content for handleElementAction using "view_file" content I read earlier.
	// Copied from Step 536.

	timeout := 10000
	if step.Timeout > 0 {
		timeout = step.Timeout
	}

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var foundNode *UINode

		if step.Selector != nil {
			if step.Selector.Type == "bounds" {
				foundNode = &UINode{Bounds: step.Selector.Value}
			} else {
				hierarchy, err := a.GetUIHierarchy(deviceId)
				if err != nil {
					time.Sleep(1 * time.Second)
					if time.Since(startTime) > time.Duration(timeout)*time.Millisecond {
						return fmt.Errorf("UI dump failed: %w", err)
					}
					continue
				}

				if step.Selector.Type == "xpath" {
					results := a.SearchElementsXPath(hierarchy.Root, step.Selector.Value)
					if len(results) > 0 {
						idx := step.Selector.Index
						if idx < len(results) {
							foundNode = results[idx].Node
						}
					}
				} else {
					foundNode = a.findElementNode(hierarchy.Root, step.Selector.Type, step.Selector.Value)
				}
			}
		}

		if step.Type == "wait_gone" {
			if foundNode == nil {
				return nil
			}
			if time.Since(startTime) > time.Duration(timeout)*time.Millisecond {
				return fmt.Errorf("timeout waiting for element to disappear")
			}
			time.Sleep(1 * time.Second)
			continue
		}

		if foundNode != nil {
			if step.Type == "wait_element" || step.Type == "assert_element" {
				return nil
			}

			bounds := foundNode.Bounds
			x, y, err := parseBoundsCenter(bounds)
			if err != nil {
				return fmt.Errorf("invalid bounds: %s", bounds)
			}

			if step.Type == "click_element" {
				_, err := a.RunAdbCommand(deviceId, fmt.Sprintf("shell input tap %d %d", x, y))
				return err
			}

			if step.Type == "long_click_element" {
				_, err := a.RunAdbCommand(deviceId, fmt.Sprintf("shell input swipe %d %d %d %d 1000", x, y, x, y))
				return err
			}

			if step.Type == "input_text" {
				a.RunAdbCommand(deviceId, fmt.Sprintf("shell input tap %d %d", x, y))
				time.Sleep(500 * time.Millisecond)

				text := step.Value
				text = strings.ReplaceAll(text, " ", "%s")
				text = strings.ReplaceAll(text, "'", "\\'")

				_, err := a.RunAdbCommand(deviceId, fmt.Sprintf("shell input text '%s'", text))
				return err
			}

			return nil
		}

		if time.Since(startTime) > time.Duration(timeout)*time.Millisecond {
			return fmt.Errorf("element not found within timeout")
		}

		time.Sleep(1 * time.Second)
	}
}

// Helper to find node and return pointer
func (a *App) findElementNode(node *UINode, checkType, checkValue string) *UINode {
	if node == nil {
		return nil
	}

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
		return node
	}

	for i := range node.Nodes {
		found := a.findElementNode(&node.Nodes[i], checkType, checkValue)
		if found != nil {
			return found
		}
	}

	return nil
}

func parseBoundsCenter(bounds string) (int, int, error) {
	re := regexp.MustCompile(`\[(\d+),(\d+)\]\[(\d+),(\d+)\]`)
	matches := re.FindStringSubmatch(bounds)
	if len(matches) != 5 {
		return 0, 0, fmt.Errorf("invalid bounds format")
	}

	x1, _ := strconv.Atoi(matches[1])
	y1, _ := strconv.Atoi(matches[2])
	x2, _ := strconv.Atoi(matches[3])
	y2, _ := strconv.Atoi(matches[4])

	centerX := x1 + (x2-x1)/2
	centerY := y1 + (y2-y1)/2

	return centerX, centerY, nil
}

func (a *App) loadAndRunSubWorkflow(ctx context.Context, deviceId, workflowID string, depth int) error {
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

	// Make SubWorkflows ALSO support graph execution?
	// YES. We can reuse RunWorkflow logic?
	// But RunWorkflow spawns a goroutine and emits events which might confuse the parent flow events?
	// AND RunWorkflow takes a 'Device' object.
	// Ideally we refactor the loop logic into 'runWorkflowLoop'.
	// But for now, to save time, I will keep loadAndRunSubWorkflow simple (Linear + Legacy).
	// OR I can duplicate the loop logic here.
	// Given user requested "Not nested", supporting graph inside nested might not be priority.
	// But consistency is good.
	// Let's stick to Linear for SubWorkflow for now to avoid massive refactor (extracting loop).
	// Main Workflow supports Graph. SubWorkflows run linearly (unless I refactor).
	// User said "Instead of nested call", meaning they will use the MAIN graph.
	// So SubWorkflow logic being linear is Acceptable for legacy support.
	// I will just update the recursive call to match signature.

	for _, subStep := range subWorkflow.Steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		subLoopCount := subStep.Loop
		if subLoopCount < 1 {
			subLoopCount = 1
		}

		for l := 0; l < subLoopCount; l++ {
			// Recursive call - discard bool result
			if _, err := a.runWorkflowStep(ctx, deviceId, subStep, l+1, subLoopCount, depth+1); err != nil {
				if subStep.OnError == "continue" {
					continue
				}
				return err
			}
		}

		if subStep.PostDelay > 0 {
			time.Sleep(time.Duration(subStep.PostDelay) * time.Millisecond)
		}
	}
	return nil
}
