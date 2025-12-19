package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed adb_bin/adb
var adbBinary []byte

// App struct
type App struct {
	ctx     context.Context
	adbPath string
}

type Device struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Model string `json:"model"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.setupAdb()
}

func (a *App) setupAdb() {
	// Create a temp file for adb
	tempDir := os.TempDir()
	adbPath := filepath.Join(tempDir, "adb-bundled")

	// On Windows, append .exe
	if runtime.GOOS == "windows" {
		adbPath += ".exe"
	}

	// Write the embedded binary to disk
	err := os.WriteFile(adbPath, adbBinary, 0755)
	if err != nil {
		fmt.Printf("Error extracting adb: %v\n", err)
		// Fallback to system adb if extraction fails
		a.adbPath = "adb"
		return
	}
	a.adbPath = adbPath
	fmt.Printf("ADB extracted to: %s\n", a.adbPath)
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

type AppPackage struct {
	Name  string `json:"name"`
	Type  string `json:"type"`  // "system" or "user"
	State string `json:"state"` // "enabled" or "disabled"
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
