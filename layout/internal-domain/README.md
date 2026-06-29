# 务实领域分层架构（Pragmatic Domain Layered Architecture）

## 架构定位

面向领域复杂度较高的单服务 Go 项目布局。它保留 DDD 战术设计中最能约束复杂度的部分（聚合根、充血模型、值对象、领域事件、严格四层），同时结合 Go 隐式接口、Go-Spring IoC 和服务内分层治理的工程现实，裁剪过度仪式化的实践。适用于稍复杂到中大型的业务服务，并在边界稳定后按 BC（Bounded Context，限界上下文）拆分为独立服务。

### 与 DDD 的关系

| DDD 要素 | 本架构采纳方式 |
|---|---|
| 聚合根与充血模型 | 必须。业务规则必须留在 `domain` 层。 |
| 值对象 | 必须。 |
| 领域事件 | 支持定义与消费，发布侧由项目补充。 |
| 分层架构 | 严格四层，禁止反向依赖。 |
| Repository 接口 | 默认不预设抽象，`application` 直连 `infra/repo` 具体类型；多实现时再在 `domain` 层提取接口。 |
| 同 BC 内防腐层 | 不强制。同 BC 子域通过 `application` 层协作；跨 BC 强制经 `infra/client/` 防腐层。 |
| 显式接口声明 | 不强制。单实现直接依赖具体类型，出现多态或跨包抽象需求时再声明接口。 |

### 技术约束

技术约束只列不可轻易打破的边界，后续扩展与演进都以这些约束为前提：

| 约束 | 规则 |
|---|---|
| 领域行为内聚 | 业务规则必须由 `domain` 层承载，禁止贫血模型。 |
| 依赖方向 | `api` 不绕过 `application`，`domain` 不引用外层实现。 |
| BC 边界 | 跨 BC 必须经 `infra/client/` 防腐层；同 BC 子域通过 `application` 层协作，禁止直接 import 对方 `domain` 模型。 |
| 装配方式 | 优先使用 Go-Spring IoC 完成装配，不为单实现预设抽象。 |
| 抽象策略 | 单实现直连具体类型；出现多态、测试替身或跨包边界时再提取接口。 |

## 目录结构

以下目录展示单个服务内部的推荐基线。`order`、`user` 仅作为领域示例，真实项目按自身 BC 或子域替换。

```
├── api/                                # 最外层接入层：协议适配、路由、请求入口、任务与消息触发入口
│   ├── job/                            # 定时任务、后台 job 等非请求型入口
│   ├── controller/                     # 按业务域纵向拆分的协议入口处理器
│   │   ├── order/                      # 订单业务域
│   │   │   ├── converter/              # IDL 请求/响应模型与 application DTO 的转换器
│   │   │   └── order_controller.go     # 订单业务入口实现
│   │   └── user/                       # 用户业务域
│   │       ├── converter/              # IDL 请求/响应模型与 application DTO 的转换器
│   │       └── user_controller.go      # 用户业务入口实现
│   └── server/                         # 协议入口聚合目录
│       ├── httpsvr/                    # HTTP 协议适配层
│       │   ├── middleware/             # HTTP 专属中间件链
│       │   ├── handler.go              # HTTP 协议 handler（组合业务 controller，实现协议接口）
│       │   └── httpsvr.go              # HTTP Server 生命周期管理
│       ├── thriftsvr/                  # Thrift RPC 协议适配层
│       │   ├── middleware/             # Thrift 拦截器与链路追踪
│       │   ├── handler.go              # Thrift 协议 handler（组合业务 controller，实现协议接口）
│       │   └── thriftsvr.go            # Thrift Server 生命周期管理
│       ├── grpcsvr/                    # gRPC 协议适配层
│       │   ├── middleware/             # gRPC Unary/Stream 拦截器
│       │   ├── handler.go              # gRPC 协议 handler（组合业务 controller，实现协议接口）
│       │   └── grpcsvr.go              # gRPC Server 生命周期管理
│       └── mqsvr/                      # MQ 消费者入口与生命周期管理
│           ├── middleware/             # MQ 消费专属中间件链
│           ├── handler.go              # MQ 消费 handler（组合业务 controller，实现消费接口）
│           └── mqsvr.go                # MQ Consumer 注册、启动和关闭
├── application/                    # 应用层：编排领域对象与基础设施能力
│   ├── order/                      # 订单业务域
│   │   ├── assembler/              # DTO 与 Entity 的转换器
│   │   ├── dto/                    # 数据传输对象
│   │   └── order_service.go        # 订单应用服务实现
│   └── user/                       # 用户业务域
│       ├── assembler/              # DTO 与 Entity 的转换器
│       ├── dto/                    # 数据传输对象
│       └── user_service.go         # 用户应用服务实现
├── domain/                         # 领域层：核心业务规则与业务逻辑
│   ├── order/                      # 订单业务域
│   │   ├── order.go                # 聚合根与实体
│   │   ├── order_value.go          # 值对象
│   │   └── order_event.go          # 领域事件
│   └── user/                       # 用户业务域
│       ├── user.go                 # 聚合根与实体
│       ├── user_value.go           # 值对象
│       └── user_event.go           # 领域事件
├── infra/                          # 基础设施实现层：访问外部系统与中间件
│   ├── repo/                       # 领域仓储实现（数据库 / 缓存）
│   │   ├── order/                  # 订单仓储实现
│   │   │   └── order_repo.go
│   │   └── user/                   # 用户仓储实现
│   │       └── user_repo.go
│   ├── client/                     # 外部服务 / SDK 调用
│   │   └── uranus/                 # Uranus 平台服务调用封装（防腐层）
│   └── mq/                         # MQ Producer（仅出向，消费者保留在 api/server/mqsvr）
├── pkg/                            # 内部通用工具，不承载业务语义
│   ├── stringutil/                 # 字符串工具
│   ├── timeutil/                   # 时间工具
│   └── safego/                     # 安全并发工具
└── consts/                         # 全局共享常量
    └── errno/                      # 全局错误码定义
```

