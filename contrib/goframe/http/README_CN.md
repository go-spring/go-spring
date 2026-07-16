# goframe — HTTP(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个 [GoFrame](https://goframe.org) `Hello` 示例:先用 `gf init` 生成脚手架,
再改造成 Go-Spring 的启动与配置方式 —— 由 `gs.Run()` 驱动生命周期,goframe 的
`*ghttp.Server` 作为 IoC bean 注入,监听地址取自 `conf/app.properties`,而不是
`manifest/config/config.yaml`。

在此基础上接入 **etcd 注册中心**做真实的**服务注册与发现**:provider 启动时把
`goframe.hello` 注册进 etcd;consumer 不知道 provider 的 host:port,而是通过
goframe `gclient` 的 discovery 中间件从同一 etcd 解析出可用地址再发起调用。
这体现了 goframe `gsvc` 层标榜的微服务治理能力,而非早期的直连示例。

这是 **HTTP** 协议版本。GoFrame 的 gRPC 走的是另一种 server(`grpcx.GrpcServer`
而非 `*ghttp.Server`)、另一条代码生成链(`protoc` 而非 `gf gen ctrl`),因此
两个协议被拆到两个平级子模块中。gRPC 版本见 [`../grpc`](../grpc)。

这是一个**可运行示例**,并非可复用的 starter 模块。

## 拓扑

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ goframe.hello                            │ 解析 provider 地址
  │ → http://<host>:8000                     │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── HTTP ────────│  consumer  │
│ gs.Run()   │      GET /hello        │  一次性调用 │
│ :8000      │──────────────────────▶│  断言后退出 │
└────────────┘     "Hello World!"     └────────────┘
```

## 目录结构

```
contrib/goframe/http/
├── provider/main.go              # gs.Run(),长驻并注册到 etcd
├── provider/server.go            # GoFrameServer 适配器(gs.Server)+ Config + etcd registry + 可观测接线
├── provider/handler.go           # 手写 HelloController(g.Meta 路由 + 响应),通过请求 ctx 打日志
├── consumer/main.go              # 通过 etcd 发现 provider,调用并断言后退出
├── conf/app.properties           # provider 配置
├── scripts/gen-code.sh           # 空操作:handler 手写,无 IDL 代码生成
├── docker-compose.yml            # 本地 etcd + 可观测后端(Prometheus/Jaeger/Loki/Promtail)
├── docker/prometheus.yml         # Prometheus 抓取配置(目标为宿主机 :8000/metrics)
├── docker/promtail-config.yml    # Promtail 配置(tail ./logs 推送到 Loki)
└── scripts/smoke-test.sh         # 冒烟脚本:起后端+provider,跑 consumer,断言三支柱,自动清理
```

## 如何生成

```bash
# 工具(一次性)
go install github.com/gogf/gf/cmd/gf/v2@latest

# 生成单仓库脚手架(拆分成 HTTP/gRPC 两个子模块时,module 名改为
# go-spring.org/goframe/http)。
gf init goframe -g go-spring.org/goframe/http
```

原始 `gf init` 脚手架(`api/`、`internal/`、`manifest/`、`resource/` 目录及
`gf gen ctrl` 生成的 controller)已被**移除**,改用与其它协议示例一致的扁平
`provider/{main,server,handler}.go` 布局。Hello handler 现在手写在
`provider/handler.go` 中;Go-Spring 改造的部分只有*配置、启动和服务发现方式*。

## 改造:原生 goframe → Go-Spring + 注册发现

| 关注点   | goframe 脚手架                                       | Go-Spring 版本                                                           |
| -------- | ---------------------------------------------------- | ------------------------------------------------------------------------ |
| 启动     | `cmd.Main.Run()` → `main()` 中 `s.Run()` 阻塞        | `GoFrameServer` 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop            |
| 建 server| `internal/cmd` 中直接调用 `g.Server()`               | `provider.NewGoFrameServer`,作为 `gs.Server` bean                       |
| 路由     | `internal/cmd` 中直接 `s.Group(...)`                 | 放到 server bean 构造函数中完成                                          |
| 配置来源 | `manifest/config/config.yaml`,由 `g.Cfg()` 隐式加载 | `conf/app.properties`,通过 `value:"${...}"` 标签在 `${goframe}` 下绑定  |
| 服务注册 | 无(直连)                                          | provider 在 `g.Server(name)` 前调用 `gsvc.SetRegistry(etcd.New(addr))`  |
| 服务发现 | consumer 硬编码 `http://localhost:8000/hello`        | consumer `g.Client().Discovery(etcd.New(addr)).Get(ctx, "http://<name>/hello")` |
| 关停     | `s.Run()` 自持信号处理                               | 由 Go-Spring 协调优雅关停(SIGTERM → `Stop()`,注销 etcd 注册)          |

`provider/server.go` 中的适配器是关键。`ghttp.Server` 在构造时会读取
一次 `gsvc.GetRegistry()`(见 `ghttp_server.go` 中的 `registrar: gsvc.GetRegistry()`),
因此构造函数在调用 `g.Server(name)` 之前先设置好 etcd registry。`s.Start()`
非阻塞,所以 `Run` 阻塞在一个 done channel 上,由 `Stop()` 关闭以把控制权交回
Go-Spring 的关停流程,后者进一步触发 `s.Shutdown()` 与 etcd 注销。

consumer 侧不知道 provider 的 host:port:它把同一 etcd 地址加上服务名
(`goframe.hello`,与 `conf/app.properties` 中的 `goframe.name` 一致)传给
`gclient`,其内置的 `Discovery` 中间件会把 `r.URL.Host` 视作服务名并从 etcd
中解析出真实地址。

## 注册中心的选择

