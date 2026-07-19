# Go-Spring Starter 设计约定

[English](DESIGN.md) | [中文](DESIGN_CN.md)

本文沉淀所有官方 Go-Spring starter 共同遵循的设计约束,用于保持整个 starter 家族
的一致性,并指导新增 starter。当前已有 starter 的按领域分类目录见
[README_CN.md](README_CN.md)。

starter 是**集成模块**:只负责把某一个第三方服务或框架接入 Go-Spring 的 IoC 容器
和服务生命周期,仅此而已。业务逻辑、部署脚手架、跨 starter 的公共抽象都不属于这里。

## 1. 模块布局

- **一个 starter 一个 Go module。** 仓库根目录没有 `go.mod`,每个 starter 独立管理
  自己的 module 与依赖图,避免用 Redis 的应用被迫拉进 Kafka 的传递依赖。
- **固定文件骨架。** 一个 starter 目录包含 `starter.go`(bean 注册 + 生命周期)、
  `config.go`(绑定用的 `Config` 结构体及 driver 注册表)、`README.md` /
  `README_CN.md`,以及仅用于冒烟和集成的 `example/` module —— 不放
  `build.sh` / `bootstrap.sh` 等部署脚手架,只留 `check.sh` / `gen.sh` 与源码。
  配置 Provider 类(§2.5)在此之上有变体:用 `provider.go` 取代 `config.go`
  (没有绑定 `Config` —— 连接参数从导入 source 串解析),冒烟 module 为
  `example-config/`。
- **每个源文件都要有 Apache License 头**(见 [../LICENSE_HEADER](../LICENSE_HEADER))。
- **内部依赖靠 `go.work` 解析,不写 `require`。** 工作区内模块互相依赖通过 workspace
  文件解析;给内部模块加 `require` 会让 `go mod tidy` 去 proxy 拉包并 404。

## 2. 五种形态

每个 starter 都恰好属于以下五种形态之一。形态决定了它的生命周期、端口行为,以及
应用如何消费它。

### 2.1 Server 类(自持监听端口)

Web(`gin`、`echo`、`hertz`……)与 RPC(`grpc`、`kitex`、`thrift`、`dubbo`……)
类 starter 自持一个网络监听器,通过导出 `gs.Server` bean 接入 Go-Spring 服务生命周期。

- **每个 server 监听各自独立的端口。** server 类 starter 从自己的 `Config` 读取一个
  独立地址(如 `${spring.grpc.server}` → `addr:=:9494`)。同一进程内的两个 server
  starter 不得共用端口,由应用分配互不冲突的地址。Contributor 类(§2.3)刻意**不**开
  端口 —— 它们挂载到应用已运行的 server 上。
- **提前监听,就绪信号后再 serve。** `Run(ctx, sig)` 先立即绑定监听器,让端口冲突在
  启动期就暴露,再阻塞在 `<-sig.TriggerAndWait()` 之后才 `Serve`。这样保证端口在
  Go-Spring 报告就绪前已绑定,但所有 bean 装配完成前不对外提供流量。
- **`Stop()` 优雅关闭。** HTTP server 调 `Shutdown`;RPC server 调 `GracefulStop`。
- **应用持有路由,starter 持有 server。** 应用提供一个注册函数 bean
  (`RouterRegister`、`ServiceRegister`、`HandlerRegister`……);starter 负责创建并
  配置引擎与传输层。注册函数就是接缝。
- **默认开启的开关。** 用 `gs.OnProperty("spring.<x>.server.enabled").
  HavingValue("true").MatchIfMissing()` 控制注册,通常再叠加一个"注册 bean 存在"
  的条件(`Condition(gs.OnBean[...])`)。

### 2.2 Client 类(driver 模式 + 多实例)

数据库、缓存、消息队列客户端(`go-redis`、`gorm-*`、`mongodb`、`kafka`、`nats`……)
向外连接一个外部服务。

- **只做多实例,走 `gs.Group` / `gs.Module`。** client 类 starter **不**注册默认单例
  bean。它把前缀下的配置绑成 `map[string]Config`,每个条目注册一个具名 bean。原因:
  默认单例 + 多实例双注册易误用,且条件单例语义隐晦。应用按名选实例
  (`autowire:"a"`),新增一个实例是纯配置改动。
- **地址必填 —— fail-fast。** client 绝不能静默回退到 `localhost`。字段默认空
  (`${addr:=}`),构造函数在没有地址(以及在支持发现的场景下没有 service-name)时
  于启动期用 `errutil.Explain` 报错。go-spring 的 `expr:` tag 逐字段校验,做不了
  "addr 或 service-name 二选一"的跨字段规则,所以这条约束写在构造函数里而非 tag 上。
- **driver 模式支持可插拔后端。** client 暴露一个 `Driver` 接口加一个包级注册表
  (`RegisterDriver`,重复/空/nil 时 panic)。`DefaultDriver` 内置随包发布;公司可
  注册自己的 driver 并用 `${driver:=...}` 选中,无需 fork starter。这也是注入服务
  发现的接缝(由 driver 构建 dialer)。可选能力放到**独立**接口上(如 go-redis 的
  `ClusterDriver`),让已有的自定义 driver 保持可编译。
