# endpoint

[English](README.md) | [中文](README_CN.md)

组件想在 actuator 的管理端口上多挂一个 HTTP 路径，就贡献一个 `Endpoint`
bean。actuator 收集所有 `endpoint.Endpoint` 类型的 bean，逐个挂到自己的
mux 上，和内置探针端点并列。

## 安装

```
go get go-spring.org/cloud
```

## 用法

给 actuator 贡献 Prometheus `/metrics`：

```go
import (
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go-spring.org/gs"
    "go-spring.org/cloud/actuator/endpoint"
)

func init() {
    gs.Provide(&endpoint.Endpoint{Pattern: "/metrics", Handler: promhttp.Handler()})
}
```

Pattern 不能和 actuator 内置 pattern（`/healthz`、`/readyz`、`/info`...）或
其他端点冲突，重复 pattern 启动时 panic。贡献出来的端点直接注册：端点存不存在
由贡献者自己的 enable 开关决定，访问控制交给管理端口的鉴权，不做逐端点过滤。

## 设计说明

- actuator 只装配端点，从不裁决它们。贡献出来的端点无条件注册，和内置 `/info`
  一样——不做逐端点过滤。`spring.actuator.endpoints.include` 与
  `spring.actuator.endpoints.exclude`，连同 `endpoint.Endpoint.Sensitive`
  字段，均已删除。端点存不存在由贡献者自己的 enable 开关决定；访问控制归管理
  端口的整端口 Guard。
- 内置与贡献端点共用同一个 `Endpoint` 类型，按 `Endpoint.Pattern` 挂载
  （ServeMux 语法，可带方法）。没有 `route` 结构体，也没有「name」概念。
- 自省面收敛到只剩 `/info`（健康探测另走 `/healthz`、`/readyz`、`/startupz`）。
  `/beans`、`/configprops`、`/env`、`/loggers`、`/threaddump` 均已删除，别提议
  加回，树视图或合并型配置端点也不要；动态改日志级别端点
  （`POST /loggers/{name}`）同样被拒，因为 log 核心只导出只读的 `Loggers()`。
  内置端点无敏感项。
- 若将来确需逐端点控制条件，条件挂在 `endpoint.Endpoint` 上、由贡献方声明
  （如 `Enabled func() bool` 谓词），actuator 注册时统一询问——不是集中式
  include/exclude 配置清单，后者已被否决。这些都尚未实现，勿提前加。
