# kitex(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个 [Kitex](https://www.cloudwego.io/docs/kitex/) `EchoService` 示例:先用
`kitex` 脚手架生成,再改造成 Go-Spring 的启动与配置方式 —— 由 `gs.Run()`
驱动生命周期,handler 作为 IoC bean 注入,监听地址取自
`conf/app.properties`,而不是写死在 `main()` 里。

采用 Kitex 默认的 TTHeader/Thrift 传输,并通过
`github.com/kitex-contrib/registry-etcd` 接入 **etcd 注册中心**做真实的
**服务注册与发现**:provider 启动时把 `echo` 服务注册进 etcd;consumer
不知道 provider 的 host:port,而是从同一 etcd 解析出可用地址再发起调用。
这才是真正的 Kitex 微服务形态,而非早期的无注册中心直连。

这是一个**可运行示例**,并非可复用的 starter 模块。

## 拓扑

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ 服务名: echo                             │ 解析 provider 地址
  │ → <host>:8888                            │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── RPC ─────────│  consumer  │
│ gs.Run()   │      Echo(message)     │  一次性调用 │
│ :8888      │──────────────────────▶│  断言后退出 │
└────────────┘       echo message     └────────────┘
```

## 目录结构

```
contrib/kitex/thrift/
├── idl/echo.thrift          # Thrift IDL
├── idl/echo/...          # Kitex 生成代码(请勿手改)
├── idl/kitex_info.yaml      # 重新生成用的元数据
├── idl/gen-code.sh          # 从 IDL 重新生成 idl/echo/
├── provider/handler.go      # EchoServiceImpl,导出为 echo.EchoService bean
├── provider/server.go       # KitexServer 适配器(gs.Server)+ Config,配置 etcd registry
├── provider/main.go         # gs.Run(),长驻并注册到 etcd
├── consumer/main.go         # 通过 etcd 发现 provider,调用并断言后退出
├── conf/app.properties      # provider 配置
├── docker-compose.yml       # 本地 etcd
└── scripts/smoke-test.sh    # 冒烟脚本:起 etcd+provider,跑 consumer,自动清理
```

## 如何生成

```bash
# 工具(一次性)
go install github.com/cloudwego/thriftgo@latest
go install github.com/cloudwego/kitex/tool/cmd/kitex@latest

# 从 IDL 生成脚手架(或直接执行 ./idl/gen-code.sh)
kitex -module go-spring.org/kitex/thrift -service echo idl/echo.thrift
```

脚手架会产出 `idl/echo/`、一个空的 `handler.go`,以及直接调用 `svr.Run()`
的 `main.go`。`idl/echo/` 由 provider 与 consumer 共享。重新执行 `./idl/gen-code.sh`
只会再生成 `idl/echo/`,不会覆盖改造后的 provider/consumer 代码。

> 这是 **Thrift** 协议版本。基于 protobuf 的传输(KitexProtobuf 与 gRPC)
> 见同级示例 [`../protobuf`](../protobuf)。

## 改造:原生 Kitex → Go-Spring + 注册发现

| 关注点   | Kitex 脚手架                            | Go-Spring 版本                                                                        |
| -------- | --------------------------------------- | ------------------------------------------------------------------------------------- |
| 启动     | `main()` 中 `svr.Run()` 阻塞            | `KitexServer` 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop                           |
| handler  | 手动 `new(EchoServiceImpl)`             | `gs.Provide(&EchoServiceImpl{}).Export(gs.As[echo.EchoService]())`                    |
| 是否启用 | 总是开启                                | 通过 `gs.OnBean` 条件依赖 `echo.EchoService` bean                                     |
| 地址     | 写死的默认值                            | 取自 `conf/app.properties` 的 `${spring.kitex.server.addr}`                           |
| 服务注册 | 无(直连)                              | provider `server.WithRegistry(etcd.NewEtcdRegistry(...))` + `WithServerBasicInfo`     |
| 服务发现 | consumer `client.WithHostPorts(":8888")`| consumer `client.WithResolver(etcd.NewEtcdResolver(...))`,按服务名从 etcd 解析地址   |
| 关停     | 进程自持                                | 由 Go-Spring 协调优雅关停(SIGTERM → `Stop()`,注销 etcd 注册)                       |

`provider/server.go` 里的适配器是关键:Kitex 的 `server.Run()` 会绑定监听
端口、把 provider 注册进 etcd 后永久阻塞,因此将其放到一个仅在
`sig.TriggerAndWait()` 之后启动的 goroutine 中运行,`Run` 则阻塞在一个 done
channel 上,由 `Stop()` 关闭它,把控制权交回 Go-Spring 的关停流程。

consumer 侧只提供 etcd 地址,不提供 provider 地址:它把 provider 注册时使
用的服务名(`echo`)传给 Kitex,由 Kitex 在 etcd 中查到一个存活的 provider
并调用。

## 注册中心的选择

本示例统一用 **etcd** 便于与其他 contrib 示例横向对比。
[kitex-contrib](https://github.com/kitex-contrib) 组织下还提供了
**Nacos**、**Consul**、**ZooKeeper** 等适配:只需把
`registry-etcd` 换成对应的 `registry-nacos` / `registry-consul` /
`registry-zookeeper` 模块,并用其
`NewXxxRegistry` / `NewXxxResolver` 替换 `etcd.NewEtcdRegistry` /
`etcd.NewEtcdResolver` 即可。选用 Nacos 时还能通过其自带的 `:8848/nacos`
控制台直接查看已注册的服务列表。

## 配置

```properties
# 关闭内置 HTTP server,provider 只暴露 Kitex 端点。
spring.http.server.enabled=false

