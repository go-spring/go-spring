# Go-Spring Starters

[English](README.md) | [中文](README_CN.md)

本目录收录 Go-Spring 官方 starter —— 将第三方服务与框架接入 Go-Spring IoC 容器
及服务生命周期的集成模块。每个 starter 都是独立的 Go module。下面按领域归类，
方便你快速定位所需的 starter。

所有 starter 共同遵循的设计约束(形态、端口、driver 模式、多实例、fail-fast……）
见 [DESIGN_CN.md](DESIGN_CN.md)。

## Web / HTTP 框架

通过 Go-Spring 服务生命周期托管由应用提供的 Web 引擎。

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-gin` | [gin-gonic/gin](https://github.com/gin-gonic/gin) | 托管 `*gin.Engine` bean |
| `starter-echo` | [labstack/echo](https://github.com/labstack/echo) | 托管 `*echo.Echo` bean |
| `starter-hertz` | [CloudWeGo Hertz](https://github.com/cloudwego/hertz) | 托管 Hertz HTTP 服务 |
| `starter-go-zero/rest` | [zeromicro/go-zero](https://github.com/zeromicro/go-zero) | 通过 `HandlerRegister` bean 托管 go-zero `rest.Server` |
| `starter-goframe/http` | [gogf/gf](https://github.com/gogf/gf) | 托管 goframe `*ghttp.Server`(另有 `/tcp` 裸 TCP 子包) |
| `starter-kratos/http` | [go-kratos/kratos](https://github.com/go-kratos/kratos) | 托管 kratos HTTP 传输服务 |

## HTTP 客户端

把远程服务声明为接口,生成调用代码,再注入一个装配好的 `*http.Client`——服务
发现、负载均衡与韧性都已接入。

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-http-client` | Go `net/http` + [`gs-http-gen`](../gs/gs-http-gen) | 声明式 HTTP 客户端(对标 OpenFeign / `@HttpExchange`):在同一个 `*http.Client` 背后接入发现 + 负载均衡 + 韧性 + 链路追踪透传 |

## RPC 框架

注册服务后由 starter 负责监听/服务构建、生命周期与优雅关闭。

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-grpc` | [google.golang.org/grpc](https://pkg.go.dev/google.golang.org/grpc) | 轻量 gRPC 服务封装 |
| `starter-kitex` | [cloudwego/kitex](https://github.com/cloudwego/kitex) | 服务封装，可选 etcd 注册 |
| `starter-thrift` | [Apache Thrift](https://thrift.apache.org/) | 基于 `TSimpleServer` 封装 `TProcessor` bean |
| `starter-trpc` | [trpc-group/trpc-go](https://github.com/trpc-group/trpc-go) | 通过属性配置服务（不使用 `trpc_go.yaml`），直连方式 |
| `starter-dubbo` | [dubbo-go/v3](https://pkg.go.dev/dubbo.apache.org/dubbo-go/v3) | 完整服务端 + 客户端，支持注册中心服务发现 |
| `starter-go-zero/zrpc` | [zeromicro/go-zero](https://github.com/zeromicro/go-zero) | 通过 `ServiceRegister` bean 托管 zrpc gRPC 服务，可选 etcd 注册 |
| `starter-goframe/grpc` | [gogf/gf](https://github.com/gogf/gf) | goframe gRPC 服务（`grpcx.GrpcServer`） |
| `starter-kratos/grpc` | [go-kratos/kratos](https://github.com/go-kratos/kratos) | kratos gRPC 传输服务，支持 etcd 注册 |

## WebSocket

提供已配置好的 upgrader / accept-options bean；路由挂载在应用已有的 HTTP 服务上
（自身不占用端口）。

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-websocket` | [gorilla/websocket](https://github.com/gorilla/websocket) | 提供 `*websocket.Upgrader` |
| `starter-websocket-coder` | [coder/websocket](https://github.com/coder/websocket) | 提供 `*websocket.AcceptOptions` |
| `starter-goframe/ws` | [gogf/gf](https://github.com/gogf/gf) | 基于 `*ghttp.Server` 的 WebSocket 升级 |
| `starter-kratos/ws` | [tx7do/kratos-transport](https://github.com/tx7do/kratos-transport) | kratos WebSocket 传输服务 |

## 数据库

| Starter | 底层库 | 领域 |
| --- | --- | --- |
| `starter-gorm-mysql` | [gorm](https://gorm.io/) | MySQL 关系型数据库 |
| `starter-gorm-postgres` | [gorm](https://gorm.io/) | PostgreSQL 关系型数据库 |
| `starter-gorm-sqlserver` | [gorm](https://gorm.io/) | Microsoft SQL Server 关系型数据库 |
| `starter-gorm-clickhouse` | [gorm](https://gorm.io/) | ClickHouse OLAP 列式数据库 |
| `starter-mongodb` | [mongo-driver/v2](https://go.mongodb.org/mongo-driver/v2) | MongoDB 文档数据库 |
| `starter-neo4j` | [neo4j-go-driver](https://github.com/neo4j/neo4j-go-driver) | Neo4j 图数据库 |
| `starter-elasticsearch` | [go-elasticsearch](https://github.com/elastic/go-elasticsearch) | Elasticsearch 搜索引擎 |
| `starter-repository-gorm` | [gorm](https://gorm.io/) | 基于任意 gorm `*gorm.DB` 的通用 `repository.Repository[T,ID]`(CRUD + 分页 + 审计) |

## 缓存

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-go-redis` | [go-redis](https://github.com/redis/go-redis) | Redis 客户端 |
| `starter-redigo` | [redigo](https://github.com/gomodule/redigo) | Redis 客户端（另一驱动实现） |
| `starter-memcached` | [gomemcache](https://github.com/bradfitz/gomemcache) | Memcached 客户端 |
| `starter-bigcache` | [BigCache](https://github.com/allegro/bigcache) | 进程内、GC 友好的内存缓存 |

## 消息队列

| Starter | 底层库 | 领域 |
| --- | --- | --- |
| `starter-kafka` | [twmb/franz-go](https://github.com/twmb/franz-go) | Kafka |
| `starter-kafka-sarama` | [IBM/sarama](https://github.com/IBM/sarama) | Kafka（另一驱动实现，共用 `spring.kafka` 前缀） |
| `starter-pulsar` | [apache/pulsar-client-go](https://github.com/apache/pulsar-client-go) | Apache Pulsar |
| `starter-rabbitmq` | [amqp091-go](https://github.com/rabbitmq/amqp091-go) | RabbitMQ |
| `starter-nats` | [nats.go](https://github.com/nats-io/nats.go) | NATS 核心消息 + JetStream（纯 Go） |
| `starter-mqtt` | [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) | MQTT |

## 安全 / 授权

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-casbin` | [Casbin](https://casbin.org) | 访问控制（RBAC/ABAC/ACL），enforcer 以 bean 形式注册 |
| `starter-oauth2-client` | [golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2) | OAuth2 client-credentials `*http.Client`，自动刷新令牌 |

## HTTP 中间件

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-lua-filter` | [gopher-lua](https://github.com/yuin/gopher-lua) | 在 `net/http` 层用 Lua 编写可编程的 HTTP 请求过滤器 |

## 并发

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-ants` | [ants](https://github.com/panjf2000/ants) | 进程内、资源受限的 goroutine 协程池 |

## 邮件

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-mail` | [wneessen/go-mail](https://github.com/wneessen/go-mail) | SMTP 发信（HTML/附件/多收件人）；只发信，不含模板引擎 |

## 可观测 / 诊断

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-otel` | [OpenTelemetry](https://opentelemetry.io/) | 统一可观测核心，构建共享的 Tracer/Meter Provider 并注册为 OTel 全局对象 |
| `starter-pprof` | Go `net/http/pprof` | 独立 HTTP 服务，暴露运行时性能剖析 |
