<!-- # Go-Spring 架构与边界

[English](ARCHITECTURE.md) | [中文](ARCHITECTURE_CN.md)

这是一份**权威地图**:说明每个顶层目录放什么、**不能**放什么,以及新代码如何被路由到正确的位置。它的职责是防止仓库跑偏——当你不确定某样东西*该放哪*、或*该不该存在*时,以本文档为准。

它是地图,不是百科。各模块的内部设计规则在各自的 `DESIGN.md` 里,本文档只链接、不复述(见 [CLAUDE.md](CLAUDE.zh.md) "何时记录一条约定":链接而非重复)。

## 1. 分层模型

仓库**根目录没有 `go.mod`**。每个子项目拥有自己的 module 和依赖图。模块被组织成若干层,**依赖只能单向流动——永不反向**:

```
   基础层            核心层          生态层            集成层            工具层
 ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────────┐   ┌─────────┐
 │ stdlib/  │──▶│          │──▶│  cloud/  │──▶│   starter/   │   │   gs/   │
 │ log/     │   │ spring/  │   │          │   │  starter-*   │   │  gs-*   │
 └──────────┘   └──────────┘   └──────────┘   └──────────────┘   └─────────┘
       │              │               │                │
       └──────────────┴───────────────┴────────────────┘
                            ▲
                被示例与模板消费(反向永不依赖)
        contrib/   examples/   layout/            (+ website/ docs/ scripts/ skills/)
```

已核实的依赖事实(不可违反):

- `stdlib/` **零三方依赖**——只用标准库。它是一个通用工具库(对 Go 标准库的补齐:
  类型、编解码、集合……);工具之外,它也承载**无生态依赖的纯语义件**(`httpsvr`、`httpclt`)。
  新增子包需**同时满足三条件**:该模式在仓内出现两处以上(是提取,不是预埋)、
  各 helper 共享同一动词(一包一职,不做杂物抽屉)、标准库确实没有;工具包命名指向
  它包装的标准库包(`timeutil`→`time`、`md5util`→`md5`、`randutil`→`crypto/rand`)。
- `log/` 依赖 `stdlib/`(外加一个 ANTLR 解析器用于其配置语法);它是基础模块,不属于 `spring`。
- `spring/` 只依赖 `log/` 和 `stdlib/`,且**只含纯核心**:`gs`(IoC 容器、生命周期)与
  `conf`(分层配置引擎)。不含三方业务包,不含能力家族。
- `cloud/`(模块 `go-spring.org/cloud`)是**生态抽象库**:治理(resilience/fault/traffic)、
  discovery、loadbalance、cache、repository、migration、i18n、validation、
  event/scheduling/batch、lock、messaging、transaction、actuator……它只依赖 `stdlib`/`log`,
  **不 import 任何 spring 包**——整个库可脱离容器使用。原先与这些抽象同处的 gs 接线
  已外移进 starter(如 `starter-cache`);治理规则源是个例外——file/http 源已内置于
  `cloud/governance`,原先的 `starter-governance-file` 模块已删除。
- `starter-*` 与 `gs-*` 位于上层,可以引入三方包。
- 下层模块不得 import 上层。`starter` import 另一个 `starter`、或 `spring`/`cloud` import
  `starter`,都是分层违规。全仓口径:**抽象进 cloud,三方 SDK 与 gs 接线进 starter,
  纯语义进 stdlib**。

**内部依赖靠 `go.work` 解析,永不写 `require`。** 对工作区内模块写 `require` 会让 `go mod tidy` 去 proxy 拉取并 404。完整模块清单见 [go.work](go.work) 的 `use` 列表。

## 2. 目录职责矩阵

