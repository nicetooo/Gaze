package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *MCPServer) registerRecordingTools() {
	// touch_record_start
	s.server.AddTool(
		mcp.NewTool("touch_record_start",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Start touch recording on a device.

Records touch events from the device screen using getevent.
Two recording modes are available:

MODES:
- "fast": Records raw coordinates only, zero latency. Best for simple automation.
- "precise": Pauses after each touch to capture UI hierarchy and suggest element selectors.
  Best for robust scripts that survive UI layout changes.

The recording runs until touch_record_stop is called.
While recording, every touch on the device screen is captured.

EXAMPLE:
  device_id: "emulator-5554"
  mode: "fast"`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to record on"),
			),
			mcp.WithString("mode",
				mcp.Description("Recording mode: 'fast' (default) or 'precise'"),
			),
		),
		s.handleTouchRecordStart,
	)

	// touch_record_stop
	s.server.AddTool(
		mcp.NewTool("touch_record_stop",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Stop touch recording and return the recorded script.

Returns a TouchScript containing all recorded touch events (tap, swipe, long_press).
Each event includes coordinates, timestamps, and optionally element selectors (precise mode).

The returned script can be saved with touch_script_save or played with touch_script_play.`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to stop recording on"),
			),
		),
		s.handleTouchRecordStop,
	)

	// touch_record_status
	s.server.AddTool(
		mcp.NewTool("touch_record_status",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Check if touch recording is active on a device.`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to check"),
			),
		),
		s.handleTouchRecordStatus,
	)

	// touch_script_list
	s.server.AddTool(
		mcp.NewTool("touch_script_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`List all saved touch scripts.

Returns an array of TouchScript objects with name, device info, resolution,
event count, and creation time. Scripts are stored as JSON files.`),
		),
		s.handleTouchScriptList,
	)

	// touch_script_play
	s.server.AddTool(
		mcp.NewTool("touch_script_play",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Play back a saved touch script on a device.

Replays recorded touch events (tap, swipe, long_press) using adb input commands.
Supports auto-scaling between different screen resolutions.

If events have element selectors (from precise mode recording), Smart Tap is used
to dynamically find elements on screen, making playback more robust.

PARAMETERS:
  device_id: Target device
  script_name: Name of a previously saved script

EXAMPLE:
  device_id: "emulator-5554"
  script_name: "login_flow"`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to play on"),
			),
			mcp.WithString("script_name",
				mcp.Required(),
				mcp.Description("Name of the saved script to play"),
			),
		),
		s.handleTouchScriptPlay,
	)

	// touch_script_save
	s.server.AddTool(
		mcp.NewTool("touch_script_save",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Save a touch script with a given name.

Saves the script as a JSON file. The script must include events array.
Typically used after touch_record_stop to persist the recorded script.

PARAMETERS:
  script_json: JSON string of the TouchScript object (from touch_record_stop output)`),
			mcp.WithString("script_json",
				mcp.Required(),
				mcp.Description("JSON string of the TouchScript to save"),
			),
		),
		s.handleTouchScriptSave,
	)

	// touch_script_delete
	s.server.AddTool(
		mcp.NewTool("touch_script_delete",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription(`Delete a saved touch script by name.`),
			mcp.WithString("script_name",
				mcp.Required(),
				mcp.Description("Name of the script to delete"),
			),
		),
		s.handleTouchScriptDelete,
	)

	// touch_playback_stop
	s.server.AddTool(
		mcp.NewTool("touch_playback_stop",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Stop an ongoing touch script playback on a device.`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to stop playback on"),
			),
		),
		s.handleTouchPlaybackStop,
	)
}

func (s *MCPServer) handleTouchRecordStart(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, _ := args["device_id"].(string)
	mode, _ := args["mode"].(string)
	if mode == "" {
		mode = "fast"
	}

	if deviceID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: device_id is required")},
			IsError: true,
		}, nil
	}

	err := s.app.StartTouchRecording(deviceID, mode)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Touch recording started on device %s in %s mode. Touch the device screen to record events. Call touch_record_stop to finish.", deviceID, mode))},
	}, nil
}

