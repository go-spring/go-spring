# starter-scheduler 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`job.go`）与可运行的 [example/](example/)（冒烟：
`example/check.sh`，无需 docker）及 [example-otel/](example-otel/)（docker 门控 Jaeger）
核实。**触发器语义——cron 解析、fixed-rate/fixed-delay、并发策略——来自
[go-spring.org/cloud/scheduling](../../cloud/scheduling)**；
本 starter 把它们接进 gs 生命周期并加上按 job 的跨副本锁。

**激活条件**：`spring.scheduler.enabled` 为 true（**默认开**，`MatchIfMissing`）**且**
至少存在一个 `Job` bean（starter.go:58-62）时装配 `schedulerServer` bean——只 import
不注册 job 的应用零成本。这是全局/基础设施型 starter：不开网络端口；导出 `gs.Server`
让 job 参与服务器生命周期。

---

## 1. 完整工程示例

四个 job——fixed-rate、fixed-delay、cron、带锁 fixed-rate——与
[example/example.go](example/example.go) 同构。文件树：

```
demo/
├── go.mod
├── main.go
└── conf/
    └── app.properties
```

**go.mod**（见 `example/go.mod`）：

```
module demo

require (
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-scheduler latest
    // 可选，跨副本锁（example 用进程内 locker 替代）：
    go-spring.org/starter-lock-redis latest
)
```

**main.go**：

```go
package main

import (
    "context"
    "time"

    "go-spring.org/cloud/lock"
    "go-spring.org/spring/gs"

    scheduler "go-spring.org/starter-scheduler"
)

// 每个 job 都是具体类型 scheduler.Job 的 bean，由工作自己的构造函数调
// scheduling.NewJob 建成——工作通常是自有 struct 的方法——经 gs.Provide 注册
// （容器解析构造函数的依赖）。工作、触发器、选项都是 cloud/scheduling 的
// 类型与本名；只有锁配置（cloud 不知道 bean）用 starter 的链式 setter。
// 无需 Export：调度器直接收集这个类型的 bean。
type tickWork struct{}

func (tickWork) Run(context.Context) error { return nil } // fixed-rate

func NewTickJob() *scheduling.Job {
    return scheduling.NewJob("tick", scheduling.FixedRate(200*time.Millisecond), tickWork{}.Run)
}

type cleanupWork struct{}

func (cleanupWork) Run(context.Context) error {
    return nil // 由下面的锁守护：只有持锁者运行
}

func NewCleanupJob() *scheduling.Job {
    return scheduling.NewJob("cleanup", scheduling.FixedRate(200*time.Millisecond), cleanupWork{}.Run).
        scheduling.WithLock(lk, "cleanup", 5*time.Second)
}

func main() {
    gs.Provide(NewTickJob)
    gs.Provide(NewCleanupJob)

    // 跨副本去重：按 bean 名引用 lock.Locker。生产来自
    // starter-lock-{redis,etcd,consul}；此处用进程内实现顶替。
    ml := lock.NewMemoryLocker()
    gs.Provide(ml).Name("memory").Export(gs.As[lock.Locker]()).
        Destroy(func(l lock.Locker) { _ = ml.Close() })

    gs.Run()
}
```

**conf/app.properties**——完整配置面（复制自 `example/conf/app.properties`）。调度不在这
里：每个 job 在注册处声明自己的触发方式。

```properties
# 默认开启；此行只为记录开关。
spring.scheduler.enabled=true

# 优雅退出时排空在途运行的时限。
```

上段代码里：`Every(200ms)` 是 fixed-rate 触发——自每次计划触发点起每 200ms 触发，重叠运行
受 `WithConcurrency` 约束。`After(200ms)` 在上一次运行**结束后** 200ms 触发，永不重叠。
`Cron("* * * * *")` 是标准 5 字段表达式（分 时 日 月 周）；带秒的 6 字段会被 `ParseCron`
拒绝，解析不了则在构造时失败。锁经 `WithLock(lk, key, ttl)` 挂在 Job 上，
`lock.Locker` bean 注入 job 构造函数。
`WithLockKey`（默认 job 名）是在其上获取的 key，`WithLockTTL` 是租约，持有期间自动续期。

