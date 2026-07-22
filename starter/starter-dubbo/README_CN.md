# starter-dubbo

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-dubbo` 基于 [dubbo.apache.org/dubbo-go/v3](https://pkg.go.dev/dubbo.apache.org/dubbo-go/v3)
为 Go-Spring 服务提供轻量封装。**服务端**只需注册服务，Starter 会自动完成 Dubbo
服务器构建、生命周期管理与优雅停机；**客户端**则直接提供可用的 `*client.Client`
Bean（一个默认客户端，外加任意具名实例），用于基于注册中心的服务发现。两种角色共享
同一个 dubbo `Instance` 与定义在 `${spring.dubbo.registries}` 下的全局注册中心。

## 安装

```bash
go get go-spring.org/starter-dubbo
```

## 快速开始

### 1. 引入 `starter-dubbo` 包

参见 [example.go](example/example.go) 文件。

```go
import StarterDubbo "go-spring.org/starter-dubbo"
```

### 2. 配置 Dubbo 服务器

在项目的[配置文件](example/conf/app.properties)中添加 Dubbo 配置：

```properties
spring.http.server.enabled=false
spring.dubbo.application.name=greet-example
spring.dubbo.registries.etcd.protocol=etcdv3
spring.dubbo.registries.etcd.address=127.0.0.1:2379
spring.dubbo.protocols.tri.name=tri
spring.dubbo.protocols.tri.port=20000
```

`spring.dubbo.application.name` 为**必填**：server 与 client 共享同一个 dubbo
`Instance`，应用名会作为 metrics/注册中心的身份标识，缺失时 starter 会直接报错。
其余应用字段可选：`organization`、`module`、`group`、`version`、`owner`、`environment`、
`metadata-type`（`local` 默认 / `remote`）。

`spring.dubbo.registries` 同样为**必填**，是注册中心的唯一事实来源——只在顶层定义一次，
不再内联到 server/client 之下。只有至少定义了一个注册中心时，starter 才会创建 Dubbo
组件，且每个条目都必须带 `address`（二者都会在启动时前置校验）。各角色通过 `registry-ids`
按 ID 选择要用哪些注册中心（留空表示全部）。注册中心为 map 驱动——map 的 key 即逻辑
注册中心 ID——因此可同时定义多个：

```properties
# 只在顶层定义一次注册中心（etcdv3/nacos/zookeeper/polaris/...）
spring.dubbo.registries.etcd.protocol=etcdv3
spring.dubbo.registries.etcd.address=127.0.0.1:2379
spring.dubbo.registries.nacos.protocol=nacos
spring.dubbo.registries.nacos.address=127.0.0.1:8848

# server（或 client）按 ID 选择注册中心；留空表示全部
spring.dubbo.server.registry-ids=etcd

