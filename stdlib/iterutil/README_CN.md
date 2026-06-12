# iterutil

[English](README.md) | [中文](README_CN.md)

`iterutil` 是一个简单又实用的 Go 工具包，用来让你的循环变得更优雅、更 ✨函数式✨。  
它专门用来解决在标准 `for` 循环中，`defer` 只能在整个函数退出时才执行的问题！

## 使用指南

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

## 为什么需要它？

在传统 `for` 循环中写 `defer`，所有延迟操作都会在**函数返回**时才统一执行，而不是在每次循环迭代时执行。  

使用 `iterutil`，可以通过闭包手动控制作用域，让每次循环中的 `defer` 在预期时机生效！🎯

示例：

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

本项目遵循 [MIT License](LICENSE)。