# Go-Spring 实战内容计划（博客文章 + 公开视频课）

本文档用于规划 Go-Spring 实战内容生产。当前优先级分两步：

1. 先做成一组连续的博客文章列表，降低启动成本，快速验证主题、顺序和读者反馈。
2. 再把文章体系扩展成公开视频课，每篇文章基本对应一集视频，复用代码、讲义和验收命令。

内容设计参考了当前 `docs` 目录中的项目概览、快速开始、专题指南、示例项目和 starter 集成文档，目标是把文档能力转化为一套可以跟做、可以验收、可以沉淀代码仓库的实战学习路径。

## 一、内容定位

### 目标学员

- 有 Go 基础，能独立编写 HTTP API、结构体、接口、测试。
- 做过服务端项目，但在配置管理、依赖组织、启动关闭、组件集成、测试隔离等方面缺少统一工程方案。
- 希望理解 Go-Spring 的设计理念，并能把它用于真实业务服务。

### 读完文章或看完视频以后能做到

- 从零搭建一个 Go-Spring 服务，并理解 `gs.Run()` 背后的启动流程。
- 使用配置绑定、Profile、环境变量、命令行参数和动态配置管理应用行为。
- 用 IoC 容器组织 Controller、Service、DAO、SDK、Job、Server 等业务组件。
- 使用内置 HTTP Server 或接入 Gin、chi、gorilla/mux 等路由框架。
- 通过 Runner、Server、根 Context 和优雅关闭机制组织长期运行任务。
- 使用日志系统、测试工具、Mock、starter 和常见基础设施集成完成接近生产的服务开发。

### 内容风格

内容不按 API 字典展开，而是按“业务问题 -> 框架能力 -> 代码改造 -> 验收结果”的节奏推进。每篇文章都应有可运行代码、明确验收方式和一个小作业；后续录视频时，再补充现场演示、排错过程和阶段复盘。

## 二、发布路径

### 第一阶段：博客文章列表

博客阶段重点是把主题顺序和代码链路打磨清楚。每篇文章控制在一个核心问题内，尽量做到读者可以在 30-45 分钟内读完并跑通代码。

博客文章需要固定包含：

- 本篇要解决的业务问题。
- 关联的 Go-Spring 文档。
- 从上一阶段代码演进到本篇代码的关键步骤。
- 可复制的运行、请求、测试命令。
- 常见错误和排查建议。
- 一个小作业，用于引导读者自己改一次代码。

### 第二阶段：公开视频课

视频阶段不重新设计主题，而是基于博客文章扩展成公开视频课。视频重点补足文章不容易表达的部分：

- 从零敲代码的节奏。
- 真实报错和排查过程。
- 框架设计取舍的口头解释。
- 每个阶段的代码 diff 讲解。
- 观众评论中高频问题的补充答疑。

## 三、内容主线项目

建议以 `cn/3.examples/bookman` 为基础，设计一个“BookMan 进阶版”主线项目。它天然覆盖配置加载、Bean 注入、HTTP 路由、Controller、Service、DAO、SDK、动态配置、Runner、后台 Job 和测试，适合作为内容骨架。

主线项目可命名为 `BookMan Pro`，围绕一个图书管理服务逐步扩展：

- 第 1 阶段：只有标准库 HTTP 和内存数据。
- 第 2 阶段：引入 Go-Spring 管理启动、配置和依赖。
- 第 3 阶段：拆分 Controller、Service、DAO、SDK、Job。
- 第 4 阶段：加入动态配置、日志、测试和优雅关闭。
- 第 5 阶段：接入 MySQL、Redis、pprof，并补充自定义 Server 与多服务形态。
- 第 6 阶段：完成一个可作为模板复用的小型服务。

### 可选工程示例池

第一期内容建议只让 `BookMan Pro` 做主线，其他示例作为插曲、选做或后续独立文章使用。这样能保证读者一直在同一个工程里迭代，不会因为频繁切换业务背景而丢失上下文。

