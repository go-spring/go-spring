#!/usr/bin/env bash
# 编译 go-spring 工作区内全部模块（串行，work 模式共享依赖缓存）
# 用法: bash temp/build_all.sh
set -u

ROOT="/Users/didi/go-spring"
cd "$ROOT" || { echo "无法进入 $ROOT"; exit 1; }

USE_FILE="temp/_use_paths.txt"
FAIL_LOG="temp/_build_fail.txt"
OK_LOG="temp/_build_ok.txt"

# 1) 从 go.work 提取有效 use 路径（磁盘上真实存在的目录）
python3 -c "
import os
inuse=False
for line in open('go.work'):
    s=line.strip()
    if s=='use (': inuse=True; continue
    if inuse and s==')': inuse=False; continue
    if inuse and s.startswith('./') and os.path.isdir(s[2:]):
        print(s[2:])
" > "$USE_FILE"

: > "$FAIL_LOG"; : > "$OK_LOG"
total=$(wc -l < "$USE_FILE" | tr -d ' ')
ok=0; fail=0; i=0
start=$(date +%s)

echo "开始编译 $total 个模块 (work 模式: GOWORK=$(go env GOWORK))"
echo "------------------------------------------------"

while IFS= read -r d; do
  i=$((i+1))
  err=$(cd "$d" && go build ./... 2>&1)
  if [ -z "$err" ] || [ $? -eq 0 ]; then
    ok=$((ok+1))
    echo "$d" >> "$OK_LOG"
    printf "\r\033[K[%3d/%d] ✅ %s" "$i" "$total" "$d"
  else
    fail=$((fail+1))
    {
      echo "=== FAIL: $d ==="
      echo "$err" | head -8
      echo ""
    } >> "$FAIL_LOG"
    printf "\n[%3d/%d] ❌ FAIL %s\n" "$i" "$total" "$d"
  fi
done < "$USE_FILE"

end=$(date +%s)
echo ""
echo "================================================"
echo "完成: 成功 $ok / 失败 $fail / 总计 $total  (耗时 $((end-start))s)"
if [ "$fail" -gt 0 ]; then
  echo "失败明细见: $FAIL_LOG"
else
  echo "全部编译通过 🎉"
fi
