# observability

[English](README.md) | [中文](README_CN.md)

`observability` 把每请求的属性（如租户、压测标记）挂在 `context.Context`
上，让框架**替你创建**的那些 span 也能带上它们。客户端 starter 也在这里声明
一次调用是什么（`Operation`）、累积它的尝试记录（`Recorder`），供 `resilience`
的发射端读取。

## 为什么需要它

go-spring 绝大多数插桩的 span 都在框架内部创建：注册中心的注册、加锁、
缓存和 DB 访问，各自在内部闭包里启动 span、记录完就结束。这些 span 从不
暴露给调用方，你拿到自己的 ctx 时，`trace.SpanFromContext(ctx)` 返回的是
**父** span——不是正在记录的那个，也没有任何 API 允许你给它
`SetAttributes`。

所以属性不从 span 传入，而从 context 传入：挂到 context 上，之后凡是由
这个 context 启动的 span——无论在框架哪一层——都能读到：

```go
ctx = observability.WithSpanAttributes(ctx, attribute.String("tenant", t))
```

子 span 自动继承；同名属性后设置的胜出。属性生命周期与 context 完全
一致，context 用完即弃，没有清理 API。

## 安装

```
go get go-spring.org/cloud
```

## 用法

在值已知的地方标注，通常是中间件：

