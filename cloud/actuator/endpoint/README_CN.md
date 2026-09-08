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
    gs.Provide(&endpoint.Endpoint{Pattern: "/metrics", Handler: promhttp.Handler()})
}
```

Pattern 不能和 actuator 内置 pattern（`/healthz`、`/readyz`、`/info`...）或
其他端点冲突，重复 pattern 启动时 panic。声明 `Sensitive` 的端点只有显式列入
`spring.actuator.endpoints.include` 才注册——敏感性由贡献者声明，actuator 不代为
裁决。过滤按 pattern 的路径匹配（去掉方法前缀），如 /env、/metrics。
