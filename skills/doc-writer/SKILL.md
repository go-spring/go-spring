---
name: doc-writer
description: 用于编写、改写、审阅或扩展 Go-Spring 文档。写作前应核对源码、测试、示例和已有文档；当目标文档属于已约束范围时，遵守 Go-Spring 文档写作规范。
---

# Go-Spring 文档编写者

当需要编写、改写、审阅或扩展 Go-Spring 文档时使用本技能，包括新增文档、重写文档、结构调整、术语统一、示例补充、FAQ 和集成文档。

## 工作流程

1. 先判断文档类型：项目概览、快速开始、Guide、Integration、Example、FAQ、Changelog 或贡献文档。
2. 写入 API 名称、配置 key、默认值、行为说明、版本或状态信息前，先从源码、测试、示例或已有文档中核对事实。
3. 当目标文档位于 `docs/0.overview/`、`docs/1.getting-started/`、`docs/2.guides/` 或 `docs/4.integrations/` 时，读取并遵守 `references/writing-style.md`。
4. 其他目录暂不强制套用 `references/writing-style.md`，只保持基本事实准确、术语清晰和结构可读。
5. 保持 `docs/` 作为面向读者的文档目录；AI 角色资料、写作规则、清单和模板放在 `skills/doc-writer/` 下。
6. 默认使用中文写作，除非目标文档已经是英文，或用户明确要求使用其他语言。
7. 结束前检查示例上下文是否足够、术语是否一致、链接是否相关、限制和边界行为是否明确。

## 引用文件

- `references/writing-style.md`：Go-Spring 特定文档目录的详细写作规范，包括规则等级、术语、示例、交叉引用和视觉风格。
