// Barrel file for all stores
export * from './types';
export { useDeviceStore } from './deviceStore';
export { useMirrorStore, openRecordPath } from './mirrorStore';
export { useLogcatStore } from './logcatStore';
export { useUIStore } from './uiStore';
export { useAutomationStore } from './automationStore';
export type { TouchEvent, TouchScript } from './automationStore';
