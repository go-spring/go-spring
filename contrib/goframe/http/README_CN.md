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
├── api/hello/                    # 生成的 API 定义(共享)
├── internal/config/config.go     # ${goframe} 绑定:address、name、registry.etcd
├── internal/controller/hello/    # 生成的 controller(共享)
├── internal/server/server.go     # GoFrameServer 适配器(gs.Server)+ etcd registry 接入
├── provider/main.go              # gs.Run(),长驻并注册到 etcd
├── consumer/main.go              # 通过 etcd 发现 provider,调用并断言后退出
├── conf/app.properties           # provider 配置
├── scripts/gen-code.sh           # 通过 `gf gen ctrl` 从 api/ 重新生成 internal/controller/
├── docker-compose.yml            # 本地 etcd
└── scripts/smoke-test.sh         # 冒烟脚本:起 etcd+provider,跑 consumer,自动清理
```

## 如何生成

```bash
# 工具(一次性)
go install github.com/gogf/gf/cmd/gf/v2@latest

# 生成单仓库脚手架(产出 api/、internal/、manifest/、resource/;拆分成 HTTP/gRPC
# 两个子模块时,module 名改为 go-spring.org/goframe/http)。
gf init goframe -g go-spring.org/goframe/http

# 从 api/ 重新生成 controller(或直接执行 ./scripts/gen-code.sh;hack/*.mk 中的 `make ctrl`
# 命令与其完全一致)。
gf gen ctrl
```

生成的 `api/`、`internal/controller/`、`internal/consts/` 均未被 Go-Spring
改造影响,只有*配置、启动和服务发现方式*发生了变化。

## 改造:原生 goframe → Go-Spring + 注册发现

| 关注点   | goframe 脚手架                                       | Go-Spring 版本                                                           |
| -------- | ---------------------------------------------------- | ------------------------------------------------------------------------ |
| 启动     | `cmd.Main.Run()` → `main()` 中 `s.Run()` 阻塞        | `GoFrameServer` 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop            |
| 建 server| `internal/cmd` 中直接调用 `g.Server()`               | `internal/server.NewGoFrameServer`,作为 `gs.Server` bean                |
| 路由     | `internal/cmd` 中直接 `s.Group(...)`                 | 放到 server bean 构造函数中完成                                          |
| 配置来源 | `manifest/config/config.yaml`,由 `g.Cfg()` 隐式加载 | `conf/app.properties`,通过 `value:"${...}"` 标签在 `${goframe}` 下绑定  |
| 服务注册 | 无(直连)                                          | provider 在 `g.Server(name)` 前调用 `gsvc.SetRegistry(etcd.New(addr))`  |
| 服务发现 | consumer 硬编码 `http://localhost:8000/hello`        | consumer `g.Client().Discovery(etcd.New(addr)).Get(ctx, "http://<name>/hello")` |
| 关停     | `s.Run()` 自持信号处理                               | 由 Go-Spring 协调优雅关停(SIGTERM → `Stop()`,注销 etcd 注册)          |

`internal/server/server.go` 中的适配器是关键。`ghttp.Server` 在构造时会读取
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
