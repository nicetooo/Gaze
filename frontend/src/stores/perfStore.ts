import { create } from 'zustand';
import { immer } from 'zustand/middleware/immer';
import {
  StartPerfMonitor,
  StopPerfMonitor,
  IsPerfMonitorRunning,
  GetPerfSnapshot,
  GetPerfMonitorConfig,
  GetProcessDetail,
  ListPackages,
} from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';

// @ts-ignore
const EventsOn = (window as any).runtime?.EventsOn;

// ========================================
// Types
// ========================================

export interface ProcessPerfData {
  pid: number;
  name: string;
  cpu: number;
  memoryKB: number;
  user: number;
  kernel: number;
  linuxUser: string;
  ppid: number;
  vszKB: number;
  state: string; // R=running, S=sleeping, D=disk sleep, Z=zombie, T=stopped
}

export interface PerfSampleData {
  cpuUsage: number;
  cpuApp: number;
  cpuCores: number;
  cpuFreqMHz: number;
  cpuTempC: number;
  memTotalMB: number;
  memUsedMB: number;
  memFreeMB: number;
  memUsage: number;
  memAppMB: number;
  fps: number;
  jankCount: number;
  netRxKBps: number;
  netTxKBps: number;
  netRxTotalMB: number;
  netTxTotalMB: number;
  batteryLevel: number;
  batteryTemp: number;
  packageName?: string;
  processes?: ProcessPerfData[];
}

export interface ProcessMemoryCategory {
  name: string;
  pssKB: number;
  rssKB: number;
}

export interface ProcessObjects {
  views: number;
  viewRootImpl: number;
  appContexts: number;
  activities: number;
  assets: number;
  assetManagers: number;
  localBinders: number;
  proxyBinders: number;
  deathRecipients: number;
  webViews: number;
}

export interface ProcessDetail {
  pid: number;
  packageName: string;
  totalPssKB: number;
  totalRssKB: number;
  swapPssKB: number;
  memory: ProcessMemoryCategory[];
  javaHeapSizeKB: number;
  javaHeapAllocKB: number;
  javaHeapFreeKB: number;
  nativeHeapSizeKB: number;
  nativeHeapAllocKB: number;
  nativeHeapFreeKB: number;
  objects: ProcessObjects;
  threads: number;
  fdSize: number;
  vmSwapKB: number;
  oomScoreAdj: number;
  uid: number;
}

export interface PerfMonitorConfig {
  packageName: string;
  intervalMs: number;
  enableCPU: boolean;
  enableMemory: boolean;
  enableFPS: boolean;
  enableNetwork: boolean;
  enableBattery: boolean;
}

export interface PerfHistoryEntry {
  timestamp: number;
  data: PerfSampleData;
}

// Max history entries (at 2s interval, ~5 minutes = 150 entries)
const MAX_HISTORY = 300;

// ========================================
// Store
// ========================================

interface PerfState {
  // Monitoring state
  isMonitoring: boolean;
  config: PerfMonitorConfig;

  // Live data
  latestSample: PerfSampleData | null;
  history: PerfHistoryEntry[];

  // Alert thresholds
  cpuAlertThreshold: number;   // default 80%
  memAlertThreshold: number;   // default 85%
  fpsAlertThreshold: number;   // default 30

  // UI state
  selectedMetricTab: string;   // 'overview' | 'cpu' | 'memory' | 'fps' | 'network' | 'battery'
  tableContainerHeight: number | null; // Measured height of the table container for dynamic scroll

  // Process detail (on-demand)
  selectedPid: number | null;
  processDetail: ProcessDetail | null;
  processDetailLoading: boolean;

  // Package list for selector
  packages: main.AppPackage[];
  packagesLoading: boolean;

  // Actions
  startMonitoring: (deviceId: string, config?: Partial<PerfMonitorConfig>) => Promise<void>;
  stopMonitoring: (deviceId: string) => Promise<void>;
  checkMonitoringStatus: (deviceId: string) => Promise<boolean>;
  takeSnapshot: (deviceId: string, packageName?: string) => Promise<PerfSampleData | null>;
  fetchProcessDetail: (deviceId: string, pid: number) => Promise<void>;
  clearProcessDetail: () => void;
  fetchPackages: (deviceId: string) => Promise<void>;
  
  setConfig: (config: Partial<PerfMonitorConfig>) => void;
  setAlertThresholds: (cpu: number, mem: number, fps: number) => void;
  setSelectedMetricTab: (tab: string) => void;
  setTableContainerHeight: (height: number) => void;
  clearHistory: () => void;

  // Event subscriptions
  subscribeToEvents: () => () => void;
}

export const defaultPerfConfig: PerfMonitorConfig = {
  packageName: '',
  intervalMs: 2000,
  enableCPU: true,
  enableMemory: true,
  enableFPS: true,
  enableNetwork: true,
  enableBattery: true,
};

