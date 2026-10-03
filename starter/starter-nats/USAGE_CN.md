# starter-nats 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`、`messaging.go`）与可运行的
[example/](example/) / [example-otel/](example-otel/) 核对。**NATS 自身语义（core NATS、
JetStream、queue group、subject 通配符、drain）见 [nats.go 官方文档](https://docs.nats.io/)**——
本文只写 go-spring 的增量。

**激活方式**：每个 `spring.nats.instances.<name>` 条目注册一个名为 `<name>` 的 `*Conn` bean
[starter.go:35-54]。没有条目 = starter 不生效。仅多实例；无默认单例。

---

## 1. 完整工程示例

生产者与消费者都走 messaging.Driver，并组合 actuator + otel + governance。文件树：

```
demo/
├── go.mod
├── main.go
├── messaging.go
└── conf/
    ├── app.properties
    └── governance.yaml
```

**go.mod**（关键依赖）：

```
require (
    github.com/nats-io/nats.go    v1.38.0
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-nats    latest
    go-spring.org/starter-actuator latest   // 可选：探针 + /metrics 挂载
    go-spring.org/starter-otel     latest   // 可选：真实 trace/metric 导出
)
```

**main.go**：

```go
package main

import (
    "demo/messaging"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-nats"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**messaging.go** —— 单连接上的 driver 生产者与消费者：

```go
package messaging

import (
    "context"
    "time"

    "go-spring.org/cloud/messaging"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

func init() {
    // starter 已为每个 NATS 连接自动注册一个 messaging.Driver bean，
    // 与连接同名——按名注入即可，无需手工注册。（手工用
    // StarterNats.NewDriver 构造会与自动注册的 bean 冲突。）
    gs.Provide(newConsumer).Export(gs.As[gs.Rooter]())
}

type Consumer struct {
    Sub messaging.Subscriber `autowire:"?"`
}

func newConsumer(drv messaging.Driver) *Consumer {
    sub, err := drv.NewSubscriber(context.Background(), "orders.created", "workers")
    if err != nil {
        panic(err)
    }
    return &Consumer{Sub: sub}
}

func (c *Consumer) Init(ctx context.Context) error {
    return c.Sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
        log.Infof(ctx, log.TagAppDef, "order received: %s trace-id-from-header=%v",
            string(m.Payload), m.Headers["traceparent"])
        return nil // 返回 error 走 Recover 错误路径
    })
}
```

任意位置发布（HTTP handler、cron、outbox drainer）：

```go
pub, _ := driver.NewPublisher(ctx, "orders.created")
defer pub.Close()
err := pub.Publish(ctx, &messaging.Message{
    Payload: []byte(`{"id":42}`),
    Headers: map[string]string{"tenant": "acme"},
})
```

**conf/app.properties** —— 完整注释配置面：

```properties
# --- nats（多实例：每个 spring.nats.instances.<name> = 一个 *Conn bean）------------------
spring.nats.instances.main.url=nats://127.0.0.1:4222
spring.nats.instances.main.name=orders-service        # 参与治理标签 + 服务端连接名
spring.nats.instances.main.jetstream.enabled=true     # 暴露 Conn.JetStream（同一连接）
# spring.nats.instances.main.max-reconnects=-1        # -1 = 无限（默认 60）
# spring.nats.instances.main.reconnect-wait=2s
# spring.nats.instances.main.connect-timeout=5s
# 认证（各风格互不冲突，选一种）：
# spring.nats.instances.main.username=... / password=...
# spring.nats.instances.main.token=...
# spring.nats.instances.main.creds-file=/etc/nats/app.creds
# spring.nats.instances.main.nkey-file=/etc/nats/app.nk
# TLS：spring.nats.instances.main.tls.enabled=true + ca-file/cert-file/key-file/...

# --- actuator（探针 + metrics 挂载）--------------------------------------------
spring.actuator.addr=:9370

# --- 可观测（starter-otel）------------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # /metrics 仅经 actuator

