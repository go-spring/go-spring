# flatten
[English](README.md) | [中文](README_CN.md)

`flatten` turns hierarchical JSON-shaped data into flat `key -> string` maps
and provides the storage abstraction the Go-Spring configuration binder reads
against. Part of the zero-dependency `stdlib` layer.

## Features

- `Flatten(map[string]any) map[string]string` — flatten nested maps and slices
  with dot / bracket notation (`a.b`, `a[0]`).
- `Path`, `JoinPath`, `SplitPath` — parse and render hierarchical keys.
- `Properties` / `PropertiesStorage` — flattened `key -> string` store plus a
  `Storage` interface adapter used by the binder.
- `PrefixedStorage` — transparent key prefix wrapper.
- `LayeredStorage` — multi-source configuration with fixed precedence layers
  (`StorageCommandLine`, `StorageEnvironment`, `StorageProfileFile`,
  `StorageAppFile`, `StorageDefault`).

## Usage

```go
import "go-spring.org/stdlib/flatten"

flat := flatten.Flatten(map[string]any{
    "server": map[string]any{"port": 8080, "host": "localhost"},
    "users":  []any{map[string]any{"name": "tom"}},
})
// flat == {"server.port":"8080","server.host":"localhost","users[0].name":"tom"}

path, err := flatten.SplitPath("server.port")
_ = path // [{key server} {key port}]
_ = flatten.JoinPath(path)

s := flatten.NewPropertiesStorage(flatten.NewProperties(flat))
v, _ := s.Value("server.port")
```

## Flattening rules

- Nested maps expand with `.` (`{"a":{"b":1}}` -> `"a.b"="1"`).
- Slices expand with `[i]` (`{"a":[1,2]}` -> `"a[0]"="1"`, `"a[1]"="2"`).
- Untyped and typed `nil` values become `"<nil>"`.
- Empty (non-nil) maps become `"{}"`, empty slices become `"[]"`.
- Primitive values use `strconv` formatting.
