# Go-Spring Starters

[English](README.md) | [中文](README_CN.md)

本目录收录 Go-Spring 官方 starter —— 将第三方服务与框架接入 Go-Spring IoC 容器
及服务生命周期的集成模块。每个 starter 都是独立的 Go module。下面按领域归类，
方便你快速定位所需的 starter。

## Web / HTTP 框架

通过 Go-Spring 服务生命周期托管由应用提供的 Web 引擎。

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-gin` | [gin-gonic/gin](https://github.com/gin-gonic/gin) | 托管 `*gin.Engine` bean |
| `starter-echo` | [labstack/echo](https://github.com/labstack/echo) | 托管 `*echo.Echo` bean |
| `starter-hertz` | [CloudWeGo Hertz](https://github.com/cloudwego/hertz) | 托管 Hertz HTTP 服务 |

## RPC 框架

注册服务后由 starter 负责监听/服务构建、生命周期与优雅关闭。

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-grpc` | [google.golang.org/grpc](https://pkg.go.dev/google.golang.org/grpc) | 轻量 gRPC 服务封装 |
| `starter-kitex` | [cloudwego/kitex](https://github.com/cloudwego/kitex) | 服务封装，可选 etcd 注册 |
| `starter-thrift` | [Apache Thrift](https://thrift.apache.org/) | 基于 `TSimpleServer` 封装 `TProcessor` bean |
| `starter-dubbo` | [dubbo-go/v3](https://pkg.go.dev/dubbo.apache.org/dubbo-go/v3) | 完整服务端 + 客户端，支持注册中心服务发现 |

## WebSocket

提供已配置好的 upgrader / accept-options bean；路由挂载在应用已有的 HTTP 服务上
（自身不占用端口）。

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-websocket` | [gorilla/websocket](https://github.com/gorilla/websocket) | 提供 `*websocket.Upgrader` |
| `starter-websocket-coder` | [coder/websocket](https://github.com/coder/websocket) | 提供 `*websocket.AcceptOptions` |

## 数据库

| Starter | 底层库 | 领域 |
| --- | --- | --- |
| `starter-gorm-mysql` | [gorm](https://gorm.io/) | MySQL 关系型数据库 |
| `starter-mongodb` | [mongo-driver/v2](https://go.mongodb.org/mongo-driver/v2) | MongoDB 文档数据库 |
| `starter-neo4j` | [neo4j-go-driver](https://github.com/neo4j/neo4j-go-driver) | Neo4j 图数据库 |
| `starter-elasticsearch` | [go-elasticsearch](https://github.com/elastic/go-elasticsearch) | Elasticsearch 搜索引擎 |

## 缓存 (Redis)

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-go-redis` | [go-redis](https://github.com/redis/go-redis) | Redis 客户端 |
| `starter-redigo` | [redigo](https://github.com/gomodule/redigo) | Redis 客户端（另一驱动实现） |

## 消息队列

| Starter | 底层库 | 领域 |
| --- | --- | --- |
| `starter-kafka` | [twmb/franz-go](https://github.com/twmb/franz-go) | Kafka |
| `starter-pulsar` | [apache/pulsar-client-go](https://github.com/apache/pulsar-client-go) | Apache Pulsar |
| `starter-rabbitmq` | [amqp091-go](https://github.com/rabbitmq/amqp091-go) | RabbitMQ |

## 可观测 / 诊断

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-pprof` | Go `net/http/pprof` | 独立 HTTP 服务，暴露运行时性能剖析 |
