import { useRef, useEffect, useState, useMemo } from "react";
import { Button, Input, Select, Space, Checkbox, message, Modal, Tooltip, Tag, theme } from "antd";
import { useTranslation } from "react-i18next";
import {
  PauseOutlined,
  PlayCircleOutlined,
  ClearOutlined,
  DownOutlined,
  InfoCircleOutlined,
  DeleteOutlined,
  StopOutlined,
} from "@ant-design/icons";
import { useVirtualizer } from "@tanstack/react-virtual";
import DeviceSelector from "./DeviceSelector";
// @ts-ignore
import { main } from "../../wailsjs/go/models";
// @ts-ignore
import { ListPackages, StartApp, ForceStopApp, IsAppRunning } from "../../wailsjs/go/main/App";

const { Option } = Select;

interface Device {
  id: string;
  state: string;
  model: string;
  brand: string;
}

interface LogcatViewProps {
  devices: Device[];
  selectedDevice: string;
  setSelectedDevice: (device: string) => void;
  fetchDevices?: () => void;
  isLogging: boolean;
  toggleLogcat: (pkg: string) => void;
  logs: string[];
  setLogs: (logs: string[]) => void;
  logFilter?: string;
  setLogFilter?: (filter: string) => void;
  useRegex?: boolean;
  setUseRegex?: (use: boolean) => void;
  preFilter?: string;
  setPreFilter?: (filter: string) => void;
  preUseRegex?: boolean;
  setPreUseRegex?: (use: boolean) => void;
  excludeFilter?: string;
  setExcludeFilter?: (filter: string) => void;
  excludeUseRegex?: boolean;
  setExcludeUseRegex?: (use: boolean) => void;
  selectedPackage?: string;
  setSelectedPackage?: (pkg: string) => void;
}

