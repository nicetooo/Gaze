import React from "react";
import { Modal, Button, Row, Col, Card, Tabs, Input } from "antd";
import { ReloadOutlined, AppstoreOutlined, PlayCircleOutlined } from "@ant-design/icons";
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
}) => {
  return (
    <Modal
      title={
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            paddingRight: 32,
          }}
        >
          <span>App Information</span>
          {selectedAppInfo && (
            <Button
              size="small"
              icon={<ReloadOutlined spin={infoLoading} />}
              onClick={() => handleFetchAppInfo(selectedAppInfo.name, true)}
              disabled={infoLoading}
            >
              Refresh
            </Button>
          )}
        </div>
      }
      open={visible}
      onCancel={onCancel}
      footer={[
        <Button key="close" onClick={onCancel}>
          Close
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
            Fetching detailed app information...
          </div>
          <div style={{ fontSize: 12, color: "#999", marginTop: 8 }}>
            This may take a few seconds as we extract data from the APK
          </div>
        </div>
      ) : (
        selectedAppInfo && (
          <div
            className="selectable"
            style={{
              padding: "10px 0",
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
                  flexDirection: "column",
                  alignItems: "center",
                  justifyContent: "center",
                  background: "rgba(255,255,255,0.7)",
                  borderRadius: 8,
                }}
              >
                <ReloadOutlined
                  spin
                  style={{ fontSize: 32, color: "#1890ff", marginBottom: 12 }}
                />
                <div
                  style={{
                    fontSize: 14,
                    color: "#1890ff",
                    fontWeight: "bold",
                  }}
                >
                  Refreshing App Info...
                </div>
                <div style={{ fontSize: 12, color: "#666", marginTop: 4 }}>
                  Extracting data from APK, please wait
                </div>
              </div>
            )}
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: 20,
                marginBottom: 24,
              }}
            >
              <div
                style={{
                  width: 64,
                  height: 64,
                  borderRadius: 12,
                  backgroundColor: "#f0f0f0",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  overflow: "hidden",
                  flexShrink: 0,
                  boxShadow: "0 2px 8px rgba(0,0,0,0.1)",
                }}
              >
                {selectedAppInfo.icon ? (
                  <img
                    src={selectedAppInfo.icon}
                    style={{
                      width: "100%",
                      height: "100%",
                      objectFit: "cover",
                    }}
                    alt=""
                  />
                ) : (
                  <AppstoreOutlined
                    style={{ fontSize: 32, color: "#bfbfbf" }}
                  />
                )}
              </div>
              <div>
                <h3 style={{ margin: 0, fontSize: 20 }}>
                  {selectedAppInfo.label || selectedAppInfo.name}
                </h3>
                <code style={{ fontSize: 12, color: "#888" }}>
                  {selectedAppInfo.name}
                </code>
              </div>
            </div>

            <Row gutter={[16, 16]}>
              <Col span={12}>
                <Card size="small" title="Version Info">
                  <p>
                    <strong>Version Name:</strong>{" "}
                    {selectedAppInfo.versionName || "N/A"}
                  </p>
                  <p>
                    <strong>Version Code:</strong>{" "}
                    {selectedAppInfo.versionCode || "N/A"}
                  </p>
                </Card>
              </Col>
              <Col span={12}>
                <Card size="small" title="SDK Info">
                  <p>
                    <strong>Min SDK:</strong>{" "}
                    {selectedAppInfo.minSdkVersion || "N/A"}
                  </p>
                  <p>
                    <strong>Target SDK:</strong>{" "}
                    {selectedAppInfo.targetSdkVersion || "N/A"}
                  </p>
                </Card>
              </Col>
              <Col span={24}>
                <Tabs
                  defaultActiveKey="permissions"
                  type="card"
                  size="small"
                  items={[
                    {
                      key: "permissions",
                      label: `Permissions (${
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
                              <span>Permissions</span>
                              <Input
                                placeholder="Search permissions..."
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
                                No permissions listed
                              </p>
                            )}
                          </div>
                        </Card>
                      ),
                    },
                    {
                      key: "activities",
                      label: `Activities (${
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
                              <span>Activities</span>
                              <Input
                                placeholder="Search activities..."
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
                                        Launch
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
                                No activities listed
                              </p>
                            )}
                          </div>
                        </Card>
                      ),
                    },
                  ]}
                />
              </Col>
            </Row>
          </div>
        )
      )}
    </Modal>
  );
};

export default AppInfoModal;

