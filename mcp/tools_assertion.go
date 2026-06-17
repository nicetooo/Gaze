package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerAssertionTools registers individual assertion and assertion set management tools
func (s *MCPServer) registerAssertionTools() {

	// ============================================================
	// Individual Assertion Tools
	// ============================================================

	// assertion_list - List stored assertions
	s.server.AddTool(
		mcp.NewTool("assertion_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`List stored assertions.

Returns an array of assertions with their IDs, names, types, and timestamps.
Assertions can be filtered by session, device, or template status.

PARAMETERS:
  session_id: Filter by session ID (optional)
  device_id: Filter by device ID (optional)
  templates_only: If true, only return template assertions (default: false)
  limit: Maximum number of assertions to return (default: 100)

ASSERTION TYPES:
  - exists: Assert that matching events exist
  - not_exists: Assert that no matching events exist
  - count: Assert event count is within min/max range

RETURNS: Array of stored assertions with metadata.`),
			mcp.WithString("session_id",
				mcp.Description("Filter by session ID (optional)"),
			),
			mcp.WithString("device_id",
				mcp.Description("Filter by device ID (optional)"),
			),
			mcp.WithBoolean("templates_only",
				mcp.Description("Only return template assertions (default: false)"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of assertions to return (default: 100)"),
			),
		),
		s.handleAssertionList,
	)

	// assertion_create - Create a new assertion
	s.server.AddTool(
		mcp.NewTool("assertion_create",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Create a new assertion for validating session events.

An assertion defines criteria to match events and expected results.

PARAMETERS:
  assertion_json: JSON string defining the assertion. Structure:
    {
      "id": "unique_id",
      "name": "Display name",
      "type": "exists|not_exists|count|sequence",
      "criteria": {
        "types": ["event_type1", "event_type2"],  // event types to match
        "titleMatch": "regex pattern"               // regex to match event titles
      },
      "expected": {
        "exists": true,              // for exists/not_exists type
        "minCount": 1,               // for count type
        "maxCount": 10,              // for count type
        "sequence": [{...criteria}], // for sequence type: list of per-step criteria
        "ordered": true              // for sequence type: true=strict order, false=all present in any order
      }
    }
  save_as_template: Save as a reusable template (default: false)

EXAMPLES:
  Check API calls exist:
    assertion_json: '{"id":"api-check","name":"API calls exist","type":"exists","criteria":{"types":["network_request"]},"expected":{"exists":true}}'

  No error logs occurred (prefer levels over titleMatch for logcat: bursts are
  merged into 'logcat_aggregated' events whose title is a summary like
  "Logcat Output (N entries) - <tag>", not the message text; a criteria type
  of "logcat" automatically also matches "logcat_aggregated"):
    assertion_json: '{"id":"no-err","name":"No error logs","type":"not_exists","criteria":{"types":["logcat"],"levels":["error","fatal"]},"expected":{"exists":false}}'

  Event count in range:
    assertion_json: '{"id":"count-check","name":"3-10 network requests","type":"count","criteria":{"types":["network_request"]},"expected":{"minCount":3,"maxCount":10}}'`),
			mcp.WithString("assertion_json",
				mcp.Required(),
				mcp.Description("JSON string defining the assertion (see description for structure)"),
			),
			mcp.WithBoolean("save_as_template",
				mcp.Description("Save as a reusable template (default: false)"),
			),
		),
		s.handleAssertionCreate,
	)

	// assertion_get - Get assertion details
	s.server.AddTool(
		mcp.NewTool("assertion_get",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Get detailed information about a specific stored assertion including its
name, type, criteria, expected values, and timestamps.`),
			mcp.WithString("assertion_id",
				mcp.Required(),
				mcp.Description("ID of the assertion to retrieve"),
			),
		),
		s.handleAssertionGet,
	)

	// assertion_update - Update an existing assertion
	s.server.AddTool(
		mcp.NewTool("assertion_update",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Update an existing stored assertion. The assertion JSON replaces all fields.

PARAMETERS:
  assertion_id: ID of the assertion to update
  assertion_json: New JSON definition (same structure as assertion_create)`),
			mcp.WithString("assertion_id",
				mcp.Required(),
				mcp.Description("ID of the assertion to update"),
			),
			mcp.WithString("assertion_json",
				mcp.Required(),
				mcp.Description("New JSON string defining the assertion"),
			),
		),
		s.handleAssertionUpdate,
	)

	// assertion_delete - Delete an assertion
	s.server.AddTool(
		mcp.NewTool("assertion_delete",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription(`Delete a stored assertion by ID. If this assertion is part of assertion sets,
it will be removed from those sets as well.`),
			mcp.WithString("assertion_id",
				mcp.Required(),
				mcp.Description("ID of the assertion to delete"),
			),
		),
		s.handleAssertionDelete,
	)

	// assertion_execute - Execute an assertion against a session
	s.server.AddTool(
		mcp.NewTool("assertion_execute",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Execute a stored assertion against an active session.

Evaluates the assertion's criteria against all events in the session and returns
whether the assertion passed or failed, along with details.

PARAMETERS:
  assertion_id: ID of the stored assertion to execute
  session_id: Session ID to evaluate against
  device_id: Device ID for context

RETURNS: Assertion result with pass/fail status, message, matched event count, and duration.`),
			mcp.WithString("assertion_id",
				mcp.Required(),
				mcp.Description("ID of the stored assertion to execute"),
			),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID to evaluate against"),
			),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID for context"),
			),
		),
		s.handleAssertionExecute,
	)

	// assertion_quick_no_errors - Quick assertion: no errors in session
	s.server.AddTool(
		mcp.NewTool("assertion_quick_no_errors",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Quick assertion that verifies no error or fatal events occurred in the session.

This is a convenience shortcut that creates and executes an assertion checking
for the absence of error-level and fatal-level events.

PARAMETERS:
  session_id: Session ID to check
  device_id: Device ID for context

RETURNS: Assertion result - PASS if no errors found, FAIL if errors exist.`),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID to check for errors"),
			),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID for context"),
			),
		),
		s.handleAssertionQuickNoErrors,
	)

	// assertion_quick_no_crashes - Quick assertion: no crashes in session
	s.server.AddTool(
		mcp.NewTool("assertion_quick_no_crashes",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Quick assertion that verifies no app crashes or ANRs occurred in the session.

This is a convenience shortcut that creates and executes an assertion checking
for the absence of app_crash and app_anr events.

PARAMETERS:
  session_id: Session ID to check
  device_id: Device ID for context

RETURNS: Assertion result - PASS if no crashes found, FAIL if crashes exist.`),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID to check for crashes"),
			),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID for context"),
			),
		),
		s.handleAssertionQuickNoCrashes,
	)

	// ============================================================
	// Assertion Set Tools
	// ============================================================

	// assertion_set_create - Create a new assertion set
	s.server.AddTool(
		mcp.NewTool("assertion_set_create",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Create a new assertion set (a collection of assertions to execute together).

An assertion set groups multiple existing assertions by their IDs. When executed,
all assertions in the set run in parallel and a combined result is returned.

PARAMETERS:
  name: Display name for the assertion set
  description: Optional description of what this set tests
  assertion_ids: JSON array of assertion IDs to include (e.g., '["id1","id2","id3"]')

EXAMPLES:
  Create a basic set:
    name: "App Launch Checks"
    assertion_ids: '["abc123", "def456"]'

  Create with description:
    name: "Network Validation"
    description: "Verify all API calls return 200 and complete within 2s"
    assertion_ids: '["net-check-1", "net-check-2", "net-check-3"]'

RETURNS: The ID of the newly created assertion set.`),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Name of the assertion set"),
			),
			mcp.WithString("description",
				mcp.Description("Optional description of what this set tests"),
			),
			mcp.WithString("assertion_ids",
				mcp.Required(),
				mcp.Description("JSON array of assertion IDs to include (e.g., '[\"id1\",\"id2\"]')"),
			),
		),
		s.handleAssertionSetCreate,
	)

	// assertion_set_update - Update an existing assertion set
	s.server.AddTool(
		mcp.NewTool("assertion_set_update",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Update an existing assertion set. All fields are replaced with the new values.

PARAMETERS:
  id: The assertion set ID to update
  name: New name for the set
  description: New description (optional)
  assertion_ids: New JSON array of assertion IDs

EXAMPLE:
  id: "set-abc123"
  name: "Updated App Launch Checks"
  assertion_ids: '["id1", "id2", "id4"]'`),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the assertion set to update"),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("New name for the assertion set"),
			),
			mcp.WithString("description",
				mcp.Description("New description (optional)"),
			),
			mcp.WithString("assertion_ids",
				mcp.Required(),
				mcp.Description("New JSON array of assertion IDs"),
			),
		),
		s.handleAssertionSetUpdate,
	)

	// assertion_set_delete - Delete an assertion set
	s.server.AddTool(
		mcp.NewTool("assertion_set_delete",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription(`Delete an assertion set by ID. This does not delete the individual assertions.`),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the assertion set to delete"),
			),
		),
		s.handleAssertionSetDelete,
	)

	// assertion_set_get - Get an assertion set by ID
	s.server.AddTool(
		mcp.NewTool("assertion_set_get",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Get detailed information about a specific assertion set including its name,
description, and the list of assertion IDs it contains.`),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the assertion set"),
			),
		),
		s.handleAssertionSetGet,
	)

	// assertion_set_list - List all assertion sets
	s.server.AddTool(
		mcp.NewTool("assertion_set_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`List all saved assertion sets.

Returns an array of assertion sets with their IDs, names, descriptions,
assertion counts, and timestamps. Sets are sorted by creation time.`),
		),
		s.handleAssertionSetList,
	)

	// assertion_set_execute - Execute an assertion set
	s.server.AddTool(
		mcp.NewTool("assertion_set_execute",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Execute all assertions in an assertion set against a session.

All assertions run in parallel. The result includes:
- Overall status: "passed", "failed", "partial", or "error"
- Summary with total/passed/failed/error counts and pass rate
- Individual result for each assertion (passed/failed, message, duration)

PARAMETERS:
  set_id: The assertion set ID to execute
  session_id: Session to evaluate assertions against
  device_id: Device ID (used for context)

EXAMPLE:
  set_id: "set-abc123"
  session_id: "session-xyz"
  device_id: "emulator-5554"

RETURNS: Full execution result with summary and individual assertion results.`),
			mcp.WithString("set_id",
				mcp.Required(),
				mcp.Description("ID of the assertion set to execute"),
			),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID to evaluate assertions against"),
			),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID for context"),
			),
		),
		s.handleAssertionSetExecute,
	)

	// assertion_set_results - Get execution history for an assertion set
	s.server.AddTool(
		mcp.NewTool("assertion_set_results",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Get execution history for an assertion set.

Returns a list of past execution results, sorted by most recent first.
Each result includes the status, summary, and individual assertion results.

PARAMETERS:
  set_id: The assertion set ID
  limit: Maximum number of results to return (default: 20)`),
			mcp.WithString("set_id",
				mcp.Required(),
				mcp.Description("ID of the assertion set"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default: 20)"),
			),
		),
		s.handleAssertionSetResults,
	)

	// assertion_set_result - Get a specific execution result by execution ID
	s.server.AddTool(
		mcp.NewTool("assertion_set_result",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Get a specific assertion set execution result by its execution ID.

Returns the full execution result including status, summary, and all individual
assertion results with their pass/fail status, messages, and durations.`),
			mcp.WithString("execution_id",
				mcp.Required(),
				mcp.Description("Execution ID of the result to retrieve"),
			),
		),
		s.handleAssertionSetResult,
	)
}

