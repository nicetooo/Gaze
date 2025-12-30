import React from "react";
import { Table, Button, Tag, Space, Tooltip, theme } from "antd";
import { useTranslation } from "react-i18next";
import {
  ReloadOutlined,
  CodeOutlined,
  AppstoreOutlined,
  FileTextOutlined,
  FolderOutlined,
  DesktopOutlined,
  DownloadOutlined,
  InfoCircleOutlined,

  WifiOutlined,
  UsbOutlined,
  DisconnectOutlined,
  DeleteOutlined,
  LinkOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  StopOutlined,
  SettingOutlined,
  PushpinOutlined,
  PushpinFilled,
  DashboardOutlined,
  ArrowDownOutlined,
  ArrowUpOutlined,
} from "@ant-design/icons";
// @ts-ignore
import { StartNetworkMonitor, StopNetworkMonitor, StopAllNetworkMonitors } from "../../wailsjs/go/main/App";
// @ts-ignore
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

interface Device {
  id: string;
  serial: string;
  state: string;
  model: string;
  brand: string;
  type: string;
  ids: string[];
  wifiAddr: string;
  isPinned?: boolean;
}

interface HistoryDevice {
  id: string;
  serial: string;
  model: string;
  brand: string;
  type: string;
  wifiAddr: string;
  lastSeen: string;
  isPinned?: boolean;
}

interface DevicesViewProps {
  devices: Device[];
  historyDevices: HistoryDevice[];
  loading: boolean;
  fetchDevices: (silent?: boolean) => Promise<void>;
  setSelectedKey: (key: string) => void;
  setSelectedDevice: (id: string) => void;
  handleStartScrcpy: (id: string) => Promise<void>;
  handleFetchDeviceInfo: (id: string) => Promise<void>;
  onShowWirelessConnect: () => void;
  handleSwitchToWireless: (id: string) => Promise<void>;
  handleAdbDisconnect: (address: string) => Promise<void>;
  handleAdbConnect: (address: string) => Promise<void>;
  handleRemoveHistoryDevice: (id: string) => Promise<void>;
  handleOpenSettings: (id: string, action?: string, data?: string) => Promise<void>;
  handleTogglePin: (serial: string) => Promise<void>;
  busyDevices?: Set<string>;
  mirrorStatuses?: Record<string, { isMirroring: boolean; duration: number }>;
  recordStatuses?: Record<string, { isRecording: boolean; duration: number }>;
}


