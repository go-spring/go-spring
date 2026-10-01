#!/usr/bin/env bash
#
# 维护者工具:检查配置族命名法(starter/DESIGN.md §2.2)是否被遵守。
#
# 规则一:直接子键是"用户自选实例名"的多实例家族,家族前缀下只能是两个桶 ——
#   spring.<family>.instances.<name>.*   实例
#   spring.<family>.default.*            家族级值(可被实例覆盖)
# 因此多实例家族绝不能把配置直接绑定/门控在 ${spring.<family>} 上。
# 单实例家族(server / 全局设施)与闭命名空间家族不受此限,见 ALLOW。
#
# 规则二:框架/生态侧的绑定根必须是 spring.(DESIGN §2.2)。应用自己加的字段用
# 应用自己的前缀,不属于本检查范围 —— 只扫框架侧目录且整体跳过 example/。
# 只查"显式写死前缀"的调用点(OnProperty / BindEach / gs.Group / conf.Bind 第三参):
# value tag 里的点号宿主是相对键(${predicates.path} 绑在路由块下),无法静态分辨,
# 故不查。从环境变量导出的键(GS_POD_NAME -> pod.name)不出现在这些调用点,天然不受检。
#
# 用法: bash scripts/check-config-namespace.sh
set -uo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.." || exit 1

# 闭命名空间家族:子键名由框架决定(用户的实例名在更深一层),故不套桶。
# 新增例外必须在此显式登记,并同步 starter/DESIGN.md §2.2。
ALLOW="spring.registry"

# 框架侧允许的绑定根(规则二)。logging 是唯一登记的豁免:它是框架启动期(容器
# refresh 之前)就要生效的日志配置,由 gs_app 从顶层 logging 段读取,见 DESIGN §2.2。
# 新增豁免必须在此显式登记,并同步 DESIGN。
ALLOW_ROOTS="spring logging"

fail=0
report() { printf '%s\n' "$1"; fail=1; }

# 1) 找出所有"多实例家族" = 走 instances 桶绑定的家族
families=$(grep -rhoE '\$\{spring\.[a-zA-Z0-9._-]+\.instances\}' --include='*.go' starter/ \
           | sed 's/^\${spring\.//; s/\.instances}$//' | sort -u)
[ -n "$families" ] || { echo "no multi-instance family found — check the scan"; exit 1; }

is_multi() {
  local fam="$1"
  case "$fam" in
    $ALLOW|$ALLOW.*) return 1 ;;
  esac
  printf '%s\n' "$families" | grep -qx "$fam"
}

# 2) 绑定必须落在 instances 桶里(或已登记的例外/单实例家族)
while IFS= read -r hit; do
  file="${hit%%:*}"; line="${hit#*:}"
  fam=$(printf '%s' "$line" | grep -oE '\$\{spring\.[a-zA-Z0-9._-]+' | head -1 | sed 's/^\${spring\.//')
  case "$fam" in
    *.instances) ;;
    $ALLOW|$ALLOW.*) ;;
    *) is_multi "$fam" && report "绑定未走 instances 桶: $file (\${spring.$fam})" ;;
  esac
done < <(grep -rn 'BindEach(p, "\${spring\.\|gs\.Group("\${spring\.' --include='*.go' starter/ | grep -v '_test.go' || true)

# 3) 同一个家族的门控也必须落在 instances 桶里(裸家族前缀会与实例名同层)
while IFS= read -r hit; do
  file="${hit%%:*}"; line="${hit#*:}"
  fam=$(printf '%s' "$line" | grep -oE 'OnProperty\("spring\.[a-zA-Z0-9._-]+' | head -1 | sed 's/^OnProperty("spring\.//')
  case "$fam" in
    *.instances|$ALLOW|$ALLOW.*) ;;
    *) is_multi "$fam" && report "门控未走 instances 桶: $file ($fam)" ;;
  esac
done < <(grep -rn 'OnProperty("spring\.' --include='*.go' starter/ | grep -v '_test.go' || true)

# 4) 共享注册助手(前缀是变量,如 gorm 的 Dialect.Prefix)同样必须走 instances 桶
while IFS= read -r hit; do
  file="${hit%%:*}"; line="${hit#*:}"
  case "$line" in
    *".instances"*) ;;
    *) report "计算前缀未走 instances 桶: $file (${line##*:})" ;;
  esac
done < <(grep -rn 'OnProperty(d\.Prefix)\|BindEach(p, "\${" *+\|TagArg("\${" *+\|OnProperty(d\.Prefix + ' --include='*.go' starter/ | grep -v '_test.go' || true)

# 5) 框架/生态侧的绑定根必须是 spring.(规则二)
while IFS= read -r hit; do
  file="${hit%%:*}"; line="${hit#*:}"
  root=$(printf '%s' "$line" | grep -oE '"\$\{[a-zA-Z][a-zA-Z0-9._-]*|"[a-zA-Z][a-zA-Z0-9._-]*"' \
         | head -1 | sed 's/^"\$*{*//; s/^"//; s/\..*$//')
  [ -n "$root" ] || continue
  case " $ALLOW_ROOTS " in
    *" $root "*) ;;
    *) report "绑定根不是 spring. 前缀: $file (${root}.*)" ;;
  esac
done < <(grep -rnE 'gs\.OnProperty\("|BindEach\(p, "\$\{|gs\.Group\("\$\{|conf\.Bind\([^,]+, *&[^,]+, *"\$\{' \
             --include='*.go' cloud/ spring/ log/ gs/ starter/ 2>/dev/null \
         | grep -v '/example' | grep -v '_test.go' | perl -ne 'next if /:\s*\/\//; print')

if [ "$fail" -eq 0 ]; then
  echo "config namespace OK — multi-instance families: $(printf '%s ' $families)"
else
  echo ""
  echo "规则见 starter/DESIGN.md §2.2;例外清单见本脚本 ALLOW。"
  exit 1
fi
