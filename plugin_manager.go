package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

// PluginManager 插件管理器
type PluginManager struct {
	plugins map[string]*Plugin // ID -> Plugin
	mu      sync.RWMutex       // 读写锁

	store         *PluginStore   // 数据库存储
	eventPipeline *EventPipeline // 事件管道引用
}

// NewPluginManager 创建插件管理器
func NewPluginManager(store *PluginStore, eventPipeline *EventPipeline) *PluginManager {
	return &PluginManager{
		plugins:       make(map[string]*Plugin),
		store:         store,
		eventPipeline: eventPipeline,
	}
}

// LoadAllPlugins 从数据库加载所有插件
func (pm *PluginManager) LoadAllPlugins() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugins, err := pm.store.ListPlugins()
	if err != nil {
		return fmt.Errorf("加载插件列表失败: %w", err)
	}

	for _, plugin := range plugins {
		if plugin.Metadata.Enabled {
			if err := pm.loadPluginLocked(plugin); err != nil {
				log.Printf("[PluginManager] 加载插件 %s 失败: %v", plugin.Metadata.ID, err)
				continue
			}
			log.Printf("[PluginManager] 已加载插件: %s (%s)", plugin.Metadata.Name, plugin.Metadata.ID)
		}
	}

	return nil
}

// LoadPlugin 加载单个插件 (外部调用)
func (pm *PluginManager) LoadPlugin(plugin *Plugin) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.loadPluginLocked(plugin)
}

// loadPluginLocked 加载插件 (需持有锁)
func (pm *PluginManager) loadPluginLocked(plugin *Plugin) error {
	// 创建独立的 VM 实例
	vm := goja.New()

	// 初始化插件状态
	if plugin.State == nil {
		plugin.State = make(map[string]interface{})
	}

	// 注入辅助函数到 VM 全局
	if err := pm.injectHelpers(vm, plugin); err != nil {
		return fmt.Errorf("注入辅助函数失败: %w", err)
	}

	// 执行插件脚本
	_, err := vm.RunString(plugin.CompiledCode)
	if err != nil {
		return fmt.Errorf("插件脚本执行失败: %w", err)
	}

	// 获取插件对象
	pluginObj := vm.Get("plugin")
	if goja.IsUndefined(pluginObj) {
		return fmt.Errorf("未找到 plugin 对象")
	}

	obj := pluginObj.ToObject(vm)

	// 提取 onEvent 函数 (必需)
	onEventVal := obj.Get("onEvent")
	if goja.IsUndefined(onEventVal) {
		return fmt.Errorf("未找到 onEvent 函数")
	}
	onEventFunc, ok := goja.AssertFunction(onEventVal)
	if !ok {
		return fmt.Errorf("onEvent 不是函数")
	}

	// 提取 onInit 函数 (可选)
	var onInitFunc goja.Callable
	onInitVal := obj.Get("onInit")
	if !goja.IsUndefined(onInitVal) {
		onInitFunc, _ = goja.AssertFunction(onInitVal)
	}

	// 提取 onDestroy 函数 (可选)
	var onDestroyFunc goja.Callable
	onDestroyVal := obj.Get("onDestroy")
	if !goja.IsUndefined(onDestroyVal) {
		onDestroyFunc, _ = goja.AssertFunction(onDestroyVal)
	}

	// 预编译 titleMatch 正则表达式 (避免热路径重复编译)
	if plugin.Metadata.Filters.TitleMatch != "" {
		re, err := regexp.Compile(plugin.Metadata.Filters.TitleMatch)
		if err != nil {
			return fmt.Errorf("无效的 titleMatch 正则表达式 '%s': %w", plugin.Metadata.Filters.TitleMatch, err)
		}
		plugin.titleMatchRegex = re
	} else {
		plugin.titleMatchRegex = nil
	}

	// 保存到插件实例
	plugin.VM = vm
	plugin.OnEventFunc = onEventFunc
	plugin.OnInitFunc = onInitFunc
	plugin.OnDestroy = onDestroyFunc

	// 调用 onInit (如果存在)
	if onInitFunc != nil {
		context := pm.createPluginContext(vm, plugin)
		if _, err := onInitFunc(goja.Undefined(), context); err != nil {
			log.Printf("[PluginManager] 插件 %s 的 onInit 执行失败: %v", plugin.Metadata.ID, err)
		}
	}

	// 保存到内存
	pm.plugins[plugin.Metadata.ID] = plugin

	return nil
}

