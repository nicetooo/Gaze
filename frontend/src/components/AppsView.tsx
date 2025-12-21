import React, { useState, useEffect, useRef } from "react";
import {
  Table,
  Button,
  Space,
  Tooltip,
  Input,
  Radio,
  message,
  Modal,
  Tag,
  Dropdown,
} from "antd";
import { useTranslation } from "react-i18next";
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
  SettingOutlined,
} from "@ant-design/icons";
import DeviceSelector from "./DeviceSelector";
import AppInfoModal from "./AppInfoModal";
// @ts-ignore
import { GetAppInfo, StartActivity } from "../../wailsjs/go/main/App";
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
  handleForceStop: (packageName: string) => Promise<void>;
  handleToggleState: (packageName: string, currentState: string) => Promise<void>;
  handleClearData: (packageName: string) => Promise<void>;
  handleUninstall: (packageName: string) => Promise<void>;
  handleOpenSettings: (deviceId: string, action?: string, data?: string) => Promise<void>;
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
  handleForceStop,
  handleToggleState,
  handleClearData,
  handleUninstall,
  handleOpenSettings,
}) => {
  const { t } = useTranslation();
  const containerRef = useRef<HTMLDivElement>(null);
  const [tableHeight, setTableHeight] = useState<number>(400);

  // App Info state
  const [infoModalVisible, setInfoModalVisible] = React.useState(false);
  const [infoLoading, setInfoLoading] = React.useState(false);
  const [selectedAppInfo, setSelectedAppInfo] = React.useState<main.AppPackage | null>(null);
  const [permissionSearch, setPermissionSearch] = React.useState("");
  const [activitySearch, setActivitySearch] = React.useState("");

  const handleFetchAppInfo = async (packageName: string, force: boolean = false) => {
    if (!selectedDevice) return;

    if (!force) {
      setSelectedAppInfo(packages.find((p) => p.name === packageName) || null);
    }

    setPermissionSearch("");
    setInfoModalVisible(true);
    setInfoLoading(true);
    try {
      const res = await GetAppInfo(selectedDevice, packageName, force);
      setSelectedAppInfo(res);
    } catch (err) {
      message.error(t("app.fetch_app_info_failed") + ": " + String(err));
    } finally {
      setInfoLoading(false);
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

  useEffect(() => {
    const updateHeight = () => {
      if (containerRef.current) {
        // 精确计算：容器总高度 - (顶部标题栏约72px + 过滤栏约44px + 边距约30px)
        const offset = 160; 
        const height = containerRef.current.clientHeight - offset;
        setTableHeight(height > 200 ? height : 400);
      }
    };

    updateHeight();
    const timer = setTimeout(updateHeight, 100); // 延迟执行一次确保布局已完成
    window.addEventListener("resize", updateHeight);
    return () => {
      window.removeEventListener("resize", updateHeight);
      clearTimeout(timer);
    };
  }, []);

  const filteredPackages = packages.filter((p) => {
    const matchesName = p.name
      .toLowerCase()
      .includes(packageFilter.toLowerCase());
    const matchesType = typeFilter === "all" || p.type === typeFilter;
    return matchesName && matchesType;
  });

  const appColumns = [
    {
      title: t("apps.title"),
      key: "app",
      render: (_: any, record: main.AppPackage) => {
        const firstLetter = (record.label || record.name)
          .charAt(0)
          .toUpperCase();
        const colors = [
          "#f56a00", "#7265e6", "#ffbf00", "#00a2ae", "#1890ff",
          "#52c41a", "#eb2f96", "#fadb14", "#fa541c", "#13c2c2",
        ];
        const color = colors[
            Math.abs(record.name.split("").reduce((a: number, b: string) => (a << 5) - a + b.charCodeAt(0), 0)) % colors.length
          ];

        return (
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <div
              style={{
                width: 36, height: 36, borderRadius: 8, backgroundColor: color,
                display: "flex", alignItems: "center", justifyContent: "center",
                position: "relative", overflow: "hidden", flexShrink: 0,
                fontSize: "18px", fontWeight: "bold", color: "#fff",
                boxShadow: "0 2px 4px rgba(0,0,0,0.1)",
              }}
            >
              {record.icon ? (
                <img
                  src={record.icon}
                  style={{ width: "100%", height: "100%", objectFit: "cover", position: "absolute", zIndex: 2 }}
                  onError={(e) => { (e.target as HTMLImageElement).style.opacity = "0"; }}
                  alt=""
                />
              ) : (
                <img
                  src={`https://play-lh.googleusercontent.com/i-p/get-icon?id=${record.name}&w=72`}
                  style={{ width: "100%", height: "100%", objectFit: "cover", position: "absolute", zIndex: 2 }}
                  onError={(e) => { (e.target as HTMLImageElement).style.opacity = "0"; }}
                  alt=""
                />
              )}
              <span style={{ position: "relative", zIndex: 1 }}>{firstLetter}</span>
            </div>
            <div style={{ display: "flex", flexDirection: "column", lineHeight: 1.1 }}>
              <span style={{ fontWeight: 600, fontSize: "14px", color: "#1a1a1a" }}>
                {record.label || record.name}
              </span>
              <span style={{ fontSize: "10px", color: "#888", fontFamily: "monospace" }}>
                {record.name}
              </span>
            </div>
          </div>
        );
      },
    },
    {
      title: t("apps.type"),
      dataIndex: "type",
      key: "type",
      width: 100,
      render: (type: string) => (
        <Tag color={type === "system" ? "orange" : "blue"}>
          {type === "system" ? t("apps.system") : t("apps.user")}
        </Tag>
      ),
    },
    {
      title: t("apps.state"),
      dataIndex: "state",
      key: "state",
      width: 100,
      render: (state: string) => (
        <Tag color={state === "enabled" ? "green" : "red"}>
          {state === "enabled" ? t("apps.enable").toUpperCase() : t("apps.disable").toUpperCase()}
        </Tag>
      ),
    },
    {
      title: t("apps.action"),
      key: "action",
      width: 380,
      render: (_: any, record: main.AppPackage) => {
        return (
          <Space size={4}>
            <Tooltip title={t("apps.launch_app")}>
              <Button size="small" icon={<PlayCircleOutlined />} onClick={() => handleStartApp(record.name)} />
            </Tooltip>
            <Tooltip title={t("apps.force_stop")}>
              <Button size="small" icon={<StopOutlined />} onClick={() => handleForceStop(record.name)} />
            </Tooltip>
            <Tooltip title={record.state === "enabled" ? t("apps.disable") : t("apps.enable")}>
              <Button 
                size="small" 
                icon={record.state === "enabled" ? <CloseCircleOutlined /> : <CheckCircleOutlined />} 
                onClick={() => handleToggleState(record.name, record.state)} 
              />
            </Tooltip>
            <Tooltip title={t("apps.app_settings")}>
              <Button size="small" icon={<SettingOutlined />} onClick={() => handleOpenSettings(selectedDevice, "android.settings.APPLICATION_DETAILS_SETTINGS", `package:${record.name}`)} />
            </Tooltip>
            <Tooltip title={t("menu.logcat")}>
              <Button size="small" icon={<FileTextOutlined />} onClick={() => handleAppLogcat(record.name)} />
            </Tooltip>
            <Tooltip title={t("apps.explore_files")}>
              <Button size="small" icon={<FolderOpenOutlined />} onClick={() => handleExploreAppFiles(record.name)} />
            </Tooltip>
            <Tooltip title={t("apps.export")}>
              <Button size="small" icon={<DownloadOutlined />} onClick={() => handleExportAPK(record.name)} />
            </Tooltip>
            <Tooltip title={t("app_info.title")}>
              <Button size="small" icon={<InfoCircleOutlined />} onClick={() => handleFetchAppInfo(record.name)} />
            </Tooltip>
            <Dropdown
              menu={{
                items: [
                  {
                    key: "clear",
                    icon: <ClearOutlined />,
                    label: t("apps.clear_data"),
                    danger: true,
                    onClick: () => {
                      Modal.confirm({
                        title: t("apps.clear_data_confirm_title"),
                        content: t("apps.clear_data_confirm_content", { name: record.name }),
                        okText: t("apps.clear_data"),
                        okType: "danger",
                        cancelText: t("common.cancel"),
                        onOk: () => handleClearData(record.name),
                      });
                    },
                  },
                  {
                    key: "uninstall",
                    icon: <DeleteOutlined />,
                    label: t("apps.uninstall"),
                    danger: true,
                    onClick: () => {
                      Modal.confirm({
                        title: t("apps.uninstall_confirm_title"),
                        content: t("apps.uninstall_confirm_content", { name: record.name }),
                        okText: t("apps.uninstall"),
                        okType: "danger",
                        cancelText: t("common.cancel"),
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
      ref={containerRef}
      style={{
        display: "flex",
        flexDirection: "column",
        height: "100%",
        position: "relative",
        padding: "16px 24px",
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
        <h2 style={{ margin: 0 }}>{t("apps.title")}</h2>
        <DeviceSelector
          devices={devices}
          selectedDevice={selectedDevice}
          onDeviceChange={setSelectedDevice}
          onRefresh={fetchDevices}
          loading={loading}
        />
      </div>
      <Space style={{ marginBottom: 16, flexShrink: 0 }}>
        <Input
          placeholder={t("apps.filter_placeholder")}
          value={packageFilter}
          onChange={(e) => setPackageFilter(e.target.value)}
          style={{ width: 300 }}
        />
        <Radio.Group
          value={typeFilter}
          onChange={(e) => {
            const newType = e.target.value;
            setTypeFilter(newType);
            fetchPackages(newType, selectedDevice);
          }}
        >
          <Radio.Button value="all">{t("apps.all")}</Radio.Button>
          <Radio.Button value="user">{t("apps.user")}</Radio.Button>
          <Radio.Button value="system">{t("apps.system")}</Radio.Button>
        </Radio.Group>
      </Space>
      <div
        className="selectable"
        style={{
          flex: 1,
          overflow: "hidden",
          backgroundColor: "#fff",
          borderRadius: "8px",
          border: "1px solid #f0f0f0",
          display: "flex",
          flexDirection: "column",
          userSelect: "text",
          boxShadow: "0 2px 8px rgba(0,0,0,0.05)",
        }}
      >
        <Table
          columns={appColumns}
          dataSource={filteredPackages}
          rowKey="name"
          loading={appsLoading}
          pagination={false}
          size="small"
          scroll={{ y: tableHeight }}
          style={{ flex: 1 }}
        />
      </div>
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
        getContainer={() => containerRef.current || document.body}
      />
    </div>
  );
};

export default AppsView;