**验证**：

```bash
go run .                    # "fires: tick(fixed-rate)=N delay(fixed-delay)=M locked(lock)=K"
                            # 随后 "starter-scheduler smoke test passed"
./example/check.sh          # 脚本化 marker 断言
# 观察触发日志：
grep 'scheduler: job' <log>
```

### 1.1 可观测变体（example-otel）

[example-otel/](example-otel/main.go) 加入 `starter-otel`，经 OTLP/gRPC 把 trace 导出到
Jaeger（`docker-compose.yml`：all-in-one 开 `COLLECTOR_OTLP_ENABLED`，:4317/:16686），配置见
`example-otel/conf/app.properties`：

```properties
spring.observability.enable=true
spring.observability.service-name=scheduler-otel-example
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.trace.sampler-ratio=1.0
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
spring.observability.metrics.path=/metrics
```

```bash
cd example-otel && docker compose up -d && go run .
# 之后：curl -s 'http://127.0.0.1:16686/api/traces?service=scheduler-otel-example&limit=1'
# example 自身会在退出前断言 "OK: traces found in Jaeger ..."
```

范围说明（已对源码核实）：每次 job 运行会在**全局** otel 管路上开一个 span
（`scheduler.job <name>`，带 job 名属性与 `status` 属性 `ok|error|panic`；由
`cloud/scheduling` 内部开启。未装 starter-otel 或任何 SDK provider 时 otel.Tracer 为
no-op，零开销——按协议/组件 starter 约定，starter 从不自建管路）。跳过的触发不发 span（没有真正
运行），改由 metric 与日志承载。

同一套内置插桩对每次触发发三个 metric（cloud/scheduling）：

| Instrument | 类型 | 属性 | 回答 |
|------------|------|------|------|
| `scheduling.runs` | counter | `job`、`status` | 每次触发各以什么结局收场 |
| `scheduling.run.duration` | histogram（秒） | `job`、`status` | 单次运行耗时 |
| `scheduling.lag` | histogram（秒） | `job` | 运行比计划时刻晚了多少才开始 |

`status` 是一个词表覆盖所有结局：`ok`、`error`、`panic`、`skipped_policy`、
`skipped_lock`——后者区分「被并发策略丢弃」与「别的副本正在跑」。job 名可以作 metric
维度，因为它是代码里写死的；lock key 则不然，基数是调用方决定的、无界。

每次触发还会写一行日志——即 per-fire 访问日志，走自己的 tag
`_app_scheduler_access`（`log.RegisterAppTag("scheduler", "access")`），从而能与应用日志
分开选取。生命周期行（starting / started / drain）仍留在默认 app tag。该行携带 metric
刚记下的同一组 `job` 与 `status`，外加跳过时的 `reason`、运行时的 `duration_ms` /
`error`——因此 `scheduling.runs{job,status}` 与 `scheduling.run.duration{job,status}` 能
join 到解释它的那一行。失败与 panic 为 Error 级，跳过与成功运行为 Debug 级。

`lag` 是调度器自身的健康信号：它不是漂移（下一发仍锚在计划时刻），但持续爬升说明运行
开始得越来越晚，指向一个被压垮或卡住的进程——这是框架里别处拿不到的信号。Prometheus
端点会把点号渲染成下划线、并给 counter 加 `_total` 后缀（`scheduling_runs_total`）。
example-otel 在 trace 检查之后，会额外断言这三者都出现在 `:9090/metrics` 上。

---

## 2. 装配与时序

### 2.1 bean 生命周期时间线

