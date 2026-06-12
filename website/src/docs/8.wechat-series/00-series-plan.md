# go-spring 公众号系列拆分规划

本文档用于指导后续把 `docs/2.guides` 中的内容拆分并改写为公众号系列文章。

当前规划为 **30 篇能力主题文章 + 1 篇整体说明文章**，共 31 篇。系列定位不是零基础入门文，而是面向已经熟悉 Go 工程开发、希望系统理解 go-spring 能力边界和设计取舍的读者。

`docs/2.guides/08-http-gen.md` 暂不纳入本系列。

## 写作原则

1. 尽量复用 `docs/2.guides` 的原文、示例代码和术语，只做必要润色。
2. 每篇文章只围绕一个明确主题展开，避免把多个能力点聚合成“大章”。
3. 保持工程化表达：开头从真实工程场景切入，再讲 go-spring 的抽象、典型代码和适用边界。
4. 不写小白教程，不大量解释 Go 基础语法、HTTP 基础概念、依赖注入基础概念。
5. 每篇文章需要有清楚的边界：写什么、不写什么、从哪些 guides 章节取材。
6. 文章中 API 名称、配置项、代码示例优先保持 guides 原文一致。
7. 可以补充承上启下文字，使公众号阅读更连贯，但不要改变 guides 的技术含义。
8. 不使用“本篇要解决的问题”这类提纲式开头；结尾也不要只写边界和下一篇预告，要先完成本篇主题收束。

## 系列结构

整体分为八个板块：

1. 配置系统：第 1-8 篇
2. IoC 容器：第 9-16 篇
3. 应用运行时：第 17-18 篇
4. 日志系统：第 19-26 篇
5. HTTP Server：第 27 篇
6. 组件与 Starter：第 28 篇
7. 测试与 Mock：第 29-30 篇
8. 整体说明：第 31 篇

## 文章拆分明细

### 01. 配置系统的设计模型

来源范围：

- `docs/2.guides/01-configuration.md:1-47`

文章目标：

- 介绍 go-spring 配置系统的基础抽象。
- 讲清楚 `Properties`、Path 语法和配置树模型。
- 让读者理解 go-spring 的配置不是简单的 `map[string]string`。

主要内容：

- `Properties` 的作用。
- 点分路径、数组下标、嵌套配置路径的表达方式。
- Path 语法在后续绑定、校验、变量引用中的基础地位。

写作边界：

- 不展开配置绑定细节，只说明配置树如何表达数据。
- 不展开多来源加载和优先级，留到后续文章。

### 02. 配置绑定的核心机制

来源范围：

- `docs/2.guides/01-configuration.md:48-87`
- `docs/2.guides/01-configuration.md:246-283`

文章目标：

- 说明配置如何绑定到 Go 结构体。
- 介绍结构体标签绑定和手动 `Bind` 两种使用方式。

主要内容：

- 结构体标签绑定。
- 基础字段映射方式。
- 手动 `Bind` 函数绑定。
- 自动绑定与手动绑定的适用场景。

写作边界：

- 复杂类型、类型转换器、slice、map 单独放到第 3 篇。
- 校验不在这一篇展开。

### 03. 复杂类型绑定

来源范围：

- `docs/2.guides/01-configuration.md:88-245`

文章目标：

- 展开配置绑定中的类型系统。
- 说明 go-spring 如何处理基础类型以外的配置结构。

主要内容：

- 基础类型绑定。
- 内置特殊转换器。
- 自定义类型转换器。
- slice/array 绑定。
- map 绑定。

写作边界：

- 不讨论配置来源。
- 不讨论优先级合并语义，只讨论绑定时的数据结构表达。

### 04. 配置校验

来源范围：

- `docs/2.guides/01-configuration.md:284-400`

文章目标：

- 说明 go-spring 配置校验的能力和边界。
- 强调配置校验不是简单的 required 检查。

主要内容：

- 表达式校验。
- 必填校验的误区。
- 自定义校验函数。
- 配置校验在启动阶段的意义。

写作边界：

- 不展开配置加载顺序。
- 不展开动态配置刷新后的校验策略。

### 05. 配置来源与格式扩展

来源范围：

- `docs/2.guides/01-configuration.md:401-598`

文章目标：

- 说明配置从哪里来，以及如何扩展配置输入。
- 重点介绍格式解析器和配置提供者两个扩展点。

主要内容：

- 支持的配置格式。
- 自定义配置格式解析器。
- 支持的配置来源。
- 自定义配置提供者。
- 环境变量。
- 命令行参数。

写作边界：

