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
# client 侧的"齐整"已从成员挪到**唯一的发射点**上(resilience 的观测层):
# client starter 不再自建 span/指标/访问日志,只**声明**每个操作是什么
# (observability.WithOperation)并把调用路由进 executor。于是:
#   * 声明族(DB、消息、email)—— 查"声明齐不齐",并查它**没有**退回去自建;
#   * 发射点 —— 唯一的发射点一次性满足齐整,在那里查一次,别处不再查。
# 服务端族(HTTP server、RPC)本轮未迁移,仍自建,规则照旧。
#
# 分节(每节一种机制,别把它们的判据互相套用):
#   §族规    自建后端族(HTTP server、RPC):同族共有的部分同名同型。成员靠"建了该族仪器"识别
#   §声明族  client 后端族(DB、消息、email):成员靠"声明了该族的 Metric 前缀"识别
#   §发射点  唯一的发射点一次性满足齐整(span/两级时长/在途/访问日志)
#   §仪器    三条横向规则:tracer 不得缓存、gauge 不用创建期回调、同名同描述符
#   §registry 后端在自己的缝上上报(委托调用点存在性,不是名字)
#   §config   配置源完全委托 observability.RefreshConf:查接线,且不得自建仪器
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

# tagged_log_keys 只取"该组件自己的观测行"(tag 不是 log.TagAppDef 的那种)里的字段键。
#
# 为什么按 tag 收窄:可 join 这条不变量针对的是**解释自己指标/span 的那行日志**。一行
# 走 log.TagAppDef 的日志是这个组件的普通应用日志(启动、路由、调度器的 fire-time 告警),
# 它既不是访问日志、也不带该组件观测词汇的身份,拿它去 join 指标是把规则套错了对象。
# (loadbalance 的 address/label、transaction 的 saga/transaction、scheduling 的 key/task
# 都只出现在 TagAppDef 行上;若连它们也算,规则就会逼着代码把普通日志改成指标词汇。)
#
# 用括号配平剥出每个 log.X(...) 调用,含 log.TagAppDef 的整个跳过 —— 属性列表经常跨行,
# 按行过滤会把同一调用的键一起漏掉。
tagged_log_keys() {
  awk '
    function emit(seg,   k, fn) {
      while (match(seg, /log\.[A-Za-z0-9]+\("[^"]*"/)) {
        k = substr(seg, RSTART, RLENGTH)
        fn = k; sub(/\(.*/, "", fn); sub(/^log\./, "", fn)
        # 与 log_keys 同一个排除表:这些构造器带的字符串是消息/注册名,不是字段键。
        if (fn != "Register" && fn != "BuildTag" && fn != "Msg") {
          sub(/^[^"]*"/, "", k); sub(/"$/, "", k)
          print k
        }
        seg = substr(seg, RSTART + RLENGTH)
      }
    }
    BEGIN {
      while ((getline line < ARGV[1]) > 0) src = src "\n" line
      close(ARGV[1])
      rest = src
      while (match(rest, /log\.[A-Za-z0-9]+\(/)) {
        start = RSTART + RLENGTH
        body = substr(rest, start)
        depth = 1; i = 1
        while (i <= length(body)) {
          c = substr(body, i, 1)
          if (c == "(") depth++
          else if (c == ")") { depth--; if (depth == 0) break }
          i++
        }
        if (substr(body, 1, i - 1) !~ /log\.TagAppDef/) emit(substr(body, 1, i - 1))
        rest = substr(rest, start + i)
      }
    }' "$1" | sort -u
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

# component_src 拼接一个组件目录下全部非测试源码(排除 example*),输出到 $2。
# maxdepth 3,不是 2:日志桥这类内部包在 internal/<pkg>/ 下,是第 3 层
# (kitex/kratos 的 RegisterRPCTag 就写在 internal/logger/logger.go,漏掉 = 静默误报)。
# 排除 example*:示例程序不是组件的插桩,把它们拼进来只会**多出**属性键,
# 从而掩盖真实的缺失 —— 只能造成假绿,不能造成假红。
component_src() {
  find "$1" -maxdepth 3 -name '*.go' ! -name '*_test.go' \
    ! -path '*/example/*' ! -path '*/example-*/*' -exec cat {} + > "$2" 2>/dev/null
}

# comp_of 把组件目录路径收敛成登记用的组件名(去掉 starter/ 与 experimental/ 前缀)。
comp_of() {
  printf '%s' "$1" | sed 's|^starter/experimental/||; s|^starter/||; s|/$||'
}

# ── 检查一个自建族(HTTP server / RPC)────────────────────────────────────
# 参数:族名 成员探测正则 metric 清单(名:仪器类型) label 集 span 属性集
#       日志键集 span-后端登记表 额外成员(目录名,可空) 访问 tag 正则(可空,默认 RegisterAppTag)
#
# span-后端登记表列出「span 由后端自带的 OTel 插桩发出」的成员 —— starter 再发一个
# 就是重复。登记需附接入点证据,脚本会 grep 它;grep 不到 = 登记失效或接线被删,
# 那时就是真的没有 span。只豁免 span 与 span 属性两项:指标仍必须由 starter 自建。
# 新增一个都必须先查证「库给 span 设的属性是否与本族词汇一致」。
#
# metric-后端登记表(全局 $METRIC_FROM_BACKEND)同理,但豁免的是**指标自建**这一整块 ——
# 用于 metric 由第三方库发出的成员(kitex 的观测套件、kratos 的 middleware)。
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
    component_src "$DIR" "$SRC"
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
    # metric + span **双登记**的成员(kitex/kratos)整块跳过:两个信号都由库发,
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
#
# 注:原来的 otelhttp 条(client 侧 http.client.request.duration)已撤 —— client 侧不再
# 自建指标,http-client 的 metric 与 span 现在是 resilience 发射点/otelhttp 的事,
# HTTP server 族的成员里没有 http-client,这条登记已无用武之地。
METRIC_FROM_BACKEND="
  starter-kitex:rpc.server.duration:tracing.NewServerSuite\(\)
  starter-kratos:server_requests_code_total,server_requests_seconds:kmetrics\.Server\(
"

# ── 族规(改这里等于改规约)────────────────────────────────────────────

# CLASSIFIED 累积所有已归节的组件名,末尾的反向守卫靠它兜"有插桩却没归类"。
# (实验目录下的组件在归节时已剥掉 experimental/ 前缀 —— 守卫比对的也是剥过前缀的名字。)
CLASSIFIED=''

# HTTP 族(server 类,本轮未迁移 —— 仍自建)
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

# RPC 族(server 类,本轮未迁移 —— 仍自建)
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

# ── 声明族:client 侧只声明,发射在 resilience 的上报链上 ─────────────────
#
# 参数:族名 Metric 前缀 有界属性键列表(逗号或空格分隔的多个键)
#
# 成员识别靠"声明了本族的 Metric 前缀"(Metric: "db.client"),而不靠"建了本族的仪器" ——
# 后者正是本模型拆掉的东西,再用它识别会让整族静默熄灭(成员一个也扫不到)。
# 因此这里额外要求 WithOperation 在场,免得只写了个前缀字面量的文件被误当成员。
#
# 每个成员查两类、且**方向相反**:
#   正:声明齐不齐 —— WithOperation 在场、Metric 前缀对、有界属性键齐、带 LogTag、
#       自己的 access tag 已注册、调用确实被路由进 executor(声明才读得到);
#   负:没有退回去自建 —— 不得出现 otel.Tracer(span 归发射点)、不得出现
#       Float64Histogram/Int64UpDownCounter(operation 指标归发射点)。
# 齐整本身不在这里查 —— 它是发射点的义务,见 §发射点。
#
# 允许的例外(连接的**非**per-call 遥测):连接状态计数器(mqtt/nats/rabbitmq 的
# Int64Counter(messaging.client.connection.state_changes))不在禁止之列 —— 它由客户端库
# 自己的连接回调驱动,不是每调用信号,归 starter 自己。故负检查只禁 span 与
# duration/in-flight 两类仪器,不去碰 Int64Counter。
declare_members() {
  local prefix="$1" d name src
  for d in starter/*/ starter/experimental/*/; do
    [ -d "$d" ] || continue
    case "$d" in starter/experimental/|*/example/|*/example-*/) continue;; esac
    src=$(find "$d" -maxdepth 3 -name '*.go' ! -name '*_test.go' \
      ! -path '*/example/*' ! -path '*/example-*/*' -exec cat {} + 2>/dev/null)
    # here-string, not `printf | grep -q`: $src is the whole module's source, and
    # under `set -o pipefail` a `grep -q` that matches early closes the pipe,
    # SIGPIPEs the printf, and the non-zero pipeline makes the member look
    # unclassified (a false failure that appears only under load).
    grep -q "Metric: *\"$prefix\"" <<<"$src" || continue
    grep -q 'observability\.WithOperation(' <<<"$src" || continue
    printf '%s\n' "${d%/}"
  done
}

# 声明族成员"有没有退回去自建仪器"的判据。
# descscan 输出仪器清单,每行 `<L|P>\t名字或模板\t仪器类型\t单位\t描述`。
#   L = 名字是字面量;P = 名字由 `前缀+"后缀"` 拼出(记后缀模板)。
# 用 perl 而不是 grep:名字、描述、单位常跨行,且描述本身可能是拼接表达式,需要括号配平。
descscan() {
  cat <<'PERL'
use strict; use warnings;
local $/; my $src = <STDIN> // '';
my %INSTR = map { $_ => 1 } qw(Float64Histogram Int64Histogram Float64Counter Int64Counter
  Float64UpDownCounter Int64UpDownCounter Float64ObservableGauge Int64ObservableGauge
  Float64Gauge Int64Gauge Float64ObservableCounter Int64ObservableCounter);
sub body {
  my ($s, $i) = @_; my $d = 0; my $out = '';
  for (my $j = $i; $j < length($s); $j++) {
    my $c = substr($s, $j, 1);
    if ($c eq '(') { $d++; next if $d == 1; }
    elsif ($c eq ')') { $d--; return $out if $d == 0; }
    $out .= $c;
  }
  return $out;
}
sub topcomma {
  my ($s) = @_; my $d = 0; my $c = '';
  for my $ch (split //, $s) {
    if ($ch =~ /[(\[{]/) { $d++ } elsif ($ch =~ /[)\]}]/) { $d-- }
    elsif ($ch eq ',' && $d == 0) { return $c }
    $c .= $ch;
  }
  return $s;
}
sub opt {
  my ($seg, $name) = @_;
  return '' unless $seg =~ /\Q$name\E\s*\(/;
  my $b = body($seg, $-[0] + length($name));
  $b =~ s/^\s+|\s+$//g; return $b;
}
while ($src =~ /\.([A-Za-z0-9_]+)\s*\(/g) {
  my $instr = $1; next unless $INSTR{$instr};
  my $args = body($src, $-[0] + length($1) + 1);
  my $a1 = topcomma($args); $a1 =~ s/^\s+|\s+$//g;
  my ($kind, $name);
  if ($a1 =~ /^"/) { $kind = 'L'; ($name = $a1) =~ s/^"//; $name =~ s/".*$//s; }
  elsif ($a1 =~ /"([^"]*)"\s*$/) { $kind = 'P'; $name = $1; }
  else { next }
  print join("\t", $kind, $name, $instr, opt($args, 'WithUnit'), opt($args, 'WithDescription')), "\n";
}
PERL
}
#
# 按**指标名**查,不按方法名拼写查:OTel 的仪器构造器是一个封闭的定长集合,而**指标名
# 永远是第一个参数** —— 于是"哪个拼写被禁"这种规则会随 OTel 加新仪器(Int64Gauge…)漂,
# "哪个**指标**被建出来"不会。规则因此是:声明族成员建出的每一个仪器,其名字都必须在
# 登记表里;表里只放**非 per-call** 的信号 —— 它们不在发射点的齐整性范围内,成员可以
# 自建;其余一律归发射点。
#
# 登记的非 per-call 仪器(每项都要能说出"为什么它不是每调用一条"):
#   * messaging.client.connection.state_changes —— 连接状态迁移,与调用次数无关(nats / mqtt / rabbitmq)
#
# 边界(写明白,免得以为它拦得住一切):解析器只认**第一个参数是字面量**的构造调用。名字
# 非常量(如 bigcache 的缓存 gauge,名字来自它的 stats 表)的仪器扫不到,因此也报不出 ——
# 这是刻意的取舍:另一条路是把"非常量名"一律报出来,代价是给一个合法的登记项常年报警。
DECLARE_EXTRA_INSTRUMENTS=" messaging.client.connection.state_changes "

check_self_built_instruments() {
  local fam="$1" comp="$2" code="$3" name
  # 复用 §仪器 那段同一个解析器(它做括号配平,跨行、选项里带括号都不怕),不另写正则:
  # 两处对"什么算一个仪器"的定义必须一致,否则两个规则会互相打架。
  descscan > "$TMPDIR_PREP/descscan.pl"
  while IFS=$'\t' read -r kind name instr unit desc; do
    [ -n "$name" ] || continue
    case "$DECLARE_EXTRA_INSTRUMENTS" in
      *" $name "*) ;;
      *) report "[$fam] $comp: 自建仪器 $name —— per-call 信号归发射点(只有登记的非 per-call 仪器可自建)" ;;
    esac
  done < <(perl "$TMPDIR_PREP/descscan.pl" < "$code")
}

check_declare() {
  local fam="$1" prefix="$2" attrs="$3" needs_kind="${4:-}"
  local d comp src code k n=0 nkind=0
  local members
  members=$(declare_members "$prefix")
  # 与族规同一个防漏检要求:扫不到任何成员不是"这个族还没有成员",是声明字面量漂了。
  [ -n "$members" ] || { report "[$fam] 未扫到任何成员 —— 声明字面量 Metric: \"$prefix\" 可能已与代码漂开(整族静默熄灭)"; return; }

  for d in $members; do
    n=$((n+1))
    comp=$(comp_of "$d")
    CLASSIFIED="$CLASSIFIED $comp"
    src="$TMPDIR_PREP/decl_$(printf '%s' "$comp" | tr '/' '_').go"
    component_src "$d" "$src"
    code="$src.code"
    code_only "$src" > "$code" 2>/dev/null || cp "$src" "$code"

    # 正:声明齐不齐
    grep -q 'observability\.WithOperation(' "$code" \
      || report "[$fam] $comp: 未声明 operation(缺 observability.WithOperation)"
    grep -qE "Metric: *\"$prefix\"" "$code" \
      || report "[$fam] $comp: 声明的 Metric 前缀不是 \"$prefix\""
    for k in $attrs; do
      grep -q "attribute\.String(\"$k\"" "$code" \
        || report "[$fam] $comp: 声明缺有界属性 $k(有界属性进 metric label/span/日志)"
    done
    grep -qE 'LogTag:' "$code" \
      || report "[$fam] $comp: 声明的 Operation 未带 LogTag(访问日志的 tag 归 client 自己)"
    grep -qE 'RegisterAppTag\([^)]*"access"' "$code" \
      || report "[$fam] $comp: 未注册自己的 access 日志 tag"
    grep -qE 'ExecutorFor\(|resilience\.Run\(|exec\.Execute\(' "$code" \
      || report "[$fam] $comp: 调用未路由进 resilience executor —— 声明不会被任何人读到"

    # 负:不得退回自建(齐整归发射点)
    grep -qE 'otel\.Tracer\(' "$code" \
      && report "[$fam] $comp: 自建 span(otel.Tracer)—— span 归发射点,client 只声明"
    check_self_built_instruments "$fam" "$comp" "$code"

    # 消息族额外的一条:声明必须带 span kind。一条 publish / consume 在 trace 上是一条
    # producer→consumer 的边,丢了它就只剩 internal,跨服务的拓扑断在这里。方向由 client
    # 自己知道(只有它知道这次是发还是收),所以判据也落在它的声明上。
    if [ "$needs_kind" = kind ]; then
      nkind=$((nkind+1))
      grep -qE 'SpanKind:' "$code" \
        || report "[$fam] $comp: 声明未带 SpanKind —— 消息族的 producer/consumer 边不能省"
      grep -qE 'SpanKind:[[:space:]]*(spanKind\(|trace\.SpanKind(Producer|Consumer))' "$code" \
        || report "[$fam] $comp: SpanKind 未按方向声明(publish=Producer / consume=Consumer)"
    fi
  done
  # 防漏检:要求查 kind,却一个都没查到,是规则静默失效(前缀漂了 → 没有成员 → 上面已报,
  # 但成员在、只是这条路没走,同样要报)。
  [ "$needs_kind" != kind ] || [ "$nkind" -gt 0 ] \
    || report "[$fam] span kind 规则静默失效(扫到成员但一个都没查)"
  echo "[$fam] 成员 $n 个(声明式)"
}

# ── 声明式 server 成员(方向相反,模型相同)────────────────────────────
#
# 与 §声明族 同一条判据,只是方向相反:route **声明**这个请求是什么,发射点产信号。
# 一个成员迁过来之后,它不再自建族仪器 —— 于是也不再被 §自建族 扫到(那族按
# "建了该族仪器"识人),所以必须在**这里**被查到,否则它就整个处在规则之外了。
#
# 入站比出站多一处:响应码是"活儿干完"才知道的,故额外要求它把响应后半交给
# observability.Response —— 少了这行,声明活在入口、结果永远缺席。
check_declare_server() {
  local fam="$1" comp="$2" prefix="$3" attrs="$4" dir="$5"
  local src code k
  src="$TMPDIR_PREP/declsrv_$(printf '%s' "$comp" | tr '/' '_').go"
  component_src "$dir" "$src"
  code="$src.code"
  code_only "$src" > "$code" 2>/dev/null || cp "$src" "$code"
  CLASSIFIED="$CLASSIFIED $comp"

  grep -q 'observability\.WithOperation(' "$code" \
    || report "[$fam] $comp: 未声明 operation(缺 observability.WithOperation)"
  grep -qE "Metric: *\"$prefix\"" "$code" \
    || report "[$fam] $comp: 声明缺 Metric: \"$prefix\""
  for k in $attrs; do
    grep -q "attribute\.String(\"$k\"" "$code" \
      || report "[$fam] $comp: 声明缺有界属性 $k"
  done
  grep -qE 'LogTag:' "$code" \
    || report "[$fam] $comp: 声明的 Operation 未带 LogTag"
  grep -qE 'RegisterAppTag\([^)]*"access"' "$code" \
    || report "[$fam] $comp: 未注册自己的 access 日志 tag"
  grep -qE 'exec\.Execute\(' "$code" \
    || report "[$fam] $comp: 请求未路由进 resilience executor —— 声明不会被任何人读到"
  grep -qE 'observability\.ResponseFrom\(' "$code" \
    || report "[$fam] $comp: 未记录响应后半(observability.Response)—— 响应码只有 handler 知道"

  # 负:不得退回自建,与 §声明族 同一口径(按指标名查)
  grep -qE 'otel\.Tracer\(' "$code" \
    && report "[$fam] $comp: 自建 span(otel.Tracer)—— span 归发射点"
  check_self_built_instruments "$fam" "$comp" "$code"
  echo "[$fam] $comp: 声明式入站(route 声明 + 发射点产信号)"
}

check_declare_server HTTP starter-http-server 'http.server' 'http.request.method' starter/starter-http-server
check_declare_server HTTP starter-echo 'http.server' 'http.request.method' starter/starter-echo

check_declare DB 'db.client' 'db.system db.operation'
check_declare 消息 'messaging.client' 'messaging.system messaging.operation' kind
check_declare email 'email.client' 'email.system email.operation'

# ── 登记缺口:mongodb 在 command 层自建发射 ──────────────────────────────
#
# starter-mongodb 是**唯一**不走声明式的 client:它必须在自己的 command 层自建发射。
# 原因(登记在 starter/DESIGN{,_CN}.md §3 的已登记缺口):mongo driver v2 没有 per-command
# 的 execute 钩子(只有 SetMonitor / 各种 observer,都是"只看不能拦"),所以 resilience
# executor 只在 **dial** 缝上可达 —— 而连接池让 dial 很稀少,把每 command 信号挂上去就等于
# 没有。于是它用 CommandMonitor 观测(纯观察者,不能返回错误),保护仍留在 dial 缝。
#
# 正因为它是自建,这段查的就该是"自建得对不对":词汇与发射点一致(同两个 metric 名、
# 同两个有界 label、statement 只进 span/日志),并且**保护确实在 dial 层**。
# 它不在 §声明族 里,所以不会被那里"不得自建"的负检查误伤。
check_mongodb_gap() {
  local d="starter/experimental/starter-mongodb" src k
  [ -d "$d" ] || { report "[DB] 登记缺口 starter-mongodb: 目录不存在(登记漂移)"; return; }
  CLASSIFIED="$CLASSIFIED starter-mongodb"
  src="$TMPDIR_PREP/mongo.go"
  component_src "$d" "$src"
  code_only "$src" > "$src.code" 2>/dev/null || cp "$src" "$src.code"

  grep -qE 'Float64Histogram\("db\.client\.operation\.duration"' "$src.code" \
    || report "[DB] starter-mongodb: 缺调用级时长 db.client.operation.duration(词汇须与发射点同名)"
  grep -qE 'Int64UpDownCounter\("db\.client\.active_requests"' "$src.code" \
    || report "[DB] starter-mongodb: 缺在途 gauge db.client.active_requests(词汇须与发射点同名)"
  grep -qE 'otel\.Tracer\(' "$src.code" \
    || report "[DB] starter-mongodb: 无 span(未起 tracer)"
  grep -qE 'RegisterAppTag\("mongodb", "access"\)' "$src.code" \
    || report "[DB] starter-mongodb: 无自己的 access 日志 tag"
  for k in db.system db.operation db.statement; do
    grep -q "attribute\.String(\"$k\"" "$src.code" \
      || report "[DB] starter-mongodb: 缺属性 $k(db.system/db.operation 有界,db.statement 只进 span/日志)"
  done
  grep -qE '"status"' "$src.code" \
    || report "[DB] starter-mongodb: 访问日志/metric 缺 status(分不出成败)"
  grep -qE 'log\.Float\("duration_ms"' "$src.code" \
    || report "[DB] starter-mongodb: 访问日志缺 duration_ms"
  # 保护在 dial 缝 —— 这是它"自建发射"这一豁免的另一半,少了它就成了纯自建无治理。
  grep -qE 'resilience\.NewDialer\(' "$src.code" \
    || report "[DB] starter-mongodb: 未在 dial 缝接保护(resilience.NewDialer)—— 已登记的豁免前提不成立"
  echo "[DB] 登记缺口 starter-mongodb:command 层自建发射(dial 层保护)"
}

check_mongodb_gap

# ── 发射点:唯一的发射点一次性满足齐整 ──────────────────────────────────
#
# 完整性这条原则以前逐组件查("每个组件都要有 span/时长/在途/一行访问日志")。现在它
# 是**一个**发射点的义务,故在这里查一次 —— 客户端的齐整靠声明(见 §声明族),
# 发射点靠这段。查的是 cloud/resilience/observe.go:
#   * 调用级 span(otel.Tracer( 开的那个),形状是 SpanKindInternal;
#   * 两级时长:operation.duration(整次调用,含重试与退避)与 attempt.duration(每次下游尝试);
#   * 在途 gauge active_requests(整次调用,与 span 同跨度);
#   * 永远开启的 resilience.client.calls{status}(不分声明与否,落进兜底路径);
#   * 每调用一行访问日志,且分级正确:失败 Warn、带 Detail 的成功 Debug(惰性)、
#     不带 Detail 的成功 Info;状态与时长字段是 status / duration_ms。
#   * Detail 不得进 metric label —— 发射时给指标用的只有 Attrs(callLabels(op.Attrs, ...))。
check_emitter() {
  local f="cloud/resilience/observe.go"
  [ -f "$f" ] || { report "[发射点] $f 不存在 —— 发射点漂移,client 侧信号无处发出"; return; }

  grep -qE 'otel\.Tracer\(' "$f" \
    || report "[发射点] 未开调用级 span"
  grep -qE 'prefix\+"\.operation\.duration"' "$f" \
    || report "[发射点] 缺调用级时长 <prefix>.operation.duration"
  grep -qE 'prefix\+"\.attempt\.duration"' "$f" \
    || report "[发射点] 缺尝试级时长 <prefix>.attempt.duration"
  grep -qE 'prefix\+"\.active_requests"' "$f" \
    || report "[发射点] 缺在途 gauge <prefix>.active_requests"
  grep -qE '"resilience\.client\.calls"' "$f" \
    || report "[发射点] 缺永远开启的 resilience.client.calls{status}"
  # 每调用一行访问日志,且分级覆盖三档。分开断言:合成一条会让"少了某一档"混过去。
  grep -qE 'log\.Warn\(' "$f" \
    || report "[发射点] 访问日志缺失败档(Warn)"
  grep -qE 'log\.Debug\(' "$f" \
    || report "[发射点] 访问日志缺带 Detail 的成功档(Debug,惰性)"
  grep -qE 'log\.Info\(' "$f" \
    || report "[发射点] 访问日志缺无 Detail 的成功档(Info)"
  grep -qE 'log\.Float\("duration_ms"' "$f" \
    || report "[发射点] 访问日志缺 duration_ms"
  grep -qE '"status"' "$f" \
    || report "[发射点] 访问日志缺 status"
  # Detail 与 Attrs 的分界:给指标上标签的必须是 callLabels(op.Attrs, ...),不是 op.Detail。
  grep -qE 'callLabels\(op\.Attrs' "$f" \
    || report "[发射点] metric label 未限定在声明 Attrs(Detail 可能进了 label)"
  # Detail 的另一半:它还必须进 span 属性 —— Detail 的定义是"span + 日志",只进日志就
  # 漏了一半。断言落在 spanAttrs 的函数体上,不是全文:全文里 op.Detail 也出现在写日志
  # 的那一行,全文 grep 等于没查(这正是这条规则此前"没有执行面"的原因)。
  local spanattrs
  spanattrs="$(awk '/^func .*spanAttrs\(/{f=1} f{print} f && /^}/{exit}' "$f")"
  grep -q 'op\.Detail' <<<"$spanattrs" \
    || report "[发射点] Detail 未进 span 属性(spanAttrs 里没有 op.Detail)—— Detail = span + 日志"
  echo "[发射点] 单点发射(span/两级时长/在途/status 计数/访问日志)"
}

# ── 仪器规约:三条横向规则(不属于任何族)────────────────────────────────
#
# 逐文件扫 cloud/ 与 starter/ 的非测试、非 example 源码(注释先剥掉:注释里提到不算
# 证据)。三条规则都无族可依 —— 族的成员靠"建了该族仪器"识别,而这三条问的是**仪器本身
# 建得对不对**,任何文件都适用:
#
#   1. **tracer 不得缓存。** `otel.Tracer(...)` 必须在用点现调(紧跟 `.Start(`)。存进包级
#      变量或结构体字段,provider 更换后缓存指向旧 provider —— 之后 span 静默丢失,而代码
#      看上去完全正常。
#   2. **gauge 不得用创建期回调。** ObservableGauge 只能用 `meter.RegisterCallback` 挂回调。
#      创建期回调(`metric.WithInt64Callback`)在仪器尚不可观测时会被丢弃,且不可追加、不可
#      注销;RegisterCallback 可多回调、可注销。
#   3. **同名同描述符。** 同一个 metric 名在不同模块必须同型、同单位、同描述 —— OTel 按
#      (name, unit, description) 去重,同进程内第二个注册者的描述被静默丢弃。名字是运行时
#      拼出来的(`prefix+".operation.duration"`)按各自模板比对,不会与字面名互撞。
#
# 每条都带"扫不到即失败"的兜底:空结果不是"没有问题",是扫描坏了 —— 与各族同一口径。

# KNOWN_MULTI_DESC:同一 metric 名存在多于一种描述、当前**接受**的登记表。登记的是"待统一
# 的既有分歧",不是设计许可。新增一项必须在此登记并同步 starter/DESIGN{,_CN}.md §3。
#   * http./rpc.server.* —— 服务端族本轮未迁移(见 starter/DESIGN.md §3),各 starter
#     各写各的措辞;迁移时统一到 OTel semconv 的官方描述。
# messaging.client.connection.state_changes 曾在此:三家把 system 名写进了描述,已统一为
# 中性措辞("reported by the messaging client")并从表里摘除 —— system 由 messaging.system
# 标签表达,描述里再写一遍是同一个信息两处维护。
#   * `.active_requests` —— 这一项**不是待统一**,是扫描器的固有限制:名字是 `前缀+后缀`
#     模板时,扫描器只看得见后缀,而 resilience 的发射点里有**两个**同后缀的构造器
#     (客户端在途 `operationInstruments`、入站在途 `serverOperationInstruments`),它们的
#     真名不同(db.client.active_requests vs http.server.active_requests)、描述本就该不同。
#     按后缀比等于拿两个指标互相比 —— 与其为了讨好扫描器把两者描述改成同一句(那是拿文档
#     迁就工具),不如把这条限制登记下来。字面量名字不受影响,仍全局严格比。
KNOWN_MULTI_DESC="http.server.request.duration http.server.active_requests
rpc.server.request.duration rpc.server.active_requests rpc.server.request_count
.active_requests"


check_instrument_hygiene() {
  local f code hit entries known new n_tracer=0 n_gauge=0
  descscan > "$TMPDIR_PREP/descscan.pl"
  : > "$TMPDIR_PREP/metrics.tsv"
  for f in $(find cloud starter -name '*.go' ! -name '*_test.go' \
             ! -path '*/example/*' ! -path '*/example-*/*' 2>/dev/null | sort); do
    code="$TMPDIR_PREP/hyg_$(printf '%s' "$f" | tr '/' '_')"
    code_only "$f" > "$code" 2>/dev/null || cp "$f" "$code"

    if grep -q 'otel\.Tracer(' "$code"; then
      n_tracer=$((n_tracer + 1))
      # 现调形态是 `otel.Tracer(x).Start(...)` —— 同一行必带 .Start(。带不上的就是被拿走了。
      hit="$(grep -n 'otel\.Tracer(' "$code" | grep -v '\.Start(')"
      [ -z "$hit" ] || report "[仪器] $f: tracer 非用点现调(疑被缓存):$hit"
      grep -qE '^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]+(oteltrace|trace)\.Tracer[[:space:]]' "$code" \
        && report "[仪器] $f: 结构体字段缓存了 tracer —— tracer 必须用点现调"
    fi

    if grep -q 'ObservableGauge(' "$code"; then
      n_gauge=$((n_gauge + 1))
      grep -qE 'With(Int64|Float64)Callback\(' "$code" \
        && report "[仪器] $f: ObservableGauge 用了创建期回调 —— 应为 meter.RegisterCallback"
    fi

    perl "$TMPDIR_PREP/descscan.pl" < "$code" \
      | awk -F'\t' -v F="$f" '{print $0"\t"F}' >> "$TMPDIR_PREP/metrics.tsv"
  done
  [ "$n_tracer" -gt 0 ] || report "[仪器] 扫不到任何 otel.Tracer( —— tracer 规则静默失效"
  [ "$n_gauge"  -gt 0 ] || report "[仪器] 扫不到任何 ObservableGauge( —— gauge 规则静默失效"
  entries=$(wc -l < "$TMPDIR_PREP/metrics.tsv" | tr -d ' ')
  [ "$entries" -gt 0 ] || report "[仪器] 扫不到任何 metric 仪器 —— 同名同描述符规则静默失效"

  # 同名不同(型|单位|描述)的项。名字碰巧相同的不同仪器(如 P 模板与 L 字面名)不会互撞,
  # 因为键就是名字本身。
  new=''
  local known_list
  known_list=" $(printf '%s' "$KNOWN_MULTI_DESC" | tr '\n' ' ') "
  for known in $(awk -F'\t' '
      { k=$2; v=$3"|"$4"|"$5; if (!(k SUBSEP v in s)) { s[k SUBSEP v]=1; n[k]++ } }
      END { for (k in n) if (n[k] > 1) print k }
    ' "$TMPDIR_PREP/metrics.tsv" | sort); do
    printf '%s\n' "$known_list" | grep -q " $known " || new="$new $known"
  done
  [ -z "$new" ] || report "[仪器] 同名不同描述符(未登记):$(printf '%s ' $new)—— 统一措辞,或登记进 KNOWN_MULTI_DESC"
  echo "[仪器] tracer 现调 $n_tracer 文件 / gauge 回调 $n_gauge 文件 / 仪器 $entries 条"
}

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
        # 上报走块自己的 observer 层(cloud/discovery.Observer),不写包级状态:
        # 匹配的是方法名,实例变量名随各后端自便。
        has_call '\.RegisterAttempt(' \
          || report "[registry] $b: 未上报 RegisterAttempt(注册,含自愈重注册)"
        has_call '\.DeregisterAttempt(' \
          || report "[registry] $b: 未上报 DeregisterAttempt"
        has_call '\.WeightChange(' \
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
    synced=$(grep -ro '\.Synced(' --include='*.go' --exclude='*_test.go' "$dir" 2>/dev/null | wc -l | tr -d ' ')
    [ "$synced" -ge 2 ] \
      || report "[registry] $b: 同步上报调用仅 $synced 处(需要成功与失败两侧)"
  done
}

# ── config 族:完全委托 observability.RefreshConf ───────────────────────
#
# 七个配置源 starter 自身零插桩:每条配置的刷新由 cloud/observability 的 RefreshConf
# 漏斗观测(config.refresh.total / duration / last_success),starter 只负责把它接上。
# (旧的 cloud/confrefresh 已并入 observability,故认的是 observability.RefreshConf。)
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

    n=$(grep -roE 'observability\.RefreshConf\(ctx, gs\.RefreshProperties\)' --include='*.go' --exclude='*_test.go' "$dir" 2>/dev/null | wc -l | tr -d ' ')
    [ "$n" -eq 1 ] \
      || report "[config] $b: observability.RefreshConf 调用 $n 处(需要恰好 1 处,多一处即重复刷新)"

    grep -rqE 'RegisterAppTag\("config_' --include='*.go' --exclude='*_test.go' "$dir" 2>/dev/null \
      || report "[config] $b: 无 config_<backend> 生命周期 tag"

    # 回归绊线:配置源的插桩全在 cloud/observability 的刷新漏斗,本地出现仪器就是重复上报。
    grep -rqE 'Float64Histogram\(|Int64Counter\(|Int64UpDownCounter\(|metric\.WithAttributes\(|otel\.Tracer\(' \
         --include='*.go' --exclude='*_test.go' "$dir" 2>/dev/null \
      && report "[config] $b: 自建插桩 —— config 族的可观测性完全委托 observability.RefreshConf,不该有本地仪器"

    CLASSIFIED="$CLASSIFIED starter-config-$b"
  done
}

# ── 底线段:自建插桩、但不成族的单例组件 ─────────────────────────────────
#
# config-bus 与 gateway 各自是独一无二的能力(配置总线、网关),彼此不可互换,
# 所以**共同性没有对象** —— 它们之间不需要同名,也无从"换个后端看板还得改"。
# 于是就只剩两条硬要求,也就是本段查的东西:
#
#   1. 成败可辨:必须有名为 status 的属性(metric label)。
#      "分不出成败"是缺陷,与它在哪个族无关。
#   2. 可 join:身份类日志键必须与某个属性同名,否则失败指标落不到解释它的日志行上。
#      时长(键以 _ms 结尾)与 error 是日志行自身的载荷,不是身份,豁免;
#      组件特有的叙述性字段在 PAYLOAD_KEYS 里登记豁免。
#      join 只在"该组件自己的观测行"上判定,TagAppDef 的普通应用日志不在范围内
#      (见 tagged_log_keys)。
#
# 另加一条自建断言:这些组件不委托任何后端,信号应当自己发 —— 没有 otel.Meter( 说明
# 插桩被拆掉了(或这个组件根本还没插桩,那它该进正向清单而不是这里)。
#
# 注:scheduler 与 mail 已不在本段。scheduler 自身零插桩,信号全在 cloud/scheduling
# (见 §委托);mail 走声明式(见 §声明族的 email),不再是"自建插桩的单例"。
BASELINE_MEMBERS="starter-config-bus starter-gateway"

# 载荷豁免:日志行自身的叙述,不是被观测实体的身份。新增一个都要在这里登记并说明理由。
# key 形如 `<组件名>:<键,逗号分隔>`;cloud 段的组件名前缀是 `cloud/`。
PAYLOAD_KEYS="
  starter-config-bus:origin,watched,subject
  cloud/scheduling:reason
# prefix 不再豁免:它已是 metric label(见 observe.go 的 record)
"

# join_missing 打印身份类日志键里、找不到同名属性的那些。cloud 段与底线段共用同一实现 ——
# 两份实现必然漂开,而这两段判的是同一条不变量。
# 只取该组件自己的观测行(tagged_log_keys):TagAppDef 的普通应用日志不是"解释信号的
# 那行",拿它去 join 指标是把规则套错了对象。
join_missing() {
  local src="$1" comp="$2" have k miss='' payload='' p
  have=$(attr_keys "$src")
  for p in $PAYLOAD_KEYS; do
    [ "${p%%:*}" = "$comp" ] && payload="$(printf '%s' "${p#*:}" | tr ',' ' ')"
  done
  for k in $(tagged_log_keys "$src"); do
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
    component_src "$dir" "$src"

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
# 这些组件自己不建仪器、也不发每调用信号 —— 信号全部来自它们装配的共享层
# (starter-http-client:metric 与 span 由 httpx 包里的 otelhttp 发出,每调用的日志由
# cloud/resilience 的发射点发;scheduler 的 span/指标/日志全在 cloud/scheduling)。
# 因此这里能查的**不是**"词汇齐不齐",而是"接线还在不在":每条登记必须能 grep 到全部
# 接入点,少一处即报错。
#
# 登记形如 组件:证据1|证据2 —— 竖线分隔,全部都要命中。证据同样在**去注释源码**上匹配。
#
# lock 与 transaction 两族也走这里:它们自己不建仪器,只在装配处把 cloud 侧的装饰器接上
# (lock.Observe / transaction.WithObserver),所以可查的就是"接线还在不在"。
# lock 的证据里**带上后端名**(lock.Observe(inner, "redis")) —— 那个字符串同时是指标的
# system 取值,写错后端名等于把两个后端的遥测混在一起,是这族最该防的一种漂移。
# 新增一个都必须写清:信号来自哪个共享层、本组件为什么不自己发。这条登记比族规弱
# (它证明不了成不成立,只能证明接线没被拆),登记表不是免检牌。
DELEGATES="
  starter-http-client:otelhttp.NewTransport|ClientExecutorFor\(
  starter-oauth2-client:otelhttp.NewTransport|resilience.NewRoundTripper\(
  starter-lock-consul:lock.Observe\(inner,[[:space:]]*\"consul\"
  starter-lock-etcd:lock.Observe\(inner,[[:space:]]*\"etcd\"
  starter-lock-k8s:lock.Observe\(inner,[[:space:]]*\"k8s\"
  starter-lock-redis:lock.Observe\(inner,[[:space:]]*\"redis\"
  starter-scheduler:scheduling.NewScheduler\(
  starter-transaction-at-gorm:\.WithObserver\(transaction\.AtObserver\{\}\)
  starter-transaction-saga:\.WithObserver\(transaction\.SagaObserver\{\}\)
  starter-transaction-tcc:\.WithObserver\(transaction\.TccObserver\{\}\)
"
# starter-http-client / starter-oauth2-client 的每调用日志**来自 resilience 发射点**:
# http-client 经 httpx 用注入的 Manager 的 ClientExecutorFor 拿执行器(包裹与观测都在
# Manager 内完成);oauth2-client 则用 resilience.NewRoundTripper 把执行器装到自己的
# http.Client 上。两者都在装配处把执行器接上,拆掉这条接线,日志与指标会一起消失。
#
# 残留边界(登记,未关闭):oauth2 token 端点的换取走 oauth2 库内部的 base transport,
# **不经过**这个 RoundTripper,故它只有 span、没有访问日志。库没给逐次换取的钩子,
# 要补得自己复刻库的 token 状态机,不划算。

check_delegates() {
  local e comp dir ev evs src n=0
  for e in $DELEGATES; do
    comp="${e%%:*}"; evs="${e#*:}"
    dir=$(find starter -maxdepth 2 -type d -name "$comp" 2>/dev/null | head -1)
    [ -n "$dir" ] || { report "[委托] $comp: 目录不存在(登记漂移)"; continue; }
    n=$((n+1))
    src="$TMPDIR_PREP/dlg_$(printf '%s' "$comp" | tr '/' '_').go"
    component_src "$dir" "$src"
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
# 它也不是每调用信号,所以不在 §声明族 的"不得自建仪器"之列(那是 span 与 duration/in-flight)。
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
# cloud 的域包(discovery/lock/resilience/scheduling …)不是可互换后端,没有"族"可言,
# 但它们都适用同一条不变量 —— discovery/observe.go 里写明、全生态都该满足的那条:
# **失败指标要能落到解释它的那行日志**,所以日志里的身份键必须与某个属性(spans 或 metric
# label)同名。lock 的日志写 key、span 写 lock.key,这条就断了 —— 那正是本段要抓的。
#
# 判定只落在该包自己的观测行上(tagged_log_keys):走 log.TagAppDef 的普通应用日志
# (loadbalance 的端点告警、transaction 的提交叙述、scheduling 的 fire-time 告警)不算
# "解释信号的那行",规则不套它们。
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

check_emitter
check_instrument_hygiene
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
# cloud/observability 与 cloud/discovery,本来就没有 per-starter 访问日志,不该被算作漏检。
#
# 谓词是「有插桩迹象」,不是「有 access tag」:后者只覆盖自己发访问日志的 starter,
# 而真正会漏的恰恰是不发的那种(kitex/kratos 的仪器由库发、http-client 连日志都由共享层发)。
# 用纯 otel 谓词同样会漏掉 http-client —— 它自己一行 otel.Tracer( 都没有。
# 声明迹象(observability.WithOperation() 也算:声明式成员只声明、不自建仪器,
# 少这一项它们会被漏掉。
INSTRUMENTED_SIGNAL='otel\.Tracer\(|otel\.Meter\(|GetMeterProvider\(\)\.Meter\(|otelhttp\.|metric\.WithAttributes\(|semconv\.|observability\.WithOperation\(|GetTracerProvider\(\)\.Tracer\('

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
EXPECT_INSTRUMENTED="starter-http-server starter-webhook starter-milvus"

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
