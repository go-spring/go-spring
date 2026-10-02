# Go-Spring Starter 设计约定

[English](DESIGN.md) | [中文](DESIGN_CN.md)

本文沉淀所有官方 Go-Spring starter 共同遵循的设计约束,用于保持整个 starter 家族
的一致性,并指导新增 starter。当前已有 starter 的按领域分类目录见
[README_CN.md](README_CN.md)。

starter 是**集成模块**:只负责把某一个第三方服务或框架接入 Go-Spring 的 IoC 容器
和服务生命周期,仅此而已。业务逻辑、部署脚手架、跨 starter 的公共抽象都不属于这里。

## 1. 模块布局

- **一个 starter 一个 Go module。** 每个 starter 独立管理自己的 module 与依赖图,
  避免用 Redis 的应用被迫拉进 Kafka 的传递依赖。
- **固定文件骨架。** 一个 starter 目录包含 `starter.go`(bean 注册 + 生命周期)、
  `config.go`(绑定用的 `Config` 结构体及 driver 注册表)、`README.md` /
  `README_CN.md`,以及仅用于冒烟和集成的 `example/` module —— 不放
  `build.sh` / `bootstrap.sh` 等部署脚手架,只留 `check.sh` / `gen.sh` 与源码。
  配置 Provider 类(§2.5)在此之上有变体:用 `provider.go` 取代 `config.go`
  (没有绑定 `Config` —— 连接参数从导入 source 串解析),冒烟 module 为
  `example-config/`。
- **所有 `init()` 都放在 `starter.go` 里。** 注册是 starter 的入口,必须能在一处
  读完:包内其他文件一律不声明 `init()`。各能力文件(`config.go`、`client.go`、
  `observe.go`……)只放类型、构造函数与辅助函数——即实现,永远不放注册。无可注册
  内容的 module(纯库如 `starter-http-server`,或 `starter-gorm` —— 它的注册由各
  方言自己的 `starter.go` 完成)干脆没有 `init()`。
- **子包同样适用。** starter 靠副效应导入的辅助包——`starter-otel/metric` 下的
  exporter 工厂、`internal/logger` 下的框架日志桥——对外只暴露一个普通的
  `Register` / `Install`,由 `starter.go` 的 `init()` 调用,自己不再声明 `init()`。
  这样单独导入该子包什么也不做,这正是要点:starter 注册什么,一处读完。公共子包自己
  拥有的默认值同理:`starter-otel/trace` 把 W3C propagator 放在 `RegisterDefaults`
  里,由 starter 调用——只链接该包、不链接 starter 根包的调用方(luohua 伞包、该包
  自己的测试)显式调它,而不是从一个隐藏的 `init()` 里拿到注册。
- **每个源文件都要有 Apache License 头**(见 [../LICENSE_HEADER](../LICENSE_HEADER))。
- **仓库级 module 规则同样适用**(根无 `go.mod`;一子项目一 module;内部依赖靠
  `go.work` 解析、不写 `require`)。这些规则由 [../ARCHITECTURE_CN.md §1](../ARCHITECTURE_CN.md)
  单一持有;给工作区内模块加 `require` 会让 `go mod tidy` 去 proxy 拉包并 404。

## 2. 五种形态

每个 starter 都恰好属于以下五种形态之一。形态决定了它的生命周期、端口行为,以及
应用如何消费它。

### 2.1 Server 类(自持监听端口)

Web(`gin`、`echo`、`hertz`……)与 RPC(`grpc`、`kitex`、`thrift`、`dubbo`……)
类 starter 自持一个网络监听器,通过导出 `gs.Server` bean 接入 Go-Spring 服务生命周期。

- **端口必须由用户显式配置，不设默认值。** addr 字段 tag 写作 `value:"${addr}"` 不带 `:=` 默认值。端口配置本身即为 server bean 的启动条件：仅在 `OnProperty` 检测到地址已配置时才创建 bean（`Condition(gs.OnProperty("spring.<x>.server.addr"))`）。pprof server 除外，使用统一默认端口 `:6060`。
  同一进程内的两个 server starter 不得共用端口，由应用分配互不冲突的地址。Contributor 类（§2.3）刻意**不**开端口 —— 它们挂载到应用已运行的 server 上。
- **提前监听,就绪信号后再 serve。** `Run(ctx, sig)` 先立即绑定监听器,让端口冲突在
  启动期就暴露,再阻塞在 `<-sig.TriggerAndWait()` 之后才 `Serve`。这样保证端口在
  Go-Spring 报告就绪前已绑定,但所有 bean 装配完成前不对外提供流量。
- **`Stop()` 优雅关闭。** HTTP server 调 `Shutdown`;RPC server 调 `GracefulStop`。
- **应用持有路由,starter 持有 server。** 应用提供一个注册函数 bean
  (`RouterRegister`、`ServiceRegister`、`HandlerRegister`……);starter 负责创建并
  配置引擎与传输层。注册函数就是接缝。
- **不再使用 `gs.Module` 包装。** server bean 注册直接写在 `init()` 中用 `gs.Provide(...)` 调用。端口配置本身即为启动条件，不再需要 `enabled` 开关或 `OnBean[ServiceRegister]` 条件。
- **非监听型的长驻单元仍然是 `gs.Server`。** 消息消费者、回调 server、轮询器只要与进程同寿，就注册为
  `gs.Server`，让它的 `Stop` 在关闭时执行——绝不用 `gs.Runner`（其 `Run` 必须快速返回）。当被封装的
  库自身的 `Run` 会安装信号处理器时，改用它的 `Start` 并在 ctx 上等待，这样它就不会和 Go-Spring
  自己的关闭流程抢信号。
- **面向运维的 server 在启动期就提供服务。** 运维 / 诊断 server（看板、bean / 配置查看器）在就绪
  信号之前就应答，而不是等它，好让运维能看到启动过程。面向开发者的文档 UI 则挂到已有的
  app/actuator mux 上，不自己占一个端口。

### 2.2 Client 类(driver 模式 + 多实例)

数据库、缓存、消息队列客户端(`go-redis`、`gorm-*`、`mongodb`、`kafka`、`nats`……)
向外连接一个外部服务。

- **只做多实例,走 `gs.Group` / `gs.Module`。** client 类 starter **不**注册默认单例
  bean。它把前缀下的配置绑成 `map[string]Config`,每个条目注册一个具名 bean。原因:
  默认单例 + 多实例双注册易误用,且条件单例语义隐晦。应用按名选实例
  (`autowire:"a"`),新增一个实例是纯配置改动。
