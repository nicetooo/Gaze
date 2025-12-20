# ADB GUI 🚀

一個強大、現代且自包含的 Android 管理工具，基於 **Wails**、**React** 和 **Ant Design** 構建。

> ✨ **注意**：本應用為純 **vibecoding** 產物。

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey.svg)

## ✨ 功能特性

### 📱 裝置管理
- 實時監控已連接裝置。
- 查看裝置 ID、型號和連接狀態。
- 一鍵存取應用管理、Shell、Logcat 和螢幕投屏。

### 📦 應用管理
- 列出所有已安裝的包（系統應用和用戶應用）。
- 按名稱或類型過濾和搜尋應用。
- **操作**：強制停止、清除數據、啟用/停用和解除安裝。
- **快速 Logcat**：直接從應用列表跳轉到特定應用的日誌。

### 🖥️ 螢幕投屏 (Scrcpy)
- **內建 Scrcpy**：無需額外安裝任何軟體。
- 精細控制：
  - 影片位元率和最大 FPS。
  - 解析度（最大尺寸）。
  - 保持喚醒和關閉螢幕選項。
  - 視窗置頂。
  - 音訊流開關。

### 📜 高級 Logcat
- 帶有自動捲動的實時日誌流。
- **特定應用過濾**：按特定包名過濾日誌。
- **自動監控**：在應用開啟前開始記錄；工具會自動檢測 PID 並在此應用啟動後開始過濾。
- 關鍵字搜尋/過濾。

### 💻 ADB Shell
- 集成終端，用於運行 ADB 命令。
- 快速執行命令並帶有輸出歷史記錄。

---

## 🛠️ 內建執行檔

本應用完全自包含。它捆綁了：
- **ADB** (Android Debug Bridge)
- **Scrcpy** 執行檔
- **Scrcpy-server**

啟動時，這些文件會被提取到臨時目錄並自動使用。您無需配置系統環境變數。

---

## ⚠️ 小米/Poco/紅米用戶重要提示

要在 Scrcpy 中啟用**觸控控制**，您必須：
1. 進入 **開發者選項**。
2. 啟用 **USB 偵錯**。
3. 啟用 **USB 偵錯（安全設置）**。
   *(注意：在大多數小米裝置上，這需要插入 SIM 卡並登入小米帳號)*。

---

## 🚀 開始使用

### 前置條件
- **Go** (v1.21+)
- **Node.js** (v18+)
- **Wails CLI** (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

### 開發
```bash
wails dev
```

### 構建
```bash
wails build
```
編譯後的應用將位於 `build/bin` 目錄。

### 發佈
本項目使用 GitHub Actions 自動進行多平台構建。要建立新版本：
1. 為您的提交打標籤：`git tag v1.0.0`
2. 推送標籤：`git push origin v1.0.0`
3. GitHub Action 將自動為 macOS、Windows 和 Linux 構建，並將產物上傳到 Release 頁面。

---

## 📄 許可證
本項目採用 MIT 許可證。

