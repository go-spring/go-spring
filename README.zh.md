# Go-Spring：让 Go 服务研发像 Spring Boot 一样简单，甚至更强大

<div align="center">
 <img src="https://raw.githubusercontent.com/go-spring/go-spring/master/logo@h.png" width="140" alt="logo"/>
</div>

> **如果你以为这只是另一个 Go 框架，请再往下读一段。**
>
> Go-Spring 的使命是**为 Go 构建一个完整、厂商中立的应用生态**——补上 Go 生态里缺失的那层装配能力：DI 库只管零件，框架生态各自围绕自己的传输层，组件库彼此不认识——Go-Spring 把它们装到一起。
>
> 它把 Java 社区 Spring 二十年的成功范式——依赖注入、自动装配、Starter 机制——用纯正的 Go 风格重新落地。当年 Spring 帮 Java 开发者摆脱了 EJB 地狱，把重型应用变成了可组装、可复用的模块化工程；现在 Go-Spring 想让 Go 开发者拥有同样的能力。
>
> **而在 AI 时代，这件事只会更重要。** 当 AI 写出的代码越来越多，稀缺的就不再是打字速度，而是底座：一个稳定、完整、自描述的基础——人和 agent 都能在上面做事，也都要受它约束。Go-Spring 要做的就是这个底座：90+ 个 Starter 共用同一套配置模型、同一个应用生命周期和一份机器校验的可观测契约，工程约定就写在代码旁边（[starter/DESIGN_CN.md](starter/DESIGN_CN.md)、各模块的 `DESIGN` / `USAGE` 文档，以及 [`/gs`](skills/gs) 这个 Claude Code Skill）。

## 生态全景

Go-Spring 不是单一仓库——它是由**核心框架、生态抽象库、90+ Starter、开发工具、示例工程和项目模板**共同构成的完整研发生态。每一层职责清晰，依赖单向向下流动，按需选用。

| 层次 | 定位 | 核心项目 |
|---|---|---|
| **基础层** | 零依赖通用工具 + 纯语义组件（HTTP 客户端/服务端等）+ 结构化日志引擎 | [`stdlib`](stdlib/)、[`log`](log/) |
| **核心层** | IoC 容器、依赖注入、分层配置引擎、应用生命周期——仅此而已 | [`spring`](spring/)（`gs` + `conf`） |
| **生态抽象层** | 不依赖容器的能力抽象：治理中心、服务发现、负载均衡、缓存、锁、消息、定时任务、安全等——**不 import** 任何 `spring` 包 | [`cloud`](cloud/) |
| **集成层** | 90+ 即插即用 Starter：三方 SDK + gs 接线 | [`starter/`](starter/) — Gin、gRPC、Redis、MySQL、Kafka、Dubbo、Kitex… |
| **工具层** | 命令行、代码生成、Mock、AI Skill | [`gs`](gs/gs)、[`gs-http-gen`](gs/gs-http-gen)、[`gs-mock`](gs/gs-mock)、[`skills/gs`](skills/gs) |
| **示例与模板** | 端到端示例应用 + 项目脚手架 | [`contrib/`](contrib/)、[`layout/`](layout/) |

保持分层干净的一条准则：**抽象放 `cloud`，三方 SDK 与 gs 接线放 starter，不依赖生态的纯语义放 `stdlib`**——starter 负责接线，cloud 负责抽象，spring 负责运行。

**今天这些加起来是多少：** 90+ 个 starter 模块——6 个 Web 框架外加标准库 HTTP 服务端、8 个 RPC 框架、12 种数据库、4 种缓存、7 种消息队列、7 个配置源、5 种注册中心、4 种分布式锁、3 种事务模型，再加对象存储、邮件、Webhook、任务调度、批处理、网关与安全一整套。它们走的是同一套分层配置引擎，进的是同一个应用生命周期；其中多数还共用同一个治理中心与可观测模型——碰到自带成熟生态的组件，是集成去适配它，而不是反过来。换后端只是改配置、不改代码：抽象是中立的，它周围的部件都不用动。

每个 starter 都要遵守的设计约束见 [starter/DESIGN_CN.md](starter/DESIGN_CN.md)；模块的分类导览见 [starter/README_CN.md](starter/README_CN.md)。

