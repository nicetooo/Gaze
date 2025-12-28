# Proxy & Network Monitoring Architecture

This document describes the technical implementation of the proxy and network monitoring features in `adbGUI`.

## Overview

The Network & Proxy module provides integrated HTTP/HTTPS traffic capture, SSL decryption (MITM), and network traffic shaping. It is designed to be "zero-config" for Android developers by automating the device-side setup.

## Core Components

### 1. Proxy Server (`proxy/proxy.go`)
The backend uses a customized version of `github.com/elazarl/goproxy`.
- **Customizations**:
    - **Header Mirroring**: Headers are captured without consuming the body.
    - **Body Mirroring**: Uses a `TransparentReadCloser` (custom `io.ReadCloser` wrapper) to copy data to a buffer as it's being read by the client/server, avoiding unnecessary memory copies.
    - **Decompression**: Automatically handles `gzip`, `deflate`, and `br` (Brotli) to provide readable body previews.
    - **Image Handling**: To optimize memory and performance, image bodies are not captured by default but are traced for metadata (size/headers).

### 2. HTTPS Decryption (MITM)
- **CA Generation**: The app generates a unique CA certificate in the user's config directory.
- **Dynamic Certs**: For every HTTPS host, a leaf certificate is generated on-the-fly, signed by the internal CA.
- **Bypass Rules**: Users can define keywords (e.g., `cdn`, `static`) to bypass MITM for specific domains, which is crucial for handling Certificate Pinning in apps like Instagram or Facebook.

### 3. Automated Device Configuration
- **ADB Automation**: When capture starts, the app executes `settings put global http_proxy <ip>:<port>` via ADB.
- **Permission Handling**: The app detects security restrictions (common on Xiaomi/HyperOS) and provides guided instructions for the user to enable "USB Debugging (Security Settings)".
- **Cert Deployment**: The CA certificate is pushed to `/sdcard/Download/` via ADB for easy manual installation by the user.

### 4. Traffic Shaping & Monitoring
- **Bandwidth Throttling**: Uses `golang.org/x/time/rate` to implement per-connection and global download/upload limits.
- **Artificial Latency**: Injects precise delays into the request/response pipeline to simulate unstable networks (3G/4G/High-latency Wi-Fi).
- **Metric Collection**: Aggregates throughput data every 512KB or 2 seconds to minimize IPC overhead with the frontend.

### 5. Frontend & Visualization (`ProxyView.tsx`)
- **Virtual Scrolling**: Uses `@tanstack/react-virtual` to handle 5000+ log entries smoothly.
- **Real-time Updates**: Status codes and body sizes are updated reactively as data flows through the proxy.
- **Full Body Viewer**: Supports viewing up to 100MB of response data with JSON formatting and "Copy to Clipboard" functionality.

## Data Flow

1. **Request Phase**:
   - Proxy receives request.
   - Generates unique ID.
   - If MITM enabled and not bypassed, hijacks connection.
   - Logs request headers and emits event to UI.
2. **Transfer Phase**:
   - `TransparentReadCloser` wraps the body stream.
   - As chunks are read, they are added to a `bytes.Buffer`.
   - Update events are throttled and sent to UI for large transfers.
3. **Response Phase**:
   - Analyzes `Content-Type`.
   - If non-binary (text/JSON/HTML), performs decompression and updates UI log entry.
   - Finalizes metrics (total size, duration).

## Limitations

- **Android 11+ System Certs**: Modern Android apps (target SDK 24+) do not trust user-added CAs by default. Users must either use a rooted device to move the cert to system store or modify the app's `network_security_config.xml`.
- **Certificate Pinning**: Even with CA installed, many high-security apps (Instagram, Facebook) will still fail due to Hardcoded Pinning. Use "Bypass Rules" to at least allow these apps to function (without decryption) while monitoring other traffic.
