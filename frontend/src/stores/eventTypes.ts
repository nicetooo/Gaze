/**
 * Unified Event Types - matches backend event_types.go
 * Core types for the new event system with SQLite persistence
 */

import React from 'react';
import {
  // Source icons
  FileTextOutlined,
  GlobalOutlined,
  MobileOutlined,
  AppstoreOutlined,
  PictureOutlined,
  AimOutlined,
  NodeIndexOutlined,
  BarChartOutlined,
  ToolOutlined,
  CheckCircleOutlined,
  // Level icons
  UnorderedListOutlined,
  BugOutlined,
  InfoCircleOutlined,
  WarningOutlined,
  CloseCircleOutlined,
  StopOutlined,
  // Event type icons
  PlayCircleOutlined,
  FlagOutlined,
  ApiOutlined,
  ThunderboltOutlined,
  WifiOutlined,
  DesktopOutlined,
  PauseCircleOutlined,
  FileOutlined,
  ExclamationCircleOutlined,
  ClockCircleOutlined,
  HighlightOutlined,
  ExpandOutlined,
  CaretRightOutlined,
  CheckOutlined,
  BorderOutlined,
  FieldTimeOutlined,
  AppstoreAddOutlined,
} from '@ant-design/icons';

// ========================================
// Event Source - 事件来源
// ========================================

export type EventSource =
  | 'logcat'     // 设备日志
  | 'network'    // 网络请求
  | 'device'     // 设备状态
  | 'app'        // 应用生命周期
  | 'ui'         // UI 状态变化
  | 'touch'      // 触摸事件
  | 'workflow'   // 自动化流程
  | 'perf'       // 性能指标
  | 'system'     // 系统事件
  | 'assertion'  // 断言结果
  | 'plugin';    // 插件生成的事件

// ========================================
// Event Category - 事件大类
// ========================================

export type EventCategory =
  | 'log'
  | 'network'
  | 'state'
  | 'interaction'
  | 'automation'
  | 'diagnostic'
  | 'plugin'; // 插件生成的事件

// ========================================
// Event Level - 事件级别
// ========================================

export type EventLevel =
  | 'verbose'
  | 'debug'
  | 'info'
  | 'warn'
  | 'error'
  | 'fatal';

// ========================================
// UnifiedEvent - 统一事件结构
// ========================================

export interface UnifiedEvent {
  // 标识字段
  id: string;
  sessionId: string;
  deviceId: string;

  // 时间字段
  timestamp: number;      // Unix 毫秒 (绝对时间)
  relativeTime: number;   // 相对 Session 开始的毫秒偏移
  duration?: number;      // 持续时间 (ms)

  // 分类字段
  source: EventSource;
  category: EventCategory;
  type: string;           // 具体类型 (如 "network_request", "activity_start")
  level: EventLevel;

  // 内容字段
  title: string;
  summary?: string;

  // 扩展数据 (JSON)
  data?: any;

  // Legacy alias for backward compatibility
  detail?: any;

  // 关联字段
  parentId?: string;
  stepId?: string;
  traceId?: string;

  // 聚合信息
  aggregateCount?: number;
  aggregateFirst?: number;
  aggregateLast?: number;
}

// ========================================
// Session Config - Session 启动配置
// ========================================

export interface LogcatConfig {
  enabled: boolean;
  packageName?: string;
  preFilter?: string;
  excludeFilter?: string;
}

export interface RecordingConfig {
  enabled: boolean;
  quality?: 'low' | 'medium' | 'high';
}

export interface ProxyConfig {
  enabled: boolean;
  port?: number;
  mitmEnabled?: boolean;
}

export interface MonitorConfig {
  enabled: boolean; // 设备状态监控 (电池、网络、屏幕、应用生命周期)
}

export interface SessionConfig {
  logcat: LogcatConfig;
  recording: RecordingConfig;
  proxy: ProxyConfig;
  monitor: MonitorConfig;
}

// 默认配置 - 全部启用
export const defaultSessionConfig: SessionConfig = {
  logcat: { enabled: true, packageName: '' },
  recording: { enabled: true, quality: 'medium' },
  proxy: { enabled: true, port: 8080, mitmEnabled: true },
  monitor: { enabled: true },
};

// ========================================
// DeviceSession - 设备会话
// ========================================

