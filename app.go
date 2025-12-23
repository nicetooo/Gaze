package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
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
	aaptCache       map[string]AppPackage
	aaptCacheMu     sync.RWMutex
	cachePath       string
	scrcpyCmds      map[string]*exec.Cmd
	scrcpyRecordCmd map[string]*exec.Cmd
	scrcpyMu        sync.Mutex

	openFileCmds map[string]*exec.Cmd
	openFileMu   sync.Mutex

	// Wireless Server
	httpServer *http.Server
	localAddr  string

	// History
	historyPath string
	historyMu   sync.Mutex

	version string

	// Last active tracking
	lastActive   map[string]int64
	lastActiveMu sync.RWMutex

	pinnedSerial string
	pinnedMu     sync.RWMutex

	runtimeLogs []string
	logsMu      sync.Mutex

	lastDevCount int

	// Wireless stability
	reconnectCooldown map[string]time.Time
	reconnectMu       sync.Mutex
}

type HistoryDevice struct {
	ID       string    `json:"id"`
	Serial   string    `json:"serial"`
	Model    string    `json:"model"`
	Brand    string    `json:"brand"`
	Type     string    `json:"type"`
	WifiAddr string    `json:"wifiAddr"`
	LastSeen time.Time `json:"lastSeen"`
}

type Device struct {
	ID         string   `json:"id"`
	Serial     string   `json:"serial"`
	State      string   `json:"state"`
	Model      string   `json:"model"`
	Brand      string   `json:"brand"`
	Type       string   `json:"type"` // "wired", "wireless", or "both"
	IDs        []string `json:"ids"`  // Store all adb IDs (e.g. [serial, 192.168.1.1:5555])
	WifiAddr   string   `json:"wifiAddr"`
	LastActive int64    `json:"lastActive"`
	IsPinned   bool     `json:"isPinned"`
}

type DeviceInfo struct {
	Model        string            `json:"model"`
	Brand        string            `json:"brand"`
	Manufacturer string            `json:"manufacturer"`
	AndroidVer   string            `json:"androidVer"`
	SDK          string            `json:"sdk"`
	ABI          string            `json:"abi"`
	Serial       string            `json:"serial"`
	Resolution   string            `json:"resolution"`
	Density      string            `json:"density"`
	CPU          string            `json:"cpu"`
	Memory       string            `json:"memory"`
	Props        map[string]string `json:"props"`
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
	Name                 string   `json:"name"`
	Label                string   `json:"label"` // Application label/name
	Icon                 string   `json:"icon"`  // Base64 encoded icon
	Type                 string   `json:"type"`  // "system" or "user"
	State                string   `json:"state"` // "enabled" or "disabled"
	VersionName          string   `json:"versionName"`
	VersionCode          string   `json:"versionCode"`
	MinSdkVersion        string   `json:"minSdkVersion"`
	TargetSdkVersion     string   `json:"targetSdkVersion"`
	Permissions          []string `json:"permissions"`
	Activities           []string `json:"activities"`
	LaunchableActivities []string `json:"launchableActivities"`
}

func NewApp(version string) *App {
	app := &App{
		aaptCache:         make(map[string]AppPackage),
		scrcpyCmds:        make(map[string]*exec.Cmd),
		scrcpyRecordCmd:   make(map[string]*exec.Cmd),
		openFileCmds:      make(map[string]*exec.Cmd),
		lastActive:        make(map[string]int64),
		reconnectCooldown: make(map[string]time.Time),
		version:           version,
	}
	app.initPersistentCache()
	return app
}

// updateLastActive updates the last active timestamp for a device (resolving to Serial if possible)
func (a *App) updateLastActive(deviceId string) {
	if deviceId == "" {
		return
	}

	// Try to find the true serial for this deviceId
	serial := deviceId
	devices, _ := a.GetDevices(false)
	for _, d := range devices {
		if d.ID == deviceId {
			serial = d.Serial
			break
		}
	}

	a.lastActiveMu.Lock()
	defer a.lastActiveMu.Unlock()
	a.lastActive[serial] = time.Now().Unix()
}

// TogglePinDevice pins/unpins a device by its serial. Only one device can be pinned.
func (a *App) TogglePinDevice(serial string) {
	a.pinnedMu.Lock()
	defer a.pinnedMu.Unlock()
	if a.pinnedSerial == serial {
		a.pinnedSerial = ""
	} else {
		a.pinnedSerial = serial
	}
}

// GetAppVersion returns the application version
func (a *App) GetAppVersion() string {
	return a.version
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

// Shutdown is called when the application is closing
func (a *App) Shutdown(ctx context.Context) {
	a.scrcpyMu.Lock()
	for id, cmd := range a.scrcpyCmds {
		if cmd.Process != nil {
			os.Stderr.WriteString(fmt.Sprintf("\n[SHUTDOWN] Killing mirroring for %s\n", id))
			_ = cmd.Process.Kill()
		}
	}
	for id, cmd := range a.scrcpyRecordCmd {
		if cmd.Process != nil {
			os.Stderr.WriteString(fmt.Sprintf("\n[SHUTDOWN] Killing recording for %s\n", id))
			_ = cmd.Process.Kill()
		}
	}
	a.scrcpyMu.Unlock()
	a.StopLogcat()
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
	a.historyPath = filepath.Join(appConfigDir, "history.json")

	a.loadCache()
}

func (a *App) loadHistoryInternal() []HistoryDevice {
	var history []HistoryDevice
	if a.historyPath == "" {
		return history
	}
	data, err := os.ReadFile(a.historyPath)
	if err != nil {
		// File doesn't exist yet, return empty history
		return history
	}
	if err := json.Unmarshal(data, &history); err != nil {
		// Invalid JSON, return empty history
		a.Log("Error unmarshaling history: %v", err)
		return []HistoryDevice{}
	}
	return history
}

func (a *App) saveHistory(history []HistoryDevice) error {
	data, err := json.Marshal(history)
	if err != nil {
		a.Log("Failed to marshal history: %v", err)
		return err
	}
	err = os.WriteFile(a.historyPath, data, 0644)
	if err != nil {
		a.Log("Failed to write history to %s: %v", a.historyPath, err)
		return err
	}
	return nil
}

func (a *App) addToHistory(device Device) {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()

	history := a.loadHistoryInternal()
	found := false
	for i, d := range history {
		// Group by Serial if available, fallback to ID
		if (device.Serial != "" && d.Serial == device.Serial) || d.ID == device.ID {
			history[i].LastSeen = time.Now()
			history[i].Model = device.Model
			history[i].Brand = device.Brand
			history[i].Type = device.Type
			history[i].Serial = device.Serial
			history[i].WifiAddr = device.WifiAddr
			history[i].ID = device.ID // Update to latest ID
			found = true
			break
		}
	}

	if !found {
		history = append(history, HistoryDevice{
			ID:       device.ID,
			Serial:   device.Serial,
			Model:    device.Model,
			Brand:    device.Brand,
			Type:     device.Type,
			WifiAddr: device.WifiAddr,
			LastSeen: time.Now(),
		})
	}

	// Keep only last 20 devices
	if len(history) > 20 {
		history = history[len(history)-20:]
	}

	if err := a.saveHistory(history); err != nil {
		a.Log("Failed to save history in addToHistory: %v", err)
	}
}

func (a *App) GetHistoryDevices() []HistoryDevice {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	return a.loadHistoryInternal()
}

func (a *App) RemoveHistoryDevice(deviceId string) error {
	// Try to disconnect if it's a wireless device or currently connected
	// We ignore errors here because it might be a USB device or already disconnected
	_, _ = a.AdbDisconnect(deviceId)

	a.historyMu.Lock()
	defer a.historyMu.Unlock()

	history := a.loadHistoryInternal()
	var newHistory []HistoryDevice
	for _, d := range history {
		if d.ID != deviceId && d.Serial != deviceId {
			newHistory = append(newHistory, d)
		}
	}
	return a.saveHistory(newHistory)
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
		a.Log("Error marshaling cache: %v", err)
		return
	}

	err = os.WriteFile(a.cachePath, data, 0644)
	if err != nil {
		a.Log("Error saving cache to %s: %v", a.cachePath, err)
	}
}