本示例统一使用 **etcd**,便于与其他 contrib 示例横向对比。
`github.com/gogf/gf/contrib/registry/*` 同样提供 **Nacos**、**ZooKeeper** 与
**Polaris** 适配器,均满足同一个 `gsvc.Registry` 接口:把
`github.com/gogf/gf/contrib/registry/etcd/v2` 换成
`.../registry/nacos/v2` / `.../registry/zookeeper/v2` / `.../registry/polaris/v2`,
再相应地改 `goframe.registry.etcd` 即可。选用 Nacos 时还能通过其自带的
`:8848/nacos` 控制台直接查看已注册服务。

## 配置

```properties
# 关闭 Go-Spring 内置 HTTP server,由 goframe *ghttp.Server 独占端口。
spring.http.server.enabled=false

# goframe *ghttp.Server 监听地址。
goframe.address=:8000

# provider 注册使用的服务名;consumer 从 etcd 中按此名解析地址。
goframe.name=goframe.hello

# etcd 注册中心地址,与 docker-compose.yml 一致。
goframe.registry.etcd=127.0.0.1:2379

# 可观测(详见下文):tracing → Jaeger(OTLP/HTTP),metrics → HTTP 端口上的
# Prometheus 抓取端点,logging → glog JSON 写入 logs/。
goframe.tracing.endpoint=127.0.0.1:4318
goframe.tracing.path=/v1/traces
goframe.metrics.path=/metrics
goframe.log.dir=../logs
goframe.log.file=provider.log
```

## 运行

先起注册中心:

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
Response from discovered provider: Hello World!
```

或一键冒烟(自动起 etcd + provider、跑 consumer、清理):

```bash
bash scripts/smoke-test.sh
```

## 可观测

与 dubbo-go 示例(可观测内建于 `starter-dubbo`、由 `spring.dubbo.*` 驱动)不同,
goframe **自带一套 OpenTelemetry 集成**,因此三大支柱全部使用 goframe 原生包 ——
在 `provider/server.go` 中接线,而非套用 go-spring 的 `log`/`metric`。只对
**provider** 埋点,consumer 保持裸客户端,与 dubbo-go / go-zero 示例一致。后端栈
(Prometheus/Jaeger/Loki/Promtail)沿用其它 contrib 示例共用的那一套,定义在
`docker-compose.yml`。

| 信号   | 产生方                                                 | 后端            | 查看位置                                              |
| ------ | ------------------------------------------------------ | --------------- | ----------------------------------------------------- |
| 指标   | `contrib/metric/otelmetric` → Prometheus exporter      | Prometheus      | UI http://127.0.0.1:9099(查询 `target_info`)         |
| 链路   | `contrib/trace/otlphttp` → OTLP/HTTP `127.0.0.1:4318` | Jaeger          | UI http://127.0.0.1:16686(服务 `goframe.hello`)      |
| 日志   | `glog` JSON handler → `logs/` 下文件                   | Loki(Promtail) | Loki HTTP API,端口 `:3100`                           |

### 工作原理

provider 跑在**宿主机**上(`scripts/smoke-test.sh` 编译并运行),后端都跑在
**容器**里(`docker-compose.yml`)。三支柱在 `NewGoFrameServer` →
`initObservability` 里一次性接好:

- **链路 —— push。** `otlphttp.Init(name, endpoint, "/v1/traces")` 设置全局
  tracer provider 与 OTLP/HTTP exporter,并返回 shutdown 函数(在 `Stop()` 中
  flush)。provider 设好后,**ghttp 自动对每个请求埋点**,无需中间件。用 OTLP/HTTP
  (`:4318`)是因为 `otlphttp` 写死了 `WithInsecure()`,能干净地对接明文的 Jaeger
  all-in-one collector(与 dubbo-go 示例回避 OTLP/gRPC 是同一原因)。
- **指标 —— pull。** Prometheus(pull)exporter 喂给 `otelmetric` 的
  `MeterProvider`(`WithBuiltInMetrics()` 附带 Go runtime 指标)。端点由
  `otelmetric.PrometheusHandler` 提供,绑在 server **根部**
  (`s.BindHandler("/metrics", ...)`)而非 `Group("/")` 内 —— 组上的
  `MiddlewareHandlerResponse` 会把响应包成 goframe JSON 信封,破坏 Prometheus
  文本格式。goframe 在**同一 HTTP 端口**(`:8000`)上提供 `/metrics`;Prometheus
  抓取 `host.docker.internal:8000`(`docker/prometheus.yml`)。
- **日志 —— tail 后 push。** `glog` 内建 JSON handler(`glog.HandlerJson`)把每条
  事件写成一行结构化 JSON 到 `../logs/provider.log`。用请求 ctx 打的日志会被 glog
  **自动打上 trace-id**(见 `provider/handler.go`),从而与上面的 span 关联。该
  `logs/` 目录被 bind-mount 进 Promtail,由其 tail 后推送到 Loki `:3100`。

到后端为止都是 provider 本地、确定性的;数据是否最终可在
Prometheus/Jaeger/Loki 查到属于手动步骤。栈起好且至少调用一次后:

```bash
# 指标 —— exporter 接好后必定输出 target_info
curl -s http://127.0.0.1:8000/metrics | grep target_info

# 链路 —— 首次请求后服务出现在 Jaeger
open http://127.0.0.1:16686        # 服务:goframe.hello

# 日志 —— 每条 ctx 日志带非空 "TraceId"
grep '"TraceId"' logs/provider.log
```

> 若你的 shell 导出了 HTTP 代理,请先把 `127.0.0.1,localhost` 加进
> `no_proxy`/`NO_PROXY`,否则本地 `curl` 会走代理返回空。

