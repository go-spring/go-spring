# starter-kafka-sarama 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`、`logger.go`、`scram.go`）
与可运行的 [example/](example/) / [example-otel](example-otel/) 核实，文中方括号为 file:line
抽查点。**Kafka 协议与 sarama API 语义属
[sarama 官方文档](https://github.com/IBM/sarama) 与
[kafka.apache.org](https://kafka.apache.org/documentation/)** —— 下文只写 go-spring 的增量。

**激活条件**：出现任意 `spring.kafka-sarama.*` 配置即激活（模块注册于
`gs.OnProperty("spring.kafka-sarama")`，前缀匹配 [starter.go:38]）。每个
`spring.kafka-sarama.<name>` 条目创建一个名为 `<name>` 的 `sarama.Client` bean
[starter.go:39-43]。该前缀与 franz-go 版 [starter-kafka](../starter-kafka)（`spring.kafka`）
刻意区分，二者从不同时引入 [config.go:26-28]。

**本 starter 无 messaging.Driver**（与 starter-kafka 不同）：发布/消费是在共享 client
bean 之上的裸 sarama 用法，观测是调用点显式助手。这是头号设计嫌疑 —— 见 §6。

---

## 1. 完整工程示例

一个包含 producer 与 partition consumer（topic `hello`）的服务，附追踪、访问日志与
治理包裹的发送。文件：`go.mod`、`main.go`、`service.go`、`conf/app.properties`。
**go.mod**（关键依赖）：

```
require (
    github.com/IBM/sarama       latest
    go-spring.org/spring        v1.3.x
    go-spring.org/starter-kafka-sarama latest
    go-spring.org/starter-otel    latest   // 可选：真实 trace/metric 导出
    go-spring.org/starter-governance latest // 可选：resilience/fault 策略
    go-spring.org/starter-actuator latest  // 可选：探针 + /metrics 挂载
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-kafka-sarama"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** —— 应用的全部 Kafka 面：

```go
package service

import (
    "context"

    "github.com/IBM/sarama"
    "go-spring.org/spring/gs"
    StarterKafkaSarama "go-spring.org/starter-kafka-sarama"
)

type Service struct {
    // 永远注入裸 sarama.Client bean。producer/consumer 用 sarama 的
    // *FromClient 构造器按需派生 —— 一套连接池与 metadata 缓存服务所有角色
    // （DESIGN.md §2）。
    Client sarama.Client `autowire:"main"`
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) error {
            return s.publish(ctx, "hello", "value")
        }
    })
}

// publish 发送一条记录：trace context 写入 headers，发送经治理 executor
// （breaker/rate-limit/retry）保护。
func (s *Service) publish(ctx context.Context, topic, value string) error {
    producer, err := sarama.NewSyncProducerFromClient(s.Client)
    if err != nil {
        return err
    }
    defer producer.Close() // 派生 bean 要在 client Destroy 之前关闭
    // 治理包裹 —— 治理关闭时原样返回，无条件包裹永远安全（command.go:242-244）：
    producer = StarterKafkaSarama.WrapSyncProducer(s.Client, producer)

    msg := &sarama.ProducerMessage{Topic: topic, Value: sarama.StringEncoder(value)}
    _, span := StarterKafkaSarama.StartProducerSpan(ctx, msg) // 注入 W3C ctx + 压测标记
    _, _, err = producer.SendMessage(msg)
    StarterKafkaSarama.EndSpan(span, err)
    return err
}
```

**conf/app.properties** —— 上述用到的完整带注释配置面：

```properties
# --- kafka client -----------------------------------------------------------
spring.kafka-sarama.main.brokers=127.0.0.1:9092
# 须与目标集群匹配，消费组等功能才正常（sarama 由此协商协议特性）；留空用 sarama 默认。
spring.kafka-sarama.main.version=3.7.0

# --- producer 调优 ----------------------------------------------------------
spring.kafka-sarama.main.producer.compression=snappy
spring.kafka-sarama.main.producer.required-acks=all

# --- 可选：SASL + TLS（dev broker 为明文） -----------------------------------
#spring.kafka-sarama.main.sasl.enabled=true
#spring.kafka-sarama.main.sasl.mechanism=scram-sha-512
#spring.kafka-sarama.main.sasl.username=user
#spring.kafka-sarama.main.sasl.password=pass
#spring.kafka-sarama.main.tls.enabled=true
#spring.kafka-sarama.main.tls.ca-file=/path/ca.pem
#spring.kafka-sarama.main.tls.cert-file=/path/client.pem
#spring.kafka-sarama.main.tls.key-file=/path/client.key

