# starter-config-vault 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均对照 starter 源码
（`starter.go`）与经冒烟验证的 [example/](example/) 核实。**Vault 自身语义（KV 引擎、
token、namespace、lease、seal）请看 [Vault 官方文档](https://developer.hashicorp.com/vault/docs)**，
本文只写 go-spring 的增量。本 starter 位于 `experimental/` 目录——这是全仓"未设计审核"
标记，可能有不做废弃周期的不兼容变更。

**激活方式**：blank import 本包即注册 `vault` 配置 provider
（`conf.RegisterProvider("vault", ...)`）。在 `spring.config.import` 中出现 `vault:` 条目
之前它什么都不做。没有 `enabled` 开关、没有属性 key 面、没有可注入 bean——用户面的全部
就是 import 字符串。

**边界**：只做配置中心角色——读一个 KV secret 并把字段暴露为应用属性，带轮询热刷新。
Vault Agent / CSI 挂载的 secret *文件* 属于 starter-config-file。

---

## 1. 完整工程示例

与 [example/](example/)（`example/check.sh` 对 dev 模式 Vault 冒烟验证）同构。文件树：

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── docker-compose.yml     # 本地演练用 dev Vault
```

**go.mod**（关键依赖）：

```
require (
    github.com/hashicorp/vault/api latest
    go-spring.org/spring            latest
    go-spring.org/starter-config-vault latest
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-config-vault"
)

// Demo 绑定来自 Vault 文档的两个字段。secret 变化后真正热刷新的只有
// gs.Dync[T]；普通 string 在启动时绑定一次后冻结。
type Demo struct {
    Message  gs.Dync[string] `value:"${demo.message:=none}"`
    Password gs.Dync[string] `value:"${demo.password:=none}"`
}

func main() {
    gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())
    gs.Run()
}
```

**conf/app.properties** — 完整配置面：

```properties
# 从一个 Vault KV v2 secret 导入一份配置文档。
#  - optional:  secret 尚不存在时应用也能启动
#  - 127.0.0.1:8200  Vault 地址（scheme 默认 http；TLS 加 &scheme=https）
#  - secret/gs-config-demo   <mount>/<path>
#  - key=application.properties  单字段模式：该字段承载完整 properties
#    文档，用 reader 的 "properties" 格式解析
#  - poll-ms=1000  每秒轮询一次变更
# Vault token 永远不落在这个文件里：来自 VAULT_TOKEN /
# VAULT_TOKEN_FILE / ?token-file=（见 §3）。
spring.config.import=optional:vault:127.0.0.1:8200/secret/gs-config-demo?kv-version=2&key=application.properties&format=properties&poll-ms=1000
```

**docker-compose.yml**（仅本地演练）：

```yaml
services:
  vault:
    image: hashicorp/vault:1.16
    environment:
      VAULT_DEV_ROOT_TOKEN_ID: "root"
      VAULT_DEV_LISTEN_ADDRESS: "0.0.0.0:8200"
    ports: ["127.0.0.1:8200:8200"]
```

**验证**（即 `example/check.sh` 自动化的流程）：

```bash
docker compose up -d
export VAULT_TOKEN=root

# 写入 import 指向的配置文档。单字段模式下字段名必须等于 key= 参数。
vault kv put secret/gs-config-demo application.properties="demo.message=hello
demo.password=topsecret"   # 或 docker exec ... vault kv put ...

go run .                      # 期望日志：loaded vault config from secret/gs-config-demo keys=2
# 另一个终端：
vault kv put secret/gs-config-demo application.properties="demo.message=rotated"
# 在 ~poll-ms + 刷新延迟内，Demo.Message.Value() == "rotated" —— 无需重启
```

example 还演示了属性级解密：文档值为 `ENC(aes:<base64>)` 时，conf 绑定管线在绑定前
解密，AES key 经 `GS_CONFIG_DECRYPT_AES_KEY`（或 `..._KEY_FILE`）带外注入，见
`spring/conf/decrypt/aes`。⚠ 仓库自带的 `example/check.sh` 只 export 了 `VAULT_TOKEN`
而**没有** AES key——跑加密口令演练时需 `GS_CONFIG_DECRYPT_AES_KEY=<base64 key> go run .`
（见 §6）。

---

## 2. 装配与时序

### 2.1 import 何时解析——为什么必须在 bean 之前

`spring.config.import` 在 gs 加载应用属性阶段处理，**早于任何 bean 创建**
（`spring/gs/internal/gs_conf/conf.go` 的 `loadFileImports`）：每读入一个配置文件，就解析
其 `spring.config.import` 值并经注册的 provider 链加载，结果作为 `StorageAppFile` 层
（profile 激活时为 `StorageProfileFile` 层）加入。后加载的 source 覆盖先加载的。
`spring/conf/provider/provider.go:74-104` 的两条文法规则：

- import 字符串形如 `[optional:]<provider>:<path>`——先 `optional:`，再取第一个 `:`
  前的 provider 名，其余为 provider 私有 path；
- import **只嵌套一层**：被导入 source 内部再写的 `spring.config.import` 会被静默忽略。

"pre-bean"不是实现偶然：所有 `value:"${...}"` tag——包括你 bean 上的
`${demo.message:=none}`——都从合并后的属性集绑定，因此 secret 必须先成为属性层，容器才
能装配。starter 无法提供别的时点：它运行在 IoC 容器存在之前。

### 2.2 完整链路（按序）

```
blank-import starter-config-vault
  └─ init(): gs.Provide(vaultController).Export(As[gs.Rooter]())   [starter.go:55]
     └─ conf.RegisterProvider("vault", vaultController.Load)        [starter.go:60]

