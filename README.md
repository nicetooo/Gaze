# ADB GUI 🚀

A powerful, modern, and self-contained Android management tool built with **Wails**, **React**, and **Ant Design**.

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows-lightgrey.svg)

## ✨ Features

### 📱 Device Management
- Real-time monitoring of connected devices.
- View device ID, model, and connection state.
- One-click access to Apps, Shell, Logcat, and Mirroring.

### 📦 App Management
- List all installed packages (System & User apps).
- Filter and search apps by name or type.
- **Actions**: Force Stop, Clear Data, Enable/Disable, and Uninstall.
- **Quick Logcat**: Jump to logs for a specific app directly from the app list.

### 🖥️ Screen Mirroring (Scrcpy)
- **Built-in Scrcpy**: No need to install anything externally.
- Fine-grained control over:
  - Video Bitrate & Max FPS.
  - Resolution (Max Size).
  - Stay Awake & Turn Screen Off options.
  - Always-on-top window.
  - Audio streaming toggle.

### 📜 Advanced Logcat
- Real-time log streaming with auto-scroll.
- **App-specific filtering**: Filter logs by a specific package name.
- **Auto-Monitoring**: Start logging before an app opens; the tool will automatically detect the PID and start filtering once the app launches.
- Keyword search/filtering.

### 💻 ADB Shell
- Integrated terminal for running ADB commands.
- Quick command execution with output history.

---

## 🛠️ Built-in Binaries

This application is fully self-contained. It bundles:
- **ADB** (Android Debug Bridge)
- **Scrcpy** executable
- **Scrcpy-server**

At startup, these are extracted to a temporary directory and used automatically. You don't need to configure your system PATH.

---

## ⚠️ Important Notes for Xiaomi/Poco/Redmi Users

To enable **touch control** in Scrcpy, you must:
1. Go to **Developer Options**.
2. Enable **USB Debugging**.
3. Enable **USB Debugging (Security settings)**. 
   *(Note: This requires a SIM card and Mi Account login on most Xiaomi devices).*

---

## 🚀 Getting Started

### Prerequisites
- **Go** (v1.21+)
- **Node.js** (v18+)
- **Wails CLI** (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

### Development
```bash
wails dev
```

### Build
```bash
wails build
```
The compiled application will be available in `build/bin`.

### Release
This project uses GitHub Actions to automate multi-platform builds. To create a new release:
1. Tag your commit: `git tag v1.0.0`
2. Push the tag: `git push origin v1.0.0`
The GitHub Action will automatically build for macOS, Windows, and Linux, and upload the artifacts to the Release page.

---

## 📄 License
This project is licensed under the MIT License.
