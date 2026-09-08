# starter-registry-nacos 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`registrar.go`、`discovery_nacos.go`）与可运行的
[example/](example/)（docker-compose + `check.sh`）核实。**Nacos 自身语义
（服务/分组/命名空间/集群、ephemeral 实例、心跳）见
[Nacos 官方文档](https://nacos.io/en-us/docs/what-is-nacos.html)** —— 本文只写 go-spring 的
增量：注册生命周期、discovery 适配器、权重/摘流接线。

**范围**：本 starter 同时提供 Nacos 服务发现的两个半区 ——
- **Provider 半区**（`spring.registry.nacos.*` + `spring.registry.*`）：应用就绪后把本进程
  注册进 Nacos。面向 VM / 裸机 / 混合部署；纯 Kubernetes 下平台已替你注册 Pod
  （`starter.go:17-27`）。
- **Consumer 半区**（`spring.discovery.nacos.<name>.*`）：命名注册进 `cloud/discovery` 的
  后端，供 client starter（gateway 的 `lb://` 路由等）按名解析。

**激活方式**：设置 `spring.registry.nacos.server` 才有 registrar
（`gs.OnProperty` 前缀判断）；`spring.discovery.nacos.<name>` 有条目即有 discovery
后端（块内 `server` 留空则继承中心连接）；注册块自身还按 `discovery-name`
（默认 `nacos`）派生一个发现 bean。没有任何 `enabled` 开关。

---

## 1. 完整工程示例

一个注册进 Nacos 的 provider（`orders`）+ 通过 `lb://` 路由解析它的 consumer（gateway）
—— 覆盖 register → resolve → drain 全链路。文件树：

```
demo/
├── go.mod
├── provider/
│   ├── main.go
│   └── conf/app.properties
└── gateway/
    ├── main.go
    └── conf/app.properties
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring                     v1.3.x
    go-spring.org/starter-registry-nacos     latest
    go-spring.org/starter-gateway            latest   // consumer：lb:// 路由走 cloud/discovery
    go-spring.org/starter-echo               latest   // provider 的 HTTP 服务面
)
```

**provider/main.go** —— 一行 blank-import 即完成接入：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-echo"              // 服务 :8080
    _ "go-spring.org/starter-registry-nacos"    // 就绪后把 :8080 注册进 nacos
)

func main() { gs.Run() }
```

**provider/conf/app.properties** —— 完整注释配置面：

```properties
# --- nacos naming 连接（设置 server 即激活 registrar）-------------------------
spring.registry.nacos.server=127.0.0.1:8848
spring.registry.nacos.namespace=            # 空 = "public"
spring.registry.nacos.group=DEFAULT_GROUP   # discovery 客户端必须用同一 group
spring.registry.nacos.cluster=DEFAULT       # 须与 discovery 侧 cluster 一致
# spring.registry.nacos.username= / password=   # nacos 开启鉴权时

# --- 要发布的实例（backend 无关 key）-----------------------------------------
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080         # 必填；不会替你猜
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1

# --- 服务面 -------------------------------------------------------------------
spring.http.server.enabled=false
spring.echo.server.addr=:8080
```

**gateway/main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-gateway"
    _ "go-spring.org/starter-registry-nacos" // 同一 starter 提供 discovery 后端
)

func main() { gs.Run() }
```

**gateway/conf/app.properties**：

```properties
# 每个 nacos 集群/环境一个命名后端。
spring.discovery.nacos.prod.server=127.0.0.1:8848
spring.discovery.nacos.prod.namespace=
spring.discovery.nacos.prod.group=DEFAULT_GROUP   # ⚠ 必须与 provider 的 group 一致
spring.discovery.nacos.prod.cluster=DEFAULT       # 留空 = 跨全部集群

spring.gateway.server.addr=:9440
spring.gateway.discovery=prod                     # lb:// 路由的默认后端

spring.gateway.routes.orders.path=/api/**
spring.gateway.routes.orders.upstream.target=lb://orders
spring.gateway.routes.orders.upstream.balancer=weighted
```

