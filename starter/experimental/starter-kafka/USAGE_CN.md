# starter-kafka 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`）与可运行的
[example/](example/)、[example-cloudnative/](example-cloudnative/)、
[example-otel/](example-otel/) 核验——文中以 [文件:行号] 标注。**Kafka broker 语义与
franz-go API 属于 [franz-go 官方文档](https://github.com/twmb/franz-go)与
[kafka.apache.org](https://kafka.apache.org/documentation/)**——下文只写 go-spring 的增量。

**激活方式**：任一 `spring.kafka.*` key 即激活（模块为 `gs.Module(gs.OnProperty("spring.kafka"))`
前缀匹配 [starter.go:38]）。每个 `spring.kafka.<name>` 条目创建一个名为 `<name>` 的
`*kgo.Client` bean。**starter 自身不注册 health indicator**——应用侧模式见 §4.1。

---

## 1. 完整工程示例

一个走 messaging binder 的生产 + 消费服务，组合健康探针、metrics、tracing 与运行时治理。
文件：`go.mod`、`main.go`、`service.go`、`conf/app.properties`。

**go.mod**（关键依赖）：`github.com/twmb/franz-go/pkg/kgo`、`go-spring.org/spring`、
`go-spring.org/cloud`、`go-spring.org/starter-kafka`，加可选的 `starter-actuator`
（探针 + /metrics）、`starter-otel`（trace/metric 导出）、`starter-governance`（限流/熔断）。

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-kafka"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** —— 通过 binder 收发 `messaging.Message` 信封，外加 health-indicator 逃生口：

```go
package service

import (
    "context"
    "time"

    "github.com/twmb/franz-go/pkg/kgo"
    "go-spring.org/cloud/actuator/health"
    "go-spring.org/cloud/experimental/messaging"
    "go-spring.org/spring/gs"
    StarterKafka "go-spring.org/starter-kafka"
)

type Service struct {
    Client *kgo.Client `autowire:"a"` // 原始 client 保留：admin/事务等专有特性
}

// Health* 实现 health.Indicator；starter 自身不注册，应用自己导 Ping 探针
// （readiness+startup 组，critical）。
func (s *Service) HealthName() string { return "kafka:a" }
func (s *Service) CheckHealth(ctx context.Context) error {
    pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    return s.Client.Ping(pingCtx)
}
func (s *Service) HealthGroups() []health.Group { return []health.Group{health.GroupReadiness, health.GroupStartup} }
func (s *Service) IsCritical() bool { return true }

func init() {
    gs.Provide(func(s *Service) (messaging.Binder, error) {
        return StarterKafka.NewBinder(s.Client), nil
    })
    gs.Provide(func(b messaging.Binder) gs.Runner {
        return func(ctx context.Context) {
            pub, _ := b.NewPublisher(ctx, "hello")
            _ = pub.Publish(ctx, &messaging.Message{
                Key: "k1", Payload: []byte("value"),
                Headers: map[string]string{"origin": "demo"},
            })
            sub, _ := b.NewSubscriber(ctx, "hello", "") // group 用 client 配置！
            _ = sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
                return nil // 错误只打日志——见 §2.3
            })
        }
    })
}
```

**conf/app.properties** —— 上例实际使用的完整注释配置面：

```properties
# --- kafka 实例 "a"（一个 client 同时做生产+消费）---------------------------
spring.kafka.a.brokers=127.0.0.1:9092
spring.kafka.a.topic=hello
spring.kafka.a.group=hello-group
spring.kafka.a.producer.required-acks=all
spring.kafka.a.producer.compression=snappy

# 每消息访问日志（tag kafka.access）：off / brief / detailed。
spring.kafka.a.observability.level=detailed

# 明文开发 broker 下 SASL / TLS 关闭；安全集群上启用
# sasl.enabled/mechanism/username/password + tls.enabled/ca-file（见 §3）。

# --- actuator + otel --------------------------------------------------------
spring.http.server.enabled=false
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus

# --- governance（同步生产路径上的限流）--------------------------------------
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.rate-limit=8
```

**验证**（broker 启动同 [example/docker-compose.yml](example/docker-compose.yml) —— KRaft
模式、自动建 topic、端口 127.0.0.1:9092）：

```bash
docker compose up -d                        # bitnami/kafka:3.7，KRaft 单节点
# 等端口就绪，broker 启动慢（约 30s）
go run .                                    # broker 不可达则启动快速失败
curl -s :9370/readyz | jq .                 # kafka:a 组件（应用侧 indicator）
grep kafka.access app.log | tail -3         # publish/consume 访问记录
```

自断言冒烟脚本是 [example/check.sh](example/check.sh)（`./check.sh`）；治理/健康/dync 组合见
[example-cloudnative/](example-cloudnative/)（`go run .` 依次打印 health / round-trip /
resilience / 热加载行并以 0 退出）。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-kafka
  └─ gs.Module(OnProperty("spring.kafka"))：任一 spring.kafka.* key 存在时触发
        └─ conf.BindEach("${spring.kafka}") → 每个 <name> 条目一个 Config
              └─ Provide(newClient).Name(<name>).Destroy(destroyClient)   [starter.go:38-47]

gs.Run()
  ├─ ctor newClient [starter.go:62]:
  │   1. driverRegistry 查 driver；未知名 → 启动报错               [starter.go:65-69]
  │   2. d.CreateClient：完整 client 装配（见 §2.2）                [driver.go:67]
  │   3. Ping 10s 超时——坏 brokers/凭证/TLS 让启动失败，
  │      而不是等第一次 produce 才暴露                                [starter.go:50,76-82]
  │   4. applyResilience：fault.WrapExecutor(resilience.ExecutorFor(
  │      "kafka:<brokers>")) → resilobserve.WrapExecutor，以
  │      client 指针索引进包级 sync.Map                              [command.go:99-105]
  ├─ 无 Init 钩子；*kgo.Client bean 在 ctor 后即就绪
  ├─ Destroy(destroyClient) [starter.go:95-102]:
  │   closeResilience（executor Close）→ Flush(10s ctx) → Close。
  │   ⚠ Flush 失败会记 ERROR 日志（可能丢消息）并向上返回——未送达 broker 的缓冲记录丢失。
```

resilience 注册表以 `*kgo.Client` 指针为键，这正是 `GuardedProduceSync` 可以是接收原始
bean 的自由函数的原因：guard 直接解析 executor，无需包装 client 类型 [command.go:118-125]。

### 2.2 client 装配 —— DefaultDriver 按序安装什么

`DefaultDriver.CreateClient` 组装一次 `kgo.NewClient` 调用 [driver.go:67-104]：

1. `kgo.SeedBrokers(strings.Split(c.Brokers, ",")...)` —— brokers 是 CSV；每项只是 seed，
   client 自行学习完整集群拓扑。
2. `kgo.WithHooks(append(kt.Hooks(), newObserveHook(...))...)` [driver.go:74] ——
   **kotel（tracer+meter）钩子在前，observe 访问日志钩子在后**。理由：span 与 metric 归
   kotel，observe 钩子以 `WithoutTraceAndMetric()` 构造，只补访问日志缺口
   [command.go:43-48]——信号不重复。
3. `kgo.WithLogger(newLogger())` —— franz-go 内部日志（broker 连接、请求失败、重连）经
   go-spring log 桥接，tag `log.TagAppDef`，Info 级阈值 [driver.go:173-193]。
4. `kgo.ConsumerGroup` / `kgo.ConsumeTopics` 来自 `group`/`topic` 配置——**构造期固定**；
   这是 binder 继承的 franz-go 约束（§2.3）。
5. SASL mechanism、TLS（`c.TLS.Build()`——裸 `Build`，客户端侧 key 见 §3）、producer 选项
   （compression/acks/batch/linger）。

启动 ping 与 resilience 接线**有意不放**在 driver 里：它们是 starter 的生命周期职责
[driver.go:62-66 注释]。

### 2.3 binder 的一次发布与一次消费，逐层走读

绑定到 topic `hello` 的 publisher 上 `Publish(ctx, msg)` [client.go:78-94]：

1. 信封 → `kgo.Record`：`Topic` = publisher 绑定的目的地，`Value` = Payload，
   `Headers` = 信封 headers（空则 nil），`Key` 仅非空时设置 [client.go:79-86]。
   ⚠ `msg.Timestamp` **不映射**——由 broker 盖时间戳。
2. OTel propagator 经 `recordCarrier` 把 W3C trace context 注入 record headers
   [client.go:87]；无 starter-otel 时为 no-op。
3. 若 `traffic.IsLoadTest(ctx)`，压测标记随 record header 携带，消费侧可识别合成流量
   [client.go:90-92]。
4. `GuardedProduceSync(ctx, p.cl, rec).FirstErr()` —— 同步生产，走与裸 client API 同一个
   resilience executor（该实例关治理时为透明 no-op）[client.go:93-99]，broker ack / 拒绝
   直接返回给调用方。实例 key `governance=false` 可让 binder（及 `GuardedProduceSync`）
   彻底裸调用、不挂 executor。
5. client 内部钩子触发：kotel produce span + metric；observeHook 的
   `OnProduceRecordBuffered` 开启名为 `publish` 的访问日志记录，
   `OnProduceRecordUnbuffered` 以结果与 buffered→unbuffered 时长收尾
   [command.go:57-69]。访问日志 tag：`kafka.access`（observe kit
   `RegisterAppTag("kafka","access")`）。

绑定到 source `hello` 的 subscriber 上 `Subscribe(ctx, handler)` [client.go:109-144]：

1. `messaging.SafeHandler` 把 handler panic 转为 error 路径 [client.go:112]。
2. 一个后台 goroutine 在 ctx（经 `context.WithoutCancel` + Close 持有的 cancel）上轮询
   `cl.PollFetches` [client.go:113]。
3. poll 错误打日志（tag `log.TagAppDef`），`context.Canceled` 抑制 [client.go:121-127]。
4. 逐条 record：给了 source 且 `rec.Topic != s.topic` 则过滤——source 与所配 topic 都不
   匹配时静默过滤掉一切 [client.go:129-131]。
5. 从 record headers 提取 trace context；压测标记恢复到消息 ctx [client.go:132-136]。
6. handler 收到 `fromRecord(rec)`：Key/Payload/Headers/Timestamp 全部在映射中存活
   [client.go:172-186]。⚠ handler 错误**只打日志**——本 binder 无 nack/重投
   （franz-go 组消费照常提交；设计嫌疑，§6）。
7. `Close` 取消循环并等 `done`；`sync.Once` 保证幂等 [client.go:146-156]。

franz-go 构造约束带来的两个 binder 陷阱 [client.go:43-51 注释]：
`NewSubscriber(ctx, source, group)` **静默丢弃 `group` 实参**（用 client 的 `group` 配置
——client.go:68）；且一个 client 只是一个 consumer——每个逻辑 consumer 用一个 client bean。

### 2.4 GuardedProduceSync —— resilience seam 的精确语义

franz-go 的异步 `Produce` 立即返回，因此只有同步路径可包 [command.go:21-22,126-137 注释]。
`GuardedProduceSync(ctx, cl, recs...)` [command.go:138-154]：

- guard 解析 `cl` 上挂的 executor；没有（governance 关）则原样内联执行——与
  `cl.ProduceSync` 行为一致。
- governance 开启时，调用先过 `fault.WrapExecutor(resilience.ExecutorFor(...))` 再过
  `resilobserve.WrapExecutor` [command.go:100-101]：运行时故障注入与 resilience 结果
  metric 包住 produce，资源标签 `kafka:<brokers>`
  （`resilience.ResourceLabel("kafka", c.Brokers)` [starter.go:83]，格式 `prefix:name`
  [resilience/config.go:151-158]）。
- 被拒（限流 / 熔断打开）时 produce **绝不执行**；拒绝错误编码为逐 record 错误，调用方的
  `.FirstErr()` 像真实 produce 失败一样拿到它 [command.go:145-151]。example-cloudnative
  断言突发流量会得到 `resilience.ErrRateLimited`。
- **不受保护**的路径：直接在 client bean 上调裸 `ProduceSync`/`Produce`，以及整条
  consume/poll 路径（被动）。binder 的 publish **已受保护**（§2.3 第 4 步）；实例
  `governance=false` 会把所有调用路径的 guard 摘掉。

---

## 3. 逐 key 行为参考

key 都在 `spring.kafka.<name>.*` 下——ctor 参数经 `conf.BindEach` 绑定（真正的按实例前缀
绑定）。`value:` tag 已与源码核对：共 21 个 key。

### 3.1 核心

| Key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `brokers` | string | — | **必填**（`expr:"$ != ''"` [config.go:30]）；CSV seed brokers；同时构成 resilience 资源标签 `kafka:<brokers>`。 | 空 → 启动报错；错但可达的主机在 10s 启动 Ping 处失败。 |
| `topic` | string | "" | 传给 `kgo.ConsumeTopics`——消费 topic 构造期固定；binder subscriber 按它过滤。空 = 纯生产 client。 | 能生产、消费永不投递（未订阅 topic）。 |
| `group` | string | "" | 传给 `kgo.ConsumerGroup`；group 语义属 Kafka 自身（offset、rebalance——见 kafka.apache.org）。⚠ binder `NewSubscriber` 的 group 实参是死的——本 key 是唯一 group 开关。 | 空 + 有 topic = 无 group（随机 group/急切）消费；offset 不提交。 |
| `driver` | string | `DefaultDriver` | 选择已注册的 `Driver` [driver.go:46-57]；`RegisterDriver` 重名 panic。 | 未知名 → 启动报错 "kafka driver not found" [starter.go:68]。 |
| `governance` | bool | true | 为实例挂 resilience/fault executor；同时保护 `GuardedProduceSync` 与 binder 的 `Publish`（同一 resource label）。治理中心未开时为透明 no-op。 | `false` → 所有调用路径裸跑，govern.* 规则永不生效。 |

### 3.2 SASL

| Key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `sasl.enabled` | bool | false | 控制 mechanism 装配 [driver.go:83-89]。 | 开而无凭证 → 启动 Ping 认证失败。 |
| `sasl.mechanism` | string | `plain` | `plain` / `scram-sha-256` / `scram-sha-512`，大小写不敏感 [driver.go:107-118]。 | 其他值 → 启动 CreateClient 报错。 |
| `sasl.username` / `sasl.password` | string | "" | 传给 mechanism。 | 错 → 启动 10s Ping 失败。 |

### 3.3 TLS

共享 `tlsconf` 块——跨 starter 属性名统一 [config.go:43-47]。

| Key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `tls.enabled` | bool | false | `c.TLS.Build()` → `kgo.DialTLSConfig` [driver.go:90-97]。 | — |
| `tls.cert-file` / `tls.key-file` | string | "" | mTLS 客户端证书对。 | 只配一半 → `tls.Build` 启动报错。 |
| `tls.ca-file` | string | "" | 校验 broker 的 CA。 | 私有 CA 下缺失 → Ping TLS 失败。 |
| `tls.server-name` | string | "" | SNI/校验名。 | 失配 → 校验失败。 |
| `tls.insecure-skip-verify` | bool | false | 跳过校验。 | 生产置 true = 默默暴露于 MITM。 |

### 3.4 Producer

零值保持 franz-go 默认 [config.go:79-80]。

| Key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `producer.compression` | string | "" | `none`/`gzip`/`snappy`/`lz4`/`zstd`，大小写不敏感 [driver.go:153-167]。 | 其他值 → 启动 CreateClient 报错。 |
| `producer.required-acks` | string | `all` | `all`→AllISRAcks；`leader`/`none` 还会**关闭幂等写**（协议要求）[driver.go:132-141]。 | 其他值 → 启动报错；弱化 acks 会静默丢幂等。 |
| `producer.max-batch-bytes` | int32 | 0 | >0 时 `kgo.ProducerBatchMaxBytes`。 | 低于 broker 消息上限 → 逐条 produce 报错。 |
| `producer.linger` | duration | 0s | >0 时 `kgo.ProducerLinger`；吞吐/延迟权衡。 | — |

### 3.5 Observability（共享 observe kit）

| Key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `observability.level` | string | `brief` | 访问日志 off/brief/detailed；kotel span/metric 是另一路且恒开（无 per-starter otel 开关——与 go-redis 不同）。 | `off` 只静默日志信号。 |
| `observability.maxArgBytes` | int | 512 | detailed 模式下捕获参数的字节上限。 | 过小 → 参数截断。 |
| `observability.skipOps` | list | — | 对列出的 op 名同时抑制 span+metric+log；这里 op 名为 `publish` / `consume`。 | — |

---

## 4. 验证与故障演练

### 4.1 健康（应用侧 indicator）

starter 不注册 indicator；应用自己导一个（如 §1 / example-cloudnative
[example.go:76-97]）：

```bash
curl -s :9370/readyz | jq .          # 组件 "kafka:a"，readiness+startup 组，critical
docker stop starter-kafka            # Ping 探针失败
curl -s :9370/readyz                 # 503 OUT_OF_SERVICE
```

放进 readiness/startup（绝不放 liveness——broker 故障不应重启 pod）由应用经
`HealthGroups()` 决定。

### 4.2 消息往返 + binder 映射存活字段

```bash
go run .    # §1 服务：publish Key=k1 Payload=value Header origin=demo，消费打印
grep kafka.access app.log | tail -2   # publish 记录（带时长）+ consume 记录
```

存活字段：消费侧 Key、Payload、Headers、broker 盖的 Timestamp [client.go:172-186]。
丢弃项：发布侧 `msg.Timestamp` 被忽略；与 W3C trace key 或压测头同名的 `Headers` 条目会被
注入步骤覆盖 [client.go:87-92]。

### 4.3 受保护 vs 不受保护的生产路径

```bash
# example-cloudnative 配 govern.default.rate-limit=8：
go run .    # 打印 "resilience: N produce admitted, M rejected with ErrRateLimited"
```

同一突发走 **binder** publisher 会被同样限流——它走同一个 executor（§2.3 第 4 步）；
只有直接在 client bean 上裸调 `ProduceSync` 才绕过。改被监听源里的 `govern.*` 可免重启
换策略（治理中心热加载）。

### 4.4 治理标签核对

资源标签是 `kafka:<brokers>`——就是 `brokers` 字符串本身，不是按 topic 或按 client 名
[starter.go:83]。两个共享同一 broker 列表的 client bean 共享一个 limiter/breaker；边压
`GuardedProduceSync` 边看 resilience 结果计数（`curl -s :9370/metrics | grep resilience`）
即可验证。

### 4.5 Traces / metrics / 日志 tag

- kotel span + `messaging.*` metric 挂在 starter-otel 的全局上；命名遵循 kotel 插件
  （见 [kotel](https://github.com/twmb/franz-go/tree/main/plugin/kotel)）。example-otel 配
  OTLP gRPC → :4317 Jaeger；在 Jaeger UI 检查生产/消费 span 的链路关联（trace context 随
  record headers 走，§2.3 第 2/5 步）。
- 访问日志 tag `kafka.access`（渲染形如 `_app_kafka_access`）：op 名 `publish`（带时长）与
  `consume`（无时长——消费没有成对的起始钩子 [command.go:40-42,71-75]）。
- franz-go client 内部日志（重连、请求失败）出现在 `log.TagAppDef` 下，Info 阈值桥接
  [driver.go:177-192]。

### 4.6 broker 宕机快速失败

```bash
docker compose down && go run .   # 约 10s 内启动失败："failed to ping kafka: <brokers>"
```

运行中 broker 失联表现为日志里的 poll 错误（tag `log.TagAppDef`）与逐次 produce 错误；
franz-go 自动重连（其自身语义，见 franz-go 文档）。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 "failed to ping kafka" | brokers 不可达 / SASL 错 / TLS 失配 | 修连通性或凭证；10s 探针无条件执行。 |
| 启动失败 "kafka driver not found" | `driver` 名未注册 | 在 init 里 `RegisterDriver`，或删掉该 key。 |
| 启动失败 "unsupported kafka sasl mechanism / required-acks / compression" | 枚举 key 拼写错误 | 枚举精确匹配（大小写不敏感）；改对值。 |
| binder 消费者收不到 | `NewSubscriber` source ≠ 所配 `topic`，或 `topic` 为空 | source 必须等于 client 的 `topic`；否则静默过滤。 |
| binder 消费 group "不生效" | `NewSubscriber` 的 group 实参是死的 | 配 `spring.kafka.<name>.group`（构造期固定）。 |
| govern.* 已开但无限流 | 直接在 client bean 上裸调 `ProduceSync`，或实例 `governance=false` | 只有 `GuardedProduceSync` 与 binder publisher 受保护。 |
| 无 traces/metrics | 未 import starter-otel | kotel 挂 OTel 全局；import starter-otel。 |
| 无访问日志 | `observability.level=off`，或日志 tag 被过滤 | 置 `detailed`；检查 `kafka.access` tag 过滤。 |
| handler 错误只留一行日志 | 设计如此：本 binder 无 nack/重投 | 在 handler 内自建重试，或用 messaging 的 retry.go。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 21（核心 4 + sasl 4 + tls 6 + producer 4 + observability 3） |
| 必填 | 1（`brokers`） |
| quickstart 前置外部依赖 | 1（Kafka broker） |
| "注意/坑"条数 | 6 |

设计嫌疑（审计台账；自上一版承继，均未修复）：

- binder 丢弃 `NewSubscriber` 的 `group` 实参，source 失配静默过滤 [client.go:68,129-131]——违背 fail-fast。
- 消费 handler 错误只打日志，无 nack/重投 [client.go:137-139]——与 SafeHandler 注释的说法相悖 [client.go:110-112]。
- ~~`destroyClient` 丢弃 Flush 错误~~已修：Flush 失败记 ERROR（点名丢数据后果）并从 destroy 钩子向上返回。
- starter 不注册 health indicator（家族不对称：go-redis/redigo 都注册）。
- resilience executor 以 `*kgo.Client` 指针为键存包级 sync.Map [command.go:81-85]——自定义 Driver 返回包装 client 会静默跳过 guard。
