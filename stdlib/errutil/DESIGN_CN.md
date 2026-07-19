# errutil 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`errutil` 属于 `stdlib` —— Go-Spring 的零依赖基础层。它不持有任何框架状态，
每个函数都是对 Go 内置 `error` 的纯函数封装。

## 1. 职责与边界

- 提供两个正交的包装动词 `Explain` 与 `Stack`，让调用方显式表达意图，
  避免用 `fmt.Errorf` 拼接前缀带来的歧义。
- 提供少量常用哨兵错误 `ErrForbiddenMethod` / `ErrUnimplementedMethod`，
  供 stdlib 内其它包统一复用。
- 明确不做栈追踪：`Stack` 只按名字记录传递路径，不抓取 `runtime.Callers`
  帧信息。真正的调用栈能力交给专门的 tracing 包。

## 2. 关键设计决策

- 两个动词、两种分隔符：`:` 表示语义解释，`>>` 表示传递路径。拆开这两层
  可以消除"这个前缀是原因还是位置？"的模糊性。
- 内部均使用 `%w` 包装，保证 `errors.Is` / `errors.As` 在整条链上继续可用。
  errutil 不定义自定义错误类型，就是要与标准库互通。
- `nil` 入参的行为统一：内层 err 为 `nil` 时，两个函数都退化为
  `fmt.Errorf(format, args...)`，调用方无需自己判空。

## 3. 约束

- 零第三方依赖。errutil 位于 stdlib 层的最底端，被绝大多数 stdlib 包引用。
- 格式串遵循 `fmt` 语义。调用方不要显式写 `%w`，否则会二次包装。
