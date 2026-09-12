# starter-governance 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均核对过 starter 源码
（`starter.go`、`wiring.go`、`source_file.go`、`source_http.go`、`rules/rules.go`、
`wiring_test.go`、`source_file_test.go`、`source_http_test.go`）、它接线的核心包
（`cloud/governance`：`govern.go`、`source.go`、`global.go`、`fault/config.go`、
`resilience/config.go`）以及可运行的 [example/](example/)（`example/main.go` 每秒打印
解析出的 policy 与 fault 配置，提示你在线编辑规则文件，6 秒后自动退出，`-manual` 可保持运行）。

**这个 starter 是什么**：gs 与容器无关治理核心（`cloud/governance`）之间的接线。blank import
后不配置则完全惰性。两重身份：

1. **接线**（常驻注册，`wiring.go`）：把注入的 `governance.Source` bean 交给治理中心，注册
   executor/fault seam，并标记 authority 为 live。
2. **Source 适配器**（条件注册，`source_file.go` / `source_http.go`）：配置了 `govern.source.*`
   key 后，一个 `governance.Source` bean 被注入 wiring；规则变更**只刷新治理**——绝不触发
   全应用属性 re-bind。

治理配置是它自己的一份文档（规则文件、控制台、配置中心），**不写进 `app.properties`**；
app.properties 里只放那一个 `govern.source.*` 引导 key。

治理语义本身（policy 解析、breaker/retry/ratelimit 行为、故障注入模型）见 `cloud/governance`
文档——下面全部是 starter 的接线增量加上使用契约所需的部分。

---

## 1. 完整工程示例

一个治理规则放在独立文件、支持在线热更的最小服务。文件树（即 checked-in 的 example，原样）：

```
demo/
├── go.mod
├── main.go
└── conf/
    ├── app.properties
    └── govern.yaml
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-governance latest
)
```

**main.go**：

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "time"

    "go-spring.org/cloud/governance"
    "go-spring.org/cloud/governance/fault"
    "go-spring.org/spring/gs"

    _ "go-spring.org/starter-governance"
)

// printer 每秒打印一个资源 label 解析出的 policy 与 fault 注入配置——
// resilience 与 fault 走同一个 source。
type printer struct{}

func (p *printer) Run(ctx context.Context) error {
    tk := time.NewTicker(time.Second)
    defer tk.Stop()
    for i := 0; ; i++ {
        p := governance.PolicyFor("demo:resource")
        fmt.Printf("policy: enabled=%v timeout=%v retries=%d rate-limit=%v",
            !p.IsZero(), p.Timeout, p.MaxRetries, p.RateLimit)
        if in := fault.InjectorFor(); in != nil {
            c := in.Config()
            fmt.Printf(" | fault: enabled=%v rate=%v", c.Enabled, c.Rate)
        }
        fmt.Println()
        if i == 3 {
            fmt.Println(">>> edit conf/govern.yaml now: policy AND fault toggle live")
        }
        select {
        case <-ctx.Done():
            return nil
        case <-tk.C:
        }
    }
}

func init() {
    gs.Provide(&printer{}).Export(gs.As[gs.Runner]())
}

func main() {
    manual := flag.Bool("manual", false, "run in manual verification mode (stay up until killed)")
    flag.Parse()
    go func() {
        time.Sleep(6 * time.Second) // 留时间给操作者编辑，然后退出
        _ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
    }()
    gs.Run()
}
```

**conf/app.properties** —— 一行即全部接线：

```properties
# 治理规则放在自己的文件里，由 starter-governance 的 file source 监听——
# 这里没有任何 govern.* 规则。这个 key 激活 starter 的条件 module，其
# Source bean 注入治理中心。
govern.source.file.path=conf/govern.yaml
```

**conf/govern.yaml** —— 键就是 `govern.*` 命名空间，在线监听：

```yaml
govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
    max-retries: 2
  fault:
    enabled: false        # 运行中改为 true —— 见 §4.2
    rate: 0.5
  rules:
    - resources: demo:resource
      attempt-timeout: 50ms