# --- 治理（运行时 resilience）----------------------------------------------------
spring.governance.source.file.path=conf/governance.yaml
```

**conf/governance.yaml**（§4 演练使用）：

```yaml
spring:
  governance:
    enabled: true
    resilience:
      nats:                 # 对应 ResourceLabel 方案，见 §4.3
        rate-limit: 100
        breaker:
          error-threshold: 5
```

**Broker 启动与验证**：

```bash
docker run -d --name nats -p 127.0.0.1:4222:4222 nats:2.10 -js
go run .                                   # driver 订阅后发布
curl -i :9370/healthz                      # actuator 存活
curl -s :9370/metrics | grep -i nats
```

---

## 2. 装配与时序

### 2.1 bean 生命周期时间线

```
import starter-nats
  └─ gs.Module(gs.OnProperty("spring.nats"))              [starter.go:35]
gs.Run()
  ├─ 配置绑定：每个 spring.nats.instances.<name> → Config（value tag；url 经 expr 校验 ≠ ""）
  ├─ 每个 name：Provide(newConn, IndexArg(1,ValueArg(name)), IndexArg(2,ValueArg(c)))
  │             .Name(name).Destroy((*Conn).Destroy).Caller(1)  [starter.go:45-54]
  ├─ newConn：Driver bean（由 ${spring.nats.instances.<name>.driver} 按实例选择：留空 = 按类型注入，
  │     配置 = 按 bean 名注入，指定的 bean 不存在则启动失败；无则回退到内置
  │     DefaultDriver）→ CreateClient(ctx, Config, cloud.ClientParams{Resilience: mgr, Fault: inj})
  │           （nats.Connect —— 快速失败探针；broker 不可用则中断启动）
  ├─ CreateClient：NewConn(nc, url, params)——一步同时定身份与应用治理，
  │           Driver 返回时 Conn 即完整（`nats:<url>` service label；
  │           params.ExecutorFor → fault.WrapClientExecutor(mgr.ClientExecutorFor("nats", service), service, inj)，
  │           零值 bundle 时降级为 resilience.Unmanaged）
  │           [driver.go:145; client.go:86-93]
  ├─ jetstream.enabled → jetstream.New(nc)；失败会关闭 nc 并中断启动
  │           [driver.go:149-157]
  ├─ newConn：把 mgr/inj（= 注入的 *resilience.Manager / *fault.Injector bean）
  │           打包进交给 Driver 的 cloud.ClientParams；此后无任何补装配
  │           [driver.go:181-182]
  ├─ ping 时启动连通性检查（裸 client 的 IsConnected）；失败则销毁该 bean
  │           [driver.go:197-202]
  ├─ Run / 就绪
  └─ SIGTERM：(*Conn).Destroy → exec.Close()（错误在 Drain 后向上返回）再 conn.Drain()
              —— 在途订阅收尾后关闭 socket [client.go:105-113]