| 目录 | 层 | 用途(一句话) | 属于这里 | **不**属于这里 | 深入阅读 |
|---|---|---|---|---|---|
| `stdlib/` | 基础 | 零依赖的通用工具(对 Go 标准库的补齐) | 纯 Go 工具——类型、编解码、集合、哈希、文本…… | 任何三方 import;能力抽象 / driver 注册表(它们在 `spring/`);容器/DI 逻辑 | [stdlib/README.md](stdlib/README_CN.md) |
| `log/` | 基础 | 结构化日志模型、配置语法、适配器 | 日志模型、appender、字段编码、日志配置解析器 | 业务日志;对 `spring` 的硬依赖 | [log/DESIGN.md](log/README_CN.md) |
| `spring/` | 核心 | IoC 容器、依赖注入、应用生命周期、分层配置引擎——纯核心(`gs` + `conf`) | Bean 模型、注入、启停状态机、配置绑定/刷新 | 三方业务包;能力抽象(在 `cloud/`);接真实后端的集成代码 | [spring/DESIGN.md](spring/README_CN.md) |
| `cloud/` | 生态 | 容器无关的能力抽象:治理、discovery、缓存、repository、i18n/validation…… | 可脱离容器使用的生态接口 + driver 缝 | 任何 spring import;三方 SDK;gs 接线(归 starter) | [cloud/](cloud/) 各家族文档 |
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
4. **它是容器 / DI / 生命周期 / 配置逻辑?** → `spring/`(`gs`/`conf` 的子包)。
   **它是能力抽象**(接口 + driver 缝,如 cache、治理、discovery)且无三方业务依赖?
   → `cloud/`。若它需要三方 import,抽象留在 `cloud`,具体后端与 gs 接线放进 `starter`。
5. **它是可复用的通用工具**(类型、编解码、集合……),**零三方依赖**且不涉及框架/能力
   关注点?→ `stdlib/`(若是日志则 `log/`)。
6. **它是文档吗?** 在 `website/` 里写;永不手改 `docs/`。

两个反复出现的陷阱:

- *"我就加个两个 starter 共用的小 helper。"* 不行——跨 starter 的共享 helper 包目前被禁止
  ([starter/DESIGN.md §3](starter/DESIGN_CN.md) "当前容忍重复而非过早抽象")。先重复;提取收敛可能晚点再做。
- *"这个抽象需要 Redis 客户端,我放 stdlib 吧。"* 不行——两处都错。它不是纯工具(它是能力
  抽象,家在 `cloud/` 而非 `stdlib/`),且一旦需要三方 import 就不能待在任一基础层。
  正确模式是:**抽象 + driver 缝放 `cloud/`,具体后端与 gs 接线放 `starter`**
  (见 `cloud/cache`、`cloud/lock`、`cloud/discovery`)。

## 4. 范围红线(非目标)

这些是刻意划定的边界。越过它们是跑偏,不是进步。

- **`stdlib/` 保持零依赖。** 基础层的价值在于任何模块都能用它而不继承一张依赖图。哪怕一个三方 import 都会毁掉这个价值。
- **`spring/` 不是 Web 框架。** 内置 HTTP Server 刻意**不**提供框架级 context 对象、参数绑定 / 返回值自动序列化、路由分组或优先级、模板渲染。这些属于 Web 框架 `starter`(gin/echo/hertz……)。见 [OUTLINE.md](OUTLINE.md) 五、"内置 HTTP Server"。
- **`starter-*` 只做集成。** 一个 starter 把*一个*三方服务/框架接入容器和生命周期——不含业务逻辑、部署脚手架、跨 starter 抽象。
- **`contrib/` 和 `examples/` 是示例,不是产品。** 它们只为冒烟测试和集成演示存在。不要加部署脚手架(`build.sh`、`bootstrap.sh`、额外 `script/` 目录);只保留源码 + `smoke-test.sh` / `check.sh` / `gen.sh`。
- **优先框架原生机制;仅在无原生机制时才统一。** 不要在各框架已自带的能力上再套一层 Go-Spring 抽象(如 RPC provider 注册)。理由与当前"有原生 vs 候选"的划分见 [starter/DESIGN.md §3](starter/DESIGN_CN.md)。

以下为已拍板**不做**的项——勿再提议:

