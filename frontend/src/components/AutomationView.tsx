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
} from "antd";
import { useTranslation } from "react-i18next";
import {
  PlayCircleOutlined,
  StopOutlined,
  DeleteOutlined,
  SaveOutlined,
  CaretRightOutlined,
  RobotOutlined,
} from "@ant-design/icons";
import DeviceSelector from "./DeviceSelector";
import { useDeviceStore, useAutomationStore, TouchScript } from "../stores";
import { main } from "../../wailsjs/go/models";

const AutomationView: React.FC = () => {
  const { selectedDevice } = useDeviceStore();
  const {
    isRecording,
    isPlaying,
    recordingDeviceId,
    playingDeviceId,
    recordingDuration,
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
    setCurrentScript,
    updateRecordingDuration,
    subscribeToEvents,
  } = useAutomationStore();

  const { t } = useTranslation();
  const { token } = theme.useToken();

  const [saveModalVisible, setSaveModalVisible] = useState(false);
  const [scriptName, setScriptName] = useState("");
  const [selectedScript, setSelectedScript] = useState<TouchScript | null>(null);
  const durationIntervalRef = useRef<number | null>(null);

  // Subscribe to events on mount
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
          <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: 16 }}>
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
                        {formatDuration(recordingDuration)}
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
                      <Button
                        type="primary"
                        icon={<SaveOutlined />}
                        size="small"
                        onClick={() => setSaveModalVisible(true)}
                      >
                        {t("automation.save_script")}
                      </Button>
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
                      }}
                    >
                      {formatEventDescription(event, idx)}
                    </div>
                  ))}
                </div>
              </Card>
            )}
          </div>

          {/* Right Column - Script List */}
          <div style={{ flex: 1 }}>
            <Card
              title={
                <Space>
                  <SaveOutlined />
                  {t("automation.saved_scripts")}
                </Space>
              }
              size="small"
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
                      <List.Item.Meta
                        title={script.name}
                        description={
                          <span style={{ fontSize: 11, color: token.colorTextSecondary }}>
                            {script.events?.length || 0} {t("automation.events")} · {script.resolution}
                          </span>
                        }
                      />
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
    </div>
  );
};

export default AutomationView;