# Kitex 监听地址,经 ${spring.kitex.server} 前缀读取,默认 :8888。
spring.kitex.server.addr=:8888

# 注册到 etcd 的服务名,consumer 按同一名字解析。
spring.kitex.server.service.name=echo

# etcd 注册中心地址,与 docker-compose.yml 一致。
spring.kitex.server.registry.etcd=127.0.0.1:2379
```

## 可观测(日志 / trace / metric)

provider 已接入三支柱,全部在 `starter-kitex` 内部接线,仅由
`provider/conf/app.properties` 驱动——handler 只加了一行携带 ctx 的
`klog.CtxInfof`。kitex 不像 dubbo-go/go-zero 有一键 "SetUp",所以
`starter-kitex` 组合了它的原生 [kitex-contrib](https://github.com/kitex-contrib) 组件:

| 支柱   | 机制 | 后端 |
| ------ | ---- | ---- |
| Trace  | `obs-opentelemetry` 的 `tracing.NewServerSuite()` → OTLP/gRPC | Jaeger(`:16686`,collector `:4317`) |
| Metric | `monitor-prometheus` server tracer,进程自曝抓取端点 | Prometheus(`:9099`,抓取 provider `:9090`) |
| Log    | `klog` 由 `obs-opentelemetry` logrus 适配器承接(JSON + `trace_id`/`span_id`) | 文件 → Promtail → Loki(`:3100`) |

仅 **provider** 埋点,consumer 保持裸客户端。OTel meter 已关闭,指标只走
Prometheus(不重复采集)。

`docker-compose.yml` 会同时起 etcd 与 Jaeger、Prometheus、Loki、Promtail。
跑完 provider + consumer(或冒烟脚本)后逐一核对:

- **Trace** —— Jaeger UI <http://127.0.0.1:16686>,服务 `echo`,查看 `Echo` span。
- **Metric** —— Prometheus UI <http://127.0.0.1:9099>(如查询 `kitex_server_throughput`),或 `curl 127.0.0.1:9090/metrics`。
- **Log** —— `logs/provider.log` 为 JSON 且带 `trace_id`/`span_id`;经 Loki `127.0.0.1:3100` 查询。

## 运行

先起注册中心与可观测后端:

```bash
docker compose up -d      # 或 docker-compose up -d
```

终端 A —— 启动 provider(长驻,注册进 etcd):

```bash
go run ./provider
```

终端 B —— 启动 consumer(从 etcd 发现并调用):

```bash
go run ./consumer
```

consumer 预期输出:

```
Response from discovered provider: Hello, Kitex!
```

或一键冒烟(自动起 etcd + provider、跑 consumer、清理):

```bash
bash scripts/smoke-test.sh
```