# 协议同样是全局节点，在顶层定义一次，由 server 自动继承
spring.dubbo.protocols.tri.port=20000
spring.dubbo.protocols.dubbo.port=20001
```

`spring.dubbo.protocols` 是**全局**节点，与 `registries`/`application` 同级：协议监听器
进程内只定义一次，挂在共享 `Instance` 上，server 会自动继承（无需在 `${spring.dubbo.server}`
下重复声明）。map 的 key 是协议 ID，`name` 是 dubbo-go 协议类型
（`dubbo`/`tri`/`grpc`/`rest`/`jsonrpc`，留空时回退到 key）。server 侧若完全未配置任何
协议，则回退到单个 Triple:20000 监听器。

优雅停机同样为全局节点 `${spring.dubbo.shutdown}`，全部可选，留空沿用 dubbo-go 默认
（总超时 60s、步进 3s、consumer 更新等待 3s、内部信号开启）：

```properties
spring.dubbo.shutdown.timeout=60s
spring.dubbo.shutdown.step-timeout=3s
spring.dubbo.shutdown.consumer-update-wait-time=3s
# spring.dubbo.shutdown.internal-signal=false   # 关闭内部信号触发停机
```

`${spring.dubbo.server}` 下的所有配置项都是可选的，空值/零值会被跳过，dubbo-go
沿用自身默认值。

Provider 级通用配置：

```properties
spring.dubbo.server.group=g1
spring.dubbo.server.version=1.0.0
spring.dubbo.server.cluster=failover        # failover|failfast|failsafe|failback|forking|available|broadcast|zoneAware
spring.dubbo.server.load-balance=random     # random|roundrobin|leastactive|consistenthashing|p2c
spring.dubbo.server.serialization=hessian2  # hessian2|protobuf|msgpack|json
spring.dubbo.server.retries=2
spring.dubbo.server.filter=echo,tps
spring.dubbo.server.token=xxx
spring.dubbo.server.auth=true
spring.dubbo.server.tag=gray
spring.dubbo.server.access-log=true
spring.dubbo.server.warmup=10m
spring.dubbo.server.not-register=false
spring.dubbo.server.adaptive-service=false
```

单协议（顶层 `protocols.<id>`）：`name`、`port`、`ip`、`params.<k>`。
单注册中心（顶层 `registries.<name>`）：`address`、`namespace`、`group`、`username`、
`password`、`timeout`（如 `5s`）、`ttl`（如 `15m`）、`weight`、`zone`、
`simplified`、`preferred`、`params.<k>`。

### 3. 注册 Dubbo 服务

参见 [example.go](example/example.go) 文件。`ServiceRegister` 是一个把服务注册到 Dubbo
`server.Server` 上的函数；因为 Dubbo 生成的 `Register*Handler` 会返回 error，所以它也返回
error。

```go
gs.Provide(func() StarterDubbo.ServiceRegister {
    return func(svr *server.Server) error {
        return greet.RegisterGreetServiceHandler(svr, &GreetProvider{})
    }
})
```

## 客户端

Starter 同样以 Bean 形式提供 Dubbo 客户端，其开启条件与服务端相同的 `*Instance`
（没有注册中心的项目不会得到任何客户端）。与多数 go-spring 客户端 starter 不同，dubbo 客户端
是**进程级单实例**--对齐 dubbo-go.json 里单一的 `consumer` 节点：dubbo-go 一个进程只有一个
consumer，`${spring.dubbo.client}` 是单对象（不是 map），产出单个默认 `*client.Client` Bean。
注册中心与可观测性都从共享的 `Instance` 继承，因此客户端本身只承载 consumer 级默认值：

```properties
spring.dubbo.client.protocol=tri        # dubbo(默认)|tri|triple|jsonrpc
spring.dubbo.client.timeout=3s          # 单次请求超时，如 "3s"
spring.dubbo.client.registry-ids=etcd   # 按 ID 选择全局注册中心；留空表示全部
spring.dubbo.client.filter=             # 逗号分隔 filter 链；"-name" 去掉某项
spring.dubbo.client.check=true          # false 关闭启动期 provider 在线检查
```

需要原始 `*client.Client`（如 classic Dubbo 无 stub、用 `Dial`+`CallUnary`）时按类型装配：

```go
type Caller struct {
    Client *client.Client `autowire:""` // 按类型装配单 client
}
```

## 引用（Reference）

上文的客户端 Bean 是原始的 `*client.Client`。真实应用自动装配的是每个 proto 生成的
**类型化 stub**（如 `greet.GreetService`），而非原始客户端。`RegisterReference` 把这样的
stub 注册为 Bean，由单 client（按类型装配）与按引用的调优参数装配：

```go
import StarterDubbo "go-spring.org/starter-dubbo"

func main() {
    // 把 greet.GreetService stub 注册为 Bean。"greet" 是引用键名（对应
    // ${spring.dubbo.client.references.greet}）；单 client 按类型注入。
    StarterDubbo.RegisterReference("greet", greet.NewGreetService)
    // ...
}

