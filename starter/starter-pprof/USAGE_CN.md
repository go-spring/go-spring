# starter-pprof 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均经 starter 源码核对
(`starter.go`、`pprof.go`)并锚定可自断言的 [example/](example/)(`example/check.sh`,
零外部依赖)。**profiling 语义(profile 类型、`go tool pprof` 用法)见
[Go 标准库文档](https://pkg.go.dev/net/http/pprof)**——以下全部是 go-spring 增量:
server 接线、启停开关、鉴权。

**激活条件**:blank import。**默认启用**——`gs.OnProperty("spring.pprof.enabled").
HavingValue("true").MatchIfMissing()`(starter.go:28-29)。这是仓库唯一刻意保留的
端口例外(决策 `starter-server-port-must-be-configured`):pprof starter 默认 `:9981`
全网卡监听、零配置即启动,目的就是开箱可做运行期诊断。用
`spring.pprof.enabled=false` 显式关闭。

---

## 1. 完整工程示例

一个在 loopback 上暴露受保护 pprof 端点的服务。文件树(与 `example/` 同构):

```
demo/
├── go.mod
├── main.go
└── conf/
    └── app.properties
```

**go.mod**(关键依赖):

```
require (
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-pprof   latest
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-pprof"
)

func main() { gs.Run() }
```

应用就这么多:blank import 注册 `NewSimplePProfServer` bean(starter.go:34-37),
导出为 `gs.Server`,框架与其他 server 一起启动/排空它。

**conf/app.properties**——完整注释配置面(取自 example):

```properties
# 专用 pprof HTTP server 地址。starter 默认 ":9981"(所有网卡);
# example 固定 loopback,让自测可以确定性访问。
spring.pprof.addr=127.0.0.1:9981

# token 保护。设置后,每个请求必须以 "Authorization: Bearer <token>"
# 请求头呈现 token。也支持 Basic auth
# (spring.pprof.username / spring.pprof.password)。
spring.pprof.token=s3cr3t

# 显式关闭开关(取消注释即整体停用 server):
# spring.pprof.enabled=false
```

**验证**(与 `example/example.go runTest` 同构):

```bash
go run . -manual &
curl -i http://127.0.0.1:9981/debug/pprof/                          # 401 unauthorized
curl -i -H 'Authorization: Bearer s3cr3t' http://127.0.0.1:9981/debug/pprof/   # 200
curl -i -H 'Authorization: Bearer s3cr3t' http://127.0.0.1:9981/debug/pprof/heap  # 200
go tool pprof http://127.0.0.1:9981/debug/pprof/profile?seconds=5 -H '' # 见 §4.3
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-pprof
  └─ init():gs.Provide(NewSimplePProfServer,
            gs.IndexArg(1, gs.TagArg("${spring.pprof}")))
              .Condition(OnProperty("spring.pprof.enabled").MatchIfMissing())
              .Export(gs.As[gs.Server]())                      [starter.go:28-37]

gs.Run()
  ├─ 配置绑定:${spring.pprof} → Config(value tag,pprof.go:35-48)
  ├─ bean 构造:NewSimplePProfServer
  │    ├─ mux:注册 /debug/pprof/{,cmdline,profile,symbol,trace}(pprof.go:68-72)
  │    ├─ 告警检查:!guard.Enabled() && !httpauth.IsLoopback(addr) →
  │    │   log.Warnf("pprof server listening on %q without authentication; ...")  [pprof.go:74-78]
  │    └─ guard:httpauth.Guard token/basic-auth 包装 mux(stdlib/httpauth)
  ├─ Run:gs.Server 集合与主 HTTP server 一起启动
  └─ SIGTERM:随框架的 server 优雅排空
```

注意构造形态:两参(`ctx *gs.ContextProvider`、`c Config`)经
`gs.IndexArg(1, gs.TagArg(...))` 绑定——零配置的 blank import 也能拿到默认值绑定。

### 2.2 一次请求,逐步走读

配了 `token=s3cr3t` 的 `GET /debug/pprof/heap`(带 Bearer 头):

1. `http.ServeMux` 把 `GET /debug/pprof/` 模式路由给 `pprof.Index`(heap 等其余
   profile handler 挂在 index 下——Go 标准库行为)。
2. `guard`(`httpauth.Guard.Wrap`,stdlib/httpauth):配了 token,只接受
   `Authorization: Bearer` 头,比较走 `subtle.ConstantTimeCompare`,
   时序不泄露秘密;`?token=` query 参数不再接受。
3. 匹配 → stdlib pprof handler 渲染;不匹配 → `401 unauthorized`。
4. 什么鉴权都没配时,`guard` 原样返回 handler——但构造期在非 loopback 绑定下
   已经打了无鉴权告警(pprof.go:74-78)。

**是 gs.Server,不是 Runner**:bean 导出为 `gs.Server`(starter.go:37),按类型收进
`[]Server` 集合——框架像对待任何 server 一样跑它。这很重要:阻塞的 server 会死锁
注册,而 `SimpleHttpServer.Serve` 在 `TriggerAndWait` 后返回,符合"Runner 副作用
不得阻塞就绪信号"的仓库规则。

---

## 3. 逐 key 行为参考

| Key | 类型 | 默认 | 行为/联动 | 配错的后果 |
|-----|------|------|----------|-----------|
| `spring.pprof.enabled` | bool | true | 带 `MatchIfMissing` 的激活条件(starter.go:28)。缺省=开。 | 忘了关 → 每个环境都在 `:9981` 暴露端点。 |
| `spring.pprof.addr` | string | `:9981` | 监听地址。`:9981` 绑**所有网卡**;`127.0.0.1:9981` 仅 loopback(`httpauth.IsLoopback`,stdlib/httpauth,把 ""/`0.0.0.0` 视为非 loopback)。⚠ 这是"端口不设默认"规则的刻意例外——任何出网部署必须改它或配鉴权。 | 默认 + 无鉴权 → 运行期内部信息出网可达(只有一条启动告警)。端口冲突 → 启动期 bind 报错。 |
| `spring.pprof.token` | string | `""` | 设置后,每个请求需带 `Authorization: Bearer <token>` 请求头;**优先于** username/password(`httpauth.Guard`,stdlib/httpauth)。 | 只设 username/password + token 空可行;设了 token 后 basic auth 不可达。 |
| `spring.pprof.username` | string | `""` | HTTP Basic 用户名——仅在 username 与 password **同时**设置时生效(`httpauth.Guard.Enabled`,stdlib/httpauth)。 | 只设其一 → 静默无鉴权(非 loopback 时有无鉴权告警)。 |
| `spring.pprof.password` | string | `""` | HTTP Basic 密码,常量时间比较(`httpauth`,stdlib/httpauth)。 | 与 username 同耦合。 |

---

## 4. 验证与故障演练

### 4.1 token 鉴权,仅 header 形态(对应 example `runTest`)

```bash
curl -i http://127.0.0.1:9981/debug/pprof/cmdline                # 401 unauthorized
curl -i -H 'Authorization: Bearer s3cr3t' http://127.0.0.1:9981/debug/pprof/cmdline   # 200
curl -i 'http://127.0.0.1:9981/debug/pprof/cmdline?token=s3cr3t'  # 401(query 形态已移除)
curl -i -H 'Authorization: Bearer wrong' http://127.0.0.1:9981/debug/pprof/cmdline   # 401
```

### 4.2 basic-auth 变体

```properties
spring.pprof.username=admin
spring.pprof.password=secret
```

```bash
curl -i http://127.0.0.1:9981/debug/pprof/                 # 401 + WWW-Authenticate: Basic realm="restricted"
curl -i -u admin:secret http://127.0.0.1:9981/debug/pprof/ # 200
```

(同时设 `token` 会让 basic auth 不可达——优先级规则,见 §3。)

### 4.3 采集 profile

```bash
curl -s -H 'Authorization: Bearer s3cr3t' 'http://127.0.0.1:9981/debug/pprof/profile?seconds=5' -o cpu.pb
go tool pprof -top demo-binary cpu.pb        # CPU profile 解读见 Go 标准库文档
curl -s -H 'Authorization: Bearer s3cr3t' http://127.0.0.1:9981/debug/pprof/heap -o heap.pb
go tool pprof -sample_index=alloc_space -top demo-binary heap.pb
```

### 4.4 无鉴权暴露演练

从配置删掉 `token` 并设 `spring.pprof.addr=:9981`;启动时日志出现(pprof.go:75-77):

```
WARN ... pprof server listening on ":9981" without authentication; set ${spring.pprof.token} or ${spring.pprof.username}/${spring.pprof.password}
```

此时网络上任何主机都能读 goroutine 栈与 heap 数据——这条告警是唯一信号。
恢复 loopback 或鉴权即可消除。

### 4.5 关闭演练

`spring.pprof.enabled=false` → 条件不成立,无 bean、无端口、无告警。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 所有请求 401 | token/basic 配错或凭证错误 | 确认当前生效的方案;记住 token 优先于 basic。 |
| basic auth 永远 401 | 同时设了 `token`——basic 路径不可达 | 清掉 `token`,同时设 `username` 与 `password`。 |
| 启动告警 "without authentication" | `:9981` 绑定 + 无鉴权 | 设 `spring.pprof.addr=127.0.0.1:9981` 或配鉴权(pprof.go:74-78)。 |
| :9981 端口被占 | 宿主机上其他进程或另一个 gs 应用 | 换 `addr`,或 `spring.pprof.enabled=false`。 |
| server 根本没起 | 配置链里某处 `spring.pprof.enabled=false` | 删除/翻转该 key;缺省即启用。 |
| `go tool pprof` 带鉴权拉不下来 | 某些场景不会从 URL 带凭证 | 用 curl 下载(§4.3)后把文件交给 `go tool pprof`。 |
| 想 prod 关、dev 开 | 单份共享配置 | 按环境分配置:prod profile 里 `spring.pprof.enabled=false`。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 5 |
| 必填 | 0 |
| quickstart 前置外部依赖 | 0 |
| "注意/坑"条数 | 2 |

设计嫌疑(保留自上轮审计,已复核):

1. 本 starter 违反仓库"server 端口不设默认"规则——`:9981` 全网卡 + 默认启用是
   已接受的刻意例外(starter.go:24-29;决策 `starter-server-port-must-be-configured`),
   但每份文档都必须重述暴露风险。
2. 已解决:`?token=` query 回落已移除(只收 header),因为 query 串会把 token 泄进
   访问日志、浏览器历史与 shell 历史。guard 本体已抽到共享包 `stdlib/httpauth`
   (bearer + Basic、常量时间比较、`IsLoopback`),actuator 与 admin UI starter 可复用。
