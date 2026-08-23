# errutil

[English](README.md) | [中文](README_CN.md)

`errutil` 是一个轻量级的结构化错误处理工具包；每个函数都是对 Go 内置 `error` 的纯函数封装。它提供两个正交的包装动词：**`Explain`** 添加人类可读的含义——业务层面*出了什么问题*，面向用户、语义化；**`Stack`** 添加调用路径上下文——错误*经过哪里*，面向开发者、结构化。此外还提供常用哨兵错误和快速失败的前置条件辅助函数。把解释（`:`）与追踪路径（`>>`）分开，让错误包装比临时拼 `fmt.Errorf` 前缀更具表达力。

## 使用方式

### 解释型包装

使用 `Explain` 为错误添加语义解释：

```go
err := errors.New("connection refused")
return errutil.Explain(err, "failed to connect to database")
// 输出: "failed to connect to database: connection refused"
```

### 堆栈型包装

使用 `Stack` 添加调用路径信息，便于调试或追踪：

```go
err := errors.New("file not found")
return errutil.Stack(err, "LoadConfig")
// 输出: "LoadConfig >> file not found"
```

### 组合使用

`Explain` 和 `Stack` 可以组合——先加语义解释，再附加调用路径，两者兼得：

```go
baseErr := errors.New("file not found")
baseErr = errutil.Explain(baseErr, "failed to load configuration")
err := errutil.Stack(baseErr, "InitService")
// 输出: "InitService >> failed to load configuration: file not found"
```

### API 列表

- `Explain(err, format, args...) error` —— 用 `":"` 包装。
- `Stack(err, format, args...) error` —— 用 `" >> "` 包装。
- `ErrForbiddenMethod`、`ErrUnimplementedMethod` —— 用于"禁用调用"和
  "未实现"两种场景的哨兵错误。

### 前置条件辅助

- `Field` —— 传给 `RequireAny` 的命名字符串值；`Name` 是错误信息里使用的
  人类可读属性名，`Value` 是该字段的当前值。
- `RequireField(component, field, value string) error` —— 必填值为空（含
  纯空白）时返回快速失败错误：

```go
if err := errutil.RequireField("mail", "host", cfg.Host); err != nil {
    return nil, err
}
```

- `RequireAny(component string, fields ...Field) error` —— 所有备选字段都
  为空时返回快速失败错误，用于"addr 或 service-name"这类声明式逐字段
  校验无法表达的跨字段规则：

```go
if err := errutil.RequireAny("http-client",
    errutil.Field{Name: "addr", Value: cfg.Addr},
    errutil.Field{Name: "service-name", Value: cfg.ServiceName},
); err != nil {
    return nil, err
}
```

## 关键设计

- **两个动词、两种分隔符**：`:` 是语义解释，`>>` 是传递路径。把这两层拆开，就再也不用猜"这个前缀到底是原因还是位置"。
- **一律 `%w`，不定义自己的错误类型**：内部统一走 `fmt.Errorf("... %w", err)`，所以 `errors.Is` / `errors.As` 在整条链上依然可用。与标准库互通就是目的，调用方也别再显式写 `%w`——那会二次包装。
- **`nil` 入参行为统一**：内层 err 为 `nil` 时，两个动词都退化成 `fmt.Errorf(format, args...)`。于是可以直接写 `errutil.Explain(nil, "reason: %s", x)`，不用先判空。
- **明确不做栈追踪库**：`Stack` 只按名字记传递路径，不抓 `runtime.Callers` 帧；更完整的调用栈交给专门的 tracing 包。

## 许可证

`errutil` 基于 Apache License 2.0 发布，详见 [LICENSE](../../LICENSE)。
