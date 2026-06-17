import { create } from 'zustand';
import { immer } from 'zustand/middleware/immer';

export interface WSMessageInfo {
  connectionId: string;
  direction: string; // "send" | "receive"
  typeName: string;  // "text" | "binary" | "ping" | "pong" | "close"
  payload: string;
  payloadSize: number;
  isBinary: boolean;
  isProtobuf?: boolean;
}

export interface RequestLog {
  id: string;
  time: string;
  clientIp: string;
  method: string;
  url: string;
  headers?: Record<string, string[]>;
  body?: string;
  isHttps: boolean;
  statusCode?: number;
  contentType?: string;
  bodySize?: number;
  previewBody?: string;
  respHeaders?: Record<string, string[]>;
  respBody?: string;
  isWs?: boolean;
  isWsMessage?: boolean;  // true for individual WS message rows (vs WS connection)
  wsMessageInfo?: WSMessageInfo;
  mocked?: boolean;
  isProtobuf?: boolean;
  isReqProtobuf?: boolean;
  timestamp?: number;
  duration?: number;
  partialUpdate?: boolean;
}

export interface WSMessage {
  connectionId: string;
  direction: string; // "send" | "receive"
  type: number;
  typeName: string;
  payload: string;
  payloadSize: number;
  isBinary: boolean;
  isProtobuf?: boolean;
  timestamp: number;
}

export interface NetworkStats {
  rxSpeed: number;
  txSpeed: number;
  rxBytes: number;
  txBytes: number;
  time: number;
}

export interface MockCondition {
  type: string;   // "header", "query", "body"
  key: string;
  operator: string; // "equals", "contains", "regex", "exists", "not_exists"
  value: string;
}

// Hints extracted from a captured request for pre-filling condition fields
export interface MockConditionHints {
  headers: Array<{ key: string; value: string }>;
  queryParams: Array<{ key: string; value: string }>;
}

interface MockRule {
  id: string;
  urlPattern: string;
  method?: string;
  statusCode: number;
  headers: Record<string, string>;
  body: string;
  bodyFile?: string;
  delay?: number;
  description?: string;
  enabled: boolean;
  createdAt?: number;
  conditions?: MockCondition[];
}

// Data for pre-filling a mock rule from another view (e.g. EventTimeline)
export interface PendingMockData {
  urlPattern: string;
  method?: string;
  statusCode: number;
  contentType?: string;
  body?: string;
  description?: string;
}

// Data for pre-filling a breakpoint rule from another view (e.g. EventTimeline)
export interface PendingBreakpointData {
  urlPattern: string;
  method?: string;
  phase?: string;
  description?: string;
}

// Proto file entry
export interface ProtoFileEntry {
  id: string;
  name: string;
  content: string;
  loadedAt: number;
}

// Proto URL→message mapping
export interface ProtoMapping {
  id: string;
  urlPattern: string;
  messageType: string;
  direction: string;
  description: string;
}

// Map Remote rule
export interface MapRemoteRule {
  id: string;
  sourcePattern: string;
  targetURL: string;
  method: string;
  enabled: boolean;
  description: string;
  createdAt?: number;
}

// Auto Rewrite rule
export interface RewriteRule {
  id: string;
  urlPattern: string;
  method: string;
  phase: string;      // "request", "response", "both"
  target: string;     // "header" or "body"
  headerName: string; // header name when target is "header"
  match: string;      // regex pattern
  replace: string;    // replacement string
  enabled: boolean;
  description: string;
  createdAt?: number;
}

// Breakpoint rule
export interface BreakpointRuleItem {
  id: string;
  urlPattern: string;
  method: string;   // empty = match all
  phase: string;    // "request", "response", "both"
  enabled: boolean;
  description: string;
  createdAt?: number;
}

// Pending breakpoint (paused request/response)
export interface PendingBreakpointItem {
  id: string;
  ruleId: string;
  phase: string;  // "request" or "response"
  method: string;
  url: string;
  headers?: Record<string, string[]>;
  body?: string;
  // Response fields (only for response phase)
  statusCode?: number;
  respHeaders?: Record<string, string[]>;
  respBody?: string;
  createdAt: number;
}