func (s *MCPServer) handleTouchRecordStop(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, _ := args["device_id"].(string)

	if deviceID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: device_id is required")},
			IsError: true,
		}, nil
	}

	script, err := s.app.StopTouchRecording(deviceID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	data, _ := json.MarshalIndent(script, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(data))},
	}, nil
}

func (s *MCPServer) handleTouchRecordStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, _ := args["device_id"].(string)

	if deviceID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: device_id is required")},
			IsError: true,
		}, nil
	}

	isRecording := s.app.IsRecordingTouch(deviceID)
	result := map[string]interface{}{
		"deviceId":    deviceID,
		"isRecording": isRecording,
	}
	data, _ := json.Marshal(result)
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(data))},
	}, nil
}

func (s *MCPServer) handleTouchScriptList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	scripts, err := s.app.LoadTouchScripts()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	// Return summary for each script
	type ScriptSummary struct {
		Name        string `json:"name"`
		DeviceModel string `json:"deviceModel,omitempty"`
		Resolution  string `json:"resolution"`
		EventCount  int    `json:"eventCount"`
		CreatedAt   string `json:"createdAt"`
	}

	summaries := make([]ScriptSummary, len(scripts))
	for i, s := range scripts {
		summaries[i] = ScriptSummary{
			Name:        s.Name,
			DeviceModel: s.DeviceModel,
			Resolution:  s.Resolution,
			EventCount:  len(s.Events),
			CreatedAt:   s.CreatedAt,
		}
	}

	data, _ := json.MarshalIndent(summaries, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(data))},
	}, nil
}

func (s *MCPServer) handleTouchScriptPlay(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, _ := args["device_id"].(string)
	scriptName, _ := args["script_name"].(string)

	if deviceID == "" || scriptName == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: device_id and script_name are required")},
			IsError: true,
		}, nil
	}

	// Load scripts and find by name
	scripts, err := s.app.LoadTouchScripts()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error loading scripts: %v", err))},
			IsError: true,
		}, nil
	}

	var targetScript *TouchScript
	for _, s := range scripts {
		if s.Name == scriptName {
			sCopy := s
			targetScript = &sCopy
			break
		}
	}

	if targetScript == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: script '%s' not found", scriptName))},
			IsError: true,
		}, nil
	}

	err = s.app.PlayTouchScript(deviceID, *targetScript)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Playing script '%s' on device %s (%d events)", scriptName, deviceID, len(targetScript.Events)))},
	}, nil
}

func (s *MCPServer) handleTouchScriptSave(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	scriptJSON, _ := args["script_json"].(string)

	if scriptJSON == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: script_json is required")},
			IsError: true,
		}, nil
	}

	var script TouchScript
	if err := json.Unmarshal([]byte(scriptJSON), &script); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error parsing script JSON: %v", err))},
			IsError: true,
		}, nil
	}

	if script.Name == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: script must have a name")},
			IsError: true,
		}, nil
	}

	err := s.app.SaveTouchScript(script)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Script '%s' saved successfully (%d events)", script.Name, len(script.Events)))},
	}, nil
}

func (s *MCPServer) handleTouchScriptDelete(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	scriptName, _ := args["script_name"].(string)

	if scriptName == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: script_name is required")},
			IsError: true,
		}, nil
	}

	err := s.app.DeleteTouchScript(scriptName)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Script '%s' deleted", scriptName))},
	}, nil
}

func (s *MCPServer) handleTouchPlaybackStop(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, _ := args["device_id"].(string)

	if deviceID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: device_id is required")},
			IsError: true,
		}, nil
	}

	s.app.StopTouchPlayback(deviceID)
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Touch playback stopped on device %s", deviceID))},
	}, nil
}
