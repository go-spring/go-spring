# starter-registry-zookeeper 使用说明 — 参考手册

详细使用文档。概览见 [README](README_CN.md)。所有行为声明均已对照 starter 源码（`starter.go`、
`registrar.go`、`center.go`、`discovery_zookeeper.go`、`config.go`、`registrar_test.go`）、
注册核心（`../starter-registry/starter.go`、`../starter-registry/config.go`）、`cloud/discovery`
接缝（`cloud/discovery/registrar.go`、`cloud/loadbalance/pool.go`）与可运行的 [example/](example/)
（`example/check.sh` 跑单测 + docker-compose ZooKeeper 端到端启动）核实。**ZooKeeper 自身语义
（会话、临时 znode、watcher、digest 认证）见
[ZooKeeper 文档](https://zookeeper.apache.org/doc/current/zookeeperProgrammers.html)**——
本文只讲 go-spring 的增量。

**激活条件**：每个 `spring.registry.zookeeper.<name>` 块即一个注册中心 —— 一个共享会话、一次
启动探测、一套生命周期（`center.go`）。名为 `zookeeper.<name>` 的 bean 同时导出
`discovery.Registrar`（设置了 `spring.registry.service-name` 时由 starter-registry 核心收集
—— 纯消费方应用不注册任何实例）与 `discovery.Discovery`（按 bean 名引用，纯提供方不为读侧
付出成本）。本 starter **两侧都做**：把本实例以临时 znode 发布到 ZooKeeper，同时携带客户端
发现后端（见 §1 消费侧说明）。不监听端口 —— 注册经核心的 `registryServer` 接进应用生命周期。
没有默认块 / 匿名块：块名必填，且成为 bean 名的一部分。

---

## 1. 完整工程示例

两侧角色：**提供方**（本 starter + 一个被服务的端点）与**消费方**（任意可通过 ZooKeeper 支撑的
`discovery.Discovery` 解析的服务发现型 client starter）。文件树：

```
demo/
├── go.mod
├── main.go
└── conf/
    └── app.properties
```

**前置依赖**（唯一外部依赖）：一个 ZooKeeper 节点——照抄 `example/docker-compose.yml`
（zookeeper:3.9，127.0.0.1:2181）：

```bash
docker compose -f example/docker-compose.yml up -d
# 就绪探活（四字命令；即 check.sh 的轮询方式）：
( echo ruok; sleep 1 ) | nc 127.0.0.1 2181        # -> imok
```

**go.mod**：

```
require (
    github.com/go-zookeeper/zk           v1.0.4
    go-spring.org/spring                 v1.3.x
    go-spring.org/starter-registry-zookeeper latest
    go-spring.org/starter-redigo         latest   // 任一服务发现型 client starter
)
```

**main.go**（同 `example/example.go`）：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-registry-zookeeper"
)

func main() { gs.Run() }
```

**conf/app.properties**（逐字取自 `example/conf/app.properties`）：

```properties
spring.app.name=registry-zookeeper-example

# 要注册进去的 ZooKeeper 集群。每个集群一个命名块；设置 servers 即激活该块
# （bean 名 = "zookeeper." + 块名）。
spring.registry.zookeeper.main.servers=127.0.0.1:2181
spring.registry.zookeeper.main.session-timeout=10s
spring.registry.zookeeper.main.base-path=/services

# 要发布的实例（与后端无关；更换注册后端是换一个 blank import，不是改配置）。
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

**消费侧**。本 starter 自带 ZooKeeper 发现后端（`discovery_zookeeper.go`）：
`zookeeper.<name>` bean 列举 `<base-path>/<service-name>` 的 children（每个 child 名即实例
id），`Get` 各 znode 数据，解码自描述的 `instanceValue` JSON，映射到
`discovery.Endpoint{Addr, Weight, Metadata}`，再用 `ChildrenW`/`GetW` watcher 保持快照新鲜
（watch 循环形态见 §2.3）。不存在任何 discovery 配置：后端**就是**该块的 bean，共享该块的连接
与 base-path —— 读写天然不会漂移。纯消费方只配置连接块（不设 `service-name`），不注册任何
实例；纯提供方从不引用该 bean，也不为它付出成本。