# --- 可观测（starter-otel） --------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090

# --- actuator ---------------------------------------------------------------
spring.actuator.addr=:9370
```

**启动 Kafka**（KRaft 单节点，同 [example/docker-compose.yml](example/docker-compose.yml)）：

```bash
cd demo && docker compose up -d   # bitnami/kafka:3.7，PLAINTEXT 127.0.0.1:9092，自动建 topic
```

**验证**（与 [example/check.sh](example/check.sh) 冒烟同构）：

```bash
go run .                                        # metadata 拿不到 broker 时启动直接失败
grep 'kafka sarama client initialized' app.log  # [client.go:70]
grep _app_kafka_access app.log | tail -1        # 每次 publish/consume 观测一条
curl -s :9090/metrics | grep messaging_client   # duration 直方图 + in-flight
```
---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-kafka-sarama
  ├─ init: sarama.Logger = go-spring log 桥接   [starter.go:33]（sarama 事件全转 Info，默认 app tag）
  └─ gs.Module(OnProperty("spring.kafka-sarama")) 任意 spring.kafka-sarama.* 触发
        └─ conf.BindEach → 每个 <name> 条目一个 Config
              └─ Provide(newClient, IndexArg name, IndexArg config, IndexArg 3,?Driver).Name(<name>)
                   .Destroy(destroyClient)     [starter.go:40-44]

gs.Run()
  ├─ newClient [client.go:48]:
  │    1. 可选 Driver bean —— 无则内置 DefaultDriver  [client.go:52-53]
  │    2. d.CreateClient：sarama.NewConfig + version/SASL/TLS/producer 选项，
  │       然后 sarama.NewClient(brokers) —— 这里就会拨号 seed broker 并拉取
  │       metadata，坏 broker/坏凭据/TLS 不匹配在启动期失败，而非首次使用时
  │       爆雷 [client.go:42-46 注释, driver.go:63-96]
  │    3. 防御性 fail-fast：len(Brokers())==0 → close + 启动报错 [client.go:60-64]
  │    4. applyResilience：fault.Wrap(ExecutorFor("kafka:<brokers>")) →
  │       resilience.WrapExecutor（span/计数/直方图/访问日志）→ 以 client
  │       为键存入 sync.Map [client.go:65-69, command.go:208-217]
  ├─ 派生 bean 归你所有：注入 client 处用 sarama.New*FromClient 自建
  └─ SIGTERM → Destroy：closeResilience（exec.Close、清 map）→ cl.Close()
       [client.go:78-81, command.go:220-225]
```
**装配扩展点**：client 装配由 `Driver`（接口，`driver.go:28-39`）负责。公司/伞包 starter 可把
自己的 `Driver` 作为**可选容器 bean** 提供（`gs.Provide(func() StarterKafkaSarama.Driver{...})`，
因为是 bean，可在装配期注入从配置文件绑定的配置）；`spring.kafka-sarama` 下每个实例都经它
构建。没有该 bean 时 starter 在装配内回退到内置 `DefaultDriver`（`driver.go:41-88`，
`client.go:52-53`）。没有 per-config 的 `driver` key。

派生的 producer/consumer 不是容器 bean —— 需自行在应用退出前关闭（publish 路径里
`defer producer.Close()` 即预期写法，见 [example/example.go:72-76]）。
`sarama.Client.Close` 释放共享 broker 连接。
### 2.2 wrap 机制 —— 精确顺序与未保护面

`WrapSyncProducer(cl, p)` [command.go:254-260] 取出为 `cl` 暂存的 executor；治理关闭时
无条目（`executorFor` 返回 nil [command.go:230-237]），`p` 原样返回 —— 无条件包裹是
零风险惯用法。被保护的面：

```
SendMessage / SendMessages
  → resilience executor 包装（span + outcome 计数 + duration + 访问日志）
    → fault.WrapExecutor（govern.fault 启用时注入故障）
      → resilience executor（breaker / rate limit / retry，策略来自治理中心）
        → 内层 p.SendMessage（真实 sarama）
```