| 示例名称 | 示例定位 | 覆盖能力 | 适合出现位置 |
|----------|----------|----------|--------------|
| `BookMan Pro` | 主线 CRUD 服务 | 配置、IoC、HTTP、分层、日志、测试、动态配置、starter、外部资源 | 贯穿 01-12 |
| `ConfigLab` | 配置实验台 | 配置绑定、Profile、环境变量、命令行覆盖、动态刷新、无 Web 场景 | 02 的补充文章或视频片段 |
| `LifecycleLab` | 启停与后台任务实验 | `Runner`、`Server`、根 Context、优雅关闭、启动失败回滚 | 04 的补充文章或视频片段 |
| `MultiServer Echo` | 多服务形态示例 | HTTP、Gin、自定义 `gs.Server`、Ready 信号、优雅关闭 | 11 的主要工程扩展 |
| `CacheBook` | BookMan 的缓存增强版 | Redis starter、缓存穿透处理、配置开关、测试替换 | 10 的主要工程扩展 |
| `PersistBook` | BookMan 的持久化增强版 | GORM starter、DAO 替换、Profile 切换内存/数据库实现 | 10 的主要工程扩展 |
| `StarterLab` | 自定义 starter 示例 | `gs.Provide`、`gs.Module`、条件注册、资源释放、空白导入 | 09 的主要工程扩展 |
| `ChatStream` | SSE 流式接口示例 | 长连接、HTTP 超时配置、动态配置、前端静态页 | 独立番外文章 |

### 第一批推荐示例组合

第一批博客不要一次性铺开所有示例。更稳妥的组合是：

- 主线：`BookMan Pro`。
- 配置插曲：`ConfigLab`，用很小的程序把配置优先级和动态刷新讲透。
- 生命周期插曲：`LifecycleLab`，专门演示 Runner、Server 和优雅关闭。
- 组件化插曲：`StarterLab`，把 BookMan 里的访问日志或价格 SDK 封装成 starter。
- 生产化扩展：`CacheBook` 和 `PersistBook`，用于第 10 篇解释 Redis/MySQL 接入。

`ChatStream` 建议放到第一期之后。它有展示价值，但会引入 SSE 和前端页面，容易把第一期主线拉散。

## 四、第一阶段：博客文章列表

建议第一期先设计为 12 篇连续博客文章。前 8 篇构建核心项目，后 4 篇补齐生产化能力。后续公开视频课可以沿用这 12 个主题，每篇文章扩展成一集 45-60 分钟的视频。

| 篇次 | 博客文章标题 | 工程示例 | 对应文档 | 实战产出 |
|------|--------------|----------|----------|----------|
| 01 | 从 Go 原生 HTTP 服务走到 Go-Spring | `BookMan Pro` 起步 | `0.overview`、`1.getting-started`、`05-http-server` | 一个可运行的 `/echo` 服务 |
| 02 | Go-Spring 配置系统：让服务适配不同环境 | `BookMan Pro` + `ConfigLab` | `01-configuration` | `conf/app.properties`、Profile、命令行覆盖 |
| 03 | IoC 容器实战：用 Bean 组织 Controller、Service 和 DAO | `BookMan Pro` | `02-ioc-container` | Controller 注入 Service，Service 注入 DAO |
| 04 | 应用生命周期：Runner、后台 Job 与优雅关闭 | `BookMan Pro` + `LifecycleLab` | `03-app-start-stop` | 启动自检 Runner、后台 Job、信号退出 |
| 05 | HTTP 路由与中间件：实现 BookMan CRUD API | `BookMan Pro` | `05-http-server` | CRUD API、中间件、静态首页 |
| 06 | 分层架构改造：从 demo 演进到可维护项目 | `BookMan Pro` | `bookman/README_CN.md` | `internal/app`、`biz`、`dao`、`sdk` 分层 |
| 07 | 日志系统实战：访问日志、业务日志与结构化字段 | `BookMan Pro` | `04-logging` | 访问日志、业务日志、结构化字段、日志配置 |
| 08 | 测试与依赖替换：不用真实外部环境也能测业务 | `BookMan Pro` | `07-testing` | DAO 单测、Service 容器测试、Mock 依赖 |
| 09 | Starter 机制：把通用能力封装成可复用组件 | `StarterLab` | `06-components` | 自定义 starter 或业务 module |
| 10 | MySQL、Redis 与 pprof：补齐生产化基础设施 | `PersistBook` + `CacheBook` | `4.integrations` | GORM DAO、Redis 缓存、pprof 入口 |
| 11 | 自定义 Server：让 Go-Spring 管理更多服务形态 | `MultiServer Echo` | `03-app-start-stop`、`05-http-server` | 自定义 `gs.Server`、Ready 信号、多服务优雅关闭 |
| 12 | BookMan Pro 项目验收：一个 Go-Spring 服务如何交付 | `BookMan Pro` 完整版 | 全部文档 | 一个完整可运行的 `BookMan Pro` |

## 五、文章与视频分集设计

