# starter-config-vault 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-vault` 属于 config-provider 形态（`starter/DESIGN.md` §2.5）
的集成层 starter：把 HashiCorp Vault 变成 Go-Spring 启动期和每次属性刷新时
的远程配置源。它读取一个 KV secret 并把字段当应用属性暴露；与
`spring/conf/decrypt` 的属性级解密缝隙可以自然叠用，但两者互相独立。

## 1. 职责与边界

- 只在 `init()` 里通过 `conf.RegisterProvider` 注册一个 `vault` provider 名称，
  再无别的顶层动作——无可注入 bean、无 server。
- 解析 provider source
  `vault:<host>:<port>/<mount>/<path>?kv-version=&namespace=&scheme=&key=&format=&prefix=&poll-ms=`，
  按 KV v1 或 v2 读取 secret，并返回 flatten 后的 property map。
- 支持两种模式：
  - **whole-secret 模式**（默认）：KV 每个字段都变成属性。
  - **single-field 模式**（`?key=...`）：把某一字段当文档，按 `format` 解析。
- 可通过 `?prefix=<ns>` 为输出的 key 加前缀。
- 通过轮询 watcher 检测 secret 数据指纹变化，触发 provider 重跑。

## 2. 关键抽象与缝隙

- **Provider 缝隙。** 扩展点只有 `conf.RegisterProvider("vault", loadVaultConfig)`；
  应用通过 `spring.app.imports=[optional:]vault:...` 消费。provider 运行在
  `AppConfig.Refresh` 阶段，早于任何 bean 存在。
- **Token 走带外通道解析。** 顺序：`?token=` 查询串（不推荐）→ `VAULT_TOKEN`
  环境变量 → 由 `?token-file=` 或 `VAULT_TOKEN_FILE` 指定的 token 文件。
  这样 token 不会进入任何应用绑定的配置文件。
- **Client 缓存。** Vault API client 按 `(address, namespace, token)`
  元组缓存，刷新时不会重复新建 client。
- **Refresh 钩子。** 容器域桥接 bean `configRefreshBridge`（命名
  `vaultConfigRefreshBridge`，导出 `gs.Rooter`）注入 `*gs.PropertiesRefresher`，
  把 `RefreshProperties` 存入 `atomic.Pointer[func() error]`。
- **Watch 缝隙——共享指纹。** 每个 `(client, mount, path)` 对应一个共享
  `loadedFP[watchKey]` 字符串。每次 `loadVaultConfig` 成功后都写入；轮询循环
  拿本次轮询的指纹与之比较。不一致时调 `triggerRefresh` 重跑 provider，
  provider 又会更新指纹，因此一次变更**恰好触发一次**。

## 3. 约束

- **必须先注册 watcher 再读。** `registerWatch` 在 secret 拉取之前调用，这样
  `optional:` 且 secret 尚不存在时也能热更新。
- **不要让 watcher 用“首次自己轮询到的值”做基线。** 朴素做法（首次轮询做
  基线）会静默丢掉“启动时 optional secret 不存在、之后被创建”的场景：首轮
  已经读到新值并当基线。改用共享 `loadedFP`（即应用真正加载到的值）做基线
  即可修复。
- **启动不会引起伪刷新。** `loadVaultConfig` 返回前会先播种 `loadedFP`，
  首轮轮询与该基线而非零值比较。
- **Vault token 永远不进入绑定的属性。** token 有意从 env / token 文件 /
  查询串解析，不从 `spring.config.vault.*` 读；这样自身读属性的解密缝隙
  就不会陷入鸡生蛋循环。
- **桥接 bean 必须命名。** 与其他 config-provider starter 一致：
  `gs.Rooter` 是 `any` 别名，需要稳定命名（`vaultConfigRefreshBridge`）
  以避免 `__default__` 冲突。

## 4. 权衡 / 已否决方案

- **Vault Agent / CSI 挂载文件——交给 `starter-config-file`。** 那个 starter
  已覆盖文件挂载场景；本 starter 直接与 Vault API 通信，服务于集群侧场景
  （动态 secret、非文件挂载、按请求鉴权）。
- **Push 通知——Vault 无原生 push。** 只能轮询，因此设计把精力放在
  “轻量轮询 + 恰好一次检测”，而非造一层假 push 接口。
