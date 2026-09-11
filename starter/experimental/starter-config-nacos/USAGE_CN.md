# starter-config-nacos 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`governance_nacos.go`、`starter_test.go`、`governance_nacos_test.go`）、
核心 import 文法（`spring/conf/provider/provider.go:74-104`）、refresh 链路
（`spring/gs/internal/gs_app/app.go`）以及 docker 门控冒烟验证过的
[example/](example/)。**Nacos 自身语义（dataId / group / namespace、服务端部署、控制台
使用）见 [Nacos 官方文档](https://nacos.io/en/docs/v2/guide/user/config-history/)** —— 本文只写
go-spring 的增量。

> **experimental/ 标记**：位于 `experimental/` 表示"尚未经过设计评审"，不是质量分级；
> 接口形态可能变化。

**激活方式**（两条独立路径、两个独立 bean）：

1. **配置提供方**——只要 `spring.config.import` 出现 `nacos:` 条目即生效（blank import 在
   `init()` 里注册 provider：`starter.go:58`）。没有 `enabled` 开关。
2. **治理规则源**——存在任一 `govern.source.nacos.*` key 即生效
   （`gs.OnProperty("govern.source.nacos")` 是前缀匹配，`governance_nacos.go:64`）。

两个 bean 刻意分离：config-client 管道（client 缓存、source 解析）共享，但配置提供方推送的是
**应用属性刷新**，治理源推送的**只是治理规则**——规则发布绝不触碰应用属性，应用配置发布也绝不
重新解析规则。

---

## 1. 完整工程示例

一个配置托管在 Nacos 的服务：一个可热更新字段 + 从专用 dataId 推送的治理规则。目录结构
（与冒烟验证过的 [example/](example/) 同构）：

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── docker-compose.yml          # 本地开发用的 nacos
```

**go.mod**（关键依赖）：

```
require (
    github.com/nacos-group/nacos-sdk-go/v2 v2.3.2   // 传递引入
    go-spring.org/spring                v1.3.x
    go-spring.org/starter-config-nacos  latest
    go-spring.org/starter-governance    latest      // 可选：消费规则源
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-config-nacos"
    _ "go-spring.org/starter-governance" // 可选：governance center
)

// Demo 绑定来自 Nacos dataId 的动态字段。
// 只有 gs.Dync[T] 会热更新；普通字段保持启动时的值。
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}

func init() {
    // Export 成 gs.Rooter：即使没有其它 bean 注入它，容器也会实例化。
    gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())
}

func main() { gs.Run() }
```

**conf/app.properties**——实际用到的完整配置面：

```properties
# 从 Nacos 配置服务端导入配置。"optional:" 允许 dataId 尚不存在时也能启动；
# 一旦发布，值会被填充，且经 provider 的 change listener 实时刷新。
spring.config.import=optional:nacos:127.0.0.1:8848/gs-config-demo?group=DEFAULT_GROUP&format=properties

# 可选的第二条激活路径：从专用 dataId 读取治理规则。
# 内容是治理规则文档（govern.enabled/fault/resilience...），
# 与 starter-governance 文件源的规则文件字节兼容。
govern.source.nacos.server=127.0.0.1:8848
govern.source.nacos.data-id=demo-govern.yaml
govern.source.nacos.group=DEFAULT_GROUP
```

**docker-compose.yml**（照抄 example 的——注意 gRPC 端口 `9848`；v2 SDK 在主端口 +1000 上
走 gRPC，只暴露 8848 会连不上）：

```yaml
services:
  nacos:
    image: nacos/nacos-server:v2.4.3
    environment:
      MODE: "standalone"
    ports:
      - "127.0.0.1:8848:8848"
      - "127.0.0.1:9848:9848"
```

**验证**（冷加载 → 不重启热更新）：

```bash
docker compose up -d
# 等待就绪（Nacos 启动慢，约 1-3 分钟）
curl -fsS http://127.0.0.1:8848/nacos/v1/console/health/readiness

