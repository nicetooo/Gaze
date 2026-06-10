import React, { useState, useEffect } from "react";
import {
  Modal,
  Form,
  Input,
  Select,
  Switch,
  Space,
  Typography,
  Tabs,
  Button,
  App,
  Alert,
} from "antd";

const { TabPane } = Tabs;
const { TextArea } = Input;
import { SaveOutlined } from "@ant-design/icons";
import { usePluginStore, Plugin } from "../stores/pluginStore";
import { useTranslation } from "react-i18next";
import MonacoPluginEditor from "./MonacoPluginEditor";
import * as ts from "typescript";

const { Title, Text } = Typography;

interface PluginEditorProps {
  open: boolean;
  plugin: Plugin | null;
  onClose: () => void;
}

const PluginEditor: React.FC<PluginEditorProps> = ({
  open,
  plugin,
  onClose,
}) => {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const { savePlugin, loading, editorInitialTab } = usePluginStore();
  const [sourceCode, setSourceCode] = useState("");
  const [compiledCode, setCompiledCode] = useState("");
  const [compileError, setCompileError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState("basic");
  const [configCode, setConfigCode] = useState("");

  useEffect(() => {
    if (open) {
      // 使用 store 中设置的初始 tab
      setActiveTab(editorInitialTab);
    }
    
    if (plugin) {
      form.setFieldsValue({
        id: plugin.metadata.id,
        name: plugin.metadata.name,
        version: plugin.metadata.version,
        author: plugin.metadata.author,
        description: plugin.metadata.description,
        enabled: plugin.metadata.enabled,
        sources: plugin.metadata.filters?.sources || [],
        types: plugin.metadata.filters?.types || [],
        urlPattern: plugin.metadata.filters?.urlPattern || "",
      });
      setSourceCode(plugin.sourceCode);
      
      // 设置 config 代码（如果有的话）
      if (plugin.metadata.config && Object.keys(plugin.metadata.config).length > 0) {
        setConfigCode(JSON.stringify(plugin.metadata.config, null, 2));
      } else {
        setConfigCode("");
      }
    } else {
      // 新建插件时的默认代码模板
      const defaultCode = `// Plugin metadata is managed by the form above (Basic Info & Filters tabs)
// This code only defines the event processing logic

const plugin: Plugin = {
  // Called for each event matching your filters
  onEvent: (event: UnifiedEvent, context: PluginContext): PluginResult => {
    // Access plugin configuration
    // const config = context.config;
    
    // Log messages (visible in console)
    context.log("Processing event: " + event.id);
    
    // Your logic here
    // - event: the matched event object
    // - context.emit(): emit new derived events
    // - context.state: persistent state storage
    
    return {
      derivedEvents: [],  // New events to emit
      tags: [],           // Tags to add to the event
      metadata: {}        // Additional metadata
    };
  },
  
  // Optional: Called when plugin is loaded (initialization)
  // onInit: (context: PluginContext): void => {
  //   context.log("Plugin initialized");
  //   context.state.counter = 0;
  // },
  
  // Optional: Called when plugin is unloaded (cleanup)
  // onDestroy: (context: PluginContext): void => {
  //   context.log("Plugin destroyed");
  // }
};`;
      setSourceCode(defaultCode);
      
      // 默认 config 模板（带完整实例和注释说明）
      const defaultConfig = `{
  // ========== Performance Monitoring ==========
  
  // Slow request threshold in milliseconds
  // Usage: if (duration > context.config.slowThresholdMs) { ... }
  "slowThresholdMs": 2000,
  
  // High CPU usage threshold (percentage)
  "highCpuThreshold": 80,
  
  // Memory warning threshold in MB
  "memoryWarningMb": 512,
  
  
  // ========== API Validation ==========
  
  // Required parameters for tracking API calls
  // Usage: const required = context.config.requiredParams || [];
  "requiredParams": ["user_id", "event_name", "timestamp"],
  
  // Allowed HTTP methods
  "allowedMethods": ["GET", "POST", "PUT", "DELETE"],
  
  // API endpoint patterns to monitor
  "monitoredEndpoints": [
    "*/api/track*",
    "*/api/analytics*",
    "*/api/events*"
  ],
  
  
  // ========== Error Detection ==========
  
  // Enable automatic error detection
  "autoDetectErrors": true,
  
  // Keywords to identify errors in logs
  "errorKeywords": ["ERROR", "FATAL", "Exception", "crash"],
  
  // Regex pattern for error messages (optional)
  // "errorPattern": "^ERROR:\\s+(.+)$",
  
  
  // ========== Notification Settings ==========
  
  // Enable notifications for critical events
  "enableNotifications": true,
  
  // Minimum event level for notifications (info/warn/error/fatal)
  "notificationLevel": "error",
  
  // Maximum notifications per minute (rate limiting)
  "maxNotificationsPerMinute": 5,
  
  
  // ========== Custom Rules ==========
  
  // Custom business rules configuration
  "customRules": {
    "validateUserSession": true,
    "checkDataIntegrity": true,
    "enforceRateLimits": false
  },
  
  // Timeout for async operations (milliseconds)
  "timeout": 5000,
  
  // Enable debug logging
  "debugMode": false
  
  // Add your custom configuration here...
}`;
      setConfigCode(defaultConfig);
      
      form.resetFields();
    }
  }, [plugin, form, open, editorInitialTab]);

  // 自动编译 TypeScript 代码
  useEffect(() => {
    if (sourceCode) {
      try {
        const result = ts.transpileModule(sourceCode, {
          compilerOptions: {
            target: ts.ScriptTarget.ES2020,
            module: ts.ModuleKind.CommonJS,
            removeComments: false,
            strict: false,
          },
        });
        setCompiledCode(result.outputText);
        setCompileError(null);
      } catch (error) {
        setCompileError(
          error instanceof Error ? error.message : "Compilation failed"
        );
        setCompiledCode("");
      }
    }
  }, [sourceCode]);

  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      
      // 解析 config JSON（支持 JSON with Comments）
      let config = {};
      if (configCode && configCode.trim()) {
        try {
          // 使用简单的正则移除 JSON 注释（单行 // 和多行 /* */）
          const jsonWithoutComments = configCode
            .replace(/\/\*[\s\S]*?\*\//g, '') // 移除 /* ... */
            .replace(/\/\/.*/g, '');           // 移除 // ...
          
          const parsed = JSON.parse(jsonWithoutComments);
          config = parsed;
        } catch (e) {
          message.error(t("plugins.config_invalid_json"));
          setActiveTab("config"); // 切换到 Config tab 显示错误
          return;
        }
      }
      
      // 构建插件对象
      const pluginData: Partial<Plugin> = {
        metadata: {
          id: values.id,
          name: values.name,
          version: values.version || "1.0.0",
          author: values.author || "",
          description: values.description || "",
          enabled: values.enabled ?? true,
          filters: {
            sources: values.sources || [],
            types: values.types || [],
            levels: [],
            urlPattern: values.urlPattern || "",
            titleMatch: "",
          },
          config,
          createdAt: plugin?.metadata.createdAt || new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        sourceCode,
        language: "typescript",
        compiledCode: compiledCode || sourceCode, // 使用编译后的代码
      };

      await savePlugin(pluginData);
      message.success(t("plugins.saved_success"));
      onClose();
    } catch (error: any) {
      console.error("Failed to save plugin:", error);
      
      // 如果是表单验证错误
      if (error.errorFields && error.errorFields.length > 0) {
        // 切换到基础信息 tab
        setActiveTab("basic");
        
        // 显示具体的验证错误
        const firstError = error.errorFields[0];
        const fieldName = firstError.name[0];
        const errorMessage = firstError.errors[0];
        
        message.error(`${t("plugins.validation_failed")}: ${errorMessage}`);
      } else {
        // 其他保存错误
        message.error(t("plugins.save_failed"));
      }
    }
  };

  return (
    <Modal
      title={plugin ? t("plugins.edit_plugin") : t("plugins.create_plugin")}
      open={open}
      onCancel={onClose}
      width={900}
      footer={[
        <Button key="cancel" onClick={onClose}>
          {t("common.cancel")}
        </Button>,
        <Button
          key="save"
          type="primary"
          icon={<SaveOutlined />}
          loading={loading}
          onClick={handleSave}
        >
          {t("common.save")}
        </Button>,
      ]}
    >
      {/* 编译错误提示 */}
      {compileError && (
        <Alert
          message={t("plugins.compile_error")}
          description={compileError}
          type="error"
          closable
          onClose={() => setCompileError(null)}
          style={{ marginBottom: 16 }}
        />
      )}
      
      <Tabs activeKey={activeTab} onChange={setActiveTab}>
        <TabPane tab={t("plugins.basic_info")} key="basic">
          <Form form={form} layout="vertical">
            <Form.Item
              name="id"
              label={t("plugins.id")}
              rules={[
                { required: true, message: t("plugins.id_required") },
                {
                  pattern: /^[a-z0-9-]+$/,
                  message: t("plugins.id_format"),
                },
              ]}
            >
              <Input
                placeholder="my-plugin"
                disabled={!!plugin}
              />
            </Form.Item>

            <Form.Item
              name="name"
              label={t("plugins.name")}
              rules={[{ required: true, message: t("plugins.name_required") }]}
            >
              <Input placeholder={t("plugins.name_placeholder")} />
            </Form.Item>

            <Form.Item name="version" label={t("plugins.version")}>
              <Input placeholder="1.0.0" />
            </Form.Item>

            <Form.Item name="author" label={t("plugins.author")}>
              <Input placeholder={t("plugins.author_placeholder")} />
            </Form.Item>

            <Form.Item name="description" label={t("plugins.description")}>
              <TextArea
                rows={3}
                placeholder={t("plugins.description_placeholder")}
              />
            </Form.Item>

            <Form.Item
              name="enabled"
              label={t("plugins.enabled")}
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Form>
        </TabPane>

        <TabPane tab={t("plugins.filters")} key="filters">
          <Form form={form} layout="vertical">
            <Form.Item name="sources" label={t("plugins.event_sources")}>
              <Select
                mode="tags"
                placeholder={t("plugins.sources_placeholder")}
                options={[
                  { label: "network", value: "network" },
                  { label: "logcat", value: "logcat" },
                  { label: "app", value: "app" },
                  { label: "device", value: "device" },
                  { label: "touch", value: "touch" },
                  { label: "workflow", value: "workflow" },
                ]}
              />
            </Form.Item>

            <Form.Item name="types" label={t("plugins.event_types")}>
              <Select
                mode="tags"
                placeholder={t("plugins.types_placeholder")}
                options={[
                  { label: "network_request", value: "network_request" },
                  { label: "websocket_message", value: "websocket_message" },
                  { label: "logcat", value: "logcat" },
                  { label: "app_crash", value: "app_crash" },
                  { label: "app_anr", value: "app_anr" },
                ]}
              />
            </Form.Item>

            <Form.Item name="urlPattern" label={t("plugins.url_pattern")}>
              <Input
                placeholder="*/api/track*"
                addonBefore={
                  <Text type="secondary" style={{ fontSize: "12px" }}>
                    {t("plugins.wildcard")}
                  </Text>
                }
              />
            </Form.Item>
          </Form>
        </TabPane>

        <TabPane tab={t("plugins.config")} key="config">
          <Space direction="vertical" style={{ width: "100%" }} size="small">
            <Text type="secondary" style={{ fontSize: "12px" }}>
              {t("plugins.config_description")}
            </Text>
            
            <MonacoPluginEditor
              value={configCode}
              onChange={setConfigCode}
              language="json"
              height="400px"
            />
            
            <Text type="secondary" style={{ fontSize: "11px" }}>
              {t("plugins.config_note")}
            </Text>
          </Space>
        </TabPane>

        <TabPane tab={t("plugins.code")} key="code">
          <Space direction="vertical" style={{ width: "100%" }} size="middle">
            {/* Compilation Error */}
            {compileError && (
              <Alert
                message={t("plugins.compile_error")}
                description={compileError}
                type="error"
                showIcon
                closable
                onClose={() => setCompileError(null)}
              />
            )}

            {/* Monaco Editor */}
            <MonacoPluginEditor
              value={sourceCode}
              onChange={setSourceCode}
              language="typescript"
              height="500px"
            />

            {/* Helper Functions Info */}
            <Text type="secondary" style={{ fontSize: "12px" }}>
              {t("plugins.intellisense_hint")}
            </Text>
          </Space>
        </TabPane>
      </Tabs>
    </Modal>
  );
};

export default PluginEditor;
