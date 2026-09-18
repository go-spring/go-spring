#!/usr/bin/env bash
#
# 维护者工具:一次跑完所有**只读**校验。人工与 CI 共用同一个入口。
#
# 为什么需要它:各 checker 都是独立脚本,散着跑等于没有门禁 —— 漏跑一个没人会知道。
# 一个入口才有"这次提交过了没有"的单一答案。
#
# **本脚本只跑只读检查。** 有两个脚本故意不在里面,别顺手加进来:
#   - scripts/check-go-modules.sh —— 它会就地 go fix / modernize **改写源码**,是"修"不是"查";
#     放进预提交或 CI 会静默改工作区,让"检查通过"与"文件没变"两件事混为一谈。
#   - scripts/versions.sh —— BOM 版本治理,属发布流程的一步,不是每次提交的门禁。
#
# 用法: bash scripts/check-all.sh
set -uo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.." || exit 1

failed=()

run() {
  local name="$1" desc="$2"; shift 2
  printf '── %s —— %s\n' "$name" "$desc"
  if "$@" > /tmp/check-all.out 2>&1; then
    printf '   ✓ %s\n' "$name"
  else
    local rc=$?
    printf '   ✗ %s(退出码 %s),输出:\n' "$name" "$rc"
    sed 's/^/     | /' /tmp/check-all.out | tail -40
    failed+=("$name")
  fi
}

# 便宜的排前面:格式不对是最常见的,先出结果。
run gofmt          "所有 Go 源码的格式"   bash scripts/gofmt-all.sh --check
run config-namespace "配置命名空间规约"   bash scripts/check-config-namespace.sh
run observability  "可观测规约(八节,54 个组件)" bash scripts/check-observability.sh

echo
if [ "${#failed[@]}" -eq 0 ]; then
  echo "check-all: 全绿"
else
  echo "check-all: ${#failed[@]} 项未通过 —— ${failed[*]}"
  exit 1
fi
