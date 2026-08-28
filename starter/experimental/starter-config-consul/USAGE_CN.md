# starter-config-consul 使用说明 — 参考手册

详细使用参考。总览见 [README.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`starter_test.go`）、核心 import 机制（`spring/conf/provider/provider.go`、
`spring/gs/internal/gs_conf/conf.go`、`spring/gs/internal/gs_app/app.go`）以及冒烟验证过的
[example/](example/) 核对。**Consul 自身语义（KV、blocking query、ACL token、数据中心）见
[Consul 官方文档](https://developer.hashicorp.com/consul/docs/dynamic-app-config/kv)** —— 下文只写
go-spring 的增量。

**激活方式**：blank import 即注册 `consul` 配置 provider 与变更→刷新桥接（`starter.go:44-56`）。
仅当 `spring.config.import` 中出现 `consul:` 条目时才真正生效；import 字符串就是**全部**配置面
—— starter 自身不绑定任何 `value:` tag。

> 本 starter 位于 `experimental/` 目录 —— 这是**未审核**标记，不是质量分级。

---

## 1. 完整工程示例

一个 `demo.message` 存于 Consul KV、变更即热刷新的最小服务。文件树（与冒烟验证过的
[example/](example/) 同构）：

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── docker-compose.yml
```

**go.mod**（关键依赖）：

```
module demo

require (
    github.com/hashicorp/consul/api v1.34.1   // 仅当像下文一样用 Go 发布时需要
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-config-consul latest
)
```

**conf/app.properties** —— 完整带注释的配置面：

```properties
# 从 Consul KV 导入配置。
# 语法：[optional:]consul:<host>:<port>/<kv-path>?<query>
# "optional:" 让应用在 key 尚不存在时也能启动；发布后即被读取，
# 并经 blocking-query watcher 活着刷新（optional 跳过逻辑见 starter.go:196-220，
# watch 见 §2.2）。
spring.config.import=optional:consul:127.0.0.1:8500/gs-config-demo?format=properties
```

**docker-compose.yml**（与 example/ 相同）：

```yaml
services:
  consul:
    image: hashicorp/consul:1.18
    command: agent -dev -client=0.0.0.0
    ports:
      - "127.0.0.1:8500:8500"
```

**main.go**：

```go
package main

import (
    "fmt"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-config-consul"
)

// Demo 绑定来自导入 Consul KV 路径的动态字段。
// 注册为 root object，容器会急切创建它。
// 只有 gs.Dync[T] 字段会热刷新——普通 string 在启动时就冻结了。
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}

func main() {
    demo := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())
    gs.Run()
    fmt.Println("final:", demo.Interface().(*Demo).Message.Value())
}
```

**验证**（与 example 的 check.sh 同构）：

```bash
docker compose up -d
# 等就绪 —— leader 端点才是真实就绪信号：
curl -fsS http://127.0.0.1:8500/v1/status/leader

# 发布 import 指向的配置：
consul kv put gs-config-demo "demo.message=hello"   # 或： curl -X PUT \
    --data-binary 'demo.message=hello' \
    http://127.0.0.1:8500/v1/kv/gs-config-demo

go run .        # 日志： loaded consul config from kvPath=gs-config-demo keys=1

# 不重启热刷新（见 §4.2）：
consul kv put gs-config-demo "demo.message=hello-2"
```

运行 `bash example/check.sh` 执行完整的 docker 门控冒烟（dev Consul + example 在 15 秒内
自断言热刷新成功、收到 SIGTERM 后以 0 退出）。

---

## 2. 装配与时序

### 2.1 生命周期时间线

```
blank import starter-config-consul
  └─ init(): gs.Provide(consulCtrl).Export(gs.As[gs.Rooter]())     starter.go:50
             conf.RegisterProvider("consul", consulCtrl.Load)      starter.go:55