```

快速失败：`nats.Connect` 同步完成首次拨号；url/认证错误或 broker 宕机会以
`failed to connect nats: <url>` 中断启动 [driver.go:132-136]。启动后断连会打 Warn
（`nats disconnected`）并由客户端自动重连 [driver.go:88-97]——`HealthCheck(ctx, conn)` 反映实时的
`IsConnected()` 状态 [health.go]。

### 2.2 一次发布，逐层走读

`pub.Publish(ctx, msg)` [messaging.go:81-108]：

1. 信封 → `nats.Msg{Subject, Data, Header}`；`messaging.Message.Key` 经保留 header
   `x-msg-key` 透传（消费侧还原进 `Key`，不会泄漏进 `Headers`）[messaging.go:82-89]。
   空 header map → nil header。
2. 压测标记：`prop.Inject(ctx, …)` 写入标记（canonical 为 `X-LoadTest: 1`），
   供消费侧识别合成流量；非压测流量下是空操作，header map 按需分配。
3. driver 声明本次发布的语义并路由到 Conn 的发布缝 [messaging.go:100-107]：
   `PublishMsgContext` 在 ctx 上声明 `operation(opPublish, subject)`，把 W3C `traceparent`
   从 executor 的 attempt ctx 注入 `nm.Header`（消费侧得以延续 trace），
   连接的 resilience executor——唯一发射点——打开 producer span（经 Publish 的 ctx 挂到
   调用方活动 span 下）、记录时长并写 access log。
4. 裸 `c.conn.PublishMsg`（异步缓冲写——broker 确认前即返回；需确认用 Flush，
   见 [nats.go 文档](https://docs.nats.io/)）。executor 包裹的正是这次线上调用，
   因此每条消息恰好计一次。

该 driver 路径与裸客户端路径一样受保护（见 §2.4）：它走 `PublishMsgContext`，
没有发布是未声明、未受保护的。

### 2.3 一次消费，逐层走读

`sub.Subscribe(handler)` [messaging.go:123-143]：handler 包进 `messaging.Recover`
（panic → error 路径，不冲垮 SDK goroutine）[messaging.go:127]；driver 把订阅路由到
`Conn.Consume`（group 为空即普通订阅，非空即 queue group / 竞争消费）
[messaging.go:134-137]。每条消息：

1. driver 声明本次消费的语义并路由到 Conn 的消费缝：`Consume` 从信封 headers（映射自
   `nm.Header`）提取 W3C `traceparent` 到新 ctx，在其上声明 `operation(opConsume, subject)`，
   连接的 resilience executor——唯一发射点——在调用 handler 前打开 consumer span
   （producer span 的子）、记录时长并写 access log [command.go:158-165]。
2. `X-LoadTest` header 重新物化进 ctx 作为压测标记 [messaging.go:134-137]。
3. `fromNatsMsg`：多值 NATS header 压平为单值（`Get` = 首值优先）
   [messaging.go:169-180]。
4. handler 执行；executor 记录结果，handler 返回的错误也会让 consumer span
   标记为失败。Close = `Subscription.Unsubscribe`。

这条路径服务于任何 `Conn.Consume(ctx, subject, queue, handler)` 调用方；messaging.Driver
的订阅路径则走同一个缝（driver 只额外做信封转换），因此两条路径插桩一致、互不重复计数。

未插桩：**JetStream 消费**，它有自己的 API 面。需要可追踪的 pub/sub 请用
`PublishMsgContext`/`Consume`。

### 2.4 保护机制（guard）——哪些受保护、哪些不受

NATS 没有可拒绝的 middleware（不像 redis Hook / http RoundTripper），因此 resilience
executor 在调用点驱动，而非作为 interceptor 串入。每次发布/消费都**声明**本次操作的语义
（见 observe.go）并经 `guard` 跑在 executor 下 [command.go:197-202]——executor 是**唯一发射点**：
它从 ctx 读取声明的操作，打开调用 span，记录调用级 `messaging.client.operation.duration`
与尝试级 `messaging.client.attempt.duration` histogram、在途 gauge、`resilience.client.calls`
counter 以及那一条 access log。publish 与 consume 声明为 `NonIdempotent`，所以针对该 label、
带重试策略的治理规则，其重试会被**抑制**（每个 service 告警一次）：重新发布或再跑一遍
handler 是第二个副作用，不是第二次尝试。

| 入口 | 发射方 | Resilience guard |
|---|---|---|
| `Conn.PublishMsg` / `Conn.PublishMsgContext`（裸客户端路径） | starter 声明 + executor 发射 | 是 |
| `Conn.Consume(ctx, subject, queue, handler)`（裸客户端路径） | starter 声明 + executor 发射 | 是 |
| messaging.Driver 发布/订阅（走 `PublishMsgContext` / `Consume`） | starter 声明 + executor 发射 | 是 |
| `Conn.Publish` / `Conn.Request` / `Subscribe` / `QueueSubscribe` / JetStream | **否** | **否** |
| `Conn.PublishGuarded(ctx, subj, data)` | starter 声明 + executor 发射（经 `PublishMsgContext`） | 是 |
| `Conn.RequestGuarded(ctx, subj, data, timeout)` | 仅 executor 的兜底信号 | 是 |

`RequestGuarded` 不声明操作，因此 executor 只发射本层的兜底信号（`resilience.client.duration`、
以 service 命名的 span），不会产生 `messaging.client.*` 信号——它没有专属的操作 span。

`Governance.ExecutorFor` 内部包裹顺序（见 `cloud/governance/governance.go`，由
[NewConn](client.go:86-93) 调用）：`mgr.ClientExecutorFor("nats", service)`（由注入的
`*resilience.Manager` bean 装备；bundle 为零值时降级为 `resilience.Unmanaged` 仅观测的 executor）→
`fault.WrapClientExecutor`（故障注入，最外层）。manager 的 resolve 步骤会用 observe 发射器包裹后端
executor（见 `resilience/observe.go`），因此声明的操作其 span/counter/histogram/access-log 只在链上
唯一能看到完整一次调用的那点产生一次。拒绝时 guarded 调用返回
resilience 哨兵错误（`ErrRateLimited` / `ErrCircuitOpen`），底层发布/请求不会被调用
——[resilience_test.go:53-84] 有证明。`service` 是 `nats:<url>`（按连接而非
按 subject）[client.go:86-93]，限流/熔断状态在同一连接的全部 subject 间共享。

`PublishGuarded` 接收调用方 ctx，并把它同时透传给 executor 与调用 span
（经 `PublishMsgContext`）。`RequestGuarded` 接收调用方 ctx，但 timeout 是 `Request`
内部的每次尝试超时，且它不声明操作，故不打开自己的操作 span。

### 2.5 健康检查

除非条目设置 `health=false`，每个实例都会贡献一个名为 `nats:<name>` 的
`health.Indicator` [starter.go]。探针直接读取裸 client 的 `IsConnected()`（不开 span、
不花限流/熔断额度），因此重连间隙中的实例会报 down 并在重连后自行恢复。对应用离开它就无法服务的依赖而言这是正确默认；若某条连接
是可选旁路、不应计入就绪，设 `health=false`——该指标将完全不注册，而不是注册成
永远绿的摆设。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.nats.instances.<name>.*`。分组 key `tls` 绑定嵌套共享
struct——其子 key 属于 security，不属于本 starter。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `url` | string | — | **必填**（`expr:"$ != ''"` [config.go:31]）；逗号分隔多服务器交给 `nats.Connect`。 | 缺失/空 → 启动期绑定错误。 |
| `name` | string | "" | 服务端连接名（不是治理 label 的组成部分，label 固定为 `nats:<url>`）[driver.go:77]。 | 空名可用。 |
| `username` / `password` | string | "" | 都设置 → `nats.UserInfo`。与其他认证风格正交 [driver.go:103-105]。 | 只设 username → 发送空密码。 |
| `token` | string | "" | → `nats.Token` [driver.go:106-108]。 | 与 username 并用 → nats option 后者覆盖（NATS 定义）。 |
| `creds-file` | string | "" | JWT+nkey seed 文件 → `nats.UserCredentials` [driver.go:109-111]。 | 路径错误 → 启动失败。 |
| `nkey-file` | string | "" | nkey seed → `nats.NkeyOptionFromSeed`；加载失败带解释并中断启动 [driver.go:112-118]。 | 坏 seed → 启动报错。 |
| `tls` | group | 关 | `tls.enabled=true` → `security.BuildClient()` → `nats.Secure`；BuildClient()==nil → 裸 `nats.Secure()` [driver.go:119-131]。⚠ 用 `BuildClient()` 而非 `BuildServer()`——与其他 client starter 同样的 client-TLS 姿态。子 key：`enabled`/`ca-file`/`cert-file`/`key-file`/`insecure-skip-verify`/`server-name`（security 的 tag）。 | TLS 不匹配 → 启动期连接错误。 |
| `max-reconnects` | int | 60 | → `nats.MaxReconnects`；-1 = 无限 [driver.go:78]。 | -1 且 broker 宕机 → 永久重连循环（设计如此）。 |
| `reconnect-wait` | duration | 2s | 重连尝试间隔 [driver.go:79]。 | 过小 → 对宕机集群高频重连。 |
| `connect-timeout` | duration | 5s | 仅约束**首次拨号** [driver.go:80]。 | 过小 → 慢网络误判启动失败。 |
| `jetstream` | group | — | `enabled` 的容器。 | — |
| `jetstream.enabled` | bool | false | 在**同一**连接上派生 `jetstream.New(nc)`；失败关闭 nc 并中断启动；否则 `Conn.JetStream` 保持 nil [driver.go:143-151]。 | 对未开 `-js` 的 broker 启用 → 启动错误。 |
| `ping` | bool | false | 可选启动探测：`HealthCheck` 单次检查连接状态，broker 不可达则启动失败；默认关，未就绪的 broker 到首次发布才暴露 [driver.go:197-202]。 | 期待 fail-fast 却没开 → 启动"成功"，首次发布失败。 |
| `health` | bool | true | 为实例注册 `nats:<name>` 健康指示器；false 让该连接不卷入聚合健康 [starter.go]。 | false → 无指示器 bean，不再上报该连接的就绪。 |

