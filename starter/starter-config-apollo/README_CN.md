# starter-config-apollo

[English](README.md) | [中文](README_CN.md)

`starter-config-apollo` 接入 [Apollo](https://github.com/apolloconfig/apollo)
作为远程配置中心。空白导入即注册 `apollo` 配置 provider，经
`spring.config.import` 消费，通过 agollo 的变更通知实现实时热刷新。

## 安装

```bash
go get go-spring.org/starter-config-apollo
```

## 快速开始

### 1. 导入

```go
import _ "go-spring.org/starter-config-apollo"
```

### 2. 配置

```properties
spring.config.import=optional:apollo:127.0.0.1:8080/application?appId=demo&format=properties
```

### 3. 使用

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

## Source 语法

```
apollo:<host>:<port>/<namespace>?appId=&cluster=&secret=&format=
```

- `namespace` — Apollo 命名空间（如 `application`、`application.properties`）
- `appId` — 必填
- `cluster` — 默认 `default`
- `secret` — 可选访问密钥（带访问密钥的命名空间）
- `format` — 配置格式；默认按命名空间后缀推断，否则 `properties`

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
example 的 mock 恰好实现 agollo 所需的两个端点，即可端到端覆盖，无需整栈、
CI 也不依赖 docker。

### 日志 tag

本模块的运行期日志使用 tag `_app_config_apollo`（apollo 配置源）。如需与主日志分开单独调整，可为该 tag 绑定独立的 logger：

```properties
logger.config_apollo.type=Logger
logger.config_apollo.level=WARN
logger.config_apollo.tag=_app_config_apollo
```

## 说明

- 每个 `(server, appId, cluster, secret, namespace)` 元组一个 agollo client。
- 变更监听**先于首次获取**安装，因此 `optional:` 导入一个尚不存在的命名
  空间，一旦出现仍能热刷新。
- 尚未提供 `governance.Source` 接入；如需 Apollo 支撑治理规则，照
  `starter-config-nacos` 的 `governance.go` 模式添加。
