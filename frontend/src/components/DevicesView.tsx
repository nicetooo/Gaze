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
  WifiOutlined,
  UsbOutlined,
  DisconnectOutlined,
  DeleteOutlined,
  LinkOutlined,
} from "@ant-design/icons";

interface Device {
  id: string;
  state: string;
  model: string;
  brand: string;
  type: string;
}

interface HistoryDevice {
  id: string;
  model: string;
  brand: string;
  type: string;
  lastSeen: string;
}

interface DevicesViewProps {
  devices: Device[];
  historyDevices: HistoryDevice[];
  loading: boolean;
  fetchDevices: () => Promise<void>;
  setSelectedKey: (key: string) => void;
  setSelectedDevice: (id: string) => void;
  setShellCmd: (cmd: string) => void;
  fetchFiles: (path: string) => Promise<void>;
  handleStartScrcpy: (id: string) => Promise<void>;
  handleFetchDeviceInfo: (id: string) => Promise<void>;
  onShowWirelessConnect: () => void;
  handleSwitchToWireless: (id: string) => Promise<void>;
  handleAdbDisconnect: (address: string) => Promise<void>;
  handleAdbConnect: (address: string) => Promise<void>;
  handleRemoveHistoryDevice: (id: string) => Promise<void>;
}

const DevicesView: React.FC<DevicesViewProps> = ({
  devices,
  historyDevices,
  loading,
  fetchDevices,
  setSelectedKey,
  setSelectedDevice,
  setShellCmd,
  fetchFiles,
  handleStartScrcpy,
  handleFetchDeviceInfo,
  onShowWirelessConnect,
  handleSwitchToWireless,
  handleAdbDisconnect,
  handleAdbConnect,
  handleRemoveHistoryDevice,
}) => {
  const { t } = useTranslation();

  // Merge history devices that are not currently active
  const allDevices = [...devices];
  historyDevices.forEach(hd => {
    if (!devices.find(d => d.id === hd.id)) {
      allDevices.push({
        id: hd.id,
        state: "offline",
        model: hd.model,
        brand: hd.brand,
        type: hd.type
      });
    }
  });

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
      title: t("devices.connection_type"),
      dataIndex: "type",
      key: "type",
      width: 120,
      render: (type: string) => (
        <Space>
          {type === "wireless" ? (
            <Tag icon={<WifiOutlined />} color="blue">
              {t("devices.wireless")}
            </Tag>
          ) : (
            <Tag icon={<UsbOutlined />} color="orange">
              {t("devices.wired")}
            </Tag>
          )}
        </Space>
      ),
    },
    {
      title: t("devices.state"),
      dataIndex: "state",
      key: "state",
      width: 130,
      render: (state: string) => (
        <Tag color={state === "device" ? "green" : state === "offline" ? "default" : "red"}>
          {state === "device" ? t("devices.online") : state === "offline" ? t("devices.offline") : t("devices.unauthorized")}
        </Tag>
      ),
    },
    {
      title: t("devices.action"),
      key: "action",
      width: 320,
      render: (_: any, record: Device) => (
        <Space size="small">
          {record.state === "device" ? (
            <>
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
              {record.type === "wired" && (
                <Tooltip title={t("devices.switch_to_wireless")}>
                  <Button
                    size="small"
                    icon={<WifiOutlined />}
                    onClick={() => handleSwitchToWireless(record.id)}
                    style={{ color: "#1677ff" }}
                  />
                </Tooltip>
              )}
              {record.type === "wireless" && (
                <Tooltip title={t("devices.disconnect")}>
                  <Button
                    size="small"
                    danger
                    icon={<DisconnectOutlined />}
                    onClick={() => handleAdbDisconnect(record.id)}
                  />
                </Tooltip>
              )}
            </>
          ) : (
            <>
              {record.type === "wireless" && (
                <Tooltip title={t("devices.reconnect")}>
                  <Button
                    size="small"
                    type="primary"
                    icon={<LinkOutlined />}
                    onClick={() => handleAdbConnect(record.id)}
                  />
                </Tooltip>
              )}
              <Tooltip title={t("devices.remove_history")}>
                <Button
                  size="small"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={() => handleRemoveHistoryDevice(record.id)}
                />
              </Tooltip>
            </>
          )}
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
        <Space>
          <Button icon={<WifiOutlined />} onClick={onShowWirelessConnect}>
            {t("devices.wireless_connect")}
          </Button>
          <Button
            icon={<ReloadOutlined />}
            onClick={fetchDevices}
            loading={loading}
          >
            {t("common.refresh")}
          </Button>
        </Space>
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
          dataSource={allDevices}
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