grep 核对：本 starter Go 文件中 15 个去重 `value:` tag 恰为 `url`、`name`、`username`、
`password`、`token`、`creds-file`、`nkey-file`、`tls`、`max-reconnects`、
`reconnect-wait`、`connect-timeout`、`jetstream`、`jetstream.enabled`
（即 `${enabled:=false}`）、`ping`、`health`——全部在表内；两边无多余项。

---

## 4. 验证与故障演练

### 4.1 broker 宕机快速失败

```bash
docker stop nats
go run .          # 启动中断："failed to connect nats: nats://127.0.0.1:4222"
```

启动后演练：`docker stop nats` → Warn `nats disconnected`，应用存活，`HealthCheck(ctx, conn)` 返回
错误；恢复 broker → Info `nats reconnected to ...` [driver.go:88-97]。

### 4.2 guarded vs 未 guarded 路径

治理开启且限流收紧（§1 governance.yaml）时：

- `conn.Publish("s", b)`——始终成功（无 guard）[delegate.go:42]。
- 热循环里调 `conn.PublishGuarded(ctx, "s", b)` → burst 耗尽后以
  `resilience.ErrRateLimited` 拒绝，且被拒调用不会触达 socket
  [resilience_test.go:53-65]。
- 熔断演练：让发布持续失败（启动后停 broker）直到 `error-threshold` 跳闸 → 后续
  guarded 调用返回 `ErrCircuitOpen`，不执行发布 [resilience_test.go:69-84]。

