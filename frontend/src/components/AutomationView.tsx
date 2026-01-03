import React, { useState, useEffect, useRef } from "react";
import {
  Button,
  Space,
  Tag,
  Card,
  List,
  Modal,
  Input,
  message,
  theme,
  Progress,
  Empty,
  Popconfirm,
  Tooltip,
  Checkbox,
  Dropdown,
} from "antd";
import { useTranslation } from "react-i18next";
import {
  PlayCircleOutlined,
  StopOutlined,
  DeleteOutlined,
  SaveOutlined,
  CaretRightOutlined,
  RobotOutlined,
  PlusOutlined,
  ClockCircleOutlined,
  FileTextOutlined,
  PauseCircleOutlined,
  EditOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
  InfoCircleOutlined,
  BranchesOutlined,
  ForkOutlined,
} from "@ant-design/icons";
import { Tabs, Form, InputNumber, Select, Divider } from "antd";
import DeviceSelector from "./DeviceSelector";
import { useDeviceStore, useAutomationStore, TouchScript, ScriptTask, TaskStep } from "../stores";
import { main } from "../../wailsjs/go/models";

const AutomationView: React.FC = () => {
  const { selectedDevice } = useDeviceStore();
  const {
    isRecording,
    isPlaying,
    recordingDeviceId,
    playingDeviceId,
    recordingDuration,
    recordedActionCount,
    currentScript,
    scripts,
    playbackProgress,
    startRecording,
    stopRecording,
    playScript,
    stopPlayback,
    loadScripts,
    saveScript,
    deleteScript,
    deleteScripts,
    renameScript,
    setCurrentScript,
    updateRecordingDuration,
    subscribeToEvents,
    // Task Actions
    tasks,
    loadTasks,
    saveTask,
    deleteTask,
    deleteTasks,
    runTask,
    isTaskRunning,
    runningTaskName,
    taskProgress,
    isPaused,
    pauseTask,
    resumeTask,
    stopTask,
  } = useAutomationStore();

  const [selectedScriptNames, setSelectedScriptNames] = useState<string[]>([]);
  const [selectedTaskNames, setSelectedTaskNames] = useState<string[]>([]);

  const { t } = useTranslation();
  const { token } = theme.useToken();

  const [saveModalVisible, setSaveModalVisible] = useState(false);
  const [scriptName, setScriptName] = useState("");

  // Rename Script State
  const [renameModalVisible, setRenameModalVisible] = useState(false);
  const [editingScriptName, setEditingScriptName] = useState(""); // Old name
  const [newScriptName, setNewScriptName] = useState("");

  const [selectedScript, setSelectedScript] = useState<TouchScript | null>(null);
  const durationIntervalRef = useRef<number | null>(null);

  // Task State
  const [taskModalVisible, setTaskModalVisible] = useState(false);
  const [editingTask, setEditingTask] = useState<ScriptTask | null>(null);
  const [taskForm] = Form.useForm();
  const [activeTab, setActiveTab] = useState("scripts");

  // Subscribe to events on mount
  useEffect(() => {
    const unsubscribe = subscribeToEvents();
    loadScripts();
    loadTasks();
    return () => {
      unsubscribe();
      if (durationIntervalRef.current) {
        clearInterval(durationIntervalRef.current);
      }
    };
  }, []);

  // Update recording duration
  useEffect(() => {
    if (isRecording) {
      durationIntervalRef.current = window.setInterval(() => {
        updateRecordingDuration();
      }, 1000);
    } else {
      if (durationIntervalRef.current) {
        clearInterval(durationIntervalRef.current);
        durationIntervalRef.current = null;
      }
    }
    return () => {
      if (durationIntervalRef.current) {
        clearInterval(durationIntervalRef.current);
      }
    };
  }, [isRecording]);

  const handleStartRecording = async () => {
    if (!selectedDevice) {
      message.warning(t("app.select_device"));
      return;
    }
    try {
      await startRecording(selectedDevice);
      message.success(t("automation.recording"));
    } catch (err) {
      message.error(String(err));
    }
  };

  const handleStopRecording = async () => {
    try {
      const script = await stopRecording();
      if (script && script.events && script.events.length > 0) {
        setCurrentScript(script);
        message.success(t("automation.events_count", { count: script.events.length }));
      } else {
        message.warning(t("automation.no_events"));
      }
    } catch (err) {
      message.error(String(err));
    }
  };

  const handleSaveScript = async () => {
    if (!currentScript) return;
    if (!scriptName.trim()) {
      message.warning(t("automation.enter_name"));
      return;
    }

    try {
      const scriptToSave = main.TouchScript.createFrom({
        ...currentScript,
        name: scriptName.trim(),
      });
      await saveScript(scriptToSave);
      message.success(t("automation.script_saved"));
      setSaveModalVisible(false);
      setScriptName("");
      setCurrentScript(null);
    } catch (err) {
      message.error(String(err));
    }
  };

  const handlePlayScript = async (script: TouchScript) => {
    if (!selectedDevice) {
      message.warning(t("app.select_device"));
      return;
    }
    try {
      await playScript(selectedDevice, script);
      setSelectedScript(script);
    } catch (err) {
      message.error(String(err));
    }
  };

  const handleStopPlayback = () => {
    stopPlayback();
    setSelectedScript(null);
  };

  const handleDeleteScript = async (name: string) => {
    try {
      await deleteScript(name);
      message.success(t("automation.script_deleted"));
    } catch (err) {
      message.error(String(err));
    }
  };

  const handleBulkDeleteScripts = async () => {
    if (selectedScriptNames.length === 0) return;
    try {
      await deleteScripts(selectedScriptNames);
      message.success(t("automation.scripts_deleted_msg", { count: selectedScriptNames.length }));
      setSelectedScriptNames([]);
    } catch (err) {
      message.error(String(err));
    }
  };

  const handleBulkDeleteTasks = async () => {
    if (selectedTaskNames.length === 0) return;
    try {
      await deleteTasks(selectedTaskNames);
      message.success(t("automation.tasks_deleted_msg", { count: selectedTaskNames.length }));
      setSelectedTaskNames([]);
    } catch (err) {
      message.error(String(err));
    }
  };

  const handleRenameScript = async () => {
    if (!newScriptName) return;
    try {
      await renameScript(editingScriptName, newScriptName);
      message.success(t("automation.script_renamed"));
      setRenameModalVisible(false);
      setEditingScriptName("");
      setNewScriptName("");
    } catch (err) {
      message.error(String(err));
    }
  };

  const handleConvertToWorkflow = async (script: TouchScript) => {
    if (!script.events || script.events.length === 0) {
      message.warning(t("automation.no_events_to_convert"));
      return;
    }

    const steps: main.WorkflowStep[] = [];
    let lastTimestamp = 0;
    const now = Date.now();

    // Add Start node
    const startId = `step_start_${now}`;
    steps.push({
      id: startId,
      type: 'start',
      name: 'Start',
      posX: 250,
      posY: 50,
    } as main.WorkflowStep);

    script.events.forEach((event, idx) => {
      // Add wait step if there is a significant delay
      const delay = event.timestamp - lastTimestamp;
      if (delay > 50) { // 50ms threshold
        steps.push({
          id: `step_wait_${now}_${idx}`,
          type: 'wait',
          name: `Wait ${delay}ms`,
          value: String(delay),
          loop: 1,
          postDelay: 0,
          posX: 250,
          posY: 150 + steps.length * 100,
        } as main.WorkflowStep);
      }
      lastTimestamp = event.timestamp;

      // Add Action Step
      const id = `step_action_${now}_${idx}`;

      const buildStepName = (action: string, event: main.TouchEvent, fallback: string) => {
        if (event.selector && event.selector.value) {
          const val = event.selector.value;
          const shortVal = val.split('/').pop() || val;
          return `${action}: "${shortVal}"`;
        }
        return fallback;
      };

      if (event.type === 'tap') {
        const selector = event.selector;
        if (selector && selector.type !== 'coordinates') {
          steps.push({
            id,
            type: 'click_element',
            name: buildStepName('Click', event, `Click ${idx + 1}`),
            selector: { ...selector },
            loop: 1,
            postDelay: 0,
            onError: 'stop',
            posX: 250,
            posY: 150 + steps.length * 100,
          } as main.WorkflowStep);
        } else {
          steps.push({
            id,
            type: 'click_element',
            name: `Click (${event.x}, ${event.y})`,
            selector: { type: 'bounds', value: `[${event.x},${event.y}]`, index: 0 },
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
            name: buildStepName('Long Click', event, `Long Click ${idx + 1}`),
            selector: { ...selector },
            loop: 1,
            postDelay: 0,
            onError: 'stop',
            posX: 250,
            posY: 150 + steps.length * 100,
          } as main.WorkflowStep);
        } else {
          steps.push({
            id,
            type: 'adb',
            name: `Long Press (${event.x}, ${event.y})`,
            value: `shell input swipe ${event.x} ${event.y} ${event.x} ${event.y} ${event.duration || 1000}`,
            loop: 1,
            postDelay: 0,
            onError: 'stop',
            posX: 250,
            posY: 150 + steps.length * 100,
          } as main.WorkflowStep);
        }
      } else if (event.type === 'swipe') {
        steps.push({
          id,
          type: 'adb',
          name: `Swipe ${idx + 1}`,
          value: `shell input swipe ${event.x} ${event.y} ${event.x2 || event.x} ${event.y2 || event.y} ${event.duration || 300}`,
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
          name: `Wait ${idx + 1}`,
          value: String(event.duration || 1000),
          loop: 1,
          postDelay: 0,
          posX: 250,
          posY: 150 + steps.length * 100,
        } as main.WorkflowStep);
      }
    });

    // Link steps
    for (let i = 0; i < steps.length - 1; i++) {
      steps[i].nextStepId = steps[i + 1].id;
    }

    const newWorkflow = new main.Workflow({
      id: `wf_converted_${now}`,
      name: `${script.name} (Converted)`,
      description: `Converted from recording on ${new Date().toLocaleString()}`,
      steps: steps,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    });

    try {
      await (window as any).go.main.App.SaveWorkflow(newWorkflow);
      message.success(t("automation.converted_success"));
    } catch (err) {
      message.error(t("automation.convert_failed") + ": " + String(err));
    }
  };

  // Task Handlers
  const handleCreateTask = async (values: any) => {
    const steps: TaskStep[] = values.steps.map((s: any) => {
      let val = "";
      if (s.type === "wait") val = String(s.duration || 1000);
      else if (s.type === "script") val = s.scriptName;
      else if (s.type === "adb") val = s.adbCommand;

      return {
        type: s.type,
        value: val,
        loop: Number(s.loop || 1),
        postDelay: Number(s.postDelay || 0),
        checkType: s.checkType,
        checkValue: s.checkValue,
        waitTimeout: Number(s.waitTimeout || 5000),
        onFailure: s.onFailure || "stop",
      };
    });

    const newTask: ScriptTask = {
      name: values.name,
      steps: steps,
      createdAt: new Date().toISOString(),
    };

    try {
      await saveTask(newTask);

      // If editing and name changed, delete old task
      if (editingTask && editingTask.name !== newTask.name) {
        await deleteTask(editingTask.name);
      }

      message.success(t("automation.task_saved"));
      setTaskModalVisible(false);
      setEditingTask(null);
      taskForm.resetFields();
    } catch (err) {
      message.error(String(err));
    }
  };

  const handleEditTask = (task: ScriptTask) => {
    setEditingTask(task);
    const formValues = {
      name: task.name,
      steps: task.steps.map(s => ({
        type: s.type,
        loop: s.loop,
        postDelay: s.postDelay,
        duration: s.type === 'wait' ? Number(s.value) : undefined,
        scriptName: s.type === 'script' ? s.value : undefined,
        adbCommand: s.type === 'adb' ? s.value : undefined,
        checkType: s.checkType,
        checkValue: s.checkValue,
        waitTimeout: s.waitTimeout,
        onFailure: s.onFailure,
      }))
    };
    taskForm.setFieldsValue(formValues);
    setTaskModalVisible(true);
  };

  const handleRunTask = async (task: ScriptTask) => {
    if (!selectedDevice) {
      message.warning(t("app.select_device"));
      return;
    }
    try {
      await runTask(selectedDevice, task);
      message.info(t("automation.running"));
    } catch (err) {
      message.error(String(err));
    }
  };

  const handleDeleteTask = async (name: string) => {
    try {
      await deleteTask(name);
      message.success(t("automation.task_deleted"));
    } catch (err) {
      message.error(String(err));
    }
  };

  const formatDuration = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  };

  const formatEventDescription = (event: any, index: number) => {
    switch (event.type) {
      case "tap":
        return `${index + 1}. tap (${event.x}, ${event.y}) @ ${event.timestamp}ms`;
      case "swipe":
        return `${index + 1}. swipe (${event.x}, ${event.y}) → (${event.x2}, ${event.y2}) ${event.duration}ms @ ${event.timestamp}ms`;
      case "wait":
        return `${index + 1}. wait ${event.duration}ms`;
      default:
        return `${index + 1}. unknown`;
    }
  };

  const isDeviceRecording = isRecording && recordingDeviceId === selectedDevice;
  const isDevicePlaying = isPlaying && playingDeviceId === selectedDevice;

  return (
    <div
      style={{
        padding: "16px 24px",
        height: "100%",
        display: "flex",
        flexDirection: "column",
        overflow: "hidden",
      }}
    >
      {/* Header */}
      <div
        style={{
          marginBottom: 16,
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          flexShrink: 0,
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <h2 style={{ margin: 0, color: token.colorText }}>{t("automation.title")}</h2>
          <Tag color="purple">{t("automation.touch_script")}</Tag>
        </div>
        <DeviceSelector />
      </div>

      {/* Main content */}
      <div style={{ flex: 1, overflowY: "auto", paddingRight: 8 }}>
        <div style={{ display: "flex", gap: 16, alignItems: "flex-start" }}>
          {/* Left Column - Recording Control */}
          <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: 16, overflow: "hidden" }}>
            {/* Recording Card */}
            <Card
              title={
                <Space>
                  <RobotOutlined />
                  {t("automation.record_control")}
                </Space>
              }
              size="small"
              style={{
                border: isDeviceRecording ? `1px solid ${token.colorError}` : undefined,
                backgroundColor: isDeviceRecording ? token.colorErrorBg : undefined,
              }}
            >
              <Space direction="vertical" style={{ width: "100%" }}>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                  <span>{t("automation.touch_on_device")}</span>
                  {isDeviceRecording ? (
                    <Button
                      type="primary"
                      danger
                      icon={<StopOutlined />}
                      onClick={handleStopRecording}
                    >
                      {t("automation.stop_record")}
                    </Button>
                  ) : (
                    <Button
                      type="primary"
                      icon={<PlayCircleOutlined />}
                      onClick={handleStartRecording}
                      disabled={!selectedDevice || isPlaying}
                    >
                      {t("automation.start_record")}
                    </Button>
                  )}
                </div>

                {isDeviceRecording && (
                  <div
                    style={{
                      padding: 12,
                      backgroundColor: token.colorErrorBg,
                      borderRadius: 8,
                      border: `1px dashed ${token.colorErrorBorder}`,
                    }}
                  >
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                      <Tag color="error" icon={<div className="record-dot" />}>
                        {t("automation.recording")}
                      </Tag>
                      <span style={{ fontWeight: "bold", fontFamily: "monospace" }}>
                        {formatDuration(recordingDuration)} · {recordedActionCount} {t("automation.events")}
                      </span>
                    </div>
                    <div style={{ fontSize: 12, color: token.colorTextSecondary, marginTop: 8 }}>
                      {t("automation.touch_device_tip")}
                    </div>
                  </div>
                )}

                {currentScript && currentScript.events && currentScript.events.length > 0 && (
                  <div
                    style={{
                      padding: 12,
                      backgroundColor: token.colorSuccessBg,
                      borderRadius: 8,
                      border: `1px solid ${token.colorSuccessBorder}`,
                    }}
                  >
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                      <span>
                        {t("automation.events_count", { count: currentScript.events.length })}
                      </span>
                      <Space>
                        <Tooltip title={t("automation.test_tooltip")}>
                          <Button
                            icon={<CaretRightOutlined />}
                            size="small"
                            onClick={() => handlePlayScript(currentScript)}
                            disabled={isPlaying || !selectedDevice}
                          >
                            {t("automation.test_script")}
                          </Button>
                        </Tooltip>
                        <Tooltip title={t("automation.convert_to_workflow")}>
                          <Button
                            icon={<BranchesOutlined />}
                            size="small"
                            onClick={() => handleConvertToWorkflow(currentScript)}
                            disabled={isRecording || isPlaying}
                          >
                            {t("workflow.convert")}
                          </Button>
                        </Tooltip>
                        <Button
                          type="primary"
                          icon={<SaveOutlined />}
                          size="small"
                          onClick={() => setSaveModalVisible(true)}
                        >
                          {t("automation.save_script")}
                        </Button>
                      </Space>
                    </div>
                  </div>
                )}
              </Space>
            </Card>

            {/* Playback Progress */}
            {isDevicePlaying && playbackProgress && (
              <Card
                title={
                  <Space>
                    <CaretRightOutlined />
                    {t("automation.playing")}
                  </Space>
                }
                size="small"
                style={{
                  border: `1px solid ${token.colorPrimaryBorder}`,
                  backgroundColor: token.colorPrimaryBg,
                }}
              >
                <div style={{ marginBottom: 12 }}>
                  <Progress
                    percent={Math.round((playbackProgress.current / playbackProgress.total) * 100)}
                    status="active"
                    format={() => `${playbackProgress.current}/${playbackProgress.total}`}
                  />
                </div>
                <Button danger icon={<StopOutlined />} onClick={handleStopPlayback}>
                  {t("automation.stop")}
                </Button>
              </Card>
            )}

            {/* Task Progress */}
            {isTaskRunning && taskProgress && (
              <Card
                title={
                  <Space>
                    <RobotOutlined spin={true} />
                    {t("automation.running")}: {runningTaskName}
                    {isPaused && <Tag color="warning">{t("automation.paused")}</Tag>}
                  </Space>
                }
                extra={
                  <Space>
                    {isPaused ? (
                      <Tooltip title={t("automation.resume")}>
                        <Button
                          type="primary"
                          shape="circle"
                          icon={<CaretRightOutlined />}
                          size="small"
                          onClick={() => resumeTask()}
                        />
                      </Tooltip>
                    ) : (
                      <Tooltip title={t("automation.pause")}>
                        <Button
                          type="default"
                          shape="circle"
                          icon={<PauseCircleOutlined />}
                          size="small"
                          onClick={() => pauseTask()}
                        />
                      </Tooltip>
                    )}
                    <Tooltip title={t("automation.stop_task")}>
                      <Button
                        type="primary"
                        danger
                        shape="circle"
                        icon={<StopOutlined />}
                        size="small"
                        onClick={() => stopTask()}
                      />
                    </Tooltip>
                  </Space>
                }
                size="small"
                style={{
                  border: `1px solid ${token.colorPrimaryBorder}`,
                  backgroundColor: token.colorPrimaryBg,
                  marginTop: 16,
                }}
              >
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  <div>
                    <strong>{t("automation.step")}:</strong> {taskProgress.stepIndex + 1} / {taskProgress.totalSteps}
                  </div>
                  <div>
                    <strong>{t("automation.loop")}:</strong> {taskProgress.currentLoop} / {taskProgress.totalLoops}
                  </div>
                  <div style={{ color: token.colorTextSecondary }}>
                    {taskProgress.currentAction}
                  </div>
                  <Progress
                    percent={Math.round(((taskProgress.stepIndex) / taskProgress.totalSteps) * 100)}
                    status="active"
                    showInfo={false}
                  />
                </div>
              </Card>
            )}

            {/* Script Preview */}
            {(currentScript || selectedScript) && (
              <Card
                title={t("automation.script_preview")}
                size="small"
                style={{ maxHeight: 300, overflow: "auto" }}
              >
                <div style={{ fontFamily: "monospace", fontSize: 12 }}>
                  {((currentScript || selectedScript)?.events || []).map((event, idx) => (
                    <div
                      key={idx}
                      style={{
                        padding: "4px 0",
                        borderBottom: `1px solid ${token.colorBorderSecondary}`,
                        color:
                          isDevicePlaying && playbackProgress && idx < playbackProgress.current
                            ? token.colorSuccess
                            : token.colorText,
                        wordBreak: "break-all",
                        overflowWrap: "anywhere",
                      }}
                    >
                      {formatEventDescription(event, idx)}
                    </div>
                  ))}
                </div>
              </Card>
            )}
          </div>

          {/* Right Column - Script/Task List */}
          <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
            <Tabs
              activeKey={activeTab}
              onChange={setActiveTab}
              items={[
                {
                  key: "scripts",
                  label: (
                    <span>
                      <FileTextOutlined /> {t("automation.scripts")}
                    </span>
                  ),
                  children: (
                    <Card
                      title={
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                          <Space>
                            <Checkbox
                              indeterminate={selectedScriptNames.length > 0 && selectedScriptNames.length < scripts.length}
                              checked={scripts.length > 0 && selectedScriptNames.length === scripts.length}
                              onChange={(e) => setSelectedScriptNames(e.target.checked ? scripts.map(s => s.name) : [])}
                            />
                            <SaveOutlined />
                            {t("automation.saved_scripts")}
                            {selectedScriptNames.length > 0 && (
                              <Popconfirm
                                title={t("automation.delete_selected_confirm", { count: selectedScriptNames.length })}
                                onConfirm={handleBulkDeleteScripts}
                                okText={t("common.ok")}
                                cancelText={t("common.cancel")}
                              >
                                <Button danger size="small" icon={<DeleteOutlined />}>
                                  {t("automation.delete_selected")} ({selectedScriptNames.length})
                                </Button>
                              </Popconfirm>
                            )}
                          </Space>
                          <Tooltip title={t("automation.scaling_info")}>
                            <Tag icon={<InfoCircleOutlined />} color="blue" style={{ margin: 0 }}>
                              {t("automation.auto_scaling")}
                            </Tag>
                          </Tooltip>
                        </div>
                      }
                      size="small"
                      styles={{ body: { padding: 0, overflowY: "auto", maxHeight: "calc(100vh - 200px)" } }}
                    >
                      {scripts.length === 0 ? (
                        <Empty description={t("automation.no_scripts")} image={Empty.PRESENTED_IMAGE_SIMPLE} />
                      ) : (
                        <List
                          size="small"
                          dataSource={scripts}
                          renderItem={(script) => (
                            <List.Item
                              style={{
                                cursor: "pointer",
                                padding: "8px 16px",
                                backgroundColor:
                                  selectedScript?.name === script.name ? token.colorPrimaryBg : undefined,
                              }}
                              onClick={() => {
                                setSelectedScript(script);
                                setCurrentScript(null);
                              }}
                              actions={[
                                <Tooltip title={t("automation.play")} key="play">
                                  <Button
                                    type="text"
                                    size="small"
                                    icon={<CaretRightOutlined />}
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      handlePlayScript(script);
                                    }}
                                    disabled={isRecording || isPlaying || !selectedDevice}
                                  />
                                </Tooltip>,
                                <Tooltip title={t("common.rename")} key="rename">
                                  <Button
                                    type="text"
                                    size="small"
                                    icon={<EditOutlined />}
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      setEditingScriptName(script.name);
                                      setNewScriptName(script.name);
                                      setRenameModalVisible(true);
                                    }}
                                    disabled={isRecording || isPlaying}
                                  />
                                </Tooltip>,
                                <Tooltip title={t("workflow.convert")} key="convert">
                                  <Button
                                    type="text"
                                    size="small"
                                    icon={<BranchesOutlined />}
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      handleConvertToWorkflow(script);
                                    }}
                                    disabled={isRecording || isPlaying}
                                  />
                                </Tooltip>,
                                <Popconfirm
                                  key="delete"
                                  title={t("automation.delete_confirm", { name: script.name })}
                                  onConfirm={() => handleDeleteScript(script.name)}
                                  okText={t("common.ok")}
                                  cancelText={t("common.cancel")}
                                >
                                  <Tooltip title={t("automation.delete")}>
                                    <Button
                                      type="text"
                                      size="small"
                                      danger
                                      icon={<DeleteOutlined />}
                                      onClick={(e) => e.stopPropagation()}
                                    />
                                  </Tooltip>
                                </Popconfirm>,
                              ]}
                            >
                              <div style={{ display: 'flex', alignItems: 'center', width: '100%' }}>
                                <Checkbox
                                  checked={selectedScriptNames.includes(script.name)}
                                  onChange={(e) => {
                                    e.stopPropagation();
                                    const name = script.name;
                                    setSelectedScriptNames(prev =>
                                      e.target.checked ? [...prev, name] : prev.filter(n => n !== name)
                                    );
                                  }}
                                  style={{ marginRight: 12 }}
                                />
                                <List.Item.Meta
                                  title={<span style={{ wordBreak: 'break-all' }}>{script.name}</span>}
                                  description={
                                    <span style={{ fontSize: 11, color: token.colorTextSecondary }}>
                                      {script.events?.length || 0} {t("automation.events")} · {script.resolution}
                                      {(script as any).deviceModel ? ` · ${(script as any).deviceModel}` : null}
                                    </span>
                                  }
                                />
                              </div>
                            </List.Item>
                          )}
                        />
                      )}
                    </Card>
                  ),
                },
                {
                  key: "tasks",
                  label: (
                    <span>
                      <RobotOutlined /> {t("automation.saved_tasks")}
                    </span>
                  ),
                  children: (
                    <Card
                      title={
                        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                          <Space>
                            <Checkbox
                              indeterminate={selectedTaskNames.length > 0 && selectedTaskNames.length < tasks.length}
                              checked={tasks.length > 0 && selectedTaskNames.length === tasks.length}
                              onChange={(e) => setSelectedTaskNames(e.target.checked ? tasks.map(t => t.name) : [])}
                            />
                            <span>{t("automation.saved_tasks")}</span>
                            {selectedTaskNames.length > 0 && (
                              <Popconfirm
                                title={t("automation.delete_selected_confirm", { count: selectedTaskNames.length })}
                                onConfirm={handleBulkDeleteTasks}
                                okText={t("common.ok")}
                                cancelText={t("common.cancel")}
                              >
                                <Button danger size="small" icon={<DeleteOutlined />}>
                                  {t("automation.delete_selected")} ({selectedTaskNames.length})
                                </Button>
                              </Popconfirm>
                            )}
                          </Space>
                          <Button
                            type="primary"
                            size="small"
                            icon={<PlusOutlined />}
                            onClick={() => {
                              setEditingTask(null);
                              taskForm.resetFields();
                              setTaskModalVisible(true);
                            }}
                          >
                            {t("automation.create_task")}
                          </Button>
                        </div>
                      }
                      size="small"
                      styles={{ body: { padding: 0, overflowY: "auto", maxHeight: "calc(100vh - 200px)" } }}
                    >
                      {tasks.length === 0 ? (
                        <Empty description={t("automation.no_tasks")} image={Empty.PRESENTED_IMAGE_SIMPLE} />
                      ) : (
                        <List
                          size="small"
                          dataSource={tasks}
                          renderItem={(task) => (
                            <List.Item
                              style={{ padding: "8px 16px" }}
                              actions={[
                                <Tooltip title={t("automation.run_task")} key="run">
                                  <Button
                                    type="text"
                                    size="small"
                                    icon={<CaretRightOutlined />}
                                    onClick={() => handleRunTask(task)}
                                    loading={isTaskRunning && runningTaskName === task.name}
                                    disabled={isTaskRunning || isRecording || isPlaying}
                                  />
                                </Tooltip>,
                                <Tooltip title={t("automation.edit_task")} key="edit">
                                  <Button
                                    type="text"
                                    size="small"
                                    icon={<EditOutlined />}
                                    onClick={() => handleEditTask(task)}
                                    disabled={isTaskRunning}
                                  />
                                </Tooltip>,
                                <Popconfirm
                                  key="delete"
                                  title={t("automation.delete_task_confirm", { name: task.name })}
                                  onConfirm={() => handleDeleteTask(task.name)}
                                  okText={t("common.ok")}
                                  cancelText={t("common.cancel")}
                                >
                                  <Button
                                    type="text"
                                    size="small"
                                    danger
                                    icon={<DeleteOutlined />}
                                  />
                                </Popconfirm>,
                              ]}
                            >
                              <div style={{ display: 'flex', alignItems: 'center', width: '100%' }}>
                                <Checkbox
                                  checked={selectedTaskNames.includes(task.name)}
                                  onChange={(e) => {
                                    e.stopPropagation();
                                    const name = task.name;
                                    setSelectedTaskNames(prev =>
                                      e.target.checked ? [...prev, name] : prev.filter(n => n !== name)
                                    );
                                  }}
                                  style={{ marginRight: 12 }}
                                />
                                <List.Item.Meta
                                  title={task.name}
                                  description={
                                    <span style={{ fontSize: 11, color: token.colorTextSecondary }}>
                                      {task.steps?.length || 0} {t("automation.steps_count")}
                                    </span>
                                  }
                                />
                              </div>
                            </List.Item>
                          )}
                        />
                      )}
                    </Card>
                  ),
                },
              ]}
            />

            {/* Save Script Modal */}
            <Modal
              title={t("automation.save_script")}
              open={saveModalVisible}
              onOk={handleSaveScript}
              onCancel={() => {
                setSaveModalVisible(false);
                setScriptName("");
              }}
              okText={t("common.ok")}
              cancelText={t("common.cancel")}
            >
              <Input
                placeholder={t("automation.script_name")}
                value={scriptName}
                onChange={(e) => setScriptName(e.target.value)}
                onPressEnter={handleSaveScript}
              />
            </Modal>

            {/* Rename Script Modal */}
            <Modal
              title={t("automation.rename_script")}
              open={renameModalVisible}
              onOk={handleRenameScript}
              onCancel={() => {
                setRenameModalVisible(false);
                setEditingScriptName("");
                setNewScriptName("");
              }}
              okText={t("common.ok")}
              cancelText={t("common.cancel")}
            >
              <Input
                placeholder={t("automation.new_script_name")}
                value={newScriptName}
                onChange={(e) => setNewScriptName(e.target.value)}
                onPressEnter={handleRenameScript}
              />
            </Modal>

            {/* New Task Modal */}
            <Modal
              title={editingTask ? t("automation.edit_task") : t("automation.create_task")}
              open={taskModalVisible}
              onOk={() => taskForm.submit()}
              onCancel={() => {
                setTaskModalVisible(false);
                setEditingTask(null);
                taskForm.resetFields();
              }}
              width={600}
            >
              <Form form={taskForm} layout="vertical" onFinish={handleCreateTask}>
                <Form.Item
                  name="name"
                  label={t("automation.task_name")}
                  rules={[{ required: true, message: t("automation.enter_name") }]}
                >
                  <Input placeholder={t("automation.task_name")} />
                </Form.Item>

                <Form.List name="steps">
                  {(fields, { add, remove, move }) => (
                    <>
                      {fields.map(({ key, name, ...restField }, index) => (
                        <Card
                          key={key}
                          size="small"
                          style={{ marginBottom: 8, background: token.colorBgLayout }}
                          styles={{ body: { padding: 8 } }}
                        >
                          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                            <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                              <Form.Item
                                {...restField}
                                name={[name, "type"]}
                                rules={[{ required: true }]}
                                style={{ width: 130, marginBottom: 0 }}
                              >
                                <Select
                                  options={[
                                    { label: t("automation.step_type.script"), value: "script" },
                                    { label: t("automation.step_type.wait"), value: "wait" },
                                    { label: t("automation.step_type.adb"), value: "adb" },
                                    { label: t("automation.step_type.check"), value: "check" },
                                  ]}
                                />
                              </Form.Item>

                              <div style={{ flex: 1, minWidth: 0 }}>
                                <Form.Item
                                  noStyle
                                  shouldUpdate={(prevValues, currentValues) =>
                                    prevValues.steps?.[name]?.type !== currentValues.steps?.[name]?.type
                                  }
                                >
                                  {({ getFieldValue }) => {
                                    const type = getFieldValue(["steps", name, "type"]);
                                    if (type === "wait") {
                                      return (
                                        <Form.Item
                                          {...restField}
                                          name={[name, "duration"]}
                                          rules={[{ required: true }]}
                                          style={{ marginBottom: 0 }}
                                        >
                                          <InputNumber
                                            addonBefore={t("automation.duration")}
                                            addonAfter="ms"
                                            placeholder={t("automation.duration")}
                                            min={100}
                                            step={100}
                                            style={{ width: "100%" }}
                                          />
                                        </Form.Item>
                                      );
                                    } else if (type === "adb") {
                                      return (
                                        <Form.Item
                                          {...restField}
                                          name={[name, "adbCommand"]}
                                          rules={[{ required: true, message: t("common.required") }]}
                                          style={{ marginBottom: 0 }}
                                        >
                                          <Input placeholder={t("automation.adb_placeholder")} />
                                        </Form.Item>
                                      );
                                    } else if (type === "check") {
                                      return (
                                        <div style={{ display: "flex", gap: 8 }}>
                                          <Form.Item
                                            {...restField}
                                            name={[name, "checkType"]}
                                            initialValue="text"
                                            style={{ width: 150, marginBottom: 0 }}
                                          >
                                            <Select
                                              options={[
                                                { label: t("automation.check_types.text"), value: "text" },
                                                { label: t("automation.check_types.contains"), value: "contains" },
                                                { label: t("automation.check_types.id"), value: "id" },
                                                { label: t("automation.check_types.description"), value: "description" },
                                                { label: t("automation.check_types.class"), value: "class" },
                                              ]}
                                            />
                                          </Form.Item>
                                          <Form.Item
                                            {...restField}
                                            name={[name, "checkValue"]}
                                            rules={[{ required: true, message: t("common.required") }]}
                                            style={{ flex: 1, marginBottom: 0 }}
                                          >
                                            <Input placeholder={t("automation.check_value")} />
                                          </Form.Item>
                                        </div>
                                      );
                                    } else if (type === "check") {
                                      return (
                                        <div style={{ display: "flex", gap: 8 }}>
                                          <Form.Item
                                            {...restField}
                                            name={[name, "checkType"]}
                                            initialValue="text"
                                            style={{ width: 150, marginBottom: 0 }}
                                          >
                                            <Select
                                              options={[
                                                { label: t("automation.check_types.text"), value: "text" },
                                                { label: t("automation.check_types.contains"), value: "contains" },
                                                { label: t("automation.check_types.id"), value: "id" },
                                                { label: t("automation.check_types.description"), value: "description" },
                                                { label: t("automation.check_types.class"), value: "class" },
                                              ]}
                                            />
                                          </Form.Item>
                                          <Form.Item
                                            {...restField}
                                            name={[name, "checkValue"]}
                                            rules={[{ required: true, message: t("common.required") }]}
                                            style={{ flex: 1, marginBottom: 0 }}
                                          >
                                            <Input placeholder={t("automation.check_value")} />
                                          </Form.Item>
                                        </div>
                                      );
                                    } else {
                                      return (
                                        <Form.Item
                                          {...restField}
                                          name={[name, "scriptName"]}
                                          rules={[{ required: true }]}
                                          style={{ marginBottom: 0 }}
                                        >
                                          <Select
                                            placeholder={t("automation.select_script")}
                                            options={scripts.map((s) => ({ label: s.name, value: s.name }))}
                                            style={{ width: "100%" }}
                                          />
                                        </Form.Item>
                                      );
                                    }
                                  }}
                                </Form.Item>
                              </div>
                            </div>

                            <div style={{ display: "flex", gap: 8, alignItems: "center", width: "100%" }}>
                              <div style={{ flex: 1, minWidth: 0, display: "flex", gap: 16 }}>
                                <div style={{ flex: 1, display: "flex", alignItems: "center", gap: 8 }}>
                                  <span style={{ whiteSpace: "nowrap" }}>{t("automation.loop")}</span>
                                  <Form.Item
                                    {...restField}
                                    name={[name, "loop"]}
                                    initialValue={1}
                                    style={{ flex: 1, marginBottom: 0 }}
                                  >
                                    <InputNumber
                                      min={1}
                                      style={{ width: "100%", maxWidth: 100 }}
                                    />
                                  </Form.Item>
                                </div>

                                <div style={{ flex: 1.5, display: "flex", alignItems: "center", gap: 8 }}>
                                  <span style={{ whiteSpace: "nowrap" }}>{t("automation.wait")}</span>
                                  <Form.Item
                                    {...restField}
                                    name={[name, "postDelay"]}
                                    initialValue={0}
                                    style={{ flex: 1, marginBottom: 0 }}
                                  >
                                    <InputNumber
                                      min={0}
                                      step={100}
                                      addonAfter="ms"
                                      placeholder="0"
                                      style={{ width: "100%", maxWidth: 160 }}
                                    />
                                  </Form.Item>
                                </div>
                              </div>
                            </div>

                            <Form.Item
                              noStyle
                              shouldUpdate={(prevValues, currentValues) =>
                                prevValues.steps?.[name]?.type !== currentValues.steps?.[name]?.type
                              }
                            >
                              {({ getFieldValue }) => {
                                const type = getFieldValue(["steps", name, "type"]);
                                if (type !== "check") return null;
                                return (
                                  <div style={{ display: "flex", gap: 8, alignItems: "center", width: "100%", marginTop: 8 }}>
                                    <div style={{ flex: 1, display: "flex", alignItems: "center", gap: 8 }}>
                                      <span style={{ whiteSpace: "nowrap" }}>{t("automation.wait_timeout")}</span>
                                      <Form.Item
                                        {...restField}
                                        name={[name, "waitTimeout"]}
                                        initialValue={5000}
                                        style={{ flex: 1, marginBottom: 0 }}
                                      >
                                        <InputNumber
                                          min={100}
                                          step={1000}
                                          addonAfter="ms"
                                          style={{ width: "100%", maxWidth: 160 }}
                                        />
                                      </Form.Item>
                                    </div>
                                    <div style={{ flex: 1, display: "flex", alignItems: "center", gap: 8 }}>
                                      <span style={{ whiteSpace: "nowrap" }}>{t("automation.on_failure")}</span>
                                      <Form.Item
                                        {...restField}
                                        name={[name, "onFailure"]}
                                        initialValue="stop"
                                        style={{ flex: 1, marginBottom: 0 }}
                                      >
                                        <Select
                                          options={[
                                            { label: t("automation.fail_stop"), value: "stop" },
                                            { label: t("automation.fail_continue"), value: "continue" },
                                          ]}
                                        />
                                      </Form.Item>
                                    </div>
                                    <div style={{ width: 44 }}></div>
                                  </div>
                                );
                              }}
                            </Form.Item>

                            <div style={{ display: "flex", gap: 8, alignItems: "center", width: "100%", justifyContent: 'flex-end', marginTop: 8 }}>
                              <div style={{ display: 'flex', gap: 2 }}>
                                <div style={{ display: 'flex', flexDirection: 'column' }}>
                                  <Button
                                    type="text"
                                    size="small"
                                    icon={<ArrowUpOutlined />}
                                    disabled={index === 0}
                                    onClick={() => move(index, index - 1)}
                                    style={{ height: 16, lineHeight: 1 }}
                                  />
                                  <Button
                                    type="text"
                                    size="small"
                                    icon={<ArrowDownOutlined />}
                                    disabled={index === fields.length - 1}
                                    onClick={() => move(index, index + 1)}
                                    style={{ height: 16, lineHeight: 1 }}
                                  />
                                </div>

                                <Button
                                  type="text"
                                  danger
                                  icon={<DeleteOutlined />}
                                  onClick={() => remove(name)}
                                />
                              </div>
                            </div>
                          </div>
                        </Card>
                      ))}
                      <Form.Item>
                        <Button type="dashed" onClick={() => add({ type: "script", loop: 1 })} block icon={<PlusOutlined />}>
                          {t("automation.add_step")}
                        </Button>
                      </Form.Item>
                    </>
                  )}
                </Form.List>
              </Form>
            </Modal>
          </div>
        </div>
      </div>
    </div >
  );
};

export default AutomationView;
