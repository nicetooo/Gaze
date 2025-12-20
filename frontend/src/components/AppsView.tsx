import React from "react";
import { Table, Button, Tag, Space, Tooltip, Input, Radio, Dropdown, Modal } from "antd";
import {
  ReloadOutlined,
  PlayCircleOutlined,
  FileTextOutlined,
  FolderOpenOutlined,
  DownloadOutlined,
  InfoCircleOutlined,
  StopOutlined,
  CloseCircleOutlined,
  CheckCircleOutlined,
  ClearOutlined,
  DeleteOutlined,
  MoreOutlined,
} from "@ant-design/icons";
import DeviceSelector from "./DeviceSelector";
// @ts-ignore
import { main } from "../wailsjs/go/models";

interface Device {
  id: string;
  state: string;
  model: string;
  brand: string;
}

interface AppsViewProps {
  devices: Device[];
  selectedDevice: string;
  setSelectedDevice: (id: string) => void;
  fetchDevices: () => Promise<void>;
  loading: boolean;
  packages: main.AppPackage[];
  appsLoading: boolean;
  fetchPackages: (packageType?: string, deviceId?: string) => Promise<void>;
  packageFilter: string;
  setPackageFilter: (val: string) => void;
  typeFilter: string;
  setTypeFilter: (val: string) => void;
  handleStartApp: (packageName: string) => Promise<void>;
  handleAppLogcat: (packageName: string) => Promise<void>;
  handleExploreAppFiles: (packageName: string) => void;
  handleExportAPK: (packageName: string) => Promise<void>;
  handleFetchAppInfo: (packageName: string, force?: boolean) => Promise<void>;
  handleForceStop: (packageName: string) => Promise<void>;
  handleToggleState: (packageName: string, currentState: string) => Promise<void>;
  handleClearData: (packageName: string) => Promise<void>;
  handleUninstall: (packageName: string) => Promise<void>;
}

