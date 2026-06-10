/**
 * EventTimeline - 新事件系统的时间线组件
 * 使用 eventStore 和 SQLite 持久化
 */
import { useEffect, useRef, useMemo, useCallback, memo, Fragment } from 'react';
import {
  Card,
  Tag,
  Select,
  Button,
  Empty,
  Space,
  Typography,
  Tooltip,
  Badge,
  Input,
  Descriptions,
  Segmented,
  theme,
  Spin,
  Slider,
  Popover,
  Checkbox,
  Tabs,
  Modal,
  List,
} from 'antd';
import VirtualList, { VirtualListHandle } from './VirtualList';
import {
  ReloadOutlined,
  FilterOutlined,
  HistoryOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  BookOutlined,
  DownloadOutlined,
  SearchOutlined,
  CloseOutlined,
  VerticalAlignBottomOutlined,
  ExpandOutlined,
  ThunderboltOutlined,
  LeftOutlined,
  RightOutlined,
  DeleteOutlined,
  PushpinOutlined,
  ExclamationCircleOutlined,
  ClockCircleOutlined,
  WarningOutlined,
  BarChartOutlined,
  LoadingOutlined,
  BlockOutlined,
  BugOutlined,
} from '@ant-design/icons';
import AssertionsPanel from './AssertionsPanel';
import SessionStats from './SessionStats';
import NetworkWaterfall from './NetworkWaterfall';
import EventLanes from './EventLanes';
import { useTranslation } from 'react-i18next';
import {
  useEventStore,
  useDeviceStore,
  useCurrentBookmarks,
  useEventTimelineStore,
  useUIStore,
  VIEW_KEYS,
  type UnifiedEvent,
  type DeviceSession,
  type EventSource,
  type EventCategory,
  type EventLevel,
  type SessionConfig,
  type Bookmark,
  sourceConfig,
  categoryConfig,
  levelConfig,
  formatRelativeTime,
  formatTimestamp,
  getEventIcon,
  getEventColor,
  isCriticalEvent,
} from '../stores';
import { useProxyStore } from '../stores/proxyStore';
import SessionConfigModal from './SessionConfigModal';
import JsonViewer from './JsonViewer';

const { Text, Title } = Typography;

// ========================================
// 时间轴可视化组件
// ========================================

interface TimeIndexEntry {
  second: number;
  eventCount: number;
  firstEventId: string;
  hasError: boolean;
}

// 关键事件类型（用于时间轴标记）
interface CriticalEvent {
  id: string;
  relativeTime: number;
  type: string;
  title: string;
  level: string;
}

interface TimeRulerProps {
  sessionStart: number;
  sessionDuration: number;
  timeIndex: TimeIndexEntry[];
  currentTime: number;
  onSeek: (time: number) => void;
  timeRange: { start: number; end: number } | null;
  onTimeRangeChange: (range: { start: number; end: number } | null) => void;
  bookmarks?: Bookmark[];
  onBookmarkClick?: (bookmark: Bookmark) => void;
  criticalEvents?: CriticalEvent[];
  onCriticalEventClick?: (event: CriticalEvent) => void;
}

