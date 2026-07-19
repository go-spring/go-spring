# starter-transaction-saga

[English](README.md) | [中文](README_CN.md)

`starter-transaction-saga` 把
[`go-spring.org/stdlib/transaction`](../../stdlib/transaction) 中定义的 **Saga**
分布式事务能力接入 Go-Spring 应用。它以进程内 Coordinator + aspect 链的形式
提供 `@GlobalTransactional(SAGA)` 的 Go 惯用等价物,而不复刻 Seata 的
TC/TM/RM 角色,也不依赖字节码魔法。

它属于 **Contributor** 形态(见 [DESIGN.md](../DESIGN.md) §2.3):不开端口、
不起 server,只注册 bean。

## Saga vs. TCC——选哪个?

两种模式都提供,按一致性需求二选一:

| | **Saga**(`starter-transaction-saga`) | **TCC**(`starter-transaction-tcc`) |
|---|---|---|
| 前向步骤 | 立刻产生真实效果 | *预留*资源(未提交前对业务不可见) |
| 失败时 | 业务补偿函数撤销效果 | 第二阶段取消预留 |
| 隔离性 | 无——短暂可见的中间态 | Try 与 Confirm 之间对业务不可见 |
| 适用场景 | 长链路、跨服务(MQ、HTTP、缓存) | 短链、强一致(冻结余额 / 占用库存) |

## 安装

```bash
go get go-spring.org/starter-transaction-saga
```

## 快速开始

### 1. 导入 starter

```go
import _ "go-spring.org/starter-transaction-saga"
```

容器随即持有三个 bean:

- 一个 `transaction.Coordinator`——进程内编排器,按序执行 Step,失败时逆序
  补偿;
- 一个 `*transaction.StepRegistry`——应用侧按方法名声明 Step;
- 一个 `transaction.Store`——默认内存实现,可被持久化 Store 顶替(见下)。

### 2. 声明 Step

每个 Step 有前向 `Action` 与 `Compensate`,两者都必须**幂等**(崩溃可能重放
Action、恢复可能重放 Compensate)。前序 Step 的返回值通过 `StepResults` 供
后序访问。

```go
deductInventory := transaction.Step{
    Name:       "DeductInventory",
    Action:     func(ctx context.Context, r *transaction.StepResults) (any, error) { ... },
    Compensate: func(ctx context.Context, r *transaction.StepResults) error { ... },
}
```

### 3. 执行 Saga

注入 Coordinator 直接调用:

```go
type OrderService struct {
    Coord transaction.Coordinator `autowire:""`
}

func (s *OrderService) Place(ctx context.Context, id string) (transaction.Result, error) {
    return s.Coord.Execute(ctx, transaction.Saga{
        ID:     id,
        Method: "OrderService.Place",
        Steps:  []transaction.Step{deductInventory, chargePayment, publishEvent},
    })
}
```

### 4. 或声明为 `@GlobalTransactional`

把同样的 Step 按方法名注册,再把 `transaction.GlobalTransactional` 接入
aspect 链——即 `@GlobalTransactional(SAGA)` 的无反射等价物:

```go
func RegisterOrder(reg *transaction.StepRegistry) {
    reg.Register("OrderService.Place", deductInventory, chargePayment, publishEvent)
}

chain := aspect.NewChain(transaction.GlobalTransactional(coord, reg))
```

Step 必须在**装配时**(bean 构造)注册,不能放到自定义 `Runner`,以确保启动
时恢复扫描前 registry 已就绪。

## 配置

绑定到 `${spring.transaction.saga}`(与 `starter-transaction-tcc` 共享
`spring.transaction` 能力命名空间)。

| 键 | 默认 | 说明 |
|---|---|---|
| `spring.transaction.saga.enabled` | `true` | 启停 starter bean。 |
| `spring.transaction.saga.tracing` | `true` | 每个 Step 阶段向 `starter-otel` 安装的 globals 发一个 otel 子 span。无 otel 时为 no-op。 |
| `spring.transaction.saga.recover-on-start` | `true` | 启动时扫描 Store,对崩溃留下的在途 Saga 做后向补偿。 |

## 崩溃恢复

默认 Saga 日志只放**内存**——单进程场景与测试足够,但不抗崩溃。当
`recover-on-start` 为 true 时,`gs.Runner` 会在启动时扫描 `transaction.Store`,
对每条 `StatusRunning` 快照,按 `StepRegistry` 里的方法名重建 Step,交给
Coordinator 做后向恢复。若 Step 已不再注册,则记日志并跳过——恢复不能凭空
造出业务逻辑。使用内存 Store 时 `Pending` 重启后必为空,故本扫描是无害
no-op。

要让恢复有意义,需再导入一个持久化 Store 实现——目前是
[`starter-transaction-saga-gorm`](../starter-transaction-saga-gorm)。默认内存
Store 用 `gs.OnMissingBean` 注册,故一旦有 `transaction.Store` 贡献,
Coordinator 与恢复扫描会同时接过它,无需改动业务代码。

## 可观测

`tracing=true` 时,Coordinator 每个 Step 阶段发一个 otel 子 span
(`saga.action <step>`、`saga.compensate <step>`),带上 `saga.id`、
`saga.step`、`saga.phase` 属性;失败会记录到 span。`Observer` 缝隙放在
Coordinator 上,故 `stdlib/transaction` 不必依赖 otel。

## 许可

Apache 2.0。见 [LICENSE](../../LICENSE)。