**验证**（nacos 来自 `example/docker-compose.yml` —— `docker compose up -d`，然后等
`curl -fsS :8848/nacos/v1/console/health/readiness` 就绪，与 `example/check.sh` 同构）：

```bash
go run ./provider &      # 日志：tag _app_registry_nacos "registered \"orders\" at 127.0.0.1:8080"
go run ./gateway &
curl -s :9440/api/hello  # 200，经 lb://orders → 127.0.0.1:8080
# 直接查询（与 example/example.go 的 verifyOnce 同构）：
curl -s '127.0.0.1:8848/nacos/v1/ns/instance/list?serviceName=orders' | jq '.hosts[] | {ip,port,weight,enabled,healthy}'
```

example 的一次性自验（`go run ./example -manual` 可保持运行）在就绪后打印
`registered addr=... meta=...`，随后自行 SIGTERM 走完反注册路径。

---

## 2. 装配与时序

### 2.1 注册生命周期（provider 半区）

```
import starter-registry-nacos
  └─ gs.Provide(NewServer)  Name("registryServer").Export(gs.As[gs.Server]())
        └─ Condition: OnProperty("spring.registry.nacos.server")     [starter.go:64-69]
gs.Run()
  ├─ 绑定 ${spring.registry.nacos} → NacosConfig（TagArg 构造参数）
  ├─ NewServer：建 nacos naming client + FAIL-FAST 探活
  │    （GetAllServicesInfo 取 1 行 —— server 不可达/凭证错误直接终止启动）[registrar.go:83-90]
  ├─ 绑定 ${spring.registry} → Server.Config（RegistrationConfig）
  ├─ Run：就绪前校验 service-name/addr 非空                                    [starter.go:100-102]
  ├─ <-sig.TriggerAndWait()   ← 就绪门：整个应用起来后才注册
  ├─ RegisterInstance(ephemeral=true, weight 归一 <=0→1)                        [registrar.go:98-128]
  │    SDK 后台心跳保活；进程无 Deregister 死亡后 Nacos 约 15s 自动摘除 ——
  │    正确性从不依赖 Deregister
  ├─ <-ctx.Done()            ← 阻塞到停机
  └─ SIGTERM：PreStop → 先 Deregister（早于 pre-stop 延迟、早于任何 server 停止
       服务）→ 在途请求无损排空                                                [starter.go:127-129]
       Stop/Stop 兜底再反注册一次；反注册幂等
```

为什么就绪后才注册而不是启动即注册：consumer 绝不能解析到 HTTP server 还没监听的地址。
为什么在 PreStop 反注册：server 还在服务在途请求时 discovery 就停止分发该地址 —— 这个
顺序正是滚动重启无损的原因（`starter.go:30-34`）。

### 2.2 WATCH 链路（consumer 半区）

一次推送的完整走读（`discovery_nacos.go:177-245`）：

1. `client.Subscribe(ServiceName, GroupName, cb)` —— Nacos 在每次变化
   （注册/反注册/权重/健康）时推送全量实例列表。Subscribe 失败则 `Watch` 调用报错。
2. 回调**从不碰 channel**：把实例映射为 endpoint（`Enable` 取反为 `Disabled`、携带
   `Metadata["scheme"]`、按 addr 排序），互斥锁下存快照，再向 `cap-1` channel 非阻塞发
   一个信号 —— 一个 pending 信号就够，快照是全量不是增量（`discovery_nacos.go:184-195`）。
3. 单个 goroutine 独占输出 channel 的写与关闭（无 send-after-close 竞态 —— 沿用 k8s
   starter 的纪律），并用一次显式 `SelectInstances`（`HealthyOnly=true`）做种子 —— 首个
   结果不依赖 SDK 回调时序；随后循环：收到信号就把快照渲染成可比较 key
   （`addr,scheme,weight;disabled,healthy`，`endpointsKey`），**key 变了才转发** ——
   no-op 重复投递不许扰动 consumer（`discovery_nacos.go:226-242`）。
