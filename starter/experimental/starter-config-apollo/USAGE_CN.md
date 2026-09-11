# starter-config-apollo 使用手册

深度使用参考。概览见 [README.md](README.md)。所有行为声明均对照源码核验：
starter 本体（`starter.go`、`starter_test.go`）、gs 核心（`spring/conf/provider/provider.go`、
`spring/gs/internal/gs_conf/conf.go`、`spring/gs/internal/gs_app/app.go`）以及带 `check.sh`
冒烟脚本的可运行 [example/](example/)。**Apollo 自身语义（namespace、cluster、meta server、
portal 操作、access key）见 [Apollo 官方文档](https://www.apolloconfig.com/#/design/apollo-introduction)** ——
本文只写 go-spring 的增量。

**激活方式**：blank import 即注册 `apollo` 配置 provider（`starter.go:79`）。在
`spring.config.import` 出现 `apollo:` 条目之前 starter 不做任何事；没有 `enabled` key、
没有属性前缀、无可注入 bean。本 starter 位于 `experimental/` 之下——这是未审核标记，
不是质量分级。

---

## 1. 完整工程示例

一个真实服务：从 Apollo 冷加载 namespace、portal 发布后热刷新 `gs.Dync` 字段、
并经 HTTP 暴露该值用于验证。与冒烟验证过的 [example/](example/)（mock Apollo 服务 +
冷加载断言）同构，扩展为真实 Apollo 与 HTTP 探针。文件树：

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/apolloconfig/agollo/v4 v4.4.0   // starter 传递引入
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-echo    latest        // 任意 server starter 均可
    go-spring.org/starter-config-apollo latest
)
```

**main.go**：

```go
package main

import (
    "net/http"

    "github.com/labstack/echo/v4"
    "go-spring.org/spring/gs"
    StarterEcho "go-spring.org/starter-echo"
    _ "go-spring.org/starter-config-apollo" // 注册 "apollo" provider
)

// Demo 持有来自 Apollo 的可热刷新属性。
// value tag 是顶层绝对 key：${demo.message:=none} 绑定拍平后的 key
// `demo.message`（不管它从哪个 source 导入），缺省回退 "none"。
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}

func init() {
    demo := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())

    gs.Provide(func() StarterEcho.RouterRegister {
        return func(e *echo.Echo) {
            // .Value() 始终返回最新刷新值。
            e.GET("/message", func(c echo.Context) error {
                return c.JSON(http.StatusOK, map[string]string{
                    "message": demo.Interface().(*Demo).Message.Value(),
                })
            })
        }
    })
}

func main() { gs.Run() }
```

**conf/app.properties** —— 上述代码用到的完整带注释配置面：

```properties
# 导入一个 Apollo namespace。语法见 §3：所有参数都写在 import 字符串里，
# 不存在属性前缀。
#   optional:                 namespace 为空/缺失时跳过而非致命
#   apollo:                   starter 注册的 provider 名
#   127.0.0.1:8080            Apollo meta/config server 地址（host:port）
#   /application              每个 import 条目恰好一个 namespace
#   ?appId=demo&cluster=default&secret=...&format=properties
#
# 多条目：逗号分隔；key 冲突时后加载的覆盖先加载的（与文件 import 同规则，
# conf.go:202）。
spring.config.import=optional:apollo:127.0.0.1:8080/application?appId=demo&format=properties

