# starter-batch 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`job.go`、`launcher.go`、`config.go`）与自断言的 [example/](example/)
（`example/check.sh` —— docker Redis + 两阶段崩溃/恢复冒烟）。**批处理语义（chunk step、
重启 checkpoint、job 执行模型）见
[cloud/batch](../../../cloud/batch)** —— 下文只写 go-spring 的接线。

**激活方式**：blank import + 至少一个 `JobDefinition` bean。两个 starter bean 都以
`gs.OnBean[JobDefinition]()` 为条件（starter.go 的 `init`），导入 starter 但不注册 job
零成本。`spring.batch.enabled=false` 是显式退出开关。

---

## 1. 完整工程示例

一个把整数 1..10000 拷进 Redis set 的 chunk job：持久化（Redis）repository、崩溃后
重启续跑、手动 Launch。文件树（即 example 本身）：

```
demo/
├── go.mod
├── main.go            // 注册 JobDefinition + 测试 runner
├── conf/app.properties
├── docker-compose.yml // Redis
└── check.sh           // 两阶段崩溃/恢复驱动
```

**go.mod**（关键依赖；兄弟模块经 go.work 解析）：

```
require (
    github.com/redis/go-redis/v9 v9.21.0
    go-spring.org/cloud            v0.0.0
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-batch    latest
    go-spring.org/starter-batch-redis  latest   // 持久化 JobRepository 后端
    go-spring.org/starter-go-redis    latest   // *redis.Client bean
)
```

**main.go**（节选自 `example/example.go`，结构一致）：

```go
package main

import (
    _ "go-spring.org/starter-batch-redis"
    _ "go-spring.org/starter-go-redis"

    "go-spring.org/cloud/experimental/batch"
    "go-spring.org/spring/gs"
    StarterBatch "go-spring.org/starter-batch"
)

// reconcileJob 是 writer 需要活体 *redis.Client 的 JobDefinition，
// 无法用普通的 Provide(name, steps...) —— 它注入 client 并在 Build 里
// 构造 ChunkStep。
type reconcileJob struct {
    Client *redis.Client `autowire:"cache"`
}

func (j *reconcileJob) JobName() string { return "reconcile" }
func (j *reconcileJob) Build() (*batch.Job, error) {
    step := &batch.ChunkStep[int, int]{
        Name:      "load",
        Reader:    &seqReader{n: 10000},          // 实现 batch.Checkpointer
        Processor: batch.Passthrough[int](),
        Writer: batch.WriterFunc[int](func(ctx context.Context, items []int) error {
            return j.Client.SAdd(ctx, "demo:done", toAny(items)...).Err()
        }),
        ChunkSize: 100,
    }
    return &batch.Job{Name: "reconcile", Steps: []batch.Step{step}}, nil
}

// Runner 注入共享 *Launcher —— 与 scheduler.Job 使用同一 seam。
type Runner struct {
    Launcher *StarterBatch.Launcher `autowire:""`
    Client   *redis.Client          `autowire:"cache"`
}

func main() {
    // Export 是关键承载：gs 只装配根可达 bean，JobDefinition 靠 export
    // 被收集。漏掉 Export 则 job 静默不注册（见 §2.1）。
    gs.Provide(&reconcileJob{}).
        Name("reconcile").
        Export(gs.As[StarterBatch.JobDefinition]())
    gs.Provide(&Runner{}).Export(gs.As[gs.Rooter]())
    gs.Run()
}
```

**conf/app.properties** —— 完整注释配置（取自 example）：

```properties
# starter-go-redis 管理的 Redis client。批处理 repository 后端与
# example 自己的结果集都按名复用该 client。
spring.go-redis.instances.cache.addr=127.0.0.1:6379

# 名为 "main" 的 Redis 版 batch.JobRepository，复用上面的 redis client。
# 它使 step checkpoint 在进程崩溃后仍持久。
spring.batch-repository.instances.main.client=cache
spring.batch-repository.instances.main.key-prefix=starter-batch:example:

# 告诉 batch runner 用 "main" repository 作为进度存储。
spring.batch.repository=main
```

**验证**（与 example 的 check.sh 同构）：

