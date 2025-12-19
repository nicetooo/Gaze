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
	tempDir := os.TempDir()

	// Setup ADB
	adbPath := filepath.Join(tempDir, "adb-bundled")
	if runtime.GOOS == "windows" {
		adbPath += ".exe"
	}
	_ = os.WriteFile(adbPath, adbBinary, 0755)
	a.adbPath = adbPath

	// Setup Scrcpy
	scrcpyPath := filepath.Join(tempDir, "scrcpy-bundled")
	if runtime.GOOS == "windows" {
		scrcpyPath += ".exe"
	}
	_ = os.WriteFile(scrcpyPath, scrcpyBinary, 0755)
	a.scrcpyPath = scrcpyPath

	// Setup Scrcpy Server
	serverPath := filepath.Join(tempDir, "scrcpy-server")
	_ = os.WriteFile(serverPath, scrcpyServerBinary, 0644)
	a.serverPath = serverPath

	fmt.Printf("Binaries extracted to: %s\n", tempDir)
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// GetDevices returns a list of connected ADB devices
func (a *App) GetDevices() ([]Device, error) {
	cmd := exec.Command(a.adbPath, "devices", "-l")
	output, err := cmd.Output()
	if err != nil {
		// If adb is not found or fails, return error
		return nil, fmt.Errorf("failed to run adb: %w", err)
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
	cmd := exec.Command(a.adbPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}
	return string(output), nil
}

// StartScrcpy starts scrcpy for the given device
func (a *App) StartScrcpy(deviceId string) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}

	cmd := exec.Command(a.scrcpyPath, "-s", deviceId)

	// Use the embedded server and adb
	cmd.Env = append(os.Environ(),
		"SCRCPY_SERVER_PATH="+a.serverPath,
		"ADB="+a.adbPath,
	)

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
