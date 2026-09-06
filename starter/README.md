# Go-Spring Starters

[English](README.md) | [中文](README_CN.md)

This directory holds the official Go-Spring starters — integration modules that
wire third-party services and frameworks into the Go-Spring IoC container and
server lifecycle. Each starter is its own Go module. Below is a domain-based
overview to help you find the right one.

For the shared design constraints every starter follows (archetypes, ports,
driver mode, multi-instance, fail-fast, ...), see [DESIGN.md](DESIGN.md).

## Company baseline (aggregator / profile)

Most starters below integrate one third-party capability. An **aggregator /
profile starter** does the opposite: it *composes* other starters and supplies a
whole organization's defaults through their public seams — its identity,
wire/propagation vocabulary, error catalog and standard drivers — so a service
re-bases onto company conventions by blank-importing one module and setting one
config prefix. `starter-luohua` is the reference (see DESIGN §2.6).

## Web / HTTP Frameworks

Serve an application-provided web engine through the Go-Spring server lifecycle.

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-gin` | [gin-gonic/gin](https://github.com/gin-gonic/gin) | Serves a `*gin.Engine` bean |
| `starter-echo` | [labstack/echo](https://github.com/labstack/echo) | Serves a `*echo.Echo` bean |
| `starter-http-server` | Go `net/http` | Security middleware kit (`Authenticate` / `Authorize` / `CORS` / `CSRF`) for the framework-built-in stdlib HTTP server |
| `starter-hertz` | [CloudWeGo Hertz](https://github.com/cloudwego/hertz) | Serves a Hertz HTTP server |
| `starter-go-zero/rest` | [zeromicro/go-zero](https://github.com/zeromicro/go-zero) | Serves a go-zero `rest.Server` via a `HandlerRegister` bean |
| `starter-goframe/http` | [gogf/gf](https://github.com/gogf/gf) | Serves a goframe `*ghttp.Server` (also ships a `/tcp` raw-TCP sub-package) |
| `starter-kratos/http` | [go-kratos/kratos](https://github.com/go-kratos/kratos) | Serves a kratos HTTP transport server |

## HTTP Client

Declare a remote service as an interface, generate the call sites, and inject an
assembled `*http.Client` with discovery, load balancing and resilience wired in.

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-http-client` | Go `net/http` + [`gs-http-gen`](../gs/gs-http-gen) | Declarative HTTP client (OpenFeign / `@HttpExchange` equivalent): discovery + load balancing + resilience + trace propagation behind one `*http.Client` |

## Config Providers