export const usePerfStore = create<PerfState>()(
  immer((set, get) => ({
    // Initial state
    isMonitoring: false,
    config: { ...defaultPerfConfig },
    latestSample: null,
    history: [],
    cpuAlertThreshold: 80,
    memAlertThreshold: 85,
    fpsAlertThreshold: 30,
    selectedMetricTab: 'overview',
    tableContainerHeight: null,
    selectedPid: null,
    processDetail: null,
    processDetailLoading: false,
    packages: [],
    packagesLoading: false,

    // Fetch process detail (on-demand, ~2-3s)
    fetchProcessDetail: async (deviceId: string, pid: number) => {
      set((s) => {
        s.selectedPid = pid;
        s.processDetail = null;
        s.processDetailLoading = true;
      });
      try {
        const detail = await GetProcessDetail(deviceId, pid);
        if (detail) {
          set((s) => {
            s.processDetail = detail as unknown as ProcessDetail;
            s.processDetailLoading = false;
          });
        }
      } catch (err) {
        console.error('Failed to fetch process detail:', err);
        set((s) => {
          s.processDetailLoading = false;
        });
      }
    },

    clearProcessDetail: () => {
      set((s) => {
        s.selectedPid = null;
        s.processDetail = null;
        s.processDetailLoading = false;
      });
    },

    // Start monitoring
    startMonitoring: async (deviceId: string, configOverride?: Partial<PerfMonitorConfig>) => {
      const state = get();
      const config = { ...state.config, ...configOverride };
      
      try {
        await StartPerfMonitor(deviceId, config);
        set((s) => {
          s.isMonitoring = true;
          s.config = config;
          s.history = []; // Clear history on new session
        });
      } catch (err) {
        console.error('Failed to start perf monitor:', err);
        throw err;
      }
    },

    // Stop monitoring
    stopMonitoring: async (deviceId: string) => {
      try {
        await StopPerfMonitor(deviceId);
        set((s) => {
          s.isMonitoring = false;
        });
      } catch (err) {
        console.error('Failed to stop perf monitor:', err);
        throw err;
      }
    },

    // Check status
    checkMonitoringStatus: async (deviceId: string) => {
      try {
        const running = await IsPerfMonitorRunning(deviceId);
        set((s) => {
          s.isMonitoring = running;
        });
        if (running) {
          try {
            const cfg = await GetPerfMonitorConfig(deviceId);
            if (cfg) {
              set((s) => {
                s.config = cfg as PerfMonitorConfig;
              });
            }
          } catch { /* ignore */ }
        }
        return running;
      } catch {
        return false;
      }
    },

    // One-time snapshot
    takeSnapshot: async (deviceId: string, packageName?: string) => {
      try {
        const sample = await GetPerfSnapshot(deviceId, packageName || '');
        if (sample) {
          const perfData = sample as unknown as PerfSampleData;
          set((s) => {
            s.latestSample = perfData;
            s.history.push({ timestamp: Date.now(), data: perfData });
            if (s.history.length > MAX_HISTORY) {
              s.history = s.history.slice(-MAX_HISTORY);
            }
          });
          return perfData;
        }
        return null;
      } catch (err) {
        console.error('Failed to take perf snapshot:', err);
        return null;
      }
    },

    // Fetch packages for selector
    fetchPackages: async (deviceId: string) => {
      set((s) => { s.packagesLoading = true; });
      try {
        const res = await ListPackages(deviceId, 'user');
        set((s) => {
          s.packages = res || [];
          s.packagesLoading = false;
        });
      } catch {
        set((s) => { s.packagesLoading = false; });
      }
    },

    // Config
    setConfig: (config: Partial<PerfMonitorConfig>) => {
      set((s) => {
        Object.assign(s.config, config);
      });
    },

    // Alert thresholds
    setAlertThresholds: (cpu: number, mem: number, fps: number) => {
      set((s) => {
        s.cpuAlertThreshold = cpu;
        s.memAlertThreshold = mem;
        s.fpsAlertThreshold = fps;
      });
    },

    // UI state
    setSelectedMetricTab: (tab: string) => {
      set((s) => {
        s.selectedMetricTab = tab;
      });
    },

    setTableContainerHeight: (height: number) => {
      set((s) => {
        s.tableContainerHeight = height;
      });
    },

    clearHistory: () => {
      set((s) => {
        s.history = [];
        s.latestSample = null;
      });
    },

    // Subscribe to perf events from backend
    subscribeToEvents: () => {
      if (!EventsOn) return () => {};

      // Listen for perf_sample events from the unified event pipeline
      const handler = (events: any[]) => {
        if (!Array.isArray(events)) return;
        
        for (const event of events) {
          if (event.type === 'perf_sample' && event.data) {
            const data = typeof event.data === 'string' ? JSON.parse(event.data) : event.data;
            set((s) => {
              s.latestSample = data;
              s.history.push({ timestamp: event.timestamp || Date.now(), data });
              if (s.history.length > MAX_HISTORY) {
                s.history = s.history.slice(-MAX_HISTORY);
              }
            });
          }
        }
      };

      // 'session-events-batch' 是共享通道：用 EventsOn 返回的 cancel 函数精确退订，
      // EventsOff(name) 会误删其他 store/组件的监听器
      const offBatch = EventsOn('session-events-batch', handler);

      // Also listen for perf status changes
      const statusHandler = (status: any) => {
        if (status) {
          set((s) => {
            s.isMonitoring = status.running;
            if (status.config) {
              s.config = status.config;
            }
          });
        }
      };

      const offStatus = EventsOn('perf-status', statusHandler);

      return () => {
        offBatch?.();
        offStatus?.();
      };
    },
  }))
);
