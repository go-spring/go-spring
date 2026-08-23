# flatten

[English](README.md) | [中文](README_CN.md)

`flatten` 把 JSON 结构的嵌套数据打平成 `key -> string`，并提供 Go-Spring 配置
绑定器所依赖的 `Storage` 抽象。它不是配置加载器：不会读文件、环境变量或命令行，
来源由调用方自己拼装 `Properties` 并放入某一层。

## 使用方式

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

### API 列表

| API | 说明 |
|---|---|
| `Flatten(map[string]any) map[string]string` | 嵌套 map 打平为 `key -> string` |
| `Path` / `PathType` | 层级 key 的一个解析段（key 或 index） |
| `SplitPath(key) ([]Path, error)` | 把 `"a.b[0]"` 解析成 `[]Path`，语法非法时报错 |
| `JoinPath([]Path) string` | `SplitPath` 的逆操作，可往返 |
| `Properties` | 扁平 `key -> string` 存储，支持 `Get` / `Set` / `Data` |
| `MapProperties(map[string]any) *Properties` | 等价于 `NewProperties(Flatten(m))` |
| `PropertiesStorage` | 把 `Properties` 适配成 `Storage` 接口 |
| `PrefixedStorage` | 包装任意 `Storage`，透明地为所有 key 增加前缀 |
| `LayeredStorage` | 按固定优先级组合多个配置源，实现 `Storage` |
| `LayeredStorage.Sources() []Source` | 各配置源的只读快照（按优先级降序、不合并） |

## 扁平化规则

`Flatten` 面向 `encoding/json.Unmarshal` 产出的数据形态，递归展开：

| 输入形态 | 输出 |
|---|---|
| `{"a":{"b":1}}` | `"a.b" = "1"`（map 用 `.` 展开） |
| `{"a":[1,2]}` | `"a[0]" = "1"`、`"a[1]" = "2"`（切片用 `[i]` 展开） |
| `{"a":nil}` / 有类型 nil（nil map / nil slice / nil 指针） | `"a" = "<nil>"` |
| `{"a":{}}`（非 nil 空 map） | `"a" = "{}"` |
| `{"a":[]}`（非 nil 空切片） | `"a" = "[]"` |
| 基本类型 | 走 `strconv` 确性格式化（`true`、`8080`、`3.14`） |

## Key 路径语法（Path / SplitPath / JoinPath）

`SplitPath` 是绑定器解析层级 key 的入口，语法与 `Flatten` 的输出严格对应：

```
"foo.bar[0]" -> [{key foo} {key bar} {index 0}]
"a[1][2]"    -> [{key a} {index 1} {index 2}]
```

`JoinPath` 是 `SplitPath` 的逆操作，`JoinPath(SplitPath(k)) == k` 对合法 key
可往返，因此 `Path` 切片是代码中操作 key 的规范中间表示（遍历、改写、截取）。

## Storage 接口

绑定器只依赖这四个方法，是接入自定义配置源的唯一契约：

```go
type Storage interface {
    Exists(key string) bool                          // 属性条件判断用（OnProperty 等）
    Value(key string) (string, bool)                 // 叶子值查找（精确匹配）
    MapKeys(key string, result map[string]struct{}) bool // map 节点直接子 key 枚举
    SliceEntries(key string, result map[string]string) bool // 切片节点全部扁平项枚举
}
```

语义要点（以 `PropertiesStorage` 为例，`server.port=8080` 时）：

- **`Exists` 是前缀感知的**：`Exists("server")` 与 `Exists("server.port")` 都是
  `true`（中间节点也算存在），`Exists("server.host")` 是 `false`。它服务于
  "配置了某前缀就生效"这类条件判断，而非绑定。
- **`Value` 只做精确匹配**，不猜前缀、不拼路径。
- **`MapKeys` 只返回一层**：`server.host`、`server.port` 在 `key="server"` 时
  返回 `{"host","port"}`，不递归。
- **`SliceEntries` 不校验下标连续性**：只收集 `key[i]...` 形态的项，`[0]` 和
  `[2]` 都在也照样返回，合法性由绑定器裁决。

