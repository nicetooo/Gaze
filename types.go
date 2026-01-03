package main

import "time"

// Device represents a connected ADB device
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

// HistoryDevice represents a device in the connection history
type HistoryDevice struct {
	ID       string    `json:"id"`
	Serial   string    `json:"serial"`
	Model    string    `json:"model"`
	Brand    string    `json:"brand"`
	Type     string    `json:"type"`
	WifiAddr string    `json:"wifiAddr"`
	LastSeen time.Time `json:"lastSeen"`
}

// DeviceInfo contains detailed information about a device
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

// FileInfo represents a file or directory on the device
type FileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"modTime"`
	IsDir   bool   `json:"isDir"`
	Path    string `json:"path"`
}

// NetworkStats contains network usage statistics
type NetworkStats struct {
	DeviceId string `json:"deviceId"`
	RxBytes  uint64 `json:"rxBytes"`
	TxBytes  uint64 `json:"txBytes"`
	RxSpeed  uint64 `json:"rxSpeed"` // bytes per second
	TxSpeed  uint64 `json:"txSpeed"` // bytes per second
	Time     int64  `json:"time"`
}

// AppPackage represents an installed application
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

// ScrcpyConfig contains configuration for scrcpy screen mirroring
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

// AppSettings contains persistent application settings
type AppSettings struct {
	LastActive   map[string]int64 `json:"lastActive"`
	PinnedSerial string           `json:"pinnedSerial"`
}

// BatchOperation represents a batch operation to execute on multiple devices
type BatchOperation struct {
	Type        string   `json:"type"`        // "install", "uninstall", "clear", "stop", "shell", "push"
	DeviceIDs   []string `json:"deviceIds"`   // List of device IDs to operate on
	PackageName string   `json:"packageName"` // For app operations
	APKPath     string   `json:"apkPath"`     // For install operation
	Command     string   `json:"command"`     // For shell operation
	LocalPath   string   `json:"localPath"`   // For push operation
	RemotePath  string   `json:"remotePath"`  // For push operation
}

// BatchResult represents the result of a batch operation for a single device
type BatchResult struct {
	DeviceID string `json:"deviceId"`
	Success  bool   `json:"success"`
	Output   string `json:"output"`
	Error    string `json:"error"`
}

// BatchOperationResult represents the complete result of a batch operation
type BatchOperationResult struct {
	TotalDevices int           `json:"totalDevices"`
	SuccessCount int           `json:"successCount"`
	FailureCount int           `json:"failureCount"`
	Results      []BatchResult `json:"results"`
}

// TouchEvent represents a single touch event in an automation script
type TouchEvent struct {
	Timestamp int64  `json:"timestamp"` // Relative time in milliseconds from script start
	Type      string `json:"type"`      // "tap", "swipe", "wait"
	X         int    `json:"x"`
	Y         int    `json:"y"`
	X2        int    `json:"x2,omitempty"`       // End X for swipe
	Y2        int    `json:"y2,omitempty"`       // End Y for swipe
	Duration  int    `json:"duration,omitempty"` // Duration in ms for swipe or wait
	Label     string `json:"label,omitempty"`    // Optional label (e.g. text of the button clicked)
	ResID     string `json:"resId,omitempty"`    // Optional resource ID of the element
}

// TouchScript represents a recorded touch automation script
type TouchScript struct {
	Name        string       `json:"name"`
	DeviceID    string       `json:"deviceId"`
	DeviceModel string       `json:"deviceModel,omitempty"` // Store device model name
	Resolution  string       `json:"resolution"`            // e.g. "1080x2400"
	CreatedAt   string       `json:"createdAt"`
	Events      []TouchEvent `json:"events"`
}

// TouchRecordingSession represents an active recording session
type TouchRecordingSession struct {
	DeviceID    string
	StartTime   time.Time
	RawEvents   []string // Raw getevent output lines
	Resolution  string
	InputDevice string // e.g. "/dev/input/event2"
	MaxX        int
	MaxY        int
	MinX        int
	MinY        int
}

// TaskStep represents a step in a composite task
type TaskStep struct {
	Type        string `json:"type"`        // "script", "wait", "adb", "check"
	Value       string `json:"value"`       // Script name, duration, adb command, or selector
	Loop        int    `json:"loop"`        // Number of times to repeat this step
	PostDelay   int    `json:"postDelay"`   // Wait time in ms AFTER this step
	CheckType   string `json:"checkType"`   // "text", "id", "class", "contains"
	CheckValue  string `json:"checkValue"`  // Success condition value
	WaitTimeout int    `json:"waitTimeout"` // Max time to wait for condition in ms
	OnFailure   string `json:"onFailure"`   // "stop", "continue"
}

// ScriptTask represents a sequence of automation steps
type ScriptTask struct {
	Name      string     `json:"name"`
	Steps     []TaskStep `json:"steps"`
	CreatedAt string     `json:"createdAt"`
}

// --- New Workflow Types ---

type ElementSelector struct {
	Type  string `json:"type"` // "text", "id", "xpath", "advanced"
	Value string `json:"value"`
	Index int    `json:"index,omitempty"`
}

type WorkflowStep struct {
	ID            string           `json:"id"`
	Type          string           `json:"type"`
	Name          string           `json:"name,omitempty"`
	Selector      *ElementSelector `json:"selector,omitempty"`
	Value         string           `json:"value,omitempty"`
	Timeout       int              `json:"timeout,omitempty"`
	OnError       string           `json:"onError,omitempty"` // "stop", "continue"
	Loop          int              `json:"loop,omitempty"`
	PostDelay     int              `json:"postDelay,omitempty"`
	PreWait       int              `json:"preWait,omitempty"`
	SwipeDistance int              `json:"swipeDistance,omitempty"`
	SwipeDuration int              `json:"swipeDuration,omitempty"`
	ConditionType string           `json:"conditionType,omitempty"` // "exists", "not_exists", "text_equals", "text_contains", "variable_equals"
	// Graph Flow Control
	NextStepId  string `json:"nextStepId,omitempty"`  // Default next step
	NextSource  string `json:"nextSource,omitempty"`  // Handle ID for next step
	NextTarget  string `json:"nextTarget,omitempty"`  // Handle ID for next step target
	TrueStepId  string `json:"trueStepId,omitempty"`  // Branch TRUE target
	TrueSource  string `json:"trueSource,omitempty"`  // Handle ID for true step
	TrueTarget  string `json:"trueTarget,omitempty"`  // Handle ID for true step target
	FalseStepId string `json:"falseStepId,omitempty"` // Branch FALSE target
	FalseSource string `json:"falseSource,omitempty"` // Handle ID for false step
	FalseTarget string `json:"falseTarget,omitempty"` // Handle ID for false step target

	// Visual Layout
	PosX float64 `json:"posX,omitempty"`
	PosY float64 `json:"posY,omitempty"`
}

type Workflow struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Steps       []WorkflowStep    `json:"steps"`
	Variables   map[string]string `json:"variables,omitempty"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
}
