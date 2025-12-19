package main

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed bin/adb
var adbBinary []byte

//go:embed bin/scrcpy
var scrcpyBinary []byte

//go:embed bin/scrcpy-server
var scrcpyServerBinary []byte

// App struct
type App struct {
	ctx          context.Context
	adbPath      string
	scrcpyPath   string
	serverPath   string
	logcatCmd    *exec.Cmd
	logcatCancel context.CancelFunc
	mu           sync.Mutex
}

type Device struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Model string `json:"model"`
}

type AppPackage struct {
	Name  string `json:"name"`
	Type  string `json:"type"`  // "system" or "user"
	State string `json:"state"` // "enabled" or "disabled"
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// StopLogcat stops the logcat stream
func (a *App) StopLogcat() {
	if a.logcatCancel != nil {
		a.logcatCancel()
	}
	if a.logcatCmd != nil && a.logcatCmd.Process != nil {
		// Kill the process if it's still running
		_ = a.logcatCmd.Process.Kill()
	}
	a.logcatCmd = nil
	a.logcatCancel = nil
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.setupBinaries()
}

func (a *App) setupBinaries() {
	tempDir := filepath.Join(os.TempDir(), "adb-gui-bin")
	_ = os.MkdirAll(tempDir, 0755)

	extract := func(name string, data []byte) string {
		path := filepath.Join(tempDir, name)
		if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") && name != "scrcpy-server" {
			path += ".exe"
		}

		// Only write if size differs or not exists to avoid "busy" errors
		info, err := os.Stat(path)
		if err != nil || info.Size() != int64(len(data)) {
			err = os.WriteFile(path, data, 0755)
			if err != nil {
				fmt.Printf("Error extracting %s: %v\n", name, err)
			}
		}
		return path
	}

	a.adbPath = extract("adb", adbBinary)
	a.scrcpyPath = extract("scrcpy", scrcpyBinary)
	a.serverPath = extract("scrcpy-server", scrcpyServerBinary)

	fmt.Printf("Binaries setup at: %s\n", tempDir)
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// GetDevices returns a list of connected ADB devices
func (a *App) GetDevices() ([]Device, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.adbPath, "devices", "-l")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run adb devices: %w (output: %s)", err, string(output))
	}

	lines := strings.Split(string(output), "\n")
	var devices []Device

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices attached") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			device := Device{
				ID:    parts[0],
				State: parts[1],
			}
			// Try to parse model
			for _, p := range parts {
				if strings.HasPrefix(p, "model:") {
					device.Model = strings.TrimPrefix(p, "model:")
				}
			}
			devices = append(devices, device)
		}
	}
	return devices, nil
}

// RunAdbCommand executes an arbitrary ADB command
func (a *App) RunAdbCommand(args []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.adbPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}
	return string(output), nil
}

type ScrcpyConfig struct {
	MaxSize       int  `json:"maxSize"`
	BitRate       int  `json:"bitRate"`
	MaxFps        int  `json:"maxFps"`
	StayAwake     bool `json:"stayAwake"`
	TurnScreenOff bool `json:"turnScreenOff"`
	NoAudio       bool `json:"noAudio"`
	AlwaysOnTop   bool `json:"alwaysOnTop"`
}

// StartScrcpy starts scrcpy for the given device with custom configuration
func (a *App) StartScrcpy(deviceId string, config ScrcpyConfig) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}

	args := []string{"-s", deviceId}
	if config.MaxSize > 0 {
		args = append(args, "--max-size", fmt.Sprintf("%d", config.MaxSize))
	}
	if config.BitRate > 0 {
		args = append(args, "--video-bit-rate", fmt.Sprintf("%dM", config.BitRate))
	}
	if config.MaxFps > 0 {
		args = append(args, "--max-fps", fmt.Sprintf("%d", config.MaxFps))
	}
	if config.StayAwake {
		args = append(args, "--stay-awake")
	}
	if config.TurnScreenOff {
		args = append(args, "--turn-screen-off")
	}
	if config.NoAudio {
		args = append(args, "--no-audio")
	}
	if config.AlwaysOnTop {
		args = append(args, "--always-on-top")
	}

	args = append(args, "--window-title", "ADB GUI - "+deviceId)

	cmd := exec.Command(a.scrcpyPath, args...)

	// Use the embedded server and adb
	cmd.Env = append(os.Environ(),
		"SCRCPY_SERVER_PATH="+a.serverPath,
		"ADB="+a.adbPath,
	)

	// Pipe output to console for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Starting scrcpy: %s %v\n", a.scrcpyPath, cmd.Args)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start scrcpy: %w", err)
	}

	return nil
}