```
import starter-scheduler
  └─ init: gs.Provide(&Server{}).Name("schedulerServer")
        .Condition(OnProperty("spring.scheduler.enabled").HavingValue("true").MatchIfMissing())
        .Condition(OnBean[Job]())
        .Export(gs.As[gs.Server]())                                 [starter.go:58-62]
gs.Run()
  ├─ 配置绑定：${spring.scheduler} → Server.Config（无 expr 校验——
  │  见 §2.2 的不对称），另有字段注入：
  │    Jobs    []Job                 `autowire:"?"`  （全部 Job bean）
  │    （无 Lockers map：每个 job 已持有桥接好的 locker）
  ├─ Rooter Init 阶段：你 Provide 的 job 与 locker 已是 bean；
  │    调度器侧此阶段无事可做
  ├─ Runner 阶段：Server.Run（starter.go:87-105）
  │   ├─ build()——校验 job 自己不可能知道的两件事，先于就绪：
  │   │    重复 job bean → 报错
  │   │    lock 引用不存在的 locker bean → 报错
  │   │    （触发方式与选项早在构造时就定了：缺失或重复的 trigger、
  │   │      非正时长、非法 cron 都在 NewJob 处失败，
  │   │      即装配期间——见 §2.2）
  │   ├─ <-sig.TriggerAndWait() → build 成功后才翻就绪
  │   └─ sched.Start(ctx)——"Scheduling begins only after the application is
  │        ready, so jobs never race application startup"（starter.go:85-86）
  └─ SIGTERM 时：Stop 在框架 shutdown ctx 内经 sched.Stop(ctx) 排空
       在途运行；超时则记 "scheduler drain timed out"
```

### 2.2 校验发生在哪一层（源码核实）

校验按"每层能知道什么"切开：

- **构造时**——`NewJob` 返回错误（即装配期间）：空名字、nil 运行函数或 nil
  触发器；时长非正、cron 解析不了则在 cloud/scheduling 内部失败。`Config` 上**没有**任何 `expr` 校验，
  也没有 `JobConfig` 可绑（不绑任何配置）：调度从不经过配置，于是
  `jobs.<name>.*` 条目与 bean 之间那种按名字的耦合、以及它的不对称（配置→bean 报错、
  bean→配置仅告警，bean 侧拼错就是"永不触发的任务"）一并消失。
- **`Server.Run → build()`（starter.go）**——job 自己不可能知道的两件事：名字重复，
  以及 `WithLock` 指向了容器里没有的 locker bean。两者都在就绪前报错，因此配错的 job
  是启动失败，而不是某次触发才暴露。

### 2.3 命名锁的跨副本语义（源码核实）

1. 容器内所有 `lock.Locker` bean 被收进 `Server.Lockers`，按 **bean 名**索引
   （`autowire:"?"` map，starter.go）。可用后端：starter-lock-redis / -etcd /
   -consul（或像 example 那样自给 bean）。
2. job 用 `WithLock(lk, key, ttl)` 选配开启——`lock.Locker` bean 注入 job
   构造函数，给的是 bean 本体经桥接
   bean 还不存在。`build()` 解析，bean 不存在则**快速失败**（starter.go）。
3. 获取的 key 是 `WithLockKey`，**默认 job 名**——"so two jobs sharing a locker do not
   collide"（job.go）。跨副本：同一 locker + 同一 key = 同一把分布式锁。
4. `WithLockTTL` 转成 `lock.WithTTL`（job.go）；lock 包在 job 持有期间**自动续租**，
   运行超过 TTL 也不丢租约。零值——默认——保留 locker 自身的默认 TTL，所以只有在该默认
   对本 job 太紧时才需要显式设一个高于典型运行时长的值。
5. 每次触发调 `TryAcquire`（lockerAdapter，job.go）：只有抢到锁的副本执行该次
   触发；其余跳过（Debug 级记 "skipped"）。

### 2.4 一次触发逐层走读

1. 调度触发器（cron 规格 / fixed-rate 定时 / fixed-delay 定时）触发。
2. 并发策略生效（默认 `skip`；`queue` / `replace`；对 fixed-delay 无效——它结构上不重叠）。
3. 配了锁时：`TryAcquire(key)`——失败即带原因跳过。
4. `timeout > 0` 时：运行 ctx 包上超时。
5. `Job.Run(ctx)` 执行；`cloud/scheduling` 上报本次触发——metric 加一行日志
   （运行/跳过 Debug、失败 Error），运行另有 span。

---

## 3. 逐 key 行为参考