// UnloadPlugin 卸载插件
func (pm *PluginManager) UnloadPlugin(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, exists := pm.plugins[id]
	if !exists {
		return fmt.Errorf("插件不存在: %s", id)
	}

	// ⭐ 获取插件锁，确保没有正在执行的 goroutine 在使用 VM/State
	plugin.mu.Lock()
	defer plugin.mu.Unlock()

	// 调用 onDestroy (如果存在)
	if plugin.OnDestroy != nil {
		context := pm.createPluginContext(plugin.VM, plugin)
		if _, err := plugin.OnDestroy(goja.Undefined(), context); err != nil {
			log.Printf("[PluginManager] 插件 %s 的 onDestroy 执行失败: %v", id, err)
		}
	}

	// 清空插件状态和运行时，帮助 GC 回收内存
	plugin.State = nil
	plugin.VM = nil
	plugin.OnInitFunc = nil
	plugin.OnEventFunc = nil
	plugin.OnDestroy = nil

	// 从内存删除
	delete(pm.plugins, id)

	log.Printf("[PluginManager] 已卸载插件: %s (内存已清理)", id)
	return nil
}

// ProcessEvent 处理事件 (从 EventPipeline 调用)
func (pm *PluginManager) ProcessEvent(event UnifiedEvent, sessionID string) []UnifiedEvent {
	// 短暂持有读锁，仅用于收集匹配的插件列表
	pm.mu.RLock()
	var eligiblePlugins []*Plugin
	for _, plugin := range pm.plugins {
		if !plugin.Metadata.Enabled {
			continue
		}
		if plugin.MatchesEvent(event) {
			eligiblePlugins = append(eligiblePlugins, plugin)
		}
	}
	pm.mu.RUnlock()
	// 读锁已释放：后续 goroutine 通过 plugin.mu 保护 VM 访问，
	// 不再需要持有 pm.mu，这样 LoadPlugin/UnloadPlugin 可以及时获取写锁

	// 如果没有匹配的插件，直接返回
	if len(eligiblePlugins) == 0 {
		return []UnifiedEvent{}
	}

	// 并发执行所有匹配的插件
	type pluginResult struct {
		pluginID      string
		pluginName    string
		derivedEvents []UnifiedEvent
		err           error
	}

	resultsChan := make(chan pluginResult, len(eligiblePlugins))

	for _, plugin := range eligiblePlugins {
		go func(p *Plugin) {
			// panic recovery: 防止单个插件崩溃导致整个事件管道阻塞
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[PluginManager] Plugin %s panicked: %v", p.Metadata.ID, r)
					errorEvent := pm.createPluginErrorEvent(p.Metadata.ID, event.ID, fmt.Errorf("plugin panic: %v", r))
					resultsChan <- pluginResult{
						pluginID:      p.Metadata.ID,
						pluginName:    p.Metadata.Name,
						derivedEvents: []UnifiedEvent{errorEvent},
						err:           fmt.Errorf("plugin panic: %v", r),
					}
				}
			}()

			result, err := pm.executePlugin(p, event, sessionID)

			if err != nil {
				log.Printf("[PluginManager] Plugin %s failed: %v", p.Metadata.ID, err)
				errorEvent := pm.createPluginErrorEvent(p.Metadata.ID, event.ID, err)
				resultsChan <- pluginResult{
					pluginID:      p.Metadata.ID,
					pluginName:    p.Metadata.Name,
					derivedEvents: []UnifiedEvent{errorEvent},
					err:           err,
				}
				return
			}

			// 填充派生事件字段
			var derivedEvents []UnifiedEvent
			if result != nil && len(result.DerivedEvents) > 0 {
				for i := range result.DerivedEvents {
					derived := &result.DerivedEvents[i]
					derived.ID = uuid.New().String()
					derived.DeviceID = event.DeviceID
					derived.SessionID = sessionID

					// 仅在插件未设置时间戳时才使用父事件的时间戳
					if derived.Timestamp == 0 {
						derived.Timestamp = event.Timestamp
					}
					// RelativeTime 会在派生事件重新进入 EventPipeline 时自动计算

					derived.Category = CategoryPlugin
					derived.ParentEventID = event.ID
					derived.GeneratedByPlugin = p.Metadata.ID
					derived.DerivedDepth = event.DerivedDepth + 1 // 继承父事件深度 + 1

					// 添加元数据标记
					if derived.Metadata == nil {
						derived.Metadata = make(map[string]interface{})
					}
					derived.Metadata["generatedBy"] = p.Metadata.ID
					derived.Metadata["pluginName"] = p.Metadata.Name
				}
				derivedEvents = result.DerivedEvents
			}

			resultsChan <- pluginResult{
				pluginID:      p.Metadata.ID,
				pluginName:    p.Metadata.Name,
				derivedEvents: derivedEvents,
			}
		}(plugin)
	}

	// 收集所有结果
	var allDerivedEvents []UnifiedEvent
	for i := 0; i < len(eligiblePlugins); i++ {
		result := <-resultsChan
		allDerivedEvents = append(allDerivedEvents, result.derivedEvents...)
	}

	return allDerivedEvents
}

