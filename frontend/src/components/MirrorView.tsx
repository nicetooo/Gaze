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
  ScissorOutlined,
  ReloadOutlined,
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
  SelectRecordPath,
  SelectScreenshotPath,
  TakeScreenshot,
  ListCameras,
  ListDisplays,
  GetDeviceInfo,
} from "../../wailsjs/go/main/App";

const { Option } = Select;
const EventsOn = (window as any).runtime.EventsOn;
const EventsOff = (window as any).runtime.EventsOff;

interface Device {
  id: string;
  state: string;
  model: string;
  brand: string;
}

interface MirrorStatus {
  isMirroring: boolean;
  startTime: number | null;
  duration: number;
}

interface RecordStatus {
  isRecording: boolean;
  startTime: number | null;
  duration: number;
  recordPath: string;
}

interface MirrorViewProps {
  devices: Device[];
  selectedDevice: string;
  setSelectedDevice: (id: string) => void;
  fetchDevices: () => Promise<void>;
  loading: boolean;
  mirrorStatuses: Record<string, MirrorStatus>;
  recordStatuses: Record<string, RecordStatus>;
}

const MirrorView: React.FC<MirrorViewProps> = ({
  devices,
  selectedDevice,
  setSelectedDevice,
  fetchDevices,
  loading,
  mirrorStatuses,
  recordStatuses,
}) => {
  const { t } = useTranslation();

  const defaultConfig: main.ScrcpyConfig = {
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
    displayId: 0,
    videoSource: "display",
    cameraId: "",
    cameraSize: "",
    displayOrientation: "0",
    captureOrientation: "0",
    keyboardMode: "sdk",
    mouseMode: "sdk",
    noClipboardSync: false,
    showFps: false,
    noPowerOn: false,
  };

  // Scrcpy states per device
  const [deviceConfigs, setDeviceConfigs] = useState<Record<string, main.ScrcpyConfig>>({});
  const [deviceShouldRecord, setDeviceShouldRecord] = useState<Record<string, boolean>>({});
  const [availableCameras, setAvailableCameras] = useState<string[]>([]);
  const [availableDisplays, setAvailableDisplays] = useState<string[]>([]);
  const [deviceAndroidVer, setDeviceAndroidVer] = useState<number>(0);

  // Helper to get current device's config
  const currentConfig = deviceConfigs[selectedDevice] || defaultConfig;
  const currentShouldRecord = !!deviceShouldRecord[selectedDevice];
  const currentMirrorStatus = mirrorStatuses[selectedDevice] || { isMirroring: false, duration: 0 };
  const currentRecordStatus = recordStatuses[selectedDevice] || { isRecording: false, duration: 0, recordPath: "" };

  useEffect(() => {
    if (selectedDevice) {
      fetchDeviceCapabilities();
    }
  }, [selectedDevice]);

  useEffect(() => {
    const handleScrcpyFailed = (data: any) => {
      if (data.deviceId === selectedDevice || !selectedDevice) {
        message.error({
          content: `${t("app.scrcpy_failed")}: ${data.error}`,
          duration: 10,
          style: { marginTop: '20vh' }
        });
      }
    };

    EventsOn("scrcpy-failed", handleScrcpyFailed);
    return () => {
      EventsOff("scrcpy-failed", handleScrcpyFailed);
    };
  }, [selectedDevice, t]);

  const fetchDeviceCapabilities = async () => {
    if (!selectedDevice) return;
    try {
      // Fetch in parallel for better performance and isolation
      const [info, cameras, displays] = await Promise.all([
        GetDeviceInfo(selectedDevice).catch(err => {
          console.error("GetDeviceInfo error:", err);
          return null;
        }),
        ListCameras(selectedDevice).catch(err => {
          console.error("ListCameras error:", err);
          return [];
        }),
        ListDisplays(selectedDevice).catch(err => {
          console.error("ListDisplays error:", err);
          return [];
        })
      ]);

      if (info && info.androidVer) {
        setDeviceAndroidVer(parseInt(info.androidVer));
      }
      setAvailableCameras(cameras || []);
      setAvailableDisplays(displays || []);
    } catch (err) {
      console.error("Failed to fetch device capabilities:", err);
    }
  };

  const setScrcpyConfig = (config: main.ScrcpyConfig) => {
    if (!selectedDevice) return;
    setDeviceConfigs(prev => ({
      ...prev,
      [selectedDevice]: config
    }));
  };

  const setShouldRecord = (val: boolean) => {
    if (!selectedDevice) return;
    setDeviceShouldRecord(prev => ({
      ...prev,
      [selectedDevice]: val
    }));
  };

  const handleStartScrcpy = async (deviceId: string, overrideConfig?: main.ScrcpyConfig) => {
    try {
      let config = { ...(overrideConfig || currentConfig) };
      await StartScrcpy(deviceId, config);

      if (currentShouldRecord && !currentRecordStatus.isRecording) {
        const device = devices.find(d => d.id === deviceId);
        const path = await SelectRecordPath(device?.model || "");
        if (path) {
          const recordConfig = { ...config, recordPath: path };
          await StartRecording(deviceId, recordConfig);
        } else {
          setShouldRecord(false);
        }
      }

      if (!overrideConfig) {
        message.success(
          currentShouldRecord
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
      await StopScrcpy(deviceId);
      message.success(t("app.scrcpy_stopped"));
    } catch (err) {
      message.error(t("app.scrcpy_stop_failed") + ": " + String(err));
    }
  };

  const handleTakeScreenshot = async () => {
    if (!selectedDevice) return;
    try {
      const device = devices.find(d => d.id === selectedDevice);
      const defaultPath = await SelectScreenshotPath(device?.model || "");
      if (!defaultPath) return;

      // The global listener will handle the toasts based on events emitted by TakeScreenshot
      await TakeScreenshot(selectedDevice, defaultPath);
      // Immediately refresh to show re-ordered device list
      await fetchDevices();
    } catch (err) {
      // Functional errors are handled by the global listener via events
    }
  };

  const handleStartMidSessionRecord = async () => {
    if (!selectedDevice) return;
    try {
      const device = devices.find(d => d.id === selectedDevice);
      const path = await SelectRecordPath(device?.model || "");
      if (!path) {
        setShouldRecord(false);
        return;
      }
      const config = { ...currentConfig, recordPath: path };
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
    if (currentMirrorStatus.isMirroring && selectedDevice) {
      await handleStartScrcpy(selectedDevice, newConfig);
    }
  };

  // Status for display
  const isMirroring = currentMirrorStatus.isMirroring;
  const isRecording = currentRecordStatus.isRecording;
  const mirrorDuration = currentMirrorStatus.duration;
  const recordDuration = currentRecordStatus.duration;

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
          <Button
            size="large"
            icon={<ScissorOutlined />}
            onClick={handleTakeScreenshot}
            disabled={!selectedDevice}
            style={{ height: "40px", borderRadius: "8px" }}
            title={t("mirror.screenshot")}
          >
            {t("mirror.screenshot")}
          </Button>
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
                  currentShouldRecord || isRecording ? "1px solid #ff4d4f" : undefined,
                backgroundColor:
                  currentShouldRecord || isRecording ? "#fff1f0" : undefined,
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
                        currentShouldRecord || isRecording ? "bold" : "normal",
                    }}
                  >
                    {t("mirror.record_screen")}
                  </span>
                  <div style={{ fontSize: "11px", color: "#888" }}>
                    {isRecording
                      ? t("mirror.recording_in_progress")
                      : currentShouldRecord
                      ? t("mirror.save_dialog_desc")
                      : t("mirror.independent_desc")}
                  </div>
                </Space>
                {currentShouldRecord || isRecording ? (
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
                  {currentRecordStatus.recordPath && (
                    <div
                      style={{
                        fontSize: "10px",
                        color: "#888",
                        wordBreak: "break-all",
                        marginTop: 4,
                      }}
                    >
                      {t("files.title")}: {currentRecordStatus.recordPath.split(/[\\/]/).pop()}
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
                  <Tag>{currentConfig.maxSize || "Auto"}</Tag>
                </div>
                <Slider
                  min={0}
                  max={2560}
                  step={128}
                  value={currentConfig.maxSize}
                  onChange={(v) =>
                    setScrcpyConfig({ ...currentConfig, maxSize: v })
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
                  <Tag>{currentConfig.bitRate}M</Tag>
                </div>
                <Slider
                  min={1}
                  max={64}
                  value={currentConfig.bitRate}
                  onChange={(v) =>
                    setScrcpyConfig({ ...currentConfig, bitRate: v })
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
                  <Tag>{currentConfig.maxFps}</Tag>
                </div>
                <Slider
                  min={15}
                  max={144}
                  value={currentConfig.maxFps}
                  onChange={(v) =>
                    setScrcpyConfig({ ...currentConfig, maxFps: v })
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
                  value={currentConfig.videoCodec}
                  onChange={(v) =>
                    updateScrcpyConfig({ ...currentConfig, videoCodec: v })
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
                    checked={currentConfig.noAudio}
                    onChange={(v) =>
                      updateScrcpyConfig({ ...currentConfig, noAudio: v })
                    }
                  />
                </div>
                <div className="setting-item">
                  <span>{t("mirror.audio_codec")}</span>
                  <Select
                    size="small"
                    disabled={currentConfig.noAudio}
                    value={currentConfig.audioCodec}
                    onChange={(v) =>
                      updateScrcpyConfig({ ...currentConfig, audioCodec: v })
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
                    checked={currentConfig.alwaysOnTop}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...currentConfig,
                        alwaysOnTop: v,
                      })
                    }
                  />
                </div>
                <div className="setting-item">
                  <span>{t("mirror.fullscreen")}</span>
                  <Switch
                    size="small"
                    checked={currentConfig.fullscreen}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...currentConfig,
                        fullscreen: v,
                      })
                    }
                  />
                </div>
                <div className="setting-item">
                  <span>{t("mirror.borderless")}</span>
                  <Switch
                    size="small"
                    checked={currentConfig.windowBorderless}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...currentConfig,
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
                    checked={currentConfig.stayAwake}
                    onChange={(v) =>
                      updateScrcpyConfig({ ...currentConfig, stayAwake: v })
                    }
                  />
                </div>
                <div className="setting-item">
                  <Tooltip title={t("mirror.read_only_desc")}>
                    <span>{t("mirror.read_only")}</span>
                  </Tooltip>
                  <Switch
                    size="small"
                    checked={currentConfig.readOnly}
                    onChange={(v) =>
                      updateScrcpyConfig({ ...currentConfig, readOnly: v })
                    }
                  />
                </div>
                <div className="setting-item">
                  <span>{t("mirror.show_touches")}</span>
                  <Switch
                    size="small"
                    checked={currentConfig.showTouches}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...currentConfig,
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
                    checked={currentConfig.turnScreenOff}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...currentConfig,
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
                    checked={currentConfig.powerOffOnClose}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...currentConfig,
                        powerOffOnClose: v,
                      })
                    }
                  />
                </div>
                <div className="setting-item" style={{ marginTop: 4 }}>
                   <span>{t("mirror.no_power_on")}</span>
                   <Switch
                    size="small"
                    checked={currentConfig.noPowerOn}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...currentConfig,
                        noPowerOn: v,
                      })
                    }
                  />
                </div>
              </Space>
            </Card>

            {/* Advanced & experimental */}
            <Card
              title={
                <Space>
                  <SettingOutlined />
                  {t("mirror.advanced_settings")}
                </Space>
              }
              extra={
                <Tooltip title={t("common.refresh")}>
                  <Button 
                    type="text" 
                    size="small" 
                    icon={<ReloadOutlined />} 
                    onClick={fetchDeviceCapabilities}
                  />
                </Tooltip>
              }
              size="small"
            >
              <Space direction="vertical" style={{ width: "100%" }}>
                 <div className="setting-item">
                  <span>{t("mirror.video_source")}</span>
                  <Select
                    size="small"
                    value={currentConfig.videoSource}
                    onChange={(v) =>
                      updateScrcpyConfig({ ...currentConfig, videoSource: v })
                    }
                    style={{ width: 100 }}
                  >
                    <Option value="display">{t("mirror.display")}</Option>
                    <Option value="camera">{t("mirror.camera")}</Option>
                  </Select>
                </div>

                {currentConfig.videoSource === "camera" && (
                   <>
                    {deviceAndroidVer < 12 && (
                       <div style={{ color: "#ff4d4f", fontSize: "11px", marginBottom: 8 }}>
                         ⚠️ {t("mirror.camera_version_warning")}
                       </div>
                    )}
                    <div className="setting-item">
                      <span>{t("mirror.camera_id")}</span>
                      <Select
                        size="small"
                        value={currentConfig.cameraId}
                        onChange={(v) =>
                          updateScrcpyConfig({ ...currentConfig, cameraId: v })
                        }
                        style={{ width: 120 }}
                      >
                         <Option value="">Default</Option>
                         {availableCameras.map(cam => {
                            const idMatch = cam.match(/--camera-id=([^ ]+)/);
                            const id = idMatch ? idMatch[1] : cam;
                            return <Option key={id} value={id}>{cam.replace("--camera-id=", "")}</Option>;
                         })}
                      </Select>
                    </div>
                    <div className="setting-item">
                      <span>{t("mirror.camera_size")}</span>
                      <Select
                        size="small"
                        value={currentConfig.cameraSize}
                        onChange={(v) =>
                          updateScrcpyConfig({ ...currentConfig, cameraSize: v })
                        }
                        style={{ width: 120 }}
                      >
                         <Option value="">Default</Option>
                         <Option value="1920x1080">1920x1080</Option>
                         <Option value="1280x720">1280x720</Option>
                         <Option value="1024x768">1024x768</Option>
                         <Option value="640x480">640x480</Option>
                      </Select>
                    </div>
                   </>
                )}

                {currentConfig.videoSource === "display" && (
                  <div className="setting-item">
                    <span>{t("mirror.display_id")}</span>
                    <Select
                      size="small"
                      value={currentConfig.displayId}
                      onChange={(v) =>
                        updateScrcpyConfig({ ...currentConfig, displayId: v })
                      }
                      style={{ width: 120 }}
                    >
                      <Option value={0}>0 (Main)</Option>
                      {availableDisplays.map(disp => {
                         const idMatch = disp.match(/--display-id=([^ ]+)/);
                         const id = idMatch ? parseInt(idMatch[1]) : 0;
                         if (id === 0) return null;
                         return <Option key={id} value={id}>{disp.replace("--display-id=", "")}</Option>;
                      }).filter(Boolean)}
                    </Select>
                  </div>
                )}

                <div className="setting-item">
                  <span>{t("mirror.display_orientation")}</span>
                  <Select
                    size="small"
                    value={currentConfig.displayOrientation}
                    onChange={(v) =>
                      updateScrcpyConfig({ ...currentConfig, displayOrientation: v })
                    }
                    style={{ width: 100 }}
                  >
                    <Option value="0">0°</Option>
                    <Option value="90">90°</Option>
                    <Option value="180">180°</Option>
                    <Option value="270">270°</Option>
                    <Option value="flip0">Flip 0°</Option>
                  </Select>
                </div>

                <div className="setting-item">
                  <span>{t("mirror.keyboard_mode")}</span>
                  <Select
                    size="small"
                    value={currentConfig.keyboardMode}
                    onChange={(v) =>
                      updateScrcpyConfig({ ...currentConfig, keyboardMode: v })
                    }
                    style={{ width: 100 }}
                  >
                    <Option value="sdk">SDK</Option>
                    <Option value="uhid">UHID</Option>
                  </Select>
                </div>

                <div className="setting-item">
                  <span>{t("mirror.mouse_mode")}</span>
                  <Select
                    size="small"
                    value={currentConfig.mouseMode}
                    onChange={(v) =>
                      updateScrcpyConfig({ ...currentConfig, mouseMode: v })
                    }
                    style={{ width: 100 }}
                  >
                    <Option value="sdk">SDK</Option>
                    <Option value="uhid">UHID</Option>
                  </Select>
                </div>

                <div className="setting-item">
                   <span>{t("mirror.no_clipboard_sync")}</span>
                   <Switch
                    size="small"
                    checked={currentConfig.noClipboardSync}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...currentConfig,
                        noClipboardSync: v,
                      })
                    }
                  />
                </div>

                <div className="setting-item">
                   <span>{t("mirror.show_fps")}</span>
                   <Switch
                    size="small"
                    checked={currentConfig.showFps}
                    onChange={(v) =>
                      updateScrcpyConfig({
                        ...currentConfig,
                        showFps: v,
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