// Handler implementations

// ============================================================
// Individual Assertion Handlers
// ============================================================

func (s *MCPServer) handleAssertionList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	sessionID, _ := args["session_id"].(string)
	deviceID, _ := args["device_id"].(string)
	templatesOnly := false
	if t, ok := args["templates_only"].(bool); ok {
		templatesOnly = t
	}
	limit := 100
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	assertions, err := s.app.ListStoredAssertions(sessionID, deviceID, templatesOnly, limit)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	if len(assertions) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("No assertions found.")},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d assertion(s):\n\n", len(assertions)))
	for _, a := range assertions {
		sb.WriteString(fmt.Sprintf("- %s (ID: %s) [%s]", a.Name, a.ID, a.Type))
		if a.IsTemplate {
			sb.WriteString(" [template]")
		}
		sb.WriteString("\n")
	}

	jsonBytes, _ := json.MarshalIndent(assertions, "", "  ")
	sb.WriteString("\n")
	sb.WriteString(string(jsonBytes))

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(sb.String())},
	}, nil
}

func (s *MCPServer) handleAssertionCreate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	assertionJSON, _ := args["assertion_json"].(string)
	if assertionJSON == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: assertion_json is required")},
			IsError: true,
		}, nil
	}

	saveAsTemplate := false
	if t, ok := args["save_as_template"].(bool); ok {
		saveAsTemplate = t
	}

	if err := s.app.CreateStoredAssertionJSON(assertionJSON, saveAsTemplate); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent("Assertion created successfully.")},
	}, nil
}

