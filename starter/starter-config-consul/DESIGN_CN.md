# starter-config-consul 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-consul` 属于 config-provider 形态（`starter/DESIGN.md` §2.5）
的集成层 starter：把 Consul KV 变成 Go-Spring 启动期和每次属性刷新时的远程
配置源。它夹在核心容器的 provider 机制（`spring/conf`）与 Consul HTTP API
之间，自身不持有任何配置状态。

## 1. 职责与边界

- 只在 `init()` 里通过 `conf.RegisterProvider` 注册一个 `consul` provider 名称，
  再无别的顶层动作——无可注入 bean、无 server。
- 解析 provider source `consul:<host>:<port>/<kv-path>?<query>`，读取 KV 路径，
  按 `properties`/`yaml`/`toml`/`json` 之一解析内容，并 flatten 为
  `map[string]string` 交回框架合并。
- 通过阻塞查询 watcher（见 §2）监听 KV 变更，触发一次 provider 重跑，让绑定的
  `gs.Dync` 字段无需重启即热更新。
- **不做**服务发现。Consul 同时承担配置与目录两种能力；naming 角色按
  `starter/DESIGN.md` §2.5 中“按角色拆分”的决策放到别处。

## 2. 关键抽象与缝隙

- **Provider 缝隙。** 扩展点只有 `conf.RegisterProvider("consul", loadConsulConfig)`；
  应用通过 `spring.app.imports=[optional:]consul:...` 消费。provider 运行在
  `AppConfig.Refresh` 阶段，早于任何 bean 存在，因此必须**自建 client**，不能
  通过依赖注入拿。
- **Client 缓存。** Consul 客户端按 `(address, scheme, token, datacenter)`
  元组缓存。否则每次刷新都会新建 client 及其空闲连接——`loadConsulConfig`
  在启动和每次 `RefreshProperties` 都会跑。
- **Refresh 钩子。** provider 侧状态是一个 `atomic.Pointer[func() error]`，
  由容器域桥接 bean `configRefreshBridge` 填充；后者注入
  `*gs.PropertiesRefresher`，导出为 `gs.Rooter` 且命名 `consulConfigRefreshBridge`，
  确保总被实例化、且不与应用自身的默认 `__default__` Rooter（`any` 别名）冲突。
- **Watch 缝隙。** 每个 KV 路径一条后台 goroutine 跑阻塞查询
  （`WaitIndex`、`WaitTime=5m`）。用 `(client-key, kv-path)` 集合去重，避免重复
  `Load` 拉起并行 watcher。

## 3. 约束

- **必须先注册 watcher 再读。** `registerWatch` 在 `KV().Get` 之前调用，这样
  `optional:` 且 key 尚不存在时也能热更新——后续写入会触发刷新，重跑 provider
  取到新值。顺序颠倒是一次静默倒退。
- **首次响应作为 wait index 基线。** watch 循环把首次成功轮询记为基线，仅在
  后续 `LastIndex` 增大时才触发 `triggerRefresh`，所以启动本身不会造成一次
  伪刷新。
- **处理索引倒退。** Consul 状态重置后可能返回倒退的 `LastIndex`；按官方阻塞
  查询指引，此时循环重置为 `0`。
- **桥接 bean 必须命名。** `gs.Rooter` 是 `any` 别名，两个默认 `__default__`
  的 Rooter export 会在 `(name, type)` 去重上撞车。稳定命名
  `consulConfigRefreshBridge` 是关键。
- **不能对 proxy 跑 `go mod tidy`。** `spring/*`、`stdlib/*` 靠 workspace
  `go.work` 解析；tidy 会把它们送去 module proxy 而 404。

## 4. 权衡 / 已否决方案

- **一个大而全同时管 discovery 的 starter——否决。** `config` 与 `discovery`
  在不同层（provider 注册 vs bean 装配）；按角色拆分对齐 Spring Cloud Alibaba
  的 `nacos-config` / `nacos-discovery`，也让模块依赖图更干净。
- **长轮询库 / `github.com/hashicorp/consul/api/watch`——目前否决。** provider
  只需要“索引自上次是否变化”，手写阻塞 `KV().Get` 约 30 行即可，watcher 结构
  与 etcd 版本保持一致。