- 这一篇讲来源，不讲不同来源之间的优先级细节。
- 优先级和合并规则放到第 6 篇。

### 06. 配置优先级与合并语义

来源范围：

- `docs/2.guides/01-configuration.md:599-690`

文章目标：

- 说明多来源配置叠加时的确定性规则。
- 重点讲清楚不同数据类型的合并语义。

主要内容：

- 层次配置。
- 配置优先级。
- scalar、map、slice 的不同合并规则。
- 基础配置与环境覆盖的典型场景。

写作边界：

- 不讨论 Profile 激活方式，留到第 7 篇。
- 不讨论变量引用，留到第 8 篇。

### 07. Profile 与多环境配置

来源范围：

- `docs/2.guides/01-configuration.md:691-781`

文章目标：

- 讲清楚 go-spring 如何处理多环境配置。
- 强调 Profile 应该保持正交性，而不是把环境逻辑写乱。

主要内容：

- 激活 Profile。
- 配置文件命名约定。
- 自定义配置目录。
- 多个 Profile 的优先级。
- Profile 设计建议。

写作边界：

- 不展开 IoC 条件注册细节；Profile 名称与 Bean 装配的关系在本文收束，具体 `.OnProfiles()` 用法放到第 15 篇。

### 08. 配置编排与动态刷新

来源范围：

- `docs/2.guides/01-configuration.md:782-886`
- `docs/2.guides/03-app-start-stop.md:519-549`

文章目标：

- 收束配置系统的高级使用能力。
- 说明配置如何复用、引用和在运行期刷新。

主要内容：

- 配置导入。
- `optional:` 导入语义。
- 变量引用。
- 嵌套引用。
- 动态配置刷新。

写作边界：

- 动态配置只讲配置系统视角。
- 应用运行期扩展在第 18 篇再次串联。

### 09. IoC 容器的工程定位

来源范围：

- `docs/2.guides/02-ioc-container.md:1-163`

文章目标：

- 解释 go-spring 为什么需要 IoC 容器。
- 建立后续 Bean、注册、条件装配等文章的共同语境。

主要内容：

- 什么是依赖注入。
- 为什么需要 IoC 容器。
- 快速开始。
- Bean 定义。

写作边界：

- 不写成 DI 入门科普。
- 注入方式只概览，详细内容放到第 10 篇。

### 10. Bean 注入方式

来源范围：

- `docs/2.guides/02-ioc-container.md:164-226`

文章目标：

- 说明 go-spring 中 Bean 注入的两种主要形态。
- 重点解释字段注入和构造函数注入的取舍。

主要内容：

- 结构体字段注入。
- 构造函数参数注入。
- 两种注入方式的选择建议。

写作边界：

- 不展开注入目标的复杂匹配。
- 单 Bean、多 Bean、配置项注入放到第 11 篇。

### 11. Bean 注入目标

来源范围：

- `docs/2.guides/02-ioc-container.md:227-515`

文章目标：

- 说明容器如何决定把哪个值注入到目标位置。
- 覆盖单 Bean、多 Bean、配置项和延迟注入这些不同注入目标。

主要内容：

- 注入单个 Bean。
- 按类型注入。
- 按名称注入。
- 接口注入。
- 注入多个 Bean。
- 通过配置项注入。
- 延迟注入。

写作边界：

- 主题是“注入目标”，不是配置系统原理。
- 不讨论 Bean 创建阶段，后续第 16 篇讲运行流程。

### 12. Bean 类型与构造函数绑定

来源范围：

- `docs/2.guides/02-ioc-container.md:516-878`

文章目标：

- 说明 go-spring 支持哪些形态的 Bean。
- 深入讲构造函数 Bean 的参数绑定规则。

主要内容：

- 结构体指针。
- 构造函数。
- 构造函数参数绑定。
- 参数顺序。
- 函数指针。
- 不同 Bean 类型的适用场景。

写作边界：

- 不重复讲 Bean 注册 API。
- 不展开生命周期回调。

### 13. Bean 元信息配置

来源范围：

- `docs/2.guides/02-ioc-container.md:879-1160`

文章目标：

- 说明 Bean 除了类型和值之外，还可以携带哪些元信息。
- 重点讲生命周期、接口导出和依赖关系。

主要内容：

- 设置 Bean 名称。
- 设置生命周期回调。
- 通过函数指针设置回调。
- 通过方法名指定回调。
- 导出为接口。
- 附加激活条件。
- 显式依赖声明。
- 标记为根 Bean。

写作边界：

- 条件注册只做引出，详细机制放到第 15 篇。

### 14. Bean 注册 API

