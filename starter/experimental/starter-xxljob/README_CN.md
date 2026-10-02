# starter-xxljob

[English](README.md) | [中文](README_CN.md)

`starter-xxljob` 运行 [xxl-job](https://github.com/xuxueli/xxl-job) 执行器：
以纯 HTTP 对标准 xxl-job admin 说执行器协议（注册 / run / kill / log），
手写实现、零三方依赖。

## 安装

```bash
go get go-spring.org/starter-xxljob
```

## 快速开始

### 1. 导入

```go
import _ "go-spring.org/starter-xxljob"
```

### 2. 配置

```properties
spring.xxljob.instances.a.app-name=go-spring-demo
spring.xxljob.instances.a.admin-addresses=http://127.0.0.1:8080/xxl-job-admin
spring.xxljob.instances.a.port=9999
```

### 3. 注入并注册 handler

```go
type Service struct {
    Executor *StarterXxljob.Executor `autowire:"a"`
}

func (s *Service) Init() error {
    s.Executor.RegisterHandler("demoJob", func(ctx context.Context, param string) error {
        return nil // 返回错误即向 admin 上报失败
    })
    return nil
}
```

## 核心特性

- **执行器协议** — /run /beat /idleBeat /kill /log 回调服务，加对 admin 的
  注册/心跳/注销循环。
- **可取消任务** — /kill 取消运行中 handler 的 context；panic 经共享
  panic 链恢复。
- **零依赖** — 协议即 API，无 SDK。

## 高级特性

**多执行器** — 更多 `spring.xxljob.instances.<name>` 条目，各自独立的回调端口与
handler 注册表。

## 设计说明

* **回调端口必填、无默认值。** admin 必须能拨回执行器以触发任务，因此端口是
  运维的显式决策。
* **协议是刻意子集。** 注册／心跳／`/run`／`/idleBeat`／`/kill`／`/log` 加上
  `/api/callback` 回传。广播／分片（`broadcastIndex`／`broadcastTotal`）只解析不
  执行，GLUE（shell／python）执行不在范围内——这类任务需要外部 handler。
* **注册与服务器同生命周期。** 注册／心跳／注销循环跑在执行器 `Run` 生命周期里，
  关机时注销。
* **任务是带可取消 context 的 goroutine。** `/kill` 取消运行中的 handler，handler
  panic 会转成 `500` 回调，而不是让执行器崩溃。
