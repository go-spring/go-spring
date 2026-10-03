# starter-pulsar 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均与 starter 源码
（`config.go`、`starter.go`、`client.go`、`command.go`、`driver.go`、`driver.go`）及可运行的
[example/](example/) / [example-otel/](example-otel/) 核对，括号内为 file:line 抽查点。
**Pulsar 自身语义（订阅、消息 key、properties、保留/重投）见
[Pulsar 官方文档](https://pulsar.apache.org/docs/next/client-libraries-go/)** —— 下文只写
go-spring 的增量。

**激活条件**：出现任意 `spring.pulsar.instances.*` 配置 —— 模块注册于
`gs.OnProperty("spring.pulsar")`（前缀匹配）[starter.go:38]。每个 `spring.pulsar.instances.<name>`
条目创建一个**裸 `pulsar.Client` bean，名为 `<name>`** [starter.go:39-44] —— 设计上没有
包装类型：pulsar 除了 client 本身没有值得包裹的实体 [client.go:17-23]。

---

## 1. 完整工程示例

一个经 messaging.Driver 同时做生产与消费的服务，含原生指标、OTel tracing 与治理。
文件树：

```
demo/
├── go.mod
├── main.go
├── messaging.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/apache/pulsar-client-go latest
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-pulsar       latest
    go-spring.org/starter-actuator     latest   // 可选：readiness + OTel 指标挂载
    go-spring.org/starter-otel         latest   // 可选：真实 trace 导出
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "demo/messaging"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-pulsar"
)

func main() { gs.Run() }
```

**messaging.go** —— driver 发布 + 消费，以及受治理保护的裸路径：

```go
package messaging

import (
    "context"

    "github.com/apache/pulsar-client-go/pulsar"
    "go-spring.org/cloud/messaging"
    "go-spring.org/spring/gs"
    StarterPulsar "go-spring.org/starter-pulsar"
)

func init() {
    // 把注入的裸 client 适配为 broker 无关的 Driver。gs.TagArg("main")
    // 取 spring.pulsar.instances.main 实例；driver bean 此处未命名。
    gs.Provide(StarterPulsar.NewDriver, gs.TagArg("main"))

    gs.Provide(func(b messaging.Driver) (gs.Rooter, error) {
        // Publisher：driver 为 topic 惰性创建一个 producer。
        pub, err := b.NewPublisher(context.Background(), "persistent://public/demo/orders")
        if err != nil {
            return nil, err
        }
        // Subscriber："orders-group" 即 Pulsar Shared 订阅名。
        sub, err := b.NewSubscriber(context.Background(),
            "persistent://public/demo/orders", "orders-group")
        if err != nil {
            return nil, err
        }
        // handler 出错 → Nack → Pulsar 重投 [driver.go:147-151]。
        err = sub.Subscribe(context.Background(), func(ctx context.Context, m *messaging.Message) error {
            return process(ctx, m) // Key/Payload/Headers/Timestamp 均在往返中保留
        })
        if err != nil {
            return nil, err
        }

        return func(ctx context.Context) error {
            // driver 发布：driver 声明该次发布（topic/方向）并在 resilience executor 下执行，
            // 由 resilience 层发射 span、messaging.client.* 指标与访问日志，并把 W3C trace
            // 上下文注入消息 properties [messaging.go]。
            return pub.Publish(ctx, &messaging.Message{
                Key:     "user-42",                    // 成为 Pulsar 消息 key
                Payload: []byte(`{"amt":100}`),
                Headers: map[string]string{"trace-ctx": "biz"}, // 成为 Properties
            })
        }, nil
    })
}

// 受保护的裸路径：经 GuardedSend 的 Producer.Send 会被声明并受 resilience 包裹。
// GuardedSend 自己声明该操作，无需再套手动 span 助手 —— span 由 resilience 层发射。
func guarded(ctx context.Context, cl pulsar.Client, p pulsar.Producer) error {
    msg := &pulsar.ProducerMessage{Payload: []byte("x"), Key: "user-42"}
    id, err := StarterPulsar.GuardedSend(ctx, cl, p, msg)
    _ = id
    return err
}
```

**conf/app.properties** —— 上述用到的完整配置面：

```properties
# --- pulsar client（实例 "main"）---------------------------------------------
spring.pulsar.instances.main.url=pulsar://127.0.0.1:6650
spring.pulsar.instances.main.ping=true
# 对未分区 topic 的 lookup 即使 topic 不存在也会成功，
# 因此普通 topic 在全新 standalone 集群上是安全的探测目标。
spring.pulsar.instances.main.health-check-topic=persistent://public/demo/orders
spring.pulsar.instances.main.operation-timeout=30s
spring.pulsar.instances.main.connection-timeout=5s

# --- 原生 Prometheus 指标（pulsar_client_*，按实例独立 registry）-------------
spring.pulsar.instances.main.metrics.enabled=true
spring.pulsar.instances.main.metrics.port=9091
spring.pulsar.instances.main.metrics.path=/metrics

# --- actuator + otel ----------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # OTel 指标仅经 actuator 暴露

# --- 治理（服务 pulsar|pulsar://127.0.0.1:6650 的熔断/限流）-------------------
spring.governance.source.file.path=conf/governance.yaml
```

**启动 Pulsar**（同 [example/docker-compose.yml](example/docker-compose.yml)）：

```bash
docker run -d --name pulsar -p 127.0.0.1:6650:6650 -p 127.0.0.1:8080:8080 \
    apachepulsar/pulsar:3.2.0 bin/pulsar standalone
# 就绪闸门（6650 端口开放远早于 broker 可用 —— example/check.sh:36-44）：
for i in $(seq 1 60); do curl -fsS http://127.0.0.1:8080/admin/v2/brokers/health \
    | grep -qi ok && break; sleep 1; done
```

**验证**：

```bash
go run .                                   # broker 不可达时 ping 探测中止启动
curl -s :9091/metrics | grep pulsar_client_ # 原生 client 指标
curl -s :9370/metrics | grep messaging.client # 声明式操作指标（调用级 + 尝试级）
grep -E '_app_pulsar|pulsar' app.log        # driver + client 日志行（tag _app_def）
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-pulsar
  └─ gs.Module(OnProperty("spring.pulsar")) 在任意 spring.pulsar.instances.* key 存在时触发
        └─ conf.BindEach("${spring.pulsar}") → 每个 <name> 条目一份 Config     [starter.go:38-46]
              └─ Provide(newClient, IndexArg name+config, IndexArg 3,?Driver).Name(<name>)
                   .Destroy(destroyClient)

gs.Run()
  ├─ ctor newClient [starter.go:89]：
  │    1. 可选 Driver bean —— 无则内置 DefaultDriver                      [starter.go:92-95]
  │    2. d.CreateClient(c, cloud.ClientParams{Resilience: mgr, Fault: inj})：
  │       ClientOptions、认证（mTLS>token 文件>token）、TLS、原生
  │       Prometheus registry + :port /metrics server、日志桥、
  │       pulsar.NewClient，随后 attachGuard —— params.ExecutorFor("pulsar",
  │       "pulsar:<url>") 按 client 索引，客户端在此刻已完整；mgr/inj 即注入的
  │       *resilience.Manager / *fault.Injector        [driver.go:76-115, command.go:225]
  │    3. 再探测（Ping）：cl.TopicPartitions(HealthCheckTopic) —— 一次
  │       覆盖地址+认证+TLS 的 lookup（不产生消息）；失败 →
  │       closeResilience + cl.Close + metrics 下线 + 启动报错            [starter.go:108-116]
  ├─ 就绪：无 health indicator —— 探测只在启动期生效
  └─ SIGTERM → destroyClient [client.go:45]：closeResilience（executor Close）
       → cl.Close()（释放全部 producer/consumer）→ shutdownMetrics（:port server）
```

**装配扩展点**：client 装配由 `Driver`（接口，`driver.go:62-72`）负责：
`CreateClient(ctx, c Config, params cloud.ClientParams) (pulsar.Client, error)`。driver 返回的
client 已完整——它在构建时就自行挂上治理 executor（`attachGuard`），此后没有任何补丁步骤。
公司/伞包 starter 可把自己的 `Driver` 作为**可选容器 bean** 提供
（`gs.Provide(func() StarterPulsar.Driver{...})`，因为是 bean，可在装配期注入从配置文件绑定的
配置）；`spring.pulsar` 下每个实例都经它构建，自定义 driver 在 return 前调用 `attachGuard`
（同包）挂上 executor。没有该 bean 时 starter 在装配内回退到内置 `DefaultDriver`
（`driver.go:74-115`，`starter.go:92-95`）。当容器中存在多个 Driver bean 时，实例可按名指定：
`spring.pulsar.instances.<name>.driver = <bean 名>`（留空 = 先回退家族级 `spring.<family>.default.driver`，再按类型注入唯一 Driver bean；指定的
bean 不存在则启动失败）。

注意 `newLogger()` 把 pulsar 内部日志（连接/重连/lookup 失败）桥接进 go-spring 日志，
tag 为 `_app_def`，前缀 `pulsar: ` [driver.go:208-223]。

### 2. guard/wrap 机制 —— 精确包裹顺序与未保护面

ctor 里挂上的 resilience executor 只经**一个 seam** 驱动，且每次调用进入前都先**声明**其
操作 —— starter 声明，resilience 层发射：

```
GuardedSend(ctx, cl, producer, msg)                       [command.go]
  ├─ observability.WithOperation(ctx, operation("publish", producer.Topic()))
  │     —— 声明方向（span 名）、messaging.system/operation 标签，
  │        以及作为 Detail 的 topic（永不成为 label）
  └─ guard: clientGuards.Load(cl)                         [command.go]
       ├─ 未找到（driver 未调 attachGuard）→ producer.Send 原样内联执行
       └─ 找到 → exec.Execute(ctx, injectW3C + send)      —— fault 注入器最外层
                  （治理开启时），resilience observer 在其内层；由声明发射
                  span + messaging.client.* 指标 + 访问日志；拒绝时返回
                  resilience 哨兵错误，发送根本不会上线
```

消费方向同理：driver 的接收循环里声明 `operation("consume", source)`，再把 handler 放进
`guard` 执行 [messaging.go]。

`attachGuard` 挂上的 executor 来自 `params.ExecutorFor("pulsar", service)`（`service` =
`pulsar:<url>`）[command.go]，其中 `params` 是 ctor 交给 driver 的
`cloud.ClientParams{Resilience: mgr, Fault: inj}`（`mgr` 即注入的 `*resilience.Manager`）：
有 manager 时返回已完整组装的 executor（核心熔断/限流/重试外包 resilience observer——outcome
计数 + 访问日志），最外层再由注入的 `*fault.Injector` 做运行期故障注入（注入的错误穿过内层
重试循环，熔断器照常计数）；零值 bundle 时降级为仅观测、并高频告警的 unmanaged executor。

**未保护面**（均有源码注释说明是有意的）：
- `producer.SendAsync` —— 刻意不碰；异步路径没有可拒绝的同步结果 [command.go]。
- `CreateProducer`/`Subscribe`/`TopicPartitions` —— 生命周期调用，仅启动期 Ping
  探测覆盖。
- 手动 `StartProducerSpan`/`StartConsumerSpan` 助手 —— 供应用自行直连裸 client 发送时
  使用；经 `GuardedSend` 的发送已由 resilience 层依据声明开 span，不要对同一次发送两者都包。

### 2.3 一次 driver 发布，逐层走读

`pub.Publish(ctx, msg)`，其中 `Key: "k"`、`Headers: h`、ctx 带压测标记
[messaging.go:104-131]：

1. header 拷贝：若 propagator 的 `IsLoadTest(ctx)`，headers 被**复制**（绝不改调用方的 map）
   并追加 `x-load-test=1` [messaging.go:110-118]。
2. 信封 → `pulsar.ProducerMessage`：`Payload`、`Properties`（= headers）、**Key 仅在
   非空时设置** [messaging.go:119-125]。未映射：`Timestamp`（信封有，但 Pulsar 发布时间由
   服务端定）及一切 Pulsar 专有字段（OrderingKey、DeliverAt……）。
3. `GuardedSend` 声明该次发布（`operation("publish", producer.Topic())`）：方向决定 span
   名，`messaging.system`/`messaging.operation` 是有界的 label，topic 则是逐调用 Detail
   —— 只到 span 与日志，绝不成为 label。
4. `producer.Send(attemptCtx, pm)` —— 同步，阻塞到 broker ack；W3C trace 上下文从 attempt
   ctx 注入 `pm.Properties`，因此 traceparent 携带 executor 的 span。
5. resilience observer（唯一发射点）把结果记到 `messaging.client.operation.duration`
   （调用级）、`messaging.client.attempt.duration`（每次重试）与
   `messaging.client.active_requests`，并写一条访问日志。publish 与 consume 声明为
   `NonIdempotent`，故针对该 label 的重试策略会被**抑制**（每个 service 告警一次）：
   重发或再跑一遍 handler 是第二个副作用，不是第二次尝试。

### 2.4 一次 driver 消费，逐层走读

`sub.Subscribe(handler)` 启动一个后台循环 [messaging.go:156-196]：

1. handler 先包 `messaging.Recover` —— panic 转为普通错误 → Nack → 重投，绝不会
   打穿 SDK goroutine [messaging.go:159-161]。
2. 循环 ctx 派生自 `context.WithoutCancel(ctx)` —— 只有 Close 显式取消，调用方 ctx
   取消不影响 [messaging.go:157]。
3. `c.Receive` → 循环从 `msg.Properties()`（`extractTraceContext`）提取上游 W3C trace，
   把消费声明（`operation("consume", source)`）写到 handler ctx，再把 handler 放进
   `guard` 执行 —— resilience observer 依据声明开 consumer span、记录指标、写访问日志。
4. 压测标记：若生产者往 Properties 里写了 `x-load-test`，handler ctx 会经
   `prop.WithLoadTest(ctx)` 重新打标（即上面的 `prop.Extract`）。
5. `fromPulsarMsg` 反向映射：Pulsar `Key()` → 信封 Key、`Payload()`、`Properties()` →
   Headers（含注入的 `traceparent` —— 消费侧 headers 会多出 key）、`PublishTime()` →
   Timestamp [messaging.go]。两个方向都保留 Key 与 Properties。
6. handler 出错 → `Nack`（按 Shared 订阅语义重投）+ 错误日志；成功 → `Ack(msg)`，
   ack 失败记 WARN（有重投风险）[messaging.go]。
7. 非 ctx 取消的 `Receive` 错误记日志后循环重试 [messaging.go]。

Close 顺序：取消循环 ctx → 等 `done`（在途 handler 收尾）→ `consumer.Close()`
[messaging.go:198-207]；publisher Close 只调 `producer.Close()` 且**丢弃其错误**
[messaging.go:134-136]。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.pulsar.instances.<name>.`（BindEach 按实例前缀绑定，不是绝对属性的 Pool
规则）。grep 得到 18 个 value tag —— 下表全覆盖。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `url` | string | — | **必填**（`expr:"$ != ''"`）。`pulsar://` 明文或 `pulsar+ssl://` TLS。同时构成 resilience 服务标签 `pulsar\|<url>` [starter.go:76]。 | 缺失/空 → BindEach 启动报错。 |
| `operation-timeout` | duration | 30s | producer/订阅/lookup 超时，传给 ClientOptions [driver.go:72]。 | 过低 → CreateProducer 间歇失败。 |
| `connection-timeout` | duration | 5s | TCP 连接超时 [driver.go:73]。 | — |
| `token` | string | — | JWT token 值，或 `token-from-file=true` 时的文件路径 [driver.go:86-90]。⚠ 认证优先级：mTLS cert+key 高于 token。 | token 错 → `ping=true` 时 Ping 探测启动失败。 |
| `token-from-file` | bool | false | 把 `token` 切换为路径解释。 | true 配了字面 token → 文件打开失败。 |
| `tls-trust-certs-file` | string | — | 校验 broker 的 PEM CA bundle [driver.go:74]。 | `pulsar+ssl://` 下缺失 → 探测期握手失败。 |
| `tls-cert-file` | string | — | 客户端证书；与 `tls-key-file` 成对时还经 `NewAuthenticationTLS` 成为 mTLS 认证器 [driver.go:84-85]。⚠ 只配 cert 不配 key → 静默无认证。 | — |
| `tls-key-file` | string | — | 与证书配对的客户端私钥 [driver.go:76]。 | — |
| `tls-allow-insecure` | bool | false | 关闭服务端证书校验。生产禁用。 | true → MITM 暴露。 |
| `tls-validate-hostname` | bool | false | 证书内主机名校验；默认保持 pulsar-client-go 默认值 [config.go:63-66]。 | — |
| `ping` | bool | false | 可选启动期 `TopicPartitions` 探测 [starter.go:107-114]。 | true → broker 挂了则中止启动；false → 到首次生产才暴露。 |
| `health-check-topic` | string | `persistent://public/default/__health_check` | 探测目标；未分区 topic 的 lookup 即使不存在也成功 [config.go:72-76]。 | 分区/乱写 topic 名 → 探测报错挡启动。 |
| `metrics` | group | — | 结构绑定 `value:"${metrics}"` [config.go:79]。 | — |
| `metrics.enabled` | bool | true | 启动按实例的 `/metrics` server 并接入独立 registry [driver.go:97-101]。 | false → 任何地方都没有 `pulsar_client_*`。 |
| `metrics.port` | int | 9091 | 该 server 的端口。⚠ 固定默认：每个开 metrics 的实例必须各配独立端口；冲突时后起的 server 静默监听失败（错误被吞 [command.go:69-71]）。 | 两实例同端口 → 一个 metrics 端点静默死亡。 |
| `metrics.path` | string | `/metrics` | 该 server 的 HTTP 路径 [command.go:62]。 | — |

`driver` key 为实例按名指定 Driver bean：不配置 → 装配由按类型注入的可选 Driver bean（见
§2.1）或内置 `DefaultDriver` 负责；配置 → 按名注入该 bean，指定的 bean 不存在则启动失败。

`schema.json` 里 `metrics.enabled` 默认写的是 `false`，代码默认是 `true` —— 以代码为准。

---

## 4. 验证与故障演练

### 4.1 启动期 ping 探测

```bash
docker stop pulsar && go run .    # 启动中止："pulsar broker probe failed on pulsar://..."
docker start pulsar && go run .   # admin health 端点应答后即可启动（§1 闸门）
```

### 4.2 消息往返（含 driver 映射字段存活）

按 §1 发布 `Key="user-42"`、`Headers={"h1":"v1"}`；在 handler 里打印
`m.Key, m.Headers["h1"], string(m.Payload)` —— 三者全部存活，Headers 里还多出注入的
`traceparent`。可用 example 的冒烟路径验证（`example/check.sh`）。

### 4.3 受保护 vs 未保护（治理标签检查）

```yaml
# conf/governance.yaml
spring:
  governance:
    enabled: true
    resilience:
      breaker:
        enabled: true
        min-calls: 4
        failure-rate: 50
```

服务标签是 `pulsar:pulsar://127.0.0.1:6650` [starter.go:76]。停掉 broker 后：压
`GuardedSend`——或走同一 seam 的 driver `Publish`——→ 过阈值后熔断打开，调用快速失败
返回 resilience 哨兵错误，`messaging.client.*`（声明式操作）与 `resilience.client.*`
计数出现。未保护面的对比对象是 `producer.SendAsync` 与生命周期调用（`CreateProducer`/
`Subscribe`）：它们阻塞进 client 自身的重试/超时，没有哨兵、没有熔断。这个对比就是
§2.2 的边界。

### 4.4 指标 / span / 日志读取

- 原生：`curl -s :9091/metrics | grep pulsar_client_`（producer/consumer/连接统计；
  按实例独立 registry，实例间永不冲突 [command.go:85-100]）。
- OTel：声明式操作由 resilience 层发射名为 `publish` / `consume` 的 span（trace 经
  Properties 里的 W3C context 串联），外加 `messaging.client.operation.duration` 与
  `messaging.client.attempt.duration`；应用手动 span 名为 `pulsar.produce` /
  `pulsar.consume <topic>`，带 `messaging.system=pulsar` [command.go]。发流量后查 Jaeger
  （`:16686`）。
- 日志：driver handler/receive 错误与全部桥接的 client 内部日志落在 `_app_def` tag，
  前缀 `pulsar: `。

### 4.5 停机演练

SIGTERM → destroyClient 关闭 executor、client（全部 producer/consumer）与 metrics
server [client.go:44-58]。subscriber Close 先排空循环再 consumer.Close
[messaging.go:198-207]。观察日志干净退出；:9091 停止服务。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 "pulsar broker probe failed" | broker 宕 / url 错 / 认证 / TLS | 修连通性；6650 开 ≠ 就绪，用 `:8080/admin/v2/brokers/health` 闸门。 |
| 第二个实例没有 /metrics | `metrics.port` 冲突；监听失败仅记 WARN [command.go:96] | 各配独立端口。 |
| 没有 trace | 未 import starter-otel | 加上；所有助手在无它时是静默 no-op。 |
| handler 明明成功了消息却重投 | Ack 失败（已记 WARN）[messaging.go] | 检查 broker ack 权限；嫌疑见 §6。 |
| 消费者收不到消息 | 订阅名不对 / Shared 与 topic 语义 | `group` 与订阅 1:1；空 group 派生 `go-spring-<topic>` [messaging.go]。 |
| 期望 token 认证，broker 拒绝 | mTLS cert+key 已设置 → token 被忽略（优先级）[driver.go:83-90] | 去掉 cert/key，或放宽 broker 的 mTLS。 |
| 压测下熔断从不打开 | 流量走 `producer.SendAsync` 或手动助手 —— 未保护（§2.2） | 把同步发送改走 `GuardedSend`（driver `Publish` 已走）。 |
| handler panic 什么都不打死，但消息重现 | Recover 把 panic 转为 Nack | 预期行为；修 handler。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 17 个 value tag |
| 其中必填 | 1（`url`） |
| quickstart 前置外部依赖 | 1（Pulsar standalone） |
| "注意/坑"条数 | 8 |

设计嫌疑（审计台账）：`metrics.port` 固定默认 9091，多实例之间及与其他应用易冲突，且
监听失败被吞；driver 的 Publish 与消费循环现均声明各自操作并跑在同一 executor 下，与裸
路径一致，因此 starter 不再逐调用发射 —— resilience 层是唯一发射点（`SendAsync` 与
生命周期调用仍未受保护）；消费侧 ack 失败记 WARN；`producer.Close()` 无
错误返回，publisher Close 不会失败；无运行期 health indicator（ping
仅启动期 —— broker 后续宕机对 actuator 不可见）；`schema.json` 的
`metrics.enabled` 默认值与代码（true）不一致。