// Editable header entry for breakpoint editing
export interface EditableHeader {
  key: string;
  value: string;
}

// Breakpoint editing state (populated when resolve modal opens)
export interface BreakpointEditState {
  method: string;
  url: string;
  headers: EditableHeader[];
  queryParams: EditableHeader[];
  body: string;
  statusCode: number;
  respHeaders: EditableHeader[];
  respBody: string;
}

// Parse query parameters from a URL string into editable key-value pairs
export function urlToQueryParams(url: string): EditableHeader[] {
  try {
    const qIdx = url.indexOf('?');
    if (qIdx === -1) return [];
    const search = url.substring(qIdx + 1);
    const params = new URLSearchParams(search);
    const result: EditableHeader[] = [];
    params.forEach((value, key) => {
      result.push({ key, value });
    });
    return result;
  } catch {
    return [];
  }
}

// Rebuild URL from base + edited query params
export function rebuildUrlWithQuery(url: string, queryParams: EditableHeader[]): string {
  const qIdx = url.indexOf('?');
  const base = qIdx === -1 ? url : url.substring(0, qIdx);
  const validParams = queryParams.filter(p => p.key.trim());
  if (validParams.length === 0) return base;
  const search = validParams.map(p => `${encodeURIComponent(p.key)}=${encodeURIComponent(p.value)}`).join('&');
  return `${base}?${search}`;
}

// Try to pretty-print JSON string, return original if not valid JSON
export function prettyJson(s: string): string {
  if (!s) return s;
  const trimmed = s.trim();
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return s;
  try { return JSON.stringify(JSON.parse(trimmed), null, 2); } catch { return s; }
}

// Convert backend headers (Record<string, string[]>) to editable format
export function headersToEditable(headers: Record<string, string[]> | undefined): EditableHeader[] {
  if (!headers) return [];
  return Object.entries(headers).map(([key, values]) => ({
    key,
    value: Array.isArray(values) ? values.join(', ') : String(values),
  }));
}

// Convert editable headers back to modifications format (Record<string, string>)
export function editableToModHeaders(headers: EditableHeader[]): Record<string, string> {
  const result: Record<string, string> = {};
  for (const { key, value } of headers) {
    const trimmedKey = key.trim();
    if (trimmedKey) {
      result[trimmedKey] = value;
    }
  }
  return result;
}

// Build modifications object by comparing edit state with original breakpoint
export function buildModifications(
  bp: PendingBreakpointItem,
  edit: BreakpointEditState
): Record<string, any> | undefined {
  const mods: Record<string, any> = {};

  if (bp.phase === 'request') {
    if (edit.method !== bp.method) mods.method = edit.method;
    // Rebuild URL from query params and compare
    const rebuiltUrl = rebuildUrlWithQuery(edit.url, edit.queryParams);
    if (rebuiltUrl !== bp.url) mods.url = rebuiltUrl;
    if (edit.body !== prettyJson(bp.body || '')) mods.body = edit.body;
    // Compare headers
    const origHeaders = headersToEditable(bp.headers);
    if (JSON.stringify(edit.headers.filter(h => h.key.trim())) !== JSON.stringify(origHeaders)) {
      mods.headers = editableToModHeaders(edit.headers);
    }
  } else {
    // Response phase
    if (edit.statusCode !== (bp.statusCode || 200)) mods.statusCode = edit.statusCode;
    if (edit.respBody !== prettyJson(bp.respBody || '')) mods.respBody = edit.respBody;
    // Compare response headers
    const origRespHeaders = headersToEditable(bp.respHeaders);
    if (JSON.stringify(edit.respHeaders.filter(h => h.key.trim())) !== JSON.stringify(origRespHeaders)) {
      mods.respHeaders = editableToModHeaders(edit.respHeaders);
    }
  }

  return Object.keys(mods).length > 0 ? mods : undefined;
}

interface ProxyState {
  // 核心数据
  logs: RequestLog[];
  isRunning: boolean;
  port: number;
  localIP: string;
  netStats: NetworkStats;
  
