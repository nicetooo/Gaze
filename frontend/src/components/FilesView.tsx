import React, { useRef, useEffect } from "react";
import {
  Table,
  Button,
  Tag,
  Space,
  Tooltip,
  Input,
  Checkbox,
  Dropdown,
  Modal,
} from "antd";
import { useTranslation } from "react-i18next";
import {
  ReloadOutlined,
  FolderOutlined,
  FileOutlined,
  CopyOutlined,
  ScissorOutlined,
  MoreOutlined,
  ArrowLeftOutlined,
  FolderAddOutlined,
  SnippetsOutlined,
  PictureOutlined,
  PlaySquareOutlined,
  VerticalAlignBottomOutlined,
} from "@ant-design/icons";
// @ts-ignore
import { GetThumbnail } from "../../wailsjs/go/main/App";
import DeviceSelector from "./DeviceSelector";

interface Device {
  id: string;
  state: string;
  model: string;
  brand: string;
}

const NameWithThumbnail = ({
  deviceId,
  record,
  onClick,
}: {
  deviceId: string;
  record: any;
  onClick: () => void;
}) => {
  const [thumb, setThumb] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(false);

  const name = record.name.toLowerCase();
  const isImage =
    name.endsWith(".jpg") ||
    name.endsWith(".jpeg") ||
    name.endsWith(".png") ||
    name.endsWith(".webp") ||
    name.endsWith(".gif");
  const isVideo =
    name.endsWith(".mp4") ||
    name.endsWith(".mkv") ||
    name.endsWith(".mov") ||
    name.endsWith(".avi");

  React.useEffect(() => {
    if ((isImage || isVideo) && deviceId && !thumb && !loading) {
      setLoading(true);
      GetThumbnail(deviceId, record.path, record.modTime)
        .then((res: string) => {
          setThumb(res);
        })
        .catch(() => {})
        .finally(() => setLoading(false));
    }
  }, [deviceId, record.path, record.modTime]);

  const icon = thumb ? (
    <img
      src={thumb}
      style={{
        width: 24,
        height: 24,
        objectFit: "cover",
        borderRadius: 2,
        display: "block",
      }}
    />
  ) : record.isDir ? (
    <FolderOutlined style={{ color: "#1890ff" }} />
  ) : isImage ? (
    <PictureOutlined style={{ color: "#52c41a" }} />
  ) : isVideo ? (
    <PlaySquareOutlined style={{ color: "#eb2f96" }} />
  ) : (
    <FileOutlined />
  );

  const content = (
    <Space
      onClick={onClick}
      style={{
        cursor: "pointer",
        width: "100%",
      }}
    >
      {icon}
      <span>{record.name}</span>
    </Space>
  );

  if (thumb) {
    return (
      <Tooltip
        title={
          <img
            src={thumb}
            style={{
              maxWidth: 400,
              maxHeight: 400,
              display: "block",
              borderRadius: 4,
            }}
          />
        }
        mouseEnterDelay={0.3}
        overlayStyle={{ maxWidth: "none" }}
        overlayInnerStyle={{ padding: 0, backgroundColor: "transparent" }}
      >
        {content}
      </Tooltip>
    );
  }

  return content;
};

interface FilesViewProps {
  devices: Device[];
  selectedDevice: string;
  setSelectedDevice: (id: string) => void;
  fetchDevices: () => Promise<void>;
  loading: boolean;
  currentPath: string;
  setCurrentPath: (path: string) => void;
  fileList: any[];
  filesLoading: boolean;
  fetchFiles: (path: string) => Promise<void>;
  showHiddenFiles: boolean;
  setShowHiddenFiles: (show: boolean) => void;
  clipboard: { path: string; type: "copy" | "cut" } | null;
  handleFileAction: (action: string, file: any) => Promise<void>;
}

const FocusInput = ({
  defaultValue,
  onChange,
  placeholder,
  selectAll,
}: any) => {
  const inputRef = useRef<any>(null);
  useEffect(() => {
    const timer = setTimeout(() => {
      inputRef.current?.focus();
      if (selectAll) {
        inputRef.current?.select();
      }
    }, 200);
    return () => clearTimeout(timer);
  }, [selectAll]);

  return (
    <Input
      ref={inputRef}
      defaultValue={defaultValue}
      onChange={onChange}
      placeholder={placeholder}
      style={{ marginTop: 16 }}
      onPressEnter={() => {
        // Trigger the OK button of the modal
        const okBtn = document.querySelector(
          ".ant-modal-confirm-btns .ant-btn-primary"
        ) as HTMLButtonElement;
        if (okBtn) okBtn.click();
      }}
    />
  );
};

