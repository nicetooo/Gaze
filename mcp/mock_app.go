package mcp

import (
	"encoding/json"
	"errors"
	"sync"
)

// MockCall records a method call for verification
type MockCall struct {
	Method string
	Args   []interface{}
}

// MockGazeApp is a mock implementation of GazeApp for testing
type MockGazeApp struct {
	mu    sync.Mutex
	Calls []MockCall

	// Device Management
	GetDevicesResult       []Device
	GetDevicesError        error
	GetDeviceInfoResult    DeviceInfo
	GetDeviceInfoError     error
	AdbConnectResult       string
	AdbConnectError        error
	AdbDisconnectResult    string
	AdbDisconnectError     error
	AdbPairResult          string
	AdbPairError           error
	SwitchToWirelessResult string
	SwitchToWirelessError  error
	GetDeviceIPResult      string
	GetDeviceIPError       error

	// App Management
	ListPackagesResult []AppPackage
	ListPackagesError  error
	GetAppInfoResult   AppPackage
	GetAppInfoError    error
	StartAppResult     string
	StartAppError      error
	ForceStopAppResult string
	ForceStopAppError  error
	InstallAPKResult   string
	InstallAPKError    error
	UninstallAppResult string
	UninstallAppError  error
	ClearAppDataResult string
	ClearAppDataError  error
	IsAppRunningResult bool
	IsAppRunningError  error

	// Screen Control
	TakeScreenshotResult string
	TakeScreenshotError  error
	StartRecordingError  error
	StopRecordingError   error
	IsRecordingResult    bool

	// UI Automation
	GetUIHierarchyResult         *UIHierarchyResult
	GetUIHierarchyError          error
	SearchUIElementsResult       []map[string]interface{}
	SearchUIElementsError        error
	PerformNodeActionError       error
	GetDeviceResolutionResult    string
	GetDeviceResolutionError     error
	InputTextError               error
	EnsureADBKeyboardReady       bool
	EnsureADBKeyboardInstalled   bool
	EnsureADBKeyboardError       error
	IsADBKeyboardInstalledResult bool
	WaitForUIElementResult       map[string]interface{}
	WaitForUIElementError        error

	// Session Management
	CreateSessionResult          string
	StartSessionWithConfigResult string
	EndSessionError              error
	GetActiveSessionResult       string
	ListStoredSessionsResult     []DeviceSession
	ListStoredSessionsError      error
	QuerySessionEventsResult     *EventQueryResult
	QuerySessionEventsError      error
	GetSessionStatsResult        map[string]interface{}
	GetSessionStatsError         error

	// Workflow
	LoadWorkflowsResult          []Workflow
	LoadWorkflowsError           error
	GetWorkflowResult            *Workflow
	GetWorkflowError             error
	SaveWorkflowError            error
	DeleteWorkflowError          error
	RunWorkflowError             error
	ExecuteSingleStepError       error
	IsWorkflowRunningResult      bool
	WorkflowExecutionResultValue *WorkflowExecutionResult
	// StopWorkflow has no return

	// Proxy
	StartProxyResult     string
	StartProxyError      error
	StopProxyResult      string
	StopProxyError       error
	GetProxyStatusResult bool

	// Mock Rules
	AddMockRuleResult   string
	UpdateMockRuleError error
	GetMockRulesResult  []MCPMockRule
	ToggleMockRuleError error
	ResendRequestResult map[string]interface{}
	ResendRequestError  error

	// Video
	GetVideoFrameResult       string
	GetVideoFrameError        error
	GetVideoMetadataResult    *VideoMetadata
	GetVideoMetadataError     error
	GetSessionVideoInfoResult map[string]interface{}
	GetSessionVideoInfoError  error

	// Session Export/Import
	ExportSessionToPathResult   string
	ExportSessionToPathError    error
	ImportSessionFromPathResult string
	ImportSessionFromPathError  error

	// Breakpoint Rules
	AddBreakpointRuleResult     string
	UpdateBreakpointRuleError   error
	GetBreakpointRulesResult    []MCPBreakpointRule
	ToggleBreakpointRuleError   error
	ResolveBreakpointError      error
	GetPendingBreakpointsResult []MCPPendingBreakpointInfo

	// Protobuf Management
	AddProtoFileResult         string
	AddProtoFileError          error
	UpdateProtoFileError       error
	RemoveProtoFileError       error
	GetProtoFilesResult        []MCPProtoFile
	AddProtoMappingResult      string
	AddProtoMappingError       error
	UpdateProtoMappingError    error
	RemoveProtoMappingError    error
	GetProtoMappingsResult     []MCPProtoMapping
	GetProtoMessageTypesResult []string
	LoadProtoFromURLResult     []string
	LoadProtoFromURLError      error

	// Touch Recording
	StartTouchRecordingError error
	StopTouchRecordingResult *TouchScript
	StopTouchRecordingError  error
	IsRecordingTouchResult   bool
	PlayTouchScriptError     error
	LoadTouchScriptsResult   []TouchScript
	LoadTouchScriptsError    error
	SaveTouchScriptError     error
	DeleteTouchScriptError   error

	// ADB / CLI Tools
	RunAdbCommandResult string
	RunAdbCommandError  error

	// Plugin Management
	ListPluginsResult []interface{}
	ListPluginsError  error
	GetPluginResult   interface{}
	GetPluginError    error
	SavePluginError   error
	DeletePluginError error
	TogglePluginError error
	TestPluginResult  interface{}
	TestPluginError   error

	// Utility
	AppVersion string
}

// NewMockGazeApp creates a new MockGazeApp with sensible defaults
func NewMockGazeApp() *MockGazeApp {
	return &MockGazeApp{
		Calls:      make([]MockCall, 0),
		AppVersion: "1.0.0-test",
		// Default empty results
		GetDevicesResult:         []Device{},
		ListPackagesResult:       []AppPackage{},
		LoadWorkflowsResult:      []Workflow{},
		ListStoredSessionsResult: []DeviceSession{},
		SearchUIElementsResult:   []map[string]interface{}{},
	}
}

