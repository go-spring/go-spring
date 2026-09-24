# starter-lock-memory

[English](README.md) | [中文](README_CN.md)

`starter-lock-memory` 把 [lock.MemoryLocker](../../../../../cloud/lock/) 接入
Go-Spring，挂在与生产后端完全相同的 `spring.lock` 前缀下——配置键、bean 注入、
observe 包装全部一致，但零外部依赖即可运行。适用于本地开发、演示、以及消费锁的
代码的测试。

**不用于生产多副本协调**：锁的作用域是单进程。import 它就等于声明本部署单实例
运行；真需要协调时改 blank-import starter-lock-redis / -etcd / -consul / -k8s。

## 配置

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `spring.lock.instances.memory.<name>.observe.enabled` | `true` | 用共享 observe 适配器包装（span + `lock.operation.*` 指标 + 访问日志，system=`memory`）。 |

锁时序（TTL、续约、重试）不在此配置：它们由每次获取的 `lock.Option` 值承载，
所有后端完全一致。

```properties
spring.lock.instances.memory.demo.observe.enabled=true
```

```go
type Demo struct {
    Locker lock.Locker `autowire:"memory.demo"`
}
```

完整 starter 形态的示例见 [cloud/lock/example](../../../../../cloud/lock/example/)。
