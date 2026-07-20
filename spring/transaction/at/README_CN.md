# at
[English](README.md) | [中文](README_CN.md)

`at` 是 AT(Automatic Transaction)的零依赖抽象——Go 惯用法版 Seata AT。与
Saga / TCC 不同,业务只写正向 SQL;资源侧 interceptor 为每条语句抓 before-
image,回滚时自动按镜像还原行。

## 特性

- `Coordinator`:`Begin` / `Register` / `Commit` / `Rollback`;XID 通过
  `WithXID` / `XIDFromContext` 挂在 context 上。
- `Branch` 接口——由后端 starter 实现,负责持久化 undo log、还原行
  (见 `starter-transaction-at-gorm`)。
- `GlobalLock` 接口 + 内建 `MemoryGlobalLock` 提供写-写隔离(冲突返
  `ErrLockConflict`);分布式部署换共享后端。
- `Observer` 缝隙接 otel(nil 关掉观测)。
- `RetryPolicy = resilience.Policy` 别名,用于二阶段重试。
- `GlobalAT(coord)` aspect——AT 版 `@GlobalTransactional`;无需 per-method
  注册表。

## 用法

装配 coordinator 与 aspect:

```go
package main

import (
    "context"

    "go-spring.org/spring/aspect"
    "go-spring.org/spring/transaction/at"
)

var coord = at.NewCoordinator(
    at.WithGlobalLock(&at.MemoryGlobalLock{}),
)

// GORM 后端在 context 上看到 XID 时,通过资源 interceptor 注册 branch;
// 业务代码依然写普通 SQL。
var interceptors = []aspect.Interceptor{
    at.GlobalAT(coord),
}

func PlaceOrder(ctx context.Context) error {
    // 经 AT-aware ORM 写入的语句会自动被抓 before-image;函数返错就自动
    // 回滚。
    return nil
}
```

进入方法时 coordinator 开全局事务、把 XID 塞进 ctx,方法返回时按 error 决定
commit / rollback。嵌套 `GlobalAT` 复用外层 XID(不做嵌套全局事务)。
