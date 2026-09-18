#!/usr/bin/env bash
#
# 维护者工具:检查插桩是否遵循观测规约。**规约的正文在 starter/DESIGN{,_CN}.md §3**,
# 这里只写执行面,不重复规则本身 —— 一份规则只有一个家。
#
# 规则一句话:同类型 → 同名、同型、同齐整,名字取能力不取实现。由三条原则支撑 ——
# 完整性(缺信号/缺成败维度是缺陷)、共同性(同族在**含义相同**的部分上一致)、
# 灵活性(组件特有字段允许)。两档模型:同名只在同义时才要求,`rpc.grpc.status_code`
# 这类各家取值词表不同的键**不要求**同名。
#
# 本脚本只查前两条。**它不检查"有没有多余的键"** —— 那是第三条原则明确许可的
# (config-bus 的 origin/prefix、scheduler 的 reason、gin 的 request_id 都合法)。
#
# 因此族用"共同内容清单"而不是"词汇表比对":族规列出一组必须出现的东西,
# 每个成员都得有、且同名同型;清单之外的东西一律不管。加一个族 = 加一段族规。
#
# 分节(每节一种机制,别把它们的判据互相套用):
#   §族规    可替换后端族:同族共有的部分同名同型。成员靠"建了该族仪器"识别
#   §registry 后端在自己的缝上上报(委托调用点存在性,不是名字)
#   §config   配置源完全委托 cloud/confrefresh:查接线,且不得自建仪器
#   §底线     自建插桩但无同类的单例:只有完整性与可 join 可查
#   §委托     信号全部来自共享层:查接线还在不在
#   §cloud    cloud 域包的身份类日志键必须能 join
#   §守卫/正向 有插桩却没归类 = 漏检;必须插桩却没插桩 = 缺陷
#
# 每节末尾都有一条"未扫到任何成员即失败"的兜底:空结果不是"没有问题",是扫描坏了 ——
# 静默假绿是本脚本最该避免的失败模式。
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