// ExecutePluginWithLogging 执行插件并捕获日志（用于测试，带并发安全）
func (pm *PluginManager) ExecutePluginWithLogging(plugin *Plugin, event UnifiedEvent, sessionID string) (result *PluginResult, logs []string, executionTime int64, err error) {
	startTime := time.Now()
	logs = []string{}

	// 超时控制
	timeout := 5 * time.Second
	resultChan := make(chan *PluginResult, 1)
	errorChan := make(chan error, 1)
	logsChan := make(chan []string, 1)

	go func() {
		capturedLogs := []string{}

		// panic recovery
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PluginManager] Plugin %s VM panicked during test: %v", plugin.Metadata.ID, r)
				errorChan <- fmt.Errorf("plugin VM panic: %v", r)
				logsChan <- capturedLogs
			}
		}()

		// 加锁保护: goja.Runtime 不是线程安全的
		plugin.mu.Lock()
		defer plugin.mu.Unlock()

		// 检查插件是否已被卸载
		if plugin.VM == nil || plugin.OnEventFunc == nil {
			errorChan <- fmt.Errorf("plugin %s has been unloaded", plugin.Metadata.ID)
			logsChan <- capturedLogs
			return
		}

		// 清除可能残留的 interrupt 状态（上次超时可能留下的）
		plugin.VM.ClearInterrupt()

		// 创建带日志捕获的上下文
		context, collector := pm.createEventContextWithLogging(plugin.VM, plugin, event, sessionID, &capturedLogs)

		// 准备事件对象
		var eventForPlugin map[string]interface{}
		eventBytes, _ := json.Marshal(event)
		json.Unmarshal(eventBytes, &eventForPlugin)

		if len(event.Data) > 0 {
			var dataMap map[string]interface{}
			if err := json.Unmarshal(event.Data, &dataMap); err == nil {
				eventForPlugin["data"] = dataMap
			}
		}

		eventObj := plugin.VM.ToValue(eventForPlugin)

		// 调用 onEvent
		resultVal, err := plugin.OnEventFunc(goja.Undefined(), eventObj, context)
		if err != nil {
			errorChan <- err
			logsChan <- capturedLogs
			return
		}

		// 解析返回值
		result := &PluginResult{}
		if !goja.IsUndefined(resultVal) && !goja.IsNull(resultVal) {
			jsObj := resultVal.Export()
			jsonBytes, err := json.Marshal(jsObj)
			if err != nil {
				errorChan <- fmt.Errorf("JSON 序列化失败: %w", err)
				logsChan <- capturedLogs
				return
			}

			if err := json.Unmarshal(jsonBytes, result); err != nil {
				errorChan <- fmt.Errorf("解析插件返回值失败: %w", err)
				logsChan <- capturedLogs
				return
			}
		}

		// 合并 context.emit() 产生的事件到返回结果
		if len(collector.events) > 0 {
			result.DerivedEvents = append(result.DerivedEvents, collector.events...)
		}

		resultChan <- result
		logsChan <- capturedLogs
	}()

	select {
	case result := <-resultChan:
		logs = <-logsChan
		executionTime = time.Since(startTime).Milliseconds()
		return result, logs, executionTime, nil
	case err := <-errorChan:
		logs = <-logsChan
		executionTime = time.Since(startTime).Milliseconds()
		return nil, logs, executionTime, err
	case <-time.After(timeout):
		// Interrupt 是线程安全的，可以在不持有锁时调用
		if plugin.VM != nil {
			plugin.VM.Interrupt("timeout")
		}
		executionTime = time.Since(startTime).Milliseconds()
		return nil, logs, executionTime, fmt.Errorf("插件执行超时 (>%v)", timeout)
	}
}

