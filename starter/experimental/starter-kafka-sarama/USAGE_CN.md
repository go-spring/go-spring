# starter-kafka-sarama 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`、`logger.go`、`scram.go`）
与可运行的 [example/](example/) / [example-otel](example-otel/) 核实，文中方括号为 file:line
抽查点。**Kafka 协议与 sarama API 语义属
[sarama 官方文档](https://github.com/IBM/sarama) 与
[kafka.apache.org](https://kafka.apache.org/documentation/)** —— 下文只写 go-spring 的增量。

**激活条件**：出现任意 `spring.kafka-sarama.instances.*` 配置即激活（模块注册于
`gs.OnProperty("spring.kafka-sarama")`，前缀匹配 [starter.go:38]）。每个
`spring.kafka-sarama.instances.<name>` 条目创建一个名为 `<name>` 的 `sarama.Client` bean
[starter.go:39-43]。该前缀与 franz-go 版 [starter-kafka](../starter-kafka)（`spring.kafka`）
刻意区分，二者从不同时引入 [config.go:26-28]。

**本 starter 无 messaging.Driver**（与 starter-kafka 不同）：发布/消费是在共享 client
bean 之上的裸 sarama 用法。starter 只在发布/消费接缝**声明**每次操作的身份，信号本身由
resilience 层**发射**。这是头号设计嫌疑 —— 见 §6。

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
    go-spring.org/starter-governance-file latest // 可选：resilience/fault 策略
    go-spring.org/starter-actuator latest  // 可选：探针 + /metrics 挂载
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance-file"
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
    "go-spring.org/cloud/traffic"
    "go-spring.org/spring/gs"
    StarterKafkaSarama "go-spring.org/starter-kafka-sarama"
)

type Service struct {
    // 永远注入裸 sarama.Client bean。producer/consumer 用 sarama 的
    // *FromClient 构造器按需派生 —— 一套连接池与 metadata 缓存服务所有角色
    // （见 README 的设计说明）。
    Client sarama.Client `autowire:"main"`
    // 进程的压测约定，应用提供 traffic.Propagator bean 时注入；nil 则接缝走默认约定。
    Prop traffic.Propagator `autowire:"?"`
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) error {
            return s.publish(ctx, "hello", "value")
        }
    })
}

// publish 发送一条记录：trace context 写入 headers，发送经声明并路由到治理 executor
// （breaker/rate-limit/retry），由后者发射 span、指标与访问日志。
func (s *Service) publish(ctx context.Context, topic, value string) error {
    producer, err := sarama.NewSyncProducerFromClient(s.Client)
    if err != nil {
        return err
    }
    defer producer.Close() // 派生 bean 要在 client Destroy 之前关闭
    // 包裹以声明本次发布并路由到 executor；包装器无论如何都注入 W3C ctx + 压测标记，
    // 因此无条件包裹永远安全（command.go）：
    producer = StarterKafkaSarama.WrapSyncProducer(s.Client, producer, s.Prop)

    msg := &sarama.ProducerMessage{Topic: topic, Value: sarama.StringEncoder(value)}
    _, _, err = producer.SendMessage(msg)
    return err
}
```

**conf/app.properties** —— 上述用到的完整带注释配置面：

```properties
# --- kafka client -----------------------------------------------------------
spring.kafka-sarama.instances.main.brokers=127.0.0.1:9092
# 须与目标集群匹配，消费组等功能才正常（sarama 由此协商协议特性）；留空用 sarama 默认。
spring.kafka-sarama.instances.main.version=3.7.0

# --- producer 调优 ----------------------------------------------------------
spring.kafka-sarama.instances.main.producer.compression=snappy
spring.kafka-sarama.instances.main.producer.required-acks=all

