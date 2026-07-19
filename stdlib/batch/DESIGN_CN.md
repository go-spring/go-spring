# batch Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`batch` 是 stdlib 层零依赖的批处理 / 短任务抽象。它让 Go 拿到 Spring Batch +
Spring Cloud Task 的**效果**——可重启的分块处理与一次性任务记录——但不复刻
XML/注解 DSL。持久化后端(Redis / 数据库)由 starter 单独贡献
`JobRepository` bean。

## 1. 职责与边界

- 把批量工作建模为 `Reader` -> `Processor` -> `Writer`,组合进 `ChunkStep`;
  `Job` 拥有 step 顺序与 instance 身份。
- 通过 `JobRepository` 持久化进度,崩溃 + 重启从最近提交的 chunk 恢复,不必
  从零重跑。
- 用 `Func(name, fn)` 把短任务收进同一形状——单步 job,结果落进同一个
  repository。
- 拒绝做调度器、跨进程执行器、消息消费者。触发方式(cron / HTTP / MQ)是调
  用方的事;跨进程 partition v1 有意不做。

## 2. 关键抽象与缝隙

- **`JobRepository` 接口 = 后端缝隙。** 无全局 driver 注册表。后端需要活对
  象(Redis conn / `*sql.DB`),不是声明式策略,所以缝隙 = bean 类型——与
  `stdlib/lock` 同款。`NewMemoryRepository()` 内建给测试与单进程用;持久化
  starter 贡献真实 bean。
- **Instance 身份 = `(Name, Params)`。** `ObtainExecution` 对 name + 排序后的
  params 做 sha1 得到 `instanceKey`:同 params 重跑就是 resume,改一个 param
  就是新 instance。
- **Reader 可选实现 `Checkpointer`。** 支持恢复的 reader 实现它;引擎在
  `Open` 处把上次提交的 `Checkpoint` 交回,并在每次 chunk 提交后取新值。不实
  现就从头开始重放。
- **读在 retry 之外,处理+写在 retry 之内。** 每个 chunk 读一次进缓冲;
  `resilience.Executor`(由 `ChunkStep.Retry` 构造,或直接注入)包裹缓冲后
  的处理+写。reader 无法把已推进的 item 吐回来,把读也放进 retry 会破坏状态。
- **`Step` 接口抹掉泛型**,让 `Job` 能在同一个 slice 里放不同 item 类型的
  step。

## 3. 约束

- **提交是持久边界。** 进度在 chunk 写成功后才提交。写完到提交之间崩溃,重
  启会重放这一 chunk——writer 必须幂等(带 key 的 upsert / 去重键)才有
  exactly-once。框架只保证 at-least-once。
- **repository 为 nil 直接失败。** `Job.Run` 遇 nil 返 `ErrNoRepository`,不
  静默降级;`ChunkStep` 无 `Reader` / 无 `Writer` 同理(`ErrNoReader` /
  `ErrNoWriter`)。
- **context 取消 = 干净停止,不是失败。** `ctx.Err() != nil` 时 step 记为
  `StatusStopped` 而非 `StatusFailed`,主动关机能重启且监控不会误报。
- **repository 实现必须并发安全**,并且必须返回足够深的拷贝,防止调用方通
  过返回指针改到内部状态(见 `cloneJob` / `cloneStep`)。

## 4. 取舍与被否决方案

- **不做 driver 字符串注册表。** batch 不是像 `discovery` / `resilience` 那
  样的配置期选择:没有活客户端就没法表达"要哪个后端"。bean 类型缝隙比注册
  表间接更合适。
- **不引入 XML / 注解 DSL。** job 用 Go 泛型直接拼——Spring Batch 的价值在
  重启语义,不在 DSL。
- **v1 不做跨进程 partition。** 分片会改 checkpoint 模型(per-partition +
  完成协调),把分布式协调塞进 stdlib。除非有具体场景,暂不做。
- **Reader 重放 > 读进 retry。** 重试无法回滚的读副作用(offset 前推、游标
  移动)会天然错;把 chunk 缓冲进 buffer,retry 语义就简单又安全。
