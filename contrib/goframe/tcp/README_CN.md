# goframe — 原生 TCP（Go-Spring 风格）

[English](README.md) | [中文](README_CN.md)

一个 [GoFrame](https://goframe.org) 的原生 TCP **按行 echo** 服务，按
Go-Spring 风格启动：`gs.Run()` 驱动生命周期，goframe 的 `*gtcp.Server` 是一个
IoC bean，监听地址来自 `conf/app.properties` 而不是
`manifest/config/config.yaml`。

与兄弟模块 [`../http`](../http)、[`../websocket`](../websocket) 一样，也接入了
**etcd 注册中心**，做真正的**服务注册与发现**。不同的是，gtcp **没有**内建
gsvc 集成：`internal/server/server.go` 里的适配器围绕 gtcp 生命周期**手写**了
`gsvc.Registrar.Register` / `Deregister` 调用。这就是本模块存在的意义——它是
一个"如何把 goframe 里非 HTTP 的传输接入同一个 etcd 注册中心"的完整样例。

## 为什么只有 gtcp 特殊

| Server 类型            | 是否自带 gsvc 集成？ | 注册怎么发生                                              |
| ---------------------- | -------------------- | --------------------------------------------------------- |
| `*ghttp.Server`        | 是                   | 在 `g.Server(name)` 时抓 `gsvc.GetRegistry()`，自动完成   |
| `grpcx.GrpcServer`     | 是                   | 在 `grpcx.Server.New(cfg)` 时抓 `gsvc.GetRegistry()`      |
| `*gtcp.Server`（本例） | **否**               | 适配器围绕 Run/Close 手写 `Register` / `Deregister`       |

Consumer 端也是对称的：goframe 没有一个理解 `gsvc://<name>` 的原生 TCP 客户端，
所以走的是和 WebSocket 兄弟一样的原语路径—— `gsvc.Discovery.Search()` 解出一个
endpoint，然后把 host:port 交给 `gtcp.NewNetConn`。

本示例是可运行的样例，**不是**可复用的 starter 模块。

## 拓扑

```
                    ┌──────────────┐
   注册              │     etcd     │   发现 (gsvc.Search)
  ┌────────────────▶│  :2379       │◀────────────────┐
  │                 └──────────────┘                 │
  │ goframe.tcp.echo                                 │ 解出 Provider host:port
  │ → tcp://127.0.0.1:8003                           │
┌─┴──────────┐                             ┌─────────┴──────┐
│  Provider  │◀───── 原生 TCP 帧 ──────────│    Consumer    │
│ gs.Run()   │       按行 echo             │    一次性      │
│ :8003      │────────────────────────────▶│ 发送+校验+退出 │
└────────────┘                             └────────────────┘
```

## 目录结构

```
contrib/goframe/tcp/
├── internal/config/config.go     # ${goframe.tcp} 绑定：address / advertise.* / name / registry.etcd
├── internal/server/server.go     # GoFrameTCPServer 适配器（gs.Server）+ 手动 gsvc Register/Deregister
├── provider/main.go              # gs.Run()；常驻，注册进 etcd
├── consumer/main.go              # gsvc.Search → gtcp.NewNetConn 拨号，断言回显后退出
├── conf/app.properties           # Provider 配置
├── gen.sh                        # 有注释的空操作（原生 TCP 无 IDL）
├── docker-compose.yml            # 本地 etcd
└── check.sh                      # 冒烟测试：起 etcd+Provider，跑 Consumer，再拆掉
```

## 适配器要保证的顺序

因为 gsvc 是手写的（不像 ghttp/grpcx 藏在 Start/Shutdown 里），两个顺序约束
被显式暴露在适配器里：

1. **先绑定，再注册。** 在 `gtcp.Server.Run()` 开始 listen 之前就写 etcd，
   会让消费者拨到一个尚未打开的端口，看到 "connection refused"。适配器把
   `Run()` 丢到 goroutine 之后短暂轮询 `Server.GetListenedPort()`，等它非 0
   再调用 `Register`。
2. **先注销，再关闭。** 停机时先 `Deregister` 再 `Close()`，避免一个"新
   Consumer 刚从 etcd 解出这个实例，我这边的 listener 却马上要关"的窗口。

`ghttp.Server` 和 `grpcx.GrpcServer` 把这两点藏在自己的 Start/Shutdown 里；
这里因为 gtcp 不管，所以在适配器里显式做。

## 广播的 Endpoint

gtcp 不会像 ghttp 的注册路径那样帮你探测公网 IP，所以是把显式的
`advertise.host` / `advertise.port` 写进 etcd。真实部署里这是 pod/宿主机 IP；
示例里默认 `127.0.0.1`，端口与绑定地址一致。

## 配置

```properties
# 禁用 Go-Spring 内置 HTTP 服务器；端口由 goframe 的 *gtcp.Server 拥有。
spring.http.server.enabled=false

# gtcp 绑定地址。
goframe.tcp.address=:8003

# Provider 广播进 etcd 的 host:port。
goframe.tcp.advertise.host=127.0.0.1
goframe.tcp.advertise.port=8003

# Provider 注册用的服务名；Consumer 也用这个名字从 etcd 里查。
goframe.tcp.name=goframe.tcp.echo

# etcd 注册中心地址；对齐 docker-compose.yml。
goframe.tcp.registry.etcd=127.0.0.1:2379
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

终端 B—— 启动 Consumer（从 etcd 发现并拨 TCP）：

```bash
go run ./consumer
```

预期输出：

```
Dialing discovered provider: 127.0.0.1:8003
Response from discovered provider: Hello, GoFrame TCP!
```

或者跑一次性冒烟脚本：

```bash
bash check.sh
```