# 通过 HTTP open API 初始化 dataId
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
     -d 'dataId=gs-config-demo&group=DEFAULT_GROUP&content=demo.message=hello-v1'
# 必须输出：true

go run .          # 应用启动；日志出现 "loaded nacos config from DEFAULT_GROUP/gs-config-demo keys=1"

# 热更新：发布新版本；应用的 gs.Dync 字段不重启即更新
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
     -d 'dataId=gs-config-demo&group=DEFAULT_GROUP&content=demo.message=hello-v2'
# 观察应用日志/字段；自动化形式见 ./example/check.sh
```

[example](example/example.go) 把这个闭环自动化（发布 + 最多 15 秒轮询 + 断言），
`example/check.sh` 是 docker 门控的冒烟门禁。

---

## 2. 装配与时序

### 2.1 import 何时解析——早于 bean，以及为什么

```
blank-import starter-config-nacos
  └─ init(): conf.RegisterProvider("nacos", nacosController.Load)      starter.go:58

gs.Run()
  ├─ App.Start(): app.p.Refresh()                                     app.go "Start"
  │    └─ 加载 conf/app.properties → 读到 spring.config.import
  │         └─ conf.Load("optional:nacos:...")                        provider.go:74
  │              ├─ 剥 "optional:" 前缀 → optional=true                provider.go:85
  │              ├─ 剥 "nacos:" 前缀 → 查 provider 表                  provider.go:89
  │              └─ nacosCtrl.Load(optional, source)                  starter.go:202
  │                   ├─ parseSource → configSource                   starter.go:103
  │                   ├─ clientFor（按 server|ns|user|pass 缓存）      starter.go:153
  │                   ├─ registerListener（去重）                      starter.go:249
  │                   ├─ GetConfig → reader.Read(format) → flatten    starter.go:219-244
  │                   └─ key 合入分层属性存储
  ├─ IoC 容器装配
  ├─ Runners → Servers → 就绪
```

import 在**任何 bean 装配之前**解析：所有 bean 的 `value:"${...}"` 标签绑定的是**合并后**
的属性存储——远程 key 必须先就位，绑定才能看到。推论：provider 本身不能是注入 bean，
它是 `init()` 里注册的包级 controller；变更时经进程级门面 `gs.RefreshProperties()`
触达刷新，完全不需要 bean 装配。app 启动前门面返回错误，提前 `TriggerRefresh`
是安全 no-op。

### 2.2 监听 / 热更新链路（配置面）

```
Nacos 推送 dataId 变更
  └─ SDK OnChange（registerListener 安装，按 client+group+dataId 去重）
       └─ nacosCtrl.TriggerRefresh                                     starter.go:83
            └─ gs.RefreshProperties()（app 启动前返回错误）
                 └─ App.RefreshProperties()                            app.go:247
                      ├─ 重新加载全部来源：文件、env、命令行参数，并重放每个
                      │   spring.config.import 条目（→ 再次 nacosCtrl.Load——
                      │   这正是 listener 注册必须去重的原因；
                      │   TestListenerRegisteredOncePerSource 钉死）
                      ├─ 按优先级合并、原子校验（不做部分更新）
                      └─ 传播：只有 gs.Dync[T] 字段热更新
```

**没有按 key 的变更回调**：一次推送会重新拉取整个 dataId（以及所有其它来源），每个
`gs.Dync[T]` 重新解析自己的 `${...}` 表达式。普通 `value` 字段的值永远是启动时的；
`OnProperty` 条件只在启动时评估。

### 2.3 治理规则推送链路（独立面）

```
存在任一 govern.source.nacos.* key
  └─ gs.Module(OnProperty("govern.source.nacos"))                     governance_nacos.go:64
       ├─ conf.Bind("${govern.source.nacos:=}") → governNacosConfig（expr 校验）
       └─ Provide NacosSource 构造
            ├─ NEW：GetConfig + rules.Parse → 种子快照（坏文档直接启动失败）:127-137
            ├─ Init：ListenConfig(OnChange → apply)                    :140-148
            ├─ Destroy：CancelListenConfig                             :152-154
            └─ Export(gs.As[governance.Source]())——由 governance Center 经
                 Source 契约（Snapshot + Subscribe，cloud/governance）消费

