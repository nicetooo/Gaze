import { create } from 'zustand';

const EventsOn = (window as any).runtime.EventsOn;
const EventsOff = (window as any).runtime.EventsOff;

// ========================================
// Unified Element Store
// Shared state for UI elements across Recording, Workflow, and UI Inspector
// ========================================

// Re-export UINode type for convenience (also defined in automationStore)
export interface UINode {
  text: string;
  resourceId: string;
  class: string;
  package: string;
  contentDesc: string;
  bounds: string;
  checkable: string;
  checked: string;
  clickable: string;
  enabled: string;
  focusable: string;
  focused: string;
  scrollable: string;
  longClickable: string;
  password: string;
  selected: string;
  nodes: UINode[];
}

export interface ElementSelector {
  type: 'text' | 'id' | 'desc' | 'class' | 'xpath' | 'bounds' | 'contains' | 'coordinates' | 'advanced';
  value: string;
  index?: number;
}

export interface SelectorSuggestion {
  type: string;
  value: string;
  priority: number;
  description: string;
}

export interface ElementInfo {
  x: number;
  y: number;
  class?: string;
  bounds?: string;
  selector?: ElementSelector;
  timestamp?: number;
}

export interface BoundsRect {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

// Parse Android bounds string "[x1,y1][x2,y2]"
export function parseBounds(bounds: string): BoundsRect | null {
  const match = bounds.match(/\[(\d+),(\d+)\]\[(\d+),(\d+)\]/);
  if (!match) return null;
  return {
    x1: parseInt(match[1], 10),
    y1: parseInt(match[2], 10),
    x2: parseInt(match[3], 10),
    y2: parseInt(match[4], 10),
  };
}

// Get center point of bounds
export function getBoundsCenter(bounds: string): { x: number; y: number } | null {
  const rect = parseBounds(bounds);
  if (!rect) return null;
  return {
    x: rect.x1 + Math.floor((rect.x2 - rect.x1) / 2),
    y: rect.y1 + Math.floor((rect.y2 - rect.y1) / 2),
  };
}

// Check if point is inside bounds
export function isPointInBounds(x: number, y: number, bounds: string): boolean {
  const rect = parseBounds(bounds);
  if (!rect) return false;
  return x >= rect.x1 && x <= rect.x2 && y >= rect.y1 && y <= rect.y2;
}

// Find element at point in hierarchy
export function findElementAtPoint(node: UINode | null, x: number, y: number): UINode | null {
  if (!node) return null;

  // Check if point is in this node's bounds
  if (!isPointInBounds(x, y, node.bounds)) {
    return null;
  }

  // Check children (prefer smaller, more specific nodes)
  let bestMatch: UINode | null = null;
  let bestArea = Infinity;

  for (const child of node.nodes || []) {
    const found = findElementAtPoint(child, x, y);
    if (found) {
      const bounds = parseBounds(found.bounds);
      if (bounds) {
        const area = (bounds.x2 - bounds.x1) * (bounds.y2 - bounds.y1);
        if (area < bestArea) {
          bestArea = area;
          bestMatch = found;
        }
      }
    }
  }

  // Return best child match, or this node if no children contain the point
  return bestMatch || node;
}

// Find elements by selector in hierarchy
export function findElementsBySelector(
  node: UINode | null,
  selector: ElementSelector
): UINode[] {
  if (!node) return [];

  const results: UINode[] = [];
  const matchNode = (n: UINode): boolean => {
    switch (selector.type) {
      case 'text':
        return n.text === selector.value || n.contentDesc === selector.value;
      case 'id':
        return n.resourceId === selector.value || n.resourceId.endsWith(`:id/${selector.value}`);
      case 'desc':
        return n.contentDesc === selector.value;
      case 'class':
        return n.class === selector.value;
      case 'contains':
        return n.text.includes(selector.value) || n.contentDesc.includes(selector.value);
      case 'bounds':
        return n.bounds === selector.value;
      default:
        return false;
    }
  };

  const traverse = (n: UINode) => {
    if (matchNode(n)) {
      results.push(n);
    }
    for (const child of n.nodes || []) {
      traverse(child);
    }
  };

  traverse(node);
  return results;
}

// Get the best selector for a node
export function getBestSelector(node: UINode, root: UINode): ElementSelector | null {
  // Priority: unique text > unique id > desc > class
  if (node.text && isUniqueSelector(root, 'text', node.text)) {
    return { type: 'text', value: node.text };
  }
  if (node.resourceId && isUniqueSelector(root, 'id', node.resourceId)) {
    return { type: 'id', value: node.resourceId };
  }
  if (node.contentDesc && isUniqueSelector(root, 'desc', node.contentDesc)) {
    return { type: 'desc', value: node.contentDesc };
  }
  // Fallback to bounds
  if (node.bounds) {
    return { type: 'bounds', value: node.bounds };
  }
  return null;
}

// Check if a selector value is unique in the hierarchy
export function isUniqueSelector(root: UINode, type: string, value: string): boolean {
  const selector: ElementSelector = { type: type as any, value };
  return findElementsBySelector(root, selector).length === 1;
}

// Generate selector suggestions for a node
export function generateSelectorSuggestions(node: UINode, root: UINode): SelectorSuggestion[] {
  const suggestions: SelectorSuggestion[] = [];

  // Text selector
  if (node.text) {
    const isUnique = isUniqueSelector(root, 'text', node.text);
    suggestions.push({
      type: 'text',
      value: node.text,
      priority: isUnique ? 5 : 3,
      description: `Text: "${node.text}"${isUnique ? '' : ' (not unique)'}`,
    });
  }

  // Resource ID selector
  if (node.resourceId) {
    const isUnique = isUniqueSelector(root, 'id', node.resourceId);
    suggestions.push({
      type: 'id',
      value: node.resourceId,
      priority: isUnique ? 5 : 3,
      description: `ID: ${node.resourceId}${isUnique ? '' : ' (not unique)'}`,
    });
  }

  // Content description selector
  if (node.contentDesc) {
    const isUnique = isUniqueSelector(root, 'desc', node.contentDesc);
    suggestions.push({
      type: 'desc',
      value: node.contentDesc,
      priority: isUnique ? 4 : 3,
      description: `Description: "${node.contentDesc}"${isUnique ? '' : ' (not unique)'}`,
    });
  }

  // Class selector
  if (node.class) {
    const shortClass = node.class.split('.').pop() || node.class;
    suggestions.push({
      type: 'class',
      value: node.class,
      priority: 2,
      description: `Class: ${shortClass} (usually matches multiple)`,
    });
  }

  // Bounds selector
  if (node.bounds) {
    suggestions.push({
      type: 'bounds',
      value: node.bounds,
      priority: 1,
      description: `Bounds: ${node.bounds} (position dependent)`,
    });
  }

  // Sort by priority descending
  return suggestions.sort((a, b) => b.priority - a.priority);
}

interface ElementState {
  // Shared UI hierarchy
  hierarchy: UINode | null;
  rawXml: string | null;
  isLoading: boolean;
  lastFetchTime: number | null;
  lastFetchDeviceId: string | null;

