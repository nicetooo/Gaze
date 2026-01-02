import React, { useState, useEffect, useMemo } from "react";
import {
  Modal,
  Button,
  Space,
  Input,
  Tree,
  Tag,
  Select,
  Tooltip,
  Empty,
  Divider,
  theme,
  message,
  Spin,
  Radio,
} from "antd";
import { useTranslation } from "react-i18next";
import {
  SearchOutlined,
  ReloadOutlined,
  ArrowDownOutlined,
  QuestionCircleOutlined,
  CheckCircleOutlined,
  AimOutlined,
  WarningOutlined,
  SelectOutlined,
} from "@ant-design/icons";
import { useDeviceStore, useAutomationStore } from "../stores";

export interface ElementSelector {
  type: "text" | "id" | "xpath" | "advanced" | "bounds";
  value: string;
  index?: number;
}

interface ElementPickerProps {
  visible: boolean;
  onSelect: (selector: ElementSelector) => void;
  onCancel: () => void;
}

// Generate best selector for an element
const generateSelector = (node: any): ElementSelector | null => {
  if (!node) return null;

  // Priority: ID > Text > XPath (using bounds as fallback)
  if (node.resourceId) {
    // Extract just the ID part after the slash
    const idPart = node.resourceId.includes("/")
      ? node.resourceId.split("/").pop()
      : node.resourceId;
    return { type: "id", value: idPart || node.resourceId };
  }

  if (node.text && node.text.trim()) {
    return { type: "text", value: node.text };
  }

  if (node.contentDesc && node.contentDesc.trim()) {
    return { type: "advanced", value: `desc:${node.contentDesc}` };
  }

  // Fallback to XPath with class and bounds
  if (node.class && node.bounds) {
    const className = node.class.split(".").pop();
    return {
      type: "xpath",
      value: `//${className}[@bounds='${node.bounds}']`,
    };
  }

  return null;
};

// Generate all possible selectors for an element
const generateAllSelectors = (node: any): ElementSelector[] => {
  const selectors: ElementSelector[] = [];

  if (node.resourceId) {
    const idPart = node.resourceId.includes("/")
      ? node.resourceId.split("/").pop()
      : node.resourceId;
    selectors.push({ type: "id", value: idPart || node.resourceId });
  }

  if (node.text && node.text.trim()) {
    selectors.push({ type: "text", value: node.text });
  }

  if (node.contentDesc && node.contentDesc.trim()) {
    selectors.push({ type: "advanced", value: `desc:${node.contentDesc}` });
  }

  if (node.class) {
    const className = node.class.split(".").pop();

    // XPath with text
    if (node.text) {
      selectors.push({
        type: "xpath",
        value: `//${className}[@text='${node.text}']`,
      });
    }

    // XPath with resource-id
    if (node.resourceId) {
      selectors.push({
        type: "xpath",
        value: `//${className}[contains(@resource-id,'${node.resourceId.split("/").pop()}')]`,
      });
    }

    // XPath with clickable (useful for buttons)
    if (node.clickable === "true") {
      if (node.text) {
        selectors.push({
          type: "xpath",
          value: `//${className}[@clickable='true' and @text='${node.text}']`,
        });
      }
    }
  }

  // Bounds selector (always available if bounds exist)
  if (node.bounds) {
    selectors.push({ type: "bounds", value: node.bounds });
  }

  return selectors;
};