export interface DeviceSession {
  id: string;
  deviceId: string;
  type: string;       // "manual", "workflow", "recording", "debug", "auto"
  name: string;
  startTime: number;  // Unix ms
  endTime: number;    // 0 = active
  status: 'active' | 'completed' | 'failed' | 'cancelled';
  eventCount: number;

  // Session config (启用的功能)
  config?: SessionConfig;

  // Recording info
  videoPath?: string;
  videoDuration?: number;
  videoOffset?: number;

  // Metadata
  metadata?: Record<string, any>;
}

// ========================================
// Type-Specific Data Structures
// ========================================

// Logcat 事件数据
export interface LogcatData {
  tag: string;
  message: string;
  androidLevel: string; // V/D/I/W/E/F
  pid?: number;
  tid?: number;
  packageName?: string;
  raw?: string;
}

// ========================================
// Query Types
// ========================================

export interface EventQuery {
  sessionId?: string;
  deviceId?: string;
  sources?: EventSource[];
  categories?: EventCategory[];
  types?: string[];
  levels?: EventLevel[];
  startTime?: number;   // 相对时间 (ms)
  endTime?: number;
  searchText?: string;
  parentId?: string;
  stepId?: string;
  traceId?: string;
  limit?: number;
  offset?: number;
  orderDesc?: boolean;  // true = 时间倒序
  includeData?: boolean; // true = 加载完整 event_data（网络请求URL等）
}

export interface EventQueryResult {
  events: UnifiedEvent[];
  total: number;
  hasMore: boolean;
}

// ========================================
// Time Index & Bookmarks
// ========================================

export interface TimeIndexEntry {
  second: number;        // 相对 session 开始的秒数
  eventCount: number;
  firstEventId: string;
  hasError: boolean;
}

export interface Bookmark {
  id: string;
  sessionId: string;
  relativeTime: number;  // 相对 session 开始的毫秒
  label: string;
  color?: string;
  type: 'user' | 'error' | 'milestone' | 'assertion_fail';
  createdAt: number;
}

// ========================================
// UI Display Helpers
// ========================================

// Source 显示配置 - 使用 Ant Design 预设颜色名称以适配主题
export const sourceConfig: Record<EventSource, { label: string; icon: string; iconComponent: React.ReactNode; color: string }> = {
  logcat: { label: 'Logcat', icon: '📝', iconComponent: React.createElement(FileTextOutlined), color: 'green' },
  network: { label: 'Network', icon: '🌐', iconComponent: React.createElement(GlobalOutlined), color: 'purple' },
  device: { label: 'Device', icon: '📱', iconComponent: React.createElement(MobileOutlined), color: 'cyan' },
  app: { label: 'App', icon: '📦', iconComponent: React.createElement(AppstoreOutlined), color: 'blue' },
  ui: { label: 'UI', icon: '🖼️', iconComponent: React.createElement(PictureOutlined), color: 'magenta' },
  touch: { label: 'Touch', icon: '👆', iconComponent: React.createElement(AimOutlined), color: 'orange' },
  workflow: { label: 'Workflow', icon: '⚙️', iconComponent: React.createElement(NodeIndexOutlined), color: 'geekblue' },
  perf: { label: 'Perf', icon: '📊', iconComponent: React.createElement(BarChartOutlined), color: 'gold' },
  system: { label: 'System', icon: '🔧', iconComponent: React.createElement(ToolOutlined), color: 'default' },
  assertion: { label: 'Assert', icon: '✅', iconComponent: React.createElement(CheckCircleOutlined), color: 'green' },
  plugin: { label: 'Plugin', icon: '🧩', iconComponent: React.createElement(AppstoreAddOutlined), color: 'volcano' },
};

// Category 显示配置 - 使用 Ant Design 预设颜色名称以适配主题
export const categoryConfig: Record<EventCategory, { label: string; color: string }> = {
  log: { label: 'Log', color: 'green' },
  network: { label: 'Network', color: 'purple' },
  state: { label: 'State', color: 'blue' },
  interaction: { label: 'Interaction', color: 'orange' },
  automation: { label: 'Automation', color: 'geekblue' },
  diagnostic: { label: 'Diagnostic', color: 'default' },
  plugin: { label: 'Plugin', color: 'volcano' },
};