# 验证探针用 server（端口必须显式配置——不设默认值）。
spring.echo.server.addr=:8002
spring.http.server.enabled=false
```

**前置依赖**（一个外部系统）：import 字符串所指地址可达的 Apollo config service，
含 app `demo`、cluster `default`、已发布 namespace `application`。本地最快的起法是
Apollo 官方 [Quick Start docker-compose](https://www.apolloconfig.com/#/zh/deployment/quick-start-docker)；
CI/离线场景可用 starter example 内置的 mock Apollo（`example.go:73-92`），它恰好实现
agollo 冷加载需要的两个端点（`/services/config` 与 `/configfiles/json/{appId}/{cluster}/{namespace}`）
——免 docker。

**验证（冷加载）**：

```bash
go run . &
curl -s :8002/message            # {"message":"hello-from-apollo"}（或你发布的值）
grep 'loaded apollo namespace' <日志流>   # "loaded apollo namespace application keys=N"
```

**验证（热加载）**：在 Apollo portal 修改 `application` namespace 的 `demo.message`
并发布；在 agollo notification long-poll 周期内触发刷新，无需重启：

```bash
curl -s :8002/message            # 新值，进程未动
```

example 自带的冒烟门（与 `example/check.sh` 同构）：

```bash
cd example && ./check.sh && echo SMOKE-OK   # 断言输出含 "Apollo cold-load OK:"
```

---

## 2. 装配与时序

### 2.1 import 何时解析 —— pre-bean，以及为什么重要

`spring.config.import` 在 gs 加载应用属性阶段处理，即 `App.Start` 的第 2 步
（`gs_app/app.go:285-294`），**早于** IoC 容器装配：

```
blank-import starter-config-apollo
  └─ init(): conf.RegisterProvider("apollo", apolloCtrl.Load)   starter.go:77-80
gs.Run() → App.Start()
  ├─ 1. 挂载 gs.RefreshProperties / gs.AppStarted 门面目标
  ├─ 2. app.p.Refresh() —— 加载 app.properties，展开 spring.config.import     gs_conf/conf.go:216-240
  │       └─ conf.Load(source) → 前缀拆分 [optional:]<provider>:<path>        provider.go:84-92
  │             └─ apolloCtrl.Load(optional, path)                             starter.go:191
  │                   ├─ parseSource → url.Parse("apollo://"+path)            starter.go:113-148
  │                   ├─ clientFor → agollo.StartWithConfig（每元组一个）     starter.go:158-186
  │                   ├─ registerListener（先于 fetch——"a later change
  │                   │   is never missed"，starter.go:204）                   starter.go:226-241
  │                   └─ GetConfigContent → reader.Read(format) → flatten     starter.go:207-221
  ├─ 3. initLog
  ├─ 4. IoC 容器装配
  ├─ 5. Runners，6. Servers → 就绪
```

这个顺序是承重的：Apollo 的 key 在 **bean 绑定之前**就落入属性层，因此普通
`value:` 字段绑定 Apollo key 与绑定本地文件 key 完全同构——下游零特判。代价是：
required（非 `optional:`）import 失败（namespace 空、format 错、server 不可达）会在
第 2 步、任何 bean 尚不存在时中止启动——这正是 fail-fast 契约。这也意味着
`spring.config.import` 每次 Refresh 都会重新走一遍管线（见 2.2）。

import 嵌套：只处理一层——被导入 source 内部声明的 `spring.config.import` 会被静默
忽略（`gs_conf/conf.go:213-215`）。source 字符串在加载前支持 `${...}` 占位符解析
（`conf.go:224`），因此 `apollo:${APOLLO_ADDR:=127.0.0.1:8080}/application?appId=demo`
合法，是把地址挡在配置文件之外的受支持方式。重复的 import 条目在加载前去重
（`conf.go:223`）。

### 2.2 watch / 热刷新路径

agollo 自带配置变更通知 long-poll（`/notifications/v2`，见
[Apollo 配置设计](https://www.apolloconfig.com/#/zh/design/apollo-design)）。starter
把这些事件桥进 gs 的刷新链：

```
Apollo 发布 → agollo long-poll 触发 ChangeEvent / FullChangeEvent
  → apolloListener.OnChange / OnNewestChange                    starter.go:248-254
    → apolloCtrl.TriggerRefresh                                 starter.go:95-99
      → gs.RefreshProperties()                                  gs_app/app.go:149-151
        → App.RefreshProperties：先 guard "app not started yet"，随后
          重载全部 source（文件、env、cmd args、所有 import）、按层优先级
          合并、传播进容器                                      gs_app/app.go:247-256
            → 所有 gs.Dync[T] 字段原子更新