// recordCall records a method call
func (m *MockGazeApp) recordCall(method string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, MockCall{Method: method, Args: args})
}

// GetCalls returns all recorded calls
func (m *MockGazeApp) GetCalls() []MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MockCall{}, m.Calls...)
}

// ResetCalls clears all recorded calls
func (m *MockGazeApp) ResetCalls() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = make([]MockCall, 0)
}

// GetLastCall returns the last recorded call
func (m *MockGazeApp) GetLastCall() *MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Calls) == 0 {
		return nil
	}
	return &m.Calls[len(m.Calls)-1]
}

// WasMethodCalled checks if a method was called
func (m *MockGazeApp) WasMethodCalled(method string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, call := range m.Calls {
		if call.Method == method {
			return true
		}
	}
	return false
}

// GetLastCallByMethod returns the last call to a specific method
func (m *MockGazeApp) GetLastCallByMethod(method string) *MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.Calls) - 1; i >= 0; i-- {
		if m.Calls[i].Method == method {
			return &m.Calls[i]
		}
	}
	return nil
}

// === Device Management ===

func (m *MockGazeApp) GetDevices(forceLog bool) ([]Device, error) {
	m.recordCall("GetDevices", forceLog)
	return m.GetDevicesResult, m.GetDevicesError
}

func (m *MockGazeApp) GetDeviceInfo(deviceId string) (DeviceInfo, error) {
	m.recordCall("GetDeviceInfo", deviceId)
	return m.GetDeviceInfoResult, m.GetDeviceInfoError
}

func (m *MockGazeApp) AdbConnect(address string) (string, error) {
	m.recordCall("AdbConnect", address)
	return m.AdbConnectResult, m.AdbConnectError
}

func (m *MockGazeApp) AdbDisconnect(address string) (string, error) {
	m.recordCall("AdbDisconnect", address)
	return m.AdbDisconnectResult, m.AdbDisconnectError
}

func (m *MockGazeApp) AdbPair(address string, code string) (string, error) {
	m.recordCall("AdbPair", address, code)
	return m.AdbPairResult, m.AdbPairError
}

func (m *MockGazeApp) SwitchToWireless(deviceId string) (string, error) {
	m.recordCall("SwitchToWireless", deviceId)
	return m.SwitchToWirelessResult, m.SwitchToWirelessError
}

func (m *MockGazeApp) GetDeviceIP(deviceId string) (string, error) {
	m.recordCall("GetDeviceIP", deviceId)
	return m.GetDeviceIPResult, m.GetDeviceIPError
}

// === App Management ===

func (m *MockGazeApp) ListPackages(deviceId string, packageType string) ([]AppPackage, error) {
	m.recordCall("ListPackages", deviceId, packageType)
	return m.ListPackagesResult, m.ListPackagesError
}

func (m *MockGazeApp) GetAppInfo(deviceId, packageName string, force bool) (AppPackage, error) {
	m.recordCall("GetAppInfo", deviceId, packageName, force)
	return m.GetAppInfoResult, m.GetAppInfoError
}

func (m *MockGazeApp) StartApp(deviceId, packageName string) (string, error) {
	m.recordCall("StartApp", deviceId, packageName)
	return m.StartAppResult, m.StartAppError
}

func (m *MockGazeApp) ForceStopApp(deviceId, packageName string) (string, error) {
	m.recordCall("ForceStopApp", deviceId, packageName)
	return m.ForceStopAppResult, m.ForceStopAppError
}

func (m *MockGazeApp) InstallAPK(deviceId string, path string) (string, error) {
	m.recordCall("InstallAPK", deviceId, path)
	return m.InstallAPKResult, m.InstallAPKError
}

func (m *MockGazeApp) UninstallApp(deviceId, packageName string) (string, error) {
	m.recordCall("UninstallApp", deviceId, packageName)
	return m.UninstallAppResult, m.UninstallAppError
}

func (m *MockGazeApp) ClearAppData(deviceId, packageName string) (string, error) {
	m.recordCall("ClearAppData", deviceId, packageName)
	return m.ClearAppDataResult, m.ClearAppDataError
}

func (m *MockGazeApp) IsAppRunning(deviceId, packageName string) (bool, error) {
	m.recordCall("IsAppRunning", deviceId, packageName)
	return m.IsAppRunningResult, m.IsAppRunningError
}

// === Screen Control ===

func (m *MockGazeApp) TakeScreenshot(deviceId, savePath string) (string, error) {
	m.recordCall("TakeScreenshot", deviceId, savePath)
	return m.TakeScreenshotResult, m.TakeScreenshotError
}

func (m *MockGazeApp) StartRecording(deviceId string, config ScrcpyConfig) error {
	m.recordCall("StartRecording", deviceId, config)
	return m.StartRecordingError
}

func (m *MockGazeApp) StopRecording(deviceId string) error {
	m.recordCall("StopRecording", deviceId)
	return m.StopRecordingError
}

func (m *MockGazeApp) IsRecording(deviceId string) bool {
	m.recordCall("IsRecording", deviceId)
	return m.IsRecordingResult
}

// === UI Automation ===

func (m *MockGazeApp) GetUIHierarchy(deviceId string) (*UIHierarchyResult, error) {
	m.recordCall("GetUIHierarchy", deviceId)
	return m.GetUIHierarchyResult, m.GetUIHierarchyError
}

func (m *MockGazeApp) SearchUIElements(deviceId string, query string) ([]map[string]interface{}, error) {
	m.recordCall("SearchUIElements", deviceId, query)
	return m.SearchUIElementsResult, m.SearchUIElementsError
}

func (m *MockGazeApp) PerformNodeAction(deviceId string, bounds string, actionType string) error {
	m.recordCall("PerformNodeAction", deviceId, bounds, actionType)
	return m.PerformNodeActionError
}

func (m *MockGazeApp) WaitForUIElement(deviceId, selectorType, selectorValue, condition string, timeoutMs, intervalMs int) (map[string]interface{}, error) {
	m.recordCall("WaitForUIElement", deviceId, selectorType, selectorValue, condition, timeoutMs, intervalMs)
	return m.WaitForUIElementResult, m.WaitForUIElementError
}

