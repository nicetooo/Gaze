package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
)

// registerWorkflowTools registers workflow management tools
func (s *MCPServer) registerWorkflowTools() {
	// workflow_list - List workflows
	s.server.AddTool(
		mcp.NewTool("workflow_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("List all saved workflows"),
		),
		s.handleWorkflowList,
	)

	// workflow_get - Get workflow details
	s.server.AddTool(
		mcp.NewTool("workflow_get",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Get detailed information about a specific workflow including all steps"),
			mcp.WithString("workflow_id",
				mcp.Required(),
				mcp.Description("Workflow ID"),
			),
		),
		s.handleWorkflowGet,
	)

	// workflow_create - Create a new workflow (V2)
	s.server.AddTool(
		mcp.NewTool("workflow_create",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Create a new workflow with the given name and steps (V2 format).

A 'start' node will be auto-added if not present in steps_json. You may include it or omit it.
Steps are automatically linked sequentially via connections.successStepId.

Step JSON format (V2):
{
  "type": "step_type",
  "name": "optional display name",
  "common": { "timeout": 5000, "onError": "stop", "loop": 1, "postDelay": 500 },
  "connections": { "successStepId": "", "errorStepId": "", "trueStepId": "", "falseStepId": "" },
  // Type-specific params (only one based on type):
  "tap": { "x": 540, "y": 960 },
  "swipe": { "x": 540, "y": 1800, "x2": 540, "y2": 600 } or { "direction": "up", "distance": 500 },
  "element": { "selector": {"type":"id","value":"com.app:id/btn"}, "action": "click" },
  "app": { "packageName": "com.example.app", "action": "launch" },
  "branch": { "condition": "exists", "selector": {...} },
  "wait": { "durationMs": 2000 },
  "adb": { "command": "shell input keyevent 4" },
  "script": { "scriptName": "my_script" },
  "variable": { "name": "myVar", "value": "myValue" },
  "readToVariable": { "selector": {"type":"id","value":"com.app:id/text"}, "variableName": "myVar", "attribute": "text", "defaultValue": "" },
  "workflow": { "workflowId": "sub_workflow_id" },
  "session": { "sessionName": "Debug Session", "logcatEnabled": true, "logcatPackageName": "com.example.app", "recordingEnabled": true, "proxyEnabled": true, "monitorEnabled": true, "status": "completed" }
}

CONNECTIONS explained:
- Most steps use: successStepId (on success), errorStepId (on execution error)
- BRANCH step uses: trueStepId (condition true), falseStepId (condition false), errorStepId (execution error)
  IMPORTANT: For branch nodes, DO NOT use successStepId/errorStepId for condition results!
  - trueStepId: next step when condition evaluates to TRUE
  - falseStepId: next step when condition evaluates to FALSE  
  - errorStepId: next step when execution fails (e.g., UI dump error)

CONNECTION LIMITS:
- Incoming: Each node can receive multiple incoming connections (from different nodes)
- Outgoing: Each output handle (success/error/true/false) connects to exactly ONE target node
- Start node: No incoming, only one success output
- Branch node: Multiple incoming, three outputs (true/false/error)
- Normal nodes: Multiple incoming, two outputs (success/error)

Branch conditions: exists, not_exists, text_equals, text_contains, variable_equals

Step types:
- COORDINATE: tap, swipe
- ELEMENT: click_element, long_click_element, input_text, swipe_element, wait_element, wait_gone, assert_element
- APP: launch_app, stop_app, clear_app, open_settings
- KEYS: key_back, key_home, key_recent, key_power, key_volume_up, key_volume_down
- SCREEN: screen_on, screen_off
- CONTROL: wait, adb, set_variable, read_to_variable, branch, run_workflow, script
- SESSION: start_session, end_session

ELEMENT WAIT BEHAVIOR:
All element-based steps automatically wait for element to appear within timeout before performing action:
- click_element: waits for element, then clicks
- long_click_element: waits for element, then long clicks
- input_text: waits for element, then taps and inputs text
- swipe_element: waits for element, then swipes from its center
- wait_element: waits for element to appear
- wait_gone: waits for element to disappear
- assert_element: waits for element (same as wait_element)

read_to_variable also waits for element, then reads attribute value into variable.
If element not found within timeout and defaultValue is set, uses defaultValue and succeeds; otherwise fails.

VARIABLE EXPRESSIONS:
set_variable supports arithmetic expressions after variable substitution:
- Operators: +, -, *, /, % (modulo)
- Parentheses supported for grouping
- Examples: "{{count}} + 1", "{{a}} * {{b}}", "({{x}} + {{y}}) / 2"
- If expression is invalid, the raw string is used as-is

SESSION CONTROL:
- start_session: Creates a new tracking session with optional config (logcat, recording, proxy, monitor).
  The session ID is stored in workflow variable '_sessionId' for later use.
- end_session: Ends the current active session. Uses '_sessionId' variable or falls back to device's active session.
  Status can be: completed, error, cancelled (default: completed).

Element selector types: id, text, contentDesc, className, xpath
Element actions: click, long_click, input, swipe, wait, wait_gone, assert`),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Workflow name"),
			),
			mcp.WithString("description",
				mcp.Description("Workflow description"),
			),
			mcp.WithString("steps_json",
				mcp.Description(`JSON array of V2 workflow steps. Example:
[
  {"type":"launch_app","app":{"packageName":"com.example.app","action":"launch"}},
  {"type":"wait","wait":{"durationMs":2000}},
  {"type":"tap","tap":{"x":540,"y":960}},
  {"type":"swipe","swipe":{"x":540,"y":1800,"x2":540,"y2":600}},
  {"type":"click_element","element":{"selector":{"type":"id","value":"com.app:id/button"},"action":"click"}},
  {"type":"input_text","element":{"selector":{"type":"id","value":"com.app:id/input"},"action":"input","inputText":"Hello"}},
  {"type":"key_back"},
  {"type":"start_session","session":{"sessionName":"Debug","logcatEnabled":true,"recordingEnabled":true}},
  {"type":"end_session","session":{"status":"completed"}}
]`),
			),
			mcp.WithString("variables_json",
				mcp.Description(`JSON object of workflow variables. Example: {"username":"test","password":"123"}
Variables can be used in steps with {{varName}} syntax.`),
			),
		),
		s.handleWorkflowCreate,
	)

	// workflow_delete - Delete a workflow
	s.server.AddTool(
		mcp.NewTool("workflow_delete",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription("Delete a saved workflow (requires confirmation)"),
			mcp.WithString("workflow_id",
				mcp.Required(),
				mcp.Description("Workflow ID to delete"),
			),
		),
		s.handleWorkflowDelete,
	)

	// workflow_update - Update an existing workflow
	s.server.AddTool(
		mcp.NewTool("workflow_update",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Update an existing workflow. Can update name, description, and/or steps.
Only provided fields will be updated. Use workflow_get first to see current state.`),
			mcp.WithString("workflow_id",
				mcp.Required(),
				mcp.Description("Workflow ID to update"),
			),
			mcp.WithString("name",
				mcp.Description("New workflow name (optional)"),
			),
			mcp.WithString("description",
				mcp.Description("New workflow description (optional)"),
			),
			mcp.WithString("steps_json",
				mcp.Description("New steps JSON array (optional, replaces all existing steps). V2 format."),
			),
			mcp.WithString("variables_json",
				mcp.Description(`New workflow variables JSON object (optional). Example: {"username":"test"}`),
			),
		),
		s.handleWorkflowUpdate,
	)

	// workflow_run - Run a workflow
	s.server.AddTool(
		mcp.NewTool("workflow_run",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription("Run a workflow on a device. By default runs asynchronously. Set wait=true to wait for completion."),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to run the workflow on"),
			),
			mcp.WithString("workflow_id",
				mcp.Required(),
				mcp.Description("Workflow ID to run"),
			),
			mcp.WithBoolean("wait",
				mcp.Description("Wait for workflow to complete before returning (default: false). If true, returns final status."),
			),
			mcp.WithString("variables_json",
				mcp.Description(`Runtime variables JSON object to override workflow defaults. Example: {"username":"runtime_user"}
These variables will be merged with workflow's default variables (runtime values take precedence).`),
			),
		),
		s.handleWorkflowRun,
	)

	// workflow_stop - Stop a running workflow
	s.server.AddTool(
		mcp.NewTool("workflow_stop",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Stop a running workflow on a device"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
		),
		s.handleWorkflowStop,
	)

	// workflow_pause - Pause a running workflow
	s.server.AddTool(
		mcp.NewTool("workflow_pause",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Pause a running workflow on a device. The workflow can be resumed later with workflow_resume."),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
		),
		s.handleWorkflowPause,
	)

	// workflow_resume - Resume a paused workflow
	s.server.AddTool(
		mcp.NewTool("workflow_resume",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Resume a paused workflow on a device"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
		),
		s.handleWorkflowResume,
	)

	// workflow_step_next - Execute next step then pause (step-by-step debugging)
	s.server.AddTool(
		mcp.NewTool("workflow_step_next",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription(`Execute the next step of a paused workflow, then automatically pause again.
This enables step-by-step debugging of workflows.

USAGE:
1. Start workflow: workflow_run device_id="xxx" workflow_id="yyy"
2. Pause it: workflow_pause device_id="xxx"
3. Step through: workflow_step_next device_id="xxx" (repeat for each step)
4. Check state: workflow_status device_id="xxx" (see current step and variables)
5. Resume normal: workflow_resume device_id="xxx"

RETURNS:
- Workflow status and progress
- Current/next step information (ID, name, type)
- Current variables state
- Whether workflow is paused`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
		),
		s.handleWorkflowStepNext,
	)

	// workflow_status - Get workflow running status
	s.server.AddTool(
		mcp.NewTool("workflow_status",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Check if a workflow is currently running on a device"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
		),
		s.handleWorkflowStatus,
	)

	// workflow_execute_step - Execute a single workflow step (V2)
	s.server.AddTool(
		mcp.NewTool("workflow_execute_step",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription(`Execute a single workflow step on a device (V2 format).

Step types:
- Coordinate: tap (x,y), swipe (x,y,x2,y2 or direction+distance)
- Element: click_element, long_click_element, input_text, swipe_element, wait_element, wait_gone, assert_element
- App: launch_app, stop_app, clear_app, open_settings
- Keys: key_back, key_home, key_recent, key_power, key_volume_up, key_volume_down
- Screen: screen_on, screen_off
- Control: wait, adb, set_variable, read_to_variable
- Session: start_session, end_session

ELEMENT WAIT BEHAVIOR: All element steps (click_element, long_click_element, input_text, swipe_element, 
wait_element, assert_element) automatically wait for element within timeout before action.
wait_gone waits for element to disappear. read_to_variable also waits, then reads attribute into variable.
If element not found and default_value is set, uses default_value and succeeds.

set_variable supports arithmetic expressions: {{count}} + 1, {{a}} * {{b}}, ({{x}} + {{y}}) / 2

SESSION CONTROL:
- start_session: Start a new tracking session. Optional params: session_name, logcat_enabled, logcat_package, recording_enabled, recording_quality, proxy_enabled, proxy_port, proxy_mitm, monitor_enabled.
- end_session: End the current active session. Optional param: session_status (completed/error/cancelled, default: completed).`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithString("step_type",
				mcp.Required(),
				mcp.Description("Step type (see description for full list)"),
			),
			mcp.WithString("selector_type",
				mcp.Description("Element selector type: id, text, contentDesc, className, xpath"),
			),
			mcp.WithString("selector_value",
				mcp.Description("Element selector value"),
			),
			mcp.WithString("value",
				mcp.Description("Value: text for input, package for app ops, duration(ms) for wait, command for adb"),
			),
			mcp.WithString("variable_name",
				mcp.Description("Variable name for set_variable or read_to_variable step"),
			),
			mcp.WithString("attribute",
				mcp.Description("Attribute to read for read_to_variable: text (default), contentDesc, resourceId, className, bounds"),
			),
			mcp.WithString("default_value",
				mcp.Description("Default value for read_to_variable if element not found within timeout (step succeeds with this value)"),
			),
			mcp.WithString("regex",
				mcp.Description("Optional regex to extract part of the value (for read_to_variable). Use capture group to extract."),
			),
			mcp.WithNumber("timeout",
				mcp.Description("Timeout in milliseconds (default: 5000)"),
			),
			mcp.WithNumber("post_delay",
				mcp.Description("Delay after step in milliseconds (default: 500)"),
			),
			mcp.WithNumber("x",
				mcp.Description("X coordinate for tap/swipe"),
			),
			mcp.WithNumber("y",
				mcp.Description("Y coordinate for tap/swipe"),
			),
			mcp.WithNumber("x2",
				mcp.Description("End X coordinate for swipe"),
			),
			mcp.WithNumber("y2",
				mcp.Description("End Y coordinate for swipe"),
			),
			mcp.WithString("swipe_direction",
				mcp.Description("Swipe direction: up, down, left, right (alternative to x2,y2)"),
			),
			mcp.WithNumber("swipe_distance",
				mcp.Description("Swipe distance in pixels when using direction (default: 500)"),
			),
			mcp.WithString("condition_type",
				mcp.Description("Condition for assert/branch: exists, not_exists, text_equals, text_contains"),
			),
			// Session params (for start_session / end_session)
			mcp.WithString("session_name",
				mcp.Description("Session name for start_session"),
			),
			mcp.WithBoolean("logcat_enabled",
				mcp.Description("Enable logcat capture for start_session"),
			),
			mcp.WithString("logcat_package",
				mcp.Description("Package name to filter logcat for start_session"),
			),
			mcp.WithBoolean("recording_enabled",
				mcp.Description("Enable screen recording for start_session"),
			),
			mcp.WithString("recording_quality",
				mcp.Description("Recording quality for start_session: low, medium, high (default: medium)"),
			),
			mcp.WithBoolean("proxy_enabled",
				mcp.Description("Enable HTTP/HTTPS proxy for start_session"),
			),
			mcp.WithNumber("proxy_port",
				mcp.Description("Proxy port for start_session (default: 8080)"),
			),
			mcp.WithBoolean("proxy_mitm",
				mcp.Description("Enable MITM for HTTPS inspection for start_session"),
			),
			mcp.WithBoolean("monitor_enabled",
				mcp.Description("Enable device state monitoring for start_session"),
			),
			mcp.WithString("session_status",
				mcp.Description("Session end status for end_session: completed, error, cancelled (default: completed)"),
			),
		),
		s.handleWorkflowExecuteStep,
	)
}

// Tool handlers

func (s *MCPServer) handleWorkflowList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflows, err := s.app.LoadWorkflows()
	if err != nil {
		return nil, fmt.Errorf("failed to load workflows: %w", err)
	}

	if len(workflows) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("No workflows found"),
			},
		}, nil
	}

	result := fmt.Sprintf("Found %d workflow(s):\n\n", len(workflows))
	for i, wf := range workflows {
		result += fmt.Sprintf("%d. %s (ID: %s)\n", i+1, wf.Name, wf.ID)
		if wf.Description != "" {
			result += fmt.Sprintf("   Description: %s\n", wf.Description)
		}
		result += fmt.Sprintf("   Steps: %d, Version: %d\n", len(wf.Steps), wf.Version)
		if wf.CreatedAt != "" {
			result += fmt.Sprintf("   Created: %s\n", wf.CreatedAt)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleWorkflowGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	workflowID, ok := args["workflow_id"].(string)
	if !ok || workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}

	workflow, err := s.app.GetWorkflow(workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	result := fmt.Sprintf("Workflow: %s\n", workflow.Name)
	result += fmt.Sprintf("ID: %s\n", workflow.ID)
	result += fmt.Sprintf("Version: %d\n", workflow.Version)
	if workflow.Description != "" {
		result += fmt.Sprintf("Description: %s\n", workflow.Description)
	}
	if workflow.CreatedAt != "" {
		result += fmt.Sprintf("Created: %s\n", workflow.CreatedAt)
	}
	if workflow.UpdatedAt != "" {
		result += fmt.Sprintf("Updated: %s\n", workflow.UpdatedAt)
	}
	result += fmt.Sprintf("\nSteps (%d):\n", len(workflow.Steps))

	for i, step := range workflow.Steps {
		result += fmt.Sprintf("\n%d. [%s] %s\n", i+1, step.Type, step.Name)

		// V2: Show type-specific params
		if step.Tap != nil {
			result += fmt.Sprintf("   Tap: (%d, %d)\n", step.Tap.X, step.Tap.Y)
		}
		if step.Swipe != nil {
			if step.Swipe.Direction != "" {
				result += fmt.Sprintf("   Swipe: %s %dpx\n", step.Swipe.Direction, step.Swipe.Distance)
			} else {
				result += fmt.Sprintf("   Swipe: (%d,%d) -> (%d,%d)\n", step.Swipe.X, step.Swipe.Y, step.Swipe.X2, step.Swipe.Y2)
			}
		}
		if step.Element != nil {
			result += fmt.Sprintf("   Element: %s = %s, Action: %s\n", step.Element.Selector.Type, step.Element.Selector.Value, step.Element.Action)
			if step.Element.InputText != "" {
				result += fmt.Sprintf("   InputText: %s\n", step.Element.InputText)
			}
		}
		if step.App != nil {
			result += fmt.Sprintf("   App: %s (%s)\n", step.App.PackageName, step.App.Action)
		}
		if step.Branch != nil {
			result += fmt.Sprintf("   Branch: condition=%s\n", step.Branch.Condition)
		}
		if step.Wait != nil {
			result += fmt.Sprintf("   Wait: %dms\n", step.Wait.DurationMs)
		}
		if step.ADB != nil {
			result += fmt.Sprintf("   ADB: %s\n", step.ADB.Command)
		}
		if step.Script != nil {
			result += fmt.Sprintf("   Script: %s\n", step.Script.ScriptName)
		}
		if step.Variable != nil {
			result += fmt.Sprintf("   Variable: %s = %s\n", step.Variable.Name, step.Variable.Value)
		}
		if step.ReadToVariable != nil {
			result += fmt.Sprintf("   ReadToVariable: %s=%s -> %s (attr: %s)\n",
				step.ReadToVariable.Selector.Type, step.ReadToVariable.Selector.Value,
				step.ReadToVariable.VariableName, step.ReadToVariable.Attribute)
		}
		if step.Workflow != nil {
			result += fmt.Sprintf("   SubWorkflow: %s\n", step.Workflow.WorkflowId)
		}
		if step.Session != nil {
			if step.Type == "start_session" {
				features := []string{}
				if step.Session.LogcatEnabled {
					features = append(features, "logcat")
				}
				if step.Session.RecordingEnabled {
					features = append(features, "recording")
				}
				if step.Session.ProxyEnabled {
					features = append(features, "proxy")
				}
				if step.Session.MonitorEnabled {
					features = append(features, "monitor")
				}
				result += fmt.Sprintf("   Session: start")
				if step.Session.SessionName != "" {
					result += fmt.Sprintf(" (name: %s)", step.Session.SessionName)
				}
				if len(features) > 0 {
					result += fmt.Sprintf(" [%s]", strings.Join(features, ", "))
				}
				result += "\n"
			} else {
				result += fmt.Sprintf("   Session: end (status: %s)\n", step.Session.Status)
			}
		}

		// Show connections
		if step.Connections.SuccessStepId != "" {
			result += fmt.Sprintf("   -> Success: %s\n", step.Connections.SuccessStepId)
		}
		if step.Connections.ErrorStepId != "" {
			result += fmt.Sprintf("   -> Error: %s\n", step.Connections.ErrorStepId)
		}
		if step.Connections.TrueStepId != "" {
			result += fmt.Sprintf("   -> True: %s\n", step.Connections.TrueStepId)
		}
		if step.Connections.FalseStepId != "" {
			result += fmt.Sprintf("   -> False: %s\n", step.Connections.FalseStepId)
		}

		// Show common config
		if step.Common.Timeout > 0 {
			result += fmt.Sprintf("   Timeout: %dms\n", step.Common.Timeout)
		}
	}

	// Also return JSON for programmatic access
	jsonData, _ := json.MarshalIndent(workflow, "", "  ")
	result += fmt.Sprintf("\n\nJSON:\n%s", string(jsonData))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleWorkflowCreate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("name is required")
	}

	description := ""
	if d, ok := args["description"].(string); ok {
		description = d
	}

	// Parse steps from JSON (V2 format)
	var steps []WorkflowStep
	if stepsJSON, ok := args["steps_json"].(string); ok && stepsJSON != "" {
		if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
			return nil, fmt.Errorf("invalid steps_json format: %w", err)
		}
	}

	// Generate IDs for steps that don't have one
	for i := range steps {
		if steps[i].ID == "" {
			steps[i].ID = fmt.Sprintf("step_%s", uuid.New().String()[:8])
		}
		// Ensure common has default values if not set
		if steps[i].Common.Timeout == 0 && steps[i].Common.OnError == "" {
			steps[i].Common = StepCommon{
				Timeout:   5000,
				OnError:   "stop",
				Loop:      1,
				PostDelay: 500,
			}
		}
		// Set layout positions if not set
		if steps[i].Layout.PosX == 0 && steps[i].Layout.PosY == 0 {
			steps[i].Layout = StepLayout{
				PosX: 20,
				PosY: float64(180 + i*160),
			}
		}
	}

	// Auto-link steps sequentially (if not already linked)
	// Skip branch nodes - they use trueStepId/falseStepId instead of successStepId
	for i := range steps {
		if steps[i].Type == "branch" {
			continue // Branch nodes should not use successStepId
		}
		if steps[i].Connections.SuccessStepId == "" && i < len(steps)-1 {
			steps[i].Connections.SuccessStepId = steps[i+1].ID
		}
	}

	// Check if start node already exists
	hasStart := false
	for _, s := range steps {
		if s.Type == "start" {
			hasStart = true
			break
		}
	}

	// Auto-add start node at the beginning only if not already present
	if !hasStart {
		startStep := WorkflowStep{
			ID:   fmt.Sprintf("step_%s", uuid.New().String()[:8]),
			Type: "start",
			Name: "Start",
			Common: StepCommon{
				OnError: "stop",
				Loop:    1,
			},
			Connections: StepConnections{},
			Layout: StepLayout{
				PosX: 20,
				PosY: 20,
			},
		}
		// Link start node to first step if exists
		if len(steps) > 0 {
			startStep.Connections.SuccessStepId = steps[0].ID
		}
		steps = append([]WorkflowStep{startStep}, steps...)
	}

	// Parse variables if provided
	var variables map[string]string
	if varsJSON, ok := args["variables_json"].(string); ok && varsJSON != "" {
		if err := json.Unmarshal([]byte(varsJSON), &variables); err != nil {
			return nil, fmt.Errorf("invalid variables_json format: %w", err)
		}
	}

	// Create workflow (V2)
	now := time.Now().Format(time.RFC3339)
	workflow := Workflow{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Version:     2, // V2 schema
		Steps:       steps,
		Variables:   variables,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.app.SaveWorkflow(workflow); err != nil {
		return nil, fmt.Errorf("failed to save workflow: %w", err)
	}

	result := fmt.Sprintf("Created workflow '%s' (V2)\n", name)
	result += fmt.Sprintf("ID: %s\n", workflow.ID)
	result += fmt.Sprintf("Steps: %d\n", len(steps))
	if len(variables) > 0 {
		result += fmt.Sprintf("Variables: %d defined\n", len(variables))
		for k := range variables {
			result += fmt.Sprintf("  - %s\n", k)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleWorkflowDelete(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	workflowID, ok := args["workflow_id"].(string)
	if !ok || workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}

	// Get workflow info first
	workflow, err := s.app.GetWorkflow(workflowID)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %w", err)
	}

	if err := s.app.DeleteWorkflow(workflowID); err != nil {
		return nil, fmt.Errorf("failed to delete workflow: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Deleted workflow '%s' (ID: %s)", workflow.Name, workflowID)),
		},
	}, nil
}

