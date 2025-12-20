import React from "react";
import { Button, Space, Input } from "antd";
import { ClearOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import DeviceSelector from "./DeviceSelector";

interface Device {
  id: string;
  state: string;
  model: string;
  brand: string;
}

interface ShellViewProps {
  devices: Device[];
  selectedDevice: string;
  setSelectedDevice: (id: string) => void;
  fetchDevices: () => Promise<void>;
  loading: boolean;
  shellCmd: string;
  setShellCmd: (val: string) => void;
  shellOutput: string;
  setShellOutput: (val: string) => void;
  handleShellCommand: () => Promise<void>;
}

const ShellView: React.FC<ShellViewProps> = ({
  devices,
  selectedDevice,
  setSelectedDevice,
  fetchDevices,
  loading,
  shellCmd,
  setShellCmd,
  shellOutput,
  setShellOutput,
  handleShellCommand,
}) => {
  const { t } = useTranslation();
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
        <h2 style={{ margin: 0 }}>{t("shell.title")}</h2>
        <Space>
          <DeviceSelector
            devices={devices}
            selectedDevice={selectedDevice}
            onDeviceChange={setSelectedDevice}
            onRefresh={fetchDevices}
            loading={loading}
          />
          <Button icon={<ClearOutlined />} onClick={() => setShellOutput("")}>
            {t("common.clear") || "Clear"}
          </Button>
        </Space>
      </div>
      <Space.Compact style={{ width: "100%", marginBottom: 16 }}>
        <Input
          placeholder={t("shell.placeholder")}
          value={shellCmd}
          onChange={(e) => setShellCmd(e.target.value)}
          onPressEnter={handleShellCommand}
        />
        <Button type="primary" onClick={handleShellCommand}>
          {t("shell.run")}
        </Button>
      </Space.Compact>
      <Input.TextArea
        rows={15}
        value={shellOutput}
        readOnly
        className="selectable"
        style={{
          fontFamily: "monospace",
          backgroundColor: "#fff",
          flex: 1,
          resize: "none",
          userSelect: "text",
        }}
      />
    </div>
  );
};

export default ShellView;