- **启动期连接校验。** 客户端库允许时,构造函数做一次有超时上界的探测(如 Redis
  `PING` 用 `DialTimeout`),让配置错误在启动期暴露而非首个请求时。
- **每个实例都有 `Destroy`。** 每个 bean 注册析构函数,`Close()` 连接并停掉其背后的
  后台 goroutine 或服务发现 watch。缺失 destroy 曾是已知缺口,现在是硬性要求。

### 2.3 Contributor 类(不自持端口)

WebSocket(`websocket`、`websocket-coder`)、中间件(`lua-filter`)、鉴权
(`casbin`、`oauth2-client`)类 starter 贡献一个配置好的 bean,由应用挂载到它已经
运行的基础设施上。

- **不开监听器。** WebSocket starter 贡献一个 `*websocket.Upgrader` /
  `*websocket.AcceptOptions`;应用在已有的 HTTP server 上升级连接。这正是 WebSocket
  独立于 server 形态的原因。
- **bean 类型就是接缝。** 在同一能力的两种实现间切换只需改一行 blank import,详见
  §3 的共用前缀规则。

### 2.4 全局 / 基础设施类

`otel`(可观测核心)与 `pprof`(诊断)安装进程级设施。

- **`starter-otel`** 构建共享的 Tracer/Meter provider 并注册为 OTel 全局;client 类
  starter 针对这些全局埋点,otel 缺席时这些 hook 是 no-op(零配置、可选启用)。
- **`starter-pprof`** 在**独立**端口跑一个专用 HTTP server 暴露运行时 profile,刻意
  与应用主端口隔开。

### 2.5 配置 Provider 类(远程配置中心)

`starter-config-nacos`、`starter-config-etcd`、`starter-config-consul` 把远程配置
中心(Nacos / etcd / Consul KV)接入应用,使其能在启动时从中加载配置、运行时热更新。

- **按角色拆,不按后端拆。** Nacos、Consul、etcd 都是**双能力**后端 —— 既做配置也做
  服务发现。这两者在 Go-Spring 里是不同的接入点,因此落在不同 starter:**config**
  角色是配置 Provider 类 starter(本形态);**discovery** 角色走 client 侧
  (`stdlib/discovery`,§3)或框架原生(`contrib/registry/`,§3)。配置 Provider 类
  starter 只做 config 角色,别的都不做。命名对标 Spring Cloud Alibaba
  (`nacos-config` vs `nacos-discovery`)。
- **它注册的是 provider,不是 bean。** 接缝是 `init()` 里的
  `conf.RegisterProvider(name, fn)`,不是 `gs.Provide`。配置 Provider 类 starter
  不产生可注入的 bean;应用只需 blank import。这也是它带 `provider.go` 而没有
  `config.go` 的原因。
- **provider 在容器存在之前运行。** `spring.app.imports=`
  `[optional:]<name>:<host>:<port>/<key>?<query>` 会在 `AppConfig.Refresh` 阶段
  调用 provider,此时任何 bean 都还没装配。因此它拿不到 client bean —— 只能从 source
  串自建 client,并按连接维度缓存该 client,避免每次 refresh 泄漏 goroutine。连接参数
  (鉴权、namespace、format……)来自 source 的 query 串,而非绑定的 `Config`。
- **变更监听必须无条件先注册,在拉取之前。** provider 必须在"拉取的
  `optional`+不存在提前 return"**之前**装好 watch/监听器。否则应用在 key 尚不存在时
  启动就永远不注册 watch,后续 publish 也永不触发刷新。监听器按 `(client, key)` 去重。
- **热更新复用框架刷新,经 `Rooter` 桥接。** 一个 `configRefreshBridge` bean 注入
  `*gs.PropertiesRefresher`,把它的 `RefreshProperties` 存进 provider 的
  `refreshHook`(一个 `atomic.Pointer`)。远端变更时监听器调用该 hook,重新加载所有
  配置源(重跑 provider),并通过 `gs_dync` 的两阶段原子提交重新绑定所有 `gs.Dync[T]`
  字段。把需要热更新的 key 绑到 `gs.Dync[T]`。
- **内容解析复用核心 reader。** 用 `spring/conf/reader/{prop,yaml,toml,json}` 的
  `Read` 函数(按 `format` query 参数选择)解析远端字节,`flatten.Flatten` 后返回
  `map[string]string`。

## 3. 横切约束

- **配置前缀按能力划分,不按实现划分。** 实现**同一**能力的两个 starter 共用一个前缀
  —— `starter-websocket` 与 `starter-websocket-coder` 都用 `spring.websocket`;
  `starter-kafka`(franz-go)与 `starter-kafka-sarama` 都用 `spring.kafka`。用户只选
  一种实现;切换是 blank import 替换,配置零迁移。不要为形式上的隔离按实现拆前缀 ——
  隔离已由模块独立和 bean 类型不同天然保证。
