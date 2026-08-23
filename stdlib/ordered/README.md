# ordered

[English](README.md) | [中文](README_CN.md)

`ordered` currently exposes a single helper for deterministic iteration order
over a map. A named
location for future "deterministic order" helpers — not an ordered-map data
structure, since Go's built-in map plus this helper is enough for the current
use cases.

## Usage

```go
import "go-spring.org/stdlib/ordered"

for _, k := range ordered.MapKeys(m) {
    fmt.Println(k, m[k])
}
```

### API

- `MapKeys[M ~map[K]V, K cmp.Ordered, V any](m M) []K` — sorted slice of the
  map's keys.

## License

Apache License 2.0. See [LICENSE](../../LICENSE).
