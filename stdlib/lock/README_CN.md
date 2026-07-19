# lock
[English](README.md) | [中文](README_CN.md)

`lock` 是与框架无关、零依赖的分布式锁与选主抽象。它回答多副本部署下的问题
"这个副本现在能不能跑这段独占工作?"——面向定时任务、单例后台工作者、一次性
迁移,以及所有需要跨部署至多同时一份运行的场景。

## 特性

- 抽象层零第三方依赖。
- `Locker` 接口由 Redis、etcd、Consul(各自独立 starter)或内置进程内
  `MemoryLocker`(测试 / 单节点)支撑——切换后端只需 blank import 换包。
- `Lock` 句柄暴露 `Key`、`Token`(fencing token)、幂等 `Unlock`,以及租约失
  效时会 close 的 `Lost()` 通道,让临界区能够中止。
- 通过函数选项配置 `TTL`、`RenewInterval`(负值禁用自动续期)、
  `RetryInterval` 与显式 `Token`。
- `Election` 基于任意 `Locker` 提供选主,一份代码对 Redis / etcd / Consul /
  内存后端通用。

## 快速开始

Import 路径: `go-spring.org/stdlib/lock`。

```go
package main

import (
    "context"
    "log"
    "time"

    "go-spring.org/stdlib/lock"
)

func main() {
    locker := lock.NewMemoryLocker()
    defer locker.Close()

    l, err := locker.Acquire(context.Background(), "jobs/rollup",
        lock.WithTTL(30*time.Second))
    if err != nil {
        log.Fatal(err)
    }
    defer l.Unlock(context.Background())

    select {
    case <-l.Lost():
        log.Println("锁失效,中止")
    default:
        // 执行独占工作
    }
}
```

基于同一抽象做选主:

```go
elect := lock.NewElection(lock.ElectionConfig{
    Locker: locker,
    Key:    "leaders/reporter",
    OnElected:  func(ctx context.Context) { /* 直到 ctx done 前跑 leader 工作 */ },
    OnResigned: func()                    { /* 清理 */ },
})
_ = elect.Run(context.Background())
```

生产集群用 starter 贡献 Redis / etcd / Consul 的 `Locker` bean;业务代码只
注入 `lock.Locker`,始终不需要改动。