# code_only 打印去掉注释后的源码。**登记表的证据必须落在真实调用点上**:
# 注释里、文档里提到同一个名字不算接入点 —— 否则把接线删掉、注释留着,登记照样"通过",
# 登记表就成了免检牌。这不是假想:实测 kitex 的 tracing.NewServerSuite() 在 2 个
# USAGE 文档与 1 处注释里都出现,只按原始源码 grep 时,删掉真正的调用点也不报错。
#
# 只去整行注释与块注释;行尾注释保留(在那里切会把字符串里的 // 一起切掉,例如
# "https://…",反而可能截掉同一行后面的真代码)。各族的证据都锚在调用形态上(带 \(),
# 行尾注释碰巧写出同样形态的概率可以忽略。
code_only() {
  perl -0777 -pe 's{/\*.*?\*/}{}gs; s{^\s*//.*$}{}gm' "$1"
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
#       日志键集 span-后端登记表 额外成员(目录名,可空) 访问 tag 正则(可空,默认 RegisterAppTag)
#
# span-后端登记表列出「span 由后端自带的 OTel 插桩发出」的成员 —— starter 再发一个
# 就是重复。登记需附接入点证据,脚本会 grep 它;grep 不到 = 登记失效或接线被删,
# 那时就是真的没有 span。只豁免 span 与 span 属性两项:指标仍必须由 starter 自建。
# 新增一个都必须先查证「库给 span 设的属性是否与本族词汇一致」。
#
# metric-后端登记表(全局 $METRIC_FROM_BACKEND)同理,但豁免的是**指标自建**这一整块 ——
# 用于 metric 由第三方库发出的成员(kitex 的观测套件、kratos 的 middleware、otelhttp)。
# 这类成员连 span 也一并由库提供,此时本族的名词清单在本仓源码里无从查起,故
# 「metric + span 双登记」的成员跳过属性清单,只查访问 tag 与两处接入点证据。
#
# 访问 tag 正则(第 9 参,可空):默认要求 RegisterAppTag(x,"access") —— action 必须是
# "access",生命周期 tag(x,"") 不算。族规可覆盖它(例如某族用别的注册形态)。
check_family() {
  local fam="$1" detect="$2" metrics="$3" labels="$4" span_attrs="$5" log_want="$6" span_backend="$7" extra="$8"
  # 默认要求 action 是 "access"。只匹配函数名是不够的:生命周期 tag
  # RegisterAppTag(x, "") 也会通过 —— 而那是分类,不是访问日志。守卫查的正是 "access",
  # 两者必须同一个口径。
  local tag_re="${9:-RegisterAppTag\\([^)]*\"access\"}"

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
  # 空成员不是"这个族还没有成员",是**扫描坏了** —— detect 字面量一旦与代码漂开,
  # 整族会静默熄灭且退出码 0,那正是本脚本最该避免的漏检。故置 fail 而不是 echo。
  [ -n "$members" ] || { report "[$fam] 未扫到任何成员 —— detect 字面量 /$detect/ 可能已与代码漂开(整族静默熄灭)"; return; }

  local f comp ev m name kind mb_names mb_ev rest
  CLASSIFIED="$CLASSIFIED $(printf '%s\n' $members | sed 's|^starter/experimental/||; s|^starter/||; s|/.*$||' | tr '\n' ' ')"
  PREP=""
  for f in $members; do
    comp=$(printf '%s' "$f" | sed 's|starter/experimental/||; s|starter/||; s|/observe.go$||; s|/observe/plugin.go$||; s|/.*$||')

    # 检查以「组件」为单位,不以 grep 命中的那个文件为单位:echo 的指标在 metrics.go、
    # span 在 tracing.go、日志在 middleware.go,只查一个文件必然误报。常量也可能定义在
    # 与使用点不同的文件里,故先拼接组件全部非测试源码,再内联常量。
    DIR=$(dirname "$f")
    SRC="$TMPDIR_PREP/src_$(printf '%s' "$comp" | tr '/' '_').go"
    # maxdepth 3,不是 2:日志桥这类内部包在 internal/<pkg>/ 下,是第 3 层
    # (kitex/kratos 的 RegisterRPCTag 就写在 internal/logger/logger.go,漏掉 = 静默误报)。
    # 排除 example*:示例程序不是组件的插桩,把它们拼进来只会**多出**属性键,
    # 从而掩盖真实的缺失 —— 只能造成假绿,不能造成假红。
    find "$DIR" -maxdepth 3 -name '*.go' ! -name '*_test.go' \
      ! -path '*/example/*' ! -path '*/example-*/*' -exec cat {} + > "$SRC" 2>/dev/null
    PREP="$TMPDIR_PREP/prep_$(printf '%s' "$comp" | tr '/' '_').go"
    inline_consts "$SRC" > "$PREP" 2>/dev/null || PREP="$SRC"
    # 空白归一化的副本:指标名常写在构造调用的下一行,不归一化就匹配不到。
    NORM="$TMPDIR_PREP/norm_$(printf '%s' "$comp" | tr '/' '_').go"
    tr -s ' \t\n' ' ' < "$PREP" > "$NORM"
    # 去注释副本:登记表的证据与访问 tag 只认它。
    CODE="$TMPDIR_PREP/code_$(printf '%s' "$comp" | tr '/' '_').go"
    code_only "$SRC" > "$CODE" 2>/dev/null || cp "$SRC" "$CODE"

    # metric:自建,或已登记为「metric 由后端库发出」(登记需能 grep 到接入点)
    mb_names=''; mb_ev=''
    for e in $METRIC_FROM_BACKEND; do
      [ "${e%%:*}" = "$comp" ] || continue
      rest="${e#*:}"; mb_names="${rest%%:*}"; mb_ev="${rest#*:}"
    done
    if [ -n "$mb_names" ]; then
      grep -qE "$mb_ev" "$CODE" 2>/dev/null \
        || report "[$fam] $comp: 登记为「metric 由后端提供」,但代码里找不到接入点 /$mb_ev/"
    else
      # 完整性 + 同型:每个 metric 都得在,且仪器类型对得上
      for m in $metrics; do
        name="${m%%:*}"; kind="${m##*:}"
        grep -q "$kind( *\"$name\"" "$NORM" || report "[$fam] $comp: 缺 $name(或类型不是 $kind)"
      done
    fi
    # 用 -E:谓词写成 ERE 形式(如 RegisterAppTag\() —— 在 BRE 里 \( 是分组符,会报 "parentheses not balanced" 并让匹配全数失败。
    grep -qE "$tag_re" "$CODE" || report "[$fam] $comp: 无访问日志 tag /$tag_re/"

    # span:自建,或已登记为「后端插桩提供」(登记需能 grep 到接入点)
    ev=''
    for e in $span_backend; do
      [ "${e%%:*}" = "$comp" ] && ev="${e#*:}"
    done

    # 共同性:共有的名字必须一致
    #
    # label 清单**能精确就精确**:键内联写在 metric.WithAttributes(...) 里的可以精确取出。
    #
    # 但**不能拿"字面量出现在 metric 里"去反推"它只上 metric"** —— 本仓大量组件用同一批键
    # 同时喂两个信号(cassandra: inflight 的字面量给 metric,另一份同样的字面量攒成切片给
    # span),减法会把这些键误判成"span 上缺"。同理,键先攒进 []attribute.KeyValue 再
    # WithAttributes(attrs...) 的组件,括号内取键必然取空(gin 的 durAttrs 就是)。
    #
    # 所以精确路径的准入条件是:**该组件没有任何"变量展开式"的 WithAttributes 调用**
    # (形如 WithAttributes(attrs...))。只要有一处,提取就可能不完整,整组件回退到
    # 组件级并集判定。回退是**静默降级**(仍保证键名一致与键存在,只是不辨信号),不产生误报。
    #
    # span_attrs 只能对**并集**判定,且对"span 由后端插桩提供"的成员豁免 —— 那些成员的
    # span 属性由库设置,仓库源码里本就没有。shell 无法把 span 属性与 metric label 真正切开,
    # 这两条是该能力下的上界,不要试图收得更紧。
    #
    # metric + span **双登记**的成员(kitex/kratos/http-client)整块跳过:两个信号都由库发,
    # 本族的名词清单在本仓源码里无从查起 —— 查了只会误报。
    #
    # **豁免范围到此为止,且必须照实写**:这类成员只剩下「访问 tag」与「两处接入点证据」
    # 两项能查。它们因此天然缺本族的 metric 维度(如 RPC 族对 kitex/kratos 就缺 status ——
    # 库的时长指标本就没有这一维),那是条款里**已登记的缺口**,不是检查器没写。
    # 新增一条双登记 = 多接受一个这样大小的缺口,登记时必须把缺口内容写进注释。
    if [ -z "$mb_names" ]; then
      if ! grep -qE 'WithAttributes\( *[A-Za-z_][A-Za-z0-9_]* *(\.\.\.|[,)])' "$PREP"; then
        has_all "$PREP" "$(metric_labels "$PREP" | sort -u)" "$labels" "metric label" "[$fam] $comp"
      else
        has_all "$PREP" "$(attr_keys "$PREP")" "$labels" "属性键(信号不可辨)" "[$fam] $comp"
      fi
    fi
    if [ -z "$ev" ]; then
      has_all "$PREP" "$(attr_keys "$PREP")" "$span_attrs" "span 属性" "[$fam] $comp"
    fi
    # 访问日志**始终查**,库委托型成员也不例外:那是它们唯一自产的东西,
    # 若连这条也免掉,登记就只剩两处证据 grep 可查了。
    has_all "$PREP" "$(log_keys "$PREP")" "$log_want" "访问日志字段" "[$fam] $comp"

    if [ -n "$ev" ]; then
      grep -qE "$ev" "$CODE" 2>/dev/null \
        || report "[$fam] $comp: 登记为「span 由后端插桩提供」,但代码里找不到接入点 /$ev/"
    else
      grep -q 'otel\.Tracer(\|tracer\.' "$SRC" || report "[$fam] $comp: 无 span(未起 tracer)"
    fi
  done
  echo "[$fam] 成员 $(printf '%s\n' $members | wc -l | tr -d ' ') 个"
}

# ── 登记表 ──────────────────────────────────────────────────────────────
#
# 「metric 由后端库发出」—— 形如 组件:库实际发出的 metric 名(逗号分隔):接入点证据正则。
#
#   * 中间那个名字字段是**给人复核的**,不是机器强制的:库里的名字本仓 grep 不到,
#     脚本无权声称验过它。**不要把这条包装成"已校验"** —— 它的作用是让复核者知道
#     该去库里对什么名字,库版本因此必须钉在这里。
#   * 证据字段才是机器强制的:grep 不到即报错(防登记变免检牌)。
#   * 登记 = 本族放弃这个信号上的 metric 名/类型/单位要求。若 span 也一并由库提供,
#     该成员整块跳过属性清单。**豁免范围到此为止**,访问 tag 与证据照查。
#
# 库版本钉在这里(升级即须复核):
#   kitex-contrib/obs-opentelemetry v0.3.0 → tracing/metrics.go: ServerDuration = "rpc.server.duration"
#   go-kratos/kratos/v2 v2.9.2            → middleware/metrics: server_requests_code_total / server_requests_seconds
#   otelhttp v0.62.0                      → http.client.request.duration(单位 s)
METRIC_FROM_BACKEND="
  starter-kitex:rpc.server.duration:tracing.NewServerSuite\(\)
  starter-kratos:server_requests_code_total,server_requests_seconds:kmetrics\.Server\(
  starter-http-client:http.client.request.duration:otelhttp.NewTransport
"

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

# RPC 族
#
#   两档模型的样板:族内强制同名的只有**语义与取值都完全相同**的键 ——
#   rpc.system / rpc.method / status,加上三个取能力不取实现的 metric 名。
#   各家 transport 专属的状态码(grpc 的 rpc.grpc.status_code)取值词表彼此不同,
#   强行同名会把两个含义挤进一个键,故**不要求、不检查**。
#
#   访问日志五家都有,形态与其他族一致(RegisterAppTag(x,"access") + 每调用一行)。
#   注意 RegisterRPCTag 是**另一件事**:kitex/kratos/trpc 用它把框架自己的日志桥进
#   go-spring log,那是 edge-bridge,不是 per-call 访问日志,两者并存不冲突。
#
#   metric/span 由后端插桩提供(双登记)—— 缺口内容登记如下:
#     - starter-kitex: kitex-contrib/obs-opentelemetry 的 tracing 套件。其 tracer 设的
#       正是 semconv 的 rpc.system / rpc.method / rpc.service,与族内同源;其 metric 是
#       rpc.server.duration,**单位是毫秒**,与族内自建的 unit=s 不同(库事实,已登记)。
#       其 label 只有 rpc.* 与 peer.*,**没有 status 维度** —— 本族的时长指标对 kitex
#       告不出错,这是已登记的缺口。
#     - starter-kratos: kratos/v2 的 tracing 与 metrics middleware。metric 名是
#       server_requests_code_total / server_requests_seconds,label 是 kind/operation/code/reason,
#       其中 seconds 只有 kind/operation —— duration 同样分不出成败(已登记缺口)。
#       另:其 ws 子包零插桩,尚未裁决(见 USAGE)。
check_family RPC \
  'rpc.server.request.duration' \
  'rpc.server.request.duration:Float64Histogram rpc.server.active_requests:Int64UpDownCounter rpc.server.request_count:Int64Counter' \
  'rpc.system rpc.method status' \
  'rpc.system rpc.method' \
  'rpc.method status duration_ms' \
  'starter-kitex:tracing.NewServerSuite\(\) starter-kratos:tracing.Server\(\)' \
  'starter-kitex starter-kratos'

# ── registry 族:后端在自己的缝上上报 ───────────────────────────────────
#
# 规则来自 starter/DESIGN{,_CN}.md §3「Registry backends report at their own seams」,
# 原先由 scripts/check-registry-observability.sh 单独跑,现已并入本脚本 —— 一个入口,
# 免得"例外表在哪"有两份答案。
#
# 为什么查"上报点"而不是"接口装饰":自愈重注册不经过 discovery.Registrar 接口,
# 只在后端自己的漏斗里(etcd publish / zk createNode / consul upsert),所以
# "在接口外面包一层"看起来合规却漏掉最高价值的信号。名字全部落在 cloud/discovery/observe.go,
# 后端从不自己命名 instrument —— 本段查的是"调用确实存在于模块内"。
#
# 无自愈路径的后端:存活由 SDK 自身的 ephemeral 心跳维持,故永远不产出 self_heal 上报。
# 新增例外必须在此登记,并同步 starter/DESIGN{,_CN}.md §3。
NO_SELF_HEAL="nacos"

# 无注册写侧的后端(k8s):集群内平台已经把每个 Pod 注册在 Service 之后,故它只做读侧发现,
# 没有 registrar。规则 2/2b 的三个注册操作与 Reason 对它无对象;规则 1 与规则 3 照旧适用。
# 新增例外必须在此登记,并同步 starter/DESIGN{,_CN}.md §3。
NO_REGISTRAR="k8s"

check_registry() {
  local b dir backends synced
  backends=$(ls -d starter/starter-registry-* 2>/dev/null \
             | grep -v '^starter/starter-registry$' \
             | sed 's|^starter/starter-registry-||' | sort)
  [ -n "$backends" ] || { report "[registry] 未扫到任何后端 —— 本段静默失效"; return; }

  for b in $backends; do
    dir="starter/starter-registry-$b"

    # 1) 后端名常量(指标的 system 属性取值)
    grep -q "obsSystem = " "$dir/starter.go" 2>/dev/null \
      || report "[registry] $b: starter.go 未定义 obsSystem(后端名/指标 system 属性)"
    CLASSIFIED="$CLASSIFIED starter-registry-$b"

    # 2) & 2b) 写侧:只对真正有 registrar 的后端要求
    case " $NO_REGISTRAR " in
      *" $b "*) ;;
      *)
        has_call() { grep -rq "$1" --include='*.go' --exclude='*_test.go' "$dir"; }
        has_call 'discovery.RegisterAttempt(' \
          || report "[registry] $b: 未上报 RegisterAttempt(注册,含自愈重注册)"
        has_call 'discovery.DeregisterAttempt(' \
          || report "[registry] $b: 未上报 DeregisterAttempt"
        has_call 'discovery.WeightChange(' \
          || report "[registry] $b: 未上报 WeightChange"
        has_call 'discovery.ReasonInitial' \
          || report "[registry] $b: RegisterAttempt 未用 ReasonInitial"
        case " $NO_SELF_HEAL " in
          *" $b "*)
            has_call 'discovery.ReasonSelfHeal' \
              && report "[registry] $b: 登记为无自愈路径,却使用了 ReasonSelfHeal"
            ;;
          *)
            has_call 'discovery.ReasonSelfHeal' \
              || report "[registry] $b: 有自愈路径却未用 ReasonSelfHeal(自愈重注册会退化成 initial)"
            ;;
        esac
        ;;
    esac

    # 3) 读侧成功/失败两侧都报。数出现次数而非命中行数:一行两处也要算两处。
    synced=$(grep -ro 'discovery\.Synced(' --include='*.go' --exclude='*_test.go' "$dir" 2>/dev/null | wc -l | tr -d ' ')
    [ "$synced" -ge 2 ] \
      || report "[registry] $b: discovery.Synced 调用仅 $synced 处(需要成功与失败两侧)"
  done
}

