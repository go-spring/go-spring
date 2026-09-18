#!/usr/bin/env bash
#
# 维护者工具:检查插桩是否遵循观测规约(starter/DESIGN.md §3「可观测」)。
#
# 规约是一条规则 —— «同类型 → 同名、同型、同齐整;名字取能力,不取实现»,
# 由三条原则支撑:
#
#   完整性 —— 每个组件都要有完善的可观测能力。缺信号、缺 status(分不出成败)都是缺陷。
#   共同性 —— 同族同类型的组件,共有的那部分必须一致(同样的名字、同样的仪器类型)。
#   灵活性 —— 组件特有的字段是允许的,不因"和别人不一样"而违规。
#
# 本脚本只查前两条。**它不检查"有没有多余的键"** —— 那是第三条原则明确许可的
# (config-bus 的 origin/prefix、scheduler 的 reason、gin 的 request_id 都合法)。
#
# 因此每个族用"共同内容清单"而不是"词汇表比对":族规列出一组必须出现的东西,
# 每个成员都得有、且同名同型;清单之外的东西一律不管。加一个族 = 加一段族规。
#
# 用法: bash scripts/check-observability.sh
set -uo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.." || exit 1

fail=0
report() { printf '%s\n' "$1"; fail=1; }

# 内联常量用的临时目录(每个组件一份,跑完删)
TMPDIR_PREP=$(mktemp -d)
trap 'rm -rf "$TMPDIR_PREP"' EXIT

# ── 工具 ────────────────────────────────────────────────────────────────

