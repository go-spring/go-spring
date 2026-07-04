---
name: gs
description: 使用 gs CLI 创建或改造 Go-Spring 项目骨架。当用户提到用 gs 初始化项目、生成 layout、添加模块时触发。
---

# gs CLI Skill

用于在 Go-Spring 仓库中调用 `gs` 命令行工具完成项目骨架相关操作。

## 何时使用

- 用户要求「用 gs 创建新项目」「gs init」「生成一个 Go-Spring 项目」。
- 用户要求切换 layout(project / domain 变体)。
- 用户要求向已有工程追加模块或子命令。

## 常用命令

- `gs init <name>`:创建新项目骨架。
- `gs init <name> --layout domain`:使用 domain 变体 layout。
- `gs -h`:查看子命令列表。

## 执行前须知

- `gs` 位于本仓库 `gs/gs/`,发布二进制通过 `go install go-spring.org/gs/gs@latest` 安装。
- 生成后的项目结构约定参见 `layout/`,不要在审阅时质疑目录划分(job vs mqsvr 等属项目约定)。
- 生成命令一律用流式输出,不要缓冲 stdout/stderr。

## 参考

- 源码:`gs/gs/`
- Layout 文档:`layout/README.md`
