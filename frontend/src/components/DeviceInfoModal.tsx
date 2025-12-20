import React, { useState } from "react";
import { Modal, Button, Descriptions, Card, Tabs, Table, Tag, Input } from "antd";
import { InfoCircleOutlined, ReloadOutlined, SearchOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
// @ts-ignore
import { main } from "../../wailsjs/go/models";

interface DeviceInfoModalProps {
  visible: boolean;
  onCancel: () => void;
  deviceInfo: main.DeviceInfo | null;
  loading: boolean;
  onRefresh: () => void;
}

const DeviceInfoModal: React.FC<DeviceInfoModalProps> = ({
  visible,
  onCancel,
  deviceInfo,
  loading,
  onRefresh,
}) => {
  const { t } = useTranslation();
  const [searchText, setSearchText] = useState("");

  const propColumns = [
    {
      title: t("device_info.prop"),
      dataIndex: "key",
      key: "key",
      width: "40%",
      render: (text: string) => <code style={{ fontSize: "11px" }}>{text}</code>,
    },
    {
      title: t("device_info.value"),
      dataIndex: "value",
      key: "value",
      render: (text: string) => <span style={{ fontSize: "12px" }}>{text}</span>,
    },
  ];

  const propData = deviceInfo?.props
    ? Object.entries(deviceInfo.props)
        .map(([key, value]) => ({
          key,
          value: String(value),
        }))
        .filter(item => 
          item.key.toLowerCase().includes(searchText.toLowerCase()) || 
          item.value.toLowerCase().includes(searchText.toLowerCase())
        )
        .sort((a, b) => a.key.localeCompare(b.key))
    : [];

  return (
    <Modal
      title={
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", paddingRight: "32px" }}>
          <span>
            <InfoCircleOutlined style={{ marginRight: 8, color: "#1890ff" }} />
            {t("device_info.title")}
          </span>
          <Button 
            size="small" 
            icon={<ReloadOutlined spin={loading} />} 
            onClick={onRefresh}
            disabled={loading}
          >
            {t("common.refresh")}
          </Button>
        </div>
      }
      open={visible}
      onCancel={onCancel}
      footer={[
        <Button key="close" onClick={onCancel}>
          {t("common.close")}
        </Button>,
      ]}
      width={700}
      bodyStyle={{ padding: "12px 24px 24px" }}
    >
      {loading && !deviceInfo ? (
        <div style={{ padding: "60px 0", textAlign: "center" }}>
          <ReloadOutlined spin style={{ fontSize: 32, color: "#1890ff", marginBottom: 16 }} />
          <div>{t("device_info.fetching")}</div>
        </div>
      ) : (
        deviceInfo && (
          <div className="selectable" style={{ userSelect: "text" }}>
            <Tabs defaultActiveKey="basic">
              <Tabs.TabPane tab={t("device_info.tabs.basic")} key="basic">
                <Card size="small" bordered={false}>
                  <Descriptions bordered column={1} size="small">
                    <Descriptions.Item label={t("device_info.model")}>{deviceInfo.model || "N/A"}</Descriptions.Item>
                    <Descriptions.Item label={t("device_info.brand")}>{deviceInfo.brand || "N/A"}</Descriptions.Item>
                    <Descriptions.Item label={t("device_info.manufacturer")}>{deviceInfo.manufacturer || "N/A"}</Descriptions.Item>
                    <Descriptions.Item label={t("device_info.android_ver")}>
                      <Tag color="blue">{deviceInfo.androidVer || "N/A"}</Tag>
                      (SDK {deviceInfo.sdk || "N/A"})
                    </Descriptions.Item>
                    <Descriptions.Item label={t("device_info.serial")}>{deviceInfo.serial || "N/A"}</Descriptions.Item>
                    <Descriptions.Item label={t("device_info.cpu")}>{deviceInfo.abi || "N/A"}</Descriptions.Item>
                    <Descriptions.Item label={t("device_info.hardware")}>{deviceInfo.cpu || "N/A"}</Descriptions.Item>
                    <Descriptions.Item label={t("device_info.memory")}>{deviceInfo.memory || "N/A"}</Descriptions.Item>
                    <Descriptions.Item label={t("device_info.resolution")}>{deviceInfo.resolution || "N/A"}</Descriptions.Item>
                    <Descriptions.Item label={t("device_info.density")}>{deviceInfo.density || "N/A"}</Descriptions.Item>
                  </Descriptions>
                </Card>
              </Tabs.TabPane>
              <Tabs.TabPane tab={t("device_info.tabs.all_props")} key="props">
                <div style={{ marginBottom: 16 }}>
                  <Input
                    placeholder={t("device_info.search_props")}
                    prefix={<SearchOutlined />}
                    value={searchText}
                    onChange={e => setSearchText(e.target.value)}
                    allowClear
                  />
                </div>
                <Table
                  dataSource={propData}
                  columns={propColumns}
                  size="small"
                  pagination={false}
                  scroll={{ y: 400 }}
                  rowKey="key"
                />
              </Tabs.TabPane>
            </Tabs>
          </div>
        )
      )}
    </Modal>
  );
};

export default DeviceInfoModal;

