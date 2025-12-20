#!/bin/bash
# Helper script to copy aapt from Android SDK if installed

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$PROJECT_ROOT/bin"

# Common Android SDK paths
SDK_PATHS=(
    "$HOME/Library/Android/sdk"
    "$HOME/Android/Sdk"
    "/usr/local/android-sdk"
    "/opt/android-sdk"
    "$ANDROID_HOME"
    "$ANDROID_SDK_ROOT"
)

echo "Searching for Android SDK..."

for SDK_PATH in "${SDK_PATHS[@]}"; do
    if [ -d "$SDK_PATH" ]; then
        echo "Found Android SDK at: $SDK_PATH"
        
        # Find aapt in build-tools
        AAPT_PATH=$(find "$SDK_PATH/build-tools" -name "aapt" -type f 2>/dev/null | head -1)
        
        if [ -n "$AAPT_PATH" ]; then
            echo "Found aapt at: $AAPT_PATH"
            
            # Determine platform
            if [[ "$OSTYPE" == "darwin"* ]]; then
                TARGET="$BIN_DIR/darwin/aapt"
            elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
                TARGET="$BIN_DIR/linux/aapt"
            else
                echo "Unknown platform: $OSTYPE"
                exit 1
            fi
            
            echo "Copying to: $TARGET"
            cp "$AAPT_PATH" "$TARGET"
            chmod +x "$TARGET"
            
            echo "✓ Successfully copied aapt!"
            exit 0
        fi
    fi
done

echo "✗ Android SDK not found or aapt not available"
echo ""
echo "Please:"
echo "1. Install Android SDK, or"
echo "2. Run download_aapt.sh to download, or"
echo "3. Manually copy aapt to bin/darwin/aapt or bin/linux/aapt"
exit 1