来源范围：

- `docs/2.guides/02-ioc-container.md:1161-1386`

文章目标：

- 系统说明 go-spring 提供的 Bean 注册入口。
- 讲清楚不同注册 API 的使用边界。

主要内容：

- `gs.Provide()`。
- `gs.Module()`。
- `gs.Group()`。
- `Configuration`。
- `app.Provide()`。
- Provide、Module、Group 的适用差异。

写作边界：

- 不深入 Starter 机制。
- Starter 单独放到第 28 篇。

### 15. 条件注册机制

来源范围：

- `docs/2.guides/02-ioc-container.md:1387-1574`

文章目标：

- 说明条件注册如何让模块根据环境和上下文装配。
- 重点讲条件表达能力，而不是只罗列 API。

主要内容：

- 属性条件。
- Bean 存在条件。
- 自定义函数条件。
- 组合条件。
- Profile 条件。
- `OnOnce` 缓存条件结果。

写作边界：

- Profile 条件作为条件注册的专用形式在本篇收束。

### 16. IoC 容器运行流程

来源范围：

- `docs/2.guides/02-ioc-container.md:1592-1698`

文章目标：

- 从运行阶段解释容器内部工作顺序。
- 让读者理解注册、解析、注入、运行、关闭之间的关系。

主要内容：

- 注册阶段。
- 解析阶段。
- 注入阶段。
- 运行阶段。
- 关闭阶段。

写作边界：

- 设计取舍已经分散收束到接口导出、运行流程和后续能力地图中，不再单独开 FAQ 篇。

### 17. App 运行模型

来源范围：

- `docs/2.guides/03-app-start-stop.md:1-88`
- `docs/2.guides/03-app-start-stop.md:178-402`
- `docs/2.guides/03-app-start-stop.md:404-491`

文章目标：

- 说明 go-spring App 从入口、启动、Ready、运行期到关闭的完整模型。
- 把配置、日志、IoC 容器、Runner、Server、Ready 和关闭链路放进同一条生命周期主线。

主要内容：

- 阻塞启动。
- 非阻塞启动。
- 启动前配置窗口。
- 启动链路。
- 内置 Bean 与容器入口。
- Runner 的生命周期位置。
- Server 的生命周期位置。
- Ready 信号。
- 运行期边界。
- 关闭链路。

写作边界：

- 不重复配置系统和 IoC 的底层细节。
- 不把 App API 写成清单；具体使用和定制能力放到第 18 篇。

### 18. App 使用与定制

来源范围：

- `docs/2.guides/03-app-start-stop.md:8-176`
- `docs/2.guides/03-app-start-stop.md:404-549`
- `docs/2.guides/01-configuration.md:843-886`

文章目标：

- 说明 go-spring App 的主要使用入口和定制能力。
- 把启动前配置、Runner、Server、root context 和动态配置放回第 17 篇的生命周期位置里解释。

主要内容：

- 选择启动入口。
- 启动前定制。
- 关闭内置 HTTP Server。
- 设置代码默认配置。
- 注册当前 App 的 Bean。
- 指定 root bean。
- 自定义 Banner。
- 实现 Runner。
- 实现 Server。
- 使用 root context。
- 刷新动态配置。
- 关闭策略的业务边界。

写作边界：

- 不重复完整 App 生命周期主线。
- 不展开 HTTP 路由细节。
- HTTP Server 单独在第 27 篇处理。

### 19. 日志系统架构

来源范围：

- `docs/2.guides/04-logging.md:1-361`

文章目标：

- 建立 go-spring 日志系统的整体模型。
- 说明日志 API、Tag、Logger、Appender、Layout、Encoder 和 Field 怎样组成完整管线。

主要内容：

- 日志调用入口。
- Tag 路由。
- Logger 过滤和调度。
- Appender 输出。
- Layout 与 Encoder 编码。
- Field 与上下文字段。
- 配置拓扑。
- 一条日志的完整处理流程。

写作边界：

- 只讲整体关系，不展开各组件的详细 API 和配置。
- 不在架构篇混入扩展和第三方适配。

### 20. 结构化日志

来源范围：

- `docs/2.guides/04-logging.md:362-499`

文章目标：

- 说明业务代码怎样通过 API 和 Field 描述一条日志。
- 讲清楚格式化日志、结构化字段和 Event 之间的关系。

主要内容：

- 日志 API 与级别选择。
- 格式化接口和结构化接口。
- 基础类型、指针类型、数组和嵌套对象 Field。
- `Msg`、`Any` 和 `FieldsFromMap`。
- `Trace`、`Debug` 的惰性构造。
- Event 的组成和作用。

