package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerDeviceTools registers device management tools
func (s *MCPServer) registerDeviceTools() {
	// device_list - List connected devices
	s.server.AddTool(
		mcp.NewTool("device_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("List all connected Android devices"),
		),
		s.handleDeviceList,
	)

	// device_info - Get device information
	s.server.AddTool(
		mcp.NewTool("device_info",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Get detailed information about a specific device"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to get information for"),
			),
		),
		s.handleDeviceInfo,
	)

	// device_connect - Connect to a wireless device
	s.server.AddTool(
		mcp.NewTool("device_connect",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Connect to a device via ADB over network (IP:port)"),
			mcp.WithString("address",
				mcp.Required(),
				mcp.Description("Device address in format IP:port (e.g., 192.168.1.100:5555)"),
			),
		),
		s.handleDeviceConnect,
	)

	// device_disconnect - Disconnect a wireless device
	s.server.AddTool(
		mcp.NewTool("device_disconnect",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Disconnect a device from ADB"),
			mcp.WithString("address",
				mcp.Required(),
				mcp.Description("Device address to disconnect"),
			),
		),
		s.handleDeviceDisconnect,
	)

	// device_pair - Pair with a device
	s.server.AddTool(
		mcp.NewTool("device_pair",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Pair with a device using wireless debugging"),
			mcp.WithString("address",
				mcp.Required(),
				mcp.Description("Device pairing address (IP:port)"),
			),
			mcp.WithString("code",
				mcp.Required(),
				mcp.Description("6-digit pairing code from device"),
			),
		),
		s.handleDevicePair,
	)

	// device_wireless - Switch device to wireless mode
	s.server.AddTool(
		mcp.NewTool("device_wireless",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Switch a USB-connected device to wireless ADB mode"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to switch to wireless"),
			),
		),
		s.handleDeviceWireless,
	)

	// device_ip - Get device IP address
	s.server.AddTool(
		mcp.NewTool("device_ip",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Get the IP address of a connected device"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to get IP for"),
			),
		),
		s.handleDeviceIP,
	)

	// adb_execute - Execute arbitrary ADB command
	s.server.AddTool(
		mcp.NewTool("adb_execute",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription(`Execute an arbitrary ADB command on a device.

SHELL COMMANDS (prefix with 'shell'):
- shell ls /sdcard: List files
- shell pm list packages: List all packages
- shell pm list packages -3: List third-party packages only
- shell pm path <package>: Get APK path
- shell dumpsys activity activities: Current activity stack
- shell dumpsys window displays: Display info
- shell getprop ro.build.version.sdk: Get SDK version
- shell settings get secure android_id: Get device ID
- shell input tap <x> <y>: Tap screen
- shell input swipe <x1> <y1> <x2> <y2> <duration>: Swipe
- shell input text "hello": Input text
- shell input keyevent <keycode>: Send key (BACK=4, HOME=3, ENTER=66)
- shell screencap -p /sdcard/screen.png: Screenshot
- shell am start -n <package>/<activity>: Start activity
- shell am force-stop <package>: Force stop app
- shell pm clear <package>: Clear app data
- shell cat /proc/meminfo: Memory info
- shell cat /proc/cpuinfo: CPU info
- shell logcat -d: Dump logcat
- shell logcat -c: Clear logcat

FILE OPERATIONS:
- push <local> <remote>: Push file to device
- pull <remote> <local>: Pull file from device

PACKAGE OPERATIONS:
- install <apk_path>: Install APK
- install -r <apk_path>: Replace existing app
- uninstall <package>: Uninstall app

OTHER:
- forward tcp:<local> tcp:<remote>: Port forwarding
- reverse tcp:<remote> tcp:<local>: Reverse port forwarding
- reboot: Reboot device
- reboot bootloader: Reboot to bootloader
- reboot recovery: Reboot to recovery

NOTE: Command is passed directly to 'adb -s <device_id> <command>'.
Do NOT include 'adb' or '-s' in the command string.`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to execute the command on (use device_list to get available IDs)"),
			),
			mcp.WithString("command",
				mcp.Required(),
				mcp.Description("ADB command without 'adb -s <device>' prefix (e.g., 'shell ls /sdcard')"),
			),
			mcp.WithNumber("timeout",
				mcp.Description("Command timeout in seconds (default: 30, max: 300)"),
			),
			mcp.WithNumber("max_output_bytes",
				mcp.Description("Maximum output size in bytes before head/tail truncation (default: 51200 = 50KB, max: 5MB)"),
			),
		),
		s.handleAdbExecute,
	)

	// aapt_execute - Execute arbitrary aapt command
	s.server.AddTool(
		mcp.NewTool("aapt_execute",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Execute an aapt (Android Asset Packaging Tool) command for APK analysis.

COMMON COMMANDS:
- dump badging <apk>: Get package name, version, permissions, activities, etc.
- dump permissions <apk>: List all permissions declared by the APK
- dump resources <apk>: Dump resource table (very verbose)
- dump configurations <apk>: List all configurations in the APK
- dump xmltree <apk> <file>: Print compiled XML tree (e.g., AndroidManifest.xml)
- list <apk>: List contents of the APK archive
- list -a <apk>: List contents with attributes

EXAMPLES:
  dump badging /path/to/app.apk
  dump permissions /path/to/app.apk
  dump xmltree /path/to/app.apk AndroidManifest.xml
  list -a /path/to/app.apk

OUTPUT FORMAT:
- dump badging returns key-value pairs like: package: name='com.example' versionCode='1'
- Use grep patterns to extract specific info from the output

NOTE: The APK path must be accessible from the host machine (not the Android device).
For device APKs, first pull them using adb_execute: 'pull /data/app/.../base.apk /tmp/'`),
			mcp.WithString("command",
				mcp.Required(),
				mcp.Description("aapt command arguments without 'aapt' prefix (e.g., 'dump badging /path/to/app.apk')"),
			),
			mcp.WithNumber("timeout",
				mcp.Description("Command timeout in seconds (default: 30, max: 300)"),
			),
			mcp.WithNumber("max_output_bytes",
				mcp.Description("Maximum output size in bytes before head/tail truncation (default: 51200 = 50KB, max: 5MB)"),
			),
		),
		s.handleAaptExecute,
	)

	// ffmpeg_execute - Execute arbitrary ffmpeg command
	s.server.AddTool(
		mcp.NewTool("ffmpeg_execute",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription(`Execute an ffmpeg command for video/audio processing.

COMMON OPERATIONS:

1. VIDEO CONVERSION & TRANSCODING:
   -i input.mp4 -c:v libx264 -crf 23 output.mp4
   -i input.webm -c:v libx264 -c:a aac output.mp4

2. RESIZE / SCALE:
   -i input.mp4 -vf scale=1280:720 output.mp4
   -i input.mp4 -vf scale=720:-1 output.mp4  (maintain aspect ratio)
   -i input.mp4 -vf scale=-1:480 output.mp4

3. TRIM / CUT:
   -i input.mp4 -ss 00:00:10 -t 5 output.mp4  (start at 10s, duration 5s)
   -i input.mp4 -ss 00:01:00 -to 00:02:00 output.mp4  (from 1min to 2min)

4. EXTRACT FRAMES:
   -i input.mp4 -r 1 frame_%04d.png  (1 frame per second)
   -i input.mp4 -vf fps=1/10 frame_%04d.png  (1 frame every 10 seconds)
   -i input.mp4 -ss 00:00:05 -vframes 1 thumbnail.png  (single frame at 5s)

5. EXTRACT AUDIO:
   -i input.mp4 -vn -acodec copy output.aac
   -i input.mp4 -vn -ar 44100 -ac 2 -ab 192k output.mp3

6. CREATE GIF:
   -i input.mp4 -vf "fps=10,scale=320:-1" -t 5 output.gif

7. CONCATENATE:
   -f concat -i filelist.txt -c copy output.mp4

IMPORTANT FLAGS:
- -y: Overwrite output without asking
- -hide_banner: Suppress banner info
- -loglevel error: Only show errors

NOTE: Input/output paths must be accessible from the host machine.`),
			mcp.WithString("command",
				mcp.Required(),
				mcp.Description("ffmpeg command arguments without 'ffmpeg' prefix (e.g., '-i input.mp4 -vf scale=720:-1 output.mp4')"),
			),
			mcp.WithNumber("timeout",
				mcp.Description("Command timeout in seconds (default: 60, max: 600). Increase for long videos."),
			),
			mcp.WithNumber("max_output_bytes",
				mcp.Description("Maximum output size in bytes before head/tail truncation (default: 51200 = 50KB, max: 5MB)"),
			),
		),
		s.handleFfmpegExecute,
	)

	// ffprobe_execute - Execute arbitrary ffprobe command
	s.server.AddTool(
		mcp.NewTool("ffprobe_execute",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Execute an ffprobe command for media file analysis.

COMMON OPERATIONS:

1. GET FULL INFO (JSON):
   -v quiet -print_format json -show_format -show_streams input.mp4

2. GET DURATION:
   -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 input.mp4

3. GET RESOLUTION:
   -v error -select_streams v:0 -show_entries stream=width,height -of csv=p=0 input.mp4

4. GET CODEC INFO:
   -v error -select_streams v:0 -show_entries stream=codec_name -of default=noprint_wrappers=1:nokey=1 input.mp4

5. GET FRAME RATE:
   -v error -select_streams v:0 -show_entries stream=r_frame_rate -of default=noprint_wrappers=1:nokey=1 input.mp4

6. GET BIT RATE:
   -v error -show_entries format=bit_rate -of default=noprint_wrappers=1:nokey=1 input.mp4

7. COUNT FRAMES:
   -v error -select_streams v:0 -count_frames -show_entries stream=nb_read_frames -of default=noprint_wrappers=1:nokey=1 input.mp4

OUTPUT FORMATS (-of or -print_format):
- json: JSON format (best for parsing)
- csv: Comma-separated values
- flat: Flat format with full key names
- default: Default text format

COMMON FLAGS:
- -v quiet/error: Suppress info messages
- -show_format: Show container format info
- -show_streams: Show stream info (video, audio, etc.)
- -show_frames: Show frame-level info (verbose)
- -select_streams v:0: Select first video stream
- -select_streams a:0: Select first audio stream`),
			mcp.WithString("command",
				mcp.Required(),
				mcp.Description("ffprobe command arguments without 'ffprobe' prefix (e.g., '-v quiet -print_format json -show_format input.mp4')"),
			),
			mcp.WithNumber("timeout",
				mcp.Description("Command timeout in seconds (default: 30, max: 300)"),
			),
			mcp.WithNumber("max_output_bytes",
				mcp.Description("Maximum output size in bytes before head/tail truncation (default: 51200 = 50KB, max: 5MB)"),
			),
		),
		s.handleFfprobeExecute,
	)

	// file_upload - Upload file from host to device
	s.server.AddTool(
		mcp.NewTool("file_upload",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Upload a file from the host machine to an Android device.

This tool uses 'adb push' internally to transfer files from your computer to the device.

COMMON USE CASES:
- Upload APKs for manual installation
- Push configuration files
- Transfer media files (images, videos, audio)
- Deploy test data files

COMMON REMOTE PATHS:
- /sdcard/: Main external storage (most common)
- /sdcard/Download/: Downloads folder
- /sdcard/DCIM/: Camera photos folder
- /sdcard/Pictures/: Pictures folder
- /sdcard/Music/: Music folder
- /sdcard/Android/data/<package>/: App-specific data

EXAMPLES:
  Upload an image:
    local_path: /Users/me/image.png
    remote_path: /sdcard/Pictures/image.png

  Upload test data:
    local_path: /tmp/test_data.json
    remote_path: /sdcard/Download/test_data.json

NOTE: 
- The local_path must be an absolute path on your computer
- Ensure the remote directory exists or use /sdcard/ for auto-creation
- Some system directories require root access`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to upload the file to"),
			),
			mcp.WithString("local_path",
				mcp.Required(),
				mcp.Description("Absolute path to the file on the host machine"),
			),
			mcp.WithString("remote_path",
				mcp.Required(),
				mcp.Description("Destination path on the Android device (e.g., /sdcard/Download/file.txt)"),
			),
		),
		s.handleFileUpload,
	)

	// file_list - List files in a directory on device
	s.server.AddTool(
		mcp.NewTool("file_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`List files and directories in a path on an Android device.

Returns information about each file including:
- name: File or directory name
- path: Full path on device
- size: File size in bytes (0 for directories)
- isDir: Whether it's a directory
- modTime: Last modification time

COMMON PATHS:
- /sdcard/: Main external storage
- /sdcard/Download/: Downloads folder
- /sdcard/DCIM/Camera/: Camera photos
- /sdcard/Android/data/: App data folders
- /data/data/: App internal data (requires root)

EXAMPLES:
  List sdcard root:
    path: /sdcard

  List downloads:
    path: /sdcard/Download

  List app data:
    path: /sdcard/Android/data/com.example.app/files`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to list files from"),
			),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Directory path on the device to list (e.g., /sdcard)"),
			),
		),
		s.handleFileList,
	)
}

// Tool handlers

func (s *MCPServer) handleDeviceList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	devices, err := s.app.GetDevices(false)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	if len(devices) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("No devices connected"),
			},
		}, nil
	}

	// Format device list
	result := fmt.Sprintf("Found %d device(s):\n\n", len(devices))
	for i, d := range devices {
		connType := ""
		if d.Type == "wireless" || d.Type == "both" {
			connType = " [wireless]"
		}
		result += fmt.Sprintf("%d. %s (%s)%s\n   Model: %s, Brand: %s, State: %s\n",
			i+1, d.ID, d.Serial, connType, d.Model, d.Brand, d.State)
	}

	// Also include JSON for structured access
	jsonData, _ := json.MarshalIndent(devices, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
			mcp.NewTextContent(fmt.Sprintf("\nJSON data:\n```json\n%s\n```", string(jsonData))),
		},
	}, nil
}

func (s *MCPServer) handleDeviceInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	info, err := s.app.GetDeviceInfo(deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}

	result := fmt.Sprintf("Device: %s\n\n", deviceID)
	result += fmt.Sprintf("Model: %s\n", info.Model)
	result += fmt.Sprintf("Brand: %s\n", info.Brand)
	result += fmt.Sprintf("Manufacturer: %s\n", info.Manufacturer)
	result += fmt.Sprintf("Android Version: %s\n", info.AndroidVer)
	result += fmt.Sprintf("SDK Level: %s\n", info.SDK)
	result += fmt.Sprintf("ABI: %s\n", info.ABI)
	result += fmt.Sprintf("Serial: %s\n", info.Serial)
	result += fmt.Sprintf("Resolution: %s\n", info.Resolution)
	result += fmt.Sprintf("Density: %s\n", info.Density)
	result += fmt.Sprintf("CPU: %s\n", info.CPU)
	result += fmt.Sprintf("Memory: %s\n", info.Memory)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleDeviceConnect(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	address, ok := args["address"].(string)
	if !ok || address == "" {
		return nil, fmt.Errorf("address is required")
	}

	result, err := s.app.AdbConnect(address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleDeviceDisconnect(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	address, ok := args["address"].(string)
	if !ok || address == "" {
		return nil, fmt.Errorf("address is required")
	}

	result, err := s.app.AdbDisconnect(address)
	if err != nil {
		return nil, fmt.Errorf("failed to disconnect: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleDevicePair(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	address, ok := args["address"].(string)
	if !ok || address == "" {
		return nil, fmt.Errorf("address is required")
	}
	code, ok := args["code"].(string)
	if !ok || code == "" {
		return nil, fmt.Errorf("pairing code is required")
	}

	result, err := s.app.AdbPair(address, code)
	if err != nil {
		return nil, fmt.Errorf("failed to pair: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleDeviceWireless(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	result, err := s.app.SwitchToWireless(deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to switch to wireless: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleDeviceIP(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	ip, err := s.app.GetDeviceIP(deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device IP: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Device %s IP address: %s", deviceID, ip)),
		},
	}, nil
}

func (s *MCPServer) handleAdbExecute(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	command, ok := args["command"].(string)
	if !ok || command == "" {
		return nil, fmt.Errorf("command is required")
	}

	timeout := clampTimeout(args, 30, 300)

	// Execute the ADB command
	output, err := s.app.RunAdbCommandWithTimeout(deviceID, command, timeout)
	output = truncateOutput(output, outputLimitFromArgs(args))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Command failed: %v\n\nOutput:\n%s", err, output)),
			},
			IsError: true,
		}, nil
	}

	// Format result
	result := fmt.Sprintf("Command: adb -s %s %s\n\n", deviceID, command)
	if output == "" {
		result += "Command executed successfully (no output)"
	} else {
		result += fmt.Sprintf("Output:\n%s", output)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleAaptExecute(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	command, ok := args["command"].(string)
	if !ok || command == "" {
		return nil, fmt.Errorf("command is required")
	}

	timeout := clampTimeout(args, 30, 300)

	output, err := s.app.RunAaptCommand(command, timeout)
	output = truncateOutput(output, outputLimitFromArgs(args))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Command failed: %v\n\nOutput:\n%s", err, output)),
			},
			IsError: true,
		}, nil
	}

	result := fmt.Sprintf("Command: aapt %s\n\n", command)
	if output == "" {
		result += "Command executed successfully (no output)"
	} else {
		result += fmt.Sprintf("Output:\n%s", output)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleFfmpegExecute(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	command, ok := args["command"].(string)
	if !ok || command == "" {
		return nil, fmt.Errorf("command is required")
	}

	timeout := clampTimeout(args, 60, 600)

	output, err := s.app.RunFfmpegCommand(command, timeout)
	output = truncateOutput(output, outputLimitFromArgs(args))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Command failed: %v\n\nOutput:\n%s", err, output)),
			},
			IsError: true,
		}, nil
	}

	result := fmt.Sprintf("Command: ffmpeg %s\n\n", command)
	if output == "" {
		result += "Command executed successfully (no output)"
	} else {
		result += fmt.Sprintf("Output:\n%s", output)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleFfprobeExecute(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	command, ok := args["command"].(string)
	if !ok || command == "" {
		return nil, fmt.Errorf("command is required")
	}

	timeout := clampTimeout(args, 30, 300)

	output, err := s.app.RunFfprobeCommand(command, timeout)
	output = truncateOutput(output, outputLimitFromArgs(args))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Command failed: %v\n\nOutput:\n%s", err, output)),
			},
			IsError: true,
		}, nil
	}

	result := fmt.Sprintf("Command: ffprobe %s\n\n", command)
	if output == "" {
		result += "Command executed successfully (no output)"
	} else {
		result += fmt.Sprintf("Output:\n%s", output)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleFileUpload(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	localPath, ok := args["local_path"].(string)
	if !ok || localPath == "" {
		return nil, fmt.Errorf("local_path is required")
	}

	remotePath, ok := args["remote_path"].(string)
	if !ok || remotePath == "" {
		return nil, fmt.Errorf("remote_path is required")
	}

	err := s.app.UploadFile(deviceID, localPath, remotePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Failed to upload file: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Successfully uploaded %s to %s on device %s", localPath, remotePath, deviceID)),
		},
	}, nil
}

func (s *MCPServer) handleFileList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	path, ok := args["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("path is required")
	}

	files, err := s.app.ListFiles(deviceID, path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Failed to list files: %v", err)),
			},
			IsError: true,
		}, nil
	}

	if len(files) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Directory %s is empty or does not exist", path)),
			},
		}, nil
	}

	// Format as readable text
	result := fmt.Sprintf("Files in %s (%d items):\n\n", path, len(files))
	for _, f := range files {
		typeStr := "FILE"
		if isDir, ok := f["isDir"].(bool); ok && isDir {
			typeStr = "DIR "
		}
		size := int64(0)
		if s, ok := f["size"].(int64); ok {
			size = s
		}
		name := f["name"]
		modTime := f["modTime"]

		if typeStr == "DIR " {
			result += fmt.Sprintf("[%s] %s/\n", typeStr, name)
		} else {
			sizeStr := formatSize(size)
			result += fmt.Sprintf("[%s] %s (%s, %v)\n", typeStr, name, sizeStr, modTime)
		}
	}

	// Also include JSON for structured access
	jsonData, _ := json.MarshalIndent(files, "", "  ")
	result += fmt.Sprintf("\nJSON data:\n```json\n%s\n```", string(jsonData))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

const (
	// defaultMaxOutputBytes limits CLI tool output flowing into the model context
	defaultMaxOutputBytes = 50 * 1024
	// maxOutputBytesCap is the hard upper bound for the max_output_bytes argument
	maxOutputBytesCap = 5 * 1024 * 1024
)

// clampTimeout reads the optional "timeout" argument and clamps it to [1, maxSec],
// falling back to defaultSec when absent or invalid
func clampTimeout(args map[string]interface{}, defaultSec, maxSec int) int {
	timeout := defaultSec
	if t, ok := args["timeout"].(float64); ok && t > 0 {
		timeout = int(t)
	}
	if timeout > maxSec {
		timeout = maxSec
	}
	return timeout
}

// outputLimitFromArgs reads the optional "max_output_bytes" argument
func outputLimitFromArgs(args map[string]interface{}) int {
	if v, ok := args["max_output_bytes"].(float64); ok && v > 0 {
		return int(v)
	}
	return defaultMaxOutputBytes
}

// truncateOutput limits output to maxBytes by keeping the head (80%) and tail (20%)
// with a marker in between. Commands like 'shell logcat -d' or 'aapt dump resources'
// can return multiple MB which would otherwise flood the model context.
func truncateOutput(output string, maxBytes int) string {
	if maxBytes <= 0 {
		maxBytes = defaultMaxOutputBytes
	}
	if maxBytes > maxOutputBytesCap {
		maxBytes = maxOutputBytesCap
	}
	if len(output) <= maxBytes {
		return output
	}
	head := maxBytes * 4 / 5
	tail := maxBytes - head
	// Back off to rune boundaries so the cut never splits a multi-byte
	// UTF-8 character (adb output frequently contains CJK app names/logs)
	for head > 0 && !utf8.RuneStart(output[head]) {
		head--
	}
	tailStart := len(output) - tail
	for tailStart < len(output) && !utf8.RuneStart(output[tailStart]) {
		tailStart++
	}
	truncated := tailStart - head
	return output[:head] +
		fmt.Sprintf("\n[... %d bytes truncated, narrow the command output (e.g. grep/head filters) or raise max_output_bytes ...]\n", truncated) +
		output[tailStart:]
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
}
