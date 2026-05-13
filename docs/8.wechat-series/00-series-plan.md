# go-spring 公众号系列拆分规划

本文档用于指导后续把 `docs/2.guides` 中的内容拆分并改写为公众号系列文章。

当前规划为 **29 篇能力主题文章 + 1 篇整体说明文章**，共 30 篇。系列定位不是零基础入门文，而是面向已经熟悉 Go 工程开发、希望系统理解 go-spring 能力边界和设计取舍的读者。

`docs/2.guides/08-http-gen.md` 暂不纳入本系列。

## 写作原则

1. 尽量复用 `docs/2.guides` 的原文、示例代码和术语，只做必要润色。
2. 每篇文章只围绕一个明确主题展开，避免把多个能力点聚合成“大章”。
3. 保持工程化表达：先讲问题域，再讲 go-spring 的抽象，再给出典型代码和适用边界。
4. 不写小白教程，不大量解释 Go 基础语法、HTTP 基础概念、依赖注入基础概念。
5. 每篇文章需要有清楚的边界：写什么、不写什么、从哪些 guides 章节取材。
6. 文章中 API 名称、配置项、代码示例优先保持 guides 原文一致。
7. 可以补充少量承上启下文字，使公众号阅读更连贯，但不要改变 guides 的技术含义。

## 系列结构

整体分为八个板块：

1. 配置系统：第 1-8 篇
2. IoC 容器：第 9-18 篇
3. 应用运行时：第 19-20 篇
4. 日志系统：第 21-26 篇
5. HTTP Server：第 27 篇
6. 组件与 Starter：第 28 篇
7. 测试体系：第 29 篇
8. 整体说明：第 30 篇

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
- 校验不在本篇展开。

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

- 本篇讲来源，不讲不同来源之间的优先级细节。
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

- 不讨论 IoC 条件注册中的 Profile 条件，后续第 16 篇处理。

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
- 应用运行期扩展在第 20 篇再次串联。

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

- 本篇主题是“注入目标”，不是配置系统原理。
- 不讨论 Bean 创建阶段，后续第 17 篇讲运行流程。

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
- `OnOnce` 缓存条件结果。

写作边界：

- Profile 条件单独放到第 16 篇。

### 16. Profile 条件与装配边界

来源范围：

- `docs/2.guides/02-ioc-container.md:1575-1591`
- `docs/2.guides/01-configuration.md:691-781`

文章目标：

- 讲清楚 Profile 不只是配置文件选择，也会影响 Bean 装配。
- 把配置 Profile 和 IoC Profile 条件放在同一条语义线上说明。

主要内容：

- Profile 条件。
- Profile 与配置系统的关系。
- Profile 与 Bean 条件注册的关系。
- 多环境下的装配边界。

写作边界：

- 不展开 Starter 机制。
- 不重复第 7 篇 Profile 配置细节，只引用必要背景。

### 17. IoC 容器运行流程

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

- 不深入 FAQ 和设计取舍，第 18 篇专门讨论。

### 18. IoC 设计取舍

来源范围：

- `docs/2.guides/02-ioc-container.md:1699-1947`

文章目标：

- 解释 go-spring IoC 容器背后的关键设计选择。
- 用 FAQ 的形式处理读者可能最关心的争议点。

主要内容：

- 接口分离。
- 按需创建。
- 循环依赖。
- 销毁顺序。
- 为什么不提供 `getBean()`。
- 为什么接口需要显式导出。
- 是否支持原型模式。
- 运行时反射开销。
- 为什么不用自动包扫描。
- 和 Wire 的差异。
- root bean 的意义。

写作边界：

- 不继续展开测试用法，测试放到第 29 篇。

### 19. 应用启动机制

来源范围：

- `docs/2.guides/03-app-start-stop.md:1-362`

文章目标：

- 说明 go-spring 应用从启动入口到服务可用的完整机制。
- 把启动方式、启动配置和启动流程放在一篇文章中形成闭环。

主要内容：

- 阻塞启动。
- 非阻塞启动。
- 禁用内置 HTTP Server。
- 设置默认配置。
- 注册容器 Bean。
- 注册 Root Bean。
- 自定义 Banner。
- 配置加载。
- 初始化日志。
- 初始化 IoC 容器。
- 执行 Runner。
- 启动 Server。

写作边界：

- 不重复配置系统和 IoC 的底层细节。
- 退出、优雅关闭和运行期扩展放到第 20 篇。

### 20. 应用退出与运行期扩展

来源范围：

- `docs/2.guides/03-app-start-stop.md:363-549`
- `docs/2.guides/01-configuration.md:843-886`

文章目标：

