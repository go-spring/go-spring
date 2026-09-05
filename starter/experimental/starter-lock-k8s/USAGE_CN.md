# starter-lock-k8s 使用说明 — 参考手册

[English](USAGE.md) | [中文](USAGE_CN.md)

详尽使用参考。概览见 [README_CN.md](README_CN.md)。下文所有行为声明均对照 starter 源码
（`starter.go`、`config.go`、`k8slock.go`、`observe.go`）、共享抽象
[cloud/lock](../../../cloud/lock) 与 [example/](example)（含集群内
运行的 `deploy/`）核实。Lease 语义对齐 client-go 的 leader election——见
[Kubernetes Lease API](https://kubernetes.io/docs/concepts/architecture/leases/)；本文只写
go-spring 的增量。

**激活方式**：任一 `spring.lock.<name>.*` 配置即为每个 `<name>` 注册一个 Lease 后端的
`lock.Locker` 实例。这是 K8s 原生后端：加锁/选主直接复用控制面的
`coordination.k8s.io/Lease` API（即 `--leader-elect` 背后的机制），集群内应用**无需额外
中间件**。`spring.lock` 前缀为四个锁后端共享——一个二进制只 blank-import 一个锁后端。

---

## 1. 完整工程示例

部署在集群内的选主单例 worker。文件树：

```
demo/
├── go.mod
├── main.go
├── leader.go
├── conf/
│   └── app.properties
└── deploy/
    ├── deployment.yaml
    └── rbac.yaml
```

**go.mod**（关键依赖）：

```
require (
    k8s.io/client-go              latest
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-lock-k8s latest
    go-spring.org/starter-actuator latest   // 可选：探针
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-lock-k8s"
)

func main() { gs.Run() }
```

**leader.go** —— 应用的全部锁面：

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
    Locker lock.Locker `autowire:"default"`
}

func init() {
    gs.Provide(&Worker{}).Export(gs.As[gs.Rooter]())
}

func (w *Worker) Init(ctx context.Context) {
    e := lock.NewElection(lock.ElectionConfig{
        Locker: w.Locker,
        Key:    "demo-leader",
        OnElected: func(context.Context) {
            log.Infof(ctx, log.TagAppDef, "elected leader over Lease demo-leader")
        },
        RetryInterval: 500 * time.Millisecond,
    })
    go func() { _ = e.Run(ctx) }()
}
```

**conf/app.properties** —— 完整配置面（集群内极简）：

```properties
# 集群内零 key 即可激活（ServiceAccount + "default" namespace）。
# 此处显式写出以便说明：
spring.lock.default.namespace=default
# 仅集群外（本地开发/测试）需要：
# spring.lock.default.kubeconfig=/home/me/.kube/config
# Lease 名 = key-prefix + key，必须是合法 DNS-1123 subdomain
# （小写字母数字、'-' 和 '.'）：
# spring.lock.default.key-prefix=demo-

# observe-lock 适配器的访问日志粒度（以下为默认值）。
```

**deploy/rbac.yaml** —— ServiceAccount 需要在目标 namespace 拥有
`coordination.k8s.io/leases` 的 get/create/update 权限（可用的 Role 见
`example/deploy/rbac.yaml`）。

**验证**：

```bash
kubectl apply -f deploy/
kubectl logs deploy/demo -f            # "elected leader over Lease demo-leader"
kubectl get lease demo-leader -o yaml  # holderIdentity 即 fencing token
```

集群外时，example 以 wiring-only 模式启动（不声明 `spring.lock` 条目，因此不构建 Locker
bean）——见 `example/example.go`。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-lock-k8s
  └─ gs.Module(gs.OnProperty("spring.lock"))
        └─ conf.BindEach("${spring.lock}") 逐条目 <name>：
             ├─ Provide newLocker  → bean "<name>"           （Export lock.Locker，
             │                                                       Destroy → Close）
             └─ 除非 observe.enabled=false，newLocker 默认用 observe-lock 包装
  ├─ newK8sLocker：buildClient——集群内 ServiceAccount 配置，或设置了
  │  kubeconfig 时用文件。急切构建：ServiceAccount 缺失 / kubeconfig 损坏
  │  都在启动期失败。
  ├─ bean 装配：消费方 autowire:"<name>" 解析（bean 已默认带 observe 包装）
  └─ SIGTERM：Destroy → Close 是空操作（clientset 无必须拆除的连接）。
                 已发出的锁保留各自的续约 goroutine；进程死亡后其 Lease
                 自然过期。
```