## 分层与边界约束

### 四层职责

目录结构中的四层按职责划分，不能只用文件位置判断代码归属：

| 层级 | 职责 |
|---|---|
| `api` | 承接 HTTP / RPC / MQ / Job 等入向流量，完成协议适配和请求转换。 |
| `application` | 组织用例流程，编排 `domain` 与 `infra`，不承载核心业务规则。 |
| `domain` | 定义聚合根、实体、值对象、领域事件等领域对象，承载业务不变量。 |
| `infra` | 实现 DB、缓存、RPC、SDK、MQ Producer 等出向能力。 |

### 依赖方向

合法依赖方向以职责为准：`api` 只调用 `application`；`application` 可以编排 `domain` 与 `infra`；`infra` 可以依赖 `domain` 模型完成持久化或适配；`domain` 不依赖其他业务层。`pkg` 与 `consts` 只提供无业务语义的通用能力。

```
                ┌────────────────────────┐
                │       pkg / consts     │
                └───────────▲────────────┘
                            │
     ┌──────────────┬───────┴──────┬──────────────┐
     │              │              │              │
  ┌───────┐    ┌───────────┐    ┌────────┐    ┌────────┐
  │  api  │───▶│application│    │ domain │◀───│ infra  │
  └───────┘    └─────┬─────┘    └────▲───┘    └───▲────┘
                     │               │            │
                     └───────────────┴────────────┘
```

### 跨域协作与防腐层

- **同 Bounded Context（限界上下文）内**：子域通过 `application` 层 service 单向协作（如 `order` → `user`），禁止直接 import 对方 `domain` 模型。出现循环依赖时，通过领域事件或只读查询服务解耦。
- **跨 BC 或外部系统**：必须通过 `infra/client/` 下的防腐层隔离外部语义，业务层不直接感知外部协议和模型。

防腐层负责封装协议转换、错误适配与外部模型映射。只要涉及不同 BC、外部模型语义不一致、网络 IO / RPC / 第三方 SDK，就必须放入 `infra/client/`。

## 扩展与演进约定

这一节描述项目复杂度上升后的演进通道。默认做法保持出厂态简单，演进方式只在触发条件真实出现后启用。

### 默认策略与演进通道

以下表格按“默认做法 + 触发条件 + 演进方式”阅读：

| 默认做法 | 触发条件 | 演进方式 |
|---|---|---|
| `application` 直接依赖 `infra/repo` 具体类型 | 出现主从、缓存、读写分离等多种 Repository 实现 | 在 `domain` 层提取 Repository 接口，由 `infra` 层实现 |
| 同 BC 子域通过 `application` 层单向协作 | 跨团队 / 多仓库协作，或子域边界频繁变化 | 提取稳定契约，必要时引入防腐层隔离语义 |
| 保持服务内四层目录 | 子域需要独立部署或独立演进 | 将子域提取为独立服务，沿用原有分层结构 |
| 测试默认使用 Go-Spring Mock | 测试框架独立性要求强，或关键依赖需要替身 | 仅对关键依赖提取接口，其余依赖仍保持直接连接 |

### 接口使用

Go 遵循隐式接口。接口应当命名真实变化点，而不是作为层与层之间的默认胶水：

- 只有一种实现时，直接依赖具体类型，不为了“未来可能”提前定义接口。
- 出现多种实现共存、关键依赖替身、跨包边界抽象等真实多态需求时，再提取接口。
- 单元测试默认使用 Go-Spring Mock 工具，不为了测试本身强制抽象所有依赖。

### Repository

Repository 默认只保留具体实现，`application` 直接依赖 `infra/repo` 下的类型。当出现主从、缓存、读写分离等多种实现需求时，再在 `domain` 层提取 Repository 接口，由 `infra/repo` 实现。

## 工程组织约定

### 文件命名

文件是否带领域前缀，按“是否会被外部包识别”判断：

- **带前缀**：文件中的类型会被跨包注入、嵌入或构造（如 controller、service、SDK 实现），文件名加前缀方便全局 grep 定位。
  - 示例：`order_controller.go`、`order_service.go`、`order_sdk.go`
- **不带前缀**：文件只承载包内辅助角色，不会被外部直接引用（如 converter、assembler），保持简洁即可。
  - 示例：`converter.go`、`dto.go`、`assembler.go`

### Handler 组装

`api/server/<proto>/handler.go` 通过嵌入 `api/controller/` 下的类型聚合方法，不在 handler 内堆叠业务入口实现。新增业务方法在 controller 包中定义，handler 自动获得对应方法，避免单文件随业务增长膨胀。

### pkg 组织

`pkg/` 只承载无业务语义的通用工具。按职能拆包组织：`stringutil/`、`timeutil/`、`sliceutil/`、`safego/` 等，不设大杂烩聚合包。

### init.go 导入

`init.go` 只处理需要 side-effect import 的入口或注册逻辑，按入向 / 出向区分：

- **入向**（被外部触发）：`api/server/*`、`api/job` → 写入 `init.go`。
- **出向被直接 import**：不写。
- **出向无显式 import，但 `init()` 有注册逻辑**（如 `infra/mq`）：写入 `init.go` 以触发注册。
