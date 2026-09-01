# starter-pulsar 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均与 starter 源码
（`config.go`、`starter.go`、`client.go`、`command.go`、`driver.go`、`binder.go`）及可运行的
[example/](example/) / [example-otel/](example-otel/) 核对，括号内为 file:line 抽查点。
**Pulsar 自身语义（订阅、消息 key、properties、保留/重投）见
[Pulsar 官方文档](https://pulsar.apache.org/docs/next/client-libraries-go/)** —— 下文只写
go-spring 的增量。

**激活条件**：出现任意 `spring.pulsar.*` 配置 —— 模块注册于
`gs.OnProperty("spring.pulsar")`（前缀匹配）[starter.go:38]。每个 `spring.pulsar.<name>`
条目创建一个**裸 `pulsar.Client` bean，名为 `<name>`** [starter.go:39-44] —— 设计上没有
包装类型：pulsar 除了 client 本身没有值得包裹的实体 [client.go:17-23]。

---

## 1. 完整工程示例

一个经 messaging.Binder 同时做生产与消费的服务，含原生指标、OTel tracing 与治理。
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
    go-spring.org/starter-governance   latest   // 可选：熔断/限流策略
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "demo/messaging"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-pulsar"
)

func main() { gs.Run() }
```

**messaging.go** —— binder 发布 + 消费，以及受治理保护的裸路径：

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
    // 把注入的裸 client 适配为 broker 无关的 Binder。gs.TagArg("main")
    // 取 spring.pulsar.main 实例；binder bean 此处未命名。
    gs.Provide(StarterPulsar.NewBinder, gs.TagArg("main"))

    gs.Provide(func(b messaging.Binder) (gs.Rooter, error) {
        // Publisher：binder 为 topic 惰性创建一个 producer。
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
        // handler 出错 → Nack → Pulsar 重投 [binder.go:147-151]。
        err = sub.Subscribe(context.Background(), func(ctx context.Context, m *messaging.Message) error {
            return process(ctx, m) // Key/Payload/Headers/Timestamp 均在往返中保留
        })
        if err != nil {
            return nil, err
        }

        return func(ctx context.Context) error {
            // binder 发布：span + trace context 注入内置 [binder.go:98-101]。
            return pub.Publish(ctx, &messaging.Message{
                Key:     "user-42",                    // 成为 Pulsar 消息 key
                Payload: []byte(`{"amt":100}`),
                Headers: map[string]string{"trace-ctx": "biz"}, // 成为 Properties
            })
        }, nil
    })
}

// 受保护的裸路径：只有经 GuardedSend 的 Producer.Send 会被 resilience 包裹。
func guarded(ctx context.Context, cl pulsar.Client, p pulsar.Producer) error {
    msg := &pulsar.ProducerMessage{Payload: []byte("x"), Key: "user-42"}
    ctx, span := StarterPulsar.StartProducerSpan(ctx, msg) // 手动 span 助手
    id, err := StarterPulsar.GuardedSend(ctx, cl, p, msg)
    StarterPulsar.EndSpan(span, err)
    _ = id
    return err
}
```

**conf/app.properties** —— 上述用到的完整配置面：