func (a *App) setupBinaries() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	appBinDir := filepath.Join(configDir, "adbGUI", "bin")
	_ = os.MkdirAll(appBinDir, 0755)

	extract := func(name string, data []byte) string {
		if len(data) == 0 {
			return ""
		}

		path := filepath.Join(appBinDir, name)
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

		// Ensure executable permissions on Unix-like systems
		if runtime.GOOS != "windows" {
			_ = os.Chmod(path, 0755)
			// Remove macOS quarantine attribute if it exists
			if runtime.GOOS == "darwin" {
				_ = exec.Command("xattr", "-d", "com.apple.quarantine", path).Run()
			}
		}

		return path
	}

	a.adbPath = extract("adb", adbBinary)
	a.scrcpyPath = extract("scrcpy", scrcpyBinary)
	a.serverPath = extract("scrcpy-server", scrcpyServerBinary)

	// Fallback for adb: if bundled one doesn't work or isn't extracted, try system adb
	if a.adbPath != "" {
		cmd := exec.Command(a.adbPath, "version")
		if err := cmd.Run(); err != nil {
			fmt.Printf("Bundled adb failed, looking for system adb...\n")
			if path, err := exec.LookPath("adb"); err == nil {
				a.adbPath = path
				fmt.Printf("Using system adb at: %s\n", a.adbPath)
			}
		}
	} else {
		if path, err := exec.LookPath("adb"); err == nil {
			a.adbPath = path
			fmt.Printf("Using system adb as bundled adb is missing: %s\n", a.adbPath)
		}
	}

	// Extract aapt if available (may be empty placeholder)
	if len(aaptBinary) > 0 {
		a.aaptPath = extract("aapt", aaptBinary)
		fmt.Printf("AAPT setup at: %s\n", a.aaptPath)
	}

	a.Log("Binaries setup at: %s", appBinDir)
	a.Log("Final ADB path: %s", a.adbPath)
}

// Log adds a message to the runtime logs
func (a *App) Log(format string, args ...interface{}) {
	a.logsMu.Lock()
	defer a.logsMu.Unlock()
	msg := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
	a.runtimeLogs = append(a.runtimeLogs, msg)
	if len(a.runtimeLogs) > 1000 {
		a.runtimeLogs = a.runtimeLogs[len(a.runtimeLogs)-1000:]
	}
	// Also print to console
	fmt.Println(msg)
}

// GetBackendLogs returns the captured backend logs
func (a *App) GetBackendLogs() []string {
	a.logsMu.Lock()
	defer a.logsMu.Unlock()
	// Return a copy to avoid data races
	logs := make([]string, len(a.runtimeLogs))
	copy(logs, a.runtimeLogs)
	return logs
}

// normalizeActivityName ensures activity name is in format "package/full.class.Name"
func (a *App) normalizeActivityName(activity, packageName string) string {
	if !strings.Contains(activity, "/") {
		// If it's just a class name, prepend package
		if strings.HasPrefix(activity, ".") {
			return packageName + "/" + packageName + activity
		}
		return packageName + "/" + activity
	}

	parts := strings.SplitN(activity, "/", 2)
	pkg := parts[0]
	class := parts[1]

	if strings.HasPrefix(class, ".") {
		return pkg + "/" + pkg + class
	}
	return activity
}

// GetAppInfo returns detailed information for a specific package
func (a *App) GetAppInfo(deviceId, packageName string, force bool) (AppPackage, error) {
	// 1. 获取快速实时信息 (Activities, Permissions, State) via ADB
	pkg, _ := a.getAdbDetailedInfo(deviceId, packageName)

	// 2. 检查缓存中是否有 AAPT 信息 (Label, Icon, Versions)
	a.aaptCacheMu.RLock()
	cached, hasCache := a.aaptCache[packageName]
	a.aaptCacheMu.RUnlock()

	// 如果没有缓存，或者是强制刷新，或者是旧版本缓存（缺少 LaunchableActivities），则执行耗时的 AAPT 流程
	if force || !hasCache || cached.Label == "" || cached.LaunchableActivities == nil {
		detailedPkg, err := a.getAppInfoWithAapt(deviceId, packageName)
		if err == nil {
			// 合并 AAPT 信息到实时信息中
			pkg.Label = detailedPkg.Label
			pkg.Icon = detailedPkg.Icon
			pkg.VersionName = detailedPkg.VersionName
			pkg.VersionCode = detailedPkg.VersionCode
			pkg.MinSdkVersion = detailedPkg.MinSdkVersion
			pkg.TargetSdkVersion = detailedPkg.TargetSdkVersion
			pkg.LaunchableActivities = detailedPkg.LaunchableActivities

			// 合并 Activity 列表并去重
			if len(detailedPkg.Activities) > 0 {
				seen := make(map[string]bool)
				for _, act := range pkg.Activities {
					seen[act] = true
				}
				for _, act := range detailedPkg.Activities {
					if !seen[act] {
						pkg.Activities = append(pkg.Activities, act)
						seen[act] = true
					}
				}
			}
		}
	} else {
		// 使用缓存的静态信息
		pkg.Label = cached.Label
		pkg.Icon = cached.Icon
		pkg.VersionName = cached.VersionName
		pkg.VersionCode = cached.VersionCode
		pkg.MinSdkVersion = cached.MinSdkVersion
		pkg.TargetSdkVersion = cached.TargetSdkVersion
		pkg.LaunchableActivities = cached.LaunchableActivities

		// 如果 ADB 没拿到，从缓存拿
		if len(pkg.Activities) == 0 {
			pkg.Activities = cached.Activities
		}
	}

	return pkg, nil
}

// getAdbDetailedInfo 获取可以通过 ADB 快速获取的信息
func (a *App) getAdbDetailedInfo(deviceId, packageName string) (AppPackage, error) {
	var pkg AppPackage
	pkg.Name = packageName

	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "dumpsys", "package", packageName)
	output, err := cmd.Output()
	if err != nil {
		return pkg, err
	}

	outputStr := string(output)
	pkg.Activities = a.parseActivitiesFromDumpsys(outputStr, packageName)
	pkg.Permissions = a.parsePermissionsFromDumpsys(outputStr)

	return pkg, nil
}

// parseActivitiesFromDumpsys 从 dumpsys 输出中解析 Activity
func (a *App) parseActivitiesFromDumpsys(output, packageName string) []string {
	var activities []string
	seen := make(map[string]bool)
	lines := strings.Split(output, "\n")
	inActivities := false

	// 更加宽松的匹配模式：
	// 1. 匹配 com.package/.Activity
	// 2. 匹配 com.package/com.package.Activity
	// 3. 匹配格式如 "7d4b655 com.package/.Activity"
	pkgPattern := regexp.QuoteMeta(packageName)
	activityRegex := regexp.MustCompile(`(?i)(` + pkgPattern + `\/[\.\w\$]+)`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 识别进入 Activities 区域
		if strings.EqualFold(trimmed, "Activities:") {
			inActivities = true
			continue
		}

		if inActivities {
			// 如果遇到新的顶级分类（通常是不带空格且以冒号结尾的行），停止解析
			if len(line) > 0 && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.HasSuffix(trimmed, ":") {
				inActivities = false
				continue
			}

			// 尝试提取符合格式的组件名
			matches := activityRegex.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				act := a.normalizeActivityName(match[1], packageName)
				if !seen[act] {
					activities = append(activities, act)
					seen[act] = true
				}
			}
		}
	}

	// 兜底方案：如果没找到带 Activities: 标签的区域，全文本搜索一遍
	if len(activities) == 0 {
		matches := activityRegex.FindAllStringSubmatch(output, -1)
		for _, match := range matches {
			act := a.normalizeActivityName(match[1], packageName)
			if !seen[act] {
				activities = append(activities, act)
				seen[act] = true
			}
		}
	}

	return activities
}

// parsePermissionsFromDumpsys 从 dumpsys 输出中解析权限
func (a *App) parsePermissionsFromDumpsys(output string) []string {
	var permissions []string
	lines := strings.Split(output, "\n")
	inPermissions := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "requested permissions:") {
			inPermissions = true
			continue
		}
		if inPermissions {
			if strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, "android.permission") {
				inPermissions = false
				continue
			}
			// 权限行通常是 android.permission.INTERNET 这种格式
			if strings.HasPrefix(trimmed, "android.permission") || strings.Contains(trimmed, "permission") {
				perm := strings.Split(trimmed, ":")[0]
				permissions = append(permissions, strings.TrimSpace(perm))
			}
		}
	}
	return permissions
}

