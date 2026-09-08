# starter-admin-ui 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均经 starter 源码核对
(`starter.go`——整个 server 就在这一个文件里,`config.go`,以及内嵌 HTML 模板字符串)
并锚定可自断言的 [example/](example/)(`example/check.sh`,零外部依赖——它自己拉起
两个 fake actuator)。actuator 端点语义属于 starter-actuator——这里只讲 dashboard
server 接线。

**激活条件**:blank import;bean 条件是 `gs.OnProperty("spring.admin-ui.addr")`——
starter 默认不装配,**只有配置了监听地址才激活**,与 starter-actuator 的
`spring.actuator.addr` 契约一致。没有 `spring.admin-ui.enabled` key,也没有默认端口:
不配 `addr` 就没有 admin-ui server bean(决策 `starter-server-port-must-be-configured`)。

> 迁移说明(相对旧的默认启用行为):旧版本默认在 `:9280` 启用 dashboard。依赖该行为
> 的用户需在配置中加上 `spring.admin-ui.addr=:9280`——`spring.admin-ui.enabled` 不再读取。

**鉴权**:dashboard 可通过 `spring.admin-ui.token`(Bearer)或
`spring.admin-ui.username` + `spring.admin-ui.password`(HTTP Basic)保护,由共享的
`stdlib/httpauth` guard 实现。未配置任何凭证且 `addr` 绑定非 loopback 接口时,启动会打
WARN(与 starter-pprof 同款模式)。

---

## 1. 完整工程示例

一个为两个 actuator 实例提供聚合 dashboard 的进程。文件树(与 `example/` 同构):

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
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-admin-ui   latest
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-admin-ui"
)

func main() { gs.Run() }
```

应用就这么多:starter 的 `init()` 注册名为 `adminUIServer` 的 `Server` bean
(starter.go:62-65),导出为 `gs.Server`;容器把 `spring.admin-ui` 树填充进它的
`Config` 字段。

**conf/app.properties**——完整注释配置面(取自 example):

```properties
# 监听地址——必填;配置它就是激活条件。与主 HTTP server(:9090)、
# actuator(:9370)、pprof(:9981)互不冲突,四者可在同一进程共存。
spring.admin-ui.addr=:9280

# 可选的 dashboard 鉴权(stdlib/httpauth guard):Bearer token,或 HTTP Basic
# (spring.admin-ui.username + spring.admin-ui.password)。全不配且 addr 非
# loopback 时,启动会打告警。示例用的是 token。
spring.admin-ui.token=example-token

# UI 轮询的 actuator base URL(下标形式;逗号分隔也可)。
# /health、/readiness、/startup、/info 由 UI 自行拼接。
spring.admin-ui.instances[0]=http://127.0.0.1:19371
spring.admin-ui.instances[1]=http://127.0.0.1:19372

# 轮询节奏与单请求超时。页面也按此节奏经 <meta http-equiv="refresh">
# 自动刷新。
spring.admin-ui.interval=1s
spring.admin-ui.timeout=500ms

# 页面标题——按环境标注 dashboard。
spring.admin-ui.title=Go-Spring Admin

# 本例只需要 Admin UI server,关闭默认主 HTTP server 保持端口干净。
spring.http.server.enabled=false
```

**验证**(与 `example/example.go runTest` 同构):

```bash
go run . -manual &
curl -s -H "Authorization: Bearer example-token" http://127.0.0.1:9280/ | grep -E '19371|19372|pill up'   # 行渲染、UP 药丸
curl -s -H "Authorization: Bearer example-token" http://127.0.0.1:9280/api/status | python3 -m json.tool  # 机器可读快照
```

杀掉一个 fake actuator(`lsof -ti:19371 | xargs kill`),一个 `interval` 内即可看到
该行翻成红色 DOWN 药丸并附错误信息。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-admin-ui
  └─ init():gs.Provide(&Server{}).Name("adminUIServer")
              .Condition(OnProperty("spring.admin-ui.addr"))
              .Export(gs.As[gs.Server]())                       [starter.go:62-66]

gs.Run()
  ├─ 配置绑定:${spring.admin-ui} → Server.Config(字段级 value tag,config.go)
  ├─ Server.Run(starter.go:100-156),顺序刻意为之:
  │    1. net.Listen(addr)        — 端口冲突在就绪屏障之前快速失败
  │    2. template.Parse          — 内嵌常量;出错即 starter bug
  │    3. http.Client 构建        — MaxIdleConns 按 len(Instances)*4 预留连接复用
  │    4. s.refresh(ctx)          — 同步种子轮询:首屏即真实数据
  │    5. newHandler():mux GET / 与 GET /api/status,外包 httpauth guard
  │       (token / basic);非 loopback 无凭证时打 WARN
  │    6. sig.TriggerAndWait()    — 触发就绪但**不阻塞**:启动期间 dashboard
  │                                  仍可访问,"与 actuator 一致"
  │    7. go pollLoop();Serve
  ├─ 稳态:一个 poller goroutine;handler 在 RWMutex 下读最后一份快照
  └─ SIGTERM:Stop → close(stop) → 等待 <-done → http.Server.Shutdown(ctx)
```

