# starter-scheduler 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`job.go`）与可运行的 [example/](example/)（冒烟：
`example/check.sh`，无需 docker）及 [example-otel/](example-otel/)（docker 门控 Jaeger）
核实。**触发器语义——cron 解析、fixed-rate/fixed-delay、并发策略——来自
[go-spring.org/cloud/scheduling](../../../cloud/scheduling)**；
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

func main() {
    // 每个工作单元一个 Job bean。Provide 以 job 名命名 bean 并导出为 Job，
    // 调度器收集后按名字匹配 ${spring.scheduler.jobs.<name>} 配置条目。
    scheduler.Provide("tick", func(ctx context.Context) error { // fixed-rate
        return nil
    })
    scheduler.Provide("delay", func(ctx context.Context) error { // fixed-delay
        time.Sleep(50 * time.Millisecond) // 结构上保证不重叠
        return nil
    })
    scheduler.Provide("beat", func(ctx context.Context) error { // cron，5 字段
        return nil
    })
    scheduler.Provide("cleanup", func(ctx context.Context) error {
        return nil // 由下面的锁守护：只有持锁者运行
    })

    // 跨副本去重：按 bean 名引用 lock.Locker。生产来自
    // starter-lock-{redis,etcd,consul}；此处用进程内实现顶替。
    ml := lock.NewMemoryLocker()
    gs.Provide(ml).Name("memory").Export(gs.As[lock.Locker]()).
        Destroy(func(l lock.Locker) { _ = ml.Close() })

    gs.Run()
}
```

**conf/app.properties**——完整注释配置面（复制自 `example/conf/app.properties`）：

```properties
# 默认开启；此行只为记录开关。
spring.scheduler.enabled=true

# 优雅退出时排空在途运行的时限。
spring.scheduler.drain-timeout=5s

# fixed-rate：自每次计划触发点起每 200ms 触发；重叠运行受 <job>.concurrency 约束。
spring.scheduler.jobs.tick.fixed-rate=200ms

# fixed-delay：上一次运行结束后 200ms 触发；永不重叠。
spring.scheduler.jobs.delay.fixed-delay=200ms

# cron：标准 5 字段表达式（分 时 日 月 周），此处每分钟。注意：带秒的 6 字段
# 表达式会被 ParseCron 拒绝。
spring.scheduler.jobs.beat.cron=* * * * *

# 带锁 job：`lock` 指向 lock.Locker bean 名；获取的 key 为 `lock-key`
#（默认 job 名）；租约为 `lock-ttl`，持有期间自动续期。
spring.scheduler.jobs.cleanup.fixed-rate=200ms
spring.scheduler.jobs.cleanup.lock=memory
spring.scheduler.jobs.cleanup.lock-ttl=5s
```

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
（`scheduler.job <name>`，带 job 名属性；starter.go 的 `instrument`。未装 starter-otel
或任何 SDK provider 时 otel.Tracer 为 no-op，零开销——按协议/组件 starter 约定，本
starter 从不自建管路）。结局/耗时日志仍来自日志观察者（starter.go 的 `observe`）；
跳过的触发只有日志、无 span（没有真正运行）。example-otel 额外证明应用 starter-otel
管路端到端可用，其 metrics 端口可供你自己的埋点使用——逐 job 的调度 metric 仍是缺口
（见 §6）。

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
  │    Lockers map[string]lock.Locker `autowire:"?"` （全部 locker bean，按名）
  ├─ Rooter Init 阶段：你 Provide 的 job 与 locker 已是 bean；
  │    调度器侧此阶段无事可做
  ├─ Runner 阶段：Server.Run（starter.go:87-105）
  │   ├─ build()——完整校验在此，先于就绪：
  │   │    重复 job bean → 报错
  │   │    有 bean 无配置条目 → 仅 WARN（永不运行）
  │   │    有配置条目无 Job bean → 报错
  │   │    触发器互斥（cron/fixed-rate/fixed-delay 恰选其一，
  │   │      job.go:93-110）/ cron 非法 / concurrency 非法 → 报错
  │   │    lock 引用不存在的 locker bean → 报错
  │   ├─ <-sig.TriggerAndWait() → build 成功后才翻就绪
  │   └─ sched.Start(ctx)——"Scheduling begins only after the application is
  │        ready, so jobs never race application startup"（starter.go:85-86）
  └─ SIGTERM 时：StopContext 用 drain-timeout 包 ctx，经 sched.Stop(ctx) 排空
       在途运行；超时则记 "scheduler drain timed out"
```

### 2.2 不对称的校验（源码核实）

