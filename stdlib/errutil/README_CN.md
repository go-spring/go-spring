# errutil

[English](README.md) | [中文](README_CN.md)

`errutil` 是一个轻量级的错误处理工具包，它提供了两种不同的语义方式来包装错误：

1. `解释型`包装（Explanatory wrapping）- 为错误添加人类可读的含义或解释，阐明业务或逻辑层面出了什么问题。
2. `堆栈型`包装（Stack wrapping）- 添加上下文调用路径信息，显示错误在调用链中的传递路径。

这两种模式有不同的用途：

- 解释型错误面向用户，具有语义性：例如 "无法加载配置: 文件未找到"
- 堆栈型错误面向开发者，具有结构性：例如 "InitService >> LoadConfig >> 文件未找到"

目标是通过将解释（":"）与追踪路径（">>"）分离，使错误包装更具表达力。

## 使用方法

### 解释型包装

使用 `Explain` 函数为错误添加语义解释：

```go
err := errors.New("connection refused")
return errutil.Explain(err, "failed to connect to database")
// 输出: "failed to connect to database: connection refused"
```

### 堆栈型包装

使用 `Stack` 函数添加调用路径信息：

```go
err := errors.New("file not found")
return errutil.Stack(err, "LoadConfig")
// 输出: "LoadConfig >> file not found"
```

### 组合使用

`Explain` 和 `Stack` 可以组合使用，先为错误添加语义，再附加调用路径：

```go
baseErr := errors.New("file not found")
baseErr = errutil.Explain(baseErr, "failed to load configuration")
err := errutil.Stack(baseErr, "InitService")
// 输出: "InitService >> failed to load configuration: file not found"
````

这种组合方式既保留了业务语义，又体现了错误传播路径，非常适合在中大型项目中使用。

## 公开 API

- `Explain(err, format, args...) error` — 用 `":"` 包装。
- `Stack(err, format, args...) error` — 用 `" >> "` 包装。
- `ErrForbiddenMethod`、`ErrUnimplementedMethod` — 用于"禁用调用"和"未实现"
  两种场景的哨兵错误。

两个包装函数都通过 `fmt.Errorf("... %w", err)` 保留原始错误，因此
`errors.Is` / `errors.As` 在整条链上仍然可用。