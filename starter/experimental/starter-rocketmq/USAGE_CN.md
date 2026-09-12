# starter-rocketmq 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均与 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`、`driver.go`）及可运行的
[example/](example/) / [example-otel/](example-otel/) 核对——文中方括号为 file:line 锚点。
**RocketMQ 语义（topic、消费组、tag、重试、clustering/broadcasting）见
[rocketmq-client-go 官方文档](https://github.com/apache/rocketmq-client-go) 与
[RocketMQ 官方文档](https://rocketmq.apache.org/docs/)**——本文只写 go-spring 的增量。
客户端库为 `github.com/apache/rocketmq-client-go/v2`（remoting 客户端，非 5.x gRPC 的
`rocketmq-clients`）；选型理由见 DESIGN.md §4。

**激活条件**：出现任意 `spring.rocketmq.instances.*` 配置（模块注册 `OnProperty("spring.rocketmq")`
前缀匹配 [starter.go:40]）。每个 `spring.rocketmq.instances.<name>` 条目创建一个名为 `<name>` 的
`*StarterRocketmq.Client` bean。

---

## 1. 完整工程示例

一个经 driver（broker 中立信封）收发的服务，附裸 SDK 逃生口、actuator 与 OTel。文件树：

```
demo/
├── go.mod
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/apache/rocketmq-client-go/v2 v2.1.2
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-rocketmq   latest
    go-spring.org/starter-actuator   latest   // 可选：/metrics 挂载
    go-spring.org/starter-otel       latest   // 可选：真实 trace/metric 导出
    go-spring.org/starter-governance latest   // 可选：resilience/fault 策略
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** —— 业务消息走 driver，一次关键发送走 guarded 裸路径：

```go
package service

import (
    "context"

    "github.com/apache/rocketmq-client-go/v2/primitive"
    "go-spring.org/cloud/messaging"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
    StarterRocketmq "go-spring.org/starter-rocketmq"
)

type Service struct {
    // 包装 bean，名字来自配置条目（spring.rocketmq.instances.a）。
    Client *StarterRocketmq.Client `autowire:"a"`
}

func init() {
    gs.Provide(func(s *Service) (gs.Runner, error) {
        driver := StarterRocketmq.NewDriver(s.Client)

        sub, err := driver.NewSubscriber(context.Background(), "orders", "order-workers")
        if err != nil {
            return nil, err
        }
        if err := sub.Subscribe(context.Background(), func(ctx context.Context, msg *messaging.Message) error {
            log.Info(ctx, "order received", log.String("key", msg.Key))
            return nil // nil = ConsumeSuccess；error = ConsumeRetryLater（broker 重投）
        }); err != nil {
            return nil, err
        }

        return func(ctx context.Context) {
            pub, err := driver.NewPublisher(ctx, "orders")
            if err != nil {
                log.Error(ctx, "publisher", log.Any("error", err))
                return
            }
            _ = pub.Publish(ctx, &messaging.Message{
                Key:     "order-42",
                Payload: []byte(`{"id":42}`),
                Headers: map[string]string{"from": "demo"},
            })
            _ = pub.Close()

            // 关键发送：唯一被 resilience 保护的路径（§2.2）。
            p, _ := s.Client.NewProducer()
            msg := primitive.NewMessage("orders", []byte("urgent"))
            _, err = StarterRocketmq.GuardedSend(ctx, s.Client, p, msg)
            log.Info(ctx, "guarded send done", log.Any("error", err))
        }, nil
    })
}
```

**conf/app.properties**：

```properties
# --- rocketmq --------------------------------------------------------------
spring.rocketmq.instances.a.name-servers=127.0.0.1:9876
spring.rocketmq.instances.a.send-timeout=5s
spring.rocketmq.instances.a.fail-fast=true          # 启动期对 name server 做 TCP 探测
# spring.rocketmq.instances.a.access-key=...         # ACL：必须与 secret-key 成对
# spring.rocketmq.instances.a.secret-key=...

# --- actuator + otel -------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090

# --- governance（rocketmq:127.0.0.1:9876 的限流/熔断）---------------------
govern.source.file.path=conf/govern.yaml
```

**启动 RocketMQ**（复制 [example/docker-compose.yml](example/docker-compose.yml) 与
[example/broker.conf](example/broker.conf)；`brokerIP1=127.0.0.1` + `listenPort=10911`
的广播地址组合与 `autoCreateTopicEnable=true` 对宿主机侧客户端很关键）：

```bash
docker compose -p gs-rocketmq-demo up -d        # namesrv :9876 + broker :10911
docker exec rmqbroker sh mqadmin updateTopic -n namesrv:9876 -c DefaultCluster -t orders
go run .
```

⚠ push consumer 的 Subscribe 会立刻查询 topic 路由信息，topic 不存在即失败；而
`autoCreateTopicEnable` 只在首次**生产**时生效——应用启动前必须先建 topic（这也是
[example/check.sh](example/check.sh) 以 `mqadmin topicList` 可见性做门禁的原因）。

**验证**：

```bash
grep _app_rocketmq_access app.log | tail -2   # driver publish/consume 记录
curl -s :9090/metrics | grep messaging_client_operation_duration
curl -s :9370/healthz
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-rocketmq
  └─ gs.Module(OnProperty("spring.rocketmq"))：出现任意 spring.rocketmq.instances.* 即触发
        └─ conf.BindEach("${spring.rocketmq}") → 每个 <name> 条目一份 Config  [starter.go:40-48]
              └─ Provide(newClient).Name(<name>).Destroy((*Client).Close)     [starter.go:42-45]

gs.Run()
  ├─ 构造 newClient [starter.go:57]：
  │    1. access-key/secret-key 成对校验（单边 → 启动失败）                  [starter.go:60-62]
  │    2. Driver.CreateClient —— 可选的 Driver bean（由
  │       ${spring.rocketmq.instances.<name>.driver} 按实例选择：留空 = 按类型注入，配置 = 按
  │       bean 名注入，指定的 bean 不存在则启动失败），未提供时回退到内置
  │       DefaultDriver：安装 rlog→go-spring 日志桥接
  │       （进程级全局，sync.Once 仅一次）                                    [driver.go:94-96]
  │    3. FailFast 探测：TCP dial，首个可达地址即通过，每地址 3s 预算；
  │       失败 → 启动失败                                                     [driver.go:146-160]
  │    4. applyResilience：fault.WrapExecutor(resilience.ExecutorFor("rocketmq", resource))
  │       → 挂到 Client                                                    [command.go:156-161]
  ├─ 应用按 `autowire:"<name>"` 注入 *Client
  ├─ 应用自行随时创建 producer/consumer/driver（均在锁内注册到 Client）       [client.go:104-157]
  └─ SIGTERM → Client.Close：先 closeResilience，再逐个 Shutdown 已注册的
     producer 与 consumer；shutdown 错误只记日志、不返回                     [client.go:162-184]
```

源码核对要点：

- 探测是 TCP dial，**不是** broker 往返——能抓地址写错，抓不到 ACL/凭证错误
  （DESIGN.md §3；探测循环 [driver.go:148-155] 首个成功即返回）。
- producer 创建即启动（NewProducer 内 `p.Start()` [client.go:116]）；consumer 返回
  **未启动**——需自行先 Subscribe 后 Start，或交给 driver [client.go:130-135]。
- Close 与并发 NewProducer/NewPushConsumer 竞争时，新来者被 shutdown 并返回
  `errClosedClient` [client.go:122-125]；Close 之后两个构造函数直接失败
  [client.go:105-110]。
- **没有 health.Indicator**（家族统一决策；见 DESIGN.md §3）。

### 2.2 守护机制 —— 精确包裹顺序与未守护范围

```
GuardedSend(ctx, cl, producer, msg)                      [command.go:208]
  └─ cl.execute                                           [client.go:188]
       └─ exec.Execute(ctx, "rocketmq:<name-servers>", call)     — resilience.Executor
            applyResilience 中的由外向内组合顺序 [command.go:157]：
            fault.Injector（外）→ resilience 观察器（6 种 outcome）
            → resilience 核心（限流/熔断/...）→ producer.SendSync
```

- executor 只包裹 `GuardedSend` 的同步 `SendSync`。**未守护**：driver 的 `Publish`
  （直接调 `p.p.SendSync` [driver.go:98]）、裸 `SendSync`，以及有意不碰的
  `SendAsync`/`SendOneWay` [command.go:205-207]。consume 路径完全不经过 executor。
- 资源标签为 `rocketmq:<name-servers>`——逗号拼接的 `name-servers` 列表，与 kafka
  starter 同一约定；两个指向同一 name-server 集群的配置条目共享同一个治理资源
  [starter.go:80, resilience/config.go:151-158]。
- governance 关闭时 `ExecutorFor` 返回透明直通，`GuardedSend` 与 `SendSync` 行为完全一致
  [command.go:176-179, client.go:189-191]（TestExecutePassThrough 证明
  [rocketmq_test.go:47-53]）。
- 拒绝（限流/熔断开启）返回 resilience 哨兵错误，**发送不会被调用**（
  TestExecuteRateLimit 证明 [rocketmq_test.go:57-69]）。

### 2.3 一次 publish 逐层走读（driver 路径）

`pub.Publish(ctx, &messaging.Message{Key, Payload, Headers})` [driver.go:84-101]：

1. 为 publisher 的固定 topic 构建 `primitive.Message`；`Key`（单个字符串）经 `WithKeys`
   成为消息 key [driver.go:87-88]；每个 Header 写入 user property [driver.go:89-91]。
2. 若 ctx 携带压测标记，写入 `x-loadtest=1` user property [driver.go:92-96; traffic.go:60]。
3. `startProduce` 打开 starter 自带的生产观测（"publish"，producer span）并把 W3C
   traceparent 注入 user properties [observe.go]——无 starter-otel 时为 no-op。
4. 裸 `SendSync`（绕过 resilience executor——见 §2.2）。
5. `sp.End(err)` 记录时长直方图、平衡 in-flight 计数、结束 span 并输出
   `_app_rocketmq_access` 日志记录 [observe.go]。

### 2.4 一次 consume 逐层走读（driver 路径）

SDK push-consumer 协程回调 starter 的 handler [driver.go:125-141]：

1. Subscribe 时已用 `messaging.Recover` 预包裹：handler panic 转为普通错误
   （nack/重投），不会 unwind 进 SDK 协程 [driver.go:119]。
2. 每条消息：`startConsume` 从 user properties 提取上游 trace 并打开 "consume" 观测
   [command.go:160-163]。
3. `x-loadtest` property 映射回 ctx，handler 里 `traffic.IsLoadTest(ctx)` 为真
   [driver.go:131-133]。
4. `fromMessageExt` 构建信封：`Key` 取 KEYS property、`Payload` = body、
   `Headers` = **全部** user properties、`Timestamp` 取 StoreTimestamp [driver.go:154-161]。
5. handler 出错 → Error 日志 + `ConsumeRetryLater`（broker 按 RocketMQ 重试语义重投）；
   成功 → `ConsumeSuccess` [driver.go:136-141]。

已知映射丢失（双向）——均在 driver.go 核对：

| 字段 | Publish（信封 → RocketMQ） | Consume（RocketMQ → 信封） |
|---|---|---|
| Key | 单字符串 → 单个消息 key [driver.go:87] | KEYS property 读回为一个字符串；多 key 生产方得到拼接值而非原列表 [driver.go:156] |
| Headers | 条目原样写入 user properties [driver.go:89] | 返回**全部** properties，含 SDK 内部项（`traceparent`、KEYS、`x-loadtest`）——会漏进 `msg.Headers` [driver.go:158] |
| Tags | **无映射**——`messaging.Message` 无 tag 概念；driver 发的是无 tag 消息 | **无映射**——订阅硬编码选择器 `TAG *` [driver.go:122-124]；按 tag 过滤消费需走裸客户端 |
| Timestamp | 不发送 | StoreTimestamp（broker 存储时间），非 BornTimestamp [driver.go:159] |
| Topic | 在 NewPublisher 时固定 | 不回填到信封 |

能干净存活的双向字段：Payload、自定义 Headers、Key（单个）、trace context——正是
[example/example.go] 断言、TestFromMessageExt 覆盖的内容 [rocketmq_test.go:102-117]。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.rocketmq.instances.<name>..` 下（经 `conf.BindEach` 的实例前缀绑定，不是
绝对属性的 Pool 规则）。starter 内 7 个 value tag——已与 grep 结果比对，两边均无多余项。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `name-servers` | []string | — | **必填**（`expr:"len($) > 0"` [config.go:35]）。NameServer 地址列表；同时供 fail-fast 探测使用。 | 缺失/为空 → 绑定期校验错误。 |
| `instance-name` | string | `""` | 区分同主机多客户端。留空安全：SDK 会把 "DEFAULT" 改写为每个 producer/consumer 唯一名（DESIGN.md §3）。设置后 remoting 客户端共享一个连接池。 | 多余的显式值 → 想隔离时却共享了池。 |
| `access-key` | string | `""` | ACL access key。⚠ 必须与 `secret-key` 成对——单边启动即失败，错误信息带客户端名 [starter.go:60-62]。 | 单边 → 启动错误；值错误 → 首次收发失败（探测只到 TCP 层）。 |
| `secret-key` | string | `""` | 与 access-key 配对的 ACL secret key [config.go:48]。 | 同上。 |
| `send-timeout` | duration | `3s` | 套到每个 producer（`WithSendMsgTimeout`）[client.go:73]。 | 过小 → 高压下同步发送超时。 |
| `retry` | int | `2` | producer 内部重试次数（`WithRetry`）；2 = 最多 3 次尝试 [client.go:74, config.go:55]。⚠ 注意 governance 侧重试——两个循环都会生效。 | 大值 + 慢 broker → 延迟放大。 |
| `fail-fast` | bool | `true` | bean 创建期对 name server 列表 TCP dial；首个可达地址即通过，每地址 3s [driver.go:146-160]。 | 关闭 → 地址错误延迟到首次使用才暴露。 |

---

## 4. 验证与故障演练

### 4.1 含 driver 字段存活的消息往返

运行 [example/check.sh](example/check.sh)（compose 带 `-p` 隔离、topic 创建、成功标记门禁
——各门禁存在的原因见脚本注释），或手动：

```bash
go run .                                  # 期望输出 "Response from server: value"
```

example 断言 Payload 与自定义 Headers 往返存活 [example/example.go:110-120]。要自行断言
Key 存活，可在 handler 里打印 `msg.Key`——应得到同一个字符串。

### 4.2 broker 宕机 fail-fast

```properties
spring.rocketmq.instances.a.name-servers=127.0.0.1:19876   # 无监听
```

启动失败并报 "rocketmq name server probe failed on ..." [starter.go:74-79]。设
`fail-fast=false` 后同样配置可正常启动，首次使用才失败。注意探测只证明可达性——
TCP 可达但 ACL 错误照样能启动。

### 4.3 guarded 与 unguarded 对比

对资源 `rocketmq:127.0.0.1:9876` 配 governance 限流策略（标签 = `rocketmq:` + 逗号拼接的
name-servers——须与 govern 规则一致）：

- 压 `GuardedSend` → 拒绝返回 `resilience.ErrRateLimited`，发送未被调用，出现
  `resilience.outcome=rate_limited` 的 `_app_rocketmq_resilience` 记录
  [cloud/governance/resilience/observe.go:66-78]。
- 同策略下压 driver 的 `Publish` → 毫无反应：该路径绕过 executor（§2.2）。
  这个不对称正是本演练要验证的点。

### 4.4 governance 标签核对

```bash
curl -s :9090/metrics | grep resilience_calls
# resilience.resource / resource 标签必须是 "rocketmq:127.0.0.1:9876"——
# name server 列表，而非配置条目名（§2.2）
```

### 4.5 metrics / span / 日志读取

- driver 观察器：直方图 `messaging.client.operation.duration`（单位 s）与 up-down 计数器
  `messaging.client.active_requests`，属性 `messaging.system=rocketmq`、
  `messaging.operation=publish|consume`（直方图另有 `status`）[observe.go]。访问日志 tag
  `_app_rocketmq_access`：字段 `operation`、`destination`（截断至 512）、`duration_ms`；
  带目的地成功 Debug、无目的地成功 Info、出错 Warn。
- guarded 路径：计数器 `resilience.calls` / `resilience.breaker.state_change`，日志 tag
  `_app_rocketmq_resilience`。
- 手动 helper（裸客户端）：tracer `go-spring.org/starter-rocketmq` 产出 span
  `rocketmq.produce` / `rocketmq.consume <topic>`，属性 `messaging.system/
  destination.name/operation` [command.go:64-97]；W3C context 经 `msgCarrier` 随
  user properties 传播（往返由 TestMsgCarrierRoundTrip 证明 [rocketmq_test.go:74-98]）。
  [example-otel](example-otel/main.go) 验证关联 span 落入 Jaeger（`:16686`，服务名
  `rocketmq-otel-example`）。
- 压测标记演练：在带流量层标记的 ctx 下 publish；consumer handler 的
  `traffic.IsLoadTest(ctx)` 经 `x-loadtest` property 变为真 [driver.go:92-96, 131-133]。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| 启动失败 "name server probe failed" | `name-servers` 错误/不可达 | 修正列表；探测按每地址 3s 预算逐个 dial [driver.go:149]。 |
| 启动失败 "access-key and secret-key must be set together" | ACL 单边配置 [starter.go:60] | 成对设置或都留空。 |
| 建完 topic 立刻 Subscribe 失败 | 路由尚未在 name server 可见（心跳滞后，最长 60s） | 应用启动前以 `mqadmin topicList -n namesrv:9876` 做门禁（见 check.sh）；example-otel 的 Subscribe 20×500ms 重试同理。 |
| consumer 静默收不到消息 | Subscribe 时 topic 不存在，或消费组不对 | 提前建 topic；记住消费组是 competing-consumers 单元。 |
| driver 发布毫无 resilience 效果 | Publish 按设计绕过 executor | 需要保护的发送用 `GuardedSend`（§2.2）。 |
| 一切正常但没有 trace | 未 import starter-otel | 所有 OTel helper 无它是静默 no-op [command.go:46-48]。 |
| 看不到 SDK 连接/rebalance 日志 | 级别配置过滤 | 它们经桥接以 `rocketmq:` 前缀、tag `app` 到达 [driver.go:124-139]；Fatal 映射为 Error [driver.go:114-116]。 |
| consumer 不停重投 | handler 持续返回 error → ConsumeRetryLater | 修 handler；driver 无 DLQ 接线——需要时用裸客户端配合 RocketMQ 重试/DLQ 语义。 |
| 重新部署后重复消费 | 同组 rebalancing；信封时间戳基于 StoreTimestamp | RocketMQ clustering 的预期行为；见官方消费组文档。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 7（全部在 Config） |
| 其中必填 | 1（`name-servers`） |
| quickstart 前置外部依赖 | 2（namesrv + broker，同一 compose） |
| "注意/坑"条数 | 6 |

设计嫌疑（审计台账；除注明外均承接自上一版）：

- 无 TLS 配置面，尽管 SDK 支持（承接，未修）。
- driver Subscribe 硬编码 `TAG *` 选择器；不支持 tag 子表达式，且
  `messaging.Message` 根本无法携带 tag（§2.4 表）（承接，未修）。
- driver Timestamp 用 StoreTimestamp 而非出生时间（承接，未修）。
- driver 发布不经过守护而裸路径可以——两条生产路径在保护/可观测上不对称
  （承接，未修）。
- 消费信封把 SDK 内部 user properties（`traceparent`、KEYS、`x-loadtest`）漏进
  `Headers`，多 key 往返返回拼接字符串（§2.4；新增）。
- 资源标签的 name-servers 段是死代码——`ResourceLabel` 在客户端名处即返回，
  传入的地址列表永远不可能出现在标签里（§2.2；新增；上一版文档的
  `rocketmq|<name>|<name-servers>` 标签说法有误，本文已更正）。
- 已修复：截至本文撰写，历史嫌疑均未在代码中处理。