func (s *MCPServer) handleWorkflowUpdate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	workflowID, ok := args["workflow_id"].(string)
	if !ok || workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}

	// Get existing workflow
	workflow, err := s.app.GetWorkflow(workflowID)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %w", err)
	}

	updated := false

	// Update name if provided
	if name, ok := args["name"].(string); ok && name != "" {
		workflow.Name = name
		updated = true
	}

	// Update description if provided
	if description, ok := args["description"].(string); ok {
		workflow.Description = description
		updated = true
	}

	// Update steps if provided (V2 format)
	if stepsJSON, ok := args["steps_json"].(string); ok && stepsJSON != "" {
		var steps []WorkflowStep
		if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
			return nil, fmt.Errorf("invalid steps_json format: %w", err)
		}

		// Generate IDs and ensure V2 structure
		for i := range steps {
			if steps[i].ID == "" {
				steps[i].ID = fmt.Sprintf("step_%s", uuid.New().String()[:8])
			}
			if steps[i].Common.Timeout == 0 && steps[i].Common.OnError == "" {
				steps[i].Common = StepCommon{Timeout: 5000, OnError: "stop", Loop: 1, PostDelay: 500}
			}
			if steps[i].Layout.PosX == 0 && steps[i].Layout.PosY == 0 {
				steps[i].Layout = StepLayout{PosX: 20, PosY: float64(180 + i*160)}
			}
		}

		// Auto-link steps sequentially (if not already linked)
		// Skip branch nodes - they use trueStepId/falseStepId instead of successStepId
		for i := range steps {
			if steps[i].Type == "branch" {
				continue // Branch nodes should not use successStepId
			}
			if steps[i].Connections.SuccessStepId == "" && i < len(steps)-1 {
				steps[i].Connections.SuccessStepId = steps[i+1].ID
			}
		}

		// Check if start node exists, if not add it
		hasStart := false
		for _, s := range steps {
			if s.Type == "start" {
				hasStart = true
				break
			}
		}

		if !hasStart {
			startStep := WorkflowStep{
				ID:          fmt.Sprintf("step_%s", uuid.New().String()[:8]),
				Type:        "start",
				Name:        "Start",
				Common:      StepCommon{OnError: "stop", Loop: 1},
				Connections: StepConnections{},
				Layout:      StepLayout{PosX: 20, PosY: 20},
			}
			if len(steps) > 0 {
				startStep.Connections.SuccessStepId = steps[0].ID
			}
			steps = append([]WorkflowStep{startStep}, steps...)
		}

		workflow.Steps = steps
		workflow.Version = 2 // Ensure V2
		updated = true
	}

	// Update variables if provided
	if varsJSON, ok := args["variables_json"].(string); ok && varsJSON != "" {
		var variables map[string]string
		if err := json.Unmarshal([]byte(varsJSON), &variables); err != nil {
			return nil, fmt.Errorf("invalid variables_json format: %w", err)
		}
		workflow.Variables = variables
		updated = true
	}

	if !updated {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("No fields provided to update"),
			},
		}, nil
	}

	// Update timestamp
	workflow.UpdatedAt = time.Now().Format(time.RFC3339)

	if err := s.app.SaveWorkflow(*workflow); err != nil {
		return nil, fmt.Errorf("failed to save workflow: %w", err)
	}

	result := fmt.Sprintf("Updated workflow '%s'\n", workflow.Name)
	result += fmt.Sprintf("ID: %s\n", workflow.ID)
	result += fmt.Sprintf("Steps: %d\n", len(workflow.Steps))
	if len(workflow.Variables) > 0 {
		result += fmt.Sprintf("Variables: %d defined\n", len(workflow.Variables))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleWorkflowRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	workflowID, ok := args["workflow_id"].(string)
	if !ok || workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}
	waitForCompletion, _ := args["wait"].(bool)

	// Check if already running
	if s.app.IsWorkflowRunning(deviceID) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("A workflow is already running on device %s. Use workflow_stop to stop it first.", deviceID)),
			},
		}, nil
	}

	// Find the workflow
	workflow, err := s.app.GetWorkflow(workflowID)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %w", err)
	}

	// Parse and merge runtime variables if provided
	if varsJSON, ok := args["variables_json"].(string); ok && varsJSON != "" {
		var runtimeVars map[string]string
		if err := json.Unmarshal([]byte(varsJSON), &runtimeVars); err != nil {
			return nil, fmt.Errorf("invalid variables_json format: %w", err)
		}
		// Merge runtime variables into workflow (runtime takes precedence)
		if workflow.Variables == nil {
			workflow.Variables = make(map[string]string)
		}
		for k, v := range runtimeVars {
			workflow.Variables[k] = v
		}
	}

	// Get device info to construct Device struct
	devices, err := s.app.GetDevices(false)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	var targetDevice *Device
	for _, d := range devices {
		if d.ID == deviceID {
			dCopy := d
			targetDevice = &dCopy
			break
		}
	}

	if targetDevice == nil {
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}

	startTime := time.Now()

	// Start the workflow
	err = s.app.RunWorkflow(*targetDevice, *workflow)
	if err != nil {
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}

	// If not waiting, return immediately
	if !waitForCompletion {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Started workflow '%s' (V%d) on device %s\n\nWorkflow has %d steps and is running in background.\n\nUse workflow_status to check progress or workflow_stop to stop.", workflow.Name, workflow.Version, deviceID, len(workflow.Steps))),
			},
		}, nil
	}

	// Wait for workflow to complete (poll every 500ms, timeout after 30 minutes)
	maxWait := 30 * time.Minute
	pollInterval := 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
			if !s.app.IsWorkflowRunning(deviceID) {
				// Workflow completed - get the execution result
				result := s.app.GetWorkflowExecutionResult(deviceID)
				duration := time.Since(startTime)

				if result != nil && result.Status == "error" {
					// Return error information to AI
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							mcp.NewTextContent(fmt.Sprintf("Workflow '%s' FAILED after %s\n\nDevice: %s\nStatus: %s\nError: %s\nSteps: %d",
								workflow.Name, duration.Round(time.Millisecond), deviceID, result.Status, result.Error, len(workflow.Steps))),
						},
						IsError: true,
					}, nil
				}

				if result != nil && result.Status == "cancelled" {
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							mcp.NewTextContent(fmt.Sprintf("Workflow '%s' was CANCELLED after %s\n\nDevice: %s\nSteps: %d",
								workflow.Name, duration.Round(time.Millisecond), deviceID, len(workflow.Steps))),
						},
					}, nil
				}

				// Success
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.NewTextContent(fmt.Sprintf("Workflow '%s' completed successfully in %s\n\nDevice: %s\nSteps: %d", workflow.Name, duration.Round(time.Millisecond), deviceID, len(workflow.Steps))),
					},
				}, nil
			}

			if time.Since(startTime) > maxWait {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.NewTextContent(fmt.Sprintf("Workflow '%s' is still running after %s. Use workflow_status to check progress.", workflow.Name, maxWait)),
					},
				}, nil
			}
		}
	}
}

