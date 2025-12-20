package main

import (
	"archive/zip"
	"bufio"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Binaries are embedded in platform-specific files (bin_*.go) and bin_common.go

// App struct
type App struct {
	ctx          context.Context
	adbPath      string
	scrcpyPath   string
	serverPath   string
	aaptPath     string
	logcatCmd    *exec.Cmd
	logcatCancel context.CancelFunc

	// Generic mutex for shared state
	mu sync.Mutex

	// aaptCache caches app label & icon so each package is processed at most once.
	// Keyed by package name.
	aaptCache   map[string]AppPackage
	aaptCacheMu sync.RWMutex
	cachePath   string
}

type Device struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Model string `json:"model"`
	Brand string `json:"brand"`
}

type FileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"modTime"`
	IsDir   bool   `json:"isDir"`
	Path    string `json:"path"`
}

type AppPackage struct {
	Name             string   `json:"name"`
	Label            string   `json:"label"` // Application label/name
	Icon             string   `json:"icon"`  // Base64 encoded icon
	Type             string   `json:"type"`  // "system" or "user"
	State            string   `json:"state"` // "enabled" or "disabled"
	VersionName      string   `json:"versionName"`
	VersionCode      string   `json:"versionCode"`
	MinSdkVersion    string   `json:"minSdkVersion"`
	TargetSdkVersion string   `json:"targetSdkVersion"`
	Permissions      []string `json:"permissions"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		aaptCache: make(map[string]AppPackage),
	}
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
	a.initPersistentCache()
}

func (a *App) initPersistentCache() {
	// Use application config directory for persistent cache
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	appConfigDir := filepath.Join(configDir, "adbGUI")
	_ = os.MkdirAll(appConfigDir, 0755)
	a.cachePath = filepath.Join(appConfigDir, "aapt_cache.json")

	a.loadCache()
}

func (a *App) loadCache() {
	a.aaptCacheMu.Lock()
	defer a.aaptCacheMu.Unlock()

	data, err := os.ReadFile(a.cachePath)
	if err != nil {
		return // File doesn't exist yet or is unreadable
	}

	_ = json.Unmarshal(data, &a.aaptCache)
}

func (a *App) saveCache() {
	a.aaptCacheMu.RLock()
	data, err := json.Marshal(a.aaptCache)
	a.aaptCacheMu.RUnlock()

	if err != nil {
		fmt.Printf("Error marshaling cache: %v\n", err)
		return
	}

	err = os.WriteFile(a.cachePath, data, 0644)
	if err != nil {
		fmt.Printf("Error saving cache to %s: %v\n", a.cachePath, err)
	}
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

	// Extract aapt if available (may be empty placeholder)
	if len(aaptBinary) > 0 {
		a.aaptPath = extract("aapt", aaptBinary)
		fmt.Printf("AAPT setup at: %s\n", a.aaptPath)
	} else {
		fmt.Printf("Warning: aapt binary not embedded. App icons and names may not be available.\n")
		fmt.Printf("Please run scripts/download_aapt.sh to download aapt binaries.\n")
	}

	fmt.Printf("Binaries setup at: %s\n", tempDir)
}

// GetAppInfo returns detailed information for a specific package
func (a *App) GetAppInfo(deviceId, packageName string, force bool) (AppPackage, error) {
	return a.getAppInfoWithAapt(deviceId, packageName, force)
}

