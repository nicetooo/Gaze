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

# 1. 检查是否有未提交的改动
if [[ -n $(git status -s) ]]; then
    echo "📦 发现未提交的改动，正在自动提交..."
    git add .
    git commit -m "chore: release $VERSION"
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