// Level 显示配置 - 使用 Ant Design 预设颜色名称以适配主题
export const levelConfig: Record<EventLevel, { label: string; color: string; icon: string; iconComponent: React.ReactNode; priority: number }> = {
  verbose: { label: 'Verbose', color: 'default', icon: '📋', iconComponent: React.createElement(UnorderedListOutlined), priority: 0 },
  debug: { label: 'Debug', color: 'default', icon: '🔧', iconComponent: React.createElement(BugOutlined), priority: 1 },
  info: { label: 'Info', color: 'blue', icon: 'ℹ️', iconComponent: React.createElement(InfoCircleOutlined), priority: 2 },
  warn: { label: 'Warn', color: 'gold', icon: '⚠️', iconComponent: React.createElement(WarningOutlined), priority: 3 },
  error: { label: 'Error', color: 'red', icon: '❌', iconComponent: React.createElement(CloseCircleOutlined), priority: 4 },
  fatal: { label: 'Fatal', color: 'red', icon: '💀', iconComponent: React.createElement(StopOutlined), priority: 5 },
};

// Event Type 显示配置
export const eventTypeConfig: Record<string, { label: string; icon: string; iconComponent: React.ReactNode }> = {
  // Session
  session_start: { label: 'Session Start', icon: '🚀', iconComponent: React.createElement(PlayCircleOutlined, { style: { color: '#52c41a' } }) },
  session_end: { label: 'Session End', icon: '🏁', iconComponent: React.createElement(FlagOutlined) },

  // Logcat
  logcat: { label: 'Log', icon: '📝', iconComponent: React.createElement(FileTextOutlined) },
  logcat_aggregated: { label: 'Logs', icon: '📝', iconComponent: React.createElement(FileTextOutlined) },

  // Network
  // network_request 是生产代码实际发送的类型；http_request 为废弃旧名，保留以渲染历史数据
  network_request: { label: 'HTTP Request', icon: '🌐', iconComponent: React.createElement(ApiOutlined) },
  http_request: { label: 'HTTP Request', icon: '🌐', iconComponent: React.createElement(ApiOutlined) },
  websocket_message: { label: 'WebSocket', icon: '🔌', iconComponent: React.createElement(ApiOutlined, { style: { color: '#722ed1' } }) },

  // Device State
  battery_change: { label: 'Battery', icon: '🔋', iconComponent: React.createElement(ThunderboltOutlined, { style: { color: '#faad14' } }) },
  network_change: { label: 'Network', icon: '📶', iconComponent: React.createElement(WifiOutlined) },
  screen_change: { label: 'Screen', icon: '📱', iconComponent: React.createElement(DesktopOutlined) },

  // App Lifecycle
  app_start: { label: 'App Start', icon: '▶️', iconComponent: React.createElement(CaretRightOutlined, { style: { color: '#52c41a' } }) },
  app_stop: { label: 'App Stop', icon: '⏹️', iconComponent: React.createElement(PauseCircleOutlined) },
  activity_start: { label: 'Activity', icon: '📄', iconComponent: React.createElement(FileOutlined, { style: { color: '#1890ff' } }) },
  activity_stop: { label: 'Activity Stop', icon: '📄', iconComponent: React.createElement(FileOutlined) },
  app_crash: { label: 'Crash', icon: '💥', iconComponent: React.createElement(ExclamationCircleOutlined, { style: { color: '#ff4d4f' } }) },
  app_anr: { label: 'ANR', icon: '⏰', iconComponent: React.createElement(ClockCircleOutlined, { style: { color: '#fa8c16' } }) },

  // Touch
  touch: { label: 'Touch', icon: '👆', iconComponent: React.createElement(HighlightOutlined) },
  gesture: { label: 'Gesture', icon: '🤚', iconComponent: React.createElement(ExpandOutlined) },

  // Workflow
  workflow_start: { label: 'Workflow Start', icon: '▶️', iconComponent: React.createElement(CaretRightOutlined, { style: { color: '#2f54eb' } }) },
  workflow_step_start: { label: 'Step Start', icon: '🔷', iconComponent: React.createElement(BorderOutlined, { style: { color: '#1890ff' } }) },
  workflow_step_end: { label: 'Step End', icon: '✅', iconComponent: React.createElement(CheckOutlined, { style: { color: '#52c41a' } }) },
  workflow_complete: { label: 'Workflow Complete', icon: '🏁', iconComponent: React.createElement(FlagOutlined, { style: { color: '#52c41a' } }) },
  workflow_error: { label: 'Workflow Error', icon: '❌', iconComponent: React.createElement(CloseCircleOutlined, { style: { color: '#ff4d4f' } }) },

  // Performance
  perf_sample: { label: 'Perf Sample', icon: '📊', iconComponent: React.createElement(BarChartOutlined) },

  // Assertion
  assertion_result: { label: 'Assertion', icon: '✅', iconComponent: React.createElement(CheckCircleOutlined, { style: { color: '#52c41a' } }) },

  // Recording
  recording_start: { label: 'Recording Start', icon: '🔴', iconComponent: React.createElement(FieldTimeOutlined, { style: { color: '#ff4d4f' } }) },
  recording_end: { label: 'Recording End', icon: '⏹️', iconComponent: React.createElement(PauseCircleOutlined) },

  // Plugin - TikTok events
  tiktok_launch: { label: 'TikTok Launch', icon: '🚀', iconComponent: React.createElement(PlayCircleOutlined, { style: { color: '#ff6b35' } }) },
  tiktok_event: { label: 'TikTok Event', icon: '📱', iconComponent: React.createElement(MobileOutlined, { style: { color: '#ff6b35' } }) },
  tiktok_event_v3: { label: 'TikTok Event V3', icon: '📊', iconComponent: React.createElement(BarChartOutlined, { style: { color: '#ff6b35' } }) },
};