# ── config 族:完全委托 cloud/confrefresh ────────────────────────────────
#
# 七个配置源 starter 自身零插桩:每条配置的刷新由 cloud/confrefresh 观测
# (config.refresh.total / duration / last_success),starter 只负责把它接上。
# 成员是**显式清单**而非 ls starter/starter-config-*:后者会把 starter-config-bus
# 卷进来 —— 它是总线不是配置源,属于底线段。
#
# 判据全部用目录级 grep:starter-config-file 的调用点在 watch.go,任何"只看 starter.go"
# 的写法都会漏掉它。
CONFIG_DELEGATES="apollo consul etcd k8s nacos vault file"

check_config() {
  local b dir n
  for b in $CONFIG_DELEGATES; do
    dir="starter/starter-config-$b"
    [ -d "$dir" ] || { report "[config] $b: 目录不存在(config 族清单漂移)"; continue; }

    n=$(grep -ro 'confrefresh\.Run(' --include='*.go' --exclude='*_test.go' "$dir" 2>/dev/null | wc -l | tr -d ' ')
    [ "$n" -eq 1 ] \
      || report "[config] $b: confrefresh.Run 调用 $n 处(需要恰好 1 处,多一处即重复刷新)"

    grep -rqE 'RegisterAppTag\("config_' --include='*.go' --exclude='*_test.go' "$dir" 2>/dev/null \
      || report "[config] $b: 无 config_<backend> 生命周期 tag"

    # 回归绊线:配置源的插桩全在 cloud/confrefresh,本地出现仪器就是重复上报。
    grep -rqE 'Float64Histogram\(|Int64Counter\(|Int64UpDownCounter\(|metric\.WithAttributes\(|otel\.Tracer\(' \
         --include='*.go' --exclude='*_test.go' "$dir" 2>/dev/null \
      && report "[config] $b: 自建插桩 —— config 族的可观测性完全委托 cloud/confrefresh,不该有本地仪器"

    CLASSIFIED="$CLASSIFIED starter-config-$b"
  done
}