- **配置命名:每个家族两个桶。** 家族前缀下只有两个子键,别无其它:

  ```
  spring.<family>.default.*             家族级值,每一项都可被实例覆盖
  spring.<family>.instances.<name>.*    一个实例;<name> 即 bean 名
  ```

  - **bean 名带实现限定词 —— 当返回类型是可替换接缝时。** 判据是「**这个类型将来会
    不会有多种实现**」，与现在有几种实现无关。

    是接缝（类型在 `cloud/` 下，是中性、可整体替换的契约：`session.SessionStore`、
    `batch.JobRepository`、`discovery.Discovery`、`resilience.Driver`、
    `loadbalance.Factory`……；或框架级接口 `gs.Server`）→ **每个实现的 bean 名必须自带
    实现名**，家族有实例概念时写成 `<实现名>.<实例名>`。bean 以（名字，类型）为键，第二个
    实现带一个同名实例就会注册出同一个键，容器直接拒绝启动。gorm 的五个方言模块
    （`starter-gorm-mysql`、`-postgres`、`-sqlite`、`-sqlserver`、`-clickhouse`）共享
    `gormcore.DB`，所以 bean 名是 `<dialect>.<name>`（`gormcore.Module` 从配置前缀的
    最后一段取限定词，可用 `Dialect.BeanPrefix` 覆盖）；discovery 的 `etcd.<name>` /
    `nacos.<name>`、session-redis 与 batch-redis 的 `redis.<name>` 同理。

    不是接缝（该 starter 私有的类型，不会有第二个实现来抢，如 `*redis.Client`）→ 裸的
    实例名即可。另外「一个构造函数一个 bean」的实现可以不写 `Name`：gs 的默认 bean 名
    取自构造函数名（`gs_bean/bean.go`），本身就是天然的区分度（kratos / goframe / hertz
    的 server bean 就是这么活的）。

    限定词跟随「**这个实现是什么**」，不跟随「现在有几个实现」—— gorm 在只有 mysql 一个
    方言时也叫 `mysql.default`。拿个数当判据，家族长第二个成员时就必须回头给第一个改名，
    而那次改名由「别人加了个 starter」触发，最难预料。以上所有情况下配置 key 都是
    `instances.<name>` —— 只有 bean 名带限定词。

  分两个桶的理由:一个家族的配置恰好只有这两级,而把它们在结构上分开,才使得实例名
  永远不可能撞上家族级 key。平铺命名(`spring.<family>.<name>.*`,家族级 key 做兄弟)
  只能改用保留词表:一个叫 `driver` 的实例会把 `spring.<family>.driver` 从标量变成子树,
  启动报错还指向那个标量。分桶从构造上消除了整类失败——**没有保留词,任何实例名都合法**。
  (参考 Spring Cloud Stream,同一个问题同一种解法:`spring.cloud.stream.default.*` +
  `spring.cloud.stream.bindings.<name>.*`。)
  - `default` 只放**可被覆盖的默认值**。不可覆盖的策略不放这里;进程级策略有自己的
    命名空间(`spring.governance.*`)。
  - `instances` 桶是唯一的激活信号,所以注册必须 gate 在它上面
    (`gs.OnProperty("spring.X.instances")`),并用
    `conf.BindEach(p, "${spring.X.instances}", ...)` 绑定。只配了 `${spring.X.default}`
    的进程没有配置任何 client,不得激活该 starter。
  - **`default` 经由 binder 继承。** 绑定前包一层 ——
    `p = flatten.WithFallback(p, "spring.X.instances", "spring.X.default")` —— 实例没定义的
    key 会去掉实例名后去 `default` 里读同名 key。**单值逐 key 覆盖**:实例的叶子只替换该叶子,
    周围的取值仍由 default 供给(同一结构体的其它成员、同一 map 的其它条目)。**数组整体
    覆盖** —— 实例只要定义了一个元素,整个数组就是实例的。**wiring 期的选择用不了它**:
    `${...driver}` 选的是 *bean* 而不是值,所以仍保留显式的
    `${instances.<name>.driver:=${default.driver:=?}}` 链条。
  - **适用边界。** 两个桶针对的是"直接子键就是**用户自选**实例名"的家族。自己拥有
    子命名空间的家族保持自己的形状:`spring.discovery.*` 下是本进程的注册身份
    (`service-name`、`addr`、`weight`……)与各中心块 `spring.discovery.<backend>.<name>`,
    那里不存在用户可控的名字可以撞(`<backend>` 段归框架,用户自选的 `<name>` 在更深
    一层)。在 `instances` **里面**再多一层框架级的家族,`default` 仍挂在家族前缀一个 ——
    `spring.lock.instances.<backend>.<name>` 继承的是共用的 `spring.lock.default.*` —— wrap 的
    第一个参数取那个 backend 的 instances 路径。另外**不可被实例覆盖的强制项也不进
    `default`**:同家族的进程级策略(如 discovery 的身份)直接挂在家族前缀,进程级策略挂
    自己的命名空间(`spring.governance.*`)。
  - 两条路的不变量一致:**永远不要让用户自选的名字和框架 key 同层。** 单实例家族
    (`spring.http.server`)的 key 直接挂在家族前缀下,因为它根本没有实例名。
- **地址必填 —— fail-fast。** client 绝不能静默回退到 `localhost`。字段默认空
  (`${addr:=}`)。单字段必填校验通过 `expr` tag 在配置绑定阶段完成（字符串用
  `expr:"$ != ''"`,切片用 `expr:"len($) > 0"`）。跨字段规则（"addr 或 service-name
  至少一个"）在构造函数中用 `errutil.RequireAny` 校验,因为 `expr` 不支持跨字段约束。
- **driver 模式支持可插拔后端。** client 暴露一个 `Driver` 接口;`DefaultDriver`
  内置随包发布,容器即 driver 目录 —— 公司用
  `gs.Provide(...).Name("corp").Export(gs.As[Driver]())` 贡献自己的 driver,再用
  `${driver:=...}` 选中,无需 fork starter。没有包级注册表:点名一个不存在的 bean
  在启动期失败。这也是注入服务发现的接缝(由 driver 构建 dialer)。可选能力放到
  **独立**接口上(如 go-redis 的 `ClusterDriver`),让已有的自定义 driver 保持可编译。
- **要对外访问的 client starter 靠自己的 import 拿到治理能力。** 不需要任何治理 import:每个 authority
  由**管它的那个包**注册 —— `cloud/resilience` 注册 `*resilience.Manager`、`cloud/loadbalance`
  注册 `*loadbalance.Manager`、`cloud/fault` 注册 `*fault.Injector` —— 而注入其中一个的 client
  必然已经为了类型 import 了那个包。于是 `*resilience.Manager` /
  `*loadbalance.Manager` / `*fault.Injector` 必然在容器里;注入写成**必填**
  (`gs.IndexArg(N, gs.TagArg(""))`),不写可空(`"?"`)。这三个 bean 本就是 client
  契约的一部分(它天生可治理、可观测、可路由),而 starter-governance-file 在不绑定规则来源
  时完全惰性,所以集成它不改变任何默认行为。关掉治理是
  `spring.governance.enabled=false`(或不配来源),**不是"bean 不存在"**。写成可空会把
  "用户忘了 import"变成静默降级(治理看着在工作、其实没有),这正是本规则要消除的错误。
  (只在非测试代码里 blank-import;`gs.RunTest` 会经
  `spring.force-autowire-is-nullable` 把一切注入强制 nullable —— 那是 gs 的行为,
  不是本规则的例外。)
  构造函数在**构造期**把它们接上,那是 client **是什么**的一部分,而不是"造完要记得补的一步"
  —— 它组装 `cloud.ClientParams{Resilience, Fault, Loadbalance, Discovery}`,并在返回前
  `exec = params.ExecutorFor(system, label)`。零值 `ClientParams`(容器之外自己调构造函数的
  应用)退化为 `resilience.Unmanaged`:仍然被观测,外加一次"没有任何保护生效"的告警 —— 容器
  之外因此永远不是黑洞。