## 为什么选择 Go-Spring

### 使命：补全 Go 生态

Go 标准库在语言运行时这一层以完整著称，在应用层却刻意保持极简：没有依赖注入模型，没有分层配置约定，没有 Starter 式自动装配，没有统一的应用生命周期与治理层。Java 有 Spring 补上了这块；**Go 至今没人补**——DI 库只管零件，框架生态各自围绕自己的传输层，组件库彼此不认识。

Go-Spring 就是为了补上这块：**为 Go 构建一个完整、厂商中立的应用生态**——不是又一个 RPC 框架，不是又一个 Web 框架，也不是把你正在用的东西重写一遍。它不跟栈里的任何组件竞争，它装配你的栈。

这个位置上还有谁？如实列出：

| 相邻玩家 | 代表 | 为什么不是同一件事 |
|---|---|---|
| **DI 库** | Wire、fx、dig | 只解决注入——没有配置引擎、没有 Starter 模型、没有生命周期与治理。是零件，不是生态。 |
| **框架为中心的生态** | Kratos、go-zero、CloudWeGo | 很优秀，但生态围绕**它自己的**框架转：传输、目录结构、代码生成、工具链。选一个就是一次承诺。 |
| **组件库** | dubbo-go、Kitex、Hertz、GORM、Gin… | **同行，不是对手**——Go-Spring 把它们一律作为 Starter 对等地集成进来。 |

Go 生态缺的正是这样一个中立的、Spring Boot 形态的应用平台。这就是 Go-Spring 要坐的位置。

### 不是又一个 RPC 框架，而是应用平台

这是它与 **dubbo-go、Kitex、Kratos、go-zero** 这类项目最本质的区别：那些（主要）是 **RPC / 微服务框架**——它们拥有传输层、服务模型，往往还有一套代码生成流水线。你的应用*就是*一个 dubbo 应用或 Kitex 应用；采纳它意味着采纳它的编程模型。

Go-Spring **不拥有任何协议、任何传输**。它是**它们之下、它们之旁的装配与运行时层**：依赖注入、带热更新的分层配置、生命周期管理、集中的服务治理。有本仓库为证——dubbo-go、Kitex、Kratos、go-zero、GoFrame、gRPC、tRPC、Thrift 在这里各自都只是 **90 个 Starter 中的一个**，与 Redis、MySQL 用同一套 `gs.Module` 机制接进来：

| | dubbo-go / Kitex / go-zero / Kratos | Go-Spring |
|---|---|---|
| **品类** | RPC / 微服务框架（拥有传输、服务模型、代码生成） | 应用装配与运行时平台（IoC + 配置 + 生命周期 + 治理） |
| **关系** | *被 Go-Spring 集成*——`starter-dubbo` 包住 dubbo-go；`starter-kitex`、`starter-kratos`、`starter-go-zero`… 包住其余 | 一视同仁地集成它们；不选边 |
| **配置** | 各带一套配置模型 | 一套分层引擎（命令行 → 环境变量 → 配置文件 → Nacos/etcd/Consul/Vault/K8s）驱动所有组件，配 `gs.Dync[T]` 热更新 |
| **治理** | 只管自己的 RPC 调用（dubbo-go：URL 参数覆盖） | **一个集中式治理中心**——超时/重试/熔断/限流 + 故障注入，统一作用于 Redis、GORM、HTTP、gRPC、gin、dubbo…，出自同一份治理规则文档，规则源可插拔（文件/HTTP 控制台/nacos/etcd 直连监听） |
| **编程模型** | 必须使用框架定义的接口与结构 | 零侵入：标准 `net/http`、普通 struct、你自己的目录结构 |
| **能否单独使用** | 可以 | 可以——核心（`spring`）与生态抽象库（`cloud`）完全不依赖任何 RPC 框架 |

