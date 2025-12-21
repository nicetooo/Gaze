import { useState, useEffect, useRef } from "react";
import {
  Layout,
  Menu,
  Button,
  message,
  notification,
  Dropdown,
} from "antd";
import { useTranslation } from "react-i18next";
import LogcatView from "./components/LogcatView";
import DevicesView from "./components/DevicesView";
import AppsView from "./components/AppsView";
import FilesView from "./components/FilesView";
import ShellView from "./components/ShellView";
import MirrorView from "./components/MirrorView";
import DeviceInfoModal from "./components/DeviceInfoModal";
import AboutModal from "./components/AboutModal";
import WirelessConnectModal from "./components/WirelessConnectModal";
import {
  MobileOutlined,
  AppstoreOutlined,
  CodeOutlined,
  FileTextOutlined,
  DesktopOutlined,
  FolderOutlined,
  GithubOutlined,
  BugOutlined,
  InfoCircleOutlined,
  TranslationOutlined,
} from "@ant-design/icons";
import "./App.css";
// @ts-ignore
const BrowserOpenURL = (window as any).runtime.BrowserOpenURL;
// @ts-ignore
import {
  GetDevices,
  StopLogcat,
  OpenPath,
  GetDeviceInfo,
  AdbPair,
  AdbConnect,
  AdbDisconnect,
  SwitchToWireless,
  GetHistoryDevices,
  RemoveHistoryDevice,
  OpenSettings,
  StartLogcat,
} from "../wailsjs/go/main/App";
// @ts-ignore
import { main } from "../wailsjs/go/models";

// @ts-ignore
const EventsOn = (window as any).runtime.EventsOn;
// @ts-ignore
const EventsOff = (window as any).runtime.EventsOff;

const { Content, Sider } = Layout;

interface Device {
  id: string;
  serial: string;
  state: string;
  model: string;
  brand: string;
  type: string;
  ids: string[];
  wifiAddr: string;
}