```properties
# --- pulsar client（实例 "main"）---------------------------------------------
spring.pulsar.main.url=pulsar://127.0.0.1:6650
spring.pulsar.main.fail-fast=true
# 对未分区 topic 的 lookup 即使 topic 不存在也会成功，
# 因此普通 topic 在全新 standalone 集群上是安全的探测目标。
spring.pulsar.main.health-check-topic=persistent://public/demo/orders
spring.pulsar.main.operation-timeout=30s
spring.pulsar.main.connection-timeout=5s

# --- 原生 Prometheus 指标（pulsar_client_*，按实例独立 registry）-------------
spring.pulsar.main.metrics.enabled=true
spring.pulsar.main.metrics.port=9091
spring.pulsar.main.metrics.path=/metrics

# --- 受保护调用的访问日志（off/brief/detailed）--------------------------------
spring.pulsar.main.observability.level=brief
spring.pulsar.main.observability.maxArgBytes=512

# --- actuator + otel ----------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # OTel 指标仅经 actuator 暴露

# --- 治理（资源 pulsar|pulsar://127.0.0.1:6650 的熔断/限流）-------------------
govern.source.file.path=conf/govern.yaml
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
go run .                                   # broker 不可达时 fail-fast 探测中止启动
curl -s :9091/metrics | grep pulsar_client_ # 原生 client 指标
curl -s :9370/metrics | grep messaging      # observe-kit 逐消息指标
grep -E '_app_pulsar|pulsar' app.log        # binder + client 日志行（tag _app_def）
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-pulsar
  └─ gs.Module(OnProperty("spring.pulsar")) 在任意 spring.pulsar.* key 存在时触发
        └─ conf.BindEach("${spring.pulsar}") → 每个 <name> 条目一份 Config     [starter.go:38-45]
              └─ Provide(newClient, IndexArg name+config).Name(<name>).Destroy(destroyClient)

gs.Run()
  ├─ ctor newClient [starter.go:55-84]：
  │    1. driverRegistry 查找 driver —— 未知名 → 启动报错                   [starter.go:58-62]
  │    2. d.CreateClient：ClientOptions、认证（mTLS>token 文件>token）、TLS、
  │       原生 Prometheus registry + :port /metrics server、日志桥、
  │       pulsar.NewClient                                                [driver.go:69-115]
  │    3. FailFast 探测：cl.TopicPartitions(HealthCheckTopic) —— 一次
  │       覆盖地址+认证+TLS 的 lookup（不产生消息）；失败 →
  │       cl.Close + metrics 下线 + 启动报错                              [starter.go:68-75]
  │    4. applyResilience：fault.WrapExecutor(resilience.ExecutorFor("pulsar:<url>"))
  │       → resilience.WrapExecutor → 按 client 索引                     [command.go:224-230]
  ├─ 就绪：无 health indicator —— 探测只在启动期生效
  └─ SIGTERM → destroyClient [client.go:44-49]：closeResilience（executor Close）
       → cl.Close()（释放全部 producer/consumer）→ shutdownMetrics（:port server）
```

注意 `newLogger()` 把 pulsar 内部日志（连接/重连/lookup 失败）桥接进 go-spring 日志，
tag 为 `_app_def`，前缀 `pulsar: ` [driver.go:218-233]。

### 2. guard/wrap 机制 —— 精确包裹顺序与未保护面

ctor 里挂上的 resilience executor 只经**一个 seam** 驱动：

```
GuardedSend(ctx, cl, producer, msg)                       [command.go:263-274]
  └─ guard: resilienceExecs.Load(cl)                      [command.go:243-250]
       ├─ 未找到（治理关闭）→ producer.Send 原样内联执行，与裸调用一致
       └─ 找到 → exec.Execute(ctx, "pulsar:<url>", send) —— fault 注入器最外层
                  （fault.WrapExecutor），resilience observer 最内层；拒绝时返回
                  resilience 哨兵错误，发送根本不会上线
```

`applyResilience` 内部包裹顺序 [command.go:225-226]：`resilience.ExecutorFor(resource)`
（核心熔断/限流/重试）→ `fault.WrapExecutor`（运行期故障注入在 executor **外**——注入的
故障不消耗熔断预算）→ `resilience.WrapExecutor`（outcome 计数 + 访问日志最外层）。

**未保护面**（均有源码注释说明是有意的）：
- `producer.SendAsync` —— 刻意不碰；异步路径没有可拒绝的同步结果 [command.go:261-263]。
- binder 的 `Publish` —— **现已受保护**：binder 走 `GuardedSend` 与 client 级 executor
  [binder.go]，span+trace 注入与熔断/限流/fault 都有。实例 key `governance=false` 可让
  所有调用路径裸跑。
