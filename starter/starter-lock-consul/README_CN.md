# starter-lock-consul

[English](README.md) | [中文](README_CN.md)

`starter-lock-consul` 将 [Consul](https://www.consul.io/) 接入
[`go-spring.org/stdlib/lock`](../../stdlib/lock) 定义的 `lock.Locker`
抽象，作为**分布式锁后端**。空导入该 starter 会为
`spring.lock.<name>` 下的每一项注册一个 `lock.Locker` Bean，每个 Bean 各自持有
一个自动续约的 Consul 会话。

由于注入的类型是中立的 `lock.Locker` 接口，在 Redis、etcd 与 Consul 三种后端
之间切换只需替换一行空导入，业务代码无需改动。

## 安装

```bash
go get go-spring.org/starter-lock-consul
```

## 快速开始

### 1. 导入包

```go
import _ "go-spring.org/starter-lock-consul"
```

### 2. 配置一个锁实例

在[配置文件](example/conf/app.properties)中按名字添加实例：

```properties
spring.lock.jobs.address=127.0.0.1:8500
spring.lock.jobs.ttl=30s
spring.lock.jobs.key-prefix=demo/lock/
```

### 3. 注入并使用 `lock.Locker`

```go
import "go-spring.org/stdlib/lock"

type Service struct {
    Locker lock.Locker `autowire:"jobs"`
}

func (s *Service) Run(ctx context.Context) error {
    l, ok, err := s.Locker.TryAcquire(ctx, "nightly-sync")
    if err != nil {
        return err
    }
    if !ok {
        return nil // 其他副本正在执行
    }
    defer l.Unlock(ctx)

    select {
    case <-l.Lost():
        return errors.New("lost lock, aborting")
    default:
    }
    return doWork(ctx)
}
```

## 配置项

所有配置都在 `spring.lock.<name>` 前缀下：

| 键                  | 默认值    | 说明                                                             |
|---------------------|-----------|------------------------------------------------------------------|
| `address`           | 必填      | Consul agent 地址，如 `127.0.0.1:8500`；为空时启动阶段 fail-fast。|
| `scheme`            | `http`    | `http` 或 `https`；开启 TLS 时自动升级为 `https`。               |
| `token`             | (空)      | Consul ACL 令牌（与每次加锁的 fencing token 无关）。             |
| `ttl`               | `30s`     | 会话 TTL；Consul 允许范围 `[10s, 86400s]`，由 `api.Lock` 自动续约。|
| `key-prefix`        | `lock/`   | 会拼接到每个 Key 前面，便于多应用共用同一 Consul 集群。          |
| `tls.enabled`       | `false`   | 是否启用到 Consul agent 的 TLS。                                 |
| `tls.server-name`   | (空)      | 覆盖用于校验服务器证书的主机名。                                 |
| `tls.ca-file`       | (空)      | 用于校验 agent 证书的 CA PEM 束。                                |
| `tls.cert-file`     | (空)      | mTLS 客户端证书。                                                |
| `tls.key-file`      | (空)      | mTLS 客户端私钥。                                                |

## Leader 选举

任意 `lock.Locker` 都能与 `lock.NewElection` 组合，因此同一份选举代码可以在
不同后端间无缝切换：

```go
e := lock.NewElection(lock.ElectionConfig{
    Locker: locker,
    Key:    "singleton-worker",
    OnElected: func(ctx context.Context) {
        // ctx 被取消前，我是 Leader。
    },
})
go e.Run(ctx)
```

## 工作原理

- 每个实例持有自己的 `*consul/api.Client`，由绑定的 `Config` 构造。
- 每次 `Acquire` / `TryAcquire` 都会用配置的 `SessionTTL` 构建一个新的
  `*api.Lock`；持锁期间 Consul 会自动续约会话。
- `api.Lock.Lock` 返回的 `<-chan struct{}` 直接作为句柄的 `Lost()`，因此会话
  失效或 agent 分区时业务临界区能立即感知。
- `Unlock` 释放锁并销毁会话，具备幂等性；`api.ErrLockNotHeld` 视为无害的
  "已释放" 信号，不会向上层抛错。