# ── 底线段:自建插桩、但不成族的单例组件 ─────────────────────────────────
#
# scheduler / config-bus / mail 各自是独一无二的能力(定时、配置总线、发信),
# 彼此不可互换,所以**共同性没有对象** —— 它们之间不需要同名,也无从"换个后端看板还得改"。
# 于是就只剩两条硬要求,也就是本段查的东西:
#
#   1. 成败可辨:必须有名为 status 的属性(metric label)。
#      "分不出成败"是缺陷,与它在哪个族无关。
#   2. 可 join:身份类日志键必须与某个属性同名,否则失败指标落不到解释它的日志行上。
#      时长(键以 _ms 结尾)与 error 是日志行自身的载荷,不是身份,豁免;
#      组件特有的叙述性字段(如 scheduler 的 reason)在 BASELINE_PAYLOAD 里登记豁免。
#
# 另加一条自建断言:这些组件不委托任何后端,信号应当自己发 —— 没有 otel.Meter( 说明
# 插桩被拆掉了(或这个组件根本还没插桩,那它该进正向清单而不是这里)。
BASELINE_MEMBERS="starter-scheduler starter-config-bus starter-mail starter-gateway"

# 载荷豁免:日志行自身的叙述,不是被观测实体的身份。新增一个都要在这里登记并说明理由。
BASELINE_PAYLOAD="
  starter-scheduler:reason
  starter-config-bus:origin,watched,subject