- 消费侧 `Receive`/handler —— 没有消费端保护。
- `CreateProducer`/`Subscribe`/`TopicPartitions` —— 生命周期调用，仅启动期 FailFast
  探测覆盖。

### 2.3 一次 binder 发布，逐层走读

`pub.Publish(ctx, msg)`，其中 `Key: "k"`、`Headers: h`、ctx 带压测标记
[binder.go:81-102]：

1. header 拷贝：若 `traffic.IsLoadTest(ctx)`，headers 被**复制**（绝不改调用方的 map）
   并追加 `x-load-test=1` [binder.go:85-90]。
2. 信封 → `pulsar.ProducerMessage`：`Payload`、`Properties`（= headers）、**Key 仅在
   非空时设置** [binder.go:91-97]。未映射：`Timestamp`（信封有，但 Pulsar 发布时间由
   服务端定）及一切 Pulsar 专有字段（OrderingKey、DeliverAt……）。
3. `startProduce` 打开 kit 观测（span `publish`、指标、固定 **brief** 级访问日志 ——
   实例级 `observability` 配置到不了这条路径，见 §6），并把 W3C trace context 注入
   `pm.Properties` [command.go:184-191]。
4. `producer.Send(ctx, pm)` —— 同步，阻塞到 broker ack。
5. `sp.End(err)` 在三路信号上记录结果。

### 2.4 一次 binder 消费，逐层走读

`sub.Subscribe(handler)` 启动一个后台循环 [binder.go:119-155]：

1. handler 先包 `messaging.Recover` —— panic 转为普通错误 → Nack → 重投，绝不会
   打穿 SDK goroutine [binder.go:121-123]。
2. 循环 ctx 派生自 `context.WithoutCancel(ctx)` —— 只有 Close 显式取消，调用方 ctx
   取消不影响 [binder.go:123]。
3. `c.Receive` → `startConsume` 从 `msg.Properties()` 提取上游 trace，开 consumer 子
   span [command.go:195-198]。
4. 压测标记：若生产者往 Properties 里写了 `x-load-test`，handler ctx 会经
   `traffic.WithLoadTest` 重新打标 [binder.go:141-143]。
5. `fromPulsarMsg` 反向映射：Pulsar `Key()` → 信封 Key、`Payload()`、`Properties()` →
   Headers（含注入的 `traceparent` —— 消费侧 headers 会多出 key）、`PublishTime()` →
   Timestamp [binder.go:171-178]。两个方向都保留 Key 与 Properties。
6. handler 出错 → `Nack`（按 Shared 订阅语义重投）+ 错误日志；成功 → `Ack(msg)`，
   ack 失败记 WARN（有重投风险）[binder.go:145-151]。
7. 非 ctx 取消的 `Receive` 错误记日志后循环重试 [binder.go:131-137]。