每次获取锁对应**一个 Lease 对象**（名 = `keyPrefix + key`）并运行**自己的续约
goroutine**（`k8slock.go` tryOnce），各持有互不影响。

### 2.2 三层时序解析（所有锁后端共享）

TTL / renew / retry 经 `lock.Resolve`（cloud/lock/resolve.go）解析，高层优先：

| 层 | 来源 | 本后端 |
|----|------|--------|
| 1. 每次调用 option | `lock.WithTTL` / `WithRenewInterval` / `WithRetryInterval` | 本 starter 喂入的**唯一一层**——完全没有时序 key |
| 2. starter 默认 | 无 | 直接用 `lock.Resolve(lock.DefaultOptions{}, opts...)`（零默认层） |
| 3. 包默认 | TTL `30s`、renew `TTL/3`、retry `100ms` | 兜底层 1 未设置的项 |

解析后的 TTL 写入 Lease 的 `leaseDurationSeconds`——整秒、向上取整、下限 1 秒
（`k8slock.go` leaseSeconds）。

### 2.3 一次锁的逐层走读（acquire → 持有 → release）

`TryAcquire(ctx, "demo-leader")`：

1. `lock.Resolve(lock.DefaultOptions{}, opts...)` —— TTL/renew/retry 来自每次调用的 option 或包默认；无
   `WithToken` 时生成 fencing token。
2. 在 Lease `<namespace>/<keyPrefix+key>` 上构建 `resourcelock.LeaseLock`
   （`Identity = token`），执行一次 **acquire-or-renew**（`tryAcquireOrRenew`，对齐
   client-go leaderelection）：
   - Lease 不存在 → 创建（创建竞争失败 ⇒ 视为被竞争）；
   - Lease 被别的 identity 有效持有（`RenewTime + ttl` 仍在未来）⇒ `ok=false, err=nil`；
   - 已过期 / 无人持有 / 已是自己 ⇒ 接管或续约——仅真正易主时保留 `AcquireTime` 并递增
     `LeaderTransitions`（更新冲突 ⇒ 视为被竞争）。
3. 持有期：续约 goroutine 每 `RenewInterval`（默认 TTL/3）刷新 Lease。
   - 续约证明被接管（`ok=false`）⇒ 立即触发 `Lost()`。
   - 瞬时 API 错误容忍到 `time.Since(lastRenew) >= TTL` 才触发 `Lost()`——短于 TTL 的抖动
     不丢锁，更长的中断会丢。
4. `Acquire` 每 `RetryInterval`（默认 100ms）重试 `tryOnce` 直到拿锁或 ctx 结束；瞬时
   API 错误直接中止（只有普通竞争才静默重试）。