const AppsView: React.FC<AppsViewProps> = ({
  devices,
  selectedDevice,
  setSelectedDevice,
  fetchDevices,
  loading,
  packages,
  appsLoading,
  fetchPackages,
  packageFilter,
  setPackageFilter,
  typeFilter,
  setTypeFilter,
  handleStartApp,
  handleAppLogcat,
  handleExploreAppFiles,
  handleExportAPK,
  handleFetchAppInfo,
  handleForceStop,
  handleToggleState,
  handleClearData,
  handleUninstall,
}) => {
  const filteredPackages = packages.filter((p) => {
    const matchesName = p.name
      .toLowerCase()
      .includes(packageFilter.toLowerCase());
    const matchesType = typeFilter === "all" || p.type === typeFilter;
    return matchesName && matchesType;
  });

  const appColumns = [
    {
      title: "App",
      key: "app",
      render: (_: any, record: main.AppPackage) => {
        const firstLetter = (record.label || record.name)
          .charAt(0)
          .toUpperCase();
        const colors = [
          "#f56a00",
          "#7265e6",
          "#ffbf00",
          "#00a2ae",
          "#1890ff",
          "#52c41a",
          "#eb2f96",
          "#fadb14",
          "#fa541c",
          "#13c2c2",
        ];
        const color =
          colors[
            Math.abs(
              record.name
                .split("")
                .reduce((a, b) => (a << 5) - a + b.charCodeAt(0), 0)
            ) % colors.length
          ];

        return (
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <div
              style={{
                width: 36,
                height: 36,
                borderRadius: 8,
                backgroundColor: color,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                position: "relative",
                overflow: "hidden",
                flexShrink: 0,
                fontSize: "18px",
                fontWeight: "bold",
                color: "#fff",
                boxShadow: "0 2px 4px rgba(0,0,0,0.1)",
              }}
            >
              {record.icon ? (
                <img
                  src={record.icon}
                  style={{
                    width: "100%",
                    height: "100%",
                    objectFit: "cover",
                    position: "absolute",
                    zIndex: 2,
                  }}
                  onError={(e) => {
                    (e.target as HTMLImageElement).style.opacity = "0";
                  }}
                  alt=""
                />
              ) : (
                <img
                  src={`https://play-lh.googleusercontent.com/i-p/get-icon?id=${record.name}&w=72`}
                  style={{
                    width: "100%",
                    height: "100%",
                    objectFit: "cover",
                    position: "absolute",
                    zIndex: 2,
                  }}
                  onError={(e) => {
                    (e.target as HTMLImageElement).style.opacity = "0";
                  }}
                  alt=""
                />
              )}
              <span style={{ position: "relative", zIndex: 1 }}>
                {firstLetter}
              </span>
            </div>
            <div
              style={{
                display: "flex",
                flexDirection: "column",
                lineHeight: 1.1,
              }}
            >
              <span
                style={{ fontWeight: 600, fontSize: "14px", color: "#1a1a1a" }}
              >
                {record.label || record.name}
              </span>
              <span
                style={{
                  fontSize: "10px",
                  color: "#888",
                  fontFamily: "monospace",
                }}
              >
                {record.name}
              </span>
            </div>
          </div>
        );
      },
    },
    {
      title: "Type",
      dataIndex: "type",
      key: "type",
      width: 100,
      render: (type: string) => (
        <Tag color={type === "system" ? "orange" : "blue"}>
          {type === "system" ? "System" : "User"}
        </Tag>
      ),
    },
    {
      title: "State",
      dataIndex: "state",
      key: "state",
      width: 100,
      render: (state: string) => (
        <Tag color={state === "enabled" ? "green" : "red"}>
          {state.toUpperCase()}
        </Tag>
      ),
    },
    {
      title: "Action",
      key: "action",
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
            <Dropdown
              menu={{
                items: [
                  {
                    key: "stop",
                    icon: <StopOutlined />,
                    label: "Force Stop",
                    onClick: () => handleForceStop(record.name),
                  },
                  {
                    key: "state",
                    icon:
                      record.state === "enabled" ? (
                        <CloseCircleOutlined />
                      ) : (
                        <CheckCircleOutlined />
                      ),
                    label: record.state === "enabled" ? "Disable" : "Enable",
                    onClick: () => handleToggleState(record.name, record.state),
                  },
                  {
                    type: "divider",
                  },
                  {
                    key: "clear",
                    icon: <ClearOutlined />,
                    label: "Clear Data",
                    danger: true,
                    onClick: () => {
                      Modal.confirm({
                        title: "Clear App Data",
                        content: `Are you sure you want to clear all data for ${record.name}? This cannot be undone.`,
                        okText: "Clear",
                        okType: "danger",
                        cancelText: "Cancel",
                        onOk: () => handleClearData(record.name),
                      });
                    },
                  },
                  {
                    key: "uninstall",
                    icon: <DeleteOutlined />,
                    label: "Uninstall",
                    danger: true,
                    onClick: () => {
                      Modal.confirm({
                        title: "Uninstall App",
                        content: `Are you sure you want to uninstall ${record.name}?`,
                        okText: "Uninstall",
                        okType: "danger",
                        cancelText: "Cancel",
                        onOk: () => handleUninstall(record.name),
                      });
                    },
                  },
                ],
              }}
              trigger={["click"]}
            >
              <Button size="small" icon={<MoreOutlined />} />
            </Dropdown>
          </Space>
        );
      },
    },
  ];

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
        <h2 style={{ margin: 0 }}>Installed Apps</h2>
        <DeviceSelector
          devices={devices}
          selectedDevice={selectedDevice}
          onDeviceChange={setSelectedDevice}
          onRefresh={fetchDevices}
          loading={loading}
        />
      </div>
      <Space style={{ marginBottom: 12, flexShrink: 0 }}>
        <Input
          placeholder="Filter packages..."
          value={packageFilter}
          onChange={(e) => setPackageFilter(e.target.value)}
          style={{ width: 300 }}
        />
        <Radio.Group
          value={typeFilter}
          onChange={(e) => {
            const newType = e.target.value;
            setTypeFilter(newType);
            // Fetch packages when type changes
            if (newType === "all" || newType === "system") {
              fetchPackages(newType, selectedDevice);
            } else {
              // user - just filter existing packages
              fetchPackages("user", selectedDevice);
            }
          }}
        >
          <Radio.Button value="all">All</Radio.Button>
          <Radio.Button value="user">User</Radio.Button>
          <Radio.Button value="system">System</Radio.Button>
        </Radio.Group>
      </Space>
      <div
        className="selectable"
        style={{
          flex: 1,
          overflow: "hidden",
          backgroundColor: "#fff",
          borderRadius: "4px",
          border: "1px solid #f0f0f0",
          display: "flex",
          flexDirection: "column",
          userSelect: "text",
        }}
      >
        <Table
          columns={appColumns}
          dataSource={filteredPackages}
          rowKey="name"
          loading={appsLoading}
          pagination={false}
          size="small"
          scroll={{ y: "calc(100vh - 190px)" }}
          style={{ flex: 1 }}
        />
      </div>
    </div>
  );
};

export default AppsView;