在规则 dataId 上发布
  └─ NacosSource.apply(data)                                           governance_nacos.go:173
       ├─ 字节相同的重复投递（SDK 重连可能重推）→ no-op
       ├─ rules.Parse 失败 → Error 日志、保留最后一份好快照、不推送
       ├─ 解析后规则等价 → 换快照但不推送
       └─ 确有变化 → 换快照 + 调 Center 的 Subscribe 回调
```

这条链路绝不调用 `RefreshProperties`：规则推送只刷新治理执行器
（超时、重试、故障注入……）。

---

## 3. 逐 key 行为参考

### 3.1 import 字符串文法（配置面）

核心文法（provider.go:74-104）：`[optional:]<provider>:<path>`——先剥 `optional:` 前缀，
再取 provider 名，第一个 `:` 之后的全部是 path。`nacos` 的 path 形如
`<host>:<port>/<dataId>?<query>`（按 `nacos://` URL 解析，starter.go:103-145）。每个 import
条目一个 dataId；多条 import 在 `spring.config.import` 里并列。

| 部分 / 参数 | 类型 | 默认值 | 行为与联动 | 配错后果 |
|---|---|---|---|---|
| `optional:` | 前缀 | 无（即 required） | 位于整个 source 串最前、`nacos:` 之前。拉取失败**或内容为空**时跳过该来源（warn 日志），放行启动（starter.go:219-235）。 | 不写时 dataId 缺失或服务端宕机会中止启动——生产通常应该如此，bootstrap 配置则相反。 |
| `host:port` | string | — | **必填**。必须是 `host:port`；仅单台 server（不支持地址列表）。端口非数字在解析期被拒（starter.go:187-197）。 | `noport` / `1.2.3.4:x` → 启动报错 `nacos server address must be host:port`。⚠ 集群用户需前置 VIP。 |
| `dataId` | string | — | **必填**。第一个 `/` 之后的路径段；同时驱动格式推断。 | 缺失（`...8848` 无 path）→ 启动报错 `missing data id`。 |
| `group` | string | `DEFAULT_GROUP` | query 参数（starter.go:126-128）。Nacos group 语义见官方文档。 | group 错 → "config not found"；配了 `optional:` 则退化为静默跳过——用启动日志 `loaded nacos config` 核对。 |
| `namespace` | string | 空（public） | 填 namespace **id** 而非名称。参与 client 缓存 key，不同 namespace 得到不同 client。 | 填了名称 → 空配置（同 group 错的失败形态）。 |
| `username` / `password` | string | 空 | 服务端鉴权；同样参与 client 缓存 key。 | 开鉴权的服务端上缺失 → 启动时 GetConfig 报错（optional 则跳过）。 |
| `format` | string | dataId 扩展名，否则 `properties` | reader 支持的格式：`properties` / `yaml` / `toml` / `json`。无扩展名的 dataId 默认 properties（starter.go:129-135）。 | 内容/格式不符 → 启动报错 `parse nacos config ... as <fmt> failed`（`TestLoadParseErrorPropagates` 钉死）。 |
| `timeout-ms` | uint64 | `5000` | 每个 source 的 SDK 请求超时（starter.go:136-143）。非数字在解析期被拒。 | 过小 → 慢 Nacos 下启动抖动。 |

⚠ **没有** `endpoint` 参数，也没有集群/地址列表形态——server 永远是单个 `host:port`。

### 3.2 属性 key（治理面）