- **没有东西可保护的组件,自己发信号。** 上一条针对的是访问外部服务的 client;依赖就在本进程内的
  组件是例外,目前只有 `starter-bigcache` 一个成员。那里限流、熔断、重试都无从作用——而且一次
  瞬时的本地失败(条目超限)把熔断打开后,反而会拒掉本来能命中的调用,因为"共享下游挂了"这个前提
  不成立。所以它不注入任何治理 bean,`Driver.CreateClient` 不收 `cloud.ClientParams`,**也不走
  执行器链**:自己开操作的 span、自己记 `bigcache.operation.total` / `.duration`,不写访问日志——
  缓存调用频率高,而且它调的不是进程外的任何东西。它的 span 仍然骑着 span 属性载体,所以上层贡献
  的属性两边都不用配合就能落上去。这是下一条"唯一发射端"规则的唯一例外;
  `scripts/check-observability.sh` 为它单开一节,既不会在每一族的检查里失败,也不会悄悄飘回链上。
- **启动期连接校验。** 客户端库允许时,构造函数做一次有超时上界的探测(如 Redis
  `PING` 用 `DialTimeout`),让配置错误在启动期暴露而非首个请求时。
- **每个实例都有 `Destroy`。** 每个 bean 注册析构函数,`Close()` 连接并停掉其背后的
  后台 goroutine 或服务发现 watch。缺失 destroy 曾是已知缺口,现在是硬性要求。
- **一个关注点一个文件 —— 标准的 client starter 骨架。** client starter 把横切关注点
  拆到独立文件,而不是全堆进 `config.go` / `starter.go`,让每个能力的接线落在维护者
  预期的地方:
  - `config.go` —— `Config` 结构、`Driver` 接口。只负责建连接。
  - `starter.go` —— `init()` 期的 `gs.Group` / `gs.Module` 注册,以及组装 bean 的
    构造函数(`newClient`)。
  - `discovery.go` —— 客户端服务发现接缝:mesh 门控的构建器(用按配置 `discovery`
    标签注入的后端 bean 调 `discovery.NewResolver` + `WithScheme`,后端缺失、
    `ServiceName` 为空或 mesh 开启时返回 `nil`)。`Resolver` 是纯快照函数——无资源、
    无 `Stop`,新鲜度全在 discovery 后端 bean 内部——所以不按 client 缓存、
    `Destroy` 时也无需回收。driver 各自的 dialer(把
    resolver 包进 round-robin `Pool`,每次建连 `Pick`)留在 `config.go` /
    `starter.go`;这里只放 resolver 的构建。所有发现模式的池都按同一套建:挂
    suspension `Tracker`、在建连处把 `Pick` 与 `Complete` 配对、再
    `lbMgr.Bind(pool, entry label)`(在注入的 `*loadbalance.Manager` 上)——于是该 entry 的
    治理规则原地驱动 `balancer` / `outlier-threshold` / `outlier-suspend-for`,走的是
    `loadbalance` manager,不需要 import `cloud/governance`。
  - `client.go` —— 组装好的 client 及其 resilience 那一侧:构造函数钉在它上面的 executor、
    把每次调用路由进该 executor、`Close` Destroy 钩子,以及任何 per-client 保护缝
    (如 mongodb 的 dial 层 `resilience.NewDialer`)。
  - `observe.go` —— 交给框架唯一发射点的 operation 声明(本 starter 的 `Metric`
    前缀、有界属性、无界 detail、access `LogTag`);发射点本身在
    `cloud/resilience`(见 §3)。
  - `health.go` —— health indicator 构造函数,放在 starter 根包里(单函数子包不值那个
    import 代价)。组件没有可探测项时不建此文件(bigcache,见 §3)。
  这一划分与生态内既有 client starter(go-redis、gorm-*、mongodb、elasticsearch、
  neo4j……)现遵循的关注点边界一致,新增 starter 应照抄。§4 清单第 4 条引用此骨架。
- **`gs.Group` 没有注册表能力——够不到时用 `gs.Module`。** `gs.Group` 每个桶条目返回一个 bean，
  且无法 `.Name(...)` / `.Export(...)`，也无法注入其它 bean。若 starter 必须导出每实例的可替换
  接缝 bean，或其构造函数还必须注入别的 bean，就在 instances 桶上自己写一个 `gs.Module`。
- **构造期钉死的传输层要加一层可变的间接，且只保护对过载敏感的那条路径。** 当被封装的 SDK 在构造期
  就固定了传输层 / 连接器时，starter 装一个可替换的持有者，在构造**之后**把它的声明层 + resilience
  传输层指进去——声明层包住 resilience 层，绝不包它的基座。SDK 已在后台 goroutine 上重试的路径
  保持不保护，免得保护被重复计入。
- **把库的错误 channel 排空进日志。** 被封装的库若暴露一个错误 channel，starter 把它排空进框架日志，
  而不通过封装层再暴露出去：没人排空的 channel 最终会阻塞写入方。
- **把保护放在库给的那条缝上。** 基于 `database/sql` 的 client 在 driver 连接层保护，于是每个调用方
  ——包括坐在同一个池上的 ORM——都被覆盖，无需在每个调用点加 `Guarded*` helper。当库既没有拦截器、
  其 send 又不带 ctx 时，starter 改从调用点接缝插桩：包住 producer、让每条消费记录都过一遍 helper、
  trace 上下文随记录头传递。
- **消息驱动的约定。** 每个 publisher 一个 producer，每个 subscriber 一个 push consumer，handler
  出错即触发重投。
- **进程级 driver bean 服务每一个实例。** 因为 `Driver` bean 是进程级的，自定义 driver 委托给内置
  默认 driver 才能保住每实例行为。封装本身即进程级的库（一个进程一个 client）会让该 starter 变成
  单实例，覆盖多实例的默认。
- **声明式 client 靠代码生成，不靠运行时反射。**
- **resilience 包在负载均衡之外。** 包在里面会让重试复用负载均衡器已经选中的端点，所以 round-tripper
  坐在 `Pool` 之外，每次重试重新挑选。
- **client 的归属按后端划分。** store 不暴露共享 client 的后端，自己持有并在 destroy 时关闭自己的
  client；复用应用 client 的后端不关任何东西。
- **每个后端把共享的 TTL 旋钮适配成自己 store 接受的形式。**
- **无状态 client 既不注册 health indicator 也不注册 destroy 钩子。** 不持有连接的 client 没有可
  探测项，也没有可关闭的东西。

### 2.3 Contributor 类(不自持端口)

