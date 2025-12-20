#!/bin/bash
# Download aapt2 from Google Maven Repository

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$PROJECT_ROOT/bin"

# aapt2 version from Maven
VERSION="8.13.2-14304508"
BASE_URL="https://dl.google.com/dl/android/maven2/com/android/tools/build/aapt2/${VERSION}"

echo "Downloading aapt2 from Google Maven Repository (version $VERSION)..."

# Function to download and extract aapt2 for a platform
download_aapt2() {
    local platform=$1
    local maven_platform=$2
    local output_dir=$3
    local aapt_name=$4
    
    echo ""
    echo "=== Downloading aapt2 for $platform ==="
    
    local jar_file="/tmp/aapt2-${platform}.jar"
    local url="${BASE_URL}/aapt2-${VERSION}-${maven_platform}.jar"
    
    echo "Downloading from: $url"
    
    # Try with certificate bundle, fallback to insecure if needed
    if curl -L -f --cacert /etc/ssl/certs/ca-certificates.crt -o "$jar_file" "$url" 2>/dev/null || \
       curl -L -f --cacert /etc/ssl/cert.pem -o "$jar_file" "$url" 2>/dev/null || \
       curl -L -f -k -o "$jar_file" "$url"; then
        echo "Extracting aapt2 from jar..."
        
        # Create temp directory for extraction
        local temp_dir="/tmp/aapt2-extract-${platform}"
        rm -rf "$temp_dir"
        mkdir -p "$temp_dir"
        
        # Extract jar (which is a zip file)
        unzip -q "$jar_file" -d "$temp_dir" || {
            echo "Failed to extract jar file"
            rm -f "$jar_file"
            return 1
        }
        
        # Find aapt2 binary in extracted files
        local aapt2_binary=$(find "$temp_dir" -name "aapt2" -o -name "aapt2.exe" | head -1)
        
        if [ -n "$aapt2_binary" ] && [ -f "$aapt2_binary" ]; then
            # Copy to output directory
            cp "$aapt2_binary" "$output_dir/$aapt_name"
            
            # Make executable (for Unix-like systems)
            if [ "$platform" != "windows" ]; then
                chmod +x "$output_dir/$aapt_name"
            fi
            
            echo "✓ Successfully extracted aapt2 for $platform"
            rm -rf "$temp_dir"
            rm -f "$jar_file"
            return 0
        else
            echo "✗ aapt2 binary not found in jar file"
            rm -rf "$temp_dir"
            rm -f "$jar_file"
            return 1
        fi
    else
        echo "✗ Failed to download for $platform"
        rm -f "$jar_file"
        return 1
    fi
}

# Create bin directories
mkdir -p "$BIN_DIR/darwin"
mkdir -p "$BIN_DIR/linux"
mkdir -p "$BIN_DIR/windows"

# Download for current platform or all platforms
if [ "$1" = "all" ]; then
    # Download for all platforms
    download_aapt2 "darwin" "osx" "$BIN_DIR/darwin" "aapt" || echo "Warning: Failed for darwin"
    download_aapt2 "linux" "linux" "$BIN_DIR/linux" "aapt" || echo "Warning: Failed for linux"
    download_aapt2 "windows" "windows" "$BIN_DIR/windows" "aapt.exe" || echo "Warning: Failed for windows"
else
    # Download for current platform
    case "$(uname)" in
        Darwin*)
            download_aapt2 "darwin" "osx" "$BIN_DIR/darwin" "aapt" || exit 1
            ;;
        Linux*)
            download_aapt2 "linux" "linux" "$BIN_DIR/linux" "aapt" || exit 1
            ;;
        *)
            echo "Unknown platform. Use 'all' to download for all platforms."
            exit 1
            ;;
    esac
fi

echo ""
echo "Done! aapt2 binaries should now be in:"
echo "  - $BIN_DIR/darwin/aapt"
echo "  - $BIN_DIR/linux/aapt"
echo "  - $BIN_DIR/windows/aapt.exe"
