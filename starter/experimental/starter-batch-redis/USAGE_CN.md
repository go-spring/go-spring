# starter-batch-redis 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均对照源码
（`starter.go`、`config.go`、`redisrepo.go`）与可运行的冒烟示例
[starter-batch/example](../starter-batch/example)（`example/check.sh`，docker 门控）核对。
批量语义本身（JobExecution / StepExecution、chunk 模型、断点续跑）属于
`go-spring.org/cloud/experimental/batch`——本文只讲 Redis 后端与 go-spring 接线。

**激活方式**：任一 `spring.batch-repository.*` key。空导入本包后，每个条目注册一个
`batch.JobRepository`（starter.go:51-70）；每个条目复用 starter-go-redis 发布的
`*redis.Client` bean。本 starter 不持有自己的连接——Contributor 形态（starter.go:23-28）：
把 Redis 换成 SQL 后端只需换一个空导入。

---

## 1. 完整工程示例

一个 chunk 型对账 job：把 10 000 行数据写入 Redis，checkpoint 持久化，另带一个进度查询
bean。这正是冒烟示例运行的形态；§4 的崩溃续跑演练原样复用它。文件树：

```
demo/
├── go.mod
├── main.go
├── job.go
├── conf/
│   └── app.properties
└── docker-compose.yml     # redis 在 127.0.0.1:6379
```

**go.mod**（关键依赖）：

```
require (
    github.com/redis/go-redis/v9   latest
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-batch    latest
    go-spring.org/starter-batch-redis latest
    go-spring.org/starter-go-redis latest
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-batch"
    _ "go-spring.org/starter-batch-redis"   // 贡献 Redis JobRepository
    _ "go-spring.org/starter-go-redis"      // 贡献 *redis.Client beans
)

func main() { gs.Run() }
```

**job.go** —— 一个 JobDefinition 加一个进度查询 bean：

```go
package main

import (
    "context"

    "github.com/redis/go-redis/v9"
    "go-spring.org/cloud/experimental/batch"
    "go-spring.org/spring/gs"
    StarterBatch "go-spring.org/starter-batch"
)

// reconcileJob 需要活的 client，因此不能写成 batch.Provide(name, steps...)——
// 在 Build 里 autowire client 并构建 ChunkStep（与冒烟示例同形态）。
type reconcileJob struct {
    Client *redis.Client `autowire:"cache"`
}

func (j *reconcileJob) JobName() string { return "reconcile" }

func (j *reconcileJob) Build() (*batch.Job, error) {
    step := &batch.ChunkStep[int, int]{
        Name:      "load",
        Reader:    &seqReader{n: 10000},          // 实现 batch.Checkpointer
        Processor: batch.Passthrough[int](),
        Writer:    batch.WriterFunc[int](j.writeChunk),
        ChunkSize: 100,
    }
    return &batch.Job{Name: "reconcile", Steps: []batch.Step{step}}, nil
}

// Progress 暴露 runner 使用的同一 repository，供 /jobs 端点查询。
type Progress struct {
    Repo batch.JobRepository `autowire:"main"`
}

func init() {
    gs.Provide(&reconcileJob{}).
        Name("reconcile").
        Export(gs.As[StarterBatch.JobDefinition]())
    gs.Provide(&Progress{}) // 在需要状态查询处注入
}
```

（seqReader 与 writeChunk 属应用代码——完整带 checkpoint 的 reader 见
`../starter-batch/example/example.go:117-145`。）

**conf/app.properties** —— 全量配置面：

```properties
# --- redis client（starter-go-redis 命名空间）----------------------------------
spring.go-redis.cache.addr=127.0.0.1:6379

# --- 本 starter：每个 spring.batch-repository.<name> 条目一个 repository ------
spring.batch-repository.main.client=cache
spring.batch-repository.main.key-prefix=demo:batch:
spring.batch-repository.main.ttl=24h

# --- batch runner（starter-batch 命名空间）——按名引用 repository --------------
spring.batch.repository=main
spring.batch.drain-timeout=30s
spring.batch.jobs.reconcile.run-on-startup=true
spring.batch.jobs.reconcile.params.date=2026-08-28
```

