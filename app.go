package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"Gaze/pkg/cache"

	"github.com/google/uuid"
)

// Binaries are embedded in platform-specific files (bin_*.go) and bin_common.go

// App struct
type App struct {
	ctx              context.Context
	ctxCancel        context.CancelFunc // For MCP mode cleanup
	adbPath          string
	scrcpyPath       string
	serverPath       string
	aaptPath         string
	ffmpegPath       string
	ffprobePath      string
	adbKeyboardPath  string // Path to extracted ADBKeyboard APK for Unicode text input
	protocPath       string // Path to extracted protoc binary
	protocIncludeDir string // Path to extracted protoc well-known type includes
	logcatCmd        *exec.Cmd
	logcatCancel     context.CancelFunc

	// Generic mutex for shared state
	mu sync.Mutex

	// Services
	cacheService *cache.Service

	// History (still managed by device.go)
	historyMu sync.Mutex

	// Scrcpy process management
	scrcpyCmds      map[string]*exec.Cmd
	scrcpyRecordCmd map[string]*exec.Cmd
	scrcpyMu        sync.Mutex

	// File open process management
	openFileCmds map[string]*exec.Cmd
	openFileMu   sync.Mutex

	// Wireless Server
	httpServer *http.Server
	localAddr  string

	version string

	// Runtime logs
	runtimeLogs []string
	logsMu      sync.Mutex

	// Device tracking
	lastDevCount int
	idToSerial   map[string]string
	idToSerialMu sync.RWMutex

	// Wireless stability
	reconnectCooldown map[string]time.Time
	reconnectMu       sync.Mutex

	// Device monitor
	deviceMonitorCancel context.CancelFunc
	deviceMonitorMu     sync.Mutex

	// Session device monitors (per device)
	sessionMonitors   map[string]*DeviceMonitor
	sessionMonitorsMu sync.Mutex

	// Event System (new)
	eventStore      *EventStore
	eventPipeline   *EventPipeline
	assertionEngine *AssertionEngine
	pluginStore     *PluginStore
	pluginManager   *PluginManager
	eventSystemMu   sync.RWMutex
	dataDir         string

	// MCP mode flag (no Wails GUI)
	mcpMode bool

	// Workflow file watcher (for MCP → GUI sync)
	workflowWatcher *WorkflowWatcher
}

// NewApp creates a new App instance
func NewApp(version string) *App {
	app := &App{
		scrcpyCmds:        make(map[string]*exec.Cmd),
		scrcpyRecordCmd:   make(map[string]*exec.Cmd),
		openFileCmds:      make(map[string]*exec.Cmd),
		idToSerial:        make(map[string]string),
		reconnectCooldown: make(map[string]time.Time),
		sessionMonitors:   make(map[string]*DeviceMonitor),
		version:           version,
	}
	app.initCacheService()
	return app
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.initCore()

	// Start workflow file watcher for MCP → GUI sync
	a.workflowWatcher = NewWorkflowWatcher(a)
	if err := a.workflowWatcher.Start(); err != nil {
		LogWarn("app").Err(err).Msg("Failed to start workflow watcher")
	}
}

// Shutdown is called when the application is closing
func (a *App) Shutdown(ctx context.Context) {
	LogAppState(StateShuttingDown, map[string]interface{}{
		"reason": "application_close",
	})

	// Stop workflow watcher
	if a.workflowWatcher != nil {
		a.workflowWatcher.Stop()
	}

	a.shutdownCore()
}

// GetAppVersion returns the application version
func (a *App) GetAppVersion() string {
	return a.version
}

// GetStorageInfo returns storage location and total size used by the app
func (a *App) GetStorageInfo() map[string]interface{} {
	result := map[string]interface{}{
		"dataDir":       "",
		"totalSize":     int64(0),
		"dbSize":        int64(0),
		"recordingSize": int64(0),
		"binSize":       int64(0),
		"workflowSize":  int64(0),
		"scriptSize":    int64(0),
		"logSize":       int64(0),
		"cacheSize":     int64(0),
		"otherSize":     int64(0),
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return result
	}

	gazeDir := filepath.Join(configDir, "Gaze")
	result["dataDir"] = gazeDir

	// Calculate total size by walking the directory
	var totalSize, dbSize, recordingSize, binSize, workflowSize, scriptSize, logSize, cacheSize int64
	_ = filepath.Walk(gazeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		size := info.Size()
		totalSize += size

		rel, _ := filepath.Rel(gazeDir, path)
		ext := filepath.Ext(info.Name())

		switch {
		case ext == ".db" || ext == ".db-wal" || ext == ".db-shm":
			dbSize += size
		case strings.HasPrefix(rel, "recordings"):
			recordingSize += size
		case strings.HasPrefix(rel, "bin"):
			binSize += size
		case strings.HasPrefix(rel, "workflows"):
			workflowSize += size
		case strings.HasPrefix(rel, "scripts") || strings.HasPrefix(rel, "tasks"):
			scriptSize += size
		case strings.HasPrefix(rel, "logs"):
			logSize += size
		case strings.HasPrefix(rel, "thumbnails") || strings.HasPrefix(rel, "video_cache") || strings.HasPrefix(rel, "cache"):
			cacheSize += size
		}
		return nil
	})

	classified := dbSize + recordingSize + binSize + workflowSize + scriptSize + logSize + cacheSize

	result["totalSize"] = totalSize
	result["dbSize"] = dbSize
	result["recordingSize"] = recordingSize
	result["binSize"] = binSize
	result["workflowSize"] = workflowSize
	result["scriptSize"] = scriptSize
	result["logSize"] = logSize
	result["cacheSize"] = cacheSize
	result["otherSize"] = totalSize - classified

	return result
}

