# kitex — protobuf(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个由 **protobuf** IDL 生成的 [Kitex](https://www.cloudwego.io/docs/kitex/)
`EchoService` 示例,并改造成 Go-Spring 的启动与配置方式:由 `gs.Run()` 驱动
生命周期,handler 是 IoC bean,绑定地址来自 `conf/app.properties`,而非硬编码
在 `main()` 里。

因为服务用 protobuf 定义,同一个 provider 在同一端口上同时对外提供**两种**
protobuf 传输:

- **KitexProtobuf** —— Kitex 自有的、承载于 TTHeader 之上的 protobuf 载荷(默认)。
- **gRPC** —— 承载于 HTTP/2 之上的 protobuf。

server 会嗅探每个进来的连接并据此分发,因此 provider 侧无需任何协议相关配置;
consumer 通过 `client.WithTransportProtocol` 在每次调用时选择线上协议。本示例
与 [`../thrift`](../thrift) 的 Thrift 版本互为补充。

它接入 **etcd 注册中心**(通过 `github.com/kitex-contrib/registry-etcd`)实现
真正的**服务注册与发现**:provider 启动时把 `echo` 服务注册进 etcd;consumer
不知道 provider 的 host:port,而是从同一个 etcd 解析出一个存活地址。

这是一个可运行示例,**不是**可复用的 starter 模块。

## 拓扑

```
                ┌──────────────┐
    注册        │     etcd     │    发现
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ service: echo                            │ 解析 provider 地址
  │ → <host>:8888                            │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀── KitexProtobuf ──────│  consumer  │
│ gs.Run()   │◀────── gRPC ───────────│  一次性    │
│ :8888      │──────────────────────▶│ 断言后退出 │
└────────────┘       echo message     └────────────┘
```

## 目录结构

```
contrib/kitex/protobuf/
├── idl/echo.proto           # protobuf IDL
├── kitex_gen/echo/...       # Kitex 生成代码(请勿手改)
├── kitex_info.yaml          # 重新生成用的元数据
├── gen.sh                   # 从 IDL 重新生成 kitex_gen/
├── provider/handler.go      # EchoServiceImpl,导出为 echo.EchoService bean
├── provider/server.go       # KitexServer 适配器(gs.Server)+ Config,配置 etcd registry
├── provider/main.go         # gs.Run(),长驻并注册到 etcd
├── consumer/main.go         # 通过 etcd 发现,分别用两种传输各调一次,断言后退出
├── conf/app.properties      # provider 配置
├── docker-compose.yml       # 本地 etcd
└── check.sh                 # 冒烟脚本:起 etcd+provider,跑 consumer,自动清理
```

## 如何生成

```bash
# 工具(一次性)
go install github.com/cloudwego/kitex/tool/cmd/kitex@latest

# 从 IDL 生成脚手架(或直接执行 ./gen.sh)
kitex -module go-spring.org/kitex/protobuf -service echo idl/echo.proto
```

脚手架会产出 `kitex_gen/`、一个空的 `handler.go`,以及直接调用 `svr.Run()`
的 `main.go`。`kitex_gen/` 由 provider 与 consumer 共享,且天生同时支持
KitexProtobuf 与 gRPC —— 传输是运行时选择,而非生成期选择。重新执行
`./gen.sh` 只会再生成 `kitex_gen/`,不会覆盖改造后的 provider/consumer 代码。

## 选择传输协议

provider 与传输无关。consumer 侧:

```go
// KitexProtobuf(默认):不加传输选项。
cli, _ := echoservice.NewClient("echo", client.WithResolver(r))

// gRPC:加上 WithTransportProtocol。
cli, _ := echoservice.NewClient("echo",
    client.WithResolver(r),
    client.WithTransportProtocol(transport.GRPC))
```

`consumer/main.go` 会用两种传输分别调用一次发现到的 provider 并各自断言,
证明同一个 provider 同时讲两种协议。

## 配置

```properties
# 关闭内置 HTTP server;provider 只暴露 Kitex。
spring.http.server.enabled=false

# Kitex 绑定地址;通过 ${spring.kitex.server} 前缀读取,默认 :8888。
# 这一个端口同时服务 KitexProtobuf 与 gRPC。
spring.kitex.server.addr=:8888

# 注册进 etcd 的服务名;consumer 用同名解析。
spring.kitex.server.service.name=echo

# etcd 注册中心地址;与 docker-compose.yml 一致。
spring.kitex.server.registry.etcd=127.0.0.1:2379
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

终端 B —— 启动 consumer(通过 etcd 发现,并用两种传输各调用一次):

```bash
go run ./consumer
```

预期 consumer 输出:

```
[KitexProtobuf] response from discovered provider: Hello, Kitex!
[gRPC] response from discovered provider: Hello, Kitex!
```

或运行一次性冒烟脚本(起 etcd + provider,跑 consumer,然后全部清理):

```bash
bash check.sh
```
