# starter-lock-k8s 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-lock-k8s` 属于 Contributor 形态（`starter/DESIGN.md` §2.3）的集成层
starter：贡献 Kubernetes `coordination.k8s.io/Lease` 对象后端的 `lock.Locker`
命名 bean。

## 1. 职责与边界

- 把 `spring.lock.<name>` 条目绑定到 Lease 后端的 `lock.Locker` bean，每条
  一个，按 config 名注册并导出为 `lock.Locker`。
- 底层使用 client-go 的 `resourcelock.LeaseLock` 原语，`kube-controller-manager`
  与 spring-cloud-kubernetes 也是同一套构件。
- **不需要**任何外部中间件（Redis / etcd / Consul）；Lease API 是每个
  Kubernetes 控制面自带的。

## 2. 关键抽象与缝隙

- **缝隙是 bean 类型。** 无包级 driver 字符串；切换后端是空导入换包。K8s
  后端的存在证明该缝隙不局限于存储系统。
- **`buildClient` 缝隙。** 测试通过 `newK8sLockerWithClient` 注入 client-go
  的 fake clientset，无需真集群即可对 RBAC / API-server 行为做单测
  （`k8slock_test.go`）。
- **每个持有一条续期 goroutine。** 每个已持有锁自持续期 ticker 与 `Lost()`
  通道；shared clientset 在各持有间复用。`renewLoop` 模仿 client-go
  leaderelection：缺失则创建 Lease、过期/自持则续、临时 API 错误容忍到
  lease 逾期再触发 `Lost`。
- **Unlock 尽力而为释放。** 清空 `holderIdentity`，让 waiter 立刻拿到，
  而不是等 lease duration 到期。失败被吞（lease 到期后会自然释放），保持
  `Unlock` 幂等。

## 3. 约束

- **Lease 名字必须是 DNS-1123 子域。** Lease 对象名为 `KeyPrefix + key`，
  所以 key 只能包含小写字母数字、`-`、`.`——按此选 `KeyPrefix` 与锁 key。
- **RBAC。** 应用 ServiceAccount 必须在 `Config.Namespace` 内对
  `coordination.k8s.io/leases` 拥有 `get/create/update`。
- **Kubeconfig 与集群内。** `Kubeconfig` 为空时使用 in-cluster
  ServiceAccount 配置（K8s 内部署路径）；提供 kubeconfig 路径时通过该文件
  拨号（本地开发 / 测试）。错误路径在启动 fail-fast。
- **Lease TTL 是整秒。** `leaseSeconds` 向上取整、至少为 1，在 K8s 侧保持
  抽象层 TTL 契约。
- **锁时序参数在 `lock.Option` 上，不在 config 上。** TTL、RenewInterval、
  RetryInterval 按 Acquire 传入，让同一套旋钮适配任何后端。

## 4. 权衡 / 已否决方案

- **自定义 CRD——否决。** Lease 是标准 GA API；自定义 CRD 会破坏每个原生
  集群与本地 minikube。
- **复用 client-go 的 `leaderelection.LeaderElector`——否决。** 那个 helper
  拥有自己的回调驱动循环，并不建模按需 `Acquire(key)` API；直接用会让
  锁语义从抽象泄漏。