// OpenDataDir opens the application data directory in the system file manager
func (a *App) OpenDataDir() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("cannot determine config directory: %w", err)
	}
	gazeDir := filepath.Join(configDir, "Gaze")

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", gazeDir)
	case "windows":
		cmd = exec.Command("explorer", gazeDir)
	default:
		cmd = exec.Command("xdg-open", gazeDir)
	}
	return cmd.Start()
}

// initCore contains the shared initialization logic for both GUI and MCP modes.
func (a *App) initCore() {
	a.setupBinaries()

	// Configure protoc paths for proto compiler (must be before LoadProtoConfig)
	getProtoRegistry().compiler.SetPaths(a.protocPath, a.protocIncludeDir)

	a.initEventSystem()
	a.StartDeviceMonitor()
	a.LoadMockRules()
	a.LoadBreakpointRules()
	a.LoadMapRemoteRules()
	a.LoadRewriteRules()
	a.LoadProtoConfig()
	a.SetupBreakpointCallbacks()
}

// shutdownCore contains the shared shutdown logic for both GUI and MCP modes.
func (a *App) shutdownCore() {
	// Cancel context if available (MCP mode creates its own cancellable context)
	if a.ctxCancel != nil {
		a.ctxCancel()
	}

	// Stop proxy and clean up device settings to prevent network issues
	if a.GetProxyStatus() {
		a.StopProxy()
	}

	a.shutdownEventSystem()

	a.scrcpyMu.Lock()
	for id, cmd := range a.scrcpyCmds {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			LogInfo("shutdown").Str("device", id).Msg("Killed mirroring process")
		}
	}
	for id, cmd := range a.scrcpyRecordCmd {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			LogInfo("shutdown").Str("device", id).Msg("Killed recording process")
		}
	}
	a.scrcpyMu.Unlock()

	a.StopLogcat()
	a.StopDeviceMonitor()
	a.stopAllTouchRecordings()
	a.stopAllActiveTasks()
	a.StopAllDeviceStateMonitors()
	a.stopAllSessionMonitors()
	a.StopAllNetworkMonitors()
	a.stopAllOpenFileCommands()

	LogAppState(StateStopped, nil)
	CloseLogger()
}

// InitializeWithoutGUI initializes the app for non-GUI mode (MCP server)
func (a *App) InitializeWithoutGUI() {
	// Create a cancellable context for MCP mode
	ctx, cancel := context.WithCancel(context.Background())
	a.ctx = ctx
	a.ctxCancel = cancel
	a.mcpMode = true // No Wails GUI, skip EventsEmit calls
	a.initCore()
}

// IsMCPMode returns true if running in MCP server mode (no GUI)
func (a *App) IsMCPMode() bool {
	return a.mcpMode
}

// ShutdownWithoutGUI shuts down the app in non-GUI mode
func (a *App) ShutdownWithoutGUI() {
	LogAppState(StateShuttingDown, map[string]interface{}{
		"reason": "mcp_server_shutdown",
	})
	a.shutdownCore()
}

// stopAllSessionMonitors stops all DeviceMonitors tracked in a.sessionMonitors.
// These are created by StartSessionWithConfig and are separate from the package-level deviceStateMonitors.
func (a *App) stopAllSessionMonitors() {
	a.sessionMonitorsMu.Lock()
	defer a.sessionMonitorsMu.Unlock()

	for deviceId, monitor := range a.sessionMonitors {
		monitor.Stop()
		LogInfo("shutdown").Str("device", deviceId).Msg("Stopped session monitor")
	}
	a.sessionMonitors = make(map[string]*DeviceMonitor)
}

// stopAllOpenFileCommands kills all in-flight adb pull commands for file opening.
func (a *App) stopAllOpenFileCommands() {
	a.openFileMu.Lock()
	defer a.openFileMu.Unlock()

	for path, cmd := range a.openFileCmds {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			LogInfo("shutdown").Str("path", path).Msg("Killed open file command")
		}
	}
	a.openFileCmds = make(map[string]*exec.Cmd)
}

