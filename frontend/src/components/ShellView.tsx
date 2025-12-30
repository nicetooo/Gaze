import React, { useState } from "react";
import { Button, Space, Input, message, theme } from "antd";
import { ClearOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import DeviceSelector from "./DeviceSelector";
// @ts-ignore
import { RunAdbCommand } from "../../wailsjs/go/main/App";

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
}

const ShellView: React.FC<ShellViewProps> = ({
  devices,
  selectedDevice,
  setSelectedDevice,
  fetchDevices,
  loading,
}) => {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const [shellCmd, setShellCmd] = useState("");
  const [shellOutput, setShellOutput] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);

  const presets = [
    { label: t("shell.presets.current_activity"), cmd: "shell dumpsys window | grep mCurrentFocus" },
    { label: t("shell.presets.battery_info"), cmd: "shell dumpsys battery" },
    { label: t("shell.presets.list_packages"), cmd: "shell pm list packages" },
    { label: t("shell.presets.device_info"), cmd: "shell getprop ro.build.version.release" },
    { label: t("shell.presets.screen_size"), cmd: "shell wm size" },
    { label: t("shell.presets.ip_address"), cmd: "shell ip addr show wlan0 | grep inet" },
  ];

  const handleShellCommand = async (command?: string) => {
    const cmdToRun = command || shellCmd;
    if (!cmdToRun) return;

    // Add to history if it's a new unique command
    setHistory((prev) => {
      const newHist = [cmdToRun, ...prev.filter((c) => c !== cmdToRun)].slice(0, 50);
      return newHist;
    });
    setHistoryIndex(-1);

    try {
      const res = await RunAdbCommand(selectedDevice, cmdToRun.trim());
      setShellOutput(res);
      // Keep the command in the input field so user can tweak it or see what was run
      // if (!command) setShellCmd(""); 
    } catch (err) {
      message.error(t("app.command_failed"));
      setShellOutput(String(err));
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowUp") {
      e.preventDefault();
      if (historyIndex < history.length - 1) {
        const newIndex = historyIndex + 1;
        setHistoryIndex(newIndex);
        setShellCmd(history[newIndex]);
      }
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      if (historyIndex > 0) {
        const newIndex = historyIndex - 1;
        setHistoryIndex(newIndex);
        setShellCmd(history[newIndex]);
      } else if (historyIndex === 0) {
        setHistoryIndex(-1);
        setShellCmd("");
      }
    }
  };

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
        <h2 style={{ margin: 0, color: token.colorText }}>{t("shell.title")}</h2>
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

      <div style={{ marginBottom: 12, display: "flex", flexWrap: "wrap", gap: 8 }}>
        {presets.map((p) => (
          <Button
            key={p.label}
            size="small"
            onClick={() => {
              setShellCmd(p.cmd);
              handleShellCommand(p.cmd);
            }}
          >
            {p.label}
          </Button>
        ))}
      </div>

      <Space.Compact style={{ width: "100%", marginBottom: 16 }}>
        <Input
          placeholder={t("shell.placeholder")}
          value={shellCmd}
          onChange={(e) => setShellCmd(e.target.value)}
          onPressEnter={() => handleShellCommand()}
          onKeyDown={handleKeyDown}
          autoFocus
        />
        <Button type="primary" onClick={() => handleShellCommand()}>
          {t("shell.run")}
        </Button>
      </Space.Compact>

      <Input.TextArea
        value={shellOutput}
        readOnly
        className="selectable"
        style={{
          fontFamily: "'Fira Code', 'Roboto Mono', monospace",
          backgroundColor: "#1e1e1e",
          color: "#d4d4d4",
          flex: 1,
          resize: "none",
          userSelect: "text",
          padding: "12px",
          borderRadius: "4px",
          fontSize: "13px",
          lineHeight: "1.5",
        }}
      />
    </div>
  );
};

export default ShellView;