func (s *MCPServer) handleAssertionGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	assertionID, _ := args["assertion_id"].(string)

	if assertionID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: assertion_id is required")},
			IsError: true,
		}, nil
	}

	stored, err := s.app.GetStoredAssertion(assertionID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	jsonBytes, _ := json.MarshalIndent(stored, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonBytes))},
	}, nil
}

func (s *MCPServer) handleAssertionUpdate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	assertionID, _ := args["assertion_id"].(string)
	assertionJSON, _ := args["assertion_json"].(string)

	if assertionID == "" || assertionJSON == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: assertion_id and assertion_json are required")},
			IsError: true,
		}, nil
	}

	if err := s.app.UpdateStoredAssertionJSON(assertionID, assertionJSON); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Assertion '%s' updated successfully.", assertionID))},
	}, nil
}

func (s *MCPServer) handleAssertionDelete(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	assertionID, _ := args["assertion_id"].(string)

	if assertionID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: assertion_id is required")},
			IsError: true,
		}, nil
	}

	if err := s.app.DeleteStoredAssertion(assertionID); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Assertion '%s' deleted successfully.", assertionID))},
	}, nil
}

func (s *MCPServer) handleAssertionExecute(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	assertionID, _ := args["assertion_id"].(string)
	sessionID, _ := args["session_id"].(string)
	deviceID, _ := args["device_id"].(string)

	if assertionID == "" || sessionID == "" || deviceID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: assertion_id, session_id, and device_id are all required")},
			IsError: true,
		}, nil
	}

	result, err := s.app.ExecuteStoredAssertionInSession(assertionID, sessionID, deviceID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	status := "PASS"
	if !result.Passed {
		status = "FAIL"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s\n", status, result.AssertionName))
	sb.WriteString(fmt.Sprintf("Message: %s\n", result.Message))
	sb.WriteString(fmt.Sprintf("Duration: %dms\n\n", result.Duration))

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	sb.WriteString(string(jsonBytes))

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(sb.String())},
	}, nil
}

