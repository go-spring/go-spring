# starter-ants

[English](README.md) | [中文](README_CN.md)

`starter-ants` 基于 [ants](https://github.com/panjf2000/ants) 提供了进程内协程池封装，
让你在 Go-Spring 应用中轻松集成并使用高性能、资源可控的协程池。

## 安装

```bash
go get go-spring.org/starter-ants
```

## 快速开始

### 1. 引入 `starter-ants` 包

参考 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-ants"
```

### 2. 配置 ants 协程池

在项目的[配置文件](example/conf/app.properties)中添加 ants 配置，例如：

```properties
spring.ants.main.size=256
```

### 3. 注入 ants 协程池

参考 [example.go](example/example.go) 文件。

```go
import "github.com/panjf2000/ants/v2"

type Service struct {
    Pool *ants.Pool `autowire:"main"`
}
```

### 4. 使用 ants 协程池

参考 [example.go](example/example.go) 文件。

```go
err := s.Pool.Submit(func() {
    // 在池化协程上执行任务
})
```

## 核心特性

[example.go](example/example.go) 程序演示并断言了三个核心 ants 操作：

* **Submit** —— 把任务分发到池化协程上执行，并确认全部运行。
* **实例隔离** —— 两个命名池彼此完全独立，由各自配置的容量证明。
* **非阻塞过载** —— 配置为非阻塞的池在满载时，`Submit` 返回 `ErrPoolOverload` 而非阻塞等待。

## 高级特性

* **支持多个 ants 协程池**：你可以在配置文件中定义多个池，并在项目中按名称引用它们。
* **支持 ants 扩展**：你可以通过实现 `Driver` 接口来扩展池的创建逻辑。
