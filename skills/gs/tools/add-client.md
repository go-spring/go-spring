# add-client 子流程(占位,待补充)

在已有 Go-Spring 项目里新增一个 infra 防腐层客户端:隔离外部系统 / RPC 的协议、模型与错误映射。

## 何时使用

- 关键词:接一个外部系统 / 调下游服务 / 加 client / 加防腐层

## 触及的 layout 位置

- `infra/client/<system>/`(防腐层本身,**不**再抽独立 `acl/`)
- `domain`(定义端口 / 领域模型,按真实触发条件才提取接口)

## 工作流程

> **待补充**:此工具尚未编写。填充时遵守 `docs/agent-rules/domain-rules.md`:`infra/client` 即防腐层、负责模型与错误映射,跨 BC/外部系统必须走它;以及 `../../SKILL.md` 共用约定。