type Caller struct {
    Svc greet.GreetService `autowire:""` // 按 stub 类型自动装配
}
```

引用**不会**像客户端那样按配置自动注册：类型化 stub 及其 `NewXxxService` 构造函数是
应用侧的生成代码，starter 无法感知，因此应用必须为每个 stub 显式调用 `RegisterReference`。
正是这一次显式调用保证了自动装配是类型安全的；该辅助函数只负责把装配方式（单 client +
引用配置）标准化，让每个 stub 的注册方式保持一致。

每个引用在 `${spring.dubbo.client.references.<name>}` 下调优——它是 `${spring.dubbo.client}`
的引用级对应物，为该 stub 覆盖 consumer 级默认值（连 protocol/registry-ids/filter 都可逐 stub
覆盖，所以一个进程内不同 stub 用不同协议/注册中心无需多个 client）。所有字段可选；留空则保留
dubbo-go 默认值或 consumer 级默认值：

```properties
spring.dubbo.client.references.greet.protocol=tri             # 覆盖客户端级 protocol
spring.dubbo.client.references.greet.registry-ids=etcd        # 覆盖客户端级 registry-ids
spring.dubbo.client.references.greet.timeout=3s               # 单次请求超时；覆盖客户端级
spring.dubbo.client.references.greet.retries=2                # 仅在 cluster=failover（默认）下生效；-1 保留 dubbo-go 默认，0 禁用
spring.dubbo.client.references.greet.cluster=failover         # failover(默认)|failfast|failsafe|failback|forking|available|broadcast|zoneAware
spring.dubbo.client.references.greet.load-balance=roundrobin  # random(默认)|roundrobin|leastactive|consistenthashing|p2c
```

不同 stub 用不同协议的典型写法（单 client 给默认，逐引用覆盖）：

```properties
spring.dubbo.client.protocol=tri                          # 默认协议
spring.dubbo.client.references.orders.timeout=3s          # orders 用默认 tri
spring.dubbo.client.references.legacy.protocol=dubbo      # legacy 单独用 dubbo
spring.dubbo.client.references.legacy.timeout=5s
```

## Filter（过滤器）

dubbo-go 内置了一组 filter，并在两端各启用一条默认链,因此无需配置即有合理行为。
**provider** 默认链为 `echo,token,accesslog,tps,generic_service,execute,pshutdown`；
**consumer** 默认链为 `cshutdown`。常用 filter：

| filter | 端 | 作用 | 是否默认启用 |
|---|---|---|---|
| `echo` | provider | 回声/健康探测 | 是 |
| `token` | provider | token 鉴权 | 是 |
| `accesslog` | provider | 访问日志 | 是 |
| `tps` | provider | TPS 限流 | 是 |
| `execute` | provider | 并发数限流 | 是 |
| `generic_service` | provider | 泛化调用 | 是 |
| `pshutdown` / `cshutdown` | provider / consumer | 优雅停机 | 是 |
| `auth` / `sign` | provider / consumer | 请求签名 | 否 |
| `active` | consumer | 客户端活跃/并发数控制 | 否 |
| `metrics` / `tracing` | 双端 | 指标 / 链路 | 否（已由可观测性内置） |
| `hystrix_provider` / `hystrix_consumer` | 双端 | Hystrix 熔断 | 否 |
| `sentinel-provider` / `sentinel-consumer` | 双端 | Sentinel 流控 | 否 |
| `seata` | 双端 | 分布式事务 | 否 |
| `padasvc` | provider | 自适应并发限流 | 否 |

`filter`（逗号分隔）是**整链覆盖**语义。若只想在默认链上增删，用 dubbo-go 原生的
`-name` 前缀去掉某一项，例如 `spring.dubbo.server.filter=-tps` 关闭默认的 TPS 限流。

需要调参的 filter 读取**服务级**参数（对该 server 导出的所有服务生效）。全部可选，
留空/负值沿用 dubbo-go 默认（tps/execute 默认 `-1`，即不限流）：

```properties
# TPS 限流（"tps" filter 在默认链中）
spring.dubbo.server.tps-limit-rate=100                 # 每个周期允许的请求数
spring.dubbo.server.tps-limit-strategy=slidingWindow   # fixedWindow|slidingWindow|threadSafeFixedWindow
spring.dubbo.server.tps-limiter=                        # 自定义限流器实现；留空用默认
spring.dubbo.server.tps-limit-rejected-handler=        # 超限时的处理器

# 并发限流（"execute" filter 在默认链中）
spring.dubbo.server.execute-limit=200                  # 最大并发执行数
spring.dubbo.server.execute-limit-rejected-handler=

# 请求签名 —— 与 "auth" filter 搭配
spring.dubbo.server.filter=echo,token,accesslog,tps,generic_service,execute,pshutdown,auth
spring.dubbo.server.param-sign=true

# 逃生舱：任意其他 provider 级 filter 参数，原样透传
spring.dubbo.server.params.some-filter-key=some-value
```

**客户端**侧的 `filter` 与 `params` 与服务端对称，在 consumer 级（`${spring.dubbo.client}`）
设置，也可逐引用覆盖：

```properties
spring.dubbo.client.filter=cshutdown,active
spring.dubbo.client.params.some-filter-key=some-value
# 或逐引用：
spring.dubbo.client.references.orders.filter=cshutdown,active
spring.dubbo.client.references.orders.params.some-filter-key=some-value
```

### 无法在此处配置的 filter

有些 filter 存在,但**不由本 starter 的配置驱动**——在 dubbo-go 当前(基于
Instance)的 API 下,它们从别处读取设置。需要就把它们加进链里,但要用各自的方式配置:

- **`seata`** —— 零配置;仅透传 `SEATA_XID` attachment。把 `seata` 加进链即可,
  没有可调参数。
- **`sentinel-provider` / `sentinel-consumer`** —— 加入该 filter 会调用 Sentinel 的
  `InitDefault()`,但流控/熔断**规则**来自 Sentinel-go 自身(通过环境变量指定的配置
  文件,或代码里的 `flow.LoadRules`),而非 dubbo 配置,本 starter 喂不进去。
- **`hystrix_provider` / `hystrix_consumer`** —— ⚠️ 它们从 dubbo-go 的**旧版全局
  config 单例**读取 command 配置,而本 starter 使用的 Instance API 从不填充这个单例。
  因此 Hystrix 配置在这里会被静默忽略,filter 实际上是空操作——不要依赖它。(上游
  `github.com/afex/hystrix-go` 也已归档停维。)限流/熔断请改用 `tps` / `execute`
  或 Sentinel。

## 可观测性（内置）

metrics 与 tracing 默认开启，每个 server 和 client 零配置即具备可观测能力：

```properties
# Metrics —— Prometheus，默认开启，暴露在 http://127.0.0.1:9090/metrics
spring.dubbo.metrics.enable=true
spring.dubbo.metrics.port=9090
spring.dubbo.metrics.path=/metrics
spring.dubbo.metrics.push-gateway-address=   # 非空时额外推送到该 Prometheus pushgateway

