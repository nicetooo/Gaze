/**
 * Event Store - 新事件系统的前端状态管理
 *
 * 特点:
 * - 全量主表事件加载 (不含 event_data)，客户端过滤
 * - 独立加载全量网络事件 (含 event_data)，供瀑布图使用
 * - 点击事件按需加载 event_data 详情
 * - 与后端 SQLite 存储对接
 * - 支持虚拟滚动的数据加载
 */

import { useMemo } from 'react';
import { create } from 'zustand';
import { immer } from 'zustand/middleware/immer';
import { enableMapSet } from 'immer';
import {
  UnifiedEvent,
  DeviceSession,
  EventQuery,
  TimeIndexEntry,
  Bookmark,
} from './eventTypes';

// Enable Map/Set support in Immer
enableMapSet();

const EventsOn = (window as any).runtime?.EventsOn;

// ========================================
// Store State & Actions
// ========================================

interface EventStoreState {
  // Sessions
  sessions: Map<string, DeviceSession>;
  sessionsVersion: number;  // Force re-render when sessions change
  activeSessionId: string | null;
  activeDeviceId: string | null;

  // Events - 全量主表事件 (不含 event_data)
  allEvents: UnifiedEvent[];

  // 时间索引
  timeIndex: Map<string, TimeIndexEntry[]>;

  // 书签
  bookmarks: Map<string, Bookmark[]>;
  bookmarksVersion: number;  // Force re-render when bookmarks change

  // 当前视图
  visibleRange: { start: number; end: number };
  visibleEvents: UnifiedEvent[];  // 经过 filter 后的事件 (渲染用)

  // Network events (always unfiltered by source, for waterfall chart, 含 event_data)
  networkEvents: UnifiedEvent[];

  // 筛选条件
  filter: EventQuery;

  // 加载状态
  isLoading: boolean;
  totalEventCount: number;    // session 的总事件数
  filteredEventCount: number; // 匹配过滤条件的事件数

  // UI 状态
  selectedEventId: string | null;
  isTimelineOpen: boolean;
  autoScroll: boolean;
}

interface EventStoreActions {
  // Session 管理
  setActiveSession: (sessionId: string | null) => void;
  setActiveDevice: (deviceId: string) => void;
  loadSession: (sessionId: string) => Promise<void>;
  startSession: (deviceId: string, type: string, name: string) => Promise<string>;
  endSession: (sessionId: string, status: string) => Promise<void>;
  deleteSession: (sessionId: string) => Promise<void>;
  loadSessions: (deviceId?: string, limit?: number) => Promise<DeviceSession[]>;

  // 实时事件
  receiveLiveEvents: (events: UnifiedEvent[]) => void;

  // 按需加载
  loadEventsInRange: (startTime: number, endTime: number) => Promise<void>;
  searchEvents: (query: string) => Promise<UnifiedEvent[]>;
  loadEvent: (eventId: string) => Promise<UnifiedEvent | null>;

  // 时间跳转
  jumpToTime: (relativeTime: number) => Promise<void>;
  jumpToEvent: (eventId: string) => Promise<void>;

  // 筛选
  setFilter: (filter: Partial<EventQuery>) => void;
  clearFilter: () => void;
  applyFilter: () => Promise<void>;

  // 可视区域
  setVisibleRange: (start: number, end: number) => void;

  // 时间索引
  loadTimeIndex: (sessionId: string) => Promise<void>;

  // 书签
  loadBookmarks: (sessionId: string) => Promise<void>;
  createBookmark: (relativeTime: number, label: string, color?: string, type?: string) => Promise<void>;
  deleteBookmark: (bookmarkId: string) => Promise<void>;

  // UI
  selectEvent: (eventId: string | null) => void;
  openTimeline: () => void;
  closeTimeline: () => void;
  setAutoScroll: (enabled: boolean) => void;

  // 统计
  getSessionStats: (sessionId: string) => Promise<Record<string, any>>;
  getSystemStats: () => Promise<Record<string, any>>;