const TimeRuler = memo(({ sessionStart, sessionDuration, timeIndex, currentTime, onSeek, timeRange, onTimeRangeChange, bookmarks = [], onBookmarkClick, criticalEvents = [], onCriticalEventClick }: TimeRulerProps) => {
  const { token } = theme.useToken();
  const containerRef = useRef<HTMLDivElement>(null);
  const {
    isDragging,
    dragStart,
    dragEnd,
    startDrag,
    updateDrag,
    endDrag,
  } = useEventTimelineStore();

  const totalSeconds = Math.ceil(sessionDuration / 1000);

  // Convert array to map for quick lookup
  const indexMap = useMemo(() => {
    const map = new Map<number, TimeIndexEntry>();
    timeIndex.forEach(entry => map.set(entry.second, entry));
    return map;
  }, [timeIndex]);

  const maxCount = useMemo(() => {
    let max = 1;
    timeIndex.forEach(entry => {
      if (entry.eventCount > max) max = entry.eventCount;
    });
    return max;
  }, [timeIndex]);

  // Convert x position to time
  const posToTime = useCallback((clientX: number) => {
    if (!containerRef.current) return 0;
    const rect = containerRef.current.getBoundingClientRect();
    const x = Math.max(0, Math.min(clientX - rect.left, rect.width));
    const ratio = x / rect.width;
    return Math.round(ratio * sessionDuration);
  }, [sessionDuration]);

  // Handle mouse events for drag selection
  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    if (e.button !== 0) return; // Only left click
    e.preventDefault();
    const time = posToTime(e.clientX);
    startDrag(time);
  }, [posToTime, startDrag]);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!isDragging) return;
    const time = posToTime(e.clientX);
    updateDrag(time);
  }, [isDragging, posToTime, updateDrag]);

  const handleMouseUp = useCallback((e: MouseEvent) => {
    const result = endDrag();
    if (!result) return;

    const { start, end } = result;

    // If range is too small (< 100ms), treat as click to seek
    if (end - start < 100) {
      onTimeRangeChange(null);
      onSeek(start);
    } else {
      onTimeRangeChange({ start, end });
    }
  }, [endDrag, onSeek, onTimeRangeChange]);

  // Add/remove global mouse event listeners
  useEffect(() => {
    if (isDragging) {
      window.addEventListener('mousemove', handleMouseMove);
      window.addEventListener('mouseup', handleMouseUp);
      return () => {
        window.removeEventListener('mousemove', handleMouseMove);
        window.removeEventListener('mouseup', handleMouseUp);
      };
    }
  }, [isDragging, handleMouseMove, handleMouseUp]);

  // Calculate selection display (either from drag or from timeRange prop)
  const selectionStart = isDragging && dragStart !== null && dragEnd !== null
    ? Math.min(dragStart, dragEnd)
    : timeRange?.start ?? null;
  const selectionEnd = isDragging && dragStart !== null && dragEnd !== null
    ? Math.max(dragStart, dragEnd)
    : timeRange?.end ?? null;

  const hasSelection = selectionStart !== null && selectionEnd !== null && selectionEnd > selectionStart;

  return (
    <div style={{ position: 'relative' }}>
      <div
        ref={containerRef}
        onMouseDown={handleMouseDown}
        style={{
          height: 50,
          background: token.colorBgContainer,
          borderRadius: 4,
          position: 'relative',
          cursor: 'crosshair',
          border: `1px solid ${token.colorBorder}`,
          overflow: 'hidden',
          userSelect: 'none',
        }}
      >
        {/* Event density bars - 使用绝对定位让每个条形正确对应时间位置 */}
        <div style={{ position: 'relative', height: 36 }}>
          {Array.from({ length: Math.max(totalSeconds, 1) }).map((_, i) => {
            const entry = indexMap.get(i);
            const height = entry ? Math.max(4, (entry.eventCount / maxCount) * 30) : 0;
            const hasError = entry?.hasError;
            const secStart = i * 1000;
            const secEnd = (i + 1) * 1000;
            const isInSelection = hasSelection && secStart < selectionEnd! && secEnd > selectionStart!;

            // 使用百分比定位，确保条形位置与时间轴对齐
            const leftPercent = (i / totalSeconds) * 100;
            const widthPercent = (1 / totalSeconds) * 100;

            return (
              <div
                key={i}
                style={{
                  position: 'absolute',
                  left: `${leftPercent}%`,
                  width: `calc(${widthPercent}% - 1px)`,
                  height,
                  bottom: 0,
                  background: hasError ? token.colorError : token.colorPrimary,
                  opacity: isInSelection ? 1 : (hasError ? 0.9 : 0.7),
                  borderRadius: 1,
                }}
                title={entry ? `${i}s: ${entry.eventCount} events` : `${i}s: 0 events`}
              />
            );
          })}
        </div>

        {/* Selection overlay */}
        {hasSelection && sessionDuration > 0 && (
          <div
            style={{
              position: 'absolute',
              left: `${(selectionStart! / sessionDuration) * 100}%`,
              width: `${((selectionEnd! - selectionStart!) / sessionDuration) * 100}%`,
              top: 0,
              bottom: 0,
              background: token.colorPrimaryBg,
              borderLeft: `2px solid ${token.colorPrimary}`,
              borderRight: `2px solid ${token.colorPrimary}`,
              pointerEvents: 'none',
            }}
          />
        )}

        {/* Current position indicator (yellow) */}
        {sessionDuration > 0 && (
          <div
            style={{
              position: 'absolute',
              left: `${(currentTime / sessionDuration) * 100}%`,
              top: 0,
              bottom: 0,
              width: 2,
              background: token.colorWarning,
              pointerEvents: 'none',
              zIndex: 10,
            }}
          />
        )}

        {/* Bookmark markers */}
        {bookmarks.map(bookmark => (
          <Tooltip
            key={bookmark.id}
            title={`${bookmark.label} (${formatRelativeTime(bookmark.relativeTime)})`}
          >
            <div
              onClick={(e) => {
                e.stopPropagation();
                onBookmarkClick?.(bookmark);
              }}
              style={{
                position: 'absolute',
                left: `${(bookmark.relativeTime / sessionDuration) * 100}%`,
                top: 0,
                width: 0,
                height: 0,
                borderLeft: '6px solid transparent',
                borderRight: '6px solid transparent',
                borderTop: `10px solid ${bookmark.color || token.colorSuccess}`,
                transform: 'translateX(-6px)',
                cursor: 'pointer',
                zIndex: 15,
              }}
            />
          </Tooltip>
        ))}

        {/* Critical event markers */}
        {criticalEvents.map(event => {
          // 获取关键事件的颜色
          const eventColor = event.type === 'app_crash' ? '#ff4d4f' : 
                            event.type === 'app_anr' ? '#fa8c16' :
                            event.level === 'error' || event.level === 'fatal' ? '#ff4d4f' : '#faad14';
          return (
            <Tooltip
              key={event.id}
              title={`${event.title} (${formatRelativeTime(event.relativeTime)})`}
            >
              <div
                onClick={(e) => {
                  e.stopPropagation();
                  onCriticalEventClick?.(event);
                }}
                style={{
                  position: 'absolute',
                  left: `${(event.relativeTime / sessionDuration) * 100}%`,
                  bottom: 2,
                  transform: 'translateX(-6px)',
                  cursor: 'pointer',
                  zIndex: 18,
                  fontSize: 12,
                  color: eventColor,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  width: 12,
                  height: 12,
                }}
              >
                {event.type === 'app_crash' ? <ExclamationCircleOutlined /> :
                 event.type === 'app_anr' ? <ClockCircleOutlined /> :
                 <WarningOutlined />}
              </div>
            </Tooltip>
          );
        })}

        {/* Time labels */}
        <div style={{
          position: 'absolute',
          bottom: 2,
          left: 4,
          fontSize: 10,
          color: token.colorTextSecondary,
          pointerEvents: 'none',
        }}>
          {formatRelativeTime(0)}
        </div>
        <div style={{
          position: 'absolute',
          bottom: 2,
          right: 4,
          fontSize: 10,
          color: token.colorTextSecondary,
          pointerEvents: 'none',
        }}>
          {formatRelativeTime(sessionDuration)}
        </div>

        {/* Selection time labels */}
        {hasSelection && (
          <>
            <div style={{
              position: 'absolute',
              top: 2,
              left: `${(selectionStart! / sessionDuration) * 100}%`,
              transform: 'translateX(-50%)',
              fontSize: 10,
              fontWeight: 500,
              color: token.colorPrimary,
              background: token.colorBgContainer,
              padding: '0 2px',
              borderRadius: 2,
              pointerEvents: 'none',
            }}>
              {formatRelativeTime(selectionStart!)}
            </div>
            <div style={{
              position: 'absolute',
              top: 2,
              left: `${(selectionEnd! / sessionDuration) * 100}%`,
              transform: 'translateX(-50%)',
              fontSize: 10,
              fontWeight: 500,
              color: token.colorPrimary,
              background: token.colorBgContainer,
              padding: '0 2px',
              borderRadius: 2,
              pointerEvents: 'none',
            }}>
              {formatRelativeTime(selectionEnd!)}
            </div>
          </>
        )}
      </div>

      {/* Selection info and clear button */}
      {timeRange && (
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          marginTop: 4,
          fontSize: 12,
        }}>
          <Text type="secondary">
            {formatRelativeTime(timeRange.start)} - {formatRelativeTime(timeRange.end)}
            {' '}({((timeRange.end - timeRange.start) / 1000).toFixed(1)}s)
          </Text>
          <Button
            type="link"
            size="small"
            onClick={() => onTimeRangeChange(null)}
            style={{ padding: 0, height: 'auto' }}
          >
            <CloseOutlined />
          </Button>
        </div>
      )}

    </div>
  );
});

TimeRuler.displayName = 'TimeRuler';

// ========================================
// 事件行组件
// ========================================

interface EventRowProps {
  event: UnifiedEvent;
  isSelected: boolean;
  onClick: () => void;
  style?: React.CSSProperties;
}