4. 下游 discovery `Loader` 只读该后端当前快照 —— 没有客户端侧循环,新鲜度全在
   后端内部;`loadbalance.Pool.Pick` 每次 pick 读取活快照。

失败形态：回调出错**保留上一份快照**（过期地址好过没有地址 —— Warn 日志，
`discovery_nacos.go:197-205`）；首次查询失败打 Warn 并等第一次推送
（`discovery_nacos.go:220-224`）；ctx 取消关闭 channel 并 Unsubscribe。

### 2.3 DRAIN 链路（UpdateWeight(0)）

1. 运维/`preStop` 钩子调用 `Server.UpdateWeight(ctx, 0)`（按名注入 `registryServer`
   这个 `gs.Server` bean，或构造期留存引用）。前置条件：注册已发生
   （`starter.go:149-154`，否则报 "instance not registered yet"）。
2. 写语义（`registrar.go:155-185`）：一次 `UpdateInstance` 调用 —— **0 原样透传**
   （Nacos 原生把 0 当作不接流量）；只有负数归一为 1。实例仍是 ephemeral，心跳继续，
   订阅者在下一次推送拿到新权重 —— 无需重新注册。
3. 观察侧看到 0：推送重新渲染 endpoint `Weight: 0`；key 变化 → 快照转发；
   `endpointsKey` 含 weight，所以这是真实变更事件。
4. `loadbalance.Pool.Pick` 在 eligibility/ejection 过滤之后跑 `excludeDrained`：剔除所有
   `Weight == 0` 的 endpoint；**回退**：全部 endpoint 都是 0 权重时过滤器让位输入
   （退化为均分），未归一化的快照不会把 pool 打成黑洞（`cloud/loadbalance/pool.go:93-134`）。
   负权重保留（配错不该静默摘除实例）。
5. 停机时的 Deregister 再把条目彻底移除。

---

## 3. 逐 key 行为参考

### 3.1 `spring.registry.nacos.*`（nacos 连接 —— 7 个 key，`config.go:21-44`）

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `server` | string | — | **激活 key**（OnProperty）。`host:port`，构造期解析。 | 缺失 → registrar 静默不存在。格式错 → 启动报 "must be host:port"。 |
| `namespace` | string | ""（public） | 注册进的 namespace id。 | 与客户端不一致 → 对方解析到另一个（空的）namespace。 |
| `group` | string | DEFAULT_GROUP | 实例发布到的分组。 | ⚠ 必须与 discovery 侧 `group` 相同，否则解析静默返回空。 |
| `cluster` | string | DEFAULT | Nacos 集群名；与 discovery 侧默认值对称（3.3）。 | provider 在集群 X、consumer 钉在别处 → consumer 看不到。 |
| `username` / `password` | string | "" | Nacos 鉴权；由启动探活验证。 | 凭证错误 → 启动在探活处失败，而非首次 Register。 |
| `timeout-ms` | uint64 | 5000 | 界定每次 nacos API 调用（含探活）。 | 过小 → 慢链路上探活/注册抖动。 |

### 3.2 `spring.registry.*`（实例 —— 4 个 key，`config.go:50-66`）

| Key | 类型 | 默认值 | 行为 | 配错后果 |
|-----|------|--------|------|----------|
| `service-name` | string | "" | 客户端解析的逻辑名。**必填** —— Run 在就绪前校验。 | 空 → 启动报错并列出两个必填 key。 |
| `addr` | string | "" | 对外通告的 `host:port`。**必填**；从不猜测。 | 空 → 启动报错；格式错 → register 期 `splitAddr` 报错。 |
| `weight` | int | 0 | 写侧归一：`<=0` 存为 **1**（registrar.go:104-107）。只有运行时 API `UpdateWeight(0)` 能存 0（摘流）。⚠ 同一字段，调用路径不同语义相反。 | 配置 `weight=0` 不会摘流 —— 实例以权重 1 接流量。 |
| `metadata` | map | 空 | 随实例存储；`scheme` key 是传输约定（`tls`/`https`/...），其余自由（zone、version）。 | 消费方启用 scheme 过滤而无 `scheme` key 时，实例按裸 TCP 对待。 |

