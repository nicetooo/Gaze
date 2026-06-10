import React, { useEffect, useCallback, useMemo, useRef } from "react";
import { useShallow } from 'zustand/react/shallow';
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  Button,
  Space,
  Card,
  Modal,
  Input,
  message,
  theme,
  Empty,
  Popconfirm,
  Tooltip,
  Tag,
  Form,
  Select,
  InputNumber,
  Collapse,
  Divider,
  Drawer,
  Typography,
  AutoComplete,
  Progress
} from "antd";
import { useTranslation } from "react-i18next";
import {
  StopOutlined,
  DeleteOutlined,
  PlusOutlined,
  EditOutlined,
  RobotOutlined,
  AimOutlined,
  CloseOutlined,
  ClockCircleOutlined,
  PlayCircleOutlined,
  CheckCircleOutlined,
  SwapOutlined,
  FormOutlined,
  BranchesOutlined,
  ReloadOutlined,
  PauseCircleOutlined,
  CaretRightOutlined,
  CopyOutlined,
  SaveOutlined,
  ArrowLeftOutlined,
  HomeOutlined,
  AppstoreOutlined,
  PoweroffOutlined,
  SoundOutlined,
  ExpandOutlined,
  LockOutlined,
  ForkOutlined,
  PicCenterOutlined,
  PicRightOutlined,
  LoadingOutlined,
  SettingOutlined,
  IdcardOutlined,
  ThunderboltOutlined,
  StepForwardOutlined,
  UnorderedListOutlined,
  PartitionOutlined as CanvasOutlined,
} from "@ant-design/icons";
import {
  ReactFlow,
  MiniMap,
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  addEdge,
  Handle,
  Position,
  BackgroundVariant,
  MarkerType,
  ConnectionLineType,
  SelectionMode,
  Panel,
  getSmoothStepPath,
  BaseEdge,
  EdgeLabelRenderer,
} from '@xyflow/react';
import type { Connection, Edge, Node, NodeChange, EdgeProps } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import dagre from 'dagre';


import DeviceSelector from "./DeviceSelector";
import ElementPicker, { ElementSelector } from "./ElementPicker";
import WorkflowStepListView from "./WorkflowStepListView";
import { useDeviceStore, useAutomationStore, useWorkflowStore } from "../stores";
import type { WorkflowViewMode } from "../stores/workflowStore";
import type { Workflow, WorkflowStep } from "../types/workflow";

const { Text, Title } = Typography;

// Step type definitions
const STEP_TYPES = {
  COORDINATE_ACTIONS: [
    { key: 'tap', icon: <AimOutlined />, color: 'magenta' },
    { key: 'swipe', icon: <SwapOutlined />, color: 'magenta' },
  ],
  ELEMENT_ACTIONS: [
    { key: 'click_element', icon: <AimOutlined />, color: 'blue' },
    { key: 'long_click_element', icon: <AimOutlined />, color: 'blue' },
    { key: 'input_text', icon: <FormOutlined />, color: 'cyan' },
    { key: 'swipe_element', icon: <SwapOutlined />, color: 'purple' },
  ],
  WAIT_CONDITIONS: [
    { key: 'wait_element', icon: <ClockCircleOutlined />, color: 'orange' },
    { key: 'wait_gone', icon: <ClockCircleOutlined />, color: 'orange' },
    { key: 'wait', icon: <ClockCircleOutlined />, color: 'default' },
  ],
  FLOW_CONTROL: [
    { key: 'start', icon: <CaretRightOutlined />, color: 'green' },
    { key: 'branch', icon: <ForkOutlined />, color: 'purple' },
    { key: 'scroll_to', icon: <ReloadOutlined />, color: 'magenta' },
    { key: 'assert_element', icon: <CheckCircleOutlined />, color: 'lime' },
    { key: 'set_variable', icon: <IdcardOutlined />, color: 'orange' },
    { key: 'read_to_variable', icon: <FormOutlined />, color: 'cyan' },
  ],
  SCRIPT_ACTIONS: [
    { key: 'script', icon: <PlayCircleOutlined />, color: 'geekblue' },
    { key: 'adb', icon: <RobotOutlined />, color: 'volcano' },
  ],
  SYSTEM_ACTIONS: [
    { key: 'key_back', icon: <ArrowLeftOutlined />, color: 'default' },
    { key: 'key_home', icon: <HomeOutlined />, color: 'default' },
    { key: 'key_recent', icon: <AppstoreOutlined />, color: 'default' },
    { key: 'key_power', icon: <PoweroffOutlined />, color: 'red' },
    { key: 'key_volume_up', icon: <SoundOutlined />, color: 'default' },
    { key: 'key_volume_down', icon: <SoundOutlined />, color: 'default' },
    { key: 'screen_on', icon: <ExpandOutlined />, color: 'default' },
    { key: 'screen_off', icon: <LockOutlined />, color: 'default' },
  ],
  NESTED: [
    { key: 'run_workflow', icon: <BranchesOutlined />, color: 'gold' },
  ],
  APP_ACTIONS: [
    { key: 'launch_app', icon: <PlayCircleOutlined />, color: 'green' },
    { key: 'stop_app', icon: <StopOutlined />, color: 'red' },
    { key: 'clear_app', icon: <DeleteOutlined />, color: 'orange' },
    { key: 'open_settings', icon: <SettingOutlined />, color: 'blue' },
  ],
  SESSION_CONTROL: [
    { key: 'start_session', icon: <PlayCircleOutlined />, color: 'cyan' },
    { key: 'end_session', icon: <StopOutlined />, color: 'volcano' },
  ],
};

const getStepTypeInfo = (type: string) => {
  for (const category of Object.values(STEP_TYPES)) {
    const found = category.find(s => s.key === type);
    if (found) return found;
  }
  return { key: type, icon: <RobotOutlined />, color: 'default' };
};

// Helper function to check if a targetHandle value is valid
const isValidTargetHandle = (handle: any): boolean => {
  if (!handle) return false;
  const handleStr = String(handle);
  return handleStr !== 'default' && handleStr !== 'null' && handleStr.trim() !== '';
};


// Workflow types
// Workflow types are now imported from stores/workflowStore


