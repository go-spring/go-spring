# starter-pprof

[English](README.md) | [中文](README_CN.md)

`starter-pprof` 是一个 Go-Spring starter，用于通过 Go-Spring IoC 容器管理的轻量级 HTTP
服务器暴露标准 Go `net/http/pprof` 调试端点。

它适用于需要快速查看运行时状态、采集 CPU profile、捕获 trace，以及调试 goroutine、heap、
thread、mutex、block 等 profile 信息的 Go-Spring 应用。

## 功能特性

- 通过导入 starter 自动注册 `gs.Server` bean。
- 暴露 Go 标准库 `net/http/pprof` 提供的 `/debug/pprof/` 相关端点。
- 使用独立 HTTP 地址启动 pprof 服务，与主应用服务隔离。
- 支持通过配置项控制是否启用以及监听地址。
- 使用空白导入即可接入，无需手动注册路由。

## 安装

```bash
go get github.com/go-spring/starter-pprof
```

## 使用方式

在 Go-Spring 应用中通过空白导入启用 starter：

```go
package main

import (
	"go-spring.org/spring/gs"
	_ "github.com/go-spring/starter-pprof"
)

func main() {
	gs.Run()
}
```

默认配置下，pprof 服务监听 `:9981`：

```text
http://127.0.0.1:9981/debug/pprof/
```

## 配置项

该 starter 读取以下 Go-Spring 配置：

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `spring.pprof.enabled` | `true` | 是否启用 pprof 服务。 |
| `spring.pprof.addr` | `:9981` | pprof 独立 HTTP 服务监听地址。 |

示例：

```properties
spring.pprof.enabled=true
spring.pprof.addr=:9090
```

配置后访问：

```text
http://127.0.0.1:9090/debug/pprof/
```

## 可用端点

该 starter 注册了标准 pprof handler：

- `/debug/pprof/`
- `/debug/pprof/cmdline`
- `/debug/pprof/profile`
- `/debug/pprof/symbol`
- `/debug/pprof/trace`

goroutine、heap、allocs、mutex、block、threadcreate 等 profile 视图由 pprof index
handler 提供，具体可用项取决于 Go 运行时。

## 许可证

本项目基于 Apache License 2.0 开源。