// Log adds a message to the runtime logs (legacy method, forwards to zerolog)
func (a *App) Log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)

	// Forward to structured logger
	LogInfo("app").Msg(msg)

	// Keep legacy runtime logs for frontend display
	a.logsMu.Lock()
	defer a.logsMu.Unlock()
	timestampedMsg := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
	a.runtimeLogs = append(a.runtimeLogs, timestampedMsg)
	if len(a.runtimeLogs) > 1000 {
		a.runtimeLogs = a.runtimeLogs[len(a.runtimeLogs)-1000:]
	}
}

// GetBackendLogs returns the captured backend logs
func (a *App) GetBackendLogs() []string {
	a.logsMu.Lock()
	defer a.logsMu.Unlock()
	logs := make([]string, len(a.runtimeLogs))
	copy(logs, a.runtimeLogs)
	return logs
}

// updateLastActive updates the last active timestamp for a device
func (a *App) updateLastActive(deviceId string) {
	if deviceId == "" || a.cacheService == nil {
		return
	}

	serial := deviceId
	a.idToSerialMu.RLock()
	if s, ok := a.idToSerial[deviceId]; ok {
		serial = s
	}
	a.idToSerialMu.RUnlock()

	a.cacheService.SetLastActive(serial, time.Now().Unix())
	go a.saveSettings()
}

// Initialization functions

func (a *App) initCacheService() {
	svc, err := cache.New(cache.Config{
		LogFunc: a.Log,
	})
	if err != nil {
		a.Log("Error initializing cache service: %v", err)
		return
	}
	a.cacheService = svc
}

// saveSettings delegates to cache service
func (a *App) saveSettings() {
	if a.cacheService != nil {
		_ = a.cacheService.SaveSettings()
	}
}

// saveCache delegates to cache service
func (a *App) saveCache() {
	if a.cacheService != nil {
		_ = a.cacheService.SaveCache()
	}
}

func (a *App) setupBinaries() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	appBinDir := filepath.Join(configDir, "Gaze", "bin")
	_ = os.MkdirAll(appBinDir, 0755)

	// Version stamp: force re-extraction after an app upgrade, since the
	// size-comparison shortcut below misses new binaries of identical size
	stampPath := filepath.Join(appBinDir, ".version")
	stamp, _ := os.ReadFile(stampPath)
	force := string(stamp) != a.version
	extractOK := true // any failed write keeps the stamp stale so next launch retries

	extract := func(name string, data []byte) string {
		if len(data) == 0 {
			return ""
		}

		path := filepath.Join(appBinDir, name)
		if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") && name != "scrcpy-server" {
			path += ".exe"
		}

		info, err := os.Stat(path)
		if force || err != nil || info.Size() != int64(len(data)) {
			err = os.WriteFile(path, data, 0755)
			if err != nil {
				extractOK = false
				LogDebug("app").Str("name", name).Err(err).Msg("Error extracting embedded binary")
			}
		}

		if runtime.GOOS != "windows" {
			_ = os.Chmod(path, 0755)
			if runtime.GOOS == "darwin" {
				_ = exec.Command("xattr", "-d", "com.apple.quarantine", path).Run()
			}
		}

		return path
	}

	// Prefer system ADB if available
	if path, err := exec.LookPath("adb"); err == nil {
		a.adbPath = path
		a.Log("Using system adb found in PATH: %s", a.adbPath)
	} else {
		a.adbPath = extract("adb", adbBinary)
		if a.adbPath != "" {
			a.Log("Using bundled adb at: %s", a.adbPath)
		}
	}

	a.scrcpyPath = extract("scrcpy", scrcpyBinary)
	a.serverPath = extract("scrcpy-server", scrcpyServerBinary)

	if len(aaptBinary) > 0 {
		a.aaptPath = extract("aapt", aaptBinary)
		a.Log("AAPT setup at: %s", a.aaptPath)
	}

	// Setup FFmpeg and FFprobe
	if len(ffmpegBinary) > 0 {
		a.ffmpegPath = extract("ffmpeg", ffmpegBinary)
		a.Log("FFmpeg setup at: %s", a.ffmpegPath)
	}
	if len(ffprobeBinary) > 0 {
		a.ffprobePath = extract("ffprobe", ffprobeBinary)
		a.Log("FFprobe setup at: %s", a.ffprobePath)
	}

	// Setup ADBKeyboard APK (cross-platform, runs on Android device)
	if len(adbKeyboardAPK) > 0 {
		apkPath := filepath.Join(appBinDir, "ADBKeyboard.apk")
		info, err := os.Stat(apkPath)
		if force || err != nil || info.Size() != int64(len(adbKeyboardAPK)) {
			if writeErr := os.WriteFile(apkPath, adbKeyboardAPK, 0644); writeErr != nil {
				extractOK = false
				LogDebug("app").Str("name", "ADBKeyboard.apk").Err(writeErr).Msg("Error extracting ADBKeyboard APK")
			}
		}
		a.adbKeyboardPath = apkPath
		a.Log("ADBKeyboard APK setup at: %s", a.adbKeyboardPath)
	}

	// Setup protoc binary
	if len(protocBinary) > 0 {
		a.protocPath = extract("protoc", protocBinary)
		a.Log("Protoc setup at: %s", a.protocPath)
	}

	// Setup protoc well-known type includes (embedded filesystem → disk)
	protocIncludeDir := filepath.Join(appBinDir, "protoc-include")
	if !a.extractEmbedDir(protocIncludeFS, "bin/protoc-include", protocIncludeDir, force) {
		extractOK = false
	}
	a.protocIncludeDir = protocIncludeDir
	a.Log("Protoc includes at: %s", a.protocIncludeDir)

	// Write the stamp only after all extractions succeed, so both an
	// interrupted run and a failed write are retried with force=true next launch
	if extractOK {
		_ = os.WriteFile(stampPath, []byte(a.version), 0644)
	} else {
		a.Log("Some binaries failed to extract; version stamp not updated, will retry on next launch")
	}

	a.Log("Binaries setup at: %s", appBinDir)
	a.Log("Final ADB path: %s", a.adbPath)
}

