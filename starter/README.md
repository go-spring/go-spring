# Go-Spring Starters

[English](README.md) | [中文](README_CN.md)

This directory holds the official Go-Spring starters — integration modules that
wire third-party services and frameworks into the Go-Spring IoC container and
server lifecycle. Each starter is its own Go module. Below is a domain-based
overview to help you find the right one.

## Web / HTTP Frameworks

Serve an application-provided web engine through the Go-Spring server lifecycle.

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-gin` | [gin-gonic/gin](https://github.com/gin-gonic/gin) | Serves a `*gin.Engine` bean |
| `starter-echo` | [labstack/echo](https://github.com/labstack/echo) | Serves a `*echo.Echo` bean |
| `starter-hertz` | [CloudWeGo Hertz](https://github.com/cloudwego/hertz) | Serves a Hertz HTTP server |
| `starter-go-zero/rest` | [zeromicro/go-zero](https://github.com/zeromicro/go-zero) | Serves a go-zero `rest.Server` via a `HandlerRegister` bean |

## RPC Frameworks

Register a service and let the starter handle listener/server setup, lifecycle,
and graceful shutdown.

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-grpc` | [google.golang.org/grpc](https://pkg.go.dev/google.golang.org/grpc) | Lightweight gRPC server wrapper |
| `starter-kitex` | [cloudwego/kitex](https://github.com/cloudwego/kitex) | Server wrapper with optional etcd registration |
| `starter-thrift` | [Apache Thrift](https://thrift.apache.org/) | `TSimpleServer` wrapper for a `TProcessor` bean |
| `starter-dubbo` | [dubbo-go/v3](https://pkg.go.dev/dubbo.apache.org/dubbo-go/v3) | Full server + client with registry-based discovery |
| `starter-go-zero/zrpc` | [zeromicro/go-zero](https://github.com/zeromicro/go-zero) | zrpc gRPC server via a `ServiceRegister` bean, with optional etcd registration |

## WebSocket

Contribute a configured upgrader/accept-options bean; you mount routes onto an
HTTP server the application already runs (no own port).

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-websocket` | [gorilla/websocket](https://github.com/gorilla/websocket) | Contributes a `*websocket.Upgrader` |
| `starter-websocket-coder` | [coder/websocket](https://github.com/coder/websocket) | Contributes a `*websocket.AcceptOptions` |

## Databases

| Starter | Underlying library | Domain |
| --- | --- | --- |
| `starter-gorm-mysql` | [gorm](https://gorm.io/) | MySQL relational database |
| `starter-mongodb` | [mongo-driver/v2](https://go.mongodb.org/mongo-driver/v2) | MongoDB document database |
| `starter-neo4j` | [neo4j-go-driver](https://github.com/neo4j/neo4j-go-driver) | Neo4j graph database |
| `starter-elasticsearch` | [go-elasticsearch](https://github.com/elastic/go-elasticsearch) | Elasticsearch search engine |

## Cache (Redis)

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-go-redis` | [go-redis](https://github.com/redis/go-redis) | Redis client |
| `starter-redigo` | [redigo](https://github.com/gomodule/redigo) | Redis client (alternative driver) |

## Message Queues

| Starter | Underlying library | Domain |
| --- | --- | --- |
| `starter-kafka` | [twmb/franz-go](https://github.com/twmb/franz-go) | Kafka |
| `starter-pulsar` | [apache/pulsar-client-go](https://github.com/apache/pulsar-client-go) | Apache Pulsar |
| `starter-rabbitmq` | [amqp091-go](https://github.com/rabbitmq/amqp091-go) | RabbitMQ |

## Observability / Diagnostics

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-pprof` | Go `net/http/pprof` | Dedicated HTTP server exposing runtime profiles |
