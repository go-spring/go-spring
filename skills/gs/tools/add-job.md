# add-job 子流程(占位,待补充)

在已有 Go-Spring 项目里新增一个定时任务 / 后台 job:业务逻辑走 controller/application,job 入口只做调度与编排。

## 何时使用

- 关键词:加定时任务 / 加 job / 加 cron / 后台任务

## 触及的 layout 位置

- `api/job/`(任务入口与调度注册)
- `application/<bc>/`(用例编排)

## 工作流程

> **待补充**:此工具尚未编写。填充时明确调度方式与幂等边界(幂等责任在 application);遵守 `docs/agent-rules/domain-rules.md` 与 `../../SKILL.md` 共用约定。