# Tracing —— OTel，默认开启，使用 stdout exporter
spring.dubbo.tracing.enable=true
spring.dubbo.tracing.exporter=stdout        # stdout|jaeger|zipkin|otlp-http|otlp-grpc
spring.dubbo.tracing.endpoint=              # exporter 非 stdout 时必填
spring.dubbo.tracing.propagator=w3c         # w3c|b3
spring.dubbo.tracing.mode=                  # always|never|ratio（留空沿用 dubbo-go 默认）
spring.dubbo.tracing.ratio=1.0
```

任一项可用 `enable=false` 关闭。当 `exporter` 非 `stdout` 时必须提供 `endpoint`
（否则 starter 会直接报错）。例如把 trace 发往 OTLP 采集器：

```properties
spring.dubbo.tracing.exporter=otlp-grpc
spring.dubbo.tracing.endpoint=127.0.0.1:4317
```

## 日志（内置）

引入本 starter 即把 dubbo-go 纳入 go-spring 的托管:其内部日志会被自动桥接进
go-spring 的 `log` 模块(在 `init()` 中安装,无需配置)。dubbo-go 有两层 logger
门面 —— `dubbo-go/v3/logger`(上层栈)和 `dubbogo/gost/log/logger`(getty 及底层
模块)—— 桥接会同时接管两者,使每一条框架日志都走 go-spring 的日志管道,而不是
dubbo-go 默认的 stdout sink。

桥接只改变"由谁写日志",你仍需自行配置 go-spring 的日志 sink,否则转发过来的日志
会落到 go-spring 的默认 console,而不是你的应用输出。照常配置一个 root logger 即可:

```properties
logging.logger.root.type=FileLogger
logging.logger.root.level=INFO
logging.logger.root.dir=../logs
logging.logger.root.file=app.log
logging.logger.root.layout.type=JSONLayout
```

注意:dubbo-go 的 Logger 方法不带 `context.Context`,因此这条链路上无法透传
trace-id,记录的调用位置(file:line)也会指向桥接层而非真实打点处。

## 定制化（逃生舱）

类型化配置未覆盖的能力可以通过 map 形式的 `params` 字段（如每个协议的 `params`）补充，
这些参数会原样透传给 dubbo-go。

## 核心功能

[示例](example/example.go) 展示了一次 Dubbo Triple 端到端调用，并在 `runTest` 中做了断言：

1. **一元 Greet 调用**：服务端通过 Triple 协议在配置端口导出 `greet.GreetService`。客户端用
   `client.WithClientURL` 直连并调用 `Greet`，拿到原样返回的请求名作为问候语，验证标准的
   请求/响应链路。
2. **服务无关的服务器**：`SimpleDubboServer` 完全不认识 `GreetService`，只依赖一个
   `ServiceRegister` Bean，因此同一个服务器可驱动任意 Dubbo 服务；具体注册逻辑放在
   应用层。

## 说明

- 协议是全局 map 驱动节点，位于 `${spring.dubbo.protocols}`，map 的 key 即协议 ID，
  `name` 为 dubbo-go 协议类型（留空取 key），只有配置了的条目才会生效，并由 server 从
  共享 `Instance` 自动继承；注册中心则只在顶层 `${spring.dubbo.registries}` 定义一次，
  各角色通过 `registry-ids` 按 ID 选择（留空表示全部）。空字段会被跳过。未配置任何
  协议时，默认使用 20000 端口的 Triple 监听。
- Dubbo 服务器默认开启，可通过 `spring.dubbo.server.enabled=false` 关闭。
- 只需要注册一个 `ServiceRegister` Bean 即可激活整个服务器。
- `spring.dubbo.application.name` 必填；metrics（Prometheus）与 tracing（OTel/stdout）
  内置且默认开启 —— 参见[可观测性](#可观测性内置)。