func (m *MockGazeApp) GetDeviceResolution(deviceId string) (string, error) {
	m.recordCall("GetDeviceResolution", deviceId)
	return m.GetDeviceResolutionResult, m.GetDeviceResolutionError
}

func (m *MockGazeApp) InputText(deviceId string, text string) error {
	m.recordCall("InputText", deviceId, text)
	return m.InputTextError
}

func (m *MockGazeApp) EnsureADBKeyboard(deviceId string) (bool, bool, error) {
	m.recordCall("EnsureADBKeyboard", deviceId)
	return m.EnsureADBKeyboardReady, m.EnsureADBKeyboardInstalled, m.EnsureADBKeyboardError
}

func (m *MockGazeApp) IsADBKeyboardInstalled(deviceId string) bool {
	m.recordCall("IsADBKeyboardInstalled", deviceId)
	return m.IsADBKeyboardInstalledResult
}

// === Session Management ===

func (m *MockGazeApp) CreateSession(deviceId, sessionType, name string) string {
	m.recordCall("CreateSession", deviceId, sessionType, name)
	return m.CreateSessionResult
}

func (m *MockGazeApp) StartSessionWithConfig(deviceId, name string, config MCPSessionConfig) string {
	m.recordCall("StartSessionWithConfig", deviceId, name, config)
	if m.StartSessionWithConfigResult != "" {
		return m.StartSessionWithConfigResult
	}
	return m.CreateSessionResult
}

func (m *MockGazeApp) EndSession(sessionId string, status string) error {
	m.recordCall("EndSession", sessionId, status)
	return m.EndSessionError
}

func (m *MockGazeApp) GetActiveSession(deviceId string) string {
	m.recordCall("GetActiveSession", deviceId)
	return m.GetActiveSessionResult
}

func (m *MockGazeApp) ListStoredSessions(deviceID string, limit int) ([]DeviceSession, error) {
	m.recordCall("ListStoredSessions", deviceID, limit)
	return m.ListStoredSessionsResult, m.ListStoredSessionsError
}

func (m *MockGazeApp) QuerySessionEvents(query EventQuery) (*EventQueryResult, error) {
	m.recordCall("QuerySessionEvents", query)
	return m.QuerySessionEventsResult, m.QuerySessionEventsError
}

func (m *MockGazeApp) GetSessionStats(sessionID string) (map[string]interface{}, error) {
	m.recordCall("GetSessionStats", sessionID)
	return m.GetSessionStatsResult, m.GetSessionStatsError
}

// === Workflow ===

func (m *MockGazeApp) LoadWorkflows() ([]Workflow, error) {
	m.recordCall("LoadWorkflows")
	return m.LoadWorkflowsResult, m.LoadWorkflowsError
}

func (m *MockGazeApp) GetWorkflow(workflowID string) (*Workflow, error) {
	m.recordCall("GetWorkflow", workflowID)
	if m.GetWorkflowError != nil {
		return nil, m.GetWorkflowError
	}
	if m.GetWorkflowResult != nil {
		return m.GetWorkflowResult, nil
	}
	// Fallback: search in LoadWorkflowsResult
	for _, wf := range m.LoadWorkflowsResult {
		if wf.ID == workflowID {
			return &wf, nil
		}
	}
	return nil, errors.New("workflow not found")
}

func (m *MockGazeApp) SaveWorkflow(workflow Workflow) error {
	m.recordCall("SaveWorkflow", workflow)
	return m.SaveWorkflowError
}

func (m *MockGazeApp) DeleteWorkflow(id string) error {
	m.recordCall("DeleteWorkflow", id)
	return m.DeleteWorkflowError
}

func (m *MockGazeApp) RunWorkflow(device Device, workflow Workflow) error {
	m.recordCall("RunWorkflow", device, workflow)
	return m.RunWorkflowError
}

func (m *MockGazeApp) StopWorkflow(device Device) {
	m.recordCall("StopWorkflow", device)
}

func (m *MockGazeApp) PauseTask(deviceId string) {
	m.recordCall("PauseTask", deviceId)
}

func (m *MockGazeApp) ResumeTask(deviceId string) {
	m.recordCall("ResumeTask", deviceId)
}

func (m *MockGazeApp) ExecuteSingleWorkflowStep(deviceId string, step WorkflowStep) error {
	m.recordCall("ExecuteSingleWorkflowStep", deviceId, step)
	return m.ExecuteSingleStepError
}

func (m *MockGazeApp) IsWorkflowRunning(deviceId string) bool {
	m.recordCall("IsWorkflowRunning", deviceId)
	return m.IsWorkflowRunningResult
}

func (m *MockGazeApp) GetWorkflowExecutionResult(deviceId string) *WorkflowExecutionResult {
	m.recordCall("GetWorkflowExecutionResult", deviceId)
	return m.WorkflowExecutionResultValue
}

func (m *MockGazeApp) StepNextWorkflow(deviceId string) (*WorkflowExecutionResult, error) {
	m.recordCall("StepNextWorkflow", deviceId)
	return m.WorkflowExecutionResultValue, nil
}

// === Proxy ===

func (m *MockGazeApp) StartProxy(port int) (string, error) {
	m.recordCall("StartProxy", port)
	return m.StartProxyResult, m.StartProxyError
}

func (m *MockGazeApp) StopProxy() (string, error) {
	m.recordCall("StopProxy")
	return m.StopProxyResult, m.StopProxyError
}

func (m *MockGazeApp) GetProxyStatus() bool {
	m.recordCall("GetProxyStatus")
	return m.GetProxyStatusResult
}

// === Mock Rules ===

func (m *MockGazeApp) AddMockRule(urlPattern, method string, statusCode int, headers map[string]string, body, bodyFile string, delay int, description string, conditions []MCPMockCondition) string {
	m.recordCall("AddMockRule", urlPattern, method, statusCode, headers, body, bodyFile, delay, description, conditions)
	return m.AddMockRuleResult
}

