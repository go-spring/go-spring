# Go-Spring 实战第 1 课 —— 统一配置模型：Properties 与 Path

我们先从一个几乎所有项目都会遇到的场景说起。项目刚起步时，配置通常只有少量 key，业务代码往往直接通过简单 API 读取对应的值。这个阶段，把配置理解成 `map[string]string` 并没有问题。

但应用稍微复杂后，这种理解就不够用了。配置来源可能包括 YAML/TOML/JSON 文件、环境变量、命令行参数等；值也不再只是字符串，而可能是结构体、数组、Map。与此同时，还会出现默认值、变量引用、动态刷新等需求。如果仍然把配置看成一组散落的 key-value，绑定、合并和刷新就很难建立统一的处理逻辑。

因此，Go-Spring 配置系统的第一步，就是把所有配置收拢到同一个抽象：`Properties`。

## Properties

无论原始配置采用哪种格式，Go-Spring 最终都会把它转换成扁平化的 key-value 结构，也就是 `Properties`。

这个设计的关键不在于把数据压平成 key-value，而在于提供统一的访问接口。只有接口统一，上层配置绑定才不需要关心最初来源：不管输入来自 YAML 文件、命令行参数还是环境变量，只要能转换成同一套路径和值，就可以按相同方式处理。

Go-Spring 的 key 匹配区分大小写，不提供类似松散绑定的能力。写入和读取使用什么 key，匹配时就严格使用什么 key。

## Path 语法

有了统一的 `Properties` 模型，还需要一套规则来表达嵌套结构。Go-Spring 使用常见的 Path 语法来定位配置项。

- 使用点号 `.` 分隔嵌套层级，例如 `a.b.c` 表示 `a -> b -> c`。
- 使用方括号 `[index]` 表示数组下标，例如 `a.b[0].c` 表示 `a.b` 数组第一个元素的 `c` 字段。

下面这段 YAML 同时包含对象和数组：

```yaml
app:
  port: 8080
  database:
    - host: localhost
      port: 5432
    - host: repli.ca
      port: 5433
```

转换为 `Properties` 后如下：

```properties
app.port = 8080
app.database[0].host = localhost
app.database[0].port = 5432
app.database[1].host = repli.ca
app.database[1].port = 5433
```

可以看到，对象层级、数组元素和叶子值都可以用同一套路径规则表达，每个配置值都有唯一的 Path。

## 扁平化的配置树

从表面上看，`Properties` 是扁平化的 key-value。但如果只停在这个层面，就会忽略它真正的作用：我们得到的其实是一棵可以用 path 访问的配置树。

这棵配置树会贯穿后续的所有能力：

- 结构体绑定会根据 path 找到对应字段。
- slice、array 和 map 绑定会依赖数组下标与子路径。
- 多来源合并会根据 path 判断覆盖、合并或整体替换。
- 变量引用会从同一套路径空间中查找其他配置项。

因此，`Properties` 是理解 Go-Spring 整个配置系统的基础。