const EventRow = memo(({ event, isSelected, onClick, style }: EventRowProps) => {
  const { token } = theme.useToken();
  const color = getEventColor(event);
  const icon = getEventIcon(event);
  const isCritical = isCriticalEvent(event);
  const isWarning = event.level === 'warn';

  // Determine background color based on state and level
  const getBackground = () => {
    if (isSelected) return token.colorPrimaryBg;
    if (isCritical) return token.colorErrorBg;
    if (isWarning) return token.colorWarningBg;
    return 'transparent';
  };

  return (
    <div
      onClick={onClick}
      style={{
        ...style,
        display: 'flex',
        alignItems: 'center',
        padding: '4px 8px',
        cursor: 'pointer',
        background: getBackground(),
        borderLeft: `3px solid ${color}`,
        borderBottom: `1px solid ${token.colorBorderSecondary}`,
        transition: 'background 0.15s',
      }}
    >
      {/* Time */}
      <Text
        type="secondary"
        style={{
          width: 80,
          flexShrink: 0,
          fontSize: 11,
          fontFamily: 'monospace'
        }}
      >
        {formatRelativeTime(event.relativeTime)}
      </Text>

      {/* Icon */}
      <span style={{ width: 24, display: 'inline-flex', justifyContent: 'center', alignItems: 'center', fontSize: 14 }}>
        {icon}
      </span>

      {/* Source tag */}
      <Tag
        color={sourceConfig[event.source]?.color || '#888'}
        style={{
          margin: '0 8px',
          fontSize: 10,
          padding: '0 4px',
          lineHeight: '18px',
        }}
      >
        {event.source}
      </Tag>

      {/* Title */}
      <Text
        ellipsis
        style={{
          flex: 1,
          fontSize: 12,
          color: isCritical ? token.colorError : token.colorText,
        }}
      >
        {event.title}
      </Text>

      {/* Duration if present */}
      {event.duration && event.duration > 0 && (
        <Text type="secondary" style={{ fontSize: 10, marginLeft: 8 }}>
          {event.duration}ms
        </Text>
      )}

      {/* Level indicator */}
      {(event.level === 'error' || event.level === 'warn') && (
        <Badge
          color={levelConfig[event.level].color}
          style={{ marginLeft: 8 }}
        />
      )}
    </div>
  );
});

EventRow.displayName = 'EventRow';

// ========================================
// 事件详情抽屉
// ========================================

interface EventDetailProps {
  event: UnifiedEvent | null;
  onClose: () => void;
  globalSearchText?: string;
}

// Network event detail component
const NetworkEventDetail = memo(({ data, token, externalSearchTerm }: { data: any; token: any; externalSearchTerm?: string }) => {
  const { t } = useTranslation();
  const { networkDetailActiveTab: activeTab, setNetworkDetailActiveTab: setActiveTab } = useEventTimelineStore();

  const statusColor = data.statusCode >= 500 ? '#ff4d4f' :
    data.statusCode >= 400 ? '#faad14' :
    data.statusCode >= 300 ? '#1890ff' : '#52c41a';

  const formatSize = (bytes: number) => {
    if (!bytes) return '-';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
  };

  const formatHeaders = (headers: Record<string, string> | null) => {
    if (!headers || Object.keys(headers).length === 0) {
      return <Text type="secondary">{t('timeline.no_headers')}</Text>;
    }
    return (
      <div style={{ fontSize: 12 }}>
        {Object.entries(headers).map(([key, value]) => (
          <div key={key} style={{ marginBottom: 4 }}>
            <Text strong style={{ color: token.colorPrimary }}>{key}: </Text>
            <Text copyable={{ text: value }} style={{ wordBreak: 'break-all' }}>{value}</Text>
          </div>
        ))}
      </div>
    );
  };

  const formatBody = (body: string | null, contentType: string | null) => {
    if (!body) return <Text type="secondary">{t('timeline.no_body')}</Text>;

    // Try to parse and display as JSON tree if content type suggests it
    if (contentType?.includes('json')) {
      try {
        const parsed = JSON.parse(body);
        return <JsonViewer data={parsed} fontSize={11} externalSearchTerm={externalSearchTerm} />;
      } catch {
        // Fall through to plain text
      }
    }

    return <JsonViewer data={body} fontSize={11} externalSearchTerm={externalSearchTerm} />;
  };

  // Wrapper to make each tab's content scrollable within the constrained height
  const scrollableTab = (content: React.ReactNode) => (
    <div style={{ overflow: 'auto', height: '100%', padding: '8px 12px 12px' }}>
      {content}
    </div>
  );

  // Parse query params from URL
  let queryParams: [string, string][] = [];
  try {
    const urlObj = new URL(data.url);
    queryParams = Array.from(urlObj.searchParams.entries());
  } catch {
    if (data.url?.includes('?')) {
      const search = data.url.split('?')[1];
      queryParams = search.split('&').map((p: string) => {
        const [k, v] = p.split('=');
        return [decodeURIComponent(k), decodeURIComponent(v || '')] as [string, string];
      });
    }
  }

  const tabItems = [
    {
      key: 'overview',
      label: t('timeline.overview'),
      children: scrollableTab(
        <Descriptions column={1} size="small" bordered>
          <Descriptions.Item label={t('timeline.url')}>
            <Text copyable style={{ fontSize: 11, wordBreak: 'break-all' }}>
              {data.url}
            </Text>
          </Descriptions.Item>
          <Descriptions.Item label={t('timeline.method')}>
            <Tag color="blue">{data.method}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label={t('timeline.status')}>
            <Tag color={statusColor}>{data.statusCode}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label={t('timeline.duration')}>
            {data.duration ? `${data.duration}ms` : '-'}
          </Descriptions.Item>
          <Descriptions.Item label={t('timeline.size')}>
            {formatSize(data.bodySize)}
          </Descriptions.Item>
          <Descriptions.Item label={t('timeline.content_type')}>
            {data.contentType || '-'}
          </Descriptions.Item>
          <Descriptions.Item label={t('timeline.protocol')}>
            {data.isHttps ? 'HTTPS' : 'HTTP'}
            {data.isWs && ' (WebSocket)'}
            {(data.isProtobuf || data.isReqProtobuf) && ' '}
            {(data.isProtobuf || data.isReqProtobuf) && <Tag color="geekblue">Protobuf</Tag>}
          </Descriptions.Item>
        </Descriptions>
      ),
    },
    {
      key: 'reqHeaders',
      label: t('timeline.request_headers'),
      children: scrollableTab(formatHeaders(data.requestHeaders)),
    },
    ...(queryParams.length > 0 ? [{
      key: 'queryParams',
      label: <span>{t('timeline.query_params')} <Tag style={{ marginLeft: 2, fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>{queryParams.length}</Tag></span>,
      children: scrollableTab(
        <div style={{
          border: `1px solid ${token.colorBorderSecondary}`,
          borderRadius: 6,
          overflow: 'hidden',
        }}>
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'minmax(80px, auto) 1fr',
            fontSize: 11,
          }}>
            {queryParams.map(([k, v], idx) => (
              <Fragment key={idx}>
                <div style={{
                  padding: '4px 8px',
                  fontWeight: 500,
                  color: token.colorTextSecondary,
                  background: token.colorFillAlter,
                  borderBottom: idx < queryParams.length - 1 ? `1px solid ${token.colorBorderSecondary}` : 'none',
                  borderRight: `1px solid ${token.colorBorderSecondary}`,
                  wordBreak: 'break-all',
                }}>{k}</div>
                <div style={{
                  padding: '4px 8px',
                  borderBottom: idx < queryParams.length - 1 ? `1px solid ${token.colorBorderSecondary}` : 'none',
                }}>
                  <Text copyable={{ text: v }} style={{ fontSize: 11, fontFamily: 'monospace', wordBreak: 'break-all', color: token.colorLink }}>{v}</Text>
                </div>
              </Fragment>
            ))}
          </div>
        </div>
      ),
    }] : []),
    {
      key: 'reqBody',
      label: data.isReqProtobuf ? `${t('timeline.request_body')} [PB]` : t('timeline.request_body'),
      children: scrollableTab(
        <>
          {data.isReqProtobuf && (
            <div style={{ marginBottom: 8 }}><Tag color="geekblue">Protobuf Decoded</Tag></div>
          )}
          {formatBody(data.requestBody, data.requestHeaders?.['Content-Type'] || data.requestHeaders?.['content-type'])}
        </>
      ),
    },
    {
      key: 'resHeaders',
      label: t('timeline.response_headers'),
      children: scrollableTab(formatHeaders(data.responseHeaders)),
    },
    {
      key: 'resBody',
      label: data.isProtobuf ? `${t('timeline.response_body')} [PB]` : t('timeline.response_body'),
      children: scrollableTab(
        <>
          {data.isProtobuf && (
            <div style={{ marginBottom: 8 }}><Tag color="geekblue">Protobuf Decoded</Tag></div>
          )}
          {formatBody(data.responseBody, data.contentType)}
        </>
      ),
    },
  ];

  return (
    <Tabs
      activeKey={activeTab}
      onChange={setActiveTab}
      size="small"
      items={tabItems}
      style={{ flex: 1, minHeight: 0 }}
      tabBarStyle={{ flexShrink: 0, margin: 0, paddingLeft: 12, paddingRight: 12 }}
      className="event-detail-tabs"
    />
  );
});

