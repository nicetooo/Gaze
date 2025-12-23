import React, { useState, useEffect } from "react";
import { Modal, Form, Input, Tabs, Button, message, Space, Typography, QRCode } from "antd";
import { useTranslation } from "react-i18next";
import { WifiOutlined, LinkOutlined } from "@ant-design/icons";
// @ts-ignore
import { StartWirelessServer } from "../../wailsjs/go/main/App";
// @ts-ignore
const EventsOn = (window as any).runtime?.EventsOn;
// @ts-ignore
const EventsOff = (window as any).runtime?.EventsOff;

const { Text } = Typography;

interface WirelessConnectModalProps {
  visible: boolean;
  onCancel: () => void;
  onConnect: (address: string) => Promise<void>;
  onPair: (address: string, code: string) => Promise<void>;
}

const WirelessConnectModal: React.FC<WirelessConnectModalProps> = ({
  visible,
  onCancel,
  onConnect,
  onPair,
}) => {
  const { t } = useTranslation();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState("connect");
  const [connectUrl, setConnectUrl] = useState("");

  useEffect(() => {
    if (visible) {
      // Start the server and get the URL
      StartWirelessServer()
        .then((url: string) => setConnectUrl(url + "/c"))
        .catch((err: any) => message.error("Failed to start wireless server: " + err));

      // Listen for successful connection
      const handleConnected = (ip: string) => {
        message.success(t("app.connect_success") + ": " + ip);
        onCancel();
      };

      const handleConnectFailed = (data: { ip: string; error: string }) => {
        message.error({
          content: `${t("app.connect_failed")} (${data.ip}): ${data.error}`,
          duration: 10,
        });
      };

      if (EventsOn) {
        EventsOn("wireless-connected", handleConnected);
        EventsOn("wireless-connect-failed", handleConnectFailed);
      }

      return () => {
        if (EventsOff) {
          EventsOff("wireless-connected");
          EventsOff("wireless-connect-failed");
        }
      };
    }
  }, [visible]);

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      setLoading(true);
      if (activeTab === "connect") {
        await onConnect(values.address);
        message.success(t("app.connect_success"));
        onCancel();
      } else if (activeTab === "pair") {
        await onPair(values.address, values.pairCode);
        message.success(t("app.pairing_success"));
      }
    } catch (err) {
      // Error handled in parent
    } finally {
      setLoading(false);
    }
  };

  const items = [
    {
      key: "connect",
      label: (
        <span>
          <LinkOutlined />
          {t("devices.wireless_connect")}
        </span>
      ),
      children: (
        <Form form={form} layout="vertical">
          <Form.Item
            name="address"
            label={t("devices.ip_address")}
            rules={[{ required: true, message: t("devices.ip_address") + " " + t("common.required") }]}
          >
            <Input placeholder="192.168.1.100:5555" />
          </Form.Item>
          <Text type="secondary" style={{ fontSize: "12px" }}>
            {t("devices.wireless_connect_desc")}
          </Text>
        </Form>
      ),
    },
    {
      key: "pair",
      label: (
        <span>
          <WifiOutlined />
          {t("devices.pair_device")}
        </span>
      ),
      children: (
        <Form form={form} layout="vertical">
          <Form.Item
            name="address"
            label={t("devices.ip_address")}
            rules={[{ required: true, message: t("devices.ip_address") + " " + t("common.required") }]}
          >
            <Input placeholder="192.168.1.100:37891" />
          </Form.Item>
          <Form.Item
            name="pairCode"
            label={t("devices.pair_code")}
            rules={[{ required: true, message: t("devices.pair_code") + " " + t("common.required") }]}
          >
            <Input placeholder="123456" />
          </Form.Item>
          <Text type="secondary" style={{ fontSize: "12px" }}>
            {t("devices.pairing_guide")}
          </Text>
        </Form>
      ),
    },
  ];

  return (
    <Modal
      title={t("devices.wireless_connect")}
      open={visible}
      onCancel={onCancel}
      onOk={handleSubmit}
      confirmLoading={loading}
      destroyOnHidden
      width={400}
      centered
      styles={{ body: { maxHeight: "calc(80vh - 100px)", overflowY: "auto" } }}
    >
      <Tabs activeKey={activeTab} onChange={setActiveTab} items={items} />
      <div style={{
        marginTop: 16,
        padding: '10px 14px',
        background: 'rgba(250, 173, 20, 0.08)',
        borderRadius: 10,
        border: '1px solid rgba(250, 173, 20, 0.2)'
      }}>
        <Text type="warning" style={{ fontSize: '12px', fontWeight: 'bold', display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
          <WifiOutlined style={{ fontSize: '14px' }} /> {t("devices.proxy_tip_title")}
        </Text>
        <Text type="secondary" style={{ fontSize: '11px', lineHeight: '1.5', display: 'block' }}>
          {t("devices.proxy_tip_desc")}
        </Text>
      </div>
    </Modal>
  );
};

export default WirelessConnectModal;