gs.Run()
  ├─ 配置阶段（无 bean 参与，因此是 pre-bean）：
  │    AppConfig.Refresh()                                  gs_conf/conf.go:84
  │      └─ loadFiles → app.properties
  │            └─ loadFileImports: ${spring.config.import}   gs_conf/conf.go:216
  │                  └─ conf.Load("optional:consul:...")    provider.go:74
  │                        ├─ optional/provider 切分（首个 ':'）
  │                        ├─ consulCtrl.Load → parseSource → KV Get（冷加载）
  │                        └─ registerWatch → 每个 (client, kvPath) 一个 goroutine
  ├─ IoC 装配：consulCtrl 是 Rooter，autowire 注入的 *gs.PropertiesRefresher
  │    填充 c.Refresher（此前为 nil —— TriggerRefresh 是 no-op，starter.go:86-90；
  │    由 TestTriggerRefreshNilRefresherIsNoop 覆盖）
  ├─ Runners/Servers、就绪
  └─ 稳态：watchLoop 的 blocking query 在 index 抬升时触发 RefreshProperties
```

**为什么是 pre-bean**：import 在 `AppConfig.Refresh()` 内解析，而它先于 IoC 容器装配执行
（`app.go:272-279` 启动序列）。因此 provider 必须从 `init()` 注册 —— 包级状态而非 bean ——
刷新桥接也必须容忍未装配状态（`TriggerRefresh` 的 nil 检查，`starter.go:86-90`）。

### 2.2 watch / refresh 路径走读

1. `registerWatch`（`starter.go:234-249`）按 `clientKey + "|" + kvPath` 去重；同一 source 的
   重复 Load —— 每次属性刷新都会发生 —— 恰好只产生一个 goroutine
   （`TestWatchRegisteredOncePerSource`）。
2. `watchLoop`（`starter.go:252-283`）发 Consul **blocking query**：`Get` 带
   `WaitIndex: lastIndex`、`WaitTime: 5m`。它吞掉初始 index（首次轮询只建立基线），
   `LastIndex` 前进时触发 `TriggerRefresh()`，index 回退时重置为 0（Consul 重启/index 重置），
   传输错误 2 秒后重试。
3. `TriggerRefresh` → `PropertiesRefresher.RefreshProperties()`（`app.go:149-151`）→ 完整的
   `AppConfig.Refresh()` 从零重建分层存储（文件、env、cmd、import —— KV 条目因此被**重新
   拉取**，`starter.go:196`）→ 容器把新快照原子传播到每个 `gs.Dync[T]` 字段
   （`app.go:234-256`）。非 `Dync` 绑定不会重跑。
4. 刷新失败（例如非 optional 的 key 被删除）会保留旧快照并记日志：watcher 对删除打一条
   点名 key 的 WARN、对每次刷新失败各打一条 WARN，loop 继续监听。

import 机制要点（`gs_conf/conf.go`）：被导入文件里声明的 import 会被**静默忽略**（只处理一层，
`conf.go:213-215`）；import 字符串会经过占位符解析（`conf.go:224`），`${...}` 表达式可用；
导入源落入 `StorageAppFile` 层，即与 `app.properties` 同优先级，后导入覆盖先导入。

---

## 3. 逐 key 行为参考

starter **没有属性 key** —— 模块树里仅有的 `value:` tag 属于 example 的 demo bean
（`${demo.message:=none}`，是对 KV 条目*内部* key 的顶层绝对引用，不是 starter 的 key）。
全部配置面就是 import 字符串：

语法（核心切分在 `provider.go:84-92`，consul 部分在 `starter.go:104-137`）：

```
[optional:]consul:<host>:<port>/<kv-path>?format=..&scheme=..&token=..&datacenter=..
```

| 组成 | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|------|------|--------|------------|----------|
| `optional:` 前缀 | 开关 | 无 | 核心层语义（`provider.go:85-88`）：**冷加载**时 Get 失败 / key 缺失 / 值为空均跳过并打 Warn（`starter.go:198-220`）。⚠ **不**覆盖解析错误、client 创建错误，也不覆盖 watch 期刷新路径。 | 不加则 key 缺失或 agent 不可达直接中止启动（`consul kv <path> not found`）。 |
| `host:port` | string | — | Consul agent 地址（HTTP API）。**必填** —— host 为空解析失败（`starter.go:109-111`）。 | 启动报错 `missing consul server address in %q`。 |
| `<kv-path>` | string | — | 每个 import 恰好**一个** KV 条目（`cli.Get`，非前缀列表）。前导 `/` 被去除。必填（`starter.go:112-115`）。 | 启动报错 `missing kv path in %q`。无法读 key 目录。 |
| `format` | string | kv-path 扩展名，否则 `properties` | 内容解析器选择（`reader.Read`，`starter.go:222`）：`properties`/`yaml`/`toml`/`json`。按路径扩展名推断 —— 名为 `app` 的 key 里放 YAML，不加 `?format=yaml` 会按 properties 解析。 | 加载期解析错误：`parse consul kv %s as %s failed` —— 即便 `optional:` 也失败（解析错误不在 optional 豁免内，`starter.go:222-226`）。 |
| `scheme` | string | `http` | 传入 `api.Config.Scheme`（`starter.go:161-166`）。参与 client 缓存 key。 | 对纯 HTTP 端口用 `https` → 每次 Get 报错；非 optional 启动失败，optional 则静默跳过。 |
| `token` | string | 空（匿名） | Consul ACL token，随每个请求发送。⚠ 内联在 import 字符串里会落进配置文件/日志 —— 没有 env/token-file 兜底。 | token 错误/权限不足 → Get 403；表现为 `get consul kv %s failed`。 |
| `datacenter` | string | agent 默认 | 冷加载与 watch 的 `QueryOptions.Datacenter` 覆盖（`starter.go:196, 256`）。参与 client 缓存 key。 | 未知 dc → 走上述错误路径。 |

client 按 `(address, scheme, token, datacenter)` 元组缓存（`clientKey`，`starter.go:142-144`）；
watch 按 client+kvPath 去重。两个 import 共享元组则共享一个 client、各有一个 watch goroutine。

优先级说明：导入源位于 `StorageAppFile` 层（与 `app.properties` 平级），低于 profile 文件、env
与命令行（`gs_conf/conf.go:27-41`）—— 对非 `Dync` 绑定与合并属性集而言，env 变量覆盖 Consul
KV 值。

---

## 4. 验证与故障演练

### 4.1 冷加载

```bash
consul kv put gs-config-demo "demo.message=hello"
go run .    # 预期日志： loaded consul config from kvPath=gs-config-demo keys=1（tag "def"）
```

开 Debug 级可看解析明细：starter 以 Debug 记录 `loading config from address=... kvPath=...
format=...`（`starter.go:186`）。缺失/失败同样以 `def` app tag 记 Error/Warn ——
`grep 'consul' app.log`。

### 4.2 watch 推送（热刷新，不重启）

```bash
# 应用运行中（§4.1）：
consul kv put gs-config-demo "demo.message=hello-2"
# 数秒内 blocking query 返回；gs.Dync[string] Message 翻转为 hello-2
```

example 自动化的正是这一步：发布 `hello-<timestamp>`，轮询 `Dync` 字段最长 15 秒，超时非零
退出（`example/example.go` 的 `runTest`）。本地 agent 下 blocking query 延迟亚秒级；错过通知的
最坏情形是一个 `WaitTime` 周期（5 分钟）。

### 4.3 畸形值

```bash
consul kv put app-json '{not-json'    # import 为 .../app-json?format=json
go run .                              # 启动失败： parse consul kv app-json as json failed
```

注意 `optional:` 救不了这个 —— 只有 拉取失败/缺失/为空 走 optional 跳过
（`starter.go:196-220` 对比 `222-226`；`TestLoadParseErrorPropagates`）。

### 4.4 optional vs 必填

```bash
consul kv delete gs-config-demo
go run .   # optional: 正常启动，日志 "optional config kv gs-config-demo not found (skipped)"
           # 非 optional: 以 "consul kv gs-config-demo not found" 中止