func (m *MockGazeApp) UpdateMockRule(id, urlPattern, method string, statusCode int, headers map[string]string, body, bodyFile string, delay int, enabled bool, description string, conditions []MCPMockCondition) error {
	m.recordCall("UpdateMockRule", id, urlPattern, method, statusCode, headers, body, bodyFile, delay, enabled, description, conditions)
	return m.UpdateMockRuleError
}

func (m *MockGazeApp) RemoveMockRule(ruleID string) {
	m.recordCall("RemoveMockRule", ruleID)
}

func (m *MockGazeApp) GetMockRules() []MCPMockRule {
	m.recordCall("GetMockRules")
	return m.GetMockRulesResult
}

func (m *MockGazeApp) ToggleMockRule(ruleID string, enabled bool) error {
	m.recordCall("ToggleMockRule", ruleID, enabled)
	return m.ToggleMockRuleError
}

func (m *MockGazeApp) ExportMockRules() (string, error) {
	m.recordCall("ExportMockRules")
	return "[]", nil
}

func (m *MockGazeApp) ImportMockRules(jsonStr string) (int, error) {
	m.recordCall("ImportMockRules", jsonStr)
	return 0, nil
}

func (m *MockGazeApp) ResendRequest(method, url string, headers map[string]string, body string) (map[string]interface{}, error) {
	m.recordCall("ResendRequest", method, url, headers, body)
	return m.ResendRequestResult, m.ResendRequestError
}

// === Map Remote Rules ===

func (m *MockGazeApp) AddMapRemoteRule(sourcePattern, targetURL, method, description string) string {
	m.recordCall("AddMapRemoteRule", sourcePattern, targetURL, method, description)
	return "map-test-id"
}

func (m *MockGazeApp) UpdateMapRemoteRule(id, sourcePattern, targetURL, method string, enabled bool, description string) error {
	m.recordCall("UpdateMapRemoteRule", id, sourcePattern, targetURL, method, enabled, description)
	return nil
}

func (m *MockGazeApp) RemoveMapRemoteRule(ruleID string) {
	m.recordCall("RemoveMapRemoteRule", ruleID)
}

func (m *MockGazeApp) GetMapRemoteRules() []MCPMapRemoteRule {
	m.recordCall("GetMapRemoteRules")
	return []MCPMapRemoteRule{}
}

func (m *MockGazeApp) ToggleMapRemoteRule(ruleID string, enabled bool) error {
	m.recordCall("ToggleMapRemoteRule", ruleID, enabled)
	return nil
}

// === Rewrite Rules ===

func (m *MockGazeApp) AddRewriteRule(urlPattern, method, phase, target, headerName, match, replace, description string) string {
	m.recordCall("AddRewriteRule", urlPattern, method, phase, target, headerName, match, replace, description)
	return "rw-test-id"
}

func (m *MockGazeApp) UpdateRewriteRule(id, urlPattern, method, phase, target, headerName, match, replace string, enabled bool, description string) error {
	m.recordCall("UpdateRewriteRule", id, urlPattern, method, phase, target, headerName, match, replace, enabled, description)
	return nil
}

func (m *MockGazeApp) RemoveRewriteRule(ruleID string) {
	m.recordCall("RemoveRewriteRule", ruleID)
}

func (m *MockGazeApp) GetRewriteRules() []MCPRewriteRule {
	m.recordCall("GetRewriteRules")
	return []MCPRewriteRule{}
}

func (m *MockGazeApp) ToggleRewriteRule(ruleID string, enabled bool) error {
	m.recordCall("ToggleRewriteRule", ruleID, enabled)
	return nil
}

// === Breakpoint Rules ===

func (m *MockGazeApp) AddBreakpointRule(urlPattern, method, phase, description string) string {
	m.recordCall("AddBreakpointRule", urlPattern, method, phase, description)
	return m.AddBreakpointRuleResult
}

func (m *MockGazeApp) UpdateBreakpointRule(id, urlPattern, method, phase string, enabled bool, description string) error {
	m.recordCall("UpdateBreakpointRule", id, urlPattern, method, phase, enabled, description)
	return m.UpdateBreakpointRuleError
}

func (m *MockGazeApp) RemoveBreakpointRule(ruleID string) {
	m.recordCall("RemoveBreakpointRule", ruleID)
}

func (m *MockGazeApp) GetBreakpointRules() []MCPBreakpointRule {
	m.recordCall("GetBreakpointRules")
	if m.GetBreakpointRulesResult != nil {
		return m.GetBreakpointRulesResult
	}
	return []MCPBreakpointRule{}
}

func (m *MockGazeApp) ToggleBreakpointRule(ruleID string, enabled bool) error {
	m.recordCall("ToggleBreakpointRule", ruleID, enabled)
	return m.ToggleBreakpointRuleError
}

func (m *MockGazeApp) ResolveBreakpoint(breakpointID string, action string, modifications map[string]interface{}) error {
	m.recordCall("ResolveBreakpoint", breakpointID, action, modifications)
	return m.ResolveBreakpointError
}

func (m *MockGazeApp) GetPendingBreakpoints() []MCPPendingBreakpointInfo {
	m.recordCall("GetPendingBreakpoints")
	if m.GetPendingBreakpointsResult != nil {
		return m.GetPendingBreakpointsResult
	}
	return []MCPPendingBreakpointInfo{}
}

func (m *MockGazeApp) ForwardAllBreakpoints() {
	m.recordCall("ForwardAllBreakpoints")
}

// === Proxy Configuration ===

func (m *MockGazeApp) SetProxyDevice(deviceId string) {
	m.recordCall("SetProxyDevice", deviceId)
}

func (m *MockGazeApp) GetProxyDevice() string {
	m.recordCall("GetProxyDevice")
	return ""
}

func (m *MockGazeApp) SetupProxyForDevice(deviceId string, port int) error {
	m.recordCall("SetupProxyForDevice", deviceId, port)
	return nil
}

func (m *MockGazeApp) CleanupProxyForDevice(deviceId string, port int) error {
	m.recordCall("CleanupProxyForDevice", deviceId, port)
	return nil
}

func (m *MockGazeApp) SetProxyMITM(enabled bool) {
	m.recordCall("SetProxyMITM", enabled)
}

