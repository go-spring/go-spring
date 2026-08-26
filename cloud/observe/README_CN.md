# observe
[English](README.md) | [中文](README_CN.md)

`observe` 是面向客户端操作(cache、database、messaging、resilience 保护调用)
的统一 trace + metric + log 观测件。每个 client starter 把自己的插桩缝隙 ——
driver hook、连接包装、gorm callback —— 接到一个 `Observer` 上,于是所有
client 以同一套词汇发出同样的三信号,并共用一个详略开关。

## 新 starter 接线三步

1. **embed 配置** 到 starter 的 per-instance Config:

   ```go
   type Config struct {
       ...
       Observability observe.ObserveConfig `value:"${observability:=}"`
   }
   ```

2. **构造 Observer**,带上该 client 的 system 标签:

   ```go
   obs := observe.NewDB("redis", c.Observability)      // 数据库 / cache
   obs := observe.NewProducer("kafka", c.Observability) // messaging 发布端
   obs := observe.NewConsumer("kafka", c.Observability) // messaging 消费端
   obs := observe.New(system, mySemConv, kind, cfg)     // 自定义约定
   ```

   若 client 库已自带 trace+metric(go-redis 的 redisotel、elasticsearch 的
   transport 插桩),传 `observe.WithoutTraceAndMetric()`(或
   `WithoutTrace()`),让本件只补 access log,不重复出 span。

3. **在 seam 处 Start/End。** 操作开始处开 span,结束时带着操作的 error
   恰好 End 一次:

   ```go
   func (c *Client) Get(ctx context.Context, key string) (string, error) {
       ctx, sp := c.obs.Start(ctx, "GET", key)
       v, err := c.inner.Get(ctx, key)
       sp.End(err)
       return v, err
   }
   ```

   参数晚于开 span 才知道时(gorm callback 在 After 阶段才拿到 SQL),在
   `End` 前调 `sp.SetArg(sql)`。

## SemConv 词汇表

一个 `SemConv` 把 metric 名前缀与属性 key 打包成一体,永远成对出现:

| SemConv | 指标 | 属性 | span kind |
|---|---|---|---|
| `DBSemConv` | `db.client.operation.duration`、`db.client.active_requests` | `db.system`、`db.operation`、`db.statement` | client |
| `MessagingSemConv` | `messaging.client.operation.duration`、`messaging.client.active_requests` | `messaging.system`、`messaging.operation`、`messaging.destination.name` | producer / consumer |
| `ResilienceSemConv` | `resilience.operation.duration`、`resilience.active_requests` | `resilience.system`、`resilience.resource`、`resilience.arg` | internal |

所有 metric 另带 `status` 维度(`ok`/`error`);错误细节在 span 和 access
log 上。自定义 SemConv 时从 `DBSemConv` / `MessagingSemConv` 起步。

## access log 的 level 约定

`observability.level` 只控制 access log(trace 和 metric 走全局 OTel
管线,不做 per-instance 配置):

- `off` —— 不出 access log;trace、metric 照常。
- `brief` —— 每操作一条:operation、status、duration、error。这是默认值;
  未配置(`""`)同样按 `brief` 处理。
- `detailed` —— brief 再加操作参数(命令 key、SQL、topic……),由
  `maxArgBytes` 截断(默认 512)。

## SkipOps 匹配规则

`observability.skipOps` 列出的操作名(即传给 `Observer.Start` 的名字,如
`PING`)会把三信号 —— span、metric、access log —— 一起压制,避免高频健康
探测刷爆后端。匹配是精确字符串相等。

## 与 starter-otel 的关系

本件挂在 OTel 全局上:引入 starter-otel 时它安装真实的 TracerProvider /
MeterProvider;不引入时全局是 no-op,trace 和 metric 几乎零开销、不改变任
何行为。access log 始终走项目 `log` 包,与 starter-otel 无关。

## 共享桥接

- `observe/lock` —— `WrapLocker(system, cfg, inner)` 包装任意
  `lock.Locker` 出三信号(`lock.*` 指标;TryAcquire 未命中记
  `lock.acquired=false`)。
- `observe/resilience` —— `WrapExecutor(inner, system, cfg)` 包装任意
  `resilience.Executor`:三信号外加按 outcome 分类的 `resilience.calls`
  计数和 `resilience.breaker.state_change` 事件。
- `observe/transaction` —— `SagaObserver` / `TccObserver` / `AtObserver`
  为每个事务阶段开一个子 span。

## 安装

```
go get go-spring.org/cloud
```
