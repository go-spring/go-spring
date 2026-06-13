# Go-Spring 实战第 24 课 —— 日志配置：组织日志链路

前面几篇文章，我们已经分别介绍了 Tag、Logger、Appender、Layout 和上下文提取。到了实际项目里，这些组件最终要通过配置组织起来：哪类日志交给哪个 Logger，采用同步还是异步方式，写到哪里，又使用什么格式。

这一篇不再分别介绍组件，而是从一份完整配置出发，看看 Go-Spring 怎样把它们连接成可以运行的日志链路。

## 完整配置

假设应用有两类日志：

- 普通应用日志使用 INFO 级别，以文本格式输出到控制台。
- 业务日志匹配 `_biz_*` Tag，经过异步缓冲后，以 JSON 格式写入滚动文件。

对应配置如下：

```properties
# 默认日志：同步输出到控制台
logging.logger.root.type = ConsoleLogger
logging.logger.root.level = INFO
logging.logger.root.layout.type = TextLayout

# 业务日志的输出目标：滚动文件
logging.appender.bizFile.type = RollingFileAppender
logging.appender.bizFile.dir = ./logs
logging.appender.bizFile.file = biz.log
logging.appender.bizFile.interval = 1h
logging.appender.bizFile.maxAge = 168h
logging.appender.bizFile.layout.type = JSONLayout
logging.appender.bizFile.layout.fileLineMaxLength = 48

# 业务日志的处理方式：异步写入 bizFile
logging.logger.biz.type = AsyncLogger
logging.logger.biz.tag = _biz_*
logging.logger.biz.level = INFO
logging.logger.biz.bufferSize = 10000
logging.logger.biz.onBufferFull = discard
logging.logger.biz.appenderRef[0].ref = bizFile
```

这份配置最终形成两条链路：

```text
其他 Tag  → root Logger → ConsoleAppender → TextLayout

_biz_*   → biz Logger  → bizFile Appender → JSONLayout
              │
              └─ AsyncLogger 后台异步调度
```

`root` 使用的是集成式 `ConsoleLogger`，内部已经包含控制台 Appender，因此配置比较紧凑。`biz` 使用组合式 `AsyncLogger`，输出目标单独定义为 `bizFile`，两者通过 `appenderRef` 连接。

## 配置结构

Go-Spring App 的日志配置统一位于 `logging` 前缀下。应用启动时，框架会取出这个前缀下的配置，再交给日志库。因此，日志库实际看到的是 `logger.*` 和 `appender.*`。

如果单独使用 `go-spring.org/log`，配置可以直接从这两个前缀开始，不需要增加 `logging`。

### 实例与类型

下面两段配置分别定义了一个名为 `biz` 的 Logger 和一个名为 `bizFile` 的 Appender：

```properties
logging.logger.biz.type = AsyncLogger
logging.appender.bizFile.type = RollingFileAppender
```

它们都遵循同一个结构：

```text
logging.<组件类别>.<实例名>.<属性>
```

`logger` 和 `appender` 表示组件类别，`biz` 和 `bizFile` 是配置中的实例名，`type` 决定使用哪个具体实现。后面的其他属性，则由这个实现负责解释。

例如，`AsyncLogger` 会读取 `bufferSize` 和 `onBufferFull`，`RollingFileAppender` 会读取 `dir`、`file`、`interval` 和 `maxAge`。Layout 是 Appender 或集成式 Logger 的子组件，因此继续放在对应实例的 `layout.*` 下面。

```properties
logging.appender.bizFile.layout.type = JSONLayout
logging.appender.bizFile.layout.fileLineMaxLength = 48
```

这样，一段配置既描述了要创建什么组件，也描述了组件之间的包含关系。

### Tag 与级别

`biz` Logger 通过 `tag` 选择自己负责的日志：

```properties
logging.logger.biz.tag = _biz_*
logging.logger.biz.level = INFO
```

`_biz_order_create`、`_biz_user_login` 等 Tag 都会匹配 `_biz_*`。匹配到 `biz` 以后，Logger 再使用 `level` 判断日志是否继续处理。

`root` 是特殊的兜底 Logger，不需要配置 `tag`。没有匹配到其他 Logger 的 Tag，最终都会进入 `root`。

`level` 可以使用单个级别，也可以使用左闭右开的范围：