**未保护** [command.go:246-249, 292-303]：`Close` 与整个事务家族
（`BeginTxn`/`CommitTxn`/`AbortTxn`/`AddOffsetsToTxn`/`AddMessageToTxn`/`TxnStatus`/
`IsTransactional`）直接透传 —— 它们是控制面，不是被保护的数据路径。同样未保护：
`sarama.AsyncProducer`（无包装器）、所有消费路径（`Consumer`、`ConsumerGroup`）、
以及任何忘记包裹的 producer。`SendMessage` 不收 `context.Context`（sarama API 无
ctx），包装器只能用 `context.Background()` —— 逐调用时限要用 resilience 的
`AttemptTimeout`/`MaxDuration`。
### 2.3 一次发布，逐层走读

追踪开启时，wrapped producer 上执行 `publish(ctx, "hello", "value")`：

1. `StartProducerSpan(ctx, msg)` 在 topic `hello` 上开启 `publish` 观测
   [command.go:95-104]：span + `messaging.client` duration/in-flight 指标 + 访问日志
   记录，system 为 `kafka`（属性命名空间：`messaging.system`、
   `messaging.operation`、`messaging.destination.name`，指标
   `messaging.client.operation.duration`；见 observe.go）。
2. W3C propagator 把 `traceparent`/`tracestate` 注入 `msg.Headers` [command.go:97]。
   ⚠ 注入会**丢弃同 key 的既有 header** 以保证重复注入幂等 [command.go:140-149] ——
   别把业务数据放在 `traceparent` 下。
3. ctx 带压测标记时，标记 header 一并写入 [command.go:100-102] —— 消费侧据此还原
   `traffic.IsLoadTest`（§2.4）。
4. wrapper 的 `SendMessage` → executor 许可（breaker/rate-limit 作用于资源
   `kafka:<brokers>` —— 按 client，所有 topic 共用一个标签 [client.go:65]）。
5. sarama 发送并（starter 强制 `Producer.Return.Successes=true` [driver.go:72]）在
   broker ack 后返回 partition/offset（`required-acks=all` → WaitForAll
   [driver.go:131]）。
6. `EndSpan(span, err)` 记录结果并关闭 span/日志/指标 [command.go:123-125]。

### 2.4 一次消费，逐层走读

收到一条 `*sarama.ConsumerMessage`（这里用 partition consumer；ConsumerGroup handler
同样适用）：

1. `StartConsumerSpan(ctx, msg)` 从 `msg.Headers` **提取**上游 trace context
   [command.go:113-120] —— 只要两侧都用助手，消费 span 就是生产 span 的子 span。
   提取永不改动消息（`consumerCarrier.Set` 是 no-op [command.go:162-173]）。
2. 压测标记 header 读回 ctx（`traffic.WithLoadTest`，reason `kafka-sarama-header`
   [command.go:116-118]）—— 下游业务代码与 client 不靠任何 HTTP header 即可分支。
