# starter-gorm

gorm 族的核心：所有 go-spring gorm 方言 starter 共用的骨架。Go 包名为
`gormcore`。

**应用永远不直接 import 本模块。** 这里没有可独立配置的东西——import 一个方言
starter（`starter-gorm-mysql`、`-postgres`、`-sqlite`、`-sqlserver`、
`-clickhouse`），本模块随之传递引入。

## 它做什么

现有五个方言 starter，每个负责自己的方言——`Config` 块、DSN、TLS / 服务发现
拨号——以及注册 bean 的那个 `gs.Module` 块，它就写在 starter 里，读的人一眼能看到
它贡献了什么：每个已配置的 `spring.gorm.<dialect>.instances.<name>` 条目对应一个
`*DB` bean 加一个配对的健康指示器。其余与方言无关的部分全部在这里，五个方言共用
一份实现而非各自复制：

- 各 starter 的 bean 所跑的每实例构造 `NewDB`：open / ping / customize 序列、
  连接池调优，以及 `*DB` 包装 bean（内嵌 `*gorm.DB`，gorm 全部方法原样提升）；
- gorm observe 插件——把每次 Create / Query / Update / Delete 声明为一个客户端
  操作（名称、`db.system` / `db.operation` 标签、SQL 语句与访问 tag）挂在调用的
  ctx 上。信号本身——call span、call 级与 attempt 级时长直方图、在途计数器、那一条
  访问日志——由 resilience 层（executor 链条上的唯一发射点）发射。插件不发射任何
  东西，因此走 OTel 全局对象，未装 `starter-otel` 时开销近乎为零；
- resilience 回调——每个操作都在同一个后端中立的 `resilience.ClientExecutor` 下执行，
  `gorm.ErrRecordNotFound` 视为成功；
- open 之后的 `DBCustomizer` 扩展缝，以及各 starter 采用的带方言限定的 bean 命名
  （`<dialect>.<name>`），使两个方言可以携带同名实例而不冲突。

## 完整参考

[USAGE_CN.md](USAGE_CN.md) 是五个方言 starter 共用的唯一完整参考——装配、配置
绑定、observe、resilience、健康检查、卸载，附一个完整可跑的工程；
[USAGE.md](USAGE.md) 为英文版。各方言自己的 README 只记录它新增的
DSN / TLS / 服务发现 key。