```bash
docker compose up -d && wait-6379
PHASE=1 go run . ; echo "rc=$?"     # 崩溃跑：约半数提交后非零退出
PHASE=2 go run .                    # 续跑；打印 "starter-batch smoke test passed"
docker exec starter-batch-redis redis-cli SCARD starter-batch:example:done   # 10000
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-batch
  ├─ init(): gs.Provide(&Launcher{}).Name("batchLauncher")
  │            .Init((*Launcher).Init)
  │            .Condition(enabled, gs.OnBean[JobDefinition]())
  ├─ init(): gs.Provide(&Server{}).Name("batchServer")
  │            .Condition(enabled, gs.OnBean[JobDefinition]())
  │            .Export(gs.As[gs.Server]())
gs.Run()
  ├─ 绑定：Launcher.Config ← ${spring.batch}（value tag）
  ├─ 注入：Launcher.Defs ← []JobDefinition（"?"）、Launcher.Repos ←
  │     map[string]batch.JobRepository（"?"）
  ├─ Launcher.Init：pickRepository（具名 > 唯一 bean > 内存兜底）；
  │     按 JobName 去重（重名 = 启动失败）；对每个定义 Build() 一次
  │     （builder 错误 fail-fast）；校验每个 spring.batch.jobs.<name>
  │     都指向真实定义
  ├─ Server.Run：阻塞在 sig.TriggerAndWait() —— 启动型 launch 在应用
  │     就绪之后才触发
  ├─ 每个 run-on-startup job：goroutine + wg.Add → Launcher.Launch
  │     （与 scheduler 走同一调用）
  └─ SIGTERM 时：Server.Stop —— cancel(runCtx)、按 drain-timeout
        有界等待 wg，然后返回
```

**Export 强制性**（记忆：gs 只装配根可达 bean）：用 `gs.Provide` 注册、既不被任何
对象注入、又没有 export `JobDefinition` 接口的定义不会被收集 —— `Launcher.Defs`
为空，job 静默不可 Launch（`Launch` 报 "unknown job"）。这正是 `Provide` 与 example
都链上 `.Name(...).Export(gs.As[JobDefinition]())` 的原因。见 §6 嫌疑 #1。

### 2.2 一次 job launch 的逐层走读

`Launcher.Launch(ctx, "reconcile", params)`（launcher.go）：

1. 解析 `l.defs["reconcile"]` —— 未知名称是明确报错，不是静默 no-op。
2. `def.Build()` —— 为本次运行重建 Job（Init 里的 Build 只做校验；再次调用 Build
   是为了按运行注入状态，见源码注释）。
3. `job.Run(ctx, l.repo, params)` —— batch 引擎从 repository 加载同一 (name, params)
   实例的上次执行；未完成则从 reader 的 checkpoint 续跑（`seqReader.Open(cp)`），
   已完成/新建则从头开始。
4. 每个 chunk：reader 读满 `ChunkSize` → processor → writer 提交 → checkpoint 持久化。
   计数（`ReadCount`/`WriteCount`）跨重启累加（从 repository 加载）—— 恢复后与
   `total` 精确相等即证明已提交 chunk 没有被重复处理。
5. 返回的 `*batch.JobExecution` 携带 `Status`（断言 `batch.StatusCompleted`）与
   `FailureMsg`。

### 2.3 停机时的 drain 语义

`Server.Stop`（starter.go）：先 cancel launch context（运行中的 chunk step 在
Reader/Processor/Writer 里看到 ctx 取消 —— 尊重 ctx 的 writer 停在两次提交之间，
checkpoint 保持一致），再按 `drain-timeout` 有界等待 WaitGroup。超时 → Warn 日志
"drain timed out ... abandoning in-flight launches"，Stop 照常返回。⚠ 只跟踪**启动型**
launch：`Run` 返回后由你自己的 scheduler/handler 发起的 `Launch` 归调用方所有
（`Stop` 源码注释）。

### 2.4 为什么是 gs.Server 而不是 Runner

按包文档：这是 global/infrastructure 型 starter —— 不开端口。导出 `gs.Server` 换来
server 生命周期的参与：启动型 launch 在就绪信号之后才开始，Stop 加入优雅停机编排
既有的 drain。

---

## 3. 逐 key 行为参考

`spring.batch` 之下：

| key | 类型 | 默认值 | 行为/联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `spring.batch.enabled` | bool | true（MatchIfMissing） | 两个 bean 的退出闸门，与 OnBean[JobDefinition] 组合。 | false → 即使注册了 job 也没有 Launcher/Server bean。 |
| `spring.batch.repository` | string | — | 指名一个 `batch.JobRepository` bean。解析顺序：具名 > 唯一 repo bean > `batch.NewMemoryRepository()`（进程内，不持久）。 | ⚠ 名字无对应 bean = 启动 fail-fast；⚠ 多个 repo bean 且留空 = fail-fast 要求消歧。 |
| `spring.batch.drain-timeout` | duration | 30s | Stop 等待运行中启动型 launch 的上限；`<=0` 无限等。 | 过小 → "drain timed out ... abandoning"、可能中途放弃（checkpoint 仍一致）；过大 → 停机缓慢。 |
| `spring.batch.jobs.<name>.run-on-startup` | bool | false | 应用就绪后 launch 一次（Cloud Task 形态）。 ⚠ `<name>` 必须与某 JobDefinition bean 的 `JobName()` 匹配 —— 否则启动 fail-fast。 | 拼写错误 → 启动报 "no JobDefinition bean of that name"。 |
| `spring.batch.jobs.<name>.params.<k>` | string | — | 启动 launch 参数。(name, params) 在 repository 中标识 job 实例 —— 改参数会新建实例而非恢复旧实例。 | 期望改参数后续跑 → 实际全新跑一遍。 |