```

该路径两个值得记住的性质：

- **启动前是 no-op。** app 启动前到达的事件碰到的 `gs.RefreshProperties()` 会返回
  错误，被无害丢弃（由 `TestListenerChangeFiresRefresh` 钉死，`starter_test.go:133-139`）
  ——初始加载已捕获该状态（`starter.go:93-94`）。
- **整应用刷新，而非按 namespace。** 一个 key 变更会重载*所有* source，因此绑定
  本地文件的 Dync 字段在同一窗口内的文件改动也会被重读。只有 `gs.Dync[T]` 会重新
  绑定；普通 `value:` 字段仅启动期生效（gs 不存在 per-key callback）。监听器按
  source 去重注册（`TestListenerRegisteredOncePerSource`，`starter_test.go:118-128`），
  刷新引发的重复 Load 不会堆叠监听器。

### 2.3 一次冷加载逐层走读

`optional:apollo:127.0.0.1:8080/application?appId=demo`：

1. `conf.Load` 剥掉 `optional:` → `optional=true`；按第一个 `:` 拆分 → provider
   `apollo`、path `127.0.0.1:8080/application?appId=demo`（`provider.go:84-92`）。
2. `parseSource` 前缀 `apollo://` 后 `url.Parse`：host、path 里的 namespace、query
   参数；补默认 cluster=`default`、format 取 namespace 扩展名否则 `properties`
   （`starter.go:113-148`）。
3. `clientFor` 构造缓存 key `server|appId|cluster|secret|namespace`；未命中则
   `agollo.StartWithConfig` 创建 client，`IsBackupConfig: false`（不落本地缓存文件）、
   指向 `http://<server>`（`starter.go:150-186`）。
4. 先注册 change listener，再 `GetConfigContent` 取内容；内容为空且 optional →
   warn 跳过；为空且 required → 报错（`starter.go:207-214`）。
5. `reader.Read(format, content)` 解析、`flatten.Flatten` 把嵌套格式拍平成 dotted key，
   该 map 作为 app 层 source 并入分层存储（`starter.go:216-221`、
   `gs_conf/conf.go:233-237`）。

---

## 3. 逐 key 行为参考

### 3.1 import 字符串语法

形态（核心语法 `provider.go:60-72`，apollo 细则 `starter.go:111-148`）：

```
[optional:]apollo:<host>[:<port>]/<namespace>?appId=<id>[&cluster=<c>][&secret=<s>][&format=<f>]
```

`spring.config.import` 接受逗号分隔列表；按序解析，key 冲突时后条目覆盖前条目。

| 组成 | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|------|------|--------|-------------|----------|
| `optional:` | flag | 缺省 | namespace 为空*或从未同步*时打 warn（`optional apollo namespace %s is empty (skipped)`，`starter.go:210`）且不贡献任何 key。 | 不加时同样情况是启动错误——预期中的 fail-fast；若 namespace 会晚些创建则反直觉。 |
| `apollo` | 名字 | — | starter init 注册的 provider key。 | 拼错 → 启动报 `unsupported provider type ...`（`provider.go:96`）。 |
| host[:port] | string | — | Apollo meta/config server；以 `http://<host>` 传给 agollo。占位符（`${APOLLO_ADDR}`）先行解析。 | 缺失（source 以 `/` 开头）→ `missing apollo server address in ...`（`starter.go:118-120`）。地址错 → agollo 拉取失败（required 启动失败 / optional 静默跳过）。 |
| namespace | string | — | 每个 import 条目恰好一个 namespace。其文件扩展名决定默认 format。 | 缺失 → `missing namespace in ...`（`starter.go:122-124`）。访问非公开 namespace 未带 `secret` 得到空内容 → 后果取决于 `optional:`。 |
| `appId` | string | — | 必填，无默认。 | 缺失 → `missing appId in ...`（`starter.go:134-136`）。 |
| `cluster` | string | `default` | 参与client 缓存 key——同 appId+namespace 在两个 cluster 是两个 client、两个 import，不去重。 | 拼错会静默读到另一个（可能为空的）cluster；叠加 `optional:` 时*安静地*失败。 |
| `secret` | string | 空 | 受保护 namespace 的 access key。⚠ 落进 import 字符串，随之进入配置文件与加载层日志。 | 受保护 namespace 缺失/错误 → 空内容 → required 启动报错或 optional 跳过。 |
| `format` | string | namespace 扩展名，否则 `properties` | 显式解析器覆盖（`properties`/`yaml`/`yml`/`json`/`toml`，`reader.Read` 所支持者）。`TestParseSourceFormatOverride` 钉死。 | 与实际内容不符 → `parse apollo namespace %s as %s failed`（`starter.go:218`）；即使 `optional:` 也启动报错（parse 失败永不 optional）。 |