经 `conf.Bind` 以前缀 `${govern.source.nacos}` 绑定（这些是前缀下的实例 key——本 starter
仅有的 value tag）：

| Key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|---|---|---|---|---|
| `govern.source.nacos.server` | string | — | **必填**（`expr:"$ != ''"`）。与 import 串相同的 `host:port` 形态。 | 为空 → 启动时模块绑定报错。 |
| `govern.source.nacos.data-id` | string | — | **必填**（`expr:"$ != ''"`）。专用规则 dataId——**不要**复用应用配置 dataId。 | 为空 → 绑定报错；id 错且文档缺失/损坏 → 构造直接失败（不同于 optional 的配置导入）。 |
| `govern.source.nacos.group` | string | `DEFAULT_GROUP` | 规则分组，独立于配置导入的 group。 | 找不到 → 启动报 `get ... failed`。 |
| `govern.source.nacos.namespace` / `.username` / `.password` | string | 空 | 鉴权/隔离，语义同 import 串参数。 | 失败形态同上。 |
| `govern.source.nacos.format` | string | dataId 扩展名，否则 `properties` | 覆盖文档格式探测（governance_nacos.go:80-86）。 | 不符 → rules.Parse 失败；种子期启动失败，后续坏发布保留最后好快照。 |

⚠ 治理面**没有 timeout key**——SDK 超时硬编码 5000 ms（governance_nacos.go:78），与
import 串的 `timeout-ms` 不对称。已记入第 6 节。

value tag 全量核对（与源码一致）：
`value:"${server}"`、`${data-id}`、`${group:=DEFAULT_GROUP}`、`${namespace:=}`、
`${username:=}`、`${password:=}`、`${format:=}`（治理面），以及 example 的
`${demo.message:=none}`（应用字段，非 starter 面）。

---

## 4. 验证与故障演练

所有演练使用第 1 节的 open API 发布命令，观察日志 tag `_app_config_nacos`
（经 `logger.config_nacos.*` 调级）。

### 4.1 冷加载

```bash
# 启动前：dataId 已存在且 demo.message=hello-v1
go run . 2>&1 | grep config_nacos
# → "loaded nacos config from DEFAULT_GROUP/gs-config-demo keys=1"（Info）
```

### 4.2 监听推送（热更新）

```bash
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
     -d 'dataId=gs-config-demo&group=DEFAULT_GROUP&content=demo.message=hello-DRILL'
# 数秒内 gs.Dync[string] 字段观察到 "hello-DRILL"，无需重启
```

只有 `gs.Dync[T]` 字段会变。自动化形式：`cd example && ./check.sh`（docker 门控、
自断言、60 秒看门狗）。

### 4.3 治理规则推送

```bash
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
     -d 'dataId=demo-govern.yaml&group=DEFAULT_GROUP&content=govern:
enabled: true
default:
  enabled: true
  attempt-timeout: 300ms'
```

governance center 经 Subscribe 收到新 Config；熔断/重试/故障注入行为即时变化。发布一份
**损坏**文档（`govern: {`）→ 一条 Error 日志
`published an invalid document (keeping last good config)`，原规则继续生效
（`TestNacosSource_PushChain` 钉死）。字节相同的重推（SDK 重连）是 no-op。

### 4.4 畸形 dataId / source 串

| 演练 | 结果 |
|---|---|
| `nacos:onlydata`（无 `host:port`） | 启动报错 `missing nacos server address` |
| `nacos:127.0.0.1:8848`（无 dataId） | 启动报错 `missing data id` |
| `...?timeout-ms=abc` | 启动报错 `invalid timeout-ms`（解析期拒绝） |
| required 导入 + dataId 不存在 | 启动报错 `get nacos config ... failed` / `is empty` |
| `optional:` + 上述任一拉取失败 | warn 日志，应用照常启动但缺这些 key |

### 4.5 Nacos 服务端宕机