// getAppInfoWithAapt extracts app label, icon and other metadata using aapt
func (a *App) getAppInfoWithAapt(deviceId, packageName string, force bool) (AppPackage, error) {
	// 0. Check cache first to ensure we only process each package once
	if !force {
		a.aaptCacheMu.RLock()
		if cached, ok := a.aaptCache[packageName]; ok {
			// If we have more than just basic info (e.g., VersionName), return it
			if cached.VersionName != "" || cached.Label != "" {
				a.aaptCacheMu.RUnlock()
				return cached, nil
			}
		}
		a.aaptCacheMu.RUnlock()
	}

	var pkg AppPackage
	pkg.Name = packageName

	if a.aaptPath == "" {
		return pkg, fmt.Errorf("aapt not available (binary not embedded)")
	}

	// Check if aapt file actually exists and is not empty
	if info, err := os.Stat(a.aaptPath); err != nil || info.Size() == 0 {
		return pkg, fmt.Errorf("aapt not available (file missing or empty)")
	}

	// 1. Get APK path from device (no timeout)
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "pm", "path", packageName)
	output, err := cmd.Output()
	if err != nil {
		return pkg, fmt.Errorf("failed to get APK path: %w", err)
	}

	remotePath := strings.TrimSpace(string(output))
	if remotePath == "" {
		return pkg, fmt.Errorf("empty output from pm path for %s", packageName)
	}

	lines := strings.Split(remotePath, "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "package:") {
		return pkg, fmt.Errorf("unexpected output from pm path: %s", remotePath)
	}
	remotePath = strings.TrimPrefix(lines[0], "package:")

	// 2. Create temporary file for APK
	tmpDir := filepath.Join(os.TempDir(), "adb-gui-apk")
	_ = os.MkdirAll(tmpDir, 0755)
	tmpAPK := filepath.Join(tmpDir, packageName+".apk")
	defer os.Remove(tmpAPK) // Clean up

	// 3. Pull APK to local (no timeout)
	pullCmd := exec.Command(a.adbPath, "-s", deviceId, "pull", remotePath, tmpAPK)
	pullOutput, err := pullCmd.CombinedOutput()
	if err != nil {
		return pkg, fmt.Errorf("failed to pull APK: %w (output: %s)", err, string(pullOutput))
	}

	// Check if APK file was actually downloaded
	if info, err := os.Stat(tmpAPK); err != nil || info.Size() == 0 {
		return pkg, fmt.Errorf("APK file not downloaded or empty: %v", err)
	}

	// 4. Use aapt to dump badging (get metadata) - no timeout
	aaptCmd := exec.Command(a.aaptPath, "dump", "badging", tmpAPK)
	aaptOutput, err := aaptCmd.CombinedOutput()
	if err != nil {
		if len(aaptOutput) == 0 {
			return pkg, fmt.Errorf("aapt command failed with no output: %w", err)
		}
		return pkg, fmt.Errorf("failed to run aapt: %w, output: %s", err, string(aaptOutput))
	}

	if len(aaptOutput) == 0 {
		return pkg, fmt.Errorf("aapt command succeeded but output is empty")
	}

	// 5. Parse information from aapt output
	outputStr := string(aaptOutput)
	pkg.Label = a.parseLabelFromAapt(outputStr)
	pkg.VersionName, pkg.VersionCode = a.parseVersionFromAapt(outputStr)
	pkg.MinSdkVersion = a.parseSdkVersionFromAapt(outputStr, "sdkVersion:")
	pkg.TargetSdkVersion = a.parseSdkVersionFromAapt(outputStr, "targetSdkVersion:")
	pkg.Permissions = a.parsePermissionsFromAapt(outputStr)

	// Debug logging
	fmt.Printf("DEBUG: Extracted for %s: Label='%s', Version='%s'\n", packageName, pkg.Label, pkg.VersionName)

	// 6. Extract icon using aapt (don't fail if icon extraction fails)
	icon, err := a.extractIconWithAapt(tmpAPK)
	if err == nil {
		pkg.Icon = icon
	}

	// 7. Store in cache
	a.aaptCacheMu.Lock()
	a.aaptCache[packageName] = pkg
	a.aaptCacheMu.Unlock()

	// Save to persistent storage
	go a.saveCache()

	return pkg, nil
}

// parseVersionFromAapt parses versionName and versionCode from aapt output
func (a *App) parseVersionFromAapt(output string) (versionName, versionCode string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "package:") {
			// package: name='com.example' versionCode='123' versionName='1.2.3' ...
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "versionCode=") {
					versionCode = strings.Trim(strings.TrimPrefix(part, "versionCode="), "'\"")
				}
				if strings.HasPrefix(part, "versionName=") {
					versionName = strings.Trim(strings.TrimPrefix(part, "versionName="), "'\"")
				}
			}
			return
		}
	}
	return
}

// parseSdkVersionFromAapt parses sdkVersion or targetSdkVersion
func (a *App) parseSdkVersionFromAapt(output, prefix string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			val := strings.TrimPrefix(line, prefix)
			return strings.Trim(val, "'\"")
		}
	}
	return ""
}

