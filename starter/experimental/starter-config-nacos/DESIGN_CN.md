# starter-config-nacos 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-nacos` 属于 config-provider 形态（`starter/DESIGN.md` §2.5）
的集成层 starter：把 Nacos Config 变成 Go-Spring 启动期和每次属性刷新时的
远程配置源。它夹在核心容器的 provider 机制（`spring/conf`）与 Nacos SDK
之间，自身不持有任何配置状态。

## 1. 职责与边界

- 只在 `init()` 里通过 `conf.RegisterProvider` 注册一个 `nacos` provider 名称，
  再无别的顶层动作——无可注入 bean、无 server。
- 解析 provider source
  `nacos:<host>:<port>/<dataId>?group=&namespace=&format=&username=&password=&timeout-ms=`，
  拉取 dataId，按 `properties`/`yaml`/`toml`/`json` 之一解析，并 flatten 为
  `map[string]string` 交回框架合并。
- 对 `(group, dataId)` 安装 `ListenConfig`，远端发布触发一次 provider 重跑，
  让所有绑定的 `gs.Dync` 字段热更新。
- **不做**服务发现。Nacos naming 是独立能力，按 `starter/DESIGN.md` §2.5
  的“按角色拆分”决策放到别处。

## 2. 关键抽象与缝隙

- **Provider 缝隙。** 扩展点只有 `conf.RegisterProvider("nacos", loadNacosConfig)`；
  应用通过 `spring.app.imports=[optional:]nacos:...` 消费。provider 运行在
  `AppConfig.Refresh` 阶段，早于任何 bean 存在，因此从 source 串**自建 SDK
  client**，不能通过依赖注入拿。
- **Client 缓存。** Nacos SDK client 按连接串元组缓存。否则每次刷新都会新建
  client 及其后台 gRPC 连接。
- **Refresh 钩子。** provider 侧状态是一个 `atomic.Pointer[func() error]`，
  由容器域桥接 bean `configRefreshBridge` 填充；后者注入
  `*gs.PropertiesRefresher`，导出为 `gs.Rooter` 且命名 `nacosConfigRefreshBridge`，
  确保总被实例化、且不与应用自身的默认 `__default__` Rooter 冲突。
- **Listener 缝隙。** `ListenConfig` 按 `(client, group, dataId)` 三元组去重，
  避免重复 `Load` 注册并行 listener。

## 3. 约束

- **`ListenConfig` 必须在 `GetConfig` 之前注册。** 这是最关键的不变式。若
  listener 仅在 `GetConfig` 成功之后才注册，那么 `optional:` 且 dataId 尚不
  存在时会提前返回，永远不装 listener——之后再发布也永远不触发刷新。因此
  provider 在拉取之前**无条件**注册 listener。
- **桥接 bean 必须命名。** `gs.Rooter` 是 `any` 别名，两个默认 `__default__`
  的 Rooter export 会在 `(name, type)` 去重上撞车。稳定命名
  `nacosConfigRefreshBridge` 是关键。
- **内容解析复用 `spring/conf/reader/*`。** reader 包并未暴露“按格式名读
  bytes”的 helper；provider 直接 import 具体 `Read` 函数，用 format 名做键。
- **不能对 proxy 跑 `go mod tidy`。** `spring/*`、`stdlib/*` 靠 workspace
  `go.work` 解析。

## 4. 权衡 / 已否决方案

- **一个大而全同时管 discovery 的 starter——否决。** `config` 与 `discovery`
  在不同层；按角色拆分对齐 Spring Cloud Alibaba 的 `nacos-config` /
  `nacos-discovery`。
- **通过 IoC 复用应用的 Nacos SDK client——否决。** provider 运行在容器
  存在之前；共享 bean 只能在需要它之后才构造。按连接元组缓存能等价达到
  “每个 endpoint 一个 client”的效果，而没有顺序风险。
