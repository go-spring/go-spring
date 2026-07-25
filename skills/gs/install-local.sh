#!/usr/bin/env bash
# install-local.sh — 从本地 go-spring 仓库安装 gs skill 到 Claude Code
#
# 用法（从 go-spring 仓库根目录运行）：
#   bash skills/gs/install-local.sh
#
# 行为：
#   将本地 skills/gs/ 完整镜像到 ~/.claude/skills/gs/
#   幂等：可重复运行覆盖更新

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SRC_DIR="$REPO_ROOT/skills/gs"
DST_DIR="${CLAUDE_SKILLS_DIR:-$HOME/.claude/skills}/gs"

if [ ! -d "$SRC_DIR" ]; then
    echo "错误: 找不到 $SRC_DIR，请从 go-spring 仓库根目录运行" >&2
    exit 1
fi

echo "=== gs skill 本地安装 ==="
echo "源: $SRC_DIR"
echo "目标: $DST_DIR"
echo ""

mkdir -p "$DST_DIR"

if command -v rsync >/dev/null 2>&1; then
    rsync -a --delete --exclude 'install-local.sh' "$SRC_DIR/" "$DST_DIR/"
else
    find "$DST_DIR" -mindepth 1 -delete
    (cd "$SRC_DIR" && find . -type f ! -name install-local.sh -print0 | \
        while IFS= read -r -d '' f; do
            mkdir -p "$DST_DIR/$(dirname "$f")"
            cp "$f" "$DST_DIR/$f"
        done)
fi

echo ""
echo "=== 安装完成 ==="
echo "gs skill 已安装到 $DST_DIR"
echo "在 Claude Code 中输入 /gs 或描述研发意图即可触发。"