# --- 可选：SASL + TLS（dev broker 为明文） -----------------------------------
#spring.kafka-sarama.instances.main.sasl.enabled=true
#spring.kafka-sarama.instances.main.sasl.mechanism=scram-sha-512
#spring.kafka-sarama.instances.main.sasl.username=user
#spring.kafka-sarama.instances.main.sasl.password=pass
#spring.kafka-sarama.instances.main.tls.enabled=true
#spring.kafka-sarama.instances.main.tls.ca-file=/path/ca.pem
#spring.kafka-sarama.instances.main.tls.cert-file=/path/client.pem
#spring.kafka-sarama.instances.main.tls.key-file=/path/client.key

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
curl -s :9090/metrics | grep messaging_client   # operation + attempt 两个 duration 直方图 + in-flight
```
---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-kafka-sarama
  ├─ init: sarama.Logger = go-spring log 桥接   [starter.go:33]（sarama 事件全转 Info，默认 app tag）
  └─ gs.Module(OnProperty("spring.kafka-sarama")) 任意 spring.kafka-sarama.instances.* 触发
        └─ conf.BindEach → 每个 <name> 条目一个 Config
              └─ Provide(newClient, IndexArg name, IndexArg config, IndexArg 3,?Driver).Name(<name>)
                   .Destroy(destroyClient)     [starter.go:40-44]

gs.Run()
  ├─ newClient [client.go:70]:
  │    1. 可选 Driver bean —— 无则内置 DefaultDriver  [client.go:73-75]
  │    2. d.CreateClient：sarama.NewConfig + version/SASL/TLS/producer 选项，
  │       然后 sarama.NewClient(brokers) —— 这里就会拨号 seed broker 并拉取
  │       metadata，坏 broker/坏凭据/TLS 不匹配在启动期失败，而非首次使用时
  │       爆雷 [client.go:53-69 注释, driver.go:71-112]
  │    3. attachExecutor（在 driver 内、client 刚建好时执行）：
  │       params.ExecutorFor("kafka", "kafka:<brokers>") —— 容器存在时为受治理的
  │       executor（fault 包裹），容器缺席时为仅观测的 resilience.Unmanaged
  │       executor（带一次性警告）—— 以 client 为键存入 sync.Map。治理**在构造
  │       期施加**，故 driver 返回的 client 已是完整的，没有事后补挂步骤
  │       [driver.go CreateClient, command.go attachExecutor]
  │    4. 再防御性探测（ping=true 时）：len(Brokers())==0 → closeResilience +
  │       close + 启动报错                                          [client.go:86-89]
  ├─ 派生 bean 归你所有：注入 client 处用 sarama.New*FromClient 自建
  └─ SIGTERM → Destroy：closeResilience（exec.Close、清 map）→ cl.Close()
       [client.go:96-101, command.go closeResilience]
```
**装配扩展点**：client 装配由 `Driver`（接口，`driver.go:44-66`）负责。公司/伞包 starter 可把
自己的 `Driver` 作为**可选容器 bean** 提供（`gs.Provide(func() StarterKafkaSarama.Driver{...})`，
因为是 bean，可在装配期注入从配置文件绑定的配置）；`spring.kafka-sarama` 下每个实例都经它
构建。`CreateClient` 接收容器的 `cloud.ClientParams` 并返回完整的 client —— 它必须像内置
driver 那样，从 `params` 构建并挂上 executor（见 `attachExecutor`）。没有该 bean 时 starter 在
装配内回退到内置 `DefaultDriver`（`driver.go:67-112`，`client.go:73-75`）。当容器中存在多个
Driver bean 时，实例可按名指定：
`spring.kafka-sarama.instances.<name>.driver = <bean 名>`（留空 = 先回退家族级 `spring.<family>.default.driver`，再按类型注入唯一 Driver bean；指定的
bean 不存在则启动失败）。

派生的 producer/consumer 不是容器 bean —— 需自行在应用退出前关闭（publish 路径里
`defer producer.Close()` 即预期写法，见 [example/main.go:72-76]）。
`sarama.Client.Close` 释放共享 broker 连接。
### 2.2 declare/guard 机制 —— 精确顺序与未保护面

