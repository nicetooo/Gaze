import { useState, useEffect, useRef } from 'react';
import { Layout, Menu, Table, Button, Tag, Space, message, Input, Select, Popconfirm, Radio, Dropdown, List, Switch, Slider, InputNumber, Card, Row, Col, Modal } from 'antd';
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
  DownloadOutlined
} from '@ant-design/icons';
import './App.css';
// @ts-ignore
import { GetDevices, RunAdbCommand, ListPackages, UninstallApp, ClearAppData, ForceStopApp, StartApp, EnableApp, DisableApp, StartLogcat, StopLogcat, StartScrcpy, InstallAPK, ExportAPK } from '../wailsjs/go/main/App';
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
  const [typeFilter, setTypeFilter] = useState('all'); // all, system, user

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
      if (res && res.length > 0 && !selectedDevice) {
        setSelectedDevice(res[0].id);
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
    if ((selectedKey === '2' || selectedKey === '4') && selectedDevice) {
      fetchPackages();
    }
  }, [selectedKey, selectedDevice]);

  const fetchPackages = async () => {
    if (!selectedDevice) return;
    setAppsLoading(true);
    try {
      const res = await ListPackages(selectedDevice);
      setPackages(res || []);
    } catch (err) {
      message.error('Failed to list packages: ' + String(err));
    } finally {
      setAppsLoading(false);
    }
  };

  const handleUninstall = async (packageName: string) => {
    console.log(`Uninstalling ${packageName} from device ${selectedDevice}`);
    try {
      await UninstallApp(selectedDevice, packageName);
      message.success(`Uninstalled ${packageName}`);
      fetchPackages();
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
      fetchPackages();
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
          setLogs(prev => [...prev, data]);
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
            fetchPackages();
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
      title: 'Package Name',
      dataIndex: 'name',
      key: 'name',
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
      width: 100,
      render: (_: any, record: main.AppPackage) => {
        return (
          <Dropdown menu={{ items: [
            {
              key: 'logcat',
              icon: <FileTextOutlined />,
              label: 'Logcat',
              onClick: () => handleAppLogcat(record.name)
            },
            {
              key: 'export',
              icon: <DownloadOutlined />,
              label: 'Export APK',
              onClick: () => handleExportAPK(record.name)
            },
            {
              type: 'divider'
            },
            {
              key: 'start',
              icon: <PlayCircleOutlined />,
              label: 'Launch App',
              onClick: () => handleStartApp(record.name)
            },
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
                <Button icon={<ReloadOutlined />} onClick={fetchPackages} loading={appsLoading}>
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
              <Radio.Group value={typeFilter} onChange={e => setTypeFilter(e.target.value)}>
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
        const filteredLogs = logs.filter(l => l.toLowerCase().includes(logFilter.toLowerCase()));
        return <LogcatView 
          devices={devices}
          selectedDevice={selectedDevice}
          setSelectedDevice={setSelectedDevice}
          packages={packages}
          selectedPackage={selectedPackage}
          setSelectedPackage={setSelectedPackage}
          isLogging={isLogging}
          toggleLogcat={toggleLogcat}
          logs={filteredLogs}
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
    </Layout>
  );
}

export default App;
