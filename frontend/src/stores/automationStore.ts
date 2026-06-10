import { create } from 'zustand';
import { immer } from 'zustand/middleware/immer';
import { main } from '../types/wails-models';
import type { UINode } from './elementStore';
import {
  StartTouchRecording,
  StopTouchRecording,
  PlayTouchScript,
  StopTouchPlayback,
  LoadTouchScripts,
  SaveTouchScript,
  DeleteTouchScript,
  RenameTouchScript,
  SaveScriptTask,
  LoadScriptTasks,
  DeleteScriptTask,
  RunScriptTask,
  PauseTask,
  ResumeTask,
  StopTask,
} from '../../wailsjs/go/main/App';

const EventsOn = (window as any).runtime.EventsOn;

// Re-export types from Wails models for convenience
export type TouchEvent = main.TouchEvent;
export type TouchScript = main.TouchScript;

export interface TaskStep {
  type: string;
  value: string;
  loop: number;
  postDelay?: number;
  checkType?: string;
  checkValue?: string;
  waitTimeout?: number;
  onFailure?: string;
}

export interface ScriptTask {
  name: string;
  steps: TaskStep[];
  createdAt: string;
}

// Re-export UINode from elementStore for backward compatibility
export type { UINode };

// Convert a TouchScript to a Workflow - shared utility
export async function convertScriptToWorkflow(
  script: main.TouchScript,
  t: (key: string, opts?: any) => string,
): Promise<void> {
  if (!script.events || script.events.length === 0) {
    throw new Error(t("automation.no_events_to_convert"));
  }

  const steps: main.WorkflowStep[] = [];
  let lastTimestamp = 0;
  const now = Date.now();

  // Add Start node
  const startId = `step_start_${now}`;
  steps.push({
    id: startId,
    type: 'start',
    name: t('workflow.step_type.start'),
    posX: 250,
    posY: 50,
  } as main.WorkflowStep);

  const buildStepName = (action: string, event: main.TouchEvent, fallback: string) => {
    if (event.selector && event.selector.value) {
      const val = event.selector.value;
      const shortVal = val.split('/').pop() || val;
      return `${action}: "${shortVal}"`;
    }
    return fallback;
  };

  script.events.forEach((event, idx) => {
    // Add wait step if there is a significant delay
    const delay = event.timestamp - lastTimestamp;
    if (delay > 50) {
      steps.push({
        id: `step_wait_${now}_${idx}`,
        type: 'wait',
        name: `${t('workflow.generated_step_name.wait')} ${delay}ms`,
        value: String(delay),
        loop: 1,
        postDelay: 0,
        posX: 250,
        posY: 150 + steps.length * 100,
      } as main.WorkflowStep);
    }
    lastTimestamp = event.timestamp;

    const id = `step_action_${now}_${idx}`;

    if (event.type === 'tap') {
      const selector = event.selector;
      if (selector && selector.type !== 'coordinates') {
        steps.push({
          id,
          type: 'click_element',
          name: buildStepName(t('workflow.generated_step_name.click'), event, `${t('workflow.generated_step_name.click')} ${idx + 1}`),
          selector: { ...selector },
          loop: 1,
          postDelay: 0,
          onError: 'stop',
          posX: 250,
          posY: 150 + steps.length * 100,
        } as main.WorkflowStep);
      } else {
        const tapSize = 10;
        const x1 = Math.max(0, event.x - tapSize);
        const y1 = Math.max(0, event.y - tapSize);
        const x2 = event.x + tapSize;
        const y2 = event.y + tapSize;
        const bounds = `[${x1},${y1}][${x2},${y2}]`;
        steps.push({
          id,
          type: 'click_element',
          name: `${t('workflow.generated_step_name.click')} (${event.x}, ${event.y})`,
          selector: { type: 'bounds', value: bounds, index: 0 },
          loop: 1,
          postDelay: 0,
          onError: 'stop',
          posX: 250,
          posY: 150 + steps.length * 100,
        } as main.WorkflowStep);
      }
    } else if (event.type === 'long_press') {
      const selector = event.selector;
      if (selector && selector.type !== 'coordinates') {
        steps.push({
          id,
          type: 'long_click_element',
          name: buildStepName(t('workflow.generated_step_name.long_press'), event, `${t('workflow.generated_step_name.long_press')} ${idx + 1}`),
          selector: { ...selector },
          loop: 1,
          postDelay: 0,
          onError: 'stop',
          posX: 250,
          posY: 150 + steps.length * 100,
        } as main.WorkflowStep);
      } else {
        const tapSize = 10;
        const x1 = Math.max(0, event.x - tapSize);
        const y1 = Math.max(0, event.y - tapSize);
        const x2 = event.x + tapSize;
        const y2 = event.y + tapSize;
        const bounds = `[${x1},${y1}][${x2},${y2}]`;
        steps.push({
          id,
          type: 'long_click_element',
          name: `${t('workflow.generated_step_name.long_press')} (${event.x}, ${event.y})`,
          selector: { type: 'bounds', value: bounds, index: 0 },
          loop: 1,
          postDelay: 0,
          onError: 'stop',
          posX: 250,
          posY: 150 + steps.length * 100,
        } as main.WorkflowStep);
      }
    } else if (event.type === 'swipe') {
      const dx = (event.x2 || event.x) - event.x;
      const dy = (event.y2 || event.y) - event.y;
      const distance = Math.sqrt(dx * dx + dy * dy);
      let direction = 'up';
      if (Math.abs(dx) > Math.abs(dy)) {
        direction = dx > 0 ? 'right' : 'left';
      } else {
        direction = dy > 0 ? 'down' : 'up';
      }
      const tapSize = 10;
      const x1 = Math.max(0, event.x - tapSize);
      const y1 = Math.max(0, event.y - tapSize);
      const x2 = event.x + tapSize;
      const y2 = event.y + tapSize;
      const bounds = `[${x1},${y1}][${x2},${y2}]`;
      steps.push({
        id,
        type: 'swipe_element',
        name: `${t('workflow.generated_step_name.swipe')} ${direction}`,
        selector: { type: 'bounds', value: bounds, index: 0 },
        value: direction,
        swipeDistance: Math.round(distance),
        swipeDuration: event.duration || 300,
        loop: 1,
        postDelay: 0,
        onError: 'stop',
        posX: 250,
        posY: 150 + steps.length * 100,
      } as main.WorkflowStep);
    } else if (event.type === 'wait') {
      steps.push({
        id,
        type: 'wait',
        name: `${t('workflow.generated_step_name.wait')} ${idx + 1}`,
        value: String(event.duration || 1000),
        loop: 1,
        postDelay: 0,
        posX: 250,
        posY: 150 + steps.length * 100,
      } as main.WorkflowStep);
    }
  });

  // Link steps sequentially
  for (let i = 0; i < steps.length - 1; i++) {
    steps[i].nextStepId = steps[i + 1].id;
  }

  const newWorkflow = new (main as any).Workflow({
    id: `wf_converted_${now}`,
    name: `${script.name}${t('workflow.converted_suffix')}`,
    description: t('workflow.converted_desc', { date: new Date().toLocaleString() }),
    steps: steps,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  });

  await (window as any).go.main.App.SaveWorkflow(newWorkflow);
}

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
  tasks: ScriptTask[];
  playbackProgress: { current: number; total: number } | null;
  taskProgress: { 
    stepIndex: number; 
    totalSteps: number; 
    currentLoop: number; 
    totalLoops: number; 
    currentAction?: string 
  } | null;
  isTaskRunning: boolean;
  isPaused: boolean;
  runningTaskName: string | null;

  recordedActionCount: number;
  uiHierarchy: UINode | null;
  rawXml: string | null;
  isFetchingHierarchy: boolean;

  // Recording mode state
  recordingMode: 'fast' | 'precise';
  isWaitingForSelector: boolean;
  pendingSelectorData: any | null;
  isPreCapturing: boolean;
  isAnalyzing: boolean;
  
  // RecordingView UI state
  selectedScriptNames: string[];
  saveModalVisible: boolean;
  scriptName: string;
  renameModalVisible: boolean;
  editingScriptName: string;
  newScriptName: string;
  selectedScript: main.TouchScript | null;

  // Actions
  startRecording: (deviceId: string, mode?: 'fast' | 'precise') => Promise<void>;
  stopRecording: () => Promise<main.TouchScript | null>;
  playScript: (deviceId: string, script: main.TouchScript) => Promise<void>;
  stopPlayback: () => void;
  loadScripts: () => Promise<void>;
  saveScript: (script: main.TouchScript) => Promise<void>;
  deleteScript: (name: string) => Promise<void>;
  deleteScripts: (names: string[]) => Promise<void>;
  renameScript: (oldName: string, newName: string) => Promise<void>;
  setCurrentScript: (script: main.TouchScript | null) => void;
  updateRecordingDuration: () => void;

  // Selector choice actions
  submitSelectorChoice: (selectorType: string, selectorValue: string) => Promise<void>;
  setRecordingMode: (mode: 'fast' | 'precise') => void;

  // Task Actions
  loadTasks: () => Promise<void>;
  saveTask: (task: ScriptTask) => Promise<void>;
  deleteTask: (name: string) => Promise<void>;
  deleteTasks: (names: string[]) => Promise<void>;
  runTask: (deviceId: string, task: ScriptTask) => Promise<void>;
  pauseTask: () => Promise<void>;
  resumeTask: () => Promise<void>;
  stopTask: () => Promise<void>;
  fetchUIHierarchy: (deviceId: string) => Promise<void>;
  checkAndRefreshUIHierarchy: (deviceId: string) => Promise<boolean>;

  // Event subscription
  subscribeToEvents: () => () => void;
  
  // RecordingView UI actions
  setSelectedScriptNames: (names: string[]) => void;
  toggleScriptSelection: (name: string) => void;
  setSaveModalVisible: (visible: boolean) => void;
  setScriptName: (name: string) => void;
  setRenameModalVisible: (visible: boolean) => void;
  setEditingScriptName: (name: string) => void;
  setNewScriptName: (name: string) => void;
  setSelectedScript: (script: main.TouchScript | null) => void;
  openRenameModal: (scriptName: string) => void;
  closeRenameModal: () => void;
  closeSaveModal: () => void;

  // Playback settings
  playbackSpeed: number;
  smartTapTimeoutMs: number;
  setPlaybackSpeed: (speed: number) => void;
  setSmartTapTimeoutMs: (ms: number) => void;

  // Script editing actions
  editingEventIndex: number | null;
  setEditingEventIndex: (index: number | null) => void;
  updateScriptEvent: (index: number, updates: Partial<{ x: number; y: number; x2: number; y2: number; duration: number; type: string }>) => void;
  deleteScriptEvent: (index: number) => void;
  moveScriptEvent: (fromIndex: number, toIndex: number) => void;
  insertWaitEvent: (afterIndex: number, durationMs: number) => void;
  isScriptDirty: boolean;
  setScriptDirty: (dirty: boolean) => void;
}

