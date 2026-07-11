# go-kratos(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个 [Kratos](https://go-kratos.dev/) `Greeter` 示例:从 `kratos` 工具链脚手架
生成的项目出发,再改造成 Go-Spring 的启动与配置方式——由 `gs.Run()` 驱动生命周期,
各层通过 Go-Spring IoC 容器装配(取代 `google/wire`),服务监听地址来自
`conf/app.properties`(取代 Kratos 的 YAML 配置)。

示例同时暴露脚手架生成的 **HTTP**(`:8000`)与 **gRPC**(`:9000`)两个 Greeter
端点,并接入 **etcd 注册中心**做真实的**服务注册与发现**:provider 启动时把
`kratos-greeter` 这个 kratos.App 注册进 etcd;consumer 不知道 provider 的
host:port,而是通过 kratos 的 `discovery:///` scheme 从同一 etcd 解析出可用地址
再发起调用。这体现的是 Kratos 标榜的微服务治理能力,而非早期的无注册中心直连。

这是一个可运行的示例,**不是**可复用的 starter 模块。

## 拓扑

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ kratos-greeter                           │ 解析 provider 地址
  │ → grpc://<host>:9000                     │
  │ → http://<host>:8000                     │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── RPC ─────────│  consumer  │
│ gs.Run()   │      SayHello(name)    │  一次性调用 │
│ :8000/:9000│──────────────────────▶│  断言后退出 │
└────────────┘       "Hello "+name    └────────────┘
```

## 目录结构

```
contrib/go-kratos/
├── api/helloworld/v1/          # protoc 生成的 gRPC + HTTP stub(请勿编辑)
├── internal/biz/               # 业务逻辑(GreeterUsecase + GreeterRepo 接口)
├── internal/data/              # 数据层(Data、greeterRepo)+ 共享的 kratos logger bean
├── internal/service/           # 服务层(GreeterService)
├── provider/handler.go         # ServiceRegister bean,把 GreeterService 绑到 HTTP+gRPC
├── provider/server.go          # KratosServer 适配器(gs.Server)+ Config,组合 kratos.App
│                               #   并注入 etcd Registrar
├── provider/main.go            # gs.Run(),长驻并注册到 etcd
├── consumer/main.go            # 通过 etcd 发现 provider,调用 SayHello 并断言后退出
├── conf/app.properties         # provider 配置
├── docker-compose.yml          # 本地 etcd
└── check.sh                    # 冒烟脚本:起 etcd+provider,跑 consumer,自动清理
```

## 如何生成

```bash
# 工具(一次性)
go install github.com/go-kratos/kratos/cmd/kratos/v2@latest

# 生成项目(会克隆 kratos-layout 模板)
kratos new go-kratos
```

脚手架会产出 `cmd/`(`wire` + `kratos.App` 启动)、`configs/config.yaml`、
`internal/conf/`(`conf.proto` 的 Bootstrap 消息)以及分层的 `internal/` 代码。
改造删除了 `cmd/`、`configs/`、`internal/conf/` 及 `wire` 文件,保留生成的
`api/` stub 原样不动,并按 provider + consumer 双进程模式重连其余部分。

## 改造对照:原生 Kratos → Go-Spring + 注册发现

| 关注点         | Kratos 脚手架                                            | Go-Spring 版本                                                                            |
| -------------- | -------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| 启动           | `kratos.New(...).Run()` 占有进程                         | `KratosServer` 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop                              |
| 依赖装配       | `google/wire` `ProviderSet` + 生成的 `wire_gen.go`       | 每层 `init()` + `gs.Provide`;`provider/main.go` 用空导入触发注册                          |
| handler        | `internal/server` 中 `v1.RegisterGreeterHTTPServer(...)` | `ServiceRegister` bean 把 `GreeterService` 同时绑定到 HTTP 与 gRPC 两个 transport         |
| 是否启用       | 总是开启                                                 | `KratosServer` 通过 `gs.OnBean` 条件依赖 `ServiceRegister` bean                            |
| 配置来源       | `configs/config.yaml` 扫描进 `conf.proto` `Bootstrap`    | `conf/app.properties`,经 `value:"${spring.kratos.http}"` / `${spring.kratos.grpc}` 绑定  |
| 服务注册       | 无(直连)                                               | provider `kratos.Registrar(etcd.New(clientv3.New(...)))` 注册进 etcd                       |
| 服务发现       | consumer `transgrpc.WithEndpoint("host:port")` 直连      | consumer `transgrpc.WithEndpoint("discovery:///<name>") + WithDiscovery(etcd.New(...))`   |
| 关停           | `kratos.App` 自己捕获 SIGTERM                            | 由 Go-Spring 协调优雅关停(SIGTERM → `Stop()` → `App.Stop()`,注销 etcd 注册)             |

`provider/server.go` 里的适配器是关键:Kratos 的服务注册发生在 `kratos.App` 层面
(而非单个 transport),因此把 `khttp.Server` 与 `kgrpc.Server` 都构建好后一并交给
`kratos.New(...)`,并注入 `kratos.Registrar(etcdRegistry)`。`App.Run` 会绑定监听、
把服务实例发布进 etcd 后永久阻塞,因此放在一个仅在 `sig.TriggerAndWait()` 之后
启动的 goroutine 中运行;`Run` 阻塞在 done channel 上,由 `Stop()` 关闭,再把控制权
交回 Go-Spring 的关停流程(由它调用 `App.Stop` 注销并停止各 transport)。

consumer 侧只提供 etcd 地址和服务名:通过 `transgrpc.WithDiscovery(r)` 装上
`discovery:///` scheme 后,kratos 会在 etcd 里查到一个存活的 provider 并用 gRPC
调用它。

## 注册中心的选择

本示例统一用 **etcd** 便于与其他 contrib 示例横向对比。Kratos contrib 同样支持
**Consul**、**Nacos**、**ZooKeeper**、**Polaris** 等:只需把 provider 的
`etcd.New(clientv3.New(...))` 与 consumer 的对应调用换成 `consul.New(...)` /
`nacos.New(...)` / `zookeeper.New(...)` / `polaris.New(...)`,并调整 client 配置
即可。选用 Nacos 时还能通过其自带的 `:8848/nacos` 控制台直接查看已注册的服务列表。

## 配置

```properties
# 关闭内置 HTTP server,provider 只暴露 kratos transport。
spring.http.server.enabled=false

# 应用名 —— kratos.App 注册到 etcd 的键。
spring.kratos.name=kratos-greeter

# Kratos HTTP transport,经 ${spring.kratos.http} 前缀绑定。
spring.kratos.http.addr=0.0.0.0:8000
spring.kratos.http.timeout=1s

# Kratos gRPC transport,经 ${spring.kratos.grpc} 前缀绑定。
spring.kratos.grpc.addr=0.0.0.0:9000
spring.kratos.grpc.timeout=1s

# etcd 注册中心地址,与 docker-compose.yml 一致。
spring.kratos.registry.etcd=127.0.0.1:2379
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
Response from discovered provider: Hello Kratos
```

或一键冒烟(自动起 etcd + provider、跑 consumer、清理):

```bash
bash check.sh
```
