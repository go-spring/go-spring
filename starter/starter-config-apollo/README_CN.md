# starter-config-apollo

[English](README.md) | [中文](README_CN.md)

`starter-config-apollo` 接入 [Apollo](https://github.com/apolloconfig/apollo)
作为远程配置中心，构建于 github.com/apolloconfig/agollo/v4。空白导入即注册
`apollo` 配置 Provider，经 `spring.config.import` 消费；远端配置变更通过 agollo
的变更通知在运行期热刷新，无需重启。

本 starter 只承担配置中心角色。

## 安装

```bash
go get go-spring.org/starter-config-apollo
```

## 快速开始

### 1. 引入包

```go
import _ "go-spring.org/starter-config-apollo"
```

### 2. 从 Apollo 导入配置

在配置文件中使用 Provider 语法声明导入
`[optional:]apollo:<host>:<port>/<namespace>?<query>`：

```properties
spring.config.import=optional:apollo:127.0.0.1:8080/application?appId=demo
```

查询参数：

| 参数      | 默认值                            | 说明                                              |
|-----------|-----------------------------------|---------------------------------------------------|
| `appId`   | （必填）                          | Apollo 应用 id                                    |
| `cluster` | `default`                         | Apollo 集群名                                     |
| `secret`  | （空）                            | 需要访问密钥的命名空间所用的访问密钥              |
| `format`  | 命名空间扩展名，否则 `properties` | 内容格式：`properties`/`yaml`/`yml`/`json`        |

加上 `optional:` 前缀后，即使命名空间尚不存在（或为空）应用也能正常启动；发布后
其值会被自动补全。

### 3. 绑定动态字段

将导入的配置项绑定到 `gs.Dync[T]` 字段即可实现实时更新：

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

远端命名空间变更时，Provider 的变更监听器会触发一次应用属性刷新，所有绑定的
`gs.Dync` 字段都会被原子重新绑定。完整的 发布 → 热更新 流程见
[example](example/main.go)。

## 工作原理

- 每个 `(server, appId, cluster, secret, namespace)` 元组一个 agollo client ——
  agollo 在 client 创建时即固定 namespace。
- 变更监听器安装**先于**首次获取，因此导入一个尚不存在的命名空间，一旦出现仍能
  热刷新。
- 纯命名空间名（`application`）是 Apollo 原生的 `properties`；带
  `.yaml`/`.yml`/`.json` 扩展名的命名空间会让 Apollo 服务端返回该格式，Provider
  据此解析。

## 设计要点

**用 agollo v4 而非手写 HTTP client。** agollo 是官方 Apollo Go SDK，自带
`notifications/v2` 长轮询、namespace 内存缓存、本地文件备份与 secret 签名。
在标准库上重造这些——或改走没有推送通知的 Apollo OpenAPI——都等于手写通知循环，
毫无收益。唯一代价：agollo 在 client 创建时固定 `NamespaceName`，因此 namespace
进了 client 缓存 key。

**线路解析交给 agollo。** agollo 初始同步走 `/configfiles/json/...`（纯 JSON
对象，非 ApolloConfig envelope）；starter 只依赖 agollo 自身的解析，namespace
*内容*才由 `spring/conf/reader` 自行解析。

**mock config service 而非 docker 化 quick-start。** Apollo quick-start 需要
MySQL 加 configservice/admin/portal 三件套；starter 的契约是 provider seam，
example 的 mock 实现 agollo 驱动的端点（含 `notifications/v2` 长轮询），即可端到端
覆盖冷加载与热更新，无需整栈、CI 也不依赖 docker。

### 日志 tag

本模块的运行期日志使用 tag `_app_config_apollo`（apollo 配置源）。如需与主日志分开单独调整，可为该 tag 绑定独立的 logger：

```properties
logger.config_apollo.type=Logger
logger.config_apollo.level=WARN
logger.config_apollo.tag=_app_config_apollo
```

完整参考（逐 key 语义、装配时序、故障演练）见 [USAGE_CN.md](USAGE_CN.md)。