```properties
# 纯消费方：只配置连接块，注册保持关闭
spring.registry.zookeeper.main.servers=10.0.0.9:2181

spring.redis.demo.service-name=orders
spring.redis.demo.discovery=zookeeper.main   # 后端 bean 名
```

**验证**（同构于 `example/example.go` 与 `example/check.sh`）：

```bash
go run .                                   # 日志：registered "orders" at 127.0.0.1:8080
# example 自校验并打印：discovered endpoint=127.0.0.1:8080 weight=100 metadata=map[version:v1 zone:cn-north]
```

用 ZooKeeper shell 交互验证：

```bash
docker exec -it starter-registry-zookeeper zkCli.sh
[zk: ...] ls /services/orders                       # -> [orders-127.0.0.1:8080]
[zk: ...] get /services/orders/orders-127.0.0.1:8080
#   {"service_name":"orders","addr":"127.0.0.1:8080","weight":100,
#    "metadata":{"version":"v1","zone":"cn-north"}}
#   ... ctime/ephemeralOwner != 0 表明它是临时节点
```

---

## 2. 装配与时序

### 2.1 bean 生命周期时间线（`center.go` / `registrar.go` / 核心的 `starter.go`）

```
blank-import starter-registry-zookeeper
  ├─ 每个块 ${spring.registry.zookeeper.<name>}：Provide(newZkBackend)
  │    Name("zookeeper."+name).Export(As[discovery.Discovery], As[discovery.Registrar])
  │    条件：gs.OnProperty("spring.registry.zookeeper")              [center.go]
  └─ import starter-registry（核心，每进程恰一次）
       └─ gs.Provide(NewServer).Name("registryServer").Export(gs.As[gs.Server]())
            条件：gs.OnProperty("spring.registry.service-name")
gs.Run()
  ├─ conf.BindEach 遍历 spring.registry.zookeeper.* → 每块一份 ZookeeperConfig
  ├─ 每块 newZkBackend：zk.Connect(servers, session-timeout)；设置了凭证则 digest
  │    AddAuth + fail-fast 探活 Exists("/") —— 阻塞到会话连上，因此集群不可达在
  │    启动期失败，而不是等第一次 Register 才暴露                    [center.go]
  ├─ registryServer 经 []discovery.Registrar 切片注入收集所有后端的 registrar
  │    （跨所有后端 —— zookeeper、nacos……）
  ├─ Run：就绪前校验 service-name/addr 且 registrar ≥ 1 个
  ├─ 等待 <-sig.TriggerAndWait() —— 就绪闸门：所有其他 server 起来后才注册
  ├─ 逐中心 Register：ensureParents（按需创建持久目录）+ 在
  │    <base-path>/<service>/<id> Create 临时 znode                  [registrar.go]
  ├─ Run 阻塞在 <ctx.Done() —— 没有心跳 goroutine：存活即会话本身
  └─ SIGTERM：PreStop → 向每一个中心最先 Deregister（早于 pre-stop 延迟、早于任何
       server 停止），发现侧在存量请求排空期间就不再分发本实例；Stop 幂等
       兜底再注销一次（容忍 ErrNoNode）
```

### 2.2 会话与临时节点机制 —— 崩溃安全契约

- 注册即 `Create(path, val, zk.FlagEphemeral, ...)`——每实例一个临时 znode，归该块客户端会话
  所有（`registrar.go`）。
- 临时节点只活与会话一样长：进程未注销而死时，会话一过期 ZooKeeper 就删除该节点——自愈，
  无 reaper、无 TTL 心跳 goroutine、除 `session-timeout` 外无需任何配置。正确性从不依赖
  Deregister 执行。
- zk 库在瞬时网络断连时会自动重连并重建会话；若在一个 session timeout 内无法恢复，会话过期，
  **节点无声消失**——见 §4 演练与排障表"运行中实例消失"一行。本 starter 不会在会话丢失后
  重注册（未接线任何重连回调）。
- 重启替换：上一会话的临时节点可能短暂残留；Register 容忍 `ErrNodeExists`，先删后建，
  重启即刷新条目而非报错（`registrar.go`）。