# prefix 不再豁免:它已是 metric label(见 observe.go 的 record)
"

# join_missing 打印身份类日志键里、找不到同名属性的那些。cloud 段与底线段共用同一实现 ——
# 两份实现必然漂开,而这两段判的是同一条不变量。
join_missing() {
  local src="$1" comp="$2" have k miss='' payload='' p
  have=$(attr_keys "$src")
  for p in $BASELINE_PAYLOAD; do
    [ "${p%%:*}" = "$comp" ] && payload="$(printf '%s' "${p#*:}" | tr ',' ' ')"
  done
  for k in $(log_keys "$src"); do
    case "$k" in *_ms|error) continue;; esac
    case " $payload " in *" $k "*) continue;; esac
    printf '%s\n' "$have" | grep -qx "$k" || miss="$miss $k"
  done
  printf '%s' "$miss"
}

check_baseline() {
  local comp dir src n=0 k
  for comp in $BASELINE_MEMBERS; do
    dir=$(find starter -maxdepth 2 -type d -name "$comp" 2>/dev/null | head -1)
    [ -n "$dir" ] || { report "[底线] $comp: 目录不存在(成员表漂移)"; continue; }
    n=$((n+1))
    src="$TMPDIR_PREP/base_$(printf '%s' "$comp" | tr '/' '_').go"
    find "$dir" -maxdepth 3 -name '*.go' ! -name '*_test.go' \
      ! -path '*/example/*' ! -path '*/example-*/*' -exec cat {} + > "$src" 2>/dev/null

    grep -qE 'otel\.Meter\(|GetMeterProvider\(\)\.Meter\(' "$src" \
      || report "[底线] $comp: 无自建 instrument —— 单例组件不委托后端,信号应当自己发"

    # 成败可辨。与族规同一策略:能精确取 metric label 就精确取(span 上孤零零一个
    # status 不算"分得出成败"),组件用了变量展开式 WithAttributes 时回退到并集。
    if ! grep -qE 'WithAttributes\( *[A-Za-z_][A-Za-z0-9_]* *(\.\.\.|[,)])' "$src"; then
      printf '%s\n' "$(metric_labels "$src")" | grep -qx status \
        || report "[底线] $comp: metric label 里没有 status(分不出成败)"
    else
      printf '%s\n' "$(attr_keys "$src")" | grep -qx status \
        || report "[底线] $comp: 没有任何名为 status 的属性(分不出成败)"
    fi

    k="$(join_missing "$src" "$comp")"
    [ -z "$k" ] || report "[底线] $comp: 身份类日志键无同名属性(join 不上):$k"

    CLASSIFIED="$CLASSIFIED $comp"
  done
  [ "$n" -gt 0 ] || report "[底线] 未扫到任何成员 —— 本段静默失效"
}

