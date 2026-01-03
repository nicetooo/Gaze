import React, { useState, useEffect, useCallback, useMemo, useRef } from "react";
import {
  Button,
  Space,
  Card,
  List,
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
  Typography
} from "antd";
import { useTranslation } from "react-i18next";
import {
  PlayCircleOutlined,
  StopOutlined,
  DeleteOutlined,
  PlusOutlined,
  EditOutlined,
  RobotOutlined,
  AimOutlined,
  ClockCircleOutlined,
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
};

const getStepTypeInfo = (type: string) => {
  for (const category of Object.values(STEP_TYPES)) {
    const found = category.find(s => s.key === type);
    if (found) return found;
  }
  return { key: type, icon: <RobotOutlined />, color: 'default' };
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
  createdAt?: string;
  updatedAt?: string;
}

// Custom Node Component
const WorkflowNode = ({ data, selected }: any) => {
  const { step, isRunning, isCurrent } = data;
  const { token } = theme.useToken();
  const typeInfo = getStepTypeInfo(step.type);
  const isBranch = step.type === 'branch';
  const isStart = step.type === 'start';

  return (
    <div style={{ position: 'relative' }}>
      {/* Start node has no input handle */}
      {!isStart && (
        <>
          <Handle type="target" position={Position.Top} style={{ background: token.colorTextSecondary }} />
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
        bodyStyle={{ padding: '8px 12px' }}
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
        </div>
      </Card>

      {isBranch ? (
        <>
          <div style={{ position: 'absolute', bottom: -14, left: '25%', fontSize: 9, color: token.colorSuccess, fontWeight: 'bold' }}>True</div>
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

          <div style={{ position: 'absolute', bottom: -14, right: '25%', fontSize: 9, color: token.colorError, fontWeight: 'bold' }}>False</div>
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

  // Execution state
  const [isRunning, setIsRunning] = useState(false);
  const [currentStepId, setCurrentStepId] = useState<string | null>(null);
  const [executionLogs, setExecutionLogs] = useState<string[]>([]);

  // React Flow state
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  // Wrapper for onNodesChange to protect Start node from deletion
  const onNodesChangeWithProtection = useCallback(
    (changes: NodeChange[]) => {
      const filteredChanges = changes.filter(c => {
        if (c.type === 'remove') {
          const node = nodes.find(n => n.id === c.id);
          if (node && (node.data.step as WorkflowStep).type === 'start') {
            message.warning(t("workflow.error_delete_start"));
            return false;
          }
        }
        return true;
      });
      onNodesChange(filteredChanges);
    },
    [nodes, onNodesChange, t]
  );

  // Node Editing
  const [editingNodeId, setEditingNodeId] = useState<string | null>(null);
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

  useEffect(() => {
    // Skip re-rendering nodes if we just saved (nodes are already up-to-date)
    if (skipNextRenderRef.current) {
      skipNextRenderRef.current = false;
      return;
    }

    if (selectedWorkflow) {
      const newNodes: Node[] = selectedWorkflow.steps.map((step, index) => {
        return {
          id: step.id,
          type: 'workflowNode',
          position: (step.posX !== undefined && step.posY !== undefined) ? { x: step.posX, y: step.posY } : { x: 0, y: 0 },
          data: {
            step,
            label: step.name || t(`workflow.step_type.${step.type}`),
            isCurrent: step.id === currentStepId
          },
        };
      });

      const newEdges: Edge[] = [];
      const hasGraphData = selectedWorkflow.steps.some(s => s.nextStepId || s.trueStepId || s.falseStepId);

      if (hasGraphData) {
        selectedWorkflow.steps.forEach(step => {
          if (step.nextStepId) {
            newEdges.push({
              id: `e-${step.id}-${step.nextStepId}`,
              source: step.id,
              target: step.nextStepId,
              type: 'smoothstep',
              sourceHandle: step.nextSource || 'default',
              targetHandle: step.nextTarget,
              markerEnd: { type: MarkerType.ArrowClosed }
            });
          }
          if (step.trueStepId) {
            newEdges.push({
              id: `e-${step.id}-${step.trueStepId}-true`,
              source: step.id,
              target: step.trueStepId,
              type: 'smoothstep',
              sourceHandle: step.trueSource || 'true',
              targetHandle: step.trueTarget,
              label: 'True',
              style: { stroke: token.colorSuccess },
              labelStyle: { fill: token.colorSuccess, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: token.colorSuccess }
            });
          }
          if (step.falseStepId) {
            newEdges.push({
              id: `e-${step.id}-${step.falseStepId}-false`,
              source: step.id,
              target: step.falseStepId,
              type: 'smoothstep',
              sourceHandle: step.falseSource || 'false',
              targetHandle: step.falseTarget,
              label: 'False',
              style: { stroke: token.colorError },
              labelStyle: { fill: token.colorError, fontWeight: 700 },
              markerEnd: { type: MarkerType.ArrowClosed, color: token.colorError }
            });
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
      if (hasPositions) {
        setNodes(newNodes);
        setEdges(newEdges);
      } else {
        const layouted = getLayoutedElements(newNodes, newEdges);
        setNodes(layouted.nodes);
        setEdges(layouted.edges);
      }
    } else {
      setNodes([]);
      setEdges([]);
    }
  }, [selectedWorkflow, getLayoutedElements, t, token]);

  useEffect(() => {
    setNodes((nds) =>
      nds.map((node) => {
        const isCurrent = node.id === currentStepId;
        if (node.data.isCurrent !== isCurrent) {
          return {
            ...node,
            data: { ...(node.data as object), isCurrent, isRunning }
          }
        }
        return node;
      })
    );
  }, [currentStepId, isRunning]);

  const onConnect = useCallback(
    (params: Connection) => setEdges((eds) => addEdge({
      ...params,
      type: 'smoothstep',
      markerEnd: { type: MarkerType.ArrowClosed }
    }, eds)),
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
      ranksep: isHorizontal ? 150 : 120,
      nodesep: isHorizontal ? 60 : 100, // In LR, nodesep is vertical distance.
      edgesep: 50,
      marginx: 50,
      marginy: 50,
    });

    nodes.forEach((node) => {
      // In LR mode, we need more 'height' (vertical space) for the node ghost
      // In TB mode, we need more 'width' (horizontal space)
      const ghostWidth = isHorizontal ? 320 : 400;
      const ghostHeight = isHorizontal ? 200 : 150;

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
          trueTarget = t.targetHandle || '';
        }
        if (f) {
          falseStepId = f.target;
          falseSource = f.sourceHandle || 'false';
          falseTarget = f.targetHandle || '';
        }
      } else {
        const next = outgoing.find(e => e.sourceHandle === 'default' || e.sourceHandle === 'source-left' || e.sourceHandle === 'source-right' || !e.sourceHandle);
        if (next) {
          nextStepId = next.target;
          nextSource = next.sourceHandle || 'default';
          nextTarget = next.targetHandle || '';
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
      stepForm.setFieldsValue({ type, onError: 'stop', loop: 1 });
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
          value: values.value,
          timeout: values.timeout,
          onError: values.onError,
          loop: values.loop,
          postDelay: values.postDelay,
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
    message.success(t("workflow.step_updated"));
  };

  const handleDeleteNode = () => {
    if (!editingNodeId) return;

    // Prevent deleting start node
    const nodeToDelete = nodes.find(n => n.id === editingNodeId);
    if (nodeToDelete && (nodeToDelete.data.step as WorkflowStep).type === 'start') {
      message.warning(t("workflow.error_delete_start"));
      return;
    }

    setNodes(nds => nds.filter(n => n.id !== editingNodeId));
    setEdges(eds => eds.filter(e => e.source !== editingNodeId && e.target !== editingNodeId));
    setDrawerVisible(false);
    setEditingNodeId(null);
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
      message.error("Device not found");
      return;
    }

    const workflowToRun = await handleSaveGraph(true);
    if (!workflowToRun) return;

    if (workflowToRun.steps.length === 0) {
      message.warning("Run aborted: Workflow has 0 steps.");
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
        runtime.EventsOff("workflow-completed", onComplete);
        runtime.EventsOff("workflow-error", onError);
        runtime.EventsOff("workflow-step-running", onStep);
      };

      const onComplete = (data: any) => {
        if (data.deviceId === deviceObj.id) {
          cleanUp();
          resolve();
        }
      };

      const onError = (data: any) => {
        if (data.deviceId === deviceObj.id) {
          cleanUp();
          reject(data.error);
        }
      };

      const onStep = (data: any) => {
        if (data.deviceId === deviceObj.id) {
          setCurrentStepId(data.stepId);
        }
      };

      runtime.EventsOn("workflow-completed", onComplete);
      runtime.EventsOn("workflow-error", onError);
      runtime.EventsOn("workflow-step-running", onStep);
    });

    try {
      await (window as any).go.main.App.RunWorkflow(deviceObj, sanitizedWorkflow);
      await executionPromise;
      setExecutionLogs(prev => [...prev, `[${new Date().toLocaleTimeString()}] ${t("workflow.completed")}`]);
    } catch (err) {
      setExecutionLogs(prev => [...prev, `[${new Date().toLocaleTimeString()}] ${t("workflow.error")}: ${err}`]);
      message.error(String(err));
    } finally {
      setIsRunning(false);
      setCurrentStepId(null);
    }
  };

  const handleStopWorkflow = async () => {
    const deviceObj = useDeviceStore.getState().devices.find(d => d.id === selectedDevice);
    if (!deviceObj) {
      message.error("Device not found");
      return;
    }

    try {
      await (window as any).go.main.App.StopWorkflow(deviceObj);
      setIsRunning(false);
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
              <Button icon={<SaveOutlined />} onClick={() => handleSaveGraph()}>
                {t("workflow.save")}
              </Button>
              {isRunning ? (
                <Button danger icon={<StopOutlined />} onClick={handleStopWorkflow}>
                  {t("workflow.stop")}
                </Button>
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
              <List
                size="small"
                dataSource={workflows}
                renderItem={(workflow) => (
                  <List.Item
                    onClick={() => {
                      if (selectedWorkflow?.id !== workflow.id) {
                        setSelectedWorkflow(workflow);
                      }
                    }}
                    style={{
                      cursor: 'pointer',
                      padding: '12px 16px',
                      borderLeft: selectedWorkflow?.id === workflow.id ? `3px solid ${token.colorPrimary}` : '3px solid transparent',
                      backgroundColor: selectedWorkflow?.id === workflow.id ? token.colorPrimaryBg : undefined
                    }}
                    actions={[
                      <Popconfirm key="del" title={t("workflow.delete_confirm")} onConfirm={(e) => { e?.stopPropagation(); handleDeleteWorkflow(workflow.id); }}>
                        <Button type="text" size="small" danger icon={<DeleteOutlined />} onClick={(e) => e.stopPropagation()} />
                      </Popconfirm>
                    ]}
                  >
                    <div style={{ width: '100%' }}>
                      <div style={{ fontWeight: 500 }}>{workflow.name}</div>
                      <div style={{ fontSize: 12, color: token.colorTextSecondary }}>{workflow.steps.length} steps</div>
                    </div>
                  </List.Item>
                )}
              />
            )}
          </div>
        </div>

        <div style={{ flex: 1, position: 'relative' }}>
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
                  <Collapse ghost size="small" defaultActiveKey={['1', '2', '3']}>
                    <Collapse.Panel header={t("workflow.category.element_actions")} key="1">
                      <Space wrap size={[8, 8]}>
                        {STEP_TYPES.ELEMENT_ACTIONS.map(s => (
                          <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                            <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                          </Tooltip>
                        ))}
                      </Space>
                    </Collapse.Panel>
                    <Collapse.Panel header={t("workflow.category.wait_conditions")} key="2">
                      <Space wrap size={[8, 8]}>
                        {STEP_TYPES.WAIT_CONDITIONS.map(s => (
                          <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                            <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                          </Tooltip>
                        ))}
                      </Space>
                    </Collapse.Panel>
                    <Collapse.Panel header={t("workflow.category.flow_control")} key="3">
                      <Space wrap size={[8, 8]}>
                        {STEP_TYPES.FLOW_CONTROL.map(s => (
                          <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                            <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                          </Tooltip>
                        ))}
                      </Space>
                    </Collapse.Panel>
                    <Collapse.Panel header={t("workflow.category.system_actions")} key="4">
                      <Space wrap size={[8, 8]}>
                        {STEP_TYPES.SYSTEM_ACTIONS.map(s => (
                          <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                            <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                          </Tooltip>
                        ))}
                      </Space>
                    </Collapse.Panel>
                    <Collapse.Panel header={t("workflow.category.nested")} key="5">
                      <Space wrap size={[8, 8]}>
                        {STEP_TYPES.NESTED.map(s => (
                          <Tooltip title={t(`workflow.step_type.${s.key}`)} key={s.key}>
                            <Button size="small" icon={s.icon} onClick={() => handleAddStep(s.key)} />
                          </Tooltip>
                        ))}
                      </Space>
                    </Collapse.Panel>
                  </Collapse>
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

        <Drawer
          title={t("workflow.edit_step")}
          placement="right"
          onClose={() => setDrawerVisible(false)}
          open={drawerVisible}
          width={320}
          mask={false}
          style={{ background: token.colorBgContainer }}
        >
          {editingNodeId && (
            <>
              <Form layout="vertical" form={stepForm}>
                <Form.Item name="type" label={t("workflow.step_type_label")}>
                  <Select
                    disabled
                    options={Object.values(STEP_TYPES).flat().map(t => ({ label: t.key, value: t.key }))}
                  />
                </Form.Item>
                <Form.Item name="name" label={t("workflow.name")}>
                  <Input placeholder={t("workflow.name")} />
                </Form.Item>

                <Form.Item
                  noStyle
                  shouldUpdate={(prev, cur) => prev.type !== cur.type}
                >
                  {({ getFieldValue }) => {
                    const type = getFieldValue('type');
                    const isBranch = type === 'branch';
                    const needsSelector = ['click_element', 'long_click_element', 'input_text', 'swipe_element', 'wait_element', 'wait_gone', 'assert_element', 'branch'].includes(type);
                    const needsValue = ['input_text', 'wait', 'adb', 'script', 'run_workflow'].includes(type);
                    const isWorkflow = type === 'run_workflow';

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
                        {needsSelector && (
                          <>
                            <Form.Item label={isBranch ? t("workflow.branch_condition") : t("workflow.selector_type")}>
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
                                <Button icon={<AimOutlined />} onClick={() => setElementPickerVisible(true)} />
                              </div>
                            </Form.Item>
                            <Form.Item name="selectorValue" label={t("workflow.selector_value")}>
                              <Input.TextArea
                                placeholder={t("workflow.selector_placeholder")}
                                autoSize={{ minRows: 1, maxRows: 6 }}
                              />
                            </Form.Item>
                          </>
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

                        {needsValue && (
                          <Form.Item name="value" label={t("workflow.value")}>
                            {type === 'script' ? (
                              <Select
                                placeholder={t("workflow.select_script")}
                                options={scripts.map(s => ({ label: s.name, value: s.name }))}
                              />
                            ) : type === 'wait' ? (
                              <InputNumber addonAfter="ms" min={100} step={100} style={{ width: '100%' }} />
                            ) : (
                              <Input placeholder={type === 'adb' ? 'shell input keyevent 4' : t("workflow.value_placeholder")} />
                            )}
                          </Form.Item>
                        )}
                      </>
                    );
                  }}
                </Form.Item>

                <div style={{ display: 'flex', gap: 16 }}>
                  <Form.Item name="timeout" label={t("workflow.timeout")} style={{ flex: 1 }}>
                    <InputNumber addonAfter="ms" min={100} step={1000} style={{ width: '100%' }} placeholder="5000" />
                  </Form.Item>
                  <Form.Item name="loop" label={t("workflow.loop")} style={{ width: 100 }}>
                    <InputNumber min={1} style={{ width: '100%' }} placeholder="1" />
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

                <Button type="primary" block onClick={() => handleUpdateStep(stepForm.getFieldsValue())}>
                  {t("workflow.update")}
                </Button>

                <Divider style={{ margin: '12px 0' }} />

                <Button danger block icon={<DeleteOutlined />} onClick={handleDeleteNode}>
                  {t("workflow.delete_step")}
                </Button>
              </Form>
            </>
          )}
        </Drawer>
      </div>

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

      <ElementPicker
        visible={elementPickerVisible}
        onCancel={() => setElementPickerVisible(false)}
        onSelect={handleElementSelected}
      />
    </div>
  );
};

export default WorkflowView;