### 3.3 `spring.discovery.nacos.<name>.*`（每个命名后端 7 个 key，`discovery_nacos.go:58-78`）

| Key | 类型 | 默认值 | 行为 | 配错后果 |
|-----|------|--------|------|----------|
| `server` | string | "" | 留空则**继承** `${spring.registry.nacos}` 中心连接（共享 naming client）。`<name>` 重复 → 容器按 bean 名报 duplicate。 | 两处都空 → 注入期报 "no ... center is configured"。 |
| `namespace` | string | "" | 必须与 provider 的一致。registrar 侧也配置了本 starter 时，两侧不一致会在启动时 WARN（`discovery_nacos.go` warnRegistryDivergence）。 | 静默解析为空；WARN 只覆盖同进程 registrar+discovery。 |
| `group` | string | DEFAULT_GROUP | 解析范围。与 registrar 侧不一致同样启动 WARN。 | ⚠ 与 provider group 不匹配 → 结果集为空，无报错。 |
| `cluster` | string | DEFAULT | 收窄到单个 nacos 集群；显式留空 = 跨全部集群。默认值 2026-08 起与 registrar 的 `DEFAULT` 对齐：零配置 consumer 能看到零配置 provider。 | **迁移**：2026-08 之前默认是 ""（全部集群）——依赖该行为的 consumer 现在须显式设 `cluster=`（空 = 全部）或钉到 provider 所在集群。 |
| `username` / `password` | string | "" | 与 registrar 相同的鉴权口径 + 启动探活。 | 启动在探活处失败。 |
| `timeout-ms` | uint64 | 5000 | 单次调用上界。 | 同 3.1。 |

后端是 IoC 容器里的命名 bean（bean 名=标签）：client starter 按名注入
（gateway：`spring.gateway.discovery` / 路由级 `upstream.discovery`）。双角色应用通常
不需要该块：`spring.registry.nacos.discovery-name`（默认 `nacos`）已从中心配置派生
同 server、同 namespace/group/cluster 的后端 bean，读写天然不漂移。

---

## 4. 验证与故障演练

以下演练均假设 §1 工程 + `example/docker-compose.yml` 的 nacos。

### 4.1 注册 / 解析 / 反注册

```bash
go run ./provider &           # 等日志：_app_registry_nacos "registered \"orders\" ..."
curl -s '127.0.0.1:8848/nacos/v1/ns/instance/list?serviceName=orders&groupName=DEFAULT_GROUP' \
  | jq '.hosts[] | {ip,port,weight,ephemeral,enabled,healthy}'
kill -TERM <provider-pid>     # PreStop 反注册：同一查询现在 hosts: []
```

example 会自跑这个闭环（`example/check.sh` 断言输出含 `registered addr=`，随后的
SIGTERM 走反注册路径）。

### 4.2 权重传播与摘流（§2.3 链路）

暴露 `gs.Server` bean（按名注入 `registryServer`），在信号处理器或管理接口里调
`UpdateWeight(ctx, 0)`；或直接用 nacos 控制台/open-api：

```bash
# 经 nacos open-api 摘流（等价于 starter 调的 UpdateInstance）：
curl -s -X PUT '127.0.0.1:8848/nacos/v1/ns/instance?serviceName=orders&ip=127.0.0.1&port=8080&weight=0&ephemeral=true&groupName=DEFAULT_GROUP'
# 消费侧观察：gateway 的 /orders 流量不再打到 :8080；
# 2 个以上权重不等的 provider 副本时，weighted 均衡器的分担比例随之变化。
```

观察 consumer 日志的"无扰动"：no-op 重复投递被 `endpointsKey` 抑制；权重变化会被转发
（weight 在 key 里）。全部副本都摘流时 pool 回退为均分而不是报错 —— 全量摘流要靠监控
发现（只有集合为空或全部 Disabled 才 ErrNoAvailable）。

