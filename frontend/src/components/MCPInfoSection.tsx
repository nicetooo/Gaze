import React from "react";
import { Collapse, Typography, Tag, Space, Table, Button, message } from "antd";
import { useTranslation } from "react-i18next";
import CodeBlock from "./CodeBlock";
import {
  ApiOutlined,
  CopyOutlined,
  MobileOutlined,
  AppstoreOutlined,
  CameraOutlined,
  AimOutlined,
  DatabaseOutlined,
  BranchesOutlined,
  GlobalOutlined,
  VideoCameraOutlined,
  CheckOutlined,
  CheckSquareOutlined,
  CodeOutlined,
  LineChartOutlined,
  PlayCircleOutlined,
} from "@ant-design/icons";

const { Text, Paragraph } = Typography;

// Tool definitions organized by category
const TOOL_CATEGORIES = [
  {
    key: "device",
    icon: <MobileOutlined />,
    color: "#1677ff",
    tools: [
      "device_list", "device_info", "device_connect", "device_disconnect",
      "device_pair", "device_wireless", "device_ip",
      "adb_execute", "aapt_execute", "ffmpeg_execute", "ffprobe_execute",
      "file_upload", "file_list",
    ],
  },
  {
    key: "apps",
    icon: <AppstoreOutlined />,
    color: "#52c41a",
    tools: [
      "app_list", "app_info", "app_start", "app_stop",
      "app_running", "app_install", "app_uninstall", "app_clear_data",
    ],
  },
  {
    key: "screen",
    icon: <CameraOutlined />,
    color: "#fa8c16",
    tools: [
      "screen_screenshot", "screen_record_start", "screen_record_stop", "screen_recording_status",
    ],
  },
  {
    key: "automation",
    icon: <AimOutlined />,
    color: "#722ed1",
    tools: [
      "ui_hierarchy", "ui_search", "ui_tap", "ui_swipe", "ui_input", "ui_wait_for", "ui_resolution", "keyboard_setup",
    ],
  },
  {
    key: "recording",
    icon: <PlayCircleOutlined />,
    color: "#ff4d4f",
    tools: [
      "touch_record_start", "touch_record_stop", "touch_record_status",
      "touch_script_list", "touch_script_play", "touch_script_save", "touch_script_delete",
      "touch_playback_stop",
    ],
  },
  {
    key: "session",
    icon: <DatabaseOutlined />,
    color: "#13c2c2",
    tools: [
      "session_create", "session_end", "session_active", "session_list",
      "session_events", "session_stats", "session_export", "session_import",
    ],
  },
  {
    key: "workflow",
    icon: <BranchesOutlined />,
    color: "#eb2f96",
    tools: [
      "workflow_list", "workflow_get", "workflow_create", "workflow_update",
      "workflow_delete", "workflow_run", "workflow_stop", "workflow_pause",
      "workflow_resume", "workflow_step_next", "workflow_status", "workflow_execute_step",
    ],
  },
  {
    key: "proxy",
    icon: <GlobalOutlined />,
    color: "#faad14",
    tools: [
      "proxy_start", "proxy_stop", "proxy_status", "proxy_configure", "proxy_settings",
      "proxy_device_setup", "proxy_device_cleanup", "proxy_cert_install", "proxy_cert_trust_check",
      "mock_rule_list", "mock_rule_add", "mock_rule_update", "mock_rule_remove", "mock_rule_toggle",
      "mock_rule_export", "mock_rule_import",
      "map_remote_add", "map_remote_update", "map_remote_remove", "map_remote_list", "map_remote_toggle",
      "rewrite_rule_add", "rewrite_rule_update", "rewrite_rule_remove", "rewrite_rule_list", "rewrite_rule_toggle",
      "resend_request",
      "breakpoint_rule_add", "breakpoint_rule_update", "breakpoint_rule_remove", "breakpoint_rule_list", "breakpoint_rule_toggle",
      "breakpoint_resolve", "breakpoint_pending_list", "breakpoint_forward_all",
    ],
  },
  {
    key: "video",
    icon: <VideoCameraOutlined />,
    color: "#f5222d",
    tools: ["video_frame", "video_metadata", "session_video_frame", "session_video_info"],
  },
  {
    key: "perf",
    icon: <LineChartOutlined />,
    color: "#2f54eb",
    tools: ["perf_start", "perf_stop", "perf_snapshot", "perf_process_detail"],
  },
  {
    key: "proto",
    icon: <CodeOutlined />,
    color: "#9254de",
    tools: [
      "proto_file_list", "proto_file_add", "proto_file_update", "proto_file_remove",
      "proto_mapping_list", "proto_mapping_add", "proto_mapping_update", "proto_mapping_remove",
      "proto_message_types", "proto_load_url",
    ],
  },
  {
    key: "assertions",
    icon: <CheckSquareOutlined />,
    color: "#389e0d",
    tools: [
      "assertion_list", "assertion_create", "assertion_get",
      "assertion_update", "assertion_delete", "assertion_execute",
      "assertion_quick_no_errors", "assertion_quick_no_crashes",
      "assertion_set_create", "assertion_set_update", "assertion_set_delete",
      "assertion_set_get", "assertion_set_list",
      "assertion_set_execute", "assertion_set_results", "assertion_set_result",
    ],
  },
  {
    key: "plugins",
    icon: <ApiOutlined />,
    color: "#d4380d",
    tools: [
      "plugin_list", "plugin_get", "plugin_create", "plugin_update",
      "plugin_delete", "plugin_toggle", "plugin_test", "plugin_test_detailed",
      "plugin_test_custom", "plugin_test_batch", "plugin_sample_events",
    ],
  },
];

