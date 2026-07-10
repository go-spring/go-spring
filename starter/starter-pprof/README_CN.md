# starter-pprof

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-pprof` 通过 Go-Spring IoC 容器管理的独立轻量级 HTTP 服务器暴露标准
`net/http/pprof` 调试端点。适用于需要快速查看运行时状态、采集 CPU profile、捕获 trace，
以及调试 goroutine、heap、thread、mutex、block 等 profile 信息的 Go-Spring 应用。

## 安装

```bash
go get go-spring.org/starter-pprof
```

## 快速开始

### 1. 引入 `starter-pprof` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-pprof"
```

### 2. 配置 pprof 服务

在项目的[配置文件](example/conf/app.properties)中添加 pprof 配置：

```properties
spring.pprof.enabled=true
spring.pprof.addr=:9981
```

### 3. 访问 pprof 端点

默认配置下，pprof 服务监听 `:9981`：

```text
http://127.0.0.1:9981/debug/pprof/
```

## 核心功能

示例会访问 pprof 独立 HTTP 服务器（默认 `:9981`）上的三个代表性端点：

- **`GET /debug/pprof/`** —— 索引页，列出全部可用 profile。
- **`GET /debug/pprof/heap`** —— 堆分配快照。
- **`GET /debug/pprof/cmdline`** —— 当前进程的命令行参数，便于将 profile 与运行参数对齐。

三个端点必须全部返回 HTTP 200，示例才会自我关闭。

## 配置项

该 starter 读取以下 Go-Spring 配置：

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `spring.pprof.enabled` | `true` | 是否启用 pprof 服务。 |
| `spring.pprof.addr` | `:9981` | pprof 独立 HTTP 服务监听地址。 |

## 可用端点

该 starter 注册了标准 pprof handler：

- `/debug/pprof/`（`pprof.Index` 也会处理 `/heap`、`/goroutine`、`/allocs`、
  `/block`、`/mutex`、`/threadcreate` 等子路径）
- `/debug/pprof/cmdline`
- `/debug/pprof/profile`
- `/debug/pprof/symbol`
- `/debug/pprof/trace`

## 许可证

本项目基于 Apache License 2.0 开源。
