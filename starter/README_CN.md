# Go-Spring Starters

[English](README.md) | [中文](README_CN.md)

本目录收录 Go-Spring 官方 starter —— 将第三方服务与框架接入 Go-Spring IoC 容器
及服务生命周期的集成模块。每个 starter 都是独立的 Go module。下面按领域归类，
方便你快速定位所需的 starter。

所有 starter 共同遵循的设计约束(形态、端口、driver 模式、多实例、fail-fast……）
见 [DESIGN_CN.md](DESIGN_CN.md)。

## 公司基线(聚合 / profile 类)

下面大多 starter 各接一个第三方能力。**聚合 / profile starter** 相反:它**组合**其它
starter,并通过它们的公开缝提供一整家组织的默认 —— 身份、wire/传播词表、错误词表、
标准 driver —— 让一个服务**空白导入一个模块 + 配一个前缀**即可再基线到公司约定。
`starter-luohua` 是参考实现(见 DESIGN §2.6)。

## Web / HTTP 框架

通过 Go-Spring 服务生命周期托管由应用提供的 Web 引擎。

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-gin` | [gin-gonic/gin](https://github.com/gin-gonic/gin) | 托管 `*gin.Engine` bean |
| `starter-echo` | [labstack/echo](https://github.com/labstack/echo) | 托管 `*echo.Echo` bean |
| `starter-http-server` | Go `net/http` | 框架内置 stdlib HTTP 服务器的安全中间件套件(`Authenticate` / `Authorize` / `CORS` / `CSRF`) |
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

## 配置中心

| Starter | 底层库 | 领域 |
| --- | --- | --- |
| `starter-config-apollo` | [apolloconfig/agollo/v4](https://github.com/apolloconfig/agollo) | Apollo 配置中心 |
| `starter-config-nacos` | [nacos-sdk-go/v2](https://github.com/nacos-group/nacos-sdk-go) | Nacos 配置中心 |
| `starter-config-consul` | [consul/api](https://github.com/hashicorp/consul) | Consul KV |
| `starter-config-etcd` | [etcd/client/v3](https://go.etcd.io/etcd) | etcd KV |
| `starter-config-vault` | [vault/api](https://github.com/hashicorp/vault) | Vault secret/配置 |
| `starter-config-k8s` | [client-go](https://github.com/kubernetes/client-go) | K8s ConfigMap/Secret |
| `starter-config-bus` | [nats.go](https://github.com/nats-io/nats.go) | 配置总线（多实例广播） |

## 服务发现 / 注册中心

把本实例注册进注册中心，和/或把服务名解析成实时端点。每个后端都配置在
`${spring.registry.<backend>.<name>}` 命名块下，客户端按 bean 名引用
（`discovery: consul.main`）。

| Starter | 底层库 | 领域 |
| --- | --- | --- |
| `starter-registry` | 仅 stdlib | 注册核心：唯一那个把本实例发布进每个已配置中心的 `gs.Server` |
| `starter-registry-consul` | [consul/api](https://github.com/hashicorp/consul) | Consul agent：注册（TTL 健康检查）+ 发现 |
| `starter-registry-etcd` | [etcd/client/v3](https://go.etcd.io/etcd) | etcd KV：注册（lease）+ 发现（watch） |
| `starter-registry-nacos` | [nacos-sdk-go/v2](https://github.com/nacos-group/nacos-sdk-go) | Nacos naming：注册 + 发现（push） |
| `starter-registry-zookeeper` | [go-zookeeper/zk](https://github.com/go-zookeeper/zk) | ZooKeeper：注册（临时节点）+ 发现 |
| `starter-registry-k8s` | [client-go](https://github.com/kubernetes/client-go) | K8s Service：**只做发现** —— 平台已经把每个 Pod 注册好了 |

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
| `starter-gorm-mysql` | [gorm](https://gorm.io/) | MySQL 关系型数据库（TiDB、OceanBase MySQL 模式同样适用 —— 二者均兼容 MySQL 线协议） |
| `starter-gorm-postgres` | [gorm](https://gorm.io/) | PostgreSQL 关系型数据库 |
| `starter-gorm-sqlserver` | [gorm](https://gorm.io/) | Microsoft SQL Server 关系型数据库 |
| `starter-gorm-clickhouse` | [gorm](https://gorm.io/) | ClickHouse OLAP 列式数据库 |
| `starter-mongodb` | [mongo-driver/v2](https://go.mongodb.org/mongo-driver/v2) | MongoDB 文档数据库 |
| `starter-neo4j` | [neo4j-go-driver](https://github.com/neo4j/neo4j-go-driver) | Neo4j 图数据库 |
| `starter-elasticsearch` | [go-elasticsearch](https://github.com/elastic/go-elasticsearch) | Elasticsearch 搜索引擎 |
| `starter-influxdb` | [influxdb-client-go/v2](https://github.com/influxdata/influxdb-client-go) | InfluxDB 2.x 时序数据库 |
| `starter-tdengine` | [driver-go/v3 (taosWS)](https://github.com/taosdata/driver-go) | TDengine 时序数据库（websocket，零 CGO） |
| `starter-cassandra` | [gocql](https://github.com/gocql/gocql) | Cassandra / ScyllaDB 宽表数据库 |

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
| `starter-rocketmq` | [rocketmq-client-go/v2](https://github.com/apache/rocketmq-client-go) | Apache RocketMQ 4.x/5.x（NameServer 协议）；自带 `messaging.Driver` |
| `starter-rabbitmq` | [amqp091-go](https://github.com/rabbitmq/amqp091-go) | RabbitMQ |
| `starter-nats` | [nats.go](https://github.com/nats-io/nats.go) | NATS 核心消息 + JetStream（纯 Go） |
| `starter-mqtt` | [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) | MQTT |

## 对象存储

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-s3` | [minio-go](https://github.com/minio/minio-go) | S3 协议 —— MinIO/AWS 原生，阿里云 OSS 与腾讯云 COS 走 S3 兼容端点（`bucket-lookup=path`） |

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

## 邮件 / 通知

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-mail` | [wneessen/go-mail](https://github.com/wneessen/go-mail) | SMTP 发信（HTML/附件/多收件人）；只发信，不含模板引擎 |
| `starter-migration-goose` | [goose](https://github.com/pressly/goose) | 启动期 goose schema 迁移，作用于任意 gorm `*gorm.DB`（只进不退、fail-fast） |
| `starter-webhook` | 仅标准库 | 聊天 webhook：generic + 钉钉/飞书/企微/Slack 载荷格式与 HMAC 加签，零第三方依赖 |

## 可观测 / 诊断

| Starter | 底层库 | 说明 |
| --- | --- | --- |
| `starter-otel` | [OpenTelemetry](https://opentelemetry.io/) | 统一可观测核心，构建共享的 Tracer/Meter Provider 并注册为 OTel 全局对象 |
| `starter-pprof` | Go `net/http/pprof` | 独立 HTTP 服务，暴露运行时性能剖析 |