生产端 `WrapSyncProducer(cl, p, prop)` 取出为 `cl` 暂存的 executor，返回一个包装器：
它**声明**每次发送的身份并将其路由到该 executor。包装器总是生效（治理关闭时也是），
因为它同时是 W3C 链路上下文与压测标记的注入点 —— 注入是传播而非发射，无论如何都要发生；
没有 executor 时调用照样执行，只是内联、没有 span。被声明并被保护的面：

```
SendMessage / SendMessages
  → 声明：observability.WithOperation(ctx, operation("publish", msg.Topic))
  → fault.WrapClientExecutor（spring.governance.client.fault 启用时注入故障）
    → resilience executor 包装（span + outcome 计数 + call/attempt duration + 访问日志）
      → resilience executor（breaker / rate limit / retry，策略来自治理中心）
        → 注入 W3C 链路上下文 + 压测标记到 msg.Headers
          → 内层 p.SendMessage（真实 sarama）
```

**未保护**：`Close` 与整个事务家族
（`BeginTxn`/`CommitTxn`/`AbortTxn`/`AddOffsetsToTxn`/`AddMessageToTxn`/`TxnStatus`/
`IsTransactional`）直接透传 —— 它们是控制面，不是被保护的数据路径。同样未声明：
`sarama.AsyncProducer`（无包装器），以及任何忘记包裹的 producer。`SendMessage` 不收
`context.Context`（sarama API 无 ctx），声明只能从消息本身构建，起点是
`context.Background()` —— 发布 span 是一个新的根；逐调用时限要用 resilience 的
`AttemptTimeout`/`MaxDuration`。消费侧由 `Consume` 单独声明（§2.4）。
### 2.3 一次发布，逐层走读

追踪开启时，wrapped producer 上执行 `publish(ctx, "hello", "value")`：

1. `SendMessage` 在 topic `hello` 上声明操作 `publish`（`observability.WithOperation`）
   并进入 executor —— starter 自身不发射任何东西（observe.go）。属性命名空间：
   `messaging.system=kafka`、`messaging.operation=publish`、
   `messaging.destination.name=<topic>`。
2. resilience 层开启发布 span，并在调用级记录 `messaging.client.operation.duration`
   （含重试与退避）与 `messaging.client.active_requests`；在尝试级记录
   `messaging.client.attempt.duration`。它写出 `kafka` 访问 tag 下的一条访问日志。
   publish 与 consume 声明为 `NonIdempotent`，故针对该 label 的重试策略会被**抑制**
   （每个 service 告警一次）：重发或再跑一遍 handler 是第二个副作用，不是第二次尝试。
3. 尝试内部，W3C propagator 把 `traceparent`/`tracestate` 注入 `msg.Headers`。
   ⚠ 注入会**丢弃同 key 的既有 header** 以保证重复注入幂等 ——
   别把业务数据放在 `traceparent` 下。
4. `prop.Inject` 一并写入标记 header（非压测流量下为空操作）
   —— 消费侧据此还原压测标记（§2.4）。
5. sarama 发送并（starter 强制 `Producer.Return.Successes=true` [driver.go:72]）在
   broker ack 后返回 partition/offset（`required-acks=all` → WaitForAll
   [driver.go:131]）。
6. executor 依据它看到的结果关闭 span/日志/指标。发布 span 是一个新的根
   （SendMessage 不带 ctx）；注入到消息头的 `traceparent` 才是把两侧串起来的东西。

### 2.4 一次消费，逐层走读

收到一条 `*sarama.ConsumerMessage`（这里用 partition consumer；ConsumerGroup handler
同样适用）：

1. `Consume(ctx, cl, msg, prop, handle)` 从 `msg.Headers` **提取**上游 trace context
   —— 只要两侧都用接缝，消费 span 就是生产 span 的子 span。
   提取永不改动消息（`consumerCarrier.Set` 是 no-op）。