// executePlugin 执行单个插件 (带超时保护和并发安全)
// 通过 plugin.mu 锁保证同一插件的 VM 和 State 不会被并发访问
func (pm *PluginManager) executePlugin(plugin *Plugin, event UnifiedEvent, sessionID string) (*PluginResult, error) {
	// 超时控制
	timeout := 5 * time.Second
	resultChan := make(chan *PluginResult, 1)
	errorChan := make(chan error, 1)

	go func() {
		// panic recovery: 防止 VM 内部 panic 导致 channel 阻塞
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PluginManager] Plugin %s VM panicked: %v", plugin.Metadata.ID, r)
				errorChan <- fmt.Errorf("plugin VM panic: %v", r)
			}
		}()

		// 加锁保护: goja.Runtime 不是线程安全的，同一插件的 VM 必须串行访问
		plugin.mu.Lock()
		defer plugin.mu.Unlock()

		// 检查插件是否已被卸载 (VM 被设为 nil)
		if plugin.VM == nil || plugin.OnEventFunc == nil {
			errorChan <- fmt.Errorf("plugin %s has been unloaded", plugin.Metadata.ID)
			return
		}

		// 清除可能残留的 interrupt 状态（上次超时可能留下的）
		plugin.VM.ClearInterrupt()

		// 准备事件上下文
		context, collector := pm.createEventContext(plugin.VM, plugin, event, sessionID)

		// 将 event.Data (json.RawMessage) 解析为 map，以便插件可以直接访问字段
		var eventForPlugin map[string]interface{}
		eventBytes, _ := json.Marshal(event)
		json.Unmarshal(eventBytes, &eventForPlugin)

		if len(event.Data) > 0 {
			var dataMap map[string]interface{}
			if err := json.Unmarshal(event.Data, &dataMap); err == nil {
				eventForPlugin["data"] = dataMap
			}
		}

		eventObj := plugin.VM.ToValue(eventForPlugin)

		// 调用 onEvent
		resultVal, err := plugin.OnEventFunc(goja.Undefined(), eventObj, context)
		if err != nil {
			errorChan <- err
			return
		}

		// 解析返回值
		result := &PluginResult{}
		if !goja.IsUndefined(resultVal) && !goja.IsNull(resultVal) {
			// 先导出为 JSON，再用 json.Unmarshal 解析
			// 这样可以正确使用 Go 结构体的 JSON tags (derivedEvents -> DerivedEvents)
			jsObj := resultVal.Export()
			jsonBytes, err := json.Marshal(jsObj)
			if err != nil {
				errorChan <- fmt.Errorf("JSON 序列化失败: %w", err)
				return
			}
			if err := json.Unmarshal(jsonBytes, result); err != nil {
				errorChan <- fmt.Errorf("解析插件返回值失败: %w", err)
				return
			}
		}

		// 合并 context.emit() 产生的事件到返回结果
		if len(collector.events) > 0 {
			result.DerivedEvents = append(result.DerivedEvents, collector.events...)
		}

		resultChan <- result
	}()

	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errorChan:
		return nil, err
	case <-time.After(timeout):
		// 超时：中断 VM (Interrupt 是线程安全的，可以在不持有锁时调用)
		// 注意: 如果插件已被卸载 (VM=nil)，此处需要保护
		if plugin.VM != nil {
			plugin.VM.Interrupt("timeout")
		}
		return nil, fmt.Errorf("插件执行超时 (>%v)", timeout)
	}
}

// ========== 辅助函数注入 ==========

