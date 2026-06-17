package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// splitAndTrim splits a comma-separated string and trims whitespace
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// registerSessionTools registers session management tools
func (s *MCPServer) registerSessionTools() {
	// session_create - Create a new session with optional configuration
	s.server.AddTool(
		mcp.NewTool("session_create",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Create a new tracking session for a device with optional configuration.

Configuration options:
- logcat: Capture device logs (can filter by package)
- recording: Record device screen
- proxy: Enable HTTP/HTTPS proxy for network inspection
- monitor: Monitor device state (battery, network, screen, app lifecycle)

Examples:
  Basic session:
    {"device_id": "abc123"}
  
  Session with logcat:
    {"device_id": "abc123", "name": "Debug", "logcat_enabled": true, "logcat_package": "com.example.app"}
  
  Full featured session:
    {"device_id": "abc123", "logcat_enabled": true, "recording_enabled": true, "proxy_enabled": true}`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithString("name",
				mcp.Description("Session name (optional)"),
			),
			mcp.WithString("type",
				mcp.Description("Session type: manual, recording, workflow (default: manual)"),
			),
			// Logcat config
			mcp.WithBoolean("logcat_enabled",
				mcp.Description("Enable logcat capture"),
			),
			mcp.WithString("logcat_package",
				mcp.Description("Package name to filter logcat"),
			),
			mcp.WithString("logcat_pre_filter",
				mcp.Description("Pre-filter for logcat (grep pattern)"),
			),
			mcp.WithString("logcat_exclude_filter",
				mcp.Description("Exclude filter for logcat"),
			),
			// Recording config
			mcp.WithBoolean("recording_enabled",
				mcp.Description("Enable screen recording"),
			),
			mcp.WithString("recording_quality",
				mcp.Description("Recording quality: low, medium, high (default: medium)"),
			),
			// Proxy config
			mcp.WithBoolean("proxy_enabled",
				mcp.Description("Enable HTTP/HTTPS proxy"),
			),
			mcp.WithNumber("proxy_port",
				mcp.Description("Proxy port (default: 8080)"),
			),
			mcp.WithBoolean("proxy_mitm",
				mcp.Description("Enable MITM for HTTPS inspection"),
			),
			// Monitor config
			mcp.WithBoolean("monitor_enabled",
				mcp.Description("Enable device state monitoring (battery, network, screen, app lifecycle)"),
			),
		),
		s.handleSessionCreate,
	)

	// session_end - End a session
	s.server.AddTool(
		mcp.NewTool("session_end",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("End an active session"),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID to end"),
			),
			mcp.WithString("status",
				mcp.Description("Session end status: completed, cancelled, failed (default: completed)"),
			),
		),
		s.handleSessionEnd,
	)

	// session_active - Get active session
	s.server.AddTool(
		mcp.NewTool("session_active",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Get the active session for a device"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
		),
		s.handleSessionActive,
	)

	// session_list - List sessions
	s.server.AddTool(
		mcp.NewTool("session_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("List sessions for a device"),
			mcp.WithString("device_id",
				mcp.Description("Device ID (optional, lists all if not provided)"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of sessions to return (default: 20, use 0 or -1 for all)"),
			),
		),
		s.handleSessionList,
	)

	// session_events - Query session events
	s.server.AddTool(
		mcp.NewTool("session_events",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Query events from a session. Use 'search' for text search, 'types' for event type filter."),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID"),
			),
			mcp.WithString("search",
				mcp.Description("Search text in event title/content"),
			),
			mcp.WithString("types",
				mcp.Description("Event types to filter (comma-separated, e.g., 'logcat,network_request')"),
			),
			mcp.WithString("sources",
				mcp.Description("Event sources to filter (comma-separated, e.g., 'logcat,network,app')"),
			),
			mcp.WithString("levels",
				mcp.Description("Event levels to filter (comma-separated, e.g., 'error,warn')"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of events (default: 100)"),
			),
		),
		s.handleSessionEvents,
	)

	// session_export - Export a session to a .gaze archive
	s.server.AddTool(
		mcp.NewTool("session_export",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Export a session (events, bookmarks, recording) to a .gaze archive file.

The .gaze file is a ZIP archive containing:
- manifest.json: Archive metadata (format version, app version, export time)
- session.json: Session metadata (name, type, status, timestamps, config)
- events.jsonl: All events with full data payloads (JSON Lines format)
- bookmarks.json: User bookmarks (if any)
- recording.mp4: Screen recording video (if session has one)

The exported file can be imported on another machine using session_import.

EXAMPLES:
  Export session to file:
    session_id: "abc12345"
    output_path: "/tmp/debug_session.gaze"

NOTE: output_path must be an absolute path on the host machine.`),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID to export"),
			),
			mcp.WithString("output_path",
				mcp.Required(),
				mcp.Description("Absolute file path to save the .gaze archive"),
			),
		),
		s.handleSessionExport,
	)

	// session_import - Import a session from a .gaze archive
	s.server.AddTool(
		mcp.NewTool("session_import",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Import a session from a .gaze archive file.

Imports all data from a .gaze archive:
- Session metadata (assigned a new ID to avoid conflicts)
- All events with data payloads
- Bookmarks
- Screen recording video (extracted to recordings directory)

The imported session name will have " (imported)" appended.

EXAMPLES:
  Import from file:
    input_path: "/tmp/debug_session.gaze"

NOTE: input_path must be an absolute path to an existing .gaze file.`),
			mcp.WithString("input_path",
				mcp.Required(),
				mcp.Description("Absolute file path to the .gaze archive to import"),
			),
		),
		s.handleSessionImport,
	)

	// session_stats - Get session statistics
	s.server.AddTool(
		mcp.NewTool("session_stats",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Get statistics for a session"),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID"),
			),
		),
		s.handleSessionStats,
	)
}

// Tool handlers

func (s *MCPServer) handleSessionCreate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	name := ""
	if n, ok := args["name"].(string); ok {
		name = n
	}

	// Check if any config is provided
	hasConfig := false
	config := MCPSessionConfig{}

	// Logcat config
	if enabled, ok := args["logcat_enabled"].(bool); ok && enabled {
		hasConfig = true
		config.LogcatEnabled = true
		if pkg, ok := args["logcat_package"].(string); ok {
			config.LogcatPackageName = pkg
		}
		if filter, ok := args["logcat_pre_filter"].(string); ok {
			config.LogcatPreFilter = filter
		}
		if exclude, ok := args["logcat_exclude_filter"].(string); ok {
			config.LogcatExcludeFilter = exclude
		}
	}

	// Recording config
	if enabled, ok := args["recording_enabled"].(bool); ok && enabled {
		hasConfig = true
		config.RecordingEnabled = true
		if quality, ok := args["recording_quality"].(string); ok {
			config.RecordingQuality = quality
		}
	}

	// Proxy config
	if enabled, ok := args["proxy_enabled"].(bool); ok && enabled {
		hasConfig = true
		config.ProxyEnabled = true
		if port, ok := args["proxy_port"].(float64); ok {
			config.ProxyPort = int(port)
		}
		if mitm, ok := args["proxy_mitm"].(bool); ok {
			config.ProxyMitmEnabled = mitm
		}
	}

	// Monitor config
	if enabled, ok := args["monitor_enabled"].(bool); ok && enabled {
		hasConfig = true
		config.MonitorEnabled = true
	}

	var sessionID string
	if hasConfig {
		sessionID = s.app.StartSessionWithConfig(deviceID, name, config)
	} else {
		sessionType := "manual"
		if t, ok := args["type"].(string); ok && t != "" {
			sessionType = t
		}
		sessionID = s.app.CreateSession(deviceID, sessionType, name)
	}

	if sessionID == "" {
		return nil, fmt.Errorf("failed to create session")
	}

	// Build response with enabled features
	features := []string{}
	if config.LogcatEnabled {
		features = append(features, "logcat")
	}
	if config.RecordingEnabled {
		features = append(features, "recording")
	}
	if config.ProxyEnabled {
		features = append(features, "proxy")
	}
	if config.MonitorEnabled {
		features = append(features, "monitor")
	}

	response := fmt.Sprintf("Created session: %s\nDevice: %s", sessionID, deviceID)
	if name != "" {
		response += fmt.Sprintf("\nName: %s", name)
	}
	if len(features) > 0 {
		response += fmt.Sprintf("\nEnabled: %s", strings.Join(features, ", "))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(response),
		},
	}, nil
}

func (s *MCPServer) handleSessionEnd(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	sessionID, ok := args["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	status := "completed"
	if st, ok := args["status"].(string); ok && st != "" {
		status = st
	}

	err := s.app.EndSession(sessionID, status)
	if err != nil {
		return nil, fmt.Errorf("failed to end session: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Session %s ended with status: %s", sessionID, status)),
		},
	}, nil
}

func (s *MCPServer) handleSessionActive(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	sessionID := s.app.GetActiveSession(deviceID)
	if sessionID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("No active session for device %s", deviceID)),
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Active session for device %s: %s", deviceID, sessionID)),
		},
	}, nil
}

func (s *MCPServer) handleSessionList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID := ""
	if d, ok := args["device_id"].(string); ok {
		deviceID = d
	}

	// Default limit is 20, use 0 or negative for all records
	limit := 20
	limitSpecified := false
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
		limitSpecified = true
	}

	sessions, err := s.app.ListStoredSessions(deviceID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("No sessions found"),
			},
		}, nil
	}

	// Build result header
	var result string
	if limitSpecified && limit <= 0 {
		result = fmt.Sprintf("Found %d session(s) (all):\n\n", len(sessions))
	} else if limit > 0 && len(sessions) >= limit {
		result = fmt.Sprintf("Found %d session(s) (limit: %d, may have more):\n\n", len(sessions), limit)
	} else {
		result = fmt.Sprintf("Found %d session(s):\n\n", len(sessions))
	}

	for i, session := range sessions {
		result += fmt.Sprintf("%d. %s\n   ID: %s\n   Type: %s, Status: %s, Events: %d\n",
			i+1, session.Name, session.ID, session.Type, session.Status, session.EventCount)
		if session.VideoPath != "" {
			result += fmt.Sprintf("   Video: %s", session.VideoPath)
			if session.VideoDuration > 0 {
				result += fmt.Sprintf(" (%ds)", session.VideoDuration/1000)
			}
			result += "\n"
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleSessionEvents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	sessionID, ok := args["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	query := EventQuery{
		SessionID: sessionID,
		Limit:     100,
	}

	if l, ok := args["limit"].(float64); ok {
		query.Limit = int(l)
	}

	// Search text
	if search, ok := args["search"].(string); ok && search != "" {
		query.SearchText = search
	}

	// Parse comma-separated types
	if types, ok := args["types"].(string); ok && types != "" {
		query.Types = splitAndTrim(types)
	}

	// Parse comma-separated sources
	if sources, ok := args["sources"].(string); ok && sources != "" {
		query.Sources = splitAndTrim(sources)
	}

	// Parse comma-separated levels
	if levels, ok := args["levels"].(string); ok && levels != "" {
		query.Levels = splitAndTrim(levels)
	}

	result, err := s.app.QuerySessionEvents(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	if len(result.Events) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("No events found in session %s", sessionID)),
			},
		}, nil
	}

	summary := fmt.Sprintf("Session %s: %d events (showing %d)\n\n", sessionID, result.Total, len(result.Events))

	// Show event details
	for i, event := range result.Events {
		if i >= 50 { // Limit display to 50 events
			summary += fmt.Sprintf("\n... and %d more events\n", len(result.Events)-50)
			break
		}
		eventMap, ok := event.(map[string]interface{})
		if ok {
			eventType := eventMap["type"]
			title := eventMap["title"]
			timestamp := eventMap["timestamp"]
			relativeTime := eventMap["relativeTime"]

			summary += fmt.Sprintf("%d. [%v] %v\n", i+1, eventType, title)

			// Show timestamp info
			if relativeTime != nil {
				summary += fmt.Sprintf("   Time: +%vms\n", relativeTime)
			} else if timestamp != nil {
				summary += fmt.Sprintf("   Timestamp: %v\n", timestamp)
			}

			// Show data details for touch/interaction events
			if data, ok := eventMap["data"].(map[string]interface{}); ok && len(data) > 0 {
				// For touch events, show coordinates
				if x, hasX := data["x"]; hasX {
					if y, hasY := data["y"]; hasY {
						summary += fmt.Sprintf("   Coords: (%v, %v)\n", x, y)
					}
				}
				// For swipe events, show end coordinates
				if x2, hasX2 := data["x2"]; hasX2 {
					if y2, hasY2 := data["y2"]; hasY2 {
						summary += fmt.Sprintf("   End: (%v, %v)\n", x2, y2)
					}
				}
				// Show gesture type if present
				if gesture, ok := data["gestureType"].(string); ok && gesture != "" {
					summary += fmt.Sprintf("   Gesture: %s\n", gesture)
				}
				// Show action if present
				if action, ok := data["action"].(string); ok && action != "" {
					summary += fmt.Sprintf("   Action: %s\n", action)
				}
				// Show duration if present
				if duration, ok := data["duration"]; ok {
					summary += fmt.Sprintf("   Duration: %vms\n", duration)
				}
			}
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(summary),
		},
	}, nil
}

func (s *MCPServer) handleSessionStats(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	sessionID, ok := args["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	stats, err := s.app.GetSessionStats(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	jsonData, _ := json.MarshalIndent(stats, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Session %s statistics:\n\n```json\n%s\n```", sessionID, string(jsonData))),
		},
	}, nil
}

func (s *MCPServer) handleSessionExport(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	sessionID, ok := args["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	outputPath, ok := args["output_path"].(string)
	if !ok || outputPath == "" {
		return nil, fmt.Errorf("output_path is required")
	}

	resultPath, err := s.app.ExportSessionToPath(sessionID, outputPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Export failed: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Session %s exported successfully to:\n%s", sessionID, resultPath)),
		},
	}, nil
}

func (s *MCPServer) handleSessionImport(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	inputPath, ok := args["input_path"].(string)
	if !ok || inputPath == "" {
		return nil, fmt.Errorf("input_path is required")
	}

	newSessionID, err := s.app.ImportSessionFromPath(inputPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Import failed: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Session imported successfully.\nNew session ID: %s\nSource: %s", newSessionID, inputPath)),
		},
	}, nil
}