```

agent 不可达时同样可对比：停掉 docker（`compose stop`），比较 `optional:consul:...`（Warn 后
跳过）与 `consul:...`（失败于 `get consul kv ... failed`）。

### 4.5 稳态时 Consul 宕机

应用运行中且 KV 已发布时执行 `docker compose stop`：watcher 的 blocking query 报错，每 2 秒
重试（`starter.go:261-263`），应用继续用最后快照服务 —— 配置变陈旧但不崩溃，除 SDK 传输错误
外无每轮重试日志。重启 Consul 后下一次 KV 变更自动恢复热刷新（index 回退会重置基线，
`starter.go:269-271`）。

### 4.6 稳态时 key 被删除（非 optional）

运行中删除 key：watch 触发刷新，但刷新内的 re-Load 失败（key 不存在）→ `RefreshProperties`
报错 → watcher 打一条点名 key 的 WARN 和一条刷新失败 WARN，**旧快照保留**（快照只在成功时交换，
`conf.go:130`）。应用继续用最后已知值；没有任何信号标记配置已陈旧。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 `consul kv <path> not found` | key 缺失且未加 `optional:` | `consul kv put` 补 key，或加 `optional:` 前缀（先想清楚“缺配置也能跑”是否成立）。 |
| 启动失败 `get consul kv <path> failed` | agent 不可达、scheme 错、ACL token 错、datacenter 未知 | `curl http://<host>:8500/v1/status/leader`；检查 `scheme=`/`token=`/`datacenter=` 参数。 |
| 启动失败 `parse consul kv %s as %s failed` | 内容与 format 不符（常见于无扩展名 key 被推断为 `properties`） | 加 `?format=yaml|json|toml`，或让 key 名带正确扩展名。 |
| 应用起来了但 Consul 配置没生效，只有一条 Warn | `optional:` 跳过了缺失/失败的加载 | 看 Warn 行确认三种跳过原因中的哪种；发布 key / 修连通性。 |
| Consul 里改了值但应用不更新 | 绑定字段不是 `gs.Dync[T]`（普通字段启动即冻结），或 watch goroutine 没起来 | 改用 `gs.Dync`；确认 import 真正执行过（找 `loaded consul config` 的 Info 日志）。 |
| 热刷新一次后失效 | Consul 重启导致 index 回退，或 KV 被删（刷新静默失败、快照陈旧） | 重新发布 key；对照 §4.5/§4.6 语义。 |
| Get 403 / permission denied | ACL token 缺失或对该路径无 `key:read` | 传 `?token=...`（策略需含读权限）；见 [Consul ACL 文档](https://developer.hashicorp.com/consul/docs/security/acl)。 |
| 文件与 Consul 都定义某 key，赢家不对 | 分层：import 在 `StorageAppFile` 层；env/cmd/profile 都在其上 | 把覆盖移到更高层，或从低层删掉该 key（`gs_conf/conf.go:27-41`）。 |
| 导入的 KV 条目里的 import 不生效 | 设计上只处理一层 import（`conf.go:213-215`） | 把所有 import 声明在本地 `app.properties`。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|--------|------|
| 配置 key 总数（import 参数） | 7（含 `optional:` 前缀） |
| starter 绑定的属性 key 数 | 0 |
| 其中必填 | 2（host:port、kv-path） |
| quickstart 前置外部依赖数 | 1（Consul agent） |
| 注意/坑条数 | 5（§3 ⚠ ×2、§4.3/§4.6/§5 条目） |

设计嫌疑清单（交设计裁决）：

- ACL token 内联在 import 字符串里会落进配置文件——没有像 vault 那样的 env / token-file 兜底 → 候选带外凭证通道。
- 一个 import 只读一个 KV 条目；不支持前缀/列表读取（key 目录）——按目录建模的应用只能每个 key 一个 import。
- watch 出错每 2 秒静默重试，本模块不打日志、无陈旧度信号（指标/健康检查）——“配置已陈旧”不可观测。
- watch goroutine 停机时从不停止；非 optional key 被删除时仍退化为陈旧快照（§4.6），但加了 WARN 后该退化在日志中可见。
- 刷新是全有或全无且全局：一个 KV 条目变更会重读所有 import 和所有文件；当前规模无碍，import 变多后需记在案。