// parsePermissionsFromAapt parses uses-permission from aapt output
func (a *App) parsePermissionsFromAapt(output string) []string {
	var permissions []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "uses-permission: name=") {
			perm := strings.TrimPrefix(line, "uses-permission: name=")
			perm = strings.Trim(perm, "'\"")
			permissions = append(permissions, perm)
		}
	}
	return permissions
}

// parseLabelFromAapt parses the application label from aapt dump badging output
func (a *App) parseLabelFromAapt(output string) string {
	// Look for application-label or application-label-zh-CN etc.
	lines := strings.Split(output, "\n")

	// First, try to find application-label: (default label)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "application-label:") {
			// Format: application-label:'App Name' or application-label:App Name
			label := strings.TrimPrefix(line, "application-label:")
			label = strings.Trim(label, "'\"")
			// Remove any trailing whitespace or special characters
			label = strings.TrimSpace(label)
			if label != "" {
				return label
			}
		}
	}

	// Then, try to find localized labels (prefer English, then any other)
	preferredLocales := []string{"en", "zh-CN", "zh", ""}
	for _, locale := range preferredLocales {
		prefix := "application-label"
		if locale != "" {
			prefix = fmt.Sprintf("application-label-%s", locale)
		}

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, prefix+":") {
				label := strings.TrimPrefix(line, prefix+":")
				label = strings.Trim(label, "'\"")
				label = strings.TrimSpace(label)
				if label != "" {
					return label
				}
			}
		}
	}

	// Fallback: look for any application-label-* line
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "application-label-") && strings.Contains(line, ":") {
			idx := strings.Index(line, ":")
			if idx > 0 && idx < len(line)-1 {
				label := line[idx+1:]
				label = strings.Trim(label, "'\"")
				label = strings.TrimSpace(label)
				if label != "" {
					return label
				}
			}
		}
	}

	// NEW Fallback: look for application: label='...'
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "application:") && strings.Contains(line, "label='") {
			idx := strings.Index(line, "label='")
			if idx > 0 {
				start := idx + 7
				end := strings.Index(line[start:], "'")
				if end > 0 {
					label := line[start : start+end]
					if label != "" {
						return label
					}
				}
			}
		}
	}

	return ""
}

// extractIconWithAapt extracts the app icon using aapt
func (a *App) extractIconWithAapt(apkPath string) (string, error) {
	// 1. List resources to find icon
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.aaptPath, "dump", "badging", apkPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run aapt dump badging: %w, output: %s", err, string(output))
	}

	// 2. Find icon path from badging output
	// Format: application-icon-120:'res/mipmap-hdpi/ic_launcher.png'
	// aapt2 may use different format, try both
	outputStr := string(output)
	iconPath := a.parseIconPathFromAapt(outputStr)
	if iconPath == "" {
		// Try alternative parsing for aapt2 format
		iconPath = a.parseIconPathFromAapt2(outputStr)
	}
	if iconPath == "" {
		return "", fmt.Errorf("icon path not found in aapt output")
	}

	// 3. Extract icon file from APK
	// APK is a zip file, so we can extract the icon directly
	iconData, err := a.extractFileFromAPK(apkPath, iconPath)
	if err != nil {
		// Try alternative paths
		altPaths := a.getAlternativeIconPaths(iconPath)
		for _, altPath := range altPaths {
			if data, err2 := a.extractFileFromAPK(apkPath, altPath); err2 == nil {
				iconData = data
				iconPath = altPath
				err = nil
				break
			}
		}
		if err != nil {
			return "", fmt.Errorf("failed to extract icon from APK: %w", err)
		}
	}

	// 4. Convert to base64
	// Determine image format from extension
	var mimeType string
	if strings.HasSuffix(iconPath, ".png") {
		mimeType = "image/png"
	} else if strings.HasSuffix(iconPath, ".jpg") || strings.HasSuffix(iconPath, ".jpeg") {
		mimeType = "image/jpeg"
	} else if strings.HasSuffix(iconPath, ".webp") {
		mimeType = "image/webp"
	} else {
		mimeType = "image/png" // Default
	}

	base64Str := base64.StdEncoding.EncodeToString(iconData)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Str), nil
}

