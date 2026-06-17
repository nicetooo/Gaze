import React, { useEffect, useCallback, useMemo, type ReactNode } from 'react';
import { Card, Button, InputNumber, Space, Typography, Tag, Divider, Switch, Tooltip, Radio, Input, Tabs, theme, Form, Table, Popconfirm, Popover, Spin, App, Modal, Select, AutoComplete, Dropdown } from 'antd';
import { PoweroffOutlined, PlayCircleOutlined, DeleteOutlined, SettingOutlined, LockOutlined, GlobalOutlined, ArrowUpOutlined, ArrowDownOutlined, ApiOutlined, SafetyCertificateOutlined, DownloadOutlined, HourglassOutlined, CopyOutlined, BlockOutlined, SendOutlined, CloseOutlined, PlusOutlined, EditOutlined, CodeOutlined, CloudDownloadOutlined, FolderOpenOutlined, LoadingOutlined, MinusCircleOutlined, BugOutlined, FastForwardOutlined, StopOutlined, SwapOutlined } from '@ant-design/icons';
import VirtualList from './VirtualList';
import DeviceSelector from './DeviceSelector';
import JsonViewer from './JsonViewer';
import JsonEditor from './JsonEditor';
import { useShallow } from 'zustand/react/shallow';
import { useDeviceStore, useProxyStore, RequestLog as StoreRequestLog, WSMessage, WSMessageInfo } from '../stores';
import { buildModifications, type EditableHeader } from '../stores/proxyStore';
// @ts-ignore
import { StartProxy, StopProxy, GetProxyStatus, GetLocalIP, RunAdbCommand, StartNetworkMonitor, StopNetworkMonitor, SetProxyLimit, SetProxyWSEnabled, SetProxyMITM, InstallProxyCert, SetProxyLatency, SetMITMBypassPatterns, GetMITMBypassPatterns, SetProxyDevice, ResendRequest, AddMockRule, UpdateMockRule, RemoveMockRule, GetMockRules, ToggleMockRule, ExportMockRules, ImportMockRules, CheckCertTrust, SetupProxyForDevice, CleanupProxyForDevice, GetProtoFiles, AddProtoFile, UpdateProtoFile, RemoveProtoFile, GetProtoMappings, AddProtoMapping, UpdateProtoMapping, RemoveProtoMapping, GetProtoMessageTypes, LoadProtoFromURL, LoadProtoFromDisk, AddBreakpointRule, UpdateBreakpointRule, RemoveBreakpointRule, GetBreakpointRules, ToggleBreakpointRule, ResolveBreakpoint, GetPendingBreakpoints, ForwardAllBreakpoints, AddMapRemoteRule, UpdateMapRemoteRule, RemoveMapRemoteRule, GetMapRemoteRules, ToggleMapRemoteRule, AddRewriteRule, UpdateRewriteRule, RemoveRewriteRule, GetRewriteRules, ToggleRewriteRule } from '../../wailsjs/go/main/App';
// @ts-ignore
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import { useTranslation } from 'react-i18next';
import { generateCode, type CodeLanguage } from '../utils/codeGenerators';

const { Title, Text } = Typography;

// HTTP Method → color mapping (Postman-style)
const METHOD_COLORS: Record<string, string> = {
    GET: '#52c41a',     // green
    POST: '#1677ff',    // blue
    PUT: '#fa8c16',     // orange
    DELETE: '#f5222d',  // red
    PATCH: '#722ed1',   // purple
    HEAD: '#8c8c8c',    // grey
    OPTIONS: '#8c8c8c', // grey
};

const getMethodColor = (method: string): string => METHOD_COLORS[method?.toUpperCase()] || '#8c8c8c';

const HTTP_METHODS = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS'];

// Shared method select options with colored labels (used by compose request and all proxy rule forms)
const METHOD_SELECT_OPTIONS = HTTP_METHODS.map(m => ({
    label: <span style={{ color: getMethodColor(m), fontWeight: 600 }}>{m}</span>,
    value: m,
}));

// Status code → Ant Design Tag color
const getStatusColor = (code: number): string => {
    if (code >= 500) return 'red';
    if (code >= 400) return 'orange';
    if (code >= 300) return 'blue';
    if (code >= 200) return 'success';
    return 'default';
};

interface NetworkStats {
    rxSpeed: number;
    txSpeed: number;
    rxBytes: number;
    txBytes: number;
    time: number;
}

interface RequestLog {
    id: string;
    time: string;
    clientIp: string;
    method: string;
    url: string;
    headers: Record<string, string[]>;
    body: string; // Legacy field, might be unused now
    isHttps: boolean;

    // New fields
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
}