实现方只需保证"输入数据已经合法"——`Storage` 自身不做结构校验，这让远程配置
中心（nacos/etcd 等）的适配器可以做到极薄。

## LayeredStorage：分层优先级

`LayeredStorage` 按 Spring 风格的分层模型组合配置源，五个固定层级（数值越小
优先级越高）：

| 层级 | 含义 |
|---|---|
| `StorageCommandLine` | 命令行参数（最高） |
| `StorageEnvironment` | 环境变量 |
| `StorageProfileFile` | profile 文件（如 `application-dev.properties`） |
| `StorageAppFile` | 主配置文件（`application.properties` / `.yml`） |
| `StorageDefault` | 内置默认值（最低） |

```go
ls := &flatten.LayeredStorage{}
ls.AddStorage(flatten.StorageAppFile, appStorage, "application.properties")
ls.AddStorage(flatten.StorageEnvironment, envStorage, "env")
```

优先级规则：

1. 层级 index 小者优先；
2. **同层内后加入的 source 优先**（新 source 插到层内切片头部）。

### 跨层的三种聚合语义

这是本包最容易被误解、也最重要的一组规则——**不同结构在层间的合并方式刻意
不同**：

| 结构 | 语义 | 行为 |
|---|---|---|
| 叶子值 | 覆盖 | 从最高优先层往下找到第一个即返回 |
| map | 合并 | 所有层的子 key 并集；同名 key 的取值仍按叶子覆盖规则解析 |
| 切片 | 整体覆盖 | 第一个定义了该切片的层胜出，低层切片被整体遮蔽 |

map 合并示例——两个 source 各定义一半，绑定方看到完整的 map：

```
source1: server.port=8080
source2: server.host=localhost
MapKeys("server") -> {port, host}
```

切片覆盖示例——**不是**逐元素覆盖：

```
source1: my.list[0]=a, my.list[1]=b     （低优先级层）
source2: my.list[0]=c                    （高优先级层）
SliceEntries("my.list") -> [c]           ✅ 而非 [c, b]
```

非对称是有意的：跨配置源"合并数组"语义不清（拼接？按下标覆盖？谁长听谁的？），
而合并 map key 才是调用方期望的形态——profile 文件覆盖主文件里的个别字段、
其余字段继续生效。另外 `SliceEntries` 还有一个细节：若更高优先级的层存在该 key
的**叶子值**（`my.list=x`），更低层定义的切片会被叶子遮蔽，直接返回未找到。

### Sources()：自省快照

`Sources()` 按优先级降序返回每个配置源的 `Name` + `Data`（深拷贝，可随意改），
**不做任何合并**。因为叶子/map/切片三者的聚合方式互不相同，任何单一合并视图
都必然失真——保留原始分层才是忠实呈现（actuator 的 env 端点就用它展示
"每个值来自哪个 source"）。它是诊断路径，不是绑定路径。

## 关键设计

- `Flatten` 是面向展示的单向转换，**不可逆**；只支持 JSON 原生类型，结构体、
  非字符串 map key、自定义类型都被显式排除。
- `Path` + `Split/JoinPath` 提供 key 路径的可往返表示；`Storage` 接口保持最小
  ——`Value` / `MapKeys` / `SliceEntries` 三种绑定能力加上属性条件判断用的
  `Exists`——方便接入远程配置等替代实现。
- `LayeredStorage` 有意混用两种覆盖规则（见上节）：叶子与切片高优先级层胜出，
  map 跨层合并 key、叶子值仍按覆盖解析。
- `PrefixedStorage.SliceEntries` 剥前缀保证命名空间透明；`LayeredStorage.
  Sources()` 是自省快照，不是绑定路径。
- 所有遍历都是 O(n) 全表扫描（`Exists` / `MapKeys` / `SliceEntries` 都在
  扫 `map[string]string`）：配置规模下足够快，且换取了 `Properties` 的极简
  表示；若未来需要大规模 key 集，优化点在索引结构而不是接口。

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。
