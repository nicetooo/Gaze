import React, { useState, useEffect } from "react";
import {
    Button,
    Space,
    Card,
    Input,
    message,
    theme,
    Empty,
    Tooltip,
    Divider,
    Tree,
    Select,
} from "antd";
import { useTranslation } from "react-i18next";
import {
    SearchOutlined,
    BlockOutlined,
    ReloadOutlined,
    ArrowDownOutlined,
    InfoCircleOutlined,
    CodeOutlined,
    PlayCircleOutlined,
    CopyOutlined,
    QuestionCircleOutlined,
} from "@ant-design/icons";
import { useDeviceStore, useAutomationStore } from "../stores";

const DetailItem: React.FC<{
    label: string;
    value: string | boolean;
    copyable?: boolean;
}> = ({ label, value, copyable }) => {
    const { token } = theme.useToken();
    const valStr = String(value ?? "");
    const isFalsey = value === null || value === undefined || value === "" || valStr === "false";

    // Don't hide Clickable/Long Clickable rows just because they are false
    const isActionRow = label === "Clickable" || label === "Long Clickable";
    if (isFalsey && !isActionRow) return null;

    return (
        <div style={{ marginBottom: 10 }}>
            <div style={{ color: token.colorTextSecondary, fontSize: 11, marginBottom: 4 }}>{label}</div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <div style={{
                    wordBreak: 'break-all',
                    backgroundColor: token.colorFillSecondary,
                    padding: '4px 10px',
                    borderRadius: 4,
                    fontSize: 12,
                    flex: 1,
                    minHeight: 28,
                    display: 'flex',
                    alignItems: 'center',
                    border: `1px solid ${token.colorBorderSecondary}`
                }}>
                    {valStr}
                </div>
                {copyable && (
                    <Tooltip title="Copy">
                        <Button
                            type="text"
                            size="small"
                            style={{ width: 28, height: 28, padding: 0 }}
                            icon={<CodeOutlined style={{ fontSize: 14 }} />}
                            onClick={() => {
                                navigator.clipboard.writeText(valStr);
                                message.success("Copied");
                            }}
                        />
                    </Tooltip>
                )}
            </div>
        </div>
    );
};