- **fail-fast 优先于静默默认。** 必填输入(地址、凭证、模式相关字段)在启动期用清晰的
  `errutil.Explain` 校验,而不是默认成一个半可用的值。
- **生产能力是封装的一部分。** 健康/就绪检查、启动期连接校验、TLS、destroy 回调都被视为
  starter 必须提供的能力,而非可选附加项。TLS 是一个嵌套的 `TLSConfig`
  (`enabled` + cert/key/CA),默认关闭。
- **现阶段容忍重复优先于过早抽象。** 公共能力(health、TLS、fail-fast)刻意在每个模块
  各写一份,而不抽到公共包。后续可能有统一收敛的一轮重构;在那之前不要建跨 starter 的
  helper 包。
- **优先用框架自带的注册与发现;没有的才考虑统一。** 默认使用每个框架**自己**的
  注册与发现机制,而不是在其之上硬套一层 Go-Spring 抽象。只有对**本身没有原生机制**
  的传输层,才**考虑**由 Go-Spring 提供统一能力。原因:各 RPC 框架各自带一套互不
  兼容的注册抽象(kitex 的 `registry.Registry`、kratos 的 `registry.Registrar`、
  dubbo-go 的 config-only registries、go-zero 的 `discov.EtcdConf`……),再加一层
  Go-Spring `Registrar` 只会变成把我们的抽象翻译进各框架抽象的**第二层胶水**——正是
  这层耦合让"统一"变成净亏损。截至 2026-07-18 的评估:
  - *有原生注册+发现,用它们的(opt-in):* `kitex`(`kitex-contrib/registry-etcd`)、
    `kratos`(`kratos.Registrar`)、`go-zero`(`discov.EtcdConf`)、`goframe`(`gsvc`)、
    `dubbo`(config registries)、`trpc`(naming 插件)。各 starter 均已用"空=直连"
    的开关接好。
  - *无原生 provider 注册,有真实需求时才是候选:* 裸 gRPC(`starter-grpc`)、
    Apache Thrift(`starter-thrift`)、纯 HTTP web(gin/echo/hertz)。只有这些才值得
    做 Go-Spring 注册 seam,且要等具体需求落地。
- **client 侧发现已统一,provider 注册不统一。** client 类 starter 可通过
  `stdlib/discovery`(由 driver 的 dialer 钩子注入 `LiveDialer`)把 `ServiceName`
  解析成实时端点,这对各基础设施客户端是通用的。RPC 的 **provider** 注册按上述原则
  保持框架原生。`ServiceName` 为空时 client 按地址直连,行为不变。各框架原生注册进
  consul/etcd/nacos/zookeeper/polaris 的示例见 `contrib/registry/`。
- **实例级注册(ServiceRegistry)已提供;RPC 框架 provider 注册仍不统一。** 别把两种
  "注册"混为一谈。(1)把**本进程**注册进外部注册中心(Nacos/Consul/Eureka)—— 即
  Spring Cloud `@EnableDiscoveryClient` 的方向 —— 是与传输无关的通用能力,已通过
  `stdlib/discovery` 的 `Registrar` 抽象(`Register`/`Deregister` + 由后端自持的
  TTL/心跳,复用与 `Discovery` 相同的 driver 注册表 seam)及其首个后端
  `starter-registry-consul` 提供。(2)注册某 RPC 框架的**服务**仍按上一条保持框架
  原生。纯 Kubernetes 下两者都不需要 —— 平台已把每个 Pod 注册在 Service 之后(用
  `starter-discovery-k8s` 去发现);`Registrar` 是给虚机 / 裸机 / 混合部署用的。这个
  "注册自己"的 starter 属全局 / 基础设施形态(§2.4):导出一个 `gs.Server`,应用就绪后
  注册、`PreStop` 时注销,使滚动重启无损。
- **可观测遵循"中心定义、边缘桥接"。** starter 通过 OTel 全局输出,或用 `SetLogger`
  钩子把库的内部日志桥接进 go-spring `log`;桥接时必须同时补一个 go-spring
  `FileLogger` sink,否则会丢掉 console 输出。

## 4. 新增 starter —— 检查清单

1. 定形态(§2);它决定你的生命周期与端口行为。
2. 独立 module、标准文件骨架、license 头。
3. 按**能力**选配置前缀(如果你是第二种实现就复用已有前缀)。
4. Client? → `gs.Group` 多实例、driver 注册表、地址必填 + fail-fast、启动期探测、
   每实例 `Destroy`。
5. Server? → 自持端口、提前监听/就绪后 serve、优雅 `Stop`、应用提供注册 bean、
   默认开启开关。
6. 配置 Provider? → `provider.go` 里 `conf.RegisterProvider`(无 `config.go`、
   无 bean),从 source 串解析参数、缓存 client、在拉取前无条件注册监听、经 `Rooter`
   bean 把 `PropertiesRefresher` 桥接进 `refreshHook`,配 `example-config/`。
7. 在底层库支持的前提下补 health、TLS、destroy。
8. 提供双语 README,以及只含 `check.sh` 的 `example/`(不放部署脚手架)。
9. 内部依赖走 `go.work`,不写 `require`。
