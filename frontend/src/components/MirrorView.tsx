import React, { useState, useEffect } from "react";
import { Button, Space, Tag, Card, Switch, Tooltip, Slider, Select, message } from "antd";
import { useTranslation } from "react-i18next";
import {
  StopOutlined,
  DesktopOutlined,
  DownloadOutlined,
  PlayCircleOutlined,
  FileTextOutlined,
  SettingOutlined,
  MobileOutlined,
  FolderOpenOutlined,
} from "@ant-design/icons";
import DeviceSelector from "./DeviceSelector";
// @ts-ignore
import { main } from "../../wailsjs/go/models";
// @ts-ignore
import { 
  OpenPath, 
  StartScrcpy, 
  StopScrcpy, 
  StartRecording, 
  StopRecording, 
  SelectRecordPath 
} from "../../wailsjs/go/main/App";

const { Option } = Select;

interface Device {
  id: string;
  state: string;
  model: string;
  brand: string;
}

interface MirrorViewProps {
  devices: Device[];
  selectedDevice: string;
  setSelectedDevice: (id: string) => void;
  fetchDevices: () => Promise<void>;
  loading: boolean;
  isMirroring: boolean;
  mirrorDuration: number;
  isRecording: boolean;
  recordDuration: number;
}

const MirrorView: React.FC<MirrorViewProps> = ({
  devices,
  selectedDevice,
  setSelectedDevice,
  fetchDevices,
  loading,
  isMirroring,
  mirrorDuration,
  isRecording,
  recordDuration,
}) => {
  const { t } = useTranslation();

  // Scrcpy local state
  const [scrcpyConfig, setScrcpyConfig] = useState<main.ScrcpyConfig>({
    maxSize: 0,
    bitRate: 8,
    maxFps: 60,
    stayAwake: true,
    turnScreenOff: false,
    noAudio: false,
    alwaysOnTop: false,
    showTouches: false,
    fullscreen: false,
    readOnly: false,
    powerOffOnClose: false,
    windowBorderless: false,
    videoCodec: "h264",
    audioCodec: "opus",
    recordPath: "",
  });
  const [shouldRecord, setShouldRecord] = useState(false);

  const handleStartScrcpy = async (deviceId: string, overrideConfig?: main.ScrcpyConfig) => {
    try {
      let currentConfig = { ...(overrideConfig || scrcpyConfig) };
      await StartScrcpy(deviceId, currentConfig);

      if (shouldRecord && !isRecording) {
        const path = await SelectRecordPath();
        if (path) {
          const recordConfig = { ...currentConfig, recordPath: path };
          await StartRecording(deviceId, recordConfig);
        } else {
          setShouldRecord(false);
        }
      }

      if (!overrideConfig) {
        message.success(
          shouldRecord
            ? t("app.scrcpy_started_record")
            : t("app.scrcpy_started_mirror")
        );
      }
    } catch (err) {
      message.error(t("app.scrcpy_failed") + ": " + String(err));
    }
  };

  const handleStopScrcpy = async (deviceId: string) => {
    try {
      if (isRecording) {
        await StopRecording(deviceId);
      }
      await StopScrcpy(deviceId);
      message.success(t("app.scrcpy_stopped"));
    } catch (err) {
      message.error(t("app.scrcpy_stop_failed") + ": " + String(err));
    }
  };

  const handleStartMidSessionRecord = async () => {
    if (!selectedDevice) return;
    try {
      const path = await SelectRecordPath();
      if (!path) {
        setShouldRecord(false);
        return;
      }
      const config = { ...scrcpyConfig, recordPath: path };
      await StartRecording(selectedDevice, config);
      message.success(t("app.record_started"));
    } catch (err) {
      setShouldRecord(false);
      message.error(t("app.record_failed") + ": " + String(err));
    }
  };

  const handleStopMidSessionRecord = async () => {
    if (!selectedDevice) return;
    try {
      setShouldRecord(false);
      await StopRecording(selectedDevice);
    } catch (err) {
      message.error(t("app.record_stop_failed") + ": " + String(err));
    }
  };

  const updateScrcpyConfig = async (newConfig: main.ScrcpyConfig) => {
    setScrcpyConfig(newConfig);
    if (isMirroring && selectedDevice) {
      await handleStartScrcpy(selectedDevice, newConfig);
    }
  };

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
          <h2 style={{ margin: 0 }}>{t("mirror.title")}</h2>
          <Tag color="blue">{t("mirror.powered_by")}</Tag>
        </div>
        <Space>
          <DeviceSelector
            devices={devices}
            selectedDevice={selectedDevice}
            onDeviceChange={setSelectedDevice}
            onRefresh={fetchDevices}
            loading={loading}
          />
          {isMirroring ? (
            <Button
              type="primary"
              danger
              size="large"
              icon={<StopOutlined />}
              onClick={() => handleStopScrcpy(selectedDevice)}
              disabled={!selectedDevice}
              style={{ height: "40px", borderRadius: "8px" }}
            >
              {t("mirror.stop")}
            </Button>
          ) : (
            <Button
              type="primary"
              size="large"
              icon={<DesktopOutlined />}
              onClick={() => handleStartScrcpy(selectedDevice)}
              disabled={!selectedDevice}
              style={{ height: "40px", borderRadius: "8px" }}
            >
              {t("mirror.start")}
            </Button>
          )}
        </Space>
      </div>

      {isMirroring && (
        <div
          style={{
            marginBottom: 16,
            padding: "8px 16px",
            backgroundColor: "#e6f7ff",
            border: "1px solid #91d5ff",
            borderRadius: "8px",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <Tag color="processing">MIRRORING</Tag>
            <span style={{ fontWeight: "bold", fontFamily: "monospace" }}>
              {new Date(mirrorDuration * 1000).toISOString().substr(11, 8)}
            </span>
          </div>
          <div style={{ fontSize: "12px", color: "#1890ff" }}>
            {t("mirror.active_session")}
          </div>
        </div>
      )}

      <div style={{ flex: 1, overflowY: "auto", paddingRight: 8 }}>
        <div
          style={{
            display: "flex",
            gap: "16px",
            alignItems: "flex-start",
          }}
        >
          {/* Left Column */}
          <div
            style={{
              flex: 1,
              display: "flex",
              flexDirection: "column",
              gap: "16px",
            }}
          >
            {/* Recording */}
            <Card
              title={
                <Space>
                  <DownloadOutlined />
                  {t("mirror.recording")}
                </Space>
              }
              extra={
                <Tooltip title={t("app.show_in_folder")}>
                  <Button
                    type="text"
                    size="small"
                    icon={<FolderOpenOutlined />}
                    onClick={() => OpenPath("::recordings::")}
                  />
                </Tooltip>
              }
              size="small"
              style={{
                border:
                  shouldRecord || isRecording ? "1px solid #ff4d4f" : undefined,
                backgroundColor:
                  shouldRecord || isRecording ? "#fff1f0" : undefined,
              }}
            >
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                  marginBottom: isRecording ? 12 : 0,
                }}
              >
                <Space direction="vertical" size={0}>
                  <span
                    style={{
                      fontWeight:
                        shouldRecord || isRecording ? "bold" : "normal",
                    }}
                  >
                    {t("mirror.record_screen")}
                  </span>
                  <div style={{ fontSize: "11px", color: "#888" }}>
                    {isRecording
                      ? t("mirror.recording_in_progress")
                      : shouldRecord
                      ? t("mirror.save_dialog_desc")
                      : t("mirror.independent_desc")}
                  </div>
                </Space>
                {shouldRecord || isRecording ? (
                  <Button
                    type="primary"
                    danger
                    size="small"
                    icon={<StopOutlined />}
                    onClick={async () => {
                      setShouldRecord(false);
                      await handleStopMidSessionRecord();
                    }}
                  >
                    {t("mirror.record_stop")}
                  </Button>
                ) : (
                  <Button
                    type="primary"
                    size="small"
                    icon={<PlayCircleOutlined />}
                    onClick={async () => {
                      setShouldRecord(true);
                      await handleStartMidSessionRecord();
                    }}
                  >
                    {t("mirror.record_start")}
                  </Button>
                )}
              </div>

              {isRecording && (
                <div
                  style={{
                    padding: "8px",
                    backgroundColor: "rgba(255, 77, 79, 0.05)",
                    borderRadius: "4px",
                    border: "1px dashed #ffa39e",
                  }}
                >
                  <div
                    style={{
                      display: "flex",
                      justifyContent: "space-between",
                      alignItems: "center",
                      marginBottom: 4,
                    }}
                  >
                    <Tag
                      color="error"
                      icon={<div className="record-dot" />}
                      style={{ margin: 0 }}
                    >
                      {t("mirror.recording").toUpperCase()}
                    </Tag>
                    <span
                      style={{
                        fontWeight: "bold",
                        fontFamily: "monospace",
                        color: "#cf1322",
                      }}
                    >
                      {new Date(recordDuration * 1000).toISOString().substr(11, 8)}
                    </span>
                  </div>
                  {scrcpyConfig.recordPath && (
                    <div
                      style={{
                        fontSize: "10px",
                        color: "#888",
                        wordBreak: "break-all",
                        marginTop: 4,
                      }}
                    >
                      {t("files.title")}: {scrcpyConfig.recordPath.split(/[\\/]/).pop()}
                    </div>
                  )}
                </div>
              )}
            </Card>

            {/* Video Settings */}
            <Card
              title={
                <Space>
                  <PlayCircleOutlined />
                  {t("mirror.video_settings")}
                </Space>
              }
              size="small"
              className="mirror-card"
            >
              <div style={{ marginBottom: 16 }}>
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                  }}
                >
                  <span>{t("mirror.max_res")}</span>
                  <Tag>{scrcpyConfig.maxSize || "Auto"}</Tag>
                </div>
                <Slider
                  min={0}
                  max={2560}
                  step={128}
                  value={scrcpyConfig.maxSize}
                  onChange={(v) =>
                    setScrcpyConfig({ ...scrcpyConfig, maxSize: v })
                  }
                  marks={{
                    0: "Auto",
                    1024: "1024",
                    1920: "1080",
                    2560: "2K",
                  }}
                />
              </div>

              <div style={{ marginBottom: 16, marginTop: 24 }}>
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                  }}
                >
                  <span>{t("mirror.bitrate")}</span>
                  <Tag>{scrcpyConfig.bitRate}M</Tag>
                </div>
                <Slider
                  min={1}
                  max={64}
                  value={scrcpyConfig.bitRate}
                  onChange={(v) =>
                    setScrcpyConfig({ ...scrcpyConfig, bitRate: v })
                  }
                />
              </div>

              <div style={{ marginBottom: 16 }}>
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                  }}
                >
                  <span>{t("mirror.max_fps")}</span>
                  <Tag>{scrcpyConfig.maxFps}</Tag>
                </div>
                <Slider
                  min={15}
                  max={144}
                  value={scrcpyConfig.maxFps}
                  onChange={(v) =>
                    setScrcpyConfig({ ...scrcpyConfig, maxFps: v })
                  }
                />
              </div>

              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                }}
              >
                <span>{t("mirror.video_codec")}</span>
                <Select
                  size="small"
                  value={scrcpyConfig.videoCodec}
                  onChange={(v) =>
                    updateScrcpyConfig({ ...scrcpyConfig, videoCodec: v })
                  }
                  style={{ width: 100 }}
                >
                  <Option value="h264">H.264</Option>
                  <Option value="h265">H.265</Option>
                  <Option value="av1">AV1</Option>
                </Select>
              </div>
            </Card>

            {/* Audio Settings */}
            <Card
              title={
                <Space>
                  <FileTextOutlined />
                  {t("mirror.audio_settings")}
                </Space>
              }
              size="small"
            >
              <Space direction="vertical" style={{ width: "100%" }}>
                <div className="setting-item">
                  <span>{t("mirror.disable_audio")}</span>
                  <Switch
                    size="small"
                    checked={scrcpyConfig.noAudio}
                    onChange={(v) =>
                      updateScrcpyConfig({ ...scrcpyConfig, noAudio: v })
                    }
                  />
                </div>
                <div className="setting-item">
                  <span>{t("mirror.audio_codec")}</span>
                  <Select
                    size="small"
                    disabled={scrcpyConfig.noAudio}
                    value={scrcpyConfig.audioCodec}
                    onChange={(v) =>
                      updateScrcpyConfig({ ...scrcpyConfig, audioCodec: v })
                    }
                    style={{ width: 100 }}
                  >
                    <Option value="opus">Opus</Option>
                    <Option value="aac">AAC</Option>
                    <Option value="flac">FLAC</Option>
                    <Option value="raw">RAW</Option>
                  </Select>
                </div>
              </Space>
            </Card>
          </div>

          {/* Right Column */}
          <div
            style={{
              flex: 1,
              display: "flex",
              flexDirection: "column",
              gap: "16px",
            }}
          >
            {/* Window Options */}
            <Card
              title={
                <Space>
                  <SettingOutlined />
                  {t("mirror.window_options")}
                </Space>
              }
              size="small"
            >
              <Space direction="vertical" style={{ width: "100%" }}>
                <div className="setting-item">
                  <span>{t("mirror.always_on_top")}</span>
                  <Switch
                    size="small"
                    checked={scrcpyConfig.alwaysOnTop}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...scrcpyConfig,
                        alwaysOnTop: v,
                      })
                    }
                  />
                </div>
                <div className="setting-item">
                  <span>{t("mirror.fullscreen")}</span>
                  <Switch
                    size="small"
                    checked={scrcpyConfig.fullscreen}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...scrcpyConfig,
                        fullscreen: v,
                      })
                    }
                  />
                </div>
                <div className="setting-item">
                  <span>{t("mirror.borderless")}</span>
                  <Switch
                    size="small"
                    checked={scrcpyConfig.windowBorderless}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...scrcpyConfig,
                        windowBorderless: v,
                      })
                    }
                  />
                </div>
              </Space>
            </Card>

            {/* Control & Interaction */}
            <Card
              title={
                <Space>
                  <MobileOutlined />
                  {t("mirror.control_interaction")}
                </Space>
              }
              size="small"
            >
              <Space direction="vertical" style={{ width: "100%" }}>
                <div className="setting-item">
                  <Tooltip title={t("mirror.stay_awake_desc")}>
                    <span>{t("mirror.stay_awake")}</span>
                  </Tooltip>
                  <Switch
                    size="small"
                    checked={scrcpyConfig.stayAwake}
                    onChange={(v) =>
                      updateScrcpyConfig({ ...scrcpyConfig, stayAwake: v })
                    }
                  />
                </div>
                <div className="setting-item">
                  <Tooltip title={t("mirror.read_only_desc")}>
                    <span>{t("mirror.read_only")}</span>
                  </Tooltip>
                  <Switch
                    size="small"
                    checked={scrcpyConfig.readOnly}
                    onChange={(v) =>
                      updateScrcpyConfig({ ...scrcpyConfig, readOnly: v })
                    }
                  />
                </div>
                <div className="setting-item">
                  <span>{t("mirror.show_touches")}</span>
                  <Switch
                    size="small"
                    checked={scrcpyConfig.showTouches}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...scrcpyConfig,
                        showTouches: v,
                      })
                    }
                  />
                </div>
              </Space>
            </Card>

            {/* Power Management */}
            <Card
              title={
                <Space>
                  <StopOutlined />
                  {t("mirror.power_management")}
                </Space>
              }
              size="small"
            >
              <Space direction="vertical" style={{ width: "100%" }}>
                <div className="setting-item">
                  <Tooltip title={t("mirror.turn_screen_off_desc")}>
                    <span>{t("mirror.turn_screen_off")}</span>
                  </Tooltip>
                  <Switch
                    size="small"
                    checked={scrcpyConfig.turnScreenOff}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...scrcpyConfig,
                        turnScreenOff: v,
                      })
                    }
                  />
                </div>
                <div className="setting-item">
                  <Tooltip title={t("mirror.power_off_on_close_desc")}>
                    <span>{t("mirror.power_off_on_close")}</span>
                  </Tooltip>
                  <Switch
                    size="small"
                    checked={scrcpyConfig.powerOffOnClose}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...scrcpyConfig,
                        powerOffOnClose: v,
                      })
                    }
                  />
                </div>
              </Space>
            </Card>
          </div>
        </div>
      </div>
    </div>
  );
};

export default MirrorView;


