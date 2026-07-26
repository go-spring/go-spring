# starter-batch-redis

[English](README.md) | [中文](README_CN.md)

`starter-batch-redis` 为 Go-Spring 应用贡献一个基于 Redis 的
[`batch.JobRepository`](../../spring/batch) Bean，让 chunk 作业和短生命周期
任务把进度持久化到 Redis：进程崩溃后从最后一次已提交的 chunk 恢复，不会
重复处理已经写入的数据。

它属于 *Contributor* 形态（见 [starter/DESIGN.md](../DESIGN.md)）：
Starter 本身不占端口，也不持有自己的客户端；它复用 `starter-go-redis` 已
注册的 `*redis.Client`，仅在 `batch.JobRepository` 这个与框架无关的接缝
上贡献一个 Bean。切换到 SQL 数据库版只需要换一个 blank import，业务代码
不动。

## 安装

```bash
go get go-spring.org/starter-batch-redis
```

## 快速开始

### 1. 同时引入两个 Starter（如使用批处理 runner，再引入一个）

```go
import (
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-batch-redis"
    // _ "go-spring.org/starter-batch"        // 批处理 runner，可选
)
```

### 2. 先配置一个 Redis 客户端，再配置一个引用它的 JobRepository

```properties
# 由 starter-go-redis 管理的 Redis 客户端。
spring.go-redis.cache.addr=127.0.0.1:6379

# 绑定到该客户端的 JobRepository，`client` 是 Redis 实例名。
spring.batch-repository.jobs.client=cache
spring.batch-repository.jobs.key-prefix=myapp:batch:
spring.batch-repository.jobs.ttl=168h

# 批处理 runner 通过名字挑选一个 JobRepository。
spring.batch.repository=jobs
```

`client` 属性是**必填**的。启动时若缺失，Starter 会 fail-fast 直接拒绝
启动，不会静默 fallback 到某个默认实例。

### 3. 注入 `batch.JobRepository`

```go
import "go-spring.org/spring/cloud/batch"

type Service struct {
    Repo batch.JobRepository `autowire:"jobs"`
}
```

绝大多数业务代码不会直接使用 `JobRepository`——批处理 runner 才是主要
消费方。手动注入通常出现在进度看板 / 管理工具里，用来调用
`ListStepExecutions`。

## 配置项

所有键都位于 `spring.batch-repository.<name>` 下。这里刻意避开
`spring.batch.*`——那是批处理 runner 用于 job / step / chunk 的命名空间。

| 键           | 默认值   | 说明                                                                                     |
|--------------|----------|------------------------------------------------------------------------------------------|
| `client`     | —        | **必填。** 位于 `spring.go-redis.<client>` 下的 `*redis.Client` Bean 名。                |
| `key-prefix` | *空*     | 键名前缀，多应用共享同一 Redis 时用于隔离命名空间。                                       |
| `ttl`        | `0`      | 每次写入时对相关键执行 `EXPIRE`。`0` 表示永不过期。                                       |

## 键结构

以 `key-prefix=myapp:batch:` 为例：

| Redis 键                              | 类型      | 内容                                               |
|---------------------------------------|-----------|----------------------------------------------------|
| `myapp:batch:job:<instanceKey>`       | string    | 该实例的 `batch.JobExecution`（JSON）。            |
| `myapp:batch:steps:<jobExecutionID>`  | hash      | 字段名=step 名，值为 `StepExecution`（JSON）。     |
| `myapp:batch:seq`                     | counter   | `INCR` 源，用于生成单调递增的 execution ID。       |

`<instanceKey>` 是 job name 加上排序后的 params 的 SHA-1——和内存版仓库
完全一致的算法，所以同一 `(name, params)` 的两次运行共享一个
`JobExecution`；若上一次没跑完，第二次会自动恢复。

## 保证

* **进程崩溃后可恢复** —— 每个已提交的 chunk 都会通过 `HSET` 落 Redis，
  重启后从最后一次成功提交继续，不会重复读取已经提交过的 chunk。
* **fail-fast 配置** —— 缺少 `client` 直接启动失败，而不是等到第一次
  `SaveStepExecution` 才暴露问题。
* **客户端生命周期隔离** —— 仓库不会关闭注入的 `*redis.Client`，那是
  `starter-go-redis` 的职责。
* **TTL 维护** —— 当 `ttl > 0` 时，每次写入都会刷新键的过期时间，长时间
  运行的 job 不会中途丢记录。

## 切换后端

由于仓库是通过 `batch.JobRepository` 接口贡献的，换到别的后端只需要
换一个 blank import 和配置前缀——批处理 runner 通过
`spring.batch.repository` 按名字引用仓库，业务代码不用改。