NetworkEventDetail.displayName = 'NetworkEventDetail';

const EventDetail = memo(({ event, onClose, globalSearchText }: EventDetailProps) => {
  const { token } = theme.useToken();
  const { t } = useTranslation();
  const { setSelectedKey } = useUIStore();
  const { openMockWithPrefill, openBreakpointWithPrefill } = useProxyStore();

  if (!event) return null;

  const getData = () => {
    if (!event.data) return null;
    try {
      return typeof event.data === 'string' ? JSON.parse(event.data) : event.data;
    } catch {
      return null;
    }
  };

  const data = getData();
  const isNetworkEvent = event.type === 'network_request' || event.category === 'network';

  const handleMockRequest = () => {
    if (!data) return;

    // Extract URL pattern (use path with wildcard for query params)
    let urlPattern = data.url;
    try {
      const urlObj = new URL(data.url);
      urlPattern = `*${urlObj.pathname}*`;
    } catch {
      urlPattern = `*${data.url.split('?')[0]}*`;
    }

    // Pre-fill mock data and navigate to proxy page
    openMockWithPrefill({
      urlPattern,
      method: data.method,
      statusCode: data.statusCode || 200,
      contentType: data.contentType?.split(';')[0] || 'application/json',
      body: data.responseBody || '',
      description: `Mock for ${data.method} ${urlPattern}`,
    });
    setSelectedKey(VIEW_KEYS.PROXY);
  };

  const handleBreakpointRequest = () => {
    if (!data) return;

    let urlPattern = data.url;
    try {
      const urlObj = new URL(data.url);
      urlPattern = `*${urlObj.pathname}*`;
    } catch {
      urlPattern = `*${data.url.split('?')[0]}*`;
    }

    openBreakpointWithPrefill({
      urlPattern,
      method: data.method,
      phase: 'both',
      description: `BP for ${data.method} ${urlPattern}`,
    });
    setSelectedKey(VIEW_KEYS.PROXY);
  };

  const renderData = () => {
    if (!event.data) return <Text type="secondary">No data</Text>;

    // Use specialized view for network events
    if (isNetworkEvent && data) {
      return <NetworkEventDetail data={data} token={token} externalSearchTerm={globalSearchText} />;
    }

    // Generic JSON view for other events
    return <JsonViewer data={data ?? event.data} fontSize={11} externalSearchTerm={globalSearchText} />;
  };

  // For network events, show a simplified header
  const getTitle = () => {
    if (isNetworkEvent && data) {
      const statusColor = data.statusCode >= 500 ? '#ff4d4f' :
        data.statusCode >= 400 ? '#faad14' :
        data.statusCode >= 300 ? '#1890ff' : '#52c41a';
      try {
        const urlPath = new URL(data.url).pathname;
        return (
          <Space size={4} wrap>
            <Tag color="blue" style={{ margin: 0 }}>{data.method}</Tag>
            <Tag color={statusColor} style={{ margin: 0 }}>{data.statusCode}</Tag>
            <Text style={{ fontSize: 12 }} ellipsis={{ tooltip: urlPath }}>{urlPath}</Text>
          </Space>
        );
      } catch {
        return <Text ellipsis>{event.title}</Text>;
      }
    }
    return (
      <Space size={4}>
        <span style={{ display: 'inline-flex', alignItems: 'center' }}>{getEventIcon(event)}</span>
        <Text ellipsis style={{ maxWidth: 280 }}>{event.title}</Text>
      </Space>
    );
  };

  return (
    <Card
      size="small"
      title={getTitle()}
      extra={
        <Space size={0}>
          {isNetworkEvent && data && (
            <>
              <Tooltip title={t('timeline.mock_request')}>
                <Button type="text" size="small" onClick={handleMockRequest}>
                  <BlockOutlined />
                </Button>
              </Tooltip>
              <Tooltip title={t('proxy.create_breakpoint')}>
                <Button type="text" size="small" onClick={handleBreakpointRequest}>
                  <BugOutlined />
                </Button>
              </Tooltip>
            </>
          )}
          <Button type="text" size="small" onClick={onClose}>
            <CloseOutlined />
          </Button>
        </Space>
      }
      style={{ height: '100%', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}
      styles={{ body: { flex: 1, overflow: 'hidden', padding: 0, display: 'flex', flexDirection: 'column', minHeight: 0 } }}
    >
      {!isNetworkEvent && (
        <div style={{ padding: '12px 12px 0', flexShrink: 0 }}>
          <Descriptions column={3} size="small" bordered style={{ marginBottom: 12 }}>
            <Descriptions.Item label="Time">
              <Text style={{ fontSize: 11 }}>
                {formatTimestamp(event.timestamp)} ({formatRelativeTime(event.relativeTime)})
              </Text>
            </Descriptions.Item>
            <Descriptions.Item label="Source">
              <Tag color={sourceConfig[event.source]?.color} style={{ margin: 0 }}>
                {sourceConfig[event.source]?.label || event.source}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="Level">
              <Tag color={levelConfig[event.level]?.color} style={{ margin: 0 }}>
                {levelConfig[event.level]?.label || event.level}
              </Tag>
            </Descriptions.Item>
            {event.duration && (
              <Descriptions.Item label="Duration">{event.duration}ms</Descriptions.Item>
            )}
          </Descriptions>
        </div>
      )}

      {isNetworkEvent ? (
        <div style={{ flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
          {renderData()}
        </div>
      ) : (
        <div style={{ flex: 1, overflow: 'auto', padding: '0 12px 12px', minHeight: 0 }}>
          <Text strong style={{ fontSize: 12 }}>Data</Text>
          <div style={{ marginTop: 8 }}>
            {renderData()}
          </div>
        </div>
      )}
    </Card>
  );
});

EventDetail.displayName = 'EventDetail';

// ========================================
// 筛选器弹出框
// ========================================

interface FilterPopoverProps {
  sources: EventSource[];
  categories: EventCategory[];
  levels: EventLevel[];
  onSourcesChange: (sources: EventSource[]) => void;
  onCategoriesChange: (categories: EventCategory[]) => void;
  onLevelsChange: (levels: EventLevel[]) => void;
}

const FilterPopover = memo(({
  sources,
  categories,
  levels,
  onSourcesChange,
  onCategoriesChange,
  onLevelsChange,
}: FilterPopoverProps) => {
  const { t } = useTranslation();
  const allSources: EventSource[] = ['logcat', 'network', 'device', 'app', 'ui', 'touch', 'workflow', 'perf', 'system', 'assertion', 'plugin'];
  const allCategories: EventCategory[] = ['log', 'network', 'state', 'interaction', 'automation', 'diagnostic', 'plugin'];
  const allLevels: EventLevel[] = ['verbose', 'debug', 'info', 'warn', 'error', 'fatal'];

  return (
    <div style={{ width: 300 }}>
      <div style={{ marginBottom: 12 }}>
        <Text strong style={{ display: 'block', marginBottom: 8 }}>{t('timeline.sources')}</Text>
        <Checkbox.Group
          value={sources}
          onChange={(v) => onSourcesChange(v as EventSource[])}
          options={allSources.map(s => ({
            label: <Tag color={sourceConfig[s]?.color}>{sourceConfig[s]?.label}</Tag>,
            value: s,
          }))}
          style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}
        />
      </div>

      <div style={{ marginBottom: 12 }}>
        <Text strong style={{ display: 'block', marginBottom: 8 }}>{t('timeline.categories')}</Text>
        <Checkbox.Group
          value={categories}
          onChange={(v) => onCategoriesChange(v as EventCategory[])}
          options={allCategories.map(c => ({
            label: <Tag color={categoryConfig[c]?.color}>{categoryConfig[c]?.label}</Tag>,
            value: c,
          }))}
          style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}
        />
      </div>

      <div>
        <Text strong style={{ display: 'block', marginBottom: 8 }}>{t('timeline.levels')}</Text>
        <Checkbox.Group
          value={levels}
          onChange={(v) => onLevelsChange(v as EventLevel[])}
          options={allLevels.map(l => ({
            label: <Tag color={levelConfig[l]?.color}>{levelConfig[l]?.label}</Tag>,
            value: l,
          }))}
          style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}
        />
      </div>
    </div>
  );
});