- `@Cacheable` 注解缓存 + 多级缓存 L1/L2、region/zone 多级亲和(2026-08-27)。
- 按生态理由不做:分库分表、Cloud Bus 通用化、batch skip/flow、集群流控 / 热点参数、actuator `/shutdown` `/mappings`、灰度发布平台、持久化调度 / misfire、http 拉取配置源。
- 筛选标准 = 生态过时不做、重复其他生态不做、只做与 Go 生态相关的。
- cloud 审计判定**「不动」**(勿再提):`health.Indicator` 四字段(`Name` / `Probe` / `Groups` / `Optional`)、podinfo 目录归属、`traffic.Propagator` 可定制 key、resilience 双 registry 不做类型合并、contract 的 HTTP 纯形状(2026-08-26)。

## 5. 扩展点是框架的契约

Go-Spring 的存在意义是服务所有团队的全场景;它无法交付一套固定功能就指望正好合用。所以在框架各层——`stdlib/`、`spring/`、`starter/`——**扩展点不是可选项**:

- **每个能力都留一处缝。** 能力抽象定义接口 + driver 注册表,具体行为插在其后。框架没预料到的场景也必须有路进来——否则架构是以"缺失"的方式失效,而非以一个看得见的 bug 失效。
- **内置功能走它对外暴露的同一处缝。** Go-Spring 自己的内置实现必须经由提供给用户的那些扩展点,绝不走特权私有路径——`cloud/cache` 的 Memory 后端、`cloud/resilience` 的内置策略、starter 各形态,都在消费自己的注册表/接口。若某个内置功能无法经由公开缝表达,那是缝错了,不是内置错了。
- **扩展是一层封装,不是表里一行。** 尽量用洋葱模型:包住你要扩展的东西(`WrapClientExecutor(inner, ...)`、`Observe(inner Driver, ...)`、一条中间件链),而不是往全局 map 里注册一个名字,也不是让所有调用点都去写同一个包级状态对象。层靠**嵌套**组合——retry → 熔断 → 隔离 → 观测,每层只认识它包住的那个接口,各自持有自己累积的状态:那份状态的生命周期就是被包对象的生命周期,所以没有全局的东西要重置,同进程内两个实例也不可能互相污染。注册表仍有它的位置,那是**选择**机制(配置值挑一个实现);但它交出来的必须是一层携带行为与状态的封装,而不是一个让行为往里写的全局物。这条规则在客户端侧的落地就是掏空的客户端链条——库交付无 hook 的具体类型时,客户端嵌入一个 `Inner*` 接口(身份层套治理层套原生适配层),而不是把原始实例私藏;见 [starter/DESIGN.md §2.2](starter/DESIGN.md)。
- **层持有自己的状态,但它可以读一份共享的仪器集。** 上一条说的是层累积的**状态**——那份状态的生命周期就是被包对象的生命周期。**仪器**是另一回事:OTel SDK 本来就按 name/description/unit/kind 在进程范围内给仪器建索引,所以每实例一份副本最好是"不可见",而对 observable gauge 来说则是**实打实的错**。因此层解析的是**每进程一份、解析后不可变**的共享仪器集,同时仍然持有自己累积的每一份状态。配套的规则(gauge 怎么注册、谁来反注册)在 [starter/DESIGN.md §3](starter/DESIGN_CN.md)。
- **一层只能看见穿过它所包之缝的东西,所以要包住所有路径都经过的最低那层。** 若某条路径绕过了接口——库内部的自动重试、一次不再进入 `Register` 的自愈重注册——那么在接口外面包的这一层会静默漏掉它,而"静默漏掉覆盖"比没有这一层更糟。要么包住两条路径共同的漏斗,要么把这次绕过当作"缝的位置不对"的证据——与"若某个内置功能无法经由公开缝表达,那是缝错了,不是内置错了"同一个判据。
- **这是框架层的职责,不是普适要求。** 下游业务代码(`layout/` stamp 出的应用,以及 `examples/` / `contrib/`)反而遵循 YAGNI:仅当真实的第二个场景越过判断线时才留缝(见编码风格文档「可扩展性与扩展点」)。框架的判断线几乎是天生被跨过的;业务应用的则很少。
- **聚合 starter 是让这条保持诚实的消费者。** 公司基线 starter(`starter-luohua`,DESIGN §2.6)通过*同样的这些缝*组合其它 starter,把整支舰队再基线到一家组织的约定 —— 它是每条缝的第一个外部消费者,所以"只有内置能用、外部接不进"的缝,在聚合 starter 需要它的那一刻就会被发现。

