# dubbo-go — Dubbo/Hessian2(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个使用 **经典 Dubbo 协议** —— TCP 传输 + **Hessian2** 序列化 —— 的
[Dubbo-go](https://dubbo.apache.org/zh-cn/overview/mannual/golang-sdk/)
`GreetService` 示例,并改造成 Go-Spring 的启动与配置方式:由 `gs.Run()`
驱动生命周期,provider 作为 IoC bean 注入,监听端口取自 `conf/app.properties`,
而不是写死在 `main()` 里。

与 [`../triple`](../triple) 中的 Triple 版本不同:经典 Dubbo 协议在 dubbo-go v3
里**没有 protobuf IDL,也没有代码生成器** —— 服务就是纯 Go 结构体,注册时通过
反射读取其导出方法签名,通信线上用 Hessian2 序列化。因此,经典 Dubbo 协议是与
Java Dubbo 服务互通的最佳路径(Java Dubbo 原生就用这一协议);对纯 Go 的新业务
仍推荐 Triple。

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
  │ → dubbo://<host>:20001                   │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀── Dubbo (Hessian2) ───│  consumer  │
│ gs.Run()   │      Greet(name)       │  一次性调用 │
│ :20001     │──────────────────────▶│  断言后退出 │
└────────────┘       echo name        └────────────┘
```

## 目录结构

```
contrib/dubbo-go/dubbo/
├── proto/greet.go           # 「IDL」:接口名与方法名常量
├── gen.sh                   # no-op —— 经典 Dubbo 无 IDL codegen
├── provider/handler.go      # GreetProvider(Go 结构体,注册时反射解析方法)
├── provider/server.go       # DubboServer 适配器(gs.Server)+ Config,配置 etcd registry
├── provider/main.go         # gs.Run(),长驻并注册到 etcd
├── consumer/main.go         # 通过 etcd 发现 provider,调用并断言后退出
├── conf/app.properties      # provider 配置
├── docker-compose.yml       # 本地 etcd
└── check.sh                 # 冒烟脚本:起 etcd+provider,跑 consumer,自动清理
```

## 如何生成

**什么都不用生成**。经典 Dubbo/Hessian2 在 dubbo-go v3 里没有 protobuf/thrift
IDL,也没有代码生成器 —— 服务表面就是一份手写的 Go 文件(`proto/greet.go`),
固定 Java 风格接口名与方法名,再加一份匹配签名的手写 provider 结构体。
执行 `./gen.sh` 只会打印一行 “nothing to do”,只是为了与 Triple 兄弟目录
保持一致的入口。

如果你的服务用到非基本类型,需要通过 `hessian.RegisterPOJO(&MyStruct{})`
把类型注册进 Hessian2,让 Go↔Java 的类型映射在启动时就绪。本示例只用
`string`,不需要额外注册。

## 与 Triple 的对比

| 关注点        | Triple(`../triple`)              | Dubbo/Hessian2(本模块)                                |
| ------------- | -------------------------------- | ------------------------------------------------------ |
| 传输          | HTTP/2                            | 原始 TCP                                              |
| 负载          | protobuf                          | Hessian2                                               |
| IDL           | `.proto` + `protoc-gen-go-triple` | 无 —— 手写 Go 结构体                                   |
| 跨语言可达    | 任何 gRPC/Triple 客户端           | Java Dubbo(原生)、任何支持 Hessian2 的运行时         |
| 客户端调用    | 类型化桩(`svc.Greet(ctx, req)`) | 反射式(`conn.CallUnary(ctx, args, resp, "Greet")`)   |
| 何时选它      | 纯 Go 新业务                       | 与既有 Java Dubbo 服务互通                             |

## 配置

```properties
# 关闭内置 HTTP server,provider 只暴露 Dubbo 端点。
spring.http.server.enabled=false

# Dubbo 监听端口,经 ${spring.dubbo.server} 前缀读取,默认 20001
# (20000 留给 Triple 兄弟,便于两者同机共存)。
spring.dubbo.server.port=20001

# etcd 注册中心地址,与 docker-compose.yml 一致。
spring.dubbo.server.registry.etcd=127.0.0.1:2379
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
bash check.sh
```