WebSocket(`websocket`、`websocket-coder`)、中间件(`lua-filter`)、鉴权
(`casbin`、`oauth2-client`)类 starter 贡献一个配置好的 bean,由应用挂载到它已经
运行的基础设施上。

- **不开监听器。** WebSocket starter 贡献一个 `*websocket.Upgrader` /
  `*websocket.AcceptOptions`;应用在已有的 HTTP server 上升级连接。这正是 WebSocket
  独立于 server 形态的原因。
- **bean 类型就是接缝。** 在同一能力的两种实现间切换只需改一行 blank import,详见
  §3 的共用前缀规则。
- **filter 或中间件提供的是一个包好的 mux。** 框架在 `OnMissingBean` 下安装默认的
  `*gs.HttpServeMux`，所以 filter 或中间件通过提供一个把它包住的 mux 来贡献自己。
- **JWT 校验器绝不接受用 HMAC 校验非对称密钥来源。**
- **session cookie 永远带 HttpOnly**，没有开关。
- **编译型 Go 没有可刷新的脚本 bean 文化。** 进程内的动态逻辑归中间件、CEL 或 WASM，所以 Lua
  filter starter 是网关边缘的工具，而非通用组件。

### 2.4 全局 / 基础设施类

`otel`(可观测核心)与 `pprof`(诊断)安装进程级设施。

- **`starter-otel`** 构建共享的 Tracer/Meter provider 并注册为 OTel 全局;client 类
  starter 针对这些全局埋点,otel 缺席时这些 hook 是 no-op(零配置、可选启用)。
- **`starter-pprof`** 在**独立**端口跑一个专用 HTTP server 暴露运行时 profile,刻意
  与应用主端口隔开。
- **`starter-governance-sentinel`** 只贡献一个进程级 bean —— 名为 `sentinel` 的
  `resilience.Driver` —— 给治理中心的 driver 目录。与 `starter-governance-file` 一起导入后,
  治理文档的 `spring.governance.driver=sentinel` 一次切换全部 executor——**含入站准入**，因为同一个
  `Driver` 同时应答两个方向。无端口、无自有 key。
- **全局 starter 在 `gs.Module` 装配期安装自己的全局设施，并通过 `gs.RegisterStopper` 拆除。**
  装配在任何 bean 构造函数之前运行；stopper 在所有 server 停止、容器关闭之后运行。
- **读优化的快照模式：单写者加一把 RWMutex。** 处理函数拷贝快照，绝不在处理过程中阻塞等待实时工作。
- **每次后台扫描的 ctx 上限为自己那一轮的间隔**，这样卡住的目标不会让连续两轮扫描重叠。

### 2.5 配置 Provider 类(远程配置中心)

`starter-config-nacos`、`starter-config-etcd`、`starter-config-consul` 把远程配置
中心(Nacos / etcd / Consul KV)接入应用,使其能在启动时从中加载配置、运行时热更新。

- **按角色拆,不按后端拆。** Nacos、Consul、etcd 都是**双能力**后端 —— 既做配置也做
  服务发现。这两者在 Go-Spring 里是不同的接入点,因此落在不同 starter:**config**
  角色是配置 Provider 类 starter(本形态);**discovery** 角色走 client 侧
  (`cloud/discovery`,§3)或框架原生(`contrib/discovery/`,§3)。配置 Provider 类
  starter 只做 config 角色,别的都不做。命名对标 Spring Cloud Alibaba
  (`nacos-config` vs `nacos-discovery`)。
- **它注册的是 provider,不是 bean。** 接缝是 `init()` 里的
  `conf.RegisterProvider(name, fn)`,不是 `gs.Provide`。配置 Provider 类 starter
  不产生可注入的 bean;应用只需 blank import。这也是它带 `provider.go` 而没有
  `config.go` 的原因。
- **provider 在容器存在之前运行。** `spring.config.import=`
  `[optional:]<name>:<host>:<port>/<key>?<query>` 会在 `AppConfig.Refresh` 阶段
  调用 provider,此时任何 bean 都还没装配。因此它拿不到 client bean —— 只能从 source
  串自建 client,并按连接维度缓存该 client,避免每次 refresh 泄漏 goroutine。连接参数
  (鉴权、namespace、format……)来自 source 的 query 串,而非绑定的 `Config`。
- **变更监听必须无条件先注册,在拉取之前。** provider 必须在"拉取的
  `optional`+不存在提前 return"**之前**装好 watch/监听器。否则应用在 key 尚不存在时
  启动就永远不注册 watch,后续 publish 也永不触发刷新。监听器按 `(client, key)` 去重。
- **热更新复用框架刷新,经进程级门面 `gs.RefreshProperties()`。** 控制器不再是
  bean:远端变更时监听器直接调用 `gs.RefreshProperties()`(由最近一次 `Start` 的
  app 挂载的包级门面,供容器外的 watch goroutine 使用,无需依赖注入)。该调用
  重新加载所有配置源(重跑 provider),并通过 `gs_dync` 的两阶段原子提交重新绑定
  所有 `gs.Dync[T]` 字段;app 启动前门面返回错误,提前触发是安全的 no-op。
  把需要热更新的 key 绑到 `gs.Dync[T]`。
- **内容解析复用核心 reader。** 用 `spring/conf/reader/{prop,yaml,toml,json}` 的
  `Read` 函数(按 `format` query 参数选择)解析远端字节,`flatten.Flatten` 后返回
  `map[string]string`。

### 2.6 聚合 / profile 类 starter(公司基线)

聚合 starter 把 go-spring 再基线到一家组织的约定上 —— **组合**既有 starter 并给默认,
**绝不重新实现**。`starter-luohua` 是参考。它区别于其它原型:导入并接线多个既有 starter,
而非一个第三方 —— single-concern 依然成立:它接的"第三方"是**公司基线**(身份、wire 词表、
错误词表、标准 driver)。

- **只走公开缝,绝无私有路径。** 它提供的每个默认 —— 一个 `security.TokenValidator` bean、
  一个 `i18n.MessageSource` 词表、一个注册的 driver、一个 log ctx 钩子 —— 都经该能力既有的
  公开缝 (ARCHITECTURE §5:内置实现走同样的缝)。若某聚合默认必须走私有路径才生效,错的是缝,不是默认。
- **默认一律让步。** 公司默认用 `gs.Provide` + `gs.OnMissingBean` / 配置门控提供,app 自带身份 /
  词表 / driver 时优先;聚合 starter 不裁决。
- **独有前缀、按配置装配。** 绑在自己的 `${spring.<name>}` 前缀下(如 `spring.luohua`);
  用 `gs.OnProperty` 前缀武装,能力开关在内。导入即 inert,配置才生效。
- **主体是一组 `gs.Module` + `Register*`。** 每个能力一个 `gs.Module`(绑配置 → 给默认 / 覆盖)
  或一个 `init()` 里的 driver-registry 注册。
- **一关切一文件。** 身份 / 传播 / 可观测 / driver / i18n 各居一文件,按关切命名(同 §2.2 client 骨架)。

## 3. 横切约束

