import React from "react";
import { Table, Button, Tag, Space, Tooltip } from "antd";
import { useTranslation } from "react-i18next";
import {
  ReloadOutlined,
  CodeOutlined,
  AppstoreOutlined,
  FileTextOutlined,
  FolderOutlined,
  DesktopOutlined,
  InfoCircleOutlined,
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
  handleFetchDeviceInfo: (id: string) => Promise<void>;
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
  handleFetchDeviceInfo,
}) => {
  const { t } = useTranslation();
  const deviceColumns = [
    {
      title: t("devices.id"),
      dataIndex: "id",
      key: "id",
    },
    {
      title: t("devices.brand"),
      dataIndex: "brand",
      key: "brand",
      render: (brand: string) => (brand ? brand.toUpperCase() : "-"),
    },
    {
      title: t("devices.model"),
      dataIndex: "model",
      key: "model",
    },
    {
      title: t("devices.state"),
      dataIndex: "state",
      key: "state",
      width: 130,
      render: (state: string) => (
        <Tag color={state === "device" ? "green" : "red"}>
          {state === "device" ? t("devices.online") : t("devices.offline")}
        </Tag>
      ),
    },
    {
      title: t("devices.action"),
      key: "action",
      width: 280,
      render: (_: any, record: Device) => (
        <Space size="small">
          <Tooltip title={t("device_info.title")}>
            <Button
              size="small"
              icon={<InfoCircleOutlined />}
              onClick={() => handleFetchDeviceInfo(record.id)}
            />
          </Tooltip>
          <Tooltip title={t("menu.shell")}>
            <Button
              size="small"
              icon={<CodeOutlined />}
              onClick={() => {
                setShellCmd(`-s ${record.id} shell ls -l`);
                setSelectedKey("3");
              }}
            />
          </Tooltip>
          <Tooltip title={t("menu.apps")}>
            <Button
              size="small"
              icon={<AppstoreOutlined />}
              onClick={() => {
                setSelectedDevice(record.id);
                setSelectedKey("2");
              }}
            />
          </Tooltip>
          <Tooltip title={t("menu.logcat")}>
            <Button
              size="small"
              icon={<FileTextOutlined />}
              onClick={() => {
                setSelectedDevice(record.id);
                setSelectedKey("4");
              }}
            />
          </Tooltip>
          <Tooltip title={t("menu.files")}>
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
          <Tooltip title={t("devices.mirror_screen")}>
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
        <h2 style={{ margin: 0 }}>{t("devices.title")}</h2>
        <Button
          icon={<ReloadOutlined />}
          onClick={fetchDevices}
          loading={loading}
        >
          {t("common.refresh")}
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

