# event 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`event` 是 stdlib 基础层的零依赖进程内发布/订阅事件总线。它用 Go 惯用法
(泛型 + reflect 类型键)补齐 Spring `ApplicationEventPublisher` /
`@EventListener` 的缺口,而不是复刻注解扫描或基于反射的处理器签名机制。

## 1. 职责与边界

- 仅进程内。跨副本广播是另一个议题(基于消息队列的 "config bus"),不在本包
  职责内。
- 按**精确动态类型**路由——发布一个具体值只送达订阅该类型的处理器。接口订阅
  故意不匹配实现该接口的具体值:路由预期可控且调用点无反射,Go 惯用法本就是
  一事件一具体 struct。
- 无 marker 接口,无注解扫描,无基于反射的处理器签名。全包唯一的反射是
  `reflect.TypeFor[T]()` / `reflect.TypeOf` 作为 map 键。

## 2. 关键抽象与缝隙

- `Bus` 接口只有 `Publish(ctx, any) error` 与 `Close() error`。具体 `*bus`
  实现一个未导出的 `subscribable` 接口,泛型自由函数 `Subscribe[T]` /
  `SubscribeAsync[T]` 在内部断言到它,保调用点类型安全、零反射。
- `Listener` 是容器侧的非泛型收集缝隙:bean 通过 Export 为 `event.Listener`
  收集(与 `health.Indicator` 的 Export 收集范式同构),`Register(bus)` 内部
  再调用泛型 `Subscribe[T]`。
- `SubOption`(`WithOrder` / `WithBuffer` / `WithErrorHandler`)修改归一化的
  `subOptions`;选项作用于每个订阅,而不是整个 bus。

## 3. 约束(禁止破坏)

- **同步错误不短路**:每个同步处理器都会运行,错误用 `errors.Join` 聚合(与
  aspect 拦截链的透传精神一致);单个失败订阅者绝不能静默吞掉其他错误。
- **顺序**:`WithOrder` 越小越先(index 0 最外层,与 aspect 链一致);相同
  order 按注册 seq 稳定排序。
- **异步 worker 通道**:发送用 `select { ch<- / done / ctx.Done() }` 三路;
  `done` 一旦 close,send 永不阻塞,取消订阅时既不会 send-on-closed panic 也
  不会泄漏 publisher。
- **优雅 drain**:`Close` 先关每个 worker 的 `done`,再把已缓冲的事件排空后
  返回——已接收的事件不会被静默丢弃。
- **Close 后是硬错误而非空透传**:`Close` 之后 `Publish` 返回 `ErrClosed`。
  无订阅者时 Publish 是良性 no-op;闭桥后再 publish 是误用,显式报错。
- **边界 nil 透传**:`Publish(nil)` 为 no-op;nil bus 或 nil handler 上
  `Subscribe` 返回 no-op cancel。

## 4. 权衡 / 未做的方案

- **不做接口路由**:订阅 `io.Reader` 不会收到具体 `*bytes.Buffer` 的发布。若
  要支持,发布时得遍历方法集或维护第二级索引,拖累现有廉价的类型键路由,而 Go
  惯用法本身无迫切场景。
- **Close 不隐式等待同步 handler**:同步处理器在 publisher 协程内跑完,
  Publish 返回时已结束,无所谓等待。Close 仅等异步 worker。
- **无全局 bus**:`New()` 按应用创建。进程级单例会让测试易碎、模糊生命周期
  语义;容器装配时显式传 bus。