`Config`/`JobConfig` 上**没有**任何 `expr` 校验（config.go 无）。全部校验都在
`Server.Run → build()`（starter.go:134-191），即 Runner 期——但仍先于就绪，因此配错的
job 是启动失败而不是某次触发才暴露。不对称在名字耦合的**两个方向**：

- 配置条目 → 无 Job bean：硬**报错**（"job %q is configured but no Job bean of that name
  is registered"，starter.go:154-157）；
- Job bean → 无配置条目：**仅告警**——"job bean %q has no ${spring.scheduler.jobs.%s}
  entry; it will not run"（starter.go:144-150）。

即配置侧拼错会快速失败；bean 侧拼错（或忘了写配置）只得到一行 Warn 的死 job。

### 2.3 命名锁的跨副本语义（源码核实）

1. 容器内所有 `lock.Locker` bean 被收进 `Server.Lockers`，按 **bean 名**索引
   （`autowire:"?"` map，starter.go:76-77）。可用后端：starter-lock-redis / -etcd /
   -consul（或像 example 那样自给 bean）。
2. job 用 `lock: <bean名>` 选配开启；`build()` 解析，bean 不存在则**快速失败**
   （starter.go:172-177）。
3. 获取的 key 是 `lock-key`，**默认 job 名**——"so two jobs sharing a locker do not
   collide"（config.go:66-68）。跨副本：同一 locker + 同一 key = 同一把分布式锁。
4. `lock-ttl` 转成 `lock.WithTTL`（job.go:156-161）；lock 包在 job 持有期间**自动续租**，
   运行超过 TTL 也不丢租约（config.go:70-73、job.go:139-142）。仍建议 TTL 高于典型
   运行时长，让首个租约窗口从容。
5. 每次触发调 `TryAcquire`（lockerAdapter，job.go:148-154）：只有抢到锁的副本执行该次
   触发；其余跳过（Debug 级记 "skipped"）。

### 2.4 一次触发逐层走读

1. 调度触发器（cron 规格 / fixed-rate 定时 / fixed-delay 定时）触发。
2. 并发策略生效（默认 `skip`；`queue` / `replace`；对 fixed-delay 无效——它结构上不重叠）。
3. 配了锁时：`TryAcquire(key)`——失败即带原因跳过。
4. `timeout > 0` 时：运行 ctx 包上超时。
5. `Job.Run(ctx)` 执行；Observer seam 以 `{Name, Duration, Err, Skipped, Reason}` 回调，
   starter 记日志——运行/跳过 Debug、失败 Error（starter.go:196-206）。

---

## 3. 逐 key 行为参考

