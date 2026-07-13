# dubbo-go — Dubbo/Hessian2(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个使用 **经典 Dubbo 协议** —— TCP 传输 + **Hessian2** 序列化 —— 的
[Dubbo-go](https://dubbo.apache.org/zh-cn/overview/mannual/golang-sdk/)
`GreetService` 示例,并通过可复用的 **starter-dubbo** 模块以 Go-Spring 的方式装配:
由它提供 `gs.Server` 适配器,`gs.Run()` 驱动生命周期,provider 只是一个
`ServiceRegister` bean,协议与注册中心都取自 `conf/app.properties`,而不是
写死在 `main()` 里。

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
├── provider/handler.go      # GreetProvider + StarterDubbo.ServiceRegister bean(server 由 starter-dubbo 提供)
├── provider/main.go         # gs.Run(),长驻并注册到 etcd
├── consumer/main.go         # 通过 etcd 发现 provider,调用并断言后退出(Go-Spring 风格:client bean + gs.Run())
├── conf/app.properties      # 共享配置(provider server + consumer 注册中心)
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
# 关闭内置 HTTP server:provider 只暴露 Dubbo 端点,consumer 无 server 运行。
spring.http.server.enabled=false

# 全局注册中心(跨角色共享)。map 驱动:key 是逻辑注册中心 ID;未给 `protocol`
# 时注册中心类型默认取 key。provider(server)与 consumer(client)都通过
# 「角色优先、全局兜底」从这里解析注册中心——两者都不设角色专属 registries map。
# 与 docker-compose.yml 一致。
spring.dubbo.registries.etcdv3.address=127.0.0.1:2379

# Provider 协议监听;${spring.dubbo.server.protocols} 下的 key 即 dubbo-go 协议名。
# 经典 Dubbo 在 20001(20000 留给 Triple 兄弟,便于两者同机共存)。
spring.dubbo.server.protocols.dubbo.port=20001
```

Dubbo **client** 由 starter-dubbo 作为默认 bean(`__default__`)提供,由
`${spring.dubbo.client}` 加全局 `${spring.dubbo.registries}` 构建;consumer 直接
autowire 它并 dial 服务。可在 `${spring.dubbo.client.instances}` 下声明多个命名
client(bean 名 = map key)。若要运行两个同类型注册中心,给各自一个不同的 map-key
ID 并显式设置 `protocol`,例如 `spring.dubbo.registries.bj.protocol=etcdv3` /
`...sh.protocol=etcdv3`。

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
