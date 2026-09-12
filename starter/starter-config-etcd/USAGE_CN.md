# starter-config-etcd 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`）、配置核心
（`spring/conf/provider/provider.go`、`spring/gs/internal/gs_conf/conf.go`、`spring/gs/internal/gs_app/app.go`）
与经冒烟验证的 [example/](example/) 核对。**etcd 自身语义（KV 模型、watch、鉴权、lease、etcdctl
用法）见 [etcd 官方文档](https://etcd.io/docs/v3.5/)**——本文只写 go-spring 的增量。

**激活——只有一条路径，任何地方都没有 `enabled` 开关：** `spring.config.import` 中出现
`etcd:` 条目（如 `spring.config.import=etcd:127.0.0.1:2379/key`）。仅 blank import 不产生
任何效果。导入的 key 进应用属性，并热刷新 `gs.Dync[T]` 字段。

etcd 治理规则源现已独立为 `go-spring.org/starter-governance-etcd` 模块（`govern.source.etcd.*`）。

---

## 1. 完整工程示例

一个把 `demo.message` 放在 etcd、不重启即可热刷新的服务。文件树（与冒烟验证的
`example/` 同构）：

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
    go.etcd.io/etcd/client/v3         v3.6.x   // 仅当应用自己要写 etcd 时需要
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-config-etcd"
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
```

启动前先种好 key（也可依赖 `optional:` 省略）：

```bash
docker compose up -d
ETCDCTL_API=3 etcdctl put gs-config-demo "demo.message=hello"
go run .
```

**验证**（与 example/check.sh 同构——冒烟脚本自断言同一热刷新链路）：

```bash
# 冷加载成功——日志出现（tag _app_config_etcd）：
#   loaded etcd config from key=gs-config-demo keys=1
# 热刷新，无需重启：
etcdctl put gs-config-demo "demo.message=hello-2"     # 绑定的 gs.Dync 字段翻新
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

etcd 治理规则源现已独立为 `go-spring.org/starter-governance-etcd` 模块（`govern.source.etcd.*`）。

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

### 3.2 治理属性面——已移出

etcd 治理规则源现已独立为 `go-spring.org/starter-governance-etcd` 模块（`govern.source.etcd.*`）。

### 3.3 本 starter 没有的 key

无 `enabled` 开关（激活即 import 条目的存在性）、无 TLS key、无 endpoints 列表、
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
| `unsupported config format` | format 参数/属性指向未注册格式 | 用已注册格式（properties/yaml/toml/json）或注册 reader。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置面 | import 串参数 5 个 |
| 其中必填 | import 2 项（host、key） |
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
