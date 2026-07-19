# starter-config-etcd 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-etcd` 属于 config-provider 形态（`starter/DESIGN.md` §2.5）
的集成层 starter：把 etcd 变成 Go-Spring 启动期和每次属性刷新时的远程配置源。
它夹在核心容器的 provider 机制（`spring/conf`）与 etcd v3 client 之间，自身
不持有任何配置状态。

## 1. 职责与边界

- 只在 `init()` 里通过 `conf.RegisterProvider` 注册一个 `etcd` provider 名称。
  无可注入 bean、无 server。
- 解析 provider source `etcd:<host>:<port>/<key>?<query>`，读取 key，按
  `properties`/`yaml`/`toml`/`json` 之一解析内容，并 flatten 为
  `map[string]string` 交回框架合并。
- 对 key 安装一次 etcd `Watch`，后续 put 会重跑 provider 并热更新所有绑定的
  `gs.Dync` 字段。
- **不做**服务发现。etcd 同时承担配置与目录两种能力；naming 角色按
  `starter/DESIGN.md` §2.5 中“按角色拆分”的决策放到别处。

## 2. 关键抽象与缝隙

- **Provider 缝隙。** 扩展点只有 `conf.RegisterProvider("etcd", loadEtcdConfig)`；
  应用通过 `spring.app.imports=[optional:]etcd:...` 消费。provider 运行在
  `AppConfig.Refresh` 阶段，早于任何 bean 存在，因此必须**自建 client**，不能
  通过依赖注入拿。
- **Client 缓存。** `clientv3.Client` 按 `(endpoint, username, password)`
  元组缓存。否则每次刷新都会新建 client 及其后台 goroutine——`loadEtcdConfig`
  在启动和每次 `RefreshProperties` 都会跑。
- **Refresh 钩子。** provider 侧状态是一个 `atomic.Pointer[func() error]`，
  由容器域桥接 bean `configRefreshBridge` 填充；后者注入
  `*gs.PropertiesRefresher`，导出为 `gs.Rooter` 且命名 `etcdConfigRefreshBridge`，
  确保总被实例化、且不与应用自身的默认 `__default__` Rooter 冲突。
- **Watch 缝隙。** 每个 key 一条 `cli.Watch` 通道，后台 goroutine 消费；每个
  非空事件批次触发 `triggerRefresh`。用 `(client-key, etcd-key)` 集合去重，
  避免重复 `Load` 注册并行 watcher。

## 3. 约束

- **必须先注册 watcher 再读。** `registerWatcher` 在 `cli.Get` 之前调用，这样
  `optional:` 且 key 尚不存在时也能热更新——后续 put 会触发刷新，重跑 provider
  取到新值。顺序颠倒是一次静默倒退。
- **Watch 是尽力而为。** watch 通道带错关闭时 goroutine 退出；这只是丢失该
  key 的热更新，不阻塞启动，初次拉取已经产出了值。
- **`optional:` 只吞 Get 错误，不吞解析错误。** 网络故障或缺失 key 在
  optional 下返回 `(nil, nil)`；解析错误始终致命，让格式配错立刻暴露。
- **桥接 bean 必须命名。** `gs.Rooter` 是 `any` 别名，两个默认 `__default__`
  的 Rooter export 会在 `(name, type)` 去重上撞车。稳定命名
  `etcdConfigRefreshBridge` 是关键。
- **不能对 proxy 跑 `go mod tidy`。** `spring/*`、`stdlib/*` 靠 workspace
  `go.work` 解析。

## 4. 权衡 / 已否决方案

- **一个大而全同时管 discovery 的 starter——否决。** `config` 与 `discovery`
  在不同层；按角色拆分对齐 Spring Cloud Alibaba，也让模块依赖图更干净。
- **前缀 watch（`clientv3.WithPrefix`）——刻意不暴露。** provider source
  面向一个持有配置文档的单 key；多 key 扇出交给应用，让心智模型与 Consul、
  Nacos 版本完全一致。
