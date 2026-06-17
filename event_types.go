package main

import (
	"encoding/json"
)

// ========================================
// Event Source - 事件来源
// ========================================

type EventSource string

const (
	SourceLogcat    EventSource = "logcat"    // 设备日志
	SourceNetwork   EventSource = "network"   // 网络请求
	SourceDevice    EventSource = "device"    // 设备状态
	SourceApp       EventSource = "app"       // 应用生命周期
	SourceUI        EventSource = "ui"        // UI 状态变化
	SourceTouch     EventSource = "touch"     // 触摸事件
	SourceWorkflow  EventSource = "workflow"  // 自动化流程
	SourcePerf      EventSource = "perf"      // 性能指标
	SourceSystem    EventSource = "system"    // 系统事件
	SourceAssertion EventSource = "assertion" // 断言结果
	SourcePlugin    EventSource = "plugin"    // 插件生成的事件
)

// ========================================
// Event Category - 事件大类
// ========================================

type EventCategory string

const (
	CategoryLog         EventCategory = "log"
	CategoryNetwork     EventCategory = "network"
	CategoryState       EventCategory = "state"
	CategoryInteraction EventCategory = "interaction"
	CategoryAutomation  EventCategory = "automation"
	CategoryDiagnostic  EventCategory = "diagnostic"
	CategoryPlugin      EventCategory = "plugin" // 插件生成的事件
)

// ========================================
// Event Level - 事件级别
// ========================================

type EventLevel string

const (
	LevelVerbose EventLevel = "verbose"
	LevelDebug   EventLevel = "debug"
	LevelInfo    EventLevel = "info"
	LevelWarn    EventLevel = "warn"
	LevelError   EventLevel = "error"
	LevelFatal   EventLevel = "fatal"
)

// ========================================
// UnifiedEvent - 统一事件结构
// ========================================

type UnifiedEvent struct {
	// === 标识字段 ===
	ID        string `json:"id"`        // UUID
	SessionID string `json:"sessionId"` // 所属会话
	DeviceID  string `json:"deviceId"`  // 设备 ID

	// === 时间字段 ===
	Timestamp    int64 `json:"timestamp"`          // Unix 毫秒 (绝对时间)
	RelativeTime int64 `json:"relativeTime"`       // 相对 Session 开始的毫秒偏移
	Duration     int64 `json:"duration,omitempty"` // 持续时间 (ms)

	// === 分类字段 ===
	Source   EventSource   `json:"source"`   // 事件来源
	Category EventCategory `json:"category"` // 事件大类
	Type     string        `json:"type"`     // 具体类型 (如 "network_request", "activity_start")
	Level    EventLevel    `json:"level"`    // 事件级别

	// === 内容字段 ===
	Title   string `json:"title"`             // 简短标题 (用于列表显示)
	Summary string `json:"summary,omitempty"` // 摘要 (可搜索)

	// === 扩展数据 (JSON) ===
	Data   json.RawMessage `json:"data,omitempty"`   // 类型特定的详细数据
	Detail json.RawMessage `json:"detail,omitempty"` // 别名，向后兼容

	// === 关联字段 ===
	ParentID string `json:"parentId,omitempty"` // 父事件 ID
	StepID   string `json:"stepId,omitempty"`   // Workflow Step ID
	TraceID  string `json:"traceId,omitempty"`  // 跨事件追踪 ID

	// === 聚合信息 ===
	AggregateCount int   `json:"aggregateCount,omitempty"` // 聚合的事件数量
	AggregateFirst int64 `json:"aggregateFirst,omitempty"` // 聚合的第一条时间
	AggregateLast  int64 `json:"aggregateLast,omitempty"`  // 聚合的最后一条时间

	// === 插件扩展字段 ===
	Tags              []string               `json:"tags,omitempty"`              // 标签（插件可添加）
	Metadata          map[string]interface{} `json:"metadata,omitempty"`          // 元数据（插件可附加）
	ParentEventID     string                 `json:"parentEventId,omitempty"`     // 派生事件的父事件 ID
	GeneratedByPlugin string                 `json:"generatedByPlugin,omitempty"` // 生成此事件的插件 ID
	DerivedDepth      int                    `json:"derivedDepth,omitempty"`      // 派生深度 (0=原始事件, 1=一级派生, ...)
}

// ========================================
// DeviceSession - 设备会话
// ========================================

// SessionConfig - Session 启动配置
type SessionConfig struct {
	Logcat    LogcatConfig    `json:"logcat"`
	Recording RecordingConfig `json:"recording"`
	Proxy     ProxyConfig     `json:"proxy"`
	Monitor   MonitorConfig   `json:"monitor"`
}

