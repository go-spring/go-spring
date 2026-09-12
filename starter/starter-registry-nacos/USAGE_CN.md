# starter-registry-nacos 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`center.go`、`registrar.go`、`discovery_nacos.go`）、注册核心
（`../starter-registry/starter.go`、`../starter-registry/config.go`）与可运行的
[example/](example/)（docker-compose + `check.sh`）核实。**Nacos 自身语义
（服务/分组/命名空间/集群、ephemeral 实例、心跳）见
[Nacos 官方文档](https://nacos.io/en-us/docs/what-is-nacos.html)** —— 本文只写 go-spring 的
增量：注册生命周期、discovery 适配器、权重/摘流接线。

**范围**：本 starter 同时提供 Nacos 服务发现的两个半区，且都由每个已配置中心的同一个
bean 承载 ——
- **Provider 半区**（`spring.registry.nacos.<name>.*` + `spring.registry.*`）：应用就绪后把
  本进程注册进每一个已配置的 Nacos 中心。面向 VM / 裸机 / 混合部署；纯 Kubernetes 下平台
  已替你注册 Pod。
- **Consumer 半区**（自身零配置）：同一个 bean 就是一个 `cloud/discovery` 后端，client
  starter（gateway 的 `lb://` 路由、`spring.http-client.backends.<n>.discovery` 等）通过引用
  bean 名 `nacos.<name>` 来解析。

**激活方式**：每个 `spring.registry.nacos.<name>` 块即一个注册中心 —— 一个共享 naming
client、一次启动探测、一套生命周期（`center.go`）。名为 `nacos.<name>` 的 bean 同时导出
`discovery.Registrar`（设置了 `spring.registry.service-name` 时由 starter-registry 核心收集
—— 纯消费方应用不注册任何实例）与 `discovery.Discovery`（按 bean 名引用，纯提供方不为读
侧付出任何成本）。

没有任何 `enabled` 开关。没有默认块 / 匿名块：块名必填，且成为 bean 名的一部分。

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

**provider/main.go** —— 一行 blank-import 即完成接入（注册核心 `starter-registry` 随之传递
导入）：

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
# --- nacos naming 连接：每个服务端一个命名块（设置 server 即激活该块；
#     bean 名 = "nacos." + 块名）---------------------------------------------
spring.registry.nacos.main.server=127.0.0.1:8848
spring.registry.nacos.main.namespace=            # 空 = "public"
spring.registry.nacos.main.group=DEFAULT_GROUP   # 注册与发现共享
spring.registry.nacos.main.cluster=DEFAULT       # 注册与发现共享
# spring.registry.nacos.main.username= / password=   # nacos 开启鉴权时
# 第二个中心就是第二个块：注册同时写进两处（双写）
# spring.registry.nacos.dr.server=10.0.2.1:8848

# --- 要发布的实例（backend 无关 key，所有中心共享）---------------------------
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080         # 注册时必填；不会替你猜
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

**gateway/conf/app.properties** —— 只需连接块；发现引用后端 bean 名，自身零配置：

```properties
# 中心：与纯提供方相同的连接块（无 service-name → 不注册任何实例）。
spring.registry.nacos.main.server=127.0.0.1:8848
spring.registry.nacos.main.group=DEFAULT_GROUP

spring.gateway.server.addr=:9440
spring.gateway.discovery=nacos.main                 # 后端 bean 名

spring.gateway.routes.orders.path=/api/**
spring.gateway.routes.orders.upstream.target=lb://orders
# 路由的负载均衡策略不是 gateway 的 key，而是该路由标签上的治理规则：
# govern.rules[N].resources=gateway:orders + govern.rules[N].balancer=weighted
# （见 cloud/governance/CONFIG_CN.md §3.1）
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

注册由本 starter 传递导入的 **starter-registry 核心**（`../starter-registry`）持有；nacos
starter 只按块贡献 registrar bean：

```
import starter-registry-nacos
  ├─ 每个块 ${spring.registry.nacos.<name>}：Provide(newNacosBackend)
  │    Name("nacos."+name).Export(As[discovery.Discovery], As[discovery.Registrar])
  │    条件：gs.OnProperty("spring.registry.nacos")                   [center.go]
  └─ import starter-registry（核心，每进程恰一次）
       └─ gs.Provide(NewServer).Name("registryServer").Export(gs.As[gs.Server]())
            条件：gs.OnProperty("spring.registry.service-name")
gs.Run()
  ├─ conf.BindEach 遍历 spring.registry.nacos.* → 每块一份 NacosConfig
  ├─ 每块 newNacosBackend：建 nacos naming client + FAIL-FAST 探活
  │    （GetAllServicesInfo 取 1 行 —— server 不可达/凭证错误直接终止启动）[center.go]
  ├─ registryServer 经 []discovery.Registrar 切片注入收集所有后端的 registrar
  │    （跨所有后端 —— nacos、zookeeper……）
  ├─ Run：就绪前校验 service-name/addr 非空且 registrar ≥ 1 个
  ├─ <-sig.TriggerAndWait()   ← 就绪门：整个应用起来后才注册
  ├─ 逐个 registrar：RegisterInstance(ephemeral=true, 仅负权重归一为 1)
  │    SDK 后台心跳保活；进程无 Deregister 死亡后 Nacos 约 15s 自动摘除 ——
  │    正确性从不依赖 Deregister；任一中心失败即终止启动（各中心消费侧视图不得分裂）
  ├─ <-ctx.Done()            ← 阻塞到停机
  └─ SIGTERM：PreStop → 向每一个中心 Deregister（早于 pre-stop 延迟、早于任何
       server 停止服务）→ 在途请求无损排空
       Stop 兜底再反注册一次；反注册幂等
```

为什么就绪后才注册而不是启动即注册：consumer 绝不能解析到 HTTP server 还没监听的地址。
为什么在 PreStop 反注册：server 还在服务在途请求时 discovery 就停止分发该地址 —— 这个
顺序正是滚动重启无损的原因。

### 2.2 新鲜度链路（consumer 半区）

新鲜度全在后端内部，没有客户端侧 watch 循环（`discovery_nacos.go`）：

1. 对某服务的**第一次** `Resolve` 支付一次种子查询（`SelectInstances`，`HealthyOnly=true`，
   限定在该块的 group/cluster 内），随后打开 Nacos 订阅
   （`client.Subscribe(ServiceName, GroupName, cb)`）。Subscribe 失败则该次 Resolve 报错。
2. Nacos 的每次推送（注册/反注册/权重/健康变化）触发回调：把实例映射为 endpoint
   （`Enable` 取反为 `Disabled`、携带 `Metadata["scheme"]`、按 addr 排序），在 entry 的
   互斥锁下存入全量快照。
3. 之后的 `Resolve` 就是对缓存快照的内存读（可按 `scheme` 查询选项收窄）；discovery
   `Loader` 只读该后端当前快照，`loadbalance.Pool.Pick` 每次 pick 读取活快照。

失败形态：推送出错保留上一份快照（过期地址好过没有地址 —— Warn 日志）；种子查询失败则
第一次 Resolve 报错，订阅仍会在首次推送后恢复。

### 2.3 DRAIN 链路（UpdateWeight(0)）

1. 运维/`preStop` 钩子在 `registryServer` bean 上调用 `Server.UpdateWeight(ctx, 0)`（按名
   注入，或构造期留存引用）。前置条件：注册已发生（否则报 "instance not registered yet"）。
2. 核心向每一个 registrar 广播；nacos 侧（`registrar.go`）对每个中心发一次
   `UpdateInstance` —— **0 原样透传**（Nacos 原生把 0 当作不接流量）；只有负数归一为 1。
   实例仍是 ephemeral，心跳继续，订阅者在下一次推送拿到新权重 —— 无需重新注册。
3. 观察侧看到 0：推送重新渲染 endpoint `Weight: 0`；快照被替换；
   `loadbalance.Pool.Pick` 在 eligibility/ejection 过滤之后跑 `excludeDrained`：剔除所有
   `Weight == 0` 的 endpoint；**回退**：全部 endpoint 都是 0 权重时过滤器让位输入（退化为
   均分），未归一化的快照不会把 pool 打成黑洞。负权重保留（配错不该静默摘除实例）。
4. 停机时的 Deregister 再把条目彻底移除。

---

## 3. 逐 key 行为参考

### 3.1 `spring.registry.nacos.<name>.*`（nacos 连接 —— 每块 7 个 key，`config.go`）

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `server` | string | — | **块激活 key**。`host:port`，构造期解析。 | 块内缺失 → 该块绑定失败。格式错 → 启动报 "must be host:port"。 |
| `namespace` | string | ""（public） | 注册与发现共享的 namespace id（块持有）。 | 与被消费的 provider 不一致 → 对方在另一个 namespace。 |
| `group` | string | DEFAULT_GROUP | 注册与发现共享的分组 —— 一个值服务两半。 | 与别处注册的 provider 不匹配 → 解析静默返回空。 |
| `cluster` | string | DEFAULT | 两半共享的 Nacos 集群名。 | provider 在集群 X、本块钉在别处 → 看不到。 |
| `username` / `password` | string | "" | Nacos 鉴权；由启动探活验证。 | 凭证错误 → 启动在探活处失败，而非首次 Register。 |
| `timeout-ms` | uint64 | 5000 | 界定每次 nacos API 调用（含探活）。 | 过小 → 慢链路上探活/注册抖动。 |

同一 `<name>` 的两个块会在容器里因 bean 重名而响亮报错；不同后端之间的块名永不冲突
（bean 名带后端类型，如 `nacos.main` 与 `zookeeper.main`）。

### 3.2 `spring.registry.*`（实例 —— 5 个 key，`../starter-registry/config.go`）

| Key | 类型 | 默认值 | 行为 | 配错后果 |
|-----|------|--------|------|----------|
| `service-name` | string | "" | 客户端解析的逻辑名。**注册意图信号**：设置即本进程向每个已配置中心发布自己；不设即纯消费方。 | 配了连接块但不设 → 什么都不注册（合法的纯消费方应用）。 |
| `addr` | string | "" | 对外通告的 `host:port`。注册时必填；从不猜测。 | 设了 service-name 而为空 → 启动报错并列出两个必填 key。 |
| `id` | string | "" | 由核心绑定；nacos 后端忽略它（Nacos 按 ip:port 识别实例）。 | — |
| `weight` | int | 100 | 写侧归一：只有**负权重**存为 1。0 即摘流信号，两条写路径均原样透传。 | 负权重静默变 1，不报错。 |
| `metadata` | map | 空 | 随实例存储；`scheme` key 是传输约定（`tls`/`https`/...），其余自由（zone、version）。 | 消费方启用 scheme 过滤而无 `scheme` key 时，实例按裸 TCP 对待。 |

### 3.3 发现（零 key —— 引用 bean 名）

不存在按后端命名的 discovery 配置路径。每个块的 bean **就是**发现后端，名为
`nacos.<name>`（`center.go`），共享该块的 naming client、namespace、group 与 cluster ——
读写用的是同一组值，天然不会漂移。client 按 bean 名引用：gateway 的
`spring.gateway.discovery=nacos.main` / 路由级 `upstream.discovery=nacos.main`、
`spring.http-client.backends.<n>.discovery=nacos.main` 等。读侧在首次引用前零成本：纯提供方
不为它付出任何成本。要从多个中心解析，在需要处分别引用各中心的 bean 名即可。

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

### 4.2 双写（两个块）与摘流

给 provider 加第二个块（`spring.registry.nacos.dr.server=...`），对**两台** server 重复 §4.1
的查询：实例出现在每一处（一次发布扇出），日志行为 `in 2 registry center(s)`。然后摘流
—— 暴露 `registryServer` bean（按名注入），在信号处理器或管理接口里调
`UpdateWeight(ctx, 0)`；或直接用 nacos 控制台/open-api：

```bash
# 经 nacos open-api 摘流（等价于 starter 调的 UpdateInstance）：
curl -s -X PUT '127.0.0.1:8848/nacos/v1/ns/instance?serviceName=orders&ip=127.0.0.1&port=8080&weight=0&ephemeral=true&groupName=DEFAULT_GROUP'
# 消费侧观察：gateway 的 /orders 流量不再打到 :8080；
# 2 个以上权重不等的 provider 副本时，weighted 均衡器的分担比例随之变化。
```

全部副本都摘流时 pool 回退为均分而不是报错 —— 全量摘流要靠监控发现（只有集合为空或全部
Disabled 才 ErrNoAvailable）。

### 4.3 杀掉 provider / 模拟心跳过期

1. `kill -9 <provider-pid>`（非优雅 TERM）：Deregister 没跑；SDK 心跳停止，Nacos 在心跳
   超时（默认约 15s）后摘除该 ephemeral 实例。
2. 消费侧：Nacos 推送触发，后端快照失去该 endpoint，discovery `Loader` 重读快照，
   `Pool.Pick` 不再选它。零代码改动 —— 这正是 Register 走 ephemeral、正确性不依赖
   Deregister 的原因。
3. 与优雅 TERM（4.1）对比：那是立即摘除，不是 ~15s。

### 4.4 新鲜度降级演练

- 停掉 nacos（`docker compose stop nacos`）：consumer 打 Warn
  `push for orders failed (keeping last snapshot)` —— 过期地址继续服务。
- 对空/未就绪 server 的首次解析：种子查询报错；订阅在首次推送后自愈。

### 4.5 启动期 fail-fast

```properties
spring.registry.nacos.main.server=127.0.0.1:9999   # 该端口无服务
```

```bash
go run ./provider   # 启动中止："registry-nacos: startup probe failed for 127.0.0.1:9999"
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 没有注册、也没有任何日志 | 没有任何 `spring.registry.nacos.*` 块（或块内缺 `server`） | 配置一个命名块 —— `spring.registry.nacos.<name>.*` 下任意 key 都会激活后端。 |
| 启动中止 "startup probe failed" | Nacos 不可达，或 namespace/凭证错误 | 修正 server/namespace/username/password；探活就是取 1 行服务列表。 |
| 启动中止 "service-name and addr are required" | `spring.registry.*` 不完整 | 两个都设上；校验发生在就绪前。 |
| 启动中止 "…… but no registry center is configured" | 设了 `service-name` 却没有任何连接块 | 至少加一个 `spring.registry.<backend>.<name>` 块。 |
| 已注册但 consumer 解析为空 | provider 注册在了与被引用块不同的 group/namespace 下 | 两侧必须使用相同的块级 `group`/`namespace`（本 starter 读写共享一个值）。 |
| consumer 看得到服务、看不到实例 | 块的 `cluster`（默认 DEFAULT）钉在 provider 不在的集群 | 把该块的 `cluster` 对齐到 provider 的集群。 |
| consumer 长期持有过期地址 | nacos 挂了 / 推送报错 —— 后端有意保留上一份快照 | 恢复 nacos；查 Warn `push ... failed` 日志。 |
| `UpdateWeight` 报 "instance not registered yet" | Run 注册之前就调了 | 只在就绪后调用（看 `registered ...` 日志行）。 |
| 负 `weight` 静默按 1 注册 | 写侧归一仅钳 `<0` | 属配错防护；应设 `0`（或正值）。 |
| 进程崩溃 ~15s 后实例没被摘 | 实例被注册成 persistent —— 本 starter 不可能 | 本 starter 全部 `Ephemeral: true`；检查是否有外部写入者写同一 ip:port。 |

运行日志携带 tag `_app_registry_nacos`（`log.RegisterAppTag("registry_nacos", "")`）；
经 `logger.<name>.tag=_app_registry_nacos` 调整级别。

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 12（每块连接 7 + 实例 5） |
| 其中必填 | 每块 1 个（`server`）+ 注册时 2 个（`service-name`、`addr`）；纯消费方只需一个块的 `server` |
| quickstart 前置外部依赖数 | 1（nacos server；example 中 docker 门控） |
| "注意/坑"条数 | 6 |

2026-08 配置卫生改造已解决：死 key `id` 已删除（Nacos 按 ip:port 识别实例；核心仍在
`${spring.registry}` 下为其他后端绑定 `id`，nacos 侧忽略）。2026-09 多注册中心命名块改造
已解决：单块形态替换为 `spring.registry.nacos.<name>.*` —— 每块一个名为 `nacos.<name>`
的后端 bean 同时承载 registrar 与发现后端，注册收进共享的 starter-registry 核心（一次发布
扇出到每个中心），发现改为引用 bean 名而非固定标签。可观测性：注册与发现经 OTel 全局产出指标——未引入 `starter-otel` 时全部 no-op。`register`、`deregister`、`update_weight` 各有一个 client span 与一条 `registry.operation.duration`（标签 `system`/`operation`/`service`/`status`）；`registry.registration.attempts_total` 按 `reason` 与 `status` 计数；`registry.instance.registered` gauge 在发布中为 1、否则为 0——自愈失败会落在这里，而不只是出现在日志里。发现半边把每次后台缓存同步上报到 `discovery.sync_total`，并维持 `discovery.cache.age_seconds`（距上次确认新鲜的秒数），watch 死掉时表现为持续爬升，而不是静默返回陈旧地址。 nacos 无自愈重注册（存活由 SDK 的 ephemeral 心跳维持），`reason` 恒为 `initial`。
