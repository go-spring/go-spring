# starter-transaction-tcc

[English](README.md) | [中文](README_CN.md)

> 项目已正式发布，欢迎使用！

`starter-transaction-tcc` 将
[`go-spring.org/spring/transaction/tcc`](../../spring/transaction/tcc) 定义的
**TCC(Try / Confirm / Cancel)** 分布式事务能力接入 Go-Spring 应用。它是 Seata
TCC 的 Go 惯用法等价实现——不复刻 Seata 的 TC/TM/RM 角色，也不依赖字节码/代理魔法。

它属于 **Contributor**(贡献者)形态 starter(见 [DESIGN.md](../DESIGN.md) §2.3):
不监听端口、不启动服务，只注册 bean。

## TCC 还是 Saga?

Go-Spring 提供两种分布式事务模式，按一致性需求选择:

| | **Saga**(`starter-transaction-saga`) | **TCC**(`starter-transaction-tcc`) |
|---|---|---|
| 正向阶段 | 立即产生真实副作用 | **预留**资源(试探性，不以已提交状态对外可见) |
| 失败处理 | 补偿业务函数撤销副作用 | 第二阶段 **Cancel** 释放预留 |
| 隔离性 | 无——被撤销的值会短暂可见 | Try 与 Confirm 之间预留对业务不可见 |
| 适用场景 | 长的跨服务流程(MQ、HTTP、缓存) | 短的强一致流程(冻结余额 / 锁定库存) |

当资源需要在 Try 与 Commit 之间被"持有"时用 TCC;当每一步都产生真实副作用、靠补偿
撤销时用 Saga。

## 安装

```bash
go get go-spring.org/starter-transaction-tcc
```

## 快速开始

### 1. 导入 starter

```go
import _ "go-spring.org/starter-transaction-tcc"
```

容器中随即出现两个 bean:

- `tcc.Coordinator`——进程内编排器;
- `*tcc.ParticipantRegistry`——用于声明每个方法的参与者。

### 2. 定义参与者

每个参与者把工作拆成三个**幂等**阶段。`Try` 预留资源并返回令牌;`Confirm` 提交预留;
`Cancel` 释放预留。三者缺一不可。

```go
reserveStock := tcc.Participant{
    Name:    "ReserveStock",
    Try:     func(ctx context.Context) (any, error) { return stock.reserve(txID, qty) },
    Confirm: func(ctx context.Context, tried any) error { return stock.commit(txID) },
    Cancel:  func(ctx context.Context, tried any) error { return stock.release(txID) },
}
```

### 3. 执行事务

注入协调器并直接执行:

```go
type OrderService struct {
    Coord tcc.Coordinator `autowire:""`
}

func (s *OrderService) Place(ctx context.Context, txID string) (tcc.Result, error) {
    return s.Coord.Execute(ctx, tcc.Transaction{
        ID:           txID,
        Participants: []tcc.Participant{reserveStock, freezeBalance},
    })
}
```

协调器按顺序 Try 所有参与者。全部成功则全部 Confirm;任一 `Try` 失败则对已 Try 的
参与者逆序 Cancel。可运行的库存 + 余额示例(覆盖提交与回滚两条路径)见
[example/example.go](example/example.go)。

### 4. 或声明为 `@GlobalTransactional`

把参与者按方法名注册，并将 `GlobalTCC` 织入拦截链——这是 `@GlobalTransactional(TCC)`
的无反射等价形式:

```go
func RegisterOrder(reg *tcc.ParticipantRegistry) {
    reg.Register("OrderService.Place", reserveStock, freezeBalance)
}

chain := aspect.NewChain(tcc.GlobalTCC(coord, reg))
```

在入口处用 `tcc.WithTransactionID(ctx, id)` 设置事务 id，使其与业务幂等键对齐。

## 参与者义务

由于崩溃可能中断任意阶段、恢复会重放第二阶段，你的参与者必须自行处理 TCC 的三个经典
问题——本 starter 无法跨进程强制保证:

- **幂等**——`Confirm`/`Cancel` 可能被多次调用,第二次必须是 no-op。
- **空回滚**——`Cancel` 可能针对一个 `Try` 从未记录结果的参与者运行(此时值为 `nil`),
  这种情况下它必须什么都不做。
- **防悬挂**——延迟到达的 `Try`(在 `Cancel` 之后)不得重新预留。用事务 id 作为预留键
  即可检测。

## 配置

绑定于 `${spring.transaction.tcc}`(与 Saga 共享 `spring.transaction` 能力命名空间)。

| 键 | 默认值 | 说明 |
|---|---|---|
| `spring.transaction.tcc.enabled` | `true` | 开关 starter 的 bean。 |
| `spring.transaction.tcc.tracing` | `true` | 在 `starter-otel` 安装的全局上为每个参与者阶段发一个 otel 子 span;无 otel 时为 no-op。 |
| `spring.transaction.tcc.recover-on-start` | `true` | 启动时扫描 Store,把被中断的事务驱动到既定结局。 |

## 崩溃恢复

默认 TCC 日志**仅存内存**——足以覆盖常见的单进程场景与测试,但不具备崩溃恢复能力。当
`recover-on-start` 为 true 时,一个 `gs.Runner` 在启动时扫描 `tcc.Store`,对每个被中断
的事务:若已记录提交决策则向前 **Confirm**,否则向后 **Cancel**。恢复会依据持久化的
方法名从 `ParticipantRegistry` 重建参与者,因此你必须在装配期(bean 构造)注册参与者,
而不能在自定义 `Runner` 里注册。

要让恢复真正生效,请导入持久化 `Store` starter(`spring.transaction.tcc.store=...`);
由于内存默认实现以 `gs.OnMissingBean` 注册,持久化 Store 会随即接管协调器与启动恢复扫描。

## 许可证

Apache 2.0，见 [LICENSE](../../LICENSE)。