Close 顺序：取消循环 ctx → 等 `done`（在途 handler 收尾）→ `consumer.Close()`
[binder.go:157-168]；publisher Close 只调 `producer.Close()` 且**丢弃其错误**
[binder.go:104-107]。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.pulsar.<name>.`（BindEach 按实例前缀绑定，不是绝对属性的 Pool
规则）。grep 得到 18 个 value tag —— 下表全覆盖。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `url` | string | — | **必填**（`expr:"$ != ''"`）。`pulsar://` 明文或 `pulsar+ssl://` TLS。同时构成 resilience 资源标签 `pulsar\|<url>` [starter.go:76]。 | 缺失/空 → BindEach 启动报错。 |
| `operation-timeout` | duration | 30s | producer/订阅/lookup 超时，传给 ClientOptions [driver.go:72]。 | 过低 → CreateProducer 间歇失败。 |
| `connection-timeout` | duration | 5s | TCP 连接超时 [driver.go:73]。 | — |
| `token` | string | — | JWT token 值，或 `token-from-file=true` 时的文件路径 [driver.go:86-90]。⚠ 认证优先级：mTLS cert+key 高于 token。 | token 错 → FailFast 探测启动失败。 |
| `token-from-file` | bool | false | 把 `token` 切换为路径解释。 | true 配了字面 token → 文件打开失败。 |
| `tls-trust-certs-file` | string | — | 校验 broker 的 PEM CA bundle [driver.go:74]。 | `pulsar+ssl://` 下缺失 → 探测期握手失败。 |
| `tls-cert-file` | string | — | 客户端证书；与 `tls-key-file` 成对时还经 `NewAuthenticationTLS` 成为 mTLS 认证器 [driver.go:84-85]。⚠ 只配 cert 不配 key → 静默无认证。 | — |
| `tls-key-file` | string | — | 与证书配对的客户端私钥 [driver.go:76]。 | — |
| `tls-allow-insecure` | bool | false | 关闭服务端证书校验。生产禁用。 | true → MITM 暴露。 |
| `tls-validate-hostname` | bool | false | 证书内主机名校验；默认保持 pulsar-client-go 默认值 [config.go:63-66]。 | — |
| `fail-fast` | bool | true | 启动期 `TopicPartitions` 探测 [starter.go:68-75]。 | false → broker 挂了要到首次生产才暴露。 |
| `health-check-topic` | string | `persistent://public/default/__health_check` | 探测目标；未分区 topic 的 lookup 即使不存在也成功 [config.go:72-76]。 | 分区/乱写 topic 名 → 探测报错挡启动。 |
| `metrics` | group | — | 结构绑定 `value:"${metrics}"` [config.go:79]。 | — |
| `metrics.enabled` | bool | true | 启动按实例的 `/metrics` server 并接入独立 registry [driver.go:97-101]。 | false → 任何地方都没有 `pulsar_client_*`。 |
| `metrics.port` | int | 9091 | 该 server 的端口。⚠ 固定默认：每个开 metrics 的实例必须各配独立端口；冲突时后起的 server 静默监听失败（错误被吞 [command.go:69-71]）。 | 两实例同端口 → 一个 metrics 端点静默死亡。 |
| `metrics.path` | string | `/metrics` | 该 server 的 HTTP 路径 [command.go:62]。 | — |
| `observability` | group | — | `observe.ObserveConfig` `value:"${observability:=}"` [config.go:85]。只作用于 GuardedSend 的 executor observer —— 不影响 binder 路径（§6）。 | — |
| `driver` | string | `DefaultDriver` | driver 注册表查找；`RegisterDriver` 重名 panic [driver.go:53-58]。 | 未知名 → 启动报错 "pulsar driver not found"。 |
| `governance` | bool | true | 为实例挂 resilience/fault executor；同时保护 `GuardedSend` 与 binder 的 `Publish`（同一 resource label）。治理中心未开时为透明 no-op。 | `false` → 所有调用路径裸跑，govern.* 规则永不生效。 |

`observability` 子 key（`level` off/brief/detailed、`maxArgBytes`、`skipOps`）是共享的
observe-kit 配置；语义见 go-redis 的 USAGE §3.3。`schema.json` 里 `metrics.enabled`
默认写的是 `false`，代码默认是 `true` —— 以代码为准。

---

## 4. 验证与故障演练

### 4.1 启动期 fail-fast

```bash
docker stop pulsar && go run .    # 启动中止："pulsar broker probe failed on pulsar://..."
docker start pulsar && go run .   # admin health 端点应答后即可启动（§1 闸门）
```

### 4.2 消息往返（含 binder 映射字段存活）

按 §1 发布 `Key="user-42"`、`Headers={"h1":"v1"}`；在 handler 里打印
`m.Key, m.Headers["h1"], string(m.Payload)` —— 三者全部存活，Headers 里还多出注入的
`traceparent`。可用 example 的冒烟路径验证（`example/check.sh`）。

### 4.3 受保护 vs 未保护（治理标签检查）