### 4.3 治理标签核对

executor 的 service 是 `nats:<url>` [client.go:89]。把 governance.yaml 规则限定到
`nats:orders-service`（或前缀），策略即精确落到该连接。验证：wrapped-executor
的拒绝会发 span + counter（`system="nats"`，由 resilience-observe 桥命名）
[command.go:179-181]——演练后去 trace/metric 里 grep `nats`。

### 4.4 消息往返与 driver 映射存活

发布带 `Payload` + `Headers{"tenant":"acme"}` 的信封；消费侧断言：

- `m.Payload` 逐字节存活。
- `m.Headers["tenant"]` 存活（string→nats.Header→string）。
- tracing 生效时 `m.Headers["traceparent"]` 存在（§2.2 第 4 步注入）。
- `m.Key` 经保留 header `x-msg-key` 往返（`m.Headers` 中不可见）。
- 多值 header：仅首值存活。

### 4.5 指标 / span / 日志读取

- access log：每次声明的发布/消费一条结构化记录，由 resilience 发射器写在 `nats`/`access`
  tag 下（默认 `brief`；`detailed` 追加至多 `maxArgBytes` 的 payload 字节）。失败 → Warn；
  带 subject 的成功 → Debug；无 subject 的成功 → Info。
- metrics：`messaging.client.operation.duration`（调用级）与
  `messaging.client.attempt.duration`（尝试级）histogram，加
  `messaging.client.active_requests` gauge，标签为 `messaging.system="nats"`、
  `messaging.operation`、`status`——subject 只作为 span/log 细节、永不进标签；
  `resilience.client.calls` 每次调用计数。`curl -s :9370/metrics | grep -i nats`。