### 4.3 杀掉 provider / 模拟心跳过期

1. `kill -9 <provider-pid>`（非优雅 TERM）：Deregister 没跑；SDK 心跳停止，Nacos 在心跳
   超时（默认约 15s）后摘除该 ephemeral 实例。
2. 消费侧：Nacos 推送触发，后端快照失去该 endpoint，discovery `Loader` 重读快照，
   `Pool.Pick` 不再选它。零代码改动 —— 这正是 Register 走 ephemeral、正确性不依赖
   Deregister 的原因（`registrar.go:44-47`）。
3. 与优雅 TERM（4.1）对比：那是立即摘除，不是 ~15s。

### 4.4 Watch 降级演练

- 停掉 nacos（`docker compose stop nacos`）：consumer 打 Warn
  `watch orders callback error (keeping last snapshot)` —— 过期地址继续服务。
- 对空/未就绪 server 的首次查询：Warn
  `initial query for orders failed (waiting for first push)` —— 首次推送后自愈。

### 4.5 启动期 fail-fast

```bash
spring.registry.nacos.server=127.0.0.1:9999   # 该端口无服务
go run ./provider   # 启动中止："registry-nacos: startup probe failed for 127.0.0.1:9999"
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 没有注册、也没有任何日志 | 缺 `spring.registry.nacos.server` | 设置它 —— 该 key 就是激活开关。 |
| 启动中止 "startup probe failed" | Nacos 不可达，或 namespace/凭证错误 | 修正 server/namespace/username/password；探活就是取 1 行服务列表。 |
| 启动中止 "service-name and addr are required" | `spring.registry.*` 不完整 | 两个都设上；校验发生在就绪前。 |
| 已注册但 consumer 解析为空 | provider↔consumer 的 group 或 namespace 不匹配 | 对齐 `spring.registry.nacos.group/namespace` 与 `spring.discovery.nacos.<name>.group/namespace`。 |
| consumer 看得到服务、看不到实例 | consumer 的 `cluster`（默认 DEFAULT）钉在 provider 不在的集群 | 设 `cluster=`（空 = 全部集群）或对齐 provider 的 cluster。 |
| consumer 长期持有过期地址 | nacos 挂了 / 推送报错 —— Watch 有意保留上一份快照 | 恢复 nacos；查 Warn `callback error` 日志。 |
| `UpdateWeight` 报 "instance not registered yet" | Run 注册之前就调了 | 只在就绪后调用（看 `registered ...` 日志行）。 |
| 配置 `weight=0` 但实例仍接流量 | 写侧归一把 0 存成了 1 | 用运行时 API `UpdateWeight(ctx, 0)` —— 只有该路径能存 0。 |
| 进程崩溃 ~15s 后实例没被摘 | 实例被注册成 persistent —— 本 starter 不可能 | 本 starter 全部 `Ephemeral: true`；检查是否有外部写入者写同一 ip:port。 |

运行日志携带 tag `_app_registry_nacos`（`log.RegisterAppTag("registry_nacos", "")`，
`starter.go:53`）；经 `logger.<name>.tag=_app_registry_nacos` 调整级别。

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 18（连接 7 + 实例 4 + 每个 discovery 后端 7） |
| 其中必填 | provider 侧 3（`server`、`service-name`、`addr`）+ 每个 discovery 后端 1（`server`） |
| quickstart 前置外部依赖数 | 1（nacos server；example 中 docker 门控） |
| "注意/坑"条数 | 6 |

2026-08 配置卫生改造已解决：死 key `id` 已删除（Nacos 按 ip:port 识别实例）；`cluster`
两侧默认值对齐为 `DEFAULT`（显式空值仍 = 全部集群）；registrar/discovery 的
namespace+group 不一致启动即 WARN（warnRegistryDivergence）。遗留项：配置 `weight=0`
变 1 而 API `UpdateWeight(0)` 摘流 —— 同一 key 按调用路径语义相反；注册/watch 健康无
指标（仅日志可观测）。
