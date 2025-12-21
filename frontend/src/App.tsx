import { useState, useEffect, useRef } from "react";
import {
  Layout,
  Menu,
  Button,
  message,
  notification,
  Modal,
  Dropdown,
} from "antd";
import { useTranslation } from "react-i18next";
import LogcatView from "./components/LogcatView";
import DeviceSelector from "./components/DeviceSelector";
import DevicesView from "./components/DevicesView";
import AppsView from "./components/AppsView";
import FilesView, { FocusInput } from "./components/FilesView";
import ShellView from "./components/ShellView";
import MirrorView from "./components/MirrorView";
import AppInfoModal from "./components/AppInfoModal";
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
  WifiOutlined,
} from "@ant-design/icons";
import "./App.css";
// @ts-ignore
const BrowserOpenURL = (window as any).runtime.BrowserOpenURL;
// @ts-ignore
import {
  GetDevices,
  RunAdbCommand,
  ListPackages,
  GetAppInfo,
  UninstallApp,
  ClearAppData,
  ForceStopApp,
  StartApp,
  EnableApp,
  DisableApp,
  StartActivity,
  StartLogcat,
  StopLogcat,
  StartScrcpy,
  StopScrcpy,
  StartRecording,
  StopRecording,
  OpenPath,
  SelectRecordPath,
  InstallAPK,
  ExportAPK,
  ListFiles,
  DeleteFile,
  MoveFile,
  CopyFile,
  Mkdir,
  OpenFileOnHost,
  CancelOpenFile,
  DownloadFile,
  GetThumbnail,
  GetDeviceInfo,
  AdbPair,
  AdbConnect,
  AdbDisconnect,
  SwitchToWireless,
  GetHistoryDevices,
  RemoveHistoryDevice,
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

  // Shell state
  const [shellOutput, setShellOutput] = useState("");
  const [shellCmd, setShellCmd] = useState("");

  // Apps state
  const [selectedDevice, setSelectedDevice] = useState<string>("");
  const [packages, setPackages] = useState<main.AppPackage[]>([]);
  const [appsLoading, setAppsLoading] = useState(false);
  const [packageFilter, setPackageFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState("user"); // all, system, user - default to user
  const [selectedAppInfo, setSelectedAppInfo] =
    useState<main.AppPackage | null>(null);
  const [infoModalVisible, setInfoModalVisible] = useState(false);
  const [infoLoading, setInfoLoading] = useState(false);
  const [permissionSearch, setPermissionSearch] = useState("");
  const [activitySearch, setActivitySearch] = useState("");

  // Files state
  const [currentPath, setCurrentPath] = useState("/");
  const [fileList, setFileList] = useState<any[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [clipboard, setClipboard] = useState<{
    path: string;
    type: "copy" | "cut";
  } | null>(null);
  const [showHiddenFiles, setShowHiddenFiles] = useState(false);

  // Logcat state
  const [logs, setLogs] = useState<string[]>([]);
  const [isLogging, setIsLogging] = useState(false);
  const [logFilter, setLogFilter] = useState("");
  const [selectedPackage, setSelectedPackage] = useState<string>("");
  const [autoScroll, setAutoScroll] = useState(true);

  // Scrcpy state
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

  // Mirror tracking state
  const [isMirroring, setIsMirroring] = useState(false);
  const [isRecording, setIsRecording] = useState(false);
  const [mirrorStartTime, setMirrorStartTime] = useState<number | null>(null);
  const [recordStartTime, setRecordStartTime] = useState<number | null>(null);
  const [mirrorDuration, setMirrorDuration] = useState(0);
  const [recordDuration, setRecordDuration] = useState(0);
  const [shouldRecord, setShouldRecord] = useState(false);

  // Refs for event listeners to avoid stale closures
  const isRecordingRef = useRef(false);
  const recordPathRef = useRef("");
  const loadingRef = useRef(false);

  useEffect(() => {
    isRecordingRef.current = isRecording;
  }, [isRecording]);

  useEffect(() => {
    recordPathRef.current = scrcpyConfig.recordPath;
  }, [scrcpyConfig.recordPath]);

  useEffect(() => {
    loadingRef.current = loading;
  }, [loading]);

  const fetchDevices = async (silent: boolean = false) => {
    if (!silent) {
      setLoading(true);
    }
    try {
      const res = await GetDevices();
      setDevices(res || []);
      
      // Try to load history, but don't fail if it errors
      try {
        const history = await GetHistoryDevices();
        setHistoryDevices(history || []);
      } catch (historyErr) {
        console.warn("Failed to load device history:", historyErr);
        setHistoryDevices([]);
      }

      if (res && res.length > 0) {
        const deviceId = res[0].id;
        if (!selectedDevice) {
          setSelectedDevice(deviceId);
        }
        // Automatically fetch user packages when device is connected (only for non-silent refresh)
        if (deviceId && !silent) {
          // Use setTimeout to avoid blocking the UI update
          setTimeout(() => {
            fetchPackages("user", deviceId);
          }, 100);
        }
      }
    } catch (err) {
      // Only show error message for non-silent refreshes
      if (!silent) {
        message.error(t("app.fetch_devices_failed") + ": " + String(err));
      }
    } finally {
      if (!silent) {
        setLoading(false);
      }
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
        // Pairing success often means we can now connect
        // But the user might need to enter the connect port (which is different from pair port)
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
    try {
      const res = await SwitchToWireless(deviceId);
      if (res.includes("connected to")) {
        message.success(t("app.switch_success"));
        fetchDevices();
      } else {
        throw new Error(res);
      }
    } catch (err) {
      message.error(t("app.switch_failed") + ": " + String(err));
    } finally {
      hide();
    }
  };

  const selectedDeviceRef = useRef(selectedDevice);
  const lastDropTime = useRef(0);

  useEffect(() => {
    selectedDeviceRef.current = selectedDevice;
  }, [selectedDevice]);

  useEffect(() => {
    fetchDevices();

    const handleFileDrop = (...args: any[]) => {
      // Wails v2 can fire with (x, y, paths) OR just (paths) depending on platform/version
      let actualPaths: string[] = [];
      if (args.length === 1 && Array.isArray(args[0])) {
        actualPaths = args[0];
      } else if (args.length >= 3 && Array.isArray(args[2])) {
        actualPaths = args[2];
      }

      if (actualPaths && actualPaths.length > 0) {
        const now = Date.now();
        if (now - lastDropTime.current < 500) return;
        lastDropTime.current = now;

        const apkFiles = actualPaths.filter(
          (p) => typeof p === "string" && p.toLowerCase().endsWith(".apk")
        );
        if (apkFiles.length > 0) {
          const currentDevice = selectedDeviceRef.current;
          if (!currentDevice) {
            message.error(t("app.select_device"));
            return;
          }
          handleInstallAPKs(currentDevice, apkFiles);
        }
      }
    };

    // Use only ONE listener to start with, or keep both with the debounce
    EventsOn("wails:file-drop", handleFileDrop);

    EventsOn("scrcpy-started", (data: any) => {
      setIsMirroring(true);
      // Only set start time if it's not already set (prevents reset during config updates)
      setMirrorStartTime((prev) => prev || data.startTime);
      if (data.recording) {
        setIsRecording(true);
        setShouldRecord(true);
        setRecordStartTime((prev) => prev || data.startTime);
        setScrcpyConfig((prev) => ({ ...prev, recordPath: data.recordPath }));
      }
    });

    EventsOn("scrcpy-stopped", (deviceId: string) => {
      setIsMirroring(false);
      setMirrorStartTime(null);
      // NOTE: We no longer clean up recording here.
      // Recording has its own dedicated "scrcpy-record-stopped" event.
    });

    EventsOn("scrcpy-record-started", (data: any) => {
      setIsRecording(true);
      setShouldRecord(true);
      setRecordStartTime(data.startTime);
      setRecordDuration(0);
      setScrcpyConfig((prev) => ({ ...prev, recordPath: data.recordPath }));
    });

    EventsOn("scrcpy-record-stopped", (deviceId: string) => {
      const path = recordPathRef.current;
      setIsRecording(false);
      setShouldRecord(false);
      setRecordStartTime(null);

      // Show notification with action to open folder
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
        key: "scrcpy-record-saved",
        duration: 5,
      });
    });

    return () => {
      EventsOff("wails:file-drop");
      EventsOff("scrcpy-started");
      EventsOff("scrcpy-stopped");
      EventsOff("scrcpy-record-started");
      EventsOff("scrcpy-record-stopped");
      StopLogcat();
    };
  }, []);

  useEffect(() => {
    let timer: any;
    if (isMirroring || isRecording) {
      timer = setInterval(() => {
        if (isMirroring && mirrorStartTime) {
          setMirrorDuration(Math.floor(Date.now() / 1000 - mirrorStartTime));
        }
        if (isRecording && recordStartTime) {
          setRecordDuration(Math.floor(Date.now() / 1000 - recordStartTime));
        }
      }, 1000);
    }

    if (!isMirroring) setMirrorDuration(0);
    if (!isRecording) setRecordDuration(0);

    return () => clearInterval(timer);
  }, [isMirroring, isRecording, mirrorStartTime, recordStartTime]);

  useEffect(() => {
    // When switching to devices tab, auto refresh
    if (selectedKey === "1") {
      fetchDevices();
    }
    // When switching to apps, logcat or files tab, if data is not loaded yet, fetch it
    if (
      (selectedKey === "2" || selectedKey === "4" || selectedKey === "6") &&
      selectedDevice
    ) {
      if (selectedKey === "2" && packages.length === 0) {
        fetchPackages("user", selectedDevice);
      } else if (selectedKey === "6" && fileList.length === 0) {
        fetchFiles(currentPath);
      }
    }
  }, [selectedKey, selectedDevice]);

  // Poll devices list when staying on devices tab
  useEffect(() => {
    if (selectedKey !== "1") {
      return; // Don't poll if not on devices tab
    }

    const pollInterval = setInterval(() => {
      // Only poll if not currently loading to avoid concurrent requests
      // Use silent=true to avoid showing loading state
      if (!loadingRef.current) {
        fetchDevices(true);
      }
    }, 3000); // Poll every 3 seconds

    return () => clearInterval(pollInterval);
  }, [selectedKey]);

  const fetchPackages = async (packageType?: string, deviceId?: string) => {
    const targetDevice = deviceId || selectedDevice;
    if (!targetDevice) return;
    const typeToFetch = packageType || typeFilter;
    setAppsLoading(true);
    try {
      const res = await ListPackages(targetDevice, typeToFetch);
      if (typeToFetch === "all") {
        // Replace all packages when fetching all
        setPackages(res || []);
      } else if (typeToFetch === "system") {
        // Merge system packages with existing user packages
        setPackages((prev) => {
          const userPackages = prev.filter((p) => p.type === "user");
          return [...userPackages, ...(res || [])];
        });
      } else {
        // user - replace all with user packages only
        setPackages(res || []);
      }
    } catch (err) {
      message.error(t("app.list_packages_failed") + ": " + String(err));
    } finally {
      setAppsLoading(false);
    }
  };

  const handleFetchAppInfo = async (
    packageName: string,
    force: boolean = false
  ) => {
    if (!selectedDevice) return;

    // If not forcing, show what we already have in the packages list
    if (!force) {
      setSelectedAppInfo(packages.find((p) => p.name === packageName) || null);
    }

    setPermissionSearch(""); // Reset search when opening info
    setInfoModalVisible(true);
    setInfoLoading(true);
    try {
      const res = await GetAppInfo(selectedDevice, packageName, force);
      setSelectedAppInfo(res);
      // Update the app in the packages list if we got new data
      // Only merge fields that are not empty to avoid overwriting existing type/state
      setPackages((prev) =>
        prev.map((p) => {
          if (p.name === packageName) {
            return {
              ...p,
              label: res.label || p.label,
              icon: res.icon || p.icon,
              versionName: res.versionName || p.versionName,
              versionCode: res.versionCode || p.versionCode,
              minSdkVersion: res.minSdkVersion || p.minSdkVersion,
              targetSdkVersion: res.targetSdkVersion || p.targetSdkVersion,
              permissions: res.permissions || p.permissions,
              activities: res.activities || p.activities,
            };
          }
          return p;
        })
      );
    } catch (err) {
      message.error(t("app.fetch_app_info_failed") + ": " + String(err));
    } finally {
      setInfoLoading(false);
    }
  };

  const fetchFiles = async (path: string) => {
    if (!selectedDevice) return;
    setFilesLoading(true);
    try {
      const res = await ListFiles(selectedDevice, path);
      setFileList(res || []);
      setCurrentPath(path);
    } catch (err) {
      message.error(t("app.list_files_failed") + ": " + String(err));
    } finally {
      setFilesLoading(false);
    }
  };

  const handleFileAction = async (action: string, file: any) => {
    if (!selectedDevice) return;
    try {
      switch (action) {
        case "open":
          const openKey = `open_${file.path}`;
          message.loading({
            content: (
              <span>
                {t("app.launching", { name: file.name })}
                <Button
                  type="link"
                  size="small"
                  onClick={async () => {
                    await CancelOpenFile(file.path);
                    message.destroy(openKey);
                  }}
                >
                  {t("common.cancel")}
                </Button>
              </span>
            ),
            key: openKey,
            duration: 0,
          });
          try {
            await OpenFileOnHost(selectedDevice, file.path);
            message.destroy(openKey);
          } catch (err) {
            if (String(err).includes("cancelled")) {
              message.info(t("app.open_cancelled"), 2);
            } else {
              message.error(t("app.open_failed") + ": " + String(err), 3);
            }
            message.destroy(openKey);
          }
          break;
        case "download":
          const downloadKey = `download_${file.path}`;
          try {
            const savePath = await DownloadFile(selectedDevice, file.path);
            if (savePath) {
              message.success(t("app.export_success", { path: savePath }));
            }
          } catch (err) {
            message.error(t("app.export_failed") + ": " + String(err));
          }
          break;
        case "delete":
          await DeleteFile(selectedDevice, file.path);
          message.success(t("app.delete_success", { name: file.name }));
          fetchFiles(currentPath);
          break;
        case "copy":
          setClipboard({ path: file.path, type: "copy" });
          message.info(t("app.copy_success", { name: file.name }));
          break;
        case "cut":
          setClipboard({ path: file.path, type: "cut" });
          message.info(t("app.cut_success", { name: file.name }));
          break;
        case "paste":
          if (!clipboard) return;
          const dest =
            currentPath +
            (currentPath.endsWith("/") ? "" : "/") +
            clipboard.path.split("/").pop();
          if (clipboard.type === "copy") {
            await CopyFile(selectedDevice, clipboard.path, dest);
            message.success(
              t("app.paste_success", { name: clipboard.path.split("/").pop() })
            );
          } else {
            await MoveFile(selectedDevice, clipboard.path, dest);
            message.success(
              t("app.move_success", { name: clipboard.path.split("/").pop() })
            );
            setClipboard(null);
          }
          fetchFiles(currentPath);
          break;
        case "rename":
          let newName = file.name;
          Modal.confirm({
            title: t("common.rename"),
            okText: t("common.ok"),
            cancelText: t("common.cancel"),
            autoFocusButton: null,
            content: (
              <FocusInput
                defaultValue={file.name}
                onChange={(e: any) => (newName = e.target.value)}
                placeholder={t("common.enter_new_name")}
                selectAll={true}
              />
            ),
            onOk: async () => {
              if (newName && newName !== file.name) {
                try {
                  const parentPath = currentPath.endsWith("/")
                    ? currentPath
                    : currentPath + "/";
                  const newPath = parentPath + newName;
                  await MoveFile(selectedDevice, file.path, newPath);
                  message.success(t("app.rename_success", { name: newName }));
                  fetchFiles(currentPath);
                } catch (err) {
                  message.error(t("app.rename_failed") + ": " + String(err));
                  throw err; // Keep modal open
                }
              }
            },
          });
          break;
        case "mkdir":
          let folderName = "";
          Modal.confirm({
            title: t("common.new_directory"),
            okText: t("common.ok"),
            cancelText: t("common.cancel"),
            autoFocusButton: null,
            content: (
              <FocusInput
                onChange={(e: any) => (folderName = e.target.value)}
                placeholder={t("common.folder_name")}
              />
            ),
            onOk: async () => {
              if (folderName) {
                try {
                  const parentPath = currentPath.endsWith("/")
                    ? currentPath
                    : currentPath + "/";
                  const newPath = parentPath + folderName;
                  await Mkdir(selectedDevice, newPath);
                  message.success(t("app.mkdir_success", { name: folderName }));
                  fetchFiles(currentPath);
                } catch (err) {
                  message.error(t("app.mkdir_failed") + ": " + String(err));
                  throw err; // Keep modal open
                }
              }
            },
          });
          break;
      }
    } catch (err) {
      message.error(t("app.command_failed") + ": " + String(err));
    }
  };

  const handleExploreAppFiles = (packageName: string) => {
    const path = `/sdcard/Android/data/${packageName}`;
    setCurrentPath(path);
    setSelectedKey("6"); // Switch to Files tab
    fetchFiles(path);
  };

  const handleUninstall = async (packageName: string) => {
    console.log(`Uninstalling ${packageName} from device ${selectedDevice}`);
    try {
      await UninstallApp(selectedDevice, packageName);
      message.success(t("app.uninstall_success", { name: packageName }));
      fetchPackages(typeFilter, selectedDevice);
    } catch (err) {
      console.error("Uninstall error:", err);
      message.error(t("app.uninstall_failed") + ": " + String(err));
    }
  };

  const handleClearData = async (packageName: string) => {
    try {
      await ClearAppData(selectedDevice, packageName);
      message.success(t("app.clear_data_success", { name: packageName }));
    } catch (err) {
      message.error(t("app.clear_data_failed") + ": " + String(err));
    }
  };

  const handleForceStop = async (packageName: string) => {
    try {
      await ForceStopApp(selectedDevice, packageName);
      message.success(t("app.force_stop_success", { name: packageName }));
    } catch (err) {
      message.error(t("app.force_stop_failed") + ": " + String(err));
    }
  };

  const handleStartApp = async (packageName: string) => {
    const hide = message.loading(t("app.launching", { name: packageName }), 0);
    try {
      // Force stop first as requested
      await ForceStopApp(selectedDevice, packageName);
      // Then start
      await StartApp(selectedDevice, packageName);
      message.success(t("app.start_app_success", { name: packageName }));
    } catch (err) {
      message.error(t("app.start_app_failed") + ": " + String(err));
    } finally {
      hide();
    }
  };

  const handleStartActivity = async (activityName: string) => {
    const hide = message.loading(t("app.launching", { name: activityName }), 0);
    try {
      await StartActivity(selectedDevice, activityName);
      message.success(t("app.start_activity_success"));
    } catch (err) {
      message.error(t("app.start_activity_failed") + ": " + String(err));
    } finally {
      hide();
    }
  };

  const handleToggleState = async (
    packageName: string,
    currentState: string
  ) => {
    try {
      if (currentState === "enabled") {
        await DisableApp(selectedDevice, packageName);
        message.success(t("app.disabled_success", { name: packageName }));
      } else {
        await EnableApp(selectedDevice, packageName);
        message.success(t("app.enabled_success", { name: packageName }));
      }
      fetchPackages(typeFilter, selectedDevice);
    } catch (err) {
      message.error(t("app.change_state_failed") + ": " + String(err));
    }
  };

  const handleShellCommand = async () => {
    if (!shellCmd) return;
    try {
      const args = shellCmd.trim().split(/\s+/);
      const res = await RunAdbCommand(args);
      setShellOutput(res);
    } catch (err) {
      message.error(t("app.command_failed"));
      setShellOutput(String(err));
    }
  };

  const startLogging = async (device: string, pkg: string) => {
    setLogs([]);
    try {
      await StartLogcat(device, pkg);
      setIsLogging(true);
      EventsOn("logcat-data", (data: string) => {
        setLogs((prev) => {
          const next = [...prev, data];
          if (next.length > 50000) {
            return next.slice(next.length - 50000);
          }
          return next;
        });
      });
    } catch (err) {
      message.error(t("app.start_logcat_failed") + ": " + String(err));
    }
  };

  const toggleLogcat = async () => {
    if (isLogging) {
      await StopLogcat();
      setIsLogging(false);
      EventsOff("logcat-data");
    } else {
      if (!selectedDevice) {
        message.error(t("app.no_device_selected"));
        return;
      }
      startLogging(selectedDevice, selectedPackage);
    }
  };

  const handleAppLogcat = async (pkgName: string) => {
    if (isLogging) {
      await StopLogcat();
      EventsOff("logcat-data");
      setIsLogging(false);
    }
    setSelectedPackage(pkgName);
    setSelectedKey("4");
    startLogging(selectedDevice, pkgName);
  };

  const handleStartScrcpy = async (
    deviceId: string,
    overrideConfig?: main.ScrcpyConfig
  ) => {
    try {
      let currentConfig = { ...(overrideConfig || scrcpyConfig) };

      await StartScrcpy(deviceId, currentConfig);

      // Separate recording launch if enabled
      if (shouldRecord && !isRecordingRef.current) {
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
      if (isRecordingRef.current) {
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
      // setShouldRecord(true); // Should already be true from switch
      setIsRecording(true);
      message.success(t("app.record_started"));
    } catch (err) {
      setIsRecording(false);
      setShouldRecord(false);
      message.error(t("app.record_failed") + ": " + String(err));
    }
  };

  const handleStopMidSessionRecord = async () => {
    if (!selectedDevice) return;
    try {
      setIsRecording(false);
      setShouldRecord(false);
      await StopRecording(selectedDevice);
    } catch (err) {
      message.error(t("app.record_stop_failed") + ": " + String(err));
    }
  };

  const updateScrcpyConfig = async (newConfig: main.ScrcpyConfig) => {
    setScrcpyConfig(newConfig);
    if (isMirroring && selectedDevice) {
      // Use setTimeout to allow state to propagate or just pass config directly
      await handleStartScrcpy(selectedDevice, newConfig);
    }
  };

  const handleInstallAPKs = async (deviceId: string, paths: string[]) => {
    if (!deviceId) {
      message.error(t("app.select_device"));
      return;
    }

    // Immediately switch to Apps tab and ensure correct device is selected
    setSelectedKey("2");
    if (selectedDevice !== deviceId) {
      setSelectedDevice(deviceId);
    }

    for (const path of paths) {
      const fileName = path.split(/[\\/]/).pop();
      const hideMessage = message.loading(
        t("app.installing", { name: fileName }),
        0
      );
      try {
        await InstallAPK(deviceId, path);
        message.success(t("app.install_success", { name: fileName }));

        // Refresh the list if we are on the correct device
        if (selectedDevice === deviceId) {
          fetchPackages(typeFilter, selectedDevice);
        }
      } catch (err) {
        message.error(
          t("app.install_failed", { name: fileName }) + ": " + String(err)
        );
      } finally {
        hideMessage();
      }
    }
  };

  const handleExportAPK = async (packageName: string) => {
    const hideMessage = message.loading(
      t("app.exporting", { name: packageName }),
      0
    );
    try {
      const res = await ExportAPK(selectedDevice, packageName);
      if (res) {
        message.success(t("app.export_success", { path: res }));
      }
    } catch (err) {
      message.error(t("app.export_failed") + ": " + String(err));
    } finally {
      hideMessage();
    }
  };

  const handleAdbDisconnect = async (address: string) => {
    try {
      await AdbDisconnect(address);
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

  const renderContent = () => {
    switch (selectedKey) {
      case "1":
        return (
          <DevicesView
            devices={devices}
            historyDevices={historyDevices}
            loading={loading}
            fetchDevices={fetchDevices}
            setSelectedKey={setSelectedKey}
            setSelectedDevice={setSelectedDevice}
            setShellCmd={setShellCmd}
            fetchFiles={fetchFiles}
            handleStartScrcpy={handleStartScrcpy}
            handleFetchDeviceInfo={handleFetchDeviceInfo}
            onShowWirelessConnect={() => setWirelessConnectVisible(true)}
            handleSwitchToWireless={handleSwitchToWireless}
            handleAdbDisconnect={handleAdbDisconnect}
            handleAdbConnect={handleAdbConnect}
            handleRemoveHistoryDevice={handleRemoveHistoryDevice}
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
            currentPath={currentPath}
            setCurrentPath={setCurrentPath}
            fileList={fileList}
            filesLoading={filesLoading}
            fetchFiles={fetchFiles}
            showHiddenFiles={showHiddenFiles}
            setShowHiddenFiles={setShowHiddenFiles}
            clipboard={clipboard}
            handleFileAction={handleFileAction}
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
            packages={packages}
            appsLoading={appsLoading}
            fetchPackages={fetchPackages}
            packageFilter={packageFilter}
            setPackageFilter={setPackageFilter}
            typeFilter={typeFilter}
            setTypeFilter={setTypeFilter}
            handleStartApp={handleStartApp}
            handleAppLogcat={handleAppLogcat}
            handleExploreAppFiles={handleExploreAppFiles}
            handleExportAPK={handleExportAPK}
            handleFetchAppInfo={handleFetchAppInfo}
            handleForceStop={handleForceStop}
            handleToggleState={handleToggleState}
            handleClearData={handleClearData}
            handleUninstall={handleUninstall}
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
            shellCmd={shellCmd}
            setShellCmd={setShellCmd}
            shellOutput={shellOutput}
            setShellOutput={setShellOutput}
            handleShellCommand={handleShellCommand}
          />
        );
      case "4":
        return (
          <LogcatView
            devices={devices}
            selectedDevice={selectedDevice}
            setSelectedDevice={setSelectedDevice}
            fetchDevices={fetchDevices}
            packages={packages}
            selectedPackage={selectedPackage}
            setSelectedPackage={setSelectedPackage}
            isLogging={isLogging}
            toggleLogcat={toggleLogcat}
            logs={logs}
            setLogs={setLogs}
            logFilter={logFilter}
            setLogFilter={setLogFilter}
            autoScroll={autoScroll}
            setAutoScroll={setAutoScroll}
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
            isMirroring={isMirroring}
            mirrorDuration={mirrorDuration}
            handleStartScrcpy={handleStartScrcpy}
            handleStopScrcpy={handleStopScrcpy}
            scrcpyConfig={scrcpyConfig}
            setScrcpyConfig={setScrcpyConfig}
            updateScrcpyConfig={updateScrcpyConfig}
            shouldRecord={shouldRecord}
            setShouldRecord={setShouldRecord}
            isRecording={isRecording}
            recordDuration={recordDuration}
            handleStartMidSessionRecord={handleStartMidSessionRecord}
            handleStopMidSessionRecord={handleStopMidSessionRecord}
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
          <div style={{ flex: 1, overflowY: "auto" }}>
            <Menu
              theme="dark"
              selectedKeys={[selectedKey]}
              mode="inline"
              onClick={({ key }) => setSelectedKey(key)}
              items={[
                {
                  key: "1",
                  icon: <MobileOutlined />,
                  label: t("menu.devices"),
                },
                { key: "2", icon: <AppstoreOutlined />, label: t("menu.apps") },
                { key: "6", icon: <FolderOutlined />, label: t("menu.files") },
                { key: "3", icon: <CodeOutlined />, label: t("menu.shell") },
                {
                  key: "4",
                  icon: <FileTextOutlined />,
                  label: t("menu.logcat"),
                },
                {
                  key: "5",
                  icon: <DesktopOutlined />,
                  label: t("menu.mirror"),
                },
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
                  {
                    key: "en",
                    label: "English",
                    onClick: () => i18n.changeLanguage("en"),
                  },
                  {
                    key: "zh",
                    label: "简体中文",
                    onClick: () => i18n.changeLanguage("zh"),
                  },
                  {
                    key: "zh-TW",
                    label: "繁體中文 (台灣)",
                    onClick: () => i18n.changeLanguage("zh-TW"),
                  },
                  {
                    key: "ja",
                    label: "日本語",
                    onClick: () => i18n.changeLanguage("ja"),
                  },
                  {
                    key: "ko",
                    label: "한국어",
                    onClick: () => i18n.changeLanguage("ko"),
                  },
                ],
                selectedKeys: [i18n.language],
              }}
              placement="topCenter"
            >
              <Button
                type="text"
                size="small"
                icon={
                  <TranslationOutlined
                    style={{
                      fontSize: "16px",
                      color: "rgba(255,255,255,0.45)",
                    }}
                  />
                }
                title={t("app.change_language")}
              />
            </Dropdown>
            <Button
              type="text"
              size="small"
              icon={
                <InfoCircleOutlined
                  style={{ fontSize: "16px", color: "rgba(255,255,255,0.45)" }}
                />
              }
              onClick={() => setAboutVisible(true)}
              title={t("app.about")}
            />
            <Button
              type="text"
              size="small"
              icon={
                <GithubOutlined
                  style={{ fontSize: "16px", color: "rgba(255,255,255,0.45)" }}
                />
              }
              onClick={() =>
                BrowserOpenURL &&
                BrowserOpenURL("https://github.com/nicetooo/adbGUI")
              }
              title={t("app.github")}
            />
            <Button
              type="text"
              size="small"
              icon={
                <BugOutlined
                  style={{ fontSize: "16px", color: "rgba(255,255,255,0.45)" }}
                />
              }
              onClick={() => {
                if (!BrowserOpenURL) return;
                const currentDevice = devices.find(
                  (d) => d.id === selectedDevice
                );
                const deviceInfo = currentDevice
                  ? `${currentDevice.brand} ${currentDevice.model} (${
                      selectedDeviceInfo?.serial === selectedDevice &&
                      selectedDeviceInfo.androidVer
                        ? `Android ${selectedDeviceInfo.androidVer}, `
                        : ""
                    }ID: ${currentDevice.id})`
                  : "None";

                const body = encodeURIComponent(
                  `### Description\n(Please describe the issue here)\n\n### Environment\n- App Version: 1.0.0\n- Device: ${deviceInfo}\n- OS: ${navigator.platform}\n\n### Steps to Reproduce\n1. \n2. \n\n### Expected Behavior\n\n### Actual Behavior`
                );
                BrowserOpenURL(
                  `https://github.com/nicetooo/adbGUI/issues/new?body=${body}`
                );
              }}
              title={t("app.feedback")}
            />
          </div>
        </div>
      </Sider>
      <Layout className="site-layout" style={{ marginLeft: 200 }}>
        <Content
          style={{
            margin: "0",
            height: "100vh",
            overflow: "hidden",
            display: "flex",
            flexDirection: "column",
          }}
        >
          {renderContent()}
        </Content>
      </Layout>

      <AppInfoModal
        visible={infoModalVisible}
        onCancel={() => setInfoModalVisible(false)}
        selectedAppInfo={selectedAppInfo}
        infoLoading={infoLoading}
        handleFetchAppInfo={handleFetchAppInfo}
        permissionSearch={permissionSearch}
        setPermissionSearch={setPermissionSearch}
        activitySearch={activitySearch}
        setActivitySearch={setActivitySearch}
        handleStartActivity={handleStartActivity}
      />

      <DeviceInfoModal
        visible={deviceInfoVisible}
        onCancel={() => setDeviceInfoVisible(false)}
        deviceInfo={selectedDeviceInfo}
        loading={deviceInfoLoading}
        onRefresh={() =>
          selectedDeviceInfo &&
          handleFetchDeviceInfo(selectedDeviceInfo.serial || selectedDevice)
        }
      />

      <AboutModal
        visible={aboutVisible}
        onCancel={() => setAboutVisible(false)}
      />

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