func (m *MockGazeApp) SetProxyWSEnabled(enabled bool) {
	m.recordCall("SetProxyWSEnabled", enabled)
}

func (m *MockGazeApp) SetProxyLimit(uploadSpeed, downloadSpeed int) {
	m.recordCall("SetProxyLimit", uploadSpeed, downloadSpeed)
}

func (m *MockGazeApp) SetProxyLatency(latencyMs int) {
	m.recordCall("SetProxyLatency", latencyMs)
}

func (m *MockGazeApp) SetMITMBypassPatterns(patterns []string) {
	m.recordCall("SetMITMBypassPatterns", patterns)
}

func (m *MockGazeApp) GetMITMBypassPatterns() []string {
	m.recordCall("GetMITMBypassPatterns")
	return nil
}

func (m *MockGazeApp) GetProxySettings() map[string]interface{} {
	m.recordCall("GetProxySettings")
	return map[string]interface{}{"wsEnabled": true, "mitmEnabled": true, "bypassPatterns": []string{}}
}

func (m *MockGazeApp) InstallProxyCert(deviceId string) (string, error) {
	m.recordCall("InstallProxyCert", deviceId)
	return "Certificate pushed", nil
}

func (m *MockGazeApp) CheckCertTrust(deviceId string) string {
	m.recordCall("CheckCertTrust", deviceId)
	return "unknown"
}

// === Video ===

func (m *MockGazeApp) GetVideoFrame(videoPath string, timeMs int64, width int) (string, error) {
	m.recordCall("GetVideoFrame", videoPath, timeMs, width)
	return m.GetVideoFrameResult, m.GetVideoFrameError
}

func (m *MockGazeApp) GetVideoMetadata(videoPath string) (*VideoMetadata, error) {
	m.recordCall("GetVideoMetadata", videoPath)
	return m.GetVideoMetadataResult, m.GetVideoMetadataError
}

func (m *MockGazeApp) GetSessionVideoInfo(sessionID string) (map[string]interface{}, error) {
	m.recordCall("GetSessionVideoInfo", sessionID)
	return m.GetSessionVideoInfoResult, m.GetSessionVideoInfoError
}

// === ADB ===

func (m *MockGazeApp) RunAdbCommand(deviceId string, command string) (string, error) {
	m.recordCall("RunAdbCommand", deviceId, command)
	return m.RunAdbCommandResult, m.RunAdbCommandError
}

func (m *MockGazeApp) RunAdbCommandWithTimeout(deviceId string, command string, timeoutSec int) (string, error) {
	m.recordCall("RunAdbCommandWithTimeout", deviceId, command, timeoutSec)
	return m.RunAdbCommandResult, m.RunAdbCommandError
}

// === CLI Tools ===

func (m *MockGazeApp) RunAaptCommand(command string, timeoutSec int) (string, error) {
	m.recordCall("RunAaptCommand", command, timeoutSec)
	return "", nil
}

func (m *MockGazeApp) RunFfmpegCommand(command string, timeoutSec int) (string, error) {
	m.recordCall("RunFfmpegCommand", command, timeoutSec)
	return "", nil
}

func (m *MockGazeApp) RunFfprobeCommand(command string, timeoutSec int) (string, error) {
	m.recordCall("RunFfprobeCommand", command, timeoutSec)
	return "", nil
}

// === File Management ===

func (m *MockGazeApp) UploadFile(deviceId, localPath, remotePath string) error {
	m.recordCall("UploadFile", deviceId, localPath, remotePath)
	return nil
}

func (m *MockGazeApp) ListFiles(deviceId, pathStr string) ([]map[string]interface{}, error) {
	m.recordCall("ListFiles", deviceId, pathStr)
	return nil, nil
}

// === Session Export/Import ===

func (m *MockGazeApp) ExportSessionToPath(sessionID, outputPath string) (string, error) {
	m.recordCall("ExportSessionToPath", sessionID, outputPath)
	return m.ExportSessionToPathResult, m.ExportSessionToPathError
}

func (m *MockGazeApp) ImportSessionFromPath(inputPath string) (string, error) {
	m.recordCall("ImportSessionFromPath", inputPath)
	return m.ImportSessionFromPathResult, m.ImportSessionFromPathError
}

// === Performance Monitoring ===

func (m *MockGazeApp) StartPerfMonitor(deviceId string, config PerfMonitorConfig) string {
	m.recordCall("StartPerfMonitor", deviceId, config)
	return "started"
}

func (m *MockGazeApp) StopPerfMonitor(deviceId string) string {
	m.recordCall("StopPerfMonitor", deviceId)
	return "stopped"
}

func (m *MockGazeApp) IsPerfMonitorRunning(deviceId string) bool {
	m.recordCall("IsPerfMonitorRunning", deviceId)
	return false
}

func (m *MockGazeApp) GetPerfSnapshot(deviceId string, packageName string) (*PerfSampleData, error) {
	m.recordCall("GetPerfSnapshot", deviceId, packageName)
	return &PerfSampleData{
		CPUUsage:     25.5,
		MemTotalMB:   8192,
		MemUsedMB:    4096,
		MemUsage:     50.0,
		FPS:          60.0,
		BatteryLevel: 85,
	}, nil
}

func (m *MockGazeApp) GetProcessDetail(deviceId string, pid int) (*ProcessDetail, error) {
	m.recordCall("GetProcessDetail", deviceId, pid)
	return &ProcessDetail{
		PID:         pid,
		PackageName: "com.example.app",
		TotalPSSKB:  431653,
		TotalRSSKB:  585360,
		Memory: []ProcessMemoryCategory{
			{Name: "Java Heap", PssKB: 21028, RssKB: 33280},
			{Name: "Native Heap", PssKB: 43224, RssKB: 44452},
			{Name: "Graphics", PssKB: 225272, RssKB: 225272},
		},
		Objects: ProcessObjects{Views: 10, Activities: 1},
		Threads: 75,
	}, nil
}

// === Protobuf Management ===