const ElementPicker: React.FC<ElementPickerProps> = ({
  visible,
  onSelect,
  onCancel,
}) => {
  const { selectedDevice } = useDeviceStore();
  const { uiHierarchy, isFetchingHierarchy, fetchUIHierarchy } =
    useAutomationStore();

  const { t } = useTranslation();
  const { token } = theme.useToken();

  const [selectedNode, setSelectedNode] = useState<any>(null);
  const [searchText, setSearchText] = useState("");
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]);
  const [autoExpandParent, setAutoExpandParent] = useState(true);
  const [searchMode, setSearchMode] = useState<"auto" | "xpath" | "advanced">(
    "auto"
  );
  const [selectedSelectorIndex, setSelectedSelectorIndex] = useState(0);

  // Point picker state
  const [isPickingPoint, setIsPickingPoint] = useState(false);

  // Detect search mode from query
  const getEffectiveSearchMode = (query: string): string => {
    if (searchMode !== "auto") return searchMode;
    if (query.startsWith("//")) return "xpath";
    if (
      query.includes(":") ||
      query.includes("=") ||
      /\s+(AND|OR)\s+/i.test(query)
    )
      return "advanced";
    return "text";
  };

  // Helper to get node attribute by name
  const getNodeAttr = (node: any, attr: string): string => {
    const lowerAttr = attr.toLowerCase();
    switch (lowerAttr) {
      case "text":
        return node.text || "";
      case "resource-id":
      case "resourceid":
      case "id":
        return node.resourceId || "";
      case "class":
        return node.class || "";
      case "package":
        return node.package || "";
      case "content-desc":
      case "contentdesc":
      case "description":
      case "desc":
        return node.contentDesc || "";
      case "bounds":
        return node.bounds || "";
      case "clickable":
        return node.clickable || "";
      case "enabled":
        return node.enabled || "";
      case "focused":
        return node.focused || "";
      case "scrollable":
        return node.scrollable || "";
      default:
        return "";
    }
  };

  // Evaluate a single condition
  const evaluateCondition = (node: any, condition: string): boolean => {
    const operators = ["~", "^", "$", "=", ":"];
    let attr = "",
      op = "",
      value = "";

    for (const operator of operators) {
      const idx = condition.indexOf(operator);
      if (idx !== -1) {
        attr = condition.slice(0, idx).trim();
        op = operator;
        value = condition.slice(idx + 1).trim();
        break;
      }
    }

    if (!attr) {
      const lowerCond = condition.toLowerCase();
      return (
        (node.text || "").toLowerCase().includes(lowerCond) ||
        (node.contentDesc || "").toLowerCase().includes(lowerCond) ||
        (node.resourceId || "").toLowerCase().includes(lowerCond)
      );
    }

    const attrValue = getNodeAttr(node, attr).toLowerCase();
    const lowerValue = value.toLowerCase();

    switch (op) {
      case "=":
        return attrValue === lowerValue;
      case ":":
      case "~":
        return attrValue.includes(lowerValue);
      case "^":
        return attrValue.startsWith(lowerValue);
      case "$":
        return attrValue.endsWith(lowerValue);
      default:
        return false;
    }
  };

  // Match node against XPath query
  const matchXPath = (node: any, query: string): boolean => {
    if (!query.startsWith("//")) return false;
    const expr = query.slice(2);

    const bracketIdx = expr.indexOf("[");
    let className = bracketIdx !== -1 ? expr.slice(0, bracketIdx) : expr;
    let predicate =
      bracketIdx !== -1 ? expr.slice(bracketIdx + 1, -1) : "";

    if (className && className !== "node" && className !== "*") {
      const shortName = (node.class || "").split(".").pop();
      if (node.class !== className && shortName !== className) return false;
    }

    if (predicate) {
      const parts = predicate.split(/\s+and\s+/i);
      for (const part of parts) {
        const trimmed = part.trim();

        const containsMatch = trimmed.match(
          /contains\(@(\w+),\s*['"]([^'"]*)['"]\)/
        );
        if (containsMatch) {
          const attrValue = getNodeAttr(node, containsMatch[1]).toLowerCase();
          if (!attrValue.includes(containsMatch[2].toLowerCase())) return false;
          continue;
        }

        if (trimmed.startsWith("@")) {
          const attrPart = trimmed.slice(1);
          if (attrPart.includes("=")) {
            const [attr, val] = attrPart.split("=");
            const cleanVal = val.replace(/['"]/g, "");
            if (
              getNodeAttr(node, attr.trim()).toLowerCase() !==
              cleanVal.toLowerCase()
            )
              return false;
          } else {
            if (!getNodeAttr(node, attrPart)) return false;
          }
        }
      }
    }

    return true;
  };

  // Match node against advanced query
  const matchAdvanced = (node: any, query: string): boolean => {
    const orGroups = query.split(/\s+OR\s+/i);
    return orGroups.some((orGroup) => {
      const andParts = orGroup.split(/\s+AND\s+/i);
      return andParts.every((part) => evaluateCondition(node, part.trim()));
    });
  };

  // Process tree data
  const treeData = useMemo(() => {
    if (!uiHierarchy) return [];

    const processTreeData = (node: any, path: string = "0"): any => {
      if (!node) return null;

      const key = `${path}-${node.class}-${node.bounds}`;
      const children = (node.nodes || [])
        .map((child: any, i: number) => processTreeData(child, `${path}-${i}`))
        .filter(Boolean);

      const trimmedSearch = searchText.trim();
      let match = false;

      if (trimmedSearch) {
        const effectiveMode = getEffectiveSearchMode(trimmedSearch);
        if (effectiveMode === "xpath") {
          match = matchXPath(node, trimmedSearch);
        } else if (effectiveMode === "advanced") {
          match = matchAdvanced(node, trimmedSearch);
        } else {
          const lowerSearch = trimmedSearch.toLowerCase();
          match =
            (node.text || "").toLowerCase().includes(lowerSearch) ||
            (node.resourceId || "").toLowerCase().includes(lowerSearch) ||
            (node.class || "").toLowerCase().includes(lowerSearch) ||
            (node.contentDesc || "").toLowerCase().includes(lowerSearch);
        }
      }

      if (trimmedSearch && !match && children.length === 0) return null;

      return {
        key,
        title: (
          <Space size={4}>
            <span style={{ color: token.colorPrimary, fontSize: 11 }}>
              [{(node.class || "").split(".").pop()}]
            </span>
            {node.text && (
              <span
                style={{
                  fontWeight: 500,
                  backgroundColor: match ? token.colorWarningBg : "transparent",
                }}
              >
                "{node.text}"
              </span>
            )}
            {node.resourceId && (
              <span
                style={{
                  color: token.colorTextSecondary,
                  fontSize: 10,
                  backgroundColor:
                    match && !node.text ? token.colorWarningBg : "transparent",
                }}
              >
                #{node.resourceId.split("/").pop()}
              </span>
            )}
          </Space>
        ),
        data: node,
        children,
      };
    };

    const processed = processTreeData(uiHierarchy);
    return processed ? [processed] : [];
  }, [uiHierarchy, searchText, searchMode, token]);

  // Auto expand on search
  useEffect(() => {
    if (!uiHierarchy) return;

    if (searchText.trim()) {
      const keys: React.Key[] = [];
      const getAllKeys = (data: any[]) => {
        data.forEach((item) => {
          if (item && item.key) {
            keys.push(item.key);
            if (item.children) getAllKeys(item.children);
          }
        });
      };
      getAllKeys(treeData);
      setExpandedKeys(keys);
      setAutoExpandParent(true);
    } else if (expandedKeys.length === 0) {
      const keys: React.Key[] = [];
      const getInitialKeys = (data: any[], depth: number) => {
        if (depth > 2) return;
        data.forEach((item) => {
          if (item && item.key) {
            keys.push(item.key);
            if (item.children) getInitialKeys(item.children, depth + 1);
          }
        });
      };
      getInitialKeys(treeData, 0);
      setExpandedKeys(keys);
    }
  }, [searchText, treeData, uiHierarchy]);

  // Refresh hierarchy when modal opens
  useEffect(() => {
    if (visible && selectedDevice) {
      fetchUIHierarchy(selectedDevice);
      setSelectedNode(null);
      setSelectedSelectorIndex(0);
    }
  }, [visible, selectedDevice]);

  const availableSelectors = useMemo(() => {
    if (!selectedNode) return [];
    return generateAllSelectors(selectedNode);
  }, [selectedNode]);

  const handleConfirm = () => {
    if (!selectedNode) {
      message.warning(t("workflow.select_element_first"));
      return;
    }

    const selector = availableSelectors[selectedSelectorIndex];
    if (selector) {
      onSelect(selector);
    } else {
      const fallback = generateSelector(selectedNode);
      if (fallback) {
        onSelect(fallback);
      } else {
        message.error(t("workflow.no_valid_selector"));
      }
    }
  };

  // Pick point by tapping on device screen
  const handlePickPoint = async () => {
    if (!selectedDevice) {
      message.warning(t("app.select_device"));
      return;
    }

    setIsPickingPoint(true);
    message.loading({
      content: t("workflow.tap_on_device"),
      key: "point-picker",
      duration: 0,
    });

    try {
      const result = await (window as any).go.main.App.PickPointOnScreen(
        selectedDevice,
        30
      );

      if (result && result.bounds) {
        message.success({
          content: t("workflow.point_captured", { x: result.x, y: result.y }),
          key: "point-picker",
        });

        // Directly use the bounds as selector
        onSelect({
          type: "bounds",
          value: result.bounds,
        });
      }
    } catch (err) {
      message.error({
        content: t("workflow.point_capture_failed") + ": " + String(err),
        key: "point-picker",
      });
    } finally {
      setIsPickingPoint(false);
    }
  };

  const searchHelpContent = (
    <div style={{ maxWidth: 320 }}>
      <div style={{ marginBottom: 8 }}>
        <strong>{t("automation.search_modes")}</strong>
      </div>
      <div style={{ marginBottom: 12 }}>
        <div style={{ color: token.colorPrimary, marginBottom: 4 }}>
          {t("automation.xpath_mode")}
        </div>
        <code style={{ fontSize: 11, display: "block", marginBottom: 2 }}>
          //Button[@text='OK']
        </code>
        <code style={{ fontSize: 11, display: "block" }}>
          //EditText[@clickable='true']
        </code>
      </div>
      <div>
        <div style={{ color: token.colorPrimary, marginBottom: 4 }}>
          {t("automation.advanced_mode")}
        </div>
        <code style={{ fontSize: 11, display: "block", marginBottom: 2 }}>
          clickable:true AND text:OK
        </code>
        <code style={{ fontSize: 11, display: "block" }}>id~login</code>
      </div>
    </div>
  );

  return (
    <Modal
      title={
        <Space>
          <AimOutlined />
          {t("workflow.element_picker")}
        </Space>
      }
      open={visible}
      onCancel={onCancel}
      width={900}
      bodyStyle={{ padding: 0 }}
      footer={
        <Space>
          <Button onClick={onCancel}>{t("common.cancel")}</Button>
          <Button
            type="primary"
            onClick={handleConfirm}
            disabled={!selectedNode}
            icon={<CheckCircleOutlined />}
          >
            {t("workflow.use_selector")}
          </Button>
        </Space>
      }
    >
      <div style={{ display: "flex", height: 500 }}>
        {/* Left: Tree */}
        <div
          style={{
            flex: 1,
            borderRight: `1px solid ${token.colorBorderSecondary}`,
            display: "flex",
            flexDirection: "column",
          }}
        >
          {/* Point picker for WebView/Hybrid pages */}
          <div
            style={{
              padding: 12,
              borderBottom: `1px solid ${token.colorBorderSecondary}`,
              backgroundColor: token.colorFillAlter,
            }}
          >
            <Button
              type="dashed"
              icon={<SelectOutlined />}
              onClick={handlePickPoint}
              loading={isPickingPoint}
              disabled={!selectedDevice}
              block
            >
              {isPickingPoint
                ? t("workflow.waiting_for_tap")
                : t("workflow.pick_point_on_screen")}
            </Button>
            <div
              style={{
                fontSize: 11,
                color: token.colorTextSecondary,
                marginTop: 6,
                textAlign: "center",
              }}
            >
              {t("workflow.pick_point_hint")}
            </div>
          </div>

          {/* Search bar */}
          <div
            style={{
              padding: 12,
              borderBottom: `1px solid ${token.colorBorderSecondary}`,
              display: "flex",
              gap: 8,
            }}
          >
            <Select
              value={searchMode}
              onChange={setSearchMode}
              size="small"
              style={{ width: 90 }}
              options={[
                { label: t("automation.search_auto"), value: "auto" },
                { label: "XPath", value: "xpath" },
                { label: t("automation.search_advanced"), value: "advanced" },
              ]}
            />
            <Input
              placeholder={
                searchMode === "xpath"
                  ? "//Button[@text='OK']"
                  : searchMode === "advanced"
                    ? "clickable:true AND text:OK"
                    : t("common.search")
              }
              prefix={<SearchOutlined />}
              suffix={
                <Tooltip title={searchHelpContent} placement="bottomRight">
                  <QuestionCircleOutlined
                    style={{ color: token.colorTextSecondary, cursor: "help" }}
                  />
                </Tooltip>
              }
              size="small"
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              style={{ flex: 1 }}
              allowClear
            />
            <Button
              icon={<ReloadOutlined />}
              size="small"
              loading={isFetchingHierarchy}
              onClick={() => selectedDevice && fetchUIHierarchy(selectedDevice)}
            />
          </div>

          {/* Tree */}
          <div style={{ flex: 1, overflow: "auto", padding: 12 }}>
            {isFetchingHierarchy ? (
              <div
                style={{
                  display: "flex",
                  justifyContent: "center",
                  alignItems: "center",
                  height: "100%",
                }}
              >
                <Spin tip={t("common.loading")} />
              </div>
            ) : uiHierarchy ? (
              <Tree
                showLine
                switcherIcon={<ArrowDownOutlined style={{ fontSize: 10 }} />}
                onSelect={(_, info: any) => {
                  const node = info.node.data;
                  setSelectedNode(node);
                  setSelectedSelectorIndex(0);
                }}
                selectedKeys={selectedNode ? [selectedNode.key] : []}
                treeData={treeData}
                expandedKeys={expandedKeys}
                autoExpandParent={autoExpandParent}
                onExpand={(keys) => {
                  setExpandedKeys(keys);
                  setAutoExpandParent(false);
                }}
              />
            ) : (
              <Empty
                description={
                  selectedDevice
                    ? t("automation.no_hierarchy_info")
                    : t("app.select_device")
                }
                style={{ marginTop: 100 }}
              />
            )}
          </div>
        </div>

        {/* Right: Element details and selector options */}
        <div
          style={{
            width: 350,
            overflow: "auto",
            padding: 16,
            backgroundColor: token.colorFillAlter,
          }}
        >
          {selectedNode ? (
            <div>
              <h4
                style={{
                  marginBottom: 12,
                  borderBottom: `1px solid ${token.colorBorder}`,
                  paddingBottom: 8,
                }}
              >
                {t("automation.element_details")}
              </h4>

              {/* Element info */}
              <div style={{ marginBottom: 16 }}>
                {selectedNode.text && (
                  <div style={{ marginBottom: 8 }}>
                    <Tag color="blue">Text</Tag>
                    <span style={{ fontSize: 12 }}>{selectedNode.text}</span>
                  </div>
                )}
                {selectedNode.resourceId && (
                  <div style={{ marginBottom: 8 }}>
                    <Tag color="green">ID</Tag>
                    <span style={{ fontSize: 12 }}>
                      {selectedNode.resourceId.split("/").pop()}
                    </span>
                  </div>
                )}
                {selectedNode.contentDesc && (
                  <div style={{ marginBottom: 8 }}>
                    <Tag color="orange">Desc</Tag>
                    <span style={{ fontSize: 12 }}>
                      {selectedNode.contentDesc}
                    </span>
                  </div>
                )}
                <div style={{ marginBottom: 8 }}>
                  <Tag>Class</Tag>
                  <span style={{ fontSize: 12 }}>
                    {(selectedNode.class || "").split(".").pop()}
                  </span>
                </div>
                <div>
                  <Tag>Bounds</Tag>
                  <span style={{ fontSize: 12 }}>{selectedNode.bounds}</span>
                </div>
              </div>

              <Divider style={{ margin: "12px 0" }} />

              {/* Selector options */}
              <h4
                style={{
                  marginBottom: 12,
                  borderBottom: `1px solid ${token.colorBorder}`,
                  paddingBottom: 8,
                }}
              >
                {t("workflow.choose_selector")}
              </h4>

              {availableSelectors.length > 0 ? (
                <Radio.Group
                  value={selectedSelectorIndex}
                  onChange={(e) => setSelectedSelectorIndex(e.target.value)}
                  style={{ width: "100%" }}
                >
                  <Space direction="vertical" style={{ width: "100%" }}>
                    {availableSelectors.map((selector, index) => (
                      <Radio
                        key={index}
                        value={index}
                        style={{
                          display: "block",
                          padding: "8px 12px",
                          backgroundColor:
                            selectedSelectorIndex === index
                              ? token.colorPrimaryBg
                              : token.colorBgContainer,
                          borderRadius: 6,
                          border: `1px solid ${
                            selectedSelectorIndex === index
                              ? token.colorPrimary
                              : token.colorBorder
                          }`,
                          marginRight: 0,
                        }}
                      >
                        <div>
                          <Tag
                            color={
                              selector.type === "id"
                                ? "green"
                                : selector.type === "text"
                                  ? "blue"
                                  : selector.type === "xpath"
                                    ? "purple"
                                    : selector.type === "bounds"
                                      ? "cyan"
                                      : "orange"
                            }
                            style={{ marginRight: 8 }}
                          >
                            {selector.type.toUpperCase()}
                          </Tag>
                          {selector.type === "bounds" && (
                            <Tooltip title={t("workflow.bounds_warning")}>
                              <WarningOutlined style={{ color: token.colorWarning, marginRight: 4 }} />
                            </Tooltip>
                          )}
                          <span
                            style={{
                              fontSize: 12,
                              wordBreak: "break-all",
                              color: token.colorText,
                            }}
                          >
                            {selector.value}
                          </span>
                        </div>
                      </Radio>
                    ))}
                  </Space>
                </Radio.Group>
              ) : (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={t("workflow.no_selectors")}
                />
              )}

              {/* Preview selected selector */}
              {availableSelectors.length > 0 && (
                <div
                  style={{
                    marginTop: 16,
                    padding: 12,
                    backgroundColor: token.colorBgContainer,
                    borderRadius: 6,
                    border: `1px solid ${token.colorBorder}`,
                  }}
                >
                  <div
                    style={{
                      fontSize: 11,
                      color: token.colorTextSecondary,
                      marginBottom: 4,
                    }}
                  >
                    {t("workflow.selected_selector")}
                  </div>
                  <code
                    style={{
                      fontSize: 12,
                      wordBreak: "break-all",
                      color: token.colorPrimary,
                    }}
                  >
                    {availableSelectors[selectedSelectorIndex]?.type}:{" "}
                    {availableSelectors[selectedSelectorIndex]?.value}
                  </code>
                </div>
              )}
            </div>
          ) : (
            <div
              style={{
                textAlign: "center",
                color: token.colorTextDescription,
                marginTop: 100,
              }}
            >
              <AimOutlined style={{ fontSize: 48, marginBottom: 16 }} />
              <div>{t("workflow.pick_element_tip")}</div>
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
};

export default ElementPicker;
