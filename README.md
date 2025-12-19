# ADB GUI

A GUI application for ADB (Android Debug Bridge) built with Wails, React, and Ant Design.

## Features

- **Devices**: View connected devices, their state, and model.
- **Shell**: Run ADB shell commands directly from the GUI.
- **Built-in ADB**: The application comes with a bundled ADB binary, so no external installation is required (currently macOS only).
- **Apps**: (Coming Soon) Manage installed applications.

## Prerequisites

- **Go** (v1.18+)
- **Node.js** (v14+)
- **ADB**: Ensure `adb` is installed and in your system PATH.

## Development

To run in development mode:

```bash
wails dev
```

## Build

To build the application:

```bash
wails build
```

The binary will be in `build/bin`.