// injectHelpers 注入辅助函数到 VM
func (pm *PluginManager) injectHelpers(vm *goja.Runtime, plugin *Plugin) error {
	// matchURL: URL 通配符匹配（参数顺序与 context.matchURL 及 plugin.d.ts 声明一致）
	vm.Set("matchURL", func(url, pattern string) bool {
		return matchURLPattern(pattern, url)
	})

	// matchRegex: 正则匹配
	vm.Set("matchRegex", func(regexStr, text string) interface{} {
		re, err := regexp.Compile(regexStr)
		if err != nil {
			return nil
		}
		matches := re.FindStringSubmatch(text)
		if matches == nil {
			return nil
		}
		return matches
	})

	// jsonPath: JSON Path 查询
	vm.Set("jsonPath", func(obj interface{}, path string) interface{} {
		// 将 obj 转换为 JSON 字符串
		jsonBytes, err := json.Marshal(obj)
		if err != nil {
			return nil
		}
		// 使用 gjson 查询
		result := gjson.GetBytes(jsonBytes, path)
		if !result.Exists() {
			return nil
		}
		return result.Value()
	})

	// base64Decode: Base64 解码
	vm.Set("base64Decode", func(encoded string) string {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			log.Printf("[PluginManager] Base64 decode error: %v", err)
			return ""
		}
		return string(decoded)
	})

	// parseURL: URL 解析
	vm.Set("parseURL", func(urlStr string) interface{} {
		parsedURL, err := url.Parse(urlStr)
		if err != nil {
			log.Printf("[PluginManager] URL parse error: %v", err)
			return nil
		}

		// 返回 URL 对象的关键字段
		return map[string]interface{}{
			"scheme":   parsedURL.Scheme,
			"host":     parsedURL.Host,
			"hostname": parsedURL.Hostname(),
			"port":     parsedURL.Port(),
			"path":     parsedURL.Path,
			"query":    parsedURL.RawQuery,
			"fragment": parsedURL.Fragment,
			"href":     parsedURL.String(),
		}
	})

	// parseQuery: 查询参数解析
	vm.Set("parseQuery", func(query string) map[string]string {
		values, err := url.ParseQuery(query)
		if err != nil {
			log.Printf("[PluginManager] Query parse error: %v", err)
			return make(map[string]string)
		}

		// 转换为简单的 map[string]string (只取第一个值)
		result := make(map[string]string)
		for key, vals := range values {
			if len(vals) > 0 {
				result[key] = vals[0]
			}
		}
		return result
	})

	// formatTime: 时间格式化
	vm.Set("formatTime", func(timestamp int64, format string) string {
		t := time.UnixMilli(timestamp)
		// 简单格式化
		if format == "" {
			format = "2006-01-02 15:04:05"
		}
		return t.Format(format)
	})

	return nil
}

// createPluginContext 创建插件上下文 (onInit/onDestroy 用)
func (pm *PluginManager) createPluginContext(vm *goja.Runtime, plugin *Plugin) goja.Value {
	context := vm.NewObject()

	context.Set("pluginID", plugin.Metadata.ID)
	context.Set("config", plugin.Metadata.Config)

	// 暴露 state 对象（支持直接访问 context.state.xxx）
	context.Set("state", plugin.State)

	// log 函数
	context.Set("log", func(message string, level string) {
		if level == "" {
			level = "info"
		}
		log.Printf("[Plugin:%s] [%s] %s", plugin.Metadata.ID, level, message)
	})

	// setState / getState（保留向后兼容）
	context.Set("setState", func(key string, value interface{}) {
		plugin.State[key] = value
	})

	context.Set("getState", func(key string) interface{} {
		return plugin.State[key]
	})

	return context
}

// emittedEvents 收集 context.emit() 产生的派生事件
// 在 executePlugin 中使用，收集器生命周期与单次 onEvent 调用绑定
type emittedEvents struct {
	events []UnifiedEvent
}

// createEventContext 创建事件处理上下文 (onEvent 用)
// 返回 context goja.Value 和 emittedEvents 收集器
func (pm *PluginManager) createEventContext(vm *goja.Runtime, plugin *Plugin, event UnifiedEvent, sessionID string) (goja.Value, *emittedEvents) {
	collector := &emittedEvents{}

	context := pm.createPluginContext(vm, plugin)
	obj := context.ToObject(vm)

	obj.Set("deviceID", event.DeviceID)
	obj.Set("sessionID", sessionID)

	// log 函数
	obj.Set("log", func(message string, level ...string) {
		lvl := "info"
		if len(level) > 0 && level[0] != "" {
			lvl = level[0]
		}
		log.Printf("[Plugin:%s] [%s] %s", plugin.Metadata.ID, lvl, message)
	})

	// emit 辅助函数（简化派生事件生成）
	// 调用 emit 产生的事件会被收集，最终与 return derivedEvents 合并
	obj.Set("emit", func(eventType string, title string, data ...interface{}) {
		derived := UnifiedEvent{
			Source: SourcePlugin,
			Type:   eventType,
			Level:  LevelInfo,
			Title:  title,
		}
		if len(data) > 0 && data[0] != nil {
			dataBytes, err := json.Marshal(data[0])
			if err == nil {
				derived.Data = dataBytes
			}
		}
		collector.events = append(collector.events, derived)
	})

	// jsonPath 辅助函数
	obj.Set("jsonPath", func(obj interface{}, path string) interface{} {
		jsonBytes, err := json.Marshal(obj)
		if err != nil {
			return nil
		}
		result := gjson.GetBytes(jsonBytes, path)
		if !result.Exists() {
			return nil
		}
		return result.Value()
	})

	// matchURL 辅助函数
	obj.Set("matchURL", func(url, pattern string) bool {
		return matchURLPattern(pattern, url)
	})

	// setState / getState
	obj.Set("setState", func(key string, value interface{}) {
		plugin.State[key] = value
	})

	obj.Set("getState", func(key string) interface{} {
		return plugin.State[key]
	})

	return context, collector
}