### 01. 从 Go 原生服务到 Go-Spring

核心问题：为什么不是直接 `http.ListenAndServe`？

实战任务：

- 使用标准库写 `/echo`。
- 把启动方式替换为 `gs.Run()`。
- 观察默认 HTTP 端口、配置加载、退出行为。
- 对比原生写法和 Go-Spring 写法的职责差异。

验收标准：

- `curl http://127.0.0.1:9090/echo` 返回固定文本。
- 学员能说清楚 `gs.Run()` 至少接管了配置加载、容器初始化、HTTP Server 和优雅关闭。

### 02. 配置系统实战

核心问题：如何让服务在 dev/test/prod 下使用不同配置？

实战任务：

- 新增 `conf/app.properties`。
- 配置 HTTP 端口、应用名称、业务开关。
- 使用 `value` tag 绑定到结构体。
- 使用 `-D` 参数和 `GS_` 环境变量覆盖配置。
- 演示 Profile 配置文件的覆盖关系。

验收标准：

- 不改代码即可切换端口和业务参数。
- 学员能解释配置优先级：命令行、环境变量、Profile 配置、基础配置、代码默认值。

### 03. IoC 容器与 Bean 装配

核心问题：如何避免业务代码里手动层层 new 对象？

实战任务：

- 注册 `BookController`、`BookService`、`BookDao`。
- 分别演示构造函数注入和字段注入。
- 使用接口导出实现，方便后续替换 DAO。
- 演示 Bean 名称、条件注册和 root bean 的典型场景。

验收标准：

- Controller 不直接创建 Service。
- Service 不直接创建 DAO。
- 应用启动时依赖缺失可以快速失败。

### 04. 应用启动、Runner 与优雅关闭

核心问题：启动后要执行初始化任务，关闭时要释放资源，应该放在哪里？

实战任务：

- 实现一个 `gs.Runner` 做启动自检或初始化数据。
- 实现一个后台 Job，监听根 Context。
- 手动触发退出信号，观察 Job 退出。
- 讨论 Runner、Server、普通 Bean 的边界。

验收标准：

- Runner 在 HTTP Server 接收请求前完成。
- 后台 Job 能在应用关闭时退出，不遗留 goroutine。

### 05. HTTP 路由与中间件

核心问题：Go-Spring 如何和标准库 HTTP、Gin、chi、gorilla/mux 共存？

实战任务：

- 使用 `*gs.HttpServeMux` 注册 `/books` CRUD API。
- 加入访问日志中间件。
- 暴露一个静态首页。
- 选做：把路由替换为 Gin 或 chi，业务层代码保持不变。

验收标准：

- 完成 `GET /books`、`GET /books/{isbn}`、`POST /books`、`DELETE /books/{isbn}`。
- 中间件能记录请求方法、路径、状态码和耗时。

### 06. 分层项目结构改造

核心问题：一个服务长大以后，目录和依赖应该如何组织？

实战任务：

- 按 `BookMan` 示例拆分 `internal/app`、`internal/biz`、`internal/dao`、`internal/sdk`。
- Controller 只处理 HTTP 编解码。
- Service 处理业务规则。
- DAO 处理数据访问。
- SDK 模拟外部依赖。

验收标准：

- Controller 中没有数据存储细节。
- Service 中没有 HTTP 请求/响应对象。
- DAO 和 SDK 可以在测试中替换。

### 07. 日志系统实战

核心问题：如何让日志既能用于排障，又不污染业务代码？

实战任务：

- 配置 ConsoleLogger 和 FileLogger。
- 使用结构化字段记录业务事件。
- 给访问日志和业务日志设置不同 tag。
- 演示日志级别调整。

验收标准：

- 控制台能看到开发调试日志。
- 文件中能看到结构化业务日志。
- 修改配置即可调整日志级别。

### 08. 测试与依赖替换

核心问题：如何测试 Go-Spring 应用而不启动完整外部环境？

实战任务：

- 对 DAO 写纯单元测试。
- 使用 `gs.RunTest` 对 Service 做容器测试。
- 使用 `gs.Configure().Property()` 注入测试配置。
- 使用 `gs.Configure().Provide()` 替换外部 SDK 或 DAO。
- 使用 `assert`、`require` 和 Mock 工具减少样板代码。

验收标准：

- `go test ./...` 通过。
- Service 测试不依赖真实 HTTP Server、MySQL 或 Redis。
- 学员能区分纯单测和容器测试的适用场景。

### 09. Starter 机制与组件化