具体的扩展点形态(driver 注册表、seam 接口、Provider/Contributor、函数式钩子)以及"抽象放 `spring`、后端放 `starter`"的规则,已在上文 §2–§3 与 [starter/DESIGN.md §2](starter/DESIGN_CN.md) 中登记。

## 6. 设计原则

以下规则决定*框架被允许做什么*,独立于任何单个模块。各模块的机制细节在其自己的文档里;本节收拢横切的拍板与判据。

### 6.1 容器只做装配

IoC 容器只做装配,不做裁决。评估一个扩展点时问:它解决的是**使用者**的真问题,还是前面某个设计决定制造的问题?三级缓存解循环依赖 = 解决按类型注入自造的问题 → 不正当;家族收集点 = 真问题 → 正当。

- **不支持 `@Primary` / 优先级标记。** 多候选用「注入按名优先」+ 条件(`OnMissingBean` / `OnSingleBean` / `OnProperty`)表达,它决定「bean 存不存在」而非「已有的几个谁赢」。别提议容器级 `@Primary`、或「按类型注入多 bean 自动选一个」——答:注入点名,或加条件。
- **同一立场直接否决:拒绝运行时扫描、编译期 DI、隐式接口索引。** 它们都是「容器只做装配、不做裁决」的实例。
- **不使用 `autoconfig.exclude`。** starter 一律配置键条件注册、默认不启用(不配置 = 不装配)。没有 exclude / 黑名单 / 禁用清单机制;新 starter 注册必须挂配置键条件,禁止 `init()` 无条件 `Provide`,要默认启用的组件需用户明确认可 + 显式 `enabled` 开关(pprof 先例)。
- **starter 提供默认 bean 时必须挂 `OnMissingBean` / `OnProperty` 条件。**

### 6.2 洋葱模型的应用

§5 已把洋葱写成框架的扩展性契约。落地它的家族:`cloud/lock.Observe`、`observability.WrapClientExecutor` / `WrapServerExecutor`、`fault.WrapClientExecutor`。

- **已按本原则处死(勿复活):** `actuator/endpoint` 的 `serving` 全局信号(`MarkServing()` / `IsServing()`)已整体删除,勿再以「启动检测」名义加回;无人收集的 `*endpoint.Endpoint` bean 就是不被装配、保持静默(2026-10-03)。

### 6.3 方法级横切

方法级横切(事务 / 安全这类「包住业务方法」的 concern)不建共享拦截器协议包:各家族自带最小装饰器形状,组合靠普通函数嵌套(中间件模式)——`cloud/experimental/aspect` 已为此删除。

- 别再提议任何「统一拦截器链 / AOP 协议」包(含未来方法级治理 / 幂等 / 方法级锁场景——出现就让那个家族自带装饰器)。
- 新写方法级横切时照抄 `GlobalTransactional` 的形状与文档措辞(doc 里明确 "deliberately no shared interceptor-chain protocol")。
- 评估一个抽象是否该存在时先查实际使用形态(调用处数、参数个数),别信文档宣称的组合故事。

### 6.4 生态主线:开箱即用 + 可特化

go-spring 是完整、开箱即用的生态(不接任何公司扩展也能独立自用),且被设计成可通过扩展整体特化成某家公司自己的生态——不 fork、不写死成某一家的默认;所有内容围绕这一原则设计与实现(2026-09-06)。

