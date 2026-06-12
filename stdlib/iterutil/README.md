# iterutil

[English](README.md) | [中文](README_CN.md)

`iterutil` is a small and handy Go utility package that makes your loops more elegant and ✨functional✨.
It’s designed to solve the problem where `defer` statements inside standard `for` loops only execute
when the **entire function** returns!

## Usage

### 🔂 Times

`Times` executes a callback function a specified number of times.

```go
iterutil.Times(5, func (i int) {
    fmt.Println(i) // prints 0 through 4
})
```

### 📈 Ranges

`Ranges` iterates from `start` to `end` (exclusive) and applies the callback function to each index.
It supports both ascending and descending ranges.

```go
iterutil.Ranges(2, 5, func (i int) {
    fmt.Println(i) // prints 2, 3, 4
})

iterutil.Ranges(5, 2, func (i int) {
    fmt.Println(i) // prints 5, 4, 3
})
```

### 🏃 StepRanges

`StepRanges` lets you customize the step size, giving you full control over iteration intervals — forward or backward.

```go
iterutil.StepRanges(0, 10, 2, func(i int) {
    fmt.Println(i) // prints 0, 2, 4, 6, 8
})

iterutil.StepRanges(10, 0, -3, func (i int) {
    fmt.Println(i) // prints 10, 7, 4, 1
})
```

## Why Use It?

In traditional `for` loops, any `defer` statements execute only when the **enclosing function** returns — not after each
iteration.

With `iterutil`, you can use closures to scope each iteration and ensure `defer` runs **right when you expect it to**.
🎯

Example:

```go
iterutil.Times(3, func (i int) {
    defer fmt.Println("deferred", i)
    fmt.Println("running", i)
})
```

Output:

```
running 0
deferred 0
running 1
deferred 1
running 2
deferred 2
```

## License

This project is licensed under the [MIT License](LICENSE).
