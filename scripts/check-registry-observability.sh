#!/usr/bin/env bash
#
# 维护者工具:检查每个 registry 后端是否遵循"在自己的缝上上报"(starter/DESIGN.md §3)。
#
# 规则:每个 starter-registry-<backend> 必须
#   1. 定义 obsSystem 常量 —— 后端名,作为指标的 system 属性
#   2. registry 侧上报三个操作:RegisterAttempt / DeregisterAttempt / WeightChange
#      且 RegisterAttempt 必须区分 initial 与 self_heal
#   3. discovery 侧调用 Synced —— 成功与失败两侧都要(失败不重置新鲜度)
#
# 为什么检查"上报点"而不是"接口装饰":自愈重注册不经过 discovery.Registrar
# 接口,只在后端自己的漏斗里(etcd publish / zk createNode / consul upsert),
# 所以"在接口外面包一层"看起来合规却漏掉最高价值的信号。此脚本检查的是
# 调用确实存在于模块内,漏斗位置由 DESIGN §3 的清单约束。
#
# 用法: bash scripts/check-registry-observability.sh
set -uo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.." || exit 1

# 无自愈路径的后端:存活由 SDK 自身的 ephemeral 心跳维持,故永远不产出
# self_heal 上报。新增例外必须在此登记,并同步 starter/DESIGN.md §3。
NO_SELF_HEAL="nacos"

# 无注册写侧的后端(k8s):集群内平台已经把每个 Pod 注册在 Service 之后,故它只做
# 读侧发现,没有 registrar —— 规则 2/2b 的三个注册操作与 Reason 对它无对象。
# 规则 1(obsSystem)与规则 3(Synced 成功/失败两侧)照旧适用:Synced 的 system 属性
# 就取自 obsSystem,读侧新鲜度是它唯一能产出的信号。新增例外必须在此登记,
# 并同步 starter/DESIGN.md §3。
NO_REGISTRAR="k8s"

fail=0
report() { printf '%s\n' "$1"; fail=1; }

backends=$(ls -d starter/starter-registry-* 2>/dev/null \
           | grep -v '^starter/starter-registry$' \
           | sed 's|^starter/starter-registry-||' | sort)
[ -n "$backends" ] || { echo "no registry backend found — check the scan"; exit 1; }

# has_yaml_reason reports whether the module's non-test sources use a Reason const.
has_reason() { grep -rq "$1" --include='*.go' --exclude='*_test.go' "$2"; }

for b in $backends; do
  dir="starter/starter-registry-$b"

  # 1) 后端名常量
  grep -q "obsSystem = " "$dir/starter.go" 2>/dev/null \
    || report "$b: starter.go 未定义 obsSystem(后端名/指标 system 属性)"

  # 2) & 2b) registry 写侧:只对真正有 registrar 的后端要求
  case " $NO_REGISTRAR " in
    *" $b "*) ;;
    *)
      has_reason 'discovery.RegisterAttempt(' "$dir" \
        || report "$b: 未上报 RegisterAttempt(注册,含自愈重注册)"
      has_reason 'discovery.DeregisterAttempt(' "$dir" \
        || report "$b: 未上报 DeregisterAttempt"
      has_reason 'discovery.WeightChange(' "$dir" \
        || report "$b: 未上报 WeightChange"

      # initial 必报;self_heal 视有无自愈路径
      has_reason 'discovery.ReasonInitial' "$dir" \
        || report "$b: RegisterAttempt 未用 ReasonInitial"
      case " $NO_SELF_HEAL " in
        *" $b "*)
          has_reason 'discovery.ReasonSelfHeal' "$dir" \
            && report "$b: 登记为无自愈路径,却使用了 ReasonSelfHeal"
          ;;
        *)
          has_reason 'discovery.ReasonSelfHeal' "$dir" \
            || report "$b: 有自愈路径却未用 ReasonSelfHeal(自愈重注册会退化成 initial)"
          ;;
      esac
      ;;
  esac

  # 3) discovery 侧成功/失败两侧都报(数出现次数而非命中行数:一行两处也要算两处)
  synced=$(grep -ro 'discovery\.Synced(' --include='*.go' --exclude='*_test.go' "$dir" 2>/dev/null | wc -l | tr -d ' ')
  [ "$synced" -ge 2 ] \
    || report "$b: discovery.Synced 调用仅 $synced 处(需要成功与失败两侧)"
done

if [ "$fail" -eq 0 ]; then
  echo "registry observability OK — backends: $(printf '%s ' $backends)"
  echo "no-self-heal (registered exception): $NO_SELF_HEAL"
  echo "no-registrar / discovery-only (registered exception): $NO_REGISTRAR"
else
  echo ""
  echo "规则见 starter/DESIGN.md §3;例外清单见本脚本 NO_SELF_HEAL 与 NO_REGISTRAR。"
  exit 1
fi