// getAppInfoWithAapt extracts app label, icon and other metadata using aapt
func (a *App) getAppInfoWithAapt(deviceId, packageName string) (AppPackage, error) {
	var pkg AppPackage
	pkg.Name = packageName

	if a.aaptPath == "" {
		return pkg, fmt.Errorf("aapt not available (binary not embedded)")
	}

	// Check if aapt file actually exists and is not empty
	if info, err := os.Stat(a.aaptPath); err != nil || info.Size() == 0 {
		return pkg, fmt.Errorf("aapt not available (file missing or empty)")
	}

	// 1. Get APK path from device
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

	// 3. Pull APK to local
	pullCmd := exec.Command(a.adbPath, "-s", deviceId, "pull", remotePath, tmpAPK)
	pullOutput, err := pullCmd.CombinedOutput()
	if err != nil {
		return pkg, fmt.Errorf("failed to pull APK: %w (output: %s)", err, string(pullOutput))
	}

	// 4. Use aapt to dump badging
	aaptCmd := exec.Command(a.aaptPath, "dump", "badging", tmpAPK)
	aaptOutput, err := aaptCmd.CombinedOutput()
	if err != nil {
		return pkg, fmt.Errorf("failed to run aapt: %w, output: %s", err, string(aaptOutput))
	}

	// 5. Parse information from aapt output
	outputStr := string(aaptOutput)
	pkg.Label = a.parseLabelFromAapt(outputStr)
	pkg.VersionName, pkg.VersionCode = a.parseVersionFromAapt(outputStr)
	pkg.MinSdkVersion = a.parseSdkVersionFromAapt(outputStr, "sdkVersion:")
	pkg.TargetSdkVersion = a.parseSdkVersionFromAapt(outputStr, "targetSdkVersion:")
	pkg.LaunchableActivities = a.parseActivitiesFromAapt(outputStr, packageName)
	pkg.Activities = pkg.LaunchableActivities

	// 6. Extract icon using aapt
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

// OpenSettings opens a specific system settings page with optional data URI
func (a *App) OpenSettings(deviceId string, action string, data string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}

	// Default to main settings if no specific action provided
	if action == "" {
		action = "android.settings.SETTINGS"
	}

	args := []string{"-s", deviceId, "shell", "am", "start", "-a", action}
	if data != "" {
		args = append(args, "-d", data)
	}

	cmd := exec.Command(a.adbPath, args...)
	output, err := cmd.CombinedOutput()
	outStr := string(output)

	if err != nil {
		return outStr, fmt.Errorf("failed to open settings: %w", err)
	}

	if strings.Contains(outStr, "Error:") || strings.Contains(outStr, "Exception") {
		return outStr, fmt.Errorf("failed to open settings: %s", outStr)
	}

	return outStr, nil
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

// StartActivity launches a specific activity
func (a *App) StartActivity(deviceId, activityName string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}
	// activityName is usually in format "com.example/.MainActivity"
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "am", "start", "-n", activityName)
	output, err := cmd.CombinedOutput()
	outStr := string(output)

	if err != nil {
		return outStr, fmt.Errorf("failed to start activity: %w", err)
	}

	// 检查输出内容是否包含错误关键字，因为 am start 即使失败也可能返回退出码 0
	if strings.Contains(outStr, "Error:") || strings.Contains(outStr, "Exception") || strings.Contains(outStr, "requires") {
		return outStr, fmt.Errorf("failed to start activity: %s", outStr)
	}

	return outStr, nil
}

// parseActivitiesFromAapt 从 aapt dump badging 输出中解析启动 Activity
func (a *App) parseActivitiesFromAapt(output, packageName string) []string {
	var activities []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "launchable-activity:") {
			// 格式: launchable-activity: name='com.example.MainActivity' label='App' icon=''
			idx := strings.Index(line, "name='")
			if idx > 0 {
				start := idx + 6
				end := strings.Index(line[start:], "'")
				if end > 0 {
					name := line[start : start+end]
					// 转换为标准格式: packageName/fullClassName
					name = a.normalizeActivityName(name, packageName)
					activities = append(activities, name)
				}
			}
		}
	}
	return activities
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
	preferredLocales := []string{"en", "zh-TW", "zh-CN", "zh", ""}
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

// AdbPair pairs a device using the given address and code
func (a *App) AdbPair(address string, code string) (string, error) {
	if address == "" || code == "" {
		return "", fmt.Errorf("address and pairing code are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.adbPath, "pair", address, code)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("pairing failed: %w, output: %s", err, string(output))
	}
	return string(output), nil
}

// AdbConnect connects to a device using the given address
func (a *App) AdbConnect(address string) (string, error) {
	if address == "" {
		return "", fmt.Errorf("address is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.adbPath, "connect", address)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("connection failed: %w, output: %s", err, string(output))
	}
	return string(output), nil
}

// GetDeviceIP gets the local IP address of the device
func (a *App) GetDeviceIP(deviceId string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}

	// Try to get IP from ip addr show wlan0
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "ip addr show wlan0 | grep 'inet ' | awk '{print $2}' | cut -d/ -f1")
	output, err := cmd.CombinedOutput()
	ip := strings.TrimSpace(string(output))

	if err != nil || ip == "" {
		// Fallback: try getprop
		cmd = exec.Command(a.adbPath, "-s", deviceId, "shell", "getprop dhcp.wlan0.ipaddress")
		output, _ = cmd.CombinedOutput()
		ip = strings.TrimSpace(string(output))
	}

	if ip == "" {
		return "", fmt.Errorf("could not find device IP (ensure Wi-Fi is on)")
	}
	return ip, nil
}

// SwitchToWireless enables TCP/IP mode on the device and connects to it
func (a *App) SwitchToWireless(deviceId string) (string, error) {
	// 1. Get the IP first while still connected via USB
	ip, err := a.GetDeviceIP(deviceId)
	if err != nil {
		return "", err
	}

	// 2. Enable TCP mode on port 5555
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.adbPath, "-s", deviceId, "tcpip", "5555")
	if out, err := cmd.CombinedOutput(); err != nil {
		return string(out), fmt.Errorf("failed to enable tcpip mode: %w", err)
	}

	// 3. Wait a bit for the daemon to restart
	time.Sleep(1 * time.Second)

	// 4. Connect wirelessly
	return a.AdbConnect(ip + ":5555")
}

// GetLocalIP returns the first non-loopback local IPv4 address
func (a *App) GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// StartWirelessServer starts a temporary http server for QR code connection
func (a *App) StartWirelessServer() (string, error) {
	if a.httpServer != nil {
		return a.localAddr, nil
	}

	ip := a.GetLocalIP()
	if ip == "" {
		return "", fmt.Errorf("could not find local IP")
	}

	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	a.localAddr = fmt.Sprintf("http://%s:%d", ip, port)

	mux := http.NewServeMux()
	mux.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) {
		// Get phone's IP
		remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		a.Log("Wireless connect request from: %s", remoteIP)

		// Try to connect (default port 5555)
		output, err := a.AdbConnect(remoteIP + ":5555")
		success := err == nil && strings.Contains(output, "connected to")

		if success {
			wailsRuntime.EventsEmit(a.ctx, "wireless-connected", remoteIP)
		} else {
			wailsRuntime.EventsEmit(a.ctx, "wireless-connect-failed", map[string]string{
				"ip":    remoteIP,
				"error": output,
			})
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		var title, statusClass, message, hint, nextSteps string
		if success {
			title = "连接成功"
			statusClass = "success"
			message = "设备已成功连接到电脑"
			hint = "现在您可以关闭此页面并在电脑上操作了"
			nextSteps = ""
		} else {
			title = "连接失败"
			statusClass = "error"
			message = "无法连接到 ADB 服务"
			hint = "错误信息: " + strings.ReplaceAll(output, "\n", " ")
			nextSteps = `
				<div class="next-steps">
					<h3>后续操作建议：</h3>
					<ul>
						<li>检查手机 <b>无线调试</b> 是否已开启</li>
						<li>确保手机和电脑在 <b>同一个局域网</b></li>
						<li>如果手机使用了 <b>随机端口</b> (非 5555)，请在电脑上使用“无线配对”功能</li>
						<li>尝试重新扫码</li>
					</ul>
				</div>
			`
		}

		fmt.Fprintf(w, `
			<!DOCTYPE html>
			<html>
			<head>
				<meta charset="utf-8">
				<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
				<style>
					body {
						display: flex;
						flex-direction: column;
						align-items: center;
						justify-content: center;
						min-height: 100vh;
						margin: 0;
						font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
						background-color: #f5f5f5;
						color: #333;
					}
					.card {
						background: white;
						padding: 2rem;
						border-radius: 12px;
						box-shadow: 0 4px 6px rgba(0,0,0,0.1);
						text-align: center;
						width: 85%%;
						max-width: 400px;
					}
					h1 { margin-bottom: 1rem; font-size: 1.5rem; }
					.success h1 { color: #52c41a; }
					.error h1 { color: #ff4d4f; }
					p { font-size: 1.1rem; line-height: 1.5; margin: 0.5rem 0; }
					.ip-badge {
						display: inline-block;
						background: #e6f4ff;
						color: #0958d9;
						padding: 0.2rem 0.6rem;
						border-radius: 4px;
						font-family: monospace;
						font-weight: bold;
					}
					.hint { font-size: 0.9rem; color: #666; margin-top: 1rem; padding: 10px; background: #fafafa; border-radius: 4px; }
					.next-steps { text-align: left; margin-top: 1.5rem; border-top: 1px solid #eee; padding-top: 1rem; }
					.next-steps h3 { font-size: 1rem; margin-bottom: 0.5rem; }
					.next-steps ul { padding-left: 1.2rem; font-size: 0.9rem; color: #555; }
					.next-steps li { margin-bottom: 0.5rem; }
				</style>
			</head>
			<body class="%s">
				<div class="card">
					<h1>%s</h1>
					<p>手机 IP: <span class="ip-badge">%s</span></p>
					<p>%s</p>
					<div class="hint">%s</div>
					%s
				</div>
			</body>
			</html>
		`, statusClass, title, remoteIP, message, hint, nextSteps)
	})

	a.httpServer = &http.Server{Handler: mux}
	go a.httpServer.Serve(listener)

	return a.localAddr, nil
}

