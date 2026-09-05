# starter-nats 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`、`driver.go`）与可运行的
[example/](example/) / [example-otel/](example-otel/) 核对。**NATS 自身语义（core NATS、
JetStream、queue group、subject 通配符、drain）见 [nats.go 官方文档](https://docs.nats.io/)**——
本文只写 go-spring 的增量。

**激活方式**：每个 `spring.nats.<name>` 条目注册一个名为 `<name>` 的 `*Conn` bean
[starter.go:33-40]。没有条目 = starter 不生效。仅多实例；无默认单例。

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
    └── govern.yaml
```

**go.mod**（关键依赖）：

```
require (
    github.com/nats-io/nats.go    v1.38.0
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-nats    latest
    go-spring.org/starter-actuator latest   // 可选：探针 + /metrics 挂载
    go-spring.org/starter-otel     latest   // 可选：真实 trace/metric 导出
    go-spring.org/starter-governance latest // 可选：运行时 resilience/fault
)
```

**main.go**：

```go
package main

import (
    "demo/messaging"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
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

    StarterNats "go-spring.org/starter-nats"
)

func init() {
    // 每个要绑定的 NATS 连接提供一个 messaging.Driver bean。
    // TagArg("main") 解析名为 "main" 的 *Conn bean（autowire 名即配置实例名）。
    gs.Provide(func(conn *StarterNats.Conn) messaging.Driver {
        return StarterNats.NewDriver(conn)
    }, gs.TagArg("main")).Export(gs.As[gs.Rooter]())

    gs.Provide(newConsumer).Export(gs.As[gs.Rooter]())
}

type Consumer struct {
    Sub messaging.Subscriber `autowire:"?"`
}

func newConsumer(b messaging.Driver) *Consumer {
    sub, err := b.NewSubscriber(context.Background(), "orders.created", "workers")
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
# --- nats（多实例：每个 spring.nats.<name> = 一个 *Conn bean）------------------
spring.nats.main.url=nats://127.0.0.1:4222
spring.nats.main.name=orders-service        # 参与治理标签 + 服务端连接名
spring.nats.main.jetstream.enabled=true     # 暴露 Conn.JetStream（同一连接）
# spring.nats.main.max-reconnects=-1        # -1 = 无限（默认 60）
# spring.nats.main.reconnect-wait=2s
# spring.nats.main.connect-timeout=5s
# 认证（各风格互不冲突，选一种）：
# spring.nats.main.username=... / password=...
# spring.nats.main.token=...
# spring.nats.main.creds-file=/etc/nats/app.creds
# spring.nats.main.nkey-file=/etc/nats/app.nk
# TLS：spring.nats.main.tls.enabled=true + ca-file/cert-file/key-file/...

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
govern.source.file.path=conf/govern.yaml
```

**conf/govern.yaml**（§4 演练使用）：

```yaml
govern:
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
  └─ gs.Module(gs.OnProperty("spring.nats"))              [starter.go:33]
gs.Run()
  ├─ 配置绑定：每个 spring.nats.<name> → Config（value tag；url 经 expr 校验 ≠ ""）
  ├─ 每个 name：Provide(newConn, IndexArg(1,ValueArg(name)), IndexArg(2,ValueArg(c)))
  │             .Name(name).Destroy(destroyConn).Caller(1)  [starter.go:36-40]
  ├─ newConn：driver 查找 → CreateClient（nats.Connect —— 快速失败探针；broker
  │           不可用则中断启动）[driver.go:122,137]
  ├─ 挂接插桩：pubObs/subObs（模块内 observe.go）
  │           [driver.go:155-156]
  ├─ jetstream.enabled → jetstream.New(nc)；失败会关闭 nc 并中断启动
  │           [driver.go:158-164]
  ├─ applyResilience：fault.WrapExecutor(resilience.ExecutorFor(resource)) 再包
  │           resilience.WrapExecutor —— 治理关闭时是透明 no-op executor
  │           [driver.go:166; command.go:162-174]
  ├─ Run / 就绪
  └─ SIGTERM：destroyConn → exec.Close()（错误在 Drain 后向上返回）再 Conn.Drain()
              —— 在途订阅收尾后关闭 socket [client.go:70-74]
```

快速失败：`nats.Connect` 同步完成首次拨号；url/认证错误或 broker 宕机会以
`failed to connect nats: <url>` 中断启动 [driver.go:122-126]。启动后断连会打 Warn
（`nats disconnected`）并由客户端自动重连 [driver.go:57-59]——`Healthy()` 反映实时的
`IsConnected()` 状态 [client.go:62-64]。

### 2.2 一次发布，逐层走读（driver 路径）

`pub.Publish(ctx, msg)` [driver.go:58-71]：

1. 信封 → `nats.Msg{Subject, Data, Header}`；`messaging.Message.Key` 经保留 header
   `x-msg-key` 透传（消费侧还原进 `Key`，不会泄漏进 `Headers`）[driver.go:59-73]。
   空 header map → nil header。
2. 压测标记：若 `traffic.IsLoadTest(ctx)`，写入 `X-LoadTest: 1`（canonical header 名），
   供消费侧识别合成流量 [driver.go:62-67]。
3. `Conn.PublishMsg` 覆写 [command.go:82-91]：`pubObs.Start(context.Background(),
   "publish", subject)` 打开 producer span + 时长/在途 metric + access log。
   ⚠ span 父是 `context.Background()`——发布 span 永远是新根，不延续调用方的活动
   trace（nats.PublishMsg 不携带 ctx，不改 API 无法修复）。消费侧的 trace 延续不受
   影响：consumer span 经注入的 `traceparent` header 延续本发布 span（第 4 步）。
   若需要把发布挂进调用方 trace，走带 ctx 的手动路径：
   `ctx, sp := StartPublishSpan(ctx, msg)` + 内嵌 `conn.Conn.PublishMsg(msg)`
   （只出 span，无 metric/access log）。
4. `injectW3C` 把 `traceparent` 写入 `msg.Header`，消费侧得以延续该 span
   [command.go:51-56, 87]。
5. 裸 `c.Conn.PublishMsg`（异步缓冲写——broker 确认前即返回；需确认用 Flush，
   见 [nats.go 文档](https://docs.nats.io/)）。
6. `sp.End(err)` 记录结果。

该路径拿不到的东西：resilience（保护走独立方法，见 §2.4）。

### 2.3 一次消费，逐层走读（driver 路径）

`sub.Subscribe(handler)` [driver.go:79-116]：handler 包进 `messaging.Recover`
（panic → error 路径，不冲垮 SDK goroutine）[driver.go:86-88]；group 非空时走
`QueueSubscribe`（竞争消费），否则 `Subscribe` [driver.go:105-110]。每条消息：

1. `startConsume` 从 `nm.Header` 提取 W3C `traceparent`，打开 consumer span
   （producer span 的子）+ metric + log [command.go:96-102; driver.go:91-93]。
2. `X-LoadTest` header 重新物化进 ctx 作为压测标记 [driver.go:94-96]。
3. `fromNatsMsg`：多值 NATS header 压平为单值（`Get` = 首值优先）
   [driver.go:138-149]。
4. handler 执行；`sp.End(err)` [driver.go:97-99]。Close = `Subscription.Unsubscribe`
   [driver.go:119-124]。

已记录缺口：**直接 `Conn.Subscribe` / JetStream 消费不被插桩**——只有 driver 回调打开
consumer span [command.go:28-32]。手动逃生口：`StartPublishSpan` /
`StartConsumeSpan` / `EndSpan` 只发 span（无 metric/log）[command.go:109-160]，
example/ 即如此使用。

### 2.4 保护机制（guard）——哪些受保护、哪些不受

NATS 没有可拒绝的 middleware（不像 redis Hook / http RoundTripper），因此 resilience
executor 只经**方法**式选装入口触达 [command.go:152-160]：

| 入口 | Observe | Resilience guard |
|---|---|---|
| `Conn.PublishMsg`（含 driver 发布） | span+metric+log | **否** |
| `Conn.Publish` / `Conn.Request` / `Subscribe` / `QueueSubscribe` / JetStream | **否** | **否** |
| `Conn.PublishGuarded(ctx, subj, data)` | span+metric+log（经 `PublishMsg`） | 是 |
| `Conn.RequestGuarded(ctx, subj, data, timeout)` | **否** | 是 |

`applyResilience` 内部包裹顺序 [command.go:162-174]：`ExecutorFor(resource)`（治理中心
背书；治理关闭时透明 no-op）→ `fault.WrapExecutor`（故障注入）→
`resilience.WrapExecutor(exec, "nats")`（为熔断跳闸/拒绝/重试发
span/counter/histogram——resilience 核心自身不发）。拒绝时 guarded 调用返回
resilience 哨兵错误（`ErrRateLimited` / `ErrCircuitOpen`），底层发布/请求不会被调用
——[resilience_test.go:63-84] 有证明。`resource` 是 `nats:<name>` (colon format; falls back to `nats:<url>` when name unset)（按连接而非
按 subject）[driver.go:166]，限流/熔断状态在同一连接的全部 subject 间共享。

`PublishGuarded` 接收调用方 ctx（透传给 executor；生产者 span 本身仍按上方 PublishMsg
限制从 `context.Background()` 起始）。`RequestGuarded` 接收调用方 ctx，但 timeout 是
`Request` 内部的每次尝试超时。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.nats.<name>.*`。分组 key `tls` 绑定嵌套共享
struct——其子 key 属于 tlsconf，不属于本 starter。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `url` | string | — | **必填**（`expr:"$ != ''"` [config.go:31]）；逗号分隔多服务器交给 `nats.Connect`。 | 缺失/空 → 启动期绑定错误。 |
| `name` | string | "" | 服务端连接名 + **治理 resource label 组成部分** [driver.go:166]。 | 空名可用；label 用 ""。 |
| `username` / `password` | string | "" | 都设置 → `nats.UserInfo`。与其他认证风格正交 [driver.go:60-62]。 | 只设 username → 发送空密码。 |
| `token` | string | "" | → `nats.Token` [driver.go:63-65]。 | 与 username 并用 → nats option 后者覆盖（NATS 定义）。 |
| `creds-file` | string | "" | JWT+nkey seed 文件 → `nats.UserCredentials` [driver.go:66-68]。 | 路径错误 → 启动失败。 |
| `nkey-file` | string | "" | nkey seed → `nats.NkeyOptionFromSeed`；加载失败带解释并中断启动 [driver.go:69-76]。 | 坏 seed → 启动报错。 |
| `tls` | group | 关 | `tls.enabled=true` → `tlsconf.BuildClient()` → `nats.Secure`；BuildClient()==nil → 裸 `nats.Secure()` [driver.go:77-88]。⚠ 用 `BuildClient()` 而非 `BuildServer()`——与其他 client starter 同样的 client-TLS 姿态。子 key：`enabled`/`ca-file`/`cert-file`/`key-file`/`insecure-skip-verify`/`server-name`（tlsconf 的 tag）。 | TLS 不匹配 → 启动期连接错误。 |
| `max-reconnects` | int | 60 | → `nats.MaxReconnects`；-1 = 无限 [config.go:63; driver.go:49]。 | -1 且 broker 宕机 → 永久重连循环（设计如此）。 |
| `reconnect-wait` | duration | 2s | 重连尝试间隔 [driver.go:50]。 | 过小 → 对宕机集群高频重连。 |
| `connect-timeout` | duration | 5s | 仅约束**首次拨号** [driver.go:51]。 | 过小 → 慢网络误判启动失败。 |
| `jetstream` | group | — | `enabled` 的容器。 | — |
| `jetstream.enabled` | bool | false | 在**同一**连接上派生 `jetstream.New(nc)`；失败关闭 nc 并中断启动；否则 `Conn.JetStream` 保持 nil [driver.go:157-164]。 | 对未开 `-js` 的 broker 启用 → 启动错误。 |
| `driver` | string | DefaultDriver | driver 注册表查找；未知名字以 `nats driver not found` 中断启动 [driver.go:140-143]。⚠ 重复 `RegisterDriver` 在 init 期 panic。 | 拼写错误 → 启动报错。 |

grep 核对：本 starter Go 文件中 15 个去重 `value:` tag 恰为 `url`、`name`、`username`、
`password`、`token`、`creds-file`、`nkey-file`、`tls`、`max-reconnects`、
`reconnect-wait`、`connect-timeout`、`jetstream`、`jetstream.enabled`
（即 `${enabled:=false}`）、`driver`——全部在表内；两边无多余项。

---

## 4. 验证与故障演练

### 4.1 broker 宕机快速失败

```bash
docker stop nats
go run .          # 启动中断："failed to connect nats: nats://127.0.0.1:4222"
```

启动后演练：`docker stop nats` → Warn `nats disconnected`，应用存活，`Healthy()` 返回
false；恢复 broker → Info `nats reconnected to ...` [driver.go:57-65]。

### 4.2 guarded vs 未 guarded 路径

治理开启且限流收紧（§1 govern.yaml）时：

- `conn.Publish("s", b)`——始终成功（无 guard）[client.go:33-39]。
- 热循环里调 `conn.PublishGuarded(ctx, "s", b)` → burst 耗尽后以
  `resilience.ErrRateLimited` 拒绝，且被拒调用不会触达 socket
  [resilience_test.go:63-77]。
- 熔断演练：让发布持续失败（启动后停 broker）直到 `error-threshold` 跳闸 → 后续
  guarded 调用返回 `ErrCircuitOpen`，不执行发布 [resilience_test.go:80-102]。

### 4.3 治理标签核对

executor 的 resource 是 `nats:<name>` (colon format; falls back to `nats:<url>` when name unset) [driver.go:166]。把 govern.yaml 规则限定到
`nats:orders-service`（或前缀），策略即精确落到该连接。验证：wrapped-executor
的拒绝会发 span + counter（`system="nats"`，由 resilience-observe 桥命名）
[command.go:170-172]——演练后去 trace/metric 里 grep `nats`。

### 4.4 消息往返与 driver 映射存活

发布带 `Payload` + `Headers{"tenant":"acme"}` 的信封；消费侧断言：

- `m.Payload` 逐字节存活。
- `m.Headers["tenant"]` 存活（string→nats.Header→string）。
- tracing 生效时 `m.Headers["traceparent"]` 存在（§2.2 第 4 步注入）。
- `m.Key` 经保留 header `x-msg-key` 往返（`m.Headers` 中不可见）。
- 多值 header：仅首值存活。

### 4.5 指标 / span / 日志读取

- access log：每次被插桩的发布/消费一条结构化记录（默认 `brief`；`detailed` 追加
  至多 `maxArgBytes` 的 payload 字节）。
- metrics：messaging 时长 histogram + 在途 gauge，`system="nats"`、`destination` =
  subject——`curl -s :9370/metrics | grep -i nats`。
- span：producer `publish <subject>`（SpanKind producer），consumer
  `consume <subject>` 经 W3C header 关联；[example-otel/](example-otel/) 附带完整
  Jaeger 校验（`http://127.0.0.1:16686/api/traces?service=...`）。
- 连接事件：async error / disconnect / reconnect / close 落在日志 tag `app_def`
  [driver.go:52-65]。
- 停机：Drain 让在途订阅收尾；`exec.Close()` 的错误在 Drain 之后返回给
  destroy 钩子 [client.go:71-73]。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| 启动中断 `failed to connect nats` | broker 宕机 / `url` 错 / 认证被拒（首次拨号快速失败）[driver.go:122-126] | 启动 broker（`docker run ... nats:2.10 -js`），修 `url`/认证。 |
| 启动中断 `failed to create jetstream context` | `jetstream.enabled=true` 但 broker 未开 `-js` [driver.go:158-164] | 服务端加 `-js` 或关掉该 key。 |
| 启动中断 `nats driver not found` | `driver` 拼写错误 / 自定义 driver 未在 init 前注册 [driver.go:140-143] | 修名字或在更早的 init 注册。 |
| `Conn.JetStream` 为 nil | 未设 `jetstream.enabled` | 设置它；JS 在启动期从同一连接派生。 |
| 消费者能收消息但消费侧无 trace/metric | 用了裸 `Conn.Subscribe` 而非 driver（已记录缺口 [command.go:28-32]） | 走 messaging.Driver，或手写 StartConsumeSpan。 |
| guarded 调用突然报哨兵错误 | 限流耗尽或熔断打开——设计行为 [command.go:177-186] | 检查 govern.yaml 策略；熔断冷却后自愈。 |
| producer trace 不链接调用方 span | PublishMsg span 父是 `context.Background()` [command.go:79-91]——这是既定行为：nats.PublishMsg 无 ctx 参数，发布 span 必为新根 | 不改 API 无法修复；消费侧延续仍经 traceparent header 生效。需要挂进调用方 trace 时用手动路径 `StartPublishSpan(ctx, msg)` + 内嵌 `PublishMsg`。 |
| 第二个 header 值丢失 | driver 多值 header 压平取首值（单值信封） | 额外值放 payload，或用裸 Conn API。 |
| 日志出现重连风暴 | 集群宕机时 `reconnect-wait` 过小 | 调大；重连本就是客户端的可靠性机制。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 14 个 starter 本地 value tag（+ tls 分组子 key 在 tlsconf） |
| 必填 | 1（`url`） |
| quickstart 前置外部依赖 | 1（nats；collector 可选用于可观测） |
| "注意/坑"条数 | 6 |

设计嫌疑（审计台账）：

- **未解**：发布 span 的父是
  `context.Background()`（已文档化的限制——nats.PublishMsg 无 ctx，手动替代见 §2.2
  第 3 步）；直接 `Conn.Subscribe` / JetStream 面
  的消费无 trace；多值 header 压平取首值；README 的"没有包装连接"论据段与现状
  （已覆写插桩的 `Conn.PublishMsg`）矛盾；guarded 方法用 `context.Background()`
  （无截止/取消）且无 observe 插桩；starter 未接线 health.Indicator
  （`Healthy()` 仅调用方自接）。
- **已解**：上一版台账中的嫌疑均仍开放，暂无已修复项。