3. 在 topic 上开启 `consume` 观测（与 publish 同一指标/日志命名空间）。
4. 你的 handler 执行；`EndSpan(span, err)` 关闭观测。offset 提交完全由你/消费组负责
   —— starter 从不介入。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.kafka-sarama.<name>.` 下（含 tls/sasl 组共 15 个；
这里是 `conf.BindEach` 的按实例前缀绑定，不是绝对属性的字段注入）。

### 3.1 核心

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `brokers` | string | — | **必填**（`expr:"$ != ''"` [config.go:32]）；逗号分隔 seed 列表 [driver.go:95]。同时原样构成治理资源标签 `kafka:<brokers>` [client.go:65] —— 同一集群写法不同即**不同**标签。 | 缺失/为空 → 绑定报错。broker 写错 → sarama.NewClient 启动失败（fail-fast）。 |
| `version` | string | ""（sarama 默认） | `sarama.ParseKafkaVersion` 解析；决定协议特性（headers、SASL 机制、消费组）[driver.go:65-71]。 | 解析失败 → 启动报错 `invalid kafka version`。过低 → 首次使用才报功能错误。 |

无 `driver` key：client 装配由可选 Driver bean（见 §2.1）或内置 `DefaultDriver` 负责。

### 3.2 SASL（`sasl.*`）—— [config.go:64-77]、[driver.go:101-118]

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `sasl.enabled` | bool | false | 整块开关；false 时忽略其余三个 key。 | 对 SASL broker 忘开 → 启动期 metadata 拉取失败。 |
| `sasl.mechanism` | string | `plain` | `plain` / `scram-sha-256` / `scram-sha-512`（大小写不敏感）；SCRAM 每次握手接 xdg-go/scram 客户端生成器 [scram.go:56-63]。⚠ SCRAM 需够高的 `version` —— 查 [sarama 文档](https://github.com/IBM/sarama)。 | 其他值 → 启动报错 `unsupported kafka sasl mechanism`（不会静默回退 PLAIN [driver.go:99-100]）。 |
| `sasl.username` / `sasl.password` | string | "" / "" | 拷入 `cfg.Net.SASL` [driver.go:103-104]。 | 配错 → 启动期 SASL 握手失败（fail-fast）。 |

### 3.3 TLS（`tls.*`）—— 共享 `cloud/tlsconf` 块 [config.go:42-46]

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `tls.enabled` | bool | false | 开启后 `c.TLS.BuildClient()` → `cfg.Net.TLS` [driver.go:81-89]。 | 对 TLS listener 未开 → 启动期握手失败。 |
| `tls.cert-file` / `tls.key-file` | string | "" | 客户端证书对（mTLS）。⚠ 要么都给要么都不给。 | 只给一半 → `tls.Build` 启动报错。 |
| `tls.ca-file` | string | "" | 校验 broker 的 CA。 | 私有 CA 未配 → 启动期校验失败。 |
| `tls.server-name` | string | "" | SNI/校验名。 | 不匹配 → 启动期校验失败。 |
| `tls.insecure-skip-verify` | bool | false | 跳过 broker 证书校验。 | 生产置 true → 静默 MITM 风险。 |

### 3.4 producer

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `producer.compression` | string | ""（sarama: none） | `none`/`gzip`/`snappy`/`lz4`/`zstd`，大小写不敏感 [driver.go:143-158]。 | 其他值 → 启动报错 `unsupported kafka compression`。 |
| `producer.required-acks` | string | `all` | `all`→WaitForAll、`leader`→WaitForLocal、`none`→NoResponse [driver.go:129-138]。 | 其他值 → 启动报错 `unsupported kafka required-acks`。`none` 在 leader 故障时静默丢消息。 |

⚠ 无 key 的强制项：starter 恒设 `Producer.Return.Successes=true` 与
`Consumer.Offsets.Initial=OffsetOldest` [driver.go:72-73] —— 二者均无配置 key 可改
（设计嫌疑，§6）。新消费组因此从**最旧** offset 开始。

---

## 4. 验证与故障演练

### 4.1 启动 fail-fast（broker 宕机）

```bash
docker compose stop kafka
go run .    # 非零退出：sarama.NewClient 拉不到 metadata
grep 'no brokers after metadata fetch\|failed to create kafka client' app.log   # [client.go:57-63]
docker compose start kafka
```

broker 死亡/未认证时进程到不了"服务中"—— 不存在首次 produce 才爆雷 [client.go:42-46]。

### 4.2 被保护 vs 未保护路径

配置 starter-governance，对资源 `kafka|127.0.0.1:9092` 设 breaker/rate-limit 策略：

```go
wrapped := StarterKafkaSarama.WrapSyncProducer(cl, producer)
wrapped.SendMessage(msg)   // 拒绝可见于 _app_kafka_access + resilience 计数
producer.SendMessage(msg)  // 裸句柄依旧不受保护 —— wrapper 不改写 p
```

演练：打过限流阈值 → wrapped 调用返回 resilience 错误（partition/offset `-1/-1`
[command.go:280-283]）；裸调用照常通过。消费路径永远不被治理 —— 验证其不产生
`messaging.client` 拒绝记录。

### 4.3 消息往返与 header 存活

同 topic 先发后收（partition consumer 从最旧读，同 [example/example.go:89-112]）：

- `msg.Value` 原样存活（`sarama.StringEncoder` → `string(msg.Value)`）。
- `StartProducerSpan` 写入的 `traceparent` 在消费侧可读 —— Jaeger 里 `publish` →
  `consume` 是同一条 trace（[example-otel](example-otel/) 冒烟正是对 Jaeger API 断言这点）。
- 压测标记：在带标记的 ctx 下发布（如上游 echo 的 loadtest 中间件），消费侧
  `StartConsumerSpan` 之后 `traffic.IsLoadTest(ctx)` 为真 [command.go:116-118]。
- ⚠ producer 消息上预设的 `traceparent`/标记 header 会被**替换**而非合并
  [command.go:140-149]。任何地方都没有消息 Key 映射（无 driver）—— `msg.Key`
  是什么就是什么。

### 4.4 观测量读取

```bash
grep _app_kafka_access app.log | tail      # system=kafka op=publish|consume topic=... status/duration
curl -s :9090/metrics | grep messaging_client   # messaging.client.operation.duration + in-flight，属性 messaging.system=kafka
# span 名："publish"/"consume"，属性 messaging.destination.name=<topic>
```

sarama 自身连接事件以 Info 级、`kafka:` 前缀、默认 app tag 到达 [logger.go:39-51] ——
broker 连接、metadata 刷新、重连都可见。

### 4.5 治理标签核对

executor 标签就是 `kafka|` + `brokers` 字符串 [client.go:65]。策略必须精确匹配该写法；确认：

```bash
grep 'resilience' app.log | grep 'kafka|127.0.0.1:9092'
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 `failed to create kafka client` | broker 不可达 / SASL/TLS 不匹配 | 设计即 fail-fast [client.go:56-59]；修连通性/凭据；看上方桥接的 `kafka:` sarama 行。 |
| 启动失败 `kafka client has no brokers after metadata fetch` | broker 列表可解析但集群 metadata 为空 | 防御检查 [client.go:60-64]；查 broker 的 advertised listeners。 |
| 启动失败 `invalid kafka version` / `unsupported kafka ... mechanism/compression/required-acks` | `version`/`sasl.mechanism`/`producer.compression`/`producer.required-acks` 拼写错误 | 取值是精确匹配枚举 [driver.go:66,115,137,156]。 |
| 无 trace / 助手无 `_app_kafka_access` | 未引入 starter-otel，或没调用助手 | 助手是调用点显式接入 [command.go:95-120]；引入 starter-otel（否则 OTel 全局为 no-op）。 |
| 治理策略不生效 | 资源标签不匹配，或 producer 没包裹 | 标签是 `kafka:<brokers>` 原样 [client.go:65]；用 `WrapSyncProducer` 包裹 —— 裸句柄不受保护。 |
| consumer 重读整个 topic | starter 强制 `Consumer.Offsets.Initial=OffsetOldest` [driver.go:73] | 在消费组 handler 里正确提交 offset；改此默认无配置 key。 |
| 运行期 `SyncProducer` 返回 `ErrOutOfBrokers` | broker 重启且版本/凭据在启动后变了 | sarama 会自动重连；凭据变了需重启应用（client 配置是启动期定死的）。 |
| breaker 熔断但消费侧持续失败 | 消费路径不被保护 | 设计如此（§2.2）；自行用 `resilience.ExecutorFor` 保护消费，或接受该缺口。 |
| 消费侧 header 重复困惑 | 生产侧注入丢弃同 key header | 别用 `traceparent`/压测标记 key 存业务数据 [command.go:140-149]。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 14（核心 2 + sasl 4 + tls 6 + producer 2） |
| 其中必填 | 1（`brokers`） |
| quickstart 前置外部依赖 | 1（Kafka；完整可观测另需 collector） |
| "注意/坑"条数 | 6 |

设计嫌疑（审计台账；自上轮以来均未修复）：

- **无 messaging.Driver** —— 家族内唯一无 driver 的 MQ starter；发布/消费观测全靠
  手工调用点助手，"driver 的 Key 映射丢字段"这一类问题在本 starter 结构性不存在
  （根本没有映射层可丢 Key）。
- `SendMessage` 保护用 `context.Background()` [command.go:275,287] —— 逐调用时限只能靠
  resilience `AttemptTimeout`/`MaxDuration`。
- 无消费组/订阅助手 → 消费侧治理与观测全手工（§4.2 演练可见缺口）。
- `Return.Successes`/`OffsetOldest` 静默覆盖 sarama 默认且无配置 key [driver.go:72-73]。
- 治理资源标签用原始 `brokers` 字符串 —— 同一集群写法不同即不同熔断域 [client.go:65]。
- resilience executor 索引以 `sarama.Client` 接口值为键存 `sync.Map` [command.go:192-197]
  —— 对 starter 的"每名字一个 client"模型安全，但自定义 Driver 若返回逐次不同的
  wrapper 对象会破坏 `WrapSyncProducer` 的查找。
