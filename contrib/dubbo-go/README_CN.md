# dubbo-go(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个 [Dubbo-go](https://dubbo.apache.org/zh-cn/overview/mannual/golang-sdk/)
`GreetService` 示例:先用 Dubbo-go 工具链生成代码,再改造成 Go-Spring 的启动
与配置方式 —— 由 `gs.Run()` 驱动生命周期,provider 作为 IoC bean 注入,监听
端口取自 `conf/app.properties`,而不是写死在 `main()` 里。

采用 **Triple** 协议 + **无注册中心直连**,示例自包含,无需 ZooKeeper/Nacos。

这是一个**可运行示例**,并非可复用的 starter 模块。

## 目录结构

```
contrib/dubbo-go/
├── proto/greet.proto        # Protobuf IDL
├── proto/greet.pb.go        # protoc 生成的消息(请勿手改)
├── proto/greet.triple.go    # Triple 生成的桩代码(请勿手改)
├── handler.go               # GreetProvider,导出为 greet.GreetServiceHandler bean
├── server.go                # DubboServer 适配器(gs.Server)+ Config
├── main.go                  # gs.Run() + 自测客户端
└── conf/app.properties      # 配置
```

## 如何生成

```bash
# 工具(一次性)
go install github.com/dubbogo/protoc-gen-go-triple/v3@latest

# 从 IDL 生成消息 + Triple 桩代码
protoc --proto_path=proto \
  --go_out=paths=source_relative:./proto \
  --go-triple_out=paths=source_relative:./proto \
  proto/greet.proto
```

生成器会在 `proto/` 下产出 `greet.pb.go` 和 `greet.triple.go`。重新执行该命令
只会再生成这两个文件,不会覆盖改造后的 `handler.go` / `server.go` / `main.go`。

> 注意:在 `runtime.Version()` 带实验后缀(如 `go1.26.1-X:jsonv2`)的 go1.26
> 工具链上,`protoc-gen-go-triple` v3.0.3 会在解析版本时 panic。需从源码重新
> 编译,并把版本串截断为纯数字部分。

## 改造:原生 Dubbo-go → Go-Spring

| 关注点   | Dubbo-go 脚手架                            | Go-Spring 版本                                                            |
| -------- | ------------------------------------------ | ------------------------------------------------------------------------- |
| 启动     | `main()` 中 `srv.Serve()` 阻塞             | `DubboServer` 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop               |
| handler  | `RegisterGreetServiceHandler(srv, &impl)`  | `gs.Provide(&GreetProvider{}).Export(gs.As[greet.GreetServiceHandler]())` |
| 是否启用 | 总是开启                                   | 通过 `gs.OnBean` 条件依赖 `greet.GreetServiceHandler` bean                |
| 端口     | 写死的默认值                               | 取自 `conf/app.properties` 的 `${spring.dubbo.server.port}`               |
| 关停     | 进程自持                                   | 由 Go-Spring 协调优雅关停(SIGTERM → `Stop()`)                           |

`server.go` 里的适配器是关键:Dubbo-go 的 `Serve()` 会绑定监听端口后永久阻塞,
因此将其放到一个仅在 `sig.TriggerAndWait()` 之后启动的 goroutine 中运行,`Run`
则阻塞在一个 done channel 上,由 `Stop()` 关闭它,把控制权交回 Go-Spring 的关停
流程。

## 配置

```properties
# 关闭内置 HTTP server,本示例只暴露 Dubbo 端点。
spring.http.server.enabled=false

# Dubbo Triple 监听端口,经 ${spring.dubbo.server} 前缀读取,默认 20000。
spring.dubbo.server.port=20000
```

## 运行

```bash
go run .
```

`main.go` 会在启动 500ms 后拉起客户端,调用 `Greet("Hello, Dubbo-Go!")`,断言
返回的 greeting,然后发送 SIGTERM 让 Go-Spring 优雅关停。预期输出包含:

```
Response from server: Hello, Dubbo-Go!
```
