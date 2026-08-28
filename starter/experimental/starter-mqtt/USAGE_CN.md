# starter-mqtt 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`）与可运行的
[example/](example/) 核实 —— 文中括号为 file:line 抽查点。**MQTT 协议语义（QoS、
retained 消息、will、会话状态）以 [MQTT 官方文档](https://docs.oasis-open.org/mqtt/mqtt/v3.1.1/os/mqtt-v3.1.1-os.html)
为准，客户端 API 以 [paho.mqtt.golang 官方](https://github.com/eclipse/paho.mqtt.golang)
为准** —— 本文只写 go-spring 的增量。如实说明：本 starter 没有 `example-otel`，
下文可观测性声明经源码核实，但未做端到端冒烟验证。

**激活条件**：出现任意 `spring.mqtt.*` 配置 —— 模块注册为
`gs.Module(gs.OnProperty("spring.mqtt"))`，即前缀匹配 [starter.go:34]。每个
`spring.mqtt.<name>` 条目经 `conf.BindEach` 创建一个名为 `<name>` 的 `mqtt.Client`
bean [starter.go:35-39]。没有 health indicator bean（与 redis/nant 不同 —— 见 §6）。

---

## 1. 完整工程示例

一个既发布传感器读数、又通过 broker 中立的 `messaging.Binder` 消费的服务，组合
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
    go-spring.org/starter-governance latest   // 可选：运行期 resilience/fault 策略
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "demo/service"

    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-mqtt"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**service.go** —— 受保护发布走裸 client，envelope 路径走 binder：

```go
package service

import (
    "context"
    "fmt"

    mqtt "github.com/eclipse/paho.mqtt.golang"
    "go-spring.org/cloud/experimental/messaging"
    "go-spring.org/spring/gs"
    StarterMQTT "go-spring.org/starter-mqtt"
)

type Service struct {
    // starter 提供的是裸 paho client，bean 名即配置条目名。
    Client mqtt.Client `autowire:"a"`
}

func init() {
    // binder 不会自动装配：自行适配裸 client 并导出 bean。
    // destination/source 字符串即 MQTT topic。
    gs.Provide(StarterMQTT.NewBinder, gs.TagArg("a")).Export(gs.As[messaging.Binder]())

    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) error {
            // (a) 受保护发布 —— 经过挂在 client "a" 上的 resilience executor
            //     （限流 / 熔断）。
            ctx, sp := StarterMQTT.StartPublishSpan(ctx, "sensors/temp")
            err := StarterMQTT.GuardedPublish(ctx, s.Client, "sensors/temp", 1, false, []byte("21.5"))
            StarterMQTT.EndSpan(sp, err)

            // (b) binder 消费 —— envelope API，仅载荷映射。
            b := StarterMQTT.NewBinder(s.Client)
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
spring.mqtt.a.broker=tcp://127.0.0.1:1883
spring.mqtt.a.client-id=demo-publisher
# spring.mqtt.a.username= / password=          # broker 开了认证才需要
# spring.mqtt.a.clean-session=true             # 默认
# spring.mqtt.a.keep-alive=30s / connect-timeout=10s

# Last Will：客户端非正常掉线时 broker 代发的消息。
spring.mqtt.a.will.topic=demo/status
spring.mqtt.a.will.payload=offline
spring.mqtt.a.will.qos=1

# 作用于【受保护调用】与 span 助手的访问日志（助手取第一个被配置 client 的
# observability 条目，先到先得）。
spring.mqtt.a.observability.level=detailed

# MQTTS：broker 换 ssl://host:8883 并启用 tls 组：
# spring.mqtt.a.tls.enabled=true
# spring.mqtt.a.tls.ca-file=/etc/mqtt/ca.pem
# spring.mqtt.a.tls.cert-file=/etc/mqtt/client-cert.pem
# spring.mqtt.a.tls.key-file=/etc/mqtt/client-key.pem

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
# 随后 binder 订阅者输出 "received: 21.5"
curl -s :9370/metrics | grep -E 'messaging_client'   # observe kit 指标
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
  └─ gs.Module(gs.OnProperty("spring.mqtt")) 在出现任意 spring.mqtt.* key 时触发
        └─ conf.BindEach("${spring.mqtt}") → 每个 <name> 条目一份 Config
              └─ Provide(newClient).Name(<name>).Destroy(destroyClient).Caller(1)   [starter.go:36-39]

gs.Run()
  ├─ 构造 newClient [starter.go:50]：
  │    1. driver 查找 —— 未知名字直接启动失败                    [starter.go:53-57]
  │    2. driver.CreateClient：组装 paho options（broker、id、凭证、
  │       clean-session、keep-alive、connect-timeout），把
  │       connect/lost/reconnecting 事件桥接进 go-spring 日志    [driver.go:73-81]，
  │       构建 TLS（tlsconf Build）并注册 will                  [driver.go:83-94]
  │    3. client.Connect() + token.Wait() —— fail-fast 探测：broker 挂了、
  │       凭证错误或 TLS 不匹配都会中止启动                      [starter.go:64-69]
  │    4. applyResilience —— 挂上治理 executor，按 client 索引；
  │       失败则断开 client（250ms）                             [starter.go:70-74]
  ├─ 就绪：没有 indicator bean —— paho 的自动重连（保持开启）是
  │  恢复路径；连接状态可经 IsConnected() 与桥接的生命周期日志观察
  │  （掉线打 Warn）                                             [driver.go:76-78]
  └─ SIGTERM → destroyClient [starter.go:82-86]：
       closeResilience（Close executor，错误被丢弃）             [command.go:137-142]
       → client.Disconnect(250ms 宽限期收尾在途消息)
```

注意没有独立的 `Init` 阶段：连接与 resilience 都在构造函数内完成，因此注入成功的
bean 一定是"已连接且已挂保护"的。

### 2.2 guard/wrap 机制 —— 精确顺序与未被保护的部分

paho.mqtt.golang 没有钩子/插件扩展点，所以不存在透明 client 包装。这个 seam 是
构造期挂载的 executor 加**调用点自愿接入的 guard** [command.go:17-27]：

```
applyResilience [command.go:128-134]：
  exec = fault.WrapExecutor(resilience.ExecutorFor("mqtt:<broker>"))   // 治理中心
  exec = resilobserve.WrapExecutor(exec, "mqtt", c.Observability)      // observe 桥
  以 mqtt.Client 值为键存入 sync.Map

GuardedPublish [command.go:166-172]：
  guard() → executor.Execute(ctx, "mqtt:<broker>", call)              [command.go:147-154]
  call = cl.Publish(...) + token.Wait() + token.Error()
```

包裹顺序（外→内）：**fault 注入 → resilience 策略（限流/熔断/重试）→ observe
（受保护调用的 span+metric+访问日志）→ paho Publish → token 等待**。设计理由（源码
注释）：paho 自管队列与重连，因此 executor 有意保持极小 —— 只限发布速率、在 broker
不健康时短路 [command.go:117-122]。资源标签是 `mqtt:<broker-url>` —— 按 broker 而非
按 topic [starter.go:70, resilience/config.go:151-155]。

**未被保护的部分**（已核实）：

- 裸 `client.Publish`（直接调 client bean）—— 完全绕过 resilience；`GuardedPublish`
  与 binder 的 `Publish` 都走 executor
  [command.go:156-172]。
- `Subscribe` / `Unsubscribe` / 订阅回调 —— 没有对应的 guard。
- binder 订阅 handler：只有 panic 保护（`messaging.SafeHandler` 把 panic 转成 error）
  [client.go:92]；该 error 之后仅记日志 [client.go:95-97]。

治理关闭时 `ExecutorFor` 返回透明的 no-op executor，`GuardedPublish` 的行为与裸
publish + wait 完全一致 [command.go:123-127, 156-160]。

### 2.3 一次发布与一次消费的逐层走读

**受保护发布** `GuardedPublish(ctx, cl, "sensors/temp", 1, false, payload)`，外层套
span 助手：

1. `StartPublishSpan(ctx, topic)` 打开名为 `publish` 的 producer 观测，带
   `messaging.destination.name = topic` [command.go:84-86, observer.go:213-232]。
2. `guard` 从 sync.Map 解析该 client 的 executor [command.go:148-153]。
3. fault 注入检查（govern.fault.* 策略，启用时）。
4. resilience 策略：资源 `mqtt:<broker>` 上的限流器 / 熔断器；被拒时返回 sentinel
   错误且 **paho Publish 根本不会执行** [command.go:160-163]。
5. observe 桥记录受保护调用的结果（span/metric/访问日志由 `observability.*` 驱动）。
6. `cl.Publish(topic, qos, retained, payload)` 交给 paho 出站队列；`token.Wait()`
   阻塞到包写出（QoS 0）或 PUBACK/PUBCOMP 到达（QoS 1/2）[command.go:164-171]。
7. `EndSpan(sp, err)` 记录结果并结束观测 [command.go:100-103]。

**binder 消费** —— 在 topic `sensors/temp` 上 `sub.Subscribe(ctx, handler)`：

1. handler 被 `messaging.SafeHandler` 包裹（panic → error）[client.go:92]。
2. `cl.Subscribe(topic, 1, callback)` + `token.Wait()` —— 错误（非法 topic 过滤器、
   无 broker）同步返回 [client.go:93-100]。
3. 每次投递，callback 从 paho 消息构造 `messaging.Message`：**只有 Payload**。
   topic 在 envelope 之外（它是订阅者的固定 source），QoS/retained 未建模，
   Key/Headers/Timestamp 在线路上不存在 —— MQTT 3.1.1 包没有逐消息元数据
   [client.go:47-52]。
4. handler 出错 → 按级别 Error 记日志并附 topic；没有 ack/nack、没有重投
   （回调 fire-and-forget）[client.go:95-97]。
5. `sub.Close()` → `Unsubscribe(topic)` + 等待；token 错误会返回（这条路径上唯一
   不被丢弃的错误）[client.go:103-107]。

binder 固定 QoS 1（`defaultQoS`）、发布 `retained=false`；retained 消息、自定义 QoS、
通配订阅需要裸 `mqtt.Client` bean [client.go:33-45]。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.mqtt.<name>.` 下（经 `conf.BindEach` 按实例前缀绑定）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `broker` | string | — | **必填**（`expr:"$ != ''"`），如 `tcp://host:1883`，MQTTS 用 `ssl://`。同时构成 resilience 资源标签 `mqtt:<broker>`。 | 缺失 → 报出实例名的绑定错误。 |
| `client-id` | string | "" | 呈给 broker 的标识；空则由库生成。⚠ MQTT broker 拒绝两个同 client-id 的活跃连接 —— 多副本必须各配各的 id。 | 重复 id → 运行期反复被踢，启动期不报错。 |
| `username` / `password` | string | "" | broker 认证。 | 错误 → 启动期 fail-fast 连接失败 [starter.go:66-69]。 |
| `clean-session` | bool | true | 断开时 broker 丢弃会话状态。`false` + 固定 client-id 可获得离线消息补投语义。 | 见 [MQTT 规范 §3.1](https://docs.oasis-open.org/mqtt/mqtt/v3.1.1/os/mqtt-v3.1.1-os.html)。 |
| `keep-alive` | duration | 30s | PING 间隔。 | 过长 → 掉线感知慢。 |
| `connect-timeout` | duration | 10s | 约束启动期 Connect；`0` 关闭超时。 | 0 + 黑洞地址 → 启动卡死在 token 等待。 |
| `will.topic` | string | "" | **仅非空时**注册 will [driver.go:92-94]。 | 只配 payload/qos/retained 不配 topic → 全部静默失效（死 key）。 |
| `will.payload` | string | "" | will 消息体。 | — |
| `will.qos` | byte | 0 | will 的 QoS（0/1/2）。 | — |
| `will.retained` | bool | false | broker 是否保留 will。 | — |
| `tls.enabled` | bool | false | 启用 `tlsconf` 客户端 TLS；须搭配 `ssl://` 的 broker URL。 | 明文 broker + 开 TLS → 启动期连接失败。 |
| `tls.ca-file` / `cert-file` / `key-file` | string | — | CA / mTLS 客户端材料，建 client 时 `tls.Build()` [driver.go:83-90]。 | 配一半 → 启动期 Build 报错。 |
| `tls.server-name` / `insecure-skip-verify` | string/bool | — | SNI 覆写 / 跳过校验。 | — |
| `observability.level` | string | brief | **受保护调用**与 span 助手的访问日志（`off`/`brief`/`detailed`）；助手由**第一个**被配置 client 的条目播种（先到先得，多 client 不同级别以后者不生效）[command.go:61-100]。 | 多 client 需要不同级别 → 助手只有一份，取第一个 client 的。 |
| `observability.maxArgBytes` | int | 512 | detailed 模式下捕获参数的字节上限。 | 过小 → topic 被截断。 |
| `observability.skipOps` | list | — | 对受保护调用 observer 按操作名同时压制 span+metric+log。 | — |
| `driver` | string | DefaultDriver | 选择已注册的 Driver；注册重名会 panic [driver.go:46-51]。 | 未知名字 → 启动报错 "mqtt driver not found" [starter.go:56]。 |
| `governance` | bool | true | 为实例挂 resilience/fault executor；同时保护 `GuardedPublish` 与 binder 的 `Publish`（同一 resource label）。治理中心未开时为透明 no-op。 | `false` → 所有调用路径裸跑，govern.* 规则永不生效。 |

已与 `grep -rhoE 'value:"[^"]+"'` 全仓扫描比对：15 个不同 value tag → 8 个平铺 key +
tls（6）+ will（4）+ observability（3）= **21 个 key，必填 1 个**。

---

## 4. 验证与故障演练

### 4.1 启动 fail-fast（broker 宕机）

```bash
docker stop <mosquitto> || true
go run .    # 以 "mqtt: connect failed broker=..." 退出 [starter.go:67]
docker start <mosquitto> && go run .   # 正常启动，日志 "mqtt client initialized" [starter.go:75]
```

### 4.2 受保护 vs 未受保护路径

配置 starter-governance 后，为资源 `mqtt:tcp://127.0.0.1:1883` 加限流/熔断策略：

```yaml
govern:
  enabled: true
  resilience:
    mqtt:tcp://127.0.0.1:1883:
      rateLimiter: { limit: 1, period: 1s }
```

压测 `GuardedPublish` → 拒绝以 resilience sentinel 错误与 `_app_mqtt_access` 访问记录
浮出（受 `observability.level` 门控）。同等流量直接在 client bean 上裸调
`client.Publish` 则完全不受影响 —— binder 已走同一 guard，退出口是实例级
`governance=false`，不再是调用点 [command.go:147-172, client.go]。
策略免重启热切换（治理中心）。

### 4.3 消息往返（含 binder 映射字段存活）

经 binder publisher 发一个设置了 Key/Headers/Timestamp 的 envelope，用 binder
subscriber 消费：**只有 Payload 存活**；Key/Headers/Timestamp 到达时为零值 —— MQTT
3.1.1 没有元数据字段 [client.go:47-52, client.go:94]。往返断言 payload 字节相等；
超出载荷的需求请用裸 client（例如自行把元数据编码进 payload）。

### 4.4 可观测读取

- 访问日志：tag `_app_mqtt_access`（observe kit 注册 `app.<system>.access`）
  [observer.go:195]。每次受保护调用 / span 助手观测一条记录：
  `system=mqtt op=publish|consume status=ok|error duration=...`（detailed 模式附 topic）。
- 指标：`messaging.client.operation.duration`（秒）与 `messaging.client.active_requests`，
  属性 `messaging.system=mqtt`、`messaging.operation`，span 上另有
  `messaging.destination.name`（topic）[observer.go:184-193, observer.go:216-220]。

```bash
curl -s :9370/metrics | grep -E 'messaging_client_operation_duration|messaging_client_active_requests'
```

- span：producer span 名为 `publish`，consumer span 名为 `consume`。⚠ 两侧是
  **彼此独立的 trace** —— W3C trace context 无法随 MQTT 3.1.1 消息传播
  [command.go:44-47]。没有 starter-otel 的 OTel 全局时，三个信号全部是无声 no-op。
- 生命周期日志（tag `_app_def`）：connected / reconnecting（Info）、connection lost
  （Warn）[driver.go:73-81]。

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
| 启动报 "mqtt driver not found" | `driver` 指向未注册项 | 在 init 里 `RegisterDriver`（重名 panic）[driver.go:46-51]。 |
| 反复重连 / 客户端被踢 | 多副本重复 `client-id` | 各配不同 id（broker 强制唯一）。 |
| 熔断/限流不生效 | 直接裸调 `client.Publish`，或实例 `governance=false` | `GuardedPublish` 与 binder 的 `Publish` 都受保护 [command.go:156-172]；换调用点/重新开启。 |
| 无 trace/metric/访问记录 | 未 import starter-otel，或期望 binder 产出 | 助手依赖 OTel 全局；binder 什么都不产 [client.go:47-52]。 |
| `observability.level=off` 但日志仍在 | 多 client 场景：播种的是**第一个**配置 client 的级别，改的是后配置的 client | 把希望生效的级别放到第一个 client 上（或所有 client 一致）。 |
| broker 重启后订阅者沉默 | 非干净会话丢失订阅 | paho 自动重连在，但重订阅行为取决于 clean-session / broker 会话；用生命周期日志核实 [driver.go:76-81]。 |
| handler 错误石沉大海 | binder 只记日志不重投 | 在 handler 内部自行重试 [client.go:95-97]。 |
| TLS key 似乎没生效 | broker URL 仍是 `tcp://` | `ssl://` 与 `tls.enabled` 搭配使用 [config.go:53-55]。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 21（平铺 8 + tls 6 + will 4 + observability 3） |
| 其中必填 | 1（`broker`） |
| quickstart 前置外部依赖 | 1（MQTT broker —— docker compose 起 mosquitto） |
| "注意/坑"条数 | 6 |

设计嫌疑（审计台账；上轮条目均未修复）：

- binder handler 出错仅记日志；`SafeHandler` 注释宣称的 "nack/redelivery" 是 MQTT 3.1.1
  fire-and-forget 回调给不了的 [client.go:90-97]。
- ~~span 助手的 observer 硬编码 `DefaultBrief`，无视
  `spring.mqtt.<name>.observability.level`~~ 已修：第一个被装配的 client 用自己的
  observability 配置为助手 observer 播种（先到先得，见 command.go 的
  seedObserveConfig）——助手遵循配置级别；多个 client 级别不同时只有第一个生效
  （包级助手、单一 observer）。
- 治理按调用点 opt-in（`GuardedPublish`）且 README 未记载。
- README 配置表漏掉 `observability.*` 与 `driver`。
- 无 health indicator bean（家族不对称：redis/nats 都有）；`IsConnected()` 是唯一活性
  信号且无人自动探测。
- `schema.json` 把 `will.qos` 标成 object 类型，且漏掉 tls/will/observability 子 key
  [schema.json:49-53] —— schema 落后于 config.go。
