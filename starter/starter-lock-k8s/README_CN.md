# starter-lock-k8s

[English](README.md) | [中文](README_CN.md)

`starter-lock-k8s` 为 Go-Spring 提供**基于 Kubernetes Lease 的分布式锁与选主**能力,
**无需任何外部中间件**。它用 `coordination.k8s.io/Lease` 对象为 `stdlib/lock` 提供后端
——正是 `kube-controller-manager --leader-elect` 与 spring-cloud-kubernetes 所用的机制
——因此集群内应用只依赖控制面本身即可完成选主或守护独占逻辑。

空导入本 starter 并声明一条 `spring.lock.<name>` 配置,即注册一个名为 `<name>` 的
`lock.Locker` bean(来自 `stdlib/lock`)。业务代码只注入 `lock.Locker` / 构建
`lock.Election`,从不感知本包;因此在共用 `spring.lock` 前缀下切换到 etcd/consul/redis
后端只需改一行空导入。

## 安装

```bash
go get go-spring.org/starter-lock-k8s
```

## 快速开始

### 1. 导入包

```go
import _ "go-spring.org/starter-lock-k8s"
```

### 2. 声明 Locker

```properties
spring.lock.default.namespace=default
# kubeconfig 留空表示使用集群内 ServiceAccount 认证;集群外运行时填路径。
# spring.lock.default.kubeconfig=/home/me/.kube/config
```

### 3. 选主

```go
type Worker struct {
    Locker lock.Locker `autowire:""`
}

func (w *Worker) Elect(ctx context.Context) {
    e := lock.NewElection(lock.ElectionConfig{
        Locker:    w.Locker,
        Key:       "example-leader",
        OnElected: func(ctx context.Context) { /* 仅 leader 执行 */ },
    })
    _ = e.Run(ctx) // 阻塞,通常作为后台 runner
}
```

或直接守护一次性的独占逻辑:

```go
l, err := w.Locker.Acquire(ctx, "nightly-migration")
if err == nil {
    defer l.Unlock(ctx)
    // 只有一个副本执行
}
```

## 配置

绑定在 `spring.lock.<name>` 下:

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `namespace` | `default` | Lease 对象所在命名空间。 |
| `kubeconfig` | (空) | kubeconfig 路径;留空使用集群内 ServiceAccount 认证。 |
| `key-prefix` | (空) | 拼在锁 key 前构成 Lease 名;结果须为合法的 DNS-1123 名称。 |

锁的时序参数(`TTL`、续约、重试)不在此处配置,而是由每次获取时的 `lock.Option` 及其
默认值携带,使各后端的调节旋钮完全一致。

## 工作原理

- 每条 `spring.lock.<name>` 构建一个持有共享 clientset 的 Locker,提前创建,以便
  ServiceAccount 缺失或 kubeconfig 错误时在启动阶段即失败。
- `Acquire`/`TryAcquire` 将锁 key 映射到单个 Lease(`<key-prefix><key>`)。其
  "获取或续约" 逻辑对齐 client-go 选主:不存在则创建,已过期或本就属于自己则接管,
  否则报告争用。`Acquire` 按 `RetryInterval` 重试争用,`TryAcquire` 立即返回 `ok=false`。
- 持有的锁在后台续约;若续约发现租约已被他人接管,或 API 在超过租约时长内不可达,则关闭
  `Lost()`,以便长临界区及时中止。
- `Unlock` 清空 Lease 的 `holderIdentity`,等待者可立即接管而无需等到租约过期;该操作幂等。

## RBAC

ServiceAccount 需要在其命名空间内对 `coordination.k8s.io/leases` 拥有
`get/create/update`(无需 `delete`——释放通过清空 `holderIdentity` 完成)。见
[example/deploy/rbac.yaml](example/deploy/rbac.yaml)。

## 集群内验证

单元测试用 client-go fake clientset 覆盖了完整的获取/争用/续约/释放/选主逻辑。端到端选主
需要真实集群:

```bash
kubectl apply -f example/deploy/rbac.yaml
# 为 example/ 构建并推送镜像,再 apply example/deploy/deployment.yaml(replicas: 3),然后:
kubectl logs deploy/lock-k8s-example --all-containers   # 恰有一个 pod 打印 "became leader"
kubectl get lease example-leader -o yaml                # holderIdentity 即当选者
```
