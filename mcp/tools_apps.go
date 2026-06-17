package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerAppTools registers app management tools
func (s *MCPServer) registerAppTools() {
	// app_list - List installed apps
	s.server.AddTool(
		mcp.NewTool("app_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("List installed applications on a device"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to list apps from"),
			),
			mcp.WithString("type",
				mcp.Description("Package type: 'user' (default), 'system', or 'all'"),
			),
		),
		s.handleAppList,
	)

	// app_info - Get app information
	s.server.AddTool(
		mcp.NewTool("app_info",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Get detailed information about an installed app"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithString("package_name",
				mcp.Required(),
				mcp.Description("Package name (e.g., com.example.app)"),
			),
		),
		s.handleAppInfo,
	)

	// app_start - Start an app
	s.server.AddTool(
		mcp.NewTool("app_start",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Launch an application on the device"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithString("package_name",
				mcp.Required(),
				mcp.Description("Package name to launch"),
			),
		),
		s.handleAppStart,
	)

	// app_stop - Force stop an app
	s.server.AddTool(
		mcp.NewTool("app_stop",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Force stop an application"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithString("package_name",
				mcp.Required(),
				mcp.Description("Package name to stop"),
			),
		),
		s.handleAppStop,
	)

	// app_running - Check if app is running
	s.server.AddTool(
		mcp.NewTool("app_running",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Check if an application is currently running"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithString("package_name",
				mcp.Required(),
				mcp.Description("Package name to check"),
			),
		),
		s.handleAppRunning,
	)

	// app_install - Install APK (DANGEROUS)
	s.server.AddTool(
		mcp.NewTool("app_install",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription("⚠️ Install an APK file on the device (requires confirmation)"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithString("apk_path",
				mcp.Required(),
				mcp.Description("Path to the APK file"),
			),
		),
		s.handleAppInstall,
	)

	// app_uninstall - Uninstall app (DANGEROUS)
	s.server.AddTool(
		mcp.NewTool("app_uninstall",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription("⚠️ Uninstall an application (requires confirmation)"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithString("package_name",
				mcp.Required(),
				mcp.Description("Package name to uninstall"),
			),
		),
		s.handleAppUninstall,
	)

	// app_clear_data - Clear app data (DANGEROUS)
	s.server.AddTool(
		mcp.NewTool("app_clear_data",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription("⚠️ Clear all data for an application (requires confirmation)"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithString("package_name",
				mcp.Required(),
				mcp.Description("Package name to clear data for"),
			),
		),
		s.handleAppClearData,
	)
}

// Tool handlers

func (s *MCPServer) handleAppList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	packageType := "user"
	if t, ok := args["type"].(string); ok && t != "" {
		packageType = t
	}

	packages, err := s.app.ListPackages(deviceID, packageType)
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	if len(packages) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("No packages found"),
			},
		}, nil
	}

	result := fmt.Sprintf("Found %d %s package(s):\n\n", len(packages), packageType)
	for i, p := range packages {
		label := p.Label
		if label == "" {
			label = p.Name
		}
		result += fmt.Sprintf("%d. %s\n   Package: %s\n", i+1, label, p.Name)
		if p.VersionName != "" {
			result += fmt.Sprintf("   Version: %s\n", p.VersionName)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleAppInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	packageName, ok := args["package_name"].(string)
	if !ok || packageName == "" {
		return nil, fmt.Errorf("package_name is required")
	}

	info, err := s.app.GetAppInfo(deviceID, packageName, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get app info: %w", err)
	}

	result := fmt.Sprintf("App: %s\n\n", packageName)
	if info.Label != "" {
		result += fmt.Sprintf("Label: %s\n", info.Label)
	}
	result += fmt.Sprintf("Version: %s (%s)\n", info.VersionName, info.VersionCode)
	result += fmt.Sprintf("Type: %s\n", info.Type)
	result += fmt.Sprintf("State: %s\n", info.State)
	if len(info.Activities) > 0 {
		result += fmt.Sprintf("Activities: %d\n", len(info.Activities))
	}

	jsonData, _ := json.MarshalIndent(info, "", "  ")
	result += fmt.Sprintf("\nJSON:\n```json\n%s\n```", string(jsonData))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleAppStart(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	packageName, ok := args["package_name"].(string)
	if !ok || packageName == "" {
		return nil, fmt.Errorf("package_name is required")
	}

	result, err := s.app.StartApp(deviceID, packageName)
	if err != nil {
		return nil, fmt.Errorf("failed to start app: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Started %s\n%s", packageName, result)),
		},
	}, nil
}

func (s *MCPServer) handleAppStop(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	packageName, ok := args["package_name"].(string)
	if !ok || packageName == "" {
		return nil, fmt.Errorf("package_name is required")
	}

	result, err := s.app.ForceStopApp(deviceID, packageName)
	if err != nil {
		return nil, fmt.Errorf("failed to stop app: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Stopped %s\n%s", packageName, result)),
		},
	}, nil
}

func (s *MCPServer) handleAppRunning(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	packageName, ok := args["package_name"].(string)
	if !ok || packageName == "" {
		return nil, fmt.Errorf("package_name is required")
	}

	running, err := s.app.IsAppRunning(deviceID, packageName)
	if err != nil {
		return nil, fmt.Errorf("failed to check app status: %w", err)
	}

	status := "not running"
	if running {
		status = "running"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("App %s is %s", packageName, status)),
		},
	}, nil
}

// Dangerous operations - require confirmation

func (s *MCPServer) handleAppInstall(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	apkPath, ok := args["apk_path"].(string)
	if !ok || apkPath == "" {
		return nil, fmt.Errorf("apk_path is required")
	}

	// Request confirmation
	confirmed, err := s.requestConfirmation(ctx, "Install APK",
		fmt.Sprintf("Device: %s\nAPK: %s", deviceID, apkPath))
	if err != nil {
		return nil, err
	}
	if !confirmed {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Installation cancelled by user"),
			},
		}, nil
	}

	result, err := s.app.InstallAPK(deviceID, apkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to install: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("APK installed successfully\n%s", result)),
		},
	}, nil
}

func (s *MCPServer) handleAppUninstall(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	packageName, ok := args["package_name"].(string)
	if !ok || packageName == "" {
		return nil, fmt.Errorf("package_name is required")
	}

	// Request confirmation
	confirmed, err := s.requestConfirmation(ctx, "Uninstall App",
		fmt.Sprintf("Device: %s\nPackage: %s\n\nThis will remove the app and all its data!", deviceID, packageName))
	if err != nil {
		return nil, err
	}
	if !confirmed {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Uninstall cancelled by user"),
			},
		}, nil
	}

	result, err := s.app.UninstallApp(deviceID, packageName)
	if err != nil {
		return nil, fmt.Errorf("failed to uninstall: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("App %s uninstalled\n%s", packageName, result)),
		},
	}, nil
}

func (s *MCPServer) handleAppClearData(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	packageName, ok := args["package_name"].(string)
	if !ok || packageName == "" {
		return nil, fmt.Errorf("package_name is required")
	}

	// Request confirmation
	confirmed, err := s.requestConfirmation(ctx, "Clear App Data",
		fmt.Sprintf("Device: %s\nPackage: %s\n\nThis will delete all app data including saved files, settings, and cache!", deviceID, packageName))
	if err != nil {
		return nil, err
	}
	if !confirmed {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Clear data cancelled by user"),
			},
		}, nil
	}

	result, err := s.app.ClearAppData(deviceID, packageName)
	if err != nil {
		return nil, fmt.Errorf("failed to clear data: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Data cleared for %s\n%s", packageName, result)),
		},
	}, nil
}
