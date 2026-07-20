# tcc
[English](README.md) | [中文](README_CN.md)

`tcc` 是 TCC(Try / Confirm / Cancel)分布式事务的零依赖抽象——Go 惯用法版
Seata TCC。面向短、强一致的事务:Try 阶段预留资源但不暴露最终结果,全局
Confirm / Cancel 决定去留。

姊妹模式 Saga、AT 分别在 [`spring/transaction`](../README.md) 与
[`spring/transaction/at`](../at/README.md)。

## 特性

- `Transaction` + `Participant{Try, Confirm, Cancel}`——纯 Go,无注解 /
  代理。
- `Coordinator.Execute(ctx, t)`——全 Try -> 决策 -> 顺序 Confirm 或
  逆序 Cancel。
- `Coordinator.Recover(ctx, t)`——`StatusConfirming` 前向恢复,
  `StatusTrying` / `StatusCancelling` 后向恢复。
- `Store` 缝隙持久化决策日志;内建 `MemoryStore`;持久化后端由 starter 贡献
  bean。
- `Observer` 缝隙接 otel(stdlib 不 import otel)。
- `ParticipantRegistry` + `GlobalTCC(coord, reg)` 切面形态——按方法名匹配。
- `RetryPolicy = resilience.Policy` 别名,复用出站韧性配置;Confirm / Cancel
  按 TCC 契约"最终必须成功",推荐设非零策略。

## 用法

```go
package main

import (
    "context"
    "fmt"

    "go-spring.org/spring/transaction/tcc"
)

func main() {
    coord := tcc.NewCoordinator(tcc.WithStore(&tcc.MemoryStore{}))

    tx := tcc.Transaction{
        ID:     "order-42",
        Method: "PlaceOrder",
        Participants: []tcc.Participant{
            {
                Name:    "stock",
                Try:     func(ctx context.Context) (any, error) { return "hold-1", nil },
                Confirm: func(ctx context.Context, r any) error { fmt.Println("提交预留", r); return nil },
                Cancel:  func(ctx context.Context, r any) error { fmt.Println("释放预留", r); return nil },
            },
            {
                Name:    "balance",
                Try:     func(ctx context.Context) (any, error) { return "freeze-9", nil },
                Confirm: func(ctx context.Context, r any) error { return nil },
                Cancel:  func(ctx context.Context, r any) error { return nil },
            },
        },
    }

    res, err := coord.Execute(context.Background(), tx)
    fmt.Println(res.Status, err) // Committed <nil>
}
```

## Participant 义务

- **Confirm / Cancel 幂等**——崩溃重试会重放。
- **空回滚**——Cancel 可能拿到 nil result(Try 未记录),必须容忍。
- **防悬挂**——迟到的 Try 出现在 Cancel 之后不能再预留;用 transaction id
  给预留做 key。