const RESOURCES = [
  { uri: "gaze://devices", key: "devices" },
  { uri: "gaze://devices/{deviceId}", key: "device_info" },
  { uri: "gaze://sessions", key: "sessions" },
  { uri: "workflow://list", key: "workflow_list" },
  { uri: "workflow://{workflowId}", key: "workflow_detail" },
];

const TOTAL_TOOLS = TOOL_CATEGORIES.reduce((sum, cat) => sum + cat.tools.length, 0);

const CLAUDE_DESKTOP_CONFIG = `{
  "mcpServers": {
    "gaze": {
      "command": "/Applications/Gaze.app/Contents/MacOS/Gaze",
      "args": ["--mcp"]
    }
  }
}`;

const CLAUDE_CODE_COMMAND = `claude mcp add gaze -- /Applications/Gaze.app/Contents/MacOS/Gaze --mcp`;

const CURSOR_CONFIG = `{
  "mcpServers": {
    "gaze": {
      "command": "/Applications/Gaze.app/Contents/MacOS/Gaze",
      "args": ["--mcp"]
    }
  }
}`;

const MCPInfoSection: React.FC = () => {
  const { t } = useTranslation();
  const [copiedKey, setCopiedKey] = React.useState<string | null>(null);

  const handleCopy = (text: string, key: string) => {
    navigator.clipboard.writeText(text).then(() => {
      setCopiedKey(key);
      message.success(t("mcp.copied"));
      setTimeout(() => setCopiedKey(null), 2000);
    });
  };

  const toolColumns = [
    {
      title: "Tool",
      dataIndex: "tool",
      key: "tool",
      width: 200,
      render: (name: string) => (
        <Text code style={{ fontSize: 12 }}>{name}</Text>
      ),
    },
    {
      title: "Description",
      dataIndex: "description",
      key: "description",
      render: (desc: string) => <Text style={{ fontSize: 13 }}>{desc}</Text>,
    },
  ];

  const resourceColumns = [
    {
      title: "URI",
      dataIndex: "uri",
      key: "uri",
      width: 260,
      render: (uri: string) => (
        <Text code style={{ fontSize: 12 }}>{uri}</Text>
      ),
    },
    {
      title: "Description",
      dataIndex: "description",
      key: "description",
      render: (desc: string) => <Text style={{ fontSize: 13 }}>{desc}</Text>,
    },
  ];

  return (
    <div>
      {/* Overview */}
      <Paragraph type="secondary" style={{ marginBottom: 16, fontSize: 13 }}>
        {t("mcp.description")}
      </Paragraph>

      {/* Setup Configurations */}
      <Collapse
        size="small"
        style={{ marginBottom: 16 }}
        items={[
          {
            key: "setup",
            label: (
              <Space>
                <CodeOutlined />
                <Text strong style={{ fontSize: 13 }}>{t("mcp.setup_title")}</Text>
              </Space>
            ),
            children: (
              <div>
                <Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 12 }}>
                  {t("mcp.setup_desc")}
                </Paragraph>

                {/* Claude Desktop */}
                <div style={{ marginBottom: 16 }}>
                  <Text strong style={{ fontSize: 13 }}>{t("mcp.setup_claude_title")}</Text>
                  <Paragraph type="secondary" style={{ fontSize: 12, margin: "4px 0 8px" }}>
                    {t("mcp.setup_claude_desc")}
                  </Paragraph>
                  <div style={{ position: "relative" }}>
                    <Button
                      type="text"
                      size="small"
                      icon={copiedKey === "claude" ? <CheckOutlined style={{ color: "#52c41a" }} /> : <CopyOutlined />}
                      onClick={() => handleCopy(CLAUDE_DESKTOP_CONFIG, "claude")}
                      style={{ position: "absolute", top: 4, right: 12, zIndex: 10 }}
                    />
                    <CodeBlock code={CLAUDE_DESKTOP_CONFIG} language="json" />
                  </div>
                </div>

                {/* Claude Code */}
                <div style={{ marginBottom: 16 }}>
                  <Text strong style={{ fontSize: 13 }}>{t("mcp.setup_claude_code_title")}</Text>
                  <Paragraph type="secondary" style={{ fontSize: 12, margin: "4px 0 8px" }}>
                    {t("mcp.setup_claude_code_desc")}
                  </Paragraph>
                  <div style={{ position: "relative" }}>
                    <Button
                      type="text"
                      size="small"
                      icon={copiedKey === "claude_code" ? <CheckOutlined style={{ color: "#52c41a" }} /> : <CopyOutlined />}
                      onClick={() => handleCopy(CLAUDE_CODE_COMMAND, "claude_code")}
                      style={{ position: "absolute", top: 4, right: 12, zIndex: 10 }}
                    />
                    <CodeBlock code={CLAUDE_CODE_COMMAND} language="shell" />
                  </div>
                </div>

                {/* Cursor */}
                <div style={{ marginBottom: 16 }}>
                  <Text strong style={{ fontSize: 13 }}>{t("mcp.setup_cursor_title")}</Text>
                  <Paragraph type="secondary" style={{ fontSize: 12, margin: "4px 0 8px" }}>
                    {t("mcp.setup_cursor_desc")}
                  </Paragraph>
                  <div style={{ position: "relative" }}>
                    <Button
                      type="text"
                      size="small"
                      icon={copiedKey === "cursor" ? <CheckOutlined style={{ color: "#52c41a" }} /> : <CopyOutlined />}
                      onClick={() => handleCopy(CURSOR_CONFIG, "cursor")}
                      style={{ position: "absolute", top: 4, right: 12, zIndex: 10 }}
                    />
                    <CodeBlock code={CURSOR_CONFIG} language="json" />
                  </div>
                </div>

                <Paragraph type="warning" style={{ fontSize: 12, marginBottom: 0 }}>
                  {t("mcp.binary_path_note")}
                </Paragraph>
              </div>
            ),
          },
        ]}
      />

      {/* Tools by Category */}
      <Collapse
        size="small"
        style={{ marginBottom: 16 }}
        items={[
          {
            key: "tools",
            label: (
              <Space>
                <ApiOutlined />
                <Text strong style={{ fontSize: 13 }}>
                  {t("mcp.tools_title", { count: TOTAL_TOOLS })}
                </Text>
              </Space>
            ),
            children: (
              <div>
                <Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 12 }}>
                  {t("mcp.tools_desc")}
                </Paragraph>

                <Collapse
                  size="small"
                  ghost
                  items={TOOL_CATEGORIES.map((cat) => ({
                    key: cat.key,
                    label: (
                      <Space>
                        <span style={{ color: cat.color }}>{cat.icon}</span>
                        <Text strong style={{ fontSize: 13 }}>
                          {t(`mcp.category.${cat.key}`)}
                        </Text>
                        <Tag>{cat.tools.length}</Tag>
                        <Text type="secondary" style={{ fontSize: 12 }}>
                          {t(`mcp.category.${cat.key}_desc`)}
                        </Text>
                      </Space>
                    ),
                    children: (
                      <Table
                        size="small"
                        columns={toolColumns}
                        dataSource={cat.tools.map((tool) => ({
                          key: tool,
                          tool,
                          description: t(`mcp.tool.${tool}`),
                        }))}
                        pagination={false}
                        showHeader={false}
                        style={{ marginTop: -8 }}
                      />
                    ),
                  }))}
                />
              </div>
            ),
          },
        ]}
      />

      {/* Resources */}
      <Collapse
        size="small"
        items={[
          {
            key: "resources",
            label: (
              <Space>
                <DatabaseOutlined />
                <Text strong style={{ fontSize: 13 }}>
                  {t("mcp.resources_title")}
                </Text>
                <Tag>{RESOURCES.length}</Tag>
              </Space>
            ),
            children: (
              <div>
                <Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 12 }}>
                  {t("mcp.resources_desc")}
                </Paragraph>
                <Table
                  size="small"
                  columns={resourceColumns}
                  dataSource={RESOURCES.map((r) => ({
                    key: r.uri,
                    uri: r.uri,
                    description: t(`mcp.resource.${r.key}`),
                  }))}
                  pagination={false}
                />
              </div>
            ),
          },
        ]}
      />
    </div>
  );
};

export default MCPInfoSection;