const UIInspectorView: React.FC = () => {
    const { selectedDevice } = useDeviceStore();
    const {
        uiHierarchy,
        isFetchingHierarchy,
        fetchUIHierarchy,
        checkAndRefreshUIHierarchy,
    } = useAutomationStore();

    const { t } = useTranslation();
    const { token } = theme.useToken();

    const [selectedNode, setSelectedNode] = useState<any>(null);
    const [searchText, setSearchText] = useState("");
    const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]);
    const [autoExpandParent, setAutoExpandParent] = useState(true);
    const [selectedAction, setSelectedAction] = useState("click");
    const [inputText, setInputText] = useState("");
    const [searchMode, setSearchMode] = useState<"auto" | "xpath" | "advanced">("auto");

    // Detect search mode from query
    const getEffectiveSearchMode = (query: string): string => {
        if (searchMode !== "auto") return searchMode;
        if (query.startsWith("//")) return "xpath";
        if (query.includes(":") || query.includes("=") || /\s+(AND|OR)\s+/i.test(query)) return "advanced";
        return "text";
    };

    const searchHelpContent = (
        <div style={{ maxWidth: 380 }}>
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
                <div style={{ fontSize: 11, marginBottom: 6 }}>
                    <div style={{ marginBottom: 4 }}>
                        {t("automation.advanced_operators")}
                    </div>
                    <div style={{ display: "flex", marginBottom: 2 }}>
                        <code style={{ width: 80 }}>attr:value</code>
                        <span>{t("automation.op_contains")}</span>
                    </div>
                    <div style={{ display: "flex", marginBottom: 2 }}>
                        <code style={{ width: 80 }}>attr=value</code>
                        <span>{t("automation.op_equals")}</span>
                    </div>
                    <div style={{ display: "flex", marginBottom: 2 }}>
                        <code style={{ width: 80 }}>attr^value</code>
                        <span>{t("automation.op_starts")}</span>
                    </div>
                    <div style={{ display: "flex", marginBottom: 2 }}>
                        <code style={{ width: 80 }}>attr$value</code>
                        <span>{t("automation.op_ends")}</span>
                    </div>
                </div>
                <div style={{ fontSize: 11, marginBottom: 6 }}>
                    <div style={{ marginBottom: 4 }}>
                        {t("automation.advanced_attrs")}
                    </div>
                    <code>
                        text, id, desc, class, clickable, enabled, bounds...
                    </code>
                </div>
                <div style={{ fontSize: 11 }}>
                    <div style={{ marginBottom: 4 }}>
                        {t("automation.advanced_examples")}
                    </div>
                    <code style={{ display: "block", marginBottom: 2 }}>
                        clickable:true AND text:OK
                    </code>
                    <code style={{ display: "block", marginBottom: 2 }}>
                        id~login OR desc~登录
                    </code>
                    <code style={{ display: "block" }}>id^com.app</code>
                </div>
            </div>
        </div>
    );

    const generateStepJson = (node: any, type: "click" | "long_click") => {
        const step = {
            type: "check",
            value: node.text ? "text" : (node.resourceId ? "id" : "description"),
            checkValue: node.text || node.resourceId || node.contentDesc || "",
            waitTimeout: 5000,
            onFailure: "stop",
            // We can add the action to perform on success if we extend the task engine
            // For now, it's a template for the user
            label: `${type === 'click' ? 'Click' : 'Long Click'}: ${node.text || node.resourceId || node.contentDesc}`
        };
        return JSON.stringify(step, null, 2);
    };

    useEffect(() => {
        if (selectedDevice && !uiHierarchy) {
            fetchUIHierarchy(selectedDevice);
        }
    }, [selectedDevice]);

    // Helper to get node attribute by name
    const getNodeAttr = (node: any, attr: string): string => {
        const lowerAttr = attr.toLowerCase();
        switch (lowerAttr) {
            case "text": return node.text || "";
            case "resource-id": case "resourceid": case "id": return node.resourceId || "";
            case "class": return node.class || "";
            case "package": return node.package || "";
            case "content-desc": case "contentdesc": case "description": case "desc": return node.contentDesc || "";
            case "bounds": return node.bounds || "";
            case "clickable": return node.clickable || "";
            case "enabled": return node.enabled || "";
            case "focused": return node.focused || "";
            case "scrollable": return node.scrollable || "";
            case "checkable": return node.checkable || "";
            case "checked": return node.checked || "";
            case "focusable": return node.focusable || "";
            case "long-clickable": case "longclickable": return node.longClickable || "";
            case "password": return node.password || "";
            case "selected": return node.selected || "";
            default: return "";
        }
    };

    // Evaluate a single condition like "text:value" or "clickable=true"
    const evaluateCondition = (node: any, condition: string): boolean => {
        const operators = ["~", "^", "$", "=", ":"];
        let attr = "", op = "", value = "";

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
            // No operator, treat as text contains
            const lowerCond = condition.toLowerCase();
            return (node.text || "").toLowerCase().includes(lowerCond) ||
                (node.contentDesc || "").toLowerCase().includes(lowerCond) ||
                (node.resourceId || "").toLowerCase().includes(lowerCond);
        }

        const attrValue = getNodeAttr(node, attr).toLowerCase();
        const lowerValue = value.toLowerCase();

        switch (op) {
            case "=": return attrValue === lowerValue;
            case ":": case "~": return attrValue.includes(lowerValue);
            case "^": return attrValue.startsWith(lowerValue);
            case "$": return attrValue.endsWith(lowerValue);
            default: return false;
        }
    };

    // Match node against XPath query
    const matchXPath = (node: any, query: string): boolean => {
        if (!query.startsWith("//")) return false;
        const expr = query.slice(2);

        const bracketIdx = expr.indexOf("[");
        let className = bracketIdx !== -1 ? expr.slice(0, bracketIdx) : expr;
        let predicate = bracketIdx !== -1 ? expr.slice(bracketIdx + 1, -1) : "";

        // Check class name
        if (className && className !== "node" && className !== "*") {
            const shortName = (node.class || "").split(".").pop();
            if (node.class !== className && shortName !== className) return false;
        }

        // Check predicate conditions
        if (predicate) {
            const parts = predicate.split(/\s+and\s+/i);
            for (const part of parts) {
                const trimmed = part.trim();

                // contains(@attr, 'value')
                const containsMatch = trimmed.match(/contains\(@(\w+),\s*['"]([^'"]*)['"]\)/);
                if (containsMatch) {
                    const attrValue = getNodeAttr(node, containsMatch[1]).toLowerCase();
                    if (!attrValue.includes(containsMatch[2].toLowerCase())) return false;
                    continue;
                }

                // @attr='value' or @attr
                if (trimmed.startsWith("@")) {
                    const attrPart = trimmed.slice(1);
                    if (attrPart.includes("=")) {
                        const [attr, val] = attrPart.split("=");
                        const cleanVal = val.replace(/['"]/g, "");
                        if (getNodeAttr(node, attr.trim()).toLowerCase() !== cleanVal.toLowerCase()) return false;
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
        return orGroups.some(orGroup => {
            const andParts = orGroup.split(/\s+AND\s+/i);
            return andParts.every(part => evaluateCondition(node, part.trim()));
        });
    };

    // Memoize tree data to avoid unnecessary recursion on every render
    const treeData = React.useMemo(() => {
        if (!uiHierarchy) return [];

        // Recursive search and highlight
        const processTreeData = (node: any, path: string = "0"): any => {
            if (!node) return null;

            const key = `${path}-${node.class}-${node.bounds}`;
            const children = (node.nodes || []).map((child: any, i: number) => processTreeData(child, `${path}-${i}`)).filter(Boolean);

            const trimmedSearch = searchText.trim();
            let match = false;

            if (trimmedSearch) {
                const effectiveMode = getEffectiveSearchMode(trimmedSearch);
                if (effectiveMode === "xpath") {
                    match = matchXPath(node, trimmedSearch);
                } else if (effectiveMode === "advanced") {
                    match = matchAdvanced(node, trimmedSearch);
                } else {
                    // Simple text search
                    const lowerSearch = trimmedSearch.toLowerCase();
                    match = (node.text || "").toLowerCase().includes(lowerSearch) ||
                        (node.resourceId || "").toLowerCase().includes(lowerSearch) ||
                        (node.class || "").toLowerCase().includes(lowerSearch) ||
                        (node.contentDesc || "").toLowerCase().includes(lowerSearch);
                }
            }

            // If searching but no match and no matching children, prune this branch
            if (trimmedSearch && !match && children.length === 0) return null;

            const lowerSearch = searchText.toLowerCase().trim();

            return {
                key,
                title: (
                    <Space size={4}>
                        <span style={{ color: token.colorPrimary, fontSize: 11 }}>[{(node.class || "").split('.').pop()}]</span>
                        {node.text && (
                            <span style={{
                                fontWeight: 500,
                                backgroundColor: match ? token.colorWarningBg : 'transparent'
                            }}>
                                "{node.text}"
                            </span>
                        )}
                        {node.resourceId && (
                            <span style={{
                                color: token.colorTextSecondary,
                                fontSize: 10,
                                backgroundColor: match && !node.text ? token.colorWarningBg : 'transparent'
                            }}>
                                #{node.resourceId.split('/').pop()}
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

    // Automatically expand results when searching or on first load
    useEffect(() => {
        if (!uiHierarchy) return;

        if (searchText.trim()) {
            const keys: React.Key[] = [];
            const getAllKeys = (data: any[]) => {
                data.forEach(item => {
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
            // Expand first 2 levels by default if nothing is expanded
            const keys: React.Key[] = [];
            const getInitialKeys = (data: any[], depth: number) => {
                if (depth > 2) return;
                data.forEach(item => {
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

    return (
        <div style={{ padding: 16, height: "100%", overflow: 'hidden' }}>
            <Card
                title={
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <Space>
                            <BlockOutlined />
                            {t("automation.ui_hierarchy")}
                        </Space>
                        <Space>
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
                                            ? "clickable:true AND text:确定"
                                            : t("common.search")
                                }
                                prefix={<SearchOutlined />}
                                suffix={
                                    <Tooltip title={searchHelpContent} placement="bottomRight">
                                        <QuestionCircleOutlined style={{ color: token.colorTextSecondary, cursor: 'help' }} />
                                    </Tooltip>
                                }
                                size="small"
                                value={searchText}
                                onChange={e => setSearchText(e.target.value)}
                                style={{ width: 280 }}
                                allowClear
                            />
                            <Button
                                icon={<ReloadOutlined />}
                                size="small"
                                loading={isFetchingHierarchy}
                                onClick={() => selectedDevice && fetchUIHierarchy(selectedDevice)}
                            >
                                {t("common.refresh")}
                            </Button>
                        </Space>
                    </div>
                }
                styles={{ body: { padding: 0, height: 'calc(100% - 48px)', display: 'flex', flexDirection: 'column' } }}
                style={{ height: '100%', display: 'flex', flexDirection: 'column' }}
            >
                <div style={{ display: 'flex', flex: 1, minHeight: 0 }}>
                    <div style={{ flex: 1, overflow: 'auto', padding: 12, borderRight: `1px solid ${token.colorBorderSecondary}` }}>
                        {uiHierarchy ? (
                            <Tree
                                showLine
                                switcherIcon={<ArrowDownOutlined style={{ fontSize: 10 }} />}
                                onSelect={(_, info: any) => {
                                    const node = info.node.data;
                                    setSelectedNode(node);
                                    // Auto select input action for EditText
                                    if (node.class && node.class.toLowerCase().includes("edittext")) {
                                        setSelectedAction("input");
                                    } else {
                                        setSelectedAction("click");
                                    }
                                }}
                                treeData={treeData}
                                expandedKeys={expandedKeys}
                                autoExpandParent={autoExpandParent}
                                onExpand={setExpandedKeys}
                            />
                        ) : (
                            <Empty
                                description={selectedDevice ? t("automation.no_hierarchy_info") : t("app.select_device")}
                                style={{ marginTop: 100 }}
                            />
                        )}
                    </div>
                    <div style={{ width: 350, overflow: 'auto', padding: 12, backgroundColor: token.colorFillAlter }}>
                        {selectedNode ? (
                            <div style={{ fontSize: 13 }}>
                                <h4 style={{ marginBottom: 12, borderBottom: `1px solid ${token.colorBorder}`, paddingBottom: 8 }}>
                                    {t("automation.element_details")}
                                </h4>
                                <DetailItem label="Text" value={selectedNode.text} copyable />
                                <DetailItem label="ID" value={selectedNode.resourceId} copyable />
                                <DetailItem label="Class" value={selectedNode.class} />
                                <DetailItem label="Content Desc" value={selectedNode.contentDesc} />
                                <DetailItem label="Bounds" value={selectedNode.bounds} />
                                <Divider style={{ margin: '12px 0' }} />
                                <DetailItem label="Clickable" value={selectedNode.clickable} />
                                <DetailItem label="Enabled" value={selectedNode.enabled} />
                                <DetailItem label="Focused" value={selectedNode.focused} />
                                <DetailItem label="Scrollable" value={selectedNode.scrollable} />
                                <DetailItem label="Checkable" value={selectedNode.checkable} />
                                <DetailItem label="Long Clickable" value={selectedNode.longClickable} />

                                <div style={{ marginTop: 20 }}>
                                    <div style={{ color: token.colorTextSecondary, fontSize: 11, marginBottom: 8, fontWeight: 500 }}>
                                        {t("automation.node_actions")}
                                    </div>
                                    <Space.Compact style={{ width: '100%', marginBottom: 8 }}>
                                        <Select
                                            value={selectedAction}
                                            onChange={setSelectedAction}
                                            style={{ width: '40%' }}
                                            size="small"
                                            options={[
                                                { label: t("automation.click"), value: "click" },
                                                { label: t("automation.long_click"), value: "long_click" },
                                                { label: t("automation.swipe_up"), value: "swipe_up" },
                                                { label: t("automation.swipe_down"), value: "swipe_down" },
                                                { label: t("automation.swipe_left"), value: "swipe_left" },
                                                { label: t("automation.swipe_right"), value: "swipe_right" },
                                                { label: t("automation.back"), value: "back" },
                                                { label: t("automation.home"), value: "home" },
                                                { label: t("automation.recent"), value: "recent" },
                                                { label: t("automation.input"), value: "input" }
                                            ]}
                                        />
                                        <Button
                                            type="primary"
                                            size="small"
                                            style={{ width: '60%' }}
                                            icon={<PlayCircleOutlined />}
                                            onClick={async () => {
                                                if (selectedDevice && selectedNode.bounds) {
                                                    try {
                                                        if (selectedAction === "input") {
                                                            await (window as any).go.main.App.InputNodeText(selectedDevice, selectedNode.bounds, inputText);
                                                        } else {
                                                            await (window as any).go.main.App.PerformNodeAction(selectedDevice, selectedNode.bounds, selectedAction);
                                                        }
                                                        message.success(t("automation.action_success"));
                                                        // Background refresh with diffing
                                                        setTimeout(async () => {
                                                            if (selectedDevice) {
                                                                const changed = await checkAndRefreshUIHierarchy(selectedDevice);
                                                                if (changed) {
                                                                    message.info(t("automation.ui_updated"));
                                                                }
                                                            }
                                                        }, 800);
                                                    } catch (err) {
                                                        message.error(String(err));
                                                    }
                                                }
                                            }}
                                        >
                                            {t("automation.simulate")}
                                        </Button>
                                    </Space.Compact>

                                    {selectedAction === "input" && (
                                        <Input
                                            size="small"
                                            placeholder={t("automation.input_placeholder")}
                                            value={inputText}
                                            onChange={e => setInputText(e.target.value)}
                                            style={{ marginBottom: 8 }}
                                            onPressEnter={async () => {
                                                if (selectedDevice && selectedNode.bounds && inputText) {
                                                    try {
                                                        await (window as any).go.main.App.InputNodeText(selectedDevice, selectedNode.bounds, inputText);
                                                        message.success(t("automation.action_success"));
                                                        // Background refresh with diffing
                                                        setTimeout(async () => {
                                                            if (selectedDevice) {
                                                                const changed = await checkAndRefreshUIHierarchy(selectedDevice);
                                                                if (changed) {
                                                                    message.info(t("automation.ui_updated"));
                                                                }
                                                            }
                                                        }, 800);
                                                    } catch (err) {
                                                        message.error(String(err));
                                                    }
                                                }
                                            }}
                                        />
                                    )}

                                    <Button
                                        block
                                        size="small"
                                        icon={<CopyOutlined />}
                                        onClick={() => {
                                            navigator.clipboard.writeText(generateStepJson(selectedNode, selectedAction as any));
                                            message.success(t("automation.copy_step_success"));
                                        }}
                                    >
                                        {t("automation.copy_as_step")}
                                    </Button>

                                    <Divider style={{ margin: '16px 0' }} />

                                    <Button
                                        type="primary"
                                        ghost
                                        block
                                        size="small"
                                        onClick={() => {
                                            const value = selectedNode.text || selectedNode.resourceId || selectedNode.contentDesc;
                                            if (value) {
                                                navigator.clipboard.writeText(value);
                                                message.success(t("common.copy_success", { name: value }));
                                            }
                                        }}
                                    >
                                        {t("automation.copy_selector")}
                                    </Button>
                                </div>
                            </div>
                        ) : (
                            <div style={{ textAlign: 'center', color: token.colorTextDescription, marginTop: 100 }}>
                                {t("automation.select_element_tip")}
                            </div>
                        )}
                    </div>
                </div>
            </Card>
        </div>
    );
};

export default UIInspectorView;