func (m *MockGazeApp) AddProtoFile(name, content string) (string, error) {
	m.recordCall("AddProtoFile", name, content)
	return m.AddProtoFileResult, m.AddProtoFileError
}

func (m *MockGazeApp) UpdateProtoFile(id, name, content string) error {
	m.recordCall("UpdateProtoFile", id, name, content)
	return m.UpdateProtoFileError
}

func (m *MockGazeApp) RemoveProtoFile(id string) error {
	m.recordCall("RemoveProtoFile", id)
	return m.RemoveProtoFileError
}

func (m *MockGazeApp) GetProtoFiles() []MCPProtoFile {
	m.recordCall("GetProtoFiles")
	if m.GetProtoFilesResult != nil {
		return m.GetProtoFilesResult
	}
	return []MCPProtoFile{}
}

func (m *MockGazeApp) AddProtoMapping(urlPattern, messageType, direction, description string) (string, error) {
	m.recordCall("AddProtoMapping", urlPattern, messageType, direction, description)
	return m.AddProtoMappingResult, m.AddProtoMappingError
}

func (m *MockGazeApp) UpdateProtoMapping(id, urlPattern, messageType, direction, description string) error {
	m.recordCall("UpdateProtoMapping", id, urlPattern, messageType, direction, description)
	return m.UpdateProtoMappingError
}

func (m *MockGazeApp) RemoveProtoMapping(id string) error {
	m.recordCall("RemoveProtoMapping", id)
	return m.RemoveProtoMappingError
}

func (m *MockGazeApp) GetProtoMappings() []MCPProtoMapping {
	m.recordCall("GetProtoMappings")
	if m.GetProtoMappingsResult != nil {
		return m.GetProtoMappingsResult
	}
	return []MCPProtoMapping{}
}

func (m *MockGazeApp) GetProtoMessageTypes() []string {
	m.recordCall("GetProtoMessageTypes")
	if m.GetProtoMessageTypesResult != nil {
		return m.GetProtoMessageTypesResult
	}
	return []string{}
}

func (m *MockGazeApp) LoadProtoFromURL(rawURL string) ([]string, error) {
	m.recordCall("LoadProtoFromURL", rawURL)
	return m.LoadProtoFromURLResult, m.LoadProtoFromURLError
}

// === Touch Recording ===

func (m *MockGazeApp) StartTouchRecording(deviceId string, mode string) error {
	m.recordCall("StartTouchRecording", deviceId, mode)
	return m.StartTouchRecordingError
}

func (m *MockGazeApp) StopTouchRecording(deviceId string) (*TouchScript, error) {
	m.recordCall("StopTouchRecording", deviceId)
	return m.StopTouchRecordingResult, m.StopTouchRecordingError
}

func (m *MockGazeApp) IsRecordingTouch(deviceId string) bool {
	m.recordCall("IsRecordingTouch", deviceId)
	return m.IsRecordingTouchResult
}

func (m *MockGazeApp) PlayTouchScript(deviceId string, script TouchScript) error {
	m.recordCall("PlayTouchScript", deviceId, script)
	return m.PlayTouchScriptError
}

func (m *MockGazeApp) StopTouchPlayback(deviceId string) {
	m.recordCall("StopTouchPlayback", deviceId)
}

func (m *MockGazeApp) LoadTouchScripts() ([]TouchScript, error) {
	m.recordCall("LoadTouchScripts")
	return m.LoadTouchScriptsResult, m.LoadTouchScriptsError
}

func (m *MockGazeApp) SaveTouchScript(script TouchScript) error {
	m.recordCall("SaveTouchScript", script)
	return m.SaveTouchScriptError
}

func (m *MockGazeApp) DeleteTouchScript(name string) error {
	m.recordCall("DeleteTouchScript", name)
	return m.DeleteTouchScriptError
}

func (m *MockGazeApp) ExecuteSingleTouchEvent(deviceId string, event TouchEvent, resolution string) error {
	m.recordCall("ExecuteSingleTouchEvent", deviceId, event, resolution)
	return nil
}

// === Individual Assertions ===

func (m *MockGazeApp) ListStoredAssertions(sessionID, deviceID string, templatesOnly bool, limit int) ([]MCPStoredAssertion, error) {
	m.recordCall("ListStoredAssertions", sessionID, deviceID, templatesOnly, limit)
	return []MCPStoredAssertion{}, nil
}

func (m *MockGazeApp) CreateStoredAssertionJSON(assertionJSON string, saveAsTemplate bool) error {
	m.recordCall("CreateStoredAssertionJSON", assertionJSON, saveAsTemplate)
	return nil
}

func (m *MockGazeApp) GetStoredAssertion(assertionID string) (*MCPStoredAssertion, error) {
	m.recordCall("GetStoredAssertion", assertionID)
	return &MCPStoredAssertion{
		ID:        assertionID,
		Name:      "Mock Assertion",
		Type:      "exists",
		Criteria:  json.RawMessage(`{"types":["network_request"]}`),
		Expected:  json.RawMessage(`{"exists":true}`),
		CreatedAt: 1000000,
		UpdatedAt: 1000000,
	}, nil
}

func (m *MockGazeApp) UpdateStoredAssertionJSON(assertionID string, assertionJSON string) error {
	m.recordCall("UpdateStoredAssertionJSON", assertionID, assertionJSON)
	return nil
}

func (m *MockGazeApp) DeleteStoredAssertion(assertionID string) error {
	m.recordCall("DeleteStoredAssertion", assertionID)
	return nil
}

func (m *MockGazeApp) ExecuteStoredAssertionInSession(assertionID, sessionID, deviceID string) (*MCPAssertionResult, error) {
	m.recordCall("ExecuteStoredAssertionInSession", assertionID, sessionID, deviceID)
	return &MCPAssertionResult{
		ID:            "mock-result-id",
		AssertionID:   assertionID,
		AssertionName: "Mock Assertion",
		SessionID:     sessionID,
		Passed:        true,
		Message:       "Assertion passed",
		ExecutedAt:    1000000,
		Duration:      10,
	}, nil
}

