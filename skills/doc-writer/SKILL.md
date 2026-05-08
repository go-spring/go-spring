---
name: doc-writer
description: 用于编写、改写、审阅或扩展 Go-Spring 文档。写作前应核对源码、测试、示例和已有文档；当目标文档属于已约束范围时，遵守 Go-Spring 文档写作规范。
---

# Go-Spring 文档编写者

当需要编写、改写、审阅或扩展 Go-Spring 文档时使用本技能，包括新增文档、重写文档、结构调整、术语统一、示例补充、FAQ 和集成文档。

## 工作流程

### 判断文档类型

先确认目标是项目概览、快速开始、Guide、Integration、Example、FAQ、Changelog 还是贡献文档。类型或结构不明确时，读取 `references/doc-types.md`。

### 核对事实来源

写入 API、配置 key、默认值、行为、版本或状态前，先从源码、测试、示例或已有文档中确认。涉及事实性内容时，读取 `references/fact-checking.md`。

### 选择适用规则

当目标文档位于 `docs/0.overview/`、`docs/1.getting-started/`、`docs/2.guides/` 或 `docs/4.integrations/` 时，读取并遵守 `references/writing-style.md`。其他目录暂不强制套用该规则。

### 按文档目的组织内容

让结构服务于读者任务，避免把所有文档都写成同一种 Guide。默认使用中文写作，除非目标文档已经是英文，或用户明确要求使用其他语言。

### 交付前自检

结束前按 `checklists/final-review.md` 检查示例上下文、术语、链接、限制和边界行为，并说明修改范围和校验情况。

## 引用文件

- `references/writing-style.md`：Go-Spring 特定文档目录的详细写作规范，包括规则等级、术语、示例、交叉引用和视觉风格。
- `references/doc-types.md`：不同文档类型的职责、推荐结构和不应套用的写法。
- `references/fact-checking.md`：事实校验规则，用于避免编造 API、配置、默认值、版本和行为。
- `checklists/final-review.md`：文档交付前的最终审查清单。
