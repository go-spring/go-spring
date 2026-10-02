# Nacos 注册中心(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

通过 **Nacos** 实现服务注册与发现,使用
[dubbo-go](https://dubbo.apache.org/en/overview/mannual/golang-sdk/) 的
`GreetService`,走 **Triple** 协议(protobuf-over-HTTP/2,与 gRPC 线兼容)。
provider 启动时把 `greet.GreetService` 注册进 Nacos;consumer 不去拨打写死的
`host:port`,而是从同一个 Nacos 解析出存活的 provider 地址。

Nacos 既是注册中心 **也是** 配置中心;运行示例后可在自带控制台
<http://127.0.0.1:8848/nacos>(默认账号 `nacos`/`nacos`)里查看已注册的实例。

这是 [`..`](..) 下五个兄弟示例之一 —— 注册中心总览见顶层
[README](../README_CN.md)。四个 dubbo-go 示例(etcd / nacos / zookeeper /
polaris)的应用代码 **完全一致**,只有 `conf/app.properties` 里的注册中心配置块
不同。

## 目录结构

```
nacos/
├── idl/greet.proto          # protobuf IDL
├── idl/greet.pb.go          # protoc 生成的消息(请勿编辑)
├── idl/greet.triple.go      # Triple 生成的桩代码(请勿编辑)
├── idl/gen-code.sh          # 从 IDL 重新生成 idl/*.go
├── provider/handler.go      # GreetProvider + StarterDubbo.ServiceRegister bean
├── provider/main.go         # gs.Run();长期运行,注册进 Nacos
├── provider/conf/app.properties  # provider 配置(注册中心 + Triple 端口)
├── consumer/main.go         # 经 Nacos 发现、调用、断言、退出
├── consumer/conf/app.properties  # consumer 配置(注册中心 + 客户端协议)
├── docker-compose.yml       # 本地 Nacos(standalone)
└── scripts/smoke-test.sh    # 冒烟:拉起 nacos+provider,跑 consumer,拆除
```

## 注册中心配置

注册中心在 `${spring.dubbo.registries}` 下统一声明;map 的 key 是逻辑 ID,
`protocol` 选择驱动类型。切换到别的注册中心就是改这两行。

```properties
spring.dubbo.registries.nacos.protocol=nacos
spring.dubbo.registries.nacos.address=127.0.0.1:8848
```

server 默认把服务发布到所有声明的注册中心(未设置 `registry-ids`);consumer 从
同一个注册中心解析 `greet.GreetService` —— 这个接口名固化在 Triple 桩代码里。

## 运行

```bash
docker compose up -d          # 或 docker-compose up -d
go run ./provider &           # 长期运行,注册进 Nacos
go run ./consumer             # 经 Nacos 发现并调用
```

consumer 预期输出:

```
Response from discovered provider: Hello, Dubbo-Go!
```

或者一次性冒烟测试:

```bash
bash scripts/smoke-test.sh
```
