# Consul 注册中心(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

通过 **Consul** 实现服务注册与发现,使用由 **protobuf** IDL 生成的
[Kitex](https://www.cloudwego.io/docs/kitex/) `EchoService`。provider 启动时把
`echo` 服务注册进 Consul;consumer 不去拨打写死的 `host:port`,而是从同一个
Consul 解析出存活的 provider 地址。

它是 [`..`](..) 下五个兄弟示例里的“异类”:另外四个都用 dubbo-go,但 dubbo-go
**没有 Consul 注册中心扩展**,因此 Consul 改用 Kitex 演示,借助
[`github.com/kitex-contrib/registry-consul`](https://github.com/kitex-contrib/registry-consul)。
注册中心总览见顶层 [README](../README_CN.md)。

由于服务用 protobuf 定义,一个 provider 在同一端口同时提供 **两种** protobuf 传输:
**KitexProtobuf**(Kitex 自有的 protobuf over TTHeader,默认)与 **gRPC**
(protobuf over HTTP/2)。server 会嗅探每个连接并相应分发;consumer 通过
`client.WithTransportProtocol` 按调用选择线协议。

## 目录结构

```
consul/
├── idl/echo.proto           # protobuf IDL
├── idl/echo/...             # Kitex 生成的代码(请勿编辑)
├── idl/kitex_info.yaml      # 重新生成用的元数据
├── idl/gen-code.sh          # 从 IDL 重新生成 idl/echo/
├── provider/handler.go      # EchoServiceImpl(业务逻辑)
├── provider/server.go       # KitexServer(gs.Server)—— 接线 Consul 注册中心
├── provider/main.go         # gs.Run();长期运行,注册进 Consul
├── provider/conf/app.properties  # provider 配置(绑定地址 + 服务名 + 注册中心)
├── consumer/main.go         # 经 Consul 发现、逐传输调用、断言、退出
├── consumer/conf/app.properties  # consumer 配置(注册中心 + 服务名)
├── docker-compose.yml       # 本地 Consul(agent -dev)
└── scripts/smoke-test.sh    # 冒烟:拉起 consul+provider,跑 consumer,拆除
```

## 为什么注册中心是手工接线的(没有 starter)

`starter-kitex` 只会构建 **etcd** 注册中心,所以本示例自己完成 Consul 接线。
`provider/server.go` 是一个 `gs.Server` 适配器,用
`server.WithRegistry(consul.NewConsulRegister(...))` 构建 Kitex server 并注册
`EchoServiceImpl`。其余部分仍是 Go-Spring 风格:server 是一个 IoC bean,
`gs.Run()` 驱动其生命周期。

consumer 通过 `consul.NewConsulResolver(addr)` 传入 `client.WithResolver(...)`
来解析地址,然后逐传输各调用一次并分别断言。

## 注册中心配置

```properties
# provider:绑定到一个具体的 host:port(不能用通配符)。Consul 会精确注册这个地
# 址并对它做 TCP 健康检查,所以 0.0.0.0 永远过不了检查。
spring.kitex.server.addr=127.0.0.1:8888
spring.kitex.server.service.name=echo
spring.kitex.server.registry.consul=127.0.0.1:8500

# consumer:同一个 Consul agent,同一个服务名。
spring.kitex.consumer.registry.consul=127.0.0.1:8500
spring.kitex.consumer.service.name=echo
```

## 运行

```bash
docker compose up -d          # 或 docker-compose up -d
go run ./provider &           # 长期运行,注册进 Consul
go run ./consumer             # 经 Consul 发现,逐传输调用
```

consumer 预期输出:

```
[KitexProtobuf] response from discovered provider: Hello, Kitex!
[gRPC] response from discovered provider: Hello, Kitex!
```

或者一次性冒烟测试:

```bash
bash scripts/smoke-test.sh
```
