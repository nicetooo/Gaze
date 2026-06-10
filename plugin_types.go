package main

import (
	"encoding/json"
	"regexp"
	"sync"
	"time"

	"Gaze/proxy"

	"github.com/dop251/goja"
)

// ========== 插件元数据 ==========

// PluginMetadata 插件元数据
type PluginMetadata struct {
	ID          string                 `json:"id"`          // 插件唯一标识
	Name        string                 `json:"name"`        // 显示名称
	Version     string                 `json:"version"`     // 版本号
	Author      string                 `json:"author"`      // 作者
	Description string                 `json:"description"` // 描述
	Enabled     bool                   `json:"enabled"`     // 是否启用
	Filters     PluginFilters          `json:"filters"`     // 事件过滤器
	Config      map[string]interface{} `json:"config"`      // 用户配置
	CreatedAt   time.Time              `json:"createdAt"`   // 创建时间
	UpdatedAt   time.Time              `json:"updatedAt"`   // 更新时间
}

// PluginFilters 插件事件过滤器
type PluginFilters struct {
	Sources    []string `json:"sources"`    // 事件来源过滤 (如 ["network", "logcat"])
	Types      []string `json:"types"`      // 事件类型过滤 (如 ["network_request"])
	Levels     []string `json:"levels"`     // 事件级别过滤 (如 ["error", "warn"])
	URLPattern string   `json:"urlPattern"` // URL 通配符 (仅对 network 事件有效)
	TitleMatch string   `json:"titleMatch"` // 标题正则匹配
}

// ========== 插件结构 ==========

// Plugin 插件实例
type Plugin struct {
	Metadata PluginMetadata `json:"metadata"` // 元数据

	// 源码和编译代码
	SourceCode   string `json:"sourceCode"`   // TypeScript 或 JavaScript 源码 (用于编辑)
	Language     string `json:"language"`     // 源码语言: "typescript" | "javascript"
	CompiledCode string `json:"compiledCode"` // 编译后的 JavaScript 代码 (用于执行)

	// 运行时 (不序列化)
	VM          *goja.Runtime `json:"-"` // JavaScript 虚拟机实例
	OnInitFunc  goja.Callable `json:"-"` // onInit 函数引用
	OnEventFunc goja.Callable `json:"-"` // onEvent 函数引用
	OnDestroy   goja.Callable `json:"-"` // onDestroy 函数引用

	// 并发保护: goja.Runtime 不是线程安全的，每次访问 VM/State 必须持有此锁
	mu sync.Mutex `json:"-"`

	// 预编译的正则表达式 (在 loadPluginLocked 时编译，避免热路径重复编译)
	titleMatchRegex *regexp.Regexp `json:"-"`

	// 状态
	State map[string]interface{} `json:"-"` // 插件状态存储 (跨事件)
}

// ========== 插件执行结果 ==========

// PluginResult 插件执行结果
type PluginResult struct {
	DerivedEvents []UnifiedEvent         `json:"derivedEvents"` // 生成的派生事件
	Tags          []string               `json:"tags"`          // 添加到原事件的标签
	Metadata      map[string]interface{} `json:"metadata"`      // 附加到原事件的元数据
}

// ========== 插件保存请求 ==========

// PluginSaveRequest 保存插件请求
type PluginSaveRequest struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Author       string                 `json:"author"`
	Description  string                 `json:"description"`
	SourceCode   string                 `json:"sourceCode"`   // TS 或 JS 源码
	Language     string                 `json:"language"`     // "typescript" | "javascript"
	CompiledCode string                 `json:"compiledCode"` // 编译后的 JS (前端编译时提供)
	Filters      PluginFilters          `json:"filters"`
	Config       map[string]interface{} `json:"config"`
}

// ========== 插件测试结果 ==========

// PluginTestResult 插件测试结果（增强版，包含详细信息）
type PluginTestResult struct {
	Success        bool           `json:"success"`         // 是否成功
	DerivedEvents  []UnifiedEvent `json:"derivedEvents"`   // 生成的派生事件
	ExecutionTime  int64          `json:"executionTimeMs"` // 执行时间（毫秒）
	Error          string         `json:"error,omitempty"` // 错误信息
	Logs           []string       `json:"logs,omitempty"`  // 执行日志
	MatchedFilters bool           `json:"matchedFilters"`  // 是否匹配过滤器
	EventSnapshot  *UnifiedEvent  `json:"eventSnapshot"`   // 测试用的事件快照
}

// ========== 辅助方法 ==========

// MatchesEvent 检查插件是否匹配事件
func (p *Plugin) MatchesEvent(event UnifiedEvent) bool {
	filters := p.Metadata.Filters

	// 检查 source
	if len(filters.Sources) > 0 {
		matched := false
		for _, source := range filters.Sources {
			if string(event.Source) == source {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 检查 type
	if len(filters.Types) > 0 {
		matched := false
		for _, typ := range filters.Types {
			if event.Type == typ {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 检查 level
	if len(filters.Levels) > 0 {
		matched := false
		for _, level := range filters.Levels {
			if string(event.Level) == level {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 检查 URL pattern
	if filters.URLPattern != "" {
		// URLPattern 隐式要求 Source 为 network：
		// 如果事件不是 network 来源，直接不匹配
		if event.Source != SourceNetwork {
			return false
		}
		// 从 event.data 中提取 URL
		var eventData map[string]interface{}
		if event.Data != nil {
			if err := json.Unmarshal(event.Data, &eventData); err == nil {
				if url, ok := eventData["url"].(string); ok {
					if !matchURLPattern(filters.URLPattern, url) {
						return false
					}
				} else {
					return false // 没有 URL 字段
				}
			} else {
				return false // JSON 解析失败
			}
		} else {
			return false // event.Data 为 nil
		}
	}

	// 检查 titleMatch (使用预编译正则)
	if p.titleMatchRegex != nil {
		if !p.titleMatchRegex.MatchString(event.Title) {
			return false
		}
	}

	return true
}

// ToJSON 转换为 JSON (用于存储)
func (p *Plugin) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// FromJSON 从 JSON 恢复 (用于加载)
func (p *Plugin) FromJSON(data []byte) error {
	return json.Unmarshal(data, p)
}

// ========== 辅助函数 ==========

// matchURLPattern URL 通配符匹配
func matchURLPattern(pattern, url string) bool {
	// 复用 proxy 包中的 MatchPattern
	return proxy.MatchPattern(url, pattern)
}