export const useAutomationStore = create<AutomationState>()(
  immer((set, get) => ({
    // Initial state
    isRecording: false,
    isPlaying: false,
    recordingDeviceId: null,
    playingDeviceId: null,
    recordingStartTime: null,
    recordingDuration: 0,
    recordedActionCount: 0,
    currentScript: null,
    scripts: [],
    tasks: [],
    playbackProgress: null,
    taskProgress: null,
    isTaskRunning: false,
    isPaused: false,
    runningTaskName: null,
    uiHierarchy: null,
    rawXml: null,
    isFetchingHierarchy: false,
    
    // Recording mode state
    recordingMode: 'fast',
    isWaitingForSelector: false,
    pendingSelectorData: null,
    isPreCapturing: false,
    isAnalyzing: false,
    
    // RecordingView UI state
    selectedScriptNames: [],
    saveModalVisible: false,
    scriptName: '',
    renameModalVisible: false,
    editingScriptName: '',
    newScriptName: '',
    selectedScript: null,
    
    // Playback settings
    playbackSpeed: 1.0,
    smartTapTimeoutMs: 5000,

    // Script editing state
    editingEventIndex: null,
    isScriptDirty: false,

    // Actions
    startRecording: async (deviceId: string, mode: 'fast' | 'precise' = 'fast') => {
      try {
        await StartTouchRecording(deviceId, mode);
        set((state: AutomationState) => {
          state.isRecording = true;
          state.recordingDeviceId = deviceId;
          state.recordingStartTime = Date.now();
          state.recordingDuration = 0;
          state.recordedActionCount = 0;
          state.currentScript = null;
          state.recordingMode = mode;
          state.isWaitingForSelector = false;
          state.pendingSelectorData = null;
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
        set((state: AutomationState) => {
          state.isRecording = false;
          state.recordingDeviceId = null;
          state.recordingStartTime = null;
          state.recordingDuration = 0;
          state.recordedActionCount = 0;
          state.currentScript = script;
        });
        return script;
      } catch (err) {
        console.error('Failed to stop recording:', err);
        set((state: AutomationState) => {
          state.isRecording = false;
          state.recordingDeviceId = null;
          state.recordingStartTime = null;
          state.recordingDuration = 0;
          state.recordedActionCount = 0;
        });
        throw err;
      }
    },

    playScript: async (deviceId: string, script: main.TouchScript) => {
      try {
        await PlayTouchScript(deviceId, script);
        set((state: AutomationState) => {
          state.isPlaying = true;
          state.playingDeviceId = deviceId;
          state.playbackProgress = { current: 0, total: script.events?.length || 0 };
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
      set((state: AutomationState) => {
        state.isPlaying = false;
        state.playingDeviceId = null;
        state.playbackProgress = null;
      });
    },

    loadScripts: async () => {
      try {
        const scripts = await LoadTouchScripts();
        set((state: AutomationState) => {
          state.scripts = scripts || [];
        });
      } catch (err) {
        console.error('Failed to load scripts:', err);
        set((state: AutomationState) => {
          state.scripts = [];
        });
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

    deleteScripts: async (names: string[]) => {
      try {
        for (const name of names) {
          await DeleteTouchScript(name);
        }
        await get().loadScripts();
      } catch (err) {
        console.error('Failed to delete scripts:', err);
        throw err;
      }
    },

    renameScript: async (oldName: string, newName: string) => {
      try {
        await RenameTouchScript(oldName, newName);
        await get().loadScripts();
      } catch (err) {
        console.error('Failed to rename script:', err);
        throw err;
      }
    },

    loadTasks: async () => {
      try {
        const tasks = await LoadScriptTasks();
        set((state: AutomationState) => {
          state.tasks = (tasks || []) as ScriptTask[];
        });
      } catch (err) {
        console.error('Failed to load tasks:', err);
        set((state: AutomationState) => {
          state.tasks = [];
        });
      }
    },

    saveTask: async (task: ScriptTask) => {
      try {
        await SaveScriptTask(task as unknown as main.ScriptTask);
        await get().loadTasks();
      } catch (err) {
        console.error('Failed to save task:', err);
        throw err;
      }
    },

    deleteTask: async (name: string) => {
      try {
        await DeleteScriptTask(name);
        await get().loadTasks();
      } catch (err) {
        console.error('Failed to delete task:', err);
        throw err;
      }
    },

    deleteTasks: async (names: string[]) => {
      try {
        for (const name of names) {
          await DeleteScriptTask(name);
        }
        await get().loadTasks();
      } catch (err) {
        console.error('Failed to delete tasks:', err);
        throw err;
      }
    },

    runTask: async (deviceId: string, task: ScriptTask) => {
      try {
        await RunScriptTask(deviceId, task as unknown as main.ScriptTask);
        set((state: AutomationState) => {
          state.isTaskRunning = true;
          state.isPaused = false;
          state.runningTaskName = task.name;
        });
      } catch (err) {
        console.error('Failed to run task:', err);
        throw err;
      }
    },

    pauseTask: async () => {
      const deviceId = get().playingDeviceId;
      if (deviceId) {
        await PauseTask(deviceId);
        set((state: AutomationState) => {
          state.isPaused = true;
        });
      }
    },

    resumeTask: async () => {
      const deviceId = get().playingDeviceId;
      if (deviceId) {
        await ResumeTask(deviceId);
        set((state: AutomationState) => {
          state.isPaused = false;
        });
      }
    },

    stopTask: async () => {
      const deviceId = get().playingDeviceId;
      if (deviceId) {
        await StopTask(deviceId);
        set((state: AutomationState) => {
          state.isTaskRunning = false;
          state.isPaused = false;
          state.runningTaskName = null;
          state.taskProgress = null;
        });
      }
    },

    fetchUIHierarchy: async (deviceId: string) => {
      if (!deviceId) return;
      set((state: AutomationState) => {
        state.isFetchingHierarchy = true;
      });
      try {
        // Delegate to elementStore for shared caching
        const { useElementStore } = await import('./elementStore');
        const elementStore = useElementStore.getState();
        const hierarchy = await elementStore.fetchHierarchy(deviceId, true);
        // Sync state for backward compatibility
        set((state: AutomationState) => {
          state.uiHierarchy = hierarchy as unknown as UINode;
          state.rawXml = elementStore.rawXml;
          state.isFetchingHierarchy = false;
        });
      } catch (err) {
        console.error('Failed to fetch UI hierarchy:', err);
        set((state: AutomationState) => {
          state.isFetchingHierarchy = false;
        });
        throw err;
      }
    },

    checkAndRefreshUIHierarchy: async (deviceId: string) => {
      if (!deviceId) return false;
      try {
        // Delegate to elementStore for shared caching
        const { useElementStore } = await import('./elementStore');
        const elementStore = useElementStore.getState();
        const oldRawXml = get().rawXml;
        await elementStore.fetchHierarchy(deviceId, false);
        if (elementStore.rawXml !== oldRawXml) {
          set((state: AutomationState) => {
            state.uiHierarchy = elementStore.hierarchy as unknown as UINode;
            state.rawXml = elementStore.rawXml;
          });
          return true;
        }
        return false;
      } catch (err) {
        console.error('Background UI refresh failed:', err);
        return false;
      }
    },

    setCurrentScript: (script: main.TouchScript | null) => {
      set((state: AutomationState) => {
        state.currentScript = script;
      });
    },

    updateRecordingDuration: () => {
      const { isRecording, recordingStartTime } = get();
      if (isRecording && recordingStartTime) {
        const duration = Math.floor((Date.now() - recordingStartTime) / 1000);
        set((state: AutomationState) => {
          state.recordingDuration = duration;
        });
      }
    },

    submitSelectorChoice: async (selectorType: string, selectorValue: string) => {
      const { recordingDeviceId } = get();
      if (!recordingDeviceId) {
        throw new Error('No active recording');
      }
      
      try {
        await (window as any).go.main.App.SubmitSelectorChoice(
          recordingDeviceId,
          selectorType,
          selectorValue
        );
        set((state: AutomationState) => {
          state.isWaitingForSelector = false;
          state.pendingSelectorData = null;
        });
      } catch (err) {
        console.error('Failed to submit selector choice:', err);
        throw err;
      }
    },

    setRecordingMode: (mode: 'fast' | 'precise') => {
      set((state: AutomationState) => {
        state.recordingMode = mode;
      });
    },
    
    // RecordingView UI actions
    setSelectedScriptNames: (names: string[]) => {
      set((state: AutomationState) => {
        state.selectedScriptNames = names;
      });
    },
    
    toggleScriptSelection: (name: string) => {
      set((state: AutomationState) => {
        const index = state.selectedScriptNames.indexOf(name);
        if (index > -1) {
          state.selectedScriptNames.splice(index, 1);
        } else {
          state.selectedScriptNames.push(name);
        }
      });
    },
    
    setSaveModalVisible: (visible: boolean) => {
      set((state: AutomationState) => {
        state.saveModalVisible = visible;
      });
    },
    
    setScriptName: (name: string) => {
      set((state: AutomationState) => {
        state.scriptName = name;
      });
    },
    
    setRenameModalVisible: (visible: boolean) => {
      set((state: AutomationState) => {
        state.renameModalVisible = visible;
      });
    },
    
    setEditingScriptName: (name: string) => {
      set((state: AutomationState) => {
        state.editingScriptName = name;
      });
    },
    
    setNewScriptName: (name: string) => {
      set((state: AutomationState) => {
        state.newScriptName = name;
      });
    },
    
    setSelectedScript: (script: main.TouchScript | null) => {
      set((state: AutomationState) => {
        state.selectedScript = script;
      });
    },
    
    openRenameModal: (scriptName: string) => {
      set((state: AutomationState) => {
        state.editingScriptName = scriptName;
        state.newScriptName = scriptName;
        state.renameModalVisible = true;
      });
    },
    
    closeRenameModal: () => {
      set((state: AutomationState) => {
        state.renameModalVisible = false;
        state.editingScriptName = '';
        state.newScriptName = '';
      });
    },
    
    closeSaveModal: () => {
      set((state: AutomationState) => {
        state.saveModalVisible = false;
        state.scriptName = '';
      });
    },

    // Script editing actions
    setEditingEventIndex: (index: number | null) => {
      set((state: AutomationState) => {
        state.editingEventIndex = index;
      });
    },

    setScriptDirty: (dirty: boolean) => {
      set((state: AutomationState) => {
        state.isScriptDirty = dirty;
      });
    },

    setPlaybackSpeed: (speed: number) => {
      set((state: AutomationState) => {
        state.playbackSpeed = speed;
      });
    },

    setSmartTapTimeoutMs: (ms: number) => {
      set((state: AutomationState) => {
        state.smartTapTimeoutMs = ms;
      });
    },

    updateScriptEvent: (index: number, updates: Partial<{ x: number; y: number; x2: number; y2: number; duration: number; type: string }>) => {
      set((state: AutomationState) => {
        // Determine which script to edit: currentScript (unsaved) or selectedScript (saved)
        const script = state.currentScript || state.selectedScript;
        if (!script || !script.events || index < 0 || index >= script.events.length) return;

        const event = script.events[index];
        if (updates.x !== undefined) event.x = updates.x;
        if (updates.y !== undefined) event.y = updates.y;
        if (updates.x2 !== undefined) event.x2 = updates.x2;
        if (updates.y2 !== undefined) event.y2 = updates.y2;
        if (updates.duration !== undefined) event.duration = updates.duration;
        if (updates.type !== undefined) event.type = updates.type;

        state.isScriptDirty = true;
      });
    },

    deleteScriptEvent: (index: number) => {
      set((state: AutomationState) => {
        const script = state.currentScript || state.selectedScript;
        if (!script || !script.events || index < 0 || index >= script.events.length) return;

        script.events.splice(index, 1);
        // Recalculate timestamps to be relative from 0
        if (script.events.length > 0) {
          const offset = script.events[0].timestamp;
          for (const ev of script.events) {
            ev.timestamp -= offset;
          }
        }
        state.isScriptDirty = true;
        // Reset editing index if the deleted event was being edited
        if (state.editingEventIndex === index) {
          state.editingEventIndex = null;
        } else if (state.editingEventIndex !== null && state.editingEventIndex > index) {
          state.editingEventIndex--;
        }
      });
    },

    moveScriptEvent: (fromIndex: number, toIndex: number) => {
      set((state: AutomationState) => {
        const script = state.currentScript || state.selectedScript;
        if (!script || !script.events) return;
        if (fromIndex < 0 || fromIndex >= script.events.length) return;
        if (toIndex < 0 || toIndex >= script.events.length) return;
        if (fromIndex === toIndex) return;

        const [removed] = script.events.splice(fromIndex, 1);
        script.events.splice(toIndex, 0, removed);

        // Recalculate timestamps to maintain sequential order
        let accTime = 0;
        for (let i = 0; i < script.events.length; i++) {
          script.events[i].timestamp = accTime;
          accTime += (script.events[i].duration || 100) + 50; // event duration + small gap
        }
        state.isScriptDirty = true;
        // Update editing index to follow the moved item
        if (state.editingEventIndex === fromIndex) {
          state.editingEventIndex = toIndex;
        }
      });
    },

    insertWaitEvent: (afterIndex: number, durationMs: number) => {
      set((state: AutomationState) => {
        const script = state.currentScript || state.selectedScript;
        if (!script || !script.events) return;
        if (afterIndex < -1 || afterIndex >= script.events.length) return;

        const prevEvent = afterIndex >= 0 ? script.events[afterIndex] : null;
        const timestamp = prevEvent ? prevEvent.timestamp + (prevEvent.duration || 0) + 50 : 0;

        const waitEvent = main.TouchEvent.createFrom({
          type: 'wait',
          timestamp,
          x: 0,
          y: 0,
          duration: durationMs,
        });

        script.events.splice(afterIndex + 1, 0, waitEvent);
        state.isScriptDirty = true;
      });
    },

    // Event subscription
    subscribeToEvents: () => {
      const handleRecordStarted = (data: any) => {
        set((state: AutomationState) => {
          state.isRecording = true;
          state.recordingDeviceId = data.deviceId;
          state.recordingStartTime = data.startTime * 1000;
          state.recordedActionCount = 0;
        });
      };

      const handleRecordStopped = (data: any) => {
        set((state: AutomationState) => {
          state.isRecording = false;
          state.recordingDeviceId = null;
          state.recordingStartTime = null;
          state.recordingDuration = 0;
          state.recordedActionCount = 0;
        });
      };

      const handleTouchActionRecorded = (data: any) => {
        set((state: AutomationState) => {
          state.recordedActionCount += 1;
        });
      };

      const handlePlaybackStarted = (data: any) => {
        set((state: AutomationState) => {
          state.isPlaying = true;
          state.playingDeviceId = data.deviceId;
          state.playbackProgress = { current: 0, total: data.total };
        });
      };

      const handlePlaybackProgress = (data: any) => {
        set((state: AutomationState) => {
          if (state.playbackProgress) {
            state.playbackProgress.current = data.current;
            state.playbackProgress.total = data.total;
          }
        });
      };

      const handlePlaybackCompleted = (data: any) => {
        set((state: AutomationState) => {
          state.isPlaying = false;
          state.playingDeviceId = null;
          state.playbackProgress = null;
        });
      };

      const handleTaskStarted = (data: any) => {
        set((state: AutomationState) => {
          state.isTaskRunning = true;
          state.isPaused = false;
          state.runningTaskName = data.taskName;
          state.playingDeviceId = data.deviceId;
        });
      };

      const handleTaskCompleted = (data: any) => {
        set((state: AutomationState) => {
          state.isTaskRunning = false;
          state.isPaused = false;
          state.runningTaskName = null;
          state.taskProgress = null;
          state.playingDeviceId = null;
        });
      };

      const handleTaskPaused = (data: any) => set((state: AutomationState) => {
        state.isPaused = true;
      });
      const handleTaskResumed = (data: any) => set((state: AutomationState) => {
        state.isPaused = false;
      });

      const handleTaskStepRunning = (data: any) => {
        set((state: AutomationState) => {
          state.taskProgress = {
            stepIndex: data.stepIndex,
            totalSteps: data.totalSteps,
            currentLoop: data.currentLoop,
            totalLoops: data.totalLoops,
            currentAction: data.currentAction || (data.type === 'wait' ? `Wait ${data.value}ms` : `Running: ${data.value}`),
          };
        });
      };

      const handleRecordingPausedForSelector = (data: any) => {
        set((state: AutomationState) => {
          state.isWaitingForSelector = true;
          state.pendingSelectorData = data;
          state.isAnalyzing = false;
        });
      };

      const handleRecordingResumed = (data: any) => {
        set((state: AutomationState) => {
          state.isWaitingForSelector = false;
          state.pendingSelectorData = null;
          state.isAnalyzing = false;
        });
      };

      const offPreCaptureStarted = EventsOn('recording-pre-capture-started', (data: any) => {
        set((state: AutomationState) => {
          state.isPreCapturing = true;
        });
      });

      const offPreCaptureFinished = EventsOn('recording-pre-capture-finished', (data: any) => {
        set((state: AutomationState) => {
          state.isPreCapturing = false;
        });
      });

      const offAnalysisStarted = EventsOn('recording-analysis-started', (data: any) => {
        set((state: AutomationState) => {
          state.isAnalyzing = true;
        });
      });

      // 用 EventsOn 返回的 cancel 函数精确退订：task-paused/task-resumed 与
      // WorkflowView 共享，EventsOff(name) 会误删对方的监听器（其余通道顺带统一）
      const cancels = [
        EventsOn('touch-record-started', handleRecordStarted),
        EventsOn('touch-record-stopped', handleRecordStopped),
        EventsOn('touch-action-recorded', handleTouchActionRecorded),
        EventsOn('touch-playback-started', handlePlaybackStarted),
        EventsOn('touch-playback-progress', handlePlaybackProgress),
        EventsOn('touch-playback-completed', handlePlaybackCompleted),
        EventsOn('task-started', handleTaskStarted),
        EventsOn('task-completed', handleTaskCompleted),
        EventsOn('task-step-running', handleTaskStepRunning),
        EventsOn('task-paused', handleTaskPaused),
        EventsOn('task-resumed', handleTaskResumed),
        EventsOn('recording-paused-for-selector', handleRecordingPausedForSelector),
        EventsOn('recording-resumed', handleRecordingResumed),
        offPreCaptureStarted,
        offPreCaptureFinished,
        offAnalysisStarted,
      ];

      return () => {
        cancels.forEach((off) => off?.());
      };
    },
  }))
);
