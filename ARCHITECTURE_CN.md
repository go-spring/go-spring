# Go-Spring 架构与边界

[English](ARCHITECTURE.md) | [中文](ARCHITECTURE_CN.md)

这是一份**权威地图**:说明每个顶层目录放什么、**不能**放什么,以及新代码如何被路由到正确的位置。它的职责是防止仓库跑偏——当你不确定某样东西*该放哪*、或*该不该存在*时,以本文档为准。

它是地图,不是百科。各模块的内部设计规则在各自的 `DESIGN.md` 里,本文档只链接、不复述(见 [CLAUDE.md](CLAUDE.zh.md) "何时记录一条约定":链接而非重复)。

## 1. 分层模型

仓库**根目录没有 `go.mod`**。每个子项目拥有自己的 module 和依赖图。模块被组织成若干层,**依赖只能单向流动——永不反向**:

```
   基础层            核心层           集成层                 工具层
 ┌──────────┐   ┌──────────┐   ┌──────────────┐   ┌──────────────┐
 │ stdlib/  │──▶│          │──▶│ starter/     │   │ gs/          │
 │ log/     │   │ spring/  │   │  starter-*   │   │  gs-*        │
 └──────────┘   └──────────┘   └──────────────┘   └──────────────┘
       │              │                │                  │
       └──────────────┴────────────────┴──────────────────┘
                            ▲
                被示例与模板消费(反向永不依赖)
        contrib/   examples/   layout/            (+ website/ docs/ scripts/ skills/)
```

已核实的依赖事实(不可违反):

- `stdlib/` **零三方依赖**——只用标准库。它是一个通用工具库(对 Go 标准库的补齐:
  类型、编解码、集合……),**不含任何能力抽象**。
- `log/` 依赖 `stdlib/`(外加一个 ANTLR 解析器用于其配置语法);它是基础模块,不属于 `spring`。
- `spring/` 依赖 `log/` 和 `stdlib/`,且**不依赖任何三方业务包**(不引 Redis、GORM、Kafka……)。
  除 IoC 容器外,它还以子包形式承载框架的**能力抽象**(接口 + driver 注册表),并按关注面
  分族——`spring/cloud/*`(discovery、loadbalance、resilience、lock、messaging、transaction、
  event、scheduling、batch)、`spring/web/*`(httpsvr、httpclt、httpx、security、session、
  validation、i18n)、`spring/data/*`(cache、repository、migration)、`spring/actuator/*`
  (endpoint、health、podinfo)——其具体后端落在 `starter-*`。`spring/aspect` 与 `gs`/`conf`
  同处根目录,是零依赖、被广泛依赖的核心原语。权威族图见 [spring/DESIGN.md](spring/DESIGN_CN.md)。
- `starter-*` 与 `gs-*` 位于上层,可以引入三方包。
- 下层模块不得 import 上层。`starter` import 另一个 `starter`、或 `spring` import `starter`,都是分层违规。

**内部依赖靠 `go.work` 解析,永不写 `require`。** 对工作区内模块写 `require` 会让 `go mod tidy` 去 proxy 拉取并 404。完整模块清单见 [go.work](go.work) 的 `use` 列表。

## 2. 目录职责矩阵

