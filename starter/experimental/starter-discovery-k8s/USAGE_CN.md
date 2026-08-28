# starter-discovery-k8s 使用说明 — 参考

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`dns.go`、`endpointslice.go`）与可运行的 [example/](example/)
（`check.sh` = 单测 + 启动冒烟；完整端到端需真实集群）核实。**Kubernetes 自身语义
（Service、EndpointSlice、DNS、readiness）见 [Kubernetes 官方文档](https://kubernetes.io/zh-cn/docs/concepts/services-networking/)**
——下文只写 go-spring 的增量。

**激活方式**：`${spring.discovery.k8s}` 下每个条目注册一个 Kubernetes discovery backend
（`starter.go:41`）。本 starter **只有 discovery、没有 registrar**：集群内平台已为 Service 背后的
每个 Pod 完成注册，消费侧只读、Kubernetes 只写（`config.go:20-25`）。

**两种模式**（`Config.Mode`，`config.go:47-50`）：

| 模式 | 机制 | 时效 | 依赖 | 元数据 |
|------|------|------|------|--------|
| `dns`（默认） | headless Service SRV/A 查询 + 轮询 | DNS TTL + `refresh-interval` | 仅集群 DNS，无 RBAC | 仅 SRV weight |
| `endpointslice` | client-go informer 监听 EndpointSlice | 实时（watch 事件） | client-go + get/list/watch RBAC | zone、ready 状态 |

---

## 1. 完整工程示例

两侧：**provider**（任意 Deployment + headless Service——Kubernetes 本身就是注册中心）与
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
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-discovery-k8s latest
    go-spring.org/starter-redigo        latest   // 消费端 client（任何支持 discovery 的 starter 均可）
)
```

**main.go**：

```go
package main

import (
    _ "demo/conf"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-discovery-k8s"
    _ "go-spring.org/starter-redigo"
)

func main() { gs.Run() }
```

**conf/app.properties** —— 一个 k8s backend，加一个经由它取地址的 Redis 类客户端：

```properties
spring.app.name=demo-consumer

# discovery backend。前缀后的 key（"k8s"）即客户端 `discovery:` 字段引用的名字。
spring.discovery.k8s.k8s.mode=dns                 # 或：endpointslice
spring.discovery.k8s.k8s.namespace=default
# dns SRV 模式：port-name "grpc" 查询 _grpc._tcp.demo.default.svc.cluster.local
spring.discovery.k8s.k8s.port-name=grpc
spring.discovery.k8s.k8s.cluster-domain=cluster.local
spring.discovery.k8s.k8s.refresh-interval=5s      # dns 重新解析周期
# endpointslice 模式附加项：
#   spring.discovery.k8s.k8s.kubeconfig=          # 留空 = in-cluster ServiceAccount
#   spring.discovery.k8s.k8s.resync-period=0      # 0 = 仅事件驱动

