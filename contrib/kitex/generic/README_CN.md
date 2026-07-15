# kitex —— 泛化调用(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个 [Kitex](https://www.cloudwego.io/docs/kitex/) `EchoService` 示例,
**consumer 完全没有生成的桩代码**。它在运行时解析 Thrift IDL,并通过
Kitex 的 **JSON 泛化(generic)** 客户端发起调用:

```go
p, _   := generic.NewThriftFileProvider("idl/echo.thrift") // 运行时解析 IDL
g, _   := generic.JSONThriftGeneric(p)                     // JSON <-> Thrift 编解码器
cli, _ := genericclient.NewClient("echo-generic", g,
    client.WithResolver(etcd.NewEtcdResolver(...)))
resp, _ := cli.GenericCall(ctx, "Echo", `{"message":"hi"}`) // resp 是 JSON 字符串
```

provider 与 [`../thrift`](../thrift) 完全一致 —— 就是普通的、经代码生成
的 Kitex Thrift 服务端。二者在网络上流的仍然是同一份 TTHeader/Thrift
字节流,所以泛化客户端不需要服务端做任何"泛化"改造。

## 为什么单独立项

| 子项目                            | 客户端桩代码?                  | 传输协议                                                               |
| --------------------------------- | ------------------------------ | ---------------------------------------------------------------------- |
| [`../thrift`](../thrift)          | 有类型桩                       | TTHeader / Thrift                                                      |
| [`../protobuf`](../protobuf)      | 有类型桩                       | KitexProtobuf **与** gRPC 同端口,由客户端逐次选择                     |
| **本项目**                        | **无 —— 运行时解析 IDL**       | TTHeader / Thrift(与 `../thrift` 同)                                 |

前两个兄弟项目都是通过类型化桩来调用 Kitex。本项目展示的是相反的一面:
按方法名 + JSON 负载的动态调用,完全由运行时的 IDL 文件驱动。这才是这里
真正要展示的能力 —— 不是不同的传输,而是根本不同的**调用模式**。

真实场景:把 REST/JSON 代理到内部 Thrift 服务的 API 网关、需要在不重
新构建的情况下调用任意服务的运维/管理工具、集成测试脚手架、跨语言桥接
层。

这是一个**可运行示例**,并非可复用的 starter 模块。

## 拓扑

```
                ┌──────────────────────┐
   register     │         etcd         │   discover
  ┌────────────▶│         :2379        │◀───────────┐
  │             └──────────────────────┘            │
  │ 服务名: echo-generic                            │ 解析 provider 地址
  │ → <host>:8890                                   │
┌─┴──────────┐                              ┌───────┴─────────────────┐
│  provider  │◀──── TTHeader/Thrift ────────│  consumer               │
│ gs.Run()   │       (客户端做 JSON <->     │  generic.JSONThrift     │
│ :8890      │        Thrift 编解码)        │  GenericCall("Echo",    │
│ (类型化)   │                              │   `{"message":"hi"}`)   │
└────────────┘                              └─────────────────────────┘
```

## 目录结构

```
contrib/kitex/generic/
├── idl/echo.thrift          # Thrift IDL,由 CONSUMER 运行时解析
├── kitex_gen/echo/...       # Kitex 生成代码(仅 PROVIDER 使用)
├── kitex_info.yaml          # 重新生成用的元数据
├── scripts/gen-code.sh      # 从 IDL 重新生成 kitex_gen/
├── provider/handler.go      # EchoServiceImpl(与 ../thrift 一致)
├── provider/server.go       # KitexServer 适配器(gs.Server)+ Config
├── provider/main.go         # gs.Run(),长驻并注册到 etcd
├── consumer/main.go         # 不 import kitex_gen;JSON 泛化调用
├── conf/app.properties      # provider 配置(端口 :8890,服务名 `echo-generic`)
├── docker-compose.yml       # 本地 etcd(容器名独立)
└── scripts/smoke-test.sh    # 冒烟脚本:起 etcd+provider,跑 consumer,自动清理
```

IDL 文件由 consumer 以**相对路径**(`idl/echo.thrift`)加载,因此
`go run ./consumer` 与 scripts/smoke-test.sh 中启动的二进制都从模块根目录运行,以便
该路径能正确解析。

## 配置

```properties
# 关闭内置 HTTP server,provider 只暴露 Kitex 端点。
spring.http.server.enabled=false

# 监听 :8890(不是 thrift/protobuf 使用的 :8888),便于三个子项目并行运行。
spring.kitex.server.addr=:8890

# 注册到 etcd 的服务名,consumer 按同一名字解析。
# `echo-generic` 使其与兄弟子项目的 `echo` 在共享注册中心中区分开。
spring.kitex.server.service.name=echo-generic

# etcd 注册中心地址,与 docker-compose.yml 一致。
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

终端 B —— 启动 consumer(运行时解析 IDL,从 etcd 发现并泛化调用):

```bash
go run ./consumer
```

consumer 预期输出:

```
Raw JSON response from discovered provider: {"message":"Hello, Kitex!"}
Generic call round-trip OK: Hello, Kitex!
```

或一键冒烟(自动起 etcd + provider、跑 consumer、清理):

```bash
bash scripts/smoke-test.sh
```
