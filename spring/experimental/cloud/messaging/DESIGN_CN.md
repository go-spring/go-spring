# messaging 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`messaging` 是 stdlib 层零依赖抽象,给 Go-Spring 提供了 Spring Cloud Stream
等价的 broker 独立性:一个信封 + 一对接口收敛发布订阅,broker starter 在下面
接入具体实现。已有实现 `Binder` 的 broker starter:`starter-nats`、
`starter-kafka`、`starter-kafka-sarama`、`starter-pulsar`、
`starter-rabbitmq`、`starter-mqtt`。

## 1. 职责与边界

- broker 中立编程模型:`Message`、`Publisher`、`Subscriber`、`Binder`。接口
  层不泄漏任何 broker 专有语义。
- 刻意**不做**函数式 Supplier/Function/Consumer 泛型层。过度抽象会让 broker
  starter 保留的原生 client 逃生舱(JetStream、admin、事务...)变得笨重。
- 不是 tracing 库,不是重试 / DLQ 库。trace 上下文骑在 `Headers` 上;
  重试 / requeue / nack 语义因 broker 而异,starter 各自记录。
- 不是 schema registry。`Payload` 是 opaque `[]byte`;编解码在上层。

## 2. 关键抽象与缝隙

- `Message` — `{Key, Payload, Headers, Timestamp}`。`Header` / `SetHeader`
  nil-safe,因为 `Headers` 兼作 W3C `TextMapCarrier`。
- `Handler = func(ctx, *Message) error`。非 nil 返回值代表投递失败;如何呈现
  (nack、重投、log)由 broker starter 决定。
- `Publisher.Publish` / `.Close`,`Subscriber.Subscribe` / `.Close`。二者构
  造时即绑定 destination / source——Publisher 只写一处,Subscriber 只读一
  处。
- `Binder` 在同一 broker 连接上打开 `Publisher` / `Subscriber`。
  destination / source / group 字符串**按 broker 各自语义**解释(subject、
  topic、queue、consumer-group、subscription-name)。
- `RegisterBinder` / `GetBinder` / `MustGetBinder`——driver-registry 对齐缝隙
  (与 `discovery.Register`、`resilience.RegisterDriver` 同构,空名/nil/重复
  一律 panic)。真实 broker binder 通常构造函数绑活连接注入 bean;注册表留给
  想按名字选进程级 binder 的调用方。

## 3. 约束(禁止破坏)

- **`Headers` 读 nil-safe,写时按需分配**。生产者可以留零值;binder 通过
  `SetHeader` 注入 trace 上下文。
- **绑定实例**。binder 用构造函数(`NewBinder(conn)`)接线,同一 broker 连
  接支撑其上打开的 publisher / subscriber;别引回"占有 client 的全局默认
  binder"。
- **本包零第三方 import**。broker SDK 只出现在实现 `Binder` 的 starter 里。
- **`group == ""` 语义**:broker 支持时是广播(NATS fanout、JetStream 临
  时);而 broker 天然把 queue 视为竞争消费组(RabbitMQ)时忽略该参数——每个
  starter 各自记录准确解释。

## 4. 权衡 / 未做的方案

- **不做函数式 Supplier/Function/Consumer 语法糖层**。会把用户困在过度抽象
  API 里,并复杂化逃生舱。仍保留原生 client bean。
- **`Subscribe` 建立好即返回,不阻塞投递**。长期投递循环属于 binder 实现。
- **MQTT(3.1.1)刻意不使用 `Key`/`Headers`/`Timestamp`**。线上无每消息元数
  据;该 starter 记录 payload-only 且不做 trace 传播。
- **Kafka(franz-go)约束**:topics/group 在 client 构造时固定,故一个 client
  bean = 一个逻辑 consumer;需要多消费组的场景由 starter 起多个 client(或改
  用 sarama 变种)。