  // Selected element state (for element picker)
  selectedNode: UINode | null;
  highlightedNode: UINode | null;

  // Element picker modal state
  isPickerOpen: boolean;
  pickerCallback: ((selector: ElementSelector | null) => void) | null;

  // Actions
  fetchHierarchy: (deviceId: string, force?: boolean) => Promise<UINode | null>;
  clearHierarchy: () => void;
  setSelectedNode: (node: UINode | null) => void;
  setHighlightedNode: (node: UINode | null) => void;
  openElementPicker: (callback: (selector: ElementSelector | null) => void) => void;
  closeElementPicker: () => void;
  confirmElementPicker: (selector: ElementSelector) => void;

  // Backend element operations
  clickElement: (deviceId: string, selector: ElementSelector) => Promise<void>;
  inputText: (deviceId: string, selector: ElementSelector, text: string) => Promise<void>;
  waitForElement: (deviceId: string, selector: ElementSelector, timeout?: number) => Promise<void>;
  getElementProperties: (deviceId: string, selector: ElementSelector) => Promise<Record<string, any>>;

  // Event subscription
  subscribeToEvents: () => () => void;
}

export const useElementStore = create<ElementState>((set, get) => ({
  // Initial state
  hierarchy: null,
  rawXml: null,
  isLoading: false,
  lastFetchTime: null,
  lastFetchDeviceId: null,
  selectedNode: null,
  highlightedNode: null,
  isPickerOpen: false,
  pickerCallback: null,

  // Actions
  fetchHierarchy: async (deviceId: string, force = false) => {
    const { lastFetchTime, lastFetchDeviceId, rawXml } = get();

    // Cache check: skip if same device and fetched within 2 seconds
    const now = Date.now();
    if (!force && lastFetchDeviceId === deviceId && lastFetchTime && now - lastFetchTime < 2000) {
      return get().hierarchy;
    }

    set({ isLoading: true });
    try {
      const result = await (window as any).go.main.App.GetUIHierarchy(deviceId);

      // Only update if content changed
      if (result.rawXml !== rawXml) {
        set({
          hierarchy: result.root,
          rawXml: result.rawXml,
          lastFetchTime: now,
          lastFetchDeviceId: deviceId,
          isLoading: false,
        });
      } else {
        set({ isLoading: false, lastFetchTime: now });
      }
      return result.root;
    } catch (err) {
      console.error('Failed to fetch UI hierarchy:', err);
      set({ isLoading: false });
      throw err;
    }
  },

  clearHierarchy: () => {
    set({
      hierarchy: null,
      rawXml: null,
      selectedNode: null,
      highlightedNode: null,
      lastFetchTime: null,
      lastFetchDeviceId: null,
    });
  },

  setSelectedNode: (node: UINode | null) => {
    set({ selectedNode: node });
  },

  setHighlightedNode: (node: UINode | null) => {
    set({ highlightedNode: node });
  },

  openElementPicker: (callback: (selector: ElementSelector | null) => void) => {
    set({
      isPickerOpen: true,
      pickerCallback: callback,
      selectedNode: null,
    });
  },

  closeElementPicker: () => {
    const { pickerCallback } = get();
    if (pickerCallback) {
      pickerCallback(null);
    }
    set({
      isPickerOpen: false,
      pickerCallback: null,
      selectedNode: null,
    });
  },

  confirmElementPicker: (selector: ElementSelector) => {
    const { pickerCallback } = get();
    if (pickerCallback) {
      pickerCallback(selector);
    }
    set({
      isPickerOpen: false,
      pickerCallback: null,
      selectedNode: null,
    });
  },

  // Backend element operations
  clickElement: async (deviceId: string, selector: ElementSelector) => {
    await (window as any).go.main.App.ClickElement(
      { Cancelled: false } as any, // context placeholder
      deviceId,
      selector,
      null // use default config
    );
  },

  inputText: async (deviceId: string, selector: ElementSelector, text: string) => {
    await (window as any).go.main.App.InputTextToElement(
      { Cancelled: false } as any,
      deviceId,
      selector,
      text,
      false, // clearFirst
      null
    );
  },

  waitForElement: async (deviceId: string, selector: ElementSelector, timeout = 10000) => {
    await (window as any).go.main.App.WaitForElement(
      { Cancelled: false } as any,
      deviceId,
      selector,
      timeout
    );
  },

  getElementProperties: async (deviceId: string, selector: ElementSelector) => {
    return await (window as any).go.main.App.GetElementProperties(deviceId, selector);
  },

  // Event subscription
  subscribeToEvents: () => {
    // Listen for UI hierarchy updates from other sources
    const handleHierarchyUpdate = (data: any) => {
      if (data.root) {
        set({
          hierarchy: data.root,
          rawXml: data.rawXml,
          lastFetchTime: Date.now(),
          lastFetchDeviceId: data.deviceId,
        });
      }
    };

    EventsOn('ui-hierarchy-updated', handleHierarchyUpdate);

    return () => {
      EventsOff('ui-hierarchy-updated');
    };
  },
}));

// Export helper functions for use in components
export { parseBounds as parseElementBounds };
