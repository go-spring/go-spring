# observe 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`observe` 给所有 client starter 提供同一份插桩件:一个 `Observer`,对每
次操作发 trace span + 时长/在途 metric + access log;一份
`ObserveConfig`,绑在各 starter 的 `observability.*` 块下;以及面向横切
client 接口的共享桥接(lock、resilience executor、transaction observer)。

## 1. 职责与边界

- **做:** 定义 `Observer` / `Span` / `SemConv` / `ObserveConfig`,内置
  DB / messaging / resilience 三套约定。领域包的插桩放在领域包自身
  (`experimental/lock.WrapLocker`、`experimental/transaction.SagaObserver`、
  `governance/resilience.WrapExecutor` 等)。
- **不做:**
  - 不做 OTel 引导。本件只读 OTel 全局;安装 provider 是 starter-otel
    的事。
  - 不写 spring 核心代码。otel-free 的核心定义 `lock.Locker`、
    `resilience.Executor`、`transaction.Observer`;插桩属于适配侧而非
    核心 —— 正是这条边界决定了桥接放这里。
  - 不做 per-instance 的 trace/metric 配置。trace 和 metric 走同一条
    全局管线;真正 per-instance 有意义的是 access log。

## 2. 关键抽象与缝隙

- **`SemConv` 整体传递。** metric 前缀脱离配套的属性 key 就没有意义,
  所以 `New` 收整个 bundle。具名构造器(`NewDB` / `NewProducer` /
  `NewConsumer`)保证 SemConv 与 span kind 的成对一致;`New` 是自定义
  client 类型的逃生口。
- **`Start`/`End` 是唯一调用形态。** 每操作一个 span、defer 友好;被
  skip 的操作返回 no-op Span,seam 代码无需分支。`Span.SetArg` 覆盖
  gorm 那类计时先开始、参数后到位的场景。
- **OTel 全局即依赖缝隙。** 没有 starter-otel 时 tracer/meter 是 no-op、
  SpanContext 无效 —— 未配置的应用几乎零付出,因此每个 starter 都能无
  条件接线本件。
- **`statusKey` 是本件自己的属性**,刻意不用 OTel semconv:粗粒度
  ok/error 码做 metric 维度,错误细节留在 span/log 上。

## 3. resilience 桥(2026-08 收拢进主 Observer)

`WrapExecutor` 最初是独立的 trace-only 包装;现在基于 `observe.New` +
`ResilienceSemConv` 构建,保护调用因此与其他 client 操作发出同样的三
信号(span kind internal,"操作名"即被保护的 resource 名)。在此之上它
再加:

- `resilience.calls` 计数,带**六值 outcome** 维度:`success`、
  `rate_limited`、`circuit_open`、`bulkhead_full`、`timeout`、`error`
  —— 让 dashboard 能区分"被保护拒绝"与"下游失败",span 上也附
  `resilience.outcome` 属性;
- `resilience.breaker.state_change` 计数 + 日志(跳闸 Warn、恢复/半开
  Info),通过 `BreakerEventListenerSetter` 订阅 —— 不支持该能力的
  driver 被静默跳过,调用级信号照发。

动机:核心 resilience 包刻意不做 metric/trace/log,熔断跳闸在生产里是
黑盒。inner 为 nil 时返回 nil —— 未武装的 client 保持原样。

## 4. lock 桥:WrapLocker(system, cfg, inner)

包装任意 `lock.Locker`,让 `Acquire`/`TryAcquire` 在 `lock` 约定
(`lock.system` / `lock.operation` / `lock.key`)下发全三信号。两个形状
决策:

- 走 `observe.New`(不再是早期那种专门的 trace 包装),四个 lock
  starter 共享同一份 ~70 行实现,只差 system 标签;
- `TryAcquire` 未命中不是错误 —— `err` 保持 nil,miss 以
  `lock.acquired=false` 记录,metric 维度上区分输赢。

## 5. 权衡与放弃的方案

- **otel-only,不做 provider 中立的信号 API。** 支持非 OTel 后端会让
  每个发射点为一个猜想需求分叉;OTel 是事实标准,其全局提供了零配置
  的 no-op 通路。代价 —— spring 核心不能承载本件 —— 被接受,并以分层
  化解(抽象在核心,桥接在此)。
- **独立的 observe-lock / observe-resilience / observe-transaction
  模块已收拢为 `go-spring.org/cloud/observe` 的子包**:模块边界被
  证明只有开销、没有消费者差异;lock/transaction 桥接指向
  cloud 领域包(`cloud/lock`、`cloud/transaction`、
  `cloud/governance/resilience`)接口,不把 starter 拖进依赖图。gorm 桥是
  刻意的例外:它住在 `go-spring.org/starter-gorm` 的 `observe` 包,
  因为本模块不得依赖 gorm。
- **status 只有两态,metric 上不做错误分类。** 按 client 各自分类错误
  会随后端漂移;唯一值得付出分类成本的是 resilience 桥,所以六值
  taxonomy 放在那里而非通用 Observer。
- **access log 恒发(除非 `off`)。** log 信号不依赖 starter-otel,
  未插桩的应用也有 per-op 可见性;`SkipOps` 是流量逃生阀。
