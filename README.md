# Gaze

A powerful, modern, and self-contained Android device management and automation tool built with **Wails**, **React**, and **Ant Design**. Featuring a unified **Session-Event** architecture for complete device behavior tracking, a visual **Workflow** engine for test automation, and full **MCP** (Model Context Protocol) integration for AI-powered device control.

[English](README.md) | [简体中文](locales/README.zh-CN.md) | [繁體中文](locales/README.zh-TW.md) | [日本語](locales/README.ja.md) | [한국어](locales/README.ko.md) | [Español](locales/README.es.md) | [Português](locales/README.pt.md) | [Français](locales/README.fr.md) | [Deutsch](locales/README.de.md) | [Русский](locales/README.ru.md) | [Tiếng Việt](locales/README.vi.md) | [العربية](locales/README.ar.md)

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey.svg)
[![Website](https://img.shields.io/website?up_message=online&url=https%3A%2F%2Fgaze.nicetooo.com)](https://gaze.nicetooo.com)

## Why Gaze?

- **Modern & Fast**: Built with Wails (Go + React), providing a native-like experience with minimal resource overhead.
- **True Self-Contained**: No need to install `adb`, `scrcpy`, `aapt`, `ffmpeg`, or `ffprobe` on your system. Everything is bundled and ready to go.
- **Reliable File Transfer**: A robust alternative to the often-flaky *Android File Transfer* on macOS.
- **Multi-Device Power**: Supports independent, simultaneous background recording for multiple devices.
- **Session-Event Architecture**: Unified tracking of all device activities (logs, network, touch, app lifecycle) on a single timeline.
- **Visual Workflow Automation**: Build complex test flows with a drag-and-drop node editor — no code required.
- **AI-Ready via MCP**: 140+ tools exposed through the Model Context Protocol for seamless integration with AI clients like Claude Desktop and Cursor.
- **Developer First**: Integrated Logcat, Shell, MITM Proxy, and UI Inspector designed by developers, for developers.

## App Screenshots

| Device Management | Screen Mirroring |
|:---:|:---:|
| <img src="screenshots/devices.png" width="400" /> | <img src="screenshots/mirror.png" width="400" /> |

| File Manager | App Management |
|:---:|:---:|
| <img src="screenshots/files.png" width="400" /> | <img src="screenshots/apps.png" width="400" /> |

| Performance Monitor | Session Timeline |
|:---:|:---:|
| <img src="screenshots/performance.png" width="400" /> | <img src="screenshots/session.png" width="400" /> |

| Session List | Logcat Viewer |
|:---:|:---:|
| <img src="screenshots/session-list.png" width="400" /> | <img src="screenshots/logcat.png" width="400" /> |

| Visual Workflow Editor | Workflow List |
|:---:|:---:|
| <img src="screenshots/workflow-editor.png" width="400" /> | <img src="screenshots/workflow.png" width="400" /> |

| UI Inspector | Touch Recording |
|:---:|:---:|
| <img src="screenshots/ui-inspector.png" width="400" /> | <img src="screenshots/recording.png" width="400" /> |

| Network Proxy (MITM) | ADB Shell |
|:---:|:---:|
| <img src="screenshots/proxy.png" width="400" /> | <img src="screenshots/shell.png" width="400" /> |

---

## Features

### Device Management
- **Unified Device List**: Seamlessly manage physical and wireless devices with automatic USB/Wi-Fi merging.
- **Wireless Connection**: Connect effortlessly via IP/Port pairing with mDNS support.
- **Device History**: Quick access to previously connected offline devices.
- **Device Pinning**: Pin your most used device to always stay at the top of the list.
- **Device Monitoring**: Real-time tracking of battery, network, and screen state changes.
- **Batch Operations**: Execute operations across multiple devices simultaneously.

### App Management
- **Full Package Control**: Install (Drag & Drop), Uninstall, Enable, Disable, Force Stop, Clear Data.
- **APK Management**: Export installed APKs, Batch Install.
- **Smart Filtering**: Search and filter by System/User apps.
- **Quick Actions**: Launch apps or jump directly to their logs.

### Screen Mirroring (Scrcpy)
- **High Performance**: Low-latency mirroring powered by Scrcpy.
- **Recording**: Independent background recording with support for multiple devices simultaneously and one-click folder access.
- **Audio Forwarding**: Stream device audio to your computer (Android 11+).
- **Customization**: Adjust Resolution, Bitrate, FPS, and Codec (H.264/H.265).
- **Control**: Multi-touch support, Keep Awake, Screen Off mode.

### File Management
- **Full-Featured Explorer**: Browse, Copy, Cut, Paste, Rename, Delete, and Create Folders.
- **Drag & Drop**: Upload files by simply dragging them to the window.
- **Downloads**: Easy file transfer from device to computer.
- **Preview**: Open files directly on the host machine.

### Advanced Logcat
- **Real-time Streaming**: Live log viewer with auto-scroll control.
- **Powerful Filtering**: Filter by Log Level, Tag, PID, or custom Regex.
- **App-Centric**: Auto-filter logs for a specific application.
- **JSON Formatting**: Pretty-print detected JSON log segments.

### Network & Proxy (MITM)
- **Automated Capture**: One-click to start an HTTP/HTTPS proxy server and automatically configure device proxy settings via ADB.
- **HTTPS Decryption (MITM)**: Support for decrypting SSL traffic with automatic CA certificate generation and deployment.
- **WebSocket Support**: Capture and inspect real-time WebSocket traffic.
- **Big Data Handling**: Support for full body capture (up to 100MB) without truncation, with a 5000-entry log buffer.
- **Traffic Shaping**: Simulate real-world network conditions with per-device Download/Upload bandwidth limits and artificial latency.
- **Visual Metrics**: Real-time RX/TX speed monitoring for the selected device.

### Session & Event Tracking
- **Unified Event Pipeline**: All device activities (logs, network requests, touch events, app lifecycle, assertions) are captured as events and linked to a session timeline.
- **Automatic Session Management**: Sessions are created automatically when events occur, or manually with custom configurations (logcat, recording, proxy, monitoring).
- **Event Timeline**: Multi-lane visualization of all events with time-based indexing and navigation.
- **Full-Text Search**: Search across all events using SQLite FTS5.
- **Backpressure Control**: Automatic event sampling under high load while protecting critical events (errors, network, workflow).
- **Event Assertions**: Define and evaluate assertions against event streams for automated validation.
- **Video Sync**: Extract video frames synchronized to event timestamps for visual debugging.

### UI Inspector & Automation
- **UI Hierarchy Inspector**: Browse and analyze the complete UI tree of any screen.
- **Element Picker**: Click to select UI elements and inspect their properties (resource-id, text, bounds, class).
- **Touch Recording**: Record touch interactions and replay them as automation scripts.
- **Element-Based Actions**: Click, long-click, input text, swipe, wait, and assert on UI elements using selectors (id, text, contentDesc, className, xpath).

### Visual Workflow Engine
- **Node-Based Editor**: Build automation flows visually with a drag-and-drop interface powered by XYFlow.
- **30+ Step Types**: Tap, swipe, element interaction, app control, key events, screen control, wait, ADB commands, variables, branching, sub-workflows, and session control.
- **Conditional Branching**: Create intelligent flows with exists/not_exists/text_equals/text_contains conditions.
- **Variables & Expressions**: Use workflow variables with arithmetic expression support (`{{count}} + 1`).
- **Step-by-Step Debugging**: Pause, step through, and inspect variable state at each workflow step.
- **Session Integration**: Start/stop tracking sessions within workflows for comprehensive test reporting.

### ADB Shell
- **Integrated Console**: Run raw ADB commands directly within the app.
- **Command History**: Quick access to previously executed commands.

### System Tray
- **Quick Access**: Control mirroring and view device status from the menu bar/system tray.
- **Device Pinning**: Pin your primary device to appear at the top of the list and tray menu.
- **Tray Functions**: Direct access to Logcat, Shell, and File Manager for pinned devices from the tray.
- **Recording Indicators**: Visual red-dot indicator in the tray when recording is active.
- **Background Operation**: Keep the app running in the background for instant access.

---

## MCP Integration (Model Context Protocol)

Gaze includes a built-in **MCP server** that exposes 140+ tools and 5 resources, enabling AI clients to fully control Android devices through natural language. This makes Gaze the bridge between AI and Android.

### Supported AI Clients

| Client | Transport | Configuration |
|--------|-----------|---------------|
| **Claude Desktop** | stdio | `claude_desktop_config.json` |
| **Claude Code (CLI)** | stdio | `claude mcp add` |
| **Cursor** | stdio | Cursor MCP settings |

### Quick Setup

The MCP server runs over **stdio**: the AI client launches the Gaze binary with the `--mcp` flag as a subprocess. Adjust the binary path for your platform (the examples below use the macOS app bundle path).

**Claude Desktop** (`claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "gaze": {
      "command": "/Applications/Gaze.app/Contents/MacOS/Gaze",
      "args": ["--mcp"]
    }
  }
}
```

**Claude Code**:
```bash
claude mcp add gaze -- /Applications/Gaze.app/Contents/MacOS/Gaze --mcp
```

**Cursor**: Add a stdio MCP server with command `/Applications/Gaze.app/Contents/MacOS/Gaze` and argument `--mcp` in Cursor's MCP settings.

### MCP Tools (140+)

| Category | Example Tools | Description |
|----------|---------------|-------------|
| **Device** | `device_list`, `device_info`, `device_connect`, `device_disconnect`, `device_pair`, `device_wireless`, `device_ip` | Device discovery, connection, and information |
| **CLI Tools** | `adb_execute`, `aapt_execute`, `ffmpeg_execute`, `ffprobe_execute` | Execute bundled CLI tools (ADB, AAPT, FFmpeg, FFprobe) |
| **Files** | `file_upload`, `file_list` | Upload files to the device and browse device directories |
| **Apps** | `app_list`, `app_info`, `app_start`, `app_stop`, `app_running`, `app_install`, `app_uninstall`, `app_clear_data` | Full application lifecycle management |
| **Screen** | `screen_screenshot`, `screen_record_start`, `screen_record_stop`, `screen_recording_status` | Screenshots (base64) and recording control |
| **UI Automation** | `ui_hierarchy`, `ui_search`, `ui_tap`, `ui_swipe`, `ui_input`, `ui_wait_for`, `ui_resolution`, `keyboard_setup` | UI inspection, element interaction, waiting, and input |
| **Sessions** | `session_create`, `session_end`, `session_active`, `session_list`, `session_events`, `session_stats`, `session_export`, `session_import` | Session lifecycle and event querying |
| **Workflows** | `workflow_list`, `workflow_get`, `workflow_create`, `workflow_update`, `workflow_delete`, `workflow_run`, `workflow_stop`, `workflow_pause`, `workflow_resume`, `workflow_step_next`, `workflow_status`, `workflow_execute_step` | Full workflow CRUD, execution, and debugging |
| **Proxy & Network** | `proxy_start`, `proxy_stop`, `proxy_status`, `proxy_configure`, `proxy_device_setup`, `proxy_cert_install`, `mock_rule_*`, `rewrite_rule_*`, `breakpoint_*`, `map_remote_*`, `resend_request` | MITM proxy control, mocking, rewriting, breakpoints, and request replay |
| **Protobuf** | `proto_file_*`, `proto_mapping_*`, `proto_message_types`, `proto_load_url` | Protobuf definitions for decoding proxied traffic |
| **Touch Recording** | `touch_record_start`, `touch_record_stop`, `touch_script_list`, `touch_script_play`, `touch_script_save`, `touch_script_delete` | Record and replay touch interaction scripts |
| **Performance** | `perf_start`, `perf_stop`, `perf_snapshot`, `perf_process_detail` | CPU/memory/FPS/network/battery monitoring |
| **Assertions** | `assertion_*`, `assertion_set_*`, `assertion_quick_no_errors`, `assertion_quick_no_crashes` | Define and evaluate assertions against session events |
| **Plugins** | `plugin_list`, `plugin_create`, `plugin_update`, `plugin_delete`, `plugin_toggle`, `plugin_test*`, `plugin_sample_events` | JavaScript event-processing plugin management and testing |
| **Video** | `video_frame`, `video_metadata`, `session_video_frame`, `session_video_info` | Video frame extraction and metadata |

### MCP Resources

| URI | Description |
|-----|-------------|
| `gaze://devices` | List of connected devices |
| `gaze://devices/{deviceId}` | Detailed device information |
| `gaze://sessions` | Active and recent sessions |
| `workflow://list` | All saved workflows |
| `workflow://{workflowId}` | Workflow details with steps |

### What Can AI Do with Gaze?

With MCP integration, AI clients can:
- **Automate Testing**: Create and run UI test workflows through natural language instructions.
- **Debug Issues**: Take screenshots, inspect UI hierarchy, read logs, and analyze network traffic.
- **Manage Devices**: Install apps, transfer files, configure settings across multiple devices.
- **Build Workflows**: Generate complex automation workflows with branching logic and variable management.
- **Monitor Sessions**: Track device behavior over time with event-based session recording.

---

## Built-in Binaries

This application is fully self-contained. It bundles:
- **ADB** (Android Debug Bridge)
- **Scrcpy** (Screen mirroring & recording)
- **AAPT** (Android Asset Packaging Tool)
- **FFmpeg** (Video/audio processing)
- **FFprobe** (Media analysis)

At startup, these are extracted to a temporary directory and used automatically. You don't need to configure your system PATH.

---

## Important Notes for Xiaomi/Poco/Redmi Users

To enable **touch control** in Scrcpy, you must:
1. Go to **Developer Options**.
2. Enable **USB Debugging**.
3. Enable **USB Debugging (Security settings)**.
   *(Note: This requires a SIM card and Mi Account login on most Xiaomi devices).*

---

## Getting Started

### Prerequisites
- **Go** (v1.23+)
- **Node.js** (v18 LTS)
- **Wails CLI** (v2.9.2)
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.9.2
  ```

### Development
```bash
make dev
# Or directly: wails dev -tags fts5
```

### Build
```bash
make build
# Or directly: wails build -tags fts5
```
The compiled application will be available in `build/bin`.

**Important**: Always use `-tags fts5` to enable FTS5 full-text search. Without it, search functionality will fall back to slower LIKE queries.

### Running Tests
```bash
make test
# Or directly: go test -tags fts5 ./...
```

### Release
This project uses GitHub Actions to automate multi-platform builds. To create a new release:
1. Tag your commit: `git tag v1.0.0`
2. Push the tag: `git push origin v1.0.0`
The GitHub Action will automatically build for macOS, Windows, and Linux, and upload the artifacts to the Release page.

---

## Architecture Overview

```
                    +-----------------+
                    |   Wails (GUI)   |
                    +--------+--------+
                             |
              +--------------+--------------+
              |                             |
     +--------v--------+          +--------v--------+
     |  React Frontend |          |   Go Backend    |
     |  (Ant Design,   |          |  (App, Device,  |
     |   Zustand,      |          |   Automation,   |
     |   XYFlow)       |          |   Workflow)     |
     +-----------------+          +--------+--------+
                                           |
                         +-----------------+-----------------+
                         |                 |                 |
                +--------v------+  +-------v-------+  +-----v-------+
                | Event Pipeline|  |  MCP Server   |  |   Proxy     |
                | (Session,     |  |  (140+ tools, |  |  (MITM,     |
                |  SQLite,      |  |   5 resources)|  |   goproxy)  |
                |  FTS5)        |  +---------------+  +-------------+
                +---------------+
```

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Desktop Framework** | Wails v2 |
| **Backend** | Go 1.23+ |
| **Frontend** | React 18, TypeScript, Ant Design 6 |
| **State Management** | Zustand |
| **Workflow Editor** | XYFlow + Dagre |
| **Database** | SQLite (WAL mode, FTS5) |
| **Proxy** | goproxy |
| **MCP** | mcp-go (Model Context Protocol) |
| **i18n** | i18next (5 languages) |
| **Logging** | zerolog |
| **Charts** | Recharts |

---

## Troubleshooting

### macOS: "App is damaged and can't be opened"
If you download the app from GitHub and see the error *"Gaze.app is damaged and can't be opened"*, this is due to macOS Gatekeeper quarantine.

To fix this, run the following command in your terminal:
```bash
sudo xattr -cr /path/to/Gaze.app
```
*(Replace `/path/to/Gaze.app` with the actual path to your downloaded application)*

> **Or build it yourself:** If you prefer not to bypass Gatekeeper, you can easily [build the app from source](#getting-started) locally. It only takes a few minutes!

### Windows: "Windows protected your PC"
If you see a blue SmartScreen popup preventing the app from starting:
1. Click **More info**.
2. Click **Run anyway**.

---

## License
This project is licensed under the MIT License.