- span：每次发布/消费一个 span，名为 `publish` / `consume`，携带
  `messaging.system` / `messaging.operation` / `messaging.destination.name`，经 W3C header
  关联；[example-otel/](example-otel/) 附带完整
  Jaeger 校验（`http://127.0.0.1:16686/api/traces?service=...`）。
- 连接事件：`messaging.client.connection.state_changes` counter（留在 starter，由 NATS
  客户端自身回调驱动）以及 async error / disconnect / reconnect / close 落在日志 tag
  `app_def` [driver.go:81-102]。
- 停机：Drain 让在途订阅收尾；`exec.Close()` 的错误在 Drain 之后返回给
  destroy 钩子 [client.go:105-113]。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| 启动中断 `failed to connect nats` | broker 宕机 / `url` 错 / 认证被拒（首次拨号快速失败）[driver.go:132-136] | 启动 broker（`docker run ... nats:2.10 -js`），修 `url`/认证。 |
| 启动中断 `failed to create jetstream context` | `jetstream.enabled=true` 但 broker 未开 `-js` [driver.go:143-151] | 服务端加 `-js` 或关掉该 key。 |
| `Conn.JetStream` 为 nil | 未设 `jetstream.enabled` | 设置它；JS 在启动期从同一连接派生。 |
| 消费者能收消息但消费侧无 trace/metric | 用了裸 `Conn.Subscribe`/`QueueSubscribe` 委托而非 `Conn.Consume` | 走 `Conn.Consume(ctx, subject, queue, handler)` 或 messaging.Driver。 |
| guarded 调用突然报哨兵错误 | 限流耗尽或熔断打开——设计行为 [command.go:187-192] | 检查 governance.yaml 策略；熔断冷却后自愈。 |
| producer trace 不链接调用方 span | 发布走的是 `PublishMsg`（无 ctx 参数），其 span 必为新根 | 改用 `PublishMsgContext(ctx, msg)`——`PublishGuarded` 已如此。 |
| `/readiness` 报 `nats:<name>` 不健康 | 自动重连客户端正处于重连间隙 | 属预期；若该实例连通性不应计入就绪，对它设 `health=false`。 |
| 第二个 header 值丢失 | driver 多值 header 压平取首值（单值信封） | 额外值放 payload，或用裸 Conn API。 |
| 日志出现重连风暴 | 集群宕机时 `reconnect-wait` 过小 | 调大；重连本就是客户端的可靠性机制。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 17 个 starter 本地 value tag（+ tls 分组子 key 在 security） |
| 必填 | 1（`url`） |
| quickstart 前置外部依赖 | 1（nats；collector 可选用于可观测） |
| "注意/坑"条数 | 4 |

设计嫌疑（审计台账）：

- **已修**：发布 span 现可经 `PublishMsgContext` 挂到调用方 trace 下（无 ctx 的
  `PublishMsg` 保留其已文档化的新根行为以维持兼容——nats.PublishMsg 本身无 ctx 参数）；
  消费侧不再只有 driver 才有插桩，任何 `Conn.Consume` 调用方都覆盖；每实例接线了
  `health.Indicator`（`health`，见 §2.5）；`PublishGuarded` 现在把调用方 ctx
  透传进调用 span；每次发布/消费都声明操作、由 resilience executor 作唯一发射点
  （starter 内不再有按调用的插桩）。
- **未解**：JetStream 消费无 trace（独立 API 面）；多值 header 压平取首值；
  `RequestGuarded` 不声明操作，只发射 executor 的兜底信号。