const ProxyView: React.FC = () => {
    const { t } = useTranslation();
    const { token } = theme.useToken();
    const { modal, message } = App.useApp();
    const selectedDevice = useDeviceStore((state) => state.selectedDevice);

    // Grouped useShallow selectors instead of whole-store subscriptions:
    // sets that don't touch the selected fields no longer re-render this view.
    // Proxy control / config group
    const {
        isRunning,
        port,
        localIP,
        wsEnabled,
        mitmEnabled,
        latency,
        bypassPatterns,
        isBypassModalOpen,
        newPattern,
        netStats,
        dlLimit,
        ulLimit,
        setProxyRunning,
        setPort,
        setLocalIP,
        toggleWS,
        toggleMITM,
        setLatency,
        setBypassModalOpen,
        setNewPattern,
        setNetStats,
        setSpeedLimits,
        addBypassPattern,
        removeBypassPattern,
    } = useProxyStore(useShallow((state) => ({
        isRunning: state.isRunning,
        port: state.port,
        localIP: state.localIP,
        wsEnabled: state.wsEnabled,
        mitmEnabled: state.mitmEnabled,
        latency: state.latency,
        bypassPatterns: state.bypassPatterns,
        isBypassModalOpen: state.isBypassModalOpen,
        newPattern: state.newPattern,
        netStats: state.netStats,
        dlLimit: state.dlLimit,
        ulLimit: state.ulLimit,
        setProxyRunning: state.setProxyRunning,
        setPort: state.setPort,
        setLocalIP: state.setLocalIP,
        toggleWS: state.toggleWS,
        toggleMITM: state.toggleMITM,
        setLatency: state.setLatency,
        setBypassModalOpen: state.setBypassModalOpen,
        setNewPattern: state.setNewPattern,
        setNetStats: state.setNetStats,
        setSpeedLimits: state.setSpeedLimits,
        addBypassPattern: state.addBypassPattern,
        removeBypassPattern: state.removeBypassPattern,
    })));

    // Capture list group (logs, filtering, selection)
    const {
        logs,
        filterType,
        searchText,
        debouncedSearchText,
        selectedLog,
        addLog,
        updateLog,
        clearLogs,
        setFilterType,
        setSearchText,
        selectLog,
    } = useProxyStore(useShallow((state) => ({
        logs: state.logs,
        filterType: state.filterType,
        searchText: state.searchText,
        debouncedSearchText: state.debouncedSearchText,
        selectedLog: state.selectedLog,
        addLog: state.addLog,
        updateLog: state.updateLog,
        clearLogs: state.clearLogs,
        setFilterType: state.setFilterType,
        setSearchText: state.setSearchText,
        selectLog: state.selectLog,
    })));

    const [resendForm] = Form.useForm();
    const [mockForm] = Form.useForm();
    const [breakpointForm] = Form.useForm();
    const [mapRemoteForm] = Form.useForm();
    const [rewriteForm] = Form.useForm();
    const [protoFileForm] = Form.useForm();
    const [protoMappingForm] = Form.useForm();

    // Watch conditions array to reactively disable/hide fields based on type & operator
    const watchedConditions: Array<{ type?: string; operator?: string }> | undefined = Form.useWatch('conditions', mockForm);

    // Watch method field in each form to filter URL pattern autocomplete by HTTP method
    const mockFormMethod: string | undefined = Form.useWatch('method', mockForm);
    const breakpointFormMethod: string | undefined = Form.useWatch('method', breakpointForm);
    const mapRemoteFormMethod: string | undefined = Form.useWatch('method', mapRemoteForm);
    const rewriteFormMethod: string | undefined = Form.useWatch('method', rewriteForm);

    // Resend + Mock rules group
    const {
        resendModalOpen,
        resendLoading,
        resendResponse,
        resendMode,
        mockListModalOpen,
        mockEditModalOpen,
        mockRules,
        editingMockRule,
        certTrustStatus,
        setResendModalOpen,
        setResendLoading,
        setResendResponse,
        openResendModal,
        closeResendModal,
        setMockRules,
        setEditingMockRule,
        openMockListModal,
        closeMockListModal,
        openMockEditModal,
        closeMockEditModal,
        setMockEditModalOpen,
        pendingMockData,
        setPendingMockData,
        mockConditionHints,
        setMockConditionHints,
        setCertTrustStatus,
    } = useProxyStore(useShallow((state) => ({
        resendModalOpen: state.resendModalOpen,
        resendLoading: state.resendLoading,
        resendResponse: state.resendResponse,
        resendMode: state.resendMode,
        mockListModalOpen: state.mockListModalOpen,
        mockEditModalOpen: state.mockEditModalOpen,
        mockRules: state.mockRules,
        editingMockRule: state.editingMockRule,
        certTrustStatus: state.certTrustStatus,
        setResendModalOpen: state.setResendModalOpen,
        setResendLoading: state.setResendLoading,
        setResendResponse: state.setResendResponse,
        openResendModal: state.openResendModal,
        closeResendModal: state.closeResendModal,
        setMockRules: state.setMockRules,
        setEditingMockRule: state.setEditingMockRule,
        openMockListModal: state.openMockListModal,
        closeMockListModal: state.closeMockListModal,
        openMockEditModal: state.openMockEditModal,
        closeMockEditModal: state.closeMockEditModal,
        setMockEditModalOpen: state.setMockEditModalOpen,
        pendingMockData: state.pendingMockData,
        setPendingMockData: state.setPendingMockData,
        mockConditionHints: state.mockConditionHints,
        setMockConditionHints: state.setMockConditionHints,
        setCertTrustStatus: state.setCertTrustStatus,
    })));

    // Proto management group
    const {
        protoFiles,
        protoMappings,
        protoMessageTypes,
        protoListModalOpen,
        protoEditFileModalOpen,
        protoEditMappingModalOpen,
        editingProtoFile,
        editingProtoMapping,
        setProtoFiles,
        setProtoMappings,
        setProtoMessageTypes,
        openProtoListModal,
        closeProtoListModal,
        openProtoEditFileModal,
        closeProtoEditFileModal,
        openProtoEditMappingModal,
        closeProtoEditMappingModal,
        protoImportLoading,
        setProtoImportLoading,
        protoImportURLModalOpen,
        protoImportURL,
        openProtoImportURLModal,
        closeProtoImportURLModal,
        setProtoImportURL,
    } = useProxyStore(useShallow((state) => ({
        protoFiles: state.protoFiles,
        protoMappings: state.protoMappings,
        protoMessageTypes: state.protoMessageTypes,
        protoListModalOpen: state.protoListModalOpen,
        protoEditFileModalOpen: state.protoEditFileModalOpen,
        protoEditMappingModalOpen: state.protoEditMappingModalOpen,
        editingProtoFile: state.editingProtoFile,
        editingProtoMapping: state.editingProtoMapping,
        setProtoFiles: state.setProtoFiles,
        setProtoMappings: state.setProtoMappings,
        setProtoMessageTypes: state.setProtoMessageTypes,
        openProtoListModal: state.openProtoListModal,
        closeProtoListModal: state.closeProtoListModal,
        openProtoEditFileModal: state.openProtoEditFileModal,
        closeProtoEditFileModal: state.closeProtoEditFileModal,
        openProtoEditMappingModal: state.openProtoEditMappingModal,
        closeProtoEditMappingModal: state.closeProtoEditMappingModal,
        protoImportLoading: state.protoImportLoading,
        setProtoImportLoading: state.setProtoImportLoading,
        protoImportURLModalOpen: state.protoImportURLModalOpen,
        protoImportURL: state.protoImportURL,
        openProtoImportURLModal: state.openProtoImportURLModal,
        closeProtoImportURLModal: state.closeProtoImportURLModal,
        setProtoImportURL: state.setProtoImportURL,
    })));

    // Breakpoint group
    const {
        breakpointRules,
        pendingBreakpoints,
        breakpointListModalOpen,
        breakpointEditModalOpen,
        editingBreakpointRule,
        breakpointResolveModalOpen,
        selectedBreakpoint,
        breakpointEdit,
        setBreakpointRules,
        openBreakpointListModal,
        closeBreakpointListModal,
        openBreakpointEditModal,
        closeBreakpointEditModal,
        addPendingBreakpoint,
        removePendingBreakpoint,
        clearPendingBreakpoints,
        openBreakpointResolveModal,
        closeBreakpointResolveModal,
        updateBreakpointEdit,
        pendingBreakpointData,
        setPendingBreakpointData,
    } = useProxyStore(useShallow((state) => ({
        breakpointRules: state.breakpointRules,
        pendingBreakpoints: state.pendingBreakpoints,
        breakpointListModalOpen: state.breakpointListModalOpen,
        breakpointEditModalOpen: state.breakpointEditModalOpen,
        editingBreakpointRule: state.editingBreakpointRule,
        breakpointResolveModalOpen: state.breakpointResolveModalOpen,
        selectedBreakpoint: state.selectedBreakpoint,
        breakpointEdit: state.breakpointEdit,
        setBreakpointRules: state.setBreakpointRules,
        openBreakpointListModal: state.openBreakpointListModal,
        closeBreakpointListModal: state.closeBreakpointListModal,
        openBreakpointEditModal: state.openBreakpointEditModal,
        closeBreakpointEditModal: state.closeBreakpointEditModal,
        addPendingBreakpoint: state.addPendingBreakpoint,
        removePendingBreakpoint: state.removePendingBreakpoint,
        clearPendingBreakpoints: state.clearPendingBreakpoints,
        openBreakpointResolveModal: state.openBreakpointResolveModal,
        closeBreakpointResolveModal: state.closeBreakpointResolveModal,
        updateBreakpointEdit: state.updateBreakpointEdit,
        pendingBreakpointData: state.pendingBreakpointData,
        setPendingBreakpointData: state.setPendingBreakpointData,
    })));

    // Map Remote / Rewrite / Diff / WebSocket group
    const {
        mapRemoteRules,
        mapRemoteListModalOpen,
        mapRemoteEditModalOpen,
        editingMapRemoteRule,
        setMapRemoteRules,
        openMapRemoteListModal,
        closeMapRemoteListModal,
        openMapRemoteEditModal,
        closeMapRemoteEditModal,
        rewriteRules,
        rewriteListModalOpen,
        rewriteEditModalOpen,
        editingRewriteRule,
        setRewriteRules,
        openRewriteListModal,
        closeRewriteListModal,
        openRewriteEditModal,
        closeRewriteEditModal,
        diffBaseLog,
        diffModalOpen,
        diffTargetLog,
        setDiffBaseLog,
        openDiffModal,
        closeDiffModal,
        wsMessages,
        addWSMessage,
        clearWSMessages,
    } = useProxyStore(useShallow((state) => ({
        mapRemoteRules: state.mapRemoteRules,
        mapRemoteListModalOpen: state.mapRemoteListModalOpen,
        mapRemoteEditModalOpen: state.mapRemoteEditModalOpen,
        editingMapRemoteRule: state.editingMapRemoteRule,
        setMapRemoteRules: state.setMapRemoteRules,
        openMapRemoteListModal: state.openMapRemoteListModal,
        closeMapRemoteListModal: state.closeMapRemoteListModal,
        openMapRemoteEditModal: state.openMapRemoteEditModal,
        closeMapRemoteEditModal: state.closeMapRemoteEditModal,
        rewriteRules: state.rewriteRules,
        rewriteListModalOpen: state.rewriteListModalOpen,
        rewriteEditModalOpen: state.rewriteEditModalOpen,
        editingRewriteRule: state.editingRewriteRule,
        setRewriteRules: state.setRewriteRules,
        openRewriteListModal: state.openRewriteListModal,
        closeRewriteListModal: state.closeRewriteListModal,
        openRewriteEditModal: state.openRewriteEditModal,
        closeRewriteEditModal: state.closeRewriteEditModal,
        diffBaseLog: state.diffBaseLog,
        diffModalOpen: state.diffModalOpen,
        diffTargetLog: state.diffTargetLog,
        setDiffBaseLog: state.setDiffBaseLog,
        openDiffModal: state.openDiffModal,
        closeDiffModal: state.closeDiffModal,
        wsMessages: state.wsMessages,
        addWSMessage: state.addWSMessage,
        clearWSMessages: state.clearWSMessages,
    })));

    // Watch hidden _conditionHints field stored in the form (survives HMR / store resets)
    const formStoredHints: string | undefined = Form.useWatch('_conditionHints', mockForm);

    // Compute condition key presets.
    // Priority: 1) store hints (set when creating mock from a request)
    //           2) form-persisted hints (survives HMR)
    //           3) scan captured logs
    const conditionKeyPresets = useMemo(() => {
        // Priority 1: store-level hints (freshly set from createMockFromRequest)
        if (mockConditionHints && mockConditionHints.headers.length > 0) {
            return mockConditionHints;
        }
        // Priority 2: form-persisted hints (survives store resets / HMR)
        if (formStoredHints) {
            try {
                const parsed = JSON.parse(formStoredHints);
                if (parsed && parsed.headers && parsed.headers.length > 0) {
                    return parsed as { headers: Array<{ key: string; value: string }>; queryParams: Array<{ key: string; value: string }> };
                }
            } catch (_) { /* ignore */ }
        }
        // Priority 3: derive from all captured logs
        const headerMap = new Map<string, string>();
        const queryMap = new Map<string, string>();
        const cap = Math.min(logs.length, 200);
        for (let i = 0; i < cap; i++) {
            const log = logs[i];
            const hd = log.headers || (log as any).requestHeaders;
            if (hd) {
                for (const [k, v] of Object.entries(hd)) {
                    if (!headerMap.has(k)) {
                        headerMap.set(k, Array.isArray(v) ? (v as string[]).join(', ') : String(v || ''));
                    }
                }
            }
            try {
                const u = new URL(log.url);
                u.searchParams.forEach((val, key) => {
                    if (!queryMap.has(key)) queryMap.set(key, val);
                });
            } catch (_) { /* ignore */ }
        }
        const headers = Array.from(headerMap.entries())
            .map(([key, value]) => ({ key, value }))
            .sort((a, b) => a.key.localeCompare(b.key));
        const queryParams = Array.from(queryMap.entries())
            .map(([key, value]) => ({ key, value }))
            .sort((a, b) => a.key.localeCompare(b.key));
        return { headers, queryParams };
    }, [mockConditionHints, formStoredHints, logs.length]);

    // Aggregate captured URLs into wildcard patterns with per-method counts
    const capturedUrlData = useMemo(() => {
        if (logs.length === 0) return new Map<string, Map<string, number>>();
        const pathMap = new Map<string, Map<string, number>>();
        const cap = Math.min(logs.length, 500);
        for (let i = 0; i < cap; i++) {
            const log = logs[i];
            if (log.isWs) continue;
            let pathname = '';
            try {
                const urlObj = new URL(log.url);
                pathname = urlObj.pathname;
            } catch {
                pathname = log.url.split('?')[0];
            }
            if (!pathname || pathname === '/') continue;
            // Normalize: collapse numeric path segments to * (e.g. /api/users/123 -> /api/users/*)
            const normalized = pathname.replace(/\/\d+/g, '/*');
            const pattern = `*${normalized}*`;
            if (!pathMap.has(pattern)) {
                pathMap.set(pattern, new Map());
            }
            const methods = pathMap.get(pattern)!;
            methods.set(log.method, (methods.get(log.method) || 0) + 1);
        }
        return pathMap;
    }, [logs.length]);

    // Build autocomplete options from captured URL data, optionally filtered by HTTP method
    const getUrlPatternsForMethod = useCallback((method?: string): { value: string; label: ReactNode }[] => {
        const entries = Array.from(capturedUrlData.entries());
        const filtered = method
            ? entries.filter(([, methods]) => methods.has(method))
            : entries;
        return filtered
            .sort((a, b) => {
                const countA = method ? (a[1].get(method) || 0) : Array.from(a[1].values()).reduce((s, c) => s + c, 0);
                const countB = method ? (b[1].get(method) || 0) : Array.from(b[1].values()).reduce((s, c) => s + c, 0);
                return countB - countA;
            })
            .map(([pattern, methods]) => {
                const count = method ? (methods.get(method) || 0) : Array.from(methods.values()).reduce((s, c) => s + c, 0);
                const methodsLabel = method || Array.from(methods.keys()).join(',');
                return {
                    value: pattern,
                    label: (
                        <span style={{ display: 'flex', justifyContent: 'space-between', fontFamily: 'monospace', fontSize: 12 }}>
                            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>{pattern}</span>
                            <span style={{ color: 'rgba(255,255,255,0.35)', marginLeft: 8, flexShrink: 0 }}>{methodsLabel} ×{count}</span>
                        </span>
                    ),
                };
            });
    }, [capturedUrlData]);

    // Check cert trust status when MITM is enabled and proxy is running
    useEffect(() => {
        if (mitmEnabled && isRunning && selectedDevice) {
            setCertTrustStatus('checking');
            CheckCertTrust(selectedDevice).then((status: string) => {
                setCertTrustStatus(status);
            }).catch(() => {
                setCertTrustStatus('unknown');
            });
        } else {
            setCertTrustStatus(null);
        }
    }, [mitmEnabled, isRunning, selectedDevice]);

    // Re-check cert status periodically when pending
    useEffect(() => {
        if (certTrustStatus === 'pending' && selectedDevice) {
            const interval = setInterval(() => {
                CheckCertTrust(selectedDevice).then((status: string) => {
                    if (status !== 'pending') {
                        setCertTrustStatus(status);
                    }
                }).catch(() => {});
            }, 2000); // Check every 2 seconds
            return () => clearInterval(interval);
        }
    }, [certTrustStatus, selectedDevice]);

    // Consume pendingMockData from another view (e.g. EventTimeline mock button)
    useEffect(() => {
        if (pendingMockData) {
            setEditingMockRule(null);
            mockForm.resetFields();
            mockForm.setFieldsValue({
                urlPattern: pendingMockData.urlPattern,
                method: pendingMockData.method,
                statusCode: pendingMockData.statusCode,
                contentType: pendingMockData.contentType || 'application/json',
                body: pendingMockData.body || '',
                delay: 0,
                description: pendingMockData.description || '',
            });
            setMockEditModalOpen(true);
            setPendingMockData(null);
        }
    }, [pendingMockData]);

    // Consume pendingBreakpointData from another view (e.g. EventTimeline BP button)
    useEffect(() => {
        if (pendingBreakpointData) {
            openBreakpointEditModal(null);
            breakpointForm.resetFields();
            breakpointForm.setFieldsValue({
                urlPattern: pendingBreakpointData.urlPattern,
                method: pendingBreakpointData.method,
                phase: pendingBreakpointData.phase || 'both',
                description: pendingBreakpointData.description || '',
            });
            setPendingBreakpointData(null);
        }
    }, [pendingBreakpointData]);

    useEffect(() => {
        // Listen for network stats
        EventsOn("network-stats", (stats: NetworkStats) => {
            setNetStats(stats);
        });

        return () => {
            EventsOff("network-stats");
        };
    }, []);

    // Auto-start network monitor when device is selected
    useEffect(() => {
        if (selectedDevice) {
            StartNetworkMonitor(selectedDevice);
        }
        return () => {
            if (selectedDevice) {
                StopNetworkMonitor(selectedDevice);
            }
        };
    }, [selectedDevice]);

    useEffect(() => {
        // Initial status check
        GetProxyStatus().then((status: boolean) => setProxyRunning(status));
        GetLocalIP().then((ip: string) => setLocalIP(ip));
        // Sync bypass patterns from backend (backend has defaults that frontend doesn't know about)
        GetMITMBypassPatterns().then((patterns: string[]) => {
            if (patterns && patterns.length > 0) {
                useProxyStore.getState().setBypassPatterns(patterns);
            }
        }).catch(() => {});

        // Listen for proxy status changes (e.g. started by session config)
        const handleProxyStatus = (data: { running: boolean; port: number }) => {
            setProxyRunning(data.running);
            if (data.port) {
                setPort(data.port);
            }
        };
        const offProxyStatus = EventsOn("proxy-status-changed", handleProxyStatus);

        // Listen for network events from session (unified event source)
        const handleSessionBatch = (events: any[]) => {
            const networkEvents = events.filter((e: any) => e.category === 'network');

            for (const event of networkEvents) {
                // UnifiedEvent uses 'data' field (JSON), parse it
                let detail: any = {};
                if (event.data) {
                    try {
                        detail = typeof event.data === 'string' ? JSON.parse(event.data) : event.data;
                    } catch (e) {
                        console.error('Failed to parse event data:', e);
                    }
                }
                const timeStr = event.timestamp ? new Date(event.timestamp).toLocaleTimeString() : '';

                // Handle websocket_message events specially
                if (event.type === 'websocket_message') {
                    const dirArrow = detail.direction === 'send' ? '↑' : '↓';
                    const log: RequestLog = {
                        id: event.id,
                        time: timeStr,
                        clientIp: '',
                        method: dirArrow,
                        url: `${detail.typeName || 'message'} (${detail.payloadSize || 0}B)`,
                        headers: {},
                        body: '',
                        isHttps: false,
                        isWs: true,
                        isWsMessage: true,
                        wsMessageInfo: {
                            connectionId: detail.connectionId || '',
                            direction: detail.direction || '',
                            typeName: detail.typeName || 'unknown',
                            payload: detail.payload || '',
                            payloadSize: detail.payloadSize || 0,
                            isBinary: detail.isBinary || false,
                            isProtobuf: detail.isProtobuf || false,
                        },
                        bodySize: detail.payloadSize || 0,
                        isProtobuf: detail.isProtobuf || false,
                        timestamp: event.timestamp,
                    };
                    addLog(log);
                    continue;
                }

                // Convert regular HTTP/WS connection events to RequestLog format
                const log: RequestLog = {
                    id: detail.id || event.id,
                    time: timeStr,
                    clientIp: detail.clientIp || '',
                    method: detail.method || 'UNKNOWN',
                    url: detail.url || event.title,
                    headers: detail.requestHeaders || {},
                    body: detail.requestBody || '',
                    isHttps: detail.isHttps || false,
                    statusCode: detail.statusCode,
                    contentType: detail.contentType,
                    bodySize: detail.bodySize,
                    previewBody: detail.requestBody,
                    respHeaders: detail.responseHeaders,
                    respBody: detail.responseBody,
                    isWs: detail.isWs || false,
                    mocked: detail.mocked || false,
                    isProtobuf: detail.isProtobuf || false,
                    isReqProtobuf: detail.isReqProtobuf || false,
                    duration: detail.duration,
                    timestamp: event.timestamp,
                };

                // Use getState() to get current logs (avoid stale closure)
                const currentLogs = useProxyStore.getState().logs;
                const existingLog = currentLogs.find(l => l.id === log.id);
                if (existingLog) {
                    updateLog(log.id, log);
                } else {
                    addLog(log);
                }
            }
        };

        // 共享通道必须用 EventsOn 返回的 cancel 函数精确退订，
        // EventsOff(name) 会误删 eventStore/logcatStore/perfStore 的监听器
        const offBatch = EventsOn("session-events-batch", handleSessionBatch);

        // Listen for WebSocket frame messages
        const handleWSMessage = (msg: any) => {
            useProxyStore.getState().addWSMessage(msg as WSMessage);
        };
        const offWSMessage = EventsOn("proxy-ws-message", handleWSMessage);

        return () => {
            offBatch?.();
            offProxyStatus?.();
            offWSMessage?.();
        };
    }, []);

    useEffect(() => {
        // Auto-scroll logic could be added here if needed, but Table manages its own scroll usually.
        // If we want to stick to bottom, we could scroll the table body container.
        // For now, let's just let it be or use a "scroll to bottom" generic approach if user wants.
    }, [logs]);

    const handleStart = async () => {
        try {
            // 1. Start Server (binds to localhost only for security)
            await SetProxyMITM(mitmEnabled);
            await SetProxyWSEnabled(wsEnabled);
            await StartProxy(port);
            setProxyRunning(true);

            // 2. Set proxy device for event association
            if (selectedDevice) {
                await SetProxyDevice(selectedDevice);
            }

            // 3. Setup adb reverse + device proxy if device selected
            if (selectedDevice) {
                try {
                    await SetupProxyForDevice(selectedDevice, port);
                    message.success(t('proxy.start_success', { ip: '127.0.0.1', port }));
                } catch (adbErr: any) {
                    const errorStr = String(adbErr);
                    if (errorStr.includes("WRITE_SECURE_SETTINGS")) {
                        message.warning(t('proxy.link_failed', { error: "Security Settings blocked" }));
                        showSecurityError();
                    } else {
                        message.error(t('proxy.link_failed', { error: errorStr }));
                    }
                }
            } else {
                message.success(t('proxy.start_local_success'));
            }
        } catch (err) {
            message.error(t('app.command_failed') + ": " + String(err));
        }
    };

    const handleStop = async () => {
        try {
            // 1. Cleanup adb reverse and device proxy
            if (selectedDevice) {
                try {
                    await CleanupProxyForDevice(selectedDevice, port);
                } catch (e) { }
            }

            // 2. Stop Server
            await StopProxy();
            setProxyRunning(false);
            message.success(t('proxy.stop_success'));
        } catch (err) {
            message.error(t('app.command_failed') + ": " + String(err));
        }
    };

    const showSecurityError = () => {
        modal.error({
            title: t('proxy.permission_denied'),
            content: (
                <div>
                    <p>{t('proxy.permission_denied_desc')}</p>
                    <p><strong>{t('proxy.xiaomi_users')}</strong></p>
                    <p>{t('proxy.xiaomi_desc')}</p>
                    <p style={{ fontSize: '12px', color: '#999' }}>{t('proxy.xiaomi_note')}</p>
                    <br />
                    <p><strong>{t('proxy.other_devices')}</strong></p>
                    <p>{t('proxy.other_desc')}</p>
                </div>
            ),
        });
    };

    const handleWSToggle = async (checked: boolean) => {
        try {
            await SetProxyWSEnabled(checked);
            toggleWS();
            message.success(checked ? t('proxy.on') : t('proxy.off'));
        } catch (err) {
            message.error(t('app.command_failed') + ": " + String(err));
        }
    };

    const handleMITMToggle = async (checked: boolean) => {
        try {
            await SetProxyMITM(checked);
            toggleMITM();
            if (checked) {
                message.info(t('proxy.mitm_tooltip'));
            } else {
                message.success(t('proxy.off'));
            }
        } catch (err) {
            message.error(t('app.command_failed') + ": " + String(err));
        }
    };

    const handleInstallCert = async () => {
        if (!selectedDevice) return;
        try {
            const path = await InstallProxyCert(selectedDevice);
            modal.success({
                title: t('proxy.cert_pushed'),
                content: (
                    <div>
                        <p>{t('proxy.cert_pushed_desc', { path })}</p>
                        <Divider style={{ margin: '12px 0' }} />
                        <p><strong>{t('proxy.next_steps')}</strong></p>
                        <ol style={{ paddingLeft: 20 }}>
                            <li>{t('proxy.step_1')}</li>
                            <li>{t('proxy.step_2')}</li>
                            <li>{t('proxy.step_3')}</li>
                            <li>{t('proxy.step_4')}</li>
                        </ol>
                        <p style={{ fontSize: '12px', color: '#ff4d4f', marginTop: 8 }}>
                            {t('proxy.cert_warning')}
                        </p>
                    </div>
                )
            });
        } catch (err) {
            message.error(t('app.command_failed') + ": " + String(err));
        }
    };

    const handleApplyRules = async () => {
        try {
            // Apply Speed Limits
            const dlBytes = (dlLimit || 0) * 1024;
            const ulBytes = (ulLimit || 0) * 1024;
            await SetProxyLimit(ulBytes, dlBytes);

            // Apply Latency
            const ms = latency || 0;
            await SetProxyLatency(ms);

            message.success(t('proxy.apply'));
        } catch (err) {
            message.error(t('app.command_failed') + ": " + String(err));
        }
    };

    const handleClearRules = async () => {
        try {
            await SetProxyLimit(0, 0);
            await SetProxyLatency(0);
            setSpeedLimits(null, null);
            setLatency(null);
            message.success(t('proxy.off'));
        } catch (err) {
            message.error(t('app.command_failed') + ": " + String(err));
        }
    };

    const formatSpeed = (speed: number) => {
        if (!speed || speed === 0) return '0 B/s';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(speed) / Math.log(k));
        return parseFloat((speed / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i] + '/s';
    };

    const formatBytes = (bytes: number) => {
        if (!bytes || bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    };

    const formatMs = (ms?: number) => {
        if (ms === undefined || ms === null) return '-';
        if (ms < 1) return '<1ms';
        if (ms < 1000) return Math.round(ms) + 'ms';
        if (ms < 10000) return (ms / 1000).toFixed(1) + 's';
        return Math.round(ms / 1000) + 's';
    };

    const formatBody = (body: string, contentType?: string) => {
        if (!body) return "";
        const trimmed = body.trim();

        // Check if it's JSON by content-type or by content
        const isJsonType = contentType?.includes('application/json');
        const looksLikeJson = (trimmed.startsWith('{') && trimmed.endsWith('}')) ||
                             (trimmed.startsWith('[') && trimmed.endsWith(']'));

        if (isJsonType || looksLikeJson) {
            try {
                const parsed = JSON.parse(trimmed);
                return JSON.stringify(parsed, null, 2);
            } catch (e) {
                // Not valid JSON
            }
        }
        return body;
    };

    // Generate code in selected language and copy to clipboard
    const handleCopyCode = (log: StoreRequestLog, lang: CodeLanguage) => {
        const code = generateCode(lang, {
            method: log.method,
            url: log.url,
            headers: log.headers,
            body: log.previewBody,
        });
        const langLabels: Record<CodeLanguage, string> = {
            curl: 'cURL',
            fetch: 'JavaScript',
            python: 'Python',
            go: 'Go',
        };
        navigator.clipboard.writeText(code).then(() => {
            message.success(t('proxy.copied_code', { lang: langLabels[lang] }));
        }).catch(() => {
            message.error(t('proxy.copy_failed'));
        });
    };

    // Open resend modal with pre-filled data
    const handleOpenResendModal = (log: StoreRequestLog) => {
        const headersStr = log.headers
            ? Object.entries(log.headers).map(([k, v]) => `${k}: ${v.join(', ')}`).join('\n')
            : '';
        resendForm.setFieldsValue({
            method: log.method,
            url: log.url,
            headers: headersStr,
            body: log.previewBody || '',
        });
        openResendModal('resend');
    };

    // Ctrl/Cmd+Enter keyboard shortcut to send request
    useEffect(() => {
        if (!resendModalOpen) return;
        const handler = (e: KeyboardEvent) => {
            if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                e.preventDefault();
                handleResend();
            }
        };
        window.addEventListener('keydown', handler);
        return () => window.removeEventListener('keydown', handler);
    }, [resendModalOpen]); // eslint-disable-line react-hooks/exhaustive-deps

    // Handle resend request
    const handleResend = async () => {
        try {
            const values = await resendForm.validateFields();
            setResendLoading(true);

            // Parse headers string to map
            const headersMap: Record<string, string> = {};
            if (values.headers) {
                values.headers.split('\n').forEach((line: string) => {
                    const idx = line.indexOf(':');
                    if (idx > 0) {
                        const key = line.substring(0, idx).trim();
                        const val = line.substring(idx + 1).trim();
                        if (key) headersMap[key] = val;
                    }
                });
            }

            const response = await ResendRequest(values.method, values.url, headersMap, values.body || '');
            setResendResponse(response);
            message.success(t('proxy.resend_success'));
        } catch (err: any) {
            message.error(t('proxy.resend_failed') + ': ' + String(err));
        } finally {
            setResendLoading(false);
        }
    };

    // Load mock rules
    const loadMockRules = async () => {
        try {
            const rules = await GetMockRules();
            setMockRules((rules || []) as any);
        } catch (err) {
            console.error('Failed to load mock rules:', err);
        }
    };

    // Open mock list modal
    const handleOpenMockListModal = () => {
        loadMockRules();
        openMockListModal();
    };

    // Add or update mock rule
    const handleSaveMockRule = async () => {
        try {
            const values = await mockForm.validateFields();
            // Filter out incomplete conditions (must have at least type and operator)
            const conditions = (values.conditions || []).filter(
                (c: any) => c && c.type && c.operator
            );
            const rule = {
                id: editingMockRule?.id || '',
                urlPattern: values.urlPattern,
                method: values.method || '',
                statusCode: values.statusCode || 200,
                headers: { 'Content-Type': values.contentType || 'application/json' },
                body: values.body || '',
                bodyFile: values.bodyFile || '',
                delay: values.delay || 0,
                description: values.description || '',
                enabled: true,
                createdAt: editingMockRule?.createdAt || 0,
                conditions,
            };

            if (editingMockRule) {
                // Update existing rule (preserves original ID and createdAt)
                await UpdateMockRule({ ...rule, id: editingMockRule.id, createdAt: editingMockRule.createdAt || 0, enabled: editingMockRule.enabled } as any);
            } else {
                await AddMockRule(rule as any);
            }

            message.success(editingMockRule ? t('proxy.mock_rule_updated') : t('proxy.mock_rule_added'));
            mockForm.resetFields();
            closeMockEditModal();
            loadMockRules();
        } catch (err: any) {
            message.error(String(err));
        }
    };

    // Delete mock rule
    const handleDeleteMockRule = async (id: string) => {
        try {
            await RemoveMockRule(id);
            message.success(t('proxy.mock_rule_deleted'));
            loadMockRules();
        } catch (err) {
            message.error(String(err));
        }
    };

    // Toggle mock rule
    const handleToggleMockRule = async (id: string, enabled: boolean) => {
        try {
            await ToggleMockRule(id, enabled);
            loadMockRules();
        } catch (err) {
            message.error(String(err));
        }
    };

    const handleExportMockRules = async () => {
        try {
            const jsonStr = await ExportMockRules();
            const blob = new Blob([jsonStr], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'mock-rules.json';
            a.click();
            URL.revokeObjectURL(url);
            message.success(t('proxy.mock_exported'));
        } catch (err) {
            message.error(String(err));
        }
    };

    const handleImportMockRules = async () => {
        const input = document.createElement('input');
        input.type = 'file';
        input.accept = '.json';
        input.onchange = async (e: any) => {
            const file = e.target.files?.[0];
            if (!file) return;
            try {
                const text = await file.text();
                const count = await ImportMockRules(text);
                message.success(t('proxy.mock_imported', { count }));
                loadMockRules();
            } catch (err) {
                message.error(String(err));
            }
        };
        input.click();
    };

    // ========================================
    // Map Remote Rules
    // ========================================

    const loadMapRemoteRules = async () => {
        try {
            const rules = await GetMapRemoteRules();
            setMapRemoteRules((rules || []) as any);
        } catch (err) {
            console.error('Failed to load map remote rules:', err);
        }
    };

    const handleOpenMapRemoteListModal = () => {
        loadMapRemoteRules();
        openMapRemoteListModal();
    };

    const handleSaveMapRemoteRule = async () => {
        try {
            const values = await mapRemoteForm.validateFields();
            if (editingMapRemoteRule) {
                await UpdateMapRemoteRule(
                    editingMapRemoteRule.id,
                    values.sourcePattern,
                    values.targetURL,
                    values.method || '',
                    editingMapRemoteRule.enabled,
                    values.description || ''
                );
            } else {
                await AddMapRemoteRule(
                    values.sourcePattern,
                    values.targetURL,
                    values.method || '',
                    values.description || ''
                );
            }
            message.success(editingMapRemoteRule ? t('proxy.map_remote_updated') : t('proxy.map_remote_added'));
            mapRemoteForm.resetFields();
            closeMapRemoteEditModal();
            loadMapRemoteRules();
        } catch (err: any) {
            message.error(String(err));
        }
    };

    const handleDeleteMapRemoteRule = async (id: string) => {
        try {
            await RemoveMapRemoteRule(id);
            message.success(t('proxy.map_remote_deleted'));
            loadMapRemoteRules();
        } catch (err) {
            message.error(String(err));
        }
    };

    const handleToggleMapRemoteRule = async (id: string, enabled: boolean) => {
        try {
            await ToggleMapRemoteRule(id, enabled);
            loadMapRemoteRules();
        } catch (err) {
            message.error(String(err));
        }
    };

    // ========================================
    // Rewrite Rules
    // ========================================

    const loadRewriteRules = async () => {
        try {
            const rules = await GetRewriteRules();
            setRewriteRules((rules || []) as any);
        } catch (err) {
            console.error('Failed to load rewrite rules:', err);
        }
    };

    const handleOpenRewriteListModal = () => {
        loadRewriteRules();
        openRewriteListModal();
    };

    const handleSaveRewriteRule = async () => {
        try {
            const values = await rewriteForm.validateFields();
            if (editingRewriteRule) {
                await UpdateRewriteRule(
                    editingRewriteRule.id,
                    values.urlPattern,
                    values.method || '',
                    values.phase,
                    values.target,
                    values.headerName || '',
                    values.match,
                    values.replace || '',
                    editingRewriteRule.enabled,
                    values.description || ''
                );
            } else {
                await AddRewriteRule(
                    values.urlPattern,
                    values.method || '',
                    values.phase,
                    values.target,
                    values.headerName || '',
                    values.match,
                    values.replace || '',
                    values.description || ''
                );
            }
            message.success(editingRewriteRule ? t('proxy.rewrite_rule_updated') : t('proxy.rewrite_rule_added'));
            rewriteForm.resetFields();
            closeRewriteEditModal();
            loadRewriteRules();
        } catch (err: any) {
            message.error(String(err));
        }
    };

    const handleDeleteRewriteRule = async (id: string) => {
        try {
            await RemoveRewriteRule(id);
            message.success(t('proxy.rewrite_rule_deleted'));
            loadRewriteRules();
        } catch (err) {
            message.error(String(err));
        }
    };

    const handleToggleRewriteRule = async (id: string, enabled: boolean) => {
        try {
            await ToggleRewriteRule(id, enabled);
            loadRewriteRules();
        } catch (err) {
            message.error(String(err));
        }
    };

    const startEditRewriteRule = (rule: any) => {
        closeRewriteListModal();
        openRewriteEditModal(rule);
        rewriteForm.resetFields();
        rewriteForm.setFieldsValue({
            urlPattern: rule.urlPattern,
            method: rule.method,
            phase: rule.phase,
            target: rule.target,
            headerName: rule.headerName,
            match: rule.match,
            replace: rule.replace,
            description: rule.description,
        });
    };

    // Diff: mark current request as base for comparison
    const handleMarkForDiff = useCallback((log: StoreRequestLog) => {
        if (diffBaseLog && diffBaseLog.id !== log.id) {
            // We already have a base, this becomes the target — open diff modal
            openDiffModal(diffBaseLog, log);
        } else {
            // Mark as base
            setDiffBaseLog(log);
            message.info(t('proxy.diff_base_marked'));
        }
    }, [diffBaseLog, openDiffModal, setDiffBaseLog, message, t]);

    // Proxy list click: when diff mode is active, clicking a different request opens diff modal directly
    const handleProxyListClick = useCallback((log: StoreRequestLog, _index: number) => {
        if (diffBaseLog && diffBaseLog.id !== log.id && !log.isWsMessage) {
            // Diff mode active: open diff modal directly with this as target
            openDiffModal(diffBaseLog, log);
        } else {
            // Normal selection (also clears diff base if clicking a WS message or same request)
            selectLog(log);
        }
    }, [diffBaseLog, openDiffModal, selectLog]);

    // Cancel diff mode on Escape key
    useEffect(() => {
        if (!diffBaseLog) return;
        const handleEsc = (e: KeyboardEvent) => {
            if (e.key === 'Escape' && diffBaseLog && !diffModalOpen) {
                setDiffBaseLog(null);
            }
        };
        window.addEventListener('keydown', handleEsc);
        return () => window.removeEventListener('keydown', handleEsc);
    }, [diffBaseLog, diffModalOpen, setDiffBaseLog]);

    // Export captured requests as HAR 1.2 format
    const handleExportHAR = () => {
        const currentLogs = useProxyStore.getState().logs;
        if (currentLogs.length === 0) {
            message.warning(t('proxy.no_logs_to_export'));
            return;
        }

        const entries = currentLogs.filter(log => log.method !== 'CONNECT' && !log.isWs).map(log => {
            const reqHeaders = log.headers
                ? Object.entries(log.headers).flatMap(([name, values]) =>
                    (values || []).map(value => ({ name, value }))
                )
                : [];

            const resHeaders = log.respHeaders
                ? Object.entries(log.respHeaders).flatMap(([name, values]) =>
                    (values || []).map(value => ({ name, value }))
                )
                : [];

            // Parse URL for query string
            let queryString: { name: string; value: string }[] = [];
            try {
                const u = new URL(log.url);
                u.searchParams.forEach((value, name) => queryString.push({ name, value }));
            } catch { /* ignore */ }

            const reqBodySize = log.body ? new Blob([log.body]).size : 0;
            const reqMimeType = reqHeaders.find(h => h.name.toLowerCase() === 'content-type')?.value || '';
            const resMimeType = log.contentType || '';

            return {
                startedDateTime: log.timestamp ? new Date(log.timestamp).toISOString() : new Date().toISOString(),
                time: log.duration || 0,
                request: {
                    method: log.method,
                    url: log.url,
                    httpVersion: 'HTTP/1.1',
                    cookies: [],
                    headers: reqHeaders,
                    queryString,
                    postData: log.body ? { mimeType: reqMimeType, text: log.body } : undefined,
                    headersSize: -1,
                    bodySize: reqBodySize,
                },
                response: {
                    status: log.statusCode || 0,
                    statusText: '',
                    httpVersion: 'HTTP/1.1',
                    cookies: [],
                    headers: resHeaders,
                    content: {
                        size: log.bodySize || 0,
                        mimeType: resMimeType,
                        text: log.respBody || '',
                    },
                    redirectURL: '',
                    headersSize: -1,
                    bodySize: log.bodySize || -1,
                },
                cache: {},
                timings: {
                    send: 0,
                    wait: log.duration || 0,
                    receive: 0,
                },
            };
        });

        const har = {
            log: {
                version: '1.2',
                creator: { name: 'Gaze', version: '1.0' },
                entries,
            },
        };

        const blob = new Blob([JSON.stringify(har, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `gaze-capture-${new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-')}.har`;
        a.click();
        URL.revokeObjectURL(url);
        message.success(t('proxy.har_exported', { count: entries.length }));
    };

    // Edit mock rule — close list modal first, then open edit modal with prefilled form
    const startEditMockRule = (rule: any) => {
        closeMockListModal();
        setMockConditionHints(null);
        openMockEditModal(rule);
        mockForm.resetFields();
        mockForm.setFieldsValue({
            urlPattern: rule.urlPattern,
            method: rule.method,
            statusCode: rule.statusCode,
            contentType: rule.headers?.['Content-Type'] || 'application/json',
            body: rule.body,
            bodyFile: rule.bodyFile || '',
            delay: rule.delay,
            description: rule.description,
            conditions: rule.conditions || [],
        });
    };

    // Create mock from request
    const createMockFromRequest = (log: StoreRequestLog) => {
        // Extract URL pattern (use path with wildcard for query params)
        let urlPattern = log.url;
        try {
            const urlObj = new URL(log.url);
            urlPattern = `*${urlObj.pathname}*`;
        } catch (e) {
            // Use full URL as pattern if parsing fails
            urlPattern = `*${log.url.split('?')[0]}*`;
        }

        // Extract condition hints from the captured request
        const hintHeaders: Array<{ key: string; value: string }> = [];
        const headerData = log.headers || (log as any).requestHeaders;
        if (headerData) {
            for (const [key, values] of Object.entries(headerData)) {
                const val = Array.isArray(values) ? (values as string[]).join(', ') : String(values || '');
                hintHeaders.push({ key, value: val });
            }
            hintHeaders.sort((a, b) => a.key.localeCompare(b.key));
        }

        const hintQueryParams: Array<{ key: string; value: string }> = [];
        try {
            const urlObj = new URL(log.url);
            urlObj.searchParams.forEach((value, key) => {
                hintQueryParams.push({ key, value });
            });
        } catch (e) {
            const qIdx = log.url.indexOf('?');
            if (qIdx >= 0) {
                try {
                    const params = new URLSearchParams(log.url.substring(qIdx + 1));
                    params.forEach((value, key) => {
                        hintQueryParams.push({ key, value });
                    });
                } catch (_) { /* ignore */ }
            }
        }

        const hints = { headers: hintHeaders, queryParams: hintQueryParams };
        setMockConditionHints(hints);

        openMockEditModal(null);
        mockForm.resetFields();
        mockForm.setFieldsValue({
            urlPattern,
            method: log.method,
            statusCode: log.statusCode || 200,
            contentType: log.contentType?.split(';')[0] || 'application/json',
            body: log.respBody || '',
            delay: 0,
            description: `Mock for ${log.method} ${urlPattern}`,
            _conditionHints: JSON.stringify(hints),
        });
    };

    // Create breakpoint rule from captured request
    const createBreakpointFromRequest = (log: StoreRequestLog) => {
        let urlPattern = log.url;
        try {
            const urlObj = new URL(log.url);
            urlPattern = `*${urlObj.pathname}*`;
        } catch (e) {
            urlPattern = `*${log.url.split('?')[0]}*`;
        }

        openBreakpointEditModal(null);
        breakpointForm.resetFields();
        breakpointForm.setFieldsValue({
            urlPattern,
            method: log.method,
            phase: 'both',
            description: `BP for ${log.method} ${urlPattern}`,
        });
    };

    // --- Proto management handlers ---
    const loadProtoData = async () => {
        try {
            const [files, mappings, types] = await Promise.all([
                GetProtoFiles(),
                GetProtoMappings(),
                GetProtoMessageTypes(),
            ]);
            setProtoFiles(files || []);
            setProtoMappings(mappings || []);
            setProtoMessageTypes(types || []);
        } catch (err) {
            console.error('Failed to load proto data:', err);
        }
    };

    const handleOpenProtoListModal = () => {
        loadProtoData();
        openProtoListModal();
    };

    const handleSaveProtoFile = async () => {
        try {
            const values = await protoFileForm.validateFields();
            if (editingProtoFile) {
                await UpdateProtoFile(editingProtoFile.id, values.name, values.content);
                message.success(t('proxy.proto_file_updated'));
            } else {
                await AddProtoFile(values.name, values.content);
                message.success(t('proxy.proto_file_added'));
            }
            protoFileForm.resetFields();
            closeProtoEditFileModal();
            loadProtoData();
        } catch (err: any) {
            message.error(String(err));
        }
    };

    const handleDeleteProtoFile = async (id: string) => {
        try {
            await RemoveProtoFile(id);
            message.success(t('proxy.proto_file_deleted'));
            loadProtoData();
        } catch (err) {
            message.error(String(err));
        }
    };

    const startEditProtoFile = (file: any) => {
        openProtoEditFileModal(file);
        protoFileForm.resetFields();
        protoFileForm.setFieldsValue({
            name: file.name,
            content: file.content,
        });
    };

    const handleImportProtoFromDisk = async () => {
        try {
            setProtoImportLoading(true);
            const ids = await LoadProtoFromDisk();
            if (ids && ids.length > 0) {
                message.success(t('proxy.proto_files_imported', { count: ids.length }));
                loadProtoData();
            }
        } catch (err: any) {
            message.error(String(err));
        } finally {
            setProtoImportLoading(false);
        }
    };

    const handleImportProtoFromURL = () => {
        openProtoImportURLModal();
    };

    const handleConfirmImportProtoFromURL = async () => {
        const url = protoImportURL;
        if (!url || url === 'https://') return;
        try {
            setProtoImportLoading(true);
            const ids = await LoadProtoFromURL(url);
            if (ids && ids.length > 0) {
                message.success(t('proxy.proto_files_imported', { count: ids.length }));
                loadProtoData();
            }
            closeProtoImportURLModal();
        } catch (err: any) {
            message.error(String(err));
        } finally {
            setProtoImportLoading(false);
        }
    };

    const handleSaveProtoMapping = async () => {
        try {
            const values = await protoMappingForm.validateFields();
            if (editingProtoMapping) {
                await UpdateProtoMapping(
                    editingProtoMapping.id,
                    values.urlPattern,
                    values.messageType,
                    values.direction || 'response',
                    values.description || ''
                );
                message.success(t('proxy.proto_mapping_updated'));
            } else {
                await AddProtoMapping(
                    values.urlPattern,
                    values.messageType,
                    values.direction || 'response',
                    values.description || ''
                );
                message.success(t('proxy.proto_mapping_added'));
            }
            protoMappingForm.resetFields();
            closeProtoEditMappingModal();
            loadProtoData();
        } catch (err: any) {
            message.error(String(err));
        }
    };

    const handleDeleteProtoMapping = async (id: string) => {
        try {
            await RemoveProtoMapping(id);
            message.success(t('proxy.proto_mapping_deleted'));
            loadProtoData();
        } catch (err) {
            message.error(String(err));
        }
    };

    const startEditProtoMapping = (mapping: any) => {
        openProtoEditMappingModal(mapping);
        protoMappingForm.resetFields();
        protoMappingForm.setFieldsValue({
            urlPattern: mapping.urlPattern,
            messageType: mapping.messageType,
            direction: mapping.direction,
            description: mapping.description,
        });
    };

    // --- Breakpoint handlers ---

    const loadBreakpointRules = async () => {
        try {
            const rules = await GetBreakpointRules();
            setBreakpointRules(rules || []);
        } catch (err) {
            console.error('Failed to load breakpoint rules:', err);
        }
    };

    const handleOpenBreakpointListModal = async () => {
        await loadBreakpointRules();
        openBreakpointListModal();
    };

    const handleSaveBreakpointRule = async () => {
        try {
            const values = await breakpointForm.validateFields();
            if (editingBreakpointRule) {
                await UpdateBreakpointRule({
                    ...editingBreakpointRule,
                    urlPattern: values.urlPattern,
                    method: values.method || '',
                    phase: values.phase,
                    description: values.description || '',
                    createdAt: editingBreakpointRule.createdAt || Date.now(),
                });
                message.success(t('proxy.breakpoint_rules') + ' updated');
            } else {
                await AddBreakpointRule({
                    id: '',
                    urlPattern: values.urlPattern,
                    method: values.method || '',
                    phase: values.phase,
                    enabled: true,
                    description: values.description || '',
                    createdAt: Date.now(),
                });
                message.success(t('proxy.breakpoint_rules') + ' added');
            }
            breakpointForm.resetFields();
            closeBreakpointEditModal();
            loadBreakpointRules();
        } catch (err: any) {
            if (err?.errorFields) return; // validation error
            message.error(String(err));
        }
    };

    const handleDeleteBreakpointRule = async (ruleId: string) => {
        try {
            await RemoveBreakpointRule(ruleId);
            loadBreakpointRules();
        } catch (err) {
            message.error(String(err));
        }
    };

    const handleToggleBreakpointRule = async (ruleId: string, enabled: boolean) => {
        try {
            await ToggleBreakpointRule(ruleId, enabled);
            loadBreakpointRules();
        } catch (err) {
            message.error(String(err));
        }
    };

    const startEditBreakpointRule = (rule: any) => {
        closeBreakpointListModal();
        openBreakpointEditModal(rule);
        breakpointForm.resetFields();
        breakpointForm.setFieldsValue({
            urlPattern: rule.urlPattern,
            method: rule.method,
            phase: rule.phase,
            description: rule.description,
        });
    };

    const handleResolveBreakpoint = async (bpId: string, action: string, modifications?: Record<string, any>) => {
        try {
            await ResolveBreakpoint(bpId, action, modifications || {});
            removePendingBreakpoint(bpId);
            closeBreakpointResolveModal();
        } catch (err) {
            // If breakpoint not found (e.g. already timed out), clean up the stale entry
            removePendingBreakpoint(bpId);
            closeBreakpointResolveModal();
            message.warning(String(err));
        }
    };

    const handleForwardAllBreakpoints = async () => {
        try {
            await ForwardAllBreakpoints();
            clearPendingBreakpoints();
        } catch (err) {
            message.error(String(err));
        }
    };

    // Listen for breakpoint-hit events
    useEffect(() => {
        const onHit = (info: any) => {
            addPendingBreakpoint(info);
        };
        EventsOn('breakpoint-hit', onHit);
        return () => { EventsOff('breakpoint-hit'); };
    }, [addPendingBreakpoint]);

    // Listen for breakpoint-resolved events (timeout or backend cleanup)
    useEffect(() => {
        const onResolved = (data: any) => {
            if (data?.id) {
                removePendingBreakpoint(data.id);
            }
        };
        EventsOn('breakpoint-resolved', onResolved);
        return () => { EventsOff('breakpoint-resolved'); };
    }, [removePendingBreakpoint]);

    // Listen for breakpoint pool capacity warnings (matched request let through)
    useEffect(() => {
        const onCapacity = () => {
            // Fixed key so repeated warnings replace instead of stacking
            message.warning({ content: t('proxy.breakpoint_pool_full'), key: 'bp-capacity' });
        };
        const offCapacity = EventsOn('proxy-breakpoint-capacity', onCapacity);
        return () => { offCapacity?.(); };
    }, [t]);

    // Load breakpoint rules on mount
    useEffect(() => {
        loadBreakpointRules();
    }, []);

    // Memoized: deep search across 5000 logs is expensive, so it must not rerun on
    // unrelated re-renders (netStats ticks, modal toggles). Uses the debounced search text.
    const filteredLogs = useMemo(() => logs.filter(log => {
        // Filter by type (ALL, HTTP, WS)
        if (filterType === "HTTP" && log.isWs) return false;
        if (filterType === "WS" && !log.isWs) return false;

        // Filter by search text (deep search: URL, method, status, headers, body, response)
        if (debouncedSearchText) {
            const lowerSearch = debouncedSearchText.toLowerCase();

            // Basic fields
            if (log.url.toLowerCase().includes(lowerSearch)) return true;
            if (log.method.toLowerCase().includes(lowerSearch)) return true;
            if (String(log.statusCode || '').includes(lowerSearch)) return true;
            if ((log.contentType || '').toLowerCase().includes(lowerSearch)) return true;
            if ((log.clientIp || '').toLowerCase().includes(lowerSearch)) return true;

            // Request headers (key + values)
            if (log.headers) {
                for (const [key, values] of Object.entries(log.headers)) {
                    if (key.toLowerCase().includes(lowerSearch)) return true;
                    if (values?.some(v => v.toLowerCase().includes(lowerSearch))) return true;
                }
            }

            // Response headers (key + values)
            if (log.respHeaders) {
                for (const [key, values] of Object.entries(log.respHeaders)) {
                    if (key.toLowerCase().includes(lowerSearch)) return true;
                    if (values?.some(v => v.toLowerCase().includes(lowerSearch))) return true;
                }
            }

            // Request body
            if ((log.previewBody || '').toLowerCase().includes(lowerSearch)) return true;

            // Response body
            if ((log.respBody || '').toLowerCase().includes(lowerSearch)) return true;

            return false;
        }
        return true;
    }), [logs, filterType, debouncedSearchText]);

    return (
        <div style={{ padding: '16px', height: '100%', display: 'flex', flexDirection: 'column', gap: '12px', overflow: 'hidden' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexShrink: 0 }}>
                <Space align="center" size="small">
                    <GlobalOutlined style={{ fontSize: '18px' }} />
                    <Title level={4} style={{ margin: 0 }}>{t('proxy.title')}</Title>
                </Space>
                <DeviceSelector />
            </div>

            <Card size="small" styles={{ body: { padding: '12px' } }}>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
                    {/* Row 1: Proxy & Device Settings */}
                    <div style={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: '8px 0' }}>
                        <Space split={<Divider type="vertical" />} style={{ flexShrink: 0 }}>
                            <Space size="small">
                                <Text strong>{t('proxy.server')}:</Text>
                                <Tag color={isRunning ? "success" : "default"} style={{ marginRight: 0 }}>{isRunning ? t('proxy.on') : t('proxy.off')}</Tag>
                            </Space>
                            <Space size="small">
                                <Text type="secondary">{t('proxy.ip')}:</Text>
                                <Text copyable style={{ fontSize: '13px' }}>{localIP || "Unknown"}</Text>
                            </Space>
                            <Space size="small">
                                <Text type="secondary">{t('proxy.port')}:</Text>
                                <InputNumber size="small" value={port} onChange={(v) => setPort(v || 8080)} disabled={isRunning} style={{ width: 60 }} />
                            </Space>
                            <Space size="small">
                                <Tooltip title={t('proxy.ws_tooltip')}>
                                    <Space size={4}>
                                        <ApiOutlined style={{ color: wsEnabled ? '#1890ff' : undefined }} />
                                        <Switch size="small" checked={wsEnabled} onChange={handleWSToggle} />
                                    </Space>
                                </Tooltip>
                            </Space>
                            <Space size="small">
                                <Tooltip title={t('proxy.mitm_tooltip')}>
                                    <Space size={4}>
                                        <SafetyCertificateOutlined style={{ color: mitmEnabled ? '#faad14' : undefined }} />
                                        <Switch size="small" checked={mitmEnabled} onChange={handleMITMToggle} />
                                    </Space>
                                </Tooltip>
                            </Space>
                            {mitmEnabled && (
                                <Space size="small">
                                    <Tooltip title={
                                        certTrustStatus === 'trusted' ? t('proxy.cert_trusted') :
                                        certTrustStatus === 'not_trusted' ? t('proxy.cert_not_trusted') :
                                        certTrustStatus === 'pending' ? t('proxy.cert_pending') :
                                        certTrustStatus === 'checking' ? t('proxy.cert_checking') :
                                        t('proxy.cert_install_hint')
                                    }>
                                        <Button size="small" icon={<DownloadOutlined />} onClick={handleInstallCert}>
                                            Cert
                                        </Button>
                                    </Tooltip>
                                    {certTrustStatus && certTrustStatus !== 'no_proxy' && certTrustStatus !== 'unknown' && (
                                        <Tag
                                            color={
                                                certTrustStatus === 'trusted' ? 'success' :
                                                certTrustStatus === 'not_trusted' ? 'error' :
                                                certTrustStatus === 'pending' ? 'warning' :
                                                certTrustStatus === 'checking' ? 'processing' :
                                                'default'
                                            }
                                            style={{ marginRight: 0 }}
                                        >
                                            {certTrustStatus === 'trusted' ? '✓' :
                                             certTrustStatus === 'not_trusted' ? '✗' :
                                             certTrustStatus === 'pending' ? '?' :
                                             certTrustStatus === 'checking' ? '...' : '?'}
                                        </Tag>
                                    )}
                                    <Button size="small" icon={<SettingOutlined />} onClick={() => setBypassModalOpen(true)}>
                                        Rules
                                    </Button>
                                </Space>
                            )}
                        </Space>
                        <Divider type="vertical" />
                        <Space size={4} wrap style={{ flexShrink: 0 }}>
                            <Button.Group size="small">
                                <Tooltip title={t('proxy.mock_rules')}>
                                    <Button icon={<BlockOutlined />} onClick={handleOpenMockListModal}>
                                        Mock
                                    </Button>
                                </Tooltip>
                                <Tooltip title={t('proxy.add_mock_rule')}>
                                    <Button icon={<PlusOutlined />} onClick={() => { mockForm.resetFields(); setMockConditionHints(null); openMockEditModal(null); }} />
                                </Tooltip>
                            </Button.Group>
                            <Button.Group size="small">
                                <Tooltip title={t('proxy.breakpoint_rules')}>
                                    <Button icon={<BugOutlined />} onClick={handleOpenBreakpointListModal}>
                                        BP
                                        {pendingBreakpoints.length > 0 && (
                                            <Tag color="red" style={{ marginLeft: 4, padding: '0 4px', fontSize: 11, lineHeight: '18px', borderRadius: 9 }}>
                                                {pendingBreakpoints.length}
                                            </Tag>
                                        )}
                                    </Button>
                                </Tooltip>
                                <Tooltip title={t('proxy.add_breakpoint_rule')}>
                                    <Button icon={<PlusOutlined />} onClick={() => { breakpointForm.resetFields(); openBreakpointEditModal(null); }} />
                                </Tooltip>
                            </Button.Group>
                            <Tooltip title={t('proxy.map_remote_rules')}>
                                <Button size="small" icon={<SwapOutlined />} onClick={handleOpenMapRemoteListModal}>
                                    {t('proxy.map_remote')}
                                </Button>
                            </Tooltip>
                            <Tooltip title={t('proxy.rewrite_rules')}>
                                <Button size="small" icon={<EditOutlined />} onClick={handleOpenRewriteListModal}>
                                    {t('proxy.rewrite')}
                                </Button>
                            </Tooltip>
                            <Tooltip title={t('proxy.proto_management')}>
                                <Button size="small" icon={<CodeOutlined />} onClick={handleOpenProtoListModal}>
                                    Proto
                                </Button>
                            </Tooltip>
                        </Space>
                        <div style={{ marginLeft: 'auto', flexShrink: 0 }}>
                            <Button
                                type="primary"
                                size="small"
                                danger={isRunning}
                                icon={isRunning ? <PoweroffOutlined /> : <PlayCircleOutlined />}
                                onClick={isRunning ? handleStop : handleStart}
                                style={{ height: 32, padding: '0 20px', borderRadius: 16 }}
                            >
                                {isRunning ? t('proxy.stop_capture') : t('proxy.start_capture')}
                            </Button>
                        </div>
                    </div>

                    {/* Row 2: Network Monitor & Limit (conditionally rendered) */}
                    {selectedDevice && (
                        <>
                            <Divider style={{ margin: '0' }} />

                            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                                {/* Left: Monitor */}
                                <Space size="middle">
                                    <Space size="large">
                                        <Space size={4}>
                                            <ArrowDownOutlined style={{ fontSize: '12px', color: '#52c41a' }} />
                                            <Text type="secondary" style={{ fontSize: '10px' }}>RX</Text>
                                            <Text style={{ color: '#52c41a', fontFamily: 'monospace', fontWeight: 600, fontSize: '12px', minWidth: '70px' }}>{formatSpeed(netStats.rxSpeed)}</Text>
                                        </Space>
                                        <Space size={4}>
                                            <ArrowUpOutlined style={{ fontSize: '12px', color: '#1890ff' }} />
                                            <Text type="secondary" style={{ fontSize: '10px' }}>TX</Text>
                                            <Text style={{ color: '#1890ff', fontFamily: 'monospace', fontWeight: 600, fontSize: '12px', minWidth: '70px' }}>{formatSpeed(netStats.txSpeed)}</Text>
                                        </Space>
                                    </Space>
                                </Space>

                                <Divider type="vertical" style={{ height: 32 }} />

                                {/* Right: Controls */}
                                <Space size="small" align="center">
                                    <Tag color="orange" style={{ marginRight: 0 }}>{t('proxy.limit')}</Tag>

                                    {/* Bandwidth */}
                                    <InputNumber
                                        size="small"
                                        prefix={<ArrowDownOutlined style={{ fontSize: 10, color: '#aaa' }} />}
                                        suffix="K"
                                        placeholder="DL"
                                        min={0}
                                        value={dlLimit}
                                        onChange={(val) => setSpeedLimits(val, ulLimit)}
                                        style={{ width: 110 }}
                                        title={t('proxy.dl_limit')}
                                    />
                                    <InputNumber
                                        size="small"
                                        prefix={<ArrowUpOutlined style={{ fontSize: 10, color: '#aaa' }} />}
                                        suffix="K"
                                        placeholder="UL"
                                        min={0}
                                        value={ulLimit}
                                        onChange={(val) => setSpeedLimits(dlLimit, val)}
                                        style={{ width: 110 }}
                                        title={t('proxy.ul_limit')}
                                    />

                                    {/* Latency */}
                                    <InputNumber
                                        size="small"
                                        prefix={<HourglassOutlined style={{ fontSize: 10, color: '#aaa' }} />}
                                        suffix="ms"
                                        placeholder="Delay"
                                        min={0}
                                        value={latency}
                                        onChange={setLatency}
                                        style={{ width: 120 }}
                                        title={t('proxy.latency')}
                                    />

                                    {/* Actions */}
                                    <Button type="primary" size="small" onClick={handleApplyRules}>{t('proxy.apply')}</Button>
                                    <Button size="small" onClick={handleClearRules} icon={<DeleteOutlined />} title={t('common.clear')} />
                                </Space>
                            </div>
                        </>
                    )}
                </div>
            </Card>

            {/* Request List and Detail Panel Container */}
            <div style={{ flex: 1, display: 'flex', gap: 12, overflow: 'hidden' }}>
                <Card
                    style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0 }}
                    styles={{ body: { flex: 1, overflow: 'hidden', padding: 0, display: 'flex', flexDirection: 'column' } }}
                    size="small"
                    title={
                        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                            <Radio.Group size="small" value={filterType} onChange={e => setFilterType(e.target.value)} buttonStyle="solid">
                                <Radio.Button value="ALL">{t('proxy.filter_all')}</Radio.Button>
                                <Radio.Button value="HTTP">{t('proxy.filter_http')}</Radio.Button>
                                <Radio.Button value="WS">{t('proxy.filter_ws')}</Radio.Button>
                            </Radio.Group>
                            <Input
                                placeholder={t('proxy.search_placeholder')}
                                size="small"
                                allowClear
                                style={{ maxWidth: 400 }}
                                value={searchText}
                                onChange={e => setSearchText(e.target.value)}
                            />
                            <Button size="small" type="link" onClick={() => { resendForm.setFieldsValue({ method: 'GET', url: '', headers: '', body: '' }); openResendModal('compose'); }} icon={<SendOutlined />} style={{ padding: 0 }}>{t('proxy.compose_request')}</Button>
                            <Button size="small" type="link" onClick={handleExportHAR} icon={<DownloadOutlined />} style={{ padding: 0 }} disabled={logs.length === 0}>{t('proxy.export_har')}</Button>
                            <Button size="small" type="link" onClick={() => { clearLogs(); clearWSMessages(); setDiffBaseLog(null); }} icon={<DeleteOutlined />} style={{ padding: 0 }}>{t('proxy.clear_logs')}</Button>
                        </div>
                    }
                >
                {/* Pending breakpoints notification bar */}
                {pendingBreakpoints.length > 0 && (
                    <div style={{
                        background: token.colorWarningBg,
                        borderBottom: `1px solid ${token.colorWarningBorder}`,
                        fontSize: 12,
                    }}>
                        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '4px 12px' }}>
                            <Space size={8}>
                                <BugOutlined style={{ color: token.colorWarning }} />
                                <Text style={{ fontSize: 12 }}>
                                    <strong>{pendingBreakpoints.length}</strong> {t('proxy.pending_breakpoints')}
                                </Text>
                            </Space>
                            <Button size="small" type="link" onClick={handleForwardAllBreakpoints} icon={<FastForwardOutlined />}>
                                {t('proxy.breakpoint_forward_all')}
                            </Button>
                        </div>
                        {pendingBreakpoints.map(bp => (
                            <div key={bp.id} style={{
                                display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                                padding: '2px 12px 2px 32px', gap: 8,
                            }}>
                                <div
                                    style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', cursor: 'pointer', color: token.colorLink }}
                                    onClick={() => openBreakpointResolveModal(bp)}
                                    title={bp.url}
                                >
                                    <Tag color={bp.phase === 'request' ? 'green' : 'orange'} style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>{bp.phase === 'request' ? '→ REQ' : '← RES'}</Tag>
                                    <Tag style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>{bp.method}</Tag>
                                    {bp.url.length > 60 ? bp.url.substring(0, 60) + '...' : bp.url}
                                </div>
                                <Space size={2}>
                                    <Button size="small" type="text" icon={<FastForwardOutlined />} onClick={() => handleResolveBreakpoint(bp.id, 'forward')} title={t('proxy.breakpoint_forward')} />
                                    <Button size="small" type="text" danger icon={<StopOutlined />} onClick={() => handleResolveBreakpoint(bp.id, 'drop')} title={t('proxy.breakpoint_drop')} />
                                </Space>
                            </div>
                        ))}
                        <div style={{ height: 4 }} />
                    </div>
                )}
                {/* Diff mode hint bar */}
                {diffBaseLog && !diffModalOpen && (
                    <div style={{
                        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                        padding: '6px 12px',
                        background: `linear-gradient(90deg, ${token.colorWarningBg}, ${token.colorWarningBgHover || token.colorWarningBg})`,
                        borderBottom: `1px solid ${token.colorWarningBorder}`,
                    }}>
                        <Space size={8}>
                            <SwapOutlined style={{ color: token.colorWarning }} />
                            <Text style={{ fontSize: 12, color: token.colorWarningText || token.colorTextBase }}>
                                {t('proxy.diff_select_to_compare')}
                            </Text>
                        </Space>
                        <Button size="small" type="text" onClick={() => setDiffBaseLog(null)}>
                            {t('proxy.diff_cancel')}
                        </Button>
                    </div>
                )}
                {/* Virtual Table Header - Fixed widths */}
                <div style={{ display: 'grid', gridTemplateColumns: '80px 70px 80px 1fr 80px 64px 72px', padding: '8px 12px', background: token.colorFillAlter, borderBottom: `1px solid ${token.colorBorderSecondary}`, fontWeight: 'bold', fontSize: '12px', color: token.colorTextSecondary }}>
                    <div>{t('proxy.col_time')}</div>
                    <div>{t('proxy.col_method')}</div>
                    <div>{t('proxy.col_stat')}</div>
                    <div>{t('proxy.col_url')}</div>
                    <div>{t('proxy.col_type')}</div>
                    <div>{t('proxy.col_size')}</div>
                    <div>{t('proxy.col_duration')}</div>
                </div>

                <VirtualList
                    dataSource={filteredLogs}
                    rowKey="id"
                    rowHeight={35}
                    overscan={20}
                    selectedKey={selectedLog?.id}
                    onItemClick={handleProxyListClick}
                    showBorder={false}
                    style={{ flex: 1 }}
                    renderItem={(record, index, isSelected) => {
                        const isDiffBase = !!diffBaseLog && record.id === diffBaseLog.id;
                        return (
                        <div
                            style={{
                                display: 'grid',
                                gridTemplateColumns: '80px 70px 80px 1fr 80px 64px 72px',
                                padding: '6px 12px',
                                fontSize: '12px',
                                alignItems: 'center',
                                height: '100%',
                                background: isDiffBase
                                    ? token.colorWarningBg
                                    : isSelected 
                                        ? token.colorPrimaryBg 
                                        : index % 2 === 0 
                                            ? token.colorBgContainer 
                                            : token.colorFillAlter,
                                borderBottom: `1px solid ${token.colorBorderSecondary}`,
                                borderLeft: isDiffBase ? `3px solid ${token.colorWarning}` : '3px solid transparent',
                            }}
                        >
                            <div style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', color: '#888' }}>{record.time}</div>
                            <div>
                                {record.method === 'CONNECT' ? (
                                    <Tag color="purple" style={{ marginRight: 0, transform: 'scale(0.8)', transformOrigin: 'left center' }}>TUNNEL</Tag>
                                ) : record.isWsMessage ? (
                                    <Tag color={record.wsMessageInfo?.direction === 'send' ? 'green' : 'blue'} style={{ marginRight: 0, transform: 'scale(0.8)', transformOrigin: 'left center' }}>{record.method}</Tag>
                                ) : record.isWs ? (
                                    <Tag color="cyan" style={{ marginRight: 0, transform: 'scale(0.8)', transformOrigin: 'left center' }}>WS</Tag>
                                ) : (
                                    <Tag style={{ marginRight: 0, transform: 'scale(0.8)', transformOrigin: 'left center', color: getMethodColor(record.method), borderColor: getMethodColor(record.method), background: `${getMethodColor(record.method)}10` }}>{record.method}</Tag>
                                )}
                            </div>
                            <div>
                                {record.isWsMessage ? (
                                    <Tag style={{ marginRight: 0, transform: 'scale(0.8)', transformOrigin: 'left center', fontSize: 10 }}>{record.wsMessageInfo?.typeName}</Tag>
                                ) : (
                                    <>
                                        <Tag color={record.statusCode ? getStatusColor(record.statusCode) : 'default'} style={{ marginRight: 0, transform: 'scale(0.8)', transformOrigin: 'left center' }}>{record.statusCode || '-'}</Tag>
                                        {record.mocked && <Tag color="magenta" style={{ marginLeft: 2, transform: 'scale(0.7)', transformOrigin: 'left center' }}>M</Tag>}
                                    </>
                                )}
                            </div>
                            <div title={record.isWsMessage ? (record.wsMessageInfo?.payload || '') : record.url} style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', color: record.isWsMessage ? token.colorTextSecondary : token.colorLink, fontFamily: record.isWsMessage ? 'monospace' : undefined, fontSize: record.isWsMessage ? 11 : undefined }}>
                                {record.isWsMessage ? (record.wsMessageInfo?.payload || record.url) : record.url}
                                {!record.isWsMessage && record.isHttps && <LockOutlined style={{ fontSize: '10px', marginLeft: 4, color: '#52c41a' }} />}
                            </div>
                            <div style={{ color: '#888', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{record.isWsMessage ? (record.wsMessageInfo?.isProtobuf ? 'protobuf' : (record.wsMessageInfo?.isBinary ? 'binary' : 'text')) : (record.contentType?.split(';')[0].split('/')[1] || '-')}</div>
                            <div style={{ fontFamily: 'monospace', color: '#666' }}>{formatBytes(record.bodySize || 0)}</div>
                            <div style={{ fontFamily: 'monospace', color: record.duration && record.duration > 3000 ? '#ff4d4f' : record.duration && record.duration > 1000 ? '#faad14' : '#666' }}>{formatMs(record.duration)}</div>
                        </div>
                    );}}
                />
            </Card>

                {/* Detail Panel */}
                {selectedLog && (
                    <Card
                        size="small"
                        style={{ width: '50%', minWidth: 400, flexShrink: 0, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}
                        styles={{ body: { flex: 1, overflow: selectedLog.isWsMessage ? 'hidden' : 'auto', padding: 16, display: 'flex', flexDirection: 'column' } }}
                        title={
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <Text strong>{t('proxy.details')}</Text>
                                <Space size="small">
                                    {!selectedLog.isWsMessage && (
                                        <>
                                            <Dropdown
                                                menu={{
                                                    items: [
                                                        { key: 'curl', label: 'cURL' },
                                                        { key: 'fetch', label: 'JavaScript (Fetch)' },
                                                        { key: 'python', label: 'Python (requests)' },
                                                        { key: 'go', label: 'Go (net/http)' },
                                                    ],
                                                    onClick: ({ key }) => handleCopyCode(selectedLog, key as CodeLanguage),
                                                }}
                                            >
                                                <Button
                                                    type="text"
                                                    size="small"
                                                    icon={<CodeOutlined />}
                                                >
                                                    {t('proxy.copy_code')}
                                                </Button>
                                            </Dropdown>
                                            <Button
                                                type="text"
                                                size="small"
                                                icon={<SendOutlined />}
                                                onClick={() => handleOpenResendModal(selectedLog)}
                                            >
                                                {t('proxy.resend')}
                                            </Button>
                                            <Button
                                                type="text"
                                                size="small"
                                                icon={<BlockOutlined />}
                                                onClick={() => createMockFromRequest(selectedLog)}
                                            >
                                                Mock
                                            </Button>
                                            <Button
                                                type="text"
                                                size="small"
                                                icon={<BugOutlined />}
                                                onClick={() => createBreakpointFromRequest(selectedLog)}
                                            >
                                                BP
                                            </Button>
                                            <Tooltip title={diffBaseLog ? t('proxy.diff_compare') : t('proxy.diff_mark')}>
                                                <Button
                                                    type={diffBaseLog && diffBaseLog.id !== selectedLog.id ? "primary" : "text"}
                                                    size="small"
                                                    icon={<SwapOutlined />}
                                                    onClick={() => handleMarkForDiff(selectedLog as any)}
                                                >
                                                    Diff
                                                </Button>
                                            </Tooltip>
                                        </>
                                    )}
                                    <Button
                                        type="text"
                                        size="small"
                                        icon={<CloseOutlined />}
                                        onClick={() => selectLog(null)}
                                    />
                                </Space>
                            </div>
                        }
                    >
                        <div style={{ width: '100%', display: 'flex', flexDirection: 'column', gap: 12, ...(selectedLog.isWsMessage ? { height: '100%' } : {}) }}>
                            <div style={{ wordBreak: 'break-all', fontFamily: 'monospace', background: token.colorFillTertiary, padding: 8, borderRadius: 4, display: 'flex', alignItems: 'flex-start', gap: 8, flexShrink: 0 }}>
                                {selectedLog.isWsMessage ? (
                                    <>
                                        <Tag color={selectedLog.wsMessageInfo?.direction === 'send' ? 'green' : 'blue'} style={{ flexShrink: 0 }}>
                                            {selectedLog.wsMessageInfo?.direction === 'send' ? '↑ Send' : '↓ Recv'}
                                        </Tag>
                                        <Tag style={{ flexShrink: 0 }}>{selectedLog.wsMessageInfo?.typeName}</Tag>
                                        <Text type="secondary" style={{ flexShrink: 0, fontSize: 12 }}>{selectedLog.wsMessageInfo?.payloadSize}B</Text>
                                        {selectedLog.wsMessageInfo?.isProtobuf && <Tag color="purple" style={{ flexShrink: 0 }}>Protobuf</Tag>}
                                    </>
                                ) : (
                                    <>
                                        <Tag style={{ flexShrink: 0, color: getMethodColor(selectedLog.method), borderColor: getMethodColor(selectedLog.method), background: `${getMethodColor(selectedLog.method)}10` }}>{selectedLog.method}</Tag>
                                        <Text copyable={{ text: selectedLog.url }} style={{ fontFamily: 'monospace', fontSize: '13px', flex: 1, wordBreak: 'break-all' }}>{selectedLog.url}</Text>
                                    </>
                                )}
                            </div>

                            {selectedLog.isWsMessage ? (
                                <div style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0, borderRadius: 8, overflow: 'hidden', border: `1px solid ${token.colorBorderSecondary}` }}>
                                    <div style={{ padding: '8px 12px', display: 'flex', gap: 8, alignItems: 'center', background: token.colorFillAlter, flexShrink: 0 }}>
                                        <Text type="secondary" style={{ fontSize: 12 }}>{t('proxy.col_time')}: {selectedLog.time}</Text>
                                        {selectedLog.wsMessageInfo?.connectionId && (
                                            <Text type="secondary" style={{ fontSize: 12 }}>Connection: {selectedLog.wsMessageInfo.connectionId.slice(0, 8)}</Text>
                                        )}
                                    </div>
                                    <div style={{
                                        flex: 1,
                                        minHeight: 0,
                                        overflow: 'auto',
                                        background: token.colorBgContainer,
                                    }}>
                                        <JsonViewer data={selectedLog.wsMessageInfo?.payload || ''} fontSize={12} />
                                    </div>
                                </div>
                            ) : (selectedLog.method === 'CONNECT') ? (
                                <div style={{ textAlign: 'center', padding: '40px 20px', background: token.colorFillAlter, borderRadius: 8 }}>
                                    <LockOutlined style={{ fontSize: 48, color: token.colorTextDisabled, marginBottom: 16 }} />
                                    <br />
                                    <Text type="secondary" style={{ fontStyle: 'italic' }}>
                                        {t('proxy.tunnel_info')}
                                    </Text>
                                </div>
                            ) : (() => {
                                let queryParams: [string, string][] = [];
                                try {
                                    const urlObj = new URL(selectedLog.url);
                                    queryParams = Array.from(urlObj.searchParams.entries());
                                } catch (e) {
                                    if (selectedLog.url.includes('?')) {
                                        const search = selectedLog.url.split('?')[1];
                                        queryParams = search.split('&').map(p => {
                                            const [k, v] = p.split('=');
                                            return [decodeURIComponent(k), decodeURIComponent(v || '')];
                                        });
                                    }
                                }
                                const reqHasHeaders = selectedLog.headers && Object.keys(selectedLog.headers).length > 0;
                                const respHasHeaders = selectedLog.respHeaders && Object.keys(selectedLog.respHeaders).length > 0;
                                const headerGrid = (headers: Record<string, string[]>) => (
                                    <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr', gap: '4px 12px', paddingTop: 8 }}>
                                        {Object.entries(headers).map(([k, v]) => (
                                            <React.Fragment key={k}>
                                                <Text style={{ fontSize: '12px', color: token.colorTextSecondary, textAlign: 'right', fontWeight: 500 }}>{k}:</Text>
                                                <Text copyable={{ text: (v as string[]).join(', ') }} style={{ fontSize: '12px', fontFamily: 'monospace', wordBreak: 'break-all' }}>{(v as string[]).join(', ')}</Text>
                                            </React.Fragment>
                                        ))}
                                    </div>
                                );

                                return (
                                    <Tabs defaultActiveKey={selectedLog.previewBody ? 'reqBody' : 'reqHeaders'} size="small" items={[
                                        {
                                            key: 'reqHeaders',
                                            label: t('proxy.req_headers'),
                                            children: (
                                                <div style={{ paddingTop: 8 }}>
                                                    {reqHasHeaders ? headerGrid(selectedLog.headers as Record<string, string[]>) : (
                                                        <Text type="secondary" style={{ display: 'block', textAlign: 'center', margin: '40px 0' }}>{t('proxy.no_req_data')}</Text>
                                                    )}
                                                </div>
                                            )
                                        },
                                        ...(queryParams.length > 0 ? [{
                                            key: 'queryParams',
                                            label: <span>{t('proxy.query_params_tab')} <Tag style={{ marginLeft: 4, fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>{queryParams.length}</Tag></span>,
                                            children: (
                                                <div style={{ paddingTop: 8 }}>
                                                    <div style={{
                                                        border: `1px solid ${token.colorBorderSecondary}`,
                                                        borderRadius: 6,
                                                        overflow: 'hidden',
                                                    }}>
                                                        <div style={{
                                                            display: 'grid',
                                                            gridTemplateColumns: 'minmax(100px, auto) 1fr',
                                                            fontSize: 12,
                                                        }}>
                                                            <div style={{
                                                                padding: '6px 12px',
                                                                fontWeight: 600,
                                                                color: token.colorTextSecondary,
                                                                background: token.colorFillAlter,
                                                                borderBottom: `1px solid ${token.colorBorderSecondary}`,
                                                                borderRight: `1px solid ${token.colorBorderSecondary}`,
                                                            }}>{t('proxy.param_name')}</div>
                                                            <div style={{
                                                                padding: '6px 12px',
                                                                fontWeight: 600,
                                                                color: token.colorTextSecondary,
                                                                background: token.colorFillAlter,
                                                                borderBottom: `1px solid ${token.colorBorderSecondary}`,
                                                            }}>{t('proxy.param_value')}</div>
                                                            {queryParams.map(([k, v], idx) => (
                                                                <React.Fragment key={idx}>
                                                                    <div style={{
                                                                        padding: '5px 12px',
                                                                        fontWeight: 500,
                                                                        color: token.colorText,
                                                                        borderBottom: idx < queryParams.length - 1 ? `1px solid ${token.colorBorderSecondary}` : 'none',
                                                                        borderRight: `1px solid ${token.colorBorderSecondary}`,
                                                                        wordBreak: 'break-all',
                                                                    }}>{k}</div>
                                                                    <div style={{
                                                                        padding: '5px 12px',
                                                                        borderBottom: idx < queryParams.length - 1 ? `1px solid ${token.colorBorderSecondary}` : 'none',
                                                                    }}>
                                                                        <Text copyable={{ text: v }} style={{ fontSize: 12, fontFamily: 'monospace', wordBreak: 'break-all', color: token.colorLink }}>{v}</Text>
                                                                    </div>
                                                                </React.Fragment>
                                                            ))}
                                                        </div>
                                                    </div>
                                                </div>
                                            )
                                        }] : []),
                                        {
                                            key: 'reqBody',
                                            label: <span>{t('proxy.req_body')}{selectedLog.isReqProtobuf ? ' [PB]' : ''}</span>,
                                            children: (
                                                <div style={{ paddingTop: 8 }}>
                                                    {selectedLog.isReqProtobuf && (
                                                        <div style={{ marginBottom: 8 }}><Tag color="geekblue">Protobuf Decoded</Tag></div>
                                                    )}
                                                    {selectedLog.previewBody ? (
                                                        <JsonViewer data={formatBody(selectedLog.previewBody)} fontSize={12} />
                                                    ) : (
                                                        <Text type="secondary" style={{ display: 'block', textAlign: 'center', margin: '40px 0' }}>{t('proxy.no_body')}</Text>
                                                    )}
                                                </div>
                                            )
                                        },
                                        {
                                            key: 'respHeaders',
                                            label: `${t('proxy.resp_headers')} ${selectedLog.statusCode ? '(' + selectedLog.statusCode + ')' : ''}`,
                                            children: (
                                                <div style={{ paddingTop: 8 }}>
                                                    {selectedLog.statusCode ? (
                                                        <>
                                                            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
                                                                <Tag color={selectedLog.statusCode >= 400 ? 'error' : 'success'} style={{ fontSize: 14, padding: '2px 8px' }}>
                                                                    {selectedLog.statusCode}
                                                                </Tag>
                                                                {selectedLog.mocked && <Tag color="magenta">Mocked</Tag>}
                                                                {selectedLog.contentType && <Tag>{selectedLog.contentType.split(';')[0]}</Tag>}
                                                            </div>
                                                            {respHasHeaders ? headerGrid(selectedLog.respHeaders as Record<string, string[]>) : (
                                                                <Text type="secondary" style={{ display: 'block', textAlign: 'center', margin: '40px 0' }}>{t('proxy.no_resp_data')}</Text>
                                                            )}
                                                        </>
                                                    ) : (
                                                        <div style={{ padding: '60px 20px', textAlign: 'center' }}>
                                                            <HourglassOutlined style={{ fontSize: 48, color: token.colorTextDisabled, marginBottom: 16 }} />
                                                            <br />
                                                            <Text type="secondary" italic>{t('proxy.waiting_for_response')}</Text>
                                                        </div>
                                                    )}
                                                </div>
                                            )
                                        },
                                        {
                                            key: 'respBody',
                                            label: <span>{t('proxy.resp_body')}{selectedLog.isProtobuf ? ' [PB]' : ''}</span>,
                                            children: (
                                                <div style={{ paddingTop: 8 }}>
                                                    {selectedLog.statusCode ? (
                                                        <>
                                                            {selectedLog.isProtobuf && (
                                                                <div style={{ marginBottom: 8 }}><Tag color="geekblue">Protobuf Decoded</Tag></div>
                                                            )}
                                                            {selectedLog.respBody ? (
                                                                <JsonViewer data={formatBody(selectedLog.respBody)} fontSize={12} />
                                                            ) : (
                                                                <Text type="secondary" style={{ display: 'block', textAlign: 'center', margin: '40px 0' }}>{t('proxy.no_body')}</Text>
                                                            )}
                                                        </>
                                                    ) : (
                                                        <div style={{ padding: '60px 20px', textAlign: 'center' }}>
                                                            <HourglassOutlined style={{ fontSize: 48, color: token.colorTextDisabled, marginBottom: 16 }} />
                                                            <br />
                                                            <Text type="secondary" italic>{t('proxy.waiting_for_response')}</Text>
                                                        </div>
                                                    )}
                                                </div>
                                            )
                                        },
                                        ...(selectedLog.isWs ? [{
                                            key: 'wsMessages',
                                            label: <span>{t('proxy.ws_messages')} {(wsMessages[selectedLog.id] || []).length > 0 && <Tag color="cyan" style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px', marginLeft: 4 }}>{(wsMessages[selectedLog.id] || []).length}</Tag>}</span>,
                                            children: (() => {
                                                const msgs = wsMessages[selectedLog.id] || [];
                                                return (
                                                    <div style={{ paddingTop: 8 }}>
                                                        {msgs.length === 0 ? (
                                                            <div style={{ textAlign: 'center', padding: '40px 20px' }}>
                                                                <ApiOutlined style={{ fontSize: 48, color: token.colorTextDisabled, marginBottom: 16 }} />
                                                                <br />
                                                                <Text type="secondary" italic>{t('proxy.ws_no_messages')}</Text>
                                                            </div>
                                                        ) : (
                                                            <div style={{ maxHeight: 400, overflow: 'auto' }}>
                                                                {msgs.map((msg, idx) => (
                                                                    <div key={idx} style={{
                                                                        display: 'flex',
                                                                        gap: 8,
                                                                        padding: '6px 8px',
                                                                        borderBottom: `1px solid ${token.colorBorderSecondary}`,
                                                                        background: msg.direction === 'send' ? 'rgba(82,196,26,0.04)' : 'rgba(24,144,255,0.04)',
                                                                        fontSize: 12,
                                                                    }}>
                                                                        <Tag color={msg.direction === 'send' ? 'green' : 'blue'} style={{ fontSize: 10, lineHeight: '18px', padding: '0 4px', flexShrink: 0 }}>
                                                                            {msg.direction === 'send' ? '↑' : '↓'}
                                                                        </Tag>
                                                                        <Tag style={{ fontSize: 10, lineHeight: '18px', padding: '0 4px', flexShrink: 0 }}>
                                                                            {msg.typeName}
                                                                        </Tag>
                                                                        {msg.isProtobuf && (
                                                                            <Tag color="purple" style={{ fontSize: 10, lineHeight: '18px', padding: '0 4px', flexShrink: 0 }}>
                                                                                Protobuf
                                                                            </Tag>
                                                                        )}
                                                                        <Text type="secondary" style={{ fontSize: 11, flexShrink: 0 }}>
                                                                            {new Date(msg.timestamp).toLocaleTimeString()}
                                                                        </Text>
                                                                        <Text style={{ fontSize: 11, color: token.colorTextSecondary, flexShrink: 0 }}>
                                                                            {msg.payloadSize}B
                                                                        </Text>
                                                                        {(() => {
                                                                            const payload = msg.payload;
                                                                            // Try to parse as JSON (protobuf decoded or plain JSON)
                                                                            let isJson = false;
                                                                            if (msg.isProtobuf || (payload && (payload.startsWith('{') || payload.startsWith('[')))) {
                                                                                try { JSON.parse(payload); isJson = true; } catch { /* not JSON */ }
                                                                            }
                                                                            return isJson ? (
                                                                                <div style={{ flex: 1, overflow: 'hidden' }}>
                                                                                    <JsonViewer data={payload} fontSize={11} collapseDepth={1} searchable={false} />
                                                                                </div>
                                                                            ) : (
                                                                                <div style={{ flex: 1, overflow: 'hidden', fontFamily: 'monospace', fontSize: 11, whiteSpace: 'nowrap', textOverflow: 'ellipsis' }}>
                                                                                    {payload}
                                                                                </div>
                                                                            );
                                                                        })()}
                                                                    </div>
                                                                ))}
                                                            </div>
                                                        )}
                                                    </div>
                                                );
                                            })()
                                        }] : [])
                                     ]} />
                                );
                            })()}
                        </div>
                    </Card>
                )}
            </div>

            <Modal
                title={
                    <Space>
                        <SettingOutlined />
                        <span>{t('proxy.rules_title')}</span>
                    </Space>
                }
                open={isBypassModalOpen}
                onCancel={() => setBypassModalOpen(false)}
                footer={[
                    <Button key="close" type="primary" onClick={() => setBypassModalOpen(false)}>{t('common.close')}</Button>
                ]}
            >
                <div style={{ marginBottom: 16 }}>
                    <Text type="secondary">
                        {t('proxy.rules_desc')}
                    </Text>
                </div>

                <Space.Compact style={{ width: '100%', marginBottom: 16 }}>
                    <Input
                        placeholder={t('proxy.add_keyword')}
                        value={newPattern}
                        onChange={e => setNewPattern(e.target.value)}
                        onPressEnter={() => {
                            if (newPattern && !bypassPatterns.includes(newPattern)) {
                                const next = [...bypassPatterns, newPattern];
                                addBypassPattern(newPattern.trim());
                                SetMITMBypassPatterns(next);
                                setNewPattern("");
                            }
                        }}
                    />
                    <Button type="primary" onClick={() => {
                        if (newPattern && !bypassPatterns.includes(newPattern)) {
                            const next = [...bypassPatterns, newPattern];
                            addBypassPattern(newPattern.trim());
                            SetMITMBypassPatterns(next);
                            setNewPattern("");
                        }
                    }}> {t('proxy.add')} </Button>
                </Space.Compact>

                <div style={{ minHeight: 100, padding: 8, border: `1px solid ${token.colorBorderSecondary}`, borderRadius: 4, background: token.colorFillAlter }}>
                    <Space wrap>
                        {bypassPatterns.map(pat => (
                            <Tag
                                key={pat}
                                closable
                                onClose={async () => {
                                    removeBypassPattern(pat);
                                    const next = bypassPatterns.filter(p => p !== pat);
                                    await SetMITMBypassPatterns(next);
                                }}
                            >
                                {pat}
                            </Tag>
                        ))}
                        {bypassPatterns.length === 0 && <Text type="secondary">{t('proxy.no_rules')}</Text>}
                    </Space>
                </div>
            </Modal>

            {/* Resend Request Modal */}
            <Modal
                title={<Space size={8}>
                    <SendOutlined />
                    {resendMode === 'compose' ? t('proxy.compose_request') : t('proxy.resend_request')}
                    <Text type="secondary" style={{ fontSize: 11, fontWeight: 400 }}>⌘/Ctrl+Enter {t('proxy.to_send')}</Text>
                </Space>}
                open={resendModalOpen}
                onCancel={closeResendModal}
                width={720}
                style={{ top: 40 }}
                styles={{ body: { maxHeight: 'calc(100vh - 160px)', display: 'flex', flexDirection: 'column', overflow: 'hidden', padding: '16px 0 0' } }}
                footer={[
                    <Button key="cancel" onClick={closeResendModal}>{t('common.cancel')}</Button>,
                    <Button key="send" type="primary" loading={resendLoading} onClick={handleResend} icon={<SendOutlined />}>
                        {t('proxy.send')}
                    </Button>
                ]}
            >
                {/* Request Form */}
                <Form form={resendForm} layout="vertical" style={{ flexShrink: 0 }}>
                    {/* Method + URL row */}
                    <Space.Compact style={{ width: '100%' }}>
                        <Form.Item name="method" noStyle>
                            <Select
                                style={{ width: 110 }}
                                options={METHOD_SELECT_OPTIONS}
                            />
                        </Form.Item>
                        <Form.Item name="url" noStyle rules={[{ required: true, message: t('proxy.url_required') }]}>
                            <Input style={{ flex: 1 }} placeholder="https://api.example.com/path" autoComplete="off" />
                        </Form.Item>
                    </Space.Compact>

                    {/* Request Tabs: Headers / Body */}
                    <Tabs
                        size="small"
                        style={{ marginTop: 8 }}
                        destroyOnHidden={false}
                        items={[
                            {
                                key: 'headers',
                                label: t('proxy.headers'),
                                forceRender: true,
                                children: (
                                    <Form.Item name="headers" noStyle>
                                        <Input.TextArea
                                            rows={3}
                                            placeholder={"Content-Type: application/json\nAuthorization: Bearer token"}
                                            style={{ fontFamily: '"SF Mono", Menlo, monospace', fontSize: 12 }}
                                            autoSize={{ minRows: 2, maxRows: 6 }}
                                        />
                                    </Form.Item>
                                ),
                            },
                            {
                                key: 'body',
                                label: t('proxy.body'),
                                forceRender: true,
                                children: (
                                    <Form.Item name="body" noStyle>
                                        <JsonEditor autoHeight minHeight={120} maxHeight={280} placeholder='{"key": "value"}' />
                                    </Form.Item>
                                ),
                            },
                        ]}
                    />
                </Form>

                {/* Response area */}
                {resendResponse && (
                    <div style={{ flex: 1, marginTop: 8, display: 'flex', flexDirection: 'column', overflow: 'hidden', minHeight: 120, border: `1px solid ${token.colorBorderSecondary}`, borderRadius: 8 }}>
                        {/* Response status bar */}
                        <div style={{ padding: '6px 12px', borderBottom: `1px solid ${token.colorBorderSecondary}`, background: token.colorFillAlter, borderRadius: '8px 8px 0 0', flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                            <Space size={8} wrap>
                                <Tag color={getStatusColor(resendResponse.statusCode)} style={{ margin: 0 }}>
                                    {resendResponse.statusCode} {resendResponse.status?.replace(/^\d+\s*/, '')}
                                </Tag>
                                {resendResponse.mocked && <Tag color="magenta" style={{ margin: 0 }}>Mocked</Tag>}
                            </Space>
                            <Space size={12}>
                                <Text type="secondary" style={{ fontSize: 12 }}>{resendResponse.duration}ms</Text>
                                <Text type="secondary" style={{ fontSize: 12 }}>{formatBytes(resendResponse.bodySize)}</Text>
                                {resendResponse.contentType && <Text type="secondary" style={{ fontSize: 12 }}>{resendResponse.contentType.split(';')[0]}</Text>}
                            </Space>
                        </div>

                        {/* Response Tabs: Headers / Body (same order as request) */}
                        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minHeight: 0 }}>
                            <Tabs
                                size="small"
                                style={{ flex: 1, padding: '0 12px' }}
                                items={[
                                    {
                                        key: 'headers',
                                        label: `${t('proxy.headers')}${resendResponse.headers ? ` (${Object.keys(resendResponse.headers).length})` : ''}`,
                                        children: (
                                            <div style={{ overflow: 'auto', maxHeight: 'calc(100vh - 460px)' }}>
                                                {resendResponse.headers && Object.keys(resendResponse.headers).length > 0 ? (
                                                    <table style={{ width: '100%', fontSize: 12, fontFamily: '"SF Mono", Menlo, monospace', borderCollapse: 'collapse' }}>
                                                        <tbody>
                                                            {Object.entries(resendResponse.headers as Record<string, string>).map(([key, val]) => (
                                                                <tr key={key} style={{ borderBottom: `1px solid ${token.colorBorderSecondary}` }}>
                                                                    <td style={{ padding: '4px 8px 4px 0', fontWeight: 600, whiteSpace: 'nowrap', verticalAlign: 'top', color: token.colorTextSecondary }}>{key}</td>
                                                                    <td style={{ padding: '4px 0', wordBreak: 'break-all' }}>{val}</td>
                                                                </tr>
                                                            ))}
                                                        </tbody>
                                                    </table>
                                                ) : (
                                                    <Text type="secondary">{t('proxy.no_headers')}</Text>
                                                )}
                                            </div>
                                        ),
                                    },
                                    {
                                        key: 'body',
                                        label: t('proxy.body'),
                                        children: (
                                            <div style={{ overflow: 'auto', maxHeight: 'calc(100vh - 460px)' }}>
                                                {resendResponse.contentType?.includes('json') || (resendResponse.body?.trim().startsWith('{') || resendResponse.body?.trim().startsWith('[')) ? (
                                                    <JsonEditor
                                                        value={formatBody(resendResponse.body, resendResponse.contentType)}
                                                        readOnly
                                                        height={Math.min(Math.max((resendResponse.body?.split('\n').length || 1) * 19 + 16, 80), 360)}
                                                        language="json"
                                                    />
                                                ) : (
                                                    <pre style={{ margin: 0, padding: 8, fontSize: 12, fontFamily: '"SF Mono", Menlo, monospace', whiteSpace: 'pre-wrap', wordBreak: 'break-all', background: token.colorFillQuaternary, borderRadius: 6, minHeight: 60 }}>
                                                        {resendResponse.body || t('proxy.empty_body')}
                                                    </pre>
                                                )}
                                            </div>
                                        ),
                                    },
                                ]}
                            />
                        </div>
                    </div>
                )}
            </Modal>

            {/* Mock Rules List Modal */}
            <Modal
                title={t('proxy.mock_rules')}
                open={mockListModalOpen}
                onCancel={closeMockListModal}
                width={800}
                footer={null}
                style={{ top: 32, paddingBottom: 0 }}
            >
                <div style={{ marginBottom: 12, display: 'flex', gap: 8 }}>
                    <Button type="primary" icon={<PlusOutlined />} onClick={() => { closeMockListModal(); mockForm.resetFields(); setMockConditionHints(null); openMockEditModal(null); }}>
                        {t('proxy.add_mock_rule')}
                    </Button>
                    <Button icon={<DownloadOutlined />} onClick={handleExportMockRules} disabled={mockRules.length === 0}>
                        {t('proxy.mock_export')}
                    </Button>
                    <Button icon={<CloudDownloadOutlined />} onClick={handleImportMockRules}>
                        {t('proxy.mock_import')}
                    </Button>
                </div>
                <div style={{ maxHeight: 'calc(100vh - 220px)', overflow: 'auto' }}>
                    {mockRules.length === 0 ? (
                        <Text type="secondary" style={{ display: 'block', textAlign: 'center', padding: 32 }}>{t('proxy.no_mock_rules')}</Text>
                    ) : (
                        <Table
                            dataSource={mockRules}
                            rowKey="id"
                            size="small"
                            pagination={false}
                            columns={[
                                { title: t('proxy.url_pattern'), dataIndex: 'urlPattern', ellipsis: true },
                                { title: t('proxy.col_method'), dataIndex: 'method', width: 80, render: (v: string) => v || '*' },
                                { title: t('proxy.status_code'), dataIndex: 'statusCode', width: 80 },
                                { title: t('proxy.delay_ms'), dataIndex: 'delay', width: 80, render: (v: number) => v ? `${v}ms` : '-' },
                                { title: t('proxy.conditions'), dataIndex: 'conditions', width: 90, render: (v: any[]) => v && v.length > 0 ? <Tag color="blue">{v.length}</Tag> : '-' },
                                {
                                    title: t('proxy.mock_enabled'),
                                    dataIndex: 'enabled',
                                    width: 80,
                                    render: (v: boolean, record: any) => (
                                        <Switch size="small" checked={v} onChange={(checked) => handleToggleMockRule(record.id, checked)} />
                                    )
                                },
                                {
                                    title: '',
                                    width: 100,
                                    render: (_: any, record: any) => (
                                        <Space size="small">
                                            <Button type="text" size="small" icon={<EditOutlined />} onClick={() => startEditMockRule(record)} />
                                            <Popconfirm title={t('common.delete') + '?'} onConfirm={() => handleDeleteMockRule(record.id)}>
                                                <Button type="text" size="small" danger icon={<DeleteOutlined />} />
                                            </Popconfirm>
                                        </Space>
                                    )
                                }
                            ]}
                        />
                    )}
                </div>
            </Modal>

            {/* Mock Rule Edit Modal */}
            <Modal
                title={editingMockRule ? t('proxy.edit_mock_rule') : t('proxy.add_mock_rule')}
                open={mockEditModalOpen}
                onCancel={() => { closeMockEditModal(); mockForm.resetFields(); }}
                width={900}
                footer={null}
                style={{ top: 32, paddingBottom: 0 }}
            >
                <Form form={mockForm} layout="vertical" size="small">
                    <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
                        <Form.Item name="urlPattern" label={t('proxy.url_pattern')} rules={[{ required: true }]} style={{ marginBottom: 0, flex: 1, minWidth: 0 }}>
                            <AutoComplete
                                placeholder="*/api/*"
                                options={getUrlPatternsForMethod(mockFormMethod)}
                                filterOption={(inputValue, option) =>
                                    (option?.value as string)?.toLowerCase().includes(inputValue.toLowerCase())
                                }
                            />
                        </Form.Item>
                        <Form.Item name="method" label={t('proxy.col_method')} style={{ marginBottom: 0, width: 100, flexShrink: 0 }}>
                            <Select allowClear placeholder="All" options={METHOD_SELECT_OPTIONS} />
                        </Form.Item>
                        <Form.Item name="statusCode" label={t('proxy.status_code')} style={{ marginBottom: 0, width: 90, flexShrink: 0 }}>
                            <InputNumber min={100} max={599} placeholder="200" style={{ width: '100%' }} />
                        </Form.Item>
                        <Form.Item name="delay" label={t('proxy.delay_ms')} style={{ marginBottom: 0, width: 80, flexShrink: 0 }}>
                            <InputNumber min={0} placeholder="0" style={{ width: '100%' }} />
                        </Form.Item>
                    </div>
                    <Form.Item name="contentType" label="Content-Type" style={{ marginBottom: 8 }}>
                        <Input placeholder="application/json" />
                    </Form.Item>
                    <Form.Item name="body" label={t('proxy.response_body')} style={{ marginBottom: 8 }}>
                        <JsonEditor height={'calc(100vh - 560px)'} placeholder='{"success": true}' />
                    </Form.Item>
                    <Form.Item name="bodyFile" label={t('proxy.body_file')} style={{ marginBottom: 8 }}
                        tooltip={t('proxy.body_file_tooltip')}
                    >
                        <Input placeholder="/path/to/response.json" allowClear />
                    </Form.Item>
                    <Form.Item name="description" label={t('proxy.description')} style={{ marginBottom: 12 }}>
                        <Input placeholder={t('proxy.description')} />
                    </Form.Item>

                    {/* Conditions editor */}
                    <Divider style={{ margin: '8px 0' }}>{t('proxy.conditions')}</Divider>
                    <Form.List name="conditions">
                        {(fields, { add, remove }) => (
                            <>
                                {fields.map(({ key, name, ...restField }) => {
                                    const rowType = watchedConditions?.[name]?.type;
                                    const rowOperator = watchedConditions?.[name]?.operator;
                                    const isBodyType = rowType === 'body';
                                    const isExistenceOp = rowOperator === 'exists' || rowOperator === 'not_exists';

                                    // Filter AutoComplete presets based on selected condition type
                                    const keyOptions = (() => {
                                        const showHeaders = rowType !== 'query';
                                        const showQuery = rowType !== 'header';
                                        const groups: Array<{ label: React.ReactNode; options: Array<{ value: string; label: React.ReactNode }> }> = [];
                                        if (showHeaders && conditionKeyPresets.headers.length > 0) {
                                            groups.push({
                                                label: <span style={{ fontWeight: 600, fontSize: 11, color: '#888' }}>Headers</span>,
                                                options: conditionKeyPresets.headers.map(h => ({
                                                    value: h.key,
                                                    label: (
                                                        <span>
                                                            <strong>{h.key}</strong>
                                                            <span style={{ color: '#888', marginLeft: 6, fontSize: 12 }}>
                                                                {h.value.length > 30 ? h.value.substring(0, 30) + '...' : h.value}
                                                            </span>
                                                        </span>
                                                    ),
                                                })),
                                            });
                                        }
                                        if (showQuery && conditionKeyPresets.queryParams.length > 0) {
                                            groups.push({
                                                label: <span style={{ fontWeight: 600, fontSize: 11, color: '#888' }}>Query Params</span>,
                                                options: conditionKeyPresets.queryParams.map(q => ({
                                                    value: q.key,
                                                    label: (
                                                        <span>
                                                            <strong>{q.key}</strong>
                                                            <span style={{ color: '#888' }}> = {q.value}</span>
                                                        </span>
                                                    ),
                                                })),
                                            });
                                        }
                                        return groups;
                                    })();

                                    return (
                                    <Space key={key} align="baseline" style={{ display: 'flex', marginBottom: 4 }} wrap>
                                        <Form.Item {...restField} name={[name, 'type']} style={{ marginBottom: 0, width: 100 }}>
                                            <Select placeholder={t('proxy.condition_type')} size="small"
                                                onChange={() => {
                                                    mockForm.setFieldValue(['conditions', name, 'key'], '');
                                                    mockForm.setFieldValue(['conditions', name, 'value'], '');
                                                }}
                                            >
                                                <Select.Option value="header">{t('proxy.condition_header')}</Select.Option>
                                                <Select.Option value="query">{t('proxy.condition_query')}</Select.Option>
                                                <Select.Option value="body">{t('proxy.condition_body')}</Select.Option>
                                            </Select>
                                        </Form.Item>

                                        {/* Key field — disabled for body type, AutoComplete with presets filtered by type */}
                                        <Form.Item {...restField} name={[name, 'key']} style={{ marginBottom: 0, width: 200 }}>
                                            <AutoComplete
                                                options={isBodyType ? [] : keyOptions}
                                                placeholder={isBodyType ? '-' : t('proxy.condition_key')}
                                                size="small"
                                                disabled={isBodyType}
                                                filterOption={(input, option) =>
                                                    ((option as { value?: string })?.value ?? '').toLowerCase().includes(input.toLowerCase())
                                                }
                                                onSelect={(val: string) => {
                                                    const hint = conditionKeyPresets.headers.find(h => h.key === val)
                                                        || conditionKeyPresets.queryParams.find(q => q.key === val);
                                                    if (hint) {
                                                        mockForm.setFieldValue(['conditions', name, 'value'], hint.value);
                                                    }
                                                }}
                                            />
                                        </Form.Item>

                                        <Form.Item {...restField} name={[name, 'operator']} style={{ marginBottom: 0, width: 120 }}>
                                            <Select placeholder={t('proxy.condition_operator')} size="small"
                                                onChange={(val: string) => {
                                                    // Clear value when switching to existence operators
                                                    if (val === 'exists' || val === 'not_exists') {
                                                        mockForm.setFieldValue(['conditions', name, 'value'], '');
                                                    }
                                                }}
                                            >
                                                <Select.Option value="equals">{t('proxy.op_equals')}</Select.Option>
                                                <Select.Option value="contains">{t('proxy.op_contains')}</Select.Option>
                                                <Select.Option value="regex">{t('proxy.op_regex')}</Select.Option>
                                                <Select.Option value="exists">{t('proxy.op_exists')}</Select.Option>
                                                <Select.Option value="not_exists">{t('proxy.op_not_exists')}</Select.Option>
                                            </Select>
                                        </Form.Item>
                                        {/* Value field — hidden for exists/not_exists operators */}
                                        {!isExistenceOp && (
                                        <Form.Item {...restField} name={[name, 'value']} style={{ marginBottom: 0, flex: 1, minWidth: 140 }}>
                                            <Input placeholder={t('proxy.condition_value')} size="small" />
                                        </Form.Item>
                                        )}
                                        <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#ff4d4f' }} />
                                    </Space>
                                    );
                                })}
                                <Button type="dashed" onClick={() => add({ type: 'header', key: '', operator: 'equals', value: '' })} icon={<PlusOutlined />} size="small" style={{ marginBottom: 12 }}>
                                    {t('proxy.add_condition')}
                                </Button>
                            </>
                        )}
                    </Form.List>

                    <Space>
                        <Button type="primary" onClick={handleSaveMockRule}>
                            {editingMockRule ? t('common.save') : t('proxy.add_mock_rule')}
                        </Button>
                        <Button onClick={() => { closeMockEditModal(); mockForm.resetFields(); }}>
                            {t('common.cancel')}
                        </Button>
                    </Space>
                </Form>
            </Modal>

            {/* Proto Management Modal */}
            <Modal
                title={t('proxy.proto_management')}
                open={protoListModalOpen}
                onCancel={closeProtoListModal}
                width={900}
                footer={null}
                style={{ top: 32, paddingBottom: 0 }}
            >
                <Tabs size="small" items={[
                    {
                        key: 'files',
                        label: t('proxy.proto_files'),
                        children: (
                            <div>
                                <div style={{ marginBottom: 12 }}>
                                    <Space wrap>
                                        <Button type="primary" icon={<PlusOutlined />} onClick={() => { protoFileForm.resetFields(); openProtoEditFileModal(null); }}>
                                            {t('proxy.add_proto_file')}
                                        </Button>
                                        <Button icon={<FolderOpenOutlined />} loading={protoImportLoading} onClick={handleImportProtoFromDisk}>
                                            {t('proxy.import_local_file')}
                                        </Button>
                                        <Button icon={<CloudDownloadOutlined />} loading={protoImportLoading} onClick={handleImportProtoFromURL}>
                                            {t('proxy.import_from_url')}
                                        </Button>
                                    </Space>
                                </div>
                                <div style={{ maxHeight: 'calc(100vh - 320px)', overflow: 'auto' }}>
                                    {protoFiles.length === 0 ? (
                                        <Text type="secondary" style={{ display: 'block', textAlign: 'center', padding: 32 }}>{t('proxy.no_proto_files')}</Text>
                                    ) : (
                                        <Table
                                            dataSource={protoFiles}
                                            rowKey="id"
                                            size="small"
                                            pagination={false}
                                            columns={[
                                                { title: t('proxy.proto_file_name'), dataIndex: 'name', ellipsis: true },
                                                {
                                                    title: t('proxy.proto_file_size'),
                                                    dataIndex: 'content',
                                                    width: 100,
                                                    render: (content: string) => `${(content || '').length} chars`
                                                },
                                                {
                                                    title: '',
                                                    width: 100,
                                                    render: (_: any, record: any) => (
                                                        <Space size="small">
                                                            <Button type="text" size="small" icon={<EditOutlined />} onClick={() => startEditProtoFile(record)} />
                                                            <Popconfirm title={t('common.delete') + '?'} onConfirm={() => handleDeleteProtoFile(record.id)}>
                                                                <Button type="text" size="small" danger icon={<DeleteOutlined />} />
                                                            </Popconfirm>
                                                        </Space>
                                                    )
                                                }
                                            ]}
                                        />
                                    )}
                                </div>
                            </div>
                        )
                    },
                    {
                        key: 'mappings',
                        label: t('proxy.proto_mappings'),
                        children: (
                            <div>
                                <div style={{ marginBottom: 12 }}>
                                    <Button type="primary" icon={<PlusOutlined />} onClick={() => { protoMappingForm.resetFields(); openProtoEditMappingModal(null); }}>
                                        {t('proxy.add_proto_mapping')}
                                    </Button>
                                </div>
                                <div style={{ maxHeight: 'calc(100vh - 320px)', overflow: 'auto' }}>
                                    {protoMappings.length === 0 ? (
                                        <Text type="secondary" style={{ display: 'block', textAlign: 'center', padding: 32 }}>{t('proxy.no_proto_mappings')}</Text>
                                    ) : (
                                        <Table
                                            dataSource={protoMappings}
                                            rowKey="id"
                                            size="small"
                                            pagination={false}
                                            columns={[
                                                { title: t('proxy.url_pattern'), dataIndex: 'urlPattern', ellipsis: true },
                                                { title: t('proxy.message_type'), dataIndex: 'messageType', ellipsis: true },
                                                { title: t('proxy.direction'), dataIndex: 'direction', width: 100, render: (dir: string) => dir === 'both' ? t('proxy.proto_both') : dir === 'request' ? t('proxy.request') : dir === 'response' ? t('proxy.response') : dir },
                                                { title: t('proxy.description'), dataIndex: 'description', ellipsis: true },
                                                {
                                                    title: '',
                                                    width: 100,
                                                    render: (_: any, record: any) => (
                                                        <Space size="small">
                                                            <Button type="text" size="small" icon={<EditOutlined />} onClick={() => startEditProtoMapping(record)} />
                                                            <Popconfirm title={t('common.delete') + '?'} onConfirm={() => handleDeleteProtoMapping(record.id)}>
                                                                <Button type="text" size="small" danger icon={<DeleteOutlined />} />
                                                            </Popconfirm>
                                                        </Space>
                                                    )
                                                }
                                            ]}
                                        />
                                    )}
                                </div>
                            </div>
                        )
                    }
                ]} />
            </Modal>

            {/* Proto File Edit Modal */}
            <Modal
                title={editingProtoFile ? t('proxy.edit_proto_file') : t('proxy.add_proto_file')}
                open={protoEditFileModalOpen}
                onCancel={() => { closeProtoEditFileModal(); protoFileForm.resetFields(); }}
                width={800}
                footer={null}
                style={{ top: 32, paddingBottom: 0 }}
            >
                <Form form={protoFileForm} layout="vertical" size="small" style={{ marginTop: 8 }}>
                    <Form.Item name="name" label={t('proxy.proto_file_name')} rules={[{ required: true }]}>
                        <Input placeholder="user.proto" />
                    </Form.Item>
                    <Form.Item name="content" label={t('proxy.proto_file_content')} rules={[{ required: true }]}>
                        <Input.TextArea
                            rows={18}
                            placeholder={'syntax = "proto3";\n\npackage example;\n\nmessage UserResponse {\n  int32 id = 1;\n  string name = 2;\n  string email = 3;\n}'}
                            style={{ fontFamily: "'Fira Code', monospace", fontSize: 12 }}
                        />
                    </Form.Item>
                    <Space>
                        <Button type="primary" onClick={handleSaveProtoFile}>
                            {editingProtoFile ? t('common.save') : t('proxy.add_proto_file')}
                        </Button>
                        <Button onClick={() => { closeProtoEditFileModal(); protoFileForm.resetFields(); }}>
                            {t('common.cancel')}
                        </Button>
                    </Space>
                </Form>
            </Modal>

            {/* Proto Mapping Edit Modal */}
            <Modal
                title={editingProtoMapping ? t('proxy.edit_proto_mapping') : t('proxy.add_proto_mapping')}
                open={protoEditMappingModalOpen}
                onCancel={() => { closeProtoEditMappingModal(); protoMappingForm.resetFields(); }}
                width={600}
                footer={null}
                style={{ top: 32 }}
            >
                <Form form={protoMappingForm} layout="vertical" size="small" style={{ marginTop: 8 }}>
                    <Form.Item name="urlPattern" label={t('proxy.url_pattern')} rules={[{ required: true }]}>
                        <AutoComplete
                            placeholder="*/api/user/*"
                            options={getUrlPatternsForMethod()}
                            filterOption={(inputValue, option) =>
                                (option?.value as string)?.toLowerCase().includes(inputValue.toLowerCase())
                            }
                        />
                    </Form.Item>
                    <Form.Item name="messageType" label={t('proxy.message_type')} rules={[{ required: true }]}>
                        <AutoComplete
                            placeholder="example.UserResponse"
                            options={protoMessageTypes.map(msgType => ({
                                value: msgType,
                                label: <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{msgType}</span>,
                            }))}
                            filterOption={(inputValue, option) =>
                                (option?.value as string)?.toLowerCase().includes(inputValue.toLowerCase())
                            }
                        />
                    </Form.Item>
                    <Form.Item name="direction" label={t('proxy.direction')} initialValue="response">
                        <Radio.Group buttonStyle="solid" size="small">
                            <Radio.Button value="response">{t('proxy.response')}</Radio.Button>
                            <Radio.Button value="request">{t('proxy.request')}</Radio.Button>
                            <Radio.Button value="both">{t('proxy.proto_both')}</Radio.Button>
                        </Radio.Group>
                    </Form.Item>
                    <Form.Item name="description" label={t('proxy.description')}>
                        <Input placeholder={t('proxy.description')} />
                    </Form.Item>
                    <Space>
                        <Button type="primary" onClick={handleSaveProtoMapping}>
                            {editingProtoMapping ? t('common.save') : t('proxy.add_proto_mapping')}
                        </Button>
                        <Button onClick={() => { closeProtoEditMappingModal(); protoMappingForm.resetFields(); }}>
                            {t('common.cancel')}
                        </Button>
                    </Space>
                </Form>
            </Modal>

            {/* Proto Import URL Modal */}
            <Modal
                title={t('proxy.import_from_url')}
                open={protoImportURLModalOpen}
                onCancel={closeProtoImportURLModal}
                onOk={handleConfirmImportProtoFromURL}
                confirmLoading={protoImportLoading}
                okText={t('proxy.import')}
                cancelText={t('common.cancel')}
                width={500}
            >
                <div style={{ marginTop: 16 }}>
                    <Input
                        placeholder="https://raw.githubusercontent.com/..."
                        value={protoImportURL}
                        onChange={(e) => setProtoImportURL(e.target.value)}
                        onPressEnter={handleConfirmImportProtoFromURL}
                        autoFocus
                    />
                </div>
            </Modal>

            {/* === Breakpoint Rules List Modal === */}
            <Modal
                open={breakpointListModalOpen}
                onCancel={closeBreakpointListModal}
                title={t('proxy.breakpoint_rules')}
                width={700}
                footer={null}
                style={{ top: 32 }}
            >
                <div style={{ marginBottom: 12 }}>
                    <Button type="primary" icon={<PlusOutlined />} onClick={() => { closeBreakpointListModal(); breakpointForm.resetFields(); openBreakpointEditModal(null); }}>
                        {t('proxy.add_breakpoint_rule')}
                    </Button>
                </div>
                {breakpointRules.length === 0 ? (
                    <Text type="secondary" style={{ display: 'block', textAlign: 'center', padding: 32 }}>{t('proxy.no_breakpoint_rules')}</Text>
                ) : (
                    <Table
                        dataSource={breakpointRules}
                        rowKey="id"
                        size="small"
                        pagination={false}
                        columns={[
                            { title: t('proxy.breakpoint_url_pattern'), dataIndex: 'urlPattern', key: 'urlPattern', ellipsis: true },
                            { title: t('proxy.breakpoint_method'), dataIndex: 'method', key: 'method', width: 80, render: (v: string) => v || '*' },
                            { title: t('proxy.breakpoint_phase'), dataIndex: 'phase', key: 'phase', width: 100, render: (v: string) => (
                                <Tag color={v === 'both' ? 'blue' : v === 'request' ? 'green' : 'orange'}>
                                    {v === 'request' ? t('proxy.breakpoint_phase_request') : v === 'response' ? t('proxy.breakpoint_phase_response') : t('proxy.breakpoint_phase_both')}
                                </Tag>
                            )},
                            { title: t('proxy.breakpoint_enabled'), key: 'enabled', width: 70, render: (_: any, record: any) => (
                                <Switch size="small" checked={record.enabled} onChange={(checked) => handleToggleBreakpointRule(record.id, checked)} />
                            )},
                            { key: 'actions', width: 80, render: (_: any, record: any) => (
                                <Space size={4}>
                                    <Button type="link" size="small" icon={<EditOutlined />} onClick={() => startEditBreakpointRule(record)} />
                                    <Popconfirm title="Delete?" onConfirm={() => handleDeleteBreakpointRule(record.id)}>
                                        <Button type="link" size="small" danger icon={<DeleteOutlined />} />
                                    </Popconfirm>
                                </Space>
                            )},
                        ]}
                    />
                )}
                {/* Pending breakpoints section */}
                {pendingBreakpoints.length > 0 && (
                    <>
                        <Divider style={{ margin: '12px 0' }} />
                        <div style={{ marginBottom: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                            <Text strong>{t('proxy.pending_breakpoints')} ({pendingBreakpoints.length})</Text>
                            <Button size="small" onClick={handleForwardAllBreakpoints} icon={<FastForwardOutlined />}>{t('proxy.breakpoint_forward_all')}</Button>
                        </div>
                        {pendingBreakpoints.map(bp => (
                            <div key={bp.id} style={{
                                display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                                padding: '6px 8px', marginBottom: 4,
                                background: token.colorWarningBg, borderRadius: 4,
                                border: `1px solid ${token.colorWarningBorder}`,
                            }}>
                                <Space size={8}>
                                    <Tag color={bp.phase === 'request' ? 'green' : 'orange'}>{bp.phase}</Tag>
                                    <Tag>{bp.method}</Tag>
                                    <Text style={{ fontSize: 12 }} ellipsis>{bp.url.length > 50 ? bp.url.substring(0, 50) + '...' : bp.url}</Text>
                                </Space>
                                <Space size={4}>
                                    <Button size="small" type="primary" onClick={() => openBreakpointResolveModal(bp)}>{t('proxy.breakpoint_resolve')}</Button>
                                    <Button size="small" onClick={() => handleResolveBreakpoint(bp.id, 'forward')}>{t('proxy.breakpoint_forward')}</Button>
                                    <Button size="small" danger onClick={() => handleResolveBreakpoint(bp.id, 'drop')}>{t('proxy.breakpoint_drop')}</Button>
                                </Space>
                            </div>
                        ))}
                    </>
                )}
            </Modal>

            {/* === Breakpoint Rule Edit Modal === */}
            <Modal
                open={breakpointEditModalOpen}
                onCancel={closeBreakpointEditModal}
                title={editingBreakpointRule ? t('proxy.edit_breakpoint_rule') : t('proxy.add_breakpoint_rule')}
                width={500}
                onOk={handleSaveBreakpointRule}
                okText={editingBreakpointRule ? t('proxy.edit_breakpoint_rule') : t('proxy.add_breakpoint_rule')}
            >
                <Form form={breakpointForm} layout="vertical" initialValues={{ phase: 'both' }}>
                    <Form.Item name="urlPattern" label={t('proxy.breakpoint_url_pattern')} rules={[{ required: true }]}>
                        <AutoComplete
                            placeholder="*/api/*"
                            options={getUrlPatternsForMethod(breakpointFormMethod)}
                            filterOption={(inputValue, option) =>
                                (option?.value as string)?.toLowerCase().includes(inputValue.toLowerCase())
                            }
                        />
                    </Form.Item>
                    <Form.Item name="method" label={t('proxy.breakpoint_method')}>
                        <Select allowClear placeholder="All" options={METHOD_SELECT_OPTIONS} />
                    </Form.Item>
                    <Form.Item name="phase" label={t('proxy.breakpoint_phase')} rules={[{ required: true }]}>
                        <Radio.Group>
                            <Radio value="request">{t('proxy.breakpoint_phase_request')}</Radio>
                            <Radio value="response">{t('proxy.breakpoint_phase_response')}</Radio>
                            <Radio value="both">{t('proxy.breakpoint_phase_both')}</Radio>
                        </Radio.Group>
                    </Form.Item>
                    <Form.Item name="description" label={t('proxy.breakpoint_description')}>
                        <Input placeholder="Optional description" />
                    </Form.Item>
                </Form>
            </Modal>

            {/* === Breakpoint Resolve Modal (Editable) === */}
            <Modal
                open={breakpointResolveModalOpen}
                onCancel={() => closeBreakpointResolveModal()}
                title={t('proxy.breakpoint_resolve')}
                width={860}
                footer={
                    selectedBreakpoint && breakpointEdit ? (() => {
                        const mods = buildModifications(selectedBreakpoint, breakpointEdit);
                        const hasChanges = !!mods;
                        return (
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <Text type="secondary" style={{ fontSize: 11 }}>{t('proxy.breakpoint_auto_timeout')}</Text>
                                <Space>
                                    <Button onClick={() => handleResolveBreakpoint(selectedBreakpoint.id, 'drop')} danger icon={<StopOutlined />}>
                                        {t('proxy.breakpoint_drop')}
                                    </Button>
                                    <Button
                                        type="primary"
                                        icon={<FastForwardOutlined />}
                                        onClick={() => handleResolveBreakpoint(selectedBreakpoint.id, 'forward', mods)}
                                    >
                                        {hasChanges ? t('proxy.breakpoint_forward_modified') : t('proxy.breakpoint_forward')}
                                    </Button>
                                </Space>
                            </div>
                        );
                    })() : null
                }
                style={{ top: 32 }}
            >
                {selectedBreakpoint && breakpointEdit && (
                    <div>
                        {/* Phase indicator */}
                        <Space style={{ marginBottom: 12 }}>
                            <Tag color={selectedBreakpoint.phase === 'request' ? 'green' : 'orange'}>
                                {selectedBreakpoint.phase === 'request' ? t('proxy.breakpoint_request_phase') : t('proxy.breakpoint_response_phase')}
                            </Tag>
                        </Space>

                        {selectedBreakpoint.phase === 'request' ? (
                            /* ===== REQUEST PHASE EDITING ===== */
                            <div>
                                {/* Method + URL row */}
                                <Space.Compact style={{ width: '100%', marginBottom: 12 }}>
                                    <Select
                                        value={breakpointEdit.method}
                                        onChange={(val) => updateBreakpointEdit({ method: val })}
                                        style={{ width: 110 }}
                                        options={['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'].map(m => ({ label: m, value: m }))}
                                    />
                                    <Input
                                        value={breakpointEdit.url}
                                        onChange={(e) => updateBreakpointEdit({ url: e.target.value })}
                                        style={{ fontFamily: 'monospace', fontSize: 12 }}
                                        placeholder="URL"
                                    />
                                </Space.Compact>

                                <Tabs size="small" items={[
                                    {
                                        key: 'reqHeaders',
                                        label: t('proxy.tab_req_headers'),
                                        children: (
                                            <div style={{ maxHeight: 280, overflow: 'auto' }}>
                                                {breakpointEdit.headers.map((h: EditableHeader, i: number) => (
                                                    <Space key={i} style={{ display: 'flex', marginBottom: 4 }} align="start">
                                                        <Input
                                                            size="small"
                                                            placeholder={t('proxy.breakpoint_header_name')}
                                                            value={h.key}
                                                            onChange={(e) => {
                                                                const updated = [...breakpointEdit.headers];
                                                                updated[i] = { ...updated[i], key: e.target.value };
                                                                updateBreakpointEdit({ headers: updated });
                                                            }}
                                                            style={{ width: 200, fontFamily: 'monospace', fontSize: 12 }}
                                                        />
                                                        <Input
                                                            size="small"
                                                            placeholder={t('proxy.breakpoint_header_value')}
                                                            value={h.value}
                                                            onChange={(e) => {
                                                                const updated = [...breakpointEdit.headers];
                                                                updated[i] = { ...updated[i], value: e.target.value };
                                                                updateBreakpointEdit({ headers: updated });
                                                            }}
                                                            style={{ flex: 1, fontFamily: 'monospace', fontSize: 12 }}
                                                        />
                                                        <Button
                                                            size="small"
                                                            icon={<MinusCircleOutlined />}
                                                            onClick={() => {
                                                                const updated = breakpointEdit.headers.filter((_: EditableHeader, idx: number) => idx !== i);
                                                                updateBreakpointEdit({ headers: updated });
                                                            }}
                                                            danger
                                                        />
                                                    </Space>
                                                ))}
                                                <Button
                                                    size="small"
                                                    type="dashed"
                                                    icon={<PlusOutlined />}
                                                    onClick={() => {
                                                        updateBreakpointEdit({ headers: [...breakpointEdit.headers, { key: '', value: '' }] });
                                                    }}
                                                    style={{ marginTop: 4 }}
                                                >
                                                    {t('proxy.breakpoint_add_header')}
                                                </Button>
                                            </div>
                                        ),
                                    },
                                    {
                                        key: 'reqQuery',
                                        label: `${t('proxy.tab_req_query')}${breakpointEdit.queryParams.length > 0 ? ` (${breakpointEdit.queryParams.length})` : ''}`,
                                        children: (
                                            <div style={{ maxHeight: 280, overflow: 'auto' }}>
                                                {breakpointEdit.queryParams.map((p: EditableHeader, i: number) => (
                                                    <Space key={i} style={{ display: 'flex', marginBottom: 4 }} align="start">
                                                        <Input
                                                            size="small"
                                                            placeholder={t('proxy.breakpoint_query_name')}
                                                            value={p.key}
                                                            onChange={(e) => {
                                                                const updated = [...breakpointEdit.queryParams];
                                                                updated[i] = { ...updated[i], key: e.target.value };
                                                                updateBreakpointEdit({ queryParams: updated });
                                                            }}
                                                            style={{ width: 200, fontFamily: 'monospace', fontSize: 12 }}
                                                        />
                                                        <Input
                                                            size="small"
                                                            placeholder={t('proxy.breakpoint_query_value')}
                                                            value={p.value}
                                                            onChange={(e) => {
                                                                const updated = [...breakpointEdit.queryParams];
                                                                updated[i] = { ...updated[i], value: e.target.value };
                                                                updateBreakpointEdit({ queryParams: updated });
                                                            }}
                                                            style={{ flex: 1, fontFamily: 'monospace', fontSize: 12 }}
                                                        />
                                                        <Button
                                                            size="small"
                                                            icon={<MinusCircleOutlined />}
                                                            onClick={() => {
                                                                const updated = breakpointEdit.queryParams.filter((_: EditableHeader, idx: number) => idx !== i);
                                                                updateBreakpointEdit({ queryParams: updated });
                                                            }}
                                                            danger
                                                        />
                                                    </Space>
                                                ))}
                                                <Button
                                                    size="small"
                                                    type="dashed"
                                                    icon={<PlusOutlined />}
                                                    onClick={() => {
                                                        updateBreakpointEdit({ queryParams: [...breakpointEdit.queryParams, { key: '', value: '' }] });
                                                    }}
                                                    style={{ marginTop: 4 }}
                                                >
                                                    {t('proxy.breakpoint_add_query')}
                                                </Button>
                                            </div>
                                        ),
                                    },
                                    {
                                        key: 'reqBody',
                                        label: t('proxy.tab_req_body'),
                                        children: (
                                            <JsonEditor
                                                value={breakpointEdit.body}
                                                onChange={(val) => updateBreakpointEdit({ body: val })}
                                                height={260}
                                                language={breakpointEdit.body.trimStart().startsWith('{') || breakpointEdit.body.trimStart().startsWith('[') ? 'json' : 'plaintext'}
                                            />
                                        ),
                                    },
                                ]} />
                            </div>
                        ) : (
                            /* ===== RESPONSE PHASE EDITING ===== */
                            <div>
                                {/* URL (read-only reference) */}
                                <div style={{ marginBottom: 8, wordBreak: 'break-all', fontSize: 12, fontFamily: 'monospace', padding: '6px 12px', background: token.colorFillAlter, borderRadius: 4, color: token.colorTextSecondary }}>
                                    {selectedBreakpoint.method} {selectedBreakpoint.url}
                                </div>

                                {/* Status Code */}
                                <Space style={{ marginBottom: 12 }}>
                                    <Text style={{ fontSize: 13 }}>{t('proxy.breakpoint_status_code')}:</Text>
                                    <InputNumber
                                        size="small"
                                        value={breakpointEdit.statusCode}
                                        onChange={(val) => updateBreakpointEdit({ statusCode: val || 200 })}
                                        min={100}
                                        max={599}
                                        style={{ width: 100 }}
                                    />
                                </Space>

                                <Tabs size="small" items={[
                                    {
                                        key: 'respHeaders',
                                        label: t('proxy.tab_resp_headers'),
                                        children: (
                                            <div style={{ maxHeight: 240, overflow: 'auto' }}>
                                                {breakpointEdit.respHeaders.map((h: EditableHeader, i: number) => (
                                                    <Space key={i} style={{ display: 'flex', marginBottom: 4 }} align="start">
                                                        <Input
                                                            size="small"
                                                            placeholder={t('proxy.breakpoint_header_name')}
                                                            value={h.key}
                                                            onChange={(e) => {
                                                                const updated = [...breakpointEdit.respHeaders];
                                                                updated[i] = { ...updated[i], key: e.target.value };
                                                                updateBreakpointEdit({ respHeaders: updated });
                                                            }}
                                                            style={{ width: 200, fontFamily: 'monospace', fontSize: 12 }}
                                                        />
                                                        <Input
                                                            size="small"
                                                            placeholder={t('proxy.breakpoint_header_value')}
                                                            value={h.value}
                                                            onChange={(e) => {
                                                                const updated = [...breakpointEdit.respHeaders];
                                                                updated[i] = { ...updated[i], value: e.target.value };
                                                                updateBreakpointEdit({ respHeaders: updated });
                                                            }}
                                                            style={{ flex: 1, fontFamily: 'monospace', fontSize: 12 }}
                                                        />
                                                        <Button
                                                            size="small"
                                                            icon={<MinusCircleOutlined />}
                                                            onClick={() => {
                                                                const updated = breakpointEdit.respHeaders.filter((_: EditableHeader, idx: number) => idx !== i);
                                                                updateBreakpointEdit({ respHeaders: updated });
                                                            }}
                                                            danger
                                                        />
                                                    </Space>
                                                ))}
                                                <Button
                                                    size="small"
                                                    type="dashed"
                                                    icon={<PlusOutlined />}
                                                    onClick={() => {
                                                        updateBreakpointEdit({ respHeaders: [...breakpointEdit.respHeaders, { key: '', value: '' }] });
                                                    }}
                                                    style={{ marginTop: 4 }}
                                                >
                                                    {t('proxy.breakpoint_add_header')}
                                                </Button>
                                            </div>
                                        ),
                                    },
                                    {
                                        key: 'respBody',
                                        label: t('proxy.tab_resp_body'),
                                        children: (
                                            <JsonEditor
                                                value={breakpointEdit.respBody}
                                                onChange={(val) => updateBreakpointEdit({ respBody: val })}
                                                height={260}
                                                language={breakpointEdit.respBody.trimStart().startsWith('{') || breakpointEdit.respBody.trimStart().startsWith('[') ? 'json' : 'plaintext'}
                                            />
                                        ),
                                    },
                                    {
                                        key: 'originalReq',
                                        label: t('proxy.breakpoint_original_request'),
                                        children: (
                                            <div style={{ maxHeight: 280, overflow: 'auto', fontSize: 12, fontFamily: 'monospace' }}>
                                                {selectedBreakpoint.headers && Object.entries(selectedBreakpoint.headers).map(([k, v]) => (
                                                    <div key={k}><strong>{k}:</strong> {Array.isArray(v) ? v.join(', ') : v}</div>
                                                ))}
                                                {selectedBreakpoint.body && (
                                                    <div style={{ marginTop: 8, padding: '8px', background: token.colorFillAlter, borderRadius: 4 }}>
                                                        <JsonViewer data={selectedBreakpoint.body} />
                                                    </div>
                                                )}
                                            </div>
                                        ),
                                    },
                                ]} />
                            </div>
                        )}
                    </div>
                )}
            </Modal>

            {/* === Map Remote Rules List Modal === */}
            <Modal
                open={mapRemoteListModalOpen}
                onCancel={closeMapRemoteListModal}
                title={t('proxy.map_remote_rules')}
                width={700}
                footer={null}
                style={{ top: 32 }}
            >
                <div style={{ marginBottom: 12 }}>
                    <Button type="primary" icon={<PlusOutlined />} onClick={() => { closeMapRemoteListModal(); mapRemoteForm.resetFields(); openMapRemoteEditModal(null); }}>
                        {t('proxy.add_map_remote_rule')}
                    </Button>
                </div>
                {mapRemoteRules.length === 0 ? (
                    <Text type="secondary" style={{ display: 'block', textAlign: 'center', padding: 32 }}>{t('proxy.no_map_remote_rules')}</Text>
                ) : (
                    <Table
                        dataSource={mapRemoteRules}
                        rowKey="id"
                        size="small"
                        pagination={false}
                        columns={[
                            { title: t('proxy.map_remote_source'), dataIndex: 'sourcePattern', key: 'sourcePattern', ellipsis: true },
                            { title: t('proxy.map_remote_target'), dataIndex: 'targetURL', key: 'targetURL', ellipsis: true },
                            { title: t('proxy.breakpoint_method'), dataIndex: 'method', key: 'method', width: 80, render: (v: string) => v || '*' },
                            { title: t('proxy.breakpoint_enabled'), key: 'enabled', width: 70, render: (_: any, record: any) => (
                                <Switch size="small" checked={record.enabled} onChange={(checked) => handleToggleMapRemoteRule(record.id, checked)} />
                            )},
                            { key: 'actions', width: 80, render: (_: any, record: any) => (
                                <Space size={4}>
                                    <Button type="link" size="small" icon={<EditOutlined />} onClick={() => {
                                        closeMapRemoteListModal();
                                        mapRemoteForm.setFieldsValue({
                                            sourcePattern: record.sourcePattern,
                                            targetURL: record.targetURL,
                                            method: record.method,
                                            description: record.description,
                                        });
                                        openMapRemoteEditModal(record);
                                    }} />
                                    <Popconfirm title="Delete?" onConfirm={() => handleDeleteMapRemoteRule(record.id)}>
                                        <Button type="link" size="small" danger icon={<DeleteOutlined />} />
                                    </Popconfirm>
                                </Space>
                            )},
                        ]}
                    />
                )}
            </Modal>

            {/* === Map Remote Rule Edit Modal === */}
            <Modal
                open={mapRemoteEditModalOpen}
                onCancel={closeMapRemoteEditModal}
                title={editingMapRemoteRule ? t('proxy.edit_map_remote_rule') : t('proxy.add_map_remote_rule')}
                width={500}
                onOk={handleSaveMapRemoteRule}
                okText={editingMapRemoteRule ? t('proxy.edit_map_remote_rule') : t('proxy.add_map_remote_rule')}
            >
                <Form form={mapRemoteForm} layout="vertical">
                    <Form.Item name="sourcePattern" label={t('proxy.map_remote_source')} rules={[{ required: true }]}>
                        <AutoComplete
                            placeholder="*/api/v1/*"
                            options={getUrlPatternsForMethod(mapRemoteFormMethod)}
                            filterOption={(inputValue, option) =>
                                (option?.value as string)?.toLowerCase().includes(inputValue.toLowerCase())
                            }
                        />
                    </Form.Item>
                    <Form.Item name="targetURL" label={t('proxy.map_remote_target')} rules={[{ required: true }]}>
                        <Input placeholder="http://localhost:3000/api/v1/*" />
                    </Form.Item>
                    <Form.Item name="method" label={t('proxy.breakpoint_method')}>
                        <Select allowClear placeholder="All" options={METHOD_SELECT_OPTIONS} />
                    </Form.Item>
                    <Form.Item name="description" label={t('proxy.breakpoint_description')}>
                        <Input placeholder="Optional description" />
                    </Form.Item>
                </Form>
            </Modal>

            {/* === Rewrite Rules List Modal === */}
            <Modal
                title={t('proxy.rewrite_rules')}
                open={rewriteListModalOpen}
                onCancel={closeRewriteListModal}
                width={900}
                footer={null}
            >
                <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
                    <Button type="primary" icon={<PlusOutlined />} onClick={() => { closeRewriteListModal(); rewriteForm.resetFields(); openRewriteEditModal(null); }}>
                        {t('proxy.add_rewrite_rule')}
                    </Button>
                </div>
                {rewriteRules.length === 0 ? (
                    <Text type="secondary" style={{ display: 'block', textAlign: 'center', padding: 32 }}>{t('proxy.no_rewrite_rules')}</Text>
                ) : (
                    <Table
                        dataSource={rewriteRules}
                        rowKey="id"
                        size="small"
                        pagination={false}
                        scroll={{ y: 400 }}
                        columns={[
                            { title: t('proxy.url_pattern'), dataIndex: 'urlPattern', key: 'urlPattern', ellipsis: true, width: 160 },
                            { title: t('proxy.rewrite_phase'), dataIndex: 'phase', key: 'phase', width: 80 },
                            { title: t('proxy.rewrite_target'), dataIndex: 'target', key: 'target', width: 70 },
                            { title: t('proxy.rewrite_match'), dataIndex: 'match', key: 'match', ellipsis: true, width: 140 },
                            { title: t('proxy.rewrite_replace'), dataIndex: 'replace', key: 'replace', ellipsis: true, width: 140 },
                            { title: t('proxy.description'), dataIndex: 'description', key: 'description', ellipsis: true },
                            { title: t('proxy.mock_enabled'), key: 'enabled', width: 70, render: (_: any, record: any) => (
                                <Switch size="small" checked={record.enabled} onChange={(checked) => handleToggleRewriteRule(record.id, checked)} />
                            )},
                            { title: '', key: 'actions', width: 70, render: (_: any, record: any) => (
                                <Space size={4}>
                                    <Button type="link" size="small" icon={<EditOutlined />} onClick={() => startEditRewriteRule(record)} />
                                    <Popconfirm title="Delete?" onConfirm={() => handleDeleteRewriteRule(record.id)}>
                                        <Button type="link" size="small" danger icon={<DeleteOutlined />} />
                                    </Popconfirm>
                                </Space>
                            )},
                        ]}
                    />
                )}
            </Modal>

            {/* === Rewrite Rule Edit Modal === */}
            <Modal
                open={rewriteEditModalOpen}
                onCancel={() => { closeRewriteEditModal(); rewriteForm.resetFields(); }}
                title={editingRewriteRule ? t('proxy.edit_rewrite_rule') : t('proxy.add_rewrite_rule')}
                width={600}
                onOk={handleSaveRewriteRule}
                okText={editingRewriteRule ? t('common.save') : t('proxy.add_rewrite_rule')}
            >
                <Form form={rewriteForm} layout="vertical" size="small">
                    <Form.Item name="urlPattern" label={t('proxy.url_pattern')} rules={[{ required: true }]}>
                        <AutoComplete
                            placeholder="*/api/*"
                            options={getUrlPatternsForMethod(rewriteFormMethod)}
                            filterOption={(inputValue, option) =>
                                (option?.value as string)?.toLowerCase().includes(inputValue.toLowerCase())
                            }
                        />
                    </Form.Item>
                    <Space wrap style={{ width: '100%' }}>
                        <Form.Item name="method" label={t('proxy.col_method')} style={{ width: 110 }}>
                            <Select allowClear placeholder="All" options={METHOD_SELECT_OPTIONS} />
                        </Form.Item>
                        <Form.Item name="phase" label={t('proxy.rewrite_phase')} rules={[{ required: true }]} style={{ width: 130 }}>
                            <Select placeholder="Phase">
                                <Select.Option value="request">{t('proxy.phase_request')}</Select.Option>
                                <Select.Option value="response">{t('proxy.phase_response')}</Select.Option>
                                <Select.Option value="both">{t('proxy.phase_both')}</Select.Option>
                            </Select>
                        </Form.Item>
                        <Form.Item name="target" label={t('proxy.rewrite_target')} rules={[{ required: true }]} style={{ width: 130 }}>
                            <Select placeholder="Target">
                                <Select.Option value="body">{t('proxy.rewrite_target_body')}</Select.Option>
                                <Select.Option value="header">{t('proxy.rewrite_target_header')}</Select.Option>
                            </Select>
                        </Form.Item>
                    </Space>
                    <Form.Item noStyle shouldUpdate={(prev, cur) => prev.target !== cur.target}>
                        {({ getFieldValue }) => getFieldValue('target') === 'header' ? (
                            <Form.Item name="headerName" label={t('proxy.rewrite_header_name')} rules={[{ required: true }]}>
                                <Input placeholder="Content-Type" />
                            </Form.Item>
                        ) : null}
                    </Form.Item>
                    <Form.Item name="match" label={t('proxy.rewrite_match')} rules={[{ required: true }]}
                        tooltip={t('proxy.rewrite_match_tooltip')}
                    >
                        <Input placeholder="old-value|regex-pattern" style={{ fontFamily: 'monospace' }} />
                    </Form.Item>
                    <Form.Item name="replace" label={t('proxy.rewrite_replace')}
                        tooltip={t('proxy.rewrite_replace_tooltip')}
                    >
                        <Input placeholder="new-value ($1 for groups)" style={{ fontFamily: 'monospace' }} />
                    </Form.Item>
                    <Form.Item name="description" label={t('proxy.description')}>
                        <Input placeholder={t('proxy.description')} />
                    </Form.Item>
                </Form>
            </Modal>

            {/* Diff Modal */}
            <Modal
                open={diffModalOpen}
                onCancel={closeDiffModal}
                title={t('proxy.diff_title')}
                width={1100}
                footer={null}
                styles={{ body: { maxHeight: '70vh', overflow: 'auto' } }}
            >
                {diffBaseLog && diffTargetLog && (() => {
                    const formatHeaders = (headers?: Record<string, string[]>) => {
                        if (!headers) return '';
                        return Object.entries(headers).map(([k, v]) => `${k}: ${(v || []).join(', ')}`).sort().join('\n');
                    };
                    const formatBody = (body?: string) => {
                        if (!body) return '';
                        try { return JSON.stringify(JSON.parse(body), null, 2); } catch { return body; }
                    };
                    const parseQuery = (url: string) => {
                        try {
                            const u = new URL(url);
                            return Array.from(u.searchParams.entries()).map(([k, v]) => `${k}=${v}`).join('\n');
                        } catch { return ''; }
                    };

                    type DiffLine = { left: string; right: string; type: 'same' | 'changed' | 'added' | 'removed' };

                    // Text line diff (for body content)
                    const diffLines = (a: string, b: string): DiffLine[] => {
                        const linesA = a.split('\n');
                        const linesB = b.split('\n');
                        const result: DiffLine[] = [];
                        const maxLen = Math.max(linesA.length, linesB.length);
                        for (let i = 0; i < maxLen; i++) {
                            const la = i < linesA.length ? linesA[i] : undefined;
                            const lb = i < linesB.length ? linesB[i] : undefined;
                            if (la !== undefined && lb !== undefined) {
                                result.push({ left: la, right: lb, type: la === lb ? 'same' : 'changed' });
                            } else if (la !== undefined) {
                                result.push({ left: la, right: '', type: 'removed' });
                            } else {
                                result.push({ left: '', right: lb!, type: 'added' });
                            }
                        }
                        return result;
                    };

                    // Key-value diff: align rows by key so same keys are on the same row
                    const diffKV = (a: string, b: string, separator: string): DiffLine[] => {
                        const parseLines = (s: string) =>
                            s.split('\n').filter(Boolean).map(line => {
                                const idx = line.indexOf(separator);
                                return { key: idx >= 0 ? line.substring(0, idx).trim() : line.trim(), line };
                            });

                        const linesA = parseLines(a);
                        const linesB = parseLines(b);

                        // Build multimap: key → [fullLine, ...] (preserves duplicate keys)
                        const buildMap = (lines: Array<{ key: string; line: string }>) => {
                            const map = new Map<string, string[]>();
                            for (const { key, line } of lines) {
                                if (!map.has(key)) map.set(key, []);
                                map.get(key)!.push(line);
                            }
                            return map;
                        };

                        const mapA = buildMap(linesA);
                        const mapB = buildMap(linesB);

                        // Collect all unique keys preserving first-appearance order
                        const allKeys: string[] = [];
                        const seen = new Set<string>();
                        for (const { key } of [...linesA, ...linesB]) {
                            if (!seen.has(key)) { seen.add(key); allKeys.push(key); }
                        }

                        const result: DiffLine[] = [];
                        for (const key of allKeys) {
                            const valsA = mapA.get(key) || [];
                            const valsB = mapB.get(key) || [];
                            const maxLen = Math.max(valsA.length, valsB.length);
                            for (let i = 0; i < maxLen; i++) {
                                const la = i < valsA.length ? valsA[i] : undefined;
                                const lb = i < valsB.length ? valsB[i] : undefined;
                                if (la !== undefined && lb !== undefined) {
                                    result.push({ left: la, right: lb, type: la === lb ? 'same' : 'changed' });
                                } else if (la !== undefined) {
                                    result.push({ left: la, right: '', type: 'removed' });
                                } else {
                                    result.push({ left: '', right: lb!, type: 'added' });
                                }
                            }
                        }
                        return result;
                    };

                    // mode: 'query' = align by query key (sep '='), 'header' = align by header name (sep ': '), 'text' = line diff
                    const DiffBlock = ({ title, leftVal, rightVal, mode = 'text' }: { title: string; leftVal: string; rightVal: string; mode?: 'query' | 'header' | 'text' }) => {
                        if (!leftVal && !rightVal) return null;
                        const lines = mode === 'query'
                            ? diffKV(leftVal, rightVal, '=')
                            : mode === 'header'
                                ? diffKV(leftVal, rightVal, ': ')
                                : diffLines(leftVal, rightVal);
                        const hasDiff = lines.some(l => l.type !== 'same');
                        return (
                            <div style={{ marginBottom: 16 }}>
                                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                                    <Text strong style={{ fontSize: 13 }}>{title}</Text>
                                    {!hasDiff && <Tag color="green" style={{ fontSize: 11 }}>{t('proxy.diff_identical')}</Tag>}
                                    {hasDiff && <Tag color="orange" style={{ fontSize: 11 }}>{t('proxy.diff_different')}</Tag>}
                                </div>
                                {hasDiff && (
                                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2, fontFamily: 'monospace', fontSize: 12, border: `1px solid ${token.colorBorderSecondary}`, borderRadius: 4, overflow: 'hidden' }}>
                                        {lines.map((line, idx) => (
                                            <React.Fragment key={idx}>
                                                <div style={{
                                                    padding: '2px 8px',
                                                    background: line.type === 'removed' ? 'rgba(255,77,79,0.12)' : line.type === 'changed' ? 'rgba(250,173,20,0.08)' : line.type === 'added' ? 'transparent' : token.colorFillQuaternary,
                                                    borderBottom: idx < lines.length - 1 ? `1px solid ${token.colorBorderSecondary}` : 'none',
                                                    whiteSpace: 'pre-wrap',
                                                    wordBreak: 'break-all',
                                                    minHeight: 22,
                                                }}>{line.left}</div>
                                                <div style={{
                                                    padding: '2px 8px',
                                                    background: line.type === 'added' ? 'rgba(82,196,26,0.12)' : line.type === 'changed' ? 'rgba(250,173,20,0.08)' : line.type === 'removed' ? 'transparent' : token.colorFillQuaternary,
                                                    borderBottom: idx < lines.length - 1 ? `1px solid ${token.colorBorderSecondary}` : 'none',
                                                    borderLeft: `1px solid ${token.colorBorderSecondary}`,
                                                    whiteSpace: 'pre-wrap',
                                                    wordBreak: 'break-all',
                                                    minHeight: 22,
                                                }}>{line.right}</div>
                                            </React.Fragment>
                                        ))}
                                    </div>
                                )}
                            </div>
                        );
                    };

                    return (
                        <div>
                            {/* Column headers */}
                            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, marginBottom: 16 }}>
                                <div style={{ background: token.colorFillTertiary, padding: '8px 12px', borderRadius: 6 }}>
                                    <Text type="secondary" style={{ fontSize: 11 }}>{t('proxy.diff_request_a')}</Text>
                                    <div style={{ fontFamily: 'monospace', fontSize: 12, marginTop: 4 }}>
                                        <Tag style={{ fontSize: 11, color: getMethodColor(diffBaseLog.method), borderColor: getMethodColor(diffBaseLog.method), background: `${getMethodColor(diffBaseLog.method)}10` }}>{diffBaseLog.method}</Tag>
                                        {diffBaseLog.statusCode && <Tag style={{ fontSize: 11 }}>{diffBaseLog.statusCode}</Tag>}
                                        <div style={{ marginTop: 4, wordBreak: 'break-all', color: token.colorTextSecondary }}>{diffBaseLog.url}</div>
                                    </div>
                                </div>
                                <div style={{ background: token.colorFillTertiary, padding: '8px 12px', borderRadius: 6 }}>
                                    <Text type="secondary" style={{ fontSize: 11 }}>{t('proxy.diff_request_b')}</Text>
                                    <div style={{ fontFamily: 'monospace', fontSize: 12, marginTop: 4 }}>
                                        <Tag style={{ fontSize: 11, color: getMethodColor(diffTargetLog.method), borderColor: getMethodColor(diffTargetLog.method), background: `${getMethodColor(diffTargetLog.method)}10` }}>{diffTargetLog.method}</Tag>
                                        {diffTargetLog.statusCode && <Tag style={{ fontSize: 11 }}>{diffTargetLog.statusCode}</Tag>}
                                        <div style={{ marginTop: 4, wordBreak: 'break-all', color: token.colorTextSecondary }}>{diffTargetLog.url}</div>
                                    </div>
                                </div>
                            </div>
                            <DiffBlock title={t('proxy.diff_query_params')} leftVal={parseQuery(diffBaseLog.url)} rightVal={parseQuery(diffTargetLog.url)} mode="query" />
                            <DiffBlock title={t('proxy.diff_req_headers')} leftVal={formatHeaders(diffBaseLog.headers)} rightVal={formatHeaders(diffTargetLog.headers)} mode="header" />
                            <DiffBlock title={t('proxy.diff_req_body')} leftVal={formatBody(diffBaseLog.body)} rightVal={formatBody(diffTargetLog.body)} />
                            <DiffBlock title={t('proxy.diff_resp_headers')} leftVal={formatHeaders(diffBaseLog.respHeaders)} rightVal={formatHeaders(diffTargetLog.respHeaders)} mode="header" />
                            <DiffBlock title={t('proxy.diff_resp_body')} leftVal={formatBody(diffBaseLog.respBody)} rightVal={formatBody(diffTargetLog.respBody)} />
                        </div>
                    );
                })()}
            </Modal>

        </div>
    );
};

export default ProxyView;
