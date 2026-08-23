# iterutil

[English](README.md) | [中文](README_CN.md)

`iterutil` is a small, handy set of callback-driven loops that make your code
more elegant and ✨functional✨. Each helper hands the loop body to a callback,
which gives `defer` per-iteration semantics: deferred calls fire when the
iteration's callback returns, not when the enclosing function returns — the
classic footgun of `defer` inside a standard `for` loop. It is deliberately not a full iteration
DSL: Go's own `for` is still the tool for the vast majority of loops; use
these helpers only when per-iteration cleanup matters.

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

### Why Use It?

In a traditional `for` loop, `defer` statements execute only when the
**enclosing function** returns — not after each iteration. With `iterutil`,
closures scope each iteration so `defer` runs **right when you expect it
to**. 🎯

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

Apache License 2.0. See [LICENSE](../../LICENSE).