核心问题：如何把一组通用 Bean 封装成可复用组件？

实战任务：

- 把访问日志中间件或业务 SDK 封装成一个 starter。
- 使用 `gs.Provide` 注册默认 Bean。
- 使用 `gs.Module` 按配置决定是否注册。
- 使用条件注册实现“配置存在才启用”。
- 讨论 `gs.Group` 适合多实例资源的场景。

验收标准：

- 应用侧通过空白导入启用 starter。
- 不配置关键属性时 starter 不创建资源。
- starter 提供资源释放函数。

### 10. MySQL、Redis 与 pprof 集成

核心问题：如何把常见基础设施接入项目，而不是每个项目重复写初始化代码？

实战任务：

- 使用 `starter-gorm-mysql` 替换内存 DAO。
- 使用 `starter-go-redis` 增加图书详情缓存。
- 使用 `starter-pprof` 暴露性能分析端口。
- 给外部资源配置超时、开关和 Profile。

验收标准：

- 开发环境可以继续使用内存 DAO。
- 集成环境通过配置切换到 MySQL 和 Redis。
- pprof 可以访问 `/debug/pprof/`。

### 11. 自定义 Server 与多服务形态

核心问题：当应用里不只有 HTTP Server，而是还有后台消费者、定时任务或自定义长运行服务时，应该如何接入 Go-Spring 生命周期？

实战任务：

- 实现一个最小 `gs.Server`。
- 演示 `Run`、`Stop` 和 Ready 信号。
- 在同一个应用中同时启动 HTTP Server 和自定义 Server。
- 模拟启动失败、运行期错误和关闭超时。
- 讨论 Runner、Job、Server 的边界。

验收标准：

- 自定义 Server 可以随 `gs.Run()` 一起启动。
- Ready 信号能影响应用整体启动完成时机。
- 应用收到退出信号时，自定义 Server 能按生命周期停止。

### 12. 综合项目验收与工程复盘

核心问题：一个 Go-Spring 服务达到什么程度才算“可交付”？

实战任务：

- 整理项目 README。
- 给出启动方式、配置方式、测试方式。
- 补齐关键 API 的 curl 示例。
- 执行一次故障演练：配置错误、依赖缺失、端口占用、外部资源不可用。
- 总结 Go-Spring 的适用边界和迁移策略。

验收标准：

- 新同学按 README 可以启动项目。
- `go test ./...` 通过。
- 错误配置能在启动期暴露。
- 项目能清晰展示 Go-Spring 的核心价值。

## 六、配套资料规划

### 代码仓库结构

建议把文章和示例代码分开管理。文章负责讲清楚问题和步骤，示例代码负责保证读者可以直接运行。

```text
misc/articles/
  01-from-net-http-to-go-spring.md
  02-configuration-for-environments.md
  03-ioc-controller-service-dao.md
  04-lifecycle-runner-job-shutdown.md
  05-http-routing-and-middleware.md
  06-layered-project-structure.md
  07-logging-in-practice.md
  08-testing-and-replacement.md
  09-custom-starter.md
  10-mysql-redis-pprof.md
  11-custom-server-lifecycle.md
  12-bookman-pro-delivery.md

misc/course/
  01-hello-http/
  02-configuration/
  03-ioc/
  04-lifecycle/
  05-http-routing/
  06-layered-bookman/
  07-logging/
  08-testing/
  09-starter/
  10-integrations/
  11-custom-server/
  12-final/
```

博客阶段建议优先采用目录式版本，读者可以直接对照文章打开代码。等进入视频阶段后，可以再补充分支式版本，方便用 diff 讲解每一集的改动。

每个示例目录建议至少包含：

- `README.md`：说明这一篇对应的目标、运行方式和验收命令。
- `go.mod`、`go.sum`：让读者可以单独进入目录运行。
- `main.go` 或 `cmd/*`：保持入口简单。
- `conf/app.properties` 或 `conf/app.yaml`：涉及配置的文章必须提供。
- `internal/*`：从第 6 篇开始再引入完整分层，前 5 篇不要过早复杂化。
- `*_test.go`：从第 8 篇开始成为必选项。

### 每篇文章固定材料

- 课前阅读：对应 docs 链接。
- 起始代码：上一篇结果或一个最小 scaffold。
- 实战任务：必须跟做的代码步骤。
- 验收命令：`go run .`、`curl`、`go test ./...` 等。
- 课后作业：一个小扩展，不引入过多新概念。
- 常见错误：依赖未注册、配置 key 不一致、Bean 条件未满足、端口占用、测试污染等。

