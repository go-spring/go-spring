# starter-config-etcd 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`governance_etcd.go`、`governance_etcd_test.go`）、配置核心
（`spring/conf/provider/provider.go`、`spring/gs/internal/gs_conf/conf.go`、`spring/gs/internal/gs_app/app.go`）
与经冒烟验证的 [example/](example/) 核对。**etcd 自身语义（KV 模型、watch、鉴权、lease、etcdctl
用法）见 [etcd 官方文档](https://etcd.io/docs/v3.5/)**——本文只写 go-spring 的增量。

**激活——两条互相独立的路径，任何地方都没有 `enabled` 开关：**

1. **应用配置路径**：`spring.config.import` 中出现 `etcd:` 条目（如
   `spring.config.import=etcd:127.0.0.1:2379/key`）。仅 blank import 不产生任何效果。
2. **治理路径**：出现任意 `govern.source.etcd.*` key（OnProperty 对 `govern.source.etcd` 做前缀匹配）。

两条路径的数据互不触碰：应用配置 key 进应用属性（并热刷新 `gs.Dync[T]` 字段）；治理 key 只进治理中心。

---

## 1. 完整工程示例

一个把 `demo.message` 放在 etcd、不重启即可热刷新、并从第二个专用 etcd key 拉取治理
（resilience/fault）规则的服务。文件树（与冒烟验证的 `example/` 同构）：

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── docker-compose.yml        # 单节点 etcd v3.5，端口绑 127.0.0.1:2379
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring              v1.3.x
    go-spring.org/starter-config-etcd latest
    go-spring.org/starter-governance  latest   // 可选：消费治理 Source
    go.etcd.io/etcd/client/v3         v3.6.x   // 仅当应用自己要写 etcd 时需要
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-config-etcd"
    _ "go-spring.org/starter-governance"
)

// Demo 绑定来自 etcd import key 的动态字段。只有 gs.Dync[T] 字段会热刷新——
// 普通 string 字段仅启动期生效。
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}

func main() {
    // Export 为 gs.Rooter，让容器急切创建该 bean（未导出且不被注入的
    // bean 在 prod 运行中不会被装配）。
    gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())
    gs.Run()
}
```

**conf/app.properties**——完整配置面：

```properties
# --- 应用配置来自 etcd --------------------------------------------------------
# optional: 即使 key 尚不存在应用也能启动；发布后首次刷新即拿到值。
# 此处 format= 其实冗余——key 无扩展名时默认推断本就是 properties。
spring.config.import=optional:etcd:127.0.0.1:2379/gs-config-demo?format=properties

# --- 治理规则来自专用 etcd key（与 import 无关）-------------------------------
# key 存放一份完整的 govern.* 文档；推送只刷新治理，绝不触碰应用属性。
# 必填：endpoint、key。
govern.source.etcd.endpoint=127.0.0.1:2379
govern.source.etcd.key=/app/govern.yaml
```

启动前先种好两个 key（应用配置那条也可依赖 `optional:` 省略）：

```bash
docker compose up -d
ETCDCTL_API=3 etcdctl put gs-config-demo "demo.message=hello"
ETCDCTL_API=3 etcdctl put /app/govern.yaml '
govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
'
go run .
```

**验证**（与 example/check.sh 同构——冒烟脚本自断言同一热刷新链路）：

```bash
# 冷加载成功——日志出现（tag _app_config_etcd）：
#   loaded etcd config from key=gs-config-demo keys=1
# 热刷新，无需重启：
etcdctl put gs-config-demo "demo.message=hello-2"     # 绑定的 gs.Dync 字段翻新
# 治理推送，无需重启——resilience 超时参数实时生效：
etcdctl put /app/govern.yaml 'govern: {enabled: true, default: {enabled: true, attempt-timeout: 300ms}}'
```

---

## 2. 装配与时序

### 2.1 import 在生命周期中的解析位置——以及为什么这么早

```
blank-import starter-config-etcd
  └─ init(): conf.RegisterProvider("etcd", etcdController.Load)

gs.Run() → App.Start()
  1. 挂载 gs.RefreshProperties / gs.AppStarted 门面目标
  2. 刷新属性：加载 ./conf 文件 → loadFileImports 读取 spring.config.import
     → conf.Resolve（解析 source 串中的 ${...} 占位符）
     → provider.Load 解析 [optional:]etcd:<path> → etcdCtrl.Load：
        parseSource → clientFor（按 endpoint|user|pass 缓存）
        → registerWatcher（按 client+key 去重） ← watch 先于 get 装上
        → Get（5s ctx 超时）→ reader.Read(format) → flatten
  3. 初始化日志
  4. IoC 容器装配（App 为根）；所有 bean 的 value tag 此时对合并后的属性求值
  5. Runners → Servers → 就绪
