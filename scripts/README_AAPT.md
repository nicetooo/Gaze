# 下载 aapt 工具

aapt2 (Android Asset Packaging Tool 2) 用于从 APK 文件中提取应用名称和图标。

**推荐方法**：使用 Maven 仓库下载脚本（最简单可靠）

## 方法一：从 Google Maven 仓库下载（最推荐）⭐

这是最简单可靠的方法，从 Google 官方 Maven 仓库下载 aapt2：

### macOS / Linux

```bash
# 下载当前平台的 aapt2
./scripts/download_aapt_from_maven.sh

# 下载所有平台的 aapt2
./scripts/download_aapt_from_maven.sh all
```

### Windows

在 PowerShell 中运行：

```powershell
# 需要先安装 Git Bash 或 WSL，然后运行：
bash scripts/download_aapt_from_maven.sh all
```

**优势**：
- ✅ 直接从 Google 官方 Maven 仓库下载
- ✅ 版本最新（8.13.2-14304508）
- ✅ 支持所有平台（macOS、Linux、Windows）
- ✅ 自动提取和设置权限

## 方法二：使用旧版下载脚本

### macOS / Linux

```bash
./scripts/download_aapt.sh
```

### Windows

在 PowerShell 中运行：

```powershell
.\scripts\download_aapt.ps1
```

**注意**：如果脚本下载失败，请使用方法一或方法三。

## 方法二：从 Android SDK 复制（最可靠）

如果你已经安装了 Android SDK：

### macOS
```bash
# 查找 Android SDK 路径（通常在以下位置之一）
ANDROID_SDK=~/Library/Android/sdk
# 或
ANDROID_SDK=~/Android/Sdk

# 复制 aapt
cp $ANDROID_SDK/build-tools/*/aapt bin/darwin/aapt
chmod +x bin/darwin/aapt
```

### Linux
```bash
# 查找 Android SDK 路径
ANDROID_SDK=~/Android/Sdk
# 或
ANDROID_SDK=/opt/android-sdk

# 复制 aapt
cp $ANDROID_SDK/build-tools/*/aapt bin/linux/aapt
chmod +x bin/linux/aapt
```

### Windows
```powershell
# 查找 Android SDK 路径（通常在）
$ANDROID_SDK = "$env:LOCALAPPDATA\Android\Sdk"

# 复制 aapt.exe
Copy-Item "$ANDROID_SDK\build-tools\*\aapt.exe" "bin\windows\aapt.exe"
```

## 方法三：手动下载

1. 访问 Android SDK 下载页面：
   - https://developer.android.com/studio#command-tools
   - 或直接下载：https://dl.google.com/android/repository/

2. 下载对应平台的 build-tools（版本 33.0.0 或更高）：
   - macOS: `build-tools_r33.0.0-mac.zip`
   - Linux: `build-tools_r33.0.0-linux.zip`
   - Windows: `build-tools_r33.0.0-windows.zip`

3. 解压 zip 文件，找到 `aapt` (或 Windows 的 `aapt.exe`)

4. 复制到对应目录：
   ```bash
   # macOS
   cp <解压路径>/aapt bin/darwin/aapt
   chmod +x bin/darwin/aapt
   
   # Linux
   cp <解压路径>/aapt bin/linux/aapt
   chmod +x bin/linux/aapt
   
   # Windows
   copy <解压路径>\aapt.exe bin\windows\aapt.exe
   ```

## 方法四：使用 Homebrew (macOS)

```bash
# 安装 Android SDK
brew install --cask android-sdk

# 复制 aapt
cp ~/Library/Android/sdk/build-tools/*/aapt bin/darwin/aapt
chmod +x bin/darwin/aapt
```

## 验证

下载完成后，验证文件：

```bash
# macOS / Linux
ls -lh bin/darwin/aapt
ls -lh bin/linux/aapt
file bin/darwin/aapt  # 应该显示 "Mach-O" 或 "ELF"

# Windows
dir bin\windows\aapt.exe
```

文件大小应该约为 1-2 MB，不应该为 0 字节。

## 测试 aapt

```bash
# macOS / Linux
./bin/darwin/aapt version
# 或
./bin/linux/aapt version

# Windows
bin\windows\aapt.exe version
```

## 注意事项

- aapt 文件必须存在且可执行才能正常工作
- 如果 aapt 不存在，应用仍可正常运行，但无法显示应用图标和准确的应用名称
- 确保文件有执行权限（macOS/Linux）
- 不同版本的 aapt 功能基本相同，建议使用 33.0.0 或更高版本

## 故障排除

### 问题：下载脚本失败
- 检查网络连接
- 尝试手动下载（方法三）
- 使用已安装的 Android SDK（方法二）

### 问题：aapt 文件为 0 字节
- 删除空文件，重新下载
- 确保下载的是完整的二进制文件，不是 HTML 错误页面

### 问题：aapt 无法执行
- 检查文件权限：`chmod +x bin/darwin/aapt`
- 确认文件是二进制文件，不是文本文件
- 检查系统架构是否匹配（x86_64 vs arm64）
