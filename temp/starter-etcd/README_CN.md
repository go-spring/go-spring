# starter-etcd

[English](README.md) | [中文](README_CN.md)

`starter-etcd` 提供了基于 go.etcd.io/etcd/client/v3 的 etcd v3 客户端封装，
方便在 Go-Spring 服务中快速集成和使用 etcd。

## 安装

```bash
go get go-spring.org/starter-etcd
```

## 快速开始

### 1. 引入 `starter-etcd` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-etcd"
```

### 2. 配置 etcd 实例

在项目的[配置文件](example/conf/app.properties)中添加 etcd 配置，比如：

```properties
spring.etcd.endpoints=127.0.0.1:2379
```

多个 endpoint 以英文逗号分隔：`spring.etcd.endpoints=127.0.0.1:2379,127.0.0.1:2380`。

### 3. 注入 etcd 实例

参见 [example.go](example/example.go) 文件。

```go
import clientv3 "go.etcd.io/etcd/client/v3"

type Service struct {
    Etcd *clientv3.Client `autowire:"__default__"`
}
```

### 4. 使用 etcd 实例

参见 [example.go](example/example.go) 文件。

```go
_, err := s.Etcd.Put(ctx, "key", "value")
resp, err := s.Etcd.Get(ctx, "key")
```

## 高级功能

* **支持多 etcd 实例**：可以在配置文件的 `spring.etcd.instances` 下定义多个 etcd 实例，并在项目中使用 name 进行引用。

## 核心功能

[example.go](example/example.go) 演示了 etcd v3 的三个核心功能：

* **Put/Get**：使用 `cli.Put` 写入键值，再通过 `cli.Get` 读取并校验 `Kvs[0].Value`。
* **Watch**：通过 `cli.Watch` 订阅键变化，触发一次 `Put` 后在限定超时内接收到事件。
* **Lease + TTL**：使用 `cli.Grant` 申请租约，通过 `clientv3.WithLease` 绑定到键，再用 `cli.TimeToLive` 查询剩余 TTL。