前缀 `spring.scheduler.*`（单一全局调度器——无多实例）。Server 的 `Config` 字段直接绑
`${spring.scheduler}`（starter.go:68-69），因此这些就是绝对 key。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `spring.scheduler.enabled` | bool | true（MatchIfMissing） | 总开关；false 时即便有 job 也移除 bean。 | 误设 false → 所有 job 静默停摆。 |

这就是配置的全部面。job 的其余一切都在注册处作为选项给出，所以下表是 `JobOption` 的
参考，而非配置 key：

| 选项 | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|------|------|--------|------------|----------|
| `Every(d)` | duration | — | fixed-rate：自每次计划触发点起每 `d` 触发；重叠由 `WithConcurrency` 治理。 | `d <= 0` → 注册时 panic。 |
| `After(d)` | duration | — | fixed-delay：上次运行结束后再过 `d` 触发；永不重叠；`WithConcurrency` 被忽略。 | `d <= 0` → 注册时 panic。 |
| `Cron(expr)` | string | — | 5 字段 cron（`ParseCron` 拒绝其它形态；支持 `@宏`）。 | 解析不了 → 注册时 panic。最常见的错是带秒的 6 字段。 |
| — | — | — | 一个 job 必须**恰好声明一个**上面的触发方式。 | 一个都没有、或给了两个 → 注册时 panic。 |
| `WithTimeout(d)` | duration | 0（关） | `>0` 时到点取消运行 ctx。 | 0 → 卡死的 job 不会被切断（持锁时还一直占租约）。 |
| `WithConcurrency(p)` | `scheduling.ConcurrencyPolicy` | `Skip` | `Skip` \| `Queue` \| `Replace`——类型化取值，非法值根本无法表达。 | — |
| `WithLock(l, key, ttl)` | — | — | 挂上 lock.Locker；每次触发 TryAcquire——仅持锁者运行。 | — |
| `WithLockKey(k)` | string | job 名 | 在 locker 上获取的 key——跨副本协同点。⚠ 想让两个 job 互斥须共用 locker 且共用 key。 | job 间意外共用 key → 相互串行。 |
| `WithLockTTL(d)` | duration | locker 自身默认 | 租约时长，持有期间自动续。 | 低于典型运行+续租抖动 → 运行中丢租约，第二个副本启动。 |

---

## 4. 验证与故障演练

### 4.1 job 执行

```bash
go run .                       # fires: tick(...)≥3 delay(...)≥2 locked(...)≥1
./example/check.sh             # 断言 marker "starter-scheduler smoke test passed"
grep 'scheduler: job' <log>    # "job \"tick\" ran in ..."、"job \"locked\" ran in ..."
```

### 4.2 锁去重（跨副本演练）

并发跑两份 example（同机双副本）；进程内 `memory` locker 只在进程内去重，真实演练请换
starter-lock-redis：

```bash
docker run -d -p 6379:6379 redis:7
# 配 spring.lock.instances.redis... 并把该 lock.Locker bean 注入 job 构造函数，然后：
go run . & go run . & wait
# 只有一个进程记录 "job \"cleanup\" ran"；另一个对它无任何日志
```

### 4.3 失败与跳过的可观测

让一个 job 返回 error，另一个慢跑且 `concurrency=skip` + `fixed-rate=100ms`：

```
scheduler: job "bad" failed after 10ms: boom          # Error 级
scheduler: job "slow" skipped (previous run still in flight)   # Debug，含原因
```

panic 的 job 同样上报，级别为 Error，措辞是 `panicked` 而非 `failed`——它的错误包着
`scheduling.ErrJobPanicked`，用 `errors.Is` 就能和「跑完返回了错误」区分开。

同一批触发也计入 metric（见 §1.1）：失败 job 会让 `scheduling.runs{status="error"}` 上涨；
两种跳过也能区分——`skipped_policy` 说明这个 job 已经跟不上趟，`skipped_lock` 则只是多副本
共用一个调度时的常态。

这些行在 `_app_scheduler_access` tag 上，可以与应用自己的日志分开路由
（`logger.scheduler_access.tag=_app_scheduler_access`）；生命周期行仍在默认 app tag。

没有重试：失败的运行记日志后按计划等下一次触发。需要重试请自包（或把工作推进 asynq）。