func (m *MockGazeApp) QuickAssertNoErrors(sessionID, deviceID string) (*MCPAssertionResult, error) {
	m.recordCall("QuickAssertNoErrors", sessionID, deviceID)
	return &MCPAssertionResult{
		ID:            "mock-result-id",
		AssertionName: "Quick check: no errors",
		SessionID:     sessionID,
		Passed:        true,
		Message:       "No errors found",
		ExecutedAt:    1000000,
		Duration:      5,
	}, nil
}

func (m *MockGazeApp) QuickAssertNoCrashes(sessionID, deviceID string) (*MCPAssertionResult, error) {
	m.recordCall("QuickAssertNoCrashes", sessionID, deviceID)
	return &MCPAssertionResult{
		ID:            "mock-result-id",
		AssertionName: "Quick check: no crashes",
		SessionID:     sessionID,
		Passed:        true,
		Message:       "No crashes found",
		ExecutedAt:    1000000,
		Duration:      5,
	}, nil
}

// === Assertion Sets ===

func (m *MockGazeApp) CreateAssertionSet(name, description string, assertionIDs []string) (string, error) {
	m.recordCall("CreateAssertionSet", name, description, assertionIDs)
	return "mock-set-id", nil
}

func (m *MockGazeApp) UpdateAssertionSet(id, name, description string, assertionIDs []string) error {
	m.recordCall("UpdateAssertionSet", id, name, description, assertionIDs)
	return nil
}

func (m *MockGazeApp) DeleteAssertionSet(id string) error {
	m.recordCall("DeleteAssertionSet", id)
	return nil
}

func (m *MockGazeApp) GetAssertionSet(id string) (*MCPAssertionSet, error) {
	m.recordCall("GetAssertionSet", id)
	return &MCPAssertionSet{
		ID:          id,
		Name:        "Mock Set",
		Description: "A mock assertion set",
		Assertions:  []string{"a1", "a2"},
		CreatedAt:   1000000,
		UpdatedAt:   1000000,
	}, nil
}

func (m *MockGazeApp) ListAssertionSets() ([]MCPAssertionSet, error) {
	m.recordCall("ListAssertionSets")
	return []MCPAssertionSet{}, nil
}

func (m *MockGazeApp) ExecuteAssertionSet(setID, sessionID, deviceID string) (*MCPAssertionSetResult, error) {
	m.recordCall("ExecuteAssertionSet", setID, sessionID, deviceID)
	return &MCPAssertionSetResult{
		ID:          "mock-result-id",
		SetID:       setID,
		SetName:     "Mock Set",
		SessionID:   sessionID,
		DeviceID:    deviceID,
		ExecutionID: "mock-exec-id",
		Status:      "passed",
		Summary: MCPAssertionSetSummary{
			Total:    0,
			Passed:   0,
			Failed:   0,
			Error:    0,
			PassRate: 100,
		},
		Results: []MCPAssertionResult{},
	}, nil
}

func (m *MockGazeApp) GetAssertionSetResults(setID string, limit int) ([]MCPAssertionSetResult, error) {
	m.recordCall("GetAssertionSetResults", setID, limit)
	return []MCPAssertionSetResult{}, nil
}

func (m *MockGazeApp) GetAssertionSetResultByExecution(executionID string) (*MCPAssertionSetResult, error) {
	m.recordCall("GetAssertionSetResultByExecution", executionID)
	return &MCPAssertionSetResult{
		ID:          "mock-result-id",
		ExecutionID: executionID,
		Status:      "passed",
		Summary: MCPAssertionSetSummary{
			Total:    0,
			Passed:   0,
			Failed:   0,
			Error:    0,
			PassRate: 100,
		},
		Results: []MCPAssertionResult{},
	}, nil
}

// === Plugin Management ===

func (m *MockGazeApp) ListPlugins() ([]interface{}, error) {
	m.recordCall("ListPlugins")
	return m.ListPluginsResult, m.ListPluginsError
}

func (m *MockGazeApp) GetPlugin(id string) (interface{}, error) {
	m.recordCall("GetPlugin", id)
	return m.GetPluginResult, m.GetPluginError
}

func (m *MockGazeApp) SavePlugin(req interface{}) error {
	m.recordCall("SavePlugin", req)
	return m.SavePluginError
}

func (m *MockGazeApp) DeletePlugin(id string) error {
	m.recordCall("DeletePlugin", id)
	return m.DeletePluginError
}

func (m *MockGazeApp) TogglePlugin(id string, enabled bool) error {
	m.recordCall("TogglePlugin", id, enabled)
	return m.TogglePluginError
}

func (m *MockGazeApp) TestPlugin(script string, eventID string) (interface{}, error) {
	m.recordCall("TestPlugin", script, eventID)
	return m.TestPluginResult, m.TestPluginError
}

func (m *MockGazeApp) TestPluginDetailed(script string, eventID string) (interface{}, error) {
	m.recordCall("TestPluginDetailed", script, eventID)
	return m.TestPluginResult, m.TestPluginError
}

func (m *MockGazeApp) TestPluginWithEventData(script string, eventDataJSON string) (interface{}, error) {
	m.recordCall("TestPluginWithEventData", script, eventDataJSON)
	return m.TestPluginResult, m.TestPluginError
}

func (m *MockGazeApp) TestPluginBatch(script string, eventIDs []string) (interface{}, error) {
	m.recordCall("TestPluginBatch", script, eventIDs)
	return m.TestPluginResult, m.TestPluginError
}

func (m *MockGazeApp) GetSampleEvents(sessionID string, sources []string, types []string, limit int) ([]interface{}, error) {
	m.recordCall("GetSampleEvents", sessionID, sources, types, limit)
	// Return empty slice as default
	return []interface{}{}, nil
}

// === Utility ===

func (m *MockGazeApp) GetAppVersion() string {
	m.recordCall("GetAppVersion")
	return m.AppVersion
}

// === Test Helper Functions ===

// SetupWithDevices configures mock with sample devices
func (m *MockGazeApp) SetupWithDevices(devices ...Device) *MockGazeApp {
	m.GetDevicesResult = devices
	return m
}