function App() {
  const { t, i18n } = useTranslation();
  const [selectedKey, setSelectedKey] = useState("1");
  const [devices, setDevices] = useState<Device[]>([]);
  const [historyDevices, setHistoryDevices] = useState<any[]>([]);
  const [busyDevices, setBusyDevices] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);

  // Device Info state
  const [deviceInfoVisible, setDeviceInfoVisible] = useState(false);
  const [deviceInfoLoading, setDeviceInfoLoading] = useState(false);
  const [selectedDeviceInfo, setSelectedDeviceInfo] =
    useState<main.DeviceInfo | null>(null);

  // About state
  const [aboutVisible, setAboutVisible] = useState(false);

  // Wireless Connect state
  const [wirelessConnectVisible, setWirelessConnectVisible] = useState(false);

  // Global state for views that need background synchronization
  const [selectedDevice, setSelectedDevice] = useState<string>("");
  const [logs, setLogs] = useState<string[]>([]);
  const [isLogging, setIsLogging] = useState(false);
  
  // Multi-device mirror status
  const [mirrorStatuses, setMirrorStatuses] = useState<Record<string, {
    isMirroring: boolean;
    startTime: number | null;
    duration: number;
  }>>({});

  // Multi-device record status
  const [recordStatuses, setRecordStatuses] = useState<Record<string, {
    isRecording: boolean;
    startTime: number | null;
    duration: number;
    recordPath: string;
  }>>({});

  const recordPathRefs = useRef<Record<string, string>>({});
  // Refs for event listeners
  const loadingRef = useRef(false);
  const isFetchingRef = useRef(false);

  useEffect(() => {
    loadingRef.current = loading;
  }, [loading]);

  const fetchDevices = async (silent: boolean = false) => {
    if (isFetchingRef.current) return;
    isFetchingRef.current = true;
    
    const isSilent = silent === true;
    if (!isSilent) {
      setLoading(true);
    }
    try {
      const res = await GetDevices();
      setDevices(res || []);
      
      try {
        const history = await GetHistoryDevices();
        setHistoryDevices(history || []);
      } catch (historyErr) {
        console.warn("Failed to load device history:", historyErr);
        setHistoryDevices([]);
      }

      if (res && res.length > 0) {
        if (!selectedDevice) {
          setSelectedDevice(res[0].id);
        }
      }
    } catch (err) {
      if (!isSilent) {
        message.error(t("app.fetch_devices_failed") + ": " + String(err));
      }
    } finally {
      if (!isSilent) {
        setLoading(false);
      }
      isFetchingRef.current = false;
    }
  };

  const handleFetchDeviceInfo = async (deviceId: string) => {
    setDeviceInfoVisible(true);
    setDeviceInfoLoading(true);
    try {
      const res = await GetDeviceInfo(deviceId);
      setSelectedDeviceInfo(res);
    } catch (err) {
      message.error(t("app.fetch_device_info_failed") + ": " + String(err));
    } finally {
      setDeviceInfoLoading(false);
    }
  };

  const handleAdbConnect = async (address: string) => {
    try {
      const res = await AdbConnect(address);
      if (res.includes("connected to")) {
        fetchDevices();
      } else {
        throw new Error(res);
      }
    } catch (err) {
      message.error(t("app.connect_failed") + ": " + String(err));
      throw err;
    }
  };

  const handleAdbPair = async (address: string, code: string) => {
    try {
      const res = await AdbPair(address, code);
      if (res.includes("Successfully paired")) {
        message.success(t("app.pairing_success"));
        fetchDevices();
      } else {
        throw new Error(res);
      }
    } catch (err) {
      message.error(t("app.pairing_failed") + ": " + String(err));
      throw err;
    }
  };

  const handleSwitchToWireless = async (deviceId: string) => {
    const hide = message.loading(t("app.switching_to_wireless"), 0);
    setBusyDevices(prev => new Set(prev).add(deviceId));
    try {
      const res = await SwitchToWireless(deviceId);
      if (res.includes("connected to")) {
        message.success(t("app.switch_success"));
        await fetchDevices(true);
      } else {
        throw new Error(res);
      }
    } catch (err) {
      message.error(t("app.switch_failed") + ": " + String(err));
    } finally {
      setBusyDevices(prev => {
        const next = new Set(prev);
        next.delete(deviceId);
        return next;
      });
      hide();
    }
  };

  const handleAdbDisconnect = async (deviceId: string) => {
    try {
      await AdbDisconnect(deviceId);
      message.success(t("app.disconnect_success"));
      fetchDevices();
    } catch (err) {
      message.error(t("app.disconnect_failed") + ": " + String(err));
    }
  };

  const handleRemoveHistoryDevice = async (deviceId: string) => {
    try {
      await RemoveHistoryDevice(deviceId);
      message.success(t("app.remove_success"));
      fetchDevices();
    } catch (err) {
      message.error(t("app.remove_failed") + ": " + String(err));
    }
  };

  const handleOpenSettings = async (deviceId: string, action: string = "", data: string = "") => {
    const hide = message.loading(t("app.opening_settings"), 0);
    try {
      await OpenSettings(deviceId, action, data);
      message.success(t("app.open_settings_success"));
    } catch (err) {
      message.error(t("app.open_settings_failed") + ": " + String(err));
    } finally {
      hide();
    }
  };

  const toggleLogcat = async (pkg: string) => {
    if (!selectedDevice) {
      message.error(t("app.no_device_selected"));
      return;
    }
    if (isLogging) {
      await StopLogcat();
      setIsLogging(false);
      EventsOff("logcat-line");
    } else {
      setLogs([]);
      setIsLogging(true);
      EventsOn("logcat-line", (line: string) => {
        setLogs((prev) => {
          const next = [...prev, line];
          return next.slice(-1000); // Keep last 1000 lines
        });
      });
      try {
        await StartLogcat(selectedDevice, pkg);
      } catch (err) {
        message.error(t("app.logcat_failed") + ": " + String(err));
        setIsLogging(false);
        EventsOff("logcat-line");
      }
    }
  };

  useEffect(() => {
    fetchDevices();

    EventsOn("scrcpy-started", (data: any) => {
      const deviceId = data.deviceId;
      setMirrorStatuses(prev => ({
        ...prev,
        [deviceId]: {
          isMirroring: true,
          startTime: data.startTime,
          duration: 0
        }
      }));
    });

    EventsOn("scrcpy-stopped", (deviceId: string) => {
      setMirrorStatuses(prev => {
        const next = { ...prev };
        if (next[deviceId]) {
          next[deviceId] = { ...next[deviceId], isMirroring: false, startTime: null };
        }
        return next;
      });
    });

    EventsOn("scrcpy-record-started", (data: any) => {
      const deviceId = data.deviceId;
      setRecordStatuses(prev => ({
        ...prev,
        [deviceId]: {
          isRecording: true,
          startTime: data.startTime,
          duration: 0,
          recordPath: data.recordPath
        }
      }));
      recordPathRefs.current[deviceId] = data.recordPath;
    });

    EventsOn("scrcpy-record-stopped", (deviceId: string) => {
      const path = recordPathRefs.current[deviceId];
      setRecordStatuses(prev => {
        const next = { ...prev };
        if (next[deviceId]) {
          next[deviceId] = { ...next[deviceId], isRecording: false, startTime: null };
        }
        return next;
      });

      notification.success({
        message: t("app.recording_saved"),
        description: t("app.recording_saved_desc"),
        btn: (
          <Button
            type="primary"
            size="small"
            onClick={() => {
              if (path) {
                OpenPath(path);
              }
              notification.destroy();
            }}
          >
            {t("app.show_in_folder")}
          </Button>
        ),
        key: "scrcpy-record-saved-" + deviceId,
        duration: 5,
      });
    });


    EventsOn("tray:navigate", (data: any) => {
      if (data.deviceId) {
        setSelectedDevice(data.deviceId);
      }
      if (data.view) {
        const viewMap: Record<string, string> = {
          "devices": "1",
          "apps": "2",
          "shell": "3",
          "logcat": "4",
          "mirror": "5",
          "files": "6"
        };
        if (viewMap[data.view]) {
          setSelectedKey(viewMap[data.view]);
        }
      }
    });

    return () => {
      EventsOff("scrcpy-started");
      EventsOff("scrcpy-stopped");
      EventsOff("scrcpy-record-started");
      EventsOff("scrcpy-record-stopped");
      EventsOff("tray:navigate");
      StopLogcat();
    };
  }, []);

  // Poll devices list sequentially after each fetch completes
  useEffect(() => {
    const timer = setInterval(() => {
      const now = Math.floor(Date.now() / 1000);
      
      setMirrorStatuses(prev => {
        let changed = false;
        const next = { ...prev };
        Object.keys(next).forEach(id => {
          if (next[id].isMirroring && next[id].startTime) {
            next[id].duration = now - next[id].startTime!;
            changed = true;
          }
        });
        return changed ? next : prev;
      });

      setRecordStatuses(prev => {
        let changed = false;
        const next = { ...prev };
        Object.keys(next).forEach(id => {
          if (next[id].isRecording && next[id].startTime) {
            next[id].duration = now - next[id].startTime!;
            changed = true;
          }
        });
        return changed ? next : prev;
      });
    }, 1000);
    return () => clearInterval(timer);
  }, []);


  // Poll devices list sequentially after each fetch completes
  useEffect(() => {
    if (selectedKey !== "1") {
      return;
    }

    let timeoutId: any;
    let isActive = true;

    const poll = async () => {
      if (!isActive) return;
      
      // Use silent=true to avoid showing loading state
      if (!loadingRef.current) {
        await fetchDevices(true);
      }
      
      if (isActive) {
        timeoutId = setTimeout(poll, 3000); // Wait 3s after the previous one finishes
      }
    };

    timeoutId = setTimeout(poll, 3000);

    return () => {
      isActive = false;
      clearTimeout(timeoutId);
    };
  }, [selectedKey]);

  const renderContent = () => {
    switch (selectedKey) {
      case "1":
        return (
          <DevicesView
            devices={devices}
            historyDevices={historyDevices}
            loading={loading}
            fetchDevices={fetchDevices}
            busyDevices={busyDevices}
            setSelectedKey={setSelectedKey}
            setSelectedDevice={setSelectedDevice}
            handleStartScrcpy={async (deviceId) => {
              setSelectedDevice(deviceId);
              setSelectedKey("5");
            }}
            handleFetchDeviceInfo={handleFetchDeviceInfo}
            onShowWirelessConnect={() => setWirelessConnectVisible(true)}
            handleSwitchToWireless={handleSwitchToWireless}
            handleAdbConnect={handleAdbConnect}
            handleAdbDisconnect={handleAdbDisconnect}
            handleRemoveHistoryDevice={handleRemoveHistoryDevice}
            handleOpenSettings={handleOpenSettings}
            mirrorStatuses={mirrorStatuses}
            recordStatuses={recordStatuses}
          />
        );
      case "6":
        return (
          <FilesView
            devices={devices}
            selectedDevice={selectedDevice}
            setSelectedDevice={setSelectedDevice}
            fetchDevices={fetchDevices}
            loading={loading}
          />
        );
      case "2":
        return (
          <AppsView
            devices={devices}
            selectedDevice={selectedDevice}
            setSelectedDevice={setSelectedDevice}
            fetchDevices={fetchDevices}
            loading={loading}
          />
        );
      case "3":
        return (
          <ShellView
            devices={devices}
            selectedDevice={selectedDevice}
            setSelectedDevice={setSelectedDevice}
            fetchDevices={fetchDevices}
            loading={loading}
          />
        );
      case "4":
        return (
          <LogcatView
            devices={devices}
            selectedDevice={selectedDevice}
            setSelectedDevice={setSelectedDevice}
            fetchDevices={fetchDevices}
            isLogging={isLogging}
            toggleLogcat={toggleLogcat}
            logs={logs}
            setLogs={setLogs}
          />
        );
      case "5":
        return (
          <MirrorView
            devices={devices}
            selectedDevice={selectedDevice}
            setSelectedDevice={setSelectedDevice}
            fetchDevices={fetchDevices}
            loading={loading}
            mirrorStatuses={mirrorStatuses}
            recordStatuses={recordStatuses}
          />
        );

      default:
        return <div style={{ padding: 24 }}>{t("app.select_option")}</div>;
    }
  };

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <Sider
        width={200}
        theme="dark"
        style={{
          height: "100vh",
          position: "fixed",
          left: 0,
          top: 0,
          bottom: 0,
          display: "flex",
          flexDirection: "column",
        }}
      >
        <div
          style={{ display: "flex", flexDirection: "column", height: "100%" }}
        >
          <div className="drag-handle" style={{ height: 38, width: "100%", flexShrink: 0 }} />
          <div style={{ flex: 1, overflowY: "auto" }}>
            <Menu
              theme="dark"
              selectedKeys={[selectedKey]}
              mode="inline"
              onClick={({ key }) => setSelectedKey(key)}
              items={[
                { key: "1", icon: <MobileOutlined />, label: t("menu.devices") },
                { key: "5", icon: <DesktopOutlined />, label: t("menu.mirror") },
                { key: "2", icon: <AppstoreOutlined />, label: t("menu.apps") },
                { key: "6", icon: <FolderOutlined />, label: t("menu.files") },
                { key: "3", icon: <CodeOutlined />, label: t("menu.shell") },
                { key: "4", icon: <FileTextOutlined />, label: t("menu.logcat") },
              ]}
            />
          </div>
          <div
            style={{
              padding: "8px 16px",
              borderTop: "1px solid rgba(255, 255, 255, 0.1)",
              display: "flex",
              justifyContent: "center",
              gap: "8px",
              flexWrap: "wrap",
            }}
          >
            <Dropdown
              menu={{
                items: [
                  { key: "en", label: "English", onClick: () => i18n.changeLanguage("en") },
                  { key: "zh", label: "简体中文", onClick: () => i18n.changeLanguage("zh") },
                  { key: "zh-TW", label: "繁體中文 (台灣)", onClick: () => i18n.changeLanguage("zh-TW") },
                  { key: "ja", label: "日本語", onClick: () => i18n.changeLanguage("ja") },
                  { key: "ko", label: "한국어", onClick: () => i18n.changeLanguage("ko") },
                ],
                selectedKeys: [i18n.language],
              }}
              placement="topCenter"
            >
              <Button
                type="text"
                size="small"
                icon={<TranslationOutlined style={{ fontSize: "16px", color: "rgba(255,255,255,0.45)" }} />}
                title={t("app.change_language")}
              />
            </Dropdown>
            <Button
              type="text"
              size="small"
              icon={<InfoCircleOutlined style={{ fontSize: "16px", color: "rgba(255,255,255,0.45)" }} />}
              onClick={() => setAboutVisible(true)}
              title={t("app.about")}
            />
            <Button
              type="text"
              size="small"
              icon={<GithubOutlined style={{ fontSize: "16px", color: "rgba(255,255,255,0.45)" }} />}
              onClick={() => BrowserOpenURL && BrowserOpenURL("https://github.com/nicetooo/adbGUI")}
              title={t("app.github")}
            />
            <Button
              type="text"
              size="small"
              icon={<BugOutlined style={{ fontSize: "16px", color: "rgba(255,255,255,0.45)" }} />}
              onClick={() => {
                if (!BrowserOpenURL) return;
                const currentDevice = devices.find(d => d.id === selectedDevice);
                const deviceInfo = currentDevice ? `${currentDevice.brand} ${currentDevice.model} (ID: ${currentDevice.id})` : "None";
                const body = encodeURIComponent(`### Description\n(Please describe the issue here)\n\n### Environment\n- App Version: 1.0.0\n- Device: ${deviceInfo}\n- OS: ${navigator.platform}`);
                BrowserOpenURL(`https://github.com/nicetooo/adbGUI/issues/new?body=${body}`);
              }}
              title={t("app.feedback")}
            />
          </div>
        </div>
      </Sider>
      <Layout className="site-layout" style={{ marginLeft: 200 }}>
        <Content style={{ margin: "0", height: "100vh", overflow: "hidden", display: "flex", flexDirection: "column" }}>
          <div className="drag-handle" style={{ height: 38, width: "100%", flexShrink: 0 }} />
          {renderContent()}
        </Content>
      </Layout>

      <DeviceInfoModal
        visible={deviceInfoVisible}
        onCancel={() => setDeviceInfoVisible(false)}
        deviceInfo={selectedDeviceInfo}
        loading={deviceInfoLoading}
        onRefresh={() => selectedDeviceInfo && handleFetchDeviceInfo(selectedDeviceInfo.serial || selectedDevice)}
      />

      <AboutModal visible={aboutVisible} onCancel={() => setAboutVisible(false)} />

      <WirelessConnectModal
        visible={wirelessConnectVisible}
        onCancel={() => setWirelessConnectVisible(false)}
        onConnect={handleAdbConnect}
        onPair={handleAdbPair}
      />
    </Layout>
  );
}

export default App;
