# starter-registry-k8s 使用说明 — 参考

详细用法参考。总览见 [README_CN.md](README_CN.md)。文中所有行为断言都对过 starter 源码
（`starter.go`、`config.go`、`dns.go`、`endpointslice.go`）与可运行的 [example/](example/)
（单测 + `check.sh` 启动冒烟；完整端到端需要真集群）。
**Kubernetes 自身的语义（Service、EndpointSlice、DNS、就绪度）以
[Kubernetes 文档](https://kubernetes.io/docs/concepts/services-networking/)为准**——下文
只讲 go-spring 的增量。

**激活方式**：一个 `spring.registry.k8s.<name>` 命名块变一个名为 `k8s.<name>` 的后端
bean（`starter.go:39-47`）。本 starter **只做发现**：集群内平台已经把每个 Pod 注册在
Service 之后，因此它不提供 registrar，也不读家族级的 `${spring.registry.service-name}` /
`.addr`（`config.go:52-60`）。

**两种模式**（`Config.Mode`，`config.go:47-50`）：

| 模式 | 机制 | 新鲜度 | 需要 | 元数据 |
|------|------|--------|------|--------|
| `dns`（默认） | headless Service 的 SRV/A 查询，按 `refresh-interval` 缓存 | `refresh-interval`（默认 10s） | 仅集群 DNS，无 RBAC | 只有 SRV weight |
| `endpointslice` | client-go informer 监听 EndpointSlice | 实时（watch 事件） | client-go + get/list/watch RBAC | zone、ready 状态 |

---

## 1. 完整工程示例

两侧：**provider**（任意跑在 headless Service 后的 Deployment——Kubernetes 自己就是注册方）与
**consumer**（通过本 starter 解析该 Service 的 Go-Spring 应用）。文件树：

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── deploy/
    ├── demo-service.yaml    # provider：Deployment + headless Service "demo"
    ├── consumer.yaml        # consumer Deployment
    └── rbac.yaml            # 仅 endpointslice 模式需要
```

**go.mod**：

```
require (
    go-spring.org/spring                v1.3.x
    go-spring.org/starter-registry-k8s  latest
    go-spring.org/starter-redigo        latest   // 消费方客户端（任何支持发现的 starter 均可）
)
```

**main.go**：

```go
package main

import (
    _ "demo/conf"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-registry-k8s"
    _ "go-spring.org/starter-redigo"
)

func main() { gs.Run() }
```

**conf/app.properties** —— 一个 k8s backend，加一个经由它取地址的 Redis 类客户端：

```properties
spring.app.name=demo-consumer

# spring.registry.k8s 下的块 "main" 声明了 bean "k8s.main"，
# 客户端 `discovery:` 字段引用的就是这个名字。
spring.registry.k8s.main.mode=dns                 # 或：endpointslice
spring.registry.k8s.main.namespace=default
# dns SRV 模式：port-name "grpc" 查询 _grpc._tcp.demo.default.svc.cluster.local
spring.registry.k8s.main.port-name=grpc
spring.registry.k8s.main.cluster-domain=cluster.local
spring.registry.k8s.main.refresh-interval=5s      # dns 重新解析周期
# endpointslice 模式附加项：
#   spring.registry.k8s.main.kubeconfig=           # 留空 = in-cluster ServiceAccount
#   spring.registry.k8s.main.resync-period=0       # 0 = 仅事件驱动

# 消费端：地址来自上述 backend 的 redigo 客户端。
spring.redis.demo.service-name=demo
spring.redis.demo.discovery=k8s.main
spring.redis.demo.conn-max-lifetime=30s           # 短连接跟随端点变动
```

（example/ 应用演示的是直连形式——注入 `k8s.main` 后端 bean 后 `Resolve("demo")`——这正是每个
客户端 starter 底下做的事；见 `example/example.go:77-100`。）

**Provider 清单**（照搬 `example/deploy/demo-service.yaml`）：2 副本 Deployment 带 readiness
探针，以及一个 `clusterIP: None` 且带命名端口 `grpc` 的 Service。dns 模式**必须** headless——
普通 Service 只会解析出 ClusterIP，而不是逐 Pod 记录。

**验证**（集群内，同 `example/README.md` + `check.sh`）：

```bash
kubectl apply -f deploy/demo-service.yaml
kubectl apply -f deploy/rbac.yaml                  # 仅 endpointslice 模式
kubectl apply -f deploy/consumer.yaml
kubectl logs deploy/registry-k8s-example -f
# 预期：每个 ready Pod 一行 "endpoint addr=10.x.x.x:80 healthy=true zone="
```

集群外开发：设 `kubeconfig=/home/me/.kube/config` 且 `mode=endpointslice`；dns 模式需要集群
DNS，集群外解析会以 lookup 错误失败（example 把它当警告处理并干净退出——`example/example.go:97-99`）。

---

## 2. 装配与时序

生命周期（全在 `starter.go`）：

1. **Bean 注册阶段**：`gs.Module(gs.OnProperty("spring.registry.k8s"), ...)` 对每个块跑
   `conf.BindEach`，一块提供一个命名 bean——`"k8s." + <name>`——并挂 `Destroy(destroyBackendBean)`
   （`starter.go:39-47`）。声明发生在任一 client starter 的 bean 构造函数之前，所以客户端可以在
   自己的配置里直接引用 `k8s.<name>`，不必与声明赛跑。
2. **构造推迟到注入时**（`newBackendBean`，`starter.go:52-59`）：只有被引用时才构建后端。因此非法
   mode 或连不上的 API server 会在第一次注入时报错，而不是在启动时序里。
3. 重复的 bean 名由容器拒绝——两个块永远不会静默覆盖对方。
4. 关闭时调 bean 析构函数，仅对实现 `io.Closer` 的后端调 `Close`（`starter.go:64-69`）：dns 后端
   无资源，informer 后端停掉其 watch。
5. 日志：`declared k8s discovery backend bean name=%s mode=%s namespace=%s` 与
   `creating k8s discovery backend mode=%s namespace=%s`（`starter.go:57, 66`）；informer 警告
   （`endpointslice.go:209, 218`）。每一行都带本 starter 自己的 tag `_app_registry_k8s`，因此可用
   `logger.<name>.tag=_app_registry_k8s` 与主日志分开调级。

### 2.1 新鲜度 —— endpointslice 模式（重点）

没有公开的 `Watch`：新鲜度是后端内部的事，每个 Service 缓存一份快照（`esEntry`）。
`Resolve`（`endpointslice.go:143-158`）：

1. 某名字的第一次 `Resolve` 付出**种子列表**代价——一次按标签选择器
   `kubernetes.io/service-name=<name>` 过滤的 `EndpointSlices.List`（`endpointslice.go:135-137, 161-...`）
   ——然后启动一个作用域受限的 informer goroutine。
2. informer factory 以 namespace + 同一标签选择器构建，并带 `ResyncPeriod`
   （`endpointslice.go:181-191`）。
3. handler 只**发信号**（非阻塞写入容量 1 的 channel）；对该 entry 的所有写由那个 goroutine 独占
   （`endpointslice.go:193-209`）。
4. `WaitForCacheSync` 决定快照是否可用；失败时后端打一条警告，之后一直提供种子/陈旧快照，而不是让
   每次后续调用都报错（`endpointslice.go:213-217`）。
5. 每次 add/update/delete 都从 lister 缓存重算：slices → `slicesToEndpoints`（端口由 `pickPort`
   决定，`Healthy = Conditions.Ready == nil || *Ready`，`zone` 进 Metadata）→ 按地址排序
   （`endpointslice.go:228-238, 266-291`）。
6. 之后的 `Resolve` 读缓存集合，并按本次调用的 scheme 过滤（`discovery.FilterByScheme`）；因此
   `e.eps` 存的是**未过滤**的全集（`endpointslice.go:66-71`）。
7. `Close` 停掉所有已跟踪的 watch——为「消费者忘了放手」兜底的关闭保险
   （`endpointslice.go:251-262`，接在 bean 析构上）。

下游，客户端 starter 从后端构建 `discovery.Resolver`
（`discovery.NewResolver(ctx, backend, serviceName, opts...)`，`cloud/discovery/discovery.go:227`）——
一个普通的 `func() ([]Endpoint, error)`（`cloud/discovery/discovery.go:209`），由 loadbalance
`Pool` 每次 pick 时调用。这就是把端点“推”进客户端连接池的那条路。

### 2.2 新鲜度 —— dns 模式

没有 push，快照靠 TTL 缓存：仅当缓存为空或超过 `RefreshInterval` 时 `Resolve` 才重新拉取
（`dns.go:90-118`）。**刷新失败时继续提供陈旧快照**，只有在还没有任何缓存时才报错
（`dns.go:106-115`）。SRV 记录带逐端点 weight（`dns.go:130-146`）；A 记录把每个 IP 配上配置的
`port` 并标记为健康（`dns.go:150-165`）——headless Service 默认只为就绪地址发布记录。两条路径
都按地址排序以得到稳定快照（`dns.go:169-171`）。

### 2.3 摘除 / drain 路径

Kubernetes 没有 `UpdateWeight`；摘流是平台的事，走的是同一份快照：

- **缩容 / Pod 删除** → EndpointSlice 缩小 → informer delete 事件 → 重算 → 地址从快照中消失 →
  客户端池不再挑到它。
- **就绪度翻假**（探针失败）→ 端点仍在 slice 里，但 `Conditions.Ready` 变假 → `Healthy=false` →
  在 pick 时被 `discovery.Allows` 排除（`endpointslice.go:276`，`cloud/discovery/discovery.go:65-77`）。
  dns 模式则是记录直接消失（headless DNS 只发布就绪地址，`dns.go:82-89`），滞后一个
  `refresh-interval`。
- Weight 语义：只有 SRV 记录带 weight（`dns.go:140`）；那里的 `Weight == 0` 受池的软摘流过滤影响
  （`cloud/loadbalance/pool.go:93-134`）。

---

## 3. 逐 key 行为参考

所有 key 都在 `${spring.registry.k8s.<块名>}` 下；块名即 bean 名 `k8s.<块名>`。对过
`config.go:63-97`。

| key | 类型 | 默认 | 行为 | 配错后果 |
|-----|------|------|------|----------|
| `mode` | string | `dns` | `dns` 或 `endpointslice` | 其他值：启动错误 `invalid mode`（`config.go:113-114`） |
| `namespace` | string | `default` | 目标 Service 的命名空间 | 解析为空（空端点集，不是错误） |
| `port-name` | string | `` | dns：SRV 查询 `_<port-name>._tcp.<fqdn>`；endpointslice：按名匹配 slice 端口 | 名字写错：dns 模式返回 lookup 错误 / 该 slice 被跳过（`endpointslice.go:297-305`） |
| `port` | int | `0` | `port-name` 为空时的数字端口 | ⚠ dns 模式下 `port-name` 与 `port` **都**为空则启动失败（`config.go:105-108`）；endpointslice 模式下 `0` 退回该 slice 的唯一端口（`endpointslice.go:306-309`） |
| `cluster-domain` | string | `cluster.local` | dns 模式的 FQDN 后缀 | 域写错 → 每次解析 NXDOMAIN；endpointslice 模式**忽略** |
| `refresh-interval` | duration | `10s` | dns 快照 TTL | `0` 等同 `10s`（`dns.go:91-94`）；endpointslice 模式忽略 |
| `kubeconfig` | string | `` | endpointslice 鉴权；空 = in-cluster ServiceAccount | 集群外留空 → 报 `in-cluster config (set kubeconfig ...)`（`endpointslice.go:127-130`）；dns 模式忽略 |
| `resync-period` | duration | `0` | informer 周期重同步 | `0` = 仅事件驱动；dns 模式忽略 |

耦合：dns 模式需要 **headless** Service（ClusterIP Service 只会解析成一个虚拟地址）；
endpointslice 模式需要 `deploy/rbac.yaml` 的 RBAC（对 `discovery.k8s.io/endpointslices` 的
get/list/watch）。客户端的 `discovery:` 字段要填**完整 bean 名**——`k8s.<块名>`——不是裸块名。

**不要**为了注册而设 `${spring.registry.service-name}` / `.addr`：本后端不提供 registrar，且在完全
没有 registrar 后端时，registry 核心会以 `no registry center is configured` 启动失败，而不是静默
什么都不发布。

---

## 4. 验证与故障演练

前置：有可用开发集群的 `kubectl`；provider 按 §1 应用。

1. **端点出现**：`kubectl logs deploy/registry-k8s-example`——每个 ready Pod 一行
   `endpoint addr=...`；集群感知 zone 时 endpointslice 模式还会打印 `zone=`。
2. **扩缩容**：`kubectl scale deploy/demo --replicas=4` → 数秒内 endpointslice 模式推出 4 个地址；
   `--replicas=1` → 被移除的地址从下一份快照消失。
3. **实例丢失（TTL 类比）**：`kubectl delete pod -l app=demo --grace-period=0`——该 Pod 的端点由
   kube-controller-manager 从 EndpointSlice 摘掉，informer 触发，池不再挑到该地址。dns 模式下同一
   变更要到 `refresh-interval` 之后才显现。
4. **未就绪摘流（weight=0 类比）**：弄坏 readiness 探针
   （`kubectl patch deploy/demo -p '{"spec":{"template":{"spec":{"containers":[{"name":"demo","readinessProbe":{"tcpSocket":{"port":9999}}}]}}}}'`）
   → 滚动中的 Pod 变未就绪 → endpointslice 模式显示 `healthy=false`（被 Allows 排除）；dns 模式直接
   丢掉记录。
5. **watch 卫生**：停掉消费者——bean 析构会调 `Close`，即使消费者漏掉了它的快照也会停掉所有 informer
   （`starter.go:64-69`，`endpointslice.go:251-262`）。
6. **启动快速失败（endpointslice）**：集群外不配 `kubeconfig` → 首次注入时给出指明修法的错误。
   单测覆盖：`dns_test.go`、`endpointslice_test.go`（fake resolver + fake clientset 演练解析、
   端口选择、ready/zone 元数据、缓存刷新——这正是 `check.sh` 真正断言的）。

运行期日志都带 tag `_app_registry_k8s`（`log.RegisterAppTag("registry_k8s", "")`，`starter.go:34`），
经 `logger.<name>.tag=_app_registry_k8s` 单独调级。本 starter 的读侧只产出指标、不产出 span
（`discovery.sync_total` / `discovery.cache.age_seconds`，定义在 `cloud/discovery/observe.go`）；
客户端通过自己的池观测。

---

## 5. 排障表

| 症状 | 原因 | 修法 |
|------|------|------|
| `resolve "demo" failed ... lookup demo...: no such host` | 不在集群内 / `cluster-domain` 写错 / Service 非 headless | 集群内运行、修 domain、设 `clusterIP: None` |
| `resolve` 返回空且无错 | `namespace` 写错，或没有就绪端点 | 查 `kubectl get endpointslices -l kubernetes.io/service-name=demo` |
| 启动错误 `dns mode requires port-name (SRV) or port` | `port-name` 与 `port` 都没设 | 设其一（`config.go:105-108`） |
| 错误 `in-cluster config` | endpointslice 模式在集群外 | 设 `kubeconfig`（`endpointslice.go:127-130`） |
| 日志出现 `informer handler ... failed` / `cache sync ... failed` | RBAC 被拒，或 watch 启动时 API server 不可达 | 应用 `deploy/rbac.yaml`、查连通性；期间后端继续提供种子快照（`endpointslice.go:206-217`） |
| dns 模式下扩缩容后端点不更新 | 快照 TTL 滞后 | 降低 `refresh-interval`，或改用 `endpointslice` |
| 多端口且未命名端口的 slice 解析不出东西 | `pickPort` 无法选择（`endpointslice.go:306-309`） | 设 `port-name` 或 `port` |
| `:0` 地址从不出现 | 无端口的 slice 被跳过，而不是以 0 输出 | 设与 Service 端口名一致的 `port-name` |

---

## 6. 设计体检表

| 指标 | 值 |
|------|-----|
| 配置 key 数 | 8 |
| 必填（按模式） | 1（dns 模式的 `port-name` **或** `port`） |
| 快速上手外部依赖 | 1（一个 Kubernetes 集群） |
| “注意/坑”数 | 4 |

可疑清单：

- dns 与 endpointslice 的摘流保真度不同（未就绪 → Healthy=false vs 记录消失）——DNS 固有，
  已记录，不可修。
- dns 刷新失败会静默提供陈旧快照，只有首次拉取才报错（`dns.go:106-115`）。这是有意的（一次抖动不能
  清空池），但代价是：一个永久坏掉的 DNS 配置在种子之后看起来是健康的。
- 只配 `port` 的 endpointslice 配置隐含依赖「slice 恰好只有一个端口」
  （`endpointslice.go:306-309`）——多端口 Service 会静默配错。