5. `Unlock`：停止续约、触发 `Lost()`，随后**清空 Lease 的 `holderIdentity`**（并把
   `leaseDurationSeconds` 降为 1）做 best-effort 释放——等待者立即接管而无需等满 TTL。
   Lease 已属他人时什么都不做。幂等。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.lock.<name>` 之下（精确匹配，无宽松形态）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `namespace` | string | `default` | Lease 对象所在 namespace；ServiceAccount 需在此具备 lease RBAC。 | RBAC 缺失 → 首次 acquire 报 API 错误（启动仍成功——建 client 不碰 lease）。 |
| `kubeconfig` | string | — | 集群外 kubeconfig 路径；空则用集群内 ServiceAccount 配置。 | 集群外留空 → 启动报 `in-cluster config (set kubeconfig when running outside a cluster)`。 |
| `key-prefix` | string | — | 拼在每个锁 key 前构成 Lease 名。结果必须是合法 DNS-1123 subdomain。 | 名字非法（下划线、大写）→ acquire 时 Lease 创建/更新被拒。 |
| `observe.enabled` | bool | `true` | 默认用 observe-lock 适配器包装 `<name>` 主 Locker bean（trace span + metric + 访问日志）。`false` = 裸 locker。 | 迁移：`<name>-observed` bean 已移除，请注入 `<name>`。 |

⚠ 本 starter **完全没有时序 key**：TTL/renew/retry 只走每次调用的 `lock.Option`（§2.2）。
实例权重（`Weight=0` 摘流）是注册中心/负载均衡概念，与锁后端无关。

---

## 4. 验证与故障演练

### 4.1 竞争锁（两副本）

```bash
kubectl scale deploy/demo --replicas=2
kubectl get lease demo-leader -o jsonpath='{.spec.holderIdentity}'   # 只有一个 token
kubectl logs deploy/demo -c demo --prefix | grep -c 'elected leader' # 恰好一个副本
```

落选副本的 `TryAcquire` 返回 `ok=false, err=nil`；`Acquire` 每 `RetryInterval` 重试。

### 4.2 持锁期间 TTL 到期（故障转移演练）

1. `WithTTL(10*time.Second)` 获取后 `kill -9` 持有 pod。
2. 续约停止，Lease 的 `renewTime` 变陈旧。约 TTL 后 lease 判定过期，等待中的 `Acquire`
   获胜（client-go 式 `RenewTime + ttl` 检查）。
3. 更快路径：**优雅**退出会执行 `Unlock`，清空 `holderIdentity` 并把
   `leaseDurationSeconds` 置 1——约 1 秒完成接管而非等满 TTL。验证：

```bash
kubectl delete pod <leader-pod>    # 优雅退出 → 快速易主
kubectl get lease demo-leader -o jsonpath='{.spec.holderIdentity}'   # 新 token
```

### 4.3 API 中断容忍演练

续约循环对瞬时 API 错误的容忍上限是 `lastRenew + TTL`。`WithTTL(30s)` 时短于 30s 的
API-server 抖动不丢领导权；超过 TTL 才触发 `Lost()`——可在人为长时间中断后观察
`Lost()` 消费方触发。

### 4.4 观察 locker（默认开启）

注入 `autowire:"default"`，配置 starter-otel 后制造 acquire 流量：span
`acquire`/`try_acquire` 带 `lock.system="k8s"`，指标 `lock.operation.duration`。未 import
starter-otel 时包装器近乎无感的 no-op。

### 4.5 集群外 wiring 检查

`KUBECONFIG=~/... go run ./example` 对开发集群运行；完全无集群时 example 以 wiring-only
模式干净启动并退出（未声明 `spring.lock` 条目）。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报 `in-cluster config` | 集群外运行且未设 `kubeconfig` | 设置 `spring.lock.<n>.kubeconfig`。 |
| 启动报加载 kubeconfig 失败 | 路径/格式错误 | 报错会点名文件；修路径。 |
| acquire 报 `forbidden` | ServiceAccount 缺 lease get/create/update RBAC | 应用类似 `example/deploy/rbac.yaml` 的 Role；注意启动本身是成功的。 |
| Lease 创建/更新因名字被拒 | `key-prefix`/key 非 DNS-1123 subdomain | 只用小写字母数字、`-`、`.`。 |
| 领导权每几秒抖动一次 | 续约跑不过 API 时延/配额；TTL 太紧 | 调大 `WithTTL`；续约容忍度由 TTL 限定。 |
| 故障转移要等满 TTL | 持有者崩溃没走 Unlock | 崩溃情形属预期；优雅退出会清 holder 实现 ~1s 接管。 |
| `<name>-observed` bean 不存在了 | 2026-08 移除 | 注入 `<name>`——默认已带观测；`observe.enabled=false` 得裸 locker。 |
| 两个副本都自认 leader | 选主逻辑忽略 `Lost()`，或用手工 TryAcquire 当所有权凭据 | select `Lost()` 并中止；Lease 是唯一事实源。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数（含 observer） | 7 |
| 其中必填 | 0 |
| quickstart 前置外部依赖 | 1（K8s API server） |
| "注意/坑"条数 | 4 |

设计嫌疑清单：

- 已解决（2026-08）：主 `<name>` bean 现在默认自带观测（`newLocker` 内透明包装）；独立的
  `<name>-observed` bean 已移除——迁移：注入 `<name>`。
- K8s 是唯一零 starter 级时序 key 的锁后端而 redis 有三个——不对称是有意的（控制面 Lease
  时序本来就粗），但任何调参都得走每次调用 option。
- RBAC 缺口在首次 acquire 而非启动期暴露（建 client 不碰 lease）——启动期做一次试探性
  acquire 可恢复 fail-fast。
