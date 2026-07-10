# add-repo 子流程(占位,待补充)

在已有 Go-Spring 项目里新增一个 infra 仓储:持久化聚合,隔离 ORM model,不把它泄漏给上层。

## 何时使用

- 关键词:加一个 repo / 加仓储 / 存一张表 / 持久化聚合

## 触及的 layout 位置

- `infra/repo/`(仓储实现,ORM model 不外泄)
- `domain`(仓储接口 / 聚合,按真实触发条件才提取接口)

## 工作流程

> **待补充**:此工具尚未编写。填充时遵守 `docs/agent-rules/domain-rules.md`:`infra/repo` 不泄漏 ORM model、可用唯一索引/乐观锁兜底幂等;集成测试连真实 DB;以及 `../../SKILL.md` 共用约定。