gs.Run()
  ├─ 配置加载：读 conf/app.properties
  │    └─ loadFileImports 看到 spring.config.import=vault:...
  │         └─ conf.Load → vaultController.Load(optional, source)   [starter.go:228]
  │              ├─ parseSource："vault://"+source → URL → host、mount/path、query  [starter.go:107]
  │              ├─ resolveToken：?token → VAULT_TOKEN → ?token-file / VAULT_TOKEN_FILE  [starter.go:164]
  │              ├─ clientFor：按 address|namespace|token 缓存 api.Client         [starter.go:193]
  │              ├─ registerWatch：启动 watchLoop goroutine（每 poll-ms 轮询）    [starter.go:347]
  │              ├─ readSecret：KVv2(mount).Get / KVv1(mount).Get，5s 超时        [starter.go:280]
  │              │    └─ 404 → nil data → optional? 警告+跳过 : 报错 "secret not found"
  │              ├─ toProperties：整表 flatten，或 key 模式解析单字段             [starter.go:315]
  │              └─ 返回 props → 加入 StorageAppFile 层
  ├─ bean 装配：vaultCtrl 是 Rooter；其 *gs.PropertiesRefresher 经 autowire 注入
  ├─ 字段绑定：${demo.message} 从 Vault 层解析；gs.Dync 字段注册进刷新
  ├─ Run / 就绪
  └─ 稳态：每个 watchLoop 周期执行
       ├─ readSecret；出错 → continue（静默）
       ├─ fingerprint(json.Marshal(data)) 对比上次加载指纹
       └─ 有变化 → TriggerRefresh → Refresher.RefreshProperties()
            └─ 重跑整个属性加载：import 重新解析、Load 重读 secret、
               层重建，gs.Dync[T] 字段换值
```

从源码核实的关键时序：

- **不是 cold-load only。** 每个 secret 有一个永久轮询 watcher（`watchLoop`，
  starter.go:366-381），默认 5000 ms（example 用 1000 ms）。secret 轮换无需重启即可被
  感知——*但只刷新 `gs.Dync[T]` 字段*；普通 `value` tag 只在启动时绑定一次（gs 的
  refresh 仅 Dync 生效）。
- **刷新受装配状态保护**：容器完成装配前 `TriggerRefresh` 是无害 no-op——启动加载已经
  捕获了状态（starter.go:82-89；gs_app/app.go 的 `PropertiesRefresher.Started()`）。
- **指纹基于内容**（KV data map 的 `json.Marshal`）：内容完全相同的 KV v2 重写**不会**
  触发刷新；KV v2 version 号被忽略。
- **watcher 从不重读 token**：过期 token 的 client 仍按
  address|namespace|token 缓存，且 `watchLoop` 吞掉读错误——见 §4.4。

### 2.3 一次轮换，逐层走读

`vault kv put secret/gs-config-demo application.properties="demo.message=rotated"` →

1. 下一个 `watchLoop` tick（≤ poll-ms 后）重读 secret；
2. 指纹与加载时不同 → `TriggerRefresh`；
3. `RefreshProperties` 重跑配置管线：`spring.config.import` 重新解析，`Load` 重读
   （新）secret，更新存储的指纹；
4. 层自上而下重建，`demo.message` 现在解析为 `rotated`；
5. 所有绑到 `${demo.message}` 的 `gs.Dync` 字段换值；普通字段不变。

---

## 3. 逐 key 行为参考

本 starter **没有任何属性 key**——对包执行 `grep -rhoE 'value:"[^"]+"'` 返回空
（example 里的 `demo.*` 是示例数据 key，不是 starter 面）。全部配置面即 import 字符串：

```
[optional:]vault:<host>:<port>/<mount>/<path>?<query params>
```

解析方式是前面拼 `vault://` 后走 `url.Parse`（starter.go:107-161），因此 `host:port`
必须是合法 URL host，path 必须是按第一个 `/` 切分的 `<mount>/<path>`（两段都非空）。

