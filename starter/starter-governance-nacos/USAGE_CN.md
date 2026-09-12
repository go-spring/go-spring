# starter-governance-nacos 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。以下每条行为断言都对照过 starter
源码（`governance.go`、`governance_test.go`）、共享解析胶水
[starter-governance/rules](../../starter-governance/rules/rules.go)、核心契约
[cloud/governance](../../../cloud/governance)（`source.go`），以及自断言的
[example/](example)（`example/example.go`、`example/check.sh`）。Nacos 自身的语义（dataId、
group、namespace、`ListenConfig`）见 [Nacos 文档](https://nacos.io/docs/latest/manual/admin/config/)——
以下都是 go-spring 的增量。

**本 starter 是什么**：治理规则源家族的 Nacos 适配器，是一个 `governance.Source` 实现；
治理中心本身的呈现是 [starter-governance](../../starter-governance) 的职责。存在
`govern.source.nacos.*` 配置项之前，空导入本包是惰性的。

---

## 1. 完整工程示例

一个把治理规则放进自己的 Nacos dataId、并在每次发布时热更新的服务。以下是签入的示例（节选），
完整流程见 [example/example.go](example/example.go)。

```
demo/
├── go.mod
├── main.go            (example.go)
└── conf/
    └── app.properties
```

**go.mod**（关键模块依赖）：

```
require (
    go-spring.org/spring                  v1.3.x
    go-spring.org/starter-governance-nacos latest
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/cloud/governance"
    "go-spring.org/spring/gs"

    _ "go-spring.org/starter-governance-nacos"
)

// poller 观察某个 label 解析出的策略，因此无需任何客户端接线即可看到规则推送。
type poller struct{}

func (p *poller) Run(ctx context.Context) error {
    // ... 约 1 秒后发布一份新文档，然后轮询 governance.PolicyFor，
    //     直到观察到被推送的 timeout ...
    pol := governance.PolicyFor("demo:resource")
    fmt.Printf("policy: enabled=%v timeout=%v retries=%d\n", !pol.IsZero(), pol.Timeout, pol.MaxRetries)
}

func init() {
    gs.Provide(&poller{}).Export(gs.As[gs.Runner]())
}

func main() { gs.Run() }
```

**conf/app.properties**——全部接线：

```properties
# 治理规则放在它自己的 dataId 里，由 starter-governance-nacos 监听——
# app.properties 里不含任何 govern.* 键。下面这段就是全部接线：它武装条件模块，
# 其 Source Bean 被注入治理中心。
govern.source.nacos.server=127.0.0.1:8848
govern.source.nacos.data-id=gs-govern-demo.yaml
govern.source.nacos.group=DEFAULT_GROUP
```

**规则文档**（发布到 dataId，不存于 `app.properties`）：

```yaml
govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
    max-retries: 2
```

**验证**（配合本地 Nacos，例如 [example/docker-compose.yml](example/docker-compose.yml)）：

```bash
# 通过 HTTP open API 播种 dataId，然后运行：
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
     -d 'dataId=gs-govern-demo.yaml&group=DEFAULT_GROUP&content=<上面的文档>'
go run . -manual
# policy: enabled=true timeout=100ms retries=2
#   ... 向同一 dataId 发布 attempt-timeout: 900ms ...
# rule push observed: attempt-timeout is now 900ms
```

可运行的 [example/](example) 完成上述全部动作、自断言并以 0 退出；
[example/check.sh](example/check.sh) 把它包进 docker compose。

---

## 2. 装配与时序

### 2.1 Source 契约——中心真正依赖的东西

`cloud/governance` 与容器无关：其中心只依赖两方法的 `Source` 接口（`cloud/governance/source.go`）：

```go
type Source interface {
    Snapshot() Config                       // 最近一次已提交的值；任何推送前为零值 Config
    Subscribe(cb func(Config))              // 每个新配置提交后回调
}
```

刻意省略（取自接口文档注释）：

- **无 error 返回**——中心无法回滚一次坏推送。「你推什么，你就为它背书」：坏文档时保留上一份
  好快照是规则源的责任（本适配器正是如此）。
- **接口无 Close**——生命周期属于实现；中心在 Destroy 时类型断言 `interface{ Close() error }`，
  关闭恰好实现了它的规则源。
- **单一回调**——中心是唯一消费者；第二次 Subscribe 可能替换第一次。

`NacosSource` 实现 `Snapshot`/`Subscribe`，并为 gs Bean 生命周期额外导出 `Init`/`Close`，
因此中心会在 Destroy 时关闭它。

### 2.2 Bean 生命周期时序

```
空导入 starter-governance-nacos
  └─ init() governance.go: gs.Module(gs.OnProperty("govern.source.nacos"), ...)
         （OnProperty 是前缀匹配：任何 govern.source.nacos.* 键都会武装它）
       ├─ conf.Bind(p, &c, "${govern.source.nacos:=}")   绑定并 expr 校验各 key
       └─ Provide newNacosSource:
            clients.NewConfigClient（namespace、5s 超时、鉴权、NotLoadCacheAtStart）
              → NewNacosSource: 初始 GetConfig + rules.Parse   ← 在此快速失败
            .Init((*NacosSource).Init).Destroy((*NacosSource).Close)
            .Export(gs.As[governance.Source]())

gs.Run()
  ├─ Bean 装配：导出的 Source Bean 被注入 starter-governance 的 wiring
  ├─ 规则源 Bean Init: ListenConfig 安装 OnChange 监听器
  ├─ 你的 Runner 运行（治理已武装——Rooter 先于 Runner）
  └─ SIGTERM 时：规则源 Bean Destroy → Close：CancelListenConfig + CloseClient
```

**两点须知：**

- **Export 是承重的。** 缺少 `Export(gs.As[governance.Source]())` 时，Bean 对中心的接口注入不可见，
  治理会保持 disabled。本 starter 已正确接线；手写的 Source Bean 也必须如此导出自己。
- **启动即校验。** `NewNacosSource` 在 Bean 存在之前就调用 `GetConfig` 与 `rules.Parse`。
  dataId 缺失、客户端报错、或文档不可解析都会使构造（进而启动）失败，而不是静默装一个 disabled
  中心（[`TestNacosSource_BadSeedFailsFast`](governance_test.go) 钉住了这一点）。

### 2.3 一次规则发布，端到端

1. 发布方向 dataId 写入内容（控制台、HTTP open API、SDK，任选）。
2. Nacos 把新内容投递给 `Init` 安装的监听器（`ListenConfig` → `OnChange` → `NacosSource.apply`）。
3. `apply` 先比字节：逐字节相同的重复投递（Nacos 重连时可能重推）是 no-op。否则经 `rules.Parse`
   重新解析；坏文档以 tag `_app_governance_nacos` 打日志并保留上一份好快照，不推送任何东西。
4. 解析成功的配置用 `reflect.DeepEqual` 与当前快照去重；仅当规则确实变化时才替换快照并触发 `cb`。
5. 中心订阅的回调采纳该配置，为每个已注册 label 重新解析策略并热替换 fault——因此一次推送在下一次
   调用即生效，无需重启，也不触发全应用重绑。

### 2.4 格式解析

文档格式由 `sourceFormat` 解析（与 etcd 兄弟模块同款）：显式的 `format` 键优先，否则取 dataId 的
点号后缀，否则 `properties`。dataId 带受支持后缀（`app-govern.yaml`、`app-govern.json`、
`app-govern.toml`）时无需 `format` 键；无后缀或未知后缀的 dataId 默认为 `properties`。

`rules.Parse` 经相同的 `conf` value-tag 机制以前缀 `govern` 绑定，并拒绝「能解析但不含任何
`govern.*` 键」的文档（被截断或清空的文档）——关闭治理的正确姿势是 `govern.enabled=false`，
一个确实存在的键。

---

## 3. 逐 key 行为参考

所有配置项挂在 `govern.source.nacos` 之下，经 `conf.Bind` 用显式前缀 `${govern.source.nacos:=}`
绑定（精确匹配，无归一化形态）。这是本 starter 唯一的对外接口面。

| Key | 类型 | 默认值 | 必填 | 行为 / 交互 | 错配后果 |
|-----|------|--------|------|-------------|----------|
| `server` | string | — | 是（`expr:"$ != ''"`） | Nacos 服务地址，`host:port` 形态；仅单服务端（无集群列表）。拆分为 host + 数字端口。 | 非 `host:port` → 启动报错 `nacos server address must be host:port`；端口非数字 → `invalid nacos server port ...`。 |
| `data-id` | string | — | 是（`expr:"$ != ''"`） | 专用规则 dataId——不要复用应用配置的 dataId。也驱动格式推断。 | 为空 → 启动绑定报错；服务端不存在该 dataId → 启动时 `get <group>/<dataId> failed`（此接口面没有 `optional:` 行为）。 |
| `group` | string | `DEFAULT_GROUP` | 否 | dataId 所在的 Nacos 分组。 | 分组错误 → 找不到 dataId → 启动 `get ... failed`。 |
| `namespace` | string | `""`（public） | 否 | 命名空间 **id**（非名称）。 | 传名称而非 id → 找不到 dataId → 启动失败。 |
| `username` / `password` | string | `""` | 否 | 服务端鉴权；为空表示不鉴权。 | 开启鉴权的服务端缺凭据 → 启动时 `get ... failed`。 |
| `format` | string | dataId 后缀，否则 `properties` | 否 | `properties` / `yaml` / `toml` / `json`；覆盖推断。 | 格式不符 → 解析失败：播种失败使启动报错，后续坏发布保留上一份好配置。 |

⚠ 没有 `timeout` 键：SDK 客户端超时固定为 5000 ms（`governance.go` 中的 `dialTimeoutMs`），
这与 `starter-config-nacos` import 串的 `timeout-ms` 不同。

⚠ `govern.source.*` 只是引导面。规则文档本身永不走 `app.properties`——它住在自己的 dataId 里，
其键是 `govern.*` 词表，文档见 [starter-governance 的 USAGE](../../starter-governance/USAGE.md)。

### 3.1 文档的逐字节可移植性

文档由 `starter-governance` 的 file 源与 http 源共用的同一个 `rules.Parse` 解析：先扁平化，
要求至少含一个 `govern.*` 键，再绑定进 `governance.Config`。因此一份能作为本地规则文件工作的文档，
作为 Nacos dataId（反之亦然，以及作为 etcd 值配合
[starter-governance-etcd](../../starter-governance-etcd)）也能原样工作。

---

## 4. 验证与故障演练

### 4.1 热推送（示例应用）

```bash
cd example && ./check.sh        # docker 门控：compose 拉起 Nacos，运行自断言示例
```

示例在启动前用 100 ms 的 `attempt-timeout` 播种 dataId（使规则源的初始 `GetConfig` 成功），随后
发布一份 900 ms 的文档；poller 观察到 `governance.PolicyFor("demo:resource").Timeout == 900ms`
后以 0 退出。

### 4.2 坏发布保留上一份好配置（手动）

应用运行中，向 dataId 发布一份被截断的文档：

```bash
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
     -d 'dataId=gs-govern-demo.yaml&group=DEFAULT_GROUP&content=govern: { broken'
# 日志：governance nacos source: DEFAULT_GROUP/gs-govern-demo.yaml published an
#      invalid document (keeping last good config): ...
# 解析出的策略不变
```

[`TestNacosSource_PushChain`](governance_test.go) 钉住了整条链路：播种 → 发布解析并推送一次 →
坏发布保留上一份好快照且不推送 → 逐字节相同的重推是 no-op。

### 4.3 启动时 dataId 不存在

把 `data-id` 指向服务端上不存在的名字并启动：构造时的 `GetConfig` 失败，启动中止。此接口面没有
`optional:` 逃生口——请先创建 dataId（示例正是这么做的）。

### 4.4 冒烟测试

```bash
cd example && ./check.sh
```

docker 或 compose 命令不可用时，`check.sh` 优雅跳过。否则它拉起 standalone 模式的
`nacos/nacos-server:v2.4.3`，轮询 `/nacos/v1/console/health/readiness` 最多 3 分钟（Nacos 启动慢；
裸 TCP 探测会与示例的首次发布竞争），在 60 s 看门狗下运行示例，并在退出时拆除容器。

---

## 5. 排障表

| 症状 | 可能原因 | 处理 |
|------|----------|------|
| 启动报错 `nacos server address must be host:port` | `server` 无 `:` 或 host/port 为空 | 使用 `host:port`，单服务端。 |
| 启动报错 `governance nacos source: get <group>/<dataId> failed` | dataId 缺失、分组/命名空间错误、凭据错误、或服务端宕机 | 在正确的分组/命名空间下创建 dataId；检查鉴权。 |
| 启动在 `rules.Parse` 内报错 | dataId 里的文档不可解析或不含 `govern.*` 键 | 先发布一份合法文档——播种按设计快速失败。 |
| 配了治理，`PolicyFor` 却始终为零 | 文档里 `govern.enabled` 为 false（默认） | 设为 `govern.enabled=true`——它是总开关。 |
| 一次发布没改变策略 | 日志出现 `published an invalid document (keeping last good config)` | 修正文档；「关闭」是 `govern.enabled=false`，不是空文档。 |
| 发布内容完全相同却毫无反应 | 设计如此——逐字节相同的重复投递与 DeepEqual 相等的文档都不推送 | 符合预期。 |
| 自定义 Source Bean 被静默忽略 | 缺少 `Export(gs.As[governance.Source]())` | 补上 Export——否则该 Bean 对接口注入不可见。 |
| 规则变更始终不到达 | Nacos 连通性，或推送目标不是被监听的 `(group, dataId)` | 确认发布落在配置的 group/dataId 上。 |

---

## 6. 设计体检表

| 指标 | 值 |
|------|-----|
| 配置项 | 7（`server`、`data-id`、`group`、`namespace`、`username`、`password`、`format`） |
| 必填 | 2（`server`、`data-id`） |
| 快速开始外部依赖 | 1（Nacos） |
| ⚠ 注意项 | 2 |

设计备注（供审计台账）：

- 播种快速失败是刻意的：与配置导入角色不同，此接口面没有 `optional:` 模式，因此配置错误的 dataId
  无法静默装一个 disabled 中心。代价是 dataId 必须在应用启动前存在——对于一份缺失即不可见的规则
  文档，这是可接受的。
- 解析胶水（`rules.Parse`）作为 `starter-governance` 的子包安放，而非放进 `cloud/governance`，
  这样容器无关的核心保持不含 `spring` 依赖，同时各后端共享同一个解析器。
- etcd 兄弟适配器（[starter-governance-etcd](../../starter-governance-etcd)）形态相同；两者保持
  结构对齐，便于读者在两者之间迁移。
