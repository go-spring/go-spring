# starter-mongodb

[English](README.md) | [中文](README_CN.md)

`starter-mongodb` 提供了基于 go.mongodb.org/mongo-driver/v2 的 MongoDB 客户端封装，
方便在 Go-Spring 服务中快速集成和使用 MongoDB。

## 安装

```bash
go get go-spring.org/starter-mongodb
```

## 快速开始

### 1. 引入 `starter-mongodb` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-mongodb"
```

### 2. 配置 MongoDB 实例

在项目的[配置文件](example/conf/app.properties)中，在 `spring.mongodb.<name>`
下定义一个或多个具名实例，比如：

```properties
spring.mongodb.a.uri=mongodb://127.0.0.1:27017
spring.mongodb.b.uri=mongodb://127.0.0.1:27017
```

### 3. 注入 MongoDB 实例

参见 [example.go](example/example.go) 文件。每个具名实例都会以该名称注册为一个
`*mongo.Client` bean，按名称注入所需实例即可。

```go
import "go.mongodb.org/mongo-driver/v2/mongo"

type Service struct {
    Mongo *mongo.Client `autowire:"a"`
}
```

### 4. 使用 MongoDB 实例

参见 [example.go](example/example.go) 文件。

```go
coll := s.Mongo.Database("test").Collection("kv")
_, err := coll.InsertOne(ctx, bson.M{"key": "key", "value": "value"})
err = coll.FindOne(ctx, bson.M{"key": "key"}).Decode(&res)
```

## 核心功能

[example.go](example/example.go) 端到端演示了三个核心 MongoDB 操作：

* **InsertOne** —— 插入文档并校验返回的 `InsertedID`。
* **FindOne** —— 读取文档并断言字段值。
* **UpdateOne** —— 通过 `$set` 更新字段，断言 `ModifiedCount == 1`，再次读取确认新值。

## 高级功能

* **多 MongoDB 实例**：`spring.mongodb` 下的每一项都会成为一个独立配置的
  `*mongo.Client` bean，按名称注入即可访问不同的集群或数据库。

* **可观测**：每个客户端通过一个 command monitor 桥接进 go-spring 的统一可观测体系，
  为每条 MongoDB 命令经 `starter-otel` 安装的 OpenTelemetry 全局 `TracerProvider`
  输出一个 client span。未引入 `starter-otel` 时该全局为 no-op，span 零开销、也无需
  逐应用接线。（该桥接直接基于 v2 驱动的 event API 实现，因为官方 `otelmongo`
  面向 v1 驱动，与此处使用的 v2 驱动类型不兼容。）

* **服务发现**：在实例上设置 `service-name` 后，地址将通过已注册的 discovery 后端解析，
  而非直接使用 URI 中的 host。框架会注入一个 `LiveDialer` 作为客户端的 dialer，
  于是每条新建连接都会连到当前存活的实例，地址变更无需重建客户端即可生效。
  用 `discovery` 选择后端（默认 `default`）；公司通过 `discovery.Register` 注册一次
  自己的命名服务即可。

  ```properties
  spring.mongodb.disc.uri=mongodb://0.0.0.0:0/?directConnection=true
  spring.mongodb.disc.service-name=mongo-cluster
  ```

  注意：这会绕过 MongoDB 自身的副本集 / mongos 拓扑发现——驱动直接连命名服务给出的地址。
  当目标是"经公司命名服务寻址"时使用；若要让驱动依据 URI 自行管理拓扑，请留空 `service-name`。

