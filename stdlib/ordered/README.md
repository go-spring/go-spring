# ordered
[English](README.md) | [中文](README_CN.md)

`ordered` currently exposes a single helper for producing deterministic
iteration order over a map. Part of Go-Spring's zero-dependency `stdlib`
layer.

## API

- `MapKeys[M ~map[K]V, K cmp.Ordered, V any](m M) []K` — sorted slice of the
  map's keys.

## Usage

```go
import "go-spring.org/stdlib/ordered"

for _, k := range ordered.MapKeys(m) {
    fmt.Println(k, m[k])
}
```

Used inside the framework whenever log output, JSON marshaling, or diagnostic
dumps need a stable key order.
