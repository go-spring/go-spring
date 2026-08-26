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

## 说明

- 每个 `(server, appId, cluster, secret, namespace)` 元组一个 agollo client。
- 变更监听**先于首次获取**安装，因此 `optional:` 导入一个尚不存在的命名
  空间，一旦出现仍能热刷新。
- 尚未提供 `governance.Source` 接入；如需 Apollo 支撑治理规则，照
  `starter-config-nacos` 的 `governance_nacos.go` 模式添加。
