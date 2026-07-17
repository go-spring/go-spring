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

server 生命周期、日志桥接和可选的指标已不再在这里手写:它们都放进了可复用的
[`starter-goframe/http`](../../../starter/starter-goframe) 模块。本示例只是导入
该 starter 并提供一个 `ServiceRegister` bean;链路追踪交给
[`starter-otel`](../../../starter/starter-otel)。

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
├── provider/main.go              # gs.Run() + 空导入 starter-otel;长驻并注册到 etcd
├── provider/handler.go           # 导入 starter-goframe/http 并提供其 ServiceRegister;手写 HelloController
├── consumer/main.go              # 通过 etcd 发现 provider,调用并断言后退出
├── conf/app.properties           # provider 配置(${spring.goframe.http.server} + ${spring.observability})
├── scripts/gen-code.sh           # 空操作:handler 手写,无 IDL 代码生成
├── docker-compose.yml            # 本地 etcd + 可观测后端(Prometheus/Jaeger/Loki/Promtail)
├── docker/prometheus.yml         # Prometheus 抓取配置(目标为宿主机 :8000/metrics)
├── docker/promtail-config.yml    # Promtail 配置(tail ./logs 推送到 Loki)
└── scripts/smoke-test.sh         # 冒烟脚本:起后端+provider,跑 consumer,断言三支柱,自动清理
```

`GoFrameServer` 适配器(`gs.Server` + `Config` + etcd registry + 指标)以及原先
放在 `provider/server.go`、`provider/logbridge.go` 里的 glog 桥接,现在都住进了
`starter-goframe/http`,所以这两个文件已从本示例中删除。

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
`provider/{main,handler}.go` 布局。Hello handler 现在手写在
`provider/handler.go` 中;Go-Spring 改造的部分只有*配置、启动和服务发现方式*。

## 改造:原生 goframe → Go-Spring + 注册发现

| 关注点   | goframe 脚手架                                       | Go-Spring 版本                                                           |
| -------- | ---------------------------------------------------- | ------------------------------------------------------------------------ |
| 启动     | `cmd.Main.Run()` → `main()` 中 `s.Run()` 阻塞        | starter-goframe/http 的 server 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop |
| 建 server| `internal/cmd` 中直接调用 `g.Server()`               | `starter-goframe/http` 的 `NewHTTPServer`,作为 `gs.Server` bean         |
| 路由     | `internal/cmd` 中直接 `s.Group(...)`                 | 由 app 的 `ServiceRegister` bean 提供,在 starter 的 server 构造函数中绑定 |
| 配置来源 | `manifest/config/config.yaml`,由 `g.Cfg()` 隐式加载 | `conf/app.properties`,通过 `value:"${...}"` 标签在 `${spring.goframe.http.server}` 下绑定 |
| 服务注册 | 无(直连)                                          | 当设置了 `registry.etcd` 时,starter 在 `g.Server(name)` 前调用 `gsvc.SetRegistry(etcd.New(addr))` |
| 服务发现 | consumer 硬编码 `http://localhost:8000/hello`        | consumer `g.Client().Discovery(etcd.New(addr)).Get(ctx, "http://<name>/hello")` |
| 关停     | `s.Run()` 自持信号处理                               | 由 Go-Spring 协调优雅关停(SIGTERM → `Stop()`,注销 etcd 注册)          |

适配器现在住在 `starter-goframe/http`,不在本示例里。`ghttp.Server` 在构造时会
读取一次 `gsvc.GetRegistry()`(见 `ghttp_server.go` 中的
`registrar: gsvc.GetRegistry()`),因此 starter 在调用 `g.Server(name)` 之前先
设置好 etcd registry。`s.Start()` 非阻塞,所以 `Run` 阻塞在一个 done channel 上,
由 `Stop()` 关闭以把控制权交回 Go-Spring 的关停流程,后者进一步触发
`s.Shutdown()` 与 etcd 注销。

consumer 侧不知道 provider 的 host:port:它把同一 etcd 地址加上服务名
(`goframe.hello`,与 `conf/app.properties` 中的 `spring.goframe.http.server.name`
一致)传给 `gclient`,其内置的 `Discovery` 中间件会把 `r.URL.Host` 视作服务名并
从 etcd 中解析出真实地址。

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
spring.goframe.http.server.address=:8000

