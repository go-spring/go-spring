# ordered

[English](README.md) | [中文](README_CN.md)

`ordered` 目前只提供一个产生 map 稳定遍历顺序的工具。这个包是未来"稳定遍历顺序"类工具的命名归口，不是有序 map
容器——当前场景用原生 map 加这个 helper 就够了。

## 使用方式

```go
import "go-spring.org/stdlib/ordered"

for _, k := range ordered.MapKeys(m) {
    fmt.Println(k, m[k])
}
```

### API 列表

- `MapKeys[M ~map[K]V, K cmp.Ordered, V any](m M) []K` —— 排序后的 key
  切片。

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。
