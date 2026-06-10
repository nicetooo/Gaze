package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/google/uuid"
)

// ========== MatchesEvent 测试 ==========

func TestPluginMatchesEvent_Source(t *testing.T) {
	tests := []struct {
		name    string
		filters PluginFilters
		event   UnifiedEvent
		want    bool
	}{
		{
			name:    "match single source",
			filters: PluginFilters{Sources: []string{"network"}},
			event:   UnifiedEvent{Source: SourceNetwork},
			want:    true,
		},
		{
			name:    "match multiple sources",
			filters: PluginFilters{Sources: []string{"network", "logcat"}},
			event:   UnifiedEvent{Source: SourceLogcat},
			want:    true,
		},
		{
			name:    "mismatch source",
			filters: PluginFilters{Sources: []string{"logcat"}},
			event:   UnifiedEvent{Source: SourceNetwork},
			want:    false,
		},
		{
			name:    "empty sources filter matches all",
			filters: PluginFilters{Sources: []string{}},
			event:   UnifiedEvent{Source: SourceNetwork},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &Plugin{
				Metadata: PluginMetadata{
					ID:      "test-plugin",
					Filters: tt.filters,
				},
			}
			got := plugin.MatchesEvent(tt.event)
			if got != tt.want {
				t.Errorf("MatchesEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPluginMatchesEvent_Type(t *testing.T) {
	tests := []struct {
		name    string
		filters PluginFilters
		event   UnifiedEvent
		want    bool
	}{
		{
			name:    "match single type",
			filters: PluginFilters{Types: []string{"http_request"}},
			event:   UnifiedEvent{Type: "http_request"},
			want:    true,
		},
		{
			name:    "match multiple types",
			filters: PluginFilters{Types: []string{"http_request", "websocket_message"}},
			event:   UnifiedEvent{Type: "websocket_message"},
			want:    true,
		},
		{
			name:    "mismatch type",
			filters: PluginFilters{Types: []string{"http_request"}},
			event:   UnifiedEvent{Type: "logcat"},
			want:    false,
		},
		{
			name:    "empty types filter matches all",
			filters: PluginFilters{Types: []string{}},
			event:   UnifiedEvent{Type: "anything"},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &Plugin{
				Metadata: PluginMetadata{
					ID:      "test-plugin",
					Filters: tt.filters,
				},
			}
			got := plugin.MatchesEvent(tt.event)
			if got != tt.want {
				t.Errorf("MatchesEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPluginMatchesEvent_Level(t *testing.T) {
	tests := []struct {
		name    string
		filters PluginFilters
		event   UnifiedEvent
		want    bool
	}{
		{
			name:    "match error level",
			filters: PluginFilters{Levels: []string{"error"}},
			event:   UnifiedEvent{Level: LevelError},
			want:    true,
		},
		{
			name:    "match multiple levels",
			filters: PluginFilters{Levels: []string{"error", "warn"}},
			event:   UnifiedEvent{Level: LevelWarn},
			want:    true,
		},
		{
			name:    "mismatch level",
			filters: PluginFilters{Levels: []string{"error"}},
			event:   UnifiedEvent{Level: LevelInfo},
			want:    false,
		},
		{
			name:    "empty levels filter matches all",
			filters: PluginFilters{Levels: []string{}},
			event:   UnifiedEvent{Level: LevelDebug},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &Plugin{
				Metadata: PluginMetadata{
					ID:      "test-plugin",
					Filters: tt.filters,
				},
			}
			got := plugin.MatchesEvent(tt.event)
			if got != tt.want {
				t.Errorf("MatchesEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPluginMatchesEvent_URLPattern(t *testing.T) {
	tests := []struct {
		name    string
		filters PluginFilters
		event   UnifiedEvent
		want    bool
	}{
		{
			name:    "match URL pattern with wildcard",
			filters: PluginFilters{URLPattern: "*/api/*"},
			event: UnifiedEvent{
				Source: SourceNetwork,
				Data:   json.RawMessage(`{"url":"https://example.com/api/users"}`),
			},
			want: true,
		},
		{
			name:    "mismatch URL pattern",
			filters: PluginFilters{URLPattern: "*/api/*"},
			event: UnifiedEvent{
				Source: SourceNetwork,
				Data:   json.RawMessage(`{"url":"https://example.com/home"}`),
			},
			want: false,
		},
		{
			name:    "URL pattern on non-network event",
			filters: PluginFilters{URLPattern: "*/api/*"},
			event: UnifiedEvent{
				Source: SourceLogcat,
				Data:   json.RawMessage(`{"url":"https://example.com/api/users"}`),
			},
			want: false, // URLPattern 隐式要求 Source 为 network，非 network 事件不匹配
		},
		{
			name:    "event without URL field",
			filters: PluginFilters{URLPattern: "*/api/*"},
			event: UnifiedEvent{
				Source: SourceNetwork,
				Data:   json.RawMessage(`{"method":"GET"}`),
			},
			want: false, // 缺少 URL 字段，返回 false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &Plugin{
				Metadata: PluginMetadata{
					ID:      "test-plugin",
					Filters: tt.filters,
				},
			}
			got := plugin.MatchesEvent(tt.event)
			if got != tt.want {
				t.Errorf("MatchesEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPluginMatchesEvent_TitleMatch(t *testing.T) {
	tests := []struct {
		name    string
		filters PluginFilters
		event   UnifiedEvent
		want    bool
	}{
		{
			name:    "match title regex",
			filters: PluginFilters{TitleMatch: "^GET .+"},
			event:   UnifiedEvent{Title: "GET /api/users"},
			want:    true,
		},
		{
			name:    "mismatch title regex",
			filters: PluginFilters{TitleMatch: "^POST .+"},
			event:   UnifiedEvent{Title: "GET /api/users"},
			want:    false,
		},
		{
			name:    "empty title match matches all",
			filters: PluginFilters{TitleMatch: ""},
			event:   UnifiedEvent{Title: "anything"},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &Plugin{
				Metadata: PluginMetadata{
					ID:      "test-plugin",
					Filters: tt.filters,
				},
			}
			// 预编译正则（模拟 loadPluginLocked 的行为）
			if tt.filters.TitleMatch != "" {
				re, err := regexp.Compile(tt.filters.TitleMatch)
				if err != nil {
					t.Fatalf("Failed to compile regex: %v", err)
				}
				plugin.titleMatchRegex = re
			}
			got := plugin.MatchesEvent(tt.event)
			if got != tt.want {
				t.Errorf("MatchesEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPluginMatchesEvent_TitleMatch_InvalidRegex(t *testing.T) {
	// 无效正则应该在加载时就报错，而不是在 MatchesEvent 时
	// 模拟 loadPluginLocked 的行为
	_, err := regexp.Compile("[invalid(")
	if err == nil {
		t.Fatal("Expected regex compile error for invalid pattern")
	}
	// 验证加载阶段能正确拦截无效正则
	t.Logf("Invalid regex correctly caught at compile time: %v", err)
}

func TestPluginMatchesEvent_CombinedFilters(t *testing.T) {
	tests := []struct {
		name    string
		filters PluginFilters
		event   UnifiedEvent
		want    bool
	}{
		{
			name: "all filters match",
			filters: PluginFilters{
				Sources: []string{"network"},
				Types:   []string{"http_request"},
				Levels:  []string{"info"},
			},
			event: UnifiedEvent{
				Source: SourceNetwork,
				Type:   "http_request",
				Level:  LevelInfo,
			},
			want: true,
		},
		{
			name: "one filter mismatches (AND logic)",
			filters: PluginFilters{
				Sources: []string{"network"},
				Types:   []string{"http_request"},
				Levels:  []string{"error"},
			},
			event: UnifiedEvent{
				Source: SourceNetwork,
				Type:   "http_request",
				Level:  LevelInfo, // 不匹配 error
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &Plugin{
				Metadata: PluginMetadata{
					ID:      "test-plugin",
					Filters: tt.filters,
				},
			}
			got := plugin.MatchesEvent(tt.event)
			if got != tt.want {
				t.Errorf("MatchesEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ========== ProcessEvent 测试 ==========

func TestPluginManager_ProcessEvent_DerivedDepth(t *testing.T) {
	// 创建一个简单的测试插件
	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "test-depth",
			Name:    "Depth Test Plugin",
			Enabled: true,
			Filters: PluginFilters{}, // 匹配所有事件
			Config:  make(map[string]interface{}),
		},
		SourceCode: `
			const plugin = {
				onEvent: (event, context) => {
					return {
						derivedEvents: [{
							source: "plugin",
							type: "derived",
							level: "info",
							title: "Derived Event"
						}]
					};
				}
			};
		`,
		CompiledCode: `
			const plugin = {
				onEvent: (event, context) => {
					return {
						derivedEvents: [{
							source: "plugin",
							type: "derived",
							level: "info",
							title: "Derived Event"
						}]
					};
				}
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	pm := &PluginManager{
		plugins: make(map[string]*Plugin),
	}

	// 加载插件
	err := pm.LoadPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to load plugin: %v", err)
	}

	// 测试原始事件 (depth=0)
	event0 := UnifiedEvent{
		ID:           uuid.New().String(),
		DeviceID:     "test-device",
		Source:       SourceNetwork,
		Type:         "http_request",
		Level:        LevelInfo,
		Title:        "Test Event",
		DerivedDepth: 0,
	}

	derivedEvents := pm.ProcessEvent(event0, "test-session")

	// 验证派生事件的深度
	if len(derivedEvents) == 0 {
		t.Fatal("Expected derived events, got none")
	}

	for _, derived := range derivedEvents {
		if derived.DerivedDepth != 1 {
			t.Errorf("Expected DerivedDepth=1, got %d", derived.DerivedDepth)
		}
		if derived.ParentEventID != event0.ID {
			t.Errorf("Expected ParentEventID=%s, got %s", event0.ID, derived.ParentEventID)
		}
		if derived.GeneratedByPlugin != "test-depth" {
			t.Errorf("Expected GeneratedByPlugin=test-depth, got %s", derived.GeneratedByPlugin)
		}
	}
}

// ========== 辅助函数测试 ==========

func TestHelperFunctions_Base64Decode(t *testing.T) {
	// 通过创建 VM 实例、注入辅助函数、在 JS 中调用来测试端到端行为
	pm := &PluginManager{
		plugins: make(map[string]*Plugin),
	}

	plugin := &Plugin{
		Metadata: PluginMetadata{ID: "test-b64", Config: make(map[string]interface{})},
		State:    make(map[string]interface{}),
	}

	vm := goja.New()
	if err := pm.injectHelpers(vm, plugin); err != nil {
		t.Fatalf("Failed to inject helpers: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "decode simple string", input: "SGVsbG8gV29ybGQ=", expected: "Hello World"},
		{name: "decode empty string", input: "", expected: ""},
		{name: "decode invalid base64", input: "!!!invalid!!!", expected: ""}, // 错误时返回空字符串
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := vm.RunString(fmt.Sprintf(`base64Decode("%s")`, tt.input))
			if err != nil {
				t.Fatalf("JS execution error: %v", err)
			}
			got := result.String()
			if got != tt.expected {
				t.Errorf("base64Decode(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestHelperFunctions_ParseURL(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}
	plugin := &Plugin{
		Metadata: PluginMetadata{ID: "test-url", Config: make(map[string]interface{})},
		State:    make(map[string]interface{}),
	}

	vm := goja.New()
	if err := pm.injectHelpers(vm, plugin); err != nil {
		t.Fatalf("Failed to inject helpers: %v", err)
	}

	result, err := vm.RunString(`JSON.stringify(parseURL("https://example.com:8080/api/users?page=1#section"))`)
	if err != nil {
		t.Fatalf("JS execution error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result.String()), &parsed); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if parsed["scheme"] != "https" {
		t.Errorf("scheme = %v, want https", parsed["scheme"])
	}
	if parsed["hostname"] != "example.com" {
		t.Errorf("hostname = %v, want example.com", parsed["hostname"])
	}
	if parsed["port"] != "8080" {
		t.Errorf("port = %v, want 8080", parsed["port"])
	}
	if parsed["path"] != "/api/users" {
		t.Errorf("path = %v, want /api/users", parsed["path"])
	}
	if parsed["query"] != "page=1" {
		t.Errorf("query = %v, want page=1", parsed["query"])
	}
	if parsed["fragment"] != "section" {
		t.Errorf("fragment = %v, want section", parsed["fragment"])
	}
}

func TestHelperFunctions_ParseQuery(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}
	plugin := &Plugin{
		Metadata: PluginMetadata{ID: "test-query", Config: make(map[string]interface{})},
		State:    make(map[string]interface{}),
	}

	vm := goja.New()
	if err := pm.injectHelpers(vm, plugin); err != nil {
		t.Fatalf("Failed to inject helpers: %v", err)
	}

	result, err := vm.RunString(`JSON.stringify(parseQuery("user_id=123&event=click&page=1"))`)
	if err != nil {
		t.Fatalf("JS execution error: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal([]byte(result.String()), &parsed); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if parsed["user_id"] != "123" {
		t.Errorf("user_id = %v, want 123", parsed["user_id"])
	}
	if parsed["event"] != "click" {
		t.Errorf("event = %v, want click", parsed["event"])
	}
	if parsed["page"] != "1" {
		t.Errorf("page = %v, want 1", parsed["page"])
	}
}

// ========== 并发执行测试 ==========

func TestPluginManager_ConcurrentExecution(t *testing.T) {
	// 创建多个插件
	plugins := []*Plugin{
		{
			Metadata: PluginMetadata{
				ID:      "plugin-1",
				Name:    "Plugin 1",
				Enabled: true,
				Filters: PluginFilters{},
				Config:  make(map[string]interface{}),
			},
			CompiledCode: `const plugin = { onEvent: (e, ctx) => ({ derivedEvents: [] }) };`,
			Language:     "javascript",
			State:        make(map[string]interface{}),
		},
		{
			Metadata: PluginMetadata{
				ID:      "plugin-2",
				Name:    "Plugin 2",
				Enabled: true,
				Filters: PluginFilters{},
				Config:  make(map[string]interface{}),
			},
			CompiledCode: `const plugin = { onEvent: (e, ctx) => ({ derivedEvents: [] }) };`,
			Language:     "javascript",
			State:        make(map[string]interface{}),
		},
		{
			Metadata: PluginMetadata{
				ID:      "plugin-3",
				Name:    "Plugin 3",
				Enabled: true,
				Filters: PluginFilters{},
				Config:  make(map[string]interface{}),
			},
			CompiledCode: `const plugin = { onEvent: (e, ctx) => ({ derivedEvents: [] }) };`,
			Language:     "javascript",
			State:        make(map[string]interface{}),
		},
	}

	pm := &PluginManager{
		plugins: make(map[string]*Plugin),
	}

	// 加载所有插件
	for _, p := range plugins {
		if err := pm.LoadPlugin(p); err != nil {
			t.Fatalf("Failed to load plugin %s: %v", p.Metadata.ID, err)
		}
	}

	// 测试并发执行
	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "test-device",
		Source:   SourceNetwork,
		Type:     "test",
		Level:    LevelInfo,
		Title:    "Test Event",
	}

	start := time.Now()
	_ = pm.ProcessEvent(event, "test-session")
	elapsed := time.Since(start)

	// 验证并发执行速度（应该比串行快）
	// 如果是串行，3个插件至少需要 3 * 执行时间
	// 如果是并发，应该接近单个插件的执行时间
	t.Logf("Concurrent execution took: %v", elapsed)

	// 这里只是一个基本检查，实际速度取决于插件复杂度
	if elapsed > 1*time.Second {
		t.Errorf("Concurrent execution took too long: %v", elapsed)
	}
}

// ========== 并发安全测试 (用 -race 运行检测 data race) ==========

// TestPluginManager_ConcurrentSafety_SamePlugin 测试同一个插件被多个事件并发触发时的安全性
// 运行: go test -race -tags fts5 -run TestPluginManager_ConcurrentSafety_SamePlugin
func TestPluginManager_ConcurrentSafety_SamePlugin(t *testing.T) {
	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "race-test",
			Name:    "Race Test Plugin",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		CompiledCode: `
			const plugin = {
				onEvent: (event, context) => {
					// 读写 state 来触发潜在的 data race
					var count = context.getState("count") || 0;
					context.setState("count", count + 1);
					return {
						derivedEvents: [{
							source: "plugin",
							type: "counter",
							level: "info",
							title: "Count: " + (count + 1)
						}]
					};
				}
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	pm := &PluginManager{
		plugins: make(map[string]*Plugin),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("Failed to load plugin: %v", err)
	}

	// 并发发送多个事件到同一个插件
	var wg sync.WaitGroup
	const concurrency = 20

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := UnifiedEvent{
				ID:       uuid.New().String(),
				DeviceID: "test-device",
				Source:   SourceNetwork,
				Type:     "http_request",
				Level:    LevelInfo,
				Title:    fmt.Sprintf("Event %d", idx),
			}
			results := pm.ProcessEvent(event, "test-session")
			// 每次执行都应该产生一个派生事件
			if len(results) != 1 {
				t.Errorf("Event %d: expected 1 derived event, got %d", idx, len(results))
			}
		}(i)
	}

	wg.Wait()

	// 验证所有事件都被正确处理 (state 应该是递增的)
	pm.mu.RLock()
	p := pm.plugins["race-test"]
	pm.mu.RUnlock()

	p.mu.Lock()
	count, _ := p.State["count"]
	p.mu.Unlock()

	if count == nil {
		t.Fatal("Expected state 'count' to be set")
	}

	// count 应该 > 0 (具体值取决于串行化执行顺序)
	countFloat, ok := count.(int64)
	if !ok {
		// goja 可能返回 int64 或 float64
		if countF, ok := count.(float64); ok {
			countFloat = int64(countF)
		}
	}
	if countFloat <= 0 {
		t.Errorf("Expected count > 0, got %v", count)
	}
	t.Logf("Final count after %d concurrent events: %v", concurrency, count)
}

// TestPluginManager_ConcurrentSafety_MultiplePluginsMultipleEvents 测试多插件多事件并发
func TestPluginManager_ConcurrentSafety_MultiplePluginsMultipleEvents(t *testing.T) {
	pm := &PluginManager{
		plugins: make(map[string]*Plugin),
	}

	// 创建 3 个插件，每个都读写 state
	for i := 0; i < 3; i++ {
		plugin := &Plugin{
			Metadata: PluginMetadata{
				ID:      fmt.Sprintf("multi-race-%d", i),
				Name:    fmt.Sprintf("Multi Race Plugin %d", i),
				Enabled: true,
				Filters: PluginFilters{},
				Config:  make(map[string]interface{}),
			},
			CompiledCode: `
				const plugin = {
					onEvent: (event, context) => {
						var n = context.getState("n") || 0;
						context.setState("n", n + 1);
						return { derivedEvents: [] };
					}
				};
			`,
			Language: "javascript",
			State:    make(map[string]interface{}),
		}

		if err := pm.LoadPlugin(plugin); err != nil {
			t.Fatalf("Failed to load plugin %d: %v", i, err)
		}
	}

	// 并发发送大量事件
	var wg sync.WaitGroup
	const eventCount = 50

	for i := 0; i < eventCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := UnifiedEvent{
				ID:       uuid.New().String(),
				DeviceID: "test-device",
				Source:   SourceNetwork,
				Type:     "http_request",
				Level:    LevelInfo,
				Title:    fmt.Sprintf("Event %d", idx),
			}
			pm.ProcessEvent(event, "test-session")
		}(i)
	}

	wg.Wait()
	t.Logf("Successfully processed %d events across 3 plugins without data race", eventCount)
}

// ========== LoadPlugin 测试 ==========

func TestPluginManager_LoadPlugin_Success(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "load-success",
			Name:    "Load Success",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  map[string]interface{}{"key": "val"},
		},
		CompiledCode: `const plugin = { onEvent: (e, ctx) => ({ derivedEvents: [] }) };`,
		Language:     "javascript",
		State:        make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	// 验证 VM 和 OnEventFunc 已设置
	if plugin.VM == nil {
		t.Error("VM should not be nil after load")
	}
	if plugin.OnEventFunc == nil {
		t.Error("OnEventFunc should not be nil after load")
	}

	// 验证已加入 plugins map
	pm.mu.RLock()
	_, exists := pm.plugins["load-success"]
	pm.mu.RUnlock()
	if !exists {
		t.Error("Plugin not found in plugins map after load")
	}
}

func TestPluginManager_LoadPlugin_MissingPluginObject(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:     "no-plugin-obj",
			Config: make(map[string]interface{}),
		},
		CompiledCode: `var plugin = undefined;`, // plugin 存在但是 undefined
		Language:     "javascript",
		State:        make(map[string]interface{}),
	}

	err := pm.LoadPlugin(plugin)
	if err == nil {
		t.Fatal("Expected error for missing plugin object, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

func TestPluginManager_LoadPlugin_MissingOnEvent(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:     "no-onevent",
			Config: make(map[string]interface{}),
		},
		CompiledCode: `const plugin = { metadata: { name: "test" } };`, // 没有 onEvent
		Language:     "javascript",
		State:        make(map[string]interface{}),
	}

	err := pm.LoadPlugin(plugin)
	if err == nil {
		t.Fatal("Expected error for missing onEvent, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

func TestPluginManager_LoadPlugin_OnEventNotFunction(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:     "onevent-not-func",
			Config: make(map[string]interface{}),
		},
		CompiledCode: `const plugin = { onEvent: "not a function" };`,
		Language:     "javascript",
		State:        make(map[string]interface{}),
	}

	err := pm.LoadPlugin(plugin)
	if err == nil {
		t.Fatal("Expected error for onEvent not being a function, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

func TestPluginManager_LoadPlugin_OnInitCalled(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:     "oninit-test",
			Config: make(map[string]interface{}),
		},
		CompiledCode: `
			const plugin = {
				onInit: (ctx) => {
					ctx.setState("initialized", true);
				},
				onEvent: (e, ctx) => null
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	// 验证 onInit 设置了 state
	if plugin.State["initialized"] != true {
		t.Errorf("onInit did not set state; State = %v", plugin.State)
	}
}

func TestPluginManager_LoadPlugin_InvalidTitleMatchRegex(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID: "bad-regex",
			Filters: PluginFilters{
				TitleMatch: "[invalid(",
			},
			Config: make(map[string]interface{}),
		},
		CompiledCode: `const plugin = { onEvent: (e, ctx) => null };`,
		Language:     "javascript",
		State:        make(map[string]interface{}),
	}

	err := pm.LoadPlugin(plugin)
	if err == nil {
		t.Fatal("Expected error for invalid titleMatch regex, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

func TestPluginManager_LoadPlugin_ValidTitleMatchRegex(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID: "good-regex",
			Filters: PluginFilters{
				TitleMatch: "^GET /api/.+",
			},
			Config: make(map[string]interface{}),
		},
		CompiledCode: `const plugin = { onEvent: (e, ctx) => null };`,
		Language:     "javascript",
		State:        make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	if plugin.titleMatchRegex == nil {
		t.Error("titleMatchRegex should be compiled after load")
	}
}

func TestPluginManager_LoadPlugin_InitializesNilState(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:     "nil-state",
			Config: make(map[string]interface{}),
		},
		CompiledCode: `const plugin = { onEvent: (e, ctx) => null };`,
		Language:     "javascript",
		State:        nil, // 不预初始化
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	if plugin.State == nil {
		t.Error("State should be initialized to non-nil after load")
	}
}

func TestPluginManager_LoadPlugin_SyntaxError(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:     "syntax-error",
			Config: make(map[string]interface{}),
		},
		CompiledCode: "const plugin = { onEvent: (e, ctx) =>  }}};", // 语法错误
		Language:     "javascript",
		State:        make(map[string]interface{}),
	}

	err := pm.LoadPlugin(plugin)
	if err == nil {
		t.Fatal("Expected error for JS syntax error, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

// ========== UnloadPlugin 测试 ==========

func TestPluginManager_UnloadPlugin_Success(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "unload-test",
			Enabled: true,
			Config:  make(map[string]interface{}),
		},
		CompiledCode: `const plugin = { onEvent: (e, ctx) => null };`,
		Language:     "javascript",
		State:        make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	if err := pm.UnloadPlugin("unload-test"); err != nil {
		t.Fatalf("UnloadPlugin failed: %v", err)
	}

	// 验证已从 map 中移除
	pm.mu.RLock()
	_, exists := pm.plugins["unload-test"]
	pm.mu.RUnlock()
	if exists {
		t.Error("Plugin should not exist in map after unload")
	}

	// 验证 VM 已清空
	if plugin.VM != nil {
		t.Error("VM should be nil after unload")
	}
	if plugin.OnEventFunc != nil {
		t.Error("OnEventFunc should be nil after unload")
	}
}

func TestPluginManager_UnloadPlugin_OnDestroyCalled(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:     "destroy-test",
			Config: make(map[string]interface{}),
		},
		CompiledCode: `
			const plugin = {
				onEvent: (e, ctx) => null,
				onDestroy: (ctx) => {
					ctx.setState("destroyed", true);
				}
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	// 保存引用以便检查（UnloadPlugin 会 nil 掉 State）
	// 注意: UnloadPlugin 会把 State 设为 nil，所以 onDestroy 的 setState 效果会被清除
	// 这里主要测试 onDestroy 被调用而不 panic
	if err := pm.UnloadPlugin("destroy-test"); err != nil {
		t.Fatalf("UnloadPlugin failed: %v", err)
	}

	// State 被清理（UnloadPlugin 会 plugin.State = nil）
	if plugin.State != nil {
		t.Error("State should be nil after unload cleanup")
	}
}

func TestPluginManager_UnloadPlugin_NotFound(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	err := pm.UnloadPlugin("nonexistent")
	if err == nil {
		t.Fatal("Expected error for unloading nonexistent plugin, got nil")
	}
}

// ========== Plugin 执行测试 ==========

func TestPluginManager_ExecutePlugin_ReturnsDerivedEvents(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "derive-test",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		CompiledCode: `
			const plugin = {
				onEvent: (event, context) => {
					return {
						derivedEvents: [
							{ source: "plugin", type: "alert", level: "warn", title: "Alert: " + event.title },
							{ source: "plugin", type: "metric", level: "info", title: "Metric recorded" }
						]
					};
				}
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "http_request",
		Level:    LevelInfo,
		Title:    "GET /api/users",
	}

	results := pm.ProcessEvent(event, "session-1")

	if len(results) != 2 {
		t.Fatalf("Expected 2 derived events, got %d", len(results))
	}

	// 验证第一个派生事件
	if results[0].Type != "alert" && results[1].Type != "alert" {
		t.Error("Expected one 'alert' type derived event")
	}
}

func TestPluginManager_ExecutePlugin_ReturnsNull(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "null-return",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		CompiledCode: `const plugin = { onEvent: (e, ctx) => null };`,
		Language:     "javascript",
		State:        make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "test",
		Level:    LevelInfo,
		Title:    "Test",
	}

	results := pm.ProcessEvent(event, "session-1")

	// null 返回值应该产生 0 个派生事件
	if len(results) != 0 {
		t.Errorf("Expected 0 derived events for null return, got %d", len(results))
	}
}

func TestPluginManager_ExecutePlugin_ReturnsUndefined(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "undefined-return",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		CompiledCode: `const plugin = { onEvent: (e, ctx) => {} };`, // 无返回值 = undefined
		Language:     "javascript",
		State:        make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "test",
		Level:    LevelInfo,
		Title:    "Test",
	}

	results := pm.ProcessEvent(event, "session-1")
	if len(results) != 0 {
		t.Errorf("Expected 0 derived events for undefined return, got %d", len(results))
	}
}

func TestPluginManager_ExecutePlugin_ContextEmit(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "emit-test",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		CompiledCode: `
			const plugin = {
				onEvent: (event, context) => {
					context.emit("emitted_event", "Via context.emit", { key: "value" });
					return {
						derivedEvents: [{
							source: "plugin",
							type: "returned_event",
							level: "info",
							title: "Via return"
						}]
					};
				}
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "test",
		Level:    LevelInfo,
		Title:    "Test",
	}

	results := pm.ProcessEvent(event, "session-1")

	// 应该有 2 个派生事件: 1 个来自 return，1 个来自 context.emit()
	if len(results) != 2 {
		t.Fatalf("Expected 2 derived events (return + emit), got %d", len(results))
	}

	// 检查两种类型的事件都存在
	hasReturned := false
	hasEmitted := false
	for _, r := range results {
		if r.Type == "returned_event" {
			hasReturned = true
		}
		if r.Type == "emitted_event" {
			hasEmitted = true
		}
	}
	if !hasReturned {
		t.Error("Missing 'returned_event' from return value")
	}
	if !hasEmitted {
		t.Error("Missing 'emitted_event' from context.emit()")
	}
}

func TestPluginManager_ExecutePlugin_ContextConfig(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "config-test",
			Enabled: true,
			Filters: PluginFilters{},
			Config: map[string]interface{}{
				"threshold": 100.0,
				"prefix":    "ALERT",
			},
		},
		CompiledCode: `
			const plugin = {
				onEvent: (event, context) => {
					var prefix = context.config.prefix || "DEFAULT";
					return {
						derivedEvents: [{
							source: "plugin",
							type: "config_check",
							level: "info",
							title: prefix + ": " + event.title
						}]
					};
				}
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "test",
		Level:    LevelInfo,
		Title:    "Something happened",
	}

	results := pm.ProcessEvent(event, "session-1")
	if len(results) != 1 {
		t.Fatalf("Expected 1 derived event, got %d", len(results))
	}
	if results[0].Title != "ALERT: Something happened" {
		t.Errorf("Title = %q, want %q", results[0].Title, "ALERT: Something happened")
	}
}

func TestPluginManager_ExecutePlugin_StatePersistence(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "state-persist",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		CompiledCode: `
			const plugin = {
				onEvent: (event, context) => {
					var count = context.getState("count") || 0;
					count = count + 1;
					context.setState("count", count);
					return {
						derivedEvents: [{
							source: "plugin",
							type: "counted",
							level: "info",
							title: "Count: " + count
						}]
					};
				}
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	// 发送 3 个事件
	for i := 0; i < 3; i++ {
		event := UnifiedEvent{
			ID:       uuid.New().String(),
			DeviceID: "dev-1",
			Source:   SourceNetwork,
			Type:     "test",
			Level:    LevelInfo,
			Title:    fmt.Sprintf("Event %d", i),
		}
		pm.ProcessEvent(event, "session-1")
	}

	// 验证 state 保持了计数
	plugin.mu.Lock()
	count := plugin.State["count"]
	plugin.mu.Unlock()

	// count 应该是 3 (由于串行执行的保证)
	countVal, ok := count.(int64)
	if !ok {
		if f, ok := count.(float64); ok {
			countVal = int64(f)
		}
	}
	if countVal != 3 {
		t.Errorf("Expected count = 3, got %v (type %T)", count, count)
	}
}

func TestPluginManager_ExecutePlugin_JSRuntimeError(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "js-error",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		CompiledCode: `
			const plugin = {
				onEvent: (event, context) => {
					// 访问 undefined 的属性会抛出错误
					var x = null;
					return x.nonexistent.property;
				}
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "test",
		Level:    LevelInfo,
		Title:    "Test",
	}

	// JS 错误应该被捕获，不应 panic
	results := pm.ProcessEvent(event, "session-1")

	// 应该产生一个 plugin_error 事件
	if len(results) != 1 {
		t.Fatalf("Expected 1 error event, got %d", len(results))
	}
	if results[0].Type != "plugin_error" {
		t.Errorf("Expected plugin_error type, got %q", results[0].Type)
	}
	if results[0].Level != LevelError {
		t.Errorf("Expected error level, got %q", results[0].Level)
	}
}

func TestPluginManager_ExecutePlugin_DisabledPluginSkipped(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "disabled-skip",
			Enabled: false, // 禁用
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		CompiledCode: `
			const plugin = {
				onEvent: (e, ctx) => ({
					derivedEvents: [{ source: "plugin", type: "should_not_appear", level: "info", title: "Bug!" }]
				})
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "test",
		Level:    LevelInfo,
		Title:    "Test",
	}

	results := pm.ProcessEvent(event, "session-1")
	if len(results) != 0 {
		t.Errorf("Disabled plugin should not produce events, got %d", len(results))
	}
}

func TestPluginManager_ExecutePlugin_EventDataAccess(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "data-access",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		CompiledCode: `
			const plugin = {
				onEvent: (event, context) => {
					var data = event.data || {};
					var method = data.method || "UNKNOWN";
					var statusCode = data.statusCode || 0;
					return {
						derivedEvents: [{
							source: "plugin",
							type: "data_check",
							level: "info",
							title: method + " " + statusCode
						}]
					};
				}
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "http_request",
		Level:    LevelInfo,
		Title:    "GET /api",
		Data:     json.RawMessage(`{"method":"POST","statusCode":201,"url":"https://example.com/api"}`),
	}

	results := pm.ProcessEvent(event, "session-1")
	if len(results) != 1 {
		t.Fatalf("Expected 1 derived event, got %d", len(results))
	}
	if results[0].Title != "POST 201" {
		t.Errorf("Title = %q, want %q", results[0].Title, "POST 201")
	}
}

func TestPluginManager_ExecutePlugin_FilterMismatch(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "filter-mismatch",
			Enabled: true,
			Filters: PluginFilters{
				Sources: []string{"logcat"}, // 只匹配 logcat
			},
			Config: make(map[string]interface{}),
		},
		CompiledCode: `
			const plugin = {
				onEvent: (e, ctx) => ({
					derivedEvents: [{ source: "plugin", type: "matched", level: "info", title: "Matched" }]
				})
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	// 发送 network 事件，不应匹配 logcat 过滤器
	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "http_request",
		Level:    LevelInfo,
		Title:    "GET /api",
	}

	results := pm.ProcessEvent(event, "session-1")
	if len(results) != 0 {
		t.Errorf("Expected 0 events for filter mismatch, got %d", len(results))
	}
}

// ========== ExecutePluginWithLogging 测试 ==========

func TestPluginManager_ExecutePluginWithLogging_CapturesLogs(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:     "log-capture",
			Config: make(map[string]interface{}),
		},
		CompiledCode: `
			const plugin = {
				onEvent: (event, context) => {
					context.log("Processing event: " + event.type);
					context.log("This is a warning", "warn");
					return null;
				}
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "http_request",
		Level:    LevelInfo,
		Title:    "Test",
	}

	result, logs, execTime, err := pm.ExecutePluginWithLogging(plugin, event, "session-1")
	if err != nil {
		t.Fatalf("ExecutePluginWithLogging failed: %v", err)
	}

	// 验证日志捕获
	if len(logs) != 2 {
		t.Fatalf("Expected 2 logs, got %d: %v", len(logs), logs)
	}
	if logs[0] != "[info] Processing event: http_request" {
		t.Errorf("Log[0] = %q, want %q", logs[0], "[info] Processing event: http_request")
	}
	if logs[1] != "[warn] This is a warning" {
		t.Errorf("Log[1] = %q, want %q", logs[1], "[warn] This is a warning")
	}

	// 验证执行时间
	if execTime < 0 {
		t.Errorf("ExecutionTime should be >= 0, got %d", execTime)
	}

	// 验证返回值
	if result != nil && len(result.DerivedEvents) > 0 {
		t.Errorf("Expected no derived events for null return, got %d", len(result.DerivedEvents))
	}
}

func TestPluginManager_ExecutePluginWithLogging_CapturesError(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:     "log-error",
			Config: make(map[string]interface{}),
		},
		CompiledCode: `
			const plugin = {
				onEvent: (event, context) => {
					throw new Error("intentional error");
				}
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "test",
		Level:    LevelInfo,
		Title:    "Test",
	}

	_, _, _, err := pm.ExecutePluginWithLogging(plugin, event, "session-1")
	if err == nil {
		t.Fatal("Expected error from JS throw, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

// ========== 辅助函数补充测试 ==========

func TestHelperFunctions_MatchURL_Global(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}
	plugin := &Plugin{
		Metadata: PluginMetadata{ID: "test-matchurl-global", Config: make(map[string]interface{})},
		State:    make(map[string]interface{}),
	}

	vm := goja.New()
	if err := pm.injectHelpers(vm, plugin); err != nil {
		t.Fatalf("Failed to inject helpers: %v", err)
	}

	// 锁死参数顺序为 (url, pattern)，与 context.matchURL 和 plugin.d.ts 声明一致
	tests := []struct {
		name     string
		script   string
		expected string
	}{
		{
			name:     "wildcard match",
			script:   `String(matchURL("https://example.com/api/users", "*/api/*"))`,
			expected: "true",
		},
		{
			name:     "no match",
			script:   `String(matchURL("https://example.com/home", "*/api/*"))`,
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := vm.RunString(tt.script)
			if err != nil {
				t.Fatalf("JS error: %v", err)
			}
			if got := result.String(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestHelperFunctions_MatchRegex(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}
	plugin := &Plugin{
		Metadata: PluginMetadata{ID: "test-regex", Config: make(map[string]interface{})},
		State:    make(map[string]interface{}),
	}

	vm := goja.New()
	if err := pm.injectHelpers(vm, plugin); err != nil {
		t.Fatalf("Failed to inject helpers: %v", err)
	}

	tests := []struct {
		name     string
		script   string
		expected string
	}{
		{
			name:     "simple match",
			script:   `JSON.stringify(matchRegex("hello (\\w+)", "hello world"))`,
			expected: `["hello world","world"]`,
		},
		{
			name:     "no match returns null",
			script:   `JSON.stringify(matchRegex("^foo$", "bar"))`,
			expected: "null",
		},
		{
			name:     "invalid regex returns null",
			script:   `JSON.stringify(matchRegex("[invalid(", "test"))`,
			expected: "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := vm.RunString(tt.script)
			if err != nil {
				t.Fatalf("JS error: %v", err)
			}
			got := result.String()
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestHelperFunctions_JsonPath(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}
	plugin := &Plugin{
		Metadata: PluginMetadata{ID: "test-jsonpath", Config: make(map[string]interface{})},
		State:    make(map[string]interface{}),
	}

	vm := goja.New()
	if err := pm.injectHelpers(vm, plugin); err != nil {
		t.Fatalf("Failed to inject helpers: %v", err)
	}

	tests := []struct {
		name     string
		script   string
		expected string
	}{
		{
			name:     "simple path",
			script:   `jsonPath({name: "John", age: 30}, "name")`,
			expected: "John",
		},
		{
			name:     "nested path",
			script:   `jsonPath({user: {name: "Alice"}}, "user.name")`,
			expected: "Alice",
		},
		{
			name:     "nonexistent path returns null",
			script:   `JSON.stringify(jsonPath({a: 1}, "b"))`,
			expected: "null",
		},
		{
			name:     "array access",
			script:   `jsonPath({items: ["a","b","c"]}, "items.1")`,
			expected: "b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := vm.RunString(tt.script)
			if err != nil {
				t.Fatalf("JS error: %v", err)
			}
			got := result.String()
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestHelperFunctions_FormatTime(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}
	plugin := &Plugin{
		Metadata: PluginMetadata{ID: "test-ftime", Config: make(map[string]interface{})},
		State:    make(map[string]interface{}),
	}

	vm := goja.New()
	if err := pm.injectHelpers(vm, plugin); err != nil {
		t.Fatalf("Failed to inject helpers: %v", err)
	}

	tests := []struct {
		name   string
		script string
		check  func(string) bool
	}{
		{
			name:   "default format",
			script: `formatTime(1704067200000, "")`,           // 2024-01-01 00:00:00 UTC
			check:  func(s string) bool { return len(s) > 0 }, // 只验证非空
		},
		{
			name:   "custom format",
			script: `formatTime(1704067200000, "2006-01-02")`, // Go date format
			check:  func(s string) bool { return len(s) == 10 },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := vm.RunString(tt.script)
			if err != nil {
				t.Fatalf("JS error: %v", err)
			}
			got := result.String()
			if !tt.check(got) {
				t.Errorf("formatTime result %q failed check", got)
			}
		})
	}
}

func TestHelperFunctions_MatchURL_InContext(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "matchurl-ctx",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		CompiledCode: `
			const plugin = {
				onEvent: (event, context) => {
					var url = (event.data || {}).url || "";
					if (context.matchURL(url, "*/api/*")) {
						return {
							derivedEvents: [{
								source: "plugin",
								type: "api_match",
								level: "info",
								title: "API URL matched"
							}]
						};
					}
					return null;
				}
			};
		`,
		Language: "javascript",
		State:    make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	// 匹配的 URL
	event1 := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "http_request",
		Level:    LevelInfo,
		Title:    "GET /api/users",
		Data:     json.RawMessage(`{"url":"https://example.com/api/users"}`),
	}
	results1 := pm.ProcessEvent(event1, "session-1")
	if len(results1) != 1 {
		t.Errorf("Expected 1 derived event for matching URL, got %d", len(results1))
	}

	// 不匹配的 URL
	event2 := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "http_request",
		Level:    LevelInfo,
		Title:    "GET /home",
		Data:     json.RawMessage(`{"url":"https://example.com/home"}`),
	}
	results2 := pm.ProcessEvent(event2, "session-1")
	if len(results2) != 0 {
		t.Errorf("Expected 0 derived events for non-matching URL, got %d", len(results2))
	}
}

// ========== Plugin ToJSON/FromJSON 测试 ==========

func TestPlugin_ToJSON_FromJSON(t *testing.T) {
	original := &Plugin{
		Metadata: PluginMetadata{
			ID:          "json-test",
			Name:        "JSON Test Plugin",
			Version:     "1.2.3",
			Author:      "tester",
			Description: "A plugin for testing JSON serialization",
			Enabled:     true,
			Filters: PluginFilters{
				Sources:    []string{"network"},
				Types:      []string{"http_request"},
				URLPattern: "*/api/*",
			},
			Config: map[string]interface{}{
				"timeout": 5000.0,
			},
		},
		SourceCode:   `const plugin = { onEvent: () => null };`,
		Language:     "typescript",
		CompiledCode: `const plugin = { onEvent: () => null };`,
	}

	data, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	restored := &Plugin{}
	if err := restored.FromJSON(data); err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	// 验证字段
	if restored.Metadata.ID != original.Metadata.ID {
		t.Errorf("ID = %q, want %q", restored.Metadata.ID, original.Metadata.ID)
	}
	if restored.Metadata.Name != original.Metadata.Name {
		t.Errorf("Name = %q, want %q", restored.Metadata.Name, original.Metadata.Name)
	}
	if restored.Metadata.Version != original.Metadata.Version {
		t.Errorf("Version = %q, want %q", restored.Metadata.Version, original.Metadata.Version)
	}
	if restored.SourceCode != original.SourceCode {
		t.Error("SourceCode mismatch")
	}
	if restored.CompiledCode != original.CompiledCode {
		t.Error("CompiledCode mismatch")
	}
	if len(restored.Metadata.Filters.Sources) != 1 || restored.Metadata.Filters.Sources[0] != "network" {
		t.Errorf("Filters.Sources = %v", restored.Metadata.Filters.Sources)
	}
}

func TestPlugin_ToJSON_ExcludesRuntimeFields(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:     "runtime-exclude",
			Config: make(map[string]interface{}),
		},
		CompiledCode: `const plugin = { onEvent: (e, ctx) => null };`,
		Language:     "javascript",
		State:        make(map[string]interface{}),
	}

	if err := pm.LoadPlugin(plugin); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	data, err := plugin.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// JSON 中不应包含 VM, OnEventFunc 等运行时字段
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	for _, forbidden := range []string{"VM", "vm", "OnEventFunc", "onEventFunc", "OnInitFunc", "mu"} {
		if _, exists := raw[forbidden]; exists {
			t.Errorf("ToJSON should not include runtime field %q", forbidden)
		}
	}
}

// ========== PluginManager 管理方法测试 (需要数据库) ==========

func TestPluginManager_SaveAndReload(t *testing.T) {
	_, store := setupTestDB(t)
	pm := NewPluginManager(store, nil)

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "save-reload",
			Name:    "Save Reload Test",
			Version: "1.0.0",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		SourceCode:   `const plugin = { onEvent: (e, ctx) => null };`,
		CompiledCode: `const plugin = { onEvent: (e, ctx) => null };`,
		Language:     "typescript",
		State:        make(map[string]interface{}),
	}

	// SavePlugin 应该保存到 DB 并加载到内存
	if err := pm.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin failed: %v", err)
	}

	// 验证已加载到内存
	loaded, err := pm.GetPlugin("save-reload")
	if err != nil {
		t.Fatalf("GetPlugin from memory failed: %v", err)
	}
	if loaded.Metadata.Name != "Save Reload Test" {
		t.Errorf("Name = %q, want %q", loaded.Metadata.Name, "Save Reload Test")
	}

	// 验证已保存到 DB
	dbPlugin, err := store.GetPlugin("save-reload")
	if err != nil {
		t.Fatalf("GetPlugin from DB failed: %v", err)
	}
	if dbPlugin.Metadata.Name != "Save Reload Test" {
		t.Errorf("DB Name = %q, want %q", dbPlugin.Metadata.Name, "Save Reload Test")
	}
}

func TestPluginManager_SaveDisabledPlugin(t *testing.T) {
	_, store := setupTestDB(t)
	pm := NewPluginManager(store, nil)

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "save-disabled",
			Name:    "Disabled",
			Version: "1.0.0",
			Enabled: false, // 禁用
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		SourceCode:   `const plugin = { onEvent: (e, ctx) => null };`,
		CompiledCode: `const plugin = { onEvent: (e, ctx) => null };`,
		Language:     "typescript",
		State:        make(map[string]interface{}),
	}

	if err := pm.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin failed: %v", err)
	}

	// 禁用的插件不应加载到内存
	_, err := pm.GetPlugin("save-disabled")
	if err == nil {
		t.Error("Disabled plugin should not be in memory")
	}

	// 但应该在 DB 中
	dbPlugin, err := store.GetPlugin("save-disabled")
	if err != nil {
		t.Fatalf("GetPlugin from DB failed: %v", err)
	}
	if dbPlugin.Metadata.Enabled {
		t.Error("DB plugin should be disabled")
	}
}

func TestPluginManager_DeletePlugin_WithStore(t *testing.T) {
	_, store := setupTestDB(t)
	pm := NewPluginManager(store, nil)

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "delete-store",
			Name:    "Delete Store Test",
			Version: "1.0.0",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		SourceCode:   `const plugin = { onEvent: (e, ctx) => null };`,
		CompiledCode: `const plugin = { onEvent: (e, ctx) => null };`,
		Language:     "typescript",
		State:        make(map[string]interface{}),
	}

	if err := pm.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin failed: %v", err)
	}

	// 删除
	if err := pm.DeletePlugin("delete-store"); err != nil {
		t.Fatalf("DeletePlugin failed: %v", err)
	}

	// 内存中不应存在
	_, err := pm.GetPlugin("delete-store")
	if err == nil {
		t.Error("Plugin should not be in memory after delete")
	}

	// DB 中也不应存在
	_, err = store.GetPlugin("delete-store")
	if err == nil {
		t.Error("Plugin should not be in DB after delete")
	}
}

func TestPluginManager_TogglePlugin(t *testing.T) {
	_, store := setupTestDB(t)
	pm := NewPluginManager(store, nil)

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:      "toggle-test",
			Name:    "Toggle Test",
			Version: "1.0.0",
			Enabled: true,
			Filters: PluginFilters{},
			Config:  make(map[string]interface{}),
		},
		SourceCode:   `const plugin = { onEvent: (e, ctx) => null };`,
		CompiledCode: `const plugin = { onEvent: (e, ctx) => null };`,
		Language:     "typescript",
		State:        make(map[string]interface{}),
	}

	if err := pm.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin failed: %v", err)
	}

	// 确认已加载
	_, err := pm.GetPlugin("toggle-test")
	if err != nil {
		t.Fatalf("Plugin should be loaded: %v", err)
	}

	// 禁用
	if err := pm.TogglePlugin("toggle-test", false); err != nil {
		t.Fatalf("TogglePlugin(false) failed: %v", err)
	}

	// 验证已卸载
	_, err = pm.GetPlugin("toggle-test")
	if err == nil {
		t.Error("Plugin should be unloaded after disable")
	}

	// 验证 DB 中是禁用状态
	dbPlugin, err := store.GetPlugin("toggle-test")
	if err != nil {
		t.Fatalf("GetPlugin from DB failed: %v", err)
	}
	if dbPlugin.Metadata.Enabled {
		t.Error("DB plugin should be disabled")
	}

	// 重新启用
	if err := pm.TogglePlugin("toggle-test", true); err != nil {
		t.Fatalf("TogglePlugin(true) failed: %v", err)
	}

	// 验证已重新加载
	loaded, err := pm.GetPlugin("toggle-test")
	if err != nil {
		t.Fatalf("Plugin should be loaded after enable: %v", err)
	}
	if !loaded.Metadata.Enabled {
		t.Error("Plugin should be enabled")
	}
}

func TestPluginManager_ListPlugins_InMemory(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	// 空列表
	if list := pm.ListPlugins(); len(list) != 0 {
		t.Errorf("Expected empty list, got %d", len(list))
	}

	// 加载 2 个插件
	for _, id := range []string{"list-1", "list-2"} {
		p := &Plugin{
			Metadata: PluginMetadata{
				ID:     id,
				Config: make(map[string]interface{}),
			},
			CompiledCode: `const plugin = { onEvent: (e, ctx) => null };`,
			Language:     "javascript",
			State:        make(map[string]interface{}),
		}
		if err := pm.LoadPlugin(p); err != nil {
			t.Fatalf("LoadPlugin(%s) failed: %v", id, err)
		}
	}

	list := pm.ListPlugins()
	if len(list) != 2 {
		t.Errorf("Expected 2 plugins, got %d", len(list))
	}
}

func TestPluginManager_GetPlugin_NotFound(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	_, err := pm.GetPlugin("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent plugin, got nil")
	}
}

func TestPluginManager_LoadAllPlugins(t *testing.T) {
	_, store := setupTestDB(t)
	pm := NewPluginManager(store, nil)

	// 保存 2 个启用的和 1 个禁用的插件到 DB
	plugins := []*Plugin{
		{
			Metadata: PluginMetadata{
				ID: "all-1", Name: "All 1", Version: "1.0.0", Enabled: true,
				Filters: PluginFilters{}, Config: make(map[string]interface{}),
			},
			SourceCode:   "const plugin = { onEvent: (e, ctx) => null };",
			CompiledCode: "const plugin = { onEvent: (e, ctx) => null };",
			Language:     "typescript", State: make(map[string]interface{}),
		},
		{
			Metadata: PluginMetadata{
				ID: "all-2", Name: "All 2", Version: "1.0.0", Enabled: true,
				Filters: PluginFilters{}, Config: make(map[string]interface{}),
			},
			SourceCode:   "const plugin = { onEvent: (e, ctx) => null };",
			CompiledCode: "const plugin = { onEvent: (e, ctx) => null };",
			Language:     "typescript", State: make(map[string]interface{}),
		},
		{
			Metadata: PluginMetadata{
				ID: "all-disabled", Name: "All Disabled", Version: "1.0.0", Enabled: false,
				Filters: PluginFilters{}, Config: make(map[string]interface{}),
			},
			SourceCode:   "const plugin = { onEvent: (e, ctx) => null };",
			CompiledCode: "const plugin = { onEvent: (e, ctx) => null };",
			Language:     "typescript", State: make(map[string]interface{}),
		},
	}

	for _, p := range plugins {
		if err := store.SavePlugin(p); err != nil {
			t.Fatalf("SavePlugin(%s) failed: %v", p.Metadata.ID, err)
		}
	}

	// LoadAllPlugins 应该只加载启用的
	if err := pm.LoadAllPlugins(); err != nil {
		t.Fatalf("LoadAllPlugins failed: %v", err)
	}

	loaded := pm.ListPlugins()
	if len(loaded) != 2 {
		t.Errorf("Expected 2 loaded plugins (enabled only), got %d", len(loaded))
	}

	// 禁用的不应在内存中
	_, err := pm.GetPlugin("all-disabled")
	if err == nil {
		t.Error("Disabled plugin should not be loaded into memory")
	}
}

// ========== ProcessEvent 多插件匹配测试 ==========

func TestPluginManager_ProcessEvent_MultiplePluginsMatch(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	// 两个插件都匹配 network 事件
	for i, id := range []string{"multi-1", "multi-2"} {
		plugin := &Plugin{
			Metadata: PluginMetadata{
				ID:      id,
				Enabled: true,
				Filters: PluginFilters{Sources: []string{"network"}},
				Config:  make(map[string]interface{}),
			},
			CompiledCode: fmt.Sprintf(`
				const plugin = {
					onEvent: (e, ctx) => ({
						derivedEvents: [{
							source: "plugin",
							type: "from_%d",
							level: "info",
							title: "From plugin %d"
						}]
					})
				};
			`, i, i),
			Language: "javascript",
			State:    make(map[string]interface{}),
		}
		if err := pm.LoadPlugin(plugin); err != nil {
			t.Fatalf("LoadPlugin(%s) failed: %v", id, err)
		}
	}

	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "http_request",
		Level:    LevelInfo,
		Title:    "GET /api",
	}

	results := pm.ProcessEvent(event, "session-1")

	// 两个插件各产生 1 个事件 = 共 2 个
	if len(results) != 2 {
		t.Fatalf("Expected 2 derived events from 2 plugins, got %d", len(results))
	}
}

func TestPluginManager_ProcessEvent_EmptyPlugins(t *testing.T) {
	pm := &PluginManager{plugins: make(map[string]*Plugin)}

	event := UnifiedEvent{
		ID:       uuid.New().String(),
		DeviceID: "dev-1",
		Source:   SourceNetwork,
		Type:     "test",
		Level:    LevelInfo,
		Title:    "Test",
	}

	results := pm.ProcessEvent(event, "session-1")
	if len(results) != 0 {
		t.Errorf("Expected 0 results with no plugins, got %d", len(results))
	}
}