// extractEmbedDir extracts an embedded filesystem directory to disk.
// srcPrefix is the embedded path prefix (e.g. "bin/protoc-include"),
// dstDir is the target directory on disk.
// It reports whether every file was written successfully.
func (a *App) extractEmbedDir(fsys embed.FS, srcPrefix string, dstDir string, force bool) bool {
	ok := true
	_ = os.MkdirAll(dstDir, 0755)
	entries, err := fs.ReadDir(fsys, srcPrefix)
	if err != nil {
		LogDebug("app").Str("prefix", srcPrefix).Err(err).Msg("Failed to read embedded dir")
		return false
	}
	for _, entry := range entries {
		srcPath := srcPrefix + "/" + entry.Name()
		dstPath := filepath.Join(dstDir, entry.Name())
		if entry.IsDir() {
			if !a.extractEmbedDir(fsys, srcPath, dstPath, force) {
				ok = false
			}
		} else {
			data, err := fsys.ReadFile(srcPath)
			if err != nil {
				ok = false
				continue
			}
			// Only write if content changed (same smart-extract logic as binaries)
			info, statErr := os.Stat(dstPath)
			if force || statErr != nil || info.Size() != int64(len(data)) {
				if writeErr := os.WriteFile(dstPath, data, 0644); writeErr != nil {
					ok = false
				}
			}
		}
	}
	return ok
}

// Command helper functions

// newAdbCommand creates an exec.Cmd with a clean environment to avoid proxy issues
func (a *App) newAdbCommand(ctx context.Context, args ...string) *exec.Cmd {
	var cmd *exec.Cmd
	if ctx != nil {
		cmd = exec.CommandContext(ctx, a.adbPath, args...)
	} else {
		cmd = exec.Command(a.adbPath, args...)
	}

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
	return a.newScrcpyCommandContext(context.Background(), args...)
}

func (a *App) newScrcpyCommandContext(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, a.scrcpyPath, args...)

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

	newEnv = append(newEnv,
		"SCRCPY_SERVER_PATH="+a.serverPath,
		"ADB="+a.adbPath,
	)

	cmd.Env = newEnv
	return cmd
}

// ========================================
// Event System
// ========================================

// initEventSystem initializes the new event storage and pipeline
func (a *App) initEventSystem() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	a.dataDir = filepath.Join(configDir, "Gaze", "data")

	// Create event store
	store, err := NewEventStore(a.dataDir)
	if err != nil {
		a.Log("Failed to initialize event store: %v", err)
		return
	}
	a.eventStore = store

	// Create event pipeline
	a.eventPipeline = NewEventPipeline(context.Background(), a.ctx, store, a.mcpMode)
	a.eventPipeline.Start()

	// Create assertion engine
	a.assertionEngine = NewAssertionEngine(a, store, a.eventPipeline)

	// Create plugin system
	pluginStore := NewPluginStore(store.db)
	if err := pluginStore.InitSchema(); err != nil {
		a.Log("Failed to initialize plugin store: %v", err)
	} else {
		a.pluginStore = pluginStore
		a.pluginManager = NewPluginManager(pluginStore, a.eventPipeline)

		// 连接到 EventPipeline
		a.eventPipeline.SetPluginManager(a.pluginManager)

		// 加载所有启用的插件
		if err := a.pluginManager.LoadAllPlugins(); err != nil {
			a.Log("Failed to load plugins: %v", err)
		} else {
			a.Log("Plugin system initialized, loaded %d plugins", len(a.pluginManager.ListPlugins()))
		}
	}

	a.Log("Event system initialized at: %s", a.dataDir)
}