  // 配置
  wsEnabled: boolean;
  mitmEnabled: boolean;
  latency: number | null;
  bypassPatterns: string[];
  dlLimit: number | null;
  ulLimit: number | null;
  
  // 过滤
  filterType: "ALL" | "HTTP" | "WS";
  searchText: string;
  // Debounced copy of searchText (200ms) — use this for expensive log filtering
  debouncedSearchText: string;
  
  // UI 状态
  selectedLog: RequestLog | null;
  detailsDrawerOpen: boolean;
  isBypassModalOpen: boolean;
  newPattern: string;
  
  // Resend 请求相关
  resendModalOpen: boolean;
  resendLoading: boolean;
  resendResponse: any;
  resendMode: 'compose' | 'resend';
  
  // Mock 规则相关
  mockListModalOpen: boolean;
  mockEditModalOpen: boolean;
  mockRules: MockRule[];
  editingMockRule: MockRule | null;
  pendingMockData: PendingMockData | null;
  // 证书状态
  certTrustStatus: string | null;
  
  // Condition hints from captured request
  mockConditionHints: MockConditionHints | null;
  
  // Proto management
  protoFiles: ProtoFileEntry[];
  protoMappings: ProtoMapping[];
  protoMessageTypes: string[];
  protoListModalOpen: boolean;
  protoEditFileModalOpen: boolean;
  protoEditMappingModalOpen: boolean;
  editingProtoFile: ProtoFileEntry | null;
  editingProtoMapping: ProtoMapping | null;
  protoImportLoading: boolean;
  protoImportURLModalOpen: boolean;
  protoImportURL: string;
  
  // Map Remote
  mapRemoteRules: MapRemoteRule[];
  mapRemoteListModalOpen: boolean;
  mapRemoteEditModalOpen: boolean;
  editingMapRemoteRule: MapRemoteRule | null;
  
  // Rewrite
  rewriteRules: RewriteRule[];
  rewriteListModalOpen: boolean;
  rewriteEditModalOpen: boolean;
  editingRewriteRule: RewriteRule | null;

  // Diff
  diffBaseLog: RequestLog | null;
  diffTargetLog: RequestLog | null;
  diffModalOpen: boolean;

  // WebSocket Messages
  wsMessages: Record<string, WSMessage[]>; // keyed by connectionId

  // Breakpoint
  breakpointRules: BreakpointRuleItem[];
  pendingBreakpoints: PendingBreakpointItem[];
  breakpointListModalOpen: boolean;
  breakpointEditModalOpen: boolean;
  editingBreakpointRule: BreakpointRuleItem | null;
  breakpointResolveModalOpen: boolean;
  selectedBreakpoint: PendingBreakpointItem | null;
  breakpointEdit: BreakpointEditState | null;
  pendingBreakpointData: PendingBreakpointData | null;
  
  // 操作方法
  addLog: (log: RequestLog) => void;
  updateLog: (id: string, updates: Partial<RequestLog>) => void;
  clearLogs: () => void;
  setProxyRunning: (running: boolean) => void;
  setPort: (port: number) => void;
  setLocalIP: (ip: string) => void;
  setNetStats: (stats: NetworkStats) => void;
  toggleWS: () => void;
  toggleMITM: () => void;
  setFilterType: (type: "ALL" | "HTTP" | "WS") => void;
  setSearchText: (text: string) => void;
  setLatency: (ms: number | null) => void;
  setSpeedLimits: (dl: number | null, ul: number | null) => void;
  setBypassPatterns: (patterns: string[]) => void;
  addBypassPattern: (pattern: string) => void;
  removeBypassPattern: (pattern: string) => void;
  selectLog: (log: RequestLog | null) => void;
  setDetailsDrawerOpen: (open: boolean) => void;
  setBypassModalOpen: (open: boolean) => void;
  setNewPattern: (pattern: string) => void;
  
  // Resend 请求
  setResendModalOpen: (open: boolean) => void;
  setResendLoading: (loading: boolean) => void;
  setResendResponse: (response: any) => void;
  setResendMode: (mode: 'compose' | 'resend') => void;
  openResendModal: (mode?: 'compose' | 'resend') => void;
  closeResendModal: () => void;
  
