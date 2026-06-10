# Plugin Example: Performance Monitor

这是一个完整的插件示例，展示如何使用 `context.config` 来配置插件行为。

## 插件功能

监控应用性能，检测：
- 慢速网络请求
- 高 CPU 使用率
- 内存警告
- 异常错误模式

## 配置参数 (Config Tab)

```json
{
  // ========== Performance Monitoring ==========
  
  // Slow request threshold in milliseconds
  "slowThresholdMs": 2000,
  
  // High CPU usage threshold (percentage)
  "highCpuThreshold": 80,
  
  // Memory warning threshold in MB
  "memoryWarningMb": 512,
  
  
  // ========== API Validation ==========
  
  // Required parameters for tracking API calls
  "requiredParams": ["user_id", "event_name", "timestamp"],
  
  // Allowed HTTP methods
  "allowedMethods": ["GET", "POST", "PUT", "DELETE"],
  
  // API endpoint patterns to monitor
  "monitoredEndpoints": [
    "*/api/track*",
    "*/api/analytics*",
    "*/api/events*"
  ],
  
  
  // ========== Error Detection ==========
  
  // Enable automatic error detection
  "autoDetectErrors": true,
  
  // Keywords to identify errors in logs
  "errorKeywords": ["ERROR", "FATAL", "Exception", "crash"],
  
  
  // ========== Notification Settings ==========
  
  // Enable notifications for critical events
  "enableNotifications": true,
  
  // Minimum event level for notifications (info/warn/error/fatal)
  "notificationLevel": "error",
  
  // Maximum notifications per minute (rate limiting)
  "maxNotificationsPerMinute": 5,
  
  
  // ========== Custom Rules ==========
  
  // Custom business rules configuration
  "customRules": {
    "validateUserSession": true,
    "checkDataIntegrity": true,
    "enforceRateLimits": false
  },
  
  // Timeout for async operations (milliseconds)
  "timeout": 5000,
  
  // Enable debug logging
  "debugMode": false
}
```

## 插件代码 (Code Tab)

```typescript
// Plugin metadata is managed by the form (Basic Info & Filters tabs)
// This code only defines the event processing logic

const plugin: Plugin = {
  // Initialize plugin state
  onInit: (context: PluginContext): void => {
    context.log("Performance Monitor Plugin initialized");
    
    // Initialize state counters
    context.state.slowRequestCount = 0;
    context.state.errorCount = 0;
    context.state.lastNotificationTime = 0;
    context.state.notificationCount = 0;
    
    // Log current configuration
    const debugMode = context.config.debugMode || false;
    if (debugMode) {
      context.log("Debug mode enabled");
      context.log("Config: " + JSON.stringify(context.config));
    }
  },
  
  // Process each matching event
  onEvent: (event: UnifiedEvent, context: PluginContext): PluginResult | null => {
    const derivedEvents: any[] = [];
    
    // 1. Monitor slow network requests
    if (event.source === "network" && event.type === "network_request") {
      const threshold = context.config.slowThresholdMs || 2000;
      const duration = event.data?.duration || 0;
      
      if (duration > threshold) {
        context.state.slowRequestCount++;
        
        derivedEvents.push({
          source: "plugin",
          type: "performance_warning",
          level: "warn",
          title: `Slow request detected: ${duration}ms`,
          data: {
            url: event.data?.url,
            duration: duration,
            threshold: threshold,
            totalSlowRequests: context.state.slowRequestCount
          }
        });
        
        context.log(`Slow request: ${event.data?.url} (${duration}ms)`);
      }
    }
    
    // 2. Monitor CPU usage
    if (event.source === "perf" && event.type === "perf_sample") {
      const cpuThreshold = context.config.highCpuThreshold || 80;
      const cpuUsage = event.data?.cpu?.usage || 0;
      
      if (cpuUsage > cpuThreshold) {
        derivedEvents.push({
          source: "plugin",
          type: "cpu_warning",
          level: "warn",
          title: `High CPU usage: ${cpuUsage}%`,
          data: {
            cpuUsage: cpuUsage,
            threshold: cpuThreshold,
            timestamp: event.timestamp
          }
        });
      }
    }
    
    // 3. Monitor memory usage
    if (event.source === "perf" && event.type === "perf_sample") {
      const memThreshold = context.config.memoryWarningMb || 512;
      const memUsageMb = event.data?.memory?.appMemoryMb || 0;
      
      if (memUsageMb > memThreshold) {
        derivedEvents.push({
          source: "plugin",
          type: "memory_warning",
          level: "warn",
          title: `High memory usage: ${memUsageMb}MB`,
          data: {
            memoryMb: memUsageMb,
            threshold: memThreshold
          }
        });
      }
    }
    
    // 4. Validate API tracking calls
    if (event.source === "network" && event.type === "network_request") {
      const url = event.data?.url || "";
      const monitoredEndpoints = context.config.monitoredEndpoints || [];
      
      // Check if URL matches monitored endpoints
      let shouldValidate = false;
      for (const pattern of monitoredEndpoints) {
        if (context.matchURL(url, pattern)) {
          shouldValidate = true;
          break;
        }
      }
      
      if (shouldValidate) {
        try {
          const urlObj = new URL(url);
          const params = urlObj.searchParams;
          const requiredParams = context.config.requiredParams || [];
          const missing: string[] = [];
          
          for (const param of requiredParams) {
            if (!params.has(param)) {
              missing.push(param);
            }
          }
          
          if (missing.length > 0) {
            derivedEvents.push({
              source: "plugin",
              type: "validation_error",
              level: "error",
              title: `Missing required parameters: ${missing.join(", ")}`,
              data: {
                url: url,
                missingParams: missing,
                foundParams: Array.from(params.keys())
              }
            });
            
            context.log(`Validation failed for ${url}: missing ${missing.join(", ")}`);
          }
        } catch (err: any) {
          context.log("Failed to parse URL: " + err.message);
        }
      }
    }
    
    // 5. Auto-detect errors in logs
    if (context.config.autoDetectErrors && event.source === "logcat") {
      const errorKeywords = context.config.errorKeywords || [];
      const message = event.title + " " + (event.content || "");
      
      for (const keyword of errorKeywords) {
        if (message.includes(keyword)) {
          context.state.errorCount++;
          
          derivedEvents.push({
            source: "plugin",
            type: "error_detected",
            level: "error",
            title: `Error keyword detected: ${keyword}`,
            data: {
              keyword: keyword,
              message: message,
              totalErrors: context.state.errorCount
            }
          });
          
          break; // Only emit once per event
        }
      }
    }
    
    // 6. Rate limiting for notifications
    if (derivedEvents.length > 0 && context.config.enableNotifications) {
      const now = Date.now();
      const maxPerMinute = context.config.maxNotificationsPerMinute || 5;
      
      // Reset counter if more than 1 minute has passed
      if (now - context.state.lastNotificationTime > 60000) {
        context.state.notificationCount = 0;
        context.state.lastNotificationTime = now;
      }
      
      // Check rate limit
      if (context.state.notificationCount >= maxPerMinute) {
        context.log("Notification rate limit reached, skipping notification");
        return null; // Skip notifications
      }
      
      context.state.notificationCount++;
    }
    
    // Return derived events
    if (derivedEvents.length > 0) {
      return {
        derivedEvents: derivedEvents,
        tags: ["performance-monitor"],
        metadata: {
          monitoredBy: "performance-monitor-plugin",
          configVersion: "1.0.0"
        }
      };
    }
    
    return null;
  },
  
  // Cleanup on plugin unload
  onDestroy: (context: PluginContext): void => {
    context.log("Performance Monitor Plugin destroyed");
    context.log(`Total slow requests: ${context.state.slowRequestCount}`);
    context.log(`Total errors detected: ${context.state.errorCount}`);
  }
};
```