- **配置前缀按实现划分,每个 starter 用自己的唯一 key。** 每个 starter 通过自己独有的
  `${spring.<name>}` 前缀绑定 —— client 类通过 `gs.Group`,server 类通过 `gs.Provide`
  加 `TagArg`。实现**同一**能力的两个 starter 使用不同前缀（`spring.kafka` 对应
  franz-go,`spring.kafka-sarama` 对应 sarama；`spring.redis` 对应 go-redis,
  `spring.redigo` 对应 redigo）。配置 key 本身就是技术选型的显式声明：用户通过写
  `spring.kafka.instances.xxx` 还是 `spring.kafka-sarama.instances.xxx` 来决定用哪个实现。这同时避免了
  两个实现被意外同时导入时的 bean 冲突,也让配置文件成为自解释文档。
- **框架读的键一律在 `spring.` 下 —— 只有一个登记豁免。** 由 starter、`cloud/` 包或
  gs 核心绑定的键都带 `spring.` 根（`spring.kafka.instances.*`、
  `spring.actuator.podinfo.labels-path`、`spring.profiles.active`）。**应用自己加的
  字段不带**：它用应用自己选的前缀，绝不用 `spring.` —— `spring.` 的含义是"这个键由
  框架定义",所以 starter 写进来才算名正言顺,应用写进来则是在冒用它没有的权限。
  唯一豁免是 `logging.*`：它由 `gs_app` 在容器 refresh **之前**读取,而一个要等装配
  完成后才生效的日志系统没法报告装配失败,所以它保留自己的顶层根。它是
  `scripts/check-config-namespace.sh` 的 `ALLOW_ROOTS` 里唯一一项;新增一项必须是有意
  为之,并同时登记在该脚本与本文件。从环境变量导出的键（`GS_POD_NAME` -> `pod.name`）
  根本不是配置键,不在本规则范围内。
- **fail-fast 优先于静默默认。** 必填输入(地址、凭证、模式相关字段)在配置绑定阶段通过
  `expr` tag 校验（单字段用 `expr:"$ != ''"`,切片用 `expr:"len($) > 0"`）,跨字段
  规则（"addr 或 service-name 至少一个"）在构造函数中用 `errutil.RequireAny` 校验,
  因为 `expr` 不跨字段。
- **生产能力是封装的一部分。** 健康/就绪检查、启动期连接校验、TLS、destroy 回调都被视为
  starter 必须提供的能力,而非可选附加项。其中两项各带一个每实例开关:启动期连通性
  探测(`ping`,默认关,未就绪的后端不该阻塞启动)与 health indicator(`health`,默认开)。
  health indicator 是给"有外部依赖可探测"的组件用的;纯进程内、无可探测项的组件
  (bigcache)不贡献 indicator——一个不可能失败的探针,在 AND 聚合的 readiness 里是个
  恒真项,不携带任何信号。TLS 是一个嵌套的 `TLSConfig`
  (`enabled` + cert/key/CA),默认关闭。
- **现阶段容忍重复优先于过早抽象。** 公共能力(health、TLS、fail-fast)刻意在每个模块
  各写一份,而不抽到公共包。后续可能有统一收敛的一轮重构;在那之前不要建跨 starter 的
  helper 包。判据是 helper 的**性质**,不是谁在用它:`cloud/governance.Parse`(规则文档
  解析器)被 file/http/etcd/nacos 四个源共享,放进了它产出模型所在的 `cloud/governance`
  —— 这是 `cloud/` 唯一一处 `spring` 依赖,可以接受,因为文档格式本就是框架自己的。
- **优先用框架自带的注册与发现;没有的才考虑统一。** 默认使用每个框架**自己**的
  注册与发现机制,而不是在其之上硬套一层 Go-Spring 抽象。只有对**本身没有原生机制**
  的传输层,才**考虑**由 Go-Spring 提供统一能力。原因:各 RPC 框架各自带一套互不
  兼容的注册抽象(kitex 的 `registry.Registry`、kratos 的 `registry.Registrar`、
  dubbo-go 的 config-only registries、go-zero 的 `discov.EtcdConf`……),再加一层
  Go-Spring `Registry` 只会变成把我们的抽象翻译进各框架抽象的**第二层胶水**——正是
  这层耦合让"统一"变成净亏损。截至 2026-07-18 的评估:
  - *有原生注册+发现,用它们的(opt-in):* `kitex`(`kitex-contrib/registry-etcd`)、
    `kratos`(`kratos.Registrar`)、`go-zero`(`discov.EtcdConf`)、`goframe`(`gsvc`)、
    `dubbo`(config registries)、`trpc`(naming 插件)。各 starter 均已用"空=直连"
    的开关接好。
  - *无原生 provider 注册,有真实需求时才是候选:* 裸 gRPC(`starter-grpc`)、
    Apache Thrift(`starter-thrift`)、纯 HTTP web(gin/echo/hertz)。只有这些才值得
    做 Go-Spring 注册 seam,且要等具体需求落地。
- **client 侧发现已统一,provider 注册不统一。** client 类 starter 可通过
  `cloud/discovery`(由 driver 的 dialer 钩子构建 `Loader`)把 `ServiceName`
  解析成实时端点,这对各基础设施客户端是通用的。RPC 的 **provider** 注册按上述原则
  保持框架原生。`ServiceName` 为空时 client 按地址直连,行为不变。各框架原生注册进
  consul/etcd/nacos/zookeeper/polaris 的示例见 `contrib/discovery/`。
- **服务网格模式集中退化客户端栈,而非逐 starter 判断。** 注入 sidecar
  (Istio/Envoy、Linkerd)后它已负责发现与负载均衡,应用自带的再叠加就会双重负载
  均衡,并让拓扑/离群逻辑错乱。用一个进程级全局开关(`mesh.Enabled`,默认按 sidecar
  环境变量自动探测;`GS_MESH_MODE` 环境变量 on/off/auto 可覆盖),在发现与负载均衡的
  Factory 装配处 —— 各 client starter 的
  mesh 门控 loader 构建器(见 §2.2)与 `loadbalance.Pool` —— 读取并统一退化为直通:
  服务名解析为唯一稳定的 Service 地址(ClusterIP)交给 sidecar 拦截,负载均衡器不再
  选择、不再剔除。因为 loader 构建器内部读取 `mesh.Enabled()`、mesh 开启时返回
  `nil`,client starter 无需在调用处逐分支即可感知开关 —— driver 直接跳过安装
  发现拨号器,按配置地址直连。代码不删除 —— 关掉开关即恢复完整的客户端行为。