func (s *MCPServer) handleWorkflowStop(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	// Check if running
	if !s.app.IsWorkflowRunning(deviceID) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("No workflow is running on device %s", deviceID)),
			},
		}, nil
	}

	// Get device info
	devices, err := s.app.GetDevices(false)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	var targetDevice *Device
	for _, d := range devices {
		if d.ID == deviceID {
			dCopy := d
			targetDevice = &dCopy
			break
		}
	}

	if targetDevice == nil {
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}

	s.app.StopWorkflow(*targetDevice)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Stopped workflow on device %s", deviceID)),
		},
	}, nil
}

func (s *MCPServer) handleWorkflowPause(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	// Check if running
	if !s.app.IsWorkflowRunning(deviceID) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("No workflow is running on device %s", deviceID)),
			},
		}, nil
	}

	s.app.PauseTask(deviceID)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Paused workflow on device %s. Use workflow_resume to continue.", deviceID)),
		},
	}, nil
}

func (s *MCPServer) handleWorkflowResume(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	// Check if running (paused workflows are still considered "running")
	if !s.app.IsWorkflowRunning(deviceID) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("No workflow is running or paused on device %s", deviceID)),
			},
		}, nil
	}

	s.app.ResumeTask(deviceID)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Resumed workflow on device %s", deviceID)),
		},
	}, nil
}