// shutdownEventSystem shuts down the event system
func (a *App) shutdownEventSystem() {
	a.eventSystemMu.Lock()
	defer a.eventSystemMu.Unlock()

	if a.eventPipeline != nil {
		a.eventPipeline.Stop()
		a.eventPipeline = nil
	}
	if a.eventStore != nil {
		if err := a.eventStore.Close(); err != nil {
			a.Log("Error closing event store: %v", err)
		}
		a.eventStore = nil
	}
}

// ========================================
// Event System APIs (exposed to frontend)
// ========================================

// EmitEvent emits a new event through the pipeline
func (a *App) EmitEvent(deviceID string, source string, eventType string, level string, title string, data interface{}) {
	if a.eventPipeline == nil {
		return
	}
	a.eventPipeline.EmitRaw(
		deviceID,
		EventSource(source),
		eventType,
		EventLevel(level),
		title,
		data,
	)
}

// StartNewSession starts a new session for a device (legacy API, no config)
func (a *App) StartNewSession(deviceID, sessionType, name string) string {
	log.Printf("[StartNewSession] Called with deviceID=%s, type=%s, name=%s", deviceID, sessionType, name)
	if a.eventPipeline == nil {
		log.Printf("[StartNewSession] ERROR: eventPipeline is nil!")
		return ""
	}
	sessionID := a.eventPipeline.StartSession(deviceID, sessionType, name, nil)
	log.Printf("[StartNewSession] Created session: %s", sessionID)
	return sessionID
}

// StartSessionWithConfig starts a new session with configuration (logcat, recording, proxy)
func (a *App) StartSessionWithConfig(deviceID, name string, config SessionConfig) string {
	log.Printf("[StartSessionWithConfig] Called with deviceID=%s, name=%s, config=%+v", deviceID, name, config)
	if a.eventPipeline == nil {
		log.Printf("[StartSessionWithConfig] ERROR: eventPipeline is nil!")
		return ""
	}

	// 创建 session
	sessionID := a.eventPipeline.StartSession(deviceID, "manual", name, &config)
	log.Printf("[StartSessionWithConfig] Created session: %s", sessionID)

	// 根据配置启动相关功能
	if config.Logcat.Enabled {
		log.Printf("[StartSessionWithConfig] Starting logcat for package: %s", config.Logcat.PackageName)
		go func() {
			if err := a.StartLogcat(deviceID, config.Logcat.PackageName,
				config.Logcat.PreFilter, false,
				config.Logcat.ExcludeFilter, false); err != nil {
				log.Printf("[StartSessionWithConfig] Failed to start logcat: %v", err)
			}
		}()
	}

	if config.Recording.Enabled {
		log.Printf("[StartSessionWithConfig] Starting headless recording")
		go func() {
			// 生成录屏文件路径
			recordPath := a.generateRecordPath(deviceID, sessionID)
			scrcpyConfig := ScrcpyConfig{
				RecordPath: recordPath,
			}
			// 根据质量设置参数
			switch config.Recording.Quality {
			case "low":
				scrcpyConfig.MaxSize = 480
				scrcpyConfig.BitRate = 2000000
			case "high":
				scrcpyConfig.MaxSize = 1080
				scrcpyConfig.BitRate = 8000000
			default: // medium
				scrcpyConfig.MaxSize = 720
				scrcpyConfig.BitRate = 4000000
			}
			if err := a.StartRecording(deviceID, scrcpyConfig); err != nil {
				log.Printf("[StartSessionWithConfig] Failed to start recording: %v", err)
			} else {
				// 更新 session 的 videoPath
				a.updateSessionVideoPath(sessionID, recordPath)
			}
		}()
	}

	if config.Proxy.Enabled {
		log.Printf("[StartSessionWithConfig] Starting proxy on port: %d", config.Proxy.Port)
		// Set proxy device BEFORE starting proxy to ensure events are associated correctly
		a.SetProxyDevice(deviceID)
		proxyAlreadyRunning := a.GetProxyStatus()
		go func() {
			port := config.Proxy.Port
			if port == 0 {
				port = 8080
			}
			if proxyAlreadyRunning {
				// Proxy was already running — just reuse it, don't take ownership
				log.Printf("[StartSessionWithConfig] Proxy already running, reusing for session %s", sessionID)
			} else {
				// Proxy not running — start it and take ownership
				a.SetProxyMITM(config.Proxy.MitmEnabled)
				if _, err := a.StartProxy(port); err != nil {
					log.Printf("[StartSessionWithConfig] Failed to start proxy: %v", err)
					if !a.GetProxyStatus() {
						a.SetProxyDevice("")
						return
					}
				} else {
					// Successfully started — this session owns the proxy
					setProxyOwnerSession(sessionID)
					log.Printf("[StartSessionWithConfig] Proxy started by session %s", sessionID)
				}

				// Setup adb reverse + device proxy so device traffic routes through the proxy
				if err := a.SetupProxyForDevice(deviceID, port); err != nil {
					log.Printf("[StartSessionWithConfig] Failed to setup proxy for device: %v", err)
				} else {
					log.Printf("[StartSessionWithConfig] Proxy setup complete for device %s on port %d", deviceID, port)
				}
				// Notify frontend of proxy status change
				a.emitProxyStatus(true, port)
			}
		}()
	}

	if config.Monitor.Enabled {
		log.Printf("[StartSessionWithConfig] Starting device monitor")
		a.sessionMonitorsMu.Lock()
		// Stop existing monitor for this device if any
		if existing := a.sessionMonitors[deviceID]; existing != nil {
			existing.Stop()
		}
		monitor := NewDeviceMonitor(a, deviceID)
		a.sessionMonitors[deviceID] = monitor
		a.sessionMonitorsMu.Unlock()
		monitor.Start()
	}

	return sessionID
}

