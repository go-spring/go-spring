# Go-Spring 实战第 1 课 —— 配置统一模型：Properties 与 Path

我们先从一个几乎所有项目都会经历的场景说起。项目刚开始的时候，配置一般只有很少的几个 key，此时通常在业务代码使用简单 API 直接读取 key 的值。这个阶段，把配置理解成 `map[string]string` 没有什么问题。

但是当应用再复杂一点时，这个理解就不适合了。因为此时配置可能来自 YAML/TOML/JSON 文件、环境变量、命令行参数等多种来源。值也不再只是单纯的字符串，而是结构体、数组、Map。通常还需要默认值、变量引用、动态刷新等高级能力。这时候如果还把配置当成一组散落的 key-value，那么对配置的绑定、合并和刷新都会变得很难统一。

所以，Go-Spring 的第一步，是把所有的配置都收拢到同一个抽象里，即 `Properties`。

## Properties

无论原始配置是什么格式，Go-Spring 最终都会把它转换成扁平化的 key-value 结构，也就是 `Properties`。

这个设计的关键不在于“扁平化”本身，而在于统一访问接口。因为只有访问接口统一了，上层的配置绑定才不需要关心配置的最初来源，不管配置来自 YAML 文件、命令行参数还是环境变量，只要输入能转换成同一套配置路径和值，就能被统一处理。

Go-Spring 的 key 匹配是大小写敏感的，没有松散绑定那样的功能。配置和获取用的 key 是什么字符串，匹配时就用什么字符串。

## Path 语法

有了统一的 `Properties` 模型，我们还需要一套统一的规则来表达嵌套结构。Go-Spring 使用广泛流行的 Path 语法来定位配置项。

- 使用点号 `.` 表示分隔嵌套层级，例如 `a.b.c` 表示 `a -> b -> c`。
- 使用方括号 `[index]` 表示数组索引，例如 `a.b[0].c` 表示 `a.b` 数组第一个元素的 `c` 字段。

下面这段 YAML 同时包含了对象和数组：

```yaml
app:
  port: 8080
  database:
    - host: localhost
      port: 5432
    - host: repli.ca
      port: 5433
```

转换成 `Properties` 后如下：

```properties
app.port = 8080
app.database[0].host = localhost
app.database[0].port = 5432
app.database[1].host = repli.ca
app.database[1].port = 5433
```

可以看到，对象层级、数组元素和叶子值，都可以用同一套路径规则表达，每一个配置值都有了唯一的 Path。

## 扁平化的配置树

从表面上看，`Properties` 是扁平化的 key-value。但如果只停在这个表面，就会漏掉它真正的作用。我们真正得到的其实是一棵可以用 path 访问的配置树。

这棵配置树会贯穿后续所有的能力：

- 结构体绑定会根据 path 找到对应字段。
- slice、array 和 map 绑定会依赖数组下标与子路径。
- 多来源合并会根据 path 判断覆盖、合并或整体替换。
- 变量引用会从同一套路径空间中查找其他配置项。

所以说 `Properties` 是理解 Go-Spring 整个配置系统的基础。
