# 项目 AI 协作说明

本项目由 Go-Spring 脚手架生成,形态由 `gs.json` 中的 `form` 字段决定(默认为 `domain`)。本文件为项目级 `CLAUDE.md`,给 AI 协作者提供必须遵守的边界与常用上下文。详细规范以 `docs/` 为准,本文件只做约束汇总与索引。

## 权威文档

- `docs/project-layout.md` — 顶层目录职责(`conf/`、`idl/`、`internal/`、`main.go`、`gs.json` 等)。
- `docs/domain-layout.md` — `internal/` 领域分层(api / application / domain / infra)、依赖方向、错误处理、事务与幂等、测试策略、AI 协作模板。

修改代码或写方案时,**必须**先按上述文档确认所在层与依赖方向,不要凭直觉推断目录用途。

## 项目结构硬约束

- 顶层实际目录为 `idl/`、`internal/`,文档中的 `-<form>` 后缀仅用于标注形态,**不要**生成带后缀的目录。
- `main.go` 只做 side-effect import 与 IoC 启动,**禁止**承载业务装配代码;注册逻辑落到各层 `init.go`。
- IDL 生成产物统一落 `idl/<protocol>/gen/`,**禁止**手工修改生成文件,**禁止**把生成代码搬进 `internal/`。
- `internal/` 内部分层随 `gs.json` 的形态而定;domain 形态下遵循 `docs/domain-layout.md`。

## Domain 形态四层边界(默认形态)

依赖方向单向:`api → application → domain ← infra`,`pkg` / `consts` 可被任意层引用,不得反向依赖业务包。

| 层 | 允许 | 禁止 |
|---|---|---|
| `api` | 依赖 `application`、`pkg`、`consts`、协议生成代码 | 依赖 `domain` / `infra`;返回 `domain` 对象;写业务规则 |
| `application` | 依赖 `domain`、`infra`、同 BC 内其他 application service(单向无环)、`pkg`、`consts` | 依赖协议生成代码 / `api/server`;实现核心业务规则;把外部模型传入 `domain` |
| `domain` | 自身包、`pkg`、`consts/errno`、`errutil` | 依赖 `api` / `application` / `infra` / ORM / RPC SDK / IDL / DTO |
| `infra` | 依赖 `domain`、`pkg`、`consts`、外部 SDK | 依赖 `api`;编排业务用例;决定业务规则 |

补充要点:

- `api/controller/<biz>/` 承载协议无关的业务入口;`api/server/<proto>/handler.go` 通过嵌入 controller 聚合方法,**不要**在 handler 里写方法体。
- MQ 消费入口属于 `api/server/mqsvr`;MQ producer 属于 `infra/mq`。
- `infra/client/<system>/` 本身即防腐层,**不要**建议再抽一层独立的 `acl/`。
- 同 BC 内子域协作走 `application` service 单向调用;跨 BC 或外部系统**必须**走 `infra/client/`。
- 默认**不**为单实现依赖预设接口;只有出现多实现、读写分离、缓存装饰等真实触发条件时,才提取接口。

## 错误处理

- **禁止**用 `errors.New` / `fmt.Errorf` 直接构造错误,统一走 `errutil`。
- 业务语义补充用 `errutil.Explain(err, "...")`;调用链跟踪用 `errutil.Stack(err, "Xxx")`。
- 错误码在 `consts/errno` 集中定义;`errutil` 与 `errno` 关注点正交,不需要互相配合。
- `domain` **禁止**依赖 DB / RPC / SDK 的错误类型;`api` 层只做协议映射,不再补写业务决策。

## 事务、幂等与领域事件

- 一个 `application` service 方法即一个事务单元;**禁止**把事务对象泄漏到 `domain` 或跨 service 传递。
- 幂等责任在 `application`(按业务 ID / 请求 ID 校验);`infra/repo` 可用唯一索引 / 乐观锁兜底。
- 跨聚合 / 跨 BC **不**做分布式事务,通过领域事件或补偿任务达到最终一致。
- 领域事件由聚合根产生,`application` 在事务边界内收集,持久化成功后发布(发布侧由项目按需接入项目事件机制,当前脚手架属于待补实现,不是设计矛盾)。

## 测试策略

- `domain` — 纯单元测试,直接构造聚合验证不变量。
- `application` — 单元测试,用 Go-Spring Mock 替换 `infra` 依赖;**不要**为了测试给所有依赖预留接口。
- `infra/repo` — 集成测试,连真实 DB / 容器化中间件。
- `infra/client` — 契约测试,重点验证模型与错误映射。
- `api` — 端到端测试,启动完整 server 走协议入口。

## 命名与组织

- 包名全小写,包名不要与包内类型重名。
- 会被跨包引用的类型所在文件带领域前缀(`order_controller.go`、`order_service.go`);包内辅助文件(`converter.go`、`assembler.go`、`dto.go`)不带前缀。
- `pkg/` 只承载无业务语义的通用工具,按职能拆包(`stringutil/`、`timeutil/`、`safego/`);**禁止**新增 `common/` / `goutil/` / `helper/` 聚合包。
- Bean 就近声明,不集中到统一装配文件;初始化顺序交给 IoC 容器,**禁止**用包级单例或全局变量长期持有依赖。

## AI 协作硬规则

- 涉及聚合边界、状态机、不变量、跨 BC 协作方式时,**必须**先与人对齐,不得替团队拍板。
- 新增接口、抽象层、`manager` / `processor` 类结构默认**禁止**,除非已经给出真实触发条件。
- converter / assembler / DTO / 测试样例 / 依赖检查这类机械工作可放宽,但仍需明确输入输出。
- 修改代码前,若业务规则、错误码、事务边界、幂等键或外部依赖不完整,**必须先提问,不得自行编造**。
- 大范围重构分阶段执行:先输出影响范围与迁移步骤,再小步修改并跑测试,**禁止**一次改穿多层。
- 交付时必须说明:改了哪些文件、规则落在 `domain` 的哪里、`application` 做了哪些编排、是否新增接口及其触发条件、测试运行结果。

## 落地检查清单(提交前自检)

- 业务规则集中在 `domain`,没有散落到 `api` / `application`。
- `api` 只做协议适配、参数校验、DTO 转换。
- `application` 表达一个用例并拥有事务与幂等边界。
- `infra/client` 隔离了外部协议、模型和错误;`infra/repo` 没把 ORM model 泄漏给上层。
- 错误用 `errutil.Explain` / `errutil.Stack` 保留上下文,错误码走 `consts/errno`。
- 没有无触发条件的接口、`manager`、`common` 包和过度抽象。
- 测试覆盖了 `domain` 不变量,以及关键编排与基础设施语义。