type LogcatConfig struct {
	Enabled       bool   `json:"enabled"`
	PackageName   string `json:"packageName,omitempty"`
	PreFilter     string `json:"preFilter,omitempty"`
	ExcludeFilter string `json:"excludeFilter,omitempty"`
}

type RecordingConfig struct {
	Enabled bool   `json:"enabled"`
	Quality string `json:"quality,omitempty"` // "low", "medium", "high"
}

type ProxyConfig struct {
	Enabled     bool `json:"enabled"`
	Port        int  `json:"port,omitempty"`
	MitmEnabled bool `json:"mitmEnabled,omitempty"`
}

type MonitorConfig struct {
	Enabled bool `json:"enabled"` // 设备状态监控 (电池、网络、屏幕、应用生命周期)
}

type DeviceSession struct {
	ID         string `json:"id"`
	DeviceID   string `json:"deviceId"`
	Type       string `json:"type"` // "manual", "workflow", "recording", "debug", "auto"
	Name       string `json:"name"`
	StartTime  int64  `json:"startTime"` // Unix ms
	EndTime    int64  `json:"endTime"`   // 0 = active
	Status     string `json:"status"`    // "active", "completed", "failed", "cancelled"
	EventCount int    `json:"eventCount"`

	// Session config (启用的功能)
	Config SessionConfig `json:"config"`

	// Recording info
	VideoPath     string `json:"videoPath,omitempty"`
	VideoDuration int64  `json:"videoDuration,omitempty"`
	VideoOffset   int64  `json:"videoOffset,omitempty"` // 视频相对 session 开始的偏移

	// Metadata
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ========================================
// Type-Specific Data Structures
// ========================================

// LogcatData logcat 事件数据
type LogcatData struct {
	Tag          string `json:"tag"`
	Message      string `json:"message"`
	AndroidLevel string `json:"androidLevel"` // V/D/I/W/E/F
	PID          int    `json:"pid,omitempty"`
	TID          int    `json:"tid,omitempty"`
	PackageName  string `json:"packageName,omitempty"`
	Raw          string `json:"raw,omitempty"`
}

// ========================================
// Time Index Entry - 时间索引条目
// ========================================

type TimeIndexEntry struct {
	Second       int    `json:"second"`       // 相对 session 开始的秒数
	EventCount   int    `json:"eventCount"`   // 该秒内的事件数量
	FirstEventID string `json:"firstEventId"` // 该秒内第一个事件的 ID
	HasError     bool   `json:"hasError"`     // 该秒内是否有错误事件
}

// ========================================
// Bookmark - 时间书签
// ========================================

type Bookmark struct {
	ID           string `json:"id"`
	SessionID    string `json:"sessionId"`
	RelativeTime int64  `json:"relativeTime"` // 相对 session 开始的毫秒
	Label        string `json:"label"`
	Color        string `json:"color,omitempty"`
	Type         string `json:"type"` // user, error, milestone, assertion_fail
	CreatedAt    int64  `json:"createdAt"`
}

// ========================================
// Event Type Registry - 事件类型注册表
// ========================================

type EventTypeInfo struct {
	Type        string        // 类型标识
	Source      EventSource   // 来源
	Category    EventCategory // 大类
	Description string        // 描述
}

var EventRegistry = map[string]EventTypeInfo{
	// === Logcat 事件 ===
	"logcat": {
		Type: "logcat", Source: SourceLogcat, Category: CategoryLog,
		Description: "Android logcat output",
	},
	"logcat_aggregated": {
		Type: "logcat_aggregated", Source: SourceLogcat, Category: CategoryLog,
		Description: "Aggregated logcat entries",
	},

	// === Network 事件 ===
	"http_request": {
		Type: "http_request", Source: SourceNetwork, Category: CategoryNetwork,
		Description: "DEPRECATED: Use 'network_request' instead. HTTP/HTTPS request (legacy name)",
	},
	"network_request": {
		Type: "network_request", Source: SourceNetwork, Category: CategoryNetwork,
		Description: "Network request (proxy-captured) - Standard event type for all HTTP/HTTPS traffic",
	},
	"websocket_message": {
		Type: "websocket_message", Source: SourceNetwork, Category: CategoryNetwork,
		Description: "WebSocket message",
	},

	// === Device State 事件 ===
	"battery_change": {
		Type: "battery_change", Source: SourceDevice, Category: CategoryState,
		Description: "Battery level or status change",
	},
	"network_change": {
		Type: "network_change", Source: SourceDevice, Category: CategoryState,
		Description: "Network connectivity change",
	},
	"screen_change": {
		Type: "screen_change", Source: SourceDevice, Category: CategoryState,
		Description: "Screen state change",
	},

	// === App Lifecycle 事件 ===
	"app_start": {
		Type: "app_start", Source: SourceApp, Category: CategoryState,
		Description: "Application started",
	},
	"app_stop": {
		Type: "app_stop", Source: SourceApp, Category: CategoryState,
		Description: "Application stopped",
	},
	"activity_start": {
		Type: "activity_start", Source: SourceApp, Category: CategoryState,
		Description: "Activity started",
	},
	"activity_stop": {
		Type: "activity_stop", Source: SourceApp, Category: CategoryState,
		Description: "Activity stopped",
	},
	"app_crash": {
		Type: "app_crash", Source: SourceApp, Category: CategoryState,
		Description: "Application crash",
	},
	"app_anr": {
		Type: "app_anr", Source: SourceApp, Category: CategoryState,
		Description: "Application Not Responding",
	},

	// === Touch 事件 ===
	"touch": {
		Type: "touch", Source: SourceTouch, Category: CategoryInteraction,
		Description: "Touch event",
	},
	"gesture": {
		Type: "gesture", Source: SourceTouch, Category: CategoryInteraction,
		Description: "Recognized gesture",
	},

	// === Workflow 事件 ===
	"workflow_start": {
		Type: "workflow_start", Source: SourceWorkflow, Category: CategoryAutomation,
		Description: "Workflow execution started",
	},
	"workflow_step_start": {
		Type: "workflow_step_start", Source: SourceWorkflow, Category: CategoryAutomation,
		Description: "Workflow step started",
	},
	"workflow_step_end": {
		Type: "workflow_step_end", Source: SourceWorkflow, Category: CategoryAutomation,
		Description: "Workflow step completed",
	},
	"workflow_complete": {
		Type: "workflow_complete", Source: SourceWorkflow, Category: CategoryAutomation,
		Description: "Workflow execution completed",
	},
	"workflow_error": {
		Type: "workflow_error", Source: SourceWorkflow, Category: CategoryAutomation,
		Description: "Workflow execution error",
	},

	// === Performance 事件 ===
	"perf_sample": {
		Type: "perf_sample", Source: SourcePerf, Category: CategoryDiagnostic,
		Description: "Performance metric sample",
	},

	// === Assertion 事件 ===
	"assertion_result": {
		Type: "assertion_result", Source: SourceAssertion, Category: CategoryAutomation,
		Description: "Assertion evaluation result",
	},

	// === Breakpoint 事件 ===
	"breakpoint_hit": {
		Type: "breakpoint_hit", Source: SourceNetwork, Category: CategoryNetwork,
		Description: "Request/response paused at breakpoint",
	},
	"breakpoint_resolved": {
		Type: "breakpoint_resolved", Source: SourceNetwork, Category: CategoryNetwork,
		Description: "Breakpoint resolved (forwarded/dropped/modified)",
	},

	// === System 事件 ===
	"session_start": {
		Type: "session_start", Source: SourceSystem, Category: CategoryState,
		Description: "Session started",
	},
	"session_end": {
		Type: "session_end", Source: SourceSystem, Category: CategoryState,
		Description: "Session ended",
	},
	"recording_start": {
		Type: "recording_start", Source: SourceSystem, Category: CategoryState,
		Description: "Screen recording started",
	},
	"recording_end": {
		Type: "recording_end", Source: SourceSystem, Category: CategoryState,
		Description: "Screen recording ended",
	},
	"batch_operation": {
		Type: "batch_operation", Source: SourceSystem, Category: CategoryState,
		Description: "Batch operation result on a device",
	},
	"plugin_error": {
		Type: "plugin_error", Source: SourceSystem, Category: CategoryLog,
		Description: "Plugin execution error",
	},
}

// ParseEventLevel converts a string level (e.g. from old SessionEvent) to EventLevel.
// Returns LevelInfo for unrecognized values.
func ParseEventLevel(level string) EventLevel {
	switch level {
	case "fatal":
		return LevelFatal
	case "error":
		return LevelError
	case "warn":
		return LevelWarn
	case "info":
		return LevelInfo
	case "debug":
		return LevelDebug
	case "verbose":
		return LevelVerbose
	default:
		return LevelInfo
	}
}

// GetCategoryForType 根据事件类型获取分类
func GetCategoryForType(eventType string) EventCategory {
	if info, ok := EventRegistry[eventType]; ok {
		return info.Category
	}
	return CategoryDiagnostic
}