| Starter | Underlying library | Domain |
| --- | --- | --- |
| `starter-config-apollo` | [apolloconfig/agollo/v4](https://github.com/apolloconfig/agollo) | Apollo config center |
| `starter-config-nacos` | [nacos-sdk-go/v2](https://github.com/nacos-group/nacos-sdk-go) | Nacos config center |
| `starter-config-consul` | [consul/api](https://github.com/hashicorp/consul) | Consul KV |
| `starter-config-etcd` | [etcd/client/v3](https://go.etcd.io/etcd) | etcd KV |
| `starter-config-vault` | [vault/api](https://github.com/hashicorp/vault) | Vault secret/config |
| `starter-config-k8s` | [client-go](https://github.com/kubernetes/client-go) | K8s ConfigMap/Secret |
| `starter-config-bus` | [nats.go](https://github.com/nats-io/nats.go) | Config bus (multi-instance broadcast) |

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
| `starter-gorm-mysql` | [gorm](https://gorm.io/) | MySQL relational database (also TiDB and OceanBase MySQL mode — both speak the MySQL wire protocol) |
| `starter-gorm-postgres` | [gorm](https://gorm.io/) | PostgreSQL relational database |
| `starter-gorm-sqlserver` | [gorm](https://gorm.io/) | Microsoft SQL Server relational database |
| `starter-gorm-clickhouse` | [gorm](https://gorm.io/) | ClickHouse OLAP columnar database |
| `starter-mongodb` | [mongo-driver/v2](https://go.mongodb.org/mongo-driver/v2) | MongoDB document database |
| `starter-neo4j` | [neo4j-go-driver](https://github.com/neo4j/neo4j-go-driver) | Neo4j graph database |
| `starter-elasticsearch` | [go-elasticsearch](https://github.com/elastic/go-elasticsearch) | Elasticsearch search engine |
| `starter-gorm-sqlite` | [glebarez/sqlite](https://github.com/glebarez/sqlite) | SQLite (pure Go, in-process) via the gorm dialect family |
| `starter-milvus` | [milvus-sdk-go/v2](https://github.com/milvus-io/milvus-sdk-go) | Milvus vector database |
| `starter-influxdb` | [influxdb-client-go/v2](https://github.com/influxdata/influxdb-client-go) | InfluxDB 2.x time-series database |
| `starter-tdengine` | [driver-go/v3 (taosWS)](https://github.com/taosdata/driver-go) | TDengine time-series database (websocket, zero CGO) |
| `starter-cassandra` | [gocql](https://github.com/gocql/gocql) | Cassandra / ScyllaDB wide-column database |

## Cache

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-go-redis` | [go-redis](https://github.com/redis/go-redis) | Redis client |
| `starter-redigo` | [redigo](https://github.com/gomodule/redigo) | Redis client (alternative driver) |
| `starter-memcached` | [gomemcache](https://github.com/bradfitz/gomemcache) | Memcached client |
| `starter-bigcache` | [BigCache](https://github.com/allegro/bigcache) | In-process, GC-friendly in-memory cache |

## Object Storage

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-s3` | [minio-go](https://github.com/minio/minio-go) | S3 protocol — MinIO/AWS natively, Aliyun OSS & Tencent COS S3-compatible endpoints (`bucket-lookup=path`) |

## Message Queues

| Starter | Underlying library | Domain |
| --- | --- | --- |
| `starter-kafka` | [twmb/franz-go](https://github.com/twmb/franz-go) | Kafka |
| `starter-kafka-sarama` | [IBM/sarama](https://github.com/IBM/sarama) | Kafka (alternative driver, shares the `spring.kafka` prefix) |
| `starter-pulsar` | [apache/pulsar-client-go](https://github.com/apache/pulsar-client-go) | Apache Pulsar |
| `starter-rocketmq` | [rocketmq-client-go/v2](https://github.com/apache/rocketmq-client-go) | Apache RocketMQ 4.x/5.x via the NameServer protocol; ships a `messaging.Driver` |
| `starter-rabbitmq` | [amqp091-go](https://github.com/rabbitmq/amqp091-go) | RabbitMQ |
| `starter-nats` | [nats.go](https://github.com/nats-io/nats.go) | NATS core messaging + JetStream (pure Go) |
| `starter-mqtt` | [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) | MQTT |

## Task Queues / Scheduling

| Starter | Underlying library | Domain |
| --- | --- | --- |
| `starter-asynq` | [hibiken/asynq](https://github.com/hibiken/asynq) | Redis-backed task queue (producer + opt-in worker) |
| `starter-xxljob` | stdlib only | xxl-job executor (registry/run/kill/log protocol, hand-rolled) |

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

## Mail / Notification

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-mail` | [wneessen/go-mail](https://github.com/wneessen/go-mail) | SMTP mailer (HTML/attachments/multi-recipient); send-only, no template engine |
| `starter-migration-goose` | [goose](https://github.com/pressly/goose) | Goose schema migrations at startup over any gorm `*gorm.DB` (forward-only, fail-fast) |
| `starter-webhook` | stdlib only | Chat webhooks: generic + DingTalk/Feishu/WeCom/Slack payload formats, HMAC signing, zero dependencies |

## Observability / Diagnostics

| Starter | Underlying library | Notes |
| --- | --- | --- |
| `starter-otel` | [OpenTelemetry](https://opentelemetry.io/) | Unified observability core; builds shared Tracer/Meter providers as OTel globals |
| `starter-pprof` | Go `net/http/pprof` | Dedicated HTTP server exposing runtime profiles |
