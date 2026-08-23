# iterutil

[English](README.md) | [中文](README_CN.md)

`iterutil` 是一组简单实用的回调驱动循环，让你的代码更优雅、更 ✨函数式✨。每个工具都把
循环体交给一个回调函数，让 `defer` 拥有"每次迭代"的语义：延迟调用在当次回调返回时立即
触发，而不是等到外层函数返回 —— 这正是标准 `for` 循环里写 `defer` 的经典陷阱。它刻意不做完整的迭代 DSL：绝大多数循环用原生 `for` 就够了，
只有真正需要按次清理时才用这里的工具。

## 使用方式

### 🔂 Times

`Times` 函数执行一个回调函数指定的次数。

```go
iterutil.Times(5, func (i int) {
    fmt.Println(i) // 输出 0 到 4
})
```

### 📈 Ranges

`Ranges` 从 `start` 到 `end`（不包含 `end`）进行遍历。支持正向和反向！

```go
iterutil.Ranges(2, 5, func (i int) {
    fmt.Println(i) // 输出 2, 3, 4
})

iterutil.Ranges(5, 2, func (i int) {
    fmt.Println(i) // 输出 5, 4, 3
})
```

### 🏃 StepRanges

`StepRanges` 允许你自定义步长，灵活控制每次迭代的间隔。正着走也行，倒着走也行！

```go
iterutil.StepRanges(0, 10, 2, func(i int) {
    fmt.Println(i) // 输出 0, 2, 4, 6, 8
})

iterutil.StepRanges(10, 0, -3, func (i int) {
    fmt.Println(i) // 输出 10, 7, 4, 1
})
```

### 为什么需要它？

在传统 `for` 循环中写 `defer`，所有延迟操作都会在**函数返回**时才统一执行，而不是在每次迭代时执行。
使用 `iterutil`，闭包为每次迭代划定作用域，让 `defer` 在预期时机生效！🎯

```go
iterutil.Times(3, func (i int) {
    defer fmt.Println("deferred", i)
    fmt.Println("running", i)
})
```

输出：

```
running 0
deferred 0
running 1
deferred 1
running 2
deferred 2
```

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。