**验证**：

```bash
docker compose up -d
go run .
# 出现 "batch runner started (1 job definition(s), 1 run-on-startup)" 之后：
grep -c 'job "reconcile" finished' <log>      # 1 行，status=COMPLETED
redis-cli --scan --pattern 'demo:batch:*'     # job:<sha1>、steps:<id>、seq 三个 key
```

外部依赖：一个 Redis（唯一依赖）。无需注册中心、无需数据库。

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-go-redis + starter-batch + starter-batch-redis
  ├─ go-redis: gs.Group("${spring.go-redis}") → *redis.Client bean "cache"
  ├─ batch-redis: gs.Module(OnProperty("spring.batch-repository"), BindEach)
  │     └─ 每个条目 <name>: Provide(newRedisRepository, ValueArg(c), TagArg(<client>))
  │                        .Name(<name>).Export(batch.JobRepository)   starter.go:64-67
  └─ batch: Launcher + Server bean，OnBean[JobDefinition]() 门控

gs.Run()
  ├─ 配置绑定：${spring.batch-repository.<name>} → Config（client / key-prefix / ttl）
  ├─ fail-fast #1：client=="" → 启动报错 "instance %q missing required property"
  │                （starter.go:57-60——绝不静默回退默认 client）
  ├─ 装配：TagArg(c.Client) 按名注入 *redis.Client（repository 与 redis 实例的
  │         接缝；starter.go:62-64）
  ├─ Launcher.Init：pickRepository 解析 spring.batch.repository（launcher.go:164-185）；
  │         名字不存在 → fail-fast；0 个 repo → 回退内存（有日志）；
  │         >1 个且未命名 → fail-fast "name one explicitly"
  ├─ Server.Run：等就绪信号后，后台 goroutine 触发 run-on-startup job
  │         （starter-batch/starter.go:123-161）
  └─ SIGTERM：Stop 在 spring.batch.drain-timeout 内排空在途 launch
