# flatten
[English](README.md) | [中文](README_CN.md)

`flatten` 把 JSON 结构的嵌套数据打平成 `key -> string`，并提供 Go-Spring 配置
绑定器所依赖的 `Storage` 抽象。属于零依赖的 `stdlib` 层。

## 功能

- `Flatten(map[string]any) map[string]string` —— 用点号 / 方括号
  （`a.b`、`a[0]`）打平嵌套 map 和切片。
- `Path`、`JoinPath`、`SplitPath` —— 层级 key 的解析与拼接。
- `Properties` / `PropertiesStorage` —— 扁平化 `key -> string` 存储，实现
  `Storage` 接口，供绑定器使用。
- `PrefixedStorage` —— 透明地为所有 key 增加前缀。
- `LayeredStorage` —— 按固定优先级组合多个配置源
  （`StorageCommandLine`、`StorageEnvironment`、`StorageProfileFile`、
  `StorageAppFile`、`StorageDefault`）。

## 用法

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

## 扁平化规则

- 嵌套 map 用 `.` 展开：`{"a":{"b":1}}` -> `"a.b"="1"`。
- 切片用 `[i]` 展开：`{"a":[1,2]}` -> `"a[0]"="1"`、`"a[1]"="2"`。
- 无类型或有类型的 `nil` 都表示为 `"<nil>"`。
- 非 nil 但为空的 map 表示为 `"{}"`，空切片表示为 `"[]"`。
- 基本类型走 `strconv` 格式化。