2. 压测标记 header 读回 ctx（`prop.Extract`）——
   下游业务代码与 client 不靠任何 HTTP header 即可分支。
3. 声明 topic 上的 `consume` 操作，并在 executor 下执行 `handle`，由后者发射观测
   （与 publish 同一指标/日志命名空间；消费 span 的父级来自消息头）。
4. 你的 handler 执行并提交 offset。offset 提交完全由你/消费组负责
   —— starter 从不介入。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.kafka-sarama.instances.<name>.` 下（含 tls/sasl 组共 16 个；
这里是 `conf.BindEach` 的按实例前缀绑定，不是绝对属性的字段注入）。

### 3.1 核心

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `brokers` | string | — | **必填**（`expr:"$ != ''"` [config.go:32]）；逗号分隔 seed 列表 [driver.go:103]。同时原样构成治理服务标签 `kafka:<brokers>` [command.go:221] —— 同一集群写法不同即**不同**标签。 | 缺失/为空 → 绑定报错。broker 写错 → sarama.NewClient 启动失败（fail-fast）。 |
| `version` | string | ""（sarama 默认） | `sarama.ParseKafkaVersion` 解析；决定协议特性（headers、SASL 机制、消费组）[driver.go:65-71]。 | 解析失败 → 启动报错 `invalid kafka version`。过低 → 首次使用才报功能错误。 |
| `ping` | bool | false | 可选：装配后的一次防御性元数据探测，`len(cl.Brokers())==0` 则启动失败 [client.go:77-82]。拨号本身始终由 driver 内的 `sarama.NewClient` 完成，故 `ping=false` 不会让 broker 宕机变静默。 | true → 空集群中止启动；false → 跳过该检查。 |

`driver` key 为实例按名指定 Driver bean：不配置 → 装配由按类型注入的可选 Driver bean（见
§2.1）或内置 `DefaultDriver` 负责；配置 → 按名注入该 bean，指定的 bean 不存在则启动失败。

### 3.2 SASL（`sasl.*`）—— [config.go:64-77]、[driver.go:101-118]

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `sasl.enabled` | bool | false | 整块开关；false 时忽略其余三个 key。 | 对 SASL broker 忘开 → 启动期 metadata 拉取失败。 |
| `sasl.mechanism` | string | `plain` | `plain` / `scram-sha-256` / `scram-sha-512`（大小写不敏感）；SCRAM 每次握手接 xdg-go/scram 客户端生成器 [scram.go:56-63]。⚠ SCRAM 需够高的 `version` —— 查 [sarama 文档](https://github.com/IBM/sarama)。 | 其他值 → 启动报错 `unsupported kafka sasl mechanism`（不会静默回退 PLAIN [driver.go:99-100]）。 |
| `sasl.username` / `sasl.password` | string | "" / "" | 拷入 `cfg.Net.SASL` [driver.go:103-104]。 | 配错 → 启动期 SASL 握手失败（fail-fast）。 |

### 3.3 TLS（`tls.*`）—— 共享 `cloud/security` 块 [config.go:42-46]

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

配置 starter-governance-file，对服务 `kafka|127.0.0.1:9092` 设 breaker/rate-limit 策略：

```go
wrapped := StarterKafkaSarama.WrapSyncProducer(cl, producer, prop)
wrapped.SendMessage(msg)   // 拒绝可见于 _app_kafka_access + resilience 计数
producer.SendMessage(msg)  // 裸句柄依旧不受保护 —— wrapper 不改写 p
```

演练：打过限流阈值 → wrapped 调用返回 resilience 错误（partition/offset `-1/-1`）；
裸调用照常通过。经 `Consume` 声明的消费同样受治理（同一个 `kafka:<brokers>` 标签）——
被限流的消费会从 `Consume` 返回 resilience 哨兵，你的 handler 根本不会执行。

### 4.3 消息往返与 header 存活

同 topic 先发后收（partition consumer 从最旧读，同 [example/main.go:89-112]）：

- `msg.Value` 原样存活（`sarama.StringEncoder` → `string(msg.Value)`）。
- `WrapSyncProducer` 注入的 `traceparent` 在消费侧可读 —— Jaeger 里 `publish` →
  `consume` 是同一条 trace（[example-otel](example-otel/) 冒烟正是对 Jaeger API 断言这点）。
- 压测标记：在带标记的 ctx 下发布（如上游 echo 的 loadtest 中间件），消费侧
  `Consume` 提取之后 propagator 的 `IsLoadTest(ctx)` 为真。
- ⚠ producer 消息上预设的 `traceparent`/标记 header 会被**替换**而非合并。
  任何地方都没有消息 Key 映射（无 driver）—— `msg.Key` 是什么就是什么。

### 4.4 观测量读取

```bash
grep _app_kafka_access app.log | tail      # messaging.system=kafka messaging.operation=publish|consume messaging.destination.name=... status/duration_ms
curl -s :9090/metrics | grep messaging_client   # operation + attempt 两个 duration 直方图 + in-flight，属性 messaging.system=kafka
# span 名："publish"/"consume"，属性 messaging.destination.name=<topic>
```

sarama 自身连接事件以 Info 级、`kafka:` 前缀、默认 app tag 到达 [logger.go:39-51] ——
broker 连接、metadata 刷新、重连都可见。

### 4.5 治理标签核对

executor 标签就是 `kafka|` + `brokers` 字符串 [command.go:221]。策略必须精确匹配该写法；确认：

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
| 无 trace / 无 `_app_kafka_access` | 未引入 starter-otel，或未在接缝声明发布/消费 | 声明在接缝处显式接入 —— 用 `WrapSyncProducer` 包裹、消息过 `Consume`；引入 starter-otel（否则 OTel 全局为 no-op）。 |
| 治理策略不生效 | 服务标签不匹配，或 producer 没包裹 | 标签是 `kafka:<brokers>` 原样 [command.go:221]；用 `WrapSyncProducer` 包裹 —— 裸句柄不受保护。 |
| consumer 重读整个 topic | starter 强制 `Consumer.Offsets.Initial=OffsetOldest` [driver.go:73] | 在消费组 handler 里正确提交 offset；改此默认无配置 key。 |
| 运行期 `SyncProducer` 返回 `ErrOutOfBrokers` | broker 重启且版本/凭据在启动后变了 | sarama 会自动重连；凭据变了需重启应用（client 配置是启动期定死的）。 |
| breaker 熔断但消费侧持续失败 | 消费经 `Consume` 声明，拒绝在那里浮现 | executor 在你的 handler 之前就返回 resilience 哨兵；检查 `resilience.IsRejection(err)`，并且（partition consumer 场景）不提交 offset 以便消息重投。 |
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

- **无 messaging.Driver** —— 家族内唯一无 driver 的 MQ starter；发布声明在接缝处手工
  完成，"driver 的 Key 映射丢字段"这一类问题在本 starter 结构性不存在
  （根本没有映射层可丢 Key）。
- `WrapSyncProducer`/`SendMessage` 从消息推导声明且起点是 `context.Background()` ——
  发布 span 是新根；逐调用时限只能靠 resilience `AttemptTimeout`/`MaxDuration`。
- 无消费组/订阅接缝 → 消费侧是调用方逐消息手工调用的 `Consume`，没有谁替你声明订阅。
- `Return.Successes`/`OffsetOldest` 静默覆盖 sarama 默认且无配置 key [driver.go:72-73]。
- 治理服务标签用原始 `brokers` 字符串 —— 同一集群写法不同即不同熔断域 [command.go:221]。
- resilience executor 索引以 `sarama.Client` 接口值为键存 `sync.Map` —— 对 starter 的
  "每名字一个 client"模型安全，但自定义 Driver 若返回逐次不同的
  wrapper 对象会破坏 `WrapSyncProducer`/`Consume` 的查找。
