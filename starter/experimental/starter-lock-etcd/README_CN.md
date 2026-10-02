# starter-lock-etcd

[English](README.md) | [中文](README_CN.md)

`starter-lock-etcd` 是 [`go-spring.org/cloud/lock`](../../../cloud/lock)
分布式锁抽象的 etcd 后端实现。空导入本 starter 会为每个配置实例注册一个
`lock.Locker` Bean；切换到 Redis 或 Consul 只需换空导入，业务代码无需改动。

## 安装

```bash
go get go-spring.org/starter-lock-etcd
```

## 快速开始

### 1. 引入 `starter-lock-etcd` 包

```go
import _ "go-spring.org/starter-lock-etcd"
```

### 2. 配置锁实例

在项目的[配置文件](example/conf/app.properties) 中添加
`spring.lock.instances.etcd.<name>` 配置，例如：

```properties
spring.lock.instances.etcd.main.endpoints=127.0.0.1:2379
spring.lock.instances.etcd.main.ttl=30s
spring.lock.instances.etcd.main.key-prefix=/lock/
```

只有 `endpoints` 是必填项，其余字段都有默认值；`endpoints` 为空会在启动时快速失败。

### 3. 注入 `lock.Locker`

```go
import "go-spring.org/cloud/lock"

type Service struct {
    Locker lock.Locker `autowire:"main"`
}
```

### 4. 获取与释放

```go
l, ok, err := s.Locker.TryAcquire(ctx, "invoice/42")
if err != nil {
    return err
}
if !ok {
    return nil // 已被其他实例持有
}
defer l.Unlock(ctx)

select {
case <-l.Lost():
    // 租约失效，终止临界区
case <-workDone:
}
```

## 配置项

所有配置项挂在 `spring.lock.instances.etcd.<name>` 之下：

| Key             | 默认值    | 说明                                    |
|-----------------|-----------|-----------------------------------------|
| `endpoints`     | (必填)    | etcd 集群地址                           |
| `username`      | `""`      | etcd 认证用户名                         |
| `password`      | `""`      | etcd 认证密码                           |
| `dial-timeout`  | `5s`      | 初始连接超时，同时是启动探针预算        |
| `ttl`           | `30s`     | 每次加锁的租约 TTL（向上取整为整秒，最小 1 秒） |
| `key-prefix`    | `/lock/`  | 所有锁键的前缀                          |
| `tls.enabled`   | `false`   | 启用 TLS                                |
| `tls.cert-file` | `""`      | 客户端证书（mTLS）                      |
| `tls.key-file`  | `""`      | 客户端私钥（mTLS）                      |
| `tls.ca-file` | `""`   | 受信任 CA 的 PEM 集                     |

## 核心行为

* **独立租约。** 每次 `Acquire`/`TryAcquire` 都会新建一个
  `concurrency.Session`，因此每个持锁的 `Lost()` 通道和续约相互独立。
* **自动续约。** etcd concurrency 包内部维护 session 的 keepalive，业务无需
  自己启动续约 goroutine。
* **幂等 `Unlock`。** 重复调用返回 `nil`。启动前发放的锁在停机时不受影响，直到
  被显式释放或租约到期。
* **快速失败。** 集群不可达、凭据错误、`endpoints` 为空都会在启动阶段抛错，
  而不是延后到第一次加锁。

## 领袖选举

由于 `lock.NewElection` 是基于 `lock.Locker` 之上的通用能力，切换任何后端
都能复用同一份选举代码：

```go
elec := lock.NewElection(lock.ElectionConfig{
    Locker: locker, // 注入的 lock.Locker
    Key:    "workers/leader",
    OnStartedLeading: func(ctx context.Context) { runLeaderWork(ctx) },
})
go elec.Run(ctx)
```

## 日志 tag

`cloud/lock` 的运行期日志使用 tag `_app_lock_access`（lock 访问日志——每次 acquire、
try_acquire、unlock，以及持锁期间发生的丢锁）。如需与主日志分开单独调整，可为该 tag 绑定
独立的 logger：

```properties
logger.lock_access.type=Logger
logger.lock_access.level=WARN
logger.lock_access.tag=_app_lock_access
```

## 设计说明

* **自持客户端——没有 `client=` 键。** 与复用应用 `*redis.Client` 的 Redis 后端不同，仅用于协调的 etcd 集群十分常见，因此锁从绑定的配置自建 `clientv3.Client`，并在 destroy 时关闭。
* **TTL 归一化为整秒。** etcd 拒绝亚秒级的 session TTL，因此配置的 TTL 会向上取整为整秒（最小 1s）；`0` 或负值则回退到 `30s` 默认值。
* **此处不适用 `renew-interval`。** etcd 的 `concurrency.Session` 自行维持租约存活，没有可调的续期旋钮——该配置项只服务于 consul 与 redis 后端。