- **实例级注册按 starter 各自提供;RPC 框架 provider 注册仍不统一。** 别把两种
  "注册"混为一谈。(1)把**本进程**注册进外部注册中心
  (Nacos/Consul/Eureka/ZooKeeper)-- 即 Spring Cloud `@EnableDiscoveryClient` 的方向
  -- 是与传输无关的通用能力。它的发布生命周期住在 `cloud/discovery`:`Server` 是
  唯一那个导出 `gs.Server`、统摄所有已配置中心的东西;每个
  `starter-discovery-<backend>`(etcd/nacos/consul/zookeeper)贡献一个 `Registry`
  -- 与它自己那个中心对话的协议适配器。换后端就是换 starter。
  所有 discovery starter 共守一条规则:**Register 必须自续约**
  (TTL、心跳或临时节点),correctness 绝不依赖 `Deregister` 被调用 -- 进程若崩溃
  (SIGKILL、OOM)未及注销,注册中心也须在保活静默后自行摘除;`Deregister` 只是干净
  停机时的快捷路径。(2)注册某 RPC 框架的**服务**仍按上一条保持框架原生。纯 Kubernetes
  下两者都不需要 -- 平台已把每个 Pod 注册在 Service 之后(用 `starter-discovery-k8s`
  去发现);实例级注册是给虚机 / 裸机 / 混合部署用的。注册核心属全局 /
  基础设施形态(§2.4):导出一个 `gs.Server`,应用就绪后注册、`PreStop` 时注销,使滚动
  重启无损。
- **可观测遵循"中心定义、边缘桥接"。** client 侧 starter 根本不上报:它把每个操作**声明**给
  框架唯一的发射点(下一条)。真正上报的组件走 OTel 全局;把库的内部日志桥接进 go-spring
  `log` 的组件走 `SetLogger` 钩子,且必须同时补一个 go-spring `FileLogger` sink,
  否则会丢掉 console 输出。
- **仪器集是共享的,观测不是。** 组件解析仪器**每进程一次**,不是每实例一次:OTel SDK 按
  name/description/unit/kind 给仪器建索引,之后每次创建都拿回第一份,所以重复创建是个空操作,
  且会静默保留第一份的描述。因此风险不在**重复创建**,而在**注册**。
  - *注册归属持有该值的那个人。* 实例为共享的 observable gauge 注册自己那一份观测,并在**销毁
    自己的同一个析构**里反注册;进程级的值由组件自己注册一次。注册若被别处持有,它就会活得比
    值更久 —— 上报一个已死的实例,并把它钉住不放。
  - *gauge 要注册,不要用创建期回调创建。* `metric.WithInt64Callback` 只对某个 descriptor 的
    **第一次**创建生效,之后每一次都被丢弃且不报错。它只在「每进程恰好创建一次」时才正确,而
    这个条件在调用点看不出来,违反它的表现是**少一条 series**,不是启动失败。`RegisterCallback`
    既可叠加又可撤销。
  - *同名即同一 descriptor。* name、description、unit 每次创建都必须一致,否则后一次连同它的
    描述一起被忽略。尚存的不一致是**登记,不是许可** —— 见 `scripts/check-observability.sh`
    的 `KNOWN_MULTI_DESC`:HTTP/RPC 服务端族(未迁移,见 §5),以及
    `messaging.client.connection.state_changes`(三个后端把 system 名写进了描述)。
  - *tracer 永不缓存*在结构体字段或包级变量里:被捕获的 `otel.Tracer` 在全局 provider 被重新
    设置之后就停止转发。到使用点现取。
  - *仪器集里的状态最多是成员关系* —— 哪些实例还活着 —— 绝不是测量数据。一个缓冲数据的共享
    observer 就是一个无人能界定边界的全局数据存储。
- **观测是两层 —— 那是数据粒度,不是第二个发射点。** 一次逻辑调用产出**调用级**记录
  （总耗时 = 尝试 + 退避、最终状态、尝试次数）与每次下游尝试的**尝试级**记录（下游自己那次
  花了多久）。两者量的不是一回事:下游耗时是**下游的属性**,退避是**我们策略的属性**,混起来
  会得出"调大退避 → 下游变慢"的荒谬结论。首次就成功的调用两层数值相等 —— 那是**两个语义**,
  不是重复。**被重试抹掉的错误绝不记为 error**（OTel 明文规则）:调用报的是最终状态。