设计理由(引源码注释):

- **先 bind 后就绪**(starter.go:95-98):端口冲突在启动期、就绪屏障之前快速失败。
- **种子轮询同步执行**(starter.go:130-133):受轮询超时约束,不会拖延启动;首屏
  "显示真实数据而非空表"。
- **只触发不阻塞**(starter.go:144-147):启动期间 dashboard 必须可达,运维能看到
  状态翻转过程。
- **轮询节奏封顶**(pollLoop,starter.go:204-209):每轮 sweep 的 context 以一个
  interval 为限,"慢实例集也不会让相邻两轮重叠"。
- **快照优先于实时读**(starter.go:86-88):"过期快照好过慢页面"——handler 永不
  阻塞在某次轮询上。

### 2.2 一次轮询周期,逐步走读(refresh → pollOne)

1. Ticker 每 `interval` 触发(≤0 时回落 10s——pollLoop、handleDashboard、配置默认
   三处各自兜底)。
2. `refresh` **并行轮询全部实例**(每实例一个 goroutine,`sync.WaitGroup`)。
3. 每实例 `pollOne`(starter.go:275-358)发 **四个相互独立的 GET**:
   `/health`(失败 ⇒ `Health=DOWN` + `Error`——运维判断"整机不可达"的信号)、
   `/readiness`(另把 `components` map 抽成按名排序的逐指示器行)、
   `/startup`、`/info`(构建元数据;错误静默忽略——"nice-to-have")。
   每次调用:以 `timeout` 为单请求 context 超时(≤0 回落 3s);
   非 2xx 响应体照样解码(503 的 readiness body 带着合法 status)。
4. 结果按 base URL 排序(表格顺序稳定),加锁一次性存为快照并记 `polledAt`。
5. 下一次 `GET /` 或 `GET /api/status` 在 RLock 下拷贝快照后渲染。