- 每层恒成立的不变式(递归贯穿 框架 → 公司伞包 → app):谁都有默认、谁的默认都从公开缝进;谁的默认都能被更上层显式覆盖(默认一律 `OnMissingBean` 让步 + `OnProperty` 门控,app 显式 bean/键即覆盖);契约在中性层、默认可替换层。
- 一句话:层可以多,但每一层只做同一件事——给默认 + 留缝。
- 标准组件必留自定义位;默认实现也必须走对外暴露的缝,绝不走私有特权路径。
- 每实现一个默认 / 每落一个缝,标注「标准组件是哪个 + 公司自定义位留哪」(三态:框架自有 / 公司必填 / 配置声明)。
- 共存过渡:凡跨进程边界的能力,若公司有旧约定,必须能「入站认旧 + 出站写旧」,业务只读中性的进程内契约、零改动。
- 抽象判据(三缺一不抽象):能力是否 (a) 公司有旧约定 ∧ (b) 会新旧共存 ∧ (c) 有真实中性契约;否则是预构抽象(本仓否 `aspect` / 共享 interceptor 即此)。
- gold standard 形状:中性契约(业务读什么)+ 可整体替换实现(注册 / `LastWriteWins` / `OnMissingBean` / config 选),边界只做旧↔新翻译。
- 每个默认 / 扩展必须带实际、可观察、可验证的行为;纯别名 / 空壳 = 惰性 → 问题发现不了。

### 6.5 封装成熟组件

尽量封装成熟组件,不自己搞;只有某领域存在多个可互换组件时才维护一层抽象(2026-09-01)。

- 抽象层立项条件 = 至少两个可互换实现(或明确的多后端路线图);只有单后端时直接封装那个组件。
- 「go-spring 自研 X」需论证该领域没有可用的成熟件;成熟组件久经验证的边界(方言 / 并发 / 锁)自研要重付学费。
- 云原生取舍:注册中心只做注册、不解析是可接受的(不必为 etcd / consul / nacos / zk 补 Discovery 后端);gRPC / thrift 协议建设不追求完善,倾向依赖成熟框架(dubbo-go、kitex 等自带完整 RPC 生态)(2026-08-11)。
- **框架自带治理生态的组件不接 go-spring 治理**(2026-09-12)。判据 = 该组件是否自带一套治理 / 观测生态(接入会重复或打架);看对应 starter 是否暴露框架自己的 observer / registry / middleware / filter 配置块。
  - **不接**(自带治理生态):dubbo、trpc、kitex、hertz、kratos、go-zero、goframe。
  - **接**(薄封装,框架侧无治理层):gin、echo、grpc、thrift、http-server、websocket。
  - 别再提议给 dubbo 入站侧、或给 trpc / kitex / hertz / kratos / go-zero / goframe 接治理。
  - 边界:本决策只约束「还要不要往里加」,**不追溯已有的**——dubbo 出站 timeout/retries 桥接是已做完且有测试的既有事实,保留;`fault` 在这些组件上的既有接入同理(2026-09-12)。
  - `starter-websocket` 只提供配置好的 `*websocket.Upgrader` bean、不拥有 server,升级请求走应用自己选的 HTTP 服务器。

### 6.6 稳定性五件套

稳定性五件套 = 生态体系里每个组件必须具备的稳定性基线:①可观测 ②服务发现 ③负载均衡 ④放火 ⑤服务治理(2026-10-01)。

- 形状 = 两元 + 三服务:①④ 都不改正常路径行为(①是眼睛、④是手,且没有①则④无法验证);②③⑤ 是每一跳的寻址与保护。
- 评审组件只问三句:能被看见吗?能被放火吗?能被治理吗?——①要组件暴露观测点、④要暴露注入点(合起来 = 必须可被验证);②③⑤ = 必须可被治理。
- 命名提醒:「服务治理」被用在两层(整个 bundle 的俗称 / 第⑤块);写文档时把第⑤块叫「容错」(代码即 `resilience`),免得歧义。
- ③与⑤分界看「输出」不看「输入」:两者都吃运行时反馈;③的输出 = 一个目标地址(去哪),⑤的输出 = 这次调用做不做 / 怎么做。接线 ≠ 概念分类(`Governance` 参数装 ③④⑤ 是容器交付的权威接线事实,不代表概念上同一块)。
- 安全不在五件套中,前提是「内部可信」;判据 = 它有没有 per-hop 决策——跨租户 / 跨 BU / 对外暴露 / 合规要求每跳身份时,要么回「配置」,要么升级成第六块。
- 配置不在五件套中(输送机制不是能力),但五块的阈值 / 策略 / 兜底全部由配置驱动(治理中心的策略源 + 热更);写文档必须带这一句。
- 一句话:五件套是能力,全部由配置(治理中心)驱动。