### 2.3 WATCH 路径（消费侧）

自带的发现后端（`discovery_zookeeper.go`）其 watch 循环与 pool 消费 snapshot 的方式互为镜像：

1. `ChildrenW(basePath/service)` 在任意实例加入/离开时触发（包括会话过期引发的临时节点删除——
   那是 ZooKeeper 自己的 watch 事件，不是我们的代码）。
2. 后端列举 children 并 `Get` 各节点数据（一个 instance-id child 即一个实例）；`GetW` 覆盖
   原位数据改写（改权重）。
3. 每个事件产出一份全新的完整 `[]discovery.Endpoint` snapshot，写入后端按服务缓存的内部缓存。
   后端不做 diff，每份存下的快照都是权威。
4. discovery `Loader` 每次调用重读后端快照；loadbalance `Pool` 从该集合挑选。

### 2.4 DRAIN 路径 —— UpdateWeight(0)

在 `registryServer` bean（核心的 `starter.go`）上调用 `Server.UpdateWeight(ctx, 0)` 会向每一个
registrar 广播；zk 侧（`registrar.go`）先确认节点仍在（否则报 `update weight for
unregistered instance`），负权重映射为 1 但 0 原样放行，然后用 `conn.Set(path, val, -1)` 重写
载荷——**原位 set 数据，不是删除重建**。设计理由（源码注释）：临时节点的归属和 watcher 都
不受扰动——会话继续拥有该节点，`GetW` 消费方直接看到新值。权重 0 序列化为**省略**的
`weight` 字段（`json:"weight,omitempty"`），读侧重建为 0。消费方下一个 snapshot 携带
`Endpoint.Weight == 0` → `excludeDrained` 把它从所有负载均衡策略中剔除；仅当全部 endpoint
都被摘流时才回退用全集。`UpdateWeight(ctx, 100)` 恢复。

初始 Register 时负权重归一为 1；**0 原样透传即摘流信号**，配置 `weight=0` 即注册一个
已摘流实例。

---

## 3. 逐 key 行为参考

连接 key 按块绑定在 `${spring.registry.zookeeper.<name>}` 下（`config.go`）；实例 key 绑定在
`${spring.registry}` 下（`../starter-registry/config.go`）。不存在 discovery 配置：后端 bean
**就是**该块的 bean，名为 `zookeeper.<name>`（`center.go`）。

| key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|---------|
| `spring.registry.zookeeper.<n>.servers` | []string | —（必填） | 集群成员；**设置它即激活该块** | 到处未设：starter 沉默不生效；配错：启动在探活处失败（`registry-zookeeper: startup probe failed`） |
| `spring.registry.zookeeper.<n>.session-timeout` | duration | `10s` | zk 会话超时；同时限定启动探活时长、以及崩溃进程的临时节点残留多久 | ⚠ 过长拖慢崩溃摘除；过短则在 GC 停顿/瞬时分区下会话过期 → 无声注销 |
| `spring.registry.zookeeper.<n>.base-path` | string | `/services` | 持久父 znode；尾部 `/` 会被裁剪；服务目录按需创建 | 消费方必须列同一 path；不一致对提供方不可见 |
| `spring.registry.zookeeper.<n>.username` | string | `` | digest 认证，经 `AddAuth("digest", user:pass)` 生效；与 `password` 成对设置 | ⚠ 只设其一 → 认证报错 / ACL 拒绝写入 |
| `spring.registry.zookeeper.<n>.password` | string | `` | digest 密码（同上） | 同上 |
| `spring.registry.service-name` | string | `` | 逻辑服务名；成为 znode 目录名，也是发现侧解析的名字。**注册意图信号**：配了块而不设 → 合法的纯消费方 | 设了而 `addr` 为空：Run 返回 `registry: ${spring.registry.service-name} and ${spring.registry.addr} are required`——此时应用其他部分已起来 |
| `spring.registry.addr` | string | ``（注册时必填） | 广播的 `host:port`；绝不猜测 | 空：同上 Run 报错；格式错误不做校验（不同于 consul 的数字端口检查）——原样存储，消费方拨号才失败 |
| `spring.registry.id` | string | `` | 实例 id 覆写；空则派生 `<service-name>-<addr>`，重启替换同一 znode（`registrar.go`） | ⚠ 跨进程 id 重复 → 后注册者删除并顶掉先注册者的节点 |
| `spring.registry.weight` | int | `100` | 广播的 LB 权重；负权重写入时归一为 1 | 0 = 摘流，启动期即生效，与 `UpdateWeight(0)` 同语义 |
| `spring.registry.metadata.*` | map[string]string | 空 | 任意属性（zone、version……）存进 znode 载荷并透传到 discovery Metadata | — |