前缀 `spring.scheduler.*`（单一全局调度器——无多实例）。Server 的 `Config` 字段直接绑
`${spring.scheduler}`（starter.go:68-69），因此这些就是绝对 key。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `spring.scheduler.enabled` | bool | true（MatchIfMissing） | 总开关；false 时即便有 job 也移除 bean。 | 误设 false → 所有 job 静默停摆。 |
| `spring.scheduler.drain-timeout` | duration | 30s | 退出时排空在途运行的时限（starter.go:120-124）。 | 过短 → 发布时运行中途被弃（无重试——不像队列）。 |
| `jobs.<n>.cron` | string | — | 5 字段 cron（`ParseCron` 拒绝其它形态，scheduling/cron.go:81-83；支持 `@宏`）。⚠ cron/fixed-rate/fixed-delay 恰选其一——0 个或 >1 个是 build 期报错（job.go:93-110）。 | 触发器非法/歧义 → 就绪前启动报错。 |
| `jobs.<n>.fixed-rate` | duration | 0 | 自每次计划触发点起每间隔触发；重叠由 `concurrency` 治理。 | — |
| `jobs.<n>.fixed-delay` | duration | 0 | 上次运行结束后再过该时长触发；永不重叠；`concurrency` 被忽略。 | — |
| `jobs.<n>.timeout` | duration | 0（关） | `>0` 时到点取消运行 ctx。 | 0 → 卡死的 job 不会被切断（持锁时还一直占租约）。 |
| `jobs.<n>.concurrency` | string | skip | `skip` \| `queue` \| `replace`（大小写不敏感，job.go:126-137）。 | 其它值 → 启动报错。 |
| `jobs.<n>.lock` | string | 空 | `lock.Locker` 的 bean 名；每次触发 TryAcquire——仅持锁者运行。⚠ 必须与 locker bean 名完全一致（否则快速失败，starter.go:172-177）。 | 名字不存在 → 启动报错。 |
| `jobs.<n>.lock-key` | string | job 名 | 在 locker 上获取的 key——跨副本协同点。⚠ 想让两个 job 互斥须共用 locker 且共用 key。 | job 间意外共用 key → 相互串行。 |
| `jobs.<n>.lock-ttl` | duration | 30s | 租约时长，持有期间自动续（job.go:139-161）。 | 低于典型运行+续租抖动 → 运行中丢租约，第二个副本启动。 |

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
# 配 spring.lock.redis... 的 "redis" bean + jobs.cleanup.lock=redis，然后：
go run . & go run . & wait
# 只有一个进程记录 "job \"cleanup\" ran"；另一个对它无任何日志
```

### 4.3 失败与跳过的可观测

让一个 job 返回 error，另一个慢跑且 `concurrency=skip` + `fixed-rate=100ms`：

```
scheduler: job "bad" failed after 10ms: boom          # Error 级
scheduler: job "slow" skipped (previous run still in flight)   # Debug，含原因
```

没有重试：失败的运行记日志后按计划等下一次触发。需要重试请自包（或把工作推进 asynq）。

### 4.4 退出排空

起一个 3s 的 job，运行中 `kill -TERM`：退出最多等 `drain-timeout`；调到 1s 可看到
`scheduler drain timed out: context deadline exceeded`（Warn）。

### 4.5 校验演练（快速失败）

```bash
# 1) 配了却未注册：
spring.scheduler.jobs.ghost.fixed-rate=1s            # → 启动报错点名 "ghost"
# 2) 一个 job 两个触发器：
spring.scheduler.jobs.tick.fixed-rate=1s
spring.scheduler.jobs.tick.fixed-delay=1s            # → "sets more than one of ..."
# 3) 未知 locker：
spring.scheduler.jobs.tick.lock=nope                 # → "references lock \"nope\" ..."
# 4) 6 字段 cron：
spring.scheduler.jobs.beat.cron=0 */5 * * * *        # → "must have 5 fields, got 6"
# 5) 有 bean 无条目（静默方向）：
scheduler.Provide("orphan", fn)  # 无 jobs.orphan.* → 一行 Warn，永不运行
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| job 从不触发，启动只有一行 Warn | 注册了 bean 但没有 `jobs.<name>.*` 条目（仅告警方向，starter.go:144-150） | 补配置条目；两侧名字拼写都查。 |
| 启动报 "configured but no Job bean" | 配置条目没有对应的 `scheduler.Provide` | 注册 bean 或删条目。 |
| "sets more than one of cron/fixed-rate/fixed-delay" / "must set exactly one" | 触发器互斥（job.go:93-110） | 每个 job 恰保留一个触发 key。 |
| cron 报 "must have 5 fields" | 用了带秒的 6 字段表达式 | 用 5 字段（`* * * * *`）。 |
| "references lock %q but no lock.Locker bean" | `lock` 的值不是 locker bean 名 | 与 starter-lock 的 bean 名完全一致（starter.go:172-177）。 |
| 两个副本都在跑"带锁" job | locker 是进程内的（memory），或两套部署 `lock-key` 不同 | 用共享后端且同 key；key 默认即 job 名。 |
| 发布时运行被弃 | `drain-timeout` 短于最长运行 | 调大；排空的 job 不会被补跑。 |
| 两个 job 莫名串行 | 同一 locker 且同一 `lock-key` | 各给各的 key（默认已如此）。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 11（全局 2 + 每 job 9） |
| 其中必填 | 0 个 key，但强制每 job 恰一个触发器 |
| quickstart 前置外部依赖数 | 0（跨副本去重才需要锁后端） |
| 注意/坑条数 | 4 |

设计嫌疑清单（保留旧版条目，另加新发现）：

- job ↔ 配置按名字跨两个来源耦合，校验不对称：配置→bean 报错、bean→配置仅告警——
  bean 侧拼错得到的是静默死 job。
- ~~observe seam 只有日志~~ 已修：每次 job 运行在全局 otel 管路开 span
  （`scheduler.job <name>`，经 otel.Tracer，见 starter.go 的 instrument）——未装
  starter-otel（或任何 SDK provider）时为 no-op tracer；日志观察者不变，跳过的触发
  仍只有日志。有 span + 日志，尚无 metric。
- ~~包文档注释的 cron 示例是 6 字段~~ 已修：示例改为 5 字段（`*/5 * * * *`），与
  ParseCron 一致（scheduling/cron.go:81-83）。
- 无 Init 期校验：全部 job 校验在 `Run/build()`；报错在 Runner 期浮出（仍先于就绪，
  但晚于其它 starter 用 `expr` 做的绑定期校验）。
- `drain-timeout` 到期直接弃置在途运行，无补跑无交接——对幂等 job 可接受，对其它
  job 是未成文的策略空白。