### 3.1 路径部分

| 部分 | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|------|------|--------|-------------|----------|
| `host:port` | string | —（**必填**） | 与 `scheme` 拼成 client 地址。 | 缺失 → 启动报 `missing vault server address`。 |
| `mount` | string | —（**必填**） | KV 引擎挂载点，传给 `cli.KVv2/KVv1(mount)`。 | 错 mount → 404 → required：启动报错；optional：警告跳过。 |
| `path` | string | —（**必填**） | mount 内的 secret 路径。 | 同上；⚠ KV v2 路径**不含** `data/` 前缀——SDK 会补。 |

### 3.2 query 参数

| 参数 | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-------|------|--------|-------------|----------|
| `kv-version` | int | `2` | `1` → `cli.KVv1(mount).Get`；`2` → `cli.KVv2(mount).Get`。只接受 1/2。 | 其他值 → 启动报 `kv-version must be 1 or 2`。把 v1 secret 当 v2 读 → 404 类失败。 |
| `scheme` | string | `http` | 拼 `scheme://host:port`。⚠ 在 query 里，不在 URL scheme 位。 | HTTPS Vault 忘加 → connection reset / 明文打 TLS 端口报错。 |
| `namespace` | string | 空 | `cli.SetNamespace`——仅 Vault Enterprise。 | 对 OSS Vault 设置 → 请求报错。 |
| `key` | string | 空 | **单字段模式**：只读 secret 的该字段；必须存在且为 string；内容按 `format` 解析后 flatten。 | 字段缺失 / 非 string → 启动报错并指名字段。 |
| `format` | string | `properties` | 单字段模式的解析器（`reader.Read`：properties/yaml/json/toml…）。无 `key` 时被忽略。 | 格式不匹配 → 启动解析报错。 |
| `prefix` | string | 空 | 给每个产出 key 加 `prefix.` 前缀。 | 前缀错 → 下游 `${...}` 静默落到默认值。 |
| `poll-ms` | int | `5000` | 每个 secret 的轮询间隔；必须 > 0。 | `0`/负数/非数字 → 启动报 `invalid poll-ms`。 |
| `token` | string | — | 内联 token；**不建议**（会落配置文件/日志）。 | — |
| `token-file` | string | — | 指向文件，trim 后内容作为 token。 | 文件不可读 → 启动报错（`optional:` 也一样）。 |

### 3.3 token 解析与 `optional:` 语义

解析顺序（starter.go:164-185）：`?token=` → `VAULT_TOKEN` 环境变量 → `?token-file=` →
`VAULT_TOKEN_FILE` 环境变量（文件内容 trim）。⚠ **缺 token 时即使 `optional:` 源也会启动
失败**——token 解析发生在 `parseSource` 内部，早于 optional 判断。`optional:` 只软化
*secret 读取*失败（404 / 网络错）：有它，应用带着零个 key 启动；没它，第一次读失败即中止
启动。

环境变量：`VAULT_TOKEN`、`VAULT_TOKEN_FILE`；文档含 `ENC(aes:...)` 值时另有解密用的
`GS_CONFIG_DECRYPT_AES_KEY` / `GS_CONFIG_DECRYPT_AES_KEY_FILE`。

---

## 4. 验证与故障演练

演练均用 §1 工程。`vault` CLI 需 `export VAULT_ADDR=http://127.0.0.1:8200
VAULT_TOKEN=root`。

### 4.1 冷加载验证

```bash
vault kv put secret/gs-config-demo application.properties="demo.message=cold"
go run . 2>&1 | grep 'loaded vault config'   # "loaded vault config from secret/gs-config-demo keys=1"
```

### 4.2 轮换演练（热）

```bash
go run . -manual &        # example 的 manual 模式保持进程存活
vault kv put secret/gs-config-demo application.properties="demo.message=rotated"
# ~1s 内（poll-ms=1000）：hot-reload observed: rotated —— 无需重启，仅 gs.Dync 字段
```

内容相同的重写**不会**刷新（指纹相等）——重新 put 同一份文档，观察什么都不发生即可验证。

### 4.3 错误路径（required vs optional）

```bash
# required：spring.config.import=vault:.../secret/nope
go run .   # ERROR "vault secret secret/nope not found" → 退出

# optional：optional:vault:.../secret/nope
go run .   # WARN "optional config secret secret/nope not found (skipped)" → 启动，字段走默认值
```