- **启动时 + required 导入** → 启动中止（GetConfig 报错）。
- **启动时 + optional 导入** → warn `optional config get ... failed (skipped)`；应用带默认值
  运行；服务端恢复后下一次发布触发 OnChange，经一次完整刷新补齐 key。
- **启动后** → SDK 持续重连；不会触发刷新；`gs.Dync` 值保持最后好值（配置过期风险，
  见第 5 节）。listener 状态没有健康指示器或指标：过期只能从 `config_nacos` 日志沉寂
  侧面发现。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|---|---|---|
| 启动报 `unsupported provider type nacos` | 未 blank-import starter（provider 在 `init()` 注册） | 加 `_ "go-spring.org/starter-config-nacos"`。 |
| 启动报 `missing nacos server address` / `missing data id` | source 串缺 `host:port` 或缺 `/dataId` 路径 | 修 `spring.config.import`；文法是 `[optional:]nacos:host:port/dataId?...`。 |
| 启动报 `create nacos config client ... failed`，端口通但 SDK 报错 | gRPC 端口（主端口+1000，即 9848）未暴露 | 同时暴露 8848 **和** 9848（v2 SDK 走 gRPC）。 |
| 启动报 `get nacos config ... failed`，服务端开了鉴权 | 缺 `username`/`password` query 参数 | 补上；它们也参与 client 缓存 key。 |
| 应用启动但缺 key，warn `optional config ... (skipped)` | `optional:` 下 group/namespace 写错 | 修 group / 用 namespace **id**；到 Nacos 控制台核对。 |
| 配置只加载一次，之后不刷新 | listener 注册失败（有日志），或服务端重启后只暴露了 8848（gRPC 重监听失败） | 检查 9848 暴露与 SDK 日志；字段必须是 `gs.Dync[T]`——普通 `value` 字段永不热更。 |
| `parse nacos config ... as yaml failed` | `format`（显式或按扩展名推断）与内容不符 | 发布匹配内容或显式设 `format=`；注意无扩展名 dataId 默认 properties。 |
| 治理：启动即挂在 `rules.Parse` | 初始规则文档损坏——种子快照按设计快速失败 | 修文档；之后的坏发布只记日志并保留最后好配置。 |
| 网络抖动后值不动了 | SDK 重连后重投字节相同配置 → 有意的 no-op | 发布真正变更的文档；用控制台历史核对。 |

---

## 6. 设计体检表与嫌疑清单

| 指标 | 数值 |
|---|---|
| import 串参数 | 7（另 2 个必填部分） |
| 属性 key（治理面） | 7（必填 2） |
| quickstart 前置外部依赖 | 1（Nacos 服务端） |
| "注意/坑" 条数 | 6（gRPC 端口、namespace-id、仅 Dync、无 endpoint/集群、optional 吞掉错 group、治理面超时不对称） |

嫌疑清单（供设计裁决）：

- **两套并行的 Nacos 配置面**：provider source 串（query 参数）与 `govern.source.nacos.*`
  （属性 key）共享 `configSource` 解析但拼写完全不同，且治理路径硬编码 `timeoutMs: 5000`
  无 key 可调 → 统一候选。
- **仅单 server 地址形态**：无集群列表 / `endpoint` 参数；每个 client 由单个 `host:port`
  构建（starter.go:170）。集群部署需要外部 VIP。
- **刷新粒度**：一个 dataId 变更触发全部来源与 import 的整体重载——正确但偏重；
  listener 去重是保障其安全的关键。
- **无 listener 存活信号**：无健康指示器/指标；listener 静默死亡只能表现为配置过期。
- **example 配置里的死 key**：`example/conf/app.properties` 中曾出现的
  `spring.nacos.ip-addr` / `spring.nacos.port` 在本 starter 内无任何绑定（兄弟 starter 约定
  的残留）→ 已删除。
- **`optional:` 把错 group 当跳过**：optional 导入里 group 拼错与"尚未发布"在启动时无法区分。
