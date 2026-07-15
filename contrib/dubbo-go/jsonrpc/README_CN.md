# dubbo-go — JSON-RPC(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个使用 **JSON-RPC 2.0 协议** —— HTTP/1.1 传输 + **JSON** 负载 —— 的
[Dubbo-go](https://dubbo.apache.org/zh-cn/overview/mannual/golang-sdk/)
`GreetService` 示例,并通过可复用的 **starter-dubbo** 模块以 Go-Spring 的方式装配:
由它提供 `gs.Server` 适配器,`gs.Run()` 驱动生命周期,provider 只是一个
`ServiceRegister` bean,协议与注册中心都取自 `conf/app.properties`,而不是
写死在 `main()` 里。

与 [`../triple`](../triple) 中的 Triple 版本不同:JSON-RPC 协议在 dubbo-go v3
里**没有 protobuf IDL,也没有代码生成器** —— 服务就是纯 Go 结构体,注册时通过
反射读取其导出方法签名,通信线上用 `encoding/json` 序列化。JSON-RPC 是最"底线"
的互通协议:任何能发 HTTP + JSON 的调用方(curl、浏览器、没有 Dubbo SDK 的其他
语言)都可以直接调用 provider,不需要专门的客户端库。

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
  │ → jsonrpc://<host>:20002                 │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀─ JSON-RPC (HTTP/1) ───│  consumer  │
│ gs.Run()   │      Greet(name)       │  一次性调用 │
│ :20002     │──────────────────────▶│  断言后退出 │
└────────────┘       echo name        └────────────┘
```

## 目录结构

```
contrib/dubbo-go/jsonrpc/
├── proto/greet.go           # 「IDL」:接口名与方法名常量
├── scripts/gen-code.sh      # no-op —— JSON-RPC 无 IDL codegen
├── provider/handler.go      # GreetProvider + StarterDubbo.ServiceRegister bean(server 由 starter-dubbo 提供)
├── provider/main.go         # gs.Run(),长驻并注册到 etcd
├── consumer/main.go         # 通过 etcd 发现 provider,调用并断言后退出
├── conf/app.properties      # provider 配置
├── docker-compose.yml       # 本地 etcd
└── scripts/smoke-test.sh    # 冒烟脚本:起 etcd+provider,跑 consumer,自动清理
```

## 如何生成

**什么都不用生成**。JSON-RPC 在 dubbo-go v3 里没有 protobuf/thrift IDL,也没有
代码生成器 —— 服务表面就是一份手写的 Go 文件(`proto/greet.go`),固定 Java 风格
接口名与方法名,再加一份匹配签名的手写 provider 结构体。
执行 `./scripts/gen-code.sh` 只会打印一行 "nothing to do",只是为了与 Triple 兄弟目录保持一致
的入口。

参数和返回值可以是任意 JSON 可序列化的 Go 类型,不像 Hessian2 需要注册 POJO 表。

## 与 Triple / Dubbo 的对比

| 关注点        | Triple(`../triple`)              | Dubbo/Hessian2(`../dubbo`)      | JSON-RPC(本模块)                                    |
| ------------- | -------------------------------- | ------------------------------- | ---------------------------------------------------- |
| 传输          | HTTP/2                            | 原始 TCP                        | HTTP/1.1                                             |
| 负载          | protobuf                          | Hessian2                        | JSON                                                 |
| IDL           | `.proto` + `protoc-gen-go-triple` | 无 —— 手写 Go 结构体            | 无 —— 手写 Go 结构体                                 |
| 跨语言可达    | 任何 gRPC/Triple 客户端           | Java Dubbo(原生)、Hessian2 运行时 | 任何能发 HTTP + JSON 的客户端(curl、浏览器等)      |
| 客户端调用    | 类型化桩(`svc.Greet(ctx, req)`) | 反射式(`conn.CallUnary(...)`)   | 反射式(`conn.CallUnary(...)`)                       |
| 何时选它      | 纯 Go 新业务                       | 与既有 Java Dubbo 服务互通       | 调试 / 裸 HTTP 客户端 / 找不到其他共同协议的兜底方案 |

## 配置

```properties
# 关闭内置 HTTP server,provider 只暴露 JSON-RPC 端点。
spring.http.server.enabled=false

# JSON-RPC 监听端口;${spring.dubbo.server.protocols} 下的 key 即 dubbo-go 协议名。
# JSON-RPC 在 20002(20000/20001 留给 Triple / Dubbo 兄弟,便于三者同机共存)。
spring.dubbo.server.protocols.jsonrpc.port=20002

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

## 已知上游问题:Go 1.26(`jsonv2` 实验)与 dubbo-go v3.3.1 不兼容

在启用了 `jsonv2` 实验的 Go 工具链下(`runtime.Version()` 带 `-X:jsonv2`
后缀,Go 1.26 默认如此),
`dubbo.apache.org/dubbo-go/v3/protocol/jsonrpc.(*serverRequest).UnmarshalJSON`
会陷入无限递归:该方法内部对自身接收者类型调用 `encoding/json.Unmarshal`,
v2 arshaler 把该方法当成覆盖后重新分派回 `UnmarshalJSON`,goroutine 栈爆掉。
provider 进程在第一个请求上崩溃,后续 consumer 拨号同一端口就会得到
`connect: connection refused`。

示例代码本身是对的——这是 dubbo-go v3.3.1 的 JSONRPC 协议实现里的上游缺陷。
可选方案:

- 用**没有** `jsonv2` 实验的 Go 工具链(Go 构建时 `GOEXPERIMENT=nojsonv2`,
  或直接 Go 1.25)。
- 等 dubbo-go 出一版不再从自身 `UnmarshalJSON` 里调 `json.Unmarshal` 的实现。

任意工具链下 `go build ./...` / `go vet ./...` 都能通过;`scripts/smoke-test.sh` 会持续
失败,直到上游修复到位。
