#!/bin/bash

# Script to download aapt from Android SDK build-tools and extract it
# This script downloads aapt for all platforms and places them in the bin/ directory

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$PROJECT_ROOT/bin"

# Create bin directories if they don't exist
mkdir -p "$BIN_DIR/darwin"
mkdir -p "$BIN_DIR/linux"
mkdir -p "$BIN_DIR/windows"

# Android SDK build-tools version
VERSION="33.0.0"
BASE_URL="https://dl.google.com/android/repository"

echo "Downloading aapt from Android SDK build-tools $VERSION..."

# Function to download and extract aapt for a platform
download_aapt() {
    local platform=$1
    local os_name=$2
    local output_dir=$3
    local aapt_name=$4
    
    echo ""
    echo "=== Downloading aapt for $platform ==="
    
    local zip_file="/tmp/build-tools-${platform}.zip"
    local url="${BASE_URL}/build-tools_r${VERSION}-${os_name}.zip"
    
    echo "Downloading from: $url"
    curl -L -k -o "$zip_file" "$url" || {
        echo "Failed to download for $platform. Trying alternative method..."
        # Alternative: try direct download if available
        return 1
    }
    
    echo "Extracting aapt from zip..."
    
    # Extract aapt from zip
    # The zip structure is: android-{version}/aapt or build-tools_r{version}-{os}/aapt
    if unzip -l "$zip_file" | grep -q "aapt"; then
        # Try to find and extract aapt
        unzip -j "$zip_file" "*/aapt" -d "$output_dir" 2>/dev/null || \
        unzip -j "$zip_file" "*/aapt.exe" -d "$output_dir" 2>/dev/null || {
            echo "Could not extract aapt directly, trying to find it in zip..."
            # List files to find aapt
            unzip -l "$zip_file" | grep aapt | head -1 | awk '{print $4}' | while read aapt_path; do
                if [ -n "$aapt_path" ]; then
                    unzip -j "$zip_file" "$aapt_path" -d "$output_dir"
                    # Rename if needed
                    if [ -f "$output_dir/aapt" ] && [ "$aapt_name" != "aapt" ]; then
                        mv "$output_dir/aapt" "$output_dir/$aapt_name"
                    fi
                    break
                fi
            done
        }
        
        # Rename to correct name if needed
        if [ -f "$output_dir/aapt" ] && [ "$aapt_name" != "aapt" ]; then
            mv "$output_dir/aapt" "$output_dir/$aapt_name"
        fi
        
        # Make executable (for Unix-like systems)
        if [ "$platform" != "windows" ]; then
            chmod +x "$output_dir/$aapt_name"
        fi
        
        echo "✓ Successfully extracted aapt for $platform"
        rm -f "$zip_file"
    else
        echo "✗ aapt not found in zip file for $platform"
        rm -f "$zip_file"
        return 1
    fi
}

# Download for macOS
if [ "$(uname)" = "Darwin" ] || [ "$1" = "all" ]; then
    download_aapt "darwin" "mac" "$BIN_DIR/darwin" "aapt" || echo "Warning: Failed to download aapt for darwin"
fi

# Download for Linux
if [ "$(uname)" = "Linux" ] || [ "$1" = "all" ]; then
    download_aapt "linux" "linux" "$BIN_DIR/linux" "aapt" || echo "Warning: Failed to download aapt for linux"
fi

# Download for Windows (can be run on any platform if "all" is specified)
if [ "$1" = "all" ]; then
    download_aapt "windows" "windows" "$BIN_DIR/windows" "aapt.exe" || echo "Warning: Failed to download aapt for windows"
fi

echo ""
echo "Done! aapt binaries should now be in:"
echo "  - $BIN_DIR/darwin/aapt"
echo "  - $BIN_DIR/linux/aapt"
echo "  - $BIN_DIR/windows/aapt.exe"
echo ""
echo "If any platform failed, you can:"
echo "1. Install Android SDK and copy aapt from build-tools directory"
echo "2. Download manually from: $BASE_URL"
echo "3. Run this script again with 'all' argument to download for all platforms"