const FilesView: React.FC<FilesViewProps> = ({
  devices,
  selectedDevice,
  setSelectedDevice,
  fetchDevices,
  loading,
  currentPath,
  setCurrentPath,
  fileList,
  filesLoading,
  fetchFiles,
  showHiddenFiles,
  setShowHiddenFiles,
  clipboard,
  handleFileAction,
}) => {
  const { t } = useTranslation();
  const fileColumns = [
    {
      title: t("files.name"),
      key: "name",
      sorter: (a: any, b: any) => a.name.localeCompare(b.name),
      render: (_: any, record: any) => (
        <NameWithThumbnail
          deviceId={selectedDevice}
          record={record}
          onClick={() => {
            if (record.isDir) {
              fetchFiles(record.path);
            } else {
              handleFileAction("open", record);
            }
          }}
        />
      ),
    },
    {
      title: t("files.size"),
      dataIndex: "size",
      key: "size",
      width: 100,
      sorter: (a: any, b: any) => a.size - b.size,
      render: (size: number, record: any) =>
        record.isDir
          ? "-"
          : size > 1024 * 1024
          ? (size / (1024 * 1024)).toFixed(2) + " MB"
          : (size / 1024).toFixed(2) + " KB",
    },
    {
      title: t("files.time"),
      dataIndex: "modTime",
      key: "modTime",
      width: 180,
      defaultSortOrder: "descend" as const,
      sorter: (a: any, b: any) => {
        if (a.modTime === "N/A" && b.modTime === "N/A") return 0;
        if (a.modTime === "N/A") return -1;
        if (b.modTime === "N/A") return 1;
        return a.modTime.localeCompare(b.modTime);
      },
    },
    {
      title: t("files.action"),
      key: "action",
      width: 180,
      render: (_: any, record: any) => (
        <Space>
          {!record.isDir && (
            <Tooltip title={t("common.download")}>
              <Button
                size="small"
                icon={<VerticalAlignBottomOutlined />}
                onClick={() => handleFileAction("download", record)}
              />
            </Tooltip>
          )}
          <Tooltip title={t("common.copy")}>
            <Button
              size="small"
              icon={<CopyOutlined />}
              onClick={() => handleFileAction("copy", record)}
            />
          </Tooltip>
          <Tooltip title={t("common.cut")}>
            <Button
              size="small"
              icon={<ScissorOutlined />}
              onClick={() => handleFileAction("cut", record)}
            />
          </Tooltip>
          <Dropdown
            menu={{
              items: [
                {
                  key: "open",
                  label: t("files.open_on_host"),
                  disabled: record.isDir,
                  onClick: () => handleFileAction("open", record),
                },
                {
                  key: "rename",
                  label: t("files.rename"),
                  onClick: () => handleFileAction("rename", record),
                },
                {
                  key: "delete",
                  label: t("files.delete"),
                  danger: true,
                  onClick: () => {
                    Modal.confirm({
                      title: t("files.delete_confirm_title"),
                      okText: t("common.delete"),
                      okType: "danger",
                      cancelText: t("common.cancel"),
                      content: t("files.delete_confirm_content", {
                        name: record.name,
                      }),
                      onOk: () => handleFileAction("delete", record),
                    });
                  },
                },
              ],
            }}
          >
            <Button size="small" icon={<MoreOutlined />} />
          </Dropdown>
        </Space>
      ),
    },
  ];

  const filteredFiles = fileList.filter(
    (f) => showHiddenFiles || !f.name.startsWith(".")
  );

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
        <h2 style={{ margin: 0 }}>{t("files.title")}</h2>
        <DeviceSelector
          devices={devices}
          selectedDevice={selectedDevice}
          onDeviceChange={(val) => {
            setSelectedDevice(val);
            fetchFiles(currentPath);
          }}
          onRefresh={fetchDevices}
          loading={loading}
        />
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
        <div style={{ flex: 1, display: "flex", gap: 8 }}>
          <Button
            icon={<ArrowLeftOutlined />}
            onClick={() => {
              const parts = currentPath.split("/").filter(Boolean);
              parts.pop();
              fetchFiles("/" + parts.join("/"));
            }}
            disabled={currentPath === "/" || currentPath === ""}
          />
          <Input
            value={currentPath}
            onChange={(e) => setCurrentPath(e.target.value)}
            onPressEnter={() => fetchFiles(currentPath)}
            className="selectable"
            style={{ flex: 1, userSelect: "text" }}
          />
        </div>
        <Space>
          <Checkbox
            checked={showHiddenFiles}
            onChange={(e: any) => setShowHiddenFiles(e.target.checked)}
          >
            {t("files.show_hidden")}
          </Checkbox>
          <Button
            icon={<FolderAddOutlined />}
            onClick={() => handleFileAction("mkdir", null)}
          >
            {t("files.new_folder")}
          </Button>
          <Button
            icon={<SnippetsOutlined />}
            disabled={!clipboard}
            onClick={() => handleFileAction("paste", null)}
            type={clipboard ? "primary" : "default"}
          >
            {t("files.paste")}{" "}
            {clipboard &&
              `(${
                clipboard.type === "copy" ? t("common.copy") : t("common.cut")
              })`}
          </Button>
        </Space>
      </div>

      <div
        className="selectable"
        style={{
          flex: 1,
          overflow: "hidden",
          backgroundColor: "#fff",
          borderRadius: "4px",
          border: "1px solid #f0f0f0",
          display: "flex",
          flexDirection: "column",
          userSelect: "text",
        }}
      >
        <Table
          columns={fileColumns}
          dataSource={filteredFiles}
          rowKey="path"
          loading={filesLoading}
          pagination={false}
          size="small"
          scroll={{ y: "calc(100vh - 170px)" }}
          onRow={(record) => ({
            onDoubleClick: () => {
              if (record.isDir) {
                fetchFiles(record.path);
              } else {
                handleFileAction("open", record);
              }
            },
          })}
          style={{ flex: 1 }}
        />
      </div>
    </div>
  );
};

export { FocusInput };
export default FilesView;
