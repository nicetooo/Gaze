import React, { useState, useEffect, useCallback, useMemo, useRef } from "react";
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
  AutoComplete
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
  PartitionOutlined,
  PicCenterOutlined,
  PicRightOutlined,
  LoadingOutlined,
  SettingOutlined,
  IdcardOutlined,
  ThunderboltOutlined,
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
} from '@xyflow/react';
import type { Connection, Edge, Node, NodeChange } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import dagre from 'dagre';


import DeviceSelector from "./DeviceSelector";
import ElementPicker, { ElementSelector } from "./ElementPicker";
import { useDeviceStore, useAutomationStore } from "../stores";

const { Text, Title } = Typography;

// Step type definitions
const STEP_TYPES = {
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
interface WorkflowStep {
  id: string;
  type: string;
  label?: string;
  name?: string; // Some steps use name instead of label
  selector?: ElementSelector;
  value?: string;
  loop?: number; // Number of times to loop (for wait/loop steps)
  timeout?: number; // Timeout for element wait steps in ms
  postDelay?: number; // Delay after execution in ms
  preWait?: number; // Delay before execution in ms
  swipeDistance?: number; // Distance for swipe actions
  swipeDuration?: number; // Duration for swipe actions in ms
  conditionType?: string; // Condition type for branch steps: 'exists', 'not_exists', 'text_equals', 'text_contains', 'variable_equals'
  onError?: 'stop' | 'continue'; // Error handling strategy

  // Flow connections (next step IDs)
  nextStepId?: string;
  trueStepId?: string;
  falseStepId?: string;

  // Visual layout position on the graph
  posX?: number;
  posY?: number;

  // Persistence for specific handle IDs
  nextSource?: string;
  trueSource?: string;
  falseSource?: string;
  nextTarget?: string;
  trueTarget?: string;
  falseTarget?: string;

  children?: WorkflowStep[]; // For nested steps
  scriptId?: string; // For script steps
  workflowId?: string; // For run_workflow steps
}

interface Workflow {
  id: string;
  name: string;
  description?: string;
  steps: WorkflowStep[];
  variables?: Record<string, string>;
  createdAt?: string;
  updatedAt?: string;
}

// Custom Node Component
const WorkflowNode = ({ data, selected }: any) => {
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
            {step.selector && (
              <div style={{ fontSize: 11, color: token.colorTextSecondary, display: 'flex', alignItems: 'center', gap: 4 }}>
                <Tag style={{ margin: 0, fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>{step.selector.type}</Tag>
                <Text ellipsis style={{ fontSize: 11, maxWidth: 120, color: token.colorTextSecondary }}>{step.selector.value}</Text>
              </div>
            )}
            {!isBranch && step.value && (
              <div style={{ fontSize: 11, color: token.colorTextSecondary }}>
                <Text ellipsis style={{ fontSize: 11, color: token.colorTextSecondary }}>{step.value}</Text>
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
          <div style={{ position: 'absolute', bottom: -14, left: '25%', fontSize: 9, color: token.colorSuccess, fontWeight: 'bold' }}>{t('workflow.branch_true')}</div>
          <Handle
            type="source"
            position={Position.Bottom}
            id="true"
            style={{ left: '30%', background: token.colorSuccess }}
          />
          <Handle
            type="source"
            position={Position.Left}
            id="true-left"
            style={{ top: '70%', background: token.colorSuccess }}
          />

          <div style={{ position: 'absolute', bottom: -14, right: '25%', fontSize: 9, color: token.colorError, fontWeight: 'bold' }}>{t('workflow.branch_false')}</div>
          <Handle
            type="source"
            position={Position.Bottom}
            id="false"
            style={{ left: '70%', background: token.colorError }}
          />
          <Handle
            type="source"
            position={Position.Right}
            id="false-right"
            style={{ top: '70%', background: token.colorError }}
          />
        </>
      ) : (
        <>
          <Handle type="source" position={Position.Bottom} id="default" style={{ background: token.colorTextSecondary }} />
          {!isStart && (
            <>
              <Handle type="source" position={Position.Left} id="source-left" style={{ top: '70%', background: token.colorTextSecondary }} />
              <Handle type="source" position={Position.Right} id="source-right" style={{ top: '70%', background: token.colorTextSecondary }} />
            </>
          )}
        </>
      )}
    </div>
  );
};

const WorkflowView: React.FC = () => {
  const { selectedDevice } = useDeviceStore();
  const { scripts } = useAutomationStore();
  const { t } = useTranslation();
  const { token } = theme.useToken();

  const [workflows, setWorkflows] = useState<Workflow[]>([]);
  const [selectedWorkflow, setSelectedWorkflow] = useState<Workflow | null>(null);
  const [workflowModalVisible, setWorkflowModalVisible] = useState(false);
  const [workflowForm] = Form.useForm();
  const [packages, setPackages] = useState<any[]>([]);
  const [appsLoading, setAppsLoading] = useState(false);
  const [editingWorkflowId, setEditingWorkflowId] = useState<string | null>(null);
  const [editingWorkflowName, setEditingWorkflowName] = useState("");

  // Execution state
  const [isRunning, setIsRunning] = useState(false);
  const [runningWorkflowIds, setRunningWorkflowIds] = useState<string[]>([]);
  const [isPaused, setIsPaused] = useState(false);
  const [currentStepId, setCurrentStepId] = useState<string | null>(null);
  const [waitingStepId, setWaitingStepId] = useState<string | null>(null);
  const [waitingPhase, setWaitingPhase] = useState<'pre' | 'post' | null>(null);
  const [executionLogs, setExecutionLogs] = useState<string[]>([]);
  const [variablesModalVisible, setVariablesModalVisible] = useState(false);
  const [tempVariables, setTempVariables] = useState<{ key: string, value: string }[]>([]);

  // Track current step for each workflow (including nested workflows)
  // Key: workflowId, Value: { stepId, isWaiting, waitPhase }
  const [workflowStepMap, setWorkflowStepMap] = useState<Record<string, { stepId: string, isWaiting: boolean, waitPhase?: 'pre' | 'post' }>>({});

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
  const [needsAutoSave, setNeedsAutoSave] = useState(false);
  const reconnectionEdgesRef = useRef<Edge[]>([]);

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
  const [editingNodeId, setEditingNodeId] = useState<string | null>(null);

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
  const [drawerVisible, setDrawerVisible] = useState(false);
  const [stepForm] = Form.useForm();

  // Element Picker
  const [elementPickerVisible, setElementPickerVisible] = useState(false);

  // Track if we should skip the next node re-render (after saving)
  const skipNextRenderRef = useRef(false);

  const nodeTypes = useMemo(() => ({ workflowNode: WorkflowNode }), []);

  useEffect(() => {
    loadWorkflows();
  }, []);

  const loadWorkflows = async () => {
    try {
      const result = await (window as any).go.main.App.LoadWorkflows();
      setWorkflows(result || []);
    } catch (err) {
      setWorkflows([]);
    }
  };

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

  useEffect(() => {
    // Skip re-rendering nodes if we just saved (nodes are already up-to-date)
    if (skipNextRenderRef.current) {
      console.log('[Workflow Render] Skipping render because skipNextRenderRef is true');
      skipNextRenderRef.current = false;
      return;
    }

    console.log('[Workflow Render] Rendering workflow:', selectedWorkflow?.name);

    if (selectedWorkflow) {
      const newNodes: Node[] = selectedWorkflow.steps.map((step, index) => {
        // Check if this step is currently running in this workflow
        const stepState = workflowStepMap[selectedWorkflow.id];
        const isCurrent = stepState?.stepId === step.id && !stepState?.isWaiting;
        const isWaiting = stepState?.stepId === step.id && stepState?.isWaiting;
        const waitPhase = stepState?.waitPhase;

        return {
          id: step.id,
          type: 'workflowNode',
          position: (step.posX !== undefined && step.posY !== undefined) ? { x: step.posX, y: step.posY } : { x: 0, y: 0 },
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
      const hasGraphData = selectedWorkflow.steps.some(s => s.nextStepId || s.trueStepId || s.falseStepId);

      if (hasGraphData) {
        selectedWorkflow.steps.forEach(step => {
          if (step.nextStepId) {
            const edge: any = {
              id: `e-${step.id}-${step.nextStepId}`,
              source: step.id,
              target: step.nextStepId,
              type: 'smoothstep',
              sourceHandle: step.nextSource || 'default',
              markerEnd: { type: MarkerType.ArrowClosed }
            };
            // Only set targetHandle if it has a valid, non-empty value
            if (isValidTargetHandle(step.nextTarget)) {
              edge.targetHandle = step.nextTarget;
            }
            newEdges.push(edge);
          }
          if (step.trueStepId) {
            const edge: any = {
              id: `e-${step.id}-${step.trueStepId}-true`,
              source: step.id,
              target: step.trueStepId,
              type: 'smoothstep',
              sourceHandle: step.trueSource || 'true',
              label: t('workflow.branch_true'),
              style: { stroke: token.colorSuccess },
              labelStyle: { fill: token.colorSuccess, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: token.colorSuccess }
            };
            // Only set targetHandle if it has a valid, non-empty value
            if (isValidTargetHandle(step.trueTarget)) {
              edge.targetHandle = step.trueTarget;
            }
            newEdges.push(edge);
          }
          if (step.falseStepId) {
            const edge: any = {
              id: `e-${step.id}-${step.falseStepId}-false`,
              source: step.id,
              target: step.falseStepId,
              type: 'smoothstep',
              sourceHandle: step.falseSource || 'false',
              label: t('workflow.branch_false'),
              style: { stroke: token.colorError },
              labelStyle: { fill: token.colorError, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: token.colorError }
            };
            // Only set targetHandle if it has a valid, non-empty value
            if (isValidTargetHandle(step.falseTarget)) {
              edge.targetHandle = step.falseTarget;
            }
            newEdges.push(edge);
          }
        });
      } else {
        selectedWorkflow.steps.slice(0, -1).forEach((step, index) => {
          newEdges.push({
            id: `e-${step.id}-${selectedWorkflow.steps[index + 1].id}`,
            source: step.id,
            target: selectedWorkflow.steps[index + 1].id,
            type: 'smoothstep',
            sourceHandle: 'default',
            markerEnd: { type: MarkerType.ArrowClosed },
          });
        });
      }

      const hasPositions = selectedWorkflow.steps.some(s => s.posX !== undefined);

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

    setNodes((nds) =>
      nds.map((node) => {
        // Get the step state for the current workflow
        const stepState = workflowStepMap[selectedWorkflow.id];
        const isCurrent = stepState?.stepId === node.id && !stepState?.isWaiting;
        const isWaiting = stepState?.stepId === node.id && stepState?.isWaiting;
        const phase = stepState?.stepId === node.id ? stepState?.waitPhase : null;

        if (node.data.isCurrent !== isCurrent || node.data.isWaiting !== isWaiting || node.data.waitingPhase !== phase) {
          return {
            ...node,
            data: { ...(node.data as object), isCurrent, isWaiting, isRunning, waitingPhase: phase }
          }
        }
        return node;
      })
    );
  }, [workflowStepMap, selectedWorkflow, isRunning]);

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
    stepForm.setFieldsValue({
      ...step,
      selectorType: step.selector?.type,
      selectorValue: step.selector?.value,
      type: step.type,
      conditionType: step.conditionType || 'exists', // Default to 'exists' for branch nodes
    });
    setDrawerVisible(true);
  };

  const handleCreateWorkflow = () => {
    workflowForm.resetFields();
    setSelectedWorkflow(null);
    setWorkflowModalVisible(true);
  };

  const handleCreateWorkflowSubmit = async (values: any) => {
    // Auto-create a Start node
    const startStep: WorkflowStep = {
      id: `start_${Date.now()}`,
      type: 'start',
      name: 'Start',
      posX: 250,
      posY: 50,
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
      setWorkflows(prev => [...prev, newWorkflow]);
      setSelectedWorkflow(newWorkflow);
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

      let nextStepId = "";
      let nextSource = "";
      let nextTarget = "";
      let trueStepId = "";
      let trueSource = "";
      let trueTarget = "";
      let falseStepId = "";
      let falseSource = "";
      let falseTarget = "";

      if (originalStep.type === 'branch') {
        const t = outgoing.find(e => e.sourceHandle === 'true' || e.sourceHandle === 'true-left');
        const f = outgoing.find(e => e.sourceHandle === 'false' || e.sourceHandle === 'false-right');
        if (t) {
          trueStepId = t.target;
          trueSource = t.sourceHandle || 'true';
          // Only save valid targetHandle values
          const tHandle = t.targetHandle;
          trueTarget = isValidTargetHandle(tHandle) ? String(tHandle) : '';
        }
        if (f) {
          falseStepId = f.target;
          falseSource = f.sourceHandle || 'false';
          // Only save valid targetHandle values
          const fHandle = f.targetHandle;
          falseTarget = isValidTargetHandle(fHandle) ? String(fHandle) : '';
        }
      } else {
        const next = outgoing.find(e => e.sourceHandle === 'default' || e.sourceHandle === 'source-left' || e.sourceHandle === 'source-right' || !e.sourceHandle);
        if (next) {
          nextStepId = next.target;
          nextSource = next.sourceHandle || 'default';
          // Only save valid targetHandle values
          const nHandle = next.targetHandle;
          nextTarget = isValidTargetHandle(nHandle) ? String(nHandle) : '';
        }
      }

      return {
        ...originalStep,
        nextStepId,
        nextSource,
        nextTarget,
        trueStepId,
        trueSource,
        trueTarget,
        falseStepId,
        falseSource,
        falseTarget,
        value: originalStep.value,
        posX: node.position.x,
        posY: node.position.y
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

      // Update both workflows list and selectedWorkflow
      setWorkflows(prev => {
        const idx = prev.findIndex(w => w.id === updatedWorkflow.id);
        if (idx >= 0) {
          const newList = [...prev];
          newList[idx] = updatedWorkflow;
          return newList;
        }
        return [...prev, updatedWorkflow];
      });
      setSelectedWorkflow(updatedWorkflow);

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
    const newStep: WorkflowStep = {
      id,
      type,
      loop: 1,
      postDelay: 0,
      onError: 'stop'
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
          const newEdge = {
            id: `e-${lastNode.id}-${id}`,
            source: lastNode.id,
            target: id,
            type: 'smoothstep',
            sourceHandle: 'default',
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
        const updatedStep: WorkflowStep = {
          ...(node.data.step as WorkflowStep),
          type: values.type,
          name: values.name,
          selector: values.selectorValue ? {
            type: values.selectorType || 'text',
            value: values.selectorValue,
          } : undefined,
          value: values.value !== undefined ? String(values.value) : undefined,
          timeout: values.timeout,
          onError: values.onError,
          loop: values.loop,
          preWait: Number(values.preWait || 0),
          postDelay: Number(values.postDelay || 0),
          swipeDistance: values.swipeDistance,
          swipeDuration: values.swipeDuration,
          conditionType: values.conditionType,
          nextStepId: (node.data.step as WorkflowStep).nextStepId,
          trueStepId: (node.data.step as WorkflowStep).trueStepId,
          falseStepId: (node.data.step as WorkflowStep).falseStepId,
        };
        return {
          ...node,
          data: {
            ...node.data,
            step: updatedStep,
            label: values.name || t(`workflow.step_type.${values.type}`)
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

    const sanitizedSteps = workflowToRun.steps.map(s => {
      if (s.type === 'adb' && s.value && s.value.trim().startsWith('input ')) {
        return { ...s, value: `shell ${s.value}` };
      }
      return s;
    });
    const sanitizedWorkflow = { ...workflowToRun, steps: sanitizedSteps };

    setIsRunning(true);
    setRunningWorkflowIds([selectedWorkflow.id]);
    setCurrentStepId(null);
    setExecutionLogs([
      `[${new Date().toLocaleTimeString()}] ${t("workflow.started")}: ${sanitizedWorkflow.name}`,
      `[Info] Executing ${sanitizedWorkflow.steps.length} steps.`
    ]);

    const executionPromise = new Promise<void>((resolve, reject) => {
      const runtime = (window as any).runtime;
      if (!runtime) {
        setTimeout(resolve, 1000);
        return;
      }

      const cleanUp = () => {
        runtime.EventsOff("workflow-started", onSubStarted);
        runtime.EventsOff("workflow-completed", onComplete);
        runtime.EventsOff("workflow-error", onError);
        runtime.EventsOff("workflow-step-running", onStep);
        runtime.EventsOff("workflow-step-waiting", onWait);
        runtime.EventsOff("task-paused", onPaused);
        runtime.EventsOff("task-resumed", onResumed);
      };

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
        if (data.deviceId === deviceObj.id) {
          setIsPaused(true);
        }
      };

      const onResumed = (data: any) => {
        if (data.deviceId === deviceObj.id) {
          setIsPaused(false);
        }
      };

      runtime.EventsOn("workflow-started", onSubStarted);
      runtime.EventsOn("workflow-completed", onComplete);
      runtime.EventsOn("workflow-error", onError);
      runtime.EventsOn("workflow-step-running", onStep);
      runtime.EventsOn("workflow-step-waiting", onWait);
      runtime.EventsOn("task-paused", onPaused);
      runtime.EventsOn("task-resumed", onResumed);
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
      setIsRunning(false);
      setIsPaused(false);
      setRunningWorkflowIds([]);
      setCurrentStepId(null);
      setWorkflowStepMap({}); // Clear all workflow step tracking
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
      setWorkflows(prev => {
        const idx = prev.findIndex(w => w.id === updatedWorkflow.id);
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
    const deviceObj = useDeviceStore.getState().devices.find(d => d.id === selectedDevice);
    if (!deviceObj) {
      message.error(t("workflow.error_device_not_found"));
      return;
    }

    try {
      await (window as any).go.main.App.StopWorkflow(deviceObj);
      setIsRunning(false);
      setIsPaused(false);
      setCurrentStepId(null);
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
      setWorkflows(prev => {
        const idx = prev.findIndex(w => w.id === id);
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
    if (!selectedDevice) return;
    await (window as any).go.main.App.PauseTask(selectedDevice);
  };

  const handleResumeWorkflow = async () => {
    if (!selectedDevice) return;
    await (window as any).go.main.App.ResumeTask(selectedDevice);
  };

  return (
    <div style={{ height: "100%", display: "flex", flexDirection: "column" }}>
      <div style={{ padding: "12px 24px", borderBottom: `1px solid ${token.colorBorderSecondary}`, display: "flex", justifyContent: "space-between", alignItems: "center", background: token.colorBgContainer }}>
        <Space>
          <Title level={4} style={{ margin: 0 }}>{t("workflow.title")}</Title>
          <Tag color="green">Visual Editor</Tag>
        </Space>
        <Space>
          <DeviceSelector />
          {selectedWorkflow && (
            <>
              <Tooltip title={t("workflow.auto_layout") + " (Vertical)"}>
                <Button icon={<PicCenterOutlined />} onClick={() => handleAutoLayout('TB')} />
              </Tooltip>
              <Tooltip title={t("workflow.auto_layout") + " (Horizontal)"}>
                <Button icon={<PicRightOutlined />} onClick={() => handleAutoLayout('LR')} />
              </Tooltip>
              <Button icon={<SettingOutlined />} onClick={handleOpenVariablesModal}>
                {t("workflow.variables")}
              </Button>
              <Button icon={<SaveOutlined />} onClick={() => handleSaveGraph()}>
                {t("workflow.save")}
              </Button>
              {isRunning ? (
                <Space>
                  {isPaused ? (
                    <Button icon={<PlayCircleOutlined />} onClick={handleResumeWorkflow}>
                      {t("workflow.resume")}
                    </Button>
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
          <div style={{ flex: 1, overflowY: 'auto' }}>
            {workflows.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("workflow.no_workflows")} />
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column' }}>
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
                            style={{ fontWeight: 500, display: 'flex', alignItems: 'center', gap: 8 }}
                            onDoubleClick={(e) => {
                              e.stopPropagation();
                              setEditingWorkflowId(workflow.id);
                              setEditingWorkflowName(workflow.name);
                            }}
                          >
                            <Text strong style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                              {workflow.name}
                            </Text>
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
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodesChange={onNodesChangeWithProtection}
              onEdgesChange={onEdgesChange}
              onConnect={onConnect}
              onNodeClick={handleNodeClick}
              nodeTypes={nodeTypes}
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
                    defaultActiveKey={['1', '2', '3']}
                    items={[
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
                      }
                    ]}
                  />
                </div>
              </Panel>
            </ReactFlow>
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
                        const label = type === 'set_variable' ? t("workflow.variable_name") : t("workflow.name");
                        return (
                          <Form.Item name="name" label={label}>
                            <Input placeholder={label} />
                          </Form.Item>
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
                        const conditionType = getFieldValue('conditionType') || 'exists';
                        const needsSelector = ['click_element', 'long_click_element', 'input_text', 'swipe_element', 'wait_element', 'wait_gone', 'assert_element', 'branch'].includes(type);
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
                onClick={() => setTempVariables(prev => prev.filter((_, i) => i !== index))}
              />
            </div>
          ))}
        </div>
        <Button
          type="dashed"
          onClick={() => setTempVariables(prev => [...prev, { key: '', value: '' }])}
          block
          icon={<PlusOutlined />}
          style={{ marginTop: 8 }}
        >
          {t("workflow.add_variable")}
        </Button>
      </Modal>

      <ElementPicker
        visible={elementPickerVisible}
        onCancel={() => setElementPickerVisible(false)}
        onSelect={handleElementSelected}
      />
    </div >
  );
};

export default WorkflowView;
