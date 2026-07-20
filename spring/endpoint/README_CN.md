# endpoint
[English](README.md) | [中文](README_CN.md)

`endpoint` 是一个极小的零依赖 seam,让组件向运维管理端口(通常是 actuator 的
端口)贡献 HTTP handler。组件导出 `Endpoint` bean,收集方(actuator)自动
把每个此类 bean 挂到 mux 上 —— 两侧不必互相 import。

## 特性

- 单一接口 `Endpoint { Path() string; http.Handler }`。
- 与 `go-spring.org/spring/health.Indicator` 同构:贡献方与收集方都只依赖
  stdlib,不产生跨 starter 依赖。

## 安装

```
go get go-spring.org/stdlib
```

## 用法

在不 import actuator 的前提下贡献 Prometheus `/metrics`:

```go
import (
    "net/http"

    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go-spring.org/gs"
    "go-spring.org/spring/endpoint"
)

type promEndpoint struct{ http.Handler }

func (promEndpoint) Path() string { return "/metrics" }

func init() {
    gs.Provide(func() endpoint.Endpoint {
        return promEndpoint{Handler: promhttp.Handler()}
    }).Export(gs.As[endpoint.Endpoint]())
}
```

actuator 会收集所有 `endpoint.Endpoint` bean 并逐一挂载。`Path` 要区别于
`/health`、`/readiness`、`/info` 以及其他贡献的路径。