支持的格式以 `spring/conf/reader` 为准；未知值在 `reader.Read` 内报错。

### 3.2 属性 key

starter 模块自身绑定的属性 key 为**零**——没有 `apollo.*` 前缀、没有 `enabled`
开关、没有 client 池配置；模块树里唯一的 `value:` tag 是 example 的演示字段：

| key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `spring.config.import` | list | 空 | 声明于 `app.properties`/profile 文件；逗号分隔；只展开一层；支持占位符。整个 starter 由它驱动。 | 缺失 → starter 完全不生效。Apollo namespace 内嵌套声明的 import 被静默忽略。 |
| `demo.message`（example） | string | `none` | 仅 example：从 namespace 内容拍平出的顶层绝对 key `demo.message`，演示 Dync 绑定。 | — |

从 namespace 导入的 key 是**顶层绝对 key**——namespace 条目 `demo.message=hi` 直接
绑定 `${demo.message}`；没有实例前缀，且（依 no-relaxed-binding 拍板）匹配是精确的。

---

## 4. 验证与故障演练

所有演练假设 §1 工程指向真实 Apollo（演练 4.1/4.2 也可用 example 的 mock）。

### 4.1 冷加载

```bash
go run . &
curl -s :8002/message                          # 已发布的值
grep -E 'loaded apollo namespace' <log>        # keys=N —— N 为拍平后的 key 数
cd example && ./check.sh                       # CI 门："Apollo cold-load OK:" 标记
```

example 自包含：在 `127.0.0.1:18080` 起 mock Apollo、导入
`optional:apollo:127.0.0.1:18080/application?appId=demo&format=properties`，若
`demo.message` 未冷加载为 `hello-from-apollo` 则非零退出（`example.go:94-103`）。注意
mock 的 `/notifications/v2` 返回 304（`example.go:85-86`）——agollo 会持续 long-poll，
这正是演练 4.2 所依赖的机制。

### 4.2 watch 推送（热加载，免重启）

1. 应用运行中，`curl -s :8002/message` → 记下当前值。
2. Apollo portal 修改 `demo.message` 并发布。
3. notification long-poll 周期内再 curl → 新值；进程从未重启。刷新会重载*所有*
   source（§2.2），同一窗口内本地文件的改动也会被绑定该文件的 Dync 字段读到。
4. 反证：普通（非 Dync）`value:` 字段不动——仅启动期绑定。

### 4.3 畸形 namespace / format

```properties
spring.config.import=apollo:127.0.0.1:8080/app.json?appId=demo
```

在 `app.json` 发布非 JSON 内容 → 启动失败
`parse apollo namespace app.json as json failed`（`TestLoadParseErrorPropagates`，
`starter_test.go:109-114`）。注意 `optional:` 不能软化 parse 错误——只有空/缺失
namespace 是 optional 的（`starter.go:207-214`）。

### 4.4 optional 与 required 对比

- required、namespace 缺失/为空：启动中止，`apollo namespace %s is empty`。
- 同一 source 加 `optional:`：warn 一行 `optional apollo namespace ... is empty
  (skipped)`，启动继续，字段落在 `:=` 默认值（demo 中为 `none`）——由
  `TestLoadOptionalSkipsOnMissingNamespace` 钉死（`starter_test.go:94-105`）。