// generateRecordPath generates a unique recording path for a session
func (a *App) generateRecordPath(deviceID, sessionID string) string {
	recordDir := a.GetRecordingsDir()
	os.MkdirAll(recordDir, 0755)
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	return filepath.Join(recordDir, fmt.Sprintf("%s_%s.mp4", timestamp, sessionID[:8]))
}

// updateSessionVideoPath updates the video path for a session
func (a *App) updateSessionVideoPath(sessionID, videoPath string) {
	if a.eventPipeline == nil {
		return
	}
	a.eventPipeline.UpdateSessionVideoPath(sessionID, videoPath)
}

// EndActiveSession ends the current session for a device
func (a *App) EndActiveSession(sessionID, status string) {
	if a.eventPipeline == nil {
		return
	}

	// 获取 session 配置，自动停止相关功能
	session := a.eventPipeline.GetSession(sessionID)
	if session != nil {
		if session.Config.Logcat.Enabled {
			log.Printf("[EndActiveSession] Stopping logcat")
			a.StopLogcat()
			a.stopAllTouchRecordings()
			a.StopAllDeviceStateMonitors()
			a.StopAllPerfMonitors()
			a.StopAllNetworkMonitors()
		}
		if session.Config.Recording.Enabled {
			log.Printf("[EndActiveSession] Stopping recording")
			a.StopRecording(session.DeviceID)
		}
		if session.Config.Proxy.Enabled {
			owner := getProxyOwnerSession()
			if owner == sessionID {
				// This session started the proxy — stop it and clean up
				log.Printf("[EndActiveSession] Stopping proxy (owned by this session)")
				a.StopProxy()
			} else {
				// Proxy was already running before this session — leave it alone
				log.Printf("[EndActiveSession] Proxy not owned by this session (owner=%s), leaving running", owner)
			}
		}
		if session.Config.Monitor.Enabled {
			log.Printf("[EndActiveSession] Stopping device monitor")
			a.sessionMonitorsMu.Lock()
			if monitor := a.sessionMonitors[session.DeviceID]; monitor != nil {
				monitor.Stop()
				delete(a.sessionMonitors, session.DeviceID)
			}
			a.sessionMonitorsMu.Unlock()
		}
	}

	a.eventPipeline.EndSession(sessionID, status)
}

// GetDeviceActiveSession returns the active session for a device
func (a *App) GetDeviceActiveSession(deviceID string) *DeviceSession {
	if a.eventPipeline == nil {
		return nil
	}
	return a.eventPipeline.GetActiveSession(deviceID)
}

// GetInstalledPackages returns a list of installed packages on the device
func (a *App) GetInstalledPackages(deviceID string, thirdPartyOnly bool) ([]string, error) {
	args := []string{"-s", deviceID, "shell", "pm", "list", "packages"}
	if thirdPartyOnly {
		args = append(args, "-3") // Only third-party (user-installed) apps
	}

	cmd := a.newAdbCommand(nil, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %v", err)
	}

	var packages []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package:") {
			pkg := strings.TrimPrefix(line, "package:")
			packages = append(packages, pkg)
		}
	}

	return packages, nil
}

// ========================================
// Persistent Log API Methods (exposed to frontend)
// ========================================

// GetLogFilePath returns the current log file path
func (a *App) GetLogFilePath() string {
	return GetLogFilePath()
}

// GetLogDir returns the log directory path
func (a *App) GetLogDir() string {
	return GetLogDir()
}

// ListLogFiles returns all log files
func (a *App) ListLogFiles() ([]string, error) {
	return ListLogFiles()
}

// ReadRecentLogs reads the most recent log lines
func (a *App) ReadRecentLogs(lines int) ([]string, error) {
	return ReadRecentLogs(lines)
}

// OpenLogDir opens the log directory in the system file manager
func (a *App) OpenLogDir() error {
	logDir := GetLogDir()
	if logDir == "" {
		return fmt.Errorf("log directory not available")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", logDir)
	case "windows":
		cmd = exec.Command("explorer", logDir)
	default:
		cmd = exec.Command("xdg-open", logDir)
	}
	return cmd.Start()
}

// ========================================
// Plugin System APIs
// ========================================

