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

在项目的[配置文件](example/conf/app.properties)中添加 MongoDB 配置，比如：

```properties
spring.mongodb.uri=mongodb://127.0.0.1:27017
```

### 3. 注入 MongoDB 实例

参见 [example.go](example/example.go) 文件。

```go
import "go.mongodb.org/mongo-driver/v2/mongo"

type Service struct {
    Mongo *mongo.Client `autowire:"__default__"`
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

* **支持多 MongoDB 实例**：可以在配置文件的 `spring.mongodb.instances` 下定义多个 MongoDB 实例，并在项目中使用 name 进行引用。
