---
name: gs
description: Go-Spring 项目研发托管入口:一句话诉求经 规划(Plan)→ 执行(Execute)→ 交付(Settle)循环,从工具集(起骨架/加接口/重生成/接组件/补测试)中选取组合并落地。在 Go-Spring 项目中开展任一研发任务时触发;由 Plan 判定是否接管,超范围时放手。
version: v0.0.1
---

# /gs Skill

Go-Spring 项目研发的 **PES 控制环**。不是 CLI 外壳,也不是按生命周期排好的流水线,而是:**任何请求先规划,再从独立工具集里选取本次所需能力的组合,执行并验证,最后交付沉淀。**

核心认知:一个请求往往需要**多个工具的组合**,所以无论大小,都先过 PES。工具集保持独立,由 Plan 编排。

## PES 控制环

```
Plan ──确认──▶ Execute ──全绿──▶ Settle
  ▲              │
  └──失败按半径回环─┘
```

| 环节 | 职责 | 文档 |
| --- | --- | --- |
| **Plan** | 判定接不接 + 分解意图 + 选工具 + 排序 + 定验收 | [`plan/plan.md`](plan/plan.md) |
| **Execute** | 按序调用工具 + 内建验证 + 失败局部回环 | [`execute/execute.md`](execute/execute.md) |
| **Settle** | 交付说明 + 沉淀约定 | [`settle/settle.md`](settle/settle.md) |

**进入本 skill,一律先读 `plan/plan.md`**(判定门在里面);不要凭记忆跳过 Plan 直接动手。

## 工具集(Execute 按 Plan 选用)

各工具**逻辑独立**、自带前置检查,可被自由组合。**执行前先读对应文档**,不凭记忆。标「待补充」的为占位工具,骨架已就位、流程待填。

| 工具 | 关键词 | 文档 |
| --- | --- | --- |
| 起项目骨架 | 新建 / 初始化 / `gs init` / 切换 layout | [`tools/scaffold.md`](tools/scaffold.md) |
| 新增业务模块(BC) | 加模块 / 新增业务域 / 新建 BC / 加 domain | [`tools/add-module.md`](tools/add-module.md) 待补充 |
| 新增 HTTP 接口 | 加接口 / 新增 API / 改 IDL 加路由 | [`tools/add-http.md`](tools/add-http.md) |
| 新增 gRPC 接口 | 加 gRPC / 改 proto 加 rpc | [`tools/add-grpc.md`](tools/add-grpc.md) 待补充 |
| 新增 Thrift 接口 | 加 Thrift / 改 thrift IDL | [`tools/add-thrift.md`](tools/add-thrift.md) 待补充 |
| 新增 WebSocket 入口 | 加 ws / 长连接推送 | [`tools/add-ws.md`](tools/add-ws.md) 待补充 |
| 新增定时任务 | 加 job / 加 cron / 后台任务 | [`tools/add-job.md`](tools/add-job.md) 待补充 |
| 新增 MQ 消费/生产 | 加 consumer / 加 producer / 收发消息 | [`tools/add-mq.md`](tools/add-mq.md) 待补充 |
| 新增 infra 防腐层 client | 调下游 / 接外部系统 / 加 client | [`tools/add-client.md`](tools/add-client.md) 待补充 |
| 新增 infra 仓储 repo | 加 repo / 持久化聚合 / 存表 | [`tools/add-repo.md`](tools/add-repo.md) 待补充 |
| 重新生成代码 | 重新生成 / IDL 改了 / `gs-http-gen` / `gs-mock` | [`tools/gen.md`](tools/gen.md) |
| 接入组件(消费 Starter) | 接 Redis / 接 MySQL / 接 Kafka / 加 Starter | [`tools/wire-starter.md`](tools/wire-starter.md) |
| 补测试 / 示例 | 补测试 / 加用例 / 加 example | [`tools/add-test.md`](tools/add-test.md) |
| 创作新 Starter(框架侧) | 新建 starter / 集成新组件到 go-spring | [`tools/new-starter.md`](tools/new-starter.md) 待补充 |

> 构建/测试([`execute/build-test.md`](execute/build-test.md))与报错定位([`execute/fix-compile.md`](execute/fix-compile.md))是 **Execute 的内建验证**,不是可选工具——任何改动后自动执行,Plan 无需显式排入。通用编译/测试报错以 AI 自身调试能力为主;这两份文档只补 go-spring 生态特有的部分(多 module 定位、生成物过期回 `gen` 重生成而非手改、四层依赖边界)。

## 会话状态机

会话进入 PES 后,**PES 状态是默认处理框架**,但不是牢笼:

- **Plan 是可变状态**:用户任何时候可回头调 Plan,调完重新对齐再继续;
- **失败按爆炸半径回环,不推倒重来**:局部失败只在当前步修复重试,已完成步骤不动;仅当失败证伪了 Plan 前提,才回整体重规划;
- **离题旁问不破坏状态**:执行中临时问无关问题,答完回到原进度;
- **保真度可伸缩**:阶段永远走,产物按需生——小需求的 Plan/Settle 可以只是一句话,别为琐事产出规划/交付文档。

## 共用约定(所有环节与工具遵守)

**代码/布局规约(生产代码怎么写、目录怎么分)不在此重复,执行前按需查阅:**

- 编码风格与架构取向(错误包装 `errutil`、IoC 启动期注入、Bean 冲突显式化、Starter 优先、子进程 IO 等):[`coding-style.zh.md`](../../layout/docs/coding-style/coding-style.zh.md)
- 分层边界与目录划分(`job`/`mqsvr`、`api/controller` vs `api/server/<proto>/handler` 等):[`domain-rules.zh.md`](../../layout/docs/agent-rules/domain-rules.zh.md)
- 多 module 结构(仓库根无 `go.mod`;`go build`/`test`/`gofmt` 下钻到子 module):见项目 `CLAUDE.md`

**以下是 skill 自身的执行约定:**

- **上下文优先**:执行前先读 `AGENTS.md` / `CLAUDE.md` / `CODING_STYLE.md` / 目录约定,不凭记忆假设结构。
- **前置检查**:所需二进制在 PATH 中、目标目录状态符合预期、外部服务可达;不满足直接终止。
- **入参校验**:module path、layout、语言、接口名等参数在动手前完成合法性校验,非法值立即报错。
- **冲突检测**:目标目录/文件已存在等冲突直接终止,不覆盖不删除。
- **改动收敛**:每次代码改动后跑 gofmt + `go test`(必要时 `go build`),给出验证结论。
- **变更摘要**:代码/配置/接口/文档改动完成后,输出变更清单 + 验证命令 + 遗留风险。
- **文档同步**:接口、配置、示例的改动必须同步到对应文档,不留事后补。