// ListPlugins 列出所有插件
func (a *App) ListPlugins() ([]PluginMetadata, error) {
	if a.pluginStore == nil {
		return nil, fmt.Errorf("plugin system not initialized")
	}

	plugins, err := a.pluginStore.ListPlugins()
	if err != nil {
		return nil, err
	}

	result := make([]PluginMetadata, len(plugins))
	for i, p := range plugins {
		result[i] = p.Metadata
	}

	return result, nil
}

// GetPlugin 获取插件详情（包含源代码）
func (a *App) GetPlugin(id string) (*Plugin, error) {
	if a.pluginStore == nil {
		return nil, fmt.Errorf("plugin system not initialized")
	}

	return a.pluginStore.GetPlugin(id)
}

// SavePlugin 保存插件（创建或更新）
func (a *App) SavePlugin(req PluginSaveRequest) error {
	if a.pluginManager == nil {
		return fmt.Errorf("plugin system not initialized")
	}

	// 构造插件对象
	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:          req.ID,
			Name:        req.Name,
			Version:     req.Version,
			Author:      req.Author,
			Description: req.Description,
			Enabled:     true, // 默认启用
			Filters:     req.Filters,
			Config:      req.Config,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		SourceCode:   req.SourceCode,
		Language:     req.Language,
		CompiledCode: req.CompiledCode,
	}

	// 保存并加载
	if err := a.pluginManager.SavePlugin(plugin); err != nil {
		return fmt.Errorf("save plugin failed: %w", err)
	}

	a.Log("Plugin saved: %s (%s)", plugin.Metadata.Name, plugin.Metadata.ID)
	return nil
}

// DeletePlugin 删除插件
func (a *App) DeletePlugin(id string) error {
	if a.pluginManager == nil {
		return fmt.Errorf("plugin system not initialized")
	}

	if err := a.pluginManager.DeletePlugin(id); err != nil {
		return fmt.Errorf("delete plugin failed: %w", err)
	}

	a.Log("Plugin deleted: %s", id)
	return nil
}

// TogglePlugin 启用/禁用插件
func (a *App) TogglePlugin(id string, enabled bool) error {
	if a.pluginManager == nil {
		return fmt.Errorf("plugin system not initialized")
	}

	if err := a.pluginManager.TogglePlugin(id, enabled); err != nil {
		return fmt.Errorf("toggle plugin failed: %w", err)
	}

	action := "enabled"
	if !enabled {
		action = "disabled"
	}
	a.Log("Plugin %s: %s", action, id)
	return nil
}

// TestPlugin 测试插件（对单个事件运行，不写入数据库）
func (a *App) TestPlugin(script string, eventID string) ([]UnifiedEvent, error) {
	if a.pluginManager == nil || a.eventStore == nil {
		return nil, fmt.Errorf("plugin system not initialized")
	}

	// 获取测试事件
	event, err := a.eventStore.GetEvent(eventID)
	if err != nil {
		return nil, fmt.Errorf("get test event failed: %w", err)
	}

	// 创建临时插件（使用唯一 ID 避免并发冲突）
	tempID := "test-" + uuid.New().String()[:8]
	tempPlugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      tempID,
			Name:    "Test Plugin",
			Version: "1.0.0",
			Enabled: true,
			Filters: PluginFilters{}, // 匹配所有事件
			Config:  make(map[string]interface{}),
		},
		SourceCode:   script,
		Language:     "javascript",
		CompiledCode: script, // 假设已编译
		State:        make(map[string]interface{}),
	}

	// 加载插件（不保存到数据库）
	if err := a.pluginManager.LoadPlugin(tempPlugin); err != nil {
		return nil, fmt.Errorf("load test plugin failed: %w", err)
	}
	defer a.pluginManager.UnloadPlugin(tempID)

	// 执行插件
	derivedEvents := a.pluginManager.ProcessEvent(*event, event.SessionID)

	return derivedEvents, nil
}

