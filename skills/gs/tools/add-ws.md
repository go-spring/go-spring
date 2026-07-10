# add-ws 子流程(占位,待补充)

在已有 Go-Spring 项目里新增一个 WebSocket 入口 / 消息处理:controller 承载业务,`wssvr` 承载连接与协议适配。

## 何时使用

- 关键词:加 WebSocket / 加 ws 接口 / 长连接推送

## 触及的 layout 位置

- `api/controller/<bc>/`(协议无关业务)
- `api/server/wssvr/`(连接管理、消息路由、协议适配)

## 工作流程

> **待补充**:此工具尚未编写。填充时明确是否需要 IDL / 手写消息协议;遵守 `docs/agent-rules/domain-rules.md` 分层边界与 `../../SKILL.md` 共用约定。
