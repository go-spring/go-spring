# starter-lock-etcd 使用说明 — 参考手册

[English](USAGE.md) | [中文](USAGE_CN.md)

详尽使用参考。概览见 [README_CN.md](README_CN.md)。下文所有行为声明均对照 starter 源码
（`starter.go`、`config.go`、`etcdlock.go`、`observe.go`）、共享抽象
[cloud/lock](../../../cloud/lock) 与自校验的 [example/](example)
（`example/check.sh`）核实。etcd 自身语义（lease、concurrency 包）见
[etcd 官方文档](https://etcd.io/docs/latest/dev-guide/api_concurrency_reference/)——本文只写
go-spring 的增量。

**激活方式**：任一 `spring.lock.<name>.*` 配置即为每个 `<name>` 注册一个 etcd 后端的
`lock.Locker` 实例（`spring.lock` 前缀为四个锁后端共享——一个二进制只 blank-import 一个
锁后端）。

---

## 1. 完整工程示例

一个长任务持锁的批处理服务，带启动就绪探针与可观测。文件树：

```
demo/
├── go.mod
├── main.go
├── batch.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    go.etcd.io/etcd/client/v3     latest
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-lock-etcd latest
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
    _ "go-spring.org/starter-lock-etcd"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**batch.go** —— 应用的全部锁面：

```go
package main

import (
    "context"
    "time"

    "go-spring.org/cloud/lock"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

type Batch struct {
    // 实例名下的 bean 默认已带 observe 包装（trace span + metric + 访问日志）；
    // observe.enabled=false 可退回裸 locker（见 §6）。
    Locker lock.Locker `autowire:"main"`
}

func init() {
    gs.Provide(&Batch{}).Export(gs.As[gs.Rooter]())
}

func (b *Batch) Run(ctx context.Context) {
    lk, err := b.Locker.Acquire(ctx, "batch-run") // 竞争时阻塞
    if err != nil {
        return
    }
    defer lk.Unlock(ctx)
    select {
    case <-lk.Lost(): // lease 过期 / 连接丢失 —— 中止批处理
        log.Warnf(ctx, log.TagAppDef, "lock lost mid-run, aborting")
    case <-time.After(30 * time.Second):
        log.Infof(ctx, log.TagAppDef, "batch finished")
    }
}
```

**conf/app.properties** —— 上述代码用到的完整注释配置面：

```properties
# --- etcd 锁 -------------------------------------------------------------------
spring.lock.main.endpoints=127.0.0.1:2379
spring.lock.main.ttl=10s
spring.lock.main.key-prefix=/starter-lock-etcd/
# dial-timeout 同时限定初始连接与启动就绪探针——集群不可达时启动即失败，
# 而不是等到第一次 Acquire。
# spring.lock.main.dial-timeout=5s

# --- 可观测（starter-otel） ----------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317

# observe-lock 适配器的访问日志粒度（以下为默认值）。
# spring.lock.main.observability.level=brief
# spring.lock.main.observability.maxArgBytes=512
```

**验证**（本地 etcd，可用 `example/docker-compose.yml`）：

```bash
go run .                                                  # etcd 不可达则启动失败（探针）
ETCDCTL_API=3 etcdctl get --prefix /starter-lock-etcd/   # 持锁期间可见锁 key
```

可运行的 [example/](example) 覆盖 TryAcquire(+WithTTL)/竞争/idempotent-Unlock/重新获取并在
成功时退出 0；`example/check.sh` 用 docker compose 包裹执行。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-lock-etcd
  └─ gs.Module(gs.OnProperty("spring.lock"))
        └─ conf.BindEach("${spring.lock}") 逐条目 <name>：
             ├─ endpoints 为空时 fail fast（启动报错并点名实例）
             ├─ Provide newLocker  → bean "<name>"           （Export lock.Locker，
             │                                                        Destroy → Close）
             └─ 除非 observe.enabled=false，newLocker 默认用 observe-lock 包装
  ├─ newEtcdLocker：
  │    clientv3.New（DialTimeout，tls 时经 tlsconf.Build）
  │    + 就绪探针：DialTimeout 内对 endpoints[0] 执行 cli.Status——
  │      启动期即验证凭据/TLS；失败则关闭 client 并中止启动
  ├─ bean 装配：消费方 autowire:"<name>" 解析（bean 已默认带 observe 包装）
  └─ SIGTERM：Destroy → Close 关闭共享 *clientv3.Client。
               已发出的锁各持自己的 session，Unlock 前一直有效——
               进程死亡后 lease 自然过期。
```

每次获取锁都会开**全新 `concurrency.Session`**（自带 lease 与自动 keepalive，
`etcdlock.go` newMutex），因此每次持有的 lease 与 `Lost()` 通道与其他持有完全隔离。

### 2.2 三层时序解析（所有锁后端共享）

TTL / renew / retry 经 `lock.Resolve`（cloud/lock/defaults.go）解析，高层优先：

| 层 | 来源 | 本后端 |
|----|------|--------|
| 1. 每次调用 option | `lock.WithTTL` / `WithRenewInterval` / `WithRetryInterval` | 全部生效 |
| 2. starter 默认 | `spring.lock.<name>.ttl` 等 | **仅 TTL** —— etcd concurrency 包自行维持每个 session 的 lease，因此没有 renew/retry key |
| 3. 包默认 | TTL `30s`、renew `TTL/3`、retry `100ms` | 兜底仍未设置的项 |

解析后的 TTL 换算为整秒，不足 1 秒**向上取整**且下限 1 秒（`etcdlock.go` ttlSeconds），
因为 etcd lease 使用整数 TTL。

### 2.3 一次锁的逐层走读（acquire → 持有 → release）

`TryAcquire(ctx, "report")`：

1. `lock.Resolve(defaults, opts...)` —— 无 `WithTTL` 时用配置 TTL（example 传了
   `WithTTL(10s)`）；无 `WithToken` 时生成 fencing token。
2. `newMutex`：`concurrency.NewSession(client, WithTTL(ttlSeconds(o.TTL)))` —— session 的
   lease 自动 keepalive 由 client-go 内部执行；再 `concurrency.NewMutex(sess, keyPrefix+key)`。
3. `mu.TryLock(ctx)` —— 竞争以哨兵错误 `concurrency.ErrLocked` 出现，被翻译为
   `ok=false, err=nil`；其他错误即后端故障。`Acquire` 调 `mu.Lock(ctx)`，由 etcd watch
   驱动阻塞直到拿锁或 ctx 结束；失败时关闭新开的 session。
4. 持有期：etcd 自动续 lease（keepalive 流）。句柄起一个 goroutine，把 `session.Done()`
   （lease 过期/连接丢失）与主动 `released` 信号扇入 `Lost()` 返回的单通道——任一触发即
   退出，goroutine 不超出持有期。
5. `Unlock`：best-effort `mutex.Unlock` + `session.Close`（关 session 即释放 lease 并触发
   `Lost()`）。幂等。lease 已丢失后的 unlock 错误被吞掉（非 canceled 的后端错误除外），
   契合"已释放 = 成功"的抽象契约。`Key()` 返回不带前缀的调用方 key。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.lock.<name>` 之下（精确匹配，无宽松形态）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `endpoints` | []string | — | **必填**。集群节点地址，注册前检查。 | 空 → 启动失败并点名实例。 |
| `username` / `password` | string | — | etcd 鉴权凭据；匿名集群留空。 | 凭据错误在启动 `Status` 探针即失败（不只是建 client）。 |
| `dial-timeout` | duration | `5s` | 同时限定初始连接**与**就绪探针预算。 | 过低 → 慢网络启动失败；过高 → fail-fast 变慢。 |
| `ttl` | duration | `30s` | 每把锁的 lease TTL，进 §2.2 第 2 层。不足 1 秒**向上取整**。 | `500ms` 无声变 `1s`。 |
| `key-prefix` | string | `/lock/` | 拼在每个锁 key 前；尾部斜杠保留。 | 跨应用共享前缀 → 互相争锁。 |
| `tls.enabled` | bool | `false` | 经 `tlsconf.Build` 应用共享 `tlsconf` 块（`server-name`、`ca-file`、`cert-file`、`key-file`、`insecure-skip-verify`）。 | 材料错误 → 启动期建 client 失败。 |
| `observe.enabled` | bool | `true` | 默认用 observe-lock 适配器包装 `<name>` 主 Locker bean（trace span + metric + 访问日志）。`false` = 裸 locker。 | 迁移：`<name>-observed` bean 已移除，请注入 `<name>`。 |
| `observability.level` | string | `brief` | 访问日志粒度 `off`/`brief`/`detailed`（detailed 记录锁 key）。 | 非法值 → 启动期绑定错误。 |
| `observability.maxArgBytes` | int | `512` | 记录锁 key 的字节上限。 | 过低会截断日志里的 key。 |
| `observability.skipOps` | []string | — | 从访问日志排除的操作（`acquire`、`try_acquire`）。 | 拼错则静默无效。 |

⚠ 本 starter **没有** `renew-interval`/`retry-interval` key：etcd concurrency 包自动维持
lease（§2.2 第 2 层）。实例权重（`Weight=0` 摘流）是注册中心/负载均衡概念，与锁后端无关。

---

## 4. 验证与故障演练

### 4.1 竞争锁

两个副本指向同一 etcd。A 的 `Acquire` 立即返回；B 的 `Acquire` 阻塞；B 的 `TryAcquire`
返回 `ok=false, err=nil`：

```bash
ETCDCTL_API=3 etcdctl get --prefix /starter-lock-etcd/ --keys-only   # 持有者的 key
```

持锁期间可见 concurrency 的 MVCC key，Unlock 后消失。

### 4.2 持锁期间 TTL 到期（宕机演练）

1. 短 TTL 获取：`lock.WithTTL(5 * time.Second)`（或配 `spring.lock.main.ttl=5s`）。
2. `kill -9` 持有者——不 Unlock、无 keepalive。
3. lease 约 TTL 后过期；key 消失，副本 B 中等待的 `Acquire` 在 watch 时延内获胜。
4. 也演练活进程路径：改用 SIGSTOP 暂停持有者——keepalive 停但 session 对象还在；TTL 后
   lease 过期，恢复后 `Lost()` 触发，长临界区中止而非无保护地继续写。

### 4.3 启动就绪探针

把 `endpoints` 指向死端口：启动报 `lock-etcd: startup probe failed for ...` 并关闭
client——配置错的应用到不了 `Acquire`。

### 4.4 观察 locker（默认开启）

注入 `autowire:"main"`，配置 starter-otel 后制造锁流量：

- span 名 `acquire` / `try_acquire`，属性 `lock.system="etcd"`、`lock.operation`、
  `lock.key`（detailed 级别）；竞争失败带 `lock.acquired=false`。
- 指标直方图 `lock.operation.duration`，属性同上——流量后到采集端查看。

未 import starter-otel 时包装器近乎无感的 no-op。

### 4.5 冒烟测试

```bash
cd example && ./check.sh    # docker 门控：compose 起 etcd，跑自校验 example
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报 `endpoints is required for instance "<n>"` | 实例缺 endpoints | 补 key 或删实例。 |
| 启动报 `startup probe failed` | 集群不可达 / 凭据错 / TLS 材料错 | `Status` 探针启动期三合一验证——修连通性或鉴权。 |
| 进程健康却中途丢锁 | keepalive 流断（网络分区、etcd 丢仲裁） | keepalive 依赖 client→server 流量；查连通与仲裁，必要时缩短 TTL 限定暴露面。 |
| 亚秒 TTL 表现为 1s | lease 整秒取整（`ttlSeconds` 向上、下限 1s） | 选整秒 TTL；`500ms` 不可表达。 |
| `TryAcquire` 返回 `ok=false` 无错误 | 普通竞争（`ErrLocked` 被翻译） | 预期行为；等待用 `Acquire`。 |
| `<name>-observed` bean 不存在了 | 2026-08 移除 | 注入 `<name>`——默认已带观测；`observe.enabled=false` 得裸 locker。 |
| 默认包装了却无 span/metric | 未 import starter-otel | 加上；OTel 钩子缺它时静默 no-op。 |
| 进程死后锁存活超过 TTL | TTL 比以为的大，或取整 | 用 `etcdctl lease list`/计时核对；注意 `ttl` 是 lease 时长不是重试节奏。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数（含 tls/observer） | 15 |
| 其中必填 | 1（`endpoints`） |
| quickstart 前置外部依赖 | 1（etcd） |
| "注意/坑"条数 | 3 |

设计嫌疑清单：

- 已解决（2026-08）：主 `<name>` bean 现在默认自带观测（`newLocker` 内透明包装）；独立的
  `<name>-observed` bean 已移除——迁移：注入 `<name>`。
- etcd 无原生 fencing token——`Token()` 只是 lock 包的 token，下游资源无法对照 etcd 状态
  验证（已注明的限制，任何 fencing 设计都应记一笔）。
