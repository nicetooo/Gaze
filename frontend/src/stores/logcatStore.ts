import { create } from 'zustand';
import { immer } from 'zustand/middleware/immer';
import { StartLogcat, StopLogcat } from '../../wailsjs/go/main/App';
// @ts-ignore
import { main } from '../types/wails-models';

export interface FilterPreset {
  id: string;
  name: string;
  pattern: string;
  isRegex: boolean;
}

// 结构化日志数据
export interface ParsedLog {
  id: string;           // 唯一标识
  raw: string;          // 原始文本
  timestamp: string;    // "12:34:56.789"
  date: string;         // "01-04"
  level: 'V' | 'D' | 'I' | 'W' | 'E' | 'F';
  tag: string;          // Tag 名称
  message: string;      // 消息内容
  pid?: number;         // 进程 ID
  packageName?: string; // 包名
}

// 从原始日志行解析时间部分
function extractTimeFromRaw(raw: string): { date: string; timestamp: string } {
  // 格式: "01-04 12:34:56.789 ..."
  const match = raw.match(/^(\d{2}-\d{2})\s+(\d{2}:\d{2}:\d{2}\.\d{3})/);
  if (match) {
    return { date: match[1], timestamp: match[2] };
  }
  return { date: '', timestamp: '' };
}

// 从原始日志行解析 PID
function extractPidFromRaw(raw: string): number | undefined {
  // 格式: "... D/Tag( 1234): ..." 或 "... D/Tag(1234): ..."
  const match = raw.match(/\(\s*(\d+)\s*\):/);
  if (match) {
    return parseInt(match[1], 10);
  }
  return undefined;
}

// 生成唯一 ID
let logIdCounter = 0;
function generateLogId(): string {
  return `log-${Date.now()}-${logIdCounter++}`;
}

// @ts-ignore
const EventsOn = (window as any).runtime.EventsOn;

interface LogcatState {
  // State
  logs: ParsedLog[];
  isLogging: boolean;
  selectedPackage: string;

  // Selection state for detail panel
  selectedLogId: string | null;
  detailPanelOpen: boolean;
  activeDetailTab: 'message' | 'raw';  // Tab state for detail panel

  // Filter states
  logFilter: string;
  useRegex: boolean;
  preFilter: string;
  preUseRegex: boolean;
  excludeFilter: string;
  excludeUseRegex: boolean;

  // Persistent Filter Presets Selection
  selectedPresetIds: string[];
  selectedExcludePresetIds: string[];

  // UI State
  packages: main.AppPackage[];
  appRunningStatus: boolean;
  autoScroll: boolean;
  levelFilter: string[];
  matchCase: boolean;
  matchWholeWord: boolean;
  savedFilters: FilterPreset[];
  
  // Modal state
  isSaveModalOpen: boolean;
  newFilterName: string;
  
  // AI Search state
  isAIParsing: boolean;
  aiSearchText: string;
  aiPopoverOpen: boolean;

  // Actions
  setLogs: (logs: ParsedLog[] | ((prev: ParsedLog[]) => ParsedLog[])) => void;
  clearLogs: () => void;
  
  // Selection actions
  setSelectedLogId: (id: string | null) => void;
  setDetailPanelOpen: (open: boolean) => void;
  setActiveDetailTab: (tab: 'message' | 'raw') => void;
  selectLog: (id: string) => void;  // 选中日志并打开详情
  closeDetail: () => void;          // 关闭详情面板
  setSelectedPackage: (pkg: string) => void;
  setLogFilter: (filter: string) => void;
  setUseRegex: (useRegex: boolean) => void;
  setPreFilter: (filter: string) => void;
  setPreUseRegex: (useRegex: boolean) => void;
  setExcludeFilter: (filter: string) => void;
  setExcludeUseRegex: (useRegex: boolean) => void;

  setSelectedPresetIds: (ids: string[] | ((prev: string[]) => string[])) => void;
  setSelectedExcludePresetIds: (ids: string[] | ((prev: string[]) => string[])) => void;
  
  setPackages: (packages: main.AppPackage[]) => void;
  setAppRunningStatus: (status: boolean) => void;
  setAutoScroll: (autoScroll: boolean) => void;
  setLevelFilter: (filter: string[]) => void;
  setMatchCase: (match: boolean) => void;
  setMatchWholeWord: (match: boolean) => void;
  setSavedFilters: (filters: FilterPreset[] | ((prev: FilterPreset[]) => FilterPreset[])) => void;
  
  // Modal actions
  setIsSaveModalOpen: (open: boolean) => void;
  setNewFilterName: (name: string) => void;
  openSaveModal: () => void;
  closeSaveModal: () => void;
  
  // AI Search actions
  setIsAIParsing: (parsing: boolean) => void;
  setAiSearchText: (text: string) => void;
  setAiPopoverOpen: (open: boolean) => void;

  // Logcat control
  toggleLogcat: (deviceId: string, pkg: string) => Promise<void>;
  stopLogcat: () => void;

  // Reset for device change
  reset: () => void;
}

