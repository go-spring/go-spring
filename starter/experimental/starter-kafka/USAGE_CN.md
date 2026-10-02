# starter-kafka 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`）与可运行的
[example/](example/)、[example-cloudnative/](example-cloudnative/)、
[example-otel/](example-otel/) 核验——文中以 [文件:行号] 标注。**Kafka broker 语义与
franz-go API 属于 [franz-go 官方文档](https://github.com/twmb/franz-go)与
[kafka.apache.org](https://kafka.apache.org/documentation/)**——下文只写 go-spring 的增量。

**激活方式**：任一 `spring.kafka.instances.*` key 即激活（模块为 `gs.Module(gs.OnProperty("spring.kafka"))`
前缀匹配 [starter.go:38]）。每个 `spring.kafka.instances.<name>` 条目创建一个名为 `<name>` 的
`*kgo.Client` bean。**starter 自身不注册 health indicator**——应用侧模式见 §4.1。

---

## 1. 完整工程示例

一个走 messaging driver 的生产 + 消费服务，组合健康探针、metrics、tracing 与运行时治理。
文件：`go.mod`、`main.go`、`service.go`、`conf/app.properties`。

**go.mod**（关键依赖）：`github.com/twmb/franz-go/pkg/kgo`、`go-spring.org/spring`、
`go-spring.org/cloud`、`go-spring.org/starter-kafka`，加可选的 `starter-actuator`
（探针 + /metrics）、`starter-otel`（trace/metric 导出）、`starter-governance-file`（限流/熔断）。

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance-file"
    _ "go-spring.org/starter-kafka"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** —— 通过 driver 收发 `messaging.Message` 信封，外加 health-indicator 逃生口：

```go
package service

import (
    "context"
    "time"

    "github.com/twmb/franz-go/pkg/kgo"
    "go-spring.org/cloud/actuator/health"
    "go-spring.org/cloud/messaging"
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
    gs.Provide(func(s *Service) (messaging.Driver, error) {
        // propagator 传 nil → 回退到 traffic.NewDefaultPropagator(traffic.DefaultBinding())
        return StarterKafka.NewDriver(s.Client, nil), nil
    })
    gs.Provide(func(b messaging.Driver) gs.Runner {
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
spring.kafka.instances.a.brokers=127.0.0.1:9092
spring.kafka.instances.a.topic=hello
spring.kafka.instances.a.group=hello-group
spring.kafka.instances.a.producer.required-acks=all
spring.kafka.instances.a.producer.compression=snappy

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
# NOTE: governance RULES go in conf/governance.properties, referenced by spring.governance.source.file.path in app.properties (see starter-governance-file USAGE).
spring.governance.enabled=true
spring.governance.driver=default
spring.governance.client.default.rate-limit=8
```

**验证**（broker 启动同 [example/docker-compose.yml](example/docker-compose.yml) —— KRaft
模式、自动建 topic、端口 127.0.0.1:9092）：

```bash
docker compose up -d                        # bitnami/kafka:3.7，KRaft 单节点
# 等端口就绪，broker 启动慢（约 30s）
go run .                                    # broker 不可达则启动快速失败
curl -s :9370/readyz | jq .                 # kafka:a 组件（应用侧 indicator）
grep messaging.access app.log | tail -3         # publish/consume 访问记录
```

自断言冒烟脚本是 [example/check.sh](example/check.sh)（`./check.sh`）；治理/健康/dync 组合见
[example-cloudnative/](example-cloudnative/)（`go run .` 依次打印 health / round-trip /
resilience / 热加载行并以 0 退出）。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-kafka
  └─ gs.Module(OnProperty("spring.kafka"))：任一 spring.kafka.instances.* key 存在时触发
        └─ conf.BindEach("${spring.kafka}") → 每个 <name> 条目一个 Config
              └─ Provide(newClient, IndexArg(name,c), IndexArg(3,?Driver)).Name(<name>) [starter.go:39-45]
                    .Destroy(destroyClient)

gs.Run()
  ├─ ctor newClient [starter.go:96]:
  │   1. 可选 Driver bean——无则用内置 DefaultDriver              [starter.go:101-102]
  │   2. d.CreateClient：完整 client 装配并在此完成——driver 在构建时即
  │      装配治理：AttachGovernance → params.ExecutorFor("kafka",
  │      "kafka:<brokers>")                                  [driver.go:78,122]
  │      （bundle 有值时 = fault.WrapClientExecutor(mgr.ClientExecutorFor(
  │      "kafka", service), service, inj)；bundle 为零值时 = 仅观测的
  │      resilience.Unmanaged executor），以 client 指针索进包级 sync.Map
  │                                                         [command.go:83-89]
  │   3. 再探测（`ping=true` 时）：Ping 10s 超时——坏 brokers/凭证/TLS 让启动失败，
  │      而不是等第一次 produce 才暴露；探测失败释放 driver 刚装配的 executor
  │                                                         [starter.go:113,119]
  ├─ 无 Init 钩子；*kgo.Client bean 在 ctor 后即就绪；此后无任何补装配——
  │  治理已在构造器内应用
  ├─ Destroy(destroyClient) [starter.go:130]:
  │   closeResilience（executor Close）→ Flush(10s ctx) → Close。
  │   ⚠ Flush 失败会记 ERROR 日志（可能丢消息）并向上返回——未送达 broker 的缓冲记录丢失。
```

**装配扩展点**：client 装配由 `Driver`（接口，`driver.go:37-63`）负责。公司/伞包 starter 可把
自己的 `Driver` 作为**可选容器 bean** 提供（`gs.Provide(func() StarterKafka.Driver{...})`，
因为是 bean，可在装配期注入从配置文件绑定的配置）；`spring.kafka` 下每个实例都经它构建。
没有该 bean 时 starter 在装配内回退到内置 `DefaultDriver`（`driver.go:66-124`，
`starter.go:100-102`）。当容器中存在多个 Driver bean 时，实例可按名指定：
`spring.kafka.instances.<name>.driver = <bean 名>`（留空 = 先回退家族级 `spring.<family>.default.driver`，再按类型注入唯一 Driver bean；指定的
bean 不存在则启动失败）。

`CreateClient` 接收治理 bundle，必须调用 `AttachGovernance` 才算完成 client——guard 以 driver
返回的原始 `*kgo.Client` 指针为键，自定义 Driver 只能在那里装配；正是这个以指针为键的
resilience 注册表，让 `GuardedProduceSync` 可以是接收原始 bean 的自由函数：guard 直接解析
executor，无需包装 client 类型 [command.go:83-89,102-109]。

### 2.2 client 装配 —— DefaultDriver 按序安装什么

`DefaultDriver.CreateClient` 组装一次 `kgo.NewClient` 调用并随后完成它 [driver.go:78-124]：

1. `kgo.SeedBrokers(strings.Split(c.Brokers, ",")...)` —— brokers 是 CSV；每项只是 seed，
   client 自行学习完整集群拓扑。
2. `kgo.WithHooks(kt.Hooks()...)` [driver.go:75] —— **kotel（tracer+meter）钩子**。broker/
   client 级 span 与 metric（`messaging.kafka.*`）归 kotel；逐消息信号由本 starter **声明**、
   由 resilience executor **发射**（§2.3、§4.5），故不再装第二个钩子，信号不重复。
3. `kgo.WithLogger(newLogger())` —— franz-go 内部日志（broker 连接、请求失败、重连）经
   go-spring log 桥接，tag `log.TagAppDef`，Info 级阈值 [driver.go:195-211]。
4. `kgo.ConsumerGroup` / `kgo.ConsumeTopics` 来自 `group`/`topic` 配置——**构造期固定**；
   这是 driver 继承的 franz-go 约束（§2.3）。
5. SASL mechanism、TLS（`c.TLS.BuildClient()`——裸 `BuildClient`，客户端侧 key 见 §3）、producer 选项
   （compression/acks/batch/linger）。
6. `AttachGovernance(cl, c.Brokers, params)` [driver.go:122]——治理最后装配，就在构造器内，
   故返回的 client 即完整（见 §2.1 第 2 步与 §4.5）。

只有启动 ping **有意不放**在 driver 里：它是 starter 的生命周期职责 [starter.go:113]。
resilience 接线**现在就在** driver 内：随 client 构建时应用，而非稍后的补装配。

### 2.3 driver 的一次发布与一次消费，逐层走读

绑定到 topic `hello` 的 publisher 上 `Publish(ctx, msg)` [client.go:78-94]：

1. 信封 → `kgo.Record`：`Topic` = publisher 绑定的目的地，`Value` = Payload，
   `Headers` = 信封 headers（空则 nil），`Key` 仅非空时设置 [client.go:79-86]。
   ⚠ `msg.Timestamp` **不映射**——由 broker 盖时间戳。
2. OTel propagator 经 `recordCarrier` 把 W3C trace context 注入 record headers
   [client.go:87]；无 starter-otel 时为 no-op。
3. `prop.Inject` 把压测标记写入 record header（非压测流量下为空操作），
   消费侧可识别合成流量。
4. `GuardedProduceSync(ctx, p.cl, rec).FirstErr()` —— 同步生产，走与裸 client API 同一个
   resilience executor（没有规则命中该 client 的 service label 时为透明 no-op）
   [client.go:93-99]，broker ack / 拒绝直接返回给调用方。
5. `GuardedProduceSync` **声明**发布的身份（`observability.WithOperation`，span 名 `publish`，
   metric 前缀 `messaging.client`，topic 进 Detail），再把调用交给该 client 的 resilience
   executor，由它发射 span、调用级/尝试级时长直方图、调用计数器与唯一一条访问日志
   [command.go, observe.go]。访问日志 tag：`messaging.access`（observe.go 的
   `RegisterAppTag("messaging","access")`）。client 内部 kotel 的 produce span 与 client
   metric 照常触发（本 starter 只是启用的第三方插桩）。

绑定到 source `hello` 的 subscriber 上 `Subscribe(ctx, handler)` [client.go:109-144]：

1. `messaging.Recover` 把 handler panic 转为 error 路径 [client.go:112]。
2. 一个后台 goroutine 在 ctx（经 `context.WithoutCancel` + Close 持有的 cancel）上轮询
   `cl.PollFetches` [client.go:113]。
3. poll 错误打日志（tag `log.TagAppDef`），`context.Canceled` 抑制 [client.go:121-127]。
4. 逐条 record：给了 source 且 `rec.Topic != s.topic` 则过滤——source 与所配 topic 都不
   匹配时静默过滤掉一切 [client.go:129-131]。
5. 从 record headers 提取 trace context；压测标记恢复到消息 ctx；并**声明**消费的身份
   （`operation(opConsume, rec.Topic)`）[client.go:132-140]。
6. handler 在**该 client 的 resilience executor 之下**收到 `fromRecord(rec)`：Key/Payload/
   Headers/Timestamp 全部在映射中存活 [client.go:172-186]；executor 发射消费的 span、metric
   与访问日志。⚠ handler 错误事后仍**只打日志**——本 driver 无 nack/重投（franz-go 组消费
   照常提交；设计嫌疑，§6）——且消费声明为非幂等，故命中的治理规则也无法在进程内重试
   handler：再跑一次就是第二个副作用。
7. `Close` 取消循环并等 `done`；`sync.Once` 保证幂等 [client.go:146-156]。

franz-go 构造约束带来的两个 driver 陷阱 [client.go:43-51 注释]：
`NewSubscriber(ctx, source, group)` **静默丢弃 `group` 实参**（用 client 的 `group` 配置
——client.go:68）；且一个 client 只是一个 consumer——每个逻辑 consumer 用一个 client bean。

### 2.4 GuardedProduceSync —— resilience 路径的精确语义

franz-go 的异步 `Produce` 立即返回，因此只有同步路径可包 [command.go:18-21,69-74,137-141 注释]。
`GuardedProduceSync(ctx, cl, recs...)` [command.go:142-159]：

- guard 解析 `cl` 上挂的 executor；没有（governance 关）则原样内联执行——与
  `cl.ProduceSync` 行为一致。
- 发布的身份在进入 executor **之前**就按首条 record 的 topic 声明，故其 span 名为 `publish`，
  metric/访问日志带上 topic（进 Detail，绝不进 metric label）[command.go]。
- governance 开启时，调用经过已组装好的 executor
  `fault.WrapClientExecutor(mgr.ClientExecutorFor("kafka", service), service, inj)` [command.go:86]：运行时
  故障注入与 resilience 结果 metric 包住 produce，服务标签 `kafka:<brokers>`
  （`resilience.ServiceLabel("kafka", c.Brokers)` [starter.go:83]，格式 `prefix:name`
  [resilience/policy.go:216-230]）。
- 被拒（限流 / 熔断打开）时 produce **绝不执行**；拒绝错误编码为逐 record 错误，调用方的
  `.FirstErr()` 像真实 produce 失败一样拿到它 [command.go:150-157]。example-cloudnative
  断言突发流量会得到 `resilience.ErrRateLimited`。
- **不受保护**的路径：直接在 client bean 上调裸 `ProduceSync`/`Produce`。driver 的 publish
  **已受保护**（§2.3 第 4 步），driver 的消费 handler 亦已受保护（§2.3 消费第 6 步）。
  想让某个 client 事实上不受治理：给它的 service label（`kafka:<brokers>`）配一条
  所有旋钮都为 0 的 rule —— Rule 是整体替换 default，全零 rule 即透传。

### 2.5 GuardedConsume —— 自持 poll 循环的受管入口

裸 `*kgo.Client` bean 的 `PollFetches` 拦不住（franz-go 的 hook 只观察一条 record，包不住
调用），所以应用自己轮询 client 时，消费就没有限流/熔断/重试/超时、没有 `messaging.*`
指标、没有访问日志。`GuardedConsume(ctx, cl, rec, fn)` [command.go] 正是给这种形态准备的、
`GuardedProduceSync` 的消费侧对偶：

- 按 `rec.Topic` 声明本次消费的身份（span `consume`、`messaging.*` 指标、访问日志），
  再把 `fn` 交给挂在 `cl` 上的 guard 执行。
- driver 的订阅循环对每条投递的 record 调用的就是这个函数，所以受管循环与自持循环不会漂移。
- 异步 `Produce` 没有对应入口：它在 broker 确认前就返回，包 executor 无意义 —— 用异步
  produce 的应用即接受该次调用不受治理、不上报。

---

## 3. 逐 key 行为参考

key 都在 `spring.kafka.instances.<name>.*` 下——ctor 参数经 `conf.BindEach` 绑定（真正的按实例前缀
绑定）。`value:` tag 已与源码核对：共 21 个 key。

### 3.1 核心

| Key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `brokers` | string | — | **必填**（`expr:"$ != ''"` [config.go:30]）；CSV seed brokers；同时构成 resilience 服务标签 `kafka:<brokers>`。 | 空 → 启动报错；`ping=true` 时错但可达的主机在 10s 启动 Ping 处失败。 |
| `topic` | string | "" | 传给 `kgo.ConsumeTopics`——消费 topic 构造期固定；driver subscriber 按它过滤。空 = 纯生产 client。 | 能生产、消费永不投递（未订阅 topic）。 |
| `group` | string | "" | 传给 `kgo.ConsumerGroup`；group 语义属 Kafka 自身（offset、rebalance——见 kafka.apache.org）。⚠ driver `NewSubscriber` 的 group 实参是死的——本 key 是唯一 group 开关。 | 空 + 有 topic = 无 group（随机 group/急切）消费；offset 不提交。 |
| `ping` | bool | false | 可选启动连通性探测：`cl.Ping`，10s 超时 [starter.go:116-123]。 | true → brokers 不可达则中止启动；false → 首次 produce/consume 才暴露。 |

`driver` key 为实例按名指定 Driver bean：不配置 → 装配由按类型注入的可选 Driver bean（见
§2.1）或内置 `DefaultDriver` 负责；配置 → 按名注入该 bean，指定的 bean 不存在则启动失败。（下方
conf 里的 `driver` 指治理的 `spring.governance.driver` 选择规则源，与本 starter 无关。）

### 3.2 SASL

| Key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `sasl.enabled` | bool | false | 控制 mechanism 装配 [driver.go:94-100]。 | 开而无凭证 → `ping=true` 时启动 Ping 认证失败。 |
| `sasl.mechanism` | string | `plain` | `plain` / `scram-sha-256` / `scram-sha-512`，大小写不敏感 [driver.go:127-138]。 | 其他值 → 启动 CreateClient 报错。 |
| `sasl.username` / `sasl.password` | string | "" | 传给 mechanism。 | 错 → `ping=true` 时启动 10s Ping 失败。 |

### 3.3 TLS

共享 `security` 块——跨 starter 属性名统一 [config.go:43-47]。

| Key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `tls.enabled` | bool | false | `c.TLS.BuildClient()` → `kgo.DialTLSConfig` [driver.go:100-108]。 | — |
| `tls.cert-file` / `tls.key-file` | string | "" | mTLS 客户端证书对。 | 只配一半 → `tls.Build` 启动报错。 |
| `tls.ca-file` | string | "" | 校验 broker 的 CA。 | 私有 CA 下缺失 → `ping=true` 时 Ping TLS 失败。 |
| `tls.server-name` | string | "" | SNI/校验名。 | 失配 → 校验失败。 |
| `tls.insecure-skip-verify` | bool | false | 跳过校验。 | 生产置 true = 默默暴露于 MITM。 |

### 3.4 Producer

零值保持 franz-go 默认 [config.go:79-80]。

| Key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `producer.compression` | string | "" | `none`/`gzip`/`snappy`/`lz4`/`zstd`，大小写不敏感 [driver.go:173-187]。 | 其他值 → 启动 CreateClient 报错。 |
| `producer.required-acks` | string | `all` | `all`→AllISRAcks；`leader`/`none` 还会**关闭幂等写**（协议要求）[driver.go:141-160]。 | 其他值 → 启动报错；弱化 acks 会静默丢幂等。 |
| `producer.max-batch-bytes` | int32 | 0 | >0 时 `kgo.ProducerBatchMaxBytes`。 | 低于 broker 消息上限 → 逐条 produce 报错。 |
| `producer.linger` | duration | 0s | >0 时 `kgo.ProducerLinger`；吞吐/延迟权衡。 | — |

---

## 4. 验证与故障演练

### 4.1 健康（应用侧 indicator）

starter 不注册 indicator；应用自己导一个（如 §1 / example-cloudnative
[main.go:76-97]）：

```bash
curl -s :9370/readyz | jq .          # 组件 "kafka:a"，readiness+startup 组，critical
docker stop starter-kafka            # Ping 探针失败
curl -s :9370/readyz                 # 503 OUT_OF_SERVICE
```

放进 readiness/startup（绝不放 liveness——broker 故障不应重启 pod）由应用经
`HealthGroups()` 决定。

### 4.2 消息往返 + driver 映射存活字段

```bash
go run .    # §1 服务：publish Key=k1 Payload=value Header origin=demo，消费打印
grep messaging.access app.log | tail -2   # publish 记录（带时长）+ consume 记录
```

存活字段：消费侧 Key、Payload、Headers、broker 盖的 Timestamp [client.go:172-186]。
丢弃项：发布侧 `msg.Timestamp` 被忽略；与 W3C trace key 或压测头同名的 `Headers` 条目会被
注入步骤覆盖 [client.go:87-92]。

### 4.3 受保护 vs 不受保护的生产路径

```bash
# example-cloudnative 配 spring.governance.client.default.rate-limit=8：
go run .    # 打印 "resilience: N produce admitted, M rejected with ErrRateLimited"
```

同一突发走 **driver** publisher 会被同样限流——它走同一个 executor（§2.3 第 4 步）；
只有直接在 client bean 上裸调 `ProduceSync` 才绕过。改被监听源里的 `spring.governance.*` 可免重启
换策略（治理中心热加载）。

### 4.4 治理标签核对

服务标签是 `kafka:<brokers>`——就是 `brokers` 字符串本身，不是按 topic 或按 client 名
[starter.go:83]。两个共享同一 broker 列表的 client bean 共享一个 limiter/breaker；边压
`GuardedProduceSync` 边看 resilience 结果计数（`curl -s :9370/metrics | grep resilience`）
即可验证。

### 4.5 Traces / metrics / 日志 tag

- 本 starter **声明**每个操作的身份（span 名 `publish`/`consume`，metric 前缀 `messaging.client`，
  label 为 `messaging.system` + `messaging.operation`，topic 进 Detail）；**resilience 层发射**
  信号，且只在能看到整次调用的那一点发——span、调用级 `messaging.client.operation.duration`、
  尝试级 `messaging.client.attempt.duration`、在途 `messaging.client.active_requests` 表
  与访问日志 [observe.go, command.go]。两个方向都覆盖：生产路径（`GuardedProduceSync`）与
  消费路径（每条投递的 record）。
- kotel 另外在 client 内发 broker/client 级 span + `messaging.kafka.*` metric——本 starter 只是
  启用的第三方插桩，此处未改（见 [kotel](https://github.com/twmb/franz-go/tree/main/plugin/kotel)）。
  example-otel 配 OTLP gRPC → :4317 Jaeger；在 Jaeger UI 检查生产/消费 span 的链路关联
  （trace context 随 record headers 走，§2.3 第 2/5 步）。
- 访问日志 tag `messaging.access`（渲染形如 `_app_messaging_access`）：op 名 `publish` 与
  `consume`，都带时长与 topic；级别随结果而定（失败 Warn，带 topic 的成功 Debug，不带 topic 的
  成功 Info）。
- franz-go client 内部日志（重连、请求失败）出现在 `log.TagAppDef` 下，Info 阈值桥接
  [driver.go:195-211]。

### 4.6 broker 宕机 ping 探测

```bash
docker compose down && go run .   # ping=true 时约 10s 内启动失败："failed to ping kafka: <brokers>"
```

运行中 broker 失联表现为日志里的 poll 错误（tag `log.TagAppDef`）与逐次 produce 错误；
franz-go 自动重连（其自身语义，见 franz-go 文档）。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 "failed to ping kafka" | brokers 不可达 / SASL 错 / TLS 失配 | 修连通性或凭证；10s 探针仅在 `ping=true` 时执行。 |
| 启动失败 "unsupported kafka sasl mechanism / required-acks / compression" | 枚举 key 拼写错误 | 枚举精确匹配（大小写不敏感）；改对值。 |
| driver 消费者收不到 | `NewSubscriber` source ≠ 所配 `topic`，或 `topic` 为空 | source 必须等于 client 的 `topic`；否则静默过滤。 |
| driver 消费 group "不生效" | `NewSubscriber` 的 group 实参是死的 | 配 `spring.kafka.instances.<name>.group`（构造期固定）。 |
| spring.governance.* 已开但无限流 | 直接在 client bean 上裸调 `ProduceSync` | 只有 `GuardedProduceSync`/`GuardedConsume`（与 driver publisher/消费 handler）受保护。 |
| 无 traces/metrics | 未 import starter-otel | kotel 与 resilience 发射器都挂 OTel 全局；import starter-otel。 |
| 无 `messaging.client.*` metric/日志 | 该调用绕过了声明 | 只有 `GuardedProduceSync`、`GuardedConsume` 与 driver publish/消费 handler 会声明；在 client bean 上裸调 `ProduceSync`/`PollFetches` 什么都不发。 |
| 无访问日志 | 日志 tag 被过滤 | 检查 `messaging.access` tag 过滤。 |
| handler 错误只留一行日志 | 设计如此：本 driver 无 nack/重投 | 在 handler 内自建重试，或用 messaging 的 retry.go。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 17（核心 3 + sasl 4 + tls 6 + producer 4） |
| 必填 | 1（`brokers`） |
| quickstart 前置外部依赖 | 1（Kafka broker） |
| "注意/坑"条数 | 6 |

设计嫌疑（审计台账；自上一版承继，均未修复）：

- driver 丢弃 `NewSubscriber` 的 `group` 实参，source 失配静默过滤 [client.go:68,129-131]——违背 fail-fast。
- 消费 handler 错误只打日志，无 nack/重投 [client.go:137-139]——与 Recover 注释的说法相悖 [client.go:110-112]。
- ~~`destroyClient` 丢弃 Flush 错误~~已修：Flush 失败记 ERROR（点名丢数据后果）并从 destroy 钩子向上返回。
- starter 不注册 health indicator（家族不对称：go-redis/redigo 都注册）。
- resilience executor 以 `*kgo.Client` 指针为键存包级 sync.Map [command.go:83-89]——guard 在 `CreateClient` 内（经 `AttachGovernance`）装配；自定义 Driver 返回 client 却不调用它会静默跳过 guard。