写作边界：

- 不讲日志输出目的地。
- 不展开 Logger 的级别范围和调度方式。
- 不讲上下文提取。

### 21. 日志路由与调度

来源范围：

- `docs/2.guides/04-logging.md:500-800`

文章目标：

- 说明 Tag 怎样选择 Logger，以及 Logger 怎样完成级别过滤、同步异步调度和日志分发。
- 讲清楚路由规则、Root Logger 和 AppenderRef 的关系。

主要内容：

- Tag 注册与命名。
- 精确匹配、后缀通配和 Root 回退。
- LevelRange 和自定义 Level。
- SyncLogger。
- AsyncLogger。
- AppenderRef。
- ConsoleLogger、FileLogger 和 RollingFileLogger。

写作边界：

- 不展开 Appender、Layout、Encoder。
- 不展开自定义 Logger，自定义组件统一放到第 25 篇。

### 22. 日志输出与格式化

来源范围：

- `docs/2.guides/04-logging.md:801-1114`

文章目标：

- 说明 Event 怎样从 Logger 进入输出目标并转换成最终字节。
- 展开 Appender、Layout、Encoder 的职责划分。

主要内容：

- DiscardAppender。
- ConsoleAppender。
- FileAppender。
- RollingFileAppender。
- TextLayout。
- JSONLayout。
- Encoder。

写作边界：

- 不展开上下文提取。
- 不展开配置系统中的插件注入。
- 不展开自定义 Appender 和 Layout，第 25 篇统一处理。

### 23. 日志上下文提取

来源范围：

- `docs/2.guides/04-logging.md:1115-1262`

文章目标：

- 说明日志如何从 `context.Context` 中提取业务字段和链路信息。
- 重点讲上下文提取与观测体系的连接方式。

主要内容：

- `FieldsFromContext`。
- 基础使用示例。
- 与 OpenTelemetry 集成。
- `StringFromContext`。
- 上下文提取的性能注意事项。

写作边界：

- 不展开日志配置项。
- 不展开标准库和第三方日志适配。

### 24. 日志配置

来源范围：

- `docs/2.guides/04-logging.md:1263-1550`

文章目标：

- 通过完整示例说明怎样配置一条日志处理链路。
- 沿着日志流讲清楚配置节点怎样连接成完整链路。

主要内容：

- 一份包含 Root Logger 和业务 Logger 的完整配置。
- 配置节点、实例名和 `type`。
- Tag、Logger、AppenderRef、Appender 和 Layout 的连接关系。
- 日志级别与数组配置规则。
- 属性引用。
- 配置刷新。

写作边界：

- 主题是怎样用配置组织完整的日志链路。
- 完整配置及其解析是文章主体，属性引用和刷新只作为补充能力。
- 不展开插件创建、自定义组件和生态适配，第 25-26 篇单独处理。

### 25. 日志扩展

来源范围：

- `docs/2.guides/04-logging.md:700-1105`
- `docs/2.guides/04-logging.md:1360-1545`

文章目标：

- 说明自定义能力应该落在 Logger、Appender、Layout 还是 Encoder。
- 讲清楚插件注册、配置注入、Event 所有权和生命周期边界。

主要内容：

- 扩展位置选择。
- 自定义 Layout。
- 自定义 Appender。
- 自定义 Logger。
- `RegisterPlugin`。
- `PluginAttribute` 与 `PluginElement`。
- `RegisterConverter`。
- Event 所有权和并发约束。
- 刷新与生命周期。

写作边界：

- 主题是扩展已有日志管线，不重复内置组件用法。
- 不展开旧日志入口和第三方框架适配。

### 26. 日志适配

来源范围：

- `docs/2.guides/04-logging.md:1550-1812`

文章目标：

- 说明已有日志入口如何进入 Go-Spring 统一日志管线。
- 讲清楚字节转发和事件转换两种适配深度。

主要内容：

- RawBytes 与结构化事件转换。
- `GetLogger`。
- 适配标准库 `log`。
- 适配 `log/slog`。
- 适配 Zap。
- 级别、字段、调用位置和 Panic/Fatal 语义差异。
- 旧入口和新日志 API 的迁移边界。

写作边界：

- 主题是兼容、迁移和生态适配。
- 不再重复 Logger、Appender、Layout 的基本概念。
- 不把适配器写成第三方日志库的完整使用教程。

### 27. 内置 HTTP Server

来源范围：

- `docs/2.guides/05-http-server.md:1-300`

文章目标：

