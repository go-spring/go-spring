# starter-lock-consul 使用说明 — 参考手册

[English](USAGE.md) | [中文](USAGE_CN.md)

详尽使用参考。概览见 [README_CN.md](README_CN.md)。下文所有行为声明均对照 starter 源码
（`starter.go`、`config.go`、`lock.go`、`observe.go`）、共享抽象
[cloud/lock](../../../cloud/lock) 与自校验的 [example/](example)
（`example/check.sh`）核实。Consul 自身语义（session、KV、blocking query）见
[Consul 官方文档](https://developer.hashicorp.com/consul/docs/dynamic-app-config/sessions)——
本文只写 go-spring 的增量。

**激活方式**：任一 `spring.lock.instances.<name>.*` 配置即为每个 `<name>` 注册一个 Consul 后端的
`lock.Locker` 实例（`spring.lock` 前缀为四个锁后端共享——一个二进制只 blank-import 一个
锁后端）。

---

## 1. 完整工程示例

一个双锁服务（定时任务 + 单例 worker），带选主与可观测。文件树：

```
demo/
├── go.mod
├── main.go
├── worker.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/hashicorp/consul/api  latest
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-lock-consul latest
    go-spring.org/starter-actuator    latest   // 可选：探针
    go-spring.org/starter-otel        latest   // 可选：真实 trace/metric 导出
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-lock-consul"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**worker.go** —— 应用的全部锁面：

```go
package main

import (
    "context"
    "time"

    "go-spring.org/cloud/lock"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

type Worker struct {
    // 实例名下的 bean 默认已带 observe 包装（trace span + metric + 访问日志）；
    // observe.enabled=false 可退回裸 locker（见 §6）。
    Jobs      lock.Locker `autowire:"jobs"`
    Singleton lock.Locker `autowire:"singleton"`
}

func init() {
    gs.Provide(&Worker{}).Export(gs.As[gs.Rooter]())
}

func (w *Worker) Init(ctx context.Context) {
    // 同一个 Locker 之上直接选主——任何后端都免费获得。
    e := lock.NewElection(lock.ElectionConfig{
        Locker:    w.Singleton,
        Key:       "singleton-worker",
        OnElected: func(context.Context) { log.Infof(ctx, log.TagAppDef, "elected leader") },
    })
    go func() { _ = e.Run(ctx) }()
}

func (w *Worker) RunOnce(ctx context.Context) {
    lk, ok, err := w.Jobs.TryAcquire(ctx, "nightly-sync")
    if err != nil {
        return // 后端故障 —— 不执行任务
    }
    if !ok {
        return // 别的副本持有 —— 跳过
    }
    defer lk.Unlock(ctx)
    select {
    case <-lk.Lost():
        return // session 中途失效 —— 中止
    case <-time.After(5 * time.Second):
    }
}
```

**conf/app.properties** —— 上述代码用到的完整注释配置面：

```properties
# --- 锁：定时任务 -------------------------------------------------------------
spring.lock.instances.jobs.address=127.0.0.1:8500
spring.lock.instances.jobs.ttl=15s
# session TTL 必须落在 Consul 的 [10s, 86400s] 窗口内；越界值按次钳制而非报错。
spring.lock.instances.jobs.key-prefix=demo/jobs/

# --- 锁：单例 worker -----------------------------------------------------------
spring.lock.instances.singleton.address=127.0.0.1:8500
spring.lock.instances.singleton.key-prefix=demo/singleton/

# --- 可观测（starter-otel） ----------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317

# observe-lock 适配器的访问日志粒度（以下为默认值）。
```

**验证**（本地 Consul agent，可用 `example/docker-compose.yml`）：

```bash
go run .                                        # 启动并完成选主
curl -s localhost:8500/v1/kv/demo/jobs/?keys    # 持锁期间可见锁 key
```

可运行的 [example/](example) 覆盖 TryAcquire/竞争/idempotent-Unlock/重新获取并在成功时
退出 0；`example/check.sh` 用 docker compose 包裹执行。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-lock-consul
  └─ gs.Module(gs.OnProperty("spring.lock"))
        └─ conf.BindEach("${spring.lock}") 逐条目 <name>：
             ├─ address == "" 时 fail fast（启动报错并点名实例）
             ├─ Provide newLocker  → bean "<name>"        （Export lock.Locker，
             │                                                        Destroy → Close）
             └─ 除非 observe.enabled=false，newLocker 默认用 observe-lock 包装
  ├─ 配置绑定：${spring.lock.instances.<name>} → Config（value tag）
  ├─ newConsulLocker：api.NewClient（tls.enabled 时加载 TLS）；记录 TTL 默认值
  ├─ bean 装配：消费方 autowire:"<name>" 解析（bean 已默认带 observe 包装）
  └─ SIGTERM：逐 bean Destroy —— consulLocker.Close 只是契约空操作
               （api.Client 无 Close；已发出的句柄各持自己的 session）
```

每次获取锁都会构造**全新的 `*api.Lock`** 及其专属 Consul session（`lock.go`
buildLock），取消一个句柄不影响其他句柄。

### 2.2 三层时序解析（所有锁后端共享）

TTL / renew / retry 经 `lock.Resolve`（cloud/lock/resolve.go）解析，高层优先：

| 层 | 来源 | 本后端 |
|----|------|--------|
| 1. 每次调用 option | `lock.WithTTL` / `WithRenewInterval` / `WithRetryInterval` | 全部生效 |
| 2. starter 默认 | `spring.lock.instances.<name>.ttl` 等 | **仅 TTL** —— consul 在 `api.Lock` 内部自动续约并在自己的 acquire 循环里阻塞，因此没有 renew/retry key |
| 3. 包默认 | TTL `30s`、renew `TTL/3`、retry `100ms` | 兜底仍未设置的项 |

解析后的 TTL 会在每次获取时被钳进 Consul 的 `[10s, 86400s]` session 窗口
（`lock.go` buildLock）——即使调用方 `WithTTL(5*time.Second)` 也被无声抬到 10s，
而不是被拒绝。

### 2.3 一次锁的逐层走读（acquire → 持有 → release）

`TryAcquire(ctx, "nightly-sync")`：

1. `lock.Resolve(defaults, opts...)` —— 无 `WithTTL` 时用配置 TTL；无 `WithToken` 时生成
   fencing token（随机 16 字节 hex）。
2. `buildLock`：key = `key-prefix + "nightly-sync"`（默认前缀 `lock/`）；session TTL 取
   钳制后的值；`LockTryOnce=true`、`LockWaitTime=500ms`（限定单发往返时延，小而非零，
   给 agent 应答时间）。
3. `al.Lock(stopCh)` —— leaderCh 为 nil ⇒ `ok=false, err=nil`（普通竞争）；错误非 nil ⇒
   后端故障。`Acquire` 则不带 `LockTryOnce`，在 Consul 的 blocking-query 循环里阻塞直到
   拿锁、ctx 结束（经 ctx→stopCh 监听 goroutine 转换）或出错。
4. 持有期：**Consul 服务端自动续约 session**（`api.Lock` 背后），客户端没有续约
   goroutine。session 失效（agent 重启、过期）时 leaderCh 关闭——它就是 `Lost()`。
5. `Unlock`：`api.Lock.Unlock`（删 KV 条目）+ best-effort `Destroy` session。幂等，第二次
   调用返回 nil。`api.ErrLockNotHeld` 被当作良性"已释放"——Consul 无法区分"我们释放的"
   还是"别人释放的"，因此本后端的 Unlock 永不返回 `lock.ErrNotHeld`。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.lock.instances.<name>` 之下（精确匹配，无宽松形态）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `address` | string | — | **必填**。Consul agent 端点，如 `127.0.0.1:8500`，注册前检查。 | 空 → 启动失败并点名实例（`lock-consul: spring.lock.instances.<n>.address is required`）。 |
| `scheme` | string | `http` | 仅 URL scheme。`tls.enabled=true` 会强制 `https`（scheme 显式非 http 时除外）。 | 开 TLS 且 scheme 留 `http` 反而正常（自动 https）；期望明文 + tls 块 → 莫名变成 https。 |
| `token` | string | — | API client 的 Consul ACL token。与每次获取的 fencing token 是两回事。 | token 错误在首次 Acquire 才 403，启动不报（建 client 不鉴权）。 |
| `ttl` | duration | `30s` | session TTL，进 §2.2 第 2 层。每次获取时钳入 `[10s, 86400s]`。 | `5s` 无声变 `10s`；`100000h` 变 `24h`——无任何告警。 |
| `key-prefix` | string | `lock/` | 拼在每个锁 key 前，让共享一个 Consul 集群的多个应用互不冲突。 | 前缀相同的两个应用会互相争锁。 |
| `tls.enabled` | bool | `false` | 应用共享 `tlsconf` 块（`server-name`、`ca-file`、`cert-file`、`key-file`、`insecure-skip-verify`）。 | 开了没材料 → 启动期建 client 报错（fail fast）。 |
| `observe.enabled` | bool | `true` | 默认用 observe-lock 适配器包装 `<name>` 主 Locker bean（trace span + metric + 访问日志）。`false` = 裸 locker。 | 迁移：`<name>-observed` bean 已移除，请注入 `<name>`。 |

⚠ 本 starter **没有** `renew-interval`/`retry-interval` key：Consul 自动续约 session 且
在内部阻塞（§2.2 第 2 层）。实例权重（`Weight=0` 摘流）是注册中心/负载均衡概念，与锁
后端无关。

---

## 4. 验证与故障演练

### 4.1 竞争锁（两个终端）

```bash
# 终端 1 —— manual 模式保持应用存活
go run ./example -manual
# 应用内（或第二个副本）持有 demo/jobs/nightly-sync
# 终端 2 —— 第二个进程抢同一 key
```

他人持锁期间 `TryAcquire` 返回 `ok=false, err=nil`；持锁期间 Consul UI/API 可见 KV 条目
`<key-prefix><key>`：

```bash
curl -s 'localhost:8500/v1/kv/demo/jobs/nightly-sync?raw'   # 即 fencing token
```

### 4.2 持锁期间 TTL 到期（宕机演练）

1. 配短 TTL：`spring.lock.instances.jobs.ttl=10s`（钳制下限）。
2. 拿锁后 `kill -9` 持有进程（不 Unlock）。
3. Consul 在 ~TTL 后判定 session 无效；几秒内 KV 释放，副本中等待的 `Acquire` 获胜。
4. 在被杀进程的孪生副本里（死前）`Lost()` 经 leaderCh 关闭而触发——存活的临界区 select
   到它即中止。

### 4.3 观察 locker（默认开启）

注入 `autowire:"jobs"`，配置 starter-otel 后制造锁流量：

- span 名 `acquire` / `try_acquire`，属性 `lock.system="consul"`、`lock.operation`、
  `lock.key`（detailed 级别）。竞争失败带 `lock.acquired=false`。
- 指标直方图 `lock.operation.duration`，属性同上——流量后到采集端（Jaeger UI 等）查看。

```bash
grep -r 'lock.operation.duration' <otel-export-dump>   # 指标存在性
```

未 import starter-otel 时包装器近乎无感的 no-op（全局 provider 为空操作）。

### 4.4 冒烟测试

```bash
cd example && ./check.sh    # docker 门控：compose 起 consul，跑自校验 example
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报 `spring.lock.instances.<n>.address is required` | 实例缺 address | 补 key 或删实例。 |
| 启动期建 client / TLS 失败 | `tls.*` 材料错误或 scheme 不通 | 修证书；创建是 fail-fast 的。 |
| `Acquire` 返回 403 类 ACL 错误 | `token` 错/缺 | ACL 在首次使用才校验——设置 `spring.lock.instances.<n>.token`。 |
| 锁过期时间与配置不符 | TTL 被钳入 `[10s, 86400s]` | 选窗口内 TTL；本后端做不到 10s 以下。 |
| 被抢后 `Unlock` 也不返回 `ErrNotHeld` | Consul 无法归因释放；`api.ErrLockNotHeld` 被吞为良性 | 需要"被接管证明"语义 → 换 redis 后端（Lua compare-and-DEL）。 |
| `<name>-observed` bean 不存在了 | 2026-08 移除 | 注入 `<name>`——默认已带观测；`observe.enabled=false` 得裸 locker。 |
| 默认包装了却无 span/metric | 未 import starter-otel | 加上；OTel 钩子缺它时静默 no-op。 |
| 两个副本都"持有"锁 | 跨应用同 `key-prefix`/key，或宕机后 TTL 被上钳 | 拆前缀；计算故障转移时长时把钳制算进去。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数（含 tls/observer） | 14 |
| 其中必填 | 1（`address`） |
| quickstart 前置外部依赖 | 1（Consul） |
| "注意/坑"条数 | 4 |

设计嫌疑清单：

- 已解决（2026-08）：主 `<name>` bean 现在默认自带观测（`newLocker` 内透明包装）；独立的
  `<name>-observed` bean 已移除——迁移：注入 `<name>`。
- TTL 钳制是静默的——调用方要 5s 拿到 10s，无日志；一条钳制告警能把配错的故障转移预算
  暴露出来。
- 时序 key 仅 TTL 是必然，但 key 表与 redis 后端观感不一致——可接受，上文按后端分别注明。
