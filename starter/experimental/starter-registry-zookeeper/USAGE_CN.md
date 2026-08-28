# starter-registry-zookeeper 使用说明 — 参考手册

详细使用文档。概览见 [README](README_CN.md)。所有行为声明均已对照 starter 源码（`starter.go`、
`registrar.go`、`config.go`、`registrar_test.go`）、`cloud/discovery` 接缝（`cloud/discovery/discovery.go`、
`cloud/discovery/resolver.go`、`cloud/loadbalance/pool.go`）与可运行的 [example/](example/)
（`example/check.sh` 跑单测 + docker-compose ZooKeeper 端到端启动）核实。**ZooKeeper 自身语义（会话、
临时 znode、watcher、digest 认证）见 [ZooKeeper 文档](https://zookeeper.apache.org/doc/current/zookeeperProgrammers.html)**——
本文只讲 go-spring 的增量。

**激活条件**：只有设置了 `spring.registry.zookeeper.servers` 才会创建注册 server bean——该 key 即
开关（`starter.go:63-68`）。本 starter **只做注册侧**：把本实例以临时 znode 发布到 ZooKeeper，不携带
客户端发现后端（见 §1 消费侧说明）。不监听端口——导出 `gs.Server` 纯粹为了把注册接进应用生命周期
（`starter.go:84-93`）。

---

## 1. 完整工程示例

两侧角色：**提供方**（本 starter + 一个被服务的端点）与**消费方**（任意可通过 ZooKeeper 支撑的
`discovery.Discovery` 解析的服务发现型 client starter）。文件树：

```
demo/
├── go.mod
├── main.go
├── consumer_side/
│   └── discovery.go     # ZooKeeper 版 Discovery（见下方说明）
└── conf/
    └── app.properties
```

**前置依赖**（唯一外部依赖）：一个 ZooKeeper 节点——照抄 `example/docker-compose.yml`
（zookeeper:3.9，127.0.0.1:2181）：

```bash
docker compose -f example/docker-compose.yml up -d
# 就绪探活（四字命令；即 check.sh 的轮询方式，example/check.sh:52-58）：
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

# 要注册进去的 ZooKeeper 集群。设置 servers 即激活 starter。
spring.registry.zookeeper.servers=127.0.0.1:2181
spring.registry.zookeeper.session-timeout=10s
spring.registry.zookeeper.base-path=/services

# 要发布的实例（与后端无关；更换注册后端是换一个 blank import，不是改配置）。
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

**消费侧**。本 starter 不带 ZooKeeper 发现后端。既定接缝是 `cloud/discovery`：注册一个
ZooKeeper 版 `Discovery` 到某个名字下，之后任意 client starter 的 `discovery:` 字段都经它解析。
后端可以很小，因为载荷是自描述 JSON（`registrar.go:44-50`）：列举 `<base-path>/<service-name>`
的 children（每个 child 名即实例 id），`Get` 各 znode 数据，解码 `instanceValue`，映射到
`discovery.Endpoint{Addr, Weight, Metadata}`，再用 `ChildrenW`/`GetW` watcher 保持新鲜
（watch 循环形态见 §2.3）：

```go
// consumer_side/discovery.go — 启动时调用一次。
func registerZkDiscovery(name, servers, basePath string) error {
    b, err := zkdisc.New(servers, basePath) // ChildrenW/GetW → snapshot → WatchResult
    if err != nil { return err }
    discovery.RegisterDiscovery(name, b)    // cloud/discovery 接缝
    return nil
}
```

```properties
spring.redis.demo.service-name=orders
spring.redis.demo.discovery=zookeeper   # 与上面注册的名字一致
```

**验证**（同构于 `example/example.go:87-105 verifyOnce` 与 `example/check.sh`）：

```bash
go run .                                   # 日志：registered "orders" at 127.0.0.1:8080
# example 自校验并打印：registered node=orders-127.0.0.1:8080 value={...}
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

### 2.1 bean 生命周期时间线（均在 `starter.go` / `registrar.go`）

```
blank-import starter-registry-zookeeper
  └─ gs.Provide(NewServer).Name("registryServer").Export(gs.As[gs.Server]())
         condition: gs.OnProperty("spring.registry.zookeeper.servers")   [starter.go:63-68]
        │
gs.Run()
  ├─ 绑定：${spring.registry.zookeeper} → ZookeeperConfig（连接，config.go:23-42）
  ├─ 绑定：${spring.registry} → Server.Config 字段（实例，config.go:48-68）
  ├─ 构造：zk.Connect(servers, session-timeout)；设置了凭证则 digest AddAuth
  │    + fail-fast 探活 Exists("/") —— 阻塞到会话连上，因此集群不可达在
  │      启动期失败，而不是等第一次 Register 才暴露
  │    （registrar.go:64-90；设计理由注释 registrar.go:61-63、79-80）
  ├─ Run：先校验 service-name/addr，再发就绪信号                     [starter.go:98-101]
  ├─ 等待 <-sig.TriggerAndWait() —— 就绪闸门：所有其他 server 起来后才注册
  │                                                              [starter.go:110]
  ├─ Register：ensureParents（按需创建持久目录）+ 在
  │    <base-path>/<service>/<id> Create 临时 znode                [registrar.go:109-147]
  │    日志：registered %q at %s                                  [starter.go:117]
  ├─ Run 阻塞在 <ctx.Done() —— 没有心跳 goroutine：存活即会话本身
  └─ SIGTERM：PreStop → 最先 Deregister（早于 pre-stop 延迟、早于任何 server
       停止），发现侧在存量请求排空期间就不再分发本实例；Stop/StopContext 幂等
       兜底再注销一次（容忍 ErrNoNode）                [starter.go:126-142, registrar.go:181-187]
```

### 2.2 会话与临时节点机制 —— 崩溃安全契约

- 注册即 `Create(path, val, zk.FlagEphemeral, WorldACL(PermAll))`——每实例一个临时 znode，
  归本客户端会话所有（`registrar.go:135`、`registrar.go:85-89`）。
- 临时节点只活与会话一样长：进程未注销而死时，会话一过期 ZooKeeper 就删除该节点——自愈，
  无 reaper、无 TTL 心跳 goroutine、除 `session-timeout` 外无需任何配置（包文档
  `starter.go:27-30`；`config.go:29-32`）。正确性从不依赖 Deregister 执行
  （`registrar.go:31-34`）。
- zk 库在瞬时网络断连时会自动重连并重建会话；若在一个 session timeout 内无法恢复，会话过期，
  **节点无声消失**——见 §4.4 演练与排障表"运行中实例消失"一行。本 starter 不会在会话丢失后
  重注册（未接线任何重连回调）。
- 重启替换：上一会话的临时节点可能短暂残留；Register 容忍 `ErrNodeExists`，先删后建，
  重启即刷新条目而非报错（`registrar.go:133-145`）。

### 2.3 WATCH 路径（消费侧）

本 starter 自身无 watch；消费方走 `cloud/discovery` 接缝。ZooKeeper 后端的循环与 pool 消费
snapshot 的方式互为镜像：

1. `ChildrenW(basePath/service)` 在任意实例加入/离开时触发（包括会话过期引发的临时节点删除——
   那是 ZooKeeper 自己的 watch 事件，不是我们的代码）。
2. 后端列举 children 并 `Get` 各节点数据（一个 instance-id child 即一个实例，
   `registrar.go:101-104`）；`GetW` 覆盖原位数据改写（改权重）。
3. 每个事件产出一份全新的完整 `[]discovery.Endpoint` snapshot，作为一条 `WatchResult` 推送到
   `Discovery.Watch` 返回的 channel 上（`cloud/discovery/discovery.go:182-198`）。变更检测按
   snapshot 进行——后端不做 diff，消费方把每条结果都当权威。
4. `discovery.NewResolver` 消费该 channel 并换掉自己的 endpoint 集合
   （`cloud/discovery/resolver.go:57-108`）；loadbalance `Pool` 从该集合挑选。

### 2.4 DRAIN 路径 —— UpdateWeight(0)

`Server.UpdateWeight(ctx, 0)`（`starter.go:149-154`）→ registrar 先确认节点仍在（否则报
`update weight for unregistered instance`，`registrar.go:168-172`），负权重映射为 1 但 0 原样
放行（`registrar.go:155-157`），然后用 `conn.Set(path, val, -1)` 重写载荷——**原位 set 数据，
不是删除重建**（`registrar.go:173`）。设计理由（源码注释，`registrar.go:149-153`）：临时节点
的归属和 watcher 都不受扰动——会话继续拥有该节点，`GetW` 消费方直接看到新值。权重 0 序列化为
**省略**的 `weight` 字段（`json:"weight,omitempty"`，`registrar.go:48`），读侧重建为 0
（`registrar_test.go:41-51`）。消费方下一个 snapshot 携带 `Endpoint.Weight == 0` →
`excludeDrained` 把它从所有负载均衡策略中剔除；仅当全部 endpoint 都被摘流时才回退用全集
（`cloud/loadbalance/pool.go:93-100, 122-134`）。`UpdateWeight(ctx, 100)` 恢复。

初始 Register 时权重归一 `<=0 → 1`，"默认"绝不存成 0——0 保留给运行期摘流信号，只有
`UpdateWeight` 能到达（`registrar.go:113-117`）。

---

## 3. 逐 key 行为参考

已用 `grep -rhoE 'value:"[^"]+"' ... | sort -u` 核对——10 个 key + 结构体级的 `${spring.registry}`
字段绑定（`starter.go:89`）。连接 key 绑定在 `${spring.registry.zookeeper}` 下
（`config.go:23-42`）；实例 key 绑定在 `${spring.registry}` 下（`config.go:48-68`）。

| key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|---------|
| `spring.registry.zookeeper.servers` | []string | —（必填） | 集群成员；**设置它即激活 starter**（`starter.go:68`） | 未设：starter 沉默不生效；配错：启动在探活处失败（`registry-zookeeper: startup probe failed`，`registrar.go:81-84`） |
| `spring.registry.zookeeper.session-timeout` | duration | `10s` | zk 会话超时；同时限定启动探活时长、以及崩溃进程的临时节点残留多久（`config.go:29-32`，`registrar.go:68,81`） | ⚠ 过长拖慢崩溃摘除；过短则在 GC 停顿/瞬时分区下会话过期 → 无声注销 |
| `spring.registry.zookeeper.base-path` | string | `/services` | 持久父 znode；尾部 `/` 会被裁剪（`registrar.go:87`）；服务目录按需创建 | 消费方必须列同一 path；不一致对提供方不可见 |
| `spring.registry.zookeeper.username` | string | `` | digest 认证，经 `AddAuth("digest", user:pass)` 生效；与 `password` 成对设置（`registrar.go:73-78`） | ⚠ 只设其一 → 认证报错 / ACL 拒绝写入 |
| `spring.registry.zookeeper.password` | string | `` | digest 密码（同上） | 同上 |
| `spring.registry.service-name` | string | ``（必填） | 逻辑服务名；成为 znode 目录名，也是发现侧解析的名字 | 空：Run 返回 `registry: ${spring.registry.service-name} and ${spring.registry.addr} are required`（`starter.go:99-101`）——此时应用其他部分已起来 |
| `spring.registry.addr` | string | ``（必填） | 广播的 `host:port`；绝不猜测 | 空：同上 Run 报错；格式错误不做校验（不同于 consul 的数字端口检查）——原样存储，消费方拨号才失败 |
| `spring.registry.id` | string | `` | 实例 id 覆写；空则派生 `<service-name>-<addr>`，重启替换同一 znode（`registrar.go:94-99`） | ⚠ 跨进程 id 重复 → 后注册者删除并顶掉先注册者的节点（`registrar.go:139-144`） |
| `spring.registry.weight` | int | `0` | 广播的 LB 权重；`<=0` 写入时归一为 1（`registrar.go:116-118`） | 0 在这里**不**摘流（被归一）；摘流只有 `UpdateWeight(0)` |
| `spring.registry.metadata.*` | map[string]string | 空 | 任意属性（zone、version……）存进 znode 载荷并透传到 discovery Metadata | — |

---

## 4. 验证与故障演练

zk 侧检查均可用容器内 `zkCli.sh`（见 §1）或任意 zk 客户端完成。

1. **注册 → 解析**：启动 example——它自己列举 `/services/orders` 并打印
   `registered node=... value=...`（`example/example.go:87-105`）；`check.sh` 正是 grep 这个
   标记（`example/check.sh:76-79`）。zkCli 里 `ls /services/orders` 见一个 child
   `orders-127.0.0.1:8080`；`get` 见 JSON 载荷含 `"weight":100`，且 `ephemeralOwner != 0`
   （临时节点标记）。
2. **UpdateWeight(0) 摘流 + 恢复**：注入名为 `registryServer` 的 `gs.Server`，调用
   `server.UpdateWeight(ctx, 0)`，zkCli `get /services/orders/orders-127.0.0.1:8080`——载荷中
   **不再有 `weight` 字段**（0 时省略，`registrar_test.go:44-46`）；节点仍在
   （ephemeralOwner 不变——是 `Set` 而非重建）。消费 pool 的下一个 snapshot 将其剔除。
   `UpdateWeight(ctx, 100)` 恢复 `"weight":100`。在 Run 注册前调用返回
   `registry-zookeeper: instance not registered yet`（`starter.go:150-152`）。
3. **优雅退出注销**：example 校验完后自行 SIGTERM（`example/example.go:52-53`）——PreStop 在
   server 停止前注销；退出后 `ls /services/orders` 立即为空。`Stop` 会再注销一次：幂等，
   容忍 `ErrNoNode`（`registrar.go:181-187`）。
4. **崩溃 / 会话过期实例消失**：以 manual 模式启动 example（`go run . -manual`——server 常驻，
   `example/example.go:29,55-59`），然后 `kill -9 <pid>` → 无注销执行；临时节点在会话过期时
   消失，约一个 `session-timeout`（示例 10s）之后——没有 critical 标记阶段、没有 reaper 配置，
   不同于基于 TTL 的注册中心。观察消失：`zkCli.sh ls -w /services/orders` 或轮询 `get` 直到
   `NoNode`。消费方的 `ChildrenW` 在删除时触发，下一个 snapshot 丢弃该 endpoint。
5. **坏地址 fail-fast**：设 `servers=127.0.0.1:9999` 启动 → 启动失败，报
   `registry-zookeeper: build registrar ... startup probe failed`（`registrar.go:79-84`、
   `starter.go:77-80`）——这是刻意设计，不让它拖到第一次 Register 才暴露。
6. **重启替换**：kill -9 后立刻重启（赶在旧会话过期前）——尽管旧临时节点仍在，Register 依旧
   成功（先删后建，`registrar.go:133-145`）；zkCli 里恰好一个 child。

运行期日志均带 `log.TagAppDef`（`logger.appdef`）：`creating zookeeper registrar servers=...`
（Debug）、`registering service=...`（Debug）、`registered %q at %s`（Info）、
`deregister %q`（Warn）。自身不产出 metrics/trace/健康 indicator——会话无声过期时**什么日志都
没有**。

---

## 5. 排障表

| 症状 | 原因 | 处置 |
|------|------|------|
| starter 不生效，没注册 | `spring.registry.zookeeper.servers` 未设 | 设置它——该 key 即激活开关 |
| 启动失败 `startup probe failed` | 集群不可达 / servers 配错 | 启动 ZooKeeper、修 `servers`；探活最多阻塞一个 `session-timeout` |
| 启动报 `service-name and addr are required` | 任一实例 key 未设 | 都设上——注意这在 Run 期才报，其他 server 已起来（`starter.go:99-101`） |
| 运行中实例消失 | 会话过期（长 GC 停顿、网络分区、session-timeout 过低）；zk 删掉了临时节点，且 starter 不会重注册 | 调大 `session-timeout`；重启进程；关注 zk 客户端日志里的重连空档 |
| `kill -9` 后节点残留很久 | 会话尚未过期——摘除最多要一个 `session-timeout` | 等待，或调低 `session-timeout`；不要加 reaper，机制本身就是 ephemeral-by-design |
| `UpdateWeight(0)` 后消费方仍来流量 | snapshot 未刷新（watch 未触发），或后端把省略的 weight 字段默认成 1 | 重读节点；确保后端把缺省 `weight` 解码为 0 而非 1（`registrar_test.go:48-50`） |
| 两个进程只剩一个节点 | 派生 id 冲突（同名同 addr）——后注册者删掉并顶掉前者（`registrar.go:139-144`） | 每实例设不同 `spring.registry.id` |
| `UpdateWeight` 报 `update weight for unregistered instance` | 节点已不在（会话过期）或 Run 前调用 | 重启重新注册，或等就绪后再调 |
| 写入报 ACL / 认证错误 | 集群要求 digest 认证，`username`/`password` 未设或只设一半 | 成对设置 |
| 重启报 `create ... ErrNodeExists` 类替换错误 | 替换与同 path 上的并发注册者撞车 | 每实例独立 id 即可规避 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 10（连接 5 + 实例 5） |
| 其中必填 | 3（`servers`、`service-name`、`addr`） |
| quickstart 前置外部依赖 | 1（ZooKeeper，docker） |
| "注意/坑" 条数 | 4（会话过期无声；weight 归一化；重启替换；id 冲突） |

嫌疑清单（保留旧版条目，另记新发现）：

1. `spring.registry.*` 实例 key 是各 registry starter 的共享词汇，却绑定在本 starter 内——
   第二个 registry starter 需要再实现一份 `RegistrationConfig`（原有条目）。
2. 实验性：位于 `experimental/`（未审核标记，非质量分级）（原有条目）。
3. 会话丢失后无重注册：zk 库会透明重连，但会话一旦过期临时节点已删，运行中的进程毫无感知——
   实例无声从发现侧消失，直到重启。候选修法是 SessionW/State 驱动的重注册循环。
4. 不带消费侧：每个用户都要手写 ZooKeeper `discovery.Discovery`（ChildrenW/GetW 循环，
   §2.3）——与 consul 同样的缺口，候选做 discovery 兄弟包。
5. `addr` 写入时不校验 `host:port` 形态（consul 校验数字端口）；畸形值原样存储，只在消费方
   拨号时失败。
6. 启动校验发生在 Run 而非绑定期——空 `service-name` 要等应用其他部分都起来了才报（与 consul
   一致，仍是失败时机偏晚的气味）。