```go
func TenantMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := observability.WithSpanAttributes(r.Context(),
            attribute.String("tenant", tenantOf(r)),
        )
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

读方是一个 `SpanProcessor`，由 starter-otel 自动注册到 provider
（见该 starter 的 USAGE §2.5）。自己构造 `TracerProvider` 的用户需要手动
注册 `observability.SpanAttributesProcessor()`——没有这个读方，挂上去的属性
不会被任何 span 采集。

## span 是一个窗口

span 观测的是一个**过程**，不是某个瞬时：它在工作开始前启动、在工作结束后
结算。正是这个生命周期让它区别于这里的其他信号——工作运行期间它一直可写，
而且可以通过工作所携带的 context 寻址。属性可以在三个时刻进入，第四个不行：

| 时刻 | 方式 |
|---|---|
| `Start` 时 | 组件传给 `Tracer.Start` 的静态属性 |
| `Start` 之前（来自框架之外的调用方） | `observability.WithSpanAttributes`，由 processor 在 span 启动时应用 |
| span 运行期间 | `observability.SetSpanAttributes(ctx, ...)`，由持有该 span context 的一方调用 |
| `End` 之后 | 没有——`OnEnd` 拿到的是只读 span |

第三行是包裹插桩的那一层用的。context 是派生且不可变的，所以
`trace.SpanFromContext(ctx)` 只会解析到本次操作的 span，不会是别的：这一层写
进去的内容不可能落到另一次调用上，被插桩的组件也无需知道这一层存在。

这个设计带着两个限制。窗口的宽度等于 context 传播的宽度——从
`context.Background()` 起的 goroutine、或不认 ctx 的客户端，都会把 span 落在
身后。窗口在 `End` 关闭：调用返回之后才知道的信息，再也加不到它对应的 span
上。

metric 没有这样的窗口。它是一次测量而不是一个过程，所以每个标签都只能在记录
的那一刻交上来。

## 指标

指标回答「多少 / 多久 / 现在怎样」。节点需要看趋势、算速率或配告警时就必须有指标。
三种形态覆盖它，形态决定命名：

| 形态 | 何时 | 命名 |
|---|---|---|
| **操作类** | 每请求 / 每消息的跨边界动作 | `<family>.operation.total` + `<family>.operation.duration`，配互斥 `status` 轴（按 status 求和 = 操作数） |
| **事件类** | 低频、语义重大的状态变迁 | 按事件命名的专用计数器（`lock.lost.total`、`loadbalance.endpoint.suspension.total`）——不用 total/duration 模板 |
| **状态类** | 「现在怎样」，需连续读取 | gauge（`messaging.operation.active`、`loadbalance.endpoint.suspended`） |

保持词汇可 join 的约定：

- **无界基数永不进指标。** key、地址、destination——它们只进 span 或日志，绝不进标签。metric
  属性**默认封闭**；放开一个须配基数护栏（参照 Micrometer `maximumAllowableTags`）。纯低频配置变更
  （治理策略应用）一条日志足够——不强求指标。
- **名取能力，不取实现。** 同类型组件共用同名同型指标，实现身份是维度值（`db.system`、
  `messaging.system`）。要更多实现信息就加字段，不改既有名字。
- **属性轴统一 `status`**（`ok` / `error` / …），不用 `outcome` / `result`。唯一例外是 config-bus 的
  `outcome`（消息处置轴 malformed/ignored/refreshed，语义不同）。（2026-09-22）
- **耗时名词根一律 `duration`**（`<family>.operation.duration`），单位 `WithUnit("s")`；日志耗时字段用
  `duration_ms`（同词根 + `_ms`）。`scheduling.lag` 是队列积压不是操作耗时，不算违反。发现
  `cost` / `latency` / `elapsed` / `outcome` 等偏离视为待整改信号。（2026-09-22）
- **duration 直方图桶界只有一份定义**：未导出的 `durationBuckets` + 导出的 `DurationBuckets()`（返回
  clone），都在 `cloud/observability/buckets.go`。调用点写
  `metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...)`。某域要特化就**本地定义**
  （如 `cloud/scheduling` 的 `lagBuckets`），别改回导出 var（元素可被就地改写）。（2026-09-30）
- **instrument 默认在 wiring/构造期创建，不可包级 `init`**——OTel global meter 只委托第一个 provider，
  init 期创建的 instrument 会绑死 noop。cloud 层新写法的范本是「包级 `sync.OnceValue` + 首次使用时解析」。
- **多实例插桩用 `Meter.RegisterCallback` + `Unregister`**，不用创建期回调（`WithInt64Callback`）——
  第二个实例的回调会被静默丢弃。换 per-instance 的代价是生命周期：实例必须有 `Close` / `Destroy` 钩子，
  否则回调泄漏、同键重复序列（Prometheus 直接拒绝）。nil observer 语义按仓内约定穿透而非 panic，
  `Close()` 对 nil 也 no-op。（2026-09-30）

一个已拍板的例外：`RefreshConf` 的 instrument 在 `Run` 开头每次现建——它必须晚于 starter-otel 装全局
provider，且 `Run` 低频。一般判据：bean 只承载需要容器裁决的东西，`otel.Meter()` 这类只读进程级标准缝
不算「全局逃逸」。

## 两个典型场景

两种都是上面表格里"`Start` 之前"那一行的情况：都**不需要拿到 span 对象**，
只需要 span 将要从中启动的那个 context。

**span 在框架内部启动。** 注册中心的注册、加锁、缓存或 DB 访问，你在自己的
ctx 上看不到这些 span，`trace.SpanFromContext(ctx)` 返回的是父 span。标注你
传进去的 ctx 即可：

```go
ctx = observability.WithSpanAttributes(ctx, attribute.String("tenant", t))
client.Call(ctx, ...)   // 内部启动的 span 带上 tenant
```

**拦截器位于插桩之外。** 组件暴露拦截器链时，外层拦截器在"启动 span 的
那一层"外面执行，它手里的 ctx 上还没有 span。在委派给下一层之前标注：

```go
func (myInterceptor) Do(ctx context.Context, ...) {
    ctx = observability.WithSpanAttributes(ctx, attribute.String("tenant", t))
    return next(ctx, ...)   // 下一层用这个 ctx 启动 span
}
```

## RefreshConf：属性刷新漏斗

每个配置后端（nacos、etcd、consul、vault、k8s、file……）在 watch 到变更后
触发的都是同一个应用级属性刷新。`RefreshConf` 就是这些触发点的共享漏斗：
执行一次刷新并记录结果——按互斥 `status` 统计的 `config.refresh.total`、
`config.refresh.duration`、`config.refresh.last_success_timestamp`，以及
这个全局事件应有的那一条日志。调用方只记自己的后端事件。

```go
_ = observability.RefreshConf(ctx, func(ctx context.Context) error {
	return gs.RefreshProperties(ctx)
})
```

`ctx` 是触发点自己的 context，也是身份的唯一载体：路径起点给它的字段（后端坐标、
路径的 `trace_id`）就是这一轮的记录与日志会显示的东西。漏斗自己不加任何身份。
刷新函数由调用方传入（而非包内引用），因此本包不依赖 spring；fn 的错误原样返回——
这里是插桩，不是错误策略。

## Operation 与 Recorder：客户端 starter 的"声明"接口

客户端不发射信号——它们**声明**，发射端只有一处：`resilience` 的 wrapped
client executor。context 上承载这份声明及其结果的是两个类型。

`Operation` 说明这次调用是什么。它必须挂在**调用方**的 context 上、位于
executor 之外：发射端在 `Execute` 入口读取，写在被包裹函数里的声明没人读。

```go
ctx = observability.WithOperation(ctx, observability.Operation{
    Name:   "get",                   // span 名
    Metric: "db.client",             // 指标名前缀——为空会 panic
    Attrs:  []attribute.KeyValue{…}, // 有界 → 指标标签 + span + 日志
    Detail: []attribute.KeyValue{…}, // 可能无界 → 仅 span + 日志

    LogTag: accessTag,               // starter 自己的访问日志 tag
})
```

`Attrs` 必须保持有界：缓存 key、SQL 语句、URL 路径、topic 一律放 `Detail`，
它永远不会变成指标标签。

`Recorder` 是尝试级累加器：治理 executor 为每次下游尝试追加一条，发射端在
重试循环之外把它读走。

```go
rec := observability.RecorderFrom(ctx)   // nil 安全——没有 recorder 也正常
rec.AddAttempt(d, status, err)
```

`WithRecorder` 刻意**不幂等**：链上最近的一个胜出，嵌套的两层 executor 各写
各的。在循环入口读一次并持有指针——每次写入都重新查会把手外层调用的记录
交给内层 executor。

## 调用 tracer

- **`otel.Tracer` / `otel.Meter` 在使用点现调**（`otel.Tracer(name).Start(...)`），不要包级
  `var tracer = otel.Tracer(...)` 缓存——init 期捕获的 tracer 在 provider set/restore/set 后不再向新
  provider 转发，span 会全丢。（2026-09-24）
- **由 helper 创建 span 时，helper 要把 span ctx 交回调用方**，否则调用点旁边的日志接不上刚建的 span。
- **观测缝 API 一律回调式。**
  `Observer.RegisterAttempt(ctx, service, reason string, fn func(ctx context.Context) error) error`
  的 `system` 在 `NewObserver(system, ...)` 构造期绑定（不在每次调用上传），fn 收 span ctx、其 error 即
  上报结果。它**不**返回 `(ctx, finisher)`——旧 finisher 式纵容 `_, attempt :=` 丢弃 ctx。发现
  `(context.Context, func(error))` 返回形状即反模式信号。被观测操作要真正收 ctx；客户端库不支持 ctx 时
  用 `func(context.Context) error` 显式占位。（2026-09-17）

## 插桩落位与组件契约

- **观测能力在 cloud 实现，starter 只管「是否启用」**，与能力实现无关。starter 只回答「开没开」，不关心
  信号怎么产生。
- **新组件按「本地插桩惯用法」接观测**，范本 `cloud/lock/observe.go`、`starter/starter-redigo/observe.go`：
  包级静态 tag `log.RegisterAppTag("<system>", "access")`；访问日志分级用 log 原生语义（错误 `Warn` /
  带参成功 `Debug`（`log.Debug` 是惰性 `func() []Field` 签名）/ 普通 `Info`，静默 = 对 tag 配 logger
  级别）；span 属性/metric 名沿用 OTel `db.*` / `messaging.*` 惯例。（2026-09-05）
- **同类型组件的观测词汇必须齐整**（同名、同型、同取值词表），组件特有字段允许不同（完整性 / 共同性 /
  灵活性三原则）；缺信号、缺 `status`（分不出成败）是缺陷不是风格差异。判据：换个后端，运维资产还看得懂吗。
  （2026-09-18）
- **后端自带 OTel 插桩是合法信号来源**（elasticsearch 的 `elastictransport`、go-redis 的 `redisotel`、
  kafka 的 `kotel`），别再给它们加插桩，别用「必须自己调 `otel.Tracer(`」判违规。
- **`observe` 套件已拆除**（`cloud/observe` 整包删除），各域自建插桩；`DurationBuckets` 由
  `cloud/observability` 统一，其余逻辑各自本地实现。
- **进程级仪器状态不必全局。** 多实例改用 per-instance Observer（块构造期建、块析构期 `Close()` 注销回调，
  状态是实例字段），范本 `cloud/discovery.Observer`。（2026-09-30）

## 可观测扩展点

- **基于 go-spring 的 `log`、基于 otel 的 span 和 metric，不考虑自建 api。** 统一不发生在新建的层，而
  发生在既有两层——span 的统一协议是 ctx（`trace.SpanFromContext`），日志的统一钩子是
  `log.FieldsFromContext`；metric 内建 label 保持封闭，业务自定义 label 用 `otel.Meter(...)` 自建
  instrument。（2026-09-17）
- **每个自建观测点要既齐整又留自定义位**，由文档规约承诺 + 脚本机械校验
  （`scripts/check-observability.sh`，族规驱动）。checker 判据 = 族规列一组必须出现的共同内容，清单之外
  一律不管。
- **验收判据 V1–V5**：V1 词汇合规 / V2 同类可 join / **V3 每个插桩点都有规约定义的扩展入口且被校验** /
  V4 只用 log+otel 既有 API / V5 漂移能被 CI 挡住。V3 是问题落点，V1/V2 是前提，V5 防复发。
- **进程级维度入口 = 环境变量**，不是 go-spring 新增 API：`starter-otel` 的 `NewResource` 合并
  `resource.Default()`，于是 `OTEL_RESOURCE_ATTRIBUTES` 声明进程级维度、`OTEL_SERVICE_NAME` 覆盖服务名。
  **陷阱（勿改回）**：必须**先** `resource.Default()`、**后** go-spring 自己的属性——OTel 默认 resource
  无条件带 `service.name`（`unknown_service:<binary>` 回退），顺序反了会冲掉配置的服务名；另需显式读
  `resource.Environment()` 的 service.name，`OTEL_SERVICE_NAME` 才赢过配置。（2026-09-17）
- **用户自定义日志字段名原样用作 log key 与 span attribute key**，不改写。
- **`attribute.Value.AsString()` 对非 STRING 类型返回空串**（bool 会渲染成 `load_test=` 使断言静默失效），
  断言用 `Value.Emit()`。

## 不覆盖什么

| 不覆盖 | 应去哪里 |
|---|---|
| **context 属性进 metric**——metric SDK 没有记录时的钩子，内置 metric 标签按设计保持封闭 | 给你自己的 instrument 在记录处传属性 |
| **进程级维度**（env、cluster、version）——进程内每个 span 都一样 | OTel resource：`spring.observability.service-name`、`OTEL_RESOURCE_ATTRIBUTES` |
| **日志字段** | log 模块的 `log.WithFields` / `log.Collector` |