func (s *MCPServer) handleAssertionQuickNoErrors(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	sessionID, _ := args["session_id"].(string)
	deviceID, _ := args["device_id"].(string)

	if sessionID == "" || deviceID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: session_id and device_id are required")},
			IsError: true,
		}, nil
	}

	result, err := s.app.QuickAssertNoErrors(sessionID, deviceID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	status := "PASS"
	if !result.Passed {
		status = "FAIL"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s\n", status, result.AssertionName))
	sb.WriteString(fmt.Sprintf("Message: %s\n", result.Message))
	sb.WriteString(fmt.Sprintf("Duration: %dms\n\n", result.Duration))

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	sb.WriteString(string(jsonBytes))

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(sb.String())},
	}, nil
}

func (s *MCPServer) handleAssertionQuickNoCrashes(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	sessionID, _ := args["session_id"].(string)
	deviceID, _ := args["device_id"].(string)

	if sessionID == "" || deviceID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: session_id and device_id are required")},
			IsError: true,
		}, nil
	}

	result, err := s.app.QuickAssertNoCrashes(sessionID, deviceID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	status := "PASS"
	if !result.Passed {
		status = "FAIL"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s\n", status, result.AssertionName))
	sb.WriteString(fmt.Sprintf("Message: %s\n", result.Message))
	sb.WriteString(fmt.Sprintf("Duration: %dms\n\n", result.Duration))

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	sb.WriteString(string(jsonBytes))

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(sb.String())},
	}, nil
}

// ============================================================
// Assertion Set Handlers
// ============================================================

func (s *MCPServer) handleAssertionSetCreate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	name, _ := args["name"].(string)
	description, _ := args["description"].(string)
	assertionIDsJSON, _ := args["assertion_ids"].(string)

	if name == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: name is required")},
			IsError: true,
		}, nil
	}

	var assertionIDs []string
	if assertionIDsJSON != "" {
		if err := json.Unmarshal([]byte(assertionIDsJSON), &assertionIDs); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: invalid assertion_ids JSON: %v", err))},
				IsError: true,
			}, nil
		}
	}

	id, err := s.app.CreateAssertionSet(name, description, assertionIDs)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	result := map[string]interface{}{
		"id":      id,
		"message": fmt.Sprintf("Assertion set '%s' created with %d assertions", name, len(assertionIDs)),
	}
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonBytes))},
	}, nil
}

