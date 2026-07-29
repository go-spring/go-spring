# Go-Spring：让 Go 服务研发像 Spring Boot 一样简单，甚至更强大

<div align="center">
 <img src="https://raw.githubusercontent.com/go-spring/go-spring/master/logo@h.png" width="140" alt="logo"/>
</div>

> **如果你以为这只是另一个 Go 框架，请再往下读一段。**
>
> Go-Spring 把 Java 社区 Spring 二十年的成功范式——依赖注入、自动装配、Starter 机制——用纯正的 Go 风格重新落地。当年 Spring 帮 Java 开发者摆脱了 EJB 地狱，把重型应用变成了可组装、可复用的模块化工程；现在 Go-Spring 想让 Go 开发者拥有同样的能力。
>
> **但这还不是全部。** Go-Spring 正在尝试一件更大的事：[Process as Code](MANIFESTO.md)——把软件研发流程本身，变成一个可装配、可复用、可版本化的"应用"。应用装配与研发流程共享同一套 IoC 思想，只是装配对象从运行时组件变成了研发动作。这是一个大胆的方向，也是一份值得验证的同构假设。

## 生态全景

Go-Spring 不是单一仓库——它是由 **核心框架、70+ Starter、开发工具、示例工程、项目模板** 共同构成的完整研发生态。每一层职责清晰，按需选用。

| 层次 | 定位 | 核心项目 |
|---|---|---|
| **基础层** | 零依赖通用工具 + 结构化日志引擎 | [`stdlib`](stdlib/), [`log`](log/) |
| **核心层** | IoC 容器、依赖注入、配置引擎、应用生命周期、能力抽象 | [`spring`](spring/)（含 `cloud/`、`web/`、`data/`、`actuator/` 等能力族） |
| **集成层** | 70+ 三方服务/框架的即插即用 Starter | [`starter/`](starter/) — Gin、gRPC、Redis、MySQL、Kafka、Dubbo、Kitex… |
| **工具层** | 命令行、代码生成、Mock | [`gs`](gs/gs)、[`gs-http-gen`](gs/gs-http-gen)、[`gs-mock`](gs/gs-mock) |
| **示例与模板** | 端到端示例应用 + 项目脚手架 | [`examples/`](examples/)、[`contrib/`](contrib/)、[`layout/`](layout/) |

完整模块清单及架构约束见 [ARCHITECTURE_CN.md](ARCHITECTURE_CN.md)。

## 为什么选择 Go-Spring

### 开箱即用，零侵入

所有能力以 **Starter** 形式交付。不需要继承、不需要适配器、不需要在 `main.go` 里写一长串初始化代码——`import` 一个 starter，它就自动把你的组件接入生命周期。框架不抢占 `main()`，不强加路由分组，不要求特定目录结构。你按自己的方式写业务代码，框架负责装配与生命周期。

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

你不必手写信号处理，不必管理 goroutine 退出——框架已经做好了。

### 内置企业级基础设施

| 领域 | 能力 | 覆盖范围 |
|---|---|---|
| **配置** | 多来源分层合并（命令行 → 环境变量 → 配置文件 → 远程配置中心），类型安全绑定，动态刷新 | Nacos、Consul、Etcd、K8s ConfigMap、Vault |
| **日志** | 结构化日志模型，精简配置语法，插件化 Appender | Console、File、自定义 |
| **服务发现** | 统一 `Discovery` 抽象，多注册中心后端 | Consul、Etcd、Nacos、Zookeeper、Polaris、K8s |
| **分布式协同** | 分布式锁、消息、事务、事件、任务调度、批量处理 | Lock（4 种后端）、Kafka、Pulsar、RabbitMQ、NATS、MQTT、Saga、TCC、AT |
| **可观测** | 统一 OpenTelemetry 接入，一行配置启用全链路追踪和指标 | starter-otel + 各 starter 的 example-otel |
| **安全** | 访问控制、OAuth2、JWT、Session | Casbin（RBAC/ABAC/ACL）、OAuth2 Client/Server、JWT、分布式 Session |

### 丰富到奢侈的 Starter 生态（70+ 模块）

每个 starter 是一个独立的 Go module，按需引入，不污染依赖图：

- **Web 框架**：Gin、Echo、Hertz、go-zero、GoFrame、Kratos
- **RPC 框架**：gRPC、Kitex、Thrift、tRPC、Dubbo-go、go-zero/zrpc、GoFrame/gRPC、Kratos/gRPC
- **WebSocket**：Gorilla、Coder、GoFrame、Kratos
- **数据库**：MySQL、PostgreSQL、SQL Server、ClickHouse、MongoDB、Neo4j、Elasticsearch
- **缓存**：Redis（go-redis / redigo 双驱动）、Memcached、BigCache
- **消息队列**：Kafka（franz-go / Sarama 双驱动）、Pulsar、RabbitMQ、NATS、MQTT
- **配置中心**：File、Consul、Etcd、Nacos、K8s ConfigMap、Vault、配置总线
- **服务注册**：Consul、Etcd、Nacos、Zookeeper、K8s
- **分布式协同**：锁（Consul/Etcd/K8s/Redis）、事务（Saga/TCC/AT）、任务调度、批处理、协程池
- **安全**：Casbin、OAuth2 Client/Server、JWT、Session-Redis
- **可观测**：OpenTelemetry（Tracing + Metrics）、pprof、Actuator、Admin UI
- **更多**：邮件、Lua 过滤器、Swagger、数据校验、仓库模式、数据迁移、API 网关、Service Mesh、弹性（resilience）

> 完整分类列表见 [starter/README_CN.md](starter/README_CN.md)。

### 强大的工具链

| 工具 | 用途 |
|---|---|
| `gs` | 一站式命令行：创建项目、添加组件、生成代码、运行服务 |
| `gs-http-gen` | 现代化 IDL 语法 → HTTP 服务端 + 声明式客户端代码生成（支持 nullable、泛型、嵌入等特性，对标 OpenFeign） |
| `gs-mock` | 类型安全的 Go mock 库，泛型原生支持，并行安全 |

### 无缝测试集成

与 `go test` 深度集成。用 `gs.RunTest()` 启动真实容器，注入真实依赖——不必 mock 一切。需要 mock 特定组件时，`gs-mock` 提供类型安全的方法/函数级 mock，并行安全，基于 `context` 实现数据隔离。

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

## 交流

<table style="border: none;">
<tr style="border: none;">
<td style="text-align: center; border:none;"><img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/qq(1).jpeg" width="*" height="180" alt="QQ群二维码"/></td>
<td style="text-align: center; border:none;"><img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/go-spring-action.jpg" width="*" height="180" alt="公众号二维码"/></td>
</tr>
<tr style="border: none;">
<td style="text-align: center; border:none;">QQ群号: 721077608</td>
<td style="text-align: center; border:none;">公众号: GoSpring实战</td>
</tr>
</table>

## 捐赠

<img src="https://raw.githubusercontent.com/go-spring/go-spring/master/sponsor.png" width="140" />

为了推动 Go-Spring 的持续发展，我们诚挚邀请您支持本项目。您的捐赠将帮助我们更快迭代功能、完善生态、壮大社区。

## Star History

<img src="https://api.star-history.com/svg?repos=go-spring/go-spring&type=Date" width="600" alt="Star History"/>

## 许可证

Go-Spring 基于 [Apache License 2.0](LICENSE) 发布。