- 说明应用启动之后如何被管理。
- 覆盖退出、优雅关闭、Runner、Server、Root Context 和动态刷新。

主要内容：

- 监听退出信号。
- 优雅关闭。
- 实现 Runner。
- 实现 Server。
- 注入根 Context。
- 刷新动态配置。

写作边界：

- 不展开 HTTP 路由细节。
- HTTP Server 单独在第 27 篇处理。

### 21. 日志系统架构

来源范围：

- `docs/2.guides/04-logging.md:1-361`

文章目标：

- 建立 go-spring 日志系统的整体模型。
- 说明标签、级别、输出这些基础概念如何组合。

主要内容：

- 快速开始。
- 核心组件。
- 标签系统。
- 标签命名规范。
- 标签注册。
- 标签路由。
- 日志级别。
- 自定义级别。
- 日志输出。
- 惰性求值。
- 调整堆栈深度。

写作边界：

- 不展开结构化字段模型。
- 不展开 Logger/Appender/Layout 的实现细节。

### 22. 结构化日志

来源范围：

- `docs/2.guides/04-logging.md:362-499`

文章目标：

- 说明 go-spring 如何表达结构化日志字段。
- 重点讲字段类型、嵌套结构和自动推断。

主要内容：

- 基础类型。
- 指针类型。
- 消息字段。
- 数组和嵌套对象。
- map 展开。
- 自动类型推断。

写作边界：

- 不讲日志输出目的地。
- 不讲上下文提取。

### 23. Logger 体系

来源范围：

- `docs/2.guides/04-logging.md:500-800`

文章目标：

- 介绍 go-spring 日志系统中 Logger 层的设计。
- 说明同步、异步、控制台、文件、滚动文件、自定义 Logger 的差异。

主要内容：

- SyncLogger。
- AsyncLogger。
- ConsoleLogger。
- FileLogger。
- RollingFileLogger。
- 自定义 Logger。

写作边界：

- Appender、Layout、Encoder 放到第 24 篇。

### 24. 日志输出管线

来源范围：

- `docs/2.guides/04-logging.md:801-1114`

文章目标：

- 说明一条日志从 Logger 到最终输出的管线。
- 展开 Appender、Layout、Encoder 的职责划分。

主要内容：

- DiscardAppender。
- ConsoleAppender。
- FileAppender。
- RollingFileAppender。
- 自定义 Appender。
- TextLayout。
- JSONLayout。
- 自定义 Layout 扩展。
- Encoder。

写作边界：

- 不展开上下文提取。
- 不展开配置系统中的插件注入。

### 25. 日志上下文提取

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
- 不展开标准库和 Zap 适配。

### 26. 日志配置、刷新与生态适配

来源范围：

- `docs/2.guides/04-logging.md:1263-1812`

文章目标：

- 收束日志系统的工程治理能力。
- 说明日志如何通过配置驱动，并接入标准库和 Zap 等生态。

主要内容：

- 日志配置分类。
- 日志级别配置。
- 数组配置。
- 插件注入。
- 生命周期管理。
- 类型转换器。
- 错误处理。
- 配置刷新。
- `GetLogger`。
- 适配标准库 `log`。
- 适配 Zap。

写作边界：

- 本篇主题是日志系统的配置治理和生态适配。
- 不再重复 Logger、Appender、Layout 的基本概念。

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

- 不把组件和测试混进本篇。
- 本篇只围绕 HTTP Server 展开。

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

- `docs/2.guides/07-testing.md:1-403`
- `docs/2.guides/02-ioc-container.md:1819-1827`

文章目标：

- 单独介绍 go-spring 项目中的测试方式。
- 讲清楚纯单测、IoC 容器测试、断言库和 Mock 框架的使用边界。

主要内容：

- 纯单元测试。
- 基于 IoC 容器的测试。
- 自定义配置。
- 替换依赖。
- 测试隔离性。
- assert 与 require。
- 断言库基础用法。
- 自定义错误消息。
- 接口 Mock。
- 函数和方法 Mock。
- Mock 使用提示。

写作边界：

- 不混入 HTTP Server 和组件机制。
- 只讨论测试体系及其与 IoC 的必要连接。

### 30. go-spring 能力地图

来源范围：

- 前 29 篇文章。
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
- 测试体系负责工程验证。
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
  27-http-server.md
  28-components-starter.md
  29-testing.md
  30-go-spring-capability-map.md
```

每篇文章可以采用固定结构：

1. 本篇要解决的问题。
2. 对应 guides 原文内容的整理和润色。
3. 关键 API 或配置项。
4. 典型示例代码。
5. 设计边界或使用建议。
6. 与下一篇文章的衔接。