```yaml
# conf/govern.yaml
govern:
  enabled: true
  resilience:
    breaker:
      enabled: true
      min-calls: 4
      failure-rate: 50
```

资源标签是 `pulsar:pulsar://127.0.0.1:6650` [starter.go:76]。停掉 broker 后：压
`GuardedSend` → 过阈值后熔断打开，调用快速失败返回 resilience 哨兵错误，出现
observe-kit 访问日志记录 + resilience outcome 计数；改压 binder `Publish` → 每次调用
阻塞进 client 自身的重试/超时，没有哨兵、没有熔断。这个对比就是 §2.2 的边界。

### 4.4 指标 / span / 日志读取

- 原生：`curl -s :9091/metrics | grep pulsar_client_`（producer/consumer/连接统计；
  按实例独立 registry，实例间永不冲突 [command.go:59-74]）。
- OTel：observe-kit span `publish` / `consume`（trace 经 Properties 里的 W3C context
  串联）；手动助手发 `pulsar.produce` / `pulsar.consume <topic>`，带
  `messaging.system=pulsar` [command.go:99-134]。发流量后查 Jaeger（`:16686`）。
- 日志：binder handler/receive 错误与全部桥接的 client 内部日志落在 `_app_def` tag，
  前缀 `pulsar: `。

### 4.5 停机演练

SIGTERM → destroyClient 关闭 executor、client（全部 producer/consumer）与 metrics
server [client.go:44-58]。subscriber Close 先排空循环再 consumer.Close
[binder.go:157-168]。观察日志干净退出；:9091 停止服务。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 "pulsar broker probe failed" | broker 宕 / url 错 / 认证 / TLS | 修连通性；6650 开 ≠ 就绪，用 `:8080/admin/v2/brokers/health` 闸门。 |
| 启动失败 "pulsar driver not found" | `driver` 拼错或未注册 | 用 DefaultDriver，或 init 里 RegisterDriver。 |
| 第二个实例没有 /metrics | `metrics.port` 冲突；监听失败仅记 WARN [command.go:69-71] | 各配独立端口。 |
| 没有 trace | 未 import starter-otel | 加上；所有助手在无它时是静默 no-op。 |
| handler 明明成功了消息却重投 | Ack 失败（已记 WARN）[binder.go:158] | 检查 broker ack 权限；嫌疑见 §6。 |
| 消费者收不到消息 | 订阅名不对 / Shared 与 topic 语义 | `group` 与订阅 1:1；空 group 派生 `go-spring-<topic>` [binder.go:63-66]。 |
| 期望 token 认证，broker 拒绝 | mTLS cert+key 已设置 → token 被忽略（优先级）[driver.go:83-90] | 去掉 cert/key，或放宽 broker 的 mTLS。 |
| 压测下熔断从不打开 | 流量走 binder Publish 或 SendAsync —— 未保护（§2.2） | 改走 GuardedSend。 |
| handler panic 什么都不打死，但消息重现 | Recover 把 panic 转为 Nack | 预期行为；修 handler。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 18 个 value tag（+ observability 子 key） |
| 其中必填 | 1（`url`） |
| quickstart 前置外部依赖 | 1（Pulsar standalone） |
| "注意/坑"条数 | 8 |

设计嫌疑（审计台账）：`metrics.port` 固定默认 9091，多实例之间及与其他应用易冲突，且
监听失败被吞；binder 的 Publish 现已与裸路径共用同一 executor 受保护（Subscribe/消费
侧仍不受保护）；binder 路径的 span 仍走固定 "brief" observer 而非实例级
`observability` 配置（[command.go:152-159]）；消费侧 ack 失败记 WARN；`producer.Close()` 无
错误返回，publisher Close 不会失败；无运行期 health indicator（fail-fast
仅启动期 —— broker 后续宕机对 actuator 不可见）；`schema.json` 的
`metrics.enabled` 默认值与代码（true）不一致。