- **组件可观测遵循同一条规则:同类型 → 同名、同型、同齐整,且名字取能力不取实现**
  （`db.client.operation.duration`,绝不是 `redis.command.duration`）。框架现在有**唯一
  一个发射点**,落在 resilience 链上;client 侧 starter 靠**声明**每个操作是什么来喂它
  （唯一的例外是上面那个进程内组件,它自己发射）。
  client starter 声明 `observability.WithOperation(ctx, observability.Operation{...})` ——
  操作的 `Name`、`Metric` 前缀（`db.client`、`messaging.client`）、有界的 `Attrs`、
  无界的 `Detail` 与 `LogTag` —— 再把调用路由进 resilience executor。resilience 的观测层
  （`cloud/resilience/observe.go` 的 `wrappedClientExecutor.Execute`）就按这份
  声明发射:调用 span、`<prefix>.operation.duration`（调用级,含重试与退避）、
  `<prefix>.attempt.duration`（尝试级,下游自己那次花了多久）、
  `<prefix>.active_requests`（在途调用）、永远开启的 `resilience.client.calls{status}`,
  以及每调用一行访问日志。
  - **发射点在重试循环之外**,所以尝试层只有指标、没有 span —— 在那个位置开不出 per-attempt
    的 span。这是刻意的取舍,不是缺口。
  - **关掉治理不等于关掉观测。** manager 对**透传 executor 也套**观测层,所以
    `spring.governance.enabled=false` 保留全部 span、指标与访问日志,只丢掉保护。发射点落在
    `resilience` 而不是 `cloud/observability`,是因为它的 fallback 路径要带 `resilience.*`
    名字与 resilience 的日志 tag —— 放中立包就得去借这套词汇。
  - **`Attrs` 必须有界**:这里的每个属性都会同时变成 metric label、span 属性和日志字段,
    无界的值（缓存键、主题名）会把序列撑爆。**`Detail` 可以无界** —— 键、语句、URL 路径、
    主题 —— 它进 span 和日志,但**永不**进 metric label。
  - **发射点在 `Execute` 入口就读 operation**,所以在 executor **内部**做的声明没有任何人
    会读到:钩子挂在 resilience round-tripper **下方**的传输层（见 `starter-s3`）要在它外面
    声明。
  - **日志分级归发射点**:失败 Warn、带 `Detail` 的成功 Debug（惰性）、不带 `Detail` 的成功 Info。
  - **非幂等操作绝不重试**:重复一次会变成**第二个副作用**（而非第二次尝试）的调用 —— 发信、
    向 broker 发布、处理一条消费记录 —— 声明 `NonIdempotent: true`,执行器**只跑一次**,
    无论策略配了多少重试（配置被这样丢弃时,每个 service 告警一次）。策略不可能知道这一点,
    只有客户端知道,所以这个声明归客户端做。
  三条原则支撑这条规则,少一条就会走偏:
  - *完整性是发射点的义务,一次付清。* 这些信号 —— span、时长指标、在途 gauge,以及每调用
    一行访问日志 —— 以前是逐组件的清单,现在只在唯一的发射点上查一次,并对每个做了声明的
    client 成立。于是 client 侧 starter 的完整性就是它的**声明**:没声明、或把发射点已经
    拥有的信号又自建一遍（自己的 `otel.Tracer`、自己的时长/在途仪器）,才是缺陷。缺一个能
    分辨成败的结果维度仍是缺陷;发射点的那个维度就是 `status`。
  - *共同性随完整性一起挪了。* 同类型同名依然成立,但名字现在由发射点按声明的前缀产出,
    所以同族成员（同一能力背后的可互换后端）靠**声明同一个 `Metric` 前缀和同一批有界属性键**
    来取得一致。这正是「换一个后端不必重写看板、告警与查询」的由来。
  - *灵活性* —— 组件特有的键是允许的。没有任何东西仅因为「和别人不一样」而违规。
  - *分两档,因为同名只在同义时才成立。* `status`（`ok`/`error`）、`rpc.method`、
    `duration_ms` 在每个 RPC 后端里含义完全相同,所以必须同名 —— 它们是跨框架查询
    唯一能 join 的东西。而 transport 专属的状态码（`rpc.grpc.status_code`、
    `rpc.thrift.status_code`）各家取值词表不同,强行同名等于把两个含义塞进一个键,
    比不同名更糟。HTTP 同此一刀:共用的轴是 `status`,细节是
    `http.response.status_code`。
    客户端发射点同此一刀:`status` 与别处一样是 `ok`/`error`,而
    **`resilience.outcome`** 承载保护各阶段对这次调用做了什么
    （`rate_limited`、`circuit_open`、`bulkhead_full`、`retry_budget_exceeded`、
    `timeout`）—— 什么都没做时它**不出现**。把这些词放进 `status`,就是让一个键有两套
    词表,正是这一档要防的失败。
  - *日志必须能 join 指标。* 日志行里的身份键必须在某个属性（span 属性或 metric label）
    上同名存在,否则失败的指标落不到解释它的那行日志上。时长键（以 `_ms` 结尾）与
    `error` 是日志行自身的载荷,不是身份。这条规则针对的是**解释组件自己信号**的那行日志;
    组件的普通应用日志（写在 `log.TagAppDef` 下）不算。
  各族的键清单是机械的,落在 `scripts/check-observability.sh` 里,它就是执行面 ——
  族规列出每个成员必须有的东西,清单之外一律不管。它的各节,以及**登记而非关闭**的缺口:
  - *族* —— DB 与消息。成员**声明**（本族的 `Metric` 前缀与本族的有界属性键,经
    `observability.WithOperation`）并把调用路由进 executor;族规查这份声明,并查它**没有**
    退回自建 span（`otel.Tracer`）或自建时长/在途仪器。HTTP server 与 RPC 两族本轮**未迁移**
    —— 它们的成员仍自建 span 与仪器,规则照旧。指标或 span 由第三方库发出的成员需附接入点
    证据正则登记（证据在**去注释源码**上匹配,所以删掉接线、留着注释照样失败）;
    go-redis（`redisotel.InstrumentTracing`）与 elasticsearch
    （`ElasticsearchOpenTelemetry`）原先开的第三方**每调用** span 已撤 —— 它们和发射点现在
    开的调用级 span 重复。第三方**非**每调用的遥测（go-redis 的连接池指标、kotel 的客户端/
    连接指标）仍照旧保留。
  - *发射点* —— 唯一的发射点对上述齐整一次性负责,故只在那里查一次:调用 span、
    `<prefix>.operation.duration`、`<prefix>.attempt.duration`、`<prefix>.active_requests`、
    永远开启的 `resilience.client.calls{status}`、分级正确的每调用一行访问日志,以及
    `Detail` 不得进 metric label。
  - *底线段* —— 自建插桩、但没有可互换同类的组件（`config-bus`、`gateway`）:只查完整性与
    可 join,因为共同性没有对象可绑。
  - *委托段* —— 信号完全来自共享层（`http-client`、`oauth2-client`、四个 `lock`
    后端、三个 `transaction` 后端、`scheduler` —— 它的信号在 `cloud/scheduling`、各配置源、
    discovery 后端）:查的是接线还在不在。
  - *正向清单* —— 必须被插桩的组件。这套模型靠「已经带了什么信号」反向识别成员,所以一个
    从未插桩的组件对它完全隐形;这张清单让「该做而没做」变得可见。
  - *已登记缺口* —— `starter-mongodb` 在 **command 层**发射,而不是声明 operation,因为
    mongo driver v2 没有 per-command 的 execute 钩子（只有 `CommandMonitor` 这种「只看不能拦」
    的观察者）,所以它的 resilience/executor 接缝是 **dialer**;每 command 信号只能挂在驱动的
    command monitor 上,保护则留在 dial 层。它保持发射点的词汇 —— 有界的 `db.system` /
    `db.operation` label、只进 span 与日志的语句 detail、同样的指标名、自己的 access tag ——
    保护也保持在 dial 层。这是驱动的约束,是有意接受的。
    `starter-oauth2-client` 的业务调用经 resilience 层打每调用日志（与
    `starter-http-client` 同源:它走注入的 `resilience.Manager` 而非自己
    组装包装层）;但 oauth2
    库内部的 token 端点换取不经过那个 RoundTripper,故它只有 span、没有日志。
    kitex 与 kratos 的时长指标没有 `status` 维度,因为指标由库发出;
    `cloud/experimental/transaction` 只有 span
    （它的 `Observer` 是整条替换型缝,§1.5 已裁决保持现状,故框架不为它规定另外两种信号）;
    属性刷新漏斗（`observability.RefreshConf`）是唯一既出指标又记录刷新结果日志的地方 ——
    调用方只记自己的后端事件,「集群是否刷新成功」这条日志归漏斗
    （日志该由知道主题的那个模块来打）。
  - *连接状态* —— `messaging.client.connection.state_changes` 只在**客户端库提供连接状态回调**
    的成员上出现（mqtt、nats、rabbitmq）。它**不进**消息族的共同清单:对没有这种回调的库硬
    要求,只能逼出假数据。checker 持有这份登记,并在「已登记成员不再上报」或「未登记成员开始
    上报」时报错。
