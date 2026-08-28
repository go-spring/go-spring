# starter-rabbitmq 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均已对照 starter 源码
（`config.go`、`starter.go`、`client.go`、`command.go`、`driver.go`）与可运行的
[example/](example/) / [example-otel/](example-otel/) 核对。**AMQP 0-9-1 语义
（exchange、routing key、队列持久化、publisher confirms、心跳）见
[RabbitMQ 官方文档](https://www.rabbitmq.com/docs)** —— 本文只写 go-spring 的增量。

**激活条件**：模块经 `gs.OnProperty("spring.rabbitmq")` 注册 [starter.go:34] ——
前缀匹配，出现任意 `spring.rabbitmq.*` 配置即激活。仅多实例：每个
`spring.rabbitmq.<name>` 块产出一个命名的 `*amqp.Connection` bean；无
`__default__` 单例。

---

## 1. 完整工程示例

一个包含 publisher 与一对竞争消费者的服务（经 messaging binder），并保留裸连接
作为 binder 未建模 AMQP 特性的逃生口。文件树：

```
demo/
├── go.mod
├── main.go
├── messaging_app.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/rabbitmq/amqp091-go  v1.10.0
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-rabbitmq   latest
    go-spring.org/starter-actuator   latest   // 可选：探针 + /metrics
    go-spring.org/starter-otel       latest   // 可选：真实 trace/metric 导出
    go-spring.org/starter-governance latest   // 可选：GuardedPublish 的运行期治理
)
```

**main.go**：

```go
package main

import (
    _ "demo/messaging_app" // 应用接线在此

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-rabbitmq"
)

func main() { gs.Run() }
```

**messaging_app.go** —— 经 binder 的 publisher + consumer，加一次受治理的裸发布：

```go
package messaging_app

import (
    "context"

    amqp "github.com/rabbitmq/amqp091-go"
    "go-spring.org/cloud/experimental/messaging"
    "go-spring.org/log"
    "go-spring.org/spring/gs"

    StarterRabbitMQ "go-spring.org/starter-rabbitmq"
)

type App struct {
    Conn *amqp.Connection `autowire:"demo"`
}

func init() {
    // 导出为 Rooter，使容器在 prod 也实例化它（gs 只装配根可达 bean）。
    gs.Provide(&App{}).Export(gs.As[gs.Rooter]())

    // binder 把连接适配为 broker 中立的 messaging.Binder；
    // 应用代码此后只依赖 messaging.Publisher / messaging.Subscriber。
    gs.Provide(StarterRabbitMQ.NewBinder, gs.TagArg("demo"))
}

// Init 在注入完成后运行（gs.Rooter）：启动竞争消费者。
func (a *App) Init(ctx context.Context) error {
    // binder 提供的 publisher：trace context 与 observe kit 自动生效。
    b := StarterRabbitMQ.NewBinder(a.Conn)
    pub, err := b.NewPublisher(ctx, "orders.created")
    if err != nil { return err }
    if err := pub.Publish(ctx, &messaging.Message{
        Key: "order-42", Payload: []byte(`{"id":42}`),
        Headers: map[string]string{"origin": "demo"},
    }); err != nil { return err }

    // 竞争消费者：同一队列上的两个 subscriber 分摊投递。
    for i := 0; i < 2; i++ {
        sub, err := b.NewSubscriber(ctx, "orders.created", "")
        if err != nil { return err }
        if err := sub.Subscribe(ctx, handle); err != nil { return err }
    }

    // 裸路径逃生口：exchange 路由 + 治理守卫。只有 GuardedPublish
    // 经过 resilience executor（见 §2.2）。
    ch, err := a.Conn.Channel()
    if err != nil { return err }
    defer ch.Close()
    if err := ch.ExchangeDeclare("logs", "direct", false, true, false, false, nil); err != nil {
        return err
    }
    pub2 := amqp.Publishing{Body: []byte("routed"), ContentType: "text/plain"}
    pctx, span := StarterRabbitMQ.StartPublishSpan(ctx, "logs", "info", &pub2)
    err = StarterRabbitMQ.GuardedPublish(pctx, a.Conn, ch, "logs", "info", false, false, pub2)
    StarterRabbitMQ.EndSpan(span, err)
    return err
}

func handle(ctx context.Context, msg *messaging.Message) error {
    log.Infof(ctx, log.TagAppDef, "got key=%s headers=%v body=%s", msg.Key, msg.Headers, msg.Payload)
    return nil // 返回 error → Nack(requeue)，见 §2.3
}
```

**conf/app.properties** —— 完整注释配置面：

```properties
# --- rabbitmq（第二个实例即 spring.rabbitmq.<b>.* 等）------------------------
spring.rabbitmq.demo.url=amqp://guest:guest@127.0.0.1:5672/
spring.rabbitmq.demo.vhost=
spring.rabbitmq.demo.heartbeat=10s
spring.rabbitmq.demo.driver=DefaultDriver

# TLS（默认关；amqps:// URL 也会隐式启用）：
#spring.rabbitmq.demo.tls.enabled=true
#spring.rabbitmq.demo.tls.ca-file=/etc/ssl/rabbit-ca.pem
#spring.rabbitmq.demo.tls.cert-file=/etc/ssl/rabbit-client.pem
#spring.rabbitmq.demo.tls.key-file=/etc/ssl/rabbit-client.key
#spring.rabbitmq.demo.tls.server-name=rabbit.example.com
#spring.rabbitmq.demo.tls.insecure-skip-verify=false

# 受治理（GuardedPublish）调用的访问日志：
#spring.rabbitmq.demo.observability.level=brief
#spring.rabbitmq.demo.observability.maxArgBytes=512
#spring.rabbitmq.demo.observability.skipOps=

# --- actuator + otel（与 example-otel/conf/app.properties 同 key）------------
spring.actuator.addr=:9370
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0
```

**broker 启动**（与示例 docker-compose.yml 同镜像）：

```bash
docker run -d --name demo-rabbit -p 127.0.0.1:5672:5672 -p 127.0.0.1:15672:15672 rabbitmq:3-management
# 或：cd starter-rabbitmq/example && docker compose up -d
```

**验证**（与 example/example.go 的 `-manual` 模式同构，其在 :9090 提供 /publish、
/consume）：

```bash
cd starter-rabbitmq/example && ./check.sh        # 完整冒烟：默认 exchange、direct
                                                  # exchange、QoS+手动 ack
# 或交互式：
docker compose up -d && sleep 10
go run . -manual
curl http://127.0.0.1:9090/publish    # OK
curl http://127.0.0.1:9090/consume    # value
```

---

## 2. 装配与时序

### 2.1 bean 生命周期时间线

```
import starter-rabbitmq
  └─ gs.Module(gs.OnProperty("spring.rabbitmq")) 注册 group               [starter.go:34]
        │
gs.Run()
  ├─ 绑定：conf.BindEach 遍历 spring.rabbitmq.* → 每实例一个 Config，
  │   Provide newClient 并 .Name(<instance>).Destroy(destroyClient)        [starter.go:35-39]
  ├─ newClient（每实例）：
  │   ├─ driver 注册表查找；未命中 → 启动失败                             [starter.go:58-62]
  │   ├─ Driver.CreateClient：TLS 构建 + amqp.Dial/DialConfig —— TCP +
  │   │   AMQP 握手是同步的：错误 URL / 错误凭据 / TLS 不匹配都在启动期
  │   │   失败，而非首次 publish 时                                       [starter.go:46-49]
  │   ├─ 打开并关闭一条探测 channel，确认 AMQP 层可用                      [starter.go:69-76]
  │   ├─ NotifyClose/NotifyBlocked 桥接进 go-spring 日志（连接关闭时
  │   │   amqp091 关闭 channel，goroutine 自然退出）                        [starter.go:83-103]
  │   └─ applyResilience：executor 以 *amqp.Connection 为键索引             [starter.go:106]
  ├─ 就绪：连接 bean 可按实例名注入 *amqp.Connection
  └─ SIGTERM：destroyClient → closeResilience（executor Close）后
      conn.Close()，同时排空 notifier goroutine                            [starter.go:118-121]
```

### 2.2 治理守卫的包裹顺序

每连接的 executor 在 `applyResilience` 内由内向外构建 [command.go:234-240]：

```
fault.WrapExecutor( resilience.ExecutorFor(resource) )   ← 外层
        │
   resilobserve.WrapExecutor(exec, "rabbitmq", c.Observability)  ← 包在它外面
        │
   你的调用（ch.PublishWithContext）                       ← 最内层
```

即：受治理的 publish 先经过 fault 注入 / 限流 / 熔断裁决，只有存活下来的调用才被
观测（span + `resilience.*` 指标 + 访问日志）。executor 经中立的
`resilience.ExecutorFor` seam 获取 —— 治理关闭时它是透明 no-op，`guard` 甚至查不到
executor 而直接透传（command.go:253-260）。

**不在守卫内**：`GuardedPublish` 是唯一受守卫的入口 [command.go:271-275]。binder 的
裸 `ch.PublishWithContext`、整个消费路径、queue/exchange 声明与 ack 全部绕过
resilience。消费侧保护是 handler 自己的事。binder 的 `Publish` **已受保护**——走
`GuardedPublish` 与连接级 executor [client.go]；实例 key `governance=false` 可让所有
调用路径裸跑。

### 2.3 一次 publish 与一次 consume 逐层走读（binder 路径）

Publish [client.go:89-107]：

1. 信封 → `amqp.Publishing`：`Body` ← Payload、`Headers` ← 字符串头
   （toAMQPTable，空则 nil）、`MessageId` ← `msg.Key` [client.go:90-94]。
2. 若 `traffic.IsLoadTest(ctx)`，标记写入 AMQP header `x-loadtest`
   [client.go:97-102]，让消费侧识别压测流量。
3. `startPublish` 开启 observe kit 的生产者观测（span `publish <queue>`、指标
   `messaging.client.operation.duration`）并向 `pub.Headers` 注入 W3C trace
   context [command.go:194-201]。
4. `PublishWithContext` 发往默认 exchange（`""`），队列名即 routing key；
   `sp.End(err)` 记时长、平衡 in-flight 计数、出访问日志 [client.go:103-106]。

Consume [client.go:122-150]：

1. handler 先包 `messaging.SafeHandler` —— panic 转为正常 error 路径，不再
   冲散 SDK goroutine [client.go:125]。
2. `Consume(autoAck=false)`；后台循环 range delivery channel [client.go:126-133]。
3. 每条投递：`startConsume` 从 headers 提取上游 trace 并开启消费者观测
   [command.go:205-212]；load-test 标记从 AMQP headers 读回并写进 ctx
   [client.go:136-138]。
4. `fromDelivery` 反向映射：`Key` ← MessageId、仅字符串值的 headers、
   `Timestamp` ← 投递时间戳 [client.go:178-194]。
5. handler 出错 → Error 日志 + `Nack(multiple=false, requeue=true)`（broker 重投）；
   成功 → `Ack(false)`。ack/nack 失败会记 WARN（有重投风险）[client.go:141-146]。

### 2.4 受守卫的裸路径

`GuardedPublish(ctx, conn, ch, exchange, key, mandatory, immediate, pub)` 按
**连接**（而非 channel）解析 executor —— channel 可能在某些模式下比连接 bean 活得久，
而 executor 始终限定在 starter 创建的连接上（command.go:266-270 注释）。拒绝时底层
publish 根本不执行，返回的是 resilience 哨兵错误。手动追踪助手
（`StartPublishSpan` / `StartConsumeSpan` / `EndSpan`，command.go:72-129）是裸
channel 侧的对应物。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.rabbitmq.<name>.*`。自有 value tag 6 个（config.go:33-57）
加共享 tlsconf（6 个）与 observe（3 个）块共 15 个；必填 1 个。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `url` | string | — | **必填**（`expr:"$ != ''"`，config.go:33）。`amqp://` 或 `amqps://`；仅 `amqps://` scheme 也会强制走 TLS 配置拨号分支 [driver.go:69-72]。 | 为空 → 绑定报错；凭据/TLS 错 → **启动失败**（同步拨号）。 |
| `vhost` | string | "" | 传入 `amqp.Config`，覆盖从 URL 解析出的 vhost [driver.go:72-76]。同时进入治理 resource label。 | URL 与 vhost 冲突 → 启动期 AMQP 握手错误。 |
| `heartbeat` | duration | 10s | `>0`（或有 TLS/vhost）使 `amqp.Dial` → `amqp.DialConfig` 并携带 `Heartbeat` [driver.go:72-80]。`0` = URL/服务端默认。⚠ 非默认 heartbeat 会把普通拨号也切到 DialConfig 分支，值仍生效。 | 过低 → 高负载下误判断连；0 → 服务端默认可能超过 TCP 空闲超时。 |
| `tls.enabled` | bool | false | 与 `amqps://` 隐式等效；显式开启后把构建的 `*tls.Config` 接进拨号 [driver.go:69-79]。 | 明文 `amqp://` + `tls.enabled=true` → 对 cleartext 端口跑 TLS → 启动报错。 |
| `tls.ca-file` / `cert-file` / `key-file` | string | "" | 自定义 CA / mTLS 证书对，由共享 `tlsconf` 块加载（跨 starter 统一 key，config.go:44-48）。 | 文件缺失 → 启动期 TLS 构建失败 [driver.go:64-68]。 |
| `tls.server-name` | string | "" | SNI/校验名覆盖。 | 不匹配 → 启动期 x509 hostname 错误。 |
| `tls.insecure-skip-verify` | bool | false | 跳过证书校验。 | 生产置 true = 静默 MITM 暴露。 |
| `observability.level` | 枚举 | brief | `off`/`brief`/`detailed` 访问日志详细度 —— **仅作用于 GuardedPublish 路径**（resilience observer，command.go:236）。⚠ binder 路径的 observe kit 是包级默认、固定 `brief`，读不到这个 key（注释见 command.go:181-190）。 | 想借此关掉 binder 访问日志 → 无效。 |
| `observability.maxArgBytes` | int | 512 | detailed 模式下截取的操作参数上限。 | 过低 → 日志参数被截断。 |
| `observability.skipOps` | 列表 | "" | resilience observer 跳过的操作名（span+指标+日志全免）。 | — |
| `driver` | string | DefaultDriver | 从 `RegisterDriver` 填充的注册表选择 [driver.go:31-53]。 | 未知名字 → 启动报 "rabbitmq driver not found" [starter.go:58-62]；重名注册 panic [driver.go:50]。 |
| `governance` | bool | true | 为实例挂 resilience/fault executor；同时保护 `GuardedPublish` 与 binder 的 `Publish`（同一 resource label）。治理中心未开时为透明 no-op。 | `false` → 所有调用路径裸跑，govern.* 规则永不生效。 |

已与 `grep -rhoE 'value:"[^"]+"'` 对账：自有 tag 恰为 `${url}`、`${vhost:=}`、
`${heartbeat:=10s}`、`${tls}`、`${observability:=}`、`${driver:=DefaultDriver}`；
tls.*/observability.* 各列来自经 `${tls}`、`${observability:=}` 绑定的共享 cloud 块。

---

## 4. 验证与故障演练

### 4.1 broker 宕机 fail-fast

```bash
docker stop demo-rabbit && go run .   # example 下：broker 停掉跑 ./check.sh
# 启动即中断："failed to dial rabbitmq: ..."（driver.go:86）—— 不是首次使用才
# 懒失败；探测 channel 还能额外拦下 TCP 通但 AMQP 层坏掉的端点（starter.go:69-76）。
```

### 4.2 受守卫 vs 未守卫路径

引入 starter-governance，并对 rabbitmq 资源（label 为
`rabbitmq:<vhost>`(冒号格式;vhost 为空时回落 `rabbitmq:<url>`)，starter.go:106）配置 fault 规则：

1. `GuardedPublish` 调用 → resilience 哨兵错误，publish 根本不进 channel，
   产出 `resilience.*` 指标 + `_app_rabbitmq_access` 日志记录。
2. 同一规则下的 binder `Publish` → 同样的 resilience 哨兵错误（走连接级 executor，
   §2.2）。自行开 channel 裸调 `PublishWithContext` → 不受影响（没有 executor 跳跃）。
   实例级退出口：`governance=false`。

### 4.3 消息往返 / 映射字段存活

```bash
curl :9090/publish && curl :9090/consume        # body "value" 存活
```

binder 到 binder 往返：`Payload`、字符串 `Headers`、`Key`（经 MessageId）、消费侧
`Timestamp` 存活。**丢失项**：生产侧设置的 `msg.Timestamp` 不会写入
`amqp.Publishing.Timestamp` [client.go:90-94]；非字符串 header 值消费侧被丢弃
[client.go:183-186]；`ContentType`/`DeliveryMode`/优先级未建模 —— binder 消息在
非持久化队列上是瞬态的，broker 重启即丢（见
[durability](https://www.rabbitmq.com/docs/durability)）。

### 4.4 观测量读取

- 指标（binder 路径，observe kit）：`messaging.client.operation.duration` 与
  `messaging.client.active_requests`，属性 `messaging.system=rabbitmq`、
  `messaging.operation=publish|consume`、`messaging.destination.name=<queue>`，
  另有 `status=ok|error` 维度（cloud/observe/observer.go:66-71,187-213）。受守卫
  路径另有 `resilience.operation.duration` / `resilience.active_requests`。
- span：binder —— `publish <queue>`（producer kind）/ `consume <queue>`（consumer
  kind），经 W3C headers 跨 broker 串联；手动助手 —— `rabbitmq.publish <dest>` /
  `rabbitmq.consume <dest>`，带 `messaging.rabbitmq.destination.routing_key` 属性
  [command.go:79-87,109-117]。
- 访问日志：tag `_app_rabbitmq_access`（`log.RegisterAppTag("rabbitmq","access")`，
  cloud/observe/observer.go:213-214）。
- 用 example-otel 验证：`docker compose up -d`（rabbitmq + jaeger），`go run .` ——
  程序会经 Jaeger API :16686 自验 trace（example-otel/main.go:214-223）。

```bash
curl -s :9370/metrics | grep -E 'messaging_client|rabbitmq'
```

### 4.5 连接生命周期事件

应用运行中杀掉 broker：打出 Warn 日志 `rabbitmq connection closed: code=...
reason=... server=... recover=...` [starter.go:91-92]；内存告警限流时记
`connection blocked` / `unblocked` [starter.go:96-101]。**没有自动重连**，也没有
health indicator —— 进程会在死连接上继续跑。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败：`failed to dial rabbitmq` | broker 未起 / URL / 凭据错误 | 同步拨号 fail-fast —— 修环境或 URL（driver.go:86）。 |
| 启动失败：`rabbitmq driver not found` | `driver=` 拼写错或自定义 driver 未注册 | 在 gs.Run 前 `RegisterDriver`（starter.go:61）。 |
| panic：`rabbitmq driver already registered` | `RegisterDriver` 重名 | 改名（driver.go:50）。 |
| 启动失败：`failed to open probe channel` | TCP 通但 AMQP 层坏（如 vhost/权限错） | 检查该用户的 vhost 权限（starter.go:69-76）。 |
| 启动正常、之后 publish 报错；伴随 close/blocked Warn 日志 | broker 中途挂了；无自动重连 | 重启进程或在裸 bean 上自建重连；盯 `connection closed` Warn。 |
| 消费者收不到消息 | handler 出错 → Nack(requeue) 死循环；查 `rabbitmq binder handler error on %q` Error 日志 | 修 handler；任何 error 都会永久重投 —— 没有 DLQ（client.go:141-146）。 |
| broker 重启后消息消失 | binder 队列非持久化且消息瞬态 | 用裸连接做 durable 声明（client.go:64,76；rabbitmq.com/docs/durability）。 |
| 跨服务 `Key`/headers "丢失" | 对端生产者写了原生 AMQP 路由头 / 非字符串值消费侧被丢弃 | 信封 headers 仅字符串；Key 走 MessageId（client.go:183-194）。 |
| binder 无 trace/指标 | 未 import starter-otel | 加上；否则 OTel 全局是静默 no-op（command.go:57-59）。 |
| GuardedPublish 返回 resilience 哨兵错误 | 触发限流 / 熔断开启 / 注入 fault | 读 `resilience.*` 指标 + 访问日志；这是治理契约（command.go:264-266）。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 15（自有 6 + tls 6 + observability 3） |
| 其中必填 | 1（`url`） |
| quickstart 前置外部依赖 | 1（RabbitMQ） |
| "注意/坑"条数 | 6 |

设计嫌疑（供裁决台账；自上轮审计以来无已修复项）：

- binder 硬编码队列声明（非持久化、非排他）且无配置逃生口（client.go:64,76）——
  默认即 broker 重启丢数据。
- `Key` ↔ `MessageId` 是有损约定；routing key —— AMQP 原生键 —— 未被 binder 建模。
- `Ack/Nack` 失败记 WARN（client.go:143,145）—— 若 handler 成功仍被重投，先查日志
  （有重投风暴风险）。
- 生产侧 `msg.Timestamp` 未写入 `amqp.Publishing.Timestamp`（client.go:90-94）。
- 非字符串 AMQP header 值消费侧被静默丢弃（client.go:183-186）。
- binder 发布不受守卫而裸路径可以 —— starter 自身携带的两条路径治理面不对称
  （client.go:104 vs command.go:271）。
- binder observe kit 是包级默认（`brief`），实例级 `observability.*` key 只影响
  GuardedPublish（command.go:181-190）。
