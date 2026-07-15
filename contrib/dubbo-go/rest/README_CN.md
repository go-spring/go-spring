# dubbo-go — REST(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个使用 **REST 协议** —— HTTP/1.1 传输 + go-restful 逐方法(动词、路径、参数来源)路由
—— 的 [Dubbo-go](https://dubbo.apache.org/zh-cn/overview/mannual/golang-sdk/)
`GreetService` 示例,并通过可复用的 **starter-dubbo** 模块以 Go-Spring 的方式装配:
由它提供 `gs.Server` 适配器,`gs.Run()` 驱动生命周期,provider 只是一个
`ServiceRegister` bean,协议与注册中心都取自 `conf/app.properties`,而不是
写死在 `main()` 里。

与 [`../triple`](../triple) 的 Triple 版本不同:REST 没有 protobuf IDL,也没有
代码生成器;与 [`../dubbo`](../dubbo)/[`../jsonrpc`](../jsonrpc) 的兄弟示例也不同:
**REST 不能仅靠方法反射**驱动 —— dubbo-go 需要一份 `RestServiceConfig` 映射把每个
Go 方法钉到具体的 `(HTTP 动词、URL 路径、参数来源)` 三元组上,才能在 Serve 前完成
路由注册。provider 端在 `provider/handler.go` 里安装该映射,consumer 端在
`consumer/main.go` 里安装同名映射,两侧必须一致,且都必须在进程注册/拨号前就位。

它接入 **etcd 注册中心**做真实的**服务注册与发现**:provider 启动时把
`com.example.GreetService`(Java 风格的点分接口名)注册进 etcd;consumer 不知道
provider 的 host:port,而是从同一 etcd 解析出可用地址再发起调用。

这是一个**可运行示例**,并非可复用的 starter 模块。

## 拓扑

```
                ┌──────────────┐
    注册        │     etcd     │    发现
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ com.example.GreetService                 │ 解析 provider 地址
  │ → rest://<host>:20003                    │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀──── REST (HTTP/1) ────│  consumer  │
│ gs.Run()   │  GET /greet?name=...   │  一次性调用 │
│ :20003     │──────────────────────▶│  断言后退出 │
└────────────┘   echo name (JSON)     └────────────┘
```

## 目录结构

```
contrib/dubbo-go/rest/
├── proto/greet.go           # 「IDL」:接口名、方法名、HTTP 动词/路径/查询键常量
├── scripts/gen-code.sh      # no-op —— REST 无 IDL codegen
├── provider/handler.go      # GreetProvider(Go 结构体)+ RestServiceConfig + StarterDubbo.ServiceRegister bean(server 由 starter-dubbo 提供)
├── provider/main.go         # gs.Run(),长驻并注册到 etcd
├── consumer/main.go         # 注册 RestServiceConfig,通过 etcd 发现 provider,调用并断言后退出
├── conf/app.properties      # provider 配置
├── docker-compose.yml       # 本地 etcd
└── scripts/smoke-test.sh    # 冒烟脚本:起 etcd+provider,跑 consumer,自动清理
```

## 如何生成

**什么都不用生成**。REST 在 dubbo-go v3 里没有 protobuf/thrift IDL,也没有代码
生成器 —— 服务表面就是手写的 Go 文件(`proto/greet.go`)——固定 Java 风格接口名、
方法名、HTTP 动词/路径/查询键常量——再加一份匹配签名的手写 provider 结构体,以及
provider/consumer 各自手写的 `RestServiceConfig` 映射。执行 `./scripts/gen-code.sh` 只会打印
一行 "nothing to do",只是为了与 Triple 兄弟目录保持一致的入口。

## 与其他协议的对比

| 关注点        | Triple(`../triple`)              | Dubbo/Hessian2(`../dubbo`)      | JSON-RPC(`../jsonrpc`)             | REST(本模块)                                       |
| ------------- | -------------------------------- | ------------------------------- | ----------------------------------- | ---------------------------------------------------- |
| 传输          | HTTP/2                            | 原始 TCP                        | HTTP/1.1                            | HTTP/1.1                                             |
| 负载          | protobuf                          | Hessian2                        | JSON-RPC 2.0 信封                    | 纯 JSON,无信封                                       |
| URL 布局      | 协议固定                          | 协议固定                        | 协议固定(`POST /<interface>`)      | 每个方法自定义(动词 + 路径 + 参数来源)             |
| IDL           | `.proto` + `protoc-gen-go-triple` | 无 —— 手写 Go 结构体            | 无 —— 手写 Go 结构体                 | 无 —— Go 结构体 + 手写 RestServiceConfig 映射         |
| 客户端连接    | 类型化桩                          | 只需接口名                      | 只需接口名                          | 接口名 + 方法映射                                     |
| 跨语言可达    | 任何 gRPC/Triple 客户端           | Java Dubbo(原生)、Hessian2 运行时 | 任何能发 HTTP + JSON 的客户端       | 任何能发 HTTP 的客户端(curl、浏览器、网关等)         |
| 何时选它      | 纯 Go 新业务                       | 与既有 Java Dubbo 服务互通       | 调试 / 裸 HTTP / 兜底                | 对外 REST API、网关友好的端点                        |

## 配置

```properties
# 关闭内置 HTTP server,provider 只暴露 REST 端点。
spring.http.server.enabled=false

# REST 监听端口;${spring.dubbo.server.protocols} 下的 key 即 dubbo-go 协议名。
# REST 在 20003(20000/20001/20002 留给 Triple / Dubbo / JSON-RPC 兄弟,便于四者同机共存)。
spring.dubbo.server.protocols.rest.port=20003

# etcd 注册中心,只在 ${spring.dubbo.registries} 定义一次:key 是逻辑注册中心 ID
# (类型默认取 key)。角色通过 ${...registry-ids} 按 ID 引用;只有一个注册中心时
# 两个角色都不设。与 docker-compose.yml 一致。
spring.dubbo.registries.etcdv3.address=127.0.0.1:2379
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
Response from discovered provider: Hello, Dubbo-Go!
```

或一键冒烟(自动起 etcd + provider、跑 consumer、清理):

```bash
bash scripts/smoke-test.sh
```