// parseIconPathFromAapt2 tries to parse icon path from aapt2 output (alternative format)
func (a *App) parseIconPathFromAapt2(output string) string {
	// aapt2 may output icon information differently
	// Look for icon references in the output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Try to find icon references like: icon='res/mipmap-mdpi/ic_launcher.png'
		// or in XML format or other formats
		if strings.Contains(line, "icon=") {
			// Extract path after icon=
			parts := strings.Split(line, "icon=")
			if len(parts) >= 2 {
				iconPath := strings.Trim(parts[1], "'\"")
				iconPath = strings.TrimSpace(iconPath)
				// Remove any trailing characters that might be part of the line
				if idx := strings.IndexAny(iconPath, " \t\n"); idx > 0 {
					iconPath = iconPath[:idx]
				}
				if iconPath != "" && (strings.HasSuffix(iconPath, ".png") ||
					strings.HasSuffix(iconPath, ".jpg") ||
					strings.HasSuffix(iconPath, ".jpeg") ||
					strings.HasSuffix(iconPath, ".webp")) {
					return iconPath
				}
			}
		}
		// Also try to find in package: line or other formats
		if strings.Contains(line, "package:") && strings.Contains(line, "icon") {
			// Try to extract icon from package line
			if idx := strings.Index(line, "icon='"); idx > 0 {
				start := idx + 6
				if end := strings.Index(line[start:], "'"); end > 0 {
					iconPath := line[start : start+end]
					if iconPath != "" {
						return iconPath
					}
				}
			}
		}
	}
	return ""
}

// getAlternativeIconPaths returns alternative paths to try if the primary path fails
func (a *App) getAlternativeIconPaths(originalPath string) []string {
	var alternatives []string

	// Try different density folders
	densities := []string{"xxxhdpi", "xxhdpi", "xhdpi", "hdpi", "mdpi", "ldpi"}
	for _, density := range densities {
		if strings.Contains(originalPath, "mipmap-") {
			alt := strings.Replace(originalPath, "mipmap-", "mipmap-"+density+"-", 1)
			alternatives = append(alternatives, alt)
		}
		if strings.Contains(originalPath, "drawable-") {
			alt := strings.Replace(originalPath, "drawable-", "drawable-"+density+"-", 1)
			alternatives = append(alternatives, alt)
		}
	}

	// Try common icon names
	iconNames := []string{"ic_launcher.png", "ic_launcher_foreground.png", "ic_launcher_round.png", "icon.png"}
	baseDir := filepath.Dir(originalPath)
	for _, iconName := range iconNames {
		alternatives = append(alternatives, filepath.Join(baseDir, iconName))
	}

	return alternatives
}

// parseIconPathFromAapt parses the icon path from aapt dump badging output
func (a *App) parseIconPathFromAapt(output string) string {
	if output == "" {
		return ""
	}

	lines := strings.Split(output, "\n")

	// Look for application-icon-* entries (prefer higher resolution icons)
	iconSizes := []string{"480", "320", "240", "160", "120", "80", "48"}
	for _, size := range iconSizes {
		prefix := fmt.Sprintf("application-icon-%s:", size)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, prefix) {
				iconPath := strings.TrimPrefix(line, prefix)
				iconPath = strings.Trim(iconPath, "'\"")
				iconPath = strings.TrimSpace(iconPath)
				if iconPath != "" {
					return iconPath
				}
			}
		}
	}

	// Fallback: look for application-icon: (default icon)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "application-icon:") {
			iconPath := strings.TrimPrefix(line, "application-icon:")
			iconPath = strings.Trim(iconPath, "'\"")
			iconPath = strings.TrimSpace(iconPath)
			if iconPath != "" {
				return iconPath
			}
		}
	}

	// NEW Fallback: look for application: ... icon='...'
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "application:") && strings.Contains(line, "icon='") {
			idx := strings.Index(line, "icon='")
			if idx > 0 {
				start := idx + 6
				end := strings.Index(line[start:], "'")
				if end > 0 {
					iconPath := line[start : start+end]
					if iconPath != "" {
						return iconPath
					}
				}
			}
		}
	}

	return ""
}