# provider 注册使用的服务名;consumer 从 etcd 中按此名解析地址。
spring.goframe.http.server.name=goframe.hello

# etcd 注册中心地址,与 docker-compose.yml 一致。留空则为普通 server,客户端直连。
spring.goframe.http.server.registry.etcd=127.0.0.1:2379

# 指标:starter 在 HTTP 端口上提供 goframe 原生的 Prometheus(pull)端点。
# 链路交给 starter-otel(见下文)。
spring.goframe.http.server.metrics.enabled=true
spring.goframe.http.server.metrics.path=/metrics

# starter-otel:安装全局 OTel TracerProvider,ghttp 会自动基于它埋点。指标走上面
# 的 goframe 原生方案,所以 starter-otel 的 metrics 保持关闭。
spring.observability.service-name=goframe.hello
spring.observability.trace.exporter=otlp-http
spring.observability.trace.endpoint=127.0.0.1:4318
spring.observability.metrics.exporter=none
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

可观测被拆到两个 starter:**链路**走
[`starter-otel`](../../../starter/starter-otel)(安装全局 OTel `TracerProvider`,
goframe 的 `ghttp` 自动基于它埋点),而**指标**保持 goframe 原生,由
`starter-goframe/http` 在 `spring.goframe.http.server.metrics.enabled=true` 时提供。
**日志**经 starter 的 glog→go-spring 桥接汇入统一的 `log` 管道。只对 **provider**
埋点,consumer 保持裸客户端,与 dubbo-go / go-zero 示例一致。后端栈
(Prometheus/Jaeger/Loki/Promtail)沿用其它 contrib 示例共用的那一套,定义在
`docker-compose.yml`。

| 信号   | 产生方                                                       | 后端            | 查看位置                                              |
| ------ | ------------------------------------------------------------ | --------------- | ----------------------------------------------------- |
| 指标   | `starter-goframe/http` → goframe 原生 Prometheus exporter    | Prometheus      | UI http://127.0.0.1:9099(查询 `target_info`、`otel_scope_info`) |
| 链路   | `starter-otel` 全局 `TracerProvider` → OTLP/HTTP `127.0.0.1:4318` | Jaeger     | UI http://127.0.0.1:16686(服务 `goframe.hello`)      |
| 日志   | glog → `starter-goframe` 桥接 → go-spring `log` FileLogger(`logs/` 下) | Loki(Promtail) | Loki HTTP API,端口 `:3100`               |

### 工作原理

provider 跑在**宿主机**上(`scripts/smoke-test.sh` 编译并运行),后端都跑在
**容器**里(`docker-compose.yml`)。

- **链路 —— push,经 starter-otel。** 空导入 `starter-otel`(`provider/main.go`
  中的 blank import)会根据 `${spring.observability.trace}` 安装全局 OTel
  `TracerProvider` —— 这里是 `otlp-http` 到 Jaeger `:4318`。全局设好后,
  **ghttp 自动对每个请求埋点**,无需逐 server 接线,替代了旧 `provider/server.go`
  里内联的 `contrib/trace/otlphttp` 块。
- **指标 —— pull,goframe 原生。** `starter-goframe/http` 把一个 Prometheus(pull)
  exporter 喂给 goframe `otelmetric` 的 `MeterProvider`,并把
  `otelmetric.PrometheusHandler` 绑在 server **根部**(在响应包装 group 之外,
  这样输出保持合法的 Prometheus 文本)。goframe 在**同一 HTTP 端口**(`:8000`)上
  提供 `/metrics`;Prometheus 抓取 `host.docker.internal:8000`
  (`docker/prometheus.yml`)。这是与 starter-otel 的 metrics 相互独立的管道,因此
  `spring.observability.metrics.exporter` 留空为 `none`,避免出现第二个
  `MeterProvider`。
- **日志 —— 桥接后 tail。** starter 把 goframe 的 `glog` 接入 go-spring 的 `log`
  模块;根 `FileLogger`(JSONLayout,`conf/app.properties`)把每条事件写成一行
  结构化 JSON 到 `../logs/provider.log`。该 `logs/` 目录被 bind-mount 进 Promtail,
  由其 tail 后推送到 Loki `:3100`。

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
