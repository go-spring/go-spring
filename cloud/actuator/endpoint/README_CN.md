# endpoint

[English](README.md) | [中文](README_CN.md)

组件想在 actuator 的管理端口上多挂一个 HTTP 路径,就贡献一个 `Endpoint`
bean。actuator 收集所有 `endpoint.Endpoint` 类型的 bean,逐个挂到自己的
mux 上,和内置探针端点并列。

## 安装

```
go get go-spring.org/cloud
```

## 用法

给 actuator 贡献 Prometheus `/metrics`:

```go
import (
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go-spring.org/gs"
    "go-spring.org/cloud/actuator/endpoint"
)

func init() {
    gs.Provide(&endpoint.Endpoint{Path: "/metrics", Handler: promhttp.Handler()})
}
```

Path 不能和 actuator 内置路径(`/healthz`、`/readyz`、`/info`...)或其他
贡献的 endpoint 冲突,重复路径启动时 panic。每个 endpoint 还受 actuator 的
`spring.actuator.endpoints.include` / `.exclude` 过滤,名字取去掉首斜杠的
路径。