## 基础信息 (Basic Info Tab)

- **ID**: `performance-monitor`
- **Name**: Performance Monitor
- **Version**: 1.0.0
- **Author**: Gaze Team
- **Description**: Monitors app performance and detects issues like slow requests, high CPU/memory usage, and API validation errors

## 事件过滤器 (Filters Tab)

- **Event Sources**: `network`, `perf`, `logcat`
- **Event Types**: `network_request`, `perf_sample`, `logcat`
- **URL Pattern**: (留空，由 config 中的 monitoredEndpoints 控制)

## 使用说明

1. 创建插件时，复制上面的配置参数到 Config tab
2. 根据需要调整阈值（如 slowThresholdMs, highCpuThreshold）
3. 配置要监控的 API 端点（monitoredEndpoints）
4. 设置必需的参数列表（requiredParams）
5. 保存并启用插件
6. 插件会自动检测性能问题并生成警告事件

## Config 参数说明

| 参数 | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| slowThresholdMs | number | 2000 | 慢请求阈值（毫秒） |
| highCpuThreshold | number | 80 | 高 CPU 使用率阈值（百分比） |
| memoryWarningMb | number | 512 | 内存警告阈值（MB） |
| requiredParams | string[] | [] | API 必需参数列表 |
| allowedMethods | string[] | [] | 允许的 HTTP 方法 |
| monitoredEndpoints | string[] | [] | 监控的 API 端点模式 |
| autoDetectErrors | boolean | true | 自动检测日志中的错误 |
| errorKeywords | string[] | [] | 错误关键词列表 |
| enableNotifications | boolean | true | 启用通知 |
| notificationLevel | string | "error" | 通知的最低事件级别 |
| maxNotificationsPerMinute | number | 5 | 每分钟最大通知数 |
| customRules | object | {} | 自定义规则配置 |
| timeout | number | 5000 | 超时时间（毫秒） |
| debugMode | boolean | false | 调试模式 |

## 生成的事件类型

插件会生成以下派生事件：

- `performance_warning`: 慢请求检测
- `cpu_warning`: 高 CPU 使用率
- `memory_warning`: 高内存使用
- `validation_error`: API 参数验证失败
- `error_detected`: 日志中检测到错误关键词

这些事件可以在事件时间线中查看，也可以被其他插件处理或触发断言。