```

时序要点（restart 保证的来源）：repository 的写是 chunk 提交路径里的同步 Redis 往返——
已提交 chunk 在引擎读下一批之前就已落盘，这正是崩溃续跑精确一次的基础（见 §2.2）。

### 2.2 一次 Launch 的逐层走读（含崩溃）

`Launcher.Launch(ctx, "reconcile", params)` —— runner 的 run-on-startup 与 scheduler 触发
走的是同一个调用（`launcher.go:44-46`）：

1. `ObtainExecution(name, params)` —— Redis `GET job:<instanceKey>`，instanceKey 为参数
   按序拼接后的 `SHA1(name \0 k=v \0 ...)`（`redisrepo.go:95-111`，与内存后端算法一致，
   两个后端对"同一实例"判定相同）。
2. 已存 execution 且 `Status != COMPLETED` → 返回 restart=true，引擎续跑
   （`redisrepo.go:138-146`）；COMPLETED 或缺失 → `INCR seq` 铸出单调唯一 ID
   `<sha8>-<n>`，SET 写入新 JobExecution，restart=false（`redisrepo.go:148-166`）。
3. 每个 step：reader 从保存的 checkpoint 重开；每次 chunk 提交落地为一条
   `HSET steps:<jobExecutionID> <stepName> <JSON StepExecution>` —— 按 step 名原子且幂等，
   每次保存刷新 TTL（`redisrepo.go:197-210`）。
4. **此处崩溃**（`kill -9` / os.Exit）：已提交 chunk 与 step checkpoint 都留在 Redis。
5. 同一 (name, params) 再次 `Launch` → 步骤 2 走 restart 分支，reader 越过 checkpoint
   打开，已提交 chunk 不会重放（冒烟示例证明：续跑后 ReadCount == WriteCount == total）。
6. 状态迁移各自经唯一 JSON+SET+EXPIRE 路径重写 `job:<instanceKey>`
   （`redisrepo.go:181-191`），迁移间崩溃也留下一致快照。

### 2.3 Redis 中的 key 布局

设 `key-prefix=P`（`redisrepo.go:41-46`）：

| Key | 类型 | 内容 |
|-----|------|------|
| `Pjob:<instanceKey>` | string | JSON JobExecution（状态、时间、参数） |
| `Psteps:<jobExecutionID>` | hash | 每 stepName 一个字段 → JSON StepExecution（计数 + checkpoint 信封） |
| `Pseq` | counter | INCR 产生 execution ID |

`ttl > 0` 时三者每次写入都追加 `EXPIRE`。一个 execution 的所有 step 共享一个 hash 是有意
为之：Save/Find/List 各只需一次往返（`redisrepo.go:73-79`）。

---

## 3. 逐 key 行为参考

### 3.1 本 starter —— `spring.batch-repository.<name>.*`（3 个 key）

| Key | 类型 | 默认 | 行为 / 联动 | 配错后果 |
|-----|------|------|------------|----------|
| `<name>.client` | string | — | **必填**。`spring.go-redis.<client>` 下 `*redis.Client` bean 的名字；由 `gs.TagArg` 注入（starter.go:64）。⚠ 联动：go-redis 侧必须存在同名实例。 | 空 → 启动失败 "missing required property"（fail-fast，starter.go:57-60）；错名 → 容器 bean 查找报错。 |
| `<name>.key-prefix` | string | "" | 前置到 `job:`/`steps:`/`seq`。多个应用共用一个 Redis 时用不同前缀隔离。⚠ 改前缀会孤儿化旧历史——续跑变成全新开始。 | 前缀冲突 → 跨应用互相续跑、数据损坏。 |
| `<name>.ttl` | duration | 0 | `>0` → 每次写入追加并**刷新** EXPIRE，长 step 靠刷新保活（`redisrepo.go:113-121`）。0 永久保留（重启窗口不设限）。 | 过小 → 记录中途过期：过期后重启从头重放。 |

### 3.2 runner 侧的承重 key（`spring.batch.*`，属 starter-batch）

| Key | 默认 | 行为 | 配错后果 |
|-----|------|------|----------|
| `spring.batch.repository` | "" | 按名指定本 repository。空且恰有一个 repo bean → 隐式选中；空且有多个 → fail-fast（launcher.go:164-185）。 | 拼错 → 启动失败 "no batch.JobRepository bean of that name"。 |
| `spring.batch.drain-timeout` | 30s | SIGTERM 时排空 startup launch 的上限。 | 0 无限等待；过小中途放弃 launch（续跑仍安全）。 |
| `spring.batch.jobs.<job>.run-on-startup` | false | 就绪后启动一次 `<job>`。 | 配置了 job 但无 JobDefinition bean → 启动失败（launcher.go:97-103）。 |
| `spring.batch.jobs.<job>.params.*` | — | 实例身份：改参数是**新建实例**，不是重启旧的（starter-batch/config.go:58-61）。 | 换日期重试 → 旧未完成实例永远未完成（ttl=0 时还泄漏 key）。 |

⚠ 命名空间拆分是设计使然：repository 绑定在 `spring.batch-repository.<name>` 下，因为
runner 拥有 `spring.batch.*` 用于 job/step/chunk 配置（`config.go:27-31`）。
`spring.batch.repository`（单数，runner 侧）引用 `spring.batch-repository.<name>`（复数，
本 starter）。

---

## 4. 验证与故障演练

以下命令假定 §1 的工程，redis-cli 指向同一 Redis。

### 4.1 跑完一轮后的 key 与内容

```bash
redis-cli --scan --pattern 'demo:batch:*'
# demo:batch:job:<40位hex>      → json .status == "COMPLETED"
redis-cli --type demo:batch:steps:<id>     # hash
redis-cli hget demo:batch:steps:<id> load  # json .readCount/.writeCount
redis-cli get demo:batch:seq               # 单调递增
```

### 4.2 崩溃续跑演练（本 starter 的核心保证）

这正是 `starter-batch/example/check.sh:53-86` 自动化的内容：

```bash
PHASE=1 go run .   # 示例的 writer 在约半数提交后 os.Exit(1)；预期非零退出
redis-cli scard demo:done                  # 约一半条目
PHASE=2 go run .   # 新进程，同一 (name, params) → 续跑；打印
                  # "Completed: read=10000 write=10000 SCARD=10000 (no item reprocessed)"