### 文章模板

每篇文章可以使用同一套结构，降低写作和后续录制视频的成本：

```text
# 标题

## 本篇要解决的问题

## 起始代码

## 第一步：完成最小可运行版本

## 第二步：引入 Go-Spring 能力

## 第三步：验证行为

## 常见错误

## 小结

## 课后作业
```

对于第 9-11 篇这类进阶主题，可以把“第二步”拆成多段，但仍然要保持一个清晰主线，避免写成 API 说明书。

## 七、建议的发布节奏

### 博客文章阶段

- 每篇只解决一个核心问题，标题尽量具体。
- 每篇都产出一个可运行目录，避免文章只有概念没有代码。
- 每 3-4 篇做一次阶段复盘，整理读者容易卡住的问题。
- 先不要强依赖 MySQL、Redis 等外部环境，公开博客默认以内存实现跑通主线，外部资源作为选做。

建议按三批推进：

- 第一批：01-04，验证“为什么用 Go-Spring”和“配置、IoC、生命周期”这条基础线是否讲得顺。
- 第二批：05-08，完成 BookMan Pro 的主要业务形态，并把日志和测试补上。
- 第三批：09-12，补齐 starter、外部资源、自定义 Server 和项目交付。

如果第一批写完后发现读者更关心入门门槛，可以在 01 和 02 之间插入一篇短文：`用 15 分钟跑通第一个 Go-Spring 服务`。这篇只承担引流和降低门槛，不进入主线编号。

### 公开视频课阶段

- 每集 45-60 分钟。
- 每集使用对应博客作为讲义和评论区置顶资料。
- 视频重点演示完整链路、真实排错、代码 diff 和设计取舍。
- 作业保持简单，适合自学观众跟做。

### 后续扩展

- 如果后续做训练营，可以把每个主题扩展到 90-120 分钟，增加代码评审、故障排查和作业讲解。
- 如果后续做企业内训，可以压缩为 2 天或 3 天，加强迁移既有项目、配置治理、日志规范、测试策略和组件集成。

## 八、当前文档可支撑的内容

当前 docs 已经可以支撑大部分博客和视频内容：

- `0.overview/overview.md`：适合做第 1 篇/集的框架定位和设计理念。
- `1.getting-started/getting-started.md`：适合做第 1 篇/集的最小应用。
- `2.guides/01-configuration.md`：适合做第 2 篇/集配置系统。
- `2.guides/02-ioc-container.md`：适合做第 3 篇/集 IoC 与 Bean。
- `2.guides/03-app-start-stop.md`：适合做第 4 篇/集生命周期。
- `2.guides/04-logging.md`：适合做第 7 篇/集日志。
- `2.guides/05-http-server.md`：适合做第 5 篇/集 HTTP Server。
- `2.guides/06-components.md`：适合做第 9 篇/集 starter。
- `2.guides/07-testing.md`：适合做第 8 篇/集测试。
- `3.examples/bookman/README_CN.md`：适合做主线项目参考。
- `4.integrations/*.md`：适合做第 10 篇/集基础设施集成。

## 九、后续需要补齐或确认

- `cn/3.examples/examples.md` 目前还是 `todo`，建议后续整理成文章案例索引。
- `1.getting-started` 中 `gs init` 小节待补充，如果博客中要强调脚手架，需要先补齐这部分。
- starter 文档里的配置前缀需要以实际实现为准，文章和视频讲义中要避免前缀不一致造成误导。
- `BookMan Pro` 是否使用真实 MySQL/Redis，需要按发布阶段处理：博客阶段默认内存实现，外部资源作为选做；视频阶段可以增加单独演示。

## 十、第一版推荐方案

第一期建议先做“12 篇主线项目博客”，不要一开始就录视频，也不要拆成零散 API 专题。原因是 Go-Spring 的价值不是某个单点 API，而是配置、IoC、生命周期、HTTP、日志、测试、starter 组合起来形成的工程体验。博客可以更快验证这条主线是否顺，等文章顺序和代码稳定后，再进入公开视频课阶段。

推荐博客专题名或视频课名：

> Go-Spring 实战：从零构建一个可测试、可配置、可扩展的 Go 服务

推荐主线：

> 用 BookMan Pro 逐步实现一个图书管理服务，从标准库 HTTP 起步，最终演进到具备分层架构、配置治理、依赖注入、动态配置、测试替换、starter 集成和多服务生命周期管理能力的小型生产化服务。
