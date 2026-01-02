/**
 * Unified type definitions for the application stores
 * Consolidates type definitions previously scattered across 9+ files
 */

// Device types
export interface Device {
  id: string;
  serial: string;
  state: string;
  model: string;
  brand: string;
  type: string;
  ids: string[];
  wifiAddr: string;
  isPinned: boolean;
}

export interface HistoryDevice {
  id: string;
  serial: string;
  model: string;
  brand: string;
  type: string;
  wifiAddr: string;
  lastSeen: string;
  isPinned?: boolean;
}

// Mirror/Scrcpy types
export interface MirrorStatus {
  isMirroring: boolean;
  startTime: number | null;
  duration: number;
}

export interface RecordStatus {
  isRecording: boolean;
  startTime: number | null;
  duration: number;
  recordPath: string;
}

// Navigation types
export type ViewKey = '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8';

export const VIEW_KEYS = {
  DEVICES: '1' as ViewKey,
  APPS: '2' as ViewKey,
  SHELL: '3' as ViewKey,
  LOGCAT: '4' as ViewKey,
  MIRROR: '5' as ViewKey,
  FILES: '6' as ViewKey,
  PROXY: '7' as ViewKey,
  AUTOMATION: '8' as ViewKey,
} as const;

export const VIEW_NAME_MAP: Record<string, ViewKey> = {
  devices: VIEW_KEYS.DEVICES,
  apps: VIEW_KEYS.APPS,
  shell: VIEW_KEYS.SHELL,
  logcat: VIEW_KEYS.LOGCAT,
  mirror: VIEW_KEYS.MIRROR,
  files: VIEW_KEYS.FILES,
  proxy: VIEW_KEYS.PROXY,
  automation: VIEW_KEYS.AUTOMATION,
};

// Batch operation types
export interface BatchOperation {
  type: string;
  deviceIds: string[];
  packageName: string;
  apkPath: string;
  command: string;
  localPath: string;
  remotePath: string;
}

export interface BatchResult {
  deviceId: string;
  success: boolean;
  output: string;
  error: string;
}

export interface BatchOperationResult {
  totalDevices: number;
  successCount: number;
  failureCount: number;
  results: BatchResult[];
}