// Custom Node Component
const WorkflowNode = React.memo(({ data, selected }: any) => {
  const { t } = useTranslation();
  const { step, isRunning, isCurrent, isWaiting, waitingPhase, onExecuteStep, canExecute } = data;
  const { token } = theme.useToken();
  const typeInfo = getStepTypeInfo(step.type);
  const isBranch = step.type === 'branch';
  const isStart = step.type === 'start';

  const handleExecuteClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (onExecuteStep && !isStart) {
      onExecuteStep(step);
    }
  };

  return (
    <div style={{ position: 'relative' }}>
      {/* Pre-Wait Tag */}
      {isWaiting && waitingPhase === 'pre' && (
        <div style={{
          position: 'absolute',
          top: -24,
          left: '50%',
          transform: 'translateX(-50%)',
          zIndex: 10,
        }}>
          <Tag color="warning" icon={<LoadingOutlined />} style={{ margin: 0 }}>
            {t("workflow.pre_wait")}
          </Tag>
        </div>
      )}

      {/* Post-Wait Tag */}
      {isWaiting && waitingPhase === 'post' && (
        <div style={{
          position: 'absolute',
          bottom: -24,
          left: '50%',
          transform: 'translateX(-50%)',
          zIndex: 10,
        }}>
          <Tag color="processing" icon={<LoadingOutlined />} style={{ margin: 0 }}>
            {t("workflow.post_delay")}
          </Tag>
        </div>
      )}
      {/* Start node has no input handle */}
      {!isStart && (
        <>
          <Handle type="target" position={Position.Top} id="default" style={{ background: token.colorTextSecondary }} />
          <Handle type="target" position={Position.Left} id="target-left" style={{ background: token.colorTextSecondary }} />
          <Handle type="target" position={Position.Right} id="target-right" style={{ background: token.colorTextSecondary }} />
        </>
      )}
      <Card
        size="small"
        style={{
          width: 280,
          borderColor: isCurrent ? token.colorPrimary : selected ? token.colorPrimary : token.colorBorder,
          borderWidth: isCurrent || selected ? 2 : 1,
          boxShadow: isCurrent ? `0 0 10px ${token.colorPrimary}` : selected ? token.boxShadowSecondary : 'none',
          backgroundColor: isCurrent ? token.colorPrimaryBg : token.colorBgContainer, // Ensure container background
          color: token.colorText, // Explicit text color
        }}
        styles={{ body: { padding: '8px 12px' } }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <div style={{
            backgroundColor: isCurrent ? token.colorPrimaryBgHover : token.colorBgLayout, // Softer background for icon
            padding: 6,
            borderRadius: 6,
            display: 'flex',
            color: typeInfo.color === 'default' ? token.colorText : typeInfo.color
          }}>
            {typeInfo.icon}
          </div>
          <div style={{ flex: 1, overflow: 'hidden' }}>
            <Text strong ellipsis style={{ color: token.colorText }}>{data.label || step.type}</Text>
            {/* Element selector display (V2: step.element?.selector) */}
            {step.element?.selector && (
              <div style={{ fontSize: 11, color: token.colorTextSecondary, display: 'flex', alignItems: 'center', gap: 4 }}>
                <Tag style={{ margin: 0, fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>{step.element.selector.type}</Tag>
                <Text ellipsis style={{ fontSize: 11, maxWidth: 120, color: token.colorTextSecondary }}>{step.element.selector.value}</Text>
              </div>
            )}
            {/* Coordinate display for tap (V2: step.tap) */}
            {step.type === 'tap' && step.tap && (
              <div style={{ fontSize: 11, color: token.colorTextSecondary }}>
                <Text style={{ fontSize: 11, color: token.colorTextSecondary }}>
                  ({step.tap.x}, {step.tap.y})
                </Text>
              </div>
            )}
            {/* Coordinate display for swipe (V2: step.swipe) */}
            {step.type === 'swipe' && step.swipe && (
              <div style={{ fontSize: 11, color: token.colorTextSecondary }}>
                <Text style={{ fontSize: 11, color: token.colorTextSecondary }}>
                  {step.swipe.direction ? 
                    `${step.swipe.direction} ${step.swipe.distance || ''}px` :
                    `(${step.swipe.x}, ${step.swipe.y}) → (${step.swipe.x2}, ${step.swipe.y2})`
                  }
                </Text>
              </div>
            )}
            {/* App package display (V2: step.app) */}
            {step.app?.packageName && (
              <div style={{ fontSize: 11, color: token.colorTextSecondary }}>
                <Text ellipsis style={{ fontSize: 11, color: token.colorTextSecondary }}>{step.app.packageName}</Text>
              </div>
            )}
            {/* Wait duration display (V2: step.wait) */}
            {step.wait?.durationMs && (
              <div style={{ fontSize: 11, color: token.colorTextSecondary }}>
                <Text style={{ fontSize: 11, color: token.colorTextSecondary }}>{step.wait.durationMs}ms</Text>
              </div>
            )}
            {/* Script/ADB display (V2: step.script/step.adb) */}
            {step.script?.scriptName && (
              <div style={{ fontSize: 11, color: token.colorTextSecondary }}>
                <Text ellipsis style={{ fontSize: 11, color: token.colorTextSecondary }}>{step.script.scriptName}</Text>
              </div>
            )}
            {step.adb?.command && (
              <div style={{ fontSize: 11, color: token.colorTextSecondary }}>
                <Text ellipsis style={{ fontSize: 11, color: token.colorTextSecondary }}>{step.adb.command}</Text>
              </div>
            )}
            {/* ReadToVariable display */}
            {step.readToVariable && (
              <div style={{ fontSize: 11, color: token.colorTextSecondary, display: 'flex', alignItems: 'center', gap: 4 }}>
                <Tag style={{ margin: 0, fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>{step.readToVariable.selector?.type}</Tag>
                <Text ellipsis style={{ fontSize: 11, maxWidth: 80, color: token.colorTextSecondary }}>{step.readToVariable.selector?.value}</Text>
                <span style={{ color: token.colorPrimary }}>→ {step.readToVariable.variableName}</span>
              </div>
            )}
            {/* Session config display */}
            {step.session && step.type === 'start_session' && (
              <div style={{ fontSize: 11, display: 'flex', gap: 4, flexWrap: 'wrap', marginTop: 2 }}>
                {step.session.sessionName && (
                  <Text ellipsis style={{ fontSize: 11, color: token.colorTextSecondary, maxWidth: 150 }}>{step.session.sessionName}</Text>
                )}
                {step.session.logcatEnabled && <Tag color="blue" style={{ margin: 0, fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>Logcat</Tag>}
                {step.session.recordingEnabled && <Tag color="red" style={{ margin: 0, fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>Recording</Tag>}
                {step.session.proxyEnabled && <Tag color="orange" style={{ margin: 0, fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>Proxy</Tag>}
                {step.session.monitorEnabled && <Tag color="green" style={{ margin: 0, fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>Monitor</Tag>}
              </div>
            )}
            {step.session?.status && step.type === 'end_session' && (
              <div style={{ fontSize: 11, marginTop: 2 }}>
                <Tag color={step.session.status === 'completed' ? 'success' : step.session.status === 'error' ? 'error' : 'warning'} 
                     style={{ margin: 0, fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>
                  {step.session.status}
                </Tag>
              </div>
            )}
          </div>
          {/* Execute Button */}
          {!isStart && (
            <Tooltip title={t("workflow.execute_step")}>
              <Button
                type="text"
                size="small"
                icon={<ThunderboltOutlined />}
                onClick={handleExecuteClick}
                disabled={!canExecute}
                style={{ padding: '0 4px', minWidth: 24, color: token.colorPrimary }}
              />
            </Tooltip>
          )}
        </div>
      </Card>

      {isBranch ? (
        <>
          {/* Branch node: True/False/Error outputs */}
          <div style={{ position: 'absolute', bottom: -14, left: '20%', fontSize: 9, color: token.colorSuccess, fontWeight: 'bold' }}>{t('workflow.branch_true')}</div>
          <Handle
            type="source"
            position={Position.Bottom}
            id="true"
            style={{ left: '25%', background: token.colorSuccess }}
          />
          <Handle
            type="source"
            position={Position.Left}
            id="true-left"
            style={{ top: '50%', background: token.colorSuccess }}
          />

          <div style={{ position: 'absolute', bottom: -14, left: '45%', fontSize: 9, color: token.colorError, fontWeight: 'bold' }}>{t('workflow.branch_false')}</div>
          <Handle
            type="source"
            position={Position.Bottom}
            id="false"
            style={{ left: '50%', background: token.colorError }}
          />

          <div style={{ position: 'absolute', bottom: -14, right: '20%', fontSize: 9, color: token.colorWarning, fontWeight: 'bold' }}>{t('workflow.error') || 'Error'}</div>
          <Handle
            type="source"
            position={Position.Bottom}
            id="error"
            style={{ left: '75%', background: token.colorWarning }}
          />
          <Handle
            type="source"
            position={Position.Right}
            id="error-right"
            style={{ top: '70%', background: token.colorWarning }}
          />
        </>
      ) : (
        <>
          {/* Normal nodes: Success/Error outputs */}
          {!isStart && (
            <>
              <div style={{ position: 'absolute', bottom: -14, left: '25%', fontSize: 9, color: token.colorSuccess, fontWeight: 'bold' }}>{t('workflow.success') || 'Success'}</div>
              <div style={{ position: 'absolute', bottom: -14, right: '25%', fontSize: 9, color: token.colorWarning, fontWeight: 'bold' }}>{t('workflow.error') || 'Error'}</div>
            </>
          )}
          <Handle 
            type="source" 
            position={Position.Bottom} 
            id="success" 
            style={{ left: isStart ? '50%' : '30%', background: isStart ? token.colorTextSecondary : token.colorSuccess }} 
          />
          {!isStart && (
            <>
              <Handle 
                type="source" 
                position={Position.Bottom} 
                id="error" 
                style={{ left: '70%', background: token.colorWarning }} 
              />
              <Handle type="source" position={Position.Left} id="success-left" style={{ top: '50%', background: token.colorSuccess }} />
              <Handle type="source" position={Position.Right} id="error-right" style={{ top: '70%', background: token.colorWarning }} />
            </>
          )}
        </>
      )}
    </div>
  );
});

const WorkflowView: React.FC = () => {
  const { selectedDevice } = useDeviceStore();
  const { scripts } = useAutomationStore();
  const { t } = useTranslation();
  const { token } = theme.useToken();

  // Use workflowStore for workflow management
  const {
    workflows,
    selectedWorkflowId,
    workflowModalVisible,
    packages,
    appsLoading,
    editingWorkflowId,
    editingWorkflowName,
    setWorkflows,
    selectWorkflow,
    getSelectedWorkflow,
    addWorkflow,
    updateWorkflow,
    deleteWorkflow,
    setWorkflowModalVisible,
    setPackages,
    setAppsLoading,
  } = useWorkflowStore(useShallow(state => {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { executionLogs, workflowStepMap, ...rest } = state;
    return rest;
  }));

  const selectedWorkflow = getSelectedWorkflow();
  const [workflowForm] = Form.useForm();

  // Use workflowStore for execution state with selector to avoid re-rendering on high-frequency updates
  const {
    isRunning,
    runningWorkflowIds,
    isPaused,
    currentStepId,
    waitingStepId,
    waitingPhase,
    runtimeContext,
    // executionLogs, // Excluded to prevent re-renders
    variablesModalVisible,
    tempVariables,
    // workflowStepMap, // Excluded to prevent re-renders
    drawerVisible,
    elementPickerVisible,
    editingNodeId,
    needsAutoSave,
    setIsRunning,
    setRunningWorkflowIds,
    addRunningWorkflowId,
    removeRunningWorkflowId,
    setIsPaused,
    setCurrentStepId,
    setWaitingStep,
    setWaitingStepId,
    setWaitingPhase,
    setExecutionLogs,
    addExecutionLog,
    clearExecutionLogs,
    setVariablesModalVisible,
    setTempVariables,
    setWorkflowStepMap,
    updateWorkflowStepStatus,
    clearWorkflowStepStatus,
    setRuntimeContext,
    clearRuntimeContext,
    setDrawerVisible,
    setElementPickerVisible,
    setEditingNode,
    setEditingNodeId,
    setEditingWorkflow,
    setEditingWorkflowId,
    setEditingWorkflowName,
    setNeedsAutoSave,
    setSelectedWorkflow,
    viewMode,
    setViewMode,
  } = useWorkflowStore(useShallow(state => {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { executionLogs, workflowStepMap, ...rest } = state;
    return rest;
  }));

  // Access workflowStepMap directly from state for initial render logic without subscribing to updates
  const workflowStepMap = useWorkflowStore.getState().workflowStepMap;

  // Variable options for AutoComplete
  const variableOptions = useMemo(() => {
    if (!selectedWorkflow?.variables) return [];
    return Object.keys(selectedWorkflow.variables).map(key => ({
      label: `{{${key}}}`,
      value: `{{${key}}}`,
    }));
  }, [selectedWorkflow?.variables]);

  // React Flow state
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  // Flag to trigger auto-save after reconnection
  const reconnectionEdgesRef = useRef<Edge[]>([]);

  // Store cleanup function for workflow event listeners
  const cleanupEventsRef = useRef<(() => void) | null>(null);
  // Store the device ID that started the workflow (for stop/pause/resume)
  const runningDeviceIdRef = useRef<string | null>(null);

  // Wrapper for onNodesChange to protect Start node from deletion and auto-reconnect
  const onNodesChangeWithProtection = useCallback(
    (changes: NodeChange[]) => {
      // Handle node removal with auto-reconnection
      const removeChanges = changes.filter(c => c.type === 'remove');

      if (removeChanges.length > 0) {
        let hasReconnection = false;

        removeChanges.forEach(change => {
          if (change.type === 'remove') {
            const nodeToDelete = nodes.find(n => n.id === change.id);

            // Prevent deleting start node
            if (nodeToDelete && (nodeToDelete.data.step as WorkflowStep).type === 'start') {
              message.warning(t("workflow.error_delete_start"));
              return;
            }

            // Find incoming and outgoing edges for this node
            const incomingEdges = edges.filter(e => e.target === change.id);
            const outgoingEdges = edges.filter(e => e.source === change.id);

            console.log('[Delete Node via Keyboard] Deleting node:', change.id);
            console.log('[Delete Node via Keyboard] Incoming edges:', incomingEdges);
            console.log('[Delete Node via Keyboard] Outgoing edges:', outgoingEdges);

            // Create new edges to reconnect predecessors to successors
            if (incomingEdges.length > 0 && outgoingEdges.length > 0) {
              const newEdges: Edge[] = [];
              incomingEdges.forEach(inEdge => {
                outgoingEdges.forEach(outEdge => {
                  const newEdge: any = {
                    id: `edge-${inEdge.source}-${outEdge.target}-${Date.now()}-${Math.random()}`,
                    source: inEdge.source,
                    target: outEdge.target,
                    sourceHandle: inEdge.sourceHandle,
                    type: 'smoothstep' as const,
                    markerEnd: { type: MarkerType.ArrowClosed },
                  };
                  // Only set targetHandle if it has a valid value
                  if (outEdge.targetHandle && outEdge.targetHandle !== 'default' && outEdge.targetHandle !== 'null') {
                    newEdge.targetHandle = outEdge.targetHandle;
                  }
                  console.log('[Delete Node via Keyboard] Creating new edge:', newEdge);
                  newEdges.push(newEdge);
                });
              });

              // Add new reconnection edges
              setEdges(eds => {
                const filtered = eds.filter(e => e.source !== change.id && e.target !== change.id);
                const result = [...filtered, ...newEdges];
                console.log('[Delete Node via Keyboard] New edges after deletion:', result);
                return result;
              });

              // Store edges for re-application after save
              reconnectionEdgesRef.current = newEdges;
              hasReconnection = true;
            }
          }
        });

        if (hasReconnection) {
          // Set flag to trigger auto-save in useEffect
          setNeedsAutoSave(true);
        }
      }

      // Filter out Start node deletion attempts
      const filteredChanges = changes.filter(c => {
        if (c.type === 'remove') {
          const node = nodes.find(n => n.id === c.id);
          if (node && (node.data.step as WorkflowStep).type === 'start') {
            return false;
          }
        }
        return true;
      });

      onNodesChange(filteredChanges);
    },
    [nodes, edges, onNodesChange, setEdges, t]
  );

  // Auto-save workflow after node deletion with reconnection
  useEffect(() => {
    if (needsAutoSave && selectedWorkflow) {
      const timer = setTimeout(async () => {
        console.log('[Auto-save] Saving workflow after auto-reconnection');
        await handleSaveGraph(true);

        // Wait a bit for workflow to reload, then re-add the reconnection edges
        setTimeout(() => {
          if (reconnectionEdgesRef.current.length > 0) {
            console.log('[Auto-save] Re-adding reconnection edges:', reconnectionEdgesRef.current);
            setEdges(eds => {
              // Remove any existing edges with the same source-target pairs to avoid duplicates
              const filtered = eds.filter(e => {
                return !reconnectionEdgesRef.current.some(re =>
                  re.source === e.source && re.target === e.target
                );
              });
              const result = [...filtered, ...reconnectionEdgesRef.current];
              console.log('[Auto-save] Edges after re-adding:', result);
              return result;
            });
            reconnectionEdgesRef.current = [];
          }
          message.success(t("workflow.node_deleted_reconnected"));
        }, 100);

        setNeedsAutoSave(false);
      }, 200);

      return () => clearTimeout(timer);
    }
  }, [needsAutoSave, selectedWorkflow]);

  // Node Editing


  const fetchPackages = useCallback(async () => {
    if (!selectedDevice) return;
    setAppsLoading(true);
    try {
      const res = await (window as any).go.main.App.ListPackages(selectedDevice, 'user');
      setPackages(res || []);
    } catch (err) {
      console.error("Failed to fetch packages:", err);
    } finally {
      setAppsLoading(false);
    }
  }, [selectedDevice]);

  useEffect(() => {
    fetchPackages();
  }, [fetchPackages]);
  const [stepForm] = Form.useForm();

  // Element Picker


  // Track if we should skip the next node re-render (after saving)
  const skipNextRenderRef = useRef(false);

  // Custom edge component with selection state
  const CustomEdge = useCallback(({
    id,
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
    style = {},
    markerEnd,
    label,
    labelStyle,
    selected,
  }: EdgeProps) => {
    const [edgePath, labelX, labelY] = getSmoothStepPath({
      sourceX,
      sourceY,
      sourcePosition,
      targetX,
      targetY,
      targetPosition,
    });

    // Enhance style when selected
    const enhancedStyle = {
      ...style,
      strokeWidth: selected ? 3 : 1.5,
      filter: selected ? 'drop-shadow(0 0 4px currentColor)' : undefined,
    };

    return (
      <>
        <BaseEdge id={id} path={edgePath} markerEnd={markerEnd} style={enhancedStyle} />
        {label && (
          <EdgeLabelRenderer>
            <div
              style={{
                position: 'absolute',
                transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)`,
                fontSize: 10,
                fontWeight: selected ? 800 : 700,
                pointerEvents: 'all',
                background: token.colorBgContainer,
                padding: '2px 6px',
                borderRadius: 4,
                border: selected ? `2px solid currentColor` : `1px solid ${token.colorBorder}`,
                boxShadow: selected ? `0 0 6px currentColor` : undefined,
                ...labelStyle,
              }}
              className="nodrag nopan"
            >
              {label}
            </div>
          </EdgeLabelRenderer>
        )}
      </>
    );
  }, [token]);

  const nodeTypes = useMemo(() => ({ workflowNode: WorkflowNode }), []);
  const edgeTypes = useMemo(() => ({ smoothstep: CustomEdge }), [CustomEdge]);

  const loadWorkflows = async () => {
    try {
      const result = await (window as any).go.main.App.LoadWorkflows();
      setWorkflows(result || []);
    } catch (err) {
      setWorkflows([]);
    }
  };

  // Load workflows on mount and subscribe to workflow list changes (e.g., from MCP updates)
  useEffect(() => {
    loadWorkflows();

    const unsubscribe = EventsOn("workflow-list-changed", (data: any) => {
      console.log("[Workflow] List changed from backend:", data);
      loadWorkflows();
    });

    return () => {
      unsubscribe();
    };
  }, []);

  const getLayoutedElements = useCallback((nodes: Node[], edges: Edge[]) => {
    if (nodes.length === 0) return { nodes, edges };

    const dagreGraph = new dagre.graphlib.Graph();
    dagreGraph.setDefaultEdgeLabel(() => ({}));
    const nodeWidth = 280;
    const nodeHeight = 80;

    // Configure dagre with better spacing
    dagreGraph.setGraph({
      rankdir: 'TB',      // Top to Bottom
      ranksep: 80,        // Vertical spacing between levels
      nodesep: 50,        // Horizontal spacing between nodes
      edgesep: 20,        // Minimum edge separation
      marginx: 20,
      marginy: 20,
    });

    nodes.forEach((node) => {
      dagreGraph.setNode(node.id, { width: nodeWidth, height: nodeHeight });
    });

    edges.forEach((edge) => {
      dagreGraph.setEdge(edge.source, edge.target);
    });

    dagre.layout(dagreGraph);

    const layoutedNodes = nodes.map((node) => {
      const nodeWithPosition = dagreGraph.node(node.id);
      return {
        ...node,
        position: {
          x: nodeWithPosition.x - nodeWidth / 2,
          y: nodeWithPosition.y - nodeHeight / 2,
        },
      };
    });

    return { nodes: layoutedNodes, edges };
  }, []);

  const handleExecuteSingleStep = useCallback(async (step: WorkflowStep) => {
    if (!selectedDevice) {
      message.warning(t("app.select_device"));
      return;
    }
    if (step.type === 'start') {
      return; // Start node doesn't execute anything
    }
    try {
      await (window as any).go.main.App.ExecuteSingleWorkflowStep(selectedDevice, step);
      message.success(t("workflow.step_executed"));
    } catch (err) {
      message.error(String(err));
    }
  }, [selectedDevice, t]);

  useEffect(() => {
    // Also clear waiting when workflow ends
    if (!isRunning) {
      setWaitingStepId(null);
      setWaitingPhase(null);
    }
  }, [isRunning]);

  // Global event listener for workflow step updates
  useEffect(() => {
    const onStepRunning = (data: any) => {
      if (data.workflowId && data.stepId) {
        setWorkflowStepMap(prev => ({
          ...prev,
          [data.workflowId]: {
            stepId: data.stepId,
            isWaiting: false
          }
        }));
        setCurrentStepId(data.stepId);
      }
    };

    const onStepWaiting = (data: any) => {
      if (data.workflowId && data.stepId) {
        setWorkflowStepMap(prev => ({
          ...prev,
          [data.workflowId]: {
            stepId: data.stepId,
            isWaiting: true,
            waitPhase: data.phase
          }
        }));
        setWaitingStepId(data.stepId);
        setWaitingPhase(data.phase);
      }
    };

    const unsubStep = EventsOn("workflow-step-running", onStepRunning);
    const unsubWait = EventsOn("workflow-step-waiting", onStepWaiting);

    return () => {
      unsubStep();
      unsubWait();
    };
  }, [setWorkflowStepMap, setCurrentStepId, setWaitingStepId, setWaitingPhase]);

  useEffect(() => {
    // Skip re-rendering nodes if we just saved (nodes are already up-to-date)
    if (skipNextRenderRef.current) {
      skipNextRenderRef.current = false;
      return;
    }

    if (selectedWorkflow) {
      const newNodes: Node[] = selectedWorkflow.steps.map((step, index) => {
        // Check if this step is currently running in this workflow
        const stepState = workflowStepMap[selectedWorkflow.id];
        const isCurrent = stepState?.stepId === step.id && !stepState?.isWaiting;
        const isWaiting = stepState?.stepId === step.id && stepState?.isWaiting;
        const waitPhase = stepState?.waitPhase;

        // V2: Read position from layout
        const posX = step.layout?.posX ?? 0;
        const posY = step.layout?.posY ?? 0;
        
        return {
          id: step.id,
          type: 'workflowNode',
          position: { x: posX, y: posY },
          data: {
            step,
            label: step.name || t(`workflow.step_type.${step.type}`),
            isCurrent,
            isWaiting,
            waitingPhase: waitPhase,
            onExecuteStep: handleExecuteSingleStep,
            canExecute: !!selectedDevice && !isRunning,
          },
        };
      });

      const newEdges: Edge[] = [];
      // V2: Check for connections structure
      const hasGraphData = selectedWorkflow.steps.some(s => 
        s.connections?.successStepId || s.connections?.errorStepId || 
        s.connections?.trueStepId || s.connections?.falseStepId
      );

      if (hasGraphData) {
        selectedWorkflow.steps.forEach(step => {
          const conn = step.connections;
          const handles = step.layout?.handles;
          
          // Success connection (for non-branch nodes)
          if (conn?.successStepId && step.type !== 'branch') {
            const edge: any = {
              id: `e-${step.id}-${conn.successStepId}-success`,
              source: step.id,
              target: conn.successStepId,
              type: 'smoothstep',
              sourceHandle: handles?.success?.sourceHandle || 'success',
              label: step.type === 'start' ? undefined : t('workflow.success'),
              style: step.type === 'start' ? undefined : { stroke: token.colorSuccess },
              labelStyle: step.type === 'start' ? undefined : { fill: token.colorSuccess, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: step.type === 'start' ? undefined : token.colorSuccess }
            };
            if (isValidTargetHandle(handles?.success?.targetHandle)) {
              edge.targetHandle = handles?.success?.targetHandle;
            }
            newEdges.push(edge);
          }
          
          // Error connection (for non-branch nodes)
          if (conn?.errorStepId && step.type !== 'branch') {
            const edge: any = {
              id: `e-${step.id}-${conn.errorStepId}-error`,
              source: step.id,
              target: conn.errorStepId,
              type: 'smoothstep',
              sourceHandle: handles?.error?.sourceHandle || 'error',
              label: t('workflow.error') || 'Error',
              style: { stroke: token.colorWarning },
              labelStyle: { fill: token.colorWarning, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: token.colorWarning }
            };
            if (isValidTargetHandle(handles?.error?.targetHandle)) {
              edge.targetHandle = handles?.error?.targetHandle;
            }
            newEdges.push(edge);
          }
          
          // True connection (for branch nodes)
          if (conn?.trueStepId) {
            const edge: any = {
              id: `e-${step.id}-${conn.trueStepId}-true`,
              source: step.id,
              target: conn.trueStepId,
              type: 'smoothstep',
              sourceHandle: handles?.true?.sourceHandle || 'true',
              label: t('workflow.branch_true'),
              style: { stroke: token.colorSuccess },
              labelStyle: { fill: token.colorSuccess, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: token.colorSuccess }
            };
            if (isValidTargetHandle(handles?.true?.targetHandle)) {
              edge.targetHandle = handles?.true?.targetHandle;
            }
            newEdges.push(edge);
          }
          
          // False connection (for branch nodes)
          if (conn?.falseStepId) {
            const edge: any = {
              id: `e-${step.id}-${conn.falseStepId}-false`,
              source: step.id,
              target: conn.falseStepId,
              type: 'smoothstep',
              sourceHandle: handles?.false?.sourceHandle || 'false',
              label: t('workflow.branch_false'),
              style: { stroke: token.colorError },
              labelStyle: { fill: token.colorError, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: token.colorError }
            };
            if (isValidTargetHandle(handles?.false?.targetHandle)) {
              edge.targetHandle = handles?.false?.targetHandle;
            }
            newEdges.push(edge);
          }
          
          // Error connection for branch nodes
          if (conn?.errorStepId && step.type === 'branch') {
            const edge: any = {
              id: `e-${step.id}-${conn.errorStepId}-error`,
              source: step.id,
              target: conn.errorStepId,
              type: 'smoothstep',
              sourceHandle: handles?.error?.sourceHandle || 'error',
              label: t('workflow.error') || 'Error',
              style: { stroke: token.colorWarning },
              labelStyle: { fill: token.colorWarning, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: token.colorWarning }
            };
            if (isValidTargetHandle(handles?.error?.targetHandle)) {
              edge.targetHandle = handles?.error?.targetHandle;
            }
            newEdges.push(edge);
          }
        });
      } else {
        // Fallback: linear workflow without explicit connections
        selectedWorkflow.steps.slice(0, -1).forEach((step, index) => {
          newEdges.push({
            id: `e-${step.id}-${selectedWorkflow.steps[index + 1].id}`,
            source: step.id,
            target: selectedWorkflow.steps[index + 1].id,
            type: 'smoothstep',
            sourceHandle: 'success',
            markerEnd: { type: MarkerType.ArrowClosed },
          });
        });
      }

      // V2: Check for layout positions
      const hasPositions = selectedWorkflow.steps.some(s => s.layout?.posX !== undefined);

      // Clean up edges: remove invalid targetHandle values
      const cleanedEdges = newEdges.map(edge => {
        const cleanEdge = { ...edge };
        if (cleanEdge.targetHandle && !isValidTargetHandle(cleanEdge.targetHandle)) {
          console.warn('[Workflow] Removing invalid targetHandle:', cleanEdge.targetHandle, 'from edge:', cleanEdge.id);
          delete cleanEdge.targetHandle;
        }
        return cleanEdge;
      });

      if (hasPositions) {
        setNodes(newNodes);
        setEdges(cleanedEdges);
      } else {
        const layouted = getLayoutedElements(newNodes, cleanedEdges);
        setNodes(layouted.nodes);
        setEdges(layouted.edges);
      }
    } else {
      setNodes([]);
      setEdges([]);
    }
  }, [selectedWorkflow, getLayoutedElements, t, token, handleExecuteSingleStep, selectedDevice, isRunning]);

  useEffect(() => {
    if (!selectedWorkflow) return;

    // 订阅 workflowStepMap 变化，实时更新节点高亮状态
    const unsub = useWorkflowStore.subscribe((state, prevState) => {
      if (state.workflowStepMap === prevState?.workflowStepMap) {
        return;
      }

      const map = state.workflowStepMap;
      const stepState = map[selectedWorkflow.id];

      // 直接同步更新节点状态
      setNodes((nds) => nds.map((node) => {
        const isCurrent = stepState?.stepId === node.id && !stepState?.isWaiting;
        const isWaiting = stepState?.stepId === node.id && stepState?.isWaiting;
        const phase = stepState?.stepId === node.id ? stepState?.waitPhase : null;

        if (node.data.isCurrent !== isCurrent || node.data.isWaiting !== isWaiting || node.data.waitingPhase !== phase) {
          return {
            ...node,
            data: { ...(node.data as object), isCurrent, isWaiting, isRunning, waitingPhase: phase }
          };
        }
        return node;
      }));
    });

    return () => unsub();
  }, [selectedWorkflow, isRunning, setNodes]);

  const onConnect = useCallback(
    (params: Connection) => {
      const newEdge: any = {
        ...params,
        type: 'smoothstep',
        markerEnd: { type: MarkerType.ArrowClosed }
      };
      // Remove targetHandle if it's invalid to avoid warnings
      if (!isValidTargetHandle(newEdge.targetHandle)) {
        delete newEdge.targetHandle;
      }
      setEdges((eds) => addEdge(newEdge, eds));
    },
    [setEdges],
  );

  const handleAutoLayout = useCallback((direction: 'TB' | 'LR' = 'TB') => {
    if (nodes.length === 0) return;

    const dagreGraph = new dagre.graphlib.Graph();
    dagreGraph.setDefaultEdgeLabel(() => ({}));

    // Ghost Size Strategy (slightly tuned)
    // TB: Wide ghost width to push neighbors apart.
    // LR: Tall ghost height to push neighbors apart.
    const isHorizontal = direction === 'LR';

    // Adjust spacing based on direction
    dagreGraph.setGraph({
      rankdir: direction,
      ranker: 'network-simplex',
      ranksep: isHorizontal ? 150 : 40, // TB: vertical distance between ranks
      nodesep: isHorizontal ? 40 : 40,  // LR: vertical distance between siblings
      edgesep: 50,
      marginx: 50,
      marginy: 50,
    });

    nodes.forEach((node) => {
      // TB: Vertical spacing is predominantly ranksep + ghostHeight
      // LR: Vertical spacing is predominantly nodesep + ghostHeight
      const ghostWidth = isHorizontal ? 320 : 360;
      const ghostHeight = isHorizontal ? 120 : 100;

      dagreGraph.setNode(node.id, { width: ghostWidth, height: ghostHeight });
    });

    edges.forEach((edge) => {
      dagreGraph.setEdge(edge.source, edge.target);
    });

    dagre.layout(dagreGraph);

    const layoutedNodes = nodes.map((node) => {
      const nodeWithPosition = dagreGraph.node(node.id);

      return {
        ...node,
        position: {
          x: nodeWithPosition.x - 140, // 280 / 2
          y: nodeWithPosition.y - 40,  // 80 / 2
        },
      };
    });

    setNodes(layoutedNodes);
    message.success(t("workflow.layout_applied") + (isHorizontal ? " (Horizontal)" : " (Vertical)"));
  }, [nodes, edges, setNodes, t]);

  const handleNodeClick = (event: React.MouseEvent, node: Node) => {
    setEditingNodeId(node.id);
    const step = node.data.step as WorkflowStep;
    stepForm.resetFields();
    
    // V2: Extract form values from type-specific params
    const selector = step.element?.selector || step.branch?.selector || step.readToVariable?.selector;
    const conditionType = step.branch?.condition || 'exists';
    
    // Get value based on step type
    let value: string | undefined;
    if (step.app?.packageName) value = step.app.packageName;
    else if (step.wait?.durationMs) value = String(step.wait.durationMs);
    else if (step.script?.scriptName) value = step.script.scriptName;
    else if (step.variable?.value) value = step.variable.value;
    else if (step.adb?.command) value = step.adb.command;
    else if (step.workflow?.workflowId) value = step.workflow.workflowId;
    else if (step.element?.inputText) value = step.element.inputText;
    else if (step.element?.swipeDir) value = step.element.swipeDir;
    else if (step.branch?.expectedValue) value = step.branch.expectedValue;
    else if (step.readToVariable?.defaultValue) value = step.readToVariable.defaultValue;
    
    stepForm.setFieldsValue({
      type: step.type,
      name: step.name,
      selectorType: selector?.type,
      selectorValue: selector?.value,
      conditionType,
      value,
      timeout: step.common?.timeout,
      onError: step.common?.onError,
      loop: step.common?.loop,
      preWait: step.common?.preWait,
      postDelay: step.common?.postDelay,
      swipeDistance: step.element?.swipeDistance || step.swipe?.distance,
      swipeDuration: step.element?.swipeDuration || step.swipe?.duration,
      x: step.tap?.x || step.swipe?.x,
      y: step.tap?.y || step.swipe?.y,
      x2: step.swipe?.x2,
      y2: step.swipe?.y2,
      // read_to_variable fields
      variableName: step.readToVariable?.variableName || step.variable?.name,
      attribute: step.readToVariable?.attribute || 'text',
      regex: step.readToVariable?.regex,
      defaultValue: step.readToVariable?.defaultValue,
      // session fields
      sessionName: step.session?.sessionName,
      logcatEnabled: step.session?.logcatEnabled,
      logcatPackageName: step.session?.logcatPackageName,
      logcatPreFilter: step.session?.logcatPreFilter,
      logcatExcludeFilter: step.session?.logcatExcludeFilter,
      recordingEnabled: step.session?.recordingEnabled,
      recordingQuality: step.session?.recordingQuality || 'medium',
      proxyEnabled: step.session?.proxyEnabled,
      proxyPort: step.session?.proxyPort || 8080,
      proxyMitmEnabled: step.session?.proxyMitmEnabled,
      monitorEnabled: step.session?.monitorEnabled,
      sessionStatus: step.session?.status || 'completed',
    });
    setDrawerVisible(true);
  };

  const handleCreateWorkflow = () => {
    workflowForm.resetFields();
    selectWorkflow(null);
    setWorkflowModalVisible(true);
  };

  const handleCreateWorkflowSubmit = async (values: any) => {
    // V2: Auto-create a Start node with new structure
    const startStep: WorkflowStep = {
      id: `start_${Date.now()}`,
      type: 'start',
      name: 'Start',
      connections: { successStepId: '' },
      layout: { posX: 250, posY: 50 },
    };

    const newWorkflow: Workflow = {
      id: `wf_${Date.now()}`,
      name: values.name,
      description: values.description,
      steps: [startStep],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    try {
      await (window as any).go.main.App.SaveWorkflow(newWorkflow);
      message.success(t("workflow.saved"));
      setWorkflowModalVisible(false);
      addWorkflow(newWorkflow);
      selectWorkflow(newWorkflow.id);
    } catch (err) {
      message.error(String(err));
    }
  }

  const handleSaveGraph = async (silent: boolean = false): Promise<Workflow | null> => {
    if (!selectedWorkflow) return null;

    const outEdgesMap = new Map<string, Edge[]>();
    edges.forEach(e => {
      const list = outEdgesMap.get(e.source) || [];
      list.push(e);
      outEdgesMap.set(e.source, list);
    });

    // Check for start node
    const hasStartNode = nodes.some(n => (n.data.step as WorkflowStep).type === 'start');
    if (!hasStartNode) {
      if (!silent) message.error(t("workflow.error_no_start"));
      return null;
    }

    // Check if start node has outgoing connection
    const startNode = nodes.find(n => (n.data.step as WorkflowStep).type === 'start');
    const startOutgoing = outEdgesMap.get(startNode?.id || '') || [];
    if (startOutgoing.length === 0) {
      if (!silent) message.warning(t("workflow.warning_start_not_connected"));
    }

    const stepsToSave: WorkflowStep[] = nodes.map(node => {
      const originalStep = node.data.step as WorkflowStep;
      const outgoing = outEdgesMap.get(node.id) || [];

      // V2: Build connections and handles
      let successStepId: string | undefined;
      let errorStepId: string | undefined;
      let trueStepId: string | undefined;
      let falseStepId: string | undefined;
      
      const handles: Record<string, { sourceHandle?: string; targetHandle?: string }> = {};

      if (originalStep.type === 'branch') {
        // Branch node: true/false/error outputs
        const trueEdge = outgoing.find(e => e.sourceHandle === 'true' || e.sourceHandle === 'true-left');
        const falseEdge = outgoing.find(e => e.sourceHandle === 'false');
        const errorEdge = outgoing.find(e => e.sourceHandle === 'error' || e.sourceHandle === 'error-right');
        
        if (trueEdge) {
          trueStepId = trueEdge.target;
          handles['true'] = {
            sourceHandle: trueEdge.sourceHandle || 'true',
            targetHandle: isValidTargetHandle(trueEdge.targetHandle) ? String(trueEdge.targetHandle) : undefined,
          };
        }
        if (falseEdge) {
          falseStepId = falseEdge.target;
          handles['false'] = {
            sourceHandle: falseEdge.sourceHandle || 'false',
            targetHandle: isValidTargetHandle(falseEdge.targetHandle) ? String(falseEdge.targetHandle) : undefined,
          };
        }
        if (errorEdge) {
          errorStepId = errorEdge.target;
          handles['error'] = {
            sourceHandle: errorEdge.sourceHandle || 'error',
            targetHandle: isValidTargetHandle(errorEdge.targetHandle) ? String(errorEdge.targetHandle) : undefined,
          };
        }
      } else {
        // Normal node: success/error outputs
        const successEdge = outgoing.find(e => 
          e.sourceHandle === 'success' || e.sourceHandle === 'success-left' || 
          e.sourceHandle === 'default' || !e.sourceHandle
        );
        const errorEdge = outgoing.find(e => e.sourceHandle === 'error' || e.sourceHandle === 'error-right');
        
        if (successEdge) {
          successStepId = successEdge.target;
          handles['success'] = {
            sourceHandle: successEdge.sourceHandle || 'success',
            targetHandle: isValidTargetHandle(successEdge.targetHandle) ? String(successEdge.targetHandle) : undefined,
          };
        }
        if (errorEdge) {
          errorStepId = errorEdge.target;
          handles['error'] = {
            sourceHandle: errorEdge.sourceHandle || 'error',
            targetHandle: isValidTargetHandle(errorEdge.targetHandle) ? String(errorEdge.targetHandle) : undefined,
          };
        }
      }

      // V2: Build the step with new structure
      return {
        ...originalStep,
        connections: {
          successStepId,
          errorStepId,
          trueStepId,
          falseStepId,
        },
        layout: {
          posX: node.position.x,
          posY: node.position.y,
          handles: Object.keys(handles).length > 0 ? handles : undefined,
        },
      };
    });

    const updatedWorkflow = {
      ...selectedWorkflow,
      steps: stepsToSave,
      updatedAt: new Date().toISOString(),
    };

    try {
      await (window as any).go.main.App.SaveWorkflow(updatedWorkflow);

      // Mark to skip the next useEffect render - nodes are already up-to-date
      skipNextRenderRef.current = true;

      // Update workflow in store
      updateWorkflow(selectedWorkflow.id, updatedWorkflow);
      selectWorkflow(updatedWorkflow.id);

      if (!silent) {
        message.success(t("workflow.saved"));
      }
      return updatedWorkflow;
    } catch (err) {
      if (!silent) {
        message.error(String(err));
      }
      return null;
    }
  };

  const handleAddStep = (type: string) => {
    if (!selectedWorkflow) return;

    // Prevent adding multiple start nodes
    if (type === 'start') {
      const hasStart = nodes.some(n => (n.data.step as WorkflowStep).type === 'start');
      if (hasStart) {
        message.warning(t("workflow.error_duplicate_start"));
        return;
      }
    }

    const id = `step_${Date.now()}`;
    // V2: Create step with new structure
    const newStep: WorkflowStep = {
      id,
      type: type as WorkflowStep['type'],
      common: {
        loop: 1,
        postDelay: 0,
        onError: 'stop',
      },
      connections: {},
      layout: { posX: 250, posY: (nodes.length * 100) + 50 },
    };

    const newNode: Node = {
      id,
      type: 'workflowNode',
      position: { x: 250, y: (nodes.length * 100) + 50 },
      data: {
        step: newStep,
        label: t(`workflow.step_type.${type}`),
        isCurrent: false
      }
    };

    if (nodes.length > 0) {
      const hasOutgoing = new Set(edges.map(e => e.source));
      const tails = nodes.filter(n => !hasOutgoing.has(n.id));

      if (tails.length === 1) {
        const lastNode = tails[0];
        const lastType = (lastNode.data.step as WorkflowStep).type;
        if (lastType !== 'branch') {
          // V2: Use 'success' handle instead of 'default'
          const newEdge = {
            id: `e-${lastNode.id}-${id}`,
            source: lastNode.id,
            target: id,
            type: 'smoothstep',
            sourceHandle: 'success',
            markerEnd: { type: MarkerType.ArrowClosed },
          };
          setEdges(eds => [...eds, newEdge]);
          newNode.position = { x: lastNode.position.x, y: lastNode.position.y + 120 };
        } else {
          newNode.position = { x: lastNode.position.x, y: lastNode.position.y + 120 };
        }
      }
    }

    setNodes(nds => [...nds, newNode]);

    // Don't open drawer for start node - it doesn't need configuration
    if (type !== 'start') {
      setEditingNodeId(id);
      stepForm.resetFields();
      const defaultValues: any = { type, onError: 'stop', loop: 1 };
      if (type === 'branch') {
        defaultValues.conditionType = 'exists'; // Default condition type for branch nodes
      }
      stepForm.setFieldsValue(defaultValues);
      setDrawerVisible(true);
    }
  };

  const handleUpdateStep = (values: any) => {
    if (!editingNodeId) return;

    setNodes(nds => nds.map(node => {
      if (node.id === editingNodeId) {
        const originalStep = node.data.step as WorkflowStep;
        
        // V2: Build type-specific params
        let tap = originalStep.tap;
        let swipe = originalStep.swipe;
        let element = originalStep.element;
        let app = originalStep.app;
        let branch = originalStep.branch;
        let wait = originalStep.wait;
        let script = originalStep.script;
        let variable = originalStep.variable;
        let adb = originalStep.adb;
        let workflow = originalStep.workflow;
        
        const stepType = values.type || originalStep.type;
        
        // Update type-specific params based on step type
        if (stepType === 'tap') {
          tap = {
            x: values.x !== undefined ? Number(values.x) : (originalStep.tap?.x ?? 0),
            y: values.y !== undefined ? Number(values.y) : (originalStep.tap?.y ?? 0),
          };
        } else if (stepType === 'swipe') {
          swipe = {
            x: values.x !== undefined ? Number(values.x) : originalStep.swipe?.x,
            y: values.y !== undefined ? Number(values.y) : originalStep.swipe?.y,
            x2: values.x2 !== undefined ? Number(values.x2) : originalStep.swipe?.x2,
            y2: values.y2 !== undefined ? Number(values.y2) : originalStep.swipe?.y2,
            distance: values.swipeDistance,
            duration: values.swipeDuration,
          };
        } else if (['click_element', 'long_click_element', 'input_text', 'swipe_element', 'wait_element', 'wait_gone', 'assert_element'].includes(stepType)) {
          element = {
            selector: values.selectorValue ? {
              type: values.selectorType || 'text',
              value: values.selectorValue,
            } : originalStep.element?.selector || { type: 'text', value: '' },
            action: stepType === 'click_element' ? 'click' :
                    stepType === 'long_click_element' ? 'long_click' :
                    stepType === 'input_text' ? 'input' :
                    stepType === 'swipe_element' ? 'swipe' :
                    stepType === 'wait_element' ? 'wait' :
                    stepType === 'wait_gone' ? 'wait_gone' : 'assert',
            inputText: stepType === 'input_text' ? values.value : undefined,
            swipeDir: stepType === 'swipe_element' ? values.value : undefined,
            swipeDistance: values.swipeDistance,
            swipeDuration: values.swipeDuration,
          };
        } else if (['launch_app', 'stop_app', 'clear_app', 'open_settings'].includes(stepType)) {
          app = {
            packageName: values.value || '',
            action: stepType === 'launch_app' ? 'launch' :
                    stepType === 'stop_app' ? 'stop' :
                    stepType === 'clear_app' ? 'clear' : 'settings',
          };
        } else if (stepType === 'branch') {
          branch = {
            condition: (values.conditionType || 'exists') as any,
            selector: values.selectorValue ? {
              type: values.selectorType || 'text',
              value: values.selectorValue,
            } : originalStep.branch?.selector,
            expectedValue: values.value,
          };
        } else if (stepType === 'wait') {
          wait = {
            durationMs: parseInt(values.value || '1000', 10) || 1000,
          };
        } else if (stepType === 'script') {
          script = {
            scriptName: values.value || '',
          };
        } else if (stepType === 'set_variable') {
          variable = {
            name: values.variableName || values.name || '',
            value: values.value || '',
          };
        } else if (stepType === 'read_to_variable') {
          // read_to_variable is handled separately below
        } else if (stepType === 'adb') {
          adb = {
            command: values.value || '',
          };
        } else if (stepType === 'run_workflow') {
          workflow = {
            workflowId: values.value || '',
          };
        }

        // Build session params if applicable
        const session = ['start_session', 'end_session'].includes(stepType) ? {
          sessionName: values.sessionName || undefined,
          logcatEnabled: values.logcatEnabled || false,
          logcatPackageName: values.logcatPackageName || undefined,
          logcatPreFilter: values.logcatPreFilter || undefined,
          logcatExcludeFilter: values.logcatExcludeFilter || undefined,
          recordingEnabled: values.recordingEnabled || false,
          recordingQuality: values.recordingQuality || undefined,
          proxyEnabled: values.proxyEnabled || false,
          proxyPort: values.proxyPort || undefined,
          proxyMitmEnabled: values.proxyMitmEnabled || false,
          monitorEnabled: values.monitorEnabled || false,
          status: stepType === 'end_session' ? (values.sessionStatus || 'completed') : undefined,
        } : originalStep.session;
        
        // Build readToVariable params if applicable
        const readToVariable = stepType === 'read_to_variable' ? {
          selector: values.selectorValue ? {
            type: values.selectorType || 'text',
            value: values.selectorValue,
          } : originalStep.readToVariable?.selector || { type: 'text', value: '' },
          variableName: values.variableName || '',
          attribute: values.attribute || 'text',
          regex: values.regex || undefined,
          defaultValue: values.defaultValue || undefined,
        } : originalStep.readToVariable;

        const updatedStep: WorkflowStep = {
          ...originalStep,
          type: stepType,
          name: values.name,
          common: {
            ...originalStep.common,
            timeout: values.timeout,
            onError: values.onError,
            loop: values.loop,
            preWait: Number(values.preWait || 0),
            postDelay: Number(values.postDelay || 0),
          },
          // V2: Type-specific params
          tap,
          swipe,
          element,
          app,
          branch,
          wait,
          script,
          variable,
          adb,
          workflow,
          readToVariable,
          session,
        };
        
        return {
          ...node,
          data: {
            ...node.data,
            step: updatedStep,
            label: values.name || t(`workflow.step_type.${stepType}`)
          }
        };
      }
      return node;
    }));
  };

  const handleDeleteNode = async () => {
    if (!editingNodeId) return;

    // Prevent deleting start node
    const nodeToDelete = nodes.find(n => n.id === editingNodeId);
    if (nodeToDelete && (nodeToDelete.data.step as WorkflowStep).type === 'start') {
      message.warning(t("workflow.error_delete_start"));
      return;
    }

    // Find incoming and outgoing edges
    const incomingEdges = edges.filter(e => e.target === editingNodeId);
    const outgoingEdges = edges.filter(e => e.source === editingNodeId);

    console.log('[Delete Node] Deleting node:', editingNodeId);
    console.log('[Delete Node] Incoming edges:', incomingEdges);
    console.log('[Delete Node] Outgoing edges:', outgoingEdges);

    // Create new edges to reconnect predecessors to successors
    const newEdges: Edge[] = [];

    if (incomingEdges.length > 0 && outgoingEdges.length > 0) {
      incomingEdges.forEach(inEdge => {
        outgoingEdges.forEach(outEdge => {
          const newEdge: any = {
            id: `edge-${inEdge.source}-${outEdge.target}-${Date.now()}-${Math.random()}`,
            source: inEdge.source,
            target: outEdge.target,
            sourceHandle: inEdge.sourceHandle || 'default',
            type: 'smoothstep' as const,
            markerEnd: { type: MarkerType.ArrowClosed },
          };
          // Only set targetHandle if it has a valid value
          if (outEdge.targetHandle && outEdge.targetHandle !== 'default' && outEdge.targetHandle !== 'null') {
            newEdge.targetHandle = outEdge.targetHandle;
          }
          console.log('[Delete Node] Creating new edge:', newEdge);
          newEdges.push(newEdge);
        });
      });
    }

    // Update nodes and edges
    setNodes(nds => nds.filter(n => n.id !== editingNodeId));
    setEdges(eds => {
      // Remove edges connected to deleted node
      const filtered = eds.filter(e => e.source !== editingNodeId && e.target !== editingNodeId);
      // Add new reconnection edges
      const result = [...filtered, ...newEdges];
      console.log('[Delete Node] New edges after deletion:', result);
      return result;
    });

    setDrawerVisible(false);
    setEditingNodeId(null);

    // Save the workflow immediately to persist the changes
    // Wait a bit for state to update
    setTimeout(async () => {
      await handleSaveGraph(true);
      if (newEdges.length > 0) {
        message.success(t("workflow.node_deleted_reconnected"));
      }
    }, 100);
  }

  const handleElementSelected = (selector: ElementSelector) => {
    stepForm.setFieldsValue({
      selectorType: selector.type,
      selectorValue: selector.value,
    });
    setElementPickerVisible(false);
    handleUpdateStep(stepForm.getFieldsValue());
    message.success(t("workflow.selector_applied"));
  };

  const handleRunWorkflow = async () => {
    if (!selectedDevice || !selectedWorkflow) {
      message.warning(t("app.select_device"));
      return;
    }

    const deviceObj = useDeviceStore.getState().devices.find(d => d.id === selectedDevice);
    if (!deviceObj) {
      message.error(t("workflow.error_device_not_found"));
      return;
    }

    const workflowToRun = await handleSaveGraph(true);
    if (!workflowToRun) return;

    if (workflowToRun.steps.length === 0) {
      message.warning(t("workflow.error_empty_steps"));
      return;
    }

    // V2: Sanitize ADB commands - use adb.command instead of value
    const sanitizedSteps = workflowToRun.steps.map(s => {
      if (s.type === 'adb' && s.adb?.command && s.adb.command.trim().startsWith('input ')) {
        return { ...s, adb: { command: `shell ${s.adb.command}` } };
      }
      return s;
    });
    const sanitizedWorkflow = { ...workflowToRun, steps: sanitizedSteps };

    // Save the device ID for stop/pause/resume operations
    runningDeviceIdRef.current = deviceObj.id;
    
    setIsRunning(true);
    setRunningWorkflowIds([selectedWorkflow.id]);
    setCurrentStepId(null);
    setExecutionLogs([
      `[${new Date().toLocaleTimeString()}] ${t("workflow.started")}: ${sanitizedWorkflow.name}`,
      `[Info] Executing ${sanitizedWorkflow.steps.length} steps.`
    ]);

    const executionPromise = new Promise<void>((resolve, reject) => {
      // 用 EventsOn 返回的 cancel 函数精确退订：task-paused/task-resumed 与
      // automationStore 共享，EventsOff(name) 会误删对方的监听器
      const eventCancels: Array<() => void> = [];
      const cleanUp = () => {
        eventCancels.forEach((off) => off?.());
        eventCancels.length = 0;
        // Clear the ref after cleanup
        cleanupEventsRef.current = null;
      };

      // Store cleanup function in ref for manual stop
      cleanupEventsRef.current = cleanUp;

      const onSubStarted = (data: any) => {
        if (data.deviceId === deviceObj.id && data.workflowId) {
          setRunningWorkflowIds(prev => Array.from(new Set([...prev, data.workflowId])));
          setExecutionLogs(prev => [...prev, `[${new Date().toLocaleTimeString()}] ${t("workflow.started")}: ${data.workflowName}`]);
        }
      };

      const onComplete = (data: any) => {
        if (data.deviceId === deviceObj.id) {
          setRunningWorkflowIds(prev => prev.filter(id => id !== data.workflowId));

          // Clean up step map for this workflow
          setWorkflowStepMap(prev => {
            const newMap = { ...prev };
            delete newMap[data.workflowId];
            return newMap;
          });

          if (data.workflowId === selectedWorkflow.id) {
            cleanUp();
            resolve();
          }
        }
      };

      const onError = (data: any) => {
        if (data.deviceId === deviceObj.id) {
          setRunningWorkflowIds(prev => prev.filter(id => id !== data.workflowId));

          // Clean up step map for this workflow
          setWorkflowStepMap(prev => {
            const newMap = { ...prev };
            delete newMap[data.workflowId];
            return newMap;
          });

          if (data.workflowId === selectedWorkflow.id) {
            cleanUp();
            reject(data.error);
          }
        }
      };

      const onStep = (data: any) => {
        console.log("[Workflow] onStep received:", data);
        // Temporarily show message for debugging
        message.info(`Step: ${data.stepId?.slice(-8) || 'unknown'}`, 0.5);
        
        if (data.deviceId === deviceObj.id && data.workflowId) {
          console.log("[Workflow] Step running:", data.stepId, "in workflow:", data.workflowId);

          // Update the step map for this workflow
          setWorkflowStepMap(prev => ({
            ...prev,
            [data.workflowId]: {
              stepId: data.stepId,
              isWaiting: false
            }
          }));

          // Also update legacy state for backward compatibility
          if (data.workflowId === sanitizedWorkflow.id) {
            setCurrentStepId(data.stepId);
            setWaitingStepId(null);
            setWaitingPhase(null);
          }
        }
      };

      const onWait = (data: any) => {
        if (data.deviceId === deviceObj.id && data.workflowId) {
          console.log("[Workflow] Step waiting:", data.stepId, "phase:", data.phase, "in workflow:", data.workflowId);

          // Update the step map for this workflow
          setWorkflowStepMap(prev => ({
            ...prev,
            [data.workflowId]: {
              stepId: data.stepId,
              isWaiting: true,
              waitPhase: data.phase
            }
          }));

          // Also update legacy state for backward compatibility
          if (data.workflowId === sanitizedWorkflow.id) {
            setWaitingStepId(data.stepId);
            setWaitingPhase(data.phase);
          }
        }
      };

      const onPaused = (data: any) => {
        console.log('[Workflow] onPaused event received:', data, 'expected deviceId:', deviceObj.id);
        if (data.deviceId === deviceObj.id) {
          console.log('[Workflow] Setting isPaused = true');
          setIsPaused(true);
        }
      };

      const onResumed = (data: any) => {
        console.log('[Workflow] onResumed event received:', data, 'expected deviceId:', deviceObj.id);
        if (data.deviceId === deviceObj.id) {
          console.log('[Workflow] Setting isPaused = false');
          setIsPaused(false);
        }
      };

      const onRuntimeUpdate = (data: any) => {
        if (data.deviceId === deviceObj.id) {
          setRuntimeContext({
            deviceId: data.deviceId,
            workflowId: data.workflowId,
            workflowName: data.workflowName,
            status: data.status,
            currentStepId: data.currentStepId || '',
            currentStepName: data.currentStepName || '',
            currentStepType: data.currentStepType || '',
            stepsExecuted: data.stepsExecuted || 0,
            stepsTotal: data.stepsTotal || 0,
            isPaused: data.isPaused || false,
            variables: data.variables || {},
          });
        }
      };

      eventCancels.push(
        EventsOn("workflow-started", onSubStarted),
        EventsOn("workflow-completed", onComplete),
        EventsOn("workflow-error", onError),
        EventsOn("workflow-step-running", onStep),
        EventsOn("workflow-step-waiting", onWait),
        EventsOn("task-paused", onPaused),
        EventsOn("task-resumed", onResumed),
        EventsOn("workflow-runtime-update", onRuntimeUpdate),
      );
    });

    try {
      setIsPaused(false); // Reset pause state when starting
      setWorkflowStepMap({}); // Clear all workflow step tracking
      await (window as any).go.main.App.RunWorkflow(deviceObj, sanitizedWorkflow);
      await executionPromise;
      setExecutionLogs(prev => [...prev, `[${new Date().toLocaleTimeString()}] ${t("workflow.completed")}`]);
    } catch (err) {
      setExecutionLogs(prev => [...prev, `[${new Date().toLocaleTimeString()}] ${t("workflow.error")}: ${err}`]);
      message.error(String(err));
    } finally {
      // Note: Event cleanup is handled by cleanUp() in onComplete/onError handlers
      setIsRunning(false);
      setIsPaused(false);
      setRunningWorkflowIds([]);
      setCurrentStepId(null);
      setWorkflowStepMap({}); // Clear all workflow step tracking
      clearRuntimeContext(); // Clear runtime context when workflow ends
      runningDeviceIdRef.current = null; // Clear running device reference
    }
  };

  const handleOpenVariablesModal = () => {
    if (!selectedWorkflow) return;
    const vars = selectedWorkflow.variables || {};
    setTempVariables(Object.entries(vars).map(([key, value]) => ({ key, value })));
    setVariablesModalVisible(true);
  };

  const handleSaveVariables = async () => {
    if (!selectedWorkflow) return;
    const variables: Record<string, string> = {};
    tempVariables.forEach(({ key, value }) => {
      if (key.trim()) {
        variables[key.trim()] = value;
      }
    });

    const updatedWorkflow = {
      ...selectedWorkflow,
      variables,
      updatedAt: new Date().toISOString(),
    };

    try {
      // Save to backend immediately
      await (window as any).go.main.App.SaveWorkflow(updatedWorkflow);

      // Update local state
      setSelectedWorkflow(updatedWorkflow);

      // Update workflows list
      setWorkflows((prev: Workflow[]) => {
        const idx = prev.findIndex((w: Workflow) => w.id === updatedWorkflow.id);
        if (idx >= 0) {
          const newList = [...prev];
          newList[idx] = updatedWorkflow;
          return newList;
        }
        return prev;
      });

      setVariablesModalVisible(false);
      message.success(t("workflow.variables_updated"));
    } catch (err) {
      message.error(`Failed to save variables: ${err}`);
    }
  };

  const handleStopWorkflow = async () => {
    // Use the device that started the workflow, not the currently selected one
    const deviceId = runningDeviceIdRef.current || selectedDevice;
    const deviceObj = useDeviceStore.getState().devices.find(d => d.id === deviceId);
    if (!deviceObj) {
      message.error(t("workflow.error_device_not_found"));
      return;
    }

    try {
      await (window as any).go.main.App.StopWorkflow(deviceObj);

      // CRITICAL: Clean up event listeners when manually stopping
      if (cleanupEventsRef.current) {
        cleanupEventsRef.current();
      }

      setIsRunning(false);
      setIsPaused(false);
      setCurrentStepId(null);
      setRunningWorkflowIds([]);
      setWorkflowStepMap({});
      clearRuntimeContext();
      runningDeviceIdRef.current = null;
      setExecutionLogs(prev => [...prev, `[${new Date().toLocaleTimeString()}] ${t("workflow.stopped")}`]);
    } catch (err) {
      message.error(String(err));
    }
  };

  const handleDeleteWorkflow = async (id: string) => {
    try {
      await (window as any).go.main.App.DeleteWorkflow(id);
      message.success(t("workflow.deleted"));
      if (selectedWorkflow?.id === id) {
        setSelectedWorkflow(null);
      }
      loadWorkflows();
    } catch (err) {
      message.error(String(err));
    }
  };

  const handleRenameWorkflow = async (id: string, newName: string) => {
    if (!newName.trim()) {
      message.error(t("workflow.name_required"));
      return;
    }

    const workflow = workflows.find(w => w.id === id);
    if (!workflow) return;

    const updatedWorkflow = {
      ...workflow,
      name: newName.trim(),
      updatedAt: new Date().toISOString(),
    };

    try {
      await (window as any).go.main.App.SaveWorkflow(updatedWorkflow);

      // Update local state
      setWorkflows((prev: Workflow[]) => {
        const idx = prev.findIndex((w: Workflow) => w.id === id);
        if (idx >= 0) {
          const newList = [...prev];
          newList[idx] = updatedWorkflow;
          return newList;
        }
        return prev;
      });

      // Update selected workflow if it's the one being renamed
      if (selectedWorkflow?.id === id) {
        setSelectedWorkflow(updatedWorkflow);
      }

      setEditingWorkflowId(null);
      setEditingWorkflowName("");
      message.success(t("workflow.renamed"));
    } catch (err) {
      message.error(`Failed to rename workflow: ${err}`);
    }
  };

  const handlePauseWorkflow = async () => {
    const deviceId = runningDeviceIdRef.current || selectedDevice;
    console.log('[Workflow] Pause - deviceId:', deviceId, 'ref:', runningDeviceIdRef.current, 'selected:', selectedDevice);
    if (!deviceId) {
      message.error('No device ID available');
      return;
    }
    try {
      await (window as any).go.main.App.PauseTask(deviceId);
    } catch (err) {
      console.error('[Workflow] Pause error:', err);
      message.error(`Pause failed: ${err}`);
    }
  };

  const handleResumeWorkflow = async () => {
    console.log('[Workflow] handleResumeWorkflow CALLED');
    const deviceId = runningDeviceIdRef.current || selectedDevice;
    console.log('[Workflow] Resume - deviceId:', deviceId, 'ref:', runningDeviceIdRef.current, 'selected:', selectedDevice);
    if (!deviceId) {
      message.error('No device ID available');
      return;
    }
    try {
      console.log('[Workflow] Calling ResumeTask with deviceId:', deviceId);
      await (window as any).go.main.App.ResumeTask(deviceId);
      console.log('[Workflow] ResumeTask completed successfully');
    } catch (err) {
      console.error('[Workflow] Resume error:', err);
      message.error(`Resume failed: ${err}`);
    }
  };

  const handleStepNextWorkflow = async () => {
    console.log('[Workflow] handleStepNextWorkflow CALLED');
    const deviceId = runningDeviceIdRef.current || selectedDevice;
    console.log('[Workflow] StepNext - deviceId:', deviceId, 'ref:', runningDeviceIdRef.current, 'selected:', selectedDevice);
    if (!deviceId) {
      message.error('No device ID available');
      return;
    }
    try {
      console.log('[Workflow] Calling StepNextWorkflow with deviceId:', deviceId);
      await (window as any).go.main.App.StepNextWorkflow(deviceId);
      console.log('[Workflow] StepNextWorkflow completed successfully');
    } catch (err) {
      console.error('[Workflow] StepNext error:', err);
      message.error(`Step failed: ${err}`);
    }
  };

  /**
   * Handles step reordering from the list view.
   * Rebuilds the success connection chain based on the new order,
   * preserving branch/error connections unchanged.
   */
  const handleStepReorder = useCallback(async (orderedSteps: WorkflowStep[]) => {
    if (!selectedWorkflow) return;

    // Rebuild successStepId chain for the new linear order.
    // Branch true/false/error connections and error connections are preserved.
    const updatedSteps = orderedSteps.map((step, index) => {
      const nextStep = index < orderedSteps.length - 1 ? orderedSteps[index + 1] : null;

      // For branch nodes, keep existing true/false/error connections; only update successStepId for non-branch
      if (step.type === 'branch') {
        return { ...step }; // Branch connections are managed via the canvas
      }

      return {
        ...step,
        connections: {
          ...step.connections,
          successStepId: nextStep ? nextStep.id : undefined,
          // Preserve error connections
          errorStepId: step.connections?.errorStepId,
        },
      };
    });

    // Also need to include steps from the original workflow that might not be in the linear order
    // (disconnected nodes, etc.) — just preserve them unchanged
    const orderedIds = new Set(updatedSteps.map(s => s.id));
    const extraSteps = selectedWorkflow.steps.filter(s => !orderedIds.has(s.id));

    const allSteps = [...updatedSteps, ...extraSteps];

    // Update workflow with new step order and connections
    const updatedWorkflow: Workflow = {
      ...selectedWorkflow,
      steps: allSteps,
      updatedAt: new Date().toISOString(),
    };

    try {
      await (window as any).go.main.App.SaveWorkflow(updatedWorkflow);

      // Update local state
      skipNextRenderRef.current = true;
      updateWorkflow(selectedWorkflow.id, updatedWorkflow);
      selectWorkflow(updatedWorkflow.id);

      // Rebuild React Flow nodes/edges from the updated workflow
      // This is needed because the graph connections changed
      const newNodes = allSteps.map((step, index) => {
        const stepState = workflowStepMap[selectedWorkflow.id];
        const isCurrent = stepState?.stepId === step.id && !stepState?.isWaiting;
        const isWaiting = stepState?.stepId === step.id && stepState?.isWaiting;
        const waitPhase = stepState?.waitPhase;

        // If step doesn't have layout positions, auto-assign grid positions
        const posX = step.layout?.posX ?? 250;
        const posY = step.layout?.posY ?? (index * 120 + 50);

        return {
          id: step.id,
          type: 'workflowNode' as const,
          position: { x: posX, y: posY },
          data: {
            step: { ...step, layout: { ...step.layout, posX, posY } },
            label: step.name || t(`workflow.step_type.${step.type}`),
            isCurrent,
            isWaiting,
            waitingPhase: waitPhase,
            onExecuteStep: handleExecuteSingleStep,
            canExecute: !!selectedDevice && !isRunning,
          },
        };
      });

      // Rebuild edges from updated connections
      const newEdges: any[] = [];
      allSteps.forEach(step => {
        const conn = step.connections;
        const handles = step.layout?.handles;

        if (step.type === 'branch') {
          if (conn?.trueStepId) {
            newEdges.push({
              id: `e-${step.id}-${conn.trueStepId}-true`,
              source: step.id,
              target: conn.trueStepId,
              type: 'smoothstep',
              sourceHandle: handles?.true?.sourceHandle || 'true',
              label: t('workflow.branch_true'),
              style: { stroke: token.colorSuccess },
              labelStyle: { fill: token.colorSuccess, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: token.colorSuccess },
            });
          }
          if (conn?.falseStepId) {
            newEdges.push({
              id: `e-${step.id}-${conn.falseStepId}-false`,
              source: step.id,
              target: conn.falseStepId,
              type: 'smoothstep',
              sourceHandle: handles?.false?.sourceHandle || 'false',
              label: t('workflow.branch_false'),
              style: { stroke: token.colorError },
              labelStyle: { fill: token.colorError, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: token.colorError },
            });
          }
          if (conn?.errorStepId) {
            newEdges.push({
              id: `e-${step.id}-${conn.errorStepId}-error`,
              source: step.id,
              target: conn.errorStepId,
              type: 'smoothstep',
              sourceHandle: handles?.error?.sourceHandle || 'error',
              label: t('workflow.error') || 'Error',
              style: { stroke: token.colorWarning },
              labelStyle: { fill: token.colorWarning, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: token.colorWarning },
            });
          }
        } else {
          if (conn?.successStepId) {
            newEdges.push({
              id: `e-${step.id}-${conn.successStepId}-success`,
              source: step.id,
              target: conn.successStepId,
              type: 'smoothstep',
              sourceHandle: handles?.success?.sourceHandle || 'success',
              label: step.type === 'start' ? undefined : t('workflow.success'),
              style: step.type === 'start' ? undefined : { stroke: token.colorSuccess },
              labelStyle: step.type === 'start' ? undefined : { fill: token.colorSuccess, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: step.type === 'start' ? undefined : token.colorSuccess },
            });
          }
          if (conn?.errorStepId) {
            newEdges.push({
              id: `e-${step.id}-${conn.errorStepId}-error`,
              source: step.id,
              target: conn.errorStepId,
              type: 'smoothstep',
              sourceHandle: handles?.error?.sourceHandle || 'error',
              label: t('workflow.error') || 'Error',
              style: { stroke: token.colorWarning },
              labelStyle: { fill: token.colorWarning, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: token.colorWarning },
            });
          }
        }
      });

      setNodes(newNodes);
      setEdges(newEdges);
      message.success(t("workflow.steps_reordered"));
    } catch (err) {
      message.error(String(err));
    }
  }, [selectedWorkflow, workflowStepMap, t, token, handleExecuteSingleStep, selectedDevice, isRunning, setNodes, setEdges, updateWorkflow, selectWorkflow]);

  /**
   * Handle step click from list view - open the edit drawer.
   */
  const handleListStepClick = useCallback((step: WorkflowStep) => {
    // Find the corresponding node and simulate a node click
    const node = nodes.find(n => n.id === step.id);
    if (node) {
      // Reuse handleNodeClick logic
      setEditingNodeId(step.id);
      const selector = step.element?.selector || step.branch?.selector || step.readToVariable?.selector;
      const conditionType = step.branch?.condition || 'exists';
      
      let value: string | undefined;
      if (step.app?.packageName) value = step.app.packageName;
      else if (step.wait?.durationMs) value = String(step.wait.durationMs);
      else if (step.script?.scriptName) value = step.script.scriptName;
      else if (step.variable?.value) value = step.variable.value;
      else if (step.adb?.command) value = step.adb.command;
      else if (step.workflow?.workflowId) value = step.workflow.workflowId;
      else if (step.element?.inputText) value = step.element.inputText;
      else if (step.element?.swipeDir) value = step.element.swipeDir;
      else if (step.branch?.expectedValue) value = step.branch.expectedValue;
      else if (step.readToVariable?.defaultValue) value = step.readToVariable.defaultValue;
      
      stepForm.resetFields();
      stepForm.setFieldsValue({
        type: step.type,
        name: step.name,
        selectorType: selector?.type,
        selectorValue: selector?.value,
        conditionType,
        value,
        timeout: step.common?.timeout,
        onError: step.common?.onError,
        loop: step.common?.loop,
        preWait: step.common?.preWait,
        postDelay: step.common?.postDelay,
        swipeDistance: step.element?.swipeDistance || step.swipe?.distance,
        swipeDuration: step.element?.swipeDuration || step.swipe?.duration,
        x: step.tap?.x || step.swipe?.x,
        y: step.tap?.y || step.swipe?.y,
        x2: step.swipe?.x2,
        y2: step.swipe?.y2,
        variableName: step.readToVariable?.variableName || step.variable?.name,
        attribute: step.readToVariable?.attribute || 'text',
        regex: step.readToVariable?.regex,
        defaultValue: step.readToVariable?.defaultValue,
        sessionName: step.session?.sessionName,
        logcatEnabled: step.session?.logcatEnabled,
        logcatPackageName: step.session?.logcatPackageName,
        logcatPreFilter: step.session?.logcatPreFilter,
        logcatExcludeFilter: step.session?.logcatExcludeFilter,
        recordingEnabled: step.session?.recordingEnabled,
        recordingQuality: step.session?.recordingQuality || 'medium',
        proxyEnabled: step.session?.proxyEnabled,
        proxyPort: step.session?.proxyPort || 8080,
        proxyMitmEnabled: step.session?.proxyMitmEnabled,
        monitorEnabled: step.session?.monitorEnabled,
        sessionStatus: step.session?.status || 'completed',
      });
      setDrawerVisible(true);
    }
  }, [nodes, stepForm, setEditingNodeId, setDrawerVisible]);

  return (
    <div style={{ height: "100%", display: "flex", flexDirection: "column" }}>
      <div style={{ padding: "12px 24px", borderBottom: `1px solid ${token.colorBorderSecondary}`, display: "flex", justifyContent: "space-between", alignItems: "center", background: token.colorBgContainer }}>
        <Space>
          <Title level={4} style={{ margin: 0 }}>{t("workflow.title")}</Title>
          <Tag color="green">{viewMode === 'canvas' ? t("workflow.visual_editor") : t("workflow.list_editor")}</Tag>
        </Space>
        <Space>
          <DeviceSelector />
          {selectedWorkflow && (
            <>
              {/* View Mode Toggle */}
              <Button.Group>
                <Tooltip title={t("workflow.canvas_view")}>
                  <Button
                    icon={<CanvasOutlined />}
                    type={viewMode === 'canvas' ? 'primary' : 'default'}
                    onClick={() => setViewMode('canvas')}
                  />
                </Tooltip>
                <Tooltip title={t("workflow.list_view")}>
                  <Button
                    icon={<UnorderedListOutlined />}
                    type={viewMode === 'list' ? 'primary' : 'default'}
                    onClick={() => setViewMode('list')}
                  />
                </Tooltip>
              </Button.Group>

              {viewMode === 'canvas' && (
                <>
                  <Tooltip title={t("workflow.auto_layout") + " (Vertical)"}>
                    <Button icon={<PicCenterOutlined />} onClick={() => handleAutoLayout('TB')} />
                  </Tooltip>
                  <Tooltip title={t("workflow.auto_layout") + " (Horizontal)"}>
                    <Button icon={<PicRightOutlined />} onClick={() => handleAutoLayout('LR')} />
                  </Tooltip>
                </>
              )}
              <Button icon={<SettingOutlined />} onClick={handleOpenVariablesModal}>
                {t("workflow.variables")}
              </Button>
              <Button icon={<SaveOutlined />} onClick={() => handleSaveGraph()}>
                {t("workflow.save")}
              </Button>
              {isRunning ? (
                <Space>
                  {isPaused ? (
                    <>
                      <Button icon={<PlayCircleOutlined />} onClick={handleResumeWorkflow}>
                        {t("workflow.resume")}
                      </Button>
                      <Button icon={<StepForwardOutlined />} onClick={handleStepNextWorkflow}>
                        {t("workflow.step_next")}
                      </Button>
                    </>
                  ) : (
                    <Button icon={<PauseCircleOutlined />} onClick={handlePauseWorkflow}>
                      {t("workflow.pause")}
                    </Button>
                  )}
                  <Button danger icon={<StopOutlined />} onClick={handleStopWorkflow}>
                    {t("workflow.stop")}
                  </Button>
                </Space>
              ) : (
                <Button type="primary" icon={<PlayCircleOutlined />} onClick={handleRunWorkflow} disabled={!selectedDevice}>
                  {t("workflow.run")}
                </Button>
              )}
            </>
          )}
        </Space>
      </div>

      <div style={{ flex: 1, display: "flex", overflow: "hidden" }}>
        <div style={{ width: 250, borderRight: `1px solid ${token.colorBorderSecondary}`, display: "flex", flexDirection: "column", background: token.colorBgContainer }}>
          <div style={{ padding: 12, borderBottom: `1px solid ${token.colorBorderSecondary}`, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Text strong>{t("workflow.workflows")}</Text>
            <Button type="primary" size="small" icon={<PlusOutlined />} onClick={handleCreateWorkflow} />
          </div>
          <div style={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
            {workflows.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("workflow.no_workflows")} />
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', paddingBottom: 60 }}>
                {workflows.map(workflow => (
                  <div
                    key={workflow.id}
                    onClick={() => {
                      if (editingWorkflowId !== workflow.id && selectedWorkflow?.id !== workflow.id) {
                        setSelectedWorkflow(workflow);
                      }
                    }}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '12px 16px',
                      cursor: editingWorkflowId === workflow.id ? 'default' : 'pointer',
                      borderLeft: selectedWorkflow?.id === workflow.id ? `3px solid ${token.colorPrimary}` : '3px solid transparent',
                      backgroundColor: selectedWorkflow?.id === workflow.id ? token.colorPrimaryBg : undefined,
                      borderBottom: `1px solid ${token.colorBorderSecondary}`,
                      transition: 'all 0.2s'
                    }}
                  >
                    <div style={{ width: '100%', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <div style={{ flex: 1, overflow: 'hidden', marginRight: 8 }}>
                        {editingWorkflowId === workflow.id ? (
                          <Input
                            size="small"
                            value={editingWorkflowName}
                            onChange={(e) => setEditingWorkflowName(e.target.value)}
                            onPressEnter={(e) => {
                              e.stopPropagation();
                              handleRenameWorkflow(workflow.id, editingWorkflowName);
                            }}
                            onClick={(e) => e.stopPropagation()}
                            autoFocus
                            style={{ fontWeight: 500 }}
                          />
                        ) : (
                          <div
                            style={{ fontWeight: 500, display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}
                            onDoubleClick={(e) => {
                              e.stopPropagation();
                              setEditingWorkflowId(workflow.id);
                              setEditingWorkflowName(workflow.name);
                            }}
                          >
                            <Tooltip title={workflow.name} placement="topLeft" mouseEnterDelay={0.5}>
                              <Text strong style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                {workflow.name}
                              </Text>
                            </Tooltip>
                            {runningWorkflowIds.includes(workflow.id) && (
                              <LoadingOutlined style={{ color: token.colorPrimary, fontSize: 12 }} />
                            )}
                          </div>
                        )}
                        {editingWorkflowId !== workflow.id && (
                          <div style={{ fontSize: 12, color: token.colorTextSecondary, marginTop: 2 }}>{workflow.steps.length} {t("workflow.steps_count") || "steps"}</div>
                        )}
                      </div>

                      <div style={{ display: 'flex', gap: 4 }}>
                        {editingWorkflowId === workflow.id ? (
                          <>
                            <Button
                              key="save"
                              type="text"
                              size="small"
                              icon={<CheckCircleOutlined />}
                              onClick={(e) => {
                                e.stopPropagation();
                                handleRenameWorkflow(workflow.id, editingWorkflowName);
                              }}
                            />
                            <Button
                              key="cancel"
                              type="text"
                              size="small"
                              icon={<StopOutlined />}
                              onClick={(e) => {
                                e.stopPropagation();
                                setEditingWorkflowId(null);
                                setEditingWorkflowName("");
                              }}
                            />
                          </>
                        ) : (
                          <>
                            <Button
                              key="edit"
                              type="text"
                              size="small"
                              icon={<EditOutlined />}
                              onClick={(e) => {
                                e.stopPropagation();
                                setEditingWorkflowId(workflow.id);
                                setEditingWorkflowName(workflow.name);
                              }}
                            />
                            <Popconfirm
                              key="del"
                              title={t("workflow.delete_confirm")}
                              onConfirm={(e) => {
                                e?.stopPropagation();
                                handleDeleteWorkflow(workflow.id);
                              }}
                            >
                              <Button
                                type="text"
                                size="small"
                                danger
                                icon={<DeleteOutlined />}
                                onClick={(e) => e.stopPropagation()}
                              />
                            </Popconfirm>
                          </>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        <div style={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
          {selectedWorkflow ? (
            viewMode === 'list' ? (
              <WorkflowStepListView
                workflow={selectedWorkflow}
                currentStepId={currentStepId}
                editingNodeId={editingNodeId}
                onStepClick={handleListStepClick}
                onReorder={handleStepReorder}
              />
            ) : (
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodesChange={onNodesChangeWithProtection}
              onEdgesChange={onEdgesChange}
              onConnect={onConnect}
              onNodeClick={handleNodeClick}
              nodeTypes={nodeTypes}
              edgeTypes={edgeTypes}
              fitView
              attributionPosition="bottom-right"
              snapToGrid={true}
              snapGrid={[15, 15]}
              defaultEdgeOptions={{ type: 'smoothstep', animated: false }}
              style={{
                backgroundColor: token.colorBgContainer,
                // @ts-ignore
                '--xy-controls-button-background-color': token.colorBgElevated,
                '--xy-controls-button-background-color-hover': token.colorBgTextHover,
                '--xy-controls-button-border-color': token.colorBorder,
                '--xy-controls-button-color': token.colorText, // Icon color
                '--xy-minimap-background-color': token.colorBgElevated,
                '--xy-minimap-mask-background-color': token.colorFillTertiary,
                '--xy-minimap-node-background-color': token.colorFill,
                '--xy-minimap-node-stroke-color': 'transparent',
              }}
              proOptions={{ hideAttribution: true }}
            >
              <Background
                color={token.colorTextQuaternary}
                gap={16}
                variant={BackgroundVariant.Dots}
                style={{ backgroundColor: token.colorBgLayout }}
              />
              <Controls />
              <MiniMap style={{ height: 120 }} zoomable pannable />

              {/* Tool Panel fixed to Top-Left of the Graph Canvas */}
              <Panel position="top-left" style={{ margin: 12 }}>
                <div style={{
                  width: 260,
                  background: token.colorBgContainer,
                  borderRadius: 8,
                  padding: 8,
                  boxShadow: token.boxShadowSecondary,
                  border: `1px solid ${token.colorBorderSecondary}`
                }}>
                  <Collapse
                    ghost
                    size="small"
                    defaultActiveKey={['0', '1', '2', '3']}
                    items={[
                      {
                        key: '0',
                        label: t("workflow.category.coordinate_actions"),
                        children: (
                          <Space wrap size={[8, 8]}>
                            {STEP_TYPES.COORDINATE_ACTIONS.map(s => (
                              <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                                <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                              </Tooltip>
                            ))}
                          </Space>
                        )
                      },
                      {
                        key: '1',
                        label: t("workflow.category.element_actions"),
                        children: (
                          <Space wrap size={[8, 8]}>
                            {STEP_TYPES.ELEMENT_ACTIONS.map(s => (
                              <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                                <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                              </Tooltip>
                            ))}
                          </Space>
                        )
                      },
                      {
                        key: '2',
                        label: t("workflow.category.wait_conditions"),
                        children: (
                          <Space wrap size={[8, 8]}>
                            {STEP_TYPES.WAIT_CONDITIONS.map(s => (
                              <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                                <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                              </Tooltip>
                            ))}
                          </Space>
                        )
                      },
                      {
                        key: '3',
                        label: t("workflow.category.flow_control"),
                        children: (
                          <Space wrap size={[8, 8]}>
                            {STEP_TYPES.FLOW_CONTROL.map(s => (
                              <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                                <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                              </Tooltip>
                            ))}
                          </Space>
                        )
                      },
                      {
                        key: 'script_actions',
                        label: t("workflow.category.script_actions"),
                        children: (
                          <Space wrap size={[8, 8]}>
                            {STEP_TYPES.SCRIPT_ACTIONS.map(s => (
                              <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                                <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                              </Tooltip>
                            ))}
                          </Space>
                        )
                      },
                      {
                        key: '4',
                        label: t("workflow.category.system_actions"),
                        children: (
                          <Space wrap size={[8, 8]}>
                            {STEP_TYPES.SYSTEM_ACTIONS.map(s => (
                              <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                                <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                              </Tooltip>
                            ))}
                          </Space>
                        )
                      },
                      {
                        key: '5',
                        label: t("workflow.category.nested"),
                        children: (
                          <Space wrap size={[8, 8]}>
                            {STEP_TYPES.NESTED.map(s => (
                              <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                                <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                              </Tooltip>
                            ))}
                          </Space>
                        )
                      },
                      {
                        key: '6',
                        label: t("workflow.category.app_actions"),
                        children: (
                          <Space wrap size={[8, 8]}>
                            {STEP_TYPES.APP_ACTIONS.map(s => (
                              <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                                <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                              </Tooltip>
                            ))}
                          </Space>
                        )
                      },
                      {
                        key: '7',
                        label: t("workflow.category.session_control"),
                        children: (
                          <Space wrap size={[8, 8]}>
                            {STEP_TYPES.SESSION_CONTROL.map(s => (
                              <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                                <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                              </Tooltip>
                            ))}
                          </Space>
                        )
                      }
                    ]}
                  />
                </div>
              </Panel>

              {/* Runtime Context Panel - shows when workflow is running */}
              {isRunning && runtimeContext && (
                <Panel position="top-right" style={{ margin: 12 }}>
                  <div style={{
                    width: 280,
                    background: token.colorBgContainer,
                    borderRadius: 8,
                    padding: 12,
                    boxShadow: token.boxShadowSecondary,
                    border: `1px solid ${token.colorBorderSecondary}`
                  }}>
                    <div style={{ marginBottom: 8 }}>
                      <Text strong style={{ fontSize: 14 }}>{t("workflow.runtime_context")}</Text>
                      {runtimeContext.isPaused && (
                        <Tag color="orange" style={{ marginLeft: 8 }}>{t("workflow.paused")}</Tag>
                      )}
                    </div>
                    
                    <div style={{ marginBottom: 12 }}>
                      <Progress 
                        percent={runtimeContext.stepsTotal > 0 ? Math.round((runtimeContext.stepsExecuted / runtimeContext.stepsTotal) * 100) : 0}
                        size="small"
                        status={runtimeContext.isPaused ? "exception" : "active"}
                      />
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {t("workflow.progress")}: {runtimeContext.stepsExecuted} / {runtimeContext.stepsTotal}
                      </Text>
                    </div>

                    <div style={{ marginBottom: 8 }}>
                      <Text type="secondary" style={{ fontSize: 12 }}>{t("workflow.current_step")}:</Text>
                      <div style={{ 
                        background: token.colorFillTertiary, 
                        padding: '4px 8px', 
                        borderRadius: 4,
                        marginTop: 4
                      }}>
                        <Text style={{ fontSize: 13 }}>
                          {runtimeContext.currentStepName || runtimeContext.currentStepId || '-'}
                        </Text>
                        {runtimeContext.currentStepType && (
                          <Tag style={{ marginLeft: 8 }}>{runtimeContext.currentStepType}</Tag>
                        )}
                      </div>
                    </div>

                    {Object.keys(runtimeContext.variables || {}).length > 0 && (
                      <div>
                        <Text type="secondary" style={{ fontSize: 12 }}>{t("workflow.variables")}:</Text>
                        <div style={{ 
                          background: token.colorFillTertiary, 
                          padding: '4px 8px', 
                          borderRadius: 4,
                          marginTop: 4,
                          maxHeight: 120,
                          overflowY: 'auto'
                        }}>
                          {Object.entries(runtimeContext.variables).map(([key, value]) => (
                            <div key={key} style={{ fontSize: 12, fontFamily: 'monospace' }}>
                              <Text type="secondary">{key}:</Text> <Text>{value}</Text>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                </Panel>
              )}
            </ReactFlow>
            )
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', color: token.colorTextSecondary }}>
              <BranchesOutlined style={{ fontSize: 48, marginBottom: 16 }} />
              <Text>{t("workflow.select_or_create")}</Text>
            </div>
          )}
        </div>

        {drawerVisible && (
          <div style={{
            width: 450,
            borderLeft: `1px solid ${token.colorBorderSecondary}`,
            background: token.colorBgContainer,
            display: 'flex',
            flexDirection: 'column',
            height: '100%'
          }}>
            <div style={{
              padding: '16px 24px',
              borderBottom: `1px solid ${token.colorBorderSecondary}`,
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center'
            }}>
              <Text strong style={{ fontSize: 16 }}>{t("workflow.edit_step")}</Text>
              <Button
                type="text"
                icon={<CloseOutlined />}
                onClick={() => {
                  // Ensure the latest form values are saved before closing
                  if (editingNodeId) {
                    handleUpdateStep(stepForm.getFieldsValue());
                  }
                  setDrawerVisible(false);
                  message.success(t("workflow.step_updated"));
                }}
              />
            </div>

            <div style={{ flex: 1, overflowY: 'auto', padding: 24 }}>
              {editingNodeId && (
                <>
                  <Form
                    layout="vertical"
                    form={stepForm}
                    onValuesChange={(_, allValues) => handleUpdateStep(allValues)}
                  >
                    <Form.Item name="type" label={t("workflow.step_type_label")}>
                      <Select
                        disabled
                        options={Object.values(STEP_TYPES).flat().map(t => ({ label: t.key, value: t.key }))}
                      />
                    </Form.Item>
                    <Form.Item
                      noStyle
                      shouldUpdate={(prev, cur) => prev.type !== cur.type}
                    >
                      {({ getFieldValue }) => {
                        const type = getFieldValue('type');
                        const isSetVariable = type === 'set_variable';
                        const label = isSetVariable ? t("workflow.variable_name") : t("workflow.name");
                        return (
                          <>
                            {/* 变量名字段（仅 set_variable 使用） */}
                            {isSetVariable && (
                              <Form.Item name="variableName" label={t("workflow.variable_name")} rules={[{ required: true, message: t("workflow.variable_name_required") || "Variable name is required" }]}>
                                <Input placeholder={t("workflow.variable_name_placeholder") || "myVariable"} />
                              </Form.Item>
                            )}
                            {/* 步骤名称字段（所有类型都有，但 set_variable 是可选的） */}
                            <Form.Item name="name" label={isSetVariable ? t("workflow.step_name") : label}>
                              <Input placeholder={isSetVariable ? t("workflow.step_name_optional") || "Optional step name" : label} />
                            </Form.Item>
                          </>
                        );
                      }}
                    </Form.Item>

                    <Form.Item
                      noStyle
                      shouldUpdate={(prev, cur) => prev.type !== cur.type || prev.conditionType !== cur.conditionType}
                    >
                      {({ getFieldValue }) => {
                        const type = getFieldValue('type');
                        const isBranch = type === 'branch';
                        const isReadToVariable = type === 'read_to_variable';
                        const conditionType = getFieldValue('conditionType') || 'exists';
                        const needsSelector = ['click_element', 'long_click_element', 'input_text', 'swipe_element', 'wait_element', 'wait_gone', 'assert_element', 'branch', 'read_to_variable'].includes(type);
                        const isAppAction = ['launch_app', 'stop_app', 'clear_app', 'open_settings'].includes(type);
                        const needsValue = ['set_variable', 'input_text', 'swipe_element', 'wait', 'adb', 'script', 'run_workflow'].includes(type) || isAppAction;
                        const isWorkflow = type === 'run_workflow';

                        // For branch conditions, determine if we need value field
                        const branchNeedsValue = isBranch && ['text_equals', 'text_contains', 'variable_equals'].includes(conditionType);
                        const branchNeedsSelector = isBranch && conditionType !== 'variable_equals';
                        const actualNeedsSelector = isBranch ? branchNeedsSelector : needsSelector;

                        return (
                          <>
                            {isBranch && (
                              <div style={{
                                padding: '8px 12px',
                                marginBottom: 12,
                                background: token.colorInfoBg,
                                border: `1px solid ${token.colorInfoBorder}`,
                                borderRadius: 6
                              }}>
                                <Text style={{ fontSize: 12, color: token.colorTextSecondary }}>
                                  {t("workflow.branch_description")}
                                </Text>
                              </div>
                            )}

                            {actualNeedsSelector && (
                              <>
                                <Form.Item label={
                                  isBranch && conditionType === 'variable_equals' ? t("workflow.variable_name") :
                                    isBranch ? t("workflow.branch_condition") :
                                      t("workflow.selector_type")
                                }>
                                  <div style={{ display: 'flex', gap: 8 }}>
                                    <Form.Item name="selectorType" noStyle>
                                      <Select style={{ flex: 1 }} options={[
                                        { label: 'Text', value: 'text' },
                                        { label: 'Resource ID', value: 'id' },
                                        { label: 'XPath', value: 'xpath' },
                                        { label: 'Content Desc', value: 'description' },
                                        { label: 'Class', value: 'class' },
                                        { label: 'Bounds', value: 'bounds' },
                                      ]} />
                                    </Form.Item>
                                    {!isBranch && <Button icon={<AimOutlined />} onClick={() => setElementPickerVisible(true)} />}
                                  </div>
                                </Form.Item>
                                <Form.Item name="selectorValue" label={
                                  isBranch && conditionType === 'variable_equals' ? t("workflow.variable_name") :
                                    t("workflow.selector_value")
                                }>
                                  <AutoComplete
                                    options={variableOptions}
                                    filterOption={(inputValue, option) =>
                                      (option?.value as string || '').toUpperCase().indexOf(inputValue.toUpperCase()) !== -1
                                    }
                                  >
                                    <Input.TextArea
                                      placeholder={
                                        isBranch && conditionType === 'variable_equals' ?
                                          t("workflow.var_key_placeholder") :
                                          t("workflow.selector_placeholder")
                                      }
                                      autoSize={{ minRows: 1, maxRows: 6 }}
                                    />
                                  </AutoComplete>
                                </Form.Item>
                              </>
                            )}

                            {isBranch && (
                              <Form.Item name="conditionType" label={t("workflow.condition_type")}>
                                <Select
                                  placeholder={t("workflow.select_condition_type")}
                                  options={[
                                    { label: t("workflow.condition.exists"), value: "exists" },
                                    { label: t("workflow.condition.not_exists"), value: "not_exists" },
                                    { label: t("workflow.condition.text_equals"), value: "text_equals" },
                                    { label: t("workflow.condition.text_contains"), value: "text_contains" },
                                    { label: t("workflow.condition.variable_equals"), value: "variable_equals" },
                                  ]}
                                />
                              </Form.Item>
                            )}

                            {isWorkflow && (
                              <Form.Item name="value" label={t("workflow.select_workflow")} rules={[{ required: true }]}>
                                <Select
                                  placeholder={t("workflow.select_workflow")}
                                  options={workflows
                                    .filter(w => w.id !== selectedWorkflow?.id)
                                    .map(w => ({ label: w.name, value: w.id }))
                                  }
                                />
                              </Form.Item>
                            )}

                            {(needsValue || branchNeedsValue) && (
                              <Form.Item name="value" label={
                                isBranch && ['text_equals', 'text_contains'].includes(conditionType) ? t("workflow.expected_text") :
                                  isBranch && conditionType === 'variable_equals' ? t("workflow.expected_value") :
                                    type === 'swipe_element' ? t("workflow.swipe_direction") :
                                      type === 'set_variable' ? t("workflow.variable_value") :
                                        t("workflow.value")
                              }>
                                {type === 'script' ? (
                                  <Select
                                    placeholder={t("workflow.select_script")}
                                    options={scripts.map(s => ({ label: s.name, value: s.name }))}
                                  />
                                ) : type === 'wait' ? (
                                  <InputNumber addonAfter="ms" min={100} step={100} style={{ width: '100%' }} />
                                ) : type === 'swipe_element' ? (
                                  <Select options={[
                                    { label: t("workflow.direction_up"), value: 'up' },
                                    { label: t("workflow.direction_down"), value: 'down' },
                                    { label: t("workflow.direction_left"), value: 'left' },
                                    { label: t("workflow.direction_right"), value: 'right' },
                                  ]} placeholder={t("workflow.select_direction")} />
                                ) : type === 'set_variable' ? (
                                  <AutoComplete
                                    options={variableOptions}
                                    filterOption={(inputValue, option) =>
                                      (option?.value as string || '').toUpperCase().indexOf(inputValue.toUpperCase()) !== -1
                                    }
                                  >
                                    <Input placeholder={t("workflow.variable_value_placeholder")} />
                                  </AutoComplete>
                                ) : isAppAction ? (
                                  <Select
                                    showSearch
                                    placeholder={t("apps.filter_placeholder")}
                                    loading={appsLoading}
                                    onFocus={() => packages.length === 0 && fetchPackages()}
                                    options={packages.map(p => ({
                                      content: (
                                        <div style={{ display: 'flex', justifyContent: 'space-between', width: '100%' }}>
                                          <span>{p.label || p.name}</span>
                                          <span style={{ fontSize: 10, color: token.colorTextSecondary }}>{p.name}</span>
                                        </div>
                                      ),
                                      label: `${p.label || ''} ${p.name}`,
                                      value: p.name
                                    }))}
                                    filterOption={(input, option) =>
                                      (option?.label ?? '').toLowerCase().includes(input.toLowerCase())
                                    }
                                  />
                                ) : (
                                  <AutoComplete
                                    options={variableOptions}
                                    filterOption={(inputValue, option) =>
                                      (option?.value as string || '').toUpperCase().indexOf(inputValue.toUpperCase()) !== -1
                                    }
                                  >
                                    <Input placeholder={type === 'adb' ? 'shell input keyevent 4' : t("workflow.value_placeholder")} />
                                  </AutoComplete>
                                )}
                              </Form.Item>
                            )}

                            {type === 'swipe_element' && (
                              <div style={{ display: 'flex', gap: 16 }}>
                                <Form.Item name="swipeDistance" label={t("workflow.distance")} style={{ flex: 1 }}>
                                  <InputNumber addonAfter="px" min={50} step={50} style={{ width: '100%' }} placeholder="500" />
                                </Form.Item>
                                <Form.Item name="swipeDuration" label={t("workflow.duration")} style={{ flex: 1 }}>
                                  <InputNumber addonAfter="ms" min={100} step={100} style={{ width: '100%' }} placeholder="500" />
                                </Form.Item>
                              </div>
                            )}

                            {/* Coordinate fields for tap */}
                            {type === 'tap' && (
                              <div style={{ display: 'flex', gap: 16 }}>
                                <Form.Item name="x" label="X" style={{ flex: 1 }}>
                                  <InputNumber min={0} style={{ width: '100%' }} placeholder="540" />
                                </Form.Item>
                                <Form.Item name="y" label="Y" style={{ flex: 1 }}>
                                  <InputNumber min={0} style={{ width: '100%' }} placeholder="960" />
                                </Form.Item>
                              </div>
                            )}

                            {/* Coordinate fields for swipe */}
                            {type === 'swipe' && (
                              <>
                                <div style={{ display: 'flex', gap: 16 }}>
                                  <Form.Item name="x" label={t("workflow.start_x") || "Start X"} style={{ flex: 1 }}>
                                    <InputNumber min={0} style={{ width: '100%' }} placeholder="540" />
                                  </Form.Item>
                                  <Form.Item name="y" label={t("workflow.start_y") || "Start Y"} style={{ flex: 1 }}>
                                    <InputNumber min={0} style={{ width: '100%' }} placeholder="1800" />
                                  </Form.Item>
                                </div>
                                <div style={{ display: 'flex', gap: 16 }}>
                                  <Form.Item name="x2" label={t("workflow.end_x") || "End X"} style={{ flex: 1 }}>
                                    <InputNumber min={0} style={{ width: '100%' }} placeholder="540" />
                                  </Form.Item>
                                  <Form.Item name="y2" label={t("workflow.end_y") || "End Y"} style={{ flex: 1 }}>
                                    <InputNumber min={0} style={{ width: '100%' }} placeholder="600" />
                                  </Form.Item>
                                </div>
                                <div style={{ display: 'flex', gap: 16 }}>
                                  <Form.Item name="swipeDistance" label={t("workflow.distance")} style={{ flex: 1 }}>
                                    <InputNumber addonAfter="px" min={50} step={50} style={{ width: '100%' }} placeholder="500" />
                                  </Form.Item>
                                  <Form.Item name="swipeDuration" label={t("workflow.duration")} style={{ flex: 1 }}>
                                    <InputNumber addonAfter="ms" min={100} step={100} style={{ width: '100%' }} placeholder="300" />
                                  </Form.Item>
                                </div>
                              </>
                            )}

                            {/* Read to Variable specific fields */}
                            {isReadToVariable && (
                              <>
                                <Form.Item name="variableName" label={t("workflow.variable_name")} rules={[{ required: true }]}>
                                  <Input placeholder={t("workflow.variable_name_placeholder") || "myVariable"} />
                                </Form.Item>
                                <Form.Item name="attribute" label={t("workflow.attribute") || "Attribute"}>
                                  <Select
                                    defaultValue="text"
                                    options={[
                                      { label: 'Text', value: 'text' },
                                      { label: 'Content Description', value: 'contentDesc' },
                                      { label: 'Resource ID', value: 'resourceId' },
                                      { label: 'Class Name', value: 'className' },
                                      { label: 'Bounds', value: 'bounds' },
                                    ]}
                                  />
                                </Form.Item>
                                <Form.Item name="regex" label={t("workflow.regex") || "Regex (optional)"}>
                                  <Input placeholder={t("workflow.regex_placeholder") || "\\d+"} />
                                </Form.Item>
                                <Form.Item name="defaultValue" label={t("workflow.default_value") || "Default Value"}>
                                  <Input placeholder={t("workflow.default_value_placeholder") || "Value if not found"} />
                                </Form.Item>
                              </>
                            )}

                            {/* Session control fields */}
                            {type === 'start_session' && (
                              <>
                                <Form.Item name="sessionName" label={t("workflow.session_name") || "Session Name"}>
                                  <Input placeholder={t("workflow.session_name_placeholder") || "My Session"} />
                                </Form.Item>
                                
                                <Divider plain style={{ margin: '12px 0 8px 0' }}>
                                  <Text type="secondary" style={{ fontSize: 12 }}>{t("workflow.session_features") || "Session Features"}</Text>
                                </Divider>

                                <Form.Item name="logcatEnabled" valuePropName="checked" style={{ marginBottom: 8 }}>
                                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                    <input type="checkbox" id="logcatEnabled" onChange={(e) => stepForm.setFieldValue('logcatEnabled', e.target.checked)} checked={stepForm.getFieldValue('logcatEnabled')} />
                                    <label htmlFor="logcatEnabled">{t("workflow.logcat_enabled") || "Enable Logcat"}</label>
                                  </div>
                                </Form.Item>
                                <Form.Item noStyle shouldUpdate={(prev, cur) => prev.logcatEnabled !== cur.logcatEnabled}>
                                  {({ getFieldValue }) => getFieldValue('logcatEnabled') && (
                                    <div style={{ marginLeft: 24, marginBottom: 12 }}>
                                      <Form.Item name="logcatPackageName" label={t("workflow.logcat_package") || "Package Name"} style={{ marginBottom: 8 }}>
                                        <Select
                                          showSearch
                                          allowClear
                                          placeholder={t("apps.filter_placeholder")}
                                          loading={appsLoading}
                                          onFocus={() => packages.length === 0 && fetchPackages()}
                                          options={packages.map(p => ({ label: `${p.label || ''} ${p.name}`, value: p.name }))}
                                          filterOption={(input, option) => (option?.label ?? '').toLowerCase().includes(input.toLowerCase())}
                                        />
                                      </Form.Item>
                                      <Form.Item name="logcatPreFilter" label={t("workflow.logcat_filter") || "Pre-Filter"} style={{ marginBottom: 8 }}>
                                        <Input placeholder="tag:MyTag" />
                                      </Form.Item>
                                      <Form.Item name="logcatExcludeFilter" label={t("workflow.logcat_exclude") || "Exclude Filter"} style={{ marginBottom: 8 }}>
                                        <Input placeholder="chatty" />
                                      </Form.Item>
                                    </div>
                                  )}
                                </Form.Item>

                                <Form.Item name="recordingEnabled" valuePropName="checked" style={{ marginBottom: 8 }}>
                                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                    <input type="checkbox" id="recordingEnabled" onChange={(e) => stepForm.setFieldValue('recordingEnabled', e.target.checked)} checked={stepForm.getFieldValue('recordingEnabled')} />
                                    <label htmlFor="recordingEnabled">{t("workflow.recording_enabled") || "Enable Recording"}</label>
                                  </div>
                                </Form.Item>
                                <Form.Item noStyle shouldUpdate={(prev, cur) => prev.recordingEnabled !== cur.recordingEnabled}>
                                  {({ getFieldValue }) => getFieldValue('recordingEnabled') && (
                                    <div style={{ marginLeft: 24, marginBottom: 12 }}>
                                      <Form.Item name="recordingQuality" label={t("workflow.recording_quality") || "Quality"} style={{ marginBottom: 8 }}>
                                        <Select
                                          defaultValue="medium"
                                          options={[
                                            { label: t("workflow.quality_low") || "Low (480p)", value: 'low' },
                                            { label: t("workflow.quality_medium") || "Medium (720p)", value: 'medium' },
                                            { label: t("workflow.quality_high") || "High (1080p)", value: 'high' },
                                          ]}
                                        />
                                      </Form.Item>
                                    </div>
                                  )}
                                </Form.Item>

                                <Form.Item name="proxyEnabled" valuePropName="checked" style={{ marginBottom: 8 }}>
                                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                    <input type="checkbox" id="proxyEnabled" onChange={(e) => stepForm.setFieldValue('proxyEnabled', e.target.checked)} checked={stepForm.getFieldValue('proxyEnabled')} />
                                    <label htmlFor="proxyEnabled">{t("workflow.proxy_enabled") || "Enable Proxy"}</label>
                                  </div>
                                </Form.Item>
                                <Form.Item noStyle shouldUpdate={(prev, cur) => prev.proxyEnabled !== cur.proxyEnabled}>
                                  {({ getFieldValue }) => getFieldValue('proxyEnabled') && (
                                    <div style={{ marginLeft: 24, marginBottom: 12 }}>
                                      <Form.Item name="proxyPort" label={t("workflow.proxy_port") || "Port"} style={{ marginBottom: 8 }}>
                                        <InputNumber min={1024} max={65535} defaultValue={8080} style={{ width: '100%' }} />
                                      </Form.Item>
                                      <Form.Item name="proxyMitmEnabled" valuePropName="checked" style={{ marginBottom: 8 }}>
                                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                          <input type="checkbox" id="proxyMitmEnabled" onChange={(e) => stepForm.setFieldValue('proxyMitmEnabled', e.target.checked)} checked={stepForm.getFieldValue('proxyMitmEnabled')} />
                                          <label htmlFor="proxyMitmEnabled">{t("workflow.proxy_mitm") || "Enable MITM (HTTPS)"}</label>
                                        </div>
                                      </Form.Item>
                                    </div>
                                  )}
                                </Form.Item>

                                <Form.Item name="monitorEnabled" valuePropName="checked" style={{ marginBottom: 8 }}>
                                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                    <input type="checkbox" id="monitorEnabled" onChange={(e) => stepForm.setFieldValue('monitorEnabled', e.target.checked)} checked={stepForm.getFieldValue('monitorEnabled')} />
                                    <label htmlFor="monitorEnabled">{t("workflow.monitor_enabled") || "Enable Device Monitor"}</label>
                                  </div>
                                </Form.Item>
                              </>
                            )}

                            {type === 'end_session' && (
                              <Form.Item name="sessionStatus" label={t("workflow.session_status") || "End Status"}>
                                <Select
                                  defaultValue="completed"
                                  options={[
                                    { label: t("workflow.status_completed") || "Completed", value: 'completed' },
                                    { label: t("workflow.status_error") || "Error", value: 'error' },
                                    { label: t("workflow.status_cancelled") || "Cancelled", value: 'cancelled' },
                                  ]}
                                />
                              </Form.Item>
                            )}
                          </>
                        );
                      }}
                    </Form.Item>

                    <Divider plain style={{ margin: '16px 0 12px 0' }}>
                      <Text type="secondary" style={{ fontSize: 12 }}>{t("workflow.execution_control")}</Text>
                    </Divider>

                    <div style={{ display: 'flex', gap: 16 }}>
                      <Form.Item name="timeout" label={t("workflow.timeout")} style={{ flex: 1 }}>
                        <InputNumber addonAfter="ms" min={100} step={1000} style={{ width: '100%' }} placeholder="5000" />
                      </Form.Item>
                      <Form.Item name="loop" label={t("workflow.loop")} style={{ width: 100 }}>
                        <InputNumber min={1} style={{ width: '100%' }} placeholder="1" />
                      </Form.Item>
                    </div>

                    <div style={{ display: 'flex', gap: 16 }}>
                      <Form.Item name="preWait" label={t("workflow.pre_wait")} style={{ flex: 1 }}>
                        <InputNumber addonAfter="ms" min={0} step={500} style={{ width: '100%' }} placeholder="0" />
                      </Form.Item>
                      <Form.Item name="postDelay" label={t("workflow.post_delay")} style={{ flex: 1 }}>
                        <InputNumber addonAfter="ms" min={0} step={500} style={{ width: '100%' }} placeholder="0" />
                      </Form.Item>
                    </div>

                    <Form.Item name="onError" label={t("workflow.on_error")}>
                      <Select
                        options={[
                          { label: t("workflow.error_stop"), value: 'stop' },
                          { label: t("workflow.error_continue"), value: 'continue' },
                        ]}
                        placeholder={t("workflow.error_stop")}
                      />
                    </Form.Item>

                    <Divider style={{ margin: '12px 0' }} />

                    <Button danger block icon={<DeleteOutlined />} onClick={handleDeleteNode}>
                      {t("workflow.delete_step")}
                    </Button>
                  </Form>
                </>
              )}
            </div>
          </div>
        )}
      </div >

      <Modal
        title={t("workflow.create")}
        open={workflowModalVisible}
        onOk={() => workflowForm.submit()}
        onCancel={() => setWorkflowModalVisible(false)}
      >
        <Form form={workflowForm} layout="vertical" onFinish={handleCreateWorkflowSubmit}>
          <Form.Item
            name="name"
            label={t("workflow.name")}
            rules={[{ required: true }]}
          >
            <Input placeholder={t("workflow.name_placeholder")} />
          </Form.Item>
          <Form.Item name="description" label={t("workflow.description")}>
            <Input.TextArea placeholder={t("workflow.description_placeholder")} rows={2} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={t("workflow.global_variables")}
        open={variablesModalVisible}
        onOk={handleSaveVariables}
        onCancel={() => setVariablesModalVisible(false)}
        width={600}
        modalRender={(modal) => (
          <div onMouseDown={(e) => e.stopPropagation()}>{modal}</div>
        )}
      >
        <div style={{ marginBottom: 16 }}>
          <Text type="secondary">{t("workflow.variables_tip")}</Text>
        </div>
        <div style={{ maxHeight: 400, overflowY: 'auto' }}>
          {tempVariables.map((item, index) => (
            <div key={index} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
              <Input
                placeholder={t("workflow.var_key_placeholder")}
                value={item.key}
                onChange={(e) => {
                  const newVars = [...tempVariables];
                  newVars[index].key = e.target.value;
                  setTempVariables(newVars);
                }}
                style={{ width: 180 }}
              />
              <Input
                placeholder={t("workflow.var_val_placeholder")}
                value={item.value}
                onChange={(e) => {
                  const newVars = [...tempVariables];
                  newVars[index].value = e.target.value;
                  setTempVariables(newVars);
                }}
                style={{ flex: 1 }}
              />
              <Button
                type="text"
                danger
                icon={<DeleteOutlined />}
                onClick={() => setTempVariables((prev: { key: string; value: string }[]) => prev.filter((_: any, i: number) => i !== index))}
              />
            </div>
          ))}
        </div>
        <Button
          type="dashed"
          onClick={() => setTempVariables((prev: { key: string; value: string }[]) => [...prev, { key: '', value: '' }])}
          block
          icon={<PlusOutlined />}
          style={{ marginTop: 8 }}
        >
          {t("workflow.add_variable")}
        </Button>
      </Modal>

      <ElementPicker
        open={elementPickerVisible}
        onCancel={() => setElementPickerVisible(false)}
        onSelect={handleElementSelected}
      />
    </div >
  );
};

export default WorkflowView;
