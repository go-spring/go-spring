# starter-lock-redis 使用说明 — 参考手册

[English](USAGE.md) | [中文](USAGE_CN.md)

详尽使用参考。概览见 [README_CN.md](README_CN.md)。下文所有行为声明均对照 starter 源码
（`starter.go`、`config.go`、`redislock.go`、`observe.go`）、共享抽象
[cloud/lock](../../../cloud/lock) 与自校验的 [example/](example)
（`example/check.sh`）核实。Redis 自身语义（SET NX PX、脚本、过期）见
[Redis 官方文档](https://redis.io/docs/latest/commands/set/)——本文只写 go-spring 的增量。

**激活方式**：任一 `spring.lock.instances.<name>.*` 配置即为每个 `<name>` 注册一个 Redis 后端的
`lock.Locker` 实例；每个实例复用其 `client` 字段指名的 `*redis.Client` bean（由
starter-go-redis 在 `spring.go-redis.instances.<client>` 下提供）。`spring.lock` 前缀为四个锁后端
共享——一个二进制只 blank-import 一个锁后端。

---

## 1. 完整工程示例

一个在应用既有 Redis 上加锁的定时任务服务，带可观测。文件树：

```
demo/
├── go.mod
├── main.go
├── jobs.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/redis/go-redis/v9  latest
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-go-redis latest
    go-spring.org/starter-lock-redis latest
    go-spring.org/starter-actuator latest   // 可选：探针
    go-spring.org/starter-otel     latest   // 可选：真实 trace/metric 导出
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-lock-redis"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**jobs.go** —— 应用的全部锁面：

```go
package main

import (
    "context"
    "time"

    "go-spring.org/cloud/lock"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

type Jobs struct {
    // 实例名下的 bean 默认已带 observe 包装（trace span + metric + 访问日志）；
    // observe.enabled=false 可退回裸 locker（见 §6）。
    Lock lock.Locker `autowire:"jobs"`
}

func init() {
    gs.Provide(&Jobs{}).Export(gs.As[gs.Rooter]())
}

func (j *Jobs) Run(ctx context.Context) {
    // Acquire 竞争时阻塞，每 retry-interval 重试一次。
    lk, err := j.Lock.Acquire(ctx, "nightly-sync", lock.WithTTL(10*time.Second))
    if err != nil {
        return
    }
    defer lk.Unlock(ctx)
    select {
    case <-lk.Lost(): // 续约证明被接管 —— 中止任务
        log.Warnf(ctx, log.TagAppDef, "lock lost mid-run, aborting")
    case <-time.After(3 * time.Second):
        log.Infof(ctx, log.TagAppDef, "job finished")
    }
}
```

**conf/app.properties** —— 上述代码用到的完整注释配置面：

```properties
# --- redis client（starter-go-redis 持有；锁按名复用） -------------------------
spring.go-redis.instances.cache.addr=127.0.0.1:6379

# --- locker --------------------------------------------------------------------
spring.lock.instances.jobs.client=cache          # 必填；为空 fail-fast
spring.lock.instances.jobs.ttl=10s               # 默认 lease TTL（每次调用 WithTTL 优先）
spring.lock.instances.jobs.renew-interval=0      # 0 -> ttl/3；负数关闭自动续约
spring.lock.instances.jobs.retry-interval=100ms  # 竞争时 Acquire 的轮询间隔
spring.lock.instances.jobs.key-prefix=starter-lock-redis:example:

# --- 可观测（starter-otel） ----------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317

# observe-lock 适配器的访问日志粒度（以下为默认值）。
```

**验证**（本地 Redis，可用 `example/docker-compose.yml`）：

```bash
go run .                                            # 启动并跑自校验流程
redis-cli keys 'starter-lock-redis:example:*'       # 持锁期间可见锁 key
redis-cli ttl 'starter-lock-redis:example:demo'     # ~TTL，被续约循环刷新
```

可运行的 [example/](example) 覆盖 TryAcquire/竞争/idempotent-Unlock/重新获取/阻塞 Acquire
并在成功时退出 0；`example/check.sh` 用 docker compose 包裹执行。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-go-redis + starter-lock-redis
  └─ gs.Module(gs.OnProperty("spring.lock"))
        └─ conf.BindEach("${spring.lock}") 逐条目 <name>：
             ├─ client == "" 时 fail fast（启动报错并点名实例）
             ├─ Provide newLocker → bean "<name>"           （Export lock.Locker，
             │             gs.ValueArg(c), gs.TagArg(c.Client)     Destroy → Close）
             └─ 除非 observe.enabled=false，newLocker 默认用 observe-lock 包装
gs.Run()
  ├─ 配置绑定：${spring.lock.instances.<name>} → Config（value tag）
  ├─ newRedisLocker：不拨号——注入现成的 *redis.Client bean；
  │  本 starter 不持有任何连接
  ├─ bean 装配：消费方 autowire:"<name>" 解析（bean 已默认带 observe 包装）
  └─ SIGTERM：Destroy → Close 关闭 stop 通道并等待全部续约 goroutine
                 （wg.Wait）。*redis.Client 的生命周期属于 starter-go-redis，
                 此处不碰。
```

注意 ctor 签名（源码注释）：包装器的内层参数是 **Locker** bean 而非 redis client——按
历史上的复制粘贴 bug（observed bean 误绑 `*redis.Client` 而非 Locker）已随重构消失：第二个
Provide 整体移除，观测改为 `newLocker` 内的透明默认。
### 2.2 三层时序解析（所有锁后端共享）

TTL / renew / retry 经 `lock.Resolve`（cloud/lock/resolve.go）解析，高层优先
——本 starter 喂入**全部三个**旋钮：

| 层 | 来源 | key |
|----|------|-----|
| 1. 每次调用 option | `lock.WithTTL` / `WithRenewInterval` / `WithRetryInterval` | — |
| 2. starter 默认 | `spring.lock.instances.<name>.ttl` / `.renew-interval` / `.retry-interval` | 三个都有 |
| 3. 包默认 | TTL `30s`、renew `TTL/3`、retry `100ms` | 兜底仍未设置的项 |

在分层中保保留下来的特殊语义：

- `renew-interval = 0`（starter 默认）表示"无意见" → 落到 TTL/3。
- **负数** `renew-interval`（每次调用或 starter 级）非零因此被保留：**关闭自动续约**——
  锁在 TTL 后严格过期，与工作时长无关。
- 实例权重（`Weight=0` 摘流）是注册中心/负载均衡概念，与锁后端无关。

### 2.3 一次锁的逐层走读（acquire → 续约 → release）

`Acquire(ctx, "nightly-sync", WithTTL(10s))`：

1. `lock.Resolve(defaults, opts...)` —— TTL 10s（每次调用压过配置），renew 未配置则为
   TTL/3，retry 取 `retry-interval`；无 `WithToken` 时生成 fencing token（随机 16 字节
   hex）。
2. `TryAcquire`：一次 `SET key token NX PX <ttl>`（`redislock.go` TryAcquire）。
   - 未设置（`ok=false`）⇒ 竞争。
   - 设置成功 ⇒ 创建句柄；renew interval > 0 时启动 **renewLoop goroutine**。
   - 在 `Acquire` 中，竞争睡眠 `RetryInterval` 后重试，直到 ctx 结束 / locker 关闭。
3. 持有/续约：每到间隔执行续约 Lua——仅当值仍是调用方 token 时 `PEXPIRE`
   （compare-and-PEXPIRE）。瞬时 Redis 错误跳过、下个 tick 再试（Redis 仍是过期的事实
   源）；返回 `0` 表示 key 没了或已被他人持有 ⇒ 句柄的 `lost` 通道关闭——`Lost()` 触发，
   临界区必须中止。
4. `Unlock`：先停续约循环（不让新的 PEXPIRE 与 DEL 竞争），再执行 unlock Lua——三态
   compare-and-DEL：`1`（已删）、`0`（本就没了——no-op）、`-1`（他人持有）。`-1` 映射为
   `lock.ErrNotHeld`——四个后端里只有本后端能在 Unlock 时*证明*被接管。幂等；后端出错时
   句柄仍触发 `Lost()` 让消费方解除阻塞。
5. 仅单节点 Redlock：无多节点仲裁。要更强保证请换 etcd/consul 后端（blank-import 互换）。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.lock.instances.<name>` 之下（精确匹配，无宽松形态）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `client` | string | — | **必填**。`spring.go-redis.instances.<client>` 下的 `*redis.Client` bean 名，注册前检查。`TagArg(c.Client)` 即锁与 redis 实例的接线 seam。 | 空 → 启动失败并点名实例；拼错 → 启动期装配失败。 |
| `ttl` | duration | `30s` | 未传 `WithTTL` 的获取的默认 lease TTL；§2.2 第 2 层。 | 过低 + GC 停顿/网络抖动 ⇒ 工作中途丢锁；过高 ⇒ 崩溃后故障转移慢。 |
| `renew-interval` | duration | `0` | lease 刷新间隔。`0` → `ttl/3`；**负数关闭自动续约**（TTL 后严格过期）。 | 短 TTL 下关续约会按设计中途掉锁。 |
| `retry-interval` | duration | `100ms` | 竞争时 `Acquire` 的轮询间隔。 | 过低打爆 Redis；过高拉高故障转移时延。 |
| `key-prefix` | string | — | 拼在每个 key 前再进 Redis，让共享一个 Redis 的多应用键空间不冲突。 | 跨应用共享前缀 → 互相争锁。 |
| `observe.enabled` | bool | `true` | 默认用 observe-lock 适配器包装 `<name>` 主 Locker bean（trace span + metric + 访问日志）。`false` = 裸 locker。 | 迁移：`<name>-observed` bean 已移除，请注入 `<name>`。 |

⚠ 本 starter 暴露全部三个时序旋钮，与 consul/etcd（仅 TTL）、k8s（无）不同——Redis 没有
服务端 session 管理，renew/retry 只能放客户端。

---

## 4. 验证与故障演练

### 4.1 竞争锁

两个终端对同一 Redis；终端 1 的应用持有
`starter-lock-redis:example:demo`：

```bash
redis-cli get 'starter-lock-redis:example:demo'   # fencing token（随机 hex）
```

终端 2 的 `TryAcquire` 返回 `ok=false, err=nil`；其 `Acquire` 约在终端 1 Unlock 时返回
（每 `retry-interval` 轮询一次）。

### 4.2 持锁期间 TTL 到期（关续约演练）

1. 配置 `spring.lock.instances.jobs.ttl=3s`、`spring.lock.instances.jobs.renew-interval=-1`（关自动续约）。
2. 获取并持有；观察 key 无任何客户端动作即过期：

```bash
watch -n1 redis-cli ttl 'starter-lock-redis:example:demo'   # 3..2..1 → key 消失
```

3. 过期后另一副本的 `Acquire` 立即获胜。原持有者的 `select { case <-lk.Lost(): }` 只在下
   一次续约尝试时触发（此处已关）——这正是长任务必须开续约的原因。

### 4.3 续约循环（自动续约演练）

默认配置（`renew-interval=0` → ttl/3）下，以 `ttl=10s` 持锁，观察 TTL 每 ~3.3 秒被刷新：

```bash
watch -n1 redis-cli ttl 'starter-lock-redis:example:demo'   # ~10 → ~7 → 10 往复
```

带外改写 key 破坏续约（`redis-cli set <key> oops PX 99999`）：下一次续约返回 `0`，
`Lost()` 触发，之后的 `Unlock` 返回 `ErrNotHeld`（脚本 `-1`）——本后端独有的被接管证明。

### 4.4 观察 locker（默认开启）

注入 `autowire:"jobs"`，配置 starter-otel 后制造锁流量：

- span 名 `acquire` / `try_acquire`，属性 `lock.system="redis"`、`lock.operation`、
  `lock.key`（detailed 级别）；竞争失败带 `lock.acquired=false`。
- 指标直方图 `lock.operation.duration`，属性同上——流量后到采集端查看。

未 import starter-otel 时包装器近乎无感的 no-op。

### 4.5 冒烟测试

```bash
cd example && ./check.sh    # docker 门控：compose 起 redis，跑自校验 example
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报 `lock-redis: instance "<n>" missing required property ...client` | 实例缺 `client` | 把 `spring.lock.instances.<n>.client` 设为既有 `spring.go-redis.instances.<name>`。 |
| 启动期装配 `*redis.Client` 失败 | `client` 拼错——无此 redis bean | 改名对齐 `spring.go-redis.instances.<client>` 条目。 |
| 工作中途丢锁 | 续约被关（`renew-interval < 0`）或 TTL 短于最坏停顿 | 开续约；调大 TTL——续约按间隔触发，TTL 必须扛过一次漏跳。 |
| 崩溃后故障转移慢 | TTL（或 renew 间隔 × 余量）过大 | 调低 TTL；崩溃切换要等剩余 TTL 走完。 |
| `Unlock` 返回 `ErrNotHeld` | key 在 Unlock 前已过期或被接管 | 这是预期中的被接管证明——检查临界区是否超出了 lease。 |
| `<name>-observed` bean 不存在了 | 2026-08 移除 | 注入 `<name>`——默认已带观测；`observe.enabled=false` 得裸 locker。 |
| 默认包装了却无 span/metric | 未 import starter-otel | 加上；OTel 钩子缺它时静默 no-op。 |
| Redis 重启丢光所有锁 | key 在内存里；单节点 Redlock | 属设计；要更强持久性换 etcd/consul 后端（blank-import 互换）。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数（含 observer） | 9 |
| 其中必填 | 1（`client`） |
| quickstart 前置外部依赖 | 1（Redis） |
| "注意/坑"条数 | 4 |

设计嫌疑清单：

- 已解决（2026-08）：主 `<name>` bean 现在默认自带观测（`newLocker` 内透明包装）；独立的
  `<name>-observed` bean 已移除——迁移：注入 `<name>`。
- 此处三个时序旋钮 vs 其他后端零/一——Redis 无服务端 session 管理所致，但值得再评估
  consul/etcd 是否也能暴露 `retry-interval`。
