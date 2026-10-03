# starter-mqtt 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`）与可运行的
[example/](example/) 核实 —— 文中括号为 file:line 抽查点。**MQTT 协议语义（QoS、
retained 消息、will、会话状态）以 [MQTT 官方文档](https://docs.oasis-open.org/mqtt/mqtt/v3.1.1/os/mqtt-v3.1.1-os.html)
为准，客户端 API 以 [paho.mqtt.golang 官方](https://github.com/eclipse/paho.mqtt.golang)
为准** —— 本文只写 go-spring 的增量。如实说明：本 starter 没有 `example-otel`，
下文可观测性声明经源码核实，但未做端到端冒烟验证。

**激活条件**：出现任意 `spring.mqtt.instances.*` 配置 —— 模块注册为
`gs.Module(gs.OnProperty("spring.mqtt"))`，即前缀匹配 [starter.go:34]。每个
`spring.mqtt.instances.<name>` 条目经 `conf.BindEach` 创建一个名为 `<name>` 的 `mqtt.Client`
bean [starter.go:35-39]。没有 health indicator bean（与 redis/nant 不同 —— 见 §6）。

---

## 1. 完整工程示例

一个既发布传感器读数、又通过 broker 中立的 `messaging.Driver` 消费的服务，组合
探针、指标与运行期治理。文件树：

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
    github.com/eclipse/paho.mqtt.golang v1.5.0
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-mqtt       latest
    go-spring.org/starter-actuator   latest   // 可选：探针 + /metrics
    go-spring.org/starter-otel       latest   // 可选：真实 trace/metric 导出
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "demo/service"

    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-mqtt"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**service.go** —— 受保护发布走裸 client，envelope 路径走 driver：

```go
package service

import (
    "context"
    "fmt"

    mqtt "github.com/eclipse/paho.mqtt.golang"
    "go-spring.org/cloud/messaging"
    "go-spring.org/spring/gs"
    StarterMQTT "go-spring.org/starter-mqtt"
)

type Service struct {
    // starter 提供的是裸 paho client，bean 名即配置条目名。
    Client mqtt.Client `autowire:"a"`
}

func init() {
    // driver 不会自动装配：自行适配裸 client 并导出 bean。
    // destination/source 字符串即 MQTT topic。
    gs.Provide(StarterMQTT.NewDriver, gs.TagArg("a")).Export(gs.As[messaging.Driver]())

    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) error {
            // (a) 受保护发布 —— 声明 publish operation 并经挂在 client "a" 上的
            //     resilience executor（限流 / 熔断）执行；由 executor 发射 span、
            //     指标与访问日志。
            err := StarterMQTT.GuardedPublish(ctx, s.Client, "sensors/temp", 1, false, []byte("21.5"))
            _ = err

            // (b) driver 消费 —— envelope API，仅载荷映射。
            b := StarterMQTT.NewDriver(s.Client)
            sub, _ := b.NewSubscriber(ctx, "sensors/temp", "")
            return sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
                fmt.Println("received:", string(m.Payload))
                return nil
            })
        }
    })
}
```

**conf/app.properties** —— 上述代码实际用到的完整注释配置面：

```properties
# --- mqtt client "a" --------------------------------------------------------
# broker URL 的 scheme 决定传输：tcp://（MQTT）或 ssl://（MQTTS）。
spring.mqtt.instances.a.broker=tcp://127.0.0.1:1883
spring.mqtt.instances.a.client-id=demo-publisher
# spring.mqtt.instances.a.username= / password=          # broker 开了认证才需要
# spring.mqtt.instances.a.clean-session=true             # 默认
# spring.mqtt.instances.a.keep-alive=30s / connect-timeout=10s

# Last Will：客户端非正常掉线时 broker 代发的消息。
spring.mqtt.instances.a.will.topic=demo/status
spring.mqtt.instances.a.will.payload=offline
spring.mqtt.instances.a.will.qos=1

# MQTTS：broker 换 ssl://host:8883 并启用 tls 组：
# spring.mqtt.instances.a.tls.enabled=true
# spring.mqtt.instances.a.tls.ca-file=/etc/mqtt/ca.pem
# spring.mqtt.instances.a.tls.cert-file=/etc/mqtt/client-cert.pem
# spring.mqtt.instances.a.tls.key-file=/etc/mqtt/client-key.pem

# --- actuator + otel --------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # /metrics 只走 actuator
```

**验证**（broker 按 [example 的 docker-compose.yml](example/docker-compose.yml) 启动，
即在 127.0.0.1:1883 起一个免认证 mosquitto）：

```bash
docker compose -f example/docker-compose.yml up -d   # 或：docker run -d -p 1883:1883 eclipse-mosquitto:2 mosquitto -c /mosquitto-no-auth.conf
go run .                        # broker 不可达则启动直接失败
# 预期应用日志："mqtt client initialized, broker=tcp://127.0.0.1:1883"
# 随后 driver 订阅者输出 "received: 21.5"
curl -s :9370/metrics | grep -E 'messaging_client'   # resilience 层发射的指标
grep _app_mqtt_access app.log | tail -2              # 访问记录
cd example && ./check.sh                             # 完整冒烟（自断言）
mosquitto_sub -t 'demo/status' &                     # kill -9 应用 → will "offline" 到达
```

example 自带的冒烟（`example/check.sh`）在 QoS 1 上做一次 pub/sub 往返，任何失败都会
非零退出 [example/check.sh:40-48]。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-mqtt
  └─ gs.Module(gs.OnProperty("spring.mqtt")) 在出现任意 spring.mqtt.instances.* key 时触发
        └─ conf.BindEach("${spring.mqtt}") → 每个 <name> 条目一份 Config
              └─ Provide(newClient).Name(<name>).Destroy(destroyClient).Caller(1)   [starter.go:36-40]

gs.Run()
  ├─ 构造 newClient [starter.go:75]：
  │    1. Driver bean 注入 —— 有公司 Driver bean 则用它；
  │       无（nil）则回退到内置 DefaultDriver                  [starter.go:78-81]
  │       （由 ${spring.mqtt.instances.<name>.driver} 按实例选择：留空 = 按类型注入，配置 = 按
  │       bean 名注入——指定的 bean 不存在则启动失败）
  │    2. CreateClient：组装 paho options（broker、id、凭证、
  │       clean-session、keep-alive、connect-timeout），把
  │       connect/lost/reconnecting 事件桥接进 go-spring 日志    [driver.go:64-72]，
  │       构建 TLS（security BuildClient）并注册 will                  [driver.go:74-85]
  │    3. CreateClient 在构建 client 的同时装上治理 executor ——
  │       params.ExecutorFor("mqtt", "mqtt:<broker>")，按 client 索引
  │       [driver.go:115-123] —— 因此 client 返回时即已完整
  │    4. 再探测：client.Connect() + token.Wait() —— broker 挂了、
  │       凭证错误或 TLS 不匹配都会中止启动；探测失败释放刚装配的内容 [starter.go:99-110]
  ├─ 就绪：没有 indicator bean —— paho 的自动重连（保持开启）是
  │  恢复路径；连接状态可经 IsConnected() 与桥接的生命周期日志观察
  │  （掉线打 Warn）                                             [driver.go:67-69]
  └─ SIGTERM → destroyClient [starter.go:111-117]：
       closeResilience（Close executor，错误被丢弃）             [command.go:102-107]
       → client.Disconnect(250ms 宽限期收尾在途消息)
```

注意没有独立的 `Init` 阶段：连接与 resilience 都在构造函数内完成，因此注入成功的
bean 一定是"已连接且已挂保护"的。

### 2.2 guard/wrap 机制 —— 精确顺序与未被保护的部分

paho.mqtt.golang 没有 reject-capable 中间件，所以不存在透明 client 包装。这个 seam 是
构造期挂载的 executor 加**调用点自愿接入的 guard** [command.go:17-29]：

```
CreateClient [driver.go:115-123]（装配 seam）：
  exec = params.ExecutorFor("mqtt", "mqtt:<broker>")   // 注入的 cloud.ClientParams{Resilience: mgr, Fault: inj}
  attachGuard(cl, exec, label) —— 以 mqtt.Client 值为键存入 sync.Map  [command.go:98-100]
  （mgr 存在时 params.ExecutorFor = fault.WrapClientExecutor(mgr.ClientExecutorFor("mqtt", label), label, inj)，
   否则为仅观测的 resilience.Unmanaged）

GuardedPublish [command.go:134-141]：
  ctx = observability.WithOperation(ctx, operation(opPublish, topic))  // 声明
  guard() → executor.Execute(ctx, call)                               [command.go:114-121]
  call = cl.Publish(...) + token.Wait() + token.Error()
```

包裹顺序（外→内）：**fault 注入 → resilience observe（从 ctx 读取声明的 operation，
发射 span、调用级与尝试级 duration 直方图、访问日志）→ resilience 策略
（限流/熔断/重试）→ paho Publish → token 等待**。设计理由（源码注释）：paho 自管队列
与重连，因此 executor 有意保持极小 —— 只限发布速率、在 broker 不健康时短路。服务标签
是 `mqtt:<broker-url>` —— 按 broker 而非按 topic [driver.go:121]。

**未被保护的部分**（已核实）：

- 裸 `client.Publish`（直接调 client bean）—— 完全绕过 resilience；`GuardedPublish`
  与 driver 的 `Publish` 都走 executor
  [command.go:134-141]。
- 裸 `Subscribe` / `Unsubscribe` —— 订阅建立没有 guard。投递只有经回调里的
  `GuardedConsume` 才受保护与观测；裸回调既无观测也无保护 [command.go:160-162]。
- driver 订阅 handler：只有 panic 保护（`messaging.Recover` 把 panic 转成 error）
  [client.go:98]；该 error 之后仅记日志 [client.go:107-111]。除 panic 外，每次投递都经
  `GuardedConsume` 声明并受保护 [client.go:107-109]。

manager 是必需的 —— 这个 starter 传递链接 cloud/governance，"关治理"是
`spring.governance.enabled=false`，而不是 bean 缺失；独立（非 gs）调用者传 nil，
client 的 executor 退化为仅观测的 `resilience.Unmanaged`（并打一次告警），而不是静默的
no-op [starter.go:87-88, driver.go:115-123]。

### 2.3 一次发布与一次消费的逐层走读

**受保护发布** `GuardedPublish(ctx, cl, "sensors/temp", 1, false, payload)`：

1. `observability.WithOperation(ctx, operation(opPublish, topic))` 声明 publish 身份
   （`messaging.system`、`messaging.operation`，topic 走 Detail）[command.go:135,
   observe.go:78-99]。
2. `guard` 从 sync.Map 解析该 client 的 executor [command.go:114-121]。
3. fault 注入检查（spring.governance.client.fault.* 策略，启用时）。
4. resilience observe 打开名为 `publish` 的调用 span，并在调用返回后发射调用级
   `messaging.client.operation.duration`、尝试级 `messaging.client.attempt.duration`、
   `resilience.client.calls` 计数器与唯一一条访问日志 —— 全部从声明读取。publish 与 consume
   声明为 `NonIdempotent`，故针对该 label 的重试策略会被**抑制**（每个 service 告警一次）：
   重新发布或再跑一遍 handler 是第二个副作用，不是第二次尝试。
5. resilience 策略：服务 `mqtt:<broker>` 上的限流器 / 熔断器；被拒时返回 sentinel
   错误且 **paho Publish 根本不会执行**。
6. `cl.Publish(topic, qos, retained, payload)` 交给 paho 出站队列；`token.Wait()`
   阻塞到包写出（QoS 0）或 PUBACK/PUBCOMP 到达（QoS 1/2）[command.go:137-139]。

**受保护消费** `GuardedConsume(ctx, cl, msg, handler)` 是订阅的对应入口：
它声明 consume 身份（topic 取自 `msg.Topic()`）并把 handler 放进同一个 executor，
因此一次投递与一次发布以完全相同的方式被观测 [command.go:160-162]。下方的
messaging.Driver 路径用它作为消费回调的入口。

**driver 消费** —— 在 topic `sensors/temp` 上 `sub.Subscribe(ctx, handler)`：

1. handler 被 `messaging.Recover` 包裹（panic → error）[client.go:98]。
2. `cl.Subscribe(topic, 1, callback)` + `token.Wait()` —— 错误（非法 topic 过滤器、
   无 broker）同步返回 [client.go:99-114]。
3. 每次投递，callback 从 paho 消息构造 `messaging.Message`——**只有 Payload**；
   topic 在 envelope 之外（它是订阅者的固定 source），QoS/retained 未建模，
   Key/Headers/Timestamp 在线路上不存在 —— MQTT 3.1.1 包没有逐消息元数据
   [client.go:52-57]。
4. callback 随后把消息交给 `GuardedConsume`，由它声明 consume 身份
   （topic 取自 `msg.Topic()`）并把 handler 放进该 client 的 resilience executor
   [client.go:101-111]。
5. handler 出错 → 按级别 Error 记日志并附 topic；没有 ack/nack、没有重投
   （回调 fire-and-forget）[client.go:107-111]。
6. `sub.Close()` → `Unsubscribe(topic)` + 等待；token 错误会返回（这条路径上唯一
   不被丢弃的错误）[client.go:117-121]。

driver 固定 QoS 1（`defaultQoS`）、发布 `retained=false`；retained 消息、自定义 QoS、
通配订阅需要裸 `mqtt.Client` bean [client.go:33-37]。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.mqtt.instances.<name>.` 下（经 `conf.BindEach` 按实例前缀绑定）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `broker` | string | — | **必填**（`expr:"$ != ''"`），如 `tcp://host:1883`，MQTTS 用 `ssl://`。同时构成 resilience 服务标签 `mqtt:<broker>`。 | 缺失 → 报出实例名的绑定错误。 |
| `client-id` | string | "" | 呈给 broker 的标识；空则由库生成。⚠ MQTT broker 拒绝两个同 client-id 的活跃连接 —— 多副本必须各配各的 id。 | 重复 id → 运行期反复被踢，启动期不报错。 |
| `username` / `password` | string | "" | broker 认证。 | 错误 → 启动期 fail-fast 连接失败 [starter.go:66-69]。 |
| `clean-session` | bool | true | 断开时 broker 丢弃会话状态。`false` + 固定 client-id 可获得离线消息补投语义。 | 见 [MQTT 规范 §3.1](https://docs.oasis-open.org/mqtt/mqtt/v3.1.1/os/mqtt-v3.1.1-os.html)。 |
| `keep-alive` | duration | 30s | PING 间隔。 | 过长 → 掉线感知慢。 |
| `connect-timeout` | duration | 10s | 约束启动期 Connect；`0` 关闭超时。 | 0 + 黑洞地址 → 启动卡死在 token 等待。 |
| `will.topic` | string | "" | **仅非空时**注册 will [driver.go:92-94]。 | 只配 payload/qos/retained 不配 topic → 全部静默失效（死 key）。 |
| `will.payload` | string | "" | will 消息体。 | — |
| `will.qos` | byte | 0 | will 的 QoS（0/1/2）。 | — |
| `will.retained` | bool | false | broker 是否保留 will。 | — |
| `tls.enabled` | bool | false | 启用 `security` 客户端 TLS；须搭配 `ssl://` 的 broker URL。 | 明文 broker + 开 TLS → 启动期连接失败。 |
| `tls.ca-file` / `cert-file` / `key-file` | string | — | CA / mTLS 客户端材料，建 client 时 `tls.Build()` [driver.go:83-90]。 | 配一半 → 启动期 Build 报错。 |
| `tls.server-name` / `insecure-skip-verify` | string/bool | — | SNI 覆写 / 跳过校验。 | — |

已与 `grep -rhoE 'value:"[^"]+"'` 全仓扫描比对：13 个不同 value tag → 7 个平铺 key +
tls（6）+ will（4）= **17 个 key，必填 1 个**。

---

## 4. 验证与故障演练

### 4.1 启动 fail-fast（broker 宕机）

```bash
docker stop <mosquitto> || true
go run .    # 以 "mqtt: connect failed broker=..." 退出 [starter.go:67]
docker start <mosquitto> && go run .   # 正常启动，日志 "mqtt client initialized" [starter.go:75]
```

### 4.2 受保护 vs 未受保护路径

配好治理规则源后，为服务 `mqtt:tcp://127.0.0.1:1883` 加限流/熔断策略：

```yaml
spring:
  governance:
    enabled: true
    resilience:
      mqtt:tcp://127.0.0.1:1883:
        rateLimiter: { limit: 1, period: 1s }
```

压测 `GuardedPublish` → 拒绝以 resilience sentinel 错误与 `_app_mqtt_access` 访问记录
浮出。同等流量直接在 client bean 上裸调
`client.Publish` 则完全不受影响 —— driver 已走同一 guard，退出口是服务级
（给 `mqtt:<broker>` 配一条全零 rule，或整体关治理），不再是调用点
[command.go:134-162, client.go]。
策略免重启热切换（治理中心）。

### 4.3 消息往返（含 driver 映射字段存活）

经 driver publisher 发一个设置了 Key/Headers/Timestamp 的 envelope，用 driver
subscriber 消费：**只有 Payload 存活**；Key/Headers/Timestamp 到达时为零值 —— MQTT
3.1.1 没有元数据字段 [client.go:52-57, client.go:100]。往返断言 payload 字节相等；
超出载荷的需求请用裸 client（例如自行把元数据编码进 payload）。

### 4.4 可观测读取

starter 只**声明**；由 resilience 层**发射**。以下信号全部由 resilience executor 依据
声明的 operation 产出 —— 不是 starter 产出的。

- 访问日志：tag `_app_mqtt_access`（observe.go 注册 `app.mqtt.access`，作为
  `Operation.LogTag` 随声明携带）。每次受保护调用一条记录：`messaging.system=mqtt`、
  `messaging.operation=publish|consume`、`messaging.destination.name=<topic>`、
  `status=<ok|error>`、保护拒绝时的 `resilience.outcome=<rate_limited|...>`、`duration_ms=...`；失败打
  Warn 并带 `error` 字段，带 topic（Detail）的成功打 Debug，无 topic 的成功打 Info。
- 指标，全部在 `messaging.client.*` 下：调用级 `operation.duration` 直方图、尝试级
  `attempt.duration` 直方图、`active_requests` 在途 gauge（标签 `messaging.system=mqtt`、
  `messaging.operation`、`status`），外加 resilience 层自有的 `resilience.client.calls`
  计数器。topic 永不作为指标标签 —— 它是 Detail，只进 span 与日志。
- 连接状态计数器：`messaging.client.connection.state_changes`
  （`messaging.system=mqtt`、`state`），由 paho 的 connect/lost/reconnecting 回调驱动；
  它留在 starter 内，**不由** resilience 层发射。

```bash
curl -s :9370/metrics | grep -E 'messaging_client_(operation_duration|attempt_duration|active_requests|connection_state_changes)'
```

- span：publish 与 consume 各自打开名为 `publish` / `consume` 的 span。⚠ 两侧是
  **彼此独立的 trace** —— W3C trace context 无法随 MQTT 3.1.1 消息传播
  [command.go:31-38]。没有 starter-otel 的 OTel 全局时，信号全部是无声 no-op。
- 生命周期日志（tag `_app_def`）：connected / reconnecting（Info）、connection lost
  （Warn）[driver.go:72-86]。

### 4.5 will / 优雅停机

```bash
mosquitto_sub -t 'demo/status' &
go run . &      # ... 随后 SIGTERM
# 优雅退出：Disconnect(250) —— 不触发 will（正常断开）
kill -9 <pid>   # 非正常退出 → broker 代发 will "offline"（按配置 retained）
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| 启动报 "mqtt: connect failed broker=..." | broker 不可达 / 凭证错误 / TLS 不匹配 | fail-fast 连接是无条件的 [starter.go:64-69]；修连通性或配置。 |
| 启动卡死（无报错） | `connect-timeout=0` 且地址被黑洞 | 保持有限超时；0 表示关闭超时 [config.go:51]。 |
| 反复重连 / 客户端被踢 | 多副本重复 `client-id` | 各配不同 id（broker 强制唯一）。 |
| 熔断/限流不生效 | 直接裸调 `client.Publish` | `GuardedPublish` 与 driver 的 `Publish` 都受保护 [command.go:134-162]；换调用点。 |
| 无 trace/metric/访问记录 | 未 import starter-otel，或期望 driver 产出 | resilience 层依据声明的 operation 发射，并依赖 OTel 全局；driver 什么都不产 [client.go:47-50]。 |
| broker 重启后订阅者沉默 | 非干净会话丢失订阅 | paho 自动重连在，但重订阅行为取决于 clean-session / broker 会话；用生命周期日志核实 [driver.go:76-81]。 |
| handler 错误石沉大海 | driver 只记日志不重投 | 在 handler 内部自行重试 [client.go:107-111]。 |
| TLS key 似乎没生效 | broker URL 仍是 `tcp://` | `ssl://` 与 `tls.enabled` 搭配使用 [config.go:53-55]。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 17（平铺 7 + tls 6 + will 4） |
| 其中必填 | 1（`broker`） |
| quickstart 前置外部依赖 | 1（MQTT broker —— docker compose 起 mosquitto） |
| "注意/坑"条数 | 6 |

设计嫌疑（审计台账；上轮条目均未修复）：

- driver handler 出错仅记日志；`Recover` 注释现在正是这样写的（早先那句 MQTT 3.1.1
  fire-and-forget 回调给不了的 "nack/redelivery" 已修正）[client.go:95-98]。
- 治理按调用点 opt-in（`GuardedPublish`）且 README 未记载。
- README 配置表漏掉 `driver`。
- 无 health indicator bean（家族不对称：redis/nats 都有）；`IsConnected()` 是唯一活性
  信号且无人自动探测。
- `schema.json` 把 `will.qos` 标成 object 类型，且漏掉 tls/will 子 key
  [schema.json:49-53] —— schema 落后于 config.go。
