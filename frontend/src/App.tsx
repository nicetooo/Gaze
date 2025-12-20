import { useState, useEffect, useRef } from "react";
import { Layout, Menu, Button, message, notification, Modal } from "antd";
import LogcatView from "./components/LogcatView";
import DeviceSelector from "./components/DeviceSelector";
import DevicesView from "./components/DevicesView";
import AppsView from "./components/AppsView";
import FilesView, { FocusInput } from "./components/FilesView";
import ShellView from "./components/ShellView";
import MirrorView from "./components/MirrorView";
import AppInfoModal from "./components/AppInfoModal";
import {
  MobileOutlined,
  AppstoreOutlined,
  CodeOutlined,
  FileTextOutlined,
  DesktopOutlined,
  FolderOutlined,
} from "@ant-design/icons";
import "./App.css";
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
  state: string;
  model: string;
  brand: string;
}

function App() {
  const [selectedKey, setSelectedKey] = useState("1");
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(false);

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

  useEffect(() => {
    isRecordingRef.current = isRecording;
  }, [isRecording]);

  useEffect(() => {
    recordPathRef.current = scrcpyConfig.recordPath;
  }, [scrcpyConfig.recordPath]);

  const fetchDevices = async () => {
    setLoading(true);
    try {
      const res = await GetDevices();
      setDevices(res || []);
      if (res && res.length > 0) {
        const deviceId = res[0].id;
        if (!selectedDevice) {
          setSelectedDevice(deviceId);
        }
        // Automatically fetch user packages when device is connected
        if (deviceId) {
          // Use setTimeout to avoid blocking the UI update
          setTimeout(() => {
            fetchPackages("user", deviceId);
          }, 100);
        }
      }
    } catch (err) {
      message.error("Failed to fetch devices: " + String(err));
    } finally {
      setLoading(false);
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
            message.error("Please select a device first");
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
        message: "Recording Saved",
        description: "The screen recording has been saved successfully.",
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
            Show in Folder
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
      message.error("Failed to list packages: " + String(err));
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
      message.error("Failed to fetch app info: " + String(err));
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
      message.error("Failed to list files: " + String(err));
    } finally {
      setFilesLoading(false);
    }
  };

  const handleFileAction = async (action: string, file: any) => {
    if (!selectedDevice) return;
    try {
      switch (action) {
        case "delete":
          await DeleteFile(selectedDevice, file.path);
          message.success("Deleted " + file.name);
          fetchFiles(currentPath);
          break;
        case "copy":
          setClipboard({ path: file.path, type: "copy" });
          message.info("Copied " + file.name);
          break;
        case "cut":
          setClipboard({ path: file.path, type: "cut" });
          message.info("Cut " + file.name);
          break;
        case "paste":
          if (!clipboard) return;
          const dest =
            currentPath +
            (currentPath.endsWith("/") ? "" : "/") +
            clipboard.path.split("/").pop();
          if (clipboard.type === "copy") {
            await CopyFile(selectedDevice, clipboard.path, dest);
            message.success("Pasted " + clipboard.path.split("/").pop());
          } else {
            await MoveFile(selectedDevice, clipboard.path, dest);
            message.success("Moved " + clipboard.path.split("/").pop());
            setClipboard(null);
          }
          fetchFiles(currentPath);
          break;
        case "rename":
          let newName = file.name;
          Modal.confirm({
            title: "Rename",
            autoFocusButton: null,
            content: (
              <FocusInput
                defaultValue={file.name}
                onChange={(e: any) => (newName = e.target.value)}
                placeholder="Enter new name"
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
                  message.success("Renamed to " + newName);
                  fetchFiles(currentPath);
                } catch (err) {
                  message.error("Rename failed: " + String(err));
                  throw err; // Keep modal open
                }
              }
            },
          });
          break;
        case "mkdir":
          let folderName = "";
          Modal.confirm({
            title: "New Directory",
            autoFocusButton: null,
            content: (
              <FocusInput
                onChange={(e: any) => (folderName = e.target.value)}
                placeholder="Folder name"
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
                  message.success("Created directory " + folderName);
                  fetchFiles(currentPath);
                } catch (err) {
                  message.error("Create directory failed: " + String(err));
                  throw err; // Keep modal open
                }
              }
            },
          });
          break;
      }
    } catch (err) {
      message.error("File action failed: " + String(err));
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
      message.success(`Uninstalled ${packageName}`);
      fetchPackages(typeFilter, selectedDevice);
    } catch (err) {
      console.error("Uninstall error:", err);
      message.error("Failed to uninstall: " + String(err));
    }
  };

  const handleClearData = async (packageName: string) => {
    try {
      await ClearAppData(selectedDevice, packageName);
      message.success(`Cleared data for ${packageName}`);
    } catch (err) {
      message.error("Failed to clear data: " + String(err));
    }
  };

  const handleForceStop = async (packageName: string) => {
    try {
      await ForceStopApp(selectedDevice, packageName);
      message.success(`Force stopped ${packageName}`);
    } catch (err) {
      message.error("Failed to force stop: " + String(err));
    }
  };

  const handleStartApp = async (packageName: string) => {
    const hide = message.loading(`Launching ${packageName}...`, 0);
    try {
      // Force stop first as requested
      await ForceStopApp(selectedDevice, packageName);
      // Then start
      await StartApp(selectedDevice, packageName);
      message.success(`Started ${packageName}`);
    } catch (err) {
      message.error("Failed to start app: " + String(err));
    } finally {
      hide();
    }
  };

  const handleStartActivity = async (activityName: string) => {
    const hide = message.loading(`Launching activity ${activityName}...`, 0);
    try {
      await StartActivity(selectedDevice, activityName);
      message.success(`Started activity successfully`);
    } catch (err) {
      message.error("Failed to start activity: " + String(err));
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
        message.success(`Disabled ${packageName}`);
      } else {
        await EnableApp(selectedDevice, packageName);
        message.success(`Enabled ${packageName}`);
      }
      fetchPackages(typeFilter, selectedDevice);
    } catch (err) {
      message.error("Failed to change app state: " + String(err));
    }
  };

  const handleShellCommand = async () => {
    if (!shellCmd) return;
    try {
      const args = shellCmd.trim().split(/\s+/);
      const res = await RunAdbCommand(args);
      setShellOutput(res);
    } catch (err) {
      message.error("Command failed");
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
      message.error("Failed to start logcat: " + String(err));
    }
  };

  const toggleLogcat = async () => {
    if (isLogging) {
      await StopLogcat();
      setIsLogging(false);
      EventsOff("logcat-data");
    } else {
      if (!selectedDevice) {
        message.error("No device selected");
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
          shouldRecord ? "Starting Recording & Mirror..." : "Starting Mirror..."
        );
      }
    } catch (err) {
      message.error("Failed to start Scrcpy: " + String(err));
    }
  };

  const handleStopScrcpy = async (deviceId: string) => {
    try {
      if (isRecordingRef.current) {
        await StopRecording(deviceId);
      }
      await StopScrcpy(deviceId);
      message.success("Mirror stopped");
    } catch (err) {
      message.error("Failed to stop Mirror: " + String(err));
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
      message.success("Recording started");
    } catch (err) {
      setIsRecording(false);
      setShouldRecord(false);
      message.error("Failed to start recording: " + String(err));
    }
  };

  const handleStopMidSessionRecord = async () => {
    if (!selectedDevice) return;
    try {
      setIsRecording(false);
      setShouldRecord(false);
      await StopRecording(selectedDevice);
    } catch (err) {
      message.error("Failed to stop recording: " + String(err));
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
      message.error("Please select a device first");
      return;
    }

    // Immediately switch to Apps tab and ensure correct device is selected
    setSelectedKey("2");
    if (selectedDevice !== deviceId) {
      setSelectedDevice(deviceId);
    }

    for (const path of paths) {
      const fileName = path.split(/[\\/]/).pop();
      const hideMessage = message.loading(`Installing ${fileName}...`, 0);
      try {
        await InstallAPK(deviceId, path);
        message.success(`Installed ${fileName} successfully`);

        // Refresh the list if we are on the correct device
        if (selectedDevice === deviceId) {
          fetchPackages(typeFilter, selectedDevice);
        }
      } catch (err) {
        message.error(`Failed to install ${fileName}: ${String(err)}`);
      } finally {
        hideMessage();
      }
    }
  };

  const handleExportAPK = async (packageName: string) => {
    const hideMessage = message.loading(`Exporting ${packageName}...`, 0);
    try {
      const res = await ExportAPK(selectedDevice, packageName);
      if (res) {
        message.success(`Exported to ${res}`);
      }
    } catch (err) {
      message.error("Export failed: " + String(err));
    } finally {
      hideMessage();
    }
  };

  const renderContent = () => {
    switch (selectedKey) {
      case "1":
        return (
          <DevicesView
            devices={devices}
            loading={loading}
            fetchDevices={fetchDevices}
            setSelectedKey={setSelectedKey}
            setSelectedDevice={setSelectedDevice}
            setShellCmd={setShellCmd}
            fetchFiles={fetchFiles}
            handleStartScrcpy={handleStartScrcpy}
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
        return (
          <div style={{ padding: 24 }}>Select an option from the menu</div>
        );
    }
  };

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <Sider width={200} theme="dark">
        <Menu
          theme="dark"
          selectedKeys={[selectedKey]}
          mode="inline"
          onClick={({ key }) => setSelectedKey(key)}
          items={[
            { key: "1", icon: <MobileOutlined />, label: "Devices" },
            { key: "2", icon: <AppstoreOutlined />, label: "Apps" },
            { key: "6", icon: <FolderOutlined />, label: "Files" },
            { key: "3", icon: <CodeOutlined />, label: "Shell" },
            { key: "4", icon: <FileTextOutlined />, label: "Logcat" },
            { key: "5", icon: <DesktopOutlined />, label: "Mirror" },
          ]}
        />
      </Sider>
      <Layout className="site-layout">
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
    </Layout>
  );
}

export default App;