# inline_consts 把文件里的 Go 字符串常量内联进调用点后输出。
# 有些 starter 把属性键写成自己的常量(gin 的 attrHTTPRequestMethod = "http.request.method",
# 刻意不 import semconv 以避免版本耦合),不内联则 grep 字面量的取键函数一个键都看不见 ——
# 那是静默漏检,比误报危险。
inline_consts() {
  awk '
    BEGIN {
      while ((getline line < ARGV[1]) > 0) src = src "\n" line
      close(ARGV[1])
      rest = src; n = 0
      while (match(rest, /[A-Za-z_][A-Za-z0-9_]*[ \t]*=[ \t]*"[^"]*"/)) {
        pair = substr(rest, RSTART, RLENGTH)
        name = pair; sub(/[ \t]*=.*/, "", name)
        val  = pair; sub(/^[^"]*"/, "", val); sub(/"$/, "", val)
        names[++n] = name; vals[n] = val
        rest = substr(rest, RSTART + RLENGTH)
      }
      for (i = 1; i <= n; i++) gsub("\\(" names[i] ",", "(\"" vals[i] "\",", src)
      print src
    }' "$1"
}

# metric_labels 打印一个文件里所有在 metric.WithAttributes(...) 内声明的 attribute 键,每行一个。
# 用括号配平而非行匹配,因为属性列表经常跨行。
# 不区分具体仪器:同族的 duration 与 active_requests 共用身份 label,
# 差别只在 status,按并集判定足够;区分仪器名的做法试过,正则很脆且会静默失配。
metric_labels() {
  local src="$1"
  awk '
    function keys_of(seg,   k) {
      while (match(seg, /attribute\.[A-Za-z0-9]+\("[^"]*"/)) {
        k = substr(seg, RSTART, RLENGTH)
        sub(/^.*\("/, "", k); sub(/"$/, "", k)
        print k
        seg = substr(seg, RSTART + RLENGTH)
      }
    }
    BEGIN {
      while ((getline line < ARGV[1]) > 0) src = src "\n" line
      close(ARGV[1])
      rest = src
      while (match(rest, /metric\.WithAttributes\(/)) {
        body = substr(rest, RSTART + RLENGTH)
        depth = 1; i = 1
        while (i <= length(body) && depth > 0) {
          c = substr(body, i, 1)
          if (c == "(") depth++
          else if (c == ")") depth--
          if (depth == 0) break
          i++
        }
        keys_of(substr(body, 1, i - 1))
        rest = substr(body, i + 1)
      }
    }' "$1"
}

# attr_keys 打印文件里所有 attribute 键(span 属性与 metric label 一起)。
attr_keys() {
  grep -oE 'attribute\.[A-Za-z0-9]+\("[^"]*"' "$1" 2>/dev/null \
    | sed 's/^.*("/"/; s/"$//; s/^"//' | sort -u
}

# log_keys 打印文件里所有结构化日志字段的键。取"任何 log.Xxx(\"...\")"再排掉已知的非字段构造器,
# 而不是白名单列字段构造器 —— 后者漏一个(最初就漏了 `log.Float`)会让整族静默误报。
log_keys() {
  grep -oE 'log\.[A-Za-z0-9]+\("[^"]*"' "$1" 2>/dev/null \
    | grep -vE 'log\.(Register|BuildTag|Msg)' \
    | sed 's/^.*("/"/; s/"$//; s/^"//' | sort -u
}

# has_all 断言"文件 $1 的键清单 $2 里含全部 $3";$4 是描述,$5 是组件名。
#
# 要求的每一项可以是 `键`,也可以是 `键|semconv常量名` —— 键由 semconv 常量构造时
# grep 字面量看不见(gin 就是这么写的),后者让其通过。常量写法优于字面量,所以是
# checker 去认它,而不是要求代码把常量摊成字面量。
has_all() {
  local f="$1" have="$2"
  local want key cname
  for want in $3; do
    key="${want%%|*}"; cname="${want##*|}"
    if printf '%s\n' "$have" | grep -qx "$key"; then continue; fi
    if [ "$cname" != "$key" ] && grep -q "semconv\.$cname" "$f" 2>/dev/null; then continue; fi
    report "$5: $4 缺 $key"
  done
}

# ── 检查一个族 ──────────────────────────────────────────────────────────
# 参数:族名 成员探测正则 metric 清单(名:仪器类型) label 集 span 属性集
#       日志键集 span-后端登记表 额外成员(目录名:证据正则,可空)
#
# span-后端登记表列出「span 由后端自带的 OTel 插桩发出」的成员 —— starter 再发一个
# 就是重复。登记需附接入点证据,脚本会 grep 它;grep 不到 = 登记失效或接线被删,
# 那时就是真的没有 span。只豁免 span 与 span 属性两项:指标仍必须由 starter 自建。
# 新增一个都必须先查证「库给 span 设的属性是否与本族词汇一致」。
check_family() {
  local fam="$1" detect="$2" metrics="$3" labels="$4" span_attrs="$5" log_want="$6" span_backend="$7" extra="$8"

  # 跨行容错:指标名可能写在 Float64Histogram( 的下一行,故用"含名字字面量"定位候选,
  # 再用"含构造调用"确认它确实自建了该仪器(只提及不算)。
  local members cand
  members=""
  for cand in $(grep -rl "\"$detect\"" --include='*.go' starter/ 2>/dev/null | grep -v _test); do
    grep -q 'Float64Histogram(' "$cand" && members="$members $cand"
  done
  members=$(printf '%s\n' $members | sort)
  for e in $extra; do
    members="$members $(find starter -maxdepth 2 -type d -name "${e%%:*}" 2>/dev/null | head -1)/observe.go"
  done
  members=$(printf '%s\n' $members | grep -v '^$' | sort -u)
  [ -n "$members" ] || { echo "no $fam instrumentation found — check the scan"; return; }

  local f comp ev m name kind
  CLASSIFIED="$CLASSIFIED $(printf '%s\n' $members | sed 's|^starter/experimental/||; s|^starter/||; s|/.*$||' | tr '\n' ' ')"
  PREP=""
  for f in $members; do
    comp=$(printf '%s' "$f" | sed 's|starter/experimental/||; s|starter/||; s|/observe.go$||; s|/observe/plugin.go$||; s|/.*$||')

    # 检查以「组件」为单位,不以 grep 命中的那个文件为单位:echo 的指标在 metrics.go、
    # span 在 tracing.go、日志在 middleware.go,只查一个文件必然误报。常量也可能定义在
    # 与使用点不同的文件里,故先拼接组件全部非测试源码,再内联常量。
    DIR=$(dirname "$f")
    SRC="$TMPDIR_PREP/src_$(printf '%s' "$comp" | tr '/' '_').go"
    find "$DIR" -maxdepth 2 -name '*.go' ! -name '*_test.go' -exec cat {} + > "$SRC" 2>/dev/null
    PREP="$TMPDIR_PREP/prep_$(printf '%s' "$comp" | tr '/' '_').go"
    inline_consts "$SRC" > "$PREP" 2>/dev/null || PREP="$SRC"
    # 空白归一化的副本:指标名常写在构造调用的下一行,不归一化就匹配不到。
    NORM="$TMPDIR_PREP/norm_$(printf '%s' "$comp" | tr '/' '_').go"
    tr -s ' \t\n' ' ' < "$PREP" > "$NORM"

    # 完整性 + 同型:每个 metric 都得在,且仪器类型对得上
    for m in $metrics; do
      name="${m%%:*}"; kind="${m##*:}"
      grep -q "$kind( *\"$name\"" "$NORM" || report "[$fam] $comp: 缺 $name(或类型不是 $kind)"
    done
    grep -q 'RegisterAppTag(' "$SRC" || report "[$fam] $comp: 无访问日志 tag"

    # 共同性:共有的名字必须一致
    # 属性键按组件级判定。**它保证的是"该键在本组件里存在",不保证"只上 metric 不上
    # span"** —— 本仓多处先把键攒进 []attribute.KeyValue 变量再 WithAttributes(attrs...),
    # 括号内取键在这类写法上必然取空(试过,整片误报)。要精确到信号需 Go AST,shell 做不到。
    # 这仍能抓住真实漂移:键名不一致、键整个缺失。
    has_all "$PREP" "$(attr_keys "$PREP")" "$labels" "属性键" "[$fam] $comp"
    has_all "$PREP" "$(log_keys "$PREP")" "$log_want" "访问日志字段" "[$fam] $comp"

    # span:自建,或已登记为「后端插桩提供」(登记需能 grep 到接入点)
    ev=''
    for e in $span_backend; do
      [ "${e%%:*}" = "$comp" ] && ev="${e#*:}"
    done
    if [ -n "$ev" ]; then
      grep -rqE "$ev" "$DIR" 2>/dev/null \
        || report "[$fam] $comp: 登记为「span 由后端插桩提供」,但 grep 不到接入点 /$ev/"
    else
      grep -q 'otel\.Tracer(\|tracer\.' "$SRC" || report "[$fam] $comp: 无 span(未起 tracer)"
    fi
  done
  echo "[$fam] 成员 $(printf '%s\n' $members | wc -l | tr -d ' ') 个"
}

# ── 族规(改这里等于改规约)────────────────────────────────────────────

# DB 族
#   span 由后端插桩提供:
#     - starter-elasticsearch: elastictransport 的 ElasticsearchOpenTelemetry;其
#       instrumentation.go 设的正是 db.system / db.operation / db.statement,同源。
#     - starter-go-redis:      redisotel 的 InstrumentTracing。
CLASSIFIED=''

check_family DB \
  'db.client.operation.duration' \
  'db.client.operation.duration:Float64Histogram db.client.active_requests:Int64UpDownCounter' \
  'db.system db.operation status' \
  'db.system db.operation db.statement' \
  'db.operation db.statement status duration_ms error' \
  'starter-elasticsearch:newOtelInstrumentation\(\) starter-go-redis:redisotel.InstrumentTracing' \
  ''

# 消息族
#   span 由后端插桩提供:
#     - starter-kafka: franz-go 的 kotel(driver.go 装 tracer+meter);其 tracer.go 设的正是
#       semconv 的 messaging.system / messaging.operation / messaging.destination.name,同源。
#       注意 kotel 的 metric 是 messaging.kafka.*(客户端/ broker 健康,带实现名),不是族内的
#       操作指标 —— 那两项仍由 starter 自建。
#   注:messaging.client.connection.state_changes 目前只有 mqtt/nats/rabbitmq 有,
#   "长连接协议专属还是全族都要"尚未裁决 —— 未裁决就不该拿它卡人,故不进清单。
check_family 消息 \
  'messaging.client.operation.duration' \
  'messaging.client.operation.duration:Float64Histogram messaging.client.active_requests:Int64UpDownCounter' \
  'messaging.system messaging.operation status' \
  'messaging.system messaging.operation messaging.destination.name' \
  'messaging.operation messaging.destination.name status duration_ms error' \
  'starter-kafka:kotel.NewKotel' \
  'starter-kafka'

# HTTP 族(server 类)
#   gateway 不算本族:它是代理,不是 server,且自有 gateway_* 命名的一套。
#   本族统一到 OTel HTTP semconv 命名 —— metric 名本就是 semconv,日志键与 label 同名
#   才能 join,所以日志也照它写(echo/hertz 原用裸名 method/path/status/latency)。
check_family HTTP \
  'http.server.request.duration' \
  'http.server.request.duration:Float64Histogram http.server.active_requests:Int64UpDownCounter' \
  'http.request.method http.response.status_code' \
  'http.request.method http.response.status_code' \
  'http.request.method url.path http.response.status_code duration_ms' \
  '' \
  ''

# ── cloud 域包:身份类日志键必须能 join ────────────────────────────────
# cloud 的域包(discovery/lock/resilience/confrefresh …)不是可互换后端,没有"族"可言,
# 但它们都适用同一条不变量 —— discovery/observe.go 里写明、全生态都该满足的那条:
# **失败指标要能落到解释它的那行日志**,所以日志里的身份键必须与某个属性(spans 或 metric
# label)同名。lock 的日志写 key、span 写 lock.key,这条就断了 —— 那正是本段要抓的。
#
# 取值:时长(键以 _ms 结尾)与 error 是日志行自身的载荷,不是身份,豁免。
check_cloud_join() {
  local f d comp k miss src have
  for f in $(grep -rl 'otel\.Meter(\|otel\.Tracer(' --include='*.go' cloud/ 2>/dev/null | grep -v _test | sort); do
    d=$(dirname "$f"); comp=${d#cloud/}
    # 用独立变量名,不碰族检查共用的 $PREP —— 共享可变全局在这里是不确定性的来源。
    src="$TMPDIR_PREP/cloud_$(printf '%s' "$comp" | tr '/' '_').go"
    find "$d" -maxdepth 2 -name '*.go' ! -name '*_test.go' | sort | xargs cat > "$src" 2>/dev/null
    have=$(attr_keys "$src")
    miss=""
    for k in $(log_keys "$src"); do
      case "$k" in *_ms|error) continue;; esac
      printf '%s\n' "$have" | grep -qx "$k" || miss="$miss $k"
    done
    [ -z "$miss" ] || report "[cloud] $comp: 身份类日志键无同名属性(join 不上):$miss"
  done
}

check_cloud_join

# ── 完整性守卫:注册了访问 tag 却未归入任何族的 starter ─────────────────
# 族成员靠"创建了该族仪器"识别,会漏掉"只打日志"或"指标由库提供"的成员 —— go-redis 与
# starter-kafka 都这样整族漏掉过。漏检比误报危险得多,故反向兜底。
#
# 只认「访问日志 tag」RegisterAppTag(x, "access")。生命周期 tag RegisterAppTag(x, "") 是
# starter 自己的日志分类,不是访问日志 —— config / registry 族的每操作可观测性委托给
# cloud/confrefresh 与 cloud/discovery,本来就没有 per-starter 访问日志,不该被算作漏检。
#
# 现阶段只提示不失败:HTTP 等族尚有余项。**R4 全族齐了之后这里应改成 report**。
unclassified=''
for name in $(grep -rlE 'RegisterAppTag\([^)]*"access"' --include='*.go' starter/ 2>/dev/null | grep -v _test \
           | sed 's|^starter/experimental/||; s|^starter/||; s|/.*$||' | sort -u); do
  printf '%s\n' " $CLASSIFIED " | grep -q " $name " || unclassified="$unclassified $name"
done
if [ -n "$unclassified" ]; then
  echo ""
  echo "未归类(属尚未定义的族,R4 前只提示):$(printf '%s ' $unclassified)"
fi

if [ "$fail" -eq 0 ]; then
  echo ""
  echo "observability OK"
else
  echo ""
  echo "共同内容清单见本脚本各族规;规则见 starter/DESIGN.md §3。"
  echo "注:脚本只查\"共同内容齐不齐\",不查\"有没有多余的键\" —— 组件特有字段是允许的。"
  exit 1
fi