// AdbDisconnect disconnects from a wireless device (handles both IP and mDNS aliases)
func (a *App) AdbDisconnect(address string) (string, error) {
	if address == "" {
		return "", fmt.Errorf("address is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Split by comma in case multiple IDs were passed
	addresses := strings.Split(address, ",")
	var lastOut string
	var lastErr error

	for _, addr := range addresses {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		cmd := exec.CommandContext(ctx, a.adbPath, "disconnect", addr)
		output, err := cmd.CombinedOutput()
		lastOut = string(output)
		// If it fails with "no such device", it might be already disconnected
		if err != nil && !strings.Contains(string(output), "no such device") {
			lastErr = err
		}
	}

	if lastErr != nil {
		return lastOut, fmt.Errorf("disconnection failed: %w, output: %s", lastErr, lastOut)
	}
	return "disconnected", nil
}

// newAdbCommand creates an exec.Cmd with a clean environment to avoid proxy issues
func (a *App) newAdbCommand(ctx context.Context, args ...string) *exec.Cmd {
	var cmd *exec.Cmd
	if ctx != nil {
		cmd = exec.CommandContext(ctx, a.adbPath, args...)
	} else {
		cmd = exec.Command(a.adbPath, args...)
	}

	// Sanitize environment: remove proxies that might interfere with local ADB communication
	env := os.Environ()
	newEnv := make([]string, 0, len(env))
	proxyVars := []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "all_proxy", "no_proxy"}

	for _, e := range env {
		isProxy := false
		for _, v := range proxyVars {
			if strings.HasPrefix(e, v+"=") {
				isProxy = true
				break
			}
		}
		if !isProxy {
			newEnv = append(newEnv, e)
		}
	}
	cmd.Env = newEnv
	return cmd
}

// newScrcpyCommand creates an exec.Cmd for scrcpy with a clean environment
func (a *App) newScrcpyCommand(args ...string) *exec.Cmd {
	cmd := exec.Command(a.scrcpyPath, args...)

	// Sanitize environment: remove proxies
	env := os.Environ()
	newEnv := make([]string, 0, len(env))
	proxyVars := []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "all_proxy", "no_proxy"}

	for _, e := range env {
		isProxy := false
		for _, v := range proxyVars {
			if strings.HasPrefix(e, v+"=") {
				isProxy = true
				break
			}
		}
		if !isProxy {
			newEnv = append(newEnv, e)
		}
	}

	// Add scrcpy specific env vars
	newEnv = append(newEnv,
		"SCRCPY_SERVER_PATH="+a.serverPath,
		"ADB="+a.adbPath,
	)

	cmd.Env = newEnv
	return cmd
}

// tryAutoReconnect attempts to reconnect to a wireless device if it's offline or missing
func (a *App) tryAutoReconnect(address string) {
	if address == "" || (!strings.Contains(address, ":") && !strings.Contains(address, "._tcp")) {
		return
	}

	a.reconnectMu.Lock()
	last, ok := a.reconnectCooldown[address]
	if ok && time.Since(last) < 30*time.Second {
		a.reconnectMu.Unlock()
		return
	}
	a.reconnectCooldown[address] = time.Now()
	a.reconnectMu.Unlock()

	go func() {
		a.Log("Auto-reconnecting to wireless device: %s", address)
		// Use a short timeout for the connect command to not block
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := a.newAdbCommand(ctx, "connect", address)
		// We don't care much about the output here as we're just poking it
		_ = cmd.Run()
	}()
}