同一 `<name>` 的两个块会在容器里因 bean 重名而响亮报错；不同后端之间的块名永不冲突（bean 名
带后端类型，如 `zookeeper.main` 与 `nacos.main`）。

---

## 4. 验证与故障演练

zk 侧检查均可用容器内 `zkCli.sh`（见 §1）或任意 zk 客户端完成。

1. **注册 → 解析**：启动 example——它自己列举 `/services/orders` 并打印
   `discovered endpoint=... weight=... metadata=...`；`check.sh` 正是 grep 这个标记。zkCli 里
   `ls /services/orders` 见一个 child `orders-127.0.0.1:8080`；`get` 见 JSON 载荷含
   `"weight":100`，且 `ephemeralOwner != 0`（临时节点标记）。
2. **UpdateWeight(0) 摘流 + 恢复**：注入名为 `registryServer` 的 `gs.Server`，调用
   `server.UpdateWeight(ctx, 0)`，zkCli `get /services/orders/orders-127.0.0.1:8080`——载荷中
   **不再有 `weight` 字段**（0 时省略）；节点仍在（ephemeralOwner 不变——是 `Set` 而非重建）。
   消费 pool 的下一个 snapshot 将其剔除。`UpdateWeight(ctx, 100)` 恢复 `"weight":100`。在
   Run 注册前调用返回 `registry: instance not registered yet`。
3. **优雅退出注销**：example 校验完后自行 SIGTERM——PreStop 在 server 停止前注销；退出后
   `ls /services/orders` 立即为空。`Stop` 会再注销一次：幂等，容忍 `ErrNoNode`。
4. **崩溃 / 会话过期实例消失**：以 manual 模式启动 example（`go run . -manual`——server
   常驻），然后 `kill -9 <pid>` → 无注销执行；临时节点在会话过期时消失，约一个
   `session-timeout`（示例 10s）之后——没有 critical 标记阶段、没有 reaper 配置，不同于基于
   TTL 的注册中心。观察消失：`zkCli.sh ls -w /services/orders` 或轮询 `get` 直到 `NoNode`。
   消费方的 `ChildrenW` 在删除时触发，下一个 snapshot 丢弃该 endpoint。
5. **坏地址 fail-fast**：把块的 `servers` 设为 `127.0.0.1:9999` 启动 → 启动失败，报
   `registry-zookeeper: startup probe failed`——这是刻意设计，不让它拖到第一次 Register 才
   暴露。
6. **重启替换**：kill -9 后立刻重启（赶在旧会话过期前）——尽管旧临时节点仍在，Register 依旧
   成功（先删后建）；zkCli 里恰好一个 child。
7. **双写（两个块）**：加第二个块（`spring.registry.zookeeper.dr.servers=...`）——同一实例
   出现在两个集群下（一次发布扇出），启动日志为 `in 2 registry center(s)`。

本模块运行期日志带专属 tag `_app_registry_zookeeper`（`log.RegisterAppTag("registry_zookeeper", "")`），
经 `logger.<name>.tag=_app_registry_zookeeper` 单独调级；共享的注册生命周期日志带核心的 `_app_registry` tag。可观测性：注册与发现经 OTel 全局产出指标——未引入 `starter-otel` 时全部 no-op。`register`、`deregister`、`update_weight` 各有一个 client span 与一条 `registry.operation.duration`（标签 `system`/`operation`/`service`/`status`）；`registry.registration.attempts_total` 按 `reason` 与 `status` 计数；`registry.instance.registered` gauge 在发布中为 1、否则为 0——自愈失败会落在这里，而不只是出现在日志里。发现半边把每次后台缓存同步上报到 `discovery.sync_total`，并维持 `discovery.cache.age_seconds`（距上次确认新鲜的秒数），watch 死掉时表现为持续爬升，而不是静默返回陈旧地址。 `reason` 取 `initial`（初次发布）与 `self_heal`（后台重注册）。 会话丢失本身仍有日志（monitor 的 `zookeeper session lost ...`），且现在也会体现在上面的指标里。