- 单独介绍 go-spring 内置 HTTP Server 能力。
- 讲清楚 Server 配置、路由机制、第三方路由集成和生命周期。

主要内容：

- 快速开始。
- HTTP Server 配置项。
- 路由机制。
- 集成 Gin。
- 集成 gorilla/mux。
- 集成 chi。
- HTTP Server 生命周期。

写作边界：

- 不把组件和测试混进这一篇。
- 主题只围绕 HTTP Server 展开。

### 28. 组件与 Starter 机制

来源范围：

- `docs/2.guides/06-components.md:1-177`
- `docs/2.guides/02-ioc-container.md:1161-1386`

文章目标：

- 单独介绍组件和 Starter 如何封装 go-spring 能力。
- 解释 Provide、Module、Group 在组件化场景下的意义。

主要内容：

- 组件核心机制。
- Provide：注册单个 Bean。
- Module：按条件动态注册。
- Group：注册多个同类型实例。
- 自定义 Starter。
- 官方 Starter。
- 组件与普通 Bean 注册的关系。

写作边界：

- 不展开 HTTP Server 的路由细节。
- 不展开测试体系。

### 29. 测试体系

来源范围：

- `docs/2.guides/07-testing.md:1-214`
- `docs/2.guides/02-ioc-container.md:1819-1827`

文章目标：

- 单独介绍 go-spring 项目中的测试方式。
- 讲清楚纯单测、IoC 容器测试、测试配置、替代 Bean 和断言库的使用边界。

主要内容：

- 纯单元测试。
- 基于 IoC 容器的测试。
- 自定义配置。
- 替换依赖。
- 测试隔离性。
- assert 与 require。
- 断言库基础用法。
- 自定义错误消息。

写作边界：

- 不混入 HTTP Server 和组件机制。
- 不展开 Mock 细节，Mock 单独放到第 30 篇。

### 30. Mock 边界

来源范围：

- `docs/2.guides/07-testing.md:215-403`

文章目标：

- 单独介绍 `gs-mock` 的使用边界。
- 讲清楚接口 Mock、函数 Mock、方法 Mock 和编译边界。

主要内容：

- Mock 的适用场景。
- 接口 Mock 与代码生成。
- `Handle` 规则。
- `When`/`Return` 规则。
- 函数和方法 Mock。
- `context.Context` 与 Mock Manager。
- 禁用内联参数。
- Mock 使用提示。

写作边界：

- 不重复测试分层和 `RunTest` 用法。
- 不鼓励把 Mock 作为默认测试方式。

### 31. go-spring 能力地图

来源范围：

- 前 30 篇文章。
- `docs/2.guides` 的整体结构。

文章目标：

- 作为系列整体说明和收束文章。
- 不再展开具体 API，而是解释 go-spring 的能力如何形成一个整体。

主要内容：

- 配置系统负责外部化和环境差异。
- IoC 容器负责对象装配和条件化。
- 应用运行时负责启动、退出和运行期管理。
- 日志系统负责结构化观测和生态适配。
- HTTP Server 负责 Web 服务接入。
- 组件与 Starter 负责能力封装。
- 测试体系和 Mock 负责工程验证与外部边界隔离。
- go-spring 的设计边界：不是简单工具集合，而是围绕应用生命周期组织能力。

写作边界：

- 不新增技术细节。
- 不写成项目宣传稿。
- 重点是帮助读者回看整个系列的结构。

## 后续落文建议

建议后续文章文件按如下方式命名：

```text
docs/8.wechat-series/
  00-series-plan.md
  01-configuration-model.md
  02-configuration-binding.md
  03-configuration-complex-types.md
  04-configuration-validation.md
  05-configuration-sources.md
  06-configuration-priority.md
  07-configuration-profile.md
  08-configuration-refresh.md
  09-ioc-positioning.md
  ...
  25-logging-extension.md
  26-logging-adapters.md
  27-http-server.md
  28-components-starter.md
  29-testing.md
  30-mock.md
  31-go-spring-capability-map.md
```

每篇文章建议采用相对稳定但不生硬的结构：

1. 标题下直接进入两到三段场景化导语，先把工程问题说清楚。
2. 主体内容基于 guides 原文整理和润色，按关键概念、API、配置项或流程拆分。
3. 示例代码保留原有技术含义，避免为了文章感强行改写 API。
4. 设计边界或使用建议可以作为正文最后一个技术小节，也可以融入收束段。
5. 结尾先回到当前主题，说明这块能力在 go-spring 里的位置和价值。
6. 如果需要衔接下一篇，用自然过渡句带出，不写成单独的“下一篇预告”。