### 6.7 依赖方向与抽象裁剪

- **用户代码应避免使用 starter 里的类型**:若 gs 封装的三方包在 cloud 层有对应抽象,用户一律用 cloud 类型(触发器、选项、工作函数等);只有三方包无 cloud 对应物时才允许碰 starter 类型(2026-09-23)。
- starter 本职 = 提供 Server / bean 接线、按名桥接 bean、生命周期管理。
- 审查 starter 用户面 API 时逐一问「这个类型 cloud 有没有」;有则参数直接用 cloud 类型,starter 不改名重导出。
- API 不应让用户多记一个中间类型:参数直接收依赖包的本类型(如 `scheduling.WithLock(l lock.Locker, key string, ttl time.Duration)`),不为「保持本包零依赖」造适配器 / 最小接口让用户绕行。
- 删零消费方的公共抽象:单实现接口、逐字节同形的 pass-through 适配层、无消费者输入一律删;「无消费者」要数到 bean 唯一性这一层,不只是业务读取(多实例 starter 注册 `health.Indicator` 时的 `.Name(...)` 有容器去重这个真实消费者,勿删)(2026-09-23)。
- 审过保留的抽象:`cache.Codec`(有意的用户扩展点)、`messaging.Driver`(多后端契约)、各 starter Driver 接口(多实现约定)。

### 6.8 全局注册表与 bean 逃逸

- 真反模式 = 容器创建的实例逃逸进包级全局(bean ctor 自注册 + 全局 `Get` 服务定位);据此删除 `cloud/messaging` 的 `RegisterDriver` / `GetDriver`、`cloud/security` 的 `RegisterValidator` / `GetValidator`(改走 bean),`starter-casbin` 的 `RegisterAdapter` / `Watcher` 改 config `adapter` / `watcher` = bean 名(2026-09-08)。
- **保留不动**(合法静态插件目录,不是 bean 逃逸):`starter-gateway` 的 `RegisterFilter(name, FilterFactory)`、`starter-otel` 的 `RegisterSpanExporter` / `RegisterMeterExporter` / `RegisterPropagator`——都是包级注册 type-level 构造器,不是容器实例逃逸。别再提议删它们(2026-09-08)。
- 删公开 API 的代价:`cloud/messaging` / `cloud/security` 属已发布的 `go-spring.org/cloud` 模块,删公开 API 需配版本。
- 新判据(取代旧「包 init 注册 type-level 构造器 = 合法」):同一份契约另有容器内先例(`map[string]discovery.Discovery`、named Driver bean、同 wiring bean 内的 `governance.Source` 注入)时,全局注册表就是遗留而非法定形态(2026-09-13)。
- 配置源族(init 里 `conf.RegisterProvider`)不是待治理逃逸,是结构性 seam——`spring.config.import` 在容器建立前解析(容器由这份配置构建,鸡生蛋),跨阶段缝合点只能进程级。别再提议 per-container 化。包级可变的 `xxxController` 变量**已全部删除**(8 处改为 init 闭包内 `(&xxxCtrl{}).Load`,方法值闭包持状态)。
- `dubbo` mapconfig 的 `singleton` 保留:它是 dubbo-go 进程级全局世界的适配器(gs 写入与 dubbo extension factory 读取须同一实例)。
- 按名注入可选 bean 的手法:`TagArg(name)` 按名注入;键为空要传 nil 时用 `ValueArg((persist.Adapter)(nil))`(类型化 nil 接口),别用 `TagArg("?")`(by-type nullable 会误抓无关 bean)。
- `ModuleFunc` 参数固定 `(r,p)`、`BeanProvider` 只有 `Provide`,无命令式按名查 bean——一切按名注入只能走 ctor 的 `TagArg`。

### 6.9 `experimental/` 目录

