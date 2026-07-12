# goframe — WebSocket (Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个 [GoFrame](https://goframe.org) 的 WebSocket **echo** 服务，按 Go-Spring
风格启动：`gs.Run()` 驱动生命周期，goframe 的 `*ghttp.Server` 是一个 IoC bean，
监听地址来自 `conf/app.properties` 而不是 `manifest/config/config.yaml`。

与兄弟模块 [`../http`](../http) 一样，本示例也引入了 **etcd 注册中心** 完成真正
的**服务注册与发现**：Provider 启动时把 `goframe.websocket.echo` 注册进 etcd；
Consumer 完全不知道 Provider 的 host:port，而是通过 goframe 的 `gsvc` 从同一个
etcd 里查出一个存活的 endpoint，然后用 `gorilla/websocket` 拨号
`ws://<host>:<port>/echo`。

## 为什么 WebSocket 长在 `*ghttp.Server` 上

GoFrame 没有独立的 WebSocket 服务器类型。WebSocket 连接本质上是一次带
`Upgrade` 头的 HTTP `GET`，goframe 里这个升级在普通 `ghttp` handler 里一行搞定：

```go
s.BindHandler("/echo", func(r *ghttp.Request) {
    ws, err := r.WebSocket() // 底层 gorilla 升级
    if err != nil { return }
    defer ws.Close()
    for {
        t, data, err := ws.ReadMessage()
        if err != nil { return }
        _ = ws.WriteMessage(t, data)
    }
})
```

因此 `../http` 里所有 HTTP 形态的东西这里都成立——同一个 server bean、同一次
`gsvc.SetRegistry` 调用、同一条优雅停机路径——**唯一改动的就是 `/echo` handler**：
不再"写响应体"，而是"升级连接、原样回帧"。

本示例是可运行的样例，**不是**可复用的 starter 模块。

## 拓扑

```
                    ┌──────────────┐
   注册              │     etcd     │   发现 (gsvc.Search)
  ┌────────────────▶│  :2379       │◀────────────────┐
  │                 └──────────────┘                 │
  │ goframe.websocket.echo                           │ 解出 Provider host:port
  │ → ws://<host>:8002/echo                          │
┌─┴──────────┐                             ┌─────────┴──────┐
│  Provider  │◀────── WebSocket ───────────│    Consumer    │
│ gs.Run()   │       echo 帧               │    一次性      │
│ :8002/echo │────────────────────────────▶│ 发送+校验+退出 │
└────────────┘                             └────────────────┘
```

## 目录结构

```
contrib/goframe/websocket/
├── internal/config/config.go     # ${goframe.websocket} 绑定：address / name / registry.etcd
├── internal/server/server.go     # GoFrameServer 适配器（gs.Server）+ /echo 升级路由
├── provider/main.go              # gs.Run()；常驻，注册进 etcd
├── consumer/main.go              # gsvc.Search → gorilla-websocket 拨号，断言回显后退出
├── conf/app.properties           # Provider 配置
├── gen.sh                        # 有注释的空操作（WS/HTTP handler 手写，无代码生成）
├── docker-compose.yml            # 本地 etcd
└── check.sh                      # 冒烟测试：起 etcd+Provider，跑 Consumer，再拆掉
```

## 与两个兄弟协议的差异

| 关注点     | `../http`                                                                        | `../grpc`                                                       | 本模块（WebSocket）                                             |
| ---------- | -------------------------------------------------------------------------------- | --------------------------------------------------------------- | --------------------------------------------------------------- |
| 服务器     | `*ghttp.Server`（`g.Server(name)`）                                              | `grpcx.GrpcServer`（`grpcx.Server.New(cfg)`）                   | `*ghttp.Server`，同 http（升级发生在 handler 内）               |
| IDL/代码生成 | `api/*/v*/` + `gf gen ctrl`                                                    | `idl/echo.proto` + `protoc`                                     | 无——handler 手写，`gen.sh` 是空操作                             |
| 客户端 API | `g.Client().Discovery(reg).Get(ctx, "http://<name>/hello")`                      | `grpcx.Client.MustNewGrpcClientConn(<name>)`                    | `registry.Search(...)` → `gorilla/websocket.Dial(ws://host:port/echo)` |
| 为什么客户端差 | goframe gclient 的 discovery 中间件在底层改写 HTTP `URL.Host`             | grpcx 为 gRPC 注册了一个 `gsvc://` resolver builder             | goframe 没有 ws 感知的客户端，只能先解出 endpoint 再交给 gorilla|

Provider 端本质就是 http 兄弟换了 handler。真正的重点在 Consumer 端：
goframe 的 `gclient` 发现中间件只吃 HTTP，grpcx 的 resolver builder 只吃 gRPC，
剩下所有非 HTTP/非 gRPC 的协议——包括 WebSocket（以及 TCP、MQTT 之类）——都要走
通用路径：**用 `gsvc.Discovery.Search()` 解一个 endpoint，把它的 host:port 交给
对应协议的原生客户端**。这就是本示例展示的形态。

## 配置

```properties
# 禁用 Go-Spring 内置 HTTP 服务器；端口由 goframe 的 *ghttp.Server 拥有。
spring.http.server.enabled=false

# goframe *ghttp.Server 的绑定地址（/echo 路由在这里升级为 WS）。
goframe.websocket.address=:8002

# Provider 注册用的服务名；Consumer 也用这个名字从 etcd 里查。
goframe.websocket.name=goframe.websocket.echo

# etcd 注册中心地址；对齐 docker-compose.yml。
goframe.websocket.registry.etcd=127.0.0.1:2379
```

## 运行

先起注册中心：

```bash
docker compose up -d      # 或者 docker-compose up -d
```

终端 A—— 启动 Provider（常驻，注册进 etcd）：

```bash
go run ./provider
```

终端 B—— 启动 Consumer（从 etcd 发现并拨号 upgrade）：

```bash
go run ./consumer
```

预期输出：

```
Dialing discovered provider: ws://127.0.0.1:8002/echo
Response from discovered provider: Hello, GoFrame WebSocket!
```

或者跑一次性冒烟脚本（起 etcd+Provider，跑 Consumer，然后全部拆掉）：

```bash
bash check.sh
```