  // Mock 规则
  setMockListModalOpen: (open: boolean) => void;
  setMockEditModalOpen: (open: boolean) => void;
  setMockRules: (rules: MockRule[]) => void;
  setEditingMockRule: (rule: MockRule | null) => void;
  openMockListModal: () => void;
  closeMockListModal: () => void;
  openMockEditModal: (rule?: MockRule | null) => void;
  closeMockEditModal: () => void;
  setPendingMockData: (data: PendingMockData | null) => void;
  openMockWithPrefill: (data: PendingMockData) => void;
  setMockConditionHints: (hints: MockConditionHints | null) => void;
  
  // 证书状态
  setCertTrustStatus: (status: string | null) => void;
  
  // Proto management
  setProtoFiles: (files: ProtoFileEntry[]) => void;
  setProtoMappings: (mappings: ProtoMapping[]) => void;
  setProtoMessageTypes: (types: string[]) => void;
  openProtoListModal: () => void;
  closeProtoListModal: () => void;
  openProtoEditFileModal: (file?: ProtoFileEntry | null) => void;
  closeProtoEditFileModal: () => void;
  openProtoEditMappingModal: (mapping?: ProtoMapping | null) => void;
  closeProtoEditMappingModal: () => void;
    setProtoImportLoading: (loading: boolean) => void;
    openProtoImportURLModal: () => void;
    closeProtoImportURLModal: () => void;
    setProtoImportURL: (url: string) => void;
  
  // Map Remote
  setMapRemoteRules: (rules: MapRemoteRule[]) => void;
  openMapRemoteListModal: () => void;
  closeMapRemoteListModal: () => void;
  openMapRemoteEditModal: (rule?: MapRemoteRule | null) => void;
  closeMapRemoteEditModal: () => void;
  
  // Rewrite
  setRewriteRules: (rules: RewriteRule[]) => void;
  openRewriteListModal: () => void;
  closeRewriteListModal: () => void;
  openRewriteEditModal: (rule?: RewriteRule | null) => void;
  closeRewriteEditModal: () => void;

  // Diff
  setDiffBaseLog: (log: RequestLog | null) => void;
  openDiffModal: (base: RequestLog, target: RequestLog) => void;
  closeDiffModal: () => void;

  // WebSocket Messages
  addWSMessage: (msg: WSMessage) => void;
  clearWSMessages: (connectionId?: string) => void;

  // Breakpoint
  setBreakpointRules: (rules: BreakpointRuleItem[]) => void;
  openBreakpointListModal: () => void;
  closeBreakpointListModal: () => void;
  openBreakpointEditModal: (rule?: BreakpointRuleItem | null) => void;
  closeBreakpointEditModal: () => void;
  addPendingBreakpoint: (bp: PendingBreakpointItem) => void;
  removePendingBreakpoint: (id: string) => void;
  clearPendingBreakpoints: () => void;
  openBreakpointResolveModal: (bp: PendingBreakpointItem) => void;
  closeBreakpointResolveModal: () => void;
  updateBreakpointEdit: (updates: Partial<BreakpointEditState>) => void;
  setPendingBreakpointData: (data: PendingBreakpointData | null) => void;
  openBreakpointWithPrefill: (data: PendingBreakpointData) => void;
}

// Debounce timer for search text (module-level: store is a singleton)
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null;