// extractFileFromAPK extracts a file from an APK (which is a zip file)
func (a *App) extractFileFromAPK(apkPath, filePath string) ([]byte, error) {
	r, err := zip.OpenReader(apkPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// Try exact path first
	for _, f := range r.File {
		if f.Name == filePath {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}

	// Try without leading "res/"
	if strings.HasPrefix(filePath, "res/") {
		filePath = strings.TrimPrefix(filePath, "res/")
		for _, f := range r.File {
			if f.Name == filePath || strings.HasSuffix(f.Name, filePath) {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
	}

	// Try to find any file with similar name (for different densities)
	fileName := filepath.Base(filePath)
	for _, f := range r.File {
		if strings.Contains(f.Name, fileName) && (strings.Contains(f.Name, "mipmap") || strings.Contains(f.Name, "drawable")) {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err == nil {
				return data, nil
			}
		}
	}

	return nil, fmt.Errorf("file not found in APK: %s", filePath)
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
			// Try to parse basic model from -l
			for _, p := range parts {
				if strings.HasPrefix(p, "model:") {
					device.Model = strings.TrimPrefix(p, "model:")
				}
			}
			devices = append(devices, device)
		}
	}

	// Fetch details in parallel for authorized devices
	var wg sync.WaitGroup
	for i := range devices {
		if devices[i].State == "device" {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				d := &devices[idx]

				// Fetch manufacturer and model in one shell command to reduce overhead
				// Format: manufacturer;model
				cmd := exec.CommandContext(ctx, a.adbPath, "-s", d.ID, "shell", "getprop ro.product.manufacturer; getprop ro.product.model")
				out, err := cmd.Output()
				if err == nil {
					parts := strings.Split(string(out), "\n")
					if len(parts) >= 1 {
						d.Brand = strings.TrimSpace(parts[0])
					}
					if len(parts) >= 2 {
						refinedModel := strings.TrimSpace(parts[1])
						if refinedModel != "" {
							d.Model = refinedModel
						}
					}
				}
			}(i)
		}
	}
	wg.Wait()

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
// packageType: "user" for user apps only, "system" for system apps only, "all" for both
func (a *App) ListPackages(deviceId string, packageType string) ([]AppPackage, error) {
	if deviceId == "" {
		return nil, fmt.Errorf("no device specified")
	}

	// Default to user apps if not specified
	if packageType == "" {
		packageType = "user"
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

	// Fetch packages based on packageType
	if packageType == "all" {
		// Fetch system packages
		if err := fetch("-s", "system"); err != nil {
			return nil, fmt.Errorf("failed to list system packages: %w", err)
		}
		// Fetch 3rd party packages
		if err := fetch("-3", "user"); err != nil {
			return nil, fmt.Errorf("failed to list user packages: %w", err)
		}
	} else if packageType == "system" {
		// Fetch system packages only
		if err := fetch("-s", "system"); err != nil {
			return nil, fmt.Errorf("failed to list system packages: %w", err)
		}
	} else {
		// Default: fetch user packages only
		if err := fetch("-3", "user"); err != nil {
			return nil, fmt.Errorf("failed to list user packages: %w", err)
		}
	}

	// 2. Fetch labels and icons from cache in parallel (no longer calls aapt automatically)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // Higher concurrency for memory-only operations

	for i := range packages {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			pkg := &packages[idx]

			// Load from cache if available
			a.aaptCacheMu.RLock()
			if cached, ok := a.aaptCache[pkg.Name]; ok {
				if cached.Label != "" {
					pkg.Label = cached.Label
				}
				if cached.Icon != "" {
					pkg.Icon = cached.Icon
				}
				if cached.VersionName != "" {
					pkg.VersionName = cached.VersionName
				}
				if cached.VersionCode != "" {
					pkg.VersionCode = cached.VersionCode
				}
				if cached.MinSdkVersion != "" {
					pkg.MinSdkVersion = cached.MinSdkVersion
				}
				if cached.TargetSdkVersion != "" {
					pkg.TargetSdkVersion = cached.TargetSdkVersion
				}
				if len(cached.Permissions) > 0 {
					pkg.Permissions = cached.Permissions
				}
			}
			a.aaptCacheMu.RUnlock()

			// Fallback: Try to get label using brand map if cache didn't work
			if pkg.Label == "" {
				// Improved Label Extraction Logic
				// 1. Try to extract from package name using a smarter brand map
				brandMap := map[string]string{
					"com.ss.android.ugc.tiktok.lite": "TikTok Lite",
					"com.zhiliaoapp.musically":       "TikTok",
					"com.ss.android.ugc.aweme":       "Douyin",
					"com.google.android.youtube":     "YouTube",
					"com.google.android.gms":         "Google Play Services",
					"com.android.vending":            "Google Play Store",
					"com.whatsapp":                   "WhatsApp",
					"com.facebook.katana":            "Facebook",
					"com.facebook.orca":              "Messenger",
					"com.instagram.android":          "Instagram",
				}

				if brand, ok := brandMap[pkg.Name]; ok {
					pkg.Label = brand
				} else {
					// 2. Generic brand extraction
					parts := strings.Split(pkg.Name, ".")
					var meaningful []string
					skip := map[string]bool{
						"com": true, "net": true, "org": true, "android": true,
						"google": true, "ss": true, "ugc": true, "app": true,
					}
					for _, p := range parts {
						if !skip[strings.ToLower(p)] && len(p) > 2 {
							meaningful = append(meaningful, p)
						}
					}
					if len(meaningful) == 0 {
						meaningful = parts[len(parts)-1:]
					}
					for i, p := range meaningful {
						meaningful[i] = strings.ToUpper(p[:1]) + p[1:]
					}
					pkg.Label = strings.Join(meaningful, " ")
				}
			}
		}(i)
	}
	wg.Wait()

	return packages, nil
}

// UninstallApp uninstalls an app
func (a *App) UninstallApp(deviceId, packageName string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}

	fmt.Printf("Uninstalling %s from %s\n", packageName, deviceId)

	// Try standard uninstall first
	cmd := exec.Command(a.adbPath, "-s", deviceId, "uninstall", packageName)
	output, err := cmd.CombinedOutput()
	outStr := string(output)

	// adb uninstall sometimes returns 0 but prints Failure [DELETE_FAILED_INTERNAL_ERROR] etc.
	if err == nil && !strings.Contains(outStr, "Failure") {
		return outStr, nil
	}

	// If it fails, it might be a system app or have other issues.
	// Try the shell pm uninstall -k --user 0 method which works for system apps (removes for current user)
	fmt.Printf("Standard uninstall failed for %s (Output: %s), trying pm uninstall --user 0...\n", packageName, outStr)
	cmd2 := exec.Command(a.adbPath, "-s", deviceId, "shell", "pm", "uninstall", "-k", "--user", "0", packageName)
	output2, err2 := cmd2.CombinedOutput()
	outStr2 := string(output2)
	if err2 != nil || strings.Contains(outStr2, "Failure") {
		return outStr2, fmt.Errorf("failed to uninstall: %s", outStr2)
	}

	return outStr2, nil
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

// StartApp launches the application using monkey command
func (a *App) StartApp(deviceId, packageName string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}
	// Launch the main activity using monkey
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "monkey", "-p", packageName, "-c", "android.intent.category.LAUNCHER", "1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to start app: %w", err)
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

// InstallAPK installs an APK to the specified device
func (a *App) InstallAPK(deviceId string, path string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device selected")
	}

	fmt.Printf("Installing APK %s to device %s\n", path, deviceId)

	cmd := exec.Command(a.adbPath, "-s", deviceId, "install", "-r", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to install APK: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// ExportAPK extracts an installed APK from the device to the local machine
func (a *App) ExportAPK(deviceId string, packageName string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}

	// 1. Get the remote path of the APK
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "pm", "path", packageName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get APK path: %w", err)
	}

	remotePath := strings.TrimSpace(string(output))
	// Output is usually in format "package:/data/app/.../base.apk"
	lines := strings.Split(remotePath, "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "package:") {
		return "", fmt.Errorf("unexpected output from pm path: %s", remotePath)
	}
	remotePath = strings.TrimPrefix(lines[0], "package:")

	// 2. Open Save File Dialog
	fileName := packageName + ".apk"
	defaultDir, _ := os.UserHomeDir()
	downloadsDir := filepath.Join(defaultDir, "Downloads")
	// Check if Downloads exists, if not fallback to home
	if _, err := os.Stat(downloadsDir); err == nil {
		defaultDir = downloadsDir
	}

	savePath, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		DefaultFilename: fileName,
		Title:           "Export APK",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Android Package (*.apk)", Pattern: "*.apk"},
		},
		DefaultDirectory: defaultDir,
	})

	if err != nil {
		return "", fmt.Errorf("failed to open save dialog: %w", err)
	}
	if savePath == "" {
		return "", nil // User cancelled
	}

	// 3. Pull the file
	pullCmd := exec.Command(a.adbPath, "-s", deviceId, "pull", remotePath, savePath)
	pullOutput, err := pullCmd.CombinedOutput()
	if err != nil {
		return string(pullOutput), fmt.Errorf("failed to pull APK: %w (output: %s)", err, string(pullOutput))
	}

	return savePath, nil
}

