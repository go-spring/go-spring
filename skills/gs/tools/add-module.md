# add-module 子流程(占位,待补充)

在已有 Go-Spring 项目里新增一个业务模块 / 限界上下文(BC):贯通 `domain` → `application` → `api/controller` 三层骨架,含错误码与 Bean 注册。

## 何时使用

- 关键词:加一个模块 / 新增业务域 / 新建 BC / 加一个 domain

## 触及的 layout 位置

- `domain/<bc>/`(聚合、领域服务、不变量)
- `application/<bc>/`(用例服务、事务与幂等边界)
- `api/controller/<bc>/`(协议无关业务入口)
- `consts/errno`(新增错误码)

## 工作流程

> **待补充**:此工具尚未编写。填充时遵守 `docs/agent-rules/domain-rules.md` 的四层边界与「不为单实现预设接口」规则,以及 `../../SKILL.md` 共用约定。