export default function LogcatView({
  devices,
  selectedDevice,
  setSelectedDevice,
  fetchDevices,
  isLogging,
  toggleLogcat,
  logs,
  setLogs,
  logFilter: propLogFilter,
  setLogFilter: propSetLogFilter,
  useRegex: propUseRegex,
  setUseRegex: propSetUseRegex,
  preFilter,
  setPreFilter,
  preUseRegex,
  setPreUseRegex,
  excludeFilter,
  setExcludeFilter,
  excludeUseRegex,
  setExcludeUseRegex,
  selectedPackage: propSelectedPackage,
  setSelectedPackage: propSetSelectedPackage,
}: LogcatViewProps) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const parentRef = useRef<HTMLDivElement>(null);
  const scrollingRef = useRef(false);

  // Logcat local state
  const [packages, setPackages] = useState<main.AppPackage[]>([]);
  const [appRunningStatus, setAppRunningStatus] = useState<boolean>(false);
  // Use props if available, otherwise local state
  const [localLogFilter, setLocalLogFilter] = useState("");
  const [localUseRegex, setLocalUseRegex] = useState(false);
  const [localSelectedPackage, setLocalSelectedPackage] = useState<string>("");
  
  const logFilter = propLogFilter !== undefined ? propLogFilter : localLogFilter;
  const setLogFilter = propSetLogFilter || setLocalLogFilter;
  const useRegex = propUseRegex !== undefined ? propUseRegex : localUseRegex;
  const setUseRegex = propSetUseRegex || setLocalUseRegex;
  const selectedPackage = propSelectedPackage !== undefined ? propSelectedPackage : localSelectedPackage;
  const setSelectedPackage = propSetSelectedPackage || setLocalSelectedPackage;

  const [autoScroll, setAutoScroll] = useState(true);
  const [levelFilter, setLevelFilter] = useState<string[]>([]);
  const [matchCase, setMatchCase] = useState(false);
  const [matchWholeWord, setMatchWholeWord] = useState(false);

  useEffect(() => {
    const fetchPackageList = async () => {
      if (!selectedDevice) return;
      try {
        const res = await ListPackages(selectedDevice, "user");
        setPackages(res || []);
      } catch (err) {
        console.error("Failed to fetch packages for logcat:", err);
      }
    };
    fetchPackageList();
  }, [selectedDevice]);

  useEffect(() => {
    let timer: number;
    const checkRunning = async () => {
      if (selectedDevice && selectedPackage) {
        try {
          const running = await IsAppRunning(selectedDevice, selectedPackage);
          setAppRunningStatus(running);
        } catch {}
      } else {
        setAppRunningStatus(false);
      }
    };

    checkRunning();
    // Poll every 5 seconds for status updates
    timer = window.setInterval(checkRunning, 5000);
    return () => clearInterval(timer);
  }, [selectedDevice, selectedPackage]);

  const handleStartApp = async () => {
    if (!selectedPackage || !selectedDevice) return;
    const hide = message.loading(t("app.launching", { name: selectedPackage }), 0);
    try {
      await ForceStopApp(selectedDevice, selectedPackage);
      await StartApp(selectedDevice, selectedPackage);
      message.success(t("app.start_app_success", { name: selectedPackage }));
    } catch (err) {
      message.error(t("app.start_app_failed") + ": " + String(err));
    } finally {
      hide();
    }
  };

  const getLogLevel = (text: string) => {
    if (text.includes(" E/") || text.includes(" F/") || text.startsWith("E/"))
      return "E";
    if (text.includes(" W/") || text.startsWith("W/")) return "W";
    if (text.includes(" I/") || text.startsWith("I/")) return "I";
    if (text.includes(" D/") || text.startsWith("D/")) return "D";
    return "V";
  };

  // 1. 深度编译过滤信息
  const filterInfo = useMemo(() => {
    const rawInput = logFilter || "";
    if (!rawInput.trim() && levelFilter.length === 0) {
      return { regex: null, invalid: false, pattern: "", highlighter: null };
    }

    try {
      let pattern = rawInput;
      if (!useRegex) {
        pattern = pattern.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
      }

      if (matchWholeWord) {
        pattern = `\\b(?:${pattern})\\b`;
      }

      const flags = matchCase ? "" : "i";
      return {
        regex: new RegExp(pattern, flags),
        highlighter: new RegExp(pattern, flags + "g"),
        invalid: false,
        pattern,
      };
    } catch (e) {
      return {
        regex: null,
        invalid: true,
        pattern: rawInput,
        highlighter: null,
      };
    }
  }, [logFilter, useRegex, matchCase, matchWholeWord]);

  // Filter Presets state
  interface FilterPreset {
    id: string;
    name: string;
    pattern: string;
    isRegex: boolean;
  }
  const [savedFilters, setSavedFilters] = useState<FilterPreset[]>(() => {
    try {
      const saved = localStorage.getItem("adbGUI_logcat_filters");
      return saved ? JSON.parse(saved) : [];
    } catch {
      return [];
    }
  });

  // Persist filters
  useEffect(() => {
    localStorage.setItem("adbGUI_logcat_filters", JSON.stringify(savedFilters));
  }, [savedFilters]);

  // Selected presets for Pre-Filter
  const [selectedPresetIds, setSelectedPresetIds] = useState<string[]>([]);
  // Selected presets for Exclude Filter
  const [selectedExcludePresetIds, setSelectedExcludePresetIds] = useState<string[]>([]);

  // Update pre-filter when presets selection changes
  useEffect(() => {
    if (selectedPresetIds.length === 0) {
      if (setPreFilter) setPreFilter("");
      return;
    }

    const selected = savedFilters.filter(f => selectedPresetIds.includes(f.id));
    if (selected.length === 0) return;

    const parts = selected.map(f => {
       if (f.isRegex) return `(${f.pattern})`;
       return `(${f.pattern.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")})`;
    });
    
    const combinedPattern = parts.join("|");
    if (setPreFilter) setPreFilter(combinedPattern);
    if (setPreUseRegex) setPreUseRegex(true);
  }, [selectedPresetIds, savedFilters, setPreFilter, setPreUseRegex]);

  // Update exclude-filter when presets selection changes
  useEffect(() => {
    if (selectedExcludePresetIds.length === 0) {
      if (setExcludeFilter) setExcludeFilter("");
      return;
    }

    const selected = savedFilters.filter(f => selectedExcludePresetIds.includes(f.id));
    if (selected.length === 0) return;

    const parts = selected.map(f => {
       if (f.isRegex) return `(${f.pattern})`;
       return `(${f.pattern.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")})`;
    });
    
    const combinedPattern = parts.join("|");
    if (setExcludeFilter) setExcludeFilter(combinedPattern);
    if (setExcludeUseRegex) setExcludeUseRegex(true);
  }, [selectedExcludePresetIds, savedFilters, setExcludeFilter, setExcludeUseRegex]);

  // Save Filter Modal state
  const [isSaveModalOpen, setIsSaveModalOpen] = useState(false);
  const [newFilterName, setNewFilterName] = useState("");

  const handleSaveFilter = () => {
    if (!logFilter) {
       message.warning("Please enter a filter pattern first.");
       return;
    }
    setNewFilterName(logFilter);
    setIsSaveModalOpen(true);
  };

  const confirmSaveFilter = () => {
    if (!newFilterName.trim()) {
      message.error("Name cannot be empty");
      return;
    }
    const newPreset: FilterPreset = {
      id: Date.now().toString(),
      name: newFilterName,
      pattern: logFilter,
      isRegex: !!useRegex
    };
    setSavedFilters(prev => [...prev, newPreset]);
    message.success(t("logcat.filter_saved"));
    setIsSaveModalOpen(false);
  };

  // 2. 强力过滤引擎
  const filteredLogs = useMemo(() => {
    if (!logFilter && levelFilter.length === 0) return logs;

    const { regex, invalid } = filterInfo;
    const hasLevelFilter = levelFilter.length > 0;
    
    // Always prefer using the pre-calculated regex if it's valid.
    // This ensures 'Match Case' and 'Whole Word' are respected.
    const useRegexEngine = !invalid && !!regex;

    return logs.filter((log) => {
      // A. Level Filtering
      if (hasLevelFilter) {
        const level = getLogLevel(log);
        if (!levelFilter.includes(level)) return false;
      }

      // B. Text/Regex Filtering
      if (logFilter && !invalid) {
        const line = String(log).replace(/\u001b\[[0-9;]*m/g, "");

        if (useRegexEngine) {
          // Use the unified regex engine (handles Case and Whole Word correctly)
          if (regex!.test(line)) return true;
          
          // Support for legacy |-separated segments if user explicitly typed | in regex mode
          if (useRegex && logFilter.includes("|")) {
             // Already handled by the combined regex usually, but keeping for robustness
             // Actually, the main regex already contains the | if user typed it.
          }
          return false;
        } else {
          // Fallback to simple include (should rarely be hit if filterInfo is working)
          return line.toLowerCase().includes(logFilter.toLowerCase());
        }
      }
      return true;
    });
  }, [
    logs,
    levelFilter,
    logFilter,
    filterInfo,
    useRegex,
    matchCase,
    matchWholeWord,
  ]);

  const virtualizer = useVirtualizer({
    count: filteredLogs.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 24,
    overscan: 10,
  });

  // 自动滚动逻辑
  useEffect(() => {
    if (autoScroll && filteredLogs.length > 0) {
      scrollingRef.current = true;
      virtualizer.scrollToIndex(filteredLogs.length - 1, {
        align: "end",
      });
      const timer = setTimeout(() => {
        scrollingRef.current = false;
      }, 100);
      return () => clearTimeout(timer);
    }
  }, [filteredLogs.length, autoScroll, virtualizer]);

  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    if (scrollingRef.current) return;
    const target = e.currentTarget;
    const { scrollTop, scrollHeight, clientHeight } = target;
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 50;
    if (!isAtBottom && autoScroll) {
      setAutoScroll(false);
    }
    else if (isAtBottom && !autoScroll) {
      setAutoScroll(true);
    }
  };

  const scrollToBottom = () => {
    scrollingRef.current = true;
    setAutoScroll(true);
    virtualizer.scrollToIndex(filteredLogs.length - 1, {
      align: "end",
      behavior: "smooth",
    });
    setTimeout(() => {
      scrollingRef.current = false;
    }, 1000);
  };

  const getLogColor = (level: string) => {
    switch (level) {
      case "E": return "#f14c4c";
      case "W": return "#cca700";
      case "I": return "#3794ff";
      case "D": return "#4ec9b0";
      default: return "#d4d4d4";
    }
  };

  const renderLogLine = (text: string) => {
    if (!text) return null;
    const level = getLogLevel(text);
    const color = getLogColor(level);
    const { highlighter, invalid } = filterInfo;

    if (!logFilter || invalid || !highlighter) {
      return <span style={{ color }}>{text}</span>;
    }

    try {
      const parts: React.ReactNode[] = [];
      let lastIndex = 0;
      let match;
      const activeHighlighter = highlighter;
      activeHighlighter.lastIndex = 0;

      while ((match = activeHighlighter.exec(text)) !== null) {
        if (match.index > lastIndex) {
          parts.push(text.substring(lastIndex, match.index));
        }
        parts.push(
          <mark
            key={match.index}
            style={{
              backgroundColor: "#ffcc00",
              color: "#000",
              borderRadius: "2px",
              padding: "0 1px",
            }}
          >
            {match[0]}
          </mark>
        );
        lastIndex = activeHighlighter.lastIndex;
        if (match[0].length === 0) activeHighlighter.lastIndex++;
      }

      if (lastIndex < text.length) {
        parts.push(text.substring(lastIndex));
      }

      return <span style={{ color }}>{parts.length > 0 ? parts : text}</span>;
    } catch (e) {
      return <span style={{ color }}>{text}</span>;
    }
  };

  const handleRemovePreset = (id: string) => {
    setSavedFilters(prev => prev.filter(p => p.id !== id));
    setSelectedPresetIds(prev => prev.filter(pid => pid !== id));
    setSelectedExcludePresetIds(prev => prev.filter(pid => pid !== id));
    message.success(t("logcat.filter_removed") || "Filter rule removed");
  };

  return (
    <div
      style={{
        padding: "16px 24px",
        flex: 1,
        display: "flex",
        flexDirection: "column",
        height: "100%",
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
        <h2 style={{ margin: 0, color: token.colorText }}>{t("logcat.title")}</h2>
        <Space>
          <DeviceSelector
            devices={devices}
            selectedDevice={selectedDevice}
            onDeviceChange={setSelectedDevice}
            onRefresh={fetchDevices || (() => {})}
            loading={false}
          />
          <span style={{ color: "#888", fontSize: "12px", marginLeft: 8 }}>{t("logcat.package") || "Package"}:</span>
          <Select
            showSearch
            value={selectedPackage}
            onChange={setSelectedPackage}
            style={{ width: 220 }}
            placeholder={t("logcat.apps_placeholder")}
            disabled={isLogging}
            allowClear
            filterOption={(input, option) =>
              (option?.children as unknown as string)
                .toLowerCase()
                .indexOf(input.toLowerCase()) >= 0
            }
          >
            {packages.map((p) => (
              <Option key={p.name} value={p.name}>
                <Space>
                  {p.name}
                  {p.name === selectedPackage && appRunningStatus && (
                    <Tag color="success" style={{ fontSize: '10px', height: '16px', lineHeight: '14px', margin: 0, padding: '0 4px' }}>
                      RUNNING
                    </Tag>
                  )}
                </Space>
              </Option>
            ))}
          </Select>
          {selectedPackage && (
            <Tooltip title={appRunningStatus ? t("apps.force_stop") : t("apps.launch_app")}>
              <Button 
                icon={appRunningStatus ? <StopOutlined /> : <PlayCircleOutlined />} 
                onClick={appRunningStatus ? () => {
                  ForceStopApp(selectedDevice, selectedPackage).then(() => setAppRunningStatus(false));
                } : handleStartApp}
                style={{ color: appRunningStatus ? "#ff4d4f" : "#52c41a" }}
              />
            </Tooltip>
          )}
          <Button
            type={isLogging ? "primary" : "default"}
            danger={isLogging}
            icon={isLogging ? <PauseOutlined /> : <PlayCircleOutlined />}
            onClick={() => toggleLogcat(selectedPackage)}
          >
            {isLogging ? t("logcat.stop") : t("logcat.start")}
          </Button>
          <Button icon={<ClearOutlined />} onClick={() => setLogs([])}>
            {t("logcat.clear")}
          </Button>
        </Space>
      </div>

      <div
        style={{
          marginBottom: 12,
          display: "flex",
          flexDirection: "column",
          gap: 12,
          flexShrink: 0,
        }}
      >
        {/* Row 1: Fixed Pre-Filter (Advanced) - Moved up */}
        <div style={{ display: "flex", gap: 12, alignItems: "center", whiteSpace: "nowrap" }}>
          <span style={{ color: token.colorTextSecondary, fontSize: "12px", fontWeight: "bold", width: 80, minWidth: 80, textAlign: "right" }}>{t("logcat.pre_filter") || "Pre-Filter"}:</span>
          <div style={{ flex: 1 }}>
            <Select
              mode="multiple"
              style={{ width: "100%" }}
              placeholder={t("logcat.select_pre_filters") || "Select filters to strictly cache only matching logs (Pre-Filter)"}
              value={selectedPresetIds}
              onChange={setSelectedPresetIds}
              optionLabelProp="filterName"
              options={savedFilters.map(f => ({ 
                label: (
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                    <span>{f.name}</span>
                    <DeleteOutlined 
                      className="delete-preset-icon"
                      style={{ color: "#999", fontSize: '12px' }}
                      onMouseEnter={(e) => e.currentTarget.style.color = "#ff4d4f"}
                      onMouseLeave={(e) => e.currentTarget.style.color = "#999"}
                      onClick={(e) => {
                        e.stopPropagation();
                        e.preventDefault();
                        handleRemovePreset(f.id);
                      }}
                    />
                  </div>
                ), 
                value: f.id,
                filterName: f.name
              }))}
              maxTagCount="responsive"
              allowClear
            />
          </div>
          <div style={{ flexShrink: 0, display: "flex", alignItems: "center", gap: 4, fontSize: "11px", color: "#666", minWidth: 150 }}>
             <InfoCircleOutlined /> <span>{t("logcat.pre_filter_info") || "Only matching logs are buffered"}</span>
          </div>
        </div>

        {/* Row 1.5: Exclude Filter (New) */}
        <div style={{ display: "flex", gap: 12, alignItems: "center", whiteSpace: "nowrap" }}>
          <span style={{ color: "#ff4d4f", fontSize: "12px", fontWeight: "bold", width: 80, minWidth: 80, textAlign: "right" }}>{t("logcat.exclude_filter") || "Exclude"}:</span>
          <div style={{ flex: 1 }}>
            <Select
              mode="multiple"
              style={{ width: "100%" }}
              placeholder={t("logcat.select_exclude_filters") || "Select filters to exclude matching logs"}
              value={selectedExcludePresetIds}
              onChange={setSelectedExcludePresetIds}
              optionLabelProp="filterName"
              options={savedFilters.map(f => ({ 
                label: (
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                    <span>{f.name}</span>
                    <DeleteOutlined 
                      className="delete-preset-icon"
                      style={{ color: "#999", fontSize: '12px' }}
                      onMouseEnter={(e) => e.currentTarget.style.color = "#ff4d4f"}
                      onMouseLeave={(e) => e.currentTarget.style.color = "#999"}
                      onClick={(e) => {
                        e.stopPropagation();
                        e.preventDefault();
                        handleRemovePreset(f.id);
                      }}
                    />
                  </div>
                ), 
                value: f.id,
                filterName: f.name
              }))}
              maxTagCount="responsive"
              allowClear
            />
          </div>
          <div style={{ flexShrink: 0, display: "flex", alignItems: "center", gap: 4, fontSize: "11px", color: "#ff7875", minWidth: 150 }}>
             <StopOutlined /> <span>{t("logcat.exclude_filter_info") || "Matching logs will be dropped"}</span>
          </div>
        </div>

        {/* Row 2: Real-time View Filter (Most used) - Moved down */}
        <div style={{ display: "flex", gap: 12, alignItems: "center", whiteSpace: "nowrap" }}>
          <span style={{ color: token.colorTextSecondary, fontSize: "12px", fontWeight: "bold", width: 80, minWidth: 80, textAlign: "right" }}>{t("logcat.view_filter")}:</span>
          <div style={{ flex: 1, position: "relative" }}>
            <Input
              id="logcat-filter-input"
              placeholder={
                useRegex ? t("logcat.filter_regex") : t("logcat.filter_text")
              }
              value={logFilter}
              onChange={(e) => setLogFilter(e.target.value)}
              status={filterInfo.invalid ? "error" : ""}
              suffix={
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: 6,
                      marginRight: 4,
                    }}
                  >
                    {logFilter && !filterInfo.invalid && (
                      <span
                        style={{
                          fontSize: "9px",
                          padding: "0 4px",
                          borderRadius: "2px",
                          backgroundColor: useRegex ? "#e6f7ff" : "#f5f5f5",
                          color: useRegex ? "#1677ff" : "#888",
                          border: `1px solid ${useRegex ? "#91d5ff" : "#d9d9d9"}`,
                          fontWeight: "bold",
                        }}
                      >
                        {useRegex ? "REG" : "TXT"}
                      </span>
                    )}
                    <span style={{ fontSize: "11px", color: "#888" }}>
                      {logFilter ? `${filteredLogs.length} / ${logs.length}` : logs.length}
                    </span>
                  </div>
                  <Space size={2} style={{ marginRight: -7 }}>
                    <Button
                      size="small"
                      type={matchCase ? "primary" : "default"}
                      style={{
                        fontSize: "11px", padding: "0 4px", height: 20, minWidth: 24, borderRadius: 2,
                        backgroundColor: matchCase ? "#1677ff" : "#f5f5f5", color: matchCase ? "#fff" : "#555",
                        border: "none", fontWeight: "bold",
                      }}
                      onClick={() => setMatchCase(!matchCase)}
                      title={t("logcat.match_case") || "Match Case (Aa)"}
                    > Aa </Button>
                    <Button
                      size="small"
                      type={matchWholeWord ? "primary" : "default"}
                      style={{
                        fontSize: "11px", padding: "0 4px", height: 20, minWidth: 24, borderRadius: 2,
                        backgroundColor: matchWholeWord ? "#1677ff" : "#f5f5f5", color: matchWholeWord ? "#fff" : "#555",
                        border: "none", fontWeight: "bold",
                      }}
                      onClick={() => setMatchWholeWord(!matchWholeWord)}
                      title={t("logcat.match_whole_word") || "Match Whole Word (W)"}
                    > W </Button>
                    <Button
                      size="small"
                      type={useRegex ? "primary" : "default"}
                      style={{
                        fontSize: "11px", padding: "0 4px", height: 20, minWidth: 24, borderRadius: 2,
                        backgroundColor: useRegex ? "#1677ff" : "#f5f5f5", color: useRegex ? "#fff" : "#555",
                        border: "none", fontWeight: "bold",
                      }}
                      onClick={() => setUseRegex(!useRegex)}
                      title={t("logcat.use_regex") || "Use Regular Expression (.*)"}
                    > .* </Button>
                    <Button
                      size="small"
                      type="primary"
                      style={{
                        fontSize: "11px", padding: "0 8px", height: 20, borderRadius: 2,
                        backgroundColor: "#52c41a", color: "#fff",
                        border: "none", fontWeight: "bold", marginLeft: 8
                      }}
                      onClick={handleSaveFilter}
                      title={t("logcat.save_filter")}
                    > {t("logcat.save") || "Save"} </Button>
                  </Space>
                </div>
              }
            />
             {logFilter && (
                <div
                  style={{
                    position: "absolute", top: "100%", left: 0, fontSize: "10px",
                    color: filterInfo.invalid ? "#f5222d" : "#888", marginTop: 2,
                    fontFamily: "monospace", whiteSpace: "nowrap", overflow: "hidden",
                    textOverflow: "ellipsis", width: "100%", zIndex: 10
                  }}
                >
                  {filterInfo.invalid
                    ? t("logcat.invalid_regex")
                    : `${t("logcat.filter_pattern") || "Pattern"}: /${filterInfo.pattern}/${matchCase ? "" : "i"}`}
                </div>
              )}
          </div>
          
          <div style={{ flexShrink: 0 }}>
             <Checkbox.Group
              options={[
                { label: <span style={{ color: getLogColor("E") }}>{t("logcat.level.error")}</span>, value: "E" },
                { label: <span style={{ color: getLogColor("W") }}>{t("logcat.level.warn")}</span>, value: "W" },
                { label: <span style={{ color: getLogColor("I") }}>{t("logcat.level.info")}</span>, value: "I" },
                { label: <span style={{ color: getLogColor("D") }}>{t("logcat.level.debug")}</span>, value: "D" },
                { label: <span style={{ color: getLogColor("V") }}>{t("logcat.level.verbose")}</span>, value: "V" },
              ]}
              value={levelFilter}
              onChange={(vals) => setLevelFilter(vals as string[])}
              style={{ display: "flex", gap: "4px" }}
            />
          </div>
        </div>
      </div>

      <div
        style={{
          flex: 1, position: "relative", minHeight: 0, backgroundColor: "#1e1e1e",
          borderRadius: "4px", overflow: "hidden", marginTop: 12,
        }}
      >
        <div
          ref={parentRef}
          onScroll={handleScroll}
          className="selectable"
          style={{ height: "100%", overflow: "auto", userSelect: "text" }}
        >
          <div style={{ height: `${virtualizer.getTotalSize()}px`, width: "100%", position: "relative" }}>
            {virtualizer.getVirtualItems().map((virtualItem) => (
              <div
                key={virtualItem.index}
                ref={virtualizer.measureElement}
                data-index={virtualItem.index}
                style={{
                  position: "absolute", top: 0, left: 0, width: "100%",
                  transform: `translateY(${virtualItem.start}px)`, padding: "2px 12px",
                  borderBottom: "1px solid #2d2d2d", color: "#d4d4d4",
                  fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                  fontSize: "12px", lineHeight: "1.5", whiteSpace: "pre-wrap", wordBreak: "break-all",
                }}
              >
                {renderLogLine(filteredLogs[virtualItem.index])}
              </div>
            ))}
          </div>
        </div>

        {!autoScroll && filteredLogs.length > 0 && (
          <Button
            type="primary"
            shape="circle"
            icon={<DownOutlined />}
            size="large"
            onClick={scrollToBottom}
            style={{
              position: "absolute", bottom: 24, right: 24,
              boxShadow: "0 4px 12px rgba(0, 0, 0, 0.4)", zIndex: 100,
            }}
          />
        )}
       </div>
      
      {/* Save Filter Modal */}
      <Modal
        title={t("logcat.save_filter")}
        open={isSaveModalOpen}
        onOk={confirmSaveFilter}
        onCancel={() => setIsSaveModalOpen(false)}
        okText={t("common.ok") || "OK"}
        cancelText={t("common.cancel") || "Cancel"}
        centered
      >
        <div style={{ marginBottom: 8 }}>{t("logcat.enter_filter_name")}</div>
        <Input 
          value={newFilterName} 
          onChange={e => setNewFilterName(e.target.value)} 
          placeholder={t("logcat.filter_name_placeholder") || "Filter Name"}
          autoFocus
          onPressEnter={confirmSaveFilter}
        />
      </Modal>
    </div>
  );
}