| 目录 | 层 | 用途(一句话) | 属于这里 | **不**属于这里 | 深入阅读 |
|---|---|---|---|---|---|
| `stdlib/` | 基础 | 零依赖的通用工具(对 Go 标准库的补齐) | 纯 Go 工具——类型、编解码、集合、哈希、文本…… | 任何三方 import;能力抽象 / driver 注册表(它们在 `spring/`);容器/DI 逻辑 | [stdlib/README.md](stdlib/README_CN.md) |
| `log/` | 基础 | 结构化日志模型、配置语法、适配器 | 日志模型、appender、字段编码、日志配置解析器 | 业务日志;对 `spring` 的硬依赖 | [log/DESIGN.md](log/DESIGN_CN.md) |
| `spring/` | 核心 | IoC 容器、依赖注入、应用生命周期、内置 HTTP Server、conf,以及框架的能力抽象 | Bean 模型、注入、启停状态机、配置绑定/刷新、极简 HTTP Server;能力接口 + driver 注册表(cache、lock、discovery、resilience……) | 三方业务包;接真实后端的集成代码;完整 Web 框架(见 §4) | [spring/DESIGN.md](spring/DESIGN_CN.md) |
| `starter/` | 集成 | 每个三方服务/框架一个 module,接入 IoC 容器 | 遵循五形态的 `starter-*` 模块;家族设计指南 | 业务逻辑;部署脚手架;跨 starter 的共享 helper 包 | [starter/DESIGN.md](starter/DESIGN_CN.md) |
| `gs/` | 工具 | 开发工具:脚手架(`gs`)、GUI、代码生成(`gs-http-gen`)、mock(`gs-mock`) | 作用*于*项目的 CLI/codegen/工具 | 运行时框架代码;任何被运行中应用 import 的东西 | [gs/README.md](gs/README.md) |
| `contrib/` | 示例 | 展示三方框架如何按 Go-Spring 方式接线的可运行示例 | 各框架可运行变体;冒烟测试 | 可复用模块(那些应成为 `starter-*`);部署脚手架 | [contrib/DIRECTORY_CONVENTIONS.md](contrib/DIRECTORY_CONVENTIONS.md) |
| `examples/` | 示例 | 仅由已发布 starter 搭建的端到端示例应用 | *消费*框架的参考应用(fullstack、bookman……) | 新框架能力;应用不该复制的代码 | [examples/examples.md](examples/examples.md) |
| `layout/` | 模板 | `gs init` 生成的项目骨架 | 模板文件、agent 规则、各协议 IDL 布局 | 框架实现;任何不该被复制进用户项目的东西 | [layout/DESIGN.en.md](layout/DESIGN.zh.md) |
| `website/` | 站点 | 文档站点**源码**(Node.js) | Markdown 内容、站点配置、资源 | 构建产物(那是 `docs/`) | — |
| `docs/` | 站点 | **已发布**的站点产物(GitHub Pages;含 `CNAME`) | 生成的 HTML/资源 | 手写源码(去 `website/` 改) | — |
| `scripts/` | 运维 | 仓库维护脚本 | 模块检查、发布、历史审计 | 应用运行时代码;单项目构建脚本 | — |
| `skills/` | agent | 随仓库分发的 agent 技能(如 `gs`) | 技能定义 | 运行时框架代码 | — |

## 3. 新代码该放哪?(判定指引)

自上而下,第一个命中者胜出。

1. **它是可运行示例或参考应用、不打算被 import 吗?**
   - 演示某三方框架接线 → `contrib/<framework>/<variant>/`
   - 由既有 starter 搭建的端到端应用 → `examples/`
2. **它是否集成某个特定三方服务/框架**(Redis、GORM、Kafka、某 Web/RPC 框架、配置中心……)?
   → `starter/` 下的 `starter-*` 模块。从
   [starter/DESIGN.md §2](starter/DESIGN_CN.md) 选定形态——它决定生命周期、端口和配置前缀行为。
3. **它是开发期工具**(脚手架、codegen、mock、GUI),作用*于*项目而非运行在项目内?→ `gs/gs-*`。
4. **它是容器 / DI / 生命周期 / 配置 / 内置 HTTP 逻辑,或某个能力抽象**
   (接口 + driver 注册表,如 cache、lock、discovery、resilience),且无三方业务依赖?
   → `spring/`(作为子包)。若它需要三方 import,抽象仍留在 `spring/`,具体后端放进
   `starter`。
5. **它是可复用的通用工具**(类型、编解码、集合……),**零三方依赖**且不涉及框架/能力
   关注点?→ `stdlib/`(若是日志则 `log/`)。
6. **它是文档吗?** 在 `website/` 里写;永不手改 `docs/`。

两个反复出现的陷阱:

- *"我就加个两个 starter 共用的小 helper。"* 不行——跨 starter 的共享 helper 包目前被禁止
  ([starter/DESIGN.md §3](starter/DESIGN_CN.md) "当前容忍重复而非过早抽象")。先重复;提取收敛可能晚点再做。
