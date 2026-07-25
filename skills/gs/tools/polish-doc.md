# polish-doc — 文档润色

对 Go-Spring 项目文档进行 AI 润色优化。覆盖 README、开发指南、接口文档、变更日志、示例说明等。

**核心目标：去掉 AI 味，说人话。**

## 何时使用

- 用户说「润色 README」「优化文档」「polish doc」「改写这段」
- 用户选中某段文字要求改写
- 用户说「帮我写一份 README」「生成接口文档」

## 前置检查

- 目标文档存在（润色已有文档时），不存在则提示终止。
- 目标文档在当前项目范围内。
- 生成新文档时先确认项目根目录和已有文档约定（`docs/`、`README.md` 等）。

## 收集信息

缺失时用 `AskUserQuestion` 补问：

- **目标文档**：文件路径或用户直接提供的文本。
- **润色范围**：整篇 / 指定段落。
- **语言偏好**：中文 / 英文 / 双语。
- **写入方式**：直接写文件（默认）或先出建议等确认。

## 润色总则

1. **技术事实不动**：API 名、配置 key、命令行参数、代码示例原样保留。
2. **术语统一**：Go-Spring 官方术语（Starter、IoC 容器、Bean、注入、自动装配），不引入 Spring Boot/Spring Framework 术语。
3. **说人话**：像工程师写给工程师看，不像论文也不像营销文案。
4. **结构优化**：长段落拆分、缺标题补上、重复内容合并。
5. **中英文混排**：中文与英文/数字之间加空格；代码和文件名用反引号包裹。

---

## 中文润色：去 AI 味检查清单

### 禁用套话（直接删除，不替换）

| 套话 | 处理 |
|---|---|
| 在当今 / 随着…的发展 / 众所周知 | 直接删，从主语开始 |
| 本文将介绍/探讨/阐述 | 删「本文」，直接说事 |
| 值得注意的是 / 需要强调的是 / 不可忽视的是 | 直接删。重要的东西不需要喊 |
| 总而言之 / 综上所述 / 总的来说 | 删。结尾直接给结论 |
| 首先…其次…再次…最后… | 用小标题或列表替代 |
| 因此，我们可以看出 / 我们可以发现 | 删「我们可以」。直接陈述 |
| 在…的过程中 / 在…的情况下 | 删，说动作本身 |
| 这一点至关重要 / 必须指出的是 | 删 |

### 禁用句式

| 句式 | 改为 |
|---|---|
| 对 X 进行处理 | 处理 X |
| 对 X 进行分析 | 分析 X |
| 对 X 进行配置 | 配置 X |
| 进行/加以/予以 + 动词 | 直接用动词 |
| 用户应确保… | 确保…（或用「你」） |

### 中文风格规则

- **主语开头**，不用「因此/然而/此外」起句（偶尔一次可以，连续出现是 AI 指纹）。
- **「的」不过度**：拆开「X 的 Y 的 Z」链条。
- **用「你」不用「用户」** 称呼读者。
- **成语最多一段一个**，不成语堆砌（相辅相成/不可或缺/举足轻重是 AI 高频词）。
- **长短句交替**：8 字句和 30 字句混排，不要通篇等长。
- **被动转主动**：「配置被加载」→「容器加载配置」。
- **不用「我们」自称** 技术文档里（「我们来看…」→「下面看…」）。

---

## 英文润色：去 AI 味检查清单

### 禁用词/短语（直接删）

| AI 套话 | 处理 |
|---|---|
| It is important to note that… | 删，直接说 |
| It is worth mentioning that… | 删 |
| It should be noted that… | 删 |
| It is essential to understand that… | 删，直接解释 |
| Furthermore / Moreover | Also 或删（邻句自带递进） |
| In addition (to this) | Also 或合并成一句 |
| Additionally | Also 或删 |
| Consequently / As a result | So 或重构因果 |
| Nevertheless / Nonetheless | But 或 However（少用） |
| In conclusion | 删。最后一段就是结论 |
| In order to | To |
| Due to the fact that | Because |
| Has the ability to | Can |
| Make a decision | Decide |
| Conduct an analysis of | Analyze |
| Leverage / Utilize | Use |
| Facilitate | Enable / help / 或具体动词 |
| Delve into / Dive into | 删或用 describe |
| Underscores / Highlights | Shows |
| A myriad of / A plethora of | Many 或给数字 |

### 英文风格规则

- **祈使句优先**：「Set the timeout to 5s」不用「The timeout should be set to 5s」。
- **缩写自然用**：can't / don't / it's，不过度正式。
- **删掉可有可无的 that**：「The method that is called」→「The method called」。
- **主动语态默认**：「The server returns an error」不用「An error is returned」。
- **段落不等长**：一句段落和七句段落混排，不要统一 3-5 句。
- **一句一个意思**：不用 while/although/whereas 串长句。
- **不自我引用**：「This document will…」「In this section, we will…」→ 删。
- **You 和 We 自然用**：读者是 you，作者偶尔用 we，不要混成 one/the user。
- **不用 please**：文档不是邮件。「Please ensure…」→「Ensure…」。
- **删绝对副词**：obviously / clearly / undoubtedly / naturally — 显然的东西不需要说显然。

---

## 文档类型专项

| 类型 | 检查重点 |
|---|---|
| README | 简介是否一句话说清、Quick Start 是否可执行、目录结构是否完整 |
| 开发指南 | 前置条件→本地运行→目录约定→开发流程 四段式 |
| 接口文档 | URL/方法/入参/出参/错误码齐全、示例可直接跑 |
| 变更日志 | 按版本组织、每条可追溯、breaking change 显式标注 |
| 示例代码 | 编译可过、运行可复现 |

## 工作流程

### 1. 读取文档

```
Read <目标文件>
```

文件不存在则终止。用户直接给文本则跳过。

### 2. 快速诊断

扫描后告诉用户：
- 文档类型
- 当前语言
- 发现的 AI 味问题数量（套话数、长难句数、术语混用数）
- 确认润色方向

### 3. 执行润色

逐段处理，保留 Markdown 格式。代码块只改注释错别字，不改代码。配置示例和命令不动。

### 4. 输出结果

1. **变更清单**：做了哪些改动（「删 5 处套话」「拆 3 个长句」「统一 8 处术语」）。
2. **Before/After**：选 3-5 处关键对比。
3. **完整文本**：直接写入时只告知改动数量。

### 5. 写入

用户确认后写入，保留原文件权限。新文档用 `Write` 写入约定路径。

## 完成后输出

- 改动文件路径
- 改动统计（增删行数、改动段落数）
- 遗留问题（某段技术细节不确定、术语需确认等）
- 是否需要同步更新关联文档

## 关键约束

- **禁止改代码块内容**（注释错别字除外）。
- **禁止改 API 名、配置 key、命令行参数**。
- **禁止引入外部框架术语** 替代 Go-Spring 术语。
- **禁止删许可证、版权、安全警告**。
- 生成新文档遵循项目已有约定，不发明新结构。