// Local buffer to throttle updates (prevents 1000s of state updates per second)
let logBuffer: ParsedLog[] = [];
let flushTimerId: number | null = null;
// 'session-events-batch' 是多个 store/组件共享的通道，必须用 EventsOn 返回的
// cancel 函数精确退订自己的监听器；EventsOff(name) 会移除该通道全部监听器
let sessionEventsOff: (() => void) | null = null;

const MAX_LOGS = 50000; // Reduced from 200k to 50k for performance

export const useLogcatStore = create<LogcatState>()(
  immer((set, get) => ({
    // Initial state
    logs: [],
    isLogging: false,
    selectedPackage: '',

    // Selection state
    selectedLogId: null,
    detailPanelOpen: false,
    activeDetailTab: 'message' as const,

    logFilter: '',
    useRegex: false,
    preFilter: '',
    preUseRegex: false,
    excludeFilter: '',
    excludeUseRegex: false,

    selectedPresetIds: [],
    selectedExcludePresetIds: [],

    // UI State Init
    packages: [],
    appRunningStatus: false,
    autoScroll: true,
    levelFilter: [],
    matchCase: false,
    matchWholeWord: false,
    savedFilters: (() => {
      try {
        const saved = localStorage.getItem("adbGUI_logcat_filters");
        return saved ? JSON.parse(saved) : [];
      } catch {
        return [];
      }
    })(),
    
    // Modal state
    isSaveModalOpen: false,
    newFilterName: '',
    
    // AI Search state
    isAIParsing: false,
    aiSearchText: '',
    aiPopoverOpen: false,

    // Actions
    setLogs: (logsOrUpdater) => {
      set((state: LogcatState) => {
        const nextLogs = typeof logsOrUpdater === 'function'
          ? logsOrUpdater(state.logs)
          : logsOrUpdater;
        
        if (nextLogs.length > MAX_LOGS) {
          state.logs = nextLogs.slice(-MAX_LOGS);
        } else {
          state.logs = nextLogs;
        }
      });
    },

    clearLogs: () => set({ logs: [], selectedLogId: null, detailPanelOpen: false }),

    // Selection actions
    setSelectedLogId: (id) => set({ selectedLogId: id }),
    setDetailPanelOpen: (open) => set({ detailPanelOpen: open }),
    setActiveDetailTab: (tab) => set({ activeDetailTab: tab }),
    selectLog: (id) => set({ selectedLogId: id, detailPanelOpen: true }),
    closeDetail: () => set({ selectedLogId: null, detailPanelOpen: false, activeDetailTab: 'message' }),

    setSelectedPackage: (pkg) => set({ selectedPackage: pkg }),

    setLogFilter: (filter) => set({ logFilter: filter }),
    setUseRegex: (useRegex) => set({ useRegex }),
    setPreFilter: (filter) => set({ preFilter: filter }),
    setPreUseRegex: (useRegex) => set({ preUseRegex: useRegex }),
    setExcludeFilter: (filter) => set({ excludeFilter: filter }),
    setExcludeUseRegex: (useRegex) => set({ excludeUseRegex: useRegex }),

    setSelectedPresetIds: (idsOrUpdater) => set((state: LogcatState) => {
       state.selectedPresetIds = typeof idsOrUpdater === 'function' 
         ? idsOrUpdater(state.selectedPresetIds)
         : idsOrUpdater;
    }),
    
    setSelectedExcludePresetIds: (idsOrUpdater) => set((state: LogcatState) => {
       state.selectedExcludePresetIds = typeof idsOrUpdater === 'function' 
         ? idsOrUpdater(state.selectedExcludePresetIds)
         : idsOrUpdater;
    }),

    setPackages: (packages) => set({ packages }),
    setAppRunningStatus: (status) => set({ appRunningStatus: status }),
    setAutoScroll: (autoScroll) => set({ autoScroll }),
    setLevelFilter: (filter) => set({ levelFilter: filter }),
    setMatchCase: (match) => set({ matchCase: match }),
    setMatchWholeWord: (match) => set({ matchWholeWord: match }),

    setSavedFilters: (filtersOrUpdater) => set((state: LogcatState) => {
      const nextFilters = typeof filtersOrUpdater === 'function'
        ? filtersOrUpdater(state.savedFilters)
        : filtersOrUpdater;
      state.savedFilters = nextFilters;
      localStorage.setItem("adbGUI_logcat_filters", JSON.stringify(nextFilters));
    }),
    
    // Modal actions
    setIsSaveModalOpen: (open) => set({ isSaveModalOpen: open }),
    setNewFilterName: (name) => set({ newFilterName: name }),
    openSaveModal: () => set({ isSaveModalOpen: true, newFilterName: '' }),
    closeSaveModal: () => set({ isSaveModalOpen: false, newFilterName: '' }),
    
    // AI Search actions
    setIsAIParsing: (parsing) => set({ isAIParsing: parsing }),
    setAiSearchText: (text) => set({ aiSearchText: text }),
    setAiPopoverOpen: (open) => set({ aiPopoverOpen: open }),

    toggleLogcat: async (deviceId: string, pkg: string) => {
      const { isLogging } = get();

      if (isLogging) {
        get().stopLogcat();
      } else {
        set((state: LogcatState) => {
          state.logs = [];
          state.isLogging = true;
        });
        
        // Reset buffer
        logBuffer = [];

        // Flush logs every 100ms
        flushTimerId = window.setInterval(() => {
          if (logBuffer.length > 0) {
            const chunk = [...logBuffer];
            logBuffer = []; // Clear local buffer
            
            set((state: LogcatState) => {
              if (chunk.length > MAX_LOGS) {
                 // XOR swap or just replace if chunk is huge
                 state.logs = chunk.slice(-MAX_LOGS);
              } else {
                state.logs.push(...chunk);
                if (state.logs.length > MAX_LOGS) {
                  state.logs = state.logs.slice(-MAX_LOGS);
                }
              }
            });
          }
        }, 100);

        // Subscribe to session events batch (unified event source)
        sessionEventsOff?.(); // 防御重复注册
        sessionEventsOff = EventsOn('session-events-batch', (events: any[]) => {
          // Filter for log events only
          const logEvents = events.filter((e: any) => e.category === 'log');
          if (logEvents.length === 0) return;

          // Access current filter state using get()
          const { preFilter, preUseRegex, excludeFilter, excludeUseRegex } = get();

          const filteredLogs: ParsedLog[] = [];

          for (const event of logEvents) {
            // 后端同时发送 data 和 detail，优先使用 detail（向后兼容），否则用 data
            let eventData: any = event.detail || event.data;
            
            // 如果是 JSON 字符串则解析
            if (typeof eventData === 'string') {
              try {
                eventData = JSON.parse(eventData);
              } catch {
                eventData = null;
              }
            }
            
            // 处理聚合事件或单个事件
            let detailItems: any[] = [];
            
            if (Array.isArray(eventData)) {
                // Aggregated event from backend
                detailItems = eventData;
            } else if (eventData) {
                // Single event
                detailItems = [eventData];
            } else {
                // Fallback: use title as raw log
                detailItems = [{ raw: event.title, tag: '', message: event.title, level: 'I' }];
            }

            for (const item of detailItems) {
                const raw = item.raw || event.title || '';
                if (!raw) continue;

                let shouldKeep = true;

                // 1. Pre-filter (Include)
                if (preFilter.trim()) {
                  try {
                    if (preUseRegex) {
                      const regex = new RegExp(preFilter, 'i');
                      if (!regex.test(raw)) shouldKeep = false;
                    } else {
                      if (!raw.toLowerCase().includes(preFilter.toLowerCase())) shouldKeep = false;
                    }
                  } catch {
                    if (!raw.toLowerCase().includes(preFilter.toLowerCase())) shouldKeep = false;
                  }
                }

                // 2. Exclude filter (Negative)
                if (shouldKeep && excludeFilter.trim()) {
                  try {
                    if (excludeUseRegex) {
                      const regex = new RegExp(excludeFilter, 'i');
                      if (regex.test(raw)) shouldKeep = false;
                    } else {
                      if (raw.toLowerCase().includes(excludeFilter.toLowerCase())) shouldKeep = false;
                    }
                  } catch {
                    if (raw.toLowerCase().includes(excludeFilter.toLowerCase())) shouldKeep = false;
                  }
                }

                if (shouldKeep) {
                  // 构建结构化日志对象
                  const { date, timestamp } = extractTimeFromRaw(raw);
                  const parsed: ParsedLog = {
                    id: generateLogId(),
                    raw: raw,
                    timestamp: timestamp,
                    date: date,
                    level: (item.level as ParsedLog['level']) || 'V',
                    tag: item.tag || '',
                    message: item.message || '',
                    pid: item.pid || extractPidFromRaw(raw),
                    packageName: item.packageName,
                  };
                  filteredLogs.push(parsed);
                }
            }
          }

          if (filteredLogs.length > 0) {
             // Push to local buffer, NO state update here
             logBuffer.push(...filteredLogs);
          }
        });

        try {
          const { preFilter, preUseRegex, excludeFilter, excludeUseRegex } = get();
          await StartLogcat(deviceId, pkg, preFilter, preUseRegex, excludeFilter, excludeUseRegex);
        } catch (err) {
          get().stopLogcat();
          throw err;
        }
      }
    },

    stopLogcat: () => {
      StopLogcat();
      sessionEventsOff?.();
      sessionEventsOff = null;

      if (flushTimerId) {
        clearInterval(flushTimerId);
        flushTimerId = null;
      }
      logBuffer = [];

      set((state: LogcatState) => {
        state.isLogging = false;
      });
    },

    reset: () => {
      const { isLogging } = get();
      if (isLogging) {
        get().stopLogcat();
      }
      set({
        logs: [],
        selectedLogId: null,
        detailPanelOpen: false,
        selectedPackage: '',
        logFilter: '',
        useRegex: false,
        preFilter: '',
        preUseRegex: false,
        excludeFilter: '',
        excludeUseRegex: false,
        selectedPresetIds: [],
        selectedExcludePresetIds: [],
        packages: [],
        appRunningStatus: false,
        autoScroll: true,
        levelFilter: [],
        matchCase: false,
        matchWholeWord: false,
      });
    },
  }))
);