---

## 5. 排障表

| 症状 | 原因 | 处置 |
|------|------|------|
| starter 不生效，没注册 | 到处都没有 `spring.registry.zookeeper.*` 块 | 配置一个命名块——块内 `servers` 即激活开关 |
| 启动失败 `startup probe failed` | 集群不可达 / servers 配错 | 启动 ZooKeeper、修该块的 `servers`；探活最多阻塞一个 `session-timeout` |
| 启动报 `service-name and addr are required` | 注册时任一实例 key 未设 | 都设上——注意这在 Run 期才报，其他 server 已起来 |
| 启动中止 "…… but no registry center is configured" | 设了 `service-name` 却没有任何连接块 | 至少加一个 `spring.registry.<backend>.<name>` 块 |
| 运行中实例消失 | 会话过期（长 GC 停顿、网络分区、session-timeout 过低）；zk 删掉了临时节点，且 starter 不会重注册 | 调大 `session-timeout`；重启进程；关注 zk 客户端日志里的重连空档 |
| `kill -9` 后节点残留很久 | 会话尚未过期——摘除最多要一个 `session-timeout` | 等待，或调低 `session-timeout`；不要加 reaper，机制本身就是 ephemeral-by-design |
| `UpdateWeight(0)` 后消费方仍来流量 | snapshot 未刷新（watch 未触发），或后端把省略的 weight 字段默认成 1 | 重读节点；确保后端把缺省 `weight` 解码为 0 而非 1 |
| 两个进程只剩一个节点 | 派生 id 冲突（同名同 addr）——后注册者删掉并顶掉前者 | 每实例设不同 `spring.registry.id` |
| `UpdateWeight` 报 `update weight for unregistered instance` | 节点已不在（会话过期）或 Run 前调用 | 重启重新注册，或等就绪后再调 |
| 写入报 ACL / 认证错误 | 集群要求 digest 认证，`username`/`password` 未设或只设一半 | 成对设置 |
| 重启报 `create ... ErrNodeExists` 类替换错误 | 替换与同 path 上的并发注册者撞车 | 每实例独立 id 即可规避 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 10（每块连接 5 + 实例 5） |
| 其中必填 | 每块 1 个（`servers`）+ 注册时 2 个（`service-name`、`addr`） |
| quickstart 前置外部依赖 | 1（ZooKeeper，docker） |
| "注意/坑" 条数 | 4（会话过期无声；weight 负值静默归一；重启替换；id 冲突） |

嫌疑清单（保留旧版条目，另记新发现）：

1. ~~`spring.registry.*` 实例 key 绑定在本 starter 内~~ 2026-09 已解决：`${spring.registry}`
   身份块现在位于共享的 starter-registry 核心（`RegistrationConfig`），每个后端 starter 恰好
   绑定一次。
2. 会话丢失后无重注册：zk 库会透明重连，但会话一旦过期临时节点已删，运行中的进程毫无感知——
   实例无声从发现侧消失，直到重启。候选修法是 SessionW/State 驱动的重注册循环。
3. ~~不带消费侧~~ 已解决：`discovery_zookeeper.go` 自带发现后端——自 2026-09 多注册中心命名块
   改造起，它就是该块的 bean（`zookeeper.<name>`），同时实现 `discovery.Registrar` 与
   `discovery.Discovery`；注册收进共享的 starter-registry 核心。
4. `addr` 写入时不校验 `host:port` 形态（consul 校验数字端口）；畸形值原样存储，只在消费方
   拨号时失败。
5. 启动校验发生在 Run 而非绑定期——空 `service-name` 要等应用其他部分都起来了才报（仍是
   失败时机偏晚的气味）。