// ListFiles returns a list of files in the specified directory on the device
func (a *App) ListFiles(deviceId, path string) ([]FileInfo, error) {
	if deviceId == "" {
		return nil, fmt.Errorf("no device specified")
	}

	// Clean the path and ensure it's absolute
	path = filepath.ToSlash(path)
	if path == "" || path == "." {
		path = "/"
	}

	// Use ls -la and CombinedOutput to capture both stdout and stderr
	// We quote the path to handle spaces correctly in adb shell
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "ls", "-la", "\""+path+"\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w (output: %s)", err, string(output))
	}

	lines := strings.Split(string(output), "\n")
	var files []FileInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 7 {
			continue
		}

		mode := parts[0]
		isDir := strings.HasPrefix(mode, "d")
		isLink := strings.HasPrefix(mode, "l")

		var name string
		var size int64
		var modTime string

		// Find the arrow for symlinks
		arrowIdx := -1
		for i, p := range parts {
			if p == "->" {
				arrowIdx = i
				break
			}
		}

		if arrowIdx > 0 {
			// Symlink case: [mode] ... [date] [time] [name] -> [target]
			name = parts[arrowIdx-1]
			// Look backwards from name to find date and time
			if arrowIdx >= 3 {
				modTime = parts[arrowIdx-3] + " " + parts[arrowIdx-2]
			}
			isDir = true
		} else {
			// Regular file/dir case: [mode] ... [size] [date] [time] [name]
			name = parts[len(parts)-1]
			if len(parts) >= 3 {
				// Try to find if parts[len-3] and parts[len-2] look like date and time
				t1 := parts[len(parts)-3]
				t2 := parts[len(parts)-2]
				modTime = t1 + " " + t2
			}
		}

		// Cleanup: If modTime contains only '?' or doesn't look like a date, set to N/A
		if strings.Contains(modTime, "?") || len(modTime) < 5 {
			modTime = "N/A"
		}

		// Cleanup name: handle cases where name might be parsed as '?'
		if name == "?" {
			// Try to find the last non-question mark part
			for i := len(parts) - 1; i >= 0; i-- {
				if parts[i] != "?" && parts[i] != "->" {
					name = parts[i]
					break
				}
			}
		}

		// Final name cleanup: if it's a path, get the base
		if strings.Contains(name, "/") && !isLink {
			name = filepath.Base(name)
		}

		// Find size (the first numeric field that isn't the links count at index 1)
		foundSize := false
		for i := 1; i < len(parts); i++ {
			if !foundSize {
				// Don't treat permissions or single digit link counts as size
				if len(parts[i]) > 1 && parts[i][0] >= '0' && parts[i][0] <= '9' {
					if n, err := fmt.Sscanf(parts[i], "%d", &size); n > 0 && err == nil {
						foundSize = true
						break
					}
				}
			}
		}

		if name == "." || name == ".." || name == "" {
			continue
		}

		// Build full path safely using forward slashes for Android
		fullPath := strings.TrimSuffix(path, "/") + "/" + name

		files = append(files, FileInfo{
			Name:    name,
			Size:    size,
			Mode:    mode,
			ModTime: modTime,
			IsDir:   isDir,
			Path:    fullPath,
		})
	}

	return files, nil
}

// DeleteFile deletes a file or directory on the device
func (a *App) DeleteFile(deviceId, path string) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "rm", "-rf", path)
	return cmd.Run()
}

// MoveFile moves or renames a file or directory on the device
func (a *App) MoveFile(deviceId, src, dest string) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "mv", src, dest)
	return cmd.Run()
}

// CopyFile copies a file or directory on the device
func (a *App) CopyFile(deviceId, src, dest string) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}
	// Use cp -R for recursive copy
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "cp", "-R", src, dest)
	return cmd.Run()
}

// Mkdir creates a new directory on the device
func (a *App) Mkdir(deviceId, path string) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "mkdir", "-p", path)
	return cmd.Run()
}
