#!/bin/bash

# 检查是否输入了版本号
if [ -z "$1" ]; then
    echo "错误: 请提供版本号 (例如: ./release.sh v1.0.0)"
    exit 1
fi

VERSION=$1

# 确保版本号以 v 开头
if [[ ! $VERSION =~ ^v ]]; then
    VERSION="v$VERSION"
fi

echo "🚀 准备发布版本: $VERSION"

# 1. 检查是否有未提交的改动（仅自动提交已跟踪文件，避免把未跟踪的大文件/临时文件扫进发布提交）
if [[ -n $(git status -s) ]]; then
    echo "📦 发现未提交的改动，正在自动提交（仅已跟踪文件）..."
    git add -u
    if [[ -n $(git diff --cached --name-only) ]]; then
        git commit -m "chore: release $VERSION"
    fi
fi

# 提醒未跟踪文件不会进入发布提交
if [[ -n $(git status -s) ]]; then
    echo "⚠️  以下未跟踪/未提交文件不会包含在本次发布中，如需提交请手动 git add 后重新运行："
    git status -s
fi

# 2. 检查标签是否已存在
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo "⚠️  标签 $VERSION 已存在，正在删除本地和远程标签以重新发布..."
    git tag -d "$VERSION"
    git push origin --delete "$VERSION" >/dev/null 2>&1
fi

# 3. 创建新标签
echo "🏷️  创建标签 $VERSION..."
git tag "$VERSION"

# 4. 推送到远程
echo "📤 推送到 GitHub..."
git push origin main
git push origin "$VERSION"

echo "✅ 发布指令已发出！请前往 GitHub Actions 查看构建进度。"