```properties
# INFO 及以上
logging.logger.root.level = INFO

# DEBUG 和 INFO，不包含 WARN
logging.logger.debug.level = DEBUG~WARN
```

单个 `INFO` 等价于 `[INFO, MAX)`。级别范围和 Tag 路由的详细规则已经在第 21 课介绍过，这里只需要记住：`tag` 决定哪类日志进入 Logger，`level` 决定其中哪些级别继续处理。

### AppenderRef

组合式 Logger 只负责过滤和调度，不直接决定日志写到哪里。下面这行配置把 `biz` Logger 连接到前面定义的 `bizFile` Appender：

```properties
logging.logger.biz.appenderRef[0].ref = bizFile
```

`ref` 的值必须与 `logging.appender.<实例名>` 中的实例名一致。如果引用的 Appender 不存在，日志配置会在初始化阶段失败。

一个 Logger 可以连接多个输出目标。假设配置中还定义了一个名为 `console` 的 Appender：

```properties
logging.logger.biz.appenderRef[0].ref = console
logging.logger.biz.appenderRef[1].ref = bizFile
logging.logger.biz.appenderRef[1].level = WARN~MAX
```

这表示业务日志都会输出到 `console`，其中 WARN 及以上级别还会写入 `bizFile`。Logger 的 `level` 过滤整条链路，AppenderRef 的 `level` 只过滤当前输出分支。

`appenderRef` 是对象数组，因此使用连续下标描述。简单字符串数组则既可以使用下标，也可以使用逗号分隔：

```properties
logging.logger.biz.tag = _biz_order_*,_biz_user_*
```

等价于：

```properties
logging.logger.biz.tag[0] = _biz_order_*
logging.logger.biz.tag[1] = _biz_user_*
```

到这里，完整配置的关系就清楚了：Logger 通过 `tag` 接住日志，通过 `level` 过滤日志，再通过 `appenderRef` 找到输出目标；Appender 最后使用自己的 Layout 生成输出内容。

## 属性引用

目录、级别和保留时间通常会随环境变化，可以使用 `${key}` 把这些值集中定义：

```properties
logging.values.dir = /var/log/myapp
logging.values.level = INFO
logging.values.maxAge = 168h

logging.appender.bizFile.dir = ${values.dir}
logging.appender.bizFile.maxAge = ${values.maxAge}
logging.logger.biz.level = ${values.level}
```

Go-Spring App 交给日志库的是 `logging` 下面的独立配置，所以被引用的 `values.*` 也要放在 `logging` 前缀下。不同 Profile、配置文件或命令行参数只需要覆盖这些值，不必重复整套日志结构。

普通属性会在注入组件前解析引用，但 `type` 不能使用属性引用。日志系统必须先读取 `type`，才能确定要创建哪种组件：

```properties
# 不支持
logging.logger.biz.type = ${values.loggerType}
```

## 配置刷新

日志刷新不是修改当前组件的几个字段，而是根据新配置重建整条日志链路。单独使用日志库时，可以调用：

```go
err := log.RefreshConfig(config)
```

刷新过程可以概括为：

1. 根据新配置创建 Logger、Appender 和 Layout。
2. 解析 `appenderRef`，检查引用和并发组合是否合法。
3. 先启动新 Appender，再启动新 Logger。
4. 将已有 Tag 和命名 Logger 绑定到新 Logger。
5. 停止旧 Logger 和旧 Appender。

如果创建、引用解析或启动失败，本次刷新会返回 error，已经启动的临时组件会被停止，当前正在使用的日志链路不会被替换。

Go-Spring App 会在启动阶段根据 `logging.*` 初始化日志。运行期调用 `PropertiesRefresher.RefreshProperties()` 只会刷新动态配置字段，不会自动重建日志链路。应用确实需要动态调整日志配置时，应当显式调用日志库的 `Refresh` 或 `RefreshConfig`。

刷新会逐个替换 Tag 和命名 Logger 内部的 Logger 引用，并不会让所有调用点在同一个瞬间完成事务式切换。因此，它适合低频配置调整，不应当被当作逐条日志无损切换机制。

日志配置的核心不是罗列每个组件的所有属性，而是描述一条完整关系：Tag 选择 Logger，Logger 通过 AppenderRef 连接输出目标，Appender 再使用 Layout 生成最终内容。掌握这条主线以后，新增 Logger、增加输出分支或者切换格式，都只是沿着同一套配置结构进行调整。
