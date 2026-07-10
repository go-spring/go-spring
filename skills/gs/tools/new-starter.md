# new-starter 子流程(占位,待补充)

在 **go-spring 仓库自身**创作一个新的 `starter-<name>` 集成模块(如 starter-kafka),把第三方服务按核心抽象接入。区别于 `wire-starter`(在用户项目里**消费**已有 Starter)。

## 何时使用

- 关键词:新建 starter / 创作 starter / 集成 XXX 组件 / 加一个 starter 模块
- 受众:go-spring 框架维护者(不是 layout 实例化后的用户项目)

## 触及的位置

- 仓库根 `starter-<name>/`(独立 Go module,自带 `go.mod`)
- 参考现有同形态 starter(`starter-gin` / `starter-go-redis` / `starter-gorm-mysql`)的目录与 Bean 注册方式
- 发布/注册:`go-import` 与 release 脚本需登记新 starter

## 工作流程

> **待补充**:此工具尚未编写。填充时对齐现有 starter 结构,遵守 `../../SKILL.md` 共用约定与 coding-style(启动期注入、Bean 冲突显式化、Apache License header)。