- `experimental/` 目录语义 = 维护者尚未审核,不是「不成熟 / 次品」;能力补齐不以「移出 experimental」为目标。
- 提升 experimental 模块时补测试、跑 example 冒烟、完善文档即可,不改目录、不做「GA 搬家」重构;是否移出由用户审核决定。
- 搬迁是双向的:实验区既不是只出不进、也不是晋升队列;搬不搬由用户按家族逐个拍板,不要自行推断。
- 别用「核心在 `cloud/` 非实验 ⇒ starter 就该在 `starter/`」这条规则推断——它不成立。
- 实验区同时容纳「未审核」与「非生产工具」两类,此歧义用户暂不裁定,别自行写契约或再搬。
- 搬运配套动作:go.work 路径、`replace` 相对深度、各模块 USAGE / README 自引用路径与 import 全仓同步。

### 6.10 维护者工具与用户面隔离

- go-spring 自身维护的工具与面向用户项目的工具必须分开:不要在 `gs` builtins map(`gs/gs/main.go`,会进 `gs --help`)里注册维护者 / 仓内命令。
- 维护者工具放 `scripts/`(需要真 Go 逻辑时写 `scripts/<tool>/` 的 `package main` 用 `go run` 调,如 `scripts/bomtool`);注意 `gs/gs/cmd/` 下是 `package cmd`,那是 gs 的用户子命令库,不是维护者工具位。工具在 cobra Short/Long 标「maintainer-only; not part of the gs user toolkit」。
- BOM 版本治理(`versions.yaml` 基线,`scripts/versions.sh {check|diff|apply <module>}` 包 `scripts/bomtool`)是维护者专属,不进用户可见的 `gs` 二进制。
- 若维护者工具日后要发给外部采用者,走 `gs-<name>` 外部工具模型(`main.go` `tool.Call`,发现 PATH 上的 `gs-*`),不作为内建子命令。
- 加任何命令进 builtins 前先问「这是 go-spring 自身维护,还是用户项目?」。

### 6.11 已拍板的设计总则

跨模块的既有拍板,登记于此以免重议;机制细节在各自的模块文档里。

- **MQ binder 路径必须走与直连 API 相同的 guard / executor 缝**(`Manager.ClientExecutorFor`);治理按目标资源生效、与调用路径无关;per-instance 开关默认开、可关(2026-08-28)。
- **注册自愈:** etcd lease keepalive 死亡 / zk session 过期后自动重注册(退避 1s 倍增至 1min,仅 Close 退出);consul 因 `UpdateTTL` 自恢复无需改(2026-08-28)。骨架偏好:倾向家族共享骨架;若各 starter 独立实现,宁可各写本地小 helper,也不新建跨模块依赖。
- **casbin:** `policy` 与 `adapter` 互斥,fail fast——**除非**文档明确支持「文件装载 + 库持久化」模式,才做三态(2026-08-28)。
- **总则:** 死配置删除 + 语义冲突响亮校验 + 静默失败改 WARN / ERROR(2026-08-28)。
- **可观测管线归属原则:** 协议 starter 一律优先接 `starter-otel` 全局管线,不许自建;适用于未来任何新协议 starter(2026-08-28)。
- **管理面鉴权 = 共享 guard**(`cloud/security.Guard`,不引 JWT);admin-ui 激活语义 = addr 配置即激活。
- 「治理与调用路径无关」的延伸:guard 覆盖统一「按请求透明保护」姿态。

## 7. 相关文档

- [CLAUDE.md](CLAUDE.zh.md) —— 何时记录约定;输出与编码规则。
- [starter/DESIGN.md](starter/DESIGN_CN.md) —— 五种 starter 形态及全部横切约束(仓库中最深的规则集)。
- [contrib/DIRECTORY_CONVENTIONS.md](contrib/DIRECTORY_CONVENTIONS.md) —— contrib 示例的布局与命名。
- [spring/DESIGN.md](spring/README_CN.md)、[log/DESIGN.md](log/README_CN.md)、[layout/DESIGN.zh.md](layout/DESIGN.zh.md) —— 各模块内部设计。
- [layout/docs/agent-rules/common-rules.zh.md](layout/docs/agent-rules/common-rules.zh.md) —— 基于 Go-Spring 的项目共享的设计/编码/测试规则。
- [MANIFESTO.md](MANIFESTO.md) —— 长期的 "Process as Code" 方向。 -->
