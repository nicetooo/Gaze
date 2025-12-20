import { useState, useEffect, useRef } from 'react';
import { Layout, Menu, Table, Button, Tag, Space, message, Input, Select, Popconfirm, Radio, Dropdown, List, Switch, Slider, InputNumber, Card, Row, Col, Modal, Tooltip, Checkbox } from 'antd';
import LogcatView from './components/LogcatView';
import { 
  MobileOutlined, 
  AppstoreOutlined, 
  CodeOutlined, 
  ReloadOutlined,
  DeleteOutlined,
  MoreOutlined,
  ClearOutlined,
  StopOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  FileTextOutlined,
  PauseOutlined, 
  PlayCircleOutlined, 
  DesktopOutlined, 
  SettingOutlined, 
  DownloadOutlined, 
  InfoCircleOutlined,
  FolderOutlined,
  FileOutlined,
  ArrowLeftOutlined,
  CopyOutlined,
  ScissorOutlined,
  SnippetsOutlined,
  FolderAddOutlined,
  FolderOpenOutlined
} from '@ant-design/icons';
import './App.css';
// @ts-ignore
import { GetDevices, RunAdbCommand, ListPackages, GetAppInfo, UninstallApp, ClearAppData, ForceStopApp, StartApp, EnableApp, DisableApp, StartLogcat, StopLogcat, StartScrcpy, InstallAPK, ExportAPK, ListFiles, DeleteFile, MoveFile, CopyFile, Mkdir } from '../wailsjs/go/main/App';
// @ts-ignore
import { main } from '../wailsjs/go/models';
// @ts-ignore
const EventsOn = (window as any).runtime.EventsOn;
// @ts-ignore
const EventsOff = (window as any).runtime.EventsOff;

const { Content, Sider } = Layout;
const { Option } = Select;

interface Device {
  id: string;
  state: string;
  model: string;
  brand: string;
}

