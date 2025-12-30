import React, { useRef, useEffect, useState } from "react";
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
  message,
  theme,
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
import { 
  GetThumbnail, 
  ListFiles, 
  DeleteFile, 
  MoveFile, 
  CopyFile, 
  Mkdir, 
  OpenFileOnHost, 
  CancelOpenFile, 
  DownloadFile 
} from "../../wailsjs/go/main/App";
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
  initialPath?: string;
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
  initialPath = "/",
}) => {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  
  // Files state
  const [currentPath, setCurrentPath] = useState(initialPath);
  const [fileList, setFileList] = useState<any[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [clipboard, setClipboard] = useState<{
    path: string;
    type: "copy" | "cut";
  } | null>(null);
  const [showHiddenFiles, setShowHiddenFiles] = useState(false);

  useEffect(() => {
    if (initialPath && initialPath !== currentPath) {
      setCurrentPath(initialPath);
    }
  }, [initialPath]);

  const fetchFiles = async (path: string) => {
    if (!selectedDevice) return;
    setFilesLoading(true);
    try {
      const res = await ListFiles(selectedDevice, path);
      setFileList(res || []);
      setCurrentPath(path);
    } catch (err) {
      message.error(t("app.list_files_failed") + ": " + String(err));
    } finally {
      setFilesLoading(false);
    }
  };

  useEffect(() => {
    if (selectedDevice) {
      fetchFiles(currentPath);
    }
  }, [selectedDevice]);

  const handleFileAction = async (action: string, file: any) => {
    if (!selectedDevice) return;
    try {
      switch (action) {
        case "open":
          const openKey = `open_${file.path}`;
          message.loading({
            content: (
              <span>
                {t("app.launching", { name: file.name })}
                <Button
                  type="link"
                  size="small"
                  onClick={async () => {
                    await CancelOpenFile(file.path);
                    message.destroy(openKey);
                  }}
                >
                  {t("common.cancel")}
                </Button>
              </span>
            ),
            key: openKey,
            duration: 0,
          });
          try {
            await OpenFileOnHost(selectedDevice, file.path);
            message.destroy(openKey);
          } catch (err) {
            if (String(err).includes("cancelled")) {
              message.info(t("app.open_cancelled"), 2);
            } else {
              message.error(t("app.open_failed") + ": " + String(err), 3);
            }
            message.destroy(openKey);
          }
          break;
        case "download":
          try {
            const savePath = await DownloadFile(selectedDevice, file.path);
            if (savePath) {
              message.success(t("app.export_success", { path: savePath }));
            }
          } catch (err) {
            message.error(t("app.export_failed") + ": " + String(err));
          }
          break;
        case "delete":
          await DeleteFile(selectedDevice, file.path);
          message.success(t("app.delete_success", { name: file.name }));
          fetchFiles(currentPath);
          break;
        case "copy":
          setClipboard({ path: file.path, type: "copy" });
          message.info(t("app.copy_success", { name: file.name }));
          break;
        case "cut":
          setClipboard({ path: file.path, type: "cut" });
          message.info(t("app.cut_success", { name: file.name }));
          break;
        case "paste":
          if (!clipboard) return;
          const dest =
            currentPath +
            (currentPath.endsWith("/") ? "" : "/") +
            clipboard.path.split("/").pop();
          if (clipboard.type === "copy") {
            await CopyFile(selectedDevice, clipboard.path, dest);
            message.success(
              t("app.paste_success", { name: clipboard.path.split("/").pop() })
            );
          } else {
            await MoveFile(selectedDevice, clipboard.path, dest);
            message.success(
              t("app.move_success", { name: clipboard.path.split("/").pop() })
            );
            setClipboard(null);
          }
          fetchFiles(currentPath);
          break;
        case "rename":
          let newName = file.name;
          Modal.confirm({
            title: t("common.rename"),
            okText: t("common.ok"),
            cancelText: t("common.cancel"),
            autoFocusButton: null,
            content: (
              <FocusInput
                defaultValue={file.name}
                onChange={(e: any) => (newName = e.target.value)}
                placeholder={t("common.enter_new_name")}
                selectAll={true}
              />
            ),
            onOk: async () => {
              if (newName && newName !== file.name) {
                try {
                  const parentPath = currentPath.endsWith("/")
                    ? currentPath
                    : currentPath + "/";
                  const newPath = parentPath + newName;
                  await MoveFile(selectedDevice, file.path, newPath);
                  message.success(t("app.rename_success", { name: newName }));
                  fetchFiles(currentPath);
                } catch (err) {
                  message.error(t("app.rename_failed") + ": " + String(err));
                  throw err;
                }
              }
            },
          });
          break;
        case "mkdir":
          let folderName = "";
          Modal.confirm({
            title: t("common.new_directory"),
            okText: t("common.ok"),
            cancelText: t("common.cancel"),
            autoFocusButton: null,
            content: (
              <FocusInput
                onChange={(e: any) => (folderName = e.target.value)}
                placeholder={t("common.folder_name")}
              />
            ),
            onOk: async () => {
              if (folderName) {
                try {
                  const parentPath = currentPath.endsWith("/")
                    ? currentPath
                    : currentPath + "/";
                  const newPath = parentPath + folderName;
                  await Mkdir(selectedDevice, newPath);
                  message.success(t("app.mkdir_success", { name: folderName }));
                  fetchFiles(currentPath);
                } catch (err) {
                  message.error(t("app.mkdir_failed") + ": " + String(err));
                  throw err;
                }
              }
            },
          });
          break;
      }
    } catch (err) {
      message.error(t("app.command_failed") + ": " + String(err));
    }
  };

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
        <h2 style={{ margin: 0, color: token.colorText }}>{t("files.title")}</h2>
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
          backgroundColor: token.colorBgContainer,
          borderRadius: "4px",
          border: `1px solid ${token.colorBorderSecondary}`,
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

