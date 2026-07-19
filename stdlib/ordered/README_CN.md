# ordered
[English](README.md) | [中文](README_CN.md)

`ordered` 目前只提供一个用于产生 map 稳定遍历顺序的工具。属于 Go-Spring
零依赖的 `stdlib` 层。

## API

- `MapKeys[M ~map[K]V, K cmp.Ordered, V any](m M) []K` —— 排序后的 key
  切片。

## 用法

```go
import "go-spring.org/stdlib/ordered"

for _, k := range ordered.MapKeys(m) {
    fmt.Println(k, m[k])
}
```

框架内凡是日志、JSON 序列化、诊断输出需要稳定 key 顺序的地方都会用它。
