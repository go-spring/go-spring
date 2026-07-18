# Go-Spring Starters

[English](README.md) | [中文](README_CN.md)

This directory holds the official Go-Spring starters — integration modules that
wire third-party services and frameworks into the Go-Spring IoC container and
server lifecycle. Each starter is its own Go module. Below is a domain-based
overview to help you find the right one.

For the shared design constraints every starter follows (archetypes, ports,
driver mode, multi-instance, fail-fast, ...), see [DESIGN.md](DESIGN.md).

## Web / HTTP Frameworks

Serve an application-provided web engine through the Go-Spring server lifecycle.

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-gin` | [gin-gonic/gin](https://github.com/gin-gonic/gin) | Serves a `*gin.Engine` bean |
| `starter-echo` | [labstack/echo](https://github.com/labstack/echo) | Serves a `*echo.Echo` bean |
| `starter-hertz` | [CloudWeGo Hertz](https://github.com/cloudwego/hertz) | Serves a Hertz HTTP server |
| `starter-go-zero/rest` | [zeromicro/go-zero](https://github.com/zeromicro/go-zero) | Serves a go-zero `rest.Server` via a `HandlerRegister` bean |
| `starter-goframe/http` | [gogf/gf](https://github.com/gogf/gf) | Serves a goframe `*ghttp.Server` (also ships a `/tcp` raw-TCP sub-package) |
| `starter-kratos/http` | [go-kratos/kratos](https://github.com/go-kratos/kratos) | Serves a kratos HTTP transport server |

## RPC Frameworks

Register a service and let the starter handle listener/server setup, lifecycle,
and graceful shutdown.

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-grpc` | [google.golang.org/grpc](https://pkg.go.dev/google.golang.org/grpc) | Lightweight gRPC server wrapper |
| `starter-kitex` | [cloudwego/kitex](https://github.com/cloudwego/kitex) | Server wrapper with optional etcd registration |
| `starter-thrift` | [Apache Thrift](https://thrift.apache.org/) | `TSimpleServer` wrapper for a `TProcessor` bean |
| `starter-trpc` | [trpc-group/trpc-go](https://github.com/trpc-group/trpc-go) | Server wrapper configured via properties (no `trpc_go.yaml`), direct-connect |
| `starter-dubbo` | [dubbo-go/v3](https://pkg.go.dev/dubbo.apache.org/dubbo-go/v3) | Full server + client with registry-based discovery |
| `starter-go-zero/zrpc` | [zeromicro/go-zero](https://github.com/zeromicro/go-zero) | zrpc gRPC server via a `ServiceRegister` bean, with optional etcd registration |
| `starter-goframe/grpc` | [gogf/gf](https://github.com/gogf/gf) | goframe gRPC server (`grpcx.GrpcServer`) |
| `starter-kratos/grpc` | [go-kratos/kratos](https://github.com/go-kratos/kratos) | kratos gRPC transport server, with etcd registration |

## WebSocket

Contribute a configured upgrader/accept-options bean; you mount routes onto an
HTTP server the application already runs (no own port).

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-websocket` | [gorilla/websocket](https://github.com/gorilla/websocket) | Contributes a `*websocket.Upgrader` |
| `starter-websocket-coder` | [coder/websocket](https://github.com/coder/websocket) | Contributes a `*websocket.AcceptOptions` |
| `starter-goframe/ws` | [gogf/gf](https://github.com/gogf/gf) | WebSocket upgrade served on a `*ghttp.Server` |
| `starter-kratos/ws` | [tx7do/kratos-transport](https://github.com/tx7do/kratos-transport) | kratos WebSocket transport server |

## Databases

| Starter | Underlying library | Domain |
| --- | --- | --- |
| `starter-gorm-mysql` | [gorm](https://gorm.io/) | MySQL relational database |
| `starter-gorm-postgres` | [gorm](https://gorm.io/) | PostgreSQL relational database |
| `starter-gorm-sqlserver` | [gorm](https://gorm.io/) | Microsoft SQL Server relational database |
| `starter-gorm-clickhouse` | [gorm](https://gorm.io/) | ClickHouse OLAP columnar database |
| `starter-mongodb` | [mongo-driver/v2](https://go.mongodb.org/mongo-driver/v2) | MongoDB document database |
| `starter-neo4j` | [neo4j-go-driver](https://github.com/neo4j/neo4j-go-driver) | Neo4j graph database |
| `starter-elasticsearch` | [go-elasticsearch](https://github.com/elastic/go-elasticsearch) | Elasticsearch search engine |

## Cache

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-go-redis` | [go-redis](https://github.com/redis/go-redis) | Redis client |
| `starter-redigo` | [redigo](https://github.com/gomodule/redigo) | Redis client (alternative driver) |
| `starter-memcached` | [gomemcache](https://github.com/bradfitz/gomemcache) | Memcached client |
| `starter-bigcache` | [BigCache](https://github.com/allegro/bigcache) | In-process, GC-friendly in-memory cache |

## Message Queues

| Starter | Underlying library | Domain |
| --- | --- | --- |
| `starter-kafka` | [twmb/franz-go](https://github.com/twmb/franz-go) | Kafka |
| `starter-kafka-sarama` | [IBM/sarama](https://github.com/IBM/sarama) | Kafka (alternative driver, shares the `spring.kafka` prefix) |
| `starter-pulsar` | [apache/pulsar-client-go](https://github.com/apache/pulsar-client-go) | Apache Pulsar |
| `starter-rabbitmq` | [amqp091-go](https://github.com/rabbitmq/amqp091-go) | RabbitMQ |
| `starter-nats` | [nats.go](https://github.com/nats-io/nats.go) | NATS core messaging + JetStream (pure Go) |
| `starter-mqtt` | [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) | MQTT |

## Security / Authorization

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-casbin` | [Casbin](https://casbin.org) | Access control (RBAC/ABAC/ACL); enforcer registered as a bean |
| `starter-oauth2-client` | [golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2) | OAuth2 client-credentials `*http.Client` with auto token refresh |

## HTTP Middleware

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-lua-filter` | [gopher-lua](https://github.com/yuin/gopher-lua) | Programmable HTTP request filters in Lua at the `net/http` layer |

## Concurrency

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-ants` | [ants](https://github.com/panjf2000/ants) | In-process, resource-bounded goroutine pool |

## Observability / Diagnostics

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-otel` | [OpenTelemetry](https://opentelemetry.io/) | Unified observability core; builds shared Tracer/Meter providers as OTel globals |
| `starter-pprof` | Go `net/http/pprof` | Dedicated HTTP server exposing runtime profiles |