**实例如何被发现**:不发现——`instances` 是绑定期一次性读取的静态配置;没有
registry/discovery 集成(见 §6 嫌疑 #2)。

### 2.3 停机走读

`Stop`(starter.go:166-183):幂等 `close(stop)` 解开 poller 的 select;
`<-done` 等 poller 退出;随后 `http.Server.Shutdown(ctx)` 搭框架停机 context 排空
在途页面请求。

---

## 3. 逐 key 行为参考

从 `spring.admin-ui` 树绑定(前缀由 `Server.Config` 上的 `${spring.admin-ui}` tag
设定,starter.go:75——顶层绝对引用,不带实例前缀)。

| Key | 类型 | 默认 | 行为/联动 | 配错的后果 |
|-----|------|------|----------|-----------|
| `addr` | string | 无(必填) | **激活 key**:配置它才装配 server bean(starter.go:62-66);刻意与主(:9090)、actuator(:9370)、pprof(:9981)错开,四 server 共存。无 `enabled` key;旧默认启用版本迁移:补配 `addr`。 | 不设 → 完全没有 server(默认关)。端口冲突在 listen 期快速失败;全网卡绑定且无凭证时打 WARN。 |
| `token` | string | `""` | 设置后每个请求需带 `Authorization: Bearer <token>`;优先于 username/password(`newHandler` 中的 httpauth guard)。 | 非 loopback 不设 → 打 WARN;dashboard 可被匿名读取。 |
| `username` / `password` | string | `""` | 两者都设置时启用 HTTP Basic 鉴权(失败返回 `WWW-Authenticate` 挑战)。 | 只设其一 → guard 不生效(同样打 WARN)。 |
| `instances` | []string | `""`(空) | actuator base URL,支持下标(`instances[0]=`)或逗号分隔。空值刻意允许——UI 降级为 "No instances configured"(config.go:34-38;模板空态)。⚠ 静态:无发现、列表无热更新。 | 空字符串条目 → 该行显示 `empty instance URL` / UNKNOWN(pollOne 守卫,starter.go:281-285)。端口写错 → DOWN 行带错误文本。 |
| `interval` | duration | 10s | 轮询节奏;同时是页面 `<meta refresh>` 周期(模板数据中最小 1s)。≤0 回落 10s(pollLoop starter.go:193-196 与 handleDashboard)。 | 过低 + 实例多 ⇒ 对 actuator 打出 4N/interval req/s。 |
| `timeout` | duration | 3s | 轮询中单个端点的 HTTP 超时;≤0 回落 3s(refresh,starter.go:220-223)。受 sweep context=一个 interval 封顶。 | 过低 ⇒ 慢 actuator 状态抖成 UNKNOWN;过高 ⇒ 呆滞实例吃光 sweep 预算。 |
| `title` | string | `Go-Spring Admin` | 渲染进 `<title>` 与页头——按环境标注(config.go:50-53)。 | 纯外观。 |

⚠ 轮询侧对实例说**裸 HTTP,无鉴权、无 TLS**——只适用于扁平可信网络(见 §6 嫌疑
#2/#3)。dashboard 自身可用 `token` / `username`+`password` 保护(见上表)。

---

## 4. 验证与故障演练

### 4.1 dashboard 渲染(对应 example `runTest`)

```bash
body=$(curl -s -H "Authorization: Bearer example-token" http://127.0.0.1:9280/)
echo "$body" | grep -c 'pill up'          # >= 2(每个健康实例一个)
echo "$body" | grep 'redis:alpha'         # readiness 逐组件行
echo "$body" | grep 'last polled:'        # 快照时间戳 + 刷新节奏
```

### 4.2 JSON 轮询端点

```bash
curl -s -H "Authorization: Bearer example-token" http://127.0.0.1:9280/api/status
# {"polled_at": "2026-...T..Z", "instances": [
#   {"base":"http://127.0.0.1:19371","health":"UP","readiness":"UP",
#    "components":[{"name":"redis:alpha","status":"UP"}],
#    "module":"example.com/alpha","version":"v0.0.1",
#    "revision":"deadbeef","build_time":"...","go":"go1.26"} ...]}
```

### 4.3 实例宕机演练

```bash
lsof -ti:19371 | xargs kill               # "打挂"一个实例
sleep 2                                    # > 一个轮询周期
curl -s -H "Authorization: Bearer example-token" http://127.0.0.1:9280/api/status | grep 19371
#   "health":"DOWN","readiness":"DOWN","startup":"DOWN","error":"...connection refused"
```

dashboard 该行翻红色 DOWN 药丸、错误内联;server 本身照常服务——坏目标永远不会
卡死 poller(单请求超时 + sweep 封顶)。

### 4.4 部分失败演练

让某个端点返回带 body `{"status":"OUT_OF_SERVICE"}` 的 503(启动中应用的 readiness
正是这样):`getJSON` 接受非 2xx(starter.go:377-379),该行显示橙色
OUT_OF_SERVICE 药丸——body 里的状态,而非传输层 DOWN。

### 4.5 空 instances 降级

删掉 `instances` 各 key(或不设):页面照常服务,显示
"No instances configured. Set spring.admin-ui.instances ..."——刻意降级而非报错
(模板空态,starter.go:552-554)。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 全部行 DOWN 且 `connection refused` | 端口写错 / actuator 不在 base URL 上;actuator 在自己的 `:9370`,不在应用端口 | 把 `instances` 指向 actuator 管理端口。 |
| 行显示 `empty instance URL` / UNKNOWN | instances 列表里有空白条目(pollOne 守卫,starter.go:281-285) | 删除空条目。 |
| 启动失败:`admin-ui: failed to listen on :9280` | 端口冲突 | 换 `addr`;不设 `addr` 即彻底关闭 starter。 |
| dashboard 显示 "No instances configured" | `instances` 未设/拼错(key 精确匹配,无 relaxed binding) | 核对 key 拼写与下标形式。 |
| 状态不刷新 / 页面停滞 | `interval` 配错(≤0 静默回落 10s);或 sweep 整体超时 | 检查 `interval`;actuator 慢就调大 `timeout`。 |
| 健康实例显示 UNKNOWN | 目标返回非 JSON——`statusOf` 在 `status` 缺失时返回 UNKNOWN(starter.go:391-399) | 目标必须讲 actuator JSON 形状。 |
| Components 列显示 "—" | 目标 `/readiness` 没有 `components` map | 非 actuator 目标的预期行为;可能仍有 /info 构建信息。 |
| HTTPS/带鉴权的 actuator 拉不通 | 轮询是裸 HTTP、无凭证、无 TLS(嫌疑 #3) | 不支持;在可信网络暴露裸 actuator 端口。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 7 |
| 必填 | 1(`addr`——它就是激活 key) |
| quickstart 前置外部依赖 | 0(目标需 actuator) |
| "注意/坑"条数 | 2 |

设计嫌疑(保留自上轮审计,已复核):

1. 已解决(原:默认启用 + 默认端口且无鉴权/告警)。激活条件改为
   `spring.admin-ui.addr`(默认关);dashboard 经共享 stdlib/httpauth guard 支持
   token/Basic 鉴权,非 loopback 无凭证绑定时打 pprof 同款 WARN。
2. `instances` 是静态配置;加实例要重新部署或配置刷新——没有 discovery 集成。
3. 轮询目标不支持凭证/TLS,只适用于扁平可信网络。