一句话：**它们回答"服务之间怎么调用"，Go-Spring 回答"应用怎么被装配、配置、治理并保持可维护"**。一个生产服务通常两者都需要——所以 Go-Spring 选择集成它们，而不是与它们竞争。（DI 框架这一轴——Wire/fx/dig——的对比见 [spring/README_CN.md](spring/README_CN.md#11--与其他框架的对比)。）

### 开箱即用，零侵入

所有能力以 **Starter** 形式交付。不需要继承、不需要适配器、不需要在 `main.go` 里写一长串初始化代码——`import` 一个 starter，它就自动把你的组件接入生命周期。框架不抢占 `main()`，不强加路由分组，不要求特定目录结构。你按自己的方式写业务代码，框架负责装配与生命周期。

### 可观测：默认齐整，且可扩展

可观测性恰恰是"零件生态"最容易散架的地方：每个库对"指标该叫什么"都有自己的主张，而你一旦换掉某个后端，看板、告警、查询就全部失去意义。Go-Spring 把它当作契约来对待。

- **每个组件都被插桩，不是可选项。** 全部 54 个组件（数据库、消息、HTTP、RPC 各族，加上 cloud 域包）在同一份规约下产出信号，缺一个信号、或结果值分不出成败，都算**缺陷**而不是风格差异。覆盖范围包括框架**代你执行**的操作：注册中心心跳、加锁、缓存与 MQ 访问，而不只是你自己的 handler。
- **同类组件用同一套词汇。** 同类组件共用名字、仪器类型与取值词表，并遵循 OpenTelemetry 语义约定（`db.system`、`messaging.system`、`http.*`）。把 Redis 换成 Memcached、把 Kafka 换成 Pulsar，你的运维资产照样读得懂。组件仍可自由添加自己的字段——只在语义相同时才要求一致。
- **靠脚本强制，不靠自觉。** [`scripts/check-observability.sh`](scripts/check-observability.sh) 对全部 54 个组件机械校验规约，并由 [`scripts/check-all.sh`](scripts/check-all.sh) 在每次 push 与 PR 上运行——规约漂移会让构建失败，而不是悄悄腐烂。
- **你要加的数据不需要改框架。** 用 `log.WithFields` / `log.Collect` 或 `observability.WithContextAttributes` 在 context 上标注，这些值会落到日志行上，**也**会落到每一个 span 上——包括框架代你创建、你根本拿不到句柄的那些 span。metric 则刻意保持封闭（无界的标签正是基数爆炸的来源）；需要自己的指标时，用 `otel.Meter(...)` 自建。
- **没有另一套观测 API 要学。** 用的就是框架的 `log` 包和 OpenTelemetry 本身，所以你对 OTLP、Collector 与后端的既有认知完全适用。

### 依赖注入，纯 Go 方式

没有反射黑箱，没有 `@Autowired` 注解，没有 XML 配置。所有依赖通过**构造函数参数**显式声明，由容器按类型自动装配：

```go
gs.Provide(func(db *gorm.DB) *UserService {
    return &UserService{db: db}
})
```

声明你需要的，暴露你提供的——仅此而已。

### 统一的运行模型：Runner 与 Server

Go-Spring 用两种抽象覆盖了所有服务形态：

- **Runner** — 一次性执行单元（任务调度、批处理、启动后执行一次的逻辑）。容器收集所有 Runner，按配置顺序执行。
- **Server** — 常驻服务（HTTP、gRPC、Thrift、WebSocket…）。容器接管 `ListenAndServe` 和优雅关闭，通过 `ReadySignal` 通知就绪状态。

不必手写信号处理，不必操心 goroutine 何时退出——框架都替你做好了。

### 内置企业级基础设施

| 领域 | 能力 | 覆盖范围 |
|---|---|---|
| **配置** | 多来源分层合并（命令行 → 环境变量 → 配置文件 → 远程配置中心），类型安全绑定，动态刷新 | Nacos、Consul、Etcd、K8s ConfigMap、Vault、Apollo |
| **服务治理** | 集中式治理中心：超时/重试/熔断/限流/隔离 + 故障注入，一份规则文档服务所有客户端，热更新 | default 与 sentinel 两种后端；规则源：文件、HTTP 控制台、nacos、etcd |
| **日志** | 结构化日志模型，精简配置语法，插件化 Appender | Console、File、自定义 |
| **服务发现** | 统一 `Discovery` 抽象，多注册中心后端 | Consul、Etcd、Nacos、Zookeeper、K8s |
| **分布式协同** | 分布式锁、消息、事务、任务调度、批量处理 | Lock（4 种后端）、Kafka、Pulsar、RocketMQ、RabbitMQ、NATS、MQTT、Saga、TCC、AT |
| **可观测** | 每个组件在同一份规约下插桩，且有脚本机械校验；引入一个 starter 即可为所有组件点亮追踪与指标 | starter-otel + 各 starter 的 example-otel |
| **安全** | 访问控制、OAuth2、JWT、Session | Casbin（RBAC/ABAC/ACL）、OAuth2 Client/Server、JWT、分布式 Session |

### 丰富到奢侈的 Starter 生态（90+ 模块）

每个 starter 是一个独立的 Go module，按需引入，不污染依赖图：

- **Web 框架**：Gin、Echo、Hertz、go-zero、GoFrame、Kratos——另有一整套标准库 `net/http` 服务端的中间件组件
- **RPC 框架**：gRPC、Kitex、Thrift、tRPC、Dubbo-go、go-zero/zrpc、GoFrame/gRPC、Kratos/gRPC
- **WebSocket**：Gorilla、Coder、GoFrame、Kratos
- **HTTP 客户端**：以接口声明远程服务，服务发现、负载均衡、弹性都已接好（调用点由 `gs-http-gen` 生成）
- **数据库**：MySQL、PostgreSQL、SQL Server、ClickHouse、SQLite、MongoDB、Neo4j、Elasticsearch、Cassandra/ScyllaDB、Milvus、InfluxDB、TDengine
- **缓存**：Redis（go-redis / redigo 双驱动）、Memcached、BigCache
- **对象存储**：S3 协议——MinIO、AWS S3、阿里云 OSS、腾讯云 COS
- **消息队列**：Kafka（franz-go / Sarama 双驱动）、Pulsar、RocketMQ、RabbitMQ、NATS + JetStream、MQTT
- **任务队列**：asynq、xxl-job
- **配置中心**：File、Consul、Etcd、Nacos、K8s ConfigMap、Vault、Apollo、配置总线
- **服务注册**：Consul、Etcd、Nacos、Zookeeper、K8s
- **分布式协同**：锁（Consul/Etcd/K8s/Redis）、事务（Saga/TCC/AT）、Outbox、任务调度、批处理、协程池
- **安全**：Casbin、OAuth2（Client/Server/Resource Server）、JWT、Session-Redis、Lua 请求过滤器
- **可观测**：OpenTelemetry（Tracing + Metrics）、pprof、Actuator、Admin UI
- **公司基线**：`starter-luohua`——把一家公司的默认值（身份、透传词汇、错误目录、标准缓存）组合成一个模块，服务只要 blank-import 它就能切到公司约定上
- **更多**：邮件、Webhook、数据迁移（goose）、Swagger、API 网关、限流

> 分类导览见 [starter/README_CN.md](starter/README_CN.md)。

### 强大的工具链

| 工具 | 用途 |
|---|---|
| `gs` | 一站式命令行：创建项目、添加组件、生成代码、运行服务 |
| `gs-http-gen` | 现代化 IDL 语法 → HTTP 服务端 + 声明式客户端代码生成（支持 nullable、泛型、嵌入等特性，对标 OpenFeign） |
| `gs-mock` | 类型安全的 Go mock 库，泛型原生支持，并行安全 |
| `/gs` | 覆盖 Go-Spring 项目全生命周期的 Claude Code Skill：规划 → 执行 → 交付，把每次改动约束到项目既有约定上（[安装](skills/gs/README_CN.md)） |

### 无缝测试集成

与 `go test` 深度集成。用 `gs.RunTest()` 启动真实容器，注入真实依赖——不必 mock 一切。需要 mock 特定组件时，`gs-mock` 提供类型安全的方法/函数级 mock，并行安全，基于 `context` 实现数据隔离。

## Go-Spring 要往哪里走

方向不是"为了加功能而加功能"，而是把这个底座做得足够完整、足够可靠，让 Go 团队不必再自己重建一个。具体是四件事：

1. **补齐长尾集成。** 只要一个服务需要某个能力，答案就应该是 Starter，而不是团队内部的胶水包。新 starter 一律落在同一套设计约束下（[starter/DESIGN_CN.md](starter/DESIGN_CN.md)）——端口、driver 模式、多实例寻址、fail-fast 校验——行为与已有的那些保持一致。
2. **每个组件都进同一套治理与可观测模型。** 今天的覆盖已经跨过常用技术栈；方向是没有任何后端是二等公民——同一份规则文档治理、同一套词汇插桩、同一个脚本校验。
3. **保持应用层 API 稳定。** 一个框架配得上"基础"二字，靠的是自身演进时不逼业务代码跟着改。这是压在上面每一条之上的长期约束：加增量助手可以，破坏性变更不行。
4. **让约定机器可读。** 项目的规则本就写在代码旁边——[starter/DESIGN_CN.md](starter/DESIGN_CN.md)、各模块的 `DESIGN` / `USAGE` 文档，以及 [`/gs`](skills/gs) Skill。当 agent 承担越来越多的编码工作，一个约定能被 agent **读懂并遵守**（而不只是能被调用）的框架，才谈得上是底座。

如果你也以写 Go 服务为生，这个位置值得一起来填：你贡献的每一个集成，都让另一个人少写一个内部包装。

## 一分钟上手

```bash
# 1. 安装 gs 工具
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/go-spring/gs/HEAD/install.sh)"

# 2. 创建项目
gs init --module github.com/yourname/yourproject

# 3. 启动
go run main.go
```

## 文档

| 文档 | 说明 |
|---|---|
| [概览](website/cn/docs/0.overview/) | 总体介绍、AI 工程化理念、Claude Code 最佳实践 |
| [快速入门](website/cn/docs/1.getting-started/) | 项目创建、开发、运行 |
| [专题指南](website/cn/docs/2.guides/) | 配置、IoC、启停、日志、HTTP 服务、组件、测试、http-gen |
| [示例](website/cn/docs/3.examples/) | 完整示例索引 |
| [组件集成](website/cn/docs/4.integrations/) | 各 starter 的详细集成文档 |
| [FAQ](website/cn/docs/5.faq.md) | 常见问题 |
| [贡献指南](website/cn/docs/6.contributing.md) | 如何参与贡献 |
| [更新日志](website/cn/docs/7.changelog.md) | 版本变更记录 |

如果你偏好通过完整示例循序渐进地学习，推荐 [go-spring-first](https://github.com/lvan100/go-spring-first)，其中整理了 10 个入门示例。

## 贡献

如何成为贡献者？提交有意义的 PR 或者需求，并被采纳。详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

几个可以上手的地方（大致按投入从少到多）：

- **报一个粗糙点**——某个组件的配置、治理或可观测性与它所在的族不一致。
- **同步文档**——每个模块都有中英一对，它们会漂移；翻译同样是贡献。
- **写一个示例**放到 [contrib/](contrib/) 下，把多个 starter 组合起来并做端到端验证。
- **加一个 Starter**——为你在用的某个后端，照着 [starter/DESIGN_CN.md](starter/DESIGN_CN.md) 和最接近的现有 starter 来做。

## 交流

<table style="border: none;">
<tr style="border: none;">
<td style="text-align: center; border:none;"><img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/qq(1).jpeg" width="*" height="180" alt="QQ群二维码"/></td>
<td style="text-align: center; border:none;"><img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/go-spring-action.jpg" width="*" height="180" alt="公众号二维码"/></td>
</tr>
<tr style="border: none;">
<td style="text-align: center; border:none;">QQ群号：721077608</td>
<td style="text-align: center; border:none;">公众号：GoSpring实战</td>
</tr>
</table>

## 捐赠

<img src="https://raw.githubusercontent.com/go-spring/go-spring/master/sponsor.png" width="140" />

为了推动 Go-Spring 的持续发展，我们诚挚邀请您支持本项目。您的捐赠将帮助我们更快迭代功能、完善生态、壮大社区。

## Star History

<img src="https://api.star-history.com/svg?repos=go-spring/go-spring&type=Date" width="600" alt="Star History"/>

## 许可证

Go-Spring 基于 [Apache License 2.0](LICENSE) 发布。