# ── 委托段:可观测完全由共享层提供的组件 ────────────────────────────────
#
# 这些组件自己不建仪器、也不发访问日志 —— 信号全部来自它们装配的共享层
# (starter-http-client 就是:metric 与 span 由 httpx 包里的 otelhttp 发出,每调用
# 的日志由 cloud/governance/resilience 发)。因此这里能查的**不是**"词汇齐不齐",
# 而是"接线还在不在":每条登记必须能 grep 到全部接入点,少一处即报错。
#
# 登记形如 组件:证据1|证据2 —— 竖线分隔,全部都要命中。证据同样在**去注释源码**上匹配。
#
# lock 与 transaction 两族也走这里:它们自己不建仪器,只在装配处把 cloud 侧的装饰器接上
# (lock.WrapLocker / transaction.WithObserver),所以可查的就是"接线还在不在"。
# lock 的证据里**带上后端名**(lock.WrapLocker("redis")) —— 那个字符串同时是指标的 system
# 取值,写错后端名等于把两个后端的遥测混在一起,是这族最该防的一种漂移。
# 新增一个都必须写清:信号来自哪个共享层、本组件为什么不自己发。这条登记比族规弱
# (它证明不了成不成立,只能证明接线没被拆),登记表不是免检牌。
DELEGATES="
  starter-http-client:otelhttp.NewTransport|resilience.WrapExecutor
  starter-oauth2-client:otelhttp.NewTransport|resilience.ExecutorFor\(|resilience.NewRoundTripper\(
  starter-lock-consul:lock.WrapLocker\(\"consul\"
  starter-lock-etcd:lock.WrapLocker\(\"etcd\"
  starter-lock-k8s:lock.WrapLocker\(\"k8s\"
  starter-lock-redis:lock.WrapLocker\(\"redis\"
  starter-transaction-at-gorm:\.WithObserver\(transaction\.AtObserver\{\}\)
  starter-transaction-saga:\.WithObserver\(transaction\.SagaObserver\{\}\)
  starter-transaction-tcc:\.WithObserver\(transaction\.TccObserver\{\}\)
"
# starter-oauth2-client 的每调用日志**来自 resilience 层**,与 starter-http-client 同源:
# 它不自己调 WrapExecutor,而是经 resilience.ExecutorFor 取执行器 —— 而 resolve() 无条件用
# WrapExecutor 包住拿到的执行器(provider 没注册时包的是 noopExecutor),所以观察层照样接上,
# 每次 Execute 一行(resource/system/status/duration_ms),成功 Debug、失败 Warn。
# 曾把这条误判为「没有每调用日志」,是因为只看了 starter 源码里没有 WrapExecutor 字样。
# 故证据里补上 ExecutorFor + NewRoundTripper:拆掉这条接线,日志与指标会一起消失。
#
# 残留边界(登记,未关闭):token 端点的换取走 oauth2 库内部的 base transport,**不经过**
# 这个 RoundTripper,故它只有 span、没有访问日志。库没给逐次换取的钩子,要补得自己复刻
# 库的 token 状态机,不划算。

check_delegates() {
  local e comp dir ev evs src n=0
  for e in $DELEGATES; do
    comp="${e%%:*}"; evs="${e#*:}"
    dir=$(find starter -maxdepth 2 -type d -name "$comp" 2>/dev/null | head -1)
    [ -n "$dir" ] || { report "[委托] $comp: 目录不存在(登记漂移)"; continue; }
    n=$((n+1))
    src="$TMPDIR_PREP/dlg_$(printf '%s' "$comp" | tr '/' '_').go"
    find "$dir" -maxdepth 3 -name '*.go' ! -name '*_test.go' \
      ! -path '*/example/*' ! -path '*/example-*/*' -exec cat {} + > "$src" 2>/dev/null
    code_only "$src" > "$src.code" 2>/dev/null || cp "$src" "$src.code"
    for ev in $(printf '%s' "$evs" | tr '|' ' '); do
      grep -qE "$ev" "$src.code" \
        || report "[委托] $comp: 找不到接入点 /$ev/ —— 接线被拆,该组件可观测归零"
    done
    CLASSIFIED="$CLASSIFIED $comp"
  done
  [ "$n" -gt 0 ] || report "[委托] 未扫到任何组件 —— 本段静默失效"
}

# ── 连接状态指标:报了的必须一直在报 ─────────────────────────────────────
#
# `messaging.client.connection.state_changes` 只在**客户端提供连接状态回调**的成员上出现
# (mqtt/nats/rabbitmq),不进族规的共同清单:对没有这种回调的库,硬要求只能逼出假数据 ——
# 与「goframe 不把位置参数假装成键值对」是同一条道理。
#
# 所以规则是两句话:报了的,必须一直在报(否则一次重构就悄悄丢了);没登记的报了,
# 说明它其实也能观测到 —— 那是规则该更新,不是它违规。
CONN_STATE_METRIC="messaging.client.connection.state_changes"

# 已登记的、上报连接状态的成员。**用目录名**:反向守卫比对的是从路径推导出的名字,
# 两处命名空间不一致就会把已登记的成员误报成"未登记"。
CONN_STATE_MEMBERS="starter-mqtt starter-nats starter-rabbitmq"

check_connection_state() {
  local m dir stray n=0
  for m in $CONN_STATE_MEMBERS; do
    dir=$(find starter -maxdepth 2 -type d -name "$m" 2>/dev/null | head -1)
    [ -n "$dir" ] || { report "[连接状态] $m: 目录不存在(登记漂移)"; continue; }
    n=$((n+1))
    grep -rq "$CONN_STATE_METRIC" --include='*.go' --exclude='*_test.go' "$dir" 2>/dev/null \
      || report "[连接状态] $m: 登记为上报 $CONN_STATE_METRIC,却查不到了"
  done
  [ "$n" -gt 0 ] || report "[连接状态] 未扫到任何登记成员 —— 本段静默失效"

  # 反向:报了却没登记。不是违规,是登记表该补 —— 但必须有人知道。
  stray=''
  for m in $(grep -rl "$CONN_STATE_METRIC" --include='*.go' starter/ 2>/dev/null \
             | grep -v _test | sed 's|^starter/experimental/||; s|^starter/||; s|/.*$||' | sort -u); do
    printf '%s\n' " $CONN_STATE_MEMBERS " | grep -q " $m " || stray="$stray $m"
  done
  [ -z "$stray" ] || report "[连接状态] 上报了该指标却没登记:$stray(补进 CONN_STATE_MEMBERS)"
}

# ── cloud 域包:身份类日志键必须能 join ────────────────────────────────
# cloud 的域包(discovery/lock/resilience/confrefresh …)不是可互换后端,没有"族"可言,
# 但它们都适用同一条不变量 —— discovery/observe.go 里写明、全生态都该满足的那条:
# **失败指标要能落到解释它的那行日志**,所以日志里的身份键必须与某个属性(spans 或 metric
# label)同名。lock 的日志写 key、span 写 lock.key,这条就断了 —— 那正是本段要抓的。
#
# 取值:时长(键以 _ms 结尾)与 error 是日志行自身的载荷,不是身份,豁免。
check_cloud_join() {
  local f d comp k miss src have n=0
  for f in $(grep -rl 'otel\.Meter(\|otel\.Tracer(' --include='*.go' cloud/ 2>/dev/null | grep -v _test | sort); do
    n=$((n+1))
    d=$(dirname "$f"); comp=${d#cloud/}
    # 用独立变量名,不碰族检查共用的 $PREP —— 共享可变全局在这里是不确定性的来源。
    src="$TMPDIR_PREP/cloud_$(printf '%s' "$comp" | tr '/' '_').go"
    find "$d" -maxdepth 2 -name '*.go' ! -name '*_test.go' | sort | xargs cat > "$src" 2>/dev/null
    miss="$(join_missing "$src" "cloud/$comp")"
    [ -z "$miss" ] || report "[cloud] $comp: 身份类日志键无同名属性(join 不上):$miss"
  done
  # 与族检查同一个防漏检要求:扫不到任何域包不是"没有问题",是扫描失效。
  [ "$n" -gt 0 ] || report "[cloud] 未扫到任何域包 —— 本段静默失效"
}

check_registry
check_config
check_connection_state
check_baseline
check_delegates
check_cloud_join

# ── 完整性守卫:注册了访问 tag 却未归入任何族的 starter ─────────────────
# 族成员靠"创建了该族仪器"识别,会漏掉"只打日志"或"指标由库提供"的成员 —— go-redis 与
# starter-kafka 都这样整族漏掉过。漏检比误报危险得多,故反向兜底。
#
# 只认「访问日志 tag」RegisterAppTag(x, "access")。生命周期 tag RegisterAppTag(x, "") 是
# starter 自己的日志分类,不是访问日志 —— config / registry 族的每操作可观测性委托给
# cloud/confrefresh 与 cloud/discovery,本来就没有 per-starter 访问日志,不该被算作漏检。
#
# 谓词是「有插桩迹象」,不是「有 access tag」:后者只覆盖自己发访问日志的 starter,
# 而真正会漏的恰恰是不发的那种(kitex/kratos 的仪器由库发、http-client 连日志都由共享层发)。
# 用纯 otel 谓词同样会漏掉 http-client —— 它自己一行 otel.Tracer( 都没有。
INSTRUMENTED_SIGNAL='otel\.Tracer\(|otel\.Meter\(|GetMeterProvider\(\)\.Meter\(|otelhttp\.|metric\.WithAttributes\(|semconv\.'

unclassified=''
for name in $(grep -rlE "$INSTRUMENTED_SIGNAL" --include='*.go' starter/ 2>/dev/null \
             | grep -v _test | grep -v '/example' \
             | sed 's|^starter/experimental/||; s|^starter/||; s|/.*$||' | sort -u); do
  printf '%s\n' " $CLASSIFIED " | grep -q " $name " || unclassified="$unclassified $name"
done
# 有插桩却没归入任何一节 = 真漏检,置 fail。这条只有在**遍历域足够宽**时才有意义 ——
# 窄谓词下它永远为空,那是"看起来加了守卫、其实什么都没查"。
[ -z "$unclassified" ] || report "未归类(有插桩迹象却没归入任何一节,补一节或补登记):$(printf '%s ' $unclassified)"

# ── 正向清单:必须被插桩、却没有插桩迹象的 starter ──────────────────────
#
# 上面那条抓的是"做了但没归类",这里抓的是"该做而没做" —— 后者更值钱,因为整套模型
# 是**反向**识别的:成员靠"已经建了仪器"发现,一个从未插桩的 starter 对任何检查都隐形。
#
# 登记 = 声明"这个组件应当有可观测能力"。它在清单里而扫描不到插桩,就是缺陷。
EXPECT_INSTRUMENTED="starter-http-server"

for name in $EXPECT_INSTRUMENTED; do
  dir=$(find starter -maxdepth 2 -type d -name "$name" 2>/dev/null | head -1)
  [ -n "$dir" ] || { report "[正向] $name: 目录不存在(清单漂移)"; continue; }
  grep -rqE "$INSTRUMENTED_SIGNAL" --include='*.go' --exclude='*_test.go' \
       --exclude-dir=example --exclude-dir=example-otel "$dir" 2>/dev/null \
    || report "[正向] $name: 登记为必须插桩,却一处插桩迹象都没有"
done

if [ "$fail" -eq 0 ]; then
  echo ""
  echo "observability OK"
else
  echo ""
  echo "共同内容清单见本脚本各族规;规则见 starter/DESIGN.md §3。"
  echo "注:脚本只查\"共同内容齐不齐\",不查\"有没有多余的键\" —— 组件特有字段是允许的。"
  exit 1
fi