```

**验证**（在 example 目录；应用 6 秒后自灭，除非 `-manual`）：

```bash
go run . -manual
# policy: enabled=true timeout=100ms retries=2 rate-limit=0 | fault: enabled=false rate=0.5
# >>> edit conf/govern.yaml now: policy AND fault toggle live
#   ... 把 attempt-timeout 改成 77ms，或 fault.enabled 改成 true ...
# policy: enabled=true timeout=77ms retries=2 rate-limit=0 | fault: enabled=true rate=0.5
```

保存后 ~1 秒内生效（fsnotify），无需重启、无全应用 re-bind。

**变体 —— 换一个 source**：把 `govern.source.file.path` 换成 `govern.source.http.*`（控制台），
或用各自模块的 nacos/etcd source key。每个进程只有一个活跃 source；已经不存在
app.properties 内嵌规则的路径——规则一律经 Source 进入。

**变体 —— 与 server starter 组合**：任何接到中立 seam（`resilience.ExecutorFor(system, label)`、
`fault.InjectorFor()`）的 client starter 都自动获得 policy，应用代码零改动。HTTP server 侧
`scope: loadtest` 的完整放火演练见 starter-echo 的 USAGE §4.4。

---

## 2. 装配与时序

### 2.1 Source 契约 —— 中心真正依赖的东西

`cloud/governance` 容器无关：其 `center` 只依赖两方法的 `Source` 接口
（`cloud/governance/source.go:43`）：

```go
type Source interface {
    Snapshot() Config                       // 最新已提交值；推送前为零 Config
    Subscribe(cb func(Config))              // 每份新配置提交后回调
}
```

刻意的省略（来自接口文档注释）：

- **无错误返回** —— 中心无法回滚坏推送。"你推的一切你兜底"：坏文档保留上一份好配置是
  source 的职责（这里两个适配器都这么做）。
- **接口无 Close** —— 生命周期属于实现；中心在 Destroy 时对 `interface{ Close() error }`
  做类型断言，顺带关闭恰好实现了它的 source。
- **单一回调** —— 中心是唯一消费者；第二次 Subscribe 可以替换第一次。

wiring 从注入到 `wiring.Src` 的 bean 选取 source（`wiring.go:38-43`）。
适配器家族：本 starter 的 `FileSource`（fsnotify）、`HTTPSource`（定间隔轮询）；
nacos（ListenConfig 推送）在 `starter-governance-nacos`，etcd（Watch 推送）在
`starter-governance-etcd`；`governance.PushSource` 用于手写推送集成。

### 2.2 bean 生命周期时间线

```
import starter-governance
  ├─ init() wiring.go: gs.Provide(newWiring)
  │      .Init((*wiring).Init).Destroy((*wiring).Destroy)
  │      .Export(gs.As[gs.Rooter]())            ← 见下文，这堵墙 + 这个修复
  └─ init() starter.go: 两个条件 gs.Module bean
         ├─ OnProperty("govern.source.file")  → *FileSource, Export(As[governance.Source]())
         └─ OnProperty("govern.source.http")  → *HTTPSource, Export(As[governance.Source]())
              （OnProperty 是前缀匹配：任一 govern.source.<x>.* key 即激活）

gs.Run()
  ├─ config bind: 源配置来自 ${govern.source.file|http.*}（expr 校验）
  ├─ bean wiring: source bean 的 Export 让它可见；被字段注入到
  │      wiring.Src（`autowire:"?"` 可空——无 bean ⇒ nil ⇒ 治理保持 disabled）
  ├─ wiring.Init():
  │      BindDefault(Src)   （nil 安全：没有 source bean ⇒ 保持 disabled）
  │        ├─ 已显式 SetSource?  → BindDefault 无操作（SetSource 胜出）
  │        ├─ bindSource: 订阅（按 handle 指针做过期保护）+ 采纳 Snapshot()
  │      GoLive(): 从快照建全进程 *fault.Injector,
  │        resilience.RegisterExecutorProvider, fault.RegisterInjector, markLive()
  │        （→ 触发所有排队的 OnReady 回调）
  ├─ source bean Init: Start() —— fsnotify 监听 / 轮询循环开始
  ├─ 你的 Runner 运行（policy 已 arm —— Rooter 先于 Runner）
  └─ SIGTERM: wiring.Destroy() → governance.CloseActiveSource()
        （当且仅当 source 实现了 Close 才关闭；FileSource 停 watcher,
         HTTPSource 取消轮询循环）