// StartLogcat starts the logcat stream for a device, optionally filtering by package name
func (a *App) StartLogcat(deviceId, packageName string) error {
	if a.logcatCmd != nil {
		return fmt.Errorf("logcat already running")
	}

	// Clear buffer first
	exec.Command(a.adbPath, "-s", deviceId, "logcat", "-c").Run()

	ctx, cancel := context.WithCancel(context.Background())
	a.logcatCancel = cancel

	cmd := exec.CommandContext(ctx, a.adbPath, "-s", deviceId, "logcat", "-v", "time")
	a.logcatCmd = cmd

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		a.logcatCmd = nil
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		a.logcatCmd = nil
		return fmt.Errorf("failed to start logcat: %w", err)
	}

	// PID management
	var currentPid string
	var pidMutex sync.RWMutex

	// Poller goroutine to update PID if packageName is provided
	if packageName != "" {
		go func() {
			ticker := time.NewTicker(2 * time.Second) // Check every 2 seconds
			defer ticker.Stop()

			// Function to check and update PID
			checkPid := func() {
				c := exec.Command(a.adbPath, "-s", deviceId, "shell", "pidof", packageName)
				out, _ := c.Output() // Ignore error as it returns 1 if not found
				pid := strings.TrimSpace(string(out))
				// Handle multiple PIDs (take the first one)
				parts := strings.Fields(pid)
				if len(parts) > 0 {
					pid = parts[0]
				}

				pidMutex.Lock()
				if pid != currentPid { // Only emit if PID status changes
					currentPid = pid
					if pid != "" {
						wailsRuntime.EventsEmit(a.ctx, "logcat-data", fmt.Sprintf("--- Monitoring process %s (PID: %s) ---", packageName, pid))
					} else {
						wailsRuntime.EventsEmit(a.ctx, "logcat-data", fmt.Sprintf("--- Waiting for process %s to start ---", packageName))
					}
				}
				pidMutex.Unlock()
			}

			// Initial check
			checkPid()

			for {
				select {
				case <-ctx.Done():
					return // Stop polling when context is cancelled
				case <-ticker.C:
					checkPid()
				}
			}
		}()
	}

	go func() {
		reader := bufio.NewReader(stdout)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break // End of stream or error
			}

			// Filter logic
			if packageName != "" {
				pidMutex.RLock()
				pid := currentPid
				pidMutex.RUnlock()

				if pid != "" {
					// If we have a PID, strictly filter by it
					if !strings.Contains(line, fmt.Sprintf("(%s)", pid)) && !strings.Contains(line, fmt.Sprintf(" %s ", pid)) {
						continue // Skip lines not matching the PID
					}
				} else {
					// If no PID is found yet, drop lines to avoid noise (waiting for app to start)
					continue
				}
			}
			wailsRuntime.EventsEmit(a.ctx, "logcat-data", line)
		}
		// Cleanup is handled by StopLogcat or process exit
	}()

	return nil
}

// ListPackages returns a list of installed packages with their type and state
func (a *App) ListPackages(deviceId string) ([]AppPackage, error) {
	if deviceId == "" {
		return nil, fmt.Errorf("no device specified")
	}

	// 1. Get list of disabled packages
	disabledPackages := make(map[string]bool)
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "pm", "list", "packages", "-d")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "package:") {
				disabledPackages[strings.TrimPrefix(line, "package:")] = true
			}
		}
	}

	var packages []AppPackage

	// Helper to fetch packages by type
	fetch := func(flag, typeName string) error {
		cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "pm", "list", "packages", flag)
		output, err := cmd.Output()
		if err != nil {
			return err
		}
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "package:") {
				name := strings.TrimPrefix(line, "package:")
				state := "enabled"
				if disabledPackages[name] {
					state = "disabled"
				}
				packages = append(packages, AppPackage{
					Name:  name,
					Type:  typeName,
					State: state,
				})
			}
		}
		return nil
	}

	// Fetch system packages
	if err := fetch("-s", "system"); err != nil {
		return nil, fmt.Errorf("failed to list system packages: %w", err)
	}

	// Fetch 3rd party packages
	if err := fetch("-3", "user"); err != nil {
		return nil, fmt.Errorf("failed to list user packages: %w", err)
	}

	return packages, nil
}

// UninstallApp uninstalls an app
func (a *App) UninstallApp(deviceId, packageName string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}
	cmd := exec.Command(a.adbPath, "-s", deviceId, "uninstall", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to uninstall: %w", err)
	}
	return string(output), nil
}

// ClearAppData clears the application data
func (a *App) ClearAppData(deviceId, packageName string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "pm", "clear", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to clear data: %w", err)
	}
	return string(output), nil
}

// ForceStopApp force stops the application
func (a *App) ForceStopApp(deviceId, packageName string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "am", "force-stop", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to force stop: %w", err)
	}
	return string(output), nil
}

// EnableApp enables the application
func (a *App) EnableApp(deviceId, packageName string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "pm", "enable", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to enable app: %w", err)
	}
	return string(output), nil
}

// DisableApp disables the application
func (a *App) DisableApp(deviceId, packageName string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "pm", "disable-user", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to disable app: %w", err)
	}
	return string(output), nil
}