function App() {
  const [collapsed, setCollapsed] = useState(false);
  const [selectedKey, setSelectedKey] = useState('1');
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(false);
  
  // Shell state
  const [shellOutput, setShellOutput] = useState('');
  const [shellCmd, setShellCmd] = useState('');

  // Apps state
  const [selectedDevice, setSelectedDevice] = useState<string>('');
  const [packages, setPackages] = useState<main.AppPackage[]>([]);
  const [appsLoading, setAppsLoading] = useState(false);
  const [packageFilter, setPackageFilter] = useState('');
  const [typeFilter, setTypeFilter] = useState('user'); // all, system, user - default to user
  const [selectedAppInfo, setSelectedAppInfo] = useState<main.AppPackage | null>(null);
  const [infoModalVisible, setInfoModalVisible] = useState(false);
  const [infoLoading, setInfoLoading] = useState(false);
  const [permissionSearch, setPermissionSearch] = useState('');

  // Files state
  const [currentPath, setCurrentPath] = useState('/');
  const [fileList, setFileList] = useState<any[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [clipboard, setClipboard] = useState<{ path: string, type: 'copy' | 'cut' } | null>(null);
  const [showHiddenFiles, setShowHiddenFiles] = useState(false);

  // Logcat state
  const [logs, setLogs] = useState<string[]>([]);
  const [isLogging, setIsLogging] = useState(false);
  const [logFilter, setLogFilter] = useState('');
  const [selectedPackage, setSelectedPackage] = useState<string>('');
  const [autoScroll, setAutoScroll] = useState(true);
  const logsContainerRef = useRef<HTMLDivElement>(null);

  // Scrcpy state
  const [scrcpyConfig, setScrcpyConfig] = useState<main.ScrcpyConfig>({
    maxSize: 0,
    bitRate: 8,
    maxFps: 60,
    stayAwake: true,
    turnScreenOff: false,
    noAudio: false,
    alwaysOnTop: false
  });

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
            fetchPackages('user', deviceId);
          }, 100);
        }
      }
    } catch (err) {
      message.error('Failed to fetch devices: ' + String(err));
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

        const apkFiles = actualPaths.filter(p => typeof p === 'string' && p.toLowerCase().endsWith('.apk'));
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
    
    return () => {
      EventsOff("wails:file-drop");
      StopLogcat();
    };
  }, []);

  useEffect(() => {
    // When switching to apps, logcat or files tab, if data is not loaded yet, fetch it
    if ((selectedKey === '2' || selectedKey === '4' || selectedKey === '6') && selectedDevice) {
      if (selectedKey === '2' && packages.length === 0) {
        fetchPackages('user', selectedDevice);
      } else if (selectedKey === '6' && fileList.length === 0) {
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
      if (typeToFetch === 'all') {
        // Replace all packages when fetching all
        setPackages(res || []);
      } else if (typeToFetch === 'system') {
        // Merge system packages with existing user packages
        setPackages(prev => {
          const userPackages = prev.filter(p => p.type === 'user');
          return [...userPackages, ...(res || [])];
        });
      } else {
        // user - replace all with user packages only
        setPackages(res || []);
      }
    } catch (err) {
      message.error('Failed to list packages: ' + String(err));
    } finally {
      setAppsLoading(false);
    }
  };

  const handleFetchAppInfo = async (packageName: string, force: boolean = false) => {
    if (!selectedDevice) return;
    
    // If not forcing, show what we already have in the packages list
    if (!force) {
      setSelectedAppInfo(packages.find(p => p.name === packageName) || null);
    }
    
    setPermissionSearch(''); // Reset search when opening info
    setInfoModalVisible(true);
    setInfoLoading(true);
    try {
      const res = await GetAppInfo(selectedDevice, packageName, force);
      setSelectedAppInfo(res);
      // Update the app in the packages list if we got new data
      // Only merge fields that are not empty to avoid overwriting existing type/state
      setPackages(prev => prev.map(p => {
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
          };
        }
        return p;
      }));
    } catch (err) {
      message.error('Failed to fetch app info: ' + String(err));
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
      message.error('Failed to list files: ' + String(err));
    } finally {
      setFilesLoading(false);
    }
  };

  const handleFileAction = async (action: string, file: any) => {
    if (!selectedDevice) return;
    try {
      switch (action) {
        case 'delete':
          await DeleteFile(selectedDevice, file.path);
          message.success('Deleted ' + file.name);
          fetchFiles(currentPath);
          break;
        case 'copy':
          setClipboard({ path: file.path, type: 'copy' });
          message.info('Copied ' + file.name);
          break;
        case 'cut':
          setClipboard({ path: file.path, type: 'cut' });
          message.info('Cut ' + file.name);
          break;
        case 'paste':
          if (!clipboard) return;
          const dest = currentPath + (currentPath.endsWith('/') ? '' : '/') + clipboard.path.split('/').pop();
          if (clipboard.type === 'copy') {
            await CopyFile(selectedDevice, clipboard.path, dest);
            message.success('Pasted ' + clipboard.path.split('/').pop());
          } else {
            await MoveFile(selectedDevice, clipboard.path, dest);
            message.success('Moved ' + clipboard.path.split('/').pop());
            setClipboard(null);
          }
          fetchFiles(currentPath);
          break;
        case 'rename':
          Modal.confirm({
            title: 'Rename File',
            content: <Input defaultValue={file.name} id="rename-input" />,
            onOk: async () => {
              const newName = (document.getElementById('rename-input') as HTMLInputElement).value;
              if (newName && newName !== file.name) {
                const newPath = currentPath + (currentPath.endsWith('/') ? '' : '/') + newName;
                await MoveFile(selectedDevice, file.path, newPath);
                message.success('Renamed to ' + newName);
                fetchFiles(currentPath);
              }
            }
          });
          break;
        case 'mkdir':
          Modal.confirm({
            title: 'New Directory',
            content: <Input placeholder="Directory name" id="mkdir-input" />,
            onOk: async () => {
              const name = (document.getElementById('mkdir-input') as HTMLInputElement).value;
              if (name) {
                const newPath = currentPath + (currentPath.endsWith('/') ? '' : '/') + name;
                await Mkdir(selectedDevice, newPath);
                message.success('Created directory ' + name);
                fetchFiles(currentPath);
              }
            }
          });
          break;
      }
    } catch (err) {
      message.error('File action failed: ' + String(err));
    }
  };

  const handleExploreAppFiles = (packageName: string) => {
    const path = `/sdcard/Android/data/${packageName}`;
    setCurrentPath(path);
    setSelectedKey('6'); // Switch to Files tab
    fetchFiles(path);
  };

  const handleUninstall = async (packageName: string) => {
    console.log(`Uninstalling ${packageName} from device ${selectedDevice}`);
    try {
      await UninstallApp(selectedDevice, packageName);
      message.success(`Uninstalled ${packageName}`);
      fetchPackages(typeFilter, selectedDevice);
    } catch (err) {
      console.error('Uninstall error:', err);
      message.error('Failed to uninstall: ' + String(err));
    }
  };

  const handleClearData = async (packageName: string) => {
    try {
      await ClearAppData(selectedDevice, packageName);
      message.success(`Cleared data for ${packageName}`);
    } catch (err) {
      message.error('Failed to clear data: ' + String(err));
    }
  };

  const handleForceStop = async (packageName: string) => {
    try {
      await ForceStopApp(selectedDevice, packageName);
      message.success(`Force stopped ${packageName}`);
    } catch (err) {
      message.error('Failed to force stop: ' + String(err));
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
      message.error('Failed to start app: ' + String(err));
    } finally {
      hide();
    }
  };

  const handleToggleState = async (packageName: string, currentState: string) => {
    try {
      if (currentState === 'enabled') {
        await DisableApp(selectedDevice, packageName);
        message.success(`Disabled ${packageName}`);
      } else {
        await EnableApp(selectedDevice, packageName);
        message.success(`Enabled ${packageName}`);
      }
      fetchPackages(typeFilter, selectedDevice);
    } catch (err) {
      message.error('Failed to change app state: ' + String(err));
    }
  };

  const handleShellCommand = async () => {
    if (!shellCmd) return;
    try {
      const args = shellCmd.trim().split(/\s+/);
      const res = await RunAdbCommand(args);
      setShellOutput(res);
    } catch (err) {
      message.error('Command failed');
      setShellOutput(String(err));
    }
  };

  const startLogging = async (device: string, pkg: string) => {
      setLogs([]);
      try {
        await StartLogcat(device, pkg);
        setIsLogging(true);
        EventsOn("logcat-data", (data: string) => {
          setLogs(prev => {
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
      setSelectedKey('4');
      startLogging(selectedDevice, pkgName);
  };

  const handleStartScrcpy = async (deviceId: string) => {
    try {
      await StartScrcpy(deviceId, scrcpyConfig);
      message.success('Starting Scrcpy...');
    } catch (err) {
      message.error('Failed to start Scrcpy: ' + String(err));
    }
  };

  const handleInstallAPKs = async (deviceId: string, paths: string[]) => {
    if (!deviceId) {
      message.error("Please select a device first");
      return;
    }

    // Immediately switch to Apps tab and ensure correct device is selected
    setSelectedKey('2');
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
      message.error('Export failed: ' + String(err));
    } finally {
      hideMessage();
    }
  };

  const deviceColumns = [
    {
      title: 'Device ID',
      dataIndex: 'id',
      key: 'id',
    },
    {
      title: 'Brand',
      dataIndex: 'brand',
      key: 'brand',
      render: (brand: string) => brand ? brand.toUpperCase() : '-',
    },
    {
      title: 'Model',
      dataIndex: 'model',
      key: 'model',
    },
    {
      title: 'State',
      dataIndex: 'state',
      key: 'state',
      render: (state: string) => (
        <Tag color={state === 'device' ? 'green' : 'red'}>{state.toUpperCase()}</Tag>
      ),
    },
    {
      title: 'Action',
      key: 'action',
      render: (_: any, record: Device) => (
        <Space size="middle">
          <Button type="primary" size="small" onClick={() => {
             setShellCmd(`-s ${record.id} shell ls -l`);
             setSelectedKey('3');
          }}>
            Shell
          </Button>
          <Button size="small" onClick={() => {
             setSelectedDevice(record.id);
             setSelectedKey('2');
          }}>
            Apps
          </Button>
          <Button size="small" onClick={() => {
             setSelectedDevice(record.id);
             setSelectedKey('4');
          }}>
            Logcat
          </Button>
          <Button size="small" onClick={() => {
             setSelectedDevice(record.id);
             setSelectedKey('6');
             fetchFiles('/');
          }}>
            Files
          </Button>
          <Button 
            icon={<DesktopOutlined />} 
            size="small" 
            onClick={() => handleStartScrcpy(record.id)}
            title="Mirror Screen"
          >
            Mirror
          </Button>
        </Space>
      ),
    },
  ];

  const appColumns = [
    {
      title: 'App',
      key: 'app',
      render: (_: any, record: main.AppPackage) => {
        const firstLetter = (record.label || record.name).charAt(0).toUpperCase();
        const colors = [
          '#f56a00', '#7265e6', '#ffbf00', '#00a2ae', '#1890ff', 
          '#52c41a', '#eb2f96', '#fadb14', '#fa541c', '#13c2c2'
        ];
        const color = colors[Math.abs(record.name.split('').reduce((a, b) => (a << 5) - a + b.charCodeAt(0), 0)) % colors.length];

        return (
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{ 
              width: 36, 
              height: 36, 
              borderRadius: 8, 
              backgroundColor: color, 
              display: 'flex', 
              alignItems: 'center', 
              justifyContent: 'center',
              position: 'relative',
              overflow: 'hidden',
              flexShrink: 0,
              fontSize: '18px',
              fontWeight: 'bold',
              color: '#fff',
              boxShadow: '0 2px 4px rgba(0,0,0,0.1)'
            }}>
              {record.icon ? (
                <img 
                  src={record.icon}
                  style={{ 
                    width: '100%', 
                    height: '100%', 
                    objectFit: 'cover',
                    position: 'absolute',
                    zIndex: 2 
                  }}
                  onError={(e) => {
                    (e.target as HTMLImageElement).style.opacity = '0';
                  }}
                  alt=""
                />
              ) : (
                <img 
                  src={`https://play-lh.googleusercontent.com/i-p/get-icon?id=${record.name}&w=72`}
                  style={{ 
                    width: '100%', 
                    height: '100%', 
                    objectFit: 'cover',
                    position: 'absolute',
                    zIndex: 2 
                  }}
                  onError={(e) => {
                    (e.target as HTMLImageElement).style.opacity = '0';
                  }}
                  alt=""
                />
              )}
              <span style={{ position: 'relative', zIndex: 1 }}>{firstLetter}</span>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', lineHeight: 1.1 }}>
              <span style={{ fontWeight: 600, fontSize: '14px', color: '#1a1a1a' }}>{record.label || record.name}</span>
              <span style={{ fontSize: '10px', color: '#888', fontFamily: 'monospace' }}>{record.name}</span>
            </div>
          </div>
        );
      }
    },
    {
      title: 'Type',
      dataIndex: 'type',
      key: 'type',
      width: 100,
      render: (type: string) => (
        <Tag color={type === 'system' ? 'orange' : 'blue'}>
          {type === 'system' ? 'System' : 'User'}
        </Tag>
      ),
    },
    {
      title: 'State',
      dataIndex: 'state',
      key: 'state',
      width: 100,
      render: (state: string) => (
        <Tag color={state === 'enabled' ? 'green' : 'red'}>
          {state.toUpperCase()}
        </Tag>
      ),
    },
    {
      title: 'Action',
      key: 'action',
      width: 280,
      render: (_: any, record: main.AppPackage) => {
        return (
          <Space size={4}>
            <Tooltip title="Launch App">
              <Button 
                size="small" 
                icon={<PlayCircleOutlined />} 
                onClick={() => handleStartApp(record.name)}
              />
            </Tooltip>
            <Tooltip title="Logcat">
              <Button 
                size="small" 
                icon={<FileTextOutlined />} 
                onClick={() => handleAppLogcat(record.name)}
              />
            </Tooltip>
            <Tooltip title="Explore Files">
              <Button 
                size="small" 
                icon={<FolderOpenOutlined />} 
                onClick={() => handleExploreAppFiles(record.name)}
              />
            </Tooltip>
            <Tooltip title="Export APK">
              <Button 
                size="small" 
                icon={<DownloadOutlined />} 
                onClick={() => handleExportAPK(record.name)}
              />
            </Tooltip>
            <Tooltip title="App Info">
              <Button 
                size="small" 
                icon={<InfoCircleOutlined />} 
                onClick={() => handleFetchAppInfo(record.name)}
              />
            </Tooltip>
            <Dropdown menu={{ items: [
              {
                key: 'stop',
                icon: <StopOutlined />,
                label: 'Force Stop',
                onClick: () => handleForceStop(record.name)
              },
              {
                key: 'state',
                icon: record.state === 'enabled' ? <CloseCircleOutlined /> : <CheckCircleOutlined />,
                label: record.state === 'enabled' ? 'Disable' : 'Enable',
                onClick: () => handleToggleState(record.name, record.state)
              },
              {
                type: 'divider'
              },
              {
                key: 'clear',
                icon: <ClearOutlined />,
                label: 'Clear Data',
                danger: true,
                onClick: () => {
                   Modal.confirm({
                     title: 'Clear App Data',
                     content: `Are you sure you want to clear all data for ${record.name}? This cannot be undone.`,
                     okText: 'Clear',
                     okType: 'danger',
                     cancelText: 'Cancel',
                     onOk: () => handleClearData(record.name),
                   });
                }
              },
              {
                key: 'uninstall',
                icon: <DeleteOutlined />,
                label: 'Uninstall',
                danger: true,
                onClick: () => {
                   Modal.confirm({
                     title: 'Uninstall App',
                     content: `Are you sure you want to uninstall ${record.name}?`,
                     okText: 'Uninstall',
                     okType: 'danger',
                     cancelText: 'Cancel',
                     onOk: () => handleUninstall(record.name),
                   });
                }
              }
            ] }} trigger={['click']}>
              <Button size="small" icon={<MoreOutlined />} />
            </Dropdown>
          </Space>
        );
      },
    },
  ];



  const renderContent = () => {
    switch (selectedKey) {
      case '1':
        return (
          <div style={{ padding: 24 }}>
            <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <h2 style={{ margin: 0 }}>Connected Devices</h2>
              <Button icon={<ReloadOutlined />} onClick={fetchDevices} loading={loading}>
                Refresh
              </Button>
            </div>
            <Table columns={deviceColumns} dataSource={devices} rowKey="id" loading={loading} />
          </div>
        );
      case '6':
        const fileColumns = [
          {
            title: 'Name',
            key: 'name',
            sorter: (a: any, b: any) => a.name.localeCompare(b.name),
            render: (_: any, record: any) => (
              <Space onClick={() => record.isDir && fetchFiles(record.path)} style={{ cursor: record.isDir ? 'pointer' : 'default', width: '100%' }}>
                {record.isDir ? <FolderOutlined style={{ color: '#1890ff' }} /> : <FileOutlined />}
                <span>{record.name}</span>
              </Space>
            ),
          },
          {
            title: 'Size',
            dataIndex: 'size',
            key: 'size',
            width: 100,
            sorter: (a: any, b: any) => a.size - b.size,
            render: (size: number, record: any) => record.isDir ? '-' : (size > 1024 * 1024 ? (size / (1024 * 1024)).toFixed(2) + ' MB' : (size / 1024).toFixed(2) + ' KB'),
          },
          {
            title: 'Time',
            dataIndex: 'modTime',
            key: 'modTime',
            width: 180,
            defaultSortOrder: 'descend' as const,
            sorter: (a: any, b: any) => {
              if (a.modTime === 'N/A' && b.modTime === 'N/A') return 0;
              if (a.modTime === 'N/A') return -1;
              if (b.modTime === 'N/A') return 1;
              return a.modTime.localeCompare(b.modTime);
            },
          },
          {
            title: 'Action',
            key: 'action',
            width: 150,
            render: (_: any, record: any) => (
              <Space>
                <Tooltip title="Copy">
                  <Button size="small" icon={<CopyOutlined />} onClick={() => handleFileAction('copy', record)} />
                </Tooltip>
                <Tooltip title="Cut">
                  <Button size="small" icon={<ScissorOutlined />} onClick={() => handleFileAction('cut', record)} />
                </Tooltip>
                <Dropdown menu={{ items: [
                  { key: 'rename', label: 'Rename', onClick: () => handleFileAction('rename', record) },
                  { key: 'delete', label: 'Delete', danger: true, onClick: () => {
                    Modal.confirm({
                      title: 'Delete File',
                      content: `Are you sure you want to delete ${record.name}?`,
                      onOk: () => handleFileAction('delete', record),
                    });
                  }},
                ] }}>
                  <Button size="small" icon={<MoreOutlined />} />
                </Dropdown>
              </Space>
            ),
          },
        ];

        const filteredFiles = fileList.filter(f => showHiddenFiles || !f.name.startsWith('.'));

        return (
          <div style={{ padding: '16px 24px', height: '100%', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
            <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexShrink: 0 }}>
              <h2 style={{ margin: 0 }}>Files</h2>
              <Space>
                <Select 
                  value={selectedDevice} 
                  onChange={(val) => {
                    setSelectedDevice(val);
                    fetchFiles(currentPath);
                  }} 
                  style={{ width: 180 }} 
                  placeholder="Select Device"
                >
                  {devices.map(d => (
                    <Option key={d.id} value={d.id}>
                      {d.brand ? `${d.brand} ${d.model}` : (d.model || d.id)}
                    </Option>
                  ))}
                </Select>
                <Button icon={<ReloadOutlined />} onClick={() => fetchFiles(currentPath)} loading={filesLoading}>
                  Refresh
                </Button>
              </Space>
            </div>

            <div style={{ marginBottom: 12, display: 'flex', gap: 16, alignItems: 'center', flexShrink: 0 }}>
              <div style={{ flex: 1, display: 'flex', gap: 8 }}>
                <Button 
                  icon={<ArrowLeftOutlined />} 
                  onClick={() => {
                    const parts = currentPath.split('/').filter(Boolean);
                    parts.pop();
                    fetchFiles('/' + parts.join('/'));
                  }}
                  disabled={currentPath === '/' || currentPath === ''}
                />
                <Input 
                  value={currentPath} 
                  onChange={e => setCurrentPath(e.target.value)}
                  onPressEnter={() => fetchFiles(currentPath)}
                  style={{ flex: 1 }}
                />
              </div>
              <Space>
                <Checkbox checked={showHiddenFiles} onChange={(e: any) => setShowHiddenFiles(e.target.checked)}>
                  Show Hidden
                </Checkbox>
                <Button icon={<FolderAddOutlined />} onClick={() => handleFileAction('mkdir', null)}>New Folder</Button>
                <Button 
                  icon={<SnippetsOutlined />} 
                  disabled={!clipboard} 
                  onClick={() => handleFileAction('paste', null)}
                  type={clipboard ? 'primary' : 'default'}
                >
                  Paste {clipboard && `(${clipboard.type === 'copy' ? 'Copy' : 'Cut'})`}
                </Button>
              </Space>
            </div>

            <div style={{ flex: 1, overflow: 'hidden', backgroundColor: '#fff', borderRadius: '4px', border: '1px solid #f0f0f0' }}>
              <Table 
                columns={fileColumns} 
                dataSource={filteredFiles} 
                rowKey="path" 
                loading={filesLoading}
                pagination={false}
                size="small"
                scroll={{ y: 'calc(100vh - 180px)' }}
                onRow={(record) => ({
                  onDoubleClick: () => record.isDir && fetchFiles(record.path),
                })}
              />
            </div>
          </div>
        );
      case '2':
        const filteredPackages = packages
          .filter(p => {
            const matchesName = p.name.toLowerCase().includes(packageFilter.toLowerCase());
            const matchesType = typeFilter === 'all' || p.type === typeFilter;
            return matchesName && matchesType;
          });

        return (
          <div style={{ padding: 24, height: '100%', display: 'flex', flexDirection: 'column' }}>
            <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <h2 style={{ margin: 0 }}>Installed Apps</h2>
              <Space>
                <Select 
                  value={selectedDevice} 
                  onChange={setSelectedDevice} 
                  style={{ width: 200 }} 
                  placeholder="Select Device"
                >
                  {devices.map(d => (
                    <Option key={d.id} value={d.id}>
                      {d.brand ? `${d.brand} ${d.model}` : (d.model || d.id)}
                    </Option>
                  ))}
                </Select>
                <Button icon={<ReloadOutlined />} onClick={() => fetchPackages(typeFilter, selectedDevice)} loading={appsLoading}>
                  Refresh
                </Button>
              </Space>
            </div>
            <Space style={{ marginBottom: 16 }}>
              <Input 
                placeholder="Filter packages..." 
                value={packageFilter}
                onChange={e => setPackageFilter(e.target.value)}
                style={{ width: 300 }}
              />
              <Radio.Group value={typeFilter} onChange={e => {
                const newType = e.target.value;
                setTypeFilter(newType);
                // Fetch packages when type changes
                if (newType === 'all' || newType === 'system') {
                  fetchPackages(newType, selectedDevice);
                } else {
                  // user - just filter existing packages
                  fetchPackages('user', selectedDevice);
                }
              }}>
                <Radio.Button value="all">All</Radio.Button>
                <Radio.Button value="user">User</Radio.Button>
                <Radio.Button value="system">System</Radio.Button>
              </Radio.Group>
            </Space>
            <Table 
              columns={appColumns} 
              dataSource={filteredPackages} 
              rowKey="name" 
              loading={appsLoading}
              pagination={{ pageSize: 10 }}
              size="small"
            />
          </div>
        );
      case '3':
        return (
          <div style={{ padding: 24, height: '100%', display: 'flex', flexDirection: 'column' }}>
            <h2 style={{ marginBottom: 16 }}>ADB Shell</h2>
            <Space.Compact style={{ width: '100%', marginBottom: 16 }}>
              <Input 
                placeholder="Enter ADB command (e.g. shell ls -l)" 
                value={shellCmd}
                onChange={(e) => setShellCmd(e.target.value)}
                onPressEnter={handleShellCommand}
              />
              <Button type="primary" onClick={handleShellCommand}>Run</Button>
            </Space.Compact>
            <Input.TextArea 
              rows={15} 
              value={shellOutput} 
              readOnly 
              style={{ fontFamily: 'monospace', backgroundColor: '#f5f5f5', flex: 1 }} 
            />
          </div>
        );
      case '4':
        return <LogcatView 
          devices={devices}
          selectedDevice={selectedDevice}
          setSelectedDevice={setSelectedDevice}
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
        />;
      case '5':
        return (
          <div style={{ padding: 24, height: '100%', overflowY: 'auto' }}>
            <h2 style={{ marginBottom: 24 }}>Scrcpy Settings</h2>
            <Row gutter={[16, 16]}>
              <Col span={24}>
                <Card title="Device Selection" size="small">
                  <Space>
                    <Select 
                      value={selectedDevice} 
                      onChange={setSelectedDevice} 
                      style={{ width: 300 }} 
                      placeholder="Select Device"
                    >
                      {devices.map(d => (
                        <Option key={d.id} value={d.id}>
                          {d.brand ? `${d.brand} ${d.model}` : (d.model || d.id)}
                        </Option>
                      ))}
                    </Select>
                    <Button 
                      type="primary" 
                      icon={<DesktopOutlined />} 
                      onClick={() => handleStartScrcpy(selectedDevice)}
                      disabled={!selectedDevice}
                    >
                      Start Mirroring
                    </Button>
                  </Space>
                </Card>
              </Col>
              
              <Col span={12}>
                <Card title="Video Quality" size="small">
                  <div style={{ marginBottom: 16 }}>
                    <div style={{ marginBottom: 8 }}>Max Size (0 = auto)</div>
                    <InputNumber 
                      min={0} 
                      max={4096} 
                      value={scrcpyConfig.maxSize} 
                      onChange={v => setScrcpyConfig({...scrcpyConfig, maxSize: v || 0})}
                      style={{ width: '100%' }}
                    />
                  </div>
                  <div style={{ marginBottom: 16 }}>
                    <div style={{ marginBottom: 8 }}>Bit Rate (Mbps)</div>
                    <Slider 
                      min={1} 
                      max={64} 
                      value={scrcpyConfig.bitRate} 
                      onChange={v => setScrcpyConfig({...scrcpyConfig, bitRate: v})}
                    />
                  </div>
                  <div style={{ marginBottom: 16 }}>
                    <div style={{ marginBottom: 8 }}>Max FPS</div>
                    <Slider 
                      min={15} 
                      max={144} 
                      value={scrcpyConfig.maxFps} 
                      onChange={v => setScrcpyConfig({...scrcpyConfig, maxFps: v})}
                    />
                  </div>
                </Card>
              </Col>

              <Col span={12}>
                <Card title="Options" size="small">
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <span>Stay Awake</span>
                      <Switch 
                        checked={scrcpyConfig.stayAwake} 
                        onChange={v => setScrcpyConfig({...scrcpyConfig, stayAwake: v})}
                      />
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <span>Turn Screen Off</span>
                      <Switch 
                        checked={scrcpyConfig.turnScreenOff} 
                        onChange={v => setScrcpyConfig({...scrcpyConfig, turnScreenOff: v})}
                      />
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <span>No Audio</span>
                      <Switch 
                        checked={scrcpyConfig.noAudio} 
                        onChange={v => setScrcpyConfig({...scrcpyConfig, noAudio: v})}
                      />
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <span>Always On Top</span>
                      <Switch 
                        checked={scrcpyConfig.alwaysOnTop} 
                        onChange={v => setScrcpyConfig({...scrcpyConfig, alwaysOnTop: v})}
                      />
                    </div>
                  </Space>
                </Card>
              </Col>
            </Row>
          </div>
        );
      default:
        return <div style={{ padding: 24 }}>Select an option from the menu</div>;
    }
  };

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider collapsible collapsed={collapsed} onCollapse={setCollapsed}>
        <div className="logo" style={{ height: 32, margin: 16, background: 'rgba(255, 255, 255, 0.2)', borderRadius: 6, display: 'flex', justifyContent: 'center', alignItems: 'center', color: 'white', fontWeight: 'bold' }}>
          {!collapsed && 'ADB GUI'}
        </div>
        <Menu
          theme="dark"
          selectedKeys={[selectedKey]}
          mode="inline"
          onClick={({ key }) => setSelectedKey(key)}
          items={[
            { key: '1', icon: <MobileOutlined />, label: 'Devices' },
            { key: '2', icon: <AppstoreOutlined />, label: 'Apps' },
            { key: '6', icon: <FolderOutlined />, label: 'Files' },
            { key: '3', icon: <CodeOutlined />, label: 'Shell' },
            { key: '4', icon: <FileTextOutlined />, label: 'Logcat' },
            { key: '5', icon: <DesktopOutlined />, label: 'Scrcpy' },
          ]}
        />
      </Sider>
      <Layout className="site-layout">
        <Content style={{ margin: '0', height: '100vh', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
          {renderContent()}
        </Content>
      </Layout>
      
      <Modal
        title={
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', paddingRight: 32 }}>
            <span>App Information</span>
            {selectedAppInfo && (
              <Button 
                size="small" 
                icon={<ReloadOutlined spin={infoLoading} />} 
                onClick={() => handleFetchAppInfo(selectedAppInfo.name, true)}
                disabled={infoLoading}
              >
                Refresh
              </Button>
            )}
          </div>
        }
        open={infoModalVisible}
        onCancel={() => setInfoModalVisible(false)}
        footer={[
          <Button key="close" onClick={() => setInfoModalVisible(false)}>
            Close
          </Button>
        ]}
        width={600}
      >
        {infoLoading && !selectedAppInfo?.versionName ? (
          <div style={{ padding: '60px 0', textAlign: 'center' }}>
            <ReloadOutlined spin style={{ fontSize: 32, color: '#1890ff', marginBottom: 16 }} />
            <div style={{ fontSize: 16, color: '#666' }}>Fetching detailed app information...</div>
            <div style={{ fontSize: 12, color: '#999', marginTop: 8 }}>This may take a few seconds as we extract data from the APK</div>
          </div>
        ) : selectedAppInfo && (
          <div style={{ padding: '10px 0', opacity: infoLoading ? 0.6 : 1, transition: 'opacity 0.3s' }}>
            {infoLoading && (
              <div style={{ 
                position: 'absolute', 
                top: 0, 
                left: 0, 
                right: 0, 
                bottom: 0, 
                zIndex: 10, 
                display: 'flex', 
                flexDirection: 'column',
                alignItems: 'center', 
                justifyContent: 'center', 
                background: 'rgba(255,255,255,0.7)',
                borderRadius: 8
              }}>
                <ReloadOutlined spin style={{ fontSize: 32, color: '#1890ff', marginBottom: 12 }} />
                <div style={{ fontSize: 14, color: '#1890ff', fontWeight: 'bold' }}>Refreshing App Info...</div>
                <div style={{ fontSize: 12, color: '#666', marginTop: 4 }}>Extracting data from APK, please wait</div>
              </div>
            )}
            <div style={{ display: 'flex', alignItems: 'center', gap: 20, marginBottom: 24 }}>
              <div style={{ 
                width: 64, 
                height: 64, 
                borderRadius: 12, 
                backgroundColor: '#f0f0f0',
                display: 'flex', 
                alignItems: 'center', 
                justifyContent: 'center',
                overflow: 'hidden',
                flexShrink: 0,
                boxShadow: '0 2px 8px rgba(0,0,0,0.1)'
              }}>
                {selectedAppInfo.icon ? (
                  <img src={selectedAppInfo.icon} style={{ width: '100%', height: '100%', objectFit: 'cover' }} alt="" />
                ) : (
                  <AppstoreOutlined style={{ fontSize: 32, color: '#bfbfbf' }} />
                )}
              </div>
              <div>
                <h3 style={{ margin: 0, fontSize: 20 }}>{selectedAppInfo.label || selectedAppInfo.name}</h3>
                <code style={{ fontSize: 12, color: '#888' }}>{selectedAppInfo.name}</code>
              </div>
            </div>

            <Row gutter={[16, 16]}>
              <Col span={12}>
                <Card size="small" title="Version Info">
                  <p><strong>Version Name:</strong> {selectedAppInfo.versionName || 'N/A'}</p>
                  <p><strong>Version Code:</strong> {selectedAppInfo.versionCode || 'N/A'}</p>
                </Card>
              </Col>
              <Col span={12}>
                <Card size="small" title="SDK Info">
                  <p><strong>Min SDK:</strong> {selectedAppInfo.minSdkVersion || 'N/A'}</p>
                  <p><strong>Target SDK:</strong> {selectedAppInfo.targetSdkVersion || 'N/A'}</p>
                </Card>
              </Col>
              <Col span={24}>
                <Card 
                  size="small" 
                  title={
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <span>Permissions</span>
                      <Input 
                        placeholder="Search permissions..." 
                        size="small" 
                        style={{ width: 200 }} 
                        allowClear
                        value={permissionSearch}
                        onChange={e => setPermissionSearch(e.target.value)}
                      />
                    </div>
                  }
                >
                  <div style={{ maxHeight: 200, overflowY: 'auto' }}>
                    {selectedAppInfo.permissions && selectedAppInfo.permissions.length > 0 ? (
                      selectedAppInfo.permissions
                        .filter(p => p.toLowerCase().includes(permissionSearch.toLowerCase()))
                        .map((p, i) => (
                          <div key={i} style={{ fontSize: 12, padding: '4px 8px', borderBottom: '1px solid #f0f0f0' }}>
                            {p.replace('android.permission.', '')}
                          </div>
                        ))
                    ) : (
                      <p style={{ color: '#bfbfbf', fontStyle: 'italic', textAlign: 'center', padding: '20px 0' }}>No permissions listed</p>
                    )}
                    {selectedAppInfo.permissions && selectedAppInfo.permissions.length > 0 && 
                     selectedAppInfo.permissions.filter(p => p.toLowerCase().includes(permissionSearch.toLowerCase())).length === 0 && (
                      <p style={{ color: '#bfbfbf', fontStyle: 'italic', textAlign: 'center', padding: '20px 0' }}>No permissions match your search</p>
                    )}
                  </div>
                </Card>
              </Col>
            </Row>
          </div>
        )}
      </Modal>
    </Layout>
  );
}

export default App;