// ========================================
// Utility Functions
// ========================================

/**
 * 格式化相对时间为可读字符串
 */
export function formatRelativeTime(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  const millis = ms % 1000;

  if (hours > 0) {
    return `${hours}:${pad(minutes)}:${pad(seconds)}.${pad3(millis)}`;
  }
  return `${minutes}:${pad(seconds)}.${pad3(millis)}`;
}

/**
 * 格式化绝对时间戳为时间字符串
 */
export function formatTimestamp(timestamp: number): string {
  const date = new Date(timestamp);
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }) + '.' + pad3(date.getMilliseconds());
}

/**
 * 格式化持续时间 (毫秒 → 紧凑格式: "381ms", "5.0s", "2m 30s")
 */
export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const mins = Math.floor(ms / 60000);
  const secs = Math.floor((ms % 60000) / 1000);
  return `${mins}m ${secs}s`;
}

/**
 * 格式化持续时间 (毫秒 → 人类可读: "3s", "2m 3s", "1h 2m 3s")
 */
export function formatDurationHMS(ms: number): string {
  if (!ms || ms <= 0) return '-';
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);

  if (hours > 0) {
    return `${hours}h ${minutes % 60}m ${seconds % 60}s`;
  } else if (minutes > 0) {
    return `${minutes}m ${seconds % 60}s`;
  } else {
    return `${seconds}s`;
  }
}

/**
 * 格式化持续时间 (秒 → MM:SS 格式)
 */
export function formatDurationMMSS(seconds: number): string {
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
}

/**
 * 获取事件显示图标 (返回 React 组件)
 */
export function getEventIcon(event: UnifiedEvent): React.ReactNode {
  // 优先使用事件类型图标
  const typeInfo = eventTypeConfig[event.type];
  if (typeInfo?.iconComponent) return typeInfo.iconComponent;

  // 其次使用来源图标
  const sourceInfo = sourceConfig[event.source];
  if (sourceInfo?.iconComponent) return sourceInfo.iconComponent;

  // 最后使用级别图标
  const levelInfo = levelConfig[event.level];
  if (levelInfo?.iconComponent) return levelInfo.iconComponent;

  return React.createElement('span', null, '•');
}

/**
 * 获取事件显示颜色
 */
export function getEventColor(event: UnifiedEvent): string {
  // 优先级: level error/fatal > category > source
  if (event.level === 'error' || event.level === 'fatal') {
    return levelConfig[event.level].color;
  }
  if (event.level === 'warn') {
    return levelConfig.warn.color;
  }
  return categoryConfig[event.category]?.color || sourceConfig[event.source]?.color || 'default';
}

/**
 * 判断事件是否为关键事件
 */
export function isCriticalEvent(event: UnifiedEvent): boolean {
  return (
    event.level === 'error' ||
    event.level === 'fatal' ||
    event.type === 'app_crash' ||
    event.type === 'app_anr'
  );
}

// Helper functions
function pad(n: number): string {
  return n.toString().padStart(2, '0');
}

function pad3(n: number): string {
  return n.toString().padStart(3, '0');
}


