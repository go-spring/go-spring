# transaction
[English](README.md) | [中文](README_CN.md)

`transaction` 是零依赖的 Saga 抽象,面向跨资源 / 跨服务的最终一致性——Go 惯
用法版的 Seata Saga + Spring `@GlobalTransactional`。长业务被表达为一串可补偿
的 `Step`;某步失败,之前成功的步骤按逆序执行 `Compensate`。

TCC / AT 见子包 [`transaction/tcc`](tcc/README.md) 与
[`transaction/at`](at/README.md)。

## 特性

- `Saga` + `Step{Action, Compensate}`——纯 Go,无注解。
- `Coordinator` 接口 + 内建 `NewCoordinator` 进程内实现。
- `Store` 缝隙持久化 saga 日志;内建 `MemoryStore`;持久化后端由 starter 贡献
  bean。
- `Recover(ctx, s)` 后向恢复:重放崩溃进程可能已副作用的补偿。
- `Observer` 缝隙——otel 不进 stdlib。
- `StepRegistry` + `GlobalTransactional(coord, reg)`——切面级的
  `@GlobalTransactional` 等价物,按方法名匹配。
- 步骤级 `RetryPolicy`(等价 `resilience.Policy`)复用出站韧性的同一套配置。

## 用法

```go
package main

import (
    "context"
    "fmt"

    "go-spring.org/stdlib/transaction"
)

func main() {
    coord := transaction.NewCoordinator(transaction.WithStore(&transaction.MemoryStore{}))

    saga := transaction.Saga{
        ID:     "order-42",
        Method: "PlaceOrder",
        Steps: []transaction.Step{
            {
                Name:       "reserve-stock",
                Action:     func(ctx context.Context) (any, error) { return "res-1", nil },
                Compensate: func(ctx context.Context, r any) error { fmt.Println("撤库存", r); return nil },
            },
            {
                Name:       "charge-card",
                Action:     func(ctx context.Context) (any, error) { return nil, fmt.Errorf("刷卡失败") },
                Compensate: func(ctx context.Context, r any) error { return nil },
            },
        },
    }

    res, err := coord.Execute(context.Background(), saga)
    fmt.Println(res.Status, err) // Compensated, 刷卡失败
}
```

## 切面(`@GlobalTransactional`)形态

```go
reg := transaction.NewStepRegistry()
reg.Register("PlaceOrder",
    transaction.Step{Name: "reserve-stock", Action: reserveStock, Compensate: releaseStock},
    transaction.Step{Name: "charge-card",   Action: charge,       Compensate: refund},
)
gtx := transaction.GlobalTransactional(coord, reg)
// 把 gtx 装进切面链(见 stdlib/aspect):joinpoint 方法名命中就跑 saga,否则
// 透明放行。
```