export const useProxyStore = create<ProxyState>()(
  immer((set) => ({
    // 初始状态
    logs: [],
    isRunning: false,
    port: 8080,
    localIP: "",
    netStats: { rxSpeed: 0, txSpeed: 0, rxBytes: 0, txBytes: 0, time: 0 },
    
    wsEnabled: true,
    mitmEnabled: true,
    latency: null,
    bypassPatterns: [],
    dlLimit: null,
    ulLimit: null,
    
    filterType: "ALL",
    searchText: "",
    debouncedSearchText: "",
    
    selectedLog: null,
    detailsDrawerOpen: false,
    isBypassModalOpen: false,
    newPattern: "",
    
    // Resend 请求
    resendModalOpen: false,
    resendLoading: false,
    resendResponse: null,
    resendMode: 'compose' as const,
    
    // Mock 规则
    mockListModalOpen: false,
    mockEditModalOpen: false,
    mockRules: [],
    editingMockRule: null,
    pendingMockData: null,
    
    // 证书状态
    certTrustStatus: null,
    
    // Condition hints
    mockConditionHints: null,
    
    // Proto management
    protoFiles: [],
    protoMappings: [],
    protoMessageTypes: [],
    protoListModalOpen: false,
    protoEditFileModalOpen: false,
    protoEditMappingModalOpen: false,
    editingProtoFile: null,
    editingProtoMapping: null,
    protoImportLoading: false,
    protoImportURLModalOpen: false,
    protoImportURL: 'https://',
    
    // Map Remote
    mapRemoteRules: [],
    mapRemoteListModalOpen: false,
    mapRemoteEditModalOpen: false,
    editingMapRemoteRule: null,
    
    // Rewrite
    rewriteRules: [],
    rewriteListModalOpen: false,
    rewriteEditModalOpen: false,
    editingRewriteRule: null,

    // Diff
    diffBaseLog: null,
    diffTargetLog: null,
    diffModalOpen: false,

    // WebSocket Messages
    wsMessages: {},

    // Breakpoint
    breakpointRules: [],
    pendingBreakpoints: [],
    breakpointListModalOpen: false,
    breakpointEditModalOpen: false,
    editingBreakpointRule: null,
    breakpointResolveModalOpen: false,
    selectedBreakpoint: null,
    breakpointEdit: null,
    pendingBreakpointData: null,
    
    // 操作方法
    addLog: (log: RequestLog) => set((state: ProxyState) => {
      state.logs.unshift(log);
      if (state.logs.length > 5000) {
        state.logs.pop();
      }
    }),
    
    updateLog: (id: string, updates: Partial<RequestLog>) => set((state: ProxyState) => {
      const log = state.logs.find(l => l.id === id);
      if (log) {
        Object.assign(log, updates);
        // Also update selectedLog if it's the same log
        if (state.selectedLog && state.selectedLog.id === id) {
          Object.assign(state.selectedLog, updates);
        }
      }
    }),
    
    clearLogs: () => set({ logs: [] }),
    
    setProxyRunning: (running: boolean) => set({ isRunning: running }),
    
    setPort: (port: number) => set({ port }),
    
    setLocalIP: (ip: string) => set({ localIP: ip }),
    
    setNetStats: (stats: NetworkStats) => set({ netStats: stats }),
    
    toggleWS: () => set((state: ProxyState) => {
      state.wsEnabled = !state.wsEnabled;
    }),
    
    toggleMITM: () => set((state: ProxyState) => {
      state.mitmEnabled = !state.mitmEnabled;
    }),
    
    setFilterType: (type: "ALL" | "HTTP" | "WS") => set({ filterType: type }),
    
    setSearchText: (text: string) => {
      set({ searchText: text });
      if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
      if (!text) {
        // Clearing the search should take effect immediately
        searchDebounceTimer = null;
        set({ debouncedSearchText: "" });
      } else {
        searchDebounceTimer = setTimeout(() => set({ debouncedSearchText: text }), 200);
      }
    },
    
    setLatency: (ms: number | null) => set({ latency: ms }),
    
    setSpeedLimits: (dl: number | null, ul: number | null) => set({ dlLimit: dl, ulLimit: ul }),
    
    setBypassPatterns: (patterns: string[]) => set({ bypassPatterns: patterns }),
    
    addBypassPattern: (pattern: string) => set((state: ProxyState) => {
      state.bypassPatterns.push(pattern);
    }),
    
    removeBypassPattern: (pattern: string) => set((state: ProxyState) => {
      state.bypassPatterns = state.bypassPatterns.filter(p => p !== pattern);
    }),
    
    selectLog: (log: RequestLog | null) => set({ selectedLog: log }),
    
    setDetailsDrawerOpen: (open: boolean) => set({ detailsDrawerOpen: open }),
    
    setBypassModalOpen: (open: boolean) => set({ isBypassModalOpen: open }),
    
    setNewPattern: (pattern: string) => set({ newPattern: pattern }),
    
    // Resend 请求
    setResendModalOpen: (open: boolean) => set({ resendModalOpen: open }),
    
    setResendLoading: (loading: boolean) => set({ resendLoading: loading }),
    
    setResendResponse: (response: any) => set({ resendResponse: response }),
    
    setResendMode: (mode: 'compose' | 'resend') => set({ resendMode: mode }),
    
    openResendModal: (mode: 'compose' | 'resend' = 'compose') => set({ resendModalOpen: true, resendResponse: null, resendMode: mode }),
    
    closeResendModal: () => set({ resendModalOpen: false, resendLoading: false, resendResponse: null }),
    
    // Mock 规则
    setMockListModalOpen: (open: boolean) => set({ mockListModalOpen: open }),
    
    setMockEditModalOpen: (open: boolean) => set({ mockEditModalOpen: open }),
    
    setMockRules: (rules: MockRule[]) => set({ mockRules: rules }),
    
    setEditingMockRule: (rule: MockRule | null) => set({ editingMockRule: rule }),
    
    openMockListModal: () => set({ mockListModalOpen: true }),
    
    closeMockListModal: () => set({ mockListModalOpen: false }),
    
    openMockEditModal: (rule) => set({
      mockEditModalOpen: true,
      editingMockRule: rule ?? null,
    }),
    
    closeMockEditModal: () => set({ mockEditModalOpen: false, editingMockRule: null }),
    
    setPendingMockData: (data: PendingMockData | null) => set({ pendingMockData: data }),
    
    openMockWithPrefill: (data: PendingMockData) => set({
      pendingMockData: data,
      mockEditModalOpen: true,
      editingMockRule: null,
      mockConditionHints: null,
    }),
    
    setMockConditionHints: (hints: MockConditionHints | null) => set({ mockConditionHints: hints }),
    
    // 证书状态
    setCertTrustStatus: (status: string | null) => set({ certTrustStatus: status }),
    
    // Proto management
    setProtoFiles: (files: ProtoFileEntry[]) => set({ protoFiles: files }),
    setProtoMappings: (mappings: ProtoMapping[]) => set({ protoMappings: mappings }),
    setProtoMessageTypes: (types: string[]) => set({ protoMessageTypes: types }),
    openProtoListModal: () => set({ protoListModalOpen: true }),
    closeProtoListModal: () => set({ protoListModalOpen: false }),
    openProtoEditFileModal: (file) => set({ protoEditFileModalOpen: true, editingProtoFile: file ?? null }),
    closeProtoEditFileModal: () => set({ protoEditFileModalOpen: false, editingProtoFile: null }),
    openProtoEditMappingModal: (mapping) => set({ protoEditMappingModalOpen: true, editingProtoMapping: mapping ?? null }),
    closeProtoEditMappingModal: () => set({ protoEditMappingModalOpen: false, editingProtoMapping: null }),
    setProtoImportLoading: (loading) => set({ protoImportLoading: loading }),
    openProtoImportURLModal: () => set({ protoImportURLModalOpen: true, protoImportURL: 'https://' }),
    closeProtoImportURLModal: () => set({ protoImportURLModalOpen: false, protoImportURL: 'https://' }),
    setProtoImportURL: (url: string) => set({ protoImportURL: url }),
    
    // Map Remote
    setMapRemoteRules: (rules: MapRemoteRule[]) => set({ mapRemoteRules: rules }),
    openMapRemoteListModal: () => set({ mapRemoteListModalOpen: true }),
    closeMapRemoteListModal: () => set({ mapRemoteListModalOpen: false }),
    openMapRemoteEditModal: (rule) => set({ mapRemoteEditModalOpen: true, editingMapRemoteRule: rule ?? null }),
    closeMapRemoteEditModal: () => set({ mapRemoteEditModalOpen: false, editingMapRemoteRule: null }),
    
    // Rewrite
    setRewriteRules: (rules: RewriteRule[]) => set({ rewriteRules: rules }),
    openRewriteListModal: () => set({ rewriteListModalOpen: true }),
    closeRewriteListModal: () => set({ rewriteListModalOpen: false }),
    openRewriteEditModal: (rule) => set({ rewriteEditModalOpen: true, editingRewriteRule: rule ?? null }),
    closeRewriteEditModal: () => set({ rewriteEditModalOpen: false, editingRewriteRule: null }),

    // Diff
    setDiffBaseLog: (log: RequestLog | null) => set({ diffBaseLog: log }),
    openDiffModal: (base: RequestLog, target: RequestLog) => set({ diffModalOpen: true, diffBaseLog: base, diffTargetLog: target }),
    closeDiffModal: () => set({ diffModalOpen: false, diffBaseLog: null, diffTargetLog: null }),

    // WebSocket Messages
    addWSMessage: (msg: WSMessage) => set((state: ProxyState) => {
      if (!state.wsMessages[msg.connectionId]) {
        state.wsMessages[msg.connectionId] = [];
      }
      state.wsMessages[msg.connectionId].push(msg);
      // Limit per connection to prevent memory bloat
      if (state.wsMessages[msg.connectionId].length > 1000) {
        state.wsMessages[msg.connectionId] = state.wsMessages[msg.connectionId].slice(-500);
      }
    }),
    clearWSMessages: (connectionId?: string) => set((state: ProxyState) => {
      if (connectionId) {
        delete state.wsMessages[connectionId];
      } else {
        state.wsMessages = {};
      }
    }),

    // Breakpoint
    setBreakpointRules: (rules: BreakpointRuleItem[]) => set({ breakpointRules: rules }),
    openBreakpointListModal: () => set({ breakpointListModalOpen: true }),
    closeBreakpointListModal: () => set({ breakpointListModalOpen: false }),
    openBreakpointEditModal: (rule) => set({ breakpointEditModalOpen: true, editingBreakpointRule: rule ?? null }),
    closeBreakpointEditModal: () => set({ breakpointEditModalOpen: false, editingBreakpointRule: null }),
    addPendingBreakpoint: (bp: PendingBreakpointItem) => set((state: ProxyState) => {
      // Avoid duplicates
      if (!state.pendingBreakpoints.find(p => p.id === bp.id)) {
        state.pendingBreakpoints.push(bp);
      }
    }),
    removePendingBreakpoint: (id: string) => set((state: ProxyState) => {
      state.pendingBreakpoints = state.pendingBreakpoints.filter(p => p.id !== id);
      // Close resolve modal if the resolved breakpoint was selected
      if (state.selectedBreakpoint?.id === id) {
        state.breakpointResolveModalOpen = false;
        state.selectedBreakpoint = null;
        state.breakpointEdit = null;
      }
    }),
    clearPendingBreakpoints: () => set({ pendingBreakpoints: [], breakpointResolveModalOpen: false, selectedBreakpoint: null, breakpointEdit: null }),
    openBreakpointResolveModal: (bp: PendingBreakpointItem) => set({
      breakpointResolveModalOpen: true,
      selectedBreakpoint: bp,
      breakpointEdit: {
        method: bp.method,
        url: bp.url,
        headers: headersToEditable(bp.headers),
        queryParams: urlToQueryParams(bp.url),
        body: prettyJson(bp.body || ''),
        statusCode: bp.statusCode || 200,
        respHeaders: headersToEditable(bp.respHeaders),
        respBody: prettyJson(bp.respBody || ''),
      },
    }),
    closeBreakpointResolveModal: () => set({ breakpointResolveModalOpen: false, selectedBreakpoint: null, breakpointEdit: null }),
    updateBreakpointEdit: (updates: Partial<BreakpointEditState>) => set((state: ProxyState) => {
      if (state.breakpointEdit) {
        Object.assign(state.breakpointEdit, updates);
      }
    }),
    setPendingBreakpointData: (data: PendingBreakpointData | null) => set({ pendingBreakpointData: data }),
    openBreakpointWithPrefill: (data: PendingBreakpointData) => set({
      pendingBreakpointData: data,
      breakpointEditModalOpen: true,
      editingBreakpointRule: null,
    }),
  }))
);
