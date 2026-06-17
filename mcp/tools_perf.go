package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerPerfTools registers performance monitoring MCP tools
func (s *MCPServer) registerPerfTools() {
	// perf_start - Start performance monitoring
	s.server.AddTool(
		mcp.NewTool("perf_start",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Start real-time performance monitoring on a device.

Monitors CPU usage, memory usage, FPS, network I/O, and battery stats.
Data is collected at configurable intervals and emitted as perf_sample events.

Use session_events with types filter "perf_sample" to retrieve collected data.
Use perf_stop to stop monitoring.

CONFIGURATION:
- package_name: Monitor a specific app (optional, empty = system-wide)
- interval_ms: Sampling interval in milliseconds (default: 2000, min: 500)
- Metrics can be individually enabled/disabled

METRICS COLLECTED:
- CPU: Total usage %, app CPU %, core count, temperature
- Memory: Total/used/free MB, usage %, app memory
- FPS: Frame rate via SurfaceFlinger, jank count
- Network: RX/TX speed (KB/s), total traffic (MB)
- Battery: Level %, temperature`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithString("package_name",
				mcp.Description("Package name to monitor (optional, empty for system-wide)"),
			),
			mcp.WithNumber("interval_ms",
				mcp.Description("Sampling interval in milliseconds (default: 2000, min: 500)"),
			),
		),
		s.handlePerfStart,
	)

	// perf_stop - Stop performance monitoring
	s.server.AddTool(
		mcp.NewTool("perf_stop",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Stop performance monitoring on a device."),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
		),
		s.handlePerfStop,
	)

	// perf_process_detail - Get detailed info about a specific process
	s.server.AddTool(
		mcp.NewTool("perf_process_detail",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Get detailed information about a specific process by PID.

Runs 'dumpsys meminfo <pid>' and reads /proc/<pid>/status to provide:
- Memory breakdown by category (Java Heap, Native Heap, Code, Stack, Graphics, etc.)
- Heap allocation details (size, alloc, free for both Java and Native heaps)
- Android object counts (Views, Activities, WebViews, Binders, etc.)
- Process metadata (threads, file descriptors, swap, OOM priority)

NOTE: This is an on-demand call (takes 2-3 seconds due to dumpsys meminfo).
Use perf_start/perf_snapshot for continuous monitoring.`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithNumber("pid",
				mcp.Required(),
				mcp.Description("Process ID to inspect"),
			),
		),
		s.handlePerfProcessDetail,
	)

	// perf_snapshot - Get a one-time performance snapshot
	s.server.AddTool(
		mcp.NewTool("perf_snapshot",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Get a one-time performance snapshot without starting continuous monitoring.

Returns current CPU, memory, network, and battery stats.
Useful for quick checks without the overhead of continuous monitoring.`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithString("package_name",
				mcp.Description("Package name to include app-specific metrics (optional)"),
			),
		),
		s.handlePerfSnapshot,
	)
}

func (s *MCPServer) handlePerfStart(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, _ := args["device_id"].(string)
	if deviceID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: device_id is required")},
			IsError: true,
		}, nil
	}

	config := PerfMonitorConfig{
		EnableCPU:     true,
		EnableMemory:  true,
		EnableFPS:     true,
		EnableNetwork: true,
		EnableBattery: true,
		IntervalMs:    2000,
	}

	if pkg, ok := args["package_name"].(string); ok {
		config.PackageName = pkg
	}
	if interval, ok := args["interval_ms"].(float64); ok && interval >= 500 {
		config.IntervalMs = int(interval)
	}

	result := s.app.StartPerfMonitor(deviceID, config)
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Performance monitoring %s for device %s (interval: %dms, package: %s)",
			result, deviceID, config.IntervalMs, config.PackageName))},
	}, nil
}

func (s *MCPServer) handlePerfStop(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, _ := args["device_id"].(string)
	if deviceID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: device_id is required")},
			IsError: true,
		}, nil
	}

	result := s.app.StopPerfMonitor(deviceID)
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Performance monitoring %s for device %s", result, deviceID))},
	}, nil
}

func (s *MCPServer) handlePerfSnapshot(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, _ := args["device_id"].(string)
	if deviceID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: device_id is required")},
			IsError: true,
		}, nil
	}

	packageName, _ := args["package_name"].(string)

	sample, err := s.app.GetPerfSnapshot(deviceID, packageName)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	data, _ := json.MarshalIndent(sample, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(data))},
	}, nil
}

func (s *MCPServer) handlePerfProcessDetail(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, _ := args["device_id"].(string)
	if deviceID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: device_id is required")},
			IsError: true,
		}, nil
	}

	pidFloat, ok := args["pid"].(float64)
	if !ok || pidFloat <= 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: pid is required and must be > 0")},
			IsError: true,
		}, nil
	}

	detail, err := s.app.GetProcessDetail(deviceID, int(pidFloat))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	data, _ := json.MarshalIndent(detail, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(data))},
	}, nil
}