```

属性必须存在于**第 4 步之前**：每个 `value:"${...}"` tag 在装配期解析，因此 etcd import 的
key 可以喂给任意 bean 的配置。etcd Get 本身发生在第 2 步——读不到的必填 import 当场让启动
失败（或被 `optional:` 跳过）。

源码核对过的要点：

- **import 只有一层**：只有顶层配置文件里的 `spring.config.import` 被读取；被 import 文档
  内部的 `spring.config.import` key 会被静默忽略（gs_conf/conf.go `loadFileImports` 注释）。
- **顺序**：import 的 source 与文件进同一分层存储；后加载的覆盖先加载的（同上注释）。
- **占位符解析**：import source 字符串先过 `conf.Resolve`，因此
  `etcd:${etcd.addr:=127.0.0.1:2379}/key` 可用。

### 2.2 watch → 刷新路径，逐步走读

```
被 watch 的 key 上发生 etcd PUT
  → clientv3.Watch channel 投递带事件的 WatchResponse
  → watcher goroutine：len(wr.Events) > 0 → etcdCtrl.TriggerRefresh()
  → gs.RefreshProperties() → App.RefreshProperties()：
       重跑整个属性加载（所有文件 + 所有 import）→ 合并
       → 传播到容器 → gs.Dync[T] 字段原子更新
```

- app 启动之前，`gs.RefreshProperties()` 返回错误，`TriggerRefresh` 是**无害 no-op**——
  初始加载已经捕获了状态（starter.go 注释）。
- watch 是**单 key** watch（无 `WithPrefix`）：只盯 import 里点名的那个精确 key。
  删除同样算事件：必填 import 的 key 被删后会打 WARN（`etcd key ... deleted; stale snapshot
  retained until the key is restored`），刷新失败也各打一条 WARN；**旧的合并属性
  继续生效**——见第 5 节。
- 刷新是全应用级的：一个 import key 变更会重读*所有* source。只有 `gs.Dync[T]` 字段会变；
  普通字段与已绑定的 bean 配置仅启动期生效。

### 2.3 治理规则推送路径

```
出现 govern.source.etcd.* 配置
  → gs.OnProperty("govern.source.etcd") 命中（前缀匹配）
  → 绑定 governEtcdConfig（endpoint/key 经 expr 校验必填）
  → 构造：etcdCtrl.clientFor（客户端与应用配置路径共享）
          → NewEtcdSource：Get key → key 缺失或文档坏 → 启动失败（fail-fast，
            错配路径绝不能让中心带着静默禁用的状态上线）
          → bean Init()：打开 Watch 流
  → 导出为 governance.Source（Snapshot + Subscribe）；中心是唯一消费者

治理 key 上发生 etcd PUT
  → watch goroutine 只处理 EventTypePut（删除等其他类型被忽略）
  → apply()：字节相同的重 PUT → no-op；解析失败 → 保留上一份好快照 + Error 日志
    （tag _app_config_etcd）；解析成功但 DeepEqual 相同 → 换快照、不推送
  → 确有变化 → cb(cfg) → 治理中心实时应用新的 resilience/fault 规则
```

文档经 `rules.Parse`（starter-governance/rules）解析——与治理 file source 字节兼容；
**完全不含 `govern.*` key 的文档是错误**而非"无治理"（截断的文档不得静默解除整个中心武装；
关闭开关是显式的 `govern.enabled=false`）。停机时中心对 `Close() error` 做类型断言并停掉
watch 流；底层 etcd 客户端是模块共享状态，source 不会关闭它。

---

## 3. 逐 key 行为参考

### 3.1 import source 串——`[optional:]etcd:<host>:<port>/<key>?<query>`

`[optional:]<provider>:<path>` 语法属于核心（spring/conf/provider/provider.go:74-104）；`etcd:`
之后的全部内容是本 starter 的 `<path>`，按 URL 解析（`etcd://` + path，见 starter.go
`parseSource`）。