  // 清理
  clearSession: () => void;
  cleanup: (maxAgeDays: number) => Promise<number>;

  // 事件订阅
  subscribeToEvents: () => () => void;
}

// ========================================
// Store 实现
// ========================================

export const useEventStore = create<EventStoreState & EventStoreActions>()(
  immer((set, get) => ({
    // 初始状态
    sessions: new Map(),
    sessionsVersion: 0,
    activeSessionId: null,
    activeDeviceId: null,
    allEvents: [],
    timeIndex: new Map(),
    bookmarks: new Map(),
    bookmarksVersion: 0,
    visibleRange: { start: 0, end: 60000 }, // 默认显示前 60 秒
    visibleEvents: [],
    networkEvents: [],
    filter: {},
    isLoading: false,
    totalEventCount: 0,
    filteredEventCount: 0,
    selectedEventId: null,
    isTimelineOpen: false,
    autoScroll: true,

    // ========================================
    // Session 管理
    // ========================================

    setActiveSession: (sessionId) => {
      set(state => {
        state.activeSessionId = sessionId;
        if (!sessionId) {
          state.allEvents = [];
          state.visibleEvents = [];
          state.networkEvents = [];
          state.totalEventCount = 0;
        }
      });
    },

    setActiveDevice: (deviceId) => {
      set(state => {
        state.activeDeviceId = deviceId;
      });
    },

    loadSession: async (sessionId) => {
      console.log('[eventStore] loadSession called:', sessionId);

      // Reset state for new session
      const currentState = get();
      set(state => {
        state.isLoading = true;
        state.activeSessionId = sessionId;
        state.allEvents = [];
        state.visibleEvents = [];
        state.networkEvents = [];
        state.visibleRange = { start: 0, end: 60000 };
        state.filter = {}; // 重置过滤器
        state.sessionsVersion = currentState.sessionsVersion + 1;
      });

      try {
        // 加载 session 信息
        console.log('[eventStore] loadSession calling GetStoredSession...');
        const session = await (window as any).go.main.App.GetStoredSession(sessionId);
        console.log('[eventStore] loadSession got session:', session);

        if (session) {
          // Create new Map to avoid immer issues
          const newSessions = new Map(get().sessions);
          newSessions.set(sessionId, session);

          set(state => {
            state.sessions = newSessions;
            state.activeDeviceId = session.deviceId;
            state.totalEventCount = session.eventCount || 0;
          });
        }

        // 加载时间索引和书签（不阻塞事件加载）
        Promise.all([
          (window as any).go.main.App.GetSessionTimeIndex(sessionId).catch(() => []),
          (window as any).go.main.App.GetSessionBookmarks(sessionId).catch(() => []),
        ]).then(([index, bookmarks]) => {
          console.log('[eventStore] Received timeIndex:', index?.length || 0, 'entries');
          if (index && index.length > 0) {
            const minSec = Math.min(...index.map((e: any) => e.second));
            const maxSec = Math.max(...index.map((e: any) => e.second));
            const total = index.reduce((s: number, e: any) => s + e.eventCount, 0);
            console.log('[eventStore] TimeIndex range:', minSec, '-', maxSec, 'total events:', total);
          }

          const currentTimeIndex = new Map(get().timeIndex);
          const currentBookmarks = new Map(get().bookmarks);
          currentTimeIndex.set(sessionId, index || []);
          currentBookmarks.set(sessionId, bookmarks || []);

          set(state => {
            state.timeIndex = currentTimeIndex;
            state.bookmarks = currentBookmarks;
          });
        }).catch(err => {
          console.error('[eventStore] loadSession index/bookmarks error:', err);
        });

        // 全量加载主表事件 (不含 event_data) + 全量网络事件 (含 event_data，供瀑布图)
        console.log('[eventStore] loadSession calling QuerySessionEvents...');
        const [result, networkResult] = await Promise.all([
          (window as any).go.main.App.QuerySessionEvents({
            sessionId: sessionId,
            limit: 0,             // 全量加载
            includeData: false,   // 不 JOIN event_data
          }),
          (window as any).go.main.App.QuerySessionEvents({
            sessionId: sessionId,
            sources: ['network'],
            limit: 0,             // 瀑布图需要全量网络事件
            includeData: true,    // 瀑布图需要 event_data
          }),
        ]);

        const events = result?.events || [];
        const networkEvents = networkResult?.events || [];
        console.log('[eventStore] loadSession loaded events:', events.length, '/ total:', result?.total || 0);

        set(state => {
          state.allEvents = events;
          state.visibleEvents = events; // 初始无 filter，显示全部
          state.networkEvents = networkEvents;
          state.totalEventCount = events.length;
          state.filteredEventCount = events.length;
          state.isLoading = false;
        });

      } catch (err) {
        console.error('[eventStore] loadSession error:', err);
        set(state => {
          state.isLoading = false;
        });
      }
    },

    startSession: async (deviceId, type, name) => {
      console.log('[eventStore] startSession called:', { deviceId, type, name });
      try {
        const sessionId = await (window as any).go.main.App.StartNewSession(deviceId, type, name);
        console.log('[eventStore] StartNewSession returned:', sessionId);

        // Get session from backend
        const session = await (window as any).go.main.App.GetStoredSession(sessionId);
        console.log('[eventStore] GetStoredSession returned:', session);

        // Get current state
        const currentState = get();
        console.log('[eventStore] Current state before update:', {
          sessionsSize: currentState.sessions.size,
          sessionsVersion: currentState.sessionsVersion
        });

        // Create new sessions map
        const newSessions = new Map(currentState.sessions);
        if (session) {
          newSessions.set(sessionId, session);
        }

        // Update state
        set((state) => {
          state.sessions = newSessions;
          state.activeSessionId = sessionId;
          state.activeDeviceId = deviceId;
          state.allEvents = [];
          state.visibleEvents = [];
          state.networkEvents = [];
          state.totalEventCount = 0;
          state.sessionsVersion = currentState.sessionsVersion + 1;
        });

        // Verify update
        const newState = get();
        console.log('[eventStore] State after update:', {
          activeSessionId: newState.activeSessionId,
          sessionsSize: newState.sessions.size,
          sessionsVersion: newState.sessionsVersion,
          sessionInMap: newState.sessions.has(sessionId)
        });

        return sessionId;
      } catch (err) {
        console.error('[eventStore] startSession error:', err);
        throw err;
      }
    },

    endSession: async (sessionId, status) => {
      await (window as any).go.main.App.EndActiveSession(sessionId, status);

      const { activeSessionId } = get();
      if (activeSessionId === sessionId) {
        set(state => {
          state.activeSessionId = null;
        });
      }
    },

    deleteSession: async (sessionId) => {
      await (window as any).go.main.App.DeleteStoredSession(sessionId);

      set(state => {
        state.sessions.delete(sessionId);
        state.timeIndex.delete(sessionId);
        state.bookmarks.delete(sessionId);

        if (state.activeSessionId === sessionId) {
          state.activeSessionId = null;
          state.allEvents = [];
          state.visibleEvents = [];
          state.networkEvents = [];
        }
      });
    },

    loadSessions: async (deviceId, limit = 50) => {
      const sessions = await (window as any).go.main.App.ListStoredSessions(deviceId || '', limit);

      set(state => {
        (sessions || []).forEach((session: DeviceSession) => {
          state.sessions.set(session.id, session);
        });
      });

      return sessions || [];
    },

    // ========================================
    // 实时事件
    // ========================================

    receiveLiveEvents: (events) => {
      const { activeSessionId, filter, autoScroll, visibleRange } = get();

      if (!activeSessionId || events.length === 0) return;

      // 只处理当前 session 的事件
      const relevantEvents = events.filter(e => e.sessionId === activeSessionId);
      if (relevantEvents.length === 0) return;

      set(state => {
        // 追加到全量事件
        state.allEvents = [...state.allEvents, ...relevantEvents];
        state.totalEventCount = state.allEvents.length;

        // 过滤新事件并追加到 visibleEvents
        const newFiltered = filterEvents(relevantEvents, filter);
        state.filteredEventCount += newFiltered.length;

        if (newFiltered.length > 0) {
          // 如果自动滚动，更新可视范围到最新
          if (autoScroll) {
            const lastEvent = relevantEvents[relevantEvents.length - 1];
            const newEnd = lastEvent.relativeTime + 5000;
            const windowSize = visibleRange.end - visibleRange.start;
            state.visibleRange = {
              start: Math.max(0, newEnd - windowSize),
              end: newEnd,
            };
          }

          // 直接追加所有匹配的新事件
          state.visibleEvents = [...state.visibleEvents, ...newFiltered];
        }

        // Always append network events regardless of source filter
        // The waterfall chart needs full data
        const newNetworkEvents = relevantEvents.filter(e => e.source === 'network');
        if (newNetworkEvents.length > 0) {
          state.networkEvents = [...state.networkEvents, ...newNetworkEvents];
        }
      });
    },

    // ========================================
    // 按需加载
    // ========================================

    loadEventsInRange: async (startTime, endTime) => {
      const { allEvents, filter } = get();
      const start = Math.round(startTime);
      const end = Math.round(endTime);
      console.log('[eventStore] loadEventsInRange:', { start, end });

      // 从内存中的 allEvents 按时间范围过滤
      const rangeFilter: EventQuery = {
        ...filter,
        startTime: start,
        endTime: end,
      };
      const filtered = filterEvents(allEvents, rangeFilter);

      set(state => {
        state.visibleEvents = filtered;
        state.filteredEventCount = filtered.length;
      });
    },

    searchEvents: async (query) => {
      const { activeSessionId } = get();
      if (!activeSessionId) return [];

      const result = await (window as any).go.main.App.QuerySessionEvents({
        sessionId: activeSessionId,
        searchText: query,
        limit: 100,
        includeData: false,
      });

      return result?.events || [];
    },

    loadEvent: async (eventId) => {
      const event = await (window as any).go.main.App.GetStoredEvent(eventId);
      return event || null;
    },

    // ========================================
    // 时间跳转
    // ========================================

    jumpToTime: async (relativeTime) => {
      const { activeSessionId } = get();
      if (!activeSessionId) return;

      set(state => {
        state.autoScroll = false; // 手动跳转时关闭自动滚动
      });

      // visibleEvents 已经是全量的，不需要重新加载
      // 返回后由 EventTimeline 根据 relativeTime 滚动到对应位置
    },

    jumpToEvent: async (eventId) => {
      const { loadEvent, jumpToTime } = get();
      const event = await loadEvent(eventId);

      if (event) {
        await jumpToTime(event.relativeTime);
        set(state => {
          state.selectedEventId = eventId;
        });
      }
    },

    // ========================================
    // 筛选
    // ========================================

    setFilter: (newFilter) => {
      set(state => {
        state.filter = { ...state.filter, ...newFilter };
      });
    },

    clearFilter: () => {
      set(state => {
        state.filter = {};
      });
      get().applyFilter();
    },

    applyFilter: async () => {
      const { activeSessionId, filter } = get();

      if (!activeSessionId) return;

      set(state => {
        state.isLoading = true;
      });

      try {
        // 统一走后端查询: limit:0 返回全量匹配, includeData:false 不加载 event_data
        // 保留 FTS5 全文搜索能力
        const [result, networkResult] = await Promise.all([
          (window as any).go.main.App.QuerySessionEvents({
            sessionId: activeSessionId,
            ...filter,
            limit: 0,
            includeData: false,
          }),
          (window as any).go.main.App.QuerySessionEvents({
            sessionId: activeSessionId,
            sources: ['network'],
            limit: 0,
            includeData: true,  // 瀑布图需要 event_data
          }),
        ]);

        const filteredEvents = result?.events || [];
        const networkEvents: UnifiedEvent[] = networkResult?.events || [];

        // 同时刷新 allEvents（无 filter 的全量数据）
        // 如果当前有 filter，需要单独获取全量 total
        const hasFilter = filter.sources?.length || filter.categories?.length ||
          filter.levels?.length || filter.types?.length ||
          filter.searchText || filter.startTime || filter.endTime || filter.stepId;

        let allEventsTotal = get().allEvents.length;
        if (hasFilter) {
          // allEvents 已经在内存中，totalEventCount 用它的长度
          allEventsTotal = get().allEvents.length;
        } else {
          // 无 filter 时，filteredEvents 就是全量，同步更新 allEvents
          set(state => {
            state.allEvents = filteredEvents;
          });
          allEventsTotal = filteredEvents.length;
        }

        set(state => {
          state.visibleEvents = filteredEvents;
          state.networkEvents = networkEvents;
          state.totalEventCount = allEventsTotal;
          state.filteredEventCount = result?.total || filteredEvents.length;
          state.isLoading = false;
        });
      } catch (err) {
        console.error('[eventStore] applyFilter error:', err);
        set(state => {
          state.isLoading = false;
        });
      }
    },

    // ========================================
    // 可视区域
    // ========================================

    setVisibleRange: (start, end) => {
      set(state => {
        state.visibleRange = { start, end };
      });
    },

    // ========================================
    // 时间索引
    // ========================================

    loadTimeIndex: async (sessionId) => {
      const index = await (window as any).go.main.App.GetSessionTimeIndex(sessionId);

      set(state => {
        state.timeIndex.set(sessionId, index || []);
      });
    },

    // ========================================
    // 书签
    // ========================================

    loadBookmarks: async (sessionId) => {
      const newBookmarks = await (window as any).go.main.App.GetSessionBookmarks(sessionId);
      const bookmarksArray = newBookmarks || [];

      set(state => {
        state.bookmarks.set(sessionId, bookmarksArray);
        state.bookmarksVersion++;
      });
    },

    createBookmark: async (relativeTime, label, color, type = 'user') => {
      const { activeSessionId } = get();
      if (!activeSessionId) return;

      await (window as any).go.main.App.CreateSessionBookmark(
        activeSessionId,
        relativeTime,
        label,
        color || '',
        type
      );

      // Reload bookmarks after creation
      const newBookmarks = await (window as any).go.main.App.GetSessionBookmarks(activeSessionId);
      set(state => {
        state.bookmarks.set(activeSessionId, newBookmarks || []);
        state.bookmarksVersion++;
      });
    },

    deleteBookmark: async (bookmarkId) => {
      const { activeSessionId } = get();

      await (window as any).go.main.App.DeleteSessionBookmark(bookmarkId);

      if (activeSessionId) {
        // Reload bookmarks after deletion
        const newBookmarks = await (window as any).go.main.App.GetSessionBookmarks(activeSessionId);
        set(state => {
          state.bookmarks.set(activeSessionId, newBookmarks || []);
          state.bookmarksVersion++;
        });
      }
    },

    // ========================================
    // UI
    // ========================================

    selectEvent: (eventId) => {
      set(state => {
        state.selectedEventId = eventId;
      });
    },

    openTimeline: () => {
      set(state => {
        state.isTimelineOpen = true;
      });
    },

    closeTimeline: () => {
      set(state => {
        state.isTimelineOpen = false;
      });
    },

    setAutoScroll: (enabled) => {
      set(state => {
        state.autoScroll = enabled;
      });
    },

    // ========================================
    // 统计
    // ========================================

    getSessionStats: async (sessionId) => {
      return await (window as any).go.main.App.GetSessionStats(sessionId) || {};
    },

    getSystemStats: async () => {
      return await (window as any).go.main.App.GetEventSystemStats() || {};
    },

    // ========================================
    // 清理
    // ========================================

    clearSession: () => {
      set(state => {
        state.activeSessionId = null;
        state.allEvents = [];
        state.visibleEvents = [];
        state.networkEvents = [];
        state.totalEventCount = 0;
        state.selectedEventId = null;
      });
    },

    cleanup: async (maxAgeDays) => {
      const removed = await (window as any).go.main.App.CleanupOldSessionData(maxAgeDays);
      return removed || 0;
    },

    // ========================================
    // 事件订阅
    // ========================================

    subscribeToEvents: () => {
      if (!EventsOn) {
        console.warn('EventsOn not available');
        return () => {};
      }

      const { receiveLiveEvents, loadSessions } = get();

      // 处理新事件批次
      const handleEventsBatch = (events: UnifiedEvent[]) => {
        receiveLiveEvents(events);
      };

      // 处理 session 启动
      const handleSessionStarted = (session: DeviceSession) => {
        set(state => {
          state.sessions.set(session.id, session);
          // 如果是当前设备的 session，自动切换
          if (state.activeDeviceId === session.deviceId) {
            state.activeSessionId = session.id;
            state.allEvents = [];
            state.visibleEvents = [];
            state.networkEvents = [];
            state.totalEventCount = 0;
          }
        });
      };

      // 处理 session 结束
      const handleSessionEnded = (session: DeviceSession) => {
        set(state => {
          state.sessions.set(session.id, session);
        });
      };

      // 订阅事件 (统一使用 session-events-batch)
      // 共享通道必须用 EventsOn 返回的 cancel 函数精确退订，
      // EventsOff(name) 会误删 logcatStore/perfStore/ProxyView 的监听器
      const offBatch = EventsOn('session-events-batch', handleEventsBatch);
      const offStarted = EventsOn('session-started', handleSessionStarted);
      const offEnded = EventsOn('session-ended', handleSessionEnded);

      // 返回取消订阅函数
      return () => {
        offBatch?.();
        offStarted?.();
        offEnded?.();
      };
    },
  }))
);

