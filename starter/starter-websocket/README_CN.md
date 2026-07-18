# starter-websocket

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-websocket` 基于 [gorilla/websocket](https://github.com/gorilla/websocket)
向 Go-Spring 服务提供一个配置好的 `*websocket.Upgrader`。它**不拥有** HTTP 服务，
也不占用端口：WebSocket 连接本质上只是一次 HTTP `Upgrade`，因此你应把 WebSocket
路由挂载到应用已经运行的任意 HTTP 服务上（net/http、gin、echo、hertz …）。

## 安装

```bash
go get go-spring.org/starter-websocket
```

## 快速开始

### 1. 引入 `starter-websocket` 包

参见 [example.go](example/example.go) 文件。使用空导入即可——只需其 `init()` 注册
`*websocket.Upgrader` 提供者：

```go
import _ "go-spring.org/starter-websocket"
```

### 2. 调整 Upgrader（可选）

在项目的[配置文件](example/conf/app.properties)中添加配置。这里没有服务地址——
upgrader 挂载在既有 HTTP 服务上，端口与超时由该服务负责：

```properties
spring.websocket.handshakeTimeout=10s
spring.websocket.readBufferSize=1024
spring.websocket.writeBufferSize=1024
# 启用 permessage-deflate 压缩。
spring.websocket.enableCompression=false
# 与请求 Origin 头做匹配的来源白名单。留空则沿用 gorilla 默认的同源策略；
# 单独填写 "*" 表示接受任意来源。
spring.websocket.allowedOrigins=
```

若需更多定制（如 `CheckOrigin`、压缩等），可自行提供 `*websocket.Upgrader` Bean，
`OnMissingBean` 会让你的实现优先生效。

### 3. 将 WebSocket 路由挂到 HTTP 服务

在注册 HTTP 路由处注入 `*websocket.Upgrader` 并调用 `Upgrade`。示例通过提供自定义的
`*gs.HttpServeMux`，把路由挂到 gs 内置 HTTP 服务上：

```go
gs.Provide(func(c *Controller, upgrader *websocket.Upgrader) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
        conn, _ := upgrader.Upgrade(w, r, nil)
        c.Echo(conn)
    })
    return &gs.HttpServeMux{Handler: mux}
})
```

由于 WebSocket 是长连接，请关闭 HTTP 服务的读写超时，避免连接被中途掐断：

```properties
spring.http.server.readTimeout=0
spring.http.server.writeTimeout=0
spring.http.server.idleTimeout=0
```

## 核心功能

[example](example/example.go) 端到端演示了三个能力，`runTest` 客户端会逐一断言：

* **文本回显 (`/echo`)** —— 完成 WebSocket 升级后，通过 `conn.ReadMessage` /
  `conn.WriteMessage` 原样回写每一帧文本消息。
* **JSON 回显 (`/json`)** —— 使用 `conn.ReadJSON` / `conn.WriteJSON` 接收
  `{"name": "..."}` 并回复 `{"message": "Hi, ..."}`。
* **HTTP 中间件鉴权 (`/guard`, 同样保护 `/echo` 与 `/json`)** —— `requireApp`
  中间件包裹处理器，请求头缺少 `X-App: go-spring` 时直接返回 `403 Forbidden`，
  连 WebSocket 握手都不会开始。`runTest` 分别验证放行和拒绝两条路径。
