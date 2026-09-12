# starter-governance-etcd 使用说明 — 参考手册

[English](USAGE.md) | [中文](USAGE_CN.md)

详尽使用参考。概览见 [README_CN.md](README_CN.md)。下文所有行为声明均对照 starter 源码
（`governance.go`、`governance_test.go`）、共享解析子包
[`starter-governance/rules`](../starter-governance/rules)、
[`cloud/governance`](../../cloud/governance) 的两方法 `governance.Source` 契约
（`source.go`、`global.go`）与可运行、自校验的 [example](example)
（`example/main.go`、`example/check.sh`）核实。etcd 自身客户端语义见
[etcd 官方文档](https://etcd.io/docs/latest/)——本文只写 go-spring 的增量。

**激活方式**：任一 `govern.source.etcd.*` 配置即武装条件模块（`gs.OnProperty` 是**前缀**
匹配，单个子键就能触发），并注册一个 `governance.Source` Bean。空导入且无该配置则不注册
任何东西。Bean 本身并不武装治理——
[`starter-governance`](../starter-governance/README.md) 的接线 Bean 才是把它注入中心的
那一环，需一并引入。进程只有一个生效 Source（中心只持一个），因此 file/http/etcd/nacos
只配其中一个。

---

## 1. 完整工程示例

一个治理规则放在自己 etcd key 上、热更生效的服务。文件树：

```
demo/
├── go.mod
├── main.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    go.etcd.io/etcd/client/v3        latest
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-governance     latest
    go-spring.org/starter-governance-etcd latest
)
```

**main.go** —— 全部接线；`conf/app.properties` 只装两个引导键：

```go
package main

import (
    "context"
    "fmt"
    "time"

    "go-spring.org/cloud/governance"
    "go-spring.org/spring/gs"

    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-governance-etcd"
)

type printer struct{}

func (p *printer) Run(ctx context.Context) error {
    // 有意非阻塞：阻塞的 Runner 会拖住应用启动。
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case <-time.After(time.Second):
            }
            pol := governance.PolicyFor("demo:resource")
            fmt.Printf("enabled=%v timeout=%v retries=%d\n", !pol.IsZero(), pol.Timeout, pol.MaxRetries)
        }
    }()
    return nil
}

func init() { gs.Provide(&printer{}).Export(gs.As[gs.Runner]()) }

func main() { gs.Run() }
```

**conf/app.properties** —— 全部引导面（取自 `example/conf/app.properties`）：

```properties
# 治理规则放在自己的 etcd key 上，由 starter-governance-etcd 监听——
# govern.* 下的任何东西都不走 app.properties。
govern.source.etcd.endpoint=127.0.0.1:2379
govern.source.etcd.key=/app/govern.yaml
```

**规则文档**（启动前播种到该 key）：

```yaml
govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
```

**验证**（本地 etcd，可用 `example/docker-compose.yml`）：

```bash
docker compose up -d
ETCDCTL_API=3 etcdctl put /app/govern.yaml "$(cat govern.yaml)"   # 启动前先播种
go run .
ETCDCTL_API=3 etcdctl put /app/govern.yaml 'govern: {enabled: true, default: {enabled: true, attempt-timeout: 300ms}}'
# 无需重启：源推送新文档，PolicyFor 翻到 300ms
```

可运行的 [example](example) 正是此形态：它播种 key，一秒后发布 900ms 文档，每 200ms 打印
一次解析后的策略，观察到推送后打印成功信息并自终止——`example/check.sh` 用 docker compose
包裹执行，docker 不可用时优雅跳过。

---

## 2. 装配与时序

### 2.1 Source 契约——中心真正依赖的东西

`governance.Source`（`cloud/governance/source.go`）是两个方法：

```go
type Source interface {
    Snapshot() Config                  // 最新已提交值；任何推送前为零值 Config
    Subscribe(cb func(Config))         // 每次新配置提交后调用
}
```

`EtcdSource` 实现两者，外加一对挂到 Bean 生命周期上的 `Init`/`Close`。契约有意省略的部分
（本源全部继承）：

* **无错误返回**——中心无法回滚坏推送；保留上一份好快照是源自己的事（见 §2.3）。
* **接口无 `Close`**——中心在 Destroy 时对 `interface{ Close() error }` 做类型断言并关闭
  恰好实现了它的源；`EtcdSource.Close` 取消 watch 并关闭自己的 client。
* **单一回调**——中心是唯一消费者；第二次 `Subscribe` 会替换第一次。

### 2.2 Bean 生命周期时序

```
import starter-governance-etcd
  └─ gs.Module(gs.OnProperty("govern.source.etcd"))       ← 前缀匹配即武装模块
        ├─ conf.Bind → governEtcdConfig（${endpoint}/${key} 经 expr 校验非空）
        └─ Provide newEtcdSource Bean
             .Init((*EtcdSource).Init)                    ← 打开 watch 流
             .Destroy((*EtcdSource).Close)                ← 取消 watch + 关闭 client
             .Export(gs.As[governance.Source]())
  ├─ newEtcdSource：clientv3.New(Endpoints:[endpoint], Username, Password, DialTimeout:5s)
  │     └─ NewEtcdSource：初始 Get（5s ctx）播种快照
  │           ├─ key 缺失   → "governance etcd source: key <key> is empty"   （启动报错）
  │           └─ 文档坏     → rules.Parse 错误                              （启动报错）
  ├─ import starter-governance：其接线 Bean 字段注入该 Source（autowire "?"）
  │     └─ BindDefault(src) 订阅 + 采纳 Snapshot()；GoLive() 武装中心
  ├─ 源 Bean Init：watch goroutine 启动
  └─ SIGTERM 时：wiring.Destroy() → governance.CloseActiveSource() → EtcdSource.Close()
```

本源自建并独占其 etcd client（`clientv3.New`，5s 拨号超时），与任何配置导入的引导 client
相互独立——边界说明见 [README_CN.md](README_CN.md)。

### 2.3 一次规则发布的逐层走读

1. `etcdctl put <key> <document>` 写入新值。
2. watch 流投递响应；源只对 `EventTypePut` 作出反应（`governance.go` `Init`）。无事件的响应
   （compaction、认证刷新）被忽略。
3. `apply(data)`：与上一次投递文档**逐字节相同**的值立即返回——无变更的重复 put 不推送。
4. 否则执行 `rules.Parse(key, data, format)`——与治理所有后端同一份解析器，因此一份作为本地
   规则文件可用的文档在此原样可用。解析失败会在 `_app_governance_etcd` tag 下打
   `governance etcd source: key <key> got an invalid value (keeping last good config): ...`，
   保留上一份好快照，不推送。
5. 解析成功后，新绑定的 `governance.Config` 与当前值比较（`reflect.DeepEqual`）；相等则不
   推送——touch 不会触发 executor 重建。
6. 确有变化则交换快照并调用已订阅的回调（即中心）；中心重新解析各策略并原地热换 executor
   与 fault seam。

key 上的 **DELETE** 不是 PUT，因此被忽略，上一份好快照继续生效——删文档不是关闭治理的方式。
关闭的正确姿势是 `govern.enabled=false`（一个存在的键）。

### 2.4 日志 tag

本模块运行期日志带 tag `_app_governance_etcd`
（`log.RegisterAppTag("governance_etcd", "")`）。可独立于主日志设定：

```properties
logger.governance_etcd.type=Logger
logger.governance_etcd.level=WARN
logger.governance_etcd.tag=_app_governance_etcd
```

---

## 3. 逐 key 行为参考

所有 key 位于 `govern.source.etcd` 之下（精确匹配，无宽松形态）。没有 `instances` 维度——
一个源就是一个对象，中心只持一个。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `endpoint` | string | —（**必填**，`expr:"$ != ''"`） | 被监听的 etcd 端点。只有一个地址——不是集群列表。 | 为空 → 启动绑定失败。 |
| `key` | string | —（**必填**，`expr:"$ != ''"`） | 存放规则文档的单一 KV key。`format` 未设时也用它推断格式。 | 为空 → 启动绑定失败。 |
| `username` | string | `""` | etcd 认证用户名；为空表示不认证。 | 凭据错误 → 初始 `Get` 失败 → 启动报错。 |
| `password` | string | `""` | etcd 认证密码。 | 同上。 |
| `format` | string | key 的扩展名，否则 `properties` | 覆盖文档格式检测。`"yaml" \| "json" \| "properties" \| "toml"`（共享 reader 注册表里注册的任意格式均可）。 | 错但已知的格式 → 解析错误（播种期启动报错，之后保留上一份好配置）。 |

格式推断：显式 `format` 优先；否则用 key 的点号扩展名（`/app/govern.yaml` → `yaml`）；再否则
回退 `properties`。因此无扩展名的 key 配非 properties 文档时必须显式给 `format`。

文档本身由 `rules.Parse` 解析，它要求**至少有一个 `govern.*` 键**——能解析但一个 `govern.*`
键都没有的文档（被截断或被清空）是错误，而非"没有治理"。规则键（`govern.enabled`、
`govern.default.*`、`govern.rules[n].*`、`govern.fault.*`）见
[`starter-governance`](../starter-governance/USAGE_CN.md)，各后端完全一致。

---

## 4. 验证与故障演练

前置：本地 etcd（`example/docker-compose.yml`；无认证，故无需凭据）。`etcdctl` 使用
`ETCDCTL_API=3`。

### 4.1 冷播种

```bash
ETCDCTL_API=3 etcdctl put /app/govern.yaml "$(cat govern.yaml)"
go run .        # 启动；初始 Get 播种快照
```

key 必须在应用启动前存在——源的构造期会执行一次初始 `Get`。

### 4.2 Watch 推送（热更）

```bash
go run . &
etcdctl put /app/govern.yaml 'govern: {enabled: true, default: {enabled: true, attempt-timeout: 300ms}}'
```

无需重启；`governance.PolicyFor("...")` 下次调用即翻到新超时。

### 4.3 坏值保留上一份好快照

```bash
etcdctl put /app/govern.yaml 'govern: { broken'      # 或不含任何 govern.* 键的文档
```

预期：`_app_governance_etcd` 下一条 Error 日志
`governance etcd source: key ... got an invalid value (keeping last good config): ...`；
解析出的策略不变；不推送。再次投递同样的坏值会重新解析并重新打日志，因为"上一份文档"指针
只在解析成功时才前进。

### 4.4 key 缺失 / 坏播种导致启动失败

| 演练 | 预期 |
|------|------|
| 启动前 key 不存在 | 启动报错 `governance etcd source: key <key> is empty` |
| key 存放无法解析的文档 | `rules.Parse` 报错（该文档无法被武装） |
| etcd 不可达 | 启动报错 `governance etcd source: get <key> failed` |
| `endpoint`/`key` 只配了一个 | `expr:"$ != ''"` 绑定报错（任一子键都会触发模块） |

### 4.5 删除 key

`etcdctl del /app/govern.yaml` 触发 DELETE 事件，源会忽略它（只应用 PUT）。治理保持上一份
好规则，且无日志。要关闭治理，请发布 `govern.enabled=false` 的文档。

### 4.6 冒烟测试

```bash
cd example && ./check.sh    # docker 门控：compose 起 etcd，跑自校验 example
```

`example/check.sh` 拉起 etcd、运行 example，只有观察到被推送的规则并干净停机才退出 0；docker
或 docker compose 不可用时打印警告并退出 0。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| key 变了但治理仍关（`enabled=false`） | 未引入 `starter-governance` → 无接线 Bean，Source 从未被注入 | 引入 `go-spring.org/starter-governance`。 |
| 启动报 `key <key> is empty` | 启动前必填 key 不存在 | 启动前先播种 key。 |
| 启动报 `get <key> failed` | etcd 不可达 / 端口错 / 需认证却未配 `username`/`password` | 检查端点与凭据。 |
| 启动报 `endpoint`/`key` 为空 | `govern.source.etcd.*` 只配了一部分（任一子键都会触发 `OnProperty`） | 补齐两个必填键，或删掉整个前缀。 |
| `put` 之后规则不变 | (a) 值与当前逐字节相同或 DeepEqual 相等；(b) 值不可解析 / 不含 `govern.*` 键（Error 日志，保留上一份好配置）；(c) 另一个源已占用中心的唯一源位 | 看 `_app_governance_etcd` 日志行；只配一个源。 |
| 删 key 后治理仍是旧规则 | DELETE 不是 PUT，按设计被忽略 | 重新 PUT 一份文档；要关闭用 `govern.enabled=false`。 |
| 无扩展名的 key 报解析错误 | 格式推断回退到了 `properties` | 设置 `govern.source.etcd.format`。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 5 |
| 其中必填 | 2（`endpoint`、`key`） |
| quickstart 前置外部依赖 | 1（etcd，附 compose 文件） |
| "注意/坑"条数 | 3（单端点、无 TLS key、watch 静默间隙） |

设计嫌疑清单：

- **单端点、无集群列表、无 TLS key。** `endpoint` 是一个字符串，client 以 5s 拨号超时且无
  TLS 配置构建——生产 etcd 集群通常需要节点列表与 TLS。候选增量，与
  `starter-config-etcd` 记录的同类缺口一致。
- **无 watch 间隙暴露。** watch 掉线/compaction 或认证刷新被拒时不产生任何日志或指标；
  DELETE 被静默忽略。可观测性增量候选。
- **共享解析语义**是文档跨后端可移植的原因；`rules.Parse` 的任何变更（例如"必须含
  `govern.*` 键"这条规则）都会同时作用于所有源。
