import { useState, useEffect } from 'react';
import { Layout, Menu, Table, Button, Tag, Space, message, Input, Select, Popconfirm, Radio, Dropdown } from 'antd';
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
  CloseCircleOutlined
} from '@ant-design/icons';
import './App.css';
// @ts-ignore
import { GetDevices, RunAdbCommand, ListPackages, UninstallApp, ClearAppData, ForceStopApp, EnableApp, DisableApp } from '../wailsjs/go/main/App';
// @ts-ignore
import { main } from '../wailsjs/go/models';

const { Content, Sider } = Layout;
const { Option } = Select;

interface Device {
  id: string;
  state: string;
  model: string;
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

  useEffect(() => {
    fetchDevices();
  }, []);

  useEffect(() => {
    if (selectedKey === '2' && selectedDevice) {
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
    try {
      await UninstallApp(selectedDevice, packageName);
      message.success(`Uninstalled ${packageName}`);
      fetchPackages();
    } catch (err) {
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

  const deviceColumns = [
    {
      title: 'Device ID',
      dataIndex: 'id',
      key: 'id',
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
              key: 'stop',
              icon: <StopOutlined />,
              label: 'Force Stop',
              onClick: () => handleForceStop(record.name)
            },
            {
              key: 'clear',
              icon: <ClearOutlined />,
              label: 'Clear Data',
              onClick: () => handleClearData(record.name)
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
              key: 'uninstall',
              icon: <DeleteOutlined />,
              label: 'Uninstall',
              danger: true,
              onClick: () => {
                 if (window.confirm(`Are you sure you want to uninstall ${record.name}?`)) {
                   handleUninstall(record.name);
                 }
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
                    <Option key={d.id} value={d.id}>{d.model || d.id}</Option>
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
      default:
        return <div style={{ padding: 24 }}>Select an option from the menu</div>;
    }
  };

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider collapsible collapsed={collapsed} onCollapse={setCollapsed}>
        <div className="logo" style={{ height: 32, margin: 16, background: 'rgba(255, 255, 255, 0.2)', textAlign: 'center', color: 'white', lineHeight: '32px', fontWeight: 'bold', borderRadius: 6 }}>
          ADB GUI
        </div>
        <Menu theme="dark" defaultSelectedKeys={['1']} mode="inline" selectedKeys={[selectedKey]} onClick={(e) => setSelectedKey(e.key)}>
          <Menu.Item key="1" icon={<MobileOutlined />}>
            Devices
          </Menu.Item>
          <Menu.Item key="2" icon={<AppstoreOutlined />}>
            Apps
          </Menu.Item>
          <Menu.Item key="3" icon={<CodeOutlined />}>
            Shell / Command
          </Menu.Item>
        </Menu>
      </Sider>
      <Layout className="site-layout">
        <Content style={{ margin: '0' }}>
          {renderContent()}
        </Content>
      </Layout>
    </Layout>
  );
}

export default App;