- **Discovery 后端在自己的缝上上报,经由自己的 observer。** 每个配置块在构造期建一个
  `discovery.Observer`(由后端名 + 块名构成),上报走它的方法（`RegisterAttempt`、
  `DeregisterAttempt`、`WeightChange`、`Synced`）,后台重注册传
  `discovery.ReasonSelfHeal`;块的析构函数关闭 observer,连带收回它的 gauge。observer 是
  块自己持有的一层封装,所以那些 gauge 背后的状态与块同寿:同进程内两个块不会互相污染,
  测试之间也没有全局的东西要重置。要报在「首次发布」与「自愈路径」**共同经过**的漏斗上
  （etcd `publish`、zookeeper `createNode`、consul `upsert`）,而不是包在
  `discovery.Registry` 接口外面:自愈路径根本不经过那个接口,而它恰恰是「实例还在服务、
  却已不再可被发现」的那条路径。指标与 span 的定义只在 `cloud/discovery/observe.go` 里
  有一份 —— 后端自己从不命名 instrument。本家族的**只做发现**后端
  （`starter-discovery-k8s`,集群内平台已替你把 Pod 注册好）没有这样的漏斗可报:它建同一个
  observer、在成功与失败两侧上报 `Synced`,三个注册操作一个都不产出;该例外登记在
  `scripts/check-observability.sh`(discovery 段)。
  这些操作周围的日志行归后端自己写,所以字段由各后端自己拼出来。**一行日志要能解释
  某条 discovery 指标或 span,就必须按下面这些名字、一个不少地带齐,否则 join 不上:**

  | 字段 | 取值 |
  |---|---|
  | `system` | 后端名（`etcd`、`nacos`……）—— 取自 `Observer.System()`,与该指标自身的 `system` 属性同值 |
  | `center` | 配置块名（`${spring.discovery.<backend>.<name>}`）—— 取自 `Observer.Center()`。一个进程会把同一个服务注册进每一个配好的中心,没有这一维,两个集群的读数就是一条分不开的序列 |
  | `service` | 正在注册或同步的服务名 |
  | `operation` | `register` / `deregister` / `update_weight` / `sync` |
  | `reason` | `initial` / `self_heal`,仅注册时有 |
  | `status` | `ok` / `failed`,取自 `discovery.StatusOf(err)` —— 词表不是别族用的 `error`,所以这个值必须来自函数而不是手写 |
  | `error` | 错误本身,用 `log.Err` |

  消息与自有的细节(键名、采取的动作)由调用方自己补;上表只覆盖"标识这次操作"的部分,
  也就是 join 所需要的那部分。
- **活实现不进包级注册表。** `cloud/lock` 刻意不设字符串 driver 注册表（与 discovery、
  resilience 不同）：锁需要一个活的 backend 句柄，所以后端由 blank import 哪个 starter 来决定。
  更一般地，活的、由配置派生的实现绝不注册进包级 map——全局 map 在测试之间和重启之后都是错的。
- **各 starter 独立版本化，绝不互相依赖。** 同一能力的第二个 starter 重写那份已验证的模式，而不是
  import 它的兄弟；分布式事务的各种模式各自作为独立 starter 发布，绝不合并成一个抽象。
- **超过两种 wire 行为就是 TLS 模式枚举。** 当一个协议多于两种 wire 行为（SMTP：STARTTLS /
  隐式 TLS / 明文）时，TLS 配置是一个模式枚举，而不是生态惯用的 `enabled` 加证书对。
- **唯一可用的探针带副作用时就不发启动探针。** 当只有一个真实 POST 可作探针时，干脆不发探针——
  启动期打一次垃圾调用比首次发送报错更糟。
- **聚合结构体用字段级 `value:"..."` tag 绑定**，绝不用 `gs.TagArg("${prefix}")`。
- **第二个 `gs.Server` bean 必须 `.Name(...)`**——容器里已经有一个默认 web-server bean。
- **`gs.Dync[map]` 的默认值必须为空**，绝不能是非空 map 字面量。
- **能用标准库自己拿下的小协议，就不用依赖一个休眠的 SDK**；当官方对端服务需要繁重编排时，提供一个
  自包含的 mock-server example。
- **零依赖的能力包只暴露朴素的 Observer / 回调接缝。** `cloud/` 能力包不得依赖具体后端；OTel span
  辅助函数留在 starter 层，而不是中立底座。
- **能力的默认内存 store 走 `gs.OnMissingBean`**，于是持久化 store 的 starter 可为所有消费方替换它，
  业务代码无需改动。

## 4. 新增 starter —— 检查清单

1. 定形态(§2);它决定你的生命周期与端口行为。
2. 独立 module、标准文件骨架、license 头。
3. 选配置前缀 —— 使用唯一的 `${spring.<name>}` 前缀来标识**本**实现。如果是已有能力的
   第二种实现,用 `<能力>-<实现>` 格式（如 `spring.kafka-sarama`）,不要复用已有前缀。
   `spring.` 根不是可选的（§3）,`scripts/check-config-namespace.sh` 在每个绑定点强制。
4. Client? → `gs.Group` 多实例、driver 注册表、地址必填 + fail-fast、启动期探测
   (`ping`,默认关)、每实例 `Destroy`,以及"一个关注点一个文件"的骨架(§2.2):`config.go` /
   `starter.go` / `discovery.go` / `client.go` / `observe.go` / `health.go`。
   配置走两个桶:`conf.BindEach(p, "${spring.<family>.instances}", ...)`,模块 gate 用
   `gs.OnProperty("spring.<family>.instances")`,家族级值在各实例的 tag 里读
   `${spring.<family>.default.*}`。**绝不可直接绑定在家族前缀上** ——
   `scripts/check-config-namespace.sh` 会检查。
   **返回类型是可替换接缝?** → bean 名带实现限定词(`<实现名>.<实例名>`,见 §2.2),
   否则第二个实现的同名实例会撞出同一个(名字,类型)键。
   **走发现拨号?** → 建池一律这三件套:挂 `loadbalance.Tracker`、在建连处把 `Pool.Pick`
   与 `Pool.Complete` 配对(把拨号结果喂回去,不喂 tracker 就是瞎的)、再用**与 executor
   同一个** `resilience.ServiceLabel`,在注入的 `*loadbalance.Manager` 上调
   `lbMgr.Bind(pool, <entry label>)`。这才是 `balancer` / `outlier-threshold` /
   `outlier-suspend-for` 可经治理规则配置而不是写死的原因——自己不加 `balancer` 配置键,
   也不 import `cloud/governance`。不能重挑的 client
   (一次性解析、或库自持选择器)刻意不接,要在它的 USAGE 里写明。
5. Server? → 自持端口、提前监听/就绪后 serve、优雅 `Stop`、应用提供注册 bean、
   默认开启开关。
6. 配置 Provider? → `provider.go` 里 `conf.RegisterProvider`(无 `config.go`、
   无 bean),从 source 串解析参数、缓存 client、在拉取前无条件注册监听、变更回调
   直接调 `gs.RefreshProperties()` 门面,配 `example-config/`。
7. Discovery starter？→ 定义 `obsSystem`,并在「首次发布」与「自愈路径」共享的缝上上报
   （`discovery.RegisterAttempt` 带 `ReasonInitial`/`ReasonSelfHeal`、`DeregisterAttempt`、
   `WeightChange`,以及成功与失败两侧的 `discovery.Synced`);绝不要改成包 `Registry`
   接口 —— 自愈路径不经过它。本家族的只做发现后端（`starter-discovery-k8s`）没有
   registrar:只定义 `obsSystem` 并上报 `discovery.Synced`,登记为本检查的例外。
   `scripts/check-observability.sh`(discovery 段) 强制以上各项。
8. 组件有外部依赖可探测时补 health(`health` 开关,默认开);再在底层库支持的前提下补
   TLS、destroy。
9. 提供双语 README,以及只含 `check.sh` 的 `example/`(不放部署脚手架)。
10. 内部依赖走 `go.work`,不写 `require`。