| 组成 | 类型 | 默认值 | 行为/联动 | 配错后果 |
|------|------|--------|-----------|----------|
| `optional:` | 标志 | 缺省 | Get 失败**或** key 为空时跳过该 source（Warn 日志）。不豁免：source 串本身格式错误、取回内容解析失败、`dial-timeout` 值非法。 | 省略 + key 缺失 → 启动失败（符合预期）；滥用会把真实故障掩盖成"空配置"。 |
| host:port | string | —（**必填**） | 每个 source 单 endpoint；同时是客户端缓存 key 的组成部分。 | 缺失（`etcd:/key`）→ `missing etcd server address`。无集群地址列表 ⚠（见第 6 节）。 |
| key（path） | string | —（**必填**） | 去掉前导 `/`；精确单 key，无前缀/区间。扩展名参与 format 推断。 | 缺失 → `missing etcd key`。写成目录式前缀会静默读不到、也 watch 不到任何东西。 |
| `format` | string | key 扩展名，否则 `properties` | reader 注册表中的任意格式（`properties`/`yaml`/`toml`/`json`……），带不带点等价。 | 未注册格式 → 加载期 `unsupported config format`。格式对但写错 → 加载期解析错误。 |
| `username` / `password` | string | 空 | 缓存客户端上的 etcd [鉴权](https://etcd.io/docs/v3.5/op-guide/authentication/)凭证；参与客户端缓存 key，不同凭证得到不同客户端。 | 开鉴权的集群缺凭证 → Get 失败 → 启动失败（optional 则跳过）。⚠ 明文出现在 import 串里，日志/sources 可见。 |
| `dial-timeout` | duration | `5s` | `time.ParseDuration` 语法（`2s`、`500ms`）。仅客户端拨号超时——Get 本身固定用 5s context。 | 非 duration → 加载期 `invalid dial-timeout`。过低 → 慢网络下建客户端失败。 |

多条 import 组合：`spring.config.import` 接受列表；重叠 key 后者覆盖前者。每条各自拿客户端
（按 endpoint/凭证）与各自的 watcher（按 client+key 去重）。

### 3.2 治理属性面——`govern.source.etcd.*`

经 `conf.Bind` 绑定在 `${govern.source.etcd:=}` 下——这些是普通顶层属性 key（非实例前缀；
本模块只有一个实例）。

| Key | 类型 | 默认值 | 行为/联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `govern.source.etcd.endpoint` | string | —（**必填**，`expr:"$ != ''"`） | host:port 单 endpoint；与同 endpoint/凭证的 import source 共享客户端缓存。 | 空/缺失 → 装配期 bean 构造错误（OnProperty 已命中但校验失败）。 |
| `govern.source.etcd.key` | string | —（**必填**，`expr:"$ != ''"`） | 专用规则文档 key；设计上与应用配置 import key 分离。 | 空 → 同上。etcd 里无此 key → `key ... is empty`，**启动失败**（此处无 optional 模式——刻意的 fail-fast）。 |
| `govern.source.etcd.username` / `.password` | string | 空 | 共享客户端的鉴权。 | 失败模式同 import 串凭证。 |
| `govern.source.etcd.format` | string | key 扩展名，否则 `properties` | 覆盖文档格式探测；注册表同 3.1。 | 格式错 → 种子期启动失败；运行期推送被拒（保留上一份好配置）。 |

### 3.3 本 starter 没有的 key

无 `enabled` 开关（激活即 import 条目/govern key 的存在性）、无 TLS key、无 endpoints 列表、
无 watch 前缀、无刷新间隔（刷新是事件驱动的）。见第 6 节。

---

## 4. 验证与故障演练

所有演练基于第 1 节工程。前置：`docker compose up -d`、`etcdctl` 可用（`ETCDCTL_API=3`；
compose 的 etcd 未开鉴权，无需凭证）。

### 4.1 冷加载

```bash
etcdctl put gs-config-demo "demo.message=v1"
go run . &        # 日志：loaded etcd config from key=gs-config-demo keys=1（tag _app_config_etcd）
```

先删 key、并把 import 里的 `optional:` 去掉 → 启动必须失败于 `etcd key gs-config-demo is
empty`。保留 `optional:` → 应用以 `:=none` 默认值启动，并出现 Warn
`optional config key ... is empty (skipped)`。

### 4.2 watch 推送（热刷新）

```bash
go run . &
etcdctl put gs-config-demo "demo.message=v2"    # 不重启；gs.Dync 字段翻为 v2
```

这正是 example/check.sh 断言的链路（example 自发布 `hello-<ts>`，字段 15 秒内不翻即非零退出）。
注意刷新会重读所有 source——启动后在本地 conf 文件里改的值，也会在下一次 etcd 触发的刷新中翻新。

### 4.3 坏值处理

- **应用配置 key**：`etcdctl put gs-config-demo "demo.message={{{"`（配 `format=yaml`）→ 下次刷新
  解析失败；刷新中止、旧属性保留、Error 日志。启动期（非 optional）同样内容直接启动失败。
- **治理 key**：`etcdctl put /app/govern.yaml 'govern: { broken'` → Error 日志
  `got an invalid value (keeping last good config)`；中心保留旧规则。
  `etcdctl put /app/govern.yaml 'foo: bar'`（可解析但无 govern.* key）→ 同样被拒
  （`contains no govern.* keys`），保留旧配置。种子期两者均改为启动失败。
- **字节相同的重 PUT**（治理 key）→ 完全 no-op（不解析、不推送）。

### 4.4 optional 与必填 import 对比

| 演练 | 预期 |
|------|------|
| 必填 + key 缺失 | 启动错误 `etcd key ... is empty` |
| optional + key 缺失 | 正常启动，Warn 日志，`:=` 默认值生效 |
| 必填 + etcd 宕机 | 启动错误 `get etcd key ... failed` |
| optional + etcd 宕机 | 正常启动，Warn `optional config get key ... failed (skipped)` |
| optional + source 串拼写错误 | 仍是硬性启动错误——optional 从不豁免笔误 |

### 4.5 启动后连接中断

停掉 etcd（`docker compose stop`）：watcher 的 channel 静默关闭；无日志、无指标、无健康翻转。
重启 etcd：clientv3 透明重连、watch 恢复投递（etcd clientv3 语义）；宕机期间的 PUT **不会**
补播为刷新触发——用一次新的 `etcdctl put` 验证刷新是否恢复。此演练即第 6 节记录的可观测性缺口。

### 4.6 治理种子 fail-fast

```bash
etcdctl put /app/govern.yaml 'govern: {' && go run .   # 启动失败：parse ... failed
```

与 4.3 对照：运行期坏值退化为保留上一份好配置；种子期坏值直接快速失败。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 `missing etcd server address` / `missing etcd key` | import 串格式错（缺 host / 缺 path） | 修成 `etcd:<host>:<port>/<key>` 形态；`optional:` 不覆盖此类错误。 |
| 启动失败 `etcd key ... is empty` | 必填 import 指向不存在的 key（或写成了无精确 key 的前缀） | 建 key；若缺失合理则标 `optional:`。 |
| 启动失败 `get etcd key ... failed` | etcd 宕机、端口错、集群开鉴权但没给 username/password | 检查 endpoint / 加 `?username=..&password=..`；用 `etcdctl --user ... get <key>` 验证。 |
| 应用在跑但 import 的值始终不出现 | optional 条目 + 应用先于首次 PUT 启动 → 默认值生效 | PUT 该 key；watcher 触发刷新，值无需重启即到达。 |
| 绑定字段不热刷新 | 字段是普通值而非 `gs.Dync[T]` | 只有 `gs.Dync[T]` 会刷新；普通字段仅启动期生效（gs 刷新契约）。 |
| import 的 key 被删后刷新像死了 | 删除触发了刷新；必填 key 重加载失败；桥接吞掉错误并保留旧属性 | 重新 PUT 该 key；审视删 key 是否属于你的配置流程。 |
| etcdctl put 了但治理规则不变 | (a) 文档与当前字节相同或 DeepEqual 相同；(b) 值不可解析 / 无 govern.* key（Error 日志，保留旧配置）；(c) 没 import starter-governance（没有中心消费该 Source） | 查 `_app_config_etcd` 的 Error 行；补 import starter-governance。 |
| `unsupported config format` | format 参数/属性指向未注册格式 | 用已注册格式（properties/yaml/toml/json）或注册 reader。 |
| 治理 bean 装配失败 `endpoint`/`key` 为空 | `govern.source.etcd.*` 只配了一半（任一子 key 都会触发 OnProperty） | 两个必填 key 补齐，或删掉整个前缀。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置面 | import 串参数 5 个 + 治理属性 key 4 个 |
| 其中必填 | import 2 项（host、key）+ 治理 2 项 |
| quickstart 前置外部依赖 | 1（etcd；附 compose 文件） |
| 注意/坑条数 | 5（单 endpoint、明文凭证、watch 静默缺口、删除语义、仅 Dync 刷新） |

设计嫌疑清单：

- source 串只支持单 endpoint，无地址列表与 TLS 选项——生产 etcd 集群三者都需要 → 集群
  source 支持候选。
- watch 错误不上报：watch 断掉即静默配置滞留，无指标与健康信号（4.5 演练可复现）→
  可观测性增量候选。
- 必填 import key 被删除后仍保留旧属性（语义不变），但现已可见：WARN 点名被删 key，且每次刷新失败各打一条 WARN（此前完全静默）。
- 凭证以明文出现在 import 串 / 属性以及客户端缓存 key 字符串里；本 starter 无 ENC()/KMS
  专门指引。
- Get 的 5s context 写死不可调，而 dial-timeout 可调——超时面不对称。
