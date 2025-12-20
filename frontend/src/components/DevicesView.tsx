import React from "react";
import { Table, Button, Tag, Space, Tooltip } from "antd";
import {
  ReloadOutlined,
  CodeOutlined,
  AppstoreOutlined,
  FileTextOutlined,
  FolderOutlined,
  DesktopOutlined,
} from "@ant-design/icons";

interface Device {
  id: string;
  state: string;
  model: string;
  brand: string;
}

interface DevicesViewProps {
  devices: Device[];
  loading: boolean;
  fetchDevices: () => Promise<void>;
  setSelectedKey: (key: string) => void;
  setSelectedDevice: (id: string) => void;
  setShellCmd: (cmd: string) => void;
  fetchFiles: (path: string) => Promise<void>;
  handleStartScrcpy: (id: string) => Promise<void>;
}

const DevicesView: React.FC<DevicesViewProps> = ({
  devices,
  loading,
  fetchDevices,
  setSelectedKey,
  setSelectedDevice,
  setShellCmd,
  fetchFiles,
  handleStartScrcpy,
}) => {
  const deviceColumns = [
    {
      title: "Device ID",
      dataIndex: "id",
      key: "id",
    },
    {
      title: "Brand",
      dataIndex: "brand",
      key: "brand",
      render: (brand: string) => (brand ? brand.toUpperCase() : "-"),
    },
    {
      title: "Model",
      dataIndex: "model",
      key: "model",
    },
    {
      title: "State",
      dataIndex: "state",
      key: "state",
      render: (state: string) => (
        <Tag color={state === "device" ? "green" : "red"}>
          {state.toUpperCase()}
        </Tag>
      ),
    },
    {
      title: "Action",
      key: "action",
      render: (_: any, record: Device) => (
        <Space size="middle">
          <Tooltip title="Shell">
            <Button
              size="small"
              icon={<CodeOutlined />}
              onClick={() => {
                setShellCmd(`-s ${record.id} shell ls -l`);
                setSelectedKey("3");
              }}
            />
          </Tooltip>
          <Tooltip title="Apps">
            <Button
              size="small"
              icon={<AppstoreOutlined />}
              onClick={() => {
                setSelectedDevice(record.id);
                setSelectedKey("2");
              }}
            />
          </Tooltip>
          <Tooltip title="Logcat">
            <Button
              size="small"
              icon={<FileTextOutlined />}
              onClick={() => {
                setSelectedDevice(record.id);
                setSelectedKey("4");
              }}
            />
          </Tooltip>
          <Tooltip title="Files">
            <Button
              size="small"
              icon={<FolderOutlined />}
              onClick={() => {
                setSelectedDevice(record.id);
                setSelectedKey("6");
                fetchFiles("/");
              }}
            />
          </Tooltip>
          <Tooltip title="Mirror Screen">
            <Button
              icon={<DesktopOutlined />}
              size="small"
              onClick={() => handleStartScrcpy(record.id)}
            />
          </Tooltip>
        </Space>
      ),
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
        <h2 style={{ margin: 0 }}>Connected Devices</h2>
        <Button
          icon={<ReloadOutlined />}
          onClick={fetchDevices}
          loading={loading}
        >
          Refresh
        </Button>
      </div>
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
          columns={deviceColumns}
          dataSource={devices}
          rowKey="id"
          loading={loading}
          pagination={false}
          size="small"
          scroll={{ y: "calc(100vh - 130px)" }}
          style={{ flex: 1 }}
        />
      </div>
    </div>
  );
};

export default DevicesView;