```

两个值得知道的设计点（均来自源码注释，已核对）：

- **Rooter export 是承重的。** `wiring` bean 不被任何东西注入，而 gs 不会实例化既未导出又
  未被注入的 bean——没有 `Export(gs.As[gs.Rooter]())` 生产环境不会触发任何注册
  （`wiring.go:53-58`）。同理，你自己的自定义 Source bean 若不 `Export(gs.As[governance.Source]())`
  就对接口注入不可见——少了 Export 会让治理静默 disabled。
- **Center 永不进容器。** `cloud/governance` 只暴露包级函数；单例（`global.go:42`）由
  wiring bean 原地 arm。包外拿不到 `*center`。

### 2.3 Rooter 与 Runner 的顺序，以及 OnReady 为什么存在

gs 先收集运行 `gs.Rooter` 再运行 `gs.Runner`。wiring bean 是 Rooter，所以你的 Runner 启动前
治理已 arm——常见场景无需任何处理。但本身就是 Rooter 的推送型调用方（如 starter-dubbo 的
poller）可能在 wiring Rooter arm 治理之前初始化。`governance.OnReady(cb)`
（`global.go:145-158`）不依赖 bean 顺序地解决它：cb 排队直到 authority live，然后恰好触发
一次；已 live 则立即触发。双重检查加锁保证无论哪边赢得竞争，每个回调恰好执行一次。

### 2.4 一次规则编辑的端到端走读

1. 你保存 `conf/govern.yaml`（任何编辑器；原子重命名也行——watcher 监听的是父**目录**而非
   文件本身，正因为 rename 会换 inode（`source_file.go:87-94`）。循环对每个目录事件反应，
   不只看文件名。）
2. `FileSource.reload()` 重新读取并经 `rules.Parse` 解析——格式按扩展名推断（`.yaml`），
   由共享 conf reader registry 解析、展平、要求至少含一个 `govern.*` key，再经 value-tag
   机制绑成 `governance.Config`（`rules/rules.go:57-75`）。解析失败或
   无 govern key 的文档记 `reload ... failed (keeping last good config)` 日志并终止。
3. 配置未变（DeepEqual）不推送——touch 不会搅动 executor。
4. 中心订阅回调采纳配置：`refresh(cfg)` 存原子快照，为每个已注册 label 重新解析 policy，
   只通知 policy 真变了的订阅者（`govern.go:376-397`）；`injector.SetConfig(cfg.Fault)` 原地
   热换 fault。
5. seam 生效：client starter 从不 import governance——它们调 `resilience.ExecutorFor(system, label)`
   与 `fault.InjectorFor()`，都在调用期惰性解析。executor 的 Refresh 换掉在用 policy；fault
   injector 的配置原地热换，所以 `fault.enabled: true` 下一次调用即生效，无需重启。

---

## 3. 逐 key 行为参考

### 3.1 Source key（本 starter）

经 `conf.Bind` 以显式前缀 `${govern.source.file:=}` / `${govern.source.http:=}` 绑定——这是
`govern.*` 下唯一"是接线不是规则"的 key。⚠ 一个命名空间两种角色：`govern.*` 是规则，
`govern.source.*` 是规则从哪来。

| Key | 类型 | 默认值 | 必填 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------|-------------|----------|
| `govern.source.file.path` | string | — | 是（`expr:"$ != ''"`） | fsnotify 监听父目录；格式按扩展名（json/properties/yaml/toml）。初始加载失败直接启动报错。 | 路径错/缺失 → 启动错误（刻意：不许静默 arm 一个 disabled 中心）。 |
| `govern.source.http.url` | string | — | 是（`expr:"$ != ''"`） | GET 轮询；文档与 file source 逐字节兼容。 | 坏 URL / 非 200 / 坏 body → 首次抓取启动报错；之后保旧 + 日志。 |
| `govern.source.http.interval` | duration | 5s | 否 | 轮询周期同时是 HTTP client 超时（`source_http.go:70`）。 | 太小 → 抓取自我超时；太大 → 收敛慢。 |
| `govern.source.http.format` | string | 从 URL 扩展名推断 | 否 | `"yaml" \| "json" \| "properties" \| "toml"`；覆盖推断（URL 无/无意义扩展名时用）。 | 格式错 → 解析错误 → 保旧（或首次抓取启动失败）。 |
| `govern.source.http.headers` | map[string]string | 空 | 否 | 每次请求都带，如控制台的 `headers.authorization=Bearer xxx`。 | 缺鉴权 → 每次抓取 401 → 卡在上一份好配置。 |

每进程恰有一个活跃 source：同时配 file 和 http 会注册两个 bean 但中心只持一个——bean 注入
竞争结果未定义；只配一个。各模块里的远端适配器：`govern.source.nacos.*`
（starter-governance-nacos，ListenConfig 推送）与 `govern.source.etcd.*`（starter-governance-etcd：
`endpoint`、`key`……，Watch 推送）——文档在所有这些后端间逐字节可移植。

没有默认路径：没有 `govern.source.*` key（也没有注入/`SetSource` 的 source）时中心保持 disabled。
⚠ `govern.source.*` 只是引导——规则本身绝不写进 app.properties。

### 3.2 规则文档 —— 顶层

| Key | 类型 | 默认 | 行为 | 配错后果 |
|-----|------|------|------|----------|
| `govern.enabled` | bool | false | 总开关。false → 无论 Default/Rules 为何，PolicyFor 恒返回零 Policy（透传）。 | 全配了却没生效——"为什么不工作"的头号原因。 |
| `govern.driver` | string | "default" | 所有资源共用的 resilience 后端："default" 或 "sentinel"。 | 未知 driver → executor 构建失败 → 回落 no-op executor。 |
| `govern.default.*` | PolicyConfig | 全 off | 没有规则匹配的资源的基础 policy。 | — |
| `govern.rules[n].*` | []Rule | 空 | Resources 含该 label 的第一条 Rule 胜出。⚠ 命中的 Rule **整体替换** Default——不做字段级合并：零值 policy 字段意为"禁用"，部分合并无法区分"显式 0"与"未设"（`govern.go:87-90`）。具体规则放前面。 | 只设 `attempt-timeout` 的规则会静默关掉该资源的默认 retries。 |
| `govern.fault.*` | fault.Config | 全 off | 全进程故障注入，见 §3.4。 | — |

资源 label 在 **value** 里、绝不在 key 里（`govern.go:96-100`）：`redis:cache`、
`gorm:mysql:primary`、`gin:api`、`dubbo:com.example.Foo:1.0.0`——冒号和点都不用转义。

### 3.3 policy 旋钮（16 个，可用于 `govern.default.*` 与每个 `govern.rules[n].*`，默认全 0/off）

| Key | 类型 | 分组 | 说明 |
|-----|------|------|------|
| `rate-limit` / `burst` | float / int | ratelimit | ops/sec 上限；burst 未设时默认取 rate-limit 的小倍数。 |
| `error-threshold` / `open-duration` | int / duration | breaker | 连续错误跳闸。 |
| `breaker-strategy` | 枚举 | breaker | `consecutive` \| `error-rate`。 |
| `error-rate-threshold` / `min-requests` / `breaker-window` | float / int / duration | breaker | error-rate 策略输入。 |
| `max-concurrent` | int | isolate | 并发调用上限。 |
| `max-retries` / `initial-interval` / `multiplier` / `max-interval` / `randomization-factor` | … | retry | 指数退避家族。 |
| `attempt-timeout` / `max-duration` | duration / duration | timeout | 单次尝试上限；总上限（尝试预算 = min(Timeout, 剩余 MaxDuration)）。 |

driver 选择与开关是进程级的（`govern.enabled`/`govern.driver`），刻意**不可**按资源重绑。
每个旋钮的语义（退避数学、breaker 状态机）属于 `cloud/governance/resilience`——starter 只
负责绑定与分发。

### 3.4 fault 旋钮（`govern.fault.*`）

| Key | 类型 | 默认 | 行为 |
|-----|------|------|------|
| `enabled` | bool | false | false 时即使配了 rate/error 也不注入。 |
| `rate` | float | 0 | 单次调用注入所配错误的概率（0..1）。 |
| `latency` | duration | 0 | 每次调用前注入（先于错误决策），无视 rate 对**每个**调用生效——建模整体偏慢的下游而不强制报错。 |
| `error` | string | "" | `""`/`generic`（可重试 ErrInjected）\| `timeout`（包 DeadlineExceeded）\| `reset`（包 ECONNRESET）。空 + rate 0 不注入。 |
| `scope` | string | "" | `""` 全量 \| `real` 只打未标记 \| `loadtest` 只打带压测标记的流量（经 traffic 中间件的 X-LoadTest）。未知值按 "" 处理。 |
| `max-duration` | duration | 0 | 安全自动熄火：自首个受影响调用起经过该时长后停止注入——放火忘了关会自愈。⚠ 热更：调短立即生效；调长**不会**重启已过期的火（把 enabled 关再开来重新点火）。 |
| `max-affected` | int64 | 0 | 爆炸半径上限：受影响调用达到该数即停。 |
| `rules[n].resources` / `.rate` / `.latency` / `.error` | … | 空 | 按资源覆盖；第一条匹配规则胜出。⚠ 与 resilience 不对称：resources 为空的 fault 规则是 catch-all；无规则匹配时顶层值仍然生效——加规则只增加特异度。 |

---

## 4. 验证与故障演练

### 4.1 观察热更（example 应用）

```bash
cd example && go run . -manual
# 基线:  policy: enabled=true timeout=100ms retries=2 rate-limit=0 | fault: enabled=false rate=0.5
sed -i '' 's/attempt-timeout: 100ms/attempt-timeout: 77ms/' conf/govern.yaml   # ~1s 内生效
```

负向演练 —— 坏编辑保留上一份好配置（看日志 tag `governance`）：

```bash
echo "govern: {enabled: true}" > conf/govern.yaml        # 截断：仍有 govern key
printf '' > conf/govern.yaml                              # 空：无 govern.* key → reload 错误
# log: governance file source: reload conf/govern.yaml failed (keeping last good config): ...
```

关治理的正确姿势是 `govern.enabled=false`——一个**存在**的 key——绝不是清空文件。

### 4.2 放火演练，不重启

1. govern.yaml 里以 `fault.enabled: false` 启动。
2. 改成 true 并保存——`injector.SetConfig` 原地热换；下一次调用即受影响。
3. `scope: loadtest` 时只有带 `X-LoadTest: 1` 标记的流量被烧；`real` 反过来（只用于专用
   环境）；空 scope 全量。
4. 离开前加保险：`max-duration: 5m`（自动熄火）与 `max-affected: 1000`（爆炸半径上限）。
5. 把 `enabled` 改回 false 熄火（或等 max-duration 到期）。

### 4.3 在自己的代码里探查门面

```go
if governance.Enabled() { ... }                    // 是否 arm？（live 前为 false）
p := governance.PolicyFor("redis:cache")           // 零 Policy = 透传
governance.Register("redis:cache", func(p resilience.Policy) { /* 重 arm 客户端 */ })
governance.OnReady(func() { /* authority live 时恰好执行一次 */ })
in := fault.InjectorFor(); c := in.Config()        // 读实时 fault 配置（缺失时 nil）
```

### 4.4 测试 API（非 gs 路径）

`governance.Arm(cfg)` 把用 cfg 构建的 center 装上单例、标记 live 并返回 reset 函数；
`governance.Reset()` 恢复 disabled 默认。⚠ `gs.RunTest` **不会**走到这条路径：gs 为测试隔离
克隆全局 bean 定义，测试应用装配的是 wiring bean 的*副本*，而门面仍读包级单例——两者永不
相遇。本 starter 自己的测试改为直接驱动 `wiring.Init()`（`wiring_test.go:53-56`）。应用测试
里用 `Arm`/`Reset`，或直接调门面；别指望 RunTest 注册的 bean 改变 `governance.PolicyFor`
看到的东西。

### 4.5 推送式自定义 source

```go
src := governance.NewPushSource(governance.Config{})
governance.SetSource(src)   // 任意时刻：wiring 前（抢占默认）或之后（late-arm）
// 每个上游事件：
src.Push(cfg)
```

`SetSource` 胜过注入 bean 与默认；同一时刻只有一个活跃 source；中心从不合并——整体替换；
不支持移除自定义 source（重启代替）。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 全配了却没治理 | `govern.enabled` 为 false（默认） | 设 `govern.enabled=true`——它是总开关。 |
| 改了规则文件 policy 不动 | 路径错；或目录监听漏掉的罕见 symlink 场景 | 核对启动时 arm 的路径；看日志 tag `governance` 有没有 `reload ... failed`。 |
| 日志出现 `reload ... failed (keeping last good config)` | 文档被截断/清空（无 `govern.*` key）或语法错 | 修文档；"关"是 `govern.enabled=false`，不是空文件。 |
| 启动报错 `governance source: ... contains no govern.* keys` | 首次加载到空文档 | 同上，发生在构造期——刻意设计。 |
| 自定义 Source bean 被静默忽略 | 少了 `Export(gs.As[governance.Source]())` | 补上 Export——没有它 bean 对接口注入不可见。 |
| HTTP source 卡在旧规则 | 控制台鉴权/可用性：每次轮询失败，保旧 | 查 `headers.*`；确认控制台返回 200 且文档合法。 |
| 按资源的规则把默认 retries 干掉了 | 命中规则整体**替换** default（无合并） | 在规则里重述所有想保留的旋钮。 |
| fault `max-duration` 调大火没复燃 | 调长不会重启已过期的火 | 把 `fault.enabled` 关→开重新点火。 |
| RunTest 断言治理、看到 disabled 中心 | RunTest 克隆 bean；门面读包级单例 | 测试里改用 `governance.Arm`/`Reset`。 |
| 某个 Rooter bean 在治理 arm 前启动 | Rooter 之间顺序未定义 | 把依赖治理的工作包进 `governance.OnReady`。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key（source） | 5（本 starter）+ 各自模块里的 nacos/etcd |
| 必填 | 1（所选 source 的 `path`/`url`） |
| 规则文档 key | 顶层 4 + 16 个 policy 旋钮 ×2（default/rules[n]）+ fault 7 + fault 规则 4 |
| quickstart 前置外部依赖 | 0（file source）；http 需一个控制台 |
| "注意/坑"条数（上文 ⚠） | 7 |

设计嫌疑清单（审计台账；沿用上一版，无新修复项）：

1. `govern.*` 既是规则命名空间，source 配置又放在 `govern.source.*`——一个命名空间两种角色。
2. resilience 规则（整体替换、空 resources 不匹配）与 fault 规则（catch-all + 全局兜底）的
   替换/合并不对称——必须死记。
3. 少了 `Export(gs.As[...])` 的自定义 Source bean 会让治理静默 disabled（启动日志有
   保持 disabled）。
5. example 的自灭冒烟脚本仍未以可跑的 `check.sh` 形式入库。
6. 文档必须定义的概念（Center / Source / Snapshot vs Subscribe / seams / adopt）——对初次
   使用者是实打实的认知负担。
7. ~~Rooter bean 可能在治理 arm 前初始化，且无顺序保证~~ —— 已修复：`governance.OnReady`
   在 `GoLive` 时排队-恰好触发一次（`global.go:145`）。
8. ~~故障注入切换需要重启~~ —— 已修复：每次 source 推送都 `injector.SetConfig` 原地热换
   （`govern.go:209-214`）。