// GetDevices returns a list of connected ADB devices
func (a *App) GetDevices(forceLog bool) ([]Device, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if a.adbPath == "" {
		return nil, fmt.Errorf("ADB path is not initialized")
	}

	// 1. Get raw output from adb devices -l
	cmd := a.newAdbCommand(ctx, "devices", "-l")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run adb devices (path: %s): %w, output: %s", a.adbPath, err, string(output))
	}

	// Load history to help with device identification and metadata preservation
	a.historyMu.Lock()
	historyDevices := a.loadHistoryInternal()
	a.historyMu.Unlock()

	historyByID := make(map[string]HistoryDevice)
	historyBySerial := make(map[string]HistoryDevice)
	for _, hd := range historyDevices {
		if hd.ID != "" {
			historyByID[hd.ID] = hd
		}
		if hd.Serial != "" {
			historyBySerial[hd.Serial] = hd
		}
	}

	// 3. Parse raw identifiers
	lines := strings.Split(string(output), "\n")
	type adbNode struct {
		id         string
		state      string
		isWireless bool
		isMDNS     bool
		hasUSB     bool
		model      string
		serial     string // resolved hardware serial
	}
	var nodes []*adbNode

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices attached") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			node := &adbNode{
				id:    parts[0],
				state: parts[1],
			}
			// Parse properties
			for _, p := range parts[2:] {
				if strings.Contains(p, ":") {
					kv := strings.SplitN(p, ":", 2)
					if kv[0] == "model" {
						node.model = kv[1]
					}
					if kv[0] == "usb" {
						node.hasUSB = true
					}
				}
			}
			node.isWireless = strings.Contains(node.id, ":") || strings.Contains(node.id, "._tcp") || strings.Contains(node.id, "._adb-tls-connect")
			if node.hasUSB {
				node.isWireless = false
			}
			node.isMDNS = strings.Contains(node.id, "._tcp") || strings.Contains(node.id, "._adb-tls-connect")
			nodes = append(nodes, node)

			// OPTIMIZATION: If a wireless device is offline, try to reconnect it
			if node.isWireless && node.state == "offline" {
				a.tryAutoReconnect(node.id)
			}
		}
	}

	// 3.5. Proactively reconnect to recently active wireless devices missing from the current list
	// This helps when a proxy switch or network blip causes the device to drop off the adb daemon entirely.
	for _, hd := range historyDevices {
		if hd.WifiAddr != "" && time.Since(hd.LastSeen) < 15*time.Minute {
			found := false
			for _, n := range nodes {
				if n.id == hd.WifiAddr {
					found = true
					break
				}
			}
			if !found {
				a.tryAutoReconnect(hd.WifiAddr)
			}
		}
	}

	// a.Log("GetDevices found %d nodes from adb output", len(nodes))

	// Regex for mDNS serial extraction
	mdnsRe := regexp.MustCompile(`adb-([a-zA-Z0-9]+)-`)

	// 4. Phase 1: Resolve "True Serial" for every node
	var wg sync.WaitGroup
	for _, n := range nodes {
		wg.Add(1)
		go func(node *adbNode) {
			defer wg.Done()

			// A. If already authorised, ask the device
			if node.state == "device" {
				// Short timeout for serial fetch to prevent blocking
				sCtx, sCancel := context.WithTimeout(ctx, 3*time.Second)
				defer sCancel()
				c := exec.CommandContext(sCtx, a.adbPath, "-s", node.id, "shell", "getprop ro.serialno")
				out, err := c.Output()
				if err == nil {
					s := strings.TrimSpace(string(out))
					if s != "" {
						node.serial = s
						return
					}
				}
			}

			// B. Extract from mDNS ID if possible (format: adb-SERIAL-...)
			if node.isMDNS {
				matches := mdnsRe.FindStringSubmatch(node.id)
				if len(matches) > 1 {
					node.serial = matches[1]
					return
				}
			}

			// C. Try History by current ID
			if h, ok := historyByID[node.id]; ok && h.Serial != "" {
				node.serial = h.Serial
				return
			}

			// D. Fallback: use ID as serial for non-wireless or unknown
			if !node.isWireless {
				node.serial = node.id
			}
		}(n)
	}
	wg.Wait()

	// 5. Phase 2: Grouping by resolved Serial
	deviceMap := make(map[string]*Device)
	var finalDevices []*Device

	// We sort current nodes to ensure stable primary ID selection (prefer wired)
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].hasUSB != nodes[j].hasUSB {
			return nodes[i].hasUSB // Wired first
		}
		if nodes[i].state != nodes[j].state {
			return nodes[i].state == "device" // Online first
		}
		return !nodes[i].isMDNS // Prefer IP over mDNS
	})

	for _, n := range nodes {
		serialKey := n.serial
		if serialKey == "" {
			serialKey = n.id // Last resort
		}

		d, exists := deviceMap[serialKey]
		if !exists {
			d = &Device{
				ID:     n.id, // Primary ID for commands
				Serial: serialKey,
				State:  n.state,
				IDs:    []string{n.id},
				Model:  strings.TrimSpace(strings.ReplaceAll(n.model, "_", " ")),
			}
			if n.isWireless {
				d.Type = "wireless"
				d.WifiAddr = n.id
			} else {
				d.Type = "wired"
			}
			deviceMap[serialKey] = d
			finalDevices = append(finalDevices, d)
		} else {
			// Update existing record
			d.IDs = append(d.IDs, n.id)
			// Upgrade state or primary ID if preferred (prefer wired/USB for primary ID)
			if n.state == "device" {
				if d.State != "device" || n.hasUSB {
					d.State = "device"
					d.ID = n.id
				}
			}
			// Handle Connection Type
			if n.isWireless {
				// Prefer IP:Port over mDNS for WifiAddr field display
				if !strings.Contains(d.WifiAddr, ":") || strings.Contains(n.id, ":") {
					d.WifiAddr = n.id
				}
				if d.Type == "wired" {
					d.Type = "both"
				} else if d.Type == "" {
					d.Type = "wireless"
				}
			} else if n.hasUSB {
				if d.Type == "wireless" {
					d.Type = "both"
				} else if d.Type == "" {
					d.Type = "wired"
				}
			}
		}
	}

	// 6. Phase 3: Final Polishing (Metadata & History)
	for i := range finalDevices {
		dev := finalDevices[i] // Capture pointer

		// Normalize model (e.g. Pixel_7a -> Pixel 7a)
		dev.Model = strings.TrimSpace(strings.ReplaceAll(dev.Model, "_", " "))

		// Stability: if WifiAddr is an mDNS name, try to restore last known IP from history
		if (dev.Type == "wireless" || dev.Type == "both") && !strings.Contains(dev.WifiAddr, ":") {
			if h, ok := historyBySerial[dev.Serial]; ok && strings.Contains(h.WifiAddr, ":") {
				dev.WifiAddr = h.WifiAddr
			}
		}

		// Fill missing metadata from history
		if dev.Brand == "" || dev.Model == "" {
			// A. Match by Serial
			if h, ok := historyBySerial[dev.Serial]; ok {
				if dev.Brand == "" {
					dev.Brand = h.Brand
				}
				if dev.Model == "" {
					dev.Model = h.Model
				}
			}
			// B. Match by common IDs or WifiAddr if serial failed
			if dev.Brand == "" || dev.Model == "" {
				for _, hid := range dev.IDs {
					if h, ok := historyByID[hid]; ok {
						if dev.Brand == "" {
							dev.Brand = h.Brand
						}
						if dev.Model == "" {
							dev.Model = h.Model
						}
					}
				}
			}
		}

		// Fetch fresh metadata if online (faster timeout for responsiveness)
		if dev.State == "device" {
			wg.Add(1)
			go func(d *Device) {
				defer wg.Done()
				pCtx, pCancel := context.WithTimeout(ctx, 3*time.Second) // Shorter timeout for properties
				defer pCancel()
				cmd := exec.CommandContext(pCtx, a.adbPath, "-s", d.ID, "shell", "getprop ro.product.manufacturer; getprop ro.product.model")
				out, err := cmd.Output()
				if err == nil {
					parts := strings.Split(string(out), "\n")
					if len(parts) >= 1 && strings.TrimSpace(parts[0]) != "" {
						d.Brand = strings.TrimSpace(parts[0])
					}
					if len(parts) >= 2 && strings.TrimSpace(parts[1]) != "" {
						m := strings.TrimSpace(parts[1])
						d.Model = strings.ReplaceAll(m, "_", " ")
					}
				} else {
					a.Log("Failed to fetch props for %s: %v", d.ID, err)
				}
			}(dev)
		}
	}
	wg.Wait()

	// Sync to history
	for _, d := range finalDevices {
		if d.State == "device" {
			deviceCopy := *d
			go a.addToHistory(deviceCopy)
		}
	}

	// 7. Populating Metadata and Sorting
	a.lastActiveMu.RLock()
	a.pinnedMu.RLock()
	for i := range finalDevices {
		d := finalDevices[i]
		if ts, ok := a.lastActive[d.Serial]; ok {
			d.LastActive = ts
		}
		if d.Serial == a.pinnedSerial {
			d.IsPinned = true
		}
	}
	a.pinnedMu.RUnlock()
	a.lastActiveMu.RUnlock()

	// Sort: Pinned first, then by LastActive descending
	sort.SliceStable(finalDevices, func(i, j int) bool {
		if finalDevices[i].IsPinned != finalDevices[j].IsPinned {
			return finalDevices[i].IsPinned
		}
		return finalDevices[i].LastActive > finalDevices[j].LastActive
	})

	if forceLog || len(finalDevices) != a.lastDevCount {
		a.Log("GetDevices returning %d devices (prev: %d)", len(finalDevices), a.lastDevCount)
		a.lastDevCount = len(finalDevices)
	}
	// Return flat slice
	result := make([]Device, len(finalDevices))
	for i, d := range finalDevices {
		result[i] = *d
	}
	return result, nil
}

// SelectScreenshotPath returns a default screenshot path in the Downloads folder
func (a *App) SelectScreenshotPath(deviceModel string) (string, error) {
	defaultDir, _ := os.UserHomeDir()
	downloadsDir := filepath.Join(defaultDir, "Downloads")
	if _, err := os.Stat(downloadsDir); err == nil {
		defaultDir = downloadsDir
	}

	cleanModel := "Device"
	if deviceModel != "" {
		cleanModel = strings.ReplaceAll(deviceModel, " ", "_")
		reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
		cleanModel = reg.ReplaceAllString(cleanModel, "")
	}

	filename := fmt.Sprintf("Screenshot_%s_%s.png", cleanModel, time.Now().Format("20060102_150405"))
	fullPath := filepath.Join(defaultDir, filename)
	return fullPath, nil
}

// TakeScreenshot captures a screenshot of the device and saves it to the host
func (a *App) TakeScreenshot(deviceId, savePath string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}

	if savePath == "" {
		return "", fmt.Errorf("no save path specified")
	}

	a.updateLastActive(deviceId)

	// 4. Check if screen is truly active and unlocked
	// We use Case-Insensitive matching and cover multiple variants
	// Screen status: mWakefulness=Awake (Pixel), Display Power: state=ON (Xiaomi/Others)
	// Lock status: mKeyguardShowing=true, mShowingLockscreen=true
	checkCmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "dumpsys power | grep -iE 'state=|wakefulness=' ; dumpsys window | grep -iE 'keyguardShowing|showingLockscreen'")
	out, _ := checkCmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))

	// Screen is considered ON only if we explicitly see "Awake" or "state=ON"
	reOn := regexp.MustCompile(`(?i)wakefulness=Awake|state=ON|mDisplayState=ON`)
	isOn := reOn.MatchString(outStr)
	isOff := !isOn

	// Screen is considered LOCKED if we see "Showing=true" or "Lockscreen=true"
	reLocked := regexp.MustCompile(`(?i)(keyguardShowing|showingLockscreen).*true`)
	isLocked := reLocked.MatchString(outStr)

	if isOff || isLocked {
		wailsRuntime.EventsEmit(a.ctx, "screenshot-progress", "screenshot_off")
		return "", fmt.Errorf("SCREEN_OFF")
	}

	// 5. Capture screenshot via temp file on device then pull
	wailsRuntime.EventsEmit(a.ctx, "screenshot-progress", "screenshot_capturing")
	remotePath := "/sdcard/screenshot_tmp.png"
	capCmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "screencap", "-p", remotePath)
	if out, err := capCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to capture screenshot on device: %w, output: %s", err, string(out))
	}
	defer exec.Command(a.adbPath, "-s", deviceId, "shell", "rm", remotePath).Run()

	wailsRuntime.EventsEmit(a.ctx, "screenshot-progress", "screenshot_pulling")
	pullCmd := exec.Command(a.adbPath, "-s", deviceId, "pull", remotePath, savePath)
	if out, err := pullCmd.CombinedOutput(); err != nil {
		wailsRuntime.EventsEmit(a.ctx, "screenshot-progress", "screenshot_error", err.Error())
		return "", fmt.Errorf("failed to pull screenshot: %w, output: %s", err, string(out))
	}

	wailsRuntime.EventsEmit(a.ctx, "screenshot-progress", "screenshot_success", savePath)
	return savePath, nil
}