已与 `grep -rhoE 'value:"[^"]+"'` 核对 —— 上述 5 个 key 加内部的 `${spring.batch}`
结构绑定，别无其他。（example 中的 `spring.go-redis.instances.*` / `spring.batch-repository.instances.*`
属于各自 starter。）

---

## 4. 验证与故障演练

### 4.1 job 触发并完成（example check.sh，两阶段）

```bash
cd starter/experimental/starter-batch/example
docker compose up -d && ./check.sh
# PHASE 1 中途崩溃（非零退出），PHASE 2 打印：
#   "PHASE 2: found <n> items already committed from the crashed run; resuming"
#   "Completed: read=10000 write=10000 SCARD=10000 (no item reprocessed)"
#   "starter-batch smoke test passed"
```

### 4.2 崩溃/重启演练（持久化 repository）

核心保证：`PHASE=1 go run .` 在约 5000 次提交后退出码 1；`PHASE=2 go run .` 从最后
提交的 checkpoint 恢复**同一** (name, params) 实例。直接验证精确性：

```bash
docker exec starter-batch-redis redis-cli SCARD starter-batch:example:done   # == 10000，不多不少
```

换内存 repository 复跑（去掉 `spring.batch.repository` 与 starter-batch-redis 导入）：
PHASE 2 从零开始 —— 说明持久化必须依赖后端。

### 4.3 drain 演练

设置 `spring.batch.jobs.reconcile.run-on-startup=true` + 长 job +
`spring.batch.drain-timeout=2s`；job 运行中发 SIGTERM：约 2s 内日志出现
`batch: drain timed out after 2s; abandoning in-flight launches`（tag `_app_batch`）。

### 4.4 日志 tag

所有 launch/finish/drain 日志使用注册 tag `_app_batch`：

```properties
logger.batch.type=Logger
logger.batch.level=WARN
logger.batch.tag=_app_batch
```

启动还会打 `batch runner started (N job definition(s), M run-on-startup)` —— 最方便的
接线自检。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报 "no JobDefinition bean of that name is registered" | `spring.batch.jobs.<name>` 拼错，或 job bean 缺失/未 Export | 改名；给 Provide 链上 `.Export(gs.As[JobDefinition]())`。 |
| Launch 运行时报 "unknown job"，但确实注册了 | job bean 未 Export → 未被收集（根可达性） | 同上 —— Export 是关键承载（§2.1）。 |
| "duplicate JobDefinition bean named %q" | 两个定义共用 JobName | JobName 必须在容器内唯一。 |
| "%d batch.JobRepository beans present but spring.batch.repository is empty" | 导入两个 repo 后端且未指名 | 设置 `spring.batch.repository=<bean 名>`。 |
| 重启后全部重跑/从零开始 | 内存 repository 兜底（启动日志 "using in-process NewMemoryRepository"） | 导入持久化后端（starter-batch-redis）并指名。 |
| 启动报 "batch: build job %q" | Build() 返回错误（step 接线不对） | Init 会对每个定义 Build 一次以提前暴露。 |
| 停机挂住 | `drain-timeout<=0`（无限等）且 launch 卡死 | 设置正的 drain-timeout；让 writer 尊重 ctx。 |
| 启动型 launch 不触发 | job 已注册但 `run-on-startup` 未设（false） | 设 `spring.batch.jobs.<name>.run-on-startup=true`，或走 Launcher seam。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 5 |
| 其中必填 | 0 |
| quickstart 前置外部依赖 | 0（持久化才需要 Redis） |
| "注意/坑" 条数 | 6 |

设计嫌疑清单（沿用上版，交设计裁决）：

1. `JobDefinition` 必须由调用方自行 Export —— 容易忘掉
   `Export(gs.As[JobDefinition]())`，job 静默不被收集。
2. 启动型 launch 由 `Stop` drain，但 `Run` 返回后的按需 `Launch` 不被跟踪 ——
   生命周期所有权拆分微妙。
3. （新增）`Launcher.Init` 仅为校验对每个定义 Build 并丢弃结果 —— Build 按设计每
   job 跑两次；Build 里有副作用的定义会在启动时付一次代价。
4. （新增）batch runner 与 scheduler 各自维护 drain WaitGroup —— scheduler 触发的
   批跑由 scheduler drain、启动型由 batch Server drain；总停机时间是二者最大值而非
   之和，但对运维不可见。