// ========================================
// Helper Functions
// ========================================

function filterEvents(events: UnifiedEvent[], filter: EventQuery): UnifiedEvent[] {
  return events.filter(event => {
    if (filter.sources?.length && !filter.sources.includes(event.source)) {
      return false;
    }
    if (filter.categories?.length && !filter.categories.includes(event.category)) {
      return false;
    }
    if (filter.levels?.length && !filter.levels.includes(event.level)) {
      return false;
    }
    if (filter.types?.length && !filter.types.includes(event.type)) {
      return false;
    }
    if (filter.startTime && event.relativeTime < filter.startTime) {
      return false;
    }
    if (filter.endTime && event.relativeTime > filter.endTime) {
      return false;
    }
    if (filter.stepId && event.stepId !== filter.stepId) {
      return false;
    }
    if (filter.searchText) {
      const search = filter.searchText.toLowerCase();
      // Search in title and summary
      if (event.title.toLowerCase().includes(search)) return true;
      if (event.summary?.toLowerCase().includes(search)) return true;
      // Deep search in event data (when available from live events)
      if (event.data) {
        try {
          const dataStr = typeof event.data === 'string' ? event.data : JSON.stringify(event.data);
          if (dataStr.toLowerCase().includes(search)) return true;
        } catch {
          // ignore serialization errors
        }
      }
      return false;
    }
    return true;
  });
}

// ========================================
// Hooks
// ========================================

/**
 * 获取当前 session 的书签
 * Uses bookmarksVersion in selector to ensure re-renders on bookmark changes
 */
export function useCurrentBookmarks(): Bookmark[] {
  const version = useEventStore(state => state.bookmarksVersion);
  const activeSessionId = useEventStore(state => state.activeSessionId);
  const bookmarksMap = useEventStore(state => state.bookmarks);

  return useMemo(() => {
    if (!activeSessionId) return [];
    return bookmarksMap.get(activeSessionId) || [];
  }, [activeSessionId, bookmarksMap, version]);
}