// SetupWithError configures a specific method to return an error
func (m *MockGazeApp) SetupWithError(method string, err error) *MockGazeApp {
	switch method {
	case "GetDevices":
		m.GetDevicesError = err
	case "GetDeviceInfo":
		m.GetDeviceInfoError = err
	case "AdbConnect":
		m.AdbConnectError = err
	case "AdbDisconnect":
		m.AdbDisconnectError = err
	case "AdbPair":
		m.AdbPairError = err
	case "SwitchToWireless":
		m.SwitchToWirelessError = err
	case "GetDeviceIP":
		m.GetDeviceIPError = err
	case "ListPackages":
		m.ListPackagesError = err
	case "GetAppInfo":
		m.GetAppInfoError = err
	case "StartApp":
		m.StartAppError = err
	case "ForceStopApp":
		m.ForceStopAppError = err
	case "InstallAPK":
		m.InstallAPKError = err
	case "UninstallApp":
		m.UninstallAppError = err
	case "ClearAppData":
		m.ClearAppDataError = err
	case "IsAppRunning":
		m.IsAppRunningError = err
	case "TakeScreenshot":
		m.TakeScreenshotError = err
	case "StartRecording":
		m.StartRecordingError = err
	case "StopRecording":
		m.StopRecordingError = err
	case "GetUIHierarchy":
		m.GetUIHierarchyError = err
	case "SearchUIElements":
		m.SearchUIElementsError = err
	case "PerformNodeAction":
		m.PerformNodeActionError = err
	case "WaitForUIElement":
		m.WaitForUIElementError = err
	case "GetDeviceResolution":
		m.GetDeviceResolutionError = err
	case "RunAdbCommand", "RunAdbCommandWithTimeout":
		m.RunAdbCommandError = err
	case "EndSession":
		m.EndSessionError = err
	case "ListStoredSessions":
		m.ListStoredSessionsError = err
	case "QuerySessionEvents":
		m.QuerySessionEventsError = err
	case "GetSessionStats":
		m.GetSessionStatsError = err
	case "LoadWorkflows":
		m.LoadWorkflowsError = err
	case "RunWorkflow":
		m.RunWorkflowError = err
	case "StartProxy":
		m.StartProxyError = err
	case "StopProxy":
		m.StopProxyError = err
	case "StartTouchRecording":
		m.StartTouchRecordingError = err
	case "StopTouchRecording":
		m.StopTouchRecordingError = err
	case "PlayTouchScript":
		m.PlayTouchScriptError = err
	case "LoadTouchScripts":
		m.LoadTouchScriptsError = err
	case "SaveTouchScript":
		m.SaveTouchScriptError = err
	case "DeleteTouchScript":
		m.DeleteTouchScriptError = err
	}
	return m
}

// Common test errors
var (
	ErrDeviceNotFound      = errors.New("device not found")
	ErrDeviceOffline       = errors.New("device offline")
	ErrAppNotFound         = errors.New("app not found")
	ErrAppNotRunning       = errors.New("app not running")
	ErrSessionNotFound     = errors.New("session not found")
	ErrWorkflowNotFound    = errors.New("workflow not found")
	ErrProxyAlreadyRunning = errors.New("proxy already running")
	ErrProxyNotRunning     = errors.New("proxy not running")
	ErrPermissionDenied    = errors.New("permission denied")
	ErrTimeout             = errors.New("operation timed out")
)

// Sample test data factories

// SampleDevice returns a sample device for testing
func SampleDevice(id string) Device {
	return Device{
		ID:         id,
		Serial:     id,
		State:      "device",
		Model:      "Pixel 6",
		Brand:      "Google",
		Type:       "wired",
		IDs:        []string{id},
		LastActive: 1700000000000,
	}
}

// SampleDeviceInfo returns sample device info for testing
func SampleDeviceInfo() DeviceInfo {
	return DeviceInfo{
		Model:        "Pixel 6",
		Brand:        "Google",
		Manufacturer: "Google",
		AndroidVer:   "14",
		SDK:          "34",
		ABI:          "arm64-v8a",
		Serial:       "abc123",
		Resolution:   "1080x2400",
		Density:      "420",
		CPU:          "Tensor",
		Memory:       "8GB",
		Props:        map[string]string{"ro.build.id": "AP1A.240405.002"},
	}
}

// SampleAppPackage returns a sample app package for testing
func SampleAppPackage(name string) AppPackage {
	return AppPackage{
		Name:             name,
		Label:            "Sample App",
		Type:             "user",
		State:            "enabled",
		VersionName:      "1.0.0",
		VersionCode:      "1",
		TargetSdkVersion: "34",
		Permissions:      []string{"android.permission.INTERNET"},
		Activities:       []string{name + ".MainActivity"},
	}
}

// SampleWorkflow returns a sample workflow for testing
func SampleWorkflow(id, name string) Workflow {
	return Workflow{
		ID:          id,
		Name:        name,
		Description: "Test workflow",
		Version:     2,
		CreatedAt:   "2024-01-01T00:00:00Z",
		UpdatedAt:   "2024-01-01T00:00:00Z",
		Steps: []WorkflowStep{
			{
				ID:   "start",
				Type: "start",
				Name: "Start",
				Common: StepCommon{
					OnError: "stop",
					Loop:    1,
				},
				Connections: StepConnections{
					SuccessStepId: "step_1",
				},
				Layout: StepLayout{PosX: 20, PosY: 20},
			},
			{
				ID:   "step_1",
				Type: "tap",
				Name: "Tap button",
				Tap:  &TapParams{X: 540, Y: 960},
				Common: StepCommon{
					Timeout:   5000,
					PostDelay: 500,
					OnError:   "stop",
					Loop:      1,
				},
				Connections: StepConnections{},
				Layout:      StepLayout{PosX: 20, PosY: 180},
			},
		},
	}
}

// SampleSession returns a sample session for testing
func SampleSession(id, deviceID string) DeviceSession {
	return DeviceSession{
		ID:        id,
		DeviceID:  deviceID,
		Name:      "Test Session",
		Type:      "manual",
		Status:    "active",
		StartTime: 1700000000000,
	}
}
