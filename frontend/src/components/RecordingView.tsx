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
  Radio,
  Alert,
  Badge,
} from "antd";
import { useTranslation } from "react-i18next";
import {
  PlayCircleOutlined,
  StopOutlined,
  DeleteOutlined,
  SaveOutlined,
  CaretRightOutlined,
  RobotOutlined,
  EditOutlined,
  InfoCircleOutlined,
  VideoCameraOutlined,
  BranchesOutlined,
  ThunderboltOutlined,
  AimOutlined,
  LoadingOutlined,
  CheckCircleOutlined,
} from "@ant-design/icons";
import DeviceSelector from "./DeviceSelector";
import { useDeviceStore, useAutomationStore, TouchScript } from "../stores";
import { main } from "../../wailsjs/go/models";

const RecordingView: React.FC = () => {
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
    recordingMode,
    isWaitingForSelector,
    pendingSelectorData,
    isPreCapturing,
    isAnalyzing,
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
    submitSelectorChoice,
    setRecordingMode,
  } = useAutomationStore();

  const [selectedScriptNames, setSelectedScriptNames] = useState<string[]>([]);

  const { t } = useTranslation();
  const { token } = theme.useToken();

  const [saveModalVisible, setSaveModalVisible] = useState(false);
  const [scriptName, setScriptName] = useState("");

  const [renameModalVisible, setRenameModalVisible] = useState(false);
  const [editingScriptName, setEditingScriptName] = useState("");
  const [newScriptName, setNewScriptName] = useState("");

  const [selectedScript, setSelectedScript] = useState<TouchScript | null>(null);
  const durationIntervalRef = useRef<number | null>(null);

  useEffect(() => {
    const unsubscribe = subscribeToEvents();
    loadScripts();
    return () => {
      unsubscribe();
      if (durationIntervalRef.current) {
        clearInterval(durationIntervalRef.current);
      }
    };
  }, []);

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

  const handleStartRecording = async (mode: 'fast' | 'precise' = 'fast') => {
    if (!selectedDevice) {
      message.warning(t("app.select_device"));
      return;
    }
    try {
      await startRecording(selectedDevice, mode);
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

  const handleExecuteSingleEvent = async (event: any, script: TouchScript) => {
    if (!selectedDevice) {
      message.warning(t("app.select_device"));
      return;
    }
    try {
      const touchEvent = main.TouchEvent.createFrom(event);
      await (window as any).go.main.App.ExecuteSingleTouchEvent(
        selectedDevice,
        touchEvent,
        script.resolution || ""
      );
      message.success(t("recording.event_executed"));
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

    const newWorkflow = new (main as any).Workflow({
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

  const formatDuration = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  };

  const formatEventDescription = (event: main.TouchEvent, index: number) => {
    // Build element info suffix if available
    let elementSuffix = '';
    if (event.selector && event.selector.value) {
      elementSuffix = ` [${event.selector.type}: ${event.selector.value}]`;
    }

    switch (event.type) {
      case "tap":
        return `${index + 1}. tap (${event.x}, ${event.y})${elementSuffix} @ ${event.timestamp}ms`;
      case "long_press":
        return `${index + 1}. long press (${event.x}, ${event.y})${elementSuffix} ${event.duration}ms @ ${event.timestamp}ms`;
      case "swipe":
        return `${index + 1}. swipe (${event.x}, ${event.y}) → (${event.x2}, ${event.y2})${elementSuffix} ${event.duration}ms @ ${event.timestamp}ms`;
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
          <h2 style={{ margin: 0, color: token.colorText }}>{t("recording.title")}</h2>
          <Tag color="blue">{t("recording.touch_script")}</Tag>
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
                  <VideoCameraOutlined />
                  {t("recording.record_control")}
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
                  <Space direction="vertical" size={0}>
                    <span style={{ fontWeight: 500 }}>{t("recording.mode")}</span>
                    <Radio.Group
                      value={recordingMode}
                      onChange={(e) => setRecordingMode(e.target.value)}
                      disabled={isDeviceRecording}
                      size="small"
                      optionType="button"
                      buttonStyle="solid"
                    >
                      <Tooltip title={t("recording.fast_mode_desc")}>
                        <Radio.Button value="fast">
                          <ThunderboltOutlined /> {t("recording.fast")}
                        </Radio.Button>
                      </Tooltip>
                      <Tooltip title={t("recording.precise_mode_desc")}>
                        <Radio.Button value="precise">
                          <AimOutlined /> {t("recording.precise")}
                        </Radio.Button>
                      </Tooltip>
                    </Radio.Group>
                  </Space>

                  {isDeviceRecording ? (
                    <Button
                      type="primary"
                      danger
                      icon={<StopOutlined />}
                      onClick={handleStopRecording}
                      size="large"
                    >
                      {t("recording.stop_record")}
                    </Button>
                  ) : (
                    <Button
                      type="primary"
                      icon={<PlayCircleOutlined />}
                      onClick={() => handleStartRecording(recordingMode)}
                      disabled={!selectedDevice || isPlaying}
                      size="large"
                    >
                      {t("recording.start_record")}
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
                        {t("recording.recording")}
                      </Tag>
                      <span style={{ fontWeight: "bold", fontFamily: "monospace" }}>
                        {formatDuration(recordingDuration)} · {recordedActionCount} {t("recording.events")}
                      </span>
                    </div>

                    {recordingMode === 'precise' && (
                      <div style={{
                        marginTop: 12,
                        borderTop: `1px solid ${token.colorErrorBorder}`,
                        paddingTop: 8,
                        display: 'flex',
                        alignItems: 'center',
                        gap: 8,
                        color: token.colorError
                      }}>
                        {isPreCapturing ? (
                          <>
                            <LoadingOutlined spin />
                            <span>{t("recording.status.dumping")}</span>
                          </>
                        ) : isAnalyzing ? (
                          <>
                            <LoadingOutlined spin />
                            <span>{t("recording.status.analyzing")}</span>
                          </>
                        ) : isWaitingForSelector ? (
                          <>
                            <AimOutlined />
                            <span>{t("recording.status.waiting_selector")}</span>
                          </>
                        ) : (
                          <>
                            <CheckCircleOutlined style={{ color: token.colorSuccess }} />
                            <span style={{ color: token.colorSuccess }}>{t("recording.status.ready")}</span>
                          </>
                        )}
                      </div>
                    )}

                    <div style={{ fontSize: 12, color: token.colorTextSecondary, marginTop: 8 }}>
                      {t("recording.touch_device_tip")}
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
                        {t("recording.events_count", { count: currentScript.events.length })}
                      </span>
                      <Space>
                        <Tooltip title={t("recording.test_tooltip")}>
                          <Button
                            icon={<CaretRightOutlined />}
                            size="small"
                            onClick={() => handlePlayScript(currentScript)}
                            disabled={isPlaying || !selectedDevice}
                          >
                            {t("recording.test_script")}
                          </Button>
                        </Tooltip>
                        <Tooltip title={t("automation.convert_to_workflow")}>
                          <Button
                            icon={<BranchesOutlined />}
                            size="small"
                            onClick={() => handleConvertToWorkflow(currentScript)}
                            disabled={isRecording || isPlaying}
                          >
                            {t("automation.convert")}
                          </Button>
                        </Tooltip>
                        <Button
                          type="primary"
                          icon={<SaveOutlined />}
                          size="small"
                          onClick={() => setSaveModalVisible(true)}
                        >
                          {t("recording.save_script")}
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
                    {t("recording.playing")}
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
                  {t("recording.stop")}
                </Button>
              </Card>
            )}

            {/* Script Preview */}
            {(currentScript || selectedScript) && (
              <Card
                title={t("recording.script_preview")}
                size="small"
                style={{ maxHeight: 300, overflow: "auto" }}
              >
                <div style={{ fontFamily: "monospace", fontSize: 12 }}>
                  {((currentScript || selectedScript)?.events || []).map((event, idx) => {
                    const activeScript = currentScript || selectedScript;
                    return (
                      <div
                        key={idx}
                        style={{
                          padding: "4px 0",
                          borderBottom: `1px solid ${token.colorBorderSecondary}`,
                          color:
                            isDevicePlaying && playbackProgress && idx < playbackProgress.current
                              ? token.colorSuccess
                              : token.colorText,
                          display: "flex",
                          alignItems: "center",
                          gap: 8,
                        }}
                      >
                        <Tooltip title={t("recording.execute_event")}>
                          <Button
                            type="text"
                            size="small"
                            icon={<ThunderboltOutlined />}
                            onClick={() => handleExecuteSingleEvent(event, activeScript!)}
                            disabled={!selectedDevice || isRecording || isPlaying}
                            style={{ padding: "0 4px", minWidth: 24 }}
                          />
                        </Tooltip>
                        <span style={{ flex: 1, wordBreak: "break-all", overflowWrap: "anywhere" }}>{formatEventDescription(event, idx)}</span>
                      </div>
                    );
                  })}
                </div>
              </Card>
            )}
          </div>

          {/* Right Column - Script List */}
          <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
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
                    {t("recording.saved_scripts")}
                    {selectedScriptNames.length > 0 && (
                      <Popconfirm
                        title={t("recording.delete_selected_confirm", { count: selectedScriptNames.length })}
                        onConfirm={handleBulkDeleteScripts}
                        okText={t("common.ok")}
                        cancelText={t("common.cancel")}
                      >
                        <Button danger size="small" icon={<DeleteOutlined />}>
                          {t("recording.delete_selected")} ({selectedScriptNames.length})
                        </Button>
                      </Popconfirm>
                    )}
                  </Space>
                  <Tooltip title={t("recording.scaling_info")}>
                    <Tag icon={<InfoCircleOutlined />} color="blue" style={{ margin: 0 }}>
                      {t("recording.auto_scaling")}
                    </Tag>
                  </Tooltip>
                </div>
              }
              size="small"
              styles={{ body: { padding: 0, overflowY: "auto", maxHeight: "calc(100vh - 200px)" } }}
            >
              {scripts.length === 0 ? (
                <Empty description={t("recording.no_scripts")} image={Empty.PRESENTED_IMAGE_SIMPLE} />
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
                        <Tooltip title={t("recording.play")} key="play">
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
                        <Tooltip title={t("automation.convert_to_workflow")} key="convert">
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
                          title={t("recording.delete_confirm", { name: script.name })}
                          onConfirm={() => handleDeleteScript(script.name)}
                          okText={t("common.ok")}
                          cancelText={t("common.cancel")}
                        >
                          <Tooltip title={t("recording.delete")}>
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
                              {script.events?.length || 0} {t("recording.events")} · {script.resolution}
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
          </div>
        </div>
      </div>

      {/* Save Script Modal */}
      <Modal
        title={t("recording.save_script")}
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
          placeholder={t("recording.script_name")}
          value={scriptName}
          onChange={(e) => setScriptName(e.target.value)}
          onPressEnter={handleSaveScript}
        />
      </Modal>

      {/* Rename Script Modal */}
      <Modal
        title={t("recording.rename_script")}
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
          placeholder={t("recording.new_script_name")}
          value={newScriptName}
          onChange={(e) => setNewScriptName(e.target.value)}
          onPressEnter={handleRenameScript}
        />
      </Modal>

      {/* Selector Choice Modal (Precise Recording Mode) */}
      <Modal
        title={
          <Space>
            <AimOutlined style={{ color: token.colorPrimary }} />
            {t("recording.choose_selector")}
          </Space>
        }
        open={isWaitingForSelector}
        onCancel={() => {
          // If closed, default to coordinates to let recording continue
          const coordSuggestion = pendingSelectorData?.suggestions?.find((s: any) => s.type === 'coordinates');
          if (coordSuggestion) {
            submitSelectorChoice(coordSuggestion.type, coordSuggestion.value);
          } else if (pendingSelectorData?.elementInfo) {
            // Fallback manually if suggestion list is missing for some reason
            const { x, y } = pendingSelectorData.elementInfo;
            submitSelectorChoice('coordinates', `${x},${y}`);
          }
        }}
        footer={
          <Button
            onClick={() => {
              const coordSuggestion = pendingSelectorData?.suggestions?.find((s: any) => s.type === 'coordinates');
              if (coordSuggestion) {
                submitSelectorChoice(coordSuggestion.type, coordSuggestion.value);
              }
            }}
          >
            {t("recording.use_coordinates_and_close")}
          </Button>
        }
        closable={true}
        maskClosable={false}
        width={600}
        centered
      >
        <Space direction="vertical" style={{ width: "100%" }} size="middle">
          <Alert
            message={t("recording.precise_mode_active")}
            description={t("recording.please_choose_selector")}
            type="info"
            showIcon
          />

          {pendingSelectorData?.elementInfo && (
            <Card size="small" title={t("recording.element_info")} className="element-info-card">
              <div style={{ display: 'grid', gridTemplateColumns: '80px 1fr', gap: '8px' }}>
                <span style={{ color: token.colorTextDescription }}>{t("recording.class")}:</span>
                <span style={{ fontFamily: 'monospace', fontSize: '12px' }}>{pendingSelectorData.elementInfo.class}</span>

                <span style={{ color: token.colorTextDescription }}>{t("recording.bounds")}:</span>
                <span style={{ fontFamily: 'monospace', fontSize: '12px' }}>{pendingSelectorData.elementInfo.bounds}</span>
              </div>
            </Card>
          )}

          <List
            header={<strong>{t("recording.suggestions")}</strong>}
            dataSource={pendingSelectorData?.suggestions || []}
            renderItem={(suggestion: any) => {
              const getPriorityColor = (p: number) => {
                if (p >= 5) return 'success';
                if (p >= 4) return 'processing';
                if (p >= 3) return 'warning';
                return 'default';
              };

              return (
                <List.Item
                  style={{
                    cursor: 'pointer',
                    borderRadius: 8,
                    marginBottom: 8,
                    border: `1px solid ${token.colorBorderSecondary}`,
                    transition: 'all 0.2s'
                  }}
                  className="selector-option-item"
                  onClick={() => submitSelectorChoice(suggestion.type, suggestion.value)}
                >
                  <List.Item.Meta
                    avatar={
                      <div style={{
                        width: 32,
                        height: 32,
                        borderRadius: '50%',
                        backgroundColor: token.colorPrimaryBg,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        color: token.colorPrimary
                      }}>
                        {suggestion.type === 'text' && <span>T</span>}
                        {suggestion.type === 'id' && <span>ID</span>}
                        {suggestion.type === 'desc' && <span>D</span>}
                        {suggestion.type === 'xpath' && <span>X</span>}
                        {suggestion.type === 'coordinates' && <span>XY</span>}
                        {suggestion.type === 'class' && <span>C</span>}
                      </div>
                    }
                    title={
                      <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                        <span style={{ fontWeight: 600 }}>{suggestion.type.toUpperCase()}</span>
                        <Tag color={getPriorityColor(suggestion.priority)}>
                          Priority: {suggestion.priority}
                        </Tag>
                      </div>
                    }
                    description={
                      <div>
                        <div style={{
                          fontFamily: 'monospace',
                          backgroundColor: token.colorFillAlter,
                          padding: '4px 8px',
                          borderRadius: 4,
                          marginBottom: 4,
                          wordBreak: 'break-all'
                        }}>
                          {suggestion.value}
                        </div>
                        <div style={{ fontSize: '12px', color: token.colorTextDescription }}>
                          {suggestion.description}
                        </div>
                      </div>
                    }
                  />
                </List.Item>
              );
            }}
          />
        </Space>
      </Modal>
    </div>
  );
};

export default RecordingView;