# 消费端：地址来自上述 backend 的 redigo 客户端。
spring.redis.demo.service-name=demo
spring.redis.demo.discovery=k8s
spring.redis.demo.conn-max-lifetime=30s           # 短连接周期跟随端点变化
```

（example/ 应用演示的是直接形式——`discovery.GetDiscovery("k8s")` 后 `Resolve("demo")`，
即所有 client starter 内部做的事；见 `example/example.go:71-97`。）

**Provider 清单**（照搬 `example/deploy/demo-service.yaml`）：2 副本 Deployment 带 readiness
探针，加 `clusterIP: None` 的 Service、命名端口 `grpc`。dns 模式必须是 headless——普通
Service 解析到 ClusterIP，而非每 Pod 记录。

**验证**（集群内，同 `example/README.md` + `check.sh`）：

```bash
kubectl apply -f deploy/demo-service.yaml
kubectl apply -f deploy/rbac.yaml                  # 仅 endpointslice 模式
kubectl apply -f deploy/consumer.yaml
kubectl logs deploy/discovery-k8s-example -f
# 预期：每个 ready Pod 一行 "endpoint addr=10.x.x.x:80 healthy=true zone="
```

集群外开发：设 `kubeconfig=/home/me/.kube/config` 并 `mode=endpointslice`；dns 模式需要集群
DNS，集群外 resolve 报 lookup 错误（example 将其视为 warning 后干净退出——`example/example.go:83-86`）。

---

## 2. 装配与时序

生命周期（均在 `starter.go`）：

1. **bean 注册阶段**：`gs.Module(gs.OnProperty("spring.discovery.k8s"), ...)` 对每个条目跑
   `conf.BindEach`；backend 立即构建并写入全局 `discovery` 注册表，**早于任何 client starter
   的 bean 构造函数**（`starter.go:31-40`）。立即构建意味着缺失 ServiceAccount 或坏 kubeconfig
   在启动期失败（`endpointslice.go:71-86`），而非首次 resolve 时。
2. 重名 backend 跳过并打日志，不 panic（`starter.go:46-48`）。
3. 单独 Provide 一个 `manager` bean，只为容器停机时关闭所有 informer backend
   （`starter.go:61, 69-86`）；dns backend 无资源可关。
4. 启动日志：`registered k8s discovery backend name=%s mode=%s`，tag 为 `log.TagAppDef`
   （`starter.go:55`）。

### 2.1 WATCH 路径 —— endpointslice 模式（重点）

`Watch(ctx, name)`（`endpointslice.go:133-221`）：

1. 创建 shared-informer factory，作用域限定 namespace 与 label selector
   `kubernetes.io/service-name=<name>`（`endpointslice.go:41, 105-107, 135-142`）。
2. informer handler 只**发信号**（向 cap-1 channel 非阻塞 enqueue）；唯一 goroutine 独占结果
   channel 的写与关闭——不存在 send-after-close 竞态（`endpointslice.go:147-160`）。
3. `WaitForCacheSync` 把关初始 list；失败则返回错误、watch 不启动
   （`endpointslice.go:167-170`）。
4. 从 informer cache 计算**快照**：list slices → `slicesToEndpoints`（`pickPort` 选端口、
   `Healthy = Conditions.Ready == nil || *Ready`、`zone` 进 Metadata）→ `FilterByScheme` →
   按地址排序（`endpointslice.go:175-195, 247-294`）。
5. **变化检测 / 去重**：地址集折叠为字符串 key，key 不变则跳过——cache-sync 会每个对象发一次
   Add，不去重会在真实变化前堆积陈旧快照（`endpointslice.go:184-190`，与 dns 的 `addrKey`
   对齐，`dns.go:179-186`）。
6. 种子快照在写 goroutine 启动**之前**推送，首个 channel 结果即 watch 时刻的状态
   （`endpointslice.go:198-199`）；此后每个 add/update/delete 触发一次重算。ctx 取消或
   `Close()` 时 channel 关闭（`endpointslice.go:205-219`）。

下游由 `discovery.Resolver` 把 channel 变成 `Endpoints()` 快照，loadbalance `Pool` 每次
`Pick` 读取——这就是向客户端连接池的"端点推送"。

### 2.2 WATCH 路径 —— dns 模式

DNS 无推送，`Watch` 轮询：初始 `Resolve` 作种子，之后按 `refresh-interval` 重解析；`addrKey`
变化才推送，结果未变与瞬时查询错误直接跳过（`dns.go:129-169`）。SRV 记录携带每端点 weight
（`dns.go:87-103`）；A 记录将每个 IP 与配置的 `port` 配对（`dns.go:107-122`）。

### 2.3 摘除 / drain 路径

Kubernetes 没有 `UpdateWeight`；摘除 Pod 是平台的职责，走同一条 watch：

- **缩容 / 删 Pod** → EndpointSlice 收缩 → informer delete 事件 → 重算 → 地址从快照消失 →
  客户端池不再选中它。
- **readiness 翻 false**（探针失败）→ 端点仍在 slice 但 `Conditions.Ready=false` →
  `Healthy=false` → pick 时被 `discovery.Eligible` 排除（`endpointslice.go:257`、
  `cloud/discovery/discovery.go:65-77`）。dns 模式则表现为记录消失（headless DNS 只发布
  ready 地址，`dns.go:67-70`），滞后 DNS TTL + refresh-interval。
- weight 语义：仅 SRV 记录带 weight（`dns.go:97`）；该处 `Weight == 0` 同样进入 pool 的
  soft-drain 过滤（`cloud/loadbalance/pool.go:93-134`）。

---

## 3. 逐 key 行为参考

所有 key 位于 `${spring.discovery.k8s.<backend-name>}`。对照 `config.go:58-94` 核实。

| key | 类型 | 默认值 | 行为 | 配错后果 |
|-----|------|--------|------|----------|
| `mode` | string | `dns` | `dns` 或 `endpointslice` | 其他值：启动报 `invalid mode`（`config.go:108-110`） |
| `namespace` | string | `default` | 目标 Service 所在 namespace | resolve/watch 找不到任何东西（空端点集，不报错） |
| `port-name` | string | 空 | dns：SRV 查询 `_<port-name>._tcp.<fqdn>`；endpointslice：按名匹配 slice 端口 | 名字写错：dns 模式报 lookup 错误 / slice 被跳过（`endpointslice.go:279-285`） |
| `port` | int | `0` | `port-name` 为空时的数字端口 | ⚠ dns 模式 `port-name` 与 `port` **均空**则启动失败（`config.go:100-103`）；endpointslice 下 `0` 回退 slice 唯一端口（`endpointslice.go:290-293`） |
| `cluster-domain` | string | `cluster.local` | dns 模式 FQDN 后缀 | 域名错 → 每次 resolve NXDOMAIN；endpointslice 模式**忽略** |
| `refresh-interval` | duration | `10s` | dns watch 轮询周期 | `0` 等价 `10s`（`dns.go:131-133`）；endpointslice 模式忽略 |
| `kubeconfig` | string | 空 | endpointslice 认证；空 = in-cluster ServiceAccount | 集群外留空 → 启动报 `in-cluster config (set kubeconfig ...)`（`endpointslice.go:97-100`）；dns 模式忽略 |
| `resync-period` | duration | `0` | informer 周期 resync | `0` = 仅事件驱动；dns 模式忽略 |

耦合项：dns 模式需要 **headless** Service（ClusterIP Service 只解析出一个虚拟地址）；
endpointslice 模式需要 `deploy/rbac.yaml` 的 RBAC（对 `discovery.k8s.io/endpointslices` 的
get/list/watch）。backend 名（map key）必须等于客户端 `discovery:` 字段——客户端默认值是
`default` 而非 `k8s`。

---

## 4. 验证与故障演练

前置：`kubectl` + 一个开发集群；按 §1 部署 provider。

1. **端点出现**：`kubectl logs deploy/discovery-k8s-example` —— 每个 ready Pod 一行
   `endpoint addr=...`；集群有 zone 时 endpointslice 模式还打印 `zone=`。
2. **扩缩容**：`kubectl scale deploy/demo --replicas=4` → 数秒内 endpointslice 模式推送 4 个
   地址；`--replicas=1` → 被删地址从下一快照消失。
3. **实例丢失（TTL 类比）**：`kubectl delete pod -l app=demo --grace-period=0` —— 端点由
   kube-controller-manager 移出 EndpointSlice，informer 触发，pool 停止选中该地址。dns 模式
   要等 DNS TTL + `refresh-interval` 才可见。
4. **not-ready 摘流（weight=0 类比）**：破坏 readiness 探针
   （`kubectl patch deploy/demo -p '{"spec":{"template":{"spec":{"containers":[{"name":"demo","readinessProbe":{"tcpSocket":{"port":9999}}}]}}}}'`）
   → 滚动出的 Pod not-ready → endpointslice 显示 `healthy=false`（Eligible 排除）；dns 模式
   直接丢记录。
5. **watch channel 卫生**：停掉消费端——即使某个 consumer 泄漏了 watch ctx，
   `manager.Destroy` 也会关闭所有 informer（`starter.go:79-86`、`endpointslice.go:230-243`）。
6. **启动 fail-fast（endpointslice）**：集群外不带 `kubeconfig` 启动 → 报错并指出修法。
   单测覆盖：`dns_test.go`、`endpointslice_test.go`（fake resolver + fake clientset 覆盖
   resolve、端口选择、ready/zone 元数据、watch-on-scale——`check.sh` 实际断言的就是这些）。

运行期日志均带 `log.TagAppDef`；本 starter 不发 metrics/trace（可观测由各客户端池自行承担）。

---

## 5. 排障表

| 症状 | 原因 | 处置 |
|------|------|------|
| `resolve "demo" failed ... no such host` | 不在集群 / `cluster-domain` 错 / Service 非 headless | 进集群、修域名、设 `clusterIP: None` |
| `resolve` 返回空且无错 | `namespace` 错，或无 ready 端点 | `kubectl get endpointslices -l kubernetes.io/service-name=demo` |
| 启动报 `dns mode requires port-name (SRV) or port` | 两者都没配 | 配其一（`config.go:101-103`） |
| 启动报 `in-cluster config` | endpointslice 模式在集群外 | 设 `kubeconfig`（`endpointslice.go:97-100`） |
| informer watch 失败 / 日志有 RBAC 报错 | ServiceAccount 缺 endpointslice 权限 | apply `deploy/rbac.yaml`；跨 namespace 用 ClusterRole |
| `cache sync failed for "demo"` | API server 不可达或 watch 被拒 | 检查 API-server 连通与 RBAC；watch 返回错误、无 channel |
| 扩缩容后端点不更新（dns 模式） | DNS TTL + 轮询滞后 | 调低 `refresh-interval` 或切 `endpointslice` |
| 多个未命名端口的 slice 解析不出端点 | `pickPort` 无法选择（`endpointslice.go:290-293`） | 设 `port-name` 或 `port` |
| 永不出现 `:0` 地址 | 无端口的 slice 被跳过而非以 0 输出 | `port-name` 与 Service 端口名对齐 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 8 |
| 必填（按模式条件） | 1（dns 模式下 `port-name` **或** `port`） |
| quickstart 前置外部依赖 | 1（Kubernetes 集群） |
| "注意/坑" 条数 | 4 |

设计嫌疑清单：

- dns 与 endpointslice 的摘流语义不一致（not-ready → Healthy=false vs 记录消失）——DNS 的
  固有限制，已文档化，不可修。
- backend 名默认值不对齐：客户端默认 `discovery=default`，本 starter 无默认名——每份配置都要
  重述这一对；可考虑统一约定。
- 仅 `port` 的 endpointslice 配置静默依赖 slice 恰好只有一个端口（`endpointslice.go:290-293`）
  ——多端口 Service 会安静地配错。
