import React from "react";
import { Modal, Button, Row, Col, Card, Tabs, Input } from "antd";
import { ReloadOutlined, AppstoreOutlined, PlayCircleOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
// @ts-ignore
import { main } from "../../wailsjs/go/models";

interface AppInfoModalProps {
  visible: boolean;
  onCancel: () => void;
  selectedAppInfo: main.AppPackage | null;
  infoLoading: boolean;
  handleFetchAppInfo: (packageName: string, force?: boolean) => Promise<void>;
  permissionSearch: string;
  setPermissionSearch: (val: string) => void;
  activitySearch: string;
  setActivitySearch: (val: string) => void;
  handleStartActivity: (activityName: string) => Promise<void>;
  getContainer?: string | (() => HTMLElement) | false;
}

const AppInfoModal: React.FC<AppInfoModalProps> = ({
  visible,
  onCancel,
  selectedAppInfo,
  infoLoading,
  handleFetchAppInfo,
  permissionSearch,
  setPermissionSearch,
  activitySearch,
  setActivitySearch,
  handleStartActivity,
  getContainer,
}) => {
  const { t } = useTranslation();
  return (
    <Modal
      getContainer={getContainer}
      centered
      style={{ top: 0, paddingBottom: 0 }}
      styles={{
        wrapper: { position: "absolute", overflow: "hidden" },
        mask: { position: "absolute" },
        body: { overflowY: "auto", maxHeight: "calc(80vh - 120px)" }
      }}
      title={
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            paddingRight: 32,
          }}
        >
          <div style={{ display: "flex", flexDirection: "column", maxWidth: "70%" }}>
            <span style={{ fontSize: 16, fontWeight: 600, lineHeight: 1.2 }}>
              {selectedAppInfo ? (selectedAppInfo.label || selectedAppInfo.name) : t("app_info.title")}
            </span>
            {selectedAppInfo && (
              <span style={{ fontSize: 12, color: "#888", fontWeight: "normal", marginTop: 2 }}>
                {selectedAppInfo.name}
              </span>
            )}
          </div>
          {selectedAppInfo && (
            <Button
              size="small"
              icon={<ReloadOutlined spin={infoLoading} />}
              onClick={() => handleFetchAppInfo(selectedAppInfo.name, true)}
              disabled={infoLoading}
            >
              {t("common.refresh")}
            </Button>
          )}
        </div>
      }
      open={visible}
      onCancel={onCancel}
      footer={[
        <Button key="close" onClick={onCancel}>
          {t("common.close")}
        </Button>,
      ]}
      width={600}
    >
      {infoLoading && !selectedAppInfo?.versionName ? (
        <div style={{ padding: "60px 0", textAlign: "center" }}>
          <ReloadOutlined
            spin
            style={{ fontSize: 32, color: "#1890ff", marginBottom: 16 }}
          />
          <div style={{ fontSize: 16, color: "#666" }}>
            {t("app_info.fetching")}
          </div>
          <div style={{ fontSize: 12, color: "#999", marginTop: 8 }}>
            {t("app_info.fetching_desc")}
          </div>
        </div>
      ) : (
        selectedAppInfo && (
          <div
            className="selectable"
            style={{
              paddingTop: 8,
              opacity: infoLoading ? 0.6 : 1,
              transition: "opacity 0.3s",
              userSelect: "text",
            }}
          >
            {infoLoading && (
              <div
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  right: 0,
                  bottom: 0,
                  zIndex: 10,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  background: "rgba(255,255,255,0.7)",
                }}
              >
                <ReloadOutlined spin style={{ fontSize: 24, color: "#1890ff" }} />
              </div>
            )}
            
            {/* 紧凑的信息展示区 */}
            <div style={{ 
              marginBottom: 16, 
              padding: "8px 12px", 
              background: "#f5f5f5", 
              borderRadius: 6,
              fontSize: 13,
              display: "flex",
              flexWrap: "wrap",
              gap: "12px 24px"
            }}>
               <div style={{ display: "flex", alignItems: "center" }}>
                  <span style={{ color: "#666", marginRight: 4 }}>Version:</span>
                  <span style={{ fontWeight: 500 }}>
                    {selectedAppInfo.versionName || "N/A"} 
                    <span style={{ color: "#999", marginLeft: 4 }}>({selectedAppInfo.versionCode || 0})</span>
                  </span>
               </div>
               <div style={{ display: "flex", alignItems: "center" }}>
                  <span style={{ color: "#666", marginRight: 4 }}>SDK:</span>
                  <span style={{ fontWeight: 500 }}>
                    Min {selectedAppInfo.minSdkVersion || "?"} / Target {selectedAppInfo.targetSdkVersion || "?"}
                  </span>
               </div>
            </div>
                <Tabs
                  defaultActiveKey="permissions"
                  type="card"
                  size="small"
                  items={[
                    {
                      key: "permissions",
                      label: `${t("app_info.permissions")} (${
                        selectedAppInfo.permissions?.length || 0
                      })`,
                      children: (
                        <Card
                          size="small"
                          title={
                            <div
                              style={{
                                display: "flex",
                                justifyContent: "space-between",
                                alignItems: "center",
                              }}
                            >
                              <span>{t("app_info.permissions")}</span>
                              <Input
                                placeholder={t("app_info.search_permissions")}
                                size="small"
                                style={{ width: 200 }}
                                allowClear
                                value={permissionSearch}
                                onChange={(e) =>
                                  setPermissionSearch(e.target.value)
                                }
                              />
                            </div>
                          }
                        >
                          <div style={{ maxHeight: 300, overflowY: "auto" }}>
                            {selectedAppInfo.permissions &&
                            selectedAppInfo.permissions.length > 0 ? (
                              selectedAppInfo.permissions
                                .filter((p) =>
                                  p
                                    .toLowerCase()
                                    .includes(permissionSearch.toLowerCase())
                                )
                                .map((p, i) => (
                                  <div
                                    key={i}
                                    style={{
                                      fontSize: 12,
                                      padding: "4px 8px",
                                      borderBottom: "1px solid #f0f0f0",
                                    }}
                                  >
                                    {p.replace("android.permission.", "")}
                                  </div>
                                ))
                            ) : (
                              <p
                                style={{
                                  color: "#bfbfbf",
                                  fontStyle: "italic",
                                  textAlign: "center",
                                  padding: "20px 0",
                                }}
                              >
                                {t("app_info.no_permissions")}
                              </p>
                            )}
                          </div>
                        </Card>
                      ),
                    },
                    {
                      key: "activities",
                      label: `${t("app_info.activities")} (${
                        selectedAppInfo.activities?.length || 0
                      })`,
                      children: (
                        <Card
                          size="small"
                          title={
                            <div
                              style={{
                                display: "flex",
                                justifyContent: "space-between",
                                alignItems: "center",
                              }}
                            >
                              <span>{t("app_info.activities")}</span>
                              <Input
                                placeholder={t("app_info.search_activities")}
                                size="small"
                                style={{ width: 200 }}
                                allowClear
                                value={activitySearch}
                                onChange={(e) =>
                                  setActivitySearch(e.target.value)
                                }
                              />
                            </div>
                          }
                        >
                          <div style={{ maxHeight: 300, overflowY: "auto" }}>
                            {selectedAppInfo.activities &&
                            selectedAppInfo.activities.length > 0 ? (
                              selectedAppInfo.activities
                                .filter((a) =>
                                  a
                                    .toLowerCase()
                                    .includes(activitySearch.toLowerCase())
                                )
                                .sort((a, b) => {
                                  const aLaunchable =
                                    selectedAppInfo.launchableActivities?.includes(
                                      a
                                    );
                                  const bLaunchable =
                                    selectedAppInfo.launchableActivities?.includes(
                                      b
                                    );
                                  if (aLaunchable && !bLaunchable) return -1;
                                  if (!aLaunchable && bLaunchable) return 1;
                                  return a.localeCompare(b);
                                })
                                .map((a, i) => (
                                  <div
                                    key={i}
                                    style={{
                                      fontSize: 12,
                                      padding: "6px 8px",
                                      borderBottom: "1px solid #f0f0f0",
                                      display: "flex",
                                      justifyContent: "space-between",
                                      alignItems: "center",
                                    }}
                                  >
                                    <span
                                      style={{
                                        fontFamily: "monospace",
                                        wordBreak: "break-all",
                                        marginRight: 8,
                                      }}
                                    >
                                      {a.includes("/") ? a.split("/")[1] : a}
                                    </span>
                                    {selectedAppInfo.launchableActivities?.includes(
                                      a
                                    ) && (
                                      <Button
                                        size="small"
                                        icon={<PlayCircleOutlined />}
                                        onClick={() => handleStartActivity(a)}
                                      >
                                        {t("app_info.launch")}
                                      </Button>
                                    )}
                                  </div>
                                ))
                            ) : (
                              <p
                                style={{
                                  color: "#bfbfbf",
                                  fontStyle: "italic",
                                  textAlign: "center",
                                  padding: "20px 0",
                                }}
                              >
                                {t("app_info.no_activities")}
                              </p>
                            )}
                          </div>
                        </Card>
                      ),
                    },
                  ]}
                />
              </div>
        )
      )}
    </Modal>
  );
};

export default AppInfoModal;