// GetDeviceInfo returns detailed information about a device
func (a *App) GetDeviceInfo(deviceId string) (DeviceInfo, error) {
	var info DeviceInfo
	info.Props = make(map[string]string)

	if deviceId == "" {
		return info, fmt.Errorf("no device specified")
	}

	// 1. Get properties
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "getprop")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Format: [prop.name]: [prop.value]
			parts := strings.SplitN(line, "]: [", 2)
			if len(parts) == 2 {
				key := strings.TrimPrefix(parts[0], "[")
				val := strings.TrimSuffix(parts[1], "]")
				info.Props[key] = val

				switch key {
				case "ro.product.model":
					info.Model = val
				case "ro.product.brand":
					info.Brand = val
				case "ro.product.manufacturer":
					info.Manufacturer = val
				case "ro.build.version.release":
					info.AndroidVer = val
				case "ro.build.version.sdk":
					info.SDK = val
				case "ro.product.cpu.abi":
					info.ABI = val
				case "ro.serialno":
					info.Serial = val
				}
			}
		}
	}

	// 2. Get resolution
	cmd = exec.Command(a.adbPath, "-s", deviceId, "shell", "wm", "size")
	out, err := cmd.Output()
	if err == nil {
		info.Resolution = strings.TrimSpace(strings.TrimPrefix(string(out), "Physical size: "))
	}

	// 3. Get density
	cmd = exec.Command(a.adbPath, "-s", deviceId, "shell", "wm", "density")
	out, err = cmd.Output()
	if err == nil {
		info.Density = strings.TrimSpace(strings.TrimPrefix(string(out), "Physical density: "))
	}

	// 4. Get CPU info (brief)
	cmd = exec.Command(a.adbPath, "-s", deviceId, "shell", "cat /proc/cpuinfo | grep 'Hardware' | head -1")
	out, err = cmd.Output()
	if err == nil && len(out) > 0 {
		info.CPU = strings.TrimSpace(strings.TrimPrefix(string(out), "Hardware\t: "))
	}
	if info.CPU == "" {
		// Try another way to get processor info
		cmd = exec.Command(a.adbPath, "-s", deviceId, "shell", "cat /proc/cpuinfo | grep 'processor' | wc -l")
		out, err = cmd.Output()
		if err == nil {
			cores := strings.TrimSpace(string(out))
			info.CPU = fmt.Sprintf("%s Core(s)", cores)
		}
	}

	// 5. Get Memory info
	cmd = exec.Command(a.adbPath, "-s", deviceId, "shell", "cat /proc/meminfo | grep 'MemTotal'")
	out, err = cmd.Output()
	if err == nil {
		info.Memory = strings.TrimSpace(strings.TrimPrefix(string(out), "MemTotal:"))
	}

	return info, nil
}

// RunAdbCommand executes an arbitrary ADB command
func (a *App) RunAdbCommand(args []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := a.newAdbCommand(ctx, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}
	return string(output), nil
}

// SelectRecordPath returns a default recording path in the Downloads folder
func (a *App) SelectRecordPath(deviceModel string) (string, error) {
	defaultDir, _ := os.UserHomeDir()
	downloadsDir := filepath.Join(defaultDir, "Downloads")
	if _, err := os.Stat(downloadsDir); err == nil {
		defaultDir = downloadsDir
	}

	cleanModel := "Device"
	if deviceModel != "" {
		cleanModel = strings.ReplaceAll(deviceModel, " ", "_")
		// Remove other potentially problematic characters
		reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
		cleanModel = reg.ReplaceAllString(cleanModel, "")
	}

	filename := fmt.Sprintf("adbGUI_%s_%s.mp4", cleanModel, time.Now().Format("20060102_150405"))
	fullPath := filepath.Join(defaultDir, filename)
	return fullPath, nil
}

// StartRecording starts a separate scrcpy process just for recording without a window
func (a *App) StartRecording(deviceId string, config ScrcpyConfig) error {
	a.updateLastActive(deviceId)
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}

	if config.RecordPath == "" {
		return fmt.Errorf("no record path specified")
	}

	a.scrcpyMu.Lock()
	if cmd, exists := a.scrcpyRecordCmd[deviceId]; exists && cmd.Process != nil {
		// If a recording process already exists, kill it to start a new one
		_ = cmd.Process.Kill()
	}
	a.scrcpyMu.Unlock()

	args := []string{"-s", deviceId, "--no-window", "--record", config.RecordPath}

	if config.MaxSize > 0 {
		args = append(args, "--max-size", fmt.Sprintf("%d", config.MaxSize))
	}
	if config.BitRate > 0 {
		args = append(args, "--video-bit-rate", fmt.Sprintf("%dM", config.BitRate))
	}
	if config.MaxFps > 0 {
		args = append(args, "--max-fps", fmt.Sprintf("%d", config.MaxFps))
	}
	if config.VideoCodec != "" {
		args = append(args, "--video-codec", config.VideoCodec)
	}

	if config.NoAudio {
		args = append(args, "--no-audio")
	} else if config.AudioCodec != "" {
		args = append(args, "--audio-codec", config.AudioCodec)
	}

	// Advanced arguments for recording: strictly separate source-specific flags
	if config.VideoSource == "camera" {
		args = append(args, "--video-source", "camera")
		if config.CameraId != "" {
			args = append(args, "--camera-id", config.CameraId)
		}
		if config.CameraSize != "" {
			args = append(args, "--camera-size", config.CameraSize)
		}
	} else {
		if config.VideoSource == "display" {
			args = append(args, "--video-source", "display")
		}
		if config.DisplayId > 0 {
			args = append(args, "--display-id", fmt.Sprintf("%d", config.DisplayId))
		}
	}
	if config.DisplayOrientation != "" && config.DisplayOrientation != "0" {
		args = append(args, "--display-orientation", config.DisplayOrientation)
	}

	cmd := a.newScrcpyCommand(args...)

	a.Log("Starting recording process: %s %v", a.scrcpyPath, cmd.Args)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start recording: %w", err)
	}

	a.scrcpyMu.Lock()
	a.scrcpyRecordCmd[deviceId] = cmd
	a.scrcpyMu.Unlock()

	wailsRuntime.EventsEmit(a.ctx, "scrcpy-record-started", map[string]interface{}{
		"deviceId":   deviceId,
		"recordPath": config.RecordPath,
		"startTime":  time.Now().Unix(),
	})

	go func() {
		_ = cmd.Wait()
		a.scrcpyMu.Lock()
		delete(a.scrcpyRecordCmd, deviceId)
		a.scrcpyMu.Unlock()
		wailsRuntime.EventsEmit(a.ctx, "scrcpy-record-stopped", deviceId)
	}()

	return nil
}

// StopRecording stops the recording process for the given device
func (a *App) StopRecording(deviceId string) error {
	a.scrcpyMu.Lock()
	defer a.scrcpyMu.Unlock()

	if cmd, exists := a.scrcpyRecordCmd[deviceId]; exists && cmd.Process != nil {
		// On Unix-like systems, send SIGINT for graceful shutdown (finalizes MP4/MKV)
		if runtime.GOOS != "windows" {
			return cmd.Process.Signal(os.Interrupt)
		}
		return cmd.Process.Kill()
	}
	return nil
}

// IsRecording checks if a recording process is running for the device
func (a *App) IsRecording(deviceId string) bool {
	a.scrcpyMu.Lock()
	defer a.scrcpyMu.Unlock()
	_, exists := a.scrcpyRecordCmd[deviceId]
	return exists
}

// ListCameras returns a list of available cameras for the given device
func (a *App) ListCameras(deviceId string) ([]string, error) {
	if deviceId == "" {
		return nil, fmt.Errorf("no device specified")
	}
	cmd := a.newScrcpyCommand("-s", deviceId, "--list-cameras")
	output, err := cmd.CombinedOutput()
	a.Log("ListCameras for %s: err=%v, output=%s", deviceId, err, string(output))

	lines := strings.Split(string(output), "\n")
	var cameras []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "--camera-id=") {
			cameras = append(cameras, line)
		}
	}
	return cameras, nil
}

// ListDisplays returns a list of available displays for the given device
func (a *App) ListDisplays(deviceId string) ([]string, error) {
	if deviceId == "" {
		return nil, fmt.Errorf("no device specified")
	}
	cmd := a.newScrcpyCommand("-s", deviceId, "--list-displays")
	output, err := cmd.CombinedOutput()
	a.Log("ListDisplays for %s: err=%v, output=%s", deviceId, err, string(output))

	lines := strings.Split(string(output), "\n")
	var displays []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "--display-id=") {
			displays = append(displays, line)
		}
	}
	return displays, nil
}

// OpenPath opens a file or directory in the default system browser
func (a *App) OpenPath(path string) error {
	if path == "::recordings::" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, "Downloads")
	}

	// Check if path is a directory
	info, err := os.Stat(path)
	isDir := err == nil && info.IsDir()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if isDir {
			cmd = exec.Command("explorer", filepath.Clean(path))
		} else {
			cmd = exec.Command("explorer", "/select,", filepath.Clean(path))
		}
	case "darwin":
		if isDir {
			cmd = exec.Command("open", path)
		} else {
			cmd = exec.Command("open", "-R", path)
		}
	default: // Linux
		if isDir {
			cmd = exec.Command("xdg-open", path)
		} else {
			cmd = exec.Command("xdg-open", filepath.Dir(path))
		}
	}
	return cmd.Start()
}