```

或直接 `cd ../starter-batch/example && ./check.sh`（docker 门控；无 docker 时优雅跳过）。

### 4.3 TTL 演练

设 `ttl=5s` 跑一个长 job：job 运行期间 `redis-cli ttl demo:batch:job:<key>` 持续重置，
完成后才开始倒计时直至过期。

### 4.4 实例身份演练

先以 `params.date=2026-08-28` 跑一次，再以 `params.date=2026-08-29` 跑：`INCR seq` 递增
—— 新的 JobExecution，不续跑。前一实例未完成时以同日期重跑 → 命中同一
`job:<instanceKey>`，走 restart 分支。

### 4.5 可观测性

本 starter 在 `_app_def` tag 下以 Debug 级打印 repository 创建（starter.go:61）：
`creating batch redis repository name=… client=… keyPrefix=…`。运行/提交日志（`batch:`，
`_app_batch`）与 redis 健康检查/指标分别来自 starter-batch 与 starter-go-redis——本 starter
不贡献 indicator 也不贡献 metrics。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 "instance %q missing required property %q" | `spring.batch-repository.<name>.client` 为空 | 补上；刻意没有默认 client（starter.go:53-60）。 |
| 启动失败 "no batch.JobRepository bean of that name" | `spring.batch.repository` 拼错，或 batch-repository 条目未注册 | 检查 OnProperty 激活 key 拼写（`spring.batch-repository`，带连字符）。 |
| 启动失败 "N batch.JobRepository beans present but spring.batch.repository is empty" | 导入多个 repository starter 且未命名 | 用 `spring.batch.repository` 指名（launcher.go:183-185）。 |
| 重启后全部重放 | key-prefix 改过、ttl 过期、或参数变了（新实例） | 保持 prefix/参数稳定；重启窗口不设限用 ttl=0。 |
| 重启"续跑"了一个本应重跑的 job | 同 (name, params) 的上一轮没到 COMPLETED | 改参数铸新实例，或删除 `job:<instanceKey>`。 |
| 运行中出现 `decode job/step ...` 报错 | `job:`/`steps:` 的 JSON 由不兼容的旧版本写入 | 不兼容升级前清掉受影响前缀。 |
| steps hash 无限增长 | ttl=0（默认）永久保留每个 execution | 设置 ttl 回收已结束的运行（config.go:45-50）。 |
| chunk 提交慢 | 一次提交 = 3 次串行往返（SET、EXPIRE、HSET），受时延约束 | 与 Redis 就近部署 / 调大 ChunkSize（每条目 fewer commits）；当前无 pipelining。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 每实例 3（另有 5 个 runner 侧承重 key） |
| 其中必填 | 1（`client`） |
| quickstart 前置外部依赖 | 1（Redis） |
| 文档中"注意/坑"条数 | 4 |

设计嫌疑清单（交设计裁决；第 1-2 条为上一轮审计保留项）：

1. 没有自有 example —— 正确性经由 starter-batch 的示例覆盖，其 check.sh 对同一 Redis 跑
   两遍断言续跑。
2. experimental 目录 = 未审核标记，非质量分级。
3. 读操作无 pipelining：一次提交 3 次串行往返（SET、EXPIRE、HSET）——pipeline/Lua 脚本
   提交可降时延但复杂化代码；当前无此需求。
4. `seq` 计数器按前缀、从不重置；ID 泄漏命名空间的总运行次数。
5. TTL 是实例级而非按生命周期：EXPIRE 同等作用于在途与已完成记录，靠写入刷新保护长
   step。
