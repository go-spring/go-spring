# starter-lock-etcd

[English](README.md) | [中文](README_CN.md)

`starter-lock-etcd` 是 [`go-spring.org/spring/lock`](../../spring/lock)
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
`spring.lock.<name>` 配置，例如：

```properties
spring.lock.main.endpoints=127.0.0.1:2379
spring.lock.main.ttl=30s
spring.lock.main.key-prefix=/lock/
```

只有 `endpoints` 是必填项，其余字段都有默认值；`endpoints` 为空会在启动时快速失败。

### 3. 注入 `lock.Locker`

```go
import "go-spring.org/spring/lock"

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

所有配置项挂在 `spring.lock.<name>` 之下：

| Key             | 默认值    | 说明                                    |
|-----------------|-----------|-----------------------------------------|
| `endpoints`     | (必填)    | etcd 集群地址                           |
| `username`      | `""`      | etcd 认证用户名                         |
| `password`      | `""`      | etcd 认证密码                           |
| `dial-timeout`  | `5s`      | 初始连接超时，同时是启动探针预算        |
| `ttl`           | `30s`     | 每次加锁的租约 TTL（最小 1 秒，按秒下取整） |
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
    OnElected: func(ctx context.Context) { runLeaderWork(ctx) },
})
go elec.Run(ctx)
```