type ScrcpyConfig struct {
	MaxSize          int    `json:"maxSize"`
	BitRate          int    `json:"bitRate"`
	MaxFps           int    `json:"maxFps"`
	StayAwake        bool   `json:"stayAwake"`
	TurnScreenOff    bool   `json:"turnScreenOff"`
	NoAudio          bool   `json:"noAudio"`
	AlwaysOnTop      bool   `json:"alwaysOnTop"`
	ShowTouches      bool   `json:"showTouches"`
	Fullscreen       bool   `json:"fullscreen"`
	ReadOnly         bool   `json:"readOnly"`
	PowerOffOnClose  bool   `json:"powerOffOnClose"`
	WindowBorderless bool   `json:"windowBorderless"`
	VideoCodec       string `json:"videoCodec"`
	AudioCodec       string `json:"audioCodec"`
	RecordPath       string `json:"recordPath"`
	// Advanced options
	DisplayId          int    `json:"displayId"`
	VideoSource        string `json:"videoSource"` // "display" or "camera"
	CameraId           string `json:"cameraId"`
	CameraSize         string `json:"cameraSize"`
	DisplayOrientation string `json:"displayOrientation"`
	CaptureOrientation string `json:"captureOrientation"`
	KeyboardMode       string `json:"keyboardMode"` // "sdk" or "uhid"
	MouseMode          string `json:"mouseMode"`    // "sdk" or "uhid"
	NoClipboardSync    bool   `json:"noClipboardSync"`
	ShowFps            bool   `json:"showFps"`
	NoPowerOn          bool   `json:"noPowerOn"`
}

// StartScrcpy starts scrcpy for the given device with custom configuration
func (a *App) StartScrcpy(deviceId string, config ScrcpyConfig) error {
	a.updateLastActive(deviceId)
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}

	a.scrcpyMu.Lock()
	if cmd, exists := a.scrcpyCmds[deviceId]; exists && cmd.Process != nil {
		// If already running, we might want to kill it or just return error
		// For now, let's kill it to restart with new config
		_ = cmd.Process.Kill()
	}
	a.scrcpyMu.Unlock()

	args := []string{"-s", deviceId}
	// ... (rest of args) ...
	if config.MaxSize > 0 {
		args = append(args, "--max-size", fmt.Sprintf("%d", config.MaxSize))
	}
	if config.BitRate > 0 {
		args = append(args, "--video-bit-rate", fmt.Sprintf("%dM", config.BitRate))
	}
	if config.MaxFps > 0 {
		args = append(args, "--max-fps", fmt.Sprintf("%d", config.MaxFps))
	}
	isCamera := config.VideoSource == "camera"

	if config.StayAwake && !isCamera {
		args = append(args, "--stay-awake")
	}
	if config.TurnScreenOff && !isCamera {
		args = append(args, "--turn-screen-off")
	}
	if config.VideoCodec != "" {
		args = append(args, "--video-codec", config.VideoCodec)
	}

	if config.NoAudio {
		args = append(args, "--no-audio")
	} else if config.AudioCodec != "" {
		args = append(args, "--audio-codec", config.AudioCodec)
	}

	if config.AlwaysOnTop {
		args = append(args, "--always-on-top")
	}
	if config.ShowTouches && !isCamera {
		args = append(args, "--show-touches")
		// Manually set system setting via ADB to ensure mouse clicks from scrcpy are also visible
		go a.RunAdbCommand([]string{"-s", deviceId, "shell", "settings", "put", "system", "show_touches", "1"})
	} else if !isCamera {
		// Ensure it's off if explicitly requested to be off
		go a.RunAdbCommand([]string{"-s", deviceId, "shell", "settings", "put", "system", "show_touches", "0"})
	}
	if config.Fullscreen {
		args = append(args, "--fullscreen")
	}
	if config.ReadOnly {
		args = append(args, "--no-control")
	}
	if config.PowerOffOnClose && !isCamera {
		args = append(args, "--power-off-on-close")
	}
	if config.WindowBorderless {
		args = append(args, "--window-borderless")
	}

	// Advanced arguments: strictly separate source-specific flags
	if config.VideoSource == "camera" {
		args = append(args, "--video-source", "camera")
		if config.CameraId != "" {
			args = append(args, "--camera-id", config.CameraId)
		}
		if config.CameraSize != "" {
			args = append(args, "--camera-size", config.CameraSize)
		}
	} else {
		// Default or explicit display
		if config.VideoSource == "display" {
			args = append(args, "--video-source", "display")
		}
		if config.DisplayId > 0 {
			args = append(args, "--display-id", fmt.Sprintf("%d", config.DisplayId))
		}
	}
	if config.DisplayOrientation != "" && config.DisplayOrientation != "0" {
		args = append(args, "--display-orientation", config.DisplayOrientation)
	}
	if config.CaptureOrientation != "" && config.CaptureOrientation != "0" {
		args = append(args, "--capture-orientation", config.CaptureOrientation)
	}
	if config.KeyboardMode != "" && config.KeyboardMode != "sdk" {
		args = append(args, "--keyboard", config.KeyboardMode)
	}
	if config.MouseMode != "" && config.MouseMode != "sdk" {
		args = append(args, "--mouse", config.MouseMode)
	}
	if config.NoClipboardSync {
		args = append(args, "--no-clipboard-autosync")
	}
	if config.ShowFps {
		// print-fps is for logs, but some versions support a visual counter or we just enable it
		args = append(args, "--print-fps")
	}
	if config.NoPowerOn {
		args = append(args, "--no-power-on")
	}

	// Double check: scrcpy camera mirroring often requires no-audio to start reliably
	if config.VideoSource == "camera" {
		foundNoAudio := false
		for _, arg := range args {
			if arg == "--no-audio" {
				foundNoAudio = true
				break
			}
		}
		if !foundNoAudio {
			args = append(args, "--no-audio")
		}
	}

	args = append(args, "--window-title", "ADB GUI - "+deviceId)

	cmd := a.newScrcpyCommand(args...)

	// Capture output for error reporting
	var stderrBuf bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	a.Log("Starting scrcpy: %s %v", a.scrcpyPath, cmd.Args)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start scrcpy: %w", err)
	}

	a.scrcpyMu.Lock()
	a.scrcpyCmds[deviceId] = cmd
	a.scrcpyMu.Unlock()

	startTime := time.Now()

	// Notify frontend that scrcpy has started
	wailsRuntime.EventsEmit(a.ctx, "scrcpy-started", map[string]interface{}{
		"deviceId":  deviceId,
		"startTime": startTime.Unix(),
	})

	// Wait for process to exit in a goroutine
	go func() {
		err := cmd.Wait()
		duration := time.Since(startTime)

		a.scrcpyMu.Lock()
		defer a.scrcpyMu.Unlock()

		// Only cleanup and emit event if this is still the active command
		if a.scrcpyCmds[deviceId] == cmd {
			delete(a.scrcpyCmds, deviceId)

			if err != nil && duration < 5*time.Second {
				// If it failed quickly, report the error details
				errorMsg := stderrBuf.String()
				if errorMsg == "" {
					errorMsg = err.Error()
				}
				a.Log("Scrcpy failed quickly (%v): %s", duration, errorMsg)
				wailsRuntime.EventsEmit(a.ctx, "scrcpy-failed", map[string]interface{}{
					"deviceId": deviceId,
					"error":    errorMsg,
				})
			} else {
				wailsRuntime.EventsEmit(a.ctx, "scrcpy-stopped", deviceId)
			}
		}
	}()

	return nil
}

// StopScrcpy stops scrcpy for the given device
func (a *App) StopScrcpy(deviceId string) error {
	a.scrcpyMu.Lock()
	defer a.scrcpyMu.Unlock()

	if cmd, exists := a.scrcpyCmds[deviceId]; exists && cmd.Process != nil {
		// Mirroring windows can be killed immediately for better responsiveness
		return cmd.Process.Kill()
	}
	return nil
}

