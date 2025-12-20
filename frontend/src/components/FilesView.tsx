import React, { useRef, useEffect } from "react";
import { Table, Button, Tag, Space, Tooltip, Input, Checkbox, Dropdown, Modal } from "antd";
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
} from "@ant-design/icons";
import DeviceSelector from "./DeviceSelector";

interface Device {
  id: string;
  state: string;
  model: string;
  brand: string;
}

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
  const fileColumns = [
    {
      title: "Name",
      key: "name",
      sorter: (a: any, b: any) => a.name.localeCompare(b.name),
      render: (_: any, record: any) => (
        <Space
          onClick={() => record.isDir && fetchFiles(record.path)}
          style={{
            cursor: record.isDir ? "pointer" : "default",
            width: "100%",
          }}
        >
          {record.isDir ? (
            <FolderOutlined style={{ color: "#1890ff" }} />
          ) : (
            <FileOutlined />
          )}
          <span>{record.name}</span>
        </Space>
      ),
    },
    {
      title: "Size",
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
      title: "Time",
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
      title: "Action",
      key: "action",
      width: 150,
      render: (_: any, record: any) => (
        <Space>
          <Tooltip title="Copy">
            <Button
              size="small"
              icon={<CopyOutlined />}
              onClick={() => handleFileAction("copy", record)}
            />
          </Tooltip>
          <Tooltip title="Cut">
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
                  key: "rename",
                  label: "Rename",
                  onClick: () => handleFileAction("rename", record),
                },
                {
                  key: "delete",
                  label: "Delete",
                  danger: true,
                  onClick: () => {
                    Modal.confirm({
                      title: "Delete File",
                      content: `Are you sure you want to delete ${record.name}?`,
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
        <h2 style={{ margin: 0 }}>Files</h2>
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
            Show Hidden
          </Checkbox>
          <Button
            icon={<FolderAddOutlined />}
            onClick={() => handleFileAction("mkdir", null)}
          >
            New Folder
          </Button>
          <Button
            icon={<SnippetsOutlined />}
            disabled={!clipboard}
            onClick={() => handleFileAction("paste", null)}
            type={clipboard ? "primary" : "default"}
          >
            Paste {clipboard && `(${clipboard.type === "copy" ? "Copy" : "Cut"})`}
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
            onDoubleClick: () => record.isDir && fetchFiles(record.path),
          })}
          style={{ flex: 1 }}
        />
      </div>
    </div>
  );
};

export { FocusInput };
export default FilesView;

