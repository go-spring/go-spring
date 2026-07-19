# spring 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`go-spring.org/spring` 是 Go-Spring 四层栈（stdlib → spring → starter →
gs）中的容器/核心层，提供 IoC 容器、依赖注入接线、分层配置绑定与应用生命
周期模型。它只依赖 `stdlib` 与 Go 标准库，绝不 import 任何三方业务 SDK
（Redis、GORM、Kafka 等）——那类依赖归上一层的 `starter/`。

## 1. 职责与边界

- 容器职责：在 `init()` 阶段收集 bean 定义，解析条件，接线依赖，跑
  `Init` / `Destroy` 生命周期，由 `gs.Run()` 驱动 `Runner` / `Server` 两
  个角色。容器不承担协议逻辑，也不持有三方客户端——它把接缝
  （`Provide`、`Module`、`Group`、`Condition`）暴露给 starter 去贡献。
- `spring/conf` 是配置引擎：分层来源（命令行、环境变量、
  `app-<profile>.<ext>`、`app.<ext>`、内存、tag 默认值）按优先级合并，
  格式读取器在 `spring/conf/reader/{yaml,toml,prop}`，可插拔解密在
  `spring/conf/decrypt`。引擎独立于容器，容器在启动过程中驱动它。
- `spring/gs` 是对外表面：`Provide`、`Configure`、`Module`、`Group`、
  `OnProperty`、`OnBean`、`Dync[T]`、`Runner`、`Server`、`ReadySignal`、
  `PropertiesRefresher`。`spring/gs/internal/...` 全是实现细节，不对
  外承诺。

## 2. 关键抽象与接缝

- **Bean 注册**。`gs.Provide(objOrCtor, args...)` 在 `init()` 期记下
  bean 定义。构造函数的参数按类型索引进行匹配。链式 builder 配置
  `Name`、`Init` / `Destroy`、`Condition`、`DependsOn`、`Export`、
  `Configuration`。
- **按导出接口建索引**。容器为每个 bean 建两份索引：bean 自身的具体类
  型，以及 `.Export(gs.As[Iface]())` 声明的每个接口。**未在 Export 里
  列出的接口不会被索引**——就算 bean 结构上实现了该接口，
  `[]Iface autowire:""` 和 `gs.OnBean[Iface]()` 也找不到它。参见
  `spring/gs/internal/gs_core/injecting/injecting.go`（`beansByType`、
  `GetExports`）。bean 必须是引用类型（指针/接口），值结构体不能做
  bean；同类型多 bean 必须 `.Name()`，否则默认名冲突报重复。
- **依赖注入**。字段上的 `autowire:""` / `autowire:"name?"` /
  `autowire:"a,*?,b"` 与 `value:"${key:=default}"` 在一次反射遍历里全部
  填好。启动后不再反射——匹配函数与字段偏移都被缓存。
- **条件化自动装配**。`gs.Module(cond, fn)` 把 starter 的一组 bean 收在
  `PropertyCondition` 后面；`gs.OnProperty` / `gs.OnBean` /
  `gs.OnMissingBean` / `gs.OnSingleBean` 通过 `And` / `Or` / `Not` /
  `None` 组合。这是每个 starter 从配置键选择性启用自己的统一接缝。
- **动态配置**。`gs.Dync[T]` 包住字段，让
  `PropertiesRefresher.RefreshProperties()` 就地重绑而无需重启容器。这
  是配置中心类 starter（`starter-config-{nacos,etcd,consul,vault,file}`）
  共用的接缝。
- **运行时模型**。只有两个角色：一次性的 `Runner` 与长时运行的
  `Server`（带 `ReadySignal`）。所有 server 先完成监听绑定，再阻塞在
  `sig.TriggerAndWait()`，保证任何 server 都不会在其它 server 尚未绑
  定端口时就接流量。框架负责并发启动、信号处理与优雅 `Stop()`。

## 3. 约束

- `spring/` 内禁止出现三方业务依赖。Go 标准库与 `stdlib/` 之外的一切
  归 `starter/`。
- 所有注册都在 `init()` 期完成。`Configure(func(app gs.App))` 是这个阶
  段的延伸；`Run` 开始后不允许再注册 bean。
- `internal/` 子树不属于对外 API，即便通过再导出可达；下游必须走 `gs.`
  包。
- bean 暴露的接口就等于 `.Export(gs.As[Iface]())` 显式声明的那些——不
  做自动接口发现。漏 `Export` 是最常见的接线 bug，且在收集阶段静默失
  败。

## 4. 权衡与被否决的方案

- **拒绝运行时扫描（Spring Boot 那样的 classpath 扫描）**。所有 bean 元
  数据都在 `init()` 里注册，没有 classpath 遍历。代价是每个 bean 提供
  方必须被真正链接进来（`internal/init.go` 中的 blank import）；收益是
  可预测的启动、无顺序玄学、接线后零反射。
- **拒绝编译期 DI（Wire 那种代码生成）**。容器保留运行时依赖图，让条件
  模块、从配置 map 里批量建 bean 的 `Group`、`Dync[T]` 热刷新可以在启
  动/运行期决定实例化什么。反射被约束在启动那一遍。
- **拒绝隐式接口索引**。若把每个"结构上实现"的接口都索引进来，
  `OnBean[Iface]` 与 `[]Iface autowire:""` 就变得非局部、难以推理。强
  制 `Export` 让类型索引成为维护者掌控下的闭集合。