- 语法错误（缺 appId / host / namespace）无论 `optional:` 与否都失败
  （`TestParseSourceMissingAppID`）。

### 4.5 Apollo server 宕机

- **启动期、required import**：agollo client 创建/拉取失败 → 启动报错
  （`create apollo client for ... failed` 或空 namespace 错误）。这是 fail-fast
  契约；starter 没有重试循环。
- **运行期**：agollo 按自身节奏重试（其日志，非 gs 的）；gs 侧症状就是*配置滞留*
  ——Dync 值不再移动。没有健康指示器、没有指标、连接丢失不产生 gs 日志（见 §6）。
  server 恢复后下一次成功 poll 触发 listener，值追平。
- `IsBackupConfig: false`（`starter.go:178`）意味着没有本地缓存文件桥接宕机期重启：
  宕机中重启且 import 为 required 会失败。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报 `unsupported provider type apollo` | 未 blank-import starter | 加 `_ "go-spring.org/starter-config-apollo"`。 |
| `missing appId in ...` / `missing namespace in ...` / `missing apollo server address` | import 字符串语法错误 | 修 source 字符串；语法见 §3.1。 |
| 启动失败 `apollo namespace X is empty` | namespace 不存在、未发布、或受保护但缺 `secret` | 发布 namespace / 加 `secret`；若预期晚创建则标 `optional:`。 |
| 启动失败 `parse apollo namespace X as Y failed` | `format`（或 namespace 扩展名）与内容不符 | 修 `format=` 或 namespace 名；`optional:` 不覆盖 parse 错误。 |
| 字段停在 `:=` 默认值，日志有 `optional ... empty (skipped)` | appId/cluster 拼错指向空 namespace 且带 `optional:` | 核对 appId/cluster；`optional:` 下拼错就是安静失败。 |
| 热刷新不达 | (a) 字段不是 `gs.Dync[T]`；(b) agollo long-poll 未达 server；(c) 事件发生在装配前 | 只有 Dync 会刷新；查 agollo 自身日志的 poll 错误；装配前事件被有意丢弃（§2.2）。 |
| 本地文件的值被覆盖 | import 层优先级：后导入覆盖先导入；app 文件与 import 分层见 `gs_conf/conf.go` | 调整 import 顺序 / 改名冲突 key。 |
| Apollo 宕机中重启失败（required import） | 无本地备份（`IsBackupConfig: false`） | 恢复 Apollo，或改 `optional:` + 本地默认值做降级启动。 |
| 第二个 namespace 的值缺失 | 一个 import 条目一个 namespace | 再加一条逗号分隔的 import 条目。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key（source 参数 + 属性） | 5 个 source 参数 + 1 个属性（`spring.config.import`） |
| 其中必填 | 3 个 source 参数（host、namespace、appId） |
| quickstart 前置外部依赖 | 1（Apollo；example 的 mock 可去掉） |
| "注意/坑"条数 | 5（secret 落文件、optional 安静跳过、parse 永不 optional、整应用刷新、无备份配置） |

设计嫌疑清单（沿用上一版并扩充）：

- secret 写在 import 字符串里落进配置文件——不像 vault（token-file、env）没有带外
  凭证通道；只有占位符间接（`${APOLLO_SECRET}`）可用 → 候选补 env 兜底。
- 无 governance `Source` 接入（nacos/etcd 已有）→ 若治理规则要放 Apollo 则是功能缺口。
- agollo 连接无健康指示器 / 指标 / 专属日志 tag：运行期 server 挂掉对 gs 可观测性
  不可见（只有配置滞留）。
- 每个 (server, appId, cluster, secret, namespace) 元组一个 agollo client
  （`starter.go:150-155`）——namespace 多则 long-poll 连接多；agollo 本身支持多
  namespace client → 候选合并。
- `optional:` 语义把"尚未同步"与"合法为空"混为一谈——发布为*空*的 namespace 与
  appId/cluster 拼错无法区分。
- 每次 namespace 变更刷新整个属性集（任意 key、任意 source）——粗粒度但简单；
  低变更频率下无碍，值得观察。