### 4.4 退出排空

起一个 3s 的 job，运行中 `kill -TERM`：退出在框架 shutdown ctx 内等待；把它收短可看到
`scheduler drain timed out: context deadline exceeded`（Warn）。

### 4.5 校验演练（快速失败）

以下每一条都被 `NewJob` 以错误拒绝——发生在构造函数里，也就是装配期间、就绪之前：

```go
scheduling.NewJob("", tr, run)                               // → error "job name must not be empty"
scheduling.NewJob("j", nil, scheduling.FixedRate(time.Second)) // → "job run function must not be nil"
scheduling.NewJob("j", run, nil)                               // → "has no trigger"
scheduling.FixedRate(0)   // → "requires a positive duration"（cloud/scheduling panic）
```

以下在 build() 里失败，同样先于就绪：

```go
// 同名 job 的第二个构造返回的 Job 再注册 → Schedule 报 "duplicate task name"
// a lock reference cannot dangle: the locker is injected as a bean, so a wrong
// wiring fails the constructor at startup
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报错 "has no trigger" | 调 `NewJob` 时没给触发器 | 补上触发器。 |
| "sets more than one trigger" | 一个 job 给了两个触发选项 | 只留一个。 |
| cron 报 "must have 5 fields" | 用了带秒的 6 字段表达式 | 用 5 字段（`* * * * *`）。 |
| "references lock %q but no lock.Locker bean" | `WithLock` 的值不是 locker bean 名 | 与 starter-lock 的 bean 名完全一致。 |
| "duplicate job bean named %q" | 两个 job 用了同一个名字 | 改掉一个；这个名字就是任务名兼默认 lock key。 |
| 两个副本都在跑"带锁" job | locker 是进程内的（memory），或两套部署 `WithLockKey` 不同 | 用共享后端且同 key；key 默认即 job 名。 |
| 发布时运行被弃 | 编排层击杀时限短于最长运行 | 调大时限（K8s terminationGracePeriod 等）；排空的 job 不会被补跑。 |
| 两个 job 莫名串行 | 同一 locker 且同一 `WithLockKey` | 各给各的 key（默认已如此）。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 2（均为进程级；job 的调度在代码里） |
| 其中必填 | 0 个 key；每 job 恰一个触发器在注册时强制 |
| quickstart 前置外部依赖数 | 0（跨副本去重才需要锁后端） |
| 注意/坑条数 | 3 |

设计嫌疑清单（保留旧版条目，另加新发现）：

- ~~job ↔ 配置按名字跨两个来源耦合、校验不对称（配置→bean 报错、bean→配置仅告警，
  bean 侧拼错得到静默死 job）~~ 已修：调度移到注册处，于是没有第二个来源、也没有会写错的
  名字查表。整个 `jobs.<name>.*` 面消失。
- job 的节奏不再能改配置（不重新编译）就调整。这是有意的——可运维调节的节奏正是任务平台
  (`starter-xxl-job`) 的领域——但这是配置面此前唯一买到的东西。
- ~~observe seam 只有日志~~ 已被取代：可观测性现内置在 `cloud/scheduling`——每次
  运行在全局 otel 管路开 span（`scheduler.job <name>`），每次触发——运行或跳过——都
  喂三个 metric（`scheduling.runs`、`scheduling.run.duration`、`scheduling.lag`）加
  一行日志。未装 starter-otel（或任何 SDK provider）时全部为 no-op。panic 包着
  `ErrJobPanicked`（cloud/scheduling），单列为一个 `panic` status 值，而不是去匹配
  错误字符串。
- ~~包文档注释的 cron 示例是 6 字段~~ 已修：示例改为 5 字段（`*/5 * * * *`），与
  ParseCron 一致（scheduling/cron.go:81-83）。
- 无 Init 期校验：全部 job 校验在 `Run/build()`；报错在 Runner 期浮出（仍先于就绪，
  但晚于其它 starter 用 `expr` 做的绑定期校验）。
- 击杀时限一到直接弃置在途运行，无补跑无交接——对幂等 job 可接受，对其它
  job 是未成文的策略空白。