func (s *MCPServer) handleWorkflowStepNext(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	result, err := s.app.StepNextWorkflow(deviceID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	// Format output
	output := fmt.Sprintf("Step executed on device %s\n\n", deviceID)
	if result != nil {
		output += fmt.Sprintf("Workflow: %s\n", result.WorkflowName)
		output += fmt.Sprintf("Status: %s (paused: %v)\n", result.Status, result.IsPaused)
		output += fmt.Sprintf("Progress: %d / %d steps\n", result.StepsExecuted, result.StepsTotal)

		if result.CurrentStepID != "" {
			output += fmt.Sprintf("\nNext step:\n")
			output += fmt.Sprintf("  ID: %s\n", result.CurrentStepID)
			if result.CurrentStepName != "" {
				output += fmt.Sprintf("  Name: %s\n", result.CurrentStepName)
			}
			if result.CurrentStepType != "" {
				output += fmt.Sprintf("  Type: %s\n", result.CurrentStepType)
			}
		} else {
			output += "\nNext step: (workflow completed)\n"
		}

		if len(result.Variables) > 0 {
			output += "\nVariables:\n"
			for k, v := range result.Variables {
				output += fmt.Sprintf("  %s = %s\n", k, v)
			}
		} else {
			output += "\nVariables: (none)\n"
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(output),
		},
	}, nil
}

func (s *MCPServer) handleWorkflowStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	isRunning := s.app.IsWorkflowRunning(deviceID)
	result := s.app.GetWorkflowExecutionResult(deviceID)

	var statusText string
	if isRunning {
		statusText = fmt.Sprintf("Device %s: workflow is RUNNING", deviceID)
		if result != nil {
			statusText += fmt.Sprintf(" (paused: %v)", result.IsPaused)
			statusText += fmt.Sprintf("\n\nWorkflow: %s", result.WorkflowName)
			statusText += fmt.Sprintf("\nStatus: %s", result.Status)
			statusText += fmt.Sprintf("\nProgress: %d / %d steps", result.StepsExecuted, result.StepsTotal)

			// Show current/next step info
			if result.CurrentStepID != "" {
				statusText += fmt.Sprintf("\n\nCurrent/Next step:")
				statusText += fmt.Sprintf("\n  ID: %s", result.CurrentStepID)
				if result.CurrentStepName != "" {
					statusText += fmt.Sprintf("\n  Name: %s", result.CurrentStepName)
				}
				if result.CurrentStepType != "" {
					statusText += fmt.Sprintf("\n  Type: %s", result.CurrentStepType)
				}
			}

			// Show variables
			if len(result.Variables) > 0 {
				statusText += "\n\nVariables:"
				for k, v := range result.Variables {
					statusText += fmt.Sprintf("\n  %s = %s", k, v)
				}
			}
		}
	} else if result != nil {
		// Show last execution result
		statusText = fmt.Sprintf("Device %s: workflow is IDLE\n\nLast execution:\n- Workflow: %s\n- Status: %s\n- Duration: %dms",
			deviceID, result.WorkflowName, result.Status, result.Duration)
		if result.Error != "" {
			statusText += fmt.Sprintf("\n- Error: %s", result.Error)
		}
		statusText += fmt.Sprintf("\n- Steps executed: %d / %d", result.StepsExecuted, result.StepsTotal)

		// Show final variables
		if len(result.Variables) > 0 {
			statusText += "\n\nFinal variables:"
			for k, v := range result.Variables {
				statusText += fmt.Sprintf("\n  %s = %s", k, v)
			}
		}
	} else {
		statusText = fmt.Sprintf("Device %s: workflow is IDLE (no previous execution)", deviceID)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(statusText),
		},
	}, nil
}