FilterPopover.displayName = 'FilterPopover';

// ========================================
// 主组件
// ========================================

const EventTimeline = () => {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const virtualListRef = useRef<VirtualListHandle>(null);

  // Device store
  const { devices, selectedDevice, setSelectedDevice } = useDeviceStore();

  // Event store
  const {
    sessions,
    sessionsVersion,
    activeSessionId,
    visibleEvents,
    networkEvents,
    totalEventCount,
    filteredEventCount,
    filter,
    timeIndex,
    isLoading,
    autoScroll,
    loadSession,
    startSession,
    endSession,
    setFilter,
    applyFilter,
    createBookmark,
    deleteBookmark,
    subscribeToEvents,
    setAutoScroll,
  } = useEventStore();

  // Use dedicated hook for bookmarks to ensure proper re-renders
  const sessionBookmarks = useCurrentBookmarks();

  // Event Timeline Store
  const {
    selectedEventId,
    selectedEventFull,
    detailOpen,
    searchText,
    filterSources,
    filterCategories,
    filterLevels,
    timeRange,
    currentTime,
    sessionList,
    assertionsPanelOpen,
    configModalOpen,
    bookmarkModalOpen,
    bookmarkLabel,
    setSelectedEventId,
    setSelectedEventFull,
    setDetailOpen,
    setSearchText,
    setFilterSources,
    setFilterCategories,
    setFilterLevels,
    setTimeRange,
    setCurrentTime,
    setSessionList,
    setAssertionsPanelOpen,
    setConfigModalOpen,
    setBookmarkModalOpen,
    setBookmarkLabel,
    openBookmarkModal,
    closeBookmarkModal,
    visualizationPanelOpen,
    visualizationActiveTab,
    setVisualizationPanelOpen,
    toggleVisualizationPanel,
    setVisualizationActiveTab,
  } = useEventTimelineStore();

  // Subscribe to events
  useEffect(() => {
    const unsubscribe = subscribeToEvents();
    return () => unsubscribe();
  }, [subscribeToEvents]);

  // Also listen for session changes to update the dropdown list
  useEffect(() => {
    const EventsOn = (window as any).runtime?.EventsOn;
    if (!EventsOn) return;

    let isMounted = true;
    const currentDevice = selectedDevice;

    const refreshSessionList = async () => {
      if (isMounted && currentDevice) {
        const list = await (window as any).go.main.App.ListStoredSessions(currentDevice, 50);
        if (isMounted) {
          setSessionList(list || []);
        }
      }
    };

    // 共享通道：用 EventsOn 返回的 cancel 函数精确退订自己的监听器，
    // EventsOff(name) 会误删 eventStore 等其他订阅方
    const offStarted = EventsOn('session-started', refreshSessionList);
    const offEnded = EventsOn('session-ended', refreshSessionList);

    return () => {
      isMounted = false;
      offStarted?.();
      offEnded?.();
    };
  }, [selectedDevice]);

  // Load sessions when device changes
  useEffect(() => {
    const loadSessions = async () => {
      if (selectedDevice) {
        const list = await (window as any).go.main.App.ListStoredSessions(selectedDevice, 50);
        setSessionList(list || []);
      } else {
        setSessionList([]);
      }
    };
    loadSessions();
  }, [selectedDevice]);

  // Get current session - use sessionsVersion to force recompute
  const currentSession = useMemo(() => {
    if (!activeSessionId) {
      return null;
    }
    const session = sessions.get(activeSessionId);
    return session || null;
  }, [sessions, activeSessionId, sessionsVersion]);

  // Session duration
  const sessionDuration = useMemo(() => {
    if (!currentSession) return 0;
    const end = currentSession.endTime || Date.now();
    return end - currentSession.startTime;
  }, [currentSession]);

  // Get time index for current session (returns array)
  const sessionTimeIndex = useMemo((): TimeIndexEntry[] => {
    if (!activeSessionId) return [];
    return timeIndex.get(activeSessionId) || [];
  }, [timeIndex, activeSessionId]);

  // Extract critical events for timeline markers (crashes, ANRs, errors)
  const criticalEvents = useMemo((): CriticalEvent[] => {
    if (!visibleEvents.length) return [];
    return visibleEvents
      .filter(e => 
        e.type === 'app_crash' || 
        e.type === 'app_anr' || 
        e.level === 'error' || 
        e.level === 'fatal'
      )
      .slice(0, 50) // 限制数量避免性能问题
      .map(e => ({
        id: e.id,
        relativeTime: e.relativeTime,
        type: e.type,
        title: e.title,
        level: e.level,
      }));
  }, [visibleEvents]);

  // Note: Auto-scroll is now handled by VirtualList component


  // Update current time based on scroll position - only when auto-scroll is enabled
  useEffect(() => {
    if (autoScroll && visibleEvents.length > 0) {
      const lastVisible = visibleEvents[visibleEvents.length - 1];
      if (lastVisible) {
        setCurrentTime(lastVisible.relativeTime);
      }
    }
  }, [visibleEvents, autoScroll]);

  // Skip initial mount for filter effects (loadSession already loaded all events)
  const filterMountedRef = useRef(false);
  const searchMountedRef = useRef(false);

  // Handle non-search filter changes (immediate)
  useEffect(() => {
    if (!filterMountedRef.current) {
      filterMountedRef.current = true;
      return;
    }
    setFilter({
      sources: filterSources.length > 0 ? filterSources : undefined,
      categories: filterCategories.length > 0 ? filterCategories : undefined,
      levels: filterLevels.length > 0 ? filterLevels : undefined,
      startTime: timeRange?.start,
      endTime: timeRange?.end,
    });
    applyFilter();
  }, [filterSources, filterCategories, filterLevels, timeRange, setFilter, applyFilter]);

  // Handle search text changes (debounced 300ms for FTS5 backend query)
  useEffect(() => {
    if (!searchMountedRef.current) {
      searchMountedRef.current = true;
      return;
    }
    const timer = setTimeout(() => {
      setFilter({ searchText: searchText || undefined });
      applyFilter();
    }, 300);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchText]); // Only depend on searchText to avoid breaking debounce

  // Scroll to selected event after filter changes
  useEffect(() => {
    if (selectedEventId && visibleEvents.length > 0) {
      const selectedIndex = visibleEvents.findIndex(e => e.id === selectedEventId);
      if (selectedIndex >= 0) {
        // Event is in the filtered list, scroll to it
        virtualListRef.current?.scrollToIndex(selectedIndex, { align: 'center' });
      }
    }
  }, [visibleEvents, selectedEventId]);

  // Scroll to first event when time range is selected
  useEffect(() => {
    if (timeRange && visibleEvents.length > 0 && !selectedEventId) {
      // Scroll to the first event in the filtered list
      virtualListRef.current?.scrollToIndex(0, { align: 'start' });
    }
  }, [timeRange, visibleEvents.length, selectedEventId]);

  // Handle time range change - scroll to start of range
  const handleTimeRangeChange = useCallback((range: { start: number; end: number } | null) => {
    setTimeRange(range);
    if (range) {
      // Update current time indicator to range start
      setCurrentTime(range.start);
      setAutoScroll(false);
    }
  }, [setAutoScroll]);

  // Handlers
  const handleSessionSelect = useCallback((sessionId: string) => {
    if (sessionId) {
      setTimeRange(null); // Clear time range when switching sessions
      loadSession(sessionId);
    }
  }, [loadSession]);

  const handleEventClick = useCallback(async (event: UnifiedEvent) => {
    setSelectedEventId(event.id);
    setDetailOpen(true);
    // 更新黄色指示器位置到该事件的时间点
    setCurrentTime(event.relativeTime);
    // 加载完整的事件数据（包含 data 字段）
    try {
      const fullEvent = await (window as any).go.main.App.GetStoredEvent(event.id);
      setSelectedEventFull(fullEvent);
    } catch (err) {
      console.error('[EventTimeline] Failed to load event details:', err);
      setSelectedEventFull(event); // 降级使用列表中的数据
    }
  }, []);

  // Handle critical event click from timeline marker - jump to event and select it
  const handleCriticalEventClick = useCallback((event: CriticalEvent) => {
    // 先找到完整事件数据
    const fullEvent = visibleEvents.find(e => e.id === event.id);
    if (fullEvent) {
      handleEventClick(fullEvent);
    }
  }, [visibleEvents, handleEventClick]);

  const handleSeek = useCallback((time: number) => {
    const targetTime = Math.round(time);
    setAutoScroll(false);
    // 立即更新黄线位置，提供即时视觉反馈
    setCurrentTime(targetTime);

    // 获取当前状态
    const { visibleEvents: currentEvents, activeSessionId, totalEventCount } = useEventStore.getState();

    // 二分查找并滚动的函数
    const scrollToTime = (events: UnifiedEvent[]) => {
      if (events.length === 0) return;

      let left = 0;
      let right = events.length - 1;

      while (left < right) {
        const mid = Math.floor((left + right) / 2);
        if (events[mid].relativeTime < targetTime) {
          left = mid + 1;
        } else {
          right = mid;
        }
      }

      let closestIndex = left;
      if (left > 0) {
        const diffLeft = Math.abs(events[left].relativeTime - targetTime);
        const diffPrev = Math.abs(events[left - 1].relativeTime - targetTime);
        if (diffPrev < diffLeft) {
          closestIndex = left - 1;
        }
      }

      const foundEvent = events[closestIndex];
      if (foundEvent) {
        setCurrentTime(foundEvent.relativeTime);
      }
      virtualListRef.current?.scrollToIndex(closestIndex, { align: 'center', behavior: 'smooth' });
    };

    // 检查是否需要重新加载
    const needReload = currentEvents.length === 0 ||
      currentEvents.length < totalEventCount * 0.9 ||
      (currentEvents.length > 0 && (
        targetTime < currentEvents[0].relativeTime - 1000 ||
        targetTime > currentEvents[currentEvents.length - 1].relativeTime + 1000
      ));

    if (needReload && activeSessionId) {
      // 需要重新加载，异步执行
      loadSession(activeSessionId).then(() => {
        const { visibleEvents: events } = useEventStore.getState();
        scrollToTime(events);
      });
    } else {
      // 直接用当前事件滚动
      scrollToTime(currentEvents);
    }
  }, [setAutoScroll, loadSession]);

  const handleOpenBookmarkModal = useCallback(() => {
    if (activeSessionId && typeof currentTime === 'number') {
      openBookmarkModal(`${t('timeline.bookmark')} ${formatRelativeTime(currentTime)}`);
    }
  }, [activeSessionId, currentTime, t, openBookmarkModal]);

  const handleConfirmBookmark = useCallback(() => {
    if (activeSessionId && typeof currentTime === 'number' && bookmarkLabel.trim()) {
      createBookmark(currentTime, bookmarkLabel.trim());
      closeBookmarkModal();
    }
  }, [activeSessionId, currentTime, bookmarkLabel, createBookmark, closeBookmarkModal]);

  const handleBookmarkClick = useCallback((bookmark: Bookmark) => {
    // Jump to the bookmark time
    setCurrentTime(bookmark.relativeTime);
    handleSeek(bookmark.relativeTime);
  }, [handleSeek]);

  const handleDeleteBookmark = useCallback((bookmarkId: string) => {
    deleteBookmark(bookmarkId);
  }, [deleteBookmark]);

  const handleStartSession = useCallback(() => {
    if (selectedDevice) {
      setConfigModalOpen(true);
    }
  }, [selectedDevice]);

  const handleStartWithConfig = useCallback(async (name: string, config: SessionConfig) => {
    setConfigModalOpen(false);
    if (selectedDevice) {
      try {
        const sessionId = await (window as any).go.main.App.StartSessionWithConfig(
          selectedDevice,
          name,
          config
        );
        // Reload the session to get the updated data
        if (sessionId) {
          await loadSession(sessionId);
          // Also refresh the session list so it appears in dropdown
          const list = await (window as any).go.main.App.ListStoredSessions(selectedDevice, 50);
          setSessionList(list || []);
        }
      } catch (err) {
        console.error('[EventTimeline] StartSessionWithConfig error:', err);
      }
    }
  }, [selectedDevice, loadSession]);

  const handleEndSession = useCallback(async () => {
    if (activeSessionId) {
      await endSession(activeSessionId, 'completed');
    }
  }, [activeSessionId, endSession]);

  // Device options
  const deviceOptions = useMemo(() =>
    devices.map(d => ({
      label: d.model || d.id,
      value: d.id,
    })),
    [devices]
  );

  // Handle ending a session from the dropdown
  const handleEndSessionById = useCallback(async (sessionId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    await endSession(sessionId, 'completed');
  }, [endSession]);

  // Session options
  const sessionOptions = useMemo(() =>
    sessionList.map(s => ({
      label: (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}>
          <Space>
            {s.status === 'active' ? <PlayCircleOutlined style={{ color: '#52c41a' }} /> : <HistoryOutlined />}
            <span>{s.name}</span>
            <Text type="secondary" style={{ fontSize: 10 }}>
              {new Date(s.startTime).toLocaleTimeString()}
            </Text>
          </Space>
          {s.status === 'active' && (
            <Tooltip title={t('timeline.end_session')}>
              <Button
                type="text"
                size="small"
                danger
                icon={<PauseCircleOutlined />}
                onClick={(e) => handleEndSessionById(s.id, e)}
                style={{ marginLeft: 8 }}
              />
            </Tooltip>
          )}
        </div>
      ),
      value: s.id,
    })),
    [sessionList, handleEndSessionById, t]
  );

  // Get active filter count
  const activeFilterCount = filterSources.length + filterCategories.length + filterLevels.length;

  return (
    <div style={{ flex: 1, display: 'flex', padding: 16, gap: 16, overflow: 'hidden', minHeight: 0 }}>
      {/* Main Timeline Area */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0, minHeight: 0, overflow: 'hidden' }}>
        {/* Header */}
        <div style={{ marginBottom: 12 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
            <Title level={4} style={{ margin: 0, color: token.colorText }}>
              <HistoryOutlined style={{ marginRight: 8 }} />
              {t('timeline.title')}
            </Title>

            <Space>
              <Button
                type={visualizationPanelOpen ? 'primary' : 'default'}
                icon={<BarChartOutlined />}
                onClick={toggleVisualizationPanel}
              >
                {t('timeline.visualization', 'Visualization')}
              </Button>
              <Button
                type={assertionsPanelOpen ? 'primary' : 'default'}
                icon={<ThunderboltOutlined />}
                onClick={() => setAssertionsPanelOpen(!assertionsPanelOpen)}
              >
                {t('timeline.assertions')}
              </Button>
              {currentSession?.status === 'active' ? (
                <Button
                  danger
                  icon={<PauseCircleOutlined />}
                  onClick={handleEndSession}
                >
                  {t('timeline.end_session')}
                </Button>
              ) : (
                <Button
                  type="primary"
                  icon={<PlayCircleOutlined />}
                  onClick={handleStartSession}
                  disabled={!selectedDevice}
                >
                  {t('timeline.start_session')}
                </Button>
              )}
            </Space>
          </div>

        {/* Selectors */}
        <Space wrap style={{ marginBottom: 12 }}>
          <Select
            style={{ width: 180 }}
            placeholder={t('timeline.select_device')}
            value={selectedDevice || undefined}
            onChange={setSelectedDevice}
            options={deviceOptions}
          />
          <Select
            style={{ width: 280 }}
            placeholder={t('timeline.select_session')}
            value={activeSessionId || undefined}
            onChange={handleSessionSelect}
            options={sessionOptions}
          />
        </Space>

        {/* Filters */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <Input
            prefix={isLoading && searchText ? <LoadingOutlined spin /> : <SearchOutlined />}
            placeholder={t('timeline.search_placeholder', 'Search events...')}
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            allowClear
            style={{ width: 280 }}
          />

          <Popover
            content={
              <FilterPopover
                sources={filterSources}
                categories={filterCategories}
                levels={filterLevels}
                onSourcesChange={setFilterSources}
                onCategoriesChange={setFilterCategories}
                onLevelsChange={setFilterLevels}
              />
            }
            trigger="click"
            placement="bottomLeft"
          >
            <Badge count={activeFilterCount} size="small">
              <Button icon={<FilterOutlined />}>{t('timeline.filters')}</Button>
            </Badge>
          </Popover>

          <Popover
            trigger="click"
            placement="bottomRight"
            content={
              <div style={{ width: 280 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                  <Text strong>{t('timeline.bookmarks')}</Text>
                  <Button
                    type="primary"
                    size="small"
                    icon={<PushpinOutlined />}
                    onClick={(e) => {
                      e.stopPropagation();
                      handleOpenBookmarkModal();
                    }}
                    disabled={!activeSessionId}
                  >
                    {t('timeline.add_bookmark')}
                  </Button>
                </div>
                {sessionBookmarks.length === 0 ? (
                  <Empty
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                    description={t('timeline.no_bookmarks')}
                    style={{ margin: '12px 0' }}
                  />
                ) : (
                  <List
                    size="small"
                    dataSource={sessionBookmarks}
                    style={{ maxHeight: 200, overflow: 'auto' }}
                    renderItem={item => (
                      <List.Item
                        style={{ padding: '4px 0', cursor: 'pointer' }}
                        onClick={() => handleBookmarkClick(item)}
                        actions={[
                          <Tooltip key="delete" title={t('common.delete')}>
                            <Button
                              type="text"
                              size="small"
                              icon={<DeleteOutlined />}
                              onClick={(e) => {
                                e.stopPropagation();
                                handleDeleteBookmark(item.id);
                              }}
                              danger
                            />
                          </Tooltip>
                        ]}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <PushpinOutlined style={{ color: item.color || '#52c41a' }} />
                          <div>
                            <Text ellipsis style={{ maxWidth: 150 }}>{item.label}</Text>
                            <br />
                            <Text type="secondary" style={{ fontSize: 11 }}>
                              {formatRelativeTime(item.relativeTime)}
                            </Text>
                          </div>
                        </div>
                      </List.Item>
                    )}
                  />
                )}
              </div>
            }
          >
            <Badge count={sessionBookmarks.length} size="small" offset={[-5, 5]}>
              <Button icon={<BookOutlined />}>
                {t('timeline.bookmark')}
              </Button>
            </Badge>
          </Popover>

          <div style={{ flex: 1 }} />

          <Segmented
            value={autoScroll ? 'live' : 'manual'}
            onChange={v => setAutoScroll(v === 'live')}
            options={[
              { label: t('timeline.live'), value: 'live', icon: <PlayCircleOutlined /> },
              { label: t('timeline.manual'), value: 'manual', icon: <PauseCircleOutlined /> },
            ]}
          />

          <Button
            icon={<VerticalAlignBottomOutlined />}
            onClick={() => {
              if (visibleEvents.length > 0) {
                virtualListRef.current?.scrollToIndex(visibleEvents.length - 1, { align: 'end' });
              }
            }}
          >
            {t('timeline.bottom')}
          </Button>
        </div>
      </div>

      {/* Time Ruler */}
      {currentSession && (
        <TimeRuler
          sessionStart={currentSession.startTime}
          sessionDuration={sessionDuration}
          timeIndex={sessionTimeIndex}
          currentTime={currentTime}
          onSeek={handleSeek}
          timeRange={timeRange}
          onTimeRangeChange={handleTimeRangeChange}
          bookmarks={sessionBookmarks}
          onBookmarkClick={handleBookmarkClick}
          criticalEvents={criticalEvents}
          onCriticalEventClick={handleCriticalEventClick}
        />
      )}

      {/* Visualization Panel */}
      {visualizationPanelOpen && currentSession && (
        <div style={{ marginTop: 12 }}>
          <Tabs
            activeKey={visualizationActiveTab}
            onChange={setVisualizationActiveTab}
            size="small"
            items={[
              {
                key: 'stats',
                label: t('timeline.stats', 'Statistics'),
                children: (
                  <SessionStats
                    events={visibleEvents}
                    timeIndex={sessionTimeIndex}
                    sessionDuration={sessionDuration}
                  />
                ),
              },
              {
                key: 'waterfall',
                label: t('timeline.waterfall', 'Network Waterfall'),
                children: (
                  <NetworkWaterfall
                    events={networkEvents}
                    sessionStart={currentSession.startTime}
                    sessionDuration={sessionDuration}
                    onEventClick={handleEventClick}
                    maxHeight={300}
                  />
                ),
              },
              {
                key: 'lanes',
                label: t('timeline.lanes', 'Event Lanes'),
                children: (
                  <EventLanes
                    events={visibleEvents}
                    sessionDuration={sessionDuration}
                    onEventClick={handleEventClick}
                    height={250}
                  />
                ),
              },
            ]}
          />
        </div>
      )}

      {/* Event count */}
      <div style={{ padding: '8px 0', display: 'flex', alignItems: 'center', gap: 12 }}>
        <Text type="secondary">
          {filteredEventCount} / {totalEventCount} events
        </Text>
        {currentSession?.status === 'active' && (
          <Badge status="processing" text={t('timeline.recording')} />
        )}
      </div>

      {/* Event List and Detail Panel */}
      <div style={{ flex: 1, display: 'flex', gap: 12, overflow: 'hidden', minHeight: 0 }}>
        {/* Event List */}
        <Card
          size="small"
          style={{ flex: 1, overflow: 'hidden', minWidth: 0, minHeight: 0 }}
          styles={{ body: { padding: 0, height: '100%', minHeight: 0 } }}
        >
          <VirtualList
            ref={virtualListRef}
            dataSource={visibleEvents}
            rowKey="id"
            rowHeight={36}
            loading={isLoading}
            loadingText={t('timeline.loading_events', 'Loading events...')}
            emptyText={t('timeline.no_events')}
            autoScroll={autoScroll}
            onAutoScrollChange={setAutoScroll}
            onReachTop={undefined}
            onReachBottom={undefined}
            selectedKey={selectedEventId}
            onItemClick={handleEventClick}
            style={{ height: '100%' }}
            renderItem={(event, index, isSelected) => (
              <EventRow
                event={event}
                isSelected={isSelected}
                onClick={() => {}}
                style={{ height: '100%' }}
              />
            )}
          />
        </Card>

        {/* Event Detail Panel */}
        {selectedEventFull && (
          <div style={{ width: '50%', minWidth: 500, flexShrink: 0, overflow: 'hidden', height: '100%' }}>
            <EventDetail
              event={selectedEventFull}
              onClose={() => {
                setDetailOpen(false);
                setSelectedEventId(null);
                setSelectedEventFull(null);
              }}
              globalSearchText={searchText}
            />
          </div>
        )}
      </div>

        {/* Session Config Modal */}
        <SessionConfigModal
          open={configModalOpen}
          onCancel={() => setConfigModalOpen(false)}
          onStart={handleStartWithConfig}
          deviceId={selectedDevice || ''}
        />

        {/* Bookmark Modal */}
        <Modal
          title={t('timeline.add_bookmark')}
          open={bookmarkModalOpen}
          onCancel={closeBookmarkModal}
          onOk={handleConfirmBookmark}
          okText={t('common.ok')}
          cancelText={t('common.cancel')}
          width={400}
        >
          <div style={{ marginBottom: 16 }}>
            <Text type="secondary">
              {t('timeline.bookmark_time')}: {formatRelativeTime(currentTime)}
            </Text>
          </div>
          <Input
            placeholder={t('timeline.bookmark_label_placeholder')}
            value={bookmarkLabel}
            onChange={(e) => setBookmarkLabel(e.target.value)}
            onPressEnter={handleConfirmBookmark}
            autoFocus
          />
        </Modal>
      </div>

      {/* Assertions Panel */}
      {assertionsPanelOpen && (
        <div style={{ width: 320, flexShrink: 0 }}>
          <AssertionsPanel
            sessionId={activeSessionId || ''}
            deviceId={selectedDevice || ''}
          />
        </div>
      )}
    </div>
  );
};

export default EventTimeline;
