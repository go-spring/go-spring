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
- **每个源文件都要有 Apache License 头**(见 [../LICENSE_HEADER](../LICENSE_HEADER))。
- **内部依赖靠 `go.work` 解析,不写 `require`。** 工作区内模块互相依赖通过 workspace
  文件解析;给内部模块加 `require` 会让 `go mod tidy` 去 proxy 拉包并 404。

## 2. 四种形态

每个 starter 都恰好属于以下四种形态之一。形态决定了它的生命周期、端口行为,以及
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
- **服务发现只做 client 侧。** client 类 starter 可通过 `stdlib/discovery`
  (由 driver 的 dialer 钩子注入 `LiveDialer`)把 `ServiceName` 解析成实时端点。RPC
  的 **provider** 注册刻意不在范围内 —— 它太依赖具体框架,硬统一只会套壳。
  `ServiceName` 为空时 client 按地址直连,行为不变。
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
6. 在底层库支持的前提下补 health、TLS、destroy。
7. 提供双语 README,以及只含 `check.sh` 的 `example/`(不放部署脚手架)。
8. 内部依赖走 `go.work`,不写 `require`。
