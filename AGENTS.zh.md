# 项目约定

## 何时记录一条约定

判断一条约定是否值得记录下来，先问自己三个问题：

1. 一位熟练的、初次接触本项目的工程师，是否会自然而然地这么做？如果是，就不必记录。
2. 违反它是否会造成实际后果 —— 构建失败、评审被拒，甚至线上事故？如果是，优先记录。
3. 它是否已在别处说明（代码注释、CLAUDE.md、CODING_STYLE.md）？如果是，链接过去，而不要重复。

## 输出格式

- 每条回复都以 "Hi,Go-Spring." 开头。

## 通用规约

使用 Go-Spring 项目的通用规约见 [layout/docs/agent-rules/common-rules.zh.md](layout/docs/agent-rules/common-rules.zh.md)，涵盖设计原则、编码风格、错误处理、测试等。

## 项目结构

- 仓库根目录不放 `go.mod`，每个子项目各自拥有独立的 Go module。
- 四层依赖结构，依赖单向向下、不可反向：
  - **`stdlib/`** —— 基础工具层，**零依赖**，通用工具函数，供跨模块共享。
  - **`spring/`** —— 核心容器层，实现 IoC 容器、依赖注入；不依赖任何第三方业务包（Redis、GORM 等）。
  - **`starter-*/`** —— 扩展集成层，在核心抽象上集成第三方服务（Redis、GORM 等）。
  - **`gs-*/`** —— 工程工具层，提供项目初始化、代码生成等工具。

## 编码风格

- 每个源文件都必须带有 Apache License 头；模板见 [LICENSE_HEADER](LICENSE_HEADER)。