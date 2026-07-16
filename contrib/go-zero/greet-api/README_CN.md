# go-zero — HTTP/REST(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个 [go-zero](https://go-zero.dev) `Greet` 示例:桩代码由 `.api` IDL 通过
`goctl api go` 生成,再改造成 Go-Spring 的启动与配置方式 —— 由 `gs.Run()`
驱动生命周期,业务逻辑作为 IoC bean 注入,监听地址取自 `conf/app.properties`,
而不是写死在 `main()` 里。

服务跑在 go-zero 的 **rest.Server**(HTTP 框架)上。由此带来两点差异,都是
有意为之:

- **不接 etcd,没有服务发现。** 与相邻的 `../greet-rpc` 不同,本示例没有
  注册环节。`rest.Server` 不内建服务发现能力 —— 注册中心相关能力只存在于
  go-zero 的 zRPC 层 —— 所以 consumer 直接连固定的 `host:port`。(这里的
  `docker-compose.yml` 只用于拉起可观测后端,并非注册中心;见
  [可观测](#可观测)。)
- **goctl 产物已展平且保持很薄。** 只有 `types/types.go` 与
  `handler/routes.go` 由 goctl 生成、通过 `scripts/gen-code.sh` 重新生成;
  其余部分(`handler/greethandler.go`,以及 `svc/` 下的 `ServiceContext`
  与 `GreetLogic` bean)是手写,让 Greet 业务逻辑得以参与 Go-Spring 的依赖注入。
  goctl 的 `internal/` 脚手架外壳被丢弃 —— 各包直接放在模块根目录下。

这是 go-zero 示例的 HTTP 半边;zRPC/gRPC 那一半 —— 同一个 `Greet` 服务,
但由 `greet.proto` 生成 —— 在旁边的 [`../greet-rpc`](../greet-rpc)。

这是一个**可运行示例**,并非可复用的 starter 模块。

## 拓扑

```
┌────────────┐        HTTP POST /greet      ┌────────────┐
│  provider  │◀─────────────────────────────│  consumer  │
│ gs.Run()   │  {"name":"Hello, go-zero!"}  │  一次性调用 │
│  :8888     │──────────────────────────────▶│  断言后退出 │
└────────────┘   {"greeting":"Hello, ..."}  └────────────┘
```

## 目录结构

```
contrib/go-zero/greet-api/
├── greet.api                          # go-zero API IDL
├── scripts/gen-code.sh                # 重新生成 goctl 所有的两份文件
├── types/types.go                     # goctl 生成的请求/响应结构(请勿手改)
├── handler/routes.go                  # goctl 生成的路由表(请勿手改)
├── handler/greethandler.go            # 手写,解析请求并调用 Logic bean
├── svc/servicecontext.go              # 手写,承载被注入的 Logic bean
├── svc/logic.go                       # 手写,GreetLogic IoC bean
├── provider/handler.go                # HandlerRegister bean,把路由与 Logic 绑起来
├── provider/server.go                 # RestServer 适配器(gs.Server)+ Config
├── provider/main.go                   # gs.Run(),长驻 HTTP server
├── consumer/main.go                   # HTTP POST,断言响应后退出
├── conf/app.properties                # provider 配置(含可观测)
├── docker-compose.yml                 # 可观测后端(prometheus/jaeger/loki/promtail)
├── docker/                            # prometheus.yml + promtail-config.yml
└── scripts/smoke-test.sh              # 冒烟脚本:起后端、构建并启动 provider、跑 consumer、断言、自动清理
```

## 如何生成

```bash
# 工具(一次性)
go install github.com/zeromicro/go-zero/tools/goctl@latest

# 从 IDL 生成整棵脚手架
goctl api go -api greet.api -dir <tmp>/gen --style gozero
```

`scripts/gen-code.sh` 会在一个父模块名为 `greetapi` 的临时工作区里执行同样的命令,并且**只**
把 `types/types.go` 与 `handler/routes.go` 挑出来放到项目里(并把 goctl 的
`internal/` 导入路径改写为模块根路径)。
goctl 生成的其余部分 —— `greet.go`(main)、`etc/greet.yaml`、`internal/config`、
`svc/servicecontext.go`、`svc/logic.go` 对应文件、
`handler/greethandler.go` —— 有意不采纳:生命周期与配置交由 Go-Spring
管理,业务逻辑住在一个 Go-Spring bean 里,而不是每次请求都 `NewGreetLogic()`。

## 改造:原生 go-zero → Go-Spring

| 关注点   | 原生 go-zero REST 脚手架                       | Go-Spring 版本                                                            |
| -------- | ---------------------------------------------- | -------------------------------------------------------------------------- |
| 启动     | `main()` 中 `server.Start()` 阻塞              | `RestServer` 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop                 |
| handler  | `handler.RegisterHandlers(server, svcCtx)`     | `gs.Provide(func(*GreetLogic) HandlerRegister { ... })`                    |
| 业务逻辑 | `logic.NewGreetLogic(ctx, svcCtx).Greet(req)`  | `GreetLogic` 是单例 IoC bean,ServiceContext 只是把它带进 handler         |
| 是否启用 | 总是开启                                       | 通过 `gs.OnBean` 条件依赖 `HandlerRegister` bean                          |
| 配置来源 | 写死在 `etc/greet.yaml`                        | 取自 `conf/app.properties` 的 `${spring.rest.server}`                     |
| 服务注册 | 无(REST 无发现能力)                          | 无 —— 需要注册中心请看相邻的 `greet-rpc`                                  |
| 关停     | 进程自持                                       | 由 Go-Spring 协调优雅关停(SIGTERM → `Stop()` → `rest.Server.Stop()`)     |

## 配置

```properties
# 关闭 Go-Spring 内置 HTTP server,provider 只暴露下面绑定的 go-zero rest.Server。
spring.http.server.enabled=false

# go-zero rest.Server 配置,经 ${spring.rest.server} 前缀读取。
spring.rest.server.name=greet
spring.rest.server.host=0.0.0.0
spring.rest.server.port=8888

# 可观测(仅 provider),详见下方「可观测」一节。
spring.rest.server.tracing.endpoint=127.0.0.1:4317
spring.rest.server.metrics.port=6060
spring.rest.server.log.mode=file
spring.rest.server.log.path=../logs
```

## 可观测

与 dubbo-go 示例(靠 `starter-dubbo` 手工接线 OTel 和 Prometheus)不同,
go-zero 原生自带三支柱。`rest.MustNewServer` 内部会调用
`service.ServiceConf.SetUp()`,启动 tracing agent、metrics DevServer 和 logx;
`rest.Server` 的中间件(Trace/Prometheus/Metrics/Log,默认全开)随后对每个
请求自动埋点。**我们不写任何 OpenTelemetry/Prometheus 代码** ——
`provider/server.go` 只是把 `conf/app.properties` 里的值填进 `ServiceConf`。

| 支柱   | go-zero 字段            | 后端(docker-compose.yml)              |
| ------ | ----------------------- | --------------------------------------- |
| Trace  | `ServiceConf.Telemetry` | Jaeger,OTLP/gRPC(:4317,UI 16686)      |
| Metric | `ServiceConf.DevServer` | Prometheus 抓 :6060/metrics(UI 9099)   |
| 日志   | `ServiceConf.Log`(logx)| JSON 文件 → Promtail → Loki(:3100)     |

只有 **provider** 埋点;consumer 是裸 `net/http` 客户端。go-zero 的 logx 会给
每行日志打上当前 trace/span,所以 Loki 里的日志能和 Jaeger 里的 span 关联。

拉起后端并跑带可观测的冒烟脚本:

```bash
docker compose up -d
bash scripts/smoke-test.sh   # 断言 /metrics 暴露了 http_server_requests_*
```

provider 运行且发生过一次请求后,手动验证:

- **Metric**:Prometheus UI http://127.0.0.1:9099 —— 查询 `http_server_requests_duration_ms_count`。
- **Trace**:Jaeger UI http://127.0.0.1:16686 —— 选择 service `greet`。
- **日志**:`curl -s 'http://127.0.0.1:3100/loki/api/v1/query_range?query=%7Bjob%3D%22greet-api%22%7D'`。

## 运行

终端 A —— 启动 provider(长驻):

```bash
go run ./provider
```

终端 B —— 启动 consumer(直接 HTTP 调用,自我断言):

```bash
go run ./consumer
```

consumer 预期输出:

```
Response from provider: Hello, go-zero!
```

或一键冒烟(启动 provider、跑 consumer、自动清理):

```bash
bash scripts/smoke-test.sh
```
