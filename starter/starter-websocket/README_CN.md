# starter-websocket

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-websocket` 基于 [gorilla/websocket](https://github.com/gorilla/websocket)
提供了一个轻量的 WebSocket 服务实现，方便在 Go-Spring 服务中快速暴露 WebSocket 路由。

## 安装

```bash
go get go-spring.org/starter-websocket
```

## 快速开始

### 1. 引入 `starter-websocket` 包

参见 [example.go](example/example.go) 文件。

```go
import StarterWebsocket "go-spring.org/starter-websocket"
```

### 2. 配置 WebSocket 服务

在项目的[配置文件](example/conf/app.properties)中添加配置。starter 默认监听
`:9696`；同时关闭内置的 HTTP 服务，避免端口竞争：

```properties
spring.http.server.enabled=false
```

### 3. 注册 WebSocket 路由

通过提供 `StarterWebsocket.ServerRegister` Bean，starter 会把共享的
`*http.ServeMux` 与 `*websocket.Upgrader` 传入，供业务注册路由：

```go
gs.Provide(func(c *Controller) StarterWebsocket.ServerRegister {
    return func(mux *http.ServeMux, upgrader *websocket.Upgrader) {
        mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
            conn, _ := upgrader.Upgrade(w, r, nil)
            c.Echo(conn)
        })
    }
})
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