// StartLogcat starts the logcat stream for a device, optionally filtering by package name
func (a *App) StartLogcat(deviceId, packageName string) error {
	a.updateLastActive(deviceId)
	// If logcat is already running, stop it first to ensure a clean state
	if a.logcatCmd != nil {
		a.StopLogcat()
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
	a.updateLastActive(deviceId)
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}

	a.Log("Uninstalling %s from %s", packageName, deviceId)

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
	a.updateLastActive(deviceId)
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

	a.Log("Installing APK %s to device %s", path, deviceId)

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
func (a *App) ListFiles(deviceId, pathStr string) ([]FileInfo, error) {
	a.updateLastActive(deviceId)
	if deviceId == "" {
		return nil, fmt.Errorf("no device specified")
	}

	pathStr = path.Clean("/" + pathStr)
	cmdPath := pathStr
	if cmdPath != "/" {
		cmdPath += "/"
	}

	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "ls", "-la", "\""+cmdPath+"\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w (output: %s)", err, string(output))
	}

	// Regex to match typical Android ls date/time formats
	// 1. YYYY-MM-DD HH:MM (Modern Toybox)
	// 2. MMM DD HH:MM or MMM DD  YYYY (Older formats)
	dateTimeRegex := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2})|([A-Z][a-z]{2}\s+\d{1,2}\s+(\d{2}:\d{2}|\d{4}))`)

	lines := strings.Split(string(output), "\n")
	var files []FileInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}

		// Find the date-time anchor in the line
		loc := dateTimeRegex.FindStringIndex(line)
		if loc == nil {
			continue
		}

		modTime := line[loc[0]:loc[1]]

		// Everything after the date-time is the name (+ maybe link target)
		afterDateTime := strings.TrimSpace(line[loc[1]:])

		// Everything before the date-time is permissions, user, size, etc.
		beforeDateTime := strings.TrimSpace(line[:loc[0]])
		beforeParts := strings.Fields(beforeDateTime)

		if len(beforeParts) < 1 {
			continue
		}

		mode := beforeParts[0]
		isDir := strings.HasPrefix(mode, "d")
		isLink := strings.HasPrefix(mode, "l")

		// Size is usually the last field before date-time
		var size int64
		if len(beforeParts) >= 1 {
			// Try to parse the last part as size
			fmt.Sscanf(beforeParts[len(beforeParts)-1], "%d", &size)
		}

		// Handle name and symlinks
		name := afterDateTime
		if isLink {
			arrowIdx := strings.Index(name, " -> ")
			if arrowIdx != -1 {
				name = name[:arrowIdx]
			}
			// In many Android contexts, we want to treat symlinks to directories as directories
			isDir = true
		}

		// Skip current dir, parent dir, or entries that match the directory itself
		// Use path.Base for name comparison
		cleanName := strings.TrimSpace(name)
		if cleanName == "." || cleanName == ".." || cleanName == "" || cleanName == "?" {
			continue
		}

		// If the name is exactly the path we are listing (some ls outputs repeat it), skip
		if cleanName == path.Base(pathStr) || cleanName == pathStr {
			continue
		}

		files = append(files, FileInfo{
			Name:    cleanName,
			Size:    size,
			Mode:    mode,
			ModTime: modTime,
			IsDir:   isDir,
			Path:    path.Join(pathStr, cleanName),
		})
	}

	return files, nil
}

// OpenFileOnHost pulls a file from the device to a temporary location and opens it
func (a *App) OpenFileOnHost(deviceId, remotePath string) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}

	// 1. Create a temporary local path
	fileName := path.Base(remotePath)
	tmpDir := filepath.Join(os.TempDir(), "adb-gui-open")
	_ = os.MkdirAll(tmpDir, 0755)
	localPath := filepath.Join(tmpDir, fileName)

	// 2. Pull the file from device
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, a.adbPath, "-s", deviceId, "pull", remotePath, localPath)

	a.openFileMu.Lock()
	a.openFileCmds[remotePath] = cmd
	a.openFileMu.Unlock()

	defer func() {
		a.openFileMu.Lock()
		delete(a.openFileCmds, remotePath)
		a.openFileMu.Unlock()
	}()

	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("open cancelled")
		}
		return fmt.Errorf("failed to pull file: %w, output: %s", err, string(output))
	}

	// 3. Open the file with system default
	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		openCmd = exec.Command("cmd", "/c", "start", "", localPath)
	case "darwin":
		openCmd = exec.Command("open", localPath)
	default: // Linux
		openCmd = exec.Command("xdg-open", localPath)
	}

	return openCmd.Start()
}

// CancelOpenFile cancels the pull process for a specific file
func (a *App) CancelOpenFile(remotePath string) {
	a.openFileMu.Lock()
	defer a.openFileMu.Unlock()
	if cmd, exists := a.openFileCmds[remotePath]; exists {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		delete(a.openFileCmds, remotePath)
	}
}

// DownloadFile pulls a file from the device to a user-selected local path
func (a *App) DownloadFile(deviceId, remotePath string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}

	fileName := path.Base(remotePath)
	defaultDir, _ := os.UserHomeDir()
	downloadsDir := filepath.Join(defaultDir, "Downloads")
	if _, err := os.Stat(downloadsDir); err == nil {
		defaultDir = downloadsDir
	}

	savePath, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		DefaultFilename:  fileName,
		Title:            "Download File",
		DefaultDirectory: defaultDir,
	})

	if err != nil {
		return "", err
	}
	if savePath == "" {
		return "", nil // Cancelled
	}

	cmd := exec.Command(a.adbPath, "-s", deviceId, "pull", remotePath, savePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to download file: %w, output: %s", err, string(output))
	}

	return savePath, nil
}

// GetThumbnail returns a base64 encoded thumbnail for an image or video file
func (a *App) GetThumbnail(deviceId, remotePath, modTime string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}

	ext := strings.ToLower(filepath.Ext(remotePath))
	isImage := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" || ext == ".gif"
	isVideo := ext == ".mp4" || ext == ".mkv" || ext == ".mov" || ext == ".avi"

	if !isImage && !isVideo {
		return "", fmt.Errorf("unsupported file type")
	}

	// 1. Check cache
	configDir, _ := os.UserConfigDir()
	thumbDir := filepath.Join(configDir, "adbGUI", "thumbnails")
	_ = os.MkdirAll(thumbDir, 0755)

	cacheKey := fmt.Sprintf("%x", md5.Sum([]byte(deviceId+remotePath+modTime+"v2")))
	cachePath := filepath.Join(thumbDir, cacheKey+".jpg")

	if _, err := os.Stat(cachePath); err == nil {
		data, err := os.ReadFile(cachePath)
		if err == nil {
			return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data), nil
		}
	}

	// 2. Pull file and generate thumbnail
	tmpDir := filepath.Join(os.TempDir(), "adb-gui-thumb")
	_ = os.MkdirAll(tmpDir, 0755)
	localPath := filepath.Join(tmpDir, cacheKey+ext)
	defer os.Remove(localPath)

	pullCmd := exec.Command(a.adbPath, "-s", deviceId, "pull", remotePath, localPath)
	if err := pullCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to pull file: %w", err)
	}

	var thumbData []byte
	var err error

	if isImage {
		thumbData, err = a.generateImageThumbnail(localPath)
	} else if isVideo {
		thumbData, err = a.generateVideoThumbnail(localPath)
	}

	if err != nil {
		return "", err
	}

	// 3. Save to cache
	_ = os.WriteFile(cachePath, thumbData, 0644)

	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(thumbData), nil
}

func (a *App) generateImageThumbnail(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	targetSize := 512
	scale := 1
	if width > targetSize || height > targetSize {
		if width > height {
			scale = width / targetSize
		} else {
			scale = height / targetSize
		}
	}

	if scale < 1 {
		scale = 1
	}

	newWidth := width / scale
	newHeight := height / scale
	thumb := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			thumb.Set(x, y, img.At(x*scale, y*scale))
		}
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 70})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *App) generateVideoThumbnail(localPath string) ([]byte, error) {
	// Try to use ffmpeg if available
	tmpThumb := localPath + ".jpg"
	defer os.Remove(tmpThumb)

	cmd := exec.Command("ffmpeg", "-y", "-i", localPath, "-ss", "00:00:01", "-vframes", "1", "-s", "512x512", "-f", "image2", tmpThumb)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg not available or failed: %w", err)
	}

	return os.ReadFile(tmpThumb)
}

// DeleteFile deletes a file or directory on the device
func (a *App) DeleteFile(deviceId, pathStr string) error {
	a.updateLastActive(deviceId)
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}
	pathStr = path.Clean("/" + pathStr)
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "rm", "-rf", "\""+pathStr+"\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// MoveFile moves or renames a file or directory on the device
func (a *App) MoveFile(deviceId, src, dest string) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}
	src = path.Clean("/" + src)
	dest = path.Clean("/" + dest)
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "mv", "\""+src+"\"", "\""+dest+"\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// CopyFile copies a file or directory on the device
func (a *App) CopyFile(deviceId, src, dest string) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}
	src = path.Clean("/" + src)
	dest = path.Clean("/" + dest)
	// Use cp -R for recursive copy
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "cp", "-R", "\""+src+"\"", "\""+dest+"\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// Mkdir creates a new directory on the device
func (a *App) Mkdir(deviceId, pathStr string) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}
	pathStr = path.Clean("/" + pathStr)
	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "mkdir", "-p", "\""+pathStr+"\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}