func (s *MCPServer) handleWorkflowExecuteStep(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	stepType, ok := args["step_type"].(string)
	if !ok || stepType == "" {
		return nil, fmt.Errorf("step_type is required")
	}

	// Validate step type
	validTypes := map[string]bool{
		// Coordinate operations
		"tap": true, "swipe": true,
		// Element operations
		"click_element": true, "long_click_element": true, "input_text": true,
		"swipe_element": true, "wait_element": true, "wait_gone": true, "assert_element": true,
		// App operations
		"launch_app": true, "stop_app": true, "clear_app": true, "open_settings": true,
		// Key events
		"key_back": true, "key_home": true, "key_recent": true, "key_power": true,
		"key_volume_up": true, "key_volume_down": true,
		// Screen control
		"screen_on": true, "screen_off": true,
		// Control flow
		"wait": true, "adb": true, "set_variable": true, "read_to_variable": true,
		// Session control
		"start_session": true, "end_session": true,
	}
	if !validTypes[stepType] {
		return nil, fmt.Errorf("invalid step_type '%s'", stepType)
	}

	// Build the V2 step
	step := WorkflowStep{
		ID:   fmt.Sprintf("mcp_step_%s", uuid.New().String()[:8]),
		Type: stepType,
		Name: fmt.Sprintf("MCP %s step", stepType),
		Common: StepCommon{
			Timeout:   5000,
			OnError:   "stop",
			Loop:      1,
			PostDelay: 500,
		},
		Connections: StepConnections{},
	}

	// Handle timeout
	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		step.Common.Timeout = int(timeout)
	}

	// Handle post delay
	if postDelay, ok := args["post_delay"].(float64); ok && postDelay >= 0 {
		step.Common.PostDelay = int(postDelay)
	}

	// Build type-specific params based on step type
	value, _ := args["value"].(string)

	switch stepType {
	case "tap":
		x, _ := args["x"].(float64)
		y, _ := args["y"].(float64)
		if x == 0 && y == 0 {
			return nil, fmt.Errorf("x and y coordinates are required for tap")
		}
		step.Tap = &TapParams{X: int(x), Y: int(y)}

	case "swipe":
		x, _ := args["x"].(float64)
		y, _ := args["y"].(float64)
		if x == 0 && y == 0 {
			return nil, fmt.Errorf("x and y coordinates are required for swipe")
		}
		step.Swipe = &SwipeParams{X: int(x), Y: int(y)}

		x2, hasX2 := args["x2"].(float64)
		y2, hasY2 := args["y2"].(float64)
		direction, hasDir := args["swipe_direction"].(string)

		if hasX2 && hasY2 {
			step.Swipe.X2 = int(x2)
			step.Swipe.Y2 = int(y2)
		} else if hasDir && direction != "" {
			step.Swipe.Direction = direction
			if dist, ok := args["swipe_distance"].(float64); ok && dist > 0 {
				step.Swipe.Distance = int(dist)
			} else {
				step.Swipe.Distance = 500
			}
			step.Swipe.Duration = 300
		} else {
			return nil, fmt.Errorf("either x2,y2 or swipe_direction is required for swipe")
		}

	case "click_element", "long_click_element", "wait_element", "wait_gone", "assert_element":
		selectorType, _ := args["selector_type"].(string)
		selectorValue, _ := args["selector_value"].(string)
		if selectorType == "" || selectorValue == "" {
			return nil, fmt.Errorf("selector_type and selector_value are required for %s", stepType)
		}

		action := "click"
		if stepType == "long_click_element" {
			action = "long_click"
		} else if stepType == "wait_element" {
			action = "wait"
		} else if stepType == "wait_gone" {
			action = "wait_gone"
		} else if stepType == "assert_element" {
			action = "assert"
		}

		step.Element = &ElementParams{
			Selector: ElementSelector{Type: selectorType, Value: selectorValue},
			Action:   action,
		}

	case "input_text":
		selectorType, _ := args["selector_type"].(string)
		selectorValue, _ := args["selector_value"].(string)
		if selectorType == "" || selectorValue == "" {
			return nil, fmt.Errorf("selector_type and selector_value are required for input_text")
		}
		step.Element = &ElementParams{
			Selector:  ElementSelector{Type: selectorType, Value: selectorValue},
			Action:    "input",
			InputText: value,
		}

	case "swipe_element":
		selectorType, _ := args["selector_type"].(string)
		selectorValue, _ := args["selector_value"].(string)
		if selectorType == "" || selectorValue == "" {
			return nil, fmt.Errorf("selector_type and selector_value are required for swipe_element")
		}
		direction, _ := args["swipe_direction"].(string)
		if direction == "" {
			direction = value // Allow direction in value field for compatibility
		}
		step.Element = &ElementParams{
			Selector:      ElementSelector{Type: selectorType, Value: selectorValue},
			Action:        "swipe",
			SwipeDir:      direction,
			SwipeDistance: 500,
			SwipeDuration: 300,
		}
		if dist, ok := args["swipe_distance"].(float64); ok && dist > 0 {
			step.Element.SwipeDistance = int(dist)
		}

	case "launch_app", "stop_app", "clear_app", "open_settings":
		if value == "" {
			return nil, fmt.Errorf("value (package name) is required for %s", stepType)
		}
		action := "launch"
		if stepType == "stop_app" {
			action = "stop"
		} else if stepType == "clear_app" {
			action = "clear"
		} else if stepType == "open_settings" {
			action = "settings"
		}
		step.App = &AppParams{PackageName: value, Action: action}

	case "wait":
		duration := 1000
		if value != "" {
			fmt.Sscanf(value, "%d", &duration)
		}
		step.Wait = &WaitParams{DurationMs: duration}

	case "adb":
		if value == "" {
			return nil, fmt.Errorf("value (adb command) is required for adb step")
		}
		step.ADB = &ADBParams{Command: value}

	case "set_variable":
		varName, _ := args["variable_name"].(string)
		if varName == "" {
			varName = step.Name
		}
		step.Variable = &VariableParams{Name: varName, Value: value}

	case "read_to_variable":
		selectorType, _ := args["selector_type"].(string)
		selectorValue, _ := args["selector_value"].(string)
		if selectorType == "" || selectorValue == "" {
			return nil, fmt.Errorf("selector_type and selector_value are required for read_to_variable")
		}
		varName, _ := args["variable_name"].(string)
		if varName == "" {
			return nil, fmt.Errorf("variable_name is required for read_to_variable")
		}
		attribute, _ := args["attribute"].(string)
		if attribute == "" {
			attribute = "text"
		}
		defaultValue, _ := args["default_value"].(string)
		regexPattern, _ := args["regex"].(string)
		step.ReadToVariable = &ReadToVariableParams{
			Selector:     ElementSelector{Type: selectorType, Value: selectorValue},
			VariableName: varName,
			Attribute:    attribute,
			DefaultValue: defaultValue,
			Regex:        regexPattern,
		}

	case "start_session":
		sessionParams := &SessionParams{}
		if name, ok := args["session_name"].(string); ok && name != "" {
			sessionParams.SessionName = name
		}
		if enabled, ok := args["logcat_enabled"].(bool); ok {
			sessionParams.LogcatEnabled = enabled
		}
		if pkg, ok := args["logcat_package"].(string); ok {
			sessionParams.LogcatPackageName = pkg
		}
		if enabled, ok := args["recording_enabled"].(bool); ok {
			sessionParams.RecordingEnabled = enabled
		}
		if quality, ok := args["recording_quality"].(string); ok {
			sessionParams.RecordingQuality = quality
		}
		if enabled, ok := args["proxy_enabled"].(bool); ok {
			sessionParams.ProxyEnabled = enabled
		}
		if port, ok := args["proxy_port"].(float64); ok {
			sessionParams.ProxyPort = int(port)
		}
		if mitm, ok := args["proxy_mitm"].(bool); ok {
			sessionParams.ProxyMitmEnabled = mitm
		}
		if enabled, ok := args["monitor_enabled"].(bool); ok {
			sessionParams.MonitorEnabled = enabled
		}
		step.Session = sessionParams

	case "end_session":
		sessionParams := &SessionParams{}
		if status, ok := args["session_status"].(string); ok && status != "" {
			sessionParams.Status = status
		} else {
			sessionParams.Status = "completed"
		}
		step.Session = sessionParams
	}

	// Execute the step
	startTime := time.Now()
	err := s.app.ExecuteSingleWorkflowStep(deviceID, step)
	duration := time.Since(startTime)

	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Step execution failed after %dms: %v", duration.Milliseconds(), err)),
			},
		}, nil
	}

	result := fmt.Sprintf("Step executed successfully in %dms\n", duration.Milliseconds())
	result += fmt.Sprintf("Type: %s\n", stepType)

	// Show type-specific info
	if step.Tap != nil {
		result += fmt.Sprintf("Coordinates: (%d, %d)\n", step.Tap.X, step.Tap.Y)
	}
	if step.Swipe != nil {
		if step.Swipe.Direction != "" {
			result += fmt.Sprintf("Direction: %s, Distance: %dpx\n", step.Swipe.Direction, step.Swipe.Distance)
		} else {
			result += fmt.Sprintf("From: (%d,%d) To: (%d,%d)\n", step.Swipe.X, step.Swipe.Y, step.Swipe.X2, step.Swipe.Y2)
		}
	}
	if step.Element != nil {
		result += fmt.Sprintf("Selector: %s = %s\n", step.Element.Selector.Type, step.Element.Selector.Value)
	}
	if step.App != nil {
		result += fmt.Sprintf("Package: %s\n", step.App.PackageName)
	}
	if step.ADB != nil {
		result += fmt.Sprintf("Command: %s\n", step.ADB.Command)
	}
	if step.Session != nil {
		if stepType == "start_session" {
			if step.Session.SessionName != "" {
				result += fmt.Sprintf("Session name: %s\n", step.Session.SessionName)
			}
			features := []string{}
			if step.Session.LogcatEnabled {
				features = append(features, "logcat")
			}
			if step.Session.RecordingEnabled {
				features = append(features, "recording")
			}
			if step.Session.ProxyEnabled {
				features = append(features, "proxy")
			}
			if step.Session.MonitorEnabled {
				features = append(features, "monitor")
			}
			if len(features) > 0 {
				result += fmt.Sprintf("Enabled: %s\n", strings.Join(features, ", "))
			}
		} else {
			result += fmt.Sprintf("Status: %s\n", step.Session.Status)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}
