import { create } from 'zustand';
import { main } from '../../wailsjs/go/models';
import {
  StartTouchRecording,
  StopTouchRecording,
  PlayTouchScript,
  StopTouchPlayback,
  LoadTouchScripts,
  SaveTouchScript,
  DeleteTouchScript,
} from '../../wailsjs/go/main/App';

const EventsOn = (window as any).runtime.EventsOn;
const EventsOff = (window as any).runtime.EventsOff;

// Re-export types from Wails models for convenience
export type TouchEvent = main.TouchEvent;
export type TouchScript = main.TouchScript;

interface AutomationState {
  // State
  isRecording: boolean;
  isPlaying: boolean;
  recordingDeviceId: string | null;
  playingDeviceId: string | null;
  recordingStartTime: number | null;
  recordingDuration: number;
  currentScript: main.TouchScript | null;
  scripts: main.TouchScript[];
  playbackProgress: { current: number; total: number } | null;

  // Actions
  startRecording: (deviceId: string) => Promise<void>;
  stopRecording: () => Promise<main.TouchScript | null>;
  playScript: (deviceId: string, script: main.TouchScript) => Promise<void>;
  stopPlayback: () => void;
  loadScripts: () => Promise<void>;
  saveScript: (script: main.TouchScript) => Promise<void>;
  deleteScript: (name: string) => Promise<void>;
  setCurrentScript: (script: main.TouchScript | null) => void;
  updateRecordingDuration: () => void;

  // Event subscription
  subscribeToEvents: () => () => void;
}

export const useAutomationStore = create<AutomationState>((set, get) => ({
  // Initial state
  isRecording: false,
  isPlaying: false,
  recordingDeviceId: null,
  playingDeviceId: null,
  recordingStartTime: null,
  recordingDuration: 0,
  currentScript: null,
  scripts: [],
  playbackProgress: null,

  // Actions
  startRecording: async (deviceId: string) => {
    try {
      await StartTouchRecording(deviceId);
      set({
        isRecording: true,
        recordingDeviceId: deviceId,
        recordingStartTime: Date.now(),
        recordingDuration: 0,
        currentScript: null,
      });
    } catch (err) {
      console.error('Failed to start recording:', err);
      throw err;
    }
  },

  stopRecording: async () => {
    const { recordingDeviceId } = get();
    if (!recordingDeviceId) return null;

    try {
      const script = await StopTouchRecording(recordingDeviceId);
      set({
        isRecording: false,
        recordingDeviceId: null,
        recordingStartTime: null,
        recordingDuration: 0,
        currentScript: script,
      });
      return script;
    } catch (err) {
      console.error('Failed to stop recording:', err);
      set({
        isRecording: false,
        recordingDeviceId: null,
        recordingStartTime: null,
        recordingDuration: 0,
      });
      throw err;
    }
  },

  playScript: async (deviceId: string, script: main.TouchScript) => {
    try {
      await PlayTouchScript(deviceId, script);
      set({
        isPlaying: true,
        playingDeviceId: deviceId,
        playbackProgress: { current: 0, total: script.events?.length || 0 },
      });
    } catch (err) {
      console.error('Failed to play script:', err);
      throw err;
    }
  },

  stopPlayback: () => {
    const { playingDeviceId } = get();
    if (playingDeviceId) {
      StopTouchPlayback(playingDeviceId);
    }
    set({
      isPlaying: false,
      playingDeviceId: null,
      playbackProgress: null,
    });
  },

  loadScripts: async () => {
    try {
      const scripts = await LoadTouchScripts();
      set({ scripts: scripts || [] });
    } catch (err) {
      console.error('Failed to load scripts:', err);
      set({ scripts: [] });
    }
  },

  saveScript: async (script: main.TouchScript) => {
    try {
      await SaveTouchScript(script);
      await get().loadScripts();
    } catch (err) {
      console.error('Failed to save script:', err);
      throw err;
    }
  },

  deleteScript: async (name: string) => {
    try {
      await DeleteTouchScript(name);
      await get().loadScripts();
    } catch (err) {
      console.error('Failed to delete script:', err);
      throw err;
    }
  },

  setCurrentScript: (script: main.TouchScript | null) => {
    set({ currentScript: script });
  },

  updateRecordingDuration: () => {
    const { isRecording, recordingStartTime } = get();
    if (isRecording && recordingStartTime) {
      const duration = Math.floor((Date.now() - recordingStartTime) / 1000);
      set({ recordingDuration: duration });
    }
  },

  // Event subscription
  subscribeToEvents: () => {
    const handleRecordStarted = (data: any) => {
      set({
        isRecording: true,
        recordingDeviceId: data.deviceId,
        recordingStartTime: data.startTime * 1000,
      });
    };

    const handleRecordStopped = (data: any) => {
      set({
        isRecording: false,
        recordingDeviceId: null,
        recordingStartTime: null,
        recordingDuration: 0,
      });
    };

    const handlePlaybackStarted = (data: any) => {
      set({
        isPlaying: true,
        playingDeviceId: data.deviceId,
        playbackProgress: { current: 0, total: data.total },
      });
    };

    const handlePlaybackProgress = (data: any) => {
      set({
        playbackProgress: { current: data.current, total: data.total },
      });
    };

    const handlePlaybackCompleted = (data: any) => {
      set({
        isPlaying: false,
        playingDeviceId: null,
        playbackProgress: null,
      });
    };

    EventsOn('touch-record-started', handleRecordStarted);
    EventsOn('touch-record-stopped', handleRecordStopped);
    EventsOn('touch-playback-started', handlePlaybackStarted);
    EventsOn('touch-playback-progress', handlePlaybackProgress);
    EventsOn('touch-playback-completed', handlePlaybackCompleted);

    return () => {
      EventsOff('touch-record-started');
      EventsOff('touch-record-stopped');
      EventsOff('touch-playback-started');
      EventsOff('touch-playback-progress');
      EventsOff('touch-playback-completed');
    };
  },
}));
