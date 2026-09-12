# starter-governance-etcd

[English](README.md) | [中文](README_CN.md)

`starter-governance-etcd` 是治理规则源家族中的 etcd 成员——与
[`starter-governance`](../starter-governance/README.md) 内 file、http 两个源同款
datasource 形态。它监听一个 etcd key 上的一份治理规则文档，并把每一次变更版本
经 [`go-spring.org/cloud/governance`](../../cloud/governance) 的 `governance.Source`
契约推入治理中心。

空导入本模块在未配置 `govern.source.etcd.*` 时是惰性的；配置后注册一个
`governance.Source` Bean。key 里的文档使用与其他源相同的 `govern.*` 键，因此一份
规则文件在 file、http、etcd 各后端间逐字节可移植。

## 为何与 `starter-config-etcd` 分成两个模块

两个 starter 都说 etcd，但扮演不同角色，不能合并：

* 治理规则文档是**治理自己的**，不是应用配置导入。开启治理绝不能顺带开启全应用属性
  刷新，开启远程配置也绝不能顺带武装治理。
* [`starter-config-etcd`](../starter-config-etcd/README_CN.md) 承担
  `spring.config.import` 远程配置角色：那里一变会重读并重绑整个应用。本模块承担治理
  规则角色：它的 key 一变**只**刷新治理。
* 拆开也让本源独占自己的 client，可独立于配置导入的引导 client 携带自己的插桩。

## 安装

```bash
go get go-spring.org/starter-governance-etcd
```

## 快速开始

### 1. 引入本 starter（以及接线）

```go
import (
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-governance-etcd"
)
```

`starter-governance` 才是把注入的 `governance.Source` Bean 交给治理中心的模块；
只引入 etcd 适配器会注册 Bean 但什么都不武装。

### 2. 配置源

在[配置文件](example/conf/app.properties) 中添加两个引导键：

```properties
govern.source.etcd.endpoint=127.0.0.1:2379
govern.source.etcd.key=/app/govern.yaml
```

只有 `endpoint` 与 `key` 是必填项，认证与格式走各自默认值。

### 3. 把规则文档发布到该 key

key 中存放治理的规则，使用 `govern.*` 命名空间：

```yaml
govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
```

### 4. 运行

启动时的初始 `Get` 用来播种快照，因此 key 必须在应用启动前就存在；文档缺失或无法解析
会让启动失败。此后 key 上的每次 PUT 都被推入中心，无需重启。

## 配置项

所有配置项位于 `govern.source.etcd` 之下（精确匹配）：

| Key | 默认值 | 说明 |
|-----|--------|------|
| `endpoint` | （必填） | 要监听的 etcd 端点 |
| `key` | （必填） | 存放规则文档的 KV key |
| `username` | `""` | etcd 认证用户名；为空表示不认证 |
| `password` | `""` | etcd 认证密码 |
| `format` | key 的扩展名，否则 `properties` | 文档格式覆盖 |

## 核心行为

* **条件注册。** 仅当存在 `govern.source.etcd.*` 配置时注册 Bean（`gs.OnProperty`
  是前缀匹配），因此空导入且不配置是惰性的。
* **快速失败播种。** 构造期执行一次初始 `Get`；key 缺失或文档无法解析都会让启动
  失败，而不是静默装一个 disabled 的中心。
* **Watch 推送。** 启动后本源监听这一个 key 并应用每次 PUT：取值经共享的规则解析器
  解析，规则确有变化时才推入中心。
* **坏编辑保底。** 解析失败的值保留上一份快照并打日志，绝不推送；关闭治理的正确姿势是
  `govern.enabled=false`，不是坏文档或空文档。
* **无变更去重。** 逐字节相同的重复投递，或重新解析后配置相等，都不会推送——touch
  不会触发 executor 重建。
* **独占 client。** 本源自建并独占其 etcd client，Bean 销毁时关闭。

完整行为参考与可运行 [example](example)（`example/check.sh`，docker 门控、自校验）
见 [USAGE_CN.md](USAGE_CN.md)。
