import { useRef, useEffect, useState, useMemo } from "react";
import { Button, Input, Select, Space, Checkbox, message } from "antd";
import { useTranslation } from "react-i18next";
import {
  PauseOutlined,
  PlayCircleOutlined,
  ClearOutlined,
  DownOutlined,
} from "@ant-design/icons";
import { useVirtualizer } from "@tanstack/react-virtual";
import DeviceSelector from "./DeviceSelector";
// @ts-ignore
import { main } from "../../wailsjs/go/models";
// @ts-ignore
import { ListPackages } from "../../wailsjs/go/main/App";

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
}: LogcatViewProps) {
  const { t } = useTranslation();
  const parentRef = useRef<HTMLDivElement>(null);
  const scrollingRef = useRef(false);

  // Logcat local state
  const [packages, setPackages] = useState<main.AppPackage[]>([]);
  const [selectedPackage, setSelectedPackage] = useState<string>("");
  const [logFilter, setLogFilter] = useState("");
  const [autoScroll, setAutoScroll] = useState(true);
  const [levelFilter, setLevelFilter] = useState<string[]>([]);
  const [matchCase, setMatchCase] = useState(false);
  const [matchWholeWord, setMatchWholeWord] = useState(false);
  const [useRegex, setUseRegex] = useState(false);

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

  // 2. 强力过滤引擎
  const filteredLogs = useMemo(() => {
    if (!logFilter && levelFilter.length === 0) return logs;

    const { regex, invalid } = filterInfo;
    const hasLevelFilter = levelFilter.length > 0;
    const isRegexMode = useRegex && !invalid && !!regex;
    const simpleFlags = matchCase ? "" : "i";

    const orParts =
      isRegexMode && logFilter.includes("|")
        ? (logFilter
            .split("|")
            .map((p) => {
              const t = p.trim();
              if (!t) return null;
              try {
                return new RegExp(
                  matchWholeWord ? `\\b(?:${t})\\b` : t,
                  simpleFlags
                );
              } catch {
                return null;
              }
            })
            .filter(Boolean) as RegExp[])
        : [];

    return logs.filter((log) => {
      // A. 级别过滤
      if (hasLevelFilter) {
        const level = getLogLevel(log);
        if (!levelFilter.includes(level)) return false;
      }

      // B. 文本/正则过滤
      if (logFilter && !invalid) {
        const line = String(log).replace(/\u001b\[[0-9;]*m/g, "");

        if (isRegexMode) {
          if (line.match(regex!)) return true;
          if (orParts.length > 0) {
            for (const r of orParts) {
              if (line.match(r)) return true;
            }
          }
          return false;
        } else {
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
        <h2 style={{ margin: 0 }}>{t("logcat.title")}</h2>
        <Space>
          <DeviceSelector
            devices={devices}
            selectedDevice={selectedDevice}
            onDeviceChange={setSelectedDevice}
            onRefresh={fetchDevices || (() => {})}
            loading={false}
          />
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
                {p.name}
              </Option>
            ))}
          </Select>
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
          gap: 16,
          alignItems: "center",
          flexShrink: 0,
        }}
      >
        <div style={{ flex: 1, position: "relative" }}>
          <Input
            placeholder={
              useRegex ? t("logcat.filter_regex") : t("logcat.filter_text")
            }
            value={logFilter}
            onChange={(e) => setLogFilter(e.target.value)}
            status={filterInfo.invalid ? "error" : ""}
            suffix={
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                {logFilter && !filterInfo.invalid && (
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: 6,
                      marginRight: 4,
                    }}
                  >
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
                    <span style={{ fontSize: "11px", color: "#888" }}>
                      {filteredLogs.length} / {logs.length}
                    </span>
                  </div>
                )}
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
                textOverflow: "ellipsis", width: "100%",
              }}
            >
              {filterInfo.invalid
                ? t("logcat.invalid_regex")
                : `Pattern: /${filterInfo.pattern}/${matchCase ? "" : "i"}`}
            </div>
          )}
        </div>
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
        />
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
    </div>
  );
}