// createEventContextWithLogging 创建带日志捕获的事件上下文（用于测试）
func (pm *PluginManager) createEventContextWithLogging(vm *goja.Runtime, plugin *Plugin, event UnifiedEvent, sessionID string, logs *[]string) (goja.Value, *emittedEvents) {
	// 复用生产环境的 context 创建逻辑
	context, collector := pm.createEventContext(vm, plugin, event, sessionID)
	obj := context.ToObject(vm)

	// 只覆盖 log 函数，添加日志捕获
	obj.Set("log", func(message string, level ...string) {
		lvl := "info"
		if len(level) > 0 && level[0] != "" {
			lvl = level[0]
		}
		logMsg := fmt.Sprintf("[%s] %s", lvl, message)
		*logs = append(*logs, logMsg)
		log.Printf("[Plugin:%s] %s", plugin.Metadata.ID, logMsg)
	})

	return context, collector
}

// createPluginErrorEvent 创建插件错误事件
func (pm *PluginManager) createPluginErrorEvent(pluginID, eventID string, err error) UnifiedEvent {
	return UnifiedEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now().UnixMilli(),
		Source:    SourceSystem,
		Category:  CategoryLog,
		Type:      "plugin_error",
		Level:     LevelError,
		Title:     fmt.Sprintf("插件执行错误: %s", pluginID),
		Summary:   err.Error(),
		Data: mustMarshal(map[string]interface{}{
			"pluginID":     pluginID,
			"error":        err.Error(),
			"triggerEvent": eventID,
		}),
	}
}

// ========== 管理方法 ==========

// GetPlugin 获取插件
func (pm *PluginManager) GetPlugin(id string) (*Plugin, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	plugin, exists := pm.plugins[id]
	if !exists {
		return nil, fmt.Errorf("插件不存在: %s", id)
	}

	return plugin, nil
}

// ListPlugins 列出所有插件
func (pm *PluginManager) ListPlugins() []*Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	plugins := make([]*Plugin, 0, len(pm.plugins))
	for _, plugin := range pm.plugins {
		plugins = append(plugins, plugin)
	}

	return plugins
}

// SavePlugin 保存插件
func (pm *PluginManager) SavePlugin(plugin *Plugin) error {
	// 保存到数据库
	if err := pm.store.SavePlugin(plugin); err != nil {
		return fmt.Errorf("保存插件到数据库失败: %w", err)
	}

	// 先尝试卸载旧插件（忽略不存在错误），避免无锁检查 pm.plugins 的 race condition
	_ = pm.UnloadPlugin(plugin.Metadata.ID)

	// 如果启用，则加载
	if plugin.Metadata.Enabled {
		if err := pm.LoadPlugin(plugin); err != nil {
			return fmt.Errorf("加载插件失败: %w", err)
		}
	}

	return nil
}

// DeletePlugin 删除插件
func (pm *PluginManager) DeletePlugin(id string) error {
	// 卸载插件
	if err := pm.UnloadPlugin(id); err != nil {
		log.Printf("[PluginManager] 卸载插件失败: %v", err)
	}

	// 从数据库删除
	if err := pm.store.DeletePlugin(id); err != nil {
		return fmt.Errorf("从数据库删除插件失败: %w", err)
	}

	return nil
}

// TogglePlugin 启用/禁用插件
func (pm *PluginManager) TogglePlugin(id string, enabled bool) error {
	plugin, err := pm.store.GetPlugin(id)
	if err != nil {
		return err
	}

	plugin.Metadata.Enabled = enabled

	if err := pm.store.SavePlugin(plugin); err != nil {
		return err
	}

	if enabled {
		// 加载插件
		return pm.LoadPlugin(plugin)
	} else {
		// 卸载插件
		return pm.UnloadPlugin(id)
	}
}
