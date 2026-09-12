# starter-governance-nacos

[English](README.md) | [中文](README_CN.md)

`starter-governance-nacos` 把 [Nacos](https://nacos.io/) 接入为 Go-Spring 的治理规则源。
它监听一个承载治理规则文档的 Nacos dataId，把每个已发布的版本经 `governance.Source`
契约推送进 [`go-spring.org/cloud/governance`](../../../cloud/governance) 治理中心——
规则变更实时重解析治理策略，无需重启，也不触发全应用属性重绑。

它是治理规则源适配器家族（Sentinel-datasource 形态）的 Nacos 成员；etcd 成员是
[starter-governance-etcd](../../starter-governance-etcd)，file/http 成员在
[starter-governance](../../starter-governance)。三者都经 `rules.Parse` 解析同一份规则文档，
所以文档在各后端之间逐字节可移植。

## 与 starter-config-nacos 的关系

本模块刻意与 [starter-config-nacos](../starter-config-nacos) 分离，尽管两者都对接 Nacos：

* `starter-config-nacos` 承担 `spring.config.import` 的**远程配置**角色——把应用属性拉入合并后的
  属性存储，并热更新到 `gs.Dync[T]` 字段。
* `starter-governance-nacos` 承担**治理规则文档**角色。该文档是治理自己的；`app.properties`
  里只放它的引导 key，文档变更只刷新治理。

两者互不开启对方：启用治理不会启用应用属性刷新，导入远程配置也不会武装治理。本规则源还独占
自建并持有自己的 Nacos 客户端，因此可以独立于配置导入的引导客户端携带自己的插桩。

## 安装

```bash
go get go-spring.org/starter-governance-nacos
```

## 快速开始

### 1. 引入包

```go
import _ "go-spring.org/starter-governance-nacos"
```

只有存在 `govern.source.nacos.*` 配置项时才会注册 Bean，因此空导入在其余情况下是惰性的。

### 2. 配置规则源

在[配置文件](example/conf/app.properties)中添加引导 key：

```properties
govern.source.nacos.server=127.0.0.1:8848
govern.source.nacos.data-id=app-govern.yaml
govern.source.nacos.group=DEFAULT_GROUP
```

`server` 与 `data-id` 是必填项，其余字段都有默认值（见[配置项](#配置项)）。dataId 必须已存在：
规则源构造时会先做一次 `GetConfig`，因此 dataId 缺失或文档不可解析会在启动阶段报错，而不是
静默装一个 disabled 中心。

### 3. 发布规则文档

把规则放进它自己的 dataId，键与 `app.properties` 条目完全相同：

```yaml
govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
  rules:
    - resources: demo:resource
      attempt-timeout: 50ms
```

每个已发布版本都会被重新解析并推送进治理中心。规则词表（`govern.enabled`、`govern.default.*`、
`govern.rules[n].*`、`govern.fault.*`）属于治理域，完整参考见
[starter-governance 的 USAGE](../../starter-governance/USAGE.md)。引入治理中心本身仍是
[starter-governance](../../starter-governance) 的职责——本模块只提供规则源。

## 配置项

所有配置项挂在 `govern.source.nacos` 之下：

| Key         | 默认值                 | 说明                                          |
|-------------|------------------------|-----------------------------------------------|
| `server`    | （必填）               | Nacos 服务地址，`host:port`，单服务端         |
| `data-id`   | （必填）               | 承载规则文档的 dataId                         |
| `group`     | `DEFAULT_GROUP`        | Nacos 分组                                    |
| `namespace` | `""`（public）         | 命名空间 id（非名称）                          |
| `username`  | `""`                   | 鉴权用户名；为空表示不鉴权                     |
| `password`  | `""`                   | 鉴权密码                                      |
| `format`    | dataId 后缀，否则 `properties` | `properties` / `yaml` / `toml` / `json` |

## 核心行为

* **启动即校验。** 构造时调用一次 `GetConfig` 并解析结果。dataId 缺失、客户端报错、或文档不可
  解析都会使构造（及启动）失败，因此配置错误的规则源不会静默关闭治理。
* **发布即推送。** `ListenConfig` 投递每个已发布版本；仅当规则确实变化时才重新解析并推送。
* **坏文档保底。** 坏发布保留上一份好快照并打日志；治理中心永远不会收到未经背书的配置。
* **去重。** 逐字节相同的重复投递（Nacos 重连时可能重推）是 no-op；语义未变的文档不会引起
  executor 抖动。
* **独占客户端。** `Close` 摘除监听器并关闭 Bean 自建的客户端；规则源实现了中心在 Destroy 时
  探测的可选关闭契约。

### 日志 tag

本模块的运行期日志使用 tag `_app_governance_nacos`（nacos 治理源）。如需与主日志分开单独调整，
可为该 tag 绑定独立的 logger：

```properties
logger.governance_nacos.type=Logger
logger.governance_nacos.level=WARN
logger.governance_nacos.tag=_app_governance_nacos
```

## 示例

可运行的 [example/](example) 在启动前先把被监听的 dataId 播种，随后发布一份更新后的文档并自断言
解析出的策略已变化，最后以 0 退出。配合一个 standalone Nacos 运行：

```bash
cd example && ./check.sh
```

`check.sh` 受 docker 门控（docker 或 compose 命令缺失时优雅跳过），用 `docker-compose.yml`
拉起 Nacos，等待 Nacos 就绪端点（Nacos 启动慢），在看门狗下运行示例，并在退出时拆除容器。

完整参考见 [USAGE_CN.md](USAGE_CN.md)。