func (s *MCPServer) handleAssertionSetUpdate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id, _ := args["id"].(string)
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)
	assertionIDsJSON, _ := args["assertion_ids"].(string)

	if id == "" || name == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: id and name are required")},
			IsError: true,
		}, nil
	}

	var assertionIDs []string
	if assertionIDsJSON != "" {
		if err := json.Unmarshal([]byte(assertionIDsJSON), &assertionIDs); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: invalid assertion_ids JSON: %v", err))},
				IsError: true,
			}, nil
		}
	}

	if err := s.app.UpdateAssertionSet(id, name, description, assertionIDs); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Assertion set '%s' updated successfully", name))},
	}, nil
}

func (s *MCPServer) handleAssertionSetDelete(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)

	if id == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: id is required")},
			IsError: true,
		}, nil
	}

	if err := s.app.DeleteAssertionSet(id); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Assertion set '%s' deleted successfully", id))},
	}, nil
}

func (s *MCPServer) handleAssertionSetGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)

	if id == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: id is required")},
			IsError: true,
		}, nil
	}

	set, err := s.app.GetAssertionSet(id)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	jsonBytes, _ := json.MarshalIndent(set, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonBytes))},
	}, nil
}

func (s *MCPServer) handleAssertionSetList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sets, err := s.app.ListAssertionSets()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	if len(sets) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("No assertion sets found.")},
		}, nil
	}

	// Build a summary with counts
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d assertion set(s):\n\n", len(sets)))
	for _, set := range sets {
		sb.WriteString(fmt.Sprintf("- %s (ID: %s) - %d assertions", set.Name, set.ID, len(set.Assertions)))
		if set.Description != "" {
			sb.WriteString(fmt.Sprintf("\n  %s", set.Description))
		}
		sb.WriteString("\n")
	}

	// Also include JSON
	jsonBytes, _ := json.MarshalIndent(sets, "", "  ")
	sb.WriteString("\n")
	sb.WriteString(string(jsonBytes))

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(sb.String())},
	}, nil
}

func (s *MCPServer) handleAssertionSetExecute(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	setID, _ := args["set_id"].(string)
	sessionID, _ := args["session_id"].(string)
	deviceID, _ := args["device_id"].(string)

	if setID == "" || sessionID == "" || deviceID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: set_id, session_id, and device_id are all required")},
			IsError: true,
		}, nil
	}

	result, err := s.app.ExecuteAssertionSet(setID, sessionID, deviceID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	// Build human-readable summary + JSON
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Assertion Set: %s\n", result.SetName))
	sb.WriteString(fmt.Sprintf("Status: %s\n", strings.ToUpper(result.Status)))
	sb.WriteString(fmt.Sprintf("Duration: %dms\n", result.Duration))
	sb.WriteString(fmt.Sprintf("Pass Rate: %.1f%% (%d/%d passed, %d failed, %d errors)\n\n",
		result.Summary.PassRate, result.Summary.Passed, result.Summary.Total,
		result.Summary.Failed, result.Summary.Error))

	sb.WriteString("Results:\n")
	for i, r := range result.Results {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
		}
		sb.WriteString(fmt.Sprintf("  %d. [%s] %s - %s (%dms)\n", i+1, status, r.AssertionName, r.Message, r.Duration))
	}

	sb.WriteString("\n")
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	sb.WriteString(string(jsonBytes))

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(sb.String())},
	}, nil
}

func (s *MCPServer) handleAssertionSetResults(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	setID, _ := args["set_id"].(string)
	if setID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: set_id is required")},
			IsError: true,
		}, nil
	}

	limit := 20
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	results, err := s.app.GetAssertionSetResults(setID, limit)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	if len(results) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("No execution results found for this assertion set.")},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d execution result(s):\n\n", len(results)))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s - Pass rate: %.1f%% (%d/%d) - %dms\n",
			i+1, strings.ToUpper(r.Status), r.SetName,
			r.Summary.PassRate, r.Summary.Passed, r.Summary.Total, r.Duration))
	}

	jsonBytes, _ := json.MarshalIndent(results, "", "  ")
	sb.WriteString("\n")
	sb.WriteString(string(jsonBytes))

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(sb.String())},
	}, nil
}

func (s *MCPServer) handleAssertionSetResult(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	executionID, _ := args["execution_id"].(string)
	if executionID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: execution_id is required")},
			IsError: true,
		}, nil
	}

	result, err := s.app.GetAssertionSetResultByExecution(executionID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonBytes))},
	}, nil
}
