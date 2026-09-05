# starter-influxdb 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

一个 Client 原型 starter（`starter/DESIGN.md` §2.2），面向 InfluxDB 2.x。
结构与 starter-s3 同族（HTTP 传输型客户端，Init 经 `dynamicTransport` 武装）；
InfluxDB 特有的决策是写路径与异步错误排出。

## 1. 职责与边界

- **负责**：`spring.influxdb.<name>` 组的 bean 生命周期、fail-fast
  `/health` 探针与每实例指示器、observe 传输层、韧性保护的阻塞写、托管
  异步写入器（创建 + 错误排出 + 停机 flush）。
- **不负责**：Flux 查询构建（原样透传）、bucket/org 管理、降采样任务、
  v1 兼容模式（自定义 driver 的事）。

## 2. 关键抽象与 Seam

- **`Client` 包装 bean** — 内嵌 `influxdb2.Client`。SDK 在构造期固定
  `*http.Client`
  （`Options.SetHTTPClient`），因此 DefaultDriver 安装 `dynamicTransport`
  间接层，Init 把 observe+韧性 round-tripper 换进去 —— 与
  starter-s3/starter-elasticsearch 同一机制。
- **`WritePoints` vs `ManagedWriteAPI`** — SDK 的两种写形态保留为两个显式
  方法而非一个策略开关：阻塞写是逐次的，应归治理执行器管；缓冲写在后台
  goroutine 批量重试（重试是 SDK 自己的），逐点守卫会重复计数。
  `ManagedWriteAPI` 的 `Errors()` 通道排入 go-spring 日志，因为没人排空
  通道时写入器会在第一次失败时卡死。
- **`HealthError`** — `/health` 状态映射由 fail-fast 探针与指示器共享，
  在 `health` 子包导出一份。

## 3. 约束

- 写助手需要 `org`/`bucket`，但连接不需要 —— 没配它们的客户端照样服务
  Query/Delete API；助手以指明性错误失败，而不是在装配期报错。
- 异步路径按设计不做守卫（见 §2）；其失败以日志行浮出，不是调用方错误。
- 未初始化的 OSS 服务器 `/health` 应答不同 —— 探针只接受 `pass`，因此
  引导顺序竞争会在启动期暴露，而不是表现为间歇性写失败。

## 4. 权衡 / 已否决的备选

- **两个显式写方法 vs. 一个可配置写入器**：带 `mode` 配置的单个
  `WriteAPI` 会掩盖失败语义的根本差异（逐次错误 vs. 后台失败记日志）；
  显式方法让契约在调用点可见。
- **错误排入日志 vs. 暴露通道**：包装器暴露 `Errors()` 会诱发经典的
  死锁（没人排空）；starter 默认排空，需要自定义处理时直接用内嵌客户端
  的 `WriteAPI`。
- **查询侧韧性**：`QueryRaw` 暂不守卫（阻塞写才是过载敏感路径）；日后
  加 `GuardedQuery` 是增量变更，不破坏兼容。