const DevicesView: React.FC<DevicesViewProps> = ({
  devices,
  historyDevices,
  loading,
  fetchDevices,
  setSelectedKey,
  setSelectedDevice,
  handleStartScrcpy,
  handleFetchDeviceInfo,
  onShowWirelessConnect,
  handleSwitchToWireless,
  handleAdbDisconnect,
  handleAdbConnect,
  handleRemoveHistoryDevice,
  handleOpenSettings,
  handleTogglePin,
  busyDevices = new Set(),
  mirrorStatuses = {},
  recordStatuses = {},
}) => {

  const { t } = useTranslation();
  const [netStatsMap, setNetStatsMap] = React.useState<Record<string, { rxSpeed: number; txSpeed: number }>>({});
  const monitoredIds = React.useRef<Set<string>>(new Set());

  React.useEffect(() => {
    // Subscribe to network stats updates
    const unregister = EventsOn("network-stats", (stats: any) => {
       if (stats.deviceId) {
         setNetStatsMap(prev => ({
           ...prev,
           [stats.deviceId]: { rxSpeed: stats.rxSpeed, txSpeed: stats.txSpeed }
         }));
       }
    });

    return () => {
      unregister();
      StopAllNetworkMonitors();
      monitoredIds.current.clear();
    };
  }, []);

  // Update monitors when devices list changes
  React.useEffect(() => {
    const currentOnlineIds = new Set<string>();
    
    // Start monitors for new devices
    devices.forEach(d => {
       if (d.state === 'device') {
         currentOnlineIds.add(d.id);
         if (!monitoredIds.current.has(d.id)) {
           StartNetworkMonitor(d.id);
           monitoredIds.current.add(d.id);
         }
       }
    });

    // Stop monitors for removed/offline devices
    monitoredIds.current.forEach(id => {
      if (!currentOnlineIds.has(id)) {
        StopNetworkMonitor(id);
        monitoredIds.current.delete(id);
      }
    });
  }, [devices]);

  const formatSpeed = (bytes: number) => {
      if (!bytes) return "0 B/s";
      if (bytes < 1024) return bytes + " B/s";
      if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB/s";
      return (bytes / (1024 * 1024)).toFixed(1) + " MB/s";
  };

  // Merge history devices that are not currently active
  const allDevices = devices.map(d => {
    // If active device is wired only, check history for a known wifiAddr
    if (d.type === "wired" && !d.wifiAddr) {
      const history = historyDevices.find(hd => hd.serial && hd.serial === d.serial);
      if (history && history.wifiAddr) {
        return { ...d, wifiAddr: history.wifiAddr };
      }
    }
    // Normalize model for display
    return { ...d, model: d.model?.replace(/_/g, " ") };
  });

  historyDevices.forEach(hd => {
    // Aggressive deduplication: check Serial, then exact ID, then check if ID exists in any active device's ID list or matches wifiAddr
    const isActive = devices.some(d => {
      if (hd.serial && d.serial === hd.serial) return true;
      if (d.id === hd.id) return true;
      if (d.ids && d.ids.includes(hd.id)) return true;
      // Compare by splitting ports (e.g. 192.168.0.4:5555 should match 192.168.0.4)
      const hdIP = hd.id?.split(':')[0];
      const dWifiIP = d.wifiAddr?.split(':')[0];
      if (hdIP && dWifiIP && hdIP === dWifiIP) return true;
      return false;
    });
    
    if (!isActive) {
      allDevices.push({
        id: hd.id,
        serial: hd.serial,
        state: "offline",
        model: hd.model?.replace(/_/g, " "),
        brand: hd.brand,
        type: hd.type,
        ids: [hd.id],
        wifiAddr: hd.wifiAddr || (hd.type === "wireless" ? hd.id : ""),
        isPinned: hd.isPinned,
      });
    }
  });

  const deviceColumns = [
    {
      title: t("devices.id"),
      dataIndex: "id",
      key: "id",
      width: 170,
      render: (_: string, record: Device) => {
        const displayId = record.serial || record.id;
        return (
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Tooltip title={record.isPinned ? t("devices.unpin") : t("devices.pin")}>
              <Button
                type="text"
                size="small"
                icon={record.isPinned ? <PushpinFilled style={{ color: '#1677ff' }} /> : <PushpinOutlined />}
                onClick={() => handleTogglePin(record.serial || record.id)}
                style={{ padding: 0, width: '20px' }}
              />
            </Tooltip>
            <div>
              <div style={{ fontWeight: record.state === 'device' ? 500 : 'normal' }}>
                {displayId}
              </div>
              {record.wifiAddr && record.wifiAddr !== displayId && (
                <div style={{ fontSize: '11px', color: '#8c8c8c', marginTop: '2px' }}>
                  {record.wifiAddr}
                </div>
              )}
            </div>
          </div>
        );
      }
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
      render: (model: string) => model || "-",
    },
    {
      title: t("devices.connection_type"),
      dataIndex: "type",
      key: "type",
      width: 100,
      render: (type: string, record: Device) => (
        <Space>
          {(type === "wired" || type === "both") && (
            <Tooltip title={t("devices.wired")}>
              <Tag 
                icon={<UsbOutlined />} 
                color={record.state === 'device' ? "orange" : "default"} 
                style={{ marginRight: 0, paddingInline: 8, opacity: record.state === 'device' ? 1 : 0.6 }} 
              />
            </Tooltip>
          )}
          {(type === "wireless" || type === "both") && (
            <Tooltip title={t("devices.wireless")}>
              <Tag 
                icon={<WifiOutlined />} 
                color={record.state === 'device' ? "blue" : "default"} 
                style={{ marginRight: 0, paddingInline: 8, opacity: record.state === 'device' ? 1 : 0.6 }} 
              />
            </Tooltip>
          )}
        </Space>
      ),
    },
    {
      title: t("devices.state"),
      dataIndex: "state",
      key: "state",
      width: 120,
      render: (state: string, record: Device) => {
        const isBusy = busyDevices.has(record.id) || busyDevices.has(record.serial);
        const mirrorStatus = mirrorStatuses[record.id] || mirrorStatuses[record.serial];
        const recordStatus = recordStatuses[record.id] || recordStatuses[record.serial];

        const config = isBusy 
          ? { color: "blue", icon: <ReloadOutlined spin />, text: t("common.loading") }
          : {
              device: { color: "green", icon: <CheckCircleOutlined />, text: t("devices.online") },
              offline: { color: "default", icon: <StopOutlined />, text: t("devices.offline") },
              unauthorized: { color: "red", icon: <CloseCircleOutlined />, text: t("devices.unauthorized") },
            }[state] || { color: "red", icon: <CloseCircleOutlined />, text: state };

        const formatDuration = (seconds: number) => {
          return new Date(seconds * 1000).toISOString().substr(11, 8);
        };

        return (
          <Space size={4}>
            <Tooltip title={config.text}>
              <Tag color={config.color} icon={config.icon} style={{ marginRight: 0, paddingInline: 8 }} />
            </Tooltip>
            {mirrorStatus?.isMirroring && (
              <Tooltip title={`${t("mirror.active_session")}: ${formatDuration(mirrorStatus.duration)}`}>
                <Tag color="cyan" icon={<DesktopOutlined />} style={{ marginRight: 0, paddingInline: 8 }} />
              </Tooltip>
            )}
            {recordStatus?.isRecording && (
              <Tooltip title={`${t("mirror.recording")}: ${formatDuration(recordStatus.duration)}`}>
                <Tag color="error" style={{ marginRight: 0, paddingInline: 8, display: 'inline-flex', alignItems: 'center' }}>
                  <div className="record-dot" style={{ margin: 0 }} />
                </Tag>
              </Tooltip>
            )}
          </Space>
        );
      },
    },
    {
      title: t("devices.network") || "Network",
      key: "network",
      width: 160,
      render: (_: any, record: Device) => {
        if (record.state !== 'device') return "-";
        
        const stats = netStatsMap[record.id] || { rxSpeed: 0, txSpeed: 0 };
        return (
          <Space direction="vertical" size={0} style={{ fontSize: '12px' }}>
            <div style={{ color: '#52c41a', display: 'flex', alignItems: 'center', gap: 4 }}>
              <ArrowDownOutlined style={{ fontSize: '10px' }} /> {formatSpeed(stats.rxSpeed)}
            </div>
            <div style={{ color: '#1890ff', display: 'flex', alignItems: 'center', gap: 4 }}>
              <ArrowUpOutlined style={{ fontSize: '10px' }} /> {formatSpeed(stats.txSpeed)}
            </div>
          </Space>
        );
      }
    },
    {
      title: t("devices.action"),
      key: "action",
      width: 320,
      render: (_: any, record: Device) => {
        const isBusy = busyDevices.has(record.id) || busyDevices.has(record.serial);
        return (
        <Space size="small">
          {(record.state === "device" || isBusy) ? (
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
                    setSelectedDevice(record.id);
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
              <Tooltip title={t("devices.system_settings")}>
                <Button
                  icon={<SettingOutlined />}
                  size="small"
                  onClick={() => handleOpenSettings(record.id)}
                />
              </Tooltip>
              {record.type === "wired" && (
                <Tooltip title={t("devices.connect_with_wireless")}>
                  <Button
                    size="small"
                    icon={<LinkOutlined />}
                    onClick={() => handleSwitchToWireless(record.id)}
                    style={{ color: "#1677ff" }}
                  />
                </Tooltip>
              )}
              {(record.type === "wireless" || record.type === "both") && (
                <Tooltip title={t("devices.disconnect_wireless")}>
                  <Button
                    size="small"
                    danger
                    icon={<DisconnectOutlined />}
                    onClick={() => {
                      const wirelessIds = record.ids.filter(id => id.includes(":") || id.startsWith("adb-"));
                      handleAdbDisconnect(wirelessIds.join(","));
                    }}
                  />
                </Tooltip>
              )}
            </>
          ) : (
            <>
              {record.wifiAddr && (
                <Tooltip title={t("devices.reconnect")}>
                  <Button
                    size="small"
                    type="primary"
                    icon={<LinkOutlined />}
                    onClick={() => handleAdbConnect(record.wifiAddr)}
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
        );
      },
    },
  ];

  const { token } = theme.useToken();

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
        <h2 style={{ margin: 0, color: token.colorText }}>{t("devices.title")}</h2>
        <Space>
          <Button icon={<WifiOutlined />} onClick={onShowWirelessConnect}>
            {t("devices.wireless_connect")}
          </Button>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => fetchDevices(false)}
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
          backgroundColor: token.colorBgContainer,
          borderRadius: token.borderRadius,
          border: `1px solid ${token.colorBorderSecondary}`,
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