- *"这个抽象需要 Redis 客户端,我放 stdlib 吧。"* 不行——两处都错。它不是纯工具(它是能力
  抽象,家在 `spring/` 而非 `stdlib/`),且一旦需要三方 import 就不能待在任一基础层。
  正确模式是:**抽象 + driver 注册表放 `spring/`,具体后端放 `starter`**
  (见 `spring/data/cache`、`spring/cloud/lock`、`spring/cloud/discovery`)。

## 4. 范围红线(非目标)

这些是刻意划定的边界。越过它们是跑偏,不是进步。

- **`stdlib/` 保持零依赖。** 基础层的价值在于任何模块都能用它而不继承一张依赖图。哪怕一个三方 import 都会毁掉这个价值。
- **`spring/` 不是 Web 框架。** 内置 HTTP Server 刻意**不**提供框架级 context 对象、参数绑定 / 返回值自动序列化、路由分组或优先级、模板渲染。这些属于 Web 框架 `starter`(gin/echo/hertz……)。见 [OUTLINE.md](OUTLINE.md) 五、"内置 HTTP Server"。
- **`starter-*` 只做集成。** 一个 starter 把*一个*三方服务/框架接入容器和生命周期——不含业务逻辑、部署脚手架、跨 starter 抽象。
- **`contrib/` 和 `examples/` 是示例,不是产品。** 它们只为冒烟测试和集成演示存在。不要加部署脚手架(`build.sh`、`bootstrap.sh`、额外 `script/` 目录);只保留源码 + `smoke-test.sh` / `check.sh` / `gen.sh`。
- **优先框架原生机制;仅在无原生机制时才统一。** 不要在各框架已自带的能力上再套一层 Go-Spring 抽象(如 RPC provider 注册)。理由与当前"有原生 vs 候选"的划分见 [starter/DESIGN.md §3](starter/DESIGN_CN.md)。

## 5. 扩展点是框架的契约

Go-Spring 的存在意义是服务所有团队的全场景;它无法交付一套固定功能就指望正好合用。所以在框架各层——`stdlib/`、`spring/`、`starter/`——**扩展点不是可选项**:

- **每个能力都留一处缝。** 能力抽象定义接口 + driver 注册表,具体行为插在其后。框架没预料到的场景也必须有路进来——否则架构是以"缺失"的方式失效,而非以一个看得见的 bug 失效。
- **内置功能走它对外暴露的同一处缝。** Go-Spring 自己的内置实现必须经由提供给用户的那些扩展点,绝不走特权私有路径——`spring/data/cache` 的 Memory 后端、`spring/cloud/resilience` 的内置策略、starter 各形态,都在消费自己的注册表/接口。若某个内置功能无法经由公开缝表达,那是缝错了,不是内置错了。
- **这是框架层的职责,不是普适要求。** 下游业务代码(`layout/` stamp 出的应用,以及 `examples/` / `contrib/`)反而遵循 YAGNI:仅当真实的第二个场景越过判断线时才留缝(见编码风格文档「可扩展性与扩展点」)。框架的判断线几乎是天生被跨过的;业务应用的则很少。

具体的扩展点形态(driver 注册表、seam 接口、Provider/Contributor、函数式钩子)以及"抽象放 `spring`、后端放 `starter`"的规则,已在上文 §2–§3 与 [starter/DESIGN.md §2](starter/DESIGN_CN.md) 中登记。

## 6. 相关文档

- [CLAUDE.md](CLAUDE.zh.md) —— 何时记录约定;输出与编码规则。
- [starter/DESIGN.md](starter/DESIGN_CN.md) —— 五种 starter 形态及全部横切约束(仓库中最深的规则集)。
- [contrib/DIRECTORY_CONVENTIONS.md](contrib/DIRECTORY_CONVENTIONS.md) —— contrib 示例的布局与命名。
- [spring/DESIGN.md](spring/DESIGN_CN.md)、[log/DESIGN.md](log/DESIGN_CN.md)、[layout/DESIGN.zh.md](layout/DESIGN.zh.md) —— 各模块内部设计。
- [layout/docs/agent-rules/common-rules.zh.md](layout/docs/agent-rules/common-rules.zh.md) —— 基于 Go-Spring 的项目共享的设计/编码/测试规则。
- [MANIFESTO.md](MANIFESTO.md) —— 长期的 "Process as Code" 方向。