// TestPluginDetailed 测试插件（返回详细结果）
func (a *App) TestPluginDetailed(script string, eventID string) (PluginTestResult, error) {
	result := PluginTestResult{
		Success:       false,
		DerivedEvents: []UnifiedEvent{},
		Logs:          []string{},
	}

	if a.pluginManager == nil || a.eventStore == nil {
		result.Error = "plugin system not initialized"
		return result, fmt.Errorf(result.Error)
	}

	// 获取测试事件
	event, err := a.eventStore.GetEvent(eventID)
	if err != nil {
		result.Error = fmt.Sprintf("get test event failed: %v", err)
		return result, err
	}

	result.EventSnapshot = event

	// 创建临时插件（使用唯一 ID 避免并发冲突）
	tempID := "test-" + uuid.New().String()[:8]
	tempPlugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      tempID,
			Name:    "Test Plugin",
			Version: "1.0.0",
			Enabled: true,
			Filters: PluginFilters{}, // 匹配所有事件
			Config:  make(map[string]interface{}),
		},
		SourceCode:   script,
		Language:     "javascript",
		CompiledCode: script,
		State:        make(map[string]interface{}),
	}

	// 加载插件
	if err := a.pluginManager.LoadPlugin(tempPlugin); err != nil {
		result.Error = fmt.Sprintf("load test plugin failed: %v", err)
		return result, err
	}
	defer a.pluginManager.UnloadPlugin(tempID)

	// 检查是否匹配过滤器
	result.MatchedFilters = tempPlugin.MatchesEvent(*event)

	// 执行插件（带日志捕获）
	pluginResult, logs, execTime, err := a.pluginManager.ExecutePluginWithLogging(tempPlugin, *event, event.SessionID)
	result.Logs = logs
	result.ExecutionTime = execTime

	if err != nil {
		result.Error = err.Error()
		result.Success = false
		return result, nil // 不返回 error，错误信息在 result 中
	}

	// 填充派生事件
	if pluginResult != nil && len(pluginResult.DerivedEvents) > 0 {
		result.DerivedEvents = pluginResult.DerivedEvents
	}

	result.Success = true
	return result, nil
}

// TestPluginWithEventData 使用自定义事件数据测试插件
func (a *App) TestPluginWithEventData(script string, eventDataJSON string) (PluginTestResult, error) {
	result := PluginTestResult{
		Success:       false,
		DerivedEvents: []UnifiedEvent{},
		Logs:          []string{},
	}

	if a.pluginManager == nil {
		result.Error = "plugin system not initialized"
		return result, fmt.Errorf(result.Error)
	}

	// 解析事件数据
	var event UnifiedEvent
	if err := json.Unmarshal([]byte(eventDataJSON), &event); err != nil {
		result.Error = fmt.Sprintf("parse event data failed: %v", err)
		return result, err
	}

	// 如果缺少必要字段，填充默认值
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	if event.DeviceID == "" {
		event.DeviceID = "test-device"
	}
	if event.SessionID == "" {
		event.SessionID = "test-session"
	}

	result.EventSnapshot = &event

	// 创建临时插件（使用唯一 ID 避免并发冲突）
	tempID := "test-" + uuid.New().String()[:8]
	tempPlugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      tempID,
			Name:    "Test Plugin",
			Version: "1.0.0",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		SourceCode:   script,
		Language:     "javascript",
		CompiledCode: script,
		State:        make(map[string]interface{}),
	}

	// 加载插件
	if err := a.pluginManager.LoadPlugin(tempPlugin); err != nil {
		result.Error = fmt.Sprintf("load test plugin failed: %v", err)
		return result, err
	}
	defer a.pluginManager.UnloadPlugin(tempID)

	// 检查是否匹配过滤器
	result.MatchedFilters = tempPlugin.MatchesEvent(event)

	// 执行插件（带日志捕获）
	pluginResult, logs, execTime, err := a.pluginManager.ExecutePluginWithLogging(tempPlugin, event, event.SessionID)
	result.Logs = logs
	result.ExecutionTime = execTime

	if err != nil {
		result.Error = err.Error()
		result.Success = false
		return result, nil
	}

	if pluginResult != nil && len(pluginResult.DerivedEvents) > 0 {
		result.DerivedEvents = pluginResult.DerivedEvents
	}

	result.Success = true
	return result, nil
}

// TestPluginBatch 批量测试插件（对多个事件运行）
func (a *App) TestPluginBatch(script string, eventIDs []string) ([]PluginTestResult, error) {
	if len(eventIDs) > 50 {
		return nil, fmt.Errorf("too many events for batch test (max: 50, got: %d)", len(eventIDs))
	}

	results := make([]PluginTestResult, 0, len(eventIDs))

	for _, eventID := range eventIDs {
		result, err := a.TestPluginDetailed(script, eventID)
		if err != nil && result.Error == "" {
			// 如果 TestPluginDetailed 返回了 error 但 result.Error 为空，填充错误信息
			result.Error = err.Error()
		}
		results = append(results, result)
	}

	return results, nil
}

// GetSampleEvents 获取会话中的示例事件供测试使用
func (a *App) GetSampleEvents(sessionID string, sources []string, types []string, limit int) ([]UnifiedEvent, error) {
	if a.eventStore == nil {
		return nil, fmt.Errorf("event store not initialized")
	}

	if limit <= 0 || limit > 100 {
		limit = 20 // 默认 20 条
	}

	// 转换 sources 类型
	eventSources := make([]EventSource, len(sources))
	for i, s := range sources {
		eventSources[i] = EventSource(s)
	}

	// 构造查询条件
	query := EventQuery{
		SessionID: sessionID,
		Sources:   eventSources,
		Types:     types,
		Limit:     limit,
		Offset:    0,
	}

	// 查询事件
	result, err := a.eventStore.QueryEvents(query)
	if err != nil {
		return nil, fmt.Errorf("query events failed: %w", err)
	}

	return result.Events, nil
}
