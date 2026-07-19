# hashutil
[English](README.md) | [中文](README_CN.md)

`hashutil` 是对 `hash/fnv` 的薄封装，属于 Go-Spring 零依赖的 `stdlib` 层。

## API

- `FNV1a64(s string) uint64` —— 使用标准库 `hash/fnv` 实现的字符串 64 位
  FNV-1a 哈希。

## 用法

```go
import "go-spring.org/stdlib/hashutil"

h := hashutil.FNV1a64("some/key")
```

FNV-1a 是快速的非密码学哈希，适合做 map 分片、缓存分桶等场景。攻击者可以
控制输入的场景请勿使用。
