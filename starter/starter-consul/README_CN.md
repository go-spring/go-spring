# starter-consul

[English](README.md) | [中文](README_CN.md)

`starter-consul` 提供了基于 github.com/hashicorp/consul/api 的 Consul 客户端封装，
方便在 Go-Spring 服务中快速集成和使用 Consul。

## 安装

```bash
go get go-spring.org/starter-consul
```

## 快速开始

### 1. 引入 `starter-consul` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-consul"
```

### 2. 配置 Consul 实例

在项目的[配置文件](example/conf/app.properties)中添加 Consul 配置，比如：

```properties
spring.consul.address=127.0.0.1:8500
```

### 3. 注入 Consul 实例

参见 [example.go](example/example.go) 文件。

```go
import "github.com/hashicorp/consul/api"

type Service struct {
    Consul *api.Client `autowire:"__default__"`
}
```

### 4. 使用 Consul 实例

参见 [example.go](example/example.go) 文件。

```go
_, err := s.Consul.KV().Put(&api.KVPair{Key: "key", Value: []byte("value")}, nil)
pair, _, err := s.Consul.KV().Get("key", nil)
```

## 核心功能

[example.go](example/example.go) 中的 runTest 演示了三项核心 Consul 能力：

1. **KV put/get**：通过 `s.Consul.KV().Put(...)` / `s.Consul.KV().Get(...)` 写入并读取键值。
2. **服务注册与发现**：通过 `s.Consul.Agent().ServiceRegister(...)` 注册服务，
   并使用 `s.Consul.Agent().Services()` 进行查询。
3. **服务下线**：通过 `s.Consul.Agent().ServiceDeregister(...)` 注销服务，
   随后再次调用 `s.Consul.Agent().Services()` 确认服务已不在列表中。

## 高级功能

* **支持多 Consul 实例**：可以在配置文件的 `spring.consul.instances` 下定义多个 Consul 实例，并在项目中使用 name 进行引用。