### 4.4 运行中 token 过期 / 错误

正常启动后撤销 token：`vault token revoke <id>`。应用继续运行——启动不受影响——但此后
每次轮询都失败，而 `watchLoop` 直接 `continue`（starter.go:370-373）：**无日志、无计数器，
配置静默变陈旧**。Vault 的新写入永远到不了；恢复需要重启进程（watcher 从不重读 token）。
生产上信任轮换机制之前先跑这个演练。

### 4.5 启动时 Vault sealed / 宕机

`vault operator seal`（或停容器）后 `go run .`：
- required 源：`read vault secret ... failed` 报错，启动中止（fail-fast）；
- optional 源：WARN `optional config read secret ... failed (skipped)`，应用带默认值
  启动——验证字段显示 `:=` 回退值。

注意 5 秒读超时（starter.go:281）：挂起的 Vault 会让每个 import 最多拖慢启动 5 秒。

### 4.6 缺 token

```bash
env -u VAULT_TOKEN go run .   # ERROR "no vault token found (set VAULT_TOKEN, VAULT_TOKEN_FILE, ...)"
```

即使 `optional:` 也失败（§3.3）。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报 `no vault token found` | 四个槽位都没 token | export `VAULT_TOKEN` 或用 `?token-file=`；`optional:` 源同样会报。 |
| `vault secret <mount>/<path> not found` | mount/path 错，或 KV v1 secret 被默认 `kv-version=2` 读 | 修路径；加 `&kv-version=1`；KV v2 路径不得含 `data/`。 |
| `vault path must be <mount>/<path>` | path 少于两段（如 `vault:...:8200/secret`） | 补全 `<host>:<port>/<mount>/<path>`。 |
| 应用起来了，Vault 值一直是默认值 | `optional:` 吞掉了读失败，或 `prefix` 错 | grep 启动日志 `optional config ... (skipped)`；核对 prefix 与 `${...}` key。 |
| Vault 改了，应用不更新 | 字段非 `gs.Dync`，或轮询已静默失败（token 被撤销） | 用 `gs.Dync[T]` 绑定；跑 §4.4 演练；死 token 靠重启恢复。 |
| connection reset / TLS 报错 | HTTPS Vault 没加 `&scheme=https` | 加上——默认 `http`。 |
| `parse vault field "x" ... failed` / `has no field "x"` | 单字段模式不匹配 | `key=` 必须指向已存在的 string 字段；`format=` 须匹配其内容。 |
| `kv-version must be 1 or 2` / `invalid poll-ms` | query 参数非数字或越界 | 修正取值；两者都在解析期校验。 |
| 字段刷新后又回跳 | 低优先级层（应用文件）覆盖了 Vault 层 | 记住覆盖序：后加载 import 覆盖先加载；profile 层高于 app 层。 |

---

## 6. 设计体检表与嫌疑清单

| 指标 | 数值 |
|--------|------|
| 属性 key | 0（全部配置面在 import 字符串） |
| import 字符串参数 | 10（3 路径段 + 7 query） |
| 必填项 | 3 路径段 + 四槽位之一的 token |
| quickstart 外部依赖 | 1（Vault） |
| 可注入 bean / 程序化 API | 0 / 0 |
| "注意/坑"条数 | 6 |

设计嫌疑清单（交设计裁决；前三条沿自上一版）：

1. config 家族里最大的 source 字符串面（10 参数）；`token`/`prefix`/`poll-ms` 作为属性
   key 更可读 → 候选拆分"连接类"参数为 key。
2. 仅轮询的变更感知：无 sys/leases 通知，KV v2 version 号被忽略——同内容重写不刷新；
   `fingerprint()` 只做内容哈希，行为已核实但未文档化。
3. 轮询出错静默 `continue`，无失败计数与日志——死 token 或 sealed Vault 退化为静默陈旧
   配置（演练 §4.4）。
4. `resolveToken` 在 `parseSource` 内执行，缺 token 连 `optional:` 源也失败——fail-fast
   可辩护，但与 `optional:` 的字面语义不对称。
5. `scheme` 藏在 query（`?scheme=https`）而非接受真正的 `vault://host/...` /
   `https://host/...` URL 形态——容易漏配，且无内联提示。
6. example 漂移：`example/check.sh` 只 export `VAULT_TOKEN` 没有
   `GS_CONFIG_DECRYPT_AES_KEY`，`example.go` 声明了未使用的 `aesKey` 常量、init 注释声称
   设置了实际没设的环境变量——按现脚本 ENC 解密一腿的冒烟无法通过；需修脚本或 init。
7. 无健康检查、无指标、无专属 log tag（用 `_app_def`）：配置陈旧与否离开外部探测不可观测。
